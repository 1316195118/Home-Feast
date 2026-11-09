# -*- coding: utf-8 -*-
"""把生成图裁掉右下角水印 -> 转 WebP -> 上传 -> 更新线上菜品 image。
只处理 chongqing 分类，全部通过应用自身 API 完成，不碰 Go 代码、不重建容器。
支持断点续跑：已处理的序号记录在 published.json 中。
"""
import json, os, io, glob, sys, time, urllib.request, urllib.error
from PIL import Image
from normalize import normalize_dish

BASE = 'http://10.10.10.251:8090'
ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
GEN = ROOT + 'scripts/_cq/gen/'
OUT = ROOT + 'scripts/_cq/out/'
STATE = ROOT + 'scripts/_cq/published.json'
DRY = '--dry' in sys.argv
LIMIT = int(sys.argv[sys.argv.index('--limit') + 1]) if '--limit' in sys.argv else 999


def http(method, path, data=None, ctype=None):
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if ctype:
        req.add_header('Content-Type', ctype)
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read().decode('utf-8'))


def multipart(path, filename):
    boundary = '----fmCQBoundary7f3a'
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
    fs = glob.glob(GEN + '%02d/*.png' % idx)
    if not fs:
        return None
    im = normalize_dish(fs[0])
    os.makedirs(OUT, exist_ok=True)
    dst = OUT + '%02d.webp' % idx
    im.save(dst, 'WEBP', quality=86, method=5)
    return dst


def main():
    rows = json.load(open(ROOT + 'scripts/_cq/manifest.json', encoding='utf-8'))
    state = json.load(open(STATE, encoding='utf-8')) if os.path.exists(STATE) else {}
    todo = [r for r in rows if str(r['idx']) not in state]
    print('total=%d done=%d todo=%d dry=%s' % (len(rows), len(state), len(todo), DRY))
    n = 0
    for r in todo:
        idx = r['idx']
        p = prepare(idx)
        if not p:
            print('%02d %-8s SKIP (无生成图)' % (idx, r['name']))
            continue
        if DRY:
            print('%02d %-8s would upload %s (%d bytes)' % (idx, r['name'], os.path.basename(p), os.path.getsize(p)))
            n += 1
            if n >= LIMIT:
                break
            continue
        try:
            body, ctype = multipart(p, '%02d.webp' % idx)
            up = http('POST', '/api/upload', body, ctype)
            url = up['url']
            http('PUT', '/api/dishes/' + r['id'], json.dumps({'image': url}).encode(), 'application/json')
            state[str(idx)] = {'name': r['name'], 'old': r['oldImage'], 'new': url}
            json.dump(state, open(STATE, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
            print('%02d %-8s OK -> %s' % (idx, r['name'], url))
        except Exception as e:
            print('%02d %-8s FAIL %s' % (idx, r['name'], e))
            break
        n += 1
        if n >= LIMIT:
            break
        time.sleep(0.15)


if __name__ == '__main__':
    main()
