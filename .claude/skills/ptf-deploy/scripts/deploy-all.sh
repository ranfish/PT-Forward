#!/usr/bin/env bash
# PT-Forward 全链部署单入口（§59.163 治本：消灭调用侧 `| tail` 吞退出码——四次事故同型）
# 串行执行：build-frontend → build-backend（含 29 systemctl 部署），任一步失败即断链
# §59.244: 243 段删除——只有 29 是开发环境；生产环境（243/fnos/PT30/249 等）
# 一律 OTA 由用户自行更新（环境隔离铁律对齐——脚本不得触生产）
# 用法：bash .claude/skills/ptf-deploy/scripts/deploy-all.sh [--frontend-only|--backend-only]
#   约定：Agent 调用部署链一律用本脚本，禁止手工串联三脚本 + tail 截断输出。
set -euo pipefail
cd "$(dirname "$0")/../../../.."

MODE="${1:-all}"

run_step() {
  local name="$1"; shift
  echo "════════ $name ════════"
  "$@"
}

case "$MODE" in
  --frontend-only)
    run_step "前端构建" bash .claude/skills/ptf-deploy/scripts/build-frontend.sh
    ;;
  --backend-only)
    run_step "后端构建（含 29 部署）" bash .claude/skills/ptf-deploy/scripts/build-backend.sh
    ;;
  all|*)
    run_step "前端构建" bash .claude/skills/ptf-deploy/scripts/build-frontend.sh
    run_step "后端构建（含 29 部署）" bash .claude/skills/ptf-deploy/scripts/build-backend.sh
    ;;
esac

echo ""
echo "✅✅ 部署链全完成（$MODE）——此行出现即真成功"
