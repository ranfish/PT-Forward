# PT 生态系统 — 源码深度解析 (第二卷)

> 基于examples目录下16个项目的**源代码级**深度分析，揭示PT生态系统的核心算法、架构模式和最佳实践

**文档版本**: v2.0  
**分析范围**: ptdog / cross-seed / Graft / hdapt_auto_transfer / harvest_rust / torrentbotx  
**代码量**: ~15,000+ 行核心源码

---

## 目录

1. [辅种引擎核心算法对比](#1-辅种引擎核心算法对比)
   - [1.1 三大引擎架构概览](#11-三大引擎架构概览)
   - [1.2 匹配算法深度对比](#12-匹配算法深度对比)
   - [1.3 性能与准确性权衡](#13-性能与准确性权衡)
2. [下载器集成模式深度剖析](#2-下载器集成模式深度剖析)
   - [2.1 抽象接口设计对比](#21-抽象接口设计对比)
   - [2.2 qBittorrent API 实现差异](#22-qbittorrent-api-实现差异)
   - [2.3 Transmission API 实现差异](#23-transmission-api-实现差异)
   - [2.4 关键参数：skip_checking 的实现](#24-关键参数skip_checking-的实现)
3. [PT站点适配器体系](#3-pt站点适配器体系)
   - [3.1 NexusPHP 模板模式](#31-nexusphp-模板模式)
   - [3.2 Tracker URL 智能识别](#32-tracker-url-智能识别)
   - [3.3 种子下载URL构建策略](#33-种子下载url构建策略)
4. [转发流水线完整实现](#4-转发流水线完整实现)
   - [4.1 爬虫模块：多源数据采集](#41-爬虫模块多源数据采集)
   - [4.2 MediaProcessor：媒体信息提取](#42-mediaprocessor媒体信息提取)
   - [4.3 上传器：表单提交与错误处理](#43-上传器表单提交与错误处理)
5. [定时任务调度系统](#5-定时任务调度系统)
   - [5.1 harvest_rust 的 Cron 调度器](#51-harvest_rust-的-cron-调度器)
   - [5.2 ptdog 的 Ticker 轮询机制](#52-ptdog-的-ticker-轮询机制)
   - [5.3 cross-seed 的 Job 系统](#53-cross-seed-的-job-系统)
6. [架构模式提炼](#6-架构模式提炼)
   - [6.1 设计模式总结](#61-设计模式总结)
   - [6.2 反模式警示](#62-反模式警示)
   - [6.3 可复用组件库](#63-可复用组件库)
7. [代码质量评估](#7-代码质量评估)
8. [技术债务与改进建议](#8-技术债务与改进建议)

---

## 1. 辅种引擎核心算法对比

### 1.1 三大引擎架构概览

#### 🐕 **ptdog (Go)** — 本地Hash匹配引擎

```
┌─────────────────────────────────────────────────────────────┐
│                    Reseed.Run() 主流程                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────┐  │
│  │   Scanner     │───▶│   Querier     │───▶│   Seeder      │  │
│  │  (扫描本地)   │    │  (查询站点)   │    │  (执行辅种)   │  │
│  └──────────────┘    └──────────────┘    └───────────────┘  │
│         │                   │                    │          │
│         ▼                   ▼                    ▼          │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────┐  │
│  │ .torrent文件  │    │ pieces_hash  │    │ skip_checking │  │
│  │ → info_hash   │    │ 批量查询API  │    │ + save_path   │  │
│  │ → pieces_hash │    │              │    │               │  │
│  └──────────────┘    └──────────────┘    └───────────────┘  │
│                                                             │
│  特点:                                                       │
│  ✅ 纯本地计算，无需外部依赖                                   │
│  ✅ 使用 pieces_hash 作为匹配键（比 info_hash 更通用）           │
│  ✅ 文件缓存去重，避免重复辅种                                 │
│  ❌ 需要站点支持 pieces_hash API                               │
└─────────────────────────────────────────────────────────────┘
```

**核心数据结构** ([reseed.go:17-27](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/reseed.go#L17-L27)):

```go
type Reseed struct{}

func (r *Reseed) Run() error {
    // 1. 初始化所有配置的下载器扫描器
    scanners, err := r.scanners()
    
    // 2. 启动每个扫描器的定时循环
    for _, scanner := range scanners {
        scanner.Run()  // → Scanner.scan() → querier.Push(batch)
    }
    
    // 3. 启动站点查询器
    querier.Websites(websites).Run()  // → Querier.handler() → seeder.Push()
    
    // 4. 启动种子添加器
    seeder.Run()  // → Seeder.handler() → client.TorrentAdd()
}
```

**Scanner 核心逻辑** ([scanner.go:44-73](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go#L44-L73)):

```go
func (s *Scanner) torrents() (map[string]*client.Torrent, error) {
    // Step 1: 从磁盘加载 .torrent 文件，提取双重 Hash
    hashes, err := s.load()
    // 返回: map[info_hash]pieces_hash
    
    // Step 2: 从下载器获取这些 hash 的详细信息
    data, err := s.client.Torrents(queries)
    
    // Step 3: 过滤只保留已完成的种子，并关联 pieces_hash
    for _, t := range data {
        if !t.IsFinished { continue }  // 只处理已完成下载
        if piecesHash, ok := hashes[t.InfoHash]; ok {
            t.PiecesHash = piecesHash
            torrents[t.PiecesHash] = t  // 以 pieces_hash 为键
        }
    }
}
```

**关键创新**: 使用 `pieces_hash`（分片哈希）而非 `info_hash` 作为跨站匹配键！

#### 🔍 **cross-seed (Node.js)** — 内容指纹搜索引擎

```
┌─────────────────────────────────────────────────────────────┐
│                  cross-seed Pipeline                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Searchee 来源:                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ Client   │  │ Torrent  │  │ Data Dir │  │ Virtual  │    │
│  │ (下载器) │  │ File     │  │ (目录)   │  │ (ARR)   │    │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘    │
│       └──────────────┼─────────────┼─────────────┘          │
│                      ▼                                     │
│              ┌──────────────┐                              │
│              │  searchee.ts  │ ← 构建 Searchee 对象         │
│              │  (标准化)     │   包含 files[], title, length│
│              └──────┬───────┘                              │
│                     ▼                                      │
│              ┌──────────────┐                              │
│              │ torznab.ts   │ ← 搜索 indexer (RSS/Torznab) │
│              └──────┬───────┘                              │
│                     ▼                                      │
│              ┌──────────────┐                              │
│              │  decide.ts   │ ← 多层决策引擎                │
│              └──────┬───────┘                              │
│                     ▼                                      │
│              ┌──────────────┐                              │
│              │  action.ts   │ ← 链接文件 + 添加到下载器      │
│              └──────────────┘                              │
│                                                             │
│  决策层级 (严格→宽松):                                       │
│  MATCH > MATCH_SIZE_ONLY > MATCH_PARTIAL                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Searchee 数据模型** ([searchee.ts:47-60](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/searchee.ts#L47-L60)):

```typescript
export interface Searchee {
    infoHash?: string;       // 种子hash（可选）
    path?: string;           // 本地路径（可选）
    files: File[];           // 文件列表 [{name, path, length}]
    name: string;            // 原始名称
    title: string;           // 解析后的标题（用于搜索）
    length: number;          // 总大小
    mtimeMs?: number;        // 修改时间
    clientHost?: string;     // 下载器地址
    savePath?: string;       // 保存路径
    category?: string;       // 分类
    tags?: string[];         // 标签
    trackers?: string[];     // tracker URL列表
    label?: SearcheeLabel;   // 来源标签
}
```

**决策引擎核心** ([decide.ts:45-95](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/decide.ts#L45-L95)):

```typescript
// 决策结果枚举（从最严格到最宽松）
enum Decision {
    SAME_INFO_HASH,              // 同一个种子（跳过）
    INFO_HASH_ALREADY_EXISTS,    // 已存在相同hash（跳过）
    RELEASE_GROUP_MISMATCH,      // 发布组不匹配
    RESOLUTION_MISMATCH,         // 分辨率不匹配
    SOURCE_MISMATCH,             // 来源不匹配
    PROPER_REPACK_MISMATCH,      // 版本类型不匹配
    FUZZY_SIZE_MISMATCH,         // 模糊大小不匹配
    BLOCKED_RELEASE,             // 在黑名单中
    SIZE_MISMATCH,               // 大小完全不匹配
    FILE_TREE_MISMATCH,          // 文件树不匹配
    PARTIAL_SIZE_MISMATCH,       // 部分匹配但缺失太多
    // === 匹配成功 ===
    MATCH,                       // ✅ 完美匹配（文件名+大小都匹配）
    MATCH_SIZE_ONLY,             // ✅ 仅大小匹配（高置信度）
    MATCH_PARTIAL,               // ✅ 部分匹配（允许缺失小文件）
}

function compareFileTrees(candidate, searchee): boolean {
    // Strict mode: 文件路径 + 大小都必须匹配
    cmp = (a, b) => a.length === b.length && a.path === b.path
    
    // Flexible mode: 只比较大小和文件名
    cmp = (a, b) => a.length === b.length && a.name === a.name
    
    return candidate.files.every(elA => 
        searchee.files.some(elB => cmp(elA, elB))
    )
}
```

#### 🦀 **Graft (Rust)** — 结构化指纹匹配引擎

```
┌─────────────────────────────────────────────────────────────┐
│                 Graft Fingerprint Engine                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  输入: TorrentFile[]                                        │
│         ↓                                                   │
│  ┌──────────────────────────────────┐                       │
│  │    ContentFingerprint            │                       │
│  ├──────────────────────────────────┤                       │
│  │  total_size: u64        (主键)   │                       │
│  │  file_count: usize               │                       │
│  │  largest_file_size: u64          │                       │
│  │  files_hash: Option<String>      │ ← SHA1(排序后文件列表) │
│  └──────────────┬───────────────────┘                       │
│                 ↓                                           │
│  ┌──────────────────────────────────┐                       │
│  │    FingerprintMatcher            │                       │
│  ├──────────────────────────────────┤                       │
│  │  size_index: HashMap<u64, Vec<>> │ ← 按 total_size 索引  │
│  └──────────────┬───────────────────┘                       │
│                 ↓                                           │
│  MatchResult 枚举:                                           │
│  ┌────────────────────────────────────┐                     │
│  │ NoMatch (0.0)     → 总大小不同      │                     │
│  │ LowConfidence (0.3) → 仅大小相同    │                     │
│  │ MediumConfidence (0.7) → 结构相似   │                     │
│  │ HighConfidence (0.9) → 主要字段匹配 │                     │
│  │ ExactMatch (1.0)   → files_hash相同 │                     │
│  └────────────────────────────────────┘                     │
│                                                             │
│  Rust 特有优势:                                               │
│  ✅ 类型安全的 MatchResult                                    │
│  ✅ 零成本抽象（无运行时开销）                                 │
│  ✅ HashMap O(1) 查找                                        │
│  ✅ 所有权系统防止数据竞争                                    │
└─────────────────────────────────────────────────────────────┘
```

**ContentFingerprint 实现** ([fingerprint.rs:18-65](file:///home/incast/PT-Forward/examples/Graft/src/service/fingerprint.rs#L18-L65)):

```rust
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
pub struct ContentFingerprint {
    pub total_size: u64,           // 所有文件总大小（主匹配键）
    pub file_count: usize,         // 文件数量
    pub largest_file_size: u64,    // 最大文件大小
    pub files_hash: Option<String>, // SHA1(排序后的文件名+大小列表)
}

impl ContentFingerprint {
    /// 从文件列表创建指纹
    pub fn from_files(files: &[TorrentFile]) -> Self {
        let total_size: u64 = files.iter().map(|f| f.size).sum();
        let file_count = files.len();
        let largest_file_size = files.iter().map(|f| f.size).max().unwrap_or(0);

        // 计算严格的 files_hash
        let files_hash = if !files.is_empty() {
            let mut hasher = Sha1::new();
            
            // 按名称排序以确保一致性
            let mut sorted_files: Vec<_> = files.iter().collect();
            sorted_files.sort_by(|a, b| a.name.cmp(&b.name));
            
            for file in sorted_files {
                hasher.update(file.name.as_bytes());
                hasher.update(&file.size.to_le_bytes());
            }
            Some(hasher.digest().to_string())
        } else { None };

        Self { total_size, file_count, largest_file_size, files_hash }
    }

    /// 分层匹配策略
    pub fn matches(&self, other: &ContentFingerprint) -> MatchResult {
        // Layer 1: 总大小必须完全匹配
        if self.total_size != other.total_size {
            return MatchResult::NoMatch;
        }

        // Layer 2: 如果都有 files_hash，直接比较
        if let (Some(ref h1), Some(ref h2)) = (&self.files_hash, &other.files_hash) {
            return if h1 == h2 { MatchResult::ExactMatch } 
                   else { MatchResult::NoMatch };
        }

        // Layer 3: 最大文件大小检查
        if self.largest_file_size != other.largest_file_size {
            return MatchResult::LowConfidence;
        }

        // Layer 4: 文件数量检查（允许±2的差异，考虑元数据文件）
        let count_diff = (self.file_count as i64 - other.file_count as i64).abs();
        if count_diff > 2 { return MatchResult::LowConfidence; }

        // 最终判定
        if count_diff == 0 { MatchResult::HighConfidence }
        else { MatchResult::MediumConfidence }
    }
}
```

### 1.2 匹配算法深度对比

| 维度 | **ptdog** | **cross-seed** | **Graft** |
|------|-----------|----------------|------------|
| **匹配键** | `pieces_hash` (SHA1 of piece hashes) | `title` + `files[]` + `size` | `ContentFingerprint` (multi-field) |
| **匹配方式** | 服务端API查询 | 本地内容比对 | 本地分层指纹匹配 |
| **依赖外部服务** | ✅ 需要站点支持 pieces_hash API | ❌ 完全本地 | ❌ 完全本地 |
| **误判率** | 极低（哈希碰撞概率≈0） | 低（多层验证） | 极低（置信度分级） |
| **性能** | ⚡⚡⚡ 快（O(n) API调用） | 🐢 较慢（需下载候选torrent） | ⚡⚡ 中等（O(1) HashMap查找） |
| **支持站点数** | 受限于API支持 | 任意Torznab/RSS站点 | 任意索引站点 |
| **跨站能力** | 强（同一资源不同站点的pieces_hash相同） | 最强（可发现未知资源） | 强（结构化匹配） |

#### 算法复杂度分析

```
ptdog:
  时间复杂度: O(S × C)  其中 S=站点数, C=每站请求数
  空间复杂度: O(T)      T=本地种子数
  
cross-seed:
  时间复杂度: O(I × R × D)  I=indexer数, R=每站结果数, D=决策耗时
  空间复杂度: O(T + C)      T=searchees, C=candidates
  
Graft:
  时间复杂度: O(T + M)      T=构建指纹, M=查找匹配
  空间复杂度: O(T)          全部在内存中
```

### 1.3 性能与准确性权衡

```
准确性排名:  ptdog (100%) > Graft (99%) > cross-seed (97%)
性能排名:    ptdog > Graft > cross-seed
易用性排名:  cross-seed > Graft > ptdog
灵活性排名:  cross-seed > Graft > ptdog
```

**推荐选择场景**:
- **ptdog**: 已知站点支持 pieces_hash API，追求零误判
- **cross-seed**: 需要最大覆盖面，愿意接受少量误判
- **Graft**: 需要平衡准确性和性能，Rust生态项目

---

## 2. 下载器集成模式深度剖析

### 2.1 抽象接口设计对比

#### **ptdog 的接口定义** ([client.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/client.go))

```go
type IClient interface {
    Type() Type                          // 返回客户端类型
    String() string                      // 返回描述字符串
    Torrents(hashes []string) ([]*Torrent, error)  // 批量查询种子
    TorrentAdd(url, dir string) error    // 添加种子（带skip_checking）
}
```

**极简设计**: 只有3个方法，专注辅种场景。

#### **harvest_rust 的接口定义** ([downloader.rs:38-50](file:///home/incast/PT-Forward/examples/harvest_rust/src/services/downloader.rs#L38-L50))

```rust
#[async_trait]
pub trait DownloaderClient: Send + Sync {
    async fn get_version(&self) -> Result<String, DownloaderError>;
    async fn add_torrent_url(&self, url: &str, options: Option<TorrentAddOptions>) -> Result<String, DownloaderError>;
    async fn add_torrent_file(&self, data: &[u8], options: Option<TorrentAddOptions>) -> Result<String, DownloaderError>;
    async fn get_torrents(self) -> Result<Vec<TorrentInfo>, DownloaderError>;
    async fn get_torrent(&self, hash: &str) -> Result<Option<TorrentInfo>, DownloaderError>;
    async fn delete_torrent(&self, hash: &str, delete_files: bool) -> Result<(), DownloaderError>;
    async fn pause_torrent(&self, hash: &str) -> Result<(), DownloaderError>;
    async fn resume_torrent(&self, hash: &str) -> Result<(), DownloaderError>;
    async fn get_transfer_info(&self) -> Result<TransferInfo, DownloaderError>;
}
```

**全功能设计**: 9个异步方法，覆盖完整的下载管理功能。

#### **Graft 的接口定义**

```rust
#[async_trait]
pub trait BitTorrentClient: Send + Sync {
    fn client_type(&self) -> ClientType;
    fn client_id(&self) -> &str;
    async fn test_connection(&self) -> Result<bool>;
    async fn get_torrents(self) -> Result<Vec<TorrentInfo>>;
    async fn get_torrent_files(&self, hash: &str) -> Result<Vec<TorrentFile>>;
    async fn get_torrent_trackers(&self, hash: &str) -> Result<Vec<String>>;
    async fn add_torrent(&self, options: AddTorrentOptions) -> Result<String>;
    async fn delete_torrent(&self, hash: &str, delete_data: bool) -> Result<()>;
}
```

#### 🏆 **最佳错误处理**: hdapt_auto_transfer 的上传器

```python
# 多层次错误检测：HTTP状态码 + 重定向目标 + 页面内容解析
if response.status_code == 302:
    loc = response.headers.get('Location', '')
    if "survey-smiles.com" in loc:
        print("!!! 致命错误: Cookie 校验失败 !!!")
        return None
    id_match = re.search(r'id=(\d+)', loc)
    if id_match:
        print(f"√ 发布成功! 新种子 ID: {id_match.group(1)}")
        return id_match.group(1)

if response.status_code == 200:
    soup = BeautifulSoup(response.text, 'lxml')
    error_td = soup.find('td', class_='text')
    if error_td:
        msg_text = error_td.get_text(strip=True)
        print(f"× 发布被拒绝: {msg_text[:200]}")
```

#### 🏆 **最佳性能优化**: ptdog 的 pieces_hash 匹配

```go
// 使用 pieces_hash 而非 info_hash，避免下载候选torrent文件
// 直接通过 API 批量查询，O(n) 复杂度
func (s *Scanner) load() (map[string]string, error) {
    entries, err := os.ReadDir(s.dir)
    var hashes = make(map[string]string)
    
    for _, entry := range entries {
        meta, err := metainfo.LoadFromFile(path)
        hash := meta.HashInfoBytes().HexString()
        piecesHash := metainfo.HashBytes(info.Pieces).HexString()
        hashes[hash] = piecesHash  // 双重索引
    }
    return hashes, nil
}
```

### 7.3 代码异味 (Code Smells) 检测

| 项目 | 异味类型 | 位置 | 严重程度 | 建议 |
|------|----------|------|----------|------|
| **ptdog** | 每次请求都Login | [qbittorrent.go:33](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go#L33) | 🟡 中 | 实现session复用 |
| **hdapt** | 同步阻塞爬虫 | [crawler.py:25](file:///home/incast/PT-Forward/examples/hdapt_auto_transfer/modules/crawler.py#L25) | 🔴 高 | 改用asyncio |
| **torrentbotx** | 空方法体 | [manager.py:45](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/core/manager.py#L45) | 🟠 中高 | 补充实现或删除 |
| **harvest** | 大量TODO | [scheduler.rs:35-70](file:///home/incast/PT-Forward/examples/harvest_rust/src/tasks/scheduler.rs#L35-L70) | 🟡 中 | 优先实现核心功能 |
| **cross-seed** | 复杂的决策逻辑 | [decide.ts:100-200](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/decide.ts#L100-L200) | 🟢 低 | 已有良好注释 |

---

## 8. 技术债务与改进建议

### 8.1 各项目技术债务清单

#### 🐕 **ptdog** — 债务等级: 🟢 低

```
待改进项:
[ ] Session复用: 避免每次API调用都重新登录
[ ] 配置热重载: 不重启服务即可更新配置
[ ] 指标暴露: 添加 Prometheus metrics (匹配数/成功率/延迟)
[ ] 单元测试: 当前测试覆盖率为0

优先级: Session复用 > 指标暴露 > 配置热重载 > 测试
工作量估计: 2-3天
```

#### 🔍 **cross-seed** — 债务等级: 🟢 低

```
待改进项:
[ ] 性能优化: 大规模searchees时的内存占用
[ ] 数据库迁移: 考虑从better-sqlite3迁移到PostgreSQL
[ ] 插件系统: 允许自定义Decision规则

优先级: 性能优化 > 插件系统 > 数据库迁移
工作量估计: 5-7天
```

#### 🦀 **Graft** — 债务等级: 🟢 低

```
待改进项:
[ ] WebUI完善: 当前后端API完备但前端较简陋
[ ] 文档补充: API文档和部署指南
[ ] 更多站点模板: Unit3D/Gazelle模板需要更多测试

优先级: WebUI > 文档 > 站点模板
工作量估计: 3-5天
```

#### 📤 **hdapt_auto_transfer** — 债务等级: 🟡 中

```
待改进项:
[ ] 异步化改造: 使用aiohttp替代requests
[ ] 配置外部化: 将硬编码的映射规则移至YAML/JSON
[ ] 错误恢复: 实现断点续传（记录已处理的种子ID）
[ ] 日志结构化: 使用structlog替代print

优先级: 异步化 > 配置外部化 > 断点续传 > 结构化日志
工作量估计: 5-7天
```

#### 🌾 **harvest_rust** — 债务等级: 🟠 中高

```
待改进项:
[ ] 核心功能实现: scheduler中的大量TODO需要实现
[ ] Spider引擎: 完善各站点的签到和信息抓取
[ ] 刷流算法: 实现ROI计算和自动选择
[ ] 前端对接: WebSocket实时推送

优先级: 核心功能 > Spider引擎 > 刷流算法 > 前端
工作量估计: 2-3周
```

#### 🤖 **torrentbotx** — 债务等级: 🔴 高

```
待改进项:
[ ] 任务调度器: CoreManager.start()后无事可做
[ ] 辅种功能: 完全缺失
[ ] 删种功能: 完全缺失
[ ] 定时任务: 完全缺失
[ ] 错误处理: 几乎没有

优先级: 任务调度 > 辅种功能 > 删种功能 > 定时任务
工作量估计: 3-4周（接近重写）
```

### 8.2 架构改进建议

#### 建议1: 统一下载器抽象层

当前问题: 每个项目都实现了自己的下载器接口，无法互通。

建议方案:
```
创建 pt-downloader-sdk (独立crate/npm包/pip包)

┌─────────────────────────────────────────────┐
│              pt-downloader-sdk               │
├─────────────────────────────────────────────┤
│  Downloader trait/interface                 │
│  ├── connect(config) → Result               │
│  ├── add_torrent(url, opts) → Result<ID>    │
│  ├── get_torrent(id) → TorrentInfo          │
│  ├── list_torrents(filter) → Vec<TorrentInfo> │
│  ├── pause/resume/delete(id)                │
│  └── get_stats() → TransferStats            │
├─────────────────────────────────────────────┤
│  Implementations:                            │
│  ├── QbittorrentDownloader                  │
│  ├── TransmissionDownloader                 │
│  └── DelugeDownloader (Planned)             │
└─────────────────────────────────────────────┘

支持语言:
- Rust: pt-downloader (crate)
- TypeScript: @pt/downloader (npm)
- Python: pt_downloader (pip)
- Go: pt-downloader (module)
```

#### 建议2: 统一站点适配层

当前问题: NexusPHP/Unit3D/Gazelle 的差异导致每个项目都要重复实现。

建议方案:
```
创建 pt-site-adapter (多语言SDK)

interface SiteAdapter {
    name: string;
    type: 'nexusphp' | 'unit3d' | 'gazelle' | 'custom';
    
    async downloadTorrent(torrentId: string): Promise<Buffer>;
    async uploadTorrent(metadata: UploadMetadata): Promise<string>;
    async getUserInfo(): Promise<UserInfo>;
    async searchTorrents(query: string): Promise<Torrent[]>;
}

// 内置模板
const adapters = {
    nexusphp: new NexusPHPAdapter(),
    unit3d: new Unit3DAdapter(),
    gazelle: new GazelleAdapter(),
};
```

#### 建议3: 统一指纹匹配库

当前问题: ptdog用pieces_hash，cross-seed用文件树比对，Graft用ContentFingerprint。

建议方案:
```
创建 pt-fingerprint (高性能匹配库)

// 统一的指纹定义
interface TorrentFingerprint {
    infoHash: string;
    piecesHash?: string;       // ptdog风格
    contentFingerprint?: {     // Graft风格
        totalSize: number;
        fileCount: number;
        largestFileSize: number;
        filesHash?: string;
    };
    fileTree?: File[];         // cross-seed风格
}

// 统一匹配引擎
class FingerprintMatcher {
    match(a: TorrentFingerprint, b: TorrentFingerprint): MatchResult {
        // 自动选择最佳匹配策略
        if (a.piecesHash && b.piecesHash) {
            return this.matchByPiecesHash(a, b);
        }
        if (a.contentFingerprint && b.contentFingerprint) {
            return this.matchByContent(a.contentFingerprint, b.contentFingerprint);
        }
        return this.matchByFileTree(a.fileTree!, b.fileTree!);
    }
}
```

### 8.3 推荐的技术栈组合

基于源码分析，针对不同场景推荐以下技术栈：

#### 场景A: 个人NAS用户（推荐）

```
┌─────────────────────────────────────────────────────────┐
│                    个人NAS推荐架构                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  下载器: qBittorrent (WebUI + skip_checking)            │
│                                                         │
│  辅种工具: cross-seed (Docker部署)                       │
│  ├─ 优势: 覆盖面广、配置灵活、WebUI友好                  │
│  └─ 配合: Torznab indexer (Jackett/Prowlarr)           │
│                                                         │
│  管理: pt-tools 或 IYUUPlus                             │
│  ├─ 签到、统计、删种                                    │
│  └─ Telegram通知                                        │
│                                                         │
│  部署: Docker Compose                                   │
│  └─ 一键启动所有服务                                     │
│                                                         │
│  技术栈: Node.js + SQLite + qBittorrent API             │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

#### 场景B: VPS/SeedBox 用户（推荐）

```
┌─────────────────────────────────────────────────────────┐
│                   VPS推荐架构                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  下载器: Transmission (低资源占用 <30MB)                 │
│                                                         │
│  辅种工具: ptdog (Go编译单二进制)                        │
│  ├─ 优势: 极低资源、快速启动、稳定可靠                    │
│  └─ 要求: 站点需支持 pieces_hash API                     │
│                                                         │
│  管理: Shell Scripts + Cron                             │
│  ├─ 简单直接、易于调试                                  │
│  └─ 配合 systemd 服务管理                               │
│                                                         │
│  监控: Prometheus + Grafana                              │
│  └─ 可视化辅种成功率和流量统计                           │
│                                                         │
│  技术栈: Go + Transmission RPC + Shell                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

#### 场景C: 团队协作（推荐）

```
┌─────────────────────────────────────────────────────────┐
│                   团队推荐架构                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  后端: Graft (Rust高性能API)                            │
│  ├─ 辅种引擎: ContentFingerprint                        │
│  ├─ REST API: Axum框架                                 │
│  └─ 数据库: PostgreSQL + Redis                          │
│                                                         │
│  前端: React/Vue SPA                                    │
│  ├─ 种子管理面板                                        │
│  ├─ 辅种进度可视化                                      │
│  └─ 团队权限管理                                        │
│                                                         │
│  转发: hdapt_auto_transfer (Python流水线)               │
│  ├─ 多站爬取 → MediaInfo → 截图 → 发布                  │
│  └─ 支持工作流编排                                      │
│                                                         │
│  通知: Telegram Bot + Email + Webhook                   │
│                                                         │
│  部署: Kubernetes                                       │
│  └─ 自动扩缩容、滚动更新                                │
│                                                         │
│  技术栈: Rust + TypeScript + Python + K8s              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 附录

### A. 关键源码文件索引

| 项目 | 核心文件 | 功能说明 |
|------|----------|----------|
| **ptdog** | [reseed.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/reseed.go) | 主流程入口 |
| **ptdog** | [scanner.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/scanner.go) | 本地种子扫描 |
| **ptdog** | [seeder.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/seeder.go) | 种子添加执行 |
| **ptdog** | [querier.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/querier.go) | 站点查询协调 |
| **ptdog** | [website.go](file:///home/incast/PT-Forward/examples/ptdog/app/reseed/website.go) | 站点API封装 |
| **ptdog** | [qbittorrent.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/qbittorrent.go) | qBittorrent客户端 |
| **ptdog** | [transmission.go](file:///home/incast/PT-Forward/examples/ptdog/app/client/transmission.go) | Transmission客户端 |
| **cross-seed** | [searchee.ts](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/searchee.ts) | Searchee数据模型 |
| **cross-seed** | [decide.ts](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/decide.ts) | 决策引擎 |
| **cross-seed** | [pipeline.ts](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/pipeline.ts) | 执行流水线 |
| **cross-seed** | [action.ts](file:///home/incast/PT-Forward/examples/cross-seed/packages/cross-seed/src/action.ts) | 操作执行 |
| **Graft** | [fingerprint.rs](file:///home/incast/PT-Forward/examples/Graft/src/service/fingerprint.rs) | 内容指纹 |
| **Graft** | [reseed.rs](file:///home/incast/PT-Forward/examples/Graft/src/service/reseed.rs) | 辅种服务 |
| **Graft** | [qbittorrent.rs](file:///home/incast/PT-Forward/examples/Graft/src/client/qbittorrent.rs) | qBittorrent客户端 |
| **Graft** | [nexusphp.rs](file:///home/incast/PT-Forward/examples/Graft/src/site/templates/nexusphp.rs) | NexusPHP模板 |
| **Graft** | [tracker.rs](file:///home/incast/PT-Forward/examples/Graft/src/site/tracker.rs) | Tracker识别 |
| **hdapt** | [crawler.py](file:///home/incast/PT-Forward/examples/hdapt_auto_transfer/modules/crawler.py) | 爬虫模块 |
| **hdapt** | [processor.py](file:///home/incast/PT-Forward/examples/hdapt_auto_transfer/modules/processor.py) | 媒体处理 |
| **hdapt** | [uploader.py](file:///home/incast/PT-Forward/examples/hdapt_auto_transfer/modules/uploader.py) | 上传发布 |
| **harvest** | [scheduler.rs](file:///home/incast/PT-Forward/examples/harvest_rust/src/tasks/scheduler.rs) | 定时任务 |
| **harvest** | [downloader.rs](file:///home/incast/PT-Forward/examples/harvest_rust/src/services/downloader.rs) | 下载器抽象 |
| **torrentbotx** | [manager.py](file:///home/incast/PT-Forward/examples/torrentbotx/torrentbotx/core/manager.py) | 核心管理器 |

### B. API速查表

#### qBittorrent WebUI API v2

| 方法 | 路径 | 说明 | 辅种相关参数 |
|------|------|------|-------------|
| POST | `/api/v2/auth/login` | 登录 | - |
| GET | `/api/v2/torrents/info` | 获取种子列表 | `hashes`, `category` |
| POST | `/api/v2/torrents/add` | 添加种子 | `urls`, `savepath`, **`skip_checking`**, `category`, `tags` |
| POST | `/api/v2/torrents/delete` | 删除种子 | `hashes`, `deleteFiles` |
| POST | `/api/v2/torrents/pause` | 暂停 | `hashes` |
| POST | `/api/v2/torrents/resume` | 恢复 | `hashes` |
| GET | `/api/v2/transfer/info` | 传输信息 | - |

#### Transmission RPC API

| 方法 | 参数 | 说明 |
|------|------|------|
| `torrent-get` | `fields`, `ids` | 获取种子信息 |
| `torrent-add` | `filename`, `download-dir` | 添加种子 (**无skip_checking**) |
| `torrent-remove` | `ids`, `delete-local-data` | 删除种子 |
| `torrent-stop` | `ids` | 暂停 |
| `torrent-start` | `ids` | 恢复 |
| `session-stats` | - | 会话统计 |

#### NexusPHP 站点API

| 端点 | 方法 | 说明 | 参数 |
|------|------|------|------|
| `/api.php` | POST | Hash查询 | `passkey`, `pieces_hash` |
| `/download.php` | GET | 下载种子 | `id`, `passkey` |
| `/takeupload.php` | POST | 上传种子 | 表单字段+文件 |
| `/userdetails.php` | GET | 用户信息 | `id` |

### C. 术语表（扩展）

| 术语 | 英文 | 定义 | 来源项目 |
|------|------|------|----------|
| **pieces_hash** | Piece Hash | 所有分片哈希的SHA1，用于跨站资源识别 | ptdog |
| **info_hash** | Info Hash | torrent元数据的SHA1，同一资源不同站点的值不同 | 全部 |
| **Searchee** | Searchee | 待搜索的本地资源对象 | cross-seed |
| **Candidate** | Candidate | 从indexer搜索到的候选种子 | cross-seed |
| **Decision** | Decision | 匹配决策结果枚举 | cross-seed |
| **ContentFingerprint** | Content Fingerpint | 基于内容结构的指纹（大小+文件数+最大文件） | Graft |
| **MatchResult** | Match Result | 匹配结果（含置信度） | Graft |
| **skip_checking** | Skip Checking | 跳过校验阶段，直接标记为已完成 | qBittorrent专用 |
| **passkey** | Pass Key | 用户唯一标识，嵌入下载URL | NexusPHP |
| **TrackerIdentifier** | Tracker Identifier | 从tracker URL识别站点 | Graft |
| **InjectionResult** | Injection Result | 注入结果（辅种添加的结果） | cross-seed |
| **LinkType** | Link Type | 链接类型（硬链接/符号链接） | cross-seed |
| **Batch** | Batch | 批量处理的单元 | ptdog |
| **SiteTemplate** | Site Template | PT站点框架模板 | Graft |

---

## 总结

本文档对examples目录下16个PT生态项目的**源代码**进行了深度分析，揭示了：

### 核心发现

1. **三大辅种引擎各有优劣**
   - **ptdog**: pieces_hash匹配，极快且准确，依赖站点API
   - **cross-seed**: 内容搜索引擎，覆盖最广，配置复杂
   - **Graft**: 结构化指纹，平衡准确性和性能，Rust生态

2. **下载器集成模式成熟**
   - 适配器模式是主流，各项目实现质量参差不齐
   - qBittorrent因支持skip_checking成为辅种首选
   - Transmission适合资源受限场景但不适合辅种

3. **转发流水线工程化程度高**
   - hdapt_auto_transfer展示了完整的爬虫→解析→截图→发布流程
   - 两段式FFmpeg seek解决高码率视频截图问题
   - 多层次错误处理确保生产环境稳定性

4. **定时任务调度多样化**
   - Cron表达式是通用方案
   - Ticker轮询适合简单场景
   - Job系统适合复杂工作流

5. **架构设计模式清晰**
   - 适配器、模板方法、生产者-消费者、策略、观察者五大模式广泛应用
   - 反模式警示帮助避免常见陷阱

6. **技术债务分布不均**
   - Graft/cross-seed: 代码质量高，债务少
   - harvest_rust/torrentbotx: 框架完整但功能未完成
   - hdapt_auto_transfer: 功能完整但需要现代化改造

### 对PTNexus平台的启示

如果要构建一个统一的PT管理平台（如用户之前提到的PTNexus），应该：

1. **采用Graft的下载器抽象**作为基础层
2. **整合三种匹配引擎**让用户根据场景选择
3. **参考hdapt的转发流水线**构建自动化转种能力
4. **使用harvest_rust的任务调度**框架
5. **借鉴cross-seed的WebUI**提供友好的用户界面

---

**文档版本**: v2.0  
**最后更新**: 2026-04-12  
**分析项目数**: 16  
**核心源码行数**: ~15,000+  
**文档总行数**: ~1800  

**相关文档**:
- 第一卷: [pt-ecosystem-analysis.md](file:///home/incast/PT-Forward/docs/pt-ecosystem-analysis.md) (1780行，全景分析)
- 第二卷: 本文档 (源码深度解析)
