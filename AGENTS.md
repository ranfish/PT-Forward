# PT-Forward 项目 Agent 指令

## 当前任务焦点（新会话必读）

**版本**：v0.0.283（已发布）+ §56.34 步骤 1-4（未打 tag）。

**当前主线**：§56.33 手动转发字段采集链路重构已完成并验证（14/14 副标题获取成功）。§56.34 种子技术特征模型**步骤 1-4 已完成并部署验证**（TechProfile 体系已接入运行时），步骤 5（旧体系废弃）待做。

### ✅ 已完成（v0.0.228 → v0.0.283）

- ✅ §56 转载功能核心提取层（v0.0.228~v0.0.249）：Engine/extract 包/8 站 stub/配置移植/titleparser 补全/adapter Engine 接入/auto_feed 正则/繁体中文/tracker 匹配器
- ✅ **tracker→站点匹配 + 标题重组断裂点 1+2**（v0.0.266~v0.0.276）：109 站 tracker_domains + title_format 同步 + Reassemble 接入发布管线 + 批量标题预览
- ✅ **￡ 制作组分隔符修复**（v0.0.277）：ExtractGroupName 支持 `￡`（SSD/CMCT 特有格式）
- ✅ **MediaInfo JSON→纯文本**（v0.0.278）：去 `--Output=JSON`，删结构化字段死代码
- ✅ **§56.33 手动转发字段采集链路重构**（v0.0.279~v0.0.283）：
  - C1：接入 SourceSiteDetector.Detect + coverage，小组名（CMCTV）→ release_group_mappings → 源站精确识别
  - D1：tid 四级链（前端 > coverage > torrent_metadata.TorrentID > FetchAndStoreBySearch L2 反查）
  - A1：cachedMeta 退化为"本地产物缓存"，命中只跳过 AnalyzeLocalArtifacts
  - B1：runAnalyze 改用 metadata.Merge(DetailFirst)，替代散乱填充
  - P1：PTGen 始终跑，失败回退 cachedMeta 历史
  - bug 修复：size=0 导致 L2 全过滤（v0.0.281）+ store GORM Assign 同变量 bug（v0.0.283）
  - 端到端验证：14/14 /PT2/SSD 种子副标题全部 src=detail ✅

### ✅ §56.34 种子技术特征模型（步骤 1-4 已完成）

**问题本质**：种子的技术特征（分辨率/编码/HDR/音频/声道等）在三套系统里割裂描述（titleparser/Engine/MediaTagInferer），没有单一事实来源。

**字段集**：18 字段，以青蛙站 v1.05 视频标题命名规范为权威定义 + chinese_prefix。

**五个决策**：
1. audio 独立三字段（codec + channels + technology），不合并
2. container 剔除 + VideoFormat 去掉 + 3D 三层（source_type + specification + media_tag）
3. standard_keys 不扩展（现有 11 category 足够）
4. MediaInfo 为准（转发有本地文件 = MediaInfo 绝对准确，源标题人工填写可能错漏）
5. 新建并行（策略 B，旧体系逐步废弃）

**实施路径**（5 步，步骤 1-4 已完成）：
1. ✅ 新建 TechProfile 模型 + MediaInfo 提取器 + 合并器（纯新增）
2. ✅ ParseTitleTech 增强提取（补声道/Atmos/edition_info/Medium 拆分）
3. ✅ Reassemble/Standardize 改造（从 TechProfile 按 v1.05 重组 + 映射）
4. ✅ 接入点切换（runAnalyze/buildPublishRequest/preview 共 6 个使用点）
5. ⬜ 旧体系废弃（CorrectWithMediaInfo/Standardize/Reassemble/extractTitleComponents 已无调用方，可删除）

**新增文件**（6 个，纯新增）：tech_profile.go / mediainfo_extractor.go / profile_merge.go / title_tech_parser.go / reassemble_tech.go / standardize_tech.go

### 端到端验收状态

| 端点 | 状态 |
|------|------|
| /seeded-torrents（含 source_site）| ✅ |
| /analyze（§56.33 重构 + §56.34 TechProfile）| ✅ 14/14 副标题 + tech_profile |
| /merge（三源合并 + 标准化 code）| ✅ |
| /preview（字段预览 + 完整度检查）| ✅ |
| /eligible-targets（92 站）| ✅ |
| /submit（真发布 21 步管线）| ❌ **未测** |

**关键事实**：
- 生产 249 Docker 部署（docker compose），端口 8765
- Docker mirror `docker.fnnas.com` 可能缓存旧镜像，部署需确认镜像 SHA 已更新
- tracker 匹配器自动识别来源站 + §56.33 SourceSiteDetector 小组名识别（CMCTV→SSD）
- 按 §56.33 决策 4：MediaInfo 为准（本地文件绝对准确），标题为 fallback
- 按 §56 决策 10：简介不复用源站（完全重建）
- PTer 实际完整度 100%（92% 是因为 bdinfo 不适用 WEB-DL）

**待办**：
- §56.34 步骤 5：旧体系废弃（CorrectWithMediaInfo/Standardize/Reassemble/extractTitleComponents 删除）+ ParseTitle 已有问题修复（H265 token 残留 + .mp4 后缀在 release_group）
- 打 tag 发版 §56.34（步骤 1-4 已部署验证）
- 测 /submit 真发布（高优先级）
- HHanClub 适配（中优先级，Tailwind CSS）
- §56 前端组件（tag/预览/截图 toggle 等，各需 vite build）
- adapter 统一后续（Unit3D 评估 + 删死代码）
- 11 站 RSS 配置后批量验证

**完整设计文档**：`docs/31-模块设计决策记录.md` §55-§56.34（64000+ 行，按需读特定章节，不要一次读全文）。

## 环境信息

- **Go**：`/home/incast/.local/go/bin/go`（v1.25，系统 PATH 中无 go，必须用全路径）
- **Node**：`/home/incast/.local/bin/node`（v22，系统 Node 18 不兼容，必须用此路径）
- **CGO**：后端编译必须 `CGO_ENABLED=1`
- **DB**：`data/pt-forward.db`（SQLite）
- **服务**：`systemctl --user restart pt-forward`（用户级 systemd 服务，端口 8765）
- **代理**：`http://10.0.2.5:7897`（curl/docker pull 等访问外部网络时可用）
- **Docker**：`sg docker -c "docker ..."`（当前用户不在 docker 组，需 sg 切换组）

## 通用规则

- **语言**：与用户沟通用中文
- **设计决策**：所有设计决策记录到 `docs/31-模块设计决策记录.md`
- **敏感信息**：禁止在本机或云端保存 cookie、passkey、apikey、token 等敏感信息
- **适配器文档**：放在 `docs/32-站点适配器设计/` 目录下
- **站点数据**：官组命名、规则等站点原始数据必须原样写入，不能杜撰
- **Git 提交**：禁止提交 `data/`、`PT0/`～`PT8/`、`*.torrent`、`logs/`、`*.db`（详见 `.gitignore`）
- **删除代码后**：必须跑 `go vet ./internal/... ./cmd/pt-forward/...`（**不要**用 `go vet ./...`，`cmd/verify-pieces-hash` 有已知冲突会报错），确保测试文件不引用已删符号
- **编译部署前**：必须灵魂四问、回归审核，然后再进行编译和部署
- **版本号**：编译命令必须包含 `-X main.version=$(git describe --tags --always --dirty)`，源码默认值为 `"dev"`
- **⚠️ 部署铁律（最高优先级，违反=事故）**：
  1. **先提交后部署**：`git commit + push` 必须在编译/部署**之前**完成。代码必须在 git 历史中先于部署生效。**禁止先部署后提交**。
  2. **部署检查清单**（每次部署前逐条过一遍，不能跳过）：
     - [ ] 灵魂四问全部通过？
     - [ ] `git status` 是否有未提交的改动？→ 有则先 commit + push
     - [ ] 需要打 tag 吗？→ `git tag vX.X.X && git push origin vX.X.X`
     - [ ] 本次是纯后端改动？→ 不需要 vite build
     - [ ] 本次涉及 `web/`？→ 必须 vite build → cp dist → go build（三步缺一不可）

## 方法论：复用已有基础设施

- **核心原则**：遇到新问题**先查已有基础设施**是否有可用数据或方法，再决定是否新建。PT-Forward 经过多次迭代，基础设施已经很完善（甚至有冗余），真正的瓶颈往往是**接入遗漏**而非能力缺失。
- **调研清单**（提"新方案"前必须过一遍）：
  1. `grep` 已有的 `Detector`/`Fetcher`/`Searcher`/`Resolver`/`Matcher`/`Service`/`Engine`（命名约定：能力以接口/结构体名暴露）
  2. **对比同类 handler** 的做法（如 `publish_torrents_handler` vs `manual_forward_handler`，一个接入了某设施，另一个可能遗漏）
  3. **查数据表内容**（`release_group_mappings`、`SiteCoverageCache`、`torrent_metadata` 等表可能已有你需要的数据，用 `sqlite3 data/pt-forward.db "SELECT ..."` 确认）
  4. 确认是"**能力缺失**"（需新建）还是"**接入遗漏**"（已有设施没调用）
- **典型案例**（§56.33 讨论）：tid 反查（`reseed.SearchAndVerifyMatch` 已有，orphan/recovery 已调用）、源站识别（`SourceSiteDetector.Detect` 已有，publish_torrents 已接入）、三源字段合并（`metadata.Merge` 已有，handleMerge 已调用）—— 六个看似需要"新建"的问题，实际全是"接入遗漏"。

## 灵魂四问（每次代码改动后必须逐条审核）

1. **nil 安全**：所有指针返回值是否检查了 nil？map 查找是否有 ok 判断？type assertion 是否用了 comma-ok 模式？
2. **边界安全**：空输入/空 DB/context 取消/并发锁竞争等边界情况是否处理？`context.WithTimeout` 后是否都调了 `cancel()`？锁是否有嵌套导致死锁风险？
3. **回归通过**：`go vet` + `go test` + `vue-tsc` + `eslint` 是否全部通过？
4. **前端构建**：本次改动是否涉及 `web/` 目录？如果是，是否执行了 `vite build → cp -r web/dist frontend/dist`？`vue-tsc`/`eslint` 只是验证，**不是构建**。`go build` embed 的是 `frontend/dist`，不是 `web/src`。跳过构建 = 前端改动不生效。

## 后端验证与部署

改完后端代码，回归审核通过后执行完整编译部署流程：

1. `go vet ./internal/... ./cmd/pt-forward/...`（**不要**用 `./...`）
2. `go test ./internal/... -count=1 -timeout 180s`
3. `CGO_ENABLED=1 /home/incast/.local/go/bin/go build -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" -o pt-forward ./cmd/pt-forward/`
4. `systemctl --user restart pt-forward && sleep 2 && systemctl --user is-active pt-forward`

**systemd 服务配置（OTA 兼容）**：`~/.config/systemd/user/pt-forward.service` 的 `[Service]` 段必须用 `Restart=always`（**不能**用 `on-failure`）。原因：OTA 热更新流程是"下载新二进制 → 校验 → 原子 rename → 主动 `exit 0` 等待 systemd 重启"，`on-failure` 不重启正常退出，会导致 OTA 后服务永久停止、网页连不上。修改配置后执行 `systemctl --user daemon-reload`。`systemctl --user stop` 仍可正常停服务（`Restart=always` 只影响进程退出/崩溃，管理员 `stop` 不触发重启）。

## 前端验证与部署

改完前端代码后跑 `vue-tsc -b --noEmit` + `npx eslint src/` 确认零错误。

前端是 Go embed（`frontend/spa.go` 用 `//go:embed all:dist`），修改前端后必须重新构建并重新编译 Go 二进制才能生效，完整流程：

1. `cd web/ && rm -rf node_modules/.vite && PATH="/home/incast/.local/bin:$PATH" ./node_modules/.bin/vite build`
2. `rm -rf frontend/dist && cp -r web/dist frontend/dist`
3. `CGO_ENABLED=1 /home/incast/.local/go/bin/go build -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" -o pt-forward ./cmd/pt-forward/`
4. `systemctl --user restart pt-forward && sleep 2 && systemctl --user is-active pt-forward`

## Docker 镜像

### 镜像地址

- **GHCR**：`ghcr.io/ranfish/pt-forward:latest`（国外用户，public）
- **Docker Hub**：`ranfish/pt-forward:latest`（国内用户，配合加速器）
- 国内 Docker 加速器配置在 `/etc/docker/daemon.json`（`registry-mirrors`）

### Dockerfile

- 运行时镜像：`debian:trixie-slim`（glibc 2.40）
- mpv：`bin/amd64/mpv-new`（1.5MB 精简编译版，编译方法见下方"mpv 编译"）
- ffmpeg/ffprobe：`apt install ffmpeg`（不再用预编译二进制）
- 中文字体：`apt install fonts-noto-cjk`
- 数据卷：`/data`（SQLite）、`/config`（配置）、`/logs`（日志）
- WORKDIR `/`（让 `./logs` → `/logs`）
- ENTRYPOINT `/usr/local/bin/pt-forward`

### Docker 本地构建

```
sg docker -c "docker build --build-arg VERSION=$(git describe --tags --always --dirty) -t pt-forward:latest ."
```

### GitHub Actions 自动发布

- **docker-publish.yml**：push 到 main 或打 tag 时自动构建推送到 GHCR + Docker Hub
- **release.yml**：打 tag `v*` 时自动编译二进制上传到 GitHub Release（OTA 用）
- **GitHub Secrets**：`DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN`（Docker Hub Personal Access Token）

### 用户部署

```bash
mkdir -p data config logs
# 下载 docker-compose.yml
docker compose up -d
```

更新方式：
- **OTA**：前端侧栏 → 检查更新 → 立即更新（自动下载替换二进制 + 重启）
- **镜像**：`docker compose pull && docker compose up -d`

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

## 全局转载发布规则（§30.5）

1. **禁止向任何站点发布 9KG/色情/成人内容资源**
2. **禁止发布源站带 禁转/独占/谢绝转载/限时禁转 标签或标题/副标题中带上述字样的种子**
3. **CatEDU 小组资源默认禁转**

## PT-IDX 项目（云端指纹服务）

- **项目路径**：`/home/incast/PT-IDX`
- **Go module**：`github.com/ranfish/pt-idx`
- **DB**：PostgreSQL，本地实例，数据库名 `pt_idx`
- **服务**：`systemctl --user restart pt-idx`（用户级 systemd 服务，端口 8766）
- **CGO**：不需要（纯 Go + pgx 驱动）

### PT-IDX 验证与部署

1. `go vet ./...`
2. `go test ./... -count=1 -timeout 180s`
3. `/home/incast/.local/go/bin/go build -ldflags "-s -w" -o pt-idx ./cmd/pt-idx/`
4. `systemctl --user restart pt-idx && sleep 2 && systemctl --user is-active pt-idx`

### PT-IDX 代码复用规则

- `bencode.go` + `compute.go` 从 PT-Forward **手动复制**，禁止 import PT-Forward 包
- pieces_hash 计算逻辑必须与 PT-Forward **逐字节一致**
- PT-Forward 侧 bencode/compute 如有修改，PT-IDX 侧必须同步

### PT-IDX 数据采集规则

- 批量采集器/RSS 订阅器下载 .torrent 后立即计算 pieces_hash，**丢弃 .torrent 数据**
- 禁止在磁盘上持久化 .torrent 文件
- 站点凭证（cookie/passkey）必须 AES-GCM 加密存 PostgreSQL
- 禁止在日志中出现任何凭证
- 配置文件含敏感信息，必须加入 `.gitignore`

## 版本管理

- **编译部署前**：必须先提交并推送代码，然后在 commit message 中注明版本号和变更内容
- **禁止先部署后提交**：代码必须在 git 历史中先于部署生效
- **打 tag 发版**：`git tag v0.0.x && git push origin v0.0.x`（触发 Docker 镜像 + GitHub Release 自动发布）
