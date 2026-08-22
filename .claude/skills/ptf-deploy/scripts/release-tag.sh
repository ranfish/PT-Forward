#!/usr/bin/env bash
# PT-Forward 打 tag 发布：触发 CI 构建 Docker 镜像（GHCR + Docker Hub）+ GitHub Release 二进制（OTA 用）
# 前置条件：改动已 commit + push 到 main
# 用法：bash .github/skills/ptf-deploy/scripts/release-tag.sh v0.0.XXX
set -euo pipefail
cd "$(dirname "$0")/../../../.."

if [ -z "${1:-}" ]; then
  echo "用法: $0 v0.0.XXX" >&2
  exit 1
fi
TAG="$1"

if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "❌ tag 格式应为 vX.X.X，收到：$TAG" >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "❌ 工作区有未提交改动，先 commit + push：" >&2
  git status --short >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "❌ tag $TAG 已存在" >&2
  exit 1
fi

git tag "$TAG"
git push origin "$TAG"
echo "✅ 已推送 $TAG"
echo "   CI 将自动：docker-publish.yml → GHCR + Docker Hub 镜像；release.yml → GitHub Release 二进制（OTA）"
echo "   提醒：生产环境由用户自行更新（OTA 或 docker compose pull && docker compose up -d）"
