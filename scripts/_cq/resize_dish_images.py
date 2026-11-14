#!/usr/bin/env python3
"""把 174 张 2400x1800 配图压到 1200x900 webp，输出到 resized/ 并写 index.json。"""
import json
import os
import sys
import hashlib
import time

from PIL import Image

sys.stdout.reconfigure(encoding='utf-8')

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '_img')
SRC = os.path.join(ROOT, 'audit')
DST = os.path.join(ROOT, 'resized')
TARGET = (1200, 900)
QUALITY = 82

os.makedirs(DST, exist_ok=True)
rows = json.load(open(os.path.join(ROOT, 'audit_list.json'), encoding='utf-8'))

t0 = time.time()
out, before, after = [], 0, 0
for i, r in enumerate(rows, 1):
    src = os.path.join(SRC, r['image'].rsplit('/', 1)[-1])
    if not os.path.exists(src):
        print('缺源文件:', r['name'], flush=True)
        continue
    before += os.path.getsize(src)
    im = Image.open(src).convert('RGB')
    if im.size != TARGET:
        im = im.resize(TARGET, Image.LANCZOS)
    tmp = os.path.join(DST, '_tmp.webp')
    im.save(tmp, 'WEBP', quality=QUALITY, method=4)
    digest = hashlib.md5(open(tmp, 'rb').read()).hexdigest()[:24]
    name = f'img{digest}.webp'
    os.replace(tmp, os.path.join(DST, name))
    size = os.path.getsize(os.path.join(DST, name))
    after += size
    out.append({'name': r['name'], 'old': r['image'], 'new': f'/uploads/{name}', 'kb': round(size / 1024)})
    if i % 25 == 0:
        print(f'  {i}/{len(rows)} …', flush=True)

json.dump(out, open(os.path.join(DST, 'index.json'), 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
kb = [o['kb'] for o in out]
print(f'完成 {len(out)} 张，耗时 {time.time() - t0:.1f}s')
print(f'总大小 {before/1024/1024:.1f} MB -> {after/1024/1024:.1f} MB（降 {(1-after/before)*100:.0f}%）')
print(f'单张 最小 {min(kb)}KB / 平均 {sum(kb)/len(kb):.0f}KB / 最大 {max(kb)}KB')
