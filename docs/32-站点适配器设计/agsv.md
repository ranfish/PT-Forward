# 末日 站点适配器设计

> AGSV 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 末日|
| 站点地址 | https://www.agsvpt.com |
| 站点框架 | NexusPHP |
| 特殊规则 | Cloudflare 防护、种审制（油猴脚本辅助审核）、双区域发布（综合区+学习区）、27 个黑名单制作组、大包规则（>1T 必须标注）、短剧/漫画/学习资料分类、ALAC/M4A 音频 |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://tracker.agsvpt.cn/announce.php` |
| 规则页面 | `forums.php?action=viewtopic&forumid=1&topicid=6` |
| 种审脚本 | `greasyfork.org/scripts/482900` |

---

## 一、发种规范

### 1.1 绝对禁止的资源

- **他站禁止转载资源**（举报奖励 2w 魔力，违者第一次警告，第二次 ban 号）
- **9kg 资源**（NC-17/III级/R18/18 级影片及露点资源）
- 质量较差视频（CAM/TC/TS/SCR/R5/HalfCD/MiniSD/MNHD/RMVB/RM/flv，针对电影）
- 重复资源（同一来源完全相同，仅保留较早发布，断种例外）
- 国内正在上映的电影资源

### 1.2 限制发布的资源

- BT 转载资源（仅允许种审组审核通过的）
- **已完结电视剧必须以合集形式发布**，限制发布单集

### 1.3 主标题规范（0DAY 格式）

**电视剧**：
```
英文名 S(季) 年份 分辨率 来源平台 来源方式 编码 画质技术 音频编码 声道数-制作组
```
范例：`The Glory S01 2023 2160p NF WEB-DL HEVC DV DDP 5.1-AGSVWEB`

**必须包含**：英文电影名称、上映年份、视频来源方式、视频编码格式、音频编码方式、制作组

**编码命名**：H.264/AVC 统一写 AVC，HEVC/H.265 统一写 HEVC，x264/x265 写 x264 或 x265

**音乐专辑**：`艺术家名 - 专辑名 年份 - 格式 分辨率 - 制作组`
范例：`林俊杰 - 伟大的渺小 2017 - FLAC 24bit 96khz`

**游戏**：`游戏英文名 制作公司(可选) 平台 类型(可选) 语言 格式`

### 1.4 资源简介要求

- 电影资源**必须包含 MediaInfo**，且 MediaInfo 不必在正文中出现
- 电影电视剧**必须包含豆瓣链接、IMDb 链接**
- **必须包含主海报一张和截图至少一张**（图片需外链有效）
- **必须使用 PT-Gen 获取简介**（`https://api.iyuu.cn/ptgen/`）
- 合集需包含完整资源目录截图
- 保留原发布者申明信息，不得包含广告

### 1.5 大包规则

超过 **1T** 的种子**必须标注"大包"标签**。

### 1.6 标签使用规则

- **非官组成员禁止使用官方标签**，违者警告
- 标签尽量包含

---

## 二、发布页面表单字段分析

### 2.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | ✓ | 副标题（不能为空） |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `douban_id` | text | - | 豆瓣 ID/链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布 |
| `offers` | checkbox | - | 候选发布 |

### 2.2 双区域类型系统

#### 综合区（`browsecat`，data-mode='4'）— 12 个

| 值 | 显示名称 |
|----|----------|
| 401 | Movie(电影) |
| 402 | TV Series(电视剧) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录片) |
| 405 | Anime(动漫) |
| 406 | MV(演唱) |
| 407 | Sports(体育) |
| 408 | Audio(音频) |
| 411 | Music(音乐) |
| 413 | Game(游戏) |
| 415 | E-Book(电子书/有声书) |
| 419 | Playlet（短剧） |

#### 学习区（`specialcat`，data-mode='5'）— 4 个

| 值 | 显示名称 |
|----|----------|
| 412 | Software(软件) |
| 416 | Comic(漫画) |
| 417 | Education(学习资料) |
| 418 | Picture(图片) |

### 2.3 媒介（`medium_sel[4]`）— 10 个

| 值 | 显示名称 |
|----|----------|
| 11 | UHD Blu-ray |
| 1 | Blu-ray |
| 3 | Remux |
| 7 | Encode |
| 10 | WEB-DL |
| 5 | HDTV |
| 2 | DVD |
| 8 | CD |
| 12 | Track |
| 13 | Other |

### 2.4 编码（`codec_sel[4]`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 6 | H.265/HEVC |
| 2 | VC-1 |
| 4 | MPEG-2 |
| 12 | AV1 |
| 5 | Other |

### 2.5 音频编码（`audiocodec_sel[4]`）— 16 个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 6 | AAC |
| 7 | Other |
| 8 | DTS-HD MA |
| 9 | TrueHD |
| 10 | LPCM |
| 11 | DD/AC3 |
| 15 | WAV |
| 16 | M4A |
| 17 | TrueHD Atmos |
| 18 | DTS:X |
| 19 | DDP/E-AC3 |
| 20 | ALAC |

**特点**：含 **M4A**、**ALAC**、**WAV**，音频编码覆盖面较全。

### 2.6 分辨率（`standard_sel[4]`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p/1080i |
| 3 | 720p/720i |
| 4 | 480p/480i |
| 5 | 4K/2160p/2160i |
| 6 | 8K/4320p/4320i |
| 8 | Other |

**特点**：分辨率含帧率类型后缀（p/i），如 720p/720i、480p/480i。有 8K 选项。

### 2.7 制作组（`team_sel`）— 11 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 6 | AGSVPT | 站方主组 |
| 21 | AGSVWEB | 站方 WEB 组 |
| 20 | AGSVMUS | 站方音乐组 |
| 23 | GodDramas | 短剧制作组 |
| 24 | CatEDU | 教育制作组 |
| 16 | Pack | 合集打包组 |
| 28 | DYZ | |
| 29 | Hares | |
| 30 | BeiTai | |
| 31 | RL | |
| 22 | Other | |

**特点**：3 个站方组（AGSVPT/AGSVWEB/AGSVMUS）+ CatEDU（教育）+ GodDramas（短剧）+ Pack（合集打包）。

### 2.8 标签 — 22 个（去重后）

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 13 | 杜比 |
| 18 | 应求 |
| 19 | 完结 |
| 20 | 英字 |
| 21 | 粤语 |
| 22 | 大包 |
| 26 | 韩剧 |
| 31 | 美剧 |
| 47 | 特效 |
| 48 | 分集 |
| 49 | 超分 |
| 50 | 补帧 |
| 51 | 合并 |
| 52 | ⛵方舟 |

**特点**：
- 含 **韩剧**（26）、**美剧**（31）地区标签
- 含 **超分**（49，AI 超分辨率）、**补帧**（50）技术标签
- 含 **大包**（22，>1T 必须标注）、**分集**（48）、**合并**（51）
- 含 **⛵方舟**（52，保种计划标签）

---

## 三、黑名单制作组（来自种审脚本）

种审脚本检测以下 **27 个**制作组关键词（标题中匹配即标记为不受信）：

| # | 制作组 | # | 制作组 | # | 制作组 |
|---|--------|---|--------|---|--------|
| 1 | fgt | 10 | seeweb | 19 | sonyhd |
| 2 | hao4k | 11 | dreamhd | 20 | minihd |
| 3 | mp4ba | 12 | blacktv | 21 | bitstv |
| 4 | rarbg（x2） | 13 | xiaomi | 22 | -alt |
| 5 | gpthd | 14 | huawei | 23 | batweb |
| 6 | fgt（重复） | 15 | momohd | 24 | dbd-raws |
| 7 | mp4ba（重复） | 16 | ddhdtv | 25 | xunlei |
| 8 | hao4k（重复） | 17 | nukehd | 26 | zerotv |
| 9 | seeweb | 18 | tagweb | 27 | lelvetv |

**适配器需在发布前检查源种子标题**：如标题含上述关键词，该资源可能不适合转载。

---

## 四、关键适配器设计要点

### 4.1 禁转资源检查

必须检查源站种子是否标记为"禁转"，AGSV 对禁转违规处罚严厉（第一次警告，第二次 ban 号）。

### 4.2 已完结电视剧必须合集

已完结电视剧禁止单集发布，适配器需检测剧集完结状态。

### 4.3 大包标签强制

种子体积 > 1T 必须选择"大包"标签（值=22）。

### 4.4 标题 0DAY 规范

标题必须使用英文（0DAY 格式），编码统一写法：AVC（非 H.264）、HEVC（非 H.265），压制版写 x264/x265。

### 4.5 MediaInfo 必填（影视类）

电影资源必须在 `technical_info` 字段填写 MediaInfo，且不应出现在简介正文中。

### 4.6 source_sel 和 processing_sel 缺失

无来源/地区/处理方式字段，质量字段仅 5 个（medium/codec/audiocodec/standard/team）。

### 4.7 官方标签限制

非官组成员（非 AGSVPT/AGSVWEB/AGSVMUS）禁止使用官方标签。

---

## 五、发布字段与通用模型的映射

### 5.1 媒介映射（medium_sel）

| 通用媒介 | AGSV medium_sel 值 |
|---------|-------------------|
| UHD Blu-ray | 11 |
| Blu-ray | 1 |
| Remux | 3 |
| Encode | 7 |
| WEB-DL | 10 |
| HDTV | 5 |
| DVD | 2 |
| CD | 8 |
| Track | 12 |
| Other | 13 |

### 5.2 编码映射（codec_sel）

| 通用编码 | AGSV codec_sel 值 |
|---------|-------------------|
| H.264/AVC | 1 |
| H.265/HEVC | 6 |
| VC-1 | 2 |
| MPEG-2 | 4 |
| AV1 | 12 |
| Other | 5 |

### 5.3 音频编码映射（audiocodec_sel）

| 通用音频编码 | AGSV audiocodec_sel 值 |
|-------------|----------------------|
| FLAC | 1 |
| APE | 2 |
| DTS | 3 |
| MP3 | 4 |
| AAC | 6 |
| Other | 7 |
| DTS-HD MA | 8 |
| TrueHD | 9 |
| LPCM | 10 |
| DD/AC3 | 11 |
| WAV | 15 |
| M4A | 16 |
| TrueHD Atmos | 17 |
| DTS:X | 18 |
| DDP/E-AC3 | 19 |
| ALAC | 20 |

### 5.4 分辨率映射（standard_sel）

| 通用分辨率 | AGSV standard_sel 值 |
|-----------|---------------------|
| 1080p/1080i | 1 |
| 720p/720i | 3 |
| 480p/480i | 4 |
| 4K/2160p | 5 |
| 8K/4320p | 6 |
| Other | 8 |

---

## 审核脚本完整逆向分析

### 脚本信息

| 项目 | 内容 |
|------|------|
| 名称 | Agsv-Torrent-Assistant |
| 来源 | Greasyfork #482900 |
| 版本 | 1.4.7 |
| 作者 | Exception & 7ommy & AGSV骄阳 |
| 大小 | 1480 行 / 65KB |
| 运行页面 | `details.php*`（详情页）+ 审核弹窗页 |
| 权限 | GM_setValue / GM_getValue / GM_registerMenuCommand / GM_xmlhttpRequest |
| 基于上游 | SpringSunday-Torrent-Assistant（改写自不可说脚本） |

### 常量映射

#### 分类 (cat_constant) — 18 个

| ID | 名称 |
|----|------|
| 401 | Movie(电影) |
| 402 | TV Series(剧集) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录) |
| 405 | Anime(动画) |
| 406 | MV(演唱) |
| 407 | Sports(体育) |
| 408 | Audio(音频) |
| 409 | Misc(其他) |
| 411 | Music(音乐) |
| 412 | Software(软件) |
| 413 | Game(游戏) |
| 415 | E-Book(电子书/有声书) |
| 416 | Comic(漫画) |
| 417 | Education(学习资料) |
| 418 | Picture(图片) |
| 419 | Playlet(短剧) |

> **注意**：ID 非连续（无 410/414）。419 为短剧分类。以下分类**跳过大部分校验**：413(游戏)/418(图片)/412(软件)/411(音乐)/408(音频)/406(MV)/415(电子书)。411(音乐)额外检查采样频率和比特率。

#### 媒介 (type_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | Blu-ray | `blu-ray`/`bluray`（有 HEVC/AVC/VC-1/MPEG 时） |
| 2 | DVD | `dvd` |
| 3 | Remux | `remux` |
| 5 | HDTV | `hdtv` |
| 7 | Encode | `bluray`+编码 / `webrip`/`dvdrip`/`bdrip` |
| 8 | CD | - |
| 10 | WEB-DL | `web-dl`/`webdl` |
| 11 | UHD Blu-ray | `uhd blu-ray`/`uhd bluray`/` uhd ` |
| 12 | Track | - |
| 13 | Other | - |

> **关键差异**：末日将 Blu-ray(1) 和 Encode(7) 的区分逻辑与不可说完全不同。末日：`bluray`+无编码词=Encode(7)，`bluray`+有编码词=Blu-ray(1)。UHD Blu-ray(11) 独立媒介。

#### 视频编码 (encode_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | H.264/AVC | `264`/`avc` |
| 6 | H.265/HEVC | `265`/`hevc` |
| 2 | VC-1 | `vc`/`vc-1` |
| 4 | MPEG-2 | `mpeg2`/`mpeg-2` |
| 12 | AV1 | `av1`/`av-1` |
| 5 | Other | - |

> **注意**：匹配非常宽泛 — `264` 匹配到 `x264/h264/h.264` 等所有含 264 的字符串。

#### 音频编码 (audio_constant) — 14 个

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | FLAC | `flac` |
| 10 | LPCM | `lpcm` |
| 19 | DDP/E-AC3 | ` ddp`/` dd+`/`E-?AC-?3` |
| 6 | AAC | `aac` |
| 11 | DD/AC3 | ` ac3`/` dd` |
| 17 | TrueHD Atmos | `truehd`+`atmos` |
| 8 | DTS-HD MA | `dts-hd ma`/`dts-hdma`/`dts-hd` |
| 18 | DTS:X | `dts:x`/`dtsx`/`dts-x` |
| 3 | DTS | `dts`（排除 dts-x） |
| 9 | TrueHD | `truehd` |
| 2 | APE | - |
| 15 | WAV | - |
| 4 | MP3 | - |
| 16 | M4A | - |
| 7 | Other | - |

> **注意**：DTS-HD MA(8) 在 DTS(3) 之前匹配，DTS:X(18) 独立。TrueHD Atmos(17) 在 TrueHD(9) 之前匹配。

#### 分辨率 (resolution_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | 1080p/1080i | `1080p`/`1080i` |
| 3 | 720p/720i | `720p`/`720i` |
| 4 | 480p/480i | `480p`/`480i` |
| 5 | 4K/2160p/2160i | `4k`/`2160p`/`2160i`/`uhd` |
| 6 | 8K/4320p/4320i | `8k`/`4320p`/`4320i` |
| 8 | Other | - |

> **注意**：1080p/1080i 合并为同一 ID(1)，4K=2160p=UHD 合并为同一 ID(5)。ID 非连续（无 2/7）。

#### 地区 (area_constant) — 空定义

```
area_constant = {} // 空对象
```

> 末日脚本**不检测地区**，但页面基本信息区仍可解析地区字段（9 个地区同不可说）。

#### 制作组 (group_constant) — 9 个

| ID | 名称 | 标题匹配/来源 |
|----|------|-------------|
| 6 | AGSVPT | 标题含 `agsv` |
| 20 | AGSVMUSIC | 标题含 `agsvmus` |
| 21 | AGSVWEB | - |
| 23 | Hares | 标题含 `-hares` |
| 25 | RL | 标题含 `-rl`/`r²`/`-vandoge@R²` |
| 26 | BeiTai | 标题含 `beitai` |
| 24 | GodDramas | - |
| 16 | Pack | - |
| 22 | Other | - |

> **方舟计划**：Hares/RL/BeiTai 三组统称为"方舟"种子，须选方舟标签。

### 官组检测逻辑

```
标题含 "agsv"        → officialSeed (AGSVPT官种)
标题含 "goddramas"   → godDramaSeed (驻站短剧)
标题含 "agsvmus"     → officialMusicSeed (音乐官种，跳过所有校验)
标题含 "-hares"      → haresSeed (白兔)
标题含 "-rl"/"r²"    → redleavesSeed (红叶)
标题含 "beitai"      → beitaiSeed (备胎)
haresSeed || redleavesSeed || beitaiSeed → arkSeed (方舟)
```

### 简介内容检测

```
简介含 IMDb/豆瓣/TMDb 链接 → dbUrl=true
简介含 MediaInfo 关键词（general+video+audio / 概览+视频+音频 / disc info 等）→ isBriefContainsInfo=true
简介含 "禁止转载" → isBriefContainsForbidReseed=true
简介含 "片名" 或 "译名" → isBriefContainsMovieBrief=true
```

### 标题解析算法

```
1. 获取 h1#top 文本
2. 清除后缀标签：(已审)/(冻结)/(待定)/(免费)/(2X免费)/(推荐)/(热门)/(经典)/(禁止)/[2X]等
3. 清除「剩余时间...」和「(禁止)」
4. 检测「禁转」→ exclusive=1
5. 标题转小写 → title_lowercase
6. 正则匹配链：
   ├── 媒介(type)：
   │   WEB-DL→10, Remux→3, bluray+无编码词→Encode(7),
   │   webrip/dvdrip/bdrip→7, HDTV→5, UHD Blu-ray→11,
   │   bluray+有编码词→Blu-ray(1)
   ├── 编码(encode)：264/avc→1, 265/hevc→6, vc/vc-1→2, mpeg2→4, av1→12
   ├── 音频(audio)：flac→1, lpcm→10, ddp/eac3→19, aac→6, ac3/dd→11,
   │   truehd+atmos→17, dts-hd ma/dts-hd→8, dts:x/dtsx→18, dts→3
   ├── 分辨率(resolution)：1080p/i→1, 720p/i→3, 480p/i→4, 4k/2160p/uhd→5, 8k/4320p→6
   ├── 完结：`complete` → title_is_complete
   ├── 分集：`S\d+E\d+` → title_is_episode
   └── 制作组检测：agsv/goddramas/agsvmus/-hares/-rl/beitai
```

### MediaInfo 解析

```
1. MediaInfo 栏为空 → isMediainfoEmpty=true
2. 从 MediaInfo 提取：
   ├── Audio Language → isAudioChinese (Chinese/Mandarin)
   ├── Text Language → isTextChinese (Chinese) / isTextEnglish (English)
   └── 编码关键词 → mi_x264 / mi_x265
3. MediaInfo 含 BBCode → mediainfo_err
```

### 校验规则 — 共 30+ 项

#### 标题校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 标题含中文/非ASCII字符 | `[^\x00-\xff]`（排除特殊符号 ￡/™/Ⅱ 等 + 电子书分类豁免） | 错误 |
| 2 | 标题含不信任制作组 | 关键词列表（20+组） | 错误 |

#### 副标题/分类校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 3 | 副标题为空 | `!subtitle` | 错误 |
| 4 | 副标题含"动画"但未选 Anime 分类 | `isSubtitleAnime && cat!==405` | 错误 |
| 5 | 未选择分类 | `!cat` | 错误 |
| 6 | 完结剧集未选完结标签 | `title_is_complete && !is_complete` + cat∈{402,403,404} | 错误 |

#### 字段选择校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 7 | 未选择媒介 | `!type` | 错误 |
| 8 | 标题媒介与选择不一致 | `title_type !== type` | 错误 |
| 9 | 未选择视频编码 | `!encode` | 错误 |
| 10 | 标题编码与选择不一致 | `title_encode !== encode` | 错误 |
| 11 | 未选择音频编码 | `!audio` | 错误 |
| 12 | 标题音频与选择不一致 | `title_audio !== audio` | 错误 |
| 13 | 未选择分辨率 | `!resolution` | 错误 |
| 14 | 标题分辨率与选择不一致 | `title_resolution !== resolution` | 错误 |

#### 简介与链接校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 15 | 简介无 IMDb/豆瓣/TMDb 链接（非短剧） | `!dbUrl && cat!==419` | 错误 |
| 16 | 官种媒体信息未解析 | `officialSeed && short===full` | 错误 |
| 17 | MediaInfo 含 BBCode | `[/b]/[/color]` 等标签 | 错误 |
| 18 | 简介含 MediaInfo | `isBriefContainsInfo` | 错误 |
| 19 | 缺少海报或截图 | `imgCount < 2` | 错误 |
| 20 | MediaInfo 栏为空 | `isMediainfoEmpty` | 错误 |

#### 标签与制作组校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 21 | 官种未选制作组 | `officialSeed && !isGroupSelected` | 错误 |
| 22 | GodDramas 禁转但未选禁转标签 | `godDramaSeed && !isReseedProhibited && 简介含禁止转载` | 错误 |
| 23 | GodDramas 未选短剧分类 | `godDramaSeed && cat!==419` | 错误 |
| 24 | GodDramas 未选驻站标签 | `godDramaSeed && !isTagResident` | 错误 |
| 25 | 非官种选了官方标签 | `!officialSeed && isOfficialSeedLabel` | 错误 |
| 26 | 官种未选官方标签 | `officialSeed && !isOfficialSeedLabel` | 错误 |
| 27 | 官种/驻站组未选冰种标签 | `(officialSeed\|godDramaSeed) && !isIceSeedLabel` | 错误 |
| 28 | 分集未选分集标签 | `!isEpisode && (title_is_episode\|subtitle_is_episode)` | 错误 |
| 29 | 非方舟选了方舟标签 | `isTagArcProj && !arkSeed` | 错误 |
| 30 | 方舟未选方舟标签 | `!isTagArcProj && arkSeed` | 错误 |
| 31 | Hares 制作组选择错误 | `haresSeed && category!==23` | 错误 |
| 32 | RL 制作组选择错误 | `redleavesSeed && category!==25` | 错误 |
| 33 | BeiTai 制作组选择错误 | `beitaiSeed && category!==26` | 错误 |
| 34 | 非 Hares 选了 Hares | `!haresSeed && category===23` | 错误 |

#### 编码标题一致性校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 35 | 官种 MI 含 x264 但标题无 x264 | `mi_x264 && !title_x264 && officialSeed && cat===6` | 错误 |
| 36 | 官种 MI 含 x265 但标题无 x265 | `mi_x265 && !title_x265 && officialSeed && cat===6` | 错误 |

#### 音乐分类特殊规则

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 37 | 音乐标题缺采样频率 | `cat===411 && !khz` | 错误 |
| 38 | 音乐标题缺比特率 | `cat===411 && !bit` | 错误 |

#### 警告类规则

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| W1 | 低分辨率请检查高清版 | `resolution∈{8,4}` 且非官种 | 警告 |
| W2 | MI 含中字但未选中字标签 | `isTextChinese && !isTagTextChinese` | 警告 |
| W3 | MI 含英字但未选英字标签 | `isTextEnglish && !isTagTextEnglish` | 警告 |
| W4 | 大于 1T 未选大包标签 | `isBiggerThan1T && !isTagBigTorrent` | 警告 |
| W5 | 简介含冗余影片参数图片 | 特定图片 URL 检测 | 警告 |

### 分类级跳过校验

```
以下分类清空所有错误和警告：
- 413(Game) / 418(Picture) / 412(Software) / 411(Music) / 408(Audio) / 406(MV) / 415(E-Book)

音乐官种 (officialMusicSeed)：
- 清空所有错误，仅检查"是否选择了制作组"

Music(411) 在清空后额外检查：
- 主标题缺少采样频率(khz)
- 主标题缺少比特率(bit)
```

### 不信任制作组（20+组）

```
FGT, Hao4K, Mp4Ba, RARBG, GPTHD, SeeWeb, DreamHD, BlackTV, Xiaomi,
Huawei, MomoHD, DDHDTV, NukeHD, TagWeb, SonyHD, MiniHD, BitsTV,
ALT, BATWEB, DBD-Raws, XunLei, ZeroTV, LelveTV
```

### UI 功能

| 功能 | 说明 |
|------|------|
| 错误/警告双提示框 | 红色(#F44336)错误 + 黄色(#ffdd59)警告 |
| 提示框位置可配置 | 3种位置：页面最上/主标题下方/主标题上方 |
| 一键通过按钮 | 审核页面添加双勾按钮 |
| 审核按钮放大 | GM_setValue 存储，可切换 |
| 审核弹窗自动操作 | 自动点击通过/拒绝，自动填写错误信息 |
| 快捷键 F4 | 一键通过（无错）或打开审核页（有错） |
| 快捷键 F3 | 关闭窗口 |
| Ctrl+Alt+1 | 自动拒绝模式（打开审核页并勾选拒绝） |
| 图片链接显示 | 种审模式下在图片前添加可点击链接 |
| 自动关闭/返回 | 通过后自动关闭或返回 |
| 清空备注按钮 | 审核弹窗中添加清空备注按钮 |
| 移动端适配 | 自动调整按钮大小和关闭行为 |
| 脚本菜单 | 注册3个菜单命令（按钮放大/自动关闭/图片链接） |

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题无中文/非ASCII字符（电子书415分类豁免）
2. 保留特殊符号：￡/™/Ⅱ 等
3. 移除源站后缀标签（(已审)/(冻结)/(免费)/(禁止)等）
4. 移除「剩余时间...」
5. UHD Blu-ray 独立媒介（非标准 Blu-ray）
6. Encode 类须正确区分：bluray+编码词=Encode，bluray无编码词=原盘
7. 检测禁转标记并选禁转标签
```

### 副标题自动处理

```
1. 禁止为空（必填）
2. 副标题含"动画" → 分类须选 Anime(405)
3. 副标题含"第X集" → 须选分集标签
4. 优先从 PT-Gen/豆瓣获取中文名
```

### 质量字段自动选择

```
从源站标题解析：
1. 媒介(type)：
   WEB-DL→10, Remux→3, Encode(含编码的bluray/webrip/dvdrip/bdrip)→7,
   HDTV→5, UHD Blu-ray→11, Blu-ray(含编码词)→1, DVD→2, CD→8, Track→12, Other→13
2. 编码(encode)：
   H.264/AVC→1, H.265/HEVC→6, VC-1→2, MPEG-2→4, AV1→12, Other→5
3. 音频(audio)：按匹配优先级
   FLAC→1, LPCM→10, DDP/E-AC3→19, AAC→6, DD/AC3→11,
   TrueHD Atmos→17, DTS-HD MA→8, DTS:X→18, DTS→3, TrueHD→9,
   APE→2, WAV→15, MP3→4, M4A→16, Other→7
4. 分辨率(resolution)：
   1080p/1080i→1, 720p/720i→3, 480p/480i→4, 4K/2160p/UHD→5, 8K/4320p→6, Other→8
5. 制作组(group)：
   AGSVPT→6, AGSVMUSIC→20, AGSVWEB→21, Hares→23, RL→25, BeiTai→26,
   GodDramas→24, Pack→16, Other→22

注意 remastered 排除在 4K 检测之外
```

### 标签自动选择

```
1. 官方：标题含 agsv → 勾选官方标签
2. 冰种：官种或驻站短剧 → 勾选冰种标签
3. 驻站：标题含 goddramas → 勾选驻站标签
4. 方舟：标题含 hares/-rl/beitai → 勾选方舟标签
5. 中字：MI Text Language 含 Chinese → 警告级建议勾选
6. 英字：MI Text Language 含 English → 警告级建议勾选
7. 国语：MI Audio Language 含 Chinese/Mandarin → 建议勾选（已注释但逻辑保留）
8. 大包：种子体积 > 1TB → 警告级建议勾选
9. 分集：标题含 S**E** 或副标题含"第X集" → 勾选
10. 完结：标题含 complete + cat∈{402,403,404} → 勾选
11. 禁转：标题含"禁转" 或 GodDramas 种子含"禁止转载" → 勾选
```

### MediaInfo 处理

```
1. MediaInfo 须放入独立栏位（禁止在简介中包含）
2. MediaInfo 禁止含 BBCode 标签
3. 官种必须解析（短格式≠原始格式）
4. 从 MI 提取音频/字幕语言用于标签自动选择
5. 简介中禁止包含冗余的影片参数图片
```

### 简介/豆瓣信息

```
1. 简介必须包含 IMDb/豆瓣/TMDb 链接（短剧419分类豁免）
2. 简介禁止包含 MediaInfo 内容（通用关键词检测+多站点NFO格式检测）
3. 简介建议包含"片名"或"译名"字段（已注释但保留逻辑）
4. 至少 2 张图片（海报+截图）
```

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-19*
*数据来源：upload.php + Wiki + Agsv-Torrent-Assistant.js v1.4.7 (1480行/65KB)*
