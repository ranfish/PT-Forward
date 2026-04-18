# 海豚 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 海豚|
| 站点地址 | https://dicmusic.com |
| **站点框架** | **Gazelle**（非 NexusPHP） |
| 内容定位 | **纯音乐站点**，禁止发布非音乐内容 |
| 特殊功能 | Log Checker、艺人/发布组/发行商黑名单、禁转资源处理、音乐垃圾（GSC/SAE/SPGM/AIGM）识别 |
| 规则页面 | rules.php?p=upload + 多个 Wiki 页面 |

---

## 一、发布页面表单字段分析

**重要**: DIC Music 使用 **Gazelle 框架**，与 NexusPHP 完全不同。提交方式为标准 POST 表单。

### 1.1 基础信息字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `file_input` | file | 种子文件 |
| `artists[]` | text + select | 艺人名称 + 重要性 |
| `importance[]` | select | 艺人重要性（主要/客座/作曲/指挥/DJ/重混/制作人） |
| `title` | text | 专辑标题 |
| `subtitle` | text | 副标题 |
| `year` | text | 发行年份 |
| `record_label` | text | 唱片公司 |
| `catalogue_number` | text | 目录编号 |
| `tags` | text | 标签（自动完成，逗号分隔） |
| `image` | text | 封面图片 URL |
| `album_desc` | textarea | 专辑描述（BBCode） |
| `release_desc` | textarea | 种子描述（BBCode） |
| `logfiles[]` | file | EAC/XLD Log 文件（可多个） |

### 1.2 版本/重制字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `remaster` | checkbox | 是否为重制版 |
| `unknown` | checkbox | 未知版本信息 |
| `remaster_year` | text | 重制年份 |
| `remaster_title` | text | 重制版本标题 |
| `remaster_record_label` | text | 重制唱片公司 |
| `remaster_catalogue_number` | text | 重制目录编号 |

### 1.3 特殊标记字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `vanity_house` | checkbox | 自制音乐 |
| `scene` | checkbox | Scene 发布 |
| `diy` | checkbox | DIY（变更时修改原始状态） |
| `jinzhuan` | checkbox | 金砖（disabled，管理组设置） |
| `buy` | checkbox | 自购 |

### 1.4 音乐分类（`releasetype`）— 15个

| 值 | 显示名称 |
|----|----------|
| 1 | 专辑 |
| 3 | 原声 |
| 5 | EP |
| 6 | 精选 |
| 7 | 集锦 |
| 9 | 单曲专辑 |
| 11 | 现场版专辑 |
| 13 | 重混音 |
| 14 | 私制唱片 |
| 15 | 访谈 |
| 16 | 集锦盒带 |
| 17 | 录音样带 |
| 18 | 音乐会录音 |
| 19 | DJ 混音 |
| 21 | 未知 |

### 1.5 媒介（`media`）— 10个

| 值 | 显示名称 |
|----|----------|
| CD | CD |
| DVD | DVD |
| Vinyl | Vinyl（黑胶） |
| Soundboard | Soundboard（调音台） |
| SACD | SACD |
| Blu-ray | Blu-ray |
| DAT | DAT |
| Cassette | Cassette（磁带） |
| WEB | WEB |
| Unknown Media | Unknown Media |

**注意**: 值使用**字符串**（非数字），这是 Gazelle 框架特征。

### 1.6 格式（`format`）— 6个

| 值 | 显示名称 |
|----|----------|
| FLAC | FLAC |
| DSD | DSD |
| MP3 | MP3 |
| AAC | AAC |
| AC3 | AC3 |
| DTS | DTS |

**注意**: AC3 和 DTS **仅限于** DVD/蓝光中提取的多声道音轨。

### 1.7 比特率/品质（`bitrate`）— 12个

| 值 | 显示名称 |
|----|----------|
| 192 | 192 |
| APS (VBR) | APS (VBR) |
| V2 (VBR) | V2 (VBR) |
| V1 (VBR) | V1 (VBR) |
| 256 | 256 |
| APX (VBR) | APX (VBR) |
| V0 (VBR) | V0 (VBR) |
| q8.x (VBR) | q8.x (VBR) |
| 320 | 320 |
| Lossless | Lossless |
| 24bit Lossless | 24bit Lossless |
| Other | Other |

### 1.8 艺人重要性（`importance[]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 主要 |
| 2 | 客座 |
| 3 | 重混 |
| 4 | 作曲 |
| 5 | 指挥 |
| 6 | DJ／编曲 |
| 7 | 制作人 |

### 1.9 类型（`type`）

类型字段选项较少，具体值待确认。

### 1.10 音乐流派标签（`genre_tags`）— 70个

使用下拉选择一个主标签 + `tags` 文本框手动输入额外标签：

时代（8个）: 1950s-2020s
地区/语言（16个）: america, asian, britain, canada, cantonese, chinese, english, europe, french, hong.kong, japanese, korean, mandarin, singaporean, spanish, taiwanese
流派（34个）: ambient, blues, classical, country, dance, disco, drum.and.bass, electronic, folk, funk, garage, hip.hop, house, idm, indie, jazz, latin, metal, noise, pop, psychedelic, psytrance, punk, reggae, rhythm.and.blues, rock, soul, synthpop, trance 等
特殊（12个）: anime, cantopop, harmonica, instrumental, jpop, kpop, live, mandopop, nationalmusic, orchestral, piano, score, soundtrack, stage.and.screen, vocaloid

---

## 二、发种规则（rules.php + Wiki）

### 2.1 黄金规则

- **只允许发布音乐**（禁止电影/电视剧/游戏/图片等一切非音乐内容）
- **不允许任何类别的重复种子**（有例外见下文）
- 发布时确保至少出种一个
- 种子中禁止打广告或包含个人信息
- 禁止带压缩或镜像文件（Scene 例外允许 .zip/.rar/.iso）

### 2.2 特别禁止

- **MQA 编码文件**（严格禁止）
- 电影/电视剧/音乐会视频
- 色情和裸露内容（除非官方商业发行中包含）
- 任何游戏
- 图片/壁纸合集
- 用户自制合集
- DTS-HD、mp3HD、HD-AAC 等混合格式

### 2.3 允许的格式

- **有损**: MP3、AAC、AC3、DTS（AC3/DTS 仅限 DVD/蓝光提取的多声道音轨）
- **无损**: FLAC（最大 24-bit/192kHz）、DSF/DFF（仅分轨，仅 SACD/Web 媒介）
- 禁止有损源转码
- 禁止有损混编（同一种子混合无损和有损文件）
- 有损格式平均比特率须 ≥ 192kbps（部分 VBR 例外）

### 2.4 版本与发行规则

- 不同采样率/位深度的 Web 种子可以共存
- 禁止种子包含不同质量差异类型的音频
- .m4a/.mp4 封装 AAC，.flac 封装 FLAC
- MP3+Cue 允许 DJ/专业混音整轨发布

### 2.5 Dupe/替代规则

- 不允许任何类别重复
- 同一音频不同采样率/位深度的 Web 种子可共存（如售卖方式不同）
- 非有损母带发行可替代 Web 源有损母带发行

### 2.6 Scene 发布

- 必须符合音乐质量和格式要求
- 修改后不可再勾选 Scene
- 未修改的 .nfo 可写入种子描述

### 2.7 音乐垃圾规则（Wiki）

禁止发布以下内容：
- **GSC（常规平台独占集锦）**: 本质为歌单的平台独占集锦
- **SAE（平台独占专辑选摘）**: 将专辑切分后单独销售的 EP
- **SPGM（程序生成音乐）**: 无人工参与的算法生成歌曲
- **AIGM（AI 生成音乐）**: AI 工具生成的音乐
- **人工智能强提规格**: AI 算法提升低规格音频到高规格

### 2.8 黑名单（Wiki）

- **图床黑名单**: imgur.com, funkyimg.com, fastpic.ru 等
- **艺人黑名单**: 章进城（偷歌/污染曲库）
- **发布组黑名单**: KIMOJI（有损混编/劣质转码）、QHStudio（低质量种子）

### 2.9 禁转资源处理（Wiki）

- 严禁转载禁转资源（转出和转入都严肃处理）
- 转出：视情况处理（警告/封禁权限/封号）
- 转入：删种 + 视情况处理
- 优先接受原发布者申诉

---

## 三、DIC Music 特殊注意事项

### 3.1 Gazelle 框架（非 NexusPHP）

DIC Music 是已采集站点中**唯一使用 Gazelle 框架**的站点。字段名、提交方式、值类型（字符串 vs 数字）均与 NexusPHP 完全不同。适配器需完全独立实现。

### 3.2 纯音乐站点

只允许发布音乐，适配器需过滤所有非音乐内容。

### 3.3 艺人+重要性系统

Gazelle 特有的艺人系统，需填写艺人名和重要性（主要/客座/作曲/指挥/DJ/重混/制作人）。

### 3.4 MQA 严格禁止

MQA 编码的 FLAC 文件完全禁止发布。适配器需检测并拒绝 MQA 资源。

### 3.5 Log Checker

支持 EAC/XLD Log 文件上传和自动评分。CD 抓轨资源建议附带 Log 文件。

### 3.6 标签使用自动完成

`tags` 字段使用 `data-gazelle-autocomplete="true"`，支持逗号分隔多标签。`genre_tags` 下拉选择主流派标签。

### 3.7 字符串值（非数字）

`media`、`format`、`bitrate` 等字段使用字符串值（如 "CD"、"FLAC"、"Lossless"），非数字 ID。

### 3.8 音乐垃圾检测

需识别 GSC/SAE/SPGM/AIGM 类型的"音乐垃圾"资源，禁止发布。

### 3.9 24-bit/192kHz 上限

FLAC 最大允许 24-bit 位深度、192kHz 采样率。超高规格需降级处理。

---

## 四、与其他站点对比

| 特征 | DIC Music | 常见 NexusPHP 站点 |
|------|-----------|-------------------|
| 框架 | **Gazelle** | NexusPHP |
| 内容 | **纯音乐** | 综合影视 |
| 媒介值类型 | **字符串**（CD/Vinyl/WEB） | 数字（1-12） |
| 格式字段 | format（FLAC/DSD/MP3） | 无（编码在 codec_sel） |
| 比特率字段 | bitrate（Lossless/320/VBR） | 无 |
| 艺人系统 | **有**（artists[]+importance[]） | 无 |
| 流派标签 | **70个** | 无 |
| Log Checker | **有** | 无 |
| MQA 禁止 | **严格禁止** | 不适用 |
| 音乐垃圾 | **GSC/SAE/SPGM/AIGM 规则** | 不适用 |
| 发布组黑名单 | KIMOJI/QHStudio | 因站而异 |

---

## 五、适配器实现要点

### 5.1 Gazelle 框架适配

```go
// DIC Music uses Gazelle, not NexusPHP
adapter.Framework = "gazelle"
adapter.ContentType = "music_only"
```

### 5.2 非音乐内容过滤

```go
func isAllowedForDIC(req *PublishRequest) bool {
    return req.Category == "Music" // Only music allowed
}
```

### 5.3 MQA 检测

```go
func isMQA(filename string, metadata map[string]string) bool {
    // Check for MQA indicators in filename or metadata
    if strings.Contains(strings.ToUpper(filename), "MQA") { return true }
    if metadata["encoder"] == "MQA" { return true }
    return false
}
```

### 5.4 字符串值映射

```go
func mapMedia(sourceMedium string) string {
    switch {
    case strings.Contains(sourceMedium, "CD"): return "CD"
    case strings.Contains(sourceMedium, "Vinyl"): return "Vinyl"
    case strings.Contains(sourceMedium, "SACD"): return "SACD"
    case strings.Contains(sourceMedium, "WEB"): return "WEB"
    case strings.Contains(sourceMedium, "Blu-ray"): return "Blu-ray"
    case strings.Contains(sourceMedium, "DVD"): return "DVD"
    default: return "WEB"
    }
}

func mapFormat(sourceFormat string) string {
    switch {
    case strings.Contains(sourceFormat, "FLAC"): return "FLAC"
    case strings.Contains(sourceFormat, "DSD"): return "DSD"
    case strings.Contains(sourceFormat, "MP3"): return "MP3"
    case strings.Contains(sourceFormat, "AAC"): return "AAC"
    default: return "FLAC"
    }
}

func mapBitrate(bitDepth int, isLossless bool) string {
    if isLossless {
        if bitDepth >= 24 { return "24bit Lossless" }
        return "Lossless"
    }
    return "Other"
}
```

### 5.5 艺人信息构建

```go
type ArtistInput struct {
    Name       string
    Importance int // 1=主要, 2=客座, 3=重混, 4=作曲, 5=指挥, 6=DJ, 7=制作人
}
```

---

*数据来源: upload.php HTML (1049行) + rules.php (1146行) + Wiki 禁转/黑名单/垃圾/首发 (442+514+500行) (2026-04-16)*
*文档创建: 2026-04-16*
