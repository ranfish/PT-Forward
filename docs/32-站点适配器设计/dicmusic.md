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
- 非音乐资源不能是免费可获取的
- 发布时确保至少出种一个
- 种子中禁止打广告或包含个人信息（提供艺术家/专辑/厂牌/零售信息不算打广告）
- 禁止带压缩或镜像文件（Scene 例外允许 .zip/.rar/.iso）
- **个人首种免惩机制**：新用户发布的 2GB 以内的首种不会因违规受惩罚（超过 2GB 不受保护，成功发布后免惩资格即失效）

### 2.2 特别禁止

- **MQA 编码文件**（严格禁止）
- 任何类型的视频（电影/电视剧/音乐会/增强型 CD 中的视频数据）
- 色情和裸露内容（除非官方商业发行中包含）
- 任何游戏
- 图片/壁纸合集
- 用户自制合集
- DTS-HD、mp3HD、HD-AAC 等混合格式
- 含 DRM 版权限制的文件
- 汽车零件和数据程序
- **HDCD 内容**（抓轨存在固有问题）
- 翻录自广播/电视/播客的音乐
- 非官方观众现场录音（AUD/IEM/ALD/Mini-Disc/Matrix）
- **用户自编的集锦**（"我最爱的 34 首"类、Billboard 榜单拼凑）
- **整轨专辑**（发布整轨用户会被警告；DJ/专业混音例外可 MP3+Cue）
- 采样率大于 320kbps 的 CBR
- 不明来源的 24-bit 资源

### 2.3 允许的格式

- **有损**: MP3、AAC、AC3、DTS（AC3/DTS 仅限 DVD/蓝光提取的多声道音轨）
- **无损**: FLAC（最大 24-bit/192kHz，仅分轨）、DSF/DFF（仅分轨，仅 SACD/Web 媒介，不允许 ISO）
- 禁止有损源转码（仅允许无损源转码）
- 官方有损母带发行不被视为有损混编，可正常发布
- 禁止有损混编（同一种子混合无损和有损文件）
- 有损格式平均比特率须 ≥ 192kbps（LAME V2/V1/V0/APS/APX 例外）
- .m4a/.mp4 封装 AAC，.flac 封装 FLAC（其他封装禁止）
- **FLAC 须使用 lv8 压缩等级**（Mora 下载的 WEB 资源须压缩后发布）
- Web 媒介须以售卖时的原始位深度和采样率发布
- 不同采样率/位深度的 Web 种子可共存（如果售卖方式不同）
- 允许发布免费音乐（来自官方来源的互联网免费音乐）

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

## 六、手把手发种教程要点（Wiki）

### 6.1 发布前检查清单

1. **避免重复**：搜索艺术家+专辑名（至少两种方式），确认站内无同版本/同格式/同规格资源
2. **添加格式 vs 普通发布 vs 替代发布**：
   - 站内无任何资源 → **普通发布**
   - 站内有同艺术家同专辑但无你手中版本/格式/规格 → **添加格式**（优先）
   - 站内有相同资源但你手中更优 → **替代发布**（须用报告功能报告旧种）
3. **检查文件**：禁止格式、文件名合规、扫图合规、FLAC lv8 压缩、多碟分子文件夹

### 6.2 JSON 文件导入

- 可从其他 Gazelle 站点下载版本的 JSON 文件（种子详情页 JS 按钮），上传到 DIC 快速填入信息
- **必须仔细检查**其他站点信息是否准确且符合 DIC 规则

### 6.3 艺术家命名

- **严禁不假思索地填"群星（Various Artists）"**
- 每行只填一位艺术家，多位艺术家点"+"另起一行
- 可先填重要的，发布后补充

### 6.4 年份字段区分

- **原始发行版年份**（必填）：专辑首版全球首次发行的年份（非录制年份）
- **发行版年份**（重制时必填）：该特定版本的发行时间
  - 例：1987 年首版，2000 年再版 → 原始=1987，发行=2000
  - 媒介选 CD → 发行版年份不可能早于 1982 年

### 6.5 厂牌和目录编号

- 非必须字段，但对 Dupe/替代判定有参考作用
- 不确定请留空，**不要随便填写或网上照搬**
- 以实体专辑/扫图/文件夹名/Log 中的信息为准

### 6.6 媒介选择

- CD：从 CD 抓轨
- WEB：从 Mora/Amazon/iTunes/QQ音乐等在线商店下载
- SACD：须在种子描述提供**来源信息**（什么设备翻录、什么软件、处理手法）
- WEB 资源须在种子描述提供**购买截图/购买链接**
- 不确定来源 → Unknown Media

### 6.7 封面和扫图

- 尽量使用官方海报图（非拍照），不含水印和不相关元素
- JPG 和 PNG 为主，**禁止 TIFF/BMP/GIF**
- 扫描分辨率高于 300 dpi 可能过大，推荐压缩为 JPG
- 允许小册子制作成 PDF，但单张扫图也要避免过大

### 6.8 专辑描述 vs 种子描述

- **专辑描述**：专辑信息、背景描述、曲目列表（[#] 自动编号 BBCode）
- **种子描述**：编码信息、抓轨设置、频谱图（PNG 不压缩）、来源说明、购买链接、作者评语
- **不要在种子描述粘贴 EAC/XLD 抓轨日志**（其他 .log 日志可用 [hide] 标签）

### 6.9 标记使用

- 自购/自抓属于"原创"标记，转载资源不得使用
- 禁转类标记仅"原创"资源可用
- 建议使用"未允禁转"

### 6.10 发布后

- **须从 DIC 重新下载种子做种**（不能用原始种子）
- 检查客户端和站点均正常做种
- 可通过"ED"按钮修改版本信息、格式、比特率、媒介、种子描述
- 可通过"编辑描述"修改专辑信息（权限不足可"请求编辑"）
- Log 未正确显示 → 用报告功能"请求重新计算 Log 得分"

---

## 七、有损替代优先级详解

```
替代方向（高 → 低）：

320 CBR
  ↓ 替代
V0 (VBR) / APX (VBR)
  ↓ 替代
V1 (VBR) / 256 CBR / 256 ABR
  ↓ 替代
V2 (VBR) / APS (VBR)
  ↓ 替代
192 CBR / 192 ABR / 192-210 VBR

规则：
- AAC 256 CBR 可替代任何 VBR AAC
- AAC 可被其他允许的编码替代（同媒介同版本）
- Scene 无损可被带 100% Log 的非 Scene 替代
- 有损 Scene 可被非 Scene 替代
```

---

*数据来源: upload.php HTML (1049行) + rules.php (完整上传规则 32000+ 字符) + Wiki 手把手发种教程 + Wiki 禁转/黑名单/垃圾/首发 (2026-04-22 补充)*
*文档创建: 2026-04-16*
*文档更新: 2026-04-22（补充完整上传规则 2.1-2.10、手把手发种教程、有损替代优先级、首种免惩机制、HDCD 禁止、WEB 来源要求）*
