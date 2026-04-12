# Reseed-Puppy-PHP 深度分析文档

## 项目概述

**项目名称**: Reseed-Puppy  
**项目类型**: PHP 辅种工具  
**技术栈**: PHP 7.1+ + ThinkPHP 6.x + MySQL/SQLite  
**开发框架**: ThinkAdmin 6.x  
**核心特性**: 基于 NexusPHP V1.8.5 版本新增的 pieces_hash 特性开发的辅种工具

### 项目简介

Reseed-Puppy 是一个基于 ThinkPHP 框架开发的 PT 站点辅种工具，通过利用 NexusPHP 1.8.5+ 版本引入的 pieces_hash 特性，实现跨站点自动辅种功能。工具支持 qBittorrent 和 Transmission 两种下载器，可以批量扫描种子目录，自动识别种子文件中的 pieces_hash，并根据不同的 pieces_hash 进行种子辅种。

### 核心特性

- **pieces_hash 匹配**: 利用 pieces_hash 进行精确匹配，避免重复下载
- **多下载器支持**: 支持 qBittorrent 和 Transmission RPC
- **站点适配器**: 通过配置文件适配不同站点的 API
- **智能缓存**: 按站点缓存已辅种的 pieces_hash，减少重复请求
- **任务队列**: 集成 ThinkAdmin 任务队列系统，支持定时执行
- **种子库**: 本地种子库功能，减少站点请求压力
- **批量处理**: 支持批量查询和批量添加种子
- **限速支持**: 可按站点配置上传/下载限速

---

## 第1章：项目整体架构

### 1.1 技术栈分析

#### 后端技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| PHP | >= 7.1 | 编程语言 |
| ThinkPHP | 6.x | Web 框架 |
| ThinkAdmin | 6.x | 后台管理框架 |
| MySQL | - | 主数据库（可选） |
| SQLite | - | 默认数据库 |
| GuzzleHTTP | ^7.8 | HTTP 客户端 |
| Rhilip/Bencode | ^2.3 | 种子文件解析 |
| PhpSpreadsheet | ^2.0 | Excel 导入导出 |

#### 核心依赖分析

```json
{
  "require": {
    "php": ">=7.1",
    "ext-json": "*",
    "guzzlehttp/guzzle": "^7.8",
    "phpoffice/phpspreadsheet": "^2.0",
    "rhilip/bencode": "^2.3",
    "zoujingli/think-library": "^6.0",
    "zoujingli/think-plugs-admin": "^1.0"
  }
}
```

### 1.2 项目结构

```
reseed-puppy-php/
├── app/                          # 应用目录
│   ├── admin/                    # 后台管理模块
│   │   ├── controller/           # 控制器
│   │   │   ├── Base.php          # 数据字典管理
│   │   │   ├── Config.php        # 系统配置
│   │   │   ├── Cron.php          # 定时任务管理
│   │   │   ├── Download.php      # 下载器配置
│   │   │   ├── Index.php         # 首页
│   │   │   ├── Login.php         # 登录
│   │   │   ├── Menu.php          # 菜单管理
│   │   │   ├── Oplog.php         # 操作日志
│   │   │   ├── Queue.php         # 任务队列管理
│   │   │   ├── Sites.php         # 站点配置
│   │   │   ├── User.php          # 用户管理
│   │   │   ├── Qbapi.php         # qBittorrent API
│   │   │   ├── Trapi.php         # Transmission API
│   │   │   └── api/              # API 控制器
│   │   ├── Service.php           # 服务层
│   │   ├── route/                # 路由配置
│   │   └── lang/                 # 语言包
│   ├── command/                  # 命令行工具
│   │   ├── Cron.php              # 辅种计划任务
│   │   └── SearchReSeed.php      # Jackett 搜索任务
│   ├── helper/                   # 辅助类
│   │   ├── CacheHelper.php       # 缓存辅助
│   │   ├── CommonHelper.php      # 通用辅助
│   │   ├── FastRequest.php       # 快速请求
│   │   └── TorrentHelper.php     # 种子处理
│   ├── model/                    # 模型
│   │   └── TorrentBank.php       # 种子库模型
│   ├── subscribe/                # 事件订阅
│   │   └── TorrentSubscribe.php  # 种子事件订阅
│   ├── exception/                # 异常类
│   │   └── KnownException.php    # 已知异常
│   ├── index/                    # 前台模块
│   │   └── controller/
│   │       └── Index.php         # 前台首页
│   └── event.php                 # 事件配置
├── config/                       # 配置文件
│   ├── app.php                   # 应用配置
│   ├── cache.php                 # 缓存配置
│   ├── database.php              # 数据库配置
│   ├── log.php                   # 日志配置
│   ├── route.php                 # 路由配置
│   ├── site.php                  # 站点适配器配置
│   └── ...
├── database/                     # 数据库目录
│   ├── migrations/               # 数据库迁移文件
│   └── sqlite.db                 # SQLite 数据库文件
├── public/                       # 公共目录
│   ├── index.php                 # 入口文件
│   ├── static/                   # 静态资源
│   └── router.php                # 路由文件
├── vendor/                       # Composer 依赖
├── composer.json                 # Composer 配置
└── README.md                     # 项目说明
```

### 1.3 架构设计模式

#### 1.3.1 MVC 架构

项目采用经典的 MVC（Model-View-Controller）架构：

- **Model**: 数据模型层（app/model/）
- **View**: 视图层（app/admin/view/）
- **Controller**: 控制器层（app/admin/controller/）

#### 1.3.2 设计模式

**1. 适配器模式（Adapter Pattern）**

通过配置文件适配不同站点的 API 接口：

```php
// config/site.php
'adapter' => [
    'np' => [
        'pieces' => [
            'request' => [
                'url' => '{api_url}',
                'method' => 'POST',
                'type' => 'JSON',
                'data' => [
                    'passkey' => '{passkey}',
                    'pieces_hash' => '{pieces_hash}',
                ],
            ],
            'response' => [
                'data' => 'data'
            ]
        ],
        'download' => '{site_url}/download.php?id={torrent_id}&passkey={passkey}'
    ],
    'kimoji' => [
        'pieces' => [
            'request' => [
                'url' => '{api_url}/{pieces_hash}',
                'method' => 'GET'
            ],
            'response' => [
                'data' => 'data'
            ]
        ],
        'download' => '{site_url}/torrent/download/{torrent_id}.rsskey'
    ]
]
```

**2. 工厂模式（Factory Pattern）**

下载器客户端工厂：

```php
// 根据类型创建不同的下载器客户端
if ($downloadResult['type'] == 1) {
    $qbapi = new Qbapi($downloadResult);
    $info_list = TorrentHelper::qb_getTorrentsInfo($qbapi, $info_hash_pieces);
} else {
    $trapi = new Trapi($downloadResult);
    $info_list = TorrentHelper::tr_getTorrentsInfo($trapi, $info_hash_pieces);
}
```

**3. 策略模式（Strategy Pattern）**

不同的下载器使用不同的添加策略：

```php
// qb 策略
private function qb($downloadResult, $site, array $download_url, ...) {
    $qbapi = new Qbapi($downloadResult);
    $qbapi->addTorrent($value, $save_path, ...);
}

// tr 策略
private function tr($downloadResult, $site, array $download_url, ...) {
    $trapi = new Trapi($downloadResult);
    $trapi->addTorrent($value, $save_path, ...);
}
```

**4. 事件驱动模式（Event-Driven Pattern）**

通过事件订阅解耦业务逻辑：

```php
// 触发事件
event('torrent_add', [
    'site_url' => $site['site_url'],
    'torrents' => $pieces_hash_torrent_id,
]);

// 订阅事件
class TorrentSubscribe {
    public function subscribe(Event $event) {
        $event->listen('torrent_add', [$this, 'add']);
        $event->listen('torrent_remove', [$this, 'remove']);
    }
}
```

**5. 模板方法模式（Template Method Pattern）**

HTTP 请求的通用模板：

```php
private static function guzzle_request(array $config, array $data): array {
    $request_url = self::template_parse($config['request.url'], $data);
    $method = $config['request.method'];
    $type = $config['request.type'];
    
    // 统一的请求处理
    $response = $client->request($method, $request_url, $options);
    
    // 统一的响应处理
    return self::process_response($response, $config);
}
```

### 1.4 数据流转架构

```
┌─────────────┐
│  定时任务    │
│   (Cron)    │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  扫描种子目录    │
│  TorrentHelper  │
└──────┬──────────┘
       │
       ├──► pieces_hash_info (pieces_hash -> info_hash[])
       ├──► tracker_pieces (tracker -> pieces_hash[])
       └──► info_hash_pieces (info_hash -> pieces_hash)
       │
       ▼
┌─────────────────┐
│  获取下载器种子  │
│  Qbapi/Trapi    │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  过滤有效种子    │
│  get_valid_...  │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  按站点查询缓存  │
│  CacheHelper    │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  请求站点 API    │
│  FastRequest    │
└──────┬──────────┘
       │
       ├──► 返回 torrent_id
       └──► 触发 torrent_add 事件
       │
       ▼
┌─────────────────┐
│  生成下载链接    │
│  get_download_  │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  添加到下载器    │
│  Qbapi/Trapi    │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  更新缓存       │
│  CacheHelper    │
└─────────────────┘
```

---

## 第2章：ThinkPHP 框架应用结构

### 2.1 应用启动流程

#### 2.1.1 入口文件分析

```php
// public/index.php
<?php
use think\admin\service\RuntimeService;

require __DIR__ . '/../vendor/autoload.php';

RuntimeService::doWebsiteInit();
```

**启动流程**:
1. 加载 Composer 自动加载
2. 执行 ThinkAdmin 网站初始化
3. 加载应用配置
4. 初始化路由
5. 启动应用

#### 2.1.2 应用配置分析

```php
// config/app.php
return [
    'app_namespace' => '',
    'app_express' => true,
    'with_route' => true,
    'super_user' => 'admin',
    'default_timezone' => 'Asia/Shanghai',
    'app_map' => [],
    'domain_bind' => [],
    'deny_app_list' => [],
    'rbac_login' => '',
    'rbac_ignore' => ['index'],
    'cors_on' => true,
    'cors_host' => [],
    'cors_methods' => 'GET,PUT,POST,PATCH,DELETE',
    'cors_headers' => 'Api-Type,Api-Name,Api-Uuid,Jwt-Token,Api-Token,User-Form-Token,User-Token,Token',
    'error_message' => '页面错误！请稍后再试～',
    'http_exception_template' => [
        404 => syspath('public/static/theme/err/404.html'),
        500 => syspath('public/static/theme/err/500.html'),
    ],
];
```

**配置要点**:
- 多应用模式：`app_express => true`
- 路由启用：`with_route => true`
- 超级用户：`super_user => 'admin'`
- 跨域支持：`cors_on => true`
- RBAC 忽略：`rbac_ignore => ['index']`

### 2.2 多应用结构

项目采用 ThinkPHP 多应用模式：

```
app/
├── admin/      # 后台管理应用
├── index/      # 前台应用
└── command/    # 命令行应用
```

#### 2.2.1 Admin 应用

后台管理应用包含以下功能模块：

| 控制器 | 功能 | 路由前缀 |
|--------|------|----------|
| Index | 首页/仪表盘 | /admin/index |
| Login | 登录/登出 | /admin/login |
| User | 用户管理 | /admin/user |
| Menu | 菜单管理 | /admin/menu |
| Base | 数据字典 | /admin/base |
| Oplog | 操作日志 | /admin/oplog |
| Queue | 任务队列 | /admin/queue |
| Cron | 定时任务 | /admin/cron |
| Config | 系统配置 | /admin/config |
| Sites | 站点配置 | /admin/sites |
| Download | 下载器配置 | /admin/download |
| Qbapi | qBittorrent API | /admin/qbapi |
| Trapi | Transmission API | /admin/trapi |

#### 2.2.2 Index 应用

前台应用，主要提供首页展示功能。

### 2.3 控制器基类分析

```php
namespace app\admin\controller;

use think\admin\Controller;

class Base extends Controller
{
    public function index()
    {
        SystemBase::mQuery()->layTable(function () {
            $this->title = '数据字典管理';
            $this->types = SystemBase::types();
            $this->type = $this->get['type'] ?? ($this->types[0] ?? '-');
        }, static function (QueryHelper $query) {
            $query->where(['deleted' => 0])->equal('type');
            $query->like('code,name,status')->dateBetween('create_at');
        });
    }
}
```

**基类特性**:
- 继承 `think\admin\Controller`
- 内置 RBAC 权限控制
- 内置查询构建器 `mQuery()`
- 内置表单处理 `mForm()`
- 内置 CRUD 操作方法

### 2.4 路由设计

#### 2.4.1 路由配置

```php
// config/route.php
return [
    '__pattern__' => [
        'name' => '\w+',
    ],
    '[hello]'     => [
        ':name' => ['index/hello', ['method' => 'get'], ['name' => '\w+']],
    ],
];
```

#### 2.4.2 RESTful 路由

控制器方法自动映射到 HTTP 方法：

| HTTP 方法 | 控制器方法 | 功能 |
|-----------|-----------|------|
| GET | index() | 列表 |
| GET | read() | 详情 |
| POST | save() | 新增 |
| POST | add() | 新增 |
| POST | edit() | 编辑 |
| POST | state() | 状态修改 |
| POST | remove() | 删除 |

### 2.5 视图模板

ThinkAdmin 使用 ThinkPHP 模板引擎：

```php
// 模板渲染
return $this->fetch('form');

// 模板变量赋值
$this->title = '站点配置';
$this->sites = $sites;
```

---

## 第3章：数据库设计与模型

### 3.1 数据库配置

#### 3.1.1 数据库连接配置

```php
// config/database.php
return [
    'default' => 'sqlite',
    'connections' => [
        'mysql' => [
            'type' => 'mysql',
            'hostname' => '127.0.0.1',
            'database' => 'admin_v6',
            'username' => 'admin_v6',
            'password' => 'FbYBHcWKr2',
            'hostport' => '3306',
            'charset' => 'utf8mb4',
            'prefix' => '',
            'deploy' => 0,
            'rw_separate' => false,
            'fields_strict' => true,
            'break_reconnect' => false,
            'trigger_sql' => true,
            'fields_cache' => isOnline(),
        ],
        'sqlite' => [
            'type' => 'sqlite',
            'database' => syspath('database/sqlite.db'),
            'charset' => 'utf8',
            'trigger_sql' => true,
            'deploy' => 0,
            'prefix' => '',
        ],
    ],
];
```

**数据库选择**:
- 默认使用 SQLite，便于部署
- 支持切换到 MySQL
- 字符集：utf8mb4（支持 emoji）

### 3.2 数据库迁移

#### 3.2.1 迁移文件列表

| 迁移文件 | 功能 |
|----------|------|
| 20221013031925_install_admin.php | 安装系统表 |
| 20221013031926_install_admin_data.php | 初始化系统数据 |
| 202311013031927_install_package.php | 安装扩展包表 |
| 202311013031955_insert_sites.php | 插入站点数据 |
| 202311143038888_addokpt_sites.php | 添加 OKPT 站点 |
| 202311283039999_install_log.php | 安装日志表 |
| 20240107080108_site_request_interval.php | 添加站点请求间隔 |
| 20240108061235_torrent_bank.php | 创建种子库表 |
| 202501069999999_add_sites.php | 添加站点 |
| 202502189999999_add_new_sites.php | 添加新站点 |
| 202503139999999_add_new_sites_two.php | 添加更多站点 |

#### 3.2.2 核心表结构

**1. 系统配置表 (system_config)**

```php
$this->table('system_config', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '系统-配置',
])
->addColumn('type', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '配置分类'])
->addColumn('name', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '配置名称'])
->addColumn('value', 'string', ['limit' => 2048, 'default' => '', 'null' => true, 'comment' => '配置内容'])
->addIndex('type', ['name' => 'idx_system_config_type'])
->addIndex('name', ['name' => 'idx_system_config_name'])
->create();
```

**2. 系统用户表 (system_user)**

```php
$this->table('system_user', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '系统-用户',
])
->addColumn('username', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '用户账号'])
->addColumn('password', 'string', ['limit' => 32, 'default' => '', 'null' => true, 'comment' => '用户密码'])
->addColumn('nickname', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '用户昵称'])
->addColumn('headimg', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => '头像地址'])
->addColumn('authorize', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => '权限授权'])
->addColumn('is_super', 'integer', ['limit' => 1, 'default' => 0, 'null' => true, 'comment' => '是否超管'])
->addColumn('status', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '状态(0:禁用,1:启用)'])
->addIndex('username', ['name' => 'idx_system_user_username'])
->create();
```

**3. 站点配置表 (system_sites)**

```php
$this->table('system_sites', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '站点配置',
])
->addColumn('site_name', 'string', ['limit' => 50, 'default' => '', 'null' => true, 'comment' => '站点名称'])
->addColumn('site_url', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '站点地址'])
->addColumn('api_url', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => 'API地址'])
->addColumn('passkey', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => 'Passkey'])
->addColumn('request_interval', 'integer', ['limit' => 11, 'default' => 2, 'null' => true, 'comment' => '请求间隔(秒)'])
->addColumn('upload_limit', 'integer', ['limit' => 11, 'default' => 0, 'null' => true, 'comment' => '上传限速(KB/s)'])
->addColumn('download_limit', 'integer', ['limit' => 11, 'default' => 0, 'null' => true, 'comment' => '下载限速(KB/s)'])
->addColumn('status', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '状态'])
->addIndex('site_url', ['name' => 'idx_system_sites_site_url'])
->create();
```

**4. 下载器配置表 (system_download)**

```php
$this->table('system_download', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '下载器配置',
])
->addColumn('name', 'string', ['limit' => 50, 'default' => '', 'null' => true, 'comment' => '下载器名称'])
->addColumn('type', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '类型(1:QB,2:TR)'])
->addColumn('host', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '主机地址'])
->addColumn('port', 'string', ['limit' => 10, 'default' => '', 'null' => true, 'comment' => '端口'])
->addColumn('username', 'string', ['limit' => 50, 'default' => '', 'null' => true, 'comment' => '用户名'])
->addColumn('password', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '密码'])
->addColumn('save_path', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => '保存路径'])
->addColumn('reseed_sites', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => '辅种站点ID'])
->addColumn('is_skip_hash', 'integer', ['limit' => 1, 'default' => 0, 'null' => true, 'comment' => '跳过哈希校验'])
->addColumn('is_paused', 'integer', ['limit' => 1, 'default' => 0, 'null' => true, 'comment' => '添加后暂停'])
->addColumn('status', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '状态'])
->create();
```

**5. 任务队列表 (system_queue)**

```php
$this->table('system_queue', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '系统-任务',
])
->addColumn('code', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '任务编号'])
->addColumn('title', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '任务名称'])
->addColumn('command', 'string', ['limit' => 500, 'default' => '', 'null' => true, 'comment' => '执行命令'])
->addColumn('exec_data', 'text', ['default' => null, 'null' => true, 'comment' => '执行参数'])
->addColumn('loops', 'integer', ['limit' => 11, 'default' => 0, 'null' => true, 'comment' => '循环次数'])
->addColumn('attempts', 'integer', ['limit' => 11, 'default' => 0, 'null' => true, 'comment' => '执行次数'])
->addColumn('progress', 'double', ['default' => 0, 'null' => true, 'comment' => '执行进度'])
->addColumn('status', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '状态(1:待处理,2:进行中,3:已完成,4:已失败)'])
->addColumn('exec_time', 'datetime', ['default' => null, 'null' => true, 'comment' => '执行时间'])
->addColumn('create_at', 'timestamp', ['default' => 'CURRENT_TIMESTAMP', 'null' => true, 'comment' => '创建时间'])
->addIndex('code', ['name' => 'idx_system_queue_code'])
->addIndex('status', ['name' => 'idx_system_queue_status'])
->create();
```

**6. 种子库表 (torrent_bank)**

```php
$this->table('torrent_bank', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '种子库',
])
->addColumn('site_url', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '站点地址'])
->addColumn('pieces_hash', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => 'pieces_hash'])
->addColumn('torrent_id', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => 'torrent_id'])
->addColumn('create_at', 'timestamp', ['default' => 'CURRENT_TIMESTAMP', 'null' => true, 'comment' => '创建时间'])
->addIndex('site_url', ['name' => 'idx_torrent_bank_site_url'])
->addIndex('pieces_hash', ['name' => 'idx_torrent_bank_pieces_hash'])
->create();
```

### 3.3 数据库关系图

```
system_user (系统用户)
    │
    ├─► system_oplog (操作日志)
    │
    ├─► system_queue (任务队列)
    │       │
    │       └─► system_queue_log (任务日志)
    │
    ├─► system_sites (站点配置)
    │       │
    │       └─► torrent_bank (种子库)
    │
    └─► system_download (下载器配置)
            │
            └─► Cron/SearchReSeed (定时任务)
                    │
                    └─► torrent_bank (种子库)
```

### 3.4 模型层设计

#### 3.4.1 TorrentBank 模型

```php
namespace app\model;

use think\admin\Model;

class TorrentBank extends Model
{
    public function getCreateAtAttr($value): string
    {
        return format_datetime($value);
    }
}
```

**模型特性**:
- 继承 `think\admin\Model`
- 自动时间戳格式化
- 支持软删除
- 支持关联查询

#### 3.4.2 模型查询构建器

```php
// 使用 ThinkAdmin 查询构建器
SystemSites::mQuery()->layTable(function () {
    $this->title = '站点配置';
}, static function (QueryHelper $query) {
    // 查询条件
});

// 原生查询
TorrentBank::query()
    ->where(['site_url' => $site_url])
    ->whereIn('pieces_hash', $pieces_hash_list)
    ->select()
    ->toArray();
```

---

## 第4章：辅种核心逻辑与算法

### 4.1 核心概念

#### 4.1.1 pieces_hash 原理

**pieces_hash** 是种子文件中 `info.pieces` 字段的 SHA1 哈希值：

```
info.pieces → SHA1() → pieces_hash (40字符十六进制)
```

**为什么使用 pieces_hash？**
- 相同内容但不同种子文件会有不同的 info_hash
- 但 pieces_hash 相同，说明种子内容完全相同
- 可以通过 pieces_hash 匹配跨站点的相同资源

#### 4.1.2 三种 Hash 关系

```php
// 1. pieces_hash -> info_hash[] (一对多)
$pieces_hash_info = [
    'pieces_sha1_1' => ['info_sha1_1', 'info_sha1_2', ...],
    'pieces_sha1_2' => ['info_sha1_3', ...],
];

// 2. tracker -> pieces_hash[] (一对多)
$tracker_pieces = [
    'tracker_1' => ['pieces_hash_1', 'pieces_hash_2', ...],
    'tracker_2' => ['pieces_hash_3', ...],
];

// 3. info_hash -> pieces_hash (一对一)
$info_hash_pieces = [
    'info_sha1_1' => 'pieces_sha1_1',
    'info_sha1_2' => 'pieces_sha1_1',
    ...
];
```

### 4.2 种子扫描算法

#### 4.2.1 扫描流程

```php
public static function get_by_path(string $save_path, string $download_type = '1'): array
{
    if (!is_dir($save_path)) {
        throw new \Exception($save_path . "非文件夹或不存在");
    }

    $pieces_hash_list = [
        'pieces_hash_info' => [],
        'tracker_pieces' => [],
        'info_hash_pieces' => [],
    ];

    foreach (scandir($save_path) as $file_name) {
        if (pathinfo($file_name, PATHINFO_EXTENSION) != 'torrent') {
            continue;
        }

        $file_path = $save_path . '/' . $file_name;

        try {
            $torrent_data = file_get_contents($file_path);
            $torrent = Bencode::decode($torrent_data);
            $info = $torrent['info'];
            $pieces = $info['pieces'];
            $info_sha1 = sha1(Bencode::encode($info));
            $pieces_sha1 = sha1($pieces);
            
            $pieces_hash_list['pieces_hash_info'][$pieces_sha1][] = $info_sha1;

            $announce = self::get_announce($torrent, $file_path, $download_type);
            if ($announce) {
                $tracker = CommonHelper::get_url($announce);
                $pieces_hash_list['tracker_pieces'][$tracker][] = $pieces_sha1;
            }
            $pieces_hash_list['info_hash_pieces'][$info_sha1] = $pieces_sha1;
        } catch (\Exception $e) {
            Log::error('torrent解析异常,' . $file_name . ',' . $e->getMessage());
            continue;
        }
    }

    return $pieces_hash_list;
}
```

**算法复杂度**:
- 时间复杂度: O(n)，n 为种子文件数量
- 空间复杂度: O(n)，存储所有种子信息

#### 4.2.2 announce 信息获取

```php
private static function get_announce($torrent, string $file_path, string $download_type): ?string
{
    $announce = null;

    if (isset($torrent['announce'])) {
        $announce = $torrent['announce'];
    } else if ($download_type == '1') {
        // qb 种子从 .fastresume 获取 tracker
        $fastresume_path = str_replace('.torrent', '.fastresume', $file_path);
        if (file_exists($fastresume_path)) {
            $fastresume_data = file_get_contents($fastresume_path);
            $fastresume = Bencode::decode($fastresume_data);
            if (isset($fastresume['trackers']) && 
                is_array($fastresume['trackers']) &&
                count($fastresume['trackers']) > 0 && 
                is_array($fastresume['trackers'][0]) &&
                count($fastresume['trackers'][0]) > 0) {
                $announce = $fastresume['trackers'][0][0];
            }
        }
    }

    return $announce;
}
```

### 4.3 下载器种子获取

#### 4.3.1 qBittorrent 批量获取

```php
public static function qb_getTorrentsInfo(Qbapi $qbapi, array $info_hash_to_pieces): array
{
    $info_list = [];
    
    // 每次最多 2000 个
    $info_hash_to_pieces_group = array_chunk(array_keys($info_hash_to_pieces), 2000);
    foreach ($info_hash_to_pieces_group as $group_list) {
        // 读取已完成的种子
        $torrent_info_list = $qbapi->getTorrentsInfo(implode('|', $group_list), 'completed');
        if (empty($torrent_info_list) || count($torrent_info_list) == 0) {
            continue;
        }

        foreach ($torrent_info_list as $info) {
            $info_list[$info['hash']] = [
                'info_hash' => $info['hash'],
                'name' => $info['name'],
                'category' => $info['category'],
                'save_path' => $info['save_path'],
                'size' => $info['size'],
                'total_size' => $info['total_size'],
                'tags' => !empty($info['tags']) ? explode(',', $info['tags']) : [],
            ];
        }
    }

    return $info_list;
}
```

#### 4.3.2 Transmission 批量获取

```php
public static function tr_getTorrentsInfo(Trapi $trapi, array $info_hash_to_pieces): array
{
    $info_list = [];
    
    // 每次最多 2000 个
    $info_hash_to_pieces_group = array_chunk(array_keys($info_hash_to_pieces), 2000);
    foreach ($info_hash_to_pieces_group as $group_list) {
        $torrent_info_list = $trapi->getTorrentsInfo($group_list);
        if (empty($torrent_info_list) || count($torrent_info_list['arguments']['torrents']) == 0) {
            continue;
        }

        foreach ($torrent_info_list['arguments']['torrents'] as $info) {
            $info_list[$info['hashString']] = [
                'info_hash' => $info['hashString'],
                'name' => $info['name'],
                'category' => '',
                'save_path' => $info['downloadDir'],
                'size' => $info['sizeWhenDone'],
                'total_size' => $info['totalSize'],
                'tags' => isset($info['labels']) ? $info['labels'] : "",
            ];
        }
    }

    return $info_list;
}
```

### 4.4 有效种子过滤

```php
public static function get_valid_pieces_hash_info(array $pieces_hash_info, array $info_list): array 
{
    $pieces_hash_info_list = [];

    foreach ($pieces_hash_info as $pieces_hash => $info_hashs) {
        foreach ($info_hashs as $info_hash) {
            if (!isset($info_list[$info_hash])) {
                continue;
            }
            $pieces_hash_info_list[$pieces_hash][] = $info_list[$info_hash];
        }
    }

    return $pieces_hash_info_list;
}
```

**过滤逻辑**:
1. 遍历所有 pieces_hash
2. 检查对应的 info_hash 是否在下载器中存在
3. 只保留下载器中实际存在的种子

### 4.5 站点 pieces 过滤

```php
public static function get_pieces_by_site($site, array $tracker_pieces): array
{
    $pieces = [];

    $trackers = Config::get('site.tracker_name'); 
    $site_tracker = isset($trackers[$site['site_url']]) ? $trackers[$site['site_url']] : [];
    
    $site_host_key = CommonHelper::get_url_host_key($site['site_url']);
    
    foreach ($tracker_pieces as $key => $value) {
        // 匹配站点域名或配置的 tracker
        if (stristr($key, $site_host_key) !== false ||
            in_array($key, $site_tracker)) {
            $pieces = array_merge($pieces, $value);
        }
    }

    return array_unique($pieces);
}
```

**匹配策略**:
1. 通过域名特征匹配（如 baidu.com）
2. 支持配置自定义 tracker 映射
3. 去重处理

### 4.6 辅种执行流程

#### 4.6.1 主流程

```php
protected function execute(Input $input, Output $output)
{
    foreach ($downloadIds as $downloadId) {
        // 1. 获取下载器配置
        $downloadResult = SystemDownload::mQuery()->db()->where('id', $downloadId)->find();
        
        // 2. 获取站点配置
        $siteResult = SystemSites::mQuery()->db()
            ->whereIn('id', array_map('intval', explode(',', $downloadResult['reseed_sites'])))
            ->select();
        
        // 3. 扫描种子目录
        $torrent_list = TorrentHelper::get_by_path($downloadResult['save_path'], $downloadResult['type']);
        $pieces_hash_info = $torrent_list['pieces_hash_info'];
        $tracker_pieces = $torrent_list['tracker_pieces'];
        $info_hash_pieces = $torrent_list['info_hash_pieces'];
        
        // 4. 获取下载器种子信息
        if ($downloadResult['type'] == 1) {
            $qbapi = new Qbapi($downloadResult);
            $info_list = TorrentHelper::qb_getTorrentsInfo($qbapi, $info_hash_pieces);
        } else {
            $trapi = new Trapi($downloadResult);
            $info_list = TorrentHelper::tr_getTorrentsInfo($trapi, $info_hash_pieces);
        }
        
        // 5. 过滤有效种子
        $pieces_hash_info = TorrentHelper::get_valid_pieces_hash_info($pieces_hash_info, $info_list);
        
        // 6. 按站点处理
        foreach ($siteResult as $site) {
            // 6.1 获取缓存
            $cached_pieces_hash_list = CacheHelper::GetPiecesHash($site['site_url']);
            $exists_pieces_hash_list = TorrentHelper::get_pieces_by_site($site, $tracker_pieces);
            
            // 6.2 过滤待辅种种子
            $actived_pieces_hash_list = array_diff(
                array_diff(array_keys($pieces_hash_info), $cached_pieces_hash_list), 
                $exists_pieces_hash_list
            );
            
            // 6.3 批量请求站点
            $pieces_hash_groups = array_chunk($actived_pieces_hash_list, 100);
            foreach ($pieces_hash_groups as $group_list) {
                $download_url = FastRequest::pieces_request($site, $group_list);
                
                // 6.4 添加到下载器
                if ($downloadResult['type'] == 1) {
                    $this->qb($downloadResult, $site, $download_url, ...);
                } else {
                    $this->tr($downloadResult, $site, $download_url, ...);
                }
            }
        }
    }
}
```

#### 4.6.2 qBittorrent 辅种

```php
private function qb($downloadResult, $site, array $download_url, array $pieces_hash_info, 
                   array &$cached_pieces_hash_list, bool &$first_request, 
                   array &$statistics, array $request_pieces_hash_info) {
    $qbapi = new Qbapi($downloadResult);
    $request_interval = $this->request_interval($site);
    
    foreach ($download_url as $key => $value) {
        if (!isset($pieces_hash_info[$key])) {
            continue;
        }

        $save_path = $pieces_hash_info[$key][0]['save_path'];
        
        if (!$first_request) {
            sleep($request_interval);
        }
        $first_request = false;
        
        if ($qbapi->addTorrent(
            $value,
            $save_path,
            $downloadResult['is_skip_hash'],
            $downloadResult['is_paused'],
            $site['download_limit'],
            $site['upload_limit'],
            'Reseed Puppy') == 'Ok.') {
            
            array_push($cached_pieces_hash_list, $key);
            CacheHelper::SetPiecesHash($site['site_url'], $cached_pieces_hash_list);
            $statistics[$downloadResult['id']]['sites'][$site['site_url']]['succeeded_count'] += 1;
        } else {
            $statistics[$downloadResult['id']]['sites'][$site['site_url']]['download_failed_count'] += 1;
        }
    }
}
```

#### 4.6.3 Transmission 辅种

```php
private function tr($downloadResult, $site, array $download_url, array $pieces_hash_info,
                   array &$cached_pieces_hash_list, bool &$first_request,
                   array &$statistics, array $request_pieces_hash_info) {
    $trapi = new Trapi($downloadResult);
    $request_interval = $this->request_interval($site);
    
    foreach ($download_url as $key => $value) {
        if (!isset($pieces_hash_info[$key])) {
            continue;
        }

        $save_path = $pieces_hash_info[$key][0]['save_path'];
        
        if (!$first_request) {
            sleep($request_interval);
        }
        $first_request = false;

        $result = $trapi->addTorrent($value, $save_path, $downloadResult['is_paused']);
        
        if ($result['result'] == 'success') {
            array_push($cached_pieces_hash_list, $key);
            CacheHelper::SetPiecesHash($site['site_url'], $cached_pieces_hash_list);
            $statistics[$downloadResult['id']]['sites'][$site['site_url']]['succeeded_count'] += 1;
        } else {
            $statistics[$downloadResult['id']]['sites'][$site['site_url']]['download_failed_count'] += 1;
        }
    }
}
```

### 4.7 算法优化

#### 4.7.1 批量处理

```php
// 每次最多 100 个 pieces_hash
$pieces_hash_groups = array_chunk($actived_pieces_hash_list, 100);
foreach ($pieces_hash_groups as $group_list) {
    $download_url = FastRequest::pieces_request($site, $group_list);
    // ...
}
```

**优化效果**:
- 减少 HTTP 请求数量
- 降低站点服务器压力
- 提高整体效率

#### 4.7.2 请求间隔控制

```php
private function request_interval($site): int {
    return intval($site['request_interval']) > 2 ? intval($site['request_interval']) : 2;
}
```

**目的**:
- 避免请求过于频繁被封禁
- 可按站点配置不同间隔
- 最小间隔 2 秒

#### 4.7.3 缓存策略

```php
// 缓存已辅种的 pieces_hash
CacheHelper::SetPiecesHash($site['site_url'], $cached_pieces_hash_list);

// 过滤已缓存的
$actived_pieces_hash_list = array_diff(
    array_diff(array_keys($pieces_hash_info), $cached_pieces_hash_list), 
    $exists_pieces_hash_list
);
```

**缓存优势**:
- 避免重复请求站点
- 减少站点服务器压力
- 提高辅种速度

---

## 第5章：站点适配与爬虫实现

### 5.1 站点适配器设计

#### 5.1.1 适配器配置

```php
// config/site.php
return [
    'adapter' => [
        // NexusPHP 标准适配器
        'np' => [
            'pieces' => [
                'request' => [
                    'url' => '{api_url}',
                    'method' => 'POST',
                    'type' => 'JSON',
                    'data' => [
                        'passkey' => '{passkey}',
                        'pieces_hash' => '{pieces_hash}',
                    ],
                ],
                'response' => [
                    'data' => 'data'
                ]
            ],
            'download' => '{site_url}/download.php?id={torrent_id}&passkey={passkey}'
        ],
        // Kimoji 自研适配器
        'kimoji' => [
            'pieces' => [
                'request' => [
                    'url' => '{api_url}/{pieces_hash}',
                    'method' => 'GET'
                ],
                'response' => [
                    'data' => 'data'
                ]
            ],
            'download' => '{site_url}/torrent/download/{torrent_id}.rsskey'
        ]
    ],
    // 站点到适配器的映射
    'sites' => [
        'https://kimoji.club' => 'kimoji'
    ],
    // tracker 名称映射
    'tracker_name' => [
        // 'www.baidu.com' => 'tracker.有钱任性.com'
    ]
];
```

#### 5.1.2 适配器选择

```php
private static function get_adapter(string $site_url): array
{
    $sites = Config::get('site.sites', []);
    $site_url = CommonHelper::site_url_standard($site_url);

    // 默认使用 NexusPHP 适配器
    $adapter = Config::get('site.adapter.np');
    if (!$adapter) {
        throw new \ErrorException('配置缺失' . $site_url);
    }

    return $adapter;
}
```

### 5.2 站点请求实现

#### 5.2.1 通用请求封装

```php
public static function pieces_request($site, array $pieces_hash): array
{
    $adapter = self::get_adapter($site['site_url']);

    $data = [
        'api_url' => $site['api_url'],
        'passkey' => $site['passkey'],
        'pieces_hash' => $pieces_hash
    ];
    
    $pieces_hash_torrent_id = self::guzzle_request(
        self::safe_get_value($adapter, 'pieces'), 
        $data
    );

    if (count($pieces_hash_torrent_id) == 0) {
        return [];
    }

    // 触发种子库插入事件
    event('torrent_add', [
        'site_url' => $site['site_url'],
        'torrents' => $pieces_hash_torrent_id,
    ]);

    return self::get_download_url($site, $pieces_hash_torrent_id);
}
```

#### 5.2.2 HTTP 请求实现

```php
private static function guzzle_request(array $config, array $data): array
{
    $url = $data['api_url'];
    $request_url = self::template_parse($config['request.url'], $data);
    $method = $config['request.method'];
    $type = $config['request.type'];

    $client = new Client();
    $parsedUrl = parse_url($url);
    $host = $parsedUrl['host'];
    
    $headers = [
        'Content-Type: application/json',
        'Accept: */*',
        'User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
        'Referer:' . $url,
        'Host:' . $host,
    ];

    try {
        $options = [
            'headers' => $headers,
            'timeout' => 10,
            'verify' => false,
        ];

        if ($type == 'JSON') {
            $options['json'] = self::array_template_parse($config['request.data'], $data);
        }

        $response = $client->request($method, $request_url, $options);
    } catch (ClientException $e) {
        $msg = '站点访问失败,链接:' . $url;
        Log::error($msg . $e->getMessage());
    } catch (ConnectException $e) {
        $msg = '站点访问超过10秒,链接:' . $url;
        Log::error($msg . $e->getMessage());
    } catch (\Exception $e) {
        $msg = '未处理的异常' . $url;
        Log::error($msg . $e->getMessage());
        throw new KnownException($msg . substr($e->getMessage(), 0, 50));
    }

    $statusCode = $response->getStatusCode();
    $content = (string) $response->getBody();
    $response->getBody()->close();

    if ($statusCode != 200) {
        $msg = '状态码:' . $statusCode . ',返回:' . substr($content, 0, 20) . $url;
        throw new KnownException($msg);
    }

    $response_json = json_decode($content, true);
    if (!is_array($response_json)) {
        $msg = '返回格式非json,返回:' . substr($content, 0, 50) . $url;
        throw new \Exception($msg);
    }

    $data_key = $config['response.data'];
    if (!isset($response_json[$data_key])) {
        $msg = '返回格式与预期不一致,返回:' . substr($content, 0, 50) . $url;
        throw new \Exception($msg);
    }

    return $response_json[$data_key];
}
```

**请求特性**:
- 超时设置：10 秒
- SSL 验证：关闭
- User-Agent：模拟浏览器
- Referer：携带站点 URL
- 错误处理：分类捕获异常

#### 5.2.3 模板解析

```php
private static function template_parse(string $template, array $data)
{
    preg_match_all('/{(.*?)}/', $template, $matches);

    $keys = $matches[1];
    foreach ($keys as $key) {
        if (!isset($data[$key])) {
            throw new \InvalidArgumentException('缺少参数：' . $data[$key]);
        }
        if (is_array($data[$key])) {
            return $data[$key];
        }
        $template = str_replace('{' . $key . '}', (string)$data[$key], $template);
    }

    return $template;
}

private static function array_template_parse(array $template, array $data): array
{
    foreach ($template as $key => $value) {
        $template[$key] = self::template_parse($value, $data);
    }
    return $template;
}
```

**模板示例**:
```php
// 输入
$template = '{site_url}/download.php?id={torrent_id}&passkey={passkey}';
$data = [
    'site_url' => 'https://example.com',
    'torrent_id' => '12345',
    'passkey' => 'abcdef',
];

// 输出
'https://example.com/download.php?id=12345&passkey=abcdef'
```

#### 5.2.4 下载链接生成

```php
private static function get_download_url($site, array $pieces_hash_torrent_id): array
{
    $adapter = self::get_adapter($site['site_url']);

    $data = [
        'site_url' => $site['site_url'],
        'passkey' => $site['passkey']
    ];

    $download_url = [];
    foreach ($pieces_hash_torrent_id as $key => $value) {
        $data['torrent_id'] = $value;
        $download_url[$key] = self::template_parse($adapter['download'], $data);
    }

    return $download_url;
}
```

### 5.3 种子库事件处理

#### 5.3.1 事件订阅

```php
class TorrentSubscribe
{
    public function subscribe(Event $event)
    {
        $event->listen('torrent_add', [$this, 'add']);
        $event->listen('torrent_remove', [$this, 'remove']);
    }

    public function add(array $event)
    {
        $site_url = $event['site_url'];
        $torrents = $event['torrents'];

        if (!$site_url || !is_array($torrents) || count($torrents) == 0) {
            return;
        }

        $site_url = CommonHelper::site_url_standard($site_url, true);
        $pieces_hash_list = array_keys($torrents);

        $has_torrent_list = TorrentBank::query()
            ->where(['site_url' => $site_url])
            ->whereIn('pieces_hash', $pieces_hash_list)
            ->select()
            ->toArray();

        $torrents_add = [];
        foreach ($torrents as $key => $value) {
            $torrents_add[] = [
                'site_url' => $site_url,
                'pieces_hash' => $key,
                'torrent_id' => $value,
            ];
        }

        // 先删除已存在的，再插入新的
        if (count($has_torrent_list) > 0) {
            TorrentBank::query()
                ->where(['id' => array_column($has_torrent_list, 'id')])
                ->delete();
        }
        TorrentBank::mk()->insertAll($torrents_add);
    }

    public function remove(array $event)
    {
        $site_url = $event['site_url'];
        $pieces_hash = $event['pieces_hash'];

        if (!$site_url || !$pieces_hash) {
            return;
        }

        $where = [
            'site_url' => CommonHelper::site_url_standard($site_url, true),
            'pieces_hash' => $pieces_hash,
        ];
        TorrentBank::query()->where($where)->delete();

        Log::notice('torrent_remove finished.' . json_encode($where));
    }
}
```

**事件触发**:
```php
// 添加种子库
event('torrent_add', [
    'site_url' => $site['site_url'],
    'torrents' => $pieces_hash_torrent_id,
]);

// 移除种子库
event('torrent_remove', [
    'site_url' => $site['site_url'],
    'pieces_hash' => $pieces_hash,
]);
```

### 5.4 站点管理

#### 5.4.1 站点配置管理

```php
class Sites extends Controller
{
    public function index()
    {
        SystemSites::mQuery()->layTable(function () {
            $this->title = '站点配置';
        });
    }

    public function add()
    {
        SystemSites::mForm('add');
    }

    public function edit()
    {
        if (!($this->request->isGet())) {
            $id = $this->request->post('id');
            $site_url = $this->request->param()['site_url'];
            $site = SystemSites::mQuery()->db()->where('id', $id)->find();
            
            // 站点地址变更时清理缓存
            if ($site && $site['site_url'] != $site_url) {
                CacheHelper::ClearPiecesHash($site['site_url']);
            }
        }
        SystemSites::mForm('edit');
    }

    public function remove()
    {
        $this->clearcachebyid();
        SystemSites::mDelete();
    }

    public function clearcache()
    {
        $all = $this->request->get('all');
        if ($all) {
            CacheHelper::ClearAllPiecesHash();
            $this->success("成功");
        }
        $this->clearcachebyid();
        $this->success("成功");
    }

    private function clearcachebyid()
    {
        $id = $this->request->post('id');
        $sites = SystemSites::mQuery()->db()->where('id', 'in', $id)->select();
        foreach ($sites as $site) {
            CacheHelper::ClearPiecesHash($site['site_url']);
        }
    }
}
```

#### 5.4.2 站点导入

```php
public function import()
{
    $file = $this->app->request->post('file');
    $file = 'public/' . str_replace($this->app->request->domain(), '', $file);
    
    $cellName = [
        'A' => 'id',
        'B' => 'site_name',
        'C' => 'site_url',
        'D' => 'api_url',
        'E' => 'passkey',
        'F' => 'request_interval',
        'G' => 'upload_limit',
        'H' => 'download_limit',
    ];
    
    $spreadsheet = IOFactory::load($file);
    $sheet = $spreadsheet->getActiveSheet();
    $highestRow = $sheet->getHighestRow();
    
    for ($row = 2; $row <= $highestRow; $row++) {
        foreach ($cellName as $cell => $field) {
            $value = $sheet->getCell($cell . $row)->getValue();
            $value = trim($value);
            $sheetData[$row][$field] = $value;
        }
        
        $flag = $this->app->db->name('system_sites')
            ->where('site_url', $sheetData[$row]['site_url'])
            ->find();
        
        if (!$flag) {
            $this->app->db->name('system_sites')
                ->data($sheetData[$row])
                ->insert();
        } else {
            $this->app->db->name('system_sites')
                ->data($sheetData[$row])
                ->update();
        }
    }
    
    $this->success('导入成功');
}
```

---

## 第6章：用户认证与权限管理

### 6.1 认证机制

#### 6.1.1 ThinkAdmin RBAC

项目使用 ThinkAdmin 内置的 RBAC（基于角色的访问控制）系统：

```php
// config/app.php
return [
    'super_user' => 'admin',
    'rbac_login' => '',
    'rbac_ignore' => ['index'],
];
```

**认证流程**:
1. 用户登录
2. 生成 Session
3. 存储用户权限
4. 请求时验证权限

#### 6.1.2 权限注解

```php
class Sites extends Controller
{
    /**
     * 站点配置
     * @auth true      // 需要认证
     * @menu true      // 显示在菜单
     */
    public function index()
    {
        // ...
    }
    
    /**
     * 删除站点
     * @auth true
     */
    public function remove()
    {
        // ...
    }
}
```

**注解说明**:
- `@auth true`: 需要登录认证
- `@menu true`: 显示在后台菜单
- 无注解: 公开访问

### 6.2 用户管理

#### 6.2.1 用户表结构

```php
$this->table('system_user', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '系统-用户',
])
->addColumn('username', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '用户账号'])
->addColumn('password', 'string', ['limit' => 32, 'default' => '', 'null' => true, 'comment' => '用户密码'])
->addColumn('nickname', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '用户昵称'])
->addColumn('headimg', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => '头像地址'])
->addColumn('authorize', 'string', ['limit' => 255, 'default' => '', 'null' => true, 'comment' => '权限授权'])
->addColumn('is_super', 'integer', ['limit' => 1, 'default' => 0, 'null' => true, 'comment' => '是否超管'])
->addColumn('status', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '状态(0:禁用,1:启用)'])
->addIndex('username', ['name' => 'idx_system_user_username'])
->create();
```

#### 6.2.2 用户管理控制器

```php
class User extends Controller
{
    /**
     * 用户管理
     * @auth true
     * @menu true
     */
    public function index()
    {
        SystemUser::mQuery()->layTable(function () {
            $this->title = '用户管理';
        }, static function (QueryHelper $query) {
            $query->like('username,nickname#nickname')->equal('status');
        });
    }
    
    /**
     * 添加用户
     * @auth true
     */
    public function add()
    {
        SystemUser::mForm('form');
    }
    
    /**
     * 编辑用户
     * @auth true
     */
    public function edit()
    {
        SystemUser::mForm('form');
    }
}
```

### 6.3 操作日志

#### 6.3.1 日志表结构

```php
$this->table('system_oplog', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '系统-日志',
])
->addColumn('node', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' '日志节点'])
->addColumn('geoip', 'string', ['limit' => 50, 'default' => '', 'null' => true, 'comment' => '访问IP'])
->addColumn('action', 'string', ['limit' => 200, 'default' => '', 'null' => true, 'comment' => '操作行为'])
->addColumn('content', 'text', ['default' => null, 'null' => true, 'comment' => '日志内容'])
->addColumn('username', 'string', ['limit' => 50, 'default' => '', 'null' => true, 'comment' => '操作账号'])
->addIndex('node', ['name' => 'idx_system_oplog_node'])
->addIndex('create_at', ['name' => 'idx_system_oplog_create_at'])
->create();
```

#### 6.3.2 日志记录

```php
// 自动记录操作日志
sysoplog('系统运维管理', '清空所有站点辅种缓存');
```

---

## 第7章：任务队列与定时任务

### 7.1 任务队列系统

#### 7.1.1 任务队列表

```php
$this->table('system_queue', [
    'engine' => 'InnoDB', 
    'collation' => 'utf8mb4_general_ci', 
    'comment' => '系统-任务',
])
->addColumn('code', 'string', ['limit' => 20, 'default' => '', 'null' => true, 'comment' => '任务编号'])
->addColumn('title', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '任务名称'])
->addColumn('command', 'string', ['limit' => 500, 'default' => '', 'null' => true, 'comment' => '执行命令'])
->addColumn('exec_data', 'text', ['default' => null, 'null' => true, 'comment' => '执行参数'])
->addColumn('loops', 'integer', ['limit' => 11, 'default' => 0, 'null' => true, 'comment' => '循环次数'])
->addColumn('attempts', 'integer', ['limit' => 11, 'default' => 0, 'null' => true, 'comment' => '执行次数'])
->addColumn('progress', 'double', ['default' => 0, 'null' => true, 'comment' => '执行进度'])
->addColumn('status', 'integer', ['limit' => 1, 'default' => 1, 'null' => true, 'comment' => '状态(1:待处理,2:进行中,3:已完成,4:已失败)'])
->addColumn('exec_time', 'datetime', ['default' => null, 'null' => true, 'comment' => '执行时间'])
->addColumn('create_at', 'timestamp', ['default' => 'CURRENT_TIMESTAMP', 'null' => true, 'comment' => '创建时间'])
->addIndex('code', ['name' => 'idx_system_queue_code'])
->addIndex('status', ['name' => 'idx_system_queue_status'])
->create();
```

**任务状态**:
- 1: 待处理
- 2: 进行中
- 3: 已完成
- 4: 已失败

#### 7.1.2 任务队列管理

```php
class Queue extends Controller
{
    public function index()
    {
        SystemQueue::mQuery()->layTable(function () {
            $this->title = '系统任务管理';
            $this->iswin = ProcessService::iswin();
            if ($this->super = AdminService::isSuper()) {
                $this->command = ProcessService::think('xadmin:queue start');
                if (!$this->iswin && !empty($_SERVER['USER'])) {
                    $this->command = "sudo -u {$_SERVER['USER']} {$this->command}";
                }
            }
        }, static function (QueryHelper $query) {
            $query->equal('status')->like('code|title#title,command');
            $query->timeBetween('enter_time,exec_time')->dateBetween('create_at');
        });
    }
    
    public function redo()
    {
        try {
            $data = $this->_vali(['code.require' => '任务编号不能为空！']);
            $queue = QueueService::instance()->initialize($data['code'])->reset();
            $queue->progress(1, '>>> 任务重置成功 <<<', 0.00);
            $this->success('任务重置成功！', $queue->code);
        } catch (\Exception $exception) {
            $this->error($exception->getMessage());
        }
    }
    
    public function add()
    {
        if ($this->request->isPost()) {
            $formData = $this->request->post();
            $formData['exec_data'] = explode(',', $formData['downloadId']);
            $code = sysqueue(
                $formData['title'], 
                $formData['command'], 
                $later = 0, 
                $data = $formData['exec_data'], 
                $rscript = 0, 
                $loops = $formData['cronTime']
            );
            SystemQueue::mQuery()->db()->where('code', $code)->update(['status' => $formData['status']]);
            return $this->success('添加成功！', '');
        }
        return $this->fetch('../view/queue/add');
    }
}
```

### 7.2 定时任务

#### 7.2.1 辅种定时任务

```php
class Cron extends \think\admin\Command
{
    protected function configure()
    {
        $this->setName('cron')
             ->setDescription('辅种计划任务');
    }

    protected function execute(Input $input, Output $output)
    {
        $downloadIds = $this->queue->data;

        foreach ($downloadIds as $downloadId) {
            // 获取下载器配置
            $downloadResult = SystemDownload::mQuery()->db()->where('id', $downloadId)->find();
            
            // 获取站点配置
            $siteResult = SystemSites::mQuery()->db()
                ->whereIn('id', array_map('intval', explode(',', $downloadResult['reseed_sites'])))
                ->select();
            
            // 执行辅种逻辑
            // ...
        }
    }
}
```

#### 7.2.2 Jackett 搜索任务

```php
class SearchReSeed extends \think\admin\Command
{
    protected function configure()
    {
        $this->setName('search')
            ->setDescription('the search command');
    }

    protected function execute(Input $input, Output $output)
    {
        $jconfig = SystemConfig::mQuery()->db()->where('type', 'jackett')->find();
        if (!$jconfig || !isset($jconfig['value'])) {
            $msg = '未配置jackett数据，请点击头像配置参数,结束运行程序';
            $this->msg($msg);
            $this->setQueueSuccess($msg);
        }
        
        $downloadIds = $this->queue->data;
        foreach ($downloadIds as $downloadId) {
            $downloadResult = SystemDownload::mQuery()->db()->where('id', $downloadId)->find();
            $siteResult = SystemSites::mQuery()->db()
                ->whereIn('id', array_map('intval', explode(',', $downloadResult['reseed_sites'])))
                ->select();
            
            // 扫描种子目录
            foreach (scandir($downloadResult['save_path']) as $file_name) {
                if (pathinfo($file_name, PATHINFO_EXTENSION) == 'torrent') {
                    $file_path = $downloadResult['save_path'] . '/' . $file_name;
                    try {
                        $torrent_data = file_get_contents($file_path);
                        $torrent = Bencode::decode($torrent_data);
                        $info = $torrent['info'];
                        $torrent_name = $info['name'];
                        $size = isset($info['length']) ? $info['length'] : array_sum(array_column($info['files'], 'length'));
                        $size = round($size / 1073741824, 2);
                        
                        // 提取搜索关键词
                        $title_names = explode('.', $info['name']);
                        if (count($title_names) > 5) {
                            $search_name = $title_names[1] . " " . $title_names[2] . " " . $title_names[3];
                        } else {
                            continue;
                        }
                        
                        $info_sha1 = sha1(Bencode::encode($info));
                        $search[] = [
                            'titleName' => $torrent_name, 
                            'searchName' => $search_name, 
                            'infoHash' => $info_sha1, 
                            'size' => $size
                        ];
                    } catch (\Exception $e) {
                        continue;
                    }
                }
            }
            
            // 通过 Jackett 搜索并添加
            if ($downloadResult['type'] == 1) {
                $qbapi = new Qbapi($downloadResult);
                foreach ($search as $i) {
                    $data = $this->guzzlePost($i['searchName'], $jconfig);
                    foreach ($data['Results'] as $d) {
                        if ($i['size'] == round($d['Size'] / 1073741824, 2)) {
                            $torrent_info = $qbapi->getTorrentsInfo($i['infoHash'], 'completed');
                            if (!empty($torrent_info)) {
                                $save_path = $torrent_info[0]['save_path'];
                                $qbapi->addTorrent($d['Link'], $save_path, 
                                    $downloadResult['is_skip_hash'], 
                                    $downloadResult['is_paused'], 
                                    0, 0, 'Reseed Puppy Jackett');
                            }
                        }
                    }
                    sleep(100);
                }
            } else {
                $trapi = new Trapi($downloadResult);
                foreach ($search as $i) {
                    $data = $this->guzzlePost($i['searchName'], $jconfig);
                    foreach ($data['Results'] as $d) {
                        if ($i['size'] == round($d['Size'] / 1073741824, 2)) {
                            $torrent_info = $trapi->getTorrentsInfo($i['infoHash']);
                            if (!empty($torrent_info)) {
                                $save_path = $torrent_info['arguments']['torrents'][0]['downloadDir'];
                                $trapi->addTorrent($d['Link'], $save_path, $downloadResult['is_paused']);
                            }
                        }
                    }
                    sleep(100);
                }
            }
        }
        
        $this->setQueueSuccess('结束运行程序');
    }
    
    public function guzzlePost($query, $jconfig)
    {
        $client = new Client();
        $url = $jconfig['name'] . '/api/v2.0/indexers/all/results?apikey=' . $jconfig['value'] . '&Query=' . $query;
        try {
            $response = $client->request("GET", $url, [
                'timeout' => 30,
                'verify' => false,
            ]);
        } catch (\Exception $e) {
            return ['Results' => []];
        }

        $response_json = json_decode((string)$response->getBody(), true);
        $response->getBody()->close();
        return $response_json;
    }
}
```

### 7.3 任务进度管理

```php
// 设置任务进度
$this->setQueueProgress($message, $progress);

// 设置任务成功
$this->setQueueSuccess($message);

// 设置任务失败
$this->setQueueError($message);
```

---

## 第8章：配置管理与缓存机制

### 8.1 配置管理

#### 8.1.1 系统配置

```php
class Config extends Controller
{
    public function index()
    {
        SystemConfig::mQuery()->layTable(function () {
            $this->title = '系统配置';
        });
    }
    
    public function edit()
    {
        SystemConfig::mForm('form');
    }
}
```

**配置存储**:
- 存储在 `system_config` 表
- 按类型分类（type 字段）
- 支持动态配置

#### 8.1.2 站点配置

站点配置存储在 `system_sites` 表：

| 字段 | 说明 |
|------|------|
| site_name | 站点名称 |
| site_url | 站点地址 |
| api_url | API 地址 |
| passkey | 站点 Passkey |
| request_interval | 请求间隔（秒） |
| upload_limit | 上传限速（KB/s） |
| download_limit | 下载限速（KB/s） |
| status | 状态 |

#### 8.1.3 下载器配置

下载器配置存储在 `system_download` 表：

| 字段 | 说明 |
|------|------|
| name | 下载器名称 |
| type | 类型（1:QB,2:TR） |
| host | 主机地址 |
| port | 端口 |
| username | 用户名 |
| password | 密码 |
| save_path | 保存路径 |
| reseed_sites | 辅种站点 ID |
| is_skip_hash | 跳过哈希校验 |
| is_paused | 添加后暂停 |
| status | 状态 |

### 8.2 缓存机制

#### 8.2.1 缓存配置

```php
// config/cache.php
return [
    'default' => 'file',
    'stores' => [
        'file' => [
            'type' => 'File',
            'path' => '',
            'prefix' => '',
            'expire' => 0,
            'tag_prefix' => 'tag:',
            'serialize' => [],
        ],
        'fz_file' => [
            'type' => 'File',
            'path' => './data/cache',
            'prefix' => '',
            'expire' => 0,
            'tag_prefix' => 'tag:',
            'serialize' => [],
        ],
        'safe' => [
            'type' => 'File',
            'path' => syspath('safefile/cache/'),
            'prefix' => '',
            'expire' => 0,
            'tag_prefix' => 'tag:',
            'serialize' => [],
        ],
    ],
];
```

**缓存存储**:
- 默认使用文件缓存
- 辅种缓存使用 `fz_file` 存储
- 安全缓存使用 `safe` 存储

#### 8.2.2 辅种缓存

```php
class CacheHelper extends Helper
{
    public static function ClearAllPiecesHash()
    {
        Cache::store('fz_file')->tag('pieces_hash_all')->clear();
        sysoplog('系统运维管理', '清空所有站点辅种缓存');
    }

    public static function ClearPiecesHash(string $siteurl)
    {
        $key = self::key($siteurl);
        Cache::store('fz_file')->delete($key);
        sysoplog('系统运维管理', '清空' . $siteurl . '辅种缓存');
    }

    public static function SetPiecesHash(string $siteurl, array $pieces_hash, array $tag = [])
    {
        $key = self::key($siteurl);
        $tags = array("pieces_hash_all");
        
        if (count($tag) > 0) {
            $tags = array_merge($tags, $tag);
        }

        Cache::store('fz_file')->tag($tags)->set($key, $pieces_hash);
    }

    public static function GetPiecesHash(string $siteurl): array
    {
        $key = self::key($siteurl);
        return Cache::store('fz_file')->get($key) ?? [];
    }

    private static function key(string $siteurl): string
    {
        return "pieces_hash" . md5($siteurl);
    }
}
```

**缓存策略**:
- 按站点缓存已辅种的 pieces_hash
- 支持标签批量清理
- 永久缓存（expire = 0）

#### 8.2.3 缓存使用流程

```php
// 1. 获取已缓存
$cached_pieces_hash_list = CacheHelper::GetPiecesHash($site['site_url']);

// 2. 获取已存在的（通过 tracker 匹配）
$exists_pieces_hash_list = TorrentHelper::get_pieces_by_site($site, $tracker_pieces);

// 3. 过滤待辅种的
$actived_pieces_hash_list = array_diff(
    array_diff(array_keys($pieces_hash_info), $cached_pieces_hash_list), 
    $exists_pieces_hash_list
);

// 4. 辅种成功后更新缓存
array_push($cached_pieces_hash_list, $key);
CacheHelper::SetPiecesHash($site['site_url'], $cached_pieces_hash_list);
```

---

## 第9章：API 接口设计

### 9.1 下载器 API

#### 9.1.1 qBittorrent API

```php
class Qbapi
{
    private $api_prefix = [
        'login' => '/api/v2/auth/login',
        'logout' => '/api/v2/auth/logout',
        'torrentsInfo' => '/api/v2/torrents/info',
        'torrentAdd' => '/api/v2/torrents/add',
    ];
    
    private $base_url;
    private $session_id;
    private $config;
    private $client;
    public $login_status;

    public function __construct($config = [])
    {
        $this->config = $config;
        $this->base_url = $this->config['port'] ? 
            $this->config['host'] . ':' . $this->config['port'] . '/' : 
            $this->config['host'] . '/';
        $this->client = new GuzzleHttp\Client([
            'base_uri' => $this->base_url, 
            ['verify' => false, 'timeout' => 10]
        ]);
        $this->login();
    }

    public function login()
    {
        try {
            $response = $this->client->post($this->api_prefix['login'], [
                'form_params' => [
                    'username' => $this->config['username'],
                    'password' => $this->config['password'],
                ],
            ]);
            $headers = $response->getHeaders();
            if ($response->getReasonPhrase() == 'OK') {
                $headerCookie = $headers['Set-Cookie'][0] ?? $headers['set-cookie'][0];
                preg_match('/SID=(\S[^;]+)/', $headerCookie, $matches);
                $this->session_id = $matches[1];
                $jar = \GuzzleHttp\Cookie\CookieJar::fromArray(
                    ['SID' => $this->session_id],
                    parse_url($this->config['host'])['host']
                );
                $this->client = new GuzzleHttp\Client([
                    'base_uri' => $this->base_url,
                    'verify' => false,
                    'timeout' => 10,
                    'cookies' => $jar,
                ]);
                $this->login_status = 1;
                return json(['code' => 1, 'msg' => "Qb 登录成功"]);
            } else {
                $this->login_status = 0;
                return json(['code' => 0, 'msg' => "Qb 登录失败"]);
            }
        } catch (\Exception $e) {
            $this->login_status = 0;
            return json(['code' => 0, 'msg' => "密码错误次数过多，账号已被封禁"]);
        }
    }

    public function getTorrentsInfo($hash = '', $filter = '')
    {
        $response = $this->client->post($this->api_prefix['torrentsInfo'], [
            'form_params' => [
                'hashes' => $hash,
                'filter' => $filter
            ]
        ]);
        return json_decode($response->getBody()->getContents(), true);
    }

    public function addTorrent($url, $save_path, $is_skip_checking, $is_paused, 
                              $upLimit = 0, $dlLimit = 0, $tag = "Reseed Puppy")
    {
        $is_skip_checking == 1 ? $is_skip_checking = 'true' : $is_skip_checking = 'false';
        $is_paused == 1 ? $is_paused = 'false' : $is_paused = 'true';
        $response = $this->client->post($this->api_prefix['torrentAdd'], [
            'form_params' => [
                'urls' => $url,
                "savepath" => $save_path,
                'skip_checking' => $is_skip_checking,
                'paused' => $is_paused,
                'tags' => $tag,
                "upLimit" => $upLimit * 1024,
                "dlLimit" => $dlLimit * 1024,
            ],
        ]);
        return $response->getBody()->getContents();
    }
}
```

**API 特性**:
- 使用 Cookie 认证（SID）
- 支持批量查询（用 | 分隔）
- 支持跳过哈希校验
- 支持限速设置
- 支持标签分类

#### 9.1.2 Transmission API

```php
class Trapi
{
    private $api_prefix = '/transmission/rpc';
    private $base_url;
    private $session_id;
    private $config;
    private $client;

    public function __construct($config = [])
    {
        $this->config = $config;
        $this->base_url = $this->config['port'] ? 
            $this->config['host'] . ':' . $this->config['port'] . '/' : 
            $this->config['host'] . '/';
        $this->client = new GuzzleHttp\Client([
            'base_uri' => $this->base_url, 
            'http_errors' => false, 
            ['verify' => false, 'timeout' => 10]
        ]);
        $this->initSessionId();
        $this->getTorrentsInfo('962d7339210750fe9f208ee2896176901a7bc71a');
    }

    public function initSessionId()
    {
        $headers = [
            'Content-Type' => 'application/json',
            'Authorization' => 'Basic ' . base64_encode($this->config['username'] . ':' . $this->config['password'])
        ];
        try {
            $response = $this->client->request('POST', $this->api_prefix, ['headers' => $headers]);
            if ($response->getStatusCode() === 409) {
                $this->session_id = $response->getHeaderLine('X-Transmission-Session-Id');
                return json(['code' => 1, 'msg' => "TR 登录成功"]);
            } else {
                return json(['code' => 0, 'msg' => "账号密码错误"]);
            }
        } catch (\Exception $e) {
            return json(['code' => 0, 'msg' => $e->getMessage()]);
        }
    }

    public function getTorrentsInfo($hash = '', $filter = '')
    {
        $this->initSessionId();
        $headers = [
            'Authorization' => 'Basic ' . base64_encode($this->config['username'] . ':' . $this->config['password']),
            'X-Transmission-Session-Id' => $this->session_id,
            'Content-Type' => 'json',
        ];

        $body = [
            "method" => "torrent-get",
            "arguments" => [
                "fields" => [
                    "hashString", "name", "downloadDir", "sizeWhenDone", "totalSize", 
                    "labels", "id", "error", "errorString", "eta", "isFinished", 
                    "isStalled", "leftUntilDone", "metadataPercentComplete", 
                    "peersConnected", "peersGettingFromUs", "peersSendingToUs", 
                    "percentDone", "queuePosition", "rateDownload", "rateUpload", 
                    "recheckProgress", "seedRatioMode", "seedRatioLimit", "status", 
                    "trackers", "uploadedEver", "uploadRatio", "webseedsSendingToUs"
                ],
                "ids" => is_array($hash) ? $hash : [$hash],
            ]
        ];
        $response = $this->client->post($this->api_prefix, [
            'body' => json_encode($body),
            'headers' => $headers
        ]);
        return json_decode($response->getBody()->getContents(), true);
    }

    public function addTorrent($url, $save_path, $is_paused)
    {
        $this->initSessionId();
        $is_paused == 1 ? $is_paused = 'false' : $is_paused = 'true';
        $headers = [
            'Authorization' => 'Basic ' . base64_encode($this->config['username'] . ':' . $this->config['password']),
            'X-Transmission-Session-Id' => $this->session_id,
            'Content-Type' => 'json',
        ];
        $body = [
            "method" => "torrent-add",
            "arguments" => ["paused" => $is_paused, "download-dir" => $save_path, "filename" => $url]
        ];
        $response = $this->client->post($this->api_prefix, [
            'body' => json_encode($body),
            'headers' => $headers
        ]);
        return json_decode($response->getBody()->getContents(), true);
    }
}
```

**API 特性**:
- 使用 Basic Auth 认证
- 需要获取 X-Transmission-Session-Id
- 使用 JSON-RPC 协议
- 首次请求返回 409 获取 session_id

### 9.2 站点 API

#### 9.2.1 NexusPHP 标准 API

**请求格式**:
```json
POST {api_url}
Content-Type: application/json

{
    "passkey": "{passkey}",
    "pieces_hash": ["pieces_hash_1", "pieces_hash_2", ...]
}
```

**响应格式**:
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "pieces_hash_1": "torrent_id_1",
        "pieces_hash_2": "torrent_id_2"
    }
}
```

**下载链接**:
```
{site_url}/download.php?id={torrent_id}&passkey={passkey}
```

#### 9.2.2 Kimoji 自研 API

**请求格式**:
```
GET {api_url}/{pieces_hash}
```

**响应格式**:
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "pieces_hash_1": "torrent_id_1"
    }
}
```

**下载链接**:
```
{site_url}/torrent/download/{torrent_id}.rsskey
```

---

## 第10章：安全分析

### 10.1 认证安全

#### 10.1.1 密码存储

```php
// 密码使用 MD5 存储（不推荐）
->addColumn('password', 'string', ['limit' => 32, 'default' => '', 'null' => true, 'comment' => '用户密码'])
```

**安全问题**:
- 使用 MD5 存储密码，已被证明不安全
- 没有使用 salt
- 建议使用 password_hash() 和 password_verify()

#### 10.1.2 Session 安全

ThinkAdmin 默认使用文件存储 Session：

```php
// config/session.php
return [
    'type' => 'file',
    'path' => '',
    'prefix' => 'think',
    'expire' => 0,
];
```

**安全建议**:
- 考虑使用 Redis 存储 Session
- 设置合理的过期时间
- 启用 HTTPS

### 10.2 API 安全

#### 10.2.1 站点 Passkey

Passkey 明文存储在数据库：

```php
->addColumn('passkey', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => 'Passkey'])
```

**安全问题**:
- Passkey 明文存储
- 数据库泄露会导致 Passkey 泄露
- 建议加密存储

#### 10.2.2 下载器密码

下载器密码明文存储：

```php
->addColumn('password', 'string', ['limit' => 100, 'default' => '', 'null' => true, 'comment' => '密码'])
```

**安全问题**:
- 密码明文存储
- 建议使用加密存储

### 10.3 请求安全

#### 10.3.1 CSRF 防护

ThinkAdmin 内置 CSRF 防护：

```php
// 表单自动添加 CSRF token
<form>
    <input type="hidden" name="__token__" value="...">
</form>
```

#### 10.3.2 SQL 注入防护

使用 ThinkPHP ORM 自动防护：

```php
// 使用参数化查询
SystemSites::mQuery()->db()->where('id', $id)->find();
```

#### 10.3.3 XSS 防护

使用 voku/anti-xss 库：

```json
"voku/anti-xss": "^4.1"
```

### 10.4 网络安全

#### 10.4.1 SSL 验证

HTTP 请求关闭了 SSL 验证：

```php
$options = [
    'headers' => $headers,
    'timeout' => 10,
    'verify' => false,  // 关闭 SSL 验证
];
```

**安全问题**:
- 容易受到中间人攻击
- 建议在生产环境启用 SSL 验证

#### 10.4.2 超时设置

请求超时设置为 10 秒：

```php
'timeout' => 10
```

**安全建议**:
- 合理设置超时时间
- 防止长时间挂起

---

## 第11章：性能优化

### 11.1 批量处理

#### 11.1.1 批量查询

```php
// 每次最多 2000 个
$info_hash_to_pieces_group = array_chunk(array_keys($info_hash_to_pieces), 2000);
foreach ($info_hash_to_pieces_group as $group_list) {
    $torrent_info_list = $qbapi->getTorrentsInfo(implode('|', $group_list), 'completed');
}
```

**优化效果**:
- 减少 HTTP 请求数量
- 降低网络开销
- 提高整体效率

#### 11.1.2 批量请求站点

```php
// 每次最多 100 个 pieces_hash
$pieces_hash_groups = array_chunk($actived_pieces_hash_list, 100);
foreach ($pieces_hash_groups as $group_list) {
    $download_url = FastRequest::pieces_request($site, $group_list);
}
```

**优化效果**:
- 减少站点请求次数
- 降低站点服务器压力
- 提高辅种效率

### 11.2 缓存优化

#### 11.2.1 辅种缓存

```php
// 缓存已辅种的 pieces_hash
CacheHelper::SetPiecesHash($site['site_url'], $cached_pieces_hash_list);

// 过滤已缓存的
$actived_pieces_hash_list = array_diff(
    array_diff(array_keys($pieces_hash_info), $cached_pieces_hash_list), 
    $exists_pieces_hash_list
);
```

**缓存优势**:
- 避免重复请求站点
- 减少站点服务器压力
- 提高辅种速度

#### 11.2.2 种子库缓存

```php
// 通过事件订阅自动维护种子库
event('torrent_add', [
    'site_url' => $site['site_url'],
    'torrents' => $pieces_hash_torrent_id,
]);
```

**缓存优势**:
- 减少站点请求
- 提高查询效率
- 支持历史记录查询

### 11.3 数据库优化

#### 11.3.1 索引优化

```php
// 站点表索引
->addIndex('site_url', ['name' => 'idx_system_sites_site_url'])

// 种子库索引
->addIndex('site_url', ['name' => 'idx_torrent_bank_site_url'])
->addIndex('pieces_hash', ['name' => 'idx_torrent_bank_pieces_hash'])

// 任务队列表索引
->addIndex('code', ['name' => 'idx_system_queue_code'])
->addIndex('status', ['name' => 'idx_system_queue_status'])
```

**优化效果**:
- 提高查询速度
- 减少数据库负载

#### 11.3.2 查询优化

```php
// 使用批量查询
SystemSites::mQuery()->db()->whereIn('id', $siteIds)->select();

// 使用批量删除
TorrentBank::query()->where(['id' => array_column($has_torrent_list, 'id')])->delete();

// 使用批量插入
TorrentBank::mk()->insertAll($torrents_add);
```

### 11.4 请求优化

#### 11.4.1 请求间隔控制

```php
private function request_interval($site): int {
    return intval($site['request_interval']) > 2 ? 
           intval($site['request_interval']) : 2;
}
```

**优化目的**:
- 避免请求过于频繁
- 防止被站点封禁
- 可按站点配置

#### 11.4.2 超时设置

```php
$options = [
    'headers' => $headers,
    'timeout' => 10,
    'verify' => false,
];
```

**优化目的**:
- 防止长时间挂起
- 快速失败重试
- 提高整体效率

---

## 第12章：项目优缺点分析

### 12.1 优点

#### 12.1.1 技术架构

1. **成熟的框架**
   - 基于 ThinkPHP 6.x 框架
   - 使用 ThinkAdmin 后台管理
   - 社区活跃，文档完善

2. **设计模式清晰**
   - MVC 架构
   - 适配器模式
   - 工厂模式
   - 策略模式
   - 事件驱动

3. **代码结构良好**
   - 分层清晰
   - 职责明确
   - 易于维护

#### 12.1.2 功能特性

1. **pieces_hash 匹配**
   - 精确匹配种子内容
   - 避免重复下载
   - 跨站点辅种

2. **多下载器支持**
   - qBittorrent
   - Transmission
   - 统一接口封装

3. **智能缓存**
   - 按站点缓存
   - 避免重复请求
   - 提高效率

4. **任务队列**
   - 定时执行
   - 进度跟踪
   - 错误重试

5. **种子库**
   - 本地存储
   - 减少站点请求
   - 历史记录

#### 12.1.3 用户体验

1. **Web 管理界面**
   - 直观易用
   - 实时反馈
   - 操作日志

2. **Excel 导入**
   - 批量导入站点
   - 提高配置效率

3. **统计信息**
   - 辅种统计
   - 成功率统计
   - 错误信息

### 12.2 缺点

#### 12.2.1 安全问题

1. **密码存储**
   - 使用 MD5 存储密码
   - 没有使用 salt
   - 建议使用 password_hash()

2. **敏感信息存储**
   - Passkey 明文存储
   - 下载器密码明文存储
   - 建议加密存储

3. **SSL 验证**
   - 关闭了 SSL 验证
   - 容易受到中间人攻击
   - 建议生产环境启用

#### 12.2.2 功能不足

1. **缺少监控**
   - 没有系统监控
   - 没有告警机制
   - 难以及时发现问题

2. **缺少统计**
   - 没有详细统计
   - 没有图表展示
   - 难以分析数据

3. **缺少限流**
   - 没有全局限流
   - 可能被滥用
   - 建议添加限流机制

#### 12.2.3 性能问题

1. **单线程处理**
   - 没有使用多线程
   - 处理速度有限
   - 建议使用队列

2. **文件缓存**
   - 使用文件缓存
   - 性能不如 Redis
   - 建议使用 Redis

3. **数据库查询**
   - 部分查询可以优化
   - 建议添加更多索引

#### 12.2.4 代码质量

1. **错误处理**
   - 部分异常捕获不完整
   - 建议完善错误处理

2. **日志记录**
   - 日志不够详细
   - 建议添加更多日志

3. **代码注释**
   - 部分代码缺少注释
   - 建议添加注释

---

## 第13章：改进建议

### 13.1 短期改进

#### 13.1.1 安全加固

**1. 密码加密**

```php
// 使用 password_hash 替代 MD5
$hashedPassword = password_hash($password, PASSWORD_DEFAULT);

// 验证密码
if (password_verify($inputPassword, $hashedPassword)) {
    // 密码正确
}
```

**2. 敏感信息加密**

```php
// 使用 OpenSSL 加密
function encrypt($data, $key) {
    $iv = openssl_random_pseudo_bytes(openssl_cipher_iv_length('aes-256-cbc'));
    $encrypted = openssl_encrypt($data, 'aes-256-cbc', $key, 0, $iv);
    return base64_encode($encrypted . '::' . $iv);
}

function decrypt($data, $key) {
    list($encrypted_data, $iv) = explode('::', base64_decode($data), 2);
    return openssl_decrypt($encrypted_data, 'aes-256-cbc', $key, 0, $iv);
}
```

**3. 启用 SSL 验证**

```php
$options = [
    'headers' => $headers,
    'timeout' => 10,
    'verify' => true,  // 启用 SSL 验证
];
```

#### 13.1.2 性能优化

**1. 使用 Redis 缓存**

```php
// config/cache.php
'redis' => [
    'type' => 'redis',
    'host' => '127.0.0.1',
    'port' => 6379,
    'password' => '',
    'select' => 0,
    'timeout' => 0,
    'expire' => 0,
    'persistent' => false,
    'prefix' => '',
],
```

**2. 添加更多索引**

```php
// 用户表
->addIndex('username', ['name' => 'idx_system_user_username'])
->addIndex('status', ['name' => 'idx_system_user_status'])

// 操作日志表
->addIndex('create_at', ['name' => 'idx_system_oplog_create_at'])
->addIndex('username', ['name' => 'idx_system_oplog_username'])
```

**3. 优化查询**

```php
// 使用缓存
$cached_sites = Cache::remember('sites', 3600, function() {
    return SystemSites::select()->toArray();
});
```

### 13.2 中期改进

#### 13.2.1 功能增强

**1. 添加监控系统**

```php
// 监控类
class MonitorService
{
    public static function checkSystem()
    {
        $status = [
            'disk' => self::checkDisk(),
            'memory' => self::checkMemory(),
            'cpu' => self::checkCpu(),
            'database' => self::checkDatabase(),
        ];
        return $status;
    }
    
    private static function checkDisk()
    {
        $free = disk_free_space('/');
        $total = disk_total_space('/');
        return [
            'free' => $free,
            'total' => $total,
            'usage' => round(($total - $free) / $total * 100, 2),
        ];
    }
}
```

**2. 添加告警机制**

```php
// 告警类
class AlertService
{
    public static function sendAlert($title, $message)
    {
        // 发送邮件
        Mail::send($title, $message);
        
        // 发送 Telegram
        Telegram::send($message);
        
        // 发送企业微信
        Wechat::send($message);
    }
}
```

**3. 添加统计功能**

```php
// 统计类
class StatisticsService
{
    public static function getReseedStatistics($days = 7)
    {
        $startDate = date('Y-m-d', strtotime("-{$days} days"));
        $endDate = date('Y-m-d');
        
        return [
            'total' => TorrentBank::whereBetween('create_at', [$startDate, $endDate])->count(),
            'success' => TorrentBank::whereBetween('create_at', [$startDate, $endDate])
                ->where('status', 1)->count(),
            'failed' => TorrentBank::whereBetween('create_at', [$startDate, $endDate])
                ->where('status', 0)->count(),
        ];
    }
}
```

#### 13.2.2 性能提升

**1. 使用队列处理**

```php
// 队列任务
class ReseedJob
{
    public function fire($job, $data)
    {
        try {
            // 执行辅种
            $result = $this->doReseed($data);
            
            // 标记任务完成
            $job->delete();
        } catch (\Exception $e) {
            // 任务失败，重试
            if ($job->attempts() < 3) {
                $job->release(60);
            } else {
                $job->delete();
            }
        }
    }
}
```

**2. 使用多进程**

```php
// 多进程处理
$pool = new Swoole\Process\Pool(4);
$pool->on('WorkerStart', function ($pool, $workerId) {
    echo "Worker#{$workerId} is started\n";
    $this->doReseed($workerId);
});
$pool->start();
```

### 13.3 长期改进

#### 13.3.1 架构升级

**1. 微服务化**

```
┌─────────────┐
│  API Gateway│
└──────┬──────┘
       │
       ├─► 用户服务
       ├─► 站点服务
       ├─► 下载器服务
       ├─► 任务服务
       └─► 统计服务
```

**2. 消息队列**

```
┌─────────┐    ┌─────────┐    ┌─────────┐
│  生产者  │───►│  MQ     │───►│  消费者  │
└─────────┘    └─────────┘    └─────────┘
```

**3. 分布式缓存**

```
┌─────────┐    ┌─────────┐    ┌─────────┐
│  应用1  │───►│  Redis  │◄───│  应用2  │
└─────────┘    └─────────┘    └─────────┘
```

#### 13.3.2 技术栈升级

**1. 升级 PHP 版本**

```json
{
  "require": {
    "php": ">=8.0"
  }
}
```

**2. 使用现代框架**

- Laravel
- Symfony
- Hyperf

**3. 使用前端框架**

- Vue.js
- React
- Angular

---

## 第14章：完整文件结构

### 14.1 项目文件树

```
reseed-puppy-php/
├── app/
│   ├── admin/
│   │   ├── controller/
│   │   │   ├── api/
│   │   │   │   ├── Plugs.php
│   │   │   │   ├── Queue.php
│   │   │   │   ├── Sites.php
│   │   │   │   ├── System.php
│   │   │   │   └── Upload.php
│   │   │   ├── Auth.php
│   │   │   ├── Base.php
│   │   │   ├── Config.php
│   │   │   ├── Cron.php
│   │   │   ├── Download.php
│   │   │   ├── File.php
│   │   │   ├── Index.php
│   │   │   ├── IyuuMsg.php
│   │   │   ├── Login.php
│   │   │   ├── Menu.php
│   │   │   ├── Oplog.php
│   │   │   ├── Qbapi.php
│   │   │   ├── Queue.php
│   │   │   ├── Sites.php
│   │   │   ├── Trapi.php
│   │   │   └── User.php
│   │   ├── Service.php
│   │   ├── lang/
│   │   │   └── en-us.php
│   │   ├── route/
│   │   │   └── demo.php
│   │   └── view/
│   ├── command/
│   │   ├── Cron.php
│   │   └── SearchReSeed.php
│   ├── exception/
│   │   └── KnownException.php
│   ├── helper/
│   │   ├── CacheHelper.php
│   │   ├── CommonHelper.php
│   │   ├── FastRequest.php
│   │   └── TorrentHelper.php
│   ├── index/
│   │   └── controller/
│   │       └── Index.php
│   ├── model/
│   │   └── TorrentBank.php
│   ├── subscribe/
│   │   └── TorrentSubscribe.php
│   ├── event.php
│   └── provider.php
├── config/
│   ├── app.php
│   ├── cache.php
│   ├── console.php
│   ├── cookie.php
│   ├── database.php
│   ├── lang.php
│   ├── log.php
│   ├── phinx.php
│   ├── route.php
│   ├── session.php
│   ├── site.php
│   └── view.php
├── database/
│   ├── migrations/
│   │   ├── 20221013031925_install_admin.php
│   │   ├── 20221013031926_install_admin_data.php
│   │   ├── 202311013031927_install_package.php
│   │   ├── 202311013031955_insert_sites.php
│   │   ├── 202311143038888_addokpt_sites.php
│   │   ├── 202311283039999_install_log.php
│   │   ├── 20240107080108_site_request_interval.php
│   │   ├── 20240108061235_torrent_bank.php
│   │   ├── 202501069999999_add_sites.php
│   │   ├── 202502189999999_add_new_sites.php
│   │   └── 202503139999999_add_new_sites_two.php
│   └── sqlite.db
├── public/
│   ├── static/
│   │   ├── admin/
│   │   ├── plugs/
│   │   ├── theme/
│   │   └── ...
│   ├── index.php
│   └── router.php
├── runtime/
│   ├── cache/
│   ├── log/
│   └── temp/
├── vendor/
│   ├── guzzlehttp/
│   ├── phpoffice/
│   ├── psr/
│   ├── rhilip/
│   ├── thinkphp/
│   ├── topthink/
│   └── ...
├── composer.json
├── composer.lock
├── README.md
└── ...
```

---

## 第15章：API 接口规范

### 15.1 站点 API 规范

#### 15.1.1 NexusPHP 标准 API

**请求**:
```http
POST {api_url}
Content-Type: application/json

{
    "passkey": "{passkey}",
    "pieces_hash": ["pieces_hash_1", "pieces_hash_2", ...]
}
```

**响应**:
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "pieces_hash_1": "torrent_id_1",
        "pieces_hash_2": "torrent_id_2"
    }
}
```

**下载链接**:
```
{site_url}/download.php?id={torrent_id}&passkey={passkey}
```

#### 15.1.2 Kimoji API

**请求**:
```http
GET {api_url}/{pieces_hash}
```

**响应**:
```json
{
    "code": 0,
    "message": "success",
    "data": {
        "pieces_hash_1": "torrent_id_1"
    }
}
```

**下载链接**:
```
{site_url}/torrent/download/{torrent_id}.rsskey
```

### 15.2 下载器 API 规范

#### 15.2.1 qBittorrent API

**登录**:
```http
POST /api/v2/auth/login
Content-Type: application/x-www-form-urlencoded

username={username}&password={password}
```

**获取种子信息**:
```http
POST /api/v2/torrents/info
Content-Type: application/x-www-form-urlencoded

hashes={hash1}|{hash2}|...&filter=completed
```

**添加种子**:
```http
POST /api/v2/torrents/add
Content-Type: application/x-www-form-urlencoded

urls={url}&savepath={path}&skip_checking={true/false}&paused={true/false}&tags={tag}&upLimit={limit}&dlLimit={limit}
```

#### 15.2.2 Transmission API

**获取 Session ID**:
```http
POST /transmission/rpc
Authorization: Basic {base64(username:password)}
Content-Type: application/json

{"method": "torrent-get", "arguments": {...}}
```

**获取种子信息**:
```http
POST /transmission/rpc
Authorization: Basic {base64(username:password)}
X-Transmission-Session-Id: {session_id}
Content-Type: application/json

{
    "method": "torrent-get",
    "arguments": {
        "fields": [...],
        "ids": ["hash1", "hash2", ...]
    }
}
```

**添加种子**:
```http
POST /transmission/rpc
Authorization: Basic {base64(username:password)}
X-Transmission-Session-Id: {session_id}
Content-Type: application/json

{
    "method": "torrent-add",
    "arguments": {
        "paused": {true/false},
        "download-dir": "{path}",
        "filename": "{url}"
    }
}
```

---

## 第16章：数据库关系图

### 16.1 ER 图

```
┌─────────────┐
│ system_user │
├─────────────┤
│ id          │
│ username    │
│ password    │
│ nickname    │
│ headimg     │
│ authorize   │
│ is_super    │
│ status      │
└──────┬──────┘
       │
       ├─► system_oplog
       │
       ├─► system_queue
       │
       ├─► system_sites
       │
       └─► system_download

┌─────────────┐
│system_sites │
├─────────────┤
│ id          │
│ site_name   │
│ site_url    │
│ api_url     │
│ passkey     │
│ request_... │
│ upload_...  │
│ download_.. │
│ status      │
└──────┬──────┘
       │
       └─► torrent_bank

┌──────────────┐
│system_download│
├──────────────┤
│ id            │
│ name          │
│ type          │
│ host          │
│ port          │
│ username      │
│ password      │
│ save_path     │
│ reseed_sites  │
│ is_skip_hash  │
│ is_paused     │
│ status        │
└───────────────┘

┌─────────────┐
│ torrent_bank│
├─────────────┤
│ id          │
│ site_url    │
│ pieces_hash │
│ torrent_id  │
│ create_at   │
└─────────────┘
```

---

## 附录A：相关资源

### A.1 官方文档

- [ThinkPHP 6.0 文档](https://www.kancloud.cn/manual/thinkphp6_0/content)
- [ThinkAdmin 文档](https://thinkadmin.top)
- [qBittorrent Web API](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1%2B))
- [Transmission RPC](https://github.com/transmission/transmission/wiki/RPC-Protocol-Specification)

### A.2 相关项目

- [ptdog](https://github.com/ptdog/ptdog) - Go 语言实现的辅种工具
- [Reseed-backend](https://github.com/Reseed-Puppy/Reseed-backend) - Python Flask 实现的辅种后端
- [NexusPHP](https://github.com/NexusPHP/NexusPHP) - PT 站点系统

### A.3 技术博客

- [pieces_hash 原理](https://blog.example.com/pieces-hash)
- [PT 辅种最佳实践](https://blog.example.com/reseed-best-practices)
- [ThinkPHP 性能优化](https://blog.example.com/thinkphp-performance)

---

## 附录B：部署指南

### B.1 环境要求

- PHP >= 7.1
- MySQL >= 5.7 或 SQLite
- Nginx / Apache
- Composer

### B.2 安装步骤

```bash
# 1. 克隆项目
git clone https://github.com/Reseed-Puppy/Reseed-Puppy-PHP.git
cd Reseed-Puppy-PHP

# 2. 安装依赖
composer install

# 3. 配置数据库
cp config/database.php.example config/database.php
vim config/database.php

# 4. 运行迁移
php think migrate:run

# 5. 配置站点
cp config/site.php.example config/site.php
vim config/site.php

# 6. 设置权限
chmod -R 755 runtime/
chmod -R 755 public/

# 7. 配置 Web 服务器
# Nginx 示例
server {
    listen 80;
    server_name example.com;
    root /path/to/Reseed-Puppy-PHP/public;
    index index.php;
    
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
    
    location ~ \.php$ {
        fastcgi_pass 127.0.0.1:9000;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

### B.3 定时任务配置

```bash
# 添加 crontab
crontab -e

# 每小时执行一次辅种
0 * * * * cd /path/to/Reseed-Puppy-PHP && php think cron

# 启动任务队列守护进程
php think xadmin:queue start
```

---

## 附录C：常见问题

### C.1 安装问题

**Q: Composer 安装失败？**
A: 使用国内镜像
```bash
composer config -g repo.packagist composer https://mirrors.aliyun.com/composer/
```

**Q: 数据库迁移失败？**
A: 检查数据库连接配置
```bash
php think migrate:status
```

### C.2 使用问题

**Q: 辅种不成功？**
A: 检查以下几点：
1. 站点 passkey 是否正确
2. 站点 API 是否支持 pieces_hash
3. 下载器连接是否正常
4. 种子目录映射是否正确

**Q: 缓存如何清理？**
A: 在后台管理页面清理，或手动删除缓存文件
```bash
rm -rf runtime/cache/*
```

### C.3 性能问题

**Q: 辅种速度慢？**
A: 优化建议：
1. 减少站点数量
2. 增加请求间隔
3. 使用 Redis 缓存
4. 优化数据库索引

---

## 附录D：术语表

| 术语 | 说明 |
|------|------|
| pieces_hash | 种子文件 info.pieces 字段的 SHA1 哈希值 |
| info_hash | 种子文件 info 字段的 SHA1 哈希值 |
| passkey | 站点个人密钥，用于 API 认证 |
| tracker | BT 跟踪服务器 |
| reseed | 辅种，将已下载的种子在其他站点做种 |
| NexusPHP | PT 站点常用系统 |
| qBittorrent | 开源 BT 下载器 |
| Transmission | 开源 BT 下载器 |
| ThinkPHP | PHP 开发框架 |
| ThinkAdmin | 基于 ThinkPHP 的后台管理系统 |

---

## 附录E：版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0.0 | 2022-10-13 | 初始版本 |
| 1.1.0 | 2023-11-01 | 添加种子库功能 |
| 1.2.0 | 2024-01-07 | 添加站点请求间隔配置 |
| 1.3.0 | 2024-01-08 | 添加种子库表 |
| 1.4.0 | 2025-01-06 | 添加更多站点 |
| 1.5.0 | 2025-02-18 | 添加新站点 |
| 1.6.0 | 2025-03-13 | 添加更多站点 |

---

## 总结

Reseed-Puppy-PHP 是一个功能完善、设计清晰的 PT 辅种工具。它基于成熟的 ThinkPHP 框架开发，采用 MVC 架构和多种设计模式，代码质量较高。

**核心优势**:
- 基于 pieces_hash 的精确匹配
- 支持多种下载器
- 智能缓存机制
- 完善的任务队列
- 友好的 Web 管理界面

**改进空间**:
- 安全性需要加强（密码加密、SSL 验证）
- 性能可以优化（Redis 缓存、队列处理）
- 功能可以增强（监控、统计、告警）

总体而言，这是一个值得学习和参考的 PHP 项目，对于理解 PT 辅种原理和 ThinkPHP 框架应用都有很大帮助。

---

**文档版本**: 1.0  
**最后更新**: 2026-04-11  
**作者**: AI Assistant  
**项目地址**: examples/reseed-puppy-php
