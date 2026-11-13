# 统一底图菜品图 · 续跑说明（更新 2026-11-13）

## 当前进度

- 目标：105 道菜各一张专属写实图，全部共用**同一套浅木纹桌面 + 光位 + 45° 俯拍 + 居中构图 + 四周等距留白**，器皿按菜性变化。
- **已完成并上线 33 / 105**：idx `001–033`。其中 014–033 于 2026-11-13 串行生成 20 张；001–013 也已统一按加大参数重处理（001 用用户确认的样图，002–013 只重跑后处理、未重新生成）。
- **待生成 72 张**：idx `034–105`，下次从 **34** 开始。
- 额度已恢复（2026-11-13 生成 21 张均成功）。
- **参数已按用户「效果图再大一点」改为加大版**：`ZOOM=0.60` / `OUT_SIZE=2400x1800`，见下表；已发布的 33 张全部是该规格，线上逐张核对无异常。

## ⚠️ 最重要的一条：严禁并行调用图片生成

2026-11-10 03:20 曾派 3 个子代理并行生成，结果 `ImageGen` 的 **`output_dir` 在并发下会串位**：
返回的 `localPath` 落在别的编号目录，`unified/002/` 里装的是「菌菇鸡汤」而不是「水煮鱼」。
当时 83 张图散落在 72 个目录、33 个目录缺图、文件名不含菜名，**无法归位，整批作废**。
残图已备份到 `C:/Users/Administrator/AppData/Local/Temp/fm-unified-broken/`，**不要使用**。

结论：**全局同时只能有一个生成调用在飞**。不要派多个子代理，也不要在同一条消息里放多个 `DeferExecuteTool/ImageGen`。

## 已验证可靠的串行流程

原理：所有调用统一输出到**同一个目录** `scripts/_cq/unified_raw/`，串行生成时文件名自带时间戳且严格递增；
按文件名排序 == 按 idx 顺序，用 `assign_raw.py` 归位即可，不依赖 `output_dir` 是否正确。

1. **生成清单**（已有，无需重做）：
   ```
   python scripts/_cq/build_unified_manifest.py
   ```
   产物 `scripts/_cq/unified_manifest.json`，105 项各含 `idx / dishId / name / vessel / prompt / base / outdir`。
   `prompt` 必须**原文使用**，里面带线上菜品的 `desc`（105/105 命中），能避免模型生成错菜。
   清单依赖 `scripts/_cq/unified_dishes.json`（线上 `/api/dishes` 的缓存）；若线上菜品变动，重新拉一次即可。

2. **逐张串行生成**（从 idx **34** 开始，严格按 idx 从小到大，一次只发一个调用）：
   先 `ToolSearch({tool_names:["ImageGen"]})` 加载 schema，然后
   ```
   DeferExecuteTool({toolName:"ImageGen", params:{
     prompt: <该项 prompt 原文>,
     size: "2048x1536",
     quality: "high",
     image1: "C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_cq/base_unified/plate.png",
     input_fidelity: "high",
     output_dir: "C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_cq/unified_raw/"
   }})
   ```
   - 不要传 `style` / `background` / `footnote`。
   - 每次确认返回 `status: completed` 且 `localPath` 文件存在；失败只重试当前这一张，仍串行。
   - 建议每 20–25 张停一次，跑一次归位 + 发布，避免中途再次 429 丢进度。
   - 底图 `base_unified/plate.png` 已入库（约 2MB），clone 后即可复现；**不要重新生成底图**，否则构图会变。

3. **归位**（把 `unified_raw/` 里的图按时间戳顺序放进 `unified/NNN/`）：
   ```
   python scripts/_cq/assign_raw.py          # 先看 --dry
   ```
   已归位的编号会自动跳过，支持多次运行、断点续跑。归位后务必抽查 1–2 张内容与菜名是否对应（用 `ViewImage` 拼图看）。

4. **发布到 PVE**（`10.10.10.251:8090`，走应用自己的 `/api/upload` + `PUT /api/dishes/{id}`）：
   ```
   python scripts/_cq/publish_unified.py --dry     # 预览
   python scripts/_cq/publish_unified.py          # 正式，自动跳过已发布/无图的编号
   ```
   `published_unified.json` 记录已发布项；`rollback_unified.json` 记录最初旧图，可随时回滚。

## 发布脚本的固定参数（改动前先备份，改动会影响后续所有图）

| 参数 | 值 | 说明 |
| --- | --- | --- |
| `OUT_SIZE` | `2400x1800` | 加大版定稿，比上一版 1600×1200 大 50% |
| `ZOOM` | `0.60` | 中心保留 60%，盘子占画面宽约 77%、四周留白约 12%、盘沿仍完整（用户 2026-11-13 看过样图后确认） |
| `SHIFT_Y` | `-0.027` | 修正底图自带的垂直偏移，使上下留白等宽；与 ZOOM 无关，勿随缩放调整 |
| 输出 | WebP q88 | 抹右下角水印（四边 12px 羽化）→ 居中裁 4:3 → 等比缩放；**不调亮度、不填色、不合成** |

## 验收标准（发布后逐项核对）

- 尺寸 `2400x1800`，平均亮度 120–180，**高光像素占比 < 1%**（历史上整图色调平移曾把高光推到 30–58%，菜品发白，已弃用）。
- 盘子/器皿完整可见、居中、四周留白等宽且不贴边；菜品完全在器皿内，不散到桌面。
- 抽查图内容与该编号菜名一致（曾因并发串位出现张冠李戴，必须抽查）。
- `/api/ping` 正常；`/api/dishes` 中该菜 `image` 等于 `published_unified.json` 里的 `new`。

## 文件说明

| 文件 | 用途 |
| --- | --- |
| `base_unified/plate.png` | **统一底图**（入库，勿重新生成） |
| `unified_dishes.json` | 线上菜品缓存，供清单取 `desc` |
| `unified_manifest.json` | 105 项生成参数（含完整 prompt） |
| `build_unified_manifest.py` | 重建清单 |
| `unified/NNN/` | 归位后的生成原图（被 `.gitignore` 忽略） |
| `unified_raw/` | 串行生成统一落盘目录（忽略） |
| `assign_raw.py` | 按时间戳归位 |
| `publish_unified.py` | 处理 + 上传 + 更新菜品，断点续跑 |
| `published_unified.json` / `rollback_unified.json` | 发布记录 / 回滚记录（入库） |
| `out_unified/NNN.webp` | 成品（忽略） |
| `sample_bigger/` | 加大版样图、参数对比、线上截图、回滚记录（忽略，不入库） |
| `make_bigger_sample.py` | 用同一原图出「现行/加大」两版并拼对比图 |

## 背景知识

- 后处理历史事故：`normalize.py` 曾用「整图通道加法统一桌面色」造成过曝（+42~88 灰阶），椭圆羽化留下圆形光晕，纯色画布填充又拉高亮度，全部弃用。现在只做几何处理。
- 用户明确要求：**统一的是桌面背景 / 光位 / 角度 / 居中构图 / 留白比例，器皿不必一样**。
- 菜品详情页与卡片均为 4:3 + `object-fit: contain`，成品必须严格 4:3。
