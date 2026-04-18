# 垃圾堆 站点适配器设计

> Lajidui 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 垃圾堆|
| 站点地址 | https://pt.lajidui.top |
| 站点框架 | NexusPHP |
| 特殊规则 | Cloudflare 防护、processing_sel = 文件格式（非处理方式）、少儿动画/短剧/游戏/APP/有声书分类、source_sel = 地区、2K 分辨率 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://pt.lajidui.top/announce.php` |

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

### 1.2 类型（`type`）— 16 个（data-mode='4'）

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/纪录片 |
| 405 | Animations/动漫 |
| 406 | Music Videos/音乐视频 |
| 407 | Sports/体育 |
| 408 | Audio/音频 |
| 409 | Misc/其他 |
| 410 | Cartoon/少儿动画 |
| 411 | Ebook/电子书 |
| 412 | ShortDrama/短剧 |
| 413 | Game/游戏 |
| 414 | APP/软件 |
| 415 | Education/教育视频 |
| 416 | Audiobook/有声书 |

**特点**：
- **16 个分类**，含少儿动画（410）、电子书（411）、短剧（412）、游戏（413）、APP（414）、教育视频（415）、有声书（416）
- 音频（408）非 Music，强调非音乐类音频

### 1.3 媒介（`medium_sel[4]`）— 11 个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | WEB-DL |
| 11 | Other |

**特点**：保留 HD DVD（已过时媒介），无 UHD 独立分类。

### 1.4 编码（`codec_sel[4]`）— 7 个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | AV1 |
| 7 | H.265 |

**特点**：不区分 x264/H.264 和 x265/H.265，支持 AV1。

### 1.5 音频编码（`audiocodec_sel[4]`）— 13 个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | WAV |
| 9 | DTS-HD |
| 10 | TrueHD |
| 11 | LPCM |
| 12 | E-AC-3 |
| 13 | AC-3 |

### 1.6 分辨率（`standard_sel[4]`）— 8 个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2k |
| 6 | 4k |
| 7 | 8k |
| 8 | Other |

**特点**：有 **2K** 分辨率选项（其他站少见）。1080p/1080i 分开。

### 1.7 地区（`source_sel[4]`）— 8 个

| 值 | 显示名称 |
|----|----------|
| 7 | 大陆 |
| 2 | 台湾 |
| 8 | 香港 |
| 10 | 日本 |
| 11 | 韩国 |
| 1 | 欧美 |
| 3 | 印度 |
| 6 | Other |

**重要**：`source_sel` 在本站表示 **地区**，非来源媒介。

### 1.8 文件格式（`processing_sel[4]`）— 16 个

| 值 | 显示名称 |
|----|----------|
| 1 | EPUB |
| 2 | PDF |
| 3 | TXT |
| 4 | DOCX |
| 5 | PPTX |
| 6 | XLSX |
| 7 | WPS |
| 8 | AZW3 |
| 9 | MOBI |
| 10 | MKV |
| 11 | MP4 |
| 12 | RAR |
| 13 | ZIP |
| 14 | 7z |
| 16 | ISO |
| 17 | Other |

**重要**：`processing_sel` 在本站表示 **文件格式/封装格式**，而非通常的处理方式（Raw/Encode）。含电子书格式（EPUB/PDF/TXT/DOCX/AZW3/MOBI）、视频容器（MKV/MP4）、压缩格式（RAR/ZIP/7z）、光盘镜像（ISO）。

### 1.9 制作组（`team_sel[4]`）— 22 个

| 值 | 显示名称 |
|----|----------|
| 1 | HDSky |
| 2 | CHD |
| 3 | 原创 |
| 4 | WiKi |
| 5 | Other |
| 6 | HHWEB |
| 7 | ADE |
| 8 | CMCT |
| 9 | FRDS |
| 10 | TJUPT |
| 11 | UBits |
| 12 | Ourbits |
| 13 | QHstudIo |
| 14 | HDHome |
| 15 | AGSVWEB |
| 16 | Pter |
| 17 | CatEDU |
| 18 | beAst |
| 19 | LHD |
| 20 | BMDru |
| 21 | BeiTai |
| 22 | GodDramas |

**特点**：含 **CatEDU**（教育专题制作组）、**GodDramas**（短剧制作组）、**TJUPT**（北洋园）。有"原创"（值=3）作为制作组。

### 1.10 标签（`tags[4][]`）— 17 个 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 已刮削 |
| 9 | 完结 |
| 10 | 杜比 |
| 11 | 粤语 |
| 12 | 单集 |
| 13 | 三级 |
| 14 | 英语 |
| 15 | 国英双语 |
| 16 | 简英双语字幕 |
| 17 | 多音轨 |

**特点**：含 **三级**（成人内容分级标签）、**单集**、**多音轨**、**已刮削** 等独特标签。

---

## 二、关键适配器设计要点

### 2.1 processing_sel = 文件格式

`processing_sel` 在通常 NexusPHP 站中表示处理方式，但在垃圾堆中表示 **文件格式/封装格式**（MKV/MP4/EPUB/PDF/RAR/ISO 等）。这是第三个字段语义特殊站（继 PTCafe 的 source_sel=地区、YHPP 的 processing_sel=地区之后）。

适配器需根据目标内容类型选择合适的 processing_sel 值：
- 视频资源 → MKV(10) 或 MP4(11)
- 电子书 → EPUB(1) / PDF(2) / TXT(3) 等
- 光盘镜像 → ISO(16)

### 2.2 source_sel = 地区

`source_sel` 表示地区（大陆/港台/日韩/欧美），非来源媒介。

### 2.3 分类覆盖面广

16 个分类涵盖影视、动漫、音乐、少儿、电子书、短剧、游戏、APP、教育、有声书等，综合性强。

### 2.4 2K 分辨率

有独立的 2K 分辨率选项（standard_sel=5），多数站无此选项。

### 2.5 Cloudflare 防护

站点使用 Cloudflare（需 `cf_clearance` cookie）。

---

## 三、发布字段与通用模型的映射

### 3.1 媒介映射（medium_sel）

| 通用媒介 | Lajidui medium_sel 值 |
|---------|----------------------|
| Blu-ray | 1 |
| HD DVD | 2 |
| Remux | 3 |
| MiniBD | 4 |
| HDTV | 5 |
| DVDR | 6 |
| Encode | 7 |
| CD | 8 |
| Track | 9 |
| WEB-DL | 10 |
| Other | 11 |

### 3.2 编码映射（codec_sel）

| 通用编码 | Lajidui codec_sel 值 |
|---------|----------------------|
| H.264 | 1 |
| VC-1 | 2 |
| Xvid | 3 |
| MPEG-2 | 4 |
| Other | 5 |
| AV1 | 6 |
| H.265 | 7 |

### 3.3 音频编码映射（audiocodec_sel）

| 通用音频编码 | Lajidui audiocodec_sel 值 |
|-------------|--------------------------|
| FLAC | 1 |
| APE | 2 |
| DTS | 3 |
| MP3 | 4 |
| OGG | 5 |
| AAC | 6 |
| Other | 7 |
| WAV | 8 |
| DTS-HD | 9 |
| TrueHD | 10 |
| LPCM | 11 |
| E-AC-3 | 12 |
| AC-3 | 13 |

### 3.4 分辨率映射（standard_sel）

| 通用分辨率 | Lajidui standard_sel 值 |
|-----------|------------------------|
| 1080p | 1 |
| 1080i | 2 |
| 720p | 3 |
| SD | 4 |
| 2K | 5 |
| 4K | 6 |
| 8K | 7 |
| Other | 8 |

### 3.5 地区映射（source_sel — 实际为地区）

| 通用地区 | Lajidui source_sel 值 |
|---------|----------------------|
| 大陆 | 7 |
| 台湾 | 2 |
| 香港 | 8 |
| 日本 | 10 |
| 韩国 | 11 |
| 欧美 | 1 |
| 印度 | 3 |
| Other | 6 |

### 3.6 文件格式映射（processing_sel — 实际为文件格式）

| 文件格式 | Lajidui processing_sel 值 |
|---------|--------------------------|
| EPUB | 1 |
| PDF | 2 |
| TXT | 3 |
| DOCX | 4 |
| PPTX | 5 |
| XLSX | 6 |
| WPS | 7 |
| AZW3 | 8 |
| MOBI | 9 |
| MKV | 10 |
| MP4 | 11 |
| RAR | 12 |
| ZIP | 13 |
| 7z | 14 |
| ISO | 16 |
| Other | 17 |

---

## 四、站点间字段语义对比

| 字段名 | 标准含义 | Lajidui 含义 | 其他特殊站 |
|--------|---------|-------------|-----------|
| `source_sel` | 来源媒介 | **地区** | PTCafe（地区） |
| `processing_sel` | 处理方式 | **文件格式** | YHPP（地区） |
| `audiocodec_sel` | 音频编码 | 音频编码 | — |
| `medium_sel` | 媒介 | 媒介 | — |

**重要结论**：NexusPHP 站间即使字段名相同，含义也可能完全不同。适配器必须按站点配置映射，不能依赖字段名推断。

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
