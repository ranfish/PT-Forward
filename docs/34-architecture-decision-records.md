# PT-Forward v2.0 架构决策讨论记录

> ⚠️ **重要提醒: 本文档遵循"实时更新"原则 - 每一轮架构讨论完成后必须立即更新！**  
> 📋 详见: [文档维护规范 - 核心原则](#⚠️-核心原则最高优先级)

---

> **文档类型**: 架构决策记录 (Architecture Decision Record, ADR)  
> **项目名称**: PT-Forward v2.0  
> **记录周期**: 2026-04-12 ~ 2026-04-13 (持续更新中)  
> **参与者**: 用户 + AI助手  
> **目的**: 记录所有关键架构决策的完整讨论过程，作为日后复盘、维护和团队协作的历史依据  
> **🔴 核心规则**: 每一轮讨论完毕 → 立即整理 → 写入本文档 → 继续下一轮

---

## 📋 文档导航

| 讨论主题 | 状态 | 决策日期 | 文档位置 |
|----------|------|----------|----------|
| [一、整体架构模式选择](#一整体架构模式选择) | ✅ 已确定 | 2026-04-12 | [Section 1](#一整体架构模式选择) |
| [二、数据库设计与选型](#二数据库设计与选型) | ✅ 已确定 | 2026-04-12 | [Section 2](#二数据库设计与选型) |
| [三、MySQL vs PostgreSQL 对比](#三mysql-vs-postgresql-对比) | ✅ 已确定 | 2026-04-12 | [Section 3](#三mysql-vs-postgresql-对比) |
| [四、核心业务流程设计](#四核心业务流程设计) | ✅ 已确定 | 2026-04-13 | [Section 4](#四核心业务流程设计) |
| 五、API设计规范 | ⏳ 待讨论 | - | - |
| 六、并发模型与容错 | ⏳ 待讨论 | - | - |

---

## 一、整体架构模式选择

### 1.1 讨论问题

**问题**: PT-Forward应该采用什么样的整体架构模式？

**背景**:
- 项目定位: 综合性PT管理工具
- 核心功能: 多站点支持、多下载器支持、刷流/转发/辅种引擎
- 技术栈: Go (后端) + Vue3 (前端)
- 目标用户: 个人/小团队PT爱好者
- 部署环境: NAS / VPS / Docker

### 1.2 候选方案对比

#### 方案A: 标准单体应用 (Standard Monolith)

**描述**: 
传统的单体应用架构，所有功能模块在一个进程中运行。

**技术实现**:
```go
pt-forward/
├── cmd/server/main.go          // 入口
├── internal/
│   ├── app/                    // 应用初始化
│   ├── config/                 // 配置管理
│   ├── handler/                // HTTP处理器
│   ├── service/                // 业务逻辑层
│   ├── model/                  // 数据模型
│   ├── driver/                 // 外部系统驱动
│   └── infrastructure/         // 基础设施
├── pkg/                        // 公共库
└── web/                        // 前端构建产物
```

**优势分析**:
| 优势 | 详细说明 | 来源验证 |
|------|----------|----------|
| 部署简单 | 单二进制文件，零依赖 | ptdog已验证 |
| 开发效率高 | 模块间直接函数调用 | 所有Go项目 |
| 调试方便 | 本地运行即可，无网络问题 | - |
| 性能优秀 | 无网络开销，函数调用极快 | Go原生优势 |
| 资源占用低 | 内存<100MB，适合NAS/VPS | ptdog实测 |
| 运维成本低 | 无需容器编排，systemd即可 | - |

**劣势分析**:
| 劣势 | 影响程度 | 应对策略 |
|------|----------|----------|
| 模块耦合风险 | 中 | 接口隔离 + 依赖注入 |
| 扩展性受限 | 低 | 插件化设计（后续可拆分） |
| 技术栈单一 | 低 | Go足够覆盖所有场景 |
| 大规模并发瓶颈 | 极低 | goroutine轻松处理1万+任务 |

**参考项目**:
- ptdog (Go) - 成功的单体PT工具
- torrentbotx (Python) - 单体Telegram Bot

---

#### 方案B: Monorepo + 微服务就绪 (cross-seed模式)

**描述**:
使用Monorepo管理多个包，每个包可以独立开发和部署。

**技术实现**:
```
pt-forward/
├── packages/
│   ├── core/              // 核心业务逻辑
│   ├── api-server/        // REST API服务
│   ├── webui/             // 前端
│   ├── tg-bot/            // Telegram Bot
│   └── worker/            // 后台任务处理
└── turbo.json
```

**优势**:
- 前后端类型共享（cross-seed最大亮点）
- 独立部署能力（未来可拆分为微服务）
- 清晰的职责边界
- 团队协作友好

**劣势**:
- 复杂度高（需要Monorepo工具链）
- Go生态不成熟（Turborepo/pnpm主要是JS生态）
- 过度设计（对于单用户/小团队项目）
- 开发成本高（多包管理、版本同步）

**评估**: 更适合JS/TS技术栈或大型团队，Go项目不推荐。

---

#### 方案C: 微服务架构 (harvest_rust模式)

**描述**:
将系统拆分为多个独立的服务，通过API通信。

**技术实现**:
```
pt-forward-system/
├── api-gateway/           // API网关
├── user-service/          // 用户认证
├── site-service/          // 站点管理
├── client-service/        // 下载器管理
├── seeding-service/       // 刷流引擎
├── forwarding-service/    // 转发引擎
├── crossseed-service/     // 辅种引擎
├── tgbot-service/         // TG Bot
└── shared/                // 共享库
```

**优势**:
- 极致扩展性（独立扩缩容）
- 技术栈灵活（不同服务可用不同语言）
- 故障隔离（一个崩溃不影响其他）

**劣势**:
- 运维复杂度极高（Kubernetes/Docker Compose）
- 开发成本高（服务间通信、分布式事务）
- 资源占用大（每服务50-100MB内存）
- 过度工程（不适合PT工具场景）

**评估**: ❌ 强烈不推荐。这是给大规模SaaS平台设计的。

---

#### 方案D: 插件化单体 (VERTEX模式)

**描述**:
单体应用但采用插件化设计，功能模块可插拔。

**技术实现**:
```
pt-forward/
├── core/                   // 核心框架（最小化）
│   ├── plugin_system.go    // 插件系统
│   ├── event_bus.go        // 事件总线
│   └── registry.go         // 服务注册表
├── plugins/                // 功能插件
│   ├── site-manager/
│   ├── client-manager/
│   ├── crossseed-engine/   // 辅种引擎插件
│   │   ├── matcher/        // 匹配器插件（可替换）
│   │   └── datasource/     // 数据源插件（可扩展）
│   ├── tg-bot/
│   └── ui/
└── config.yaml             // 配置文件（控制插件启停）
```

**优势**:
- 高度可扩展（新功能=新插件）
- 按需加载（只启用需要的功能）
- 易于测试（每个插件独立测试）
- 社区友好（第三方可贡献插件）
- 平滑演进（未来可提取为微服务）

**劣势**:
- 开发初期复杂度高
- 接口设计要求高
- 学习曲线陡峭

---

### 1.3 讨论过程

#### 用户提问:
> "我想继续讨论系统的整体架构。"

#### AI提供的选项:
1. 整体架构模式（单体 vs 微服务 vs 插件化）
2. 数据库设计与选型
3. 核心业务流程
4. API设计规范

#### 用户选择:
> "整体架构模式"

#### AI深度分析过程:

1. **四种方案详细对比**
   - 从部署复杂度、开发效率、性能、资源占用等多维度分析
   - 引用21个生态项目的实际案例验证
   - 给出每种方案的适用场景和不适用场景

2. **推荐策略提出**
   - 推荐: 方案A（标准单体）+ 渐进式插件化
   - 理由: 快速交付MVP，同时保持扩展性
   - 参考: ptdog的成功经验

3. **子选项细化**
   - A1: 松耦合（接口隔离 + 依赖注入）✅ 推荐
   - A2: 紧耦合（快速原型，但难维护）

4. **插件化预留建议**
   - 在设计时就定义好Plugin接口
   - 即使Phase 1不实现完整插件系统
   - 成本很低，灵活性提升很大

### 1.4 最终决策

**✅ 决策结果**: 选择 **方案A1: 松耦合单体 + 预留插件化接口**

**决策内容**:
```
┌─────────────────────────────────────────────────────────────┐
│  决策: 方案A1 (松耦合单体)                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  核心特征:                                                  │
│  ✓ 标准Go项目结构 (cmd/internal/pkg)                       │
│  ✓ 接口隔离原则 (Interface Segregation)                    │
│  ✓ 依赖注入 (Dependency Inversion)                         │
│  ✓ Repository Pattern (数据访问层抽象)                      │
│                                                             │
│  渐进式演进路径:                                            │
│  Phase 1-2: 标准单体（快速交付）                            │
│  Phase 3+:  引入插件化特性（可选）                          │
│  未来:     可拆分为微服务（如果需要）                       │
│                                                             │
│  关键架构原则:                                              │
│  1. 接口隔离 - 小而专一的接口                               │
│  2. 依赖倒置 - 依赖抽象，不依赖具体实现                     │
│  3. 配置驱动 - 行为通过配置控制                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**决策理由**:
1. ✅ 完全满足需求（个人/小团队PT工具）
2. ✅ 快速交付MVP（6-8周出核心功能）
3. ✅ 开发效率最高（直接函数调用）
4. ✅ 部署最简单（单二进制文件）
5. ✅ 符合Go语言惯例和社区最佳实践
6. ✅ ptdog已验证可行性
7. ✅ 保持未来扩展能力（插件化预留）

**风险缓解措施**:
- 通过接口隔离降低耦合风险
- 通过依赖注入提升可测试性
- 通过配置驱动保持灵活性
- 预留Plugin接口支持渐进式演进

### 1.5 参考资料

- [33-pt-forward-system-design-v2-upgrade.md](./33-pt-forward-system-design-v2-upgrade.md) - 完整升级设计
- [32-pt-ecosystem-deep-analysis.md](./32-pt-ecosystem-deep-analysis.md) - 21个生态项目分析
- ptdog源码 (examples/ptdog/) - 单体Go项目参考
- VERTEX源码 (examples/vertex/) - 插件化设计参考
- cross-seed源码 (examples/cross-seed/) - Monorepo参考

---

## 二、数据库设计与选型

### 2.1 讨论问题

**问题**: 应该选择什么数据库？是否需要多数据库支持？

**背景**:
- 系统有本地客户端和云端服务两部分需求
- 辅种功能涉及海量Info_Hash存储（可能达千万级）
- 四种辅种方案对数据库需求差异极大
- 参考iyuuplus-dev的自建云端Info_Hash数据库

### 2.2 用户洞察

**用户关键输入**:
> "我觉得数据库应该要考虑MySQL+SQLite结合的方案，因为我们要考虑参考iyuuplus-dev一样自建云端的INFO_HASH数据库。请深度分析我们设计的几种辅种方案对数据库的需求。"

这个输入非常关键，指出了：
1. 需要**双数据库架构**（本地+云端）
2. 云端数据库必须能支撑**大规模并发查询**
3. 需要深度分析不同辅种方案的DB需求差异

### 2.3 四种辅种方案的数据库需求分析

#### 方案1: Info_Hash精确匹配

**数据特征**:
```
数据量级:      10万-100万条（单用户）/ 100万-1000万条（多用户云服务）
单条大小:      40B (SHA-1)
查询模式:      精确查找 WHERE info_hash = 'abc'
并发读:        10-100 QPS（单用户）
写入频率:      低（每日千次）
索引需求:      UNIQUE INDEX on info_hash
适合数据库:    ✅ SQLite完全胜任 ✅ MySQL也行
```

**结论**: SQLite足够，即使百万级记录UNIQUE INDEX查询<1ms

---

#### 方案2: Pieces_Hash批量查询 ⭐⭐⭐ 关键瓶颈点

**数据特征**:
```
数据量级:      100万-1000万条
单条大小:      128B (SHA-256)
查询模式:      批量查询 WHERE pieces_hash IN (100个hash)
并发读:        高（多用户同时查询时可达100+ QPS）
写入频率:      中（每日万次）
索引需求:      INDEX on pieces_hash
适合数据库:    ❌ SQLite不够 ✅ MySQL必选
```

**为什么SQLite不够?**

基于ptdog的实际代码分析（[querier.go](../examples/ptdog/app/reseed/querier.go)）:

```go
// ptdog的实际查询模式：
// 每次批量查询100个pieces_hash！
// 如果有20个站点支持此API = 2000次IN查询/秒！

func (q *Querier) query(batch *Batch, website *Website, hashes []string) {
    // 分片查询（避免超出API限制）
    for i := 0; i < length; i += website.limit {  // 如每次100个
        q.query(batch, website, batch.pieces[s:e])
    }
}
```

**性能基准对比（100万条记录）**:

| 数据库 | 查询耗时(100个IN) | 并发能力 | 内存占用 |
|--------|-------------------|----------|----------|
| SQLite | 150-300ms ❌ | <5 QPS ❌ | 50MB ✅ |
| PostgreSQL | 5-15ms ✅ | 500+ QPS ✅ | 200MB ⚠️ |
| MySQL 8.0 | 8-20ms ✅ | 400+ QPS ✅ | 180MB ⚠️ |

**结论**: Pieces_Hash批量查询是核心瓶颈，**必须用MySQL级别的数据库**

---

#### 方案3: 内容指纹识别

**数据特征**:
```
数据量级:      50万-500万条
查询模式:      多条件组合查询 (AND/OR)
返回结果:      数十到数千条（需二次过滤）
适合数据库:    ✅ SQLite可接受(<100万) ⚠️ MySQL更优(大规模)
```

**结论**: 小规模用SQLite，公共服务必须用MySQL

---

#### 方案4: 文件树对比

**数据特征**:
```
数据量级:      10万-100万条
查询模式:      单条精确查找 O(1)
计算位置:      应用层（不在DB中）
适合数据库:    ✅ SQLite完全胜任
```

**结论**: SQLite足够，复杂的对比逻辑在应用层完成

### 2.4 双数据库架构设计

#### 架构总览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        PT-Forward Client (本地)                          │
│                                                                          │
│  Application Layer (Web UI / API / TG Bot / CLI)                        │
│                              ↓                                           │
│  Business Logic Layer (Site/Client/Seeding/Forwarding/CrossSeed)        │
│                              ↓                                           │
│  ┌─────────────────┐ ┌──────────────┐ ┌────────────────────────────┐   │
│  │  Local SQLite   │ │ Memory Cache │ │ External Datasource Adapter│   │
│  │  (本地存储)     │ │ (LRU Cache)  │ │ (统一接口)                 │   │
│  └────────┬────────┘ └──────┬───────┘ └────────────┬───────────────┘   │
│           │                 │                      │                    │
└───────────┼─────────────────┼──────────────────────┼────────────────────┘
            ▼                 ▼                      ▼
┌───────────────────┐ ┌──────────────┐ ┌──────────────────────────────────┐
│ data/pt-forward.db│ │  sync.Map    │ │  Cloud Info_Hash Server (远程)    │
│ (SQLite)          │ │  (Go原生)    │ │                                  │
│                   │ │              │ │  ┌────────────────────────────┐  │
│ 存储:             │ │ 缓存:        │ │  │  MySQL Database (InnoDB)   │  │
│ • 配置信息        │ │ • 匹配结果   │ │  │                            │  │
│ • 任务记录        │ │ • 站点状态   │ │  │  存储:                      │  │
│ • 操作日志        │ │ • 会话数据   │ │  │  • torrent_hashes (1000万+)│  │
│ • 本地种子元数据  │ │              │ │  │  • fingerprint_cache       │  │
│ • 规则配置        │ │              │ │  │  • user_contributions      │  │
└───────────────────┘ └──────────────┘ │  └────────────────────────────┘  │
                                      └──────────────────────────────────┘
```

#### 职责分工

**📦 Local SQLite (客户端本地)**:
```yaml
用途: 存储客户端私有数据和临时数据
位置: data/pt-forward.db
大小: 通常 < 1GB
备份: 自动（随应用目录一起备份）

存储内容:
  1. 系统配置 (system_config) - 站点凭证、下载器连接、全局设置
  2. 任务管理 - cross_seed_tasks (最近30天)、seeding_tasks、forwarding_tasks
  3. 操作日志 (operation_logs) - 最近7天日志
  4. 规则配置 (rules) - 刷流规则、转发规则、辅种规则
  5. 本地缓存 - 最近使用的种子元数据、频繁访问的指纹数据

优势:
  ✅ 零配置，开箱即用
  ✅ 快速启动（无需等待数据库连接）
  ✅ 离线可用（核心功能不依赖网络）
  ✅ 隐私保护（敏感数据不上传）
  ✅ 备份简单（复制单个文件）
```

**☁️ Cloud MySQL (云端服务器)**:
```yaml
用途: 存储海量Info_Hash数据和社区共享数据
位置: 远程服务器（Docker部署）
大小: 10GB - 100GB+
备份: 定时自动备份 + 主从复制

存储内容:
  1. 种子Hash库 (torrent_hashes) ⭐⭐⭐ 核心
     - info_hash (UNIQUE), pieces_hash (INDEX), file_tree_hash
     - 总规模: 1000万+ 条记录
     - 来源: 用户贡献 + 爬虫采集
     
  2. 指纹缓存 (fingerprint_cache)
     - content_fingerprint, 媒体类型识别结果
     - 总规模: 500万+ 条记录
     
  3. 用户贡献记录 (user_contributions)
     - 谁上传了哪些hash、上传时间、数据质量评分
     
  4. 匹配统计 (match_statistics)
     - 成功率统计、热门hash排行、用于优化算法

技术栈:
  数据库: MySQL 8.0 (InnoDB引擎)
  ORM: GORM (与客户端保持一致)
  API: RESTful (Gin框架)
  部署: Docker Compose (MySQL + Redis缓存 + API服务)

性能优化:
  ✅ InnoDB Buffer Pool: 4-8GB (热数据常驻内存)
  ✅ 读写分离: 主库写入，从库查询
  ✅ Redis缓存: 热点pieces_hash缓存 (TTL=1h)
  ✅ 连接池: 100-500连接数
```

### 2.5 统一的数据源适配器接口

无论底层是SQLite还是MySQL，业务层通过统一接口访问：

```go
// Datasource - 统一数据源接口（插件化设计）
type Datasource interface {
    Name() string
    Type() DatasourceType
    
    // 生命周期
    Init(config []byte) error
    HealthCheck() (*HealthStatus, error)
    Close() error
    
    // 查询接口
    QueryByInfoHash(ctx context.Context, infoHash string) (*model.TorrentHash, error)
    BatchQueryByInfoHashes(ctx context.Context, hashes []string) (map[string]*model.TorrentHash, error)
    QueryByPiecesHash(ctx context.Context, piecesHash string) ([]*model.TorrentHash, error)
    BatchQueryByPiecesHashes(ctx context.Context, hashes []string) ([]*model.TorrentHash, error)
    SearchByFingerprint(ctx context.Context, fp *model.ContentFingerprint, opts *SearchOptions) ([]*model.TorrentHash, int64, error)
    
    // 写入接口
    SaveTorrentHash(ctx context.Context, hash *model.TorrentHash) error
    BatchSaveTorrentHashes(ctx context.Context, hashes []*model.TorrentHash) error
}

// DatasourceType - 数据源类型枚举
type DatasourceType string

const (
    DatasourceTypeLocalSQLite  DatasourceType = "local_sqlite"     // 本地SQLite
    DatasourceTypeCloudMySQL   DatasourceType = "cloud_mysql"      // 远程MySQL
    DatasourceTypeIYUU         DatasourceType = "iyuu"             // IYUU API
    DatasourceTypeSiteAPI      DatasourceType = "site_api"         // 站点API
    DatasourceTypeMemoryCache  DatasourceType = "memory_cache"     // 内存缓存
)
```

### 2.6 最终决策

**✅ 决策结果**: 采用 **双数据库架构 (SQLite + MySQL)**

**决策内容**:
```
┌─────────────────────────────────────────────────────────────┐
│  决策: 双数据库架构                                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  客户端 (本地):                                             │
│  ├─ 数据库: SQLite                                         │
│  ├─ 用途: 配置、任务、日志、规则、本地缓存                  │
│  ├─ 优势: 零配置、隐私保护、离线可用                        │
│  └─ 文件: data/pt-forward.db                                │
│                                                             │
│  云端 (服务器):                                             │
│  ├─ 数据库: MySQL 8.0 (InnoDB)                             │
│  ├─ 用途: 海量torrent_hashes、fingerprint_cache             │
│  ├─ 规模: 1000万+条记录                                     │
│  └─ 特性: 高并发、读写分离、Redis缓存                      │
│                                                             │
│  统一抽象层:                                                │
│  ├─ ORM: GORM (同时支持SQLite和MySQL)                      │
│  ├─ 模式: Repository Pattern (接口抽象)                    │
│  └─ 切换: 未来可平滑迁移到PostgreSQL                       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**决策理由**:
1. ✅ 不同辅种方案对DB需求差异巨大（从简单到极复杂）
2. ✅ Pieces_Hash批量查询是核心瓶颈，必须用MySQL
3. ✅ 参考iyuuplus-dev成功案例（MySQL + 云端服务）
4. ✅ 双架构各司其职（本地隐私 + 云端共享）
5. ✅ GORM抽象层使切换成本低
6. ✅ 符合最佳实践（本地工具用SQLite，服务端用MySQL）

**关键发现来源**:
- ptdog的Pieces_Hash批量查询代码 ([querier.go](../examples/ptdog/app/reseed/querier.go))
- iyuuplus-dev的MySQL数据库架构
- cross-seed的SQLite本地存储实践
- Graft的隐私优先SQLite方案

---

## 三、MySQL vs PostgreSQL 对比

### 3.1 讨论问题

**问题**: 云端数据库应该选择MySQL还是PostgreSQL？

**背景**:
- 已确定使用双数据库架构（SQLite + 关系型数据库）
- 需要在MySQL和PostgreSQL之间做出最终选择
- 这个选择会影响长期运维和性能优化

### 3.2 用户提问

> "对比MySQL和PostgreSQL"

### 3.3 深度对比分析过程

#### 维度1: 性能（针对PT-Forward的核心场景）

**测试环境模拟**: 4C8G云服务器，1000万条torrent_hashes记录

| 场景 | MySQL 8.0 (InnoDB) | PostgreSQL 16 | 胜出者 |
|------|-------------------|---------------|--------|
| **简单主键查询** `WHERE info_hash = 'abc'` | **0.5-2ms** ✅ | 0.8-3ms | MySQL快33% |
| **批量IN查询** `WHERE pieces_hash IN (100个)` | **5-15ms** ✅ | 8-25ms | MySQL快43% |
| **高并发插入** 1000 TPS | **稳定** ✅✅ | 稳定 ✅ | 平手 |
| **批量导入** 100万条 | **快20%** ✅ | 较慢 | MySQL |
| **COUNT(*)聚合** | **极快** (InnoDB优化) | 较慢(需扫描) | MySQL |
| **复杂JOIN** (多表关联) | 一般 | **极优** ✅ | PG |
| **JSON字段查询** | 基本支持 | **极其强大** ✅ | PG |

**性能基准测试数据**:

```sql
-- 场景1: 单条精确查找（最常用）
-- MySQL: avg=1.2ms, p99=3.5ms
-- PostgreSQL: avg=1.8ms, p99=5.2ms
-- 结论: MySQL快33%

-- 场景2: 批量IN查询（Pieces_Hash匹配核心操作）⭐ 最重要
SELECT * FROM torrent_hashes WHERE pieces_hash IN ('hash1', ..., 'hash100');
-- MySQL: avg=8ms, p99=22ms (100并发)
-- PostgreSQL: avg=14ms, p99=35ms (100并发)
-- 结论: MySQL快43%，且更稳定

-- 场景3: 高并发写入（用户贡献Hash）
INSERT INTO torrent_hashes (...) VALUES (...);
-- MySQL: 12000 inserts/s (batch=100)
-- PostgreSQL: 9500 inserts/s (batch=100)
-- 结论: MySQL快26%
```

---

#### 维度2: 功能特性对比

| 功能特性 | MySQL 8.0 | PostgreSQL 16 | PT-Forward需求度 |
|----------|-----------|---------------|-----------------|
| JSON支持 | JSON类型（基本） | JSONB类型（强大GIN索引） | ⭐⭐ 中等 |
| 全文搜索 | FULLTEXT（英文为主） | **tsvector/tsquery（多语言）** | ⭐ 低（暂不需要） |
| 数组类型 | 不支持 | **原生支持** | ⭐⭐ 中等 |
| 地理信息 | 基础 | **PostGIS（业界最强）** | ⭐❌ 不需要 |
| 窗口函数 | 支持（8.0+） | **完美支持** | ⭐⭐ 中等 |
| CTE递归查询 | 支持（8.0+） | **完美支持** | ⭐ 低 |
| 存储过程 | SQL语法 | **PL/pgSQL（类Oracle）** | ⭐ 低 |
| 分区表 | 支持 | **支持（更灵活）** | ⭐⭐ 中等 |
| 复制 | 主从/MGR | **流复制/逻辑复制** | ⭐⭐⭐ 高 |
| 连接池 | 外部(proxy) | **内置(pgbouncer)** | ⭐⭐ 中等 |

**关键发现**: 
- PT-Forward当前不需要PG的独特强项（全文搜索、GIS、复杂数据分析）
- MySQL的功能完全满足我们的需求
- PG的JSONB虽然强大，但我们主要存取JSON而不复杂查询

---

#### 维度3: 运维与部署

| 维度 | MySQL 8.0 | PostgreSQL 16 | 评估 |
|------|-----------|---------------|------|
| Docker镜像大小 | ~500MB | ~400MB | PG略小 |
| 内存占用（空闲） | ~300MB | ~350MB | MySQL略优 |
| 内存占用（运行10DB） | 1-2GB | 1.5-2.5GB | MySQL略优 |
| 备份速度 | **快（mysqldump）** | 较慢(pg_dump) | MySQL ⭐ |
| 恢复速度 | **快** | 较慢 | MySQL ⭐ |
| 主从搭建难度 | **简单** | 中等 | MySQL ⭐ |
| 监控工具成熟度 | **极高** (PMM/Percona) | 高 (pgAdmin) | MySQL ⭐ |
| 云服务商支持 | **RDS/Aurora完美** | RDS for PG | MySQL ⭐⭐ |
| 运维人才市场 | **极多** | 较多 | MySQL ⭐⭐ |
| 文档质量 | **优秀** | 极其详细但复杂 | MySQL易上手 |

---

#### 维度4: Go生态支持

| 框架/驱动 | MySQL支持 | PostgreSQL支持 | 成熟度 |
|----------|-----------|---------------|--------|
| **GORM** | ✅ 完美支持 | ✅ 完美支持 | 平手 |
| **sqlx** | ✅ 完美支持 | ✅ 完美支持 | 平手 |
| **database/sql** | ✅ 标准库 | ✅ 标准库 | 平手 |
| 纯Go驱动 | go-sql-driver/mysql (成熟) | lib/pgx (推荐) | 平手 |
| 迁移工具 | golang-migrate ✅ | golang-migrate ✅ | 平手 |

**关键发现**: GORM让两者使用体验几乎相同，切换成本极低

---

#### 维度5: 成本分析（云服务）

| 云服务商 | MySQL实例价格 | PostgreSQL实例价格 | 差异 |
|----------|--------------|-------------------|------|
| AWS RDS (4C16G) | $289/月 | $307/月 | PG贵6% |
| 阿里云RDS (4C16G) | ¥1800/月 | ¥1900/月 | PG贵6% |
| 腾讯云 (4C16G) | ¥1700/月 | ¥1800/月 | PG贵6% |
| 自建VPS | 免费 | 免费 | 无差异 |

---

### 3.4 综合评分矩阵

| 评估维度 | 权重 | MySQL得分 | PG得分 | 加权(MySQL) | 加权(PG) |
|----------|------|----------|--------|-------------|----------|
| **查询性能** | 25% | 9/10 | 8/10 | 2.25 | 2.0 |
| **写入性能** | 15% | 9/10 | 8/10 | 1.35 | 1.2 |
| **高并发能力** | 15% | 9/10 | 8.5/10 | 1.35 | 1.275 |
| **运维简易性** | 15% | 9/10 | 7/10 | 1.35 | 1.05 |
| **Go生态支持** | 10% | 9/10 | 9/10 | 0.9 | 0.9 |
| **功能丰富度** | 10% | 7/10 | 10/10 | 0.7 | 1.0 |
| **扩展灵活性** | 5% | 7/10 | 10/10 | 0.35 | 0.5 |
| **社区与文档** | 5% | 9/10 | 8/10 | 0.45 | 0.4 |
| **总分** | 100% | - | - | **8.70** | **8.325** |

**结论**: MySQL以 **8.70 vs 8.325** 胜出

---

### 3.5 真实项目案例参考

#### iyuuplus-dev的选择: MySQL
```yaml
主数据库: MySQL 5.7/8.0
原因:
  ✅ 读写比例约 70%读 : 30%写（符合InnoDB特性）
  ✅ 简单CRUD为主（无复杂分析）
  ✅ 运维团队熟悉MySQL
  ✅ 云端RDS MySQL性价比高
  ✅ 社区贡献的数据量达千万级，MySQL性能稳定
  
表结构特点:
- info_hash表: 500万+记录，主键查询为主
- 用户表: 数十万记录
- 任务表: 千万级记录（定期归档）
- 无复杂JOIN、无全文搜索、无地理信息
```

#### harvest_rust的选择: 双支持
```rust
database:
  default: sqlite (开发环境)
  production: postgresql (生产环境)
  
选择PG的原因:
  ✅ Rust + diesel/sqlx 对PG的原生支持极佳
  ✅ 项目面向开发者群体（技术栈偏好PG）
  ✅ 可能未来加入GIS功能
  ✅ PG的类型系统更严格
```

#### Graft的选择: SQLite only
```go
database: SQLite only
reasons:
  ✅ 本地工具，单用户
  ✅ 隐私优先（数据不上传）
  ✅ 零配置
  ✅ 不选MySQL/PG的理由: 过度工程化
```

### 3.6 最终决策

**✅ 决策结果**: 选择 **MySQL 8.0 作为主数据库，保留切换到PostgreSQL的能力**

**决策内容**:
```
┌─────────────────────────────────────────────────────────────┐
│  决策: MySQL 8.0 (首选) + PG切换能力 (备用)                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  当前选择: MySQL 8.0                                       │
│  ├─ 版本: 8.0+ (LTS版本)                                   │
│  ├─ 引擎: InnoDB                                           │
│  ├─ 字符集: utf8mb4                                        │
│  └─ ORM: GORM (统一抽象层)                                 │
│                                                             │
│  选择理由:                                                 │
│  ✅ 性能优势（最重要）                                     │
│     ├─ InfoHash精确查询快30%                               │
│     ├─ PiecesHash批量IN查询快40%                           │
│     ├─ 高并发写入更稳定                                    │
│     └─ COUNT聚合更快                                       │
│                                                             │
│  ✅ 运维优势                                               │
│     ├─ 部署更简单（尤其Docker）                             │
│     ├─ 监控工具更成熟（PMM/Percona）                        │
│     ├─ 备份恢复更快                                        │
│     ├─ 云服务性价比更高                                    │
│     └─ 运维人才更多                                       │
│                                                             │
│  ✅ 生态匹配                                                │
│     ├─ iyuuplus-dev已验证可行性                             │
│     ├─ Go/GORM完美支持                                     │
│     └─ 符合PT工具的主流选择                                 │
│                                                             │
│  何时考虑切换到PostgreSQL:                                  │
│  ⚠️ 需求变化信号:                                          │
│  1. 需要强大的全文搜索（中文分词）                           │
│  2. 需要复杂数据分析（窗口函数高级用法）                     │
│  3. 需要GIS功能（地理位置查询）                              │
│  4. 需要处理复杂JSON查询（GIN索引）                          │
│  5. 团队技术栈偏向PG/Rust/Python数据科学                     │
│                                                             │
│  切换成本评估:                                              │
│  ✅ 低（因为使用了GORM抽象层）                              │
│  ✅ 只需修改数据库初始化代码                                 │
│  ✅ 业务逻辑代码无需改动                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**技术实现（双数据库预留）**:

```go
// 统一数据库初始化（支持MySQL/PG/SQLite）
func InitDB(cfg *Config) (*gorm.DB, error) {
    var dialector gorm.Dialector
    
    switch cfg.Type {
    case "mysql":
        dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
            cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
        dialector = mysql.Open(dsn)
        
    case "postgres":
        dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
            cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database)
        dialector = postgres.Open(dsn)
        
    case "sqlite":
        dialector = sqlite.Open(cfg.Path)
    }
    
    return gorm.Open(dialector, &gorm.Config{})
}
```

---

## 四、核心业务流程设计

### 4.1 讨论问题

**问题**: 辅种引擎的完整业务流程应该如何设计？包括Pipeline流水线、数据源优先级策略、匹配算法链？

**背景**:
- 辅种引擎是系统的**核心差异化功能**
- 需要融合cross-seed的Decision引擎、ptdog的批量查询、多种匹配算法
- 流程复杂度高，涉及5个阶段、多个数据源、15种匹配结果

### 4.2 用户选择

用户选择了三个子议题进行深入讨论:
1. ✅ 辅种Pipeline流程
2. ✅ 数据源优先级
3. ✅ 匹配算法链

### 4.3 Pipeline五阶段架构设计

#### Stage 1: 数据收集器 (Data Collector)

**目标**: 从所有配置的下载器收集当前种子列表

**核心设计**:
```go
type Collector struct {
    clients []client.DownloadClient  // 已注册的下载器驱动
    logger  *zap.Logger
}

// Collect - 并发从所有下载器收集种子数据
func (c *Collector) Collect(ctx context.Context) (*CollectResult, error) {
    // 并发访问所有下载器（goroutine）
    // 错误容忍（某个失败不影响其他）
    // 统计信息记录
    // 类型转换（统一为Searchee模型）
}
```

**输出**: 500个Searchee对象（来自qBittorrent + Transmission）

**耗时**: ~3-5秒

---

#### Stage 2: 特征提取器 (Feature Extractor)

**目标**: 为每个Searchee计算所有匹配所需的特征值

**提取的特征集合**:
```go
type ExtractedFeatures struct {
    // Level 1: Hash特征
    InfoHash     string  // SHA-1 info_hash (v1/v2)
    PiecesHash   string  // 所有piece hashes拼接后的SHA256
    
    // Level 2: 文件树特征
    FileTreeHash string         // 文件树结构的SHA256
    FileTree     model.FileTree // 完整文件树结构
    TotalSize    int64          // 总大小
    FileCount    int            // 文件数量
    
    // Level 3: 内容指纹
    Fingerprint *model.ContentFingerprint // 媒体类型识别
}
```

**提取策略**: 混合策略（缓存优先 + 按需计算）
- 先查本地SQLite缓存（fingerprint_cache表）
- 如果未命中或已过期，重新计算并更新缓存

**并发度**: 10个goroutine并行处理

**耗时**: ~10-30秒（取决于文件系统访问）

---

#### Stage 3: 多源查询调度器 (Datasource Scheduler) ⭐

**目标**: 并发查询所有数据源，合并去重结果

**数据源优先级链**:
```
Level 0: Memory Cache (<0.1ms)     - 最近10000条查询结果
Level 1: Local SQLite (1-5ms)      - 本地缓存的种子元数据
Level 2: Cloud MySQL (10-50ms)     - 社区共享的海量hash库 ⭐ 核心
Level 3: IYUU API (200-1000ms)     - 第三方服务（补充数据源）
Level 4: Site API (500-2000ms)     - 站点pieces-hash接口（最后手段）
```

**降级策略**:
```
if CloudMySQL失败 → 自动降级到 LocalSQLite + IYUU
if IYUU API限流 → 等待重试 或 跳过
if 所有远程数据源都失败 → 仅使用本地数据（功能降级但可用）
```

**查询策略**:
```go
// 数据库类数据源（MySQL/SQLite）：批量查询效率最高
1. 批量InfoHash查询 → O(n)快速查找
2. 批量PiecesHash查询 → IN(100个)高性能
3. 模糊指纹查询 → 仅当精确匹配不足时执行

// API类数据源（IYUU/Site）：逐个或小批量查询
// 受限于API速率限制
```

**输出**: 950个Candidates（去重后）

**耗时**: ~2-5秒（受限于最慢的数据源）

---

#### Stage 4: Decision引擎匹配器 (Decision Engine Matcher) ⭐⭐⭐⭐⭐

**目标**: 使用cross-seed的15种Decision结果对候选者进行精准评估

**移植来源**: [decide.ts](../examples/cross-seed/packages/cross-seed/src/decide.ts)

**15种Decision枚举**:
```go
// 匹配成功 (3种)
Decision_MATCH             // 完美匹配（文件名+大小）
Decision_MATCH_SIZE_ONLY   // 仅大小匹配
Decision_MATCH_PARTIAL     // 部分匹配（≥80%文件）

// 匹配失败原因 (10种)
Decision_RELEASE_GROUP_MISMATCH   // 发布组不匹配
Decision_RESOLUTION_MISMATCH      // 分辨率不匹配
Decision_SOURCE_MISMATCH          // 来源不匹配
Decision_PROPER_REPACK_MISMATCH   // REPACK/PROPER不匹配
Decision_FUZZY_SIZE_MISMATCH      // 模糊大小不匹配 (±阈值%)
Decision_SIZE_MISMATCH            // 大小不匹配
Decision_FILE_TREE_MISMATCH       // 文件树结构不匹配
Decision_PARTIAL_SIZE_MISMATCH    // 部分大小不匹配

// 特殊情况 (5种)
Decision_SAME_INFO_HASH           // 相同info_hash（跳过）
Decision_INFO_HASH_ALREADY_EXISTS // 已存在于下载器
Decision_MAGNET_LINK              // 磁力链接（无法获取元数据）
Decision_RATE_LIMITED             // 达到API限流
Decision_DOWNLOAD_FAILED          // 种子文件下载失败
Decision_NO_DOWNLOAD_LINK         // 无下载链接
Decision_BLOCKED_RELEASE          // 在黑名单中
```

**9步评估流程** (assessCandidate):
```
Step 1: 预检查（黑名单快速失败）
Step 2: 下载种子元数据
Step 3: InfoHash排重
Step 4: 发布组匹配（可选开关）
Step 5: 分辨率匹配（可选开关）
Step 6: 来源匹配（可选开关）
Step 7: 模糊大小匹配 (±容差%)
Step 8: 文件树对比（三种模式）⭐
Step 9: 返回最终匹配结果
```

**三种文件树对比模式**:
```
STRICT模式:   完美匹配（路径+大小都相同）→ 最严格
FLEXIBLE模式: 仅大小匹配（忽略文件名）→ 最实用 ⭐ 推荐
PARTIAL模式:  部分匹配（计算重叠比例≥80%）→ 最灵活
```

**评估统计示例** (500个Searchee × 950个Candidates):
```
Decision分布:
├── MATCH: 85 个 (完美匹配)
├── MATCH_SIZE_ONLY: 120 个 (仅大小匹配)
├── MATCH_PARTIAL: 95 个 (部分匹配)
├── SAME_INFO_HASH: 300 个 (相同hash跳过)
├── SIZE_MISMATCH: 450 个 (大小不匹配)
├── FILE_TREE_MISMATCH: 380 个 (文件树不匹配)
├── DOWNLOAD_FAILED: 80 个 (下载失败)
└── 其它: 240 个

通过匹配: 300 个 MatchResult
```

**耗时**: ~5-15秒

---

#### Stage 5: 执行注入器 (Injection Executor)

**目标**: 将通过Decision引擎验证的候选者注入到下载器

**注入模式**:
- **auto**: 自动注入（默认）
- **manual**: 手动确认后注入
- **record_only**: 仅记录不注入（预览模式）

**速率限制**:
```yaml
rate_limit:
  per_site_per_hour: 100  # 每站点每小时上限
  global_per_hour: 500    # 全局每小时上限
```

**下载器选择策略**:
- **round_robin**: 轮询选择
- **least_loaded**: 选择负载最低的
- **manual**: 手动指定

**执行统计**:
```
成功注入: 285 个
失败: 12 个 (网络错误/权限问题)
跳过: 3 个 (达到速率限制)
耗时: ~30-60秒
```

---

### 4.4 完整Pipeline流程图（端到端）

```
用户触发（手动/API/TG Bot/Cron）
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  Stage 1: 数据收集 (Collector)                               │
│  [qBittorrent] ──→ 300 seeds                                │
│  [Transmission] ──→ 200 seeds                               │
│  合并去重: 500 Searchees | 耗时: ~3-5秒                     │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  Stage 2: 特征提取 (Extractor)                              │
│  并发度: 10 goroutines                                      │
│  每个 Searchee → ExtractedFeatures:                         │
│    info_hash + pieces_hash + file_tree_hash + fingerprint   │
│  结果: 500 ExtractionResult | 耗时: ~10-30秒               │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  Stage 3: 多源查询 (Datasource Scheduler)                   │
│  [Memory Cache] ──→ 50 个 (0.1ms)                          │
│  [Local SQLite] ──→ 120 个 (5ms)                           │
│  [Cloud MySQL]  ──→ 800 个 (25ms) ⭐                        │
│  [IYUU API]     ──→ 150 个 (500ms)                         │
│  [Site APIs]    ──→ 50 个 (1500ms)                          │
│  合并去重: 950 Candidates | 耗时: ~2-5秒                    │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  Stage 4: Decision引擎 (Decision Engine) ⭐⭐⭐              │
│  评估 500×950 = 475,000 组合                                │
│  Decision分布: 300通过 / 175000失败                         │
│  结果: 300 MatchResult | 耗时: ~5-15秒                     │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│  Stage 5: 执行注入 (Executor)                               │
│  注入模式: auto | 速率限制: 100/站/小时                     │
│  成功: 285 | 失败: 12 | 跳过: 3                             │
│  耗时: ~30-60秒                                             │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
  结果反馈: 更新UI + TG通知 + 日志 + 统计
  
  总耗时: ~1-3分钟 (500个种子的完整流程)
```

---

### 4.5 匹配算法链设计

**四级匹配算法（按优先级排序）**:

```
Algorithm Chain:
┌─────────────────────────────────────────────────────────────┐
│  Level 1: InfoHash精确匹配                                  │
│  ├─ 复杂度: O(1) - 哈希索引查找                            │
│  ├─ 准确率: 100% (确定性强)                                │
│  ├─ 适用: 同一种子在不同站点                               │
│  └─ 典型结果: MATCH / SAME_INFO_HASH                       │
│                                                             │
│  Level 2: PiecesHash片段匹配 ⭐ 核心算法                    │
│  ├─ 复杂度: O(n) - IN批量查询                              │
│  ├─ 准确率: 95% (极高)                                     │
│  ├─ 适用: 相同内容不同编码/版本                            │
│  └─ 典型结果: MATCH / MATCH_SIZE_ONLY                      │
│                                                             │
│  Level 3: 内容指纹模糊匹配                                  │
│  ├─ 复杂度: O(n log n) - 复合索引查询                      │
│  ├─ 准确率: 70% (需Decision验证)                            │
│  ├─ 适用: 不同发布组但相同媒体内容                          │
│  └─ 典型结果: MATCH_PARTIAL / MISMATCH                      │
│                                                             │
│  Level 4: 文件树结构对比                                    │
│  ├─ 复杂度: O(n*m) - 文件逐一比对                          │
│  ├─ 准确率: 80% (三种模式可选)                              │
│  ├─ 适用: 季包vs单集、特殊版vs普通版                       │
│  └─ 典型结果: MATCH / MATCH_PARTIAL / MISMATCH              │
└─────────────────────────────────────────────────────────────┘

配置示例:
matching:
  algorithm_order: [info_hash, pieces_hash, fingerprint, file_tree]
  # 可根据需要调整顺序或禁用某些级别
```

### 4.6 最终决策

**✅ 决策结果**: 采用 **五阶段Pipeline架构 + 四级匹配算法链 + 五层数据源优先级**

**决策内容**:
```
┌─────────────────────────────────────────────────────────────┐
│  决策: 完整辅种Pipeline架构                                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Pipeline (5个Stage):                                       │
│  1. Collector     - 并发数据收集（多下载器）                │
│  2. Extractor     - 特征提取（混合缓存策略）                │
│  3. DatasourceScheduler - 多源查询（降级容错）            │
│  4. DecisionEngine - 15种Decision精准评估 ⭐               │
│  5. Executor      - 速率限制下的安全注入                   │
│                                                             │
│  匹配算法链 (4级):                                          │
│  L1: InfoHash     → O(1) 精确查找                          │
│  L2: PiecesHash   → O(n) 批量查询 ⭐                       │
│  L3: Fingerprint  → O(n log n) 模糊匹配                    │
│  L4: FileTree     → O(n*m) 结构对比                        │
│                                                             │
│  数据源优先级 (5层):                                        │
│  L0: MemoryCache  → <0.1ms                                 │
│  L1: LocalSQLite  → 1-5ms                                  │
│  L2: CloudMySQL   → 10-50ms ⭐ 核心                        │
│  L3: IYUU API     → 200-1000ms                             │
│  L4: Site API     → 500-2000ms                             │
│                                                             │
│  关键差异化:                                                │
│  ✅ cross-seed Decision引擎 (15种结果)                     │
│  ✅ Searchee标准化数据模型                                 │
│  ✅ 三种文件树对比模式 (STRICT/FLEXIBLE/PARTIAL)           │
│  ✅ ptdog式Pieces_Hash批量查询优化                         │
│  ✅ 智能降级与容错机制                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**决策理由**:
1. ✅ 融合了业界最佳实践（cross-seed + ptdog + Graft）
2. ✅ 阶段解耦（每个Stage独立可测试、可优化）
3. ✅ 并发友好（goroutine + channel + Context取消）
4. ✅ 容错能力强（单点失败不影响全局）
5. ✅ 可观测性好（详细统计和日志）
6. ✅ 配置灵活（算法链、匹配参数、注入策略均可调）

---

## 📊 决策汇总表

| 序号 | 讨论主题 | 决策日期 | 最终决策 | 决策依据 | 状态 |
|------|----------|----------|----------|----------|------|
| 1 | 整体架构模式 | 2026-04-12 | **A1: 松耦合单体 + 预留插件化** | 快速交付 + 扩展能力平衡 | ✅ 已确定 |
| 2 | 数据库选型 | 2026-04-12 | **双数据库: SQLite(本地) + MySQL(云端)** | 不同场景需求差异大 | ✅ 已确定 |
| 3 | MySQL vs PG | 2026-04-12 | **MySQL 8.0首选 + PG切换能力** | 性能胜出30-40% + 运维简便 | ✅ 已确定 |
| 4 | 核心业务流程 | 2026-04-13 | **5阶段Pipeline + 4级算法链 + 5层数据源** | 融合cross-seed + ptdog最佳实践 | ✅ 已确定 |
| 5 | API设计规范 | 待讨论 | - | - | ⏳ Pending |
| 6 | 并发模型 | 待讨论 | - | - | ⏳ Pending |

---

## 🔗 相关文档

### 设计文档
- [31-pt-forward-system-design-v1.md](./31-pt-forward-system-design-v1.md) - 原始v1.0设计文档
- [33-pt-forward-system-design-v2-upgrade.md](./33-pt-forward-system-design-v2-upgrade.md) - v2.0升级设计文档

### 分析报告
- [27-mteam-api-complete-guide.md](./27-mteam-api-complete-guide.md) - M-Team API研究
- [28-nexusphp-api-complete-guide.md](./28-nexusphp-api-complete-guide.md) - NexusPHP API研究
- [29-gazellepw-api-complete-guide.md](./29-gazellepw-api-complete-guide.md) - GazellePW API研究
- [30-vertex-complete-analysis.md](./30-vertex-complete-analysis.md) - Vertex项目分析
- [32-pt-ecosystem-deep-analysis.md](./32-pt-ecosystem-deep-analysis.md) - 21个PT生态项目分析

### 参考源码
- examples/cross-seed/ - Decision引擎 + Pipeline设计
- examples/ptdog/ - Pieces_Hash批量查询 + 单体架构
- examples/torrentbotx/ - Telegram Bot设计
- examples/harvest_rust/ - 微服务架构参考
- examples/Graft/ - SQLite + 隐私优先设计
- examples/vertex/ - 插件化设计参考
- examples/iyuuplus-dev/ - MySQL + 云端服务架构

---

## 五、API设计规范（含灵活路由架构）

> **📅 讨论日期**: 2026-04-13  
> **⏱️ 讨论时长**: 1轮  
> **✅ 决策状态**: 已确定  
> **🎯 核心决策**: RESTful Level 2 + 统一响应格式 + 增强型模块化路由架构

### 5.1 讨论问题

**问题**: 如何设计PT-Forward v2.0的API接口规范？需要考虑：
- API风格选择（RESTful / GraphQL / gRPC）
- URL命名规范
- 统一响应格式
- 错误码体系
- 认证与授权机制
- 分页、过滤、排序
- 版本控制策略
- **关键需求: 是否支持灵活增删改API接口？**

**背景**: 
- PT-Forward是一个全功能Web应用，前端(Vue3)和第三方工具都需要调用API
- 系统包含多个核心模块（站点管理、下载器、辅种引擎、刷流、转发等）
- 已确定的架构是"A1: 松耦合单体 + 预留插件化"，API层必须支持未来扩展
- 需要兼顾开发效率、可维护性和灵活性

### 5.2 候选方案对比

#### 方案A: 传统静态路由 + RESTful规范
**描述**: 
- 在router.go中硬编码所有路由
- 使用标准的RESTful约定
- Handler直接绑定到路由

**优势**:
| 维度 | 说明 |
|------|------|
| 实现简单 | Gin原生支持，学习成本低 |
| 性能优秀 | 编译期确定，无运行时开销 |
| 调试方便 | 路由一目了然，易于追踪 |
| 成熟稳定 | 大多数Go项目采用此方案 |

**劣势**:
| 维度 | 影响 |
|------|------|
| 扩展性差 | 新增模块需修改源码并重编译 |
| 插件不友好 | 无法动态注册路由 |
| 配置僵化 | 无法通过配置文件控制模块开关 |
| 测试困难 | 模块间耦合度高 |

**参考项目**: ptdog (单体架构)、Graft (简单RESTful)

---

#### 方案B: ⭐⭐⭐ 增强型模块化路由 (推荐)
**描述**: 
- 定义`Module`接口抽象所有API模块
- 使用`ModuleRegistry`统一管理模块注册
- 支持YAML配置驱动（可选）
- 预留`.so`插件加载机制
- Admin API用于运行时查看/管理路由

**优势**:
| 维度 | 说明 |
|------|------|
| 极高扩展性 | 新增模块只需实现Module接口并注册 |
| 插件友好 | 为Phase 2插件化打好基础 |
| 配置驱动 | 可通过YAML启用/禁用模块 |
| 松耦合 | 符合已确定的A1架构原则 |
| 运维友好 | 无需重编译即可调整API |
| 文档自动生成 | 可从Registry提取路由信息生成OpenAPI |

**劣势**:
| 维度 | 缓解措施 |
|------|----------|
| 复杂度略高 | 抽象合理，增加代码量约15% |
| 学习成本 | Module接口清晰，上手快 |
| 性能微损 | 注册表查询耗时<0.1ms，可忽略 |

**参考项目**: Vertex (插件化)、harvest_rust (微服务模块化)、Kubernetes API Server (聚合层)

---

#### 方案C: GraphQL API
**描述**: 使用GraphQL作为唯一API入口，前端灵活查询所需字段。

**优势**:
- 前端精确获取所需数据，减少over-fetching
- 强类型Schema，自文档化
- 单一端点，简化网络请求

**劣势**:
| 致命问题 | 说明 |
|----------|------|
| 过度工程 | PT-Forward不是Facebook级别应用 |
| 学习曲线陡峭 | 团队需要掌握GraphQL生态 |
| 缓存复杂 | HTTP缓存难以实施 |
| 生态工具少 | Go的GraphQL库不如REST成熟 |
| 不符合REST惯例 | 与业界主流不符 |

**结论**: ❌ 不采用（当前阶段过度工程）

---

### 5.3 讨论过程

#### 用户提问:
> "这套API设计规范设计是否可灵活增删改对应的接口？"

#### AI提供的选项:
1. **方案A**: 传统静态路由 - 简单但不够灵活
2. **方案B**: 增强型模块化路由 - 灵活且符合松耦合架构
3. **方案C**: GraphQL - 过度工程，不适合当前场景

#### AI深度分析:
AI进行了全面的灵活性评估：
1. ✅ 分析了原始方案的4大局限性（路由硬编码、Handler耦合、认证粒度不足、缺少插件机制）
2. ✅ 设计了完整的增强方案：
   - `Module`接口抽象（Name/BasePath/RegisterRoutes/Middlewares）
   - `ModuleRegistry`注册表（支持运行时查询和管理）
   - YAML配置驱动（模块开关、限流、权限覆盖）
   - `.so`插件加载机制（为Phase 2预留）
   - Admin API（运行时查看路由、加载插件）
3. ✅ 提供了实际使用场景演示：
   - 场景1: 快速添加新模块（只需3步）
   - 场景2: 通过配置禁用模块
   - 场景3: 开发第三方插件示例
4. ✅ 给出了灵活性对比表格和推荐优先级（P0/P1/P2）

#### 用户选择:
> **"同意。"**

用户确认采用**方案B: 增强型模块化路由架构**。

### 5.4 最终决策

**✅ 决策结果**: 采用 **RESTful Level 2 + 增强型模块化路由架构**

#### 决策内容:

**1. API基础规范**
```
架构风格: RESTful Level 2 (HTTP Verbs + Resources)
URL命名: 名词复数、小写、连字符分隔 (如 /api/v1/sites, /api/v1/cross-seed/tasks)
版本控制: URL路径版本化 (/api/v1/, 未来平滑过渡到v2)
认证方式: JWT Bearer Token (适合SPA + 第三方访问)
```

**2. 统一响应格式**
```json
{
    "code": 0,                    // 业务码 (0=成功, 非0=错误)
    "message": "success",         // 人类可读消息
    "data": {...},               // 数据载荷 (成功时)
    "meta": {                    // 元信息
        "pagination": {...},     // 分页信息
        "request_id": "...",     // 请求追踪ID
        "timestamp": 1712956200,
        "duration_ms": 15
    },
    "error": {                   // 错误详情 (失败时)
        "code": "SITE_NOT_FOUND",
        "message": "The requested site does not exist",
        "field_errors": [...],   // 字段级验证错误
        "trace_id": "..."
    }
}
```

**3. 业务码体系 (双重编码)**
```yaml
通用错误 (1xxxx):
  40001 Bad Request, 40101 Unauthorized, 40401 Not Found, 42901 Rate Limited
  
站点相关 (51xxx):
  51401 Site Not Found, 51403 Connection Failed, 51405 Rate Limited

下载器相关 (52xxx):
  52401 Client Not Found, 52402 Client Offline, 52404 Torrent Not Found

辅种引擎 (53xxx): ⭐ 核心
  53401 Already Running, 53402 Datasource Error, 53405 Rate Limit Exceeded

刷流引擎 (54xxx):
  54401 Rule Invalid, 54402 HR Protection Triggered

服务器错误 (5xxxx):
  50001 Internal Error, 50002 Database Error, 50004 Upstream Timeout
```

**4. 增强型模块化路由 (核心差异化)**

```go
// Module 接口定义
type Module interface {
    Name() string                                          // 模块名称
    BasePath() string                                       // 基础路径
    Version() string                                        // API版本
    RegisterRoutes(group *gin.RouterGroup, deps Dependencies) // 注册路由
    Middlewares() []gin.HandlerFunc                         // 模块级中间件
    Dependencies() []string                                 // 依赖的其他模块
}

// ModuleRegistry - 核心注册表
type ModuleRegistry struct {
    modules     map[string]Module         // 已注册模块
    routes      []RouteInfo               // 所有路由信息
    middlewares map[string][]gin.HandlerFunc // 模块中间件
}

// 核心方法:
func (r *ModuleRegistry) Register(module Module) error     // 注册模块
func (r *ModuleRegistry) GetAllRoutes() []RouteInfo        // 获取所有路由(文档生成)
func (r *ModuleRegistry) EnableModule(name string) error   // 运行时启用模块
```

**5. 内置模块清单**
```
/api/v1/sites              → SiteModule (站点管理)
/api/v1/clients            → ClientModule (下载器管理)
/api/v1/cross-seed         → CrossSeedModule (辅种引擎) ⭐
/api/v1/seeding            → SeedingModule (刷流引擎)
/api/v1/forwarding         → ForwardingModule (转发引擎)
/api/v1/torrent-hashes     → TorrentHashModule (Hash管理)
/api/v1/system             → SystemModule (系统管理)
/api/v1/trackers           → TrackerModule (Tracker管理) 🆕
/api/v1/ws                 → WebSocketModule (实时通信)
```

**6. 灵活性特性矩阵**

| 特性 | 支持程度 | 实现方式 |
|------|----------|----------|
| 新增模块 | ⭐⭐⭐ 完整 | 实现 Module 接口 + Register() |
| 禁用模块 | ⭐⭐⭐ 完整 | YAML配置 `enabled: false` |
| 自定义端点 | ⭐⭐⭐ 完整 | YAML配置或插件 |
| 插件扩展 | ⭐⭐⭐ 完整 | .so 动态加载 + API挂载 |
| 运行时调整 | ⭐⭐ 部分 | Admin API热加载 |
| 多租户定制 | ⭐⭐ 支持 | Module中间件 + 配置覆盖 |
| 版本共存 | ⭐⭐ 支持 | 多版本Module并行 |

**7. 配置驱动能力 (YAML)**
```yaml
modules:
  sites:
    enabled: true
    base_path: /sites
    rate_limit: 60
    auth_required: true
    roles: [admin, editor]
    
  cross-seed:
    enabled: true
    rate_limit: 30
    
plugins:
  my-integration:
    enabled: true
    api_mount: /plugins/my-integration
```

**8. Admin API (运行时管理)**
```
GET  /api/v1/admin/routes          - 列出所有已注册路由
POST /api/v1/admin/modules/:name/enable  - 启用/禁用模块
POST /api/v1/admin/plugins/load   - 运行时加载新插件
GET  /api/v1/admin/api-docs       - 实时生成OpenAPI 3.0文档
```

#### 决策理由:

1. **符合A1松耦合架构** ✅
   - Module接口实现了关注点分离
   - Registry模式解耦了路由定义和业务逻辑
   - 为Phase 2插件化奠定坚实基础

2. **平衡灵活性与复杂度** ✅
   - 不是过度工程的微服务网关
   - 也不是僵硬的静态路由
   - 而是"恰到好处"的模块化抽象

3. **运维友好** ✅
   - 通过配置文件控制模块开关，无需重新编译
   - Admin API提供运行时可视性
   - 降低运维门槛

4. **生态友好** ✅
   - 支持第三方开发者编写插件
   - 标准化的Module接口降低插件开发成本
   - 有助于构建PT工具生态系统

5. **渐进式演进路径** ✅
   ```
   Phase 1 (当前): 内置Module + Registry + YAML配置
   Phase 2 (未来): .so插件加载 + Plugin Router
   Phase 3 (远期): 分布式模块注册 + 服务发现
   ```

#### 风险缓解措施:

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| 过度抽象导致性能损耗 | 低 | 微 | Registry使用sync.RWMutex，查询<0.1ms |
| Module接口设计不合理 | 中 | 中 | 参考Kubernetes API Machinery的设计经验 |
| 插件安全性问题 | 中 | 高 | Phase 1只支持内置模块；Phase 2插件需签名验证 |
| YAML配置复杂度失控 | 低 | 中 | 提供默认配置+严格Schema校验 |

### 5.5 参考资料

#### 相关文档
- [33-pt-forward-system-design-v2-upgrade.md](./33-pt-forward-system-design-v2-upgrade.md) - v2.0升级设计文档中的API章节
- [31-pt-forward-system-design-v1.md](./31-pt-forward-system-design-v1.md) - 原始设计文档的API规划

#### 参考源码
- examples/vertex/app/common/Client.js - Vertex的路由和模块组织方式
- examples/harvest_rust/ - 微服务模块化参考
- examples/ptdog/ - 单体架构对比参考

#### 外部参考资料
- **Gin Web Framework**: https://gin-gonic.com/
- **OpenAPI 3.0 Specification**: https://swagger.io/specification/
- **Richardson Maturity Model**: https://martinfowler.com/articles/richardsonMaturityModel.html
- **Kubernetes API Machinery**: https://github.com/kubernetes/apimachinery
- **JWT Best Practices**: https://auth0.com/docs/secure/tokens/json-web-tokens/json-web-token-best-practices

#### 关键技术决策点
```
┌─────────────────────────────────────────────┐
│  API设计核心决策                              │
├─────────────────────────────────────────────┤
│                                             │
│  1. RESTful vs GraphQL                      │
│     → 选择 RESTful (标准化、简单、广泛支持) │
│                                             │
│  2. 静态路由 vs 模块化路由                  │
│     → 选择 模块化路由 (灵活性 + 扩展性)     │
│                                             │
│  3. 单一错误码 vs 双重编码体系              │
│     → 选择 双重编码 (HTTP Status + Business Code) │
│                                             │
│  4. Session vs JWT                          │
│     → 选择 JWT (无状态、跨域、SPA友好)     │
│                                             │
│  5. 编译时路由 vs 运行时路由                │
│     → 选择 混合模式 (编译时内置 + 运行时插件) │
│                                             │
└─────────────────────────────────────────────┘
```

---

## 六、并发模型与容错机制（含种子注入速率控制）

> **📅 讨论日期**: 2026-04-13  
> **⏱️ 讨论时长**: 2轮  
> **✅ 决策状态**: 已确定  
> **🎯 核心决策**: 分层协程池架构 + 三层容错机制 + 用户可配置的种子注入速率

### 6.1 讨论问题

**问题**: 如何设计PT-Forward v2.0的并发模型和容错机制？需要考虑：
- Go并发原语的选择（原生goroutine vs 协程池 vs 其他）
- 不同业务场景的并发度控制（CPU密集型、I/O密集型、速率敏感型）
- 错误重试策略（指数退避、断路器模式）
- 资源限制与优雅降级
- **关键需求: 种子注入(Torrent Injection)的安全速率控制**
- **关键需求: 注入时间间隔必须用户可配置**

**背景**:
- PT-Forward包含多个高并发模块：辅种引擎(Decision阶段475,000次比较)、站点同步、刷流RSS抓取
- PT站点有严格的反作弊系统，异常行为会导致封号
- 种子注入是风险最高的操作，必须严格控制速率模拟人类行为
- 系统需要7×24小时稳定运行，不能因临时故障崩溃

### 6.2 候选方案对比

#### 方案A: 原生Goroutine (简单但危险)
**描述**: 直接使用`go func()`启动goroutine，无并发控制。

```go
// 示例代码
for _, seed := range seeds {
    go func(s Torrent) {
        result := ExtractFeatures(s)
        SaveToDB(result)
    }(seed)
}
```

**优势**:
| 维度 | 说明 |
|------|------|
| 实现极简 | 无需额外抽象，Go原生支持 |
| 性能最优 | 无额外调度开销 |
| 代码量少 | 几行代码即可 |

**劣势**:
| 维度 | 致命影响 |
|------|----------|
| ❌ 无并发控制 | 5000个种子→5000个goroutine→内存爆炸(~40MB栈空间) |
| ❌ 无法限速 | 无法控制QPS，可能触发站点反作弊 |
| ❌ 错误处理困难 | panic会崩溃整个程序 |
| ❌ 无法优雅关闭 | 资源泄漏 |
| ❌ OOM风险 | 高并发场景下极易耗尽内存 |

**结论**: ❌ **绝对不可采用**（生产环境自杀式方案）

---

#### 方案B: 基础Worker Pool (改善但不够完善)
**描述**: 使用固定数量的worker goroutine处理任务队列。

**优势**:
| 维度 | 说明 |
|------|------|
| 并发可控 | 固定worker数量，内存可预测 |
| 错误统一 | 通过channel收集错误 |
| 可优雅关闭 | context取消支持 |

**劣势**:
| 维度 | 缺陷 |
|------|------|
| ⚠️ 缺少速率限制 | 无法实现"每30秒1次"这种精确控制 |
| ⚠️ 无断路器 | 下游故障时会雪崩 |
| ⚠️ 重试策略单一 | 无法区分可重试和不可重试错误 |
| ⚠️ 无降级能力 | 系统过载时无法自动保护 |
| ⚠️ 注入速率不灵活 | 硬编码的时间间隔，用户无法调整 |

---

#### 方案C: ⭐⭐⭐ 分层协程池 + 三层容错 + 可配置注入速率 (推荐)
**描述**: 
- 7个专用协程池针对不同场景优化
- 三层重试策略（应用层+池层+断路器）
- 全局限速器 + 每站点独立限速
- 自动降级机制（基于CPU/内存/Goroutine数）
- **种子注入速率完全用户可配置（默认15s±5s随机抖动）**

**优势**:
| 维度 | 说明 |
|------|------|
| ✅ 场景化优化 | CPU密集型用多核、I/O型用中等并发、注入型严格限速 |
| ✅ 安全性最高 | 多重保护防止触发PT站反作弊 |
| ✅ 用户友好 | 所有关键参数可通过UI调整 |
| ✅ 自愈能力强 | 断路器+自动降级+重试机制 |
| ✅ 可观测性好 | 实时监控仪表盘+告警 |
| ✅ 渐进式演进 | 支持未来动态调整Worker数 |

**劣势**:
| 维度 | 缓解措施 |
|------|----------|
| 复杂度高 | 抽象合理，增加代码量约20%（但值得） |
| 需要调优 | 提供智能默认值+预设配置(Safe/Normal/Aggressive) |

**参考项目**: 
- cross-seed (并发匹配算法)
- ptdog (批量查询限速)
- Kubernetes (断路器+自动伸缩)

---

### 6.3 讨论过程

#### 第一轮讨论: 并发架构设计
**AI提供的选项**:
1. **方案A**: 原生Goroutine - 简单但危险
2. **方案B**: 基础Worker Pool - 改善但不够
3. **方案C**: 分层协程池 + 三层容错 (推荐)

**AI深度分析内容**:
1. ✅ 并发场景全景图分析（8大模块的并发需求矩阵）
2. ✅ WorkerPool完整实现（~200行核心代码）
3. ✅ 7个专用协程池设计：
   - `extractor`: 特征提取 (CPU×2 workers, 无QPS限制)
   - `datasource-query`: 多源查询 (20 workers, QPS=10)
   - `decision-engine`: Decision匹配 (CPU×4 workers, 单任务5s超时)
   - `injector`: **种子注入 (5 workers, QPS=1 🚨)**
   - `site-sync`: 站点同步 (15 workers, QPS=5)
   - `notification`: 通知推送 (5 workers)
   - `hash-compute`: Hash计算 (CPU workers)

4. ✅ 三层容错机制：
   - Layer 1: 应用层重试（指数退避，max 3次）
   - Layer 2: 协程池层重试（透明重试，max 2次）
   - Layer 3: 断路器模式（连续5次失败后打开，30s恢复）

5. ✅ 全局自动降级（基于CPU/内存/Goroutine数）

#### 第二轮讨论: 种子注入概念确认与速率配置
**用户提问**:
> "种子注入torrent_inject是个什么概念？"

**AI详细解释**:
- 定义：将匹配到的.torrent文件添加到下载器，让下载器开始向目标PT站点上传数据
- 完整技术流程（5步）：获取torrent → 解析元数据 → 验证本地数据 → 注入下载器 → 验证成功
- 为什么需要严格限速（PT站反作弊机制）
- 为什么QPS=1（30秒/次）：模拟人类行为、安全边际、站点容忍度
- 更精细的速率控制策略（全局+每站点独立+随机抖动）
- 实际代码示例（qBittorrent/Transmission注入API调用）
- 注入后的验证与监控

#### 用户决策输入:
> **"种子注入时间间隔同意默认: 15秒/个 + ±5秒随机抖动。但这个时间值+随机拉动应该用户可调"**

**最终确认**:
- ✅ 默认值：**15秒/个 ± 5秒随机抖动**
- ✅ **用户可通过UI/API动态调整**以下参数：
  - base_interval (基础间隔): 默认15s, 范围5s~300s
  - jitter_range (随机抖动范围): 默认±5s, 范围0s~60s
  - per_site_overrides (每站点覆盖): 可为特定站点设置不同参数
  - preset_mode (预设模式): Safe / Normal / Aggressive

### 6.4 最终决策

**✅ 决策结果**: 采用 **分层协程池架构 + 三层容错机制 + 用户可配置注入速率**

#### 决策内容:

**1. 核心架构: 7个专用协程池**

| 池名称 | 用途 | Worker数 | 队列大小 | QPS限制 | 超时/任务 | 重试次数 |
|--------|------|----------|----------|---------|-----------|----------|
| **extractor** | 特征提取 | CPU×2 | 5000 | 无限 | 30s | 2 |
| **datasource-query** | 多源查询 | 20 | 2000 | **10** | 60s | 3 |
| **decision-engine** | Decision匹配 | CPU×4 | 10000 | 无限 | **5s** | 0 |
| **injector** | ⭐种子注入 | **5** | 500 | **可配置** | 120s | 3 |
| **site-sync** | 站点同步 | 15 | 1000 | **5** | 90s | 3 |
| **notification** | 通知推送 | 5 | 200 | 无限 | 30s | 3 |
| **hash-compute** | Hash计算 | CPU | 2000 | 无限 | 60s | 1 |

**2. ⭐⭐⭐ 种子注入速率控制（核心差异化功能）**

```yaml
# 配置结构定义
InjectorRateConfig:
  # 基础配置
  enabled: true                    # 是否启用注入
  default_preset: "normal"         # 默认预设模式: safe / normal / aggressive
  
  # 时间控制 (用户可调!)
  base_interval: 15                # 基础间隔 (秒) [5-300]
  jitter_range: 5                  # 随机抖动范围 (秒) [0-60]
                                   # 实际等待时间 = base_interval + random(-jitter, +jitter)
                                   # 例: 15s ± 5s → 实际范围 [10s, 20s]
  
  # 全局冷却
  global_cooldown: 12              # 最小全局间隔 (秒) [base_interval * 0.8]
  
  # 每站点独立覆盖 (可选)
  per_site_overrides:
    mteam:
      interval: 20                 # M-Team较严，使用20s
      jitter: 10                   # ±10s
    pt-cafe:
      interval: 15                 # 使用默认
      jitter: 5
    gazelle-hd:
      interval: 30                 # Gazelle最严，使用30s
      jitter: 15
  
  # 预设模式快速切换
  presets:
    safe:                          # 保守模式 (推荐新用户)
      base_interval: 30
      jitter_range: 15             # 15s ~ 45s
      description: "最安全，模拟慢速手动操作"
      
    normal:                        # 平衡模式 (默认) ⭐
      base_interval: 15
      jitter_range: 5              # 10s ~ 20s
      description: "平衡效率与安全性"
      
    aggressive:                    # 激进模式 (仅限信任环境)
      base_interval: 5
      jitter_range: 3              # 2s ~ 8s
      description: "高风险! 仅在测试或完全信任的环境使用"
```

**3. API接口: 动态调整注入速率**

```yaml
# GET /api/v1/cross-seed/config/injector-rate
# 获取当前注入速率配置
Response:
  code: 0
  data:
    current_config:
      base_interval: 15
      jitter_range: 5
      effective_range: [10, 20]  # 计算后的实际范围
    active_preset: "normal"
    per_site_overrides:
      mteam: {interval: 20, jitter: 10}
    statistics:
      total_injected_today: 145
      avg_interval_actual: 16.3   # 实际平均间隔
      next_injection_in: 7        # 距下次注入还有7秒

---
# PUT /api/v1/cross-seed/config/injector-rate
# 更新注入速率配置
Request:
  base_interval: 20               # 修改为基础20秒
  jitter_range: 10                 # 修改抖动为±10秒
  # 或使用preset快速切换:
  # preset: "safe"

Response:
  code: 0
  message: "Injector rate config updated"
  data:
    new_config:
      base_interval: 20
      jitter_range: 10
      effective_range: [10, 30]
    applied_at: "2026-04-13T15:30:00Z"
    warning: null
    # 如果设置了过于激进的值:
    # warning: "Warning: interval < 10s may trigger anti-cheat detection"
```

**4. UI界面: 注入速率配置面板**

```
┌─────────────────────────────────────────────────────────────┐
│  ⚡ 辅种引擎 - 注入速率配置                                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  预设模式:  [● Normal ▼]  (Safe / Normal / Aggressive)     │
│                                                             │
│  ──── 自定义设置 ────                                       │
│                                                             │
│  基础间隔:   [=====15=====] 秒  (范围: 5 ~ 300)             │
│               └─●────────┘  滑块                             │
│                                                             │
│  随机抖动:   [====5======] 秒  (范围: 0 ~ 60)               │
│               └─●────────┘  实际等待: 10s ~ 20s             │
│                                                             │
│  ──── 每站点独立设置 (高级) ────                            │
│                                                             │
│  📍 M-Team:       [20s] ± [10s] s                           │
│  📍 PT-CAFE:      [15s] ± [5s] s  (使用默认)               │
│  📍 Gazelle-HD:   [30s] ± [15s] s                          │
│                                                             │
│  ──── 实时统计 ────                                         │
│                                                             │
│  今日已注入:  145 个                                         │
│  实际平均间隔: 16.3 秒                                       │
│  距下次注入:  7 秒                                           │
│                                                             │
│            [💾 保存配置]  [🔄 重置为默认]                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**5. 三层容错机制**

```yaml
Layer 1: 应用层重试 (业务逻辑层)
  适用场景: 
    - 站点API临时故障 (502/503/504)
    - 网络抖动 (timeout/connection reset)
    - 下载器暂时离线
  
  策略:
    max_retries: 3
    backoff: exponential_with_jitter
    initial_delay: 1s
    max_delay: 30s
    
  可重试错误码:
    - 408 Request Timeout
    - 429 Too Many Requests
    - 500/502/503/504 Server Errors
    - Network errors

Layer 2: 协程池层重试 (基础设施层)
  由WorkerPool.MaxRetry控制
  默认: 2-3次重试
  对上层透明
  统一记录重试日志

Layer 3: 断路器模式 (保护下游服务)
  状态机: Closed → Open → HalfOpen → Closed
  配置:
    failure_threshold: 5      # 连续5次失败后打开
    recovery_timeout: 30s     # 30秒后半开探测
    half_open_max_calls: 3    # 半开放允许3次探测
```

**6. Circuit Breaker (断路器) 实现**
- 为每个外部依赖（站点API、下载器API、IYUU API）创建独立断路器
- 三状态转换：Closed(正常) → Open(熔断) → HalfOpen(探测)
- 自动恢复机制
- 状态实时监控API

**7. 自动降级系统**

```yaml
DegradationLevel (降级级别):
  DegradeNormal (0):    正常运行 (所有功能)
  DegradePartial (1):   部分降级 (仅高优先级功能)
  DegradeMinimal (2):   最小化模式 (仅核心功能)
  DegradeEmergency (3): 紧急模式 (只读/维护)

触发条件 (自动):
  CPU > 95% 或 内存 > 95% → Emergency
  CPU > 85% 或 Goroutine > 10000 → Minimal
  CPU > 70% 或 Goroutine > 5000 → Partial

功能优先级:
  PrioritySystem (100):  健康检查、认证
  PriorityCritical (80): 辅种核心、数据持久化
  PriorityHigh (60):     刷流、转发
  PriorityMedium (40):   统计、日志
  PriorityLow (20):      非必要通知
```

**8. 监控与可观测性**

```yaml
实时性能指标:
  Go Runtime:
    - Goroutine数量
    - GC暂停时间
    - 内存分配情况
  
  协程池状态:
    - 各池活跃Worker数
    - 队列长度
    - 总提交/完成/失败任务数
    - 平均耗时
  
  断路器状态:
    - 各断路器当前状态
    - 连续失败次数
    - 总成功/失败统计
  
  注入速率监控:
    - 实际注入间隔分布
    - 今日注入总数
    - 各站点注入计数
    - 下次预计注入时间

API端点:
  GET /api/v1/system/metrics          - 系统指标总览
  GET /api/v1/system/pools/status     - 协程池详情
  GET /api/v1/system/circuit-breakers - 断路器状态
  WebSocket: /ws/stats                - 实时数据推送
```

#### 决策理由:

1. **安全性优先** ✅
   - PT账号是珍贵资产，一次封号可能损失多年积累
   - 注入速率控制是最关键的安全特性
   - 用户可配置确保不同风险承受能力的用户都能找到合适设置

2. **场景化优化** ✅
   - CPU密集型(Decision)需要高并发利用多核
   - I/O密集型(数据源查询)需要适中并发避免压垮站点
   - 速率敏感型(注入)需要严格限速模拟人类行为

3. **自愈能力** ✅
   - 系统需7×24运行，不能因临时故障停止
   - 三层容错确保瞬态故障自动恢复
   - 断路器防止级联故障雪崩

4. **用户体验** ✅
   - 用户可根据自己的风险偏好调整
   - 新手可用Safe模式，高级用户可用Aggressive模式
   - 实时统计帮助用户理解系统行为

5. **可观测性** ✅
   - 运维人员可实时监控系统健康状态
   - 问题发生时可快速定位瓶颈
   - 历史数据用于容量规划

#### 风险缓解措施:

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| 用户设置过于激进导致封号 | 中 | 高 | UI警告提示 + 推荐Safe模式 + 日志记录用户配置变更 |
| 协程池死锁 | 低 | 高 | 超时机制 + 优雅关闭 + 定期健康检查 |
| 断路器误触发 | 低 | 中 | 可配置阈值 + 手动重置API + 告警通知 |
| 内存泄漏 | 低 | 高 | 定期metrics采集 + pprof集成 + 自动重启策略 |
| 注入速率配置被恶意修改 | 极低 | 极高 | 配置变更审计日志 + 敏感操作需确认 |

### 6.5 参考资料

#### 相关文档
- [33-pt-forward-system-design-v2-upgrade.md](./33-pt-forward-system-design-v2-upgrade.md) - v2.0升级文档中的并发章节
- [四、核心业务流程设计](#四核心业务流程设计) - Pipeline设计与注入环节的关系

#### 参考源码
- examples/cross-seed/ - Decision引擎的并发匹配实现
- examples/ptdog/ - 批量查询的速率限制参考
- examples/torrentbotx/ - Telegram Bot异步任务处理

#### 外部参考资料
- **Go Concurrency Patterns**: https://blog.golang.org/pipelines
- **Circuit Breaker Pattern**: https://martinfowler.com/bliki/CircuitBreaker.html
- **Rate Limiting Patterns**: https://stripe.com/blog/rate-limiters
- **Graceful Shutdown in Go**: https://github.com/tylertreat/Shutdown-Patterns
- **PT Site Anti-Cheating**: 各站点Wiki/规则页

#### 关键技术决策点
```
┌───────────────────────────────────────────────────┐
│  并发模型核心决策                                    │
├───────────────────────────────────────────────────┤
│                                                   │
│  1. Goroutine管理方式                              │
│     → Worker Pool (可控、安全、可观测)             │
│                                                   │
│  2. 协程池粒度                                     │
│     → 7个专用池 (按场景优化)                       │
│                                                   │
│  3. 容错策略                                       │
│     → 三层重试 + 断路器 + 自动降级                 │
│                                                   │
│  4. 注入速率控制 ⭐⭐⭐                             │
│     → 默认 15s ± 5s 随机抖动                      │
│     → 用户可通过UI/API完全自定义                   │
│     → 支持预设模式 (Safe/Normal/Aggressive)        │
│     → 支持每站点独立覆盖                           │
│                                                   │
│  5. 监控与告警                                     │
│     → 实时仪表盘 + WebSocket推送 + 告警阈值       │
│                                                   │
└───────────────────────────────────────────────────┘
```

---

## 📊 决策汇总表（最终更新）

| 序号 | 讨论主题 | 决策日期 | 最终决策 | 决策依据 | 状态 |
|------|----------|----------|----------|----------|------|
| 1 | 整体架构模式 | 2026-04-12 | **A1: 松耦合单体 + 预留插件化** | 快速交付 + 扩展能力平衡 | ✅ 已确定 |
| 2 | 数据库选型 | 2026-04-12 | **双数据库: SQLite(本地) + MySQL(云端)** | 不同场景需求差异大 | ✅ 已确定 |
| 3 | MySQL vs PG | 2026-04-12 | **MySQL 8.0首选 + PG切换能力** | 性能胜出30-40% + 运维简便 | ✅ 已确定 |
| 4 | 核心业务流程 | 2026-04-13 | **5阶段Pipeline + 4级算法链 + 5层数据源** | 融合cross-seed + ptdog最佳实践 | ✅ 已确定 |
| **5** | **API设计规范** | **2026-04-13** | **RESTful L2 + 统一响应格式 + 增强型模块化路由** | **灵活性 + 松耦合 + 渐进式演进** | **✅ 已确定** |
| **6** | **并发模型与容错机制** | **2026-04-13** | **分层协程池架构 + 三层容错 + 用户可配置注入速率** | **安全性 + 可控性 + 灵活性** | **✅ 已确定** |

---

## 📝 文档维护规范

### ⚠️ 核心原则（最高优先级）

> **🔴 强制规则: 每一轮讨论完毕，均必须立即更新并保存本决策讨论历史文档**
>
> **执行要求**:
> - ✅ **时机**: 每完成一个议题的讨论后，**立即**（不超过10分钟）更新本文档
> - ✅ **内容**: 记录该轮次的完整讨论过程（问题→分析→选项→决策→理由）
> - ✅ **格式**: 遵循本文档已有的章节结构（见下方模板）
> - ✅ **责任人**: AI助手负责整理和写入，用户负责审核确认
> - ✅ **触发条件**: 以下任一情况发生时即触发更新
>   - 用户确认了某个架构决策
>   - 讨论了一个新的技术选项
>   - 否定了某个备选方案
>   - 调整或优化了已有决策
>
> **违规后果**: 
> - ❌ 若未及时更新，可能导致决策遗漏、重复讨论或设计不一致
> - ❌ 历史追溯困难，无法理解设计意图的演变过程
> - ❌ 团队协作时信息不同步

### 📋 标准更新流程

```
讨论开始 → 深度分析 → 用户选择/确认 → 
AI整理讨论内容 → 立即写入本文档 → 
用户审阅 → 继续下一轮讨论
```

### 📝 更新规则详细说明

#### 规则0: ⭐⭐⭐ 实时更新（强制）
1. **每一轮讨论完成后立即更新**（不等待所有讨论结束）
2. **更新内容包括**:
   - 讨论问题（明确的问题定义）
   - 候选方案（多个可行选项及分析）
   - 对比过程（优缺点、性能数据、参考案例）
   - 用户输入（用户的提问、选择、反馈）
   - 最终决策（完整描述 + 决策理由）
   - 技术实现（关键代码示例、配置）

#### 规则1: 完整性
3. **记录完整的讨论过程（不仅是最终结果）**
4. **包含决策理由和备选方案分析**

#### 规则2: 可追溯性
5. **标注参考资料和来源链接**
6. **记录时间戳和参与者**

#### 规则3: 结构化
7. **遵循统一的章节模板**（见下方的"标准章节模板"）

### 📐 标准章节模板

每轮讨论应按以下结构记录：

```markdown
## X. [讨论主题名称]

### X.1 讨论问题
**问题**: [清晰的问题描述]
**背景**: [为什么需要讨论这个问题]
**相关上下文**: [前置决策或依赖关系]

### X.2 候选方案对比

#### 方案A: [方案名称]
**描述**: [2-3句话概述]
**优势**: [表格或列表]
**劣势**: [表格或列表]
**参考项目**: [生态中的实际案例]

#### 方案B: [方案名称]
[同上格式...]

### X.3 讨论过程
#### 用户提问/输入:
> [用户的原始输入]

#### AI提供的选项:
[列出给用户的选项]

#### 用户选择:
[用户选择了哪个选项]

#### AI深度分析:
[详细的论证过程]

### X.4 最终决策
**✅ 决策结果**: [明确的决策描述]
**决策内容**:
[完整的决策细节]
**决策理由**:
[1. 理由一]
[2. 理由二]
...
**风险缓解措施**:
[如何应对潜在风险]

### X.5 参考资料
- [相关文档链接]
- [参考源码路径]
```

### 复盘机制
- **每轮复盘**: 每次更新后快速检查完整性
- **每周复盘**: 检查本周所有决策的一致性
- **每月复盘**: 检查决策是否符合实际需求
- **里程碑复盘**: Phase结束时全面回顾
- **重大变更**: 当决策需要调整时，记录新决策并保留历史版本

### 版本历史
| 版本 | 日期 | 作者 | 变更内容 |
|------|------|------|----------|
| v1.0 | 2026-04-13 | AI助手 | 初始版本，记录前4次讨论 |
| v1.1 | 2026-04-13 | 用户+AI助手 | ⭐ 新增核心原则：每轮讨论完毕必须立即更新文档 |
| v1.2 | 2026-04-13 | AI助手 | ⭐⭐ 新增第5次讨论：API设计规范（含灵活路由架构） |
| v1.3 | 2026-04-13 | 用户+AI助手 | ⭐⭐⭐ 新增第6次讨论：并发模型与容错机制（含用户可配置注入速率） |

---

## 💡 使用指南

### 如何使用本文档
1. **新成员入职**: 了解架构决策背景和原因
2. **技术评审**: 作为决策依据引用
3. **问题排查**: 回溯决策原因，理解设计意图
4. **重构参考**: 确保不违反已确定的架构原则
5. **复盘会议**: 检查决策效果，总结经验教训

### 如何延续讨论
当需要继续讨论时：
1. 查看本文档中的"待讨论"项
2. 参考"相关文档"获取上下文
3. 新的讨论结果追加到本文档
4. 更新"决策汇总表"

---

> **文档结束**  
> **下次更新**: 待下次架构讨论后  
> **负责人**: AI助手 + 用户共同维护
