# 馒头 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 馒头|
| Web 前端 | https://kp.m-team.cc |
| API 域名 | https://api.m-team.io |
| 站点框架 | **mTorrent（自研 SPA + REST API）**，非 NexusPHP |
| 认证方式 | `x-api-key` header（API Key）+ `authorization` header（JWT） |
| Swagger 文档 | https://test2.m-team.cc/api/swagger-ui/index.html（测试环境） |

**站点角色**: 源站 + 发布站（可同时作为源站和目标站）。

**⚠ 成人版块禁止规则（§30.3）**：
- 馒头包含 4 个成人父分类：AV(有码)(115)、AV(无码)(120)、IV 写真(445)、H-ACG(446)，共 15 个成人子分类
- API 搜索 mode=`adult` 分组包含 15 个成人子分类 ID：410, 429, 424, 430, 426, 437, 431, 432, 436, 425, 433, 411, 412, 413, 440
- **禁止下载和发布这些分类的资源**，YAML 配置 `skip_categories` + `skip_modes: ["adult"]`
- **作为源站时**：MTeamFetcher 搜索 API 永远不传 `mode: "adult"`，且搜索结果命中 skip 列表的种子直接丢弃（§31.10.9）
- 启动时由 RequiredSkips 审计该配置存在性，缺失则拒绝启动（§30.4.2）

---

## 一、API 认证

### 1.1 双重认证机制

M-Team API 使用两种认证方式，请求需**同时携带**：

| Header | 说明 | 获取方式 |
|--------|------|----------|
| `x-api-key` | 站点颁发的 API Key（UUID 格式） | 用户设置页面生成 |
| `authorization` | JWT Token（登录后获取） | 登录认证返回 |

- API Key 是长期有效的静态凭证，但可被系统停用（异常使用检测）
- JWT Token 有过期时间（exp 字段），需要定期刷新
- **生产环境 API 从部分 IP 段直接访问会被 nginx 拦截（302→Google），需通过浏览器环境（Playwright）中转请求**

### 1.2 API 基础信息

| 项目 | 内容 |
|------|------|
| 协议 | HTTPS POST（所有端点均为 POST） |
| Content-Type | application/json |
| 响应格式 | `{"code": "0"/0, "message": "SUCCESS", "data": ...}` |
| 错误码 | code=1 业务错误（如 key無效）、code=401 认证失败 |
| 分页 | pageNumber + pageSize（search 端点） |

### 1.3 关键域名

| 用途 | 域名 |
|------|------|
| Web 前端 | kp.m-team.cc |
| API 主域名 | api.m-team.io |
| 备用域名 | zp.m-team.io, ob.m-team.cc |
| 测试环境 | test2.m-team.cc |
| RSS | rss.m-team.cc / rss.m-team.io |

---

## 二、分类系统（categoryList）

### 2.1 父分类 — 12个（API 验证）

| ID | 中文名 | 英文名 | order | 说明 |
|----|--------|--------|-------|------|
| 100 | 电影 | Movie | 1 | 主分类 |
| 105 | 影剧/综艺 | TV Series | 2 | 主分类 |
| 444 | 紀錄 | BBC | 3 | 主分类 |
| 110 | Music | Music | 4 | 主分类 |
| 447 | 遊戲 | 遊戲 | 6 | 主分类 |
| 449 | 動漫 | Anime | 7 | 主分类 |
| 443 | 教育 | Education | 5 | 主分类（新增） |
| 450 | 其他 | 其他 | 8 | 杂项父分类 |
| 115 | AV(有码) | AV(有碼) | 20 | 成人内容 |
| 120 | AV(无码) | AV(無碼) | 21 | 成人内容 |
| 445 | IV | IV | 22 | 写真 |
| 446 | H-ACG | H-ACG | 23 | 成人动漫 |

### 2.2 子分类 — 36个（API 验证）

> 以下子分类数量不含成人区 15 个（成人区单独列出）。

#### 电影（parent=100）— 5个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 401 | 电影/SD | Movie/SD | 1 |
| 419 | 电影/HD | Movie/HD | 2 |
| 420 | 电影/DVDiSo | Movie/DVDiSo | 3 |
| 421 | 电影/Blu-Ray | Movie/Blu-Ray | 4 |
| 439 | 电影/Remux | Movie/Remux | 5 |

#### 影剧/综艺（parent=105）— 4个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 403 | 影剧/综艺/SD | TV Series/SD | 1 |
| 402 | 影剧/综艺/HD | TV Series/HD | 2 |
| 438 | 影剧/综艺/BD | TV Series/BD | 3 |
| 435 | 影剧/综艺/DVDiSo | TV Series/DVDiSo | 4 |

#### Music（parent=110）— 2个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 434 | Music(无损) | Music(Lossless) | 1 |
| 408 | Music(AAC/ALAC) | Music(AAC/ALAC) | 2 |

#### 紀錄（parent=444）— 1个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 404 | 纪录 | Record | 1 |

#### 教育（parent=443）— 3个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 427 | 教育書面 | education book | 1 |
| 441 | 教育(影片) | edu Video | 2 |
| 442 | 教育(音檔) | edu Audio | 3 |

#### 遊戲（parent=447）— 2个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 423 | PC游戏 | PCGame | 1 |
| 448 | TV遊戲 | TvGame | 2 |

#### 動漫（parent=449）— 1个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 405 | 动画 | Anime | 1 |

#### 其他（parent=450）— 4个子分类

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 407 | 运动 | Sports | 1 |
| 422 | 软件 | Software | 2 |
| 409 | Misc(其他) | Misc(Other)  | 3 |

#### 成人区子分类 — ⛔ 全局禁止（§30.3）

> 以下 15 个子分类属于成人内容，被全局策略屏蔽，**禁止下载和发布**。
> 数据仅作逆向参考，代码层面不可触及。
> 数据来源：API `categoryList` 实际返回验证。

##### AV(有码) — parent=115

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 410 | AV(有码)/HD Censored | AV(有碼)/HD Censored | 31 |
| 424 | AV(有码)/SD Censored | AV(有碼)/SD Censored | 33 |
| 437 | AV(有码)/DVDiSo Censored | AV(有碼)/DVDiSo Censored | 36 |
| 431 | AV(有码)/Blu-Ray Censored | AV(有碼)/Blu-Ray Censored | 37 |

##### AV(无码) — parent=120

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 429 | AV(无码)/HD Uncensored | AV(無碼)/HD Uncensored | 32 |
| 430 | AV(无码)/SD Uncensored | AV(無碼)/SD Uncensored | 34 |
| 426 | AV(无码)/DVDiSo Uncensored | AV(無碼)/DVDiSo Uncensored | 35 |
| 432 | AV(无码)/Blu-Ray Uncensored | AV(無碼)/Blu-Ray Uncensored | 38 |
| 436 | AV(网站)/0Day | AV(網站)/0Day | 39 |
| 440 | AV(Gay)/HD | AV(Gay)/HD | 440 |

##### IV(写真) — parent=445

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 425 | IV(写真影集) | IV/Video Collection | 40 |
| 433 | IV(写真图集) | IV/Picture Collection | 41 |

##### H-ACG — parent=446

| ID | 中文名 | 英文名 | order |
|----|--------|--------|-------|
| 411 | H-游戏 | H-Game | 51 |
| 412 | H-动漫 | H-Anime | 52 |
| 413 | H-漫画 | H-Comic | 53 |

### 2.3 预定义分组

API 返回的 `data` 中包含预定义分类分组：

| 分组 key | 包含的分类 ID | 用途 |
|----------|--------------|------|
| `movie` | 401, 419, 420, 421, 404, 439 | 电影模式（含纪录/Remux） |
| `tvshow` | 403, 402, 435, 438 | 剧集模式 |
| `music` | 406, 434 | 音乐模式 |
| `adult` | 410, 429, 424, 430, 426, 437, 431, 432, 436, 440, 425, 433, 411, 412, 413 | 成人模式（15 个） |
| `waterfall` | 上述全部 34 个子分类（含成人） | 瀑布流模式 |
| `list` | 完整分类对象数组 | 分类管理 |

---

## 三、视频编码（videoCodecList）— 8个

| ID | 名称 | order | 说明 |
|----|------|-------|------|
| 1 | H.264(x264/AVC) | 1 | 最常用 |
| 16 | H.265(x265/HEVC) | 2 | 次常用 |
| 2 | VC-1 | 3 | |
| 4 | MPEG-2 | 7 | |
| 3 | Xvid | 8 | 老格式 |
| 19 | AV1 | 9 | 新格式 |
| 21 | VP8/9 | 10 | Google 编码 |
| 22 | AVS | 11 | 中国自主编码标准 |

**注意**: ID 编号不连续（1,2,3,4,16,19,21,22），编码名称直接标注常用别名（如 `H.264(x264/AVC)`）。

---

## 四、音频编码（audioCodecList）— 15个

| ID | 名称 | order | 说明 |
|----|------|-------|------|
| 6 | AAC | 1 | |
| 8 | AC3(DD) | 2 | |
| 3 | DTS | 3 | |
| 11 | DTS-HD MA | 4 | |
| 12 | E-AC3(DDP) | 5 | Dolby Digital Plus |
| 13 | E-AC3 Atoms(DDP Atoms) | 6 | Dolby Atmos via DDP |
| 9 | TrueHD | 7 | |
| 10 | TrueHD Atmos | 8 | Dolby Atmos via TrueHD |
| 14 | LPCM/PCM | 9 | |
| 15 | WAV | 19 | |
| 1 | FLAC | 20 | 无损音频 |
| 2 | APE | 21 | 无损音频 |
| 4 | MP2/3 | 22 | |
| 5 | OGG | 23 | |
| 7 | Other | 99 | 兜底 |

**注意**: ID 编号不连续；音频编码分两组 order 范围：1-9（视频音轨）和 19-23（独立音频文件）。

---

## 五、来源（sourceList）— 7个

| ID | 名称 | order |
|----|------|-------|
| 8 | Web-DL | 1 |
| 1 | Bluray | 2 |
| 4 | Remux | 3 |
| 5 | HDTV/TV | 5 |
| 3 | DVD | 6 |
| 10 | CD | 7 |
| 6 | Other | 99 |

**注意**: 无测试环境中的 `Encode(9)` 来源，`CD` 的 ID 从 7 变为 10。`order` 也不同（测试环境 Encode 排第4，生产环境无此选项）。

---

## 六、媒介（mediumList）— 10个

| ID | 名称 | order |
|----|------|-------|
| 1 | Blu-ray | 0 |
| 2 | HD DVD | 1 |
| 3 | Remux | 2 |
| 7 | Encode | 3 |
| 4 | MiniBD | 4 |
| 5 | HDTV | 5 |
| 6 | DVDR | 6 |
| 8 | CD | 7 |
| 10 | Web-DL | 8 |
| 9 | Track | 9 |

**重要**: `source` 和 `medium` 是两个**独立字段**：
- `source` 表示来源类型（Web-DL/Blu-ray/Remux/HDTV/DVD/CD/Other）
- `medium` 表示物理媒介格式（Blu-ray/HD DVD/Remux/Encode/MiniBD/HDTV/DVDR/CD/Web-DL/Track）
- 例如一个种子可以 `source=8(Web-DL)` + `medium=10(Web-DL)`，也可以 `source=1(Blu-ray)` + `medium=3(Remux)`

---

## 七、分辨率（standardList）— 6个

| ID | 名称 | order |
|----|------|-------|
| 1 | 1080p | 1 |
| 2 | 1080i | 2 |
| 3 | 720p | 3 |
| 5 | SD | 4 |
| 6 | 4K | 5 |
| 7 | 8K | 6 |

**注意**: ID 4 缺失（无 1440p）；SD 的 ID=5（非标准的按分辨率命名）。

---

## 八、地区/处理（processingList）— 6个

| ID | 名称 | order |
|----|------|-------|
| 1 | CN | 1 |
| 2 | US/EU | 1 |
| 3 | HK/TW | 1 |
| 4 | JP | 1 |
| 5 | KR | 1 |
| 6 | OT | 1 |

所有 order=1（无排序优先级）。使用英文缩写（CN/US/EU/HK/TW/JP/KR/OT）。

---

## 九、制作组（teamList）— 23个

| ID | 名称 | order | freeOffer |
|----|------|-------|-----------|
| 9 | MTeam | 1 | true |
| 23 | TnP | 3 | false |
| 44 | MWeb | 3 | - |
| 43 | TPTV | 3 | - |
| 6 | BMDru | 6 | false |
| 19 | CNHK | 8 | true |
| 8 | Pack | 10 | false |
| 25 | CatEDU | 11 | true |
| 26 | ARiC | 12 | true |
| 30 | 7³ACG | 14 | true |
| 34 | QHstudIo | 15 | true |
| 31 | JKCT | 16 | true |
| 35 | G00DB0Y | 17 | true |
| 36 | D0 | 18 | true |
| 57 | DST | 20 | - |
| 40 | HBO | 22 | - |
| 41 | REE | 23 | - |
| 45 | CTRL | 25 | - |
| 59 | StarfallWeb | 26 | - |
| 48 | ZTR | 28 | - |
| 49 | 126811 | 29 | - |
| 61 | RRS | 30 | - |
| 62 | lijiang-tv | 31 | - |

**注意**: 测试环境仅 20 个制作组，生产环境 23 个（多出 MWeb/TPTV/DST/StarfallWeb/RRS/lijiang-tv 等）。`freeOffer=true` 表示该制作组发布的种子有免费优惠。

---

## 十、上传 API（createOredit）

### 10.1 端点

```
POST /api/torrent/createOredit
Content-Type: multipart/form-data（含文件上传）
```

### 10.2 请求参数（TorrentUploadForm）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string(255) | **是** | 标题 |
| `category` | int64 | **是** | 分类 ID（见第二章） |
| `descr` | string(65535) | **是** | 简介（BBCode） |
| `anonymous` | boolean | **是** | 匿名发布 |
| `smallDescr` | string(255) | 否 | 副标题 |
| `source` | int64 | 否 | 来源 ID（见第五章） |
| `medium` | int64 | 否 | 媒介 ID（见第六章） |
| `standard` | int64 | 否 | 分辨率 ID（见第七章） |
| `videoCodec` | int64 | 否 | 视频编码 ID（见第三章） |
| `audioCodec` | int64 | 否 | 音频编码 ID（见第四章） |
| `team` | int64 | 否 | 制作组 ID（见第九章） |
| `processing` | int64 | 否 | 地区 ID（见第八章） |
| `countries` | string | 否 | 国家/地区 |
| `imdb` | string | 否 | IMDb ID/链接 |
| `douban` | string | 否 | 豆瓣链接 |
| `dmmCode` | string | 否 | DMM 编号（成人内容） |
| `cids` | string | 否 | 专辑 ID 列表 |
| `aids` | string | 否 | 艺人 ID 列表 |
| `labels` | int32 | 否 | 标签（位掩码） |
| `labelsNew` | string | 否 | 新标签系统 |
| `tags` | string | 否 | 自定义标签 |
| `file` | binary | 否 | 种子文件 |
| `nfo` | binary | 否 | NFO 文件 |
| `mediainfo` | string | 否 | MediaInfo 文本 |
| `mediaInfoAnalysisResult` | boolean | 否 | MediaInfo 解析结果 |
| `torrent` | int64 | 否 | 编辑模式：种子 ID |
| `offer` | int64 | 否 | 候选 ID |

### 10.3 搜索参数（TorrentSearch）

| 字段 | 类型 | 说明 |
|------|------|------|
| `mode` | string enum | `normal`/`adult`/`movie`/`music`/`tvshow`/`waterfall`/`rss`/`rankings`/`all` |
| `keyword` | string(100) | 搜索关键词 |
| `categories` | int64[] | 分类 ID 数组 |
| `sources` | int64[] | 来源 ID 数组 |
| `mediums` | int64[] | 媒介 ID 数组 |
| `standards` | int64[] | 分辨率 ID 数组 |
| `videoCodecs` | int64[] | 视频编码 ID 数组 |
| `audioCodecs` | int64[] | 音频编码 ID 数组 |
| `teams` | int64[] | 制作组 ID 数组 |
| `processings` | int64[] | 地区 ID 数组 |
| `countries` | int64[] | 国家 ID 数组 |
| `imdb` | string | IMDb 筛选 |
| `douban` | string | 豆瓣筛选 |
| `pageNumber` | int32 | 页码（1-1000） |
| `pageSize` | int32 | 每页条数（1-200） |
| `lastId` | int64 | 游标分页（上次最后一条 ID） |

### 10.4 促销类型（discount/spstate）

搜索筛选中的 `discount` 和管理修改中的 `spstate` 共享以下枚举值：

| 值 | 含义 |
|----|------|
| NORMAL | 正常 |
| PERCENT_70 | 7折 |
| PERCENT_50 | 5折 |
| FREE | 免费下载 |
| _2X_FREE | 2x 免费 |
| _2X | 2x 上传 |
| _2X_PERCENT_50 | 2x 5折 |

---

## 十一、M-Team 特殊注意事项

### 11.1 非 NexusPHP 架构

M-Team 是已采集站点中唯一使用自研 SPA + REST API 架构的站点（非 NexusPHP、非 UNIT3D、非 Gazelle）。这意味着：

- **无 HTML 表单可解析**——所有交互通过 JSON API
- 字段值使用**数字 ID**（与 NexusPHP 类似），但通过 API 端点动态获取
- 认证使用 `x-api-key` + JWT 双重机制，非 cookie/session
- 种子上传使用 `multipart/form-data`，但参数通过 JSON schema 定义

### 11.2 source 与 medium 双字段

M-Team 同时维护 `source`（来源类型）和 `medium`（媒介格式）两个独立字段，这在 NexusPHP 站点中较罕见（大多数站点只有其中一个）。适配器需要同时映射两个字段。

### 11.3 搜索模式（mode）

搜索 API 支持 9 种 mode（normal/adult/movie/music/tvshow/waterfall/rss/rankings/all），不同 mode 下可见的分类不同。`waterfall` 模式展示所有分类（瀑布流），`normal` 仅展示非成人分类。

### 11.4 制作组 freeOffer

部分制作组（如 MTeam/CNHK/CatEDU/ARiC 等）的种子自动享受免费优惠。适配器在发布时可参考此信息。

### 11.5 IP 风控

生产环境 API 对部分 IP 段做了拦截（nginx 层 302→Google），需要通过浏览器环境（Playwright headless）中转请求，或在受信任的网络环境中调用。

---

> **API 调用方法**详见 [25-M-Team-API完整指南.md](../25-M-Team-API完整指南.md) 「API 调用方法（开发参考）」章节。

---

## 十二、与其他站点对比

| 特征 | M-Team | NexusPHP 站点 |
|------|--------|---------------|
| 框架 | **mTorrent（自研 SPA+API）** | NexusPHP（服务端渲染） |
| 发布接口 | **REST API（JSON）** | HTML 表单（POST） |
| 认证 | **x-api-key + JWT** | Cookie/Session |
| 分类获取 | **API 动态获取** | HTML 解析 |
| 分类数量 | **47个（11父+36子）** | 通常 5-15 个 |
| 视频编码 | 8个（含 VP8/9、AVS） | 通常 5-7 个 |
| 音频编码 | **15个** | 通常 6-12 个 |
| 制作组 | **23个** | 通常 3-30 个 |
| source+medium | **双字段独立** | 通常单字段 |
| 地区 | 6个（CN/US-EU/HK-TW/JP/KR/OT） | 部分站有 |
| 分辨率 | 6个（含 8K） | 通常 4-5 个 |

---

## 十三、适配器实现要点

### 13.1 API 客户端

```go
type MTeamClient struct {
    APIKey       string
    JWTToken     string
    BaseURL      string
    Playwright   *playwright.BrowserContext
}
```

### 13.2 列表数据缓存

所有 list 端点返回的数据较稳定，可在启动时获取并缓存：

```go
func (c *MTeamClient) FetchAllLists() error {
    for _, ep := range []string{
        "categoryList", "videoCodecList", "audioCodecList",
        "sourceList", "mediumList", "standardList",
        "processingList", "teamList",
    } {
        // POST /api/torrent/{ep}
    }
    return nil
}
```

### 13.3 发布流程

```go
func (c *MTeamClient) Upload(form *TorrentUploadForm) error {
    // POST /api/torrent/createOredit
    // Content-Type: multipart/form-data
    // Headers: x-api-key + authorization
}
```

### 13.4 分类映射

```go
func mapMTeamCategory(standardCategory string) int64 {
    switch standardCategory {
    case "Movie/HD":    return 419
    case "Movie/SD":    return 401
    case "Movie/BluRay": return 421
    case "Movie/Remux": return 439
    case "TV/HD":       return 402
    case "TV/SD":       return 403
    case "Anime":       return 405
    case "Music/Lossless": return 434
    case "Music/MV":    return 406
    case "Documentary": return 404
    default:            return 0
    }
}
```

---

*数据来源: 生产环境 API（api.m-team.io）2026-04-16 via Playwright；测试环境 Swagger API spec（test2.m-team.cc）*
*文档创建: 2026-04-16*
