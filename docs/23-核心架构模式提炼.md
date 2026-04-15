# Examples 项目核心架构深度研究

本文档深入分析 examples 目录下项目的核心架构设计和实现细节。

---

## 1. Graft - 内容指纹辅种系统

### 1.1 核心设计理念

Graft 采用**本地内容指纹匹配**实现跨站辅种，与 IYUU 的云端 Hash 匹配形成鲜明对比：

| 对比项 | IYUU | Graft |
|--------|------|-------|
| 匹配方式 | 云端 info_hash API | 本地内容指纹 |
| 数据来源 | 云端维护 | 从下载器提取 |
| 隐私性 | Hash 上传云端 | 数据完全本地 |
| 依赖 | 需要网络 API | 无外部依赖 |

### 1.2 内容指纹算法

**文件**: [fingerprint.rs](file:///home/incast/PT-Forward/examples/Graft/src/service/fingerprint.rs)

```rust
pub struct ContentFingerprint {
    pub total_size: u64,        // 总大小 (主匹配键)
    pub file_count: usize,      // 文件数量
    pub largest_file_size: u64, // 最大文件大小
    pub files_hash: Option<String>, // 文件列表哈希 (精确匹配)
}
```

**分层匹配策略**:

```
┌─────────────────────────────────────────────────────────────┐
│                    匹配流程                                   │
├─────────────────────────────────────────────────────────────┤
│  1. total_size 必须精确匹配 (主键)                            │
│                    ↓                                         │
│  2. files_hash 可用? ──→ 是 ──→ 精确匹配 (ExactMatch)         │
│                    ↓ 否                                      │
│  3. largest_file_size 匹配? ──→ 否 ──→ 低置信度               │
│                    ↓ 是                                      │
│  4. file_count 差异 ≤ 2? ──→ 否 ──→ 中置信度                  │
│                    ↓ 是                                      │
│              高置信度匹配                                      │
└─────────────────────────────────────────────────────────────┘
```

**置信度评分**:

| 匹配结果 | 置信度 | 条件 |
|---------|--------|------|
| ExactMatch | 1.0 | files_hash 完全匹配 |
| HighConfidence | 0.9 | 大小+文件数+最大文件完全匹配 |
| MediumConfidence | 0.7 | 大小+最大文件匹配，文件数差≤2 |
| LowConfidence | 0.3 | 仅大小匹配 |
| NoMatch | 0.0 | 大小不匹配 |

### 1.3 辅种服务流程

**文件**: [reseed.rs](file:///home/incast/PT-Forward/examples/Graft/src/service/reseed.rs)

```rust
pub async fn execute(&self, request: ReseedRequest) -> Result<ReseedResult> {
    // 1. 获取预览结果
    let preview = self.preview(source_client, sites).await?;
    
    // 2. 获取目标客户端已有种子 (避免重复)
    let existing_hashes: HashSet<String> = target_client
        .get_torrents().await?
        .into_iter().map(|t| t.hash.to_lowercase()).collect();
    
    // 3. 遍历匹配项
    for m in preview.matches {
        // 检查是否已存在
        if existing_hashes.contains(&m.target_hash.to_lowercase()) {
            result.skipped += 1;
            continue;
        }
        
        // 下载种子文件
        let torrent_bytes = template.download_torrent(&http_client, &torrent_id).await?;
        
        // 添加到目标客户端
        let options = AddTorrentOptions {
            save_path: Some(m.save_path.clone()),
            paused: request.add_paused,
            skip_checking: request.skip_checking,
        };
        target_client.add_torrent(&torrent_bytes, options).await?;
    }
}
```

### 1.4 站点模板系统

**文件**: [nexusphp.rs](file:///home/incast/PT-Forward/examples/Graft/src/site/templates/nexusphp.rs)

```rust
impl SiteTemplate for NexusPHPTemplate {
    fn build_download_url(&self, torrent_id: &str) -> Result<String> {
        let passkey = self.config.passkey.as_ref()
            .ok_or(TemplateError::MissingPasskey)?;
        
        // 构建下载链接: {base_url}/download.php?id={id}&passkey={passkey}
        Ok(format!("{}{}&passkey={}", 
            self.config.base_url,
            self.config.download_pattern.replace("{id}", torrent_id),
            passkey))
    }
    
    async fn download_torrent(&self, http_client: &reqwest::Client, torrent_id: &str) 
        -> Result<Vec<u8>> {
        let url = self.build_download_url(torrent_id)?;
        let response = http_client.get(&url)
            .header("Cookie", &self.config.cookie)
            .send().await?;
        
        // 验证是否为有效种子文件 (以 'd' 开头)
        let bytes = response.bytes().await?;
        if bytes.first() != Some(&b'd') {
            return Err(TemplateError::InvalidResponse("Invalid torrent file".into()));
        }
        Ok(bytes.to_vec())
    }
}
```

### 1.5 qBittorrent 客户端实现

**文件**: [qbittorrent.rs](file:///home/incast/PT-Forward/examples/Graft/src/client/qbittorrent.rs)

```rust
impl BitTorrentClient for QBittorrentClient {
    async fn get_torrents(&self) -> Result<Vec<TorrentInfo>> {
        self.ensure_logged_in().await?;
        let url = self.api_url("/torrents/info");
        let torrents: Vec<QBTorrent> = self.http.get(&url).send().await?.json().await?;
        Ok(torrents.into_iter().map(|t| t.into()).collect())
    }
    
    async fn add_torrent(&self, torrent_bytes: &[u8], options: AddTorrentOptions) -> Result<String> {
        let file_part = multipart::Part::bytes(torrent_bytes.to_vec())
            .file_name("torrent.torrent")
            .mime_str("application/x-bittorrent")?;
        
        let mut form = multipart::Form::new().part("torrents", file_part);
        if let Some(path) = options.save_path {
            form = form.text("savepath", path);
        }
        if options.skip_checking {
            form = form.text("skip_checking", "true");
        }
        
        self.http.post(&self.api_url("/torrents/add"))
            .multipart(form).send().await?;
        Ok(String::new())
    }
}
```

---

## 2. pt-tools - 站点适配器架构

### 2.1 工厂模式设计

**文件**: [factory.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/factory.go)

```go
type SiteFactory struct {
    logger *zap.Logger
}

var typeToSchemaMap = map[string]Schema{
    "nexusphp": SchemaNexusPHP,
    "mtorrent": SchemaMTorrent,
    "unit3d":   SchemaUnit3D,
    "gazelle":  SchemaGazelle,
    "hddolby":  SchemaHDDolby,
    "rousi":    SchemaRousi,
}

func (f *SiteFactory) CreateSite(config SiteConfig) (Site, error) {
    // 1. 从注册表获取站点定义
    siteDef := GetDefinitionRegistry().GetOrDefault(config.ID)
    
    // 2. 根据定义创建站点实例
    return CreateSiteFromDefinition(siteDef, config, f.logger)
}
```

### 2.2 站点选择器系统

**文件**: [nexusphp_driver.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/nexusphp_driver.go)

```go
type SiteSelectors struct {
    TableRows          string `json:"tableRows"`          // 种子列表行
    Title              string `json:"title"`              // 标题
    TitleLink          string `json:"titleLink"`          // 标题链接
    Size               string `json:"size"`               // 大小
    Seeders            string `json:"seeders"`            // 做种数
    Leechers           string `json:"leechers"`           // 下载数
    Snatched           string `json:"snatched"`           // 完成数
    DiscountIcon       string `json:"discountIcon"`       // 优惠图标
    DiscountEndTime    string `json:"discountEndTime"`    // 优惠结束时间
    DownloadLink       string `json:"downloadLink"`       // 下载链接
    Category           string `json:"category"`           // 分类
    UploadTime         string `json:"uploadTime"`         // 发布时间
    HRIcon             string `json:"hrIcon"`             // H&R图标
    Subtitle           string `json:"subtitle"`           // 副标题
    UserInfoUsername   string `json:"userInfoUsername"`   // 用户名
    UserInfoUploaded   string `json:"userInfoUploaded"`   // 上传量
    UserInfoDownloaded string `json:"userInfoDownloaded"` // 下载量
    UserInfoRatio      string `json:"userInfoRatio"`      // 分享率
    UserInfoBonus      string `json:"userInfoBonus"`      // 魔力值
}

func DefaultNexusPHPSelectors() SiteSelectors {
    return SiteSelectors{
        TableRows:       "table.torrents > tbody > tr:not(:first-child)",
        Title:           "td:nth-child(2) a[href*='details.php']",
        Size:            "td:nth-child(5)",
        Seeders:         "td:nth-child(6)",
        Leechers:        "td:nth-child(7)",
        Snatched:        "td:nth-child(8)",
        DiscountIcon:    "img.pro_free, img.pro_free2up, img.pro_50pctdown",
        DownloadLink:    "a[href*='download.php']",
        // ...
    }
}
```

### 2.3 BaseSite 泛型包装

**文件**: [base_site.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/base_site.go)

```go
type BaseSite[Req any, Res any] struct {
    id       string
    name     string
    kind     SiteKind
    driver   Driver[Req, Res]
    limiter  *rate.Limiter  // 速率限制
    logger   *zap.Logger
    creds    Credentials
    loggedIn bool
    mu       sync.RWMutex
}

func (b *BaseSite[Req, Res]) Search(ctx context.Context, query SearchQuery) ([]TorrentItem, error) {
    // 1. 验证查询
    if err := query.Validate(); err != nil {
        return nil, fmt.Errorf("invalid query: %w", err)
    }
    
    // 2. 速率限制
    if err := b.limiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit: %w", err)
    }
    
    // 3. 准备请求
    req, err := b.driver.PrepareSearch(query)
    
    // 4. 执行请求
    res, err := b.driver.Execute(ctx, req)
    
    // 5. 解析响应
    items, err := b.driver.ParseSearch(res)
    
    return items, nil
}
```

### 2.4 登录状态检测

```go
func isLoginPage(doc *goquery.Document) bool {
    // 1. 检查登录表单
    if doc.Find("form[action*='takelogin']").Length() > 0 {
        return true
    }
    
    // 2. 检查登录面板
    if doc.Find(".login-panel").Length() > 0 {
        return true
    }
    
    // 3. 检查标题
    title := strings.ToLower(doc.Find("title").Text())
    if strings.Contains(title, "登录") || strings.Contains(title, "login") {
        if doc.Find("input[name='username']").Length() > 0 {
            return true
        }
    }
    
    return false
}

func is2FAPage(doc *goquery.Document) bool {
    scripts := doc.Find("script").Text()
    return strings.Contains(scripts, "take2fa.php") || 
           strings.Contains(scripts, "2fa")
}
```

---

## 3. harvest_rust - 现代异步架构

### 3.1 Actix-web 服务架构

**文件**: [main.rs](file:///home/incast/PT-Forward/examples/harvest_rust/src/main.rs)

```rust
#[actix_web::main]
async fn main() -> std::io::Result<()> {
    // 1. 初始化日志
    tracing_subscriber::registry()
        .with(tracing_subscriber::EnvFilter::from_default_env())
        .with(tracing_subscriber::fmt::layer())
        .init();
    
    // 2. 数据库连接
    let db_pool = setup_database(&config.database_url).await;
    
    // 3. Redis 连接
    let redis_pool = redis::Client::open(config.redis_url.clone())
        .get_connection_manager().await;
    
    // 4. 任务调度器
    let scheduler = setup_scheduler(db_pool.clone(), redis_pool.clone()).await;
    scheduler.start().await;
    
    // 5. HTTP 服务
    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::new(db_pool.clone()))
            .app_data(web::Data::new(redis_pool.clone()))
            .service(
                web::scope("/api/v1")
                    .service(web::scope("/auth").route("/login", web::post().to(auth::login)))
                    .wrap(AuthMiddleware)
                    .service(web::scope("/mysite").route("/{id}/sign", web::post().to(mysite::sign_in)))
            )
    }).bind((host, port))?.run().await
}
```

### 3.2 Spider Trait 设计

**文件**: [spider.rs](file:///home/incast/PT-Forward/examples/harvest_rust/src/services/spider.rs)

```rust
#[async_trait]
pub trait Spider: Send + Sync {
    fn name(&self) -> &str;
    
    async fn sign_in(&self, cookie: &str, user_agent: &str) 
        -> Result<SignInResult, SpiderError>;
    
    async fn get_user_info(&self, cookie: &str, user_agent: &str) 
        -> Result<SiteStatus, SpiderError>;
    
    async fn search(&self, cookie: &str, user_agent: &str, keyword: &str) 
        -> Result<Vec<TorrentInfo>, SpiderError>;
    
    async fn get_free_torrents(&self, cookie: &str, user_agent: &str) 
        -> Result<Vec<TorrentInfo>, SpiderError>;
}

pub enum SpiderError {
    Network(reqwest::Error),
    Parse(String),
    Auth(String),
    RateLimited,
    Unknown(String),
}
```

### 3.3 SeaORM 实体定义

**文件**: [my_site.rs](file:///home/incast/PT-Forward/examples/harvest_rust/src/db/entities/my_site.rs)

```rust
#[derive(Clone, Debug, DeriveEntityModel, Serialize, Deserialize)]
#[sea_orm(table_name = "my_sites")]
pub struct Model {
    #[sea_orm(primary_key)]
    pub id: i32,
    pub site: String,
    pub nickname: String,
    pub cookie: Option<String>,
    pub passkey: Option<String>,
    pub rss: Option<String>,
    pub available: bool,
    pub sign_in: bool,
    pub get_info: bool,
    pub brush_free: bool,
    pub brush_rss: bool,
    pub hr_discern: bool,
    pub search_torrents: bool,
    pub status: Option<Json>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

impl Model {
    pub fn has_today_sign(&self) -> bool {
        if let Some(sign_info) = &self.sign_info {
            let today = chrono::Utc::now().format("%Y-%m-%d").to_string();
            return sign_info.as_object().map_or(false, |o| o.contains_key(&today));
        }
        false
    }
}
```

---

## 4. torrentbotx - 下载器适配模式

### 4.1 注册器模式

**文件**: [base.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/downloaders/base.py)

```python
_downloader_registry: Dict[DownloaderType, Type["BaseDownloader"]] = {}

def register_downloader(downloader_type: DownloaderType):
    def decorator(cls):
        _downloader_registry[downloader_type] = cls
        return cls
    return decorator

def get_downloader_instance(downloader_type: DownloaderType):
    cls = _downloader_registry.get(downloader_type)
    if not cls:
        raise ValueError(f"未注册下载器类型：{downloader_type}")
    return cls()

class BaseDownloader(ABC):
    @abstractmethod
    def add_torrent(self, torrent_url: str) -> bool: pass
    
    @abstractmethod
    def get_torrents(self) -> list: pass
    
    @abstractmethod
    def pause_torrent(self, torrent_id: str) -> bool: pass
    
    @abstractmethod
    def resume_torrent(self, torrent_id: str) -> bool: pass
```

### 4.2 qBittorrent 实现

**文件**: [qbittorrent.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/downloaders/qbittorrent.py)

```python
@register_downloader(DownloaderType.QBITTORRENT)
class QBittorrentDownloader(BaseDownloader):
    def __init__(self):
        self.config = load_config()
        self.client = None
        self.connect()
    
    def connect(self):
        self.client = qbittorrentapi.Client(
            host=self.config.get("QBIT_HOST", "localhost"),
            port=self.config.get("QBIT_PORT", 8080),
            username=self.config.get("QBIT_USERNAME", "admin"),
            password=self.config.get("QBIT_PASSWORD", "adminadmin")
        )
        self.client.auth_log_in()
    
    def add_torrent(self, torrent_url: str) -> bool:
        try:
            self.client.torrents_add(urls=torrent_url)
            return True
        except Exception as e:
            log.error(f"添加种子失败: {e}")
            return False
```

### 4.3 CoreManager 统一管理

**文件**: [manager.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/core/manager.py)

```python
class CoreManager:
    def __init__(self, config=None, notifier: Optional[Notifier] = None):
        self.config = config or load_config()
        self.notifier = notifier or TelegramNotifier(
            bot_token=self.config.get("TG_BOT_TOKEN"),
            chat_id=self.config.get("TG_ALLOWED_CHAT_IDS")
        )
        self.downloaders = self._init_downloaders()
    
    def _init_downloaders(self) -> List:
        types = self.config.get("DOWNLOADERS", "qbittorrent")
        downloader_list = []
        for name in types.split(","):
            dtype = DownloaderType.from_name(name.strip())
            instance = get_downloader_instance(dtype)
            downloader_list.append(instance)
        return downloader_list
    
    def execute_download_task(self, params: dict):
        torrent_id = params.get("torrent_id")
        success_list = []
        for downloader in self.downloaders:
            success = downloader.add_torrent(torrent_id)
            success_list.append(success)
        
        if any(success_list):
            self.notifier.send_message(f"下载任务已添加: {torrent_id}")
            return True
        return False
```

---

## 5. 架构模式对比

### 5.1 站点适配模式

| 项目 | 模式 | 特点 |
|------|------|------|
| **Graft** | Trait + 模板 | Rust async trait，编译时多态 |
| **pt-tools** | 工厂 + 泛型 | Go 接口 + CSS选择器配置化 |
| **harvest_rust** | Trait + SeaORM | Rust async trait + ORM实体 |
| **torrentbotx** | 注册器 + ABC | Python 装饰器注册 |

### 5.2 下载器适配模式

| 项目 | 模式 | 支持客户端 |
|------|------|-----------|
| **Graft** | async trait | qBittorrent, Transmission |
| **pt-tools** | 接口 | qBittorrent, Transmission |
| **torrentbotx** | ABC + 注册器 | qBittorrent, aria2, Transmission |

### 5.3 数据存储

| 项目 | 数据库 | ORM |
|------|--------|-----|
| **Graft** | SQLite | rusqlite |
| **pt-tools** | SQLite | GORM |
| **harvest_rust** | PostgreSQL/SQLite | SeaORM |
| **torrentbotx** | SQLite | SQLAlchemy |

---

## 6. 关键设计模式总结

### 6.1 工厂模式

用于创建不同类型的站点/下载器实例：

```
SiteConfig → SiteFactory.CreateSite() → Site实例
```

### 6.2 策略模式

不同站点使用不同的解析策略：

```
NexusPHPDriver → ParseSearch() → 使用CSS选择器解析
Unit3DDriver   → ParseSearch() → 使用API解析
```

### 6.3 模板方法模式

BaseSite 提供通用流程，子类实现具体细节：

```
BaseSite.Search() {
    1. Validate()    // 通用
    2. RateLimit()   // 通用
    3. PrepareSearch()  // 子类实现
    4. Execute()     // 子类实现
    5. ParseSearch() // 子类实现
}
```

### 6.4 注册器模式

动态注册和获取实例：

```python
@register_downloader(DownloaderType.QBITTORRENT)
class QBittorrentDownloader(BaseDownloader): ...

# 使用
downloader = get_downloader_instance(DownloaderType.QBITTORRENT)
```

---

## 7. PT-Forward 设计建议

基于以上分析，PT-Forward 可采用以下架构：

### 7.1 站点适配器

- 采用 **工厂 + 泛型** 模式 (参考 pt-tools)
- CSS 选择器配置化，支持自定义站点
- 内置 NexusPHP、Unit3D、Gazelle 模板

### 7.2 下载器适配器

- 采用 **注册器 + 接口** 模式 (参考 torrentbotx)
- 支持 qBittorrent、Transmission、aria2
- 统一的添加/暂停/删除接口

### 7.3 辅种引擎

- 采用 **内容指纹** 匹配 (参考 Graft)
- 本地索引，隐私优先
- 分层置信度评分

### 7.4 数据存储

- SQLite 轻量级存储
- GORM 或原生 SQL
- 支持迁移和版本管理
