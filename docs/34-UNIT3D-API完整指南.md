# UNIT3D v8.x API 完整指南

> **基于源码深度分析** — UNIT3D 是基于 Laravel 的现代 PT 建站框架（PHP 8.2+，Blade 模板）
> **源码位置**: `examples/UNIT3D/`
> **创建日期**: 2026-04-15
> **状态**: ✅ 完成

---

## 目录

| 章节 | 内容 | PT-Forward 相关性 |
|------|------|------------------|
| [一、认证体系](#一认证体系) | API Token / RSS Key / Passkey / Session | ⭐⭐⭐⭐⭐ |
| [二、REST API 端点](#二rest-api-端点) | 种子列表/搜索/详情/上传 | ⭐⭐⭐⭐⭐ |
| [三、RSS Feed](#三rss-feed) | URL 格式/过滤参数/字段映射 | ⭐⭐⭐⭐⭐ |
| [四、种子下载](#四种子下载) | 下载 URL / announce URL 注入 | ⭐⭐⭐⭐⭐ |
| [五、种子上传/发布](#五种子上传发布) | 完整上传流程/字段/验证 | ⭐⭐⭐⭐⭐ |
| [六、免费/促销系统](#六免费促销系统) | 多层免费体系 | ⭐⭐⭐⭐ |
| [七、Tracker 协议](#七tracker-协议) | Announce 格式/参数/响应 | ⭐⭐⭐ |
| [八、用户 API](#八用户-api) | 用户信息/统计/书签 | ⭐⭐⭐ |
| [九、数据模型](#九数据模型) | 核心 Eloquent 模型字段 | ⭐⭐⭐⭐ |
| [十、配置参考](#十配置参考) | 关键配置项 | ⭐⭐⭐ |
| [十一、PT-Forward 集成要点](#十一pt-forward-集成要点) | 与设计文档的对应关系 | ⭐⭐⭐⭐⭐ |

---

## 一、认证体系

UNIT3D 使用**四种独立凭据**，分别服务于不同场景：

| 凭据 | 字段 | 格式 | 用途 | 存储 |
|------|------|------|------|------|
| **Passkey** | `users.passkey` | 32 位 hex | Tracker announce 认证 | 数据库 + 8h 缓存 |
| **RSS Key** | `users.rsskey` | 32 位 hex | RSS 访问 + 种子下载 | 数据库 |
| **API Token** | `users.api_token` | 字符串 | REST API 认证 | 数据库（明文存储） |
| **Session** | Laravel Session | Cookie | Web 页面登录 | session 驱动 |

### 1.1 API 认证

```
方式一（Query Parameter）:
GET /api/torrents?api_token=<your_api_token>

方式二（Header）:
GET /api/torrents
Authorization: Bearer <your_api_token>
```

- Guard: `auth:api`（Laravel token driver）
- Token **不明文哈希**存储（`config/auth.php` → `'hash' => false`）
- 用户 Provider: `cache-user`（带缓存的 Eloquent Provider）

### 1.2 RSS 认证

```
GET /rss/{feed_id}.{rsskey}
GET /torrent/download/{torrent_id}.{rsskey}
```

- Guard: `auth:rss`（自定义 rsskey driver）
- RSS feed 必须属于该用户或为公开（`is_private = false`）

### 1.3 Tracker 认证

```
GET /announce/{passkey}?info_hash=...&peer_id=...&port=...
```

- Passkey 嵌入 URL 路径（非 query parameter）
- 验证：长度 32 + 仅 `[a-f0-9]` + 查询 users 表

### 1.4 凭据管理端点

| 操作 | 方法 | 路径 |
|------|------|------|
| 查看 API Token | GET | `/users/{username}/apikeys` |
| 重新生成 API Token | PATCH | `/users/{username}/apikeys` |
| 查看 Passkey | GET | `/users/{username}/passkeys` |
| 重新生成 Passkey | PATCH | `/users/{username}/passkeys` |

### 1.5 限速配置

| 限速器 | 频率 | Key |
|--------|------|-----|
| API | 30/min | user ID |
| RSS | 30/min | user ID |
| Announce | 500/min | IP |
| Search | 100/min | user ID |
| Login | 5/min | IP |

---

## 二、REST API 端点

> **Base URL**: `{site_url}/api`
> **认证**: API Token（见 §1.1）
> **中间件**: `auth:api` + `banned`

### 2.1 种子列表（最新）

```
GET /api/torrents
```

**说明**: 返回最新种子，游标分页，每页 25 条。

**排序**: `sticky DESC, bumped_at DESC`

**缓存**: 5-6 分钟 flexible cache

**响应** (`TorrentsResource`):

```json
{
  "data": [
    {
      "id": 123,
      "name": "Movie.2026.1080p.BluRay.x264",
      "size": 12345678901,
      "leechers": 5,
      "seeders": 42,
      "times_completed": 100,
      "created_at": "2026-04-15T10:00:00Z",
      "bumped_at": "2026-04-15T10:00:00Z",
      "category": {"id": 1, "name": "Movie"},
      "type": {"id": 1, "name": "Encode"},
      "resolution": {"id": 3, "name": "1080p"},
      "user": {"id": 1, "username": "uploader"},
      "anon": false,
      "sticky": false,
      "free": 0,
      "doubleup": false,
      "refundable": false,
      "internal": false,
      "meta": {
        "tmdb_movie_id": 12345,
        "imdb": 1234567
      }
    }
  ],
  "links": {
    "first": "...",
    "last": "...",
    "prev": "...",
    "next": "..."
  },
  "meta": {
    "path": "...",
    "per_page": 25,
    "next_cursor": "eyJpZCI6MTAwfQ=="
  }
}
```

### 2.2 种子搜索/过滤

```
GET /api/torrents/filter
```

**搜索引擎**: Meilisearch（默认）/ SQL（需 staff 权限，`?driver=sql`）

**分页**: 游标分页，每页最多 100 条，默认 25

**查询参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | string | 按名称搜索 |
| `description` | string | 搜索描述 |
| `mediainfo` | string | 搜索 MediaInfo |
| `uploader` | string | 上传者用户名 |
| `keywords` | string | 逗号分隔关键词 |
| `startYear` / `endYear` | int | 年份范围 |
| `categories` | int[] | 分类 ID 列表 |
| `types` | int[] | 类型 ID 列表 |
| `resolutions` | int[] | 分辨率 ID 列表 |
| `genres` | int[] | TMDB Genre ID |
| `tmdbId` | int | TMDB ID |
| `imdbId` | int | IMDB ID（纯数字） |
| `tvdbId` | int | TVDB ID |
| `malId` | int | MAL ID |
| `playlistId` | int | 播放列表 ID |
| `collectionId` | int | TMDB 合集 ID |
| `free` | int[] | 免费百分比（25/50/75/100） |
| `doubleup` | bool | 双倍上传 |
| `featured` | bool | 精选 |
| `highspeed` | bool | 高速 |
| `internal` | bool | 内部发布 |
| `personalRelease` | bool | 个人发布 |
| `alive` | bool | 有做种者 |
| `dying` | bool | 1 做种 + ≥3 完成 |
| `dead` | bool | 0 做种 |
| `seasonNumber` | int | 季数 |
| `episodeNumber` | int | 集数 |
| `sortField` | string | 排序字段: name/size/seeders/leechers/times_completed/created_at/bumped_at |
| `sortDirection` | string | asc / desc |
| `perPage` | int | 每页条数（max 100） |

**Meilisearch 响应**额外包含 `download_link` 和 `magnet_link`（如启用），含用户 rsskey/passkey。

### 2.3 种子详情

```
GET /api/torrents/{id}
```

**响应** (`TorrentResource`):

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int | 种子 ID |
| `name` | string | 标题 |
| `description` | string | BBCode 描述 |
| `mediainfo` | string | MediaInfo 输出 |
| `bdinfo` | string | BDInfo 输出 |
| `info_hash` | string | 40 字符 hex |
| `size` | float | 总大小（字节） |
| `num_file` | int | 文件数 |
| `folder` | string | 顶层文件夹名 |
| `leechers` | int | 当前下载数 |
| `seeders` | int | 当前做种数 |
| `times_completed` | int | 下载完成数 |
| `free` | int | 免费百分比 0-100 |
| `doubleup` | bool | 双倍上传 |
| `refundable` | bool | 可退还 |
| `fl_until` | datetime? | 免费到期时间 |
| `du_until` | datetime? | 双倍到期时间 |
| `status` | int | 0=PENDING, 1=APPROVED, 2=REJECTED, 3=POSTPONED |
| `anon` | bool | 匿名上传 |
| `sticky` | bool | 置顶 |
| `internal` | int | 内部发布 |
| `personal_release` | bool | 个人发布 |
| `highspeed` | int | 高速标记 |
| `category` | object | 分类信息 |
| `type` | object | 类型信息 |
| `resolution` | object | 分辨率信息 |
| `user` | object | 上传者（匿名时为 null） |
| `files` | array | 文件列表 [{name, size}] |
| `download_link` | string | 下载链接（含 rsskey） |
| `magnet_link` | string | 磁力链接（如启用） |
| `meta` | object | TMDB/IMDB/TVDB/MAL 元数据 |

### 2.4 API 上传种子

```
POST /api/torrents/upload
Content-Type: multipart/form-data
```

**必填字段**:

| 字段 | 类型 | 验证 |
|------|------|------|
| `torrent` | file | .torrent 扩展名, 合法 bencode, 非 v2/hybrid, info_hash 唯一 |
| `name` | string | max:255, 全站唯一 |
| `description` | string | 必须 |
| `category_id` | int | exists:categories,id |
| `type_id` | int | exists:types,id |
| `anonymous` | int | 0 或 1 |

**条件必填**（movie/tv 分类时）:

| 字段 | 条件 |
|------|------|
| `resolution_id` | movie/tv 分类必填 |
| `season_number` | tv 分类必填 |
| `episode_number` | tv 分类必填 |

**可选字段**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `nfo` | file | NFO 文件 |
| `mediainfo` | string | MediaInfo 输出 |
| `bdinfo` | string | BDInfo 输出 |
| `region_id` | int | 地区 ID |
| `distributor_id` | int | 发行商 ID |
| `imdb` | int | IMDB 数字 ID |
| `tvdb` | int | TVDB ID |
| `tmdb` | int | TMDB ID |
| `mal` | int | MAL ID |
| `igdb` | int | IGDB ID（game 分类） |
| `personal_release` | bool | 个人发布 |
| `keywords` | string | 空格分隔关键词 |

**Staff/Internal 专用字段**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `free` | int | 免费百分比 0-100 |
| `doubleup` | bool | 双倍上传 |
| `internal` | int | 内部发布 |
| `refundable` | bool | 可退还 |
| `sticky` | bool | 置顶 |
| `featured` | bool | 精选 |
| `fl_until` | int | 免费天数 |
| `du_until` | int | 双倍天数 |

**成功响应**:

```json
{
  "success": true,
  "data": "https://site.tld/torrent/download/{id}.{rsskey}",
  "message": "Torrent uploaded successfully."
}
```

**上传内部流程**:

```
1. 检查上传权限 (user.can_upload || user.group.can_upload)
2. 验证 .torrent 文件 (扩展名 + bencode + 非 v2 + info_hash 唯一)
3. 提取元数据 (文件数, 大小, 文件夹名, info_hash)
4. 存储 .torrent 到磁盘
5. 验证所有字段（按分类规则）
6. 创建 Torrent 记录
7. 插入 TorrentFile 记录（每 21666 文件分批）
8. 通知外部 Tracker (Unit3dAnnounce::addTorrent)
9. 抓取 TMDB/IGDB 元数据
10. 存储关键词
11. 可信用户 + 非 mod_queue: 自动审核 + 聊天通知
```

### 2.5 请求（Request）API

```
GET /api/requests/filter    — 搜索求种
GET /api/requests/{id}      — 求种详情
```

### 2.6 用户 API

```
GET /api/user               — 当前用户信息
```

**响应**:

```json
{
  "username": "string",
  "group": "string",
  "uploaded": "50.00 GiB",
  "downloaded": "1.00 GiB",
  "ratio": "50.00",
  "buffer": "125.00 GiB",
  "seeding": 0,
  "leeching": 0,
  "seedbonus": "100.00",
  "hit_and_runs": 0
}
```

---

## 三、RSS Feed

### 3.1 URL 格式

```
GET /rss/{feed_id}.{rsskey}
```

- `feed_id` — RSS 订阅 ID（整数，用户在站内创建）
- `rsskey` — 用户 32 位 hex RSS 密钥

### 3.2 认证

- Guard: `auth:rss`（自定义驱动，通过 `users.rsskey` 验证）
- Feed 必须属于该用户或为公开（`is_private = false`）
- 被禁用户返回 404

### 3.3 过滤参数（存储在 `rss.json_torrent`）

每个 RSS feed 在创建时可配置过滤条件，存储为 JSON：

```json
{
  "search": null,
  "description": null,
  "uploader": null,
  "imdb": null,
  "mal": null,
  "tvdb": null,
  "tmdb": null,
  "categories": null,
  "types": null,
  "resolutions": null,
  "genres": null,
  "freeleech": null,
  "doubleupload": null,
  "featured": null,
  "highspeed": null,
  "internal": null,
  "personalrelease": null,
  "bookmark": null,
  "alive": null,
  "dying": null,
  "dead": null
}
```

> **注意**: 这些过滤参数在服务端存储，不在 URL 中传递。用户通过 Web UI 创建 RSS feed 时设置。

### 3.4 RSS 输出字段

基于 `resources/views/rss/show.blade.php` 模板：

| RSS 字段 | 内容 | 说明 |
|----------|------|------|
| `<title>` | 种子名称 | 直接取 `torrent.name` |
| `<category>` | 分类名 | 如 "Movie", "TV" |
| `<contentlength>` | 大小（字节） | 整数 |
| `<link>` | 下载链接 | `https://site/torrent/download/{id}.{rsskey}` |
| `<guid>` | 种子 ID | 整数 |
| `<description>` | HTML CDATA | 含名称/分类/类型/分辨率/大小/日期/做种数/上传者/IMDB 等 |
| `<dc:creator>` | 上传者 | 匿名时为 "Anonymous" |
| `<pubDate>` | 创建时间 | RSS 标准时间格式 |
| `<ttl>` | 5 | 分钟 |

### 3.5 RSS 生成逻辑

- **限制**: 最多 50 条结果
- **排序**: `bumped_at DESC`（最后被顶的时间）
- **缓存**: 5-6 分钟 flexible cache
- **搜索引擎**: Meilisearch + TorrentSearchFiltersDTO

### 3.6 RSS 管理 Web 路由

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/rss` | 列出所有 RSS feeds |
| GET | `/rss/create` | 创建 RSS feed 表单 |
| POST | `/rss/store` | 保存 RSS feed |
| GET | `/rss/{id}/edit` | 编辑 RSS feed |
| PATCH | `/rss/{id}/update` | 更新 RSS feed |
| DELETE | `/rss/{id}/destroy` | 删除 RSS feed |

### 3.7 PT-Forward 适配器配置

```yaml
# UNIT3D 站点 RSS 适配器配置
framework: "unit3d"
rss_url_template: "{base_url}/rss/{feed_id}.{rsskey}"

# hash/size/id 策略
hash_strategy: "fake_from_id"   # RSS 中无 info_hash，使用 fake hash
size_strategy: "xml_tag"        # <contentlength> 标签
id_strategy: "link_regex"       # 从 download URL 提取
id_pattern: "download/(\d+)"

# 下载 URL 格式
download_mode: "template"
download_url_template: "{base_url}/torrent/download/{torrent_id}.{rsskey}"
```

---

## 四、种子下载

### 4.1 Web 下载

```
GET /torrents/download/{id}
```

- 认证: Session（Web 登录）
- 流程: 读 .torrent → 注入 announce URL（含 passkey）→ 设置 comment → 返回

### 4.2 RSS/API 下载

```
GET /torrent/download/{id}.{rsskey}
```

- 认证: rsskey
- 流程同上，但使用 rsskey 认证
- 返回文件名: `[UNIT3D]{torrent_name}.torrent`（source 可配置）

### 4.3 Announce URL 注入

下载的 .torrent 文件中 announce URL 被替换为：

```
https://{site_url}/announce/{user_passkey}
```

- Comment 设置为 `config('torrent.comment')`（默认 "This torrent was downloaded from UNIT3D"）
- `created_by` 字段设置或追加 `config('torrent.created_by')`

### 4.4 PT-Forward 下载策略

UNIT3D 的种子下载链接格式统一为：

```
{base_url}/torrent/download/{torrent_id}.{rsskey}
```

模板配置：

```yaml
download_mode: "template"
download_url_template: "{base_url}/torrent/download/{torrent_id}.{rsskey}"
```

无需 Cookie，rsskey 拼在 URL 路径中即可下载。

---

## 五、种子上传/发布

### 5.1 上传端点

```
POST /api/torrents/upload
Content-Type: multipart/form-data
Authorization: Bearer <api_token>
```

### 5.2 完整上传表单字段

**必填**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `torrent` | file(.torrent) | 种子文件 |
| `name` | string(255) | 标题（全站唯一） |
| `description` | string | BBCode 描述 |
| `category_id` | int | 分类 ID |
| `type_id` | int | 类型 ID（编码方式） |
| `anonymous` | int | 0 或 1 |

**条件必填**:

| 字段 | 条件 | 说明 |
|------|------|------|
| `resolution_id` | movie/tv 分类 | 分辨率 ID |
| `season_number` | tv 分类 | 季数 |
| `episode_number` | tv 分类 | 集数 |

**可选**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `nfo` | file | NFO 文件 |
| `mediainfo` | string | MediaInfo 输出 |
| `bdinfo` | string | BDInfo 输出 |
| `region_id` | int | 地区 |
| `distributor_id` | int | 发行商 |
| `imdb` | int | IMDB 数字 ID |
| `tvdb` | int | TVDB ID |
| `tmdb` | int | TMDB ID |
| `mal` | int | MAL ID |
| `igdb` | int | IGDB ID |
| `personal_release` | bool | 个人发布 |
| `keywords` | string | 空格分隔关键词 |

### 5.3 Bencode 处理

```
Bencode::get_infohash()  → SHA1(bencode(info_dict))  → 20 字节二进制
Bencode::get_meta()      → {count: 文件数, size: 总字节数}
Bencode::get_name()      → info_dict.name（文件夹名）
Bencode::is_v2_or_hybrid() → 检查 piece_layers 键（拒绝 v2）
```

### 5.4 审核流程

| 用户组 | 审核 |
|--------|------|
| Trusted + 未选 mod_queue | 自动审核 + 聊天通知 |
| 其他用户 | 进入审核队列（status=PENDING） |
| Staff/Internal | 自动审核 |

### 5.5 PT-Forward 发布适配

```go
// Unit3D 发布适配器
type Unit3DPublisher struct{}

func (p *Unit3DPublisher) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
    // POST {base_url}/api/torrents/upload
    // multipart/form-data:
    //   torrent      = req.TorrentData
    //   name         = req.Title
    //   description  = req.Description
    //   category_id  = mappedCategory
    //   type_id      = mappedType
    //   resolution_id = mappedResolution (movie/tv)
    //   anonymous    = 0
    //   mediainfo    = req.Mediainfo
    //
    // Header: Authorization: Bearer {api_token}
}

func (p *Unit3DPublisher) GetUploadForm(ctx context.Context) (*UploadForm, error) {
    // 需要获取的映射表:
    //   categories    → GET /api/torrents (观察 category 字段)
    //   types         → 同上观察 type 字段
    //   resolutions   → 同上观察 resolution 字段
    // 站内通常有固定列表，可通过 Web 页面抓取
}
```

---

## 六、免费/促销系统

UNIT3D 拥有 PT 站点中最复杂的**多层免费体系**：

### 6.1 层级概览

| 层级 | 范围 | 配置位置 |
|------|------|---------|
| 全局 | 所有种子 | `config('other.freeleech')` |
| 分组 | 用户组内所有种子 | `groups.is_freeleech` |
| 个人 | 用户所有种子 | `personal_freeleech` 表 |
| 种子级 | 单个种子 | `torrents.free` (0-100) |
| 精选 | 精选种子 | `featured_torrents` 表 |
| 令牌 | 用户+种子 | `freeleech_tokens` 表 |
| 自动规则 | 按分类/类型/分辨率 | `automatic_torrent_freeleech` 表 |

### 6.2 种子级免费字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `free` | int | 免费百分比 0-100（0=无折扣, 100=完全免费） |
| `fl_until` | datetime? | 免费到期时间 |
| `doubleup` | bool | 双倍上传 |
| `du_until` | datetime? | 双倍到期时间 |
| `refundable` | bool | 可退还 |
| `balance` | int | 平衡追踪 |
| `balance_offset` | int | 平衡偏移 |

### 6.3 免费计算优先级（Announce 中）

```
下载免费判定（任一命中 → 不扣下载量）:
  1. 个人免费 (PersonalFreeleech)
  2. 捐赠者 (is_donor)
  3. 分组免费 (group.is_freeleech)
  4. 免费令牌 (FreeleechToken)
  5. 精选种子 (FeaturedTorrent)
  6. 全局免费 (config)
  
  否则:
    实际下载量 = 原始下载量 × (100 - min(100, torrent.free)) / 100

双倍上传判定（任一命中 → 上传量 ×2）:
  1. 种子双倍 (torrent.doubleup)
  2. 分组双倍 (group.is_double_upload)
  3. 精选种子 (FeaturedTorrent)
  4. 全局双倍 (config)
```

### 6.4 PT-Forward DiscountLevel 映射

| UNIT3D 原始值 | PT-Forward 枚举 | 说明 |
|---------------|-----------------|------|
| `free=100` | `FREE` | 完全免费 |
| `free=100 + doubleup=true` | `2XFREE` | 免费+双倍上传 |
| `free=50` | `PERCENT_50` | 半价 |
| `free=25` | `PERCENT_25` | 75 折（PT-Forward 需新增） |
| `free=75` | `PERCENT_75` | 25 折（PT-Forward 需新增） |
| `doubleup=true + free=0` | `2XUP` | 仅双倍上传 |
| `free=50 + doubleup=true` | `2X50` | 半价+双倍 |
| `free=0 + doubleup=false` | `NONE` | 无优惠 |

> **注意**: UNIT3D 使用百分比整数（0-100），而非固定枚举。PT-Forward DiscountLevel 需要新增 `PERCENT_25` 和 `PERCENT_75` 两个级别以完整映射。

### 6.5 API 中的免费字段

```json
{
  "free": 100,
  "doubleup": true,
  "refundable": false,
  "fl_until": "2026-04-20T00:00:00Z",
  "du_until": "2026-04-20T00:00:00Z"
}
```

搜索过滤时使用 `free` 参数（整数数组）：

```
GET /api/torrents/filter?free[]=100        → 仅完全免费
GET /api/torrents/filter?free[]=50&free[]=100  → 半价和全免费
GET /api/torrents/filter?doubleup=1         → 双倍上传
```

---

## 七、Tracker 协议

### 7.1 Announce URL

```
GET /announce/{passkey}?info_hash={binary}&peer_id={binary}&port={int}&uploaded={int}&downloaded={int}&left={int}
```

### 7.2 参数

**必填**:

| 参数 | 类型 | 验证 |
|------|------|------|
| `info_hash` | binary(20) | 恰好 20 字节 |
| `peer_id` | binary(20) | 恰好 20 字节 |
| `port` | int | ≥ 1024 (stopped 除外), ≤ 65535 |
| `uploaded` | int | ≥ 0 |
| `downloaded` | int | ≥ 0 |
| `left` | int | ≥ 0 |

**可选**:

| 参数 | 默认 | 说明 |
|------|------|------|
| `event` | `""` | started/completed/stopped/paused |
| `numwant` | 25 | 最大 25 |
| `corrupt` | 0 | ≥ 0 |
| `key` | `""` | 客户端 key |

### 7.3 端口黑名单

`8080, 8081, 1214, 3389, 4662, 6346, 6347, 6699`

### 7.4 客户端验证

- User-Agent 必填，max 64 字符
- 屏蔽浏览器 UA（Mozilla/Chrome/Safari 等）
- 屏蔽爬虫
- 屏蔽 Aria2（通过 `want-digest` header 检测）
- 屏蔽 `blacklist_clients` 表中的 peer_id 前缀
- 屏蔽请求头含 `accept-language`, `referer`, `accept-charset`, `want-digest`

### 7.5 响应格式（bencode）

**成功**:

```
d
  8:complete    i{seeder_count}e
  10:downloaded i{times_completed}e
  10:incomplete i{leecher_count}e
  8:interval    i{random 1800-3600}e
  12:min interval i{random 1710-1800}e
  5:peers       {len}:{compact_ipv4_peers}
  6:peers6      {len}:{compact_ipv6_peers}
e
```

**失败**:

```
d
  14:failure reason {len}:{message}
  8:interval    i1800e
  12:min interval i1800e
e
```

### 7.6 关键参数

| 参数 | 值 | 说明 |
|------|-----|------|
| announce 最小间隔 | 1800s (30min) | 随机 1800-3600 |
| 重复 announce 锁 | 30s | 同 user+torrent+peer_id+event |
| 每用户每种子连接上限 | 3 | `config('announce.rate_limit')` |
| 下载槽系统 | 可配置 | 按 group.download_slots 限制 |

### 7.7 Scrape

UNIT3D **不提供** 独立 scrape 端点。

### 7.8 外部 Tracker 支持

UNIT3D 支持独立部署的 [UNIT3D-Announce](https://github.com/UNIT3D/UNIT3D-Announce)（高性能外部 Tracker）：

```php
// config/announce.php
'external_tracker' => [
    'is_enabled'   => false,       // 启用外部 Tracker
    'host'         => env('EXTERNAL_TRACKER_HOST'),
    'port'         => env('EXTERNAL_TRACKER_PORT'),
    'unix_socket'  => env('EXTERNAL_TRACKER_SOCKET'),
    'key'          => env('EXTERNAL_TRACKER_KEY'),
],
```

---

## 八、用户 API

### 8.1 用户信息

```
GET /api/user
Authorization: Bearer <token>
```

### 8.2 用户相关 Web 端点

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/users/{username}` | 用户资料 |
| GET | `/users/{username}/bookmarks` | 书签列表 |
| POST | `/bookmarks/{torrentId}` | 添加书签 |
| DELETE | `/bookmarks/{torrentId}` | 删除书签 |
| GET | `/users/{username}/wishes` | 心愿单 |
| POST | `/users/{username}/wishes` | 添加心愿 |
| DELETE | `/users/{username}/wishes/{wish}` | 删除心愿 |
| GET | `/users/{username}/history` | 下载/做种历史 |
| GET | `/users/{username}/active` | 活跃种子 |
| DELETE | `/users/{username}/active` | 清除幽灵连接 |
| GET | `/users/{username}/notifications` | 通知列表 |
| PATCH | `/users/{username}/notifications/{id}` | 标记已读 |

---

## 九、数据模型

### 9.1 torrents 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int PK | |
| `name` | string(255) | 标题（唯一） |
| `description` | text | BBCode 描述 |
| `mediainfo` | text? | MediaInfo 输出 |
| `bdinfo` | text? | BDInfo 输出 |
| `info_hash` | binary(20) | SHA1(info_dict) |
| `file_name` | string | 磁盘存储文件名 |
| `num_file` | int | 文件数 |
| `folder` | string? | 顶层文件夹名 |
| `size` | float | 总大小（字节） |
| `nfo` | text? | NFO 内容 |
| `leechers` | int | 下载数 |
| `seeders` | int | 做种数 |
| `times_completed` | int | 完成数 |
| `category_id` | int FK | 分类 |
| `type_id` | int FK | 类型 |
| `resolution_id` | int FK? | 分辨率 |
| `region_id` | int FK? | 地区 |
| `distributor_id` | int FK? | 发行商 |
| `user_id` | int FK | 上传者 |
| `imdb` | int | IMDB 数字 ID |
| `tvdb` | int | TVDB ID |
| `tmdb_movie_id` | int? | TMDB Movie ID |
| `tmdb_tv_id` | int? | TMDB TV ID |
| `mal` | int | MAL ID |
| `igdb` | int? | IGDB ID |
| `season_number` | int? | 季 |
| `episode_number` | int? | 集 |
| `free` | int | 免费百分比 0-100 |
| `doubleup` | bool | 双倍上传 |
| `refundable` | bool | 可退还 |
| `fl_until` | datetime? | 免费到期 |
| `du_until` | datetime? | 双倍到期 |
| `status` | enum | 0=PENDING, 1=APPROVED, 2=REJECTED, 3=POSTPONED |
| `moderated_at` | datetime | 审核时间 |
| `moderated_by` | int? | 审核人 |
| `anon` | bool | 匿名 |
| `sticky` | bool | 置顶 |
| `internal` | int | 内部发布 |
| `personal_release` | bool | 个人发布 |
| `highspeed` | int | 高速 |
| `bumped_at` | datetime | 最后顶时间 |
| `balance` | int | 平衡 |
| `balance_offset` | int | 平衡偏移 |
| `created_at` / `updated_at` | datetime | 时间戳 |
| `deleted_at` | datetime? | 软删除 |

**全局 Scope**: `ApprovedScope`（默认仅查询 status=APPROVED 的种子）

### 9.2 users 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int PK | |
| `username` | string | 唯一用户名 |
| `email` | string | 邮箱 |
| `password` | string | bcrypt 哈希 |
| `passkey` | string(32) | Tracker 认证 |
| `rsskey` | string(32) | RSS 认证 |
| `api_token` | string? | API 认证 |
| `group_id` | int FK | 用户组 |
| `uploaded` | int | 总上传字节 |
| `downloaded` | int | 总下载字节 |
| `fl_tokens` | int | 免费令牌数 |
| `seedbonus` | decimal(18,2) | 魔力值 |
| `invites` | int | 邀请数 |
| `hitandruns` | int | H&R 数 |
| `can_upload` | bool | 上传权限 |
| `can_download` | bool | 下载权限 |
| `can_chat` | bool | 聊天权限 |
| `can_comment` | bool | 评论权限 |
| `can_request` | bool | 求种权限 |
| `can_invite` | bool | 邀请权限 |
| `is_donor` | bool | 捐赠者 |
| `is_lifetime` | bool | 终身捐赠 |
| `read_rules` | int | 已读规则 |
| `last_login` | datetime | 最后登录 |
| `last_action` | datetime | 最后活动 |

### 9.3 peers 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int PK | |
| `peer_id` | string | 客户端 peer ID |
| `ip` | binary | IP 地址 |
| `port` | int | 端口 |
| `agent` | string | User-Agent |
| `uploaded` | int | 已上传 |
| `downloaded` | int | 已下载 |
| `left` | int | 剩余 |
| `seeder` | bool | 是否做种 |
| `connectable` | bool | 可连接 |
| `active` | bool | 活跃 |
| `torrent_id` | int FK | |
| `user_id` | int FK | |

### 9.4 history 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | int FK | |
| `torrent_id` | int FK | |
| `agent` | string | 客户端 |
| `uploaded` | int | 实际上传 |
| `actual_uploaded` | int | |
| `client_uploaded` | int | 客户端上报 |
| `downloaded` | int | 实际下载 |
| `actual_downloaded` | int | |
| `client_downloaded` | int | 客户端上报 |
| `refunded_download` | int | 退还的下载 |
| `seeder` | bool | |
| `active` | bool | |
| `seedtime` | int | 做种秒数 |
| `immune` | bool | 免疫 H&R |
| `hitrun` | bool | H&R 标记 |
| `prewarned_at` | datetime? | 预警时间 |
| `completed_at` | datetime? | 完成时间 |

### 9.5 groups 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int PK | |
| `slug` | string | 标识符（如 user, vip, moderator 等） |
| `name` | string | 显示名称 |
| `position` | int | 排序 |
| `level` | int | 等级 |
| `color` | string | 颜色 |
| `icon` | string | 图标 |
| `effect` | string | 特效 |
| `download_slots` | int | 下载槽位数 |
| `is_immune` | bool | 免疫 H&R |
| `is_freeleech` | bool | 分组免费 |
| `is_double_upload` | bool | 分组双倍 |
| `is_modo` | bool | 版主 |
| `is_admin` | bool | 管理员 |
| `is_trusted` | bool | 可信 |
| `is_editor` | bool | 编辑 |
| `is_internal` | bool | 内部 |
| `can_upload` | bool | 上传权限 |

### 9.6 其他关键表

| 表名 | 说明 |
|------|------|
| `torrent_files` | id, name, size, torrent_id |
| `bookmarks` | user_id, torrent_id |
| `wishes` | user_id, tmdb_movie_id, tmdb_tv_id |
| `freeleech_tokens` | user_id, torrent_id |
| `personal_freeleech` | user_id |
| `featured_torrents` | user_id, torrent_id |
| `rss` | id, user_id, name, is_private, json_torrent |
| `torrent_downloads` | user_id, torrent_id, type |
| `automatic_torrent_freeleech` | category_id, type_id, resolution_id |

---

## 十、配置参考

### 10.1 config/torrent.php

| Key | 默认值 | 说明 |
|-----|--------|------|
| `download_check_page` | `0` | 下载确认页 |
| `source` | `'UNIT3D'` | 下载 .torrent 的 source 标签 |
| `created_by` | `'Edited by UNIT3D'` | created_by 字段 |
| `created_by_append` | `true` | 追加到已有 created_by |
| `comment` | `'This torrent was downloaded from UNIT3D'` | .torrent comment |
| `magnet` | `0` | 磁力链接开关 |

### 10.2 config/announce.php

| Key | 默认值 | 说明 |
|-----|--------|------|
| `external_tracker.is_enabled` | `false` | 外部 Tracker |
| `rate_limit` | `3` | 每用户每种子最大连接数 |
| `connectable_check` | `false` | 可连接检查（泄露 IP！） |
| `connectable_check_interval` | `1800` | 检查间隔（秒） |
| `slots_system.enabled` | `true` | 下载槽位系统 |
| `log_announces` | `false` | 记录所有 announce |

### 10.3 config/other.php

| Key | 默认值 | 说明 |
|-----|--------|------|
| `title` | `'UNIT3D'` | 站点标题 |
| `freeleech` | `false` | 全局免费 |
| `doubleup` | `false` | 全局双倍 |
| `refundable` | `false` | 全局可退还 |
| `ratio` | `0.4` | 最低下载分享率 |
| `invite-only` | `true` | 仅邀请注册 |
| `invite_expire` | `14` | 邀请过期天数 |
| `default_upload` | `'53687091200'` | 初始上传 50GB |
| `default_download` | `'1073741824'` | 初始下载 1GB |

### 10.4 config/hitrun.php

| Key | 默认值 | 说明 |
|-----|--------|------|
| `enabled` | `true` | H&R 系统开关 |
| `seedtime` | `604800` | 保种时间（7 天） |
| `max_warnings` | `3` | 最大 H&R 警告数 |
| `grace` | `3` | 宽限期（天） |
| `expire` | `14` | 警告过期（天） |
| `prewarn` | `1` | 预警期（天） |

---

## 十一、PT-Forward 集成要点

### 11.1 与 §13 站点管理模块的映射

| PT-Forward 字段 | UNIT3D 来源 | 说明 |
|-----------------|------------|------|
| `framework` | `"unit3d"` | 固定值 |
| `passkey` | `users.passkey` | Tracker 用 |
| `rsskey` | `users.rsskey` | RSS + 下载用 |
| `api_key` | `users.api_token` | API 用 |
| `download_mode` | `"template"` | URL 模板 |
| `download_url_template` | `{base_url}/torrent/download/{torrent_id}.{rsskey}` | |
| `hash_strategy` | `"fake_from_id"` | RSS 无真实 hash |
| `size_strategy` | `"xml_tag"` | `<contentlength>` |
| `id_strategy` | `"link_regex"` | |
| `id_pattern` | `"download/(\d+)"` | |

### 11.2 与 §8 RSS 模块的映射

| PT-Forward RSS 字段 | UNIT3D RSS 映射 |
|---------------------|-----------------|
| `<title>` → `TorrentEvent.Title` | 直接映射 |
| `<contentlength>` → `TorrentEvent.Size` | 字节 |
| `<link>` → 下载链接 → `TorrentEvent.DownloadURL` | 含 rsskey |
| `<guid>` → `TorrentEvent.TorrentID` | 种子 ID |
| `<category>` → 分类信息 | 可用于过滤 |
| `<description>` → seeders/leechers | HTML 解析 |

### 11.3 与 §11 发种流水线的映射

| PT-Forward 字段 | UNIT3D 上传字段 | 说明 |
|-----------------|----------------|------|
| `torrent_data` | `torrent` (file) | .torrent 文件 |
| `title` | `name` | 标题 |
| `description` | `description` | BBCode |
| `category` → mapped | `category_id` | 需映射为 ID |
| `codec` → mapped | `type_id` | 需映射为 ID |
| `resolution` → mapped | `resolution_id` | movie/tv 必须 |
| `mediainfo` | `mediainfo` | 原始文本 |
| — | `anonymous` | 固定 0 |

### 11.4 与 §17 DiscountLevel 的映射

UNIT3D 的 `free` 字段是 0-100 的**百分比整数**，需转换为 PT-Forward 枚举：

```go
func Unit3DDiscountToLevel(free int, doubleup bool) DiscountLevel {
    if free >= 100 && doubleup {
        return Discount2xFree
    }
    if free >= 100 {
        return DiscountFree
    }
    if free == 75 && doubleup {
        return Discount2xUp // 近似
    }
    if free == 75 {
        return DiscountPercent75
    }
    if free == 50 && doubleup {
        return Discount2x50
    }
    if free == 50 {
        return DiscountPercent50
    }
    if free == 25 {
        return DiscountPercent25
    }
    if doubleup {
        return Discount2xUp
    }
    return DiscountNone
}
```

> **注意**: UNIT3D 支持 `free=25` 和 `free=75`，PT-Forward 需在 DiscountLevel 枚举中新增 `PERCENT_25` 和 `PERCENT_75`。

### 11.5 刷流选种 S/L 数据获取

UNIT3D 的 seeders/leechers 可通过以下方式获取：

**批量获取**（§18.5 `GetBatchSLData`）:
```
GET /api/torrents/filter?perPage=100&sortField=created_at&sortDirection=desc
```
响应中每个种子含 `seeders` 和 `leechers` 字段。

**精确获取**（§18.5 `GetPreciseSLData`）:
```
GET /api/torrents/{id}
```
响应含 `seeders`, `leechers`, `times_completed` 字段。

### 11.6 免费状态检测

UNIT3D 种子详情 API 直接返回免费字段：

```json
{
  "free": 100,
  "doubleup": true,
  "fl_until": "2026-04-20T00:00:00Z",
  "du_until": "2026-04-20T00:00:00Z"
}
```

但需注意**隐藏免费**（个人免费、免费令牌、精选、分组、全局）不在 API 响应中体现，这些是**用户级**优惠，不同用户看到的效果不同。

### 11.7 完整凭据清单

| 凭据 | 获取方式 | PT-Forward 用途 |
|------|---------|-----------------|
| `passkey` | Web 登录 → `/users/{username}/passkeys` | Tracker announce（辅种做种用） |
| `rsskey` | Web 登录 → 自动生成 | RSS 订阅 + 种子下载 |
| `api_token` | Web 登录 → `/users/{username}/apikeys` | REST API（搜索/上传/详情） |
| `Cookie` | Web 登录 → session cookie | 页面抓取（如需） |

> **PT-Forward 站点配置**: `api_key` 存储 api_token，`rsskey` 存储 rsskey，`passkey` 存储 passkey。

---

## 附录 A: UNIT3D 特殊行为

### A.1 种子审核机制

UNIT3D 有种子审核队列，非可信用户上传的种子需人工审核后才可见。API 返回的种子都是已审核（APPROVED）的。

### A.2 Bencode V2 拒绝

UNIT3D 拒绝 BitTorrent v2 和 hybrid 种子（检查 `piece_layers` 键）。

### A.3 无 Scrape 端点

UNIT3D 不实现标准 scrape 端点，客户端无法批量查询种子状态。

### A.4 信息脱敏

`TorrentTools::anonymizeMediainfo()` 会自动去除 MediaInfo 中的个人信息（如用户名路径等）。

### A.5 H&R 系统

UNIT3D 有完整的 H&R 系统，默认保种 7 天（604800 秒），最多 3 次警告后禁用下载。

### A.6 文件数分批插入

TorrentFile 记录按 21666 条/批插入数据库（优化大批量文件种子）。

---

## 附录 B: 与其他框架对比

| 维度 | UNIT3D | NexusPHP | M-Team | Gazelle |
|------|--------|----------|--------|---------|
| 语言 | PHP/Laravel | PHP(原生) | Go(自定义) | PHP(原生) |
| API 认证 | Token | Cookie/Passkey | API Key | Cookie |
| RSS 认证 | rsskey(路径) | passkey(query) | sign+uid | 4 参数 |
| 免费类型 | 0-100% + bool | 枚举字符串 | 组合枚举 | CSS class |
| 种子下载 | rsskey(路径) | passkey(query) | genDlToken | authkey(query) |
| 种子上传 | multipart API | multipart 表单 | JSON+file | multipart 表单 |
| 审核机制 | 有（可配） | 无 | 有 | 有 |
| Scrape | 无 | 有 | 无 | 无 |
| 搜索引擎 | Meilisearch | SQL | 内置 | SQL |

---

*最后更新: 2026-04-15*
*源码版本: UNIT3D v8.x (dev branch)*
*分析文件数: 30+ 源文件*
