# 库非 站点适配器设计

> Kufei 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 库非|
| 站点地址 | https://kufei.org |
| 站点框架 | NexusPHP |
| 特殊规则 | Cloudflare 防护、游戏/电子书/软件分类、媒介细分压制/原盘/DIY、22 音频编码（含 DSD/TTA/MPEG） |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://kufei.org/announce.php` |
| PT-Gen | 支持 imdb/douban/bangumi/indienova |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `pt_gen` | text | - | PT-Gen 链接（支持 imdb/douban/bangumi/indienova） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

**特点**：
- PT-Gen 字段支持 **4 种来源**：imdb、douban、bangumi、indienova
- 无独立 IMDb 链接字段（通过 PT-Gen 整合）
- 无 MediaInfo/BDInfo 独立字段
- 无豆瓣独立字段（通过 PT-Gen）

### 1.2 缺失字段

- **`source_sel`**：无来源/地区字段
- **`processing_sel`**：无处理方式字段
- **`offers`**：无候选发布选项
- **`technical_info`**：无 MediaInfo 独立字段

### 1.3 类型（`type`）— 16 个（data-mode='4'）

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/纪录片 |
| 405 | Animations/动画、动漫 |
| 406 | Music Videos/音乐、视频 |
| 407 | Sports/体育 |
| 408 | Music/音乐 |
| 409 | Others/其他 |
| 410 | Games/游戏 |
| 411 | E-Books/电子书 |
| 412 | Software/软件 |
| 413 | Education/教育 |
| 414 | Concern/演唱会 |
| 415 | Drama/戏剧 |
| 416 | Audio Books/有声读物 |

**特点**：
- **16 个分类**，是目前分析过的 NexusPHP 站中最多的之一
- 含游戏（410）、电子书（411）、软件（412）、教育（413）、有声读物（416）等非常规分类
- 演唱会单独分类（414），非合并到音乐

### 1.4 媒介（`medium_sel[4]`）— 17 个

| 值 | 显示名称 |
|----|----------|
| 1 | UHD原盘 |
| 2 | UHD DIY |
| 3 | UHD Remux |
| 4 | BD 原盘 |
| 5 | BD DIY |
| 6 | BD Remux |
| 7 | UHD 压制 |
| 8 | 1080P/i 压制 |
| 9 | 720P 压制 |
| 10 | MiniSD |
| 11 | WEB-DL |
| 12 | HDTV |
| 13 | DVD |
| 14 | CD |
| 15 | SACD |
| 16 | Others |
| 18 | Encode |

**特点**：
- **17 个媒介**，细分程度极高
- UHD 和 BD 都区分原盘/DIY/Remux/压制
- 有 MiniSD（极少见）、SACD
- 值 17 缺失（15→16→18），Encode=18

### 1.5 编码（`codec_sel[4]`）— 10 个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 2 | X264 |
| 3 | H.265/HEVC |
| 4 | X265 |
| 5 | VC-1 |
| 6 | MPEG-2 |
| 7 | MPEG-4 |
| 8 | Xvid |
| 9 | AV1 |
| 10 | Othe |

**特点**：区分硬件编码（H.264/AVC vs H.265/HEVC）和软件编码（X264 vs X265），支持 AV1。

### 1.6 音频编码（`audiocodec_sel[4]`）— 22 个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 4 | MP3 |
| 5 | OGG |
| 7 | Other |
| 8 | TrueHD Atmos |
| 9 | DTS |
| 10 | DTS X |
| 11 | DTS-HDMA |
| 12 | DTS-HD HR |
| 13 | True-HD |
| 14 | LPCM |
| 15 | DDP/DD+ |
| 16 | Dolby Digital/DD |
| 17 | AC3 |
| 18 | AAC |
| 19 | WAV |
| 20 | DSD |
| 21 | OGG |
| 22 | TTA |
| 23 | MPEG |
| 24 | DDP Atmos |

**特点**：
- **22 个音频编码**，是所有已分析站点中最多的之一
- 含 **DSD**（DSD 音频）、**TTA**（无损压缩）、**MPEG** 音频
- OGG 出现两次（值=5 和值=21）
- DDP 细分为 DDP/DD+（15）和 DDP Atmos（24）
- Dolby Digital/DD（16）和 AC3（17）同时存在

### 1.7 分辨率（`standard_sel[4]`）— 7 个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 8K/4320P |
| 6 | 4K/UHD/2160P |
| 7 | Other |

**特点**：1080p 和 1080i 独立分开（不合并），有 8K 和 4K 分别。

### 1.8 制作组（`team_sel[4]`）— 5 个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |

**极简**：仅 5 个制作组，全部为老牌制作组（HDS、CHD、MySiLU、WiKi）。

### 1.9 标签（`tags[4][]`）— 12 个 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 9 | 动漫 |
| 2 | 首发 |
| 4 | DIY |
| 8 | 英语 |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 14 | 游戏 |
| 13 | 应求 |
| 12 | 完结 |
| 11 | 书籍 |

**特点**：含"动漫"、"游戏"、"书籍"等内容类型标签，与其他站的语言/质量标签不同。

---

## 二、关键适配器设计要点

### 2.1 Cloudflare 防护

站点使用 Cloudflare（需 `cf_clearance` cookie），适配器需支持 Cloudflare cookie 刷新机制。

### 2.2 分类覆盖面广

16 个分类涵盖影视、音乐、游戏、电子书、软件、教育等，是综合性较强的站点。

### 2.3 媒介细分

17 个媒介，UHD/BD 各分原盘/DIY/Remux/压制四级，适配器需准确匹配。

### 2.4 音频编码特殊值

- OGG 出现两次（值=5 和 值=21），需确认是否有含义差异
- DSD/TTA/MPEG 为少见音频格式
- DDP Atmos（24）与 TrueHD Atmos（8）并存

### 2.5 PT-Gen 多源支持

PT-Gen 字段支持 4 种来源（imdb/douban/bangumi/indienova），适配器可灵活选择。

---

## 三、发布字段与通用模型的映射

### 3.1 类型映射（type）

| 通用类型 | Kufei type 值 |
|---------|--------------|
| 电影 | 401 |
| 电视剧 | 402 |
| 综艺 | 403 |
| 纪录 | 404 |
| 动漫 | 405 |
| 音乐视频 | 406 |
| 体育 | 407 |
| 音乐 | 408 |
| 其他 | 409 |
| 游戏 | 410 |
| 电子书 | 411 |
| 软件 | 412 |
| 教育 | 413 |
| 演唱会 | 414 |
| 戏剧 | 415 |
| 有声读物 | 416 |

### 3.2 媒介映射（medium_sel）

| 通用媒介 | Kufei medium_sel 值 |
|---------|---------------------|
| UHD 原盘 | 1 |
| UHD DIY | 2 |
| UHD Remux | 3 |
| BD 原盘 | 4 |
| BD DIY | 5 |
| BD Remux | 6 |
| UHD 压制 | 7 |
| 1080P/i 压制 | 8 |
| 720P 压制 | 9 |
| MiniSD | 10 |
| WEB-DL | 11 |
| HDTV | 12 |
| DVD | 13 |
| CD | 14 |
| SACD | 15 |
| Others | 16 |
| Encode | 18 |

### 3.3 编码映射（codec_sel）

| 通用编码 | Kufei codec_sel 值 |
|---------|---------------------|
| H.264/AVC | 1 |
| x264 | 2 |
| H.265/HEVC | 3 |
| x265 | 4 |
| VC-1 | 5 |
| MPEG-2 | 6 |
| MPEG-4 | 7 |
| Xvid | 8 |
| AV1 | 9 |
| Other | 10 |

### 3.4 音频编码映射（audiocodec_sel）

| 通用音频编码 | Kufei audiocodec_sel 值 |
|-------------|------------------------|
| FLAC | 1 |
| APE | 2 |
| MP3 | 4 |
| OGG | 5（或 21） |
| Other | 7 |
| TrueHD Atmos | 8 |
| DTS | 9 |
| DTS:X | 10 |
| DTS-HDMA | 11 |
| DTS-HD HR | 12 |
| TrueHD | 13 |
| LPCM | 14 |
| DDP/DD+ | 15 |
| Dolby Digital | 16 |
| AC3 | 17 |
| AAC | 18 |
| WAV | 19 |
| DSD | 20 |
| TTA | 22 |
| MPEG Audio | 23 |
| DDP Atmos | 24 |

### 3.5 分辨率映射（standard_sel）

| 通用分辨率 | Kufei standard_sel 值 |
|-----------|----------------------|
| 1080p | 1 |
| 1080i | 2 |
| 720p | 3 |
| SD | 4 |
| 8K/4320P | 5 |
| 4K/2160P | 6 |
| Other | 7 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
