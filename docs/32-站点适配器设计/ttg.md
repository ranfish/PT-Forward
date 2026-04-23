# TTG（ToTheGlory）站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | TTG（ToTheGlory，套套哥） |
| 域名 | totheglory.im |
| 框架 | TTG 自研（非标准 NexusPHP，有 NP 血统但大量定制） |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（offers.php） |
| PT-Gen | 是 |
| IMDb | 是（imdb_c） |
| 豆瓣 | 是（douban_id） |
| 匿名发布 | 是（anonymity: yes/no） |
| NFO | 是 |
| H&R | 是（上传者可选，hr 字段） |
| 禁转 | 是（nodistr 字段） |
| 标签体系 | **无**（无 tags 字段） |

## Tracker URL
`https://totheglory.im/announce.php`（种子文件名需去 `[TTG]` 前缀）

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `subtitle` | 否 | |
| 高亮 | `highlight` | 否 | |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| IMDb链接 | `imdb_c` | 否 | |
| 豆瓣ID | `douban_id` | 否 | |
| 分类 | `type` | 是 | 58 个分类 |
| 匿名 | `anonymity` | 否 | yes/no |
| 禁转 | `nodistr` | 否 | yes/no |
| 制作组 | `team` | 否 | hidden，由系统分配 |
| H&R | `hr` | 否 | hidden，默认 no |

### 隐藏字段
- `MAX_FILE_SIZE`: 4000000
- `team`: 空（由系统分配）
- `hr`: "no"（默认）

### 缺失字段（与其他站点的重大差异）
- **无 medium_sel**：无媒介下拉框
- **无 codec_sel**：无编码下拉框
- **无 standard_sel**：无分辨率下拉框
- **无 audiocodec_sel**：无音频编码下拉框
- **无 team_sel**：无制作组下拉框
- **无 tags**：无标签体系
- **无 source_sel / processing_sel**：无来源/处理下拉框

**TTG 的质量信息完全依赖标题解析和分类推断，不提供表单级质量下拉框。**

## 分类 (type，58 个)

### 电影（Movies）

| ID | 名称 |
|----|------|
| 51 | 电影DVDRip |
| 52 | 电影720p |
| 53 | 电影1080i/p |
| 54 | BluRay原盘 |
| 108 | 影视2160p |
| 109 | UHD原盘 |

### 剧集（TV Series）

| ID | 名称 |
|----|------|
| 69 | 欧美剧720p(单集) |
| 70 | 欧美剧1080i/p(单集) |
| 73 | 高清日剧 |
| 74 | 高清韩剧 |
| 75 | 大陆港台剧1080i/p(单集) |
| 76 | 大陆港台剧720p(单集) |
| 87 | 欧美剧包(全集) |
| 88 | 日剧包 |
| 90 | 华语剧包(全集) |
| 99 | 韩剧包 |

### 纪录片（Documentaries）

| ID | 名称 |
|----|------|
| 62 | 纪录片720p |
| 63 | 纪录片1080i/p |
| 67 | 纪录片BluRay原盘 |

### 动漫（Animations）

| ID | 名称 |
|----|------|
| 58 | 高清动漫 |
| 111 | 动漫原盘 |

### 综艺（TV Shows）

| ID | 名称 |
|----|------|
| 60 | 高清综艺 |
| 101 | 日本综艺 |
| 103 | 韩国综艺 |

### 音乐/音轨（Music/Audio）

| ID | 名称 |
|----|------|
| 59 | MV&演唱会 |
| 82 | (电影原声&Game)OST |
| 83 | 无损音乐FLAC&APE |
| 84 | 补充音轨 |

### 体育（Sports）

| ID | 名称 |
|----|------|
| 57 | 高清体育节目 |

### 游戏（Games）

| ID | 名称 |
|----|------|
| 28 | PC |
| 47 | MAC |
| 5 | XBOX360 |
| 105 | XBOX1 |
| 45 | XBOX to XBOX360 |
| 49 | XBLA |
| 26 | PS2 |
| 46 | PS3 |
| 104 | PS4 |
| 29 | PSP |
| 107 | PSV |
| 110 | SWITCH |
| 44 | NDS |
| 106 | WIIU |
| 27 | WII |
| 43 | NGC |
| 48 | PSP兼容高清&标清 |
| 33 | PS3兼容高清 |
| 30 | Game Video |
| 31 | XBOX360兼容高清 |
| 93 | iPhone/iPad游戏 |

### 移动端/其他（Mobile/Misc）

| ID | 名称 |
|----|------|
| 91 | MiniVideo |
| 92 | iPhone/iPad视频 |
| 94 | iPad书籍 |
| 95 | iPhone/iPad软件 |
| 56 | Ebook |
| 32 | Game Ebook |
| 77 | APPZ |

## 质量字段

**无任何标准质量下拉框。** TTG 没有以下字段：
- medium_sel（媒介）
- codec_sel（编码）
- audiocodec_sel（音频编码）
- resolution_sel（分辨率）
- source_sel（来源）
- processing_sel（处理方式）
- tags（标签）

**所有质量信息（分辨率、媒介、编码等）都编码在 type 分类中**，例如：
- `电影1080i/p` = 电影 + 1080 分辨率
- `BluRay原盘` = 电影 + Blu-ray 媒介
- `UHD原盘` = 电影 + UHD 媒介

## 禁转标识（关键：无标签体系）

TTG **没有标签体系**，不能通过标签判断是否禁转。禁转的识别方式：

### 方式一：种子列表页（browse.php）
在种子列表的"名称"列中，标题链接**前面**会出现 `<b>禁转</b>` 文字：

```
正常种子：[img:置顶][link:/t/811032/]My Beautiful Man...
禁转种子：[img:置顶][b]禁转[/b][link:/t/810773/]The Super Mario Bros...
```

真实 HTML 结构对比：
```html
<!-- 正常种子 -->
<a href="/t/811032/"><b>My Beautiful Man Eternal 2023 1080p...</b></a>

<!-- 禁转种子 -->
<b>禁转</b><a href="/t/810773/"><b>The Super Mario Bros Movie 2023 BluRay...</b></a>
```

**检测方法**：在标题链接 `<a>` 的前一个兄弟节点中查找 `<b>禁转</b>` 文字。

### 方式二：种子详情页（details.php）
详情页中会显示"本种子是禁转资源"文字提示。

### 方式三：上传表单（upload.php）
上传者通过 `nodistr` 字段标记禁转（值为 "yes" 或 "no"）。

### 官方规则原文
> 标记"🔒"的种子文件和内容不允许重新做种或上传到其他站点；在某些被允许的特殊情况下，转载时要务必保留原作者信息和网站出处等字样！同样我们也尊重其他网站声明的独占资源，被举报确认后将会被处理！

## H&R 制度

### 规则
- 单集档案：下载开始累计做种 **24 小时** or 上传量 > 种子体积
- 整季打包：下载开始累计做种 **60 小时** or 上传量 > 种子体积
- 下载未达种子体积 **10%** 以上者，不启动 H&R
- H&R 约束自发布之日起 **60 天**后自动取消
- **14 天缓冲期**：从下载开始之时起，14 天内完成标准即算及格
- 资深组、工作组和 **VIP 免疫** HR

### HP（Health Points）
- 每会员初始 **5 点 HP**
- 每个 H&R 违规扣 **1 点 HP**
- 单个 H&R 做种超 **240 小时**奖 **1 点 HP**（上限 15 点）
- HP 可用积分购买回血道具
- HP 归零 → 积分兑换 → 仍不足 → 自动禁用

## 盒子（SeedBox）规则

- **禁止共享 IP 的盒子**，违者 BAN
- 使用盒子必须到控制面板登记（供应商、IP、带宽、客户端版本、起止时间）
- 单种上传限速 **100 MB/s**，全局不限
- 盒子下载流量按"黑种"(100%)计量，不享受促销优惠（free/50%/30% 等）
- 发布 **72 小时**内，盒子最多得到种子体积 **3 倍**上传流量
- 72 小时后正常统计
- **自己发布的种子不受此限制**
- VIP 不受此约束

## 账号保留规则

| 等级 | 保留条件 |
|------|---------|
| BrontoByte 及以上 | 永久保留 |
| ExaByte 及以上 | 挂起账号后不会被禁用 |
| PetaByte 及以下 | 挂起账号连续 180 天不登录 → 自动禁用 |
| BrontoByte 以下未挂起 | 连续 12 周不登录 → 自动禁用 |
| 新注册 | 连续 7 天无流量 → 自动禁用 |

## 下载规则

### 分享率标准
| 下载量 | 最低分享率 |
|--------|-----------|
| < 10 GB | > 0.3 |
| > 10 GB | > 0.4 |
| > 20 GB | > 0.5 |
| > 50 GB | > 0.6 |
| > 100 GB | > 0.7 |

### 允许的客户端
- µTorrent 2.0.4 / 2.2.1 / 3.5.3 ~ 3.5.5
- µTorrent for Mac 1.8.4+
- Azureus 3.0.5.0+
- libTorrent(rtorrent) 9.4+
- Transmission 0.96+
- KTorrent 2.2.4+
- BitSpirit 3.6.0+
- qBittorrent 3.3+

## 上传规则

- 必须禁止 DHT 网络
- 至少 3 人下载后才能撤种（无人下载 72 小时后可撤）
- 种子禁止含无关内容（广告链接等）
- 游戏 > 2G 必须为原始镜像（ISO）
- 必须按规定命名种子（详见 https://totheglory.im/forums.php?action=viewtopic&topicid=8&page=p70#70）
- Uploader 组成员可无视描述要求

## 合集打包规则

### 电影类
- **禁止任何形式的打包**
- 不得发布已完结系列电影合集
- 不得发布发行商官方原盘合集及 DIY/Remux/重编码合集
- 不得发布导演/演员/IMDb Top 250/豆瓣 Top 250 等私人合集

### 剧集类
- **仅允许单季打包，禁止多季打包**
- 允许：全集（单季）参数同源
- 鼓励将已完结单剧打包为全集

### 音乐/MV/体育/综艺/教学
- 允许打包，但必须先发布候选

## 标题属性解析

TTG 的质量信息需从标题正则解析。

### 默认值
codec=x264, audio=DTS, resolution=1080p, medium=Encode, team=other

### 编码检测
| 匹配 | 值 |
|------|-----|
| x265/HEVC | x265 |
| 默认 | x264 |

### 音频检测
| 匹配 | 值 |
|------|-----|
| Atmos | TrueHD Atmos |
| TrueHD | TrueHD |
| DD5.1/AC3 | AC3 |
| 默认 | DTS |

### 分辨率检测
| 匹配 | 值 |
|------|-----|
| 2160p/4K | 2160p |
| 1080i | 1080i |
| 720p | 720p |
| 默认 | 1080p |

### 媒介检测
| 匹配 | 值 |
|------|-----|
| HDTV | HDTV |
| WEB-DL/WEB | WEB-DL |
| BluRay + 编码关键词 | Encode |
| BluRay（无编码关键词） | BluRay |
| REMUX | REMUX |
| 默认 | Encode |

### 制作组检测
| 匹配 | 值 |
|------|-----|
| WiKi | WiKi |
| NGB | NGB |
| ARiN | ARiN |
| TTG | TTG |
| 默认 | other |

### TTG→HDA 分类映射（参考 hdapt_auto_transfer）
| TTG 分类 | HDA 分类 |
|----------|---------|
| UHD原盘 / 影视2160p | Movie UHD-4K (300) |
| BluRay原盘 | Movies Blu-ray (401) |
| 电影1080i/p | Movies 1080p (410) |
| 电影720p | Movies 720p (411) |
| 电影DVDRip | Movies DVDRip (414) |
| 欧美剧*/大陆港台剧*/日剧/韩剧/剧包 | TV SERIES (402) |
| 纪录片* | Documentaries (404) |
| MV&演唱会 | Music Videos (406) |
| (电影原声&Game)OST/无损音乐FLAC&APE/补充音轨 | HQ Audio (408) |
| 高清体育节目 | SPORTS (407) |
| 高清动漫/动漫原盘 | Animations (405) |
| *综艺 | TV SHOWS (403) |
| iPhone/iPad视频 | Movies iPad (417) |
| MiniVideo | Misc (409) |

## 文件名清理

TTG 种子文件名需清理：
1. 去除 `[TTG]` 前缀：`re.sub(r'^\[TTG\]\s*', '', filename, flags=re.I)`
2. 去除中文字符及之后内容：`re.sub(r'[\u4e00-\u9fa5]+.*\.torrent$', '.torrent', filename)`

## examples/hdapt_auto_transfer 项目分析

### 项目架构
TTG（+ M-Team） → 下载 → MediaInfo 分析 → 截图 → HDA 发布的全自动流水线。

### 核心模块

| 模块 | 文件 | 职责 |
|------|------|------|
| 爬虫 | `modules/crawler.py` | TTG 种子列表抓取、标题解析、分类映射 |
| 媒体处理 | `modules/processor.py` | MediaInfo 提取（编码/分辨率/音频）、截图生成 |
| 元数据 | `modules/metadata.py` | IMDb/豆瓣简介获取 |
| 图片托管 | `modules/imghost.py` | PixHost 截图上传 → BBCode |
| 发布 | `modules/uploader.py` | HDArea NexusPHP 表单提交 |
| 主控 | `main.py` | 全流程编排、qB 管理、状态持久化 |

### TTG 数据采集方式（crawler.py）

**方式一：browse 页面解析**（`_fetch_url_torrents`）
- 解析 `#torrent_table` 表格
- 从 `<a href="/t/{id}/">` 提取种子 ID 和标题
- 从首列 `<img alt="电影1080i/p">` 提取分类名
- 通过正则从行 HTML 提取 IMDb ID
- TTL 识别（"X 小时"/"X 天"/"X 分钟"）
- 过滤 ID < 500000 的条目（排除侧边栏热搜）
- 从末尾列倒序解析体积（GB/MB/TB）

**方式二：RSS 解析**（`_fetch_rss_torrents`）
- 解析 RSS `<item>` 中的 `<title>`/`<link>`/`<enclosure>`
- 访问详情页补充分类和 IMDb/豆瓣 ID
- TTG RSS bug 修复：`{@}` → `.`；`5 1-` → `5.1-`

### 质量推断策略
项目采用**两阶段推断**：
1. **爬虫阶段**：`parse_title_attributes()` 从标题正则推断（codec/audio/resolution/medium/team）
2. **处理阶段**：`processor.parse_media_attributes()` 从 MediaInfo 精确识别覆盖

**分辨率以标题标注为准**（MediaInfo 分辨率被注释掉），因为有些压制组切掉黑边后物理分辨率不再是标准数值。

### 禁转排除现状

**`hdapt_auto_transfer` 项目完全没有实现禁转检测。**

在代码库中搜索 `禁转`/`nodistr`/`exclusive`/`forbidden` 返回 **0 结果**。项目会无差别地尝试转发所有 TTG 种子（包括禁转资源），存在严重违规风险。

### PT-Forward 需要实现的禁转检测方案

根据采集到的 TTG browse 页面真实数据（2026-04-23），禁转种子的 HTML 结构为：

```html
<!-- 正常种子 -->
<a href="/t/811032/"><b>My Beautiful Man Eternal 2023 1080p...</b></a>

<!-- 禁转种子（如 ID=810773, ID=810759） -->
<b>禁转</b><a href="/t/810773/"><b>The Super Mario Bros Movie 2023 BluRay...</b></a>
```

**推荐检测策略**：
1. **browse 层快速过滤**：解析 `<a href="/t/{id}/">` 的前一个兄弟节点，检查是否含 `<b>禁转</b>` 文字
2. **details 层精确确认**：检查详情页是否包含"本种子是禁转资源"文字
3. **双保险**：两个层面都检测，browse 层优先（减少 HTTP 请求）

## 特殊说明

1. **非标准 NP 框架**：TTG 是自研框架，表单结构与 NexusPHP 完全不同
2. **分类即质量**：58 个分类已包含分辨率+内容类型，无独立质量字段
3. **team 字段语义异常**：team 为 hidden 字段，实际制作组从标题后缀解析（如 `-WiKi`、`-NGB`、`-ARiN`）
4. **无标签系统**："禁转"通过 browse 页标题前 `<b>禁转</b>` 文字识别，非标签
5. **剧集按地区+分辨率细分**：欧美剧/日剧/韩剧/华语剧各有独立分类，分单集和包
6. **游戏分类极多**：PC/MAC/XBOX360/XBOX1/PS2/PS3/PS4/PSP/PSV/SWITCH/NDS/WIIU/WII/NGC 等
7. **Cloudflare 保护**：需要 cf_clearance cookie
8. **种子文件名含 [TTG] 前缀**：需清理后再发到目标站
9. **hr 字段**：有 H&R 标记，上传者可选
10. **官方制作组**：WiKi / NGB / DoA / TTG原盘发布DIY 为官方小组，必须尊重
11. **盒子限速 100MB/s**：单种上传限速，全局不限
12. **盒子 72h 三倍流量上限**：发布 72 小时内盒子最多获得种子体积 3 倍上传流量
13. **电影禁止打包**：严格的打包限制，电影类完全禁止任何形式打包
14. **评论规则**：禁止在评论中发表打击上传者积极性的言论（求Free/等XX版本/HR就不下等）
