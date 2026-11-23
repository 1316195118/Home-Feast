#!/usr/bin/env python3
"""Generate crisp PWA icons from the same bowl-and-steam brand mark."""

import json
from pathlib import Path

from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "web" / "public"


def draw_icon(size: int) -> Image.Image:
    scale = 4
    s = size * scale
    image = Image.new("RGBA", (s, s), "#fbf6ee")
    draw = ImageDraw.Draw(image)
    bg = (197, 90, 60, 255)
    ink = (98, 86, 74, 255)
    cream = (251, 246, 238, 255)
    rice = (239, 216, 169, 255)
    green = (109, 145, 119, 255)
    tomato = (216, 88, 61, 255)

    draw.rounded_rectangle((2 * scale, 2 * scale, s - 2 * scale, s - 2 * scale), radius=20 * scale, fill=cream)
    draw.ellipse((10 * scale, 7 * scale, 54 * scale, 51 * scale), fill=(247, 230, 210, 255))
    for x, offset in ((24, 0), (32, 2), (40, 0)):
        draw.arc(
            ((x - 5) * scale, (16 + offset) * scale, (x + 3) * scale, (29 + offset) * scale),
            200,
            310,
            fill=(192, 90, 60, 180),
            width=max(2, round(2.4 * scale)),
        )
    draw.line(((41 * scale, 11 * scale), (55 * scale, 25 * scale)), fill=ink, width=round(2.4 * scale))
    draw.line(((46 * scale, 8 * scale), (57 * scale, 19 * scale)), fill=ink, width=round(2.2 * scale))
    draw.pieslice((17 * scale, 21 * scale, 49 * scale, 44 * scale), 180, 360, fill=rice, outline=(164, 70, 44, 255), width=round(1.8 * scale))
    draw.pieslice((12 * scale, 27 * scale, 54 * scale, 57 * scale), 0, 180, fill=(207, 107, 76, 255), outline=(164, 70, 44, 255), width=round(1.8 * scale))
    draw.line((12 * scale, 34 * scale, 54 * scale, 34 * scale), fill=bg, width=round(3 * scale))
    draw.ellipse((19 * scale, 25 * scale, 27 * scale, 33 * scale), fill=tomato)
    draw.ellipse((23 * scale, 21 * scale, 29 * scale, 25 * scale), fill=green)
    return image.resize((size, size), Image.Resampling.LANCZOS)


def main() -> None:
    outputs = {}
    for name, size in (
        ("apple-touch-icon.png", 180),
        ("logo-192.png", 192),
        ("logo-512.png", 512),
        ("icon-192.png", 192),
        ("icon-512.png", 512),
    ):
        path = PUBLIC / name
        draw_icon(size).save(path, "PNG", optimize=True)
        outputs[name] = path
        print(path)

    favicon = PUBLIC / "favicon.ico"
    draw_icon(256).save(
        favicon,
        format="ICO",
        sizes=[(16, 16), (32, 32), (48, 48)],
    )
    outputs[favicon.name] = favicon
    print(favicon)

    manifest = {
        "name": "家里吃点啥 · 家庭点菜系统",
        "short_name": "家里吃点啥",
        "description": "一家人自己用的点菜系统，数据只存在自己的 NAS / 本机",
        "start_url": "/",
        "scope": "/",
        "display": "standalone",
        "orientation": "portrait-primary",
        "background_color": "#fbf6ee",
        "theme_color": "#fbf6ee",
        "lang": "zh-CN",
        "icons": [
            {
                "src": "/logo-192.png",
                "sizes": "192x192",
                "type": "image/png",
                "purpose": "any maskable",
            },
            {
                "src": "/logo-512.png",
                "sizes": "512x512",
                "type": "image/png",
                "purpose": "any maskable",
            },
        ],
    }
    for name in ("site.webmanifest", "manifest.json"):
        path = PUBLIC / name
        path.write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        outputs[name] = path
        print(path)


if __name__ == "__main__":
    main()
