# 附录A Context 使用规范

- **状态**：生效（随 01-Context传播与取消语义 整改建立）
- **日期**：2026-11-24
- **来源**：[01-Context传播与取消语义](./01-Context传播与取消语义.md) §4.1 统一约定落地
- **适用范围**：`internal/` 生产代码；新增/修改 `context.Background()` 时必须对照本规范归类

## 1. 四类 ctx 定义

| 类别 | 来源 | 用途 | 典型形态 |
|---|---|---|---|
| **Request ctx** | `r.Context()` | HTTP 请求链路内的一切操作 | handler → service → DB/RPC |
| **Task ctx** | `context.WithTimeout(serviceCtx, timeout)` | 脱离请求的后台任务（批量抓取/截图/同步） | handler 内 `go func` 派生 |
| **Service ctx** | 进程启动处创建（`main.go` 根 ctx），信号触发 cancel | 服务生命周期·优雅停机总闸 | `Start(ctx)` 注入组件 |
| **Finalize ctx** | `context.WithTimeout(context.Background(), 短超时)` | 父 ctx 已取消后的落库/清理收尾 | 必须**写注释**说明为何脱离取消链 |

## 2. 使用规则

1. **请求链路禁用裸 Background**：handler 请求路径内默认 `r.Context()`；确需脱离请求生命周期（如后台任务），必须满足两点——派生自 service ctx + 带显式超时，并写注释。
2. **后台任务必须接入停机**：脱离 HTTP 请求不等于脱离服务。后台任务从 service ctx 派生；"跑完当前颗再停"的 graceful 语义保留，但循环须同时检查业务停止标志与 `ctx.Done()`。
3. **消费者须有超时边界**：事件/回调式消费（EventBus consumer 等）使用 `WithTimeout(serviceCtx, consumerTimeout)`；Consumer 实现必须尊重 ctx，超时/取消快速返回。
4. **Finalize ctx 三要件**：`Background()` + 短超时 + 注释（说明父 ctx 可能已取消、此处必须完成落库/清理）。缺一即违规。
5. **nil 兜底可保留**：入参 ctx 为 nil 时兜底 `Background()` 属边界防御，可保留（参见 bdinfo 先例）。
6. **副作用不可静默**：reload/缓存刷新/注册类副作用调用，错误至少 Warn 日志（含 op/对象标识字段）；强一致场景纳入响应错误。

## 3. 超时值约定

- 超时一律**命名常量**（如 `eventBusConsumerTimeout`），禁止散落魔法值。
- 起步从宽（按消费面 DB 写为主 → 60s 级），凭观测证据调紧，不预拍。

## 4. 合理保留示例（勿盲目替换）

| 位置 | 模式 | 归类 |
|---|---|---|
| `internal/reseed/engine_match.go` | 父任务取消后状态回写 `Background()+5s` | Finalize ctx（有注释） |
| `internal/bdinfo/bdinfo.go` | 入参 nil 兜底 | 边界防御 |
| `internal/middleware/ssrf.go` | 工具函数自建 timeout ctx | 工具边界（调用方知悉） |

## 5. 评审清单（新增/修改 ctx 时逐条过）

- [ ] 此处 `context.Background()` 归哪类：root / task / finalize / 边界防御？
- [ ] 若是后台任务：是否派生自 service ctx？停机时能退出吗？
- [ ] 若有超时：是命名常量吗？
- [ ] 若是 finalize：有注释说明吗？超时够短吗？
- [ ] 副作用调用的错误去向：日志 / 响应 / 重试，三者必有其一？

## 6. 整改对照

| 审计发现 | 落地刀 | 状态 |
|---|---|---|
| client reload 静默失败（§3.3） | 刀1 v0.0.1582 | 已完成 |
| eventbus consumer 无超时（§3.2） | 刀2 v0.0.1583 | 已完成 |
| batch/manual/stats 脱离停机（§3.1） | 刀3 | 待实施 |
| seeding 自建局部根 ctx（§3.4） | 刀4 | 待实施 |
