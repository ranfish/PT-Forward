# 不可杜 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 不可杜|
| 站点地址 | https://www.hddolby.com |
| 论坛地址 | https://forums.orcinusorca.org |
| Wiki 地址 | https://wiki.orcinusorca.org |
| 站点框架 | NexusPHP |
| 主题 | Classic |
| 官方组 | Dream、QHstudIo、CornerMV、DBTV、Telesto |
| 特殊规则 | 需 2FA 验证才能访问发布/规则页面；Cloudflare 防护 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

**字段名无后缀**（裸名，如 `medium_sel` 而非 `medium_sel[4]`）。

**前置要求**：访问发布页面前需通过 `take2fa.php` 双因素认证。

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `tmdb_url` | text | ✓ | **TMDb 链接**（必填，来自 themoviedb.org） |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布 |

注意：HDDolby 使用 **TMDb 链接**（非 IMDb），且为**必填项**。无 `url`（IMDb链接）、`pt_gen`（PTGen）、`nfo`（NFO文件）、`technical_info`（MediaInfo）。

### 1.2 质量选择字段

无 data-mode，字段名无后缀。

#### 类型（`type`）— 11个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies电影 |
| 402 | TV Series电视剧 |
| 403 | TV Shows综艺 |
| 404 | Documentaries纪录片 |
| 405 | Animations动漫 |
| 406 | Music Videos |
| 407 | Sports体育 |
| 408 | HQ Audio音乐 |
| 409 | Others其他 |
| 410 | Games游戏 |
| 411 | Study学习 |

注意：类型名称使用中英双语（如 "Movies电影"）。包含 Study(411) 独特分类。

#### 媒介（`medium_sel`）— 12个

| 值 | 显示名称 |
|----|----------|
| 1 | UHD |
| 2 | Blu-ray |
| 3 | Remux |
| 4 | HD DVD |
| 5 | HDTV |
| 6 | WEB-DL |
| 7 | Webrip |
| 8 | DVD |
| 9 | CD |
| 10 | Encode |
| 11 | Other |
| 12 | FEED |

注意：有 FEED(12) 独特选项（可能指 RSS Feed 来源）。有 Webrip(7) 独立选项。UHD(1) 排在最前。

#### 视频编码（`codec_sel`）— 14个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 2 | H.265/HEVC |
| 5 | VC-1 |
| 6 | MPEG-2 |
| 7 | Other |
| 11 | AV1 |
| 12 | VP9 |
| 13 | H.266/VVC |
| 14 | AVS3 |
| 15 | AVS+ |
| 16 | AVS2 |

注意：编码选项是已分析站点中最多的（14个），包含：
- 前瞻性编码：H.266/VVC(13)
- 中国自主标准：AVS3(14)、AVS+(15)、AVS2(16)
- 网页编码：AV1(11)、VP9(12)

#### 音频编码（`audiocodec_sel`）— 18个

| 值 | 显示名称 |
|----|----------|
| 1 | DTS-HD MA |
| 2 | TrueHD |
| 3 | LPCM |
| 4 | DTS |
| 5 | DD/AC3 |
| 6 | AAC |
| 7 | FLAC |
| 8 | APE |
| 9 | WAV |
| 10 | MP3 |
| 11 | M4A |
| 12 | Other |
| 13 | Opus |
| 14 | DDP/EAC3 |
| 15 | DTS-X |
| 16 | AV3A |
| 17 | AVSA |
| 18 | MPEG |

注意：音频编码也是已分析站点中最多的（18个），包含：
- 中国自主音频标准：AV3A(16)、AVSA(17)
- DDP/EAC3(14) 独立选项
- DTS-X(15) 独立选项
- MPEG 音频(18)

#### 分辨率（`standard_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 2160p/4K |
| 2 | 1080p |
| 3 | 1080i |
| 4 | 720p |
| 5 | Others |
| 6 | 4320/8K |

注意：分辨率名称使用 "2160p/4K" 双写法。4K(1) 排在最前。含 4320/8K(6)。

#### 制作组（`team_sel`）— 14个

| 值 | 显示名称 |
|----|----------|
| 1 | Dream |
| 2 | MTeam |
| 3 | PTHome |
| 4 | WiKi |
| 5 | CHD |
| 6 | CMCT |
| 7 | FRDS |
| 8 | Other |
| 9 | HDo |
| 10 | DBTV |
| 11 | beAst |
| 12 | QHstudIo |
| 13 | CornerMV |
| 14 | Telesto |

注意：制作组数量多（14个），包含：
- 官方组：Dream(1)、QHstudIo(12)、CornerMV(13)、DBTV(10)、Telesto(14)
- 外站组：MTeam(2)、PTHome(3)、WiKi(4)、CHD(5)、CMCT(6)、FRDS(7)、HDo(9)、beAst(11)

#### 标签（`tags[]`）— 17个，使用字符串值

| 值 | 显示名称 |
|----|----------|
| gy | 国语 |
| yy | 粤语 |
| ja | 日语 |
| ko | 韩语 |
| tx | 特效 |
| zz | 中字 |
| lz | 连载 |
| wj | 完结 |
| diy | DIY |
| db | DOLBY VISION |
| hdr10 | HDR10 |
| hdrm | HDR10+ |
| hlg | HLG |
| hdrv | HDR Vivid |
| hq | 高码率 |
| hfr | 高帧率 |
| yq | 应求 |

注意：标签使用**字符串缩写值**（非数字），是已分析站点中仅有的两个之一（另一个是 PTHome 音乐页）。HDR 细分5种（DV/HDR10/HDR10+/HLG/HDR Vivid）。语言标签含日语/韩语。

### 1.3 缺失字段

- `url` — 无 IMDb 链接输入框
- `pt_gen` — 无 PTGen 链接
- `nfo` — 无 NFO 文件上传
- `technical_info` — 无 MediaInfo 输入框
- `processing_sel` — 无地区选择

---

## 二、标题命名规范

来源：Wiki (https://wiki.orcinusorca.org/zh/rules/upload)

### 2.1 打包规则

允许的打包：
- 完结单季剧集/动画/纪录片（制作组官方打包）
- 未完结但同一季的连续集数
- 不同分辨率质量的剧集/动画/纪录片
- 不同季的剧集
- 不同来源或处理的剧集/动画/纪录片（BTN Internal Groups 除外）
- 同一系列的电影

禁止的打包：
- 无意义打包（如 Douban250 合集）

### 2.2 简介要求

- 无 NFO 文件上传功能
- 无 MediaInfo 输入框
- 所有描述信息写入简介

---

## 三、发布规则

来源：Wiki (https://wiki.orcinusorca.org/zh/rules/upload)

### 3.1 发布资格

- 任何人都能发布资源
- 候选积分制度：积分达到 20+ 时种子直接进入种子区，少于 20 进入候选区审核
- 通过审核 +2 分，被踢回 -5 分，低于 -100 禁止发布

### 3.2 ⚠️ 黑名单制作组

**以下制作组资源禁止发布**：
- **FGT**、**RARTV**、**RARBG**、**MP4BA** — 公网盗窃组
- **DreamHD**、**DDHDTV** — 其他公网组
- **HDVideo**、**HDVbits** — 网站小组

> 其他管理员判断为来自公网/BT的劣质资源也禁止发布。

### 3.3 禁止的资源

- 后缀为黑名单组的资源
- 他站尚未出种的资源
- 总体积 < 100MB（官方小组除外）
- 已完结剧集的分集资源
- Pad 专用资源
- 标清 upscale 视频
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD
- RealVideo/RMVB/RM/FLV（学习类视频除外）
- 单独样片
- 游戏硬盘版/高压版
- RAR 压缩文件
- 重复资源
- 损坏文件、垃圾文件
- 集数有缺失的完结剧集合集

### 3.4 Dupe 规则

- 来源为 WEB-DL 和 HDTV 的资源，如与官组资源规格一致或官组资源质量更高 → 判定为重复
- 正式发行的影视 Untouched 原盘套装不视为 dupe
- 标准媒介/发布组优先级规则

### 3.5 其他规则

- 单种上传限速 100 MB/s，超速警告，多次违反封号
- 禁止截图本站任何内容并传播
- H&R 规则（详见 Wiki）
- 做种时间不足24小时会警告

---

## 四、站点适配器配置参考

```yaml
site:
  id: "hddolby"
  name: "HDDolby"
  url: "https://www.hddolby.com"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"
  wiki_url: "https://wiki.orcinusorca.org/zh/rules/upload"
  forum_url: "https://forums.orcinusorca.org"

  preconditions:
    2fa: "需要通过take2fa.php双因素认证后才能访问发布页"

  blacklist_groups:
    - "FGT"
    - "RARTV"
    - "RARBG"
    - "MP4BA"
    - "DreamHD"
    - "DDHDTV"
    - "HDVideo"
    - "HDVbits"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动漫": 405
      "MV": 406
      "体育": 407
      "音乐": 408
      "其他": 409
      "游戏": 410
      "学习": 411

    medium_sel:
      "UHD": 1
      "Blu-ray": 2
      "Remux": 3
      "HD DVD": 4
      "HDTV": 5
      "WEB-DL": 6
      "Webrip": 7
      "DVD": 8
      "CD": 9
      "Encode": 10
      "Other": 11
      "FEED": 12

    codec_sel:
      "H264": 1
      "H265": 2
      "VC-1": 5
      "MPEG-2": 6
      "Other": 7
      "AV1": 11
      "VP9": 12
      "VVC": 13
      "AVS3": 14
      "AVS+": 15
      "AVS2": 16

    audiocodec_sel:
      "DTS-HDMA": 1
      "TrueHD": 2
      "LPCM": 3
      "DTS": 4
      "DD": 5
      "AAC": 6
      "FLAC": 7
      "APE": 8
      "WAV": 9
      "MP3": 10
      "M4A": 11
      "Other": 12
      "Opus": 13
      "DDP": 14
      "DTS:X": 15
      "AV3A": 16
      "AVSA": 17
      "MPEG": 18

    standard_sel:
      "2160p": 1
      "1080p": 2
      "1080i": 3
      "720p": 4
      "Other": 5
      "8K": 6

    team_sel:
      "Dream": 1
      "MTeam": 2
      "PTHome": 3
      "WiKi": 4
      "CHD": 5
      "CMCT": 6
      "FRDS": 7
      "Other": 8
      "HDo": 9
      "DBTV": 10
      "beAst": 11
      "QHstudIo": 12
      "CornerMV": 13
      "Telesto": 14

    tags:
      "国语": "gy"
      "粤语": "yy"
      "日语": "ja"
      "韩语": "ko"
      "特效": "tx"
      "中字": "zz"
      "连载": "lz"
      "完结": "wj"
      "DIY": "diy"
      "Dolby Vision": "db"
      "HDR10": "hdr10"
      "HDR10+": "hdrm"
      "HLG": "hlg"
      "HDR Vivid": "hdrv"
      "高码率": "hq"
      "高帧率": "hfr"
      "应求": "yq"

  field_names:
    suffix: ""
    medium: "medium_sel"
    codec: "codec_sel"
    audiocodec: "audiocodec_sel"
    standard: "standard_sel"
    team: "team_sel"
    tags: "tags[]"
    anonymous: "uplver"

  missing_fields:
    - "url"
    - "pt_gen"
    - "nfo"
    - "technical_info"
    - "processing_sel"

  quirks:
    requires_2fa: "访问发布页需先通过2FA验证"
    tmdb_required: "TMDb链接(tmdb_url)为必填项，非IMDb"
    string_tag_values: "标签使用字符串缩写值（gy/yy/ja/ko等）"
    chinese_codecs: "编码含中国自主标准：AVS3/AVS+/AVS2/AV3A/AVSA"
    most_codecs: "视频编码14个+音频编码18个，已分析站点中最多"
    hdr_5_variants: "HDR标签细分5种：DV/HDR10/HDR10+/HLG/HDR Vivid"
    feed_medium: "媒介含FEED(12)独特选项"
    upload_speed_limit: "单种上传限速100MB/s"
    candidate_system: "候选积分制度，20+直接进种子区"
    no_screenshot: "禁止截图本站任何内容并传播"
    cloudflare: "使用Cloudflare防护"
```

---

## 五、发布流水线注意事项

### 5.1 2FA 前置要求

访问发布页面前需先完成双因素认证（`take2fa.php`）。自动化发布时需处理 2FA 流程。

### 5.2 黑名单检查

发布前必须检查制作组：
- FGT/RARTV/RARBG/MP4BA/DreamHD/DDHDTV（公网盗窃组）→ 拒绝
- HDVideo/HDVbits（网站小组）→ 拒绝
- 管理员判断为公网/BT劣质资源 → 拒绝

### 5.3 中国自主编码标准

HDDolby 是唯一支持中国自主编码标准的站点：
- 视频：AVS3(14)、AVS+(15)、AVS2(16)
- 音频：AV3A(16)、AVSA(17)
- 需在映射中处理这些特殊编码

### 5.4 标签字符串值

标签使用字符串缩写值（非数字），提交时需使用 `tags[]=gy&tags[]=zz` 格式。

### 5.5 上传限速

单种上传限速 100 MB/s，自动化发布需注意控制上传速度。

---

*分析时间：2026-04-16*
*数据来源：upload.php（用户提供HTML）+ Wiki (https://wiki.orcinusorca.org/zh/rules/upload)*
