# 04 API 输入契约与一致性

- **状态**：整改中（刀1=D1+D3a+D4 已落地 v0.0.1591·D2 待实施）
- **日期**：2026-11-24
- **基线**：`27be5f17`（`fix §59.564 刀B 5xx 业务日志窄版（审计 03 §2.3 P1·03 篇整改收官）`）
- **审计范围**：`internal/api/` 请求解析、参数默认值/上限、错误码与响应契约；抽样核对 `internal/middleware/maxbody.go`、`web/src/api/client.ts` 的前端消费方式。
- **审计方法**：扫描 `decodeJSONBody`/原始 `json.NewDecoder`、分页解析、`MaxBytesReader`、`Error/ErrorWithDetail` 调用与 5xx `err` 透传；定点阅读 `request.go`、`pagination.go`、`response.go`、`middleware/maxbody.go` 及代表 handler。

> 本文为代码质量定点精读结论，不是安全评估结论。行号/统计为基线快照，实施前请重新定位。

## 1. 结论摘要

- API 已有局部收敛基础：`decodeJSONBody`、`methodNotAllowed`、`parsePagination/parsePaginationWith` 已存在并被较多 handler 复用，说明契约统一是可行的增量治理，不需要推倒重来。
- 主要缺口集中在**契约边界不一致**：body 解码未限制未知字段、分页/limit 存在手写特例与无上限旧模式、错误码语义重载、5xx 对外 message 仍有内部 `err` 透传。
- 03 篇刀D（内部 err 透传客户端）确认归本篇：当前 5xx 透传面不大（抽样 18 处 `fmt.Sprintf(...%v, err)` + 1 处 `ErrorWithDetail(..., err.Error())`），前端未见对具体后端 5xx 文案做硬编码匹配，但通用错误展示会直接显示 `message`，整改需按“稳定 message + 服务端日志/detail 策略”小步收敛。

## 2. 现状基线（已有收敛点）

| 机制 | 现状 | 证据 | 评价 |
|---|---|---|---|
| JSON body 解码 | `decodeJSONBody` 统一 40001“请求格式错误” | `internal/api/request.go:21-26`，106 处调用 | 正向基线；仍有 7 处原始 decode 例外 |
| 405 响应 | `methodNotAllowed()` 单点 | `internal/api/request.go:30-33`，124 处调用 | 正向基线；但 code 仍为 40001 |
| 分页解析 | `parsePagination/With` 统一 page/size/key 优先级 | `internal/api/pagination.go:21-48` | 正向基线；存在业务特例与手写旧模式 |
| 请求体大小 | 全局 `MaxBodySize` 中间件 1MB/上传 32MB | `internal/middleware/maxbody.go:9-24` | 有全局兜底；handler 仍可局部覆盖 |

## 3. 重点发现

### 3.1 Body 解码契约偏松（P2）

**证据**：

- `internal/api/request.go:21` 使用 `json.NewDecoder(r.Body).Decode(dst)`，未设置 `DisallowUnknownFields()`，未设置解码后“body 只能有单个 JSON 值”的检查。
- `request.go` 已登记 6 处刻意例外（DryRun 40002 详情、form_config 响应 shape/校验合并、image_host/download 可选 body、site 批量审核坏 body 容忍）。
- 全局有 `MaxBodySize` 中间件（1MB，multipart 32MB）；`form_config_handler.go:72` 另设 5MB。

**风险**：

- 未知字段静默忽略，前端字段名拼写错误或旧字段残留时，API 可能“看似成功但未生效”。
- 未检查尾随多余 JSON 值时，请求体可携带额外数据而不被拒绝，契约不够严格。
- 例外点缺少统一测试锁定时，后续容易把“刻意例外”误扩散成普通模式。

**建议**：

- 新增严格解码变体（如 `decodeJSONBodyStrict`）只在**新接口/明确契约边界**启用：`DisallowUnknownFields` + 单对象校验；避免对存量接口一刀切。
- 对已登记例外保持显式注释与测试锚点；新增例外必须登记原因。
- body 大小继续由中间件兜底，handler 只在确有业务需要时显式放宽/收紧。

### 3.2 分页与 limit 契约不一致（P2）

**证据**：

- 统一 helper：`parsePagination` 默认 `page=1,size=20,maxPage=10000`，size 有效域 1..200，键优先级 `size > pageSize > page_size`（`pagination.go:14-48`）。
- 业务特例：`seed_list.go:50` 使用 `parsePaginationWith(r, 50, 100, 10000)`，注释明确是发布批量业务定案。
- 手写特例：`publish_handler.go:285-317` 结果列表新模式手写分页，注释说明“>200 clamp 到 200”与统一 helper“超限回落默认”语义冲突；旧模式 `limit` 仅在 `<1` 时回落 20，未见上限 clamp。
- 手写 Atoi：非测试代码仍有 6 处 `strconv.Atoi(r.URL.Query().Get(...))` 直接解析分页/limit（`publish_handler.go:278,287,288,290`，`seeding_handler_stats.go:106`，`seeding_handler_config.go:402`）。

**风险**：

- 同类分页接口的边界行为不同，调用方难以形成稳定预期；超大 limit 可能造成重查询或大响应。
- 手写解析容易漏掉 key 优先级、maxPage、上限或负数处理。

**建议**：

- 以 `parsePagination/With` 为默认入口；确有语义冲突的特例保留局部实现，但必须写明“为何不复用”并补测试。
- 对旧模式 `limit` 补上限（例如 200）并在文档/测试锁定；若担心兼容，可先 clamp + Warn 日志观测。
- 新增列表接口禁止手写 `Atoi+默认值`，统一走 helper。

### 3.3 错误码语义重载（P2）

**证据**：

- `request.go`：JSON 解码失败返回 `40001 "请求格式错误"`；`methodNotAllowed` 返回 `405 + 40001 "方法不允许"`。
- 非测试代码中 `40001` 作为 BadRequest 业务码出现 233 处，MethodNotAllowed 也复用 40001（helper 单点）。
- `response.go` 的业务 code 与 HTTP status 分离，但当前 code 粒度较粗，`50000`/`40001` 承载大量场景。

**风险**：

- 客户端若按 `code` 分支，难区分“参数格式错误”“方法不允许”“业务校验失败”等不同类型。
- 后续做错误码治理或前端提示细化时，历史重载会增加兼容成本。

**建议**：

- 短期不做全量 code 重编；先制定约定：HTTP status 表达协议语义，业务 code 表达可分支的业务/校验语义。
- 中期为 method not allowed、decode error、validation error 预留可区分 code（或保持 code 不变但保证 message 稳定并文档化）；改动前先确认前端是否依赖 code。
- 新增接口避免复用模糊 code 承载新语义。

### 3.4 5xx 内部错误透传客户端（P2，03 刀D）

**证据**：

- 抽样扫描：`internal/api` 中 18 处 `Error(w, ..., fmt.Sprintf("...%v", err))` 将内部 `err` 拼入 message；另有 `supported_sites_handler.go:35` 通过 `ErrorWithDetail(..., err.Error())` 透传。
- 代表位置：`publish_torrents_handler_torrents.go`（连接下载器/获取种子/保存/CRUD）、`manual_forward_handler.go`（源站重新获取/MediaInfo）、`publish_torrents_handler_seed_detail.go`（502/500 带 err）、`cloud_fp_handler.go`、`iyuu_handler.go`。
- 前端消费：`web/src/api/client.ts:85` 统一取 `error.response?.data?.message || error.message || ...` 展示；仓库内未检索到前端对上述具体 5xx 中文 message 的硬编码匹配。
- `response.go` 支持 `detail` 字段，但本次未见通用 API 客户端消费 `detail`。

**风险**：

- 内部错误字符串可能暴露下游地址、驱动、文件路径、SQL/解析细节，且不稳定。
- 前端虽未硬编码匹配这些字符串，但用户可见文案会随内部异常变化，影响体验与排障一致性。

**建议**（刀D 整改范围）：

- **第一刀范围**：仅收敛“已拿到 err 且返回 5xx/502 的 message 透传”高频路径，优先 03/04 已点名的 `publish_torrents_handler_torrents.go`、`manual_forward_handler.go`、`publish_torrents_handler_seed_detail.go`、`iyuu_handler.go`、`supported_sites_handler.go`。
- **契约策略**：对外 `message` 固定为用户可理解的稳定文案；`err` 只写 zap 日志（带 `op/request_id`/关键标识）。如确需给前端/运维可见细节，使用受控 `detail` 或错误码，不直接透传原始 `err.Error()`。
- **前置确认**：发布前抽查前端是否依赖 `detail`/特定 message；本次仓库快照未见硬编码依赖，但前端构建产物不在仓库内，仍建议灰度观察。
- **不做**：不改 `response.Error` 自动打日志；不把所有 4xx/5xx 文案大扫除；不在同一刀修改业务语义。

### 3.5 路径/查询参数解析缺少统一 helper（P3）

**证据**：

- 分页已有 helper，但普通 `int/bool/uint` 查询参数仍分散在 handler：`parseUintQuery`、`parseBoolQuery` 已有局部使用（`seed_list.go`），也存在直接 `Atoi`。
- 路径参数多为 `strings.TrimPrefix/Split` 手写，错误时返回 404/400 的 code 和文案不完全统一。

**风险**：

- 参数非法值的处理策略不一致（忽略、回落默认、报 400）会造成契约漂移。
- 重复手写解析增加小错误概率。

**建议**：

- 增量引入小型 query/path 解析 helper（仅处理通用 int/bool/uint/enum），不引入重框架。
- 约定非法值策略：分页类回落默认或 clamp 需有明确理由；业务 ID/枚举类非法值优先 400。

## 4. 整改建议（按刀）

| 候选刀 | 范围 | 预期收益 | 风险 | 前置 |
|---|---|---|---|---|
| D1（03刀D） | 5xx/502 内部 err 透传 message 收敛为稳定文案 + 服务端日志 | 减少内部细节暴露、稳定契约 | 中：前端展示文案变化 | 确认前端不依赖具体 message；保留必要 detail 策略 | → 刀1 v0.0.1591 已完成（21 处·detail 收敛） |
| D2 | 旧模式 `limit`/手写分页补上限与 helper 收敛 | 防止大查询、统一分页行为 | 低-中：行为边界变化 | 为 clamp/回落补测试 |
| D3 | 严格 JSON decode 变体（新接口/选定接口试点） | 提前暴露字段拼写/契约漂移 | 中：存量兼容性 | 选无存量兼容包袱接口试点 | → 刀1 v0.0.1591 helper+锚已落地（试点目标待新接口） |
| D4 | 错误码约定文档化（不全量改码） | 降低 code 语义重载扩散 | 低 | 先盘点前端对 code 的依赖 | → 刀1 v0.0.1591 已完成（附录C） |

## 5. 验证建议

- **契约测试**：为分页边界（负数、超大、非法字符串、多 key 优先级）、decode 错误、method not allowed 补/保留 table-driven 测试。
- **前端抽查**：发布 D1 前检查 web 构建产物/运行时是否依赖具体 5xx message；至少确认通用错误提示可接受稳定文案。
- **日志观测**：D1 后通过 zap `op` + `request_id` 能定位原始 err；必要时对被 clamp 的 limit 打 Warn 观察调用方。
- **兼容策略**：涉及默认值/上限语义变化时，优先“clamp + 观测”而非静默回落到更小值。

## 6. 本次未覆盖

- 未逐条盘点全部 40001/50000 业务码语义；只确认了重载现象与代表证据。
- 未审读所有 handler 的路径参数解析；结论基于模式扫描和代表文件。
- 未验证仓库外前端构建产物/移动端/脚本调用方对 `message/detail/code` 的依赖。
- 未覆盖认证、权限、CSRF 等安全契约（属于安全评估范围，不在本篇展开）。

## 7. 后续方向

下一篇建议进入 **05 数据库访问纪律与性能卫生**。若优先做整改，建议先评审 04-D1（03刀D）范围与前端依赖，再按“稳定 message + 服务端日志”窄范围落地。
