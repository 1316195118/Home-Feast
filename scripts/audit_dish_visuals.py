#!/usr/bin/env python3
"""Use the Ark image-understanding recipe to audit every dish illustration.

The existing framing audit is intentionally conservative and produces many
candidates. This script adds content matching and a second opinion on whether
the main subject is centered, safely inside the frame, and complete.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

from generate_dish_repairs import OUT as REPAIR_OUT, REPAIRS


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_DATA = ROOT / "data" / "dishes.json"
DEFAULT_REPORT = ROOT / "scripts" / "_cq" / "_img" / "audit-current" / "report.json"
DEFAULT_OUT = ROOT / "scripts" / "_cq" / "_img" / "visual-current"
DEFAULT_GENERATED = ROOT / "scripts" / "_cq" / "_gen_seedream55"
ARKCLI = Path(r"C:\Users\Administrator\.local\bin\.arkcli-managed\arkcli.cmd")
PROFILE = "platform_cn-beijing_accountwide"

GENERATED = {
    "烤冷面": DEFAULT_GENERATED / "kaolengmian" / "ark-gen.png",
    "油面筋塞肉": DEFAULT_GENERATED / "youmianjin-sairou" / "2026-11-22" / "seedream_1795343343_1.png",
    "响油鳝糊": DEFAULT_GENERATED / "xiangyou-shanhu" / "2026-11-22" / "seedream_1795343536_1.png",
    "溜肉段": DEFAULT_GENERATED / "liurouduan" / "2026-11-22" / "seedream_1795343375_1.png",
    "酱大骨": DEFAULT_GENERATED / "jiangdagu" / "2026-11-22" / "seedream_1795343389_1.png",
    "太平燕": DEFAULT_GENERATED / "taipingyan" / "2026-11-22" / "seedream_1795343406_1.png",
    "红糟肉": DEFAULT_GENERATED / "hongzaorou" / "2026-11-22" / "seedream_1795343419_1.png",
    "清蒸石鸡": DEFAULT_GENERATED / "qingzheng-shiji" / "2026-11-22" / "seedream_1795343433_1.png",
    "问政山笋": DEFAULT_GENERATED / "wenzheng-shansun" / "2026-11-22" / "seedream_1795343448_1.png",
    "蜜汁火方": DEFAULT_GENERATED / "mizhi-huofang" / "2026-11-22" / "seedream_1795343467_1.png",
    "西湖莼菜汤": DEFAULT_GENERATED / "xihu-chuncaitang" / "2026-11-22" / "seedream_1795343481_1.png",
    "湘西酸肉": DEFAULT_GENERATED / "xiangxi-suanrou" / "2026-11-22" / "seedream_1795343493_1.png",
    "皮芽子馕": DEFAULT_GENERATED / "piyazinaan" / "2026-11-22" / "seedream_1795343507_1.png",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", type=Path, default=DEFAULT_DATA)
    parser.add_argument("--report", type=Path, default=DEFAULT_REPORT)
    parser.add_argument("--out", type=Path, default=DEFAULT_OUT)
    parser.add_argument("--batch-size", type=int, default=3)
    parser.add_argument("--limit", type=int)
    parser.add_argument("--refresh", action="store_true")
    parser.add_argument("--repairs-only", action="store_true")
    parser.add_argument("--only", nargs="*", default=[])
    parser.add_argument("--curated-only", action="store_true")
    return parser.parse_args()


def image_map(report_path: Path) -> dict[str, Path]:
    report = json.loads(report_path.read_text(encoding="utf-8"))
    source = report_path.parent / "source"
    return {
        record["name"]: (source / record["file"]).resolve()
        for record in report["records"]
    }


def prompt_for(batch: list[dict]) -> str:
    expected = []
    for index, dish in enumerate(batch, 1):
        desc = str(dish.get("desc") or "").strip()
        tags = "、".join(str(tag) for tag in dish.get("tags") or [])
        clues = "；".join(part for part in (desc, tags) if part)
        expected.append(f"{index}. {dish['name']}（{clues or '无可补充说明'}）")
    return (
        "按输入顺序检查这些中国菜品图片。输入顺序与以下菜名严格对应："
        + "；".join(expected)
        + "。只判断图片画面本身，不要因为文件名或菜名而假定画面对。"
        "只输出 JSON 数组，不要 Markdown。每张图一个对象，字段："
        "index（从1起）、dish_guess、content_match（主体是否明显对应预期菜名）、"
        "centered（菜品与盛器是否大致居中）、safe_margin（四周是否有足够留白）、"
        "cropped（食材或盛器是否被边缘裁断）、visual_issue（简短指出错菜、"
        "明显无关物品、餐具、手、文字、裁切或构图问题；没有则为空字符串）、"
        "vessel_match（盛器是否符合该菜系和菜型，不能与其他菜共用明显不合适的盘子）、"
        "portion_full（主料分量是否充足饱满、不显得空盘）、reason。"
        "content_match 仅在主体明显对应时为 true。"
    )


def strip_fence(value: str) -> str:
    value = value.strip()
    match = re.fullmatch(r"```(?:json)?\s*(.*?)\s*```", value, flags=re.S | re.I)
    return match.group(1).strip() if match else value


def call_ark(batch: list[dict]) -> tuple[list[dict], dict]:
    env = os.environ.copy()
    env.update(
        {
            "ARKCLI_NO_UPDATE_NOTIFIER": "1",
            "ARKCLI_CALLER_TYPE": "ai_agent",
            "ARKCLI_CALLER_NAME": "codex",
            "ARKCLI_SKILL_NAME": "arkcli-understand",
        }
    )
    command = [
        str(ARKCLI),
        "+understand",
        "image-caption",
    ]
    for item in batch:
        command.extend(["--input", f"@{item['path']}"])
    command.extend(
        [
            prompt_for(batch),
            "--profile",
            PROFILE,
            "--format",
            "json",
            "--no-progress",
            "--thinking",
            "disabled",
            "--reasoning-effort",
            "minimal",
            "--max-output-tokens",
            str(max(1000, len(batch) * 220)),
            "--timeout",
            "4m",
        ]
    )
    result = subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        timeout=300,
    )
    if result.returncode:
        raise RuntimeError(
            f"arkcli exited {result.returncode}: "
            f"{(result.stderr or result.stdout).strip()[:1200]}"
        )
    payload = json.loads(result.stdout)
    checks = json.loads(strip_fence(payload["content"]))
    if not isinstance(checks, list) or len(checks) != len(batch):
        raise RuntimeError(
            f"expected {len(batch)} checks, got {len(checks) if isinstance(checks, list) else type(checks).__name__}"
        )
    return checks, payload.get("usage") or {}


def main() -> None:
    args = parse_args()
    out_dir = args.out.resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    checkpoint = out_dir / "results.jsonl"

    dishes = json.loads(args.data.read_text(encoding="utf-8"))
    current = image_map(args.report)
    repairs = {
        dish: max((REPAIR_OUT / slug).rglob("*.png"), key=lambda path: path.stat().st_mtime)
        for dish, slug, _ in REPAIRS
        if list((REPAIR_OUT / slug).rglob("*.png"))
    }
    items: list[dict] = []
    missing_sources: list[str] = []
    for dish in dishes:
        path = repairs.get(dish["name"]) or GENERATED.get(dish["name"]) or current.get(dish["name"])
        if not path or not path.exists():
            if dish.get("image") or dish["name"] in GENERATED:
                missing_sources.append(dish["name"])
            continue
        items.append({"name": dish["name"], "desc": dish.get("desc"), "tags": dish.get("tags"), "path": path})
    if missing_sources:
        raise SystemExit(f"missing audit sources: {', '.join(missing_sources)}")
    if args.limit:
        items = items[: args.limit]
    if args.repairs_only:
        repair_names = {dish for dish, _, _ in REPAIRS}
        items = [item for item in items if item["name"] in repair_names]
    if args.only:
        selected = set(args.only)
        items = [item for item in items if item["name"] in selected]
    if args.curated_only:
        curated = json.loads(
            (ROOT / "web" / "src" / "dish-photo-map.json").read_text(encoding="utf-8")
        )
        items = [item for item in items if item["name"] in curated]

    done: dict[str, dict] = {}
    if checkpoint.exists() and not args.refresh:
        for line in checkpoint.read_text(encoding="utf-8").splitlines():
            if not line.strip():
                continue
            record = json.loads(line)
            if record.get("status") == "ok":
                done[record["name"]] = record

    pending = [item for item in items if item["name"] not in done]
    batches = [
        pending[index : index + args.batch_size]
        for index in range(0, len(pending), args.batch_size)
    ]
    print(f"total={len(items)} cached={len(items) - len(pending)} pending={len(pending)} batches={len(batches)}", flush=True)

    with checkpoint.open("a", encoding="utf-8") as handle:
        for number, batch in enumerate(batches, 1):
            names = [item["name"] for item in batch]
            print(f"[{number}/{len(batches)}] {'、'.join(names)}", flush=True)
            error: Exception | None = None
            for attempt in range(1, 4):
                try:
                    checks, usage = call_ark(batch)
                    break
                except Exception as exc:  # noqa: BLE001 - retry transient uploads/model errors
                    error = exc
                    print(f"  attempt {attempt} failed: {exc}", file=sys.stderr, flush=True)
            else:
                for item in batch:
                    record = {
                        "name": item["name"],
                        "path": str(item["path"]),
                        "status": "error",
                        "error": str(error),
                    }
                    handle.write(json.dumps(record, ensure_ascii=False) + "\n")
                handle.flush()
                continue

            for item, check in zip(batch, checks):
                record = {
                    "name": item["name"],
                    "path": str(item["path"]),
                    "status": "ok",
                    "usage": usage,
                    **check,
                }
                handle.write(json.dumps(record, ensure_ascii=False) + "\n")
            handle.flush()

    records = []
    for line in checkpoint.read_text(encoding="utf-8").splitlines():
        if line.strip():
            record = json.loads(line)
            if record.get("status") == "ok":
                records.append(record)
    expected_names = {item["name"] for item in items}
    latest = {
        record["name"]: record
        for record in records
        if record["name"] in expected_names
    }
    issues = [
        record
        for record in latest.values()
        if not record.get("content_match")
        or not record.get("centered")
        or not record.get("safe_margin")
        or record.get("cropped")
        or not record.get("vessel_match")
        or not record.get("portion_full")
        or record.get("visual_issue")
    ]
    report = {
        "audited": len(latest),
        "expected": len(items),
        "issues": sorted(issues, key=lambda record: record["name"]),
        "records": sorted(latest.values(), key=lambda record: record["name"]),
    }
    (out_dir / "report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(
        f"done audited={len(latest)}/{len(items)} issues={len(issues)} report={out_dir / 'report.json'}",
        flush=True,
    )


if __name__ == "__main__":
    main()
