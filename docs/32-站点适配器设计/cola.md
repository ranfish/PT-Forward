# 可乐（ColaPT）站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 可乐（ColaPT） |
| 域名 | www.colapt.com（Web）/ api.colapt.com（API 兼容面）/ tracker.colapt.com（Tracker） |
| 引擎 | reelcore（自研）+ **M-Team 兼容面**（MoviePilot v3 官方站点目录收录） |
| 认证方式 | **`x-api-key` 头**（Key 前缀 `rcapi_`） |
| ⚠ 认证禁忌 | 禁止同带 Cookie / Authorization / Origin——带了反被判浏览器请求拒绝 |
| API Key 管理 | /account/tracker?tab=api（Web 登录态）；Key 只显示一次（仅存哈希）；按 scope 分能力 |
| scope 面 | 「兼容面（MoviePilot）」= mtorrent.compat（搜索/下载）；**automation 面**=上传（需单独勾选） |
| CookieCloud | colapt.com |
| Tracker | https://tracker.colapt.com/announce?passkey=xxx（passkey 嵌 announce——tracker 专用，不进系统凭据） |
| 候选制 | submit → 审核流（API 上传三步终态） |

## 架构分层（三面分离——全库首见）

```
www.colapt.com   = Web 前端外壳（SPA）+ automation 上传面（/api/v1/automation/*）
api.colapt.com   = MoviePilot 兼容面（/api/*——搜索/下载/用户数据）
tracker.colapt.com = Tracker（passkey 嵌入 announce）
```

- 站点表单地址填 **www 主域**（MoviePilot 自动换算 api 子域）
- PT-Forward 的 base_url 配 **api.colapt.com**（接口主机——MTeamAdapter 直连）

### framework 定案：`cola`（2026-11-03 用户定案）

可乐是 **reelcore 自研架构**，与 M-Team 各自架构——**不混用 mteam 架构名**：

- `framework: "cola"`（资产+migration142 存量刷正）
- Factory：`case "cola"` → MTeamAdapter（**实现层同构复用**，架构语义独立）
- RSS 策略表/默认路径/发布配置/API Key 测试端点：cola 分支同 mteam 值
- detect 特征判定：`reelcore` 字样 → cola（探测完备）
- 前端架构标签：`Cola (ReelCore)`

## MoviePilot 兼容面契约（获取链——MTeamAdapter 同构复用）

| 端点 | 用途 | 权限 | 实测（2026-11-03） |
|---|---|---|---|
| POST /api/member/profile | 用户资料 | Key 有效 | ✓ uid=907 ranfish |
| POST /api/torrent/search | 关键词搜索 | catalog.read | ✓ M-Team 官方字段（id/name/smallDescr/size/category/imdb/status.discount） |
| POST /api/torrent/genDlToken?id=N | 换下载票 | torrent.download | ✓（首试超时=节流，冷却后通） |
| GET /downloads/<凭据>/<id> | 下载 .torrent | 凭票访问 | ✓ 7997B bencode |
| POST /api/member/getUserTorrentList | 做种列表 | Key 有效 | 未实测 |
| POST /api/tracker/myPeerStatus | peer 状态 | Key 有效 | 未实测 |
| POST /api/msg/search · markRead | 站内信 | Key 有效 | — |
| POST /api/subtitle/* | 字幕四件套 | catalog.read | 返回空（本站无字幕文件） |
| ~~POST /api/torrent/detail~~ | **✗ 不在契约**（scope_denied 实证） | — | Detail/DetectDiscount 需替代方案 |

### 响应形态

- `{"code":"0"}` 字符串与 `{"code":0}` 数字**混合形态**（profile 字符串/search 数字）——MTeamAdapter `json.Number` 解析容错 ✓
- 错误：`{"error":{"code":"api_key_scope_denied","message":...,"request_id":...}}`

### MTeamAdapter 复用对照

| MTeamAdapter 实现 | 可乐复用性 |
|---|---|
| x-api-key 头（:198） | ✓ 直接复用 |
| POST /api/torrent/search（searchViaAPI :92） | ✓ 同路径同字段 |
| POST /api/torrent/genDlToken（DownloadTorrent :218） | ✓ 同路径 |
| code json.Number 解析 | ✓ 混合形态容错 |
| POST /api/torrent/detail（GetTorrentDetail/Detect） | ✗ **兼容面无此端点**——需特化：search 按 id 反查构造 detail |

### 契约差异修复（2026-11-03 实施代码层，§59.460）

reelcore 与 M-Team 官方 API 的**字段类型差异**（同路径不同型——兼容面的微妙处）：

| 字段 | M-Team 官方 | reelcore（可乐） | 修法 |
|---|---|---|---|
| search data[].id | 字符串 `"13573"` | 数字 `13573` | `json.Number` 双兼容 |
| memberCount.shareRate 等 4 字段 | 字符串 `"999999"` | 数字 `999999` | `json.Number` 双兼容 |
| URL 基址 | domain 即 API 主机（api.m-team.cc） | **domain=www 主机，base_url=api 子域** | `resolveBaseURL` BaseURL 优先 |
| 签名 URL 下载 | 带 x-api-key ✓ | **带 x-api-key → 403 scope_denied** | 403 时无头重试（签名自含鉴权） |

### 节流（环境层）

连续快速请求被节流（genDlToken 首试 20s 超时 0 bytes，冷却 30s 后 200）——肉丝同族。

## automation 上传面（发布链——三步 REST，全库首见第三形态）

**主机：www.colapt.com**（api 子域网关 404 实证——与兼容面分开部署）。

```
① POST /api/v1/automation/torrents          上传原始 .torrent 文件
② PUT  /api/v1/automation/torrents/{id}/metadata  保存版本信息+截图地址
③ POST /api/v1/automation/torrents/{id}/submit    提交审核（media_id 或 new_media）
```

规则（站方 API 文档页）：
- 只能操作自己的上传
- **截图 ≥3 个不同有效图片地址**（URL 直投——野马同款；API 不提供图片上传）
- **无 PTGen**（描述自组装）
- 作品可使用已有编号（media_id）或自行填写资料（new_media）
- 修改携带 **expected_revision**（乐观锁）
- 提交携带 **Idempotency-Key**（幂等头——肉丝同款）
- 请求结果未知时读该记录核对再继续，不要重复上传
- 退出登录不使 Key 失效；改密/重置 2FA 撤销 Key

### 探测状态（2026-11-03）

| 探测 | 结果 |
|---|---|
| api.colapt.com/api/v1/automation/torrents | 网关 404（openresty——不在此主机） |
| www.colapt.com/api/v1/automation/torrents | `api_key_scope_denied`（**路由存在 ✓**——Key 未勾 automation 面） |

**待办**：用户在 Key 管理页勾 automation 面 scope 后，探测三步字段级契约（①torrent 字段 ②metadata 字段集 ③submit 的 media_id/new_media 结构）。

## 摸底时间线（2026-11-03）

- profile → 200（uid=907 ranfish，2026-10-20 注册，上传 86MB）
- search → 200（M-Team 官方字段；FREE 促销在 status.discount）
- genDlToken → 首试超时（节流）→ 冷却 30s 后 200（签名 URL exp+sig）
- 下载 → 200 7997B bencode（announce=tracker.colapt.com+passkey）
- detail → 403 api_key_scope_denied（兼容面外）
- automation 双主机探测 → 定位 www 主机 + scope 待勾

## 骨架收录（2026-11-03）

- supported_sites.json：framework=mteam / auth_type=apikey / show_apikey+hint（rcapi_ 前缀+勾面提示）/ cookiecloud_domain=colapt.com
- sites.json：base_url=**api.colapt.com**（接口主机）/ tracker_domains=[tracker.colapt.com] / groups=[]（官组未知待挖）/ enabled=false（新站铁律）
- 凭证：用户前端填写（API Key 不落盘文档）；cookie 走 CookieCloud 自动同步

## 待挖清单（cookie 通道 + scope 勾选后）

1. automation 三步字段级契约（发布适配核心输入）
2. 上传表单 Web 页（表单字段/分类值集——form 资产）
3. 站规（发布规则/禁转/标签）
4. 官组信息（groups 收录判断——is_source 定案）
5. RSS 通道存在性
6. getUserTorrentList/myPeerStatus 实测（做种统计链）

## 摸底方法论：Web 面 vs API 面分离（2026-11-03 用户定案）

三面分离架构下**摸站必须区分通道**——方法/凭证/头/解析四维全不同：

| 维度 | Web 面摸底 | API 面摸底 |
|---|---|---|
| 主机 | www.colapt.com | api.colapt.com |
| 凭证 | cookie（reelcore_session，经 CookieCloud 同步） | x-api-key（rcapi_ 前缀） |
| 头 | 浏览器形态（UA/Referer/常规 Accept） | **仅 x-api-key**（禁同带 Cookie/Authorization/Origin——带了反被拒） |
| 响应 | HTML/SPA（需解析或渲染） | JSON |
| 节流 | Cloudflare 层（reset/超时） | 站方层（scope 校验/节流） |

**混合形态警觉**（本案实战）：API 返回的下载 URL 指向 **www 主机** `/downloads/<hash>/<id>?exp&sig`——这是"API 签发的 Web 通道 URL"：对它带 x-api-key 头会触发 www 路由的 scope 校验（`api_key_scope_denied` 403），**无头裸 GET 即 200**（签名参数自含鉴权）。判断通道归属看 URL 主机+路径，不看它从哪个 API 拿到的。
