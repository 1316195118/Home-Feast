# 家里吃点啥 · 家庭点菜系统 — 任务交接（Handoff）

> **一句话接管提示（直接发给下一个 AI）：**
> 请接管「家里吃点啥」（项目根 `C:/Users/Administrator/WorkBuddy AI/家庭点菜系统`，**Go 标准库零依赖 + Vue 3 + Vite**，数据纯 JSON 存 `/data`，单二进制 embed 前端）。代码已发布到 **https://github.com/1316195118/Home-Feast**（中文名「家里吃点啥」），并部署在 PVE docker-LXC `10.10.10.251:8090`。A–E、交互反馈、**第一批 5 项新功能**（深色主题 / PWA / Excel / 避雷标签 / 购物清单）以及 16 张写实菜系封面**全部完成并上线**；剩余待办见第三节：**105 道菜逐道写实图（需确认范围与积分）**、第二批排班/智能推荐、第三批架构级需求。注意版权硬约束与部署约定。

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

**16 个菜系专属插画**：原来只有 10 张图、6 个菜系在复用别人的图。补生成 7 张同风格水彩插画（`chongqing/soup/staple/cold/breakfast/dessert/western.jpg`，1024×1024 JPEG，原始 PNG 留档 `docs/artwork/cuisine-*.png`），`internal/store/model.go` 的 16 个 `Cuisines` 现在一一对应；`NOTICE.md` 与 README 同步。

**交互反馈、音效与布局（本轮新增）**
- `web/src/audio.js` 新增 6 个通用合成音：`playTap`（轻点击）、`playAdd`（点菜）、`playFav`（收藏）、`playSave`（保存）、`playRemove`（删除）、`playError`（失败），音量比 PK 音效小一档，统一受 `fm.sound` 静音开关与首次手势解锁管理。
- `App.vue` 用 `withSound()` 在写操作成功后播对应音；`toast(type==='error')` 播失败音（配合已有的 toast 抖动）。
- `ripple.js` 一次点击给四件事：轻音、涟漪、从落点扩散的**指向环** `.ripple-pointer`、目标 `.is-tapped` 短暂高亮；`base.css` 补 `pointer-ring` keyframes、统一 `:focus-visible` 焦点环。`prefers-reduced-motion: reduce` 下**只保留声音、跳过视觉扩散**（CDP 实测验证）。
- 布局：手机端 `.btn` / `.icon-btn` 点击区统一 40px、编辑图标不再被挤扁，`.setting-actions` 换行、弹窗标题字号收敛。
- 实测：真实指针事件下 `pointer/ripple/is-tapped` 全为 true；轻点击产生 1 oscillator + 1 bufferSource + 3 gain，点菜后再 +2 oscillator；双视口无 JS 报错。

**第一批 5 项新功能（已完成）**
- **深色主题**：`tokens.css` 加语义变量 + `html[data-theme="dark"]` 覆盖；`theme.js` 管理 `fm.theme`（跟随系统/浅色/深色）并监听 `prefers-color-scheme`；设置页三选一。CDP 实测背景 `rgb(251,246,238)` → `rgb(24,20,16)`。
- **PWA 离线**：`web/public/manifest.json` + 自绘 `icon-192/512.png` + `sw.js`（app shell / 静态资源缓存，`/api/` 一律走网络，导航 network-first、离线回退 index.html）；`index.html` 加 manifest/apple meta，`main.js` 注册。CDP 实测 registration/controller 均存在，断网刷新后页面仍能渲染。
- **Excel 导出**：`internal/api/excel.go` 用 Go 标准库 `archive/zip` 生成真 `.xlsx`（今日菜单 + 菜谱清单两个工作表，inlineStr 中文），`GET /api/export/excel`，设置页「📊 导出 Excel」。实测 zip 结构完整、sheet2 110 行中文正常。
- **口味避雷**：`Member.Avoid` 落 `members.json`（omitempty 兼容旧数据），`members.go` 清洗去重；`avoidChoices` + `avoidConflicts`；FamilyView 预设/自定义选择、成员卡展示；菜卡 `⚠️` 提示。**注意**内置菜配料由 `recipes.js` 动态生成，匹配时要并入 `getRecipe()` 的配料。
- **购物清单**：`ShoppingList.vue` 从今日订单聚合配料（同名合并、记来源），按 `shoppingKind()` 分成「🥬 食材 / 🧂 佐料」两类分节显示（佐料词表排除「蒜苗 / 洋葱」等易误判主料）；可勾选（`fm.shopping.<date>` 存本机）与复制；今日页「🛒 购物清单」入口。同样需先用 `getRecipe()` 展开配料，否则内置菜 `ingredients` 为空会算出空清单（实测踩过并修好）。
  - **清单图**：「🖼️ 生成清单图」用 Canvas 自绘暖米纸风格 PNG（720 逻辑宽 ×2 DPR，分组色条、圆角行卡、勾选框、纸纹点、落款），不引入任何图片素材；「📱 保存到相册」优先 `navigator.canShare({files})` + `navigator.share` 调系统面板（含「存储图像」），不支持时降级 `<a download>`，并提示长按预览图保存；预览图 CSS 允许 `-webkit-touch-callout` 长按。实测双视口生成 1440×1676 PNG、按钮可用、无 JS 报错。
- **菜谱卡片**：16 张菜系封面全部换成写真实拍风（1200×900 4:3），`.dish-photo img` 改 `object-fit: contain` 整图可见；曾短暂加过右下角「＋」快捷角标，因与底部「点这道」按钮功能重复已按用户要求移除，现在图片上只保留 📖 与 ☆ 两个轻角标。「🎲 选择困难？」推荐卡的 📖 / ＋ 则按用户要求改为**吸附在图片角上**（`top/right/bottom: -6px`，手机 -7px，超出热区由 `overflow:hidden` 裁掉）。
- **点菜去重（用户反馈「菜单里有重复」）**：排查后确认线上 3 条订单 ID 唯一、菜各不相同，**不是一次点击生成两条的技术 bug**；本地 16 条同菜订单是自动化验收反复点菜留下的测试数据。为杜绝同类观感问题，仍加了双重保护：`store.go AddOrder` 同一天同 `dishId` 幂等返回现有 plan；`App.vue actions.order` 提前检查并 toast「这道菜已经在今日菜单里啦」（返回 true 让详情弹窗正常关闭）。本地测试订单已清空；实测同菜连点两次订单数 1→1、换菜后 2，前端点击不新增且提示正常，api-test 14/14。

**上线验收**：`go vet` / `go test` / `vite build` 通过；`api-test` 14/14；CDP 双视口（1280/390）截图无布局问题；三个新弹窗与第一批 5 项双视口专项实测通过（主题/PWA 离线/避雷/购物清单/＋角标），无 JS 报错；16 张写实封面 HTTP 200；部署后线上 JS/CSS hash 与本地一致，健康检查 `{"ok":true}`。

---

## 三、剩余待办

### 0. 已完成：GitHub 发布
- 用本机缓存的 GitHub 凭据（`git credential fill`，身份 `1316195118`）通过 API 创建了**公开仓库** `1316195118/Home-Feast`，描述「家里吃点啥 · 家庭点菜系统：Go 标准库 + Vue 3，零依赖单二进制，纯 JSON 数据，Docker 一键部署」，`main` 分支已推送。
- `.gitignore` 排除了 `data/`、构建产物、AI 绘图中间产物、`.workbuddy/`，以及**含 PVE root 密码的 `scripts/_deploy.py`** 和抓取的原始素材；公开仓库无凭据、无第三方素材。
- README 顶部有中文名 + 优点速览表 +「30 秒部署」一条命令 + **完整可复制的 `docker-compose.yml`**；仓库根自带 compose 与 Dockerfile。

### 1. ✅ 已完成：PVE 从线上重新拉取并换掉旧目录（最近一次 2026-11-10 01:53，`6af6e74`）
- PVE 宿主 `10.10.10.252` 与 CT 100 **都直连不通 GitHub**（curl 返回 000、`git ls-remote` 超时），CT 内没法直接 `git pull`。
- 采用等价方式：本机从 GitHub `git clone` 最新 `main`（`6af6e74`）→ `git archive` 导出约 10MB 源码包 → 上传 PVE → CT 内 `docker compose down` → 备份并移出 `data/` → **删除旧 `/opt/family-menu`** → 解压线上源码 → 移回 `data/` → `docker compose up -d --build` → 健康检查通过。源码确实来自线上仓库，不是本地工作副本；数据全程只移出再移回。
- **推送注意**：某次 `git push` 报 `schannel: server closed abruptly`，用 `git -c http.version=HTTP/1.1 push origin main` 重试即成功；上线前务必用 GitHub API 核对 `main` 的 SHA 与本地一致，再跑部署脚本，否则会拉到旧版本。
- 线上核对：JS/CSS hash 与本地一致（`index-B2YOV6bN.js` / `index-C4dbfzGF.css`）；JS 含 `这道菜已经在今日菜单里啦`（点菜去重）且无 `dish-add` 残留，CSS 含 `-6px`（角标贴边）；`manifest.json` / `sw.js` / `/api/export/excel` / 16 张写实封面全部 200；`data` 完好，当天 3 条订单无重复（157 道菜 / 16 菜系 / 2 成员）。
- 若以后要 CT 内直接 `git pull`，需给 LXC 配代理 / 镜像加速。

### 2. ✅ 第一批已完成（2026-11-10）
深色主题、PWA 离线、Excel 导出、家庭口味避雷、购物清单 5 项已实现并双视口验收，详见第二节。

### 3. 105 道菜逐道写实图（范围待用户确认，尚未做）
- 现状：157 道菜里只有重庆菜 52 道有逐道写实图；其余 15 个菜系共 **105 道**仍显示各自菜系的封面（同一菜系共用一张图，截图上 5 道川菜是同一张）。
- 本轮只把 **16 张菜系封面**换成写真实拍（1200×900 4:3，约 80–160 积分），没有做逐道图。
- 若用户要的是「每道菜都像重庆菜那样各一张写实图」：需 105 张文生图（其它菜系没有 `scripts/_cq/base/*.png` 底图，不能照搬重庆菜「图生图换餐具」的流程），约 **525–1050 积分**；另需写批量发布脚本上传并 `PUT /api/dishes/{id}` 更新 image，建议每批 ≤3 张 + 断点续跑。
- **先确认范围**：只到菜系封面（已完成）还是要做 105 道逐道图。

### 4. 剩余批次（待确认优先级）

| 批次 | 需求 | 说明 / 风险 |
| --- | --- | --- |
| **二（新增数据模型）** | 每周掌勺排班与轮换提醒、按季节 / 冰箱食材智能推荐 | 需要新的 `roster.json` / `pantry.json` 与设置 UI；推荐建议用本地规则（季节 + 现有食材 + 辣度 / 耗时），不接外部 LLM 更稳妥 |
| **三（架构级，需先定方案）** | 多人实时同步 SSE、多家庭 / 多账簿隔离、自动化定时备份到 NAS 另一目录 | SSE 要后端事件流 + 前端 EventSource + 写冲突策略；多家庭隔离要引入租户 / 登录与数据分仓，**与当前「单家庭纯 JSON」模型冲突**，需先确认是否真的需要；定时备份属于部署侧 cron + 备份路径与保留策略，不进应用代码 |

**建议**：先确认第 3 节的图片范围，再做第二批；第三批逐项出方案确认后再动手。不要一次性全上，避免把已经稳定的单文件架构改坏。

### 5. 可选：真机试听音效
- 结果音（win/lose/safe）只验证了合成路径与代码分支，尚未在真实设备上试听音量与听感；让家人摇一局确认即可。

---

## 四、关键文件索引
- 骰子 PK：`web/src/components/DicePK.vue`（hydrate/persist/playResultOnce/restart）、`web/src/components/TodayView.vue`（`openPK`/`applyPK`）、`web/src/App.vue`（`saveDuties`/`saveDice`/`dutyName`）
- 后端 Plan：`internal/store/model.go`（`Plan`/`PlanPatch` + 骰子字段）、`internal/store/store.go`（`SavePlan`）、`internal/api/plans.go`（`updatePlan` 校验）
- 音效：`web/src/audio.js`、`web/src/main.js`（`installAudioUnlock`）
- 菜卡 / 详情：`web/src/components/DishCard.vue`、`web/src/components/DishDetail.vue`、`web/src/styles/screens.css`（「图片角标」段、`.detail-photo`）
- 图标 / 头像 / 动效：`web/src/utils.js`（`mealIconChoices`/`avatarGroups`）、`SettingsView.vue`、`FamilyView.vue`、`styles/tokens.css`、`styles/base.css`
- 图片：`scripts/_cq/remaining.json`、`manifest.json`、`publish.py`、`published.json`（52 项）、`gen_progress.json`
- 第一批：`web/src/theme.js`（深色）、`web/public/manifest.json` + `sw.js` + `icon-*.png`（PWA）、`internal/api/excel.go`（xlsx）、`web/src/components/ShoppingList.vue`（购物清单）、`web/src/recipes.js`（**内置菜配料的真实来源**，避雷/清单都要经过 `getRecipe()`）
- 验收：`scripts/_ui/capture.mjs`（CDP 双视口截图）、`scripts/api-test.mjs`
- 部署 / 发布：`scripts/_deploy.py`（本地，含凭据）、`scripts/fm_patch.tgz`、`Dockerfile`、`docker-compose.yml`、`README.md`、`NOTICE.md`

## 五、验证与部署流程（已跑通）
1. `go vet ./... && go test ./... && node scripts/build.mjs`
2. 启动本地 `menu.exe` → `BASE=http://127.0.0.1:8090 node scripts/api-test.mjs` → CDP 双视口截图（桌面 1280 / 手机 390）+ 弹窗 DOM 检查
3. `git add -A && git commit ...` → `git archive --format=tar.gz -o scripts/fm_patch.tgz HEAD`
4. `MSYS_NO_PATHCONV=1 python scripts/_deploy.py`（自动备份 data → 解压 → compose build → 健康检查）
5. 线上核对：`/api/ping`、首页 JS/CSS hash 与本地一致、抽查 `/uploads/*.webp` 200、`/api/bootstrap` 的带图菜品数
6. **从线上重新拉取并换掉 PVE 旧目录**：本机 `git clone https://github.com/1316195118/Home-Feast.git <临时目录>` → 从该 clone `git archive` 导出 `main` → 上传 PVE → 在 CT 100 里 `docker compose down`、备份并移出 `data/`、删除旧 `/opt/family-menu`、解压线上源码、把 `data/` 移回、`docker compose up -d --build`、健康检查。CT / PVE 宿主直连 GitHub 不通，因此用这个等价路径；数据目录全程不动（只移出再移回）。
