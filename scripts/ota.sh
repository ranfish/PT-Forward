#!/bin/bash
# PT-Forward 批量/指定环境 OTA 脚本
#
# 用法:
#   bash scripts/ota.sh                    # 全部 9 个环境
#   bash scripts/ota.sh 10.0.0.28          # 指定单个环境
#   bash scripts/ota.sh 10.0.0.28 10.0.0.12  # 指定多个环境
#   bash scripts/ota.sh -v v0.0.1120       # 指定版本（默认最新）
#   bash scripts/ota.sh -c                 # 仅检查版本不执行 OTA
#
# 环境变量:
#   PTF_PASS   默认 Xsy2026!
#   PTF_USER   默认 admin

set -euo pipefail

ALL_HOSTS="10.0.0.243 10.0.0.12 10.0.0.99 10.0.0.249 10.0.0.22 10.0.0.65 10.0.0.245 10.0.0.28 10.0.0.242"
USER="${PTF_USER:-admin}"
PASS="${PTF_PASS:-Xsy2026!}"
PORT=8765
TARGET_VER=""
CHECK_ONLY=false
HOSTS=()

while [[ $# -gt 0 ]]; do
  case $1 in
    -v|--version) TARGET_VER="$2"; shift 2 ;;
    -c|--check)   CHECK_ONLY=true; shift ;;
    -h|--help)
      head -12 "$0" | tail -10; exit 0 ;;
    *)            HOSTS+=("$1"); shift ;;
  esac
done

[[ ${#HOSTS[@]} -eq 0 ]] && HOSTS=($ALL_HOSTS)

login() {
  curl -s -m 10 -X POST "http://$1:$PORT/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" 2>/dev/null | \
    python3 -c "import sys,json;print(json.load(sys.stdin)['data']['accessToken'])" 2>/dev/null
}

get_version() {
  curl -s -m 10 -H "Authorization: Bearer $2" \
    "http://$1:$PORT/api/v1/system/check-update" 2>/dev/null | \
    python3 -c "
import sys,json
d=json.load(sys.stdin)['data']
print(d['current_version'],d['latest_version'])" 2>/dev/null
}

trigger_ota() {
  curl -s -m 120 -X POST -H "Authorization: Bearer $2" \
    "http://$1:$PORT/api/v1/system/update" 2>/dev/null | \
    python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print(d.get('status','?'))" 2>/dev/null
}

echo "════════════════════════════════════════════"
[[ $CHECK_ONLY == true ]] && echo "  版本检查（不执行 OTA）" || echo "  PT-Forward OTA"
echo "  目标: ${HOSTS[*]}"
echo "════════════════════════════════════════════"

NEED_OTA=()
for HOST in "${HOSTS[@]}"; do
  printf "%-16s" "$HOST"
  TOKEN=$(login "$HOST")
  if [ -z "$TOKEN" ]; then
    echo "❌ 登录失败"
    continue
  fi
  read -r CURRENT LATEST <<< "$(get_version "$HOST" "$TOKEN")"
  if [ -z "$CURRENT" ]; then
    echo "❌ 版本查询失败"
    continue
  fi

  if [ -n "$TARGET_VER" ]; then
    LATEST="$TARGET_VER"
  fi

  if [ "$CURRENT" = "$LATEST" ] && [ -z "$TARGET_VER" ]; then
    echo "✅ 已是 $CURRENT"
    continue
  fi

  if [[ $CHECK_ONLY == true ]]; then
    echo "当前 $CURRENT → 最新 $LATEST"
  else
    RESULT=$(trigger_ota "$HOST" "$TOKEN")
    echo "$CURRENT → $LATEST ($RESULT)"
    NEED_OTA+=("$HOST")
  fi
done

if [[ $CHECK_ONLY == true ]] || [[ ${#NEED_OTA[@]} -eq 0 ]]; then
  echo "════════════════════════════════════════════"
  exit 0
fi

echo ""
echo "等待 60s 后验证..."
echo "════════════════════════════════════════════"
sleep 60

FAIL=0
for HOST in "${NEED_OTA[@]:-${HOSTS[@]}}"; do
  printf "%-16s" "$HOST"
  TOKEN=$(login "$HOST")
  if [ -z "$TOKEN" ]; then
    echo "❌ 不可达"
    FAIL=$((FAIL+1))
    continue
  fi
  read -r CURRENT LATEST <<< "$(get_version "$HOST" "$TOKEN")"
  EXPECTED="${TARGET_VER:-$LATEST}"
  if [ "$CURRENT" = "$EXPECTED" ]; then
    echo "✅ $CURRENT"
  else
    echo "⚠️  $CURRENT（期望 $EXPECTED）"
    FAIL=$((FAIL+1))
  fi
done

echo "════════════════════════════════════════════"
[ $FAIL -gt 0 ] && echo "⚠️  $FAIL 个环境待确认" || echo "✅ 全部成功"
