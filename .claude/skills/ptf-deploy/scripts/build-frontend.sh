#!/usr/bin/env bash
# PT-Forward 前端验证 + 构建 + 嵌入 + 部署（仅限开发环境 29）
# 前置条件：改动已 commit + push（先提交后部署铁律）
# 用法：bash .github/skills/ptf-deploy/scripts/build-frontend.sh
set -euo pipefail
cd "$(dirname "$0")/../../../.."

GO=/home/incast/.local/go/bin/go
export PATH="/home/incast/.local/bin:$PATH"  # Node v22（系统 Node 18 不兼容）

if [ -n "$(git status --porcelain)" ]; then
  echo "⚠️  工作区有未提交改动（铁律：先提交后部署）：" >&2
  git status --short >&2
  exit 1
fi

echo "==> [1/5] vue-tsc + eslint"
cd web
./node_modules/.bin/vue-tsc -b --noEmit
npx eslint src/

echo "==> [2/5] vite build（清 vite 缓存）"
rm -rf node_modules/.vite
./node_modules/.bin/vite build
cd ..

echo "==> [3/5] 嵌入 frontend/dist（go build embed 的目标是它，不是 web/src）"
rm -rf frontend/dist
cp -r web/dist frontend/dist

echo "==> [4/5] go build（CGO_ENABLED=1 + 版本号 ldflags）"
CGO_ENABLED=1 "$GO" build \
  -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" \
  -o pt-forward ./cmd/pt-forward/

echo "==> [5/5] 重启服务"
systemctl --user restart pt-forward
sleep 2
systemctl --user is-active pt-forward
echo "✅ 前端部署完成"
