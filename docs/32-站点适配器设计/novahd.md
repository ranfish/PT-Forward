# NovaHD 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | NovaHD |
| 站点地址 | https://pt.novahd.top |
| 站点框架 | NexusPHP |
| 特殊规则 | 分辨率含帧率细分（60FPS/120FPS）、番剧分类、17个制作组 |

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
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/记录片 |
| 405 | Animations/动画 |
| 406 | MV/演唱会 |
| 407 | Sports/体育 |
| 409 | Music/音乐 |
| 410 | Othes/其他 |
| 411 | Short Play/短剧 |
| 412 | Anime/动漫 |
| 413 | Anime Series/番剧 |
| 414 | Game/游戏 |

#### 媒介（`medium_sel[4]`）

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
| 10 | UHD Blu-ray |
| 11 | WEB-DL |
| 12 | DVD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H264/x264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H265/HEVC/x265 |

#### 音频编码（`audiocodec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | ALAC |
| 8 | TrueHD Atmos |
| 9 | DDP/E-AC3 |
| 10 | DD/AC3 |
| 11 | LPCM |
| 12 | TrueHD |
| 13 | DTS-HD MA |
| 14 | DTS:X |
| 15 | Other |

#### 分辨率（`standard_sel[4]`）— 含帧率细分

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p/4K |
| 6 | 4320p/8K |
| 7 | 2160p/4K 60Fps |
| 8 | 2160p/4K 120Fps |

#### 制作组（`team_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | HDSky |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | FRDS |
| 7 | beAst |
| 8 | CMCT |
| 9 | TLF |
| 10 | M-Team |
| 11 | BeiTai |
| 12 | AGSV |
| 13 | HDHome |
| 14 | TTG |
| 15 | NHDWeb |
| 16 | NDJWEB |

#### 标签（`tags[4][]`）— 18个多选 checkbox

| 值 | 显示名称 |
|----|----------|
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 驻站 |
| 9 | 分集 |
| 10 | 完结 |
| 11 | 英字 |
| 12 | 应求 |
| 13 | 大包 |
| 14 | 杜比 |
| 15 | 特效 |
| 17 | 番组 |
| 18 | 连载 |
| 19 | 高码 |
| 20 | 10Bit |
| 21 | 60FPS |

### 1.3 缺失字段

- `processing_sel` — 无地区选择

---

## 二、与其他站点对比

### 2.1 NovaHD 特色

| 维度 | NovaHD | HDVideo | HDFans |
|------|--------|---------|--------|
| 分类 | 13（含番剧/短剧/游戏） | 8 | 16 |
| 分辨率 | 含帧率（60FPS/120FPS） | 标准列表 | 标准列表 |
| 编码 | 合并原盘+压制 | 合并 | 区分 H.264/x264 |
| 音频 | 15种 | 21种 | 24种 |
| 制作组 | 17（含 AGSV/M-Team/BeiTai） | 3 | 30 |
| 标签 | 18（含 60FPS/10Bit/高码/番组） | 25 | 27 |
| MediaInfo | 有 | 无 | 有 |

### 2.2 关键差异

1. **分辨率含帧率** — 4K 60FPS(7) 和 4K 120FPS(8) 是独立选项，需要从 MediaInfo 的 FrameRate 字段判断
2. **番剧分类** — 区分 Anime(412) 和 Anime Series(413)，还有 Short Play(411)
3. **10Bit 标签** — 有独立的 10Bit 标签(20)，需从 MediaInfo BitDepth 判断
4. **60FPS 标签** — 有独立的 60FPS 标签(21)，与分辨率选项 60FPS 对应
5. **自制组** — NHDWeb(15) 和 NDJWEB(16) 是 NovaHD 自家制作组

---

## 三、站点适配器配置参考

```yaml
site:
  id: "novahd"
  name: "NovaHD"
  url: "https://pt.novahd.top"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动画": 405
      "MV": 406
      "体育": 407
      "音乐": 409
      "其他": 410
      "短剧": 411
      "动漫": 412
      "番剧": 413
      "游戏": 414

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
      "UHD": 10
      "WEB-DL": 11
      "DVD": 12

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
      "ALAC": 7
      "TrueHD Atmos": 8
      "DDP": 9
      "AC3": 10
      "LPCM": 11
      "TrueHD": 12
      "DTS-HDMA": 13
      "DTS-X": 14
      "Other": 15

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "4K": 5
      "8K": 6
      "4K 60Fps": 7
      "4K 120Fps": 8

    team_sel:
      "HDSky": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "FRDS": 6
      "beAst": 7
      "CMCT": 8
      "TLF": 9
      "MTeam": 10
      "BeiTai": 11
      "AGSV": 12
      "HDHome": 13
      "TTG": 14
      "NHDWeb": 15
      "NDJWEB": 16

    tags:
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "驻站": 8
      "分集": 9
      "完结": 10
      "英字": 11
      "应求": 12
      "大包": 13
      "杜比": 14
      "特效": 15
      "番组": 17
      "连载": 18
      "高码": 19
      "10Bit": 20
      "60FPS": 21

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
```

---

## 四、发布流水线注意事项

### 4.1 分辨率帧率判断

NovaHD 的分辨率选项含帧率细分，需要从 MediaInfo 提取：

```
FrameRate >= 120 → 8 (2160p/4K 120Fps)
FrameRate >= 60  → 7 (2160p/4K 60Fps)
其他 4K          → 5 (2160p/4K)
```

同时需勾选对应的 60FPS 标签(21)。

### 4.2 10Bit 标签

从 MediaInfo BitDepth 判断：
- BitDepth = 10 → 勾选 10Bit 标签(20)

### 4.3 番剧分类

NovaHD 区分三个动漫相关分类：
- Animations(405) — 动画电影
- Anime(412) — 动漫（单季/单集）
- Anime Series(413) — 番剧（多季合集）

### 4.4 制作组映射

NovaHD 有 17 个制作组，含自家组 NHDWeb(15) 和 NDJWEB(16)。转种时非列表内制作组选 Other(5)。

---

*分析时间：2026-04-16*
*数据来源：https://pt.novahd.top/upload.php 发布页面 HTML 分析*
