#!/usr/bin/env python3
# 下载源站全部菜品图片到 scripts/_src_images/<uuid>.webp
import json, os, urllib.request, ssl

BASE = "http://10.10.10.253:3080/api/uploads/"
OUTDIR = r"C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_src_images"
SRC = r"C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_src_dishes.json"
COOKIE = "gancook_session=eyJhbGciOiJIUzI1NiJ9.eyJ1c2VySWQiOiIxMmIxMTZjOS1hZGZhLTQyOTctOGVmOC03YTMwZTdlMjI5ZTgiLCJuYW1lIjoi5b6Q6ICB5aSnIiwiaXNBZG1pbiI6dHJ1ZSwiaWF0IjoxNzkzOTgzNTk5LCJleHAiOjE3OTY1NzU1OTl9.ohFc3RK4b8BtweeHXelSQxHVSiCdITwX-5TJGouQMfg"

os.makedirs(OUTDIR, exist_ok=True)
ctx = ssl.create_default_context()
dishes = json.load(open(SRC, encoding="utf-8"))
ok = 0; fail = []
for d in dishes:
    uid = d["id"]
    ip = d["imagePath"]  # dishes/<uuid>.webp
    url = BASE + ip
    fname = os.path.join(OUTDIR, uid + ".webp")
    if os.path.exists(fname) and os.path.getsize(fname) > 1000:
        ok += 1; continue
    try:
        req = urllib.request.Request(url, headers={"Cookie": COOKIE, "User-Agent": "Mozilla/5.0"})
        data = urllib.request.urlopen(req, timeout=30, context=ctx).read()
        if data[:4] != b"RIFF" or b"WEBP" not in data[:16]:
            fail.append((d["name"], "bad magic")); continue
        open(fname, "wb").write(data)
        ok += 1
    except Exception as e:
        fail.append((d["name"], str(e)[:60]))
print("下载成功:", ok, "/", len(dishes))
print("失败:", fail)
