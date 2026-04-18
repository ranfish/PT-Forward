# 家园 站点适配器设计

> HDHome 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 家园|
| 站点地址 | https://hdhome.org |
| 备用域名 | https://hdbiger.org |
| 站点框架 | NexusPHP |
| 特殊规则 | 候选制（Crazy User 及以上免候选）、双区域发布（种子区/LIVE 区）、8K 分类、豆瓣 ID 字段 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://t.hdhome.org/announce.php` 或 `https://hdbiger.org/announce.php` |
| 规则页面 | `forums.php?action=viewtopic&forumid=14&topicid=601`（发种规则）、`topicid=8847`（资源格式规范） |

---

## 一、发种规范

### 1.1 候选制

- **Crazy User（营长）及以上**：可直接发布种子，无需经过候选
- **Peasant（俘虏）及以上**：可以添加候选
- 候选在投票通过或被管理通过后会收到通知短信，显示为【允许】

### 1.2 不允许的资源

- 分辨率未达 **1080i** 或以上
- 总体积小于 **100MB** 的资源
- 标清视频 upscale 或部分 upscale 而成的视频
- 质量较差的视频文件（CAM、TC、TS、SCR、DVDSCR、R5、R5.Line、HalfCD 等）
- RealVideo 编码（RMVB/RM）、flv 文件
- 单独的样片（样片请和正片一起上传）
- 未达到 5.1 声道标准的有损音频（有损 MP3、有损 WMA 等）
- 无正确 cue 表单的多轨音频文件
- RAR 等压缩文件
- 重复（dupe）资源
- 禁忌或敏感内容
- 损坏的文件、垃圾文件
- **杂项（课程、电子书、软件、漫画、墙纸、素材等）概不接受**

### 1.3 标题命名规则

**电影**：
```
[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```
范例：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 1080p BluRay x264-SiNNERS`

**电视剧**：
```
[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```
范例：`越狱 Prison Break S04E01 PROPER 1080p HDTV x264-CTU`

**音轨**：
```
[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]
```
范例：`恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC`

**标题注意事项**（来自 topicid=8847）：
- 主标题不能含有中文名（除了片名）
- DTS-HD 不能出现其他标点符号
- 末尾不能出现文件后缀名
- 副标题不需要重复出现和主标题相同英文片名
- 转载来源注明在简介中即可，副标题不能出现

---

## 二、发布页面表单字段分析

### 2.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（不填则使用种子文件名，要求规范填写） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `douban_id` | text | - | 豆瓣 ID 或链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |

**注意**：有 `douban_id` 字段（非标准 NexusPHP），可输入豆瓣影视 ID 或链接。

### 2.2 类型（`type`）— 双区域

HDHome 有两个 `type` 下拉框，**只选其中之一**：

#### 种子区 — 49 个分类

| 值 | 显示名称 |
|----|----------|
| 506 | Movies 8K UHD BD |
| 499 | Movies UHD Blu-ray |
| 518 | Movies UHD REMUX |
| 450 | Movies Bluray |
| 415 | Movies REMUX |
| 505 | Movies 8K/4320p |
| 416 | Movies 2160p |
| 414 | Movies 1080p |
| 413 | Movies 720p |
| 411 | Movies SD |
| 412 | Movies IPad |
| 523 | TVSeries 8KUHD |
| 502 | TVSeries 4K Bluray |
| 451 | Doc Bluray |
| 421 | Doc REMUX |
| 526 | TVSeries 4320p |
| 431 | TVShow 2160p |
| 433 | TVSeries IPad |
| 434 | TVSeries 720p |
| 435 | TVSeries 1080i |
| 436 | TVSeries 1080p |
| 437 | TVSeries REMUX |
| 453 | TVSereis Bluray |
| 438 | TVSeries 2160p |
| 439 | Musics APE |
| 432 | TVSeries SD |
| 440 | Musics FLAC |
| 441 | Musics MV |
| 503 | Musics Bluray |
| 442 | Sports 720p |
| 510 | Anime 8K UHD BD |
| 443 | Sports 1080i |
| 444 | Anime SD |
| 445 | Anime IPad |
| 446 | Anime 720p |
| 447 | Anime 1080p |
| 448 | Anime REMUX |
| 454 | Anime Bluray |
| 531 | Anime UHD REMUX |
| 409 | Misc |
| 449 | Anime 2160p |
| 509 | Anime 8K/4320p |
| 501 | Anime UHD Blu-ray |
| 504 | Sports 2160p |
| 511 | Sport 8K/4320p |
| 508 | Doc 8K UHD BD |
| 529 | Doc 8K UHD BD REMUX |
| 500 | Doc UHD Blu-ray |
| 507 | Doc 8K/4320p |
| 422 | Doc 2160p |
| 420 | Doc 1080p |
| 419 | Doc 720p |
| 417 | Doc SD |
| 418 | Doc IPad |
| 424 | TVMusic 1080i |
| 423 | TVMusic 720p |
| 452 | TVShows Bluray |
| 430 | TVShow REMUX |
| 429 | TVShow 1080p |
| 428 | TVShow 1080i |
| 427 | TVShow 720p |
| 425 | TVShow SD |
| 426 | TVShow IPad |

#### LIVE 区 — 27 个分类

| 值 | 显示名称 |
|----|----------|
| 494 | Movies Bluray |
| 495 | Doc Bluray |
| 469 | TVMusic 1080i |
| 472 | TVShow 720p |
| 473 | TVShow 1080i |
| 474 | TVShow 1080p |
| 475 | TVShow REMUX |
| 496 | TVShows Bluray |
| 476 | TVShow 2160p |
| 477 | TVSeries SD |
| 478 | TVSeries IPad |
| 479 | TVSeries 720p |
| 480 | TVSeries 1080p |
| 481 | TVSeries REMUX |
| 497 | TVSereis Bluray |
| 482 | TVSeries 2160p |
| 483 | Musics APE |
| 484 | Musics FLAC |
| 486 | Sports 720p |
| 487 | Sports 1080i |
| 488 | Anime SD |
| 489 | Anime IPad |
| 490 | Anime 720p |
| 491 | Anime 1080p |
| 492 | Anime REMUX |
| 498 | Anime Bluray |
| 493 | Anime 2160p |

**分类特点**：
- 分辨率编码在分类值中（如 414=Movies 1080p），与 HDFans 类似的 data-mode 方式
- 有完整的 8K/4320p 分类体系（505/506/507/509/511/523/526/529/531）
- TV 系列细分为 TVShow（综艺）和 TVSeries（剧集）两大类
- 有 IPad 分类（小屏设备专用编码）
- LIVE 区为实时直播内容

### 2.3 来源（`source_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 9 | UHD Blu-ray |
| 1 | Blu-ray |
| 4 | HDTV |
| 3 | DVD |
| 7 | WEB-DL |
| 8 | Other |

### 2.4 媒介（`medium_sel`）— 8 个

| 值 | 显示名称 |
|----|----------|
| 10 | UHD Blu-ray |
| 1 | Blu-ray |
| 3 | Remux |
| 7 | Encode |
| 5 | HDTV |
| 8 | CD |
| 4 | MiniBD |
| 11 | WEB-DL |

### 2.5 编码（`codec_sel`）— 5 个

| 值 | 显示名称 |
|----|----------|
| 1 | AVC/H264/x264 |
| 2 | HEVC/H265/x265 |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | Other |

### 2.6 音频编码（`audiocodec_sel`）— 13 个

| 值 | 显示名称 |
|----|----------|
| 6 | AAC |
| 15 | AC3/DD |
| 2 | APE |
| 16 | WAV |
| 1 | FLAC |
| 3 | DTS |
| 13 | TrueHD |
| 14 | LPCM |
| 11 | DTS-HDMA |
| 18 | DTS-HDHRA |
| 12 | TrueHD Atmos |
| 17 | DTS-HDMA:X 7.1 |
| 7 | Other |

### 2.7 分辨率（`standard_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | 2160p/4K |
| 2 | 1080p |
| 3 | 1080i |
| 4 | 720p |
| 5 | SD |
| 10 | 4320p/8K |

### 2.8 处理（`processing_sel`）— 2 个

| 值 | 显示名称 |
|----|----------|
| 1 | Raw |
| 2 | Encode |

### 2.9 制作组（`team_sel`）— 14 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | HDHome | 站方制作组 |
| 2 | HDH | 站方简称 |
| 3 | HDHTV | 站方 TV 组 |
| 4 | HDHPad | 站方 Pad 组 |
| 12 | HDHWEB | 站方 WEB 组 |
| 20 | 3201 | 制作组 |
| 17 | SHMA | 制作组 |
| 21 | TVman | 制作组 |
| 19 | ARiN | 制作组 |
| 6 | TTG | TTG |
| 7 | M-Team | 馒头 |
| 11 | Other | 其他 |
| 22 | 969154968 | 制作组 |
| 23 | BMDru | 制作组 |

### 2.10 标签（`tags[]`）— 18 个 checkbox

| 值 | 显示名称 | CSS 类 |
|----|----------|--------|
| yc | 原创 | tyc |
| gy | 国语 | tgy |
| yy | 粤语 | tyy |
| gz | 官字 | tgz |
| zz | 中字 | tzz |
| tx | 特效 | ttx |
| ybyp | 原生 | tybyp |
| lz | 连载 | tlz |
| wj | 完结 | twj |
| diy | DIY | tdiy |
| db | DOLBY VISION | tdb |
| hdr10 | HDR10 | thdr10 |
| hdrm | HDR10+ | thdrm |
| cc | Criterion | tcc |
| jz | 禁转 | tjz |
| xz | 限转 | txz |
| sf | 首发 | tsf |
| yq | 应求 | tyq |

**注意**：标签使用字符串值（如 `yc`、`gy`），非数字 ID。

### 2.11 其他字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `offers` | checkbox | 候选发布（value="yes"） |
| `uplver` | checkbox | 匿名发布（value="yes"） |

---

## 三、关键适配器设计要点

### 3.1 双区域 type 字段

发布页面有两个同名的 `type` 下拉框（`browsecat` 和 `specialcat`），选择一个时另一个自动禁用。适配器需注意：
- 正常转发使用 **种子区**（`browsecat`）
- LIVE 区一般不用于转发
- 两个下拉框的 `name` 都是 `type`，提交时只发选中的那个

### 3.2 分类编码包含分辨率和类型

与 HDFans 类似，type 值本身编码了内容类型+分辨率信息。例如：
- 414 = Movies + 1080p
- 438 = TVSeries + 2160p
- 506 = Movies + 8K UHD BD

适配器需根据内容类型和分辨率组合来确定正确的 type 值。

### 3.3 豆瓣 ID 字段

`douban_id` 字段支持输入豆瓣影视 ID 或链接（如 `https://movie.douban.com/subject/35050809/` 或 `35050809`）。

### 3.4 标题规则特殊要求

- 主标题不能含中文名（除了片名）
- DTS-HD 不能出现其他标点符号
- 末尾不能出现文件后缀名
- 副标题不能重复主标题英文片名
- 副标题不能出现转载来源（来源注明在简介中）

### 3.5 候选制

非 Crazy User 等级的用户需先提交候选，通过后才能发布。适配器可设置 `offers=yes` 参数来候选发布。

### 3.6 资源简介要求

- 必须包含海报、横幅或封面
- 尽可能包含画面截图
- 尽可能包含文件详细信息（格式、时长、编码、码率、分辨率、语言、字幕）
- 尽可能包含演职员名单和剧情概要
- 无 NFO 文件时必须填写编码信息

---

## 四、发布字段与通用模型的映射

### 4.1 来源映射（source_sel）

| 通用来源 | HDHome source_sel 值 |
|----------|---------------------|
| UHD Blu-ray | 9 |
| Blu-ray | 1 |
| HDTV | 4 |
| DVD | 3 |
| WEB-DL | 7 |
| Other | 8 |

### 4.2 媒介映射（medium_sel）

| 通用媒介 | HDHome medium_sel 值 |
|----------|---------------------|
| UHD Blu-ray | 10 |
| Blu-ray | 1 |
| Remux | 3 |
| Encode | 7 |
| HDTV | 5 |
| CD | 8 |
| MiniBD | 4 |
| WEB-DL | 11 |

### 4.3 编码映射（codec_sel）

| 通用编码 | HDHome codec_sel 值 |
|----------|---------------------|
| AVC/x264 | 1 |
| HEVC/x265 | 2 |
| VC-1 | 3 |
| MPEG-2 | 4 |
| Other | 5 |

### 4.4 音频编码映射（audiocodec_sel）

| 通用音频编码 | HDHome audiocodec_sel 值 |
|-------------|-------------------------|
| FLAC | 1 |
| APE | 2 |
| DTS | 3 |
| AAC | 6 |
| Other | 7 |
| TrueHD | 13 |
| LPCM | 14 |
| AC3/DD | 15 |
| WAV | 16 |
| DTS-HDMA | 11 |
| DTS-HDHRA | 18 |
| TrueHD Atmos | 12 |
| DTS-HDMA:X 7.1 | 17 |

### 4.5 分辨率映射（standard_sel）

| 通用分辨率 | HDHome standard_sel 值 |
|-----------|----------------------|
| 4320p/8K | 10 |
| 2160p/4K | 1 |
| 1080p | 2 |
| 1080i | 3 |
| 720p | 4 |
| SD | 5 |

### 4.6 处理映射（processing_sel）

| 通用处理 | HDHome processing_sel 值 |
|---------|------------------------|
| Raw | 1 |
| Encode | 2 |

### 4.7 type 分类映射（种子区主要分类）

| 内容类型 | 分辨率/媒介 | type 值 |
|---------|------------|---------|
| Movies | 8K UHD BD | 506 |
| Movies | UHD Blu-ray | 499 |
| Movies | UHD REMUX | 518 |
| Movies | Bluray | 450 |
| Movies | REMUX | 415 |
| Movies | 8K/4320p | 505 |
| Movies | 2160p | 416 |
| Movies | 1080p | 414 |
| Movies | 720p | 413 |
| Movies | SD | 411 |
| Movies | IPad | 412 |
| TVSeries | 8KUHD | 523 |
| TVSeries | 4K Bluray | 502 |
| TVSeries | 4320p | 526 |
| TVSeries | 2160p | 438 |
| TVSeries | 1080p | 436 |
| TVSeries | 1080i | 435 |
| TVSeries | 720p | 434 |
| TVSeries | SD | 432 |
| TVSeries | IPad | 433 |
| TVSeries | REMUX | 437 |
| TVSeries | Bluray | 453 |
| TVShow | 2160p | 431 |
| TVShow | 1080p | 429 |
| TVShow | 1080i | 428 |
| TVShow | 720p | 427 |
| TVShow | SD | 425 |
| TVShow | IPad | 426 |
| TVShow | REMUX | 430 |
| TVShows | Bluray | 452 |
| Doc | 8K UHD BD | 508 |
| Doc | 8K UHD BD REMUX | 529 |
| Doc | UHD Blu-ray | 500 |
| Doc | 8K/4320p | 507 |
| Doc | Bluray | 451 |
| Doc | REMUX | 421 |
| Doc | 2160p | 422 |
| Doc | 1080p | 420 |
| Doc | 720p | 419 |
| Doc | SD | 417 |
| Doc | IPad | 418 |
| Anime | 8K UHD BD | 510 |
| Anime | UHD Blu-ray | 501 |
| Anime | UHD REMUX | 531 |
| Anime | 8K/4320p | 509 |
| Anime | Bluray | 454 |
| Anime | REMUX | 448 |
| Anime | 1080p | 447 |
| Anime | 720p | 446 |
| Anime | IPad | 445 |
| Anime | SD | 444 |
| Anime | 2160p | 449 |
| Musics | APE | 439 |
| Musics | FLAC | 440 |
| Musics | MV | 441 |
| Musics | Bluray | 503 |
| Sports | 2160p | 504 |
| Sports | 8K/4320p | 511 |
| Sports | 720p | 442 |
| Sports | 1080i | 443 |
| TVMusic | 1080i | 424 |
| TVMusic | 720p | 423 |
| Misc | - | 409 |

---

## 五、特殊注意事项

### 5.1 分类体系特点

HDHome 的分类是 **内容类型 × 分辨率/媒介** 的矩阵式编码：
- 内容大类：Movies、TVSeries（剧集）、TVShow（综艺）、Doc、Anime、Musics、Sports、TVMusic、Misc
- 分辨率/媒介：8K UHD BD、UHD Blu-ray、Bluray、REMUX、8K/4320p、2160p、1080p、1080i、720p、SD、IPad
- 所有分类值均以 4xx/5xx 编号，无明显规律需查表

### 5.2 LIVE 区

LIVE 区是 HDHome 的特色功能，用于实时直播内容。一般转发场景不涉及 LIVE 区。

### 5.3 质量字段

`source_sel`、`medium_sel`、`codec_sel`、`audiocodec_sel`、`standard_sel`、`processing_sel` 均为非必填字段，但建议填写以提高种子质量。

### 5.4 最低分辨率要求

所有视频资源分辨率必须达到 **1080i** 或以上。SD 资源仅限于非视频内容。

### 5.5 杂项分类（Misc）

Misc 分类（type=409）存在但规则中明确表示「课程、电子书、软件、漫画、墙纸、素材等概不接受」，实际用途有限。

### 5.6 CSS 标签样式映射

标签在种子列表中有对应的颜色样式：

| 标签值 | CSS 类 | 背景色 |
|--------|--------|--------|
| gf | tgf | #06c |
| yc | tyc | #085 |
| gz | tgz | #530 |
| db | tdb | #358 |
| hdr10 | thdr10 | #9a3 |
| hdrm | thdrm | #9b5 |
| gy | tgy | #f96 |
| yy | tyy | #f66 |
| zz | tzz | #9c0 |
| tx | ttx | #f38 |
| jz | tjz | #903 |
| xz | txz | #c03 |
| diy | tdiy | #993 |
| sf | tsf | #339 |
| yq | tyq | #f90 |
| m0 | tm0 | #096 |
| bz | tbz | #333 |
| ybyp | tybyp | #5C44BB |
| xy | txy | #553300 |
| cc | tcc | #4488bb |
| wj | twj | #f02498 |
| lz | tlz | #d064e8 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
