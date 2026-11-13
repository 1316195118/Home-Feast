# -*- coding: utf-8 -*-
"""把 recipes.js 里内联的 const HOWTO = {...23...} 改为 import web/src/howto-amounts.json，
使 howto-amounts.json（已含全部 109 道）成为菜谱克数的唯一来源。"""
import io, os, re

SRC = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/web/src/recipes.js'
txt = open(SRC, encoding='utf-8').read()

# 起点：HowToCook 注释行；终点：内联 HOWTO 对象的闭合 }
start = txt.index('// HowToCook (Unlicense')
end_m = re.search(r'\n\}\n', txt[start:])
assert end_m, '未找到内联 HOWTO 的闭合 }'
end = start + end_m.end()

new_block = (
    "// HowToCook (Unlicense, 公共领域) 精确用量表：覆盖全部内置菜（含 parse_howto_amounts.py\n"
    "// 从「计算」段解析的 23 道，以及后续人工补全的其余内置菜）。存于 web/src/howto-amounts.json，\n"
    "// 运行时整表替换模板配料；做法步骤仍用本项目自写模板；用户自定义配料永远优先。\n"
    "// 注意：此文件是菜谱克数的唯一来源，勿直接用 parse_howto_amounts.py 覆盖（会清空人工补全部分）。\n"
    "import HOWTO from './howto-amounts.json'\n"
)

new_txt = txt[:start] + new_block + txt[end:]
open(SRC, 'w', encoding='utf-8').write(new_txt)
print('已替换内联 HOWTO 为 import howto-amounts.json')
print('残留 const HOWTO = { :', 'const HOWTO = {' in new_txt)
print('存在 import HOWTO :', 'import HOWTO from' in new_txt)
