#!/bin/bash
# 憨憨 JS 残留清洗脚本（§59.516 v0.0.1462/1463·2026-11-14 修正版）
#
# 用法：bash scripts/clean-hhanclub-js.sh <IP或域名> [用户名] [认证方式]
#   认证方式：key（默认·root+私钥 ~/.ssh/xsy）| pass（ranfish+密码）
#   密码经环境变量 PTF_PASS 传入（禁止硬编码入库）：
#     export PTF_PASS='...'   # SSH 密码 + sudo 密码 + API admin 密码（同源）
# 示例：
#   PTF_PASS='...' bash scripts/clean-hhanclub-js.sh pt31.ranfish.uk              # 私钥组
#   PTF_PASS='...' bash scripts/clean-hhanclub-js.sh pt29.ranfish.uk ranfish pass # 密码组
#   PTF_PASS='...' bash scripts/clean-hhanclub-js.sh pt4.ranfish.uk incast key    # 私钥组+API兜底
#
# 清洗语义（幂等·可重复执行）：
#   对 source_description 含 JS 标记（jQuery/function/viewpeerlist）的行清空
#   source_description 发布链零消费·清空安全（§59.516 消费方审计）
#   干净内容（PTGen 形态）不受影响
#
# 策略（2026-11-14 修正·8 环境实测适配）：
#   ① SQLite 直写（root 可写·秒级）
#   ② sudo -S 直写（ranfish@fnOS 组：DB 属主 root·ranfish sudo 可用）
#   ③ API 兜底（找不到 DB / sudo 不通·需目标环境 ≥ v0.0.1463）
#   修正点：API 端点路径（原多 /publish/torrents 段必 404）；DB 搜索加 /vol*
#   （fnOS 挂载）；PTF_PASS 经 env 注入远端（原 API 兜底断链）
#
# 2026-11-14 实测分组：
#   key  组：pt31/pt35（root+xsy）·pt4（incast+xsy）
#   pass 组：pt29/pt30/pt32/pt33/fnos（ranfish+密码）

set -euo pipefail

IP="${1:?用法: $0 <IP> [用户名] [认证方式]}"
USER="${2:-root}"
AUTH="${3:-key}"

echo "◆ $IP (user=$USER auth=$AUTH)"

# ── SSH 执行封装（PTF_PASS 经 env 注入远端·不落远端磁盘）──
run_remote() {
  local script="$1"
  local envprefix=""
  if [ -n "${PTF_PASS:-}" ]; then
    envprefix="env PTF_PASS='$PTF_PASS'"
  fi
  if [ "$AUTH" = "key" ]; then
    # envprefix 为空时 " bash -s" 前导空格无害；非空时 env 与 bash 必须同命令（分离则 PTF_PASS 丢失）
    ssh -i ~/.ssh/xsy -o ConnectTimeout=15 -o StrictHostKeyChecking=no -o BatchMode=yes \
      "$USER@$IP" "${envprefix} bash -s" < "$script" 2>&1 | grep -v "password for"
  else
    if [ -z "${PTF_PASS:-}" ]; then
      echo "  ❌ 密码组需要环境变量 PTF_PASS（export PTF_PASS='...'）"
      exit 1
    fi
    if ! command -v sshpass &>/dev/null; then
      echo "  ❌ 需要 sshpass（sudo apt install sshpass）"
      exit 1
    fi
    sshpass -p "$PTF_PASS" ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=no \
      -o PreferredAuthentications=password -o PubkeyAuthentication=no \
      "$USER@$IP" "$envprefix bash -s" < "$script" 2>&1 | grep -v "password for"
  fi
}

# ── 远端脚本（$ 不在本地展开·远端 bash 展开）──
REMOTE_SCRIPT="$(mktemp /tmp/hh_clean_XXXX.sh)"
cat > "$REMOTE_SCRIPT" <<'REMOTE_EOF'
set -uo pipefail

SQL_CLEAN="UPDATE torrent_metadata SET source_description = ''
WHERE source_description != ''
AND (source_description LIKE '%jQuery%' OR source_description LIKE '%function %' OR source_description LIKE '%viewpeerlist%');"

SQL_COUNT="SELECT COUNT(*) FROM torrent_metadata
WHERE source_description != ''
AND (source_description LIKE '%jQuery%' OR source_description LIKE '%function %' OR source_description LIKE '%viewpeerlist%');"

# ── 定位 DB（普通 find·fnOS 的 /vol* 目录普通用户可遍历）──
DB=""
for p in \
  /home/docker/PT-Forward/data/pt-forward.db \
  /opt/PT-Forward/data/pt-forward.db \
  /data/PT-Forward/data/pt-forward.db \
  /vol1/1000/docker/PT-Forward/data/pt-forward.db \
  $(find /home /opt /data /vol1 /vol2 /vol3 /vol4 /vol5 /vol6 /vol7 /vol8 /vol9 /vol10 /vol11 -maxdepth 5 -name "pt-forward.db" -path "*PT-Forward*" 2>/dev/null | head -3); do
  if [ -f "$p" ]; then
    DB="$p"
    break
  fi
done

# ── sqlite 执行封装：直写 → sudo -S 兜底 ──
sq() {
  if [ -w "$DB" ]; then
    sqlite3 "$DB" "$1"
  elif [ -n "${PTF_PASS:-}" ]; then
    printf '%s\n' "$PTF_PASS" | sudo -S -p '' sqlite3 "$DB" "$1" 2>/dev/null
  else
    sudo -n sqlite3 "$DB" "$1" 2>/dev/null
  fi
}

if [ -n "$DB" ]; then
  N=$(sq "$SQL_COUNT")
  if [ -z "$N" ]; then
    echo "  ⚠️ SQLite 不可访问（无权限/sudo 不通）·转 API"
  elif [ "$N" = "0" ]; then
    echo "  ✅ 零残留（$DB 已干净）"
    exit 0
  else
    if sq "$SQL_CLEAN"; then
      echo "  ✅ 清洗完成：$N 行（$DB）"
      exit 0
    else
      echo "  ⚠️ SQLite 写失败·转 API"
    fi
  fi
else
  echo "  ⚠️ 未找到 DB 文件·转 API"
fi

# ── 路径 3：API 兜底（需 ≥v0.0.1463）──
echo "  路径: API"

API_PW="${PTF_PASS:-}"
if [ -z "$API_PW" ]; then
  echo "  ❌ API 兜底需要环境变量 PTF_PASS（admin 密码）"
  exit 1
fi

PAYLOAD=$(printf '{"username":"admin","password":"%s"}' "$API_PW")
TOKEN=$(curl -s --max-time 15 -X POST http://127.0.0.1:8765/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" \
  | python3 -c "import json,sys;print(json.load(sys.stdin)['data']['accessToken'])" 2>/dev/null)

if [ -z "$TOKEN" ]; then
  echo "  ❌ API 登录失败"
  exit 1
fi

VER=$(curl -s --max-time 10 http://127.0.0.1:8765/api/v1/system/info \
  -H "Authorization: Bearer $TOKEN" \
  | python3 -c "import json,sys;print(json.load(sys.stdin)['data']['version'])" 2>/dev/null)
echo "  版本: $VER"

# 触发（大库可能慢·给 300s；2026-11-14 修正端点：真实路由无 /publish/torrents 段）
RESP=$(curl -s --max-time 300 -X POST \
  http://127.0.0.1:8765/api/v1/publish/seeds/fix-source-description \
  -H "Authorization: Bearer $TOKEN")

TOTAL=$(echo "$RESP" | python3 -c "import json,sys;d=json.load(sys.stdin).get('data',{});print(d.get('total',-1))" 2>/dev/null || echo -1)
CHANGED=$(echo "$RESP" | python3 -c "import json,sys;d=json.load(sys.stdin).get('data',{});print(d.get('changed',-1))" 2>/dev/null || echo -1)

if [ "$TOTAL" = "-1" ]; then
  echo "  ⚠️ API 超时或响应异常（大库 LIKE 扫描慢·后台可能已完成）"
  echo "  建议：等 5 分钟后重跑（幂等·重复执行安全）"
else
  echo "  ✅ 清洗完成：total=$TOTAL changed=$CHANGED"
fi
REMOTE_EOF

run_remote "$REMOTE_SCRIPT"
RC=${PIPESTATUS[0]}
rm -f "$REMOTE_SCRIPT"
exit $RC
