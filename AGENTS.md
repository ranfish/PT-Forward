# PT-Forward 项目 Agent 指令

## Skills 索引（按需加载，正文在 .claude/skills/）

| skill | 用途 | 触发时机 |
|-------|------|---------|
| `ptf-dev-protocol` | 开发前协议：设计文档优先 + 复用优先 | 任何开发/修复/优化动手前 |
| `ptf-quality-gate` | 灵魂四问 + 回归审核 + 强制清单 | 任何代码改动后、commit 前 |
| `ptf-deploy` | 编译部署脚本 + 打 tag 发布 | 部署到 29、发版时 |
| `ptf-infra-notes` | mpv 编译/截图引擎/采集策略/CookieCloud/Playwright | 涉及基础设施时 |
| `pt-idx-ops` | PT-IDX 云端指纹服务运维 | 操作 PT-IDX 时 |

> ⚠️ 强制流程内容（铁律/灵魂四问/强制清单）已内嵌本文件，**必须执行**；skills 是其扩展细节，加载 skill 不替代本文件的强制项。

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

**版本**：v0.0.823（已发布）。

**主线状态一句话**：幸运专线切片 0-4 全部跑通（配置中心/执行器/发布 UI/批量发布/补推），新架构（资产消费白名单+四终态模型）实战验证中（53534/53535 pushed 首两例）；编辑器六 Tab/发布页打磨/数据治理（§59.135-160）全部收官。

**下一步**：
1. 幸运站持续验证：LuckAudit 过审率观察（站内信驱动修正）+ 批次多站场景 + 补推按钮实测
2. 发布站点适配推进：第二站接入（HTML 上传→diff→落库→DryRun 验证→实战）——每站适配第一步=问用户经验（§59.151 铁律）
3. R3-5/R3-6 遗留：TagApplier 灰度已随 §59.156 退役；team 域词条建设待做

**活跃观察项（未闭环）**：
- 53534/53535 等 9 个新种 LuckAudit 过审情况（用户读站内信驱动）
- 错加的无关种清理：UBits The.Drama（52394）/ZmWeb 200 Million（10136）在 243 下载器
- build-backend go test 偶发"首跑 FAIL 重跑过"（两次，未定位）
- 阮玲玉 no_transfer_until=09-02 20:28 过后 ready 自动放行验证（§59.162 两态首例）

**行为铁律（全文保留——约束每次开发）**：
- **发布链资产消费白名单**：发布=纯消费种子配置落库资产。网络动作仅允许 ①目标站交互（pre-audit/dedup/上传/下载新种）②下载器 RPC——PTGen/图床转存/豆瓣等外部 API 一律禁止（发布用 assembleDescription 纯本地组装，禁止复用采集组件）
- **发布四终态**：pushed/pushed_existing（302 权威 ID 自动推种）/existing（信文案不信页面 ID，不推走辅种）/failed——已存在=终态非失败；重试=人工幂等再执行，无自动重试引擎
- **上传判定二元化**：仅接受成功页种子（302 最终 URL+uploaded=1）；body 通用提取禁止（推荐位误推）；"其它页面的种子是辅种业务的工作"
- **幸运判据八条**（§59.151 终版：MI 唯一真相/HDR Profile 语义/语言四通道/粤语复合/媒介 title 词优先/音频组合键/英语条件映射/映射完备）——全文见 docs/31 §59.151 终局节
- **站点代理解析公共单点**：`site.ResolveSiteProxy(db,ctx,site)` + `httpclient.NewSiteHTTPClient` 组合，禁止裸 client（§59.156）

**历史详情**：§55-§59.161 全过程见 `docs/31-模块设计决策记录.md`（grep "§59.X" 定位章节）。

**⚠️ 本段维护铁律**：任务焦点只保留——版本/下一步/未闭环观察项/行为铁律。完成的章节一律一句索引行（`§59.X 见 docs/31`），**禁止累积过程叙事**（本段曾膨胀至 11.4k token——2026-09-01 治理）。


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
- **设计决策**：所有设计决策记录到 `docs/31-模块设计决策记录.md`（grep "§59.X" 定位）
- **敏感信息**：禁止保存 cookie/passkey/apikey/token
- **适配器文档**：`docs/32-站点适配器设计/<站点>.md`；站点数据原样写入不杜撰
- **删除代码后**：跑 `go vet ./internal/... ./cmd/pt-forward/`（不用 `./...`——cmd/verify-pieces-hash 有已知冲突）
- **Git 提交**：禁止提交 `data/`、`PT0/`～`PT8/`、`*.torrent`、`logs/`、`*.db*`（详见 .gitignore+pre-commit hook 双保险）
- **版本号**：编译必须 `-X main.version=$(git describe --tags --always --dirty)`
- **环境隔离铁律**：开发只在 29；生产（249/242/22 等）只读诊断+scp 二进制，禁止直接改生产；生产更新由用户执行
- **部署铁律**：先提交后部署（commit+push 必须在编译部署前）；前端涉及必走三步（vite build → cp dist → go build）；完整流程见 skill `ptf-deploy`
- **部署后必须数据级验证**（版本号/migration 记录/功能端点实测——脚本退出码≠生效，§59.155 竞态教训）
- **设计文档改动后必须 `wc -l` 行数验证**（docs/31 曾被清空未察觉——§59.161 教训）


## 方法论：复用已有基础设施

**核心**：新问题先查已有基建（Detector/Fetcher/Searcher/Resolver 等命名约定+同类 handler 对比+DB 表内容），区分"能力缺失"vs"接入遗漏"；**优先公共函数**（grep 同名/同义函数，私有跨包先提升公共——重复造轮子是 §59.26/§59.156/§59.159 三次事故同型）。完整调研清单与典型清单见 skill `ptf-dev-protocol`。


## 回归审核第一项（每次审核先查）

- **AGENTS.md 任务焦点同步**：`git log --oneline -10` 的版本推进与"当前主线"描述对照——主线/支线完成项是否已反映？滞后即先补（历史教训：v0.0.631、v0.0.642、v0.0.670 三次同类违规）。

## 灵魂四问（每次代码改动后必须逐条审核）

1. **nil 安全**：所有指针返回值是否检查了 nil？map 查找是否有 ok 判断？type assertion 是否用了 comma-ok 模式？
2. **边界安全**：空输入/空 DB/context 取消/并发锁竞争等边界情况是否处理？`context.WithTimeout` 后是否都调了 `cancel()`？锁是否有嵌套导致死锁风险？
3. **回归通过**：`go vet` + `go test` + `vue-tsc` + `eslint` 是否全部通过？
4. **前端构建**：本次改动是否涉及 `web/` 目录？如果是，是否执行了 `vite build → cp -r web/dist frontend/dist`？`vue-tsc`/`eslint` 只是验证，**不是构建**。`go build` embed 的是 `frontend/dist`，不是 `web/src`。跳过构建 = 前端改动不生效。

## 验证与部署

后端/前端验证部署完整流程与脚本：见 skill `ptf-deploy`（`bash .claude/skills/ptf-deploy/scripts/build-backend.sh` / `build-frontend.sh` / `release-tag.sh vX.X.X`）。关键点：Go/Node 必须全路径（系统 PATH 无）；CGO_ENABLED=1；版本号 ldflags；前端三步缺一不可（vite build → cp dist → go build）；systemd 必须 Restart=always（OTA 兼容）。

<!-- 镜像地址/Dockerfile/本地构建/CI 发布/用户部署：已抽出至 skill ptf-deploy（.claude/skills/ptf-deploy/SKILL.md）-->

<!-- mpv 编译/截图工具引擎/数据采集策略/CookieCloud/Playwright 注意事项：已抽出至 skill ptf-infra-notes（.claude/skills/ptf-infra-notes/SKILL.md）-->

<!-- PT-IDX 云端指纹服务（路径/DB/部署/代码复用/采集规则）：已抽出至 skill pt-idx-ops（.claude/skills/pt-idx-ops/SKILL.md）-->

## 版本管理

- **编译部署前**：必须先提交并推送代码，然后在 commit message 中注明版本号和变更内容
- **禁止先部署后提交**：代码必须在 git 历史中先于部署生效
- **打 tag 发版**：`git tag v0.0.x && git push origin v0.0.x`（触发 Docker 镜像 + GitHub Release 自动发布）
