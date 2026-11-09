# 家庭点菜系统 — 任务交接（Handoff）

> **一句话接管提示（直接发给下一个 AI）：**
> 请接管并继续完善「家庭点菜系统」（项目根 `C:/Users/Administrator/WorkBuddy AI/家庭点菜系统`，Go 零三方依赖 + Vue3 + Vite，数据纯 JSON 存 `/data`，单二进制 embed 前端，默认端口 8090，已部署在 PVE docker-LXC `10.10.10.251:8090`），按本文件「未完成」清单接着做剩余任务（A 剩 12 张图、B/C/D、E 的 1–3、F 发 GitHub；E 的 4–5 已本地完成但**尚未部署**），注意版权硬约束与部署约定。

---

## 一、项目关键信息（必读）

- **技术栈**：后端 Go（运行时零第三方依赖）、前端 Vue3 + Vite；`go run` 仅用标准库，`node scripts/build.mjs` 构建前端并 embed。
- **数据**：全部纯 JSON 存 `/data`（dishes.json / members.json / plans.json / settings.json / access.json）。**任何 patch / 部署都不要包含 `data/`**。
- **版权硬约束**：结构可参考 gift-book（MIT），但**不得复制**任何开源项目的源码/CSS/文案/图标/图片；新增依赖先确认许可证（避开 GPL/AGPL）并更新 `NOTICE.md`；素材优先自写 / AI 生成 / 用户照片。
- **部署**：本机无 Docker，镜像必须在 NAS 上构建；本机用 `menu.exe` 验证。`_deploy.py` 已含部署前 data 备份 + 部署后健康检查；patch 必须显式列源码路径、绝不含 data。
- **前端验收**：Edge headless `--window-size=390` 会被钳到 innerWidth=504（假象）；要用 CDP `Emulation.setDeviceMetricsOverride(mobile:true)` + `Page.captureScreenshot`，并用 `Runtime.evaluate` 量 innerWidth。本机 `&` 后台起的 `menu.exe` 会随 Bash 调用结束被杀，验证要把「启动服务 + 截图」放同一条命令；复验前用 PowerShell `Get-Process menu | Stop-Process -Force` 清残留。
- **身份**：`fm.user` 只在用户主动选择/切换时写入，绝不在 `refresh()` 兜底时写。

---

## 二、已完成（上一会话已上线）

- 今日菜单：只渲染有订单的餐别组，过期/没点的不显示，整天没菜才显示空态。
- 桌面「🎲 选择困难？」固定 3 卡一排 + 常驻「＋」角标；骰子放大（58px / 手机 48px），按钮 124×42。
- 设置页 7 张卡等宽；「🕐 用餐时段」改为紧凑摘要卡 + 管理弹窗（编辑在弹窗内）。
- 访问口令可在网页设置：独立 `data/access.json`（不进 settings/bootstrap/备份 zip）；`MENU_ACCESS_CODE` 环境变量优先（此时 Web 改返回 400）；改口令先更新本机请求头再 refresh。
- 后端 `PlanPatch` 指针语义区分未传/清空；谁掌勺/谁洗碗平行字段；骰子 PK 两人摇完自动定分工（1.4s 后自动 emit apply，无「就这么定」按钮）；平局系统自动加摇。
- **【本次会话新增：仅本地完成，尚未部署到 10.10.10.251】** E5 两项：菜图角标轻量化 + 骰子 PK Web Audio 合成音效。已重新 `node scripts/build.mjs` 构建 `menu.exe`，双视口 CDP 验收、`go vet` / `go test` / 14 项接口冒烟全通过；按第五节流程部署即可上线。实现细节见第三节 E.5。

---

## 三、未完成（待下一个 AI 接着做）

### A. 剩余 12 张重庆菜效果图（图片生成，有配额限制）
- **状态（2026-11-09 23:36 复核，以 `scripts/_cq/gen/` 实际文件为准）**：
  - 52 道重庆菜中第一批 **26 张已上线**（`published.json` 的 key = 1–24、31、47；`gen/31`、`gen/47` 里的 `sample.png` 属于这批，**勿重复处理**）。
  - 第二批 `remaining.json` 共 26 项（idx 25–30、32–46、48–52），其中 **14 张已生成**：idx 25–30、32–39；**尚缺 12 张**：40–46、48–52。
  - `gen_progress.json` 的 `done` 只登记了 12 项（25–30、32–37），**漏记 38、39**；断点续跑请以目录实文件为准，不要只信该进度文件（必要时把 38/39 补登记）。
  - 原交接记录写的「`gen/25/` 试成、其余 25 项未生成」已过时，本段已更正。
- **优先做（不耗图片配额）**：直接 `python scripts/_cq/publish.py` 把已生成的 14 张裁水印／转 WebP／上传并更新线上菜品 `image`；脚本用 `published.json` 断点续跑，不会重复处理已上线的 1–24/31/47。
- **怎么做（补缺的 12 张）**：调用 ImageGen（图生图，`size:1536x1024`、`quality:high`、`input_fidelity:high`，`prompt` 取 remaining.json 原文，`image1` 取 `base` 路径，`output_dir` 取 `outdir`），每批 ≤3 张避免 429；成功后同上跑 `publish.py`。
- **注意**：图片生成接口有累计配额，曾触发 **429**，重置时间约 **2026-11-10 21:53 UTC+8 之后**；截至本次复核（11-09 23:36）尚未重置，**暂不要调用图片生成**，先把不耗配额的发布环节做掉。
- **验收**：图片 200 加载、4:3 裁切、避开原图右下角平台标识。

### B. 用餐时段图标选择器
- 把 `SettingsView.vue` 用餐时段弹窗里「手输 emoji 图标」改成**预设图标面板**（`mealIconChoices` 一批可选 emoji，点击即选，仍可自定义），桌面/手机布局正常。

### C. 用户头像扩充
- `web/src/utils.js` 的 `avatarChoices` 从 12 个扩到更丰富一组（人物/动物/食物/兴趣）；`FamilyView.vue` 选择面板支持滚动与分组，避免弹窗被撑高。

### D. 完整动效与反馈体系
- `tokens.css` 定义动效令牌（时长/缓动分级）；`base.css` 补齐缺失 keyframes（错误抖动、成功弹跳、骨架微光、数值变化、收藏等）；统一按钮/表单/保存/删除/点菜/收藏的即时反馈；`prefers-reduced-motion` 全量降级；不引入依赖。

### E. 骰子 PK 改进（用户最新需求，重点；1–3 待做，4–5 本次已完成）
当前 `DicePK.vue` 的 `values` 只存本地 ref，**切用户/刷新即丢**，且 PK 动画缺失（音效本次已补，见第 5 条）。需做：

1. **点数持久化**：后端 `Plan`（`internal/store/model.go`）新增骰子结果字段（如 `Dice map[string]int` 或 `DiceRolls`），落盘到 `plans.json`；摇完即写库；前端打开 PK 时从 `state.plans[today]` 读取已有点数，跨用户/刷新不丢。
2. **都摇完自动开局**：已部分实现（`watch(decided)` 1.4s 后 `emit('apply')`），但要保证**跨用户**：A 摇完退出、B 上线摇完，B 这边能检测到「两人都摇完」并自动 `setDuties` 并关闭。
3. **PK 动画只显示一次**：出结果后有赢家/输家 PK 动画，**每个用户只看一次**。用已展示标记（存 Plan 的 `pkShown` 列表或 localStorage）控制；先摇的 A 重新上线也应再看到一次（即标记按用户维度、且在他登录态内只播一次）。
4. ✅ **音效（本次已完成，实现见第 5 条）**：摇骰声 / 胜利欢快声 / 失败声已全部用 **Web Audio API 合成**，做成 `web/src/audio.js` 单例，**未引入任何外部音频文件或新依赖**（版权硬约束）。用户原话「请帮我找对应的声音」即指此。剩余只差部署后在真机上试听确认音量与听感。
5. ✅ **菜图 📖 / ＋ 挡图优化 + 音效合成（本次已完成，仅本地）**
   - **角标**：`DishCard.vue`（📖/☆）与 `TodayView.vue`（📖/＋）改为「36px 点击热区 + 24px 半透明磨砂视觉圆点」，emoji 包一层 `.badge-ico`；默认 `rgba(255,253,249,.46)` + `blur(3px)`，只有鼠标 hover / 键盘聚焦 / 按下才高亮放大（hover 规则包在 `@media (hover: hover)`，避免触屏把 hover 粘住）。≤720px 热区 34px、视觉 20px、opacity .72；＋ 用半透明品牌色并保留 `pointer-events:none`（纯装饰）。样式集中在 `screens.css` 的「图片角标」段。
   - **音效**：新增 `web/src/audio.js`，纯 Web Audio API 合成 `playRoll`（8 个带通噪声碰撞 + 收尾闷响）、`playWin`（C 大三和弦上行琶音）、`playLose`（A 小调下行）、`playSafe`（安全过关轻两声）；`fm.sound` localStorage 静音开关 + `installAudioUnlock()`（首次用户手势解锁 AudioContext，在 `main.js` 调用）；`DicePK.vue` 摇骰播 roll、出结果按当前身份播 win/lose/safe（弹窗生命周期内只播一次），弹窗头部加 🔊/🔇 按钮。**不含任何外部音频文件，未新增第三方依赖**。
   - **验收**：桌面 1280 / 手机 390 双视口 CDP 实测角标热区 36/34px、视觉 24/20px、opacity .82/.72；摇一次产生 8 个 bufferSource + 1 个 oscillator（合成音路径确实执行）；静音重载后图标 🔇、再摇不创建任何音频节点；浏览器无 JS 报错。截图见 `scripts/_ui/shots/*-e5-pk.png`、`desktop-menu.png`、`phone-menu.png`。
   - ⚠️ **未部署**：本次按约定不连线上，改动只在本地 `webapp/dist` 与 `menu.exe`；上线按第五节流程走。
   - 结果音（win/lose/safe）未做端到端「三人依次摇完」的听感验证，只验证了同一合成路径与代码分支；真机试听建议在部署后补一次。

### F. 发布到 GitHub
- **先**全量检查 bug（`go vet ./...`、`go test ./...`、`node scripts/api-test.mjs`、CDP 双视口截图）。
- 发布到 `github.com/1316195118/Home-Feast`，中文名 **「家里吃点啥」**。
- 生成项目简介 README：**突出优点**（零依赖单二进制、纯 JSON 数据、手机/电脑自适应、今日菜单自动分类、骰子 PK 分工、做法教程、网页设口令、Docker 一键部署）、**详细教程**（clone → `docker-compose up -d` 一键起）、**docker-compose.yml 放在仓库根**，让用户复制即用。

---

## 四、关键文件索引
- 骰子 PK：`web/src/components/DicePK.vue`、`web/src/components/TodayView.vue`（`openPK`/`applyPK`）、`web/src/App.vue`（`setDuties`）
- 后端 Plan：`internal/store/model.go`（`Plan`/`PlanPatch`）、`internal/store/store.go`（`SavePlan`）、`internal/api/plans.go`
- 菜卡浮层：`web/src/components/DishCard.vue`
- 动效：`web/src/styles/tokens.css`、`web/src/styles/base.css`
- 头像/图标：`web/src/utils.js`（`avatarChoices`）、`web/src/components/FamilyView.vue`、`web/src/components/SettingsView.vue`
- 图片生成：`scripts/_cq/remaining.json`（第二批 26 项）、`scripts/_cq/manifest.json`（52 道总表）、`scripts/_cq/publish.py`、`scripts/_cq/published.json`（已上线 26 张 + 旧图可回滚）、`scripts/_cq/gen_progress.json`（漏记 38/39，仅作参考）
- 音效：`web/src/audio.js`（Web Audio 实时合成，无外部音频）、`web/src/main.js`（`installAudioUnlock`）
- 验收：`scripts/_ui/capture.mjs`（CDP 双视口截图，复用它做布局验收）、`scripts/_ui/shots/`
- 部署：`scripts/_deploy.py`、`Dockerfile`、`docker-compose.yml`

## 五、验证与部署流程
1. `go vet ./... && go test ./... && node scripts/build.mjs`
2. `BASE=http://127.0.0.1:8090 node scripts/api-test.mjs`
3. CDP 双视口（桌面 1280 / 手机 390）截图验收，量 innerWidth 防假象
4. 打包源码 patch（**不含 data/**）→ `MSYS_NO_PATHCONV=1 python scripts/_deploy.py` → 健康检查 → 线上 `curl` 验证
