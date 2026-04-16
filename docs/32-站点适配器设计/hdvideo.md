# HDVideo 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | HDVideo |
| 站点地址 | https://hdvideo.top |
| 站点框架 | NexusPHP |
| 特殊规则 | HDR 细分标签、音乐/演唱会专属标签、仅3个制作组 |

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
| `hr[4]` | input | - | HR（不明字段） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | 音轨 |
| 407 | 体育 |
| 408 | 音乐MV |

#### 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 10 | UHD Blu-ray |
| 11 | FHD Blu-ray |
| 12 | Remux |
| 13 | Encode |
| 14 | WEB-DL |
| 16 | UHDTV/HDTV |
| 18 | DVD |
| 19 | Other |
| 20 | CD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 6 | HEVC/H.265/x265 |
| 7 | AVC/H.264/x264 |
| 8 | VC-1 |
| 9 | MPEG-2 |
| 10 | VP8/9 |
| 11 | Other |
| 12 | AV1/S |
| 14 | ProRes |

#### 音频编码（`audiocodec_sel[4]`）— 21个选项

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS_HD\|MA |
| 5 | Opus |
| 7 | Other |
| 8 | LPCM/PCM |
| 9 | DD/AC3 |
| 10 | DDP/EAC3 |
| 11 | TrueHD |
| 12 | WAV |
| 14 | TrueHD_Atmos |
| 15 | DTS_HD\|HR |
| 16 | DTS_X |
| 17 | DTS |
| 18 | MP2/3 |
| 19 | TAA |
| 20 | OGG |
| 21 | AAC |
| 22 | MPEG |
| 23 | DDP Atmos/EAC3 |

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 5 | 4320p/8K |
| 6 | 2160p/4K |
| 8 | 1080p |
| 9 | 1080i |
| 10 | 720p |
| 11 | Other |

#### 制作组（`team_sel[4]`）— 仅4个

| 值 | 显示名称 |
|----|----------|
| 1 | HDVWEB |
| 2 | HDVMV |
| 4 | Other |

#### 标签（`tags[4][]`）— 25个多选 checkbox

| 值 | 显示名称 | 类别 |
|----|----------|------|
| 1 | 禁转 | 转载控制 |
| 3 | 官方 | 身份标识 |
| 5 | 国语 | 音轨 |
| 6 | 中字 | 字幕 |
| 8 | 源码 | 技术 |
| 9 | 限转 | 转载控制 |
| 10 | 粤语 | 音轨 |
| 11 | HDR10 | HDR |
| 12 | Dolby Vision | HDR |
| 13 | HLG | HDR |
| 15 | 零魔 | 系统 |
| 16 | 金种 | 系统 |
| 17 | 完结 | 状态 |
| 18 | 原创 | 身份标识 |
| 19 | 特效 | 字幕 |
| 20 | HDR10+ | HDR |
| 21 | 应求 | 状态 |
| 22 | DIY | 类型 |
| 23 | MV | 音乐 |
| 24 | LIVE现场 | 音乐 |
| 25 | 音乐专辑 | 音乐 |
| 26 | 演唱会 | 音乐 |
| 27 | HDR Vivid | HDR |
| 28 | 连载 | 状态 |
| 29 | 首发 | 身份标识 |

### 1.3 缺失字段

- `technical_info` — 无 MediaInfo/BDInfo 专用字段
- `processing_sel` — 无地区选择
- 分辨率缺少 SD 选项

---

## 二、与其他站点对比

### 2.1 HDVideo 特色

| 维度 | HDVideo | HDFans | GTK |
|------|---------|--------|-----|
| 分类数 | 8 | 16 | 12 |
| 媒介 | UHD/FHD Blu-ray 分开 | UHD/BD 各细分4种 | 简单列表 |
| 编码 | 合并原盘+压制 | 区分 H.264/x264 | 合并 |
| 音频 | 21种 | 24种 | 无 |
| 制作组 | 仅3个（HDVWEB/HDVMV/Other） | 30个 | 11个 |
| 标签 | 25个（HDR细分5种，音乐5种） | 27个 | 6个 |
| 地区 | 无 | 13种 | 无 |
| MediaInfo字段 | 无 | 有 | 有 |

### 2.2 关键差异

1. **媒介分 UHD/FHD Blu-ray** — UHD Blu-ray(10) vs FHD Blu-ray(11)，而非按 DIY/Remux 细分
2. **制作组极少** — 仅 HDVWEB、HDVMV、Other，转种时几乎都选 Other(4)
3. **HDR 标签细分** — 5种 HDR 标签：HDR10、HDR10+、Dolby Vision、HLG、HDR Vivid
4. **音乐专属标签** — MV、LIVE现场、音乐专辑、演唱会
5. **无 MediaInfo 专用字段** — 只能放在简介 `descr` 中
6. **分辨率值非标准** — 1080p=8, 720p=10（多数站点为 1-4）

---

## 三、站点适配器配置参考

```yaml
site:
  id: "hdvideo"
  name: "HDVideo"
  url: "https://hdvideo.top"
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
      "音轨": 406
      "体育": 407
      "MV": 408

    medium_sel:
      "UHD Blu-ray": 10
      "FHD Blu-ray": 11
      "Remux": 12
      "Encode": 13
      "WEB-DL": 14
      "HDTV": 16
      "DVD": 18
      "Other": 19
      "CD": 20

    codec_sel:
      "H265": 6
      "H264": 7
      "VC-1": 8
      "MPEG-2": 9
      "VP9": 10
      "Other": 11
      "AV1": 12
      "ProRes": 14

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS-HDMA": 3
      "Opus": 5
      "Other": 7
      "LPCM": 8
      "AC3": 9
      "DDP": 10
      "TrueHD": 11
      "WAV": 12
      "TrueHD Atmos": 14
      "DTS-HDHR": 15
      "DTS-X": 16
      "DTS": 17
      "MP3": 18
      "AAC": 21
      "DDP Atmos": 23

    standard_sel:
      "8K": 5
      "4K": 6
      "1080p": 8
      "1080i": 9
      "720p": 10
      "Other": 11

    team_sel:
      "HDVWEB": 1
      "HDVMV": 2
      "Other": 4

    tags:
      "禁转": 1
      "官方": 3
      "国语": 5
      "中字": 6
      "源码": 8
      "限转": 9
      "粤语": 10
      "HDR10": 11
      "Dolby Vision": 12
      "HLG": 13
      "零魔": 15
      "金种": 16
      "完结": 17
      "原创": 18
      "特效": 19
      "HDR10+": 20
      "应求": 21
      "DIY": 22
      "MV": 23
      "LIVE现场": 24
      "音乐专辑": 25
      "演唱会": 26
      "HDR Vivid": 27
      "连载": 28
      "首发": 29

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
    - "technical_info"
```

---

## 四、发布流水线注意事项

### 4.1 制作组映射

HDVideo 仅有 3 个制作组（HDVWEB、HDVMV、Other）。转种时：
- 源站是 HDVideo 官组 → 映射到 HDVWEB(1) 或 HDVMV(2)
- 其他所有情况 → 使用 Other(4)

### 4.2 媒介映射逻辑

HDVideo 区分 UHD Blu-ray(10) 和 FHD Blu-ray(11)：
- 标准媒介为 UHD + 原盘 → 10 (UHD Blu-ray)
- 标准媒介为 Blu-ray + 1080p → 11 (FHD Blu-ray)
- 需要根据分辨率选择正确的 Blu-ray 子类型

### 4.3 HDR 标签选择

HDVideo 的 HDR 标签细分 5 种，需要从 MediaInfo 或标题中精确识别：
- HDR10 → 标签 11
- HDR10+ → 标签 20
- Dolby Vision → 标签 12
- HLG → 标签 13
- HDR Vivid → 标签 27

### 4.4 MediaInfo 处理

HDVideo 无专用 `technical_info` 字段，MediaInfo 需嵌入 `descr` 简介中（通常用 `[code]` 标签包裹）。

---

*分析时间：2026-04-16*
*数据来源：https://hdvideo.top/upload.php 发布页面 HTML 分析*
