# 家里吃点啥 · 家庭点菜系统

> 仓库：https://github.com/1316195118/Home-Feast

一家人自己用的点菜系统：网页访问、内置常见菜系、菜品效果图由 AI 绘制、每个人都有身份、每天定谁掌勺。
**数据只存在自己的 NAS / 本机**，纯 JSON 文件，无数据库、无外部服务、无需注册。

单容器 / 单文件即可运行：Go 内嵌 Vue 3 前端，Docker 多阶段构建后运行镜像里只有一个二进制。

---

## 一、现在能做什么

| 功能 | 说明 |
| --- | --- |
| 内置菜谱 | 100+ 道常见菜，覆盖川、粤、鲁、苏、浙、闽、湘、徽八大菜系，以及家常菜、汤羹、主食、凉菜、早餐、甜品饮品、西餐烘焙 |
| AI 效果图 | 10 张统一水彩手绘风的菜系插画，压缩后随前端嵌入；也可为任意一道菜上传自己的照片 |
| 家庭成员 | 每人一个昵称 + emoji 头像 + 颜色，切换身份即可点菜；不用注册、不填手机号 |
| 今天谁做 | 每天指定掌勺人、写一句备注，点菜墙按人展示，可标记「已上桌」或撤掉 |
| 选择困难 | 从菜谱随机捞三道，点一下直接加进今日菜单 |
| 自己加菜 | 增删改菜品、选菜系、写描述标签、定辣度与耗时、收藏 |
| 记录与统计 | 按天回看历史菜单、掌勺次数、本月点菜数；删菜后历史仍保留菜名快照 |
| 备份 | 一键导出 zip（4 份 JSON + 上传的照片），可从 zip 恢复；直接打包数据目录也行 |
| 可选口令 | 可在网页「设置」里直接改，也可用 `MENU_ACCESS_CODE` 环境变量锁定，防止局域网内外人改菜单 |
| 重庆菜效果图 | 52 道重庆菜逐道 AI 生成写实图，统一浅木纹桌面与不同餐具，4:3 裁切适配卡片；菜品详情里整张图完整可见 |
| 骰子 PK 定分工 | 每人摇一次，点数落库（换人、刷新都不丢）；都摇完自动定掌勺与洗碗，平局自动加摇；结果动画按人只播一次；音效由 Web Audio 实时合成，**不含任何外部音频文件** |
| 自定义用餐时段 | 每餐的名字、图标（预设面板点选，也可自定义）、收起时间都能改，过饭点自动从今日菜单收起 |
| 头像与动效 | 56 个分组头像（人物 / 动物 / 食物 / 兴趣）；统一的动效令牌与即时反馈，并支持系统「减少动效」偏好 |

界面为响应式：电脑上是顶栏 + 多列，手机上自动变成底部导航 + 双列菜谱。

---

## 界面预览

**今天** — 定谁掌勺、点菜墙、随手推荐三道

![今天页](docs/images/today.png)

**菜谱** — 15 个分类、109 道内置菜，插画由 AI 绘制

![菜谱页](docs/images/menu.png)

**家人与设置**

![家人页](docs/images/family.png)

![设置页](docs/images/settings.png)

---

## 二、Docker 部署到 NAS（推荐）

### 最快：clone 下来一条命令起

`docker-compose.yml` 已经放在**仓库根目录**，克隆完直接起，不用装 Go、不用装 Node：

```bash
git clone https://github.com/1316195118/Home-Feast.git
cd Home-Feast
docker compose up -d --build
```

浏览器打开 `http://<这台机器的 IP>:8090`。数据全部落在项目下的 `./data`（已加入 `.gitignore`，不会进仓库）。
以后升级：`git pull && docker compose up -d --build`，`data/` 不动，菜单、成员、历史都在。

### 拷到 NAS 上跑

把**整个项目目录**拷到 NAS，例如 `/volume1/docker/family-menu/`，然后：

```bash
cd /volume1/docker/family-menu
docker compose up -d --build
```

浏览器打开 `http://<NAS 的 IP>:8090`。

`docker-compose.yml` 里按你的 NAS 调整两处：

```yaml
    volumes:
      - /volume1/docker/family-menu/data:/data   # 群晖 DSM 示例
    # user: "1000:10"                            # 绿联 UGOS Pro 首用户；群晖可省略
```

| NAS / 系统 | volumes 左侧写法 | 备注 |
| --- | --- | --- |
| 群晖 DSM | `/volume1/docker/family-menu/data:/data` | 一般不用设 `user` |
| 绿联 UGOS Pro | `/volume1/docker/family-menu/data:/data` | 建议 `user: "1000:10"` |
| 极空间 / 铁威马 / 普通 Linux | `./data:/data` | 相对路径最简单 |
| Windows + Docker Desktop | `./data:/data` | 也可直接跑 `menu.exe` |

升级不会丢数据：容器内 `/data` 是唯一持久化目录。

端口冲突就改 `ports` 左边，例如 `"8091:8090"`。想加访问口令，取消 `MENU_ACCESS_CODE` 注释并改成自己的值，再 `docker compose up -d --build`。

也可以手动构建镜像：

```bash
docker build -t family-menu:local .
docker run -d --name family-menu -p 8090:8090 \
  -v /volume1/docker/family-menu/data:/data \
  -e TZ=Asia/Shanghai --restart unless-stopped family-menu:local
```

### 已部署实例：PVE 里的 Docker LXC（2026-11-06）

| 项 | 值 |
| --- | --- |
| 访问地址 | **http://10.10.10.251:8090** |
| PVE 宿主机 | `10.10.10.252`，PVE 9.2.24 |
| 容器 | CT 100 `docker-LXC`，Debian 13，Docker 29.9.2 / Compose v5.7.0 |
| 项目目录 | `/opt/family-menu/` |
| 数据目录 | `/opt/family-menu/data` → 容器内 `/data` |
| 源码备份包 | `/opt/family-menu-src.tgz` |
| 镜像 / 容器名 | `family-menu:local` / `family-menu`，端口 `8090:8090` |
| 访问口令 | 未启用（家庭局域网） |

该 LXC 已配置 Docker Hub 镜像加速；`Dockerfile` 里固定用 `registry.npmmirror.com` 装前端依赖，规避国内网络抖动。首次 `up -d --build` 约 2 分钟。

常用运维（在 CT 100 内执行，或由 PVE 宿主机 `pct exec 100 -- <命令>`）：

```bash
cd /opt/family-menu
docker compose ps                     # 状态与健康检查
docker compose logs -f family-menu    # 看日志
docker compose restart               # 重启
docker compose up -d --build          # 更新源码后重建
docker compose down                   # 回滚/停机，data 目录保留
cp -a /opt/family-menu/data /opt/family-menu-data-backup-$(date +%F)   # 备份数据
```

回滚：先备份 `data/`，`docker compose down` 后如需恢复源码可重新解开 `/opt/family-menu-src.tgz`。部署过程未改动 PVE 宿主机配置、CT 配置或同机其它容器。

---

## 三、不用 Docker，直接跑单文件

```bash
# Windows
build.bat                 # 生成 menu.exe
menu.exe -port 8090 -data .\data

# Linux / macOS / WSL
./build.sh                # 生成 ./menu
./menu -port 8090 -data ./data
```

构建需要 Node 20+ 与 Go 1.24+；**运行只需要那一个可执行文件**。打开 `http://127.0.0.1:8090`。

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-addr` | `0.0.0.0` | 监听地址，只给本机用可设 `127.0.0.1` |
| `-port` | `8090` | 监听端口 |
| `-data` | 程序旁 `data/` | 数据目录，NAS 上指向挂载卷 |
| `-version` | — | 打印版本号 |

等价环境变量：`MENU_ADDR`、`MENU_PORT`、`MENU_DATA_DIR`、`MENU_ACCESS_CODE`。命令行显式传入的参数优先级更高。

---

## 四、目录结构

与 [gift-book](https://github.com/1316195118/gift-book) 保持同类的组织方式（**仅结构参考，代码完全自写，详见 [NOTICE.md](NOTICE.md)**）：

```
cmd/menu/            Go 入口：参数、嵌入资源、优雅退出
internal/api/        HTTP 接口、静态资源、上传、备份
internal/store/      数据模型 + JSON 文件存储（原子写、并发锁）
internal/seed/       首次启动的内置菜品与示例成员
web/                 Vue 3 + Vite 前端源码
web/src/styles/      四层 CSS：设计令牌 / 基础 / 组件 / 版面
web/public/dishes/   AI 绘制的菜系插画（压缩后的 JPEG）
webapp/dist/         前端构建产物，Go embed 目录（构建时生成）
docs/artwork/        AI 插画原始 PNG（含生成平台标识，留档）
scripts/             构建、接口测试、图片转码
data/                运行期数据（已 gitignore）
```

---

## 五、数据长什么样

```
data/
├── dishes.json      菜品库（内置 + 自定义）
├── members.json     家庭成员
├── plans.json       每日安排：掌勺人、备注、点菜记录
├── settings.json    家庭名等设置
└── uploads/         自己上传的菜品照片
```

- 每次写入走「写临时文件 → fsync → 原子改名」，断电最多回到上一份完整文件。
- 点菜记录里冗余保存了菜名与点菜人昵称，删掉菜品或成员后历史依然读得懂。
- 备份：页面「设置 → 导出备份」，或直接 `tar czf menu-backup-$(date +%F).tgz data/`。

---

## 六、开发与验证

```bash
cd web && npm install && npm run dev     # 前端热更新，代理到本机 8090
node scripts/build.mjs                  # 前端 + Go 一步构建
BASE=http://127.0.0.1:8090 node scripts/api-test.mjs   # 接口冒烟测试
go vet ./... && go test ./...           # 后端静态检查
go run scripts/optimize-images.go       # 把 docs/artwork 的 PNG 重新转成 JPEG
```

`api-test.mjs` 会临时建一道测试菜、点一次菜再清理，并还原当天的掌勺人与备注。
开启了访问口令时用 `CODE=你的口令 node scripts/api-test.mjs`。

---

## 七、版权与素材

用户特别关心这一点，这里说清楚：

- **全部源码、CSS、文案独立编写**，MIT 许可。只参考了 gift-book 的「目录怎么分、构建怎么串」，**没有复制它的任何代码、样式、文案、图标或图片**。
- 运行时零第三方 Go 依赖；构建期依赖 Vue 3 / Vite / @vitejs/plugin-vue（均 MIT），Go 标准库（BSD-3-Clause）。未引入 GPL/AGPL 代码、商业模板、UI 组件库。
- 菜名与菜系属于通用饮食常识；描述、标签、耗时为自行撰写，**未摘录任何菜谱网站或图书**。
- 菜品插画由 AI 生成，属本项目自生成素材；原始图留档在 `docs/artwork/`，未使用任何图库照片。图标为 emoji 与自写 SVG/CSS，字体走系统字体栈，不分发第三方字体/图标文件。

完整清单见 [NOTICE.md](NOTICE.md)，许可证见 [LICENSE](LICENSE)。

---

## 八、后续可以继续优化的方向

第一版刻意保持轻量，下面是自然延伸点：接入菜品做法/配料与购物清单、每周掌勺排班与轮换提醒、家庭口味避雷标签、多人实时同步（SSE）、PWA 离线可用、按季节/冰箱食材智能推荐、Excel 导出、深色主题、多家庭/多账簿隔离、自动化定时备份到 NAS 另一目录。

---

MIT © 2026 Family Menu contributors
