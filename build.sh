#!/bin/sh
# 在 Linux / macOS / WSL 上构建单文件服务端。
set -e
cd "$(dirname "$0")"
node scripts/build.mjs
echo "完成：./menu（Linux/macOS）"
