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
| `name` | text | - | 标题（必须全英文，空格用`.`填充） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | 豆瓣/IMDb 链接（**豆瓣优先**） |
| `url_poster` | text | - | 海报图床地址（图片原始链接，非 BBCode） |
| `url_vimages` | textarea | ✓ | 截图图床地址（每行一个 URL，至少 3 张 PNG，白名单图床） |
| `Media_BDInfo` | textarea | ✓ | MediaInfo/BDInfo（文本格式，完整原始信息，须英文） |
| `descr` | textarea (SCEditor) | - | 简介/附加说明（BBCode 富文本编辑器，`startInSourceMode: true`） |
| `uplver` | checkbox | - | 匿名发布（**默认勾选** `checked="checked"`，value="yes"） |
| `offer` | checkbox | - | 候选发布开关（value="yes"，默认不勾选） |

注意：SpringSunday 有**独立的海报 URL**（`url_poster`）和**截图 URL**（`url_vimages`）字段，这是其他站点没有的。`url` 字段**优先使用豆瓣链接**（因为站点采用豆瓣建库）。

### 1.2 质量选择字段

#### 类型（`type`，id=browsecat）— 9个

| 值 | 显示名称 |
|----|----------|
| 501 | Movies(电影) |
| 502 | TV Series(剧集) |
| 503 | Docs(纪录) |
| 505 | TV Shows(综艺) |
| 506 | Sports(体育) |
| 507 | MV(音乐视频) |
| 508 | Music(音乐) |
| 510 | Audio(有声) |
| 509 | Other(其他类型) |

注意：类型值从 501 开始（非标准 401），无动漫独立分类（动漫归类为剧集）。双语显示（中英文）。**注意**：审核脚本 `cat_constant` 中尚未包含 510(Audio)。

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
      "有声": 510
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
    offer: "offer"
    sceditor: "descr"

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
    type_5xx: "类型值从501开始（非标准401），含510(Audio有声)"
    medium_bdrip_dvdrip: "区分BDRip(6)和DVDRip(10)，WEB-DL(7)和WEBRip(8)"
    poster_screenshot_urls: "独立的poster URL和screenshot URL字段"
    massive_blacklist: "黑名单40+小组，不受信组20+"
    wiki_detailed: "Wiki有极其详细的标题/编码/地区/槽位规范"
    neutral_seed_system: "中性种子系统，自动/人工标记"
    anonymous_default_on: "uplver默认勾选"
    offer_checkbox: "候选发布offer复选框，默认不勾选"
    sceditor: "descr使用SCEditor富文本BBCode编辑器"
    seeding_48h_required: "做种不足48小时→警告甚至取消上传权限"
    double_upload_credit: "发布者获得双倍上传量"
    webdl_bitrate_baseline: "WEB-DL码率基准线：1080p≥4000kb/s，2160p≥10000kb/s"
    forced_candidate_tags: "自购/特效/应求/活动标签强制走候选"
    management_only_tags: "驻站/可替代/已取代/中性标签限制管理组使用"
    preserve_slot: "高质量字幕资源可申请共存/替代"
    repost_preserve: "转载不得去掉/篡改原创小组标识、制作说明、媒体信息"
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

### 5.7 核心发布规则（Wiki 补充）

- **做种要求**：做种不足 48 小时 → 警告甚至取消上传权限（比 HDDolby 的 24h 更严格）
- **上传量**：发布者获得**双倍**上传量
- **WEB-DL 码率基准线**：1080p ≥ 4000 kb/s，2160p ≥ 10000 kb/s。低于基准线的同级来源可通过码率进行取代
- **转载要求**：不得随意去掉/篡改原创小组的标识后缀、制作说明、媒体信息；不得随意修改原始文件夹名/文件名
- **保留槽位**：高质量字幕资源（如自译字幕）可申请共存或替代，需在附加说明内填写制作说明
- **标签权限分级**：
  - 强制候选标签：自购(`selfpurchase`)、特效(`subtitlesp`)、应求(`request`)、活动（管理组标签）
  - 限制使用标签：驻站、可替代、已取代、中性 — 仅管理组可设置
- **Dupe 细节**：
  - 解压格式原盘可替代同版本 ISO（例外：3D 原盘和 MGVC 原盘必须 ISO）
  - DIY 原盘：国语音轨+中文字幕 > 仅中文字幕；菜单二次制作 > 仅追加素材
  - Remux：除默片外，非国语版本必须包含国语音轨或中文字幕
- **匿名发布**：`uplver` 默认勾选，转发时无需特别处理
- **候选系统**：`offer` checkbox 可选择发布为候选而非直接进种子区

---

## 审核脚本完整逆向分析

### 脚本信息

| 项目 | 内容 |
|------|------|
| 名称 | SpringSunday-Torrent-Assistant |
| 来源 | Greasyfork #448012 |
| 版本 | 1.1.67 |
| 作者 | SSD |
| 大小 | 2727 行 / 135KB |
| 运行页面 | `details.php*`（详情页）/ `torrents.php*`（列表页） |
| 权限 | GM_xmlhttpRequest / GM_setValue / GM_getValue |
| 外部连接 | movie.douban.com, cmct.xyz, static.hdcmct.org, gifyu.com, imgbox.com, pixhost.to, ptpimg.me, ibb.pics |

> **注意**：该脚本是青蛙/末日/劳改所等站审核脚本的**上游模板**，功能最为完善，含种审模式+图片分辨率检测+Dupe参考+截图体积检查。

### 审核模式

| 模式 | 说明 |
|------|------|
| 普通用户模式（isEditor=false） | 基础校验，红色/绿色提示 |
| 种审员模式（isEditor=true） | 额外60+项激进检查（灰色提示），含图片分辨率验证、Dupe参考、Encode编码限制 |

### 常量映射

#### 分类 (cat_constant)

| ID | 名称 |
|----|------|
| 501 | Movies(电影) |
| 502 | TV Series(剧集) |
| 503 | Docs(纪录) |
| 505 | TV Shows(综艺) |
| 506 | Sports(体育) |
| 507 | MV(音乐视频) |
| 508 | Music(音乐) |
| 509 | Others(其他类型) |

> **注意**：ID 非连续，无 504。分类从 501 起始（非标准 401）。**upload.php 中还有 510(Audio/有声)，但审核脚本 `cat_constant` 中尚未包含。**

#### 媒介 (type_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | Blu-ray | `.bluray`/`.blu-ray`（无 x264/5） |
| 2 | MiniBD | `.minibd` |
| 3 | DVD | `.dvd`（无 x264/5） |
| 4 | Remux | `.remux` |
| 5 | HDTV | `.hdtv` |
| 6 | BDRip | `.bdrip` 或 `bluray` + `x264/5` |
| 7 | WEB-DL | `.web-dl`/`.webdl`/`.web.`（无 x264/5） |
| 8 | WEBRip | `.webrip` 或 `.web.` + `x264/5` |
| 9 | TVRip | `.tvrip` |
| 10 | DVDRip | `.dvdrip` 或 `dvd` + `x264/5` |
| 11 | CD | - |
| 99 | Other | - |

#### 视频编码 (encode_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | H.265/HEVC | `.x265`/`.h265`/`.h.265`/`.hevc` |
| 2 | H.264/AVC | `.x264`/`.h264`/`.h.264`/`.avc` |
| 3 | VC-1 | `.vc-1`/`.vc1` |
| 4 | MPEG-2 | `mpeg2`/`mpeg-2` |
| 5 | AV1 | `.av1` |
| 99 | Other | - |

#### 音频编码 (audio_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | DTS-HD | `.dts-hd`/`.dtshd`/`.dts-x`/`.dts:x`/`.dts.x.` |
| 2 | TrueHD | `.truehd` |
| 3 | DTS | `.dts`（排除 dts-hd 等） |
| 4 | AC-3 | `.ac3`/`.ac-3`/`.dd2`/`.dd5`/`.dd.2`/`.dd.5` |
| 5 | AAC | `.aac` |
| 6 | LPCM | `.lpcm`/`.pcm` |
| 7 | FLAC | `.flac` |
| 8 | APE | - |
| 9 | WAV | - |
| 10 | MP3 | - |
| 11 | E-AC-3 | `.ddp`/`.dd+`/`.e-ac-3`/`.eac3` |
| 12 | OPUS | `.opus` |
| 13 | AV3A | `.av3a` |
| 99 | Other | - |

> **注意**：常量定义中 ID=12 写为 `OUPS`（疑似笔误），但页面显示和匹配逻辑使用 OPUS。AV3A(13) 仅在标题检测中出现，常量映射和页面解析中未定义。

#### 分辨率 (resolution_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | 2160p | `.2160p` 或 `.uhd`(无1080p) 或 `.4k.`（排除 remastered） |
| 2 | 1080p | `.1080p` |
| 3 | 1080i | `.1080i` |
| 4 | 720p | `.720p` |
| 5 | SD | - |
| 99 | 未检测到分辨率 | 默认值（无匹配时） |

#### 地区 (area_constant)

| ID | 名称 |
|----|------|
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

#### 制作组 (group_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | CMCT | `cmct`（需在 cmctv/cmcta 之后匹配） |
| 8 | CMCTA | `cmcta` |
| 9 | CMCTV | `cmctv` |
| 3 | DIY | - |
| 6 | 个人原创 | - |

> **检测顺序**：cmctv(9) → cmcta(8) → cmct(1)，长匹配优先。

### 标题解析算法

```
1. 获取 h1#torrent-name 文本
2. 检测「禁转」→ exclusive=1
3. 标题转小写 → title_lowercase
4. 正则匹配链（按优先级，均使用 .分隔）：
   ├── 媒介(type)：minibd→2, remux→4, bdrip/bluray+x264/5→6, bluray→1,
   │             webrip/web+x264/5→8, webdl→7, tvrip→9, hdtv→5,
   │             dvdrip/dvd+x264/5→10, dvd→3
   ├── 编码(encode)：x265/h265/hevc→1, x264/h264/avc→2, vc1→3, mpeg2→4, av1→5
   ├── 音频(audio)：dts-hd/dts-x→1, truehd→2, lpcm→6, dts→3, ddp/eac3→11,
   │             ac3/dd→4, aac→5, flac→7, opus→12, av3a→13
   ├── 分辨率(resolution)：2160p/uhd/4k→1(排除remastered), 1080p→2, 1080i→3, 720p→4
   ├── 完结检测：`complete` → title_is_complete=true
   └── 制作组(group)：cmctv→9, cmcta→8, cmct→1
```

> **与青蛙站的关键区别**：不可说标题使用 `.` 分隔（0day 命名法），青蛙使用空格分隔。脚本中所有正则均以 `\.` 前缀匹配。

### 豆瓣分类判定算法

```
1. 获取豆瓣"类别"字段
   - 包含"真人秀" → isshow=true
   - 包含"纪录片" → isdoc=true
   - 包含"动画" → isani=true

2. 获取豆瓣"类型"字段（取第一个）
   - "电视剧"：
     isshow → 综艺(505)
     isdoc → 纪录(503)
     否则 → 剧集(502)
   - 非电视剧：
     isdoc → 纪录(503)
     否则 → 电影(501)

3. 豆瓣检测分类 vs 用户选择分类 → 交叉验证
```

### 豆瓣地区判定算法

```
从豆瓣"产地"字段提取国家名，映射到地区ID：
- 中国/中国大陆 → 1(大陆)
- 香港/中国香港 → 2(香港)
- 台湾/中国台湾 → 3(台湾)
- 日本 → 5(日本)
- 韩国 → 6(韩国)
- 印度 → 7(印度)
- 泰国 → 9(泰国)
- 苏联/俄罗斯 → 8(俄国)
- 60+欧美/北美/南美/大洋洲国家 → 4(欧美)
- 其余 → 99(其他)

支持多地区：一个产地可映射到多个地区ID
```

### 中文字幕检测

```
1. 字幕区 img[title="简体中文"] 或 img[title="繁體中文"] 存在 → externalSubtitles=true
2. 字幕区第一个链接含 "chs" 或 "cht" → externalSubtitles=true
3. MediaInfo 含 "字幕.*Chinese" 或 "字幕.*Mandarin" → embeddedSubtitles=true
4. 任一条件满足 → sub_chinese=true
```

### 中文音轨检测

```
1. 非Blu-ray(type≠1)：MediaInfo 含 "音频:.*chinese.*字幕" → true
2. Blu-ray(type=1)：MediaInfo 含 "Audio:\s?(Chinese|Mandarin)" → true
```

### 校验规则（普通用户模式）— 共 30+ 项

#### 标题校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 标题包含空格 | `\s+` | 错误 |
| 2 | 标题含全角符号 | `[\uFF00-\uFFEF]` | 错误 |
| 3 | 标题含中文 | `[\u4e00-\u9fa5]` | 错误 |
| 4 | 标题含禁发小组（绝对） | `CTRLHD\|SmY\|FZHD` | 错误 |
| 5 | 标题含禁发小组（转名） | `FGT\|NSBC\|BATWEB\|...`（40+组） | 错误 |
| 6 | 标题含不受信小组 | `Eleph\|HDH\|HDS\|HDHome\|...` | 错误 |

#### 副标题校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 7 | 副标题为空 | `!subtitle` | 错误 |
| 8 | 副标题含【】 | `/【】/` | 错误（应改为 []） |

#### 字段选择校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 9 | 未选择分类 | `!cat` | 错误 |
| 10 | 未选择格式(媒介) | `!type` | 错误 |
| 11 | 标题媒介与选择不一致 | `title_type !== type` | 错误 |
| 12 | 未选择视频编码 | `!encode` | 错误 |
| 13 | 标题编码与选择不一致 | `title_encode !== encode` | 错误 |
| 14 | 编码=Other（非CMCTA） | `encode===99 && group!==8` | 错误 |
| 15 | 未选择音频编码 | `!audio` | 错误 |
| 16 | 标题音频与选择不一致 | `title_audio !== audio` | 错误 |
| 17 | 音频=Other | `audio===99` | 错误 |
| 18 | 未选择分辨率（非CMCTA） | `!resolution && group!==8` | 错误 |
| 19 | 标题分辨率与选择不一致 | `title_resolution !== resolution` | 错误 |
| 20 | 未选择地区（非CMCTA） | `!area && group!==8` | 错误 |

#### 媒体信息校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 21 | 海报使用防盗链图床 | `tu.totheglory.im` | 错误 |
| 22 | MediaInfo 含广告 | `论坛\|公众号\|微信` | 错误 |
| 23 | Blu-ray 用 MediaInfo 而非 BDInfo | `type===1 && MediaInfo` | 错误 |
| 24 | 未检测到豆瓣或 IMDb（非CMCTA） | `!douban && !imdb` | 错误 |
| 25 | 有 IMDb 但无豆瓣 | `!douban && imdb` | 错误 |
| 26 | 无做种者 | `0个做种者` | 错误 |
| 27 | 媒体信息未解析 | 短格式 === 原始格式 | 错误 |
| 28 | WEB-DL 低于 1080p | `type===7 && (resolution===4\|5)` | 错误 |
| 29 | 媒体信息过短 | `mediainfo_s.length < 50` | 错误 |
| 30 | 标题检测到制作组但未选择 | `title_group && !group` | 错误 |

#### 标签与 MI 交叉校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 31 | 非原盘选了「原生」标签 | `type!==1 && is_bd` | 错误 |
| 32 | 完结但未选「合集」标签 | `title_is_complete` 或副标题含集全/合集 | 错误 |
| 33 | 有中字但未选「中字」标签 | `sub_chinese && !is_chinese` | 错误 |
| 34 | MI 含 DoVi 但未选标签 | MI Dolby Vision + `!is_dovi` | 错误 |
| 35 | 选了 DoVi 但 MI 无 | `is_dovi && !MI Dolby Vision` | 错误 |
| 36 | MI 含 HDR10+ 但未选标签 | MI HDR10+ + `!is_hdr10p` | 错误 |
| 37 | 选了 HDR10+ 但 MI 无 | `is_hdr10p && !MI HDR10+` | 错误 |
| 38 | MI 含 HDR10 但未选标签 | MI HDR10 + `!is_hdr10` | 错误 |
| 39 | 选了 HDR10 但 MI 无 | `is_hdr10 && !MI HDR10` | 错误 |
| 40 | 同时选了 HDR10 和 HDR10+ | `is_hdr10 && is_hdr10p` | 错误 |
| 41 | MI 含 HLG 但未选标签 | MI HLG + `!is_hlg` | 错误 |
| 42 | 选了 HLG 但 MI 无 | `is_hlg && !MI HLG` | 错误 |
| 43 | MI 含 HDR Vivid 但未选标签 | MI HDR Vivid + `!is_hdr_vivid` | 错误 |
| 44 | 选了 HDR Vivid 但 MI 无 | `is_hdr_vivid && !MI HDR Vivid` | 错误 |
| 45 | 活动+无中字 | `is_contest && !is_chinese` | 错误 |
| 46 | 活动+非WEB-DL | `is_contest && type!==7` | 错误 |
| 47 | 附加信息含非致谢内容 | `◎` + 无制作组标记 | 错误 |

#### 截图校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 48 | PNG 截图 < 3 张（白名单例外） | `pngCount < 3` | 错误 |
| 49 | 总截图 < 3 张 | `pngCount + jpgCount < 3` | 错误 |
| 50 | 截图不在白名单图床 | 域名匹配 pichost_list | 错误 |
| 51 | Pixhost 链接格式错误 | 非 `img*.pixhost.to/images/` | 错误 |

#### 豆瓣交叉验证

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 52 | 豆瓣分类与选择不一致 | `douban_cat !== cat` | 错误 |
| 53 | 豆瓣地区与选择不一致 | `!douban_area.includes(area)` | 错误 |
| 54 | 豆瓣动画标签与选择不一致 | `isani && !is_anime` 或 `!isani && is_anime` | 错误 |

**白名单豁免条件**（截图数量检查）：

```
isWhiteList = (area===1 && 标题含官组PterWEB/CatEDU/CMCTV/HHWEB/OurBits)
           || ((type===5||type===7) && resolution===1 && 有HDR标签)
```

### 校验规则（种审员模式）— 额外 60+ 项

#### Dupe 参考类

| # | 规则 | 说明 |
|---|------|------|
| E1 | Movies Anywhere 单独槽位 | `.ma.` + WEB-DL |
| E2 | Netflix 2160P 单独槽位 | `.nf.` + WEB-DL 2160p |
| E3 | 优质字幕小组 Dupe 参考 | Nest/n!ck/lancertony/vandoge/Breeze@Sunny |
| E4 | 优质小组 Dupe 参考 | CiNEPHiLES/FraMeSToR/BLURANiUM/ZQ 等（需保留附加信息） |
| E5 | REMUX Dupe 参考 | Remux 需保留附加信息 |
| E6 | WEB 源优先级（二次元） | CR = B-Global 2160p > B-Global 1080p > AMZN > Other |
| E7 | WEB 源优先级（非二次元） | DSNP/MAX/AMZN > Paramount/iTunes > Netflix > Mytvs 等 |
| E8 | DV P5 Dupe 参考 | dvhe.05（不含 HDR10） |
| E9 | DV P7/P8 Dupe 参考 | dvhe.07/08（含 HDR10） |
| E10 | 外挂字幕 < 内置字幕优先级 | externalSubtitles && !embeddedSubtitles |

#### 禁发/限制类

| # | 规则 | 说明 |
|---|------|------|
| E11 | DV P7 压制禁发 | 兼容性过低 |
| E12 | TV/OVA 番剧原盘禁发 | 仅允许剧场版/电影原盘（含合集） |
| E13 | 音乐分类限制 | 仅官组驻站可发音乐，演唱会归 MV |
| E14 | Encode 2160p 必须用 x265 | type∈{6,8,9,10} && resolution=1 && encode≠1 |
| E15 | Encode 1080p SDR 必须用 x264 | 非动画（或非日本动画） |
| E16 | Encode 1080p+HDR 禁发 | 例外：UHD 日本二次元 + 非外挂中字 |
| E17 | Encode x264 10bit 禁发 | 硬件兼容性差 |
| E18 | BDRip 低于 720p 禁发 | resolution∈{5,99} && type=6 |

#### 内容检查类

| # | 规则 | 说明 |
|---|------|------|
| E19 | 国产豆瓣低分 | 评分<4 + 大陆 |
| E20 | 单集识别 | 副标题含"第X集/期" + 无合集标签 → 免审 |
| E21 | 疑似单集但选了合集 | 副标题含"第X集" + 有合集标签 → 检查 |
| E22 | 非电影 WEB 无中字 | 中性提示 |
| E23 | 非电影 Remux 无中字 | 中性提示 |
| E24 | 非电影动漫无中字 | 中性提示 |
| E25 | 剧集原盘无中字 | 中性提示 |
| E26 | 疑似短剧 | 时长<10min |
| E27 | 应求标签需悬赏链接 | 需提供链接 |
| E28 | PAD 资源检查 | iPad/Pad 相关 |
| E29 | 0 kbps 检测 | MediaInfo 含 0 kbps |
| E30 | 52pt 原盘疑似 Remux | 52pt + 原生标签 |

#### 标题/副标题深入检查

| # | 规则 | 说明 |
|---|------|------|
| E31 | 地区=Other 无豆瓣 | 请人工核对 |
| E32 | 标题无年份 | `\.(18xx-2030)\.` |
| E33 | 标题无音频编码 | title_audio 为空 |
| E34 | 标题无分辨率 | title_resolution 为空 |
| E35 | 标题无视频编码 | title_encode 为空 |
| E36 | MediaInfo 含网址 | URL 正则匹配 |
| E37 | 中文 MediaInfo | "概览"/"概要"开头 |
| E38 | BDMV/BDISO/BDBOX/DVDISO | 应替换为 Blu-ray/DVD |
| E39 | HDR 标题但 MI 无 HDR | 交叉验证 |
| E40 | CC/Criterion 但无 CC 标签 | 标题/副标题交叉验证 |
| E41 | 副标题开头无中文译名 | 1-8 个中文字符 |
| E42 | 2in1 标题 | 需检查 BDInfo 齐全 |
| E43 | DTS-HDMA 标点错误 | 应为 DTS-HD MA |
| E44 | 标题含 ".." | 标点错误 |
| E45 | 副标题含"原盘"但无原生标签 | 交叉验证 |
| E46 | 国配标签无普通话配音 | MI/豆瓣交叉验证 |
| E47 | DV 标题但 MI 无杜比视界 | 交叉验证 |
| E48 | BDInfo 含 SUBtitleS | 需检查 |
| E49 | MediaInfo 空格过少 | <30 个双空格 |
| E50 | 港版原盘区号应用 HKG 而非 HK | 地区码规范 |
| E51 | 大陆蓝光 DIY 不接受 | CHN + DIY |
| E52 | 中字标签但无中文字幕 | 反向验证 |
| E53 | Progressive 但选 1080i | MI 扫描方式 vs 分辨率 |
| E54 | 1080i 视频但标题 1080p | MI vs 标题 |
| E55 | BDInfo Size: 0 | 数据异常 |
| E56 | BDInfo 含多行空格 | 格式问题 |
| E57 | DVDRip 非 SD | 疑似超分 |
| E58 | 附加信息含网址 | 需移除 |
| E59 | 活动标签评分检查 | 显示豆瓣/IMDb 评分 |

#### 可替代类

| # | 规则 | 说明 |
|---|------|------|
| E60 | 音频臃肿 | 高端音频 + 低分辨率 + Encode |
| E61 | PCM 音频 Encode | 音频臃肿 |
| E62 | BHDStudio 原盘 | 质量较差 |

#### 截图分辨率验证（种审员专属）

```
1. 从 MediaInfo/BDInfo 提取视频分辨率（宽×高）
2. 加载截图图片（异步获取实际尺寸）
3. 逐张对比：
   ├── 非原盘：截图分辨率须 ≈ MediaInfo 分辨率
   ├── 原盘 2160p：截图须 3840×2160
   ├── 原盘 1080p：截图须 1920×1080
   ├── DVD：提示人工确认（多比例可能）
   └── Amazon 源：宽高比可能是原始或调整后的
4. 截图体积检查：
   ├── HDR/2160p：≥1800KB
   ├── 1080p/1080i：≥1000KB
   └── 其他：无限制
```

### 禁发制作组

**绝对禁止（无条件）**：
```
CTRLHD, SmY, FZHD
```

**转名/盗用禁止（标题含即报错）**：
```
FGT, ZAX, Ubits, UBWEB, NSBC, BATWEB, GPTHD, DreamHD, BlackTV, CatWEB,
Xiaomi, Huawei, MOMOWEB, DDHDTV, SeeWeb, TagWeb, SonyHD, MiniHD, BitsTV,
ALT, NukeHD, ZeroTV, HotTV, EntTV, GameHD, SeeHD, VeryPSP, DWR, XLMV,
XJCTV, Mp4Ba, GodDramas, FRDS, BeiTai, Ying, VCB-Studio, toothless,
YTS.MX, BMDru, ParkHD, Xunlei, BestWEB, TBMaxUB, 13city
```

**不受信小组（需人工判断）**：
```
Eleph, HDH, HDS(非HDSky), HDHome, HDSWEB, Dream(非DreamRu), DYZ-Movie
```

> **特殊规则**：`Dream` 仅在非原盘类型（BDRip/WEBRip/TVRip/DVDRip）时触发不受信检测。

### 白名单图床

```
cmct.xyz, static.ssdforum.org, static.hdcmct.org,
gifyu.com, imgbox.com, pixhost.to, ptpimg.me, ssdforum.org
```

> **Pixhost 特殊要求**：链接必须为 `img*.pixhost.to/images/` 格式，缩略页链接不合规。

### 豆瓣语言与国配标签交叉验证

```
1. 豆瓣原始语言含"普通话"且不含"粤语" → 不允许使用「国配」标签
   （原始语言已是普通话的国配无意义）
2. 外语片/粤语片包含普通话配音 → 必须使用「国配」标签
3. 粤语片含普通话配音 → 需人工确认是否使用「国配」标签
4. 豆瓣无语言信息 → 提示检查
```

### UI 辅助功能

| 功能 | 说明 | 可用模式 |
|------|------|---------|
| 错误/通过提示框 | 红/绿色背景显示检查结果 | 普通 |
| 种审模式开关 | 复选框切换，GM_setValue 存储 | 普通 |
| 版本自动检查 | 15分钟一次 Greasyfork API | 普通 |
| 一键复制按钮 | MediaInfo/附加信息/截图URL | 普通 |
| 海报悬浮缩略图 | 种子详情页海报改为悬浮小图 | 普通 |
| 隐藏已审/老种/匹配项 | torrents.php 列表过滤按钮 | 测试版 |
| 导出种子列表 | HTML格式导出 | 测试版 |
| 快速回复模板 | 预设16条+自定义评论+导入导出 | 测试版 |
| 监控选择框 | 自动收集错误信息供种审员快速回复 | 测试版 |
| 冻结/反馈按钮 | 种审一键冻结种子 | 测试版 |
| 文件列表对比 | 保存/对比种子内文件变化 | 测试版 |
| 显示副标题 | 相关资源区显示/隐藏副标题 | 测试版 |

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题使用 0day 命名法（. 分隔，非空格）
2. 确保无中文、无全角符号、无空格
3. 确保 【】→ [] 替换
4. BDMV/BDISO/BDBOX → Blu-ray，DVDISO → DVD
5. DTS-HDMA → DTS-HD MA（加空格）
6. 移除 ".." 等重复标点
7. HK → HKG（原盘区码）
8. 移除源站前缀标签（如 [馒头]、[HDArea] 等）
9. 确保 .remastered 不触发 4K 误判
```

### 副标题自动处理

```
1. 禁止为空（必填）
2. 开头建议包含中文译名（1-8个中文字符）
3. 禁止使用【】（应改为 []）
4. 建议格式：中文译名 | 包含内容等
5. 优先从 PT-Gen/豆瓣获取中文名
6. 含"第X集/期全"→ 须选合集标签
7. 含"版原盘"→ 须选原生标签
```

### 质量字段自动选择

```
从源站标题解析（注意使用 . 分隔的正则）：
1. 媒介(type)：
   MiniBD→2, Remux→4, BDRip→6, Blu-ray→1, WEBRip→8,
   WEB-DL→7, TVRip→9, HDTV→5, DVDRip→10, DVD→3, CD→11, Other→99
2. 编码(encode)：
   H.265/HEVC→1, H.264/AVC→2, VC-1→3, MPEG-2→4, AV1→5, Other→99
3. 音频(audio)：按匹配优先级
   DTS-HD/DTS-X→1, TrueHD→2, LPCM→6, DTS→3, E-AC-3/DDP→11,
   AC-3/DD→4, AAC→5, FLAC→7, OPUS→12, AV3A→13, Other→99
4. 分辨率(resolution)：
   2160p/4K/UHD→1, 1080p→2, 1080i→3, 720p→4, SD→5, 未检测到→99
5. 地区(area)：
   根据豆瓣产地字段自动映射（支持多地区）
6. 制作组(group)：
   CMCTV→9, CMCTA→8, CMCT→1, DIY→3, 个人原创→6

注意 remastered 排除在 4K 检测之外
CMCTA(group=8) 豁免分辨率/地区/豆瓣链接必选检查
```

### 标签自动选择

```
1. 中字：MI 字幕语言含 Chinese 或字幕区有简繁中文 → 勾选
2. DoVi：MI 含 Dolby Vision → 勾选
3. HDR10：MI 含 HDR10（非 HDR10+）→ 勾选
4. HDR10+：MI 含 HDR10+ → 勾选（不可与 HDR10 同时选）
5. HLG：MI 含 HLG → 勾选
6. 菁彩 HDR：MI 含 HDR Vivid → 勾选
7. 国配：音频含普通话 + 原始语言非普通话 → 勾选
8. 原生：媒介为 Blu-ray/DVD 且未修改 → 勾选
9. 合集：标题含 complete 或副标题含集全/合集 → 勾选
10. 动画：豆瓣类别含"动画" → 勾选
11. CC：标题/副标题含 Criterion/CC → 勾选
12. 应求：源站标题含应求标记 → 勾选（需悬赏链接）
13. 活动：源站标记 → 勾选（须中字+WEB-DL）
```

### MediaInfo 处理

```
1. 非蓝光原盘用 MediaInfo，蓝光原盘用 BDInfo
2. MediaInfo/BDInfo 须英文（排除"概览"/"概要"开头）
3. MediaInfo 不含广告（论坛/公众号/微信）
4. MediaInfo 不含网址
5. MediaInfo 双空格数量须 ≥ 30
6. 非原盘：短格式 ≠ 原始格式（已解析）
7. BDInfo 须含 Disc Title/Disc Label
8. 截图至少 3 张 PNG（白名单图床）
```

### 豆瓣信息

```
1. 优先使用豆瓣链接（必选），其次 IMDb
2. 豆瓣链接放入 url 字段
3. 豆瓣分类→自动选择 cat
4. 豆瓣产地→自动选择 area（支持多地区）
5. 豆瓣年份→与标题年份交叉验证
6. 豆瓣语言→与国配标签交叉验证
7. 豆瓣动画类别→与动画标签交叉验证
```

### Encode 编码限制规则（重要）

```
自动判断是否允许发布：
1. Encode 2160p → 只接受 x265(HEVC)
2. Encode 1080p + SDR → 只接受 x264(AVC)
   - 例外：日本二维动画允许 x265
3. Encode 1080p + HDR → 禁止发布
   - 例外：UHD 原盘制作的日本二次元 + 非外挂中字
4. Encode x264 10bit → 禁止发布（兼容性差）
5. BDRip < 720p → 禁止发布
6. WEB-DL < 1080p → 禁止发布
```

---

*分析时间：2026-04-19（Wiki+upload.php 更新：2026-04-22）*
*数据来源：upload.php (Playwright) + Wiki (https://wiki.hdcmct.org/zh/Rule/上传规则) + rules.php (Playwright) + SpringSunday-Torrent-Assistant.js v1.1.67 (2727行/135KB)*
