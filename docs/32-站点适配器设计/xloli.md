# 萝莉 站点适配器设计

> 萝莉站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 萝莉|
| 站点地址 | https://mua.xloli.cc |
| 站点框架 | NexusPHP |
| 特殊规则 | **双区域发布**：综合区（data-mode=4）+ 9KG 区（data-mode=6）、大量成人内容分类、动漫向制作组、OPUS 音频、舞台演出分类 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://A.x-anime.cc/announce.php` |

**重要约束**：
- 适配器只使用综合区（browsecat，data-mode=4）的分类

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

### 1.2 双区域类型系统

#### 综合区（`browsecat`，data-mode='4'）— 10 个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 (Movie) |
| 402 | 电视剧 (TV Series) |
| 430 | 综艺 (TV Show) |
| 405 | 动画 (Animation) |
| 408 | 音乐 (Music) |
| 410 | 舞台演出 (Stage Performance) |
| 404 | 纪录片 (Documentary) |
| 412 | 游戏 (Game) |
| 413 | 软件 (Software) |
| 411 | 漫画/CG杂图/动漫杂志 |

#### 9KG 区（`specialcat`，data-mode='6'）— 11 个

| 值 | 显示名称 |
|----|----------|
| 420 | AV(有码/Censored) |
| 419 | AV(无码/Uncensored) |
| 423 | 里番(Hanime) |
| 425 | H-2D同人动画(H 2D doujin anime) |
| 426 | H-3D同人动画(H 3D doujin anime) |
| 424 | H-漫画(H Manga) |
| 427 | H-CG杂图(H-CG) |
| 418 | 写真(Picture) |
| 429 | 音声(Audio) |
| 428 | H-游戏(H Game) |
| 422 | H-综艺(H TV Show) |

**注意**：9KG 区分类极其丰富（11 个成人内容分类）。

**综合区特点**：
- 有 **舞台演出**（410）分类，其他站极少有
- 有 **漫画/CG杂图/动漫杂志**（411）分类
- 综艺使用 430（非标准 403）
- 有游戏（412）和软件（413）

### 1.3 两个区域共享相同的质量字段

综合区（mode=4）和 9KG 区（mode=6）使用完全相同的质量字段值，仅字段名后缀不同（`[4]` vs `[6]`）。

#### 地区（`source_sel`）— 10 个

| 值 | 显示名称 |
|----|----------|
| 1 | 大陆 (CHN) |
| 2 | 香港 (HKG) |
| 3 | 台湾 (TWN) |
| 4 | 欧美 (West) |
| 5 | 韩国 (KOR) |
| 6 | 日本 (JPN) |
| 7 | 印度 (IND) |
| 8 | 俄国 (RUS) |
| 11 | 泰国 (THA) |
| 13 | 其它 (Other) |

**特点**：`source_sel` 表示地区（非来源媒介），含 **俄国** 和 **泰国** 地区。

#### 媒介（`medium_sel`）— 9 个

| 值 | 显示名称 |
|----|----------|
| 14 | UHD Blu-ray |
| 1 | Blu-ray |
| 3 | Remux |
| 7 | Encode |
| 5 | HDTV |
| 12 | WEB-DL |
| 2 | DVD |
| 8 | CD |
| 11 | Other |

#### 编码（`codec_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 9 | AVC/x264 |
| 8 | HEVC/x265 |
| 15 | AV1 |
| 12 | VC-1 |
| 13 | MPEG-2 |
| 5 | Other |

**特点**：支持 AV1。值编号较大（8/9/12/13/15），非连续。

#### 音频编码（`audiocodec_sel`）— 12 个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 3 | DTS |
| 6 | AAC |
| 9 | TrueHD |
| 10 | DTS-HD MA |
| 13 | AC3/DD |
| 14 | Other |
| 15 | LPCM |
| 20 | DDP/E-AC3 |
| 21 | MP3 |
| 23 | DTS:X |
| 24 | OPUS |

**特点**：含 **OPUS** 音频编码。

#### 分辨率（`standard_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p/4K |
| 9 | Other |

#### 制作组（`team_sel`）— 13 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 5 | Other | 其他 |
| 11 | VCB-Studio | 动漫压制组 |
| 15 | 7³ACG | 动漫压制组 |
| 39 | Snow-Raws | 动漫 Raw 组 |
| 40 | jsum@U2 | 动漫压制组 |
| 41 | GodDramas | 短剧组 |
| 42 | AI-Raws | AI 修复 Raw 组 |
| 46 | CMCT | CMCT |
| 47 | beAst | beAst |
| 48 | ANK-Raws | 动漫 Raw 组 |
| 49 | LittleBakas! | 动漫组 |
| 50 | mawen1250 | 动漫压制组 |
| 53 | Moozzi2 | 动漫压制组 |

**特点**：
- **动漫向制作组为主**（VCB-Studio/7³ACG/AI-Raws/ANK-Raws/Snow-Raws/Moozzi2/mawen1250/LittleBakas!）
- 值编号不连续（5/11/15/39-53），可能是后续逐步添加
- 仅 CMCT、beAst、GodDramas 为常规 PT 制作组

### 1.4 标签 — 22 个（两个模式共用）

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR10 |
| 8 | R18 |
| 9 | 原生原盘 |
| 10 | 其他 |
| 11 | LOLI |
| 13 | DoVi |
| 14 | HDR10+ |
| 15 | 自购 |
| 16 | Atmos |
| 18 | ENSub |
| 19 | GalGame |
| 20 | 原创 |
| 21 | 分集 |
| 22 | 特效 |
| 23 | 粤语 |
| 24 | 无对白 |

**特点**：
- HDR 细分三级：HDR10（7）/ DoVi（13）/ HDR10+（14）
- 含 **LOLI**（11，站标标签）、**R18**（8）、**GalGame**（19）
- 含 **ENSub**（18，英文字幕）、**无对白**（24）
- 含 **原生原盘**（9）、**自购**（15）、**分集**（21）

---

## 二、关键适配器设计要点

### 2.1 9KG 区

- 适配器使用 `browsecat`（data-mode=4）
- `specialcat`（data-mode=6）含 11 个分类（418-430）

### 2.2 source_sel = 地区

`source_sel` 表示地区（大陆/港台/日韩/欧美/俄国/泰国等），非来源媒介。含泰国和俄国，与其他站不同。

### 2.3 动漫向定位

制作组以动漫 Raw 组和压制组为主（VCB-Studio、AI-Raws、Snow-Raws 等），适配器在匹配制作组时需优先匹配动漫相关组。

### 2.4 两个区域字段完全相同

综合区和 9KG 区的所有质量字段值完全一致（source_sel/medium_sel/codec_sel/audiocodec_sel/standard_sel/team_sel），仅字段名后缀不同。适配器只需使用 `[4]` 后缀。

---

## 三、发布字段与通用模型的映射

### 3.1 综合区类型映射（type，仅允许综合区）

| 通用类型 | 萝莉 type 值 |
|---------|-------------|
| 电影 | 401 |
| 电视剧 | 402 |
| 综艺 | 430 |
| 纪录片 | 404 |
| 动画 | 405 |
| 音乐 | 408 |
| 舞台演出 | 410 |
| 漫画/CG | 411 |
| 游戏 | 412 |
| 软件 | 413 |

### 3.2 地区映射（source_sel）

| 通用地区 | 萝莉 source_sel 值 |
|---------|-------------------|
| 大陆 | 1 |
| 香港 | 2 |
| 台湾 | 3 |
| 欧美 | 4 |
| 韩国 | 5 |
| 日本 | 6 |
| 印度 | 7 |
| 俄国 | 8 |
| 泰国 | 11 |
| 其它 | 13 |

### 3.3 媒介映射（medium_sel）

| 通用媒介 | 萝莉 medium_sel 值 |
|---------|-------------------|
| UHD Blu-ray | 14 |
| Blu-ray | 1 |
| Remux | 3 |
| Encode | 7 |
| HDTV | 5 |
| WEB-DL | 12 |
| DVD | 2 |
| CD | 8 |
| Other | 11 |

### 3.4 编码映射（codec_sel）

| 通用编码 | 萝莉 codec_sel 值 |
|---------|-------------------|
| AVC/x264 | 9 |
| HEVC/x265 | 8 |
| AV1 | 15 |
| VC-1 | 12 |
| MPEG-2 | 13 |
| Other | 5 |

### 3.5 音频编码映射（audiocodec_sel）

| 通用音频编码 | 萝莉 audiocodec_sel 值 |
|-------------|----------------------|
| FLAC | 1 |
| DTS | 3 |
| AAC | 6 |
| TrueHD | 9 |
| DTS-HD MA | 10 |
| AC3/DD | 13 |
| Other | 14 |
| LPCM | 15 |
| DDP/E-AC3 | 20 |
| MP3 | 21 |
| DTS:X | 23 |
| OPUS | 24 |

### 3.6 分辨率映射（standard_sel）

| 通用分辨率 | 萝莉 standard_sel 值 |
|-----------|---------------------|
| 1080p | 1 |
| 1080i | 2 |
| 720p | 3 |
| SD | 4 |
| 2160p/4K | 5 |
| Other | 9 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
