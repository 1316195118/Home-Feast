# -*- coding: utf-8 -*-
"""样图对比：用同一张新生成的原图，出「现行参数」与「加大参数」两版，
再和线上现有成品并排。只写本目录，不发布、不改线上数据。

加大参数：
  ZOOM 0.69 -> 0.60（保留更少画面 => 菜品主体更大，盘子占比 66% -> 约 77%）
  OUT_SIZE 1600x1200 -> 2400x1800（成品分辨率 +50%）
"""
import glob
import io
import os
import urllib.request

from PIL import Image, ImageDraw, ImageFilter, ImageFont

HERE = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_cq/sample_bigger/'
BASE = 'http://10.10.10.251:8090'
SHIFT_Y = -0.027  # 底图固有俯拍偏移补偿，与 ZOOM 无关，沿用定稿值


def hide_watermark(im):
    """复用 publish_unified.py 的大图抹水印逻辑：从左侧复制同高度木纹盖上。"""
    w, h = im.size
    ow, oh = int(w * 0.18), int(h * 0.11)
    patch = im.crop((w - 2 * ow, h - oh, w - ow, h))
    edge = max(3, min(ow, oh) // 12)
    mask = Image.new('L', (ow, oh), 255)
    mp = mask.load()
    for y in range(oh):
        for x in range(ow):
            d = min(x, y, ow - 1 - x, oh - 1 - y)
            mp[x, y] = int(255 * min(1.0, d / edge))
    mask = mask.filter(ImageFilter.GaussianBlur(2))
    im.paste(patch, (w - ow, h - oh), mask)
    return im


def prepare(im, zoom, out_size):
    im = hide_watermark(im.convert('RGB'))
    w, h = im.size
    if abs(w / h - 4 / 3) > 0.02:
        if w / h > 4 / 3:
            nw = int(h * 4 / 3)
            x0 = (w - nw) // 2
            im = im.crop((x0, 0, x0 + nw, h))
        else:
            nh = int(w * 3 / 4)
            y0 = (h - nh) // 2
            im = im.crop((0, y0, w, y0 + nh))
    zw, zh = int(im.width * zoom), int(im.height * zoom)
    zx = (im.width - zw) // 2
    zy = int((im.height - zh) / 2 + im.height * SHIFT_Y)
    zy = max(0, min(im.height - zh, zy))
    im = im.crop((zx, zy, zx + zw, zy + zh))
    return im.resize(out_size, Image.LANCZOS)


def font(size, bold=False):
    for p in ['C:/Windows/Fonts/msyhbd.ttc' if bold else 'C:/Windows/Fonts/msyh.ttc',
              'C:/Windows/Fonts/simhei.ttf']:
        if os.path.exists(p):
            try:
                return ImageFont.truetype(p, size)
            except Exception:
                pass
    return ImageFont.load_default()


def main():
    # 只认 AI 生成的原图，排除本脚本自己产出的 compare*.png，避免重跑时误用对比拼图
    candidates = [p for p in glob.glob(HERE + '*.png')
                  if not os.path.basename(p).startswith('compare')]
    if not candidates:
        raise SystemExit('找不到 AI 生成原图')
    src = sorted(candidates)[0]
    raw = Image.open(src).convert('RGB')
    print('原图', os.path.basename(src), raw.size)

    cur = prepare(raw.copy(), 0.69, (1600, 1200))
    cur.save(HERE + 'A_current_1600x1200_Z69.webp', 'WEBP', quality=88, method=5)
    print('A 现行', cur.size)

    big = prepare(raw.copy(), 0.60, (2400, 1800))
    big.save(HERE + 'B_bigger_2400x1800_Z60.webp', 'WEBP', quality=88, method=5)
    print('B 加大', big.size)

    # 线上现有成品（仅用于「实际替换效果」那张对比）
    old = Image.open(io.BytesIO(urllib.request.urlopen(
        BASE + '/uploads/img82b383c2e0c4bf89acb4eced.webp', timeout=30).read())).convert('RGB')
    print('线上现有', old.size)

    def side_by_side(left, right, title, subtitle, l1, l2, r1, r2, dst):
        TW, TH = 760, 570
        gap, pad, head = 24, 24, 96
        canvas = Image.new('RGB', (pad * 2 + TW * 2 + gap, head + TH + 76), '#fbf6ee')
        d = ImageDraw.Draw(canvas)
        d.text((pad, 14), title, fill='#3a2e26', font=font(26, True))
        d.text((pad, 52), subtitle, fill='#8a7a6b', font=font(17))
        canvas.paste(left.resize((TW, TH), Image.LANCZOS), (pad, head))
        canvas.paste(right.resize((TW, TH), Image.LANCZOS), (pad + TW + gap, head))
        d.text((pad, head + TH + 12), l1, fill='#3a2e26', font=font(17))
        d.text((pad + TW + gap, head + TH + 12), r1, fill='#c25a3a', font=font(17))
        d.text((pad, head + TH + 44), l2, fill='#8a7a6b', font=font(17))
        d.text((pad + TW + gap, head + TH + 44), r2, fill='#8a7a6b', font=font(17))
        canvas.save(dst)
        print('对比图', dst, canvas.size)

    # 主对比：同一张新生成原图，器皿/光线/色调完全一致，只有裁切与输出尺寸不同
    side_by_side(
        cur, big,
        '效果图「再大一点」参数对比 · 宫保鸡丁',
        '同一张新生成原图 / 同一底图与光线，只改裁切与输出尺寸',
        '左 · 现行 1600×1200 · ZOOM 0.69',
        '盘子约占 66% · 四周留白约 17%',
        '右 · 加大 2400×1800 · ZOOM 0.60',
        '盘子约占 77% · 留白约 12%，盘沿完整',
        HERE + 'compare.png',
    )
    # 参考对比：线上现有成品 vs 加大版（器皿不同属两次生成差异，非参数造成）
    side_by_side(
        old, big,
        '线上现有 vs 加大版（实际替换效果）',
        '器皿颜色不同是两次生成各自的器皿选择，不是裁切参数造成',
        '左 · 线上现有 1600×1200 · 白瓷盘',
        '盘子约占 66% · 四周留白约 17%',
        '右 · 加大版 2400×1800 · 米黄釉陶盘',
        '盘子约占 77% · 留白约 12%，盘沿完整',
        HERE + 'compare_online.png',
    )


if __name__ == '__main__':
    main()
