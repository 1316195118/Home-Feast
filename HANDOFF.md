# 家里吃点啥 · 家庭点菜系统 — 任务交接（Handoff）

> **一句话接管提示（直接发给下一个 AI）：**
> 请接管「家里吃点啥」（项目根 `C:/Users/Administrator/WorkBuddy AI/家庭点菜系统`，**Go 标准库零依赖 + Vue 3 + Vite**，数据纯 JSON 存 `/data`，单二进制 embed 前端，已部署在 PVE docker-LXC `10.10.10.251:8090`）。A–E 与两个额外需求**已全部完成并上线**，本地 git 仓库也已建好；**唯一剩余待办是把代码推到 `github.com/1316195118/Home-Feast`**（远程仓库尚不存在、且本机无 GitHub 凭据，见第三节）。注意版权硬约束与部署约定。

---

## 一、项目关键信息（必读）

- **技术栈（用户明确要求保持 Go + Vue 3）**：后端 Go（运行时零第三方依赖，仅标准库）、前端 Vue 3 + Vite；`node scripts/build.mjs` 构建前端并 embed 进单文件 `menu.exe`。本轮**没有更换技术栈**。
- **数据**：全部纯 JSON 存 `/data`（dishes.json / members.json / plans.json / settings.json / access.json）。**任何 patch / 部署都不要包含 `data/`**。
- **版权硬约束**：结构可参考 gift-book（MIT），但**不得复制**任何开源项目的源码/CSS/文案/图标/图片；新增依赖先确认许可证（避开 GPL/AGPL）并更新 `NOTICE.md`；素材优先自写 / AI 生成 / 用户照片。音效为 Web Audio 实时合成，无任何音频文件。
- **部署**：本机无 Docker，镜像必须在 NAS / PVE LXC 上构建；本机用 `menu.exe` 验证。`scripts/_deploy.py`（**含内网 root 凭据，已被 `.gitignore` 排除，绝不进公开仓库**）会自动备份 data、解压 patch、`docker compose up -d --build` 并做健康检查。
- **部署包**：用 `git archive --format=tar.gz -o scripts/fm_patch.tgz HEAD` 从 git 提交导出，天然不含 `data/`、`node_modules/`、凭据与工作台数据。
- **前端验收**：Edge headless `--window-size=390` 会被钳到 innerWidth=504（假象）；要用 CDP `Emulation.setDeviceMetricsOverride(mobile:true)` + `Page.captureScreenshot`，并用 `Runtime.evaluate` 量 innerWidth。本机 `&` 后台起的 `menu.exe` 会随 Bash 调用结束被杀，验证要把「启动服务 + 截图」放同一条命令。
- **身份**：`fm.user` 只在用户主动选择/切换时写入，绝不在 `refresh()` 兜底时写。
- **Python 依赖**：`scripts/_cq/publish.py` 需要 Pillow、`scripts/_deploy.py` 需要 paramiko；本机已装（用默认 PyPI 源，清华镜像当时报 "no matching distribution"）。

---

## 二、已完成并已上线（2026-11-10 00:36 部署到 PVE）

**A. 52 道重庆菜效果图全部完成**
- 12 张缺图用 ImageGen 图生图补齐（配额已恢复，未再触发 429）；`gen/50` 曾因提示词错字重生成，错图已删。
- `python scripts/_cq/publish.py` 把第二批 26 张裁水印／转 WebP／上传并更新线上 `image`，`published.json` 现为 **52/52**；抽查图片 HTTP 200，线上 `with image 52`。
- `scripts/_cq/gen_progress.json` 已补齐为全部 26 项。

**B. 用餐时段图标选择器**：`utils.js` 新增 `mealIconChoices`（24 个）；`SettingsView.vue` 把手输 emoji 换成点击展开的预设面板 + 自定义输入；CDP 实测面板 24 项、桌面/手机均正常。

**C. 用户头像扩充**：`avatarGroups` 四组（人物/动物/食物/兴趣）共 56 个，`avatarChoices` 扁平数组兼容旧逻辑；`FamilyView.vue` 面板分组 + `max-height:216px` 内滚动；实测 4 组 56 项。

**D. 动效与反馈体系**：`tokens.css` 新增 `--dur-instant/fast/base/slow/loop` 与 `--ease-out/in-out/spring`；`base.css` 补 `shake-error` / `success-bounce` / `count-pop` / `clash-left` / `clash-right` 与统一反馈类；toast 成功/错误图标反馈、今日统计数字 `value-pop`；`prefers-reduced-motion` 全量降级。

**E. 骰子 PK 全部完成**
- **E1 点数持久化**：`internal/store/model.go` 的 `Plan` 新增 `Dice map[string]int` / `DicePhase` / `PKShown`（`omitempty` 兼容旧数据）；`PlanPatch` 同步扩展；`store.go SavePlan` 深拷贝落盘；`api/plans.go` 校验成员 id、点数 1–6、阶段枚举、名单去重。前端 `App.vue` 新增 `saveDice`（不走 guard，避免整页 refresh 打断动画）。
- **E2 跨用户自动开局**：`DicePK.vue` 打开时 `hydrate()` 从 `state.plan` 恢复点数/阶段/名单；每次摇完 `persist()` 写库；全员摇完 `decided` → 1.4s 后 `emit('apply')` 自动 `setDuties` 并关闭；分工已落库时 `alreadyDecided` 只展示、不重复提交。另有「🎲 再来一局」清空骰子。
- **E3 动画每人一次**：`Plan.pkShown` 按用户维度记录；`playResultOnce()` 用 `fxOwner` 防止同一次结果被 watch + 初始化重复触发时把播放态清掉（这是实测抓到并修掉的真 bug）；`.pk-result.is-fx` 才播对撞动画，已看过的人不再播。
- **E4/E5 音效与角标**（上一轮完成，本轮一并上线）：`web/src/audio.js` 纯 Web Audio 合成 roll/win/lose/safe + `fm.sound` 静音开关；菜图 📖/☆/＋ 半透明小角标。
- **端到端实测**：3 人依次摇完 → A 摇完切 B 上线能看到 A 点数、第三人摇完自动出结果并落库分工（`chef=爸爸 / wash=我 / phase=done`）、第三人首次看 `fx=true`、A 重新上线首次看 `fx=true`、A 再打开 `fx=false`、`pkShown` 最终含两人，全程无 JS 报错。

**额外两个需求**
- **菜品详情页图片看不全**：`.detail-photo` 原 `display:grid` 使 `height:100%` 失效、img 被裁（实测 img 240×240 vs 容器 240×180）。改为 `position:relative` + img `position:absolute; inset:0; object-fit:contain`，桌面/手机实测 img 尺寸严格等于容器（240×180 / 320×180），整图可见。
- **「TA」改具体用户名**：`App.vue` 分工 toast 用 `dutyName(id)` 输出真实姓名；`TodayView` 的「又做饭又洗碗」加名字；DicePK 里「切换到 TA」改为具体人名/名单 `needOtherNames`。

**F. GitHub 发布准备（本地已完成，推送未完成）**
- `README.md` 顶部加仓库地址、功能表补最新特性、新增「clone → `docker compose up -d --build` 一键起」章节；`docker-compose.yml` 本就在仓库根。
- 已 `git init -b main`，两个提交（`9a6564b`、`283c22d`），104 个文件 / 8.8MB patch。
- `.gitignore` 排除 `data/`、`web/node_modules`、`web/dist`、`webapp/dist`、AI 绘图中间产物、`.workbuddy/`，以及**含 PVE 密码的 `scripts/_deploy.py`** 和抓取的原始素材（版权与体积考虑）。公开仓库无凭据、无第三方素材。

**上线验收**：`go vet` / `go test` / `vite build` 通过；`api-test` 14/14；CDP 双视口（1280/390）截图无布局问题；三个新弹窗（详情图/图标面板/头像面板）双视口实测通过；部署后线上 JS/CSS hash 与本地一致，含 `--dur-base`、`AudioContext`、`fm.sound`、`createBufferSource`，健康检查 `{"ok":true}`，数据备份在 `/opt/family-menu/data-backup-20261110-003526`。

---

## 三、剩余待办

### 1. 推送到 GitHub（唯一硬阻塞，需用户操作）
- `git push -u origin main` 返回 **`remote: Repository not found`**：`https://github.com/1316195118/Home-Feast` 目前不存在（`api.github.com` 返回 404），且 `gh auth status` 显示未登录、无 `GH_TOKEN`。
- **下一步**：请用户在 GitHub 网页创建**空仓库** `Home-Feast`（不要勾选 README/LICENSE/.gitignore，避免冲突），然后回来让 AI 执行 `GIT_TERMINAL_PROMPT=0 git push -u origin main`；若 GCM 无缓存凭据，还需用户提供 PAT 或完成一次 `gh auth login`。
- 仓库就绪后可直接推送，本地 `origin` 已配好、分支 `main`、提交齐全。

### 2. 可选：真机试听音效
- 结果音（win/lose/safe）只验证了合成路径与代码分支，尚未在真实设备上试听音量与听感；部署后让家人摇一局确认即可。

---

## 四、关键文件索引
- 骰子 PK：`web/src/components/DicePK.vue`（hydrate/persist/playResultOnce/restart）、`web/src/components/TodayView.vue`（`openPK`/`applyPK`）、`web/src/App.vue`（`saveDuties`/`saveDice`/`dutyName`）
- 后端 Plan：`internal/store/model.go`（`Plan`/`PlanPatch` + 骰子字段）、`internal/store/store.go`（`SavePlan`）、`internal/api/plans.go`（`updatePlan` 校验）
- 音效：`web/src/audio.js`、`web/src/main.js`（`installAudioUnlock`）
- 菜卡 / 详情：`web/src/components/DishCard.vue`、`web/src/components/DishDetail.vue`、`web/src/styles/screens.css`（「图片角标」段、`.detail-photo`）
- 图标 / 头像 / 动效：`web/src/utils.js`（`mealIconChoices`/`avatarGroups`）、`SettingsView.vue`、`FamilyView.vue`、`styles/tokens.css`、`styles/base.css`
- 图片：`scripts/_cq/remaining.json`、`manifest.json`、`publish.py`、`published.json`（52 项）、`gen_progress.json`
- 验收：`scripts/_ui/capture.mjs`（CDP 双视口截图）、`scripts/api-test.mjs`
- 部署 / 发布：`scripts/_deploy.py`（本地，含凭据）、`scripts/fm_patch.tgz`、`Dockerfile`、`docker-compose.yml`、`README.md`、`NOTICE.md`

## 五、验证与部署流程（已跑通）
1. `go vet ./... && go test ./... && node scripts/build.mjs`
2. 启动本地 `menu.exe` → `BASE=http://127.0.0.1:8090 node scripts/api-test.mjs` → CDP 双视口截图（桌面 1280 / 手机 390）+ 弹窗 DOM 检查
3. `git add -A && git commit ...` → `git archive --format=tar.gz -o scripts/fm_patch.tgz HEAD`
4. `MSYS_NO_PATHCONV=1 python scripts/_deploy.py`（自动备份 data → 解压 → compose build → 健康检查）
5. 线上核对：`/api/ping`、首页 JS/CSS hash 与本地一致、抽查 `/uploads/*.webp` 200、`/api/bootstrap` 的带图菜品数
