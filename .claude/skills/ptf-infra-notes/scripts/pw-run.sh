#!/usr/bin/env bash
# Playwright 容器化运行封装（§59.499 工具化——用户指令：Docker 容器应开发调试工具化）
#
# 用法：bash .claude/skills/ptf-infra-notes/scripts/pw-run.sh /tmp/opencode/pwtest/xxx.js
#
# 固化的经验（免每次重查）：
#   - 镜像 mcr.microsoft.com/playwright:v1.53.0-noble（自带 node+chromium·与 playwright-core 配套）
#   - 浏览器路径容器内 /ms-playwright/chromium-*/chrome-linux/chrome（容器内探测·镜像升级自动适配）
#   - playwright-core npm 包在 /tmp/opencode/node_modules（缺则提示安装）
#   - 脚本所在目录挂载为 /pwtest（截图等产物直接落宿主机）
#   - --network host：访问内网目标（10.0.0.x:8765 等）
#   - 29 宿主直跑 chromium 缺 libnspr4 等系统库——必须走容器
#
# 脚本内约定：
#   const { chromium } = require('playwright-core');
#   chromium.launch({ executablePath: process.env.CHROME_PATH })
#   登录注入：localStorage.setItem('pt-forward-access-token', token)
set -euo pipefail

SCRIPT="$(realpath "${1:?用法: pw-run.sh <script.js>}")"
IMG="mcr.microsoft.com/playwright:v1.53.0-noble"
NODEMODS="/tmp/opencode/node_modules"

if [ ! -d "$NODEMODS/playwright-core" ]; then
  echo "缺 $NODEMODS/playwright-core —— 先安装：npm --prefix /tmp/opencode install playwright-core@1.53.0" >&2
  exit 1
fi

DIR="$(dirname "$SCRIPT")"
BASE="$(basename "$SCRIPT")"

echo "==> [1/3] 探测容器内 chromium 路径"
CHROME=$(sg docker -c "docker run --rm $IMG sh -c 'ls -d /ms-playwright/chromium-*/chrome-linux/chrome | head -1'")
[ -n "$CHROME" ] || { echo "镜像内未找到 chromium" >&2; exit 1; }
echo "    $CHROME"

echo "==> [2/3] 容器运行 $BASE"
sg docker -c "docker run --rm -e CHROME_PATH=$CHROME -v $DIR:/pwtest -v $NODEMODS:/nodemodules:ro -e NODE_PATH=/nodemodules --network host $IMG node /pwtest/$BASE"
echo "==> [3/3] 完成（产物在 $DIR/）"
