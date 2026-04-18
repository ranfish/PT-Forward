# 克隆 站点适配器设计

> HDClone 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 克隆|
| 站点地址 | https://pt.hdclone.top |
| 站点框架 | NexusPHP |
| 特殊规则 | 无 source_sel/audiocodec_sel/processing_sel、短剧分类、AV1 编码 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://tracker.hdclone.top/announce.php` |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（不填则使用种子文件名） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo |

### 1.2 缺失字段

以下标准 NexusPHP 字段在 HDClone **不存在**：

- **`source_sel`**：无来源/地区字段
- **`audiocodec_sel`**：无音频编码字段
- **`processing_sel`**：无处理方式字段
- **`uplver`**：无匿名发布选项
- **`pt_gen`**：无 PT-Gen 链接字段
- **`douban_id`**：无豆瓣字段

### 1.3 类型（`type`）— 9 个（data-mode='4'）

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 404 | Documentaries/纪录片 |
| 403 | TV Shows/综艺 |
| 405 | Animations/动漫、动画 |
| 409 | Playlet/短剧 |
| 410 | MV/演唱会 |
| 408 | Music/音乐 |
| 407 | Othes/其他（慎选） |

**特点**：
- 有 **Playlet/短剧**（409）分类，多数站无此分类
- MV/演唱会合并为一个分类（410）
- 值 409 在其他站通常为"其他"，此处为"短剧"

### 1.4 媒介（`medium_sel[4]`）— 10 个

| 值 | 显示名称 |
|----|----------|
| 9 | UHD Blu-ray |
| 1 | Blu-ray |
| 3 | Remux |
| 7 | Encode |
| 6 | WEB-DL |
| 4 | MiniBD |
| 10 | DVD |
| 8 | CD |
| 2 | HDTV |
| 5 | Track |

### 1.5 编码（`codec_sel[4]`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/x264/AVC |
| 6 | H265/HEVC/x265 |
| 2 | VC-1 |
| 3 | AV1 |
| 4 | MPEG-2 |
| 5 | Other |

**特点**：支持 **AV1** 编码（值=3），较前卫。H.264/x264/AVC 合并为一个选项。

### 1.6 分辨率（`standard_sel[4]`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 6 | 4320p/8K |
| 1 | 2160p/4K |
| 2 | 1080p/1080i |
| 3 | 720p |
| 4 | SD/720p以下 |
| 5 | Othes（慎选） |

**特点**：1080p/1080i 合并；SD 标注为"720p以下"；有"Othes（慎选）"选项。

### 1.7 制作组（`team_sel[4]`）— 14 个

| 值 | 显示名称 |
|----|----------|
| 6 | FRDS |
| 9 | beAst |
| 8 | CMCT |
| 7 | TLF |
| 4 | WiKi |
| 13 | UBits |
| 14 | PTer |
| 1 | M-Team |
| 2 | CHD |
| 3 | BeiTai |
| 5 | AGSV |
| 10 | HDSky |
| 11 | HDHome |
| 12 | TTG |

**注意**：无 "Other" 选项，只有这 14 个制作组。

### 1.8 标签（`tags[4][]`）— 5 个 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

**极简标签**：仅 5 个，是所有已分析站点中最少的之一。

### 1.9 其他字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `offers` | checkbox | 候选发布 |

---

## 二、关键适配器设计要点

### 2.1 字段极简

HDClone 是目前分析过的 NexusPHP 站点中字段最精简的之一：
- 无 source_sel（来源/地区）
- 无 audiocodec_sel（音频编码）
- 无 processing_sel（处理方式）
- 仅 5 个标签
- 仅 14 个制作组（无 Other）
- 无匿名发布选项

### 2.2 短剧分类

有独立的 Playlet/短剧（type=409）分类。如果源站有短剧内容，映射到此分类；否则"其他"为 type=407。

### 2.3 AV1 编码支持

编码字段支持 AV1（codec_sel=3），适配器需能识别 AV1 编码。

### 2.4 data-mode='4'

所有质量字段带 `[4]` 后缀（如 `medium_sel[4]`、`codec_sel[4]`），对应 `data-mode='4'`。

---

## 三、发布字段与通用模型的映射

### 3.1 类型映射（type）

| 通用类型 | HDClone type 值 |
|---------|----------------|
| 电影 | 401 |
| 电视剧 | 402 |
| 综艺 | 403 |
| 纪录 | 404 |
| 动漫 | 405 |
| 短剧 | 409 |
| MV/演唱会 | 410 |
| 音乐 | 408 |
| 其他 | 407 |

### 3.2 媒介映射（medium_sel）

| 通用媒介 | HDClone medium_sel 值 |
|---------|----------------------|
| UHD Blu-ray | 9 |
| Blu-ray | 1 |
| Remux | 3 |
| Encode | 7 |
| WEB-DL | 6 |
| MiniBD | 4 |
| DVD | 10 |
| CD | 8 |
| HDTV | 2 |
| Track | 5 |

### 3.3 编码映射（codec_sel）

| 通用编码 | HDClone codec_sel 值 |
|---------|----------------------|
| H.264/x264/AVC | 1 |
| H.265/HEVC/x265 | 6 |
| VC-1 | 2 |
| AV1 | 3 |
| MPEG-2 | 4 |
| Other | 5 |

### 3.4 分辨率映射（standard_sel）

| 通用分辨率 | HDClone standard_sel 值 |
|-----------|------------------------|
| 4320p/8K | 6 |
| 2160p/4K | 1 |
| 1080p/1080i | 2 |
| 720p | 3 |
| SD | 4 |
| Other | 5 |

### 3.5 制作组映射（team_sel）

| 制作组 | HDClone team_sel 值 |
|--------|---------------------|
| M-Team | 1 |
| CHD | 2 |
| BeiTai | 3 |
| WiKi | 4 |
| AGSV | 5 |
| FRDS | 6 |
| TLF | 7 |
| CMCT | 8 |
| beAst | 9 |
| HDSky | 10 |
| HDHome | 11 |
| TTG | 12 |
| UBits | 13 |
| PTer | 14 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
