#!/usr/bin/env bash
# PT-Forward 打 tag 发布：触发 CI 构建 Docker 镜像（GHCR + Docker Hub）+ GitHub Release 二进制（OTA 用）
# 前置条件：改动已 commit（push 由本脚本完成）；AGENTS.md 版本行已同步为本 tag
# 用法：bash .claude/skills/ptf-deploy/scripts/release-tag.sh v0.0.XXX
#
# §59.542 追记七 推送顺序铁律（v0.0.1525 固化）：tag 先推、main 自动补推。
# - tag 先推 = tag run（Docker Publish/Release）开工时版本号正确（describe 见 tag）
# - main 补推 = main run（Docker Publish/CI）开工时 tag 已在远端 → describe 同样
#   输出正式版本号 → 两个 Docker Publish 产物同版本号 → latest 无竞争
# - 根治 v0.0.1521「main 忘推」违规（脚本自动补推·不再依赖人记得）
# - 反例（v0.0.1522-1524 曾用）：先 push main 再 tag → main run 开工时 tag 未到
#   → 版本号=commit hash → 与 tag run 产物竞争 latest（243 拉到 1ed629b 案）
set -euo pipefail
cd "$(dirname "$0")/../../../.."

if [ -z "${1:-}" ]; then
  echo "用法: $0 v0.0.XXX" >&2
  exit 1
fi
TAG="$1"

# §59.542 追记七: 必须在 main 分支上打 tag（否则 push main 会推错分支）
CUR_BRANCH=$(git branch --show-current)
if [ "$CUR_BRANCH" != "main" ]; then
  echo "❌ 当前分支为 ${CUR_BRANCH:-（detached）}——发版必须在 main 分支上" >&2
  exit 1
fi

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
    echo "   先更新版本行（含任务焦点叙述）并 commit，再打 tag" >&2
    exit 1
  fi
else
  echo "❌ 未找到 AGENTS.md（应在仓库根目录）" >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "❌ 工作区有未提交改动，先 commit：" >&2
  git status --short >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "❌ tag $TAG 已存在" >&2
  exit 1
fi

# ═══ 顺序铁律：tag 先推（版本号正确）→ main 自动补推（同版本号·无竞争）═══
git tag "$TAG"
git push origin "$TAG"
echo "✅ ① tag $TAG 已推送（tag run 先开工：Docker Publish + Release·版本号=正式版）"

git push origin main
echo "✅ ② main 已补推（main run 后开工：tag 已在远端 → Docker Publish + CI 同版本号 → latest 无竞争）"
echo ""
echo "   健康模式（四行齐全·tag 行先出）："
echo "     Docker Publish（tag）+ Release（tag）→ 先"
echo "     Docker Publish（main）+ CI（main）    → 后"
echo "   CI 完成后：GHCR + Docker Hub 镜像；GitHub Release 二进制（OTA）"
echo "   提醒：生产环境由用户自行更新（优先 OTA·基础镜像变更才 docker compose pull）"
