#!/usr/bin/env python3
"""给全量菜谱补「地域标签」，并把彭水小吃归回小吃分类。

规则（用户约定）：
  - 地域标签用纯地名（北京 / 重庆 / 广东 …），与菜系标签（川渝菜 / 粤菜）并存
  - 小吃全部归 snack，不再单独建分类；彭水菜靠「重庆 + 彭水」两个标签筛出来
  - 通用菜（家常汤羹、主食、西餐等）不强行编造归属，宁可不标
"""
import collections
import json
import sys

sys.stdout.reconfigure(encoding='utf-8')

SRC = 'scripts/_cq/_test/dishes_full.json'
OUT = 'scripts/_cq/_test/dishes_regions.json'
arr = json.load(open(SRC, encoding='utf-8'))
print(f'读入 {len(arr)} 道')

# 按菜系给默认地域（逐菜例外在 EXCEPT 里覆盖）
CUISINE_REGION = {
    'xinjiang': '新疆',
    'cantonese': '广东',
    'shandong': '山东',
    'dongbei': '东北',
    'jiangsu': '江苏',
    'benbang': '上海',
    'zhejiang': '浙江',
    'fujian': '福建',
    'hunan': '湖南',
    'anhui': '安徽',
}

# 川渝菜逐个细分（默认四川，重庆起源的单独列出）
SICHUAN_CHONGQING = {
    '水煮鱼', '辣子鸡', '毛血旺', '麻辣香锅', '火锅', '酸菜鱼', '烤脑花', '怪味胡豆',
    '山城小汤圆', '红糖糍粑', '凉虾', '重庆小面', '酸辣粉',
}

# 小吃按实际产地
SNACK_REGION = {
    '重庆小面': '重庆', '酸辣粉': '重庆', '山城小汤圆': '重庆', '红糖糍粑': '重庆',
    '烤脑花': '重庆', '怪味胡豆': '重庆', '凉虾': '重庆',
    '红油抄手': '四川', '冰粉': '四川', '凉糕': '四川', '锅巴土豆': '四川',
    '狼牙土豆': '四川', '钟水饺': '四川', '三大炮': '四川', '蛋烘糕': '四川',
    '驴打滚': '北京', '糖火烧': '北京',
    '煎饼果子': '天津',
    '生煎包': '上海',
    '鸭血粉丝汤': '江苏',
    '定胜糕': '浙江',
    '土笋冻': '福建',
    '肠粉': '广东', '叉烧包': '广东',
    '热干面': '湖北', '三鲜豆皮': '湖北',
    '臭豆腐': '湖南', '糖油粑粑': '湖南',
    '肉夹馍': '陕西', '凉皮': '陕西', '羊肉泡馍': '陕西',
    '兰州牛肉面': '甘肃',
    '过桥米线': '云南',
    '丝娃娃': '贵州',
    '螺蛳粉': '广西',
    '胡辣汤': '河南',
    '烤冷面': '东北',
}

# 零星例外（非小吃类里能确定产地的）
EXCEPT = {
    '手撕茄子': '四川',
    '扬州炒饭': '江苏',
    '双皮奶': '广东',
    '桂花酒酿圆子': '浙江',
    '鸡蛋灌饼': '河南',
    '葱油拌面': '上海',
    '阳春面': '江苏',
    '黄豆焖猪蹄': None,  # 占位，不存在则忽略
}

# 彭水 10 道：重庆 + 彭水 双标签
PENGSHUI = {
    '都卷子', '郁山擀酥饼', '郁山三香', '太子豆花', '涪翁烧白',
    '苗乡米粉', '苗乡口口脆', '绿野鲜鲊', '晶丝山凤汤', '摩围山珍',
}

changed_cuisine = 0
tagged = 0
skipped = []

for d in arr:
    # 1. 彭水小吃归回小吃
    if d['cuisine'] == 'pengshui':
        d['cuisine'] = 'snack'
        changed_cuisine += 1

    # 2. 决定地域标签
    name = d['name']
    regions = []
    if name in PENGSHUI:
        regions = ['重庆', '彭水']
    elif name in SNACK_REGION:
        regions = [SNACK_REGION[name]]
    elif d['cuisine'] == 'sichuan':
        regions = ['重庆'] if name in SICHUAN_CHONGQING else ['四川']
    elif d['cuisine'] in CUISINE_REGION:
        regions = [CUISINE_REGION[d['cuisine']]]
    elif name in EXCEPT and EXCEPT[name]:
        regions = [EXCEPT[name]]
    else:
        skipped.append(name)
        continue

    # 3. 写进标签：去掉旧的「彭水小吃」这种分类名，避免和地域标签混
    tags = [t for t in d.get('tags', []) if t != '彭水小吃']
    for r in regions:
        if r not in tags:
            tags.append(r)
    d['tags'] = tags[:6]
    tagged += 1

print(f'彭水归回小吃：{changed_cuisine} 道')
print(f'加地域标签：{tagged} 道')
print(f'未标地域（通用菜，符合预期）：{len(skipped)} 道')
print('  例：' + '、'.join(skipped[:12]))

dist = collections.Counter(d['cuisine'] for d in arr)
print(f'\n最终 {len(arr)} 道')
for k, v in dist.most_common():
    print(f'  {k:<10} {v}')

# 地域分布
regions = collections.Counter()
for d in arr:
    for t in d['tags']:
        if t in ('北京', '天津', '上海', '重庆', '四川', '广东', '山东', '东北', '江苏',
                 '浙江', '福建', '湖南', '安徽', '湖北', '陕西', '甘肃', '云南', '贵州',
                 '广西', '河南', '新疆', '彭水'):
            regions[t] += 1
print('\n地域标签分布：')
for k, v in regions.most_common():
    print(f'  {k:<6} {v:>3} 道')

ids = [d['id'] for d in arr]
assert len(ids) == len(set(ids))
json.dump(arr, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
open(OUT, 'a', encoding='utf-8').write('\n')
print(f'\n已写出 {OUT}')
