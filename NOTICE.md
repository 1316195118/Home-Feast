# 版权与素材说明

本项目重视版权与授权清晰，以下内容对使用者与二次开发者完全公开。

## 1. 源码

- `cmd/`、`internal/`、`web/src/`、`scripts/` 下的全部 Go / JavaScript / Vue / CSS 代码与文案均为本项目**独立编写**，采用 [MIT](LICENSE) 许可。
- 目录组织与「单二进制内嵌前端 + 纯 JSON 文件存储 + 多阶段 Docker 构建」这一**工程思路**参考了开源项目 [1316195118/gift-book](https://github.com/1316195118/gift-book)（MIT）。
  **本项目未复制、未改写、未翻译该项目的任何源代码、样式、文案、图标或图片素材**；界面设计、数据模型、接口设计、代码风格均独立实现。参考仅停留在「目录怎么分、构建怎么串」的层面。
- 未使用任何来自 GPL / AGPL / SSPL 等 reciprocal 许可项目的代码；未引入任何商业模板、后台框架或 UI 组件库。

## 2. 第三方依赖

| 组件 | 用途 | 许可证 |
| --- | --- | --- |
| Go 标准库（含 `embed`、`archive/zip`、`image/jpeg`） | 后端、静态资源嵌入、备份、图片转码 | BSD-3-Clause |
| Vue 3 | 前端响应式框架 | MIT |
| Vite、`@vitejs/plugin-vue` | 前端构建工具 | MIT |
| `node:*-alpine`、`golang:*-alpine`、`alpine` 基础镜像及 `tzdata`、`ca-certificates` | 构建与运行容器 | 各自上游许可证 |

运行时依赖只有 Go 标准库；`npm` 依赖仅用于构建前端，产物为静态文件并嵌入二进制，运行镜像中不含 Node.js。

除上表外**没有引入任何其它库**。深浅主题的圆形扩散、手机震动、点击涟漪、飘字与彩带分别使用浏览器自带的 View Transitions / Web Animations / Vibration API 与本项目自写的 CSS 动画，均为浏览器原生能力或本项目自写代码，未复制任何第三方动画库、动效模板或代码片段。

## 3. 菜品数据

- 菜名、菜系归属属于通用事实信息与公共领域饮食文化常识，不受著作权保护。
- 每道菜的口味标签、一句话描述、建议耗时、辣度均为本项目自行撰写，**未摘录任何菜谱网站、图书或 App 的文字**。
- 内置菜品仅为点菜清单，做法步骤与基础配料表均为本项目自写模板，不含任何第三方菜谱的步骤。
- 部分同名菜（23 道）的**精确用量数值**（克 / 毫升，如「盐 2 g」「生抽 15ml」）提取自开源项目 [Anduin2017/HowToCook](https://github.com/Anduin2017/HowToCook)，该项目采用 **Unlicense（公共领域）** 授权。提取逻辑见 `scripts/_cq/parse_howto_amounts.py`，结果以字符串表内嵌于 `web/src/recipes.js` 的 `HOWTO` 常量，仅用于把模板里的「少许 / 适量」替换为可称量的具体数值；做法步骤、口味标签、描述、菜系与图片仍全部为本项目自有内容。HowToCook 未参与本项目的界面、数据模型或图片创作。

## 4. 图片与视觉素材

- `web/public/dishes/*.jpg` 为本项目通过 AI 绘图生成的**写实菜系封面**：16 个菜系各一张浅木纹桌面、45° 俯拍的真实食物摄影风图片（4:3 JPG），用于菜品卡片与分类展示；原始生成图留档为 `docs/artwork/real-*.png`（含生成平台的标识，备查）。
- `web/public/icon-192.png` / `icon-512.png` 是用 Pillow 自画的碗形 PWA 图标，与 `web/index.html` 里内联的 favicon SVG 同一图形，不含任何第三方图标素材。
- 52 道重庆菜的写实效果图同样为 AI 图生图生成，经裁切与 WebP 转码后由使用者上传至自有实例的 `/data/uploads`，不随源码分发。
  图片不是任何图库、菜谱网站或摄影作品的复制或再加工。前端卡片通过构图裁切显示画面主体。
- 每道自定义菜品可上传自己的照片，上传内容由使用者自行负责权利。
- `docs/images/*.png` 是本项目界面在自带无头浏览器中的截图，用于 README 展示，非第三方素材。
- 图标使用 Unicode emoji 与项目自写的 SVG / CSS 图形；字体使用操作系统自带字体栈，**不打包、不分发任何第三方字体或图标库文件**。
- `web/index.html` 中的 favicon 为本项目自写的内联 SVG。
- 骰子 PK 的音效由 `web/src/audio.js` 用浏览器原生 **Web Audio API 实时合成**（振荡器 + 带通白噪声 + 增益包络），**不含任何第三方音频文件、采样、音效库或在线音源**，也未引入任何新的第三方依赖；声音只在用户交互时于本机生成，不随项目分发。
- 点击飘字与成功彩带（`web/index.html` 的 `#fx-layer` + `web/src/feedback.js` + `base.css`）：飘字仅用 **Unicode emoji**，彩带是**纯 CSS 绘制的纯色小矩形**配合 CSS keyframes 抛洒旋转，颜色取自项目自有配色。不含任何图片、贴纸、GIF、Lottie 文件、图标包或动效库；特效节点在页面加载时预置，运行时只改 CSS 变量与 class，不增删 DOM。
- 深浅主题圆形扩散（`web/src/theme.js`）使用浏览器原生 **View Transitions API + Web Animations API** 的 `clip-path` 动画；手机轻震（`web/src/feedback.js`）使用浏览器原生 **Vibration API**。二者都不分发任何素材、不需要联网、不引入依赖，并在系统「减少动效」偏好或站内音效静音时自动关闭；不支持这些 API 的浏览器会降级为普通配色过渡 / 无震动。
- 点按涟漪 `web/src/ripple.js`、校验抖动 / 成功弹跳 / 数字弹跳 / 骰子对撞等均为本项目自写的 CSS keyframes（`web/src/styles/base.css`），无第三方动效代码。

## 5. 商标

本项目不含任何第三方商标、品牌标识或商业名称。若二次开发时加入自有名称或素材，请同步更新本文件。
