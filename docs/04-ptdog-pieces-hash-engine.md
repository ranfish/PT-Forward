# PTDog 项目深度分析报告

## 项目概述

PTDog 是一个开源的 PT 站点自动辅种工具，专门用于支持开放 pieces_hash 查询的站点。该工具通过监控下载器中的已下载种子，自动在多个 PT 站点中查找相同的资源并进行辅种，帮助用户增加上传量和保持分享率。

**版本**: v1.0  
**开发语言**: Go 1.21.2  
**核心功能**: 自动辅种、多下载器支持、多站点支持、pieces_hash 匹配

---

## 一、项目架构与设计模式

### 1.1 整体架构

PTDog 采用分层架构设计，主要分为以下几个层次：

```
PTDog
├── main.go                 # 应用入口
├── app/
│   ├── app.go             # 应用协调器
│   ├── config/            # 配置管理
│   ├── client/            # 下载器客户端抽象
│   ├── reseed/            # 辅种核心逻辑
│   └── http/              # HTTP 服务器
└── config.json            # 配置文件
```

### 1.2 核心设计模式

#### 1.2.1 工厂模式 (Factory Pattern)

在 [reseed.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/reseed.go#L70-L84) 中，使用工厂模式创建不同类型的下载器客户端：

```go
func (r *Reseed) client(config config.Client) (client.IClient, error) {
    switch config.Type {
    case client.TypeTransmission:
        return client.NewTransmission(conf)
    case client.TypeQbittorrent:
        return client.NewQbittorrent(conf)
    }
    return nil, fmt.Errorf("the type of the client is %d, not supported", config.Type)
}
```

#### 1.2.2 接口抽象 (Interface Abstraction)

[client.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/client.go#L3-L8) 定义了统一的下载器接口：

```go
type IClient interface {
    Type() Type
    String() string
    Torrents([]string) ([]*Torrent, error)
    TorrentAdd(filename, dir string) error
}
```

这种设计允许轻松扩展新的下载器类型，只需实现 IClient 接口即可。

#### 1.2.3 生产者-消费者模式 (Producer-Consumer)

项目在多个地方使用了生产者-消费者模式：

1. **Scanner → Querier**: Scanner 扫描种子后推送到 Querier 队列
2. **Querier → Seeder**: Querier 查询站点后推送到 Seeder 队列

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L23-L31):

```go
var querier = &Querier{
    queue: make(chan *Batch, 8),  // 缓冲队列，容量 8
}

func (q *Querier) Push(batch *Batch) {
    q.queue <- batch  // 生产者推送
}

func (q *Querier) Run() {
    go func() {
        for batch := range q.queue {  // 消费者处理
            q.handler(batch)
        }
    }()
}
```

#### 1.2.4 单例模式 (Singleton Pattern)

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L22-L23) 和 [seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L35-L36) 使用包级变量实现单例：

```go
var querier = &Querier{
    queue: make(chan *Batch, 8),
}

var seeder = &Seeder{
    queue: make(chan Seed, 8),
}
```

这种设计确保全局只有一个 Querier 和 Seeder 实例，统一管理队列处理。

#### 1.2.5 策略模式 (Strategy Pattern)

不同下载器的实现采用策略模式，通过接口统一调用：

- [transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go): Transmission 策略
- [qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go): qBittorrent 策略

---

## 二、辅种核心算法与流程

### 2.1 辅种流程图

```
启动
  ↓
初始化 Scanner (每个下载器一个)
  ↓
定时扫描下载器目录 (默认 15 分钟)
  ↓
读取 .torrent 文件，计算 info_hash 和 pieces_hash
  ↓
查询下载器 API 获取种子状态
  ↓
过滤已完成的种子
  ↓
推送到 Querier 队列
  ↓
Querier 批量查询站点 API (按 limit 分批)
  ↓
匹配 pieces_hash，获取站点资源 ID
  ↓
推送到 Seeder 队列
  ↓
检查缓存，避免重复辅种
  ↓
调用下载器 API 添加种子
  ↓
缓存成功记录
```

### 2.2 Scanner - 种子扫描器

[scanner.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go#L29-L37) 实现了定时扫描逻辑：

```go
func (s *Scanner) Run() {
    go func() {
        ticker := time.NewTicker(s.sleep)  // 定时器
        for {
            s.scan()
            <-ticker.C  // 等待下一次触发
        }
    }()
}
```

#### 2.2.1 种子文件解析

[scanner.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go#L90-L113) 使用 `github.com/anacrolix/torrent/metainfo` 库解析 .torrent 文件：

```go
func (s *Scanner) load() (map[string]string, error) {
    entries, err := os.ReadDir(s.dir)
    var hashes = make(map[string]string)
    
    for _, entry := range entries {
        path := path.Join(s.dir, entry.Name())
        
        meta, err := metainfo.LoadFromFile(path)
        if err != nil {
            continue
        }
        
        info, err := meta.UnmarshalInfo()
        if err != nil {
            continue
        }
        
        hash := meta.HashInfoBytes().HexString()      // info_hash
        piecesHash := metainfo.HashBytes(info.Pieces).HexString()  // pieces_hash
        hashes[hash] = piecesHash
    }
    
    return hashes, nil
}
```

**关键点**:
- `info_hash`: 种子的唯一标识符
- `pieces_hash`: 种子内容的哈希值，用于跨站点匹配相同资源
- 使用 `continue` 跳过解析失败的文件，不中断整个扫描过程

#### 2.2.2 种子状态过滤

[scanner.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go#L48-L73) 只处理已完成的种子：

```go
func (s *Scanner) torrents() (map[string]*client.Torrent, error) {
    hashes, err := s.load()
    // ...
    
    data, err := s.client.Torrents(queries)
    
    var torrents = make(map[string]*client.Torrent)
    for _, t := range data {
        if !t.IsFinished {  // 只处理已完成的种子
            continue
        }
        
        if piecesHash, ok := hashes[t.InfoHash]; ok {
            t.PiecesHash = piecesHash
            torrents[t.PiecesHash] = t  // 以 pieces_hash 为键
        }
    }
    
    return torrents, nil
}
```

### 2.3 Querier - 站点查询器

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L49-L67) 实现了批量查询逻辑：

#### 2.3.1 并发查询多个站点

```go
func (q *Querier) handler(batch *Batch) {
    if len(q.websites) == 0 {
        return
    }
    
    batch.init()
    
    q.wg.Add(len(q.websites))  // 等待所有站点查询完成
    for _, website := range q.websites {
        go q.batch(batch, website)  // 每个站点一个 goroutine
    }
    q.wg.Wait()  // 等待所有查询完成
}
```

#### 2.3.2 分批查询优化

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L69-L77) 根据站点的 limit 参数分批查询：

```go
func (q *Querier) batch(batch *Batch, website *Website) {
    defer q.wg.Done()
    
    length := len(batch.pieces)
    
    for i := 0; i < length; i++ {
        s := i * website.limit
        e := s + website.limit
        if e >= length {
            q.query(batch, website, batch.pieces[s:])  // 最后一批
            break
        }
        q.query(batch, website, batch.pieces[s:e])  // 中间批次
    }
}
```

**设计优势**:
- 避免单次请求数据量过大
- 适应不同站点的 API 限制
- 提高查询效率

### 2.4 Seeder - 种子播种器

[seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L47-L51) 实现了种子添加逻辑：

```go
func (s *Seeder) Run() {
    go func() {
        for seed := range s.queue {  // 持续处理队列
            go s.handler(seed)  // 每个 seed 一个 goroutine
        }
    }()
}
```

#### 2.4.1 缓存去重

[seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L53-L69) 使用文件缓存避免重复辅种：

```go
func (s *Seeder) handler(seed Seed) {
    url := seed.url()
    if cache.Has(url) {  // 检查是否已辅种
        return
    }
    
    if err := seed.client.TorrentAdd(url, seed.torrent.DownloadPath); err != nil {
        seed.log(log.Err(err)).Msg("辅种失败")
        return
    }
    
    if err := cache.Set(url, true, cache.Forever); err != nil {
        log.Err(err).Msg("缓存失败")
    }
    
    seed.log(log.Info()).Msg("辅种成功")
}
```

**缓存机制**:
- 使用 `github.com/gookit/cache` 库
- 文件存储在 `.cache` 目录
- 永久存储 (cache.Forever)
- 以种子下载 URL 为键

---

## 三、下载器客户端实现

### 3.1 客户端接口设计

[client.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/client.go#L3-L8) 定义了统一的接口：

```go
type IClient interface {
    Type() Type              // 返回客户端类型
    String() string          // 返回客户端描述
    Torrents([]string) ([]*Torrent, error)  // 获取种子列表
    TorrentAdd(filename, dir string) error  // 添加种子
}
```

### 3.2 Transmission 客户端

[transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go#L15-L28) 使用 `github.com/hekmon/transmissionrpc/v2` 库：

```go
func NewTransmission(conf Config) (*Transmission, error) {
    c, err := transmissionrpc.New(conf.Host, conf.Username, conf.Password, &transmissionrpc.AdvancedConfig{
        HTTPS:       conf.Https,
        Port:        conf.Port,
        HTTPTimeout: conf.Timeout,
    })
    if err != nil {
        return nil, err
    }
    
    return &Transmission{
        conf: conf,
        c:    c,
    }, nil
}
```

#### 3.2.1 获取种子列表

[transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go#L40-L64) 使用超时上下文：

```go
func (t *Transmission) Torrents(hashes []string) ([]*Torrent, error) {
    ctx, cencel := context.WithTimeout(context.Background(), t.conf.Timeout)
    defer cencel()  // 确保上下文取消
    
    data, err := t.c.TorrentGetAllForHashes(ctx, hashes)
    if err != nil {
        return nil, err
    }
    
    var torrents []*Torrent
    for _, t := range data {
        if t.HashString == nil || t.DownloadDir == nil || t.Status == nil {
            continue  // 跳过无效数据
        }
        torrents = append(torrents, &Torrent{
            InfoHash:     *t.HashString,
            DownloadPath: *t.DownloadDir,
            Status:       TorrentStatus(*t.Status),
            Name:         *t.Name,
            IsFinished:   t.DoneDate.Unix() > 0,  // 通过完成日期判断
        })
    }
    
    return torrents, nil
}
```

#### 3.2.2 添加种子

[transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go#L66-L73):

```go
func (t *Transmission) TorrentAdd(filename, dir string) error {
    ctx, cencel := context.WithTimeout(context.Background(), t.conf.Timeout)
    defer cencel()
    
    _, err := t.c.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
        Filename:    &filename,
        DownloadDir: &dir,
    })
    return err
}
```

### 3.3 qBittorrent 客户端

[qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go#L18-L32) 使用 `github.com/autobrr/go-qbittorrent` 库：

```go
func NewQbittorrent(conf Config) (*Qbittorrent, error) {
    scheme := "http"
    if conf.Https {
        scheme = "https"
    }
    
    host := fmt.Sprintf("%s://%s:%d", scheme, conf.Host, conf.Port)
    
    c := qbittorrent.NewClient(qbittorrent.Config{
        Host:          host,
        Username:      conf.Username,
        Password:      conf.Password,
        TLSSkipVerify: !conf.Https,
    })
    
    return &Qbittorrent{
        conf: conf,
        c:    c,
    }, nil
}
```

#### 3.3.1 获取种子列表

[qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go#L40-L62):

```go
func (q *Qbittorrent) Torrents(hashes []string) ([]*Torrent, error) {
    if err := q.c.Login(); err != nil {
        return nil, err
    }
    
    data, err := q.c.GetTorrents(qbittorrent.TorrentFilterOptions{
        Hashes: hashes,
    })
    if err != nil {
        return nil, err
    }
    
    var torrents []*Torrent
    for _, t := range data {
        torrents = append(torrents, &Torrent{
            Name:         t.Name,
            InfoHash:     t.Hash,
            DownloadPath: t.SavePath,
            Status:       q.transform(t.State),
            IsFinished:   t.CompletionOn > 0,  // 通过完成时间判断
        })
    }
    
    return torrents, nil
}
```

#### 3.3.2 状态转换

[qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go#L78-L86):

```go
func (q *Qbittorrent) transform(status qbittorrent.TorrentState) TorrentStatus {
    switch status {
    case qbittorrent.TorrentStateUploading, 
         qbittorrent.TorrentStateStalledUp, 
         qbittorrent.TorrentStateForcedUp:
        return TorrentStatusSeed
    }
    return TorrentStatus(-1)
}
```

#### 3.3.3 添加种子

[qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go#L64-L76):

```go
func (q *Qbittorrent) TorrentAdd(url, dir string) error {
    if err := q.c.Login(); err != nil {
        return err
    }
    
    skipChecking := "false"
    if q.conf.SkipChecking {
        skipChecking = "true"
    }
    
    return q.c.AddTorrentFromUrl(url, map[string]string{
        "savepath":      dir,
        "skip_checking": skipChecking,  // 可选：跳过校验
    })
}
```

### 3.4 种子状态定义

[torrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/torrent.go#L1-L35) 定义了完整的种子状态：

```go
type TorrentStatus int

const (
    TorrentStatusStopped      TorrentStatus = 0  // 已停止
    TorrentStatusCheckWait    TorrentStatus = 1  // 等待校验
    TorrentStatusCheck        TorrentStatus = 2  // 校验中
    TorrentStatusDownloadWait TorrentStatus = 3  // 等待下载
    TorrentStatusDownload     TorrentStatus = 4  // 下载中
    TorrentStatusSeedWait     TorrentStatus = 5  // 等待做种
    TorrentStatusSeed         TorrentStatus = 6  // 做种中
    TorrentStatusIsolated     TorrentStatus = 7  // 无连接
)

type Torrent struct {
    Name         string        // 种子名称
    InfoHash     string        // info_hash
    PiecesHash   string        // pieces_hash
    DownloadPath string        // 下载路径
    Status       TorrentStatus // 状态
    IsFinished   bool          // 是否完成
}
```

---

## 四、站点 API 适配与 pieces_hash 查询机制

### 4.1 站点 API 统一接口

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go#L1-L96) 实现了站点 API 的统一适配：

```go
type Website struct {
    name     string  // 站点名称
    domain   string  // 站点域名
    api      string  // API 地址
    passkey  string  // 用户密钥
    download string  // 自定义下载链接
    limit    int     // 每批次查询限额
}
```

### 4.2 pieces_hash 查询机制

#### 4.2.1 查询请求

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go#L79-L87) 构建 POST 请求：

```go
func (w *Website) params(hashes []string) ([]byte, error) {
    params := map[string]interface{}{
        "passkey":     w.passkey,
        "pieces_hash": hashes,  // 批量查询
    }
    return json.Marshal(params)
}
```

#### 4.2.2 HTTP 请求

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go#L60-L76) 发送 HTTP 请求：

```go
func (w *Website) do(hashes []string) ([]byte, error) {
    params, err := w.params(hashes)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequest("POST", w.api, bytes.NewReader(params))
    if err != nil {
        return nil, err
    }
    for k, v := range headers {
        req.Header.Set(k, v)
    }
    
    resp, err := httpc.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()  // 确保响应体关闭
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("http status: %s", resp.Status)
    }
    
    return io.ReadAll(resp.Body)
}
```

**HTTP 客户端配置**:
```go
var (
    headers = map[string]string{
        "Content-Type": "application/json",
        "Accept":       "application/json",
    }
    httpc = &http.Client{
        Timeout: time.Minute,  // 1 分钟超时
    }
)
```

#### 4.2.3 响应解析

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go#L29-L48) 解析响应：

```go
func (w *Website) Query(hashes []string) (map[string]int, error) {
    data, err := w.do(hashes)
    if err != nil {
        return nil, err
    }
    
    var result struct {
        Data map[string]int `json:"data"`
    }
    if err := json.Unmarshal(data, &result); err != nil {
        result.Data = nil
    }
    
    if result.Data != nil {
        return result.Data, nil  // 返回 pieces_hash -> 资源 ID 的映射
    }
    return map[string]int{}, nil
}
```

**响应格式**:
```json
{
  "data": {
    "abc123...": 12345,
    "def456...": 67890
  }
}
```

### 4.3 下载链接生成

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go#L89-L96) 支持自定义下载链接：

```go
func (w *Website) FormatDownload(id int) string {
    if w.download != "" {
        url := strings.Replace(w.download, "{id}", strconv.Itoa(id), 1)
        return strings.Replace(url, "{passkey}", w.passkey, 1)
    }
    // 默认格式：NexusPHP 风格
    return fmt.Sprintf("%s/download.php?id=%d&passkey=%s", w.domain, id, w.passkey)
}
```

**支持的占位符**:
- `{id}`: 资源 ID
- `{passkey}`: 用户密钥

### 4.4 支持的站点

根据 README.md，PTDog 支持以下站点（开放 pieces_hash 查询）：

| 站点名称 | 域名 | API 地址 |
|---------|------|---------|
| 红叶 | https://leaves.red | https://leaves.red/api/pieces-hash |
| 猪猪 | https://piggo.me | https://api.piggo.me/api/pieces-hash |
| ultrahd | https://ultrahd.net | https://ultrahd.net/api/pieces-hash |
| zmpt(织梦) | https://zmpt.cc | https://zmpt.cc/api/pieces-hash |
| hdtime | https://hdtime.org | https://hdtime.org/api/pieces-hash |
| 月月 | https://pt.keepfrds.com | https://pt.keepfrds.com/api/torrents/pieces-hash |
| ptlsp | https://www.ptlsp.com | https://www.ptlsp.com/api/pieces-hash |
| 憨憨 | https://hhanclub.top | https://hhanclub.top/npapi/pieces-hash |
| 大青虫 | https://cyanbug.net | https://cyanbug.net/api/pieces-hash |
| icc | https://www.icc2022.com | https://www.icc2022.com/api/pieces-hash |
| 1ptba | https://1ptba.com | https://1ptba.com/api/pieces-hash |
| ptcafe | https://ptcafe.club | https://ptcafe.club/api/pieces-hash |
| kufei | https://kufei.org | https://kufei.org/api/pieces-hash |
| rousi | https://rousi.zip | https://rousi.zip/api/pieces-hash |
| 东樱 | https://wintersakura.net | https://wintersakura.net/api/pieces-hash |
| oshen | https://www.oshen.win | https://www.oshen.win/api/pieces-hash |
| 明教 | https://hdpt.xyz | https://hdpt.xyz/api/pieces-hash |
| 2xfree | https://pt.2xfree.org | https://pt.2xfree.org/api/pieces-hash |
| 阿童木 | https://hdatmos.club | https://hdatmos.club/api/pieces-hash |
| 3wmg明教 | https://www.3wmg.com | https://www.3wmg.com/api/pieces-hash |

---

## 五、并发控制与同步机制

### 5.1 并发模型

PTDog 采用多 goroutine 并发处理模型：

```
主 goroutine
  ├── Scanner goroutine (每个下载器一个)
  ├── Querier goroutine (处理队列)
  │   ├── 站点查询 goroutine (每个站点一个)
  └── Seeder goroutine (处理队列)
      ├── 辅种 goroutine (每个种子一个)
```

### 5.2 WaitGroup 同步

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L29) 使用 `sync.WaitGroup` 等待所有站点查询完成：

```go
type Querier struct {
    websites []*Website
    queue    chan *Batch
    wg       sync.WaitGroup  // WaitGroup
}

func (q *Querier) handler(batch *Batch) {
    batch.init()
    
    q.wg.Add(len(q.websites))  // 添加等待任务
    for _, website := range q.websites {
        go q.batch(batch, website)
    }
    q.wg.Wait()  // 等待所有任务完成
}

func (q *Querier) batch(batch *Batch, website *Website) {
    defer q.wg.Done()  // 完成一个任务
    
    // 查询逻辑
}
```

### 5.3 Channel 缓冲

使用缓冲 channel 控制并发量：

```go
// Querier 队列
queue: make(chan *Batch, 8),  // 容量 8

// Seeder 队列
queue: make(chan Seed, 8),  // 容量 8
```

**设计考虑**:
- 缓冲大小为 8，平衡内存使用和吞吐量
- 避免无限增长导致内存溢出
- 提供背压机制

### 5.4 Context 超时控制

[transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go#L40-L41) 使用 `context.WithTimeout` 控制请求超时：

```go
ctx, cencel := context.WithTimeout(context.Background(), t.conf.Timeout)
defer cencel()  // 确保上下文取消

data, err := t.c.TorrentGetAllForHashes(ctx, hashes)
```

**优势**:
- 防止请求挂起
- 及时释放资源
- 提高系统可靠性

### 5.5 Defer 资源清理

使用 `defer` 确保资源释放：

```go
// HTTP 响应体关闭
defer resp.Body.Close()

// 上下文取消
defer cencel()

// WaitGroup 完成
defer q.wg.Done()

// 主程序等待
defer func() {
    fmt.Println("Press 'Enter' to continue...")
    fmt.Scanln()
}()
```

---

## 六、缓存机制与去重策略

### 6.1 缓存初始化

[seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L11-L14) 使用 `github.com/gookit/cache` 库：

```go
func init() {
    cache.Register(cache.DvrFile, cache.NewFileCache(".cache"))
    cache.DefaultUse(cache.DvrFile)
}
```

**缓存配置**:
- 驱动类型: 文件缓存 (DvrFile)
- 存储路径: `.cache` 目录
- 默认使用: 文件缓存

### 6.2 去重逻辑

[seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L53-L69) 实现去重：

```go
func (s *Seeder) handler(seed Seed) {
    url := seed.url()
    if cache.Has(url) {  // 检查是否已存在
        return  // 跳过重复辅种
    }
    
    if err := seed.client.TorrentAdd(url, seed.torrent.DownloadPath); err != nil {
        seed.log(log.Err(err)).Msg("辅种失败")
        return
    }
    
    if err := cache.Set(url, true, cache.Forever); err != nil {
        log.Err(err).Msg("缓存失败")
    }
    
    seed.log(log.Info()).Msg("辅种成功")
}
```

**去重策略**:
- 键: 种子下载 URL
- 值: true
- 过期时间: 永久 (cache.Forever)

### 6.3 缓存优势

1. **避免重复辅种**: 已辅种的种子不会再次尝试
2. **提高效率**: 减少 API 调用和下载器操作
3. **持久化**: 重启程序后缓存仍然有效
4. **简单可靠**: 基于文件系统，无需额外依赖

---

## 七、配置管理与热加载机制

### 7.1 配置结构

[config.go](file:///home/incast/PT-Forward/examples/ptdog/app/config/config.go#L11-L18) 定义配置结构：

```go
type Config struct {
    Http     Http      `json:"http"`
    System   System    `json:"system"`
    Clients  []Client  `json:"clients"`
    Websites []Website `json:"websites"`
}
```

### 7.2 配置加载

[config.go](file:///home/incast/PT-Forward/examples/ptdog/app/config/config.go#L20-L42) 实现配置加载：

```go
var Conf = &Config{
    Http: Http{
        Addr: "127.0.0.1:1688",  // 默认地址
    },
    System: System{
        Sleep: 10,  // 默认扫描周期 10 分钟
    },
}

func Exist(path string) bool {
    if _, err := os.Stat(path); err != nil {
        return false
    }
    return true
}

func Load(path string) error {
    if !Exist(path) {
        return nil  // 配置文件不存在时使用默认值
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    
    return json.Unmarshal(data, Conf)
}
```

**加载流程**:
1. 检查配置文件是否存在
2. 读取配置文件内容
3. JSON 解析到 Conf 全局变量
4. 文件不存在时使用默认值

### 7.3 配置项详解

#### 7.3.1 HTTP 配置

[http.go](file:///home/incast/PT-Forward/examples/ptdog/app/config/http.go#L1-L5):

```go
type Http struct {
    Addr string `json:"addr"`  // HTTP 服务器地址
}
```

#### 7.3.2 系统配置

[system.go](file:///home/incast/PT-Forward/examples/ptdog/app/config/system.go#L1-L14):

```go
type System struct {
    Sleep int64 `json:"sleep"`  // 扫描周期（分钟）
}

func (c System) SleepDuration() time.Duration {
    if c.Sleep <= 0 {
        c.Sleep = 60  // 默认 60 分钟
    }
    return time.Duration(c.Sleep) * time.Minute
}
```

#### 7.3.3 下载器配置

[client.go](file:///home/incast/PT-Forward/examples/ptdog/app/config/client.go#L1-L18):

```go
type Client struct {
    Enable       bool        `json:"enable"`        // 是否启用
    Https        bool        `json:"https"`         // 是否使用 HTTPS
    Type         client.Type `json:"type"`          // 客户端类型
    Host         string      `json:"host"`          // 主机地址
    Username     string      `json:"username"`      // 用户名
    Password     string      `json:"password"`      // 密码
    Port         uint16      `json:"port"`          // 端口
    Timeout      int64       `json:"timeout"`       // 超时时间（秒）
    Dir          string      `json:"dir"`           // 种子目录
    SkipChecking bool        `json:"skip_checking"` // 是否跳过校验
}
```

#### 7.3.4 站点配置

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/config/website.go#L1-L11):

```go
type Website struct {
    Enable   bool   `json:"enable"`   // 是否启用
    Name     string `json:"name"`     // 站点名称
    Domain   string `json:"domain"`   // 站点域名
    Api      string `json:"api"`      // API 地址
    Passkey  string `json:"passkey"`  // 用户密钥
    Download string `json:"download"` // 自定义下载链接
    Limit    int    `json:"limit"`    // 每批次查询限额
}
```

### 7.4 热加载机制

**注意**: PTDog 当前版本**不支持热加载**。配置只在程序启动时加载一次，修改配置需要重启程序。

**潜在改进方案**:
1. 使用 `fsnotify` 监听配置文件变化
2. 使用信号处理 (SIGHUP) 触发重载
3. 提供 HTTP API 手动触发重载

---

## 八、日志系统与错误处理

### 8.1 日志系统

[main.go](file:///home/incast/PT-Forward/examples/ptdog/main.go#L13-L18) 使用 `github.com/rs/zerolog` 库：

```go
func init() {
    log.Logger = log.Output(zerolog.ConsoleWriter{
        Out: os.Stdout,
        // TimeFormat: time.RFC3339,
    })
}
```

**日志特性**:
- 控制台输出
- 结构化日志
- JSON 格式（生产环境推荐）
- 支持日志级别

### 8.2 日志使用示例

```go
// 信息日志
log.Info().Msgf("PTDog %s", Version)
log.Info().Int("匹配种子", len(torrents)).Msg("扫描完成")

// 错误日志
log.Err(err).Str("配置", "config.json").Msg("配置加载失败")
log.Err(err).Str("站点", website.String()).Msg("查询失败")

// 带上下文的日志
seed.log(log.Err(err)).Msg("辅种失败")
seed.log(log.Info()).Msg("辅种成功")
```

### 8.3 错误处理策略

#### 8.3.1 错误检查

项目中所有可能失败的操作都进行了错误检查：

```go
// 文件操作
entries, err := os.ReadDir(s.dir)
if err != nil {
    return nil, err
}

// 网络请求
data, err := w.do(hashes)
if err != nil {
    return nil, err
}

// JSON 解析
if err := json.Unmarshal(data, &result); err != nil {
    result.Data = nil
}

// 客户端调用
if err := seed.client.TorrentAdd(url, seed.torrent.DownloadPath); err != nil {
    seed.log(log.Err(err)).Msg("辅种失败")
    return
}
```

#### 8.3.2 错误传播

错误通过返回值向上传播：

```go
func (r *Reseed) Run() error {
    scanners, err := r.scanners()
    if err != nil {
        return err  // 向上传播错误
    }
    // ...
}

func main() {
    if err := config.Load("config.json"); err != nil {
        log.Err(err).Str("配置", "config.json").Msg("配置加载失败")
        return
    }
    
    if err := app.New().Run(); err != nil {
        log.Err(err).Send()
    }
}
```

#### 8.3.3 容错处理

**扫描器容错**:

[scanner.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go#L100-L106) 跳过无法解析的文件：

```go
for _, entry := range entries {
    path := path.Join(s.dir, entry.Name())
    
    meta, err := metainfo.LoadFromFile(path)
    if err != nil {
        continue  // 跳过解析失败的文件
    }
    
    info, err := meta.UnmarshalInfo()
    if err != nil {
        continue  // 跳过无法解析的文件
    }
    // ...
}
```

**查询器容错**:

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L82-L87) 单个站点失败不影响其他站点：

```go
func (q *Querier) query(batch *Batch, website *Website, hashes []string) {
    data, err := website.Query(hashes)
    if err != nil {
        log.Err(err).Str("站点", website.String()).Msg("查询失败")
        return  // 只记录错误，不中断流程
    }
    // ...
}
```

**播种器容错**:

[seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L61-L65) 辅种失败只记录日志：

```go
if err := seed.client.TorrentAdd(url, seed.torrent.DownloadPath); err != nil {
    seed.log(log.Err(err)).Msg("辅种失败")
    return  // 不重试，继续处理下一个
}
```

### 8.4 错误处理特点

1. **不使用 panic**: 项目中没有使用 `panic`，所有错误都通过返回值处理
2. **不使用 recover**: 没有 `recover` 逻辑，依赖 Go 的错误传播机制
3. **容错优先**: 单个失败不影响整体流程
4. **日志记录**: 所有错误都记录到日志
5. **不重试**: 当前实现没有重试机制

---

## 九、HTTP 服务器与管理面板

### 9.1 HTTP 服务器实现

[server.go](file:///home/incast/PT-Forward/examples/ptdog/app/http/server.go#L1-L38) 使用标准库 `net/http`：

```go
type Server struct {
    addr string
}

func NewServer() *Server {
    return &Server{
        addr: config.Conf.Http.Addr,
    }
}

func (s *Server) Run() error {
    s.info()
    http.HandleFunc("/", s.view)
    return http.ListenAndServe(s.addr, nil)
}

func (s *Server) view(w http.ResponseWriter, r *http.Request) {
    t, _ := template.ParseFS(view, "view/index.html")
    t.Execute(w, nil)
}

func (s *Server) info() {
    log.Info().Msgf("控制面板: %s (未实现，自行配置config.json)", s.addr)
}
```

**特点**:
- 使用 `embed.FS` 嵌入静态文件
- 简单的 HTTP 服务器
- 只有一个路由 `/`
- 未实现完整的管理功能

### 9.2 前端页面

[index.html](file:///home/incast/PT-Forward/examples/ptdog/app/http/view/index.html) 使用 Bootstrap 5：

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>PTDog</title>
    <link href="https://lf6-cdn-tos.bytecdntp.com/cdn/expire-1-M/bootstrap/5.1.3/css/bootstrap.min.css" rel="stylesheet">
  </head>
  <body>
    <!-- 导航栏 -->
    <nav class="navbar navbar-expand-md navbar-dark fixed-top bg-dark">
      <div class="container">
        <a class="navbar-brand" href="#">PTDog</a>
        <div class="navbar-nav me-auto mb-2 mb-md-0">
          <span class="badge">PT站点自动辅种工具，支持开放pieces_hash查询的站点。未适配的站点请联系！QQ群: 123456789</span>
        </div>
        <span class="badge">v1.0</span>
      </div>
    </nav>

    <!-- 标签页 -->
    <ul class="nav nav-tabs" id="myTab" role="tablist">
      <li class="nav-item">
        <button class="nav-link active" data-bs-toggle="tab" data-bs-target="#home">站点</button>
      </li>
      <li class="nav-item">
        <button class="nav-link" data-bs-toggle="tab" data-bs-target="#profile">下载器</button>
      </li>
      <li class="nav-item">
        <button class="nav-link" data-bs-toggle="tab" data-bs-target="#contact">系统</button>
      </li>
    </ul>

    <!-- 站点标签页内容 -->
    <div class="tab-pane fade show active" id="home">
      <button type="button" class="btn btn-primary" data-bs-toggle="modal" data-bs-target="#website">新增站点</button>
    </div>

    <!-- 新增站点模态框 -->
    <div class="modal fade" id="website" tabindex="-1">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">站点</h5>
          </div>
          <div class="modal-body">
            <form>
              <div class="mb-3">
                <label for="name" class="col-form-label">名称:</label>
                <input type="text" class="form-control" id="name" placeholder="站点名称">
              </div>
              <div class="mb-3">
                <label for="domain" class="col-form-label">域名:</label>
                <input class="form-control" id="domain" placeholder="站点域名">
              </div>
              <div class="mb-3">
                <label for="api" class="col-form-label">api:</label>
                <input class="form-control" id="api" placeholder="pieces hash查询的接口地址">
              </div>
              <div class="mb-3">
                <label for="passkey" class="col-form-label">passkey:</label>
                <input class="form-control" id="passkey" placeholder="你账户的密钥">
              </div>
              <div class="mb-3">
                <label for="limit" class="col-form-label">每批次查询限额:</label>
                <input class="form-control" id="limit" placeholder="默认0，每批次查询100个Hash">
              </div>
              <div class="mb-3">
                <label for="download" class="col-form-label">自定义种子下载链接:</label>
                <input class="form-control" id="download" placeholder="默认留空">
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-primary">保存</button>
          </div>
        </div>
      </div>
    </div>

    <script src="https://lf6-cdn-tos.bytecdntp.com/cdn/expire-1-M/bootstrap/5.1.3/js/bootstrap.min.js"></script>
  </body>
</html>
```

**前端状态**:
- 仅提供 UI 框架
- 未实现后端 API
- 未实现数据绑定
- 未实现配置保存
- 需要手动编辑 `config.json`

### 9.3 管理功能缺失

根据 [app.go](file:///home/incast/PT-Forward/examples/ptdog/app/app.go#L15-L24) 的日志输出：

```go
func (s *Server) info() {
    log.Info().Msgf("控制面板: %s (未实现，自行配置config.json)", s.addr)
}
```

**缺失功能**:
1. 站点列表查看
2. 站点添加/编辑/删除
3. 下载器列表查看
4. 下载器添加/编辑/删除
5. 辅种日志查看
6. 辅种统计信息
7. 配置实时编辑
8. 辅种状态监控

---

## 十、依赖库与第三方集成

### 10.1 核心依赖

[go.mod](file:///home/incast/PT-Forward/examples/ptdog/go.mod#L5-L11) 列出了主要依赖：

```go
require (
    github.com/anacrolix/torrent v1.53.1          // Torrent 文件解析
    github.com/autobrr/go-qbittorrent v1.5.0      // qBittorrent 客户端
    github.com/gookit/cache v0.4.0                // 缓存库
    github.com/hekmon/transmissionrpc/v2 v2.0.1   // Transmission 客户端
    github.com/rs/zerolog v1.31.0                 // 日志库
)
```

### 10.2 依赖替换

[go.mod](file:///home/incast/PT-Forward/examples/ptdog/go.mod#L29-L31) 使用了依赖替换：

```go
replace github.com/autobrr/go-qbittorrent v1.5.0 => github.com/iaping/go-qbittorrent v0.0.0-20231106074650-9991b94e4419
replace github.com/gookit/cache v0.4.0 => github.com/iaping/cache v0.0.0-20231106113618-edded85d0f13
```

**原因**:
- 使用 fork 版本修复 bug
- 定制化功能
- 版本兼容性

### 10.3 依赖详解

#### 10.3.1 github.com/anacrolix/torrent

**用途**: 解析 .torrent 文件

**使用位置**: [scanner.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go#L9)

```go
import "github.com/anacrolix/torrent/metainfo"

meta, err := metainfo.LoadFromFile(path)
info, err := meta.UnmarshalInfo()
hash := meta.HashInfoBytes().HexString()
piecesHash := metainfo.HashBytes(info.Pieces).HexString()
```

#### 10.3.2 github.com/autobrr/go-qbittorrent

**用途**: qBittorrent API 客户端

**使用位置**: [qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go#L9)

```go
import "github.com/autobrr/go-qbittorrent"

c := qbittorrent.NewClient(qbittorrent.Config{
    Host:          host,
    Username:      conf.Username,
    Password:      conf.Password,
    TLSSkipVerify: !conf.Https,
})

data, err := q.c.GetTorrents(qbittorrent.TorrentFilterOptions{
    Hashes: hashes,
})
```

#### 10.3.3 github.com/hekmon/transmissionrpc/v2

**用途**: Transmission RPC 客户端

**使用位置**: [transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go#L9)

```go
import "github.com/hekmon/transmissionrpc/v2"

c, err := transmissionrpc.New(conf.Host, conf.Username, conf.Password, &transmissionrpc.AdvancedConfig{
    HTTPS:       conf.Https,
    Port:        conf.Port,
    HTTPTimeout: conf.Timeout,
})

data, err := t.c.TorrentGetAllForHashes(ctx, hashes)
```

#### 10.3.4 github.com/gookit/cache

**用途**: 缓存库

**使用位置**: [seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go#L11)

```go
import "github.com/gookit/cache"

cache.Register(cache.DvrFile, cache.NewFileCache(".cache"))
cache.DefaultUse(cache.DvrFile)

if cache.Has(url) {
    return
}

cache.Set(url, true, cache.Forever)
```

#### 10.3.5 github.com/rs/zerolog

**用途**: 结构化日志

**使用位置**: [main.go](file:///home/incast/PT-Forward/examples/ptdog/main.go#L13)

```go
import "github.com/rs/zerolog"
import "github.com/rs/zerolog/log"

log.Logger = log.Output(zerolog.ConsoleWriter{
    Out: os.Stdout,
})

log.Info().Msgf("PTDog %s", Version)
log.Err(err).Str("站点", website.String()).Msg("查询失败")
```

### 10.4 间接依赖

```go
require (
    github.com/anacrolix/missinggo v1.3.0
    github.com/anacrolix/missinggo/v2 v2.7.2
    github.com/avast/retry-go v3.0.0+incompatible
    github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8
    github.com/gookit/gsr v0.1.0
    github.com/hashicorp/go-cleanhttp v0.5.2
    github.com/hekmon/cunits/v2 v2.1.0
    github.com/huandu/xstrings v1.4.0
    github.com/mattn/go-colorable v0.1.13
    github.com/mattn/go-isatty v0.0.19
    github.com/pkg/errors v0.9.1
    golang.org/x/net v0.17.0
    golang.org/x/sys v0.13.0
)
```

---

## 十一、项目启动流程

### 11.1 启动序列

[main.go](file:///home/incast/PT-Forward/examples/ptdog/main.go#L20-L33) 定义了启动流程：

```go
func main() {
    defer func() {
        fmt.Println("Press 'Enter' to continue...")
        fmt.Scanln()
    }()
    
    // 1. 加载配置
    if err := config.Load("config.json"); err != nil {
        log.Err(err).Str("配置", "config.json").Msg("配置加载失败")
        return
    }
    
    // 2. 运行应用
    if err := app.New().Run(); err != nil {
        log.Err(err).Send()
    }
}
```

### 11.2 应用运行流程

[app.go](file:///home/incast/PT-Forward/examples/ptdog/app/app.go#L15-L24) 定义了应用运行流程：

```go
func (app *App) Run() (err error) {
    app.info()
    
    // 1. 启动辅种服务
    if err := reseed.New().Run(); err != nil {
        return err
    }
    
    // 2. 启动 HTTP 服务器
    return http.NewServer().Run()
}
```

### 11.3 辅种服务启动流程

[reseed.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/reseed.go#L11-L32) 定义了辅种服务启动流程：

```go
func (r *Reseed) Run() error {
    // 1. 创建扫描器
    scanners, err := r.scanners()
    if err != nil {
        return err
    }
    
    // 2. 启动扫描器
    if len(scanners) == 0 {
        log.Warn().Msg("未配置下载器")
    } else {
        for _, scanner := range scanners {
            scanner.Run()
        }
    }
    
    // 3. 启动查询器和播种器
    if websites := r.websites(); len(websites) == 0 {
        log.Warn().Msg("未配置辅种站点")
    } else {
        querier.Websites(websites).Run()
        seeder.Run()
    }
    
    return nil
}
```

### 11.4 完整启动流程图

```
main()
  ↓
config.Load("config.json")
  ↓
app.New().Run()
  ↓
reseed.New().Run()
  ↓
  ├─ 创建 Scanner (每个下载器一个)
  │   └─ scanner.Run() → 启动定时扫描 goroutine
  │
  ├─ querier.Websites(websites).Run()
  │   └─ 启动查询处理 goroutine
  │
  └─ seeder.Run()
      └─ 启动播种处理 goroutine
  ↓
http.NewServer().Run()
  └─ 启动 HTTP 服务器
  ↓
程序运行（阻塞）
```

---

## 十二、性能优化与资源管理

### 12.1 性能优化策略

#### 12.1.1 批量查询

[querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go#L69-L77) 实现批量查询：

```go
func (q *Querier) batch(batch *Batch, website *Website) {
    length := len(batch.pieces)
    
    for i := 0; i < length; i++ {
        s := i * website.limit
        e := s + website.limit
        if e >= length {
            q.query(batch, website, batch.pieces[s:])
            break
        }
        q.query(batch, website, batch.pieces[s:e])
    }
}
```

**优势**:
- 减少 HTTP 请求数量
- 降低网络开销
- 提高查询效率

#### 12.1.2 并发处理

- 多个下载器并发扫描
- 多个站点并发查询
- 多个种子并发辅种

#### 12.1.3 缓存去重

避免重复辅种，减少不必要的 API 调用和下载器操作。

### 12.2 资源管理

#### 12.2.1 HTTP 客户端复用

[website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go#L24-L28) 使用全局 HTTP 客户端：

```go
var httpc = &http.Client{
    Timeout: time.Minute,
}
```

**优势**:
- 复用 TCP 连接
- 减少 TLS 握手开销
- 提高网络效率

#### 12.2.2 Channel 缓冲控制

使用缓冲 channel 控制并发量，避免资源耗尽。

#### 12.2.3 Context 超时

使用 `context.WithTimeout` 防止请求挂起，及时释放资源。

#### 12.2.4 Defer 清理

使用 `defer` 确保资源正确释放：
- HTTP 响应体关闭
- 上下文取消
- WaitGroup 完成

---

## 十三、安全考虑

### 13.1 密码安全

**问题**: 配置文件中明文存储密码

```json
{
  "clients": [
    {
      "username": "admin",
      "password": "123456"
    }
  ],
  "websites": [
    {
      "passkey": "your passkey"
    }
  ]
}
```

**风险**:
- 配置文件泄露导致密码泄露
- 无法限制配置文件访问权限

**改进建议**:
1. 使用环境变量存储敏感信息
2. 实现配置加密
3. 支持密钥管理服务 (KMS)
4. 提供配置文件权限检查

### 13.2 网络安全

**当前实现**:
- 支持 HTTPS
- qBittorrent 可跳过 TLS 验证

```go
TLSSkipVerify: !conf.Https,  // 不安全
```

**风险**:
- 跳过 TLS 验证容易遭受中间人攻击

**改进建议**:
1. 强制使用 HTTPS
2. 验证 TLS 证书
3. 支持 CA 证书配置
4. 实现证书固定

### 13.3 API 安全

**当前实现**:
- 使用 passkey 认证
- 无速率限制
- 无请求签名

**风险**:
- passkey 泄露导致账号被盗
- 容易遭受暴力破解
- 无重放攻击防护

**改进建议**:
1. 实现请求签名
2. 添加速率限制
3. 使用 OAuth2 等标准认证
4. 实现 CSRF 防护

### 13.4 文件系统安全

**当前实现**:
- 扫描用户指定目录
- 读取 .torrent 文件
- 写入缓存文件

**风险**:
- 可能读取敏感文件
- 缓存文件权限问题

**改进建议**:
1. 限制文件访问范围
2. 验证文件类型
3. 设置缓存文件权限
4. 实现文件路径白名单

---

## 十四、可扩展性分析

### 14.1 下载器扩展

**当前支持**:
- Transmission
- qBittorrent

**扩展方法**:
1. 实现 `IClient` 接口
2. 添加新的 `Type` 常量
3. 在工厂方法中添加 case

```go
// 1. 定义新类型
const (
    TypeTransmission Type = iota
    TypeQbittorrent
    TypeDeluge  // 新增
)

// 2. 实现接口
type Deluge struct {
    conf Config
    c    *deluge.Client
}

func (d *Deluge) Type() Type {
    return TypeDeluge
}

func (d *Deluge) Torrents(hashes []string) ([]*Torrent, error) {
    // 实现
}

func (d *Deluge) TorrentAdd(filename, dir string) error {
    // 实现
}

// 3. 添加到工厂
func (r *Reseed) client(config config.Client) (client.IClient, error) {
    switch config.Type {
    case client.TypeTransmission:
        return client.NewTransmission(conf)
    case client.TypeQbittorrent:
        return client.NewQbittorrent(conf)
    case client.TypeDeluge:
        return client.NewDeluge(conf)  // 新增
    }
}
```

### 14.2 站点扩展

**当前支持**:
- 20+ 个开放 pieces_hash 查询的站点

**扩展方法**:
只需在配置文件中添加新站点配置：

```json
{
  "websites": [
    {
      "enable": true,
      "name": "新站点",
      "domain": "https://new-site.com",
      "api": "https://new-site.com/api/pieces-hash",
      "passkey": "your passkey",
      "download": "",
      "limit": 100
    }
  ]
}
```

**要求**:
- 站点必须开放 pieces_hash 查询 API
- API 必须接受 `passkey` 和 `pieces_hash` 参数
- API 必须返回 `pieces_hash -> 资源 ID` 的映射

### 14.3 缓存扩展

**当前实现**:
- 文件缓存

**扩展方法**:
支持其他缓存后端：

```go
// Redis 缓存
cache.Register(cache.DvrRedis, cache.NewRedisCache(redisConfig))
cache.DefaultUse(cache.DvrRedis)

// 内存缓存
cache.Register(cache.DvrMemory, cache.NewMemoryCache())
cache.DefaultUse(cache.DvrMemory)
```

---

## 十五、项目优缺点分析

### 15.1 优点

#### 15.1.1 架构设计
- ✅ 清晰的分层架构
- ✅ 良好的接口抽象
- ✅ 使用设计模式（工厂、策略、生产者-消费者）
- ✅ 模块化设计，易于扩展

#### 15.1.2 功能实现
- ✅ 支持多个下载器
- ✅ 支持多个站点
- ✅ 自动扫描和辅种
- ✅ pieces_hash 精确匹配
- ✅ 缓存去重

#### 15.1.3 性能优化
- ✅ 批量查询减少网络开销
- ✅ 并发处理提高效率
- ✅ 缓存减少重复操作
- ✅ HTTP 客户端复用

#### 15.1.4 代码质量
- ✅ 错误处理完善
- ✅ 日志记录详细
- ✅ 资源管理规范
- ✅ 代码结构清晰

### 15.2 缺点

#### 15.2.1 功能缺失
- ❌ 管理面板未实现
- ❌ 无热加载配置
- ❌ 无重试机制
- ❌ 无统计信息
- ❌ 无日志文件

#### 15.2.2 安全问题
- ❌ 密码明文存储
- ❌ 可跳过 TLS 验证
- ❌ 无速率限制
- ❌ 无请求签名

#### 15.2.3 可靠性问题
- ❌ 无健康检查
- ❌ 无故障恢复
- ❌ 无数据备份
- ❌ 无监控告警

#### 15.2.4 用户体验
- ❌ 需要手动配置
- ❌ 无配置验证
- ❌ 错误提示不友好
- ❌ 无进度显示

---

## 十六、改进建议

### 16.1 短期改进

#### 16.1.1 配置验证
```go
func (c *Config) Validate() error {
    if len(c.Clients) == 0 {
        return errors.New("至少需要配置一个下载器")
    }
    
    for _, client := range c.Clients {
        if client.Host == "" {
            return errors.New("下载器主机地址不能为空")
        }
        if client.Port == 0 {
            return errors.New("下载器端口不能为空")
        }
    }
    
    for _, website := range c.Websites {
        if website.Api == "" {
            return errors.New("站点 API 地址不能为空")
        }
        if website.Passkey == "" {
            return errors.New("站点 passkey 不能为空")
        }
    }
    
    return nil
}
```

#### 16.1.2 重试机制
```go
func (w *Website) QueryWithRetry(hashes []string, maxRetries int) (map[string]int, error) {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        result, err := w.Query(hashes)
        if err == nil {
            return result, nil
        }
        lastErr = err
        time.Sleep(time.Second * time.Duration(i+1))  // 指数退避
    }
    
    return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}
```

#### 16.1.3 日志文件
```go
func initLogger() {
    file, _ := os.OpenFile("ptdog.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    multi := zerolog.MultiLevelWriter(os.Stdout, file)
    log.Logger = zerolog.New(multi).With().Timestamp().Logger()
}
```

### 16.2 中期改进

#### 16.2.1 热加载配置
```go
func watchConfig(path string) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Err(err).Msg("创建文件监听器失败")
        return
    }
    
    defer watcher.Close()
    
    watcher.Add(path)
    
    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Op&fsnotify.Write == fsnotify.Write {
                log.Info().Msg("配置文件已修改，重新加载")
                if err := config.Load(path); err != nil {
                    log.Err(err).Msg("配置加载失败")
                }
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            log.Err(err).Msg("文件监听错误")
        }
    }
}
```

#### 16.2.2 统计信息
```go
type Statistics struct {
    TotalScanned   int64
    TotalMatched   int64
    TotalReseeded  int64
    TotalFailed    int64
    LastScanTime   time.Time
    LastReseedTime time.Time
}

var stats = &Statistics{}

func updateStats(scanned, matched, reseeded, failed int) {
    atomic.AddInt64(&stats.TotalScanned, int64(scanned))
    atomic.AddInt64(&stats.TotalMatched, int64(matched))
    atomic.AddInt64(&stats.TotalReseeded, int64(reseeded))
    atomic.AddInt64(&stats.TotalFailed, int64(failed))
}
```

#### 16.2.3 健康检查
```go
func (c *Transmission) HealthCheck() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    _, err := c.c.SessionGet(ctx)
    return err
}

func (q *Qbittorrent) HealthCheck() error {
    return q.c.Login()
}
```

### 16.3 长期改进

#### 16.3.1 完整管理面板
- Vue.js + Element Plus 前端
- RESTful API 后端
- 实时状态监控
- 配置可视化编辑
- 日志查看和搜索
- 统计图表展示

#### 16.3.2 分布式部署
- 支持多实例部署
- 分布式缓存 (Redis)
- 消息队列 (RabbitMQ/Kafka)
- 负载均衡
- 高可用架构

#### 16.3.3 智能推荐
- 基于历史数据推荐辅种站点
- 自动选择最优下载器
- 智能调度策略
- 预测性维护

---

## 十七、总结

PTDog 是一个功能完善、设计良好的 PT 站点自动辅种工具。项目采用了合理的架构设计和设计模式，具有良好的可扩展性。

### 17.1 核心优势
1. **架构清晰**: 分层架构，模块化设计
2. **接口抽象**: 统一的下载器接口，易于扩展
3. **性能优化**: 批量查询、并发处理、缓存去重
4. **容错设计**: 单点失败不影响整体流程

### 17.2 主要不足
1. **管理面板**: 未实现，需要手动配置
2. **安全加固**: 密码明文存储，TLS 可跳过
3. **可靠性**: 无重试机制，无健康检查
4. **监控**: 无统计信息，无日志文件

### 17.3 适用场景
- 个人 PT 玩家自动辅种
- 小规模 PT 站点资源管理
- 下载器资源同步

### 17.4 不适用场景
- 大规模生产环境（缺少监控和告警）
- 高安全要求环境（存在安全风险）
- 需要可视化管理的场景（管理面板未实现）

### 17.5 学习价值
PTDog 项目是一个很好的 Go 语言学习案例，涵盖了：
- 接口设计和抽象
- 并发编程 (goroutine, channel, WaitGroup)
- 错误处理和资源管理
- HTTP 客户端和服务器
- 第三方库集成
- 设计模式应用

通过分析这个项目，可以学到如何设计一个功能完整、结构清晰、易于扩展的 Go 应用程序。

---

## 附录

### A. 文件结构

```
ptdog/
├── main.go                          # 应用入口
├── config.json                      # 配置文件
├── go.mod                           # Go 模块定义
├── go.sum                           # 依赖锁定文件
├── README.md                        # 项目说明
├── screenshot.png                   # 截图
└── app/
    ├── app.go                       # 应用协调器
    ├── config/                      # 配置管理
    │   ├── config.go                # 配置加载
    │   ├── client.go                # 下载器配置
    │   ├── website.go               # 站点配置
    │   ├── system.go                # 系统配置
    │   └── http.go                  # HTTP 配置
    ├── client/                      # 下载器客户端
    │   ├── client.go                # 客户端接口
    │   ├── config.go                # 客户端配置
    │   ├── torrent.go               # 种子定义
    │   ├── transmission.go          # Transmission 实现
    │   └── qbittorrent.go           # qBittorrent 实现
    ├── reseed/                      # 辅种核心
    │   ├── reseed.go                # 辅种服务
    │   ├── scanner.go               # 种子扫描器
    │   ├── querier.go               # 站点查询器
    │   ├── seeder.go                # 种子播种器
    │   └── website.go               # 站点 API
    └── http/                        # HTTP 服务器
        ├── server.go                # 服务器实现
        └── view/
            └── index.html           # 前端页面
```

### B. 配置示例

```json
{
  "http": {
    "addr": "127.0.0.1:1689"
  },
  "system": {
    "sleep": 15
  },
  "clients": [
    {
      "enable": true,
      "https": false,
      "type": 0,
      "host": "127.0.0.1",
      "username": "admin",
      "password": "123456",
      "port": 9091,
      "timeout": 60,
      "dir": "/Users/aping/Downloads",
      "skip_checking": false
    },
    {
      "enable": true,
      "https": false,
      "type": 1,
      "host": "127.0.0.1",
      "username": "admin",
      "password": "123456",
      "port": 8080,
      "timeout": 60,
      "dir": "/Users/aping/Downloads",
      "skip_checking": false
    }
  ],
  "websites": [
    {
      "enable": true,
      "name": "ptcafe",
      "domain": "https://ptcafe.club",
      "api": "https://ptcafe.club/api/pieces-hash",
      "passkey": "your passkey",
      "download": "",
      "limit": 0
    },
    {
      "enable": true,
      "name": "hdtime",
      "domain": "https://hdtime.org",
      "api": "https://hdtime.org/api/pieces-hash",
      "passkey": "your passkey",
      "download": "",
      "limit": 0
    }
  ]
}
```

### C. API 接口规范

#### C.1 pieces_hash 查询接口

**请求**:
```http
POST /api/pieces-hash
Content-Type: application/json

{
  "passkey": "your_passkey",
  "pieces_hash": ["hash1", "hash2", "hash3"]
}
```

**响应**:
```json
{
  "data": {
    "hash1": 12345,
    "hash2": 67890
  }
}
```

**说明**:
- `passkey`: 用户密钥
- `pieces_hash`: pieces_hash 数组
- `data`: 匹配结果，key 为 pieces_hash，value 为资源 ID

#### C.2 种子下载接口

**格式**:
```
{domain}/download.php?id={id}&passkey={passkey}
```

**示例**:
```
https://ptcafe.club/download.php?id=12345&passkey=abcdef123456
```

**自定义格式**:
```
{download}?id={id}&passkey={passkey}
```

### D. 种子状态映射

| 状态值 | 状态名称 | 说明 |
|--------|---------|------|
| 0 | Stopped | 已停止 |
| 1 | CheckWait | 等待校验 |
| 2 | Check | 校验中 |
| 3 | DownloadWait | 等待下载 |
| 4 | Download | 下载中 |
| 5 | SeedWait | 等待做种 |
| 6 | Seed | 做种中 |
| 7 | Isolated | 无连接 |

### E. 相关资源

- **GitHub**: https://github.com/iaping/ptdog
- **Releases**: https://github.com/iaping/ptdog/releases
- **作者**: iaping

---

**分析完成时间**: 2026-04-11  
**分析工具**: Trae IDE  
**分析深度**: 全面深度分析