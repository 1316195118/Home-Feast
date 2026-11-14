#!/usr/bin/env python3
"""把本地菜品图转成 2400x1800 webp 并入库到线上 family-menu 容器。

用法：
  python scripts/_cq/import_dish_image.py --image <本地图片> --dish "大盘鸡" [--dry]

流程：PIL 居中裁切 4:3 → 缩放 2400x1800 → webp(q=86) → SFTP 到 PVE →
pct push 到 LXC100 → docker cp 进容器 /data/uploads/ → 更新 dishes.json 的 image 字段（原子替换）。

依赖：paramiko、Pillow
"""
import argparse
import hashlib
import io
import json
import os
import posixpath
import sys
import time

import paramiko
from PIL import Image

PVE_HOST = '10.10.10.252'
PVE_USER = 'root'
PVE_PASS = 'admin@123'
CT = '100'
CONTAINER = 'family-menu'
TARGET_SIZE = (2400, 1800)
QUALITY = 86


def log(msg):
    print(msg, flush=True)


def ssh_connect():
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(PVE_HOST, 22, PVE_USER, PVE_PASS, look_for_keys=False, allow_agent=False, timeout=20)
    return c


def run(ssh, cmd, timeout=120):
    _, out, err = ssh.exec_command(cmd, timeout=timeout)
    o = out.read().decode('utf-8', 'replace').strip()
    e = err.read().decode('utf-8', 'replace').strip()
    return o, e


def to_webp(src, dst, crop_bottom=0):
    """裁掉底部水印 → 居中裁成 4:3 → 缩放到 2400x1800 → 输出 webp。"""
    im = Image.open(src).convert('RGB')
    if crop_bottom > 0:
        w, h = im.size
        im = im.crop((0, 0, w, h - crop_bottom))
    w, h = im.size
    target_ratio = TARGET_SIZE[0] / TARGET_SIZE[1]
    ratio = w / h
    if ratio > target_ratio:  # 太宽 → 裁左右
        new_w = int(h * target_ratio)
        left = (w - new_w) // 2
        im = im.crop((left, 0, left + new_w, h))
    elif ratio < target_ratio:  # 太高 → 裁上下
        new_h = int(w / target_ratio)
        top = (h - new_h) // 2
        im = im.crop((0, top, w, top + new_h))
    im = im.resize(TARGET_SIZE, Image.LANCZOS)
    im.save(dst, 'WEBP', quality=QUALITY, method=6)
    return dst


def import_one(image_path, dish_name, dry=False, crop_bottom=0):
    if not os.path.isfile(image_path):
        raise SystemExit(f'图片不存在: {image_path}')

    tmp_webp = os.path.join(os.path.dirname(os.path.abspath(image_path)), '_tmp_import.webp')
    to_webp(image_path, tmp_webp, crop_bottom)
    size = os.path.getsize(tmp_webp)
    digest = hashlib.md5(open(tmp_webp, 'rb').read()).hexdigest()
    remote_name = f'img{digest[:24]}.webp'
    remote_url = f'/uploads/{remote_name}'
    log(f'预处理: {os.path.basename(image_path)} -> {remote_name} ({size/1024:.0f} KB, {TARGET_SIZE[0]}x{TARGET_SIZE[1]})')

    if dry:
        log('[dry] 跳过上传')
        return remote_url

    ssh = ssh_connect()
    try:
        # 1) 传图并放进容器
        sftp = ssh.open_sftp()
        sftp.put(tmp_webp, f'/tmp/{remote_name}')
        sftp.close()
        o, e = run(ssh, f'pct push {CT} /tmp/{remote_name} /tmp/{remote_name}')
        if e and 'error' in e.lower():
            raise RuntimeError(f'pct push 失败: {e}')
        o, e = run(ssh, f'pct exec {CT} -- docker cp /tmp/{remote_name} {CONTAINER}:/data/uploads/{remote_name}')
        o, e = run(ssh, f'pct exec {CT} -- sh -c "rm -f /tmp/{remote_name}"')
        o, e = run(ssh, f'rm -f /tmp/{remote_name}')
        log(f'  上传完成: {remote_url}')

        # 2) 更新 dishes.json
        raw, _ = run(ssh, f'pct exec {CT} -- docker exec {CONTAINER} cat /data/dishes.json', timeout=180)
        arr = json.loads(raw)
        hit = [d for d in arr if d.get('name') == dish_name]
        if not hit:
            raise RuntimeError(f'线上没有这道菜: {dish_name}')
        for d in hit:
            d['image'] = remote_url
        payload = json.dumps(arr, ensure_ascii=False, indent=2) + '\n'
        chk = json.loads(payload)
        assert len(chk) == len(arr)

        local_json = os.path.join(os.path.dirname(os.path.abspath(image_path)), '_tmp_dishes.json')
        open(local_json, 'w', encoding='utf-8').write(payload)
        sftp = ssh.open_sftp()
        sftp.put(local_json, '/tmp/_dishes_patch.json')
        sftp.close()
        run(ssh, f'pct push {CT} /tmp/_dishes_patch.json /tmp/_dishes_patch.json')
        run(ssh, f'pct exec {CT} -- docker cp /tmp/_dishes_patch.json {CONTAINER}:/data/_dishes_patch.json')
        o, e = run(ssh, f'pct exec {CT} -- docker exec {CONTAINER} sh -c '
                        f'"mv /data/_dishes_patch.json /data/dishes.json && wc -c /data/dishes.json"')
        log(f'  dishes.json 已更新: {o}')
        run(ssh, f'rm -f /tmp/_dishes_patch.json')
        run(ssh, f'pct exec {CT} -- sh -c "rm -f /tmp/_dishes_patch.json"')
        return remote_url
    finally:
        ssh.close()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--image', required=True)
    ap.add_argument('--dish', required=True)
    ap.add_argument('--dry', action='store_true')
    ap.add_argument('--crop-bottom', type=int, default=0, help='先裁掉图片底部若干像素（去 AI 生成水印）')
    a = ap.parse_args()
    import_one(a.image, a.dish, a.dry, a.crop_bottom)


if __name__ == '__main__':
    main()
