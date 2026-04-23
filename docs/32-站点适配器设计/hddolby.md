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
| `name` | text | - | 标题（示例：`DARK S01 2017 1080p Netflix WebDL H264 - Dream`） |
| `small_descr` | text | - | 副标题（示例：`暗黑 / 暗黑世界 / 黑暗世界 / 黑暗 第一季 全10集 外挂中文字幕`） |
| `tmdb_url` | text | ✓ | **TMDb 链接**（必填，来自 themoviedb.org） |
| `descr` | textarea | ✓ | 简介（BBCode，含完整编辑器：B/I/U/URL/IMG/Video/List/Quote/颜色/字体/字号） |
| `media_info` | textarea | 条件必填 | MediaInfo（影视类必填，其他分类可选，JS 动态控制 `*` 显示） |
| `screenshots` | textarea | ✓ | 截图（每行一个 URL，HTML `required` 属性） |
| `uplver` | checkbox | - | 匿名发布（**默认勾选** `checked`，value="yes"） |

注意：HDDolby 使用 **TMDb 链接**（非 IMDb），且为**必填项**。无 `url`（IMDb链接）、`pt_gen`（PTGen）、`nfo`（NFO文件）字段。

**Tracker Announce URLs**（3 个）：
1. `https://t.hddolby.com/announce.php`
2. `t.ptdream.net/announce.php`
3. `t.orcinusorca.org/announce.php`

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
- `processing_sel` — 无地区选择

### 1.4 隐藏标签（CSS 中定义但上传表单未显示）

CSS 注释中列出完整标签集：`gf, gy, yy, ja, ko, zz, jz, xz, diy, sf, yq, m0, yc, gz, db, hdr10, hdrm, tx, lz, wj, hdrv, hlg, hq, hfr`

其中仅 17 个出现在上传表单。隐藏标签可能用于系统内部或种子列表显示：

| 缩写 | CSS 类 | 推测含义 |
|------|--------|----------|
| gf | .tgf | 官方（官组标记？） |
| jz | .tjz | 禁转 |
| xz | .txz | 限转 |
| sf | .tsf | 私种/收费？ |
| m0 | .tm0 | 免费标记？ |
| yc | .tyc | 应采？ |
| gz | .tgz | 国语？（与 gy 重复？需确认） |

---

## 二、标题命名规范

来源：Wiki (https://wiki.orcinusorca.org/zh/rules/upload)

### 2.1 打包规则

**可发布打包合集**：
- （制作组官方打包的）完结的单季剧集/动画/纪录片
- （制作组官方打包的）未完结但属于同一季的连续集数的剧集/动画/纪录片
- 部分小组的完结剧集多季合集（设为零魔且无优惠）：CMCT、FRDS、HDH、HHWEB、jsum@U2、VCB-Studio、WiKi
- 部分小组的电影合集（设为零魔且无优惠）：beAst、BeiTai、CMCT、FRDS、HDH、jsum@U2、VCB-Studio、WiKi
- 正式发行的影视 Untouched 原盘套装

**不可发布打包合集**：
- 同一系列的电影
- 不同分辨率质量的剧集/动画/纪录片
- 不同季的剧集
- 不同来源或处理的剧集/动画/纪录片（BTN Internal Groups 除外）
- 非单一制作组的剧集/动画/纪录片（BTN Internal Groups 除外）
- 无意义打包（如 Douban250 合集）

### 2.2 简介要求

- 影视类种子请参考 Wiki「影视类种子发种标准格式」
- 发布者应及时按照管理员的种子修改意见对种子进行修改，多次无视意见将被强制候选或取消上传权限
- 无 NFO 文件上传功能
- 无 MediaInfo 输入框
- 所有描述信息写入简介

---

## 三、发布规则

来源：Wiki (https://wiki.orcinusorca.org/zh/rules/upload)

### 3.1 上传总则

- 上传者必须保证上传速度与做种时间
- 撤种或做种时间不足 24 小时，或故意低速上传 → 警告甚至取消上传权限
- 发布者获得 1 倍上传量

### 3.2 发布资格

- 任何人都能发布资源
- 候选积分制度：积分达到 20+ 时种子直接进入种子区，少于 20 进入候选区审核
- 通过审核 +2 分，被踢回 -5 分，低于 -100 禁止发布

### 3.3 ⚠️ 黑名单制作组

**以下制作组资源禁止发布**：
- **FGT**、**RARTV**、**RARBG**、**MP4BA** — 公网盗窃组
- **DreamHD**、**DDHDTV** — 其他公网组
- **HDVideo**、**HDVbits** — 网站小组

> 其他管理员判断为来自公网/BT的劣质资源也禁止发布。

### 3.4 禁止的资源

- 涉及禁忌或敏感内容（如色情、敏感政治话题等）→ 警告/取消上传权限/禁用账号
- 种子内包含有害文件（病毒、木马等）→ 警告/取消上传权限/禁用账号
- 他站禁转或处于限转期的资源 → 警告/取消上传权限/禁用账号
- 后缀为黑名单组的资源
- 他站尚未出种的资源
- 总体积 < 100MB（官方小组除外）
- 已完结剧集的分集资源
- Pad 专用资源（制作组为 Pad 组或标题注明 Pad）
- 标清 Upscale 或部分 Upscale 的视频
- 标清级别质量较差的视频：CAM/TC/TS/SCR/DVDSCR/R5/R5.Line/HalfCD
- RealVideo/RMVB/RM/FLV（学习类视频除外）
- 单独样片（样片请和正片一起上传）
- 游戏硬盘版/高压版、非官方游戏镜像、第三方 mod、小游戏合集、单独游戏破解/补丁
- RAR 等压缩文件
- 种子内包含网站链接、广告文档、其他无关文件或嵌套种子文件
- 重复资源（dupe 判定见下文）
- 损坏文件
- 集数有缺失的完结剧集合集

### 3.5 Dupe 规则

- 来源为 WEB-DL 和 HDTV 的资源，如与官组资源规格一致或官组资源质量更高 → 判定为重复
- 正式发行的影视 Untouched 原盘套装不视为 dupe
- 标准媒介/发布组优先级规则

### 3.6 其他规则

- 单种上传限速 100 MB/s，超速警告，警告期内多次违反封号；部分 IP 将受阻断
- 禁止截图本站任何内容并传播
- 做种时间不足 24 小时会警告

---

## 三-B、盒子规则

来源：Wiki (https://wiki.orcinusorca.org/zh/rules/seedbox)

### 盒子标记判定

| 条件 | 阈值 |
|------|------|
| 海外 IP 地址 | 上传速度超过 20 MB/s |
| 大陆 IP 地址 | 上传速度超过 50 MB/s |

### 盒子限制

- 盒子黑名单：任何时候下载全黑种
- 种子发布 72 小时内：盒子上传按实际量统计，**不享受 2X、2XFREE**
- 超过 72 小时后恢复正常

### 豁免

- VIP 用户不受限制
- 发种者不受限制

### 取消盒子标记

- 因代理导致：取消代理即可
- 家宽因超速导致：原则上不予取消

---

## 三-C、H&R 规则

来源：Wiki (https://wiki.orcinusorca.org/zh/rules/hitandrun)

### H&R 标识

- 带有 H&R 的种子会有 H&R 标识
- 开始下载后自动记入考核中 HR 条目

### H&R 考核要求

下载完成起 **30 天内**，满足以下**任意一条**即达标：

| 达标条件 | 要求 |
|----------|------|
| 保持上传 | 共计 48 小时 |
| 单种共享率 | ≥ 1.5 |

- 达标后自动标记（最长一天延迟）
- **未达标**：转为未达标状态，只能使用魔力值转变为免考核
- **累计 10 个未达标 HR** → 禁用账号
- 种子发布超过 **90 天** → 不再参与 H&R 检查（取消前的记录继续检查）

### H&R 发布权限

- 发布员及以上用户可发布带有 H&R 的种子

### H&R 豁免

- 贵宾（VIP）及以上用户不受 H&R 限制

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
    mediainfo: "media_info"
    screenshots: "screenshots"

  missing_fields:
    - "url"
    - "pt_gen"
    - "nfo"
    - "processing_sel"

  quirks:
    requires_2fa: "访问发布页需先通过2FA验证"
    tmdb_required: "TMDb链接(tmdb_url)为必填项，非IMDb"
    string_tag_values: "标签使用字符串缩写值（gy/yy/ja/ko等）"
    chinese_codecs: "编码含中国自主标准：AVS3/AVS+/AVS2/AV3A/AVSA"
    hdr_5_variants: "HDR标签细分5种：DV/HDR10/HDR10+/HLG/HDR Vivid"
    feed_medium: "媒介含FEED(12)独特选项"
    upload_speed_limit: "单种上传限速100MB/s，部分IP受阻断"
    candidate_system: "候选积分制度，20+直接进种子区"
    no_screenshot: "禁止截图本站任何内容并传播"
    cloudflare: "使用Cloudflare防护"
    anonymous_default_on: "uplver默认勾选"
    mediainfo_conditional: "media_info字段影视类必填，JS动态控制"
    screenshots_required: "screenshots字段必填，每行一个URL"
    hidden_tags: "CSS中有7个隐藏标签（gf/jz/xz/sf/m0/yc/gz）不在上传表单"
    seedbox_overseas_20mb: "海外IP上传>20MB/s标记盒子，大陆>50MB/s"
    seedbox_overseas_20mb: "海外IP上传>20MB/s标记盒子，大陆>50MB/s"
    seedbox_72h_no_promo: "盒子72h内不享受2X/2XFREE"
    hr_30day: "H&R 30天考核期，48h上传或共享率≥1.5"
    hr_10_fail_ban: "累计10个未达标HR禁用账号"
    hr_90day_expire: "种子发布90天后不再参与H&R检查"
    forbidden_sensitive: "禁止禁忌/敏感内容（色情/政治等）"
    forbidden_other_site_only: "禁止他站尚未出种的资源"
    pack_multi_season_groups: "多季合集允许组：CMCT/FRDS/HDH/HHWEB/jsum@U2/VCB-Studio/WiKi"
    pack_movie_groups: "电影合集允许组：beAst/BeiTai/CMCT/FRDS/HDH/jsum@U2/VCB-Studio/WiKi"
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

### 5.3 MediaInfo 与截图

- `media_info` 字段：影视类种子必填（JS 动态验证），非影视类可选
- `screenshots` 字段：HTML `required` 属性，每行一个截图 URL
- **不是**之前文档标注的"无 MediaInfo 输入框"——该字段名为 `media_info`（非 `technical_info`）

### 5.4 中国自主编码标准

HDDolby 是唯一支持中国自主编码标准的站点：
- 视频：AVS3(14)、AVS+(15)、AVS2(16)
- 音频：AV3A(16)、AVSA(17)
- 需在映射中处理这些特殊编码

### 5.5 标签字符串值

标签使用字符串缩写值（非数字），提交时需使用 `tags[]=gy&tags[]=zz` 格式。

### 5.6 上传限速

单种上传限速 100 MB/s，自动化发布需注意控制上传速度。

### 5.7 匿名发布

`uplver` 默认勾选（`checked`），value="yes"。转发时无需特别处理，默认即匿名。

---

*分析时间：2026-04-16（Wiki 规则更新：2026-04-22）*
*数据来源：upload.php（用户提供HTML）+ Wiki (https://wiki.orcinusorca.org/zh/rules/upload) + Wiki (https://wiki.orcinusorca.org/zh/rules/seedbox) + Wiki (https://wiki.orcinusorca.org/zh/rules/hitandrun)*
