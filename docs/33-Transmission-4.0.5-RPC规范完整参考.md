# Transmission 4.0.5 RPC 规范完整参考 + PT 优化指南

> 来源：https://github.com/transmission/transmission/blob/4.0.5/docs/rpc-spec.md
> 适用于 Transmission 4.0.5，RPC 版本 17（rpc-version-semver: 5.3.0）
> 本文档为 PT-Forward 项目唯一的 Transmission 参考文档（合并了原 RPC 指南的独有内容）
> **版本锁定 4.0.5，但保留后续升级或降级的能力（见附录 F）**
> 整理日期：2026-04-14

---

## 目录

1. [简介](#1-简介)
2. [消息格式](#2-消息格式)
3. [种子请求](#3-种子请求)
4. [Session 请求](#4-session-请求)
5. [协议版本历史](#5-协议版本历史)
6. [废弃字段与破坏性变更](#6-废弃字段与破坏性变更)
7. [单位参考](#7-单位参考)
8. [附录 A：技术架构](#附录-a技术架构)
9. [附录 B：PT 优化指南](#附录-bpt-优化指南)
10. [附录 C：安全机制](#附录-c安全机制)
11. [附录 D：配置系统](#附录-d配置系统)
12. [附录 E：Docker 部署](#附录-e-docker-部署)
13. [附录 F：版本管理策略](#附录-f版本管理策略)

---

## 1. 简介

### 1.1 术语

使用 RFC 4627 中的 JSON 术语。RPC 请求和响应均为 JSON 格式。

### 1.2 调试工具

- `transmission-remote --debug` — 终端输出 RPC 通信内容
- `transmission-qt` + `TR_RPC_VERBOSE` 环境变量 — 终端输出 RPC 请求/响应
- 浏览器开发者工具 — Transmission Web 客户端中检查

### 1.3 第三方库封装（非官方）

| 语言 | 链接 |
|------|------|
| Go | https://github.com/hekmon/transmissionrpc |
| Python | https://github.com/Trim21/transmission-rpc |
| Rust | https://crates.io/crates/transmission-rpc |
| C# | https://www.nuget.org/packages/Transmission.API.RPC |

---

## 2. 消息格式

所有文本 **必须** 使用 UTF-8 编码。

### 2.1 请求

请求支持三个键：

1. **`method`**（string，必填）— 调用的方法名
2. **`arguments`**（object，可选）— 键值对，键由方法定义
3. **`tag`**（number，可选）— 客户端追踪号，响应必须返回相同值

```json
{
   "arguments": { "fields": ["version"] },
   "method": "session-get",
   "tag": 912313
}
```

### 2.2 响应

1. **`result`**（string，必填）— 成功时为 `"success"`，失败时为错误字符串
2. **`arguments`**（object，可选）— 由方法定义的键值对
3. **`tag`**（number，可选）— 与请求中的 tag 匹配

```json
{
   "arguments": { "version": "4.0.5 (xxxx)" },
   "result": "success",
   "tag": 912313
}
```

### 2.3 传输机制

- HTTP POST JSON 编码的请求到 RPC 服务器
- 默认 URL：`http://host:9091/transmission/rpc`
- 端口和路径可配置

#### 2.3.1 CSRF 防护

- 需要 `X-Transmission-Session-Id` 请求头
- 首次请求或 token 过期时，服务器返回 HTTP 409，响应头包含正确的 Session ID
- 客户端应更新 Session ID 并重发请求

#### 2.3.2 DNS 重绑定防护

- 主机白名单默认启用
- 检查 `Host:` HTTP 头（去掉端口后）是否匹配白名单
- `localhost`、`localhost.` 和所有 IP 地址始终隐式允许
- 配置：`rpc-host-whitelist-enabled` 和 `rpc-host-whitelist`

#### 2.3.3 认证

- 可选，使用 HTTP Basic Access Authentication
- 请求头：`Authorization: Basic <base64(username:password)>`

---

## 3. 种子请求

### 3.1 种子操作请求

| 方法名 | 功能 |
|--------|------|
| `torrent-start` | 开始种子 |
| `torrent-start-now` | 立即开始（绕过队列） |
| `torrent-stop` | 停止种子 |
| `torrent-verify` | 验证本地数据 |
| `torrent-reannounce` | 请求 tracker 获取更多 peers |

**请求参数：** `ids` — 指定种子。省略则对所有种子操作。

**`ids` 格式：**
1. 整数（种子 ID）
2. 数组（种子 ID、SHA1 hash 字符串，或混合）
3. 字符串 `"recently-active"`（最近活跃的种子）

**响应参数：** 无

### 3.2 种子修改器：`torrent-set`

**请求参数：**

| 键 | 类型 | 说明 |
|----|------|------|
| `bandwidthPriority` | number | 带宽优先级 |
| `downloadLimit` | number | 最大下载速度 (**KBps**) |
| `downloadLimited` | boolean | 是否启用 downloadLimit |
| `files-unwanted` | array | 不下载的文件索引 |
| `files-wanted` | array | 要下载的文件索引 |
| `group` | string | 带宽组名称（4.0.0 新增） |
| `honorsSessionLimits` | boolean | 是否遵守会话限速 |
| `ids` | array | 种子列表 |
| `labels` | array | 标签字符串数组（3.00 新增） |
| `location` | string | 新位置 |
| `peer-limit` | number | 最大 peer 数 |
| `priority-high` | array | 高优先级文件索引 |
| `priority-low` | array | 低优先级文件索引 |
| `priority-normal` | array | 普通优先级文件索引 |
| `queuePosition` | number | 队列位置 [0...n) |
| `seedIdleLimit` | number | 做种空闲限制（分钟） |
| `seedIdleMode` | number | 空闲限制模式 |
| `seedRatioLimit` | double | 分享率限制 |
| `seedRatioMode` | number | 分享率模式 |
| `trackerList` | string | tracker URL，每行一个，层间空行分隔（4.0.0 新增） |
| `trackerAdd` | array | **已废弃**，用 trackerList 替代 |
| `trackerRemove` | array | **已废弃**，用 trackerList 替代 |
| `trackerReplace` | array | **已废弃**，用 trackerList 替代 |
| `uploadLimit` | number | 最大上传速度 (**KBps**) |
| `uploadLimited` | boolean | 是否启用 uploadLimit |

空 `ids` = 所有种子。空文件数组 = 所有文件。

**响应参数：** 无

### 3.3 种子查询：`torrent-get`

**请求参数：**

| 键 | 类型 | 说明 |
|----|------|------|
| `ids` | array | 可选，种子列表 |
| `fields` | array | **必填**，要查询的字段列表 |
| `format` | string | 可选，`"objects"`（默认）或 `"table"`（3.00 新增） |

**响应参数：**

| 键 | 说明 |
|----|------|
| `torrents` | `objects` 格式：对象数组；`table` 格式：二维数组（首行键名，后续行值） |
| `removed` | 仅当 `ids="recently-active"` 时返回，已移除的种子 ID 数组 |

#### 3.3.1 种子字段完整列表

| 字段 | 类型 | 说明 |
|------|------|------|
| `activityDate` | number | 最后活动时间 |
| `addedDate` | number | 添加时间 |
| `availability` | array | 每块的可用 peer 数（-1=已拥有）（4.0.0 新增） |
| `bandwidthPriority` | number | 带宽优先级 |
| `comment` | string | 种子注释 |
| `corruptEver` | number | 损坏数据量 |
| `creator` | string | 创建者 |
| `dateCreated` | number | 创建时间 |
| `desiredAvailable` | number | 可用但未下载的数据量 |
| `doneDate` | number | 完成时间 |
| `downloadDir` | string | 下载目录 |
| `downloadedEver` | number | 总下载量 |
| `downloadLimit` | number | 下载限速 (KBps) |
| `downloadLimited` | boolean | 是否启用下载限速 |
| `editDate` | number | 最后编辑时间（3.00 新增） |
| `error` | number | 错误码 |
| `errorString` | string | 错误信息 |
| `eta` | number | 预估剩余时间（秒） |
| `etaIdle` | number | 空闲 ETA（2.80 新增） |
| `file-count` | number | 文件数量（4.0.0 新增） |
| `files` | array | 文件列表（见 3.3.2） |
| `fileStats` | array | 文件状态列表（见 3.3.3） |
| `group` | string | 带宽组名（4.0.0 新增） |
| `hashString` | string | SHA1 hash |
| `haveUnchecked` | number | 未验证数据量 |
| `haveValid` | number | 已验证数据量 |
| `honorsSessionLimits` | boolean | 是否遵守会话限速 |
| `id` | number | 种子 ID |
| `isFinished` | boolean | 是否已完成 |
| `isPrivate` | boolean | 是否私有种子 |
| `isStalled` | boolean | 是否停滞 |
| `labels` | array | 标签数组（3.00 新增） |
| `leftUntilDone` | number | 剩余数据量 |
| `magnetLink` | string | 磁力链接 |
| `manualAnnounceTime` | number | 下次手动宣告时间 |
| `maxConnectedPeers` | number | 最大连接 peer 数 |
| `metadataPercentComplete` | double | 元数据完成百分比 |
| `name` | string | 种子名称 |
| `peer-limit` | number | peer 限制 |
| `peers` | array | Peer 列表（见 3.3.5） |
| `peersConnected` | number | 已连接 peer 数 |
| `peersFrom` | object | Peer 来源（见 3.3.6） |
| `peersGettingFromUs` | number | 从我们下载的 peer 数 |
| `peersSendingToUs` | number | 向我们上传的 peer 数 |
| `percentComplete` | double | 完成百分比（含未选中文件）（4.0.0 新增） |
| `percentDone` | double | 完成百分比（仅选中文件） |
| `pieces` | string | base64 编码的位图 |
| `pieceCount` | number | 块数量 |
| `pieceSize` | number | 块大小 |
| `priorities` | array | 每文件优先级 |
| `primary-mime-type` | string | 主要 MIME 类型（4.0.0 新增） |
| `queuePosition` | number | 队列位置 |
| `rateDownload` | number | 下载速度 (B/s) |
| `rateUpload` | number | 上传速度 (B/s) |
| `recheckProgress` | double | 验证进度 |
| `secondsDownloading` | number | 下载耗时（秒） |
| `secondsSeeding` | number | 做种耗时（秒） |
| `seedIdleLimit` | number | 空闲限制（分钟） |
| `seedIdleMode` | number | 空闲模式 |
| `seedRatioLimit` | double | 分享率限制 |
| `seedRatioMode` | number | 分享率模式 |
| `sizeWhenDone` | number | 选中文件总大小 |
| `startDate` | number | 开始时间 |
| `status` | number | 状态（见下表） |
| `trackers` | array | Tracker 列表（见 3.3.10） |
| `trackerList` | string | tracker URL 列表字符串（4.0.0 新增） |
| `trackerStats` | array | Tracker 统计（见 3.3.11） |
| `totalSize` | number | 全部文件总大小 |
| `torrentFile` | string | .torrent 文件路径 |
| `uploadedEver` | number | 总上传量 |
| `uploadLimit` | number | 上传限速 (KBps) |
| `uploadLimited` | boolean | 是否启用上传限速 |
| `uploadRatio` | double | 分享率 |
| `wanted` | array | 每文件是否要下载（0/1） |
| `webseeds` | array | Web seed URL 数组 |
| `webseedsSendingToUs` | number | 向我们发送的 web seed 数 |

#### 3.3.2 子字段：`files`

| 字段 | 类型 | 说明 |
|------|------|------|
| `bytesCompleted` | number | 已完成字节数 |
| `length` | number | 文件大小 |
| `name` | string | 文件名（含相对路径） |

#### 3.3.3 子字段：`fileStats`

| 字段 | 类型 | 说明 |
|------|------|------|
| `bytesCompleted` | number | 已完成字节数 |
| `wanted` | number | 0 或 1（4.0.2+ 恢复为 0/1） |
| `priority` | number | 优先级 |

#### 3.3.4 子字段：`status` 枚举

| 值 | 含义 |
|----|------|
| 0 | 已停止 |
| 1 | 排队等待验证 |
| 2 | 正在验证 |
| 3 | 排队等待下载 |
| 4 | 下载中 |
| 5 | 排队等待做种 |
| 6 | 做种中 |

#### 3.3.5 子字段：`peers`

| 字段 | 类型 | 说明 |
|------|------|------|
| `address` | string | IP 地址 |
| `clientName` | string | 客户端名称 |
| `clientIsChoked` | boolean | 我们是否阻塞对方 |
| `clientIsInterested` | boolean | 我们是否对对方感兴趣 |
| `flagStr` | string | 状态标志字符串 |
| `isDownloadingFrom` | boolean | 对方是否向我们上传 |
| `isEncrypted` | boolean | 是否加密 |
| `isIncoming` | boolean | 是否入站连接 |
| `isUploadingTo` | boolean | 我们是否向对方上传 |
| `isUTP` | boolean | 是否 uTP 连接 |
| `peerIsChoked` | boolean | 对方是否阻塞我们 |
| `peerIsInterested` | boolean | 对方是否对我们感兴趣 |
| `port` | number | 端口 |
| `progress` | double | 完成进度 |
| `rateToClient` | number | 发给我们的速度 (B/s) |
| `rateToPeer` | number | 我们发送的速度 (B/s) |

#### 3.3.6 子字段：`peersFrom`

| 字段 | 说明 |
|------|------|
| `fromCache` | 来自缓存 |
| `fromDht` | 来自 DHT |
| `fromIncoming` | 来自入站 |
| `fromLpd` | 来自本地发现 |
| `fromLtep` | 来自 LTEP |
| `fromPex` | 来自 PEX |
| `fromTracker` | 来自 Tracker |

#### 3.3.7 子字段：`trackers`

| 字段 | 类型 | 说明 |
|------|------|------|
| `announce` | string | 宣告 URL |
| `id` | number | Tracker ID |
| `scrape` | string | 刮擦 URL |
| `sitename` | string | 站点名（4.0.0 新增） |
| `tier` | number | 层级 |

#### 3.3.8 子字段：`trackerStats`

| 字段 | 类型 | 说明 |
|------|------|------|
| `announceState` | number | 宣告状态 |
| `announce` | string | 宣告 URL |
| `downloadCount` | number | 下载数 |
| `hasAnnounced` | boolean | 是否已宣告 |
| `hasScraped` | boolean | 是否已刮擦 |
| `host` | string | 主机名 |
| `id` | number | Tracker ID |
| `isBackup` | boolean | 是否备用 |
| `lastAnnouncePeerCount` | number | 上次宣告的 peer 数 |
| `lastAnnounceResult` | string | 上次宣告结果 |
| `lastAnnounceStartTime` | number | 上次宣告开始时间 |
| `lastAnnounceSucceeded` | boolean | 上次宣告是否成功 |
| `lastAnnounceTime` | number | 上次宣告时间 |
| `lastAnnounceTimedOut` | boolean | 上次宣告是否超时 |
| `lastScrapeResult` | string | 上次刮擦结果 |
| `lastScrapeStartTime` | number | 上次刮擦开始时间 |
| `lastScrapeSucceeded` | boolean | 上次刮擦是否成功 |
| `lastScrapeTime` | number | 上次刮擦时间 |
| `lastScrapeTimedOut` | boolean | 上次刮擦是否超时 |
| `leecherCount` | number | 下载者数 |
| `nextAnnounceTime` | number | 下次宣告时间 |
| `nextScrapeTime` | number | 下次刮擦时间 |
| `scrapeState` | number | 刮擦状态 |
| `scrape` | string | 刮擦 URL |
| `seederCount` | number | 做种者数 |
| `sitename` | string | 站点名（4.0.0 新增） |
| `tier` | number | 层级 |

### 3.4 添加种子：`torrent-add`

**请求参数：**

| 键 | 类型 | 说明 |
|----|------|------|
| `cookies` | string | Cookie 字符串（`name1=val1; name2=val2`） |
| `download-dir` | string | 下载路径 |
| `filename` | string | 文件名或 URL（与 metainfo 二选一） |
| `labels` | array | 标签数组（4.0.0 新增） |
| `metainfo` | string | base64 编码的 .torrent 内容（与 filename 二选一） |
| `paused` | boolean | 添加后暂停 |
| `peer-limit` | number | 最大 peer 数 |
| `bandwidthPriority` | number | 带宽优先级 |
| `files-wanted` | array | 要下载的文件索引 |
| `files-unwanted` | array | 不下载的文件索引 |
| `priority-high` | array | 高优先级文件索引 |
| `priority-low` | array | 低优先级文件索引 |
| `priority-normal` | array | 普通优先级文件索引 |

**必须包含 `filename` 或 `metainfo` 之一。**

**响应参数：**
- 成功：`torrent-added` 对象（含 `id`、`name`、`hashString`）
- 重复：`torrent-duplicate` 对象（`result` 仍为 `"success"`）

### 3.5 移除种子：`torrent-remove`

| 键 | 类型 | 说明 |
|----|------|------|
| `ids` | array | 种子列表 |
| `delete-local-data` | boolean | 是否删除本地数据（默认 false） |

**响应参数：** 无

### 3.6 移动种子：`torrent-set-location`

| 键 | 类型 | 说明 |
|----|------|------|
| `ids` | array | 种子列表 |
| `location` | string | 新位置 |
| `move` | boolean | true=从旧位置移动，false=在新位置查找文件（默认 false） |

**响应参数：** 无

### 3.7 重命名路径：`torrent-rename-path`

| 键 | 类型 | 说明 |
|----|------|------|
| `ids` | array | 仅限 1 个种子 |
| `path` | string | 要重命名的文件/文件夹路径 |
| `name` | string | 新名称 |

**响应参数：** `path`、`name`、`id`

---

## 4. Session 请求

### 4.1 Session 参数

可用于 `session-get` 和 `session-set`。

| 键 | 类型 | 说明 |
|----|------|------|
| `alt-speed-down` | number | 备用下载限速 (KBps) |
| `alt-speed-enabled` | boolean | 是否启用备用速度 |
| `alt-speed-time-begin` | number | 备用速度开始时间（分钟，午夜后） |
| `alt-speed-time-day` | number | 备用速度生效日 |
| `alt-speed-time-enabled` | boolean | 是否启用定时备用速度 |
| `alt-speed-time-end` | number | 备用速度结束时间 |
| `alt-speed-up` | number | 备用上传限速 (KBps) |
| `blocklist-enabled` | boolean | 是否启用黑名单 |
| `blocklist-size` | number | 黑名单规则数（只读） |
| `blocklist-url` | string | 黑名单 URL |
| `cache-size-mb` | number | 磁盘缓存最大大小 (MB) |
| `config-dir` | string | 配置目录（只读） |
| `default-trackers` | string | 默认 tracker 列表（4.0.0 新增） |
| `dht-enabled` | boolean | 是否启用 DHT |
| `download-dir` | string | 默认下载路径 |
| `download-dir-free-space` | number | **已废弃**，用 `free-space` 方法替代 |
| `download-queue-enabled` | boolean | 是否限制同时下载数 |
| `download-queue-size` | number | 最大同时下载数 |
| `encryption` | string | 加密模式：`required`/`preferred`/`tolerated` |
| `idle-seeding-limit-enabled` | boolean | 是否启用做种空闲限制 |
| `idle-seeding-limit` | number | 空闲做种限制（分钟） |
| `incomplete-dir-enabled` | boolean | 是否使用未完成目录 |
| `incomplete-dir` | string | 未完成文件目录 |
| `lpd-enabled` | boolean | 是否启用本地 Peer 发现 |
| `peer-limit-global` | number | 全局最大 peer 数 |
| `peer-limit-per-torrent` | number | 每种子最大 peer 数 |
| `peer-port-random-on-start` | boolean | 启动时随机端口 |
| `peer-port` | number | 监听端口 |
| `pex-enabled` | boolean | 是否启用 PEX |
| `port-forwarding-enabled` | boolean | 是否启用 UPnP/NAT-PMP 端口转发 |
| `queue-stalled-enabled` | boolean | 是否将空闲种子视为停滞 |
| `queue-stalled-minutes` | number | 停滞判定分钟数 |
| `rename-partial-files` | boolean | 未完成文件是否加 `.part` 后缀 |
| `rpc-version-minimum` | number | 最低兼容 RPC 版本（只读） |
| `rpc-version-semver` | string | RPC 版本 semver 字符串（只读，4.0.0 新增） |
| `rpc-version` | number | RPC 版本号（只读） |
| `script-torrent-added-enabled` | boolean | 是否启用添加脚本（4.0.0 新增） |
| `script-torrent-added-filename` | string | 添加脚本路径（4.0.0 新增） |
| `script-torrent-done-enabled` | boolean | 是否启用完成脚本 |
| `script-torrent-done-filename` | string | 完成脚本路径 |
| `script-torrent-done-seeding-enabled` | boolean | 是否启用做种完成脚本（4.0.0 新增） |
| `script-torrent-done-seeding-filename` | string | 做种完成脚本路径（4.0.0 新增） |
| `seed-queue-enabled` | boolean | 是否限制同时做种数 |
| `seed-queue-size` | number | 最大同时做种数 |
| `seedRatioLimit` | double | 默认分享率限制 |
| `seedRatioLimited` | boolean | 是否启用默认分享率限制 |
| `speed-limit-down-enabled` | boolean | 是否启用全局下载限速 |
| `speed-limit-down` | number | 全局下载限速 (KBps) |
| `speed-limit-up-enabled` | boolean | 是否启用全局上传限速 |
| `speed-limit-up` | number | 全局上传限速 (KBps) |
| `start-added-torrents` | boolean | 添加后自动开始 |
| `trash-original-torrent-files` | boolean | 添加后删除原始 .torrent 文件 |
| `units` | object | 单位配置（只读，见下） |
| `utp-enabled` | boolean | 是否启用 uTP |
| `version` | string | 版本字符串（只读） |

#### 子字段：`units`

| 键 | 类型 | 说明 |
|----|------|------|
| `speed-units` | array | 4 个字符串：KB/s, MB/s, GB/s, TB/s |
| `speed-bytes` | number | 每 KB 的字节数（1000 或 1024） |
| `size-units` | array | 同上 |
| `size-bytes` | number | 同上 |
| `memory-units` | array | 同上 |
| `memory-bytes` | number | 同上 |

### 4.1.1 修改器：`session-set`

设置 4.1 中的可变参数。以下字段为**只读**，不可设置：

`blocklist-size`、`config-dir`、`rpc-version-minimum`、`rpc-version-semver`、`rpc-version`、`session-id`、`units`、`version`

**响应参数：** 无

### 4.1.2 查询器：`session-get`

**请求参数：** 可选 `fields` 数组

**响应参数：** 匹配 `fields` 的键值对，或全部支持字段

### 4.2 Session 统计：`session-stats`

**请求参数：** 无

**响应参数：**

| 键 | 类型 | 说明 |
|----|------|------|
| `activeTorrentCount` | number | 活跃种子数 |
| `downloadSpeed` | number | 当前下载速度 |
| `pausedTorrentCount` | number | 暂停种子数 |
| `torrentCount` | number | 总种子数 |
| `uploadSpeed` | number | 当前上传速度 |
| `cumulative-stats` | object | 累计统计 |
| `current-stats` | object | 当前会话统计 |

**统计对象字段：**

| 字段 | 说明 |
|------|------|
| `uploadedBytes` | 上传字节数 |
| `downloadedBytes` | 下载字节数 |
| `filesAdded` | 添加文件数 |
| `sessionCount` | 会话数 |
| `secondsActive` | 活跃秒数 |

### 4.3 黑名单更新：`blocklist-update`

**请求参数：** 无

**响应参数：** `blocklist-size`（number）

### 4.4 端口检测：`port-test`

测试入站 peer 端口是否可从外部访问。

**请求参数：** 无

**响应参数：** `port-is-open`（boolean）

### 4.5 关闭 Session：`session-close`

通知 Transmission 会话关闭。

**请求参数：** 无

**响应参数：** 无

### 4.6 队列移动

| 方法名 | 功能 |
|--------|------|
| `queue-move-top` | 移到队列顶部 |
| `queue-move-up` | 上移一位 |
| `queue-move-down` | 下移一位 |
| `queue-move-bottom` | 移到队列底部 |

**请求参数：** `ids`（array）

**响应参数：** 无

### 4.7 磁盘空间：`free-space`

**请求参数：**

| 键 | 类型 | 说明 |
|----|------|------|
| `path` | string | 要查询的目录 |

**响应参数：**

| 键 | 类型 | 说明 |
|----|------|------|
| `path` | string | 同请求参数 |
| `size-bytes` | number | 可用空间（字节） |
| `total_size` | number | 总容量（字节，4.0.0 新增） |

### 4.8 带宽组

#### 4.8.1 设置带宽组：`group-set`（4.0.0 新增）

| 键 | 类型 | 说明 |
|----|------|------|
| `name` | string | 带宽组名称 |
| `honorsSessionLimits` | boolean | 是否遵守会话限速 |
| `speed-limit-down-enabled` | boolean | 是否启用下载限速 |
| `speed-limit-down` | number | 下载限速 (KBps) |
| `speed-limit-up-enabled` | boolean | 是否启用上传限速 |
| `speed-limit-up` | number | 上传限速 (KBps) |

**响应参数：** 无

#### 4.8.2 查询带宽组：`group-get`（4.0.0 新增）

**请求参数：** 可选 `group`（字符串或字符串数组）。省略则返回全部。

**响应参数：** `group`（数组，每个元素含 name、honorsSessionLimits、speed-limit-down-enabled、speed-limit-down、speed-limit-up-enabled、speed-limit-up）

---

## 5. 协议版本历史

RPC 版本通过 `session-get` 的 `rpc-version` 和 `rpc-version-semver` 字段查询。

### 4.0.0 新增（rpc-version 17，semver 5.3.0）

- 移除未公开的 `/upload` 端点
- **废弃** `download-dir-free-space`，用 `free-space` 替代
- **废弃** `trackerAdd`/`trackerRemove`/`trackerReplace`，用 `trackerList` 替代
- **新增** `group-set`、`group-get` 方法
- **新增** 字段：`default-trackers`、`rpc-version-semver`、`script-torrent-added-*`、`script-torrent-done-seeding-*`、`torrent-add` 的 `labels`、`torrent-get` 的 `availability`/`file-count`/`group`/`percentComplete`/`primary-mime-type`/`trackerList`/`tracker.sitename`/`trackerStats.sitename`
- **破坏性变更**：`wanted` 字段在 4.0.0-4.0.1 改为布尔值，4.0.2 恢复为 0/1

### 3.00 新增（rpc-version 16，semver 5.2.0）

- `session-get` 新增 `fields` 请求参数、`session-id` 字段
- `torrent-get`/`torrent-set` 新增 `labels`
- `torrent-get` 新增 `editDate`、`format` 请求参数

### 2.80 新增（rpc-version 15，semver 5.1.0）

- 新增 `torrent-rename-path`、`free-space` 方法
- `torrent-get` 新增 `etaIdle`
- `torrent-add` 新增 `torrent-duplicate` 返回

### 2.40 新增（rpc-version 14，semver 5.0.0）

- **破坏性变更**：`status` 字段值发生变化
- 新增 `queue-move-*` 四个方法、`torrent-start-now`
- 新增队列相关 session 参数

### 2.20 新增（rpc-version 12，semver 3.6.0）

- 新增 `session-close` 方法
- `session-get` 新增 `download-dir-free-space`

### 1.60 新增（rpc-version 5，semver 2.0.0）

- **多项重命名**（破坏性）
- 新增 `blocklist-update`、`port-test`、`torrent-reannounce`
- 新增 `torrent-get` 的 `recently-active` ids 选项

---

## 6. 废弃字段与破坏性变更

### 废弃字段

| 字段 | 所在方法 | 废弃版本 | 替代方案 |
|------|---------|---------|---------|
| `download-dir-free-space` | `session-get` | 4.0.0 | 使用 `free-space` 方法 |
| `trackerAdd` | `torrent-set` | 4.0.0 | 使用 `trackerList` |
| `trackerRemove` | `torrent-set` | 4.0.0 | 使用 `trackerList` |
| `trackerReplace` | `torrent-set` | 4.0.0 | 使用 `trackerList` |

### 重大破坏性变更历史

| 版本 | 变更 |
|------|------|
| 1.60 (rpc 5) | 多项字段重命名 |
| 1.80 (rpc 7) | 移除 13 个 tracker 相关字段，用 `trackerStats` 替代 |
| 2.40 (rpc 14) | `status` 字段值变化 |
| 4.0.0 (rpc 17) | 移除 `/upload` 端点 |

---

## 7. 单位参考

### 关键：Transmission 限速单位与 qBittorrent 不同！

| 上下文 | 单位 |
|--------|------|
| `torrent-set` 的 `downloadLimit`/`uploadLimit` | **KBps** (千字节/秒) |
| `session-set` 的 `speed-limit-down`/`speed-limit-up` | **KBps** |
| `session-set` 的 `alt-speed-down`/`alt-speed-up` | **KBps** |
| `torrent-get` 的 `rateDownload`/`rateUpload` | **B/s** (字节/秒) |
| `session-stats` 的 `downloadSpeed`/`uploadSpeed` | **B/s** |
| 所有数据量字段 | **bytes** |
| 缓存大小 | **MB** |
| 做种空闲限制 | **分钟** |
| 时间相关字段 | **Unix 时间戳**（秒） |

### 与 qBittorrent 的单位差异

| 操作 | qBittorrent | Transmission |
|------|-------------|--------------|
| 设置单种子限速 | bytes/s | **KBps** |
| 设置全局限速 | bytes/s | **KBps** |
| 查询当前速度 | bytes/s | **B/s** |

**PT-Forward 统一策略：** 内部统一使用 **bytes/s**，Transmission 实现中 ×1024 转换。

---

## 附录 A：技术架构

### A.1 基本信息

| 属性 | 值 |
|------|-----|
| **项目名称** | Transmission |
| **技术栈** | C++17 / CMake / libevent |
| **许可证** | GPL-2.0+ / MIT（部分） |
| **代码规模** | ~80,000 行 C++ 代码 |
| **主要用途** | BT 下载客户端（轻量备选） |
| **默认 RPC 端口** | 9091 |
| **API 风格** | JSON-RPC（POST） |

### A.2 架构图

```
┌──────────────────────────────────────────────────┐
│              Transmission 架构                    │
├──────────────────────────────────────────────────┤
│                                                   │
│  ┌───────────┐  ┌───────────┐  ┌──────────────┐  │
│  │  WebUI    │  │   CLI     │  │  Daemon      │  │
│  │ (HTTP)    │  │ (remote)  │  │ (后台服务)    │  │
│  └─────┬─────┘  └─────┬─────┘  └──────┬───────┘  │
│        │              │               │           │
│        └──────────────┼───────────────┘           │
│                       ▼                           │
│  ┌────────────────────────────────────────────┐   │
│  │          RPC Server (JSON-RPC)             │   │
│  └────────────────────┬───────────────────────┘   │
│                       ▼                           │
│  ┌────────────────────────────────────────────┐   │
│  │          libtransmission (核心库)           │   │
│  │  Session | Torrent | PeerMgr | Announcer   │   │
│  └────────────────────┬───────────────────────┘   │
│                       ▼                           │
│  ┌────────────────────────────────────────────┐   │
│  │          libevent (事件循环)                │   │
│  └────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────┘
```

### A.3 设计哲学

| 原则 | 说明 |
|------|------|
| 轻量 | 空闲 <50MB 内存，适合 NAS/嵌入式 |
| 事件驱动 | 基于 libevent 的高性能事件循环 |
| 模块化 | libtransmission 可独立使用 |
| 跨平台 | Linux/macOS/Windows/BSD |
| JSON 配置 | settings.json 简单配置 |

---

## 附录 B：PT 优化指南

### B.1 资源占用对比

| 场景 | Transmission | qBittorrent |
|------|-------------|-------------|
| 空闲 | ~30MB | ~120MB |
| 10 个种子 | ~50MB | ~200MB |
| 100 个种子 | ~80MB | ~350MB |
| 启动时间 | <1s | 2-3s |

### B.2 三种场景配置

#### NAS/嵌入式（<512MB 内存）

```json
{
    "cache-size-mb": 2,
    "peer-limit-global": 100,
    "peer-limit-per-torrent": 20,
    "download-queue-size": 2,
    "seed-queue-size": 5,
    "lpd-enabled": false,
    "pex-enabled": false,
    "dht-enabled": false
}
```

#### 家用服务器（1-4GB 内存）

```json
{
    "cache-size-mb": 8,
    "peer-limit-global": 300,
    "peer-limit-per-torrent": 50,
    "download-queue-size": 5,
    "seed-queue-size": 10
}
```

#### 高性能服务器（>8GB 内存）

```json
{
    "cache-size-mb": 32,
    "peer-limit-global": 1000,
    "peer-limit-per-torrent": 200,
    "upload-slots-per-torrent": 20,
    "seed-queue-size": 20
}
```

### B.3 PT 推荐会话配置

```json
{
    "seedRatioLimit": 0,
    "seedRatioLimited": false,
    "idle-seeding-limit-enabled": false,
    "idle-seeding-limit": 0,
    "cache-size-mb": 16,
    "upload-slots-per-torrent": 10,
    "seed-queue-size": 20,
    "encryption": "preferred",
    "peer-port": 51413,
    "port-forwarding-enabled": true,
    "start-added-torrents": true,
    "script-torrent-done-enabled": true,
    "script-torrent-done-filename": "/path/to/script.sh"
}
```

### B.4 预分配模式

| 模式 | 速度 | 碎片 | 适用场景 |
|------|------|------|---------|
| `none` | 最快 | 多 | 测试 |
| `sparse` | 快 | 少 | 默认推荐 |
| `full` | 慢 | 无 | 机械硬盘 |

---

## 附录 C：安全机制

### C.1 四层防御体系

| 层级 | 机制 | 实现 |
|------|------|------|
| L4 | 认证 | HTTP Basic Auth + salted hash |
| L3 | Session ID | CSRF 防护（48 字符，1 小时过期） |
| L2 | 防暴力破解 | Per-IP 失败追踪 + 阈值封禁 |
| L1 | 网络 | IP/主机名白名单 + Bind Address |

### C.2 Session ID 机制

- 48 字符随机 ID
- 1 小时过期（`SessionIdDurationSec`）
- 双 ID 存储（current + previous，平滑过渡）
- HTTP 409 响应触发更新

### C.3 密码安全

Transmission 使用 salted hash，但安全性低于 qBittorrent 的 PBKDF2。

### C.4 生产环境加固

```bash
# 1. 启用认证
transmission-remote --auth username:password

# 2. 设置 IP 白名单
# settings.json: "rpc-whitelist": "127.0.0.1,192.168.*"

# 3. 更改默认端口
# settings.json: "rpc-port": 9091

# 4. 使用反向代理 + HTTPS（Nginx）
```

---

## 附录 D：配置系统

### D.1 配置目录

```
~/.config/transmission-daemon/
├── settings.json      # 主配置文件
├── torrents/          # 种子文件
├── resume/            # 恢复数据
├── blocklists/        # 黑名单
├── certs/             # TLS 证书
└── logs/              # 日志
```

### D.2 动态更新配置的三种方式

1. **RPC**：`transmission-remote --session-set`
2. **信号**：`kill -HUP <pid>` 重载 settings.json
3. **WebUI**：通过 Web 界面修改

---

## 附录 E：Docker 部署

```yaml
services:
  transmission:
    image: lscr.io/linuxserver/transmission:version-4.0.5
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - USER=admin
      - PASS=adminadmin
    volumes:
      - ./config:/config
      - /downloads:/downloads
    ports:
      - 9091:9091
      - 51413:51413
      - 51413:51413/udp
    restart: unless-stopped
```

---

## 附录 F：版本管理策略

### 当前锁定版本

- **Transmission**: 4.0.5
- **RPC 版本**: 17
- **RPC Semver**: 5.3.0

### 版本检测

```go
type TransmissionVersion struct {
    Version        string // e.g. "4.0.5"
    RPCVersion     int    // e.g. 17
    RPCSemver      string // e.g. "5.3.0"
    RPCMinVersion  int    // e.g. 1
}
```

### 版本兼容性检查

```go
const (
    MinSupportedRPCVersion = 16 // 3.00+
    TargetRPCVersion       = 17 // 4.0.x
)

func checkCompatibility(rpcVersion int) error {
    if rpcVersion < MinSupportedRPCVersion {
        return fmt.Errorf("unsupported RPC version %d, minimum is %d", rpcVersion, MinSupportedRPCVersion)
    }
    if rpcVersion > TargetRPCVersion {
        log.Warn("RPC version %d is newer than target %d, some features may not be tested", rpcVersion, TargetRPCVersion)
    }
    return nil
}
```

### 升级/降级步骤

1. 修改 Docker image tag
2. 检查 RPC 版本是否在兼容范围内
3. 验证废弃字段是否影响现有代码
4. 测试所有核心操作

---

*整理日期：2026-04-14*
*来源：https://github.com/transmission/transmission/blob/4.0.5/docs/rpc-spec.md*
*适用于 Transmission 4.0.5，RPC 版本 17（semver 5.3.0）*
*附录内容合并自原 Transmission RPC 完整指南*
