# -*- coding: utf-8 -*-
"""统一底图方案：把 unified/NNN/ 内的生成图处理成 4:3 大图，上传并更新菜品 image。

生成图由「一张固定浅木纹桌面底图」图生图得到：背景、光位、角度、居中构图、
四周等距留白全部一致；器皿可按菜性更换，不强求同一只盘。

用法：
  python scripts/_cq/publish_unified.py --only 1     # 只发布一张（样品检查）
  python scripts/_cq/publish_unified.py --dry        # 空跑预览
  python scripts/_cq/publish_unified.py              # 全量（断点续跑）

输出 2400x1800（加大版定稿，比旧的 1600x1200 大 50%），给后续裁切留足余量。
处理只做：抹右下角水印 + 必要时居中裁 4:3 + 等比缩放；
绝不调亮度、不填色、不合成 —— 之前整图色调平移导致过曝，已弃用。
"""
import glob
import io
import json
import os
import sys
import time
import urllib.request

from PIL import Image, ImageFilter

ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
BASE = 'http://10.10.10.251:8090'
GEN = ROOT + 'scripts/_cq/unified/'
OUT = ROOT + 'scripts/_cq/out_unified/'
STATE = ROOT + 'scripts/_cq/published_unified.json'
ROWS = ROOT + 'scripts/_cq/dishes_remaining.json'
OUT_SIZE = (2400, 1800)
# 加大版定稿（用户 2026-11-13 确认）：生成图留白偏多（盘子约占宽 46%），
# 以画面中心保留 60% 再出图，盘子占比升到约 77%，四周留约 12% 等距桌面，
# 盘沿仍完整。105 张共用同一系数，构图保持一致。
# 生成图 2048x1536 裁后约 1229x922，放大到 2400x1800 属插值放大，
# 但原图细节充足、成品经 88 质量 WebP 压缩后观感已验证（见 sample_bigger/）。
ZOOM = 0.60
# 底图俯拍自带约 3.5% 的垂直偏移（上留白少于下留白），中心裁切会把偏差放大。
# 同一底图生成的 105 张偏移方向一致，统一把裁切框上移 2.7% 原图高度即可拉正，
# 使上下留白等宽；数值由成品图白色盘面检测量化得出，不是目测。
SHIFT_Y = -0.027

DRY = '--dry' in sys.argv
ONLY = int(sys.argv[sys.argv.index('--only') + 1]) if '--only' in sys.argv else None
LIMIT = int(sys.argv[sys.argv.index('--limit') + 1]) if '--limit' in sys.argv else 999


def http(method, path, data=None, ctype=None):
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if ctype:
        req.add_header('Content-Type', ctype)
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read().decode('utf-8'))


def multipart(path, filename):
    boundary = '----fmUnifiedBoundary7f3a'
    with open(path, 'rb') as f:
        payload = f.read()
    body = io.BytesIO()
    body.write(('--%s\r\n' % boundary).encode())
    body.write(('Content-Disposition: form-data; name="file"; filename="%s"\r\n' % filename).encode())
    body.write(b'Content-Type: image/webp\r\n\r\n')
    body.write(payload)
    body.write(('\r\n--%s--\r\n' % boundary).encode())
    return body.getvalue(), 'multipart/form-data; boundary=%s' % boundary


def _hide_watermark_big(im):
    """大图专用：水印固定在最右下角的纯桌面区域。
    从紧邻左侧同高度复制一块等大的木纹（水平纹理连续）盖上去，
    四边只羽化 12px，水印所在区域保持 100% 不透明替换 —— 旧版从左上角
    大范围渐入，导致靠近上沿的水印残留。"""
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


def prepare(idx):
    """抹水印 -> 必要时居中裁 4:3 -> 等比缩放到 1600x1200 -> WebP。"""
    fs = glob.glob(GEN + '%03d/*.png' % idx)
    if not fs:
        return None
    im = _hide_watermark_big(Image.open(fs[0]).convert('RGB'))
    w, h = im.size
    ratio = w / h
    # 生成图已是 4:3 时不做任何裁切：构图在生成阶段已经居中、四周留白等距，
    # 再裁一次反而会把盘子推偏或切掉盘沿。
    if abs(ratio - 4 / 3) > 0.02:
        if ratio > 4 / 3:
            nw = int(h * 4 / 3)
            x0 = (w - nw) // 2
            im = im.crop((x0, 0, x0 + nw, h))
        else:
            nh = int(w * 3 / 4)
            y0 = (h - nh) // 2
            im = im.crop((0, y0, w, y0 + nh))
    # 统一中心裁切放大菜品；裁切框严格居中，保证上下左右留白等宽、盘沿不被切
    zw, zh = int(im.width * ZOOM), int(im.height * ZOOM)
    zx = (im.width - zw) // 2
    zy = int((im.height - zh) / 2 + im.height * SHIFT_Y)
    zy = max(0, min(im.height - zh, zy))  # 裁切框始终留在原图内，不补边
    im = im.crop((zx, zy, zx + zw, zy + zh))
    im = im.resize(OUT_SIZE, Image.LANCZOS)
    os.makedirs(OUT, exist_ok=True)
    dst = OUT + '%03d.webp' % idx
    im.save(dst, 'WEBP', quality=88, method=5)
    return dst


def main():
    rows = json.load(open(ROWS, encoding='utf-8'))
    state = json.load(open(STATE, encoding='utf-8')) if os.path.exists(STATE) else {}
    todo = [r for r in rows if str(r['idx']) not in state]
    if ONLY is not None:
        todo = [r for r in rows if r['idx'] == ONLY]
        if ONLY in [int(k) for k in state]:
            print('--only %d 已发布过，强制重新发布覆盖' % ONLY)
    print('总 %d 道 / 已发布 %d / 本次 %d / dry=%s' % (len(rows), len(state), len(todo), DRY))
    n = 0
    for r in todo:
        idx = r['idx']
        p = prepare(idx)
        if not p:
            print('%03d %-10s SKIP（还没有生成图）' % (idx, r['name']))
            continue
        if DRY:
            print('%03d %-10s would upload %s (%d bytes)' % (idx, r['name'], os.path.basename(p), os.path.getsize(p)))
            n += 1
            if n >= LIMIT:
                break
            continue
        try:
            body, ctype = multipart(p, '%03d.webp' % idx)
            up = http('POST', '/api/upload', body, ctype)
            url = up['url']
            http('PUT', '/api/dishes/' + r['dishId'], json.dumps({'image': url}).encode(), 'application/json')
            state[str(idx)] = {'name': r['name'], 'dishId': r['dishId'], 'new': url}
            json.dump(state, open(STATE, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
            print('%03d %-10s OK -> %s' % (idx, r['name'], url))
        except Exception as e:
            print('%03d %-10s FAIL %s' % (idx, r['name'], e))
            break
        n += 1
        if n >= LIMIT:
            break
        time.sleep(0.12)


if __name__ == '__main__':
    main()
