# 01 Context 传播与取消语义

- **状态**：已完成（刀1-5 全落地·见文末整改落地记录）
- **日期**：2026-11-24（精读）/ 2026-11-24（整改收官）
- **基线**：精读 `66bb3cf4` → 整改落点 `c9763248`（v0.0.1585·生产实证）
- **审计范围**：`internal/` 生产代码（排除 `_test.go`、`tools/`、`cmd/`）中的 `context.Background()` 使用。
- **审计方法**：全局扫描 `context.Background()` 调用点；按“根上下文/取消后收尾/后台任务/请求链路/副作用”分类；对高风险点定点阅读上下文代码。

> 本文为代码质量定点精读结论，不是安全评估结论。行号与统计为基线快照，实施前请重新定位。

## 1. 结论摘要

- `internal/` 非测试生产代码共扫描到 **114 处** `context.Background()`。
- `context.Background()` 本身不是问题；问题在于**在错误的层级使用它**，导致取消链断裂、服务停机不可控、错误静默或超时边界不清。
- 本次定点精读识别出 4 类主要风险：后台任务脱离服务生命周期、消费者无超时边界、Client reload 静默失败、长生命周期 worker 以 Background 自建局部根上下文。

## 2. 分类总览

| 分类 | 判断 | 处理原则 |
|---|---|---|
| 进程根上下文 / 边界守卫 | 合理 | 仅在入口、默认兜底、明确边界使用，并加注释 |
| 父 ctx 已取消后的收尾 | 合理 | 使用 `Background()+短超时`，用于落库/清理，需注释原因 |
| 后台任务脱离 HTTP 请求但应受服务停机管理 | 需改进 | 派生自服务级 `appCtx` 或任务管理器 ctx |
| 副作用调用失败静默 | 需改进 | 至少记录日志；强一致场景应返回错误或重试 |
| 请求链路内无说明地使用 Background | 需评估 | 若应随请求取消则用 `r.Context()`；若必须脱离请求，写明原因与超时 |

## 3. 重点发现

### 3.1 后台任务脱离服务生命周期（P1）

**证据**：

| 位置 | 现象 | 风险 |
|---|---|---|
| `internal/api/publish_torrents_handler_batch_fetch.go:138` | `runBatchFetch` 进入后台后 `ctx := context.Background()` | 批量抓取无法接收服务停机取消；下游 DB/检测/抓取调用也无法继承统一取消语义 |
| `internal/api/publish_torrents_handler_torrents.go:223` | `startBackgroundQuery` 使用 `context.WithTimeout(context.Background(), 10*time.Minute)` | 脱离 HTTP 请求是合理的，但同时脱离服务停机控制 |
| `internal/api/manual_forward_handler.go:535` | 截图后台任务使用 `context.WithTimeout(context.Background(), 15*time.Minute)` | 任务长时间运行，停机或重启时缺少统一取消入口 |
| `internal/site/stats_sync.go:61` | 站点统计同步 goroutine 使用 `SyncAllSites(context.Background())` | 后台同步不可被服务停机统一收敛 |

**说明**：`runBatchFetch` 已有 `stopRequested`/`active` 手动状态，能实现“优雅不再开始新颗”，但它不是标准 context 取消链，不能自然传递到下游接口。

**建议**：
- 为后台任务引入 `task ctx`：由服务级 `appCtx` 派生，带 timeout/cancel。
- 任务循环同时检查业务停止标志和 `ctx.Done()`。
- 对“必须跑完当前颗再停”的任务，保留 graceful stop 语义，但仍应接入服务停机信号。

### 3.2 事件消费者缺少超时边界（P1）

**证据**：

- `internal/pusher/eventbus.go:72`：`consumer.OnPushed(context.Background(), ev)`。

**风险**：

- 单个消费者阻塞会占用处理 goroutine。
- 消费者内部若执行 DB/外部调用，无法感知服务停机。
- EventBus 有 panic recover，但没有超时/取消保护。

**建议**：

- 将 `Background()` 改为 `context.WithTimeout(serviceCtx, consumerTimeout)`。
- 在 Consumer 契约中明确：实现必须尊重 ctx，超时或取消应快速返回。
- 为慢消费者增加日志字段（consumer 名称、事件类型、耗时）。

### 3.3 Client reload 静默失败（P1）

**证据**：

- `internal/api/client_handler.go:329`：创建后 `_ = h.clientMgr.Reload(context.Background())`
- `internal/api/client_handler.go:481`：更新后 `_ = h.clientMgr.ReloadClient(context.Background(), client.ID)`
- `internal/api/client_handler.go:546`：删除后 `_ = h.clientMgr.Reload(context.Background())`

**风险**：

- API 可能返回成功，但运行时下载器管理器未刷新，造成“界面/DB 已变更、实际运行态未生效”的隐性不一致。
- 出现问题时缺少日志证据。

**建议**：

- 最低要求：捕获错误并记录 `Warn/Error` 日志，包含 `client_id`、`client_name`、`op`。
- 若 reload 是业务强一致要求，评估将失败纳入响应；若允许最终一致，应明确日志和补偿策略。

### 3.4 长生命周期 worker 自建局部根上下文（P2）

**证据**：

- `internal/seeding/engine.go:285`：`refreshCtx, cancel := context.WithCancel(context.Background())`，cancel 存入 engine。
- `internal/seeding/engine_patrol.go:395`：`go e.transferRecord(context.Background(), recCopy, key)`。

**风险**：

- worker 生命周期由组件内部管理，不易纳入统一服务停机。
- 后台 transfer 任务脱离 patrol/engine 的取消链，停机时可能残留。

**建议**：

- Engine 引入统一 `rootCtx`（由 `Run(ctx)` 或构造/启动入口注入）。
- 子 goroutine 统一从 `rootCtx` 派生；需要独立超时的任务使用 `WithTimeout(rootCtx, ...)`。
- 停机时先 cancel rootCtx，再等待 wg。

### 3.5 合理使用示例（保留）

以下模式不建议盲目替换：

| 位置 | 模式 | 为什么合理 |
|---|---|---|
| `internal/reseed/engine_match.go:111` | 父任务 ctx 可能已取消，最终状态回写使用 `Background()+5s` | 避免任务取消导致状态落库失败；这是明确的“收尾 ctx” |
| `internal/bdinfo/bdinfo.go:138,218` | 入参 ctx 为 nil 时兜底 `Background()` | 边界防御，避免 nil ctx panic |
| `internal/middleware/ssrf.go:13` | 基于 `Background()` 创建 timeout ctx | 作为工具函数可接受；调用方应清楚其不继承上游取消 |

## 4. 整改建议

### 4.1 统一约定

1. **Request ctx**：HTTP 请求链路默认使用 `r.Context()`。
2. **Task ctx**：后台任务使用 `context.WithTimeout(serviceCtx, timeout)` 或任务管理器派生 ctx。
3. **Service ctx**：由进程启动/服务管理器持有，优雅停机时 cancel。
4. **Finalize ctx**：父 ctx 取消后仍需落库/清理时，使用 `context.WithTimeout(context.Background(), shortTimeout)`，并写注释说明。

### 4.2 优先整改清单

| 优先级 | 改动 | 预期收益 | 风险 |
|---|---|---|---|
| P1 | `client_handler` reload 错误日志化 | 消除静默不一致 | 低 |
| P1 | `eventbus` consumer 增加 timeout ctx | 防止慢消费者拖垮事件处理 | 低中 |
| P1 | batch/manual/stats 后台任务接入 service ctx | 提升停机可控性 | 中 |
| P2 | seeding/rss 等 engine 统一 root ctx | 收敛长生命周期 worker | 中 |
| P2 | 补充 ctx 使用规范（开发文档/评审清单） | 防止回归 | 低 |

## 5. 验证建议

- **静态检查**：新增/修改 `context.Background()` 时，要求说明归类（root/task/finalize）。
- **单测/集成测试**：为后台任务补“取消后退出/不再启动新工作”测试。
- **停机验证**：在本地或测试环境验证服务停机时，批量抓取、统计同步、eventbus 消费者能在预期时间内退出。
- **日志验证**：reload 失败、consumer 超时、任务取消应有结构化日志。

## 6. 本次未覆盖

- 未逐条审读全部 114 处 `Background()`；本篇聚焦高风险代表点。
- 未对 cmd/tools 入口中的 `Background()` 做治理建议，它们通常是进程根上下文。
- 未进行 race/压力/停机实测；结论基于静态精读。

## 7. 后续方向

下一篇建议进入 **02 后台任务生命周期与优雅停机**，将本篇发现的后台任务模式与 `time.Sleep` 轮询、`stopRequested`、worker 退出协作一起审计。

## 8. 整改落地记录（2026-11-24·§59.562）

| 发现 | 刀 | 版本 | 落地 |
|---|---|---|---|
| §3.3 client reload 静默失败 | 刀1 | v0.0.1582 | 三处 Warn 日志化（op/client_id/client_name） |
| §4.1/4.2 规范缺失 | 刀5 | v0.0.1582 | 附录A-Context使用规范（四类 ctx+六规则+评审清单）+AGENTS 短指针 |
| §3.2 eventbus consumer 无超时 | 刀2 | v0.0.1583 | baseCtx 注入+60s 命名常量+超时五字段 Warn·锚三枚 |
| §3.1 batch/manual/stats 脱离停机 | 刀3 | v0.0.1584 | NewRouter ctx 穿透+语义分型三型接线（循环 graceful/幂等/工件）·锚两枚 |
| §3.4 seeding 自建局部根 ctx | 刀4 | v0.0.1585 | rootCtx 单源+refreshCtx 双通道+transferRecord 派生巡检 ctx·锚两枚 |

生产实证：v0.0.1585 OTA 重启复验（版本/七端点 200/零 panic/停机收敛日志——
seeding engine stopped→started 2.7s 干净换血 21403 记录）。

**已知取舍（定向移交方向 2 跟踪）**：
- batch fetch 颗内 Background 有意保留（graceful 当前颗跑完）；更强停机保证
  需评估「当前颗可安全中断」边界，不可直接全换 ctx。
- 停机期 consumer timeout Warn 可能噪音（停机取消未响应≠真超时）——先观察
  日志上下文区分，不改代码。
- flush.go:249 / free_end.go:108 / EventBus Close() 零调用面——移交方向 2。
