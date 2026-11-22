"""Generate deterministic warm food illustrations for dishes without safe photos.

These are code-native raster illustrations: every subject is drawn inside an
8% safe area, uses the same overhead ceramic-table language, and contains no
text, hands, cutlery, or third-party source material.
"""

from __future__ import annotations

import math
import random
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageFilter, ImageOps


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "web" / "public" / "dish-art"
W, H = 1200, 900


def grain_background(seed: int) -> Image.Image:
    rng = np.random.default_rng(seed)
    y, x = np.mgrid[0:H, 0:W]
    wave = (
        np.sin(x / 31 + y / 180) * 2.2
        + np.sin(x / 9 + y / 51) * 1.1
        + np.sin(y / 11) * 1.8
    )
    noise = rng.normal(0, 2.2, (H, W))
    base = np.empty((H, W, 3), dtype=np.float32)
    base[..., 0] = 214 + wave + noise
    base[..., 1] = 190 + wave * 0.7 + noise * 0.65
    base[..., 2] = 156 + wave * 0.35 + noise * 0.45
    xx, yy = np.meshgrid(np.linspace(-1, 1, W), np.linspace(-1, 1, H))
    vignette = np.clip(1.06 - 0.17 * (xx * xx + yy * yy), 0.82, 1.03)
    base *= vignette[..., None]
    image = Image.fromarray(np.clip(base, 0, 255).astype(np.uint8), "RGB")
    return image.filter(ImageFilter.GaussianBlur(0.35))


def shadowed_plate(
    image: Image.Image,
    center: tuple[int, int] = (600, 455),
    radius: tuple[int, int] = (334, 252),
    style: str = "round",
) -> tuple[Image.Image, tuple[int, int, int, int]]:
    layer = Image.new("RGBA", image.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)
    cx, cy = center
    rx, ry = radius
    if style == "oval":
        rx, ry = int(rx * 1.03), int(ry * 0.86)
        box = (cx - rx, cy - ry, cx + rx, cy + ry)
        draw.ellipse((box[0] + 20, box[1] + 28, box[2] + 20, box[3] + 28), fill=(87, 57, 35, 95))
        shadow = layer.filter(ImageFilter.GaussianBlur(25))
        image = Image.alpha_composite(image.convert("RGBA"), shadow)
        draw = ImageDraw.Draw(image)
        draw.ellipse(box, fill=(225, 212, 188, 255), outline=(176, 148, 117, 255), width=7)
        draw.ellipse((box[0] + 22, box[1] + 18, box[2] - 18, box[3] - 24), outline=(250, 241, 219, 255), width=4)
        inner = (box[0] + 38, box[1] + 35, box[2] - 38, box[3] - 42)
        draw.ellipse(inner, fill=(239, 227, 201, 255), outline=(205, 177, 141, 255), width=3)
        return image, inner

    if style == "rect":
        box = (cx - int(rx * 0.91), cy - int(ry * 0.8), cx + int(rx * 0.91), cy + int(ry * 0.8))
        draw.rounded_rectangle(
            (box[0] + 18, box[1] + 24, box[2] + 18, box[3] + 24),
            radius=74,
            fill=(80, 55, 38, 88),
        )
        image = Image.alpha_composite(image.convert("RGBA"), layer.filter(ImageFilter.GaussianBlur(23)))
        draw = ImageDraw.Draw(image)
        draw.rounded_rectangle(box, radius=72, fill=(226, 214, 191, 255), outline=(170, 143, 116, 255), width=7)
        draw.rounded_rectangle(
            (box[0] + 22, box[1] + 18, box[2] - 18, box[3] - 24),
            radius=54,
            outline=(248, 238, 218, 255),
            width=4,
        )
        inner = (box[0] + 42, box[1] + 37, box[2] - 42, box[3] - 43)
        draw.rounded_rectangle(inner, radius=45, fill=(240, 228, 204, 255), outline=(204, 176, 142, 255), width=2)
        return image, inner

    if style == "bowl":
        rx, ry = int(rx * 0.92), int(ry * 0.86)
        box = (cx - rx, cy - ry, cx + rx, cy + ry)
        draw.ellipse((box[0] + 20, box[1] + 28, box[2] + 20, box[3] + 28), fill=(77, 54, 38, 102))
        image = Image.alpha_composite(image.convert("RGBA"), layer.filter(ImageFilter.GaussianBlur(26)))
        draw = ImageDraw.Draw(image)
        draw.ellipse(box, fill=(185, 128, 88, 255), outline=(120, 75, 48, 255), width=9)
        draw.ellipse((box[0] + 28, box[1] + 22, box[2] - 22, box[3] - 30), fill=(232, 207, 170, 255))
        inner = (box[0] + 60, box[1] + 54, box[2] - 60, box[3] - 62)
        draw.ellipse(inner, fill=(238, 219, 184, 255), outline=(170, 117, 77, 255), width=4)
        return image, inner

    box = (cx - rx, cy - ry, cx + rx, cy + ry)
    draw.ellipse((box[0] + 20, box[1] + 28, box[2] + 20, box[3] + 28), fill=(78, 54, 36, 100))
    image = Image.alpha_composite(image.convert("RGBA"), layer.filter(ImageFilter.GaussianBlur(25)))
    draw = ImageDraw.Draw(image)
    draw.ellipse(box, fill=(221, 207, 181, 255), outline=(171, 142, 111, 255), width=8)
    draw.ellipse((box[0] + 18, box[1] + 14, box[2] - 15, box[3] - 22), outline=(250, 241, 220, 255), width=5)
    inner = (box[0] + 42, box[1] + 39, box[2] - 42, box[3] - 47)
    draw.ellipse(inner, fill=(240, 227, 201, 255), outline=(204, 175, 140, 255), width=3)
    return image, inner


def speckles(draw: ImageDraw.ImageDraw, box: tuple[int, int, int, int], seed: int) -> None:
    rng = random.Random(seed)
    x0, y0, x1, y1 = box
    for _ in range(85):
        x = rng.randint(x0 + 12, x1 - 12)
        y = rng.randint(y0 + 12, y1 - 12)
        radius = rng.choice([1, 1, 1, 2])
        color = rng.choice([(168, 142, 112, 55), (120, 91, 68, 42), (255, 248, 225, 75)])
        draw.ellipse((x - radius, y - radius, x + radius, y + radius), fill=color)


def sauce(draw: ImageDraw.ImageDraw, center: tuple[int, int], size: tuple[int, int], color: tuple[int, ...]) -> None:
    cx, cy = center
    sx, sy = size
    draw.ellipse((cx - sx, cy - sy, cx + sx, cy + sy), fill=color)
    draw.arc(
        (cx - sx * 0.76, cy - sy * 0.73, cx + sx * 0.75, cy + sy * 0.66),
        205,
        330,
        fill=(255, 223, 170, 92),
        width=7,
    )


def glossy_blob(
    draw: ImageDraw.ImageDraw,
    box: tuple[int, int, int, int],
    fill: tuple[int, int, int, int],
    highlight: tuple[int, int, int, int] = (255, 232, 190, 135),
) -> None:
    draw.ellipse(box, fill=fill, outline=(92, 52, 30, 80), width=3)
    x0, y0, x1, y1 = box
    draw.ellipse(
        (x0 + (x1 - x0) * 0.18, y0 + (y1 - y0) * 0.12, x0 + (x1 - x0) * 0.58, y0 + (y1 - y0) * 0.38),
        fill=highlight,
    )


def herb(draw: ImageDraw.ImageDraw, x: float, y: float, angle: float, length: int, color: tuple[int, ...]) -> None:
    dx, dy = math.cos(angle), math.sin(angle)
    nx, ny = -dy, dx
    p = [
        (x, y),
        (x + dx * length * 0.45 + nx * 4, y + dy * length * 0.45 + ny * 4),
        (x + dx * length, y + dy * length),
        (x + dx * length * 0.45 - nx * 4, y + dy * length * 0.45 - ny * 4),
    ]
    draw.polygon(p, fill=color)


def diamond(draw: ImageDraw.ImageDraw, x: int, y: int, r: int, fill: tuple[int, ...]) -> None:
    draw.rounded_rectangle((x - r, y - r, x + r, y + r), radius=5, fill=fill, outline=(101, 68, 43, 60), width=2)


def steam(image: Image.Image, points: list[tuple[int, int]]) -> Image.Image:
    layer = Image.new("RGBA", image.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)
    for x, y in points:
        draw.arc((x - 34, y - 54, x + 28, y + 10), 190, 335, fill=(255, 250, 235, 76), width=8)
        draw.arc((x - 17, y - 82, x + 36, y - 7), 195, 345, fill=(255, 250, 235, 58), width=6)
    return Image.alpha_composite(image, layer.filter(ImageFilter.GaussianBlur(4)))


def base(seed: int, plate_style: str = "round") -> tuple[Image.Image, ImageDraw.ImageDraw, tuple[int, int, int, int]]:
    image = grain_background(seed).convert("RGBA")
    image, inner = shadowed_plate(image, style=plate_style)
    draw = ImageDraw.Draw(image)
    speckles(draw, inner, seed)
    return image, draw, inner


def save(image: Image.Image, slug: str) -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    thumb = OUT / "thumb"
    thumb.mkdir(parents=True, exist_ok=True)
    image.convert("RGB").save(OUT / f"{slug}.webp", "WEBP", quality=89, method=6)
    small = ImageOps.contain(image.convert("RGB"), (480, 360), Image.Resampling.LANCZOS)
    canvas = Image.new("RGB", (480, 360), (227, 207, 174))
    canvas.paste(small, ((480 - small.width) // 2, (360 - small.height) // 2))
    canvas.save(thumb / f"{slug}.webp", "WEBP", quality=82, method=6)


def kaolengmian() -> None:
    image, draw, _ = base(101)
    sauce(draw, (600, 455), (292, 196), (151, 76, 47, 52))
    for i, x in enumerate([410, 505, 600, 695, 790]):
        y = 470 + (i % 2) * 20
        draw.rounded_rectangle((x - 66, y - 142, x + 62, y + 138), radius=28, fill=(95, 56, 35, 255))
        draw.rounded_rectangle((x - 54, y - 128, x + 45, y + 124), radius=22, fill=(205, 141, 63, 255))
        draw.line((x - 43, y - 100, x + 34, y + 102), fill=(239, 198, 103, 255), width=12)
        draw.line((x - 50, y + 88, x + 40, y - 93), fill=(182, 44, 34, 255), width=6)
        draw.ellipse((x - 70, y - 58, x - 42, y + 60), fill=(199, 92, 74, 255))
    for x, y in [(465, 350), (560, 610), (745, 370), (700, 605), (835, 520)]:
        herb(draw, x, y, -0.7, 28, (87, 137, 58, 255))
    save(image, "kaolengmian")


def youmianjin() -> None:
    image, draw, _ = base(102, "oval")
    sauce(draw, (600, 454), (288, 174), (118, 47, 29, 115))
    for i, (x, y, r) in enumerate(
        [(470, 390, 63), (580, 350, 68), (690, 402, 62), (515, 505, 68), (635, 520, 72), (755, 505, 58)]
    ):
        glossy_blob(draw, (x - r, y - r, x + r, y + r), (191 + i * 3, 117, 49, 255))
        draw.arc((x - 26, y - 18, x + 22, y + 21), 20, 145, fill=(111, 59, 28, 170), width=5)
    for x, y in [(458, 330), (690, 322), (790, 447), (510, 596), (673, 600)]:
        herb(draw, x, y, -0.8, 31, (90, 139, 62, 255))
    save(image, "youmianjin-sairou")


def xiangyou_shanhu() -> None:
    image, draw, _ = base(103)
    sauce(draw, (600, 452), (270, 178), (99, 42, 27, 185))
    rng = random.Random(203)
    for _ in range(27):
        x = rng.randint(430, 770)
        y = rng.randint(325, 565)
        length = rng.randint(55, 100)
        angle = rng.choice([-0.85, -0.35, 0.2, 0.85])
        draw.arc((x - length, y - 22, x + length, y + 24), 200, 350, fill=(92, 48, 31, 255), width=17)
    glossy_blob(draw, (556, 365, 645, 455), (237, 219, 178, 255), (255, 249, 220, 160))
    for x, y, a in [(506, 375, -0.7), (690, 395, -0.2), (520, 526, 0.5), (695, 520, 0.7)]:
        herb(draw, x, y, a, 35, (94, 146, 62, 255))
    draw.line((470, 445, 722, 474), fill=(221, 126, 55, 90), width=4)
    save(image, "xiangyou-shanhu")


def liurouduan() -> None:
    image, draw, _ = base(104)
    sauce(draw, (600, 452), (278, 182), (125, 66, 31, 100))
    rng = random.Random(204)
    for _ in range(18):
        x = rng.randint(445, 755)
        y = rng.randint(330, 565)
        rw = rng.randint(38, 64)
        rh = rng.randint(34, 58)
        draw.rounded_rectangle(
            (x - rw, y - rh, x + rw, y + rh),
            radius=15,
            fill=rng.choice([(184, 109, 49, 255), (210, 138, 59, 255), (158, 84, 39, 255)]),
            outline=(103, 55, 28, 120),
            width=3,
        )
        draw.arc((x - rw + 8, y - rh + 7, x + rw - 9, y - rh + 31), 200, 330, fill=(255, 219, 143, 120), width=5)
    for x, y in [(470, 320), (565, 588), (681, 330), (737, 552)]:
        diamond(draw, x, y, 23, (83, 139, 64, 255))
    for x, y in [(515, 582), (710, 356)]:
        diamond(draw, x, y, 19, (222, 139, 59, 255))
    for x, y in [(620, 444), (659, 485), (565, 516)]:
        draw.arc((x - 22, y - 16, x + 24, y + 18), 200, 340, fill=(244, 207, 134, 135), width=4)
    save(image, "liurouduan")


def jiangdagu() -> None:
    image, draw, _ = base(105, "rect")
    sauce(draw, (600, 452), (285, 154), (80, 34, 25, 145))
    bones = [(470, 375, -0.5), (625, 350, 0.12), (675, 520, -0.12), (510, 535, 0.6)]
    for x, y, angle in bones:
        length, width = 170, 66
        dx, dy = math.cos(angle), math.sin(angle)
        nx, ny = -dy, dx
        p = [
            (x - dx * length / 2 + nx * width / 2, y - dy * length / 2 + ny * width / 2),
            (x + dx * length / 2 + nx * width / 2, y + dy * length / 2 + ny * width / 2),
            (x + dx * length / 2 - nx * width / 2, y + dy * length / 2 - ny * width / 2),
            (x - dx * length / 2 - nx * width / 2, y - dy * length / 2 - ny * width / 2),
        ]
        draw.line(p + [p[0]], fill=(86, 45, 30, 255), width=8, joint="curve")
        draw.line(
            (x - dx * 45, y - dy * 45, x + dx * 72, y + dy * 72),
            fill=(236, 222, 192, 255),
            width=28,
        )
        for sign in (-1, 1):
            sx = x + sign * dx * 82
            sy = y + sign * dy * 82
            draw.ellipse((sx - 28, sy - 28, sx + 28, sy + 28), fill=(239, 226, 199, 255), outline=(126, 92, 66, 150), width=3)
    for x, y in [(430, 330), (770, 352), (430, 570), (750, 572), (600, 305)]:
        herb(draw, x, y, -0.6, 33, (81, 134, 52, 255))
    for x, y in [(470, 455), (710, 445)]:
        star = []
        for i in range(16):
            angle = i * math.pi / 8
            radius = 18 if i % 2 == 0 else 7
            star.append((x + math.cos(angle) * radius, y + math.sin(angle) * radius))
        draw.polygon(star, fill=(91, 49, 31, 255))
    save(image, "jiangdagu")


def taipingyan() -> None:
    image, draw, inner = base(106, "bowl")
    ix0, iy0, ix1, iy1 = inner
    draw.ellipse((ix0 + 28, iy0 + 28, ix1 - 28, iy1 - 28), fill=(215, 192, 137, 235))
    for x, y in [(475, 372), (600, 352), (715, 390), (505, 505), (635, 520), (730, 505)]:
        draw.ellipse((x - 47, y - 34, x + 47, y + 34), fill=(239, 214, 166, 255), outline=(198, 160, 104, 150), width=3)
        draw.arc((x - 28, y - 26, x + 24, y + 10), 205, 345, fill=(252, 241, 209, 170), width=5)
    for x, y in [(420, 460), (780, 452)]:
        draw.ellipse((x - 36, y - 44, x + 36, y + 44), fill=(246, 236, 207, 255), outline=(214, 193, 159, 180), width=3)
    for x, y in [(540, 435), (690, 440), (590, 575)]:
        draw.ellipse((x - 7, y - 4, x + 7, y + 4), fill=(214, 159, 104, 255))
    for x, y, a in [(450, 322, -0.7), (739, 330, -0.2), (545, 589, 0.3), (690, 590, 0.8)]:
        herb(draw, x, y, a, 29, (82, 145, 61, 255))
    image = steam(image, [(500, 295), (600, 275), (700, 294)])
    save(image, "taipingyan")


def hongzaorou() -> None:
    image, draw, _ = base(107, "oval")
    sauce(draw, (600, 452), (280, 170), (111, 39, 31, 105))
    for i, x in enumerate([430, 482, 535, 588, 641, 694, 747]):
        y = 455 + (i % 2) * 10
        draw.rounded_rectangle((x - 38, y - 128, x + 38, y + 125), radius=28, fill=(155, 51, 43, 255), outline=(86, 30, 27, 150), width=3)
        draw.line((x - 18, y - 90, x - 18, y + 91), fill=(223, 134, 102, 210), width=6)
        draw.arc((x - 28, y - 105, x + 24, y - 65), 195, 330, fill=(255, 202, 152, 120), width=5)
    for x, y, a in [(446, 316, -0.5), (736, 342, -0.2), (500, 592, 0.4), (686, 580, 0.8)]:
        herb(draw, x, y, a, 34, (84, 142, 57, 255))
    save(image, "hongzaorou")


def qingzheng_shiji() -> None:
    image, draw, _ = base(108, "oval")
    sauce(draw, (600, 458), (265, 150), (209, 176, 122, 70))
    for x, y, scale in [(520, 420, 1.0), (675, 470, 0.88)]:
        draw.ellipse((x - 65 * scale, y - 38 * scale, x + 65 * scale, y + 44 * scale), fill=(207, 190, 139, 255), outline=(133, 103, 67, 130), width=3)
        draw.ellipse((x - 28 * scale + 49 * scale, y - 34 * scale - 22 * scale, x - 28 * scale + 77 * scale, y - 34 * scale + 2 * scale), fill=(61, 48, 36, 255))
        draw.ellipse((x + 42 * scale, y - 3 * scale, x + 74 * scale, y + 17 * scale), fill=(61, 48, 36, 255))
        for leg in [-1, 1]:
            draw.arc(
                (x + leg * 42 * scale - 38 * scale, y + 20 * scale, x + leg * 42 * scale + 42 * scale, y + 105 * scale),
                205,
                330,
                fill=(198, 174, 119, 255),
                width=14,
            )
    for x, y, a in [(470, 335, -0.8), (684, 328, -0.3), (523, 566, 0.4), (701, 568, 0.8), (596, 355, -0.1)]:
        herb(draw, x, y, a, 36, (77, 137, 54, 255))
    for x, y in [(580, 505), (650, 360)]:
        draw.rounded_rectangle((x, y, x + 40, y + 11), radius=5, fill=(231, 190, 92, 255))
    image = steam(image, [(552, 300), (670, 300)])
    save(image, "qingzheng-shiji")


def wenzheng_shansun() -> None:
    image, draw, inner = base(109, "bowl")
    ix0, iy0, ix1, iy1 = inner
    draw.ellipse((ix0 + 24, iy0 + 25, ix1 - 24, iy1 - 25), fill=(221, 198, 153, 235))
    for x, y, rot in [(485, 380, 0.25), (610, 355, -0.2), (690, 470, 2.4), (530, 520, 2.9), (645, 545, 0.4)]:
        draw.ellipse((x - 66, y - 42, x + 66, y + 42), fill=(227, 208, 151, 255), outline=(154, 121, 71, 150), width=3)
        draw.polygon([(x - 58, y - 23), (x + 18, y - 36), (x + 57, y + 15), (x - 20, y + 35)], fill=(239, 222, 165, 255))
        draw.line((x - 32, y - 5, x + 34, y + 6), fill=(183, 146, 83, 180), width=4)
    draw.rounded_rectangle((405, 430, 495, 470), radius=17, fill=(167, 101, 67, 255), outline=(106, 56, 36, 140), width=3)
    for x, y in [(432, 325), (708, 330), (532, 584), (689, 585)]:
        herb(draw, x, y, -0.6, 30, (87, 145, 60, 255))
    image = steam(image, [(545, 295), (650, 290)])
    save(image, "wenzheng-shansun")


def mizhi_huofang() -> None:
    image, draw, _ = base(110, "rect")
    sauce(draw, (600, 455), (280, 170), (95, 40, 25, 95))
    draw.rounded_rectangle((458, 350, 742, 575), radius=34, fill=(128, 48, 28, 255), outline=(72, 29, 21, 180), width=5)
    draw.rounded_rectangle((480, 366, 720, 560), radius=24, fill=(171, 69, 36, 255))
    draw.line((510, 395, 686, 395), fill=(247, 169, 97, 180), width=8)
    draw.line((500, 430, 698, 430), fill=(95, 34, 24, 170), width=5)
    draw.line((505, 530, 691, 530), fill=(255, 194, 116, 90), width=7)
    for x, y in [(408, 356), (787, 370), (420, 555), (779, 548), (510, 305), (688, 300)]:
        draw.ellipse((x - 20, y - 13, x + 20, y + 13), fill=(226, 197, 121, 255), outline=(150, 110, 62, 130), width=2)
    for x, y in [(350, 455), (850, 460)]:
        draw.ellipse((x - 25, y - 31, x + 25, y + 31), fill=(124, 44, 36, 255), outline=(82, 30, 24, 130), width=2)
    save(image, "mizhi-huofang")


def xihu_chuncai() -> None:
    image, draw, inner = base(111, "bowl")
    ix0, iy0, ix1, iy1 = inner
    draw.ellipse((ix0 + 24, iy0 + 25, ix1 - 24, iy1 - 25), fill=(216, 205, 154, 225))
    rng = random.Random(311)
    for _ in range(38):
        x = rng.randint(470, 735)
        y = rng.randint(345, 570)
        rx = rng.randint(18, 35)
        ry = rng.randint(8, 16)
        draw.ellipse((x - rx, y - ry, x + rx, y + ry), fill=rng.choice([(77, 129, 59, 255), (93, 146, 64, 255), (62, 110, 52, 255)]))
        draw.arc((x - rx, y - ry, x + rx, y + ry), 180, 345, fill=(174, 202, 118, 150), width=3)
    for x, y in [(500, 390), (690, 370), (555, 505), (700, 520)]:
        draw.rounded_rectangle((x, y, x + 58, y + 10), radius=5, fill=(236, 207, 171, 255))
    for x, y in [(610, 430), (650, 468), (555, 465)]:
        draw.line((x, y, x + 30, y + 4), fill=(205, 111, 83, 230), width=7)
    image = steam(image, [(540, 300), (660, 292)])
    save(image, "xihu-chuncaitang")


def xiangxi_suanrou() -> None:
    image, draw, _ = base(112)
    sauce(draw, (600, 452), (276, 182), (94, 38, 23, 120))
    rng = random.Random(412)
    for _ in range(16):
        x = rng.randint(445, 750)
        y = rng.randint(335, 560)
        w = rng.randint(42, 69)
        h = rng.randint(22, 40)
        draw.rounded_rectangle((x - w, y - h, x + w, y + h), radius=13, fill=rng.choice([(143, 64, 44, 255), (169, 83, 50, 255), (121, 53, 40, 255)]), outline=(77, 34, 25, 130), width=3)
        draw.arc((x - w + 8, y - h + 5, x + w - 10, y - h + 21), 200, 335, fill=(241, 158, 99, 95), width=4)
    for x, y in [(470, 318), (562, 590), (699, 326), (733, 555)]:
        diamond(draw, x, y, 23, (74, 133, 52, 255))
    for x, y in [(521, 576), (672, 350)]:
        diamond(draw, x, y, 19, (184, 45, 35, 255))
    for x, y, a in [(446, 450, -0.8), (751, 446, -0.2), (535, 316, -0.4), (688, 590, 0.8)]:
        herb(draw, x, y, a, 33, (84, 143, 56, 255))
    save(image, "xiangxi-suanrou")


def piyazinaan() -> None:
    image = grain_background(113).convert("RGBA")
    layer = Image.new("RGBA", image.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)
    draw.ellipse((358, 214, 842, 700), fill=(88, 54, 34, 95))
    image = Image.alpha_composite(image, layer.filter(ImageFilter.GaussianBlur(25)))
    draw = ImageDraw.Draw(image)
    draw.ellipse((350, 200, 850, 702), fill=(175, 112, 50, 255), outline=(119, 69, 31, 255), width=7)
    draw.ellipse((380, 230, 820, 672), fill=(217, 155, 73, 255))
    draw.ellipse((423, 275, 777, 628), fill=(190, 123, 48, 255), outline=(140, 79, 30, 180), width=8)
    for x in range(470, 750, 48):
        for y in range(320, 580, 42):
            draw.ellipse((x - 6, y - 4, x + 6, y + 4), fill=(132, 74, 30, 160))
    rng = random.Random(513)
    for _ in range(65):
        x = rng.randint(410, 790)
        y = rng.randint(260, 620)
        if 432 < x < 768 and 286 < y < 610:
            if rng.random() < 0.5:
                continue
        draw.arc((x - 13, y - 7, x + 13, y + 7), 170, 350, fill=(246, 227, 182, 255), width=4)
    for x, y in [(440, 290), (750, 315), (470, 605), (730, 590), (600, 250), (590, 635)]:
        draw.ellipse((x, y, x + 10, y + 6), fill=(235, 213, 169, 255))
    save(image, "piyazinaan")


def main() -> None:
    for draw_dish in [
        kaolengmian,
        youmianjin,
        xiangyou_shanhu,
        liurouduan,
        jiangdagu,
        taipingyan,
        hongzaorou,
        qingzheng_shiji,
        wenzheng_shansun,
        mizhi_huofang,
        xihu_chuncai,
        xiangxi_suanrou,
        piyazinaan,
    ]:
        draw_dish()
    print(f"generated 13 illustrations in {OUT}")


if __name__ == "__main__":
    main()
