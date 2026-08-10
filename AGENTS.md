# PT-Forward 项目 Agent 指令

## 当前任务焦点（新会话必读）

**版本**：v0.0.547（已发布）。

**当前主线**：辅种误匹配修复 + 孤儿恢复重构 + RSS recheck 状态机重构 + 配置表合并 **全部完成**。

### ✅ 已完成（v0.0.228 → v0.0.547）

**Phase 1（v0.0.228~v0.0.310）：转载/发布/标题重组/技术特征模型** — 详见 §56 全系列。

**Phase 2（v0.0.522~v0.0.547）：辅种/孤儿/RSS/配置表**

- ✅ **辅种误匹配修复栈**（§59.1~§59.9）：
  - ValidateInjection 公共方法 + 孤儿恢复接入（v0.0.522）
  - 续集号比较 extractSequelNumber（I~IX，v0.0.528~v0.0.529）
  - 标题关键词词长 4→3 + 停用词（v0.0.531）
  - truncation retry 组名泄漏修复（v0.0.541）
  - negative cache 确定性失败 + 覆盖→追加 bug（v0.0.530）
- ✅ **孤儿恢复重构**（§59.14~§59.16）：
  - ClassifyTorrent 统一分类系统（movie/tv_series/music + season_pack/partial_pack/single_episode）
  - 按分类选择搜索策略 + 文件级大小匹配修复（v0.0.537~v0.0.539）
  - 同站扩展：主恢复命中后在同一站点搜索其他集（v0.0.540）
  - 剧集孤儿修复（目录总大小→单集文件大小，v0.0.537）
- ✅ **RSS recheck 状态机重构**（§59.12）：
  - 移除 rss_recheck_waiting 全局任务（v0.0.523）
  - 废弃 NonFreeRecheck/PendingScoringQueue/skipped_nonstate（v0.0.524）
  - FreeWaitRecheckSec/MinRemain 生效 + wait-queue API（v0.0.525~v0.0.527）
- ✅ **配置表合并**（§59.13）：
  - seeding_client_configs + download_client_configs → 统一表（阶段 A/B/C，v0.0.535~v0.0.546）
  - Role 字段区分 seeding/download，评分清理只对 seeding 生效（v0.0.534）
- ✅ **RSS seen 每订阅独立**（§59.17）：UNIQUE 约束加 subscription_id + IsSeen 每订阅 + MarkSeen UPSERT（v0.0.543~v0.0.545）
- ✅ **DryRun 缓存修复**（§59.18）：试运行从 rss_torrent_seen 恢复缓存的免费状态（v0.0.546）
- ✅ **死代码清理**：删除 ExtractType（v0.0.547）、海胆 pieces_hash_api 修正（v0.0.542）

### 端到端验收状态

| 端点/功能 | 状态 |
|------|------|
| 辅种引擎（L0/L1/L2 + 注入校验 + 续集号 + 标题关联性）| ✅ |
| 孤儿恢复（分类 + 文件级搜索 + 同站扩展 + 注入校验）| ✅ |
| RSS 订阅（每订阅独立 seen + FreeWait + DryRun 缓存）| ✅ |
| 配置表统一（seeding + download 合并，阶段 A/B/C 完成）| ✅ |
| 发布管线（CrossSeedPanel + 5 API + 手动转发）| ✅ |

**关键事实**：
- 开发环境 29（systemctl --user pt-forward，端口 8765）
- 多生产环境 Docker（249/243/65/12/pt30），端口 8765
- 辅种引擎三层匹配：L0 pieces_hash、L1 fingerprint、L2 search_verify
- 孤儿恢复：ClassifyFromDir 分类 → 按类型选择搜索策略 → 同站扩展
- 辅种引擎与孤儿恢复共享 VerifyMatchWithTruncationCheckAndSource + ValidateInjection
- 配置表合并后统一表 seeding_client_configs（Role=seeding/download 区分）
- RSS seen 去重已改为每订阅独立（UNIQUE: site_name + torrent_id + subscription_id）
- 下载器是去重的唯一权威（同 info_hash 拒绝），RSS 引擎不做跨订阅去重

**待办**：
- （暂无）

**完整设计文档**：`docs/31-模块设计决策记录.md` §55-§59.18（71000+ 行，按需读特定章节，不要一次读全文）。

## 环境信息

- **Go**：`/home/incast/.local/go/bin/go`（v1.25，系统 PATH 中无 go，必须用全路径）
- **Node**：`/home/incast/.local/bin/node`（v22，系统 Node 18 不兼容，必须用此路径）
- **CGO**：后端编译必须 `CGO_ENABLED=1`
- **DB**：`data/pt-forward.db`（SQLite）
- **服务**：`systemctl --user restart pt-forward`（用户级 systemd 服务，端口 8765）
- **开发环境 29**：10.0.0.29，Agent 唯一可操作的机器（编译/测试/部署/DB）
- **生产环境 249**：10.0.0.249 Docker（docker compose），端口 8765，**禁止 Agent 直接操作**
- **pt30 环境**：pt30.ranfish.uk（192.168.100.30）Docker，**禁止 Agent 直接操作**
- **65 环境**：10.0.0.65 Docker，**禁止 Agent 直接操作**
- **243 环境**：10.0.0.243 Docker，SSH root/~/.ssh/xsy，**禁止 Agent 直接操作**
- **12 环境**：10.0.0.12 Docker，**禁止 Agent 直接操作**
- **PT8 环境 242**：10.0.0.242 Docker，**禁止 Agent 直接操作**
- **22 环境 22**：10.0.0.22 Docker，**禁止 Agent 直接操作**
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
- **⚠️ 环境隔离铁律（最高优先级，违反=事故）**：
  1. **开发只在开发机（29）上进行**：所有代码编写、编译、测试、部署仅在开发环境 29 执行（`systemctl --user restart pt-forward`）。
  2. **可以 SSH 到生产环境查看状态**：可以 SSH 到生产机（249/242/22）查看日志、磁盘空间、Docker 状态、DB 数据等（只读诊断）；可以 scp 二进制到生产环境；但**禁止**：在生产环境编译、在生产环境直接修改代码或数据。
     - **⚠️ 部分 Docker 生产环境 scp 不可用**（exit=1），改用 SSH 管道传输二进制：`gzip -c pt-forward | ssh <user>@<host> 'cat > /tmp/pt-forward.gz && gunzip -f /tmp/pt-forward.gz && chmod +x /tmp/pt-forward'`，然后用 `sudo docker cp /tmp/pt-forward <container>:/usr/local/bin/pt-forward && sudo docker restart <container>` 部署。
  3. **生产环境由用户自行更新**：生产环境的 Docker 镜像更新（`docker compose pull && docker compose up -d`）由用户自行执行。Agent 的职责是提交代码、打 tag、推送（触发 CI/CD 构建镜像），不负责生产部署。
  4. **Agent 可以做的**：在 29 上编译部署验证 → commit + push → 打 tag → 告知用户生产环境需更新。SSH 到生产环境做只读诊断（磁盘/日志/DB 查询）。
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
- **⚠️ 优先使用公共函数（最高优先级）**：遇到功能需求时，**必须先 grep 查是否已有公共实现**，禁止重复造轮子。
  1. `grep -rn 'func.*Xxx' internal/` 查全项目是否已有同名/同义函数
  2. 特别注意跨包重复：`publish.ExtractGroupName` 和 `reseed.ExtractGroupName` 曾经是两份副本，改一处漏一处导致 bug（已合并到 `util.ExtractGroupName`）
  3. **典型清单**：`util.ExtractGroupName`（制作组名提取）、`reseed.ExtractSearchKeyword`（搜索关键词提取）、`reseed.SearchAndVerifyMatch`（搜索+验证匹配）、`reseed.VerifyMatchWithTruncationCheckAndSource`（匹配验证核心）、`reseed.ValidateInjection`（注入前校验）、`util.ClassifyTorrent`/`util.ClassifyFromDir`（种子类型分类）、`util.DetectContentType`（音乐/视频检测原语）
  4. 如果已有函数是私有的且跨包需要，**先考虑提升为公共函数**（改首字母大写或委托到 `util` 包），而非在新包重写一份

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
