#!/usr/bin/env bash
# PT-Forward 部署到 243 生产环境（Docker）
# 前置条件：已 commit+push、已跑 build-backend.sh（29 验证通过）、前端如涉及已跑 build-frontend.sh
# 用法：bash .claude/skills/ptf-deploy/scripts/deploy-243.sh
set -euo pipefail
cd "$(dirname "$0")/../../../.."

SSH_HOST=root@10.0.0.243
CONTAINER=pt-forward

if [ ! -f pt-forward ]; then
  echo "❌ 缺 pt-forward 二进制——先跑 build-backend.sh" >&2
  exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "⚠️  工作区有未提交改动（铁律：先提交后部署）：" >&2
  git status --short >&2
  exit 1
fi

LOCAL_VER=$(./pt-forward --version 2>/dev/null | tail -1)
echo "==> [1/4] 传输二进制（gzip+ssh 管道）: ${LOCAL_VER}"
gzip -c pt-forward | ssh "$SSH_HOST" "cat > /tmp/pt.gz && gunzip -f /tmp/pt.gz && chmod +x /tmp/pt"

echo "==> [2/4] 原子替换 + 重启容器"
ssh "$SSH_HOST" "docker cp /tmp/pt ${CONTAINER}:/usr/local/bin/.new && docker exec ${CONTAINER} sh -c 'mv -f /usr/local/bin/.new /usr/local/bin/pt-forward' && rm /tmp/pt && docker restart ${CONTAINER}"

echo "==> [3/4] 等待启动 + 版本核对"
sleep 15
REMOTE_VER=$(ssh "$SSH_HOST" "docker exec ${CONTAINER} /usr/local/bin/pt-forward --version | tail -1")
echo "    243: ${REMOTE_VER}"
if [ "$REMOTE_VER" != "$LOCAL_VER" ]; then
  echo "❌ 版本不一致（本地 ${LOCAL_VER} vs 243 ${REMOTE_VER}）" >&2
  exit 1
fi

echo "==> [4/4] 前端资产一致性"
LOCAL_JS=$(ls frontend/dist/assets/ 2>/dev/null | grep -E '^index-.*\.js$' | head -1)
REMOTE_JS=$(ssh "$SSH_HOST" "curl -s --noproxy '*' http://127.0.0.1:8765/ | grep -oE 'index-[A-Za-z0-9_-]+\.js' | head -1")
echo "    本地: ${LOCAL_JS}  243: ${REMOTE_JS}"
if [ "$LOCAL_JS" != "$REMOTE_JS" ]; then
  echo "❌ 前端资产不一致——build-frontend.sh 是否已跑？" >&2
  exit 1
fi

echo "✅ 243 部署完成：${REMOTE_VER} / ${REMOTE_JS}"
