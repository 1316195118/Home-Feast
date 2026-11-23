#!/usr/bin/env python3
"""Audit and losslessly/visually safely optimize images embedded in web/public.

The project already uses WebP for dish photos and pantry assets. This script
keeps existing URLs stable, only replaces a file when the new WebP is materially
smaller, and defaults to a dry run.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import tempfile

from PIL import Image, ImageOps


ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "web" / "public"
CONTENT_EXTS = {".webp"}
BRAND_FILES = {
    "og-cover.png",
    "favicon.ico",
    "apple-touch-icon.png",
    "logo-192.png",
    "logo-512.png",
    "icon-192.png",
    "icon-512.png",
}


def fits_webp(path: Path, min_bytes: int) -> bool:
    return path.suffix.lower() in CONTENT_EXTS and path.name not in BRAND_FILES and path.stat().st_size >= min_bytes


def optimize(path: Path, quality: int, min_saving: float, write: bool) -> tuple[int, int, bool]:
    before = path.stat().st_size
    with Image.open(path) as source:
        image = ImageOps.exif_transpose(source)
        image.load()
        alpha = "A" in image.getbands() or image.mode == "P"
        if image.mode not in ("RGB", "RGBA"):
            image = image.convert("RGBA" if alpha else "RGB")

        fd, temp_name = tempfile.mkstemp(prefix=".fm-image-", suffix=".webp", dir=path.parent)
        os.close(fd)
        temp = Path(temp_name)
        try:
            save_options = {
                "format": "WEBP",
                "quality": quality,
                "method": 6,
                "optimize": True,
            }
            if alpha:
                save_options["alpha_quality"] = min(quality, 88)
            image.save(temp, **save_options)
            after = temp.stat().st_size
            improved = after < before * (1 - min_saving)
            if write and improved:
                os.replace(temp, path)
            return before, after, improved
        finally:
            temp.unlink(missing_ok=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--write", action="store_true", help="replace files that improve by at least --min-saving")
    parser.add_argument("--quality", type=int, default=84)
    parser.add_argument("--min-bytes", type=int, default=80_000)
    parser.add_argument("--min-saving", type=float, default=0.04)
    args = parser.parse_args()

    files = sorted((p for p in PUBLIC.rglob("*") if p.is_file() and fits_webp(p, args.min_bytes)),
                   key=lambda p: p.stat().st_size, reverse=True)
    total_before = total_after = replaced = 0
    for path in files:
        before, after, improved = optimize(path, args.quality, args.min_saving, args.write)
        total_before += before
        total_after += after if improved or not args.write else before
        if improved:
            replaced += 1
            action = "optimized" if args.write else "would optimize"
        else:
            action = "keep"
        rel = path.relative_to(ROOT)
        print(f"{action:14} {rel}  {before / 1024:.1f} KB -> {after / 1024:.1f} KB")

    saved = total_before - total_after
    mode = "written" if args.write else "dry-run"
    print(f"\n{mode}: {len(files)} files, {replaced} improved, saved {saved / 1024:.1f} KB")
    if not args.write and replaced:
        print("Run again with --write to apply the improvements.")


if __name__ == "__main__":
    main()
