"""Download and audit dish images for framing, centering, and duplicate reuse.

The report is intentionally conservative: automated metrics identify candidates,
then the generated contact sheets are reviewed visually for content mismatches.
"""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageFilter, ImageFont, ImageOps


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_DATA = ROOT / "data" / "dishes.json"
DEFAULT_OUT = ROOT / "scripts" / "_cq" / "_img" / "audit-current"
DEFAULT_BASE = "http://10.10.10.251:8090"
FONT_PATH = Path(r"C:\Windows\Fonts\msyh.ttc")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", type=Path, default=DEFAULT_DATA)
    parser.add_argument("--out", type=Path, default=DEFAULT_OUT)
    parser.add_argument("--base-url", default=DEFAULT_BASE)
    parser.add_argument("--workers", type=int, default=8)
    parser.add_argument("--refresh", action="store_true")
    parser.add_argument("--skip-download", action="store_true")
    return parser.parse_args()


def safe_name(value: str) -> str:
    return "".join(c if c.isalnum() or c in "-_." else "_" for c in value)


def download_one(base_url: str, dish: dict, target: Path) -> tuple[str, str | None]:
    if target.exists() and target.stat().st_size > 128:
        return dish["name"], None
    url = base_url.rstrip("/") + dish["image"]
    last_error: Exception | None = None
    for _ in range(3):
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "family-menu-image-audit/1.0"})
            with urllib.request.urlopen(req, timeout=30) as response:
                payload = response.read()
            if len(payload) < 128:
                raise ValueError(f"small response ({len(payload)} bytes)")
            target.write_bytes(payload)
            return dish["name"], None
        except Exception as exc:  # noqa: BLE001 - report per-image failures
            last_error = exc
    return dish["name"], f"{type(last_error).__name__}: {last_error}"


def ahash(image: Image.Image) -> str:
    gray = ImageOps.grayscale(image).resize((16, 16), Image.Resampling.LANCZOS)
    values = np.asarray(gray, dtype=np.float32)
    bits = values > float(values.mean())
    return f"{int(''.join('1' if bit else '0' for bit in bits.flat), 2):064x}"


def hamming_hex(left: str, right: str) -> int:
    return int(left, 16).__xor__(int(right, 16)).bit_count()


def analyze(path: Path, dish: dict, digest: str) -> dict:
    with Image.open(path) as source:
        image = source.convert("RGB")
    width, height = image.size

    analysis = image.resize((600, round(600 * height / width)), Image.Resampling.LANCZOS)
    aw, ah = analysis.size
    gray = np.asarray(analysis.convert("L"), dtype=np.float32)
    rgb = np.asarray(analysis, dtype=np.float32)

    edge = np.asarray(
        analysis.convert("L").filter(ImageFilter.FIND_EDGES),
        dtype=np.float32,
    )
    edge_norm = edge / max(float(edge.max()), 1.0)
    edge_mask = edge_norm > 0.18

    band_x = max(4, round(aw * 0.025))
    band_y = max(4, round(ah * 0.025))
    border_mask = np.zeros_like(edge_mask, dtype=bool)
    border_mask[:band_y, :] = True
    border_mask[-band_y:, :] = True
    border_mask[:, :band_x] = True
    border_mask[:, -band_x:] = True
    border_edges = float(edge_mask[border_mask].mean())

    border = np.concatenate(
        [
            rgb[:band_y].reshape(-1, 3),
            rgb[-band_y:].reshape(-1, 3),
            rgb[:, :band_x].reshape(-1, 3),
            rgb[:, -band_x:].reshape(-1, 3),
        ]
    )
    background = np.median(border, axis=0)
    color_distance = np.sqrt(((rgb - background) ** 2).sum(axis=2))
    foreground = color_distance > 42
    foreground_ratio = float(foreground.mean())

    ys, xs = np.nonzero(edge_mask)
    if len(xs):
        center_x = float(xs.mean() / max(aw - 1, 1))
        center_y = float(ys.mean() / max(ah - 1, 1))
        edge_pixels = float(len(xs))
    else:
        center_x = center_y = 0.5
        edge_pixels = 0.0

    channel_mean = rgb.reshape(-1, 3).mean(axis=0)
    warmth = float(channel_mean[0] - channel_mean[2])
    contrast = float(gray.std())

    flags: list[str] = []
    if abs((width / height) - 4 / 3) > 0.035:
        flags.append("非4:3")
    if border_edges > 0.105:
        flags.append("疑似主体贴边")
    if abs(center_x - 0.5) > 0.095 or abs(center_y - 0.5) > 0.095:
        flags.append("疑似偏心")
    if foreground_ratio > 0.90:
        flags.append("疑似无留白")
    if contrast < 25:
        flags.append("疑似低对比")

    return {
        "name": dish["name"],
        "cuisine": dish.get("cuisine", ""),
        "tags": dish.get("tags", []),
        "image": dish.get("image", ""),
        "file": path.name,
        "width": width,
        "height": height,
        "aspect": round(width / height, 5),
        "sha256": digest,
        "ahash": ahash(image),
        "border_edges": round(border_edges, 5),
        "foreground_ratio": round(foreground_ratio, 5),
        "edge_centroid_x": round(center_x, 5),
        "edge_centroid_y": round(center_y, 5),
        "edge_pixels": int(edge_pixels),
        "warmth": round(warmth, 2),
        "contrast": round(contrast, 2),
        "flags": flags,
    }


def load_font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    if FONT_PATH.exists():
        return ImageFont.truetype(str(FONT_PATH), size)
    return ImageFont.load_default()


def contact_sheets(records: list[dict], images: dict[str, Image.Image], out_dir: Path) -> None:
    cols, rows = 5, 5
    cell_w, image_h, label_h = 304, 228, 42
    cell_h = image_h + label_h
    title_font = load_font(18)
    meta_font = load_font(13)
    sheet_dir = out_dir / "sheets"
    thumb_dir = out_dir / "thumbs"
    sheet_dir.mkdir(parents=True, exist_ok=True)
    thumb_dir.mkdir(parents=True, exist_ok=True)

    for index, record in enumerate(records, 1):
        source = images[record["name"]]
        thumb = ImageOps.fit(
            source,
            (cell_w - 8, image_h - 8),
            method=Image.Resampling.LANCZOS,
        )
        thumb.save(thumb_dir / f"{index:03d}-{safe_name(record['name'])}.jpg", quality=88)

    per_sheet = cols * rows
    for page, start in enumerate(range(0, len(records), per_sheet), 1):
        chunk = records[start : start + per_sheet]
        sheet = Image.new("RGB", (cell_w * cols, cell_h * rows), (246, 240, 228))
        draw = ImageDraw.Draw(sheet)
        for offset, record in enumerate(chunk):
            index = start + offset + 1
            col, row = offset % cols, offset // cols
            x, y = col * cell_w, row * cell_h
            thumb = Image.open(thumb_dir / f"{index:03d}-{safe_name(record['name'])}.jpg").convert("RGB")
            sheet.paste(thumb, (x + 4, y + 4))
            draw.rectangle(
                (x + 1, y + 1, x + cell_w - 2, y + cell_h - 2),
                outline=(170, 146, 122),
                width=1,
            )
            draw.rectangle(
                (x + 5, y + 5, x + cell_w - 5, y + image_h - 5),
                outline=(212, 92, 64) if record["flags"] else (255, 255, 255),
                width=3 if record["flags"] else 1,
            )
            label = f"{index:03d} {record['name']}"
            draw.text((x + 8, y + image_h + 1), label, font=title_font, fill=(49, 39, 34))
            meta = "  ".join(record["flags"]) if record["flags"] else record["cuisine"]
            meta_color = (180, 66, 48) if record["flags"] else (106, 91, 80)
            draw.text((x + 8, y + image_h + 22), meta, font=meta_font, fill=meta_color)
        sheet.save(sheet_dir / f"sheet-{page:02d}.jpg", quality=90)


def main() -> None:
    args = parse_args()
    out_dir = args.out.resolve()
    source_dir = out_dir / "source"
    source_dir.mkdir(parents=True, exist_ok=True)

    dishes = json.loads(args.data.read_text(encoding="utf-8"))
    selected = [dish for dish in dishes if dish.get("image")]

    jobs: dict[str, Path] = {}
    for index, dish in enumerate(selected, 1):
        suffix = Path(dish["image"]).suffix or ".webp"
        jobs[dish["name"]] = source_dir / f"{index:03d}-{safe_name(dish['name'])}{suffix}"

    failures: dict[str, str] = {}
    if not args.skip_download:
        with ThreadPoolExecutor(max_workers=max(1, args.workers)) as pool:
            futures = {
                pool.submit(download_one, args.base_url, dish, jobs[dish["name"]]): dish["name"]
                for dish in selected
            }
            for future in as_completed(futures):
                name, error = future.result()
                if error:
                    failures[name] = error

    records: list[dict] = []
    images: dict[str, Image.Image] = {}
    for dish in selected:
        path = jobs[dish["name"]]
        if not path.exists():
            continue
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        record = analyze(path, dish, digest)
        records.append(record)
        with Image.open(path) as source:
            images[dish["name"]] = source.convert("RGB")

    exact_groups: dict[str, list[str]] = {}
    for record in records:
        exact_groups.setdefault(record["sha256"], []).append(record["name"])
    duplicates = [names for names in exact_groups.values() if len(names) > 1]

    near_pairs: list[dict] = []
    for left_index, left in enumerate(records):
        for right in records[left_index + 1 :]:
            distance = hamming_hex(left["ahash"], right["ahash"])
            if distance <= 4:
                near_pairs.append(
                    {"a": left["name"], "b": right["name"], "distance": distance}
                )

    contact_sheets(records, images, out_dir)
    report = {
        "source": args.base_url,
        "total_dishes": len(dishes),
        "with_image": len(selected),
        "missing_image": [dish["name"] for dish in dishes if not dish.get("image")],
        "downloaded": len(records),
        "download_failures": failures,
        "metric_suspects": [record for record in records if record["flags"]],
        "exact_duplicates": duplicates,
        "near_duplicates": sorted(near_pairs, key=lambda item: item["distance"]),
        "records": records,
    }
    (out_dir / "report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    with (out_dir / "suspects.csv").open("w", newline="", encoding="utf-8-sig") as handle:
        writer = csv.writer(handle)
        writer.writerow(
            [
                "菜名",
                "菜系",
                "疑似问题",
                "贴边指数",
                "前景占比",
                "焦点X",
                "焦点Y",
            ]
        )
        for record in report["metric_suspects"]:
            writer.writerow(
                [
                    record["name"],
                    record["cuisine"],
                    "、".join(record["flags"]),
                    record["border_edges"],
                    record["foreground_ratio"],
                    record["edge_centroid_x"],
                    record["edge_centroid_y"],
                ]
            )

    print(f"images={len(records)} missing={len(report['missing_image'])} failures={len(failures)}")
    print(f"metric_suspects={len(report['metric_suspects'])} exact_duplicates={len(duplicates)} near_pairs={len(near_pairs)}")
    print(f"report={out_dir / 'report.json'}")
    print(f"sheets={out_dir / 'sheets'}")


if __name__ == "__main__":
    main()
