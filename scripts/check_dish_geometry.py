"""Detect plate bounds and estimate safe framing for downloaded dish images."""

from __future__ import annotations

import json
import math
from pathlib import Path

import cv2
import numpy as np
from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parents[1]
AUDIT = ROOT / "scripts" / "_cq" / "_img" / "audit-current"
REPORT = AUDIT / "report.json"
SOURCE = AUDIT / "source"
OVERLAYS = AUDIT / "geometry-overlays"
OUTPUT = AUDIT / "geometry-report.json"


def detect_plate(cv_image: np.ndarray) -> dict | None:
    height, width = cv_image.shape[:2]
    scale = 600 / width
    small = cv2.resize(cv_image, (600, round(height * scale)), interpolation=cv2.INTER_AREA)
    gray = cv2.cvtColor(small, cv2.COLOR_BGR2GRAY)
    gray = cv2.medianBlur(gray, 7)
    min_dim = min(gray.shape[:2])
    best = None
    for param2 in (58, 48, 40):
        circles = cv2.HoughCircles(
            gray,
            cv2.HOUGH_GRADIENT,
            dp=1.2,
            minDist=min_dim * 0.35,
            param1=105,
            param2=param2,
            minRadius=round(min_dim * 0.27),
            maxRadius=round(min_dim * 0.52),
        )
        if circles is None:
            continue
        for x, y, radius in np.round(circles[0]).astype(int):
            nx, ny = x / small.shape[1], y / small.shape[0]
            margin = min(x - radius, y - radius, small.shape[1] - x - radius, small.shape[0] - y - radius)
            score = math.hypot(nx - 0.5, ny - 0.5) * 2.0 + max(0, margin / min_dim) * 0.2
            candidate = {
                "x": float(nx),
                "y": float(ny),
                "r": float(radius / min_dim),
                "margin": float(margin / min_dim),
                "score": float(score),
                "method": f"hough-{param2}",
            }
            if best is None or candidate["score"] < best["score"]:
                best = candidate
        if best and best["margin"] >= -0.03:
            break
    return best


def food_bbox(cv_image: np.ndarray, plate: dict | None) -> dict:
    height, width = cv_image.shape[:2]
    image = cv2.resize(cv_image, (600, round(height * 600 / width)), interpolation=cv2.INTER_AREA)
    h, w = image.shape[:2]
    if plate:
        cx, cy = plate["x"] * w, plate["y"] * h
        radius = plate["r"] * min(w, h)
    else:
        cx, cy, radius = w * 0.5, h * 0.5, min(w, h) * 0.43

    yy, xx = np.mgrid[0:h, 0:w]
    distance = np.sqrt((xx - cx) ** 2 + (yy - cy) ** 2)
    inner = distance < radius * 0.82
    ring = (distance > radius * 0.84) & (distance < radius * 0.96)
    if ring.sum() < 200:
        return {"x0": 0, "y0": 0, "x1": 1, "y1": 1, "cx": 0.5, "cy": 0.5, "coverage": 0.0}

    lab = cv2.cvtColor(image, cv2.COLOR_BGR2LAB).astype(np.float32)
    plate_color = np.median(lab[ring], axis=0)
    delta = np.sqrt(((lab - plate_color) ** 2).sum(axis=2))
    hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
    saturation = hsv[..., 1].astype(np.float32)
    dark = 255 - hsv[..., 2].astype(np.float32)
    mask = ((delta > 27) | (saturation > 105) | (dark > 125)) & inner
    mask_u8 = (mask.astype(np.uint8) * 255)
    mask_u8 = cv2.morphologyEx(mask_u8, cv2.MORPH_OPEN, np.ones((7, 7), np.uint8))
    mask_u8 = cv2.morphologyEx(mask_u8, cv2.MORPH_CLOSE, np.ones((21, 21), np.uint8))

    count, labels, stats, centroids = cv2.connectedComponentsWithStats(mask_u8, 8)
    min_area = (radius * radius) * 0.025
    components = [
        (area, x, y, cw, ch)
        for x, y, cw, ch, area in stats[1:]
        if area >= min_area
    ]
    if not components:
        return {"x0": 0, "y0": 0, "x1": 1, "y1": 1, "cx": 0.5, "cy": 0.5, "coverage": 0.0}

    x0 = min(item[1] for item in components)
    y0 = min(item[2] for item in components)
    x1 = max(item[1] + item[3] for item in components)
    y1 = max(item[2] + item[4] for item in components)
    area = sum(item[0] for item in components)
    return {
        "x0": round(x0 / w, 4),
        "y0": round(y0 / h, 4),
        "x1": round(x1 / w, 4),
        "y1": round(y1 / h, 4),
        "cx": round((x0 + x1) / (2 * w), 4),
        "cy": round((y0 + y1) / (2 * h), 4),
        "coverage": round(area / (math.pi * radius * radius), 4),
    }


def save_overlay(path: Path, name: str, plate: dict | None, bbox: dict, index: int) -> None:
    OVERLAYS.mkdir(parents=True, exist_ok=True)
    with Image.open(path) as source:
        image = source.convert("RGB")
    draw = ImageDraw.Draw(image)
    width, height = image.size
    if plate:
        r = plate["r"] * min(width, height)
        x, y = plate["x"] * width, plate["y"] * height
        draw.ellipse((x - r, y - r, x + r, y + r), outline=(30, 220, 100), width=6)
        draw.line((x - 18, y, x + 18, y), fill=(255, 230, 40), width=4)
        draw.line((x, y - 18, x, y + 18), fill=(255, 230, 40), width=4)
    draw.rectangle(
        (bbox["x0"] * width, bbox["y0"] * height, bbox["x1"] * width, bbox["y1"] * height),
        outline=(255, 70, 45),
        width=6,
    )
    image.save(OVERLAYS / f"{index:03d}-{name}.jpg", quality=84)


def main() -> None:
    report = json.loads(REPORT.read_text(encoding="utf-8"))
    output = []
    for index, record in enumerate(report["records"], 1):
        path = SOURCE / record["file"]
        cv_image = cv2.imdecode(np.fromfile(path, dtype=np.uint8), cv2.IMREAD_COLOR)
        if cv_image is None:
            continue
        plate = detect_plate(cv_image)
        bbox = food_bbox(cv_image, plate)
        flags = []
        if plate is None:
            flags.append("未识别器皿")
        else:
            if plate["margin"] < 0.015:
                flags.append("器皿贴边")
            if abs(plate["x"] - 0.5) > 0.045 or abs(plate["y"] - 0.5) > 0.045:
                flags.append("器皿偏心")
        if abs(bbox["cx"] - 0.5) > 0.055 or abs(bbox["cy"] - 0.5) > 0.055:
            flags.append("菜品偏心")
        if bbox["x0"] < 0.025 or bbox["y0"] < 0.025 or bbox["x1"] > 0.975 or bbox["y1"] > 0.975:
            flags.append("菜品贴边")
        item = {
            "name": record["name"],
            "cuisine": record["cuisine"],
            "file": record["file"],
            "plate": plate,
            "food_bbox": bbox,
            "flags": flags,
        }
        output.append(item)
        save_overlay(path, record["name"].replace("/", "_"), plate, bbox, index)

    OUTPUT.write_text(json.dumps(output, ensure_ascii=False, indent=2), encoding="utf-8")
    suspects = [item for item in output if item["flags"]]
    summary = []
    for item in sorted(
        output,
        key=lambda entry: (
            abs(entry["food_bbox"]["cx"] - 0.5) + abs(entry["food_bbox"]["cy"] - 0.5)
        ),
        reverse=True,
    ):
        plate = item["plate"] or {}
        bbox = item["food_bbox"]
        summary.append(
            f"{item['name']}\t"
            f"plate=({plate.get('x', 0):.3f},{plate.get('y', 0):.3f},"
            f"r={plate.get('r', 0):.3f},m={plate.get('margin', 0):.3f})\t"
            f"food=({bbox['cx']:.3f},{bbox['cy']:.3f},"
            f"{bbox['x0']:.3f},{bbox['y0']:.3f},{bbox['x1']:.3f},{bbox['y1']:.3f})\t"
            f"{','.join(item['flags'])}"
        )
    (AUDIT / "geometry-summary.txt").write_text("\n".join(summary), encoding="utf-8")
    print(f"images={len(output)} suspects={len(suspects)}")
    for flag in ["器皿贴边", "器皿偏心", "菜品偏心", "菜品贴边", "未识别器皿"]:
        print(f"{flag}={sum(flag in item['flags'] for item in suspects)}")
    print(f"report={OUTPUT}")
    print(f"overlays={OVERLAYS}")


if __name__ == "__main__":
    main()
