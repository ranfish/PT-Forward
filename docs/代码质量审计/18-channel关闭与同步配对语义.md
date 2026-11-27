# 18 channel 关闭责任与 goroutine 同步配对语义

- **状态**：已完成（三刀 v0.0.1671 落地·回归审核遗物处置）
- **日期**：2026-11-27
- **基线**：`5f6d9cfa`（17 全案三刀收官·审计系列 01-17十七篇全毕）
- **审计范围**：生产代码中 channel 的「谁关闭、谁发送」责任边界（`close(send)` 与并发/后续发送是否配对），以及 goroutine 生命周期同步原语（`WaitGroup.Add/Wait` 是否成对）——两处确凿缺陷：WebSocket `client.send` 的 send-on-closed 进程级 panic（高危）与 `AuthManager.persistWg` 零 `Add` 导致 `Stop()` 空等（中危）。
- **审计方法**：全库盘点后据实立项（非预设），避开 01-17 覆盖维度；先派四路并行盘点候选面（文件路径/临时文件、channel/goroutine 语义、缓存 TTL/失效、错误链/类型断言），逐一评估正交性与可复现性，用户定题 A 后精读两处确凿点 → 用包内临时测试直接触达生产类型离线复现（取证后删除、工作区干净）→ 核对停机链调用点与项目既有正例。
- **证据文件**：`evidence/tool-versions.txt`（工具版本+基线+采集时间）、`evidence/channel-close-pairing-repro.txt`（两处源码摘录、离线复现真实输出、正交性核对、健康面核验）。

> 本文为代码质量定点精读结论，不是安全评估结论。行号为基线快照，实施前请重新定位。复现均为本地离线驱动，未请求真实站点、未接触用户数据。

## 1. 结论摘要

- 本篇是全库盘点后新立的独立方向。四路候选中（文件路径/临时文件、channel/goroutine 语义、缓存 TTL/失效、错误链/类型断言），只有 channel 面同时满足「未被 01-17 覆盖 + 可纯本地离线复现 + 确凿功能缺陷」，且头部缺陷为高危（进程级 panic）。用户定题 A 后聚焦两处确凿点。
- **V-01（高危）**：`internal/api/ws_handler.go` 的 `client.send` 有三处 `close`（stop 分支、unregister 分支、stale 驱逐满时直闭），而 `readPump`/`ServeHTTP` 的发送方与之无生命周期协调。stale 驱逐路径只关 `send` 不关 `conn`，读泵仍存活；对端一帧 `ping` 即触发 `panic: send on closed channel`，`readPump` 无 recover → panic 冒泡杀进程。已鉴权 WebSocket 客户端可稳定触发，等价于远程 DoS。离线复现（复用生产 Hub 驱逐逻辑）捕获该 panic。
- **V-02（中危）**：`internal/auth/auth.go` 的 `persistWg` 只有声明（`:152`）与 `Wait()`（`:260`），**零 `Add(1)`**；`Stop()` 空等立即返回，最后一批 refresh token 落库脱离停机生命周期（`main.go:228/844` 均调 `Stop()`）。离线复现：阻塞 `SettingStore.Set` 后 `Stop()` 在落库未完成时已返回。
- **正面结论**：全库 59 处 `make(chan)`、26 处 `close`、97 处 `select`、22 处 `WaitGroup` 中绝大多数正确；项目已有两个权威正例（`sse_log_handler.go` 的 send/close 同锁、`pusher/eventbus.go` 的 `closed` 位守卫——§59.563/审计 02 §3.1），证明作者已认知此模式，V-01/V-02 是后续新增漏接。
- **盘点另三面（诚实标注不立项）**：错误链/类型断言面经 1093 文件核验健康；缓存 TTL 面单位/方向/边界全健康（唯一候选 `discountCache` key 维度遗漏属独立方向，未并入本篇）；临时文件生命周期面仅一处低危泄漏（`screenshot.go` 全帧失败时 defer 清理死代码），不单独立篇。
- **明确不做**：请求真实站点、碰靶机、改业务语义；本篇只做精读与整改建议。

## 2. 重点发现

### V-01 · WebSocket `client.send` 的 send-on-closed 进程级 panic（高危 · channel 关闭责任）

- **级别**：高危（技术档 `AV:N/AC:L/PR:L/UI:N/S:U/C:N/I:N/A:H`，约 7.5；阅读档「高危」）。已鉴权客户端可远程触发进程级 panic；无 recover（崩溃即进程退出，非单连接断开）。
- **证据**（`internal/api/ws_handler.go`，基线 `5f6d9cfa`）：

```go
// 关闭点三处——全部由 Hub（owner）关闭 client.send：
// :92   stop 分支        close(client.send) + conn.Close()   ← 配对完整
// :108  unregister 分支   close(client.send)，不关 conn       ← 配对缺失
// :132  stale 满时直驱   close(c.send)，不关 conn            ← 配对缺失
// 发送点四处——与 close 无协调：
// :271  ServeHTTP        client.send <- NewWSMessage("server.connected", ...)  ← 裸阻塞发送
// :311  readPump/ping    select { case c.send <- NewWSMessage("pong", nil): default: }
// :321  readPump/sub     select { case c.send <- ...: default: }
// :331  readPump/unsub   select { case c.send <- ...: default: }
```

  Go 语义：`select` 的 `default` 只防「缓冲满时阻塞」，**不防向已关闭 channel 发送的 panic**。`readPump`（`:276-282`）与 `writePump`（`:338-354`）均无 recover。
- **影响**：stale 驱逐路径（`:126-135`）是可达主线——send 缓冲（容量 64）被填满即命中 `Broadcast` 的 `default` 加入 stale → `unregister` → `close(client.send)` 但 conn 未关 → 读泵仍阻塞在 `ReadJSON` → 对端一帧 `ping` 走 `:311` → `panic: send on closed channel` → 冒泡杀进程。`:271` 的裸发送另有两种失控：stop/unregister 先关 send 则同样 panic；send 已满且 pump 退出则永久阻塞。
- **复现**（包内临时测试复用生产 `Hub.Run` 的 stale 驱逐 + close 逻辑与 ping 发送形态，见证据文件 A 段）：

```
驱逐完成 clients=0（send 已由 Hub 关闭）
PANIC 捕获: send on closed channel
--- PASS: TestRepro18_WSSendOnClosedPanic (0.00s)
```

- **整改**：统一关闭责任——channel 只由唯一 owner 关闭，且关闭前先终止所有发送方。建议给 `Client` 加 `done chan struct{}` + `sync.Once`，`readPump`/`ServeHTTP` 所有发送改为 `select { case c.send <- msg: case <-c.done: }`，由 `writePump` 退出后统一 `close(c.send)`（或从不动 `close(send)`、改关 `done` 让接收方退出）；删 Hub 三处 `close(client.send)`，stale 驱逐改为关 conn + 等 pump 退出；`:271` 裸发送改带 `done` 分支。补回归锚：stale 驱逐后再发 ping 不得 panic（端到端），静态锚扫描 `close(x.send)` 与 `x.send <-` 的 owner 配对。阴性对照：恢复 `close(send)` 而不改发送方 → 锚点名 FAIL。

### V-02 · `AuthManager.persistWg` 零 Add → `Stop()` 空等（中危 · 同步配对）

- **级别**：中危（技术档 `AV:L/AC:L/PR:L/UI:N/S:U/C:N/I:L/A:L`，约 4.7；阅读档「中危」）。停机时最后一批 refresh token 可能未落库即进程退出，用户被迫重新登录；非数据破坏、非越权。
- **证据**（`internal/auth/auth.go`，基线 `5f6d9cfa`）：

```go
:152  persistWg sync.WaitGroup                 // 声明
:234  go mgr.persistLoop()                      // 没有 persistWg.Add(1)
:255  func (m *AuthManager) Stop() {
          m.stopOnce.Do(func() { close(m.stopCh) })
          m.loginLimiter.stop()
          m.persistWg.Wait()                    // :260 计数恒 0 → 空操作
      }
:263  func (m *AuthManager) persistLoop() {
          select {
          case <-m.stopCh:
              m.persistRefreshTokens()          // 最后一次落库
              return
          ... }
      }
```

  全库 `grep persistWg` 仅命中 `:152` 与 `:260` 两行，`Add` 为零。停机链：`cmd/pt-forward/main.go:228 defer authManager.Stop()` + `:844` 链内 `Stop()`。旁证：`auth_test.go:475` 因缺少同步原语只能用 `store.waitFor("refresh_tokens", 2*time.Second)` 轮询等待。
- **影响**：`Stop()` 的语义承诺是「等待持久化循环收尾」，实际立即返回。停机序列中 `persistLoop` 收到 `stopCh` 后的最后一次 `persistRefreshTokens()` 变成脱离生命周期的异步写——`main.go` 后续退出若抢先完成，token 持久化丢失。
- **复现**（包内临时测试，阻塞 `SettingStore.Set` 模拟未完成落库，见证据文件 B 段）：

```
persistLoop 已进入 Set（持久化尚未完成）
Stop() 在 Set 仍阻塞时已返回（persistWg.Wait() 空等实证）
--- PASS: TestRepro18_StopEmptyWait (0.50s)
```

- **整改**：二选一——① 在 `go mgr.persistLoop()` 前 `persistWg.Add(1)`，`persistLoop` 入口 `defer persistWg.Done()`，`Stop()` 保持 `Wait()`；② 用 `done := make(chan struct{})`，`persistLoop` 退出时 `close(done)`，`Stop()` 等 `<-done`。补回归锚：`Stop()` 在 `Set` 阻塞期间不得返回（用阻塞 SettingStore），阴性对照：删 `Add(1)` → 锚 FAIL。同步把 `auth_test.go:475` 的轮询改为 `Stop()` 同步等待（消除 flake 妥协）。

### F-01 · `write_queue` 理论窗口（低危 · 诚实标注，不入主线）

- `internal/db/write_queue.go:58-96` 的 `Stop()` close `q.ch` 与 `Execute()` 发送确有 send-on-closed 与双重 close 的理论窗口；但全库 `Execute()` 仅本文件测试使用，`cmd/pt-forward` 只做 `Start/Stop`——**无生产调用方**，当前不构成功能缺陷。登记为同族观察点，若后续接线生产则须同步套用 V-01 的 owner 规约。
- `cmd/pt-forward/main.go:430-459` 的 `freezeCh` 用 `for range` 且从不 close（停机后 goroutine 不退出）——进程退出即终止，归 02 停机语义，本篇不单列。

### 正面结论：经核验健康、不主张的维度与正例

- **项目正例（证明已认知）**：`internal/api/sse_log_handler.go:43-63`（`Unsubscribe` 持写锁 `delete`+`close`，`Broadcast` 持读锁发送——send/close 同锁互斥）；`internal/pusher/eventbus.go:63-108`（`closed` 位 + RLock/Lock 幂等 `Close`，§59.563 刀B 已修 send-on-closed 陷阱，审计 02 §3.1 有案）。V-01/V-02 是后续新增漏接该模式。
- **healthy 面**：`smart_dial.go:103-144`（resultCh 容量=生产者数、`wg.Done` 后才 close）；`orphan/recovery.go:770-918`（resultCh 从不 close，allDone 在 wg 后关闸，发送带 ctx 分支）；`notification/service.go:76-113`、`artifact_generator.go:157-169`、`adapter/errors.go:116-130`、`fetch_postprocess.go:229-242`（缓冲=生产者数，一对一发送）；多 close 处均有单一 owner 保护。
- **盘点另三面（全部经核验，不并入本篇）**：
  - 错误链/类型断言面：1093 文件中 `errors.Is`/`==` 混用仅作用于裸 GORM 错误、`fmt.Errorf` 包装全用 `%w`、`AppError.Unwrap` 与哨兵链完整、类型断言均在受控数据源、recover 语义正确——无确凿缺陷。
  - 缓存 TTL/失效面：单位/方向/`>=` 边界/命中刷新/零时间等全健康；唯一候选 `seeding/flush.go` 的 `discountCache` 裸 `torrentID` key 遗漏站点维度（中危·可离线复现）属独立方向，用户已定题 A，本篇不并入、登记为后续候选。
  - 临时文件生命周期面：`screenshot.go:116-143` 全帧失败时 defer 闭包捕获的 `err` 恒 nil、`RemoveAll` 成死代码导致 `/tmp/pt-screenshot-*` 泄漏（低危）——头部级别偏低，登记为后续候选，不单独立篇。

## 3. 整改建议

- **刀1（V-01）**：WS channel owner 单一化——`Client` 加 `done + sync.Once`，所有 `c.send <-` 改带 `<-c.done` 分支，Hub 三处 `close(client.send)` 收口（stale 路径改为关 conn + 等 pump 退出），`:271` 裸发送补 done 分支；补端到端锚（stale 驱逐后 ping 不 panic）+ 静态 owner 配对锚。
- **刀2（V-02）**：`persistWg.Add(1)` + `defer Done()`（或 done channel 收口），`Stop()` 真等待；同步化 `auth_test.go:475` 轮询；补阻塞 Set 锚（Stop 不得提前返回）。
- **刀3（F-01 + 静态锚）**：`write_queue` 若保持无生产调用则仅登记不修；静态锚扫描 `close(<expr>.send)` 与 `<same expr>.send <-` 的配对（`close` 点须有 `done`/`conn.Close`/pump join 之一配套），并覆盖 `WaitGroup` 声明处必须存在配对 `Add`；三阴性对照：恢复 WS 直接 close → 端到端锚 FAIL、删 `persistWg.Add(1)` → 阻塞 Set 锚 FAIL、注入新 `close(x.send)` 无配套 → 静态锚点名 FAIL。

## 4. 验证建议

- 三刀后跑全量 `go test ./...`、`go vet ./internal/...`、`gofmt -l`（`cmd/pt-forward`、`frontend`、`cmd/verify-pieces-hash` 三处基线既有失败与本篇无关）。
- 三阴性对照须全红（08 篇方法论：声明逐条可触发验证，杜绝虚拟安全感）；静态锚须做阴性对照并点名文件行号（沿用 13/14/15/16 补刀教训：防回归锚本身也要被锚）。
- WS 复现与锚用 `httptest` 或包内构造 `Hub`+`Client`，不请求真实站点、不接触用户数据；auth 锚用阻塞 `SettingStore`，不碰真实 DB。

## 5. 本次未覆盖

- 未请求真实站点 API、未碰靶机与生产；复现均为本地离线驱动。
- 未审计所有 channel/waitgroup 点的运行时并发行为（全库健康面已抽样核验，见 §2 正面结论）。
- 缓存 key 维度遗漏（`discountCache`）、临时文件清理死代码（`screenshot.go`）两候选登记为后续方向，本篇不主张。
- 未审计前端 WebSocket 消费层（本篇聚焦后端 channel 生命周期）。

## 6. 后续方向

- 按 §3 三刀整改后复审；复审须做三阴性对照与两处离线锚实跑。
- 后续候选（据实登记，不预设优先级）：① `discountCache` 跨站同号串数据（中危·已复核源码）；② `screenshot.go` 临时目录泄漏（低危）。
- 与本系列关系：本篇由四路盘点据实立项（非预设）；与 02（停机链是否在册）相邻但正交——本篇进入组件内部的 channel owner 与 wg 配对语义；与 10（map 并发锁）/12（限流器 channel 替换竞态）/09（HTTP 边界资源）均不重叠。
