# -*- coding: utf-8 -*-
"""从 HowToCook (Unlicense) 菜谱 md 的「## 计算」段解析精确用量。
只取 28 道与本地同名的菜，输出 web/src/howto-amounts.json，供 getRecipe() 叠加。
不改动用户数据 data/dishes.json，不引入任何图片。
"""
import os, re, json, glob

SRC = 'C:/Users/Administrator/AppData/Local/Temp/howto-md/'
OUT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/web/src/howto-amounts.json'

# 用量单位：优先长的，避免 g 抢先匹配 kg
UNIT = r'(?:kg|克|毫升|ml|升|L|斤|两|勺|小勺|大勺|汤匙|茶匙|杯|碗|个|只|条|根|片|瓣|块|棵|颗|粒|把|支|袋|盒|头|朵|尾|张|串|份|滴|撮| handful)'
NUM = r'[0-9０-９]+(?:\.[0-9０-９]+)?'


def norm_num(s):
    return s.translate(str.maketrans('０１２３４５６７８９．', '0123456789.'))


def clean_ing(s):
    """去掉括号注释、备注、可选标记，保留纯食材名"""
    s = re.sub(r'[（(][^）)]*[）)]', '', s)
    s = re.sub(r'[、,，/／].*$', '', s)  # 「干木耳: 20g / 湿木耳:120g」只留第一项
    # 食材名后面跟的描述（「蔬菜 可有不同搭配」「大葱葱白 1 根…」）只留名字
    s = re.sub(r'\s+(可有不同搭配|推荐合计|约\s|[0-9０-９]).*$', '', s)
    s = re.sub(r'(根据|依个人|建议|可选|按需|少许|适量).*$', '', s)
    s = re.sub(r'[\s\*注：:：]+$', '', s)
    return s.strip(' :：*-')


def clean_amount(amt):
    """清洗用量文字：去描述性括号、去中文引导语、规整数字。"""
    amt = amt.strip()
    # 逐个括号处理：括号内去掉数字/单位/约/至/范围符号后若还剩中文，说明是描述（如「约 1/2 中等大小的西兰花」「不怕辣想多加」），整段丢弃；
    # 若是纯计量（如「约 350g」「约 3 斤」），则去掉括号保留内容作为更精确的量。
    def fix_paren(m):
        inner = m.group(1)
        residue = re.sub(NUM + r'|' + UNIT + r'|约|至|大|小|半|多|[0-9/／\-~\s．。克毫升]', '', inner)
        return inner if not re.search(r'[\u4e00-\u9fff]', residue) else ''
    amt = re.sub(r'[（(]([^）)]*)[）)]', fix_paren, amt)
    # 去掉「可有不同搭配，推荐合计重量」这类中文引导语，保留其后的数字范围
    amt = re.sub(r'[^0-9０-９]*?(?:推荐)?合计(?:重量)?\s*', '', amt)
    amt = re.sub(r'^约\s*(?=[0-9])', '约 ', amt)
    # 并列斜杠（双语单位换算或举例）只留第一段
    amt = re.split(r'\s*[/／]\s*', amt)[0].strip()
    amt = re.sub(r'\s*[～~\-—]\s*', '-', amt)
    # 「1 根大葱的葱白即可」->「1 根」：数字+常见单字量词后若紧跟中文尾巴，只保留量词部分
    m = re.match(r'^(约?\s*[0-9０-９.]+\s*[个只条根片瓣块棵颗粒把支袋盒头朵尾张串勺杯碗斤两])[\u4e00-\u9fff]', amt)
    if m:
        amt = m.group(1)
    amt = re.sub(r'\s+', ' ', amt).strip(' ,，。;；')
    return amt


def parse_line(ln):
    """支持三种写法：
    * 食材: 20g     /  - 食材   20g     /  - 食材 = 20g
    返回 (食材, 用量) 或 None
    """
    s = ln.strip()
    if not re.match(r'^[-*]\s+', s):
        return None
    s = re.sub(r'^[-*]\s+', '', s)
    # 三种分隔：= / : / 2+空格
    if '=' in s:
        name, amt = s.split('=', 1)
    elif ':' in s or '：' in s:
        m = re.split(r'[:：]', s, 1)
        name, amt = m[0], m[1]
    else:
        m = re.match(r'^(.*?)\s{2,}(.+)$', s)
        if not m:
            # 「鸡翅 10 ～ 12 只」单空格，用量在末尾
            m = re.match(r'^(.+?)\s+((?:' + NUM + r'.*))$', s)
            if not m:
                return None
        name, amt = m.group(1), m.group(2)
    name, amt = clean_ing(name), clean_amount(amt)
    if not name or len(name) > 14:
        return None
    # 公式/变量行不是给单人份的可用量，跳过（如「份数 * 3 个」「向下取整」）
    if re.search(r'份数|向下取整|向上取整|数量|[×✕]|floor|ceil|\*', amt):
        return None
    # 用量必须含数字或「适量/少许」，否则跳过（如「盐 适量」也接受）
    if not (re.search(NUM, amt) or re.search(r'适量|少许|若干', amt)):
        return None
    return name, norm_num(amt)


def parse_doc(txt):
    m = re.search(r'^##\s*计算\s*$(.*?)(?=^##\s|\Z)', txt, re.M | re.S)
    if not m:
        return []
    out, seen = [], set()
    for ln in m.group(1).split('\n'):
        r = parse_line(ln)
        if not r:
            continue
        name, amt = r
        if name in seen:
            continue
        seen.add(name)
        out.append({'name': name, 'amount': amt})
    return out


def main():
    result = {}
    for f in sorted(glob.glob(SRC + '*.md')):
        name = os.path.basename(f)[:-3]
        txt = open(f, encoding='utf-8').read()
        items = parse_doc(txt)
        # 少于 2 项的（如紫菜蛋花汤只解析出「盐 2 克」）视为该菜无有效精确用量表，跳过
        if len(items) >= 2:
            result[name] = items
    # 输出为「食材 用量」字符串数组，name 去掉空格与「约」残留，直接可当配料用
    shaped = {}
    for n, its in result.items():
        lines = []
        for x in its:
            name = re.sub(r'\s+', '', x['name']).strip('约 ')
            if not name:
                continue
            lines.append(name + ' ' + x['amount'])
        if lines:
            shaped[n] = lines
    json.dump(shaped, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
    print('解析菜数', len(shaped))
    for n, its in sorted(result.items()):
        print(' ', n, len(its), '项:', '、'.join('%s %s' % (x['name'], x['amount']) for x in its[:6]))


if __name__ == '__main__':
    main()
