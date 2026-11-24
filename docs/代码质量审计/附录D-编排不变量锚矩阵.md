# 附录D 高风险编排不变量锚矩阵

- **状态**：生效（随 06-测试稳健性与回归锚点 整改建立）
- **日期**：2026-11-24
- **来源**：[06-测试稳健性与回归锚点](./06-测试稳健性与回归锚点.md) §2.3 落地
- **用途**：重构/改动编排路径时对照本矩阵确认回归护栏在位；PR 涉及编排路径必须指明对应锚或新增。

## 1. 不变量 × 组件矩阵

| 不变量 | 组件 | 现有锚 | 状态 |
|---|---|---|---|
| **Context 取消传播** | eventbus 消费者 | `TestEventBusBaseCtxCancelPropagation`/`TestEventBusConsumerTimeoutCtx` | ✓ |
| | seeding worker 族 | `TestEngineRootCtxCancelsWorkers` | ✓ |
| | stats 同步 | `TestStatsSyncBaseCtxCancelled` | ✓ |
| | reseed 任务取消 | `reseed/task_cancel_test.go` | ✓ |
| | publish 执行链 | — | 渐进（executor 全栈取消锚成本高·终态机已有兜底） |
| **Stop/Close 幂等** | eventbus | `TestEventBusCloseIdempotent` | ✓ |
| | eventbus 排空零丢失 | `TestEventBusCloseDrainsBuffer` | ✓ |
| | seeding Engine | `TestEngineStopIdempotent`（刀C 补） | ✓ |
| | watcher | `TestWatcher_StopIdempotent`（刀C 补） | ✓ |
| | batch fetch graceful | §59.495 语义在位（stopRequested+baseCtx 双检查点） | 语义锚 |
| **重复事件/任务** | eventbus 重复投递 | `TestEventBusDuplicatePublishDelivered`（刀C 补·at-least-once 语义） | ✓ |
| | reseed 误fetch 防护 | `reseed/misfetch_guard_test.go` | ✓ |
| | publish dedup 三路 | `execute_upload_anchor_test.go`（dedupStatusSets） | ✓ |
| **部分失败** | 发布结果落库失败 | createResultLogged Warn 九字段（03 刀A） | 日志锚 |
| | existing 哨兵终态 | `existingStatusSentinel`（executor_upload） | ✓ |
| **重试** | 上传超时重试判据 | `executor_timeout_test.go`（isUploadTimeoutErr） | ✓ |
| **关键状态机** | publish 四终态集合 | `publish_status_sets_test.go`（常量锁定+裸清单扫描锚） | ✓ |
| | reseed 匹配门十锚 | `match_gate_anchor_test.go` | ✓ |
| | publish 执行分支十锚 | `execute_branch_anchor_test.go` | ✓ |
| | seeding transferring 中断回退 | Start 回滚逻辑（§59.495·周期自愈） | 语义锚（B 类自愈·05 篇 §2.4.1） |

## 2. 维护规则

1. 新增编排路径（goroutine/事件/任务/状态机）必须在本矩阵登记锚。
2. 无锚可写时降级为「语义锚」并注明自愈通道（参考 05 篇 §2.4.1 分类的先例）。
3. 锚命名：`Test<组件>_<不变量>`（如 TestEventBus_CloseIdempotent 族）。
4. 本矩阵与 00-总索引同维护：对应篇章整改完成时更新状态列。
