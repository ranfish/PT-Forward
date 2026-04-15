# TorrentBotX 深度分析文档

> 面向多下载器、多 PT 站点的自动化下载与管理平台深度技术分析

---

## 目录

1. [项目概览与技术栈](#1-项目概览与技术栈)
2. [架构设计与模块划分](#2-架构设计与模块划分)
3. [核心管理器与业务逻辑](#3-核心管理器与业务逻辑)
4. [下载器适配器体系](#4-下载器适配器体系)
5. [PT 站点适配器体系](#5-pt-站点适配器体系)
6. [Telegram Bot 交互系统](#6-telegram-bot-交互系统)
7. [定时任务调度系统](#7-定时任务调度系统)
8. [数据库设计与持久化](#8-数据库设计与持久化)
9. [配置管理系统](#9-配置管理系统)
10. [通知系统](#10-通知系统)
11. [日志系统](#11-日志系统)
12. [工具模块](#12-工具模块)
13. [数据模型层](#13-数据模型层)
14. [测试体系](#14-测试体系)
15. [部署与容器化](#15-部署与容器化)
16. [安全分析](#16-安全分析)
17. [性能分析](#17-性能分析)
18. [设计模式分析](#18-设计模式分析)
19. [项目优缺点评估](#19-项目优缺点评估)
20. [改进建议](#20-改进建议)
21. [完整文件结构](#21-完整文件结构)
22. [API 接口规范](#22-api-接口规范)
23. [数据库关系图](#23-数据库关系图)
24. [附录](#附录)

---

## 1. 项目概览与技术栈

### 1.1 项目定位

TorrentBotX 是一个**面向多下载器、多 PT 站点**的自动化下载与管理平台。核心目标是通过统一接口，实现 qBittorrent、aria2、Transmission 三大下载器与主流 PT 站点的联动，支持定时任务、自动清理和 Telegram 机器人交互。

### 1.2 核心特性

| 特性 | 描述 |
|------|------|
| 多下载器支持 | qBittorrent、aria2、Transmission 三合一 |
| 多 PT 站点集成 | M-Team、DicMusic、Carpt、PTSKit 四大站点 |
| Telegram Bot 交互 | 远程管理、搜索、添加、监控、操作下载任务 |
| 灵活定时调度 | APScheduler 支持间隔/定时/Cron 三种触发模式 |
| 自动化任务处理 | 自动下载、自动删种/荣退、分类、批量管理 |
| 高度模块化 | 清晰的分层架构，便于二次开发与扩展 |
| 本地 SQLite 数据库 | 任务、用户、历史等数据安全持久化 |
| 优雅日志与通知 | 全局日志系统 + Telegram 消息推送 |

### 1.3 技术栈

| 类别 | 技术 | 版本 | 用途 |
|------|------|------|------|
| 语言 | Python | 3.11+ | 主开发语言 |
| 下载器 SDK | qbittorrent-api | 2025.5.0 | qBittorrent API 封装 |
| 下载器 SDK | transmission-rpc | 7.0.11 | Transmission RPC 封装 |
| 下载器 SDK | aria2p | 0.12.1 | Aria2 JSON-RPC 封装 |
| Bot 框架 | python-telegram-bot | 22.1 | Telegram Bot API |
| 任务调度 | APScheduler | 3.11.0 | 定时任务调度 |
| HTTP 客户端 | requests | 2.32.4 | PT 站点 API 请求 |
| HTTP 客户端 | httpx | 0.28.1 | 异步 HTTP 请求（已引入未使用） |
| 配置管理 | PyYAML | 6.0.2 | YAML 配置解析 |
| 配置管理 | pydantic | 2.11.7 | 数据验证与设置 |
| 配置管理 | pydantic-settings | 2.9.1 | 配置管理 |
| 环境变量 | python-dotenv | 1.1.0 | .env 文件加载 |
| 日志 | loguru | 0.7.3 | 结构化日志（已引入未完全使用） |
| 数据库 | SQLite | 内置 | 本地数据持久化 |
| 容器化 | Docker | - | 部署支持 |

### 1.4 依赖关系图

```
TorrentBotX
├── python-telegram-bot  ←── Telegram Bot 交互
├── qbittorrent-api      ←── qBittorrent 下载器
├── transmission-rpc     ←── Transmission 下载器
├── aria2p               ←── Aria2 下载器
├── APScheduler          ←── 定时任务调度
├── requests             ←── PT 站点 API 请求
├── httpx                ←── 异步 HTTP（已引入但未使用）
├── PyYAML               ←── 配置文件解析
├── pydantic             ←── 数据验证
├── pydantic-settings    ←── 配置管理
├── python-dotenv        ←── 环境变量
├── loguru               ←── 日志（已引入但未完全使用）
└── SQLite               ←── 数据持久化
```

### 1.5 许可证

MIT License，Copyright (c) 2025 AstralWave

---

## 2. 架构设计与模块划分

### 2.1 整体架构

TorrentBotX 采用**分层模块化架构**，以 `CoreManager` 为核心调度中枢，向下对接下载器适配器，向上对接 Telegram Bot 和定时任务调度器。

```
┌─────────────────────────────────────────────────────────────┐
│                      用户交互层                              │
│  ┌──────────────────┐  ┌──────────────────────────────────┐ │
│  │  Telegram Bot    │  │  定时任务调度器 (APScheduler)     │ │
│  │  /start /add     │  │  interval / cron / date          │ │
│  │  /help /qbtasks  │  │                                  │ │
│  │  /cancel         │  │                                  │ │
│  └────────┬─────────┘  └──────────────┬───────────────────┘ │
│           │                           │                     │
├───────────┼───────────────────────────┼─────────────────────┤
│           │      核心调度层           │                     │
│           ▼                           ▼                     │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              CoreManager (核心管理器)                   │ │
│  │  - 初始化下载器列表                                     │ │
│  │  - 执行下载任务                                        │ │
│  │  - 通知消息推送                                        │ │
│  └──────────┬──────────────────────────┬──────────────────┘ │
│             │                          │                    │
├─────────────┼──────────────────────────┼────────────────────┤
│             │      适配器层            │                    │
│             ▼                          ▼                    │
│  ┌─────────────────────┐  ┌────────────────────────────┐   │
│  │  下载器适配器        │  │  PT 站点适配器             │   │
│  │  - qBittorrent      │  │  - MTeamTracker            │   │
│  │  - Transmission     │  │  - DicMusicTracker         │   │
│  │  - Aria2            │  │  - CarptTracker            │   │
│  │  (注册表模式)        │  │  - PTSKitTracker           │   │
│  └─────────────────────┘  │  (工厂模式)                │   │
│                           └────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│                      基础设施层                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  SQLite  │  │  Config  │  │  Logger  │  │ Notifier │   │
│  │  数据库  │  │  配置管理 │  │  日志系统 │  │ 通知系统  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 模块划分

| 模块 | 路径 | 职责 |
|------|------|------|
| `core` | `torrentbotx/core/` | 核心管理器，业务调度中枢 |
| `downloaders` | `torrentbotx/downloaders/` | 下载器适配器（QB/TR/Aria2） |
| `trackers` | `torrentbotx/trackers/` | PT 站点适配器（M-Team 等） |
| `bots/telegram` | `torrentbotx/bots/telegram/` | Telegram Bot 交互 |
| `tasks` | `torrentbotx/tasks/` | 定时任务调度 |
| `db` | `torrentbotx/db/` | 数据库连接与操作 |
| `models` | `torrentbotx/models/` | 业务数据模型 |
| `config` | `torrentbotx/config/` | 配置管理 |
| `notifications` | `torrentbotx/notifications/` | 通知推送 |
| `utils` | `torrentbotx/utils/` | 工具模块 |
| `enums` | `torrentbotx/enums/` | 枚举类型定义 |
| `tests` | `tests/` | 单元测试 |

### 2.3 启动流程

```
run.py main()
    │
    ├── 1. load_config()          ← 加载 YAML 配置
    │       └── Config._load_yaml()
    │           ├── 读取 config.yaml
    │           └── 不存在则从 example.yaml 复制
    │
    ├── 2. init_db()              ← 初始化数据库
    │       └── create_tables()
    │           ├── CREATE TABLE torrents
    │           ├── CREATE TABLE tasks
    │           └── CREATE TABLE users
    │
    ├── 3. TelegramNotifier()     ← 创建通知器
    │       └── Bot(token=bot_token)
    │
    ├── 4. CoreManager()          ← 创建核心管理器
    │       ├── _init_downloaders()  ← 初始化下载器列表
    │       │   ├── 解析 DOWNLOADERS 配置
    │       │   ├── DownloaderType.from_name()
    │       │   └── get_downloader_instance()
    │       └── start()              ← 启动并通知
    │
    └── 5. start_bot()            ← 启动 Telegram Bot
            ├── Application.builder()
            ├── setup_application()   ← 注册命令处理器
            │   ├── /start
            │   ├── /help
            │   ├── /add
            │   ├── /qbtasks
            │   └── /cancel
            └── run_polling()         ← 轮询模式运行
```

### 2.4 数据流

```
用户 Telegram 命令
    │
    ▼
Telegram Bot Handler
    │
    ▼
CoreManager.execute_task()
    │
    ├──→ Downloader.add_torrent()  ← 添加到下载器
    │       ├── QBittorrentDownloader
    │       ├── TransmissionDownloader
    │       └── Aria2Downloader
    │
    ├──→ Tracker.search_torrents()  ← 搜索 PT 站点
    │       ├── MTeamTracker
    │       ├── DicMusicTracker
    │       ├── CarptTracker
    │       └── PTSKitTracker
    │
    └──→ Notifier.send_message()  ← 发送通知
            └── TelegramNotifier
```

---

## 3. 核心管理器与业务逻辑

### 3.1 CoreManager

**文件**: [manager.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/core/manager.py)

CoreManager 是整个系统的调度中枢，负责初始化下载器、执行下载任务和发送通知。

```python
class CoreManager:
    def __init__(self, config=None, notifier: Optional[Notifier] = None):
        self.config = config or load_config()
        self.notifier = notifier or TelegramNotifier(...)
        self.downloaders = self._init_downloaders()
```

#### 核心方法分析

| 方法 | 功能 | 实现细节 |
|------|------|----------|
| `__init__` | 初始化 | 加载配置、创建通知器、初始化下载器列表 |
| `_init_downloaders` | 初始化下载器 | 解析 DOWNLOADERS 配置，遍历创建下载器实例 |
| `start` | 启动管理器 | 检查下载器加载情况，发送启动通知 |
| `execute_download_task` | 执行下载任务 | 遍历所有下载器，尝试添加种子 |

#### 下载器初始化流程

```python
def _init_downloaders(self) -> List:
    types = self.config.get("DOWNLOADERS", "qbittorrent")
    downloader_list = []
    for name in types.split(","):
        try:
            dtype = DownloaderType.from_name(name.strip())
            instance = get_downloader_instance(dtype)
            downloader_list.append(instance)
        except Exception as e:
            logger.error(f"❌ 加载下载器 {name} 失败: {e}")
    return downloader_list
```

**关键设计**：
- 支持多下载器配置，以逗号分隔（如 `"qbittorrent,aria2"`）
- 使用注册表模式（`_downloader_registry`）获取下载器实例
- 单个下载器加载失败不影响其他下载器

#### 下载任务执行

```python
def execute_download_task(self, params: dict):
    torrent_id = params.get("torrent_id")
    success_list = []
    for downloader in self.downloaders:
        success = downloader.add_torrent(torrent_id)
        success_list.append(success)
    
    if any(success_list):
        self.notifier.send_message(f"部分下载器已成功添加任务: {torrent_id}")
        return True
    else:
        self.notifier.send_message(f"所有下载器添加任务失败: {torrent_id}")
        return False
```

**设计特点**：
- 遍历所有下载器尝试添加种子
- 只要有一个下载器成功即视为成功（`any(success_list)`）
- 每次操作后通过通知器反馈结果

### 3.2 TorrentManager

**文件**: [manager.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/core/manager.py)

简化的种子管理器，主要用于单元测试。

```python
class TorrentManager:
    def __init__(self, qb_client=None) -> None:
        self.qb_client = qb_client

    def add_torrent(self, torrent_url: str) -> None:
        if not self.qb_client:
            raise RuntimeError("qBittorrent client 未初始化")
        self.qb_client.torrents_add(urls=torrent_url)

    def get_torrent(self, torrent_hash: str):
        if not self.qb_client:
            raise RuntimeError("qBittorrent client 未初始化")
        return self.qb_client.torrents_info(torrent_hashes=torrent_hash)
```

**定位**：测试辅助类，仅封装 qBittorrent 客户端的基本操作。

### 3.3 问题分析

| 问题 | 描述 | 严重程度 |
|------|------|----------|
| `execute_task` 方法缺失 | handler.py 中调用 `core_manager.execute_task()`，但 CoreManager 未定义该方法 | 🔴 严重 |
| 下载器连接失败未处理 | 下载器 `__init__` 中调用 `connect()`，失败时抛出异常但 CoreManager 未捕获 | 🟡 中等 |
| 无任务状态管理 | 缺少任务创建、更新、查询的完整生命周期管理 | 🟡 中等 |
| 通知器硬编码 | CoreManager 默认创建 TelegramNotifier，耦合度高 | 🟢 轻微 |

---

## 4. 下载器适配器体系

### 4.1 架构设计

下载器适配器采用**注册表模式 + 抽象基类**设计：

```
BaseDownloader (ABC)
    ├── add_torrent(torrent_url: str) -> bool
    ├── get_torrents() -> list
    ├── pause_torrent(torrent_id: str) -> bool
    └── resume_torrent(torrent_id: str) -> bool

@register_downloader(DownloaderType.QBITTORRENT)
QBittorrentDownloader(BaseDownloader)

@register_downloader(DownloaderType.TRANSMISSION)
TransmissionDownloader(BaseDownloader)

@register_downloader(DownloaderType.ARIA2)
Aria2Downloader(BaseDownloader)
```

### 4.2 注册表机制

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
```

**设计分析**：
- 使用装饰器注册模式，下载器类定义时即自动注册
- 全局注册表 `_downloader_registry` 维护类型到类的映射
- `get_downloader_instance()` 通过类型获取实例
- **注意**：每次调用都创建新实例（`cls()`），无连接复用

### 4.3 下载器类型枚举

**文件**: [downloader_type.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/enums/downloader_type.py)

```python
class DownloaderType(str, Enum):
    ARIA2 = "aria2"
    QBITTORRENT = "qbittorrent"
    TRANSMISSION = "transmission"

    @classmethod
    def from_name(cls, name: str) -> "DownloaderType":
        name = name.lower()
        for member in cls:
            if member.value == name:
                return member
        raise ValueError(f"无效的下载器类型: {name}")
```

### 4.4 qBittorrent 适配器

**文件**: [qbittorrent.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/downloaders/qbittorrent.py)

| 项目 | 详情 |
|------|------|
| SDK | `qbittorrent-api` 2025.5.0 |
| 认证方式 | 用户名/密码登录，Cookie 会话 |
| 连接配置 | host, port, username, password |

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

**问题**：
- `get_torrents()`, `pause_torrent()`, `resume_torrent()` 未实现（`pass`）
- 每次实例化都重新加载配置（`load_config()`）
- 连接失败直接抛出异常，无重试机制

### 4.5 Transmission 适配器

**文件**: [transmission.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/downloaders/transmission.py)

| 项目 | 详情 |
|------|------|
| SDK | `transmission-rpc` 7.0.11 |
| 认证方式 | Basic Auth (username/password) |
| 连接配置 | host, port, path, timeout |

```python
@register_downloader(DownloaderType.TRANSMISSION)
class TransmissionDownloader(BaseDownloader):
    def connect(self):
        self.client = transmission_rpc.Client(
            username=self.config.get("TRANSMISSION_USER", "admin"),
            password=self.config.get("TRANSMISSION_PASSWORD", "password"),
            host=self.config.get("TRANSMISSION_HOST", "127.0.0.1"),
            port=self.config.get("TRANSMISSION_PORT", 9091),
            path=self.config.get("TRANSMISSION_PATH", "/transmission/rpc"),
            timeout=self.config.get("TRANSMISSION_TIME_OUT", 5000),
            logger=log
        )
```

**特点**：
- 完整实现了所有抽象方法（add/get/pause/resume）
- 支持自定义 RPC 路径
- 传入 logger 参数用于 SDK 内部日志
- timeout 单位为毫秒（5000ms = 5s）

### 4.6 Aria2 适配器

**文件**: [aria2.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/downloaders/aria2.py)

| 项目 | 详情 |
|------|------|
| SDK | `aria2p` 0.12.1 |
| 协议 | JSON-RPC over HTTP/WebSocket |
| 连接配置 | host, port |

```python
@register_downloader(DownloaderType.ARIA2)
class Aria2Downloader(BaseDownloader):
    def connect(self):
        self.client = aria2p.API(aria2p.Client(
            host=self.config.get("ARIA2_HOST", "http://localhost"),
            port=self.config.get("ARIA2_PORT", 6800)))
```

**特点**：
- 使用 `aria2p.API` + `aria2p.Client` 双层封装
- 默认端口 6800（Aria2 JSON-RPC 标准端口）
- 完整实现了所有抽象方法

### 4.7 QBittorrentManager（测试辅助）

```python
class QBittorrentManager:
    def __init__(self, api_client: qbittorrentapi.Client | None = None):
        self.api_client = api_client or qbittorrentapi.Client()

    def connect_qbit(self) -> bool:
        self.api_client.auth_log_in()
        return self.api_client.is_logged_in()

    def add_torrent(self, url: str, category: str, name: str):
        return self.api_client.torrents_add(urls=url, category=category, rename=name, tags=[])

    def remove_torrent(self, torrent_hash: str):
        return self.api_client.torrents_delete(torrent_hashes=torrent_hash, delete_files=False)

    def get_all_torrents(self):
        return self.api_client.torrents_info()
```

**对比 QBittorrentDownloader**：

| 特性 | QBittorrentDownloader | QBittorrentManager |
|------|----------------------|-------------------|
| 用途 | 生产环境 | 单元测试 |
| 添加种子 | 仅 URL | URL + 分类 + 重命名 |
| 删除种子 | 未实现 | 已实现 |
| 获取种子 | 未实现 | 已实现 |
| 依赖注入 | 无 | 支持外部注入 api_client |

### 4.8 三大下载器对比

| 特性 | qBittorrent | Transmission | Aria2 |
|------|-------------|--------------|-------|
| SDK | qbittorrent-api | transmission-rpc | aria2p |
| 协议 | HTTP REST API | JSON-RPC | JSON-RPC |
| 认证 | Cookie (用户名/密码) | Basic Auth | 无（默认） |
| 默认端口 | 8080 | 9091 | 6800 |
| add_torrent | ✅ | ✅ | ✅ |
| get_torrents | ❌ (pass) | ✅ | ✅ |
| pause_torrent | ❌ (pass) | ✅ | ✅ |
| resume_torrent | ❌ (pass) | ✅ | ✅ |
| 完整度 | 25% | 100% | 100% |

---

## 5. PT 站点适配器体系

### 5.1 架构设计

PT 站点适配器采用**抽象基类 + 工厂模式**设计：

```
BaseTracker (ABC)
    ├── search_torrents(keyword, page, page_size) -> Optional[Dict]
    ├── get_torrent_details(torrent_id) -> Optional[Dict]
    └── get_download_link(torrent_id) -> Optional[str]

MTeamTracker(BaseTracker)      ← M-Team (kp.m-team.cc)
DicMusicTracker(BaseTracker)   ← DicMusic (dicmusic.com)
CarptTracker(BaseTracker)      ← Carpt (carpt.net)
PTSKitTracker(BaseTracker)     ← PTSKit (ptskit.com)
```

### 5.2 BaseTracker 抽象基类

**文件**: [common.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/trackers/common.py)

```python
class BaseTracker(ABC):
    @abstractmethod
    def search_torrents(self, keyword: str, page: int = 1, page_size: int = 5) -> Optional[Dict[str, Any]]:
        """搜索种子"""

    @abstractmethod
    def get_torrent_details(self, torrent_id: str) -> Optional[Dict[str, Any]]:
        """获取种子详情"""

    @abstractmethod
    def get_download_link(self, torrent_id: str) -> Optional[str]:
        """获取下载链接"""
```

### 5.3 Tracker 工厂

**文件**: [trackers/__init__.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/trackers/__init__.py)

```python
TRACKERS = {
    "mteam": MTeamTracker,
    "dicmusic": DicMusicTracker,
    "carpt": CarptTracker,
    "ptskit": PTSKitTracker,
}

def get_tracker_by_name(name: str):
    tracker_class = TRACKERS.get(name.lower())
    if tracker_class:
        return tracker_class()
    else:
        raise ValueError(f"不支持的 Tracker : {name}")
```

### 5.4 MTeamTracker

**文件**: [mteam.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/trackers/mteam.py)

| 项目 | 详情 |
|------|------|
| 站点 | M-Team (馒头) |
| 基础 URL | `https://kp.m-team.cc` |
| 认证方式 | x-api-key Header |
| HTTP 库 | requests |

#### API 端点

| 功能 | 端点 | 方法 |
|------|------|------|
| 搜索种子 | `/api/torrent/search` | POST |
| 种子详情 | `/api/torrent/detail` | POST |
| 下载链接 | `/api/torrent/genDlToken` | POST |

#### 搜索实现

```python
def search_torrents(self, keyword: str, page: int = 1, page_size: int = 5) -> Optional[Dict[str, Any]]:
    url = f"{self.base_url}/api/torrent/search"
    params = {
        "keyword": keyword,
        "pageNumber": page,
        "pageSize": page_size,
    }
    response = self.session.post(url, json=params, timeout=20)
    response.raise_for_status()
    data = response.json()
    if data.get("message", "").upper() != 'SUCCESS' or "data" not in data:
        return None
    return data["data"]
```

**响应格式**：
```json
{
    "message": "SUCCESS",
    "data": {
        "list": [...],
        "total": 100,
        "pageNumber": 1,
        "totalPages": 20
    }
}
```

#### MTeamManager（测试辅助）

```python
class MTeamManager:
    def __init__(self, api_client=None) -> None:
        self.api_client = api_client

    def get_torrent_details(self, torrent_id: str):
        return self.api_client.get_torrent_details(torrent_id)

    def get_torrent_download_url(self, torrent_id: str):
        result = self.api_client.get_torrent_download_url(torrent_id)
        if isinstance(result, dict) and "data" in result:
            return result["data"]
        return result

    def se_(self, *args, **kwargs):  # 向后兼容
        return self.search_torrents_by_keyword(*args, **kwargs)
```

### 5.5 四大站点适配器对比

| 特性 | M-Team | DicMusic | Carpt | PTSKit |
|------|--------|----------|-------|--------|
| 基础 URL | kp.m-team.cc | dicmusic.com | carpt.net | ptskit.com |
| 认证方式 | x-api-key | x-api-key | x-api-key | x-api-key |
| 搜索端点 | /api/torrent/search | /api/torrent/search | /api/torrent/search | /api/torrent/search |
| 详情端点 | /api/torrent/detail | /api/torrent/detail | /api/torrent/detail | /api/torrent/detail |
| 下载端点 | /api/torrent/genDlToken | /api/torrent/genDlToken | /api/torrent/genDlToken | /api/torrent/genDlToken |
| 超时时间 | 20s | 20s | 20s | 20s |
| 代码差异 | 有 MTeamManager | 无 | 无 | 无 |

### 5.6 代码重复问题

**严重问题**：四个 Tracker 的代码几乎完全相同，仅 `base_url` 和类名不同。这是典型的代码重复（DRY 违反）。

**重复代码统计**：
- `search_torrents()`: 4 份完全相同的实现
- `get_torrent_details()`: 4 份完全相同的实现
- `get_download_link()`: 4 份完全相同的实现
- 总计约 210 行重复代码

**建议重构方案**：

```python
class NexusPHPTracker(BaseTracker):
    """NexusPHP 站点通用适配器"""
    def __init__(self, api_key, base_url):
        self.api_key = api_key
        self.base_url = base_url
        self.session = requests.Session()
        if self.api_key:
            self.session.headers.update({"x-api-key": self.api_key})
    
    # 通用实现...

class MTeamTracker(NexusPHPTracker):
    def __init__(self, api_key=None):
        super().__init__(api_key, "https://kp.m-team.cc")
```

---

## 6. Telegram Bot 交互系统

### 6.1 架构设计

```
┌─────────────────────────────────────────┐
│           Telegram Bot 架构              │
│                                         │
│  bot.py (入口)                           │
│    └── start_bot(bot_token, core_mgr)   │
│         ├── Application.builder()       │
│         ├── bot_data["core_manager"]    │
│         └── setup_application()         │
│                                         │
│  updater.py (命令注册)                   │
│    └── setup_application(app, token)    │
│         ├── CommandHandler("start")     │
│         ├── CommandHandler("help")      │
│         ├── CommandHandler("add")       │
│         ├── CommandHandler("qbtasks")   │
│         ├── CommandHandler("cancel")    │
│         └── MessageHandler(unknown)     │
│                                         │
│  handler.py (命令处理)                   │
│    ├── start()                          │
│    ├── help_command()                   │
│    ├── add_task()                       │
│    ├── qbtasks()                        │
│    └── cancel()                         │
└─────────────────────────────────────────┘
```

### 6.2 Bot 启动

**文件**: [bot.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/bots/telegram/bot.py)

```python
def start_bot(bot_token: str, core_manager: CoreManager):
    try:
        asyncio.get_event_loop()
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)

    application = Application.builder().token(bot_token).build()
    application.bot_data["core_manager"] = core_manager
    setup_application(application, bot_token)
    application.run_polling()
```

**关键设计**：
- 使用 `python-telegram-bot` v22.1 的 `Application` API
- `bot_data` 字典存储 `core_manager` 引用，供 handler 访问
- 事件循环兼容处理（避免 "no current event loop" 错误）
- 使用 `run_polling()` 轮询模式（非 Webhook）

### 6.3 命令注册

**文件**: [updater.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/bots/telegram/updater.py)

```python
def setup_application(application: Application, bot_token: str):
    application.add_handler(CommandHandler("start", start))
    application.add_handler(CommandHandler("help", help_command))
    application.add_handler(CommandHandler("add", add_task))
    application.add_handler(CommandHandler("qbtasks", qbtasks))
    application.add_handler(CommandHandler("cancel", cancel))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_unknown))
```

### 6.4 命令处理器

**文件**: [handler.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/bots/telegram/handler.py)

#### /start 命令

```python
async def start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    user = update.effective_user
    await update.message.reply_text(
        f"您好，{user.mention_html()}！欢迎使用我们的自动化下载工具。",
        parse_mode="HTML"
    )
```

#### /add 命令

```python
async def add_task(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not context.args:
        await update.message.reply_text("⚠️ 请输入 M-Team ID，例如: /add 12345")
        return

    mt_id = context.args[0]
    core_manager = context.bot_data["core_manager"]
    success = core_manager.execute_task("download", {"torrent_id": mt_id})
```

**问题**：`core_manager.execute_task()` 方法在 CoreManager 中不存在！实际定义的是 `execute_download_task()`。

#### /qbtasks 命令

```python
async def qbtasks(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    core_manager = context.bot_data["core_manager"]
    tasks = core_manager.execute_task("get_current_tasks", {})
```

**问题**：同样调用了不存在的 `execute_task()` 方法。

#### /cancel 命令

```python
async def cancel(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    core_manager = context.bot_data["core_manager"]
    success = core_manager.execute_task("cancel_current_task", {})
```

**问题**：同上。

### 6.5 命令集汇总

| 命令 | 功能 | 参数 | 状态 |
|------|------|------|------|
| `/start` | 欢迎消息 | 无 | ✅ 可用 |
| `/help` | 帮助信息 | 无 | ✅ 可用 |
| `/add [ID]` | 添加下载任务 | M-Team 种子 ID | ❌ 方法缺失 |
| `/qbtasks` | 查看当前任务 | 无 | ❌ 方法缺失 |
| `/cancel` | 取消当前任务 | 无 | ❌ 方法缺失 |

### 6.6 安全性分析

| 问题 | 描述 | 严重程度 |
|------|------|----------|
| 无用户认证 | 任何知道 bot_token 的人都可以操作 | 🔴 严重 |
| 无权限控制 | 所有用户拥有相同权限 | 🔴 严重 |
| chat_id 未验证 | 配置了 `TG_ALLOWED_CHAT_IDS` 但未在 handler 中校验 | 🟡 中等 |

---

## 7. 定时任务调度系统

### 7.1 架构设计

```
┌──────────────────────────────────────────┐
│         定时任务调度架构                    │
│                                          │
│  start_scheduler.py (启动入口)            │
│    └── start_all_tasks()                 │
│         ├── TaskScheduler()              │
│         ├── scheduler.start()            │
│         ├── add_task(example, interval)  │
│         └── add_task(example, cron)      │
│                                          │
│  scheduler.py (调度器)                    │
│    └── TaskScheduler                     │
│         ├── BackgroundScheduler          │
│         ├── add_task()                   │
│         ├── remove_task()                │
│         ├── list_jobs()                  │
│         └── _job_listener()              │
│                                          │
│  tasks.py (任务定义)                      │
│    ├── example_task()                    │
│    └── task_with_error()                 │
└──────────────────────────────────────────┘
```

### 7.2 TaskScheduler

**文件**: [scheduler.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/tasks/scheduler.py)

```python
class TaskScheduler:
    def __init__(self):
        self.scheduler = BackgroundScheduler()
        self.scheduler.add_listener(
            self._job_listener,
            EVENT_JOB_EXECUTED | EVENT_JOB_ERROR
        )

    def add_task(self, func, trigger, **kwargs):
        self.scheduler.add_job(func, trigger, **kwargs)

    @staticmethod
    def _job_listener(event):
        if event.exception:
            logger.error(f"任务 '{event.job_id}' 执行失败！")
        else:
            logger.info(f"任务 '{event.job_id}' 执行成功。")
```

**特点**：
- 基于 APScheduler 的 `BackgroundScheduler`
- 支持三种触发器：`interval`（间隔）、`cron`（定时）、`date`（一次性）
- 事件监听器监控任务执行结果
- 后台线程运行，不阻塞主线程

### 7.3 任务定义

**文件**: [tasks.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/tasks/tasks.py)

```python
def example_task():
    notifier: Notifier = TelegramNotifier(
        bot_token="YOUR_BOT_TOKEN",
        chat_id="YOUR_CHAT_ID"
    )
    notifier.send_message("任务执行完成！")

def task_with_error():
    try:
        raise ValueError("任务发生了一个错误")
    except Exception as e:
        notifier: Notifier = TelegramNotifier(
            bot_token="YOUR_BOT_TOKEN",
            chat_id="YOUR_CHAT_ID"
        )
        notifier.send_message(f"任务失败: {str(e)}")
```

**问题**：
- Bot Token 和 Chat ID 硬编码为占位符
- 仅包含示例任务，无实际业务逻辑
- 每次执行都创建新的 TelegramNotifier 实例
- 未集成到主启动流程中

### 7.4 任务启动

**文件**: [start_scheduler.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/tasks/start_scheduler.py)

```python
def start_all_tasks():
    scheduler = TaskScheduler()
    scheduler.start()
    scheduler.add_task(example_task, 'interval', minutes=10)
    scheduler.add_task(example_task, 'cron', hour=8, minute=0)
    scheduler.list_jobs()
    return scheduler
```

**问题**：
- `start_all_tasks()` 未在 `run.py` 中被调用
- 定时任务调度器未集成到主流程

### 7.5 APScheduler 触发器类型

| 触发器 | 用途 | 示例 |
|--------|------|------|
| `interval` | 固定间隔执行 | `minutes=10`（每10分钟） |
| `cron` | Cron 表达式 | `hour=8, minute=0`（每天8:00） |
| `date` | 一次性执行 | `run_date='2025-01-01 00:00:00'` |

---

## 8. 数据库设计与持久化

### 8.1 数据库架构

```
┌─────────────────────────────────────────────┐
│              数据库层架构                      │
│                                             │
│  connection.py (连接管理)                     │
│    └── create_connection() → sqlite3.Connection │
│                                             │
│  models.py (表结构定义)                       │
│    └── create_tables()                       │
│         ├── torrents 表                      │
│         ├── tasks 表                         │
│         └── users 表                         │
│                                             │
│  operations.py (CRUD 操作)                   │
│    ├── insert_torrent()                      │
│    ├── get_torrent_by_hash()                 │
│    └── update_task_status()                  │
│                                             │
│  setup.py (初始化入口)                        │
│    └── init_db()                             │
└─────────────────────────────────────────────┘
```

### 8.2 连接管理

**文件**: [connection.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/db/connection.py)

```python
DB_PATH = config.get('DB_PATH', 'data/torrentbotx.db')

def create_connection():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn
```

**问题**：
- 每次调用都创建新连接，无连接池
- 连接未使用 `with` 上下文管理器
- `row_factory = sqlite3.Row` 支持列名访问

### 8.3 表结构

#### torrents 表

```sql
CREATE TABLE IF NOT EXISTS torrents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    hash TEXT NOT NULL UNIQUE,
    category TEXT,
    state TEXT,
    added_on INTEGER,
    progress REAL,
    ratio REAL
)
```

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK, AUTO | 自增主键 |
| name | TEXT | NOT NULL | 种子名称 |
| hash | TEXT | NOT NULL, UNIQUE | 种子哈希 |
| category | TEXT | - | 分类 |
| state | TEXT | - | 状态 |
| added_on | INTEGER | - | 添加时间（Unix 时间戳） |
| progress | REAL | - | 下载进度（0.0-1.0） |
| ratio | REAL | - | 分享率 |

#### tasks 表

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    torrent_id INTEGER,
    status TEXT,
    scheduled_time INTEGER,
    FOREIGN KEY (torrent_id) REFERENCES torrents(id)
)
```

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK, AUTO | 自增主键 |
| torrent_id | INTEGER | FK → torrents.id | 关联种子 |
| status | TEXT | - | 任务状态 |
| scheduled_time | INTEGER | - | 计划执行时间 |

#### users 表

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT,
    chat_id INTEGER
)
```

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PK, AUTO | 自增主键 |
| username | TEXT | - | 用户名 |
| chat_id | INTEGER | - | Telegram Chat ID |

### 8.4 CRUD 操作

**文件**: [operations.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/db/operations.py)

| 操作 | 方法 | 参数 |
|------|------|------|
| 插入种子 | `insert_torrent()` | name, hash, category, state, added_on, progress, ratio |
| 查询种子 | `get_torrent_by_hash()` | hash |
| 更新任务状态 | `update_task_status()` | task_id, status |

**缺失操作**：
- 删除种子/任务
- 批量插入
- 列表查询（分页）
- 用户 CRUD
- 事务管理

### 8.5 ER 关系图

```
┌──────────────────┐       ┌──────────────────┐
│    torrents      │       │      tasks       │
├──────────────────┤       ├──────────────────┤
│ id (PK)          │←──┐   │ id (PK)          │
│ name             │   │   │ torrent_id (FK)  │──┘
│ hash (UNIQUE)    │   │   │ status           │
│ category         │   │   │ scheduled_time   │
│ state            │   │   └──────────────────┘
│ added_on         │   │
│ progress         │   │   ┌──────────────────┐
│ ratio            │   │   │      users       │
└──────────────────┘   │   ├──────────────────┤
                       │   │ id (PK)          │
                       │   │ username         │
                       │   │ chat_id          │
                       │   └──────────────────┘
                       │
                       └── 一对多关系：一个种子可有多个任务
```

---

## 9. 配置管理系统

### 9.1 架构设计

```
┌─────────────────────────────────────────────┐
│              配置管理架构                      │
│                                             │
│  config.yaml (用户配置)                       │
│    └── YAML 格式配置文件                      │
│                                             │
│  example.yaml (示例配置)                      │
│    └── 配置模板，首次运行自动复制               │
│                                             │
│  config.py (配置加载器)                       │
│    ├── PTItem (Pydantic 模型)                 │
│    ├── Settings (Pydantic BaseSettings)      │
│    ├── Config (配置包装器)                     │
│    └── load_config() (便捷函数)               │
└─────────────────────────────────────────────┘
```

### 9.2 Settings 模型

**文件**: [config.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/config/config.py)

```python
class Settings(BaseSettings):
    # qBittorrent 配置
    QBIT_HOST: str = "localhost"
    QBIT_PORT: int = 8080
    QBIT_USERNAME: str = "admin"
    QBIT_PASSWORD: str = "adminadmin"
    QBIT_VERIFY_CERT: bool = True
    QBIT_REQUESTS_ARGS: Dict[str, Any] = {"timeout": [10, 30]}

    # Telegram 配置
    TG_BOT_TOKEN_MT: str = ""
    TG_ALLOWED_CHAT_IDS: str = ""
    TG_BOT_TOKEN: str = ""
    TG_BOT_TOKEN_MONITOR: str = ""
    TG_CHAT_ID: str = ""
    TG_MAX_DELETED_ITEMS_IN_REPORT: int = 20

    # 下载器配置
    DOWNLOADERS: str = "qbittorrent"

    # PT 站点配置
    PT_SITES: List[PTItem] = []

    # 通用配置
    LOG_LEVEL: str = "INFO"
    DB_PATH: str = "torrentbotx.db"
    MT_HOST: str = "https://m-team.cc"
    MT_APIKEY: str = ""
    USE_IPV6_DOWNLOAD: bool = False
    LOCAL_TIMEZONE: str = "Asia/Shanghai"
```

### 9.3 配置加载流程

```python
class Config:
    def __init__(self, config_file=None):
        self.config_file = Path(config_file or os.getenv("TORRENTBOTX_CONFIG", DEFAULT_CONFIG_PATH))
        data = self._load_yaml(self.config_file)
        self.settings = Settings(**data)
        self._validate_config()
```

**加载优先级**：
1. 构造函数参数 `config_file`
2. 环境变量 `TORRENTBOTX_CONFIG`
3. 默认路径 `config/config.yaml`

**配置验证**：
```python
def _validate_config(self) -> None:
    required_keys = ["QBIT_HOST", "QBIT_PORT", "QBIT_USERNAME", "QBIT_PASSWORD", "TG_BOT_TOKEN"]
    for key in required_keys:
        if not getattr(self.settings, key, None):
            logger.warning("配置文件缺少必需的键：%s", key)
```

**注意**：仅打印警告，不阻止启动。

### 9.4 配置项分类

| 分类 | 配置项 | 数量 |
|------|--------|------|
| qBittorrent | QBIT_HOST/PORT/USERNAME/PASSWORD/VERIFY_CERT/REQUESTS_ARGS | 6 |
| Telegram | TG_BOT_TOKEN_MT/ALLOWED_CHAT_IDS/BOT_TOKEN/BOT_TOKEN_MONITOR/CHAT_ID/MAX_DELETED | 6 |
| 下载器 | DOWNLOADERS | 1 |
| PT 站点 | PT_SITES | 1 |
| M-Team | MT_HOST/MT_APIKEY | 2 |
| 通用 | LOG_LEVEL/DB_PATH/USE_IPV6_DOWNLOAD/LOCAL_TIMEZONE | 4 |
| **总计** | | **20** |

### 9.5 配置热重载

```python
def reload(self) -> None:
    data = self._load_yaml(self.config_file)
    self.settings = Settings(**data)

def save(self) -> None:
    with open(self.config_file, "w", encoding="utf-8") as fh:
        yaml.dump(self.settings.dict(), fh, default_flow_style=False, allow_unicode=True)
```

### 9.6 问题分析

| 问题 | 描述 | 严重程度 |
|------|------|----------|
| Telegram Token 冗余 | 有 3 个不同的 Token 配置，用途不清晰 | 🟡 中等 |
| PT_SITES 未使用 | 配置了 PT_SITES 列表但 CoreManager 未使用 | 🟡 中等 |
| Transmission/Aria2 配置缺失 | Settings 中无 Transmission 和 Aria2 的配置项 | 🔴 严重 |
| 配置验证不严格 | 仅警告不阻止，可能导致运行时错误 | 🟡 中等 |
| 敏感信息明文 | 密码和 API Key 以明文存储在 YAML 中 | 🟡 中等 |

---

## 10. 通知系统

### 10.1 架构设计

```
Notifier (ABC)
    └── send_message(message: str)

TelegramNotifier(Notifier)
    ├── Bot(token=bot_token)
    ├── send_message(message)     ← 同步接口
    └── _send_message(message)    ← 异步实现
```

### 10.2 Notifier 抽象基类

**文件**: [notifier.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/notifications/notifier.py)

```python
class Notifier(ABC):
    @abstractmethod
    def send_message(self, message: str):
        pass
```

### 10.3 TelegramNotifier

**文件**: [telegram_notifier.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/notifications/telegram_notifier.py)

```python
class TelegramNotifier(Notifier):
    def __init__(self, bot_token: str, chat_id: str):
        self.bot = Bot(token=bot_token)
        self.chat_id = chat_id

    def send_message(self, message):
        asyncio.run(self._send_message(message))

    async def _send_message(self, message: str):
        try:
            await self.bot.send_message(chat_id=self.chat_id, text=message)
        except TelegramError as e:
            logger.error(f"发送 Telegram 消息失败: {e}")
```

**问题**：
- `asyncio.run()` 在已有事件循环时会报错
- `chat_id` 参数类型为 `str`，但 Telegram API 需要 `int` 或 `str`
- 无重试机制
- 无消息格式化支持（Markdown/HTML）

### 10.4 通知场景

| 场景 | 触发点 | 消息内容 |
|------|--------|----------|
| 系统启动 | CoreManager.start() | "CoreManager 启动完成 ✅" |
| 下载成功 | execute_download_task() | "部分下载器已成功添加任务: {id}" |
| 下载失败 | execute_download_task() | "所有下载器添加任务失败: {id}" |
| 定时任务 | example_task() | "任务执行完成！" |
| 任务错误 | task_with_error() | "任务失败: {error}" |

---

## 11. 日志系统

### 11.1 架构设计

**文件**: [logger.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/utils/logger.py)

```
Logger
    ├── _logger_cache (缓存)
    ├── _set_handlers()
    │   ├── StreamHandler (控制台, INFO)
    │   └── RotatingFileHandler (文件, DEBUG)
    └── get_logger()

get_logger(log_name) ← 全局便捷函数
```

### 11.2 日志配置

| 配置 | 控制台 | 文件 |
|------|--------|------|
| 级别 | INFO | DEBUG |
| 格式 | `%(asctime)s - %(levelname)s - %(message)s` | `%(asctime)s - %(name)s - %(levelname)s - %(message)s` |
| 输出 | stdout | `torrentbotx/logs/app.log` |
| 轮转 | - | 10MB × 3 备份 |
| 编码 | - | UTF-8 |

### 11.3 日志缓存机制

```python
_logger_cache = {}

class Logger:
    def __init__(self, log_name='torrentbotx'):
        if log_name in _logger_cache:
            self.logger = _logger_cache[log_name]
        else:
            self.logger = logging.getLogger(log_name)
            self._set_handlers()
            _logger_cache[log_name] = self.logger
```

**设计**：单例模式，同名 logger 只创建一次，避免重复 handler。

### 11.4 问题分析

| 问题 | 描述 |
|------|------|
| loguru 未使用 | requirements.txt 中引入了 loguru，但实际使用标准 logging |
| 日志目录硬编码 | `LOG_DIR` 相对于 `__file__` 计算 |
| 无结构化日志 | 纯文本格式，无 JSON 日志支持 |
| 无日志级别动态调整 | 配置了 `LOG_LEVEL` 但未在 Logger 中使用 |

---

## 12. 工具模块

### 12.1 Utility 类

**文件**: [utility.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/utils/utility.py)

#### format_bytes - 文件大小格式化

```python
@staticmethod
def format_bytes(size: int) -> str:
    units = ["B", "KB", "MB", "GB", "TB", "PB"]
    value = float(size)
    for unit in units:
        if value < 1024 or unit == units[-1]:
            return f"{value:.1f} {unit}" if unit != "B" else f"{int(value)} {unit}"
        value /= 1024
```

**示例**：
| 输入 | 输出 |
|------|------|
| 0 | `0 B` |
| 1024 | `1.0 KB` |
| 1048576 | `1.0 MB` |
| 1073741824 | `1.0 GB` |

#### is_valid_torrent_hash - Hash 验证

```python
@staticmethod
def is_valid_torrent_hash(hash_str: str) -> bool:
    if not hash_str:
        return False
    return (
        len(hash_str) in {16, 32, 40}
        and re.fullmatch(r"[A-Za-z0-9]+", hash_str) is not None
    )
```

**支持的 Hash 长度**：
- 16 字符：短 Hash
- 32 字符：MD5 Hash
- 40 字符：SHA-1 Hash（标准 BitTorrent info_hash）

### 12.2 string_utils 模块

**文件**: [string_utils.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/utils/string_utils.py)

| 函数 | 功能 | 示例 |
|------|------|------|
| `to_snake_case(s)` | 驼峰 → 下划线 | `camelCase` → `camel_case` |
| `to_camel_case(s)` | 下划线 → 小驼峰 | `snake_case` → `snakeCase` |
| `to_pascal_case(s)` | 下划线 → 大驼峰 | `snake_case` → `SnakeCase` |
| `remove_whitespace(s)` | 去除所有空白 | `a b c` → `abc` |
| `normalize_whitespace(s)` | 压缩空白 | `a  b   c` → `a b c` |
| `is_blank(s)` | 判断空白 | `""` → True |

### 12.3 模块导出

```python
# utils/__init__.py
from torrentbotx.utils.logger import get_logger
from torrentbotx.utils.string_utils import (
    is_blank, normalize_whitespace, remove_whitespace,
    to_camel_case, to_pascal_case, to_snake_case,
)
from torrentbotx.utils.utility import Utility
```

---

## 13. 数据模型层

### 13.1 模型概览

```
┌─────────────────────────────────────────────┐
│              数据模型层                       │
│                                             │
│  Task (任务模型)                              │
│    ├── task_id: str                          │
│    ├── name: str                             │
│    ├── status: str                           │
│    ├── priority: int                         │
│    └── torrent_id: Optional[str]             │
│                                             │
│  Torrent (种子模型)                           │
│    ├── torrent_id: str                       │
│    ├── name: str                             │
│    ├── size: int                             │
│    ├── status: str                           │
│    └── category: Optional[str]               │
│                                             │
│  User (用户模型)                              │
│    ├── user_id: str                          │
│    ├── username: str                         │
│    └── permissions: List[str]                │
│                                             │
│  Category (分类模型)                          │
│    ├── category_id: str                      │
│    └── name: str                             │
└─────────────────────────────────────────────┘
```

### 13.2 模型设计模式

所有模型均采用相同的设计模式：

```python
class ModelName:
    def __init__(self, ...):
        # 属性赋值
    
    def to_dict(self) -> dict:
        # 序列化为字典
    
    @staticmethod
    def from_dict(data: dict) -> 'ModelName':
        # 从字典反序列化
```

**特点**：
- 手动实现序列化/反序列化（未使用 Pydantic）
- `to_dict()` 和 `from_dict()` 配对使用
- 类型注解完整

### 13.3 模型与数据库映射

| 模型 | 数据库表 | 映射情况 |
|------|----------|----------|
| Task | tasks | ⚠️ 字段不完全匹配 |
| Torrent | torrents | ⚠️ 字段不完全匹配 |
| User | users | ⚠️ 字段不完全匹配 |
| Category | - | ❌ 无对应表 |

**字段不匹配详情**：

| 模型字段 | 数据库字段 | 差异 |
|----------|-----------|------|
| Task.task_id | tasks.id | 类型不同（str vs INTEGER） |
| Task.priority | - | 数据库无此字段 |
| Torrent.torrent_id | torrents.id | 类型不同 |
| Torrent.size | - | 数据库无此字段 |
| User.user_id | users.id | 类型不同 |
| User.permissions | - | 数据库无此字段 |

---

## 14. 测试体系

### 14.1 测试文件

| 文件 | 测试对象 | 测试方法数 |
|------|----------|-----------|
| test_manager.py | TorrentManager | 2 |
| test_qbittorrent.py | QBittorrentManager | 4 |
| test_mteam.py | MTeamManager | 3 |
| test_telegram_bot.py | TelegramBot | 2 |
| test_utils.py | Utility | 2 |

### 14.2 测试覆盖分析

| 模块 | 测试覆盖 | 覆盖率评估 |
|------|----------|-----------|
| core/manager | TorrentManager 仅测试 add/get | 🟡 低 |
| downloaders/qbittorrent | QBittorrentManager 测试完整 | 🟢 中 |
| trackers/mteam | MTeamManager 测试详情/下载/搜索 | 🟢 中 |
| bots/telegram | 仅测试 start/help 命令 | 🟡 低 |
| utils | 仅测试 format_bytes/hash | 🟡 低 |
| downloaders/transmission | ❌ 无测试 | 🔴 零 |
| downloaders/aria2 | ❌ 无测试 | 🔴 零 |
| trackers/dicmusic | ❌ 无测试 | 🔴 零 |
| trackers/carpt | ❌ 无测试 | 🔴 零 |
| trackers/ptskit | ❌ 无测试 | 🔴 零 |
| config | ❌ 无测试 | 🔴 零 |
| db | ❌ 无测试 | 🔴 零 |
| notifications | ❌ 无测试 | 🔴 零 |
| tasks | ❌ 无测试 | 🔴 零 |

### 14.3 测试框架

- 使用 Python 内置 `unittest`
- Mock 框架使用 `unittest.mock.MagicMock`
- 无 pytest 或其他第三方测试框架
- 无测试配置文件（如 pytest.ini、setup.cfg）

### 14.4 测试问题

| 问题 | 描述 |
|------|------|
| test_telegram_bot.py 引用不存在 | 导入 `from torrentbotx.bots.telegram import TelegramBot`，但该类不存在 |
| test_utils.py 导入路径 | `from torrentbotx.utils import Utility`，需确认导出正确 |
| 无集成测试 | 所有测试均为单元测试，无端到端测试 |
| 无 CI/CD 配置 | 无 GitHub Actions、GitLab CI 等配置 |

---

## 15. 部署与容器化

### 15.1 Dockerfile

**文件**: [Dockerfile](file:///home/incast/PT-Forward/examples/torrentbotx/Dockerfile)

```dockerfile
FROM python:3.11-slim
ENV PYTHONUNBUFFERED=1
RUN apt-get update && apt-get install -y \
    curl gcc libffi-dev libssl-dev build-essential \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY . .
RUN pip install --upgrade pip && pip install -r requirements.txt
EXPOSE 8000
CMD ["python", "run.py"]
```

### 15.2 部署方式对比

| 方式 | 命令 | 说明 |
|------|------|------|
| 直接运行 | `python run.py` | 需手动安装依赖 |
| 虚拟环境 | `source venv/bin/activate && python run.py` | setup.py 自动创建 |
| Docker | `docker build -t torrentbotx . && docker run torrentbotx` | 容器化部署 |

### 15.3 setup.py 环境初始化

**文件**: [setup.py](file:///home/incast/PT-Forward/examples/torrentbotx/setup.py)

```
setup.py main()
    ├── create_venv()      ← 创建 .venv 虚拟环境
    ├── pip_install()      ← 安装依赖
    ├── check_config()     ← 复制 example.yaml → config.yaml
    └── init_db()          ← 初始化数据库表
```

### 15.4 Docker 部署问题

| 问题 | 描述 |
|------|------|
| 无 docker-compose | 缺少 docker-compose.yml，无法编排多服务 |
| 无 volume 挂载 | 数据库和配置文件未持久化 |
| EXPOSE 8000 无意义 | 项目无 HTTP 服务，仅 Bot polling |
| 缺少健康检查 | 无 HEALTHCHECK 指令 |
| 构建依赖过多 | gcc、build-essential 等可能不需要 |

---

## 16. 安全分析

### 16.1 安全问题汇总

| 类别 | 问题 | 严重程度 | 描述 |
|------|------|----------|------|
| 认证 | Telegram Bot 无用户验证 | 🔴 严重 | 任何人可与 Bot 交互 |
| 认证 | 无权限控制 | 🔴 严重 | 所有用户权限相同 |
| 认证 | chat_id 未校验 | 🟡 中等 | 配置了但未使用 |
| 数据 | 密码明文存储 | 🔴 严重 | YAML 中明文存储 qBittorrent 密码 |
| 数据 | API Key 明文存储 | 🔴 严重 | M-Team API Key 明文存储 |
| 数据 | 无数据加密 | 🟡 中等 | SQLite 数据库无加密 |
| 网络 | SSL 证书验证可选 | 🟡 中等 | QBIT_VERIFY_CERT 可关闭 |
| 网络 | 无 HTTPS 强制 | 🟢 轻微 | Tracker 请求依赖站点配置 |
| 代码 | 异常信息泄露 | 🟡 中等 | 错误消息可能包含敏感信息 |
| 代码 | SQL 注入风险低 | 🟢 轻微 | 使用参数化查询 |

### 16.2 安全建议

1. **实现 Telegram 用户认证**：验证 chat_id 是否在允许列表中
2. **敏感信息加密**：使用环境变量或密钥管理服务
3. **数据库加密**：使用 SQLCipher 替代 SQLite
4. **输入验证**：对所有用户输入进行严格校验
5. **审计日志**：记录所有关键操作

---

## 17. 性能分析

### 17.1 性能瓶颈

| 瓶颈 | 位置 | 影响 |
|------|------|------|
| 下载器每次创建新实例 | `get_downloader_instance()` | 连接开销大 |
| 数据库无连接池 | `create_connection()` | 频繁开关连接 |
| Tracker 无并发 | 顺序请求 PT 站点 | 搜索速度慢 |
| 通知同步阻塞 | `asyncio.run()` | 可能阻塞主线程 |
| 无缓存机制 | - | 重复请求相同数据 |

### 17.2 性能优化建议

1. **下载器连接池**：复用下载器实例，避免重复连接
2. **数据库连接池**：使用 `aiosqlite` 或连接池模式
3. **异步 PT 站点请求**：使用 `httpx` 异步并发请求
4. **通知异步化**：使用消息队列解耦通知
5. **添加缓存层**：Redis 或内存缓存减少重复请求

---

## 18. 设计模式分析

### 18.1 使用的设计模式

| 模式 | 应用位置 | 描述 |
|------|----------|------|
| **注册表模式** | downloaders/base.py | `_downloader_registry` 装饰器注册 |
| **抽象工厂模式** | trackers/__init__.py | `get_tracker_by_name()` 工厂方法 |
| **策略模式** | BaseDownloader/BaseTracker | 统一接口，不同实现 |
| **观察者模式** | TaskScheduler._job_listener | 任务执行事件监听 |
| **单例模式** | Logger._logger_cache | 日志实例缓存 |
| **模板方法模式** | BaseTracker | 定义算法骨架，子类实现细节 |
| **适配器模式** | 下载器/站点适配器 | 统一不同 API 的接口 |

### 18.2 模式应用评价

| 模式 | 评价 | 问题 |
|------|------|------|
| 注册表模式 | ✅ 良好 | 每次创建新实例 |
| 抽象工厂 | ✅ 良好 | 无参数传递 |
| 策略模式 | ✅ 良好 | 接口定义清晰 |
| 观察者模式 | ✅ 良好 | 仅日志记录 |
| 单例模式 | ✅ 良好 | 无线程安全保护 |
| 适配器模式 | ⚠️ 一般 | Tracker 代码重复 |

---

## 19. 项目优缺点评估

### 19.1 优点

| 优点 | 描述 |
|------|------|
| 🏗️ 架构清晰 | 分层模块化，职责明确 |
| 🔌 多下载器支持 | QB/TR/Aria2 三合一 |
| 🌐 多站点支持 | 四大 PT 站点适配 |
| 🤖 Telegram Bot | 远程管理便捷 |
| ⏰ 定时任务 | APScheduler 灵活调度 |
| 📦 容器化 | Docker 部署支持 |
| 🔧 配置管理 | Pydantic 验证 + YAML |
| 📝 代码规范 | 类型注解完整 |
| 🧪 测试覆盖 | 核心模块有单元测试 |
| 📖 文档完善 | README 详细 |

### 19.2 缺点

| 缺点 | 描述 | 严重程度 |
|------|------|----------|
| 🔴 方法缺失 | handler 调用 `execute_task()` 但 CoreManager 未定义 | 🔴 严重 |
| 🔴 代码大量重复 | 四个 Tracker 实现完全相同 | 🔴 严重 |
| 🔴 QBittorrent 不完整 | get/pause/resume 方法未实现 | 🔴 严重 |
| 🔴 定时任务未集成 | `start_all_tasks()` 未在 run.py 中调用 | 🔴 严重 |
| 🟡 配置不一致 | Settings 缺少 Transmission/Aria2 配置项 | 🟡 中等 |
| 🟡 模型与数据库脱节 | 模型字段与数据库表字段不匹配 | 🟡 中等 |
| 🟡 无辅种功能 | 不支持 pieces_hash 辅种 | 🟡 中等 |
| 🟡 无用户认证 | Telegram Bot 无访问控制 | 🟡 中等 |
| 🟢 测试覆盖不足 | 多个模块零测试覆盖 | 🟢 轻微 |
| 🟢 loguru 未使用 | 引入了但未实际使用 | 🟢 轻微 |

---

## 20. 改进建议

### 20.1 短期改进（1-2 周）

#### 1. 修复核心 Bug

```python
# CoreManager 中添加 execute_task 方法
class CoreManager:
    def execute_task(self, task_type: str, params: dict):
        if task_type == "download":
            return self.execute_download_task(params)
        elif task_type == "get_current_tasks":
            return self._get_current_tasks()
        elif task_type == "cancel_current_task":
            return self._cancel_current_task()
        else:
            logger.error(f"未知任务类型: {task_type}")
            return False

    def _get_current_tasks(self):
        tasks = []
        for downloader in self.downloaders:
            torrents = downloader.get_torrents()
            tasks.extend(torrents)
        return tasks

    def _cancel_current_task(self):
        # 实现取消逻辑
        pass
```

#### 2. 补全 qBittorrent 适配器

```python
class QBittorrentDownloader(BaseDownloader):
    def get_torrents(self) -> list:
        try:
            return self.client.torrents_info()
        except Exception as e:
            log.error(f"获取种子列表失败: {e}")
            return []

    def pause_torrent(self, torrent_id: str) -> bool:
        try:
            self.client.torrents_pause(torrent_hashes=torrent_id)
            return True
        except Exception as e:
            log.error(f"暂停种子失败: {e}")
            return False

    def resume_torrent(self, torrent_id: str) -> bool:
        try:
            self.client.torrents_resume(torrent_hashes=torrent_id)
            return True
        except Exception as e:
            log.error(f"恢复种子失败: {e}")
            return False
```

#### 3. 集成定时任务到主流程

```python
# run.py 中添加
from torrentbotx.tasks.start_scheduler import start_all_tasks

def main():
    config = load_config()
    init_db()
    
    notifier = TelegramNotifier(...)
    core_manager = CoreManager(config=config, notifier=notifier)
    core_manager.start()
    
    # 启动定时任务调度器
    scheduler = start_all_tasks()
    
    try:
        start_bot(config.get("TG_BOT_TOKEN_MT"), core_manager)
    finally:
        scheduler.stop()
```

### 20.2 中期改进（1-2 月）

#### 1. 重构 Tracker 体系

```python
class NexusPHPTracker(BaseTracker):
    """NexusPHP 站点通用适配器"""
    
    def __init__(self, api_key: Optional[str] = None, base_url: str = ""):
        self.api_key = api_key
        self.base_url = base_url
        self.session = requests.Session()
        if self.api_key:
            self.session.headers.update({"x-api-key": self.api_key})

    def search_torrents(self, keyword: str, page: int = 1, page_size: int = 5):
        url = f"{self.base_url}/api/torrent/search"
        # 通用实现...

    def get_torrent_details(self, torrent_id: str):
        url = f"{self.base_url}/api/torrent/detail"
        # 通用实现...

    def get_download_link(self, torrent_id: str):
        url = f"{self.base_url}/api/torrent/genDlToken"
        # 通用实现...


class MTeamTracker(NexusPHPTracker):
    def __init__(self, api_key=None):
        super().__init__(api_key, "https://kp.m-team.cc")


class DicMusicTracker(NexusPHPTracker):
    def __init__(self, api_key=None):
        super().__init__(api_key, "https://dicmusic.com")


class CarptTracker(NexusPHPTracker):
    def __init__(self, api_key=None):
        super().__init__(api_key, "https://carpt.net")


class PTSKitTracker(NexusPHPTracker):
    def __init__(self, api_key=None):
        super().__init__(api_key, "https://www.ptskit.com")
```

#### 2. 添加 Telegram 用户认证

```python
async def auth_check(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    """检查用户是否有权限操作"""
    allowed_ids = context.bot_data.get("allowed_chat_ids", "")
    user_id = str(update.effective_user.id)
    if allowed_ids and user_id not in allowed_ids.split(","):
        await update.message.reply_text("⛔ 您没有权限使用此机器人。")
        return False
    return True

# 在每个 handler 中添加检查
async def add_task(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not await auth_check(update, context):
        return
    # ... 原有逻辑
```

#### 3. 补全配置项

```python
class Settings(BaseSettings):
    # Transmission 配置
    TRANSMISSION_HOST: str = "127.0.0.1"
    TRANSMISSION_PORT: int = 9091
    TRANSMISSION_USER: str = "admin"
    TRANSMISSION_PASSWORD: str = "password"
    TRANSMISSION_PATH: str = "/transmission/rpc"
    TRANSMISSION_TIME_OUT: int = 5000

    # Aria2 配置
    ARIA2_HOST: str = "http://localhost"
    ARIA2_PORT: int = 6800
    ARIA2_SECRET: str = ""
```

### 20.3 长期改进（3-6 月）

#### 1. 添加辅种功能

```python
class ReseedManager:
    """辅种管理器"""
    
    def __init__(self, downloader, trackers, config):
        self.downloader = downloader
        self.trackers = trackers
        self.config = config
    
    def scan_torrents(self) -> List[TorrentInfo]:
        """扫描下载器中的种子"""
        pass
    
    def match_pieces_hash(self, local_hash, remote_hash) -> bool:
        """匹配 pieces_hash"""
        pass
    
    def auto_reseed(self, site_name: str):
        """自动辅种"""
        pass
```

#### 2. 添加 Web 管理界面

```python
# 使用 FastAPI 提供 Web 界面
from fastapi import FastAPI

app = FastAPI(title="TorrentBotX")

@app.get("/api/torrents")
async def list_torrents():
    pass

@app.post("/api/torrents/add")
async def add_torrent(torrent_id: str):
    pass

@app.get("/api/tasks")
async def list_tasks():
    pass
```

#### 3. 数据库迁移到 ORM

```python
# 使用 SQLAlchemy 或 Tortoise ORM
from sqlalchemy import Column, Integer, String, Float, ForeignKey
from sqlalchemy.ext.declarative import declarative_base

Base = declarative_base()

class Torrent(Base):
    __tablename__ = 'torrents'
    id = Column(Integer, primary_key=True, autoincrement=True)
    name = Column(String, nullable=False)
    hash = Column(String, nullable=False, unique=True)
    category = Column(String)
    state = Column(String)
    added_on = Column(Integer)
    progress = Column(Float)
    ratio = Column(Float)
```

---

## 21. 完整文件结构

```
torrentbotx/
├── .gitignore                          # Git 忽略规则
├── Dockerfile                          # Docker 构建文件
├── LICENSE                             # MIT 许可证
├── README.md                           # 项目说明文档
├── requirements.txt                    # Python 依赖列表
├── run.py                              # 主入口文件
├── setup.py                            # 环境初始化脚本
├── data/
│   └── .gitkeep                        # 数据目录占位
├── tests/
│   ├── __init__.py                     # 测试包初始化
│   ├── test_manager.py                 # TorrentManager 测试
│   ├── test_mteam.py                   # MTeamManager 测试
│   ├── test_qbittorrent.py             # QBittorrentManager 测试
│   ├── test_telegram_bot.py            # TelegramBot 测试
│   └── test_utils.py                   # Utility 测试
└── torrentbotx/
    ├── __init__.py                     # 包初始化，导出核心类
    ├── core/
    │   ├── __init__.py
    │   └── manager.py                  # CoreManager + TorrentManager
    ├── bots/
    │   └── telegram/
    │       ├── __init__.py
    │       ├── bot.py                  # Bot 启动入口
    │       ├── handler.py              # 命令处理器
    │       └── updater.py              # 命令注册
    ├── config/
    │   ├── __init__.py
    │   ├── config.py                   # 配置加载器 (Pydantic)
    │   └── example.yaml               # 示例配置
    ├── db/
    │   ├── __init__.py
    │   ├── connection.py               # 数据库连接
    │   ├── models.py                   # 表结构定义
    │   ├── operations.py               # CRUD 操作
    │   └── setup.py                    # 数据库初始化
    ├── downloaders/
    │   ├── __init__.py                 # 导出 get_downloader_instance
    │   ├── base.py                     # BaseDownloader + 注册表
    │   ├── qbittorrent.py              # qBittorrent 适配器
    │   ├── transmission.py             # Transmission 适配器
    │   └── aria2.py                    # Aria2 适配器
    ├── enums/
    │   ├── __init__.py
    │   └── downloader_type.py          # 下载器类型枚举
    ├── models/
    │   ├── __init__.py
    │   ├── category.py                 # 分类模型
    │   ├── task.py                     # 任务模型
    │   ├── torrent.py                  # 种子模型
    │   └── user.py                     # 用户模型
    ├── notifications/
    │   ├── __init__.py
    │   ├── notifier.py                 # Notifier 抽象基类
    │   └── telegram_notifier.py        # Telegram 通知器
    ├── tasks/
    │   ├── __init__.py
    │   ├── scheduler.py                # TaskScheduler (APScheduler)
    │   ├── start_scheduler.py          # 任务启动入口
    │   └── tasks.py                    # 任务定义
    ├── trackers/
    │   ├── __init__.py                 # Tracker 工厂
    │   ├── common.py                   # BaseTracker 抽象基类
    │   ├── carpt.py                    # Carpt 适配器
    │   ├── dicmusic.py                 # DicMusic 适配器
    │   ├── mteam.py                    # M-Team 适配器
    │   └── ptskit.py                   # PTSKit 适配器
    └── utils/
        ├── __init__.py                 # 工具模块导出
        ├── logger.py                   # 日志系统
        ├── string_utils.py             # 字符串工具
        └── utility.py                  # 通用工具类
```

---

## 22. API 接口规范

### 22.1 PT 站点 API

#### 搜索种子

```
POST /api/torrent/search
Content-Type: application/json
Header: x-api-key: {api_key}

Request:
{
    "keyword": "搜索关键词",
    "pageNumber": 1,
    "pageSize": 5
}

Response:
{
    "message": "SUCCESS",
    "data": {
        "list": [
            {
                "id": "12345",
                "name": "种子名称",
                "smallDescr": "简短描述",
                "size": 1073741824,
                "category": "Movies"
            }
        ],
        "total": 100,
        "pageNumber": 1,
        "totalPages": 20
    }
}
```

#### 获取种子详情

```
POST /api/torrent/detail
Content-Type: application/x-www-form-urlencoded
Header: x-api-key: {api_key}

Request:
id=12345

Response:
{
    "message": "SUCCESS",
    "data": {
        "id": "12345",
        "name": "种子名称",
        "smallDescr": "简短描述",
        "size": 1073741824,
        "category": "Movies",
        "seeders": 10,
        "leechers": 2,
        "snatched": 50
    }
}
```

#### 获取下载链接

```
POST /api/torrent/genDlToken
Content-Type: application/x-www-form-urlencoded
Header: x-api-key: {api_key}

Request:
id=12345

Response:
{
    "message": "SUCCESS",
    "data": "https://kp.m-team.cc/download.php?id=12345&token=xxx"
}
```

### 22.2 下载器 API

#### qBittorrent

| 操作 | 方法 | 端点 |
|------|------|------|
| 登录 | POST | /api/v2/auth/login |
| 添加种子 | POST | /api/v2/torrents/add |
| 获取种子列表 | POST | /api/v2/torrents/info |
| 暂停种子 | POST | /api/v2/torrents/pause |
| 恢复种子 | POST | /api/v2/torrents/resume |
| 删除种子 | POST | /api/v2/torrents/delete |

#### Transmission

| 操作 | 方法 | RPC 方法 |
|------|------|----------|
| 获取种子 | POST | torrent-get |
| 添加种子 | POST | torrent-add |
| 停止种子 | POST | torrent-stop |
| 启动种子 | POST | torrent-start |
| 删除种子 | POST | torrent-remove |

#### Aria2

| 操作 | 方法 | RPC 方法 |
|------|------|----------|
| 添加种子 | POST | aria2.addTorrent |
| 获取下载 | POST | aria2.tellActive |
| 暂停下载 | POST | aria2.pause |
| 恢复下载 | POST | aria2.unpause |
| 删除下载 | POST | aria2.remove |

### 22.3 Telegram Bot 命令

| 命令 | 参数 | 功能 | 响应格式 |
|------|------|------|----------|
| `/start` | 无 | 欢迎消息 | HTML |
| `/help` | 无 | 帮助信息 | HTML |
| `/add` | M-Team ID | 添加下载任务 | 纯文本 |
| `/qbtasks` | 无 | 查看当前任务 | 纯文本 |
| `/cancel` | 无 | 取消当前任务 | 纯文本 |

---

## 23. 数据库关系图

### 23.1 实体关系

```
┌─────────────────────────────────────────────────────────────────┐
│                      数据库 ER 图                                │
│                                                                 │
│  ┌──────────────────────┐         ┌──────────────────────┐      │
│  │      torrents        │         │       tasks          │      │
│  ├──────────────────────┤         ├──────────────────────┤      │
│  │ *id    INTEGER (PK)  │◄───────┐│ *id    INTEGER (PK)  │      │
│  │  name   TEXT (NN)    │        ││  torrent_id (FK)     │──────┘
│  │  hash   TEXT (NN,UQ) │        ││  status  TEXT        │
│  │  category TEXT       │        ││  scheduled_time INT  │
│  │  state   TEXT        │        └──────────────────────┘      │
│  │  added_on INTEGER    │                                      │
│  │  progress REAL       │        ┌──────────────────────┐      │
│  │  ratio    REAL       │        │       users          │      │
│  └──────────────────────┘        ├──────────────────────┤      │
│                                  │ *id    INTEGER (PK)  │      │
│                                  │  username TEXT       │      │
│                                  │  chat_id  INTEGER    │      │
│                                  └──────────────────────┘      │
│                                                                 │
│  关系:                                                          │
│  tasks.torrent_id → torrents.id (一对多)                         │
│  users 独立表，无外键关联                                         │
└─────────────────────────────────────────────────────────────────┘
```

### 23.2 索引设计

| 表 | 索引 | 类型 | 用途 |
|------|------|------|------|
| torrents | id | PRIMARY | 主键索引 |
| torrents | hash | UNIQUE | 去重 + 快速查询 |
| tasks | id | PRIMARY | 主键索引 |
| tasks | torrent_id | FOREIGN KEY | 关联查询 |
| users | id | PRIMARY | 主键索引 |

---

## 24. 与其他项目对比

### 24.1 四大 PT 工具对比

| 特性 | TorrentBotX | ptdog | Reseed-backend | Reseed-Puppy-PHP |
|------|-------------|-------|----------------|------------------|
| 语言 | Python | Go | Python | PHP |
| 框架 | 无 | 无 | Flask | ThinkPHP |
| 下载器 | QB/TR/Aria2 | QB/TR | QB/TR | QB/TR |
| PT 站点 | 4 个 | 多个 | 多个 | 多个 |
| 辅种功能 | ❌ | ✅ | ✅ | ✅ |
| Web 界面 | ❌ | ❌ | ✅ | ✅ |
| Telegram Bot | ✅ | ❌ | ❌ | ❌ |
| 定时任务 | ✅ | ✅ | ✅ | ✅ |
| 数据库 | SQLite | 无 | MySQL | MySQL/SQLite |
| 容器化 | ✅ | ✅ | ❌ | ❌ |
| 用户认证 | ❌ | ❌ | ✅ | ✅ |
| 代码完整度 | 60% | 90% | 85% | 95% |

### 24.2 定位差异

| 项目 | 核心定位 | 目标用户 |
|------|----------|----------|
| TorrentBotX | Telegram Bot 远程管理 | 需要移动端管理的用户 |
| ptdog | 自动辅种 | 追求自动化的用户 |
| Reseed-backend | Web 管理辅种 | 需要可视化的用户 |
| Reseed-Puppy-PHP | 全功能辅种平台 | 需要完整方案的用户 |

---

## 附录

### 附录 A：依赖版本详情

```
anyio==4.9.0
APScheduler==3.11.0
aria2p==0.12.1
certifi==2025.4.26
charset-normalizer==3.4.2
h11==0.16.0
httpcore==1.0.9
httpx==0.28.1
idna==3.10
loguru==0.7.3
packaging==25.0
platformdirs==4.3.8
pydantic==2.11.7
pydantic-settings==2.9.1
python-dotenv==1.1.0
python-telegram-bot==22.1
PyYAML==6.0.2
qbittorrent-api==2025.5.0
requests==2.32.4
sniffio==1.3.1
transmission-rpc==7.0.11
typing_extensions==4.13.2
tzlocal==5.3.1
urllib3==2.5.0
websocket-client==1.8.0
```

### 附录 B：部署指南

#### 方式一：直接运行

```bash
# 1. 克隆项目
git clone https://github.com/astralwaveorg/torrentbotx.git
cd torrentbotx

# 2. 初始化环境
python3 setup.py

# 3. 激活虚拟环境
source venv/bin/activate

# 4. 编辑配置
vim torrentbotx/config/config.yaml

# 5. 启动服务
python run.py
```

#### 方式二：Docker 部署

```bash
# 1. 构建镜像
docker build -t torrentbotx .

# 2. 运行容器
docker run -d \
    --name torrentbotx \
    -v $(pwd)/config.yaml:/app/torrentbotx/config/config.yaml \
    -v $(pwd)/data:/app/data \
    torrentbotx
```

### 附录 C：常见问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| Bot 无响应 | Token 配置错误 | 检查 TG_BOT_TOKEN_MT |
| 下载器连接失败 | 网络或认证问题 | 检查下载器配置和日志 |
| 数据库错误 | 文件权限问题 | 检查 data/ 目录权限 |
| 定时任务不执行 | 未集成到主流程 | 手动调用 start_all_tasks() |
| Tracker 搜索失败 | API Key 无效 | 检查站点 API Key |

### 附录 D：术语表

| 术语 | 英文 | 含义 |
|------|------|------|
| PT | Private Tracker | 私有种子站 |
| 种子 | Torrent | BT 下载的元数据文件 |
| 辅种 | Cross-seeding | 在不同站点添加相同内容的种子 |
| info_hash | Info Hash | 种子的唯一标识（SHA-1） |
| pieces_hash | Pieces Hash | 种子内容的分片哈希 |
| NexusPHP | NexusPHP | PT 站点常用的 PHP 框架 |
| QB | qBittorrent | 开源 BT 下载客户端 |
| TR | Transmission | 轻量级 BT 下载客户端 |
| Aria2 | Aria2 | 多协议下载工具 |
| APScheduler | Advanced Python Scheduler | Python 定时任务框架 |
| Bot | Robot | Telegram 机器人 |

### 附录 E：版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v0.1.0 | 2025 | 初始版本，基础功能框架 |

---

## 总结

TorrentBotX 是一个**架构设计优秀但实现不完整**的 PT 自动化管理平台。项目在架构层面展现了良好的设计思路：

**架构亮点**：
- 分层模块化设计，职责清晰
- 注册表模式实现下载器热插拔
- 抽象基类统一适配器接口
- Pydantic 配置验证
- APScheduler 定时任务

**核心问题**：
- 多处关键方法缺失（`execute_task`、QBittorrent 方法）
- 代码大量重复（四个 Tracker 几乎相同）
- 定时任务未集成到主流程
- 模型层与数据库层脱节
- 无用户认证和权限控制

**项目成熟度评估**：约 60%，属于**早期开发阶段**。架构框架已搭建，但核心功能尚未完善，不适合直接用于生产环境。需要完成 Bug 修复、功能补全和安全加固后方可投入使用。

与同系列项目相比，TorrentBotX 的独特价值在于 **Telegram Bot 远程管理**，这是其他三个项目所不具备的。如果能够完善核心功能并添加辅种支持，将成为一个非常有竞争力的 PT 管理工具。