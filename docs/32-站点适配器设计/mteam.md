# 馒头 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 馒头|
| Web 前端 | https://kp.m-team.cc |
| API 域名 | https://api.m-team.cc（主） / https://api.m-team.io（备用） |
| 站点框架 | **mTorrent（自研 SPA + REST API）**，非 NexusPHP |
| 认证方式 | `x-api-key` header（API Key）+ `authorization` header（JWT，仅浏览器前端） |
| Swagger 文档 | https://test2.m-team.cc/api/swagger-ui/index.html（测试环境，OpenAPI 3.1.0） |
| 不允许第三方调用 | `/login`、`/admin/**`、`/apikey/**` |

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
- **生产环境 API 需携带浏览器 User-Agent**，否则 Cloudflare WAF 拦截（302→Google 或 403）
- 本机测试确认：`-H "User-Agent: Mozilla/5.0 ..." -H "x-api-key: <KEY>"` 可直连 `api.m-team.cc`，无需代理

### 1.2 API 基础信息

| 项目 | 内容 |
|------|------|
| 协议 | HTTPS POST（绝大多数），少量 GET |
| Content-Type | application/json（绝大多数）；`detail`/`genDlToken` 使用 form-urlencoded；`createOredit` 使用 multipart/form-data |
| 响应格式 | `{"code": 0, "message": "SUCCESS", "data": ...}`（code 可为 int/string） |
| 错误码 | code=1 业务错误（如 key無效）、code=401 认证失败 |
| 分页 | pageNumber + pageSize（search 端点） |

### 1.3 关键域名

| 用途 | 域名 |
|------|------|
| Web 前端 | kp.m-team.cc |
| API 主域名 | api.m-team.cc |
| API 备用1 | api.m-team.io |
| API 备用2 | **api2.m-team.cc**（SPA 前端代码 `_APIHOSTS` 中发现） |
| Failover 域名 | zp.m-team.io, xp.m-team.cc, ap.m-team.cc, next.m-team.cc, ob.m-team.cc |
| 静态资源 | static.m-team.cc |
| 测试环境 | test2.m-team.cc |
| RSS | rss.m-team.cc / rss.m-team.io |
| Wiki 文档 | wiki.m-team.cc |

> **SPA 前端 API 发现机制**：`kp.m-team.cc` 的 HTML 中嵌入了 `_APIHOSTS` 变量，包含所有可用 API 端点：
> ```javascript
> _APIHOSTS = "https://api.m-team.cc/api,https://api.m-team.io/api,https://api2.m-team.cc/api".split(",")
> ```

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

| 字段 | 类型 | 必填 | 页面显示名 | 说明 |
|------|------|------|-----------|------|
| `name` | string(255) | 否* | 標題 | 若不填將使用種子檔案名。示例：`Lisa Frankenstein 2024 BluRay 1080p AVC DTS-HD MA5.1-ESiR` |
| `category` | int64 | **是** | 類別 | 分类 ID（见第二章） |
| `descr` | string(65535) | **是** | 簡介 | 富文本编辑器（BBCode） |
| `anonymous` | boolean | **是** | 匿名發表 | checkbox |
| `smallDescr` | string(255) | 否 | 副標題 | 示例：`720p @ 4615 kbps - DTS 5.1 @ 1536 kbps` |
| `source` | int64 | 否 | _(未在页面显示)_ | 来源 ID（见第五章），可能由系统根据 category 自动推断 |
| `medium` | int64 | 否 | _(未在页面显示)_ | 媒介 ID（见第六章），可能由系统根据 category 自动推断 |
| `standard` | int64 | 否 | 解析度 | 分辨率 ID（见第七章） |
| `videoCodec` | int64 | 否 | 視頻編碼 | 视频编码 ID（见第三章） |
| `audioCodec` | int64 | 否 | 音頻編碼 | 音频编码 ID（见第四章） |
| `team` | int64 | 否 | 製作組 | 制作组 ID（见第九章），含"申請入駐"链接 |
| `processing` | int64 | 否 | _(未在页面显示)_ | 地区 ID（见第八章） |
| `countries` | string | 否 | 國家/地區 | 国家/地区（最多 3 个，显示 `0 / 3`） |
| `imdb` | string(url) | 否 | IMDb 鏈接 | 旁有"獲取簡介"按钮，可自动拉取。示例：`https://www.imdb.com/title/tt0111161/` |
| `douban` | string(url) | 否 | 豆瓣鏈接 | 旁有"獲取簡介"按钮，可自动拉取。示例：`https://movie.douban.com/subject/1292052/` |
| `dmmCode` | string | 否 | _(搜索选择框)_ | DMM 编号（成人内容） |
| `cids` | string | 否 | _(未在页面显示)_ | 专辑 ID 列表 |
| `aids` | string | 否 | _(未在页面显示)_ | 艺人 ID 列表 |
| `labels` | int32 | 否 | 標記 | 位掩码：bit0=DIY, bit1=中配, bit2=中字（见 §11.8） |
| `labelsNew` | string | 否 | _(未在页面显示)_ | 新标签系统 |
| `tags` | string | 否 | tags | 自定义标签（搜索选择框） |
| `file` | binary | 否* | 選擇種子 | 种子文件（*name 为空时必填） |
| `nfo` | binary | 否 | _(未在页面显示)_ | NFO 文件 |
| `mediainfo` | string | 否 | MediaInfo 檔案 | textarea，提示：「請貼上Mediainfo/BDInfo 解析的資訊(英語並且是text 格式)」 |
| `mediaInfoAnalysisResult` | boolean | 否 | _(系统内部)_ | MediaInfo 解析结果 |
| `torrent` | int64 | 否 | _(编辑模式)_ | 编辑模式：种子 ID |
| `offer` | int64 | 否 | _(候选模式)_ | 候选 ID |
| `black_node` | boolean | 否 | black_node | checkbox，功能待确认（§25 API 文档未记录） |

> **页面确认框**：发布前需勾选"我已經閱讀過規則"，但此为前端约束，API 层面无此字段。

> **自动推断字段**：`source` 和 `medium` 在上传页面中**未显示**，可能由系统根据 `category` 自动推断（如 category=421→source=Bluray, medium=Blu-ray）。

> **獲取簡介功能**：填入 IMDb/豆瓣链接后，点击"獲取簡介"按钮，SPA 自动拉取影片信息填充简介编辑器。

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

### 11.5 Cloudflare WAF 与 User-Agent 要求

生产环境 API 域名（`api.m-team.cc` 等）受 Cloudflare WAF 保护：

- **不带 User-Agent 或使用 curl 默认 UA** → 302 重定向到 Google（被拦截）
- **带浏览器 User-Agent** → 正常返回（200）
- 静态资源（`kp.m-team.cc/`、`static.m-team.cc/`）不受影响
- `wiki.m-team.cc`（Outline Wiki）不受影响
- 测试环境 `test2.m-team.cc` 不受影响

**适配器必须**在所有 API 请求中携带浏览器 UA。

### 11.6 detail 端点使用 form-urlencoded（非 JSON）

`/api/torrent/detail` 端点**必须使用 form-urlencoded 格式**，与 `genDlToken` 相同：

```
POST /api/torrent/detail
Content-Type: application/x-www-form-urlencoded
Body: id=12345
```

> ⚠ §25 API 指南中记录的是 JSON 格式，但实际代码（Go driver + Rust adapter）均使用 form-urlencoded。以实际代码为准。

### 11.7 生产环境推荐请求头

除 `x-api-key` 外，生产环境建议携带以下 header（来源于 Python crawler 实际使用）：

| Header | 值 | 说明 |
|--------|-----|------|
| `x-api-key` | `<API Key>` | 必须 |
| `Content-Type` | `application/json; charset=utf-8` | JSON 端点 |
| `Accept` | `application/json, text/plain, */*` | 推荐 |
| `Origin` | `https://kp.m-team.cc` | 推荐 |
| `User-Agent` | Chrome UA 字符串 | 推荐 |
| `version` | `1.1.4` | 前端版本号（可选） |
| `did` | `<device_id>` | 设备标识（可选） |

> 注：Vertex JS 实现仅使用 `x-api-key`，不携带 JWT `authorization` header，也能正常调用。说明 **API Key 是唯一必须的认证凭证**，JWT 仅用于浏览器前端登录态。

### 11.8 labels 位掩码

搜索结果和种子详情中的 `labels` 字段是 **int32 位掩码**，各位含义：

| 位 | 值 | 含义 | 页面显示名 |
|----|-----|------|-----------|
| bit 0 | 1 | DIY | _(未在上传页显示)_ |
| bit 1 | 2 | 中文配音 | 中配 |
| bit 2 | 4 | 中文字幕 | 中字 |

判断示例：
```go
if labels & 1 != 0 { /* DIY */ }
if labels & 2 != 0 { /* 中配 */ }
if labels & 4 != 0 { /* 中字 */ }
```

> 注：Vertex JS 代码中注释为"国配"，但上传页面实际显示为"中配"。以页面为准。

### 11.9 搜索结果完整字段

搜索结果（`MTorrentTorrent`）除已记录的 id/name/smallDescr/size/createdDate/status/category 外，**实际还包含以下字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `videoCodec` | string | 视频编码 ID（如 "1" = H.264） |
| `audioCodec` | string | 音频编码 ID（如 "6" = AAC） |
| `standard` | string | 分辨率 ID（如 "1" = 1080p, "6" = 4K） |
| `imdb` | string | IMDb 链接 |
| `douban` | string | 豆瓣链接 |
| `labels` | int | 位掩码（见 §11.8） |

这些字段在 Python crawler 的 `_map_item` 方法中已使用，可用于精确匹配而无需解析标题。

### 11.10 促销类型完整枚举

除文档第十章记录的 7 种促销类型外，Rust adapter 实际处理了**额外 4 种**：

| 值 | 含义 | 文档是否记录 |
|----|------|-------------|
| `NORMAL` | 正常 | ✅ |
| `PERCENT_70` | 7 折 | ✅ |
| `PERCENT_50` | 5 折 | ✅ |
| `FREE` | 免费 | ✅ |
| `_2X_FREE` | 2x 免费 | ✅ |
| `_2X` | 2x 上传 | ✅ |
| `_2X_PERCENT_50` | 2x 5 折 | ✅ |
| `FREE_2XUP` / `TWOFREE` | 免费 + 2x 上传 | ❌ **未记录** |
| `PERCENT_50_2XUP` | 5 折 + 2x 上传 | ❌ **未记录** |
| `PERCENT_70_2XUP` | 7 折 + 2x 上传 | ❌ **未记录** |

### 11.11 视频编码 ID 18（AVC1）

Python crawler 映射中包含 ID=18（AVC1），为 H.264 的旧版/变体编码，出现在非标准内容中。API 文档的 `videoCodecList` 未列出此 ID，但搜索结果可能返回。

映射：`18 → x264`（按 Python crawler 处理方式）

### 11.12 分类 ID 差异

Go driver `mteamCategoryMap` 中有若干 ID 名称与 API `categoryList` 返回值不一致：

| ID | API categoryList 名称 | Go driver 名称 | 说明 |
|----|----------------------|---------------|------|
| 427 | 教育書面 / education book | 电子书 | 同一 ID，不同名称 |
| 442 | 教育(音檔) / edu Audio | 有声书 | 同一 ID，不同名称 |
| 451 | **未出现在 API 文档中** | 教育影片 | Go driver 独有 |
| 406 | **未出现在 API 文档子分类中** | 演唱会 | Go driver/Python crawler 均使用 |

> 451（教育影片）可能是后续新增的分类。406（演唱会）在 Vertex JS 和 Python crawler 中均有使用，但 API 文档 `categoryList` 中仅列出 Music(Lossless)=434 和 Music(AAC/ALAC)=408。

---

## 十一B、用户等级体系

> 数据来源: `definitions/mteam.go` + `mtorrent_driver.go`

### 等级列表

| Role ID | 等级名称 | 晋升条件 | 权限 |
|---------|----------|----------|------|
| 1 | User | 默认 | - |
| 2 | Power User | 注册 4 周 + 下载 200GB + 分享率 2.0 | 魔力+1%；匿名发表候选；上传字幕 |
| 3 | Elite User | 注册 8 周 + 下载 400GB + 分享率 3.0 | 魔力+2%；发送邀请；管理字幕；查看下载记录；个性条 |
| 4 | Crazy User | 注册 12 周 + 下载 500GB + 分享率 4.0 | 魔力+3% |
| 5 | Insane User | 注册 16 周 + 下载 800GB + 分享率 5.0 | 魔力+4%；查看排行榜 |
| 6 | Veteran User | 注册 20 周 + 下载 1000GB + 分享率 6.0 | 魔力+5%；封存账号后不会被删除 |
| 7 | Extreme User | 注册 24 周 + 下载 2000GB + 分享率 7.0 | 魔力+6%；永远保留 |
| 8 | Ultimate User | 注册 28 周 + 下载 2500GB + 分享率 8.0 | 魔力+7% |
| 9 | Nexus Master | 注册 32 周 + 下载 3000GB + 分享率 9.0 | 魔力+8% |
| 10 | VIP | 管理员授予 | 全部权限 |
| 11 | Retiree | 管理员授予 | 退休管理组 |
| 12 | Uploader | 管理员授予 | 发布权限 |
| 13 | Moderator | 管理员授予 | 版主权限 |
| 14 | Administrator | 管理员授予 | 管理员权限 |
| 15 | Sysop | 管理员授予 | 系统管理员 |

> 晋升条件中 Interval 使用 ISO 8601 duration 格式（如 `P4W` = 4 周）。

### 等级字段路径

API 返回的用户等级通过 `data.role` 字段获取，值为字符串格式的 Role ID（如 `"3"` = Elite User）。

---

## 十一C、API 响应数据结构（实测）

> 数据来源: Go driver 结构体定义 + Python crawler 映射 + Rust adapter + Vertex JS + 测试 fixtures

### 搜索结果字段（MTorrentTorrent）

```go
type MTorrentTorrent struct {
    ID          string `json:"id"`           // 种子 ID
    Name        string `json:"name"`         // 主标题
    SmallDescr  string `json:"smallDescr"`   // 副标题
    Size        string `json:"size"`         // 体积（字节，字符串格式如 "45097156608"）
    CreatedDate string `json:"createdDate"`  // 发布时间 "2025-01-15 10:30:00"（CST）
    Status      struct {
        Seeders         FlexInt                `json:"seeders"`
        Leechers        FlexInt                `json:"leechers"`
        TimesCompleted  FlexInt                `json:"timesCompleted"`
        Discount        string                 `json:"discount"`        // 促销类型枚举
        DiscountEndTime string                 `json:"discountEndTime"` // 促销结束时间
        PromotionRule   *MTorrentPromotionRule `json:"promotionRule"`   // 附加促销规则
        MallSingleFree  *MallSingleFree        `json:"mallSingleFree"`  // 商城单品免费
    } `json:"status"`
    Category string `json:"category"`   // 分类 ID
    // 以下字段实际存在但 Go driver 未解析入结构体（Python crawler 有使用）
    // VideoCodec string `json:"videoCodec"`  // 视频编码 ID
    // AudioCodec string `json:"audioCodec"`  // 音频编码 ID
    // Standard   string `json:"standard"`    // 分辨率 ID
    // Imdb       string `json:"imdb"`        // IMDb 链接
    // Douban     string `json:"douban"`      // 豆瓣链接
    // Labels     int    `json:"labels"`      // 位掩码
}
```

### 促销优先级逻辑

Go driver 实现的促销判断优先级：

1. **promotionRule**（附加促销）→ 若当前时间在促销区间内且优于基础折扣，取促销折扣
2. **mallSingleFree**（商城单品免费）→ 若在有效期内，视为 FREE
3. **base discount**（基础折扣）→ 默认折扣

### FlexCode 处理

API 响应的 `code` 字段可能是 **数字(0)**、**字符串("0")** 或 **字符串("SUCCESS")**，Go driver 使用 `FlexibleCode` 自定义类型统一处理：

```go
func (fc FlexibleCode) IsSuccess() bool {
    s := strings.ToUpper(string(fc))
    return s == "0" || s == "SUCCESS" || s == "200"
}
```

### FlexInt 处理

`size`/`seeders` 等字段可能是数字或字符串，Go driver 使用 `FlexInt` 自定义类型统一处理。

### 下载流程（genDlToken）

1. `POST /api/torrent/genDlToken`（**form-urlencoded**，`id=<torrent_id>`）
2. 响应 `data` 字段为**下载 URL 字符串**（如 `https://api.m-team.cc/api/rss/dlv2?sign=...`）
3. GET 该 URL 获取 `.torrent` 文件

---

## 十一D、Go Driver 实现架构

> 数据来源: `examples/pt-tools/site/v2/mtorrent_driver.go`（1100 行）

### 核心结构

```go
type MTorrentDriver struct {
    BaseURL        string              // API URL
    WebURL         string              // Web URL（详情页）
    APIKey         string
    httpClient     *SiteHTTPClient     // HTTP 客户端
    failoverClient *FailoverHTTPClient // 多 URL 故障转移
    userAgent      string
    useFailover    bool
    siteDefinition *SiteDefinition
}
```

### Failover 机制

M-Team 配置了多个备用 URL（7 个 Failover + 3 个 API），Driver 支持 Failover 自动切换：

```go
// Failover URLs
URLs: []string{
    "https://api.m-team.cc",    // 主
    "https://kp.m-team.cc",
    "https://zp.m-team.io",
    "https://xp.m-team.cc",
    "https://ap.m-team.cc",
    "https://next.m-team.cc",
    "https://ob.m-team.cc",
}

// SPA 前端发现的 API 端点（_APIHOSTS）
// https://api.m-team.cc/api   ← 主
// https://api.m-team.io/api   ← 备用1
// https://api2.m-team.cc/api  ← 备用2（Go driver 未使用）
```
```

### 并发用户信息获取

`GetUserInfo()` 使用 `errgroup` 并发调用 4 个 API：
1. `/api/member/profile`（关键，失败取消其他）
2. `/api/tracker/mybonus`（非关键，失败忽略）
3. `/api/msg/notify/statistic`（非关键，失败忽略）
4. `/api/tracker/myPeerStatistics`（非关键，失败忽略）

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

## 十四、上传标题命名规范

> 数据来源: `wiki.m-team.cc/zh-tw/upload-title-rules`（2026-04-23 采集）

### 14.1 基本原则

1. 一般情况下，**不要在主标题中添加中文**（部分分类管理特许允许中文，详见各分类说明）
2. 无英文名称资源请使用**拼音**（Adult JAV 可使用原文或英文）
3. 未经同意请不要在副标题中添加**全角标点**
4. 除 HGame / HComic / HAnime 类资源及影片名中原本包含括号的情况之外，**禁止在主标题中添加任何括号**（管理员允许的除外）
5. 不要在标题中添加 **MKV、MP4** 等文件格式字样
6. 原种子标题未带后缀名称，请于简介内容说明来源
7. 主标题请勿带有**特殊符号**如 `!;.#$%:`
8. 未经管理员允许，禁止在副标题中添加任何与资源内容**无关的信息**
9. DVD 资源请在标题中注明 **D5 或 D9**
10. 文件列表禁止出现 URL 等广告
11. 音频请标记**声道**如 2.0、5.1
12. **非蓝光原盘及 Remux** 在标题的视频编码请**勿使用 AVC 或 HEVC**（应用 x264/x265）
13. 标题需要附加**年份**，年份需要在标题之后，允许在季数前或后，需要在分辨率之前
14. 多音轨的状况下以**高码率优先**做为标题命名
15. 音频参数中 **EAC3 = DDP**，**AC3 = DD**（两种写法皆可）

### 14.2 电影类

- 视频编码跟音频编码前后顺序皆可
- 分辨率允许在来源后面
- 压制重编码的视频编码请用 **x264 或 H.264**、**x265 或 H.265**，不要使用 AVC/HEVC
- 蓝光原盘及 REMUX 的视频编码请使用 **AVC/HEVC**

**模板**：
```
主标题：英文名称 年份 分辨率 来源 视频编码 音频编码-小组名称
副标题：中文名称 其他附加信息
```

**示例**：
```
主标题：Lisa Frankenstein 2024 BluRay 1080p x264 DTS-HD MA 5.1-MTeam
副标题：丽莎·弗兰肯斯坦

主标题：Wild City 2015 1080p CATCHPLAY WEB-DL AAC2.0 H.264-MWeb
副标题：谜城

蓝光原盘示例：
主标题：Jason X 2001 UHD BluRay 2160p HEVC DTS-HD MA5.1-MTeam
副标题：杰森在太空/星际公敌/太凶杀人狂
```

### 14.3 电视类

- 视频编码跟音频编码前后顺序皆可
- 电视允许主标题**不标示年份**，但需在副标题标示
- 年份需要在标题之后，允许在季数前或后，需要在分辨率之前
- 单或复数集数资源，需要在主标题注明集数 **(S01E01)**
- 压制重编码的视频编码请用 x264/x265，不要使用 AVC/HEVC
- 蓝光原盘及 REMUX 的视频编码请使用 AVC/HEVC

**模板**：
```
主标题：英文名称 年份 集数 分辨率 来源 视频编码 音频编码-小组名称
副标题：中文名称 其他附加信息
```

**示例**：
```
主标题：Best Choice Ever 2024 S01E11-E12 1080p WEB-DL x264 AAC 2.0-QHstudIo
副标题：承欢记 第11-12集

主标题：Go Back Lover S01E19 2160p YOUKU WEB-DL AAC2.0 H.265-MWeb
副标题：再见，怦然心动 | 2024 | 第1季第19集

蓝光原盘示例：
主标题：Dune Prophecy S01 UHD BluRay 2160p HEVC Atmos TrueHD7.1-MTeam
副标题：沙丘：预言 第一季/沙丘：姐妹会 | 2025
```

### 14.4 动画类

#### 原盘

```
主标题：英文名称 年份 分辨率 介质 视频编码 音频编码-小组名称
介质统一为 BluRay，Discx* / VOL* / TV 01-12 等内容统一放在副标题
```

#### Encode

```
主标题：英文名称 年份 分辨率 媒介 视频编码 音频编码-小组名称
媒介可以是 BluRay 也可以是 BDRip
示例：Kinsou no Vermeil 2022 BDRip 1080p x265 FLAC@VCB-Studio&Nekomoe kissaten
```

#### WEB

单集：
```
主标题：英文名称 年份 季数集数 分辨率 来源 视频编码 音频编码-小组名称
示例：Mushoku Tensei Jobless Reincarnation 2023 S02E14 1080p Baha WEB-DL x265 AAC-QHstudIo
```

合集：
```
主标题：英文名称 年份 季数 分辨率 来源 视频编码 音频编码-小组名称
TV 01-12 / Fin+SP / Rev 等内容统一放在副标题
```

### 14.5 漫画类

符合基本原则的情况下自由填写。

### 14.6 音乐类

#### 无损音乐

```
主标题：艺人名称 - 专辑名称 年份 音频编码 整轨/分轨-小组名称
副标题：中文名称 其他附加信息
```

示例：
```
主标题：Kidney - Better Late Than Never 2014 FLAC 分轨
副标题：腰乐队 相见恨晚 专辑
```

**重要**：
- 在添加体积 **大于 100GB** 的音乐区种子之前，务必联系管理组事先取得发布码（`https://ticket.m-team.io/`）
- 种子命名必须以**官方发行信息**相同，严禁任何别名、译名。中港澳台发行的专辑如果官方以中文发行，则专辑名可使用官方中文名称

#### 演唱

模板同电影类：
```
主标题：英文名称 年份 分辨率 来源 视频编码 音频编码-小组名称
副标题：中文名称 其他附加信息
```

### 14.7 软件类

```
主标题：该软件完整的英文名称及版号及制作小组或发布人
副标题：中文名称 其他附加信息
```

示例：`IDM.UEStudio.v15.20.0.7.Incl.Keymaker-CORE`

### 14.8 游戏类

#### 一般游戏

```
主标题：游戏运行平台 游戏类型 游戏英文名-游戏小组/出品公司
副标题：中文名称 其他附加信息如版本号
至少添加四张游戏运行截图以及病毒扫描记录
```

#### 工口游戏

```
主标题：[发售日期][出品方/制作组][日文原名]
副标题：中文名称 其他附加信息如版本号
至少添加四张游戏运行截图
```

### 14.9 教育类

无论资源内容，如果包含影片一律发布至**教育影音**。

---

## 十五、发布规则

> 数据来源: `wiki.m-team.cc/zh-tw/upload-rules`（2026-04-23 采集）

### 15.1 上传者流量计算

- **自己发布的种子上传流量双倍计算**（基于该种子上传流量）
- Seedbox 盒子的计算方法见「盒子相关」（第十六章）
- 上传种子的 Tracker 并不用填，也可随便填，会自动覆写

### 15.2 自购自压

- 自购资源需手写站内 uid 及 `post to mteam` 字样的纸条（uid 是用户编号，非用户名，例如 `12345 to mteam`）
- 自压资源请说明参数设定，最好在 mediainfo 加上自己的 tag（例如 `Muxer`）
- **自压资源仅接受蓝光原盘为来源**，WebDL 或 Remux 之类不接受
- 自购自压资源可以向管理组申请**免费置顶**（利用回报管理组功能）

### 15.3 基本要求

1. 所有电影/电视/动画/纪录/综艺类资源，**必须强制提供豆瓣和 IMDB 链接**（都没有则可以不填）
2. **禁止上传网络游戏客户端**
3. 不要另外上传 Sample 的种子，Sample 片请和正片一起上传
4. Torrent 内容不得包含商业网站链接及他站推广或名称
5. 已发布的种子规则同候选规则，需附有图片、内容、标题需正确，违反者直接删除不另行通知
6. 除漫画和管理员认定应以压缩包形式上传的电子书、游戏和图包外，**禁止以压缩包（rar、zip、7z 等形式）上传资源**（官方及合作 0day 组经讨论后可使用压缩包）
7. 国产资源种子内容应避免包含个人身份信息，当相关信息被隐去后可以发布
8. 所有的软件 Software 及游戏 Game 类种子必须经过**病毒扫描且审核后才可发布**
9. 不要在未经允许的情况下添加国产成人类型的候选
10. **不要转载正处于禁转期或标明禁止转载的资源**，否则候选将直接被移除，情节严重的将被警告
11. 原资源页面所带有的所有资源转载声明在添加候选时也一定要附上
12. 请不要上传带密码的压缩包，一经发现将立刻删除
13. 制作的种子不得有无关信息包括其他网站名称及推广链接等
14. **普通用户单种子上传速度如超出 1Gbps，会触碰封禁规则**
15. 合作小组资源仅允许合作小组发布
16. 无论什么资源都必须在内容中**添加图片**
17. 未在网站所属分类的资源请一律使用**其他分类**
18. 不合格的直接发布资源被发现后，将**重置发布人的发布资格**，重置达 3 次以上者将警告 1 次
19. 不接受被刮销过的资源

### 15.4 影片要求

1. **禁止 RMVB**、截取部分影片转档的影片
2. 禁止抢先偷拍的影片
3. 禁止上传**低码率转高码率 UPSCALE** 档（例外见下）
4. 禁止上传**预告片**
5. 禁止上传使用 **deepfake 技术**的影片
6. 禁止上传在站内已存有原始影片且进行**二次剪辑**的影片
7. 带有非源站流出的水印应予副标题及内容中说明
8. 所有的影片包含蓝光原盘资源必须**强制提供完整 mediainfo 或 bdinfo 信息**（AVS+ 除外）
9. 所有的影片类种子必须添加**至少 3 张或 1x3 以上的影片内容截图**
10. 中国电影及戏剧不再带强制要求勾选中文字幕选项

### 15.5 例外升频影片发布要求

仅限**动画及里番**，需同时满足：
- 发布年代在 **2010 年以前**
- 该影片没有出过 HD 画质（720p）及以上的 WebDL 或圆盘
- **审核制**，未经允许发布即删除并警告

### 15.6 分类选择

| 分类 | 说明 |
|------|------|
| 分类/Bluray | **仅接受蓝光原盘/蓝光原盘 DIY** |
| 分类/Remux | 从原盘抽出的 remux |
| 分类/HD | 压制过后的高清（**最低为 720p**）格式 |
| 分类/SD | 压制过后未达高清的格式 |
| 分类/DVDiso | **仅接受 DVD 原盘** |

### 15.7 发布内容填写

- 主副标题填写请见「上传标题命名」（第十四章）
- mediainfo 及 bdinfo 填写请见站点教学
- 系统可以根据 mediainfo 来**自动选择分辨率、视频编码、音频编码**（发布时先填 mediainfo）

| 视频编码 | 说明 |
|----------|------|
| H.264(AVC) | 蓝光及压制影片格式 |
| H.265(HEVC) | 蓝光及压制影片格式 |
| Xvid | 压制影片格式 |
| AV1 | 压制影片格式 |
| VC-1 | 蓝光格式 |
| MPEG-2 | DVD 格式 |

### 15.8 资源包发布规则

1. 站内的**官组资源包仅允许官组发布**
2. **合作小组资源包仅允许合作小组发布**
3. 资源包发布后**不允许再发布其中单部资源**
4. 已有资源包不得自行抽出不同小组组成资源包
5. **多季影视资源不允许个人多季打包**，允许官方多季资源包
6. 影视资源未经允许**不得打包多小组资源**
7. 资源包不允许含站内已有资源，除非**断种**
8. DMM Adult 资源**不允许打包**
9. 自行整理资源包未经允许**不得发布**
10. 资源包如断种可申请删种重发

**资源包问答**：

| 问题 | 回答 |
|------|------|
| 可以转载单季资源包或官方多季资源包吗？ | **可以** |
| 可以自行打包相同小组发布资源包吗？ | **可以**，仅限已完结且需注意各站点规定并注明来源（不能修改原有结构或新增字幕，字幕请单独上传至字幕区） |
| 可以自行收集不同资源来发布所谓的观影单吗？ | **不可以**，相关功能即将上线 |
| 可以打包官组的资源包发布至其他站点吗？ | **可以** |

### 15.9 上传行为要求（撤种规则）

1. **直到至少出现 3 人完成以上才能撤种**，如无任何人下载，**72 小时后可以撤种**。提前撤种警告 1 次（如种子出现问题请第一时间修改掉主标题，并举报种子）
2. 如做种时间**大于 7 天以上**，种子完成数仍**少于 3 个**且无人下载状态（无人状态需至少达到 24 小时以上），也能撤种

### 15.10 转种须知（Dupe 规则）

- **同资源不同小组** → 可发布
- **同资源不同视频编码** → 可发布
- **同资源不同分辨率** → 可发布
- 同资源包出现后不可再发布其中单集

### 15.11 申请删种重发

- **死种定义**：该种子至最后种子完成下载时间起 **168 小时**后，已无做种者且无人下载
- 符合条件之种子，会员可向管理组申请删除旧有种子并重新发表
- **DUPE 定义**：站内已有重复内容且大小一致的种子，可申请删种，**留旧不留新**

### 15.12 禁止发布的小组作品

- **FGT**

---

## 十六、盒子（Seedbox）规则

> 数据来源: `wiki.m-team.cc/zh-tw/seedbox-rules`（2026-04-23 采集）

### 16.1 上传限制

- 单种上传上限最高为 **1Gbps**，请勿超出此限制

### 16.2 盒子检测与限制

所有盒子及独立主机均会被自动侦测及手动提交到监控名单，在名单内将受到以下限制：

- 个人详情页的用户端如出现盒子图示，即代表为盒子/seedbox/vpn IP
- 使用了**境外伺服器/服务器/seedbox/vpn**与 Tracker 进行回报
- **下载种子时不享用任何种子促销**（无论任何活动）
- 所有的记录仅跟随 IP，并非账号
- 如国外家宽被误判为盒子而统计下载量的情形，请利用管理组回报功能

### 16.3 上传流量限制（盒子专属）

- 仅出现在种子列表及种子详情页
- **管理组手动限制**或**种子发布 72 小时之内系统主动限制**
- 上传流量受到其下载量限制，**最多只能得到种子体积大小的 3 倍上传流量**
  - 例：种子体积 100GB，如下载 100GB，上传仅统计 300GB；如下载 30GB，仅统计 90GB；如下载 0GB，上传则为 0GB
- **自行发布种子不存在此上传限制**
- 种子图示消失，统计的流量不再受其限制（图示本身有结束时间）

### 16.4 盒子使用限制

1. **禁止使用免费线路或无法出示购买凭证的线路**进行传输
2. 付款证明需保存近 **1 年内**，以备管理组查证
3. 与二手商购买的盒子是允许使用的，同样请留好对话记录
4. 当管理组要求出示证明，请提供主机商开通信件及付款通知信件，收件人需与网站注册邮箱相同；向二手商购买的请提供自己的付款证明及对话记录
5. **禁止使用共享 IP 的 VPS 或独立主机**
6. **VPN/机场也同样在此限制**，请不要使用在 Tracker
7. **禁止使用 GCP、AWS、AZ 连接 Tracker**

---

## 十七、全局转载策略对馒头的影响

| 规则 | 馒头站点情况 | PT-Forward 处理 |
|------|-------------|-----------------|
| §30.5 禁止 9KG/成人内容 | 馒头有 4 个成人父分类共 15 子分类 | `skip_modes: ["adult"]` + `skip_categories` 屏蔽 |
| §30.5 禁止禁转/独占/谢绝转载 | Wiki 明确「不要转载正处于禁转期或标明禁止转载的资源」 | 源站带禁转标签→不转发至馒头；馒头带禁转标签→不转发至其他站 |
| §30.5 CatEDU 默认禁转 | CatEDU 在制作组列表中（ID=25, freeOffer=true） | CatEDU 小组资源默认不转发 |
| 标题映射 | 馒头有严格标题命名规范（第十四章） | 转发时需按模板重写标题（主标题英文+参数，副标题中文） |
| 分类映射 | 馒头 12 父分类 + 36 子分类，按格式细分 | 需精确映射 source→分类（Bluray/Remux/HD/SD/DVDiso） |
| 撤种规则 | 至少 3 人完成 / 72h 无下载 / 7天+<3人 | 不影响转发流程 |
| 盒子限制 | 下载不享促销，上传限 3 倍体积 | 转发程序需确保不在盒子环境运行 |
| 自购自压 | 仅接受蓝光原盘为来源 | 转发资源非自购自压，不适用 |
| 禁止小组 FGT | FGT 组作品禁止发布 | 转发时检查源站小组名，FGT→跳过 |

---

## 十八、Swagger API 完整规范分析

> 数据来源: https://test2.m-team.cc/api/v3/api-docs/normal（2026-04-23 采集，110KB OpenAPI 3.1.0 规范）
> **不允许第三方调用**: `/login`、`/admin/**`、`/apikey/**`

### 18.1 API 总览

| 项目 | 数量 |
|------|------|
| 端点总数 | **219 个** |
| Schema 总数 | **53 个** |
| GET 端点 | 11 个 |
| POST 端点 | 208 个 |
| 功能分组 | **28 个** |

### 18.2 端点分组统计

| 分组 | 端点数 | 分组 | 端点数 |
|------|--------|------|--------|
| 種子 | 27 | 用戶 | 26 |
| 系統 | 21 | dmm | 22 |
| 論壇 | 15 | 站內信 | 14 |
| 菠菜 | 15 | 片單 | 11 |
| 求種 | 11 | 評論 | 7 |
| 字幕 | 7 | tracker | 6 |
| 好友 | 6 | 實驗室 | 4 |
| 邀請 | 4 | RSS | 3 |
| 積分商店 | 5 | 首頁趣味盒 | 5 |
| 聯盟小組 | 3 | 友情連結 | 2 |
| 種子候選 | 2 | 首頁投票 | 2 |
| 積分流水 | 1 | 管理組信箱 | 1 |
| 考核 | 1 | 舉報 | 1 |
| 導航菜單 | 1 | 首頁新聞 | 1 |

### 18.3 GET 端点（仅 11 个）

| 端点 | 说明 |
|------|------|
| `GET /system/unix` | 系统时间（Unix 时间戳） |
| `GET /system/iscn?ip=` | IP 归属地查询 |
| `GET /system/ips?ips=` | 批量 IP 查询 |
| `GET /system/ip?ip=` | 单 IP 查询 |
| `GET /system/ipASN?ip=` | IP ASN 查询 |
| `GET /subtitle/dl?id=` | 字幕下载 |
| `GET /subtitle/dlV2?credential=` | 字幕下载 V2 |
| `GET /rss/fetch?params=` | RSS 抓取 |
| `GET /rss/dlv2?form=` | RSS 下载 V2 |
| `GET /forum/topic/redirect?form=` | 论坛主题重定向 |
| `GET /comment/redirect?form=` | 评论重定向 |

### 18.4 种子相关端点（27 个）— 详细参数

#### 列表数据端点（8 个，POST 无参数）

| 端点 | 说明 | 数据用途 |
|------|------|----------|
| `/torrent/categoryList` | 分类列表 | 缓存：12 父分类 + 36+15 子分类 |
| `/torrent/mediumList` | 媒介列表 | 缓存：10 个 |
| `/torrent/sourceList` | 来源列表 | 缓存：7 个 |
| `/torrent/standardList` | 分辨率列表 | 缓存：6 个 |
| `/torrent/videoCodecList` | 视频编码列表 | 缓存：8 个 |
| `/torrent/audioCodecList` | 音频编码列表 | 缓存：15 个 |
| `/torrent/teamList` | 制作组列表 | 缓存：23 个 |
| `/torrent/processingList` | 地区列表 | 缓存：6 个 |

#### 种子操作端点

| 端点 | 方法 | 参数位置 | 参数 |
|------|------|----------|------|
| `/torrent/search` | POST | body (TorrentSearch) | 完整搜索表单 |
| `/torrent/detail` | POST | query | `id`(必填), `origin`(必填) |
| `/torrent/createOredit` | POST | query + multipart | `form`(必填) + TorrentUploadForm |
| `/torrent/genDlToken` | POST | query | `id`(必填) |
| `/torrent/files` | POST | query | `id`(必填) |
| `/torrent/peers` | POST | query | `id`(必填) |
| `/torrent/mediaInfo` | POST | query | `id`(必填) |
| `/torrent/viewHits` | POST | query | `id`(必填) |
| `/torrent/sayThank` | POST | query | `id`(必填) |
| `/torrent/thanksStatus` | POST | query | `id`(必填) |
| `/torrent/sendReward` | POST | query | `id`(必填), `reward`(必填) |
| `/torrent/rewardStatus` | POST | query | `id`(必填) |
| `/torrent/requestReseed` | POST | query | `id`(必填) |
| `/torrent/collection` | POST | query | `id`(必填), `make`(必填) |
| `/torrent/chearCollection` | POST | query | _(未记录)_ |
| `/torrent/queryTorrentTrackerHistory` | POST | query | `from`(必填) |

#### 媒体信息端点

| 端点 | 参数 | 说明 |
|------|------|------|
| `/media/imdb/info` | `code`(必填), `refresh`(必填) | IMDb 信息拉取 |
| `/media/douban/infoV2` | `code`(必填), `refresh`(必填) | 豆瓣信息拉取 |
| `/media/douban/elessarV2` | `code`(必填), `refresh`(必填) | 豆瓣信息拉取（备用源） |

### 18.5 TorrentUploadForm 完整字段（Swagger 验证）

> 与 §10.2 对比，Swagger 定义了 28 个字段，**必填仅 4 个**。

| 字段 | 类型 | 必填 | Swagger 定义 | 说明 |
|------|------|------|-------------|------|
| `name` | string(255) | **是** | ✓ | 标题 |
| `descr` | string(65535) | **是** | ✓ | 简介 |
| `category` | int64 | **是** | ✓ | 分类 ID |
| `anonymous` | boolean | **是** | ✓ | 匿名发布 |
| `smallDescr` | string(255) | 否 | ✓ | 副标题 |
| `source` | int64 | 否 | ✓ | 来源 ID |
| `medium` | int64 | 否 | ✓ | 媒介 ID |
| `standard` | int64 | 否 | ✓ | 分辨率 ID |
| `videoCodec` | int64 | 否 | ✓ | 视频编码 ID |
| `audioCodec` | int64 | 否 | ✓ | 音频编码 ID |
| `team` | int64 | 否 | ✓ | 制作组 ID |
| `processing` | int64 | 否 | ✓ | 地区 ID |
| `countries` | string | 否 | ✓ | 国家/地区（字符串，非数组） |
| `imdb` | string | 否 | ✓ | IMDb 链接 |
| `douban` | string | 否 | ✓ | 豆瓣链接 |
| `dmmCode` | string | 否 | ✓ | DMM 编号 |
| `cids` | string | 否 | ✓ | 专辑 ID 列表 |
| `aids` | string | 否 | ✓ | 艺人 ID 列表 |
| `labels` | int32 | 否 | ✓ | 位掩码 |
| `labelsNew` | string | 否 | ✓ | **新标签系统**（字符串，非 int） |
| `tags` | string | 否 | ✓ | 自定义标签 |
| `file` | binary | 否 | ✓ | 种子文件 |
| `nfo` | binary | 否 | ✓ | NFO 文件 |
| `mediainfo` | string | 否 | ✓ | MediaInfo 文本 |
| `mediaInfoAnalysisResult` | boolean | 否 | ✓ | MediaInfo 解析结果标志 |
| `torrent` | int64 | 否 | ✓ | 编辑模式种子 ID |
| `offer` | int64 | 否 | ✓ | 候选模式 ID |

> **Swagger 与 §10.2 差异**：
> - Swagger **无** `black_node` 字段（页面上发现但 Swagger 未定义，可能是前端自定义字段或已弃用）
> - `countries` 在 Swagger 中是 `string`（非数组），但搜索结果中实际返回 `string[]`（如 `["8"]`）
> - `labelsNew` 是 `string`（非 int），说明新标签系统使用字符串标识

### 18.7 测试环境验证结果（2026-04-23）

> 使用 API Key `019d814f-...` 对 test2.m-team.cc 进行了全量端点验证。

#### 测试环境 vs 生产环境差异

| 项目 | 测试环境 | 生产环境（2026-04-23 实测） |
|------|---------|--------------------------|
| 视频编码 | **6 个**（无 VP8/9、AVS） | **8 个**（完整，含 VP8/9、AVS） |
| 音频编码 | **14 个**（无 WAV） | **15 个**（完整，含 WAV） |
| 制作组 | **20 个**（含测试专有组） | **23 个**（完整） |
| 来源列表 | **8 个**（**含 Encode(9)**） | **7 个**（**无 Encode**） |
| 媒介列表 | 10 个 | 10 个（相同） |
| 分辨率 | 6 个 | 6 个（相同） |
| 分类 | **49 个**（含所有父+子分类） | **47 个**（含 451 教育影片） |
| 国家 | **165 个** | 165 个（相同） |
| 标签（labelsNew） | 10 个 | 10 个（相同） |
| Tracker 域名 | test2.m-team.cc/announce | tracker.m-team.cc/announce, tra1.m-team.cc/announce, tracker.m-team.io/announce, tra1.m-team.io/announce, tra99.manfuz.co/announce |
| 下载域名 | flycrow.pro/mantou.biz/agrosino.net | fr1.halomt.com, bt1.manfuz.co, hs1.holysalt.org |
| RSS 域名 | test2.m-team.cc/api | rss.m-team.cc/api, rss.m-team.io/api |

> **关键突破**：生产 API 需要**携带浏览器 User-Agent** 才能通过 Cloudflare WAF，否则被 302→Google。curl 默认 UA（`curl/x.x`）被拦截。

> **API 访问条件**：`-H "User-Agent: Mozilla/5.0 ..." -H "x-api-key: <KEY>"` 即可直连 `api.m-team.cc`，无需代理、无需 JWT。

#### 生产环境制作组（23 个，实测验证）

| ID | 名称 | order | freeOffer | 说明 |
|----|------|-------|-----------|------|
| 9 | MTeam | 1 | true | 站方主组 |
| 23 | TnP | 3 | false | |
| 44 | MWeb | 3 | true | 站方 WEB 组 |
| 43 | TPTV | 3 | true | 站方 TV 组 |
| 6 | BMDru | 6 | false | |
| 19 | CNHK | 8 | true | |
| 8 | Pack | 10 | false | 合集打包组 |
| 25 | CatEDU | 11 | true | 教育制作组 |
| 26 | ARiC | 12 | true | |
| 30 | 7³ACG | 14 | true | |
| 34 | QHstudIo | 15 | true | |
| 31 | JKCT | 16 | true | |
| 35 | G00DB0Y | 17 | true | |
| 36 | D0 | 18 | true | |
| 57 | DST | 20 | true | |
| 40 | HBO | 22 | true | |
| 41 | REE | 23 | true | |
| 45 | CTRL | 25 | true | |
| 59 | StarfallWeb | 26 | true | |
| 48 | ZTR | 28 | true | |
| 49 | 126811 | 29 | true | |
| 61 | RRS | 30 | true | |
| 62 | lijiang-tv | 31 | true | |

> **生产 vs 测试差异**：测试环境有 20 个（含 MTT/M-TEST/TEST2/KiSHD/OneHD/StBOX/Geek/Telesto/LowPower-Raws），生产有 23 个（多 MWeb/TPTV/DST/StarfallWeb/RRS/lijiang-tv，少测试专用组）。

#### 搜索结果完整字段（实测验证）

```json
{
  "id": "768224",
  "createdDate": "2026-04-21 10:44:07",
  "lastModifiedDate": "2026-04-22 15:15:43",
  "name": "Dark and Dawn S01 1080p friDay WEB-DL AAC2.0 H.264-MWeb",
  "smallDescr": "临暗 | 2026 | 中国大陆 | 王梓 | 许佳琪 代高政",
  "imdb": "",
  "imdbRating": null,
  "douban": "https://movie.douban.com/subject/37390157/",
  "doubanRating": "0",
  "dmmCode": "",
  "author": null,
  "category": "402",
  "source": "8",
  "medium": null,
  "standard": "1",
  "videoCodec": "1",
  "audioCodec": "6",
  "team": "36",
  "processing": null,
  "countries": ["8"],
  "numfiles": "24",
  "size": "23915917866",
  "labels": "0",
  "labelsNew": ["中字", "中配"],
  "msUp": "0",
  "anonymous": true,
  "infoHash": null,
  "status": {
    "id": "768224",
    "pickType": "normal",
    "toppingLevel": "0",
    "toppingEndTime": null,
    "discount": "PERCENT_50",
    "discountEndTime": null,
    "timesCompleted": "0",
    "comments": "0",
    "lastAction": null,
    "lastSeederAction": null,
    "views": "0",
    "hits": "0",
    "support": "0",
    "oppose": "0",
    "status": "NORMAL",
    "seeders": "0",
    "leechers": "0",
    "banned": false,
    "visible": true,
    "promotionRule": null,
    "mallSingleFree": null
  },
  "dmmInfo": null,
  "editedBy": null,
  "editDate": null,
  "collection": false,
  "inRss": false,
  "canVote": false,
  "imageList": ["https://..."],
  "resetBox": null
}
```

**搜索结果新增字段（§25 文档未记录）**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `imdbRating` | string/null | IMDb 评分 |
| `doubanRating` | string | 豆瓣评分（"0" 表示暂无） |
| `numfiles` | string | 文件数 |
| `msUp` | string | 上传流量统计（种子发布者） |
| `infoHash` | string/null | InfoHash（搜索结果中通常为 null） |
| `dmmInfo` | object/null | DMM 信息（成人内容） |
| `editedBy` | string/null | 最后编辑者 |
| `editDate` | string/null | 最后编辑时间 |
| `collection` | boolean | 是否已收藏 |
| `inRss` | boolean | 是否在 RSS 中 |
| `canVote` | boolean | 是否可投票 |
| `imageList` | string[] | 图片 URL 列表 |
| `resetBox` | object/null | 盒子重置信息 |
| `status.pickType` | string | 选取类型（normal/...） |
| `status.toppingLevel` | string | 置顶级别 |
| `status.toppingEndTime` | string/null | 置顶结束时间 |
| `status.comments` | string | 评论数 |
| `status.lastAction` | string/null | 最后活动时间 |
| `status.lastSeederAction` | string/null | 最后做种活动时间 |
| `status.views` | string | 浏览数 |
| `status.hits` | string | 点击数 |
| `status.support` | string | 支持数 |
| `status.oppose` | string | 反对数 |
| `status.status` | string | 种子状态（NORMAL/...） |
| `status.banned` | boolean | 是否被封禁 |
| `status.visible` | boolean | 是否可见 |
| `status.promotionRule` | object/null | 附加促销规则 |
| `status.mallSingleFree` | object/null | 商城单品免费 |

#### detail 端点额外字段

详情比搜索多出以下字段：

| 字段 | 说明 |
|------|------|
| `descr` | 完整简介（BBCode） |
| `nfo` | NFO 内容 |
| `mediainfo` | MediaInfo 文本 |
| `cids` | 专辑 ID |
| `aids` | 艺人 ID |
| `scope` | 范围限制 |
| `scopeTeams` | 范围制作组 |
| `thanked` | 是否已感谢 |
| `rewarded` | 是否已打赏 |
| `albumList` | 所属片单列表 |
| `originFileName` | 原始文件名 |

#### TORRENT_LABEL_CONFIG（实测）— 10 个标签

| # | tag | 说明 |
|---|-----|------|
| 1 | `4k` | 4K 分辨率 |
| 2 | `8k` | 8K 分辨率 |
| 3 | `hdr10` | HDR10 |
| 4 | `hdr10+` | HDR10+ |
| 5 | `hlg` | HLG（混合对数伽马） |
| 6 | `DoVi` | Dolby Vision |
| 7 | `HDRVi` | HDR Vivid（国产 HDR 标准） |
| 8 | `中字` | 中文字幕 |
| 9 | `hdr` | HDR（通用） |
| 10 | `中配` | 中文配音 |

> **关键发现**：`labelsNew` 是字符串数组，值就是 `TORRENT_LABEL_CONFIG` 中的 `tag` 字段（如 `["中字", "中配"]`）。这与 `labels`（int32 位掩码，bit0=DIY/bit1=中配/bit2=中字）是**两套并存的标签系统**。新系统更灵活，使用可读字符串。

#### system/getConf 关键配置值（生产环境实测）

| 配置项 | 值 |
|--------|-----|
| `TORRENT_DOWNLOAD_DOMAINS` | `["https://fr1.halomt.com","https://bt1.manfuz.co","https://hs1.holysalt.org"]` |
| `TORRENT_TRACKER_DOMAINS` | `["tracker.m-team.cc/announce","tra1.m-team.cc/announce","tracker.m-team.io/announce","tra1.m-team.io/announce","tra99.manfuz.co/announce"]`（**5 个 tracker**） |
| `TORRENT_RSS_DOMAINS` | `["https://rss.m-team.cc/api","https://rss.m-team.io/api"]` |
| `TORRENT_UPLOAD_COUNTRY_HIDE_CATS` | `["401","419","420","421","439","403","402","438","435"]`（电影/剧集分类隐藏国家选择） |

#### 候选（Offer）配置

| 配置项 | 值 | 说明 |
|--------|-----|------|
| `OFFER_MIN_VOTES` | 20 | 最低投票数 |
| `OFFER_VOTE_TIMEOUT` | 604800 秒（7天） | 投票超时 |
| `OFFER_UP_TIMEOUT` | 35996400 秒（~416天） | 候选上传超时 |

#### 生产环境站点状态（system/state，2026-04-23）

| 指标 | 值 |
|------|-----|
| 用户上限 | 150,000 |
| 总用户 | 97,124 |
| 今日活跃 | 10,220 |
| 本周活跃 | 50,038 |
| 当前在线 | 946 |
| 预警用户 | 606 |
| 封禁用户 | 218 |
| 总种子 | 705,218 |
| 死种 | 6,031 |
| 做种连接 | 17,018,027 |
| 下载连接 | 117,329 |
| 做种用户 | 65,298 |
| 总体积 | 18.3 PB |

#### 字幕语言

| ID | 语言 | subLang | siteLang |
|----|------|---------|----------|
| 6 | English | true | true |
| 15 | 日本語 | true | false |
| 16 | 한국어 | true | false |
| 18 | Other | true | false |

#### 国家列表

165 个国家，关键 ID：

| ID | 国家 | ID | 国家 |
|----|------|----|------|
| 8 | 中国 | 12 | United Kingdom |
| 17 | Japan | 45 | Pakistan |
| 2 | United States | 3 | Russia |

#### 分类新增发现（生产环境实测）

生产环境 categoryList 返回 **47 个**项目，关键发现：

| ID | 名称 | 父分类 | 说明 |
|----|------|--------|------|
| **451** | **教育影片** | 450(其他) | **确认存在**（§25 文档记录在 Go driver 但 API 文档未列出） |
| 406 | MV | 110(Music) | 属于 Music 父分类 |
| 427 | E-Book | 450(其他) | 生产改名为 E-Book（测试环境为 education book） |
| 442 | AuiBook | 450(其他) | **新增**（有声书，测试环境为 edu Auido） |
| 440 | AV(Gay)/HD | 120(AV無碼) | waterfall 分组中包含 |
| 441 | edu Video | 450(其他) | 仅测试环境存在，生产**改为 451** |
| 409 | Misc(Other) | 450(其他) | order=4 |

> **生产环境 waterfall 分组（36 个）**比测试环境（34 个）多 440(AV Gay)和 441/442/451（教育子分类），少 408(Music AAC/ALAC)。
> 生产 adult 分组（15 个）同测试环境，但**不含 408/434**（Music 子分类不属于成人区）。

### 18.6 TorrentSearch 完整字段（Swagger 验证）

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `pageNumber` | int32 | 1-1000 | 页码 |
| `pageSize` | int32 | 1-200 | 每页条数 |
| `lastId` | int64 | - | 游标分页 |
| `keyword` | string(100) | - | 关键词 |
| `authorId` | int64 | - | 发布者 ID |
| `categories` | int64[] | unique | 分类筛选 |
| `sources` | int64[] | unique | 来源筛选 |
| `mediums` | int64[] | unique | 媒介筛选 |
| `standards` | int64[] | unique | 分辨率筛选 |
| `videoCodecs` | int64[] | unique | 视频编码筛选 |
| `audioCodecs` | int64[] | unique | 音频编码筛选 |
| `teams` | int64[] | unique | 制作组筛选 |
| `processings` | int64[] | unique | 地区筛选 |
| `countries` | int64[] | unique | 国家筛选 |
| `labels` | int32 | - | 标签位掩码筛选 |
| `labelsNew` | string[] | unique | **新标签筛选** |
| `imdb` | string | - | IMDb 筛选 |
| `douban` | string | - | 豆瓣筛选 |
| `dmmCode` | string | - | DMM 筛选 |
| `author` | int64 | - | 发布者 ID（与 authorId 重复？） |
| `uploadDateStart` | datetime | - | 上传时间起始 |
| `uploadDateEnd` | datetime | - | 上传时间截止 |
| `visible` | int32 | - | 可见性 |
| `onlyFav` | boolean | - | 仅收藏 |
| `offer` | boolean | - | 仅候选 |
| `hot` | boolean | - | 仅热门 |
| `mode` | string enum | - | normal/adult/movie/music/tvshow/waterfall/rss/rankings/all |
| `dmmField` | TorrentDmmSearchField | - | DMM 搜索子字段 |
| `discount` | string enum | - | NORMAL/PERCENT_70/PERCENT_50/FREE/_2X_FREE/_2X/_2X_PERCENT_50 |
| `sortField` | string enum | - | CREATED_DATE/SIZE/SEEDERS/LEECHERS/TIMES_COMPLETED/NAME |
| `sortDirection` | string enum | - | ASC/DESC |
| `formSystem` | boolean | - | 系统表单标志 |
| `withCache` | boolean | - | 使用缓存 |

> **Swagger 新发现（§25 文档未记录）**：
> - `labelsNew` 字段：**新标签系统**，使用字符串数组（非位掩码），与 `labels`（int32 位掩码）并存
> - `dmmField` 子对象：支持按 director/series/maker/label/actres/keyword/keywordIds 搜索
> - `sortField` 支持 6 种排序
> - `uploadDateStart/End`：时间范围筛选
> - `discount` 枚举仅 7 种（Swagger），但实际有 11 种（含 FREE_2XUP 等）

### 18.7 MemberConfigVo（用户配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| `trackerDomain` | string | Tracker 域名 |
| `downloadDomain` | string | 下载域名 |
| `rssDomain` | string | RSS 域名 |
| `blockCategories` | int64[] (unique) | 屏蔽的分类 ID 列表 |
| `hideFun` | boolean | 隐藏趣味盒 |
| `showThumbnail` | boolean | 显示缩略图 |
| `timeType` | string enum | timeAdded（添加时间）/ timeAlive（存活时间） |
| `anonymous` | boolean | 默认匿名发布 |
| `trackerDisableSeedbox` | boolean | Tracker 禁用盒子 |

### 18.8 system/getConf 可查询配置项（156 个）

> 通过 `POST /system/getConf?items=KEY1,KEY2,...` 可查询系统配置值。

关键配置项（适配器相关）：

| 配置项 | 说明 | 用途 |
|--------|------|------|
| `TORRENT_DOWNLOAD_DOMAINS` | 下载域名列表 | 生成下载链接 |
| `TORRENT_TRACKER_DOMAINS` | Tracker 域名列表 | 构造 announce URL |
| `TORRENT_RSS_DOMAINS` | RSS 域名列表 | RSS 链接 |
| `TORRENT_LABEL_CONFIG` | 标签配置 | 理解 labelsNew |
| `TORRENT_FORCE_OFFER_CATEGORIES` | 强制候选分类 | 某些分类必须走候选流程 |
| `TORRENT_REQUIRE_MEDIAINFO_CATEGORIES` | 强制 MediaInfo 分类 | 某些分类必须填 MediaInfo |
| `TORRENT_UPLOAD_COUNTRY_HIDE_CATS` | 隐藏国家选择的分类 | 某些分类不显示地区 |
| `ADULT_CATEGORIES` | 成人分类列表 | 屏蔽参考 |
| `MOVIE_CATEGORIES` | 电影分类列表 | 搜索 mode=movie |
| `MUSIC_CATEGORIES` | 音乐分类列表 | 搜索 mode=music |
| `TVSHOW_CATEGORIES` | 剧集分类列表 | 搜索 mode=tvshow |
| `WATERFALL_CATEGORIES` | 瀑布流分类列表 | 搜索 mode=waterfall |
| `EXAM_TIME/UPLOAD/DOWNLOAD/BONUS/DESCR` | 考核参数 | 新用户考核 |
| `TRACKER_UPLOADER_DOUBLE` | 上传者双倍 | 发布者流量计算 |
| `TRACKER_PERSEEDING_BONUS` | 每种子做种奖励 | 做种魔力计算 |
| `TORRENT_HOTDAYS/HOTSEEDER` | 热门种子判定 | 热门标准 |
| `TORRENT_OFFER_MODIFY_TIMEOUT` | 候选修改超时 | 候选管理 |
| `OFFER_MIN_VOTES/VOTE_TIMEOUT/UP_TIMEOUT` | 候选投票参数 | 候选转正条件 |
| `MEMBER_DELETE_PACKED/UNPACKED` | 删号处理 | 用户管理 |

### 18.9 其他关键 Schema

#### TorrenSubtitleForm（字幕上传）

| 字段 | 类型 | 说明 |
|------|------|------|
| `title` | string | 字幕标题 |
| `torrent` | string | 关联种子 |
| `anonymous` | boolean | 匿名 |
| `lang` | string | 语言 ID |
| `file` | binary | 字幕文件 |

#### RssGenForm（RSS 生成）

与 TorrentSearch 几乎相同的筛选字段，额外包含：
- `tkeys` (string[])：自定义关键词
- `albumId` (int64)：片单 ID

#### SeekEditForm（求种编辑）

| 字段 | 类型 | 说明 |
|------|------|------|
| `seekId` | int64 | 求种 ID |
| `title` | string(必填) | 标题 |
| `category` | int64 | 分类 |
| `source` | int64 | 来源 |
| `standard` | int64 | 分辨率 |
| `imdb` | string(128) | IMDb |
| `douban` | string(128) | 豆瓣 |
| `dmmCode` | string(128) | DMM |
| `reward` | int32(≥1000) | 悬赏积分 |
| `intro` | string(必填) | 说明 |

### 18.10 Swagger 与 §25 API 文档对比总结

| 对比项 | §25 API 文档 | Swagger 规范 | 差异说明 |
|--------|-------------|-------------|----------|
| 端点总数 | 363 | **219** | Swagger 是测试环境，可能少于生产 |
| Schema 总数 | 108 | **53** | Swagger 不含全部数据模型 |
| `black_node` | 未记录 | **未定义** | 页面可见但 API 层面可能不存在 |
| `labelsNew` | 未记录 | **string** | 新标签系统（字符串，非位掩码） |
| `mediaInfoAnalysisResult` | 未记录 | **boolean** | MediaInfo 解析结果标志 |
| `offer` 字段 | 未记录 | **int64** | 候选模式 ID |
| `tags` 字段 | 未记录 | **string** | 自定义标签 |
| `detail` Content-Type | JSON（文档） | **query param** | Swagger 确认是 query 参数 |
| `createOredit` | multipart | **query+multipart** | form 参数在 query 中 |
| discount 枚举 | 7 种 | **7 种** | 一致（实际 11 种，Swagger 未记录全部） |
| mode 枚举 | 未明确 | **9 种** | normal/adult/movie/music/tvshow/waterfall/rss/rankings/all |

---

*数据来源: 生产环境 API（api.m-team.io）2026-04-16 via Playwright；测试环境 Swagger API spec（test2.m-team.cc）2026-04-23*
*Wiki 规则来源: wiki.m-team.cc 2026-04-23 via Playwright + extraHTTPHeaders 认证*
*文档创建: 2026-04-16 | 更新: 2026-04-23*
