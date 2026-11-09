# 重庆菜配图统一 · 续跑说明

## 进度

- 目标：52 道「重庆菜」分类菜品，统一浅木纹背景 + 写实摄影风格 + 每道菜换有辨识度的器皿。
- **已完成并上线 26 / 52**（序号见 `published.json`）。
- **待生成 26 张**：`25 26 27 28 29 30 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 48 49 50 51 52`
- 暂停原因：图片生成接口触发 429 累计配额限制（2026-11-08 21:20 起），用户要求「剩下的后面再生成」。

## 为什么这样改

线上原本 52 道重庆菜已有写实 AI 图（浅木纹桌面），但**几乎全是同一个白圆盘**，右下角还有平台水印。
所以做法是「以图生图」：保留菜品形态与统一木纹背景，只替换器皿，并裁掉底部水印。

器皿按菜性成体系分配（同一系列内换釉色/纹样/器型，既成套又有变化），完整分配表见 `manifest.json`：
砂锅陶煲（炖/汤/大盆鱼）、青花青瓷（凉菜）、黑釉粗陶（红油重口）、铸铁铁板（干煸爆炒）、竹编（炸物小吃）、米黄釉陶/白瓷（清炒素菜）。

## 续跑步骤（下次直接照做）

1. 读 `remaining.json`：每项含 `idx` / `name` / `dishId` / `prompt` / `base`（底图绝对路径）/ `outdir`。
2. 逐项调用图片生成（图生图）：`image1 = base`、`prompt`、`input_fidelity: high`、`size: 1536x1024`、`quality: high`、`output_dir = outdir`。
   - 建议分批，避免再次触发 429；失败就停，已成功的会留在各自 `gen/NN/` 目录。
3. 全部生成完后执行 `python scripts/_cq/publish.py`：
   - 自动「裁掉底部 9% 去水印 → 居中裁 4:3 → 转 WebP(86) → POST /api/upload → PUT /api/dishes/{id} 更新 image」。
   - **支持断点续跑**：`published.json` 记录已成功项，重复运行只处理缺的；中途失败可直接重跑。
   - 先 `python scripts/_cq/publish.py --dry --limit 3` 可空跑预览。
4. 验收：`node scripts/_cq/verify.mjs`（检查卡片图片加载、无色调扰动、比例 4:3，并截图 `menu_shot.png`）。

## 文件说明

| 文件 | 用途 |
| --- | --- |
| `manifest.json` | 52 道菜完整清单：id / 菜名 / 原图 / 器皿描述 / 序号 |
| `remaining.json` | 待生成 26 张的全部参数（含现成 prompt） |
| `base/` | 52 张线上原图（图生图底图），已转 PNG |
| `gen/NN/` | 各序号生成图（原始 1536×1024，含水印） |
| `out/NN.webp` | 处理完成品（1241×931，4:3，无水印） |
| `publish.py` | 裁切 + 转码 + 上传 + 更新菜品，断点续跑 |
| `published.json` | 已上线记录（新图 URL），`publish.py` 据此跳过 |
| `rollback.json` | 回滚清单：需恢复时把 `restore` 写回对应菜品 `image` |
| `verify.mjs` | 线上验收脚本 |

## 相关代码改动（已上线）

- `web/src/utils.js` `artStyle(item, dishes)`：菜品**有自己的配图**时不再施加 `hue-rotate` / `saturate` / 随机取景，
  否则 52 张木纹背景会被染成不同色调、破坏「背景统一」；没有专属图、退回共用菜系插画的菜仍沿用原打散逻辑。
- 4 个调用点（`DishCard.vue` / `DishDetail.vue` / `TodayView.vue` ×2）同步传入 `state.dishes`。
- 前端已重新构建并部署（`index-DpZJUfcL.js`）。

## 注意事项

- 图片通过应用自身 API 写入线上 `data/uploads/`，**新文件名**绕开 uploads 的 1 天缓存；不改动 Go 代码、无需重建容器。
- 生成图右下角固定带 "AI生成 / WORKBUDDY" 水印，靠裁掉底部 9% 去除；补丁法会留色块，别用。
- 所有卡片是 `aspect-ratio:4/3 + object-fit:cover`，成品必须裁成 4:3，否则会被二次裁切跑偏。
