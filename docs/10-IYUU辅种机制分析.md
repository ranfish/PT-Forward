# PT 生态系统源码深度解析 — 第五卷：IYUU 云端辅种机制深度分析

> **文档版本**: v1.0  
> **最后更新**: 2026-04-12  
> **分析目标**: [examples/iyuuplus-dev](file:///home/incast/PT-Forward/examples/iyuuplus-dev/) 项目的云端辅种实现与 ptdog(pieces_hash) 模式对比

---

## 目录

1. [核心结论速览](#1-核心结论速览)
2. [IYUU 项目架构全景](#2-iyuu-项目架构全景)
3. [云端辅种核心算法解析](#3-云端辅种核心算法解析)
4. [IYUU 公开 API 接口规范](#4-iyuu-公开-api-接口规范)
5. [ptdog vs IYUU 双模式深度对比](#5-ptdog-vs-iyuu-双模式深度对比)
6. [混合辅种方案设计](#6-混合辅种方案设计)
7. [PTNexus 平台集成建议](#7-ptnexus-平台集成建议)
8. [附录](#8-附录)

---

## 1. 核心结论速览

### 🔑 关键发现

| 维度 | ptdog (pieces_hash 模式) | IYUU (云端匹配模式) |
|------|--------------------------|---------------------|
| **匹配依据** | pieces_hash (SHA1 of pieces) | **info_hash** (BT协议标准) |
| **数据来源** | 站点本地 `/api/pieces-hash` | **IYUU 云端数据库** |
| **依赖站点API** | ✅ 必须开放 pieces-hash 接口 | ❌ **不依赖站点API** |
| **支持站点数** | ~21 个（需站点支持） | **85+ 个**（云端维护） |
| **匹配精度** | 🟢 内容级精确匹配 | 🟡 种子级匹配（可能误判） |
| **隐私性** | 🟢 本地计算，无数据外泄 | 🟡 需上传 info_hash 到云端 |
| **实时性** | 🟢 即时响应 (<5ms) | 🟡 网络延迟 (~100-500ms) |
| **可用性** | ⚠️ 站点可关闭接口 | 🟢 **始终可用** |

### 💡 核心洞察

```
┌─────────────────────────────────────────────────────────────────┐
│                    当站点关闭 pieces_hash 接口时                    │
│                                                                 │
│  ptdog 模式:    ❌ 完全失效，无法辅种                              │
│  IYUU 模式:     ✅ 正常工作，通过 info_hash + 云端匹配继续辅种       │
│                                                                 │
│  结论: IYUU 是 pieces_hash 关闭后的最佳替代方案                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. IYUU 项目架构全景

### 2.1 技术栈概览

| 组件 | 版本 | 用途 |
|------|------|------|
| **Workerman** | 5.1.3 | PHP 常驻内存框架 |
| **Webman** | 1.6.14 | HTTP 服务框架 |
| **PHP** | 8.3.24 | 运行时环境 |
| **MySQL** | 5.7+ | 数据存储 |
| **Illuminate Database** | 10.48 | ORM/Eloquent |
| **GuzzleHTTP** | latest | HTTP 客户端 |
| **Chrome-PHP** | 1.11 | 浏览器自动化（Cookie获取） |

### 2.2 项目目录结构

```
iyuuplus-dev/
├── app/                          # 主应用代码
│   ├── command/
│   │   └── ReseedCommand.php     # ⭐ 辅种命令入口
│   ├── model/
│   │   ├── Reseed.php            # ⭐ 辅种数据模型
│   │   ├── Site.php              # ⭐ 站点配置模型
│   │   └── Client.php            # 下载器配置模型
│   └── admin/services/reseed/
│       ├── ReseedServices.php    # ⭐ 核心辅种逻辑
│       └── ReseedDownloadServices.php  # ⭐ 种子下载与推送
├── composer/
│   ├── reseed-client/src/
│   │   └── Client.php            # ⭐ IYUU 云端 API 客户端
│   ├── site-manager/src/         # 站点管理器（100+ 站点适配）
│   │   ├── SiteManager.php       # 站点驱动管理
│   │   ├── Spider/Helper.php     # 种子下载助手
│   │   └── Driver/               # 各站点驱动（90+个）
│   └── bittorrent-client/src/    # 下载器客户端（qB/Transmission）
├── config/                       # 配置文件
├── db/migrations/                # 数据库迁移
└── public/                       # Web UI 静态资源
```

### 2.3 核心组件关系图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        IYUUPlus 架构图                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────┐    Cron Job     ┌──────────────────────────┐      │
│  │ Crontab 调度  │ ──────────────→ │  ReseedCommand           │      │
│  └──────────────┘                 │  (iyuu:reseed)            │      │
│                                   └──────────┬───────────────┘      │
│                                              │                       │
│                                              ▼                       │
│                                   ┌──────────────────────────┐      │
│                                   │  ReseedServices          │      │
│                                   │  ┌────────────────────┐  │      │
│                                   │  │ 1. 获取下载器做种   │  │      │
│                                   │  │    info_hash 列表   │  │      │
│                                   │  ├────────────────────┤  │      │
│                                   │  │ 2. 上传至 IYUU云    │  │      │
│                                   │  │    端进行匹配       │  │      │
│                                   │  ├────────────────────┤  │      │
│                                   │  │ 3. 解析返回结果     │  │      │
│                                   │  │    写入 cn_reseed   │  │      │
│                                   │  └────────────────────┘  │      │
│                                   └──────────┬───────────────┘      │
│                                              │                       │
│                           ┌──────────────────┼──────────────────┐    │
│                           ▼                  ▼                  ▼    │
│                   ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│                   │ qBittorrent  │  │Transmission  │  │ 其他下载器│ │
│                   │   Client     │  │   Client     │  │  Client  │ │
│                   └──────┬───────┘  └──────┬───────┘  └────┬─────┘ │
│                          │                │              │       │
│                          ▼                ▼              ▼       │
│                   ┌────────────────────────────────────────────┐  │
│                   │        ReseedDownloadServices             │  │
│                   │  ┌────────────────────────────────────┐   │  │
│                   │  │ 读取 cn_reseed 表待处理记录          │   │  │
│                   │  ↓                                     │   │  │
│                   │  │ SiteManager::download(torrent)      │   │  │
│                   │  ↓                                     │   │  │
│                   │  │ Downloader::addTorrent(.torrent)    │   │  │
│                   │  └────────────────────────────────────┘   │  │
│                   └────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. 云端辅种核心算法解析

### 3.1 辅种流程详解（基于源码）

#### **Step 1: 从下载器提取 info_hash**

[ReseedServices.php#L156-L175](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedServices.php#L156-L175)

```php
// 获取下载器当前做种的种子列表
$torrentList = $this->bittorrentClient->getTorrentList();

$hashDict = $torrentList['hashString'];   // 哈希目录字典 {info_hash => download_path}
$total = count($hashDict);
echo "{$this->clientModel->title} 下载器获取到做种哈希总数：{$total}" . PHP_EOL;
```

**关键点**:
- 只使用 **info_hash**，不计算 pieces_hash
- 同时保存 `download_path` 用于后续辅种定位

#### **Step 2: 分批次上传至 IYUU 云端**

[ReseedServices.php#L180-L210](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedServices.php#L180-L210)

```php
// 批次大小限制
private const int RESEED_GROUP_NUMBER = 200;

if (self::RESEED_GROUP_NUMBER < $total) {
    // 分批次辅种（超过200个hash时分批）
    $full = json_decode($torrentList['hash'], true);
    $chunkHash = array_chunk($full, self::RESEED_GROUP_NUMBER);
    
    foreach ($chunkHash as $info_hash) {
        sort($info_hash);  // 排序确保一致性
        $hash = json_encode($info_hash, JSON_UNESCAPED_UNICODE);
        
        // 调用 IYUU 云端 API
        $result = $reseedClient->reseed(
            $hash,           // JSON 格式的 info_hash 数组
            sha1($hash),     // hash 的 SHA1 校验值
            $sid_sha1,       // 站点集合哈希（汇报后获得）
            iyuu_version()   // 客户端版本号
        );
        
        $this->currentReseed($hashDict, $result);  // 处理结果
    }
}
```

#### **Step 3: IYUU 云端 API 调用细节**

[Client.php#L114-L134](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/reseed-client/src/Client.php#L114-L134)

```php
/**
 * 获取可辅种数据
 * 
 * @param string $hash      种子info_hash (JSON数组)
 * @param string $sha1      hash的SHA1校验值
 * @param string $sid_sha1  站点集合哈希（有效期7天）
 * @param string $version   版本号
 * @return array            可辅种结果
 */
public function reseed(string $hash, string $sha1, string $sid_sha1, string $version): array
{
    $data = [
        'hash' => $hash,
        'sha1' => $sha1,
        'sid_sha1' => $sid_sha1,
        'timestamp' => time(),
        'version' => $version,
    ];
    
    // POST 到 IYUU 云端服务器
    $curl = $this->getCurl()->post(
        $this->getBaseApi() . '/reseed/index/index', 
        $data
    );
    
    $response = $this->parseResponse($curl, '获取可辅种数据失败');
    return $response['data'];
}
```

#### **Step 4: 解析云端返回结果**

[ReseedServices.php#L220-L290](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedServices.php#L220-L290)

```php
protected function currentReseed(array $hashDict, array $result): void
{
    foreach ($result as $infohash => $reseed) {
        $downloadDir = $hashDict[$infohash];   // 原始做种目录
        
        // 单个 info_hash 可能匹配多个站点的多个种子
        foreach ($reseed['torrent'] as $id => $value) {
            $sid = $value['sid'];              // 目标站点 ID
            $torrent_id = $value['torrent_id'];// 目标种子 ID
            $reseed_infohash = $value['info_hash']; // 目标种子 info_hash
            
            // ... 过滤逻辑（禁用、未选择、已存在等）
            
            // 写入辅种队列
            Reseed::firstOrCreate($attributes, $values);
        }
    }
}
```

**返回数据结构示例**:
```json
{
  "abc123def456...": {
    "torrent": {
      "1": {
        "sid": 5,
        "torrent_id": 12345,
        "info_hash": "xyz789...",
        "group": 0
      },
      "2": {
        "sid": 12,
        "torrent_id": 67890,
        "info_hash": "uvw012...",
        "group": 1
      }
    }
  }
}
```

#### **Step 5: 下载种子并推送到下载器**

[ReseedDownloadServices.php#L60-L150](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedDownloadServices.php#L60-L150)

```php
public static function sendDownloader(Reseed $reseed, int $limitSleep = 0): bool
{
    // 1. 构造 Torrent 对象
    $torrent = new Torrent([
        'site' => $reseed->site,
        'id' => $reseed->reseed_id,
        'sid' => $reseed->sid,
        'torrent_id' => $reseed->torrent_id,
        'group_id' => $reseed->group_id,
    ]);
    
    // 2. 通过 SiteManager 下载种子文件
    $response = Helper::download($torrent);
    
    // 3. 构造下载器兼容格式
    $contractsTorrent = new TorrentContract(
        $response->payload,    // .torrent 二进制内容
        $response->metadata    // 元数据
    );
    $contractsTorrent->savePath = $reseed->directory;
    
    // 4. 推送到下载器
    $result = $bittorrentClients->addTorrent($contractsTorrent);
    
    // 5. 更新状态为成功
    $reseed->status = ReseedStatusEnums::Success->value;
    $reseed->save();
}
```

### 3.2 站点汇报机制（关键优化）

[ReseedServices.php#L305-L315](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedServices.php#L305-L315)

```php
protected function getSidSha1(\Iyuu\ReseedClient\Client $reseedClient): string
{
    // 获取用户选择要辅种的站点列表
    $sites = array_keys($this->crontabSites);
    
    // 查询这些站点在 IYUU 系统中的 sid
    $sid_list = Site::getEnabled()
        ->whereIn('site', $sites)
        ->pluck('sid')
        ->toArray();
    
    // 汇报给 IYUU 云端，获取站点集合哈希（有效期7天）
    return $reseedClient->reportExisting($sid_list);
}
```

**优化意义**:
- `sid_sha1` 缓存 **7天有效**
- 只有站点列表变化时才需要重新获取
- 减少每次请求的数据传输量

---

## 4. IYUU 公开 API 接口规范

### 4.1 接口总览

基于 [IYUU 官方文档](https://doc.iyuu.cn/reference/site_list) 和源码分析：

| 接口 | 方法 | 路径 | 用途 | 认证 |
|------|------|------|------|------|
| **站点列表** | GET | `/reseed/sites/index` | 获取支持的85+站点 | Token |
| **推荐站点** | GET | `/reseed/sites/recommend` | 获取推荐站点 | Token |
| **汇报站点** | POST | `/reseed/sites/reportExisting` | 汇报持有站点 | Token |
| **辅种查询** | POST | `/reseed/index/index` | **核心：批量查询可辅种** | Token |
| **单种查询** | GET | `/reseed/index/single` | 查询单个种子 | Token |
| **绑定站点** | POST | `/reseed/users/bind` | 绑定合作站点 | Token |
| **用户信息** | GET | `/reseed/users/profile` | 获取用户信息 | Token |
| **下载签名** | GET | `/reseed/sites/signature` | 获取下载签名 | Token |

### 4.2 认证机制

[AbstractCurl.php#L35-L45](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/reseed-client/src/AbstractCurl.php#L35-L45)

```php
protected function initCurl(): void
{
    $this->curl->setTimeout(8, 8)->setSslVerify();
    
    if ($this->token) {
        // 统一使用 Header Token 认证
        $this->curl->setHeader('token', $this->token);
    }
}
```

**Token 格式验证** ([functions.php#L85-L95](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/functions.php#L85-L95)):

```php
function is_iyuu_token(string $token): bool
{
    return strlen($token) < 60 
        && str_starts_with($token, 'IYUU') 
        && strpos($token, 'T') < 15;
}

// 示例: IYUU1T15d11d6a04492ad9
```

### 4.3 站点列表接口详情

**请求**:
```http
GET /reseed/sites/index HTTP/1.1
Host: 2025.iyuu.cn
Token: IYUU1T15d11d6a04492ad9
```

**成功响应**:
```json
{
  "code": 0,
  "data": {
    "count": 85,
    "sites": [
      {
        "id": 1,
        "site": "xxx",
        "nickname": "朋友",
        "base_url": "www.xxx.com",
        "download_page": "download.php?id={}&passkey={passkey}",
        "details_page": "details.php?id={}",
        "reseed_check": "passkey",
        "is_https": 2,
        "cookie_required": 0
      }
    ]
  },
  "msg": "ok"
}
```

**关键字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `site` | string | 站点唯一标识符（用于 SiteManager 驱动选择） |
| `nickname` | string | 站点显示名称 |
| `base_url` | string | 站点域名 |
| `download_page` | string | **下载链接模板**（支持 `{id}`, `{passkey}` 占位符） |
| `details_page` | string | 详情页模板 |
| `reseed_check` | string | 辅种检查方式 (`passkey` / `cookie`) |
| `is_https` | int | HTTPS 支持 (0=http, 1=https, 2=both) |
| `cookie_required` | int | 是否必须 Cookie 下载 |

### 4.4 核心：辅种查询接口

**请求参数**:
```php
$data = [
    'hash' => json_encode(['info_hash1', 'info_hash2', ...]),  // info_hash 列表
    'sha1' => sha1($hash_json),                                  // 校验值
    'sid_sha1' => 'abc123...',                                   // 站点集合哈希
    'timestamp' => time(),                                       // 时间戳
    'version' => 'x.x.x',                                       // 客户端版本
];
```

**成功响应结构**:
```json
{
  "code": 0,
  "data": {
    "原始_info_hash_1": {
      "torrent": {
        "0": {
          "sid": 5,
          "torrent_id": 12345,
          "info_hash": "目标站点的_info_hash",
          "group": 0
        },
        "1": {
          "sid": 12,
          "torrent_id": 67890,
          "info_hash": "另一个目标站点的_info_hash",
          "group": 1
        }
      }
    },
    "原始_info_hash_2": {
      "torrent": { ... }
    }
  }
}
```

### 4.5 错误处理机制

[Client.php#L55-L75](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/reseed-client/src/Client.php#L55-L75):

```php
protected function parseResponse(Curl $curl, string $defaultMessage): array
{
    // HTTP 层错误
    if (!$curl->isSuccess()) {
        throw new InternalServerErrorException(...);
    }

    $response = json_decode($curl->response, true);
    $code = $response['code'] ?? 403;
    
    // 业务层错误
    if ($code) {
        if (429 === $code) {
            // 限流异常（包含重试时间）
            throw new TooManyRequestsException($msg, 429);
        } else {
            throw new RuntimeException($msg, $code);
        }
    }

    return $response;
}
```

**限流处理** ([ReseedServices.php#L190-L205](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedServices.php#L190-L205)):

```php
catch (TooManyRequestsException $exception) {
    $limitReset = date('Y-m-d H:i:s', $exception->getXRateLimitReset());
    $error = implode(PHP_EOL, [
        $exception->getMessage(),
        "限流重置时间：{$limitReset}",
        "请在{$exception->getRetryAfter()}秒后重试" . PHP_EOL,
    ]);
    
    // 本地缓存限流状态，避免重复请求
    $tooManyRequestsCache->set($error, $exception->getRetryAfter());
}
```

---

## 5. ptdog vs IYUU 双模式深度对比

### 5.1 架构模式对比

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        ptdog (pieces_hash 模式)                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  下载器 ──→ 提取 .torrent 文件 ──→ 计算 pieces_hash                      │
│                                         │                               │
│                                         ▼                               │
│                              POST /api/pieces-hash                      │
│                              (目标 PT 站点服务器)                        │
│                                         │                               │
│                                         ▼                               │
│                              返回 {pieces_hash: torrent_id}             │
│                                         │                               │
│                                         ▼                               │
│                         GET download.php?id={torrent_id}                │
│                         (从目标站点下载 .torrent)                        │
│                                         │                               │
│                                         ▼                               │
│                              添加到下载器做种                             │
│                                                                         │
│  ⚠️ 依赖条件: 目标站点必须开放 /api/pieces-hash 接口                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                        IYUU (云端匹配模式)                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  下载器 ──→ 提取 info_hash 列表 ──→ POST /reseed/index/index           │
│                                      (IYUU 云端服务器 2025.iyuu.cn)      │
│                                             │                           │
│                                             ▼                           │
│                              返回 {info_hash: [{sid, torrent_id, ...}]}│
│                                             │                           │
│                                             ▼                           │
│                              SiteManager::download()                    │
│                              (通过 Cookie/Passkey 从目标站点下载)         │
│                                             │                           │
│                                             ▼                           │
│                              添加到下载器做种                             │
│                                                                         │
│  ✅ 依赖条件: 仅需 IYUU 云端服务 + 目标站点账号                            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.2 技术维度对比矩阵

| 维度 | ptdog (pieces_hash) | IYUU (云端) | 优势方 |
|------|---------------------|-------------|--------|
| **匹配算法** | SHA1(pieces) 精确匹配 | info_hash + 云端数据库 | 🟢 ptdog (更精确) |
| **数据来源** | 站点本地 Redis/MySQL | IYUU 云端集中式 | - |
| **网络依赖** | 仅依赖目标站点 | 依赖 IYUU 云端 + 目标站点 | 🟢 ptdog (少一跳) |
| **隐私安全** | 🟢 不泄露任何数据 | 🟡 需上传 info_hash | 🟢 ptdog |
| **部署复杂度** | 低（单二进制） | 中（PHP环境+MySQL） | 🟢 ptdog |
| **站点覆盖** | ~21 个（需API支持） | **85+ 个**（持续增加） | 🟢 **IYUU** |
| **抗关闭能力** | ❌ 站点关接口即废 | ✅ 始终可用 | 🟢 **IYUU** |
| **维护成本** | 用户自维护 | IYUU 团队维护 | 🟢 **IYUU** |
| **定制灵活性** | 高（开源可改） | 中（核心闭源） | 🟢 ptdog |
| **速度** | <5ms (Redis Pipeline) | 100-500ms (网络往返) | 🟢 ptdog |
| **误判率** | 0% (内容级匹配) | 低 (种子级匹配) | 🟢 ptdog |

### 5.3 适用场景分析

#### **ptdog 最适合的场景**:
```
✅ 对隐私要求极高的用户
✅ 站点稳定开放 pieces-hash API
✅ 需要毫秒级响应速度
✅ 追求 0% 误判率
✅ 有能力自行维护
```

#### **IYUU 最适合的场景**:
```
✅ 站点已关闭或限制 pieces-hash API
✅ 需要覆盖更多站点（85+ vs 21）
✅ 不想频繁更新适配代码
✅ 可以接受 info_hash 上传云端
✅ 需要 WebUI 管理界面
✅ 需要多下载器集群支持
```

### 5.4 关键差异：匹配精度

**ptdog 的 pieces_hash 匹配**:
```
原始种子: Movie.1080p.BluRay.x264
  └── pieces_hash = SHA1(所有分片的哈希拼接)
  
目标站点种子: Movie.1080p.BluRay.x264-[Group]
  └── pieces_hash = SHA1(所有分片的哈希拼接)  ← 相同！

✅ 结论: 100% 确认是同一资源（即使编码、文件名不同）
```

**IYUU 的 info_hash 匹配**:
```
原始种子: Movie.1080p.BluRay.x264
  └── info_hash = SHA1(info dictionary)
  
目标站点可能匹配:
  1. Movie.1080p.BluRay.x264-[Group]     ← 同一资源 ✅
  2. Movie.1080p.BluRay.x265              ← 不同编码 ⚠️
  3. Movie.2160p.BluRay.x264              ← 不同分辨率 ⚠️

⚠️ 结论: info_hash 相同 = .torrent 文件相同（高概率同源）
   但存在极小概率不同资源因制作方式相同而产生相同 info_hash
```

---

## 6. 混合辅种方案设计

### 6.1 方案概述

针对 **"有些站点会关闭 pieces_hash 接口"** 这一问题，设计混合方案：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     PTNexus 混合辅种引擎                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Layer 1: 优先使用 pieces_hash (快速、精确、隐私)                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  for each site in enabled_sites:                                 │   │
│  │      if site.has_pieces_hash_api():                              │   │
│  │          result += ptdog_mode.query(pieces_hashes)  ✅ 优先      │   │
│  │                                                                 │   │
│  │  未匹配的 hashes → 进入 Layer 2                                 │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  Layer 2: 兜底使用 IYUU 云端 (广覆盖、抗关闭)                            │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  unmatched_hashes = all_hashes - matched_hashes                 │   │
│  │  if !empty(unmatched_hashes):                                   │   │
│  │      result += iyuu_cloud.query(unmatched_hashes)  ✅ 兜底      │   │
│  │                                                                 │   │
│  │ 仍未匹配的 → 记录日志，人工干预                                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  Layer 3: 可选 Graft 式内容指纹 (终极兜底)                               │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  if config.enable_graft_fallback && still_unmatched:            │   │
│  │      result += graft_fingerprint.match(files)   ✅ 终极兜底     │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 6.2 伪代码实现

```python
class HybridReseedEngine:
    def __init__(self, config):
        self.ptdog_client = PtdogClient(config.ptdog)
        self.iyuu_client = IyuuClient(config.iyuu.token)
        self.graft_engine = GraftEngine(config.graft)
        self.site_configs = load_site_configs()
    
    def query(self, info_hashes: List[str], pieces_hashes: Dict[str, str]) -> ReseedResult:
        result = ReseedResult()
        unmatched_info = set(info_hashes)
        
        # ===== Layer 1: pieces_hash 优先 =====
        for site_name, site_cfg in self.site_configs.items():
            if site_cfg.get('pieces_hash_api'):
                try:
                    # 筛选该站点相关的 pieces_hash
                    site_pieces = [ph for ph in pieces_hashes.values()]
                    
                    response = self.ptdog_client.query_pieces_hash(
                        url=site_cfg['pieces_hash_url'],
                        passkey=site_cfg['passkey'],
                        pieces_hashes=site_pieces[:100]  # 批量限制
                    )
                    
                    for ph, torrent_id in response.items():
                        original_hash = find_info_hash_by_pieces(ph)
                        if original_hash in unmatched_info:
                            result.add_match(original_hash, {
                                'mode': 'pieces_hash',
                                'site': site_name,
                                'torrent_id': torrent_id,
                                'confidence': 1.0  # 100% 确定
                            })
                            unmatched_info.remove(original_hash)
                            
                except PiecesHashApiDisabled:
                    logger.warning(f"{site_name} pieces_hash API 已关闭")
                    site_cfg['pieces_hash_api'] = False  # 标记不可用
        
        # ===== Layer 2: IYUU 云端兜底 =====
        if unmatched_info and self.iyuu_client.is_configured():
            try:
                iyuu_result = self.iyuu_client.reseed(
                    hashes=list(unmatched_info),
                    sid_sha1=self._get_cached_sid_sha1()
                )
                
                for info_hash, matches in iyuu_result.items():
                    if info_hash in unmatched_info:
                        for match in matches['torrent']:
                            result.add_match(info_hash, {
                                'mode': 'iyuu_cloud',
                                'site': match['site_name'],
                                'torrent_id': match['torrent_id'],
                                'sid': match['sid'],
                                'confidence': 0.95  # 高置信度
                            })
                        unmatched_info.remove(info_hash)
                        
            except Exception as e:
                logger.error(f"IYUU 云端查询失败: {e}")
        
        # ===== Layer 3: Graft 内容指纹 (可选) =====
        if unmatched_info and self.graft_engine.enabled:
            for info_hash in list(unmatched_info):
                try:
                    graft_match = self.graft_engine.match_by_content(info_hash)
                    if graft_match:
                        result.add_match(info_hash, {
                            'mode': 'graft_fingerprint',
                            'site': graft_match.site,
                            'torrent_id': graft_match.torrent_id,
                            'confidence': 0.85  # 中等置信度
                        })
                        unmatched_info.remove(info_hash)
                except Exception as e:
                    logger.debug(f"Graft 匹配失败: {e}")
        
        # 记录未匹配项
        if unmatched_info:
            logger.warning(f"仍有 {len(unmatched_info)} 个种子无法匹配")
            result.unmatched = list(unmatched_info)
        
        return result
    
    def _get_cached_sid_sha1(self) -> str:
        """缓存 sid_sha1，7天内有效"""
        cached = cache.get('iyuu:sid_sha1')
        if cached:
            return cached
        
        sid_list = [s['sid'] for s in self.site_configs.values() if s.get('sid')]
        new_sha1 = self.iyuu_client.report_existing(sid_list)
        cache.set('iyuu:sid_sha1', new_sha1, timeout=7*24*3600)
        return new_sha1
```

### 6.3 配置示例

```yaml
# config/hybrid-reseed.yaml

hybrid_reseed:
  # 全局开关
  enabled: true
  
  # Layer 1: ptdog 配置
  ptdog:
    enabled: true
    batch_size: 100
    timeout_ms: 5000
    retry_count: 3
  
  # Layer 2: IYUU 云端配置
  iyuu:
    enabled: true
    token: "${IYUU_TOKEN}"  # 从环境变量读取
    base_url: "http://2025.iyuu.cn"
    vip: false
    rate_limit:
      max_requests_per_minute: 30
      cooldown_seconds: 60
  
  # Layer 3: Graft 配置（可选）
  graft:
    enabled: false  # 默认关闭，性能开销大
    max_concurrent_downloads: 5
    similarity_threshold: 0.95
  
  # 站点特定配置
  sites:
    hdtime:
      sid: 1
      pieces_hash_api: true
      pieces_hash_url: "https://hdtime.org/api/pieces-hash"
      passkey: "${HDTIME_PASSKEY}"
      
    ptcafe:
      sid: 2
      pieces_hash_api: true
      pieces_hash_url: "https://ptcafe.club/api/pieces-hash"
      passkey: "${PTCAFE_PASSKEY}"
      
    some_closed_site:
      sid: 99
      pieces_hash_api: false  # 已关闭
      cookie_required: true
      cookie: "${SOME_SITE_COOKIE}"
```

### 6.4 性能优化策略

#### **并行查询**:
```python
import asyncio
from concurrent.futures import ThreadPoolExecutor

async def parallel_query(hashes, sites):
    """并行执行多站点查询"""
    loop = asyncio.get_event_loop()
    
    with ThreadPoolExecutor(max_workers=len(sites)) as executor:
        tasks = []
        for site in sites:
            if site.pieces_hash_api:
                tasks.append(loop.run_in_executor(
                    executor, 
                    query_ptdog_site, 
                    site, hashes
                ))
        
        # 同时发起 IYUU 云端查询
        tasks.append(loop.run_in_executor(
            executor,
            query_iyuu_cloud,
            hashes
        ))
        
        results = await asyncio.gather(*tasks, return_exceptions=True)
        return merge_results(results)
```

#### **智能缓存**:
```python
class ReseedCache:
    """多层缓存策略"""
    
    def __init__(self, redis_client):
        self.redis = redis_client
        self.local_cache = LRUCache(maxsize=10000)
    
    def get(self, pieces_hash: str) -> Optional[int]:
        # L1: 内存缓存 (最快)
        if result := self.local_cache.get(pieces_hash):
            return result
        
        # L2: Redis 缓存 (快)
        if result := self.redis.hget('reseed:cache', pieces_hash):
            self.local_cache[pieces_hash] = result
            return result
        
        return None
    
    def set(self, pieces_hash: str, torrent_id: int, ttl: int = 3600):
        self.local_cache[pieces_hash] = torrent_id
        self.redis.hset('reseed:cache', pieces_hash, torrent_id)
        self.redis.expire('reseed:cache', ttl)
```

---

## 7. PTNexus 平台集成建议

### 7.1 推荐架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    PTNexus 辅种服务架构                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    NexusPHP 后端 (现有)                          │   │
│  │  ┌─────────────────┐  ┌─────────────────┐                      │   │
│  │  │ /api/pieces-hash │  │ /api/info-hash  │ ← 新增              │   │
│  │  │ (已有，保持不变)  │  │ (建议新增)      │                      │   │
│  │  └────────┬────────┘  └────────┬────────┘                      │   │
│  └───────────┼────────────────────┼───────────────────────────────┘   │
│              │                    │                                   │
│              ▼                    ▼                                   │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                 PTNexus 辅种中间层 (新增)                        │   │
│  │                                                                  │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐     │   │
│  │  │ PtdogAdapter │  │ IYUUAdapter  │  │ GraftAdapter       │     │   │
│  │  │ (pieces_hash)│  │ (云端匹配)   │  │ (内容指纹)         │     │   │
│  │  └──────┬───────┘  └──────┬───────┘  └────────┬───────────┘     │   │
│  │         │                 │                   │                 │   │
│  │         └─────────────────┼───────────────────┘                 │   │
│  │                           ▼                                     │   │
│  │              ┌───────────────────────┐                         │   │
│  │              │  HybridReseedEngine   │                         │   │
│  │              │  (统一调度 + 结果合并) │                         │   │
│  │              └───────────┬───────────┘                         │   │
│  └──────────────────────────┼────────────────────────────────────┘   │
│                             │                                        │
│                             ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    下载器接口层                                   │   │
│  │  ┌──────────────┐  ┌──────────────┐                            │   │
│  │  │ qBittorrent  │  │ Transmission │                            │   │
│  │  │   Adapter    │  │   Adapter    │                            │   │
│  │  └──────────────┘  └──────────────┘                            │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2 API 设计建议

#### **统一辅种查询接口** (新增到 NexusPHP):

```php
// routes/api.php (建议新增)

Route::group(['middleware' => ['auth.nexus:passkey']], function () {
    
    // 现有接口（保持不变）
    Route::post("pieces-hash", [TorrentController::class, "queryByPiecesHash"]);
    
    // ===== 新增：统一辅种入口 =====
    Route::post("hybrid-reseed/query", [HybridReseedController::class, "query"])
        ->name("torrent.hybrid_reseed.query");
    
    // 新增：info_hash 反查
    Route::post("info-hash", [TorrentController::class, "queryByInfoHash"])
        ->name("torrent.info_hash.query");
});

// HybridReseedController.php

class HybridReseedController extends Controller
{
    public function query(Request $request)
    {
        $request->validate([
            'info_hashes' => 'required|array|max:200',
            'pieces_hashes' => 'nullable|array|max:100',
            'modes' => 'array',  // ['pieces_hash', 'iyuu', 'graft']
        ]);
        
        $engine = app(HybridReseedEngine::class);
        
        $result = $engine->query(
            info_hashes: $request->info_hashes,
            pieces_hashes: $request->pieces_hashes ?? [],
            modes: $request->modes ?? ['pieces_hash', 'iyuu']
        );
        
        return $this->success($result);
    }
}
```

#### **响应格式标准化**:

```json
{
  "code": 0,
  "data": {
    "summary": {
      "total_input": 150,
      "total_matched": 142,
      "by_mode": {
        "pieces_hash": 120,
        "iyuu_cloud": 20,
        "graft_fingerprint": 2
      },
      "unmatched": 8
    },
    "matches": {
      "original_info_hash_1": [
        {
          "mode": "pieces_hash",
          "site": "hdtime",
          "torrent_id": 12345,
          "target_info_hash": "xxx",
          "confidence": 1.0
        }
      ],
      "original_info_hash_2": [
        {
          "mode": "iyuu_cloud",
          "site": "ptcafe",
          "torrent_id": 67890,
          "target_info_hash": "yyy",
          "confidence": 0.95,
          "sid": 2
        }
      ]
    },
    "unmatched_hashes": ["hash_a", "hash_b", ...]
  },
  "msg": "ok"
}
```

### 7.3 IYUU Token 管理

```php
// app/Services/IyuuTokenService.php

class IyuuTokenService
{
    /**
     * 验证并缓存 IYUU Token
     */
    public static function validateAndCache(string $token): bool
    {
        if (!self::isValidFormat($token)) {
            throw new InvalidArgumentException('IYUU Token 格式无效');
        }
        
        // 测试 Token 是否有效
        $client = new \Iyuu\ReseedClient\Client($token);
        try {
            $sites = $client->sites();
            
            // 缓存站点列表（减少重复请求）
            Cache::put('iyuu:sites', $sites, now()->addDays(7));
            Cache::put('iyuu:token', $token, now()->addDays(30));
            
            return true;
        } catch (\Exception $e) {
            Log::warning("IYUU Token 验证失败: {$e->getMessage()}");
            return false;
        }
    }
    
    public static function isValidFormat(string $token): bool
    {
        return strlen($token) < 60 
            && str_starts_with($token, 'IYUU') 
            && strpos($token, 'T') < 15;
    }
    
    public static function getCachedSites(): ?array
    {
        return Cache::get('iyuu:sites');
    }
}
```

### 7.4 监控与告警

```php
// 监控各模式的成功率

class ReseedMetrics
{
    public function recordQuery(string $mode, bool $success, float $durationMs): void
    {
        Metrics::counter('reseed_queries_total', [
            'mode' => $mode,
            'success' => $success
        ])->increment();
        
        Metrics::histogram('reseed_query_duration_ms', [
            'mode' => $mode
        ])->observe($durationMs);
    }
    
    public function getDashboardData(): array
    {
        return [
            'pieces_hash_success_rate' => $this->calculateSuccessRate('pieces_hash'),
            'iyuu_success_rate' => $this->calculateSuccessRate('iyuu'),
            'avg_latency' => [
                'pieces_hash' => $this->getAvgDuration('pieces_hash'),
                'iyuu' => $this->getAvgDuration('iyuu'),
            ],
            'sites_with_disabled_api' => $this->getDisabledApiSites(),
        ];
    }
}
```

---

## 8. 附录

### 8.1 IYUU 项目关键源码索引

| 文件路径 | 功能 | 核心类/方法 |
|----------|------|-------------|
| [app/command/ReseedCommand.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/command/ReseedCommand.php) | 辅种命令入口 | `ReseedCommand::execute()` |
| [app/model/Reseed.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/model/Reseed.php) | 辅种数据模型 | `Reseed` Eloquent Model |
| [app/model/Site.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/model/Site.php) | 站点配置模型 | `Site` Eloquent Model |
| [app/admin/services/reseed/ReseedServices.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedServices.php) | **核心辅种逻辑** | `ReseedServices::run()` |
| [app/admin/services/reseed/ReseedDownloadServices.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedDownloadServices.php) | 种子下载推送 | `ReseedDownloadServices::sendDownloader()` |
| [composer/reseed-client/src/Client.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/reseed-client/src/Client.php) | **IYUU API 客户端** | `Client::reseed()`, `Client::sites()` |
| [composer/reseed-client/src/AbstractCurl.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/reseed-client/src/AbstractCurl.php) | HTTP 基础类 | `AbstractCurl` |
| [composer/site-manager/src/SiteManager.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/site-manager/src/SiteManager.php) | 站点管理器 | `SiteManager::select()` |
| [composer/site-manager/src/Spider/Helper.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/composer/site-manager/src/Spider/Helper.php) | 种子下载助手 | `Helper::download()` |
| [app/functions.php](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/functions.php) | 工具函数 | `iyuu_token()`, `iyuu_reseed_client()` |

### 8.2 IYUU API 快速参考

| 接口 | 方法 | 路径 | 参数 | 返回 |
|------|------|------|------|------|
| 站点列表 | GET | `/reseed/sites/index` | Token | 85+ 站点列表 |
| 汇报站点 | POST | `/reseed/sites/reportExisting` | sid_list[] | sid_sha1 (7天有效) |
| **批量辅种** | POST | `/reseed/index/index` | hash, sha1, sid_sha1, version | 匹配结果 |
| 单种查询 | GET | `/reseed/index/single` | sid, torrent_id, sid_sha1 | 单种结果 |
| 下载签名 | GET | `/reseed/sites/signature` | user_id, sid | 下载签名 |
| 用户信息 | GET | `/reseed/users/profile` | Token | VIP状态等 |

### 8.3 支持的站点数量对比

| 辅种工具 | 支持站点数 | 数据来源 | 维护方式 |
|----------|-----------|----------|----------|
| **ptdog** | ~21 个 | 用户配置文件 | 社区维护 |
| **IYUU** | **85+ 个** | 云端数据库 | **IYUU团队维护** |
| **cross-seed** | ~50+ 个 | Torznab/RSS | 半自动 |
| **Graft** | ~30+ 个 | 手动配置 | 用户自维护 |

### 8.4 三种辅种引擎算法总结

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        辅种算法决策树                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  开始                                                                    │
│    │                                                                     │
│    ▼                                                                     │
│  站点是否开放 /api/pieces-hash ？                                         │
│    │                                                                     │
│    ├── Yes ──→ 使用 ptdog 模式 (pieces_hash 精确匹配)                     │
│    │              ✅ 0% 误判率                                           │
│    │              ✅ <5ms 响应                                           │
│    │              ✅ 隐私友好                                            │
│    │                                                                     │
│    └── No ───→ 是否有 IYUU Token？                                       │
│                 │                                                        │
│                 ├── Yes ──→ 使用 IYUU 云端模式 (info_hash 匹配)           │
│                 │             ✅ 85+ 站点覆盖                             │
│                 │             ✅ 抗接口关闭                               │
│                 │             ⚠️ 需上传 info_hash                        │
│                 │                                                        │
│                 └── No ───→ 使用 Graft 模式 (内容指纹匹配)                 │
│                               ✅ 无需任何API                              │
│                               ⚠️ 性能开销大                              │
│                               ⚠️ 需下载 .torrent 解析                    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.5 最佳实践建议

#### **对于 PTNexus 平台开发者**:

1. **默认启用混合模式**:
   ```php
   config(['hybrid_reseed.default_modes' => ['pieces_hash', 'iyuu']]);
   ```

2. **自动检测 API 可用性**:
   ```php
   // 定期探测 pieces_hash API 状态
   Schedule::call(function () {
       foreach (Site::where('pieces_hash_api', true)->get() as $site) {
           if (!ping($site->pieces_hash_url)) {
               $site->update(['pieces_hash_api' => false]);
               notify_admin("{$site->name} pieces_hash API 已关闭");
           }
       })->hourly();
   ```

3. **提供降级策略配置**:
   ```yaml
   hybrid_reseed:
     fallback_strategy: "auto"  # auto | manual | disable
     
     auto_fallback:
       - condition: "pieces_hash_error_rate > 20%"
         action: "switch_to_iyuu"
       - condition: "iyuu_unavailable > 5min"
         action: "switch_to_graft"
   ```

4. **监控面板**:
   - 各模式成功率趋势图
   - API 响应时间分布
   - 未匹配种子统计
   - 站点 API 状态看板

---

**文档版本**: v1.0  
**最后更新**: 2026-04-12  
**分析项目**: examples/iyuuplus-dev (PHP/Webman/IYUU云端)  
**相关文档**:
- 第一卷: [pt-ecosystem-analysis.md](file:///home/incast/PT-Forward/docs/pt-ecosystem-analysis.md) (全景分析)
- 第二卷: [pt-source-deep-analysis.md](file:///home/incast/PT-Forward/docs/pt-source-deep-analysis.md) (源码深度解析)
- 第三卷: [pt-screenshot-deep-analysis.md](file:///home/incast/PT-Forward/docs/pt-screenshot-deep-analysis.md) (截图方案分析)
- 第四卷: [pt-nexusphp-hash-analysis.md](file:///home/incast/PT-Forward/docs/pt-nexusphp-hash-analysis.md) (NexusPHP Hash 接口)
- 第五卷: **本文档** (IYUU 云端辅种机制)
