# 好大 HDArea 统一 API 网关——官方文档结构化落盘

> **来源**：站方公告 2026.11.11（首页）+ 官方文档页 `api_docs.php`（2026-11-12 抓取全文转写）。
> **用途**：全量 API adapter（2026-11-12 用户定案：范围 A 全量·独立 adapter·公共/单点优先）讨论与实施的权威查阅底本。
> **约定**：本文=官方文档结构化转写；标 ⚠️ 处=文档未详述需刀1 活体探针实测确认（文档≠实测——§59.1339 教训）。原始 HTML 只读存档于抓取当日 `/tmp/opencode/hdarea/`（临时·不提交）。

## 1. 总体架构

一个内核 · 三种协议输出：

| 协议 | 入口 | 用途 |
|------|------|------|
| 标准 Torznab (XML) | `GET /torznab.php`（别名 `/api/torznab`） | 检索（MoviePilot/Prowlarr/Sonarr 等原生兼容） |
| 增强刷流 RSS & 百宝箱 | `GET /torrentrss.php` / `GET /cartrss.php` | 刷流/追更（内置促销倍率+Free 到期时间） |
| RESTful JSON API v1 | `https://hdarea.club/api/v1/` | 全功能（搜索/详情/下载凭证/批量辅种/发种/编辑/字幕/做种历史/签到） |

- **M-Team 兼容路由别名**：每个端点有 `/api/torrent/xxx` 风格别名（见端点表）——路由名兼容，参数/方法形态与 mtorrent 私有协议**有差**（结构参考≠协议克隆）。
- **无伪静态兼容入口**：`/api.php?route=v1/status`（Nginx 未配 rewrite 时用）。
- 站方政策：第三方工具尽快切换 API Key 模式；**严禁多线程高频网页爬虫**。

## 2. 鉴权与令牌

**三传递方式（优先级从高到低）**：

| 方式 | 形态 | 说明 |
|------|------|------|
| Header `X-API-Key` | `X-API-Key: <32位>`（大小写不敏感） | 最推荐（M-Team 兼容）；密钥不进 Nginx URL 日志 |
| Header `Authorization` | `Bearer <32位>` | OAuth2 风格客户端 |
| URL Query | `?apikey=<32位>` | Torznab/RSS/下载直链 |

**多子令牌**（控制面板→安全设定→API Key 存取令牌管理）：

- 每会员最多 **5 个**独立子令牌；可命名、按需勾选 scope、绑定 IP 白名单（IPv4/IPv6/CIDR）。
- Scope 四类：`read`（检索/详情/预查重/辅种匹配/历史/字幕列表/profile/site）· `download`（download.php/downloadsubs/RSS dl 直链/genDlToken）· `upload`（upload/edit/字幕上传/免 Cookie 直投 takeupload）· `user`（签到/私信/收藏/百宝箱/感谢）。
- **Passkey 零暴露**：API Key 与 tracker passkey 完全隔离；下载 .torrent 时服务端输出瞬间动态注入个人 passkey。
- **dl_token**：JSON/Torznab 接口默认签发 **1800s** HMAC-SHA256 防盗链短效下载凭证（避免向远程下载器暴露永久 API Key；过期 403 重取即可）。

我们状态：`PT-Forward` 子令牌已预建（全 scope·2/5 已用）；IP 白名单不绑（2026-11-12 用户定案④）。

## 3. 限流配额（每 60 秒窗口·超限 HTTP 429 + `Retry-After`）

| 通道 | 上限 | 我们配置建议 |
|------|------|--------------|
| read（检索/搜索/Torznab/辅种匹配） | 60 次/分 | fetch_rps ≤ 0.8（卡线收紧） |
| RSS 订阅轮询 | 15 次/分 | RSS 间隔 ≥ 5min |
| write（签到/收藏/百宝箱/感谢） | 20 次/分 | — |
| upload（发种/编辑/字幕） | 30 次/分 | 发布链天然低频 |

## 4. 协议一：Torznab（`GET /torznab.php` / `/api/torznab`）

| 参数 | 必填 | 说明 |
|------|------|------|
| `t` | 是 | `caps` / `search` / `tvsearch` / `movie` / `music`（caps=能力声明+分类映射·探活用） |
| `apikey` | caps 外必填 | 亦可 Header 传入；需 read 权限 |
| `q` | 否 | 关键词（主标题+副标题·多词空格 AND） |
| `season` / `ep` | 否 | 季集过滤（自动拼 S01E03 规则） |
| `imdbid` | 否 | `tt15239678` 或纯数字 |
| `doubanid` | 否 | 豆瓣 Subject ID |
| `cat` | 否 | Newznab 大类（2000 电影/5000 剧集/5070 动漫/3000 音乐/5060 体育）+ 本站原生 401~417 双支持 |
| `spstate` | 否 | 促销过滤（值域见 §8） |
| `limit` / `offset` | 否 | 分页（max 100·默认 50） |

⚠️ 鉴权错误时 Torznab 仍返回 **HTTP 200** + `<error code="100">` XML 节点（行业标准行为）。

## 5. 协议二：增强 RSS（`GET /torrentrss.php` & `/cartrss.php`）

| 参数 | 说明 |
|------|------|
| `apikey` | 身份凭证（生成下载直链需 read+download） |
| `linktype=dl` | `<link>`/`<enclosure>` 输出下载直链（download 权限时默认启用） |
| `ifree=1` | 条目标题前缀实时促销标签（[免费]/[2X免费]） |
| `spstate` | 服务端精准过滤有效促销种（刷流建议 `9`=所有免费） |
| `rows` | 10~50（默认 50） |
| `cat{ID}=1` 等 | `cat401=1&cat415=1` 多选；可叠 `med{ID}`/`cod{ID}`/`tea{ID}`（订阅页可视化生成） |
| `inclbookmarked=1` / `inclcarted=1` | 仅订阅收藏夹 / 百宝箱（cartrss） |

**条目扩展节点**（对 Vertex/MoviePilot 暴露体积/做种/免费因子/促销截止）：

```xml
<item>
  <title>[免费][Movies Blu-ray] Dune.Part.Two...</title>
  <enclosure url="https://hdarea.club/download.php?id=12345&amp;apikey=YOUR_KEY" length="78451204096" type="application/x-bittorrent"/>
  <torznab:attr name="size" value="78451204096"/>
  <torznab:attr name="seeders" value="42"/>
  <torznab:attr name="leechers" value="5"/>
  <torznab:attr name="infohash" value="8f4e..."/>
  <torznab:attr name="downloadvolumefactor" value="0"/>
  <torznab:attr name="uploadvolumefactor" value="1"/>
  <nexusphp:sp_state>free</nexusphp:sp_state>
  <nexusphp:promotion_until>2026-11-12 18:00:00</nexusphp:promotion_until>
</item>
```

## 6. 协议三：RESTful JSON API v1 端点全表（按 scope）

### read

| 方法 | 端点 | M-Team 兼容别名 | 说明 |
|------|------|------------------|------|
| GET/POST | `/api/v1/torrents` | `/api/torrent/search` | 列表+多维检索：关键词/IMDb/豆瓣/多选分类/spstate/体积区间/排序；兼容 M-Team `keyword/pageNumber/pageSize/discount` 参数 |
| GET | `/api/v1/torrents/{id}` | `/api/torrent/detail?id=` | 全量详情：BBCode **与渲染后 HTML** 简介、NFO、文件列表、外挂字幕、同豆瓣/IMDb 其它画质版本、最新评论、做种状态 |
| GET | `/api/v1/torrents/{id}/files` `/peers` `/snatches` `/comments` | — | 按需分页子资源 |
| POST | `/api/v1/torrents/hashes` | `/api/v1/torrents/match` | **批量辅种**：`hashes`（≤200 个 40 位 hex）或 `ids`，返回本站对应种子+短效下载链接（IYUU 专用） |
| POST | `/api/v1/tracker/queryHistory` | `/api/tracker/queryHistory` | `tids`（≤200）→ 实时做种状态 + 历史上传/下载/做种时长 |
| POST | `/api/v1/torrents/check` | `/api/v1/torrents/precheck` | 发种预查重（详见 §7.1） |
| GET | `/api/v1/subtitles` | `/api/subtitle/list` | 字幕检索（torrent_id/lang_id/q）+语言字典+短效下载链接 |
| GET | `/api/v1/user/profile` | `/api/member/profile` | 完整资料：上传/下载/分享率/魔力/**时魔公式明细拆解**/做种数/私信/邀请 |
| GET/POST | `/api/v1/user/torrents` | `/api/member/getUserTorrentList` | `type`: seeding/leeching/uploaded/completed/incomplete/bookmarked/cart |
| GET | `/api/v1/site/metadata` | — | **全站值集实时字典**（第六章同源）⚠️ 响应形态文档未详述 |

### download

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/v1/torrents/genDlToken`（别名 `/api/torrent/genDlToken`·`/api/v1/torrents/dl-token`） | 传 `id` → 带 HMAC-SHA256 过期签名（1800s）的短效下载 URL |
| — | `download.php?id=&apikey=` / `downloadsubs.php` / RSS `linktype=dl` | 直链族 |

### upload（需发布等级）

| 方法 | 端点 | M-Team 别名 | 说明 |
|------|------|--------------|------|
| POST | `/api/v1/torrents/upload` | `/api/torrent/createOredit` | 自动发种/转载（multipart `file` 或 JSON `torrent_base64`） |
| POST | `/api/v1/torrents/{id}/edit` | `/api/torrent/createOredit` | 编辑本人种子：标题/副标题/简介/分类/豆瓣 IMDb/NFO |
| POST | `/api/v1/subtitles/upload` | `/api/subtitle/upload` | 挂载外挂字幕（.srt/.ass/.ssa/.sup/.sub/.zip/.rar/.7z）+字幕魔力 |
| — | `takeupload.php` / `takeedit.php` | — | **免 Cookie 直投**（X-API-Key 头） |

### user

`POST /api/v1/user/signin`（每日签到）· `GET /api/v1/user/messages` · `POST /torrents/{id}/bookmark|cart|thanks`（`op=add|remove|toggle`）

## 7. 发布三步流（发布链核心）

### 7.1 预查重 `POST /api/v1/torrents/check`

- 入参：源站原始 `.torrent`（multipart `file` 或 `torrent_base64`）**或** 预期主标题 `name`。
- 服务端按本站规则**清洗源站私有字段重算真实 info_hash**。
- 返回：`can_upload: true`（若 `replaceable_dead_torrents` 非空=站内有同名/同 Hash **断种**，正式发种时**自动删除旧断种并顶替上架**）；`can_upload: false`（`reason`=`active_duplicate_exists`（活种或 **30 分钟新种保护期**）| `banned_duplicate`（禁发资源）→ 发种机直接跳过）。

### 7.2 发种 `POST /api/v1/torrents/upload` 参数全表

| 字段 | 必填 | 说明 |
|------|------|------|
| `file` 或 `torrent_base64`(+`filename`) | 是 | 服务端自动重写 Tracker+Private+本站 Source 标签 |
| `name` | 否* | 留空=torrent 文件名去后缀 |
| `small_descr` | 否 | 中文副标题 |
| `type`（或 `category`） | 是 | 主分类 int（§8） |
| `descr`（或 `description`） | 条件必填 | BBCode 简介；内含豆瓣/IMDb 链接而未传 `dburl`/`url` 时**自动提取关联** |
| `dburl` / `url` | 推荐 | 豆瓣链接或纯 ID / IMDb 链接或 tt；关联后站方自动同步海报与评分缓存 |
| `auto_ptgen` | 否 0/1 | 未传 descr（或希望在 MI 头部拼接）且有 dburl 时站内自动补全豆瓣 BBCode（仅入库环节触发） |
| `source_sel`/`medium_sel`/`codec_sel`/`audiocodec_sel`/`standard_sel`/`processing_sel`/`team_sel` | 推荐 int | 兼容省略 `_sel` 键名；**官方组（HDArea/EPiC/HDATV/HDApad）匹配对应分类+team_sel 自动激活置顶与限时 Free** |
| `nfo` / `nfo_base64` | 否 | ≤64KB |
| `anonymous`（或 `uplver`） | 否 yes/no | 需账号等级 |
| `allow_transfer` | 否 yes/no | 是否允许他人转载（默认 yes）——**发布侧禁转声明位**（转载者默认 yes 与 §59.511 share_rule 权利链一致） |
| `return_torrent_base64` | 否 0/1 | 响应 `data.torrent_base64` 直接返回已重算 InfoHash+注入个人 Passkey 的最终种子——解码直推下载器 |

### 7.3 官方示例流

check（file 上传预查重）→ upload（file+表单+return_torrent_base64=1）→ base64 解码推 qBittorrent → subtitles/upload。

## 8. 值集字典（第六章·与 2026-11-11 实测网页表单逐值一致）

| 域 | 键 | 值集 |
|----|-----|------|
| 主分类 | `type`/`cat` | 300 UHD-4K·401 Blu-ray·415 REMUX·416 3D·410 1080p·411 720p·414 DVD·412 WEB-DL·413 HDTV·417 iPad·404 纪录·405 动漫·402 剧集·403 综艺·406 MV·407 体育·409 Misc·408 HQ Audio（18） |
| 媒介 | `medium_sel` | 1 Blu-ray·3 REMUX·7 Encode·9 WEB-DL·4 MiniBD·5 HDTV·2 HD DVD·6 DVDR·8 CD（9） |
| 来源 | `source_sel` | 1 Blu-ray·2 HD DVD·3 DVD·4 HDTV·5 TV·6 Other（6）——**网页表单无此域·API 独有** |
| 视频编码 | `codec_sel` | 7 H.264·1 MPEG-4·6 H.265·4 MPEG-2·3 Xvid·2 VC-1·5 Other·8 AV1·9 VP8/9·10 AVS（10） |
| 音频编码 | `audiocodec_sel` | 28 项（6 AAC·5 DD5.1·7 TrueHD·3 DTS·4 DTS-HD MA·8 LPCM·9 WAV·2 APE·1 FLAC·10 TrueHD Atmos·11 DD2.0·12 DTS:X·13 DTS-HD HR·14 DSD·15 DDP Atmos·16 DDP·17 MPEG·18 Vorbis·19 TTA·20 AV3A·21 MP3·22 ALAC·25 Opus·26 WMA·27 AC-4·28 MPEG-H·29 MQA·24 Other） |
| 分辨率 | `standard_sel` | 3 720p·1 1080p·4 SD·2 1080i·5 4K（5） |
| 处理 | `processing_sel` | 1 Raw·2 Encode（2）——**网页表单无此域·API 独有** |
| 制作组 | `team_sel` | 1 EPiC·2 HDArea·3 HDWING·4 WiKi·5 TTG·6 other·7 MTeam·8 HDApad·9 CHD·10 HDAccess·11 HDATV·12 cXcY·13 CMCT（13） |
| 促销 | `spstate` | 1 普通·2 Free·3 2X·4 2X Free·5 50%·8 任意促销·9 所有免费(Free+2XFree) |
| 字幕语言 | `lang_id` | 31 项（25 简体中文·28 繁體中文·6 English·15 日本語…） |

实时字典端点：`GET /api/v1/site/metadata`（值集自愈数据源）。

## 9. 状态码与 FAQ

| 码 | 语义 | 要点 |
|----|------|------|
| 200 | 成功 | JSON `code===0` 成功；**Torznab 鉴权错仍 200**+`<error code="100">` XML |
| 400 | 参数缺失/种子不合法 | 字段提示见 `message` |
| 401 | API Key 无效/被重置 | 核对 32 位令牌 |
| 403 | scope 不足/IP 白名单/令牌停用/**dl_token 过期** | 过期重取 genDlToken |
| 404 | 种子不存在/Nginx 未配伪静态 | 伪静态 rewrite：`^/api/(.*)$ /api.php?route=$1`；临时用 `/api.php?route=` |
| 409 | 重复发种拦截 | 响应含 `existing_id` |
| 429 | 限流 | 等 `Retry-After`；辅种用 hashes 批量代替逐查 |
| 503 | 维护关停 | — |

**豆瓣合规**：不对外提供独立豆瓣/IMDb 抓取 API（防跨站盗刷）；检索时带出库内已关联评分海报；发种时自动提取关联。

## 10. 架构边界与刀序定案（2026-11-12 用户三轮讨论+回归检查定稿）

**架构边界（共享机制·不共享方言）**：
- 三层现实：L1 行业标准（Torznab/RSS attrs·冻结真契约→可共享组件）/ L2 M-Team 方言家族（营销级兼容非契约）/ L3 站点特有（per-site）。
- HDAreaAdapter 独立（framework=hdarea）·自有轻量 client·**不 import mtorrentCore**。
- 共享仅发生在机制层：`internal/adapter/apihelper.go`（X-API-Key 注入单点+短效凭证下载流程参数化+可选 429 退避）——字段映射/字典/端点常量全留各方言层。包内私有（消费者仅 adapter 包内·三次法则后再升 util）。
- mtorrentCore 的 mt↔cola 快照耦合维持 §59.460 既定策略（分叉时演进·馒头改协议时 cola fork 薄 client）；好大从第一天不进共享体。
- 回退阀：API-only+sites.framework 字段应急回退（updateSiteRequest.Framework 指针可编辑·已验证）——代码零双路。mt 的 viaWeb 五分支=历史沉积准死代码（API-only 政策后不可达）·不复制·清理另行挂账。

**刀序定案（回归检查修正版）**：

| 刀 | 范围 | 验证 |
|----|------|------|
| 刀1（含刀0 前置） | ①传输诊断（29 Go 直连超时·TLSPretend 应急）②活体探针 11 项→真实夹具 ③HDAreaAdapter **14 方法接口全集**（五获取+基础 API 上传+RSS apikey+SL×2 空实现+DetectHR false+InfoHash/VerifyExists 薄实现+SupportsPiecesHash=false）④apihelper.go 机制层 ⑤framework 注册+存量迁移（sites.framework/form_config.framework np→hdarea 跟随 mteam 惯例/auth_type=apikey/fetch_rps=0.8）⑥双 JSON seed 同步（§59.512）⑦RSS 策略表注册 | 29 端到端获取一颗+DryRun 发布一颗粒子+夹具锚 |
| 刀2 | 发布深化：check 预查重接入 dedup 链（清洗重算+30min 保护期）/断种顶替=四终态新语义（设计讨论）/409 existing_id↔四终态/成功判定 code===0+IDSource="api_response"/source_sel(6 值)+processing_sel(Raw/Encode) 补配 | DryRun 借壳 PT0 素材+生产首颗挂用户 |
| 刀3 | RSS 增强（promotion_until→FreeEndAt）/hashes 200 批辅种（QueryIYUULive 同构注入先例） | RSS 一轮+辅种单簇 |
| 刀4 | site/metadata 值集自愈（formdiff 体系新源）/return_torrent_base64 直推/web 链退役决策 | 值集漂移演练 |

**明确不接**：Torznab 协议（索引器工具向·我们搜索语义全在 REST）/user 互动域（白名单外）/字幕上传与 edit（刀2 后按需）/三协议全接（按链路各取其一）。

**记录不阻塞**：发布等级前置（刀2 首颗验证）/auto_ptgen 不传（自带 PTGen）/令牌=PT-Forward 子令牌（UI 填入·密钥不落码）。

## 11.5 刀1 活体探针终局（2026-11-12·11/11 全通·夹具已入库）

| # | 项 | 结果 |
|---|----|------|
| 1 | /api/v1/status | ✓ 身份/权限/三协议入口（gentoken 独立信封 {ok,dl_token,expires_in}） |
| 2 | 搜索 | ✓ `search=` 参数命中（keyword= 不生效）；统一信封 {success,code,data}；item 含分类 ID 全家桶/sp_state 促销/info_hash/imdb+豆瓣+poster |
| 3 | 详情 | ✓ 同 search 字段+save_as/descr_bbcode/descr_html/nfo/files/other_copies(30) |
| 4 | genDlToken | ✓ 1800s+download_url 直链→GET 真种子 84KB（d8:announce） |
| 5 | hashes | ✓ {count,items}（假 hash 零命中） |
| 6 | site/metadata | ✓ 10 域字典 |
| 7 | torznab caps | ✓ 2690B XML |
| 8 | apikey RSS | ✓ guid=**infohash 真身**（零侧载）·enclosure=apikey 直链·title 带 [免费] 前缀 |
| 9 | allow_transfer | **✓ 反转发现**：search/detail item 均有 `forbid_transfer` 字段——好大 API 有禁转体系（web 面无标记=隐藏字段·hdarea.md 负结论已修正）；adapter 映射 Flags["禁转"]→R7 消费 |
| 10 | pieces-hash | 无该参数记载·初版 SupportsSearchByPiecesHash=false |
| 11 | DetectDiscount | search/detail 响应 sp_state 族+promotion_until+双 factor 直读 |

传输诊断：站点栈对 API 路径 0.8s 通（前日超时为间歇性）；裸 Go 栈 TLS 被 CF 掐（站点栈 smart dialer 必要）。

## 12. 刀1 交付（2026-11-12·v0.0.1446）

- `internal/adapter/apihelper.go`：机制层（setXAPIKey+shortTokenDownload 流程骨架·403 过期重签一次）
- `internal/adapter/adapter_hdarea.go`：HDAreaAdapter 14 方法接口全集（五获取+基础表单驱动 API 上传+ParseRSS 委托 Generic stub+SL×2 空+DetectHR false+InfoHash/VerifyExists 薄+PiecesHash false）
- Factory `case "hdarea"`；migration159（framework/auth_type=apikey/fetch_rps=0.8/form_config.framework→hdarea）；双 JSON seed 同步；前端 frameworkLabels+HDArea API
- RSS=零 adapter 工作（引擎标准 NP 解析·guid 真身·ParseRSS 走 Generic stub——mteam 同款）
- 锚六枚（真实消毒夹具 testdata/hdarea/）：Search/Detail（MI 提取+forbid_transfer）/Discount/Stats/ThinStubs/InfoHash
- 禁转修正：API `forbid_transfer` → Flags["禁转"]（web 负结论被探针⑨反转）
- 凭证 UX 定案（2026-11-12 用户）：apikey+hdarea 下凭证区只显 API Key（Cookie/Passkey 条件隐藏=设计行为·DB 存量保留）——**不加解释文案**：用户无需知晓 Cookie/Passkey/API 关系，按表单要求填写即可；回退阀=framework+auth_type 两字段一对改回（UI 自动恢复显示）

## 11. 待实测确认清单（刀1 活体探针·11 项·read/download scope 零写操作）

1. `/api/v1/status` 响应形态（鉴权探活基线）
2. `/api/v1/torrents` 搜索响应 JSON（分页字段/条目字段名/排序参数实测）
3. `/api/v1/torrents/{id}` 详情响应（descr BBCode 结构/NFO/文件列表/其它版本字段/**allow_transfer 在场性**——R7 消费判定）
4. `genDlToken` 请求响应形态（与 mtorrentCore 同名端点对照差异）
5. `/api/v1/torrents/hashes` 响应结构（辅种匹配字段）
6. `/api/v1/site/metadata` 响应（值集字典形态）
7. `torznab.php?t=caps` XML（分类映射）
8. apikey RSS 实测（现有 NP RSS 解析器对 torznab:attr/promotion_until 扩展节点的容忍度）
9. detail 响应 allow_transfer 字段在场性（他人禁转声明·在场则 R7 消费）
10. pieces-hash 搜索能力（SupportsSearchByPiecesHash 定值依据）
11. DetectDiscount 数据源（无独立端点——search/detail 响应哪个字段带促销）
