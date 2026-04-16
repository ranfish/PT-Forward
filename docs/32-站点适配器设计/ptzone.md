# PTZone 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | PTZone |
| 站点地址 | https://ptzone.xyz |
| 站点框架 | NexusPHP |
| 特殊规则 | 标准NexusPHP站点，Cloudflare防护 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 402 | TV Series(电视剧) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录片) |
| 405 | Animations(动漫) |
| 406 | Music(音乐) |
| 407 | Sports(体育) |
| 408 | Others(其它) |
| 409 | Others(其它) |
| 410 | Software(软件) |
| 411 | Games(游戏) |

注意：408 和 409 都显示"其它"，疑似重复分类。

#### 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | UHD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | MPEG-2 |
| 4 | MPEG-4 |
| 5 | Other |
| 6 | H.265 |

注意：编码列表较简单，无 AV1、VP9，不区分原盘/压制。

#### 音频编码（`audiocodec_sel[4]`）— 16个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | AC3 |
| 9 | DTS |
| 10 | DTS-HD MA |
| 11 | DD/AC3 |
| 12 | DDP/EAC3 |
| 13 | DTS-HD |
| 14 | TrueHD |
| 15 | WAV |

注意：值3和值9都显示"DTS"，值8(AC3)和值11(DD/AC3)功能重叠，疑似站点配置冗余。

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 8K |
| 6 | 4K |

注意：8K(5)排在4K(6)前面，值顺序不常规。

#### 制作组（`team_sel[4]`）— 仅6个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | PTZWeb |

#### 标签（`tags[4][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 分集 |
| 9 | 完结 |

### 1.3 缺失字段

- `processing_sel` — 无地区选择

---

## 二、站点适配器配置参考

```yaml
site:
  id: "ptzone"
  name: "PTZone"
  url: "https://ptzone.xyz"
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
      "音乐": 406
      "体育": 407
      "其他": 408
      "软件": 410
      "游戏": 411

    medium_sel:
      "Blu-ray": 1
      "HD DVD": 2
      "Remux": 3
      "WEB-DL": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Track": 9
      "UHD": 10

    codec_sel:
      "H264": 1
      "VC-1": 2
      "MPEG-2": 3
      "MPEG-4": 4
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
      "AC3": 8
      "DTS-HDMA": 10
      "DD": 11
      "DDP": 12
      "DTS-HD": 13
      "TrueHD": 14
      "WAV": 15

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "8K": 5
      "4K": 6

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "PTZWeb": 6

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "分集": 8
      "完结": 9

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    audiocodec: "audiocodec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  missing_fields:
    - "processing_sel"

  quirks:
    duplicate_dts: "值3和值9都显示DTS，建议使用值3"
    duplicate_others: "type值408和409都显示其它，建议使用408"
    resolution_order: "8K=5在4K=6前面"
```

---

## 三、发布流水线注意事项

### 3.1 音频编码重复

PTZone 的 `audiocodec_sel` 中 DTS 出现两次（值3和值9），AC3 也出现两次（值8 "AC3" 和值11 "DD/AC3"）。建议统一使用：
- DTS → 值3
- AC3/DD → 值11（DD/AC3）
- DTS-HD MA → 值10

### 3.2 制作组映射

PTZone 仅有6个制作组，与 PTFans 类似。转种时非 HDS/CHD/MySiLU/WiKi/PTZWeb 的制作组统一选 Other(5)。

### 3.3 Cloudflare 防护

PTZone 使用 Cloudflare（`cf_clearance` cookie），适配器的 HTTP 客户端需能处理 Cloudflare 质询。

---

*分析时间：2026-04-16*
*数据来源：https://ptzone.xyz/upload.php 发布页面 HTML 分析*
