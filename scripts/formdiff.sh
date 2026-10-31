#!/bin/bash
# PT-Forward 同栈表单 Diff（§59.389 发版前门禁）
#
# 用开发机(默认 29)已部署的服务跑与生产前端完全同栈的表单 Diff——
# 发版前验证"用户 OTA 后在前端 Diff 会看到什么"，0 条才打 tag。
#
# 用法:
#   bash scripts/formdiff.sh <站名> <发布页HTML文件> [host]
#   例: bash scripts/formdiff.sh 聆音 /tmp/opencode/sv_upload.html
#
# 退出码: 0=Diff 0条(干净)  1=有差异(见输出)  2=参数/登录/API失败
#
# 定性判别（两路交叉——docs/31 §59.389）:
#   本脚本报差异 + 适配期 python 裸对说一致  → Diff 工具问题(解析器假阳性)
#   本脚本报差异 + python 裸对也报           → 适配问题(配置错)或站方改版(重抓)
#
# 环境变量: PTF_HOST(默认 localhost) PTF_USER/PTF_PASS(默认 admin/Xsy2026!)

set -euo pipefail

SITE="${1:?用法: formdiff.sh <站名> <发布页HTML文件> [host]}"
HTML_FILE="${2:?缺少 HTML 文件参数}"
HOST="${3:-${PTF_HOST:-localhost}}"
PORT=8765
USER="${PTF_USER:-admin}"
PASS="${PTF_PASS:-Xsy2026!}"

[ -f "$HTML_FILE" ] || { echo "❌ HTML 文件不存在: $HTML_FILE"; exit 2; }

TOKEN=$(curl -s -m 15 -X POST "http://$HOST:$PORT/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" 2>/dev/null | \
  python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["accessToken"])' 2>/dev/null) || true
[ -n "${TOKEN:-}" ] || { echo "❌ 登录失败"; exit 2; }

RESULT=$(HTML_FILE="$HTML_FILE" SITE="$SITE" TOKEN="$TOKEN" HOST="$HOST" PORT="$PORT" python3 <<'PYEOF'
import json, os, sys, urllib.request

html = open(os.environ['HTML_FILE'], encoding='utf-8', errors='replace').read()
body = json.dumps({'site_name': os.environ['SITE'], 'html': html}).encode()
req = urllib.request.Request(
    f"http://{os.environ['HOST']}:{os.environ['PORT']}/api/v1/publish/form-config/parse",
    data=body, headers={'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + os.environ['TOKEN']})
try:
    resp = json.load(urllib.request.urlopen(req, timeout=60))
except Exception as e:
    print(json.dumps({'error': str(e)})); sys.exit(0)
diffs = (resp.get('data') or {}).get('diffs') or []
print(json.dumps({'diffs': diffs, 'count': len(diffs)}, ensure_ascii=False))
PYEOF
)

COUNT=$(echo "$RESULT" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("count",-1))' 2>/dev/null) || COUNT=-1
if [ "$COUNT" = "-1" ]; then
  echo "❌ API 失败: $RESULT"
  exit 2
fi

echo "════════════════════════════════════════"
echo "  同栈表单 Diff: $SITE vs $(basename "$HTML_FILE")"
echo "  目标: http://$HOST:$PORT (与生产前端同栈)"
echo "════════════════════════════════════════"
if [ "$COUNT" = "0" ]; then
  echo "  ✅ Diff 0 条（干净——可发版）"
  exit 0
fi
echo "  ⚠️ $COUNT 条差异:"
echo "$RESULT" | python3 -c '
import sys, json
for d in json.load(sys.stdin)["diffs"]:
    k = d.get("kind", "?")
    dom = d.get("domain", "?")
    lb = d.get("label", "?")
    cur = d.get("current_value", "")
    draft = d.get("draft_value", "")
    suffix = f" (cur={cur} draft={draft})" if str(cur) or str(draft) else ""
    print(f"  [{k}] {dom}: {lb}{suffix}")
'
echo "  ── 定性: 与适配期 python 裸对交叉（一致→工具问题/也报→适配或改版）"
exit 1
