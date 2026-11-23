#!/usr/bin/env python3
"""Render the 1200x630 social preview with the product's warm paper style."""

from pathlib import Path
import random

from PIL import Image, ImageDraw, ImageFilter, ImageFont


ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "web" / "public"
OUTPUT = PUBLIC / "og-cover.png"
W, H = 1200, 630

FONT_REGULAR = Path(r"C:\Windows\Fonts\msyh.ttc")
FONT_BOLD = Path(r"C:\Windows\Fonts\msyhbd.ttc")

PAPER = "#fbf6ee"
PAPER_DEEP = "#f5ecdd"
INK = "#3c3229"
INK_SOFT = "#62564a"
BRAND = "#b64f32"
BRAND_DARK = "#a4462c"
BRAND_SOFT = "#f3d8c9"
GREEN = "#4e7a5a"
GREEN_SOFT = "#e4efe5"
AMBER = "#d5a05b"
AMBER_SOFT = "#f7e4c5"


def font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(str(FONT_BOLD if bold else FONT_REGULAR), size)


def rounded(draw: ImageDraw.ImageDraw, box, radius, fill, outline=None, width=1):
    draw.rounded_rectangle(box, radius=radius, fill=fill, outline=outline, width=width)


def draw_brand_mark(draw: ImageDraw.ImageDraw, x: int, y: int, size: int):
    s = size / 64
    rounded(draw, (x, y, x + size, y + size), round(16 * s), "#fffaf2")
    draw.arc(
        (x + round(18 * s), y + round(11 * s), x + round(47 * s), y + round(35 * s)),
        180,
        360,
        fill="#efd8a9",
        width=max(2, round(5 * s)),
    )
    draw.arc(
        (x + round(10 * s), y + round(26 * s), x + round(55 * s), y + round(58 * s)),
        0,
        180,
        fill=BRAND,
        width=max(2, round(4 * s)),
    )
    draw.line(
        (x + round(11 * s), y + round(34 * s), x + round(53 * s), y + round(34 * s)),
        fill=BRAND_DARK,
        width=max(2, round(3 * s)),
    )
    draw.ellipse(
        (x + round(20 * s), y + round(25 * s), x + round(29 * s), y + round(34 * s)),
        fill="#d8583d",
    )
    draw.arc(
        (x + round(25 * s), y + round(12 * s), x + round(35 * s), y + round(27 * s)),
        200,
        330,
        fill=BRAND,
        width=max(2, round(2 * s)),
    )
    draw.line(
        (x + round(39 * s), y + round(10 * s), x + round(55 * s), y + round(26 * s)),
        fill=INK_SOFT,
        width=max(2, round(2 * s)),
    )


def main() -> None:
    canvas = Image.new("RGB", (W, H), PAPER)
    glow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)
    glow_draw.ellipse((720, -190, 1320, 410), fill=(238, 196, 158, 72))
    glow_draw.ellipse((-210, 355, 410, 950), fill=(218, 232, 216, 82))
    glow = glow.filter(ImageFilter.GaussianBlur(48))
    canvas = Image.alpha_composite(canvas.convert("RGBA"), glow)

    texture = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    texture_draw = ImageDraw.Draw(texture)
    rnd = random.Random(20261123)
    for _ in range(2100):
        x, y = rnd.randrange(W), rnd.randrange(H)
        a = rnd.randrange(8, 21)
        r = rnd.choice((0, 1, 1))
        texture_draw.ellipse((x, y, x + r, y + r), fill=(115, 85, 55, a))
    canvas = Image.alpha_composite(canvas, texture)
    draw = ImageDraw.Draw(canvas)

    # Left: identity and value proposition.
    rounded(draw, (72, 62, 392, 130), 34, "#fffaf2", "#ead9c3", 1)
    draw_brand_mark(draw, 82, 72, 48)
    draw.text((144, 85), "家里吃点啥", font=font(28, True), fill=INK)
    draw.text((78, 174), "一家人的饭，一起决定", font=font(24, True), fill=BRAND_DARK)
    draw.text((72, 218), "今天吃什么，", font=font(76, True), fill=INK)
    draw.text((72, 303), "全家一起定", font=font(76, True), fill=INK)
    draw.rounded_rectangle((74, 395, 378, 403), radius=4, fill=BRAND)
    draw.text(
        (74, 426),
        "挑菜 · 点菜 · 定分工 · 看记录",
        font=font(28),
        fill=INK_SOFT,
    )

    chips = (
        ("237 道家常菜", BRAND_SOFT, BRAND_DARK),
        ("多人实时同步", GREEN_SOFT, GREEN),
        ("数据留在自己设备", AMBER_SOFT, "#7a5728"),
    )
    x = 72
    for label, bg, fg in chips:
        width = int(draw.textlength(label, font=font(20, True))) + 38
        rounded(draw, (x, 505, x + width, 555), 25, bg)
        draw.text((x + 19, 517), label, font=font(20, True), fill=fg)
        x += width + 12

    # Right: a generous illustrated table setting, kept clear of the headline.
    plate_shadow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    shadow_draw = ImageDraw.Draw(plate_shadow)
    shadow_draw.ellipse((790, 228, 1132, 570), fill=(82, 55, 35, 48))
    plate_shadow = plate_shadow.filter(ImageFilter.GaussianBlur(22))
    canvas = Image.alpha_composite(canvas, plate_shadow)
    draw = ImageDraw.Draw(canvas)

    draw.ellipse((823, 170, 1173, 520), fill="#fffaf2", outline="#dfc9ae", width=4)
    draw.ellipse((848, 195, 1148, 495), fill="#f7ecdc", outline=AMBER, width=3)
    draw.ellipse((885, 232, 1111, 458), fill="#fffdf9", outline="#d9c3a8", width=2)
    draw.arc((904, 252, 1092, 436), 196, 344, fill="#e8d3ae", width=18)
    draw.ellipse((933, 286, 1065, 404), fill="#ead39f")
    draw.ellipse((947, 296, 1051, 390), fill="#d98954")
    draw.ellipse((967, 310, 1031, 369), fill="#e7b766")

    # Food pieces and greens make the plate read as a full home-cooked meal.
    for cx, cy, rx, ry, color in (
        (925, 286, 23, 18, "#ce5d3e"),
        (979, 279, 25, 20, "#b64f32"),
        (1044, 292, 24, 20, "#d57644"),
        (944, 357, 24, 19, "#cf643e"),
        (1012, 362, 26, 20, "#bb5637"),
        (1060, 342, 18, 22, "#8d9d55"),
        (930, 323, 19, 14, "#7b9b64"),
        (1000, 329, 17, 25, "#f0d89f"),
    ):
        draw.ellipse((cx - rx, cy - ry, cx + rx, cy + ry), fill=color)
    for x1, y1, x2, y2 in (
        (913, 267, 951, 305),
        (1030, 310, 1066, 359),
        (950, 347, 986, 380),
    ):
        draw.line((x1, y1, x2, y2), fill="#3e6f4b", width=9)

    # Steam and chopsticks complete the bowl/meal motif.
    steam = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    steam_draw = ImageDraw.Draw(steam)
    for x1, offset in ((936, 0), (982, 7), (1028, 0)):
        steam_draw.arc(
            (x1 - 20, 118 + offset, x1 + 24, 226 + offset),
            200,
            320,
            fill=(182, 79, 50, 125),
            width=6,
        )
    canvas = Image.alpha_composite(canvas, steam.filter(ImageFilter.GaussianBlur(1)))
    draw = ImageDraw.Draw(canvas)
    draw.line((1082, 118, 1168, 238), fill="#54473b", width=8)
    draw.line((1103, 110, 1182, 219), fill="#766658", width=7)

    # Small quality mark for the self-hosted promise.
    rounded(draw, (842, 540, 1159, 586), 23, "#fffaf2", "#ead9c3", 1)
    draw.ellipse((858, 554, 870, 566), fill=GREEN)
    draw.text((882, 549), "自托管 · 无广告 · 无跟踪", font=font(19, True), fill=INK_SOFT)

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    canvas.convert("RGB").save(OUTPUT, "PNG", optimize=True)
    print(OUTPUT)


if __name__ == "__main__":
    main()
