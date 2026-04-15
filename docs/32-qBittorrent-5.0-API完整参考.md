# qBittorrent API 完整参考 + PT 优化指南

> 5.0 API 来源：https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-5.0)
> 4.x API 来源：https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)
> 适用于 qBittorrent v4.1.0 - v5.x，WebAPI 版本 v2.0 - v2.11.3
> 本文档为 PT-Forward 项目唯一的 qBittorrent 参考文档
> 整理日期：2026-04-14

---

## 目录

1. [版本变更记录](#版本变更记录)
2. [通用信息](#通用信息)
3. [认证](#认证)
4. [应用程序](#应用程序)
5. [日志](#日志)
6. [数据同步](#数据同步)
7. [传输信息](#传输信息)
8. [种子管理](#种子管理)
9. [RSS（实验性）](#rss实验性)
10. [搜索](#搜索)
11. [WebAPI 版本规则](#webapi-版本规则)
12. [单位参考](#单位参考)
13. [与 4.x 版本的差异（兼容性指南）](#与-4x-版本的差异)
14. [附录 A：技术架构](#附录-a技术架构)
15. [附录 B：PT 优化指南](#附录-bpt-优化指南)
16. [附录 C：安全机制](#附录-c安全机制)
17. [附录 D：与 Transmission 对比](#附录-d与-transmission-对比)
18. [附录 E：Docker 部署](#附录-e-docker-部署)

---

## 版本变更记录

### API v2.9.3

- `/torrents/info` 响应新增 `reannounce` 字段（距下次 tracker 重新宣告的秒数）

### API v2.11.3

- **新增** Cookie 管理 API：`GET /app/cookies` 和 `POST /app/setCookies`
- **删除** `/torrents/add` 请求中的 `cookie` 字段

---

## 通用信息

- 所有 API 格式：`/api/v2/{APIName}/{methodName}`
- 仅允许 `GET` 或 `POST` 方法
- 变更状态用 `POST`，查询用 `GET`
- qBittorrent v4.4.4+ 使用错误方法会返回 `405 Method Not Allowed`
- 所有 API 需要认证（除 `/auth/login`）

---

## 认证

所有认证 API 在 `auth` 下：`/api/v2/auth/{methodName}`

qBittorrent 使用基于 Cookie 的认证。

### 登录

| 项目 | 值 |
|------|---|
| 名称 | `login` |
| 方法 | `POST` |

**参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `username` | string | WebUI 用户名 |
| `password` | string | WebUI 密码 |

**返回：**

| HTTP 状态码 | 场景 |
|-------------|------|
| 403 | IP 因多次登录失败被禁止 |
| 200 | 所有其他场景 |

成功时响应包含 `SID` Cookie。后续请求必须携带此 Cookie。

**注意：** 必须设置 `Referer` 或 `Origin` 头为 HTTP 查询 `Host` 头中使用的相同域名和端口。

**示例：**

```bash
curl -i --header 'Referer: http://localhost:8080' \
  --data 'username=admin&password=adminadmin' \
  http://localhost:8080/api/v2/auth/login
# 响应: Set-Cookie: SID=hBc7TxF76ERhvIw0jQQ4LZ7Z1jQUV0tQ; path=/

curl http://localhost:8080/api/v2/torrents/info \
  --cookie "SID=hBc7TxF76ERhvIw0jQQ4LZ7Z1jQUV0tQ"
```

### 登出

| 项目 | 值 |
|------|---|
| 名称 | `logout` |
| 方法 | `POST` |
| 参数 | 无 |
| 返回 | 200 |

---

## 应用程序

所有应用程序 API 在 `app` 下：`/api/v2/app/{methodName}`

### 获取应用版本

| 项目 | 值 |
|------|---|
| 名称 | `version` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: 字符串，如 `v5.0.0` |

### 获取 API 版本

| 项目 | 值 |
|------|---|
| 名称 | `webapiVersion` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: 字符串，如 `2.11.3` |

### 获取构建信息

| 项目 | 值 |
|------|---|
| 名称 | `buildInfo` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON 对象 |

**响应字段：**

| 属性 | 类型 | 说明 |
|------|------|------|
| `qt` | string | Qt 版本 |
| `libtorrent` | string | libtorrent 版本 |
| `boost` | string | Boost 版本 |
| `openssl` | string | OpenSSL 版本 |
| `bitness` | int | 应用位数（如 64） |

### 关闭应用

| 项目 | 值 |
|------|---|
| 名称 | `shutdown` |
| 方法 | `POST` |
| 参数 | 无 |
| 返回 | 200 |

### 获取应用偏好设置

| 项目 | 值 |
|------|---|
| 名称 | `preferences` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON 对象 |

**关键字段（节选，完整列表见官方文档）：**

| 属性 | 类型 | 说明 |
|------|------|------|
| `save_path` | string | 默认保存路径 |
| `temp_path_enabled` | bool | 启用未完成种子临时目录 |
| `temp_path` | string | 未完成种子临时路径 |
| `start_paused_enabled` | bool | 添加种子时暂停 |
| `auto_tmm_enabled` | bool | 默认启用自动种子管理 |
| `dl_limit` | integer | 全局下载限速 (**KiB/s**)，-1=不限 |
| `up_limit` | integer | 全局上传限速 (**KiB/s**)，-1=不限 |
| `alt_dl_limit` | integer | 备用下载限速 (**KiB/s**) |
| `alt_up_limit` | integer | 备用上传限速 (**KiB/s**) |
| `listen_port` | integer | 监听端口 |
| `max_active_downloads` | integer | 最大活跃下载数 |
| `max_active_torrents` | integer | 最大活跃种子数 |
| `max_active_uploads` | integer | 最大活跃上传数 |
| `max_connec` | integer | 全局最大连接数 |
| `max_connec_per_torrent` | integer | 每种子最大连接数 |
| `max_ratio_enabled` | bool | 启用分享率限制 |
| `max_ratio` | float | 全局分享率限制 |
| `max_ratio_act` | integer | 达到限制后动作：0=暂停, 1=删除 |
| `max_seeding_time_enabled` | bool | 启用最大做种时间 |
| `max_seeding_time` | integer | 最大做种时间（分钟） |
| `queueing_enabled` | bool | 启用排队 |
| `dht` | bool | 启用 DHT |
| `pex` | bool | 启用 PeX |
| `lsd` | bool | 启用 LSD |
| `encryption` | integer | 加密：0=优先, 1=强制开, 2=强制关 |
| `proxy_type` | integer | 代理类型：-1=禁用, 1=HTTP无认证, 2=SOCKS5无认证, 3=HTTP有认证, 4=SOCKS5有认证, 5=SOCKS4 |
| `proxy_ip` | string | 代理 IP/域名 |
| `proxy_port` | integer | 代理端口 |
| `proxy_auth_enabled` | bool | 代理需要认证 |
| `proxy_username` | string | 代理用户名 |
| `proxy_password` | string | 代理密码 |
| `proxy_torrents_only` | bool | 仅代理种子流量 |
| `export_dir` | string | 复制 .torrent 文件到目录 |
| `export_dir_fin` | string | 复制已完成 .torrent 文件到目录 |
| `web_ui_port` | integer | WebUI 端口 |
| `web_ui_username` | string | WebUI 用户名 |
| `web_ui_session_timeout` | integer | WebUI 会话超时（秒） |
| `rss_processing_enabled` | bool | 启用 RSS 处理 |
| `rss_auto_downloading_enabled` | bool | 启用 RSS 自动下载 |
| `rss_refresh_interval` | integer | RSS 刷新间隔（秒） |
| `rss_max_articles_per_feed` | integer | 每个 Feed 最大文章数 |

### 设置应用偏好

| 项目 | 值 |
|------|---|
| 名称 | `setPreferences` |
| 方法 | `POST` |
| 参数 | JSON 对象（键值对，只需传要修改的字段） |
| 返回 | 200 |

```bash
# 示例
curl -X POST http://localhost:8080/api/v2/app/setPreferences \
  --cookie "SID=xxx" \
  --data-urlencode 'json={"save_path":"/new/path","dl_limit":1024}'
```

### 获取默认保存路径

| 项目 | 值 |
|------|---|
| 名称 | `defaultSavePath` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: 字符串路径 |

### 获取 Cookies

> API v2.11.3 新增

| 项目 | 值 |
|------|---|
| 名称 | `cookies` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON 数组 |

**每个 Cookie 对象：**

| 属性 | 类型 | 说明 |
|------|------|------|
| `name` | string | Cookie 名称 |
| `domain` | string | Cookie 域名 |
| `path` | string | Cookie 路径 |
| `value` | string | Cookie 值 |
| `expirationDate` | integer | 过期时间（Unix 时间戳） |

**响应示例：**

```json
[
    {
        "name": "session",
        "domain": "example.com",
        "path": "/",
        "value": "abc123",
        "expirationDate": 1507969127
    }
]
```

### 设置 Cookies

> API v2.11.3 新增

| 项目 | 值 |
|------|---|
| 名称 | `setCookies` |
| 方法 | `POST` |
| 参数 | JSON 数组（与获取 Cookies 格式相同） |
| 返回 | 200=保存成功, 400=无效 JSON |

这些 Cookie 在通过 URL 下载 .torrent 文件时发送，替代了旧版 `/torrents/add` 中已删除的 `cookie` 参数。

---

## 日志

所有日志 API 在 `log` 下：`/api/v2/log/{methodName}`

### 获取主日志

| 项目 | 值 |
|------|---|
| 名称 | `main` |
| 方法 | `GET` |

**参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `normal` | bool | true | 包含普通消息 |
| `info` | bool | true | 包含信息消息 |
| `warning` | bool | true | 包含警告消息 |
| `critical` | bool | true | 包含严重消息 |
| `last_known_id` | int | -1 | 最后已知日志 ID（仅返回更新的） |

**返回：** 200: JSON 数组

```json
[
    {"id": 1, "message": "...", "timestamp": 1507969127, "type": 1},
    ...
]
```

`type` 值：1=普通, 2=信息, 4=警告, 8=严重

### 获取 Peer 日志

| 项目 | 值 |
|------|---|
| 名称 | `peers` |
| 方法 | `GET` |

**参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `last_known_id` | int | -1 | 最后已知日志 ID |

**返回：** 200: JSON 数组

```json
[
    {"id": 1, "ip": "1.2.3.4", "timestamp": 1507969127, "blocked": true, "reason": "..."},
    ...
]
```

---

## 数据同步

所有同步 API 在 `sync` 下：`/api/v2/sync/{methodName}`

### 获取主数据

| 项目 | 值 |
|------|---|
| 名称 | `maindata` |
| 方法 | `GET` |

**参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `rid` | int | 0 | 响应 ID（上次请求返回的 rid，用于增量同步） |

**返回：** 200: JSON 对象

| 属性 | 类型 | 说明 |
|------|------|------|
| `rid` | integer | 响应 ID（下次请求传此值） |
| `full_update` | bool | 是否为全量更新 |
| `torrents` | object | 键: info_hash, 值: 变更字段（增量，仅返回变化的字段） |
| `torrents_removed` | array | 已移除的种子 hash 列表 |
| `categories` | object | 新增/变更的分类信息 |
| `categories_removed` | array | 已移除的分类列表 |
| `tags` | array | 新增的标签列表 |
| `tags_removed` | array | 已移除的标签列表 |
| `server_state` | object | 全局传输状态 |

`server_state` 包含字段：

| 属性 | 类型 | 说明 |
|------|------|------|
| `dl_info_speed` | integer | 全局下载速度 (bytes/s) |
| `dl_info_data` | integer | 累计下载量 (bytes) |
| `up_info_speed` | integer | 全局上传速度 (bytes/s) |
| `up_info_data` | integer | 累计上传量 (bytes) |
| `dl_rate_limit` | integer | 下载限速 (bytes/s) |
| `up_rate_limit` | integer | 上传限速 (bytes/s) |
| `dht_nodes` | integer | DHT 节点数 |
| `connection_status` | string | 连接状态：`connected`/`firewalled`/`disconnected` |
| `queueing` | bool | 排队是否启用 |
| `use_alt_speed_limits` | bool | 备用速度限制是否启用 |
| `refresh_interval` | integer | 刷新间隔（毫秒） |

`torrents` 中的每个值包含的增量字段与 `/torrents/info` 响应字段相同（仅返回变化的字段）。

### 获取种子 Peer 数据

| 项目 | 值 |
|------|---|
| 名称 | `torrentPeers` |
| 方法 | `GET` |

**参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `rid` | int | 响应 ID（默认 0） |

**返回：** 200: JSON（Peer 列表增量数据）, 404: 种子不存在

---

## 传输信息

所有传输 API 在 `transfer` 下：`/api/v2/transfer/{methodName}`

### 获取全局传输信息

| 项目 | 值 |
|------|---|
| 名称 | `info` |
| 方法 | `GET` |
| 参数 | 无 |

**返回：** 200: JSON 对象

| 属性 | 类型 | 说明 |
|------|------|------|
| `dl_info_speed` | integer | 全局下载速度 (bytes/s) |
| `dl_info_data` | integer | 累计下载量 (bytes) |
| `up_info_speed` | integer | 全局上传速度 (bytes/s) |
| `up_info_data` | integer | 累计上传量 (bytes) |
| `dl_rate_limit` | integer | 下载限速 (bytes/s) |
| `up_rate_limit` | integer | 上传限速 (bytes/s) |
| `dht_nodes` | integer | DHT 节点数 |
| `connection_status` | string | 连接状态 |

### 获取备用速度限制状态

| 项目 | 值 |
|------|---|
| 名称 | `speedLimitsMode` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: `1`=备用速度启用, `0`=未启用 |

### 切换备用速度限制

| 项目 | 值 |
|------|---|
| 名称 | `toggleSpeedLimitsMode` |
| 方法 | `POST` |
| 参数 | 无 |
| 返回 | 200 |

### 获取全局下载限速

| 项目 | 值 |
|------|---|
| 名称 | `downloadLimit` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: 整数 (bytes/s)，0=不限 |

### 设置全局下载限速

| 项目 | 值 |
|------|---|
| 名称 | `setDownloadLimit` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | integer | 下载限速 (bytes/s) |

**返回：** 200

### 获取全局上传限速

| 项目 | 值 |
|------|---|
| 名称 | `uploadLimit` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: 整数 (bytes/s)，0=不限 |

### 设置全局上传限速

| 项目 | 值 |
|------|---|
| 名称 | `setUploadLimit` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | integer | 上传限速 (bytes/s) |

**返回：** 200

### 封禁 Peers

| 项目 | 值 |
|------|---|
| 名称 | `banPeers` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `peers` | string | `host:port` 管道分隔列表 |

**返回：** 200

---

## 种子管理

所有种子管理 API 在 `torrents` 下：`/api/v2/torrents/{methodName}`

### 获取种子列表

| 项目 | 值 |
|------|---|
| 名称 | `info` |
| 方法 | `GET` |

**参数（全部可选）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `filter` | string | 状态过滤：`all`/`downloading`/`seeding`/`completed`/`stopped`/`active`/`inactive`/`running`/`stalled`/`stalled_uploading`/`stalled_downloading`/`errored` |
| `category` | string | 按分类过滤（空=无分类，不传=任意分类） |
| `tag` | string | 按标签过滤（同上） |
| `sort` | string | 按响应 JSON 字段名排序 |
| `reverse` | bool | 反向排序 |
| `limit` | integer | 限制返回数量 |
| `offset` | integer | 偏移量（负数=从末尾偏移） |
| `hashes` | string | 按 hash 过滤，管道分隔 |

**返回：** 200: JSON 数组

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `added_on` | integer | 添加时间（Unix 时间戳） |
| `amount_left` | integer | 剩余下载量 (bytes) |
| `auto_tmm` | bool | 自动种子管理 |
| `availability` | float | 文件可用百分比 |
| `category` | string | 分类名称 |
| `completed` | integer | 已完成传输数据 (bytes) |
| `completion_on` | integer | 完成时间（Unix 时间戳） |
| `content_path` | string | 内容绝对路径 |
| `dl_limit` | integer | 下载限速 (bytes/s)，-1=不限 |
| `dlspeed` | integer | 下载速度 (bytes/s) |
| `downloaded` | integer | 已下载总量 (bytes) |
| `downloaded_session` | integer | 本次会话下载量 (bytes) |
| `eta` | integer | 预估剩余时间（秒） |
| `f_l_piece_prio` | bool | 首尾块优先 |
| `force_start` | bool | 强制开始 |
| `hash` | string | 种子 info hash |
| `isPrivate` | bool | **5.0 新增** 是否来自私有 tracker |
| `last_activity` | integer | 最后活动时间（Unix 时间戳） |
| `magnet_uri` | string | 磁力链接 |
| `max_ratio` | float | 最大分享率 |
| `max_seeding_time` | integer | 最大做种时间（秒） |
| `name` | string | 种子名称 |
| `num_complete` | integer | swarm 中的做种者 |
| `num_incomplete` | integer | swarm 中的下载者 |
| `num_leechs` | integer | 已连接的下载者 |
| `num_seeds` | integer | 已连接的做种者 |
| `priority` | integer | 优先级（-1=排队禁用或做种模式） |
| `progress` | float | 进度（0.0-1.0） |
| `ratio` | float | 分享率（最大 9999） |
| `ratio_limit` | float | 单种子分享率限制 |
| `reannounce` | integer | **v2.9.3 新增** 距下次重新宣告秒数 |
| `save_path` | string | 保存路径 |
| `seeding_time` | integer | 完成后经过时间（秒） |
| `seeding_time_limit` | integer | 单种子做种时间限制 |
| `seen_complete` | integer | 最后见到完成的时间 |
| `seq_dl` | bool | 顺序下载 |
| `size` | integer | 选中文件总大小 (bytes) |
| `state` | string | 种子状态（见下表） |
| `super_seeding` | bool | 超级做种 |
| `tags` | string | 逗号分隔的标签列表 |
| `time_active` | integer | 总活跃时间（秒） |
| `total_size` | integer | 所有文件总大小 (bytes) |
| `tracker` | string | 第一个正常工作的 tracker |
| `up_limit` | integer | 上传限速 (bytes/s)，-1=不限 |
| `uploaded` | integer | 已上传总量 (bytes) |
| `uploaded_session` | integer | 本次会话上传量 (bytes) |
| `upspeed` | integer | 上传速度 (bytes/s) |

**种子状态值：**

| 状态 | 说明 |
|------|------|
| `error` | 发生错误（已暂停） |
| `missingFiles` | 种子数据文件丢失 |
| `uploading` | 做种中，正在传输数据 |
| `pausedUP` | 已暂停，下载完成 |
| `queuedUP` | 排队等待上传 |
| `stalledUP` | 做种中，无连接 |
| `checkingUP` | 下载完成，正在检查 |
| `forcedUP` | 强制上传（忽略队列） |
| `allocating` | 正在分配磁盘空间 |
| `downloading` | 下载中，正在传输数据 |
| `metaDL` | 正在获取元数据 |
| `pausedDL` | 已暂停，未完成 |
| `queuedDL` | 排队等待下载 |
| `stalledDL` | 下载中，无连接 |
| `checkingDL` | 正在检查，未完成 |
| `forcedDL` | 强制下载 |
| `checkingResumeData` | 启动时检查恢复数据 |
| `moving` | 正在移动到其他位置 |
| `unknown` | 未知状态 |

### 获取种子通用属性

| 项目 | 值 |
|------|---|
| 名称 | `properties` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |

**返回：** 200: JSON 对象, 404: 种子不存在

**响应字段：**

| 属性 | 类型 | 说明 |
|------|------|------|
| `save_path` | string | 保存路径 |
| `creation_date` | integer | 创建时间（Unix 时间戳） |
| `piece_size` | integer | 分块大小 (bytes) |
| `comment` | string | 种子注释 |
| `total_wasted` | integer | 浪费数据 (bytes) |
| `total_uploaded` | integer | 总上传 (bytes) |
| `total_uploaded_session` | integer | 本次会话上传 (bytes) |
| `total_downloaded` | integer | 总下载 (bytes) |
| `total_downloaded_session` | integer | 本次会话下载 (bytes) |
| `up_limit` | integer | 上传限速 (bytes/s) |
| `dl_limit` | integer | 下载限速 (bytes/s) |
| `time_elapsed` | integer | 已用时间（秒） |
| `seeding_time` | integer | 完成后做种时间（秒） |
| `nb_connections` | integer | 连接数 |
| `nb_connections_limit` | integer | 连接限制 |
| `share_ratio` | float | 分享率 |
| `addition_date` | integer | 添加日期（Unix 时间戳） |
| `completion_date` | integer | 完成日期（Unix 时间戳） |
| `created_by` | string | 种子创建者 |
| `dl_speed_avg` | integer | 平均下载速度 (bytes/s) |
| `dl_speed` | integer | 下载速度 (bytes/s) |
| `eta` | integer | 预估剩余时间（秒） |
| `last_seen` | integer | 最后见到完成的时间 |
| `peers` | integer | 已连接 Peers |
| `peers_total` | integer | swarm 中 Peers |
| `pieces_have` | integer | 已拥有分块数 |
| `pieces_num` | integer | 总分块数 |
| `reannounce` | integer | 距下次重新宣告秒数 |
| `seeds` | integer | 已连接做种者 |
| `seeds_total` | integer | swarm 中做种者 |
| `total_size` | integer | 总大小 (bytes) |
| `up_speed_avg` | integer | 平均上传速度 (bytes/s) |
| `up_speed` | integer | 上传速度 (bytes/s) |
| `isPrivate` | bool | **5.0 新增** 是否私有种子 |

注：整数属性值未知时返回 -1。

### 获取种子 Trackers

| 项目 | 值 |
|------|---|
| 名称 | `trackers` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |

**返回：** 200: JSON 数组, 404: 种子不存在

**每个 Tracker 对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `url` | string | Tracker URL |
| `status` | integer | 0=禁用, 1=未联系, 2=正常, 3=更新中, 4=不工作 |
| `tier` | integer | 优先级层级 |
| `num_peers` | integer | Tracker 报告的 Peers |
| `num_seeds` | integer | Tracker 报告的做种者 |
| `num_leeches` | integer | Tracker 报告的下载者 |
| `num_downloaded` | integer | Tracker 报告的完成下载数 |
| `msg` | string | Tracker 消息 |

### 获取种子 Web Seeds

| 项目 | 值 |
|------|---|
| 名称 | `webseeds` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |

**返回：** 200: JSON 数组 `[{url: string}]`, 404: 种子不存在

### 获取种子文件内容

| 项目 | 值 |
|------|---|
| 名称 | `files` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `indexes` | string | (v2.8.2+) 文件索引，管道分隔 |

**返回：** 200: JSON 数组, 404: 种子不存在

**每个文件对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | integer | 文件索引 (v2.8.2+) |
| `name` | string | 文件名含相对路径 |
| `size` | integer | 文件大小 (bytes) |
| `progress` | float | 文件进度 (0.0-1.0) |
| `priority` | integer | 0=跳过, 1=普通, 6=高, 7=最高 |
| `is_seed` | bool | 文件是否做种/完成 |
| `piece_range` | integer[] | [起始块, 结束块] |
| `availability` | float | 可用百分比 |

### 获取种子分块状态

| 项目 | 值 |
|------|---|
| 名称 | `pieceStates` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |

**返回：** 200: JSON 整数数组, 404: 种子不存在

### 获取种子分块 Hash

| 项目 | 值 |
|------|---|
| 名称 | `pieceHashes` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |

**返回：** 200: JSON 字符串数组, 404: 种子不存在

### 停止种子

| 项目 | 值 |
|------|---|
| 名称 | `stop` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200

### 开始种子

| 项目 | 值 |
|------|---|
| 名称 | `start` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200

### 删除种子

| 项目 | 值 |
|------|---|
| 名称 | `delete` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔 |
| `deleteFiles` | bool | 是否删除文件 |

**返回：** 200

### 重新检查种子

| 项目 | 值 |
|------|---|
| 名称 | `recheck` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200

### 重新宣告种子

| 项目 | 值 |
|------|---|
| 名称 | `reannounce` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200

### 编辑 Tracker

| 项目 | 值 |
|------|---|
| 名称 | `editTracker` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `origUrl` | string | 原 Tracker URL |
| `newUrl` | string | 新 Tracker URL |

**返回：** 200=成功, 400=缺少参数, 404=种子不存在, 409=未找到 tracker 或新 URL 已存在

### 移除 Tracker

| 项目 | 值 |
|------|---|
| 名称 | `removeTrackers` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `urls` | string | Tracker URL 管道分隔 |

**返回：** 200=成功, 404=种子不存在, 409=未找到 tracker

### 添加 Peers

| 项目 | 值 |
|------|---|
| 名称 | `addPeers` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔 |
| `peers` | string | `host:port` 管道分隔 |

**返回：** 200=成功, 400=缺少参数

### 添加新种子

| 项目 | 值 |
|------|---|
| 名称 | `add` |
| 方法 | `POST` (multipart/form-data) |

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `urls` | string | 二选一 | URL 列表，换行分隔。支持 `http://`/`https://`/`magnet:`/`bc://bt/` |
| `torrents` | raw | 二选一 | .torrent 文件原始数据，可多次传 |
| `savepath` | string | 可选 | 下载路径 |
| `category` | string | 可选 | 分类 |
| `tags` | string | 可选 | 标签，逗号分隔 |
| `skip_checking` | string | 可选 | 跳过哈希检查 `true`/`false` |
| `paused` | string | 可选 | 添加后暂停 `true`/`false` |
| `root_folder` | string | 可选 | 创建根文件夹 `true`/`false`/不设置 |
| `rename` | string | 可选 | 重命名种子 |
| `upLimit` | integer | 可选 | 上传限速 (**bytes/s**) |
| `dlLimit` | integer | 可选 | 下载限速 (**bytes/s**) |
| `ratioLimit` | float | 可选 | 分享率限制 (v2.8.1+) |
| `seedingTimeLimit` | integer | 可选 | 做种时间限制 (**分钟**) (v2.8.1+) |
| `autoTMM` | bool | 可选 | 启用自动种子管理 |
| `sequentialDownload` | string | 可选 | 顺序下载 `true`/`false` |
| `firstLastPiecePrio` | string | 可选 | 首尾块优先 `true`/`false` |

> **注意：** v2.11.3 删除了 `cookie` 参数。添加种子时如需 Cookie，请使用 `/app/setCookies` API 预先设置。

**返回：** 200=成功, 415=种子文件无效

### 添加 Tracker 到种子

| 项目 | 值 |
|------|---|
| 名称 | `addTrackers` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `urls` | string | Tracker URL 换行分隔 |

**返回：** 200=成功, 404=种子不存在

### 提升种子优先级

| 项目 | 值 |
|------|---|
| 名称 | `increasePrio` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200=成功, 409=排队未启用

### 降低种子优先级

| 项目 | 值 |
|------|---|
| 名称 | `decreasePrio` |
| 方法 | `POST` |
| 参数/返回 | 同上 |

### 最高优先级

| 项目 | 值 |
|------|---|
| 名称 | `topPrio` |
| 方法 | `POST` |
| 参数/返回 | 同上 |

### 最低优先级

| 项目 | 值 |
|------|---|
| 名称 | `bottomPrio` |
| 方法 | `POST` |
| 参数/返回 | 同上 |

### 设置文件优先级

| 项目 | 值 |
|------|---|
| 名称 | `filePrio` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `id` | string | 文件索引管道分隔 |
| `priority` | integer | 0=跳过, 1=普通, 6=高, 7=最高 |

**返回：** 200=成功, 400=无效参数, 404=种子不存在, 409=元数据未下载

### 获取种子下载限速

| 项目 | 值 |
|------|---|
| 名称 | `downloadLimit` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200: JSON `{hash: limit_bytes_per_sec, ...}`

### 设置种子下载限速

| 项目 | 值 |
|------|---|
| 名称 | `setDownloadLimit` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |
| `limit` | integer | 下载限速 (bytes/s) |

**返回：** 200

### 设置种子分享限制

| 项目 | 值 |
|------|---|
| 名称 | `setShareLimits` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |
| `ratioLimit` | float | 分享率限制 |
| `seedingTimeLimit` | integer | 做种时间限制（分钟） |
| `inactiveSeedingTimeLimit` | integer | 不活跃做种时间限制（分钟） |

**返回：** 200=成功, 400=无效参数

### 获取种子上传限速

| 项目 | 值 |
|------|---|
| 名称 | `uploadLimit` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200: JSON `{hash: limit_bytes_per_sec, ...}`

### 设置种子上传限速

| 项目 | 值 |
|------|---|
| 名称 | `setUploadLimit` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |
| `limit` | integer | 上传限速 (bytes/s) |

**返回：** 200

### 设置种子位置

| 项目 | 值 |
|------|---|
| 名称 | `setLocation` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔 |
| `location` | string | 新位置 |

**返回：** 200=成功, 400=空位置, 403=无权限, 409=无法设置

### 重命名种子

| 项目 | 值 |
|------|---|
| 名称 | `rename` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `name` | string | 新名称 |

**返回：** 200=成功, 404=种子不存在, 409=无法重命名

### 设置种子分类

| 项目 | 值 |
|------|---|
| 名称 | `setCategory` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔 |
| `category` | string | 分类名称 |

**返回：** 200=成功, 409=分类名无效

### 获取所有分类

| 项目 | 值 |
|------|---|
| 名称 | `categories` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON `{name: {name, savePath}}` |

### 创建分类

| 项目 | 值 |
|------|---|
| 名称 | `createCategory` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `category` | string | 分类名称 |
| `savePath` | string | 保存路径 |

**返回：** 200=成功, 400=名称无效, 409=已存在

### 编辑分类

| 项目 | 值 |
|------|---|
| 名称 | `editCategory` |
| 方法 | `POST` |
| 参数 | 同 createCategory |
| 返回 | 200=成功, 400=名称无效, 409=不存在 |

### 删除分类

| 项目 | 值 |
|------|---|
| 名称 | `removeCategories` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `categories` | string | 分类名换行分隔 |

**返回：** 200

### 添加种子标签

| 项目 | 值 |
|------|---|
| 名称 | `addTags` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |
| `tags` | string | 标签逗号分隔 |

**返回：** 200

### 移除种子标签

| 项目 | 值 |
|------|---|
| 名称 | `removeTags` |
| 方法 | `POST` |
| 参数 | 同 addTags |
| 返回 | 200 |

### 获取所有标签

| 项目 | 值 |
|------|---|
| 名称 | `tags` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON 字符串数组 |

### 创建标签

| 项目 | 值 |
|------|---|
| 名称 | `createTags` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `tags` | string | 标签逗号分隔 |

**返回：** 200

### 删除标签

| 项目 | 值 |
|------|---|
| 名称 | `deleteTags` |
| 方法 | `POST` |
| 参数 | 同 createTags |
| 返回 | 200 |

### 设置自动种子管理

| 项目 | 值 |
|------|---|
| 名称 | `setAutoManagement` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔 |
| `enable` | bool | 启用/禁用 |

**返回：** 200

### 切换顺序下载

| 项目 | 值 |
|------|---|
| 名称 | `toggleSequentialDownload` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔，或 `all` |

**返回：** 200

### 设置首尾块优先

| 项目 | 值 |
|------|---|
| 名称 | `toggleFirstLastPiecePrio` |
| 方法 | `POST` |
| 参数 | 同上 |
| 返回 | 200 |

### 设置强制开始

| 项目 | 值 |
|------|---|
| 名称 | `setForceStart` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hashes` | string | 种子 hash 管道分隔 |
| `value` | bool | 启用/禁用 |

**返回：** 200

### 设置超级做种

| 项目 | 值 |
|------|---|
| 名称 | `setSuperSeeding` |
| 方法 | `POST` |
| 参数 | 同 setForceStart |
| 返回 | 200 |

### 重命名文件

| 项目 | 值 |
|------|---|
| 名称 | `renameFile` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `hash` | string | 种子 hash |
| `oldPath` | string | 旧路径 |
| `newPath` | string | 新路径 |

**返回：** 200=成功, 400=无效路径, 409=无法重命名

### 重命名文件夹

| 项目 | 值 |
|------|---|
| 名称 | `renameFolder` |
| 方法 | `POST` |
| 参数 | 同 renameFile |
| 返回 | 同 renameFile |

---

## RSS（实验性）

所有 RSS API 在 `rss` 下：`/api/v2/rss/{methodName}`

### 添加文件夹

| 项目 | 值 |
|------|---|
| 名称 | `addFolder` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `path` | string | 文件夹路径 |

**返回：** 200=成功, 409=路径冲突

### 添加 Feed

| 项目 | 值 |
|------|---|
| 名称 | `addFeed` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `url` | string | Feed URL |
| `path` | string | 可选，存放路径 |

**返回：** 200=成功, 409=路径冲突

### 移除项目

| 项目 | 值 |
|------|---|
| 名称 | `removeItem` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `path` | string | 项目路径 |

**返回：** 200=成功, 409=路径冲突

### 移动项目

| 项目 | 值 |
|------|---|
| 名称 | `moveItem` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `itemPath` | string | 源路径 |
| `destPath` | string | 目标路径 |

**返回：** 200=成功, 409=路径冲突

### 获取所有项目

| 项目 | 值 |
|------|---|
| 名称 | `items` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `withData` | bool | 可选，是否包含数据 |

**返回：** 200: JSON 嵌套对象

### 标记已读

| 项目 | 值 |
|------|---|
| 名称 | `markAsRead` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `itemPath` | string | 项目路径 |
| `articleId` | string | 可选，文章 ID（不传则标记全部已读） |

**返回：** 200

### 刷新项目

| 项目 | 值 |
|------|---|
| 名称 | `refreshItem` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `itemPath` | string | 项目路径 |

**返回：** 200

### 设置自动下载规则

| 项目 | 值 |
|------|---|
| 名称 | `setRule` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `ruleName` | string | 规则名称 |
| `ruleDef` | string | 规则定义 JSON 字符串 |

**返回：** 200

**规则定义字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用 |
| `mustContain` | string | 种子名必须包含的子串 |
| `mustNotContain` | string | 种子名不能包含的子串 |
| `useRegex` | bool | 启用正则模式 |
| `episodeFilter` | string | 剧集过滤器 |
| `smartFilter` | bool | 启用智能剧集过滤 |
| `previouslyMatchedEpisodes` | list | 已匹配的剧集 ID |
| `affectedFeeds` | list | 适用规则的 Feed URL 列表 |
| `ignoreDays` | number | 忽略后续匹配的天数 |
| `lastMatch` | string | 最后匹配时间 |
| `addPaused` | bool | 添加后暂停 |
| `assignedCategory` | string | 指定分类 |
| `savePath` | string | 保存目录 |

### 重命名自动下载规则

| 项目 | 值 |
|------|---|
| 名称 | `renameRule` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `ruleName` | string | 旧名称 |
| `newRuleName` | string | 新名称 |

**返回：** 200

### 删除自动下载规则

| 项目 | 值 |
|------|---|
| 名称 | `removeRule` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `ruleName` | string | 规则名称 |

**返回：** 200

### 获取所有自动下载规则

| 项目 | 值 |
|------|---|
| 名称 | `rules` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON 所有规则定义 |

### 获取匹配规则的文章

| 项目 | 值 |
|------|---|
| 名称 | `matchingArticles` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `ruleName` | string | 规则名称 |

**返回：** 200: JSON `{feedName: [articleList]}`

---

## 搜索

所有搜索 API 在 `search` 下：`/api/v2/search/{methodName}`

### 开始搜索

| 项目 | 值 |
|------|---|
| 名称 | `start` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `pattern` | string | 搜索关键词 |
| `plugins` | string | 插件管道分隔，或 `all`/`enabled` |
| `category` | string | 搜索分类 |

**返回：** 200: `{id: integer}`, 409=已达到最大并发搜索数

### 停止搜索

| 项目 | 值 |
|------|---|
| 名称 | `stop` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | number | 搜索 ID |

**返回：** 200=成功, 404=未找到

### 获取搜索状态

| 项目 | 值 |
|------|---|
| 名称 | `status` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | number | 可选，搜索 ID |

**返回：** 200: `[{id, status, total}]`, 404=未找到

### 获取搜索结果

| 项目 | 值 |
|------|---|
| 名称 | `results` |
| 方法 | `GET` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | number | 搜索 ID |
| `limit` | number | 可选，限制数量 |
| `offset` | number | 可选，偏移量 |

**返回：** 200: `{results: [...], status: string, total: integer}`, 404/409

### 删除搜索

| 项目 | 值 |
|------|---|
| 名称 | `delete` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | number | 搜索 ID |

**返回：** 200=成功, 404=未找到

### 获取搜索插件

| 项目 | 值 |
|------|---|
| 名称 | `plugins` |
| 方法 | `GET` |
| 参数 | 无 |
| 返回 | 200: JSON 插件对象数组 |

### 安装搜索插件

| 项目 | 值 |
|------|---|
| 名称 | `installPlugin` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `sources` | string | URL 或路径管道分隔 |

**返回：** 200

### 卸载搜索插件

| 项目 | 值 |
|------|---|
| 名称 | `uninstallPlugin` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `names` | string | 插件名管道分隔 |

**返回：** 200

### 启用/禁用搜索插件

| 项目 | 值 |
|------|---|
| 名称 | `enablePlugin` |
| 方法 | `POST` |

| 参数 | 类型 | 说明 |
|------|------|------|
| `names` | string | 插件名管道分隔 |
| `enable` | bool | 启用/禁用 |

**返回：** 200

### 更新搜索插件

| 项目 | 值 |
|------|---|
| 名称 | `updatePlugins` |
| 方法 | `POST` |
| 参数 | 无 |
| 返回 | 200 |

---

## WebAPI 版本规则

版本号格式：`主版本.次版本.补丁版本`

| 位 | 含义 | 示例 |
|----|------|------|
| 主版本 | 全局重新设计 | 2.x.x |
| 次版本 | 不兼容变更（破坏旧客户端） | x.11.x |
| 补丁版本 | 兼容变更（不破坏旧客户端） | x.x.3 |

---

## 单位参考

> **关键：不同 API 使用的单位不一致！**

| 上下文 | 单位 | 适用 API |
|--------|------|---------|
| Transfer API 全局下载/上传限速 | **bytes/s** | `downloadLimit`, `uploadLimit`, `setDownloadLimit`, `setUploadLimit` |
| Transfer API 全局传输速率 | **bytes/s** | `dl_info_speed`, `up_info_speed`, `dl_rate_limit`, `up_rate_limit` |
| 单种子限速 | **bytes/s** | `torrents/downloadLimit`, `setDownloadLimit`, `uploadLimit`, `setUploadLimit` |
| 单种子速度 | **bytes/s** | `dlspeed`, `upspeed`, `dl_speed`, `up_speed` |
| 添加种子时限速 | **bytes/s** | `upLimit`, `dlLimit` |
| 偏好设置中的限速 | **KiB/s** | `dl_limit`, `up_limit`, `alt_dl_limit`, `alt_up_limit` |
| 慢速阈值 | **KiB/s** | `slow_torrent_dl_rate_threshold`, `slow_torrent_ul_rate_threshold` |
| 做种时间限制 | **分钟** | `seedingTimeLimit`, `inactiveSeedingTimeLimit` |
| 数据量 | **bytes** | `downloaded`, `uploaded`, `total_size`, `size` 等 |
| 缓存大小 | **MiB** | `disk_cache`, `checking_memory_use` |

### 转换速查

```
1 KiB/s = 1024 bytes/s
偏好设置值(KiB/s) × 1024 = API 值(bytes/s)
API 值(bytes/s) ÷ 1024 = 偏好设置值(KiB/s)
```

---

## 与 4.x 版本的差异

> PT-Forward 需兼容 qBittorrent v4.1.0 - v5.x（WebAPI v2.0 - v2.11.3）
> 本章节详细记录 4.x → 5.0 的所有破坏性变更和新增功能

### 4.x WebAPI 版本演进（v2.0 - v2.8.3）

| WebAPI 版本 | qBittorrent 版本 | 关键变更 |
|------------|-----------------|---------|
| v2.0 | 4.1.0 | 基线版本，本章节以此为基础 |
| v2.1.0 | 4.1.1 | `/sync/maindata` categories 从 array→object；新增 `/torrents/editCategory` |
| v2.1.1 | 4.1.2 | 新增 `/torrents/categories`、`/search/` 方法；`free_space_on_disk` 字段 |
| v2.2.0 | 4.1.3 | 新增 `/torrents/editTracker`、`/torrents/removeTracker`；tracker 状态改为整数；标签系统 |
| v2.2.1 | 4.1.4 | 新增 `rss/refreshItem` |
| v2.3.0 | 4.2.0 | 移除 `web_ui_password` 读权限；新增 `/app/buildInfo`、`/torrents/addPeers`、标签方法 |
| v2.4.0 | 4.2.5 | 新增 `/torrents/renameFile` |
| v2.4.1 | 4.3.0 | `filter` 新增 `stalled/stalled_uploading/stalled_downloading`；新增安全/超时偏好 |
| v2.5.0 | 4.3.1 | 移除 `enable_super_seeding` 偏好字段 |
| v2.5.1 | 4.3.2 | 新增 HTTP 自定义头/RSS 高级配置；新增 `rss/markAsRead`、`rss/matchingArticles` |
| v2.6.0 | 4.3.3 | 移除 `/search/categories`；修改 `/search/plugins` 响应格式 |
| v2.6.1 | 4.3.3 | 新增 `content_path` 字段到 `/torrents/info` |
| v2.6.2 | 4.3.3 | 新增 `tags` 可选参数到 `/torrents/add` |
| v2.8.0 | 4.3.3 | 新增 `/torrents/renameFolder`；修改 `/torrents/renameFile` 参数（注意版本号 bug） |
| v2.8.1 | 4.4.0 | 新增 `ratioLimit`/`seedingTimeLimit` 到 `/torrents/add`；新增 `seeding_time` 字段 |
| v2.8.2 | 4.4.0 | 新增 `indexes` 参数到 `/torrents/files`；新增 `index` 响应字段 |
| v2.8.3 | 4.4.0 | 新增 `tag` 过滤参数到 `/torrents/info` |

### 5.0 破坏性变更

#### 1. 暂停/恢复端点重命名

| 操作 | 4.x（v2.0-v2.8.3） | 5.0（v2.9.3+） |
|------|--------------------|--------------------|
| 暂停种子 | `POST /api/v2/torrents/pause` | `POST /api/v2/torrents/stop` |
| 恢复种子 | `POST /api/v2/torrents/resume` | `POST /api/v2/torrents/start` |

**兼容方案：** 运行时通过 `/api/v2/app/webapiVersion` 检测版本，动态选择端点：
```go
func (c *QBittorrentClient) Pause(hashes []string) error {
    if c.isV5() {
        return c.post("/api/v2/torrents/stop", params)
    }
    return c.post("/api/v2/torrents/pause", params)
}
```

#### 2. 过滤参数值变更

| 过滤条件 | 4.x | 5.0 |
|---------|-----|-----|
| 暂停中 | `filter=paused` | `filter=stopped`（5.0 也兼容 `paused`） |
| 进行中 | `filter=resumed` | `filter=running`（5.0 也兼容 `resumed`） |

**兼容方案：** 4.x 使用 `paused`/`resumed`，5.0 优先使用 `stopped`/`running`。种子状态值（`pausedUP`/`pausedDL` 等）**未变**，无需适配。

#### 3. Cookie 参数移除（PT-Forward 不涉及）

| 操作 | 4.x | 5.0 |
|------|-----|-----|
| 添加种子带 Cookie | `/torrents/add` 直接传 `cookie` 参数 | `cookie` 参数已删除，需先调用 `/app/setCookies` |

> **PT-Forward 不使用此功能。** PT 站点种子下载由 PT-Forward 自身完成（通过 `SiteCredentialProvider`），
> 种子以文件方式推送给下载器（`AddFromFile`）。NP 架构站点 URL 自带 passkey，`AddFromURL` 无需额外 cookie。

### 5.0 新增功能

| 变更 | WebAPI 版本 | 说明 |
|------|-----------|------|
| `reannounce` 字段 | v2.9.3 | `/torrents/info` 响应新增，距下次 tracker 重新宣告的秒数 |
| `isPrivate` 字段 | 5.0.0 | `/torrents/info` 和 `/torrents/properties` 响应新增 |
| Cookie 管理 API | v2.11.3 | `GET /app/cookies` 和 `POST /app/setCookies` |
| `inactiveSeedingTimeLimit` | v2.11.3 | `/torrents/setShareLimits` 新增不活跃做种时间限制参数 |
| `stop`/`start` 端点 | 5.0.0 | 替代 `pause`/`resume` |

### 版本检测策略

```
启动时:
  GET /api/v2/app/webapiVersion → "2.8.3" 或 "2.11.3"
  GET /api/v2/app/version      → "v4.6.7" 或 "v5.0.0"

解析主版本号:
  major = parseMajor(version)  // 4 或 5

缓存:
  client.apiVersion = "2.8.3"
  client.majorVersion = 4 或 5
  client.isV5 = (major >= 5)

运行时分支:
  if client.isV5 {
      // 使用 5.0 新端点
  } else {
      // 使用 4.x 兼容端点
  }
```

### 最小兼容版本功能清单（v4.1.0 / WebAPI v2.0）

以下功能在 4.1.0 基线上**可用**：

| 功能 | 可用性 |
|------|-------|
| 认证（Cookie SID） | ✅ |
| 种子增删查改 | ✅ |
| 暂停/恢复（pause/resume） | ✅ |
| 限速（单种子/全局） | ✅ |
| 分类管理 | ⚠️ v2.0 仅创建，v2.1.0+ 支持编辑 |
| 标签系统 | ❌ v2.3.0+（4.2.0+） |
| Tracker 编辑/删除 | ❌ v2.2.0+（4.1.3+） |
| 文件重命名 | ❌ v2.4.0+（4.2.5+） |
| 添加时指定标签 | ❌ v2.6.2+（4.4.0+） |
| 添加时指定分享率限制 | ❌ v2.8.1+（4.4.0+） |

**PT-Forward 最小推荐版本：qBittorrent 4.2.0+（WebAPI v2.3.0+）**，以确保标签系统和 Tracker 管理可用。

### PT-Forward 设计影响

1. **版本检测** — 启动时调用 `webapiVersion` 缓存主版本号，全生命周期使用
2. **暂停/恢复** — 4.x 用 `pause`/`resume`，5.x 用 `stop`/`start`，运行时分支
3. **过滤参数** — 4.x 用 `paused`/`resumed`，5.x 优先 `stopped`/`running`
4. **isPrivate 字段** — 5.x 可直接判断私有种子，4.x 无此字段需忽略
5. **reannounce 字段** — 5.x 可用于刷流引擎 tracker 策略，4.x 无此字段需忽略
6. **功能降级** — 4.1.0 基线缺少标签/Tracker 编辑等，需做特性检测降级处理
7. **限速单位** — 不随版本变化：Torrents/Transfer API 全部 bytes/s，Preferences 用 KiB/s
8. **Cookie 参数不使用** — PT-Forward 自己下载种子文件（通过 `SiteCredentialProvider`），主用 `AddFromFile` 推送。`/torrents/add` 的 `cookie` 参数（4.x）和 `/app/setCookies`（5.x）均不需要适配

---

## 附录 A：技术架构

### A.1 基本信息

| 属性 | 值 |
|------|-----|
| **项目名称** | qBittorrent |
| **技术栈** | C++ / Qt6 / libtorrent |
| **许可证** | GPL-2.0+ |
| **代码规模** | ~150,000+ 行 C++ 代码 |
| **主要用途** | BT 下载客户端（PT 场景首选） |
| **WebUI 端口** | 默认 8080 |
| **API 风格** | RESTful (JSON) |

### A.2 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                     qBittorrent 架构                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │    GUI 层     │  │   WebUI 层   │  │   CLI 层     │      │
│  │  (Qt Widgets) │  │  (HTTP API)  │  │ (命令行)     │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
│         └─────────────────┼─────────────────┘               │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────┐        │
│  │              Application Layer (src/app)          │        │
│  └─────────────────────┬────────────────────────────┘        │
│                        ▼                                      │
│  ┌──────────────────────────────────────────────────┐        │
│  │              Base Layer (src/base)                │        │
│  │  BitTorrent Session | HTTP Server | Preferences   │        │
│  └─────────────────────┬────────────────────────────┘        │
│                        ▼                                      │
│  ┌──────────────────────────────────────────────────┐        │
│  │              libtorrent (rasterbar)              │        │
│  └──────────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### A.3 核心目录结构

```
qBittorrent/src/
├── app/                    # 应用层（入口/命令行）
├── base/                   # 核心基础库
│   ├── bittorrent/       # BT 协议实现（Session 单例）
│   ├── http/             # HTTP 服务器（QTcpServer）
│   ├── preferences.h/cpp # 配置管理器
│   └── rss/              # RSS 引擎
├── webui/                  # WebUI 模块
│   ├── api/              # 9 个 API 控制器
│   │   ├── torrentscontroller.cpp (62KB, 50+ 接口)
│   │   ├── synccontroller.cpp (40KB, 增量同步)
│   │   ├── appcontroller.cpp (60KB, 30+ 接口)
│   │   ├── authcontroller.cpp (PBKDF2 认证)
│   │   └── rsscontroller.cpp (RSS 规则引擎)
│   └── www/               # Vue.js 前端
├── gui/                    # Qt GUI 界面
└── searchengine/           # Python 搜索引擎插件
```

### A.4 API 控制器清单

| 控制器 | 文件大小 | Action 数 | 核心功能 |
|--------|---------|----------|---------|
| TorrentsController | 62KB | 50+ | 种子 CRUD/限速/优先级/Tracker |
| SyncController | 40KB | 2 | 增量同步（RID 机制） |
| AppController | 60KB | 30+ | 应用配置/版本信息 |
| TransferController | 6KB | 10 | 全局速度限制 |
| AuthController | 4KB | 2 | 登录/登出（PBKDF2） |
| RSSController | 6KB | 12 | Feed 管理/自动下载规则 |
| SearchController | 12KB | 10 | 搜索引擎集成 |
| LogController | 4KB | 2 | 日志查询 |
| TorrentCreatorController | 9KB | 1 | 创建种子文件 |

### A.5 增量同步原理

```
Client                              Server
  │                                   │
  │── GET /sync/maindata?rid=0 ─────→│
  │←── 完整快照 (rid:1) ─────────────│
  │                                   │
  │      (等待变化...)                 │
  │                                   │ TorrentAdded 事件
  │                                   │ 更新缓冲区
  │                                   │
  │── GET /sync/maindata?rid=1 ─────→│
  │←── 增量更新 (rid:2) ─────────────│
  │      只传输变化部分               │
  │                                   │
  │      (循环...)                     │
```

**性能优势：** 流量从 500KB+/次 → 1-10KB/次，适合 100-1000 种子的 PT 场景。

---

## 附录 B：PT 优化指南

### B.1 BitTorrent Session 核心枚举

```cpp
enum class BTProtocol { Both=0, TCP=1, UTP=2 };
enum class ChokingAlgorithm { FixedSlots=0, RateBased=1 };        // PT 推荐 RateBased
enum class SeedChokingAlgorithm { RoundRobin=0, FastestUpload=1, AntiLeech=2 }; // PT 推荐 FastestUpload
enum class DiskIOType { Default=0, MMap=1, Posix=2, SimplePreadPwrite=3 };
enum class DiskIOMode { DisableOSCache=0, EnableOSCache=1 };
```

**PT 推荐配置：**
- 协议: Both 或 TCP（兼容性最好）
- 阻塞算法: RateBased + FastestUpload（最大化上传效率）
- 磁盘 IO: 大文件用 MMap，内存紧张用 SimplePreadPwrite

### B.2 磁盘 IO 配置指南

| 场景 | Read Mode | Write Mode | IO Type |
|------|-----------|------------|---------|
| 大文件 (>10GB) | DisableOSCache | WriteThrough | MMap |
| SSD 存储 | EnableOSCache | EnableOSCache | Default |
| HDD (大量种子) | DisableOSCache | DisableOSCache | Posix |
| 内存充足 (>16GB) | EnableOSCache | EnableOSCache | Default |
| 内存紧张 (<8GB) | DisableOSCache | DisableOSCache | SimplePreadPwrite |

### B.3 网络连接限制推荐值

| 用户级别 | 全局连接数 | 每种子连接数 |
|---------|-----------|------------|
| 普通用户 | 200 | 50 |
| VIP 用户 | 500 | 100 |
| 上传者 | 1000 | 200 |
| 服务器 | 2000 | 500 |

### B.4 分类和标签 PT 最佳实践

```python
# 推荐分类
categories = {
    "movie": "/downloads/movie/",
    "movie/4k": "/downloads/movie/4k/",
    "tv": "/downloads/tv/"
}

# 推荐标签
tags = ["pt", "free", "hr", "exclusive", "auto-dl", "reseed"]
```

---

## 附录 C：安全机制

### C.1 多层防御体系

| 层级 | 机制 | 实现 |
|------|------|------|
| L5 | 应用审计 | 操作日志/异常检测 |
| L4 | 会话管理 | Cookie Session/CSRF Token |
| L3 | 认证授权 | PBKDF2 哈希/IP 封禁/时序攻击防护 |
| L2 | 传输加密 | HTTPS/TLS 1.2+ |
| L1 | 网络层 | Host 白名单/Bind Address |

### C.2 PBKDF2 密码哈希

```cpp
namespace Utils::Password {
    class PBKDF2 {
        static constexpr int ITERATIONS = 100000;
        static constexpr int SALT_LENGTH = 32;
        static constexpr int KEY_LENGTH = 32;
    };
}
```

### C.3 IP 封禁配置

```ini
WebUI\MaxAuthFailCount=5     # 失败阈值
WebUI\BanDuration=3600       # 封禁时长（秒）
```

| 环境 | 失败阈值 | 封禁时长 |
|------|---------|---------|
| 内网 | 5 次 | 1 小时 |
| 公网 | 3 次 | 2 小时 |
| 高安全 | 1 次 | 24 小时 |

---

## 附录 D：与 Transmission 对比

| 特性 | qBittorrent | Transmission |
|------|-------------|--------------|
| **技术栈** | C++/Qt/libtorrent | C++/libevent/custom |
| **代码规模** | ~150K 行 | ~80K 行 |
| **内存占用** | ~200MB | ~50MB |
| **API 完整性** | 50+ 接口 | 基础完整 |
| **RSS 支持** | 内置引擎 | 需第三方工具 |
| **分类/标签** | 完善（层级分类+扁平标签） | 基础 |
| **搜索插件** | 内置 | 无 |
| **PT 适配性** | **最佳选择** | 轻量备选 |

**选择建议：**
- PT 重度用户/多站点/自动化 → qBittorrent
- NAS/嵌入式/资源受限 → Transmission
- 需要高级功能（RSS/搜索/分类） → 必须使用 qBittorrent

---

## 附录 E：Docker 部署

```yaml
services:
  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
      - WEBUI_PORT=8080
    volumes:
      - ./config:/config
      - /downloads:/downloads
    ports:
      - 8080:8080
      - 6881:6881
      - 6881:6881/udp
    restart: unless-stopped
```

**生产环境配置要点：**

```ini
[Preferences]
Connection\GlobalMaxConnections=500
BitTorrent\Session\Encryption=1
WebUI\MaxAuthFailCount=5
WebUI\BanDuration=3600
RSS\AutoDownloadingEnabled=true
LogLevel=1
```

---

*整理日期：2026-04-14*
*来源：qBittorrent 官方 Wiki（4.1 + 5.0 两版 API 文档）*
*适用于 qBittorrent v4.1.0 - v5.x，WebAPI v2.0 - v2.11.3*
*附录内容合并自原 qBittorrent 4.x 技术研究报告*
