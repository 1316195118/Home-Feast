# -*- coding: utf-8 -*-
"""把 AI 写实菜图归一成同一套构图，只用 Pillow：
1. 自动找出菜品主体（盘子）的边界；
2. 以主体中心裁 4:3，四周留约 11% 等距边距 —— 盘子居中、不贴边、上下左右等宽；
3. 把桌面色调平移到统一目标色，消除「每张背景都不一样」。
输出统一 1200x900（4:3），前端 contain 显示时不会再出现留白或半截盘子。
"""
from PIL import Image, ImageDraw, ImageFilter

TARGET_BG = (238, 226, 208)  # 统一的浅木纹桌面色
OUT_SIZE = (1200, 900)


def _hide_watermark(im):
    """用紧邻左侧的桌面区域覆盖右下角平台标识，并用羽化遮罩融入原图，
    避免出现矩形色块；必须在主体识别前做，否则水印会被算进菜品边界导致偏心。"""
    w, h = im.size
    # 只覆盖最右下角标识本身那一小块；区域过大会把盘子边缘一起抹成色块
    ow, oh = int(w * 0.15), int(h * 0.085)
    patch = im.crop((w - 2 * ow, h - oh, w - ow, h))
    fade = max(2, min(ow, oh) // 2)
    mask = Image.new('L', (ow, oh), 255)
    mp = mask.load()
    for y in range(oh):
        for x in range(ow):
            a = min(1.0, x / fade, y / fade)
            mp[x, y] = int(255 * a)
    mask = mask.filter(ImageFilter.GaussianBlur(fade / 2))
    im.paste(patch, (w - ow, h - oh), mask)
    return im


def _corner_color(im):
    """取四角桌面区域的平均色，作为这张图的『背景色』。"""
    w, h = im.size
    px = im.load()
    step = max(1, min(w, h) // 60)
    pts = []
    for (x0, y0, x1, y1) in [
        (0, 0, w // 8, h // 8), (w - w // 8, 0, w, h // 8),
        (0, h - h // 8, w // 8, h), (w - w // 8, h - h // 8, w, h),
    ]:
        for y in range(y0, y1, step):
            for x in range(x0, x1, step):
                pts.append(px[x, y])
    n = len(pts) or 1
    return tuple(sum(p[i] for p in pts) / n for i in range(3))


def _subject_bbox(im, bg):
    """在缩略图上按『与桌面色的差异』找主体，行列投影取有效范围。"""
    small = im.resize((300, 225))
    px = small.load()
    rows = [0] * 225
    cols = [0] * 300
    thr2 = 38 ** 2
    for y in range(225):
        for x in range(300):
            r, g, b = px[x, y]
            if (r - bg[0]) ** 2 + (g - bg[1]) ** 2 + (b - bg[2]) ** 2 > thr2:
                rows[y] += 1
                cols[x] += 1

    def span(arr, total):
        hit = [i for i, v in enumerate(arr) if v > total * 0.02]
        return (hit[0], hit[-1]) if hit else (0, len(arr) - 1)

    ry, rx = span(rows, 300), span(cols, 225)
    sx, sy = im.width / 300, im.height / 225
    return (int(rx[0] * sx), int(ry[0] * sy), int(rx[1] * sx), int(ry[1] * sy))


def _uniform_background(size):
    """所有图片共用同一套自绘浅木纹桌面，保证四角/边缘背景完全一致。"""
    w, h = size
    bg = Image.new('RGB', size, TARGET_BG)
    d = ImageDraw.Draw(bg)
    for i, y in enumerate(range(0, h, 7)):
        tone = 3 if i % 2 == 0 else -4
        c = tuple(max(0, min(255, v + tone)) for v in TARGET_BG)
        d.line((0, y, w, y), fill=c, width=1)
    return bg


def _compose_on_uniform_bg(base, rel_bbox, crop_size, size):
    """只把主体之外（四角、边缘）换成统一桌面；椭圆内是原图像素，明暗一点不动。
    rel_bbox 是主体在裁切画布坐标系里的边界；因为裁切画布以主体中心为中心，
    椭圆天然落在画面正中。"""
    sx, sy = size[0] / crop_size[0], size[1] / crop_size[1]
    cx = (rel_bbox[0] + rel_bbox[2]) / 2 * sx
    cy = (rel_bbox[1] + rel_bbox[3]) / 2 * sy
    rx = max(1, (rel_bbox[2] - rel_bbox[0]) / 2 * sx * 1.18)
    ry = max(1, (rel_bbox[3] - rel_bbox[1]) / 2 * sy * 1.18)
    mask = Image.new('L', size, 0)
    ImageDraw.Draw(mask).ellipse((cx - rx, cy - ry, cx + rx, cy + ry), fill=255)
    mask = mask.filter(ImageFilter.GaussianBlur(min(size) * 0.035))
    return Image.composite(base, _uniform_background(size), mask)


def normalize_dish(path):
    im = _hide_watermark(Image.open(path).convert('RGB'))
    bg = _corner_color(im)
    x0, y0, x1, y1 = _subject_bbox(im, bg)
    bw, bh = max(1, x1 - x0), max(1, y1 - y0)
    cx, cy = (x0 + x1) / 2, (y0 + y1) / 2

    # 以主体中心裁 4:3，让主体约占 78%、四周留等距边距。
    # 裁切框严格限制在原图内，绝不补色/合成/调亮度：
    # 早期试过的「整图色调平移」造成过曝，「椭圆统一背景」留下圆形光晕，
    # 「纯色画布填充」又拉高整图亮度，全部弃用。背景保持生成图自然桌面。
    cw = max(bw / 0.78, (bh / 0.78) * 4 / 3)
    max_cw = min(im.width, im.height * 4 / 3)
    cw = min(cw, max_cw)
    ch = cw * 3 / 4
    left = max(0, min(im.width - cw, cx - cw / 2))
    top = max(0, min(im.height - ch, cy - ch / 2))
    crop = im.crop((int(left), int(top), int(left + cw), int(top + ch)))
    return crop.resize(OUT_SIZE, Image.LANCZOS)
