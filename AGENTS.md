# PT-Forward 项目 Agent 指令

## 开发流程铁律（最高优先级，v0.0.606 教训）

**任何开发、修复、优化，动手前必须先查设计文档**：

1. `docs/31-模块设计决策记录.md`（grep 关键词定位章节，按需读 50-100 行上下文）
2. `docs/32-站点适配器设计/<站点>.md`（站点行为/标记/规则的权威定义）

```
问题/需求
  ↓ ①先查设计文档
  ↓   有记录 → 按设计实施；实现与设计不符 → 修正实现（不是绕过设计）
  ↓   无记录 → 与用户讨论设计 → 设计落盘 → 再实施
  ↓ ②实施开发
  ↓ ③灵魂四问 + 回归审核
  ↓ ④设计文档更新（实施中的偏差与新发现回写设计文档）
```

**反面案例（v0.0.606）**：extractFlags 用 `Contains("禁转")` 扫声明文本，偏离 §32/keepfrds.md 六章定义的"站点文字标记"权威源；在其上叠加致谢模板（含"禁转PTT"）时未查设计 → 跨词伪命中 + 定向禁转误判 + 误标 3 个种子禁转。修复耗时远超先查文档的成本。

## 每次代码改动后强制清单（不可跳过）

1. **设计落盘** — 改动记录到 `docs/31-模块设计决策记录.md` 或 `docs/32-站点适配器设计/`
2. **灵魂四问** — nil 安全 / 边界安全 / `go vet`+`go test` / 前端构建（如涉及 `web/`）
3. **先提交后部署** — `git commit + push` → 再编译部署
4. **打 tag** — 如需发布新版本（`git tag vX.X.X && git push origin vX.X.X`）
5. **AGENTS.md 更新** — 如任务状态变化

## 当前任务焦点（新会话必读）

**版本**：v0.0.631（已发布）。

**当前主线**：**种子配置页（数据层）打磨中**。字典（§59.35）已完成后回线，近期完成：§59.42 海报可信图源白名单+PTGen fallback（doubaninfo/pixhost 家族+pixho.st，Tab2 闭环）；§59.43 全站字节显示统一（KiB/MiB/GiB+两位）+ ESLint 硬约束防私有副本；§59.44 资源视图（虚拟种子）——(client,path,name) 三元组键 + ResourceResolver 正式接口，修复 49% 编辑 404（158/158 组全通）；§59.45 Tab4 简介对齐 PTGen 为准——朋友站 kdouban 框逆向提取（139 MI 污染修复面，8 发布者自贴保留）；§59.46 Tab4 PTGen 四项修复——doubaninfo format 解析/清空 BBCode 缓存/展示兜底/kdouban 空 body 回退（行业基准：PT-Gen 数据类型 = format BBCode，examples 四项目实证）。编辑器六 Tab 已闭环：Tab2 海报/Tab4 简介/Tab5 MI。**下一步：Tab 3 截图端到端（编辑器最后一块）→ 预览→审核闭环全流程 → 发布页重构（tag_config 激活，§56.22）。**

**支线完成**：幽灵种子巡检 v2（§59.31）；Token Registry P1-P3（§59.27）；清 seen 重推（§59.33）；侧栏信息架构重排（§59.32）；Remux 规格解析修复（v0.0.629）；Tab1 片源写法与 Encode 判定 + DOM key 归一化（§59.34，v0.0.632-634，两轮审计）。

**支线插曲（v0.0.642-643）**：compliance 关键词层防误伤（§56.2x 修正）——"The Pornographers" 被 "Porn" 子串误标系统禁转；修复 = ASCII 关键词词边界匹配（compliance.MatchKeyword 单一入口，checker/reseed/pipeline 三套统一）+ 中文误报排除补实现（成人+教育/高考/大学/学院，设计稿落地）。已知边界：xXx 由 qui 层白名单覆盖、Adult Swim 待案例。

**遗留**：Tab 3 截图 / Tab 4 PTGen 端到端验证（当前主线下一步）；发布页重构（含 tag_config 激活，§56.22，字典完成后启动）；MarkStatus 跨订阅写（§59.17 违反，预存在）；runAnalyze 分级（读 DB 跳过重拉）；integration TestE2E_SeedingFullChain flaky（eng2.Start 竞态，§59.35 P3 记录）。

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

### ✅ 种子配置页（§59.19~§59.21，全部完成）

**Phase 3 基础设施（v0.0.548~v0.0.551）**：
- ✅ torrent_snapshots 快照表 + syncer 扩展 + snapshot-paths API（v0.0.549）
- ✅ torrent_metadata 加 14 TechProfile 平铺字段 + category/form（v0.0.550）
- ✅ CrossSeedPanel maintenanceOnly prop + fillFormFromPreset + saveOnly（v0.0.551）
- ✅ SeedConfig.vue 种子配置页框架（mock 数据）（v0.0.548）

**后端实施（v0.0.552~v0.0.553，20/20 完成）**：
- ✅ torrent_metadata 加 statement + bdinfo 列 + 22 个无映射站点关闭 is_source（migration #7）
- ✅ Detect() Step 2 加 cookie 检查 + SelectFetchSite() 新函数（制作组优先）
- ✅ GET/PUT /publish/seeds 列表/读写 API（含 compliance + 9 字段校验 + 5 态标注）
- ✅ GET /downloads/snapshot-unconfigured + POST batch-fetch 异步批量获取 + 进度轮询
- ✅ GET/PUT /publish/fetch-priority（获取数据站点优先级）
- ✅ GenerateThanksQuote 加 {group_name} + 渲染器始终添加致谢
- ✅ handleRefresh('mediainfo') 联动 BDInfo + handleRefresh('intro') 不再返回 subtitle
- ✅ CheckPublishEligibility 增加源站映射检查（第 0 层合规）
- ✅ persistAnalysis 写入 statement + bdinfo
- ✅ MainDataCron 列名 bug 修复 + /seeding role 过滤（v0.0.553）

**前端实施（v0.0.552~v0.0.555，17/17 完成）**：
- ✅ SeedConfig.vue 接真实 API + snake_case + 5 态标签 + 禁转/无映射只读（v0.0.552）
- ✅ CrossSeedPanel 六 Tab 改造（18 字段只读 / 海报 / 截图左右分栏 / 简介 / 媒体信息 / 已过滤声明）（v0.0.552）
- ✅ CrossSeedPanel 维护模式 GET/PUT API + 编辑→预览→完成三步流程（v0.0.554）
- ✅ BatchFetchPanel.vue 批量获取弹窗 + 进度轮询（v0.0.552）
- ✅ BBCode→HTML parseBBCode 工具函数（v0.0.554）
- ✅ 发布预览页面（方案 A：保存即预览）（v0.0.554）
- ✅ 截图左右分栏布局（URL 列表 + 大图预览）（v0.0.555）
- ✅ 路由 /publish/exclusions → /publish/rules（4 Tab 含声明过滤 + 小组映射只读）（v0.0.552）
- ✅ 导航简化（隐藏一站多种/一种多站）（v0.0.552）

**回归审核修复（v0.0.556）**：
- ✅ batch-fetch runBatchFetch 加 defer recover()
- ✅ Migration #8 错误不再忽略
- ✅ handleRefresh('mediainfo') BDInfo 改调 bdinfoScanner.ScanIfBD

**is_local 本地发布 vs 转种上盒（§59.21，v0.0.557）**：
- ✅ ClientConfig 加 IsLocal bool（用户添加下载器时必填）
- ✅ batch-fetch：is_local=true 追加本地 mediainfo，is_local=false 仅源站采集
- ✅ GET API 通过 client_id 查 is_local 返回前端（后端是唯一真相源）
- ✅ handleRefresh：is_local=false 时从源站重抓（mediainfo/screenshots）
- ✅ 下载器编辑表单加 is_local 必填 radio（本地发布/转种上盒）
- ✅ CrossSeedPanel 按钮文案根据 is_local 切换

**暂缓**：
- ⏸ 遗漏 3（一种多站/一站多种重构），待种子配置页有数据后再讨论

### ✅ 野马 OpenAPI 适配（§59.22，v0.0.558~v0.0.560）

- ✅ pieces_hash 接入（`SearchByPiecesHash` 调 `/openApi/torrent/fetchTorrentIdWithPiecesHash.json`，APIKey 认证）
- ✅ generateDownloadKey 迁移 POST OpenAPI（优先 APIKey + fallback Cookie）
- ✅ fetchBasicInfo GET→POST（原 GET 已废弃）
- ✅ 搜索迁移 Open API（`/openApi/torrent/fetchOpenTorrentList.json`）
- ✅ migration #9-#12：supports_pieces_hash_api + auth_type=apikey
- ✅ updateSiteRequest 补 SupportsPiecesHashAPI 字段（camelCase + nil 守卫）
- ✅ Cookie/APIKey 双输入框显示（show_cookie override）

### ✅ 下载器连接管理修复（§59.22，v0.0.561~v0.0.562）

- ✅ Reload 用 `context.Background()` 替代 `r.Context()`（3 处 create/update/delete）
- ✅ update handler 改用 `ReloadClient(name)` 只重连被编辑的客户端
- ✅ is_local 字段持久化遗漏修复（Updates map 缺 is_local 列）
- ✅ JSON 命名规范加入 AGENTS.md + 强制清单

### ✅ P2 snake_case 统一（v0.0.563~v0.0.567）

- ✅ 后端 4 个文件 34 个 struct 106 个字段 snake_case → camelCase
  - publish_torrents_handler.go / manual_forward_handler.go / metadata_handler.go / download_handler.go
- ✅ 前端 types.ts + publish.ts + downloads.ts + image-host.ts API 层全量对齐
- ✅ 前端 7 个 Vue 组件 API 调用参数 + 响应类型 + 模板引用全量对齐
- ✅ 5 轮回归审核，修复 13 处遗漏（vue-tsc 无法检测的运行时 JSON key 不匹配）
- ✅ 保留 snake_case：Go model JSON tag / raw map 响应 / DB 列名 / URL query param

### ✅ 野马 pieces_hash bencode 辅种（§59.24-§59.25，v0.0.570~v0.0.579）

- ✅ 辅种移除"源站"概念（§59.24，与发布业务解耦）
- ✅ 野马 pieces_hash bencode 算法分支（§59.25，SHA1(bencode(info.pieces))）
- ✅ ContentFingerprint 加 PiecesHashBencode + 懒计算回写
- ✅ 引擎按 adapter.Framework() 选择 hash 格式
- ✅ verifyL0Size 搜索失败时降级放行（方案 A，注入阶段 ValidateInjection 兜底）
- ✅ 端到端验证通过（249 环境，4 个种子 matched + injected）

### 端到端验收状态

| 端点/功能 | 状态 |
|------|------|
| 辅种引擎（L0/L1/L2 + 注入校验 + 续集号 + 标题关联性）| ✅ |
| 孤儿恢复（分类 + 文件级搜索 + 同站扩展 + 注入校验）| ✅ |
| RSS 订阅（每订阅独立 seen + FreeWait + DryRun 缓存）| ✅ |
| 配置表统一（seeding + download 合并，阶段 A/B/C 完成）| ✅ |
| 种子配置页（基础设施 + 设计）| ✅ |
| 种子配置页（前后端实施）| ✅ v0.0.552-555 |
| 种子配置页 is_local（本地发布 vs 转种上盒）| ✅ v0.0.557 |
| 野马 OpenAPI 适配（pieces_hash + 下载 + 搜索 + 认证）| ✅ v0.0.558-560 |
| 下载器连接管理（Reload context + 单客户端重连）| ✅ v0.0.561-562 |
| P2 snake_case 统一（106 字段 + 5 轮审核）| ✅ v0.0.563-567 |
| 辅种移除源站概念 + 目标站不按 isTarget 过滤 | ✅ v0.0.570-571 |
| 野马 pieces_hash bencode 辅种（端到端验证通过）| ✅ v0.0.576-579 |

**关键事实**：
- 开发环境 29（systemctl --user pt-forward，端口 8765）
- 多生产环境 Docker（249/243/65/12/pt30），端口 8765
- 辅种引擎三层匹配：L0 pieces_hash、L1 fingerprint、L2 search_verify
- 孤儿恢复：ClassifyFromDir 分类 → 按类型选择搜索策略 → 同站扩展
- 辅种引擎与孤儿恢复共享 VerifyMatchWithTruncationCheckAndSource + ValidateInjection
- 配置表合并后统一表 seeding_client_configs（Role=seeding/download 区分）
- RSS seen 去重已改为每订阅独立（UNIQUE: site_name + torrent_id + subscription_id）
- 下载器是去重的唯一权威（同 info_hash 拒绝），RSS 引擎不做跨订阅去重
- 种子配置页六 Tab：种子详情/海报与声明/视频截图/简介详情/媒体信息/已过滤声明，仅截图和简介可编辑
- 种子 5 态：禁转(flags)/系统禁转(compliance)/无源站映射/已审核/待审核+配置不完整
- 源站约束：制作组不在 release_group_mappings 的种子不可转发，无映射站点不可开启 is_source
- 发布预览采用方案 A（保存即预览）：PUT 同时保存+标准化+9字段校验+渲染
- is_local 区分本地发布（本地 mediainfo/mpv）和转种上盒（源站数据引用），后端是唯一真相源
- 野马 OpenAPI 认证：APIKey 优先（Header `Authorization`，无 Bearer），fallback Cookie
- 下载器编辑触发 ReloadClient（单客户端重连），create/delete 触发全量 Reload

**待办**：
- 端到端测试（需有本地下载器的环境验证 is_local=true 全流程）
- 生产环境部署（用户 OTA / docker compose pull）

**完整设计文档**：`docs/31-模块设计决策记录.md` §55-§59.22（67000+ 行，按需读特定章节，不要一次读全文）。

## 前端格式化函数规范（§59.43）

- **禁止页面私有字节/速率格式化函数**：所有体积/速率展示必须 `import { formatBytes / formatSpeed } from '@/utils/format'`（单点维护，防 toFixed 位数/进制/后缀漂移）
- ESLint 硬约束已启用（no-restricted-syntax）：私有 formatSize/formatBytes 同名副本 + toFixed×字节单位拼接形态直接报 error；utils/format.ts 本体豁免；非字节场景 `eslint-disable-next-line` 注释豁免
- 历史教训：§59.26 snake_case、§59.43 字节副本——"知道该统一"不等于"不会漏"，靠机制不靠记忆

## JSON 字段命名规范

| 场景 | 风格 | 示例 |
|------|------|------|
| Go model JSON tag | snake_case | `json:"supports_pieces_hash_api"` |
| API 专用请求/响应 struct（site/downloader 等） | **camelCase** | `json:"supportsPiecesHashApi"` |
| API raw map 响应（publish/seeds 等新 handler） | snake_case | `"info_hash": hash` |
| 前端 TS 类型 | 跟 API（struct 端点 camelCase，raw map 端点 snake_case） | — |

**核心规则**：在已使用 camelCase 的专用 struct（如 `updateSiteRequest`、`siteResponse`、`downloaderResponse`）中新增字段时，**必须用 camelCase**，否则前端反序列化失败。

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

## 回归审核第一项（每次审核先查）

- **AGENTS.md 任务焦点同步**：`git log --oneline -10` 的版本推进与"当前主线"描述对照——主线/支线完成项是否已反映？滞后即先补（历史教训：v0.0.631、v0.0.642、v0.0.670 三次同类违规）。

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
