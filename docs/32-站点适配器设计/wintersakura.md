# 冬樱 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 冬樱|
| 站点地址 | https://wintersakura.net |
| 站点框架 | NexusPHP（深度定制） |
| 口号 | A little Private Sharing Website |
| 特殊功能 | 分集/合集双分类、自制组体系（WS/SakuraWEB/SakuraSUB/WScode/Sakura Academic）、种子保护（转载权限）、custom_fields 自定义字段 |
| 规则页面 | rules.php |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 模式系统

使用 `data-mode='4'` 属性控制字段显示，字段名带 `[4]` 后缀。同时存在 `tags[5][]` 和 `custom_fields[5]` 用于 mode=5 相关功能。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（英文0day命名法） |
| `small_descr` | text | - | 副标题（中文名等） |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo |

**注意**: 无 `uplver`（匿名发布）字段。

### 1.3 类型字段（`type`，data-mode=4）— 13个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/剧集(分集) |
| 403 | TV Shows/综艺(合集) |
| 406 | Music Videos/音乐MV |
| 407 | Sports/体育 |
| 408 | HQ Audio/无损音乐 |
| 409 | Misc/其他 |
| 410 | Documentaries/纪录片 |
| 413 | Animation series/动漫剧集(分集) |
| 414 | TV Series/剧集(合集) |
| 418 | TV Shows/综艺(分集) |
| 422 | Animation films/动漫电影 |
| 423 | Animation series/动漫剧集(合集) |

**独特设计**: 分类按**分集/合集**细分——剧集有分集(402)/合集(414)，综艺有合集(403)/分集(418)，动漫剧集有分集(413)/合集(423)。这是已采集站点中唯一的分集/合集双分类模式。含动漫电影(422)独立分类。

### 1.4 媒介（`medium_sel[4]`）— 14个

| 值 | 显示名称 |
|----|----------|
| 10 | UHD Blu-ray |
| 11 | UHD Blu-ray/DIY |
| 12 | Blu-ray |
| 13 | Blu-ray/DIY |
| 14 | Remux |
| 15 | Encode |
| 16 | HDTV |
| 17 | DVDR |
| 18 | CD |
| 21 | WEB-DL |
| 22 | SACD |
| 23 | HD DVD |
| 24 | 3D Blu-ray |
| 25 | Webrip |

**注意**: 值从10开始（非1）。区分标准版和 DIY 版：UHD Blu-ray(10) vs UHD Blu-ray/DIY(11)、Blu-ray(12) vs Blu-ray/DIY(13)。有 SACD(22)、3D Blu-ray(24)、Webrip(25)。无 Other 选项。

### 1.5 编码（`codec_sel[4]`）— 9个

| 值 | 显示名称 |
|----|----------|
| 6 | H264/AVC |
| 7 | x265 |
| 8 | x264 |
| 9 | H265/HEVC |
| 10 | VC-1 |
| 11 | MPEG-2 |
| 13 | Other |
| 15 | ProRes |
| 16 | AV1 |

**独特设计**: 区分编码标准名和编码器名——H264/AVC(6) vs x264(8)、H265/HEVC(9) vs x265(7)。有 ProRes(15)（专业编码）。值不连续。

### 1.6 音频编码（`audiocodec_sel[4]`）— 21个

| 值 | 显示名称 |
|----|----------|
| 8 | DTS-HDMA |
| 9 | DTS-HDMA:X 7.1 |
| 10 | TrueHD Atmos |
| 11 | PCM |
| 12 | TrueHD |
| 13 | DTS |
| 14 | LPCM |
| 15 | FLAC |
| 16 | APE |
| 17 | MP3 |
| 18 | OGG |
| 19 | AAC |
| 20 | AC3/DD |
| 21 | Other |
| 22 | DTS-HD HR |
| 23 | WAV |
| 24 | DSD |
| 25 | Dolby Digital Plus |
| 26 | Dolby Digital Plus Dolby Atmos |
| 27 | Opus |

**注意**: 21个音频编码，**已采集站点中最多**。细分了 Atmos 层级：TrueHD Atmos(10) vs DDP Atmos(26)。有 DTS-HD HR(22)、DSD(24)、OGG(18)、Opus(27)、ProRes(15) 等罕见编码。

### 1.7 分辨率（`standard_sel[4]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 2K/1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K/2160p |
| 6 | 8K/4320p |

**注意**: 有 8K/4320p(6)。1080p 显示为 "2K/1080p"(1)。区分 1080p 和 1080i。

### 1.8 制作组（`team_sel[4]`）— 19个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 5 | Other/其他制作组或转发资源 |
| 6 | CMCT |
| 7 | FRDS |
| 8 | PTer |
| 9 | BHD |
| 11 | WS/冬樱原盘DIY小组 |
| 12 | Sakura SUB/冬樱字幕组 |
| 13 | Mteam |
| 14 | HDC |
| 15 | ttg |
| 16 | WScode/冬樱重编码及Remux组 |
| 17 | tjupt |
| 18 | Sakura WEB/冬樱web制作组 |
| 19 | Sakura Academic |

**注意**: 19个制作组（已采集站点最多之一）。含5个本站官组：WS/DIY(11)、Sakura SUB(12)、WScode(16)、Sakura WEB(18)、Sakura Academic(19)。非本站小组成员**禁止使用 WinterSakura/WS 作为标识和后缀**。

### 1.9 标签（`tags[4][]`/`tags[5][]`）— 22个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR10 |
| 8 | 原创字幕 |
| 9 | 特效字幕 |
| 10 | HDR10+ |
| 11 | HDR+杜比视界 |
| 12 | 粤语 |
| 13 | SakuraSUB |
| 14 | SakuraWEB |
| 15 | SakuraAcademic |
| 16 | 独占 |
| 17 | 豆瓣电影 Top 250 |
| 18 | IMDb Top 250 |
| 19 | 标准收藏版 |
| 20 | WScode |
| 21 | 子供向 |
| 22 | 原生原盘 |

**注意**: 22个标签（已采集站点最多之一）。HDR 细分为3级：HDR10(7)/HDR10+(10)/HDR+杜比视界(11)。有官组标签(SakuraSUB/SakuraWEB/SakuraAcademic/WScode)、排行榜标签(豆瓣Top 250/IMDb Top 250)、标准收藏版(19)、子供向(21)、原生原盘(22)。

### 1.10 自定义字段（`custom_fields[5]`）

独有转载权限和学习级别自定义字段：

**`custom_fields[5][1][]`**（多选 checkbox + 图标）：

| 值 | 含义 |
|----|------|
| 冬樱原创资源 | WinterSakura 原创资源 |
| 禁止转载 | 种子保护：禁止转载 |
| 72小时内禁转 | 种子保护：72小时内禁止转载 |
| 24小时内禁转 | 种子保护：24小时内禁止转载 |
| 允许转载 | 允许被转载 |
| 零基础 | 学习要求：零基础/所有年龄 |
| 中级 | 学习要求：中级/建议有基础 |
| 高级 | 学习要求：高级/仅限专家 |

**`custom_fields[5][2]`**：文本输入字段（用途待确认）

---

## 二、发种规则（rules.php）

### 2.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 做种时间不足24小时或故意低速上传将被警告甚至取消上传权限
- 发布者获得双倍上传量
- **非 WinterSakura 小组成员禁止使用 WinterSakura/WS 作为标识和后缀**
- **转载资源不得随意去掉/篡改原创小组的标识后缀，甚至张冠李戴**

### 2.2 上传者资格

- 任何人都能发布资源
- **低于 Crazy User** 用户需先在候选区提交候选
- 游戏类资源只有上传员及以上等级可自由上传

### 2.3 允许的资源

- 高清视频（BD/Remux/Encode/WEB-DL/HDTV 等）
- 标清视频（需管理组允许）
- 标准允许的音频、预告片等

### 2.4 禁止的资源

- **非官方的往季剧集、往季动画分集**
- CAM/TC/TS/SCR 等低质量视频
- 单独的样片
- 重复（dupe）资源
- **剧集单季打包后禁止发布单集**
- **黑名单制作组**：FGT、Hao4K、RARBG、Mp4Ba
- 禁止转发其他PT站注明禁转的资源
- 不规范的转发（修改文件名/结构/增删文件）
- 单碟 BDMV 结构不完整、拆分发布
- **不得发布既不包含内封/外挂中文字幕又不包含中文音轨的影视资源**
- 外挂中文字幕应在24小时内上传至字幕区

### 2.5 Dupe 规则

- 优先级：UHD Blu-ray/Blu-ray/Remux/Encode/WEB-DL > UHDTV > HD DVD > HDTV > DVD > TV
- 超高清/高清版本使标清被视为 dupe
- 相同媒介+分辨率原则上只保留一个版本，优先保留有中文字幕的版本
- **不同版本不视为 dupe**：加长版/影院版/家庭版/CC版/4K重制版
- 不同配音的蓝光原盘不视为 dupe，**但纯字幕 DIY 的蓝光原盘视为 dupe**，不含中文字幕的 DIY 不允许发布
- **WinterSakura 制作组及合作制作组不受 dupe 规则约束**
- 断种45日或已发布18个月以上不受 dupe 约束

### 2.6 标题命名规范

- 转载资源保持所有原始文件及文件名称不变
- **影视、动漫类别标题采用英文0day命名法**
- 副标题包括中文片名等资源信息
- **不允许在标题、副标题填写重复信息**
- 电影/纪录片/动漫：`完整英文 年份 片源 分辨率 视频编码 音频编码-制作/压制组`
- 电视剧：`完整英文 季集 年份 片源 分辨率 视频编码 音频编码-制作/压制组`
- 副标题：`中文片名 (季数 集数) [字幕、音轨等信息] "一句话介绍"`

### 2.7 资源打包规则

- 尚未完结的剧集不得发布跨季或多季打包合集
- 打包视频须：相同媒介、相同分辨率、相同编码、相同制作组
- 当剧集单季打包后禁止发布单集

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "Movies": 401,
  "TV Series-分集": 402,
  "TV Shows-合集": 403,
  "Music Videos": 406,
  "Sports": 407,
  "HQ Audio": 408,
  "Misc": 409,
  "Documentaries": 410,
  "Animation series-分集": 413,
  "TV Series-合集": 414,
  "TV Shows-分集": 418,
  "Animation films": 422,
  "Animation series-合集": 423
}
```

### 3.2 媒介（`medium_sel[4]`）

```json
{
  "UHD Blu-ray": 10,
  "UHD Blu-ray/DIY": 11,
  "Blu-ray": 12,
  "Blu-ray/DIY": 13,
  "Remux": 14,
  "Encode": 15,
  "HDTV": 16,
  "DVDR": 17,
  "CD": 18,
  "WEB-DL": 21,
  "SACD": 22,
  "HD DVD": 23,
  "3D Blu-ray": 24,
  "Webrip": 25
}
```

### 3.3 编码（`codec_sel[4]`）

```json
{
  "H264/AVC": 6,
  "x265": 7,
  "x264": 8,
  "H265/HEVC": 9,
  "VC-1": 10,
  "MPEG-2": 11,
  "Other": 13,
  "ProRes": 15,
  "AV1": 16
}
```

### 3.4 音频编码（`audiocodec_sel[4]`）

```json
{
  "DTS-HDMA": 8,
  "DTS-HDMA:X 7.1": 9,
  "TrueHD Atmos": 10,
  "PCM": 11,
  "TrueHD": 12,
  "DTS": 13,
  "LPCM": 14,
  "FLAC": 15,
  "APE": 16,
  "MP3": 17,
  "OGG": 18,
  "AAC": 19,
  "AC3/DD": 20,
  "Other": 21,
  "DTS-HD HR": 22,
  "WAV": 23,
  "DSD": 24,
  "DDP": 25,
  "DDP Atmos": 26,
  "Opus": 27
}
```

### 3.5 分辨率（`standard_sel[4]`）

```json
{
  "2K/1080p": 1,
  "1080i": 2,
  "720p": 3,
  "SD": 4,
  "4K/2160p": 5,
  "8K/4320p": 6
}
```

### 3.6 制作组（`team_sel[4]`）

```json
{
  "HDS": 1,
  "CHD": 2,
  "Other": 5,
  "CMCT": 6,
  "FRDS": 7,
  "PTer": 8,
  "BHD": 9,
  "WS": 11,
  "Sakura SUB": 12,
  "Mteam": 13,
  "HDC": 14,
  "ttg": 15,
  "WScode": 16,
  "tjupt": 17,
  "Sakura WEB": 18,
  "Sakura Academic": 19
}
```

### 3.7 标签（`tags[4][]`/`tags[5][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR10": 7,
  "原创字幕": 8,
  "特效字幕": 9,
  "HDR10+": 10,
  "HDR+杜比视界": 11,
  "粤语": 12,
  "SakuraSUB": 13,
  "SakuraWEB": 14,
  "SakuraAcademic": 15,
  "独占": 16,
  "豆瓣电影 Top 250": 17,
  "IMDb Top 250": 18,
  "标准收藏版": 19,
  "WScode": 20,
  "子供向": 21,
  "原生原盘": 22
}
```

---

## 四、WinterSakura 特殊注意事项

### 4.1 分集/合集双分类

分类按分集/合集细分。适配器需判断资源是单集还是整季来选择正确分类。**单季打包后禁止再发布单集**。

### 4.2 编码标准 vs 编码器

区分编码标准名和编码器名：
- H264/AVC(6) vs x264(8)：标准 vs 编码器
- H265/HEVC(9) vs x265(7)：标准 vs 编码器

适配器需根据源资源判断选择哪个（原盘用标准名，重编码用编码器名）。

### 4.3 DIY 媒介

区分标准媒介和 DIY 版本：UHD Blu-ray(10) vs UHD Blu-ray/DIY(11)、Blu-ray(12) vs Blu-ray/DIY(13)。**不含中文字幕的 DIY 不允许发布**。

### 4.4 黑名单制作组

禁止发布以下制作组的资源：**FGT、Hao4K、RARBG、Mp4Ba**。

### 4.5 官组保护

- 非本站小组成员禁止使用 WinterSakura/WS 标识
- WinterSakura 制作组不受 dupe 规则约束
- 已有官方同优先级资源不允许再上传其他小组资源

### 4.6 中文字幕要求

不得发布既不包含内封/外挂中文字幕又不包含中文音轨的影视资源。外挂中文字幕应在24小时内上传至字幕区。

### 4.7 转载权限自定义字段

`custom_fields[5][1][]` 控制转载权限：禁止转载/72小时内禁转/24小时内禁转/允许转载。适配器转发时需尊重源站转载权限。

### 4.8 0day 命名法

影视、动漫标题必须使用英文0day命名法，副标题使用中文。

### 4.9 无匿名发布

表单中无 `uplver` 字段。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | WinterSakura | 常见 NexusPHP |
|------|-------------|---------------|
| 分集/合集 | **双分类**（6对分集/合集） | 单分类 |
| 媒介 | 14个，含DIY区分 | 通常 6-10 个 |
| 编码 | 9个，区分标准名/编码器名 | 通常 5-7 个 |
| 音频编码 | **21个（最多）** | 通常 6-12 个 |
| 制作组 | 19个（含5个官组） | 通常 3-30 个 |
| 标签 | 22个（HDR细分3级+排行榜） | 通常 3-21 个 |
| 自定义字段 | 转载权限+学习级别 | 无 |
| 匿名发布 | **无** | 有 |
| 黑名单 | FGT/Hao4K/RARBG/Mp4Ba | 因站而异 |
| 官组保护 | **不受dupe约束** | 通常无 |
| 中文字幕 | **强制要求** | 因站而异 |

---

## 六、适配器实现要点

### 6.1 分集/合集分类选择

```go
func mapTypeWithEpisode(category string, isPack bool) int {
    switch {
    case category == "TV Series" && !isPack:
        return 402  // 分集
    case category == "TV Series" && isPack:
        return 414  // 合集
    case category == "TV Shows" && isPack:
        return 403  // 合集
    case category == "TV Shows" && !isPack:
        return 418  // 分集
    case category == "Animation" && !isPack:
        return 413  // 动漫剧集分集
    case category == "Animation" && isPack:
        return 423  // 动漫剧集合集
    case category == "Movies":
        return 401
    // ...
    }
}
```

### 6.2 编码选择（标准名 vs 编码器名）

```go
func mapCodec(codec string, isEncode bool) int {
    if isEncode {
        switch {
        case strings.Contains(codec, "264"):
            return 8   // x264
        case strings.Contains(codec, "265"):
            return 7   // x265
        }
    }
    switch {
    case strings.Contains(codec, "264"):
        return 6   // H264/AVC
    case strings.Contains(codec, "265") || strings.Contains(codec, "HEVC"):
        return 9   // H265/HEVC
    case strings.Contains(codec, "AV1"):
        return 16
    // ...
    }
}
```

### 6.3 黑名单过滤

```go
var blacklistTeams = []string{"FGT", "Hao4K", "RARBG", "Mp4Ba"}

func isBlacklisted(team string) bool {
    for _, bl := range blacklistTeams {
        if strings.Contains(team, bl) { return true }
    }
    return false
}
```

### 6.4 转载权限

转发时根据源站信息设置自定义字段：

```go
// If source allows reposting
customFields["custom_fields[5][1][]"] = "允许转载"
```

---

*数据来源: upload.php HTML (488行) + rules.php HTML (445行) (2026-04-16)*
*文档创建: 2026-04-16*
