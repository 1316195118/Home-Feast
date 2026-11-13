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
- **仓库体积**：曾有一次 `git add -A` 把 `gen2/` 的 105 张生成原图（约 200MB）提交进历史，当前 HEAD 已通过 `.gitignore`（`gen2/`、`out2/`、`__pycache__/`）移除。为避免强推改写历史，统一用 **`git clone --depth 1`** 浅克隆（README 的一键命令与 PVE 部署脚本均已如此），实际下载约 10MB。
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
| **二（新增数据模型）** | ✅ 已完成：每周掌勺排班与轮换提醒、按季节 / 冰箱食材智能推荐 | 实际做法是把 `roster` / `pantry` 放进 `Settings`，复用 settings 读写、bootstrap 与备份链路，不新建 JSON 文件；推荐是纯本地规则（季节 + 现有食材 + 避雷 + 耗时 + 收藏），不接外部 LLM。详见第八节 |
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

---

## 六、本轮追加（2026-11-10 02:00–02:40，随本次部署上线）

- **157 道菜逐道写实图全部生成并发布**：`scripts/_cq/dishes_remaining.json` 列出 105 道缺图菜，`gen2/001..105` 是生成原图；加上原重庆菜 52 张，共 157 张。`publish_all.py` / `publish.py` 都支持断点续跑（`published_all.json` / `published.json`）。
- **图片归一化** `scripts/_cq/normalize.py`：自动识别盘子主体 → 以其中心裁 4:3 → 四周留约 11% 等距边距 → 桌面色调统一 → 羽化抹掉右下角平台水印。解决「只有半截 / 比例不对 / 有空白 / 盘子偏心 / 背景不一样」。输出统一 1200×900，前端 `object-fit: contain` 完整显示；详情页手机端容器也改为 4:3，不再留白。
- **菜系调整**：删除「重庆菜」分类，川菜改名「川渝菜」，原 52 道重庆菜并入；再把其中 36 道明显非川味的导入菜按常识归回家常菜 / 凉菜 / 主食 / 汤羹（川渝菜最终 24 道）。新增「新疆菜」分类（`xinjiang.jpg`）。
- **补齐菜品** `scripts/_cq/add_dishes.py`：新增新疆菜 18 道 + 川渝特色小吃 12 道，每道自带配料与 6 步做法（`getRecipe()` 优先用户自定义，不再走通用模板）。线上共 187 道菜。
- **做法与 UI**：详情页新增「下厨小贴士」；手机端配料表改两列卡片；按钮统一放大（桌面 42px / 手机 44px）；做法覆盖率实测 109/109 无缺失。
- **购物清单**：按「食材 / 佐料」分组且每组可一键全选；清单图**只画已勾选项**，未勾选不进图；「保存到相册」走 Web Share + 长按兜底。
- **点菜去重**：后端 `AddOrder` 幂等 + 前端提示拦截，线上 3 条订单无重复。

### 过曝事故复盘（2026-11-10 03:00，已修复并上线）
- **用户反馈「有些菜曝光了看不清」**：定位到 `normalize.py` 早期版本的 `_tone_shift()`，按四角平均色对整图做通道加法来「统一桌面色」，四角偏暗的图被整体提亮 **+42~88 灰阶**，高光像素占比从约 **0.1% 暴涨到 30~58%**，菜品大面积发白。
- 随后两版「统一背景」方案也各有副作用：椭圆羽化合成留下明显圆形光晕；纯色画布填充又把整图亮度拉高约 +47。**全部弃用**。
- **最终方案**：`normalize.py` 只做「识别主体 → 以主体中心裁 4:3（裁切框严格限制在原图内）→ 输出 1200×900」，不填色、不合成、不调亮度；水印只覆盖右下角 15%×8.5% 并羽化。实测亮度差 **-2.3~+2.6**、主体中心 **(598,448)**、高光占比回到 0~0.28%。
- 用户点名的「可乐鸡翅没有盘子、在桌子下」是**生成图本身构图错误**，用图生图重生成（强调白瓷圆盘、居中、完整可见），旧图已删。
- 线上抽检 6 张：尺寸均 1200×900，亮度 125~171，高光占比 0.01~0.5%，`/api/ping` 正常。
- **已知局限**：跨图桌面色调仍不完全一致（四角 RGB 各异）。靠后处理强行统一会再次引入光晕/过曝；要真正统一必须按重庆菜那套「统一浅木纹桌面 + 白瓷盘底图，图生图只替换菜品」重做，待用户确认是否投入约 105 张生成。

### 仍待做（下一批，需单独确认）
1. **成员可设置密码、登录/切换身份时输入**（用户 02:38 提出）：需 `Member` 存密码哈希、新增校验接口、FamilyView 设置入口、选人弹窗与顶部切换的密码校验链路；属于独立登录校验功能，未与这批视觉改动混做。
2. 30 道新增菜目前用各自菜系封面，若要「每道一张专属写实图」还需 30 张生成 + 发布。
3. 第二批（每周排班 / 季节智能推荐）、第三批（SSE / 多家庭隔离 / 定时备份）仍按第三节分批推进。

---

## 七、统一底图菜品图（2026-11-10 03:00–23:45，进行中）

**完整续跑说明见 `scripts/_cq/RESUME_UNIFIED.md`，下次直接照它做。**

- 用户确认的定稿方案：一张固定浅木纹桌面 + 居中白瓷盘底图（`scripts/_cq/base_unified/plate.png`）做图生图；**统一的是桌面 / 光位 / 45° 俯拍 / 居中构图 / 等距留白，器皿按菜性可变**；成品 `1600x1200`、`ZOOM=0.69`、`SHIFT_Y=-0.027`，盘子占画面宽约 66%、四周留白约 17–19%、上下等宽。
- **已生成并上线 13 / 105**（idx 001–013），`published_unified.json` 有记录，线上均为 1600×1200、亮度 126–169、高光 < 1%。
- **剩余 92 张（014–105）待生成**：图片生成额度用尽（HTTP 429，需到 `https://www.codebuddy.cn/profile/usage` 加量），用户要求「记住怎么生成，后面再生成」。
- ⚠️ **事故教训**：曾派 3 个子代理并行调用 `ImageGen`，其 `output_dir` 在并发下**串位**（返回路径落到别的编号目录），83 张图张冠李戴、33 个目录缺图，整批作废（残图备份在 Temp `fm-unified-broken/`，勿用）。**必须全局串行、统一输出到 `unified_raw/`，再用 `assign_raw.py` 按文件名时间戳归位**。已用 002/011/012/013 抽查验证串行 100% 可靠。
- 关键脚本：`build_unified_manifest.py`（105 项 prompt，含线上菜品 desc）、`assign_raw.py`（归位）、`publish_unified.py`（抹水印 + 居中裁 + 上传 PVE，断点续跑）。
- 处理原则仍是不调亮度、不填色、不合成（过曝事故见第六节）。

---

## 八、手机端布局修复 + 第二批功能（2026-11-10 23:45 – 11-11 00:20，已上线）

### 1. 菜系切换「画面会动一下」— 已量化修掉

用户反馈切菜系时画面晃动。定位到 4 个叠加原因，全部处理后用 `scripts/_ui/layout-audit.mjs` 量化复验：

| 原因 | 处理 |
| --- | --- |
| 菜谱网格 `TransitionGroup name="grid"` 的 0.3s 进出场 + move（`.grid-leave-active{position:absolute}`） | `MenuView.vue` 改回普通 `<div class="dish-grid">`；增删菜品是低频操作，稳定优先 |
| `.chip.active` 的 `result-pop` 缩放弹跳 | 只保留配色变化，删掉动画 |
| 页面滚动条出现/消失造成 10px 宽度跳 | `base.css` 加 `html { scrollbar-gutter: stable }` |
| 菜系条 `scroll-snap` 吸附 + 横向滚动条占位（桌面切菜系把网格顶下去 10px） | `scroll-snap-type: none`；横向滚动条改成不占位（`scrollbar-width:none` + `::-webkit-scrollbar{display:none}`） |

**复验**：桌面 / 手机切换菜系，视口宽度变化 0px、网格起点变化 0px、卡片位移 0px、残留动画类 0。

### 2. 手机端布局 — 已修

- 欢迎 toast 在窄屏换行成两行大绿条遮挡内容：文案加 `.toast-text { white-space:nowrap; overflow:hidden; text-overflow:ellipsis }`，手机端收紧成单行小胶囊（187×43 → 170×39）。
- 菜谱页「＋ 添加菜品」独占一行：`.section-head.menu-head` 改为标题与按钮同行。
- 漏网小按钮 `.wash-random` 34px → 40px；第二批新增控件统一 ≥44px 触摸热区。
- 双视口审计：5 个页签均 `溢出=0 / 小按钮=0`，无横向滚动。

### 3. 第二批功能 — 已完成

**设计决策**：排班与冰箱食材放进 `Settings`（`internal/store/model.go` 新增 `Roster` / `RosterDay` 类型与 `NormalizeRoster()`），复用 `PUT /api/settings` → bootstrap → 备份 zip 的既有链路，**不新建 `roster.json` / `pantry.json`**，旧 settings.json 零值兼容无需迁移。

| 模块 | 实现 |
| --- | --- |
| 每周掌勺排班 | 设置页 7 天 × 掌勺/洗碗下拉（跨两列宽卡）；后端校验只保留合法星期 key 与真实成员 ID；今日页在「排班开启 + 今天有安排 + 当前分工不一致」时提示并可一键套用，未填的那一半保留已定的人 |
| 智能推荐 | `utils.js` 新增 `seasonOf` / `rosterForDate` / `smartSuggest`；按季节 + 冰箱食材（经 `getRecipe()` 展开配料）+ 口味避雷 + 耗时 + 收藏本地打分，给出可读理由，换一批排除当前三道；**不接外部 LLM，数据不出本机** |
| 后端校验 | `internal/api/settings.go` 处理 `roster` / `pantry`：非法星期丢弃、不存在成员 ID 清空、食材去空白去重、单项 ≤12 字、最多 40 项 |

**验收**：`go vet` / `go test` / `api-test` 14/14；`scripts/_ui/batch2-verify.mjs` 双视口（1280 / 390）验证排班提醒、一键套用、推荐理由、换一批、设置页 7 行 14 下拉 20 个食材标签、横向溢出 0、过小控件 0、JS 异常 0；后端非法数据校验与持久化全部通过。

### 4. 仍待做（下一批）

1. ✅ **成员密码 / 切换身份校验** 已于 11-11 00:20 完成并上线，见第九节。
2. **统一底图菜品图 92 张**（idx 014–105）：额度用尽（HTTP 429）暂停，续跑方法见 `scripts/_cq/RESUME_UNIFIED.md`。
3. 第三批（SSE / 多家庭隔离 / 定时备份）按第三节逐项出方案。

---

## 九、成员密码 / 切换身份校验（2026-11-11 00:20，已上线）

用户 11-10 02:38 提出，作为独立登录链路单独做，没有和第二批视觉改动混在一起。

### 存储与边界（最关键）

- **绝不存明文**：`pins.json` 里每个成员只有 `{salt, hash}`，`hash = SHA256(salt + "\x00" + pin)`，salt 为 16 字节随机；比较用 `crypto/subtle.ConstantTimeCompare`。
- **不进 bootstrap**：`Member` 只多一个计算字段 `hasPin`（由 `ListMembers()` 按 pins.json 填充，不落盘），页面拿不到任何哈希或盐。
- **不进备份**：`internal/api/files.go` 的 `jsonNames` 白名单仍只有 4 份 JSON，`pins.json` 与 `access.json` 一样不进导出 zip、也不在导入白名单里；`Store.Reload()` 保持内存中的现有密码不变。
- 删除成员时同步清掉他的 pin 记录，避免孤立条目堆积。

### 接口

| 接口 | 行为 |
| --- | --- |
| `POST /api/members/{id}/pin` | 设置 / 修改 / 清除（pin 为空即清除）。已设密码时必须带对 `oldPin`，否则 400；长度 4–16 字符 |
| `POST /api/members/{id}/verify` | 校验，**失败也返回 200 + `{ok:false}`** |

> ⚠️ 这里有个必须记住的坑：前端 `web/src/api.js` 的 `request()` 把 **401 当作「访问口令错误」** 并派发 `fm:locked` 把整页锁掉。成员密码校验失败如果用 401，会误触发全局口令锁。所以校验接口统一 200 + `ok` 布尔。

### 前端链路

- `App.vue`：`pickUser()`（首次选人）与 `onSwitchUser()`（右上角下拉）都会先看目标成员的 `hasPin`；有密码就弹校验框，验证通过才真正 `selectUser`。切换下拉时先把 DOM 值复原，取消后身份不变。
- `FamilyView.vue`：成员卡片显示「🔒 切换身份需要密码」并提供「🔑 设密码 / 改密码」弹窗；修改需输当前密码，新密码两次确认，留空表示取消密码。保存后 `actions.refresh()` 拿最新 `hasPin`。
- 设置页「数据在哪」补了说明：`access.json` / `pins.json` 不随备份 zip 分发。

### 验收（`scripts/_ui/pin-verify.mjs`）

- 后端：设置成功、`bootstrap` 不含 `hash`/`salt`/明文、备份 zip 不含 `pins.json`/`access.json`、错密码 `ok:false`、对密码 `ok:true`、错误旧密码被拒、正确旧密码可改、最后清除并确认全员 `hasPin=false`。
- 双视口（1280 / 390）：首次选人弹密码框 → 错密码提示且身份不变 → 正确密码切换成功并记住 → 切到无密码成员正常 → 切回有密码成员弹框、取消后身份不变 → 家人页标识与按钮齐全；横向溢出 0、过小控件 0、JS 异常 0。
- `go vet` / `go test` / `api-test` 14/14 通过。

### 定位说明

这是**家庭内部的身份约定**，不是加密认证：应用本身默认开放写接口（与访问口令的定位一致），任何能直接调 API 的人仍可绕过 UI。目的是防止家人随手用别人名义点菜，别当账户系统用。

---

## 十、UI 调整 + HowToCook 数据评估（2026-11-11 00:45）

### 1. 本轮 7 项调整

| 需求 | 实现 |
| --- | --- |
| 排班卡改成「菜系显示」那种布局 | 去掉 `setting-card-wide` 跨列；排班表移到弹窗，卡片只留开关 + 「管理每周排班」按钮 + 「N/7 天已安排」状态 |
| 冰箱食材挪到菜谱标题后、设置页移除 | `MenuView.vue` 的 `menu-head-actions` 加「🧊 冰箱食材 N」按钮，弹窗里挑选；设置页对应卡片整块删除 |
| 食材和菜谱一样有效果图 | `pantryPresets` 改成 `{name, icon, tint}`；弹窗里 20 张自绘效果图卡片（渐变底 + 大 emoji，无第三方图片），自定义食材由 `pantryVisual()` 兜底配色 |
| 购物清单加重量 | `shoppingList()` 拆出用量并聚合：单项照抄（「200g」「2 个」「少许」），多来源同单位累加成「合计 2 勺」；列表、复制文本、Canvas 清单图三处同步显示 |
| 设置卡片统一 | 全部同宽、同 `padding:18px`、`min-height:218px`（手机单列取消 min-height），操作区 `margin-top:auto` 贴底 |
| 骰子换更明显的位置 | 从「🧽 谁洗碗」标题里移出，改成 `duty-grid` 上方独立 `.pk-cta` 条：图标 + 说明 + 「🎲 摇骰子决定」主按钮；手机端按钮占整行 |
| HowToCook 是否更好 | 见下节，**只评估未替换** |

### 2. HowToCook 评估（Anduin2017/HowToCook，master 实测）

调研方式：GitHub tree API 拉全量文件树（983 项、未截断），统计 `dishes/**/*.md` 并与本地菜名做精确同名比对；再抓宫保鸡丁、可乐鸡翅原文看格式。

| 维度 | 现有项目 | HowToCook |
| --- | --- | --- |
| 规模 / 分类 | 线上 187 道、16 个菜系 | **373 道**、10 类（荤菜 111 / 素菜 65 / 主食 59 / 水产 28 / 早餐 25 / 汤 24 / 饮品 23 / 甜品 19 / 半成品 10 / 调料 9），**没有菜系概念** |
| 与本地重合 | — | 本地 109 道中**精确同名仅 28 道**（宫保鸡丁、可乐鸡翅、麻婆豆腐、水煮鱼…） |
| 用量精度 | `recipes.js` 部分精确（「鸡腿肉 200g」「番茄 2 个」），仍有「少许 / 适量」 | **明确拒绝模糊量**，写死克/毫升（「生抽 10g」「可乐 500ml」），正好能补上购物清单的重量 |
| 做法详细度 | 内置菜统一 6 步模板；30 道新增菜各自写做法 | **7–21 步**，含火候、时间、可验证状态（「撇去浮沫」「挂丝」），明显更细 |
| 附加信息 | 辣度、耗时、标签、收藏、菜系、157 张自绘写实图 | 难度星级、卡路里、必备工具、附加内容；**图片风格不统一且属第三方素材** |
| 许可证 | 全部自写（NOTICE 声明无第三方素材） | **Unlicense 公共领域**，可自由使用，但接入后必须更新 NOTICE 注明来源 |
| 接入成本 | 已是结构化 JSON | Markdown 需写解析器：**新旧格式混用**（新版 `- 原料 = 用量`，旧版 `* 原料 用量`），还有 `～` 范围、「约」、步骤子项，需要兼容 + 单测 |

**结论：不建议整体替换菜谱数据**。会丢掉 16 菜系体系、157 张专属图、辣度/耗时/避雷标签/收藏，373 道无图菜反而让体验倒退，且要重做配图与 NOTICE。

**结论（已执行，2026-11-11）**：用户确认「用量导入」后，只做了「精确用量补充」——解析 23 道同名菜的「计算」段（28 道同名里 5 道变量公式/单项不足被过滤），结果内嵌为 `recipes.js` 的 `HOWTO` 常量，`getRecipe()` 命中时整表替换模板配料（步骤仍用本项目自写模板）。购物清单用量从「少许/适量」变成「盐 2 g / 生抽 15ml」。解析脚本 `scripts/_cq/parse_howto_amounts.py`、来源声明 `NOTICE.md` 第 3 节。**没有**替换做法步骤、菜系、图片或整体菜谱数据。进阶做法折叠展示仍未做，非必需。

### 3. 验收

- `go vet` / `go test` / `api-test` 14/14；新增 `scripts/_ui/round3-verify.mjs`。
- 双视口实测：骰子 CTA 独立且不在标题里；购物清单 11 行带用量（`200g`、`1 小勺`、`少许`…）；菜谱标题后食材入口 → 20 张卡全带效果图 → 点选保存后 `pantry` 生效；设置页 9 张卡宽度只有 **1 种**、桌面统一 `min-height:218px`、无跨列大卡；排班弹窗 7 行 14 下拉；横向溢出 0、过小控件 0、JS 异常 0。
- 第二批 / 成员密码 / 菜系切换 0 位移等既有验收回归通过。

---

## 十一、优先推荐 + 全站 UI 审计 + 第三批（2026-11-11 01:10）

### 1. 「优先推荐」

菜谱工具条「只看收藏」后新增「🥬 优先推荐」：`utils.js` 的 `pantryMatches()` 只按**主料**匹配冰箱食材，佐料（盐/油/葱蒜/酱油…，`shoppingKind` 判定）两层过滤；MenuView 按命中数降序稳定重排（相同命中保持菜谱原顺序），卡片显示「🧊 N 样现成」。今日页 `smartSuggest()` 复用同一函数，勾选食材后两处一起实时变化（state.settings.pantry 是响应式的）。开关记在本机 `fm.priorityPantry`。

### 2. 全站 UI 审计（`scripts/_ui/full-audit.mjs`，双视口量化）

| 发现 | 处理 |
| --- | --- |
| 菜谱页 109 个编辑按钮 40×40 偏小 | `.icon-btn` 统一 44×44；审计后「过小目标」从 109 降到 **0** |
| 输入控件高度/字号参差（31/36/46px，手机 13–14px） | `.input/.select/.textarea` 统一 `min-height:42px` / `15px`；手机端统一 **44px / 16px**——iOS Safari 在输入字号 <16px 时会自动放大整页，这是关键修复。排除 checkbox/radio/hidden/file 避免被撑大 |
| 泛化按钮文案「保存」「开启」自解释性差 | 改为「保存留言 / 保存名字 / 保存成员 / 保存密码 / 保存食材 / 保存时段 / 开启口令」 |
| placeholder 风格混用（斜杠、括号、省略号） | 统一为「例如：…」句式，如「给今天留句话，例如：想吃点清淡的、冰箱里有排骨…」 |
| 排班卡片和弹窗都有「保存排班」 | 卡片只留「管理每周排班」入口，开关与保存全部收进弹窗，消除重复 |
| 刷新会覆盖正在输入的留言草稿 | `TodayView` 的 note watch 加 `noteDirty` 保护：实时同步 / 本机保存触发的刷新都不覆盖未保存草稿 |
| 设置卡片尺寸 | 同宽、同 padding、桌面统一 `min-height:218px`；实测宽度只有 1 种、无跨列大卡 |

**验收脚本基线校准**：`layout-audit.mjs` 原先从「设置」切回「菜谱」后只等 700ms 就取基线，把页签进出场动画的 10px 位移误算成「切菜系抖动」。改为等 1100ms + `scrollTo(0,0)` 后，桌面/手机实测均为 **0px / 0px / 0px**；诊断脚本另证切菜系前后 `dish-grid.top` 恒为 350。

### 3. 第三批

| 模块 | 实现 |
| --- | --- |
| **SSE 多人实时同步** ✅ | `internal/api/events.go`：服务端只广播版本号（不含业务数据），25s 心跳；`protect()` 用 `statusWriter` 只对 **2xx 写操作**广播。前端 EventSource 收到后防抖 350ms 再 `refresh()`；正在输入 / 页面后台 / 有写操作在飞时挂起，失焦或回前台再补刷新；`lastLocalRefresh` 忽略本机写操作后的 SSE 回声。`/api/events` 与 ping/config 一样跳过访问口令校验（EventSource 不能带自定义头，且事件只有版本号）。设置页显示 🟢/🟡 连接状态 |
| **定时备份** ✅ | `internal/api/autobackup.go`：每天 03:30 把 4 份 JSON + uploads 打包到 `data/backups/`，按 `BackupKeep` 保留（默认 7，上限 60）；旧 settings.json 首次升级自动开启。`GET /api/backups` 列表、`POST /api/backups/run` 立即备份；设置页有开关、一键备份和最近 3 份记录。**设 `MENU_BACKUP_DIR` 可把备份写到 NAS 上独立挂载目录**，与运行数据物理分开 |
| **多家庭隔离** ⏸ 第一阶段地基已做，分仓待确认 | 已加 `Settings.FamilyID`（默认 `default`）、bootstrap 下发 `familyId`、备份文件名带 `familyId` 命名空间，**不动任何现有数据**。第二阶段方案见下 |

**多家庭隔离第二阶段的三个可选路径**（按风险从低到高）：

1. **推荐：一家庭一容器**。不同端口 + 不同 `data` 挂载 + 不同 `MENU_ACCESS_CODE`，零代码改动、隔离最彻底，已有 `familyId` 可直接用作备份区分。
2. **同实例分目录**：`data/families/{familyId}/` 存各自 4 份 JSON + uploads，需改 `Store.Open()` 支持按请求选目录、加登录/家庭切换、写迁移脚本把现有数据挪到 `default/`，并把 `fm.user` 等本机键加上 familyId 前缀。改动面覆盖全部读写链路。
3. **同份数据加 tenant 字段**：所有实体加 `familyId`，查询按租户过滤。改动最大且历史数据都要回填，不建议。

> 这一项**故意没有直接分仓**：与当前「单家庭纯 JSON」模型冲突且不可逆，需要用户先确认是不是真的多家庭共用一套部署。地基已经留好，确认后再按路径 1 或 2 推进。

### 4. 验收

`go vet` / `go test` / `api-test` 14/14；新增 `full-audit.mjs`、`batch3-verify.mjs`。实测：佐料过滤单测只命中「鸡蛋」；优先推荐在另一设备改食材后经 SSE 自动重排（21 张卡带命中标签、前 4 张 2/2/2/1 样）；另一设备点菜后本页订单 **0→1 自动同步**；远端写入时未保存草稿完整保留；立即备份生成 zip、`familyId=default`、`autoBackup=true`；桌面/手机设置页卡片与输入字号（15px / 16px）符合预期；横向溢出 0、过小控件 0、JS 异常 0。第二批、成员密码、菜系切换 0 位移等既有验收全部回归通过。

### 5. 仍待确认

1. 多家庭隔离走路径 1（一家庭一容器）还是路径 2（同实例分目录）。
2. ~~HowToCook 精确用量导入~~ 已完成（见第十节结论）。
3. 92 张统一底图菜图（额度恢复后按 `RESUME_UNIFIED.md` 串行续跑）。

## 十二、用量导入 + 手机首页改造 + 设置卡片重排（2026-11-11 01:20）

用户三项诉求：① HowToCook 用量导入 ② 手机端首页仍不合理 ③ 设置卡片顺序不合理。

### 1. 精确用量导入

- `scripts/_cq/parse_howto_amounts.py`：抓 28 道同名菜原文（缓存 `Temp/howto-md/`），只解析 `## 计算` 段的列表行，支持三种写法（`* 食材: 量` / `- 食材  量` / `- 食材 = 量`），去括号描述、去「份数×N」变量公式，最终 23 道 200 项（5 道因变量公式/单项不足被过滤）。
- 结果内嵌为 `web/src/recipes.js` 的 `HOWTO` 常量。`getRecipe()` 在用户无自定义配料且菜名命中时，用精确配料整表替换模板配料（`precise:true`），做法步骤仍用本项目自写模板；用户自定义配料永远优先。`getRecipe()` 返回新增 `precise` 字段。
- 购物清单同步显示「📏 N 道菜用量已精确到克/毫升」提示条（`round3-verify.mjs` 覆盖）。
- 佐料归类词补 `藤椒油/菜籽油/郫县豆瓣酱/白胡椒粉/花椒粉/辣椒粉/鱼露/芥末/白醋/地瓜粉/开水`，保证购物清单分组与「佐料不参与推荐」一致。
- 来源：HowToCook 为 Unlicense 公共领域，已在 `NOTICE.md` 第 3 节声明。**未替换**菜谱步骤、菜系、图片或整体数据。

### 2. 手机首页（今日页）改造

用户明确选了三处：骰子 CTA 太大太靠前、分工区太占高度、今日菜单看不到；另指出「去点菜 + 购物清单」被挤成两排。

- 骰子 CTA 从「图标 + 两行文字 + 整行大按钮」收紧成单行：按钮与标题同行、说明文字手机隐藏、按钮改「摇骰子」（`pk-cta-hint`）。
- 分工卡手机端改为两列并排、成员只显头像（≤430px 隐藏名字）、保留 44px 热区，大幅降低 hero 高度（页面总高 1515→1289px）。
- 新增「今日菜单速览」条：有菜时首屏显示「🍽️ 今天已点 N 道 · 前几道菜名 + 查看菜单 ›」，点击平滑滚动到菜单面板（`orderPreview` / `scrollToMenu` / `.today-peek`）。
- 手机隐藏与统计条/速览重复的 `hero-sub`。
- `.panel-head-actions` 改 `flex-wrap:nowrap`，「🛒 购物清单 / 去点菜 ＋」不再换行成两排。

### 3. 设置卡片顺序

给 10 张卡加语义 class（`sc-name` … `sc-meals`），用 CSS `order` 按使用动线重排（键盘 Tab / 读屏顺序仍按原 DOM，连贯）：
家的名字 → 用餐时段 → 菜系显示 → 每周排班 → 外观 → 访问口令 → 数据备份 → 自动备份 → 数据在哪 → 关于与版权。备份三卡相邻，高频基础设置前置。

### 4. 验收

`go vet`/`go test`/`api-test` 14/14；`round3-verify`（购物清单用量 `1 支/150g/5g/15g`、精确提示、骰子 CTA、设置卡 10 张同宽）、第二批、第三批（SSE 0→1、备份、优先推荐）、成员密码、菜系切换 0 位移全部回归通过；横向溢出 0、过小控件 0、JS 异常 0。桌面/手机截图目检确认新布局。

## 十三、分工选择改版 + 相似菜排查（2026-11-11 01:45）

### 1. 掌勺 / 洗碗改成两个选择

用户要求「谁掌勺和谁洗碗改成这两个选择，选中给提示或动画，一眼看懂」。

- **第一版理解偏了**（做成「先点角色 tab、再从成员里选人」两步），用户纠正：「这两个按钮就是当前用户点谁就是它」。**最终版改成一键认领**。
- `TodayView.vue`：原并排两栏（`.duty-grid` / `.chef-choice` / `.wash-choice`）改为两个大按钮 `.duty-claim`——「👨‍🍳 我来掌勺」「🧽 我来洗碗」。**当前身份点一下就认领该角色，再点取消**；别人已认领时点它会接管（改主意）。按钮上显示「是你 · 再点取消」或「XX 已认领」，右上角 ✓ 标记当前用户认领的角色。新增 `dutyPulse`、`claimRole()`（`selfMember` 为认领主体，`setTimeout` 2.4s，`onUnmounted` 清理），动画反馈「😀 XX 来掌勺啦！」/「已取消掌勺」。掌勺红、洗碗绿。
- `duty-pop` 过渡（pop-spring 进、淡出）。涟漪/焦点/点击态选择器同步换成 `.duty-claim`（`ripple.js`、`base.css`、`full-audit.mjs`）。骰子 PK 保留（一次定两人）。
- 验收 `scripts/_ui/duty-verify.mjs`（重写为认领版）：点掌勺→认领+动画、点洗碗→认领+动画、再点掌勺→取消，按钮状态/JS 异常 0 全过，测完恢复原分工。

### 2. 相似菜排查（只读，未改动任何菜）

`scripts/_cq/` 之外的一次性分析（`Temp/fm-find-similar.py`）：187 道菜名做别名归一 + difflib 字面相似 + 去做法词取主料，得 36 对，人工分级：

- **真·同菜异名，建议后续处理（未擅自动手）**：
  - `番茄炒蛋` / `西红柿炒鸡蛋`（内置 vs 自建，desc 几乎一样）
  - `凉拌黄瓜` / `拍黄瓜`（内置 vs 自建，desc 都说「拍碎」）
  - `玉米排骨汤` / `玉米炖排骨`（内置 vs 自建）
- **同主料不同做法，属正常菜谱多样性，不建议合并**：可乐/香辣/蒜香鸡翅；红烧/蒜香/醉排骨；番茄蛋汤/紫菜蛋花汤/番茄鸡蛋面；爆炒小龙虾/爆炒龙虾尾；凉拌/拍黄瓜外的凉拌三丝等；清蒸鲈鱼/糖醋鲤鱼/水煮鱼（不同鱼不同做法）；卤牛肉/泡椒牛肉等。
- 番茄炒蛋系列在「优先推荐」里同时出现（番茄炒蛋/番茄蛋汤/番茄鸡蛋面），是主料相同而非数据重复。

**没有自动删除或合并任何菜**：删除会影响历史订单引用与图片，需用户逐组确认后再定（可加「别名」字段做展示合并、或直接删自建副本）。

## 十四、备份合并与手机端细节优化（2026-11-11 02:10）

用户四项：① 自动备份+数据备份合并 ② 骰子弹窗贴顶 ③ 输入框提示显示不全 ④ 手机端两面板副标题第二排孤字。

### 1. 备份两卡合一

`SettingsView.vue`：删掉 `.sc-autobackup` 卡，把自动备份全部内容（每天开关、立即备份、最近记录、保留份数）并入 `.sc-backup` 卡，标题改「💾 备份与恢复」，标签显示「自动备份中 / 仅手动」。手动导出 zip/Excel、从备份恢复也保留在同一卡。设置卡从 10 张变 9 张；CSS `order` 改为连续（备份 7 → 数据在哪 8 → 关于 9）。`.backup-merged` 新样式让按钮区可换行、内容更紧凑。

### 2. 骰子弹窗贴顶

手机端所有弹窗是底部抽屉（`.modal-backdrop align-items:flex-end`）。骰子 PK 内容多、标题贴抽屉上沿。加 `.pk-modal { max-height:78vh; padding-top:26px }` + `.modal-head { padding-top:2px }`，顶部留白更足、离屏幕顶部更远。

### 3. 输入框提示显示不全

审计确认所有 input/textarea 的 `scrollWidth == clientWidth`（无横向溢出），问题是**过长的 placeholder 在窄输入框里被省略**：留言框「给今天留句话，例如：想吃点清淡的、冰箱里有排骨…」改成「留句话，例如：想吃点清淡的…」，手机上一眼看得全。

### 4. 手机端副标题孤字

量化审计：手机端「今日菜单」副标题容器仅 **119px**（被右侧「购物清单/去点菜」两按钮挤压）、「今天吃啥」228px，都被挤成 2 行、第二排只剩孤字。修复：手机端 `.panel-head { flex-wrap:wrap }`、标题区 `flex:1 1 100%`（占满整行），按钮组移到标题下方一行（两按钮仍同行，符合用户此前要求），加 `.section-sub { text-wrap:balance }`。复验：今日页多行副标题 **0 条**，两处都一排完整显示。

### 5. 重复自建菜删除（用户确认「删自建副本」）

删除 3 道与内置重复的自建菜：`西红柿炒鸡蛋`（保留 番茄炒蛋）、`凉拌黄瓜`（保留 拍黄瓜）、`玉米炖排骨`（保留 玉米排骨汤）。删前确认 0 历史订单引用；内置版各自已有独立图片，故自建版的 3 张孤儿图（`/uploads/imgc0dd…`、`img41d2…`、`img12e7…`）也一并从容器清除。菜数 187→184，剩余图片 850 张，删后做了一次即时备份（125.2 MB）。**注意**：线上 `familyName=小小家`、隐藏 6 菜系、pantry 9 样是用户真实配置，未改动；仅清理了被验收脚本污染的本地 `data/settings.json`。

### 6. 验收

`go vet/test`、`api-test` 14/14；分工认领、round3（设置卡 9 张同宽）、第二批、第三批（SSE 0→1、备份）、成员密码、菜系切换 0 位移全回归通过；横向溢出 0、过小控件 0、JS 异常 0。

## 十五、config 与 data 分离 + 全站布局再优化（2026-11-11 02:30）

用户：① 用户数据放 data、其他放 config ② data 不包含 config ③ 电脑端布局/窗口过长/提示动画反馈/删冗余/冰箱食材太少要分类 ④ 今日菜单与今天吃啥「我就要一排」 ⑤ 重新优化所有布局、卡片大小、补缺失信息。

### 1. config 独立目录（与 data 平级，非嵌套）

先误做成 `data/config/settings.json`，用户纠正「data 不包含 config」，改为独立顶层 `config/settings.json`。

- `store.go`：Store 加 `configDir` 字段；`pathOf(name)` 方法让配置类文件（仅 settings.json）进 configDir，其余进 data。`Open(dir, cfgDir)` 接收两个目录。迁移旧位置：`data/settings.json` 与 `data/config/settings.json` → `config/settings.json`。
- `cmd/menu/main.go`：加 `-config` 参数与 `MENU_CONFIG_DIR` 环境变量，默认 `config`，启动日志同时打印数据/配置目录。
- 备份 zip 里 settings.json 保持**扁平文件名**（兼容旧备份与旧部署），导出从 `s.st.Path()` 读、导入用 `filepath.Base()` 落到 config；`backupAllowed` 兼容 `config/settings.json`。`autobackup.go` 打包同样改用 `s.st.Path()`。
- Docker：`Dockerfile` VOLUME 加 `/config`、CMD 加 `-config /config`；`docker-compose.yml` 挂载 `./config:/config`。部署脚本 `fm-pve-online-deploy.py` 同步保护 config（备份/移出/移回/回滚/核对）。
- `.gitignore` 忽略 `/config/*`（保留 .gitkeep）。

### 2. 全站布局优化

- **设置卡**：去掉统一 `min-height:218px`，改内容自适应（桌面仍同行等高，但不再大片空白）；桌面设置页总高 1360→1095px。
- **冰箱食材**：20→70 种，按 7 类分组（🥩肉禽/🦐水产/🥬蔬菜/🍄菌菇豆制品/🍚主食/🍎水果/🥛蛋奶），每组彩色左边框标题；面板顶部新增实时反馈「已选 N 样·菜谱 X 道用得上」（带 pop 动画）；弹窗 max-width 820/88vh 可滚动。
- **今日页 hero 压缩**：padding 24→18、各块 margin 收紧、骰子 CTA/认领按钮/留言区间距缩小；删与骰子 CTA 重复的说明文案；桌面认领按钮从各占 50% 改为 `flex:0 1 320px`（手机仍等分）。桌面总高 1199→1095、手机 1447→1321px。
- **今日菜单/今天吃啥副标题**：手机端 `.panel-head` 标题区占满整行、按钮下一行（两按钮仍同行）+ `text-wrap:balance`，副标题一排无孤字（量化：今日页多行副标题 0 条）。

### 3. 验收

`go vet/test`、`api-test` 14/14；配置迁移实测（3 个旧位置→config）、备份往返（扁平名→config 落点）、分工认领、round3、第二批、第三批、成员密码、菜系切换 0 位移全回归通过；横向溢出 0、过小控件 0、JS 异常 0。

**说明**：截图里本地菜谱卡多道菜图片相同是「本地 data 无图片数据→菜系封面兜底」所致；线上每道菜有独立 `/uploads/img*.webp`，非 bug。

## 十六、全站窗口与重复功能再优化（2026-11-12 15:20）

用户要求：检查手机端窗口布局、桌面骰子 PK 弹窗过长、重新优化所有页面、功能优化、重复的都优化掉。

### 1. 骰子 PK 弹窗（重点）

量化：桌面 548px / 手机 498px，4 处重复文字。

- `DicePK.vue`：说明两行→一行；删「正在摇」整行（成员行已高亮当前人 +「你」标签）；每行「点右上角『我是』切到 X」→ 极短「待摇」；底部状态文案精简。
- CSS：`padding 22→18`、`max-width 540→500`、行 `padding 11→8`、骰子 `58→46px`、按钮 `124→104px`、list `gap 10→8`、结果/等待区 margin 收紧；删 `.pk-who*` / `.pk-hint-other` 无用样式；手机 `max-height 78→82vh`、`padding-top 26→20`。
- **结果：桌面 548→386px（-30%）、手机 498→414px**，视觉重心集中、无重复提示。

### 2. 各页重复内容清理

| 位置 | 重复/过时 | 处理 |
|---|---|---|
| 今日页 | 标题问句 + 骰子 CTA 问句 + duty-summary 三处说「定分工」，CTA 里 emoji 与左侧图标重复 | CTA 改「摇骰子，一把定分工」、去掉文案内 emoji；duty-summary 空态精简为「分工还没定」 |
| 设置页 | 顶部说明「只管名字、备份和说明」已过时 | 改为「家庭信息、菜系、时段、排班、备份与访问口令」 |
| 设置页「数据在哪」 | 列旧 `settings.json`（已移 config）、与「访问口令」卡重复「不进备份 zip」、4 段说明 | 改名「数据与配置在哪」，列 `data/` 与 `config/` 两个目录，删重复段、压成 2 段 |
| 设置页「关于与版权」 | 两段长文占整卡 | 保留一句摘要 + `<details>` 折叠完整说明（含 HowToCook 授权） |
| 菜谱页 | 副标题 141px 被挤成 3 行；「共 N 道」与「全部 N」重复；0 道菜系仍醒目 | 手机标题区占满整行、按钮下一行；副标题精简一排；`共 N 道` 改为筛选时才显示「找到 N 道」；0 道菜系 `opacity:0.5` 弱化 |
| 手机「家的名字」 | 输入框与保存按钮分两行 | 手机取消强制换行，输入框自适应 + 按钮同行 |

### 3. 功能优化

- 冰箱食材弹窗（70 种 / 7 类，内容 2200+px 长滚动）新增：**搜索框**（实时分组过滤）、**7 个分类快捷跳转 chip**（点击清空搜索并滚动到该组）、**清空已选**按钮、无结果提示。
- 顶部「已选 N 样·X 道用得上」实时反馈保留在说明下方（打开即见，不用滚到底）。

### 4. 验收

新增 `scripts/_ui/interaction-verify.mjs`：食材搜索「排骨」精确 1 项、分类跳转清空搜索、清空已选反馈归零、骰子弹窗 386px 且无「正在摇」/「点右上角切到」、关于折叠默认收起、JS 异常 0。
`go vet/test`、`api-test` 14/14、分工认领、round3、第二批、第三批（SSE 0→1）、成员密码、菜系切换 0 位移全回归通过；双视口溢出 0、过小控件 0。手机设置页 2353→2145px。

### 5. 部署脚本 config 嵌套 bug（本次发现并修复，重要）

首次重建后核对发现 `[config 配置完好] fm-config-xxx settings.json`：部署脚本用 `mv` 整个目录移出/移回 config，而 Docker Compose 挂载会**自动创建空 config 目录**，`mv` 到已存在目标时把源目录塞了进去，形成 `config/fm-config-xxx/settings.json` 嵌套。结果应用读到新建的默认配置（「我们家」/ pantry 空 / 隐藏菜系 0），**真实配置被挤进嵌套目录**。

- 立即恢复：把嵌套目录里的 `settings.json` 复制回 `config/settings.json`、删嵌套目录、`docker restart`，验证真实配置全部回来（小小家 / 9 样 pantry / 6 隐藏菜系 / 184 道菜）。
- 修脚本：`fm-pve-online-deploy.py` 的「移出/移回 config」改用 `cp -a src/. dst/` **内容合并**（先 `mkdir -p`），回滚分支同步；不再用 `mv` 目录。重跑部署验证：config 目录只剩 `settings.json` + `.gitkeep`，配置完整保留。
- **教训**：部署脚本里凡是对可能由 compose 自动创建的挂载目录做移动，一律用内容复制而非目录 mv。

### 6. 收尾小修

- 线上核对发现食材搜索时「自定义食材」组未跟着过滤 → `pantryCustomCards` 也按 `pantryQuery` 过滤；无结果提示同时判断两组。
- 「葱 / 蒜 / 姜」加入蔬菜预设（73 种），用户 pantry 里的葱蒜不再落在自定义组。佐料仍不参与推荐（`shoppingKind` 过滤），不受影响。

## 十七、即时反馈与动效增强（2026-11-13 15:40）

用户：「看看还有什么可以优化的地方，或者可以加动画、特效的地方」。

### 1. 审计结论

现有基础已很扎实：`base.css` 有 20+ keyframes（fade/pop/sheet/row-in/dice-shake/result-pop/check-pop/empty-float/shake-error/success-bounce/count-pop/img-reveal…）、`ripple.js` 点按涟漪、`audio.js` Web Audio 合成音效、`prefers-reduced-motion` 统一降级、toast、`value-pop` 统计弹跳、`star-pop` 收藏、列表 TransitionGroup。

**缺口**：`is-invalid` / `feedback-success` 两个反馈类定义了却从未被调用；`img-reveal` 只用在 DishCard，今日页两处图片没淡入；主题切换瞬间换色无过渡；缺少点击的轻量视觉特效。

### 2. 本轮做的 7 项

| 项 | 实现 |
|---|---|
| 反馈工具 | 新增 `web/src/feedback.js`：`shake()` / `bounce()` / `sparkle()` / `sparkleFromEvent()` |
| 校验抖动 | 5 处（菜名、家人称呼、家的名字、访问口令、餐次名/时间）原本只有 toast，现在输入框 `is-invalid` 抖动描红 |
| 保存弹跳 | 设置页保存名字成功后 `feedback-success` 轻弹（用 guard 返回的布尔值判断） |
| 点击飘字 | 点菜 🍚、认领掌勺 👨‍🍳 / 洗碗 🧽 → 3 个小图标上浮淡出 0.9s |
| 选中✓弹跳 | 食材卡 `.pantry-check` 加 `check-pop` |
| 图片淡入 | 今日页订单缩略图与推荐卡图补 `.img-fade` |
| 主题过渡 | `theme.js` 的 `setThemeMode()` 调 `applyTheme(true)`，短暂挂 `html.theme-switching`（380ms）只给主要容器加配色过渡；首次加载不触发避免开屏闪烁 |

`index.html` 新增 `#fx-layer` 浮层（Vue 不托管的兄弟节点），`tokens.css` 在减少动效偏好下直接隐藏浮层。

### 3. 踩坑与定案（重要，别再重复）

1. **往 `document.body` 动态 append/remove 飘字节点会与 Vue 渲染竞态**：验收出现 `TypeError: Cannot read properties of null (reading 'insertBefore')`。
2. 改成给触发元素挂 class + `::after` 也不行：Vue 的 `:class` 绑定会把手动加的类覆盖掉，且异常仍在。
3. **最终方案：`#fx-layer` 里预置 6 个固定 `<span class="sparkle">` 节点池**，运行时只改 `left/top/animationDelay`、CSS 变量 `--fx-emoji` 和 `is-on` class，**零 DOM 增删**。对比脚本异常归零。
4. **headless Chrome 默认 `prefers-reduced-motion: reduce`**，会把所有反馈类关掉，验收必须 `Emulation.setEmulatedMedia({features:[{name:'prefers-reduced-motion',value:'no-preference'}]})`，否则误判"动效没生效"。
5. **shell 优先级坑**：`node build.mjs && ./menu.exe &` 会把整条链放后台，启动的是**构建前**的二进制；且残留 `menu.exe` 会占住 8090，导致所有验收打到旧版本。必须分步执行、启动前 `taskkill` 残留进程、并核对服务端 JS hash 与 `webapp/dist` 一致。
6. **残留的 `insertBefore of null`**：用 `git stash` 检出上一版（未加任何动效）跑**同一份**验收脚本，同样出现 3~7 条，堆栈完全一致 → 属既有 Vue `<Transition>` 在"动画未结束就切页"极限时序下的竞态，正常操作 0 异常、不影响功能。`fx-verify.mjs` 已把它单独归类，只把其它异常计为回归失败；页面切换本身已是 `mode="out-in"`（最稳结构），未擅自改框架层。

### 4. 验收

新增 `scripts/_ui/fx-verify.mjs`（温和时序）：预置节点 6、认领/点菜飘字各 3、主题过渡类出现并移除、保存弹跳 true、两处校验抖动 true、食材 ✓ `check-pop`、图片淡入、网格起点 14→14、`真实 JS 异常 0`。
`go vet/test`、`api-test` 14/14、interaction/duty/round3/batch2/batch3(SSE)/pin/layout 全回归通过；双视口溢出 0、小控件 0、菜系切换 0 位移。

## 十八、彩带 / 主题圆形扩散 / 震动三项特效 + 版权声明（2026-11-13 16:00）

用户：「除了第一个（菜谱卡滚动入场淡入）其他都做，然后注意版权」。三项全部实现，且**不引入任何新的第三方依赖或素材**。

### 1. 点菜成功彩带

- `index.html` 的 `#fx-layer` 在原有 6 个 `.sparkle` 之外**预置 12 个 `.confetti` 节点**（延续上一节「零 DOM 增删」原则，杜绝 Vue `insertBefore` 竞态）。
- `feedback.js` 新增 `celebrate(x, y)`：批量移除 `is-on` → **只强制重排一次** → 批量设置 `--cx/--cy/--cr/--cc` 与 `animationDelay` → 挂 `is-on`。12 个色块按圆周角抛洒、带重力下坠并旋转，1s 淡出。
- `base.css` 的 `@keyframes confetti-burst`：纯 CSS 纯色小矩形（6×10px、2px 圆角），颜色取自项目自有品牌色系数组，**无图片 / 贴纸 / GIF / Lottie / 图标包**。
- 触发时机改为**真正成功之后**：`DishCard.onOrder` / `TodayView.quickOrder` 改成 `async`，先 `sparkleFromEvent` 做点击即时反馈，再 `await actions.order()`，仅当返回真值才 `celebrate()` + `vibrate(16)`。失败只有 toast，不会误撒彩带。

### 2. 深色主题圆形扩散

- `theme.js` 新增 `circularThemeSwap(anchor, swap)`：浏览器原生 **View Transitions API** 的 `document.startViewTransition(() => applyTheme(false))`，在 `vt.ready` 后用 **Web Animations API** 对 `::view-transition-new(root)` 做 `clip-path: circle(0px) → circle(Rpx)`，`R = hypot(max(x, W-x), max(y, H-y))`，520ms ease-in-out。
- `base.css` 关掉 View Transitions 自带的 cross-fade（`animation:none; mix-blend-mode:normal`），否则新旧两层叠成灰雾；给 old/new 指定 z-index。
- `SettingsView.chooseTheme(mode, $event)` 把点击坐标传进去，扩散从按下的位置开始。
- **降级链**：`prefers-reduced-motion: reduce` / 浏览器不支持 `startViewTransition` / 无坐标（如"跟随系统"自动切换）→ 回落到原来的 `theme-switching` 平滑配色过渡；圆形扩散时不叠加 `theme-switching`，避免两套动画打架。

### 3. 手机轻震动

- `feedback.js` 新增 `vibrate(pattern)`：浏览器原生 **Vibration API**（`navigator.vibrate`），`try/catch` 静默降级；尊重系统「减少动效」偏好，**并跟随站内音效静音开关**（`audio.js` 的 `isMuted()`），用户关了音效就不会再震。
- 触发：认领掌勺/洗碗 `vibrate(12)`、点菜成功 `vibrate(16)`、骰子摇动 `vibrate(12)`、骰子结果揭晓 `vibrate([12,35,12])`（结果沿用 `pkShown` 每人只播一次，不重复震）。

### 4. 版权声明（用户特别要求）

`NOTICE.md` 两处补充：
- 第 2 节明确「除表内依赖外没有引入任何其它库」，三项特效均为浏览器原生 Web API 或自写 CSS，未复制任何第三方动画库 / 动效模板 / 代码片段。
- 第 4 节新增 4 条：飘字仅 Unicode emoji、彩带纯 CSS 色块、圆形扩散用原生 View Transitions + Web Animations、震动用原生 Vibration API、涟漪与各 keyframes 均为自写；全部不分发素材、不联网、遵循系统减少动效与站内静音，不支持时降级。
`README.md` 的「即时反馈与动效」行同步更新并指向 `NOTICE.md`。

### 5. 验收

`scripts/_ui/fx-verify.mjs` 扩充：用 `Page.addScriptToEvaluateOnNewDocument` 挂钩原生 API 记录调用（桌面 headless 无震动硬件时补 stub）。
实测：预置 6 sparkle + 12 confetti；认领飘字 3 / 震动 1；点菜飘字 3、成功彩带 **12 个 `confetti-burst`**、累计震动 2；`ViewTransitions 可用 true`、主题扩散 `vt:1`（`cls:false` 证明走的是圆形扩散而非降级）、切回累计 2；保存弹跳 / 两处校验抖动 / 食材 ✓ / 图片淡入全 true；网格起点 14→14；**减少动效时浮层 display:none**；真实 JS 异常 0。
`go vet/test`、`api-test` 14/14、interaction/duty/round3/batch2/batch3(SSE)/pin/layout 全回归通过；双视口溢出 0、小控件 0、菜系切换 0 位移。

**未做**：菜谱卡滚动入场淡入（用户明确跳过；69 张卡逐帧入场易晃，与既有「切换 0 位移」约束冲突）。

## 十九、加大版效果图批量生成 20 张并统一（2026-11-13 17:25）

用户看样图后「满意，继续生成20张」。

### 1. 参数定稿

`publish_unified.py`：`ZOOM 0.69 → 0.60`、`OUT_SIZE (1600,1200) → (2400,1800)`，`SHIFT_Y=-0.027` 不变。盘子占画面约 66%→77%，四周留白约 17%→12%，盘沿仍完整。`RESUME_UNIFIED.md` 同步更新进度与验收尺寸。

### 2. 串行生成 20 张（idx 014–033）

严格按 `RESUME_UNIFIED.md` 的串行流程，**一次只发一个 ImageGen 调用**、全部输出到同一个 `unified_raw/`，每张确认 `status: completed` 与落盘；每 5 张查一次时间戳严格递增、无重名。20 张全部 2048×1536、约 3.6–3.9 MB。
`assign_raw.py --dry` 确认映射 014→033 后正式归位；拼 5×4 抽查图（`sample_bigger/contact_14-33.png`）确认 20 格菜名与画面对应、无串位、同一套桌面光线、器皿完整。

### 3. 发布与统一

- 变更前备份 `family-menu-default-20261113-1722.zip`（125.6 MB）。
- `publish_unified.py --dry --limit 20` → 正式发布 20 张，全部 OK。
- **额外统一**：此前已发布的 13 张仍是旧参数（小盘子）。为视觉一致，用 `--only 2..13` 把 12 张只重跑后处理/上传（**不重新生图、零额度**）；idx 001 保留用户已确认满意的那张样图。重处理前备份 `published_before_reprocess.json`、记录 12 个旧 URL。
- 清理本次产生的 13 张孤儿图（12 个重处理旧图 + 宫保鸡丁旧图），先确认无菜品引用再删；`uploads/` 剩 870 张。备份 zip 仍保留这些图，可恢复。

### 4. 线上核对

idx 001–033 共 33 张逐张下载校验，**全部 2400×1800、异常 0**；菜数 184、familyName 小小家、pantry 9、隐藏 6 全部不变；`/api/ping` 正常。成品拼图 `sample_bigger/final_14-33.png` 抽查：水印抹净、桌面光线一致、盘沿完整、无过曝。

### 5. 续跑状态

`published_unified.json` 已 33/105，**待生成 72 张（idx 034–105）**，下次从 34 开始；额度已恢复。断点续跑流程不变，参数已是加大版。

**版权**：全部为 AI 图生图 + 项目自有统一底图，无第三方素材或新增依赖，沿用 `NOTICE.md` 第 4 节声明。

## 二十、前端 UI 优化 + 成员密码权限 + 发布 16 张图（2026-11-13 21:42）

用户给三张截图要求优化，并限制密码权限；另把已生成未发布的 16 张图上架。

### 1. 发布已生成的 16 张（idx 034–049）
`assign_raw.py` 归位（映射 034→049 正确），拼抽查图核验 5 张异型器皿（034 汤盅 / 036 汤碗 / 038 砂锅 / 040 长盘 / 049 竹编小屉）均完整无切边、无串位、无糊图。`backup` 后 `publish_unified.py --limit 16` 全部 OK（备份 `family-menu-default-20261113-2134.zip`）。用户此轮明确「先不继续生成」，故未新增生图。

### 2. UI 修复（web/src）
- **图1「贴合」**：`TodayView.vue` 的 `.meal-switch` 原本「点的是」标签与早/午/晚胶囊挤在同一行 → 标签独立成行、胶囊包进新增 `.meal-chips`，`screens.css` 同步改 `flex-direction: column`。
- **图2「太长」**：`.pk-cta` 摇骰子横幅原本 `flex:1` 拉伸铺满整行（又长又空）→ 去掉 `flex:1`、加 `width:fit-content;max-width:100%`，变贴合内容的紧凑条。
- **图3「卡片大小统一」**：`FamilyView.vue` 成员卡 `.member-card` 改 `flex column`；「不吃 / 需密码」行包进新增 `.member-tags` 预留 `min-height`；`.member-actions` 加 `margin-top:auto` 贴底，同排卡片按钮行/进度条对齐。
- **密码权限**：成员卡「设密码 / 改密码」按钮加 `v-if="m.id === state.currentUserId"`，只给当前「我是」身份设密码（前端限制，沿用既有 `currentUserId` 信任模型；本地家庭 NAS 场景无独立后端鉴权）。

### 3. 构建与部署
本地 `node scripts/build.mjs`（Vite → webapp/dist + menu.exe）；交叉编译 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o menu-linux ./cmd/menu`。**运行实例是 LXC 100 内 Docker 容器 `family-menu`**（IMAGE `family-menu:local`，8090 由 docker-proxy 监听），并非直接跑在 LXC，也非 `/opt/family-menu` 目录（那只是源码镜像）。部署：`docker cp menu-linux family-menu:/usr/local/bin/menu` → `chmod 755` → `docker restart family-menu`。`docker restart` 保留 `/data`、`/config` 卷，安全。重启后 `Up healthy`，线上已服务新资源（`index-DCEljKcV.css` / `index-Cc7HRqud.js`）。

### 4. 验证
`/api/ping` 正常；新 CSS/JS 含全部新增规则（`.meal-chips`/`.member-tags`/`width:fit-content`/`margin-top:auto`）已核验；16 张新图 HTTP 200；菜数 184。浏览器硬刷新即可看到（资源带 hash 自动换新版）。

**版权/依赖**：仅 AI 图生图 + 项目自有统一底图，无第三方素材，沿用 NOTICE.md 第 4 节。

## 二十一、菜谱克数补全（2026-11-13 22:0x）

用户「把菜谱的菜g数补全」。范围：全部 109 道中缺精确克数的 86 道内置菜（其余 23 道已有 HowToCook 精确用量）。

### 1. 关键侦察
- `recipes.js` 里 `const HOWTO`（23 道）才是 `getRecipe` 真正消费源；`web/src/howto-amounts.json` 此前**未被任何前端代码 import**（仅是 `parse_howto_amounts.py` 产物），改它无效——这是本轮最易踩的坑。
- `data/dishes.json` 共 109 道；86 道内置菜只走技法模板，配料显示「盐 适量 / 生抽 1 勺」。

### 2. 数据生成
- `scripts/_cq/gen_howto_missing.py` 为 86 道各写一份「食材 克数」清单（如 `麻婆豆腐:["嫩豆腐 400g","牛肉末 100g",...]`），合并进 `howto-amounts.json`，保留原 23 道，合计 109、0 项缺数字/单位。克数为家常常识估算，非 HowToCook 权威值。

### 3. 接线（让数据真正生效）
`recipes.js` 改为 `import HOWTO from './howto-amounts.json'`，删除内联 23 道常量，使 json 成为唯一来源（`scripts/_cq/wire_howto_json.py`）。`getRecipe` 仍用 `HOWTO[dish.name]` 整表替换模板配料，菜谱页与购物清单即显示具体克数。

### 4. 部署与排障（重要经验）
- 构建：`node scripts/build.mjs`（Vite→webapp/dist + menu.exe）；交叉编译 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o menu-linux ./cmd/menu`。新前端包 hash `index-CzyMltb7.js`。
- **首部署翻车**：用 `pct exec 100 -- tee` 经 stdin 传二进制，结果文件被损坏（本地 md5 `ca64ecb1` ≠ 容器内 `47cf79af`），容器启动 exit 139（SIGSEGV）crash loop。已用部署前备份 `/usr/local/bin/menu.bak` 即时 `docker cp` 回滚，服务秒级恢复。
- **正确通道**：SFTP 到 PVE 主机 `/tmp` → `pct push 100 /tmp/menu-linux /tmp/menu-linux` → `docker cp` 进 `family-menu:/usr/local/bin/menu` → `chmod 755` → `docker restart`。每步校验 md5 一致后再重启。最终 `Up healthy`、/api/ping 正常、`/assets/index-CzyMltb7.js` 含全部新增克数条目。

### 5. 提交与边界
- 提交 `73efaa0` 推送 origin/main（recipes.js / howto-amounts.json / 两个生成脚本 / .gitignore）。含 PVE 密码的部署脚本按仓库约定(.gitignore 第45-46行)不入库；menu-linux 亦忽略。
- 遗留：线上 API 此前报菜数 184，本地 data/dishes.json 仅 109——约 75 道菜名本环境取不到（SSH 密码非交互受限），未覆盖；如需补全须提供菜名或开放 SSH。
- 注意：勿直接用 `parse_howto_amounts.py` 覆盖 howto-amounts.json（会清空本轮人工补全的 86 道）。
