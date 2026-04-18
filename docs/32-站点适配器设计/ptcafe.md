# 咖啡 站点适配器设计

> PTCafe 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 咖啡|
| 站点地址 | https://ptcafe.club |
| 站点框架 | NexusPHP |
| 特殊规则 | source_sel 为地区（非来源）、31 个制作组、18 个音频编码（含 OPUS/OGG）、data-mode='4' |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://tracker.ptcafe.club/announce.php` |
| 建站时间 | 2023 年 |

---

## 一、发种规范

### 1.1 基本规则

- 已完结电视剧，请直接发布**合集**，并勾选【完结】标签，不得拆分发布
- 已经发布的分集资源，请及时跟进到完结，不得烂尾
- 转载的资源，请保持原目录结构发布，以便于自动化辅种
- 不要发布电子书资源，本站主打影视资源

---

## 二、发布页面表单字段分析

### 2.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（不填则使用种子文件名） |
| `small_descr` | text | - | 副标题 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode，支持预览） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |

**特点**：
- 有 MediaInfo/BDInfo 独立字段 `technical_info`
- 有预览功能（BBCode 编辑器带预览按钮）
- 有附件上传功能（`attachment.php` iframe）
- 有"填写质量"快捷按钮（自动从标题解析质量信息）

### 2.2 类型（`type`）— 9 个（data-mode='4'）

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 纪录 |
| 405 | 动漫 |
| 406 | 演唱 |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | 其他 |

**注意**：类型使用标准 4xx 编号（401-409），与 HDFans 等站类似。`data-mode='4'` 表示所有质量字段带 `[4]` 后缀。

### 2.3 来源/地区（`source_sel[4]`）— 7 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | 大陆 | |
| 2 | 港台 | |
| 3 | 欧美 | |
| 4 | 日本 | |
| 5 | 韩国 | |
| 6 | 印度 | |
| 7 | 其他 | |

**重要**：`source_sel` 在本站表示 **地区**，而非通常的来源媒介（Blu-ray/HDTV 等）。这是少见的命名方式。

### 2.4 媒介（`medium_sel[4]`）— 13 个

| 值 | 显示名称 |
|----|----------|
| 1 | UHD Blu-ray 原盘 |
| 2 | UHD Blu-ray DIY |
| 3 | UHD Remux |
| 4 | Blu-ray 原盘 |
| 5 | Blu-ray DIY |
| 6 | Remux |
| 7 | Encode |
| 8 | WEB-DL |
| 9 | TV |
| 10 | DVD |
| 11 | CD |
| 12 | Track |
| 13 | Other |

**特点**：UHD 和 FHD Blu-ray 都区分原盘/DIY 两种，媒介细分程度较高。

### 2.5 编码（`codec_sel[4]`）— 11 个

| 值 | 显示名称 |
|----|----------|
| 1 | H.265/HEVC |
| 2 | H.264/AVC |
| 3 | X265 |
| 4 | X264 |
| 5 | VC-1 |
| 6 | MPEG-2 |
| 7 | MPEG-4 |
| 8 | XVID |
| 9 | VP9 |
| 10 | DIVX |
| 11 | Other |

**特点**：区分硬件编码（H.265/HEVC）和软件编码（X265），与 HDFans 类似。

### 2.6 音频编码（`audiocodec_sel[4]`）— 18 个

| 值 | 显示名称 |
|----|----------|
| 1 | DTS-HDMA:X 7.1 |
| 2 | DTS-HDMA |
| 3 | DTS-HDHR |
| 4 | DTS-HD |
| 5 | DTS-X |
| 6 | LPCM |
| 7 | AC3 |
| 8 | Atmos |
| 9 | AAC |
| 10 | TrueHD |
| 11 | DTS |
| 12 | FLAC |
| 13 | APE |
| 14 | MP3 |
| 15 | WAV |
| 16 | OPUS |
| 17 | OGG |
| 18 | Other |

**特点**：含 **OPUS** 和 **OGG**，在 PT 站中较少见。DTS 细分 5 级（DTS-HD / DTS-HDHR / DTS-HDMA / DTS-HDMA:X 7.1 / DTS-X）。

### 2.7 分辨率（`standard_sel[4]`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | 4320P/8K/FUHD |
| 2 | 2160P/4K/UHD |
| 3 | 1080p/1080i/FHD |
| 4 | 720p/720i/HD |
| 5 | 360p/360i/SD |
| 6 | Other |

**特点**：
- 1080p 和 1080i 合并为一个选项（1080p/1080i/FHD）
- 有 8K/FUHD 选项
- 有 360p（非标准 480p）SD 选项
- 分辨率名称含多重别名（4320P/8K/FUHD）

### 2.8 制作组（`team_sel[4]`）— 30 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | ADE | |
| 2 | ADWeb | |
| 3 | Audies | |
| 4 | beAst | |
| 5 | BeiTai | |
| 6 | BeyondHD | |
| 7 | BtsTV | |
| 8 | CafeTV | 站方 TV 组 |
| 9 | CafeWEB | 站方 WEB 组 |
| 10 | CHDBits | |
| 11 | CHDWEB | |
| 12 | CMCT | |
| 13 | DJWEB | |
| 14 | FRDS | |
| 15 | HDCTV | |
| 16 | HDH | |
| 17 | HDHome | |
| 18 | HDSky | |
| 19 | HDSWEB | |
| 20 | HHWEB | |
| 21 | MTeam | |
| 22 | MWeb | |
| 23 | OurBits | |
| 24 | OurTV | |
| 25 | PTCafe | 站方主组 |
| 26 | PTerWEB | |
| 27 | QHstudIo | |
| 28 | TTG | |
| 29 | WiKi | |
| 30 | Other | |

**特点**：30 个制作组，涵盖国内主要 PT 站制作组。站方组包括 PTCafe、CafeTV、CafeWEB。

### 2.9 标签（`tags[4][]`）— 16 个 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 官方 |
| 2 | 首发 |
| 3 | 完结 |
| 4 | 原创 |
| 5 | 禁转 |
| 7 | 国语 |
| 8 | 粤语 |
| 9 | 中字 |
| 10 | 备胎 |
| 11 | 杜比视界 |
| 12 | HDR |
| 13 | DIY |
| 14 | 应求 |
| 15 | 高码高帧 |
| 16 | 月月 |

**注意**：缺少值 6，编号从 5 直接跳到 7。标签使用数字 ID 值（非字符串）。

### 2.10 其他字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `uplver` | checkbox | 匿名发布（value="yes"） |

### 2.11 缺失字段

- 无 IMDb/豆瓣链接字段
- 无 PT-Gen 字段
- 无匿名候选字段（无 offers）
- 无 `processing_sel`（处理方式）

---

## 三、关键适配器设计要点

### 3.1 source_sel 含义特殊

PTCafe 的 `source_sel` 表示 **地区**（大陆/港台/欧美/日本/韩国/印度/其他），而非其他站常见的 **来源**（Blu-ray/HDTV/WEB-DL 等）。适配器需注意字段语义差异。

### 3.2 data-mode='4' 字段后缀

所有质量字段带 `[4]` 后缀（如 `source_sel[4]`、`medium_sel[4]`），对应 `data-mode='4'`。提交时需使用带后缀的字段名。

### 3.3 已完结剧集要求合集

已完结电视剧禁止分集发布，必须发布合集并勾选【完结】标签。

### 3.4 分辨率合并

1080p 和 1080i 合并为 `standard_sel=3`（1080p/1080i/FHD），适配器无需区分这两种分辨率。

### 3.5 MediaInfo/BDInfo 独立字段

有 `technical_info` 独立字段用于 MediaInfo/BDInfo，而非嵌入简介中。

---

## 四、发布字段与通用模型的映射

### 4.1 类型映射（type）

| 通用类型 | PTCafe type 值 |
|---------|---------------|
| 电影 | 401 |
| 剧集 | 402 |
| 综艺 | 403 |
| 纪录 | 404 |
| 动漫 | 405 |
| 演唱 | 406 |
| 体育 | 407 |
| 音乐 | 408 |
| 其他 | 409 |

### 4.2 地区映射（source_sel）— 注意：非来源

| 通用地区 | PTCafe source_sel 值 |
|---------|---------------------|
| 大陆 | 1 |
| 港台 | 2 |
| 欧美 | 3 |
| 日本 | 4 |
| 韩国 | 5 |
| 印度 | 6 |
| 其他 | 7 |

### 4.3 媒介映射（medium_sel）

| 通用媒介 | PTCafe medium_sel 值 |
|---------|---------------------|
| UHD Blu-ray 原盘 | 1 |
| UHD Blu-ray DIY | 2 |
| UHD Remux | 3 |
| Blu-ray 原盘 | 4 |
| Blu-ray DIY | 5 |
| Remux | 6 |
| Encode | 7 |
| WEB-DL | 8 |
| TV | 9 |
| DVD | 10 |
| CD | 11 |
| Track | 12 |
| Other | 13 |

### 4.4 编码映射（codec_sel）

| 通用编码 | PTCafe codec_sel 值 |
|---------|---------------------|
| H.265/HEVC | 1 |
| H.264/AVC | 2 |
| x265 | 3 |
| x264 | 4 |
| VC-1 | 5 |
| MPEG-2 | 6 |
| MPEG-4 | 7 |
| XVID | 8 |
| VP9 | 9 |
| DIVX | 10 |
| Other | 11 |

### 4.5 音频编码映射（audiocodec_sel）

| 通用音频编码 | PTCafe audiocodec_sel 值 |
|-------------|-------------------------|
| DTS-HDMA:X 7.1 | 1 |
| DTS-HDMA | 2 |
| DTS-HDHR | 3 |
| DTS-HD | 4 |
| DTS-X | 5 |
| LPCM | 6 |
| AC3 | 7 |
| Atmos | 8 |
| AAC | 9 |
| TrueHD | 10 |
| DTS | 11 |
| FLAC | 12 |
| APE | 13 |
| MP3 | 14 |
| WAV | 15 |
| OPUS | 16 |
| OGG | 17 |
| Other | 18 |

### 4.6 分辨率映射（standard_sel）

| 通用分辨率 | PTCafe standard_sel 值 |
|-----------|----------------------|
| 4320p/8K | 1 |
| 2160p/4K | 2 |
| 1080p/1080i | 3 |
| 720p | 4 |
| SD/360p | 5 |
| Other | 6 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
