# -*- coding: utf-8 -*-
"""把 gen2/ 里逐道生成的写实图裁掉水印 -> 4:3 -> WebP -> 上传 -> 更新线上菜品 image。
支持断点续跑：已发布的序号记在 published_all.json，重跑不会重复上传。
只处理 dishes_remaining.json 里的菜品，不碰重庆菜已发布的 52 张。
"""
import glob
import io
import json
import os
import sys
import time
import urllib.request

from PIL import Image
from normalize import normalize_dish

ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
BASE = 'http://10.10.10.251:8090'
GEN = ROOT + 'scripts/_cq/gen2/'
OUT = ROOT + 'scripts/_cq/out2/'
STATE = ROOT + 'scripts/_cq/published_all.json'
ROWS = ROOT + 'scripts/_cq/dishes_remaining.json'
DRY = '--dry' in sys.argv
LIMIT = int(sys.argv[sys.argv.index('--limit') + 1]) if '--limit' in sys.argv else 999


def http(method, path, data=None, ctype=None):
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if ctype:
        req.add_header('Content-Type', ctype)
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read().decode('utf-8'))


def multipart(path, filename):
    boundary = '----fmAllBoundary7f3a'
    with open(path, 'rb') as f:
        payload = f.read()
    body = io.BytesIO()
    body.write(('--%s\r\n' % boundary).encode())
    body.write(('Content-Disposition: form-data; name="file"; filename="%s"\r\n' % filename).encode())
    body.write(b'Content-Type: image/webp\r\n\r\n')
    body.write(payload)
    body.write(('\r\n--%s--\r\n' % boundary).encode())
    return body.getvalue(), 'multipart/form-data; boundary=%s' % boundary


def prepare(idx):
    """统一构图：主体居中、四周等距留边、桌面色调统一，输出 4:3 WebP。"""
    fs = glob.glob(GEN + '%03d/*.png' % idx)
    if not fs:
        return None
    im = normalize_dish(fs[0])
    os.makedirs(OUT, exist_ok=True)
    dst = OUT + '%03d.webp' % idx
    im.save(dst, 'WEBP', quality=86, method=5)
    return dst


def main():
    rows = json.load(open(ROWS, encoding='utf-8'))
    state = json.load(open(STATE, encoding='utf-8')) if os.path.exists(STATE) else {}
    todo = [r for r in rows if str(r['idx']) not in state]
    print('总 %d 道 / 已发布 %d / 待发布 %d / dry=%s' % (len(rows), len(state), len(todo), DRY))
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
