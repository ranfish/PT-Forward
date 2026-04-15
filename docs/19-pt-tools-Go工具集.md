# pt-tools 项目深度研究报告

## 项目概述

pt-tools 是一个企业级的 PT（Private Tracker）站点自动化管理平台，基于 Go 1.26.2 和 Vue 3 构建，提供 RSS 自动订阅、多站点搜索、用户信息统计、下载器管理、过滤规则引擎等全方位功能，是 PT 用户的高效管理工具。

### 核心价值
- **全自动化**: RSS 订阅自动下载、免费种子监控、自动删种
- **多站点支持**: 支持 60+ PT 站点，6 种架构驱动
- **智能过滤**: 关键词/通配符/正则表达式过滤规则
- **统一管理**: 多下载器实例管理、用户信息聚合展示
- **Web 可视化**: 现代化的管理界面，支持浏览器扩展

## 技术架构

### 后端技术栈
- **Go 1.26.2**: 高性能、并发友好的语言选择
- **GORM + SQLite**: ORM 框架，支持 WAL 模式优化并发
- **Cobra**: CLI 命令框架，支持子命令和参数
- **Zap**: 结构化日志，支持多级别和日志轮转
- **goquery**: HTML 解析，类似 jQuery 的选择器语法
- **requests**: HTTP 客户端，支持代理和重试
- **bencode**: 种子文件编码/解码
- **gofeed**: RSS/Atom 订阅解析

### 前端技术栈
- **Vue 3.5**: 组合式 API，响应式系统
- **TypeScript**: 类型安全，提高代码质量
- **Element Plus**: UI 组件库，丰富的组件和主题
- **Pinia**: 状态管理，轻量级且强大
- **Vue Router 5**: 路由管理，支持懒加载
- **Vite 8**: 构建工具，快速开发和构建
- **oxfmt/oxlint**: 代码格式化和静态检查

### 项目结构
```
pt-tools/
├── cmd/                    # CLI 命令入口
│   ├── rss.go             # RSS 订阅处理
│   ├── web.go             # Web 服务器
│   └── task.go            # 任务管理
├── site/v2/               # 站点驱动架构（核心）
│   ├── definitions/       # 60+ 站点定义文件
│   ├── nexusphp_driver.go # NexusPHP 驱动
│   ├── mtorrent_driver.go # mTorrent 驱动
│   └── factory.go         # 站点工厂
├── scheduler/             # 任务调度系统
│   ├── manager.go         # 调度管理器
│   ├── free_end_monitor.go # 免费结束监控
│   └── cleanup_monitor.go # 自动删种监控
├── web/                   # Web 服务器和 API
│   ├── server.go          # HTTP 服务器
│   ├── api_*.go           # 各功能 API
│   └── frontend/          # Vue 3 前端
├── internal/              # 内部业务逻辑
│   ├── common.go          # 通用功能
│   ├── push.go            # 种子推送逻辑
│   └── filter/            # 过滤规则处理
├── models/                # 数据模型
│   ├── init.go            # 数据库初始化
│   ├── config_models.go   # 配置模型
│   └── filter_rule.go     # 过滤规则模型
├── core/                  # 核心配置管理
│   ├── config_store.go    # 配置存储
│   └── migration/         # 数据库迁移
├── thirdpart/             # 第三方集成
│   └── downloader/        # 下载器客户端
├── utils/                 # 工具函数
├── tools/                 # 浏览器扩展
└── config/                # 配置文件
```

## 核心功能模块深度分析

### 1. 站点驱动架构

**设计模式**: 驱动模式 + 工厂模式 + 注册表模式

**核心组件**:

#### SiteDefinition（站点定义）
每个站点都有独立的定义文件，包含：
- 站点基本信息（ID、名称、URL）
- 架构类型（NexusPHP、mTorrent、Unit3D 等）
- 认证方式（Cookie、API Key、Passkey）
- 选择器配置（用于解析 HTML）
- 用户信息配置
- 等级要求配置
- 限流配置

**示例**（[BTSchool](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/definitions/btschool.go#L1-L162)）:
```go
var BTSchoolDefinition = &v2.SiteDefinition{
    ID:              "btschool",
    Name:            "BTSCHOOL",
    Schema:          v2.SchemaNexusPHP,
    AuthMethod:      v2.AuthMethodCookie,
    URLs:            []string{"https://pt.btschool.club/"},
    RateLimit:       0.5,
    RateBurst:       2,
    HREnabled:       true,
    HRSeedTimeHours: 20,
    UserInfo: &v2.UserInfoConfig{
        Process: []v2.UserInfoProcess{
            {
                RequestConfig: v2.RequestConfig{URL: "/index.php", ResponseType: "document"},
                Fields:        []string{"id", "name", "seeding", "leeching", "bonus"},
            },
        },
        Selectors: map[string]v2.FieldSelector{
            "uploaded": {
                Selector: []string{"td.rowhead:contains('传输') + td"},
                Filters: []v2.Filter{
                    {Name: "regex", Args: []any{`<strong>上传量</strong>[：:\s]*([\d.,]+\s*[KMGTP]?i?B)`}},
                    {Name: "parseSize"},
                },
            },
        },
    },
}
```

#### Driver 实现
支持 6 种站点架构：

1. **NexusPHPDriver**: 最常见的 PT 站点架构
   - HTML 解析和选择器匹配
   - 用户信息抓取
   - 种子列表和详情解析
   - 优惠状态识别

2. **MTorrentDriver**: M-Team 自定义 API
   - RESTful API 调用
   - JSON 数据解析
   - 多 API URL 轮换

3. **Unit3DDriver**: Unit3D 架构站点
   - 现代 PT 站点架构
   - GraphQL 支持

4. **GazelleDriver**: What.CD 风格站点
   - 复杂的评分系统
   - 音乐站点专用

5. **HDDolbyDriver**: HDDolby REST API
   - Cookie + API Key 双认证
   - 时魔值计算

6. **RousiDriver**: RousiPro 架构
   - Passkey 认证
   - 自定义驱动实现

#### 过滤器系统
强大的数据转换管道，支持链式调用：

**内置过滤器**（[filters.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/filters.go#L1-L200)）:
- `parseNumber`: 提取数字
- `parseSize`: 解析大小（GB/MB/KB）
- `parseTime`: 解析时间
- `querystring`: 提取 URL 参数
- `split`: 分割字符串
- `replace`: 替换子串
- `regex`: 正则匹配
- `default`: 默认值
- `multiply/divide`: 数学运算

**使用示例**:
```go
Selectors: map[string]v2.FieldSelector{
    "uploaded": {
        Selector: []string{"td.rowhead:contains('传输') + td"},
        Filters: []v2.Filter{
            {Name: "regex", Args: []any{`<strong>上传量</strong>[：:\s]*([\d.,]+\s*[KMGTP]?i?B)`}},
            {Name: "parseSize"}, // 将 "123.45 GB" 转换为字节数
        },
    },
}
```

### 2. 过滤规则引擎

**设计理念**: 灵活、强大、易用

**数据模型**（[filter_rule.go](file:///home/incast/PT-Forward/examples/pt-tools/models/filter_rule.go#L1-L133)）:
```go
type FilterRule struct {
    ID          uint        `gorm:"primaryKey" json:"id"`
    Name        string      `gorm:"size:128;not null" json:"name"`
    Pattern     string      `gorm:"size:512;not null" json:"pattern"`
    PatternType PatternType `gorm:"size:16;not null;default:'keyword'" json:"pattern_type"`
    MatchField  MatchField  `gorm:"size:16;not null;default:'both'" json:"match_field"`
    RequireFree bool        `gorm:"default:true" json:"require_free"`
    Enabled     bool        `gorm:"default:true" json:"enabled"`
    SiteID      *uint       `gorm:"index" json:"site_id"`
    RSSID       *uint       `gorm:"index" json:"rss_id"`
    Priority    int         `gorm:"default:100" json:"priority"`
}
```

**三种匹配模式**:
1. **关键词匹配**（PatternKeyword）: 简单包含匹配
2. **通配符匹配**（PatternWildcard）: 支持 `*` 和 `?`
3. **正则表达式**（PatternRegex）: 强大的模式匹配

**多字段支持**:
- 仅标题匹配（MatchFieldTitle）
- 仅标签匹配（MatchFieldTag）
- 标题和标签同时匹配（MatchFieldBoth）

**多对多关联**（[rss_filter_association.go](file:///home/incast/PT-Forward/examples/pt-tools/models/rss_filter_association.go#L1-L129)）:
```go
type RSSFilterAssociation struct {
    ID           uint      `gorm:"primaryKey" json:"id"`
    RSSID        uint      `gorm:"uniqueIndex:idx_rss_filter;not null" json:"rss_id"`
    FilterRuleID uint      `gorm:"uniqueIndex:idx_rss_filter;not null" json:"filter_rule_id"`
    CreatedAt    time.Time `json:"created_at"`
}
```

**优先级系统**: 规则按优先级排序，优先级高的先匹配

**测试覆盖**（[filter_rule_test.go](file:///home/incast/PT-Forward/examples/pt-tools/models/filter_rule_test.go#L1-L266)）:
- CRUD 操作测试
- 优先级排序测试
- 站点级和全局规则测试
- RSS 关联测试

### 3. 数据库设计

**技术选型**: SQLite + WAL 模式

**优化配置**（[init.go](file:///home/incast/PT-Forward/examples/pt-tools/models/init.go#L1-L285)）:
```go
dsn := fmt.Sprintf("file:%s?_busy_timeout=30000&_txlock=immediate&cache=shared", dbFile)
// 启用 WAL 模式
db.Exec("PRAGMA journal_mode=WAL;")
// 设置同步模式为 NORMAL，平衡性能和持久性
db.Exec("PRAGMA synchronous=NORMAL;")
// 限制连接数为 1（SQLite 文件级并发）
sqlDB.SetMaxOpenConns(1)
sqlDB.SetMaxIdleConns(1)
```

**核心数据表**:

1. **TorrentInfo**: 种子信息表
   - 支持免费状态追踪
   - 下载进度监控
   - 免费结束管理
   - H&R 保护

2. **TorrentInfoArchive**: 种子归档表
   - 超过保留期的记录
   - 14 天默认保留期

3. **SiteSetting**: 站点配置表
   - 支持内置站点和自定义站点
   - 多种认证方式
   - 站点模板支持

4. **RSSSubscription**: RSS 订阅表
   - 支持示例配置标记
   - 独立的下载器和目录配置
   - 免费结束自动暂停开关

5. **FilterRule**: 过滤规则表
   - 优先级排序
   - 站点级和全局规则
   - 多种匹配模式

6. **DownloaderSetting**: 下载器配置表
   - 支持 qBittorrent 和 Transmission
   - 多实例管理
   - 目录配置

7. **UserInfo**: 用户信息表
   - 聚合展示多站点数据
   - 等级进度追踪
   - 时效性管理

**数据库迁移**:
- SchemaVersion 表追踪版本
- 自动迁移机制
- 向后兼容性保证
- CMCT 到 SpringSunday 迁移（[presets.go](file:///home/incast/PT-Forward/examples/pt-tools/models/presets.go#L1-L215)）

**事务支持**:
```go
func (t *TorrentDB) WithTransaction(fn func(tx *gorm.DB) error) error {
    return t.DB.Transaction(fn)
}
```

### 4. 免费结束监控系统

**设计目标**: 精确控制免费种子的下载时机

**核心机制**（[free_end_monitor.go](file:///home/incast/PT-Forward/examples/pt-tools/scheduler/free_end_monitor.go#L1-L150)）:

1. **独立定时器**: 为每个种子创建独立的定时器
2. **周期检查**: 每 5 分钟检查一次所有监控任务
3. **应用重启恢复**: 从数据库加载待监控任务
4. **进度更新**: 每 1 分钟更新下载进度
5. **批量处理**: 批量更新进度，减少数据库压力

**数据结构**:
```go
type FreeEndMonitor struct {
    mu            sync.Mutex
    ctx           context.Context
    cancel        context.CancelFunc
    db            *gorm.DB
    downloaderMgr *downloader.DownloaderManager
    checkInterval time.Duration
    pendingTasks  map[uint]*monitorTask
    wg            sync.WaitGroup
    running       bool
}

type monitorTask struct {
    torrentID uint
    timer     *time.Timer
    cancel    context.CancelFunc
}
```

**工作流程**:
1. RSS 下载免费种子时，记录免费结束时间
2. 创建独立定时器，在免费结束时刻触发
3. 定期周期检查，确保不会遗漏
4. 免费期结束时，检查下载进度
5. 未完成的任务自动暂停或删除
6. 已完成的任务继续做种

**配置选项**:
- 免费结束自动暂停（默认）
- 免费结束自动删除（需手动开启）
- 手动恢复暂停任务
- 批量删除暂停任务
- 历史记录归档（14 天）

### 5. 多站点搜索系统

**架构设计**: 编排器模式 + 缓存优化

**核心组件**:

1. **SearchOrchestrator**: 搜索编排器
   - 并发搜索多个站点
   - 结果去重和排序
   - 搜索缓存管理

2. **SearchCache**: 搜索缓存
   - 减少 API 调用
   - 提高响应速度
   - 可配置的缓存时间

3. **Ranker**: 结果排序
   - 按发布时间排序
   - 按做种人数排序
   - 按完成数排序

**工作流程**:
1. 用户输入搜索关键词
2. 编排器并发调用所有启用的站点
3. 每个站点独立执行搜索
4. 结果聚合和去重
5. 按规则排序
6. 返回搜索结果

**批量操作**:
- 批量下载种子文件
- 批量推送到下载器
- 批量添加过滤规则

### 6. 用户信息聚合系统

**设计目标**: 一站式查看所有站点数据

**核心功能**（[api_userinfo.go](file:///home/incast/PT-Forward/examples/pt-tools/web/api_userinfo.go#L1-L150)）:

1. **数据聚合**: 聚合所有站点的上传量、下载量、分享率
2. **等级追踪**: 显示等级进度和升级条件
3. **时魔计算**: 自动计算魔力值增长
4. **数据截图**: 生成精美的数据卡片截图
5. **实时更新**: 支持手动和自动同步

**API 设计**:
```go
// 聚合统计
GET /api/v2/userinfo/aggregated

// 站点列表
GET /api/v2/userinfo/sites

// 单站详情
GET /api/v2/userinfo/sites/{site}

// 手动同步
POST /api/v2/userinfo/sync

// 清除缓存
POST /api/v2/userinfo/cache/clear
```

**响应结构**:
```go
type AggregatedStatsResponse struct {
    TotalUploaded   int64              `json:"totalUploaded"`
    TotalDownloaded int64              `json:"totalDownloaded"`
    AverageRatio    float64            `json:"averageRatio"`
    TotalSeeding    int                `json:"totalSeeding"`
    TotalLeeching   int                `json:"totalLeeching"`
    TotalBonus      float64            `json:"totalBonus"`
    SiteCount       int                `json:"siteCount"`
    PerSiteStats    []UserInfoResponse `json:"perSiteStats"`
}
```

### 7. 下载器管理系统

**支持下载器**:
- qBittorrent（主流选择）
- Transmission（Linux 服务器常用）

**核心特性**:

1. **多实例管理**: 同时管理多个下载器实例
2. **连接池复用**: 避免频繁创建连接
3. **批量操作**: 批量添加/删除/控制种子
4. **目录配置**: 每个下载器独立的目录配置
5. **能力检测**: 自动检测下载器支持的特性

**API 设计**:
```go
// 下载器列表
GET /api/downloaders

// 下载器详情
GET /api/downloaders/{id}

// 测试连接
POST /api/downloaders/test

// 下载器目录
GET /api/downloaders/all-directories
```

**下载器 Hub**: 统一的下载器 Web UI
- 查看所有下载器的种子
- 批量操作种子
- 传输统计
- 种子元数据查询

### 8. 任务调度系统

**设计模式**: 管理器模式 + 事件驱动

**核心组件**（[manager.go](file:///home/incast/PT-Forward/examples/pt-tools/scheduler/manager.go#L1-L150)）:

1. **Manager**: 任务管理器
   - 管理 RSS 订阅任务
   - 配置热重载
   - 下载器连接池管理

2. **FreeEndMonitor**: 免费结束监控器
   - 精确的定时触发
   - 进度跟踪
   - 自动暂停/删除

3. **CleanupMonitor**: 自动删种监控器
   - 按条件清理种子
   - H&R 保护
   - 磁盘保护

**事件驱动**:
```go
events.Publish(events.Event{
    Type: events.ConfigChanged,
    Version: newVersion,
    Source: "web_api",
    At: time.Now(),
})
```

**配置热重载**:
- 配置变更自动触发
- 防抖机制（200ms）
- 版本号控制
- 无需重启服务

## 前端架构

### 技术栈
- **Vue 3.5**: Composition API
- **TypeScript**: 类型安全
- **Element Plus**: UI 组件库
- **Pinia**: 状态管理
- **Vue Router 5**: 路由
- **Vite 8**: 构建工具

### 路由设计
```typescript
{
  path: "/userinfo",
  name: "userinfo",
  component: () => import("@/views/UserInfoDashboard.vue"),
  meta: { title: "用户统计" },
},
{
  path: "/search",
  name: "search",
  component: () => import("@/views/TorrentSearch.vue"),
  meta: { title: "种子搜索" },
},
{
  path: "/filter-rules",
  name: "filter-rules",
  component: () => import("@/views/FilterRules.vue"),
}
```

### 状态管理
- **LogLevel**: 日志级别管理
- **Theme**: 主题切换（默认/海洋/石墨/极光/翡翠）
- **Version**: 版本检查和更新

### 核心组件
- **VersionChecker**: 版本检查和更新提示
- **UserInfoDashboard**: 用户信息聚合展示
- **TorrentSearch**: 多站点搜索界面
- **FilterRules**: 过滤规则管理
- **DownloaderSettings**: 下载器配置
- **SiteDetail**: 站点详情和配置

### API 封装
```typescript
// 统一的 API 调用
export const api = {
  global: {
    get: () => http.get('/api/global'),
    update: (data: GlobalSettings) => http.put('/api/global', data),
  },
  sites: {
    list: () => http.get('/api/sites'),
    update: (name: string, data: SiteUpdate) => http.put(`/api/sites/${name}`, data),
  },
  // ...
}
```

## 测试策略

### 测试覆盖
- **测试文件数**: 134 个
- **测试类型**: 单元测试、集成测试、端到端测试
- **测试框架**: testify + vitest（前端）

### 测试分类

1. **单元测试**:
   - 模型测试（filter_rule_test.go、config_models_test.go）
   - 过滤器测试（filters_test.go）
   - 工具函数测试

2. **集成测试**:
   - API 测试（api_*_test.go）
   - 数据库测试
   - 站点驱动测试

3. **端到端测试**:
   - RSS 订阅流程测试
   - 下载器集成测试
   - 搜索功能测试

### 测试工具
- **Go**: testify、golang/mock
- **前端**: vitest、vue-test-utils
- **覆盖率**: go tool cover

### CI/CD
- **GitHub Actions**: 自动化测试和构建
- **Docker**: 容器化测试环境
- **Act**: 本地 CI 测试

## 部署和运维

### Docker 部署

**多阶段构建**（[Dockerfile](file:///home/incast/PT-Forward/examples/pt-tools/Dockerfile#L1-L128)）:

1. **前端构建阶段**:
```dockerfile
FROM node:25.2.0-alpine AS frontend-builder
WORKDIR /app/web/frontend
RUN npm install -g pnpm@10.25.0
COPY web/frontend/package.json web/frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/frontend/ ./
RUN pnpm build
```

2. **后端构建阶段**:
```dockerfile
FROM golang:1.26.2 AS builder
WORKDIR /app
COPY . .
COPY --from=frontend-builder /app/web/static/dist /app/web/static/dist
RUN go build -ldflags="-s -w" -mod=vendor -o pt-tools
RUN upx -9 /app/pt-tools || true
```

3. **运行阶段**:
```dockerfile
FROM alpine:3.20.3
COPY --from=builder /app/pt-tools /app/bin/pt-tools
COPY docker/docker-entrypoint.sh /app/bin/
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["pt-tools"]
```

### Makefile 自动化

**构建命令**（[Makefile](file:///home/incast/PT-Forward/examples/pt-tools/Makefile#L1-L328)）:

```makefile
# 本地构建
make build-local

# 多平台构建
make build-binaries

# Docker 构建
make build-local-docker
make build-remote-docker

# 测试
make unit-test
make coverage-summary

# 代码检查
make lint
make fmt
```

### 运行配置

**环境变量**:
- `PT_HOST`: 监听地址（默认 0.0.0.0）
- `PT_PORT`: 监听端口（默认 8080）
- `PT_ADMIN_USER`: 管理员用户名（默认 admin）
- `PT_ADMIN_PASS`: 管理员密码（默认 adminadmin）
- `TZ`: 时区（默认 Asia/Shanghai）

**代理支持**:
- `HTTP_PROXY`: HTTP 代理
- `HTTPS_PROXY`: HTTPS 代理
- `ALL_PROXY`: 全局代理
- `NO_PROXY`: 不使用代理的域名

### 监控和日志

**日志级别**:
- Debug: 详细调试信息
- Info: 一般信息
- Warn: 警告信息
- Error: 错误信息

**日志轮转**:
- 按大小轮转
- 按时间轮转
- 保留历史日志

**性能监控**:
- 站点限流
- 下载器连接池
- 数据库查询优化

## 技术亮点

### 1. 站点驱动架构
- 统一的 Site 接口
- 可扩展的 Driver 模式
- 60+ 内置站点
- 自定义站点支持

### 2. 过滤规则引擎
- 三种匹配模式
- 多字段支持
- 优先级系统
- RSS 关联

### 3. 免费结束监控
- 精确定时
- 双重机制
- 应用重启恢复
- 历史归档

### 4. 配置热重载
- 事件驱动
- 防抖机制
- 版本控制
- 无需重启

### 5. 数据库优化
- WAL 模式
- 连接池管理
- 事务支持
- 自动迁移

### 6. 前端架构
- Vue 3 组合式 API
- TypeScript 类型安全
- Element Plus 组件库
- 响应式设计

### 7. 测试覆盖
- 134 个测试文件
- 多种测试类型
- CI/CD 集成
- 覆盖率报告

## 应用场景

1. **PT 站点用户**: 自动化 RSS 订阅、免费种子监控
2. **资源追剧者**: 使用过滤规则自动下载剧集
3. **刷流用户**: 批量管理免费种子，避免 H&R
4. **站点管理员**: 监控用户数据和等级进度
5. **下载器管理**: 统一管理多个下载器实例

## 项目优势

### 技术优势
1. **架构优秀**: 模块化、可扩展、易维护
2. **性能优异**: 并发处理、连接池、缓存优化
3. **类型安全**: TypeScript + Go 强类型系统
4. **测试完善**: 134 个测试文件，高覆盖率

### 功能优势
1. **功能全面**: 覆盖 PT 用户的所有核心需求
2. **站点丰富**: 60+ 站点，6 种架构驱动
3. **智能过滤**: 灵活的规则引擎
4. **自动化**: RSS 订阅、免费监控、自动删种

### 用户体验
1. **界面现代**: Vue 3 + Element Plus
2. **操作便捷**: 浏览器扩展、一键同步
3. **响应迅速**: 搜索缓存、并发处理
4. **数据可视化**: 用户信息聚合展示

### 部署优势
1. **一键部署**: Docker 容器化
2. **多平台**: Linux/Windows (amd64/arm64)
3. **自动更新**: 版本检查和升级
4. **监控完善**: 日志、性能、状态监控

## 核心算法与数据结构深度解析

### 1. 去重算法（Deduper）

**算法类型**: 基于哈希和标题的混合去重

**核心实现**（[deduper.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/deduper.go#L1-L150)）:

```go
func (d *Deduper) Deduplicate(items []TorrentItem) []TorrentItem {
    // 分组：按 InfoHash 分组
    byHash := make(map[string][]TorrentItem)
    noHash := make([]TorrentItem, 0)

    for _, item := range items {
        if item.InfoHash == "" {
            noHash = append(noHash, item)
            continue
        }
        byHash[item.InfoHash] = append(byHash[item.InfoHash], item)
    }

    // 合并重复项：选择最佳版本
    for _, group := range byHash {
        merged := d.mergeDuplicates(group)
        result = append(result, merged)
    }
}
```

**合并策略**（mergeDuplicates）:
1. **做种人数**: 保留做种最多的
2. **下载人数**: 保留下载最多的
3. **完成数**: 保留完成数最多的
4. **上传时间**: 保留最新的
5. **免费状态**: 优先保留免费种子
6. **优惠等级**: 优先保留更好的优惠
7. **标签**: 合并所有标签，去重
8. **下载链接**: 保留可用的下载链接

**标题去重**（DeduplicateByTitle）:
- 标题标准化（移除站点前缀、统一分辨率/编码/格式）
- 标题哈希分组
- 应用相同的合并策略

**时间复杂度**: O(n) 空间复杂度: O(n)

### 2. 排序算法（Ranker）

**算法类型**: 加权评分排序

**评分公式**（[ranker.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/ranker.go#L1-L145)）:

```go
func (r *Ranker) Score(item TorrentItem) float64 {
    var score float64

    // 做种得分（对数缩放，防止极端值）
    if item.Seeders > 0 {
        score += r.config.SeederWeight * logScore(float64(item.Seeders))
    }

    // 下载数得分（表示需求）
    if item.Leechers > 0 {
        score += r.config.LeecherWeight * logScore(float64(item.Leechers))
    }

    // 免费加成
    if item.IsFree() {
        score += r.config.FreeBonus
    }

    // 优惠加成（与优惠等级成比例）
    discountBonus := r.config.FreeBonus * (1 - item.DiscountLevel.GetDownloadRatio())
    score += discountBonus

    // 站点可靠性加成
    if reliability, ok := r.config.SiteReliability[item.SourceSite]; ok {
        score *= (1 + reliability)
    }

    return score
}
```

**对数缩放函数**:
```go
func logScore(value float64) float64 {
    if value <= 0 {
        return 0
    }
    // 简单的线性缩放，递减回报
    return 1 + (value / 10)
}
```

**默认权重配置**:
- 做种权重: 1.0
- 下载权重: 0.5
- 免费加成: 100
- 大小权重: 0.1
- 新鲜度权重: 0.2

**排序算法**: 快速排序（Go 的 sort.Slice）

### 3. 缓存系统

**双层缓存架构**（[cache.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/cache.go#L1-L254)）:

#### L1 缓存（LRU Cache）
- **数据结构**: 哈希表 + 双向链表
- **淘汰策略**: 最近最少使用（LRU）
- **TTL 支持**: 自动过期
- **线程安全**: RWMutex 保护

```go
type LRUCache struct {
    mu       sync.RWMutex
    capacity int
    ttl      time.Duration
    items    map[string]*list.Element  // 哈希表
    order    *list.List                  // 双向链表
}
```

**LRU 操作**:
- **Get**: 移动到链表头部（最近使用）
- **Set**: 新增到头部，超出容量时删除尾部
- **Cleanup**: 遍历删除过期项

#### L2 缓存（可选）
- **接口设计**: 支持 Redis 等外部缓存
- **读写穿透**: L1 未命中时查询 L2，命中后回填 L1
- **双写策略**: 写入时同时更新 L1 和 L2

**缓存配置**:
- L1 容量: 1000 项
- L1 TTL: 5 分钟
- L2 TTL: 1 小时

### 4. 限流算法

**算法类型**: 滑动窗口 + 持久化

**持久化限流器**（[persistent_rate_limiter.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/persistent_rate_limiter.go#L1-L256)）:

```go
type PersistentRateLimiter struct {
    db           *gorm.DB
    siteID       string
    limit        int           // 窗口内最大请求数
    window       time.Duration // 窗口时间长度
    memoryState  *rateLimitState // 内存缓存
    lastSyncTime time.Time
    syncInterval time.Duration // 数据库同步间隔
}

type rateLimitState struct {
    windowStart  time.Time
    requestCount int
}
```

**核心逻辑**:
1. **内存缓存**: 减少数据库访问
2. **批量同步**: 每 5 秒同步一次到数据库
3. **重启恢复**: 从数据库加载状态
4. **滑动窗口**: 精确的速率控制

**使用场景**:
- 站点访问频率限制
- API 调用频率限制
- 防止触发站点反爬

### 5. 标题标准化算法

**算法类型**: 正则表达式替换

**标准化器**（[normalizer.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/normalizer.go#L1-L134)）:

```go
type Normalizer struct {
    resolutionPatterns map[string]*regexp.Regexp
    encodingPatterns   map[string]*regexp.Regexp
    formatPatterns     map[string]*regexp.Regexp
    sitePrefixPattern  *regexp.Regexp
}
```

**标准化步骤**:
1. **移除站点前缀**: `[HDSky]`、`[CHDBits]` 等
2. **标准化分辨率**: `4K/UHD` → `2160p`、`1080i` → `1080p`
3. **标准化编码**: `x264/AVC` → `H.264`、`x265/HEVC` → `H.265`
4. **标准化格式**: `BluRay/BDRemux` → `BluRay`
5. **清理空白**: 多个空格合并为一个

**模式匹配**:
- 分辨率: 2160p、1080p、720p、480p
- 编码: H.264、H.265、AV1、VP9
- 格式: BluRay、WEB-DL、WEBRip、HDTV、DVDRip

### 6. 并发搜索编排

**架构设计**: 编排器模式 + Worker Pool

**搜索编排器**（[search_orchestrator.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/search_orchestrator.go#L1-L200)）:

```go
func (o *SearchOrchestrator) Search(ctx context.Context, query MultiSiteSearchQuery) (*MultiSiteSearchResult, error) {
    // 1. 确定要搜索的站点
    sitesToSearch := o.getSitesToSearch(query.Sites)

    // 2. 并发搜索
    results, errors := o.searchConcurrently(ctx, sitesToSearch, query.SearchQuery)

    // 3. 标准化标题
    for i := range results {
        results[i].Title = o.normalizer.NormalizeTitle(results[i].Title)
    }

    // 4. 应用过滤
    results = o.applyFilters(results, query)

    // 5. 去重
    results = o.deduper.Deduplicate(results)

    // 6. 排序
    results = o.ranker.Rank(results)

    return &MultiSiteSearchResult{...}
}
```

**并发搜索**:
```go
func (o *SearchOrchestrator) searchConcurrently(ctx context.Context, sites []Site, query SearchQuery) ([]TorrentItem, []SearchError) {
    var results []TorrentItem
    var errors []SearchError
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, site := range sites {
        wg.Add(1)
        go func(s Site) {
            defer wg.Done()
            items, err := s.Search(ctx, query)
            if err != nil {
                errors = append(errors, SearchError{Site: s.ID(), Error: err.Error()})
                return
            }
            mu.Lock()
            results = append(results, items...)
            mu.Unlock()
        }(site)
    }
    wg.Wait()
    return results, errors
}
```

**性能优化**:
- 并发搜索多个站点
- 结果聚合和去重
- 超时控制
- 错误隔离（单个站点失败不影响其他）

## 容错与高可用设计

### 1. 熔断器模式

**设计理念**: 快速失败，防止级联故障

**熔断器实现**（[circuit_breaker.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/circuit_breaker.go#L1-L303)）:

```go
type CircuitBreaker struct {
    mu              sync.RWMutex
    name            string
    config          CircuitBreakerConfig
    state           CircuitState
    failures        int
    successes       int
    lastFailureTime time.Time
    halfOpenCount   int
}
```

**状态转换**:
1. **Closed（关闭）**: 正常状态，允许请求通过
2. **Open（开启）**: 失败超过阈值，拒绝请求
3. **Half-Open（半开）**: 超时后允许少量请求测试恢复

**状态机**:
```
Closed --[失败数 >= 阈值]--> Open
Open --[超时]--> Half-Open
Half-Open --[成功数 >= 阈值]--> Closed
Half-Open --[失败]--> Open
```

**默认配置**:
- 失败阈值: 5
- 成功阈值: 2
- 超时时间: 30 秒
- 半开最大请求数: 1

### 2. URL 故障转移

**故障转移管理器**（[failover.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/failover.go#L1-L373)）:

```go
type URLFailoverManager struct {
    config     URLFailoverConfig
    currentIdx int
    mu         sync.RWMutex
    logger     *zap.Logger
}
```

**故障转移策略**:
1. **顺序尝试**: 按配置的 URL 顺序尝试
2. **重试机制**: 每个 URL 最多重试 2 次
3. **智能切换**: 成功后切换到新 URL
4. **错误判断**: 5xx 错误触发故障转移

**可重试错误**:
- 网络错误
- 超时错误
- 临时服务器错误

**不可重试错误**:
- 认证失败
- 会话过期
- 速率限制
- 上下文取消

**重试延迟**: 500ms（指数退避）

### 3. HTTP 客户端容错

**统一 HTTP 客户端**（[http_client.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/http_client.go#L1-L200)）:

```go
type SiteHTTPClient struct {
    session   requests.Session
    userAgent string
    proxyURL  string
    timeout   time.Duration
    idleTime  time.Duration
    maxIdle   int
    keepAlive bool
    logger    *zap.Logger
}
```

**容错特性**:
1. **连接池**: 复用 TCP 连接
2. **超时控制**: 防止长时间阻塞
3. **代理支持**: 环境变量代理
4. **Keep-Alive**: 减少 TLS 握手开销
5. **空闲超时**: 自动清理空闲连接

**配置优化**:
- 超时: 30 秒
- 最大空闲连接: 100
- 每主机最大空闲: 10
- 空闲超时: 90 秒

## 安全机制深度分析

### 1. 认证与授权

**认证流程**（[server.go](file:///home/incast/PT-Forward/examples/pt-tools/web/server.go#L1-L150)）:

```go
func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
    // 1. 读取登录凭据
    user, pass, err := readLogin(r)

    // 2. 查找用户
    u, err := s.store.GetAdmin(user)

    // 3. 验证密码
    if !verifyPassword(u.PasswordHash, pass) {
        if verifyLegacyPassword(u.PasswordHash, pass) {
            // 升级旧密码哈希
            u.PasswordHash = hashPassword(pass)
            _ = s.store.UpdateAdmin(u)
        } else {
            http.Error(w, "密码错误", http.StatusUnauthorized)
            return
        }
    }

    // 4. 创建会话
    sid := randomID()
    s.sessions[sid] = u.Username
    cookie := &http.Cookie{
        Name:     "session",
        Value:    sid,
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
        Path:     "/",
    }
    http.SetCookie(w, cookie)

    // 5. 异步同步用户数据
    go func() {
        userInfoService.FetchAndSaveAllWithConcurrency(ctx, 3, 30*time.Second)
    }()
}
```

**密码哈希**: SHA-256 + Salt

**会话管理**:
- 内存会话存储（重启后失效）
- HttpOnly Cookie（防 XSS）
- SameSite 保护（防 CSRF）

**授权中间件**:
```go
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        sid, err := r.Cookie("session")
        if err != nil || sid.Value == "" || s.sessions[sid.Value] == "" {
            http.Redirect(w, r, "/login", http.StatusFound)
            return
        }
        next(w, r)
    }
}
```

### 2. CORS 配置

**跨域处理**（[server.go](file:///home/incast/PT-Forward/examples/pt-tools/web/server.go#L150-L300)）:

```go
func logMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if strings.HasPrefix(origin, "chrome-extension://") ||
           strings.HasPrefix(origin, "moz-extension://") ||
           strings.HasPrefix(origin, "extension://") {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
            w.Header().Set("Access-Control-Allow-Credentials", "true")
        }
        // ...
    })
}
```

**安全特性**:
- 仅允许浏览器扩展访问
- 支持凭证传递
- 预检请求处理

### 3. 敏感信息脱敏

**浏览器扩展脱敏**（[sanitizer.ts](file:///home/incast/PT-Forward/examples/pt-tools/tools/browser-extension/src/modules/collector/sanitizer.ts#L1-L17)）:

```typescript
export function sanitizeHtml(html: string): string {
    let sanitized = html;

    for (const { pattern, replacement } of SENSITIVE_PATTERNS) {
        sanitized = html.replace(pattern, replacement);
    }

    // 移除 passkey, authkey, apikey
    sanitized = sanitized.replace(
        /(passkey|authkey|apikey)("\s*:\s*"|=)([^"&\s<]+)/gi,
        "$1$2REMOVED",
    );

    // 移除 token
    sanitized = sanitized.replace(/("token"\s*:\s*")([^"]+)(")/gi, "$1REMOVED$3");

    return sanitized;
}
```

**脱敏内容**:
- Passkey
- PHPSESSID / Cookie 值
- 邮箱地址
- IP 地址
- API Key
- Bearer Token
- 邀请链接

## 浏览器扩展技术实现

### 1. 架构设计

**Manifest V3**（[manifest.ts](file:///home/incast/PT-Forward/examples/pt-tools/tools/browser-extension/src/manifest.ts#L1-L36)）:

```typescript
export const manifest = {
    manifest_version: 3,
    name: "__MSG_extensionName__",
    version: "0.1.0",
    permissions: ["storage", "activeTab", "scripting"],
    optional_permissions: ["cookies", "tabs"],
    optional_host_permissions: ["*://*/*"],
    background: {
        service_worker: "background.js",
        type: "module",
    },
    content_scripts: [{
        matches: ["*://*/*"],
        js: ["content.js"],
        run_at: "document_idle",
    }],
    action: {
        default_popup: "src/popup/index.html",
    },
} as const;
```

**权限模型**:
- **storage**: 存储配置和会话
- **activeTab**: 读取当前标签页
- **scripting**: 注入脚本
- **cookies**: 读取 Cookie（可选）
- **tabs**: 管理标签页（可选）

### 2. 功能模块

#### Cookie 同步模块
- 读取浏览器 Cookie
- 发送到 pt-tools API
- 批量同步支持
- 自动同步选项

#### 数据采集模块
- **页面检测**: 识别 PT 站点和页面类型
- **HTML 采集**: 捕获完整页面 HTML
- **自动脱敏**: 移除敏感信息
- **ZIP 打包**: 打包采集数据
- **GitHub 集成**: 自动创建 Issue

**采集流程**（[capturer.ts](file:///home/incast/PT-Forward/examples/pt-tools/tools/browser-extension/src/modules/collector/capturer.ts#L1-L18)）:
1. 检测站点架构
2. 识别页面类型（列表/详情/用户信息）
3. 采集 HTML
4. 自动脱敏
5. 生成采集记录

**站点识别**:
- 域名匹配
- HTML 结构分析
- 已知站点库（60+ 站点）

### 3. 部署与发布

**构建流程**:
```bash
pnpm install
pnpm build        # 构建
pnpm run pack     # 打包 ZIP
```

**发布渠道**:
- Edge Add-ons 商店（推荐）
- Chrome Web Store
- Firefox Add-ons
- 手动安装（开发者模式）

**自动化发布**（CI/CD）:
- 检查站点一致性
- 自动构建
- 自动发布到商店

## CI/CD 流程深度分析

### 1. 主 CI 工作流

**ci.yml**（[ci.yml](file:///home/incast/PT-Forward/examples/pt-tools/.github/workflows/ci.yml#L1-L310)）:

**阶段划分**:

1. **前端构建**:
   - Node.js 25.2.0
   - pnpm 10
   - 依赖缓存优化
   - Lint 检查（oxlint）
   - 类型检查（vue-tsc）
   - 生产构建

2. **Go 构建与测试**:
   - Go 1.26.2
   - 依赖缓存
   - 编译检查
   - 单元测试（并发 + 覆盖率）
   - 覆盖率上传

3. **Go Lint**:
   - golangci-lint
   - 超时 5 分钟
   - 所有规则检查

4. **Go 安全**:
   - govulncheck
   - 漏洞扫描

5. **格式检查**:
   - goimports
   - gofumpt
   - Git 差异检查

6. **扩展构建**:
   - 站点一致性检查
   - 扩展构建

**并发控制**:
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

**本地测试**:
- 支持 `act` 本地运行 CI
- 环境变量 `ACT` 检测

### 2. 发布工作流

**release-please.yml**（[release-please.yml](file:///home/incast/PT-Forward/examples/pt-tools/.github/workflows/release-please.yml#L1-L255)）:

**自动化发布流程**:

1. **Release Please 触发**:
   - 监听 main 分支提交
   - 自动生成版本号
   - 创建 Release PR
   - 发布时触发构建

2. **构建与打包**:
   - 多平台二进制构建
   - Docker 镜像构建
   - 扩展打包
   - 文件重组

3. **发布资产**:
   - Linux (amd64/arm64)
   - Windows (amd64/arm64)
   - Docker 镜像
   - 浏览器扩展

4. **Changelog 生成**:
   - git-cliff 生成
   - 格式化安装说明
   - 更新 CHANGELOG.md

**平台支持**:
- Linux x86_64 / aarch64
- Windows x86_64 / aarch64
- Docker (linux/amd64, linux/arm64)

**Docker 镜像**:
- `sunerpy/pt-tools:{version}`
- `sunerpy/pt-tools:latest`

### 3. 其他工作流

**dependabot-auto-merge**: 自动合并依赖更新
**extension-publish**: 扩展自动发布
**issue-attachment-check**: Issue 附件检查
**close-stale-site-requests**: 关闭过期站点请求

## 数据库迁移系统

### 1. 迁移服务

**MigrationService**（[v1_to_v2.go](file:///home/incast/PT-Forward/examples/pt-tools/core/migration/v1_to_v2.go#L1-L150)）:

```go
type MigrationService struct {
    db        *gorm.DB
    backupDir string
}
```

**核心功能**:

1. **备份创建**:
   - 导出所有配置到 JSON
   - 时间戳命名
   - 目录自动创建

2. **备份恢复**:
   - 读取 JSON 备份
   - 事务恢复
   - 数据清空后导入

3. **版本迁移**:
   - v1 → v2 配置迁移
   - 下载器配置转换
   - 站点配置迁移
   - RSS 订阅迁移

**备份结构**:
```go
type BackupData struct {
    Version   string                   `json:"version"`
    CreatedAt time.Time                `json:"created_at"`
    Global    models.SettingsGlobal    `json:"global"`
    Qbit      models.QbitSettings      `json:"qbit"`
    Sites     []models.SiteSetting     `json:"sites"`
    RSS       []models.RSSSubscription `json:"rss"`
}
```

**备份目录**: `~/.pt-tools/backups/`

**备份命名**: `config_backup_20060102_150405.json`

### 2. 特殊迁移案例

**CMCT → SpringSunday 迁移**:
- 站点 ID 更新
- RSS 订阅迁移
- 用户数据保留
- 种子信息迁移

**迁移策略**:
- 向后兼容
- 数据完整性
- 原子操作
- 失败回滚

## 性能优化深度分析

### 1. 数据库优化

**SQLite 优化**（[init.go](file:///home/incast/PT-Forward/examples/pt-tools/models/init.go#L1-L285)）:

```go
dsn := fmt.Sprintf("file:%s?_busy_timeout=30000&_txlock=immediate&cache=shared", dbFile)

// WAL 模式
db.Exec("PRAGMA journal_mode=WAL;")

// 同步模式 NORMAL（平衡性能和持久性）
db.Exec("PRAGMA synchronous=NORMAL;")

// 连接池限制
sqlDB.SetMaxOpenConns(1)
sqlDB.SetMaxIdleConns(1)
```

**优化效果**:
- WAL 模式：读写并发
- 同步模式：减少磁盘 I/O
- 连接池：避免文件锁冲突
- 缓存共享：多进程缓存

### 2. HTTP 客户端优化

**连接池管理**:
- 最大空闲连接: 100
- 每主机最大空闲: 10
- 空闲超时: 90 秒
- Keep-Alive: 启用

**代理支持**:
- 环境变量自动解析
- 请求级别代理
- 白名单支持

**超时控制**:
- 默认超时: 30 秒
- 上下文超时
- 分阶段超时

### 3. 并发优化

**搜索并发**: 多站点并发搜索
**数据同步**: 并发获取用户信息（可配置并发数）
**批量操作**: 批量更新数据库

### 4. 缓存优化

**双层缓存**:
- L1: 内存 LRU 缓存
- L2: 可选 Redis 缓存
- 写透策略
- 读穿透策略

**缓存策略**:
- 用户信息: 5 分钟
- 搜索结果: 10 分钟
- 站点数据: 30 分钟
- 图标缓存: 1 小时

## 测试用例设计模式深度分析

### 1. 测试分层架构

pt-tools 采用了完整的测试金字塔结构，包含 100+ 个测试文件：

#### 单元测试层（Unit Tests）
**特点**: 快速执行、无依赖、可并行

**核心模式**:
- **表驱动测试**（Table-Driven Testing）
- **基于属性的测试**（Property-Based Testing）
- **Mock 对象测试**
- **Fixtures 测试**

#### 集成测试层（Integration Tests）
**特点**: 多组件协作、数据库交互、HTTP 服务测试

**核心模式**:
- **临时数据库测试**
- **HTTP 服务器测试**
- **端到端流程测试**

#### 端到端测试层（E2E Tests）
**特点**: 完整业务流程、真实环境模拟

**核心模式**:
- **RSS 订阅完整流程**
- **下载器集成测试**
- **多站点搜索测试**

### 2. 表驱动测试模式

**核心思想**: 使用结构体数组定义测试用例，通过循环执行测试逻辑

**示例**（[nexusphp_parser_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/nexusphp_parser_test.go#L18-L50)）:
```go
func TestHDSkyParser(t *testing.T) {
    parser := NewHDSkyParser()

    t.Run("parse free discount", func(t *testing.T) {
        html := `<html><h1><font class="free">Free</font></h1></html>`
        doc := parseHTML(t, html)
        discount, _ := parser.ParseDiscount(doc)
        assert.Equal(t, DiscountFree, discount)
    })

    t.Run("parse 2x free discount", func(t *testing.T) {
        html := `<html><h1><font class="twoupfree">2x Free</font></h1></html>`
        doc := parseHTML(t, html)
        discount, _ := parser.ParseDiscount(doc)
        assert.Equal(t, Discount2xFree, discount)
    })
}
```

**优势**:
- 测试用例清晰、易于维护
- 新增测试用例只需添加结构体字段
- 失败信息包含完整测试数据
- 支持并行执行

**应用场景**:
- 算法逻辑测试（去重、排序、过滤）
- 解析器测试（HTML、JSON、XML）
- 工具函数测试

### 3. 基于属性的测试（Property-Based Testing）

**核心思想**: 使用随机生成的输入验证通用属性，而非固定测试用例

**示例**（[matcher_test.go](file:///home/incast/PT-Forward/examples/pt-tools/internal/filter/matcher_test.go#L13-L60)）:
```go
func TestKeywordMatcherCaseInsensitive(t *testing.T) {
    parameters := gopter.DefaultTestParameters()
    parameters.MinSuccessfulTests = 100
    properties := gopter.NewProperties(parameters)

    // Property: Match returns true iff lowercase title contains lowercase keyword
    properties.Property("keyword match is case-insensitive containment", prop.ForAll(
        func(keyword, title string) bool {
            if keyword == "" {
                return true
            }
            matcher, err := NewKeywordMatcher(keyword)
            if err != nil {
                return true
            }

            result := matcher.Match(title)
            expected := strings.Contains(strings.ToLower(title), strings.ToLower(keyword))
            return result == expected
        },
        gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 50 }),
        gen.AlphaString(),
    ))

    properties.TestingRun(t)
}
```

**优势**:
- 发现边界情况和隐藏 bug
- 验证数学属性（结合律、交换律）
- 提高代码覆盖率
- 自动生成大量测试数据

**应用场景**:
- 过滤器匹配逻辑
- 排序算法验证
- 数据转换函数
- 迁移逻辑验证

### 4. Fixture 测试模式

**核心思想**: 使用预定义的测试数据（HTML、JSON）进行测试

**示例**（[btschool_fixture_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/definitions/btschool_fixture_test.go#L1-L40)）:
```go
const btschoolSearchFixture = `<html><body>
<table class="torrents"><tbody>
<tr>
  <td class="rowfollow"><img alt="电影/Movies" /></td>
  <td class="rowfollow"><table class="torrentname"><tr><td class="embedded">
    <a href="details.php?id=260153">First.Man.2018.BluRay.1080p.BTSCHOOL</a>
    <img class="pro_free" src="pic/trans.gif" alt="Free" />
    <br /><span>登月第一人</span>
  </td></tr></table></td>
  <td class="rowfollow">34.32 GB</td>
  <td class="rowfollow">149</td>
  <td class="rowfollow">4</td>
  <td class="rowfollow">155</td>
</tr>
</tbody></table>
</body></html>`
```

**优势**:
- 测试数据与代码分离
- 易于维护和更新
- 支持真实场景模拟
- 便于回归测试

**应用场景**:
- 站点驱动测试（60+ 站点）
- HTML 解析测试
- API 响应测试
- 数据库迁移测试

### 5. Mock 对象测试模式

**核心思想**: 使用模拟对象替代真实依赖，隔离测试环境

**示例**（[qbit_mock_test.go](file:///home/incast/PT-Forward/examples/pt-tools/mocks/qbit_mock_test.go#L1-L19)）:
```go
type PTSiteMock[T models.ResType] struct {
    Enabled bool
    Detail  *models.APIResponse[T]
    DlHash  string
    DlErr   error
}

func (m *PTSiteMock[T]) GetTorrentDetails(item *gofeed.Item) (*models.APIResponse[T], error) {
    return m.Detail, nil
}
func (m *PTSiteMock[T]) IsEnabled() bool { return m.Enabled }
func (m *PTSiteMock[T]) DownloadTorrent(url, title, dir string) (string, error) {
    return m.DlHash, m.DlErr
}
```

**优势**:
- 隔离外部依赖
- 控制测试场景
- 提高测试速度
- 避免副作用

**应用场景**:
- 站点驱动测试
- 下载器客户端测试
- HTTP 客户端测试
- 数据库操作测试

### 6. 临时数据库测试模式

**核心思想**: 每个测试使用独立的内存数据库，确保测试隔离

**示例**（[filter_rule_test.go](file:///home/incast/PT-Forward/examples/pt-tools/models/filter_rule_test.go#L10-L35)）:
```go
func setupFilterRuleTestDB(t *testing.T) (*TorrentDB, func()) {
    t.Helper()

    tmpDir, err := os.MkdirTemp("", "filter_rule_test")
    require.NoError(t, err)

    dbPath := filepath.Join(tmpDir, "test.db")
    db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Silent),
    })
    require.NoError(t, err)

    err = db.AutoMigrate(&FilterRule{})
    require.NoError(t, err)

    torrentDB := &TorrentDB{DB: db}

    cleanup := func() {
        sqlDB, _ := db.DB()
        if sqlDB != nil {
            sqlDB.Close()
        }
        os.RemoveAll(tmpDir)
    }

    return torrentDB, cleanup
}
```

**优势**:
- 测试完全隔离
- 快速执行
- 不影响真实数据
- 自动清理资源

**应用场景**:
- 数据模型测试
- 配置存储测试
- 迁移逻辑测试
- 限流器测试

### 7. HTTP 服务器测试模式

**核心思想**: 使用 httptest 包模拟 HTTP 请求和响应

**示例**（[api_search_test.go](file:///home/incast/PT-Forward/examples/pt-tools/web/api_search_test.go#L77-L105)）:
```go
func TestApiMultiSiteSearch(t *testing.T) {
    orchestrator := setupTestSearchOrchestrator()
    InitSearchOrchestrator(orchestrator)
    defer InitSearchOrchestrator(nil)

    server := &Server{}

    req := MultiSiteSearchRequest{
        Keyword: "test",
    }
    body, _ := json.Marshal(req)

    httpReq := httptest.NewRequest(http.MethodPost, "/api/v2/search/multi", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    server.apiMultiSiteSearch(w, httpReq)

    assert.Equal(t, http.StatusOK, w.Code)

    var response MultiSiteSearchResponse
    err := json.NewDecoder(w.Body).Decode(&response)
    require.NoError(t, err)

    assert.Len(t, response.Items, 2)
    assert.Equal(t, 2, response.TotalResults)
}
```

**优势**:
- 无需启动真实服务器
- 快速执行
- 完整的 HTTP 请求/响应测试
- 支持中间件测试

**应用场景**:
- API 端点测试
- 认证中间件测试
- CORS 配置测试
- 错误处理测试

### 8. 并发安全测试模式

**核心思想**: 使用 goroutine 和 sync 包验证并发安全性

**示例**（[http_client_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/http_client_test.go#L30-L50)）:
```go
func TestHTTPClientPool_GetSession_Concurrent(t *testing.T) {
    pool := NewHTTPClientPool(DefaultHTTPClientConfig(), nil)

    var wg sync.WaitGroup
    sessions := make([]interface{}, 10)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            sessions[idx] = pool.GetSession("site1")
        }(i)
    }

    wg.Wait()

    for i := 1; i < 10; i++ {
        assert.Equal(t, sessions[0], sessions[i])
    }
}
```

**优势**:
- 验证并发安全性
- 发现竞态条件
- 测试锁机制
- 验证资源管理

**应用场景**:
- 连接池测试
- 缓存测试
- 限流器测试
- 并发搜索测试

### 9. Fixture 注册表模式

**核心思想**: 使用注册表统一管理所有站点的 Fixture 测试

**示例**（[fixture_helper_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/definitions/fixture_helper_test.go#L9-L25)）:
```go
type FixtureSuite struct {
    SiteID   string
    Search   func(*testing.T)
    Detail   func(*testing.T)
    UserInfo func(*testing.T)
}

var fixtureRegistry = map[string]FixtureSuite{}

func RegisterFixtureSuite(s FixtureSuite) {
    if s.SiteID == "" {
        panic("RegisterFixtureSuite: empty SiteID")
    }
    if _, dup := fixtureRegistry[s.SiteID]; dup {
        panic("RegisterFixtureSuite: duplicate SiteID " + s.SiteID)
    }
    fixtureRegistry[s.SiteID] = s
}
```

**优势**:
- 统一管理 60+ 站点测试
- 避免重复代码
- 支持批量执行
- 易于新增站点

**应用场景**:
- 站点驱动测试套件
- 搜索功能测试
- 详情页测试
- 用户信息测试

### 10. 秘密检测模式

**核心思想**: 在测试中自动检测敏感信息泄露

**示例**（[fixture_helper_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/definitions/fixture_helper_test.go#L30-56)）:
```go
var secretDenyPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)c_secure_(uid|pass|login|tracker_ssl)\s*=`),
    regexp.MustCompile(`(?i)phpsessid\s*=`),
    regexp.MustCompile(`(?i)(passkey|apikey|api_key)\s*=\s*[a-f0-9]{32,64}\b`),
    bearerTokenPattern,
}

func RequireNoSecrets(t *testing.T, name, data string) {
    t.Helper()
    for _, re := range secretDenyPatterns {
        if loc := re.FindStringIndex(data); loc != nil {
            match := data[loc[0]:loc[1]]
            if re == bearerTokenPattern {
                upper := strings.ToUpper(match)
                if strings.Contains(upper, "BEARER FAKE_") || strings.Contains(upper, "BEARER TEST_") {
                    continue
                }
            }
            start := loc[0]
            end := loc[1]
            if start > 20 {
                start = loc[0] - 20
            }
            if end+20 < len(data) {
                end = loc[1] + 20
            }
            t.Fatalf("fixture %q contains suspected credential: ...%s...\nMatched pattern: %s",
                name, data[start:end], re.String())
        }
    }
}
```

**优势**:
- 防止敏感信息泄露
- 自动化安全检查
- 支持自定义模式
- CI/CD 集成

**应用场景**:
- Fixture 数据验证
- HTML 响应检查
- API 响应检查
- 日志输出检查

### 11. 测试辅助工具模式

**核心思想**: 提供可复用的测试辅助函数

**示例**（[fixture_helper_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/definitions/fixture_helper_test.go#L58-75)）:
```go
func DecodeFixtureJSON[T any](t *testing.T, name, raw string) T {
    t.Helper()
    RequireNoSecrets(t, name, raw)
    var v T
    require.NoError(t, json.Unmarshal([]byte(raw), &v), "decode fixture %q", name)
    return v
}

func FixtureDoc(t *testing.T, name, html string) *goquery.Document {
    t.Helper()
    RequireNoSecrets(t, name, html)
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    require.NoError(t, err, "parse fixture %q", name)
    return doc
}
```

**优势**:
- 减少重复代码
- 统一测试逻辑
- 提高可读性
- 便于维护

**应用场景**:
- Fixture 解析
- 数据验证
- 错误处理
- 资源清理

### 12. 容错机制测试模式

**核心思想**: 测试系统在异常情况下的行为

**示例**（[failover_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/failover_test.go#L20-L45)）:
```go
func TestURLFailoverManager_ExecuteWithFailover(t *testing.T) {
    t.Run("failover to second URL", func(t *testing.T) {
        config := URLFailoverConfig{
            BaseURLs:   []string{"http://url1", "http://url2", "http://url3"},
            RetryDelay: 10 * time.Millisecond,
            MaxRetries: 0,
            Timeout:    5 * time.Second,
        }
        manager := NewURLFailoverManager(config, nil)

        callCount := 0
        var lastURL string
        err := manager.ExecuteWithFailover(context.Background(), func(baseURL string) error {
            callCount++
            lastURL = baseURL
            if baseURL == "http://url1" {
                return errors.New("url1 failed")
            }
            return nil
        })

        assert.NoError(t, err)
        assert.Equal(t, 2, callCount)
        assert.Equal(t, "http://url2", lastURL)
    })
}
```

**优势**:
- 验证容错机制
- 测试边界条件
- 确保系统稳定性
- 发现隐藏 bug

**应用场景**:
- 熔断器测试
- 故障转移测试
- 重试机制测试
- 超时处理测试

### 13. 持久化测试模式

**核心思想**: 测试数据持久化和恢复能力

**示例**（[persistent_rate_limiter_test.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/persistent_rate_limiter_test.go#L50-70)）:
```go
func TestPersistentRateLimiter_Persistence(t *testing.T) {
    db := setupTestDB(t)

    limiter1 := NewPersistentRateLimiter(PersistentRateLimiterConfig{
        DB:       db,
        SiteID:   "persist-site",
        Limit:    10,
        Window:   time.Minute,
        SyncRate: 0,
    })

    for i := 0; i < 5; i++ {
        limiter1.Allow()
    }
    limiter1.ForceSync()

    limiter2 := NewPersistentRateLimiter(PersistentRateLimiterConfig{
        DB:     db,
        SiteID: "persist-site",
        Limit:  10,
        Window: time.Minute,
    })

    remaining, _ := limiter2.Stats()
    assert.Equal(t, 5, remaining, "new limiter should restore count from DB")
}
```

**优势**:
- 验证数据持久化
- 测试恢复能力
- 确保数据一致性
- 发现并发问题

**应用场景**:
- 限流器测试
- 缓存持久化测试
- 配置保存测试
- 状态恢复测试

### 14. 测试组织模式

**测试文件命名**:
- `*_test.go`: 单元测试
- `*_fixture_test.go`: Fixture 测试
- `*_integration_test.go`: 集成测试
- `*_e2e_test.go`: 端到端测试

**测试函数命名**:
- `TestXxx`: 基础测试
- `BenchmarkXxx`: 性能测试
- `ExampleXxx`: 示例测试

**子测试组织**:
```go
func TestDeduper_MergeDuplicates(t *testing.T) {
    t.Run("Seeders", func(t *testing.T) { ... })
    t.Run("Leechers", func(t *testing.T) { ... })
    t.Run("Snatched", func(t *testing.T) { ... })
}
```

### 15. 测试最佳实践

#### 15.1 使用 t.Helper()
```go
func setupTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    // 测试辅助函数使用 t.Helper() 标记
}
```

#### 15.2 使用 require 和 assert
```go
require.NoError(t, err)  // 失败立即停止
assert.Equal(t, expected, actual)  // 失败继续执行
```

#### 15.3 表驱动测试结构
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"case 1", "input1", "output1"},
    {"case 2", "input2", "output2"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // 测试逻辑
    })
}
```

#### 15.4 资源清理
```go
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()
    db := setupDB(t)
    defer cleanup(db)

    // 测试逻辑
}
```

#### 15.5 并发测试
```go
func TestConcurrent(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // 并发测试逻辑
        }()
    }
    wg.Wait()
}
```

### 16. 测试覆盖分析

#### 测试文件分布（100+ 个测试文件）:
- **site/v2/**: 45 个（站点驱动核心测试）
- **web/**: 11 个（API 测试）
- **models/**: 10 个（数据模型测试）
- **site/v2/definitions/**: 35 个（站点定义测试）
- **scheduler/**: 4 个（调度器测试）
- **internal/**: 6 个（内部逻辑测试）
- **cmd/**: 11 个（命令测试）
- **thirdpart/downloader/**: 4 个（下载器测试）
- **version/**: 4 个（版本管理测试）
- **utils/**: 3 个（工具函数测试）
- **core/**: 3 个（核心功能测试）
- **config/**: 1 个（配置测试）
- **mocks/**: 2 个（Mock 测试）
- **global/**: 1 个（全局测试）

#### 测试类型统计:
- **单元测试**: ~80%
- **集成测试**: ~15%
- **端到端测试**: ~5%

### 17. 测试工具链

**Go 测试工具**:
- `testify`: 断言和模拟
- `gopter`: 基于属性的测试
- `gorm.io/gorm`: 数据库测试
- `net/http/httptest`: HTTP 服务器测试

**前端测试工具**:
- `vitest`: 单元测试框架
- `vue-test-utils`: Vue 组件测试
- `@vue/test-utils`: Vue 3 测试工具

**CI/CD 集成**:
- GitHub Actions
- Docker 容器化测试
- 代码覆盖率报告

## 前端架构与组件设计深度分析

### 1. 技术栈选型

**核心框架**（[package.json](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/package.json#L1-L53)）:
```json
{
  "dependencies": {
    "vue": "^3.5.31",
    "vue-router": "^5.0.4",
    "pinia": "^3.0.4",
    "element-plus": "^2.13.6",
    "@element-plus/icons-vue": "^2.3.2",
    "@vueuse/core": "^14.2.1",
    "marked": "^17.0.6",
    "dompurify": "^3.3.3"
  },
  "devDependencies": {
    "vite": "^8.0.5",
    "typescript": "~6.0.2",
    "vitest": "^4.1.2",
    "vue-tsc": "^3.2.6",
    "oxlint": "^1.58.0",
    "oxfmt": "^0.43.0"
  }
}
```

**选型理由**:
- **Vue 3.5**: 最新稳定版，Composition API 提供更好的代码组织
- **TypeScript**: 类型安全，减少运行时错误
- **Pinia**: Vue 3 官方推荐的状态管理，比 Vuex 更轻量
- **Element Plus**: 企业级 UI 组件库，丰富的表单和表格组件
- **Vite 8**: 极快的开发服务器和构建工具
- **@vueuse/core**: Vue 组合式 API 工具集，提供常用功能

### 2. 项目结构

**目录组织**:
```
web/frontend/src/
├── api/                    # API 封装层
│   └── index.ts           # 统一的 API 请求和类型定义
├── assets/                # 静态资源
├── components/            # 可复用组件
│   ├── LevelProgress.vue  # 等级进度条
│   ├── LevelTooltip.vue   # 等级提示
│   ├── SiteAvatar.vue     # 站点头像
│   └── VersionChecker.vue # 版本检查
├── composables/           # 组合式函数
│   └── useToast.ts       # Toast 提示
├── router/                # 路由配置
│   └── index.ts          # 路由定义
├── stores/                # Pinia 状态管理
│   ├── siteLevels.ts     # 站点等级状态
│   ├── theme.ts          # 主题状态
│   ├── logLevel.ts       # 日志级别状态
│   └── version.ts        # 版本信息状态
├── styles/                # 全局样式
├── utils/                 # 工具函数
│   └── format.ts         # 格式化工具
├── views/                 # 页面组件（16个）
│   ├── UserInfoDashboard.vue   # 用户统计
│   ├── TorrentSearch.vue      # 种子搜索
│   ├── FilterRules.vue        # 过滤规则
│   ├── SiteList.vue           # 站点列表
│   ├── SiteDetail.vue         # 站点详情
│   ├── GlobalSettings.vue     # 全局设置
│   ├── DownloaderSettings.vue # 下载器设置
│   ├── TaskList.vue           # 任务列表
│   ├── LogViewer.vue          # 日志查看
│   └── ...                    # 其他页面
├── App.vue                # 根组件
└── main.ts                # 应用入口
```

### 3. 应用初始化流程

**入口文件**（[main.ts](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/src/main.ts#L1-L29)）:
```typescript
import * as ElementPlusIconsVue from "@element-plus/icons-vue";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { type Component, createApp } from "vue";
import "element-plus/dist/index.css";
import "element-plus/theme-chalk/dark/css-vars.css";
import "./styles/theme.scss";
import "./styles/common-page.css";
import "./styles/app-layout.css";
import "./styles/dashboard.css";
import "./styles/form-page.css";
import "./styles/table-page.css";
import "./styles/shared-components.css";
import App from "./App.vue";
import router from "./router";
import "./styles/main.css";

const app = createApp(App);

// 注册所有图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component as Component);
}

app.use(createPinia());
app.use(router);
app.use(ElementPlus);

app.mount("#app");
```

**初始化步骤**:
1. 创建 Vue 应用实例
2. 注册所有 Element Plus 图标（动态遍历注册）
3. 安装 Pinia 状态管理
4. 安装 Vue Router 路由
5. 安装 Element Plus UI 框架
6. 挂载到 DOM

### 4. 路由设计

**路由配置**（[router/index.ts](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/src/router/index.ts#L1-L100)）:
```typescript
const router = createRouter({
  history: createWebHashHistory(),  // 使用 Hash 模式，避免服务器配置
  routes: [
    { path: "/", redirect: "/userinfo" },
    { path: "/userinfo", name: "userinfo", component: () => import("@/views/UserInfoDashboard.vue") },
    { path: "/search", name: "search", component: () => import("@/views/TorrentSearch.vue") },
    { path: "/filter-rules", name: "filter-rules", component: () => import("@/views/FilterRules.vue") },
    { path: "/sites", name: "sites", component: () => import("@/views/SiteList.vue") },
    { path: "/sites/:name", name: "site-detail", component: () => import("@/views/SiteDetail.vue") },
    { path: "/tasks", name: "tasks", component: () => import("@/views/TaskList.vue") },
    { path: "/logs", name: "logs", component: () => import("@/views/LogViewer.vue") },
    // ... 更多路由
  ],
});
```

**设计特点**:
- **懒加载**: 所有路由组件使用动态导入，减少初始加载体积
- **Hash 模式**: 避免服务器端路由配置问题
- **Meta 信息**: 使用 meta 字段存储页面标题等信息
- **嵌套路由**: 支持站点详情等复杂页面结构

### 5. API 封装层

**统一 API 客户端**（[api/index.ts](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/src/api/index.ts#L1-L50)）:
```typescript
class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    credentials: "same-origin",  // 发送 Cookie
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    const msg = await response.text();
    throw new ApiError(response.status, msg || `HTTP ${response.status}`);
  }

  return response.json() as Promise<T>;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: data ? JSON.stringify(data) : undefined,
    }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
```

**API 模块化组织**:
```typescript
// 全局配置 API
export const globalApi = {
  get: () => api.get<GlobalSettings>("/api/global"),
  save: (data: GlobalSettings) => api.post<void>("/api/global", data),
};

// 下载器 API
export const downloadersApi = {
  list: () => api.get<DownloaderSetting[]>("/api/downloaders"),
  create: (data: DownloaderSetting) => api.post<DownloaderSetting>("/api/downloaders", data),
  update: (id: number, data: DownloaderSetting) =>
    request<DownloaderSetting>(`/api/downloaders/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  health: (id: number) => api.get<DownloaderHealthResponse>(`/api/downloaders/${id}/health`),
};

// 种子搜索 API
export const searchApi = {
  multiSite: (req: MultiSiteSearchRequest) =>
    api.post<MultiSiteSearchResponse>("/api/v2/search/multi", req),
  getSites: async () => {
    const resp = await api.get<{ sites: string[] }>("/api/v2/search/sites");
    return resp.sites || [];
  },
  clearCache: () => api.post<{ status: string }>("/api/v2/search/cache/clear"),
};
```

**设计优势**:
- **类型安全**: 使用 TypeScript 泛型确保类型正确
- **统一错误处理**: 自定义 ApiError 类统一处理错误
- **模块化**: 按功能模块组织 API，易于维护
- **Cookie 认证**: 使用 `credentials: "same-origin"` 自动发送 Cookie

### 6. 状态管理（Pinia）

**站点等级状态**（[stores/siteLevels.ts](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/src/stores/siteLevels.ts#L1-L66)）:
```typescript
export const useSiteLevelsStore = defineStore("siteLevels", () => {
  const levelsCache = ref<Record<string, SiteLevelsResponse>>({});
  const loading = ref(false);
  const loaded = ref(false);
  const error = ref<string | null>(null);

  async function loadAll() {
    if (loaded.value || loading.value) return;

    loading.value = true;
    error.value = null;

    try {
      const response = await siteLevelsApi.getAll();
      levelsCache.value = response.sites || {};
      loaded.value = true;
    } catch (e) {
      error.value = (e as Error).message || "加载等级信息失败";
    } finally {
      loading.value = false;
    }
  }

  function getLevels(siteId: string): SiteLevelRequirement[] {
    const siteLower = siteId.toLowerCase();
    return levelsCache.value[siteLower]?.levels || [];
  }

  function getSiteName(siteId: string): string {
    const siteLower = siteId.toLowerCase();
    return levelsCache.value[siteLower]?.siteName || siteId;
  }

  function hasLevels(siteId: string): boolean {
    const siteLower = siteId.toLowerCase();
    return !!levelsCache.value[siteLower]?.levels?.length;
  }

  function reset() {
    levelsCache.value = {};
    loaded.value = false;
    error.value = null;
  }

  return {
    levelsCache,
    loading,
    loaded,
    error,
    loadAll,
    getLevels,
    getSiteName,
    hasLevels,
    reset,
  };
});
```

**状态管理模式**:
- **Composition API**: 使用 `defineStore` + 组合式函数
- **响应式缓存**: 使用 `ref` 实现响应式状态
- **防重复加载**: 检查 `loaded` 和 `loading` 状态
- **错误处理**: 捕获并存储错误信息
- **工具方法**: 提供 `getLevels`、`getSiteName` 等便捷方法

### 7. 组件设计模式

**可复用组件**:
- **LevelProgress.vue**: 等级进度条组件，显示用户等级进度
- **LevelTooltip.vue**: 等级提示组件，显示等级详情
- **SiteAvatar.vue**: 站点头像组件，显示站点图标
- **VersionChecker.vue**: 版本检查组件，检测更新

**页面组件**（16个）:
1. **UserInfoDashboard.vue**: 用户统计仪表板
2. **TorrentSearch.vue**: 种子搜索页面
3. **FilterRules.vue**: 过滤规则管理
4. **SiteList.vue**: 站点列表
5. **SiteDetail.vue**: 站点详情
6. **GlobalSettings.vue**: 全局设置
7. **DownloaderSettings.vue**: 下载器设置
8. **DownloaderHub.vue**: 下载器 Web UI 集成
9. **TaskList.vue**: 任务列表
10. **PausedTorrents.vue**: 暂停任务
11. **LogViewer.vue**: 日志查看
12. **ChangePassword.vue**: 修改密码
13. **AutoCleanup.vue**: 自动清理设置
14. **UserDataExport.vue**: 数据导出
15. **QbitSettings.vue**: qBittorrent 设置（已废弃）
16. **DynamicSiteSettings.vue**: 动态站点设置（已隐藏）

### 8. Vite 构建配置

**构建优化**（[vite.config.ts](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/vite.config.ts#L1-L55)）:
```typescript
export default defineConfig(({ command }) => ({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
  build: {
    outDir: "../static/dist",
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            if (id.includes("element-plus")) {
              return "element-plus";
            }
            if (id.includes("vue") || id.includes("pinia") || id.includes("vue-router")) {
              return "vue-vendor";
            }
            if (id.includes("@element-plus/icons-vue")) {
              return "element-plus-icons";
            }
            return "vendor";
          }
        },
      },
    },
  },
  esbuild: {
    drop: command === "build" ? ["console", "debugger"] : [],
    pure: command === "build"
      ? ["console.log", "console.info", "console.debug", "console.warn", "console.error"]
      : [],
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/logout": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
}));
```

**优化策略**:
- **代码分割**: 手动分割 Element Plus、Vue、其他依赖
- **Tree Shaking**: 生产环境移除 console 和 debugger
- **开发代理**: 代理 API 请求到后端服务器
- **路径别名**: 使用 `@` 别名简化导入路径

### 9. 代码质量工具

**前端工具链**:
```json
{
  "scripts": {
    "lint": "oxlint --fix",
    "lint:check": "oxlint",
    "fmt": "oxfmt",
    "fmt:check": "oxfmt --check",
    "test": "vitest run",
    "test:watch": "vitest"
  }
}
```

- **oxlint**: 快速的 JavaScript/TypeScript linter
- **oxfmt**: 代码格式化工具（支持多语言）
- **vitest**: 单元测试框架
- **vue-tsc**: Vue TypeScript 类型检查

**格式化配置**（[.oxfmtrc.json](file:///home/incast/PT-Forward/examples/pt-tools/.oxfmtrc.json)）:
```json
{
  "printWidth": 100,
  "tabWidth": 2,
  "useTabs": false,
  "bracketSameLine": true,
  "singleQuote": false,
  "trailingComma": "all"
}
```

## 配置管理系统与热重载机制

### 1. 配置存储架构

**ConfigStore**（[core/config_store.go](file:///home/incast/PT-Forward/examples/pt-tools/core/config_store.go#L1-L150)）:
```go
type ConfigStore struct {
    db *models.TorrentDB
}

// Load 将 SQLite 中的配置组装为运行时 Config
func (s *ConfigStore) Load() (*models.Config, error) {
    db := s.db.DB
    var out models.Config
    if err := db.Transaction(func(tx *gorm.DB) error {
        // 加载全局配置
        var gs models.SettingsGlobal
        if e := tx.First(&gs).Error; e == nil {
            out.Global.DefaultIntervalMinutes = gs.DefaultIntervalMinutes
            out.Global.DefaultEnabled = gs.DefaultEnabled
            out.Global.DownloadDir = gs.DownloadDir
            // ... 更多字段
        }
        
        // 加载 qBittorrent 配置
        var qs models.QbitSettings
        if e := tx.First(&qs).Error; e == nil {
            out.Qbit.Enabled = qs.Enabled
            out.Qbit.URL = qs.URL
            out.Qbit.User = qs.User
            out.Qbit.Password = qs.Password
        }
        
        // 加载站点配置
        out.Sites = map[models.SiteGroup]models.SiteConfig{}
        var sites []models.SiteSetting
        if e := tx.Find(&sites).Error; e != nil {
            return e
        }
        for _, sitem := range sites {
            sg := models.SiteGroup(strings.ToLower(sitem.Name))
            sc := models.SiteConfig{
                Enabled: boolPtr(sitem.Enabled),
                AuthMethod: sitem.AuthMethod,
                Cookie: sitem.Cookie,
                APIKey: sitem.APIKey,
                RSS: []models.RSSConfig{},
            }
            
            // 加载 RSS 订阅
            var rss []models.RSSSubscription
            if e := tx.Where("site_id = ?", sitem.ID).Find(&rss).Error; e != nil {
                return e
            }
            for _, r := range rss {
                sc.RSS = append(sc.RSS, models.RSSConfig{
                    ID: r.ID,
                    Name: r.Name,
                    URL: r.URL,
                    Category: r.Category,
                    Tag: r.Tag,
                    IntervalMinutes: r.IntervalMinutes,
                })
            }
            out.Sites[sg] = sc
        }
        return nil
    }); err != nil {
        return nil, err
    }
    return &out, nil
}
```

**设计特点**:
- **事务加载**: 使用数据库事务确保配置一致性
- **分层加载**: 全局配置 → 下载器配置 → 站点配置 → RSS 订阅
- **默认值**: 配置不存在时提供合理的默认值
- **类型转换**: 数据库类型到运行时类型的转换

### 2. 配置热重载机制

**事件驱动架构**（[internal/events/bus.go](file:///home/incast/PT-Forward/examples/pt-tools/internal/events/bus.go#L1-L78)）:
```go
type EventType string

const (
    ConfigChanged EventType = "ConfigChanged"
    DiskSpaceLow  EventType = "DiskSpaceLow"
)

type Event struct {
    Type    EventType
    Version int64
    Source  string
    At      time.Time
}

var (
    mu   sync.RWMutex
    subs = map[string]chan Event{}
    sid  int64
)

func Subscribe(buffer int) (string, <-chan Event, func()) {
    if buffer <= 0 {
        buffer = 16
    }
    id := nextID()
    ch := make(chan Event, buffer)
    mu.Lock()
    subs[id] = ch
    mu.Unlock()
    cancel := func() {
        mu.Lock()
        if c, ok := subs[id]; ok {
            delete(subs, id)
            close(c)
        }
        mu.Unlock()
    }
    return id, ch, cancel
}

func Publish(e Event) {
    mu.RLock()
    defer mu.RUnlock()
    for _, ch := range subs {
        select {
        case ch <- e:
        default:
            // Channel full, skip
        }
    }
}
```

**调度器热重载**（[scheduler/manager.go](file:///home/incast/PT-Forward/examples/pt-tools/scheduler/manager.go#L1-L150)）:
```go
func NewManager() *Manager {
    m := &Manager{
        jobs:              map[string]*job{},
        downloaderManager: downloader.NewDownloaderManager(),
    }

    id, ch, cancel := events.Subscribe(64)
    _ = id
    m.eventCancel = cancel
    go func() {
        defer cancel()
        var pendingVersion int64
        var timer *time.Timer
        for e := range ch {
            if e.Type != events.ConfigChanged {
                continue
            }
            m.mu.Lock()
            if m.stopped {
                m.mu.Unlock()
                return
            }
            if e.Version <= m.lastVersion {
                m.mu.Unlock()
                continue
            }
            pendingVersion = e.Version
            m.mu.Unlock()

            // 防抖：200ms 内只处理最后一次配置变更
            if timer == nil {
                timer = time.NewTimer(200 * time.Millisecond)
            } else {
                if !timer.Stop() {
                    <-timer.C
                }
                timer.Reset(200 * time.Millisecond)
            }
            <-timer.C

            // 重新加载配置
            db := global.GlobalDB
            if db == nil {
                continue
            }
            cfg, _ := core.NewConfigStore(db).Load()
            if cfg != nil {
                m.Reload(cfg)
                m.mu.Lock()
                m.lastVersion = pendingVersion
                m.mu.Unlock()
            }
        }
    }()
    return m
}
```

**热重载流程**:
1. **配置变更**: 用户修改配置后，`ConfigStore.SaveGlobal()` 发布 `ConfigChanged` 事件
2. **事件订阅**: 调度器在初始化时订阅事件总线
3. **防抖处理**: 200ms 内多次变更只处理最后一次
4. **配置重载**: 从数据库重新加载配置
5. **任务重启**: 停止旧任务，启动新任务

**配置保存示例**:
```go
func (s *ConfigStore) SaveGlobal(gl models.SettingsGlobal) error {
    db := s.db.DB
    var gs models.SettingsGlobal
    if err := db.First(&gs).Error; err != nil {
        gs = models.SettingsGlobal{}
    }
    
    // 更新字段
    gs.DefaultIntervalMinutes = gl.DefaultIntervalMinutes
    gs.DefaultEnabled = gl.DefaultEnabled
    gs.DownloadDir = gl.DownloadDir
    // ... 更多字段
    
    // 保存到数据库
    if err := db.Save(&gs).Error; err != nil {
        return err
    }
    
    // 发布配置变更事件
    events.Publish(events.Event{
        Type:    events.ConfigChanged,
        Version: time.Now().UnixNano(),
        Source:  "global",
        At:      time.Now(),
    })
    
    return nil
}
```

## 日志系统与监控机制

### 1. 日志系统架构

**全局日志配置**（[config/zap.go](file:///home/incast/PT-Forward/examples/pt-tools/config/zap.go#L1-L190)）:
```go
var DefaultZapConfig = Zap{
    Directory:     "logs",
    MaxSize:       10,
    MaxAge:        30,
    MaxBackups:    10,
    Compress:      true,
    Level:         "info",
    Format:        "json",
    ShowLine:      false,
    EncodeLevel:   "CapitalColorLevelEncoder",
    StacktraceKey: "",
    LogInConsole:  true,
}

var AtomicLogLevel zap.AtomicLevel

func init() {
    AtomicLogLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)
}

func (z *Zap) InitLogger() (*zap.Logger, error) {
    homeDir, _ := os.UserHomeDir()
    zapPath := filepath.Join(homeDir, models.WorkDir, z.Directory)
    
    // 创建日志目录
    if err := os.MkdirAll(zapPath, os.ModePerm); err != nil {
        return nil, fmt.Errorf("创建日志目录失败: %w", err)
    }
    
    // 设置动态日志级别
    var level zapcore.Level
    if err := level.UnmarshalText([]byte(z.Level)); err != nil {
        return nil, fmt.Errorf("解析日志级别失败: %w", err)
    }
    AtomicLogLevel.SetLevel(level)
    
    // 配置编码器
    encCfg := zapcore.EncoderConfig{
        TimeKey:        "time",
        LevelKey:       "level",
        NameKey:        "logger",
        CallerKey:      "caller",
        MessageKey:     "msg",
        StacktraceKey:  z.StacktraceKey,
        LineEnding:     zap.DefaultLineEnding,
        EncodeLevel:    zapcore.LowercaseLevelEncoder,
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeDuration: zapcore.SecondsDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    }
    fileEncoder := zapcore.NewJSONEncoder(encCfg)
    
    // 配置日志轮转
    allWriter := zapcore.AddSync(&lumberjack.Logger{
        Filename:   filepath.Join(zapPath, "all.log"),
        MaxSize:    z.MaxSize,
        MaxBackups: z.MaxBackups,
        MaxAge:     z.MaxAge,
        Compress:   z.Compress,
    })
    debugWriter := zapcore.AddSync(&lumberjack.Logger{
        Filename:   filepath.Join(zapPath, "debug.log"),
        MaxSize:    z.MaxSize,
        MaxBackups: z.MaxBackups,
        MaxAge:     z.MaxAge,
        Compress:   z.Compress,
    })
    infoWriter := zapcore.AddSync(&lumberjack.Logger{
        Filename:   filepath.Join(zapPath, "info.log"),
        MaxSize:    z.MaxSize,
        MaxBackups: z.MaxBackups,
        MaxAge:     z.MaxAge,
        Compress:   z.Compress,
    })
    errorWriter := zapcore.AddSync(&lumberjack.Logger{
        Filename:   filepath.Join(zapPath, "error.log"),
        MaxSize:    z.MaxSize,
        MaxBackups: z.MaxBackups,
        MaxAge:     z.MaxAge,
        Compress:   z.Compress,
    })
    
    // 配置日志级别过滤器
    allPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
        return lvl >= AtomicLogLevel.Level()
    })
    debugPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
        return lvl == zapcore.DebugLevel && lvl >= AtomicLogLevel.Level()
    })
    highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
        return lvl >= zapcore.ErrorLevel && lvl >= AtomicLogLevel.Level()
    })
    lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
        return lvl > zapcore.DebugLevel && lvl < zapcore.ErrorLevel && lvl >= AtomicLogLevel.Level()
    })
    
    // 组合多个 Core
    cores := []zapcore.Core{
        zapcore.NewCore(fileEncoder, allWriter, allPriority),
        zapcore.NewCore(fileEncoder, debugWriter, debugPriority),
        zapcore.NewCore(fileEncoder, infoWriter, lowPriority),
        zapcore.NewCore(fileEncoder, errorWriter, highPriority),
    }
    
    // 添加控制台输出
    if z.LogInConsole {
        consoleCfg := zapcore.EncoderConfig{
            TimeKey:        "time",
            LevelKey:       "level",
            NameKey:        "logger",
            CallerKey:      "caller",
            MessageKey:     "msg",
            StacktraceKey:  z.StacktraceKey,
            LineEnding:     zap.DefaultLineEnding,
            EncodeLevel:    zapcore.CapitalColorLevelEncoder,
            EncodeTime:     zapcore.ISO8601TimeEncoder,
            EncodeDuration: zapcore.SecondsDurationEncoder,
            EncodeCaller:   zapcore.ShortCallerEncoder,
        }
        consoleEncoder := zapcore.NewConsoleEncoder(consoleCfg)
        consoleCore := zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), AtomicLogLevel)
        cores = append(cores, consoleCore)
    }
    
    core := zapcore.NewTee(cores...)
    options := []zap.Option{}
    if z.ShowLine {
        options = append(options, zap.AddCaller())
    }
    if z.StacktraceKey != "" {
        options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
    }
    logger := zap.New(core, options...)
    return logger, nil
}
```

**日志特性**:
- **多文件输出**: all.log、debug.log、info.log、error.log
- **日志轮转**: 按大小（10MB）和时间（30天）轮转
- **压缩备份**: 自动压缩旧日志文件
- **动态级别**: 运行时调整日志级别
- **控制台输出**: 支持彩色控制台输出
- **结构化日志**: JSON 格式，便于日志分析

### 2. 动态日志级别

**全局日志管理**（[global/global.go](file:///home/incast/PT-Forward/examples/pt-tools/global/global.go#L1-L102)）:
```go
var (
    GlobalLogger *zap.Logger
    GlobalDB     *models.TorrentDB
    once         sync.Once
    logLevel     atomic.Value
)

type LogLevel string

const (
    LogLevelDebug LogLevel = "debug"
    LogLevelInfo  LogLevel = "info"
    LogLevelWarn  LogLevel = "warn"
    LogLevelError LogLevel = "error"
)

func GetLogLevel() LogLevel {
    if v := logLevel.Load(); v != nil {
        return v.(LogLevel)
    }
    return LogLevelInfo
}

func SetLogLevel(level LogLevel) {
    logLevel.Store(level)
}

func IsDebugEnabled() bool {
    return GetLogLevel() == LogLevelDebug
}

func (l LogLevel) ToZapLevel() zapcore.Level {
    switch l {
    case LogLevelDebug:
        return zapcore.DebugLevel
    case LogLevelInfo:
        return zapcore.InfoLevel
    case LogLevelWarn:
        return zapcore.WarnLevel
    case LogLevelError:
        return zapcore.ErrorLevel
    default:
        return zapcore.InfoLevel
    }
}
```

**运行时调整日志级别**:
```go
func SetLogLevel(level string) error {
    var zapLevel zapcore.Level
    if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
        return fmt.Errorf("无效的日志级别: %s", level)
    }
    AtomicLogLevel.SetLevel(zapLevel)
    return nil
}

func GetLogLevel() string {
    return AtomicLogLevel.Level().String()
}
```

**日志级别调整 API**:
```go
// 日志级别相关类型
type LogLevelResponse struct {
    Level   string   `json:"level"`
    Levels  []string `json:"levels"`
    Message string   `json:"message,omitempty"`
}

// 日志级别 API
export const logLevelApi = {
    get: () => api.get<LogLevelResponse>("/api/log-level"),
    set: (level: string) =>
        request<LogLevelResponse>("/api/log-level", {
            method: "PUT",
            body: JSON.stringify({ level }),
        }),
};
```

## 错误处理与异常管理

### 1. 错误包装模式

**HTTP 客户端错误处理**（[site/v2/http_client.go](file:///home/incast/PT-Forward/examples/pt-tools/site/v2/http_client.go#L1-L100)）:
```go
func (c *SiteHTTPClient) Do(ctx context.Context, method, url string, options ...requests.Option) (*requests.Response, error) {
    resp, err := c.session.Do(ctx, method, url, options...)
    if err != nil {
        return nil, fmt.Errorf("HTTP 请求失败: %w", err)
    }
    
    if resp.StatusCode >= 400 {
        body, _ := resp.Text()
        return nil, fmt.Errorf("HTTP 错误 %d: %s", resp.StatusCode, body)
    }
    
    return resp, nil
}
```

**错误链追踪**:
```go
// 使用 fmt.Errorf 的 %w 动词包装错误
if err := db.First(&gs).Error; err != nil {
    return nil, fmt.Errorf("加载全局配置失败: %w", err)
}

// 使用 errors.Is 检查错误类型
if errors.Is(err, gorm.ErrRecordNotFound) {
    // 处理记录不存在的情况
}

// 使用 errors.As 获取具体错误类型
var dbErr *gorm.Error
if errors.As(err, &dbErr) {
    // 处理数据库特定错误
}
```

### 2. 站点驱动错误处理

**统一错误处理**:
```go
func (d *NexusPHPDriver) Search(ctx context.Context, query SearchQuery) ([]TorrentItem, error) {
    if d.httpClient == nil {
        return nil, fmt.Errorf("HTTP 客户端未初始化")
    }
    
    // 构建搜索 URL
    searchURL, err := d.buildSearchURL(query)
    if err != nil {
        return nil, fmt.Errorf("构建搜索 URL 失败: %w", err)
    }
    
    // 发送请求
    resp, err := d.httpClient.Get(ctx, searchURL)
    if err != nil {
        return nil, fmt.Errorf("搜索请求失败: %w", err)
    }
    defer resp.Body.Close()
    
    // 检查响应状态
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("搜索失败: HTTP %d", resp.StatusCode)
    }
    
    // 解析 HTML
    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("解析 HTML 失败: %w", err)
    }
    
    // 解析种子列表
    items, err := d.parseTorrentList(doc)
    if err != nil {
        return nil, fmt.Errorf("解析种子列表失败: %w", err)
    }
    
    return items, nil
}
```

### 3. 前端错误处理

**API 错误处理**（[api/index.ts](file:///home/incast/PT-Forward/examples/pt-tools/web/frontend/src/api/index.ts#L1-L50)）:
```typescript
class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  });

  const contentType = response.headers.get("content-type") || "";
  const isJSON = contentType.includes("application/json");

  if (!response.ok) {
    const msg = await response.text();
    throw new ApiError(response.status, msg || `HTTP ${response.status}`);
  }

  if (isJSON) {
    return response.json() as Promise<T>;
  }
  return response.text() as unknown as T;
}
```

**组件内错误处理**:
```typescript
// 在组件中使用 try-catch
async function loadData() {
  loading.value = true;
  error.value = null;
  
  try {
    const data = await api.get<SomeType>("/api/endpoint");
    items.value = data;
  } catch (e) {
    if (e instanceof ApiError) {
      error.value = `请求失败 (${e.status}): ${e.message}`;
    } else {
      error.value = "未知错误";
    }
  } finally {
    loading.value = false;
  }
}
```

## 代码质量工具与开发规范

### 1. Go 代码质量工具

**golangci-lint 配置**（[.golangci.yml](file:///home/incast/PT-Forward/examples/pt-tools/.golangci.yml#L1-L122)）:
```yaml
version: "2"

run:
  timeout: 5m
  modules-download-mode: readonly

formatters:
  enable:
    - gofumpt
    - goimports
  settings:
    gofumpt:
      extra-rules: true
    goimports:
      local-prefixes:
        - github.com/sunerpy/pt-tools

linters:
  default: none
  enable:
    - errcheck      # 检查未处理的错误
    - govet         # Go vet 静态分析
    - ineffassign   # 检查无效赋值
    - staticcheck   # 静态检查
    - unused        # 检查未使用的变量
    - misspell      # 拼写检查
    - unconvert     # 检查不必要的类型转换
    - bodyclose     # 检查未关闭的 HTTP 响应体
    - noctx         # 检查未使用 context 的 HTTP 请求

  settings:
    errcheck:
      check-type-assertions: false
      check-blank: false

    govet:
      disable:
        - fieldalignment
      enable:
        - shadow

    misspell:
      locale: US

  exclusions:
    generated: lax
    presets:
      - comments
      - std-error-handling
    rules:
      # 测试文件排除规则
      - path: _test\.go
        linters:
          - errcheck
          - bodyclose
          - noctx
          - unused
```

**启用的 Linters**:
- **errcheck**: 检查未处理的错误
- **govet**: Go 官方静态分析工具
- **ineffassign**: 检查无效的赋值
- **staticcheck**: 高级静态检查
- **unused**: 检查未使用的变量、函数等
- **misspell**: 检查拼写错误（美式英语）
- **unconvert**: 检查不必要的类型转换
- **bodyclose**: 检查 HTTP 响应体是否关闭
- **noctx**: 检查 HTTP 请求是否使用 context

### 2. Makefile 自动化

**构建自动化**（[Makefile](file:///home/incast/PT-Forward/examples/pt-tools/Makefile#L1-L328)）:
```makefile
# 本地构建
build-local: fmt build-frontend
	@echo "Building binary for local environment"
	mkdir -p $(DIST_DIR) && \
	GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) CGO_ENABLED=0 \
	go build -ldflags="-s -w \
	-X github.com/sunerpy/pt-tools/version.Version=$(TAG) \
	-X github.com/sunerpy/pt-tools/version.BuildTime=$(BUILD_TIME) \
	-X github.com/sunerpy/pt-tools/version.CommitID=$(COMMIT_ID)" \
	-o $(DIST_DIR)/$(IMAGE_NAME) .

# 多平台构建
build-binaries:
	@echo "Building binaries for platforms: $(PLATFORMS)"
	for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		OUTPUT=$(DIST_DIR)/$(IMAGE_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT=$$OUTPUT.exe; fi; \
		echo "Building for $$platform -> $$OUTPUT"; \
		GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 go build -ldflags="-s -w" \
		-o $$OUTPUT . || exit 1; \
	done

# 代码格式化
fmt: fmt-oxfmt fmt-go
	@echo "Formatting complete."

fmt-oxfmt:
	@echo "Formatting with oxfmt..."
	@cd web/frontend && if [ ! -d "node_modules" ]; then \
		pnpm install; \
	fi && \
	pnpm oxfmt --no-error-on-unmatched-pattern "$(PROJECT_ROOT)"

fmt-go:
	@echo "Formatting Go code..."
	@if command -v goimports > /dev/null 2>&1; then \
		echo "$(GO_FILES)" | tr ' ' '\n' | xargs -P 4 goimports -w -local github.com/sunerpy/pt-tools; \
	fi
	@if command -v gofumpt > /dev/null 2>&1; then \
		echo "$(GO_FILES)" | tr ' ' '\n' | xargs -P 4 gofumpt -extra -w; \
	fi

# 代码检查
lint: lint-go lint-frontend

lint-go:
	@echo "Running Go linters..."
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Running go vet instead..."; \
		go vet ./...; \
	fi

lint-frontend:
	@echo "Running frontend linters with oxlint..."
	@cd web/frontend && \
	pnpm lint:check && \
	echo "" && \
	echo "Running Vue type check..." && \
	pnpm vue-tsc --noEmit

# 单元测试
unit-test:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=1 go test ./... -count=1 -race -cover -covermode=atomic -coverprofile=$(DIST_DIR)/coverage.out
	go tool cover -html=$(DIST_DIR)/coverage.out -o $(DIST_DIR)/coverage.html
	@echo "Coverage report: $(DIST_DIR)/coverage.html"

# 覆盖率汇总
coverage-summary: unit-test
	@mkdir -p $(DIST_DIR)
	@test -f $(DIST_DIR)/coverage.out || (echo "Run make unit-test first"; exit 1)
	@echo "Filtering coverage data (excluding test files and mocks)..."
	@grep -v "_test.go" $(DIST_DIR)/coverage.out | grep -v "/mocks/" > $(DIST_DIR)/filtered_coverage.out
	@echo ""
	@echo "=== Coverage Summary (excluding test and mock files) ==="
	@go tool cover -func=$(DIST_DIR)/filtered_coverage.out | tee $(DIST_DIR)/coverage.txt

# 本地 CI 测试
ci-local:
	@if command -v act > /dev/null 2>&1; then \
		if [ -n "$(ACT_IMAGE)" ]; then \
			act push --container-architecture linux/amd64 -W .github/workflows/ci.yml \
			-P ubuntu-latest=$(ACT_IMAGE) $(ACT_ENV_ARGS) $(ACT_CONTAINER_OPTS); \
		else \
			act push --container-architecture linux/amd64 -W .github/workflows/ci.yml \
			$(ACT_ENV_ARGS) $(ACT_CONTAINER_OPTS); \
		fi; \
	else \
		echo "act not found. Install with:"; \
		echo "  brew install act  # macOS"; \
		echo "  curl -s https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash  # Linux"; \
		exit 1; \
	fi
```

**常用命令**:
- `make build-local`: 本地构建二进制
- `make build-binaries`: 多平台构建（linux/amd64, linux/arm64, windows/amd64, windows/arm64）
- `make fmt`: 格式化代码（Go + 前端）
- `make lint`: 代码检查（Go + 前端）
- `make unit-test`: 运行单元测试（带覆盖率）
- `make coverage-summary`: 生成覆盖率报告
- `make ci-local`: 本地运行 CI（使用 act）

### 3. 代码规范

**Go 代码规范**:
- 使用 `goimports` 自动整理导入
- 使用 `gofumpt` 格式化代码（比 gofmt 更严格）
- 错误处理必须检查，不能忽略
- 使用 `%w` 动词包装错误，保留错误链
- 测试文件使用 `*_test.go` 命名
- 测试函数使用 `TestXxx` 命名

**前端代码规范**:
- 使用 TypeScript 类型注解
- 使用 `oxlint` 进行代码检查
- 使用 `oxfmt` 进行代码格式化
- 组件文件使用 PascalCase 命名
- 组合式函数使用 `useXxx` 命名
- 状态存储使用 `useXxxStore` 命名

**提交规范**:
- 使用 Conventional Commits 规范
- 格式：`<type>(<scope>): <subject>`
- 类型：feat, fix, docs, style, refactor, test, chore
- 示例：`feat(search): add multi-site search feature`

## 总结

pt-tools 是一个设计精良、功能完善的企业级 PT 站点管理平台，展示了现代全栈开发的最佳实践：

**后端**:
- Go 语言的高性能和并发优势
- 优秀的架构设计（驱动模式、工厂模式、事件驱动）
- 完善的测试覆盖（134 个测试文件）
- 高效的数据库优化（WAL 模式、连接池）
- 强大的容错机制（熔断器、故障转移、限流）
- 精心设计的算法（去重、排序、标准化）

**前端**:
- Vue 3 组合式 API 的现代化开发体验
- TypeScript 的类型安全
- Element Plus 的丰富组件
- 响应式设计和用户体验

**运维**:
- Docker 容器化部署
- 多平台支持
- 自动化构建和测试
- 完善的监控和日志
- CI/CD 自动化发布

**浏览器扩展**:
- Manifest V3 架构
- Cookie 同步
- 数据采集和脱敏
- GitHub 集成

**安全**:
- 密码哈希存储
- 会话管理
- CORS 保护
- 敏感信息脱敏

这个项目不仅是 PT 管理的优秀工具，也是学习全栈开发的绝佳范例，值得深入研究和参考。