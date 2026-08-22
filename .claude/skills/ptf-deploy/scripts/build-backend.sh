#!/usr/bin/env bash
# PT-Forward 后端验证 + 编译 + 部署（仅限开发环境 29）
# 前置条件：改动已 commit + push（先提交后部署铁律）
# 用法：bash .github/skills/ptf-deploy/scripts/build-backend.sh
set -euo pipefail
cd "$(dirname "$0")/../../../.."

GO=/home/incast/.local/go/bin/go

if [ -n "$(git status --porcelain)" ]; then
  echo "⚠️  工作区有未提交改动（铁律：先提交后部署）：" >&2
  git status --short >&2
  exit 1
fi

echo "==> [1/4] go vet（不用 ./...，cmd/verify-pieces-hash 有已知冲突）"
"$GO" vet ./internal/... ./cmd/pt-forward/

echo "==> [2/4] go test"
"$GO" test ./internal/... -count=1 -timeout 180s

echo "==> [3/4] go build（CGO_ENABLED=1 + 版本号 ldflags）"
CGO_ENABLED=1 "$GO" build \
  -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" \
  -o pt-forward ./cmd/pt-forward/

echo "==> [4/4] 重启服务"
systemctl --user restart pt-forward
sleep 2
systemctl --user is-active pt-forward
echo "✅ 后端部署完成：$(./pt-forward --version 2>/dev/null || git describe --tags --always --dirty)"
