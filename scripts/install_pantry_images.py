#!/usr/bin/env python3
"""Install realistic pantry photos, build thumbnails, contact sheets and an audit report."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
from pathlib import Path

import numpy as np
from PIL import Image, ImageFont, ImageDraw, ImageOps

from generate_pantry_images import OUT as GENERATED, catalog


ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "web" / "public" / "pantry"
THUMB = PUBLIC / "thumb"
MAP_FILE = ROOT / "web" / "src" / "pantry-photo-map.json"
REPORT_DIR = ROOT / "scripts" / "_cq" / "_pantry_audit"
REPORT_FILE = REPORT_DIR / "report.json"
SHEET_DIR = REPORT_DIR / "sheets"
FULL_SIZE = (512, 512)
THUMB_SIZE = (192, 192)
PAPER = (251, 247, 240)


def source_for(slug: str) -> Path:
    direct = GENERATED / slug / "ingredient.png"
    if direct.exists():
        return direct
    files = sorted((GENERATED / slug).rglob("*.png"), key=lambda path: path.stat().st_mtime)
    if not files:
        raise FileNotFoundError(f"missing generated photo for {slug}")
    return files[-1]


def save_variant(source: Path, target: Path, size: tuple[int, int], quality: int) -> dict:
    with Image.open(source) as opened:
        image = opened.convert("RGB")
    image = ImageOps.pad(
        image,
        size,
        method=Image.Resampling.LANCZOS,
        color=PAPER,
        centering=(0.5, 0.5),
    )
    target.parent.mkdir(parents=True, exist_ok=True)
    image.save(target, "WEBP", quality=quality, method=6)
    return {
        "file": str(target.relative_to(ROOT)).replace("\\", "/"),
        "width": image.width,
        "height": image.height,
        "bytes": target.stat().st_size,
        "sha256": hashlib.sha256(target.read_bytes()).hexdigest(),
    }


def pixel_audit(path: Path) -> dict:
    """Estimate subject bounds against the clean studio background."""
    with Image.open(path) as opened:
        image = opened.convert("RGB").resize((256, 256), Image.Resampling.BILINEAR)
    pixels = np.asarray(image, dtype=np.float32)
    corners = np.concatenate(
        [
            pixels[:12, :12].reshape(-1, 3),
            pixels[:12, -12:].reshape(-1, 3),
            pixels[-12:, :12].reshape(-1, 3),
            pixels[-12:, -12:].reshape(-1, 3),
        ]
    )
    background = np.median(corners, axis=0)
    distance = np.linalg.norm(pixels - background, axis=2)
    mask = distance > 24
    # Ignore isolated sensor-like pixels: keep rows/columns with meaningful coverage.
    row_hits = mask.sum(axis=1)
    col_hits = mask.sum(axis=0)
    rows = np.flatnonzero(row_hits >= 3)
    cols = np.flatnonzero(col_hits >= 3)
    if not len(rows) or not len(cols):
        return {
            "subjectFound": False,
            "marginsPct": [0, 0, 0, 0],
            "centerOffsetPct": [100, 100],
            "coveragePct": 0,
            "backgroundRgb": [round(float(v), 1) for v in background],
        }
    top, bottom = int(rows[0]), int(rows[-1])
    left, right = int(cols[0]), int(cols[-1])
    h, w = mask.shape
    cx = (left + right + 1) / 2 / w * 100
    cy = (top + bottom + 1) / 2 / h * 100
    return {
        "subjectFound": True,
        "bounds": [left, top, right + 1, bottom + 1],
        "marginsPct": [
            round(left / w * 100, 1),
            round(top / h * 100, 1),
            round((w - 1 - right) / w * 100, 1),
            round((h - 1 - bottom) / h * 100, 1),
        ],
        "centerOffsetPct": [round(cx - 50, 1), round(cy - 50, 1)],
        "coveragePct": round(float(mask.mean() * 100), 1),
        "backgroundRgb": [round(float(v), 1) for v in background],
    }


def font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    for candidate in (
        Path(r"C:\Windows\Fonts\msyh.ttc"),
        Path(r"C:\Windows\Fonts\msyhbd.ttc"),
        Path(r"C:\Windows\Fonts\simhei.ttf"),
    ):
        if candidate.exists():
            return ImageFont.truetype(str(candidate), size)
    return ImageFont.load_default()


def make_contact_sheets(rows: list[dict], per_sheet: int = 20) -> list[str]:
    SHEET_DIR.mkdir(parents=True, exist_ok=True)
    label_font = font(18)
    small_font = font(14)
    written: list[str] = []
    cell_w, cell_h = 250, 236
    cols = 5
    for sheet_index in range(math.ceil(len(rows) / per_sheet)):
        subset = rows[sheet_index * per_sheet : (sheet_index + 1) * per_sheet]
        sheet_rows = math.ceil(len(subset) / cols)
        sheet = Image.new("RGB", (cols * cell_w, sheet_rows * cell_h + 58), PAPER)
        draw = ImageDraw.Draw(sheet)
        draw.text((18, 14), f"食材真实图检查 · {sheet_index + 1}/{math.ceil(len(rows) / per_sheet)}", fill=(93, 55, 42), font=font(24))
        for index, row in enumerate(subset):
            x = (index % cols) * cell_w
            y = 58 + (index // cols) * cell_h
            with Image.open(ROOT / row["thumb"]["file"]) as opened:
                thumb = opened.convert("RGB").resize((192, 192), Image.Resampling.LANCZOS)
            sheet.paste(thumb, (x + 29, y + 8))
            draw.text((x + 16, y + 204), row["name"], fill=(54, 43, 38), font=label_font)
            margins = row["audit"].get("marginsPct", [0, 0, 0, 0])
            offset = row["audit"].get("centerOffsetPct", [99, 99])
            draw.text(
                (x + 120, y + 205),
                f"边距 {min(margins):.0f}%  偏心 {max(abs(offset[0]), abs(offset[1])):.0f}%",
                fill=(130, 105, 91),
                font=small_font,
            )
        target = SHEET_DIR / f"ingredients-{sheet_index + 1:02d}.jpg"
        sheet.save(target, "JPEG", quality=88, optimize=True)
        written.append(str(target.relative_to(ROOT)).replace("\\", "/"))
    return written


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--strict", action="store_true", help="fail unless all catalog photos exist")
    args = parser.parse_args()

    rows: list[dict] = []
    missing: list[str] = []
    mapping: dict[str, str] = {}
    PUBLIC.mkdir(parents=True, exist_ok=True)
    THUMB.mkdir(parents=True, exist_ok=True)
    for item in catalog():
        try:
            source = source_for(item["slug"])
        except FileNotFoundError:
            missing.append(item["name"])
            continue
        full = save_variant(source, PUBLIC / f"{item['slug']}.webp", FULL_SIZE, 88)
        thumb = save_variant(source, THUMB / f"{item['slug']}.webp", THUMB_SIZE, 82)
        mapping[item["name"]] = item["slug"]
        rows.append(
            {
                "name": item["name"],
                "group": item["group"],
                "slug": item["slug"],
                "source": str(source.relative_to(ROOT)).replace("\\", "/"),
                "full": full,
                "thumb": thumb,
                "audit": pixel_audit(ROOT / full["file"]),
            }
        )

    MAP_FILE.parent.mkdir(parents=True, exist_ok=True)
    MAP_FILE.write_text(
        json.dumps(mapping, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    REPORT_DIR.mkdir(parents=True, exist_ok=True)
    sheets = make_contact_sheets(rows)
    warnings = [
        row["name"]
        for row in rows
        if not row["audit"].get("subjectFound")
        or min(row["audit"].get("marginsPct", [0])) < 2
        or max(abs(v) for v in row["audit"].get("centerOffsetPct", [99, 99])) > 15
        or row["audit"].get("coveragePct", 0) < 12
        or row["audit"].get("coveragePct", 0) > 82
    ]
    report = {
        "installed": len(rows),
        "catalog": len(mapping) + len(missing),
        "missing": missing,
        "groups": {
            group: sum(row["group"] == group for row in rows)
            for group in sorted({row["group"] for row in rows})
        },
        "geometryWarnings": warnings,
        "contactSheets": sheets,
        "records": rows,
    }
    REPORT_FILE.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"installed={len(rows)} map={MAP_FILE}")
    print(f"report={REPORT_FILE}")
    print(
        f"contact_sheets={len(sheets)} geometry_warnings={len(warnings)} "
        f"catalog={len(mapping) + len(missing)} missing={len(missing)}"
    )
    if warnings:
        print("WARN " + "、".join(warnings))
    if missing:
        print("MISSING " + "、".join(missing))
    if args.strict and missing:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
