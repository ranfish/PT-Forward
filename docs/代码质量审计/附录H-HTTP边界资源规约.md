# 附录H HTTP 边界资源规约

- **状态**：生效（随 09-HTTP边界资源治理 刀4 建立）
- **日期**：2026-11-26
- **来源**：[09-HTTP边界资源治理](./09-HTTP边界资源治理.md) §2/§3 刀4 落地（基线 `56da5729`·行号为快照·实施前重新定位）
- **用途**：发起任何外部 HTTP 调用之前先读本附录——「client 从哪取、响应体怎么处置、请求怎么构造、状态码怎么判」

## 三条铁律

### ① client 只从公共工厂取（例外登记）

新代码一律 `httpclient.NewSiteHTTPClient`——限流/重试/熔断/WAF 完整层序·连接池复用。

**合理例外**（附录登记·不强行收编）：
| 例外 | 位置 | 理由 |
|---|---|---|
| OTA 专用代理 client | `api/update_handler.go:65` | 独立代理链·不走站点层 |
| CF 指纹栈 azuretls/chrome session | `httpclient/chrome_transport.go` | 基础设施·TLS 指纹伪装 |
| 站方 cURL `--form` dialect | `adapter_piggo.go` | 方言契约（08 篇 §2.3） |
| 图床 host 自建 client | `imagehost/` 各 host | 09 刀1 transportSwitcher 已注入 |
| 下载器 RPC client | `client/qbittorrent/`、`client/transmission/` | 结构体成员·生命周期随宿主 |

### ② 响应体 `ReadBody` + `DrainBody` 成对或等效且 err 可见

- 读响应体：`httpclient.ReadBody(resp)`——10 MiB LimitReader 上限
- 不读响应体：`httpclient.DrainBody(resp)`——1 MiB 排空+5s 超时（§59.339）
- **禁止**裸 `io.ReadAll(resp.Body)`（无上限——远端可驱动内存放大）
- **禁止**只 `Close` 不排空（连接不可复用·握手放大）
- err **不可忽略**（数据截断静默——特殊情况显式注释标注语义）

### ③ 请求用 `NewRequestWithContext`·UA 用 `SetBrowserHeaders`/`BrowserUserAgent`

- `http.NewRequestWithContext(ctx, ...)`——ctx 来源按 01 篇四类约定
- **禁止**裸 `http.NewRequest`（请求取消不传播）
- UA：`httpclient.BrowserUserAgent()`（指纹单点·Chrome 版本升级只动 `browser.go`）
- **禁止** `Mozilla/5.0 ...` 字面量硬编码

## 静态锚

`TestNoBareHTTPBoundaryPatterns`（`httpclient/boundary_patterns_test.go`）——
四类禁止形态源码扫描·新增即拦·存量白名单登记：

| 禁止形态 | 检测正则 |
|---|---|
| 裸 `io.ReadAll(resp.Body)` | `io\.ReadAll\(resp\.Body\)` |
| 裸 `http.NewRequest(` | `http\.NewRequest\(`（排除 WithContext） |
| UA 硬编码 | `"Mozilla/5\.0` |
| `http.DefaultTransport` 赋值 | `http\.DefaultTransport\s*=` |

白名单文件：`httpclient/drain.go`（ReadBody 单点本体）、
`httpclient/browser.go`（BrowserUserAgent 单点本体）、
`httpclient/chrome_transport.go`（CF 指纹栈）。

## 维护触发表

| 变更场景 | 必读节 |
|---|---|
| 新增外部 HTTP 调用 | §1（client 取法）+ §2（响应体处置）+ §3（请求构造） |
| 切换代理 / 加图床 | 附录G §2.2（URL 基址）+ 09 §2.1（transportSwitcher 注入形态） |
| 接第三方 API | §1 合理例外登记 + §2 响应体上限评估 |
| 升级 Chrome 指纹 | `httpclient/browser.go`（单点——全库自动跟） |
