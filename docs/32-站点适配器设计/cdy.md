# 传道院 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 传道院|
| 站点地址 | http://pt.cdy.skin |
| 站点框架 | NexusPHP |
| 主题 | BlasphemyOrange（橙色主题） |
| 口号 | 道可道，非常道 |
| 特殊功能 | 候选区(offers)、认领制度(claim)、签到、medium-zoom图片放大 |
| 规则页面 | rules.php |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 模式系统

使用 `data-mode='5'` 属性控制字段显示，字段名带 `[5]` 后缀。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名，要求规范填写） |
| `small_descr` | text | - | 副标题 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |

**重要缺失**:
- **无 `url`（IMDb）字段**
- **无 `pt_gen` 字段**
- **无 `uplver`（匿名发布）字段**
- 无 PT-Gen 自动获取功能
- 无 IMDb 链接输入

这是已采集站点中极少数完全没有 IMDb/PT-Gen/匿名发布字段的站点。

### 1.3 类型字段（`type`，data-mode=5）— 11个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies |
| 402 | TV Series |
| 403 | TV Shows |
| 404 | Documentaries |
| 405 | Animations |
| 406 | Music Videos |
| 407 | Sports |
| 408 | HQ Audio |
| 409 | Misc |
| 410 | Game |
| 411 | Program |

**注意**: 分类名全部使用英文。含 Music Videos(406)、Game(410)、Program(411)。无"综艺"分类（TV Shows 可能包含）。

### 1.4 媒介（`medium_sel[5]`）— 9个

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

**注意**: 值不按质量排列。有 HD DVD(2)、MiniBD(4)、DVDR(6)、Track(9)。无 WEB-DL/WEB 选项。无 UHD Blu-ray 独立分类。

### 1.5 编码（`codec_sel[5]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265 |
| 7 | AV1 |

**注意**: 值不按顺序排列（1,2,3,4,5,6,7）。编码名称简洁（H.264 非 H.264/AVC/x264）。有 AV1(7)。无 MPEG-4。

### 1.6 音频编码（`audiocodec_sel[5]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 8 | Dolby Atmos |
| 9 | DTS:X |
| 10 | DTS-HD MA |
| 12 | Dolby TrueHD |
| 13 | LPCM |
| 14 | DDP\E-AC-3 |
| 15 | DD/AC-3 |
| 16 | AAC |
| 17 | Other |

**注意**: 值从8开始（8-17），不含1-7。有 Dolby Atmos(8)、DTS:X(9)。**无 DTS（基础）、FLAC、MP3、APE 选项**。DDP 使用反斜杠（DDP\E-AC-3）。

### 1.7 分辨率（`standard_sel[5]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p |
| 6 | Other |

**注意**: 区分 1080p(1) 和 1080i(2)。有 2160p(5)。无 4320p、1440p、540p。

### 1.8 制作组（`team_sel[5]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | FRDS |
| 7 | Mteam |
| 8 | HHCLUB |

**注意**: 包含多个知名站点的官组名（HDS、CHD、MySiLU、WiKi、FRDS、Mteam、HHCLUB），Other(5) 在中间位置。

### 1.9 标签（`tags[5][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | Dolby Vision |
| 11 | 英字 |

**注意**: 值3、9、10缺失（跳过）。有 Dolby Vision(8) 和 英字(11)。与 CarPT 前6个标签完全相同（值1-7），额外增加了 DV 和英字。

---

## 二、发种规则（rules.php）

### 2.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 做种时间不足24小时或故意低速上传将被警告甚至取消上传权限
- 发布者获得双倍上传量
- 违规种子不经提醒直接删除

### 2.2 上传者资格

- 任何人都能发布资源
- 游戏类资源只有上传员及以上等级可自由上传

### 2.3 允许的资源

- 7日内高清预告片
- （其他允许资源参考标准 NexusPHP 规则）

### 2.4 不允许的资源

- 单独的样片（样片应和正片一起上传）
- 重复（dupe）资源
- 垃圾文件

### 2.5 Dupe 规则

- 优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 高清版本使标清版本被视为 dupe
- 按发布组确定优先级
- 不同区域/配音/字幕的 Blu-ray/HD DVD 不视为 dupe
- 每个无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- **断种45日或已发布18个月**以上不受 dupe 约束
- 新版本发布后旧版本保留直至断种

### 2.6 资源打包规则（试行）

- 标清 MV 按 DVD 打包，不允许单曲单独发布
- 5张以上专辑方可打包
- 两年内新专辑可单独发布
- 打包视频须：相同媒介、相同分辨率、相同编码

### 2.7 标题命名规范

- 电影：`[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 电视剧：`[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 音轨：`[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 游戏：`[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]`

### 2.8 例外

- 允许 TV/DSR 体育标清视频
- 允许 <100MB 高清软件/文档、单曲专辑
- 允许 2.0 声道及以上国语/粤语音轨

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "Movies": 401,
  "TV Series": 402,
  "TV Shows": 403,
  "Documentaries": 404,
  "Animations": 405,
  "Music Videos": 406,
  "Sports": 407,
  "HQ Audio": 408,
  "Misc": 409,
  "Game": 410,
  "Program": 411
}
```

### 3.2 媒介（`medium_sel[5]`）

```json
{
  "Blu-ray": 1,
  "HD DVD": 2,
  "Remux": 3,
  "MiniBD": 4,
  "HDTV": 5,
  "DVDR": 6,
  "Encode": 7,
  "CD": 8,
  "Track": 9
}
```

### 3.3 编码（`codec_sel[5]`）

```json
{
  "H.264": 1,
  "VC-1": 2,
  "Xvid": 3,
  "MPEG-2": 4,
  "Other": 5,
  "H.265": 6,
  "AV1": 7
}
```

### 3.4 音频编码（`audiocodec_sel[5]`）

```json
{
  "Dolby Atmos": 8,
  "DTS:X": 9,
  "DTS-HD MA": 10,
  "Dolby TrueHD": 12,
  "LPCM": 13,
  "DDP/E-AC-3": 14,
  "DD/AC-3": 15,
  "AAC": 16,
  "Other": 17
}
```

### 3.5 分辨率（`standard_sel[5]`）

```json
{
  "1080p": 1,
  "1080i": 2,
  "720p": 3,
  "SD": 4,
  "2160p": 5,
  "Other": 6
}
```

### 3.6 制作组（`team_sel[5]`）

```json
{
  "HDS": 1,
  "CHD": 2,
  "MySiLU": 3,
  "WiKi": 4,
  "Other": 5,
  "FRDS": 6,
  "Mteam": 7,
  "HHCLUB": 8
}
```

### 3.7 标签（`tags[5][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR": 7,
  "Dolby Vision": 8,
  "英字": 11
}
```

---

## 四、传道院·PT 特殊注意事项

### 4.1 无 IMDb/PT-Gen 字段

传道院·PT 是已采集站点中**唯一完全没有 IMDb 链接和 PT-Gen 字段**的站点。表单中无 `url`、`pt_gen` 字段，也无相关按钮。适配器需跳过 IMDb/PT-Gen 填写步骤。

### 4.2 无匿名发布字段

表单中无 `uplver` checkbox，无法匿名发布。

### 4.3 音频编码值从8开始

音频编码值范围为 8-17，**不含 1-7**。这与其他站点差异极大。注意 DTS-HD MA(10) 和 Dolby TrueHD(12) 之间跳过了值11。**无基础 DTS、FLAC、MP3、APE**。

### 4.4 无 WEB-DL/WEB 媒介

媒介选项中无 WEB-DL 或 WEB。WEB-DL 来源的资源可能需要归类为 Encode(7) 或 Other。

### 4.5 data-mode=5

使用 `data-mode='5'` 而非常见的 `'4'`，字段名带 `[5]` 后缀。

### 4.6 分类名全英文

所有分类名使用英文（Movies、TV Series 等），非中英双语或纯中文。

### 4.7 标签与 CarPT 高度相似

前6个标签（禁转/首发/DIY/国语/中字/HDR）的值和名称与 CarPT **完全相同**，额外增加了 Dolby Vision(8) 和英字(11)。两个站点可能基于相同的 NexusPHP 定制分支。

### 4.8 Dupe 断种45日

与 CarPT 相同，dupe 断种时限为45日（比多数站点的7-30日更长）。

### 4.9 规则页面极简

rules.php 内容非常简洁，大部分为标准 NexusPHP 默认规则文本，无站点特有的详细规范。

### 4.10 使用 HTTP（非 HTTPS）

站点使用 HTTP 协议（`http://pt.cdy.skin`），tracker 也为 HTTPS（`https://pt.cdy.skin/announce.php`）。注意域名使用 `.skin` TLD。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | 传道院·PT | CarPT | 常见 NexusPHP |
|------|-----------|-------|---------------|
| data-mode | 5 | 4 | 通常 4 |
| IMDb 字段 | **无** | 有 | 有 |
| PT-Gen 字段 | **无** | 有（4种来源） | 通常有 |
| 匿名发布 | **无** | 有 | 有 |
| 类型数量 | 11个（含MV/Game/Program） | 7个 | 通常 8-10 个 |
| 分类名 | 全英文 | 中英双语 | 因站而异 |
| 媒介 | 9个，无WEB-DL | 9个，WEB(2) | 通常 6-10 个 |
| 编码 | 7个，含AV1 | 6个，无AV1 | 通常 5-9 个 |
| 音频编码 | 8个，值8-17，无DTS/FLAC/MP3 | 11个，值1-10 | 通常 6-12 个 |
| 分辨率 | 6个，区分1080p/i | 5个，合并p/i | 通常 5-7 个 |
| 制作组 | 8个（HDS/CHD/MySiLU/WiKi/FRDS/Mteam/HHCLUB） | 5个 | 通常 3-30 个 |
| 标签 | 8个 | 6个 | 通常 3-21 个 |
| MediaInfo | 有（technical_info） | **无** | 通常有 |
| Dupe 断种 | 45日 | 45日 | 通常 7-30 日 |
| 协议 | HTTP | HTTPS | 通常 HTTPS |

---

## 六、适配器实现要点

### 6.1 字段名

```go
TorrentFileField: "file",                    // 标准
TitleField:      "name",                     // 标准
SubtitleField:   "small_descr",              // 标准
DescrField:      "descr",                    // 标准
MediaInfoField:  "technical_info",           // 标准
TypeField:       "type",                     // 标准（无后缀）
MediumField:     "medium_sel[5]",            // 带 [5] 后缀
CodecField:      "codec_sel[5]",             // 带 [5] 后缀
AudioCodecField: "audiocodec_sel[5]",        // 带 [5] 后缀
ResolutionField: "standard_sel[5]",          // 带 [5] 后缀
TeamField:       "team_sel[5]",              // 带 [5] 后缀
TagsField:       "tags[5][]",                // 带 [5] 后缀，数组
```

### 6.2 跳过 IMDb/PT-Gen/匿名

```go
// CDY has no url, pt_gen, or uplver fields
adapter.SkipIMDb = true
adapter.SkipPTGen = true
adapter.SkipAnonymous = true
```

### 6.3 音频编码特殊映射

音频编码值从8开始，缺少常见的 DTS/FLAC/MP3：

```go
func mapAudioCodec(sourceCodec string) int {
    switch {
    case strings.Contains(sourceCodec, "Atmos"):
        return 8
    case strings.Contains(sourceCodec, "DTS:X"):
        return 9
    case strings.Contains(sourceCodec, "DTS-HD MA"):
        return 10
    case strings.Contains(sourceCodec, "TrueHD"):
        return 12
    case strings.Contains(sourceCodec, "LPCM"):
        return 13
    case strings.Contains(sourceCodec, "DDP") || strings.Contains(sourceCodec, "E-AC-3"):
        return 14
    case strings.Contains(sourceCodec, "AC-3") || strings.Contains(sourceCodec, "DD"):
        return 15
    case strings.Contains(sourceCodec, "AAC"):
        return 16
    default:
        return 17 // Other
    }
}
```

### 6.4 WEB-DL 媒介映射

无 WEB-DL 选项，需映射为 Encode(7)：

```go
func mapMedium(sourceMedium string) int {
    switch {
    case containsAny(sourceMedium, "Blu-ray"):
        return 1
    case containsAny(sourceMedium, "Remux"):
        return 3
    case containsAny(sourceMedium, "WEB-DL", "WEBRip", "WEB"):
        return 7 // Encode（无WEB选项）
    // ...
    }
}
```

### 6.5 制作组映射

```go
func mapTeam(sourceTeam string) int {
    switch {
    case strings.Contains(sourceTeam, "HDS"):
        return 1
    case strings.Contains(sourceTeam, "CHD"):
        return 2
    case strings.Contains(sourceTeam, "MySiLU"):
        return 3
    case strings.Contains(sourceTeam, "WiKi"):
        return 4
    case strings.Contains(sourceTeam, "FRDS"):
        return 6
    case strings.Contains(sourceTeam, "Mteam") || strings.Contains(sourceTeam, "MT"):
        return 7
    case strings.Contains(sourceTeam, "HHCLUB"):
        return 8
    default:
        return 5 // Other
    }
}
```

---

*数据来源: upload.php HTML (468行) + rules.php HTML (343行) (2026-04-16)*
*文档创建: 2026-04-16*
