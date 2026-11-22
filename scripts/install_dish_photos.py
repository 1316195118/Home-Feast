#!/usr/bin/env python3
"""Install the curated Seedream dish photos into the bundled web assets."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

from PIL import Image, ImageOps

from generate_dish_repairs import REPAIRS, OUT as REPAIR_OUT


ROOT = Path(__file__).resolve().parents[1]
GENERATED = ROOT / "scripts" / "_cq" / "_gen_seedream55"
PUBLIC = ROOT / "web" / "public" / "dish-photo"
MAP_FILE = ROOT / "web" / "src" / "dish-photo-map.json"
REPORT_FILE = ROOT / "scripts" / "_cq" / "_img" / "dish-photo-install.json"
FULL_SIZE = (1200, 900)
THUMB_SIZE = (480, 360)
PAPER = (251, 246, 238)

INITIAL = {
    "烤冷面": "kaolengmian",
    "油面筋塞肉": "youmianjin-sairou",
    "溜肉段": "liurouduan",
    "酱大骨": "jiangdagu",
    "太平燕": "taipingyan",
    "红糟肉": "hongzaorou",
    "清蒸石鸡": "qingzheng-shiji",
    "问政山笋": "wenzheng-shansun",
    "蜜汁火方": "mizhi-huofang",
    "皮芽子馕": "piyazinaan",
}

PREFERRED = {
    "wensi-doufu": "seedream_1795346798_1.png",
    "xihu-chuncaitang-v2": "seedream_1795346087_1.png",
}


def newest_png(directory: Path) -> Path:
    files = sorted(directory.rglob("*.png"), key=lambda path: path.stat().st_mtime)
    if not files:
        raise FileNotFoundError(f"no PNG found below {directory}")
    return files[-1]


def selected_png(directory: Path, slug: str) -> Path:
    preferred = PREFERRED.get(slug)
    if not preferred:
        return newest_png(directory)
    matches = sorted(directory.rglob(preferred))
    if not matches:
        raise FileNotFoundError(f"preferred PNG {preferred} not found below {directory}")
    return matches[-1]


def save_variant(source: Path, target: Path, size: tuple[int, int], quality: int) -> dict:
    with Image.open(source) as opened:
        image = opened.convert("RGB")
    # Pad instead of crop so no food or vessel can be cut during conversion.
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
        "file": str(target.relative_to(ROOT)),
        "width": image.width,
        "height": image.height,
        "bytes": target.stat().st_size,
        "sha256": hashlib.sha256(target.read_bytes()).hexdigest(),
    }


def main() -> None:
    photos: dict[str, Path] = {}
    for dish, slug, _ in REPAIRS:
        photos[dish] = selected_png(REPAIR_OUT / slug, slug)
    for dish, slug in INITIAL.items():
        photos.setdefault(dish, selected_png(GENERATED / slug, slug))

    mapping: dict[str, str] = {}
    report: list[dict] = []
    for dish, source in sorted(photos.items()):
        slug = next(
            (slug for name, slug, _ in REPAIRS if name == dish),
            INITIAL.get(dish),
        )
        if not slug:
            raise RuntimeError(f"missing slug for {dish}")
        full = save_variant(source, PUBLIC / f"{slug}.webp", FULL_SIZE, 88)
        thumb = save_variant(source, PUBLIC / "thumb" / f"{slug}.webp", THUMB_SIZE, 82)
        mapping[dish] = slug
        report.append(
            {
                "dish": dish,
                "slug": slug,
                "source": str(source.relative_to(ROOT)),
                "full": full,
                "thumb": thumb,
            }
        )

    MAP_FILE.write_text(
        json.dumps(mapping, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    REPORT_FILE.parent.mkdir(parents=True, exist_ok=True)
    REPORT_FILE.write_text(
        json.dumps(report, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    print(f"installed={len(report)} map={MAP_FILE} public={PUBLIC}")
    print(f"report={REPORT_FILE}")


if __name__ == "__main__":
    main()
