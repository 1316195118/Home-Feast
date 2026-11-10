# -*- coding: utf-8 -*-
"""把统一输出目录 unified_raw/ 里的生成图按「生成顺序」归位到 unified/NNN/。

为什么需要：图片生成工具在并发调用时 output_dir 会串位（返回路径与期望目录不符），
因此改为【全部输出到同一个目录 + 严格串行按 idx 顺序生成】。文件名自带时间戳，
串行生成下时间戳严格递增，按文件名排序即等于按 idx 顺序，可无损归位。

用法：python scripts/_cq/assign_raw.py [--dry]
支持多次运行：已归位的 idx 会跳过，新图接着往后排（断点续跑）。
"""
import glob
import os
import sys

ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
RAW = ROOT + 'scripts/_cq/unified_raw/'
DST = ROOT + 'scripts/_cq/unified/'
DRY = '--dry' in sys.argv


def main():
    raw = sorted(glob.glob(RAW + '*.png'))  # 文件名含时间戳，按名排序即按生成顺序
    have = set()
    for d in os.listdir(DST):
        p = DST + d
        if os.path.isdir(p) and glob.glob(p + '/*.png'):
            try:
                have.add(int(d))
            except ValueError:
                pass
    targets = [i for i in range(2, 106) if i not in have]
    print('待归位 %d 张 / 空位 %d 个（已有 %d 个目录有图）' % (len(raw), len(targets), len(have)))
    n = min(len(raw), len(targets))
    for f, idx in zip(raw[:n], targets[:n]):
        out = DST + '%03d/' % idx
        os.makedirs(out, exist_ok=True)
        dst = out + os.path.basename(f)
        print('%s -> %03d/' % (os.path.basename(f)[-24:], idx))
        if not DRY:
            os.replace(f, dst)
    left = glob.glob(RAW + '*.png')
    print('完成 %d 张，unified_raw 剩余 %d 张' % (n, len(left)))


if __name__ == '__main__':
    main()
