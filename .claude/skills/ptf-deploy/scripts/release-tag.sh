#!/usr/bin/env bash
# PT-Forward 打 tag 发布：触发 CI 构建 Docker 镜像（GHCR + Docker Hub）+ GitHub Release 二进制（OTA 用）
# 前置条件：改动已 commit + push 到 main；AGENTS.md 版本行已同步为本 tag
# 用法：bash .claude/skills/ptf-deploy/scripts/release-tag.sh v0.0.XXX
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

# §59.318 二轮回归定案：AGENTS 版本行【前置强校验】。
# 历史七次滞后同型（v0.0.631/642/670/830/835/1084/1087）——§59.166 曾加后置
# sed 自动同步，但版本行格式漂移（"（已发布"→"（29 已部署；"）致模式静默失配
# 从未生效；且打 tag 后补 commit 造成 tag 指向滞后态。根治：打 tag 前版本行
# 必须已等于 tag（版本行含任务焦点叙述，只能人工更新——脚本只做门禁）。
if [ -f AGENTS.md ]; then
  AGENTS_VER=$(grep -oP '^\*\*版本\*\*：\Kv[0-9]+\.[0-9]+\.[0-9]+' AGENTS.md 2>/dev/null | head -1 || true)
  if [ -z "$AGENTS_VER" ]; then
    echo "❌ AGENTS.md 未找到版本行（**版本**：vX.Y.Z...）——格式漂移？先修版本行" >&2
    exit 1
  fi
  if [ "$AGENTS_VER" != "$TAG" ]; then
    echo "❌ AGENTS.md 版本行($AGENTS_VER) ≠ 将打的 tag($TAG)" >&2
    echo "   先更新版本行（含任务焦点叙述）并 commit+push，再打 tag" >&2
    exit 1
  fi
else
  echo "❌ 未找到 AGENTS.md（应在仓库根目录）" >&2
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
echo "✅ 已推送 $TAG（AGENTS 版本行 $AGENTS_VER 前置校验通过）"
echo "   CI 将自动：docker-publish.yml → GHCR + Docker Hub 镜像；release.yml → GitHub Release 二进制（OTA）"
echo "   提醒：生产环境由用户自行更新（OTA 或 docker compose pull && docker compose up -d）"
