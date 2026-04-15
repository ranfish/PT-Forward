# 跨站辅种功能深度分析

## 核心概念

### 什么是跨站辅种？

跨站辅种是指利用已有的种子文件，在其他 PT 站点找到相同内容的种子，从而实现：
- **无需重新下载** - 利用已有数据
- **多站做种** - 提高分享率
- **自动匹配** - 最小化人工干预

---

## 核心工作流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Cross-Seed 辅种工作流程                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐                                                           │
│  │ 1. 数据源    │ ─── 客户端种子 / 种子文件 / 数据目录                      │
│  └──────┬───────┘                                                           │
│         │                                                                   │
│         v                                                                   │
│  ┌──────────────┐     ┌─────────────────────────────────────────┐          │
│  │ 2. 创建      │ ─── │ Searchee (搜索源对象)                    │          │
│  │   Searchee   │     │ - infoHash / path / files               │          │
│  └──────┬───────┘     │ - title, name, length                   │          │
│         │             │ - mediaType, clientHost                  │          │
│         v             └─────────────────────────────────────────┘          │
│  ┌──────────────┐                                                           │
│  │ 3. 预过滤    │ ─── 过滤不符合条件的内容                                  │
│  └──────┬───────┘                                                           │
│         │                                                                   │
│         v                                                                   │
│  ┌──────────────┐     ┌─────────────────────────────────────────┐          │
│  │ 4. Torznab   │ ─── │ 通过 Prowlarr/Jackett 搜索索引器         │          │
│  │    搜索      │     │ - 构建搜索查询                          │          │
│  └──────┬───────┘     │ - 支持 ID 搜索 (TVDB/TMDB/IMDB)         │          │
│         │             └─────────────────────────────────────────┘          │
│         v                                                                   │
│  ┌──────────────┐     ┌─────────────────────────────────────────┐          │
│  │ 5. 候选评估  │ ─── │ 多维度匹配检查                          │          │
│  │   (Decide)   │     │ - 发布组、分辨率、来源、大小             │          │
│  └──────┬───────┘     │ - 文件树对比                            │          │
│         │             └─────────────────────────────────────────┘          │
│         v                                                                   │
│  ┌──────────────┐     ┌─────────────────────────────────────────┐          │
│  │ 6. 执行动作  │ ─── │ SAVE: 保存种子文件                       │          │
│  │   (Action)   │     │ INJECT: 注入到客户端 + 文件链接          │          │
│  └──────────────┘     └─────────────────────────────────────────┘          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 核心数据结构

### Searchee (搜索源)

```typescript
interface Searchee {
    infoHash?: string;      // 种子来源 (客户端/种子文件)
    path?: string;          // 数据目录来源
    files: File[];          // 文件列表
    name: string;           // 原始名称
    title: string;          // 解析后的标题
    length: number;         // 总大小
    mtimeMs?: number;       // 修改时间
    clientHost?: string;    // 客户端标识
    savePath?: string;      // 保存路径
    category?: string;      // 分类
    tags?: string[];        // 标签
    trackers?: string[];    // Tracker 列表
    label?: SearcheeLabel;  // 来源标签
}

interface File {
    name: string;   // 文件名
    path: string;   // 相对路径
    length: number; // 文件大小
}
```

### Searchee 来源类型

```typescript
enum SearcheeSource {
    CLIENT = "torrentClient",   // 从 BT 客户端获取
    TORRENT = "torrentFile",    // 从种子文件获取
    DATA = "dataDir",           // 从数据目录获取
    VIRTUAL = "virtual",        // 虚拟 (季包组合)
}
```

---

## 匹配决策引擎

### 决策类型

```typescript
enum Decision {
    // 匹配成功
    MATCH = "MATCH",                    // 完美匹配 (文件名+大小)
    MATCH_SIZE_ONLY = "MATCH_SIZE_ONLY", // 仅大小匹配
    MATCH_PARTIAL = "MATCH_PARTIAL",    // 部分匹配
    
    // 匹配失败
    RELEASE_GROUP_MISMATCH,   // 发布组不匹配
    RESOLUTION_MISMATCH,      // 分辨率不匹配
    SOURCE_MISMATCH,          // 来源不匹配 (AMZN/NF等)
    PROPER_REPACK_MISMATCH,   // REPACK/PROPER 不匹配
    FUZZY_SIZE_MISMATCH,      // 模糊大小不匹配
    SIZE_MISMATCH,            // 大小不匹配
    FILE_TREE_MISMATCH,       // 文件树不匹配
    PARTIAL_SIZE_MISMATCH,    // 部分大小不匹配
    
    // 其他状态
    SAME_INFO_HASH,           // 相同 InfoHash
    INFO_HASH_ALREADY_EXISTS, // InfoHash 已存在
    MAGNET_LINK,              // 磁力链接
    RATE_LIMITED,             // 速率限制
    DOWNLOAD_FAILED,          // 下载失败
    BLOCKED_RELEASE,          // 被阻止的发布
}
```

### 匹配流程

```
┌─────────────────────────────────────────────────────────────────┐
│                    assessCandidate() 匹配流程                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. 预下载过滤 (仅 Candidate)                                   │
│     ├── releaseGroupDoesMatch()  发布组匹配                     │
│     ├── resolutionDoesMatch()    分辨率匹配 (1080p/720p等)      │
│     ├── sourceDoesMatch()        来源匹配 (AMZN/NF/DSNP等)      │
│     ├── releaseVersionDoesMatch() REPACK/PROPER 匹配            │
│     └── fuzzySizeDoesMatch()     模糊大小匹配 (±2%)             │
│                                                                 │
│  2. 下载种子文件                                                │
│     └── snatch() → Metafile                                     │
│                                                                 │
│  3. 后下载检查                                                  │
│     ├── InfoHash 排重检查                                       │
│     ├── 黑名单检查                                              │
│     └── 单集过滤 (可选)                                         │
│                                                                 │
│  4. 文件树匹配                                                  │
│     ├── compareFileTrees()       → MATCH (完美匹配)             │
│     ├── compareFileTreesIgnoringNames() → MATCH_SIZE_ONLY       │
│     └── compareFileTreesPartial() → MATCH_PARTIAL               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 文件树对比算法

```typescript
// 完美匹配: 文件名 + 大小 + 路径
function compareFileTrees(candidate: Metafile, searchee: Searchee): boolean {
    return candidate.files.every((elOfA) =>
        searchee.files.some((elOfB) => 
            elOfB.length === elOfA.length && 
            elOfB.path === elOfA.path
        )
    );
}

// 大小匹配: 仅文件大小
function compareFileTreesIgnoringNames(
    candidate: Metafile, 
    searchee: Searchee
): boolean {
    const availableFiles = searchee.files.slice();
    for (const candidateFile of candidate.files) {
        let matched = availableFiles.filter(
            f => f.length === candidateFile.length
        );
        if (matched.length > 1) {
            matched = matched.filter(f => f.name === candidateFile.name);
        }
        if (matched.length === 0) return false;
        availableFiles.splice(availableFiles.indexOf(matched[0]), 1);
    }
    return true;
}

// 部分匹配: 计算匹配比例
function getPartialSizeRatio(candidate: Metafile, searchee: Searchee): number {
    let matchedSizes = 0;
    for (const candidateFile of candidate.files) {
        const hasFile = searchee.files.some(
            f => f.length === candidateFile.length
        );
        if (hasFile) matchedSizes += candidateFile.length;
    }
    return matchedSizes / candidate.length;
}
```

---

## 媒体类型识别

### 正则表达式解析

```typescript
// 剧集: Show.Name.S01E02.1080p.WEB-DL
const EP_REGEX = /^(?<title>.+?)[_.\s-]+(?:(?<season>S\d+)?[_.\s-]{0,3}
    (?<episode>(?:E|(?<=S\d+[_\s-]{1,3}))\d+(?:[\s-]?(?!(?:19|20)\d{2})E?\d+)?))/i;

// 季包: Show.Name.Season.1
const SEASON_REGEX = /^(?<title>.+?)[[(_.\s-]+(?<season>S(?:eason)?\s*\d+)/i;

// 电影: Movie.Name.2024.1080p
const MOVIE_REGEX = /^(?<title>.+?)-?[_.\s][[(]?(?<year>(?:18|19|20)\d{2})[)\]]?/i;

// 动漫: [Group] Title - 01 [1080p]
const ANIME_REGEX = /^(?:\[(?<group>.*?)\][_\s-]?)?(?:\[?(?<title>.+?)[_\s-]?
    (?:\(?(?:\d{1,2}(?:st|nd|rd|th))?\s?Season)?[_\s-]?\]?)
    [_\s-]?(?:#|EP?|(?:SP))?[_\s-]{0,3}(?<release>\d{1,4})/i;
```

### 媒体类型枚举

```typescript
enum MediaType {
    EPISODE = "episode",   // 单集
    SEASON = "pack",       // 季包
    MOVIE = "movie",       // 电影
    ANIME = "anime",       // 动漫
    VIDEO = "video",       // 其他视频
    AUDIO = "audio",       // 音频
    BOOK = "book",         // 电子书
    OTHER = "unknown",     // 未知
}
```

---

## Torznab 搜索集成

### 搜索类型支持

```typescript
interface Query extends IdSearchParams {
    t: "caps" | "search" | "tvsearch" | "movie";
    q?: string;           // 搜索关键词
    limit?: number;       // 结果限制
    offset?: number;      // 偏移量
    season?: number;      // 季号
    ep?: number;          // 集号
}

interface IdSearchParams {
    tvdbid?: string;      // TVDB ID
    tmdbid?: string;      // TMDB ID
    imdbid?: string;      // IMDB ID
    tvmazeid?: string;    // TVMaze ID
}
```

### 搜索流程

```
┌─────────────────────────────────────────────────────────────────┐
│                    searchTorznab() 搜索流程                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. 获取启用的索引器                                            │
│     └── getEnabledIndexers()                                    │
│                                                                 │
│  2. 构建搜索查询                                                │
│     ├── getSearchString() → 清理标题                            │
│     ├── getVideoQueries() → 视频查询                            │
│     └── getAnimeQueries() → 动漫查询                            │
│                                                                 │
│  3. ID 搜索 (可选)                                              │
│     ├── scanAllArrsForMedia() → Sonarr/Radarr 查询              │
│     └── getRelevantArrIds() → 获取外部 ID                       │
│                                                                 │
│  4. 并行请求索引器                                              │
│     └── Promise.all(indexers.map(queryIndexer))                 │
│                                                                 │
│  5. 解析 XML 响应                                               │
│     └── parseTorznabResults() → Candidate[]                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 预过滤系统

### 过滤规则

```typescript
async function filterByContent(searchee: SearcheeWithLabel): Promise<boolean> {
    // 1. 黑名单检查
    if (findBlockedStringInReleaseMaybe(searchee, blockList)) {
        return false;
    }
    
    // 2. 单集过滤 (可选)
    if (!includeSingleEpisodes && isSingleEpisode(searchee, mediaType)) {
        return false;
    }
    
    // 3. 非视频内容过滤
    const nonVideoSizeRatio = calculateNonVideoRatio(searchee);
    if (!includeNonVideos && nonVideoSizeRatio > fuzzySizeThreshold) {
        return false;
    }
    
    // 4. 跨站种子过滤 (避免重复搜索)
    if (ignoreCrossSeeds && isCrossSeed(searchee)) {
        return false;
    }
    
    // 5. Arr 目录过滤
    if (looksLikeArrDirectory(searchee)) {
        return false;
    }
    
    return true;
}
```

### 时间戳过滤

```typescript
async function filterTimestamps(searchee: Searchee): Promise<boolean> {
    const { excludeOlder, excludeRecentSearch } = getRuntimeConfig();
    
    const lastSearch = await db("timestamp")
        .where({ name: searchee.title })
        .first();
    
    // 排除太久没搜索的
    if (excludeOlder && lastSearch?.first_searched < excludeOlder) {
        return false;
    }
    
    // 排除最近搜索过的
    if (excludeRecentSearch && lastSearch?.last_searched > excludeRecentSearch) {
        return false;
    }
    
    return true;
}
```

---

## 动作执行系统

### 动作类型

```typescript
enum Action {
    SAVE = "save",     // 保存种子文件到 outputDir
    INJECT = "inject", // 注入到客户端
}
```

### INJECT 动作流程

```
┌─────────────────────────────────────────────────────────────────┐
│                    performAction() 注入流程                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. 获取保存路径                                                │
│     └── getSavePath(searchee)                                   │
│                                                                 │
│  2. 确定目标目录                                                │
│     ├── 客户端默认保存路径                                      │
│     └── linkDir 配置路径                                        │
│                                                                 │
│  3. 文件链接                                                    │
│     └── linkAllFilesInMetafile()                                │
│         ├── Hardlink (硬链接)                                   │
│         ├── Symlink (符号链接)                                  │
│         └── Reflink (写时复制)                                  │
│                                                                 │
│  4. 注入种子到客户端                                            │
│     └── client.inject(newTorrent, searchee, decision)           │
│                                                                 │
│  5. 处理结果                                                    │
│         ├── SUCCESS → 记录成功                                  │
│         ├── ALREADY_EXISTS → 跳过                               │
│         ├── TORRENT_NOT_COMPLETE → 保存待注入                   │
│         └── FAILURE → 保存种子文件                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 文件链接实现

```typescript
async function linkAllFilesInMetafile(
    searchee: Searchee,
    newMeta: Metafile,
    decision: DecisionAnyMatch,
    destinationDir: string,
): Promise<Result<LinkResult, Error>> {
    const availableFiles = searchee.files.slice();
    
    // 匹配文件
    const paths = newMeta.files.reduce((acc, newFile) => {
        let matched = availableFiles.filter(
            f => f.length === newFile.length
        );
        if (matched.length > 1) {
            matched = matched.filter(f => f.name === newFile.name);
        }
        if (!matched.length) return acc;
        
        const srcFilePath = join(savePath, matched[0].path);
        const destFilePath = join(destinationDir, newFile.path);
        acc.push([srcFilePath, destFilePath]);
        return acc;
    }, []);
    
    // 执行链接
    for (const [src, dest] of paths) {
        await linkFile(src, dest, linkType);
    }
}
```

---

## Arr 集成

### 外部 ID 获取

```typescript
interface ExternalIds {
    imdbId?: string;
    tmdbId?: string;
    tvdbId?: string;
    tvMazeId?: string;
}

async function scanAllArrsForMedia(
    searcheeTitle: string,
    mediaType: MediaType,
): Promise<Result<ParsedMedia, boolean>> {
    // 根据媒体类型选择 Arr 实例
    const uArrLs = getRelevantArrInstances(mediaType);
    // EPISODE/SEASON → Sonarr
    // MOVIE → Radarr
    // ANIME/VIDEO → Sonarr + Radarr
    
    // 调用 Arr API 解析标题
    return makeArrApiCall(uArrL, "/api/v3/parse", { title });
}
```

### ID 搜索优势

```
普通搜索:  q=Show.Name.S01E02
          ↓ 可能匹配到不同版本

ID 搜索:   tvsearch + tvdbid=12345 + season=1 + ep=2
          ↓ 精确匹配同一剧集
```

---

## RSS 扫描模式

```typescript
async function scanRssFeeds(): Promise<void> {
    // 1. 获取所有 RSS 候选
    const candidates = await queryRssFeeds(indexers);
    
    // 2. 为每个候选查找匹配的搜索源
    for (const candidate of candidates) {
        // 方式1: 通过键值匹配
        const { searchees, method } = await getSearcheesForCandidate(
            candidate, 
            Label.RSS
        );
        
        // 方式2: 通过 Ensemble 匹配 (季包组合)
        const ensemble = await getEnsembleForCandidate(candidate);
        
        // 3. 评估并执行
        if (searchees) {
            await assessAndInject(candidate, searchees);
        }
    }
}
```

### 键值匹配算法

```typescript
// 提取标题键值
function getEpisodeKeys(title: string): { keyTitle: string; season: number; episode: number } | null;
function getSeasonKeys(title: string): { keyTitle: string; season: number } | null;
function getMovieKeys(title: string): { keyTitle: string; year: number } | null;
function getAnimeKeys(title: string): { keyTitle: string; release: number } | null;

// 数据库查找
async function getSimilarByName(name: string): Promise<{
    keys: string[];
    clientSearchees: Searchee[];
    dataSearchees: Searchee[];
}> {
    const keys = extractKeys(name);
    // 从数据库查找匹配的 searchee
    const clientSearchees = await db("client_searchee").whereIn("key", keys);
    const dataSearchees = await db("data").whereIn("key", keys);
    return { keys, clientSearchees, dataSearchees };
}
```

---

## 性能优化策略

### 1. 并发控制

```typescript
// 信号量限制并发
const semaphore = new AsyncSemaphore(MAX_CONCURRENCY);

// 批量处理
await inBatches(items, async (batch) => {
    await Promise.all(batch.map(processItem));
});
```

### 2. 缓存机制

```typescript
// 种子缓存
async function cacheTorrentFile(meta: Metafile, candidate: Candidate): Promise<boolean> {
    const torrentPath = join(TORRENT_CACHE_FOLDER, `${meta.infoHash}.cached.torrent`);
    await writeFile(torrentPath, meta.encode());
}

// 搜索缓存
interface CachedSearch {
    q: string | null;
    indexerCandidates: IndexerCandidates[];
    lastSearch: number;
}
```

### 3. 速率限制处理

```typescript
// 索引器状态跟踪
enum IndexerStatus {
    OK = "ok",
    RATE_LIMITED = "rate_limited",
}

// 自动重试
async function snatch(candidate: Candidate, options: { retries: number; delayMs: number }) {
    for (let i = 0; i <= retries; i++) {
        const result = await snatchOnce(candidate);
        if (result instanceof Metafile) return result;
        if (result.snatchError === SnatchError.RATE_LIMITED) {
            await wait(result.retryAfterMs ?? delayMs);
        }
    }
}
```

---

## 匹配模式对比

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **STRICT** | 严格匹配，文件名+大小+路径必须完全一致 | 高精度需求 |
| **FLEXIBLE** | 灵活匹配，允许文件名不同但大小相同 | 常规使用 |
| **PARTIAL** | 部分匹配，允许部分文件缺失 | 季包/合集 |

---

## 配置示例

```javascript
// config.js
module.exports = {
    // 索引器配置
    torznab: [
        "http://prowlarr:9696/1/api?apikey=xxx",
        "http://prowlarr:9696/2/api?apikey=xxx",
    ],
    
    // 客户端配置
    torrentClients: [
        "qbittorrent:http://user:pass@localhost:8080",
        "rtorrent:http://localhost:8080/RPC2",
    ],
    
    // 匹配配置
    matchMode: "flexible",
    fuzzySizeThreshold: 0.02,
    includeSingleEpisodes: false,
    
    // 链接配置
    linkDirs: ["/data/links"],
    linkType: "hardlink",
    flatLinking: false,
    
    // 搜索配置
    searchCadence: "1 day",
    rssCadence: "30 minutes",
    excludeOlder: "2 weeks",
    excludeRecentSearch: "3 days",
    
    // Arr 集成
    sonarr: ["http://localhost:8989/api?apikey=xxx"],
    radarr: ["http://localhost:7878/api?apikey=xxx"],
};
```

---

## 总结

Cross-Seed 的跨站辅种功能设计精良，核心亮点包括：

1. **多维度匹配** - 发布组、分辨率、来源、大小、文件树层层过滤
2. **三种匹配模式** - 适应不同精度需求
3. **智能缓存** - 种子缓存、搜索缓存减少重复请求
4. **Arr 集成** - 利用外部 ID 实现精确匹配
5. **文件链接** - 硬链接/符号链接节省磁盘空间
6. **RSS 实时监控** - 自动发现新资源

这是一个功能完整、设计合理的自动化辅种解决方案。
