---
name: ptf-infra-notes
description: 'PT-Forward 基础设施备忘：mpv 编译（源码/编译脚本/Docker 环境/ARM）、截图工具引擎（zimg/mobius/DoVi 限制）、数据采集策略（直连优先/代理回退/临时文件清理）、CookieCloud 工具用法、Playwright 采集注意事项（CJS/sameSite/NODE_PATH）。Use when: 重新编译 mpv、修改截图引擎参数、处理 DoVi 截图偏色、执行站点数据采集、使用 CookieCloud 提取 cookie、编写或调试 Playwright 采集脚本。'
---

# PT-Forward 基础设施备忘

## mpv 编译

- **源码**：`examples/mpv/`（git tag v0.40.0）
- **编译脚本**：`scripts/build-mpv-compile.sh`
- **Docker 编译环境**：`docker/Dockerfile.mpv-build`（Debian trixie）
- **产出**：`bin/amd64/mpv-new`（1.5MB，启用 zimg，禁用 GL/X11/音频/GPU）
- **重新编译**：`sg docker -c "docker run --rm -v /home/incast/PT-Forward:/work mpv-build bash /work/scripts/build-mpv-compile.sh"`
- **ARM 编译**：在任意 ARM Linux + Docker 上用相同脚本（meson 自动检测架构）

## 截图工具

- **引擎**：mpv `--vo=image` + zimg 色彩转换（`internal/publish/screenshot.go`）
- **HDR 视频**：自动加 `--vf=lavfi=[tonemap=mobius]`（高光控制）
- **DoVi**：偏绿偏紫（已知限制，zimg 不做 MMR reshaping，详见 §40.35）
- **字幕**：`--blend-subtitles=yes` + `fonts-noto-cjk`
- **独立脚本**：`scripts/screenshot.sh`（CLI 截图，匹配 `examples/screenshot/screenshot_v2.py` 策略）

## 数据采集策略

- **优先直连**：采集站点数据时，优先使用 `--noproxy '*'` 直连（如 curl 直连 IPv6）
- **直连失败再走代理**：仅当直连无法访问时，才尝试代理访问（`-x http://10.0.2.5:7897`）
- **采集后清理**：抓取完页面后立即清理 `/tmp/` 下的临时 HTML/JS 文件
- **WebFetch 失败**：必须立即告知用户，不能忽略或静默跳过

## CookieCloud 工具

- 路径：`/home/incast/PT-Forward/tools/cookiecloud/cookiecloud`
- 用法：`cookiecloud -domain <domain> -json`
- 需设置环境变量：`COOKIECLOUD_URL` / `COOKIECLOUD_UUID` / `COOKIECLOUD_PASSWORD`

## 临时凭证文件清理铁律（§59.472 附·三次违规后机制化）

**探针/诊断类任务收尾固定动作**（不靠记忆——固化命令）：

```bash
# 任务结束即执行（保留正在运行的后续任务依赖的文件）：
rm -f /tmp/opencode/*_cookie.txt /tmp/opencode/*-cookie.txt \
      /tmp/opencode/*_token.txt /tmp/opencode/*token*.txt /tmp/opencode/tok.txt \
      /tmp/opencode/*_pwd*.txt /tmp/opencode/*pwd*.txt \
      /tmp/opencode/*_enckey*.txt /tmp/opencode/*apikey*.txt /tmp/opencode/gh-token.txt \
      /tmp/opencode/*_ck.txt /tmp/opencode/*.db
```

- **例外**：正在运行的后续任务依赖的文件（如后台验证在用的 cookie）——任务真正收尾时随最终报告一起清
- **生产 DB 副本**（docker cp 出来的 .db）：用完即删（可达 GB 级·含全部站点密文凭证+加密 key 同库）
- **脚本内嵌凭证**：`grep -lE "c_secure|passkey=|pass=|密码" *.js *.sh` 扫残留——命中即删
- 历史教训：/tmp/opencode 曾累积 40+ 站 cookie、5 个明文密码、加密 key、GitHub token、2.4GB 生产 DB 副本（几十轮会话从未清理——2026-11-06 大扫除销账）

## Playwright 采集注意事项

- 脚本必须用 CJS 格式（`require` + `async function main()`），不能用 `.mjs` 的 `import` + top-level await
- CookieCloud 工具域名匹配要求**精确匹配**（如 `pterclub.net` 而非 `.pterclub.net`）
- sameSite 必须映射为 `Strict`/`Lax`/`None`，`unspecified` 要转为 `Lax`
- `page.evaluate()` 中的变量必须用闭包或参数传递，不能直接引用外层 `const` 变量
- `innerText` 抓取可能遗漏 JS 动态渲染的 `<select>` 选项，需要用 DOM query 做 deep inspect
- NODE_PATH：`NODE_PATH=/home/incast/.npm-global/lib/node_modules`

## Playwright 容器化运行（§59.499 工具化·用户指令）

**统一入口**：`bash .claude/skills/ptf-infra-notes/scripts/pw-run.sh <脚本.js>`——封装镜像/浏览器路径/挂载/网络，免每次重查经验。

固化经验（脚本内已实现）：
- 镜像 `mcr.microsoft.com/playwright:v1.53.0-noble`（自带 node+chromium·与 playwright-core 版本配套）
- **29 宿主直跑 chromium 缺 libnspr4 等系统库——必须走容器**（§59.499 实证）
- 浏览器路径容器内探测 `/ms-playwright/chromium-*/chrome-linux/chrome`（镜像升级自动适配）
- playwright-core npm 包在 `/tmp/opencode/node_modules`（缺失时脚本提示安装命令）
- 脚本所在目录挂载 `/pwtest`（截图产物直接落宿主机）；`--network host` 访问内网目标
- 脚本约定：`chromium.launch({ executablePath: process.env.CHROME_PATH })`；PT-Forward 登录注入 `localStorage.setItem('pt-forward-access-token', token)`

**UI 验证场景**（无缓存环境渲染验证·§59.499 破案法）：Playwright 全新上下文加载页面→操作到目标组件→DOM dump（`document.querySelector` 取 innerHTML/子元素计数）——比截图更硬（模型不读图时 DOM 层可判）。
