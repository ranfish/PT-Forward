# 可乐（ColaPT）站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 可乐（ColaPT） |
| 域名 | www.colapt.com（Web）/ api.colapt.com（API 兼容面）/ tracker.colapt.com（Tracker） |
| 引擎 | reelcore（自研）。对外提供 **MoviePilot 兼容面**（接口协议与 MoviePilot 站点目录的 mTorrent schema 兼容——与 M-Team 站点本身无关联） |
| 认证方式 | **`x-api-key` 头**（Key 前缀 `rcapi_`） |
| ⚠ 认证禁忌 | 禁止同带 Cookie / Authorization / Origin——带了反被判浏览器请求拒绝 |
| API Key 管理 | /account/tracker?tab=api（Web 登录态）；Key 只显示一次（仅存哈希）；按 scope 分能力 |
| scope 面 | 「兼容面（MoviePilot）」= mtorrent.compat（搜索/下载）；**automation 面**=上传（需单独勾选） |
| CookieCloud | colapt.com |
| Tracker | https://tracker.colapt.com/announce?passkey=xxx（passkey 嵌 announce——tracker 专用，不进系统凭据） |
| ⚠ 双钥匙 | tracker passkey 与 **RSS Key 是两把不同的钥匙**（实测前缀不同）：RSS Key 用于 torrentrss.php 参数+`/feeds/downloads/{RSSKey}/{id}` 下载直链（account/tracker?tab=rss 页获取） |
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
- 默认路径/发布配置/API Key 测试端点：cola 分支同 mteam 值；**RSS 策略表独立**（guid_regex+RequiresSideLoading——见 §RSS 通道契约，v0.0.1325 修正）
- detect 特征判定：`reelcore` 字样 → cola（探测完备）
- 前端架构标签：`Cola (ReelCore)`

## MoviePilot 兼容面契约（获取链——MTeamAdapter 同构复用）

| 端点 | 用途 | 权限 | 实测（2026-11-03） |
|---|---|---|---|
| POST /api/member/profile | 用户资料 | Key 有效 | ✓ uid=907 ranfish |
| POST /api/torrent/search | 关键词搜索 | catalog.read | ✓（响应字段 id/name/smallDescr/size/category/imdb/status.discount） |
| POST /api/torrent/genDlToken?id=N | 换下载票 | torrent.download | ✓（首试超时=节流，冷却后通） |
| GET /downloads/<凭据>/<id> | 下载 .torrent | 凭票访问 | ✓ 7997B bencode |
| POST /api/member/getUserTorrentList | 做种列表 | Key 有效 | ✓ 200（v0.0.1324 实测，条目 `{torrent:{size,source}}`——做种数降级通道） |
| POST /api/tracker/myPeerStatus | peer 状态 | Key 有效 | **✗ 403**（代码实测端点名 `myPeerStatistics` scope_denied；站方文档写 `myPeerStatus`——**端点名存疑**，复验时用站方名再试一发；当前 Stats 走 getUserTorrentList 降级不依赖此端点） |
| POST /api/msg/search · markRead | 站内信 | Key 有效 | — |
| POST /api/subtitle/* | 字幕四件套 | catalog.read | 返回空（本站无字幕文件） |
| ~~POST /api/torrent/detail~~ | **✗ 不在契约**（scope_denied 实证） | — | 已解决：Web 面 `GET /api/v1/torrents/{id}` 降级（v0.0.1324/1325，见 §获取链补全） |

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
| POST /api/torrent/detail（GetTorrentDetail/Detect） | ✗ **兼容面无此端点**——已实施 Web 面 `/api/v1/torrents/{id}` 降级（403 触发，字段更全） |

### 契约差异修复（2026-11-03 实施代码层，§59.460）

**PT-Forward 工程适配记录**：MTeamAdapter 复用时发现的响应类型差异（MTeamAdapter 原实现对齐 M-Team 响应形态，可乐实际响应为真数字——两站各自实现无关联，仅 PT-Forward 内部共用 adapter）：

| 字段 | MTeamAdapter 原实现（M-Team 形态） | 可乐实际响应 | 修法 |
|---|---|---|---|
| search data[].id | 字符串 `"13573"` | 数字 `13573` | `json.Number` 双兼容 |
| memberCount.shareRate 等 4 字段 | 字符串 `"999999"` | 数字 `999999` | `json.Number` 双兼容 |
| URL 基址 | domain 即 API 主机（api.m-team.cc） | **domain=www 主机，base_url=api 子域** | `resolveBaseURL` BaseURL 优先 |
| 签名 URL 下载 | 带 x-api-key ✓ | **带 x-api-key → 403 scope_denied** | 403 时无头重试（签名自含鉴权） |

### 节流

~~连续快速请求被节流——肉丝同族~~ **定性修正：非环境层，是站方限流器**（Key 维度 60/min 突发 20——见 §限流三层结构定量节）。

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
- search → 200（FREE 促销在 status.discount）
- genDlToken → 首试超时（节流）→ 冷却 30s 后 200（签名 URL exp+sig）
- 下载 → 200 7997B bencode（announce=tracker.colapt.com+passkey）
- detail → 403 api_key_scope_denied（兼容面外）
- automation 双主机探测 → 定位 www 主机 + scope 待勾

## 骨架收录（2026-11-03）

- supported_sites.json：framework=mteam / auth_type=apikey / show_apikey+hint（rcapi_ 前缀+勾面提示）/ cookiecloud_domain=colapt.com
- sites.json：base_url=**api.colapt.com**（接口主机）/ tracker_domains=[tracker.colapt.com] / groups=[]（官组未知待挖）/ enabled=false（新站铁律）
- 凭证：用户前端填写（API Key 不落盘文档）；cookie 走 CookieCloud 自动同步

## 待挖清单（2026-11-03 深夜二轮更新）

1. automation 三步字段级契约（发布适配核心输入——**等用户申请发种机权限+勾 scope**）
2. 上传表单 Web 页（表单字段/分类值集——form 资产；官方转种脚本存在=结构稳定预期）
3. 官组信息（groups 收录判断——is_source 定案；Web 面详情 release_group+official 字段可做数据支撑）
4. ~~RSS 通道存在性~~ ✅ 已接入（v0.0.1325）
5. ~~getUserTorrentList/myPeerStatus 实测~~ ✅ 前者已实测（做种数降级通道）；后者端点名存疑待复验
6. 站规全貌（wiki 八篇已拉：计费/等级/客户端/发种——禁转标记形态仅转种脚本一句带过，适配源站角色时需深挖）

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

## wiki 权威规则（2026-11-03 实拉——/api/v1/wiki/moviepilot-setup，修订 2/2026-10-20）

> wiki 页面=SPA 前端路由；内容走 **Web 面 JSON API `GET /api/v1/wiki/{slug}`**（cookie 认证，reelcore_session）。
> 站方同时给出规则来源源码路径（internal/publication/publication.go 等——reelcore 源码可能可得，待挖）。

### 限流定量（此前"节流"现象的数字真身）

- **单 Key 60 次/分、突发 20**；全站机器请求 120 次/秒；超限 429 + `Retry-After`
- 2026-11-03 实测吻合：序列实验连发远超突发 20 → api/www 双面挂（30s deadline/reset）

### automation 发种面硬规则（发布链适配核心契约）

| 规则 | 内容 | 错误码 |
|---|---|---|
| release_scope 结构 | movie/other 传 `null`；tv 必填 category/season/scope/episodes；anime 五键（多 anime_type）；variety 另一套；分类须与作品一致 | 400 scope_category_mismatch / invalid_release_scope |
| 种子文件 | 只收**私有 v1** .torrent，≤4MiB，文件树深≤32/≤10000 文件；v2/hybrid 拒 | 400 invalid_torrent / 413 torrent_too_large |
| 截图 | API 提交 **≥3 张**不同 http(s) 直链（≤20 不重复）；**豆瓣图（doubanio.com/imgN.douban.com）站方自动镜像到图床**，镜像失败可换图重试（发布链利好：豆瓣源截图可直投） | 400 invalid_screenshots / 422 image_mirror_failed |
| info_hash 唯一 | 同 hash 仅一条；驳回≠作废可改后 resubmit（不占新额度）；换压制=换文件新 hash | 409 torrent_conflict |
| 做种门 | 审核通过时实时查 swarm，seeders=0 不予通过（查询失败不拦） | 409 torrent_not_seeding |
| 每日额度 | 按等级/用户组（可能不限量），Asia/Shanghai 零点切日 | 403 daily_publication_limit_reached |
| 作品复用 | 站内已有作品标识必须复用，不许新建重复条目 | 409 media_reuse_required |
| H&R 拦下载 | 未解决 H&R 违规禁下载 | 403 hr_download_restricted |

### 凭据/通道规则（与 §摸底方法论 互证）

- 兼容面只认 `X-API-Key`；**同时带 Cookie/Authorization/Origin → 判浏览器请求 401 api_key_required**（站方原文——与下载 URL 通道判定实证一致）
- Key scope 权威名：兼容面=`mtorrent.compat`；发种面=automation（资格单独开通）
- 站点地址只填主域（api.colapt.com 根路径 404——接口主机由客户端换算）

### 兼容面补充契约

- search 结果促销字段 `discount`/`promotionRule`：MP 认识档位 FREE/PERCENT_50/PERCENT_70/_2X，**其它比例显示 NORMAL**（DetectDiscount 消费注意）
- profile 返回中文等级名 `roleName`；`shareRate:999999`=占位值（计费下载 0+上传非 0=无限，非真实分享率）

## 第四面：插件面 + 源码可得性探测（2026-11-03）

### 源码结论：引擎闭源（roadmap gh-pages 分支深挖）

- **主引擎仓库 `ReelCoreHQ/reelcore` 存在但私有**（roadmap data.json `repository` 字段暴露仓库全名；GitHub API 404=私有不可见）
- `reelcore-roadmap` 默认分支空壳，**gh-pages 分支=主仓库进度的静态导出镜像**（80KB data.json，13 阶段 RC00-RC12 全 feature 清单，source_sha 指向主仓库 commit）
- 官方 GitHub org `ReelCoreHQ` 公开 2 仓库：`reelcore-roadmap`（进度镜像）+ **`moviepilot-plugin-colapt`**（官方 MoviePilot 辅种插件，Python v0.10.5，MIT——**API 消费权威参考**，本地 `/tmp/opencode/cola-plugin`）

### reelcore 架构全景（roadmap data.json 2026-09-21 生成）

- **Go 重写引擎，独立于 NexusPHP**（RC02 新 Go 核心三入口/自有 DB 版本化建库/双 module CI；RC06-06 与 NexusPHP 解耦验证）
- **Tracker=Torrust 血统**（RC06-02 "Redis 与 Torrust 故障"；RC04 新 Tracker：租约 fencing/成功前持久化/流水聚合事务一致/计费规则版本化）
- RC09 完整产品 API：账号邀请/完整种子业务/统计首发运营/管理审计/API 安全矩阵
- **RC10 IYUU 原生 API 框架适配**：可乐有 IYUU 协议原生辅种 API（独立凭据）+IYUU ReelCore 适配器（上游 PR 材料——IYUU 主仓库未落地，PR 未提/未并）
- RC11 React 前端（电影分组/双布局/多主题）；RC12 上线演练未完成（scope_note：RC08 长稳跳过）

### 插件面契约（www 主机 /api/v1/*，Cookie 认证）

| 端点 | 用途 | 实测 |
|---|---|---|
| GET /api/v1/plugin/match?hashes=h1,h2 | **info_hash/source_info_hash 批量反查**（辅种核心） | ✓ 200（CC cookie 直用） |
| GET /api/v1/plugin/match?keys=k1,k2 | 文件清单指纹反查（老站版本 400） | 未实测 |
| POST /api/v1/plugin/sessions | 用户名+密码+图形验证码(+2FA) 换 session_token | 未实测（CC cookie 可绕） |
| GET /api/v1/torrents/{id}/download | 下载 | 未实测 |
| GET /api/v1/torrents/{id}/authorizations | 授权信息 | 未实测 |

- **match 响应**：`{"checked":N,"matched":[{client_hash,via,torrent_id,…}],"unmatched":N,"content":[],"limit":500}`
- **via 语义**：`source_info_hash`=客户端有同内容异站种子（可辅种）/`info_hash`=已是可乐种子（跳过）
- 批量上限 **500 hash/请求**（插件保守用 100/批——CF/网关拒大 query）
- **CC 浏览器 cookie（reelcore_session）直接可用**——无需密码+验证码换会话（2026-11-03 实测）→ PT-Forward 辅种/恢复链可用 hash 级反查

### 关键知识：info.source 盖章差异

可乐发布时把 `.torrent` 的 `info.source` 盖成 `[https://www.colapt.com] ColaPT`——**同内容跨站 info_hash 必然不同**（辅种 hash 比对陷阱：可乐下载的种子 hash≠任何源站 hash，announce 嵌 tracker+passkey 同理）；官方插件做 source 替换/剥除重算 hash 归一化反查。PT-Forward 恢复链/VerifyExists 的 hash 比对需注意此语义。

## wiki 八篇全量+获取链补全（2026-11-03 深夜轮）

### wiki 全目（GET /api/v1/wiki 列表 8 篇）

| slug | 标题 | 关键情报 |
|---|---|---|
| billing-rules | 发种计费知识（修订 23） | **下载全免（仅上传计费）**；raw/billed 双口径；**发种人保护：发布后 24h 非发种人辅种上传按 30% 计账**（「发种守护中」标签）；发种奖励 72/144/360h 额外 0.2/0.5/2.0 倍（基数 raw）；离线规则单次≤1h/72h 内 2 次豁免/累计 4 次失效 |
| site-levels | 站点等级（修订 6） | 7 级迷你罐→可乐大亨；四门槛（实际上传/天数/分享率/完成数）+**付费解锁**（瓶盖+核子可乐）；分享率考核 2026-11-01 起（<1 三天警告→<0.5 限下载→14 天停用）；VIP/工作组豁免 |
| beginner-downloads | 客户端规则（修订 6） | **白名单**：qB 4.0~5.2.4（**禁增强版**）/Deluge 2.x/TR 4.0.1~4.0.4+4.1（**禁 4.0.6**）；禁 uTorrent/BiglyBT；关 DHT/LPD/PEX；当前公示阶段未启用自动拒绝 |
| 1 | Cola 转种脚本 | **站方官方转种油猴脚本**（auto_feed 衍生）——转种友好站；"有禁转需点击两次"（禁转标记存在） |
| beginner-upload-guide | 新手发种指南 | 截图 ≥3 张不同画面+直链可开（拉取截断 90%） |
| moviepilot-setup/crossseed/invite-meds | — | 已读（规则/插件/瓶盖市场） |

**对 PT-Forward 的直接影响**：①下载全免→RSS/获取链优惠检测天然全过（promotion 结构 `download:{num:0,den:1}` 实证）②24h 发种保护→转发后辅种收益 30%（可辅种非禁辅）③**客户端白名单→推种下载器版本合规**（qb 4.x~5.2.4 官方版 ✓）④官方转种脚本存在→Web 表单结构稳定预期。

### 获取链补全实施（v0.0.1324）

| 缺口 | 实施定案 |
|---|---|
| Stats 做种数恒 0 | `myPeerStatistics` 不在兼容面（403 实证）——**行为驱动降级**：非 200 时 fallback `getUserTorrentList` 翻页计数（响应条目 `{torrent:{size,source}}` 嵌套；实测 1 颗 3GB ✓；M-Team 主端点健康零变化） |
| Detail 403 | **行为驱动降级**：`/api/torrent/detail` 403 时 fallback Web 面 `GET /api/v1/torrents/{id}`（cookie 通道——**字段比兼容面假想 detail 更全**：info_hash+**source_info_hash 双 hash 原生**/完整 MI/文件树/release_group+official 官种标记/promotion 分数结构/publication_stage 审核状态机/media_id+release_scope 作品关联/screenshots 直链数组/size_bytes 字符串数字） |
| **RSS** | ~~无公开通道~~ **作废修正（2026-11-03 深夜二轮，用户供链接）**：`torrentrss.php?passkey=<RSSKey>&rows=50&linktype=dl` 完整可用（200/43.7KB/50 条，站方自述"NexusPHP 兼容格式，凭据即 RSS Key"）。**此前 403 cookie_authentication_forbidden 是探测方法错误**——passkey 通道禁混带 cookie（与 x-api-key 通道同族"每通道单一凭证"铁律，裸 GET 即 200） |
| Download 端到端复验 | 修复行为级证据完整（同栈对照：无头 200/32894B vs 带头挂 30s）；完整链复验连续 5 撞限流窗（今日配额烧尽）——**挂明日冷启动首验** |

### 限流三层结构与 PT-Forward 节流器对位（2026-11-03 定案）

**站方限流=请求限流非端点限流**（Key 维度跨端点共享：配额烧尽时 profile/search/genDlToken 同批挂——非各端点独立配额）：

| 层 | 维度 | 数字 | 撞限表现 |
|---|---|---|---|
| 兼容面（api，x-api-key） | 单 Key 请求总数 | 60/min 突发 20 | 429+Retry-After / 挂到超时 |
| www Web 面（cookie/无认证） | IP/连接层（CF） | 未知 | reset/000/截断（无 429 语义） |

**PT-Forward 节流器对位**：GlobalLimiter（§59.340）按**请求主机**分桶——可乐双主机两桶（api/www 各自默认 0.5RPS=30/min，fetch_rps 未配置）；生产链（GetAdapter doer）已在默认桶保护内（30/min < Key 配额 60/min，余量给手动操作）。**烧穿站方配额的是探针/独立 client/curl**（诊断工具绕过 limiter）非生产行为。未来站方降配额或加高频链时按 `fetch_rps` 显式收紧（注意：注册键=domain 列的 www 形态，api 子域桶需单评估）。

## RSS 通道契约与接入（2026-11-03 深夜二轮——用户供链接实测）

### feed 契约（torrentrss.php，NexusPHP 兼容自述）

- **认证**：URL passkey 参数（RSS Key——account/tracker?tab=rss 页获取；**禁混带 cookie**：带了回 403 cookie_authentication_forbidden——与 x-api-key 通道同族"每通道单一凭证"设计；裸 GET 即 200）
- **item 结构**：title/link=enclosure（`/feeds/downloads/{RSSKey}/{id}` 下载直链——RSSKey 直接当下载凭据）/guid=`/torrents/{id}` 详情页 URL（isPermaLink=true）/pubDate/category 多维（分类+分辨率+来源）/description 结构化（大小/做种/下载/完成/分类/作品/分辨率/来源——**无优惠字段**：全站免费站，免费语义在 promotion 不在 feed）/enclosure.length 精确字节
- **参数**：rows=50（条数）/linktype=dl

### cola RSS 策略表（v0.0.1325 修正）

```
HashStrategy: guid（guid=详情 URL 非 40hex → IsFakeHash 自动 true → 侧载链算真 hash，肉丝同款）
SizeStrategy: enclosure（length 精确字节）
IDStrategy: guid_regex + /torrents/(\d+)（link 是路径形态 query_param 恒 miss；guid 路径不含 RSSKey 更稳）
RequiresSideLoading: true
```

离线驱动验证 ✓：50 条全解析（id=13731/13730/13729 提取正确+fake hash 标记+size 精确）。

### 免费判定链（全站免费站的 DetectDiscount 路径）

RSS feed 无优惠字段 → 走 adapter DetectDiscount → `/api/torrent/detail` 403 → **Web 面 promotion 降级**（v0.0.1325）：`GET /api/v1/torrents/{id}`（cookie）解析 `promotion.download:{num,den}`——num==0 → FREE/比例归 PERCENT 档/不认识回 NONE（站方全站免费期恒 FREE）。侧载链走 DownloadTorrent（genDlToken 通道已修——自动可用）。

### 待明日冷启动复验（限流窗）

①Download 端到端 ②DetectDiscount 降级链（promotion→FREE）③RSS 侧载真 hash——三发收口。
