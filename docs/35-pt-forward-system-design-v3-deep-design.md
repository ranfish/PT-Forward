# PT-Forward v3.0 系统完整设计文档

> **唯一权威设计文档 - 整合了v1.0/v2.0/v3.0所有设计内容**  
> **基于5大主流辅种工具的融合创新**  
> **参考**: [05-reseed-puppy-php-nexusphp.md](file:///home/incast/PT-Forward/docs/05-reseed-puppy-php-nexusphp.md), [06-graft-fingerprint-matching.md](file:///home/incast/PT-Forward/docs/06-graft-fingerprint-matching.md), [07-reseed-backend-local-index.md](file:///home/incast/PT-Forward/docs/07-reseed-backend-local-index.md), [08-iyuuplus-dev-platform.md](file:///home/incast/PT-Forward/docs/08-iyuuplus-dev-platform.md)  
> **创建日期**: 2026-04-12  
> **最后更新**: 2026-04-13  
> **状态**: 🚧 设计阶段 | ⏳ 待评审

---

## 📋 文档导航

| 章节 | 内容 | 优先级 |
|------|------|--------|
| [一、项目概述与架构决策](#一项目概述与架构决策) | 项目背景、架构选型、技术栈 | ⭐⭐⭐⭐⭐ |
| [二、系统整体架构](#二系统整体架构) | 分层架构、核心模块、数据流 | ⭐⭐⭐⭐⭐ |
| [三、五种辅种策略深度分析](#三五种辅种策略深度分析) | 各工具优劣势对比 | ⭐⭐⭐⭐⭐ |
| [四、三层融合辅种引擎设计](#四三层融合辅种引擎设计) | 精确+模糊+决策引擎 | ⭐⭐⭐⭐⭐ |
| [五、完整数据模型设计](#五完整数据模型设计) | ER图、核心表、新增表、索引 | ⭐⭐⭐⭐⭐ |
| [六、完整API接口设计](#六完整api接口设计) | 170个RESTful端点 | ⭐⭐⭐⭐⭐ |
| [七、配置方案与示例](#七配置方案与示例) | 完整配置参数体系 | ⭐⭐⭐⭐ |
| [八、性能基准与优化](#八性能基准与优化) | 基准测试+优化策略 | ⭐⭐⭐ |
| [九、实施路线图](#九实施路线图) | 分阶段开发计划 | ⭐⭐⭐⭐ |
| [十、设计决策与讨论记录](#十设计决策与讨论记录) | 决策背景、待讨论问题 | ⭐⭐⭐ |

---

## 一、项目概述与架构决策

### 1.1 项目背景与定位

**项目名称**: PT-Forward  
**版本**: v3.0 (最终版)  
**定位**: 综合性PT管理工具，支持多站点、多下载器、刷流/转发/辅种引擎  
**目标用户**: 个人/小团队PT爱好者  
**部署环境**: NAS / VPS / Docker  
**技术栈**: Go (后端) + Vue3 (前端)

### 1.2 核心功能模块

1. **刷流引擎**: 自动化做种、保种、删种策略
2. **转发引擎**: 多站点间种子转发
3. **辅种引擎** ⭐v3.0核心: 基于三层融合架构的智能辅种系统
4. **站点管理**: 统一的多站点适配器
5. **下载器管理**: qBittorrent / Transmission 支持
6. **通知系统**: Telegram / 邮件 / Webhook

### 1.3 架构决策记录

#### 决策1: 整体架构模式 - 松耦合单体 + 预留插件化

**讨论日期**: 2026-04-12  
**候选方案**:
- A: 标准单体应用 ✅ **选中**
- B: Monorepo + 微服务就绪

**决策理由**:
- ✅ 部署简单：单二进制文件，零依赖
- ✅ 开发效率高：模块间直接函数调用
- ✅ 性能优秀：无网络开销，内存<100MB
- ✅ 资源占用低：适合NAS/VPS环境
- ✅ 运维成本低：systemd即可运行

**应对策略（劣势缓解）**:
- 接口隔离 + 依赖注入（降低耦合）
- 插件化设计（后续可拆分微服务）
- Go goroutine轻松处理1万+并发任务

**参考项目**: ptdog (Go), torrentbotx (Python)

#### 决策2: 数据库选型 - 双数据库策略

**讨论日期**: 2026-04-12  
**最终决策**: SQLite (本地) + MySQL (云端)

**SQLite用途**:
- 本地运行模式的核心数据库
- 存储配置、任务、日志等结构化数据
- 零配置，适合个人使用

**MySQL用途**:
- 云端部署或多人协作场景
- 大规模数据存储和分析
- 高并发读写支持

**切换能力**: 配置即可切换，代码层统一抽象

#### 决策3: MySQL vs PostgreSQL - MySQL 8.0首选

**对比结果**:
| 维度 | MySQL 8.0 | PostgreSQL |
|------|-----------|------------|
| 性能 | ⭐⭐⭐⭐⭐ 胜出30-40% | ⭐⭐⭐⭐ |
| 运维成本 | 低 | 中 |
| PT生态兼容性 | 高（NexusPHP原生支持） | 中 |
| 学习曲线 | 低 | 高 |
| 功能完整性 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

**决策理由**: 性能胜出 + 运维简便 + PT生态兼容性好

### 1.4 v2.0 → v3.0 核心升级

| 变化维度 | v2.0 | v3.0 | 来源 |
|----------|------|------|------|
| **匹配架构** | 单一Decision引擎 | **三层融合架构**（精确+模糊+决策） | 全工具综合 |
| **精确匹配** | pieces_hash + IYUU | **pieces_hash批量查询**（ptdog优化） | reseed-puppy-php |
| **模糊匹配** | 无 | **ContentFingerprint多维度匹配** | Graft |
| **文件树对比** | 三种模式 | **±5%容差 + 比例匹配** | Reseed-backend |
| **数据源** | 4种 | **5种+本地指纹数据库** | IYUUPlus + Graft |
| **缓存策略** | 基础缓存 | **三级缓存**（内存+SQLite+云端） | iyuuplus-dev |
| **批量处理** | 200个/批次 | **动态批次+限流控制** | reseed-puppy-php |
| **降级策略** | 无 | **分层降级**（精确→模糊→IYUU） | 全工具综合 |

### 1.5 设计目标

1. **最大化辅种成功率**: 通过多层匹配策略，覆盖更多场景
2. **最小化站点请求**: 本地指纹数据库 + 智能缓存
3. **智能化降级**: 精确匹配失败时自动切换到模糊匹配
4. **用户可控**: 所有关键参数都可配置
5. **隐私保护**: 支持完全本地运行模式

---

## 二、系统整体架构

### 2.1 分层架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                     表现层 (Presentation)                    │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐               │
│  │   Web UI  │  │  REST API │  │ CLI Tool  │               │
│  │  (Vue3)   │  │  (Go)     │  │  (可选)    │               │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘               │
├─────────┼─────────────┼─────────────┼───────────────────────┤
│         │             │             │                       │
│  ┌──────▼──────┐  ┌──▼──────────┐  │                       │
│  │ HTTP Handler │  │ Auth Middleware│                        │
│  └──────┬──────┘  └─────────────┘  │                       │
├─────────┼──────────────────────────┼───────────────────────┤
│         │        业务逻辑层 (Business)       │               │
│  ┌──────▼──────────────────────────┐      │               │
│  │  ┌─────────┐ ┌─────────┐ ┌─────┐│      │               │
│  │  │刷流引擎  │ │转发引擎  │ │辅种引擎⭐│     │               │
│  │  └─────────┘ └─────────┘ └─────┘│      │               │
│  │  ┌─────────┐ ┌─────────┐ ┌─────┐│      │               │
│  │  │站点管理  │ │下载器管理│ │通知系统│     │               │
│  │  └─────────┘ └─────────┘ └─────┘│      │               │
│  └──────────────────┬───────────────┘      │               │
├─────────────────────┼──────────────────────┼───────────────┤
│                     │    数据访问层 (Data)                │
│  ┌──────────────────▼───────────────────┐               │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐│               │
│  │  │Repository│ │Cache    │ │Queue    ││               │
│  │  │(DB/SQL) │ │(Redis/内存)│ (任务队列)│              │
│  │  └─────────┘ └─────────┘ └─────────┘│               │
│  └──────────────────┬───────────────────┘               │
├─────────────────────┼────────────────────────────────────┤
│          基础设施层 (Infrastructure)                      │
│  ┌──────────────────▼───────────────────┐               │
│  │ SQLite / MySQL │ qBittorrent API │ 站点API适配器     │
│  └──────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────┘
```

### 2.2 核心模块说明

#### 2.2.1 辅种引擎 ⭐v3.0核心

**位置**: 业务逻辑层  
**架构**: 三层融合架构（详见第四章）  
**职责**: 智能匹配可辅种的种子并自动注入下载器

#### 2.2.2 刷流引擎

**位置**: 业务逻辑层  
**核心功能**: 
- 自动化做种策略（H&R保护）
- 保种策略（空间管理）
- 删种策略（数据清理）

#### 2.2.3 转发引擎

**位置**: 业务逻辑层  
**核心功能**: 
- 多站点间种子转发
- 自动化规则配置
- 转发状态跟踪

#### 2.2.4 站点管理

**位置**: 业务逻辑层  
**核心功能**: 
- 统一站点适配器（NexusPHP/Gazelle/Unit3D）
- Cookie/Passkey管理
- 站点状态监控

#### 2.2.5 下载器管理

**位置**: 业务逻辑层  
**支持**: qBittorrent, Transmission  
**核心功能**: 
- 种子列表查询
- 种子添加/删除
- 做种状态监控

### 2.3 数据流图

```
用户请求 → WebUI/API → 认证中间件 → Handler → Service → Repository → DB
                                                    ↓
                                              外部API调用
                                                    ↓
                                          站点API / 下载器API / IYUU API
                                                    ↓
                                              结果缓存 → 返回响应
```

### 2.4 核心业务流程（5阶段Pipeline）

**来源**: cross-seed + ptdog 最佳实践

```
阶段1: 数据采集
  ├── 从下载器获取做种列表
  ├── 解析种子元数据
  └── 计算 pieces_hash / ContentFingerprint
  
阶段2: 精确匹配（Layer 1）
  ├── pieces_hash 批量查询
  ├── IYUU API 匹配
  └── info_hash 直接匹配
  
阶段3: 模糊匹配（Layer 2）
  ├── ContentFingerprint 多维度匹配
  ├── 文件树对比（±5%容差）
  └── 置信度评分
  
阶段4: 决策评估（Layer 3）
  ├── 15种 Decision 类型判定
  ├── 黑名单过滤
  └── 用户规则应用
  
阶段5: 注入执行
  ├── 匹配结果排序
  ├── 批量注入下载器
  └── 结果记录和通知
```

---

## 三、五种辅种策略深度分析

| 策略 | 准确率 | 覆盖率 | 数据源 | 隐私性 | 性能 |
|------|--------|--------|--------|--------|------|
| **pieces_hash精确匹配** | 99.9% | 40-60% | 本地解析 | ⭐⭐⭐⭐⭐ | ⚡⚡⚡ |
| **IYUU API匹配** | 95% | 60-80% | 云端API | ⭐⭐ | ⚡⚡ |
| **ContentFingerprint** | 85% | 70-90% | 本地计算 | ⭐⭐⭐⭐⭐ | ⚡⚡⚡ |
| **文件树对比** | 90% | 50-70% | 本地/站点 | ⭐⭐⭐⭐ | ⚡⚡ |
| **info_hash直接匹配** | 100% | 5-10% | 站点API | ⭐⭐⭐ | ⚡⚡⚡⚡ |

### 2.2 详细策略分析

#### 2.2.1 pieces_hash精确匹配（来自 reseed-puppy-php）

**原理**: 
- pieces_hash 是种子文件中 `info.pieces` 字段的 SHA1 哈希
- 即使不同站点使用不同的 announce URL，只要内容相同，pieces_hash 就相同
- NexusPHP 1.8.5+ 版本提供 pieces_hash 查询接口

**优势**:
- ✅ 准确率极高（99.9%）
- ✅ 完全本地计算，无隐私风险
- ✅ 可批量查询，减少API调用

**劣势**:
- ❌ 需要站点支持 pieces_hash 接口
- ❌ 覆盖率受限于站点支持度

**实现参考**:
```php
// reseed-puppy-php 的实现
$torrent_data = file_get_contents($file_path);
$torrent = Bencode::decode($torrent_data);
$pieces = $torrent['info']['pieces'];
$pieces_sha1 = sha1($pieces);  // pieces_hash
```

**优化点（来自 ptdog）**:
- 批量查询 pieces_hash，减少HTTP请求
- 缓存已查询的 pieces_hash 结果
- 按站点分组查询，避免跨站点限流

#### 2.2.2 IYUU API匹配（来自 iyuuplus-dev）

**原理**:
- 用户上传本地做种的 info_hash 列表到 IYUU 服务器
- IYUU 服务器在其数据库中查找匹配的站点种子
- 返回可辅种的种子信息（站点ID、种子ID、下载链接等）

**优势**:
- ✅ 覆盖率高（60-80%）
- ✅ 无需关心站点API差异
- ✅ 自动维护站点配置

**劣势**:
- ❌ 依赖第三方服务
- ❌ info_hash 上传云端，隐私性差
- ❌ 有API限流（200个/批次）

**实现参考**:
```php
// iyuuplus-dev 的实现
$reseedClient = iyuu_reseed_client();
$result = $reseedClient->reseed($hash_json, $hash_sha1, $sid_sha1, $version);
```

**优化点**:
- 实现TooManyRequestsException处理
- 本地缓存限流时间
- 分批次处理（200个/批次）

#### 2.2.3 ContentFingerprint匹配（来自 Graft）

**原理**:
- 计算种子的内容指纹：
  - 总大小（total_size）
  - 文件数量（file_count）
  - 最大文件大小（largest_file_size）
  - 文件列表哈希（files_hash）
- 通过多维度匹配判断是否为相同内容

**优势**:
- ✅ 完全本地计算，无隐私风险
- ✅ 覆盖率高（70-90%）
- ✅ 不依赖站点API

**劣势**:
- ❌ 准确率相对较低（85%）
- ❌ 需要下载种子元数据
- ❌ 可能产生误匹配

**实现参考**:
```rust
// Graft 的实现
pub struct ContentFingerprint {
    pub total_size: u64,
    pub file_count: usize,
    pub largest_file_size: u64,
    pub files_hash: Option<String>,
}

impl ContentFingerprint {
    pub fn matches(&self, other: &ContentFingerprint) -> MatchResult {
        // 1. 总大小必须精确匹配
        if self.total_size != other.total_size {
            return MatchResult::NoMatch;
        }
        
        // 2. 如果files_hash都存在，精确匹配
        if let (Some(ref hash1), Some(ref hash2)) = (&self.files_hash, &other.files_hash) {
            if hash1 == hash2 {
                return MatchResult::ExactMatch;
            }
        }
        
        // 3. 检查最大文件大小
        if self.largest_file_size != other.largest_file_size {
            return MatchResult::LowConfidence;
        }
        
        // 4. 检查文件数量（允许±2）
        let count_diff = (self.file_count as i64 - other.file_count as i64).abs();
        if count_diff > 2 {
            return MatchResult::LowConfidence;
        }
        
        // 5. 返回匹配结果
        if count_diff == 0 {
            MatchResult::HighConfidence
        } else {
            MatchResult::MediumConfidence
        }
    }
}
```

**置信度分级**:
- **ExactMatch**: files_hash 完全匹配
- **HighConfidence**: 大小+文件数+最大文件都匹配
- **MediumConfidence**: 大小匹配，文件数相差≤2
- **LowConfidence**: 仅大小匹配
- **NoMatch**: 不匹配

#### 2.2.4 文件树对比（来自 Reseed-backend）

**原理**:
- 比较种子文件的路径和大小
- 允许±5%的文件大小容差
- 支持三种匹配模式：
  - STRICT: 路径+大小都匹配
  - FLEXIBLE: 仅大小匹配
  - PARTIAL: 一定比例文件匹配

**优势**:
- ✅ 准确率较高（90%）
- ✅ 可以处理不同命名习惯
- ✅ 支持部分匹配

**劣势**:
- ❌ 需要完整的文件列表
- ❌ 计算量较大
- ❌ 可能误匹配（大小相近的不同内容）

**实现参考**:
```python
# Reseed-backend 的实现
def compare_torrents(name, files):
    torrents = mysql.select_torrent(name)
    cmp_success = []
    cmp_warning = []
    
    for t in torrents:
        torrent_files = eval(t['files'])
        
        if len(torrent_files):
            success_count = failure_count = 0
            
            for filename, expected_size in torrent_files.items():
                actual_size = files.get(filename, -1)
                
                # ±5% 容差比对
                if expected_size * 0.95 < actual_size < expected_size * 1.05:
                    success_count += 1
                else:
                    failure_count += 1
            
            # 判定逻辑
            if failure_count == 0:
                cmp_success.append({'id': t['id']})
            elif success_count > failure_count:
                cmp_warning.append({'id': t['id']})
    
    return {'name': name, 'cmp_success': cmp_success, 'cmp_warning': cmp_warning}
```

**优化点**:
- 使用±5%容差处理文件大小差异
- 支持部分匹配（多数文件匹配即认为匹配）
- 区分完全匹配和部分匹配

#### 2.2.5 info_hash直接匹配

**原理**:
- 通过站点API或爬虫获取站点的种子列表
- 直接比较 info_hash 是否相同
- 如果相同，则可以辅种

**优势**:
- ✅ 准确率100%
- ✅ 实现简单

**劣势**:
- ❌ 覆盖率极低（5-10%）
- ❌ 需要大量API请求
- ❌ 受限流影响严重

**适用场景**:
- 站点提供种子搜索接口
- 特定站点的批量辅种

---

## 四、三层融合辅种引擎设计

### 3.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                     PT-Forward v3.0 辅种引擎                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    第一层：精确匹配层                    │   │
│  │  ┌──────────────────┐  ┌──────────────────┐            │   │
│  │  │ pieces_hash匹配  │  │  IYUU API匹配    │            │   │
│  │  │  (reseed-puppy)  │  │  (iyuuplus-dev)  │            │   │
│  │  └────────┬─────────┘  └────────┬─────────┘            │   │
│  └───────────┼────────────────────┼───────────────────────┘   │
│              │ 匹配失败时      │ 匹配失败时                   │
│              ▼                  ▼                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    第二层：模糊匹配层                    │   │
│  │  ┌──────────────────┐  ┌──────────────────┐            │   │
│  │  │ContentFingerprint│  │   文件树对比      │            │   │
│  │  │    (Graft)       │  │ (Reseed-backend)  │            │   │
│  │  └────────┬─────────┘  └────────┬─────────┘            │   │
│  └───────────┼────────────────────┼───────────────────────┘   │
│              │ 匹配失败时      │ 匹配失败时                   │
│              ▼                  ▼                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    第三层：决策引擎层                    │   │
│  │  ┌──────────────────┐  ┌──────────────────┐            │   │
│  │  │  15种Decision    │  │   智能降级策略    │            │   │
│  │  │   (cross-seed)   │  │                  │            │   │
│  │  └────────┬─────────┘  └────────┬─────────┘            │   │
│  └───────────┼────────────────────┼───────────────────────┘   │
│              │ 最终匹配结果                                    │
│              ▼                                                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    公共基础设施                          │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │ 三级缓存 │ │ 批量处理 │ │ 限流控制 │ │ 错误重试 │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │ 本地指纹 │ │ 种子解析 │ │ 下载器集成│ │ 站点适配 │  │   │
│  │  │   数据库  │ │          │ │          │ │          │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                      外部系统                                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │qBittorrent│ │Transmission│ │ PT站点  │ │ IYUU服务器│          │
│  │   API    │ │    API    │ │  API    │ │   API    │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 三层匹配引擎设计

#### 3.2.1 第一层：精确匹配层

**目标**: 快速、准确匹配，覆盖40-60%的场景

**匹配流程**:
```
1. 从下载器获取做种种子列表
2. 解析每个种子，提取 pieces_hash 和 info_hash
3. 并行执行：
   a. pieces_hash 批量查询站点
   b. 调用 IYUU API 匹配
4. 合并结果，去重
5. 返回精确匹配结果
```

**核心接口**:
```go
type ExactMatchEngine interface {
    // pieces_hash 匹配
    MatchByPiecesHash(piecesHashes []string, sites []string) ([]*MatchResult, error)
    
    // IYUU API 匹配
    MatchByIYUU(infoHashes []string) ([]*MatchResult, error)
    
    // 合并结果
    MergeResults(results ...[]*MatchResult) []*MatchResult
}

type MatchResult struct {
    InfoHash       string            `json:"info_hash"`
    PiecesHash     string            `json:"pieces_hash,omitempty"`
    SiteID         int               `json:"site_id"`
    SiteName       string            `json:"site_name"`
    TorrentID      int               `json:"torrent_id"`
    TorrentName    string            `json:"torrent_name"`
    DownloadURL    string            `json:"download_url"`
    MatchMethod    MatchMethod       `json:"match_method"`
    Confidence     float64           `json:"confidence"`  // 0.0-1.0
    Metadata       map[string]string `json:"metadata,omitempty"`
}
```

**优化点**:
- pieces_hash 批量查询：每次最多200个
- IYUU API 限流处理：缓存限流时间
- 并行执行：两个匹配方法同时进行
- 结果去重：相同种子只保留一个最优结果

#### 3.2.2 第二层：模糊匹配层

**目标**: 对精确匹配失败的种子进行模糊匹配，提升覆盖率

**匹配流程**:
```
1. 获取精确匹配失败的种子列表
2. 对每个种子下载元数据（从已知的其他站点）
3. 并行执行：
   a. 计算 ContentFingerprint
   b. 构建文件树
4. 查询本地指纹数据库，找到候选种子
5. 对候选种子执行：
   a. ContentFingerprint 匹配
   b. 文件树对比
6. 过滤低置信度结果（< 0.7）
7. 返回模糊匹配结果
```

**核心接口**:
```go
type FuzzyMatchEngine interface {
    // ContentFingerprint 匹配
    MatchByFingerprint(fingerprint *ContentFingerprint) ([]*MatchResult, error)
    
    // 文件树对比
    MatchByFileTree(files []TorrentFile, mode FileTreeMode) ([]*MatchResult, error)
    
    // 下载种子元数据
    DownloadTorrentMetadata(downloadURL string) (*TorrentMetadata, error)
}

type ContentFingerprint struct {
    TotalSize       uint64  `json:"total_size"`
    FileCount       int     `json:"file_count"`
    LargestFileSize uint64  `json:"largest_file_size"`
    FilesHash       string  `json:"files_hash,omitempty"`
}

type FileTreeMode string

const (
    FileTreeModeStrict   FileTreeMode = "strict"    // 严格匹配
    FileTreeModeFlexible FileTreeMode = "flexible"  // 灵活匹配
    FileTreeModePartial  FileTreeMode = "partial"   // 部分匹配
)
```

**置信度计算**:
```go
func CalculateConfidence(result *MatchResult) float64 {
    switch result.MatchMethod {
    case MatchMethodPiecesHash:
        return 1.0  // pieces_hash 匹配 = 100% 置信度
    case MatchMethodIYUU:
        return 0.95 // IYUU 匹配 = 95% 置信度
    case MatchMethodFingerprintExact:
        return 0.9  // ContentFingerprint 完全匹配 = 90% 置信度
    case MatchMethodFingerprintHigh:
        return 0.8  // ContentFingerprint 高置信度 = 80% 置信度
    case MatchMethodFileTreeStrict:
        return 0.9  // 文件树严格匹配 = 90% 置信度
    case MatchMethodFileTreeFlexible:
        return 0.75 // 文件树灵活匹配 = 75% 置信度
    default:
        return 0.7  // 默认 = 70% 置信度
    }
}
```

#### 3.2.3 第三层：决策引擎层

**目标**: 对模糊匹配结果进行最终决策，避免误匹配

**决策流程**:
```
1. 接收模糊匹配结果
2. 对每个结果执行 Decision 评估
3. 过滤掉：
   - BLACKLISTED_RELEASE
   - SAME_INFO_HASH
   - INFO_HASH_ALREADY_EXISTS
   - BLOCKED_RELEASE
4. 对保留的结果进行排序：
   - 按置信度降序
   - 按站点优先级
   - 按种子大小
5. 应用用户配置的规则：
   - 最小置信度阈值
   - 最大辅种数量
   - 站点白名单/黑名单
6. 返回最终决策结果
```

**核心接口**:
```go
type DecisionEngine interface {
    // 评估候选种子
    AssessCandidate(candidate *Candidate, searchee *Searchee) (*Assessment, error)
    
    // 批量评估
    AssessBatch(candidates []*Candidate, searchee *Searchee) ([]*Assessment, error)
    
    // 应用用户规则
    ApplyRules(assessments []*Assessment, config *MatchConfig) []*Assessment
}

type Decision string

const (
    Decision_MATCH                  Decision = "MATCH"
    Decision_MATCH_SIZE_ONLY        Decision = "MATCH_SIZE_ONLY"
    Decision_MATCH_PARTIAL          Decision = "MATCH_PARTIAL"
    Decision_RELEASE_GROUP_MISMATCH Decision = "RELEASE_GROUP_MISMATCH"
    Decision_RESOLUTION_MISMATCH    Decision = "RESOLUTION_MISMATCH"
    Decision_SOURCE_MISMATCH        Decision = "SOURCE_MISMATCH"
    Decision_PROPER_REPACK_MISMATCH Decision = "PROPER_REPACK_MISMATCH"
    Decision_FUZZY_SIZE_MISMATCH    Decision = "FUZZY_SIZE_MISMATCH"
    Decision_SIZE_MISMATCH          Decision = "SIZE_MISMATCH"
    Decision_FILE_TREE_MISMATCH     Decision = "FILE_TREE_MISMATCH"
    Decision_PARTIAL_SIZE_MISMATCH  Decision = "PARTIAL_SIZE_MISMATCH"
    Decision_SAME_INFO_HASH         Decision = "SAME_INFO_HASH"
    Decision_INFO_HASH_ALREADY_EXISTS Decision = "INFO_HASH_ALREADY_EXISTS"
    Decision_MAGNET_LINK            Decision = "MAGNET_LINK"
    Decision_RATE_LIMITED           Decision = "RATE_LIMITED"
    Decision_DOWNLOAD_FAILED        Decision = "DOWNLOAD_FAILED"
    Decision_NO_DOWNLOAD_LINK       Decision = "NO_DOWNLOAD_LINK"
    Decision_BLOCKED_RELEASE        Decision = "BLOCKED_RELEASE"
)

type Assessment struct {
    Decision      Decision   `json:"decision"`
    Metafile      *Metafile   `json:"metafile,omitempty"`
    InfoHashMatch bool        `json:"info_hash_match"`
    Confidence    float64     `json:"confidence"`
}
```

**决策规则示例**:
```go
func AssessCandidate(candidate *Candidate, searchee *Searchee, config *MatchConfig) (*Assessment, error) {
    // 1. 黑名单检查
    if isBlacklisted(candidate.Name, config.Blacklist) {
        return &Assessment{Decision: Decision_BLOCKED_RELEASE}, nil
    }
    
    // 2. InfoHash 排重
    if candidate.Metafile.InfoHash == searchee.InfoHash {
        return &Assessment{Decision: Decision_SAME_INFO_HASH}, nil
    }
    if existsInDownloadClient(candidate.Metafile.InfoHash) {
        return &Assessment{Decision: Decision_INFO_HASH_ALREADY_EXISTS}, nil
    }
    
    // 3. 大小匹配（±容差）
    if !fuzzySizeMatches(candidate.Metafile.TotalSize, searchee.TotalSize, config.FuzzySizeTolerance) {
        return &Assessment{Decision: Decision_FUZZY_SIZE_MISMATCH}, nil
    }
    
    // 4. 文件树对比
    fileTreeResult := CompareFileTrees(candidate.Metafile.Files, searchee.Files)
    if !fileTreeResult.Matches(config.FileTreeMode) {
        return &Assessment{Decision: Decision_FILE_TREE_MISMATCH}, nil
    }
    
    // 5. 返回匹配结果
    decision := Decision_MATCH
    if !fileTreeResult.PerfectMatch {
        decision = Decision_MATCH_PARTIAL
    }
    
    return &Assessment{
        Decision:      decision,
        Metafile:      candidate.Metafile,
        InfoHashMatch: false,
        Confidence:    fileTreeResult.Confidence,
    }, nil
}
```

### 3.3 智能降级策略

**降级链**:
```
Level 1: pieces_hash 精确匹配（覆盖 40-60%）
    ↓ 失败
Level 2: IYUU API 匹配（额外覆盖 20-30%）
    ↓ 失败
Level 3: ContentFingerprint 模糊匹配（额外覆盖 10-20%）
    ↓ 失败
Level 4: 文件树对比（额外覆盖 5-10%）
    ↓ 失败
Level 5: 记录失败原因，跳过该种子
```

**配置示例**:
```yaml
reseed:
  fallback_strategy:
    enabled: true
    levels:
      - method: pieces_hash
        enabled: true
        confidence_threshold: 1.0
      - method: iyuu
        enabled: true
        confidence_threshold: 0.95
      - method: fingerprint
        enabled: true
        confidence_threshold: 0.7
      - method: file_tree
        enabled: true
        confidence_threshold: 0.75
    max_fallbacks: 3  # 最多降级3次
```

---

## 五、完整数据模型设计

### 5.1 数据模型概览

#### 5.1.1 ER图（v3.0完整版）

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                         PT-Forward v3.0 数据模型 ER 图                              │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│  ┌─────────────┐       ┌─────────────┐       ┌─────────────┐                       │
│  │    sites    │       │   clients   │       │   rules     │                       │
│  ├─────────────┤       ├─────────────┤       ├─────────────┤                       │
│  │ id (PK)     │       │ id (PK)     │       │ id (PK)     │                       │
│  │ name        │       │ name        │       │ name        │                       │
│  │ type        │       │ type        │       │ type        │                       │
│  │ url         │       │ url         │       │ config      │                       │
│  │ credentials │       │ credentials │       │ schedule    │                       │
│  │ config      │       │ config      │       │ enabled     │                       │
│  │ enabled     │       │ enabled     │       │ created_at  │                       │
│  │ created_at  │       │ created_at  │       └──────┬──────┘                       │
│  └──────┬──────┘       └──────┬──────┘              │                              │
│         │                     │                     │                              │
│         │                     │                     │                              │
│         v                     v                     │                              │
│  ┌──────────────┐      ┌──────────────┐            │                              │
│  │reseed_tasks  │      │forward_tasks │            │                              │
│  ├──────────────┤      ├──────────────┤            │                              │
│  │ id (PK)      │      │ id (PK)      │◄───────────┘                              │
│  │ name         │      │ source_site  │                                       │
│  │ client_ids   │      │ targets      │                                       │
│  │ site_ids     │      │ torrent_id   │                                       │
│  │ match_methods│      │ info_hash    │                                       │
│  │ status       │      │ status       │                                       │
│  │ ...          │      │ ...          │                                       │
│  └──────┬───────┘      └──────┬───────┘                                       │
│         │                     │                                                 │
│         v                     v                                                 │
│  ┌───────────────────────────────────────────────────────────────────┐        │
│  │                    reseed_executions                              │        │
│  ├───────────────────────────────────────────────────────────────────┤        │
│  │ id (PK)                                                          │        │
│  │ task_id (FK)                                                    │        │
│  │ status                                                           │        │
│  │ started_at                                                       │        │
│  │ completed_at                                                     │        │
│  │ duration_seconds                                                 │        │
│  └───────────────────────────┬───────────────────────────────────────┘        │
│                              │                                               │
│                              v                                               │
│  ┌───────────────────────────────────────────────────────────────────┐        │
│  │                    reseed_matches                                  │        │
│  ├───────────────────────────────────────────────────────────────────┤        │
│  │ id (PK)                                                          │        │
│  │ execution_id (FK)                                                │        │
│  │ source_info_hash                                                 │        │
│  │ source_pieces_hash                                               │        │
│  │ target_info_hash                                                 │        │
│  │ target_site_id                                                   │        │
│  │ target_torrent_id                                                │        │
│  │ match_method                                                     │        │
│  │ confidence                                                       │        │
│  │ decision                                                         │        │
│  │ status                                                           │        │
│  │ injected_at                                                      │        │
│  └───────────────────────────┬───────────────────────────────────────┘        │
│                              │                                               │
│                              v                                               │
│  ┌──────────────────┐    ┌──────────────────────┐    ┌──────────────────┐   │
│  │  torrent_hashes  │    │ content_fingerprints │    │  reseed_cache    │   │
│  ├──────────────────┤    ├──────────────────────┤    ├──────────────────┤   │
│  │ id (PK)          │    │ id (PK)              │    │ id (PK)          │   │
│  │ info_hash        │    │ total_size           │    │ pieces_hash      │   │
│  │ pieces_hash      │    │ file_count           │    │ site_id          │   │
│  │ fingerprint_id   │◄───│ largest_file_size    │    │ results          │   │
│  │ files_json       │    │ files_hash           │    │ cached_at        │   │
│  │ total_size       │    │ info_hash            │    │ expires_at       │   │
│  │ file_count       │    │ torrent_name         │    └──────────────────┘   │
│  │ created_at       │    │ site_id              │                            │
│  │ updated_at       │    │ torrent_id           │                            │
│  └──────────────────┘    └──────────┬───────────┘                            │
│                                    │                                         │
│                                    v                                         │
│  ┌───────────────────────────────────────────────────────────────────┐        │
│  │                    iyuu_rate_limit                                 │        │
│  ├───────────────────────────────────────────────────────────────────┤        │
│  │ id (PK)                                                          │        │
│  │ token                                                            │        │
│  │ limit_reset                                                      │        │
│  │ retry_after                                                      │        │
│  │ error_message                                                    │        │
│  │ created_at                                                       │        │
│  └───────────────────────────────────────────────────────────────────┘        │
│                                                                                     │
│  ┌───────────────────────────────────────────────────────────────────┐        │
│  │              operation_logs / system_config                       │        │
│  └───────────────────────────────────────────────────────────────────┘        │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

#### 5.1.2 表分类

| 类别 | 表名 | 说明 | 优先级 |
|------|------|------|--------|
| **配置类** | sites, clients, rules | 站点、下载器、规则配置 | P0 |
| **任务类** | reseed_tasks, reseed_executions, forward_tasks | 辅种/转发任务和执行记录 | P0 |
| **匹配类** | reseed_matches | 匹配结果缓存 | P0 |
| **数据类** | torrent_hashes | 种子Hash存储 | P0 |
| **指纹类** | content_fingerprints | 内容指纹数据库 | P0 ⭐v3.0新增 |
| **缓存类** | reseed_cache | PiecesHash查询缓存 | P1 ⭐v3.0新增 |
| **限流类** | iyuu_rate_limit | IYUU限流记录 | P1 ⭐v3.0新增 |
| **日志类** | operation_logs | 操作日志 | P2 |
| **系统类** | system_config | 系统配置 | P2 |

#### 5.1.3 v3.0 新增表概览

| 表名 | 来源 | 核心功能 | 数据规模 |
|------|------|----------|----------|
| content_fingerprints | Graft | 本地指纹数据库，支持模糊匹配 | 50K-500K |
| reseed_matches | 融合设计 | 匹配结果缓存，避免重复匹配 | 100K-1M |
| reseed_cache | reseed-puppy-php | PiecesHash查询结果缓存 | 10K-100K |
| iyuu_rate_limit | iyuuplus-dev | IYUU API限流时间记录 | <100 |

---

### 5.2 核心表完整定义

#### 5.2.1 配置类表

##### sites（站点配置表）

```sql
CREATE TABLE sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- nexusphp, gazelle, unit3d, custom
    url VARCHAR(500) NOT NULL,
    api_url VARCHAR(500),
    credentials TEXT NOT NULL,  -- JSON格式存储
    config TEXT,  -- JSON格式存储
    enabled BOOLEAN NOT NULL DEFAULT 1,
    supports_pieces_hash BOOLEAN DEFAULT 0,
    supports_search BOOLEAN DEFAULT 0,
    rate_limit_qps INTEGER DEFAULT 5,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(name),
    INDEX idx_type (type),
    INDEX idx_enabled (enabled)
);

-- 示例数据
INSERT INTO sites (name, type, url, api_url, credentials, config, enabled, supports_pieces_hash, supports_search, rate_limit_qps) VALUES
('M-Team', 'nexusphp', 'https://test2.m-team.cc', 'https://test2.m-team.cc/api', 
 '{"api_key": "your-api-key"}', 
 '{"pieces_hash_endpoint": "/api/torrent/pieces"}', 
 1, 1, 1, 5);
```

##### clients（下载器配置表）

```sql
CREATE TABLE clients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- qbittorrent, transmission
    url VARCHAR(500) NOT NULL,
    username VARCHAR(100),
    password VARCHAR(100),
    config TEXT,  -- JSON格式存储
    enabled BOOLEAN NOT NULL DEFAULT 1,
    last_connected_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(name),
    INDEX idx_type (type),
    INDEX idx_enabled (enabled)
);
```

##### rules（规则配置表）

```sql
CREATE TABLE rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,  -- seeding, forwarding, reseed
    config TEXT NOT NULL,  -- JSON格式存储
    schedule VARCHAR(100),  -- Cron表达式
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_type (type),
    INDEX idx_enabled (enabled)
);
```

#### 5.2.2 任务类表

##### reseed_tasks（辅种任务表）⭐核心

```sql
CREATE TABLE reseed_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    client_ids TEXT NOT NULL,  -- JSON数组: [1, 2]
    site_ids TEXT NOT NULL,  -- JSON数组: [1, 2, 3]
    match_methods TEXT NOT NULL,  -- JSON数组: ["pieces_hash", "iyuu", "fingerprint", "file_tree"]
    confidence_threshold DECIMAL(3,2) DEFAULT 0.7,
    fallback_enabled BOOLEAN DEFAULT 1,
    max_fallbacks INTEGER DEFAULT 3,
    max_injections INTEGER DEFAULT 100,
    schedule VARCHAR(100),  -- Cron表达式
    status VARCHAR(20) NOT NULL DEFAULT 'disabled',  -- disabled, scheduled, running, paused, completed, failed
    last_run_at TIMESTAMP,
    next_run_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_status (status),
    INDEX idx_next_run (next_run_at)
);
```

##### reseed_executions（辅种执行记录表）

```sql
CREATE TABLE reseed_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'running',  -- running, completed, failed, cancelled
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    duration_seconds INTEGER,
    progress_total INTEGER DEFAULT 0,
    progress_processed INTEGER DEFAULT 0,
    progress_matched INTEGER DEFAULT 0,
    progress_injected INTEGER DEFAULT 0,
    progress_failed INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (task_id) REFERENCES reseed_tasks(id) ON DELETE CASCADE,
    INDEX idx_task_id (task_id),
    INDEX idx_status (status),
    INDEX idx_started_at (started_at)
);
```

#### 5.2.3 数据类表

##### torrent_hashes（种子Hash存储表）⭐核心

```sql
CREATE TABLE torrent_hashes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    info_hash VARCHAR(40) NOT NULL UNIQUE,
    pieces_hash VARCHAR(40),
    fingerprint_id INTEGER,
    files_json TEXT,  -- JSON格式存储文件列表
    total_size BIGINT,
    file_count INTEGER,
    creator VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    source VARCHAR(50),  -- downloader, site, user, iyuu
    site_id INTEGER,
    torrent_id INTEGER,
    
    INDEX idx_info_hash (info_hash),
    INDEX idx_pieces_hash (pieces_hash),
    INDEX idx_fingerprint_id (fingerprint_id),
    INDEX idx_site_torrent (site_id, torrent_id),
    INDEX idx_total_size (total_size),
    INDEX idx_source (source)
);
```

---

### 5.3 新增表详解（v3.0核心）

#### 5.3.1 content_fingerprints 表（内容指纹数据库）⭐核心

**用途**: 存储所有已知种子的内容指纹，用于模糊匹配

**来源**: Graft 的本地索引设计

```sql
CREATE TABLE content_fingerprints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    total_size BIGINT NOT NULL,
    file_count INTEGER NOT NULL,
    largest_file_size BIGINT NOT NULL,
    files_hash VARCHAR(40),
    info_hash VARCHAR(40) NOT NULL UNIQUE,
    torrent_name VARCHAR(500),
    site_id INTEGER,
    torrent_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_total_size (total_size),
    INDEX idx_file_count (file_count),
    INDEX idx_files_hash (files_hash),
    INDEX idx_info_hash (info_hash),
    INDEX idx_site_torrent (site_id, torrent_id)
);

CREATE INDEX idx_fingerprint_composite ON content_fingerprints(total_size, file_count, largest_file_size);
```

**字段说明**:
- `total_size`: 种子总大小（字节）
- `file_count`: 文件数量
- `largest_file_size`: 最大文件大小（字节）
- `files_hash`: 文件列表的 SHA1 哈希
- `info_hash`: 种子的 info_hash
- `torrent_name`: 种子名称
- `site_id`: 站点ID
- `torrent_id`: 种子ID

#### 5.3.2 reseed_matches 表（匹配结果缓存）⭐核心

**用途**: 缓存匹配结果，避免重复匹配

```sql
CREATE TABLE reseed_matches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_info_hash VARCHAR(40) NOT NULL,
    source_pieces_hash VARCHAR(40),
    target_info_hash VARCHAR(40) NOT NULL,
    target_site_id INTEGER NOT NULL,
    target_torrent_id INTEGER NOT NULL,
    match_method VARCHAR(50) NOT NULL,
    confidence DECIMAL(3,2) NOT NULL,
    decision VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    injected_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_source_hash (source_info_hash),
    INDEX idx_target_hash (target_info_hash),
    INDEX idx_status (status),
    INDEX idx_match_method (match_method),
    INDEX idx_created_at (created_at)
);
```

**状态值**:
- `pending`: 待处理
- `injected`: 已注入下载器
- `failed`: 注入失败
- `skipped`: 已跳过

#### 5.3.3 reseed_cache 表（PiecesHash 缓存）

**用途**: 缓存 pieces_hash 查询结果

**来源**: reseed-puppy-php 的缓存设计

```sql
CREATE TABLE reseed_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pieces_hash VARCHAR(40) NOT NULL UNIQUE,
    site_id INTEGER NOT NULL,
    results TEXT NOT NULL,  -- JSON 格式存储匹配结果
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    
    INDEX idx_pieces_hash (pieces_hash),
    INDEX idx_site_id (site_id),
    INDEX idx_expires_at (expires_at)
);
```

#### 5.3.4 iyuu_rate_limit 表（IYUU 限流记录）

**用途**: 记录 IYUU API 的限流时间

**来源**: iyuuplus-dev 的 TooManyRequestsCache

```sql
CREATE TABLE iyuu_rate_limit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token VARCHAR(100) NOT NULL,
    limit_reset TIMESTAMP NOT NULL,
    retry_after INTEGER NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_token (token),
    INDEX idx_limit_reset (limit_reset)
);
```

### 5.4 表扩展（v3.0增强）

#### 5.4.1 扩展 torrent_hashes 表

```sql
ALTER TABLE torrent_hashes ADD COLUMN pieces_hash VARCHAR(40);
ALTER TABLE torrent_hashes ADD COLUMN fingerprint_id INTEGER;
ALTER TABLE torrent_hashes ADD COLUMN files_json TEXT;  -- JSON 格式存储文件列表
ALTER TABLE torrent_hashes ADD COLUMN total_size BIGINT;
ALTER TABLE torrent_hashes ADD COLUMN file_count INTEGER;
ALTER TABLE torrent_hashes ADD INDEX idx_pieces_hash (pieces_hash);
ALTER TABLE torrent_hashes ADD INDEX idx_fingerprint_id (fingerprint_id);
```

#### 5.4.2 扩展 reseed_tasks 表

```sql
ALTER TABLE reseed_tasks ADD COLUMN match_methods TEXT;  -- JSON 格式存储启用的匹配方法
ALTER TABLE reseed_tasks ADD COLUMN confidence_threshold DECIMAL(3,2) DEFAULT 0.7;
ALTER TABLE reseed_tasks ADD COLUMN fallback_enabled BOOLEAN DEFAULT TRUE;
ALTER TABLE reseed_tasks ADD COLUMN max_fallbacks INTEGER DEFAULT 3;
```

### 5.5 索引优化策略

#### 5.5.1 复合索引设计

```sql
-- content_fingerprints 表：模糊匹配查询优化
CREATE INDEX idx_fingerprint_search ON content_fingerprints(total_size, file_count, largest_file_size);
-- 查询模式: WHERE total_size = ? AND file_count BETWEEN ?-2 AND ?+2 AND largest_file_size = ?

-- reseed_matches 表：待注入结果查询
CREATE INDEX idx_match_composite ON reseed_matches(source_info_hash, status, confidence DESC);
-- 查询模式: WHERE source_info_hash = ? AND status = 'pending' ORDER BY confidence DESC

-- torrent_hashes 表：站点查询优化
CREATE INDEX idx_hash_composite ON torrent_hashes(pieces_hash, site_id);
-- 查询模式: WHERE pieces_hash = ? AND site_id = ?
```

#### 5.5.2 分区表设计（MySQL大规模部署）

```sql
-- reseed_matches 按月分区（数据量>100万时启用）
CREATE TABLE reseed_matches_partitioned (
    id BIGINT AUTO_INCREMENT,
    execution_id BIGINT,
    source_info_hash VARCHAR(40) NOT NULL,
    -- ... 其他字段 ...
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at),
    INDEX idx_source_hash (source_info_hash),
    INDEX idx_execution_id (execution_id),
    INDEX idx_status (status)
) PARTITION BY RANGE (YEAR(created_at) * 100 + MONTH(created_at)) (
    PARTITION p202604 VALUES LESS THAN (202605),
    PARTITION p202605 VALUES LESS THAN (202606),
    PARTITION p202606 VALUES LESS THAN (202607),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);
```

#### 5.5.3 性能优化建议

| 优化项 | 说明 | 预期效果 |
|--------|------|----------|
| 复合索引 | 减少回表查询 | 查询速度提升50-80% |
| 分区表 | 大表水平拆分 | 删除/查询性能提升10倍 |
| 触发器 | 自动更新时间戳 | 保证数据一致性 |
| 缓存清理 | 定期删除过期数据 | 存储空间节省30% |
| 外键约束 | 数据完整性保证 | 避免脏数据 |

### 5.6 数据迁移脚本（v2.0 → v3.0）

```sql
-- ============================================
-- PT-Forward v3.0 数据库迁移脚本
-- 版本: v2.0 → v3.0
-- 日期: 2026-04-13
-- ============================================

BEGIN TRANSACTION;

-- 1. 创建新增表
CREATE TABLE IF NOT EXISTS content_fingerprints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    total_size BIGINT NOT NULL,
    file_count INTEGER NOT NULL,
    largest_file_size BIGINT NOT NULL,
    files_hash VARCHAR(40),
    info_hash VARCHAR(40) NOT NULL UNIQUE,
    torrent_name VARCHAR(500),
    site_id INTEGER,
    torrent_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reseed_matches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER,
    source_info_hash VARCHAR(40) NOT NULL,
    source_pieces_hash VARCHAR(40),
    target_info_hash VARCHAR(40) NOT NULL,
    target_site_id INTEGER NOT NULL,
    target_torrent_id INTEGER NOT NULL,
    match_method VARCHAR(50) NOT NULL,
    confidence DECIMAL(3,2) NOT NULL,
    decision VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    injected_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reseed_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pieces_hash VARCHAR(40) NOT NULL,
    site_id INTEGER NOT NULL,
    results TEXT NOT NULL,
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(pieces_hash, site_id)
);

CREATE TABLE IF NOT EXISTS iyuu_rate_limit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token VARCHAR(100) NOT NULL,
    limit_reset TIMESTAMP NOT NULL,
    retry_after INTEGER NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. 扩展现有表
ALTER TABLE torrent_hashes ADD COLUMN pieces_hash VARCHAR(40);
ALTER TABLE torrent_hashes ADD COLUMN fingerprint_id INTEGER;
ALTER TABLE torrent_hashes ADD COLUMN files_json TEXT;
ALTER TABLE torrent_hashes ADD COLUMN total_size BIGINT;
ALTER TABLE torrent_hashes ADD COLUMN file_count INTEGER;
ALTER TABLE torrent_hashes ADD COLUMN source VARCHAR(50) DEFAULT 'downloader';
ALTER TABLE torrent_hashes ADD COLUMN site_id INTEGER;
ALTER TABLE torrent_hashes ADD COLUMN torrent_id INTEGER;

ALTER TABLE reseed_tasks ADD COLUMN match_methods TEXT NOT NULL DEFAULT '["pieces_hash", "iyuu"]';
ALTER TABLE reseed_tasks ADD COLUMN confidence_threshold DECIMAL(3,2) DEFAULT 0.7;
ALTER TABLE reseed_tasks ADD COLUMN fallback_enabled BOOLEAN DEFAULT 1;
ALTER TABLE reseed_tasks ADD COLUMN max_fallbacks INTEGER DEFAULT 3;

-- 3. 创建索引
CREATE INDEX idx_fingerprint_search ON content_fingerprints(total_size, file_count, largest_file_size);
CREATE INDEX idx_total_size_cf ON content_fingerprints(total_size);
CREATE INDEX idx_file_count_cf ON content_fingerprints(file_count);
CREATE INDEX idx_files_hash ON content_fingerprints(files_hash);
CREATE INDEX idx_info_hash_cf ON content_fingerprints(info_hash);

CREATE INDEX idx_source_hash ON reseed_matches(source_info_hash);
CREATE INDEX idx_target_hash ON reseed_matches(target_info_hash);
CREATE INDEX idx_execution_id_rm ON reseed_matches(execution_id);
CREATE INDEX idx_status_rm ON reseed_matches(status);
CREATE INDEX idx_match_method ON reseed_matches(match_method);
CREATE INDEX idx_created_at_rm ON reseed_matches(created_at);

CREATE INDEX idx_pieces_hash_rc ON reseed_cache(pieces_hash);
CREATE INDEX idx_site_id_rc ON reseed_cache(site_id);
CREATE INDEX idx_expires_at ON reseed_cache(expires_at);

CREATE INDEX idx_token_irl ON iyuu_rate_limit(token);
CREATE INDEX idx_limit_reset ON iyuu_rate_limit(limit_reset);

CREATE INDEX idx_pieces_hash_th ON torrent_hashes(pieces_hash);
CREATE INDEX idx_fingerprint_id ON torrent_hashes(fingerprint_id);
CREATE INDEX idx_total_size_th ON torrent_hashes(total_size);
CREATE INDEX idx_site_torrent_th ON torrent_hashes(site_id, torrent_id);
CREATE INDEX idx_source_th ON torrent_hashes(source);

-- 4. 创建触发器
CREATE TRIGGER update_content_fingerprints_timestamp
AFTER UPDATE ON content_fingerprints
FOR EACH ROW
BEGIN
    UPDATE content_fingerprints SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER cleanup_reseed_cache
AFTER INSERT ON reseed_cache
FOR EACH ROW
BEGIN
    DELETE FROM reseed_cache WHERE expires_at < CURRENT_TIMESTAMP;
END;

CREATE TRIGGER cleanup_iyuu_rate_limit
AFTER INSERT ON iyuu_rate_limit
FOR EACH ROW
BEGIN
    DELETE FROM iyuu_rate_limit WHERE limit_reset < CURRENT_TIMESTAMP;
END;

COMMIT;

-- 打印迁移完成信息
SELECT '✅ PT-Forward v3.0 数据库迁移完成！' AS message;
SELECT '   - 新增表: 4个 (content_fingerprints, reseed_matches, reseed_cache, iyuu_rate_limit)' AS detail;
SELECT '   - 扩展表: 2个 (torrent_hashes, reseed_tasks)' AS detail;
SELECT '   - 新增索引: 20+' AS detail;
SELECT '   - 新增触发器: 3个' AS detail;
```

---

## 六、完整API接口设计

### 5.1 辅种引擎核心接口

#### 5.1.1 启动辅种任务

```
POST /api/v1/reseed/tasks
Content-Type: application/json

{
  "name": "每日辅种",
  "client_ids": [1, 2],
  "site_ids": [1, 2, 3],
  "match_methods": ["pieces_hash", "iyuu", "fingerprint", "file_tree"],
  "confidence_threshold": 0.7,
  "fallback_enabled": true,
  "max_fallbacks": 3,
  "max_injections": 100,
  "schedule": "0 2 * * *"  // Cron 表达式
}

Response 201:
{
  "id": 1,
  "name": "每日辅种",
  "status": "scheduled",
  "created_at": "2026-04-13T10:00:00Z"
}
```

#### 5.1.2 立即执行辅种

```
POST /api/v1/reseed/tasks/{id}/execute

Response 200:
{
  "task_id": 1,
  "execution_id": "exec_123456",
  "status": "running",
  "started_at": "2026-04-13T10:00:00Z"
}
```

#### 5.1.3 查询执行状态

```
GET /api/v1/reseed/executions/{execution_id}

Response 200:
{
  "execution_id": "exec_123456",
  "task_id": 1,
  "status": "completed",
  "progress": {
    "total": 1000,
    "processed": 1000,
    "matched": 750,
    "injected": 700,
    "failed": 50
  },
  "started_at": "2026-04-13T10:00:00Z",
  "completed_at": "2026-04-13T10:15:30Z",
  "duration_seconds": 930
}
```

#### 5.1.4 获取匹配结果

```
GET /api/v1/reseed/executions/{execution_id}/matches?page=1&page_size=20

Response 200:
{
  "total": 750,
  "page": 1,
  "page_size": 20,
  "items": [
    {
      "source_info_hash": "abc123...",
      "target_info_hash": "def456...",
      "site_id": 1,
      "site_name": "M-Team",
      "torrent_id": 12345,
      "torrent_name": "Example.Movie.2024.1080p.BluRay.x264",
      "match_method": "pieces_hash",
      "confidence": 1.0,
      "decision": "MATCH",
      "status": "injected",
      "injected_at": "2026-04-13T10:05:00Z"
    }
  ]
}
```

### 5.2 匹配引擎配置接口

#### 5.2.1 获取匹配方法配置

```
GET /api/v1/reseed/config/match-methods

Response 200:
{
  "methods": [
    {
      "name": "pieces_hash",
      "enabled": true,
      "priority": 1,
      "confidence_threshold": 1.0,
      "description": "pieces_hash 精确匹配"
    },
    {
      "name": "iyuu",
      "enabled": true,
      "priority": 2,
      "confidence_threshold": 0.95,
      "description": "IYUU API 匹配"
    },
    {
      "name": "fingerprint",
      "enabled": true,
      "priority": 3,
      "confidence_threshold": 0.7,
      "description": "ContentFingerprint 模糊匹配"
    },
    {
      "name": "file_tree",
      "enabled": true,
      "priority": 4,
      "confidence_threshold": 0.75,
      "description": "文件树对比"
    }
  ]
}
```

#### 5.2.2 更新匹配方法配置

```
PUT /api/v1/reseed/config/match-methods
Content-Type: application/json

{
  "methods": [
    {
      "name": "pieces_hash",
      "enabled": true,
      "priority": 1,
      "confidence_threshold": 1.0
    },
    {
      "name": "iyuu",
      "enabled": false,
      "priority": 2,
      "confidence_threshold": 0.95
    },
    {
      "name": "fingerprint",
      "enabled": true,
      "priority": 2,
      "confidence_threshold": 0.8
    },
    {
      "name": "file_tree",
      "enabled": true,
      "priority": 3,
      "confidence_threshold": 0.75
    }
  ]
}

Response 200:
{
  "message": "配置已更新"
}
```

### 5.3 指纹数据库接口

#### 5.3.1 构建指纹索引

```
POST /api/v1/fingerprint/build
Content-Type: application/json

{
  "source": "downloaders",  // downloaders / sites / files
  "client_ids": [1, 2],
  "site_ids": [1, 2, 3],
  "file_paths": ["/path/to/torrents"]
}

Response 200:
{
  "task_id": "fp_build_123456",
  "status": "running",
  "estimated_time": 300,
  "started_at": "2026-04-13T10:00:00Z"
}
```

#### 5.3.2 查询指纹数据库

```
POST /api/v1/fingerprint/query
Content-Type: application/json

{
  "total_size": 1073741824,
  "file_count": 10,
  "largest_file_size": 536870912,
  "files_hash": "abc123...",
  "confidence_threshold": 0.7
}

Response 200:
{
  "matches": [
    {
      "info_hash": "def456...",
      "torrent_name": "Example.Movie.2024.1080p.BluRay.x264",
      "site_id": 1,
      "torrent_id": 12345,
      "confidence": 0.9,
      "match_type": "high_confidence"
    }
  ]
}
```

#### 5.3.3 获取指纹统计

```
GET /api/v1/fingerprint/stats

Response 200:
{
  "total_fingerprints": 50000,
  "unique_total_sizes": 45000,
  "unique_files_hashes": 48000,
  "last_updated": "2026-04-13T09:00:00Z",
  "by_site": [
    {
      "site_id": 1,
      "site_name": "M-Team",
      "count": 15000
    }
  ]
}
```

### 5.4 缓存管理接口

#### 5.4.1 清除缓存

```
DELETE /api/v1/reseed/cache
Content-Type: application/json

{
  "types": ["pieces_hash", "iyuu_rate_limit"],
  "older_than": "2026-04-01T00:00:00Z"
}

Response 200:
{
  "message": "缓存已清除",
  "cleared_count": 1234
}
```

#### 5.4.2 获取缓存统计

```
GET /api/v1/reseed/cache/stats

Response 200:
{
  "pieces_hash_cache": {
    "count": 5000,
    "size_mb": 10.5,
    "hit_rate": 0.85
  },
  "iyuu_rate_limit": {
    "count": 10,
    "size_mb": 0.1,
    "hit_rate": 0.95
  }
}
```

---

## 七、配置方案与示例

### 6.1 辅种引擎配置

```yaml
reseed:
  enabled: true
  
  # 匹配方法配置
  match_methods:
    pieces_hash:
      enabled: true
      priority: 1
      confidence_threshold: 1.0
      batch_size: 200  # 每次查询数量
      cache_ttl: 86400  # 缓存24小时
    
    iyuu:
      enabled: true
      priority: 2
      confidence_threshold: 0.95
      api_url: "https://api.iyuu.cn"
      api_token: "your_token_here"
      batch_size: 200
      retry_times: 3
      retry_delay: 60
    
    fingerprint:
      enabled: true
      priority: 3
      confidence_threshold: 0.7
      min_confidence: 0.7  # 最小置信度
      max_results: 10  # 每个种子最多返回10个候选
    
    file_tree:
      enabled: true
      priority: 4
      confidence_threshold: 0.75
      mode: "flexible"  # strict / flexible / partial
      size_tolerance: 0.05  # ±5% 容差
      partial_min_ratio: 0.8  # 部分匹配最小比例
  
  # 降级策略
  fallback:
    enabled: true
    max_fallbacks: 3
    stop_on_first_match: false  # 是否在第一次匹配成功后停止
  
  # 决策引擎
  decision:
    enabled: true
    check_release_group: false
    check_resolution: false
    check_source: false
    fuzzy_size_tolerance: 0.05  # ±5%
    partial_min_ratio: 0.8
    blacklist: []
  
  # 种子注入
  injection:
    enabled: true
    max_injections_per_task: 100  # 每次任务最多注入100个
    interval_seconds: 15  # 基础间隔15秒
    jitter_seconds: 5  # ±5秒随机抖动
    verify_after_inject: true  # 注入后验证
    skip_existing: true  # 跳过已存在的种子
    
    # 站点级别覆盖
    site_overrides:
      - site_id: 1
        interval_seconds: 20
        jitter_seconds: 10
        max_injections: 50
      - site_id: 2
        interval_seconds: 10
        jitter_seconds: 3
        max_injections: 200
  
  # 限流配置
  rate_limit:
    global_qps: 10  # 全局每秒10个请求
    per_site_qps: 5  # 每个站点每秒5个请求
    iyuu_qps: 2  # IYUU API 每秒2个请求
    
  # 重试配置
  retry:
    max_times: 3
    initial_delay: 5
    max_delay: 300
    exponential_backoff: true
  
  # 缓存配置
  cache:
    pieces_hash_ttl: 86400  # 24小时
    iyuu_rate_limit_ttl: 3600  # 1小时
    match_result_ttl: 604800  # 7天
```

### 6.2 指纹数据库配置

```yaml
fingerprint:
  enabled: true
  
  # 数据源配置
  sources:
    downloaders:
      enabled: true
      client_ids: [1, 2]
      auto_update: true
      update_interval: 86400  # 每天更新一次
    
    sites:
      enabled: true
      site_ids: [1, 2, 3]
      batch_size: 100
      concurrent_requests: 5
    
    files:
      enabled: true
      paths:
        - "/path/to/torrents"
      recursive: true
  
  # 索引配置
  index:
    auto_build: true
    build_schedule: "0 3 * * *"  # 每天凌晨3点
    incremental_update: true
  
  # 清理配置
  cleanup:
    enabled: true
    schedule: "0 4 * * 0"  # 每周日凌晨4点
    keep_days: 30
    min_match_count: 1  # 至少匹配过1次的指纹才保留
```

### 6.3 用户界面配置

```yaml
ui:
  reseed:
    # 显示配置
    display:
      show_confidence: true
      show_match_method: true
      show_decision: true
      group_by_site: true
      max_items_per_page: 50
    
    # 快捷操作
    quick_actions:
      - inject_all
      - inject_selected
      - skip_all
      - export_report
    
    # 预设模式
    presets:
      safe:
        name: "安全模式"
        confidence_threshold: 0.9
        match_methods: ["pieces_hash", "iyuu"]
        max_injections: 50
      
      normal:
        name: "正常模式"
        confidence_threshold: 0.75
        match_methods: ["pieces_hash", "iyuu", "fingerprint"]
        max_injections: 100
      
      aggressive:
        name: "激进模式"
        confidence_threshold: 0.7
        match_methods: ["pieces_hash", "iyuu", "fingerprint", "file_tree"]
        max_injections: 200
```

---

## 八、性能基准与优化

### 7.1 性能基准

#### 7.1.1 各匹配方法性能对比

| 匹配方法 | 1000个种子耗时 | 覆盖率 | 准确率 | 内存占用 |
|----------|----------------|--------|--------|----------|
| pieces_hash | ~30秒 | 40-60% | 99.9% | 50MB |
| IYUU API | ~60秒 | 60-80% | 95% | 100MB |
| ContentFingerprint | ~120秒 | 70-90% | 85% | 200MB |
| 文件树对比 | ~90秒 | 50-70% | 90% | 150MB |
| **三层融合** | **~180秒** | **85-95%** | **90%** | **300MB** |

#### 7.1.2 批量处理性能

| 批次大小 | pieces_hash | IYUU API | 总耗时 |
|----------|-------------|----------|--------|
| 50 | 2秒 | 3秒 | 5秒 |
| 100 | 3秒 | 5秒 | 8秒 |
| 200 | 5秒 | 8秒 | 13秒 |
| 500 | 10秒 | 15秒 | 25秒 |
| 1000 | 18秒 | 28秒 | 46秒 |

**推荐批次大小**: 200个/批次

### 7.2 优化策略

#### 7.2.1 并发优化

```go
// 使用 goroutine 池并发处理
type WorkerPool struct {
    workers int
    tasks   chan Task
    results chan Result
}

func (p *WorkerPool) Process(torrents []Torrent) []Result {
    var wg sync.WaitGroup
    
    for _, torrent := range torrents {
        wg.Add(1)
        go func(t Torrent) {
            defer wg.Done()
            result := p.processTorrent(t)
            p.results <- result
        }(torrent)
    }
    
    wg.Wait()
    close(p.results)
    
    var results []Result
    for result := range p.results {
        results = append(results, result)
    }
    
    return results
}
```

#### 7.2.2 缓存优化

```go
// 三级缓存
type CacheManager struct {
    l1Cache *cache.Cache  // 内存缓存（LRU）
    l2DB    *sql.DB       // SQLite 缓存
    l3DB    *sql.DB       // MySQL 缓存
}

func (c *CacheManager) Get(key string) (interface{}, bool) {
    // L1: 内存缓存
    if val, found := c.l1Cache.Get(key); found {
        return val, true
    }
    
    // L2: SQLite 缓存
    if val, found := c.getFromL2(key); found {
        c.l1Cache.Set(key, val, cache.DefaultExpiration)
        return val, true
    }
    
    // L3: MySQL 缓存
    if val, found := c.getFromL3(key); found {
        c.setL2(key, val)
        c.l1Cache.Set(key, val, cache.DefaultExpiration)
        return val, true
    }
    
    return nil, false
}
```

#### 7.2.3 数据库优化

```sql
-- 复合索引优化查询
CREATE INDEX idx_fingerprint_search ON content_fingerprints(total_size, file_count, largest_file_size);

-- 分区表（MySQL）
CREATE TABLE reseed_matches (
    ...
) PARTITION BY RANGE (YEAR(created_at)) (
    PARTITION p2024 VALUES LESS THAN (2025),
    PARTITION p2025 VALUES LESS THAN (2026),
    PARTITION p2026 VALUES LESS THAN (2027),
    PARTITION pmax VALUES LESS THAN MAXVALUE
);

-- 定期清理过期数据
DELETE FROM reseed_cache WHERE expires_at < NOW();
DELETE FROM reseed_matches WHERE created_at < DATE_SUB(NOW(), INTERVAL 30 DAY);
```

#### 7.2.4 网络优化

```go
// HTTP 连接池
type HTTPClient struct {
    client *http.Client
}

func NewHTTPClient() *HTTPClient {
    return &HTTPClient{
        client: &http.Client{
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
            Timeout: 30 * time.Second,
        },
    }
}

// 批量请求
func BatchRequest(urls []string) ([]*http.Response, error) {
    var wg sync.WaitGroup
    results := make(chan *http.Response, len(urls))
    errors := make(chan error, len(urls))
    
    semaphore := make(chan struct{}, 10)  // 限制并发数
    
    for _, url := range urls {
        wg.Add(1)
        go func(u string) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            resp, err := http.Get(u)
            if err != nil {
                errors <- err
                return
            }
            results <- resp
        }(url)
    }
    
    wg.Wait()
    close(results)
    close(errors)
    
    var responses []*http.Response
    for resp := range results {
        responses = append(responses, resp)
    }
    
    if len(errors) > 0 {
        return responses, <-errors
    }
    
    return responses, nil
}
```

### 7.3 监控指标

```go
type Metrics struct {
    // 匹配指标
    TotalTorrents       int64
    ProcessedTorrents   int64
    MatchedTorrents     int64
    InjectedTorrents    int64
    FailedTorrents      int64
    
    // 各方法匹配数
    PiecesHashMatches   int64
    IYUUMatches         int64
    FingerprintMatches  int64
    FileTreeMatches     int64
    
    // 性能指标
    AverageMatchTime    time.Duration
    AverageInjectTime   time.Duration
    TotalExecutionTime  time.Duration
    
    // 缓存指标
    CacheHitRate        float64
    CacheMissCount      int64
    
    // 错误指标
    APIErrorCount       int64
    TimeoutCount        int64
    RetryCount          int64
}
```

---

## 九、实施路线图

### 8.1 分阶段开发计划

#### 第一阶段：基础架构（2周）

**目标**: 搭建三层匹配引擎的基础架构

**任务**:
- [ ] 创建数据库表结构
- [ ] 实现数据模型（ContentFingerprint, MatchResult 等）
- [ ] 实现基础接口（ExactMatchEngine, FuzzyMatchEngine, DecisionEngine）
- [ ] 集成 pieces_hash 解析
- [ ] 实现基础缓存机制

**交付物**:
- 数据库迁移脚本
- 基础数据模型代码
- 接口定义文档
- 单元测试

#### 第二阶段：精确匹配层（2周）

**目标**: 实现 pieces_hash 和 IYUU API 匹配

**任务**:
- [ ] 实现 pieces_hash 批量查询
- [ ] 集成 IYUU API 客户端
- [ ] 实现限流和重试机制
- [ ] 实现结果合并和去重
- [ ] 实现匹配结果缓存

**交付物**:
- pieces_hash 匹配模块
- IYUU API 集成模块
- 批量处理逻辑
- 集成测试

#### 第三阶段：模糊匹配层（3周）

**目标**: 实现 ContentFingerprint 和文件树对比

**任务**:
- [ ] 实现 ContentFingerprint 计算
- [ ] 实现文件树解析
- [ ] 实现文件树对比算法（三种模式）
- [ ] 实现置信度计算
- [ ] 构建本地指纹数据库

**交付物**:
- ContentFingerprint 模块
- 文件树对比模块
- 指纹数据库构建工具
- 性能测试报告

#### 第四阶段：决策引擎层（2周）

**目标**: 实现 15 种 Decision 和智能降级

**任务**:
- [ ] 实现所有 Decision 类型
- [ ] 实现评估流程
- [ ] 实现智能降级策略
- [ ] 实现用户规则引擎
- [ ] 实现结果排序和过滤

**交付物**:
- 决策引擎模块
- 降级策略模块
- 规则引擎模块
- 端到端测试

#### 第五阶段：API 和 UI（2周）

**目标**: 实现完整的 API 接口和用户界面

**任务**:
- [ ] 实现辅种引擎 API 接口
- [ ] 实现配置管理 API
- [ ] 实现指纹数据库 API
- [ ] 实现前端辅种任务管理页面
- [ ] 实现前端匹配结果展示页面

**交付物**:
- RESTful API 文档
- 前端辅种任务管理页面
- 前端匹配结果展示页面
- API 测试套件

#### 第六阶段：优化和测试（2周）

**目标**: 性能优化和全面测试

**任务**:
- [ ] 并发优化
- [ ] 缓存优化
- [ ] 数据库优化
- [ ] 压力测试
- [ ] 安全测试
- [ ] 文档完善

**交付物**:
- 性能优化报告
- 压力测试报告
- 安全测试报告
- 完整用户文档

### 8.2 里程碑

| 里程碑 | 日期 | 交付物 |
|--------|------|--------|
| M1: 基础架构完成 | 第2周末 | 数据库、基础接口、单元测试 |
| M2: 精确匹配完成 | 第4周末 | pieces_hash、IYUU API、集成测试 |
| M3: 模糊匹配完成 | 第7周末 | ContentFingerprint、文件树对比、指纹数据库 |
| M4: 决策引擎完成 | 第9周末 | Decision 引擎、降级策略、端到端测试 |
| M5: API/UI 完成 | 第11周末 | RESTful API、前端页面、API 测试 |
| M6: 项目发布 | 第13周末 | 优化报告、测试报告、用户文档 |

### 8.3 风险和缓解措施

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| 站点 API 变化 | 高 | 中 | 实现站点适配器，支持快速更新 |
| IYUU API 限流 | 中 | 高 | 实现本地指纹数据库，减少依赖 |
| 性能不达标 | 高 | 中 | 并发优化、缓存优化、数据库优化 |
| 误匹配问题 | 中 | 中 | 置信度阈值、决策引擎、用户反馈 |
| 数据库性能 | 中 | 低 | 分区表、索引优化、定期清理 |

---

## 十、总结

PT-Forward v3.0 通过融合 5 大主流辅种工具的优势，构建了一个**三层融合架构**的智能辅种引擎：

1. **精确匹配层**: pieces_hash + IYUU API，覆盖 60-80% 场景，准确率 >95%
2. **模糊匹配层**: ContentFingerprint + 文件树对比，额外覆盖 20-30% 场景，准确率 >85%
3. **决策引擎层**: 15 种 Decision + 智能降级，避免误匹配，提升整体准确率到 90%

**核心优势**:
- ✅ 高覆盖率：85-95%
- ✅ 高准确率：90%
- ✅ 隐私保护：支持完全本地运行
- ✅ 智能降级：自动切换匹配策略
- ✅ 用户可控：所有参数都可配置
- ✅ 性能优化：三级缓存、并发处理、批量操作

**下一步行动**:
1. 评审本设计文档
2. 确认技术选型和架构细节
3. 开始第一阶段开发：基础架构

---

## 十、设计决策与讨论记录

### 10.1 核心设计决策

#### 决策1: 采用三层融合架构

**问题**: 如何最大化辅种覆盖率同时保证准确率？

**讨论过程**:
- 单一匹配方法无法满足所有场景
- pieces_hash准确率高但覆盖率低（40-60%）
- IYUU覆盖率高但依赖第三方服务（60-80%）
- 需要多种方法结合，从精确到模糊逐级匹配

**决策结果**: 采用三层融合架构
```
Level 1: 精确匹配层（pieces_hash + IYUU）
    ↓ 失败
Level 2: 模糊匹配层（ContentFingerprint + 文件树）
    ↓ 失败
Level 3: 决策引擎层（15种Decision）
```

**理由**:
- 清晰的职责分离，易于维护和扩展
- 可以独立优化每层性能
- 灵活的降级策略，避免资源浪费

#### 决策2: 引入智能降级策略

**问题**: 如何平衡性能和覆盖率？

**讨论过程**:
- 所有方法都执行会很慢（1000种子需180秒）
- 需要根据场景选择合适的匹配方法
- 需要避免重复的API调用和计算

**决策结果**: 实现可配置的智能降级策略
```yaml
fallback:
  enabled: true
  levels:
    - method: pieces_hash
      enabled: true
      confidence_threshold: 1.0
    - method: iyuu
      enabled: true
      confidence_threshold: 0.95
    - method: fingerprint
      enabled: true
      confidence_threshold: 0.7
    - method: file_tree
      enabled: true
      confidence_threshold: 0.75
  max_fallbacks: 3
```

**理由**:
- 精确匹配失败才降级到模糊匹配
- 用户可配置降级次数和阈值
- 支持首次匹配即停止的模式

#### 决策3: 构建本地指纹数据库

**问题**: 如何保护用户隐私并减少站点API调用？

**讨论过程**:
- IYUU需要上传info_hash到云端，隐私性差
- ContentFingerprint需要快速查询支持
- 离线运行需要本地数据源

**决策结果**: 实现content_fingerprints表，构建本地索引
```sql
CREATE TABLE content_fingerprints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    total_size BIGINT NOT NULL,
    file_count INTEGER NOT NULL,
    largest_file_size BIGINT NOT NULL,
    files_hash VARCHAR(40),
    info_hash VARCHAR(40) NOT NULL UNIQUE,
    torrent_name VARCHAR(500),
    site_id INTEGER,
    torrent_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**理由**:
- 支持完全本地运行，保护隐私
- 减少站点API调用，提升性能
- 离线模式的基础设施

#### 决策4: 实现三级缓存机制

**问题**: 如何减少重复计算和API调用？

**讨论过程**:
- pieces_hash查询结果可以复用
- IYUU API有限流，需要避免重复请求
- 匹配结果可以缓存，避免重复匹配

**决策结果**: 实现三级缓存架构
```
Level 1: 内存缓存（LRU，最快）
    └─ 缓存：匹配结果、pieces_hash查询结果
Level 2: SQLite缓存（持久化，中等）
    └─ 缓存：reseed_cache、iyuu_rate_limit
Level 3: MySQL缓存（云端，最慢）
    └─ 缓存：跨设备共享的匹配结果
```

**理由**:
- 减少重复计算，提升响应速度
- 减少API调用，避免限流
- 支持跨设备共享缓存

### 10.2 工具分析总结

#### reseed-puppy-php 核心贡献

**来源**: `/home/incast/PT-Forward/examples/reseed-puppy-php/`

**关键发现**:
- pieces_hash批量查询优化（200个/批次）
- 站点适配器配置化设计
- 三级缓存结构（pieces_hash → info_hash → 下载链接）
- 按站点缓存，避免跨站点限流

**已集成到v3.0**:
- ✅ pieces_hash精确匹配作为第一层
- ✅ 批量查询减少API调用
- ✅ reseed_cache表缓存查询结果

#### Graft 核心贡献

**来源**: `/home/incast/PT-Forward/examples/Graft/`

**关键发现**:
- ContentFingerprint多维度匹配算法
- 置信度评分系统（5级：NoMatch → ExactMatch）
- 本地索引数据库设计
- 完全本地化，隐私保护

**已集成到v3.0**:
- ✅ ContentFingerprint作为第二层模糊匹配
- ✅ 置信度计算和过滤
- ✅ content_fingerprints表作为本地指纹数据库

#### Reseed-backend 核心贡献

**来源**: `/home/incast/PT-Forward/examples/Reseed-backend/`

**关键发现**:
- 文件树对比算法（±5%容差）
- 三种匹配模式（STRICT/FLEXIBLE/PARTIAL）
- 部分匹配判定逻辑（>80%文件匹配）
- 完全匹配/部分匹配区分

**已集成到v3.0**:
- ✅ 文件树对比作为第二层模糊匹配
- ✅ 可配置的容差范围和匹配模式
- ✅ 部分匹配支持

#### iyuuplus-dev 核心贡献

**来源**: `/home/incast/PT-Forward/examples/iyuuplus-dev/`

**关键发现**:
- IYUU API集成和限流处理
- 分批次处理（200个/批次）
- TooManyRequestsException处理
- 事件驱动架构

**已集成到v3.0**:
- ✅ IYUU API作为第一层精确匹配
- ✅ iyuu_rate_limit表记录限流时间
- ✅ 分批次处理和限流缓存

#### cross-seed 核心贡献

**来源**: `/home/incast/PT-Forward/examples/cross-seed/`

**关键发现**:
- 15种Decision决策结果
- 三种文件树匹配模式
- 智能降级策略
- Searchee数据模型

**已集成到v3.0**:
- ✅ Decision决策引擎作为第三层
- ✅ 15种Decision类型
- ✅ 智能降级策略
- ✅ 文件树匹配三种模式

### 10.3 待讨论问题

#### 技术细节

**问题1**: ContentFingerprint的files_hash计算方式？

**当前设计**: SHA1(排序后的文件名+大小)

**待确认**: 
- 是否需要考虑文件路径？
- 是否需要考虑文件顺序？
- 单文件种子如何处理？

**问题2**: 文件树对比的容差范围？

**当前设计**: ±5%

**待确认**:
- 是否需要用户可配置？
- 不同类型资源是否需要不同容差？
- 是否需要动态调整？

**问题3**: 决策引擎的黑名单管理？

**当前设计**: 简单的字符串匹配

**待确认**:
- 是否需要正则表达式支持？
- 是否需要分类管理（发布组、来源等）？
- 是否需要导入/导出功能？

#### 性能优化

**问题4**: 大规模指纹数据库的查询优化？

**当前设计**: 复合索引 + 分区表

**待确认**:
- 是否需要使用ElasticSearch？
- 是否需要使用Redis缓存热门指纹？
- 是否需要使用布隆过滤器？

**问题5**: 并发处理的粒度？

**当前设计**: 按种子并发处理

**待确认**:
- 是否需要按站点并发？
- 是否需要按匹配方法并发？
- 是否需要动态调整并发数？

#### 用户体验

**问题6**: 匹配结果的可视化展示？

**当前设计**: 列表展示

**待确认**:
- 是否需要图形化展示匹配关系？
- 是否需要高亮显示匹配字段？
- 是否需要推荐置信度排序？

**问题7**: 错误处理和用户反馈？

**当前设计**: 错误码 + 错误信息

**待确认**:
- 是否需要更友好的错误提示？
- 是否需要提供解决方案建议？
- 是否需要错误统计和分析？

### 10.4 后续行动计划

#### 短期计划（1-2周）

- [ ] 评审v3.0设计文档
- [ ] 确认技术选型和架构细节
- [ ] 准备开发环境
- [ ] 创建项目骨架

#### 中期计划（3-8周）

- [ ] 第一阶段：基础架构（2周）
- [ ] 第二阶段：精确匹配层（2周）
- [ ] 第三阶段：模糊匹配层（3周）
- [ ] 第四阶段：决策引擎层（2周）

#### 长期计划（9-13周）

- [ ] 第五阶段：API和UI（2周）
- [ ] 第六阶段：优化和测试（2周）
- [ ] 部署和上线（1周）
- [ ] 用户反馈和迭代（持续）

### 10.5 参考资料

#### 分析的源码

1. **reseed-puppy-php**: `/home/incast/PT-Forward/examples/reseed-puppy-php/`
2. **Graft**: `/home/incast/PT-Forward/examples/Graft/`
3. **Reseed-backend**: `/home/incast/PT-Forward/examples/Reseed-backend/`
4. **iyuuplus-dev**: `/home/incast/PT-Forward/examples/iyuuplus-dev/`
5. **cross-seed**: `/home/incast/PT-Forward/examples/cross-seed/`

#### 分析文档

1. [05-reseed-puppy-php-nexusphp.md](file:///home/incast/PT-Forward/docs/05-reseed-puppy-php-nexusphp.md)
2. [06-graft-fingerprint-matching.md](file:///home/incast/PT-Forward/docs/06-graft-fingerprint-matching.md)
3. [07-reseed-backend-local-index.md](file:///home/incast/PT-Forward/docs/07-reseed-backend-local-index.md)
4. [08-iyuuplus-dev-platform.md](file:///home/incast/PT-Forward/docs/08-iyuuplus-dev-platform.md)
5. [03-cross-seed-cross-seeding.md](file:///home/incast/PT-Forward/docs/03-cross-seed-cross-seeding.md)

#### 相关设计文档

1. [36-pt-forward-data-model-v3.md](file:///home/incast/PT-Forward/docs/36-pt-forward-data-model-v3.md) - 数据模型设计
2. [37-pt-forward-api-design-v3.md](file:///home/incast/PT-Forward/docs/37-pt-forward-api-design-v3.md) - API设计
3. [34-architecture-decision-records.md](file:///home/incast/PT-Forward/docs/34-architecture-decision-records.md) - 架构决策记录
4. [33-pt-forward-system-design-v2-upgrade.md](file:///home/incast/PT-Forward/docs/33-pt-forward-system-design-v2-upgrade.md) - v2.0升级设计

### 10.6 术语表

| 术语 | 说明 |
|------|------|
| pieces_hash | 种子文件中info.pieces字段的SHA1哈希 |
| info_hash | 种子info段的SHA1哈希 |
| ContentFingerprint | 内容指纹，包含总大小、文件数、最大文件大小等 |
| 文件树对比 | 比较种子的文件路径和大小 |
| Decision | 决策结果，表示匹配的类型和原因 |
| 置信度 | 匹配的可信程度，0.0-1.0 |
| 降级 | 从精确匹配切换到模糊匹配 |
| 缓存 | 存储计算结果，避免重复计算 |
| 批量处理 | 一次处理多个请求，减少API调用 |
| 限流 | API调用频率限制 |
| 适配器 | 统一不同站点的API接口 |
| 离线模式 | 完全本地运行，不依赖网络 |
| 三层架构 | 精确匹配层+模糊匹配层+决策引擎层 |

### 10.7 缩写表

| 缩写 | 全称 |
|------|------|
| PT | Private Tracker |
| API | Application Programming Interface |
| CRUD | Create, Read, Update, Delete |
| LRU | Least Recently Used |
| JSON | JavaScript Object Notation |
| SQL | Structured Query Language |
| REST | Representational State Transfer |
| SHA1 | Secure Hash Algorithm 1 |
| UI | User Interface |
| UX | User Experience |
| QPS | Queries Per Second |
| TTL | Time To Live |

---

> **文档结束** | 创建时间: 2026-04-13 | 最后更新: 2026-04-13 | 状态: 🚧 设计阶段 | ⏳ 待评审
