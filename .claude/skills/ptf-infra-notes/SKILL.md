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

## Playwright 采集注意事项

- 脚本必须用 CJS 格式（`require` + `async function main()`），不能用 `.mjs` 的 `import` + top-level await
- CookieCloud 工具域名匹配要求**精确匹配**（如 `pterclub.net` 而非 `.pterclub.net`）
- sameSite 必须映射为 `Strict`/`Lax`/`None`，`unspecified` 要转为 `Lax`
- `page.evaluate()` 中的变量必须用闭包或参数传递，不能直接引用外层 `const` 变量
- `innerText` 抓取可能遗漏 JS 动态渲染的 `<select>` 选项，需要用 DOM query 做 deep inspect
- NODE_PATH：`NODE_PATH=/home/incast/.npm-global/lib/node_modules`
