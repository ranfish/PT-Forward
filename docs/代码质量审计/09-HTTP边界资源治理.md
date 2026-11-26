# 09 HTTP 边界资源治理

- **状态**：精读完成，整改建议待实施
- **日期**：2026-11-26
- **基线**：`678a61fe`（08 回归审核四修补齐·全案收官）
- **审计范围**：`internal/` 生产代码中所有发起外部 HTTP 调用的路径（24 处自建 `http.Client`、9 处裸 `http.NewRequest`、17 处无上限 `io.ReadAll(resp.Body)`、23 处裸 `resp.Body.Close()`），重点覆盖 `imagehost`/`httpclient`/`api`/`publish`/`adapter`/`cookiecloud`/`iyuu`/`notification`/`download` 等包；对照 `docs/31` 既有 HTTP 单点决策（§59.156 `ResolveSiteProxy`、§59.334 Session 复用、§59.367 代理接入）校正。
- **审计方法**：全库统计 client 创建方式、请求构造、响应体处置、状态码判定的分布；对每类"疑似绕过单点"逐处读实现，区分「真资源 bug」「口径漂移」「合理例外」三类；对最严重的一处（全局 `DefaultTransport` 替换）核对调用点与并发前提；所有结论对照 08 篇 §5/§6 与既有设计决策，避免与 01（Context）/03（错误）/07（超时数值）/08（适配器复用）重叠。

> 本文为代码质量定点精读结论，不是安全评估结论。行号/统计为基线快照，实施前请重新定位。

## 1. 结论摘要

- 08 篇 §5/§6 已把「HTTP 边界资源治理（`client.Do`/`Body.Close` 口径）」明确留作独立方向。本篇承接该方向，治理对象是**边界资源纪律**——client 实例从哪来、是否复用；响应体是否关闭并排空、读取是否有上限；请求怎么构造、状态码怎么判定。与前四篇正交：01 管 ctx 传什么、03 管错误怎么记、07 管超时数值、08 管站点方言复用；本篇管"连接/响应体/请求对象"的生命周期与口径。
- 公共单点**已经存在且质量良好**：`httpclient.NewSiteHTTPClient`（工厂·限流/重试/熔断/WAF 完整层序）、`ReadBody`/`DrainBody`（10 MiB 读上限+排空）、`BrowserUserAgent`/`SetBrowserHeaders`（UA 单点）。问题不在"没有单点"，而在**绕过单点的 24 处自建 client、9 处裸 `NewRequest`、17 处无上限 `ReadAll`**。
- 最严重的一处是真 bug（P1）：`imagehost.Manager.withProxy` 临时替换**进程级** `http.DefaultTransport`，注释声称"串行无并发冲突"但无机制保证——锁只覆盖读 transport，替换与恢复都在锁外；并发发布/批量转存可交叉污染全局 transport，把别的 HTTP 请求流量导到图床代理。
- 第二类真问题（P1）是响应体资源纪律破裂：17 处 `io.ReadAll(resp.Body)` 完全无上限（含 `download_handler.go` 的远端可控 `.torrent`、`publish/executor.go` 已用公共 client 却只 `Close` 不排空且忽略 err），与公共 `ReadBody` 上限口径并存。
- 第三类（P2）是同一后端两套 client / 状态码四套判定 / UA 硬编码散落，属口径漂移，非单点缺失。
- **明确不做**：站点表单/正则/业务状态码语义（08 范畴）、超时数值调整（07 范畴）、ctx 传播链改造（01 范畴）、piggo 手搓 multipart 方言（站方 cURL `--form` 契约的已知例外）。存量 24 处 client 不做一次性全清，采用「新增即拦+存量登记+渐进收敛」。

## 2. 重点发现

### 2.1 全局 `http.DefaultTransport` 临时替换——进程级并发污染（P1，真 bug）

**证据**：

`internal/imagehost/manager.go:47-60`：

```go
// withProxy 临时替换 DefaultTransport，调用后恢复。
// 图床上传/转存是串行的（for 循环逐张），不存在并发冲突。
func (m *Manager) withProxy(fn func() error) error {
	m.mu.RLock()
	transport := m.proxyTransport
	m.mu.RUnlock()
	if transport == nil {
		return fn()
	}
	original := http.DefaultTransport
	http.DefaultTransport = transport        // 锁外改全局变量
	defer func() { http.DefaultTransport = original }()
	return fn()
}
```

- `withProxy` 共有 6 处调用（`:107`/`:124`/`:144`/`:161`/`:181`/`:214`），分别来自 `Upload`/`Rehost`/`UploadWithHost`/`RehostWithFallback`/`HealthCheckAll`/健康检查批量。`Manager` 的 `mu` 只保护 hosts map 和读 `proxyTransport`（`:34`/`:63`/`:69`/`:79` 等），**不串行化整个 `withProxy(fn)` 调用**。
- 注释"图床上传/转存是串行的（for 循环逐张）"只保证**单次**批量内部逐张串行；多个发布任务、或发布与 `HealthCheckAll` 并行时，两个 `withProxy` 可同时在锁外改写 `http.DefaultTransport`：A 记为 original=默认、B 记为 original=图床代理，B 恢复时把全局永久留在图床代理（顺序依赖），窗口内其它使用 `DefaultTransport` 的 client 流量被误 routing。
- `SetProxy` 每次 `new http.Transport`（`:41`）无 `CloseIdleConnections`、无连接池复用，代理切换后旧 transport 的连接资源悬挂。

**风险**：这是进程级共享状态污染，影响所有走默认 transport 的 HTTP 调用（24 处自建 client 中未显式指定 Transport 的多数），不是图床局部问题。触发条件=并发发布（生产常态）叠加图床代理启用。严重性高于普通口径漂移。

**定性**：真 bug。修复方向是把代理 transport 显式注入各 host client（随 client 生命周期走，零全局污染），而非临时替换全局默认——后者即便全程持写锁也只是把并发降级为串行，仍留下"窗口内全局被改"的语义隐患。

### 2.2 响应体处置：无上限 `ReadAll` 与"接了单点仍破裂"（P1）

**证据**：

生产代码有 17 处 `io.ReadAll(resp.Body)` 完全无上限（无 `LimitReader` 包裹），分布在：

| 位置 | 场景 | 风险点 |
|---|---|---|
| `api/download_handler.go:363` | 下载 `.torrent`（`req.URL` 来自请求体） | 大小完全由远端控制 + 无 ctx + 裸 `!=200` |
| `publish/executor.go:625` | 已用 `NewSiteHTTPClient`（:614）+ `ResolveSiteProxy`（:613） | **只 `Close` 不 `DrainBody`（:624）·忽略 err（:625 的 `_`）**——连接不可复用、读失败静默 |
| `adapter/adapter_nexusphp_userstats.go:82/142/212/515/741` | 用户统计抓取（5 处） | 站点响应体可控，长连接内存放大 |
| `adapter/adapter_nexusphp_userstats_parse.go:68/101` | 用户统计解析 | 同上 |
| `adapter/adapter_unit3d.go:843/893` | unit3d 详情 | 同上 |
| `adapter/adapter_generic_stats.go:33/149`、`adapter/adapter_generic_probe.go:131` | 通用统计/探测 | 同上 |
| `adapter/adapter_mteam_upload.go:439`、`adapter/adapter_yemapt.go:730`、`adapter/adapter_nexusphp_edit.go:169` | 上传回执/编辑 | 同上 |

对照公共口径：`internal/httpclient/drain.go:9-38` 的 `ReadBody`（10 MiB 上限）/ `DrainBody`（1 MiB 排空）已被生产调用 86 / 12 处——说明单点可用且被广泛采用，上述 17 处是**绕过点**。`imagehost/host.go:49-55` 则是另一种破法：`io.Copy(io.Discard, resp.Body)` 排空无 `LimitReader`（无界读），紧接 `io.ReadAll(io.LimitReader(resp.Body, 50<<20))` 虽有读上限，但排空阶段已可被远端持续推流。

**风险**：无上限读取=远端可驱动的内存放大；只 `Close` 不排空=HTTP 连接无法复用（连接池失效、握手放大）；忽略读 err=数据截断静默。`publish/executor.go` 尤其典型——它已经正确取得公共 client，却因响应体处置破裂而抵消了单点的价值，证明本篇不是 07/08 的重复。

**定性**：真问题（资源后果），优先级 P1。`download_handler`/`publish/executor` 两处面向请求与发布主链，优先收编。

### 2.3 client 获取散落：24 处自建 vs 一个公共工厂（P2）

**证据**：

`internal/httpclient/client.go:62-166` 的 `NewSiteHTTPClient` 是唯一公共工厂（30s 默认超时、45s IdleConnTimeout、MaxIdleConnsPerHost=10、IPv4 优先、Proxy/WAF、限流+重试+熔断层序）。但生产代码另有 **24 处** `http.Client{`（严格口径，排除注释/测试），其中只有 1 处是工厂返回本身、1 处是全局单例（`adapter_rousi.go:453` `rousiPosterClient`），其余 22 处为各包/各函数自建：

- 每次新建（丢连接池复用）：`api/download_handler.go:352`、`api/cloud_fp_handler.go:103`、`api/iyuu_handler.go:47`、`api/site_handler_crud.go:676/870`、`imagehost/host.go:38`、`imagehost/agsvpt.go:35`、`imagehost/pixhost.go:34`、`cookiecloud/client.go:130`、`publish/image_check.go:99`、`publish/poster_fallback.go:69`、`adapter/adapter_generic.go:398`、`adapter/adapter_nexusphp_upload.go:325`、`adapter/adapter_piggo.go:361` 等。
- 结构体成员（生命周期随宿主，问题较小）：`cloudfp/service.go:34`、`client/qbittorrent/client.go:52`、`client/transmission/client.go:51`、`notification/service.go:35` 等。
- 合理例外（不应强行收编）：`api/update_handler.go:65` OTA 专用代理 client；CF 指纹栈 azuretls/chrome session（`httpclient/chrome_transport.go`）；站方 cURL `--form` dialect 所需 client。

**风险**：每次新建 client=每次重建传输层、丢连接池、超时/代理/UA 口径各自为政。这是"单点建了但被绕过"，与 07 篇 `GetIntDefault`（helper 建了零调用）同型——机制存在不等于接线完成。

**定性**：口径漂移（P2），非全量清零对象。按"新增即拦+存量登记+渐进收敛"治理。

### 2.4 请求构造与状态码判定散落（P2）

**证据**：

- **裸 `http.NewRequest`（无 Context）9 处**：`adapter/apihelper.go:46`、`api/iyuu_handler.go:421`、`api/site_handler_crud.go:684/784/813/841/862/909/944`；对照 `http.NewRequestWithContext` 188 处。这是 01 篇 ctx 纪律在**请求构造形态**上的残留（本篇只登记形态，不重述 ctx 语义）。
- **状态码 `== 200`/`!= 200` 裸字面量 9 处**（严格口径）：分布在 `imagehost`/`cookiecloud`/`download` 等包，与 `>=400`/`>=500`/`<400` 等其它判定口径并存，无统一 2xx 入口。扩展口径（含 `StatusOK` 常量、`>=400` 等变体）总量更大，但性质相同：判定口径不集中。
- **UA 硬编码**：`adapter/adapter_hdarea.go:133/330/563/670`、`adapter/adapter_hxpt.go:139/298` 等写死 `Mozilla/5.0 ... Chrome/124.0`，与公共 `httpclient/browser.go:17-49` 的 `BrowserUserAgent`/`SetBrowserHeaders` 单点并存（对照 `adapter_cola.go:128/187`、`adapter_piggo.go:339` 已调公共函数）。
- **同一后端两套 client**：IYUU 在 `iyuu/service.go:38`（60s、`Proxy:nil`、走 `ReadBody/DrainBody`）与 `api/iyuu_handler.go:47/421`（10s、手写 `MaxIdleConnsPerHost:2`、裸 `NewRequest`、手写 Token 头、裸 `!=OK`）各有一套——超时、连接池、状态判定、响应读取四处口径分裂。

**风险**：请求对象构造无单点 → 新增调用点各自造轮子；状态码口径分裂 → 边界错误分类不一致（部分 201/204 被误判为失败）；UA 硬编码 → 浏览器指纹漂移。属可维护性/一致性问题，非即时 bug。

**定性**：口径漂移（P2）。

### 2.5 正面结论：不应被误判为问题的部分

- `httpclient.NewSiteHTTPClient` + `ReadBody/DrainBody` + `BrowserUserAgent` 三套单点**健康且被广泛使用**（86/12 处响应体 helper），本篇不推翻、不重写。
- `rss/fetcher.go:32,64`、`site/stats_sync.go:181+`、`adapter/apihelper.go:31`、`client/qbittorrent/client.go:117-118`、`cloudfp/service.go:215-216` 等已正确成对 `ReadBody+DrainBody`——是正例。
- `adapter/adapter_piggo.go:757-784` 手搓 multipart boundary 有注释说明是站方 cURL `--form` 方言契约（数组字段形态），属已知方言例外，**不作为资源缺陷**——否则与 08 篇方言边界重叠。
- 结构体成员 client（`cloudfp`/`qbittorrent`/`transmission`/`notification` service）生命周期随宿主，非"每请求新建"，问题等级低。
- `http.Transport` 的 `MaxIdleConns`/`IdleConnTimeout` 数值归属 07 篇调参清单；本篇只管"从哪取、是否复用、响应体怎么处置"，不碰数值。

## 3. 整改建议

> 原则：新增即拦、存量登记、渐进收敛；不做一次性全清，不碰站点业务语义。

### 刀1 · 消除全局 `DefaultTransport` 替换（P1，真 bug）

- `imagehost.Manager` 改为持有代理 transport 并**显式注入**各 host 的 client（或让 host 的 client 从 `NewSiteHTTPClient` 取、把 proxy 作为入参），彻底移除 `withProxy` 对 `http.DefaultTransport` 的临时替换。
- 代理 transport 的创建/关闭纳入 `Manager` 生命周期（`SetProxy` 换 transport 时对旧值 `CloseIdleConnections`）。
- 锚：离线驱动并发测试（§59.338 真实组件法）——并发发起图床上传与普通 HTTP，断言 `http.DefaultTransport` 全程未被改写、普通请求未走图床代理；`-race` 通过。

### 刀2 · 响应体读上限与排空收编（P1）

- 17 处无上限 `io.ReadAll(resp.Body)` 收编到 `httpclient.ReadBody`（含 err 不忽略）；`imagehost/host.go:49` 的 `io.Copy(io.Discard, ...)` 套 `LimitReader`。
- `publish/executor.go:624-625` 补 `DrainBody`、保留 err；`download_handler.go:363` 收编并接 `r.Context()`。
- 锚：对每类改动的响应体路径断言"超限被截+err 可见+连接可复用"（`ReadBody` 既有锚可复用）。

### 刀3 · 请求构造单点登记（P2）

- 9 处裸 `http.NewRequest` 改 `NewRequestWithContext`（ctx 来源按 01 篇四类约定）；UA 硬编码改 `SetBrowserHeaders`。
- IYUU 双 client 合并为一处公共 client（超时口径以 07 篇预算表为准，本篇不自行定数值）。
- 不强行合并所有 24 处 client：在附录登记合理例外清单（CF 指纹栈/OTA/图床/第三方 API 专用），其余新增点一律走公共工厂。

### 刀4 · 落 HTTP 边界规约附录 + 静态锚

- 新建附录（类比附录 A–G → 附录 H），写清三条铁律：① client 只从公共工厂取（例外登记）② 响应体 `ReadBody`+`DrainBody` 成对或等效且 err 可见 ③ 请求用 `NewRequestWithContext`、2xx 判定走单一 helper。
- 静态锚参照 `TestNoBarePublishStatusList` 做法：CI 拦"新增裸 `&http.Client{`""裸 `io.ReadAll(resp.Body)` 无 `LimitReader`""`http.NewRequest(` 无 WithContext""新增裸 `==200`"——白名单例外文件清单 + 新增即拦，存量不要求一次清零。
- 维护触发表：新增外部 HTTP 调用、切换代理、加图床、接第三方 API 时更新附录清单。

### 明确不做（需专项授权）

- 站点表单/正则/业务状态码语义（08 范畴）；超时数值与重试退避数值（07 范畴）；ctx 传播链大改（01 范畴）。
- 一次性清零 24 处 client / 17 处 `ReadAll`（跨包广、行为风险高，按渐进收敛）。

## 4. 验证建议

- 包级：`go test -count=1 ./internal/httpclient/... ./internal/imagehost/... ./internal/api/... ./internal/publish/... ./internal/iyuu/... ./internal/cookiecloud/... ./internal/adapter/...`
- 严检：`go vet` 上述包；刀1 用 `-race` + 离线驱动并发实验（§59.338）。
- 静态锚自检（刀4 落地后）：对四类禁止形态做阴性对照——临时新增一处裸 client/裸 `ReadAll`/裸 `NewRequest`/裸 `==200`，确认 CI/测试能拦；删除后恢复。
- 灵魂四问：nil（client/transport 为 nil 时的分支）、边界（空响应体/超限/ctx 取消）、回归（上述包全绿+vet 零）、前端（本篇不涉 `web/`，无需构建）。

## 5. 本次未覆盖

- 各站点 HTML 抓取的正则/字段语义、发布表单方言（08 范畴）。
- 超时/重试/退避的具体数值合理性（07 范畴）；限流/熔断算法本身（httpclient 包内既有实现，非本篇治理对象）。
- ctx 在各调用链的传播语义（01 范畴）；错误日志字段/可观测性（03 范畴）。
- TLS 指纹/azuretls 栈 internals（基础设施，见 `ptf-infra-notes`）。
- 生产环境实态（本篇为静态精读，未做生产流量/代理切换实测）。

## 6. 后续方向

- 刀1 落地后，观察是否出现更多"全局可变状态临时替换"形态（类比 08 篇 §2.3 的方法论：函数头/注释声明的前提要用攻击实验验证，而非照抄）。
- 静态锚上线后，跟踪存量 24 处 client / 17 处 `ReadAll` 的收敛进度，第三例同类绕过出现时评估是否升级为强制单点。
- **可复现的数据竞争候选（建议列为 10 篇）**：`internal/filter/engine.go:20-44` 的 `compileRegex` 读侧 `RLock` 释放后裸写 map（淘汰循环 `:37-43` 与写入 `:43` 无锁），与 §59.186 已在 `titleparser/dict_loader.go` 修掉的同型问题漏网。基线复核时以 8 goroutine 并发压测实证触发 `fatal error: concurrent map read and map write`（栈顶 `engine.go:22`）——这是可复现真崩溃，优先级高于本篇多数 P2 项，建议单独成篇（方向名：共享状态与缓存并发治理），与 02（停机编排）/06（race 基建）划清边界后治理。
