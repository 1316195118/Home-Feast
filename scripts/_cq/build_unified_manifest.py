# -*- coding: utf-8 -*-
"""生成「统一底图」批量图生图清单 unified_manifest.json。

统一的是：浅木纹桌面 / 光位 / 45°俯拍 / 居中构图 / 四周等距留白 / 不过曝。
器皿按菜性给建议，不强求一致（用户明确「盘子不用一样的」）。
"""
import json
import os

ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
ROWS = ROOT + 'scripts/_cq/dishes_remaining.json'
DATA = ROOT + 'scripts/_cq/unified_dishes.json'  # 线上 /api/dishes 缓存，按菜名匹配 desc
BASE_IMG = ROOT + 'scripts/_cq/base_unified/plate.png'
OUT = ROOT + 'scripts/_cq/unified_manifest.json'

CUISINE_STYLE = {
    'sichuan': '川渝菜，麻辣鲜香、红亮油润', 'cantonese': '粤菜，清鲜精致',
    'shandong': '鲁菜，咸鲜醇厚', 'jiangsu': '苏菜，清雅细腻',
    'zhejiang': '浙菜，清爽原味', 'fujian': '闽菜，汤鲜味醇',
    'hunan': '湘菜，香辣浓烈', 'anhui': '徽菜，重油重色',
    'homestyle': '家常菜，朴实家常', 'soup': '汤羹，汤色清亮',
    'staple': '主食，米面朴实', 'cold': '凉菜，清爽开胃',
    'breakfast': '早餐，家常温暖', 'dessert': '甜品饮品，清甜精致',
    'western': '西餐烘焙，暖色调', 'xinjiang': '新疆菜，浓香豪迈',
}


def vessel_of(name, cuisine, idx):
    """按菜性给器皿建议；同一类内用 idx 轮换，避免 105 张全一样。"""
    n = name
    if any(k in n for k in ['汤', '羹', '炖', '煲', '粥', '火锅', '佛跳墙', '一品锅']):
        return ['白瓷汤碗', '陶质汤盅', '黑釉砂锅'][idx % 3]
    if any(k in n for k in ['凉拌', '拍黄瓜', '皮蛋', '卤', '鸡爪', '三丝', '木耳', '藕片']):
        return ['青花小碟', '青瓷浅盘', '白瓷小碟'][idx % 3]
    if any(k in n for k in ['小笼包', '饺', '包子', '蒸', '馍', '烧麦']):
        return ['竹蒸笼', '竹编小屉'][idx % 2]
    if any(k in n for k in ['饭', '面', '米线', '河粉', '意面', '拉条子', '揪片子']):
        return ['白瓷深盘', '粗陶大碗', '青瓷圆碗'][idx % 3]
    if any(k in n for k in ['糕', '奶', '豆沙', '圆子', '酸奶', '绿豆', '银耳', '水果', '粥']):
        return ['青瓷小碗', '玻璃杯', '白瓷小盅'][idx % 3]
    if any(k in n for k in ['披萨', '牛排', '沙拉', '焗', '可颂', '意面']):
        return ['白瓷宽口盘', '木质圆板'][idx % 2]
    if any(k in n for k in ['水煮', '辣子', '麻', '剁椒', '回锅', '毛血旺', '口味虾', '鱼头']):
        return ['黑釉粗陶盘', '粗陶大碗'][idx % 2]
    if any(k in n for k in ['鱼', '鸡', '鸭', '鹅', '排', '肉']) and idx % 4 == 0:
        return '白瓷长盘'
    return ['白瓷圆盘', '米黄釉陶盘', '青瓷圆盘'][idx % 3]


def main():
    rows = json.load(open(ROWS, encoding='utf-8'))
    d = json.load(open(DATA, encoding='utf-8'))
    items = d if isinstance(d, list) else d.get('dishes', [])
    by_name = {x['name']: x for x in items}
    hit = 0
    out = []
    for r in rows:
        src = by_name.get(r['name'], {})
        desc = (src.get('desc') or '').strip()
        if desc:
            hit += 1
        else:
            desc = '%s，菜品形态真实自然、份量适中' % CUISINE_STYLE.get(r['cuisine'], '家常菜')
        vessel = vessel_of(r['name'], r['cuisine'], r['idx'])
        style = CUISINE_STYLE.get(r['cuisine'], '家常菜')
        prompt = (
            '以参考图为底做菜品摄影：严格保留参考图的浅色原木木纹桌面、同一俯拍45度角度、'
            '同一柔和自然侧光与明亮不过曝的色调，以及主体位于画面正中央、四周上下左右等距留白的构图。'
            '画面中央的器皿里盛着一道「%s」（%s）：%s。菜品形态与份量真实自然、色泽鲜亮。'
            '建议器皿：%s（完整可见、居中，可以与参考图的白瓷盘不同）。'
            '菜品必须完全盛在器皿内，不溢出、不散到桌面上、不要出现半截器皿。'
            '真实食物摄影，高细节，柔和景深，画面清爽不过曝，除右下角平台标识外无任何文字、logo。'
        ) % (r['name'], style, desc, vessel)
        out.append({
            'idx': r['idx'], 'dishId': r['dishId'], 'name': r['name'],
            'cuisine': r['cuisine'], 'cuisineName': r['cuisineName'],
            'vessel': vessel, 'prompt': prompt, 'base': BASE_IMG,
            'outdir': ROOT + 'scripts/_cq/unified/%03d/' % r['idx'],
        })
    json.dump(out, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
    print('清单 %d 道，desc 命中 %d 道，输出 %s' % (len(out), hit, OUT))
    for x in out[:3]:
        print('\n%03d %s [%s]\n%s' % (x['idx'], x['name'], x['vessel'], x['prompt'][:220]))


if __name__ == '__main__':
    main()
