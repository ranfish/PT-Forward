# PTNexus 项目深度分析

## 项目概述

PTNexus 是一个基于 Go + Vue 3 构建的 PT 种子聚合管理平台，提供种子管理、流量统计、智能转种、IYUU API 集成等核心功能。项目采用前后端分离架构，支持桌面端和 Web 端两种部署方式。

## 技术栈

### 后端 (Server)
- **语言**: Go 1.23
- **Web框架**: Gin
- **ORM**: GORM
- **数据库支持**: SQLite (默认) / MySQL / PostgreSQL
- **桌面框架**: Wails v2.11.0 + systray (系统托盘)

### 前端 (WebUI)
- **框架**: Vue 3.5.18 + TypeScript
- **UI组件库**: Element Plus 2.10.7
- **状态管理**: Pinia 3.0.3
- **图表**: ECharts 6.0.0
- **构建工具**: Vite

## 项目结构

```
PTNexus/
├── server/                    # 后端服务
│   ├── internal/
│   │   ├── bootstrap/         # 应用初始化
│   │   ├── config/            # 配置管理
│   │   ├── http/handler/      # HTTP 处理器
│   │   ├── platform/          # 平台层 (日志、网络代理)
│   │   ├── repository/        # 数据访问层
│   │   └── service/           # 业务逻辑层
│   └── go.mod
├── desktop/                   # 桌面应用
│   └── go.mod                 # Wails 桌面框架
├── webui/                     # 前端界面
│   ├── src/
│   │   ├── views/             # 页面组件
│   │   ├── components/        # 通用组件
│   │   ├── stores/            # Pinia 状态
│   │   ├── router/            # 路由配置
│   │   └── types/             # TypeScript 类型
│   └── package.json
├── wiki/                      # 项目文档
└── Dockerfile                 # 多阶段构建
```

## 核心模块分析

### 1. 配置管理 ([config/manager.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/config/manager.go))

```go
type Manager struct {
    mu     sync.RWMutex
    path   string
    config map[string]any
}
```

**特性**:
- JSON 配置文件管理
- 默认配置合并机制
- 首次启动自动生成临时密码
- CookieCloud 敏感信息保护

**默认配置结构**:
```go
map[string]any{
    "downloaders":            []any{},
    "realtime_speed_enabled": true,
    "downloader_queue": map[string]any{
        "enabled":                         true,
        "max_queue_size":                  1000,
        "max_retries":                     3,
        "max_workers":                     1,
    },
    "auth": map[string]any{
        "username":             "admin",
        "must_change_password": true,
    },
}
```

### 2. 数据库层 ([repository/db.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/repository/db.go))

**多数据库支持**:
```go
switch dbType {
case "mysql":
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", ...)
    db, err = gorm.Open(mysql.Open(dsn), ...)
case "postgresql":
    dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", ...)
    db, err = gorm.Open(postgres.Open(dsn), ...)
case "sqlite":
    db, err = gorm.Open(sqlite.Open(dbPath), ...)
}
```

**连接池配置**:
- MaxOpenConns: 20
- MaxIdleConns: 10

### 3. 发布引擎 ([service/publish/publisher/engine/engine.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/service/publish/publisher/engine/engine.go))

**站点路由分发**:
```go
func Publish(input publisher.PublishInput) (publisher.PublishResult, error) {
    siteCode := strings.ToLower(strings.TrimSpace(input.SiteCode))
    switch siteCode {
    case "cbg":      return publishsites.PublishCBG(input)
    case "baozi":    return publishsites.PublishBaozi(input)
    case "hddolby":  return publishsites.PublishHDDolby(input)
    case "zhuque":   return publishsites.PublishZhuque(input)
    case "haidan":   return publishsites.PublishHaidan(input)
    // ... 更多站点
    default:         return publisher.PublishPublic(input)
    }
}
```

**支持的站点**:
- CBG、包子、HDDolby、HDKYL、LuckPT
- PTerClub、PTSKit、朱雀、海胆、肉丝
- PTLGS、HDFans、CrabPT
- 其他 NexusPHP 站点 (通用发布器)

### 4. 发布工作流 ([service/publish/workflow/publish_entry.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/service/publish/workflow/publish_entry.go))

**发布流程**:
1. **标签限制检测**: 检测禁转/限转/分集标签
2. **下载器门控检查**: 检查下载器限制
3. **种子上传**: 向目标站点发起上传请求
4. **URL 标准化**: 处理发布链接和直链下载
5. **自动编辑**: 已存在种子时自动更新

```go
func ExecutePublish(input PublishExecutionInput, deps PublishExecutionDeps) (map[string]any, int) {
    if restricted := publishuploader.DetectRestrictedTags(uploadData); len(restricted) > 0 {
        return map[string]any{
            "success":       false,
            "logs":          fmt.Sprintf("发布前标签限制: 检测到禁转/限转/分集标签 %v", restricted),
            "limit_reached": true,
        }, 200
    }
    // ... 发布逻辑
}
```

### 5. 下载器客户端 ([service/downloaderclient/client.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/service/downloaderclient/client.go))

**支持的下载器**:
- qBittorrent
- Transmission

**流量统计结构**:
```go
type TrafficStats struct {
    DownloadSpeed int64
    UploadSpeed   int64
    TotalDownload int64
    TotalUpload   int64
    Version       string
}
```

**添加种子选项**:
```go
type AddTorrentOptions struct {
    Paused          bool
    Tags            []string
    Category        string
    UploadLimitMBps int
}
```

### 6. Cross-Seed 服务 ([service/crossseed/query.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/service/crossseed/query.go))

**审核状态过滤**:
- `reviewed`: 已审核
- `unreviewed`: 未审核
- `error`: 错误状态 (禁转/限转/分集标签)

**种子状态判断**:
```go
var inactiveTorrentStates = []string{"未做种", "已暂停", "已停止", "错误", "等待", "队列"}
```

### 7. BDInfo 流程 ([service/processing/bdflow/bdinfo_flow.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/service/processing/bdflow/bdinfo_flow.go))

**BDInfo 任务执行流程**:
1. 路径回填 (通过 hash 查找当前种子)
2. 媒体信息提取
3. 写入数据库
4. 回写标题与标签

```go
type RunBDInfoTaskInput struct {
    TaskID           string
    Hash             string
    TorrentID        string
    SiteName         string
    TorrentName      string
    SeedSavePath     string
    SeedDownloaderID string
}
```

### 8. IYUU 集成 ([service/torrentdata/iyuu.go](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/service/torrentdata/iyuu.go))

**IYUU API 功能**:
- 跨站种子匹配
- 辅种查询
- 缓存机制

## 前端架构

### 路由配置 ([router/index.ts](file:///home/incast/PT-Forward/examples/PTNexus/webui/src/router/index.ts))

```typescript
routes: [
    { path: '/',           name: 'home',         component: HomeView },
    { path: '/info',       name: 'info',         component: InfoView },
    { path: '/torrents',   name: 'torrents',     component: TorrentsView },
    { path: '/data',       name: 'data',         component: CrossSeedDataView },
    { path: '/publish-logs', name: 'publish-logs', component: PublishLogsView },
    { path: '/sites',      name: 'sites',        component: SitesView },
    { path: '/settings',   name: 'settings',     component: SettingsView, redirect: '/settings/general',
        children: [
            { path: 'general',    name: 'settings-general',    component: GeneralSettings },
            { path: 'downloader', name: 'settings-downloader', component: DownloaderSettings },
            { path: 'cookie',     name: 'settings-cookie',     component: SitesSettings },
        ]
    },
    { path: '/login',      name: 'login',        component: LoginView },
]
```

### 状态管理 ([stores/crossSeed.ts](file:///home/incast/PT-Forward/examples/PTNexus/webui/src/stores/crossSeed.ts))

```typescript
export const useCrossSeedStore = defineStore('crossSeed', {
  state: () => ({
    taskId: null as string | null,
    sourceInfo: null as ISourceInfo | null,
    workingParams: null as object | null,
  }),
  actions: {
    setTaskId(id: string) { this.taskId = id },
    setSourceInfo(info: ISourceInfo) { this.sourceInfo = info },
    setParams(params: object) { this.workingParams = params },
    reset() { /* 清除所有状态 */ },
  },
})
```

## HTTP Handler 层

### Handler 结构 ([http/handler/](file:///home/incast/PT-Forward/examples/PTNexus/server/internal/http/handler))

| Handler | 功能 |
|---------|------|
| `auth_handler.go` | 认证登录 |
| `cross_seed_handler.go` | Cross-Seed 数据操作 |
| `sites_handler.go` | 站点管理 |
| `torrents_handler.go` | 种子数据 |
| `stats_handler.go` | 流量统计 |
| `settings_handler.go` | 设置管理 |
| `logs_handler.go` | 日志导出 |
| `migrate/` | 迁移相关处理器 |

### Cross-Seed Handler 示例

```go
type CrossSeedHandler struct {
    service *service.CrossSeedService
}

func (h *CrossSeedHandler) Data(c *gin.Context) {
    params := service.CrossSeedQueryParams{
        Page:               intQuery(c, "page", 1),
        PageSize:           intQuery(c, "page_size", 20),
        Search:             c.Query("search"),
        PathFilters:        stringSliceQuery(c, "path_filters"),
        ReviewStatus:       c.Query("review_status"),
    }
    result, err := h.service.QueryData(params)
    c.JSON(http.StatusOK, result)
}
```

## 部署架构

### Dockerfile 多阶段构建

```dockerfile
# Stage 1: webui-builder (Node.js 构建)
# Stage 2: updater-builder (更新器构建)
# Stage 3: server-builder (Go 服务构建)
# Stage 4: final (运行时镜像)
```

### 桌面应用

使用 Wails v2.11.0 框架:
- 跨平台支持 (Windows/Linux/macOS)
- 系统托盘集成 (systray)
- WebView2 渲染 (Windows)

## 核心功能总结

| 功能模块 | 描述 |
|---------|------|
| **流量统计** | 实时速度、累计流量、图表展示 |
| **种子聚合** | 多站点种子统一管理 |
| **智能转种** | 自动检测禁转/限转/分集标签限制 |
| **IYUU 集成** | 跨站种子匹配与辅种 |
| **发布工作流** | 批量发布、预览、结果追踪 |
| **BDInfo 提取** | 蓝光原盘媒体信息提取 |
| **下载器集成** | qBittorrent/Transmission 支持 |
| **多数据库** | SQLite/MySQL/PostgreSQL |

## 设计亮点

1. **模块化架构**: Service 层清晰分离，依赖注入设计
2. **多站点适配**: 发布引擎支持站点特定逻辑
3. **数据库抽象**: 支持 SQLite/MySQL/PostgreSQL 无缝切换
4. **桌面/Web 双模式**: Wails 框架实现跨平台桌面应用
5. **实时进度**: SSE (Server-Sent Events) 实现任务进度推送
6. **安全设计**: 首次启动生成临时密码，敏感信息保护

## 与其他项目对比

| 特性 | PTNexus | IYUUPlus | HDApt Auto Transfer |
|------|---------|----------|---------------------|
| 语言 | Go | PHP | Python |
| 框架 | Gin + Vue 3 | Webman | Flask |
| 数据库 | 多数据库支持 | SQLite | SQLite |
| 桌面支持 | Wails | 无 | 无 |
| 发布站点 | 13+ 站点适配 | IYUU API | NexusPHP 通用 |
| BDInfo | 支持 | 不支持 | 支持 |
