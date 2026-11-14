#!/usr/bin/env python3
"""按《菜系分类核对清单》的确认结果迁移菜品分类，并补新分类的菜谱数据。

变更：
  A1 蚂蚁上树 → 川渝菜      A2 干煸炒面 → 新疆菜
  B1 木须肉 → 鲁菜          B2 醋溜白菜 → 鲁菜
  B3-B7 五道粤式家常 → 粤菜
  C1-C6 六道点心按原菜系归位（方案 2：离开小吃栏）
  D1 烤肉 → 改名「家常烤肉」留家常 + 新增「新疆烤肉」
  D2 炒鸡 → 改名「山东炒鸡」归鲁菜
  D3 爆炒小龙虾 合并进 口味虾（删除前者）
  E  新增东北菜、本帮菜；地三鲜→东北，葱油拌面/阳春面→本帮
  补 8 道新菜谱数据（新疆烤肉 + 东北 4 + 本帮 3）
"""
import json
import os
import sys
import uuid

sys.stdout.reconfigure(encoding='utf-8')

SRC = 'scripts/_cq/_test/dishes_clean.json'
OUT = 'scripts/_cq/_test/dishes_new.json'

arr = json.load(open(SRC, encoding='utf-8'))
print(f'读入 {len(arr)} 道')

# ---------- 1. 分类迁移 ----------
MOVES = {
    '蚂蚁上树': ('sichuan', None),
    '干煸炒面': ('xinjiang', None),
    '木须肉': ('shandong', None),
    '醋溜白菜': ('shandong', None),
    '清炒菜心': ('cantonese', None),
    '香菇滑鸡': ('cantonese', None),
    '蒜蓉西兰花': ('cantonese', None),
    '清炒荷兰豆': ('cantonese', None),
    '蒜蓉炒大虾': ('cantonese', None),
    '虾饺': ('cantonese', None),
    '豉汁蒸排骨': ('cantonese', None),
    '小笼包': ('jiangsu', None),
    '干炸响铃': ('zhejiang', None),
    '海蛎煎': ('fujian', None),
    '油煎毛豆腐': ('anhui', None),
    '炒鸡': ('shandong', '山东炒鸡'),
    '烤肉': ('homestyle', '家常烤肉'),
    '地三鲜': ('dongbei', None),
    '葱油拌面': ('benbang', None),
    '阳春面': ('benbang', None),
}

CAT_NAME = {
    'sichuan': '川渝菜', 'xinjiang': '新疆菜', 'cantonese': '粤菜', 'shandong': '鲁菜',
    'dongbei': '东北菜', 'jiangsu': '苏菜', 'benbang': '本帮菜', 'zhejiang': '浙菜',
    'fujian': '闽菜', 'hunan': '湘菜', 'anhui': '徽菜', 'homestyle': '家常菜',
    'staple': '主食', 'snack': '小吃',
}

moved = 0
for d in arr:
    if d['name'] in MOVES:
        new_cuisine, new_name = MOVES[d['name']]
        old_c = d['cuisine']
        d['cuisine'] = new_cuisine
        if new_name:
            d['name'] = new_name
        # 标签里的旧菜系词换成新的；「家常」这种泛词也一并替换
        tags = []
        for t in d.get('tags', []):
            if t in ('家常',) or t == CAT_NAME.get(old_c):
                t = CAT_NAME[new_cuisine]
            if t not in tags:
                tags.append(t)
        # 原本没有菜系标签的，补一个，方便按标签筛
        if not any(t == CAT_NAME[new_cuisine] for t in tags):
            tags.insert(0, CAT_NAME[new_cuisine])
        d['tags'] = tags[:5]
        moved += 1
        print(f'  迁移 {d["name"]:<10} {old_c:<10} → {new_cuisine}')
print(f'共迁移 {moved} 道')

# ---------- 2. 合并重复：删除爆炒小龙虾 ----------
before = len(arr)
arr = [d for d in arr if d['name'] != '爆炒小龙虾']
print(f'合并重复：删除「爆炒小龙虾」，{before} → {len(arr)}')


def new_dish(name, cuisine, desc, tags, spicy, minutes, ingredients, steps, image=''):
    return {
        'id': uuid.uuid4().hex[:17],
        'name': name,
        'cuisine': cuisine,
        'desc': desc,
        'tags': tags,
        'image': image,
        'spicy': spicy,
        'minutes': minutes,
        'ingredients': ingredients,
        'steps': steps,
        'favorite': False,
        'builtin': False,
        'createdAt': '2026-11-15T02:55:00+08:00',
    }


NEW = [
    new_dish(
        '新疆烤肉', 'xinjiang',
        '新疆夜市的味道，肥瘦相间串起来烤，孜然一撒满屋香',
        ['新疆菜', '烧烤', '待客'], 2, 45,
        ['羊腿肉 500g', '洋葱 半个', '孜然粉 2 勺', '辣椒面 1 勺', '盐 适量', '食用油 1 勺', '竹签 20 根'],
        ['羊腿肉切 2 厘米见方的块，肥瘦分开切；洋葱切丝',
         '肉块加洋葱丝、盐、孜然粉抓匀，腌 30 分钟',
         '竹签提前泡水 10 分钟防烧焦，肥瘦相间串成串',
         '烤箱 220℃ 预热，肉串架在烤网上烤 8 分钟',
         '翻面刷一层油，撒孜然粉和辣椒面，再烤 5 分钟',
         '边缘微焦、油脂滋响时取出，趁热再撒一次孜然'],
    ),
    new_dish(
        '锅包肉', 'dongbei',
        '东北馆子的招牌，外皮酥脆、酸甜挂汁，一咬嘎嘣响',
        ['东北菜', '酸甜', '硬菜'], 0, 40,
        ['猪里脊 400g', '土豆淀粉 100g', '胡萝卜丝 少许', '葱姜丝 少许',
         '白醋 3 勺', '白糖 3 勺', '盐 少许', '食用油 适量'],
        ['里脊逆纹切 3 毫米厚片，加少许盐抓匀腌 10 分钟',
         '土豆淀粉加水调成稠糊，静置后倒掉表面清水，把肉片裹匀',
         '油温六成热逐片下锅炸至定型捞出；升高油温复炸 30 秒至金黄酥脆',
         '另起锅，白醋、白糖、少许盐调成糖醋汁，小火熬到起小泡',
         '倒入炸好的肉片与葱姜丝、胡萝卜丝，快速翻匀让每片挂上汁',
         '出锅装盘，趁热吃最脆'],
    ),
    new_dish(
        '小鸡炖蘑菇', 'dongbei',
        '东北待客第一菜，榛蘑的菌香全炖进鸡肉里',
        ['东北菜', '炖菜', '硬菜'], 0, 70,
        ['土鸡 半只', '干榛蘑 50g', '粉条 50g', '葱姜 各少许',
         '八角 2 颗', '生抽 3 勺', '老抽 1 勺', '盐 适量'],
        ['干榛蘑用温水泡 30 分钟，反复抓洗掉沙子',
         '鸡剁成块，冷水下锅焯去血沫，捞出沥干',
         '热锅冷油下葱姜、八角炒香，倒入鸡块煸到表面微黄',
         '加生抽、老抽翻炒上色，倒入没过鸡块的热水',
         '大火烧开转小火炖 30 分钟，下榛蘑再炖 20 分钟',
         '放泡软的粉条煮 8 分钟，加盐调味，收汁到浓稠'],
    ),
    new_dish(
        '猪肉炖粉条', 'dongbei',
        '一锅炖到位的东北家常，粉条吸饱肉汤最好吃',
        ['东北菜', '炖菜', '下饭'], 0, 60,
        ['五花肉 400g', '宽粉条 100g', '酸菜 100g（可选）', '葱姜 少许',
         '八角 1 颗', '生抽 2 勺', '盐 适量'],
        ['宽粉条温水泡软；五花肉切厚片',
         '锅中不放油，下五花肉小火煸出油脂至两面微黄',
         '加葱姜、八角炒香，淋生抽炒上色',
         '倒入热水没过肉，大火烧开转小火炖 25 分钟',
         '下粉条（和酸菜）继续炖 15 分钟至粉条透明',
         '加盐调味，汤汁收浓即可出锅'],
    ),
    new_dish(
        '酸菜白肉', 'dongbei',
        '东北冬天的暖锅，酸菜解腻、白肉不腻',
        ['东北菜', '炖菜', '暖身'], 0, 80,
        ['五花肉 400g', '东北酸菜 300g', '葱姜 少许', '八角 1 颗', '盐 适量', '蒜泥 1 碟'],
        ['五花肉整块冷水下锅，加葱姜煮 30 分钟至七成熟，捞出晾凉切薄片',
         '酸菜洗净挤干水分，切成细丝',
         '热锅少油下葱姜、八角炒香，倒入酸菜丝煸炒 3 分钟',
         '倒入煮肉的原汤，大火烧开',
         '铺上白肉片，转小火炖 25 分钟',
         '加盐调味，配蒜泥一起上桌'],
    ),
    new_dish(
        '红烧肉', 'benbang',
        '本帮的浓油赤酱，冰糖炒色、慢火收汁，肥而不腻',
        ['本帮菜', '浓油赤酱', '硬菜'], 0, 90,
        ['五花肉 600g', '冰糖 30g', '生抽 3 勺', '老抽 1 勺',
         '黄酒 3 勺', '葱姜 少许', '八角 1 颗'],
        ['五花肉切 3 厘米方块，冷水下锅焯水去沫，捞出洗净',
         '锅中少油下冰糖，小火炒到枣红色起泡',
         '倒入肉块快速翻炒，让每块都裹上糖色',
         '加葱姜、八角、黄酒、生抽、老抽，倒热水没过肉',
         '大火烧开转小火焖 60 分钟，中途翻动两次',
         '开大火收汁，汤汁浓稠裹住肉块即可'],
    ),
    new_dish(
        '油爆虾', 'benbang',
        '本帮经典，壳脆肉嫩、甜咸带鲜，连壳都能嚼',
        ['本帮菜', '海鲜', '快手'], 0, 25,
        ['河虾 400g', '葱姜 少许', '生抽 1 勺', '白糖 1 勺', '黄酒 1 勺', '食用油 适量'],
        ['河虾剪去虾枪和长须，洗净沥干水分',
         '油温烧到八成热，下虾快速炸 20 秒至壳红捞出',
         '升高油温复炸 10 秒，让壳更脆',
         '另起锅留底油，下葱姜爆香',
         '倒入虾，加生抽、白糖、黄酒，大火快速翻匀',
         '汤汁收紧裹住虾身即可出锅'],
    ),
    new_dish(
        '腌笃鲜', 'benbang',
        '春笋上市才做的一锅，咸肉与鲜肉吊出奶白汤',
        ['本帮菜', '汤菜', '时令'], 0, 100,
        ['咸肉 200g', '鲜五花肉 200g', '春笋 2 根', '百叶结 100g',
         '葱姜 少许', '黄酒 1 勺'],
        ['咸肉提前泡 2 小时去多余盐分，切厚片；鲜肉切块焯水',
         '春笋去壳切滚刀块，焯水 3 分钟去涩',
         '砂锅加足量冷水，下咸肉、鲜肉、葱姜、黄酒',
         '大火烧开撇净浮沫，转小火炖 60 分钟',
         '下春笋和百叶结，继续炖 25 分钟',
         '汤色奶白、笋已入味即可，咸肉够咸一般不用再加盐'],
    ),
]

# 去重：如果已存在同名菜则跳过（防止重复执行）
existing = {d['name'] for d in arr}
added = 0
for d in NEW:
    if d['name'] in existing:
        print(f'  已存在，跳过：{d["name"]}')
        continue
    arr.append(d)
    added += 1
print(f'新增 {added} 道菜谱')

# ---------- 3. 校验与写出 ----------
ids = [d['id'] for d in arr]
assert len(ids) == len(set(ids)), 'ID 重复'
names = [d['name'] for d in arr]
dups = {n for n in names if names.count(n) > 1}
assert not dups, f'菜名重复: {dups}'

import collections
dist = collections.Counter(d['cuisine'] for d in arr)
print(f'\n最终 {len(arr)} 道，分布：')
for k, v in dist.most_common():
    print(f'  {k:<10} {v}')

json.dump(arr, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
open(OUT, 'a', encoding='utf-8').write('\n')
print(f'\n已写出 {OUT}（{os.path.getsize(OUT)} 字节）')
