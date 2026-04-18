# 龙之家 站点适配器设计

> DragonHD 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 龙之家|
| 站点地址 | https://www.dragonhd.xyz |
| 站点框架 | NexusPHP |
| 特殊规则 | 繁体中文界面、AV 分类、2K/1440p 分辨率、无标签系统、无 source_sel/processing_sel/PT-Gen/豆瓣 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://www.dragonhd.xyz/announce.php` |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 缺失字段

以下常见字段在 DragonHD **不存在**：

- **`source_sel`**：无来源/地区字段
- **`processing_sel`**：无处理方式字段
- **`pt_gen`**：无 PT-Gen 字段
- **`douban_id`**：无豆瓣字段
- **`technical_info`**：无 MediaInfo 独立字段
- **`offers`**：无候选发布
- **标签系统**：完全无标签（tags）功能

### 1.3 类型（`type`）— 11 个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 411 | 剧集 |
| 412 | 游戏 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 403 | 综艺 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音乐 |
| 410 | AV |
| 409 | 其他 |

**特点**：
- 有 **AV**（410）分类（成人内容）
- 有 **游戏**（412）分类
- 剧集使用 411（非标准 402）
- 无 data-mode 属性，质量字段不带后缀

### 1.4 媒介（`medium_sel`）— 9 个

| 值 | 显示名称 |
|----|----------|
| 9 | UHD |
| 1 | Blu-ray |
| 10 | Remux |
| 11 | Encode |
| 6 | WEB-DL |
| 3 | DVD |
| 4 | HDTV |
| 7 | CD/SACD |
| 8 | Other |

**特点**：CD 和 SACD 合并为一个选项，媒介列表简洁。

### 1.5 编码（`codec_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264(AVC) |
| 2 | H.265(HEVC) |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 23 | VP9 |
| 21 | Other |

**特点**：不区分 x264/H.264，有 VP9。值 23 和 21 编号较大，可能为后续添加。

### 1.6 音频编码（`audiocodec_sel`）— 15 个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS-X |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | DD/DD+/AC3 |
| 10 | LPCM |
| 11 | DTS-HD MA |
| 12 | WAV |
| 14 | Other |
| 15 | TrueHD Atmos |
| 16 | TrueHD |
| 17 | DTS-HD HR |
| 18 | DTS |

**特点**：DD/DD+/AC3 合并为一个选项（值=7），DTS 细分 4 级（DTS/DTS-HD HR/DTS-HD MA/DTS-X）。

### 1.7 分辨率（`standard_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | 8K/4320p |
| 3 | 4K/2160p |
| 4 | 2K/1440p |
| 5 | 1080p/1080i |
| 6 | 720p |
| 7 | SD |

**特点**：
- 有 **2K/1440p** 分辨率（少数站有此选项）
- 1080p/1080i 合并
- 值编号从 1 开始但无 2（1→3→4→5→6→7）
- 无 "Other" 选项

### 1.8 制作组（`team_sel`）— 9 个

| 值 | 显示名称 |
|----|----------|
| 9 | DragonHD |
| 1 | HDS |
| 2 | CHD |
| 4 | WiKi |
| 5 | Beitai |
| 6 | FRDS |
| 7 | CMCT |
| 10 | LeagueHD |
| 8 | Other |

**特点**：含站方组 DragonHD 和 LeagueHD，其余为老牌制作组。

---

## 二、关键适配器设计要点

### 2.1 无标签系统

DragonHD 完全没有标签（tags）功能，是目前分析过唯一没有标签的站点。

### 2.2 极简字段

仅有 5 个质量字段（medium/codec/audiocodec/standard/team），无 source_sel、processing_sel。字段名不带 data-mode 后缀。

### 2.3 AV 分类

有独立的 AV（410）分类，适配器在转发时需注意内容分类的合法性。

### 2.4 2K/1440p 分辨率

分辨率先项含 2K/1440p，适配器需支持此分辨率的映射。

### 2.5 繁体中文界面

站点界面使用繁体中文，但字段名仍为英文（NexusPHP 标准）。

### 2.6 无 PT-Gen/豆瓣

没有 PT-Gen 和豆瓣独立字段，影视信息只能通过 IMDb 链接或简介手动填写。

---

## 三、发布字段与通用模型的映射

### 3.1 类型映射（type）

| 通用类型 | DragonHD type 值 |
|---------|-----------------|
| 电影 | 401 |
| 剧集 | 411 |
| 综艺 | 403 |
| 纪录片 | 404 |
| 动漫 | 405 |
| MV | 406 |
| 体育 | 407 |
| 音乐 | 408 |
| 其他 | 409 |
| AV | 410 |
| 游戏 | 412 |

### 3.2 媒介映射（medium_sel）

| 通用媒介 | DragonHD medium_sel 值 |
|---------|----------------------|
| UHD | 9 |
| Blu-ray | 1 |
| Remux | 10 |
| Encode | 11 |
| WEB-DL | 6 |
| DVD | 3 |
| HDTV | 4 |
| CD/SACD | 7 |
| Other | 8 |

### 3.3 编码映射（codec_sel）

| 通用编码 | DragonHD codec_sel 值 |
|---------|----------------------|
| H.264(AVC) | 1 |
| H.265(HEVC) | 2 |
| VC-1 | 3 |
| MPEG-2 | 4 |
| VP9 | 23 |
| Other | 21 |

### 3.4 音频编码映射（audiocodec_sel）

| 通用音频编码 | DragonHD audiocodec_sel 值 |
|-------------|--------------------------|
| FLAC | 1 |
| APE | 2 |
| DTS:X | 3 |
| MP3 | 4 |
| OGG | 5 |
| AAC | 6 |
| DD/DD+/AC3 | 7 |
| LPCM | 10 |
| DTS-HD MA | 11 |
| WAV | 12 |
| Other | 14 |
| TrueHD Atmos | 15 |
| TrueHD | 16 |
| DTS-HD HR | 17 |
| DTS | 18 |

### 3.5 分辨率映射（standard_sel）

| 通用分辨率 | DragonHD standard_sel 值 |
|-----------|-------------------------|
| 8K/4320p | 1 |
| 4K/2160p | 3 |
| 2K/1440p | 4 |
| 1080p/1080i | 5 |
| 720p | 6 |
| SD | 7 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
