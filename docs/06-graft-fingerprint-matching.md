# Graft 项目深度分析

## 一、项目概述

**Graft** (嫁接) 是一个轻量级、自托管的 PT 辅种工具，使用 Rust 开发，专注于隐私保护和本地化运行。

### 1.1 核心特性

| 特性 | 说明 |
|------|------|
| **隐私优先** | 所有数据本地存储，无云依赖 |
| **单二进制** | 一个可执行文件，无运行时依赖 |
| **智能匹配** | 基于内容指纹的跨站点匹配 |
| **多客户端** | 支持 qBittorrent 和 Transmission |
| **现代UI** | SolidJS + Tailwind CSS 响应式界面 |

### 1.2 与 IYUU 的核心差异

| 维度 | IYUU | Graft |
|------|------|-------|
| Hash 匹配 | 依赖云端 API | **本地数据库** |
| 索引来源 | 云端维护 | **从用户下载器直接读取** |
| 用户认证 | 微信扫码绑定 | **无需任何认证** |
| 部署方式 | PHP + MySQL + 多扩展 | **单二进制文件** |
| 站点配置 | 云端维护更新 | **内置 + 社区仓库订阅** |
| 数据隐私 | Hash 上传云端 | **数据不出本地** |

---

## 二、技术架构

### 2.1 技术栈总览

```
┌────────────────────────────────────────────────────────┐
│                      Graft                             │
├────────────────────────────────────────────────────────┤
│  Frontend    │  SolidJS + Tailwind CSS + DaisyUI       │
├──────────────┼─────────────────────────────────────────┤
│  Backend     │  Rust + Axum + Tower                    │
├──────────────┼─────────────────────────────────────────┤
│  Database    │  SQLite (rusqlite)                      │
├──────────────┼─────────────────────────────────────────┤
│  Packaging   │  单二进制 / Docker                      │
└──────────────┴─────────────────────────────────────────┘
```

### 2.2 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                           Graft                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   Web UI     │    │   REST API   │    │  WebSocket   │      │
│  │  (SolidJS)   │◄──►│   (Axum)     │◄──►│  (实时日志)  │      │
│  └──────────────┘    └──────┬───────┘    └──────────────┘      │
│                             │                                   │
│         ┌───────────────────┼───────────────────┐               │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌────────────┐     ┌─────────────┐     ┌─────────────┐        │
│  │  Reseed    │     │   Client    │     │    Site     │        │
│  │  Service   │     │   Manager   │     │   Manager   │        │
│  └─────┬──────┘     └──────┬──────┘     └──────┬──────┘        │
│        │                   │                   │                │
│        │           ┌───────▼───────┐           │                │
│        │           │    Index      │           │                │
│        └──────────►│   Service     │◄──────────┘                │
│                    └───────┬───────┘                            │
│                            │                                    │
│                     ┌──────▼──────┐                             │
│                     │   SQLite    │                             │
│                     │  Database   │                             │
│                     └─────────────┘                             │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                      External Systems                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ qBittorrent  │    │ Transmission │    │  PT Sites    │      │
│  │    API       │    │     API      │    │  (下载种子)  │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

### 2.3 目录结构

```
graft/
├── src/
│   ├── main.rs                 # 入口点
│   ├── api/                    # HTTP API 层
│   │   ├── mod.rs
│   │   ├── handlers/           # 请求处理器
│   │   │   ├── client.rs       # 下载器管理
│   │   │   ├── site.rs         # 站点管理
│   │   │   ├── reseed.rs       # 辅种操作
│   │   │   └── index.rs        # 索引管理
│   │   └── error.rs            # 错误处理
│   │
│   ├── service/                # 业务逻辑层
│   │   ├── reseed.rs           # 辅种核心逻辑
│   │   ├── index.rs            # 索引服务
│   │   └── fingerprint.rs      # 内容指纹匹配
│   │
│   ├── client/                 # 下载器抽象层
│   │   ├── mod.rs              # 统一接口定义
│   │   ├── qbittorrent.rs      # qBittorrent 实现
│   │   └── transmission.rs     # Transmission 实现
│   │
│   ├── site/                   # 站点适配层
│   │   ├── mod.rs              # 站点配置
│   │   ├── tracker.rs          # Tracker URL 识别
│   │   └── templates/          # 站点模板
│   │       ├── nexusphp.rs     # NexusPHP 模板
│   │       ├── unit3d.rs       # Unit3D 模板
│   │       └── gazelle.rs      # Gazelle 模板
│   │
│   ├── db/                     # 数据访问层
│   │   ├── mod.rs
│   │   └── repository/
│   │
│   └── config/                 # 配置管理
│       └── mod.rs
│
├── web/                        # 前端项目 (SolidJS)
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   ├── pages/
│   │   └── api/
│   └── package.json
│
├── migrations/                 # SQL 迁移文件
│   └── 001_initial.sql
│
├── Cargo.toml
├── Dockerfile
└── docker-compose.yml
```

---

## 三、核心模块分析

### 3.1 下载器抽象层 (`src/client/`)

#### 3.1.1 统一接口定义

```rust
/// BitTorrent 客户端统一接口
#[async_trait]
pub trait BitTorrentClient: Send + Sync {
    fn client_type(&self) -> ClientType;
    fn client_id(&self) -> &str;
    
    async fn test_connection(&self) -> Result<bool>;
    async fn get_torrents(&self) -> Result<Vec<TorrentInfo>>;
    async fn get_torrent(&self, hash: &str) -> Result<Option<TorrentInfo>>;
    async fn get_torrent_files(&self, hash: &str) -> Result<Vec<TorrentFile>>;
    async fn get_torrent_trackers(&self, hash: &str) -> Result<Vec<String>>;
    
    async fn add_torrent(&self, torrent_bytes: &[u8], options: AddTorrentOptions) -> Result<String>;
    async fn remove_torrent(&self, hash: &str, delete_files: bool) -> Result<()>;
    async fn pause_torrent(&self, hash: &str) -> Result<()>;
    async fn resume_torrent(&self, hash: &str) -> Result<()>;
    async fn recheck_torrent(&self, hash: &str) -> Result<()>;
}
```

#### 3.1.2 数据模型

```rust
/// 种子信息
pub struct TorrentInfo {
    pub hash: String,
    pub name: String,
    pub size: u64,
    pub progress: f64,
    pub state: TorrentState,
    pub save_path: String,
    pub category: Option<String>,
    pub tags: Vec<String>,
    pub tracker: Option<String>,
    pub trackers: Vec<String>,
    pub files: Vec<TorrentFile>,
}

/// 种子文件信息
pub struct TorrentFile {
    pub name: String,
    pub size: u64,
    pub progress: f64,
}

/// 添加种子选项
pub struct AddTorrentOptions {
    pub save_path: Option<String>,
    pub category: Option<String>,
    pub tags: Vec<String>,
    pub paused: bool,
    pub skip_checking: bool,
}
```

#### 3.1.3 qBittorrent 实现

**特点**：
- **自动登录管理**: 检测 403 状态码自动重新登录
- **SID Cookie**: 使用 Session ID 维持会话
- **状态映射**: 将 qBittorrent 特定状态转换为通用枚举

```rust
impl QBittorrentClient {
    async fn ensure_logged_in(&self) -> Result<()> {
        let response = self.http.get(&self.api_url("/app/version")).send().await?;
        if response.status() == StatusCode::FORBIDDEN {
            self.login().await?;
        }
        Ok(())
    }
}

impl From<QBTorrent> for TorrentInfo {
    fn from(t: QBTorrent) -> Self {
        let state = match t.state.as_str() {
            "downloading" | "forcedDL" => TorrentState::Downloading,
            "uploading" | "stalledUP" => TorrentState::Seeding,
            "pausedDL" | "pausedUP" => TorrentState::Paused,
            "checkingDL" | "checkingUP" => TorrentState::Checking,
            "error" | "missingFiles" => TorrentState::Error,
            "queuedDL" | "queuedUP" => TorrentState::Queued,
            _ => TorrentState::Unknown,
        };
        // ...
    }
}
```

---

### 3.2 内容指纹匹配 (`src/service/fingerprint.rs`)

#### 3.2.1 核心设计理念

由于不同站点的种子 info_hash 不同（因为 tracker URL 不同），Graft 使用**内容指纹**进行匹配，而非依赖 info_hash。

#### 3.2.2 指纹数据结构

```rust
/// 内容指纹
pub struct ContentFingerprint {
    /// 所有文件总大小 (主匹配键)
    pub total_size: u64,
    
    /// 文件数量
    pub file_count: usize,
    
    /// 最大文件大小
    pub largest_file_size: u64,
    
    /// 文件列表哈希 (路径+大小，用于精确匹配)
    pub files_hash: Option<String>,
}

impl ContentFingerprint {
    /// 从文件列表创建指纹
    pub fn from_files(files: &[TorrentFile]) -> Self {
        let total_size: u64 = files.iter().map(|f| f.size).sum();
        let file_count = files.len();
        let largest_file_size = files.iter().map(|f| f.size).max().unwrap_or(0);
        
        // 计算文件列表哈希
        let files_hash = if !files.is_empty() {
            let mut hasher = Sha1::new();
            let mut sorted_files: Vec<_> = files.iter().collect();
            sorted_files.sort_by(|a, b| a.name.cmp(&b.name));
            
            for file in sorted_files {
                hasher.update(file.name.as_bytes());
                hasher.update(&file.size.to_le_bytes());
            }
            Some(hasher.digest().to_string())
        } else {
            None
        };
        
        Self { total_size, file_count, largest_file_size, files_hash }
    }
}
```

#### 3.2.3 匹配策略

```rust
/// 匹配结果
pub enum MatchResult {
    NoMatch,           // 不匹配
    LowConfidence,     // 低置信度 (仅大小匹配)
    MediumConfidence,  // 中置信度 (大小+结构相似)
    HighConfidence,    // 高置信度 (所有主要字段匹配)
    ExactMatch,        // 精确匹配 (files_hash 匹配)
}

impl ContentFingerprint {
    pub fn matches(&self, other: &ContentFingerprint) -> MatchResult {
        // 1. 总大小必须精确匹配
        if self.total_size != other.total_size {
            return MatchResult::NoMatch;
        }
        
        // 2. 如果都有 files_hash，使用它进行确定性匹配
        if let (Some(h1), Some(h2)) = (&self.files_hash, &other.files_hash) {
            return if h1 == h2 { MatchResult::ExactMatch } else { MatchResult::NoMatch };
        }
        
        // 3. 检查最大文件大小
        if self.largest_file_size != other.largest_file_size {
            return MatchResult::LowConfidence;
        }
        
        // 4. 检查文件数量 (允许 ±2 的差异，考虑元数据文件)
        let count_diff = (self.file_count as i64 - other.file_count as i64).abs();
        if count_diff > 2 {
            return MatchResult::LowConfidence;
        }
        
        if count_diff == 0 { MatchResult::HighConfidence } 
        else { MatchResult::MediumConfidence }
    }
}
```

#### 3.2.4 指纹匹配器

```rust
/// 指纹匹配器
pub struct FingerprintMatcher {
    /// 按总大小索引，用于快速查找
    size_index: HashMap<u64, Vec<FingerprintEntry>>,
}

impl FingerprintMatcher {
    /// 查找匹配项
    pub fn find_matches(&self, fingerprint: &ContentFingerprint) -> Vec<MatchedEntry> {
        let mut matches = Vec::new();
        
        // 按大小快速查找
        if let Some(candidates) = self.size_index.get(&fingerprint.total_size) {
            for candidate in candidates {
                let result = fingerprint.matches(&candidate.fingerprint);
                if result.is_match() {
                    matches.push(MatchedEntry {
                        entry: candidate.clone(),
                        match_result: result,
                    });
                }
            }
        }
        
        // 按置信度排序
        matches.sort_by(|a, b| {
            b.match_result.confidence()
                .partial_cmp(&a.match_result.confidence())
                .unwrap()
        });
        
        matches
    }
}
```

---

### 3.3 站点适配层 (`src/site/`)

#### 3.3.1 站点配置

```rust
pub struct SiteConfig {
    pub id: String,
    pub name: String,
    pub base_url: String,
    pub template_type: TemplateType,
    pub tracker_domains: Vec<String>,
    pub download_pattern: String,
    pub passkey: Option<String>,
    pub cookie: Option<String>,
    pub enabled: bool,
    pub rate_limit_rpm: Option<u32>,
}

pub enum TemplateType {
    NexusPHP,   // 国内主流站点
    Unit3D,     // 国外 Laravel 架构站点
    Gazelle,    // 音乐站
}
```

#### 3.3.2 Tracker URL 识别

```rust
pub struct TrackerIdentifier {
    domain_map: HashMap<String, String>,  // domain -> site_id
}

impl TrackerIdentifier {
    pub fn identify(&self, tracker_url: &str) -> Option<SiteIdentification> {
        let url = Url::parse(tracker_url).ok()?;
        let host = url.host_str()?;
        
        let site_id = self.find_site_by_host(host)?;
        let torrent_id = self.extract_torrent_id(&url);
        
        Some(SiteIdentification { site_id, torrent_id })
    }
    
    fn extract_torrent_id(&self, url: &Url) -> Option<String> {
        // 从查询参数提取: ?torrent_id=xxx, ?id=xxx, ?tid=xxx
        for (key, value) in url.query_pairs() {
            match key.as_ref() {
                "torrent_id" | "id" | "tid" => return Some(value.to_string()),
                _ => {}
            }
        }
        
        // 从路径提取: /announce/12345
        let segments: Vec<&str> = url.path().split('/').filter(|s| !s.is_empty()).collect();
        for segment in segments.iter().rev() {
            if segment.chars().all(|c| c.is_ascii_digit()) && segment.len() > 0 {
                return Some(segment.to_string());
            }
        }
        
        None
    }
}
```

#### 3.3.3 NexusPHP 模板实现

```rust
#[async_trait]
impl SiteTemplate for NexusPHPTemplate {
    fn build_download_url(&self, torrent_id: &str) -> Result<String> {
        let passkey = self.config.passkey.as_ref()
            .ok_or(TemplateError::MissingPasskey)?;
        
        let url = self.config.download_pattern
            .replace("{id}", torrent_id)
            .replace("{passkey}", passkey);
        
        Ok(format!("{}{}", self.config.base_url, url))
    }
    
    async fn download_torrent(&self, http_client: &reqwest::Client, torrent_id: &str) -> Result<Vec<u8>> {
        let url = self.build_download_url(torrent_id)?;
        let mut request = http_client.get(&url);
        
        if let Some(ref cookie) = self.config.cookie {
            request = request.header("Cookie", cookie);
        }
        
        let response = request.send().await?;
        let bytes = response.bytes().await?;
        
        // 验证是否为有效的 torrent 文件 (以 'd' 开头)
        if bytes.first() != Some(&b'd') {
            return Err(TemplateError::InvalidResponse("Invalid torrent file".into()));
        }
        
        Ok(bytes.to_vec())
    }
}
```

---

### 3.4 辅种服务 (`src/service/reseed.rs`)

#### 3.4.1 辅种执行流程

```
用户触发 / 定时任务
        │
        ▼
┌───────────────┐
│ 1. 获取种子列表 │  ←── 从源下载器获取当前做种
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ 2. 提取 Hash   │  ←── info_hash 列表
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ 3. 本地匹配    │  ←── 查询本地 torrent_index 表
└───────┬───────┘      找出在目标站点也存在的种子
        │
        ▼
┌───────────────┐
│ 4. 筛选结果    │  ←── 排除已存在、排除黑名单站点
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ 5. 下载种子    │  ←── 从目标站点下载 .torrent 文件
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ 6. 推送下载器  │  ←── 添加种子到目标下载器
└───────┬───────┘      设置相同的保存路径
        │
        ▼
┌───────────────┐
│ 7. 记录结果    │  ←── 写入辅种历史
└───────────────┘
```

#### 3.4.2 预览模式

```rust
pub async fn preview(
    &self,
    source_client: &dyn BitTorrentClient,
    target_sites: &[SiteConfig],
) -> Result<PreviewResult> {
    let torrents = source_client.get_torrents().await?;
    let matcher = self.index_service.build_matcher()?;
    
    let target_site_ids: HashSet<_> = target_sites.iter().map(|s| s.id.clone()).collect();
    let mut matches = Vec::new();
    
    for torrent in &torrents {
        let fingerprint = ContentFingerprint::from_files(&torrent.files);
        
        for matched in matcher.find_matches(&fingerprint) {
            if !target_site_ids.contains(&matched.entry.site_id) {
                continue;
            }
            
            matches.push(ReseedMatch {
                source_hash: torrent.hash.clone(),
                source_name: torrent.name.clone(),
                target_site: matched.entry.site_id.clone(),
                confidence: matched.match_result.confidence(),
                // ...
            });
        }
    }
    
    Ok(PreviewResult { matches, total_size })
}
```

#### 3.4.3 执行模式

```rust
pub async fn execute(
    &self,
    request: ReseedRequest,
    source_client: &dyn BitTorrentClient,
    target_client: &dyn BitTorrentClient,
    sites: &[SiteConfig],
) -> Result<ReseedResult> {
    let preview = self.preview(source_client, sites).await?;
    
    let existing_hashes: HashSet<String> = target_client
        .get_torrents().await?
        .into_iter()
        .map(|t| t.hash.to_lowercase())
        .collect();
    
    let mut result = ReseedResult::default();
    
    for m in preview.matches {
        result.total += 1;
        
        if existing_hashes.contains(&m.target_hash.to_lowercase()) {
            result.skipped += 1;
            continue;
        }
        
        let site = sites_map.get(&m.target_site)?;
        let torrent_bytes = site.create_template()
            .download_torrent(&self.http_client, &torrent_id).await?;
        
        let options = AddTorrentOptions {
            save_path: Some(m.save_path.clone()),
            paused: request.add_paused,
            skip_checking: request.skip_checking,
            ..Default::default()
        };
        
        match target_client.add_torrent(&torrent_bytes, options).await {
            Ok(_) => {
                result.success += 1;
                self.record_history(&m, "success", None)?;
            }
            Err(e) => {
                result.failed += 1;
                self.record_history(&m, "failed", Some(&format!("{}", e)))?;
            }
        }
        
        tokio::time::sleep(self.request_interval).await;
    }
    
    Ok(result)
}
```

---

## 四、前端架构

### 4.1 技术选型

**为什么选择 SolidJS？**

| 框架 | 打包体积 | 响应式方式 | 性能 | 学习曲线 |
|------|---------|-----------|------|---------|
| React | ~40KB | Virtual DOM | 良好 | 中等 |
| Vue | ~30KB | Proxy | 良好 | 低 |
| **SolidJS** | **~7KB** | **细粒度响应式** | **极佳** | 低 |

**SolidJS 核心优势**：
- 无 Virtual DOM，直接更新 DOM
- 细粒度响应式系统，只更新变化的部分
- 类 React 语法，学习成本低
- 运行时性能接近原生 JS

### 4.2 前端依赖

```json
{
  "dependencies": {
    "solid-js": "^1.9.3"
  },
  "devDependencies": {
    "@solidjs/router": "^0.15.3",
    "daisyui": "^4.12.22",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.7.2",
    "vite": "^6.0.6",
    "vite-plugin-solid": "^2.11.0"
  }
}
```

### 4.3 页面结构

```
web/src/
├── index.tsx              # 入口
├── App.tsx                # 根组件
├── components/
│   └── Layout.tsx         # 布局组件
├── pages/
│   ├── Dashboard.tsx      # 仪表板
│   ├── Sites.tsx          # 站点管理
│   ├── Clients.tsx        # 下载器管理
│   ├── Reseed.tsx         # 辅种操作
│   └── History.tsx        # 历史记录
└── api/
    ├── index.ts           # API 客户端
    ├── clients.ts         # 下载器 API
    ├── sites.ts           # 站点 API
    ├── reseed.ts          # 辅种 API
    └── stats.ts           # 统计 API
```

---

## 五、数据模型

### 5.1 数据库表结构

```sql
-- 下载器配置
CREATE TABLE clients (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    url TEXT NOT NULL,
    username TEXT,
    password TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 站点配置
CREATE TABLE sites (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    template_type TEXT NOT NULL,
    tracker_domains TEXT NOT NULL,
    download_pattern TEXT NOT NULL,
    passkey TEXT,
    cookie TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 种子索引
CREATE TABLE torrent_index (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    info_hash TEXT NOT NULL,
    site_id TEXT NOT NULL,
    torrent_id TEXT,
    name TEXT,
    size INTEGER NOT NULL,
    file_count INTEGER NOT NULL,
    largest_file_size INTEGER NOT NULL,
    files_hash TEXT,
    save_path TEXT,
    indexed_at INTEGER NOT NULL,
    UNIQUE(info_hash, site_id)
);

-- 辅种历史
CREATE TABLE reseed_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT,
    info_hash TEXT NOT NULL,
    source_site TEXT,
    target_site TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    created_at INTEGER NOT NULL
);
```

### 5.2 索引优化

```sql
-- 加速按站点查询
CREATE INDEX idx_torrent_index_site ON torrent_index(site_id);

-- 加速按大小查询 (指纹匹配)
CREATE INDEX idx_torrent_index_size ON torrent_index(size);

-- 加速历史记录查询
CREATE INDEX idx_reseed_history_created ON reseed_history(created_at DESC);
CREATE INDEX idx_reseed_history_task ON reseed_history(task_id);
```

---

## 六、性能优化

### 6.1 编译优化

```toml
[profile.release]
opt-level = 3           # 最高优化级别
lto = true             # 链接时优化
codegen-units = 1       # 单个代码生成单元，更好的优化
strip = true            # 移除调试符号
```

### 6.2 前端资源嵌入

```rust
use rust_embed::RustEmbed;

#[derive(RustEmbed)]
#[folder = "web/dist"]
struct Assets;

// 在 Axum 路由中使用
async fn serve_assets(path: &str) -> Response {
    let path = path.trim_start_matches('/');
    
    if let Some(content) = Assets::get(path) {
        let mime = mime_guess::from_path(path)
            .first_or_octet_stream()
            .to_string();
        
        return Response::builder()
            .header("content-type", mime)
            .body(Body::from(content.data.to_vec()))
            .unwrap();
    }
    
    Response::builder()
        .status(StatusCode::NOT_FOUND)
        .body(Body::empty())
        .unwrap()
}
```

### 6.3 异步处理

- 全面使用 Tokio 异步运行时
- HTTP 请求使用 reqwest 异步客户端
- 数据库操作使用 rusqlite 异步包装

### 6.4 算法优化

**指纹匹配**：
- 使用 HashMap 按 total_size 索引，O(1) 查找
- 结果按置信度排序，优先处理高置信度匹配

**站点识别**：
- 域名映射使用 HashMap，快速查找
- 支持多级域名匹配，提高识别率

---

## 七、部署方案

### 7.1 Docker 部署

**Dockerfile**:

```dockerfile
FROM rust:1.75-alpine AS builder

WORKDIR /app
COPY . .

RUN apk add --no-cache musl-dev nodejs npm
RUN cd web && npm install && npm run build
RUN cargo build --release

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/target/release/graft .
COPY --from=builder /app/migrations ./migrations

EXPOSE 3000

CMD ["./graft"]
```

**docker-compose.yml**:

```yaml
version: '3.8'

services:
  graft:
    build: .
    container_name: graft
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Shanghai
      - RUST_LOG=graft=info
      - GRAFT_HOST=0.0.0.0
      - GRAFT_PORT=3000
```

### 7.2 单二进制部署

```bash
# 下载最新版本
wget https://github.com/lynthar/graft/releases/latest/download/graft-linux-amd64.tar.gz
tar -xzf graft-linux-amd64.tar.gz

# 创建配置
cp config.example.toml config.toml

# 运行
./graft
```

---

## 八、安全与隐私

### 8.1 隐私保护

1. **完全本地化**: 所有数据处理在本地完成
2. **无云依赖**: 不依赖任何外部云服务
3. **无用户追踪**: 无需注册、无需登录
4. **数据不出本地**: 种子数据、用户配置都存储在本地

### 8.2 安全措施

1. **密码存储**: 如果存储密码，应使用加密
2. **HTTPS**: 生产环境建议使用反向代理（Nginx/Caddy）提供 HTTPS
3. **CORS**: 通过 tower-http 配置 CORS 策略
4. **输入验证**: 所有用户输入都进行验证和清理

---

## 九、技术亮点总结

### 9.1 架构设计

1. **分层架构**: API 层、服务层、数据层清晰分离
2. **依赖注入**: 使用 Arc 和 trait 实现依赖注入
3. **错误处理**: 使用 anyhow 统一错误处理
4. **日志系统**: 使用 tracing 结构化日志

### 9.2 性能优化

1. **单二进制**: 无运行时依赖，启动快
2. **内存安全**: Rust 的所有权系统保证内存安全
3. **零成本抽象**: 编译时优化，运行时无额外开销
4. **细粒度响应式**: SolidJS 的响应式系统比 Virtual DOM 更高效

### 9.3 可维护性

1. **模块化设计**: 清晰的模块划分，职责明确
2. **类型安全**: Rust 的类型系统在编译时捕获错误
3. **测试友好**: 模块化设计便于单元测试
4. **文档完善**: 代码注释和文档齐全

### 9.4 扩展性

1. **插件式站点模板**: 易于添加新的站点类型
2. **下载器抽象**: 支持多种下载器，易于扩展
3. **配置驱动**: 大部分行为通过配置控制
4. **API 设计**: RESTful API，便于集成

---

## 十、开发建议

### 10.1 潜在改进方向

1. **更多站点模板**: 支持 Unit3D、Gazelle 等更多站点类型
2. **实时推送**: 使用 WebSocket 实时推送辅种进度
3. **社区配置**: 支持从社区仓库订阅站点配置
4. **自动化任务**: 更强大的定时任务调度系统
5. **统计分析**: 详细的辅种效果统计和分析

### 10.2 学习价值

Graft 项目展示了：

1. **Rust 在系统工具开发中的优势**: 性能、安全性、单二进制分发
2. **前后端分离的最佳实践**: API 设计、类型安全、资源嵌入
3. **领域驱动设计**: 清晰的模块划分和职责分离
4. **性能优化的实践**: 算法优化、编译优化、异步处理

---

## 附录

### A. 支持的站点

**NexusPHP 站点**:
- M-Team (m-team.cc)
- HDSky (hdsky.me)
- OurBits (ourbits.club)
- PTer (pterclub.com)
- HDHome (hdhome.org)
- CHDBits (chdbits.co)
- TTG (totheglory.im)
- SSD (springsunday.net)
- 更多...

**Unit3D 站点**:
- Blutopia (blutopia.cc)
- Aither (aither.cc)
- Reelflix (reelflix.xyz)

**Gazelle 站点**:
- Redacted (redacted.ch)
- Orpheus (orpheus.network)

### B. 环境变量

```bash
# 服务器配置
GRAFT_HOST=0.0.0.0
GRAFT_PORT=3000

# 数据库
GRAFT_DB_PATH=./data/graft.db

# 日志
RUST_LOG=graft=info

# 时区
TZ=Asia/Shanghai
```

### C. 配置文件示例

```toml
[server]
host = "0.0.0.0"
port = 3000

[database]
path = "./data/graft.db"

[reseed]
default_paused = false
request_interval_ms = 500
max_per_run = 100
```

---

*分析完成于 2026-04-11*
