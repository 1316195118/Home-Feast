#!/usr/bin/env python3
"""把「适量 / 少许」换成具体用量，并清理批量生成时误加的配料。

背景：早期新疆菜与川渝小吃是批量导入的，配料尾部统一加了「盐 少许 / 葱花 少许」，
连酸奶粽子、冰粉、凉糕这类甜品都被塞了葱花，需要按菜丢弃。
"""
import json
import sys

sys.stdout.reconfigure(encoding='utf-8')

SRC = 'scripts/_cq/_test/dishes_tagged_all.json'
OUT = 'scripts/_cq/_test/dishes_precise.json'
arr = json.load(open(SRC, encoding='utf-8'))
print(f'读入 {len(arr)} 道')

# 甜品 / 烤饼：不该有葱花，甜的也不该有盐
NO_SCALLION = {
    '馕', '烤包子', '皮芽子馕', '酸奶粽子',
    '冰粉', '凉糕', '凉虾', '红糖糍粑', '山城小汤圆',
    '定胜糕', '郁山擀酥饼', '三大炮', '糖油粑粑',
}
NO_SALT = {'酸奶粽子', '冰粉', '凉糕', '凉虾', '红糖糍粑', '山城小汤圆', '定胜糕'}

# 模糊用量 → 具体数值（按常见家庭份量）
REPLACE = {
    '盐 适量': '盐 3g',
    '盐 少许': '盐 2g',
    '盐 白胡椒 适量': '盐 3g、白胡椒 2g',
    '盐 白胡椒 香菜 适量': '盐 3g、白胡椒 2g、香菜 10g',
    '葱花 少许': '葱花 5g',
    '葱花 香菜 少许': '葱花 5g、香菜 10g',
    '葱花 芹菜末 少许': '葱花 5g、芹菜末 10g',
    '芝麻 葱花 少许': '芝麻 5g、葱花 10g',
    '食用油 适量': '食用油 30ml',
    '菜油 少许': '菜油 10ml',
    '葱姜 少许': '葱 2 段、姜 3 片',
    '葱姜 各少许': '葱 2 段、姜 3 片',
    '葱姜 各适量': '葱 2 段、姜 3 片',
    '葱姜丝 少许': '葱丝 10g、姜丝 5g',
    '葱段 适量': '葱 2 段',
    '姜 葱 各适量': '姜 3 片、葱 2 段',
    '姜 葱 花椒 八角 适量': '姜 3 片、葱 2 段、花椒 3g、八角 2 颗',
    '姜末 少许': '姜末 5g',
    '姜末 蒜末 少许': '姜末 5g、蒜末 5g',
    '姜末 生抽 料酒 各适量': '姜末 10g、生抽 15ml、料酒 15ml',
    '姜丝 蒜末 各适量': '姜丝 10g、蒜末 10g',
    '姜蒜 少许': '姜 8g、蒜 8g',
    '姜蒜 各适量': '姜 10g、蒜 10g',
    '姜汁 少许': '姜汁 5ml',
    '蒜片 少许': '蒜片 10g',
    '淀粉 少许': '淀粉 10g',
    '水淀粉 少许': '水淀粉 15ml',
    '地瓜粉 适量': '地瓜粉 30g',
    '胡萝卜丝 少许': '胡萝卜丝 20g',
    '花椒面 少许': '花椒面 1g',
    '陈皮 少许': '陈皮 2g',
    '红曲粉 少许': '红曲粉 1g',
    '熟芝麻 少许': '熟芝麻 10g',
    '芝麻 少许': '芝麻 10g',
    '洋葱碎 少许': '洋葱碎 30g',
    '虾米 少许': '虾米 10g',
    '冰糖 少许': '冰糖 10g',
    '香醋 少许': '香醋 5ml',
    '紫苏 少许': '紫苏 5g',
    '折耳根 少许': '折耳根 15g',
    '糖蒜 香菜 少许': '糖蒜 3 瓣、香菜 10g',
    '香菜 蒜苗 少许': '香菜 10g、蒜苗 10g',
    '豆芽 韭菜 少许': '豆芽 50g、韭菜 30g',
    '黄瓜丝 胡萝卜丝 豆芽 海带丝 各适量': '黄瓜丝 50g、胡萝卜丝 50g、豆芽 50g、海带丝 50g',
    '酱油 醋 蒜水 适量': '酱油 15ml、醋 15ml、蒜水 10ml',
    '酱油 醋 蒜泥 芥末 适量': '酱油 15ml、醋 15ml、蒜泥 10g、芥末 5g',
    '花生 木耳 酸豆角 各适量': '花生 30g、木耳 20g、酸豆角 30g',
    '海带丝 木耳 各适量': '海带丝 30g、木耳 20g',
    '八角 桂皮 香叶 各适量': '八角 2 颗、桂皮 1 段、香叶 2 片',
    '生抽 老抽 各适量': '生抽 20ml、老抽 10ml',
    '红豆沙 或 肉松 或 榨菜肉末 适量': '红豆沙 30g（或肉松 20g / 榨菜肉末 30g）',
    '白胡椒 适量': '白胡椒 2g',
    '香菜 葱花 少许': '香菜 10g、葱花 5g',
    '蒜末 少许': '蒜末 10g',
}

removed, replaced = 0, 0
for d in arr:
    ings = d.get('ingredients') or []
    if not ings:
        continue
    out = []
    for x in ings:
        # 1. 丢弃误加的配料
        if d['name'] in NO_SCALLION and x in ('葱花 少许', '葱花 5g'):
            removed += 1
            continue
        if d['name'] in NO_SALT and x in ('盐 少许', '盐 2g'):
            removed += 1
            continue
        # 2. 换精确用量
        if x in REPLACE:
            x = REPLACE[x]
            replaced += 1
        out.append(x)
    # 去掉替换后可能出现的重复项
    seen, dedup = set(), []
    for x in out:
        if x not in seen:
            seen.add(x)
            dedup.append(x)
    d['ingredients'] = dedup

print(f'替换模糊用量 {replaced} 处，删除误加配料 {removed} 处')

# 复查
VAGUE = ('适量', '少许', '若干', '随意', '按口味')
left = [(d['name'], [x for x in d['ingredients'] if any(v in x for v in VAGUE)])
        for d in arr if d.get('ingredients')]
left = [x for x in left if x[1]]
print(f'仍有模糊用量 {len(left)} 道：')
for n, b in left:
    print(f'  {n}: {b}')

json.dump(arr, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
open(OUT, 'a', encoding='utf-8').write('\n')
print(f'\n已写出 {OUT}')
