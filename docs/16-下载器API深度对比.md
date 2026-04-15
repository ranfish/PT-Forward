# qBittorrent vs Transmission API 深度对比分析

本文档深入分析两个主流 BitTorrent 下载工具的 API 设计，为 PT-Forward 项目提供参考。

## 1. API 架构对比

### 1.1 qBittorrent WebUI API

**架构模式**: RESTful API + Action-based Controller

```
基础URL: http://host:8080/api/v2/
认证方式: Cookie-based Session (SID)
```

**核心设计特点**:

1. **Controller 分层架构**
   - `APIController` - 基类，提供 `run(action, params, data)` 方法
   - 使用 Qt 元对象系统 (`QMetaObject::invokeMethod`) 动态调用 Action
   - 每个 Controller 对应一个资源域

2. **主要 Controller 划分**:

| Controller | 职责 | 端点前缀 |
|------------|------|----------|
| `TorrentsController` | 种子管理 | `/api/v2/torrents/` |
| `TransferController` | 全局传输控制 | `/api/v2/transfer/` |
| `AppController` | 应用配置 | `/api/v2/app/` |
| `SyncController` | 增量同步 | `/api/v2/sync/` |
| `AuthController` | 认证管理 | `/api/v2/auth/` |
| `LogController` | 日志查询 | `/api/v2/log/` |
| `RSSController` | RSS 订阅 | `/api/v2/rss/` |
| `SearchController` | 搜索功能 | `/api/v2/search/` |

3. **请求处理流程**:
```cpp
// apicontroller.cpp
APIResult APIController::run(const QString &action, const StringMap &params, const DataMap &data)
{
    m_result.clear();
    m_params = params;
    m_data = data;

    const QByteArray methodName = action.toLatin1() + "Action";
    if (!QMetaObject::invokeMethod(this, methodName.constData()))
        throw APIError(APIErrorType::NotFound);

    return m_result;
}
```

### 1.2 Transmission RPC

**架构模式**: JSON-RPC 2.0 风格

```
基础URL: http://host:9091/transmission/rpc
认证方式: HTTP Basic Auth + CSRF Token (X-Transmission-Session-Id)
```

**核心设计特点**:

1. **单一端点设计**
   - 所有请求发送到同一个 URL
   - 通过 `method` 字段区分操作类型
   - 通过 `arguments` 传递参数

2. **请求格式**:
```json
{
    "method": "torrent-start",
    "arguments": {
        "ids": [1, 2, 3]
    },
    "tag": 12345
}
```

3. **响应格式**:
```json
{
    "result": "success",
    "arguments": {
        "torrents": [...]
    },
    "tag": 12345
}
```

4. **CSRF 保护机制**:
   - 首次请求返回 HTTP 409
   - 响应头包含正确的 `X-Transmission-Session-Id`
   - 客户端需携带此 ID 重新发送请求

## 2. 功能对比

### 2.1 种子操作 API

#### qBittorrent 种子 API

| Action | 端点 | 功能 |
|--------|------|------|
| `info` | `/api/v2/torrents/info` | 获取种子列表（支持过滤、排序、分页） |
| `properties` | `/api/v2/torrents/properties` | 获取种子属性 |
| `trackers` | `/api/v2/torrents/trackers` | 获取 Tracker 列表 |
| `webseeds` | `/api/v2/torrents/webseeds` | 获取 WebSeed 列表 |
| `files` | `/api/v2/torrents/files` | 获取文件列表 |
| `pieceStates` | `/api/v2/torrents/pieceStates` | 获取分块状态 |
| `pieceHashes` | `/api/v2/torrents/pieceHashes` | 获取分块哈希 |
| `add` | `/api/v2/torrents/add` | 添加种子 |
| `delete` | `/api/v2/torrents/delete` | 删除种子 |
| `start` | `/api/v2/torrents/start` | 开始下载 |
| `stop` | `/api/v2/torrents/stop` | 暂停下载 |
| `recheck` | `/api/v2/torrents/recheck` | 重新校验 |
| `reannounce` | `/api/v2/torrents/reannounce` | 重新宣告 |
| `setLocation` | `/api/v2/torrents/setLocation` | 设置保存位置 |
| `rename` | `/api/v2/torrents/rename` | 重命名 |
| `setCategory` | `/api/v2/torrents/setCategory` | 设置分类 |
| `addTags` | `/api/v2/torrents/addTags` | 添加标签 |
| `removeTags` | `/api/v2/torrents/removeTags` | 移除标签 |
| `setShareLimits` | `/api/v2/torrents/setShareLimits` | 设置分享限制 |

**种子信息字段** (infoAction 返回):
```json
{
    "hash": "torrent_id",
    "name": "Torrent Name",
    "size": 1073741824,
    "progress": 0.75,
    "dlspeed": 1048576,
    "upspeed": 524288,
    "priority": 1,
    "num_seeds": 10,
    "num_complete": 100,
    "num_leechs": 5,
    "num_incomplete": 50,
    "ratio": 1.5,
    "eta": 3600,
    "state": "downloading",
    "seq_dl": false,
    "f_l_piece_prio": false,
    "force_start": false,
    "category": "movies",
    "tags": ["hd", "bluray"],
    "super_seeding": false,
    "save_path": "/downloads/",
    "content_path": "/downloads/torrent/",
    "added_on": 1640000000,
    "completion_on": 1640086400,
    "tracker": "https://tracker.example.com",
    "dl_limit": -1,
    "up_limit": -1
}
```

#### Transmission 种子 API

| Method | 功能 |
|--------|------|
| `torrent-get` | 获取种子信息（字段选择） |
| `torrent-set` | 修改种子属性 |
| `torrent-add` | 添加种子 |
| `torrent-remove` | 删除种子 |
| `torrent-start` | 开始下载 |
| `torrent-start-now` | 立即开始（跳过队列） |
| `torrent-stop` | 停止下载 |
| `torrent-verify` | 校验数据 |
| `torrent-reannounce` | 重新宣告 |
| `torrent-set-location` | 设置位置 |
| `torrent-rename-path` | 重命名路径 |
| `queue-move-top` | 移至队首 |
| `queue-move-up` | 上移一位 |
| `queue-move-down` | 下移一位 |
| `queue-move-bottom` | 移至队尾 |

**种子 ID 选择器**:
```json
// 支持多种 ID 格式
{
    "ids": 1,                    // 单个 ID
    "ids": [1, 2, 3],           // ID 列表
    "ids": ["hash1", "hash2"],  // 哈希列表
    "ids": "recently-active"    // 最近活跃的种子
}
```

**torrent-get 字段选择**:
```json
{
    "method": "torrent-get",
    "arguments": {
        "fields": ["id", "name", "totalSize", "progress", "rateDownload", "rateUpload"],
        "ids": [1, 2]
    }
}
```

### 2.2 全局传输控制

#### qBittorrent TransferController

| Action | 端点 | 功能 |
|--------|------|------|
| `info` | `/api/v2/transfer/info` | 获取传输统计 |
| `speedLimitsMode` | `/api/v2/transfer/speedLimitsMode` | 获取限速模式 |
| `toggleSpeedLimitsMode` | `/api/v2/transfer/toggleSpeedLimitsMode` | 切换限速模式 |
| `uploadLimit` | `/api/v2/transfer/uploadLimit` | 获取上传限速 |
| `downloadLimit` | `/api/v2/transfer/downloadLimit` | 获取下载限速 |
| `setUploadLimit` | `/api/v2/transfer/setUploadLimit` | 设置上传限速 |
| `setDownloadLimit` | `/api/v2/transfer/setDownloadLimit` | 设置下载限速 |
| `banPeers` | `/api/v2/transfer/banPeers` | 封禁 Peer |

**传输信息响应**:
```json
{
    "dl_info_speed": 1048576,
    "dl_info_data": 10737418240,
    "up_info_speed": 524288,
    "up_info_data": 5368709120,
    "dl_rate_limit": 0,
    "up_rate_limit": 0,
    "dht_nodes": 1000,
    "connection_status": "connected"
}
```

#### Transmission Session API

| Method | 功能 |
|--------|------|
| `session-get` | 获取会话设置 |
| `session-set` | 修改会话设置 |
| `session-stats` | 获取会话统计 |
| `session-close` | 关闭会话 |
| `blocklist-update` | 更新黑名单 |
| `port-test` | 测试端口 |
| `free-space` | 查询可用空间 |

### 2.3 增量同步机制

#### qBittorrent SyncController

**maindataAction** - 主数据同步:
```
GET /api/v2/sync/maindata?rid=0
```

响应包含:
- `rid`: 响应 ID（用于下次请求）
- `full_update`: 是否全量更新
- `torrents`: 变更的种子数据
- `categories`: 变更的分类
- `tags`: 变更的标签
- `trackers`: 变更的 Tracker
- `server_state`: 服务器状态
- `removed`: 已删除的项目

**torrentPeersAction** - Peer 同步:
```
GET /api/v2/sync/torrentPeers?hash=xxx&rid=0
```

**内部实现**:
```cpp
// synccontroller.h
struct MaindataSyncBuf
{
    QHash<QString, QVariantMap> categories;
    QVariantList tags;
    QHash<QString, QVariantMap> torrents;
    QHash<QString, QStringList> trackers;
    QVariantMap serverState;

    QStringList removedCategories;
    QStringList removedTags;
    QStringList removedTorrents;
    QStringList removedTrackers;
};
```

#### Transmission 无原生增量同步

Transmission 不提供专门的增量同步 API，客户端需要:
1. 使用 `torrent-get` 获取完整数据
2. 使用 `ids: "recently-active"` 过滤最近活跃的种子
3. 客户端自行实现差异比较

### 2.4 认证与安全

#### qBittorrent 认证流程

```
1. POST /api/v2/auth/login
   参数: username, password
   响应: SID cookie

2. 后续请求携带 Cookie: SID=xxx

3. POST /api/v2/auth/logout
   登出
```

**安全特性**:
- 登录失败计数与临时封禁
- 支持 Referer 检查
- 支持 Host 白名单

#### Transmission 认证流程

```
1. 首次请求返回 409，获取 X-Transmission-Session-Id

2. 后续请求携带:
   - Header: X-Transmission-Session-Id: xxx
   - Authorization: Basic base64(user:pass) (可选)

3. 支持 Host 白名单防 DNS 重绑定攻击
```

## 3. 设计模式对比

### 3.1 API 设计风格

| 特性 | qBittorrent | Transmission |
|------|-------------|--------------|
| 风格 | RESTful + Action | JSON-RPC |
| 端点 | 多端点，按资源划分 | 单一端点 |
| 参数传递 | URL 参数 + POST 数据 | JSON arguments |
| 响应格式 | 直接 JSON 数据 | 统一 result + arguments |
| 错误处理 | HTTP 状态码 + 异常 | result 字段 |

### 3.2 数据获取策略

**qBittorrent**: 预定义响应结构
```cpp
// 每个接口返回固定的字段集
void TorrentsController::propertiesAction()
{
    const QJsonObject ret {
        {KEY_TORRENT_NAME, torrent->name()},
        {KEY_TORRENT_SIZE, torrent->totalSize()},
        // ... 固定字段
    };
    setResult(ret);
}
```

**Transmission**: 字段选择器模式
```cpp
// 客户端指定需要的字段
void initField(tr_torrent const* tor, tr_stat const* st, 
               tr_variant* initme, tr_quark key)
{
    switch (key) {
    case TR_KEY_name:
        tr_variantInitStrView(initme, tr_torrentName(tor));
        break;
    // ... 按需初始化
    }
}
```

### 3.3 批量操作

**qBittorrent**: 使用管道符分隔的字符串
```
POST /api/v2/torrents/start
hashes=hash1|hash2|hash3
```

**Transmission**: 使用 JSON 数组
```json
{
    "method": "torrent-start",
    "arguments": {
        "ids": [1, 2, 3]
    }
}
```

### 3.4 过滤与排序

**qBittorrent**: 服务端过滤
```
GET /api/v2/torrents/info?filter=downloading&category=movies&sort=name&reverse=true&limit=10&offset=0
```

支持的过滤器:
- `filter`: all, downloading, seeding, completed, stopped, running, active, inactive, stalled
- `category`: 分类过滤
- `tag`: 标签过滤
- `private`: 私有种子过滤
- `hashes`: 哈希过滤

**Transmission**: 客户端过滤
- 服务端返回所有数据
- 客户端自行实现过滤逻辑

## 4. 实现细节对比

### 4.1 qBittorrent 核心实现

**Controller 基类**:
```cpp
class APIController : public ApplicationComponent<QObject>
{
public:
    APIResult run(const QString &action, const StringMap &params, const DataMap &data = {});

protected:
    const StringMap &params() const;
    const DataMap &data() const;
    void requireParams(const QList<QString> &requiredParams) const;
    void setResult(const QString &result);
    void setResult(const QJsonArray &result);
    void setResult(const QJsonObject &result);
};
```

**参数验证**:
```cpp
void APIController::requireParams(const QList<QString> &requiredParams) const
{
    const bool hasAllRequiredParams = std::all_of(
        requiredParams.cbegin(), requiredParams.cend(),
        [this](const QString &requiredParam) {
            return params().contains(requiredParam);
        });

    if (!hasAllRequiredParams)
        throw APIError(APIErrorType::BadParams);
}
```

### 4.2 Transmission 核心实现

**RPC 入口**:
```cpp
void tr_rpc_request_exec_json(
    tr_session* session,
    tr_variant const* request,
    tr_rpc_response_func callback,
    void* callback_user_data);
```

**种子获取**:
```cpp
auto getTorrents(tr_session* session, tr_variant* args)
{
    auto torrents = std::vector<tr_torrent*>{};

    if (tr_variant* ids = nullptr; tr_variantDictFindList(args, TR_KEY_ids, &ids)) {
        // 从 ID 列表获取
    } else if (tr_variantDictFindStrView(args, TR_KEY_ids, &sv)) {
        if (sv == "recently-active"sv) {
            // 获取最近活跃的种子
            time_t const cutoff = tr_time() - RecentlyActiveSeconds;
            // ...
        }
    } else {
        // 获取所有种子
    }

    return torrents;
}
```

## 5. 优缺点分析

### 5.1 qBittorrent API

**优点**:
1. RESTful 风格，符合 Web 开发习惯
2. 服务端过滤、排序、分页，减少数据传输
3. 增量同步机制，适合实时 UI 更新
4. 丰富的功能端点（RSS、搜索等）
5. 文档完善，社区活跃

**缺点**:
1. 端点较多，学习成本较高
2. 参数格式不统一（字符串 vs JSON）
3. 批量操作使用管道符分隔，不够优雅
4. 缺少版本化 API（v2 是固定的）

### 5.2 Transmission API

**优点**:
1. 单一端点，简单直接
2. JSON-RPC 风格，易于封装客户端库
3. 字段选择器，按需获取数据
4. 统一的请求/响应格式
5. CSRF 保护机制完善

**缺点**:
1. 无服务端过滤，大数据量时效率低
2. 无原生增量同步
3. 功能相对简单，缺少高级特性
4. 错误处理不够详细

## 6. 对 PT-Forward 的启示

### 6.1 API 设计建议

1. **采用混合模式**:
   - 核心操作使用 JSON-RPC 风格（简洁）
   - 复杂查询使用 RESTful 端点（灵活）

2. **实现增量同步**:
   - 参考 qBittorrent 的 SyncController
   - 使用版本号或时间戳追踪变更

3. **字段选择器**:
   - 参考 Transmission 的 fields 参数
   - 减少不必要的数据传输

4. **批量操作**:
   - 使用 JSON 数组而非字符串拼接
   - 支持批量操作的原子性

### 6.2 统一抽象层设计

```rust
/// 统一的下载器 API 抽象
pub trait DownloaderApi {
    // 种子操作
    async fn get_torrents(&self, filter: TorrentFilter) -> Result<Vec<TorrentInfo>>;
    async fn get_torrent(&self, id: &str) -> Result<TorrentInfo>;
    async fn add_torrent(&self, params: AddTorrentParams) -> Result<TorrentInfo>;
    async fn remove_torrent(&self, id: &str, delete_files: bool) -> Result<()>;
    
    // 状态控制
    async fn start_torrent(&self, ids: &[&str]) -> Result<()>;
    async fn stop_torrent(&self, ids: &[&str]) -> Result<()>;
    async fn recheck_torrent(&self, ids: &[&str]) -> Result<()>;
    
    // 属性修改
    async fn set_torrent_location(&self, id: &str, path: &str) -> Result<()>;
    async fn set_torrent_tags(&self, id: &str, tags: &[&str]) -> Result<()>;
    async fn set_torrent_limits(&self, id: &str, limits: SpeedLimits) -> Result<()>;
    
    // 全局控制
    async fn get_session_stats(&self) -> Result<SessionStats>;
    async fn set_speed_limits(&self, limits: SpeedLimits) -> Result<()>;
    
    // 增量同步
    async fn sync_maindata(&self, rid: i64) -> Result<MainDataSync>;
}
```

### 6.3 适配器实现示例

```rust
// qBittorrent 适配器
pub struct QbittorrentApi {
    base_url: String,
    sid: Option<String>,
}

impl DownloaderApi for QbittorrentApi {
    async fn get_torrents(&self, filter: TorrentFilter) -> Result<Vec<TorrentInfo>> {
        let mut params = vec![];
        if let Some(f) = filter.status {
            params.push(("filter", f.to_string()));
        }
        if let Some(cat) = filter.category {
            params.push(("category", cat));
        }
        // ...
        let resp = self.get("/api/v2/torrents/info", &params).await?;
        Ok(serde_json::from_value(resp)?)
    }
}

// Transmission 适配器
pub struct TransmissionApi {
    base_url: String,
    session_id: Option<String>,
}

impl DownloaderApi for TransmissionApi {
    async fn get_torrents(&self, filter: TorrentFilter) -> Result<Vec<TorrentInfo>> {
        let fields = vec!["id", "name", "totalSize", "progress", /* ... */];
        let resp = self.rpc_call("torrent-get", json!({
            "fields": fields
        })).await?;
        Ok(serde_json::from_value(resp["arguments"]["torrents"].clone())?)
    }
}
```

## 7. 总结

| 维度 | qBittorrent | Transmission | 推荐方案 |
|------|-------------|--------------|----------|
| API 风格 | RESTful | JSON-RPC | 混合使用 |
| 数据获取 | 固定结构 | 字段选择 | 字段选择 |
| 过滤排序 | 服务端 | 客户端 | 服务端 |
| 增量同步 | 原生支持 | 不支持 | 必须实现 |
| 批量操作 | 管道分隔 | JSON 数组 | JSON 数组 |
| 认证方式 | Cookie Session | Basic + CSRF | JWT Token |
| 错误处理 | HTTP 状态码 | result 字段 | 统一错误结构 |

PT-Forward 应该:
1. 提供统一的下载器抽象接口
2. 为 qBittorrent 和 Transmission 分别实现适配器
3. 支持增量同步以提高效率
4. 使用字段选择器减少数据传输
5. 实现完善的错误处理机制

---

## 8. 深入实现细节

### 8.1 qBittorrent 完整 API 端点清单

#### TorrentsController 完整端点

| Action | 方法 | 参数 | 功能描述 |
|--------|------|------|----------|
| `info` | GET | filter, category, tag, hashes, sort, reverse, limit, offset, private, includeTrackers | 获取种子列表（支持高级过滤） |
| `properties` | GET | hash | 获取种子详细属性 |
| `trackers` | GET | hash | 获取 Tracker 列表（含 DHT/PeX/LSD 虚拟 Tracker） |
| `webseeds` | GET | hash | 获取 WebSeed 列表 |
| `files` | GET | hash, indexes | 获取文件列表（支持索引过滤） |
| `pieceHashes` | GET | hash | 获取分块哈希列表 |
| `pieceStates` | GET | hash | 获取分块状态（0=未下载,1=下载中,2=已完成） |
| `add` | POST | urls, torrents(文件), savepath, category, tags, skip_checking, etc. | 添加种子 |
| `delete` | POST | hashes, deleteFiles | 删除种子 |
| `start` | POST | hashes | 开始下载 |
| `stop` | POST | hashes | 暂停下载 |
| `recheck` | POST | hashes | 重新校验 |
| `reannounce` | POST | hashes | 重新宣告 |
| `addTrackers` | POST | hash, urls | 添加 Tracker |
| `editTracker` | POST | hash, origUrl, newUrl | 编辑 Tracker |
| `removeTrackers` | POST | hash, urls | 移除 Tracker（支持 `*` 全部种子） |
| `addPeers` | POST | hashes, peers | 手动添加 Peer |
| `filePrio` | POST | hash, id, priority | 设置文件优先级 |
| `uploadLimit` | GET | hashes | 获取上传限速 |
| `downloadLimit` | GET | hashes | 获取下载限速 |
| `setUploadLimit` | POST | hashes, limit | 设置上传限速 |
| `setDownloadLimit` | POST | hashes, limit | 设置下载限速 |
| `setShareLimits` | POST | hashes, ratioLimit, seedingTimeLimit, inactiveSeedingTimeLimit | 设置分享限制 |
| `toggleSequentialDownload` | POST | hashes | 切换顺序下载 |
| `toggleFirstLastPiecePrio` | POST | hashes | 切换首尾块优先 |
| `setSuperSeeding` | POST | hashes, value | 设置超级做种 |
| `setForceStart` | POST | hashes, value | 设置强制开始 |
| `increasePrio` | POST | hashes | 提高队列优先级 |
| `decreasePrio` | POST | hashes | 降低队列优先级 |
| `topPrio` | POST | hashes | 移至队首 |
| `bottomPrio` | POST | hashes | 移至队尾 |
| `setLocation` | POST | hashes, location | 设置保存位置 |
| `setSavePath` | POST | id, path | 设置保存路径 |
| `setDownloadPath` | POST | id, path | 设置临时下载路径 |
| `rename` | POST | hash, name | 重命名种子 |
| `setAutoManagement` | POST | hashes, enable | 设置自动 TMM |
| `setCategory` | POST | hashes, category | 设置分类 |
| `createCategory` | POST | category, savePath | 创建分类 |
| `editCategory` | POST | category, savePath | 编辑分类 |
| `removeCategories` | POST | categories | 删除分类 |
| `categories` | GET | - | 获取所有分类 |
| `addTags` | POST | hashes, tags | 添加标签 |
| `removeTags` | POST | hashes, tags | 移除标签 |
| `createTags` | POST | tags | 创建标签 |
| `deleteTags` | POST | tags | 删除标签 |
| `tags` | GET | - | 获取所有标签 |
| `count` | GET | - | 获取种子数量 |
| `addWebSeeds` | POST | hash, urls | 添加 WebSeed |
| `editWebSeed` | POST | hash, origUrl, newUrl | 编辑 WebSeed |
| `removeWebSeeds` | POST | hash, urls | 移除 WebSeed |

#### addAction 完整参数

```cpp
BitTorrent::AddTorrentParams {
    name: String,                    // 重命名
    category: String,                // 分类
    tags: Set<Tag>,                  // 标签
    savePath: Path,                  // 保存路径
    useDownloadPath: Option<bool>,   // 使用临时下载路径
    downloadPath: Path,              // 临时下载路径
    sequential: bool,                // 顺序下载
    firstLastPiecePriority: bool,    // 首尾块优先
    addForced: bool,                 // 强制开始
    addToQueueTop: Option<bool>,     // 添加到队列顶部
    addStopped: Option<bool>,        // 添加后暂停
    stopCondition: Option<StopCondition>, // 停止条件
    filePaths: List<Path>,           // 文件路径映射
    filePriorities: List<Priority>,  // 文件优先级
    skipChecking: bool,              // 跳过校验
    contentLayout: Option<ContentLayout>, // 内容布局
    useAutoTMM: Option<bool>,        // 使用自动 TMM
    uploadLimit: int,                // 上传限速
    downloadLimit: int,              // 下载限速
    seedingTimeLimit: int,           // 做种时间限制
    inactiveSeedingTimeLimit: int,   // 不活跃做种时间限制
    ratioLimit: double,              // 分享率限制
    shareLimitAction: ShareLimitAction, // 分享限制动作
    sslParameters: SSLParams         // SSL 参数（私有种子）
}
```

#### RSSController 完整端点

| Action | 参数 | 功能 |
|--------|------|------|
| `addFolder` | path | 添加 RSS 文件夹 |
| `addFeed` | url, path | 添加 RSS 订阅 |
| `setFeedURL` | path, url | 修改订阅 URL |
| `removeItem` | path | 删除项目 |
| `moveItem` | itemPath, destPath | 移动项目 |
| `items` | withData | 获取所有项目 |
| `markAsRead` | itemPath, articleId | 标记已读 |
| `refreshItem` | itemPath | 刷新项目 |
| `setRule` | ruleName, ruleDef(JSON) | 设置自动下载规则 |
| `renameRule` | ruleName, newRuleName | 重命名规则 |
| `removeRule` | ruleName | 删除规则 |
| `rules` | - | 获取所有规则 |
| `matchingArticles` | ruleName | 获取匹配的文章 |

#### SearchController 完整端点

| Action | 参数 | 功能 |
|--------|------|------|
| `start` | pattern, category, plugins | 开始搜索 |
| `stop` | id | 停止搜索 |
| `status` | id | 获取搜索状态 |
| `results` | id, limit, offset | 获取搜索结果 |
| `delete` | id | 删除搜索 |
| `downloadTorrent` | torrentUrl, pluginName | 下载搜索结果 |
| `plugins` | - | 获取插件列表 |
| `installPlugin` | sources | 安装插件 |
| `uninstallPlugin` | names | 卸载插件 |
| `enablePlugin` | names, enable | 启用/禁用插件 |
| `updatePlugins` | - | 更新插件 |

#### LogController 完整端点

| Action | 参数 | 功能 |
|--------|------|------|
| `main` | normal, info, warning, critical, last_known_id | 获取主日志 |
| `peers` | last_known_id | 获取 Peer 日志 |

### 8.2 Transmission 完整方法清单

#### RPC 方法表

```cpp
// rpcimpl.cc
auto constexpr Methods = std::array<rpc_method, 24>{ {
    { "blocklist-update"sv, false, blocklistUpdate },  // 异步
    { "free-space"sv, true, freeSpace },               // 同步
    { "group-get"sv, true, groupGet },                 // 同步
    { "group-set"sv, true, groupSet },                 // 同步
    { "port-test"sv, false, portTest },                // 异步
    { "queue-move-bottom"sv, true, queueMoveBottom },  // 同步
    { "queue-move-down"sv, true, queueMoveDown },      // 同步
    { "queue-move-top"sv, true, queueMoveTop },        // 同步
    { "queue-move-up"sv, true, queueMoveUp },          // 同步
    { "session-close"sv, true, sessionClose },         // 同步
    { "session-get"sv, true, sessionGet },             // 同步
    { "session-set"sv, true, sessionSet },             // 同步
    { "session-stats"sv, true, sessionStats },         // 同步
    { "torrent-add"sv, false, torrentAdd },            // 异步
    { "torrent-get"sv, true, torrentGet },             // 同步
    { "torrent-reannounce"sv, true, torrentReannounce },// 同步
    { "torrent-remove"sv, true, torrentRemove },       // 同步
    { "torrent-rename-path"sv, false, torrentRenamePath },// 异步
    { "torrent-set"sv, true, torrentSet },             // 同步
    { "torrent-set-location"sv, true, torrentSetLocation },// 同步
    { "torrent-start"sv, true, torrentStart },         // 同步
    { "torrent-start-now"sv, true, torrentStartNow },  // 同步
    { "torrent-stop"sv, true, torrentStop },           // 同步
    { "torrent-verify"sv, true, torrentVerify },       // 同步
} };
```

#### torrent-get 支持的字段（完整列表）

```cpp
[[nodiscard]] auto constexpr isSupportedTorrentGetField(tr_quark key)
{
    switch (key)
    {
    case TR_KEY_activityDate:       // 最后活动时间
    case TR_KEY_addedDate:          // 添加时间
    case TR_KEY_availability:       // 分块可用性数组
    case TR_KEY_bandwidthPriority:  // 带宽优先级
    case TR_KEY_comment:            // 注释
    case TR_KEY_corruptEver:        // 损坏数据量
    case TR_KEY_creator:            // 创建者
    case TR_KEY_dateCreated:        // 创建时间
    case TR_KEY_desiredAvailable:   // 可用数据量
    case TR_KEY_doneDate:           // 完成时间
    case TR_KEY_downloadDir:        // 下载目录
    case TR_KEY_downloadLimit:      // 下载限速
    case TR_KEY_downloadLimited:    // 是否启用下载限速
    case TR_KEY_downloadedEver:     // 总下载量
    case TR_KEY_editDate:           // 编辑时间
    case TR_KEY_error:              // 错误码
    case TR_KEY_errorString:        // 错误信息
    case TR_KEY_eta:                // 预计完成时间
    case TR_KEY_etaIdle:            // 空闲 ETA
    case TR_KEY_fileStats:          // 文件统计
    case TR_KEY_file_count:         // 文件数量
    case TR_KEY_files:              // 文件列表
    case TR_KEY_group:              // 带宽组
    case TR_KEY_hashString:         // 哈希字符串
    case TR_KEY_haveUnchecked:      // 未校验数据量
    case TR_KEY_haveValid:          // 已校验数据量
    case TR_KEY_honorsSessionLimits:// 是否遵守会话限制
    case TR_KEY_id:                 // 种子 ID
    case TR_KEY_isFinished:         // 是否完成
    case TR_KEY_isPrivate:          // 是否私有
    case TR_KEY_isStalled:          // 是否停滞
    case TR_KEY_labels:             // 标签列表
    case TR_KEY_leftUntilDone:      // 剩余数据量
    case TR_KEY_magnetLink:         // 磁力链接
    case TR_KEY_manualAnnounceTime: // 手动宣告时间
    case TR_KEY_maxConnectedPeers:  // 最大连接数
    case TR_KEY_metadataPercentComplete: // 元数据完成度
    case TR_KEY_name:               // 种子名称
    case TR_KEY_peer_limit:         // Peer 限制
    case TR_KEY_peers:              // Peer 列表
    case TR_KEY_peersConnected:     // 已连接 Peer 数
    case TR_KEY_peersFrom:          // Peer 来源统计
    case TR_KEY_peersGettingFromUs: // 从我们下载的 Peer 数
    case TR_KEY_peersSendingToUs:   // 向我们上传的 Peer 数
    case TR_KEY_percentComplete:    // 完成百分比
    case TR_KEY_percentDone:        // 已完成百分比
    case TR_KEY_pieceCount:         // 分块数量
    case TR_KEY_pieceSize:          // 分块大小
    case TR_KEY_pieces:             // 分块位图（base64）
    case TR_KEY_primary_mime_type:  // 主 MIME 类型
    case TR_KEY_priorities:         // 文件优先级数组
    case TR_KEY_queuePosition:      // 队列位置
    case TR_KEY_rateDownload:       // 下载速率
    case TR_KEY_rateUpload:         // 上传速率
    case TR_KEY_recheckProgress:    // 校验进度
    case TR_KEY_secondsDownloading: // 下载秒数
    case TR_KEY_secondsSeeding:     // 做种秒数
    case TR_KEY_seedIdleLimit:      // 做种空闲限制
    case TR_KEY_seedIdleMode:       // 做种空闲模式
    case TR_KEY_seedRatioLimit:     // 分享率限制
    case TR_KEY_seedRatioMode:      // 分享率模式
    case TR_KEY_sizeWhenDone:       // 完成时大小
    case TR_KEY_source:             // 来源
    case TR_KEY_startDate:          // 开始时间
    case TR_KEY_status:             // 状态
    case TR_KEY_torrentFile:        // 种子文件路径
    case TR_KEY_totalSize:          // 总大小
    case TR_KEY_trackerList:        // Tracker 列表（文本）
    case TR_KEY_trackerStats:       // Tracker 统计
    case TR_KEY_trackers:           // Tracker 列表
    case TR_KEY_uploadLimit:        // 上传限速
    case TR_KEY_uploadLimited:      // 是否启用上传限速
    case TR_KEY_uploadRatio:        // 分享率
    case TR_KEY_uploadedEver:       // 总上传量
    case TR_KEY_wanted:             // 需要的文件数组
    case TR_KEY_webseeds:           // WebSeed 列表
    case TR_KEY_webseedsSendingToUs:// 向我们发送的 WebSeed 数
        return true;
    default:
        return false;
    }
}
```

#### torrent-set 支持的参数

```cpp
char const* torrentSet(tr_session* session, tr_variant* args_in, ...)
{
    // 带宽优先级
    TR_KEY_bandwidthPriority
    
    // 带宽组
    TR_KEY_group
    
    // 标签
    TR_KEY_labels
    
    // 文件选择
    TR_KEY_files_unwanted
    TR_KEY_files_wanted
    
    // 文件优先级
    TR_KEY_priority_high
    TR_KEY_priority_low
    TR_KEY_priority_normal
    
    // 速度限制
    TR_KEY_downloadLimit
    TR_KEY_downloadLimited
    TR_KEY_uploadLimit
    TR_KEY_uploadLimited
    TR_KEY_honorsSessionLimits
    
    // 做种限制
    TR_KEY_seedIdleLimit
    TR_KEY_seedIdleMode
    TR_KEY_seedRatioLimit
    TR_KEY_seedRatioMode
    
    // 队列位置
    TR_KEY_queuePosition
    
    // Peer 限制
    TR_KEY_peer_limit
    
    // Tracker 管理（已弃用，推荐使用 trackerList）
    TR_KEY_trackerAdd
    TR_KEY_trackerRemove
    TR_KEY_trackerReplace
    
    // Tracker 列表（推荐）
    TR_KEY_trackerList
}
```

#### torrent-add 参数

```cpp
char const* torrentAdd(tr_session* session, tr_variant* args_in, ...)
{
    // 必需参数（二选一）
    TR_KEY_filename      // 文件名或 URL
    TR_KEY_metainfo      // base64 编码的种子内容
    
    // 可选参数
    TR_KEY_download_dir  // 下载目录
    TR_KEY_paused        // 是否暂停
    TR_KEY_peer_limit    // Peer 限制
    TR_KEY_bandwidthPriority // 带宽优先级
    TR_KEY_cookies       // Cookie（用于 URL）
    
    // 文件选择
    TR_KEY_files_wanted
    TR_KEY_files_unwanted
    
    // 文件优先级
    TR_KEY_priority_high
    TR_KEY_priority_normal
    TR_KEY_priority_low
    
    // 标签
    TR_KEY_labels
}
```

### 8.3 错误处理机制对比

#### qBittorrent 错误处理

```cpp
// apierror.h
enum class APIErrorType
{
    BadParams,      // 参数错误
    BadData,        // 数据错误
    NotFound,       // 未找到
    AccessDenied,   // 访问拒绝
    Conflict        // 冲突
};

class APIError : public RuntimeError
{
public:
    explicit APIError(APIErrorType type, const QString &message = {});
    APIErrorType type() const;
};
```

**使用示例**:
```cpp
void TorrentsController::propertiesAction()
{
    requireParams({u"hash"_s});  // 缺少参数抛出 BadParams

    const auto id = BitTorrent::TorrentID::fromString(params()[u"hash"_s]);
    const BitTorrent::Torrent *const torrent = BitTorrent::Session::instance()->getTorrent(id);
    if (!torrent)
        throw APIError(APIErrorType::NotFound);  // 未找到抛出 NotFound
    
    // ...
}

void TorrentsController::filePrioAction()
{
    // ...
    if (!BitTorrent::isValidDownloadPriority(priority))
        throw APIError(APIErrorType::BadParams, tr("Priority is not valid"));
    
    if (!torrent->hasMetadata())
        throw APIError(APIErrorType::Conflict, tr("Torrent's metadata has not yet downloaded"));
    // ...
}
```

**HTTP 状态码映射**:
- `BadParams` → 400 Bad Request
- `BadData` → 400 Bad Request
- `NotFound` → 404 Not Found
- `AccessDenied` → 403 Forbidden
- `Conflict` → 409 Conflict

#### Transmission 错误处理

```cpp
// 通过 result 字段返回错误信息
void tr_idle_function_done(struct tr_rpc_idle_data* data, std::string_view result)
{
    tr_variantDictAddStr(&data->response, TR_KEY_result, result);
    (*data->callback)(data->session, &data->response, data->callback_user_data);
}

// 错误返回示例
char const* torrentAdd(...)
{
    if (std::empty(filename) && std::empty(metainfo_base64))
    {
        return "no filename or metainfo specified";
    }
    
    if (tr_sys_path_is_relative(download_dir))
    {
        return "download directory path is not absolute";
    }
    
    // ...
}
```

**常见错误信息**:
- `"no method name"` - 未指定方法
- `"method name not recognized"` - 方法名不识别
- `"no fields specified"` - 未指定字段
- `"no filename or metainfo specified"` - 未指定种子来源
- `"invalid or corrupt torrent file"` - 无效的种子文件
- `"file index out of range"` - 文件索引越界
- `"torrent-rename-path requires 1 torrent"` - 重命名需要单个种子

### 8.4 会话管理机制对比

#### qBittorrent 会话管理

```cpp
// isessionmanager.h
struct ISession
{
    virtual ~ISession() = default;
    virtual QString id() const = 0;
};

struct ISessionManager
{
    virtual ~ISessionManager() = default;
    virtual QString clientId() const = 0;      // 客户端标识（IP）
    virtual ISession *session() = 0;           // 当前会话
    virtual void sessionStart() = 0;           // 开始会话
    virtual void sessionEnd() = 0;             // 结束会话
};
```

**认证流程**:
```cpp
void AuthController::loginAction()
{
    // 1. 检查是否已登录
    if (m_sessionManager->session())
    {
        setResult(u"Ok."_s);
        return;
    }

    // 2. 检查是否被封禁
    if (isBanned())
    {
        throw APIError(APIErrorType::AccessDenied,
            tr("Your IP address has been banned after too many failed authentication attempts."));
    }

    // 3. 验证凭据
    const bool usernameEqual = Utils::Password::slowEquals(usernameFromWeb.toUtf8(), m_username.toUtf8());
    const bool passwordEqual = Utils::Password::PBKDF2::verify(m_passwordHash, passwordFromWeb);

    if (usernameEqual && passwordEqual)
    {
        // 4. 登录成功
        m_clientFailedLogins.remove(clientAddr);
        m_sessionManager->sessionStart();
        setResult(u"Ok."_s);
    }
    else
    {
        // 5. 登录失败，增加计数
        if (Preferences::instance()->getWebUIMaxAuthFailCount() > 0)
            increaseFailedAttempts();
        setResult(u"Fails."_s);
    }
}
```

**防暴力破解机制**:
```cpp
struct FailedLogin
{
    int failedAttemptsCount = 0;
    QDeadlineTimer banTimer {-1};  // 封禁计时器
};

void AuthController::increaseFailedAttempts()
{
    FailedLogin &failedLogin = m_clientFailedLogins[m_sessionManager->clientId()];
    ++failedLogin.failedAttemptsCount;

    if (failedLogin.failedAttemptsCount >= Preferences::instance()->getWebUIMaxAuthFailCount())
    {
        // 达到最大失败次数，开始封禁
        failedLogin.banTimer.setRemainingTime(Preferences::instance()->getWebUIBanDuration());
    }
}
```

#### Transmission 会话管理

```cpp
// rpc-server.h
class tr_rpc_server
{
    // 认证设置
    bool authentication_required_ = false;
    std::string username_;
    std::string salted_password_;  // PBKDF2 加密的密码
    
    // 防暴力破解
    bool is_anti_brute_force_enabled_ = false;
    size_t anti_brute_force_limit_ = 100U;
    size_t login_attempts_ = 0U;
    
    // CSRF 保护
    // 通过 X-Transmission-Session-Id 头实现
    
    // Host 白名单（防 DNS 重绑定）
    bool is_host_whitelist_enabled_ = true;
    std::vector<std::string> host_whitelist_;
    
    // IP 白名单
    bool is_whitelist_enabled_ = true;
    std::vector<std::string> whitelist_;
};
```

**CSRF 保护流程**:
1. 客户端首次请求（无 Session ID）
2. 服务器返回 HTTP 409 Conflict
3. 响应头包含 `X-Transmission-Session-Id: xxxxx`
4. 客户端重新发送请求，携带此 ID
5. 后续请求必须携带此 ID

### 8.5 异步操作处理

#### Transmission 异步操作模式

```cpp
// 异步操作的回调数据结构
struct tr_rpc_idle_data
{
    tr_variant response = {};
    tr_session* session = nullptr;
    tr_variant* args_out = nullptr;
    tr_rpc_response_func callback = nullptr;
    void* callback_user_data = nullptr;
};

// 异步方法标记
{ "torrent-add"sv, false, torrentAdd },      // false = 异步
{ "port-test"sv, false, portTest },          // false = 异步
{ "blocklist-update"sv, false, blocklistUpdate }, // false = 异步

// 异步操作完成时调用
void tr_idle_function_done(struct tr_rpc_idle_data* data, std::string_view result)
{
    tr_variantDictAddStr(&data->response, TR_KEY_result, result);
    (*data->callback)(data->session, &data->response, data->callback_user_data);
    tr_variantClear(&data->response);
    delete data;
}
```

**torrent-add 异步实现**:
```cpp
char const* torrentAdd(tr_session* session, tr_variant* args_in, ...)
{
    // ...
    
    if (isCurlURL(filename))
    {
        // URL 方式：异步下载种子文件
        auto* const d = new add_torrent_idle_data{ idle_data, ctor };
        auto options = tr_web::FetchOptions{ filename, onMetadataFetched, d };
        options.cookies = cookies;
        session->fetch(std::move(options));
        return nullptr;  // 返回 nullptr 表示异步进行中
    }
    else
    {
        // 本地文件：同步处理
        addTorrentImpl(idle_data, ctor);
    }
}

// URL 下载完成回调
void onMetadataFetched(tr_web::FetchResponse const& web_response)
{
    auto* data = static_cast<struct add_torrent_idle_data*>(user_data);
    
    if (status == 200 || status == 221)
    {
        tr_ctorSetMetainfo(data->ctor, std::data(body), std::size(body), nullptr);
        addTorrentImpl(data->data, data->ctor);
    }
    else
    {
        tr_idle_function_done(data->data, "Couldn't fetch torrent");
    }
    
    delete data;
}
```

#### qBittorrent 同步模式

qBittorrent 的 WebUI API 主要是同步模式，所有操作在请求处理期间完成。

```cpp
// 添加种子是同步的
void TorrentsController::addAction()
{
    // ...
    for (QString url : asConst(urls.split(u'\n')))
    {
        url = url.trimmed();
        if (!url.isEmpty())
        {
            partialSuccess |= app()->addTorrentManager()->addTorrent(url, addTorrentParams);
        }
    }
    
    if (partialSuccess)
        setResult(u"Ok."_s);
    else
        setResult(u"Fails."_s);
}
```

### 8.6 数据格式对比

#### 种子状态枚举

**qBittorrent**:
```cpp
enum class TorrentState
{
    Unknown = -1,
    ForcedDL = 0,        // 强制下载
    Downloading = 1,     // 下载中
    MetaDL = 2,          // 下载元数据
    ForcedMetaDL = 3,    // 强制下载元数据
    StalledDL = 4,       // 下载停滞
    ForcedUP = 5,        // 强制上传
    Uploading = 6,       // 上传中
    StalledUP = 7,       // 上传停滞
    QueuedDL = 8,        // 排队下载
    QueuedUP = 9,        // 排队上传
    CheckingDL = 10,     // 校验中（下载）
    CheckingUP = 11,     // 校验中（上传）
    CheckingResumeData = 12, // 校验恢复数据
    PausedDL = 13,       // 暂停下载
    PausedUP = 14,       // 暂停上传
    Moving = 15,         // 移动中
    MissingFiles = 16,   // 文件丢失
    Error = 17           // 错误
};
```

**Transmission**:
```cpp
// status 字段值
// 0: Torrent is stopped
// 1: Torrent is queued to verify local data
// 2: Torrent is verifying local data
// 3: Torrent is queued to download
// 4: Torrent is downloading
// 5: Torrent is queued to seed
// 6: Torrent is seeding
```

#### 文件优先级

**qBittorrent**:
```cpp
enum class DownloadPriority
{
    Ignored = 0,      // 忽略
    Normal = 1,       // 正常
    High = 6,         // 高
    Maximum = 7       // 最高
};
```

**Transmission**:
```cpp
// TR_PRI_LOW    = -1  // 低优先级
// TR_PRI_NORMAL = 0   // 正常
// TR_PRI_HIGH   = 1   // 高优先级
```

---

## 9. PT-Forward 实现建议

### 9.1 统一数据模型

```rust
/// 统一种子状态
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TorrentState {
    Stopped,
    QueuedToVerify,
    Verifying,
    QueuedToDownload,
    Downloading,
    QueuedToSeed,
    Seeding,
    Error,
    MissingFiles,
    Moving,
    Unknown,
}

impl From<qbittorrent::TorrentState> for TorrentState { ... }
impl From<transmission::TorrentStatus> for TorrentState { ... }

/// 统一种子信息
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TorrentInfo {
    pub id: String,
    pub hash: String,
    pub name: String,
    pub state: TorrentState,
    pub total_size: u64,
    pub progress: f64,
    pub downloaded: u64,
    pub uploaded: u64,
    pub download_speed: u64,
    pub upload_speed: u64,
    pub eta: Option<i64>,
    pub ratio: f64,
    pub category: Option<String>,
    pub tags: Vec<String>,
    pub save_path: String,
    pub added_on: i64,
    pub completion_on: Option<i64>,
    pub trackers: Vec<TrackerInfo>,
    pub peers: Vec<PeerInfo>,
    pub files: Vec<FileInfo>,
}

/// 统一添加参数
#[derive(Debug, Clone, Default)]
pub struct AddTorrentParams {
    pub urls: Vec<String>,
    pub save_path: Option<String>,
    pub category: Option<String>,
    pub tags: Vec<String>,
    pub paused: bool,
    pub skip_checking: bool,
    pub sequential_download: bool,
    pub first_last_piece_priority: bool,
    pub upload_limit: Option<i64>,
    pub download_limit: Option<i64>,
    pub ratio_limit: Option<f64>,
    pub seeding_time_limit: Option<i64>,
    pub file_priorities: Vec<FilePriority>,
}
```

### 9.2 异步客户端设计

```rust
use async_trait::async_trait;

#[async_trait]
pub trait DownloaderClient: Send + Sync {
    // 种子操作
    async fn get_torrents(&self, filter: Option<TorrentFilter>) -> Result<Vec<TorrentInfo>>;
    async fn get_torrent(&self, id: &str) -> Result<Option<TorrentInfo>>;
    async fn add_torrent(&self, params: AddTorrentParams) -> Result<TorrentInfo>;
    async fn remove_torrent(&self, id: &str, delete_files: bool) -> Result<()>;
    
    // 状态控制
    async fn start_torrents(&self, ids: &[&str]) -> Result<()>;
    async fn stop_torrents(&self, ids: &[&str]) -> Result<()>;
    async fn recheck_torrents(&self, ids: &[&str]) -> Result<()>;
    async fn reannounce_torrents(&self, ids: &[&str]) -> Result<()>;
    
    // 属性修改
    async fn set_torrent_location(&self, id: &str, path: &str, move_files: bool) -> Result<()>;
    async fn set_torrent_category(&self, id: &str, category: &str) -> Result<()>;
    async fn set_torrent_tags(&self, id: &str, tags: &[&str]) -> Result<()>;
    async fn set_torrent_limits(&self, id: &str, limits: SpeedLimits) -> Result<()>;
    async fn set_file_priorities(&self, id: &str, priorities: &[(u32, FilePriority)]) -> Result<()>;
    
    // Tracker 管理
    async fn add_trackers(&self, id: &str, trackers: &[&str]) -> Result<()>;
    async fn remove_trackers(&self, id: &str, urls: &[&str]) -> Result<()>;
    async fn get_trackers(&self, id: &str) -> Result<Vec<TrackerInfo>>;
    
    // 全局控制
    async fn get_session_stats(&self) -> Result<SessionStats>;
    async fn set_speed_limits(&self, limits: SpeedLimits) -> Result<()>;
    async fn toggle_alt_speed(&self) -> Result<()>;
    
    // 增量同步（可选）
    async fn sync_maindata(&self, rid: Option<i64>) -> Result<MainDataSync>;
}

/// qBittorrent 客户端实现
pub struct QbittorrentClient {
    base_url: String,
    sid: Option<String>,
    client: reqwest::Client,
}

/// Transmission 客户端实现
pub struct TransmissionClient {
    base_url: String,
    session_id: Option<String>,
    client: reqwest::Client,
}
```

### 9.3 错误处理统一

```rust
#[derive(Debug, thiserror::Error)]
pub enum DownloaderError {
    #[error("Connection error: {0}")]
    Connection(#[from] reqwest::Error),
    
    #[error("Authentication failed")]
    AuthFailed,
    
    #[error("Torrent not found: {0}")]
    NotFound(String),
    
    #[error("Invalid parameter: {0}")]
    InvalidParam(String),
    
    #[error("Conflict: {0}")]
    Conflict(String),
    
    #[error("Access denied: {0}")]
    AccessDenied(String),
    
    #[error("API error: {0}")]
    ApiError(String),
    
    #[error("Parse error: {0}")]
    ParseError(#[from] serde_json::Error),
}

impl From<qbittorrent::APIError> for DownloaderError { ... }
impl From<transmission::RPCError> for DownloaderError { ... }
```

---

## 10. 总结

| 维度 | qBittorrent | Transmission | PT-Forward 建议 |
|------|-------------|--------------|-----------------|
| API 风格 | RESTful + Action | JSON-RPC | 统一抽象层 |
| 端点数量 | 80+ | 24 | 按需封装 |
| 数据获取 | 固定结构 | 字段选择 | 支持两种模式 |
| 过滤排序 | 服务端 | 客户端 | 服务端实现 |
| 增量同步 | 原生支持 | 不支持 | 必须实现 |
| 批量操作 | 管道分隔 | JSON 数组 | JSON 数组 |
| 认证方式 | Cookie Session | Basic + CSRF | JWT Token |
| 错误处理 | HTTP 状态码 | result 字段 | 统一错误类型 |
| 异步操作 | 同步为主 | 支持异步 | 全异步设计 |
| RSS 支持 | 原生支持 | 不支持 | 独立模块 |
| 搜索支持 | 原生支持 | 不支持 | 独立模块 |
