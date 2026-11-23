#!/usr/bin/env python3
"""Check local links in source/docs and optionally probe external URLs."""

from __future__ import annotations

import argparse
import re
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import unquote, urlparse
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[1]
EXTENSIONS = {".md", ".html", ".vue", ".js", ".mjs"}
SKIP_PARTS = {
    ".git", "node_modules", "dist", "webapp", ".tmp", ".v2", ".p1",
    ".workbuddy", ".workbuddy-ai", "_cq", "_src_images",
}
SKIP_FILES = {"_src_dishes.html", "_src_dishes.json"}
PUBLIC_ROUTES = {"/", "/privacy", "/terms", "/sitemap.xml", "/robots.txt"}
MD_LINK = re.compile(r"!?\[[^\]]*\]\(([^)\s]+)")
ATTR_LINK = re.compile(r"""\b(?:href|src)=["']([^"']+)["']""")
ID_ATTR = re.compile(r"""\bid=["']([^"']+)["']""")


def source_files() -> list[Path]:
    files = []
    for path in ROOT.rglob("*"):
        if not path.is_file() or path.suffix.lower() not in EXTENSIONS:
            continue
        rel = path.relative_to(ROOT)
        if any(part in SKIP_PARTS for part in rel.parts) or path.name in SKIP_FILES:
            continue
        files.append(path)
    return files


def local_target(source: Path, target: str) -> bool:
    target = target.replace("__SITE_URL__", "")
    parsed = urlparse(target)
    path = unquote(parsed.path)
    if not path:
        if parsed.fragment:
            ids = set(ID_ATTR.findall(source.read_text(encoding="utf-8")))
            return parsed.fragment in ids
        return True
    if path.startswith("/"):
        if path in PUBLIC_ROUTES:
            return True
        public = ROOT / "web" / "public" / path.lstrip("/")
        source_asset = ROOT / "web" / path.lstrip("/")
        return public.exists() or source_asset.exists()
    resolved = (source.parent / path).resolve()
    try:
        resolved.relative_to(ROOT)
    except ValueError:
        return False
    return resolved.exists()


def external_ok(url: str) -> tuple[bool, str]:
    request = Request(url, method="HEAD", headers={"User-Agent": "Home-Feast-LinkCheck/1.0"})
    try:
        with urlopen(request, timeout=8) as response:
            return response.status < 400, str(response.status)
    except HTTPError as err:
        if err.code in (403, 405, 429):
            return True, f"{err.code} (restricted but reachable)"
        return False, str(err.code)
    except URLError as err:
        return False, str(err.reason)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--external", action="store_true", help="also probe external http(s) links")
    args = parser.parse_args()

    failures = []
    checked = 0
    external_seen = set()
    for source in source_files():
        text = source.read_text(encoding="utf-8", errors="replace")
        targets = MD_LINK.findall(text) + ATTR_LINK.findall(text)
        for target in targets:
            if (
                not target.startswith(("/", "#", "http://", "https://", "./", "../"))
                and not re.match(r"^[\w.-]+(?:/[\w./-]+)+$", target)
            ):
                continue
            if target.startswith(("javascript:", "mailto:", "tel:", "data:")):
                continue
            parsed = urlparse(target)
            if parsed.scheme in ("http", "https"):
                if args.external and target not in external_seen:
                    external_seen.add(target)
                    ok, status = external_ok(target)
                    print(f"{'OK  ' if ok else 'FAIL'} external {target} [{status}]")
                    if not ok:
                        failures.append((source.relative_to(ROOT), target))
                continue
            checked += 1
            if not local_target(source, target):
                failures.append((source.relative_to(ROOT), target))

    print(f"\nchecked {checked} local links, {len(external_seen)} external links")
    for source, target in failures:
        print(f"BROKEN {source}: {target}")
    if failures:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
