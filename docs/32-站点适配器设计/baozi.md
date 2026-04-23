# 包子 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 包子|
| 站点地址 | https://p.t-baozi.cc |
| 站点框架 | NexusPHP |
| 主题 | Baozi（自定义橙色主题） |
| 关联站点 | 蓝影论坛 (hdblue.cc) |
| 特殊规则 | 标准HD站dupe规则，Cloudflare防护 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀，单模式（mode=4）。

#### 类型（`type`）— 必填，data-mode='4'

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | 音乐视频 |
| 407 | 体育运动 |
| 408 | 高品质音频 |
| 409 | 其他 |
| 410 | 短剧 |

注意：有"短剧"(410)和"高品质音频"(408)独特分类。无游戏/软件分类。

#### 媒介（`medium_sel[4]`）— 13个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray 原盘 |
| 2 | HD DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | WEB-DL |
| 11 | UHD Blu-ray 原盘 |
| 12 | UHD Blu-ray DIY |
| 13 | Blu-ray DIY |

注意：媒介细分程度高——Blu-ray 分为原盘(1)和DIY(13)，UHD Blu-ray 也分为原盘(11)和DIY(12)。共13个媒介选项。

#### 视频编码（`codec_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264(AVC) |
| 2 | VC-1 |
| 3 | AV1 |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265(HEVC) |

注意：包含 AV1(3)。H.264/H.265 不区分原盘/压制，合并写法。

#### 音频编码（`audiocodec_sel[4]`）— 15个

| 值 | 显示名称 |
|----|----------|
| 1 | DD/AC3 |
| 2 | OPUS |
| 3 | DTS |
| 4 | MP3 |
| 5 | M4A |
| 6 | AAC |
| 7 | Other |
| 8 | DTS:X |
| 9 | TrueHD Atmos |
| 10 | DTS-HD MA |
| 11 | TrueHD |
| 12 | LPCM |
| 13 | FLAC |
| 14 | APE |
| 15 | WAV |

注意：AC3 写为 DD/AC3(1)。包含 OPUS(2)、M4A(5)。DTS:X(8)、TrueHD Atmos(9)、DTS-HD MA(10) 分开列出。无 DTS-HD（无 MA 后缀）选项。

#### 分辨率（`standard_sel[4]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K |
| 6 | 8K |
| 7 | None |

注意：含 None(7) 选项（可能用于音频等非视频资源）。8K(6) 在 4K(5) 后面。

#### 制作组（`team_sel[4]`）— 仅7个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | BAOZIWEB |
| 7 | Baozi |

注意：有 BAOZIWEB(6) 和 Baozi(7) 两个官方组。

#### 标签（`tags[4][]`）— 22个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 粤语 |
| 9 | 原创 |
| 11 | 动画 |
| 12 | 完结 |
| 13 | Dolby Vision |
| 14 | HDR10 |
| 15 | HDR10+ |
| 16 | 限转 |
| 17 | 应求 |
| 18 | MV |
| 19 | 卡拉OK |
| 20 | LIVE现场 |
| 21 | 演唱会 |
| 22 | 音乐专辑 |
| 23 | 成人 |

注意：标签数量多且分类丰富：
- HDR 细分3种：HDR(7)、Dolby Vision(13)、HDR10(14)、HDR10+(15)
- 音乐相关4种：MV(18)、卡拉OK(19)、LIVE现场(20)、演唱会(21)、音乐专辑(22)
- 转/原创：禁转(1)、限转(16)、首发(2)、原创(9)、应求(17)

### 1.3 缺失字段

- `processing_sel` — 无地区选择

---

## 二、标题命名规范

来源：`rules.php` → 种子信息

### 2.1 标题格式

| 类型 | 格式 | 示例 |
|------|------|------|
| 电影 | `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS |
| 电视剧 | `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU |
| 音轨 | `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组]` | 恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC |
| 游戏 | `[中文名] 名称 [年份] [版本] [发布说明][-发布组]` | 红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED |

### 2.2 简介要求

- 电影/电视剧/动漫：必须包含海报/封面，尽可能包含截图、MediaInfo、演职员和剧情概要
- NFO 写入 NFO 文件而非粘贴到简介
- 体育节目：禁止泄漏比赛结果
- 音乐：必须包含专辑封面和曲目列表
- 原始发布信息基本符合规范时，尽量使用原始发布信息

---

## 三、发布规则

### 3.1 允许的资源

- 高清视频（Blu-ray/HD DVD 原盘、Remux、HDTV、720p+ 重编码）
- 标清视频（仅限高清媒介重编码 480p+、DVDR/DVDISO/DVDRip/CNDVDRip）
- 无损音轨（FLAC、Monkey's Audio 等）
- 5.1声道+ 音轨（DTS、DTSCD 等）
- PC游戏（必须原版光盘镜像）
- 7日内高清预告片
- 高清相关软件和文档

### 3.2 禁止的资源

- 总体积 < 100MB
- 标清 upscale 视频
- CAM、TC、TS、SCR、DVDSCR、R5、HalfCD 等低质量
- RealVideo/RMVB/RM/FLV
- 单独样片
- < 5.1声道有损音频（MP3、WMA）
- 无正确 cue 的多轨音频
- 游戏硬盘版/高压版/非官方镜像
- RAR 压缩文件
- 重复资源
- 涉及禁忌或敏感内容
- 损坏文件、垃圾文件

### 3.3 Dupe 规则（标准 HD 站规则）

媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 同一视频高优先级使低优先级被判定为重复
- 动漫特例：HDTV 和 DVD 同优先级
- 按发布组优先级判定（参考论坛帖子）
- 断种45日+ 或发布18月+ → 可重发
- 不同区域/配音/字幕的原盘不视为重复
- 无损音轨只保留一个版本（分轨 FLAC 优先级最高）

### 3.4 资源打包规则

标准 HD 站打包规则（电影合集、整季剧集、纪录片合集、MV、音乐打包等）。

### 3.5 促销规则

随机促销（上传后自动触发）：
- 10% → 50%下载
- 5% → 免费
- 5% → 2x上传
- 3% → 50%下载 & 2x上传
- 1% → 免费 & 2x上传

固定促销：
- 总体积 > 20GB → 自动免费
- Blu-ray/HD DVD 原盘 → 免费
- 电视剧每季第一集 → 免费
- 所有种子发布1个月后 → 永久2x上传
- 促销限时7天（2x上传无时限）

### 3.6 账号保留规则

| 条件 | 规则 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存账号后不会被删除 |
| 封存账号 | 连续 400 天不登录删除 |
| 未封存账号 | 连续 150 天不登录删除 |
| 无流量账号 | 连续 100 天不登录删除 |

---

## 四、站点适配器配置参考

```yaml
site:
  id: "baozi"
  name: "包子"
  alt_name: "BaoZi"
  url: "https://p.t-baozi.cc"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动漫": 405
      "MV": 406
      "体育": 407
      "高品质音频": 408
      "其他": 409
      "短剧": 410

    medium_sel:
      "Blu-ray": 1
      "HD DVD": 2
      "Remux": 3
      "MiniBD": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Track": 9
      "WEB-DL": 10
      "UHD": 11
      "UHD DIY": 12
      "DIY": 13

    codec_sel:
      "H264": 1
      "VC-1": 2
      "AV1": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 6

    audiocodec_sel:
      "DD": 1
      "OPUS": 2
      "DTS": 3
      "MP3": 4
      "M4A": 5
      "AAC": 6
      "Other": 7
      "DTS:X": 8
      "Atmos": 9
      "DTS-HDMA": 10
      "TrueHD": 11
      "LPCM": 12
      "FLAC": 13
      "APE": 14
      "WAV": 15

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "2160p": 5
      "8K": 6
      "None": 7

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "BAOZIWEB": 6
      "Baozi": 7

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "粤语": 8
      "原创": 9
      "动画": 11
      "完结": 12
      "Dolby Vision": 13
      "HDR10": 14
      "HDR10+": 15
      "限转": 16
      "应求": 17
      "MV": 18
      "卡拉OK": 19
      "LIVE现场": 20
      "演唱会": 21
      "音乐专辑": 22
      "成人": 23

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    audiocodec: "audiocodec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    technical_info: "technical_info"
    pt_gen: "pt_gen"
    anonymous: "uplver"

  missing_fields:
    - "processing_sel"

  quirks:
    medium_uhd_split: "Blu-ray和UHD Blu-ray各自分原盘/DIY，共13个媒介"
    hdr_tag_variants: "标签中HDR细分4种：HDR/Dolby Vision/HDR10/HDR10+"
    music_tags: "标签含MV/卡拉OK/LIVE/演唱会/音乐专辑5种音乐标签"
    standard_none: "分辨率含None(7)选项，用于非视频资源"
    cloudflare: "使用Cloudflare防护"
    short_drama_category: "有短剧(410)独特分类"
```

---

## 五、发布流水线注意事项

### 5.1 制作组映射

仅7个选项，非 HDS/CHD/MySiLU/WiKi/BAOZIWEB/Baozi 的制作组统一选 Other(5)。BAOZIWEB(6) 用于 WEB 来源，Baozi(7) 为官方压制组。

### 5.2 媒介细分

Blu-ray 细分为4种：
- Blu-ray 原盘(1) vs Blu-ray DIY(13)
- UHD Blu-ray 原盘(11) vs UHD Blu-ray DIY(12)

转种时需区分是否 DIY 以及是否 UHD。

### 5.3 音频编码映射

注意包子与其它站点的音频编码值差异较大：
- AC3 写为 DD/AC3(1)
- DTS:X(8)、TrueHD Atmos(9)、DTS-HD MA(10) 分别独立
- 包含 OPUS(2)、M4A(5)
- 无独立 DTS-HD（无 MA）选项

---

*分析时间：2026-04-16*
*最后更新：2026-04-22*
*数据来源：https://p.t-baozi.cc/rules.php + https://p.t-baozi.cc/upload.php*
