# 柠檬不甜 站点适配器设计

> LemonHD 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 柠檬不甜|
| 站点地址 | https://lemonhd.net |
| 站点框架 | NexusPHP |
| 特殊规则 | 双语分类名（中英）、4K-UHD/8K-UHD 独立媒介、PT-Gen + IMDb + Bangumi + Douban 四来源、匿名发布、发布者 5 倍上传量 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://tracker.lemonhd.net/announce.php` |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（`data-pt-gen="url"`） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 PT-Gen 集成

字段 `data-pt-gen` 出现在 `url` 和 `pt_gen` 两个属性中。外部信息链接支持 **四来源**：
- **IMDb**（`url` 字段）
- **Douban**（豆瓣）
- **Bangumi**（番组计划）
- **PT-Gen**（自动填充）

### 1.3 缺失字段

- **`source_sel`**：无来源字段
- **`processing_sel`**：无处理方式字段
- **`dburl`**：无独立豆瓣字段（豆瓣通过 PT-Gen 间接支持）

### 1.4 类型（`type`）— 9 个分类

`<select name="type" id="browsecat" data-mode='4'>`

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 402 | Muisc(音乐) |
| 403 | Animations(动漫/动画) |
| 404 | Music Videos(音乐视频) |
| 405 | Documentaries(纪录片) |
| 406 | TV Series(电视剧) |
| 407 | TV Shows(综艺) |
| 408 | 3D(3D视频) |
| 409 | Other(其它) |

**特点**：
- 分类名使用 **中英双语** 格式
- 有 **3D**（408）独立分类（较少见）
- 音乐拼写为 "Muisc"（原文拼写错误）
- 剧集=406、综艺=407（非标准 402/404）
- 仅 `data-mode='4'`，单模式发布

### 1.5 媒介（`medium_sel[4]`）— 10 个

`<select name="medium_sel[4]" data-mode="medium_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | Encode |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | 4K-UltraHD |
| 7 | 8K-UltraHD |
| 8 | DVD |
| 9 | CD |
| 10 | Other |

**特点**：
- **4K-UltraHD**（值=6）和 **8K-UltraHD**（值=7）作为独立媒介选项
- 与其他站点不同，4K/8K 是媒介而非分辨率
- Encode=2（非标准，通常值较大）

### 1.6 编码（`codec_sel[4]`）— 5 个

`<select name="codec_sel[4]" data-mode="codec_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 2 | H.265/HEVC |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | Other |

**特点**：极简编码列表，无 AV1、VP9、Xvid 等选项。

### 1.7 音频编码（`audiocodec_sel[4]`）— 13 个

`<select name="audiocodec_sel[4]" data-mode="audiocodec_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | Atmos TrueHD |
| 2 | TrueHD |
| 3 | DTS-HD MA |
| 4 | DTS X |
| 5 | DTS |
| 6 | AC3/DD |
| 7 | EAC3/DDP |
| 8 | AAC |
| 9 | LPCM |
| 10 | FLAC |
| 11 | WAV |
| 12 | APE |
| 13 | Other |

**特点**：
- DTS 细分 3 级（DTS/DTS-HD MA/DTS X）
- Atmos TrueHD 和 TrueHD 分开
- AC3/DD 和 EAC3/DDP 分开
- 无 MP3、OGG、DSD 等选项

### 1.8 分辨率（`standard_sel[4]`）— 5 个

`<select name="standard_sel[4]" data-mode="standard_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | 4K_UHD |
| 2 | 1080p/i |
| 3 | 720p/i |
| 4 | SD |
| 5 | Other |

**特点**：
- 4K_UHD 作为分辨率选项（值=1），同时媒介也有 4K-UltraHD
- 无 8K 独立分辨率（8K 仅在媒介中体现）
- 1080p/i 和 720p/i 合并隔行/逐行

### 1.9 制作组（`team_sel[4]`）— 6 个

`<select name="team_sel[4]" data-mode="team_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | LemonHD |

**特点**：仅 6 个制作组，含站方组 LemonHD 和 4 个老牌中文组。

### 1.10 标签（`tags[4][]`）— 8 个

`<input type="checkbox" name="tags[4][]" value="XX" />`

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 4K |
| 9 | 原盘 |

**特点**：8 个标签，简洁实用。含 **禁转**（1）和 **首发**（2）标签。注意值 3 缺失。

---

## 二、关键适配器设计要点

### 2.1 双语分类名

分类名使用中英双语（如 `Movies(电影)`），适配器需匹配中英文部分。

### 2.2 4K/8K 双重体现

4K/8K 同时出现在媒介（medium_sel）和分辨率（standard_sel）中，发布时需同步填写：
- 媒介选 4K-UltraHD（6）或 8K-UltraHD（7）
- 分辨率选 4K_UHD（1）

### 2.3 极简编码和制作组

编码仅 5 种，制作组仅 6 个，适配器映射简单。但源站有更多编码/制作组时，可能大量映射到 "Other"。

### 2.4 PT-Gen 多来源

支持 IMDb + Douban + Bangumi + PT-Gen 四种来源，适配器可充分利用自动填充。

### 2.5 3D 独立分类

有 3D 视频独立分类（408），适配器需识别 3D 内容。

### 2.6 Dupe 规则

标准 NexusPHP dupe 规则：Blu-ray/HD DVD > HDTV > DVD > TV。动漫类 HDTV 和 DVD 同优先级。

### 2.7 发布者 5 倍上传量

发布者获得 5 倍上传量奖励（非标准的 2 倍），站内发布积极性较高。

---

## 三、发布字段与通用模型的映射

### 3.1 类型映射（type）

| 通用类型 | LemonHD type 值 |
|---------|----------------|
| 电影 | 401 |
| 音乐 | 402 |
| 动漫 | 403 |
| MV | 404 |
| 纪录片 | 405 |
| 电视剧 | 406 |
| 综艺 | 407 |
| 3D | 408 |
| 其他 | 409 |

### 3.2 媒介映射（medium_sel）

| 通用媒介 | LemonHD medium_sel 值 |
|---------|----------------------|
| Blu-ray | 1 |
| Encode | 2 |
| Remux | 3 |
| WEB-DL | 4 |
| HDTV | 5 |
| 4K-UHD | 6 |
| 8K-UHD | 7 |
| DVD | 8 |
| CD | 9 |
| Other | 10 |

### 3.3 编码映射（codec_sel）

| 通用编码 | LemonHD codec_sel 值 |
|---------|---------------------|
| H.264/AVC | 1 |
| H.265/HEVC | 2 |
| VC-1 | 3 |
| MPEG-2 | 4 |
| Other | 5 |

### 3.4 音频编码映射（audiocodec_sel）

| 通用音频编码 | LemonHD audiocodec_sel 值 |
|-------------|--------------------------|
| Atmos TrueHD | 1 |
| TrueHD | 2 |
| DTS-HD MA | 3 |
| DTS:X | 4 |
| DTS | 5 |
| AC3/DD | 6 |
| EAC3/DDP | 7 |
| AAC | 8 |
| LPCM | 9 |
| FLAC | 10 |
| WAV | 11 |
| APE | 12 |
| Other | 13 |

### 3.5 分辨率映射（standard_sel）

| 通用分辨率 | LemonHD standard_sel 值 |
|-----------|------------------------|
| 4K/UHD | 1 |
| 1080p/i | 2 |
| 720p/i | 3 |
| SD | 4 |
| Other | 5 |

### 3.6 制作组映射（team_sel）

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | LemonHD |

### 3.7 标签映射（tags）

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 4K |
| 9 | 原盘 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
