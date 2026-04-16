# rflush 深度分析报告

> **项目地址**: examples/rflush
> **语言**: Rust (edition 2024)
> **定位**: 基于 Web 界面的 RSS 种子下载器 + PT 刷流任务管理工具
> **分析日期**: 2026-04-15

---

## 一、项目概述

### 1.1 定位

rflush 是一个轻量级 PT 工具，将 **RSS 自动下载** 和 **PT 刷流** 两大核心功能集成在单文件二进制中，通过 React Web UI 进行管理。仅支持 qBittorrent 下载器，支持 NexusPHP 和 M-Team 两种站点框架。

### 1.2 技术栈

| 层次 | 技术 |
|------|------|
| 语言 | Rust (edition 2024) |
| 异步运行时 | Tokio (多线程) |
| Web 框架 | Axum 0.8 (CORS + Trace 中间件) |
| 数据库 | SQLite (rusqlite, bundled, WAL) |
| HTTP 客户端 | reqwest (rustls-tls, brotli/gzip/deflate/zstd) |
| XML 解析 | quick-xml (事件驱动) |
| HTML 解析 | scraper (CSS 选择器) |
| 前端 | React + TypeScript + shadcn/ui + Tailwind |
| 前端嵌入 | rust-embed (编译时嵌入 frontend/dist) |
| CLI | clap 4 (derive + env) |
| 日志 | tracing + tracing-subscriber (热重载 + SSE 广播) |
| 序列化 | serde + serde_json |
| 错误处理 | thiserror 2 |

### 1.3 核心功能

- **RSS 下载**：多订阅、并发下载、去重、自动重试、链接过期刷新
- **PT 刷流**：Cron 调度、两轮选种、8 种删种规则、OR/AND 模式
- **站点管理**：NexusPHP (HTML+API)、M-Team (API)，连接测试和用户统计
- **下载器管理**：qBittorrent，连接测试和空间统计
- **统计系统**：30s 快照采集、任务级/种子级流量统计、趋势图 API
- **Web 管理**：全配置通过 Web UI，SSE 实时日志流

---

## 二、架构分析

### 2.1 源码结构

```
src/
├── main.rs          # 入口，启动编排 (82 行)
├── cli.rs           # 命令行参数 (clap derive)
├── config.rs        # 配置数据结构
├── error.rs         # 统一错误类型 (AppError)
├── logging.rs       # 日志初始化、热重载、SSE 广播
├── db.rs            # SQLite 数据层 (~1892 行)
├── engine.rs        # RSS 下载引擎
├── web.rs           # Axum 路由 + API handler (~1399 行)
├── history.rs       # 下载记录/运行历史
├── collector.rs     # 下载器快照定期收集器
├── stats/           # 统计消费者
│   └── mod.rs
├── rss/             # RSS 解析 + 抓取
│   ├── mod.rs       # XML 解析器、文本标记识别
│   └── feed.rs      # fetch_all_rss, refresh_download_url
├── download/        # 种子文件下载
│   ├── mod.rs       # download_torrent (重试/过期/限流)
│   └── naming.rs    # 文件名处理
├── brush/           # 刷流引擎
│   ├── mod.rs       # BrushTaskRecord、工具函数
│   ├── scheduler.rs # Cron 调度器 + 核心执行逻辑 (~1292 行)
│   └── cleaner.rs   # 删种规则评估器
├── site/            # 站点适配器
│   ├── mod.rs       # SiteAdapter trait
│   ├── nexusphp.rs  # NexusPHP 适配器 (512 行)
│   ├── mteam.rs     # M-Team 适配器 (354 行)
│   └── factory.rs   # 工厂方法
├── downloader/      # 下载器客户端
│   ├── mod.rs       # DownloaderClient trait
│   ├── qbittorrent.rs # qB API 实现
│   └── factory.rs
└── net/             # 网络层
    ├── http.rs      # AppHttpClient (域名级限流)
    └── rate_limiter.rs # FIFO 限流器 + 自动冻结
```

### 2.2 启动流程

```
bootstrap_and_run()
  ├─ Cli::parse() → 解析命令行
  ├─ resolve_paths() → 确定 base_dir 和 db_dir
  ├─ Database::open() → 打开 SQLite (WAL + foreign_keys)
  ├─ get_settings() → 读取全局配置
  ├─ init_logging() → tracing + 热重载 + SSE 广播
  ├─ DownloaderSnapshotCollector::new() → 30s 轮询循环
  ├─ start_stats_consumer() → 订阅 broadcast 写统计
  ├─ BrushScheduler::new() → 15s 轮询 Cron 调度
  └─ web::serve() → Axum 服务器 (阻塞到 Ctrl+C)
```

---

## 三、核心模块详解

### 3.1 RSS 下载引擎

**涉及文件**: `engine.rs`, `rss/mod.rs`, `rss/feed.rs`, `download/mod.rs`

#### RSS 解析器 (`rss/mod.rs`)

基于 `quick-xml` 的事件驱动解析，支持 torznab 扩展属性：

| torznab 属性 | 映射字段 |
|-------------|---------|
| `seeders` | `seeders` |
| `downloadvolumefactor` | `download_volume_factor` |
| `uploadvolumefactor` | `upload_volume_factor` |
| `minimumratio` | `minimum_ratio` |
| `minimumseedtime` | `minimum_seed_time` |

**文本标记识别**：当 RSS 未提供结构化属性时，从 title/description/category 文本中识别：

| 标记 | 识别结果 |
|------|---------|
| FREE / 免费 / 🔑 | `free = true, download_volume_factor = 0` |
| 2XUP / 双倍上传 | `upload_volume_factor = 2` |
| 2XFREE / 双免 | `free = true, download_volume_factor = 0, upload_volume_factor = 2` |
| 50%DL / 半价 | `download_volume_factor = 0.5` |
| 30%DL / 30%折扣 | `download_volume_factor = 0.3` |
| H&R | `hit_and_run = true` |

#### 下载引擎 (`engine.rs`)

- **无状态设计**：每次接收一组配置，执行完本轮后返回结果
- **去重策略**：扫描目标目录已有 `.torrent` 文件，提取 guid key，`HashSet` 比对
- **并发模型**：`futures::stream::buffer_unordered(concurrency)`，默认 32 并发
- **原子写入**：先写 `.torrent.tmp`，完成后 `rename`

#### 种子下载 (`download/mod.rs`)

`download_torrent()` 完整重试循环：

```
loop {
    match download_result {
        ExpiredLink  → 触发 RSS 刷新获取新 URL → 重试
        RateLimited  → 等待限流恢复 → 重试
        Retriable    → 等待 retry_interval_secs → 重试
        Fatal        → 直接返回错误
        Success      → 返回数据
    }
}
```

### 3.2 刷流引擎

**涉及文件**: `brush/scheduler.rs` (~1292 行), `brush/cleaner.rs`

这是项目最核心的模块。

#### 调度机制

- `BrushScheduler` 维护 `running_tasks: Arc<RwLock<HashMap<i64, RunningBrushTask>>>`
- 主循环每 **15 秒** 检查所有 enabled 刷流任务的 Cron 触发时间
- 支持手动触发、停止、配置热刷新
- 每任务独立 `tokio::spawn`，通过 `Arc<RwLock<BrushTaskRecord>>` 支持运行时更新

#### 核心执行流程

```
execute_brush_task:
  1. 获取下载器配置 → 创建 DownloaderClient
  2. 获取当前管理种子列表 (DB active + 下载器 tag)
  3. 执行删种规则 (cleaner::evaluate_delete_rules)
     → 删除符合条件的种子，记录删除前流量
  4. 检查并发数
  5. 检查保种体积
  6. 检查磁盘剩余空间 (含预测)
  7. 拉取 RSS → 解析 → 排序（发布时间降序）
  8. 逐个增强、选种、添加：
     a. 跳过已存在种子
     b. 第一轮过滤 (RssPreFilter) — RSS 属性快速筛选
     c. 详情增强 — 仅 RSS 数据不足时请求站点
     d. 第二轮过滤 (PostEnhancement) — 完整属性严格筛选
     e. 再次检查并发/体积/磁盘
     f. 下载 .torrent → 提取 info_hash (自实现 SHA1+bencode)
     g. 添加到下载器 (tag/save_path/limits)
     h. 记录到 DB
```

#### ⭐ 两轮过滤设计（核心亮点）

```
第一轮 (RssPreFilter):
  - 仅使用 RSS 已有属性 (promotion/size/seeders)
  - RSS 缺少 promotion/H&R 属性时不提前淘汰，保留为候选
  - 目的：快速减少候选集，避免不必要的站点请求

第二轮 (PostEnhancement):
  - 仅对需要补充判定的候选种子请求站点详情页/API
  - 用完整属性做严格筛选 (精确促销类型/免费时长/H&R)
  - 目的：精确判定，确保不遗漏促销种子
```

**好处**：大幅减少站点 HTTP 请求，避免限流，同时确保不遗漏免费种子。

#### 选种规则

| 规则 | 字段 | 说明 |
|------|------|------|
| 体积范围 | `size_ranges` | JSON `["1-10","20-50"]`，单位 GB |
| 做种人数范围 | `seeder_ranges` | JSON 同上 |
| 促销类型 | `promotion` | `all`/`free`/`normal` |
| 最小免费时长 | `min_free_hours` | 仅 promotion=free 时生效 |
| 跳过 H&R | `skip_hit_and_run` | true 时排除 H&R 种子 |
| 最大并发 | `max_concurrent` | 默认 100 |
| 下载/上传限速 | `download_speed_limit` / `upload_speed_limit` | KB/s |
| 活跃时间窗口 | `active_time_windows` | JSON `["00:00-09:00","22:00-06:00"]`，支持跨天 |

#### 删种规则

| 规则 | 字段 | 说明 |
|------|------|------|
| 最小做种时长 | `min_seed_time_hours` | 做种时间达标后可删 |
| free 到期删除 | `delete_on_free_expiry` | 促销到期后删除 |
| H&R 最小做种 | `hr_min_seed_time_hours` | 仅对 H&R 种子 |
| 目标分享率 | `target_ratio` | 分享率达标后可删 |
| 最大上传量 | `max_upload_gb` | 上传量达标后可删 |
| 下载超时 | `download_timeout_hours` | 下载未完成且超时 |
| 最低平均上传 | `min_avg_upload_speed_kbs` | 近 10 分钟平均上传低于阈值 |
| 最大不活跃 | `max_inactive_hours` | 最后活动时间超时 |
| 删种模式 | `delete_mode` | `or`(任一满足)/`and`(全部满足) |

#### 自实现 InfoHash 提取

不依赖外部 bencode/sha1 库：
- `find_info_dict_range()` — 定位 torrent 文件中 `info` 字典范围
- `sha1_digest()` — 完整 SHA1 实现
- `hex_encode()` — 十六进制编码
- `extract_info_hash()` — 从 .torrent 提取 info_hash

### 3.3 站点适配器

#### SiteAdapter trait

```rust
pub trait SiteAdapter: Send + Sync {
    fn test_connection(&self) -> Pin<Box<dyn Future<Output = Result<SiteTestResult, String>> + Send + '_>>;
    fn get_user_stats(&self) -> Pin<Box<dyn Future<Output = Result<UserStats, String>> + Send + '_>>;
    fn get_torrent_attributes(&self, detail_url: &str) -> Pin<Box<dyn Future<Output = Result<TorrentAttributes, String>> + Send + '_>>;
}
```

#### 认证方式 (SiteAuth 枚举)

- `Cookie { cookie }`
- `Passkey { passkey }`
- `CookiePasskey { cookie, passkey }`
- `ApiKey { api_key }` — M-Team 专用

#### NexusPHP 适配器 (`nexusphp.rs`, 512 行)

- **双模式用户信息**：优先 API `/api/user`，失败回退 HTML `/index.php` 解析
- **详情页解析**：CSS 选择器 + 文本匹配检测 FREE/2XFREE/H&R 标记
- **免费到期检测**：从详情页提取 `YYYY-MM-DD HH:MM:SS` 格式时间戳
- **促销类型识别**：50%/30% 折扣、2XUP/0XUP 等

#### M-Team 适配器 (`mteam.rs`, 354 行)

- 纯 API 模式：`/api/member/profile`、`/api/torrent/detail`
- 自动退避：遇到"請求過於頻繁"最多 3 次指数退避
- 请求间隔：每次详情请求前等待 4s + 随机 0-4s
- 促销映射：FREE, FREE_2XUP, PERCENT_50, PERCENT_70 等

### 3.4 下载器客户端

#### DownloaderClient trait

```rust
pub trait DownloaderClient: Send + Sync {
    fn test_connection(&self) -> ...;
    fn add_torrent(&self, torrent_data, filename, options) -> ...;
    fn list_torrents(&self, tag: Option<&str>) -> ...;
    fn delete_torrent(&self, hash, delete_files) -> ...;
    fn get_free_space(&self, path: Option<&str>) -> ...;
    fn get_effective_free_space(&self, path, torrents) -> ...;
}
```

#### qBittorrent 实现

- Cookie 自动管理：登录缓存 SID，403 自动重新登录
- `add_torrent`：multipart 上传，支持 savepath/tags/category/dlLimit/upLimit/paused
- `list_torrents`：tag 过滤，返回 17 字段的 `TorrentInfo`
- `TorrentInfo` 字段：hash, name, size, uploaded, downloaded, upload_speed, download_speed, ratio, state, added_on, completion_on, num_seeds, num_leechs, save_path, tags, category, time_active, last_activity

### 3.5 下载器快照收集器 (`collector.rs`)

- 每 **30 秒** 轮询所有配置下载器的种子列表
- 内存存储 `RwLock<HashMap<i64, Arc<DownloaderSnapshot>>>`
- 通过 `broadcast::channel` 发布给 stats consumer
- 按需刷新：无缓存时立即拉取

### 3.6 统计系统 (`stats/mod.rs`)

订阅 collector 的 broadcast，每收到快照：
1. 汇总下载器 upload_speed/download_speed → `downloader_speed_snapshots`
2. 按 tag 过滤种子 → `task_stats_snapshots`
3. 逐种子更新 `brush_task_torrents` 流量统计
4. 写入 `torrent_traffic` 细粒度快照（用于计算近 10 分钟平均上传速度）
5. 自动清理 7 天前 `torrent_traffic`

### 3.7 网络层

#### 域名级限流 (`net/rate_limiter.rs`)

- **FIFO 滑动窗口**：按 `protocol + host(+port)` 维度的 `VecDeque<Instant>` 队列
- **自动冻结**：收到限流响应后，冻结该域名 `throttle_duration`（默认 30s）
- 冻结期间所有该域名请求阻塞等待
- `SharedRateLimiter` 全局共享

#### HTTP 客户端 (`net/http.rs`)

- 内置浏览器 UA
- 自动检测 M-Team 限流 → 触发 throttle
- 检测链接过期 → 触发刷新

### 3.8 日志系统 (`logging.rs`)

- tracing-subscriber layered 架构
- **热重载**：`reload::Layer` 支持运行时修改日志级别
- **SSE 广播**：自定义 `MakeWriter` 将日志同时写入 stdout 和 broadcast channel
- **ANSI 清理**：广播前去除 ANSI 转义序列
- **task-local 上下文**：每个异步任务有独立 ID

---

## 四、数据模型

### 4.1 数据库表

| 表名 | 用途 |
|------|------|
| `global_settings` | 全局配置（单行 id=1） |
| `rss_subscriptions` | RSS 订阅 |
| `download_runs` | RSS 下载执行批次 |
| `download_records` | 每个种子下载记录 |
| `sites` | 站点配置 |
| `downloaders` | 下载器配置 |
| `brush_tasks` | 刷流任务 |
| `brush_task_torrents` | 刷流种子记录 |
| `task_stats_snapshots` | 任务统计快照 |
| `torrent_traffic` | 种子级流量快照 |
| `downloader_speed_snapshots` | 下载器速度快照 |

### 4.2 关键结构体

| 结构体 | 文件 | 用途 |
|--------|------|------|
| `AppConfig` | config.rs | 全局运行配置 |
| `GlobalConfig` | config.rs | 限流/并发/日志设置 |
| `RssSubscription` | config.rs | RSS 订阅记录 |
| `TorrentItem` | rss/mod.rs | RSS 解析种子条目 |
| `BrushTaskRecord` | brush/mod.rs | 刷流任务完整记录 |
| `BrushTorrentRecord` | brush/mod.rs | 刷流种子记录 |
| `TorrentAttributes` | site/mod.rs | 站点返回的种子属性 |
| `UserStats` | site/mod.rs | 站点用户统计 |
| `TorrentInfo` | downloader/mod.rs | 下载器种子信息 |
| `AddTorrentOptions` | downloader/mod.rs | 添加种子选项 |
| `DownloaderSnapshot` | collector.rs | 下载器快照 |
| `AppState` | web.rs | Axum 共享状态 |

---

## 五、API 端点完整列表

### 全局设置

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/settings` | 获取全局设置 |
| PUT | `/api/settings` | 更新全局设置 |

### RSS 管理

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/rss` | 列出所有 RSS 订阅 |
| POST | `/api/rss` | 创建 RSS 订阅 |
| DELETE | `/api/rss/{id}` | 删除 RSS 订阅 |

### RSS 任务控制

| 方法 | 路径 | 用途 |
|------|------|------|
| POST | `/api/tasks/{id}/start` | 启动任务 |
| POST | `/api/tasks/{id}/pause` | 暂停任务 |
| POST | `/api/tasks/{id}/delete` | 删除任务 |
| GET | `/api/tasks/{id}/records` | 获取下载记录 |
| POST | `/api/tasks/start` | 批量启动 |
| POST | `/api/tasks/pause` | 批量暂停 |
| POST | `/api/tasks/delete` | 批量删除 |
| POST | `/api/tasks/start-all` | 全部启动 |
| POST | `/api/tasks/pause-all` | 全部暂停 |
| POST | `/api/tasks/delete-all` | 全部删除 |

### 下载历史

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/history` | 下载历史 |
| GET | `/api/runs` | 执行批次列表 |
| GET | `/api/runs/{id}/records` | 批次下载记录 |

### RSS 执行

| 方法 | 路径 | 用途 |
|------|------|------|
| POST | `/api/jobs/run-all` | 全部执行 |
| POST | `/api/jobs/run/{id}` | 单个执行 |

### 站点管理

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/sites` | 列出站点 |
| POST | `/api/sites` | 创建站点 |
| PUT | `/api/sites/{id}` | 更新站点 |
| DELETE | `/api/sites/{id}` | 删除站点 |
| POST | `/api/sites/{id}/test` | 测试连接 |
| GET | `/api/sites/{id}/stats` | 用户统计 |

### 下载器管理

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/downloaders` | 列出下载器 |
| POST | `/api/downloaders` | 创建下载器 |
| PUT | `/api/downloaders/{id}` | 更新下载器 |
| DELETE | `/api/downloaders/{id}` | 删除下载器 |
| POST | `/api/downloaders/{id}/test` | 测试连接 |
| GET | `/api/downloaders/{id}/space` | 空间统计 |

### 刷流任务

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/brush-tasks` | 列出刷流任务 |
| POST | `/api/brush-tasks` | 创建刷流任务 |
| GET | `/api/brush-tasks/{id}` | 获取单个 |
| PUT | `/api/brush-tasks/{id}` | 更新 |
| DELETE | `/api/brush-tasks/{id}` | 删除 |
| POST | `/api/brush-tasks/{id}/start` | 启用 |
| POST | `/api/brush-tasks/{id}/stop` | 停用 |
| POST | `/api/brush-tasks/{id}/run` | 立即执行 |
| GET | `/api/brush-tasks/{id}/torrents` | 种子列表 |

### 统计

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/stats/overview` | 刷流总览 |
| GET | `/api/stats/trend` | 任务趋势 |
| GET | `/api/stats/downloader-speed-trend` | 下载器速度趋势 |

### 系统

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/api/system/logs/stream` | SSE 实时日志流 |
| GET | `/` | 前端首页 |
| GET | `/{*path}` | 静态资源 |

---

## 六、配置项完整列表

### CLI 参数

| 参数 | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `-H, --host` | `RFLUSH_HOST` | `0.0.0.0` | 监听地址 |
| `-p, --port` | `RFLUSH_PORT` | `3000` | 监听端口 |
| `-d, --data-dir` | `RFLUSH_DATA_DIR` | `./data` | 数据目录 |

### 全局设置 (数据库)

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `download_rate_limit.requests` | 2 | 时间窗口最大请求数 |
| `download_rate_limit.interval` | 1 | 时间窗口间隔 |
| `download_rate_limit.unit` | second | 时间单位 |
| `retry_interval_secs` | 5 | 重试间隔 |
| `log_level` | info | 日志级别 |
| `max_concurrent_downloads` | 32 | 最大并发下载 |
| `max_concurrent_rss_fetches` | 8 | 最大并发 RSS 拉取 |
| `throttle_interval_secs` | 30 | 限流冻结时长 |

### 刷流任务配置

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `name` | - | 任务名称 |
| `cron_expression` | - | Cron (5 字段) |
| `site_id` | - | 绑定站点 |
| `downloader_id` | - | 绑定下载器 |
| `tag` | - | qB 标签 |
| `rss_url` | - | RSS URL |
| `seed_volume_gb` | - | 保种体积上限 GB |
| `save_dir` | - | 保存目录 |
| `active_time_windows` | - | 活跃时间窗口 JSON |
| `promotion` | all | 促销过滤 (all/free/normal) |
| `skip_hit_and_run` | true | 跳过 H&R |
| `max_concurrent` | 100 | 最大并发种子 |
| `download_speed_limit` | - | 下载限速 KB/s |
| `upload_speed_limit` | - | 上传限速 KB/s |
| `size_ranges` | - | 体积范围 JSON |
| `seeder_ranges` | - | 做种数范围 JSON |
| `min_free_hours` | - | 最小免费时长 h |
| `delete_mode` | or | 删种模式 |
| `delete_on_free_expiry` | false | free 到期删除 |
| `min_seed_time_hours` | - | 最小做种时长 h |
| `hr_min_seed_time_hours` | - | H&R 最小做种 h |
| `target_ratio` | - | 目标分享率 |
| `max_upload_gb` | - | 最大上传量 GB |
| `download_timeout_hours` | - | 下载超时 h |
| `min_avg_upload_speed_kbs` | - | 最低平均上传 KB/s |
| `max_inactive_hours` | - | 最大不活跃 h |
| `min_disk_space_gb` | - | 最低磁盘剩余 GB |

---

## 七、与 PT-Forward 设计对比

### 7.1 功能覆盖对比

| 功能 | PT-Forward | rflush | 说明 |
|------|-----------|--------|------|
| RSS 下载 | ✅ | ✅ | rflush 去重/重试/链接过期更完善 |
| PT 刷流 | ✅ | ✅ | rflush 两轮过滤设计更优 |
| 选种规则 | ✅ 评分排序 | ✅ 范围过滤 | 策略不同，互补 |
| 删种规则 | ✅ 39键×10运算符 | ✅ 8种规则+OR/AND | PT-Forward 更灵活 |
| Cron 调度 | ✅ | ✅ | 相当 |
| 站点适配 | ✅ 5框架+116站 | ✅ NP+M-Team | PT-Forward 远超 |
| 下载器 | ✅ qB+TR | ✅ 仅qB | PT-Forward 远超 |
| Web UI | ❌ 无设计 | ✅ React | rflush 有完整前端 |
| 统计系统 | ✅ API设计 | ✅ 完整实现 | rflush 有趋势图 |
| SSE 实时日志 | ❌ | ✅ | rflush 独有 |
| 日志热重载 | ✅ AtomicLevel | ✅ reload Layer | 相当 |
| 辅种引擎 | ✅ 三层融合 | ❌ | PT-Forward 独有 |
| 通知系统 | ✅ | ❌ | PT-Forward 独有 |
| Tracker 管理 | ✅ | ❌ | PT-Forward 独有 |
| CookieCloud | ✅ | ❌ | PT-Forward 独有 |
| API 认证 | ✅ Token | ❌ | PT-Forward 独有 |
| 配置热重载 | ✅ EventBus | ✅ RwLock | 实现方式不同 |
| Docker | ✅ | ✅ | 相当 |
| 活跃时间窗口 | ⚠️ hour条件键 | ✅ 完整 | rflush 更完善 |

### 7.2 实现方式差异

| 方面 | PT-Forward | rflush |
|------|-----------|--------|
| 语言 | Go | Rust |
| 并发 | goroutine + channel | tokio async + broadcast |
| RSS 解析 | gofeed | quick-xml 事件驱动 |
| HTML 解析 | goquery | scraper |
| 限流 | 三态熔断器 | FIFO 滑动窗口 + 自动冻结 |
| 种子去重 | DB 唯一索引 | 文件系统扫描 |
| 刷流选种 | L/S 评分排序 | 两轮过滤 + 范围匹配 |
| 免费检测 | API + 页面 | RSS 属性 + 详情页增强 |
| 前端 | 无设计 | React + shadcn |
| 数据库 | SQLite/MySQL | SQLite |

---

## 八、值得 PT-Forward 借鉴的设计

### 8.1 ⭐ 两轮过滤（RSS 预筛 + 详情增强）

**核心思想**：
1. 第一轮仅用 RSS 属性快速筛选，缺少促销/H&R 时不提前淘汰
2. 仅对候选种子请求站点详情页/API
3. 第二轮用完整属性严格筛选

**对 PT-Forward 的影响**：§18 刷流选种的 RSS 管道已有免费检测步骤，但未考虑 RSS 属性不完整时的降级策略。可借鉴两轮过滤减少站点 API 调用。

### 8.2 ⭐ 运行中配置热刷新

`Arc<RwLock<BrushTaskRecord>>` + 每次迭代 `snapshot_task()` 读取最新配置，API 更新后无需重启。

**对 PT-Forward 的影响**：§16.2 ConfigEventBus 已有配置热更新设计，但刷流策略的热刷新粒度可以更细。

### 8.3 域名级 FIFO 限流 + 自动冻结

按 `protocol + host(+port)` 限流，收到限流响应后自动冻结域名 30s。

**对 PT-Forward 的影响**：§8.3 的三态熔断器需要累计 5 次失败才断开，对限速场景（如 M-Team 30/min）反应不够快。可借鉴"收到 429 立即冻结"的策略。

### 8.4 下载器快照收集器 + broadcast 解耦

独立 collector 每 30s 拉取下载器状态，broadcast 发布给消费者。

**对 PT-Forward 的影响**：§14.13 maindata 缓存是按需拉取模式。rflush 的主动定期推送模式更适合统计场景，可参考。

### 8.5 SSE 实时日志流

tracing 输出同时写入 stdout 和 broadcast channel，前端 SSE 实时接收。

**对 PT-Forward 的影响**：§26 监控设计了日志查询 API，但缺少实时推送。SSE 是轻量级的实时方案。

### 8.6 活跃时间窗口

支持 `"22:00-06:00"` 跨天窗口，刷流任务按时段运行。

**对 PT-Forward 的影响**：§14 有 `hour` 条件键但不够完整。应增加 `activeTimeWindows` 字段支持跨天窗口。

### 8.7 磁盘空间预测

`get_effective_free_space()` 计算 `free_space - pending_download_bytes`（未完成种子的剩余下载量）。

**对 PT-Forward 的影响**：§17.5 DiskBudget 已有预留设计，但未考虑正在下载的种子未来占用。

### 8.8 刷流种子列表实时状态合并

API 返回时合并 DB 记录和下载器实时状态，标记 removed 但仍存在的种子恢复为活跃。

**对 PT-Forward 的影响**：刷流种子查询 API 应参考此设计，确保展示状态与下载器一致。

---

## 九、项目不足之处

### 9.1 功能缺失

1. **无辅种功能**
2. **无通知系统**（Telegram/邮件/Webhook）
3. **无 Tracker 管理**
4. **仅 qBittorrent**，无 Transmission/Deluge
5. **仅 NexusPHP + M-Team**，缺 Unit3D/Gazelle
6. **无 CookieCloud 集成**
7. **无凭据加密存储**
8. **无 API 认证**

### 9.2 架构不足

9. **无数据库连接池**：每次操作新建 Connection
10. **scheduler.rs 过大**（1292 行）：混合调度/执行/过滤/工具/SHA1
11. **无版本化迁移**：Schema 变更通过 `ensure_column` 逐个处理
12. **无事务保证**：删种后 DB 更新和下载器删除非原子

### 9.3 刷流引擎不足

13. **无种子评分排序**：按发布时间排序，缺综合评分
14. **无保种策略**：刷流和保种未分离
15. **无流量目标**：缺"达到日均上传 N GB 后停止"
16. **M-Team 无 H&R 检测**：硬编码 `hit_and_run = false`

---

## 十、总结

rflush 是一个功能完备、设计精巧的 Rust PT 工具。在**两轮过滤**、**运行时热刷新**、**域名级限流+自动冻结**、**下载器快照采集**、**SSE 实时日志**、**活跃时间窗口**、**磁盘空间预测**等方面有优秀设计值得 PT-Forward 借鉴。

PT-Forward 在辅种引擎、站点覆盖（5框架116站）、下载器支持（qB+TR）、通知系统、API 认证/安全等方面远超 rflush。两者互补性很强。
