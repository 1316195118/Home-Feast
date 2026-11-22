#!/usr/bin/env python3
"""Audit the curated dish photos specifically for generous, realistic portions.

The general visual audit is deliberately broad. This second pass treats the
user's "分量要足" requirement as a hard acceptance gate: a photo must look like
a normal full restaurant serving, not a tasting portion or a mostly empty plate.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
from pathlib import Path

from audit_dish_visuals import ARKCLI, PROFILE, ROOT, strip_fence


DATA = ROOT / "data" / "dishes.json"
MAP = ROOT / "web" / "src" / "dish-photo-map.json"
PHOTOS = ROOT / "web" / "public" / "dish-photo"
DEFAULT_OUT = ROOT / "scripts" / "_cq" / "_img" / "portion-current"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", type=Path, default=DEFAULT_OUT)
    parser.add_argument("--batch-size", type=int, default=3)
    parser.add_argument("--refresh", action="store_true")
    parser.add_argument("--limit", type=int)
    parser.add_argument("--only", nargs="*", default=[])
    return parser.parse_args()


def prompt_for(batch: list[dict]) -> str:
    expected = []
    for index, item in enumerate(batch, 1):
        clues = "；".join(
            part
            for part in (
                str(item.get("desc") or "").strip(),
                "、".join(str(tag) for tag in item.get("tags") or []),
            )
            if part
        )
        expected.append(f"{index}. {item['name']}（{clues or '无补充说明'}）")
    return (
        "从严检查这些中国菜图片是否达到“家常餐馆正常一整份、分量充足”的标准。"
        "输入顺序与菜名严格对应："
        + "；".join(expected)
        + "。只根据画面判断，不要因为菜名就假定分量足。"
        "重点判断主料是否成堆、堆头是否饱满、盛器是否明显大于食物、盘底或汤面是否空得过多。"
        "汤羹可让液面充足，但可见主料不能只有零星几片；面饭、炒菜、烤肉和点心应按正常一人至两人份出现。"
        "文思豆腐、西湖莼菜汤这类细丝或细叶汤菜属于特例：不要要求主料像炒菜一样铺满整碗面积；"
        "应以汤盅大小是否匹配、液面是否接近正常一份、细料密度是否充足来判断，正常餐馆汤碗且主料细密分布即可视为足量。"
        "仅输出 JSON 数组，不要 Markdown。每张图一个对象，字段："
        "index（从1起）、dish_guess、portion_generous（严格布尔值，明显丰盛才为 true）、"
        "portion_score（1到5，3表示普通餐馆一份，1表示像试吃或空盘）、"
        "main_food_area_ratio（主料在盛器内腔中可见面积占比，0到1，排除汤汁和纯装饰）、"
        "container_fill（食物和正常汤汁合计占据盛器内腔的视觉比例，0到1）、"
        "sparse_reason（若不够足，简短说明是主料少、盘太大、摊得薄、碗太空还是其他原因；合格则为空字符串）、"
        "reason（一句话验收理由）。"
        "portion_generous 只有 portion_score>=3 且没有明显空旷感时才可以为 true。"
    )


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
    command = [str(ARKCLI), "+understand", "image-caption"]
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
            str(max(1200, len(batch) * 260)),
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
            f"expected {len(batch)} checks, got "
            f"{len(checks) if isinstance(checks, list) else type(checks).__name__}"
        )
    return checks, payload.get("usage") or {}


def latest_successes(checkpoint: Path) -> dict[str, dict]:
    records: dict[str, dict] = {}
    if not checkpoint.exists():
        return records
    for line in checkpoint.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        record = json.loads(line)
        if record.get("status") == "ok":
            records[record["name"]] = record
    return records


def main() -> None:
    args = parse_args()
    out_dir = args.out.resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    checkpoint = out_dir / "results.jsonl"

    mapping = json.loads(MAP.read_text(encoding="utf-8"))
    dishes = {dish["name"]: dish for dish in json.loads(DATA.read_text(encoding="utf-8"))}
    items = []
    for name, slug in sorted(mapping.items()):
        path = (PHOTOS / f"{slug}.webp").resolve()
        if not path.exists():
            raise SystemExit(f"missing curated photo: {path}")
        dish = dishes.get(name, {})
        items.append(
            {
                "name": name,
                "path": path,
                "desc": dish.get("desc"),
                "tags": dish.get("tags"),
            }
        )
    if args.limit:
        items = items[: args.limit]
    if args.only:
        selected = set(args.only)
        items = [item for item in items if item["name"] in selected]

    done = {} if args.refresh else latest_successes(checkpoint)
    pending = [item for item in items if item["name"] not in done]
    batches = [
        pending[index : index + args.batch_size]
        for index in range(0, len(pending), args.batch_size)
    ]
    print(
        f"total={len(items)} cached={len(items) - len(pending)} "
        f"pending={len(pending)} batches={len(batches)}",
        flush=True,
    )

    with checkpoint.open("a", encoding="utf-8") as handle:
        for number, batch in enumerate(batches, 1):
            print(f"[{number}/{len(batches)}] {'、'.join(item['name'] for item in batch)}", flush=True)
            error: Exception | None = None
            for attempt in range(1, 5):
                try:
                    checks, usage = call_ark(batch)
                    break
                except Exception as exc:  # noqa: BLE001 - retry model/upload failures
                    error = exc
                    print(f"  attempt {attempt} failed: {exc}", flush=True)
            else:
                for item in batch:
                    handle.write(
                        json.dumps(
                            {
                                "name": item["name"],
                                "path": str(item["path"]),
                                "status": "error",
                                "error": str(error),
                            },
                            ensure_ascii=False,
                        )
                        + "\n"
                    )
                handle.flush()
                continue

            for item, check in zip(batch, checks):
                handle.write(
                    json.dumps(
                        {
                            "name": item["name"],
                            "path": str(item["path"]),
                            "status": "ok",
                            "usage": usage,
                            **check,
                        },
                        ensure_ascii=False,
                    )
                    + "\n"
                )
            handle.flush()

    records = latest_successes(checkpoint)
    expected = {item["name"] for item in items}
    latest = {name: record for name, record in records.items() if name in expected}
    issues = [
        record
        for record in latest.values()
        if not record.get("portion_generous")
        or int(record.get("portion_score") or 0) < 3
        or record.get("sparse_reason")
    ]
    report = {
        "audited": len(latest),
        "expected": len(items),
        "issues": sorted(issues, key=lambda item: item["name"]),
        "records": sorted(latest.values(), key=lambda item: item["name"]),
    }
    report_path = out_dir / "report.json"
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(
        f"done audited={len(latest)}/{len(items)} issues={len(issues)} report={report_path}",
        flush=True,
    )
    for item in report["issues"]:
        print(
            f"ISSUE {item['name']} score={item.get('portion_score')} "
            f"area={item.get('main_food_area_ratio')} fill={item.get('container_fill')} "
            f"reason={item.get('sparse_reason') or item.get('reason')}",
            flush=True,
        )


if __name__ == "__main__":
    main()
