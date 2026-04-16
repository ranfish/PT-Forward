# TLFBits (EastGame) 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | TLFBits (EastGame) |
| 站点地址 | https://pt.eastgame.org |
| 站点框架 | NexusPHP |
| 主题 | BlueGene |
| 官方小组 | TLF HALFCD TeaM、TLF iNT TeaM |
| 特殊规则 | **严格 dupe 规则：TLF 官方组作品具有绝对优先级，发布站使用需特别警告** |

> ⚠️ **重要警告**：此站点的 dupe 规则极为严格——若 TLF 小组已发布某作品，则不允许发布相同及更低质量片源的种子，且站内已有资源会被删除。**一般不建议作为发布站使用。当用户选择此站点做发布站时，必须在 UI 中提醒用户此风险。**

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名，要求0day格式英文标题） |
| `small_descr` | text | - | 副标题（中文名） |
| `url` | text | - | IMDb 链接 |
| `douban_url` | text | - | **豆瓣链接**（与IMDb选填其一） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode，20行） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

注意：TLFBits 有独立的 `douban_url` 字段，是已分析站点中唯一提供豆瓣链接输入框的站点。简介中推荐使用"豆瓣资源下载大师"油猴脚本生成。

### 1.2 质量选择字段

**字段名无 `[4]` 后缀**，使用裸名（如 `medium_sel` 而非 `medium_sel[4]`）。

#### 类型（`type`）— 必填，无 data-mode

| 值 | 显示名称 |
|----|----------|
| 438 | 电影 (Movie) |
| 440 | 电视剧(TV series) |
| 441 | 综艺 (TV Show) |
| 442 | 动漫 (Anime) |
| 443 | 纪录片 (Documentary) |
| 444 | 体育 (Sport) |
| 445 | 音乐视频 (Music Video) |
| 446 | 音乐(Music) |
| 447 | 游戏 (Game) |
| 448 | 软件 (Software) |
| 449 | 资料（E-Learning） |
| 450 | 其它 (Other) |

#### 来源（`source_sel`）— **独有字段**

| 值 | 显示名称 |
|----|----------|
| 16 | 0DAY/Scene |
| 17 | P2P/Non-Scene |

注意：这是已分析站点中唯一区分 Scene/P2P 来源的站点。

#### 媒介（`medium_sel`）— 9个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray/ HD DVD |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Other |
| 10 | UHD Blu-ray |

注意：Blu-ray 和 HD DVD 合并为一个选项(1)。UHD Blu-ray 单独列出(10)。

#### 视频编码（`codec_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H264/x264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H265/x265/HEVC |

注意：H.264 和 H.265 合并原盘/压制写法（H264/x264/AVC），简洁。保留 Xvid(3)。

#### 音频编码（`audiocodec_sel`）— 15个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | WAV |
| 9 | Dolby Digital/AC3 |
| 10 | DTS X |
| 11 | DTS-HD MA |
| 12 | LPCM |
| 13 | Dolby Atmos |
| 14 | Dolby TrueHD |
| 15 | Opus |

注意：包含 DTS:X(10)、Dolby Atmos(13)、LPCM(12)、Opus(15) 等在其他站点少见的编码。AC3 写为 Dolby Digital/AC3(9)。

#### 分辨率（`standard_sel`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | Other |
| 6 | 2160p/4K |
| 7 | 2K |

#### 地区（`processing_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | CN(中国大陆) |
| 2 | US/EU(欧美) |
| 3 | HK/TW(港台) |
| 4 | JP(日) |
| 5 | KR(韩) |
| 6 | OT(其他) |

#### 制作组（`team_sel`）— 仅4个

| 值 | 显示名称 |
|----|----------|
| 1 | TLF HALFCD TeaM |
| 2 | TLF iNT TeaM |
| 5 | Other |
| 8 | 个人原创 |

注意：制作组以 TLF 官方小组为核心，外部组统一选 Other(5)。没有 HDS/CHD 等常见组。

### 1.3 缺失字段

- `tags` — 无标签选择
- `technical_info` — 无 MediaInfo 输入框（写入简介）
- `pt_gen` — 无 PTGen 链接输入框

---

## 二、标题命名规范

来源：`rules.php` → 种子信息

### 2.1 标题规则

- 主标题**尽可能只包含0day格式的英文标题**（实在无法翻译方可使用中文）
- 副标题可以加入中文名字及介绍语
- 标题英文单词之间分隔符只用句点 `.` 或空格，可使用 `/` 转义
- 标题**不允许**以 avi、ts、mkv 等封装格式结尾
- 不要包含广告或求种/续种请求，不要添加描述性词汇

### 2.2 各类型标题格式

| 类型 | 标题格式 | 副标题格式 | 示例标题 |
|------|----------|------------|----------|
| 电影 | 完整英文 Release 目录名 | 中文片名 | `Infernal.Affairs.2002.BDRip.X264.iNT-TLF` / 无间道 |
| 电视剧 | 完整英文 Release 目录名 | 中文片名 (季数 集数) | `Battlestar.Galactica.S01.2004.BD.MiniSD-TLF` / 太空堡垒卡拉狄加 第一季 |
| 音轨 | Artist - Album (Year) [FILEEXT] | 艺术家 - 专辑 | `Enya - And Winter Came (2008) [CD - FLAC - Lossless]` / 恩雅 - 冬季降临 |
| 游戏 | 英文游戏名 | 中文游戏名 | `Vantage.Master.Portable.CloneDVD.CHS-GoV` / 空之轨迹x魔唤精灵4简体中文版 |
| Scene | the full name of the release | 中文名 | `The.Warring.State.2011.CHINESE.DVDRip.XviD-WZW` / 战国 |

### 2.3 简介要求

**必须包含**：
- 元数据（格式、时长、编码、码率、分辨率、语言、字幕等）
- IMDb 链接、豆瓣链接（相关条目存在时）
- 海报/封面、剧情概要、演职人员（豆瓣解析失败时）

**推荐包含**：
- 文件校验信息（MD5/SHA1）
- 影片缩略图（推荐PNG格式）

---

## 三、发布规则

### 3.1 允许的资源

1. TLF 官方小组以及合作组的资源
2. 未解压的 WareZ/MovieZ/GameZ 等 Scene 资源
3. 其他资源（若 TLF 官方小组已发布该片源的压制作品，则不允许发布同质量片源及其压制作品）

### 3.2 黑名单制作组

- **Mp4Ba** — 禁止发布
- **RARBG** — 禁止发布
- **FGT** — 禁止发布

### 3.3 禁止的资源

- 标清低质量：CAM、TC、TS、SCR、DVDSCR、R5、R5.Line
- 标清 upscale 视频
- RealVideo/RMVB/RM/FLV（稀缺资源除外）
- 损坏文件、垃圾文件
- 带水印视频
- Adult Video 纯色情资源、暴力血腥、反人类、涉及政治
- 擦边球资源（写真等）
- **不规范转发**：修改种子文件名、修改种子结构、增删种子内文件
- 压缩包文件
- 线上教育类资源（学科类、技能类培训班）
- 重复资源

### 3.4 ⚠️ 严格 Dupe 规则（核心风险点）

TLFBits 的 dupe 规则以 **TLF 官方小组作品为中心**，具有绝对优先级：

**媒介优先级**：UHD Blu-ray = Blu-ray = HD DVD > WEB-DL = HDTV = DVD > SDTV

**TLF 小组优先级**：
1. TLF 小组发布前，不同小组间作品不构成 dupe；**剧集完结后，不允许再发任何版本单集**
2. **若 TLF 小组已发布某作品**：
   - 不允许发布相同质量及更低质量片源的种子
   - 站内已有的更低/同质量资源会被**删除**
3. 示例（Joker 2019）：
   - TLF 未发布 WEB-DL 版 → 允许任何小组的 WEB-DL/更高媒介
   - TLF 发布了 WEB-DL MiniSD → 禁止发布 WEB-DL 类型资源，但允许 Blu-ray/UHD 及其 Encode
   - TLF 发布了 BDRip → **禁止发布所有更高媒介**（UHD/Bluray/Encode/WEB-DL/HDTV/DVD），全部删除
4. **TLF 小组、TLFBits 合作组作品不受 dupe 规则约束**

### 3.5 资源打包规则

- 2020年起，除电视剧/Scene组0day合集外，**其他合集必须通过候选区经管理许可**，并在副标题标注"【特许发布】"
- 禁止无关联资源打包
- 合集要求：质量相同、介质一致、同一录制组/压制组（Scene/HDB Internal除外）
- 严禁未完结合集，剧集包仅允许按季度打包
- 合集内单集命名一致，仅允许附带 nfo，不允许夹带字幕

### 3.6 其他规则

- 做种时间不足48小时会警告/取消上传权限
- 非TLF小组成员禁止使用TLF作为标识和后缀
- 发布者获得双倍上传量
- TLF 官方作品（MiniSD/iNT）新种免费48小时；种子数<5时免费
- 无 H&R 规则

---

## 四、站点适配器配置参考

```yaml
site:
  id: "eastgame"
  name: "TLFBits"
  alt_name: "EastGame"
  url: "https://pt.eastgame.org"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  publish_warning:
    enabled: true
    severity: "high"
    message: "TLFBits 拥有严格的 dupe 规则：TLF 官方小组已发布的作品会使得同/低质量片源的种子被删除。强烈不建议作为发布站使用。"

  mappings:
    type:
      "电影": 438
      "剧集": 440
      "综艺": 441
      "动漫": 442
      "纪录": 443
      "体育": 444
      "MV": 445
      "音乐": 446
      "游戏": 447
      "软件": 448
      "资料": 449
      "其他": 450

    source_sel:
      "0DAY": 16
      "P2P": 17

    medium_sel:
      "Blu-ray": 1
      "Remux": 3
      "WEB-DL": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Other": 9
      "UHD": 10

    codec_sel:
      "H264": 1
      "VC-1": 2
      "Xvid": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 6

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS": 3
      "MP3": 4
      "OGG": 5
      "AAC": 6
      "Other": 7
      "WAV": 8
      "DD": 9
      "DTS:X": 10
      "DTS-HDMA": 11
      "LPCM": 12
      "Atmos": 13
      "TrueHD": 14
      "Opus": 15

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "Other": 5
      "2160p": 6
      "2K": 7

    processing_sel:
      "大陆": 1
      "欧美": 2
      "港台": 3
      "日本": 4
      "韩国": 5
      "其他": 6

    team_sel:
      "TLF HALFCD": 1
      "TLF iNT": 2
      "Other": 5
      "个人原创": 8

  field_names:
    suffix: ""
    source: "source_sel"
    medium: "medium_sel"
    codec: "codec_sel"
    audiocodec: "audiocodec_sel"
    standard: "standard_sel"
    processing: "processing_sel"
    team: "team_sel"
    anonymous: "uplver"
    douban_url: "douban_url"

  missing_fields:
    - "tags"
    - "technical_info"
    - "pt_gen"

  quirks:
    strict_dupe: "TLF官方组绝对优先级，非TLF组发布后被TLF发布可能被删"
    no_field_suffix: "质量字段名无[4]后缀，使用裸名"
    douban_url: "独有豆瓣链接输入框"
    source_sel: "独有Scene/P2P来源区分字段"
    blacklist_groups: "禁止Mp4Ba/RARBG/FGT小组资源"
    title_english_only: "主标题仅允许0day格式英文标题"
```

---

## 五、发布流水线注意事项

### 5.1 ⚠️ Dupe 风险检查（发布前必须执行）

发布到 TLFBits 前，**必须**检查以下条件：

1. **TLF 官方组是否已发布同作品？** — 搜索站内 TLF HALFCD/TLF iNT 作品
2. **待发布资源的媒介优先级**是否高于 TLF 已发布作品？
   - TLF 已发布 BDRip/BDRip → 禁止发布所有同/低质量媒介
   - TLF 已发布 WEB-DL → 禁止 WEB-DL，允许 Blu-ray/UHD 及其 Encode
3. **剧集是否已完结？** — 完结后禁止发布任何版本单集
4. 合作组作品不受 dupe 约束，但需确认合作关系

### 5.2 来源字段处理

`source_sel` 是 TLFBits 独有字段，需要根据制作组判断：
- Scene 组（如 SPARKS、AMIABLE 等）→ 0DAY/Scene(16)
- P2P/内部组（如 DON、EbP 等）→ P2P/Non-Scene(17)

### 5.3 标题格式化

- 标题必须使用 0day 格式英文标题，不使用中文
- 分隔符使用 `.` 或空格
- 副标题填中文名
- 不要以封装格式（avi/ts/mkv）结尾

### 5.4 制作组映射

仅有4个选项，非 TLF 官方组统一选 Other(5)。个人原创内容选 个人原创(8)。

### 5.5 黑名单过滤

发布前检查制作组是否在黑名单中：
- Mp4Ba → 拒绝发布
- RARBG → 拒绝发布
- FGT → 拒绝发布

### 5.6 豆瓣链接

TLFBits 支持 `douban_url` 字段，转种时应优先填写豆瓣链接（如有），可替代 IMDb。

---

*分析时间：2026-04-16*
*数据来源：https://pt.eastgame.org/upload.php + rules.php*
