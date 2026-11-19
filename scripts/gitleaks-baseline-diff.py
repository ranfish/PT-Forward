#!/usr/bin/env python3
"""§仓库安全-1 第三刀·自建 baseline 比对层

背景：gitleaks 内置 --baseline-path 对 sanitize 版 baseline（剔 Secret/Match 字段
防二次泄漏）比对不生效——fingerprint 全在 baseline 内仍判新泄漏。
解法：gitleaks 只负责扫描（--exit-code 0 恒过）·本脚本做指纹集合 diff。
输出只含 File/RuleID（不含 Secret）——防 CI 日志（开源后公开）成为泄漏面。

用法：gitleaks-baseline-diff.py <scan_report.json> <baseline.json>
退出码：0=无新泄漏 · 1=有 baseline 外新泄漏
联动：第八刀历史重写后删 baseline 重新生成为空集。
"""
import json
import sys
from collections import Counter


def main():
    if len(sys.argv) != 3:
        print(f"用法: {sys.argv[0]} <scan_report.json> <baseline.json>", file=sys.stderr)
        return 2
    scan_path, baseline_path = sys.argv[1], sys.argv[2]

    with open(scan_path) as f:
        findings = json.load(f)

    try:
        with open(baseline_path) as f:
            baseline = {x.get("Fingerprint") for x in json.load(f).get("findings", [])}
    except (FileNotFoundError, json.JSONDecodeError):
        baseline = set()

    extra = [r for r in findings if r.get("Fingerprint") not in baseline]
    if extra:
        print(f"✗ 检出 baseline 外新泄漏 {len(extra)} 处（扫描 {len(findings)}·基线 {len(baseline)}）:")
        for (file, rule), n in Counter(
            (r.get("File", "?"), r.get("RuleID", "?")) for r in extra
        ).most_common(25):
            print(f"  {file} [{rule}] ×{n}")
        # 定位明细：File:Line + RuleID（不含任何 Secret 片段——§仓库安全-25 F-1 整改）
        for r in extra[:10]:
            print(f"    · {r.get('File','?')}:{r.get('StartLine','?')} [{r.get('RuleID','?')}]")
        print("处置：清除泄漏内容·或 .gitleaks.toml allowlist 值级豁免（commit 说明理由）")
        return 1
    print(f"✓ 扫描 {len(findings)} findings 全部在基线内（基线指纹 {len(baseline)}）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
