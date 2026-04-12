# IYUUPlus-dev 项目深度分析报告

## 一、项目概述

### 1.1 项目定位

IYUUPlus 是一款 **PT站点自动辅种工具**，基于 PHP CLI 模式常驻内存运行，集成 WebUI 界面、辅种、转移、下载、RSS订阅、定时任务等功能。

### 1.2 核心功能

| 功能模块 | 说明 |
|----------|------|
| **自动辅种** | 通过 info_hash 匹配各站点种子，自动下载并添加到下载器 |
| **转移做种** | 在不同下载器之间转移做种任务 |
| **RSS订阅** | 自动监控站点RSS，下载符合条件的种子 |
| **下载器管理** | 支持 Transmission 和 qBittorrent |
| **站点管理** | 支持 100+ PT站点的种子下载 |

### 1.3 技术栈

| 组件 | 版本 | 说明 |
|------|------|------|
| **Workerman** | 5.1.3 | 高性能PHP Socket框架 |
| **Webman** | 1.6.14 | 基于Workerman的Web框架 |
| **WebmanAdmin** | 0.6.30 | 后台管理框架 |
| **PHP** | 8.3+ | 运行环境 |
| **Laravel Components** | 10.48 | 数据库、事件等组件 |
| **Layui** | 2.8.12 | 前端UI框架 |
| **Vue** | 3.4.21 | 前端框架 |

---

## 二、架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        IYUUPlus 架构                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │   WebUI     │  │   API层     │  │  CLI命令    │             │
│  │  (Layui)    │  │ (Controller)│  │  (Command)  │             │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘             │
│         │                │                │                     │
│         └────────────────┼────────────────┘                     │
│                          ▼                                      │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                    Services Layer                         │ │
│  │  ReseedServices │ TransferServices │ RssServices │ ...   │ │
│  └───────────────────────────────────────────────────────────┘ │
│                          │                                      │
│         ┌────────────────┼────────────────┐                    │
│         ▼                ▼                ▼                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │SiteManager  │  │Bittorrent   │  │ReseedClient │            │
│  │(站点驱动)   │  │Client(下载器)│  │(IYUU服务器) │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                │                │                    │
│         ▼                ▼                ▼                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  PT站点     │  │qB/TR下载器  │  │IYUU服务器   │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 进程模型

项目通过 `config/process.php` 配置多个常驻进程：

| 进程名 | 类 | 说明 |
|--------|-----|------|
| monitor | `process\Monitor` | 文件监控进程，不可重载 |
| reseed | `process\ReseedProcess` | 辅种进程 |
| cloud | `process\MovieProcess` | 视听云进程 |

### 2.3 启动引导

`app/Bootstrap.php` 在 Worker 启动时执行初始化：

```php
class Bootstrap implements \Webman\Bootstrap
{
    public static function start(?Worker $worker): void
    {
        self::initObserver();      // 初始化模型观察者
        self::initCrontabExtend(); // 扩展计划任务类型
        ClientServices::bootstrap();
        DownloaderServices::bootstrap();
    }
}
```

---

## 三、核心模块分析

### 3.1 自动辅种模块 (ReseedServices)

#### 工作流程

```
┌─────────────────┐
│ 1. 获取下载器   │
│    做种hash列表 │
└────────┬────────┘
         ▼
┌─────────────────┐
│ 2. 路径过滤     │
│    排除指定目录 │
└────────┬────────┘
         ▼
┌─────────────────┐
│ 3. 请求IYUU     │
│    服务器匹配   │
└────────┬────────┘
         ▼
┌─────────────────┐
│ 4. 遍历返回的   │
│    可辅种数据   │
└────────┬────────┘
         ▼
┌─────────────────┐
│ 5. 下载种子文件 │
│    添加到下载器 │
└─────────────────┘
```

#### 核心代码逻辑

**文件**: `app/admin/services/reseed/ReseedServices.php`

```php
class ReseedServices
{
    private const int RESEED_GROUP_NUMBER = 200;  // 每批次分组数量
    
    public function run(): void {
        $reseedClient = iyuu_reseed_client();
        $sid_sha1 = $this->getSidSha1($reseedClient);
        
        foreach ($this->crontabClients as $client_id => $on) {
            $this->clientModel = ClientServices::getClient((int)$client_id);
            $this->bittorrentClient = ClientServices::createBittorrent($this->clientModel);
            
            $torrentList = $this->bittorrentClient->getTorrentList();
            $hashDict = $torrentList['hashString'];  // 哈希目录字典
            
            // 分批次辅种
            $full = json_decode($torrentList['hash'], true);
            $chunkHash = array_chunk($full, self::RESEED_GROUP_NUMBER);
            
            foreach ($chunkHash as $info_hash) {
                $result = $reseedClient->reseed($hash, sha1($hash), $sid_sha1, iyuu_version());
                $this->currentReseed($hashDict, $result);
            }
        }
    }
}
```

#### 辅种下载服务

**文件**: `app/admin/services/reseed/ReseedDownloadServices.php`

```php
class ReseedDownloadServices
{
    private const int SLEEP_MAX_VALUE = 30;

    public static function handle(Site $site): void
    {
        $config = new Config($site->toArray());
        $limit = $config->getLimit();
        
        if (empty($limit)) {
            // 不限速的站点
            self::handleOpen($site);
        } else {
            // 限速的站点
            $limitCount = $limit['count'] ?? 20;
            $limitSleep = $limit['sleep'] ?? 10;
            self::handleLimited($site, $limitCount, $limitSleep);
        }
    }
    
    public static function sendDownloader(Reseed $reseed, int $limitSleep = 0): bool
    {
        // 下载种子
        $response = Helper::download($torrent);
        
        // 触发事件：下载种子之后
        Event::dispatch(EventReseedEnums::reseed_torrent_download_after->value, [$response, $reseed]);
        
        // 发送到下载器
        $bittorrentClients = ClientServices::createBittorrent($clientModel);
        $contractsTorrent = new TorrentContract($response->payload, $response->metadata);
        $contractsTorrent->savePath = $reseed->directory;
        
        return $bittorrentClients->addTorrent($contractsTorrent);
    }
}
```

### 3.2 转移做种模块 (TransferServices)

#### 工作流程

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ 来源下载器      │ ──→ │ 读取种子元信息   │ ──→ │ 路径转换        │
│ 获取做种列表    │     │ (torrent文件)    │     │ (目录映射)      │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                         │
                                                         ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ 删除源做种(可选)│ ←── │ 添加到目标下载器 │ ←── │ 发送种子文件    │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

#### 核心代码逻辑

**文件**: `app/admin/services/transfer/TransferServices.php`

```php
class TransferServices
{
    public const string Delimiter = '{#**#}';
    
    public function run(): void {
        $fromBittorrentClient = ClientServices::createBittorrent($this->from_clients);
        $toBittorrentClient = ClientServices::createBittorrent($this->to_client);
        
        $torrentList = $fromBittorrentClient->getTorrentList();
        $hashDict = $torrentList['hashString'];
        
        foreach ($hashDict as $infohash => $downloadDirOriginal) {
            // 检查缓存避免重复转移
            if (Transfer::where($attributes)->exists()) {
                continue;
            }
            
            // 路径转换
            $downloadDir = $this->pathConvert($downloadDirOriginal);
            
            // 读取种子元信息
            $contractsTorrent = match ($this->from_clients->getClientEnums()) {
                ClientEnums::transmission => $this->handleTransmission($rocket),
                ClientEnums::qBittorrent => $this->handleQBittorrent($rocket, ...),
            };
            
            $contractsTorrent->savePath = $downloadDir;
            
            // 添加到目标下载器
            $ret = $toBittorrentClient->addTorrent($contractsTorrent);
        }
    }
}
```

### 3.3 RSS订阅模块 (RssServices)

#### 工作流程

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ 请求RSS地址     │ ──→ │ 解析XML          │ ──→ │ 大小过滤        │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                         │
                                                         ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ 发送到下载器    │ ←── │ 标题匹配过滤     │ ←── │ 规则匹配        │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

#### 核心代码逻辑

**文件**: `app/admin/services/rss/RssServices.php`

```php
class RssServices
{
    public function run(): void {
        $xml = $this->requestXML();
        $items = $this->parseXML($xml);
        
        $downloader = ClientServices::createBittorrent($this->client);
        
        foreach ($items as $item) {
            // 大小过滤
            if (!$this->sizeLogic->match($item)) {
                continue;
            }
            
            // 规则过滤
            if (!$this->matchLogic->match($item)) {
                continue;
            }
            
            // 发送到下载器
            $torrent = new Torrent($item->getDownload(), false);
            $torrent->savePath = $this->save_path;
            $result = $downloader->addTorrent($torrent);
        }
    }
}
```

---

## 四、数据模型

### 4.1 核心模型关系

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │     │    Site     │     │   Reseed    │
│  (下载器)   │     │   (站点)    │     │  (辅种记录) │
├─────────────┤     ├─────────────┤     ├─────────────┤
│ id          │     │ id          │     │ reseed_id   │
│ brand       │     │ sid         │     │ client_id   │──┐
│ title       │     │ site        │     │ site        │  │
│ hostname    │     │ nickname    │     │ sid         │  │
│ endpoint    │     │ base_url    │     │ torrent_id  │  │
│ username    │     │ cookie      │     │ info_hash   │  │
│ password    │     │ options     │     │ directory   │  │
│ save_path   │     │ disabled    │     │ status      │  │
│ enabled     │     └─────────────┘     │ payload     │  │
└─────────────┘                         └─────────────┘  │
       │                                                 │
       │  ┌─────────────┐                               │
       └──│  Transfer   │◄──────────────────────────────┘
          │  (转移记录) │
          ├─────────────┤
          │ transfer_id │
          │ from_client │
          │ to_client   │
          │ info_hash   │
          │ directory   │
          │ state       │
          └─────────────┘
```

### 4.2 辅种状态枚举

**文件**: `app/model/enums/ReseedStatusEnums.php`

| 状态 | 值 | 说明 |
|------|-----|------|
| Default | 0 | 默认（待处理） |
| Success | 1 | 成功 |
| Fail | 2 | 失败 |
| Repeat | 3 | 重复 |
| Skip | 4 | 跳过 |

### 4.3 辅种子类型枚举

**文件**: `app/model/enums/ReseedSubtypeEnums.php`

| 类型 | 值 | 说明 |
|------|-----|------|
| Default | 0 | 自动辅种 |
| Downloader | 1 | 自动下载 |
| Transfer | 2 | 自动转移 |

---

## 五、站点驱动系统

### 5.1 驱动架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     BaseDriver (抽象基类)                       │
│  - downloadLink() 生成下载链接                                  │
│  - download() 下载种子文件                                      │
│  - getDetails() 获取种子详情                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ NexusPHP    │     │ Zhuque      │     │ DicMusic    │
│ (Nexus架构) │     │ (雀魂架构)  │     │ (音乐站)    │
└─────────────┘     └─────────────┘     └─────────────┘
```

### 5.2 站点管理器

**文件**: `composer/site-manager/src/SiteManager.php`

```php
class SiteManager extends Manager implements DownloaderInterface
{
    public const string DRIVER_PREFIX = 'Driver';
    private const string DRIVER_NAMESPACE = __NAMESPACE__ . '\\Driver\\';
    
    public function select(string $name): BaseDriver
    {
        return $this->driver($name);
    }
    
    public function download(Torrent $torrent): Response
    {
        return $this->select($torrent->site)->download($torrent);
    }
    
    public static function supportList(bool $isBoolean = false): array
    {
        // 返回站点支持情况：
        // - 爬虫支持
        // - RSS订阅支持
        // - 下载种子元数据支持
        // - 拼接种子链接支持
    }
}
```

### 5.3 站点基类

**文件**: `composer/site-manager/src/BaseDriver.php`

```php
abstract class BaseDriver implements DownloaderInterface, DownloaderLinkInterface
{
    public const array SUPPORT_SIGNATURE_RECOMMEND_SITES = ['hddolby', 'pthome', 'hdhome'];
    
    public function downloadLink(Torrent $torrent): string
    {
        $domain = $this->getConfig()->parseDomain();
        $uri = $this->getConfig()->parseUri();
        $url_replace = $this->parseReplace($torrent);
        
        // 支持用户签名下载的站点
        if (in_array($this->getConfig()->site, self::SUPPORT_SIGNATURE_RECOMMEND_SITES, true)) {
            $signString = $this->getSiteUserSignature();
            $url_join .= '&' . $signString;
        }
        
        return $domain . '/' . $uri . $url_join;
    }
}
```

### 5.4 Cookie处理器

```php
abstract class BaseCookie
{
    public function process(string $url): array {
        // 解析HTML列表页，提取种子信息
    }
}
```

### 5.5 支持的站点类型

| 类型 | 说明 | 示例站点 |
|------|------|----------|
| NexusPHP | 标准Nexus架构 | M-Team, HDArea, HDHome |
| Zhuque | 雀魂架构 | 朱雀 |
| DicMusic | 音乐站 | 无损音乐 |
| 特殊站点 | 自定义解析 | TTG, 馒头 |

### 5.6 NexusPHP站点配置

**文件**: `app/admin/services/site/NexusPHP.php`

```php
class NexusPHP extends Decorator
{
    public function html(): string
    {
        $html = $this->generate->html();
        $html .= <<<EOF
            <div class="layui-form-item">
                <label class="layui-form-label required">Passkey</label>
                <div class="layui-input-block">
                    <input type="text" name="options[passkey]" value="" required 
                           placeholder="请输入密钥passkey" class="layui-input">
                </div>
            </div>
EOF;
        return $html;
    }
}
```

---

## 六、下载器客户端

### 6.1 客户端抽象

**文件**: `composer/bittorrent-client/src/Clients.php`

```php
abstract class Clients implements ClientsInterface
{
    public const string TORRENT_LIST = 'lists';
    
    final public function __construct(array $config)
    {
        $this->config = new Config($config);
        $this->curl = $this->initCurl();
        $this->initialize();
    }
    
    final protected function initCurl(): Curl
    {
        $curl = new Curl();
        $curl->setTimeout(60, 600);
        $curl->setSslVerify(false, false);
        return $curl;
    }
}
```

### 6.2 支持的下载器

| 下载器 | 实现类 | 特性 |
|--------|--------|------|
| qBittorrent | `Driver\qBittorrent\Client` | WebAPI, 版本检测 |
| Transmission | `Driver\transmission\Client` | RPC接口 |

### 6.3 种子操作接口

```php
interface ClientsInterface {
    public function getTorrentList(): array;           // 获取种子列表
    public function addTorrent(Torrent $torrent): mixed; // 添加种子
    public function removeTorrent(string $hash): bool;   // 删除种子
    public function pauseTorrent(string $hash): bool;    // 暂停种子
    public function resumeTorrent(string $hash): bool;   // 恢复种子
    public function recheckTorrent(string $hash): bool;  // 校验种子
}
```

---

## 七、IYUU服务器通信

### 7.1 API接口

**文件**: `composer/reseed-client/src/Client.php`

| 接口 | 方法 | 说明 |
|------|------|------|
| `/reseed/sites/index` | GET | 获取站点列表 |
| `/reseed/sites/recommend` | GET | 获取推荐站点 |
| `/reseed/sites/reportExisting` | POST | 汇报持有站点 |
| `/reseed/index/index` | POST | 获取可辅种数据 |
| `/reseed/index/single` | GET | 查询单个种子辅种数据 |

### 7.2 API域名

```php
class Client extends AbstractCurl
{
    public const string BASE_API = 'http://2025.iyuu.cn';
    public const string VIP_BASE_API = 'http://vip.iyuu.cn';
    
    public function getBaseApi(): string
    {
        return $this->isVip ? self::VIP_BASE_API : self::BASE_API;
    }
}
```

### 7.3 辅种请求流程

```php
// 1. 汇报持有站点
$sid_sha1 = $reseedClient->reportExisting($sid_list);

// 2. 请求辅种数据
$result = $reseedClient->reseed(
    $hash,        // info_hash列表JSON
    sha1($hash),  // 哈希签名
    $sid_sha1,    // 站点签名
    $version      // 版本号
);

// 3. 返回数据结构
[
    'sid' => [
        'torrent_id' => [
            'info_hash' => '...',
            'title' => '种子标题',
        ]
    ]
]
```

---

## 八、事件系统

### 8.1 辅种事件枚举

**文件**: `app/enums/EventReseedEnums.php`

| 事件 | 值 | 触发时机 |
|------|-----|----------|
| reseed_torrent_download_after | reseed.torrent.download.after | 下载种子之后 |
| reseed_torrent_send_before | reseed.torrent.send.before | 发送给下载器之前 |
| reseed_torrent_send_after | reseed.torrent.send.after | 发送给下载器之后 |
| reseed_current_before | reseed.current.before | 当前客户端辅种开始前 |
| reseed_current_after | reseed.current.after | 当前客户端辅种结束后 |
| reseed_all_done | reseed.all.done | 全部客户端辅种结束 |

### 8.2 转移事件枚举

**文件**: `app/enums/EventTransferEnums.php`

| 事件 | 值 | 触发时机 |
|------|-----|----------|
| transfer_action_before | transfer.action.before | 转移前 |
| transfer_action_after | transfer.action.after | 转移后 |

### 8.3 事件注册Trait

**文件**: `app/traits/HasEventRegister.php`

```php
trait HasEventRegister
{
    public function register(callable $listener): int
    {
        return Event::on($this->value, $listener);
    }
}
```

### 8.4 事件配置

**文件**: `config/event.php`

```php
return [
    EventReseedEnums::reseed_torrent_download_after->value => [],
    EventReseedEnums::reseed_torrent_send_before->value => [],
    EventReseedEnums::reseed_torrent_send_after->value => [],
    EventReseedEnums::reseed_current_before->value => [],
    EventReseedEnums::reseed_current_after->value => [],
    EventReseedEnums::reseed_all_done->value => [],
    EventTransferEnums::transfer_action_before->value => [],
    EventTransferEnums::transfer_action_after->value => [],
];
```

---

## 九、通知系统

### 9.1 后台通知

**文件**: `app/admin/support/NotifyAdmin.php`

```php
class NotifyAdmin
{
    const string CHANNEL_NAME = 'private-webman-admin';
    
    public static function success(string $msg) { }
    public static function error(string $msg) { }
    public static function warning(string $msg) { }
    public static function info(string $msg) { }
    
    public static function progress(string $type, int $success, int $fail, int $total): void { }
    
    public static function shellOutput(string $name, string $msg): bool { }
}
```

### 9.2 多渠道通知

**文件**: `app/admin/support/NotifyHelper.php`

| 渠道 | 方法 | 说明 |
|------|------|------|
| IYUU | `iyuu()` | IYUU官方通知 |
| Server酱 | `serverChan()` | Server酱推送 |
| Bark | `bark()` | iOS Bark推送 |
| 企业微信 | `weWork()` | 企业微信机器人 |

```php
class NotifyHelper
{
    public static function iyuu(string $text, string $desp = ''): Response
    {
        $token = $config['token'] ?? iyuu_token();
        $client = new IyuuClient(new IyuuAuthenticator($token));
        $message = IyuuMessage::make(['text' => $text, 'desp' => $desp]);
        return $client->send($message);
    }
    
    public static function serverChan(string $title, string $desp = ''): Response { }
    public static function bark(string $title, string $body = '', string $group = ''): Response { }
    public static function weWork(string $content): Response { }
}
```

### 9.3 WebSocket推送

**文件**: `app/admin/support/PushApi.php`

```php
class PushApi
{
    protected static ?Api $api = null;
    
    public static function __callStatic(string $name, array $arguments)
    {
        return static::connection()->{$name}(... $arguments);
    }
    
    protected static function connection(): Api
    {
        if (null === static::$api) {
            static::$api = new Api(
                'http://127.0.0.1:' . parse_url(config('plugin.webman.push.app.api'), PHP_URL_PORT),
                config('plugin.webman.push.app.app_key'),
                config('plugin.webman.push.app.app_secret')
            );
        }
        return static::$api;
    }
}
```

---

## 十、CLI命令

### 10.1 辅种命令

**文件**: `app/command/ReseedCommand.php`

```php
class ReseedCommand extends Command
{
    public const string COMMAND_NAME = 'iyuu:reseed';
    
    protected function configure(): void
    {
        $this->addArgument('crontab_id', InputArgument::REQUIRED, '计划任务ID');
    }
    
    protected function execute(InputInterface $input, OutputInterface $output): int
    {
        $crontab_id = $input->getArgument('crontab_id');
        
        $reseedServices = new ReseedServices(iyuu_token(), (int)$crontab_id);
        $reseedServices->run();
        
        return self::SUCCESS;
    }
}
```

---

## 十一、配置说明

### 11.1 数据库配置

**文件**: `config/database.php`

```php
return [
    'default' => 'mysql',
    'connections' => [
        'mysql' => [
            'driver' => 'mysql',
            'host' => getenv('DB_HOST'),
            'port' => getenv('DB_PORT'),
            'database' => getenv('DB_DATABASE'),
            'username' => getenv('DB_USERNAME'),
            'password' => getenv('DB_PASSWORD'),
            'charset' => 'utf8mb4',
            'collation' => 'utf8mb4_general_ci',
        ],
        'sqlite' => [
            'driver' => 'sqlite',
            'database' => runtime_path('/database.db'),
        ],
    ],
];
```

### 11.2 环境变量

```env
SERVER_LISTEN_PORT=8787
IYUU_TOKEN=your_token_here
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=iyuu
DB_USERNAME=root
DB_PASSWORD=password
CLOUD_ACCESS_TOKEN=
CLOUD_DEBUG=false
```

### 11.3 Nginx反向代理

```nginx
location ^~ / {
    proxy_pass http://127.0.0.1:8787;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
}

location /app/d9422b72cffad23098ad301eea0f8419 {
    proxy_pass http://127.0.0.1:3131;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
}
```

---

## 十二、与 PT-Forward 对比分析

### 12.1 功能对比

| 功能 | IYUUPlus | PT-Forward |
|------|----------|------------|
| **辅种方式** | IYUU服务器匹配 | 本地RSS爬取 |
| **种子来源** | 多站点自动匹配 | 指定源站点 |
| **上传目标** | 无(仅辅种) | 自动上传到目标站 |
| **转移做种** | 支持 | 不支持 |
| **RSS订阅** | 支持 | 支持 |
| **WebUI** | 完整后台 | 无 |
| **运行模式** | 常驻内存 | 定时脚本 |

### 12.2 架构对比

| 维度 | IYUUPlus | PT-Forward |
|------|----------|------------|
| **语言** | PHP | Python |
| **框架** | Webman/Workerman | 无框架 |
| **数据库** | MySQL | SQLite/文件 |
| **并发** | 多进程 | 单进程 |
| **扩展性** | 插件机制 | 模块化 |

### 12.3 集成可能性

IYUUPlus 可以作为 PT-Forward 的**辅种扩展**：

```python
class IYUUClient:
    def __init__(self, token: str):
        self.base_url = "http://2025.iyuu.cn"
        self.token = token
    
    def get_reseed_data(self, info_hash: str, sid_sha1: str):
        response = requests.post(
            f"{self.base_url}/reseed/index/index",
            data={
                "hash": json.dumps([info_hash]),
                "sha1": hashlib.sha1(json.dumps([info_hash]).encode()).hexdigest(),
                "sid_sha1": sid_sha1,
                "version": "1.0.0"
            }
        )
        return response.json()["data"]
```

---

## 十三、总结

### 13.1 项目优势

1. **成熟的辅种生态**: 依托IYUU服务器，支持100+站点
2. **完整的WebUI**: 可视化配置和监控
3. **多下载器支持**: qBittorrent + Transmission
4. **常驻内存运行**: 高性能，实时响应
5. **插件机制**: 良好的扩展性
6. **事件驱动**: 灵活的事件系统支持自定义扩展
7. **限速保护**: 站点级限速配置，防止封禁

### 13.2 与PT-Forward互补

| 场景 | 推荐方案 |
|------|----------|
| 多站点自动辅种 | IYUUPlus |
| 定向种子转发上传 | PT-Forward |
| RSS自动下载 | 两者皆可 |
| 跨下载器转移 | IYUUPlus |

### 13.3 集成建议

PT-Forward 可以借鉴 IYUUPlus 的以下设计：

1. **站点驱动模式**: 抽象站点接口，支持多种站点架构
2. **下载器抽象层**: 统一的下载器操作接口
3. **事件系统**: 基于事件的扩展机制
4. **通知模块**: 多渠道通知能力
5. **限速机制**: 站点级限速保护

---

## 附录：关键文件索引

| 文件路径 | 说明 |
|----------|------|
| `app/admin/services/reseed/ReseedServices.php` | 自动辅种核心服务 |
| `app/admin/services/reseed/ReseedDownloadServices.php` | 辅种下载服务 |
| `app/admin/services/transfer/TransferServices.php` | 转移做种服务 |
| `app/admin/services/rss/RssServices.php` | RSS订阅服务 |
| `app/admin/services/client/ClientServices.php` | 下载器客户端服务 |
| `app/admin/services/site/NexusPHP.php` | NexusPHP站点配置 |
| `app/admin/support/NotifyAdmin.php` | 后台通知 |
| `app/admin/support/NotifyHelper.php` | 多渠道通知 |
| `app/admin/support/PushApi.php` | WebSocket推送 |
| `app/enums/EventReseedEnums.php` | 辅种事件枚举 |
| `app/enums/EventTransferEnums.php` | 转移事件枚举 |
| `app/model/Client.php` | 下载器模型 |
| `app/model/Site.php` | 站点模型 |
| `app/model/Reseed.php` | 辅种记录模型 |
| `app/model/Transfer.php` | 转移记录模型 |
| `app/Bootstrap.php` | 启动引导 |
| `config/event.php` | 事件配置 |
| `config/process.php` | 进程配置 |
| `composer/site-manager/src/BaseDriver.php` | 站点驱动基类 |
| `composer/site-manager/src/SiteManager.php` | 站点管理器 |
| `composer/bittorrent-client/src/Clients.php` | 下载器客户端抽象 |
| `composer/reseed-client/src/Client.php` | IYUU API客户端 |
