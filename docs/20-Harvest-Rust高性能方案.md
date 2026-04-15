# Harvest-RS 项目深度分析

## 一、项目概述

**Harvest-RS** 是一个 PT 站点管理工具，使用 Rust + Actix-web + SeaORM 重新实现，旨在提供一站式的 PT 站点管理解决方案。

### 1.1 核心功能

| 功能模块 | 说明 |
|---------|------|
| **站点管理** | 管理多个 PT 站点的账号、cookie、passkey |
| **自动签到** | 定时自动签到，保持账号活跃 |
| **数据抓取** | 抓取站点用户数据（上传量、下载量、分享率等） |
| **刷流功能** | Free 刷流、RSS 刷流 |
| **辅种支持** | 跨站点辅种 |
| **种子搜索** | 跨站点搜索种子 |
| **实时通信** | WebSocket 实时推送 |
| **定时任务** | 异步任务调度 |

### 1.2 技术栈

```
┌────────────────────────────────────────────────────────┐
│                    Harvest-RS                          │
├────────────────────────────────────────────────────────┤
│  Web Framework  │  Actix-web 4.x                       │
├─────────────────┼──────────────────────────────────────┤
│  ORM            │  SeaORM 1.x                          │
├─────────────────┼──────────────────────────────────────┤
│  Database       │  PostgreSQL / SQLite                 │
├─────────────────┼──────────────────────────────────────┤
│  Cache          │  Redis                               │
├─────────────────┼──────────────────────────────────────┤
│  Task Scheduler │  tokio-cron-scheduler                │
├─────────────────┼──────────────────────────────────────┤
│  Async Runtime  │  Tokio                               │
└─────────────────┴──────────────────────────────────────┘
```

---

## 二、技术架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        Harvest-RS                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │  Web Client  │    │   WebSocket  │    │   Browser    │      │
│  │  (REST API)  │◄──►│   (Real-time)│◄──►│  Extension   │      │
│  └──────────────┘    └──────┬───────┘    └──────────────┘      │
│                             │                                   │
│         ┌───────────────────┼───────────────────┐               │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌────────────┐     ┌─────────────┐     ┌─────────────┐        │
│  │  Handlers  │     │  Middleware │     │    Tasks    │        │
│  │  (API层)   │     │  (认证/日志) │     │  (定时任务)  │        │
│  └─────┬──────┘     └──────┬──────┘     └──────┬──────┘        │
│        │                   │                   │                │
│        │           ┌───────▼───────┐           │                │
│        │           │   Services   │           │                │
│        └──────────►│  (业务逻辑)  │◄──────────┘                │
│                    └───────┬───────┘                            │
│                            │                                    │
│         ┌───────────────────┼───────────────────┐               │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌────────────┐     ┌─────────────┐     ┌─────────────┐        │
│  │  SeaORM    │     │   Redis     │     │  Spiders    │        │
│  │  (数据库)   │     │  (缓存/队列) │     │ (站点爬虫)  │        │
│  └────────────┘     └─────────────┘     └─────────────┘        │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                      External Systems                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ qBittorrent  │    │  PT Sites    │    │    TMDB      │      │
│  │  (下载器)    │    │  (种子源)    │    │  (电影元数据)│      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
harvest-rs/
├── src/
│   ├── config/              # 配置模块
│   │   └── mod.rs          # 环境变量配置
│   │
│   ├── db/                  # 数据库模块
│   │   ├── entities/        # SeaORM 实体定义
│   │   │   ├── mod.rs      # 实体导出
│   │   │   ├── user.rs     # 用户实体
│   │   │   ├── my_site.rs  # 站点实体
│   │   │   ├── downloader.rs # 下载器实体
│   │   │   ├── scheduled_task.rs # 定时任务实体
│   │   │   └── monkey_token.rs # 浏览器扩展token
│   │   ├── migration.rs    # 数据库迁移
│   │   └── mod.rs          # 数据库连接
│   │
│   ├── handlers/            # HTTP 处理器
│   │   ├── mod.rs          # 处理器导出
│   │   ├── auth.rs         # 认证相关
│   │   ├── mysite.rs       # 站点管理
│   │   ├── option.rs       # 下载器/任务管理
│   │   ├── source.rs       # RSS/种子处理
│   │   ├── tmdb.rs         # TMDB API
│   │   └── ws.rs           # WebSocket
│   │
│   ├── middleware/          # 中间件
│   │   ├── mod.rs          # 中间件导出
│   │   └── auth.rs         # 认证中间件
│   │
│   ├── models/              # 数据模型
│   │   ├── mod.rs          # 模型导出
│   │   ├── response.rs     # 统一响应格式
│   │   └── torrent.rs      # 种子相关模型
│   │
│   ├── services/            # 业务逻辑
│   │   ├── mod.rs          # 服务导出
│   │   ├── spider.rs       # 站点爬虫
│   │   ├── downloader.rs   # 下载器客户端
│   │   └── rss.rs          # RSS 解析
│   │
│   ├── tasks/               # 定时任务
│   │   ├── mod.rs          # 任务导出
│   │   └── scheduler.rs    # 任务调度器
│   │
│   ├── utils/               # 工具函数
│   │   ├── mod.rs          # 工具导出
│   │   ├── crypto.rs       # 加密工具
│   │   └── http.rs         # HTTP 工具
│   │
│   └── main.rs              # 入口文件
│
├── Cargo.toml
├── .env.example
└── README.md
```

---

## 三、核心模块分析

### 3.1 认证系统 (`src/handlers/auth.rs`)

#### 3.1.1 JWT 认证流程

```rust
/// 用户登录
pub async fn login(
    db: web::Data<DatabaseConnection>,
    config: web::Data<AppConfig>,
    body: web::Json<LoginRequest>,
) -> HttpResponse {
    // 1. 查询用户
    let user = User::find()
        .filter(user::Column::Username.eq(&body.username))
        .one(db.get_ref())
        .await;

    // 2. 验证密码
    if verify(&body.password, &user.password_hash).unwrap_or(false) {
        // 3. 生成 JWT token
        let claims = Claims {
            sub: user.id,
            username: user.username,
            exp: exp.timestamp(),
            iat: now.timestamp(),
        };

        let token = encode(
            &Header::default(),
            &claims,
            &EncodingKey::from_secret(config.secret_key.as_bytes()),
        );

        // 4. 返回 token
        HttpResponse::Ok().json(ApiResponse::success(serde_json::json!({
            "token": token,
            "expires_at": exp.to_rfc3339(),
            "user": { "id": user.id, "username": user.username }
        })))
    }
}
```

#### 3.1.2 认证中间件

```rust
pub struct AuthMiddleware;

impl<S> Transform<S, ServiceRequest> for AuthMiddleware
where
    S: Service<ServiceRequest, Response = ServiceResponse, Error = Error>,
{
    type Response = ServiceResponse;
    type Transform = AuthMiddlewareService<S>;

    fn new_transform(&self, service: S) -> Self::Future {
        ready(Ok(AuthMiddlewareService { service }))
    }
}

impl<S> Service<ServiceRequest> for AuthMiddlewareService<S> {
    fn call(&self, mut req: ServiceRequest) -> Self::Future {
        // 1. 从 Authorization header 获取 token
        let auth_header = req.headers().get("Authorization");
        
        if let Some(auth_header) = auth_header {
            if auth_str.starts_with("Bearer ") {
                let token = &auth_str[7..];
                
                // 2. 验证 token
                match decode::<Claims>(token, &DecodingKey::from_secret(...), &Validation::new(Algorithm::HS256)) {
                    Ok(token_data) => {
                        // 3. 将 claims 添加到 request extensions
                        req.extensions_mut().insert(token_data.claims);
                        let fut = self.service.call(req);
                        return Box::pin(async move { fut.await });
                    }
                    Err(e) => {
                        // 4. 返回 401
                        return Box::pin(async move {
                            Ok(req.into_response(HttpResponse::Unauthorized()
                                .json(serde_json::json!({ "code": 401, "msg": "Invalid token" }))))
                        });
                    }
                }
            }
        }

        // 5. 支持 Monkey token（浏览器扩展）
        if auth_str.starts_with("Monkey.") {
            let token = &auth_str[7..];
            // TODO: 验证 monkey token
        }

        // 6. 无 token，返回 401
        Box::pin(async move {
            Ok(req.into_response(HttpResponse::Unauthorized()
                .json(serde_json::json!({ "code": 401, "msg": "Missing Authorization header" }))))
        })
    }
}
```

**特点**：
- 支持 JWT Bearer token
- 支持 Monkey token（浏览器扩展）
- Token 验证失败返回 401
- Claims 存储在 request extensions 中供后续使用

---

### 3.2 站点管理 (`src/handlers/mysite.rs`)

#### 3.2.1 站点数据模型

```rust
#[derive(Clone, Debug, PartialEq, DeriveEntityModel, Serialize, Deserialize)]
#[sea_orm(table_name = "my_sites")]
pub struct Model {
    #[sea_orm(primary_key)]
    pub id: i32,
    pub site: String,              // 站点标识
    pub nickname: String,          // 站点昵称
    pub sort_id: i32,              // 排序ID
    pub tags: Option<Json>,        // 标签
    pub user_id: Option<String>,   // 用户ID
    pub username: Option<String>,  // 用户名
    pub email: Option<String>,     // 邮箱
    pub passkey: Option<String>,   // Passkey
    pub authkey: Option<String>,   // Authkey
    pub cookie: Option<String>,    // Cookie
    pub user_agent: Option<String>, // User-Agent
    pub rss: Option<String>,       // RSS URL
    pub torrents: Option<String>,  // 种子页面URL
    pub available: bool,           // 是否可用
    pub sign_in: bool,             // 是否签到
    pub get_info: bool,            // 是否获取信息
    pub repeat_torrents: bool,     // 是否重复种子
    pub brush_free: bool,          // 是否刷Free
    pub brush_rss: bool,           // 是否刷RSS
    pub package_file: bool,        // 是否打包文件
    pub hr_discern: bool,          // 是否HR识别
    pub search_torrents: bool,     // 是否搜索种子
    pub show_in_dash: bool,        // 是否在仪表板显示
    pub proxy: Option<String>,     // 代理
    pub remove_torrent_rules: Option<Json>, // 删除种子规则
    pub mirror: Option<String>,    // 镜像
    pub time_join: DateTime<Utc>,  // 加入时间
    pub latest_active: Option<DateTime<Utc>>, // 最后活跃时间
    pub mail: i32,                 // 邮件数
    pub notice: i32,               // 通知数
    pub sign_info: Option<Json>,   // 签到信息
    pub status: Option<Json>,      // 状态信息
    pub created_at: DateTime<Utc>, // 创建时间
    pub updated_at: DateTime<Utc>, // 更新时间
}
```

#### 3.2.2 站点辅助方法

```rust
impl Model {
    /// 检查今天是否已签到
    pub fn has_today_sign(&self) -> bool {
        if let Some(sign_info) = &self.sign_info {
            let today = chrono::Utc::now().format("%Y-%m-%d").to_string();
            if let Some(obj) = sign_info.as_object() {
                return obj.contains_key(&today);
            }
        }
        false
    }

    /// 检查今天是否有状态
    pub fn has_today_state(&self) -> bool {
        if let Some(status) = &self.status {
            let today = chrono::Utc::now().format("%Y-%m-%d").to_string();
            if let Some(obj) = status.as_object() {
                return obj.contains_key(&today);
            }
        }
        false
    }

    /// 获取最新状态
    pub fn latest_state(&self) -> Option<&serde_json::Value> {
        self.status.as_ref()?.as_object()?.iter().last().map(|(_, v)| v)
    }
}
```

**设计亮点**：
- 使用 JSON 字段存储灵活的签到信息和状态
- 按日期键值对存储，便于历史查询
- 提供便捷的辅助方法检查今日状态

#### 3.2.3 站点 CRUD 操作

```rust
/// 创建站点
pub async fn create_site(
    db: web::Data<DatabaseConnection>,
    body: web::Json<CreateSite>,
) -> HttpResponse {
    // 1. 检查站点是否已存在
    let existing = MySite::find()
        .filter(my_site::Column::Site.eq(&body.site))
        .one(db.get_ref())
        .await;

    match existing {
        Ok(Some(_)) => {
            HttpResponse::BadRequest().json(ApiResponse::<()>::err(400, "Site already exists"))
        }
        Ok(None) => {
            // 2. 创建新站点
            let new_site = my_site::ActiveModel {
                site: Set(body.site.clone()),
                nickname: Set(body.nickname.clone()),
                sort_id: Set(body.sort_id.unwrap_or(1)),
                tags: Set(body.tags.as_ref().map(|t| serde_json::to_value(t).unwrap())),
                // ... 设置其他字段
                ..Default::default()
            };

            match new_site.insert(db.get_ref()).await {
                Ok(site) => HttpResponse::Created().json(ApiResponse::success(site)),
                Err(e) => HttpResponse::InternalServerError().json(ApiResponse::<()>::err(500, &e.to_string())),
            }
        }
        Err(e) => HttpResponse::InternalServerError().json(ApiResponse::<()>::err(500, &e.to_string())),
    }
}
```

---

### 3.3 站点爬虫系统 (`src/services/spider.rs`)

#### 3.3.1 Spider Trait 定义

```rust
#[async_trait]
pub trait Spider: Send + Sync {
    /// 获取站点名称
    fn name(&self) -> &str;
    
    /// 签到
    async fn sign_in(&self, cookie: &str, user_agent: &str) -> Result<SignInResult, SpiderError>;
    
    /// 获取用户信息
    async fn get_user_info(&self, cookie: &str, user_agent: &str) -> Result<SiteStatus, SpiderError>;
    
    /// 搜索种子
    async fn search(&self, cookie: &str, user_agent: &str, keyword: &str) -> Result<Vec<TorrentInfo>, SpiderError>;
    
    /// 获取Free种子
    async fn get_free_torrents(&self, cookie: &str, user_agent: &str) -> Result<Vec<TorrentInfo>, SpiderError>;
}
```

#### 3.3.2 基础爬虫实现

```rust
pub struct BaseSpider {
    pub name: String,
    pub base_url: String,
    pub client: Client,
}

impl BaseSpider {
    pub fn new(name: &str, base_url: &str) -> Self {
        Self {
            name: name.to_string(),
            base_url: base_url.to_string(),
            client: Client::builder()
                .user_agent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
                .build()
                .unwrap(),
        }
    }

    /// 构建 URL
    pub fn build_url(&self, path: &str) -> String {
        format!("{}{}", self.base_url.trim_end_matches('/'), path)
    }

    /// 解析 HTML
    pub fn parse_html(&self, html: &str) -> Html {
        Html::parse_document(html)
    }

    /// 选择元素
    pub fn select<'a>(&self, html: &'a Html, selector: &str) -> scraper::Select<'a, 'a, ElementRef> {
        let selector = Selector::parse(selector).unwrap();
        html.select(&selector)
    }
}
```

#### 3.3.3 错误处理

```rust
#[derive(Debug, thiserror::Error)]
pub enum SpiderError {
    #[error("Network error: {0}")]
    Network(#[from] reqwest::Error),
    
    #[error("Parse error: {0}")]
    Parse(String),
    
    #[error("Authentication failed: {0}")]
    Auth(String),
    
    #[error("Rate limited")]
    RateLimited,
    
    #[error("Unknown error: {0}")]
    Unknown(String),
}
```

**设计优势**：
- Trait 抽象，便于扩展新站点
- 统一的错误处理
- 基础爬虫提供通用功能
- 支持异步操作

---

### 3.4 下载器客户端 (`src/services/downloader.rs`)

#### 3.4.1 下载器 Trait

```rust
#[async_trait]
pub trait DownloaderClient: Send + Sync {
    /// 获取 API 版本
    async fn get_version(&self) -> Result<String, DownloaderError>;
    
    /// 从 URL 添加种子
    async fn add_torrent_url(&self, url: &str, options: Option<TorrentAddOptions>) -> Result<String, DownloaderError>;
    
    /// 从文件添加种子
    async fn add_torrent_file(&self, data: &[u8], options: Option<TorrentAddOptions>) -> Result<String, DownloaderError>;
    
    /// 获取种子列表
    async fn get_torrents(&self) -> Result<Vec<TorrentInfo>, DownloaderError>;
    
    /// 获取种子信息
    async fn get_torrent(&self, hash: &str) -> Result<Option<TorrentInfo>, DownloaderError>;
    
    /// 删除种子
    async fn delete_torrent(&self, hash: &str, delete_files: bool) -> Result<(), DownloaderError>;
    
    /// 暂停种子
    async fn pause_torrent(&self, hash: &str) -> Result<(), DownloaderError>;
    
    /// 恢复种子
    async fn resume_torrent(&self, hash: &str) -> Result<(), DownloaderError>;
    
    /// 获取传输信息
    async fn get_transfer_info(&self) -> Result<TransferInfo, DownloaderError>;
}
```

#### 3.4.2 qBittorrent 实现

```rust
pub struct QbittorrentClient {
    client: reqwest::Client,
    url: String,
    username: String,
    password: String,
}

impl QbittorrentClient {
    pub fn new(url: &str, username: &str, password: &str) -> Self {
        Self {
            client: reqwest::Client::new(),
            url: url.trim_end_matches('/').to_string(),
            username: username.to_string(),
            password: password.to_string(),
        }
    }

    /// 登录获取 SID
    async fn login(&self) -> Result<String, DownloaderError> {
        let response = self.client
            .post(format!("{}/api/v2/auth/login", self.url))
            .form(&[
                ("username", self.username.as_str()),
                ("password", self.password.as_str()),
            ])
            .send()
            .await?;

        let sid = response.cookies()
            .find(|c| c.name() == "SID")
            .map(|c| c.value().to_string());

        sid.ok_or(DownloaderError::AuthFailed)
    }
}

#[async_trait]
impl DownloaderClient for QbittorrentClient {
    async fn add_torrent_url(&self, url: &str, options: Option<TorrentAddOptions>) -> Result<String, DownloaderError> {
        let sid = self.login().await?;
        let mut form = vec![("urls", url.to_string())];
        
        if let Some(opts) = options {
            if let Some(save_path) = opts.save_path {
                form.push(("savepath", save_path));
            }
            if let Some(category) = opts.category {
                form.push(("category", category));
            }
        }

        let response = self.client
            .post(format!("{}/api/v2/torrents/add", self.url))
            .header("Cookie", format!("SID={}", sid))
            .form(&form)
            .send()
            .await?;

        if response.status().is_success() {
            Ok("Torrent added".to_string())
        } else {
            Err(DownloaderError::Unknown(format!("Failed to add torrent")))
        }
    }

    async fn get_torrents(&self) -> Result<Vec<TorrentInfo>, DownloaderError> {
        let sid = self.login().await?;
        let response = self.client
            .get(format!("{}/api/v2/torrents/info", self.url))
            .header("Cookie", format!("SID={}", sid))
            .send()
            .await?;

        response.json().await.map_err(Into::into)
    }
}
```

**特点**：
- Trait 抽象，支持多种下载器
- 自动登录管理（SID cookie）
- 丰富的种子操作接口
- 支持传输信息查询

---

### 3.5 定时任务调度 (`src/tasks/scheduler.rs`)

#### 3.5.1 任务调度器

```rust
pub async fn setup_scheduler(
    db: DatabaseConnection,
    redis: ConnectionManager,
) -> JobScheduler {
    let scheduler = JobScheduler::new().await.expect("Failed to create scheduler");

    // 1. 签到任务（每天 8:00）
    let db_clone = db.clone();
    let job = Job::new_async("0 0 8 * * *", move |_uuid, _l| {
        let db = db_clone.clone();
        async move {
            tracing::info!("Running scheduled sign-in task");
            // TODO: 实现签到逻辑
            // 1. 获取所有 sign_in = true 的站点
            // 2. 对每个站点执行签到爬虫
            // 3. 更新数据库中的 sign_info
        }
    }).expect("Failed to create sign-in job");
    scheduler.add(job).await.expect("Failed to add sign-in job");

    // 2. 信息获取任务（每 6 小时）
    let db_clone = db.clone();
    let job = Job::new_async("0 0 */6 * * *", move |_uuid, _l| {
        let db = db_clone.clone();
        async move {
            tracing::info!("Running scheduled info fetch task");
            // TODO: 实现信息获取逻辑
        }
    }).expect("Failed to create info fetch job");
    scheduler.add(job).await.expect("Failed to add info fetch job");

    // 3. Free 刷流任务（每 30 分钟）
    let db_clone = db.clone();
    let job = Job::new_async("0 */30 * * * *", move |_uuid, _l| {
        let db = db_clone.clone();
        async move {
            tracing::info!("Running scheduled brush free task");
            // TODO: 实现 Free 刷流逻辑
        }
    }).expect("Failed to create brush free job");
    scheduler.add(job).await.expect("Failed to add brush free job");

    // 4. RSS 刷流任务（每 15 分钟）
    let db_clone = db.clone();
    let job = Job::new_async("0 */15 * * * *", move |_uuid, _l| {
        let db = db_clone.clone();
        async move {
            tracing::info!("Running scheduled RSS brush task");
            // TODO: 实现 RSS 刷流逻辑
        }
    }).expect("Failed to create RSS brush job");
    scheduler.add(job).await.expect("Failed to add RSS brush job");

    scheduler
}
```

**定时任务说明**：

| 任务 | Cron 表达式 | 频率 | 功能 |
|------|-------------|------|------|
| 签到任务 | `0 0 8 * * *` | 每天 8:00 | 自动签到 |
| 信息获取 | `0 0 */6 * * *` | 每 6 小时 | 获取用户统计信息 |
| Free 刷流 | `0 */30 * * * *` | 每 30 分钟 | 抓取并添加 Free 种子 |
| RSS 刷流 | `0 */15 * * * *` | 每 15 分钟 | 解析 RSS 并添加种子 |

---

### 3.6 WebSocket 实时通信 (`src/handlers/ws.rs`)

#### 3.6.1 WebSocket 路由

```rust
pub async fn ws_route(
    req: HttpRequest,
    body: web::Payload,
) -> Result<HttpResponse, actix_web::Error> {
    let mut session = Session::new(req, body)?;

    // 发送欢迎消息
    session.text("{\"type\":\"connected\",\"message\":\"WebSocket connected\"}").await?;

    // 处理传入消息
    while let Some(msg_result) = session.recv().await {
        match msg_result {
            Ok(msg) => {
                match msg {
                    Message::Text(text) => {
                        // 回显消息
                        session.text(format!("Received: {}", text)).await?;
                    }
                    Message::Binary(bytes) => {
                        // 处理二进制消息
                    }
                    Message::Close(reason) => {
                        // 关闭连接
                        break;
                    }
                    _ => {}
                }
            }
            Err(e) => {
                tracing::error!("WebSocket error: {}", e);
                break;
            }
        }
    }

    Ok(HttpResponse::Ok().finish())
}
```

#### 3.6.2 消息类型定义

```rust
/// WebSocket 消息类型
#[derive(Debug, serde::Deserialize)]
#[serde(tag = "type")]
enum WsMessage {
    #[serde(rename = "subscribe")]
    Subscribe { channel: String },
    #[serde(rename = "unsubscribe")]
    Unsubscribe { channel: String },
    #[serde(rename = "ping")]
    Ping,
}

/// WebSocket 响应类型
#[derive(Debug, serde::Serialize)]
#[serde(tag = "type")]
enum WsResponse {
    #[serde(rename = "pong")]
    Pong,
    #[serde(rename = "subscribed")]
    Subscribed { channel: String },
    #[serde(rename = "unsubscribed")]
    Unsubscribed { channel: String },
    #[serde(rename = "notification")]
    Notification { message: String },
    #[serde(rename = "task_update")]
    TaskUpdate { task_id: i32, status: String },
}
```

**应用场景**：
- 实时推送签到结果
- 实时推送任务执行状态
- 实时推送种子添加结果
- 实时推送站点状态更新

---

## 四、数据模型

### 4.1 数据库实体

#### 4.1.1 用户实体 (`user.rs`)

```rust
#[derive(Clone, Debug, PartialEq, DeriveEntityModel, Serialize, Deserialize)]
#[sea_orm(table_name = "users")]
pub struct Model {
    #[sea_orm(primary_key)]
    pub id: i32,
    pub username: String,
    pub email: Option<String>,
    pub password_hash: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Claims {
    pub sub: i32,        // 用户ID
    pub username: String,
    pub exp: i64,        // 过期时间
    pub iat: i64,        // 签发时间
}
```

#### 4.1.2 下载器实体 (`downloader.rs`)

```rust
#[derive(Clone, Debug, PartialEq, DereneEntityModel, Serialize, Deserialize)]
#[sea_orm(table_name = "downloaders")]
pub struct Model {
    #[sea_orm(primary_key)]
    pub id: i32,
    pub name: String,
    pub type: String,        // qBittorrent / Transmission
    pub url: String,
    pub username: String,
    pub password: String,
    pub enabled: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}
```

#### 4.1.3 定时任务实体 (`scheduled_task.rs`)

```rust
#[derive(Clone, Debug, PartialEq, DereneEntityModel, Serialize, Deserialize)]
#[sea_orm(table_name = "scheduled_tasks")]
pub struct Model {
    #[sea_orm(primary_key)]
    pub id: i32,
    pub name: String,
    pub cron_expression: String,
    pub task_type: String,    // sign_in / get_info / brush_free / brush_rss
    pub site_id: Option<i32>,
    pub enabled: bool,
    pub last_run: Option<DateTime<Utc>>,
    pub next_run: DateTime<Utc>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}
```

---

### 4.2 API 响应模型

```rust
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize)]
pub struct ApiResponse<T> {
    pub code: i32,
    pub msg: String,
    pub data: Option<T>,
}

impl<T> ApiResponse<T> {
    pub fn success(data: T) -> Self {
        Self {
            code: 200,
            msg: "success".to_string(),
            data: Some(data),
        }
    }

    pub fn ok(msg: &str) -> Self {
        Self {
            code: 200,
            msg: msg.to_string(),
            data: None,
        }
    }

    pub fn err(code: i32, msg: &str) -> Self {
        Self {
            code,
            msg: msg.to_string(),
            data: None,
        }
    }
}
```

---

## 五、配置管理

### 5.1 环境变量配置

```rust
#[derive(Debug, Clone, Deserialize)]
pub struct AppConfig {
    pub host: String,
    pub port: u16,
    pub database_url: String,
    pub redis_url: String,
    pub secret_key: String,
    pub jwt_expiration: i64,
    pub tmdb_api_key: Option<String>,
}

impl AppConfig {
    pub fn from_env() -> Self {
        dotenvy::dotenv().ok();

        Self {
            host: env::var("HOST").unwrap_or_else(|_| "0.0.0.0".to_string()),
            port: env::var("PORT")
                .unwrap_or_else(|_| "8000".to_string())
                .parse()
                .unwrap_or(8000),
            database_url: env::var("DATABASE_URL")
                .unwrap_or_else(|_| "sqlite://./data.db?mode=rwc".to_string()),
            redis_url: env::var("REDIS_URL")
                .unwrap_or_else(|_| "redis://127.0.0.1:6379/10".to_string()),
            secret_key: env::var("SECRET_KEY")
                .unwrap_or_else(|_| "your-secret-key-change-in-production".to_string()),
            jwt_expiration: env::var("JWT_EXPIRATION")
                .unwrap_or_else(|_| "86400".to_string())
                .parse()
                .unwrap_or(86400),
            tmdb_api_key: env::var("TMDB_API_KEY").ok(),
        }
    }
}
```

### 5.2 环境变量示例 (`.env.example`)

```bash
HOST=0.0.0.0
PORT=8000
DATABASE_URL=sqlite://./data.db?mode=rwc
REDIS_URL=redis://127.0.0.1:6379/10
SECRET_KEY=your-secret-key-change-in-production
JWT_EXPIRATION=86400
TMDB_API_KEY=your-tmdb-api-key
```

---

## 六、依赖分析

### 6.1 核心依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| actix-web | 4 | Web 框架 |
| sea-orm | 1 | ORM |
| tokio | 1 | 异步运行时 |
| reqwest | 0.12 | HTTP 客户端 |
| jsonwebtoken | 9 | JWT 认证 |
| bcrypt | 0.16 | 密码哈希 |
| tokio-cron-scheduler | 0.13 | 定时任务 |
| redis | 0.27 | Redis 客户端 |
| scraper | 0.20 | HTML 解析 |
| feed-rs | 2 | RSS 解析 |
| bendy | 0.3 | 种子文件解析 |

### 6.2 特性分析

```toml
[dependencies]
# Web framework with WebSocket support
actix-web = "4"
actix-ws = "0.3"

# Database with PostgreSQL and SQLite support
sea-orm = { version = "1", features = ["sqlx-postgres", "sqlx-sqlite", "runtime-tokio-rustls", "macros"] }

# HTTP client with cookies and TLS
reqwest = { version = "0.12", features = ["json", "cookies", "rustls-tls"] }

# Task scheduler
tokio-cron-scheduler = "0.13"

# Redis with async support
redis = { version = "0.27", features = ["tokio-comp", "connection-manager"] }
```

---

## 七、API 设计

### 7.1 认证 API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/auth/login` | 用户登录 | 否 |
| POST | `/api/v1/auth/register` | 用户注册 | 否 |
| POST | `/api/v1/auth/refresh` | 刷新令牌 | 是 |

### 7.2 站点管理 API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/mysite` | 获取站点列表 | 是 |
| POST | `/api/v1/mysite` | 创建站点 | 是 |
| GET | `/api/v1/mysite/{id}` | 获取站点详情 | 是 |
| PUT | `/api/v1/mysite/{id}` | 更新站点 | 是 |
| DELETE | `/api/v1/mysite/{id}` | 删除站点 | 是 |
| POST | `/api/v1/mysite/{id}/sign` | 签到 | 是 |
| POST | `/api/v1/mysite/{id}/info` | 获取站点信息 | 是 |
| POST | `/api/v1/mysite/search` | 搜索种子 | 是 |
| POST | `/api/v1/mysite/sort` | 排序站点 | 是 |

### 7.3 下载器管理 API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/option/downloader` | 获取下载器列表 | 是 |
| POST | `/api/v1/option/downloader` | 创建下载器 | 是 |
| PUT | `/api/v1/option/downloader/{id}` | 更新下载器 | 是 |
| DELETE | `/api/v1/option/downloader/{id}` | 删除下载器 | 是 |

### 7.4 定时任务 API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/option/task` | 获取任务列表 | 是 |
| POST | `/api/v1/option/task` | 创建任务 | 是 |
| DELETE | `/api/v1/option/task/{id}` | 删除任务 | 是 |

### 7.5 TMDB API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/tmdb/search` | 搜索电影/电视剧 | 是 |
| GET | `/api/v1/tmdb/movie/{id}` | 获取电影详情 | 是 |
| GET | `/api/v1/tmdb/tv/{id}` | 获取电视剧详情 | 是 |

### 7.6 其他 API

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/source/rss` | 解析 RSS | 是 |
| POST | `/api/v1/source/torrent` | 下载种子 | 是 |
| GET | `/ws` | WebSocket 连接 | 是 |
| GET | `/health` | 健康检查 | 否 |

---

## 八、部署方案

### 8.1 Docker 部署

**Dockerfile**:

```dockerfile
FROM rust:1.75-alpine AS builder

WORKDIR /app
COPY . .

RUN apk add --no-cache musl-dev pkgconfig
RUN cargo build --release

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/target/release/harvest-rs .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8000

CMD ["./harvest-rs"]
```

**docker-compose.yml**:

```yaml
version: '3.8'

services:
  harvest:
    build: .
    container_name: harvest-rs
    restart: unless-stopped
    ports:
      - "8000:8000"
    volumes:
      - ./data:/app/data
    environment:
      - DATABASE_URL=sqlite:///app/data/data.db
      - REDIS_URL=redis://redis:6379/10
      - SECRET_KEY=your-secret-key
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    container_name: harvest-redis
    restart: unless-stopped
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

### 8.2 本地运行

```bash
# 克隆仓库
git clone https://github.com/ngfchl/harvest_rust.git
cd harvest_rust

# 复制配置文件
cp .env.example .env

# 编辑配置文件
vim .env

# 运行
cargo run --release
```

---

## 九、技术亮点

### 9.1 架构设计

1. **分层架构**: Handlers → Services → Database 清晰分离
2. **Trait 抽象**: Spider 和 DownloaderClient 便于扩展
3. **中间件模式**: 认证中间件统一处理
4. **依赖注入**: 使用 Actix-web 的 App Data 共享状态

### 9.2 性能优化

1. **异步处理**: 全面使用 Tokio 异步运行时
2. **连接池**: SeaORM 自动管理数据库连接池
3. **缓存**: Redis 用于缓存和消息队列
4. **编译优化**: LTO 和优化级别 3

### 9.3 可维护性

1. **模块化设计**: 清晰的模块划分
2. **类型安全**: Rust 的类型系统
3. **错误处理**: thiserror 和 anyhow 统一错误处理
4. **日志系统**: tracing 结构化日志

### 9.4 扩展性

1. **插件式爬虫**: 通过 Spider trait 扩展新站点
2. **多下载器支持**: 通过 DownloaderClient trait 支持多种下载器
3. **灵活的配置**: 环境变量配置
4. **API 设计**: RESTful API，易于集成

---

## 十、开发建议

### 10.1 待完成功能

1. **站点爬虫实现**: 实现具体站点的 Spider
2. **RSS 刷流**: 完整的 RSS 解析和过滤逻辑
3. **Free 刷流**: Free 种子抓取和规则过滤
4. **辅种功能**: 跨站点辅种实现
5. **WebSocket 消息处理**: 完整的消息类型处理

### 10.2 潜在改进

1. **更多站点支持**: 添加更多 PT 站点的爬虫
2. **任务队列**: 使用 apalis 或类似库实现更强大的任务队列
3. **限流机制**: 添加请求限流防止被封
4. **监控告警**: 添加监控和告警功能
5. **前端界面**: 开发 Web 前端界面

### 10.3 学习价值

Harvest-RS 项目展示了：

1. **Actix-web 的使用**: RESTful API、WebSocket、中间件
2. **SeaORM 的应用**: 数据库实体、迁移、查询
3. **异步编程**: Tokio 运行时、异步 trait
4. **爬虫设计**: Trait 抽象、错误处理
5. **定时任务**: Cron 表达式、任务调度
6. **认证授权**: JWT、中间件

---

## 附录

### A. 环境变量完整列表

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| HOST | 服务器地址 | 0.0.0.0 |
| PORT | 服务器端口 | 8000 |
| DATABASE_URL | 数据库连接字符串 | sqlite://./data.db?mode=rwc |
| REDIS_URL | Redis 连接字符串 | redis://127.0.0.1:6379/10 |
| SECRET_KEY | JWT 密钥 | your-secret-key-change-in-production |
| JWT_EXPIRATION | JWT 过期时间（秒） | 86400 |
| TMDB_API_KEY | TMDB API 密钥 | - |

### B. Cron 表达式参考

| 表达式 | 说明 |
|--------|------|
| `0 0 8 * * *` | 每天 8:00 |
| `0 0 */6 * * *` | 每 6 小时 |
| `0 */30 * * * *` | 每 30 分钟 |
| `0 */15 * * * *` | 每 15 分钟 |
| `0 0 * * *` | 每小时 |
| `0 0 0 * *` | 每天 0:00 |

### C. 数据库迁移

```sql
-- 用户表
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 站点表
CREATE TABLE my_sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site TEXT NOT NULL UNIQUE,
    nickname TEXT NOT NULL,
    sort_id INTEGER NOT NULL DEFAULT 1,
    tags JSON,
    user_id TEXT,
    username TEXT,
    email TEXT,
    passkey TEXT,
    authkey TEXT,
    cookie TEXT,
    user_agent TEXT,
    rss TEXT,
    torrents TEXT,
    available INTEGER NOT NULL DEFAULT 1,
    sign_in INTEGER NOT NULL DEFAULT 1,
    get_info INTEGER NOT NULL DEFAULT 1,
    repeat_torrents INTEGER NOT NULL DEFAULT 1,
    brush_free INTEGER NOT NULL DEFAULT 1,
    brush_rss INTEGER NOT NULL DEFAULT 0,
    package_file INTEGER NOT NULL DEFAULT 1,
    hr_discern INTEGER NOT NULL DEFAULT 0,
    search_torrents INTEGER NOT NULL DEFAULT 1,
    show_in_dash INTEGER NOT NULL DEFAULT 1,
    proxy TEXT,
    remove_torrent_rules JSON,
    mirror TEXT,
    time_join TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    latest_active TIMESTAMP,
    mail INTEGER NOT NULL DEFAULT 0,
    notice INTEGER NOT NULL DEFAULT 0,
    sign_info JSON,
    status JSON,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 下载器表
CREATE TABLE downloaders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    url TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 定时任务表
CREATE TABLE scheduled_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    task_type TEXT NOT NULL,
    site_id INTEGER,
    enabled INTEGER NOT NULL DEFAULT 1,
    last_run TIMESTAMP,
    next_run TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (site_id) REFERENCES my_sites(id)
);
```

---

*分析完成于 2026-04-11*
