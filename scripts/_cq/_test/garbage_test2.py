#!/usr/bin/env python3
"""补充暴力测试：数值边界、结构化字段自由度、Excel/备份往返。"""
import io
import json
import sys
import urllib.error
import urllib.request
import zipfile

sys.stdout.reconfigure(encoding='utf-8')
BASE = 'http://127.0.0.1:18090'
probs = []


def req(method, path, body=None, timeout=60, raw=False):
    h = {}
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode()
        h['Content-Type'] = 'application/json'
    r = urllib.request.Request(BASE + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            b = resp.read()
            return resp.status, (b if raw else json.loads(b) if b else {})
    except urllib.error.HTTPError as e:
        b = e.read()
        try:
            return e.code, (b if raw else json.loads(b))
        except Exception:
            return e.code, b
    except Exception as e:
        return -1, str(e)


def alive():
    return req('GET', '/api/ping', timeout=5)[0] == 200


# ---------- 1. 数值边界 ----------
print('1) 数值边界（spicy / minutes / backupKeep）')
for label, body in [
    ('spicy=-1', {'name': '数值A', 'cuisine': 'sichuan', 'spicy': -1}),
    ('spicy=999', {'name': '数值B', 'cuisine': 'sichuan', 'spicy': 999}),
    ('spicy=NaN 字符串', {'name': '数值C', 'cuisine': 'sichuan', 'spicy': 'NaN'}),
    ('minutes=1e9', {'name': '数值D', 'cuisine': 'sichuan', 'minutes': 1000000000}),
    ('minutes=-5', {'name': '数值E', 'cuisine': 'sichuan', 'minutes': -5}),
    ('spicy=1.5 浮点', {'name': '数值F', 'cuisine': 'sichuan', 'spicy': 1.5}),
]:
    c, d = req('POST', '/api/dishes', body)
    if c >= 500:
        probs.append(f'{label} 返回 {c}')
    if c in (200, 201):
        sp, mi = d.get('spicy'), d.get('minutes')
        ok = (sp is None or 0 <= sp <= 3) and (mi is None or 0 <= mi <= 600)
        print(f'  {label:<18} HTTP {c} spicy={sp} minutes={mi} {"✓" if ok else "✗ 未夹紧"}')
        if not ok:
            probs.append(f'{label} 数值未夹紧 spicy={sp} minutes={mi}')
    else:
        print(f'  {label:<18} HTTP {c}（拒绝）')

for v in [-1, 0, 1, 365, 999999]:
    c, d = req('PUT', '/api/settings', {'backupKeep': v})
    print(f'  backupKeep={v:<7} HTTP {c} 实际={d.get("backupKeep")}')

# ---------- 2. 餐别 key 自由度 ----------
print('\n2) 自定义餐别（key 是自由字符串，最容易漏校验）')
c, d = req('PUT', '/api/settings', {'meals': [
    {'key': '<script>', 'label': '<b>x</b>', 'icon': '"><img>', 'end': '08:00'},
    {'key': 'a b"c', 'label': '正常', 'icon': '🍚', 'end': '12:00'},
    {'key': '', 'label': '空key', 'icon': '🚫', 'end': '13:00'},
    {'key': 'ok', 'label': '坏时间', 'icon': '⏰', 'end': '25:99'},
]})
print(f'  写入 HTTP {c}，返回 {len(d.get("meals") or [])} 个餐别: '
      f'{[(m.get("key"), m.get("end")) for m in (d.get("meals") or [])]}')
if c >= 500:
    probs.append('自定义餐别 5xx')

# 恢复默认餐别，避免影响后续
req('PUT', '/api/settings', {'meals': [
    {'key': 'breakfast', 'label': '早餐', 'icon': '🌅', 'end': '10:00'},
    {'key': 'lunch', 'label': '午餐', 'icon': '☀️', 'end': '15:00'},
    {'key': 'dinner', 'label': '晚餐', 'icon': '🌙', 'end': '23:00'},
]})

# ---------- 3. 排班字段 ----------
print('\n3) 排班字段乱字符')
c, d = req('PUT', '/api/settings', {'roster': {
    'enabled': True,
    'chef': {'mon': '<script>', 'tue': '../../x', 'wed': 'ok'},
    'wash': {'mon': '"\'; DROP', 'tue': '👨‍👩‍👧‍👦', 'wed': '  '},
}})
print(f'  HTTP {c}，roster.enabled={ (d.get("roster") or {}).get("enabled") }')
if c >= 500:
    probs.append('排班写入 5xx')

# ---------- 4. 冰箱食材数量 ----------
print('\n4) 冰箱食材 500 项')
c, d = req('PUT', '/api/settings', {'pantry': [f'食材{i}\x00' for i in range(500)]})
pantry = d.get('pantry') or []
print(f'  HTTP {c}，实际存 {len(pantry)} 项，含控制字符: {any("\x00" in x for x in pantry)}')
if len(pantry) > 200:
    probs.append(f'pantry 未限数量: {len(pantry)}')
if any('\x00' in x for x in pantry):
    probs.append('pantry 控制字符未清洗')
req('PUT', '/api/settings', {'pantry': []})

# ---------- 5. Excel 导出 ----------
print('\n5) Excel 导出（数据里已有大量乱字符）')
c, blob = req('GET', '/api/export/excel', raw=True)
if c == 200 and isinstance(blob, bytes):
    try:
        z = zipfile.ZipFile(io.BytesIO(blob))
        bad = z.testzip()
        names = z.namelist()[:3]
        print(f'  ✓ HTTP 200，{len(blob)/1024:.0f} KB，zip 完整性 {"OK" if bad is None else bad}，含 {names}')
    except Exception as e:
        probs.append(f'Excel 导出文件损坏: {e}')
        print(f'  ✗ 无法解析: {e}')
else:
    probs.append(f'Excel 导出 HTTP {c}')

# ---------- 6. 备份导出 → 再导入 ----------
print('\n6) 备份导出 / 导入往返')
c, zblob = req('GET', '/api/backup', raw=True)
if c == 200 and isinstance(zblob, bytes):
    z = zipfile.ZipFile(io.BytesIO(zblob))
    print(f'  导出 HTTP 200，{len(zblob)/1024:.0f} KB，{len(z.namelist())} 个条目，完整性 {"OK" if z.testzip() is None else "损坏"}')

    boundary = '----b' + str(int(__import__('time').time() * 1000))
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="b.zip"\r\n'
            f'Content-Type: application/zip\r\n\r\n').encode() + zblob + f'\r\n--{boundary}--\r\n'.encode()
    r = urllib.request.Request(BASE + '/api/backup', data=body, method='POST',
                               headers={'Content-Type': f'multipart/form-data; boundary={boundary}'})
    try:
        with urllib.request.urlopen(r, timeout=90) as resp:
            print(f'  导入回写: HTTP {resp.status} {resp.read()[:80].decode("utf-8","replace")}')
    except urllib.error.HTTPError as e:
        probs.append(f'备份导入失败 HTTP {e.code}')
        print(f'  ✗ 导入 HTTP {e.code}: {e.read()[:100]}')
    except Exception as e:
        probs.append(f'备份导入异常 {e}')
else:
    probs.append(f'备份导出 HTTP {c}')

# ---------- 7. 导入损坏 zip ----------
print('\n7) 导入损坏 / 伪造 zip')
for label, content in [('纯文本', b'not a zip'), ('空文件', b''), ('截断 zip', zblob[:200] if isinstance(zblob, bytes) else b'x')]:
    boundary = '----c' + str(int(__import__('time').time() * 1000))
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="bad.zip"\r\n'
            f'Content-Type: application/zip\r\n\r\n').encode() + content + f'\r\n--{boundary}--\r\n'.encode()
    r = urllib.request.Request(BASE + '/api/backup', data=body, method='POST',
                               headers={'Content-Type': f'multipart/form-data; boundary={boundary}'})
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            print(f'  {label}: HTTP {resp.status}（意外成功）')
            probs.append(f'损坏备份 {label} 被接受')
    except urllib.error.HTTPError as e:
        print(f'  {label}: HTTP {e.code} ✓')
    except Exception as e:
        probs.append(f'损坏备份 {label} 异常 {e}')

print('\n服务存活:', '是' if alive() else '否')
print(f'问题 {len(probs)} 个')
for p in probs:
    print('  ✗', p)
if not probs:
    print('  ✓ 无问题')
