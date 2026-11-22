#!/usr/bin/env python3
"""Generate the bundled realistic pantry ingredient photos with Ark Seedream."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ARKCLI_KEY = Path(r"C:\Users\Administrator\.arkcli\identities\volc-2134677250\apikey.json")
NODE = Path(
    r"C:\Program Files\WindowsApps\OpenAI.CodexPrimaryRuntime.v26-1119-907-0_26.1119.907.0_x64__3k8sg7r9htsxt\dependencies\node\bin\node.exe"
)
GENERATE = Path(r"C:\Users\Administrator\.codex\skills\byted-ark-seedream-skill\scripts\generate.js")
OUT = ROOT / "scripts" / "_cq" / "_gen_pantry55"

# Keep this list aligned with web/src/utils.js pantryGroups. Generated image files use
# group-number names so changing a Chinese label never leaves broken URL-encoded paths.
GROUPS: dict[str, list[str]] = {
    "meat": [
        "猪肉", "五花肉", "排骨", "牛肉", "牛腩", "鸡胸肉", "鸡腿肉", "鸡腿", "鸡翅",
        "鸡蛋", "培根", "香肠", "午餐肉", "火腿", "羊肉", "鸭肉",
    ],
    "seafood": [
        "虾", "虾仁", "鲈鱼", "鲤鱼", "鲫鱼", "草鱼", "鱼头", "带鱼", "三文鱼",
        "鳕鱼", "黄鱼", "鱿鱼", "花甲", "扇贝", "生蚝", "海带",
    ],
    "veg": [
        "番茄", "土豆", "青椒", "黄瓜", "白菜", "菠菜", "茄子", "西兰花", "胡萝卜",
        "洋葱", "芹菜", "生菜", "上海青", "豆角", "四季豆", "莴笋", "冬瓜", "南瓜",
        "玉米", "西葫芦", "苦瓜", "山药", "莲藕", "韭菜", "豆芽", "娃娃菜", "青豆",
        "芦笋", "香菜", "青蒜", "葱", "蒜", "姜",
    ],
    "mushroom": [
        "香菇", "金针菇", "平菇", "杏鲍菇", "茶树菇", "木耳", "豆腐", "豆干",
        "腐竹", "百叶", "豆皮", "黄豆", "毛豆",
    ],
    "staple": [
        "米饭", "面条", "挂面", "馒头", "饺子皮", "面粉", "糯米", "年糕", "粉丝",
        "面包", "油面筋", "春卷皮",
    ],
    "fruit": [
        "苹果", "香蕉", "柠檬", "菠萝", "梨", "橙子", "桃子", "葡萄", "草莓", "西瓜",
    ],
    "dairy": ["牛奶", "酸奶", "黄油", "奶酪", "咸鸭蛋", "豆浆"],
}

GROUP_DIRECTION = {
    "meat": (
        "冷藏生鲜食材摄影，按家庭烹饪前状态自然分切、修整干净，能看清真实肌肉纤维、"
        "脂肪纹理与湿润表面；这是生食材，绝不是做好的菜，不要酱汁、配菜、熟肉或餐盘。"
    ),
    "seafood": (
        "冷藏水产食材摄影，按买回家清洗后的真实形态展示，鳞片、鳃、贝壳纹理、鱼肉切面"
        "或虾壳质感必须可信；这是生鲜食材，不是烹饪完成的菜，不要葱姜、酱汁或餐盘。"
    ),
    "veg": (
        "新鲜蔬菜食材摄影，保留自然表皮、叶脉、根部、须根和水汽，不带泥土或腐烂斑点，"
        "像刚从菜市场买回并简单整理过；这是生鲜蔬菜，不是切好装盘的菜。"
    ),
    "mushroom": (
        "新鲜菌菇或豆制品食材摄影，呈现购买后可以下锅前的真实形态，材质、孔隙、表面水分"
        "与自然颜色准确；不是已经炒熟或装盘的成品菜。"
    ),
    "staple": (
        "家庭冰箱里常见的主食储存形态，采用简洁食品袋、纸包、捆扎或保鲜盒等真实包装方式，"
        "包装素净、没有品牌和文字，食物本身清楚可见；不要变成一碗做好的成品饭面。"
    ),
    "fruit": (
        "新鲜水果食材摄影，保留果皮纹理、果梗、自然光泽和成熟度，像家里冰箱或果篮里的真实水果；"
        "可以是一枚或少量自然聚在一起，但不要切开摆盘，不要成果盘或甜品。"
    ),
    "dairy": (
        "家庭冷藏乳品或蛋类摄影，使用素净、无品牌的真实包装或完整蛋托，包装上绝对不能有文字、"
        "商标、营养标签和二维码；食材本身清楚可见，不是饮品广告里的一杯成品。"
    ),
}

ITEM_DIRECTION = {
    "猪肉": "一块新鲜猪里脊，淡粉色肌肉和白色脂肪边清楚；不要肉馅。",
    "五花肉": "一块带清晰皮、肥、瘦分层的生五花肉，切面方正。",
    "排骨": "新鲜猪肋排斩成家庭常用小块，骨头和肉比例真实。",
    "牛肉": "一块新鲜牛里脊或牛肩肉，暗红肉色、正常脂肪纹理。",
    "牛腩": "数块切好的生牛腩，带筋膜和脂肪，表面湿润不发黑。",
    "鸡胸肉": "两片去骨生鸡胸肉，淡粉色、自然纹理，边缘修整干净。",
    "鸡腿肉": "去骨生鸡腿肉数块，带鸡皮和自然脂肪。",
    "鸡腿": "两只完整生鸡腿，皮和骨关节完整，不是卤鸡腿。",
    "鸡翅": "数只完整生鸡中翅，皮色自然，排列松散。",
    "鸡蛋": "六枚新鲜褐壳和白壳鸡蛋自然放在素色纸蛋托里。",
    "培根": "数片冷藏生培根叠放，红白纹理清楚，不煎熟。",
    "香肠": "数根中式生鲜香肠，肠衣自然，不用热狗肠。",
    "午餐肉": "一罐打开后的午餐肉完整长方体，切面可见细密肉纹，无品牌标签。",
    "火腿": "一块完整金华火腿切片或火腿块，真实干香质感，不出现熟食拼盘。",
    "羊肉": "数块新鲜生羊肉，颜色深红，脂肪洁白。",
    "鸭肉": "一块新鲜生鸭胸或斩块鸭肉，带皮，不是烤鸭。",
    "虾": "数只带壳鲜虾自然聚在一起，半透明虾壳、虾须完整。",
    "虾仁": "一小堆已去壳去虾线的生虾仁，灰白透亮。",
    "鲈鱼": "一条完整新鲜鲈鱼，鳞片和鱼眼清晰，侧面自然摆放。",
    "鲤鱼": "一条完整新鲜鲤鱼，鳞片金黄，鱼身自然弯曲。",
    "鲫鱼": "两条完整新鲜小鲫鱼，银灰鳞片，形态不同。",
    "草鱼": "一段新鲜草鱼净鱼身或完整中小草鱼，鱼皮纹理清楚。",
    "鱼头": "一个新鲜大鱼头，鳃部和鱼皮完整，不出现鱼身。",
    "带鱼": "数段银亮的新鲜带鱼段，银膜自然完整。",
    "三文鱼": "一块新鲜三文鱼柳，橙粉色鱼肉和白色脂肪线清楚。",
    "鳕鱼": "一块冷冻解冻后的雪白鳕鱼柳，肉质纹理清楚。",
    "黄鱼": "一条完整新鲜大黄鱼，金黄花鳞和鱼鳍完整。",
    "鱿鱼": "两只完整鲜鱿鱼，紫褐色表皮与触手自然。",
    "花甲": "一小盆鲜活花甲，壳纹和微张壳口清楚。",
    "扇贝": "数只带壳鲜活扇贝，扇形壳和闭壳肌清楚。",
    "生蚝": "数只带壳生蚝，粗糙壳面与蚝肉自然。",
    "海带": "一卷深绿色新鲜海带或海带结，表面湿润。",
    "番茄": "三枚自然成熟番茄，颜色略有深浅差异，带绿色果蒂。",
    "土豆": "数枚带薄皮的新鲜黄心土豆，大小略有差异。",
    "青椒": "数只翠绿薄皮青椒，果柄完整。",
    "黄瓜": "两根带刺的新鲜黄瓜，颜色深浅自然。",
    "白菜": "一棵完整大白菜，外层鲜绿叶和内层嫩黄叶都清楚。",
    "菠菜": "一把带根新鲜菠菜，叶片舒展，茎叶分明。",
    "茄子": "两根紫黑色长茄子，表皮有自然光泽。",
    "西兰花": "一整颗新鲜西兰花，紧密绿色花球和粗壮茎。",
    "胡萝卜": "数根带少量泥土感但总体干净的新鲜胡萝卜。",
    "洋葱": "两枚紫皮和黄皮洋葱各一，干燥外皮自然。",
    "芹菜": "一把新鲜西芹，粗茎和嫩叶完整。",
    "生菜": "一棵新鲜圆生菜，叶片层层包紧，外缘自然舒展。",
    "上海青": "一小把新鲜上海青，根部完整、嫩叶翠绿。",
    "豆角": "一把细长新鲜豇豆，豆荚饱满笔直。",
    "四季豆": "一把新鲜四季豆，豆荚扁平翠绿。",
    "莴笋": "两根新鲜莴笋，带叶和未削皮嫩绿茎。",
    "冬瓜": "半个或一段带皮新鲜冬瓜，白霜与绿色瓜皮自然。",
    "南瓜": "半个带籽的成熟南瓜，橙黄果肉与硬皮清楚。",
    "玉米": "两根带苞叶的新鲜甜玉米，露出一部分金黄玉米粒。",
    "西葫芦": "两根嫩绿色西葫芦，表皮细纹自然。",
    "苦瓜": "两根翠绿色苦瓜，瘤状表皮清楚。",
    "山药": "数段带薄皮新鲜铁棍山药，断面洁白。",
    "莲藕": "两节新鲜莲藕，外皮浅褐、孔洞切面清楚。",
    "韭菜": "一把新鲜韭菜，叶窄、根部整齐。",
    "豆芽": "一小筐新鲜绿豆芽，芽身洁白、豆瓣淡黄。",
    "娃娃菜": "两棵新鲜小娃娃菜，嫩黄菜心与浅绿外层。",
    "青豆": "一小盘剥好的新鲜青豌豆，豆粒饱满。",
    "芦笋": "一小把新鲜绿芦笋，笋尖紧密、茎粗细自然。",
    "香菜": "一把新鲜香菜，细根、绿叶和嫩茎完整。",
    "青蒜": "一把新鲜青蒜，白根、绿叶分明。",
    "葱": "一把新鲜小葱，根部带少量泥土感、葱白和葱叶自然。",
    "蒜": "一整头带干皮的大蒜，旁边自然散落两瓣。",
    "姜": "一块新鲜老姜，表皮浅黄、分叉自然。",
    "香菇": "数朵新鲜厚肉香菇，褐色菌盖和浅色菌柄清楚。",
    "金针菇": "一把新鲜金针菇，白色细长菌柄和浅黄小菌盖紧密成束。",
    "平菇": "数朵新鲜平菇，灰白色扇形菌盖层叠。",
    "杏鲍菇": "数根新鲜杏鲍菇，白色粗壮菌柄和褐色菌盖。",
    "茶树菇": "一把干或鲜茶树菇，深褐菌盖与细长菌柄，形态真实。",
    "木耳": "一小碗泡发后的黑木耳，耳片自然卷曲。",
    "豆腐": "一块完整嫩豆腐，放在无文字透明保鲜盒中，切面细腻。",
    "豆干": "数块酱色豆腐干整齐叠放，不切丝。",
    "腐竹": "一把干燥腐竹长条自然散开，颜色金黄。",
    "百叶": "一卷新鲜千张百叶叠放，白色薄片层次清楚。",
    "豆皮": "数张新鲜淡黄色豆皮松散折叠。",
    "黄豆": "一小袋饱满干黄豆，透明袋无文字。",
    "毛豆": "一小袋带荚鲜毛豆，豆荚翠绿饱满。",
    "米饭": "一盒素色保鲜盒里的白米饭，米粒清楚，不是盖浇饭。",
    "面条": "一把新鲜湿面条自然分成两束，用素色纸带轻捆。",
    "挂面": "一包透明无字包装的干挂面，整齐长条清楚可见。",
    "馒头": "数个白面馒头装在素色食品袋中，袋口自然敞开。",
    "饺子皮": "一摞圆形白面饺子皮，边缘带少量干面粉，透明无字袋半包。",
    "面粉": "一袋素净纸袋或透明无字袋装白面粉，袋口自然折叠。",
    "糯米": "一小袋透明无字袋装白色圆糯米。",
    "年糕": "数条水磨年糕叠放，白色表面湿润。",
    "粉丝": "一捆透明干粉丝，线状和捆扎形态清楚。",
    "面包": "一整条无切片的原味吐司面包，装在素色无字纸袋里露出上半部。",
    "油面筋": "十几个圆润空心油面筋自然堆在透明无字保鲜袋中。",
    "春卷皮": "一摞薄圆春卷皮，用素色湿布或纸轻盖，边缘完整。",
    "苹果": "两枚红富士苹果，果皮红黄渐变，果梗清楚。",
    "香蕉": "一小串自然成熟的黄香蕉，带少量褐色斑点。",
    "柠檬": "两颗新鲜黄柠檬，果皮油胞清楚。",
    "菠萝": "一只完整带冠叶的新鲜菠萝，外皮纹理清楚。",
    "梨": "两枚不同深浅的黄绿色鲜梨，果梗和表皮斑点自然。",
    "橙子": "两枚新鲜脐橙，颜色略深浅不同，果蒂清楚。",
    "桃子": "两枚新鲜粉红色水蜜桃，表皮绒毛细腻。",
    "葡萄": "一串饱满的新鲜紫葡萄，果粉和果梗自然。",
    "草莓": "十余枚新鲜红草莓自然聚在一起，带绿色萼片。",
    "西瓜": "一块完整半月形鲜红西瓜，带青皮和黑籽，不切小块。",
    "牛奶": "两盒素白无字纸盒鲜牛奶，盒体干净且没有标签。",
    "酸奶": "两杯素白无字小杯酸奶，杯盖略有冷凝水。",
    "黄油": "一块完整淡黄色黄油，外包朴素无字油纸，部分露出。",
    "奶酪": "一块淡黄色奶酪与数片切片奶酪，包装简洁无标。",
    "咸鸭蛋": "三枚带灰青外壳的完整咸鸭蛋，不切开。",
    "豆浆": "一瓶素色无字玻璃瓶装的新鲜豆浆，瓶口密封。",
}

BASE_STYLE = (
    "超写实商业食材摄影，真实相机质感，自然柔和的侧上方光，淡暖白纯色背景，"
    "主体完整居中，单件或正常一小份自然摆放，四周至少留出百分之十二安全边距，"
    "边缘绝不裁切、不超出画面。构图简洁，只有指定食材，"
    "不要人物、手、筷子、刀叉、文字、数字、品牌、标签、二维码、水印、Logo、边框、海报感。"
    "不要卡通、插画、3D渲染、塑料模型或过度磨皮；不得出现其他无关食材或成品菜。"
)


def catalog() -> list[dict]:
    rows: list[dict] = []
    for group, names in GROUPS.items():
        for index, name in enumerate(names, 1):
            rows.append(
                {
                    "name": name,
                    "group": group,
                    "slug": f"{group}-{index:02d}",
                    "detail": ITEM_DIRECTION.get(name, f"真实的新鲜{name}食材，形态和颜色准确。"),
                }
            )
    names = [row["name"] for row in rows]
    if len(names) != len(set(names)):
        raise RuntimeError("pantry catalog contains duplicate ingredient names")
    return rows


def newest_png(directory: Path) -> Path | None:
    files = sorted(directory.rglob("*.png"), key=lambda path: path.stat().st_mtime)
    return files[-1] if files else None


def download_image(url: str, target: Path) -> Path:
    """Download a generated PNG with more patience than the Skill's 30s default."""
    target.parent.mkdir(parents=True, exist_ok=True)
    partial = target.with_suffix(".part")
    last_error = ""
    proxy = os.environ.get("ARK_IMAGE_DOWNLOAD_PROXY", "http://10.10.10.254:7890")
    for label, opener in (
        ("direct", urllib.request.build_opener(urllib.request.ProxyHandler({}))),
        ("proxy", urllib.request.build_opener(urllib.request.ProxyHandler({"http": proxy, "https": proxy}))),
    ):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "family-menu-pantry-builder/1.0"})
            with opener.open(request, timeout=180) as response, partial.open("wb") as out:
                while True:
                    chunk = response.read(1024 * 1024)
                    if not chunk:
                        break
                    out.write(chunk)
            data = partial.read_bytes()
            if len(data) < 20_000 or not data.startswith(b"\x89PNG\r\n\x1a\n"):
                raise RuntimeError("downloaded file is not a usable PNG")
            partial.replace(target)
            return target
        except Exception as exc:  # noqa: BLE001 - fall back from direct to proxy
            last_error = f"{label}: {exc}"
            partial.unlink(missing_ok=True)
    raise RuntimeError(f"image download failed ({last_error})")


def generate_one(row: dict, force: bool) -> tuple[str, str]:
    save_dir = OUT / row["slug"]
    existing = newest_png(save_dir)
    if existing and not force:
        return row["name"], f"SKIP {existing}"

    prompt = (
        f"拍摄一种家庭冰箱里的真实食材：{row['name']}。{row['detail']} "
        f"{GROUP_DIRECTION[row['group']]} {BASE_STYLE}"
    )
    key = json.loads(ARKCLI_KEY.read_text(encoding="utf-8"))["api_key"]
    env = os.environ.copy()
    env["ARK_SEEDREAM_API_KEY"] = key
    env["ARK_SEEDREAM_MODEL"] = "doubao-seedream-5.5"
    env["ARK_SEEDREAM_SAVE_PATH"] = str(save_dir)
    command = [
        str(NODE),
        str(GENERATE),
        "--prompt",
        prompt,
        "--size",
        "1024x1024",
        "--mode",
        "text-to-image",
        "--watermark",
        "false",
        "--optimize",
        "false",
        "--enable_web_search",
        "false",
        "--response_format",
        "png",
    ]
    last_error = ""
    for attempt in range(3):
        result = subprocess.run(
            command,
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=240,
        )
        if result.returncode == 0:
            try:
                payload = json.loads(result.stdout)
                image = payload["images"][0]
                local_path = image.get("local_path")
                if not local_path:
                    if not image.get("url"):
                        raise RuntimeError("generation returned neither a local file nor a URL")
                    local_path = str(download_image(image["url"], save_dir / "ingredient.png"))
                return row["name"], f"OK {local_path}"
            except Exception as exc:  # noqa: BLE001 - preserve the batch and report it
                last_error = f"parse failed: {exc}"
        else:
            last_error = (result.stdout or result.stderr).strip()[-800:]
            # The CLI returns a JSON object on stdout even on handled failures.
            try:
                payload = json.loads(result.stdout)
                last_error = str(payload.get("error") or last_error)
            except Exception:  # noqa: BLE001
                pass
        if attempt < 2:
            print(f"RETRY {row['name']} attempt={attempt + 2}/3: {last_error[:180]}", flush=True)
    raise RuntimeError(f"{row['name']}: {last_error}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--only", nargs="*", default=[])
    parser.add_argument("--group", choices=sorted(GROUPS))
    parser.add_argument("--limit", type=int)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--refresh", action="store_true")
    args = parser.parse_args()

    selected = catalog()
    if args.only:
        wanted = set(args.only)
        selected = [row for row in selected if row["name"] in wanted]
    if args.group:
        selected = [row for row in selected if row["group"] == args.group]
    if args.limit:
        selected = selected[: args.limit]
    if not selected:
        raise SystemExit("no pantry ingredient selected")

    OUT.mkdir(parents=True, exist_ok=True)
    failed: list[tuple[str, str]] = []
    with ThreadPoolExecutor(max_workers=max(1, args.workers)) as pool:
        futures = {
            pool.submit(generate_one, row, args.refresh): row
            for row in selected
        }
        for index, future in enumerate(as_completed(futures), 1):
            row = futures[future]
            try:
                name, message = future.result()
                print(f"[{index}/{len(selected)}] {name} {message}", flush=True)
            except Exception as exc:  # noqa: BLE001 - continue generating the rest
                failed.append((row["name"], str(exc)))
                print(f"[{index}/{len(selected)}] FAIL {row['name']}", flush=True)

    print(
        f"done total={len(selected)} ok={len(selected) - len(failed)} failed={len(failed)}",
        flush=True,
    )
    for name, error in failed:
        print(f"FAILED {name}: {error}", flush=True)
    if failed:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
