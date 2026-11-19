#!/usr/bin/env python3
"""家庭点菜系统 端到端测试：真实浏览器（Edge headless）跑核心用户旅程。

覆盖：首屏加载、切页、点菜、今日菜单、批量多选、冰箱食材（3D 冰箱/搜索/保质期）、
购物清单、周菜单、家庭页 TOP 榜、设置页各项、口令、移动端视口、控制台错误。

用法：python scripts/_cq/_test/e2e_full.py
输出：每条 PASS/FAIL + 末尾汇总，非 0 退出码表示有失败。
"""
import json
import re
import sys
import time
import urllib.request

from playwright.sync_api import sync_playwright

BASE = sys.argv[1] if len(sys.argv) > 1 else 'http://127.0.0.1:18090'
results = []
console_errors = []


def check(name, ok, detail=''):
    results.append((name, bool(ok), detail))
    print(('  ✓ ' if ok else '  ✗ ') + name + (f'  [{detail}]' if detail and not ok else ''))


def api(path):
    with urllib.request.urlopen(BASE + path, timeout=30) as r:
        return json.loads(r.read())


def run():
    boot = api('/api/bootstrap')
    today = boot['today']
    dish0 = boot['dishes'][0]
    member0 = boot['members'][0]

    with sync_playwright() as pw:
        browser = pw.chromium.launch(channel='msedge', headless=True)
        ctx = browser.new_context(viewport={'width': 1440, 'height': 900}, locale='zh-CN')
        page = ctx.new_page()
        page.on('console', lambda m: console_errors.append(m.text) if m.type == 'error' else None)
        page.on('pageerror', lambda e: console_errors.append(str(e)))

        print('\n【A. 首屏与导航】')
        page.goto(BASE, wait_until='networkidle', timeout=45000)
        # 首次启动会弹「先选一下你是谁」，选第一个成员继续（弹窗挂载有延迟，最多等 3 秒）
        for _ in range(12):
            if page.locator('.user-pick-backdrop').count(): break
            page.wait_for_timeout(250)
        if page.locator('.user-pick-backdrop').count():
            page.locator('.user-pick-item').first.click()
            page.wait_for_timeout(800)
            check('首次选身份可完成', page.locator('.user-pick-backdrop').count() == 0)
        page.wait_for_load_state('networkidle', timeout=15000)
        check('首屏渲染', page.locator('.app-shell, #app').count() > 0)
        check('今日菜单面板存在', page.locator('.panel-title', has_text='今日菜单').count() > 0)
        check('标题已加载', 'Loading' not in page.title(), page.title())

        def goto_tab(tab):
            loc = page.locator('.tab-bar button.tab').filter(has_text=tab)
            if not loc.count():
                loc = page.locator('button.tab').filter(has_text=tab)
            loc.first.click()
            page.wait_for_timeout(700)

        for tab, sel in [('菜谱', '家里的菜谱'), ('家人', '家里的人'), ('设置', '设置')]:
            goto_tab(tab)
            check(f'切到「{tab}」页', page.get_by_text(sel, exact=False).count() > 0)

        print('\n【B. 菜谱页：搜索 / 快捷筛选 / 多选】')
        goto_tab('菜谱')
        search = page.locator('input[placeholder*="搜"]').first
        search.fill('番茄')
        page.wait_for_timeout(700)
        marks = page.locator('mark').count()
        check('搜索命中高亮 <mark>', marks > 0, f'{marks} 处')
        search.fill('')
        page.wait_for_timeout(400)
        for label in ['30分钟内', '不辣', '有图']:
            chip = page.locator('.quick-chip', has_text=label)
            if chip.count():
                chip.first.click()
                page.wait_for_timeout(500)
                check(f'快捷筛选「{label}」可切换', 'active' in (chip.first.get_attribute('class') or ''))
                chip.first.click()
                page.wait_for_timeout(300)

        # 多选批量点菜
        multi_btn = page.locator('button', has_text='多选').first
        if multi_btn.count():
            multi_btn.click()
            page.wait_for_timeout(500)
            cards = page.locator('.dish-card')
            picked = 0
            for i in range(min(2, cards.count())):
                c = cards.nth(i)
                if c.locator('.multi-check').count():
                    c.locator('.dish-photo').click()
                    picked += 1
                    page.wait_for_timeout(250)
            check('多选模式勾选 2 道', picked == 2, f'勾了 {picked}')
            bar = page.locator('.multi-bar')
            check('底部批量浮条出现', bar.count() > 0)
            if bar.count():
                bar.locator('button', has_text='加入今日菜单').click()
                page.wait_for_timeout(2500)
                check('批量点菜完成并退出多选', page.locator('.multi-bar').count() == 0)

        print('\n【C. 冰箱食材：3D / 搜索 / 自动保存】')
        pantry_btn = page.locator('button', has_text='冰箱食材').first
        pantry_btn.click()
        page.wait_for_timeout(900)
        check('冰箱面板打开', page.locator('.pantry-modal').count() > 0)
        check('3D 冰箱存在', page.locator('.fridge').count() > 0)
        check('三层结构（上双门+抽屉）', page.locator('.fridge-upper').count() + page.locator('.fridge-lower').count() >= 2)
        # 默认不铺满：无搜索时不应出现全部预设分组
        idle_items = page.locator('.pantry-section .pantry-card').count()
        check('默认不铺满食材', idle_items <= 12, f'当前 {idle_items} 张卡')
        # 搜索
        psearch = page.locator('.pantry-modal input[placeholder*="搜"]').first
        psearch.fill('土豆')
        page.wait_for_timeout(600)
        hit = page.locator('.pantry-card', has_text='土豆')
        check('食材搜索出结果', hit.count() > 0, f'{hit.count()} 张')
        if hit.count():
            hit.first.click()
            page.wait_for_timeout(1200)
            check('勾选后冰箱内出现食材', page.locator('.fridge-food').count() > 0,
                  f"{page.locator('.fridge-food').count()} 个")
            psearch.fill('')
            page.wait_for_timeout(600)
            check('已选区出现保质期徽章', page.locator('.pantry-age').count() > 0,
                  f"{page.locator('.pantry-age').count()} 个")
        # 开门/关门
        door = page.locator('.fridge-door-l').first
        cls0 = door.get_attribute('class') or ''
        door.click()
        page.wait_for_timeout(800)
        cls1 = door.get_attribute('class') or ''
        check('左门可开关（class 变化）', cls0 != cls1, f'{cls0} → {cls1}')
        # 视角切换
        knob = page.locator('.fridge-knob').first
        if knob.count():
            knob.click()
            page.wait_for_timeout(600)
            check('温度旋钮切换视角', 'view-' in (page.locator('.fridge').first.get_attribute('class') or ''))
        page.locator('.pantry-modal .icon-btn, .pantry-modal button:has-text("完成")').first.click()
        page.wait_for_timeout(800)
        check('冰箱面板关闭', page.locator('.pantry-modal').count() == 0)
        saved = api('/api/bootstrap')['settings'].get('pantry') or []
        check('食材已落库（自动保存）', '土豆' in saved, f'{saved}')

        print('\n【D. 今日菜单 / 购物清单】')
        goto_tab('今天')
        orders = page.locator('.order-name').count()
        check('今日菜单有菜品', orders > 0, f'{orders} 道')
        check('统计条显示', page.locator('.stat-strip').count() > 0)
        for btn in ['购物清单', '分享']:
            b = page.locator('button', has_text=btn).first
            if b.count():
                b.click()
                page.wait_for_timeout(900)
                opened = page.locator('.modal').count() > 0
                check(f'「{btn}」弹窗可打开', opened)
                if opened:
                    page.keyboard.press('Escape')
                    page.wait_for_timeout(500)
                    check(f'「{btn}」Esc 可关闭', page.locator('.modal').count() == 0)

        print('\n【E. 周菜单 / 家庭页】')
        goto_tab('今天')
        wk = page.locator('button', has_text='生成一周菜单')
        check('周菜单入口存在', wk.count() > 0)
        if wk.count():
            wk.first.click()
            page.wait_for_timeout(700)
            check('周菜单弹窗打开', page.locator('.week-bar').count() > 0)
            gen = page.locator('button', has_text='生成本周菜单')
            if gen.count():
                gen.first.click()
                page.wait_for_timeout(1600)
                check('周菜单生成 7 天', page.locator('.week-day').count() == 7,
                      f"{page.locator('.week-day').count()} 天")
                check('「今天」行高亮', page.locator('.week-today').count() > 0)
            page.keyboard.press('Escape')
            page.wait_for_timeout(500)
        goto_tab('家人')
        check('热力图存在', page.locator('.heat-cell').count() > 0)
        check('本月 TOP 榜存在', page.locator('.top-list li').count() > 0 or page.locator('.empty').count() > 0)

        print('\n【F. 设置页】')
        goto_tab('设置')
        check('设置卡片渲染', page.locator('.setting-card').count() >= 6, f"{page.locator('.setting-card').count()} 张")
        check('问号提示存在', page.locator('.help-btn').count() > 0)
        if page.locator('.help-btn').count():
            page.locator('.help-btn').first.hover()
            page.wait_for_timeout(400)
            check('问号悬停弹出说明', page.locator('.help-pop').count() > 0)
        check('版本号显示', page.get_by_text('1.0.', exact=False).count() > 0)

        print('\n【G. 移动端视口】')
        mobile = ctx.new_page()
        mobile.set_viewport_size({'width': 390, 'height': 844})
        mobile.goto(BASE, wait_until='networkidle', timeout=45000)
        mobile.wait_for_timeout(1200)
        check('手机端首屏渲染', mobile.locator('.panel-title').count() > 0)
        sw = mobile.evaluate('document.documentElement.scrollWidth')
        check('无横向溢出', sw <= 400, f'scrollWidth={sw}')
        mb = mobile.locator('button', has_text='冰箱食材')
        if mb.count() == 0:
            mobile.locator('button.tab').filter(has_text='菜谱').first.click()
            mobile.wait_for_timeout(700)
            mb = mobile.locator('button', has_text='冰箱食材')
        if mb.count():
            mb.first.click()
            mobile.wait_for_timeout(900)
            check('手机端冰箱可用', mobile.locator('.fridge').count() > 0)
            box = mobile.locator('.fridge').first.bounding_box()
            check('冰箱宽度适配屏幕', box and box['width'] <= 390, f"{box['width'] if box else '?'}")
        mobile.close()

        print('\n【H. 控制台错误】')
        # 过滤第三方/网络噪声
        real = [e for e in console_errors if not re.search(r'favicon|Failed to load resource|404 \(Not Found\)', e)]
        check('无 JS 运行时报错', len(real) == 0, str(real[:2]))

        browser.close()

    ok = sum(1 for _, o, _ in results if o)
    fail = len(results) - ok
    print('\n' + '=' * 52)
    print(f'E2E 汇总：{ok} 通过 / {fail} 失败 / 共 {len(results)}')
    for n, o, d in results:
        if not o:
            print(f'  FAIL  {n}  {d}')
    print('=' * 52)
    return 1 if fail else 0


if __name__ == '__main__':
    sys.exit(run())
