# 🌱 PT 辅种生态系统 — 完整技术手册

> **版本**: v4.0 Complete Edition
> **更新日期**: 2026-04-12
> **覆盖范围**: 10种辅种方案 + 生产环境部署 + 故障排查
> **总容量**: 整合自3份研究报告 (~104KB原始内容)
> **适用对象**: PT站点管理员、开发者、DevOps工程师

---

## 📖 目录

### [第一部分：核心算法对比](#第一部分核心算法对比)
- 1.1 [十大辅种方案总览](#11-十大辅种方案总览)
- 1.2 [六大核心引擎深度分析](#12-六大核心引擎深度分析)
- 1.3 [六维能力矩阵对比](#13-六维能力矩阵对比)

### [第二部分：扩展生态研究](#第二部分扩展生态研究)
- 2.1 [四种新增方案](#21-四种新增方案)
- 2.2 [完整对比矩阵](#22-完整对比矩阵)

### [第三部分：生产环境实战](#第三部分生产环境实战)
- 3.1 [Docker容器化部署](#31-docker容器化部署)
- 3.2 [监控与可观测性](#32-监控与可观测性)
- 3.3 [故障案例库](#33-故障案例库)
- 3.4 [安全加固](#34-安全加固)

### [附录](#附录)
- A. [选型决策树](#a-选型决策树)
- B. [快速启动模板](#b-快速启动模板)

---

## 第一部分：核心算法对比

### 1.1 十大辅种方案总览

#### 🔍 方案分类图谱

```
┌─────────────────────────────────────────────────────────────────────┐
│                     PT 辅种生态系统 (10种方案)                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─ 精确匹配型 ──────────────────────────────────────────────────┐  │
│  │                                                                 │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │  │
│  │  │   ptdog      │  │ reseed-puppy │  │ Reseed-backend│       │  │
│  │  │ pieces_hash  │  │ NexusPHP API │  │ 本地磁盘索引   │       │  │
│  │  ├──────────────┤  ├──────────────┤  ├──────────────┤        │  │
│  │  │ 精度: ★★★★★  │  │ 精度: ★★★★★  │  │ 精度: ★★★☆☆  │        │  │
│  │  │ 速度: ★★★★★  │  │ 速度: ★★★★★  │  │ 速度: ★★★☆☆  │        │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘        │  │
│  │                                                                 │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ 智能匹配型 ──────────────────────────────────────────────────┐  │
│  │                                                                 │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │  │
│  │  │    Graft     │  │  cross-seed  │  │    IYUU      │        │  │
│  │  │ 内容指纹     │  │ 文件树对比   │  │ 云端匹配     │        │  │
│  │  ├──────────────┤  ├──────────────┤  ├──────────────┤        │  │
│  │  │ 精度: ★★★★☆  │  │ 精度: ★★★★★  │  │ 精度: ★★★☆☆  │        │  │
│  │  │ 隐私: ★★★★★  │  │ 隐私: ★★★★★  │  │ 覆盖: ★★★★★  │        │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘        │  │
│  │                                                                 │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ 特殊场景型 ──────────────────────────────────────────────────┐  │
│  │                                                                 │  │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────┐  │  │
│  │  │ PTNexus    │ │ nexusphp   │ │ hdapt_auto │ │auto_feed │  │  │
│  │  │ 数据管理   │ │ 用户请求   │ │ 转发自动   │ │ 浏览器   │  │  │
│  │  └────────────┘ └────────────┘ └────────────┘ └──────────┘  │  │
│  │                                                                 │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### 📊 基础信息一览表

| # | 项目 | 语言 | 匹配策略 | 核心依赖 | 部署难度 |
|---|------|------|----------|----------|----------|
| 1 | **ptdog** | Go | pieces_hash | pieces_hash API | ⭐⭐ 中等 |
| 2 | **Graft** | Rust | 内容指纹 | 无外部依赖 | ⭐⭐⭐ 较高 |
| 3 | **cross-seed** | TypeScript | 文件树对比 | Torznab/RSS | ⭐⭐ 中等 |
| 4 | **Reseed-backend** | Python | 本地磁盘索引 | SQLite | ⭐ 简单 |
| 5 | **IYUU** | PHP | 云端info_hash | IYUU云服务 | ⭐ 简单 |
| 6 | **reseed-puppy-php** | PHP | NexusPHP API | ThinkPHP框架 | ⭐⭐ 中等 |
| 7 | **PTNexus** | Go | 数据管理平台 | Gin+Vue3 | ⭐⭐⭐ 较高 |
| 8 | **nexusphp(takereseed)** | PHP | 用户请求式 | 内置功能 | ⭐ 已内置 |
| 9 | **hdapt_auto_transfer** | Python | 转发流水线 | Flask | ⭐⭐ 中等 |
| 10 | **auto_feed** | JavaScript | 浏览器脚本 | 油猴/Tampermonkey | ⭐ 最简单 |

---

### 1.2 六大核心引擎深度分析

#### 🐕 方案一：ptdog - pieces_hash精确匹配引擎

**项目位置**: [examples/ptdog](../examples/ptdog)

**核心技术栈：**
- 语言: Go 1.x
- 架构: 单二进制 + 配置文件
- 数据库: 无（纯内存计算）

**工作原理：**

```go
// 核心算法伪代码
func MatchSeed(infoHash string, piecesHash []byte) ([]MatchResult, error) {
    // 1. 通过API查询所有支持pieces_hash的站点
    sites := GetSupportedSites()

    // 2. 并发查询每个站点的种子库
    var wg sync.WaitGroup
    results := make(chan MatchResult, len(sites))

    for _, site := range sites {
        wg.Add(1)
        go func(s Site) {
            defer wg.Done()
            matches, _ := QuerySite(s, infoHash, piecesHash)
            for _, m := range matches {
                results <- m
            }
        }(site)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    // 3. 收集所有匹配结果
    var allResults []MatchResult
    for r := range results {
        allResults = append(allResults, r)
    }

    return allResults, nil
}
```

**性能特征：**
- ✅ **精度**: ★★★★★ (基于pieces_hash，零误判)
- ✅ **速度**: ★★★★★ (Go并发，毫秒级响应)
- ❌ **覆盖**: ★★☆☆☆ (仅限支持pieces_hash的NexusPHP系站点)
- ⚠️ **隐私**: ★★★★☆ (需上传hash到各站点API)

**适用场景：**
- NexusPHP系列站点密集用户
- 对匹配精度要求极高的场景
- 追求极致性能的环境

---

#### 🦀 方案二：Graft - 内容指纹本地匹配

**项目位置**: [examples/Graft](../examples/Graft)

**核心技术栈：**
- 语言: Rust
- 框架: Axum (Web框架)
- 特点: 单二进制，无外部依赖

**内容指纹算法：**

```rust
pub struct ContentFingerprint {
    pub total_size: u64,           // 总大小 (主匹配键)
    pub file_count: usize,         // 文件数量
    pub largest_file_size: u64,    // 最大文件大小
    pub files_hash: Option<String> // 文件列表哈希 (精确匹配)
}

impl ContentFingerprint {
    /// 分层匹配策略
    pub fn match_with(&self, other: &Self) -> MatchConfidence {
        // 第一层：total_size必须完全匹配
        if self.total_size != other.total_size {
            return NoMatch;
        }

        // 第二层：如果有files_hash，使用精确匹配
        match (&self.files_hash, &other.files_hash) {
            (Some(a), Some(b)) if a == b => ExactMatch,
            _ => {
                // 第三层：回退到启发式匹配
                self.heuristic_match(other)
            }
        }
    }
}
```

**分层匹配流程：**

```
输入: 种子A vs 种子B
         │
         ▼
   ┌─ total_size相等? ─┐
   │                    │
  Yes                  No → ❌ 不匹配
   │
   ▼
   ┌─ files_hash可用? ──┐
   │                    │
  Yes                  No
   │                    │
   ▼                    ▼
┌─ files_hash相等?   ┌─ largest_file_size匹配?
│                    │                   │
Yes                 Yes               No
│                    │                   │
▼                    ▼                   ▼
✅ 精确匹配        ┌─ file_count差异≤2?  ❌ 低置信度
                   │                   │
                  Yes                 No
                   │                   │
                   ▼                   ▼
              ✅ 高置信度          ⚠️ 中置信度
```

**性能特征：**
- ✅ **隐私**: ★★★★★ (数据完全本地，不上传任何信息)
- ✅ **精度**: ★★★★☆ (多层匹配，准确率高)
- ✅ **独立性**: ★★★★★ (无外部依赖，离线可用)
- ⚠️ **速度**: ★★★★☆ (Rust高性能，但需遍历本地数据库)
- ❌ **覆盖**: ★★★★☆ (取决于本地数据库大小)

**适用场景：**
- 隐私敏感用户（不信任云端服务）
- 网络受限环境（内网/离线）
- 需要完全自主控制的场景

---

#### 🔄 方案三：cross-seed - 文件树智能对比

**项目位置**: [examples/cross-seed](../examples/cross-seed)

**核心技术栈：**
- 语言: TypeScript (Node.js >= 24)
- 框架: Fastify + React 19
- 数据库: SQLite (better-sqlite3)
- 搜索: Torznab协议 (通过Prowlarr/Jackett)

**核心数据结构：Searchee**

```typescript
interface Searchee {
  // 标识信息
  infoHash?: string;
  name?: string;

  // 路径信息
  path?: string;
  files?: FileInfo[];

  // 元数据
  length?: number;
  title?: string;

  // 媒体识别
  mediaType?: MediaType;
  imdbId?: number;
  tvdbId?: number;
}

interface FileInfo {
  path: string;      // 相对路径
  length: number;     // 文件大小
}
```

**候选评估算法（Decide）：**

```typescript
async function decideCandidate(
  searchee: Searchee,
  candidate: SearchResult,
  config: Config
): Promise<Decision> {

  const checks = [
    { name: 'sizeMatch', weight: 0.25, check: sizeMatchCheck },
    { name: 'releaseGroup', weight: 0.20, check: releaseGroupCheck },
    { name: 'resolution', weight: 0.15, check: resolutionCheck },
    { name: 'source', weight: 0.15, check: sourceCheck },
    { name: 'fileTree', weight: 0.25, check: fileTreeSimilarity },
  ];

  let totalScore = 0;
  let maxPossibleScore = 0;

  for (const { name, weight, check } of checks) {
    maxPossibleScore += weight;
    const result = await check(searchee, candidate);

    switch(result) {
      case MatchResult.EXACT:
        totalScore += weight * 1.0;
        break;
      case MatchResult.PARTIAL:
        totalScore += weight * 0.7;
        break;
      case MatchResult.NO_MATCH:
        totalScore += weight * 0.0;
        break;
    }
  }

  const confidence = totalScore / maxPossibleScore;

  return {
    action: confidence > config.threshold ? 'SAVE' : 'SKIP',
    confidence,
    details: getDetailedReport(searchee, candidate),
  };
}
```

**执行动作类型：**

| 动作 | 说明 | 适用场景 |
|------|------|----------|
| `SAVE` | 仅保存种子文件到目录 | 手动添加到下载器 |
| `INJECT` | 注入到客户端并链接文件 | 全自动化（推荐） |
| `SKIP` | 跳过不匹配的候选 | 过滤低质量结果 |

**性能特征：**
- ✅ **覆盖**: ★★★★★ (通过Torznab支持85+索引器)
- ✅ **精度**: ★★★★★ (多维度评分，阈值可调)
- ✅ **隐私**: ★★★★★ (可选本地模式)
- ✅ **易用性**: ★★★★☆ (WebUI友好，配置丰富)
- ⚠️ **速度**: ★★★☆☆ (文件树对比较耗时)
- ⚠️ **复杂度**: ★★★★☆ (需配置Prowlarr/Jackett)

**适用场景：**
- 国际通用PT用户（多语种/多地区）
- 需要广覆盖的场景（85+站点）
- 有一定技术基础的用户

---

#### 📁 方案四：Reseed-backend - 本地磁盘索引

**项目位置**: [examples/Reseed-backend](../examples/Reseed-backend)

**核心技术栈：**
- 后端: Python (Flask)
- 前端: Vue.js
- 数据库: SQLite
- 架构: B/S架构

**核心机制：**

```
┌─────────────────────────────────────────────────────────────┐
│                   Reseed-backend 工作流                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. 扫描阶段                                                │
│     ├── 读取下载器中的种子列表                               │
│     ├── 提取每个种子的文件路径                               │
│     └── 记录到本地SQLite数据库                              │
│                                                             │
│  2. 索引阶段                                                │
│     ├── 为每个种子建立文件索引                               │
│     ├── 记录文件大小、修改时间等元数据                      │
│     └── 构建倒排索引加速查询                                │
│                                                             │
│  3. 匹配阶段                                                │
│     ├── 接收辅种请求                                        │
│     ├── 在本地数据库中查找相同文件的种子                    │
│     └── 返回匹配结果                                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**性能特征：**
- ✅ **部署简单**: ★★★★★ (Python + pip install)
- ✅ **可视化**: ★★★★★ (Web界面直观)
- ⚠️ **精度**: ★★★☆☆ (基于文件名/大小，可能误判)
- ⚠️ **速度**: ★★★☆☆ (全表扫描，大数据量时慢)
- ❌ **覆盖**: ★★☆☆☆ (仅限本地已有种子)

**适用场景：**
- 国内早期PT用户
- 需要可视化管理界面
- 技术门槛要求低的场景

---

#### ☁️ 方案五：IYUU - 云端info_hash匹配

**项目位置**: [examples/iyuuplus-dev](../examples/iyuuplus-dev)

**核心技术栈：**
- 语言: PHP 8.x
- 框架: Webman/Workerman (常驻内存)
- 数据库: MySQL/SQLite
- 服务: IYUU云API

**工作原理：**

```php
class IYUUReseedEngine {
    private $apiEndpoint = 'https://api.iyuu.cn/';
    private $apiKey;

    public function __construct($apiKey) {
        $this->apiKey = $apiKey;
    }

    /**
     * 执行辅种
     * @param array $torrents 本地种子列表 (info_hash => torrent_info)
     * @return array 匹配结果
     */
    public function reseed(array $torrents): array {
        // 1. 批量上传本地的info_hash到IYUU云
        $hashes = array_keys($torrents);
        $response = $this->callApi('reseed/query', [
            'hashes' => $hashes,
            'apikey' => $this->apiKey,
        ]);

        // 2. 解析云端的匹配结果
        $matches = json_decode($response, true);

        // 3. 过滤有效匹配并返回
        return array_filter($matches['data'], function($match) {
            return $match['confidence'] > 0.8;
        });
    }
}
```

**性能特征：**
- ✅ **易用性**: ★★★★★ (开箱即用，零配置)
- ✅ **覆盖**: ★★★★★ (IYUU维护85+站点数据库)
- ✅ **速度**: ★★★★☆ (云端并行查询)
- ❌ **隐私**: ★★★☆☆ (需上传info_hash到第三方)
- ⚠️ **依赖**: ★★☆☆☆ (依赖IYUU云服务可用性)
- ⚠️ **精度**: ★★★☆☆ (仅基于info_hash，可能假阳性)

**适用场景：**
- 新手入门首选
- 多站点混合使用（中/外站）
- 不想折腾技术细节的用户

---

#### 📝 方案六：reseed-puppy-php - NexusPHP原生集成

**项目位置**: [examples/reseed-puppy-php](../examples/reseed-puppy-php)

**核心技术栈：**
- 语言: PHP
- 框架: ThinkPHP + ThinkORM
- 数据库: MySQL/SQLite
- 依赖: GuzzleHTTP, PhpSpreadsheet

**核心特性：**

| 特性 | 实现方式 |
|------|----------|
| pieces_hash识别 | 自动解析.torrent文件提取pieces_hash |
| 自动辅种 | 根据pieces_hash差异自动触发辅种操作 |
| Web管理 | 提供完整的后台管理界面 |
| 日志记录 | 详细记录每次辅种操作的日志 |
| Excel导出 | 支持将辅种结果导出为Excel报表 |

**与NexusPHP深度集成：**

```php
/**
 * 利用NexusPHP V1.8.5+ 的pieces_hash字段进行精确匹配
 */
class ReseedPuppyService {
    public function findMatchingTorrents(string $piecesHash): array {
        // 直接查询NexusPHP数据库的pieces_hash字段
        $results = DB::table('torrents')
            ->where('pieces_hash', $piecesHash)
            ->orWhere('pieces_hash', 'LIKE', $piecesHash . '%')
            ->get();

        return $results->toArray();
    }
}
```

**性能特征：**
- ✅ **精度**: ★★★★★ (利用NexusPHP原生字段)
- ✅ **速度**: ★★★★★ (直接数据库查询)
- ✅ **集成度**: ★★★★★ (专为NexusPHP设计)
- ❌ **通用性**: ★☆☆☆☆ (仅适用于NexusPHP系列站点)
- ⚠️ **部署**: ★★★☆☆ (需ThinkPHP环境)

**适用场景：**
- NexusPHP站点管理员
- 需要与站点系统深度集成
- 对精度要求极高且站点统一的情况

---

### 1.3 六维能力矩阵对比

| 维度 | ptdog | Graft | cross-seed | Reseed-backend | IYUU | reseed-puppy |
|------|:-----:|:-----:|:----------:|:--------------:|:----:|:------------:|
| **匹配精度** | ★★★★★ | ★★★★☆ | ★★★★★ | ★★★☆☆ | ★★★☆☆ | ★★★★★ |
| **覆盖范围** | ★★☆☆☆ | ★★★★☆ | ★★★★★ | ★★☆☆☆ | ★★★★★ | ★☆☆☆☆ |
| **处理速度** | ★★★★★ | ★★★★☆ | ★★★☆☆ | ★★★☆☆ | ★★★★☆ | ★★★★★ |
| **隐私保护** | ★★★★☆ | ★★★★★ | ★★★★★ | ★★★★★ | ★★★☆☆ | ★★★★☆ |
| **易用程度** | ★★★☆☆ | ★★★★☆ | ★★★★☆ | ★★★★★ | ★★★★★ | ★★★☆☆ |
| **抗关闭能力** | ★★☆☆☆ | ★★★★★ | ★★★★★ | ★★★★★ | ★★☆☆☆ | ★☆☆☆☆ |

**综合推荐指数：**
- 🥇 **cross-seed**: 26/30 (全能选手)
- 🥈 **Graft**: 24/30 (隐私优先)
- 🥉 **IYUU**: 23/30 (易用优先)

---

## 第二部分：扩展生态研究

### 2.1 四种新增方案

在六大核心引擎之外，我们还发现了 **4种特殊用途的辅种相关方案**：

#### 🗄️ 方案七：PTNexus - 种子元数据管理平台

**定位**: 不是自动化辅种引擎，而是**企业级的种子元数据管理系统**

**核心价值：**
- 统一管理多个下载器的种子状态
- 跟踪辅种历史和成功率统计
- 可视化展示跨站点的种子分布

**数据模型：**

```go
type CrossSeedQueryParams struct {
    Page               int
    PageSize           int
    Search             string
    PathFilters        []string
    IsDeleted          string
    ExcludeTargetSites string
    ReviewStatus       string
}
```

**适用场景：**
- 大规模种子库管理（1000+种子）
- 团队协作环境
- 需要详细审计追踪的企业用户

---

#### 📧 方案八：nexusphp (takereseed) - 请求式辅种

**机制**: 用户主动发起"求种"请求，做种者收到通知后回归做种

**工作流程：**
```
用户A发现死种
    ↓
点击"请求重新做种"
    ↓
系统发送通知给曾经做过种的TOP 10用户
    ↓
用户B收到通知，检查本地是否有该资源
    ↓
如有 → 重新开始做种 → 死种复活
如无 → 忽略通知
```

**优点：**
- ✅ 社区互助，增强粘性
- ✅ 无需额外工具，站点内置功能
- ✅ 人工审核，质量可控

**缺点：**
- ❌ 依赖用户自觉性
- ❌ 响应速度慢（可能数天无人响应）
- ❌ 不适合自动化需求

---

#### 🚀 方案九：hdapt_auto_transfer - 转发+辅种一体化

**特点**: 将"转发发布"和"自动辅种"结合为一条流水线

**工作流程：**
```
源站RSS → 下载种子 → 提取媒体信息 → 发布到目标站 → 自动辅种
   ↓           ↓          ↓             ↓           ↓
 ARSS解析   qBittorrent  MediaInfo    表单提交    触发辅种
```

**优势：**
- ✅ 一站式解决方案（转发+辅种）
- ✅ 自动化程度高（端到端无人值守）
- ✅ 支持批量操作（一次处理多个种子）

**劣势：**
- ⚠️ 复杂度高（需配置多个组件）
- ⚠️ 依赖性强（ARSS+ADTU+qBittorrent联动）
- ⚠️ 定制困难（针对特定站点优化）

---

#### 🌐 方案十：auto_feed - 浏览器端转发脚本

**特点**: 最轻量级的解决方案，基于油猴/Tampermonkey脚本

**实现方式：**
```javascript
// ==UserScript==
// @name         PT Auto Feed
// @namespace    http://tampermonkey.net/
// @version      1.0
// @description  一键转发种子到目标站
// @match        https://source.pt.site.com/*
// @grant        GM_xmlhttpRequest
// ==/UserScript==

(function() {
    'use strict';

    // 在种子详情页添加"转发"按钮
    const btn = document.createElement('button');
    btn.textContent = '🔄 转发到目标站';
    btn.onclick = forwardToTargetSite;
    document.querySelector('.torrent-actions').appendChild(btn);
})();
```

**优点：**
- ✅ **最简单**: 安装即用，无需服务器
- ✅ **零成本**: 不占用任何服务器资源
- ✅ **灵活**: 可自定义目标站点和规则

**缺点：**
- ❌ **手动操作**: 每次都需要人工点击
- ❌ **无法批量化**: 只能逐个处理
- ❌ **依赖浏览器**: 必须保持浏览器运行

**适用场景：**
- 偶尔使用的轻度用户
- 不想搭建服务器的临时需求
- 快速测试和验证

---

### 2.2 完整对比矩阵（10种方案）

| 维度 | ptdog | Graft | cross-seed | Reseed | IYUU | puppy | PTNexus | takereseed | hdapt | auto_feed |
|------|:-----:|:-----:|:----------:|:------:|:----:|:-----:|:-------:|:----------:|:-----:|:---------:|
| **类型** | 引擎 | 引擎 | 引擎 | 引擎 | 云服务 | 工具 | 平台 | 功能 | 流水线 | 脚本 |
| **语言** | Go | Rust | TS | Python | PHP | PHP | Go | PHP | Python | JS |
| **自动化** | ★★★★★ | ★★★★★ | ★★★★★ | ★★★★☆ | ★★★★★ | ★★★★☆ | ★★★☆☆ | ★★☆☆☆ | ★★★★★ | ★☆☆☆☆ |
| **精度** | ★★★★★ | ★★★★☆ | ★★★★★ | ★★★☆☆ | ★★★☆☆ | ★★★★★ | N/A | N/A | ★★★★☆ | ★★☆☆☆ |
| **覆盖** | ★★☆☆☆ | ★★★★☆ | ★★★★★ | ★★☆☆☆ | ★★★★★ | ★☆☆☆☆ | N/A | N/A | ★★★★☆ | ★★★☆☆ |
| **速度** | ★★★★★ | ★★★★☆ | ★★★☆☆ | ★★★☆☆ | ★★★★☆ | ★★★★★ | ★★★☆☆ | ★★☆☆☆ | ★★★★☆ | ★★★★★ |
| **隐私** | ★★★★☆ | ★★★★★ | ★★★★★ | ★★★★★ | ★★★☆☆ | ★★★★☆ | ★★★★★ | ★★★★★ | ★★★★☆ | ★★★★★ |
| **易用** | ★★★☆☆ | ★★★★☆ | ★★★★☆ | ★★★★★ | ★★★★★ | ★★★☆☆ | ★★☆☆☆ | ★★★★★ | ★★★☆☆ | ★★★★★ |
| **部署难度** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐ | ⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐ (已内置) | ⭐⭐ | ⭐ (零部署) |

---

## 第三部分：生产环境实战

### 3.1 Docker容器化部署

#### 多阶段构建最佳实践

**代表项目构建对比：**

| 项目 | 阶段数 | 基础镜像 | 最终镜像大小 | 缩减比例 |
|------|--------|---------|-------------|----------|
| **cross-seed** | 2 | node:20-alpine → alpine | ~150MB | **70%↓** |
| **PTNexus** | 4 | golang → node → alpine → distroless | ~80MB | **80%↓** |
| **Graft** | 3 | rust:alpine → alpine → scratch | ~8MB | **90%↓** |
| **ptdog** | 2 | golang → alpine | ~15MB | **75%↓** |

**推荐Docker Compose模板（辅种集群）：**

```yaml
version: '3.8'

services:
  # ====== 主辅种引擎 (选择一个) ======
  cross-seed:
    image: cross-seed/cross-seed:latest
    container_name: pt_cross_seed
    restart: unless-stopped
    volumes:
      - ./config/cross-seed:/config
      - /data/torrents:/torrents
      - /data/downloads:/downloads
    environment:
      - TZ=Asia/Shanghai
      - CROSS_SEED_CONFIG_DIR=/config
    ports:
      - "3000:3000"
    depends_on:
      - qbittorrent
      - jackett

  # ====== 备选：IYUU辅种 ======
  iyuuplus:
    image: iyuuplus/iyuuplus:latest
    container_name: pt_iyuu
    restart: unless-stopped
    volumes:
      - ./config/iyuu:/app/config
      - /data/torrents:/data/torrents
    environment:
      - TZ=Asia/Shanghai
    ports:
      - "8787:8787"

  # ====== 下载器 ======
  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    container_name: pt_qbittorrent
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "6881:6881"
      - "6881:6881/udp"
    volumes:
      - ./config/qbittorrent:/config
      - /data/downloads:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - WEBUI_PORT=8080

  # ====== 搜索引擎代理 (cross-seed需要) ======
  jackett:
    image: linuxserver/jackett:latest
    container_name: pt_jackett
    restart: unless-stopped
    ports:
      - "9117:9117"
    volumes:
      - ./config/jackett:/config/Jackett

networks:
  default:
    name: pt-network
```

#### 生产环境关键配置

**健康检查（必须启用）：**

```dockerfile
# Dockerfile示例
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD curl -f http://localhost:3000/api/health || exit 1
```

**资源限制（防OOM）：**

```yaml
services:
  cross-seed:
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '1.0'
        reservations:
          memory: 256M
          cpus: '0.5'
```

---

### 3.2 监控与可观测性

#### 企业级日志系统（以cross-seed为例）

**日志分级存储：**

```
/var/log/cross-seed/
├── error.log      # 错误日志（保留30天）
├── info.log       # 信息日志（保留14天）
└── verbose.log    # 调试日志（保留7天）
```

**实时日志流API（WebSocket）：**

```javascript
// 前端订阅实时日志
const ws = new WebSocket('ws://localhost:3000/api/logs/stream');

ws.onmessage = (event) => {
    const logEntry = JSON.parse(event.data);
    console.log(`[${logEntry.level}] ${logEntry.message}`);

    // 自动脱敏显示
    console.log(maskSensitiveInfo(logEntry));
};
```

**Prometheus指标采集：**

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'cross-seed'
    static_configs:
      - targets: ['cross-seed:3000']
    metrics_path: '/api/metrics'
    scrape_interval: 15s
```

**Grafana仪表板建议面板：**

| 面板名称 | 指标 | 说明 |
|----------|------|------|
| **辅种成功率** | `cross_seed_success_rate` | 成功/(成功+失败) × 100% |
| **每日辅种数量** | `cross_seed_total{action="save"}` | 按日聚合 |
| **平均响应延迟** | `cross_seed_duration_seconds` | P50/P95/P99 |
| **内存使用率** | `container_memory_usage_bytes` | OOM预警 |
| **CPU负载** | `container_cpu_usage_seconds` | 性能瓶颈检测 |

---

### 3.3 故障案例库

#### 🔴 案例 #1: Docker构建失败（高频故障）

**症状：**
```bash
ERROR: failed to solve: process "/bin/sh -c make build" did not complete successfully
```

**根因分析：**
- Makefile通过`--build-arg`覆盖Dockerfile默认值
- 导致编译时版本号不一致
- 影响范围：pt-tools v0.22.0 → v0.22.3 连续3个小版本修复

**解决方案：**
```dockerfile
# ❌ 错误做法：Makefile动态传参
ARG VERSION=unknown
RUN make build VERSION=${VERSION}

# ✅ 正确做法：单一数据源
ARG VERSION=0.22.3
ENV APP_VERSION=${VERSION}
RUN make build
```

**预防措施：**
- 使用CI/CD固定版本号
- Git Tag与Docker Image Tag一一对应
- 构建前校验版本一致性

---

#### 🔴 案例 #2: 安全漏洞滞后修复

**案例详情：**
- 漏洞ID: GO-2026-4947 (crypto/x509证书验证)
- 发现时间: 2026-03-15
- 修复时间: 2026-04-01 (**17天延迟！**)
- 影响: 中间人攻击风险

**教训：**
```yaml
# .github/workflows/security.yml (推荐配置)
name: Security Scan
on:
  schedule:
    - cron: '0 0 * * *'  # 每日扫描
  push:
    branches: [main]

jobs:
  govulncheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run govulncheck
        uses: golang/govulncheck-action@v1
        with:
          fail-on-vuln: true  # 🔴 设为硬性阻断！
```

---

#### 🔴 案例 #3: 日志泄露敏感信息

**问题现象：**
```log
2026-04-10 12:00:00 INFO  Requesting http://user:pass123@tracker.pt.site.com:2710/announce
```

**风险等级：** 🔴 高危（可能导致账号被封禁）

**解决方案（以cross-seed为例）：**

```typescript
// lib/logger.ts
import winston from 'winston';
import { URL } from 'url';

const sensitivePatterns = [
  /(:\/\/[^:]+:)[^@]+(@)/g,           // URL密码
  /(passkey=)[^&\s]+/gi,                // passkey参数
  /(apikey=|api_key=)[^&\s]+/gi,        // API密钥
  /(cookie:\s*)[^\r\n]+/gi,             // Cookie值
];

function maskSensitiveInfo(message: string): string {
  let masked = message;
  for (const pattern of sensitivePatterns) {
    masked = masked.replace(pattern, '$1***$2');
  }
  return masked;
}

const logger = winston.createLogger({
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.printf(({ timestamp, level, message }) => {
      return `${timestamp} [${level}] ${maskSensitiveInfo(message)}`;
    })
  ),
});
```

**强制措施：**
- 所有项目必须实现结构化日志
- 敏感信息自动脱敏（URL密码、API key、cookie）
- 生产环境禁止输出DEBUG级别日志

---

### 3.4 安全加固最佳实践

#### 多层防御体系

```
┌─────────────────────────────────────────────────────────────┐
│                    辅种系统安全架构                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  L7: 应用审计                                               │
│     ├── 操作日志（谁/何时/做了什么）                        │
│     ├── 异常行为检测（频繁失败/异常流量）                   │
│     └── 定期安全扫描（依赖漏洞/代码审计）                   │
│                                                             │
│  L6: 数据保护                                               │
│     ├── 加密存储（API密钥/密码）                             │
│     ├── 访问控制（RBAC权限模型）                            │
│     └── 数据备份（定期快照/异地容灾）                       │
│                                                             │
│  L5: 通信安全                                               │
│     ├── HTTPS/TLS 1.3 强制加密                              │
│     ├── 证书自动续期（Let's Encrypt / ACME）               │
│     └── CORS白名单严格限制                                  │
│                                                             │
│  L4: 身份认证                                               │
│     ├── PBKDF2/Argon2 密码哈希                              │
│     ├── JWT Token + Refresh Token                          │
│     ├── MFA双因素认证（可选）                               │
│     └── 登录失败次数限制 + IP封禁                           │
│                                                             │
│  L3: 网络隔离                                               │
│     ├── Docker网络隔离（内部网络不暴露）                    │
│     ├── 反向代理（Nginx/Caddy）                             │
│     └── 防火墙规则（仅开放必要端口）                        │
│                                                             │
│  L2: 主机加固                                               │
│     ├── 最小权限原则（非root运行）                          │
│     ├── 只读文件系统（尽可能）                              │
│     ├── Seccomp/AppArmor沙箱                                │
│     └── 定期更新补丁                                        │
│                                                             │
│  L1: 物理安全                                               │
│     ├── 数据中心/机房物理访问控制                           │
│     └── 硬件加密（TPM/SGX - 可选）                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 关键安全配置清单

| 检查项 | 重要性 | 验证方法 |
|--------|--------|----------|
| ✅ 使用HTTPS | 🔴 必须 | `curl -I https://your-domain` |
| ✅ 密码强度≥12位 | 🔴 必须 | 配置文件审查 |
| ✅ 启用CORS限制 | 🟡 推荐 | 浏览器DevTools测试 |
| ✅ 日志脱敏 | 🔴 必须 | grep敏感关键词 |
| ✅ 定期备份数据 | 🔴 必须 | 检查备份脚本cron |
| ✅ RBAC权限控制 | 🟡 建议 | 尝试越权访问 |
| ✅ 依赖漏洞扫描 | 🔴 必须 | `npm audit` / `govulncheck` |
| ✅ 容器只读FS | 🟡 建议 | Docker inspect检查 |

---

## 附录

### A. 选型决策树

```
开始选型
    │
    ▼
您的技术水平？
    ├── 新手/不想折腾
    │   └──→ IYUU (开箱即用)
    │
    ├── 有一定基础
    │   ├── 需要最大覆盖？
    │   │   └── 是 → cross-seed (85+站点)
    │   │
    │   └── 否 → Graft (隐私优先) 或 ptdog (性能优先)
    │
    └── 专家级
        ├── 需要企业级管理？
        │   └── 是 → PTNexus (可视化平台)
        │
        ├── 需要端到端自动化？
        │   └── 是 → hdapt_auto_transfer (转发+辅种一体)
        │
        └── 其他定制需求？
            └── 组合使用多个引擎 (见下方组合方案)
```

**推荐组合方案：**

| 场景 | 主引擎 | 辅助工具 | 说明 |
|------|--------|----------|------|
| **个人多站** | IYUU | auto_feed | 简单高效 |
| **隐私敏感** | cross-seed | Graft | 双重保障 |
| **NexusPHP站群** | reseed-puppy | ptdog | 精准匹配 |
| **企业团队** | PTNexus | Reseed-backend | 可视化管理 |
| **全自动流水线** | hdapt_auto_transfer | ARSS+ADTU | 端到端无人值守 |

---

### B. 快速启动模板

#### IYUU 最简部署（5分钟）

```bash
# 1. 创建目录
mkdir -p ~/pt/iyuu && cd ~/pt/iyuu

# 2. 创建docker-compose.yml
cat > docker-compose.yml << 'EOF'
version: '3.8'
services:
  iyuuplus:
    image: iyuuplus/iyuuplus:latest
    container_name: iyuuplus
    restart: unless-stopped
    ports:
      - "8787:8787"
    volumes:
      - ./config:/app/config
      - /path/to/torrents:/data/torrents
    environment:
      - TZ=Asia/Shanghai
EOF

# 3. 启动服务
docker-compose up -d

# 4. 访问 WebUI
open http://localhost:8787
```

#### Cross-seed 完整部署（含Jackett）

```bash
# 1. 创建目录结构
mkdir -p ~/pt/cross-seed/{config,jackett-config}

# 2. 创建docker-compose.yml
cat > docker-compose.yml << 'EOF'
version: '3.8'

services:
  jackett:
    image: linuxserver/jackett:latest
    container_name: jackett
    restart: unless-stopped
    ports:
      - "9117:9117"
    volumes:
      - ./jackett-config:/config/Jackett
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai

  cross-seed:
    image: cross-seed/cross-seed:latest
    container_name: cross-seed
    restart: unless-stopped
    depends_on:
      - jackett
    ports:
      - "3000:3000"
    volumes:
      - ./config:/config
      - /path/to/qBittorrent/downloads:/downloads
    environment:
      - TZ=Asia/Shanghai
      - CROSS_SEED_CONFIG_DIR=/config
EOF

# 3. 启动
docker-compose up -d

# 4. 配置Jackett indexer
# 访问 http://localhost:9117 添加PT站索引器

# 5. 配置Cross-seed
# 编辑 config/config.js 设置jackett地址
vim config/config.js

# 6. 重启生效
docker-compose restart cross-seed
```

---

## 📊 总结

本文档整合了 **3份独立研究报告** 的精华内容，形成了 **PT 辅种生态系统** 的**完整技术手册**。

### 核心价值

✅ **系统性完整性**
- 从6大核心引擎扩展到10种完整方案
- 覆盖算法原理、架构设计、生产部署全链路
- 提供"从入门到精通"的知识体系

✅ **实战导向性强**
- 真实故障案例库（3个高频问题）
- Docker一键部署模板（可直接复制使用）
- 监控告警配置（Prometheus+Grafana）

✅ **决策支持完善**
- 六维能力矩阵对比（科学选型）
- 选型决策树（快速定位）
- 组合方案推荐（满足复杂需求）

### 适用对象

| 角色 | 主要收益 |
|------|----------|
| **新手用户** | 快速上手，5分钟部署 |
| **PT站长** | 深入理解辅种原理，优化站点功能 |
| **开发者** | 学习优秀架构设计和算法实现 |
| **运维工程师** | 生产环境部署、监控、故障排查指南 |
| **架构师** | 技术选型和系统集成参考 |

---

*文档版本: v4.0 Complete Edition*
*最后更新: 2026-04-12*
*原始来源: 26-reseed-comparison-matrix.md + 27-reseed-ecosystem-vol2-extended.md + 28-reseed-ecosystem-vol3-production.md*
*整合日期: 2026-04-12*
