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

### 4.2 9kg/成人内容禁止

适配器硬编码：禁止发布包含 NC-17/III/R18 级内容的资源。

### 4.3 已完结电视剧必须合集

已完结电视剧禁止单集发布，适配器需检测剧集完结状态。

### 4.4 大包标签强制

种子体积 > 1T 必须选择"大包"标签（值=22）。

### 4.5 标题 0DAY 规范

标题必须使用英文（0DAY 格式），编码统一写法：AVC（非 H.264）、HEVC（非 H.265），压制版写 x264/x265。

### 4.6 MediaInfo 必填（影视类）

电影资源必须在 `technical_info` 字段填写 MediaInfo，且不应出现在简介正文中。

### 4.7 source_sel 和 processing_sel 缺失

无来源/地区/处理方式字段，质量字段仅 5 个（medium/codec/audiocodec/standard/team）。

### 4.8 官方标签限制

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

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
