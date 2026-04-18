# OKPT 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | OKPT|
| 站点地址 | https://www.okpt.net |
| 站点框架 | NexusPHP |
| 特殊规则 | 双分类模式（影视mode=4 / 音乐mode=5）、29个制作组、严格标题格式、黑名单制作组 |

---

## 一、发种规范

### 1.1 标题格式

**电影**：
```
英文名 年代 分辨率 介质 编码 音轨编码 制作组
```
各项以空格分隔，制作组前以英文短横线 `-` 连接。

示例：`The Dark Knight Rises 2012 UHD BluRay REMUX 2160p HEVC DTS-HD MA5.1 2Audio-CHD`

**电视剧（完结）**：
```
英文名 年代 S## Complete 分辨率 介质 编码 音轨编码 制作组
```

**电视剧（分集）**：
```
英文名 年代 S##E##-E## 分辨率 介质 编码 音轨编码 制作组
```

**补充规则**：
- 编码前可添加 HDR、EDR、DV 等标识
- 音轨编码前可添加 `2Audio`/`4Audio` 注明音轨条数
- 副标题使用英文方括号 `[ ]`，不用中文括号

### 1.2 副标题

必须包含：中文名（必选），可选：演员、音轨/字幕信息等。

示例：`蝙蝠侠：黑暗骑士崛起/黑暗骑士：黎明升起(台) [UHD原盘制作/次世代国语/国配简繁双语特效四字幕]`

### 1.3 简介格式（严格顺序）

1. 转载来源（转载资源首行必须标注）
2. 一张清晰海报
3. 影视、演员、豆瓣链接等信息（通过 PTGen 获取）
4. MediaInfo / BDInfo 信息
5. 三张以上视频截图

### 1.4 MediaInfo 要求

- **必填项** — 非原盘填 MediaInfo，原盘填 BDInfo
- 必须使用**英文状态**获取（非中文）
- 转载资源不得删减源站简介

### 1.5 转载出处规则

转载出处应根据**制作小组**决定（发布源站决定），而非根据从哪个站转载来决定。

例如：资源后缀为 WiKi，从 MT 转载 → 应说明转载于 TTG（WiKi 的源站），而非 MT。

### 1.6 允许/禁止的资源

**允许**：高清/标清视频、完整原盘、Remux、Encode、HDTV、WEB-DL、DVD 等

**禁止**：
- 标清 upscale 视频
- 枪版（CAM、TC、TS）、SCR、DVDSCR、R5
- RealVideo/RMVB/FLV
- 单独 Sample 样片
- 损坏文件、垃圾文件
- 含论坛水印的资源
- "滚雪球"形式发布（未完结剧集只能发增量包）
- 完结打包后禁止再发单集

**禁止的制作组**：FGT、RARBG、Mp4Ba、xiaomi、huawei、BlackTV、BitsTV、DreamHD、Hao4K

### 1.7 重复判定

暂无重复判定 — 允许多小组的同类型资源共存。仅完全相同的资源不允许。

### 1.8 标签使用规则

- 转载内容不得勾选"禁转"
- 转载内容或已在别处发布不得勾选"首发"
- 转载内容或个人作品不得勾选"官方"

---

## 二、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 2.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `descr` | textarea | ✓ | 简介（BBCode，严格顺序） |
| `technical_info` | textarea | ✓ | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

注意：OKPT **无** `pt_gen` 字段和 `nfo` 字段。

### 2.2 质量选择字段

OKPT 有两套分类模式，通过 `data-mode` 区分：
- **mode=4**：影视/综合类（14个分类）
- **mode=5**：音乐/写真类（6个分类）

两套模式共享大部分质量字段，但分类和标签不同。

#### 类型 — 影视模式（`type` mode=4）

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺/真人秀 |
| 404 | 纪录片 |
| 405 | 动漫/动画 |
| 407 | 体育 |
| 409 | 其它 |
| 413 | 游戏 |
| 431 | 软件 |
| 432 | 有声书 |
| 434 | 电子书 |
| 436 | 漫画书 |
| 440 | 短剧 |

#### 类型 — 音乐模式（`type` mode=5）

| 值 | 显示名称 |
|----|----------|
| 415 | 音乐 |
| 406 | MV |
| 437 | 演唱会/音乐会 |
| 410 | 图片写真 |
| 411 | 影视写真 |

#### 媒介（`medium_sel[4/5]`）

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | DVD |
| 3 | Remux |
| 5 | HDTV |
| 7 | Encode |
| 8 | CD |
| 10 | WEB-DL |
| 11 | UHD Blu-ray |
| 15 | SACD |
| 16 | 其他（Other） |
| 17 | Vinyl |
| 18 | HDCD |
| 19 | HI-RES |
| 20 | Web |

#### 视频编码 — 影视模式（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 2 | AVC/H.264/x264 |
| 7 | AV1 |
| 10 | H.266/VVC |
| 11 | HEVC/H.265/x265 |
| 12 | VP9 |
| 14 | Other |
| 15 | TXT |
| 16 | PDF |
| 17 | EPUB |
| 19 | AZW3/MOBI |
| 20 | ZIP |
| 21 | EPUB/AZW3/MOBI |

注意：mode=4 的编码字段包含非视频编码（PDF、EPUB 等），用于电子书分类。

#### 音频编码（`audiocodec_sel[4/5]`）

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC 分轨 |
| 3 | DTS |
| 4 | MP3 |
| 5 | APE |
| 6 | AAC |
| 7 | DTS-HD |
| 14 | MPEG |
| 15 | DD/DD+ |
| 16 | LPCM |
| 19 | TrueHD |
| 20 | WAV |
| 21 | Other |
| 22 | DTS:X |
| 23 | 镜像(Mirror) 整轨 |
| 24 | WAV 整轨 |
| 25 | DSF 分轨 |

注意：音频编码区分整轨/分轨（FLAC 分轨 vs 镜像整轨 vs WAV 整轨 vs DSF 分轨）。

#### 分辨率（`standard_sel[4/5]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 8K |
| 2 | 4K/2160p |
| 3 | 1080p/1080i |
| 4 | 720p |
| 10 | Other |

注意：1080p 和 1080i 合并为一个选项(3)。

#### 制作组（`team_sel[4/5]`）— 29个

| 值 | 显示名称 | 涵盖后缀 |
|----|----------|----------|
| 1 | HD4FANS | beAst |
| 2 | OurBits | OurBits, PbK, OurTV, iLoveTV, Ao, MGs, OurPad, HosT, iLoveHD |
| 3 | OKPT | OKWeb, OKTV |
| 4 | HHClub | HHWEB |
| 5 | Panda | PANDA, AilMR, AilME, AilMEPad, AilMWeb, AilMTV, AilMUpscale |
| 6 | HDHome | HDH, HDHome, HDHWEB |
| 8 | HDChina | HDChina, HDCTV |
| 9 | HDDollby | QHstudIo, Dream |
| 10 | HDSky | HDSWEB, HDSky, HDS |
| 11 | LemonHD | LeagueWEB, LeagueNF, LHD |
| 12 | M-Team | BMDru, MTeam |
| 13 | Audiences | ADWeb, ADE, Audies, ADeBook |
| 14 | Other | — |
| 16 | CMCT | CMCT |
| 17 | CHDBits | CHDBits, CHD, CHDWEB, CHDTV, CHDPAD, CHDHKTV |
| 18 | BTSchool | BTSCHOOL, BtsHD, BtsTV, BtsPAD |
| 19 | DaJiao | DJWEB |
| 20 | FRDS | FRDS |
| 21 | PterClub | Pter, PterWEB, PterMV, PterDIY, PterTV, PterGame |
| 22 | PTHome | PTH, PTHome, PTHtv |
| 23 | Red Leaves | RLeaves, R², RLWeb, RLTV, RL, RL4B |
| 24 | Rousi | RousiWeb |
| 26 | TTG | TTG, WiKi, ARiN, NGB, DOA |
| 27 | UBits | UBits |
| 28 | Ying | YingWEB |
| 29 | ZmPT | Zm, ZmWeb, ZmPT |
| 31 | BeiTai | — |
| 32 | U2 | — |

#### 地区（`processing_sel[4/5]`）

| 值 | 显示名称 |
|----|----------|
| 3 | 其他（Other） |
| 4 | 韩国（KR） |
| 5 | 日本（JP） |
| 6 | 欧美（EU/US） |
| 7 | 港澳台（HK/MAC/TW） |
| 8 | 中国大陆（CN） |
| 17 | 印度（India） |
| 18 | 东南亚（SEA） |

#### 标签 — 影视模式（`tags[4][]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | DoVi |
| 11 | 自购 |
| 12 | 特效 |
| 25 | 韩语 |
| 28 | 日语 |
| 31 | 资料教程 |
| 45 | 粤语 |
| 50 | 分集 |
| 51 | 完結 |
| 53 | Atmos |
| 56 | 驻站 |
| 57 | 英语 |
| 58 | 英字 |

### 2.3 缺失字段

- `pt_gen` — 无 PT-Gen 专用字段（需手动通过 PTGen 网站获取后粘贴到简介）
- `nfo` — 无 NFO 上传字段

---

## 二、与其他站点对比

| 维度 | OKPT | HDFans | HDVideo | NovaHD |
|------|------|--------|---------|--------|
| 分类模式 | 双模式(mode=4/5) | 单模式 | 单模式 | 单模式 |
| 分类数 | 14+5=19 | 16 | 8 | 13 |
| 制作组 | 29（含详细后缀映射） | 30 | 3 | 17 |
| 地区 | 有（8种） | 有（13种） | 无 | 无 |
| 音频编码 | 含整轨/分轨区分 | 24种 | 21种 | 15种 |
| 标签 | 18 | 27 | 25 | 18 |
| 分辨率 | 1080p/i合并 | 分开 | 无SD | 含帧率 |
| 编码字段 | 含PDF/EPUB等非视频 | 10种 | 8种 | 6种 |

### 关键差异

1. **双分类模式** — 影视(4)和音乐(5)使用不同的 type 列表和标签，适配器需根据类型切换 mode
2. **1080p/i 合并** — 1080p 和 1080i 合为选项(3)，其他站点通常分开
3. **编码字段含非视频** — mode=4 的编码字段包含 PDF、EPUB 等，用于电子书分类
4. **音频区分整轨/分轨** — FLAC 分轨(1)、镜像整轨(23)、WAV 整轨(24)、DSF 分轨(25)
5. **29个制作组带后缀映射** — 每个制作组对应多个后缀名（如 OurBits 含 PbK/OurTV/iLoveTV 等），需从种子标题中匹配后缀
6. **禁止制作组** — FGT、RARBG、Mp4Ba、xiaomi、huawei、BlackTV、BitsTV、DreamHD、Hao4K

---

## 三、站点适配器配置参考

```yaml
site:
  id: "okpt"
  name: "OKPT"
  url: "https://www.okpt.net"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  dual_mode:
    video: 4
    music: 5
    music_types: [415, 406, 437, 410, 411]

  mappings:
    type_video:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动漫": 405
      "体育": 407
      "其他": 409
      "游戏": 413
      "软件": 431
      "有声书": 432
      "电子书": 434
      "漫画书": 436
      "短剧": 440

    type_music:
      "音乐": 415
      "MV": 406
      "演唱会": 437
      "图片写真": 410
      "影视写真": 411

    medium_sel:
      "Blu-ray": 1
      "DVD": 2
      "Remux": 3
      "HDTV": 5
      "Encode": 7
      "CD": 8
      "WEB-DL": 10
      "UHD": 11
      "SACD": 15
      "Other": 16
      "Vinyl": 17
      "HDCD": 18
      "HI-RES": 19
      "Web": 20

    codec_sel:
      "H264": 2
      "AV1": 7
      "H266": 10
      "H265": 11
      "VP9": 12
      "Other": 14

    audiocodec_sel:
      "FLAC": 1
      "DTS": 3
      "MP3": 4
      "APE": 5
      "AAC": 6
      "DTS-HD": 7
      "MPEG": 14
      "DD": 15
      "LPCM": 16
      "TrueHD": 19
      "WAV": 20
      "Other": 21
      "DTS-X": 22
      "Mirror": 23
      "WAV整轨": 24
      "DSF": 25

    standard_sel:
      "8K": 1
      "4K": 2
      "1080p": 3
      "1080i": 3
      "720p": 4
      "Other": 10

    processing_sel:
      "大陆": 8
      "港澳台": 7
      "欧美": 6
      "日本": 5
      "韩国": 4
      "印度": 17
      "东南亚": 18
      "其他": 3

    team_sel:
      "HD4FANS": 1
      "OurBits": 2
      "OKPT": 3
      "HHClub": 4
      "Panda": 5
      "HDHome": 6
      "HDChina": 8
      "HDDollby": 9
      "HDSky": 10
      "LemonHD": 11
      "MTeam": 12
      "Audiences": 13
      "Other": 14
      "CMCT": 16
      "CHDBits": 17
      "BTSchool": 18
      "DaJiao": 19
      "FRDS": 20
      "PTerClub": 21
      "PTHome": 22
      "RedLeaves": 23
      "Rousi": 24
      "TTG": 26
      "UBits": 27
      "Ying": 28
      "ZmPT": 29
      "BeiTai": 31
      "U2": 32

    tags_video:
      "禁转": 1
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "DoVi": 8
      "自购": 11
      "特效": 12
      "韩语": 25
      "日语": 28
      "资料教程": 31
      "粤语": 45
      "分集": 50
      "完結": 51
      "Atmos": 53
      "驻站": 56
      "英语": 57
      "英字": 58

  field_names:
    video_suffix: "[4]"
    music_suffix: "[5]"
    medium: "medium_sel[{mode}]"
    codec: "codec_sel[{mode}]"
    audiocodec: "audiocodec_sel[{mode}]"
    standard: "standard_sel[{mode}]"
    team: "team_sel[{mode}]"
    processing: "processing_sel[{mode}]"
    tags_video: "tags[4][]"
    tags_music: "tags[5][]"
    anonymous: "uplver"

  missing_fields:
    - "pt_gen"
    - "nfo"

  blacklist_teams:
    - "FGT"
    - "RARBG"
    - "Mp4Ba"
    - "xiaomi"
    - "huawei"
    - "BlackTV"
    - "BitsTV"
    - "DreamHD"
    - "Hao4K"

  team_suffix_map:
    "HD4FANS": ["beAst"]
    "OurBits": ["OurBits", "PbK", "OurTV", "iLoveTV", "Ao", "MGs", "OurPad", "HosT", "iLoveHD"]
    "OKPT": ["OKWeb", "OKTV"]
    "HHClub": ["HHWEB"]
    "Panda": ["PANDA", "AilMR", "AilME", "AilMEPad", "AilMWeb", "AilMTV", "AilMUpscale"]
    "HDHome": ["HDH", "HDHome", "HDHWEB"]
    "HDChina": ["HDChina", "HDCTV"]
    "HDDollby": ["QHstudIo", "Dream"]
    "HDSky": ["HDSWEB", "HDSky", "HDS"]
    "LemonHD": ["LeagueWEB", "LeagueNF", "LHD"]
    "MTeam": ["BMDru", "MTeam"]
    "Audiences": ["ADWeb", "ADE", "Audies", "ADeBook"]
    "CMCT": ["CMCT"]
    "CHDBits": ["CHDBits", "CHD", "CHDWEB", "CHDTV", "CHDPAD", "CHDHKTV"]
    "BTSchool": ["BTSCHOOL", "BtsHD", "BtsTV", "BtsPAD"]
    "DaJiao": ["DJWEB"]
    "FRDS": ["FRDS"]
    "PTerClub": ["Pter", "PterWEB", "PterMV", "PterDIY", "PterTV", "PterGame"]
    "PTHome": ["PTH", "PTHome", "PTHtv"]
    "RedLeaves": ["RLeaves", "R²", "RLWeb", "RLTV", "RL", "RL4B"]
    "Rousi": ["RousiWeb"]
    "TTG": ["TTG", "WiKi", "ARiN", "NGB", "DOA"]
    "UBits": ["UBits"]
    "Ying": ["YingWEB"]
    "ZmPT": ["Zm", "ZmWeb", "ZmPT"]
```

---

## 四、发布流水线注意事项

### 4.1 双模式切换

OKPT 的分类分为两套：
- 当类型为 音乐/MV/演唱会/写真 时，使用 mode=5（`tags[5][]`、`medium_sel[5]` 等）
- 其他类型使用 mode=4（`tags[4][]`、`medium_sel[4]` 等）

适配器需要根据目标分类自动选择正确的 mode。

### 4.2 制作组后缀匹配

OKPT 的 29 个制作组各对应多个后缀名。需要从种子标题中提取制作组后缀（`-` 后的最后一段），然后在后缀映射表中查找对应的制作组ID。

### 4.3 1080p/i 合并

OKPT 将 1080p 和 1080i 合并为选项(3)，适配器无需区分。

### 4.4 音频整轨/分轨

音乐类资源需区分整轨和分轨形式：
- FLAC 分轨(1)、镜像整轨(23)、WAV 整轨(24)、DSF 分轨(25)

### 4.5 转载出处

转载出处按制作组源站决定，非转载来源站。适配器需维护制作组→源站的映射关系。

### 4.6 黑名单检查

发布前需检查制作组是否在黑名单中（FGT、RARBG 等），禁止转载这些组的资源。

---

*分析时间：2026-04-16*
*数据来源：OKPT 论坛5个规范帖 + upload.php 发布页面 HTML 分析*
