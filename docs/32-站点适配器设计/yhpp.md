# 昆仑 站点适配器设计

> YHPP 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 昆仑|
| 站点地址 | https://yhpp.cc |
| 站点框架 | NexusPHP |
| 特殊规则 | processing_sel = 地区（非处理方式）、19 媒介（含黑胶/CD+VCD/CD+DVD/SACD）、23 音频编码、29 制作组、19 标签 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://www.yhpp.cc/announce.php` |
| PT-Gen | 支持 |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `douban_id` | text | - | 豆瓣 ID/链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |
| `offers` | checkbox | - | 候选发布 |

### 1.2 缺失字段

- **`source_sel`**：无来源字段

### 1.3 类型（`type`）— 9 个（data-mode='4'）

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/纪录片 |
| 405 | Animations/动漫、动画 |
| 406 | Music Videos/音乐视频 |
| 407 | Sports/体育 |
| 408 | Music/音乐 |
| 409 | Others/其他 |

### 1.4 媒介（`medium_sel[4]`）— 19 个

| 值 | 显示名称 |
|----|----------|
| 19 | UHD原盘 |
| 18 | UHD DIY |
| 17 | UHD Remux |
| 16 | UHD压制 |
| 15 | BD原盘 |
| 14 | BD DIY |
| 13 | BD Remux |
| 12 | 1080P/i压制 |
| 11 | 720P压制 |
| 10 | MiniSD |
| 9 | WEB-DL |
| 8 | HDTV |
| 6 | DVD |
| 5 | CD |
| 4 | SACD |
| 7 | CD+VCD |
| 3 | CD+DVD |
| 2 | 黑胶 |
| 1 | Other/其他 |

**特点**：
- **19 个媒介**，目前分析站点中最多的
- UHD/BD 各四级（原盘/DIY/Remux/压制）
- 含 **黑胶**（Vinyl）、**CD+VCD**、**CD+DVD**、**SACD** 等音乐相关媒介

### 1.5 编码（`codec_sel[4]`）— 10 个

| 值 | 显示名称 |
|----|----------|
| 1 | MPEG-2 |
| 2 | MPEG-4 |
| 3 | Xvid |
| 4 | AV1 |
| 5 | Other/其他 |
| 6 | VC-1 |
| 7 | x265 |
| 8 | H.265/HEVC |
| 9 | x264 |
| 10 | H.264/AVC |

**特点**：区分硬件编码和软件编码（x264 vs H.264/AVC），支持 AV1。

### 1.6 音频编码（`audiocodec_sel[4]`）— 23 个

| 值 | 显示名称 |
|----|----------|
| 1 | AAC |
| 2 | AC3 |
| 3 | TTA |
| 4 | MP3 |
| 5 | ALAC |
| 6 | m4a |
| 7 | Other/其他 |
| 8 | OGG |
| 9 | MPEG |
| 10 | Dolby Digital/DD |
| 11 | DDP/DD+/EAC3 |
| 12 | DDP Atmos |
| 13 | DSD |
| 14 | FLAC |
| 15 | APE |
| 16 | WAV |
| 17 | LPCM |
| 18 | DTS |
| 19 | DTS-HD HR |
| 20 | DTS-HDMA |
| 21 | True-HD |
| 22 | DTS:X |
| 23 | TrueHD Atmos |

**特点**：
- **23 个音频编码**，目前分析站点中最多的
- 含 **ALAC**、**m4a**、**DSD**、**TTA** 等少见格式
- DTS 细分 4 级（DTS/DTS-HD HR/DTS-HDMA/DTS:X）
- Dolby 细分 3 级（DD/DD+/DDP Atmos）
- TrueHD 细分 2 级（True-HD/TrueHD Atmos）

### 1.7 分辨率（`standard_sel[4]`）— 7 个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | Other/其他 |
| 6 | 4K/UHD/2160P |
| 7 | 8K/4320P |

### 1.8 地区（`processing_sel[4]`）— 12 个

| 值 | 显示名称 |
|----|----------|
| 1 | MY/马来西亚 |
| 2 | Other/其他 |
| 3 | SG/新加坡 |
| 4 | IN/印度 |
| 5 | KR/韩国 |
| 6 | JP/日本 |
| 7 | UK/英国 |
| 8 | EU/欧洲 |
| 9 | US/美国 |
| 10 | TW/台湾 |
| 11 | HK/香港 |
| 12 | CN/中国大陆 |

**重要**：`processing_sel` 在本站表示 **地区**，而非通常的处理方式（Raw/Encode）。这是继 PTCafe（source_sel=地区）之后又一个地区字段命名特殊的站点。

### 1.9 制作组（`team_sel[4]`）— 29 个

| 值 | 显示名称 |
|----|----------|
| 1 | DIC |
| 2 | Red |
| 3 | GGN |
| 4 | LemonHD |
| 5 | Other/其他 |
| 6 | OpenCD |
| 7 | FraMeSToR |
| 8 | EPSiLON |
| 9 | BTN/NTb |
| 10 | PTP |
| 11 | TLF |
| 12 | Hares |
| 13 | QHstudIo |
| 14 | PTHome |
| 15 | HDHome |
| 16 | HHClub |
| 17 | Ubits |
| 18 | Audies |
| 19 | PTer |
| 20 | OurBits |
| 21 | HDS |
| 22 | FRDS |
| 23 | CMCT |
| 24 | beAst |
| 25 | WiKi |
| 26 | TTG |
| 27 | HDC |
| 28 | CHDBits |
| 29 | HDFans |

**特点**：29 个制作组，覆盖国内主要 PT 站和国外站点（PTP/BTN/FraMeSToR/EPSiLON/GGN/Red）。

### 1.10 标签（`tags[4][]`）— 19 个 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 微光星辰 |
| 9 | 原创 |
| 10 | 源站转发 |
| 11 | 中英双语 |
| 12 | 特效 |
| 13 | Dolby Vision |
| 14 | Atmos |
| 15 | 4K |
| 16 | 8K |
| 17 | Hi-Res |
| 18 | 完结 |
| 19 | 刮削 |
| 20 | AI修复 |
| 21 | 保种 |

**特点**：
- **19 个标签**，含 HDR 细分（HDR/Dolby Vision/Atmos）、分辨率标签（4K/8K）
- 含 **微光星辰**（类似 HDFans 的特别标签）、**源站转发**、**Hi-Res**
- 含 **AI修复**、**刮削**、**保种** 等功能性标签

---

## 二、关键适配器设计要点

### 2.1 processing_sel = 地区

`processing_sel` 在通常 NexusPHP 站中表示处理方式（Raw/Encode），但在 YHPP 中表示 **地区**（CN/HK/TW/US/JP 等）。适配器需按字段实际含义映射，不能按字段名假设。

### 2.2 媒介最全

19 个媒介是目前所有分析站点中最多的，UHD/BD 各四级 + 音乐媒介（黑胶/CD+VCD/CD+DVD/SACD）。

### 2.3 音频编码最全

23 个音频编码是目前最多的，含 ALAC/m4a/DSD/TTA 等少见格式。

### 2.4 标签丰富

19 个标签含 HDR 三级细分（HDR/DV/Atmos）和分辨率标签（4K/8K），适配器可精细标注。

---

## 三、发布字段与通用模型的映射

### 3.1 媒介映射（medium_sel）

| 通用媒介 | YHPP medium_sel 值 |
|---------|-------------------|
| UHD 原盘 | 19 |
| UHD DIY | 18 |
| UHD Remux | 17 |
| UHD 压制 | 16 |
| BD 原盘 | 15 |
| BD DIY | 14 |
| BD Remux | 13 |
| 1080P/i 压制 | 12 |
| 720P 压制 | 11 |
| MiniSD | 10 |
| WEB-DL | 9 |
| HDTV | 8 |
| DVD | 6 |
| CD | 5 |
| SACD | 4 |
| CD+VCD | 7 |
| CD+DVD | 3 |
| 黑胶/Vinyl | 2 |
| Other | 1 |

### 3.2 编码映射（codec_sel）

| 通用编码 | YHPP codec_sel 值 |
|---------|-------------------|
| MPEG-2 | 1 |
| MPEG-4 | 2 |
| Xvid | 3 |
| AV1 | 4 |
| Other | 5 |
| VC-1 | 6 |
| x265 | 7 |
| H.265/HEVC | 8 |
| x264 | 9 |
| H.264/AVC | 10 |

### 3.3 音频编码映射（audiocodec_sel）

| 通用音频编码 | YHPP audiocodec_sel 值 |
|-------------|----------------------|
| AAC | 1 |
| AC3 | 2 |
| TTA | 3 |
| MP3 | 4 |
| ALAC | 5 |
| m4a | 6 |
| Other | 7 |
| OGG | 8 |
| MPEG Audio | 9 |
| Dolby Digital | 10 |
| DDP/DD+ | 11 |
| DDP Atmos | 12 |
| DSD | 13 |
| FLAC | 14 |
| APE | 15 |
| WAV | 16 |
| LPCM | 17 |
| DTS | 18 |
| DTS-HD HR | 19 |
| DTS-HDMA | 20 |
| TrueHD | 21 |
| DTS:X | 22 |
| TrueHD Atmos | 23 |

### 3.4 分辨率映射（standard_sel）

| 通用分辨率 | YHPP standard_sel 值 |
|-----------|---------------------|
| 1080p | 1 |
| 1080i | 2 |
| 720p | 3 |
| SD | 4 |
| Other | 5 |
| 4K/2160P | 6 |
| 8K/4320P | 7 |

### 3.5 地区映射（processing_sel — 实际为地区）

| 通用地区 | YHPP processing_sel 值 |
|---------|----------------------|
| 马来西亚 | 1 |
| Other | 2 |
| 新加坡 | 3 |
| 印度 | 4 |
| 韩国 | 5 |
| 日本 | 6 |
| 英国 | 7 |
| 欧洲 | 8 |
| 美国 | 9 |
| 台湾 | 10 |
| 香港 | 11 |
| 中国大陆 | 12 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
