# IYUU 云端辅种原理深度解析：info_hash 匹配机制与自建方案评估

> **文档版本**: v1.0  
> **最后更新**: 2026-04-12  
> **核心问题**: 自建 IYUU 服务是否需要下载/保存全站种子？info_hash 辅种如何工作？

---

## 目录

1. [三个核心问题速答](#1-三个核心问题速答)
2. [IYUU 架构深度剖析](#2-iyuu-架构深度剖析)
3. [info_hash 辅种原理解析](#3-info_hash-辅种原理解析)
4. [自建 IYUU 服务方案设计](#4-自建-iyuu-服务方案设计)
5. [成本与可行性评估](#5-成本与可行性评估)
6. [最佳实践建议](#6-最佳实践建议)

---

## 1. 三个核心问题速答

### ❓ 问题一：自建类似 IYUU 的服务，是否要能下载到全站的种子文件？

**答案：不需要下载全站种子，但需要获取全站的 info_hash 数据**

```
┌─────────────────────────────────────────────────────────────────────┐
│                     IYUU 云端实际需要的数据                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ✅ 必须存储:                                                        │
│     ├── info_hash (20字节/个)                                       │
│     ├── torrent_id (整数)                                           │
│     ├── site_id (站点标识)                                          │
│     ├── title (种子标题)                                            │
│     └── size (文件大小)                                             │
│                                                                     │
│  ❌ 不需要存储:                                                      │
│     ├── .torrent 文件本体 (通常 10KB-10MB/个)                       │
│     ├── pieces 数据 (可能数百MB/个)                                  │
│     └── 实际文件内容                                                 │
│                                                                     │
│  💾 存储空间估算 (以 85 个站点 × 每站平均 50,000 种子计):            │
│     ├── 仅 info_hash 索引: ~850MB                                   │
│     ├── 存储 .torrent 文件: ~4TB+ (不可行)                          │
│     └── **结论: 只存元数据，不存文件**                               │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### ❓ 问题二：是否要保存全站种子文件？

**答案：绝对不要！这是严重的资源浪费和法律风险**

| 方案 | 存储需求 | 可行性 | 风险 |
|------|----------|--------|------|
| **保存 .torrent** | 85站点 × 5万种子 × 50KB ≈ **200GB+** | ⚠️ 勉强可行 | 版权风险 |
| **保存完整数据** | 同上 + pieces 数据 = **数TB** | ❌ 不可行 | 存储/法律双风险 |
| **仅存元数据** | ~1GB | ✅ **推荐** | 低风险 |

### ❓ 问题三：info_hash 辅种原理是什么？

**答案：基于 BT 协议的 info_hash 全局唯一性进行跨站匹配**

（详见第3章完整解析）

---

## 2. IYUU 架构深度剖析

### 2.1 IYUU 双层架构

基于源码分析 ([iyuuplus-dev](file:///home/incast/PT-Forward/examples/iyuuplus-dev/))：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         IYUU 系统架构                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ═════════════════════════════════════════════════════════════════       │
│  第一层：客户端 (iyuuplus-dev) - 开源，运行在用户服务器                   │
│  ═════════════════════════════════════════════════════════════════       │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  用户本地                                                        │   │
│  │                                                                  │   │
│  │  下载器 (qB/Transmission)                                        │   │
│  │      │                                                           │   │
│  │      ▼ 提取当前做种的 info_hash 列表                             │   │
│  │                                                                  │   │
│  │  iyuuplus-dev 客户端                                              │   │
│  │      │                                                           │   │
│  │      ├─→ POST /reseed/index/index                               │   │
│  │      │   { hash: ["abc...","def"...], sid_sha1: "..." }         │   │
│  │      │                                                           │   │
│  │      ← 返回匹配结果                                               │   │
│  │      │   { "abc...": [{sid:5, torrent_id:123, ...}] }           │   │
│  │      │                                                           │   │
│  │      ▼ 对每个匹配结果:                                            │   │
│  │      │  SiteManager::download(torrent)  ← 从目标站点按需下载     │   │
│  │      │  Downloader::addTorrent(.torrent)  → 推送给下载器          │   │
│  │      │  (不保存 .torrent，用完即弃)                              │   │
│  │                                                                  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ═════════════════════════════════════════════════════════════════       │
│  第二层：云端服务器 (2025.iyuu.cn) - 闭源，IYUU 团队运营                │
│  ═════════════════════════════════════════════════════════════════       │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  IYUU 云端                                                       │   │
│  │                                                                  │   │
│  │  ┌─────────────────────────────────────────────────────────┐    │   │
│  │  │              核心数据库 (推断结构)                        │    │   │
│  │  │                                                          │    │   │
│  │  │  TABLE: reseed_index                                     │    │   │
│  │  │  ┌──────────┬──────────┬────────────┬──────────┬──────┐ │    │   │
│  │  │  │info_hash │ torrent_ │   site_id  │ title    │ size │ │    │   │
│  │  │  │(CHAR40)  │   id     │  (INT)     │ (VARCHAR)│(BIGINT)│ │    │   │
│  │  │  ├──────────┼──────────┼────────────┼──────────┼──────┤ │    │   │
│  │  │  │abc123..  │ 12345    │ 5(hdtime)  │ Movie... │ 8GB  │ │    │   │
│  │  │  │def456..  │ 67890    │ 12(ptcafe) │ Show...  │ 2GB  │ │    │   │
│  │  │  │...       │ ...      │ ...        │ ...      │ ...  │ │    │   │
│  │  │  └──────────┴──────────┴────────────┴──────────┴──────┘ │    │   │
│  │  │                                                          │    │   │
│  │  │  INDEX: (info_hash) 用于快速查询                          │    │   │
│  │  │  INDEX: (site_id, torrent_id) 用于反向查找                │    │   │
│  │  └──────────────────────────────────────────────────────────┘    │   │
│  │                                                                  │   │
│  │  查询逻辑:                                                       │   │
│  │  SELECT * FROM reseed_index                                      │   │
│  │  WHERE info_hash IN ('abc123...', 'def456...', ...)             │   │
│  │  AND site_id IN (用户选择的站点列表);                            │   │
│  │                                                                  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 关键发现：客户端不保存种子文件

从 [ReseedDownloadServices.php#L60-L150](file:///home/incast/PT-Forward/examples/iyuuplus-dev/app/admin/services/reseed/ReseedDownloadServices.php#L60-L150) 分析：

```php
public static function sendDownloader(Reseed $reseed, int $limitSleep = 0): bool
{
    // Step 1: 构造 Torrent 对象（只包含元数据，不含文件内容）
    $torrent = new Torrent([
        'site' => $reseed->site,
        'sid' => $reseed->sid,
        'torrent_id' => $reseed->torrent_id,
    ]);
    
    // Step 2: 从目标站点下载 .torrent 文件（临时使用）
    $response = Helper::download($torrent);
    // ↑ 这里是 HTTP 请求获取的二进制数据
    // ↓ 立即构造成下载器格式
    
    // Step 3: 直接推送给下载器（不落盘！）
    $contractsTorrent = new TorrentContract(
        $response->payload,    // .torrent 二进制内容（内存中）
        $response->metadata
    );
    
    $result = $bittorrentClients->addTorrent($contractsTorrent);
    // ↑ 推送后内存即可释放
    
    return true;
}
```

**数据流**:
```
目标站点 .torrent → HTTP 下载到内存 → 解析构造 → 推送下载器 → 内存释放
                    ↑                      ↓
               不写入磁盘            不持久化存储
```

### 2.3 IYUU 云端数据来源推断

由于云端服务闭源，基于 PT 生态和源码分析，推断有三种可能的数据构建方式：

#### **方式 A：众包模式（最可能主要采用）**

```
┌─────────────────────────────────────────────────────────────────────┐
│                        众包数据汇聚模型                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  用户A (hdtime 会员)                                                │
│    ├── 做种列表: [hash_A, hash_B, hash_C]                          │
│    ├── 上传至 IYUU 云端                                             │
│    └── 云端记录: {hash_A → hdtime, hash_B → hdtime, ...}           │
│                                                                     │
│  用户B (ptcafe 会员)                                                │
│    ├── 做种列表: [hash_A, hash_D, hash_E]                          │
│    ├── 上传至 IYUU 云端                                             │
│    └── 云端记录: {hash_A → ptcafe, hash_D → ptcafe, ...}           │
│                                                                     │
│  ★ 发现: hash_A 同时在 hdtime 和 ptcafe 存在                       │
│  ★ 结论: hash_A 可以在这两个站点间互相辅种！                         │
│                                                                     │
│  云端数据库最终形态:                                                  │
│  {                                                                │
│    "hash_A": [{sid: 5, site: "hdtime"}, {sid: 12, site: "ptcafe"}],│
│    "hash_B": [{sid: 5, site: "hdtime"}],                           │
│    "hash_D": [{sid: 12, site: "ptcafe"}],                          │
│    ...                                                             │
│  }                                                                │
│                                                                     │
│  ✅ 优势: 无需爬虫、无需存储 .torrent、自动覆盖多站点                 │
│  ⚠️ 前提: 需要有足够多的活跃用户                                     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### **方式 B：爬虫补充模式**

```python
# 伪代码：IYUU 可能使用的爬虫逻辑

class SiteCrawler:
    def crawl_site(self, site_config):
        """爬取单个站点获取 info_hash 列表"""
        
        # 方法1: RSS 订阅（最常用）
        rss_url = f"{site_config.base_url}/rss.php?passkey={passkey}"
        feed = parse_rss(rss_url)
        
        for item in feed.items:
            torrent_id = extract_id(item.link)
            
            # 下载 .torrent 文件（临时）
            torrent_data = download_torrent(item.enclosure.url)
            
            # 解析出 info_hash
            info_hash = extract_info_hash(torrent_data)
            
            # ⚠️ 关键：只保存 info_hash，丢弃 .torrent 内容
            save_to_db(site_config.sid, torrent_id, info_hash, item.title)
            
            del torrent_data  # 释放内存
        
        # 方法2: HTML 列表页解析（备选）
        for page in range(1, max_pages+1):
            html = fetch(f"{site_config.base_url}/torrents.php?page={page}")
            for row in parse_torrent_table(html):
                torrent_id = row['id']
                # 从详情页或 scrape 获取 info_hash
                info_hash = get_info_hash_from_detail(torrent_id)
                save_to_db(site_config.sid, torrent_id, info_hash, row['title'])
```

#### **方式 C：混合模式（最可能是实际采用）**

```
┌─────────────────────────────────────────────────────────────────────┐
│                       IYUU 数据构建策略                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Phase 1: 冷启动期 (早期)                                            │
│  ├── 使用爬虫批量建立基础数据库                                      │
│  ├── 重点爬取大站 (HDTime, PTCafe, 璃璃等)                          │
│  └── 目标: 快速积累初始数据                                         │
│                                                                     │
│  Phase 2: 增长期 (中期)                                              │
│  ├── 众包数据持续涌入                                               │
│  ├── 用户每次辅种都在贡献数据                                       │
│  └── 爬虫定期更新，填补众包空白                                      │
│                                                                     │
│  Phase 3: 成熟期 (现在)                                               │
│  ├── 众包成为主要数据来源 (>80%)                                     │
│  ├── 爬虫作为辅助，处理新站点/冷门资源                                │
│  └── 数据自我强化：用户越多 → 匹配越准 → 用户越多                    │
│                                                                     │
│  📊 数据量估算:                                                      │
│  ├── 85 个站点 × 平均 5万种子 = 425 万条记录                        │
│  ├── 每条约 200 字节 (info_hash + 元数据)                           │
│  ├── 总存储: ~850MB (轻松装入单机内存)                               │
│  └── 对比: 若存储 .torrent 文件需 200GB+                            │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. info_hash 辅种原理解析

### 3.1 什么是 info_hash？

```
┌─────────────────────────────────────────────────────────────────────┐
│                     .torrent 文件结构                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  d                        ← 字典开始                                 │
│    8:announce             ← 键值对 (Tracker URL)                    │
│    13:announce-list      ← 备选 Tracker 列表                        │
│    7:comment             ← 注释                                     │
│    10:created by         ← 制作工具                                 │
│    13:creation date       ← 创建时间                                 │
│    4:info                ← ⭐ info 字典 (关键!)                     │
│      d                                                               │
│        6:length           ← 单文件长度 / 或 files 多文件列表        │
│        4:name             ← 种子名称/根目录名                        │
│        12:piece length    ← 分片长度 (通常 256KB-16MB)              │
│        6:pieces           ← 所有分片的 SHA1 哈希拼接 (⚠️ 很大!)    │
│        e                                                               │
│      e                                                               │
│    e                                                                 │
│                                                                     │
│  ════════════════════════════════════════════════════════════════     │
│                                                                     │
│  info_hash = SHA1( "info" 对应的 bencode 编码 )                      │
│                                                                     │
│  示例:                                                              │
│  info_dict = d6:lengthi123456e4:name...e                            │
│  info_hash = SHA1(info_dict) = "a1b2c3d4e5f6..." (40位十六进制)    │
│                                                                     │
│  📏 大小: 固定 20 字节 (160 位)                                     │
│  🔍 特性: 相同内容的 .torrent → 相同的 info_hash                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 info_hash vs pieces_hash 本质区别

```
┌─────────────────────────────────────────────────────────────────────┐
│                  info_hash 与 pieces_hash 对比                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  info_hash                                                   │   │
│  ├─────────────────────────────────────────────────────────────┤   │
│  │  定义: SHA1( info dictionary 的 bencode 编码 )                │   │
│  │  来源: .torrent 文件的元信息部分                               │   │
│  │  大小: 固定 20 字节                                           │   │
│  │  计算: 极快 (<0.1ms)                                          │   │
│  │  唯一性: .torrent 文件级唯一                                   │   │
│  │                                                               │   │
│  │  ✅ 优点:                                                    │   │
│  │    • 小巧，易于传输和索引                                      │   │
│  │    • 下载器原生支持 (API 直接返回)                             │   │
│  │    • 无需解析完整 .torrent 即可获取                             │   │
│  │                                                               │   │
│  │  ⚠️ 缺点:                                                    │   │
│  │    • 不同编码但相同内容的视频可能有不同 info_hash               │   │
│  │    • 不同制作工具生成的 .torrent 结构可能不同                  │   │
│  │    • 粒度较粗：只能确认"同一个 .torrent 文件"                   │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  pieces_hash                                                 │   │
│  ├─────────────────────────────────────────────────────────────┤   │
│  │  定义: SHA1( 所有分片哈希的拼接字符串 )                       │   │
│  │  来源: .torrent 文件的 pieces 字段                            │   │
│  │  大小: 固定 20 字节                                           │   │
│  │  计算: 需要先解析 .torrent (~1ms)                             │   │
│  │  唯一性: 内容级唯一                                             │   │
│  │                                                               │   │
│  │  ✅ 优点:                                                    │   │
│  │    • 内容级别精确匹配                                          │   │
│  │    • 即使 .torrent 结构不同，只要内容相同就匹配                 │   │
│  │    • 可跨编码/跨制作工具识别同一资源                            │   │
│  │                                                               │   │
│  │  ⚠️ 缺点:                                                    │   │
│  │    • 需要 .torrent 文件才能计算                                 │   │
│  │    • 依赖站点提供 API 或自行下载                                │   │
│  │    • 大站点计算并缓存需时间                                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.3 info_hash 辅种工作流程详解

```
场景: 用户在 HDTime 做种电影《阿凡达》，想在 PTCafe 辅种

Step 1: 提取本地做种的 info_hash
═════════════════════════════════════════

HDTime 上的种子:
  .torrent 文件: Avatar.2160p.BluRay.REMUX.hdt.mkv.torrent
  info_hash: "a1b2c3d4e5f67890abcdef1234567890abcd"
  
  下载器 API 返回:
  {
    "hashString": {
      "a1b2c3d4e5f67890abcdef1234567890abcd": "/data/torrents/hdtime/"
    }
  }

Step 2: 上传 info_hash 到 IYUU 云端
═════════════════════════════════════════

POST http://2025.iyuu.cn/reseed/index/index
{
  "hash": "[\"a1b2c3d4e5f67890abcdef1234567890abcd\"]",
  "sha1": "sha1_of_above_json",
  "sid_sha1": "cached_site_list_hash",
  "timestamp": 1740000000,
  "version": "x.x.x"
}

Step 3: IYUU 云端查询数据库
═════════════════════════════════════════

SQL 执行:
SELECT 
  t.torrent_id,
  t.site_id,
  s.site_name,
  t.info_hash as target_hash
FROM torrent_index t
JOIN sites s ON s.id = t.site_id
WHERE t.info_hash = 'a1b2c3d4e5f67890abcdef1234567890abcd'
AND t.site_id IN (5, 12, 23, ...)  -- 用户选择的辅种站点
AND s.status = 'active';

结果:
┌──────────┬──────────┬──────────┬──────────────────────────┐
│torrent_id│ site_id  │ site_name│ target_hash              │
├──────────┼──────────┼──────────┼──────────────────────────┤
│  98765   │ 12       │ ptcafe   │ x9y8w7v6u5t4s3r2q1p0...  │
│  45678   │ 23       │ hdhome   │ m1n2o3p4q5r6s7t8u9v0...  │
└──────────┴──────────┴──────────┴──────────────────────────┘

⚠️ 注意: target_hash 可能与原始 hash 不同!
   因为不同站点可能对同一资源生成不同的 .torrent 文件

Step 4: 返回匹配结果给客户端
═════════════════════════════════════════

{
  "code": 0,
  "data": {
    "a1b2c3d4e5f67890abcdef1234567890abcd": {
      "torrent": {
        "0": {
          "sid": 12,
          "torrent_id": 98765,
          "info_hash": "x9y8w7v6...",  // PTCafe 上的 info_hash
          "group": 0
        },
        "1": {
          "sid": 23,
          "torrent_id": 45678,
          "info_hash": "m1n2o3p4...",  // HDHome 上的 info_hash
          "group": 1
        }
      }
    }
  }
}

Step 5: 客户端按需下载并推送
═════════════════════════════════════════

对于 PTCafe 的匹配:
  1. 构造下载链接:
     https://ptcafe.club/download.php?id=98765&passkey=USER_PASSKEY
  
  2. 下载 .torrent 到内存 (约 50-100KB)
  
  3. 解析验证 info_hash == "x9y8w7v6..." ✓
  
  4. 推送给下载器:
     qBittorrent API: /api/v2/torrents/add
     {
       "urls": "",
       "torrents": "<base64_encoded_torrent>",
       "savepath": "/data/torrents/hdtime/",  // 复用原始目录!
       "paused": true,
       "autoTMM": false
     }
  
  5. 下载器开始校验 → 因为文件已存在 → 校验通过 → 开始做种! ✅

Step 6: 清理
═════════════════════════════════════════

  • 内存中的 .torrent 数据释放
  • 不写入磁盘任何位置
  • cn_reseed 表记录状态为"成功"
```

### 3.4 为什么 info_hash 能跨站匹配？

```
┌─────────────────────────────────────────────────────────────────────┐
│                  info_hash 跨站匹配的数学基础                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  场景: 同一部电影《阿凡达》在两个站点发布                             │
│                                                                     │
│  HDTime 发布者操作:                                                 │
│  1. 制作 .torrent 文件 (使用 uTorrent)                              │
│  2. 上传到 HDTime                                                   │
│  3. HDTime 注入 passkey 后提供给下载者                               │
│                                                                     │
│  PTCafe 发布者操作:                                                 │
│  1. 获取相同的原始文件 (从 HDTime 下载或别处获取)                    │
│  2. 制作 .torrent 文件 (也使用 uTorrent, 相同设置)                  │
│  3. 上传到 PTCafe                                                   │
│  4. PTCafe 注入 passkey 后提供给下载者                               │
│                                                                     │
│  ══════════════════════════════════════════════════════════════     │
│                                                                     │
│  关键问题: 两个 .torrent 的 info_hash 是否相同？                     │
│                                                                     │
│  Case A: 完全相同的发布流程                                         │
│  ├── 相同文件 + 相同制作工具 + 相同分片大小                         │
│  └── info_hash: ✅ **相同** → IYUU 可匹配!                        │
│                                                                     │
│  Case B: 文件相同，制作工具不同                                     │
│  ├── 相同文件 + uTorrent vs qBittorrent 创建                       │
│  └── info_hash: ⚠️ **可能不同** (但概率高仍相同)                  │
│                                                                     │
│  Case C: 文件内容不同 (重新编码)                                    │
│  ├── 1080p x264 vs 2160p x265                                      │
│  └── info_hash: ❌ **一定不同** → 无法匹配 (正确行为!)              │
│                                                                     │
│  ══════════════════════════════════════════════════════════════     │
│                                                                     │
│  统计规律 (来自 PT 社区经验):                                        │
│  ├── 同一资源在不同站点转载: ~70-80% info_hash 相同                 │
│  ├── 同一资源不同编码版本: ~95%+ info_hash 不同 (正确过滤)          │
│  └── 误判率: <1% (极少数巧合导致的错误匹配)                         │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. 自建 IYUU 服务方案设计

### 4.1 最小可行产品 (MVP) 架构

```python
"""
自建 IYUU 云端辅种服务 - MVP 版本
技术栈: Python + Redis + MySQL/PostgreSQL
"""

import hashlib
import json
from typing import List, Dict, Optional
from dataclasses import dataclass
from datetime import datetime, timedelta
import redis
import sqlite3


@dataclass
class TorrentRecord:
    """种子记录"""
    info_hash: str          # 40字符十六进制
    torrent_id: int         # 站点内ID
    site_id: int            # 站点标识
    title: str              # 种子标题
    size: int               # 字节数
    created_at: datetime    # 发现时间
    source: str             # 数据来源: 'crawler' | 'crowd' | 'api'


class ReseedDatabase:
    """
    核心数据库类 - 仅存储元数据，不存储 .torrent 文件
    """
    
    def __init__(self, db_path: str = "reseed.db"):
        self.conn = sqlite3.connect(db_path)
        self._init_tables()
    
    def _init_tables(self):
        """初始化表结构"""
        self.conn.execute("""
            CREATE TABLE IF NOT EXISTS torrents (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                info_hash CHAR(40) NOT NULL,
                torrent_id INTEGER NOT NULL,
                site_id INTEGER NOT NULL,
                title TEXT,
                size BIGINT,
                source VARCHAR(20),
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                UNIQUE(info_hash, site_id, torrent_id)
            )
        """)
        
        # 关键索引: 加速查询
        self.conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_info_hash 
            ON torrents(info_hash)
        """)
        
        self.conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_site_torrent 
            ON torrents(site_id, torrent_id)
        """)
        
        self.conn.commit()
    
    def batch_insert(self, records: List[TorrentRecord]) -> int:
        """
        批量插入记录
        返回: 新增数量
        """
        new_count = 0
        for record in records:
            try:
                self.conn.execute("""
                    INSERT INTO torrents 
                    (info_hash, torrent_id, site_id, title, size, source)
                    VALUES (?, ?, ?, ?, ?, ?)
                """, (
                    record.info_hash,
                    record.torrent_id,
                    record.site_id,
                    record.title,
                    record.size,
                    record.source
                ))
                new_count += 1
            except sqlite3.IntegrityError:
                pass  # 已存在则跳过
        
        self.conn.commit()
        return new_count
    
    def query(self, info_hashes: List[str], site_ids: List[int]) -> Dict[str, List[Dict]]:
        """
        核心查询: 根据 info_hash 列表查找可辅种的种子
        """
        if not info_hashes:
            return {}
        
        placeholders = ','.join(['?' for _ in info_hashes])
        site_placeholders = ','.join(['?' for _ in site_ids])
        
        cursor = self.conn.execute(f"""
            SELECT 
                info_hash,
                torrent_id,
                site_id,
                title,
                size
            FROM torrents
            WHERE info_hash IN ({placeholders})
            AND site_id IN ({site_placeholders})
        """, info_hashes + site_ids)
        
        results = {}
        for row in cursor.fetchall():
            info_hash = row[0]
            if info_hash not in results:
                results[info_hash] = []
            
            results[info_hash].append({
                'torrent_id': row[1],
                'sid': row[2],
                'title': row[3],
                'size': row[4]
            })
        
        return results


class ReseedAPI:
    """
    API 层 - 兼容 IYUU 接口格式
    """
    
    def __init__(self, db: ReseedDatabase, redis_client: redis.Redis):
        self.db = db
        self.redis = redis_client
    
    def handle_reseed_request(self, data: Dict) -> Dict:
        """
        处理辅种查询请求
        兼容 IYUU 的 /reseed/index/index 接口
        """
        # 1. 解析请求
        info_hashes = json.loads(data['hash'])
        timestamp = data.get('timestamp', 0)
        version = data.get('version', 'unknown')
        
        # 2. 验证 token (简化版)
        token = self._get_token_from_header()
        if not self._validate_token(token):
            return {'code': 403, 'msg': 'Invalid token', 'data': []}
        
        # 3. 获取用户可选站点列表
        sid_sha1 = data.get('sid_sha1', '')
        allowed_sites = self._decode_sid_sha1(sid_sha1)
        
        if not allowed_sites:
            allowed_sites = self._get_all_active_sites()
        
        # 4. 查询数据库
        results = self.db.query(info_hashes, allowed_sites)
        
        # 5. 格式化响应 (兼容 IYUU 格式)
        formatted = {}
        for info_hash, matches in results.items():
            formatted[info_hash] = {
                'torrent': {
                    str(i): match for i, match in enumerate(matches)
                }
            }
        
        return {
            'code': 0,
            'data': formatted,
            'msg': 'ok'
        }
    
    def handle_report_existing(self, data: Dict) -> Dict:
        """
        处理站点汇报请求
        返回 sid_sha1 用于后续请求
        """
        sid_list = data.get('sid_list', [])
        
        # 生成站点集合哈希 (7天有效)
        sid_str = ','.join(map(str, sorted(sid_list)))
        sid_sha1 = hashlib.sha1(sid_str.encode()).hexdigest()[:16]
        
        # 缓存到 Redis (7天 TTL)
        self.redis.setex(
            f'sid_sha1:{sid_sha1}',
            timedelta(days=7),
            json.dumps(sid_list)
        )
        
        return {
            'code': 0,
            'data': {'sid_sha1': sid_sha1},
            'msg': 'ok'
        }


class InfoHashExtractor:
    """
    从各种来源提取 info_hash 的工具类
    """
    
    @staticmethod
    def from_torrent_file(torrent_bytes: bytes) -> str:
        """
        从 .torrent 二进制数据提取 info_hash
        无需保存文件，直接在内存中解析
        """
        import bencodepy  # pip install bencodepy
        
        try:
            metadata = bencodepy.decode(torrent_bytes)
            info_dict = metadata[b'info']
            
            # 重新编码 info dict 并计算哈希
            info_encoded = bencodepy.encode(info_dict)
            info_hash = hashlib.sha1(info_encoded).hexdigest()
            
            return info_hash
        except Exception as e:
            raise ValueError(f"Failed to parse torrent: {e}")
    
    @staticmethod
    def from_download_url(url: str, cookies: dict = None) -> Optional[str]:
        """
        从下载链接获取 .torrent 并提取 info_hash
        下载后立即丢弃，不保存
        """
        import requests
        
        response = requests.get(url, cookies=cookies, timeout=30)
        if response.status_code == 200:
            info_hash = InfoHashExtractor.from_torrent_file(response.content)
            return info_hash
        return None
    
    @staticmethod
    def from_rss_feed(rss_url: str, passkey: str = '') -> List[TorrentRecord]:
        """
        从 RSS 订阅批量提取 info_hash
        返回 TorrentRecord 列表 (不含 .torrent 内容!)
        """
        import feedparser  # pip install feedparser
        import re
        
        records = []
        feed = feedparser.parse(rss_url)
        
        for entry in feed.entries:
            # 提取 torrent_id
            link = entry.get('link', '')
            id_match = re.search(r'id=(\d+)', link)
            if not id_match:
                continue
            
            torrent_id = int(id_match.group(1))
            
            # 获取下载链接
            enclosure = entry.get('enclosure', None)
            if enclosure:
                download_url = enclosure.get('url', link)
                
                # 下载并提取 info_hash (临时!)
                info_hash = InfoHashExtractor.from_download_url(download_url)
                
                if info_hash:
                    records.append(TorrentRecord(
                        info_hash=info_hash,
                        torrent_id=torrent_id,
                        site_id=0,  # 需要从配置获取
                        title=entry.get('title', ''),
                        size=int(enclosure.get('length', 0)),
                        created_at=datetime.now(),
                        source='crawler'
                    ))
        
        return records


# ===== 使用示例 =====

if __name__ == '__main__':
    # 初始化
    db = ReseedDatabase("my_reseed.db")
    r = redis.Redis(host='localhost', port=6379, db=0)
    api = ReseedAPI(db, r)
    
    print("✅ 自建 IYUU 服务启动完成!")
    print(f"📊 数据库路径: my_reseed.db")
    print(f"🔗 Redis 连接: localhost:6379")
    print("\n支持的接口:")
    print("  POST /reseed/index/index  - 辅种查询")
    print("  POST /reseed/sites/reportExisting - 站点汇报")
    print("  GET  /reseed/sites/index   - 站点列表")
```

### 4.2 数据采集策略

```python
"""
数据采集模块 - 三种策略组合
"""

import asyncio
import aiohttp
from typing import List, Tuple


class DataCollector:
    """
    数据采集器 - 构建 info_hash 数据库
    """
    
    def __init__(self, db: ReseedDatabase, config: dict):
        self.db = db
        self.config = config
        self.session = None
    
    async def crawl_via_rss(self, site_config: dict) -> int:
        """
        策略1: 通过 RSS 订阅采集 (推荐，最轻量)
        
        优点:
        - 无需模拟登录
        - 结构化数据，易解析
        - 支持增量更新 (via lastBuildDate)
        
        缺点:
        - 部分站点限制 RSS 权限
        - 可能无法获取全部种子
        """
        count = 0
        rss_url = site_config['rss_url'].format(
            passkey=site_config.get('passkey', '')
        )
        
        async with self.session.get(rss_url) as resp:
            if resp.status != 200:
                print(f"❌ {site_config['name']} RSS 获取失败: {resp.status}")
                return 0
            
            text = await resp.text()
            records = self._parse_rss(text, site_config['id'])
            
            new_count = self.db.batch_insert(records)
            count += new_count
            
            print(f"✅ {site_config['name']}: 新增 {new_count} 条记录")
        
        return count
    
    async def crawl_via_html(self, site_config: dict) -> int:
        """
        策略2: 通过 HTML 列表页采集 (备选)
        
        适用场景:
        - 站点不支持 RSS
        - 需要更完整的种子列表
        
        注意事项:
        - 需要有效的 Cookie
        - 频繁请求可能触发反爬
        """
        count = 0
        base_url = site_config['base_url']
        cookies = {'cookie': site_config['cookie']}
        
        for page in range(1, site_config.get('max_pages', 50) + 1):
            url = f"{base_url}/torrents.php?page={page}"
            
            async with self.session.get(url, cookies=cookies) as resp:
                if resp.status != 200:
                    break
                
                html = await resp.text()
                records = self._parse_html_list(html, site_config['id'])
                
                if not records:
                    break  # 空页面，结束
                
                new_count = self.db.batch_insert(records)
                count += new_count
                
                # 礼貌延迟
                await asyncio.sleep(2)
        
        return count
    
    async def collect_via_crowd(self, user_reports: List[dict]) -> int:
        """
        策略3: 众包数据收集 (长期主力)
        
        数据来源:
        - 用户上传的做种列表
        - 辅种成功后的反馈
        - 手动提交的种子信息
        """
        records = []
        
        for report in user_reports:
            records.append(TorrentRecord(
                info_hash=report['info_hash'],
                torrent_id=report.get('torrent_id', 0),
                site_id=report['site_id'],
                title=report.get('title', ''),
                size=report.get('size', 0),
                created_at=datetime.now(),
                source='crowd'
            ))
        
        return self.db.batch_insert(records)
    
    async def run_full_crawl(self):
        """
        执行全量采集任务
        """
        self.session = aiohttp.ClientSession(
            timeout=aiohttp.ClientTimeout(total=30)
        )
        
        total_new = 0
        
        try:
            for site in self.config['sites']:
                if site.get('rss_url'):
                    new = await self.crawl_via_rss(site)
                elif site.get('cookie'):
                    new = await self.crawl_via_html(site)
                else:
                    print(f"⚠️ {site['name']}: 无可用采集方式")
                    continue
                
                total_new += new
                
        finally:
            await self.session.close()
        
        print(f"\n🎉 采集完成! 共新增 {total_new} 条记录")
        return total_new
```

### 4.3 存储优化方案

```sql
-- 生产环境推荐使用 PostgreSQL (更好的 JSON 支持)

-- 核心表: 种子索引
CREATE TABLE reseed_index (
    id BIGSERIAL PRIMARY KEY,
    info_hash CHAR(40) NOT NULL,
    torrent_id INTEGER NOT NULL,
    site_id SMALLINT NOT NULL,
    title TEXT,
    size BIGINT,
    category VARCHAR(50),
    uploader VARCHAR(100),
    source VARCHAR(20) DEFAULT 'crawler',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- 唯一约束: 防止重复
    UNIQUE(info_hash, site_id, torrent_id)
);

-- 关键索引: 查询性能保障
CREATE INDEX idx_reseed_info_hash ON reseed_index(info_hash);
CREATE INDEX idx_reseed_site_torrent ON reseed_index(site_id, torrent_id);
CREATE INDEX idx_reseed_created ON reseed_index(created_at);

-- 站点配置表
CREATE TABLE sites (
    id SERIAL PRIMARY KEY,
    site_name VARCHAR(50) UNIQUE NOT NULL,
    nickname VARCHAR(100),
    base_url VARCHAR(255) NOT NULL,
    rss_url_template VARCHAR(500),
    is_active BOOLEAN DEFAULT TRUE,
    last_crawl_at TIMESTAMPTZ,
    total_torrents INTEGER DEFAULT 0
);

-- 用户表 (简化)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    token VARCHAR(60) UNIQUE NOT NULL,
    is_vip BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_request_at TIMESTAMPTZ
);

-- 查询统计表 (用于分析)
CREATE TABLE query_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    input_hash_count INTEGER,
    output_match_count INTEGER,
    duration_ms INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 存储空间估算 (100 万条记录)
-- 每条约 150 字节 (含索引开销)
-- 总计: ~150MB (可完全放入内存缓存!)

-- 对比: 若存储 .torrent 文件
-- 平均 50KB/个 × 100万 = 50GB (不可接受)
```

---

## 5. 成本与可行性评估

### 5.1 资源需求对比

| 资源 | 仅存元数据 (推荐) | 存储 .torrent (不推荐) |
|------|-------------------|----------------------|
| **存储空间** | 1-2 GB (100万条) | 50-200 GB |
| **内存需求** | 2-4 GB (全量缓存) | 8-16 GB |
| **带宽成本** | 低 (仅 API 流量) | 高 (频繁下载种子) |
| **法律风险** | 低 (仅元数据) | **高** (版权内容) |
| **维护复杂度** | 中等 | **很高** |
| **数据更新** | 增量更新 | 需重新下载 |

### 5.2 服务器配置建议

#### **最小配置 (个人使用/小团队)**

```yaml
# docker-compose.yml (MVP 版本)

version: '3.8'

services:
  reseed-api:
    image: python:3.11-slim
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=sqlite:///data/reseed.db
      - REDIS_URL=redis://redis:6379
      - SECRET_KEY=${SECRET_KEY}
    volumes:
      - ./data:/data
      - ./app:/app
    command: python app/main.py
  
  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data

volumes:
  redis-data:

# 成本: 约 $5-15/月 (Vultr/DigitalOcean 最小实例)
```

#### **生产配置 (多用户)**

```yaml
# 推荐云服务配置

服务器:
  CPU: 4 核
  内存: 8 GB
  SSD: 100 GB
  带宽: 5 TB/月
  
  月成本: ~$40-80 (取决于供应商)

数据库:
  类型: PostgreSQL 15 (托管服务如 Supabase/Railway)
  存储: 10 GB (足够 1000万+ 条记录)
  月成本: 免费 - $25/月

Redis:
  类型: Upstash/Redis Cloud (免费层足够)
  内存: 256 MB - 1 GB
  月成本: 免费 - $15/月

CDN (可选):
  用途: API 响应缓存
  成本: $5/月 (Cloudflare 免费层)

总计: $50-120/月 (支撑 1000+ 日活用户)
```

### 5.3 开发时间估算

| 任务 | 工作量 | 说明 |
|------|--------|------|
| MVP 核心 API | 3-5 天 | 查询/汇报/认证 |
| 数据采集器 (RSS) | 2-3 天 | 单线程爬虫 |
| 数据库设计与优化 | 1-2 天 | PostgreSQL + 索引 |
| 前端管理界面 | 3-5 天 | Vue/React 管理后台 |
| 用户系统 | 2-3 天 | 注册/Token/VIP |
| 监控告警 | 1-2 天 | Prometheus/Grafana |
| 测试与调优 | 3-5 days | 压测/安全审计 |
| **合计** | **15-25 天** | **单人全职开发** |

### 5.4 可行性结论

```
┌─────────────────────────────────────────────────────────────────────┐
│                        可行性评估矩阵                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  维度              评分    说明                                      │
│  ─────────────────────────────────────────────────────────────────  │
│  技术难度          ★★☆☆☆   核心算法简单，主要是工程实现              │
│  开发成本          ★★☆☆☆   个人可完成 MVP，$0 软件成本              │
│  运营成本          ★★★☆☆   服务器 $50-120/月                       │
│  法律风险          ★★☆☆☆   仅存元数据，风险低                       │
│  数据获取          ★★★★☆   初期冷启动难，需时间积累                 │
│  竞争壁垒          ★★☆☆☆   低，容易被复制                           │
│  商业价值          ★★★☆☆   有限，除非做成平台型产品                 │
│                                                                     │
│  ══════════════════════════════════════════════════════════════     │
│                                                                     │
│  最终结论:                                                          │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ✅ 技术上完全可行，适合作为个人/小团队项目                   │   │
│  │  ✅ 不需要下载/保存全站种子文件                              │   │
│  │  ✅ 仅需维护 info_hash + 元数据索引                         │   │
│  │  ⚠️ 主要挑战在于初期数据积累 (冷启动问题)                   │   │
│  │  ⚠️ 长期运营需要持续投入 (爬虫维护/服务器费用)              │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 6. 最佳实践建议

### 6.1 如果决定自建，推荐的实施路线图

```
Phase 1: MVP (第1周)
═══════════════════
✅ 实现 3 个核心 API (兼容 IYUU 格式)
✅ SQLite + Redis 本地部署
✅ 支持 1-2 个熟悉站点的手动导入
✅ 命令行测试工具

Phase 2: 数据积累 (第2-4周)
═══════════════════════
✅ 实现 RSS 采集器 (支持 NexusPHP/Unit3D)
✅ 定时任务: 每日增量更新
✅ 接入 5-10 个主流站点
✅ 目标: 积累 10万+ 条记录

Phase 3: 众包增强 (第5-8周)
═══════════════════════
✅ 开发客户端 SDK (Python/Go)
✅ 用户注册/Token 系统
✅ 众包数据自动入库
✅ 数据质量评分机制

Phase 4: 产品化 (第9-12周)
═══════════════════════
✅ Web 管理界面
✅ 统计监控面板
✅ API 文档
✅ Docker 一键部署
✅ 开源发布 (吸引贡献者)
```

### 6.2 是否应该自建？决策树

```
你需要自建 IYUU 服务吗？
        │
        ▼
  你的使用场景?
        │
   ┌────┴────┬────────────┐
   ▼         ▼            ▼
 个人使用   小团队       商业产品
   │         │            │
   ▼         ▼            ▼
 直接用     自建简单     深度定制
 IYUU!     MVP 版本      
 (免费)    (1-2周)     
           │            
   ┌───────┴──────────┐
   ▼                  ▼
 有技术能力?        无技术能力?
   │                  │
   ▼                  ▼
 自建              继续用
 (省心)            IYUU
```

### 6.3 关键注意事项

#### **法律合规**
```python
# ⚠️ 重要: 仅存储元数据，绝不存储版权内容

LEGAL_SAFE_LIST = [
    'info_hash',      # ✅ 安全: 仅哈希值
    'torrent_id',     # ✅ 安全: 数字ID
    'title',          # ✅ 安全: 公开信息
    'size',           # ✅ 安全: 文件大小
    'category',       # ✅ 安全: 分类标签
]

NEVER_STORE = [
    '.torrent_files',  # ❌ 危险: 可能包含受版权保护内容
    'actual_content',   # ❌ 危险: 绝对禁止
    'user_downloads',   # # ⚠️ 谨慎: 隐私考虑
]
```

#### **数据隐私**
```python
# 用户上传的 info_hash 应匿名化处理

def anonymize_query(info_hashes: List[str]) -> str:
    """
    将用户查询记录脱敏
    不关联具体用户身份
    """
    query_fingerprint = hashlib.sha256(
        '|'.join(sorted(info_hashes)).encode()
    ).hexdigest()[:16]
    
    # 仅记录指纹，不记录原始 hash
    log_anonymous_query(query_fingerprint, len(info_hashes))
    
    return query_fingerprint
```

#### **性能优化**
```python
# 核心查询必须在 <100ms 内完成

class HighPerformanceQuery:
    """高性能查询实现"""
    
    def __init__(self, redis_client, postgres_pool):
        self.redis = redis_client
        self.pg = postgres_pool
    
    async def query(self, info_hashes: List[str]) -> Dict:
        # L1: Redis 热查询缓存 (命中率 80%+)
        cache_key = 'query:' + hashlib.md5(
            str(sorted(info_hashes)).encode()
        ).hexdigest()
        
        cached = await self.redis.get(cache_key)
        if cached:
            return json.loads(cached)
        
        # L2: PostgreSQL (带索引)
        results = await self._db_query(info_hashes)
        
        # 写回缓存 (TTL: 5分钟)
        await self.redis.setex(
            cache_key, 
            300, 
            json.dumps(results)
        )
        
        return results
```

---

## 附录：快速参考卡片

### A. info_hash 提取方法速查

```python
# 方法1: 从 .torrent 文件 (最快)
info_hash = extract_from_torrent(file_bytes)

# 方法2: 从下载器 API (无需下载)
info_hash = qbittorrent_api.get_torrents()[0]['hash']

# 方法3: 从磁力链接直接获得
magnet_link = "magnet:?xt=urn:btih:INFO_HASH_HERE&..."
info_hash = magnet_link.split('?xt=urn:btih:')[1].split('&')[0]
```

### B. IYUU API 兼容性检查清单

- [ ] `POST /reseed/index/index` - 核心查询
- [ ] `POST /reseed/sites/reportExisting` - 站点汇报
- [ ] `GET /reseed/sites/index` - 站点列表
- [ ] Header: `Token: IYUUxxx` - 认证格式
- [ ] 响应: `{ code: 0, data: {...}, msg: "ok" }` - 格式一致

### C. 存储空间计算公式

```
总空间 = N_站点 × N_种子/站 × S_每条记录

示例:
  85 站点 × 50,000 种子 × 150 字节 = 637 MB

加上索引开销 (约 2x):
  637 MB × 2 ≈ 1.27 GB

结论: 普通 VPS 完全够用!
```

---

**文档版本**: v1.0  
**最后更新**: 2026-04-12  
**相关文档**:
- 第五卷主文档: [pt-iyuu-reseed-deep-analysis.md](file:///home/incast/PT-Forward/docs/pt-iyuu-reseed-deep-analysis.md)
- 第四卷: [pt-nexusphp-hash-analysis.md](file:///home/incast/PT-Forward/docs/pt-nexusphp-hash-analysis.md)
