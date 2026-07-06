#!/bin/bash
# 批量更新 examples 目录下所有 git 项目代码
# 用法: ./scripts/update-examples.sh [目录]
#   默认目录为脚本上级目录的 examples
# 说明: 遍历指定目录下所有子目录，对 git 仓库执行 git pull --ff-only。
#       单个项目失败不影响其他项目，末尾汇总统计。退出码非 0 表示有失败。

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${1:-$SCRIPT_DIR/../examples}"

GREEN=$'\033[32m'
RED=$'\033[31m'
YELLOW=$'\033[33m'
CYAN=$'\033[36m'
RESET=$'\033[0m'

if [[ ! -d "$EXAMPLES_DIR" ]]; then
    echo "${RED}目录不存在: $EXAMPLES_DIR${RESET}" >&2
    exit 2
fi

total=0
updated=0
uptodate=0
failed=0
skipped=0
failed_list=()

printf "${CYAN}=== 批量 git pull: %s ===${RESET}\n\n" "$(cd "$EXAMPLES_DIR" && pwd)"

shopt -s nullglob
for dir in "$EXAMPLES_DIR"/*/; do
    name="$(basename "$dir")"
    total=$((total + 1))

    if [[ ! -d "$dir/.git" ]]; then
        printf "${YELLOW}[跳过]${RESET} %-30s 非git仓库\n" "$name"
        skipped=$((skipped + 1))
        continue
    fi

    output="$(cd "$dir" && git pull --ff-only 2>&1)"
    rc=$?

    if [[ $rc -eq 0 ]]; then
        if printf '%s' "$output" | grep -qiE "already up.to.date|已是最新"; then
            printf "${GREEN}[最新]${RESET} %s\n" "$name"
            uptodate=$((uptodate + 1))
        else
            printf "${GREEN}[更新]${RESET} %s\n" "$name"
            updated=$((updated + 1))
        fi
    else
        reason="$(printf '%s' "$output" | head -1 | tr -d '\n')"
        printf "${RED}[失败]${RESET} %-30s %s\n" "$name" "$reason"
        failed=$((failed + 1))
        failed_list+=("$name")
    fi
done
shopt -u nullglob

printf "\n${CYAN}=== 汇总 ===${RESET}\n"
printf "总计 %d | ${GREEN}已更新 %d${RESET} | ${GREEN}已是最新 %d${RESET} | ${YELLOW}跳过 %d${RESET} | ${RED}失败 %d${RESET}\n" \
    "$total" "$updated" "$uptodate" "$skipped" "$failed"
if [[ ${#failed_list[@]} -gt 0 ]]; then
    printf "${RED}失败列表: %s${RESET}\n" "${failed_list[*]}"
    exit 1
fi

exit 0
