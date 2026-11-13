#!/bin/bash
# 憨憨 JS 残留清洗脚本（§59.516 v0.0.1462/1463）
#
# 用法：bash scripts/clean-hhanclub-js.sh <IP> [用户名] [认证方式]
#   认证方式：key（默认·root+私钥 ~/.ssh/xsy）| pass（ranfish+密码）
#   密码经环境变量 PTF_PASS 传入（禁止硬编码入库）：
#     export PTF_PASS='...'   # SSH 密码 + API admin 密码（同源）
# 示例：
#   bash scripts/clean-hhanclub-js.sh 10.0.0.99                    # 私钥组
#   PTF_PASS='...' bash scripts/clean-hhanclub-js.sh 10.0.0.12 ranfish pass   # 密码组
#
# 清洗语义（幂等·可重复执行）：
#   对 source_description 含 JS 标记（jQuery/function/viewpeerlist）的行清空
#   source_description 发布链零消费·清空安全（§59.516 消费方审计）
#   干净内容（PTGen 形态）不受影响
#
# 策略：SQLite 直写优先（秒级）→ API 兜底（无 DB 文件权限时）
#   SQLite 一条 UPDATE 原子完成·WAL 模式允许与应用并发
#   API 路线需目标环境 ≥ v0.0.1463（批量 UPDATE 修复版）

set -euo pipefail

IP="${1:?用法: $0 <IP> [用户名] [认证方式]}"
USER="${2:-root}"
AUTH="${3:-key}"

echo "◆ $IP (user=$USER auth=$AUTH)"

# ── SSH 执行封装 ──
run_remote() {
  local script="$1"
  if [ "$AUTH" = "key" ]; then
    ssh -i ~/.ssh/xsy -o ConnectTimeout=15 -o StrictHostKeyChecking=no "$USER@$IP" "bash -s" < "$script"
  else
    # 密码组：sshpass（需 PTF_PASS 环境变量）
    if [ -z "${PTF_PASS:-}" ]; then
      echo "  ❌ 密码组需要环境变量 PTF_PASS（export PTF_PASS='...'）"
      exit 1
    fi
    if ! command -v sshpass &>/dev/null; then
      echo "  ❌ 需要 sshpass（sudo apt install sshpass）"
      exit 1
    fi
    sshpass -p "$PTF_PASS" ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=no "$USER@$IP" "bash -s" < "$script"
  fi
}

# ── 远端脚本（API 密码同样经 PTF_PASS 注入·不留明文）──
REMOTE_SCRIPT="$(mktemp /tmp/hh_clean_XXXX.sh)"
cat > "$REMOTE_SCRIPT" <<REMOTE_EOF
#!/bin/bash
set -euo pipefail

SQL_CLEAN="UPDATE torrent_metadata SET source_description = ''
WHERE source_description != ''
AND (source_description LIKE '%jQuery%' OR source_description LIKE '%function %' OR source_description LIKE '%viewpeerlist%');"

SQL_COUNT="SELECT COUNT(*) FROM torrent_metadata
WHERE source_description != ''
AND (source_description LIKE '%jQuery%' OR source_description LIKE '%function %' OR source_description LIKE '%viewpeerlist%');"

# ── 路径 1：SQLite 直写（root 权限可访问 DB 文件时·秒级）──
DB=""
for p in \\
  /home/docker/PT-Forward/data/pt-forward.db \\
  /opt/PT-Forward/data/pt-forward.db \\
  /data/PT-Forward/data/pt-forward.db \\
  \$(find /home /opt /data -maxdepth 5 -name "pt-forward.db" -path "*/PT-Forward/*" 2>/dev/null | head -3); do
  if [ -f "\$p" ] && [ -w "\$p" ]; then
    DB="\$p"
    break
  fi
done

if [ -n "\$DB" ]; then
  echo "  路径: SQLite 直写 (\$DB)"
  N=\$(sqlite3 "\$DB" "\$SQL_COUNT" 2>/dev/null || echo -1)
  if [ "\$N" = "-1" ]; then
    echo "  ⚠️ SQLite 读失败·转 API"
  elif [ "\$N" = "0" ]; then
    echo "  ✅ 零残留（已干净）"
    exit 0
  else
    sqlite3 "\$DB" "\$SQL_CLEAN"
    echo "  ✅ 清洗完成：\$N 行"
    exit 0
  fi
fi

# ── 路径 2：API（无 DB 文件权限时·需 ≥v0.0.1463）──
echo "  路径: API"

API_PW='${PTF_PASS:-}'
if [ -z "\$API_PW" ]; then
  echo "  ❌ API 兜底需要环境变量 PTF_PASS（admin 密码）"
  exit 1
fi

PAYLOAD=\$(printf '{\\"username\\":\\"admin\\",\\"password\\":\\"%s\\"}' "\$API_PW")
TOKEN=\$(curl -s --max-time 15 -X POST http://127.0.0.1:8765/api/v1/auth/login \\
  -H "Content-Type: application/json" \\
  -d "\$PAYLOAD" \\
  | python3 -c "import json,sys;print(json.load(sys.stdin)['data']['accessToken'])" 2>/dev/null)

if [ -z "\$TOKEN" ]; then
  echo "  ❌ API 登录失败"
  exit 1
fi

VER=\$(curl -s --max-time 10 http://127.0.0.1:8765/api/v1/system/info \\
  -H "Authorization: Bearer \$TOKEN" \\
  | python3 -c "import json,sys;print(json.load(sys.stdin)['data']['version'])" 2>/dev/null)

if [ "\$VER" \\< "v0.0.1463" ]; then
  echo "  ❌ 版本 \$VER < v0.0.1463（需 OTA 后重跑）"
  exit 1
fi
echo "  版本: \$VER"

# 触发（大库可能慢·给 300s）
RESP=\$(curl -s --max-time 300 -X POST \\
  http://127.0.0.1:8765/api/v1/publish/torrents/publish/seeds/fix-source-description \\
  -H "Authorization: Bearer \$TOKEN")

TOTAL=\$(echo "\$RESP" | python3 -c "import json,sys;d=json.load(sys.stdin).get('data',{});print(d.get('total',-1))" 2>/dev/null || echo -1)
CHANGED=\$(echo "\$RESP" | python3 -c "import json,sys;d=json.load(sys.stdin).get('data',{});print(d.get('changed',-1))" 2>/dev/null || echo -1)

if [ "\$TOTAL" = "-1" ]; then
  echo "  ⚠️ API 超时或响应异常（大库 LIKE 扫描慢·后台可能已完成）"
  echo "  建议：等 5 分钟后重跑（幂等·重复执行安全）"
else
  echo "  ✅ 清洗完成：total=\$TOTAL changed=\$CHANGED"
fi
REMOTE_EOF

run_remote "$REMOTE_SCRIPT"
RC=$?
rm -f "$REMOTE_SCRIPT"
exit $RC
