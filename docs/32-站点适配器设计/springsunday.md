# 不可说 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 不可说|
| 官方组 | CMCT、CMCTV |
| 站点地址 | https://springsunday.net |
| Wiki 地址 | https://wiki.hdcmct.org |
| 站点框架 | NexusPHP（深度定制） |
| 特殊规则 | **槽位制（Slot System）dupe 规则**，极其详细的发布规范 |

> ⚠️ SpringSunday 的 dupe 规则极其严格——采用**槽位制**（按分辨率+HDR+编码组合定义槽位），每个槽位仅允许一份资源。**不建议作为发布站使用。当用户选择此站点做发布站时，必须在 UI 中提醒用户此风险。**

---

## 一、发布页面表单字段分析

**提交地址**: POST multipart/form-data

**字段名无后缀**（裸名）。

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | 豆瓣/IMDb 链接（**豆瓣优先**） |
| `url_poster` | text | - | 海报图床地址（图片原始链接，非 BBCode） |
| `url_vimages` | text | - | 截图图床地址 |
| `Media_BDInfo` | textarea | - | MediaInfo/BDInfo（文本格式，完整原始信息） |
| `descr` | textarea | ✓ | 简介/附加说明（BBCode） |
| `uplver` | checkbox | - | 匿名发布 |

注意：SpringSunday 有**独立的海报 URL**（`url_poster`）和**截图 URL**（`url_vimages`）字段，这是其他站点没有的。`url` 字段**优先使用豆瓣链接**（因为站点采用豆瓣建库）。

### 1.2 质量选择字段

#### 类型（`type`，id=browsecat）— 8个

| 值 | 显示名称 |
|----|----------|
| 501 | Movies(电影) |
| 502 | TV Series(剧集) |
| 503 | Docs(纪录) |
| 505 | TV Shows(综艺) |
| 506 | Sports(体育) |
| 507 | MV(音乐视频) |
| 508 | Music(音乐) |
| 509 | Other(其他类型) |

注意：类型值从 501 开始（非标准 401），无动漫独立分类（动漫归类为剧集）。双语显示（中英文）。

#### 地区（`source_sel`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | Mainland(大陆) |
| 2 | Hongkong(香港) |
| 3 | Taiwan(台湾) |
| 4 | West(欧美) |
| 5 | Japan(日本) |
| 6 | Korea(韩国) |
| 7 | India(印度) |
| 8 | Russia(俄国) |
| 9 | Thailand(泰国) |
| 99 | Other(其他地区) |

注意：地区字段名为 `source_sel`（非 `processing_sel`），是已分析站点中地区最标准的（含俄国、泰国）。Wiki 中有完整的地区→值映射表（见 §二.2.8）。

#### 媒介（`medium_sel`）— 11个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | MiniBD |
| 3 | DVD |
| 4 | Remux |
| 5 | HDTV |
| 6 | BDRip |
| 7 | WEB-DL |
| 8 | WEBRip |
| 9 | TVRip |
| 10 | DVDRip |
| 11 | CD |
| 99 | Other |

注意：区分 BDRip(6) 和 DVDRip(10)，WEB-DL(7) 和 WEBRip(8)，HDTV(5) 和 TVRip(9)。是最细分的媒介列表之一。

#### 分辨率（`standard_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 2160p |
| 2 | 1080p |
| 3 | 1080i |
| 4 | 720p |
| 5 | SD |
| 99 | Other |

#### 视频编码（`codec_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H.265/HEVC |
| 2 | H.264/AVC |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | AV1 |
| 99 | Other |

注意：H.265 排在 H.264 前面。不区分原盘/压制（标题中区分）。

#### 音频编码（`audiocodec_sel`）— 13个

| 值 | 显示名称 |
|----|----------|
| 1 | DTS-HD |
| 2 | TrueHD |
| 3 | DTS |
| 4 | AC-3 |
| 5 | AAC |
| 6 | LPCM |
| 7 | FLAC |
| 8 | APE |
| 9 | WAV |
| 10 | MP3 |
| 11 | E-AC-3 |
| 12 | OPUS |
| 13 | AV3A |
| 99 | Other |

注意：DTS-HD(1) 不区分 MA/HR，统一为 DTS-HD。E-AC-3(11) 独立选项。含 AV3A(13) 中国自主标准。

### 1.3 标签系统（checkbox 字段，非 select）

SpringSunday 不使用 `tags[]` 复选框组，而是使用**独立的 checkbox 字段**，每个标签是一个独立的表单字段：

| 字段名 | 显示名称 | 说明 |
|--------|----------|------|
| `animation` | 动画 | 标记为动画资源 |
| `exclusive` | 禁转 | 禁止转载 |
| `pack` | 合集 | 合集资源 |
| `untouched` | 原生 | Untouched 原盘 |
| `selfpurchase` | 自购 | 自购资源 |
| `mandarin` | 国配 | 国语配音 |
| `subtitlezh` | 中字 | 中文字幕 |
| `subtitlesp` | 特效 | 特效字幕 |
| `selfcompile` | 自译 | 自译字幕 |
| `dovi` | DoVi | Dolby Vision |
| `hdr10` | HDR10 | HDR10 |
| `hdr10plus` | HDR10+ | HDR10+ |
| `hdrvivid` | 菁彩HDR | HDR Vivid（中国标准） |
| `hlg` | HLG | HLG |
| `cc` | CC | Criterion Collection |
| `3d` | 3D | 3D 视频 |
| `request` | 应求 | 求种应求 |

注意：
- 这是已分析站点中**最独特的标签设计**——每个标签是独立的 checkbox 字段，HTML `name` 各不相同
- HDR 细分为 5 种（DoVi/HDR10/HDR10+/HDR Vivid/HLG），与 HDDolby 类似但字段独立
- 语言/字幕标签独立：`mandarin`（国配）、`subtitlezh`（中字）、`subtitlesp`（特效字幕）
- 来源标签：`selfpurchase`（自购）、`selfcompile`（自译）
- 内容标签：`animation`（动画）、`pack`（合集）、`untouched`（原生）、`cc`（CC）

### 1.4 缺失字段

- `pt_gen` — 无 PTGen 链接输入框
- `nfo` — 无 NFO 文件上传
- `technical_info` — 用 `Media_BDInfo` 替代
- `team_sel` — 无制作组下拉选择（制作组信息写在标题中）
- `tags[]` — 用独立 checkbox 替代

---

## 二、标题命名规范

来源：Wiki (https://wiki.hdcmct.org/zh/Rule/上传规则)

### 2.1 标题格式

**电影**：
```
片名 AKA原名 年份 S##E## 分辨率 地区 HDR 音频通道 音频对象 介质 视频编码-发布组 发布说明
```

**剧集**：
- 单集：`S##E##`
- 单季合集：`S##`
- 连续多季合集：`S##-S##`

### 2.2 介质命名规则

| 类型 | 命名 |
|------|------|
| 原盘 | `Blu-ray` / `UHD Blu-ray` / `3D Blu-ray` / `NTSC DVD9` 等 |
| Remux | `BluRay REMUX` / `UHD BluRay REMUX` 等 |
| Encode | `BluRay`/`UHD BluRay`/`DVDRip`（来源为 DVD 时） |
| WEB-DL | `[平台缩写] WEB-DL`，如 `NF WEB-DL`、`AMZN WEB-DL` |
| WEBRip | `[平台缩写] WEBRip` |
| HDTV | `HDTV` / `UHDTV` |

### 2.3 视频编码命名规则

| 来源类型 | 可用编码 |
|----------|----------|
| 蓝光原盘/Remux | AVC、HEVC、MPEG-2、VC-1 |
| WEB-DL/HDTV | H264、H265、x264、x265、VP9、AV1、AVS+、AVS3 |
| Encode/WEBRip | x264、x265、MPEG-2、VP9、AV1、Xvid、DivX |

区分 H264/x264：以 MediaInfo `Writing library` 为准——含 `x264` 则写 x264，否则写 H264。

### 2.4 HDR 命名

省略=SDR，否则为：`HDR10`、`HDR10+`、`DV`、`DV HDR10`、`DV HDR10+`、`HLG`、`PQ10`、`HDR Vivid`

### 2.5 音频编码命名

- 无损：TrueHD、DTS:X、DTS-HD MA、LPCM、FLAC
- 有损：DTS-HD HR、AAC、AC3/DD、E-AC3/DD+/DDP、DTS、MP3、Opus
- 以**默认音轨**为准，按声道数、编码质量优先级判断

### 2.6 分辨率标准

| 名称 | 最大尺寸 |
|------|----------|
| 2160p (4K) | 4096 × 2160 |
| 1440p (2K) | 2560 × 1440 |
| 1080p | 1920 × 1080 |
| 720p | 1280 × 720 |
| 576p | 1024 × 576 |
| 480p | 854 × 480 |
| SD | ≤ 1024 × 576 |

### 2.7 禁止的标题写法

- 不要使用 BDMV、BDISO、BDBOX、DVDISO → 用 Blu-ray/DVD 替代
- 已完结剧集可加 `Complete` 但禁止加 `TV1-27Fin` 等集数字样
- 区分大小写：1080p 不写 1080P，HEVC 不写 hevc，x264 不写 X264

### 2.8 地区划分（完整映射）

**单地区**：中国大陆、香港、台湾、印度、日本、韩国、泰国、俄国

**欧美**：美国、英国、德国、法国、加拿大、澳大利亚等 50+ 个国家

**Other**：未在以上列表的地区

---

## 三、发布规则

### 3.1 允许的资源

- 高清视频（Blu-ray/Remux/Encode/WEB-DL/HDTV）
- 与高清相关的软件和文档
- 体育类资源
- 经管理组特许的资源
- 其他高清视频

### 3.2 禁止的资源

- 总体积 < 100MB
- 标清 Upscale（AI Upscale 允许但需标明）
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD
- 二次编码（二压，MiniSD 除外）
- DivX/XviD、RealVideo/RMVB、Flash
- 单独样片/预告片
- RAR 压缩文件
- 重复资源
- 禁忌/敏感内容
- 二次添加硬水印
- 垃圾文件
- 游戏软件文档（教育教程类、无意义直播回放）
- 不规范转发
- 单碟 BDMV 不完整/拆分
- **音乐/MV**（演唱会除外，官组/驻站组除外）
- **有声书**（官组/驻站组除外）
- **电子书**
- **番剧原盘**（仅允许剧场版/电影原盘）
- **国版蓝光 DIY**
- **短视频微短剧**

### 3.3 ⚠️ 黑名单制作组（极长）

**禁止的小组**（直接删除+警告）：

| 类别 | 小组 |
|------|------|
| 偷盗/修改后缀 | FGT、NSBC、BATWEB、GPTHD、DreamHD、BlackTV、CatWEB、Xiaomi、Huawei、MOMOWEB、DDHDTV、SeeWeb、TagWeb、SonyHD、MiniHD、BitsTV、CTRLHD、ALT、NukeHD、ZeroTV、HotTV、EntTV、GameHD、ParkHD、Xunlei、BestWEB、TBMaxUB |
| 劣质资源 | SmY、SeeHD、VeryPSP、DWR、XLMV、XJCTV、Mp4Ba、13city、ZAX、FZHD |
| 内容不受欢迎 | GodDramas、toothless、YTSMX |
| 其他 | FRDS、BeiTai、Ying及关联后缀、VCB |
| 不受信站点 | UBits、UBWEB |

**不受信的小组**（需通过候选，可能标为中性和可替代）：

| 类型 | 小组 |
|------|------|
| 压制 | HDH*、HDS*、Eleph*、Dream、BMDru* |
| DIY | HDHome*、HDSky* |
| Remux | Dream*、HDH*、HDS*、DYZ-Movie* |
| WEB | HDH*、HDS* |

注：带 `*` 的不接受新种子，不带 `*` 的要求通过候选发布。

### 3.4 ⚠️ 槽位制 Dupe 规则（核心）

SpringSunday 使用**槽位制**（Slot System），远比标准 HD 站的 dupe 规则复杂：

**总则**：
- 每个槽位仅允许一份资源
- 不受信小组的作品不能取代其他资源
- 替代需主动举报并提供详细理由

**原盘槽位**：不同版本的 Untouched 原盘可共存

**DIY 原盘槽位**：每个分辨率只保留一个版本
- 国配音轨+中文字幕 > 仅中文字幕
- 菜单二次制作 > 仅追加素材
- HDR10+/DoVi P7 > SDR/HDR10

**Remux 槽位**（按分辨率+HDR组合）：
- 1080p SDR
- 2160p SDR
- 2160p HDR10 / DoVi P7 / HDR10+
- 2160p DoVi P8 / HDR10+（Hybrid）— 仅在无 DoVi/HDR10+ 原盘时允许

**Encode 槽位**（按分辨率+编码+HDR组合）：
- 720p SDR x264
- 1080p SDR（x264/x264 10bit/x265 三选一）
- 2160p x265 SDR
- 2160p x265 HDR（DV HDR10/HDR10+、HDR10+、HDR10）
- 特许槽位（全类型无资源时）
- 高质量编码槽（特许共存）

**WEB-DL 槽位**（按分辨率+HDR组合）：
- 1080p SDR
- 2160p SDR
- 1080p/2160p HDR
- 1080p/2160p DV P5
- 性价比槽（高码特许）
- 稀缺资源槽

**WEB-DL 来源分级**：
- 最高优先级（优质源）：MA、Netflix 2160p（独立槽位）、DSNP、MAX、AMZN
- 次高优先级：Paramount+、iTunes、HBO Max、Netflix HD
- 低优先级：无平台源、Mytvs、MyVideo、Hami

**跨类规则**（WEB vs Encode）：
- 已有带中字的 Encode 时，只能发布同样带中字的 WEB-DL，且码率须高 50% 以上或来自优质源
- Encode 可取代非优质源且无其他优点的近似码率 WEB-DL

### 3.5 合集打包规则

**电影**：禁止系列电影合集、DIY/Remux/Encode 合集、私人合集（仅允许发行商官方原盘合集）

**剧集**：允许单季打包，禁止多季打包；集数必须连续，参数一致

**动漫**：允许单季打包；未完结禁止 <50 集小合集

### 3.6 可替代标记

会被标记为"可替代"的资源：
1. 死种（断种 3 月+）
2. 音频臃肿（非 4K 封装 TrueHD/DTS-HD/LPCM 的 Encode）
3. 硬字幕
4. 音轨冗余
5. 未解锁原盘
6. 机翻字幕
7. AI Upscale

---

## 四、站点适配器配置参考

```yaml
site:
  id: "springsunday"
  name: "SpringSunday"
  alt_name: "CMCT"
  url: "https://springsunday.net"
  wiki_url: "https://wiki.hdcmct.org/zh/Rule/上传规则"
  framework: "nexusphp"
  upload_url: "upload.php"

  publish_warning:
    enabled: true
    severity: "critical"
    message: "SpringSunday 使用槽位制 dupe 规则，每个槽位仅允许一份资源。规则极其严格，强烈不建议作为发布站使用。"

  blacklist_groups:
    banned:
      - "FGT"
      - "NSBC"
      - "BATWEB"
      - "GPTHD"
      - "DreamHD"
      - "BlackTV"
      - "CatWEB"
      - "Xiaomi"
      - "Huawei"
      - "MOMOWEB"
      - "DDHDTV"
      - "SeeWeb"
      - "TagWeb"
      - "SonyHD"
      - "MiniHD"
      - "BitsTV"
      - "CTRLHD"
      - "Mp4Ba"
      - "13city"
      - "SmY"
      - "SeeHD"
      - "VeryPSP"
      - "DWR"
      - "FRDS"
      - "BeiTai"
      - "VCB"
      - "GodDramas"
      - "toothless"
      - "YTSMX"
    untrusted:
      - "HDH"
      - "HDS"
      - "Eleph"
      - "Dream"
      - "BMDru"
      - "HDHome"
      - "HDSky"
      - "DYZ-Movie"

  mappings:
    type:
      "电影": 501
      "剧集": 502
      "纪录": 503
      "综艺": 505
      "体育": 506
      "MV": 507
      "音乐": 508
      "其他": 509

    source_sel:
      "大陆": 1
      "香港": 2
      "台湾": 3
      "欧美": 4
      "日本": 5
      "韩国": 6
      "印度": 7
      "俄国": 8
      "泰国": 9
      "其他": 99

    medium_sel:
      "Blu-ray": 1
      "MiniBD": 2
      "DVD": 3
      "Remux": 4
      "HDTV": 5
      "BDRip": 6
      "WEB-DL": 7
      "WEBRip": 8
      "TVRip": 9
      "DVDRip": 10
      "CD": 11
      "Other": 99

    codec_sel:
      "H265": 1
      "H264": 2
      "VC-1": 3
      "MPEG-2": 4
      "AV1": 5
      "Other": 99

    audiocodec_sel:
      "DTS-HD": 1
      "TrueHD": 2
      "DTS": 3
      "AC3": 4
      "AAC": 5
      "LPCM": 6
      "FLAC": 7
      "APE": 8
      "WAV": 9
      "MP3": 10
      "EAC3": 11
      "OPUS": 12
      "AV3A": 13
      "Other": 99

    standard_sel:
      "2160p": 1
      "1080p": 2
      "1080i": 3
      "720p": 4
      "SD": 5
      "Other": 99

    checkboxes:
      animation: "animation"
      exclusive: "exclusive"
      pack: "pack"
      untouched: "untouched"
      selfpurchase: "selfpurchase"
      mandarin: "mandarin"
      subtitlezh: "subtitlezh"
      subtitlesp: "subtitlesp"
      selfcompile: "selfcompile"
      dovi: "dovi"
      hdr10: "hdr10"
      hdr10plus: "hdr10plus"
      hdrvivid: "hdrvivid"
      hlg: "hlg"
      cc: "cc"
      3d: "3d"
      request: "request"

  field_names:
    suffix: ""
    source: "source_sel"
    medium: "medium_sel"
    codec: "codec_sel"
    audiocodec: "audiocodec_sel"
    standard: "standard_sel"
    poster_url: "url_poster"
    screenshot_url: "url_vimages"
    mediainfo: "Media_BDInfo"
    anonymous: "uplver"

  missing_fields:
    - "pt_gen"
    - "nfo"
    - "team_sel"
    - "tags[]"

  quirks:
    slot_based_dupe: "槽位制dupe规则，每个槽位仅一份资源"
    douban_first: "豆瓣链接优先于IMDb（站点采用豆瓣建库）"
    independent_checkboxes: "17个独立checkbox标签字段（非tags[]复选框）"
    hdr_5_variants: "HDR细分5种checkbox：DoVi/HDR10/HDR10+/HDR Vivid/HLG"
    no_team_select: "无制作组下拉，制作组信息写在标题中"
    type_5xx: "类型值从501开始（非标准401）"
    medium_bdrip_dvdrip: "区分BDRip(6)和DVDRip(10)，WEB-DL(7)和WEBRip(8)"
    poster_screenshot_urls: "独立的poster URL和screenshot URL字段"
    massive_blacklist: "黑名单40+小组，不受信组20+"
    wiki_detailed: "Wiki有极其详细的标题/编码/地区/槽位规范"
    neutral_seed_system: "中性种子系统，自动/人工标记"
```

---

## 五、发布流水线注意事项

### 5.1 ⚠️ 槽位制 Dupe 风险

SpringSunday 的 dupe 规则远比其他站点复杂，**每个分辨率+HDR+编码组合定义一个独立槽位**。发布前需检查：
1. 目标槽位是否已被占用
2. 待发布资源是否满足取代条件
3. 是否来自不受信小组

### 5.2 标签字段差异

SpringSunday 不使用 `tags[]` 复选框组，而是 17 个独立的 checkbox 字段。每个字段 `name` 各不相同：
```html
<input type="checkbox" name="dovi" value="1"> DoVi
<input type="checkbox" name="hdr10" value="1"> HDR10
```
提交时需发送 `dovi=1&hdr10=1` 而非 `tags[]=dovi&tags[]=hdr10`。

### 5.3 豆瓣链接优先

`url` 字段优先使用豆瓣链接（因为站点采用豆瓣建库）。优先级：豆瓣 > IMDb > 无。

### 5.4 黑名单检查（极长）

发布前需检查制作组是否在 40+ 个禁止小组中。特别注意：
- **13city** 被列为禁止小组（劣质资源类别）
- FRDS、BeiTai、VCB 也在禁止名单
- HDH*、HDS* 等带星号的不接受新种子

### 5.5 制作组

无下拉选择器，制作组信息直接写在标题中（如 `-CMCT`、`-Pure@CMCT`）。非 CMCT 小组成员禁止使用 CMCT 作为标识和后缀。

### 5.6 媒介细分

区分 BDRip(6) vs DVDRip(10)、WEB-DL(7) vs WEBRip(8)、HDTV(5) vs TVRip(9)。转种时需精确选择。

---

*分析时间：2026-04-16*
*数据来源：https://springsunday.net/upload.php + Wiki (https://wiki.hdcmct.org/zh/Rule/上传规则)*
