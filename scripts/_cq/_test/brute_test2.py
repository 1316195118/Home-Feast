#!/usr/bin/env python3
"""第二轮暴力测试：业务并发、数据一致性、深嵌套、边界体积。"""
import json
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor

sys.stdout.reconfigure(encoding='utf-8')
BASE = 'http://127.0.0.1:18090'
USER = 'ua076525085fddc51'
DISH = 'd7bcdd545f95ec34a'
DATE = '2026-12-01'
rows = []


def req(method, path, body=None, raw=None, timeout=30, headers=None):
    h = dict(headers or {})
    data = None
    if raw is not None:
        data = raw if isinstance(raw, bytes) else raw.encode()
    elif body is not None:
        data = json.dumps(body, ensure_ascii=False).encode()
        h.setdefault('Content-Type', 'application/json')
    r = urllib.request.Request(BASE + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return -1, str(e).encode()


def rec(name, ok, detail):
    rows.append((name, ok, detail))


# ---------- 1. 正常下单 + 重复下单 ----------
c1, b1 = req('POST', f'/api/plans/{DATE}/orders', {'dishId': DISH, 'userId': USER, 'meal': 'lunch'})
c2, b2 = req('POST', f'/api/plans/{DATE}/orders', {'dishId': DISH, 'userId': USER, 'meal': 'lunch'})
c3, b3 = req('GET', f'/api/plans/{DATE}')
orders = json.loads(b3).get('orders', []) if c3 == 200 else []
ids = [o.get('id') for o in orders]
rec('同菜连续下单两次', c1 == 200 and c2 == 200 and len(set(ids)) == len(ids),
    f'HTTP {c1}/{c2}，订单 {len(orders)} 条，唯一 ID {len(set(ids))} 个')

# ---------- 2. 并发下单（30 并发同一菜） ----------
def order_once(_):
    return req('POST', f'/api/plans/{DATE}/orders', {'dishId': DISH, 'userId': USER, 'meal': 'lunch'})[0]

t0 = time.time()
with ThreadPoolExecutor(max_workers=15) as ex:
    codes = list(ex.map(order_once, range(30)))
c, b = req('GET', f'/api/plans/{DATE}')
orders = json.loads(b).get('orders', []) if c == 200 else []
ids = [o.get('id') for o in orders]
rec('30 并发下单', all(x == 200 for x in codes) and len(set(ids)) == len(ids),
    f'成功 {codes.count(200)}/30，订单 {len(ids)} 条，唯一 {len(set(ids))} 个，耗时 {(time.time()-t0)*1000:.0f}ms')

# 下单后 plans.json 是否可解析
try:
    raw = open('scripts/_cq/_test/data/plans.json', encoding='utf-8').read()
    json.loads(raw)
    rec('并发下单后 plans.json 完整', True, f'{len(raw)} 字节')
except Exception as e:
    rec('并发下单后 plans.json 完整', False, f'损坏: {e}')

# ---------- 3. 并发删单 ----------
first_id = ids[0] if ids else 'x'
def del_once(_):
    return req('DELETE', f'/api/plans/{DATE}/orders/{first_id}')[0]

with ThreadPoolExecutor(max_workers=10) as ex:
    dcodes = list(ex.map(del_once, range(10)))
c, b = req('GET', f'/api/plans/{DATE}')
orders2 = json.loads(b).get('orders', []) if c == 200 else []
gone = all(o.get('id') != first_id for o in orders2)
rec('10 并发删同一订单', gone and not any(x >= 500 for x in dcodes),
    f'codes={sorted(set(dcodes))}，该单已消失={gone}')

# ---------- 4. JSON 深嵌套 ----------
deep = {'name': 'deep'}
cur = deep
for _ in range(2000):
    cur['x'] = {}
    cur = cur['x']
c, b = req('POST', '/api/dishes', raw=json.dumps(deep))
rec('2000 层嵌套 JSON', c in (400, 413) and c != -1, f'HTTP {c} {b[:60].decode("utf-8","replace")}')

# ---------- 5. 超长 note（5MB） ----------
c, b = req('PUT', f'/api/plans/{DATE}', {'note': 'N' * (5 * 1024 * 1024)})
rec('5MB 留言', c in (200, 400, 413), f'HTTP {c} {b[:60].decode("utf-8","replace")}')

# ---------- 6. 超长菜品描述（2MB） ----------
c, b = req('POST', '/api/dishes', {'name': '大描述菜', 'cuisine': 'sichuan', 'desc': 'D' * (2 * 1024 * 1024)})
rec('2MB 菜品描述', c in (200, 201, 400, 413), f'HTTP {c}')

# ---------- 7. 删除菜品后订单是否残留 ----------
c, b = req('POST', '/api/dishes', {'name': '临时删除测试菜', 'cuisine': 'sichuan'})
tmp_id = json.loads(b).get('id') if c in (200, 201) else None
if tmp_id:
    req('POST', f'/api/plans/{DATE}/orders', {'dishId': tmp_id, 'userId': USER, 'meal': 'dinner'})
    dc, _ = req('DELETE', f'/api/dishes/{tmp_id}')
    pc, pb = req('GET', f'/api/plans/{DATE}')
    left = [o for o in json.loads(pb).get('orders', []) if o.get('dishId') == tmp_id] if pc == 200 else []
    rec('删除菜品后订单是否残留', len(left) == 0,
        f'删除 HTTP {dc}，残留订单 {len(left)} 条' + ('' if left else '（已级联或前端过滤）'))
else:
    rec('删除菜品后订单是否残留', False, '临时候选菜创建失败')

# ---------- 8. 订单引用不存在菜品 ----------
c, b = req('POST', f'/api/plans/{DATE}/orders', {'dishId': 'ghost_dish_id', 'userId': USER, 'meal': 'lunch'})
rec('给不存在的菜下单', c == 400, f'HTTP {c} {b[:50].decode("utf-8","replace")}')

# ---------- 9. 不存在的成员下单 ----------
c, b = req('POST', f'/api/plans/{DATE}/orders', {'dishId': DISH, 'userId': 'ghost_user', 'meal': 'lunch'})
rec('用不存在的成员下单', c in (400, 200), f'HTTP {c} {b[:50].decode("utf-8","replace")}')

# ---------- 10. 非法餐别 ----------
c, b = req('POST', f'/api/plans/{DATE}/orders', {'dishId': DISH, 'userId': USER, 'meal': 'midnight'})
rec('非法餐别 midnight', c in (400, 200), f'HTTP {c} {b[:50].decode("utf-8","replace")}')

# ---------- 11. 6MB 边界上传 ----------
def upload_bytes(n):
    payload = b'\x89PNG\r\n\x1a\n' + b'0' * (n - 8)
    boundary = '----b' + str(int(time.time() * 1000))
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="x.png"\r\n'
            f'Content-Type: image/png\r\n\r\n').encode() + payload + f'\r\n--{boundary}--\r\n'.encode()
    return req('POST', '/api/upload', raw=body,
               headers={'Content-Type': f'multipart/form-data; boundary={boundary}'}, timeout=60)

for mb, label in [(5, '5MB 图片'), (6, '6MB 图片（边界）')]:
    c, b = upload_bytes(mb * 1024 * 1024)
    rec(f'{label}', c in (200, 201, 400, 413), f'HTTP {c} {b[:50].decode("utf-8","replace")}')

# ---------- 12. Unicode 极端 ----------
long_emoji = '👨‍👩‍👧‍👦' * 200
c, b = req('POST', '/api/dishes', {'name': long_emoji, 'cuisine': 'sichuan'})
rec('800 个组合 emoji 菜名', c in (200, 201, 400), f'HTTP {c}')

# ---------- 13. 幂等/覆盖 ----------
c1, _ = req('PUT', f'/api/plans/{DATE}', {'note': 'A'})
c2, b2 = req('PUT', f'/api/plans/{DATE}', {'note': 'B'})
c3, b3 = req('GET', f'/api/plans/{DATE}')
final = json.loads(b3).get('note') if c3 == 200 else None
rec('连续两次改留言', final == 'B', f'HTTP {c1}/{c2}，最终 note={final!r}')

# ---------- 14. 并发写不同日期（不同文件/键） ----------
def write_date(i):
    d = f'2027-01-{i+1:02d}'
    return req('PUT', f'/api/plans/{d}', {'note': f'日期{i}'})[0]

with ThreadPoolExecutor(max_workers=15) as ex:
    wcodes = list(ex.map(write_date, range(20)))
try:
    raw = open('scripts/_cq/_test/data/plans.json', encoding='utf-8').read()
    plans = json.loads(raw)
    rec('20 个日期并发写', all(c == 200 for c in wcodes) and len(plans) >= 20,
        f'成功 {wcodes.count(200)}/20，plans 共 {len(plans)} 天，文件 {len(raw)} 字节')
except Exception as e:
    rec('20 个日期并发写', False, f'plans.json 损坏: {e}')

# ---------- 15. HEAD / OPTIONS ----------
rec('HEAD /api/ping', req('HEAD', '/api/ping')[0] in (200, 405), f'HTTP {req("HEAD", "/api/ping")[0]}')
rec('OPTIONS /api/dishes', req('OPTIONS', '/api/dishes')[0] in (200, 204, 405), f'HTTP {req("OPTIONS", "/api/dishes")[0]}')

# ---------- 输出 ----------
print('\n' + '=' * 100)
print(f'{"测试项":<32} {"结果":<7} 说明')
print('=' * 100)
fails = 0
for name, ok, detail in rows:
    if not ok:
        fails += 1
    print(f'{name:<32} {"✓ PASS" if ok else "✗ FAIL":<7} {detail[:72]}')
print('=' * 100)
alive = req('GET', '/api/ping')[0] == 200
print(f'共 {len(rows)} 项，失败 {fails} 项；服务存活: {"是" if alive else "否"}')
