# NexusPHP 公开 API 完整参考指南

> **文档版本**: v1.0  
> **最后更新**: 2026-04-12  
> **数据来源**: [Apifox官方API文档](https://apifox.com/apidoc/shared/43608c09-bab0-4e2e-9a56-77ffa629c8e0) + 本地代码实现深度分析  
> **适用版本**: NexusPHP 1.9+

---

## 目录

1. [架构概述](#1-架构概述)
2. [认证机制](#2-认证机制)
3. [统一响应格式](#3-统一响应格式)
4. [通用参数系统](#4-通用参数系统)
5. [核心API端点](#5-核心api端点)
6. [数据模型定义](#6-数据模型定义)
7. [折扣等级系统](#7-折扣等级系统)
8. [错误处理机制](#8-错误处理机制)
9. [本地实现代码映射](#9-本地实现代码映射)
10. [与M-Team(mTorrent)架构对比](#10-与m-teammtorrent-架构对比)
11. [最佳实践与示例代码](#11-最佳实践与示例代码)

---

## 1. 架构概述

### 1.1 NexusPHP API 双轨制架构

NexusPHP采用**传统HTML + 现代RESTful API**的双轨制架构：

| 架构层 | 访问方式 | 数据格式 | 适用场景 |
|--------|----------|----------|----------|
| **传统Web界面** | 浏览器Cookie | HTML页面 | 用户手动操作 |
| **RESTful API (1.9+)** | Bearer Token/Cookie | JSON | 程序化访问 |
| **第三方专用API** | Passkey认证 | JSON | 辅种工具(ptdog等) |

### 1.2 本地代码实现的架构抽象

根据 [types.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/types.go) 的定义：

```go
// SiteKind 表示PT站点架构类型
type SiteKind string

const (
    SiteNexusPHP SiteKind = "nexusphp"     // NexusPHP架构（最常见）
    SiteMTorrent SiteKind = "mtorrent"      // M-Team自定义API
    SiteUnit3D   SiteKind = "unit3d"        // Unit3D架构
    SiteGazelle   SiteKind = "gazelle"       // Gazelle架构
    SiteHDDolby   SiteKind = "hddolby"      // HDDolby REST API
    SiteRousi     SiteKind = "rousi"        // RousiPro架构
)
```

### 1.3 认证方法分类

```go
// AuthMethod 认证方式枚举
type AuthMethod string

const (
    AuthMethodCookie          AuthMethod = "cookie"             // 浏览器Cookie
    AuthMethodAPIKey          AuthMethod = "api_key"            // API密钥
    AuthMethodCookieAndAPIKey AuthMethod = "cookie_and_api_key" // 混合认证
    AuthMethodPasskey         AuthMethod = "passkey"            // Passkey（下载/RSS）
)
```

---

## 2. 认证机制

### 2.1 三种认证方式详解

#### 方式一：Cookie认证（传统方式）

**适用场景**: 传统HTML页面访问、旧版API兼容

```http
GET /torrents.php HTTP/1.1
Host: nexusphp.example.com
Cookie: c_secure_uid=xxx; c_secure_pass=xxx; c_secure_ssl=xxx;
User-Agent: Mozilla/5.0 ...
```

**本地实现** ([nexusphp_driver.go:48-55](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/nexusphp_driver.go#L48-L55)):

```go
type NexusPHPDriver struct {
    BaseURL        string
    Cookie         string          // Cookie存储在这里
    Selectors      SiteSelectors
    httpClient     *SiteHTTPClient
}
```

#### 方式二：Bearer Token认证（新版RESTful API）

**适用场景**: 1.9+版本的JSON API端点

```http
GET /api/user/1 HTTP/1.1
Host: nexusphp.example.com
Authorization: Bearer your_token_here
Content-Type: application/json
```

#### 方式三：Passkey认证（第三方专用）

**适用场景**: pieces_hash查询、下载链接生成

```http
POST /api/pieces-hash HTTP/1.1
Host: nexusphp.example.com
X-Passkey: your_passkey_here
Content-Type: application/json

{
    "pieces_hash": ["abc123...", "def456..."]
}
```

### 2.2 认证方式选择指南

| 使用场景 | 推荐认证方式 | 原因 |
|----------|-------------|------|
| 爬取种子列表 | Cookie | 需要解析HTML |
| 获取用户信息 | Bearer Token | RESTful API更结构化 |
| 辅种查询 | Passkey | 第三方专用接口 |
| 下载种子文件 | Passkey | download.php需要passkey |
| 发布种子 | Bearer Token + Cookie | 需要完整会话信息 |

---

## 3. 统一响应格式

### 3.1 标准响应结构

所有RESTful API端点遵循统一的响应格式：

```json
{
    "ret": 0,
    "msg": "success",
    "data": {
        "data": []
    },
    "time": 0.023,
    "rid": "req_abc123def456"
}
```

### 3.2 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `ret` | integer | 结果码，**0表示成功**，其他均为失败 |
| `msg` | string | 描述信息 |
| `data` | object | 数据包裹层 |
| `data.data` | array\|object | **主要目标数据**。失败时无此字段！ |
| `time` | number | 后端响应耗时（单位：秒） |
| `rid` | string | 请求ID（用于追踪和调试） |

> ⚠️ **重要**: 失败响应中**没有** `data.data` 字段！客户端必须先检查 `ret` 字段。

---

## 4. 通用参数系统

### 4.1 查询参数一览

| 参数名 | 类型 | 功能说明 | 示例 |
|--------|------|----------|------|
| `includes` | string | 包含关联资源 | `includes=user,category` |
| `include_fields[resourceName]` | string | 包含指定资源的字段 | `include_fields[torrent]=title,size` |
| `filter[fieldName][operator]` | mixed | AND搜索条件 | `filter[category_id][eq]=1` |
| `filter_any[fieldName][operator]` | mixed | OR搜索条件 | `filter_any[title][like]=example` |
| `sorts` | string | 排序字段（默认升序，前加`-`降序） | `sorts=-created_at` |
| `per_page` | integer | 每页数量（建议≤100） | `per_page=50` |
| `page` | integer | 页码（从1开始） | `page=2` |

### 4.2 支持的过滤操作符

| 操作符 | 含义 | 示例 |
|--------|------|------|
| `eq` | 等于 | `filter[id][eq]=123` |
| `gt` | 大于 | `filter[size][gt]=1073741824` |
| `gte` | 大于等于 | `filter[seeders][gte]=10` |
| `lt` | 小于 | `filter[leechers][lt]=100` |
| `lte` | 小于等于 | `filter[created_at][lte]=2024-01-01` |
| `like` | 模糊匹配 | `filter[title][like]=%Movie%` |
| `in` | 范围查询（逗号分隔） | `filter[category_id][in]=1,2,3` |

### 4.3 资源名称 (resourceName)

| resourceName | 对应资源 | 可用字段示例 |
|--------------|----------|-------------|
| `torrent` | 种子 | title, size, seeders, leechers, snatched |
| `user` | 用户 | username, uploaded, downloaded, ratio, bonus |

---

## 5. 核心API端点

### 5.1 用户模块

#### 5.1.1 获取用户详情

**端点**: `GET /api/user/{id}` 或 `GET /api/user/current`  
**认证**: 需要  
**描述**: 获取指定用户或当前登录用户信息

**响应示例**:
```json
{
    "ret": 0,
    "msg": "success",
    "data": {
        "data": {
            "id": 12345,
            "username": "example_user",
            "uploaded": 1099511627776,
            "downloaded": 549755813888,
            "ratio": 2.0,
            "bonus": 125000.50,
            "class": "Power User"
        }
    }
}
```

**本地实现对应** ([nexusphp_driver.go:874-882](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/nexusphp_driver.go#L874-L882)):

```go
func (d *NexusPHPDriver) GetUserInfo(ctx context.Context) (UserInfo, error) {
    if d.siteDefinition != nil && d.siteDefinition.UserInfo != nil {
        return d.getUserInfoWithDefinition(ctx)
    }
    return d.getUserInfoLegacy(ctx)
}
```

---

### 5.2 种子模块

#### 5.2.1 获取种子列表

**端点**: `GET /api/torrents`  
**认证**: 需要（部分站点公开可访问）  
**描述**: 分页获取种子列表，支持搜索、过滤、排序

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `keyword` | string | 否 | - | 搜索关键词 |
| `category_id` | integer | 否 | - | 分类ID过滤 |
| `filter[seeders][gte]` | integer | 否 | - | 做种数≥N |
| `sorts` | string | 否 | `-created_at` | 排序字段 |
| `per_page` | integer | 否 | 50 | 每页数量（最大100） |
| `page` | integer | 否 | 1 | 页码 |

**响应示例**:
```json
{
    "ret": 0,
    "msg": "success",
    "data": {
        "data": [
            {
                "id": 12345,
                "title": "Example.Movie.2024.2160p.UHD.BluRay.x265-TFPG",
                "size": 54975581389,
                "seeders": 150,
                "leechers": 23,
                "snatched": 500,
                "discount_level": "FREE"
            }
        ],
        "total": 1000,
        "current_page": 1
    }
}
```

#### 5.2.2 获取种子详情

**端点**: `GET /api/torrent/{id}`  
**认证**: 需要  
**描述**: 获取种子的完整详细信息

**本地实现对应** ([nexusphp_driver.go:1907-1945](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/nexusphp_driver.go#L1907-L1945)):

```go
func (d *NexusPHPDriver) GetTorrentDetail(ctx context.Context, guid, link, _ string) (*TorrentItem, error) {
    torrentID := extractTorrentIDFromURL(link)
    
    parser := NewNexusPHPParserFromDefinition(d.GetSiteDefinition())
    detailInfo := parser.ParseAll(res.Document.Selection)

    item := &TorrentItem{
        ID:              detailInfo.TorrentID,
        Title:           detailInfo.Title,
        SizeMB:          detailInfo.SizeMB,
        DiscountLevel:   detailInfo.DiscountLevel,
        DiscountEndTime: detailInfo.DiscountEnd,
        HasHR:           detailInfo.HasHR,
    }
    return item, nil
}
```

**详情页解析器实现** ([nexusphp_parser.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/nexusphp_parser.go)):

```go
type TorrentDetailInfo struct {
    TorrentID     string
    Title         string
    SizeMB        float64
    DiscountLevel DiscountLevel
    DiscountEnd   time.Time
    HasHR         bool
}

func (p *NexusPHPParser) ParseAll(doc *goquery.Selection) *TorrentDetailInfo {
    title, torrentID := p.ParseTitleAndID(doc)
    discount, endTime := p.ParseDiscount(doc)
    return &TorrentDetailInfo{
        TorrentID:     torrentID,
        Title:         title,
        SizeMB:        p.ParseSizeMB(doc),
        DiscountLevel: discount,
        DiscountEnd:   endTime,
        HasHR:         p.ParseHR(doc),
    }
}
```

#### 5.2.3 发布种子

**端点**: `POST /api/torrent`  
**认证**: 需要（通常需要上传权限）  
**描述**: 发布新种子

**请求体** (multipart/form-data):
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | 是 | 种子标题 |
| `category_id` | integer | 是 | 分类ID |
| `description` | string | 是 | 种子描述（支持BBCode） |
| `file` | file | 是 | .torrent文件 |
| `imdb_id` | string | 否 | IMDB ID |

---

### 5.3 评论模块

#### 5.3.1 获取评论列表

**端点**: `GET /api/torrent/{id}/comments`  
**描述**: 获取指定种子的所有评论

---

### 5.4 收藏模块

#### 5.4.1 新增收藏

**端点**: `POST /api/bookmark`  
**请求体**: `{"torrent_id": 12345}`

#### 5.4.2 删除收藏

**端点**: `DELETE /api/bookmark/{id}` 或 `POST /api/bookmark/delete`

---

### 5.5 第三方专用接口

#### 5.5.1 pieces_hash 批量查询（辅种核心）

**端点**: `POST /api/pieces-hash`  
**认证**: Passkey（第三方专用）  
**描述**: 根据pieces_hash批量查询匹配的种子（用于跨站辅种）

> 📖 **详细文档**: 参见 [09-nexusphp-hash-algorithm.md](file:///home/incast/PT-Forward/docs/09-nexusphp-hash-algorithm.md)

**控制器实现** ([TorrentController.php:154-210](file:///home/incast/PT-Forward/examples/nexusphp/app/Http/Controllers/TorrentController.php#L154-L210)):

```php
public function queryByPiecesHash(Request $request)
{
    $request->validate([
        'pieces_hash' => 'required|array',
        'pieces_hash.*' => 'string|size:40',
    ]);

    $piecesHashes = $request->input('pieces_hash');
    $results = app(PiecesHashRepository::class)->getPiecesHashCache($piecesHashes);

    return response()->json([
        'ret' => 0,
        'msg' => 'success',
        'data' => [
            'data' => $results,
            'matched_count' => count(array_filter($results)),
        ]
    ]);
}
```

**Repository层高性能实现**（Redis Pipeline批量查询）：

```php
public function getPiecesHashCache($piecesHash): array
{
    $pipe = NexusDB::redis()->multi(\Redis::PIPELINE);
    foreach ($piecesHash as $hash) {
        $pipe->hGet(self::PIECES_HASH_CACHE_KEY, $hash);
    }
    $results = $pipe->exec();
    
    $out = [];
    foreach ($results as $item) {
        $arr = json_decode($item, true);
        if (is_array($arr) && isset($arr['torrent_id'], $arr['pieces_hash'])) {
            $out[$arr['pieces_hash']] = $arr['torrent_id'];
        }
    }
    return $out;
}
```

**Python客户端示例**：

```python
def query_pieces_hash(hash_list):
    response = requests.post(
        f"{NEXUSPHP_API_BASE}/api/pieces-hash",
        headers={"X-Passkey": PASSKEY, "Content-Type": "application/json"},
        json={"pieces_hash": hash_list}
    )
    result = response.json()
    if result["ret"] != 0:
        raise Exception(f"Query failed: {result['msg']}")
    return result["data"]["data"]
```

---

## 6. 数据模型定义

### 6.1 种子模型 (Torrent)

| 字段名 | 类型 | 说明 | 示例值 |
|--------|------|------|--------|
| `id` | integer | 种子唯一标识 | 12345 |
| `title` | string | 种子标题 | "Movie.2024.2160p..." |
| `subtitle` | string | 副标题 | "示例电影" |
| `size` | integer | 大小（字节） | 54975581389 |
| `seeders` | integer | 做种人数 | 150 |
| `leechers` | integer | 下载人数 | 23 |
| `snatched` | integer | 完成次数 | 500 |
| `discount_level` | string | 折扣等级 | "FREE", "2XUP" |
| `has_hr` | boolean | 是否H&R规则 | true/false |
| `info_hash` | string | Info Hash（SHA1） | 40字符hex字符串 |
| `pieces_hash` | string | Pieces Hash（SHA1） | 40字符hex字符串 |
| `created_at` | datetime | 发布时间 | "2024-01-15 10:30:00" |

### 6.2 用户模型 (User)

| 字段名 | 类型 | 说明 | 示例值 |
|--------|------|------|--------|
| `id` | integer | 用户唯一标识 | 12345 |
| `username` | string | 用户名 | "example_user" |
| `uploaded` | integer | 上传量（字节） | 1099511627776 |
| `downloaded` | integer | 下载量（字节） | 549755813888 |
| `ratio` | float | 分享率 | 2.0 |
| `bonus` | float | 魔力值 | 125000.50 |
| `class` | string | 用户等级 | "Power User" |

---

## 7. 折扣等级系统

### 7.1 折扣等级定义

根据 [types.go:204-222](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/types.go#L204-L222):

```go
type DiscountLevel string

const (
    DiscountNone       DiscountLevel = "NONE"       // 无折扣
    DiscountFree       DiscountLevel = "FREE"       // 免费下载
    Discount2xFree     DiscountLevel = "2XFREE"     // 双倍免费
    DiscountPercent50  DiscountLevel = "PERCENT_50" // 50%下载
    DiscountPercent30  DiscountLevel = "PERCENT_30" // 30%下载
    DiscountPercent70  DiscountLevel = "PERCENT_70" // 70%下载
    Discount2xUp       DiscountLevel = "2XUP"       // 双倍上传
    Discount2x50       DiscountLevel = "2X50"       // 双倍上传+50%下载
)
```

### 7.2 折扣效果对照表

| 折扣等级 | 下载计量 | 上传计量 | 图标关键字 | CSS类名 |
|----------|----------|----------|------------|----------|
| **NONE** | 100% | 1x | - | - |
| **FREE** | **0%** | 1x | `free`, `pro_free` | `free` |
| **2XFREE** | **0%** | **2x** | `twoupfree` | `twoupfree` |
| **PERCENT_30** | **30%** | 1x | `thirtypercent` | `thirtypercent` |
| **PERCENT_50** | **50%** | 1x | `halfdown` | `halfdown` |
| **2XUP** | 100% | **2x** | `twoup` | `twoup` |
| **2X50** | **50%** | **2x** | `twouphalfdown` | `twouphalfdown` |

### 7.3 判断免费种子的辅助函数

```go
var FreeDiscountLevels = []DiscountLevel{DiscountFree, Discount2xFree}

func IsFreeTorrent(level DiscountLevel) bool {
    for _, freeLevel := range FreeDiscountLevels {
        if level == freeLevel { return true }
    }
    return false
}

func (d DiscountLevel) GetDownloadRatio() float64 {
    switch d {
    case DiscountFree, Discount2xFree: return 0.0
    case DiscountPercent30: return 0.3
    case DiscountPercent50, Discount2x50: return 0.5
    default: return 1.0
    }
}
```

---

## 8. 错误处理机制

### 8.1 标准错误码

| ret码 | HTTP状态码 | 含义 |
|-------|------------|------|
| **0** | 200 | 成功 |
| 400 | 400 | 请求参数错误 |
| 401 | 401 | 未认证 |
| 403 | 403 | 无权限 |
| 404 | 404 | 资源不存在 |
| 429 | 429 | 请求过于频繁 |
| 500 | 500 | 服务器内部错误 |

### 8.2 本地代码中的错误定义

根据 [types.go:96-106](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/types.go#L96-L106):

```go
var (
    ErrSiteNotFound       = errors.New("site not found")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrSessionExpired     = errors.New("session expired")
    ErrAuthFailed         = errors.New("authentication failed")
    ErrRateLimited        = errors.New("rate limited")
    ErrParseError         = errors.New("failed to parse response")
    ErrNetworkError       = errors.New("network error")
)
```

---

## 9. 本地实现代码映射

### 9.1 文件结构总览

```
examples/pt-tools/site/v2/
├── nexusphp_driver.go        # NexusPHP驱动主实现
├── nexusphp_parser.go        # 详情页解析器
├── types.go                  # 类型定义
└── site_definition.go        # 多站点配置
```

### 9.2 核心类与API端点映射

| 本地类/方法 | 功能说明 |
|-------------|---------|
| `NexusPHPDriver.Search()` | 搜索种子列表 |
| `NexusPHPDriver.GetTorrentDetail()` | 获取种子详情 |
| `NexusPHPDriver.GetUserInfo()` | 获取用户信息 |
| `NexusPHPParser.ParseAll()` | 解析详情页所有信息 |
| `DefaultNexusPHPSelectors()` | 返回标准CSS选择器配置 |

---

## 10. 与M-Team(mTorrent)架构对比

### 10.1 架构理念差异

| 特性 | **NexusPHP** | **M-Team (mTorrent)** |
|------|--------------|-----------------------|
| **架构理念** | 传统PT + 渐进式API现代化 | 云原生微服务架构 |
| **数据格式** | HTML为主，JSON API为辅 | 纯JSON RESTful API |
| **认证方式** | Cookie + Passkey + Bearer Token | x-api-key Header |
| **文档规范** | Apifox（社区维护） | OpenAPI 3.1.0（官方标准） |
| **辅种支持** | ✅ **原生支持** pieces_hash API | ⚠️ **间接支持** |

### 10.2 API设计对比

| 设计维度 | **NexusPHP** | **M-Team (mTorrent)** |
|----------|--------------|-----------------------|
| **响应格式** | `{ret, msg, data{data}, time, rid}` | `{code, data, message}` |
| **分页参数** | `page`, `per_page` | `pageNumber`, `pageSize` |
| **过滤语法** | `filter[field][operator]` | 内嵌在POST body中 |
| **错误码** | `ret` (integer) | `code` (string or number) |

### 10.3 迁移建议

```python
# 认证转换
# Old (NexusPHP): Cookie: c_secure_uid=xxx
# New (M-Team): x-api-key: your_api_key

# 响应解析
# NexusPHP: if response["ret"] == 0:
# M-Team:    if str(response["code"]) == "0":

# 分页参数
# NexusPHP: params = {"page": 1, "per_page": 50}
# M-Team:    payload = {"pageNumber": 1, "pageSize": 50}
```

---

## 11. 最佳实践与示例代码

### 11.1 Python完整客户端示例

```python
"""
NexusPHP API 完整客户端实现
支持RESTful API（1.9+）和传统HTML解析双模式
"""

import requests
import time
from typing import Optional, Dict, List, Any
from dataclasses import dataclass
from enum import Enum


class DiscountLevel(Enum):
    NONE = "NONE"
    FREE = "FREE"
    TWO_X_FREE = "2XFREE"
    PERCENT_30 = "PERCENT_30"
    PERCENT_50 = "PERCENT_50"
    TWO_X_UP = "2XUP"
    TWO_X_50 = "2X50"


@dataclass
class TorrentItem:
    id: int
    title: str
    size: int = 0
    seeders: int = 0
    leechers: int = 0
    snatched: int = 0
    discount: DiscountLevel = DiscountLevel.NONE
    has_hr: bool = False
    info_hash: str = ""
    pieces_hash: str = ""


@dataclass
class UserInfo:
    id: int
    username: str
    uploaded: int = 0
    downloaded: int = 0
    ratio: float = 0.0
    bonus: float = 0.0
    user_class: str = ""


class NexusPHPClient:
    def __init__(
        self,
        base_url: str,
        cookie: str = "",
        bearer_token: str = "",
        passkey: str = "",
        use_api: bool = True,
        rate_limit: float = 1.0,
        timeout: int = 30
    ):
        self.base_url = base_url.rstrip("/")
        self.cookie = cookie
        self.bearer_token = bearer_token
        self.passkey = passkey
        self.use_api = use_api
        self.rate_limit = rate_limit
        self.timeout = timeout
        
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
            'Accept': 'application/json, text/html */*',
        })
        
        if cookie:
            self.session.headers['Cookie'] = cookie
        if bearer_token:
            self.session.headers['Authorization'] = f'Bearer {bearer_token}'
        if passkey:
            self.session.headers['X-Passkey'] = passkey
        
        self._last_request = 0

    def _request(self, method: str, endpoint: str, params=None, json_data=None, expect_empty=False):
        elapsed = time.time() - self._last_request
        if elapsed < self.rate_limit:
            time.sleep(self.rate_limit - elapsed)
        
        url = f"{self.base_url}{endpoint}"
        response = getattr(self.session, method.lower())(url, params=params, json=json_data, timeout=self.timeout)
        self._last_request = time.time()
        
        if response.status_code == 429:
            raise Exception(f"Rate limited: {response.text}")
        elif response.status_code >= 400:
            raise Exception(f"HTTP {response.status_code}: {response.text}")
        
        if expect_empty or not response.text.strip():
            return {"ret": 0, "msg": "success"}
        
        try:
            result = response.json()
            if isinstance(result, dict) and "ret" in result:
                if result["ret"] != 0:
                    raise Exception(f"API Error ({result['ret']}): {result.get('msg', 'Unknown')}")
                return result
            return result
        except ValueError:
            return {"raw_html": response.text}

    def search_torrents(self, keyword="", category_id=0, min_seeders=0, page=1, per_page=50):
        params = {"page": page, "per_page": min(per_page, 100), "sorts": "-created_at"}
        if keyword:
            params["keyword"] = keyword
        if category_id > 0:
            params["filter[category_id][eq]"] = category_id
        if min_seeders > 0:
            params["filter[seeders][gte]"] = min_seeders
        
        result = self._request("GET", "/api/torrents", params=params)
        if "data" in result and "data" in result["data"]:
            return result["data"]["data"]
        return []

    def get_torrent_detail(self, torrent_id):
        result = self._request("GET", f"/api/torrent/{torrent_id}")
        if "data" in result and "data" in result["data"]:
            return result["data"]["data"]
        return None

    def get_user_info(self, user_id=None):
        endpoint = f"/api/user/{user_id}" if user_id else "/api/user/current"
        result = self._request("GET", endpoint)
        if "data" in result and "data" in result["data"]:
            return result["data"]["data"]
        return None

    def query_pieces_hash(self, hash_list):
        if not self.passkey:
            raise ValueError("Passkey is required")
        result = self._request("POST", "/api/pieces-hash", json_data={"pieces_hash": hash_list})
        if "data" in result and "data" in result["data"]:
            return result["data"]["data"]
        return {}


# 使用示例
if __name__ == "__main__":
    client = NexusPHPClient(
        base_url="https://hdsky.com",
        bearer_token="your_token_here",
        passkey="your_passkey_here",
        rate_limit=1.5
    )
    
    # 搜索种子
    torrents = client.search_torrents(keyword="Movie", min_seeders=10)
    print(f"找到 {len(torrents)} 个种子")
    
    # 获取用户信息
    user = client.get_user_info()
    if user:
        print(f"用户: {user['username']}, 分享率: {user['ratio']}")
    
    # 辅种查询
    matches = client.query_pieces_hash(["hash1...", "hash2..."])
    print(f"匹配到 {len(matches)} 个种子")
```

### 11.2 Go语言客户端示例（基于本地实现）

```go
// 使用本地NexusPHPDriver的简化示例
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    v2 "github.com/sunerpy/pt-tools/site/v2"
)

func main() {
    driver := &v2.NexusPHPDriver{
        BaseURL: "https://hdsky.com",
        Cookie:  "c_secure_uid=xxx; c_secure_pass=xxx",
    }
    
    ctx := context.Background()
    
    // 获取用户信息
    userInfo, err := driver.GetUserInfo(ctx)
    if err != nil {
        log.Fatalf("GetUserInfo failed: %v", err)
    }
    fmt.Printf("用户: %s, 分享率: %.2f\n", userInfo.Username, userInfo.Ratio)
    
    // 获取种子详情
    detail, err := driver.GetTorrentDetail(ctx, "12345", "", "")
    if err != nil {
        log.Fatalf("GetTorrentDetail failed: %v", err)
    }
    fmt.Printf("种子: %s, 折扣: %s\n", detail.Title, detail.DiscountLevel)
    
    // 判断是否为免费种子
    if v2.IsFreeTorrent(detail.DiscountLevel) {
        fmt.Println("✅ 这是一个免费种子！")
    }
}
```

---

## 附录A: 快速参考卡片

### A.1 常用端点速查

| 功能 | 方法 | 端点 | 认证 |
|------|------|------|------|
| 用户详情 | GET | `/api/user/{id}` | Bearer Token |
| 当前用户 | GET | `/api/user/current` | Bearer Token |
| 种子列表 | GET | `/api/torrents` | Bearer/Cookie |
| 种子详情 | GET | `/api/torrent/{id}` | Bearer/Cookie |
| 发布种子 | POST | `/api/torrent` | Bearer Token |
| 评论列表 | GET | `/api/torrent/{id}/comments` | 可选 |
| 新增收藏 | POST | `/api/bookmark` | Bearer Token |
| 删除收藏 | DELETE | `/api/bookmark/{id}` | Bearer Token |
| pieces_hash查询 | POST | `/api/pieces-hash` | **Passkey** |

### A.2 折扣等级速查

| 等级 | 下载 | 上传 | 关键字 |
|------|------|------|--------|
| NONE | 100% | 1x | - |
| FREE | **0%** | 1x | free |
| 2XFREE | **0%** | **2x** | twoupfree |
| 30% | **30%** | 1x | thirtypercent |
| 50% | **50%** | 1x | halfdown |
| 2XUP | 100% | **2x** | twoup |
| 2X50 | **50%** | **2x** | twouphalfdown |

### A.3 过滤操作符速查

| 操作符 | 含义 | 示例 |
|--------|------|------|
| eq | 等于 | `[id][eq]=123` |
| gt | 大于 | `[size][gt]=1GB` |
| gte | ≥ | `[seeders][gte]=10` |
| lt | 小于 | `[leechers][lt]=100` |
| lte | ≤ | `[time][lte]=now` |
| like | 模糊 | `[title][like]=%Movie%` |
| in | 范围 | `[cat][in]=1,2,3` |

---

## 附录B: 版本历史

| 版本 | 日期 | 作者 | 变更说明 |
|------|------|------|----------|
| v1.0 | 2026-04-12 | AI Assistant | 初始版本，基于Apifox文档和本地代码深度分析创建 |

---

## 附录C: 相关文档

- [M-Team API 完整指南](file:///home/incast/PT-Forward/docs/27-mteam-api-complete-guide.md) - M-Team(mTorrent) API参考
- [NexusPHP Hash算法深度分析](file:///home/incast/PT-Forward/docs/09-nexusphp-hash-algorithm.md) - pieces_hash与辅种机制
- [PT生态系统概览](file:///home/incast/PT-Forward/docs/02-pt-ecosystem-overview.md) - PT站点架构总览

---

> **文档结束** | 总计约 **800+ 行**，覆盖NexusPHP API的所有核心概念、端点、数据模型、认证机制、最佳实践及与M-Team的详细对比。
