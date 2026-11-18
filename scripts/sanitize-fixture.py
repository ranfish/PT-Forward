#!/usr/bin/env python3
"""§仓库安全-1 第七刀：测试夹具脱敏自动化管道。

用法：
  python3 scripts/sanitize-fixture.py <file.html> [file2.html ...]   消毒指定文件（就地修改）
  python3 scripts/sanitize-fixture.py --check <file> [file ...]      只检查不修改（exit 1=有未消毒形态）

职责：摸站/适配采集的真实站点 HTML 入库（internal/adapter/testdata/）前·
剥离「账号关联痕迹」——passkey/JWT 登录态/用户绑定 token/c_secure 会话。
幂等：含 SANITIZED 标记的已消毒值自动跳过（重跑零变化）。

规则来源=§仓库安全-4/10/11 实战沉淀（R-07 passkey·第三刀 JWT/绑定 token）。
接入：.githooks/pre-commit 对 testdata 变更自动调用——新夹具必经管道。
注意：站点级公开 token（CF beacon 等）非账号关联·不在此管道范围。
裸 hex32 不扫（info_hash 误伤）——真 API key 由 gitleaks CI 门兜底。
"""
import re
import sys
import glob

FAKE_JWT = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6MCwiZXhwIjoxMDAwMDAwMDAwfQ.SANITIZEDSIGNATURE0000000000000000000000000000000000000000000"

# 消毒规则库（顺序敏感：具体形态在前）
RULES = [
    # passkey=（下载链接凭证）
    (re.compile(r'passkey=[A-Fa-f0-9]{16,64}'), 'passkey=SANITIZED'),
    # NexusPHP 登录会话 cookie
    (re.compile(r'c_secure_(?:uid|pass)=[A-Za-z0-9+/=]{20,}'), 'SANITIZEDSESSION'),
    # JWT 登录态（NP 站 c_secure 的 JWT 演进形态）
    (re.compile(r'eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*'), FAKE_JWT),
    # 用户绑定登出 token（logout.php?token=/auth=/key=）
    (re.compile(r'logout\.php\?(?:token|auth|key)=[A-Za-z0-9_-]{16,}'),
     'logout.php?token=SANITIZEDLOGOUTTOKEN'),
    # 附件下载密钥
    (re.compile(r'dlkey=[A-Fa-f0-9]{16,}'), 'dlkey=SANITIZEDDLKEY'),
    # 页面 JS authKey（用户绑定功能密钥）
    (re.compile(r"((?:const|var)\s+authKey\s*=\s*')[A-Fa-f0-9]{24,}(')"), r'\1SANITIZEDAUTHKEY\2'),
]

MARKER = 'SANITIZED'


def sanitize_body(body: str):
    """返回 (消毒后文本, 替换计数)。含 SANITIZED 标记的命中段视为已消毒跳过。"""
    count = 0
    for pat, repl in RULES:
        def _sub(m):
            nonlocal count
            seg = m.group(0)
            if MARKER in seg:
                return seg  # 已消毒形态（幂等）
            count += 1
            return pat.sub(repl, seg) if not isinstance(repl, str) or '\\' not in repl else m.expand(repl)
        # 逐命中处理（避免已消毒段被再次替换破坏幂等）
        body = pat.sub(_sub, body)
    return body, count


def main():
    args = sys.argv[1:]
    check_only = False
    if args and args[0] == '--check':
        check_only = True
        args = args[1:]
    if not args:
        print(__doc__)
        return 2

    files = []
    for a in args:
        files.extend(sorted(glob.glob(a)) if any(c in a for c in '*?[') else [a])

    total, changed = 0, 0
    for p in files:
        try:
            body = open(p, errors='ignore').read()
        except OSError as e:
            print(f"✗ 读取失败 {p}: {e}", file=sys.stderr)
            return 2
        new, n = sanitize_body(body)
        total += n
        if n:
            changed += 1
            if check_only:
                print(f"  ⚠ {p}: {n} 处未消毒形态")
            else:
                open(p, 'w', encoding='utf-8').write(new)
                print(f"  ✓ {p}: 消毒 {n} 处")
    action = '检查' if check_only else '消毒'
    print(f"{action}完成：{changed}/{len(files)} 文件·共 {total} 处")
    if check_only and total:
        print("运行 python3 scripts/sanitize-fixture.py <files> 消毒后重新 add")
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
