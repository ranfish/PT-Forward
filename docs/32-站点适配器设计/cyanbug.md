# 大青虫 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 大青虫|
| 站点地址 | https://cyanbug.net |
| 站点框架 | NexusPHP |
| 主题 | DarkPassion（深色主题） |
| 口号 | 大青虫们在此聚集 |
| 特殊功能 | 认领制度(claim)、签到、21点、青虫娘勋章体系、medium-zoom图片放大 |
| 规则页面 | 论坛帖子"种子信息填写规范与指导" |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 模式系统

使用 `data-mode='4'` 属性控制字段显示，字段名带 `[4]` 后缀。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（**不可包含中文**，中文放副标题） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（`data-pt-gen="url"`） |
| `pt_gen` | text | - | PT-Gen 链接（`data-pt-gen="pt_gen"`） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布 |

**PT-Gen 支持**: 支持4种来源——imdb / douban / bangumi / indienova

**重要**: 标题**不可包含中文**，中文必须放在副标题中。

### 1.3 类型字段（`type`，data-mode=4）— 9个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音轨 |
| 409 | 其他 |

**注意**: 分类名使用中文。含 MV(406)、音轨(408)。无"音乐"分类（仅"音轨"）。

### 1.4 媒介（`medium_sel[4]`）— 12个

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
| 10 | WEB-DL |
| 11 | UHD Blu-ray |
| 12 | Other |

**注意**: 12个媒介是已采集站点中较多的。有 HD DVD(2)、MiniBD(4)、UHD Blu-ray(11)、WEB-DL(10)。区分 Blu-ray(1) 和 UHD Blu-ray(11)。无独立 WEBRip 分类。

### 1.5 编码（`codec_sel[4]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265 |
| 7 | MPEG-4 |

**注意**: 有 MPEG-4(7)。无 AV1。编码名称简洁（H.264 非 H.264/AVC/x264）。

### 1.6 音频编码（`audiocodec_sel[4]`）— 13个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 6 | AAC |
| 7 | Other |
| 8 | DD/AC3 |
| 9 | WAV |
| 10 | TrueHD |
| 11 | DTS-HDMA:X 7.1 |
| 12 | DTS-HDMA |
| 13 | Atmos |
| 14 | LPCM |
| 15 | DTS-HD |

**注意**: 13个音频编码，数量较多。值不连续。细分了 DTS(3)、DTS-HD(15)、DTS-HDMA(12)、DTS-HDMA:X 7.1(11)。有 Atmos(13) 独立选项。无 DDP/E-AC-3、MP3。

### 1.7 分辨率（`standard_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p |

**注意**: 区分 1080p(1) 和 1080i(2)。有 2160p(5)。无 Other 选项。

### 1.8 制作组（`team_sel[4]`）— 18个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | CMCT |
| 4 | WiKi |
| 5 | Other |
| 6 | MTean |
| 7 | PTer |
| 8 | FRDS |
| 9 | Audies |
| 10 | BeiTai |
| 11 | HHWEB |
| 12 | HDC |
| 13 | OurBits |
| 14 | HDH |
| 15 | Pter |
| 16 | LeagueWEB |
| 17 | SharkWEB |
| 18 | ADWeb |

**注意**: 18个制作组，数量较多。含多个知名站点的官组及 WEB 类制作组（LeagueWEB、HHWEB、SharkWEB、ADWeb）。注意 PTer(7) 和 Pter(15) 是两个不同选项。

### 1.9 标签（`tags[4][]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR 10 |
| 8 | Dolby Vision |

**注意**: 值3缺失。HDR 标签显示为 "HDR 10"(7)（含空格），Dolby Vision(8)。无"英字"、"完结"等常见标签。

---

## 二、发种规则（论坛帖子"种子信息填写规范与指导"）

### 2.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 做种时间不足24小时或故意低速上传将被警告甚至取消上传权限
- 发布者获得双倍上传量
- 违规种子不经提醒直接删除
- 如果有违规但有价值的资源，可告知管理组破例允许

### 2.2 上传者资格

- 任何人都能发布资源
- 部分用户需先在候选区提交候选
- 游戏类资源只有上传员及以上等级可自由上传

### 2.3 允许的资源

- 高清视频：Blu-ray/HD DVD 原碟、Remux、HDTV、高清重编码（至少720p）
- 标清视频：来源于高清的标清重编码（至少480p）、DVDR/DVDISO、DVDRip
- 无损音轨+cue、5.1声道及以上音轨
- PC 游戏（原版光盘镜像）
- 7日内高清预告片
- 高清相关软件和文档

### 2.4 不允许的资源

- 总体积 < 100MB
- 标清 upscale、CAM/TC/TS/SCR 等
- RealVideo/RMVB、flv
- 有损 MP3/WMA（< 5.1声道）
- 硬盘版/高压版游戏、RAR 压缩文件
- 重复资源、色情/敏感内容、损坏/垃圾文件

### 2.5 Dupe 规则

- 优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- **动漫类特例**：HDTV 和 DVD 相同优先级
- 保留一个 DVD5 大小的重编码版本
- 不同区域/配音/字幕的 Blu-ray/HD DVD 不视为 dupe
- 每个无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- **断种45日或已发布18个月**以上不受 dupe 约束

### 2.6 标题命名规范

- **标题不可包含中文**，中文放副标题
- 电影：`名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 电视剧：`名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 音轨：`艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 游戏：`名称 [年份] [版本] [发布说明][-发布组名称]`

### 2.7 简介要求

- 电影/电视剧/动漫：必须包含海报/封面，尽可能包含截图、文件详情、演职员
- 体育节目：禁止泄漏比赛结果
- 音乐：必须包含专辑封面和曲目列表

### 2.8 促销规则

种子上传时系统随机促销：
- 10% 概率：50% 下载
- 5% 概率：免费
- 5% 概率：2x 上传
- 3% 概率：50% 下载 & 2x 上传
- 1% 概率：免费 & 2x 上传

**自动促销**：
- 文件总体积 > 20GB 的种子自动成为"免费"
- Blu-ray Disk / HD DVD 原盘自动成为"免费"
- 电视剧等每季第一集自动成为"免费"

**促销时限**：
- 除 2x 上传外，其余促销限时 7 天（自种子发布起）
- 2x 上传无时限
- 所有种子发布 1 个月后自动永久成为 2x 上传

### 2.9 账号保留规则

1. Veteran User 及以上等级永久保留
2. Elite User 及以上等级封存账号后不会被删除
3. 封存账号连续 **400 天**不登录 → 删除
4. 未封存账号连续 **150 天**不登录 → 删除
5. 无流量用户（上传/下载增量都为0 且 从未完成过任何种子）连续 **15 天**不登录，或注册满 **30 天** → 删除

**注意**: 封存400天和未封存150天是大青虫独有的较长保留时限（多数站点为90-180天）。

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "电影": 401,
  "电视剧": 402,
  "综艺": 403,
  "纪录片": 404,
  "动漫": 405,
  "MV": 406,
  "体育": 407,
  "音轨": 408,
  "其他": 409
}
```

### 3.2 媒介（`medium_sel[4]`）

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
  "Track": 9,
  "WEB-DL": 10,
  "UHD Blu-ray": 11,
  "Other": 12
}
```

### 3.3 编码（`codec_sel[4]`）

```json
{
  "H.264": 1,
  "VC-1": 2,
  "Xvid": 3,
  "MPEG-2": 4,
  "Other": 5,
  "H.265": 6,
  "MPEG-4": 7
}
```

### 3.4 音频编码（`audiocodec_sel[4]`）

```json
{
  "FLAC": 1,
  "APE": 2,
  "DTS": 3,
  "AAC": 6,
  "Other": 7,
  "DD/AC3": 8,
  "WAV": 9,
  "TrueHD": 10,
  "DTS-HDMA:X 7.1": 11,
  "DTS-HDMA": 12,
  "Atmos": 13,
  "LPCM": 14,
  "DTS-HD": 15
}
```

### 3.5 分辨率（`standard_sel[4]`）

```json
{
  "1080p": 1,
  "1080i": 2,
  "720p": 3,
  "SD": 4,
  "2160p": 5
}
```

### 3.6 制作组（`team_sel[4]`）

```json
{
  "HDS": 1,
  "CHD": 2,
  "CMCT": 3,
  "WiKi": 4,
  "Other": 5,
  "MTean": 6,
  "PTer": 7,
  "FRDS": 8,
  "Audies": 9,
  "BeiTai": 10,
  "HHWEB": 11,
  "HDC": 12,
  "OurBits": 13,
  "HDH": 14,
  "Pter": 15,
  "LeagueWEB": 16,
  "SharkWEB": 17,
  "ADWeb": 18
}
```

### 3.7 标签（`tags[4][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR 10": 7,
  "Dolby Vision": 8
}
```

---

## 四、大青虫特殊注意事项

### 4.1 标题禁止中文

规则明确要求标题**不可包含中文**，中文必须放在副标题中。适配器需从源标题中分离中文部分到副标题。

### 4.2 音频编码极度细分 DTS 系列

大青虫将 DTS 系列细分为4个级别：
- DTS(3)：基础 DTS
- DTS-HD(15)：DTS-HD（不含 MA）
- DTS-HDMA(12)：DTS-HD Master Audio
- DTS-HDMA:X 7.1(11)：DTS:X 7.1 声道

适配器需精确匹配源音频编码到正确的级别。

### 4.3 18个制作组

制作组数量较多（18个），包含4个 WEB 类制作组（LeagueWEB/HHWEB/SharkWEB/ADWeb）。注意 PTer(7) 和 Pter(15) 是两个不同选项，需区分。

### 4.4 HDR 10 标签含空格

标签 "HDR 10"(7) 包含空格，区别于常见的 "HDR" 或 "HDR10"。

### 4.5 媒介含 UHD Blu-ray 和 WEB-DL

12个媒介中包含 UHD Blu-ray(11) 和 WEB-DL(10)，区分标准 Blu-ray 和 UHD。

### 4.6 无 AV1 编码

视频编码选项中无 AV1。

### 4.7 无独立 DDP/E-AC-3

音频编码中无 Dolby Digital Plus / E-AC-3 选项，WEB-DL 资源常见此编码需映射为 Other(7)。

### 4.8 分辨率无 Other

分辨率仅5个选项，无 Other，无法适配 4320p/1440p/540p 等非常见分辨率。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | 大青虫 | CarPT | 传道院·PT |
|------|--------|-------|-----------|
| data-mode | 4 | 4 | 5 |
| 标题规则 | **禁止中文** | 无特殊 | 无特殊 |
| IMDb/PT-Gen | 有（4种来源） | 有（4种来源） | **无** |
| 类型数量 | 9个 | 7个 | 11个 |
| 媒介 | 12个（最多之一） | 9个 | 9个 |
| 编码 | 7个，无AV1 | 6个，无AV1 | 7个，有AV1 |
| 音频编码 | 13个，DTS细分4级 | 11个 | 8个，值8-17 |
| 分辨率 | 5个，区分p/i，无Other | 5个，合并p/i | 6个，区分p/i |
| 制作组 | 18个（最多之一） | 5个 | 8个 |
| 标签 | 7个 | 6个 | 8个 |
| Dupe 断种 | 45日 | 45日 | 45日 |

---

## 六、适配器实现要点

### 6.1 标题中文分离

```go
func splitTitleChinese(title string) (mainTitle, subtitle string) {
    // Separate Chinese characters into subtitle
    // Keep only non-Chinese in main title
    // e.g. "蝙蝠侠 The Dark Knight 2008 720p" →
    //   main: "The Dark Knight 2008 720p"
    //   subtitle: "蝙蝠侠"
}
```

### 6.2 DTS 音频精确匹配

```go
func mapAudioCodec(sourceCodec string) int {
    switch {
    case strings.Contains(sourceCodec, "DTS:X") || strings.Contains(sourceCodec, "DTS-HD MA:X"):
        return 11  // DTS-HDMA:X 7.1
    case strings.Contains(sourceCodec, "DTS-HD MA") || strings.Contains(sourceCodec, "DTS-HDMA"):
        return 12  // DTS-HDMA
    case strings.Contains(sourceCodec, "DTS-HD"):
        return 15  // DTS-HD (non-MA)
    case strings.Contains(sourceCodec, "DTS"):
        return 3   // DTS base
    case strings.Contains(sourceCodec, "Atmos") || strings.Contains(sourceCodec, "Dolby Atmos"):
        return 13
    // ...
    }
}
```

### 6.3 制作组映射（18个）

```go
func mapTeam(sourceTeam string) int {
    switch {
    case strings.Contains(sourceTeam, "HDS"):
        return 1
    case strings.Contains(sourceTeam, "CHD"):
        return 2
    case strings.Contains(sourceTeam, "CMCT"):
        return 3
    case strings.Contains(sourceTeam, "WiKi"):
        return 4
    case strings.Contains(sourceTeam, "MTean"):
        return 6
    case strings.Contains(sourceTeam, "PTer"):
        return 7
    case strings.Contains(sourceTeam, "FRDS"):
        return 8
    case strings.Contains(sourceTeam, "Audies"):
        return 9
    case strings.Contains(sourceTeam, "BeiTai"):
        return 10
    case strings.Contains(sourceTeam, "HHWEB"):
        return 11
    case strings.Contains(sourceTeam, "HDC"):
        return 12
    case strings.Contains(sourceTeam, "OurBits"):
        return 13
    case strings.Contains(sourceTeam, "HDH"):
        return 14
    case strings.Contains(sourceTeam, "Pter"):
        return 15
    case strings.Contains(sourceTeam, "LeagueWEB"):
        return 16
    case strings.Contains(sourceTeam, "SharkWEB"):
        return 17
    case strings.Contains(sourceTeam, "ADWeb"):
        return 18
    default:
        return 5 // Other
    }
}
```

---

*数据来源: upload.php HTML + rules.php + 论坛帖子 topicid=30 (2026-04-22)*
*文档更新: 2026-04-22*
