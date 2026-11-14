#!/usr/bin/env python3
"""给所有还没有地区标签的菜补上，保证 237 道 100% 覆盖。

规则：
  - 能确定省份的直接标（鸡爪鸡翅煲/蒜香排骨→广东，酸辣土豆丝/虎皮茄子→四川）
  - 西餐类标「西式」
  - 其余国民家常菜标「家常」（明确表示无特定地域，不硬编省份）
"""
import json
import sys
import collections

sys.stdout.reconfigure(encoding='utf-8')

SRC = 'scripts/_cq/_test/dishes_regions.json'
OUT = 'scripts/_cq/_test/dishes_tagged_all.json'
arr = json.load(open(SRC, encoding='utf-8'))
print(f'读入 {len(arr)} 道')

REGIONS = {'北京', '天津', '上海', '重庆', '四川', '广东', '山东', '东北', '江苏', '浙江',
           '福建', '湖南', '安徽', '湖北', '陕西', '甘肃', '云南', '贵州', '广西', '河南',
           '新疆', '彭水', '家常', '西式'}

# 能判断省份的少数几道
FIX = {
    '鸡爪鸡翅煲': '广东',
    '蒜香排骨': '广东',
    '酸辣土豆丝': '四川',
    '虎皮茄子': '四川',
    '肉末茄子': '四川',
    '青椒炒肉丝': '湖南',
    '爆炒花甲': '广东',
}
# 西式（含早餐/甜品里的西式单品）
WESTERN = {'番茄肉酱意面', '黑椒牛排', '芝士焗饭', '凯撒沙拉', '香烤鸡翅', '玛格丽特披萨',
           '牛奶燕麦杯', '水果酸奶杯'}

filled = collections.Counter()
for d in arr:
    tags = list(d.get('tags') or [])
    if set(tags) & REGIONS:
        continue
    name = d['name']
    if name in FIX:
        region = FIX[name]
    elif name in WESTERN or d['cuisine'] == 'western':
        region = '西式'
    else:
        region = '家常'
    tags.append(region)
    d['tags'] = tags[:6]
    filled[region] += 1

print(f'本次补 {sum(filled.values())} 道：{dict(filled)}')

# 全量校验
missing = [d['name'] for d in arr if not (set(d.get('tags') or []) & REGIONS)]
print('仍无地区标签的:', missing if missing else '无（100% 覆盖）')

dist = collections.Counter()
for d in arr:
    for t in d.get('tags') or []:
        if t in REGIONS:
            dist[t] += 1
print('\n地区标签分布：')
for k, v in dist.most_common():
    print(f'  {k:<6} {v:>3} 道')

ids = [d['id'] for d in arr]
assert len(ids) == len(set(ids))
json.dump(arr, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
open(OUT, 'a', encoding='utf-8').write('\n')
print(f'\n已写出 {OUT}')
