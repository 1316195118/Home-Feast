#!/usr/bin/env python3
"""全功能乱字符暴力测试：把一批异常字符串灌进每个可写字段，检查清洗与稳定性。

关注点：
  1. 任何请求都不能让服务 5xx 或崩溃
  2. 控制字符（NUL 等）必须被剥离，换行/制表符保留
  3. 长度必须被截断
  4. 结构化字段（颜色/头像/菜系/餐别）必须走白名单回退
  5. 所有 JSON 数据文件在测试后仍可解析
"""
import json
import os
import sys
import time
import urllib.error
import urllib.request

sys.stdout.reconfigure(encoding='utf-8')
BASE = 'http://127.0.0.1:18090'
DATE = '2026-11-25'
failures = []
stats = {'req': 0, '5xx': 0}


def req(method, path, body=None, timeout=30):
    h = {}
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode()
        h['Content-Type'] = 'application/json'
    r = urllib.request.Request(BASE + path, data=data, headers=h, method=method)
    stats['req'] += 1
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            raw = resp.read()
            return resp.status, (json.loads(raw) if raw else {})
    except urllib.error.HTTPError as e:
        raw = e.read()
        if e.code >= 500:
            stats['5xx'] += 1
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {}
    except Exception as e:
        stats['5xx'] += 1
        return -1, {'error': str(e)}


def alive():
    return req('GET', '/api/ping', timeout=5)[0] == 200


def has_ctrl(s):
    """是否残留控制字符（换行、制表符除外）"""
    if not isinstance(s, str):
        return False
    return any((ord(c) < 0x20 and c not in '\n\t') or ord(c) == 0x7f for c in s)


# ---------------- 乱字符语料 ----------------
PAYLOADS = [
    ('NUL', 'abc\x00def'),
    ('控制字符', 'a\x01\x02\x1f\x7fb'),
    ('零宽字符', 'a\u200b\u200c\u200d\ufeffb'),
    ('RTL 覆盖', 'a\u202eb\u202cc'),
    ('组合符堆叠', 'a' + '\u0300' * 300),
    ('ZWJ emoji', '👨‍👩‍👧‍👦👩‍💻🏳️‍🌈'),
    ('超长中文', '宫保鸡丁' * 2500),
    ('HTML 标签', '<img src=x onerror=alert(1)>'),
    ('脚本闭合', '"><script>alert(1)</script>'),
    ('JS URI', 'javascript:alert(document.cookie)'),
    ('SQL 注入', "'; DROP TABLE dishes; --"),
    ('路径穿越', '../../../../etc/passwd'),
    ('格式化串', '%s%d%n{0}{1}${x}%(a)s'),
    ('Shell 注入', '`id`$(whoami);rm -rf /'),
    ('BOM 前缀', '\ufeffabc'),
    ('纯空白', '   \n\t   '),
    ('CR LF 注入', 'a\r\nX-Injected: 1\r\n\r\n'),
    ('JSON 片段', '{"a":1,"b":[2,3]}'),
    ('CSS 注入', 'red;background:url(//evil.example)'),
    ('emoji 边界', '😀😀😀' + '\U0001F600' + '🏴󠁧󠁢󠁳󠁣󠁴󠁿'),
    ('等号与引号', '====\'\'\'"""'),
    ('反斜杠', '\\\\\\..\\..\\windows\\system32'),
]


def clean_note(name, got, original, maxlen):
    """统一检查：控制字符、长度、是否为空"""
    if has_ctrl(got):
        failures.append(f'{name}: 控制字符残留 {got!r}')
    if len(got) > maxlen * 4:  # 按字符数粗判，留余量
        failures.append(f'{name}: 长度未截断（{len(got)} 字符）')


print('开始全字段乱字符测试…')
start = time.time()

for label, p in PAYLOADS:
    # ---------- 菜品：name / desc / tags / ingredients ----------
    c, d = req('POST', '/api/dishes', {
        'name': p, 'desc': p, 'cuisine': 'sichuan',
        'tags': [p], 'ingredients': [p], 'steps': [p],
    })
    if c >= 500:
        failures.append(f'菜品[{label}] 返回 {c}')
    name = d.get('name', '')
    if not name:
        # 纯空白/纯控制字符被清空后返回 400，属于正确拒绝
        if c not in (400, 422):
            failures.append(f'菜品[{label}] 菜名被清空但状态码 {c}')
    else:
        clean_note(f'菜品名[{label}]', name, p, 40)
        if len(name) > 40:
            failures.append(f'菜品名[{label}] 超 40 字符: {len(name)}')
    if has_ctrl(d.get('desc', '')):
        failures.append(f'菜品描述[{label}] 控制字符残留')
    for t in d.get('tags', []):
        if has_ctrl(t):
            failures.append(f'菜品标签[{label}] 控制字符残留')
        if len(t) > 12:
            failures.append(f'菜品标签[{label}] 超 12 字符')
    for ing in d.get('ingredients', []):
        if has_ctrl(ing):
            failures.append(f'配料[{label}] 控制字符残留')

    # ---------- 成员：name / avatar / color / avoid ----------
    c, m = req('POST', '/api/members', {
        'name': p, 'avatar': p, 'color': p, 'avoid': [p],
    })
    if c >= 500:
        failures.append(f'成员[{label}] 返回 {c}')
    if m:
        if len(m.get('name', '')) > 20:
            failures.append(f'成员名[{label}] 超 20 字符')
        if has_ctrl(m.get('name', '')):
            failures.append(f'成员名[{label}] 控制字符残留')
        if not str(m.get('color', '')).startswith('#') or len(m.get('color', '')) != 7:
            # 名字为空时后端直接 400，没有 color 字段可查
            if m.get('name'):
                failures.append(f'成员颜色[{label}] 未回退: {m.get("color")!r}')
        av = m.get('avatar', '')
        if any(ch in av for ch in '<>{}"\';'):
            failures.append(f'成员头像[{label}] 含危险字符: {av!r}')
        for a in m.get('avoid', []):
            if len(a) > 16:
                failures.append(f'忌口[{label}] 超 16 字符')

    # ---------- 计划留言 ----------
    c, d2 = req('PUT', f'/api/plans/{DATE}', {'note': p})
    if c >= 500:
        failures.append(f'计划留言[{label}] 返回 {c}')

    # ---------- 订单备注 + 餐别 ----------
    c, o = req('POST', f'/api/plans/{DATE}/orders', {
        'dishId': 'd7bcdd545f95ec34a', 'userId': 'ua076525085fddc51',
        'note': p, 'meal': p,
    })
    if c >= 500:
        failures.append(f'订单[{label}] 返回 {c}')

    # ---------- 设置：家庭名 / 冰箱 / 餐别 / 排班 ----------
    c, s = req('PUT', '/api/settings', {
        'familyName': p, 'pantry': [p],
        'meals': [{'key': p, 'label': p, 'icon': p, 'end': p}],
    })
    if c >= 500:
        failures.append(f'设置[{label}] 返回 {c}')

    if not alive():
        failures.append(f'★ 服务在 [{label}] 后失活')
        print(f'  ✗ {label}: 服务失活，中止')
        break

print(f'请求 {stats["req"]} 次，5xx {stats["5xx"]} 次，耗时 {time.time()-start:.1f}s')
print(f'失败项 {len(failures)} 个')
for f in failures[:40]:
    print('  ✗', f)
if not failures:
    print('  ✓ 全部通过')

# ---------------- 数据完整性 ----------------
print('\n数据文件完整性：')
for name in ['dishes.json', 'members.json', 'plans.json']:
    p = os.path.join('scripts/_cq/_test/data', name)
    try:
        arr = json.load(open(p, encoding='utf-8'))
        print(f'  ✓ {name}: {len(arr)} 条')
    except Exception as e:
        print(f'  ✗ {name}: {e}')
for name in ['settings.json']:
    p = os.path.join('scripts/_cq/_test/config', name)
    try:
        json.load(open(p, encoding='utf-8'))
        print(f'  ✓ config/{name}')
    except Exception as e:
        print(f'  ✗ config/{name}: {e}')
print('\n服务存活:', '是' if alive() else '否')
