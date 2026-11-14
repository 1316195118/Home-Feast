#!/usr/bin/env python3
"""对本地测试实例做暴力测试。全部写操作都打在副本数据上，不影响线上。

用法：python scripts/_cq/_test/brute_test.py [base_url]
"""
import json
import os
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor

sys.stdout.reconfigure(encoding='utf-8')

BASE = sys.argv[1] if len(sys.argv) > 1 else 'http://127.0.0.1:18090'
results = []


def req(method, path, body=None, headers=None, timeout=20, raw_body=None, origin=None):
    url = BASE + path
    h = dict(headers or {})
    data = None
    if raw_body is not None:
        data = raw_body if isinstance(raw_body, bytes) else raw_body.encode()
    elif body is not None:
        data = json.dumps(body, ensure_ascii=False).encode()
        h.setdefault('Content-Type', 'application/json')
    if origin:
        h['Origin'] = origin
    r = urllib.request.Request(url, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return -1, str(e).encode()


def check(name, fn, expect):
    """expect: 可接受的状态码集合"""
    t0 = time.time()
    try:
        code, body = fn()
        ok = code in expect
        snippet = body[:120].decode('utf-8', 'replace').replace('\n', ' ')
        results.append((name, ok, code, snippet, (time.time() - t0) * 1000))
    except Exception as e:
        results.append((name, False, -1, f'异常 {e}', (time.time() - t0) * 1000))


def alive():
    c, _ = req('GET', '/api/ping', timeout=5)
    return c == 200


# ---------------- A. 输入边界 ----------------
check('超长菜名 10000 字', lambda: req('POST', '/api/dishes',
      {'name': 'A' * 10000, 'cuisine': 'sichuan'}), {400, 413, 500})
check('空菜名', lambda: req('POST', '/api/dishes', {'name': '', 'cuisine': 'sichuan'}), {400})
check('纯空格菜名', lambda: req('POST', '/api/dishes', {'name': '    ', 'cuisine': 'sichuan'}), {400})
check('XSS 菜名原样存储', lambda: req('POST', '/api/dishes',
      {'name': '<script>alert(1)</script>', 'cuisine': 'sichuan'}), {200, 201})
check('SQL 风格注入串', lambda: req('POST', '/api/dishes',
      {'name': "'; DROP TABLE dishes; --", 'cuisine': 'sichuan'}), {200, 201, 400})
check('NUL 字节菜名', lambda: req('POST', '/api/dishes',
      {'name': 'abc\x00def', 'cuisine': 'sichuan'}), {200, 400, 500})
check('不存在的菜系', lambda: req('POST', '/api/dishes',
      {'name': '测试菜', 'cuisine': 'no_such_cuisine'}), {400, 200, 201})
check('超大配料数组 5000 项', lambda: req('POST', '/api/dishes',
      {'name': '巨型配料菜', 'cuisine': 'sichuan',
       'ingredients': [f'食材{i} 10g' for i in range(5000)]}), {400, 413, 200, 201})
check('负数用量', lambda: req('POST', '/api/dishes',
      {'name': '负数菜', 'cuisine': 'sichuan', 'ingredients': ['盐 -5g']}), {200, 201, 400})

# ---------------- B. 协议层 ----------------
check('畸形 JSON（截断）', lambda: req('POST', '/api/dishes', raw_body='{"name":'), {400})
check('JSON 是数组不是对象', lambda: req('POST', '/api/dishes', raw_body='[1,2,3]'), {400, 500})
check('错误 Content-Type', lambda: req('POST', '/api/dishes', raw_body='name=x',
      headers={'Content-Type': 'text/plain'}), {400, 415, 500})
check('10MB JSON body', lambda: req('POST', '/api/dishes',
      raw_body=json.dumps({'name': 'x' * (10 * 1024 * 1024)}).encode()), {400, 413, 500})
check('PATCH 方法', lambda: req('PATCH', '/api/dishes/d7bcdd545f95ec34a', {'name': 'x'}), {405})
check('TRACE 方法', lambda: req('TRACE', '/api/dishes'), {405, 400})
check('空 body POST', lambda: req('POST', '/api/dishes', raw_body=b''), {400})
check('路径穿越 /uploads/../config', lambda: req('GET', '/uploads/../config/settings.json'), {404, 400, 301, 403})
check('路径穿越编码 ../', lambda: req('GET', '/uploads/..%2fconfig%2fsettings.json'), {404, 400, 403})
check('路径穿越 %00', lambda: req('GET', '/uploads/x.webp%00.txt'), {404, 400})
check('更新不存在的菜品', lambda: req('PUT', '/api/dishes/nonexistent', {'name': 'x'}), {404})
check('删除不存在的成员', lambda: req('DELETE', '/api/members/nonexistent'), {404})
check('非法日期 2026-13-45', lambda: req('PUT', '/api/plans/2026-13-45', {'note': 'x'}), {400, 404})
check('日期 2026-02-30', lambda: req('PUT', '/api/plans/2026-02-30', {'note': 'x'}), {400, 404, 200})
check('日期 abc', lambda: req('GET', '/api/plans/abc'), {400, 404})
check('超长 URL 路径', lambda: req('GET', '/api/dishes/' + 'a' * 8000), {404, 414, 400})

# ---------------- C. 上传 ----------------
def upload(filename, content, ctype='application/octet-stream'):
    boundary = '----brute' + str(int(time.time() * 1000))
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\n'
            f'Content-Type: {ctype}\r\n\r\n').encode() + content + f'\r\n--{boundary}--\r\n'.encode()
    return req('POST', '/api/upload', raw_body=body,
               headers={'Content-Type': f'multipart/form-data; boundary={boundary}'})


check('上传纯文本伪装图片', lambda: upload('x.png', b'hello world not an image'), {400})
check('上传空文件', lambda: upload('empty.png', b''), {400})
check('上传 7MB 超限', lambda: upload('big.png', b'\x89PNG\r\n\x1a\n' + b'0' * (7 * 1024 * 1024)), {400, 413})
check('上传路径穿越文件名', lambda: upload('../../evil.png', b'\x89PNG\r\n\x1a\n' + b'0' * 100), {400, 200})

# ---------------- D. 并发 ----------------
def burst_read(n=50):
    def one(_):
        return req('GET', '/api/bootstrap', timeout=30)[0]
    with ThreadPoolExecutor(max_workers=20) as ex:
        codes = list(ex.map(one, range(n)))
    return codes


def concurrent_write(n=20):
    """并发更新同一道菜，检查最终 JSON 未损坏、响应不是 5xx"""
    dish_id = 'd7bcdd545f95ec34a'  # 宫保鸡丁
    def one(i):
        return req('PUT', f'/api/dishes/{dish_id}', {'desc': f'并发描述{i}'}, timeout=30)[0]
    with ThreadPoolExecutor(max_workers=10) as ex:
        codes = list(ex.map(one, range(n)))
    return codes


t0 = time.time()
codes = burst_read(50)
ok_read = sum(1 for c in codes if c == 200)
results.append(('50 并发读 bootstrap', ok_read == 50, f'{ok_read}/50 成功',
                f'耗时 {(time.time()-t0)*1000:.0f}ms', (time.time() - t0) * 1000))

t0 = time.time()
codes = concurrent_write(20)
bad = [c for c in codes if c >= 500]
results.append(('20 并发写同一道菜', len(bad) == 0, f'5xx={len(bad)}, codes={sorted(set(codes))}',
                f'耗时 {(time.time()-t0)*1000:.0f}ms', (time.time() - t0) * 1000))

# ---------------- E. 跨站 ----------------
check('跨站 Origin 写请求', lambda: req('POST', '/api/dishes',
      {'name': '跨站菜', 'cuisine': 'sichuan'}, origin='http://evil.example.com'), {403})
check('同站 Origin 写请求', lambda: req('POST', '/api/dishes',
      {'name': '同站测试菜', 'cuisine': 'sichuan'}, origin=BASE), {200, 201})

# ---------------- F. 业务一致性 ----------------
def order_twice():
    body = {'dishId': 'd7bcdd545f95ec34a', 'dishName': '宫保鸡丁', 'meal': 'lunch'}
    c1, _ = req('POST', '/api/plans/2026-11-20/orders', body)
    c2, _ = req('POST', '/api/plans/2026-11-20/orders', body)
    c3, payload = req('GET', '/api/plans/2026-11-20')
    orders = json.loads(payload).get('orders', []) if c3 == 200 else []
    same = [o for o in orders if o.get('dishId') == 'd7bcdd545f95ec34a']
    ids = {o.get('id') for o in same}
    return c1, c2, len(same), len(ids)


try:
    c1, c2, cnt, uniq = order_twice()
    passed = cnt == uniq and cnt >= 1  # 订单 ID 不重复
    results.append(('重复下单（同菜点两次）', passed,
                    f'HTTP {c1}/{c2} 产生 {cnt} 条订单、{uniq} 个唯一 ID',
                    '允许重复下单（产品行为）' if cnt == 2 else '自动去重', 0))
except Exception as e:
    results.append(('重复下单（同菜点两次）', False, f'异常 {e}', '', 0))

# ---------------- G. 数据完整性 ----------------
try:
    raw = open(os.path.join(os.path.dirname(__file__), 'data', 'dishes.json'), encoding='utf-8').read()
    arr = json.loads(raw)
    results.append(('并发写后 dishes.json 可解析', True, f'{len(arr)} 道菜，{len(raw)} 字节', '', 0))
except Exception as e:
    results.append(('并发写后 dishes.json 可解析', False, f'解析失败: {e}', '', 0))

try:
    raw = open(os.path.join(os.path.dirname(__file__), 'data', 'plans.json'), encoding='utf-8').read()
    json.loads(raw)
    results.append(('并发写后 plans.json 可解析', True, f'{len(raw)} 字节', '', 0))
except Exception as e:
    results.append(('并发写后 plans.json 可解析', False, f'解析失败: {e}', '', 0))

# ---------------- 输出 ----------------
print('\n' + '=' * 96)
print(f'{"测试项":<30} {"结果":<6} {"码":>5}  说明')
print('=' * 96)
fails = 0
for name, ok, code, snippet, ms in results:
    flag = '✓ PASS' if ok else '✗ FAIL'
    if not ok:
        fails += 1
    print(f'{name:<30} {flag:<7} {str(code):>4}  {snippet[:70]}')
print('=' * 96)
print(f'共 {len(results)} 项，失败 {fails} 项；服务存活: {"是" if alive() else "否"}')
