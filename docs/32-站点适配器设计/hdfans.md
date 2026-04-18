# 红豆饭 站点适配器设计

> HDFans 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 红豆饭|
| 站点地址 | https://hdfans.org |
| 站点框架 | NexusPHP |
| 特殊规则 | 媒介细分（20种）、30个制作组、27个标签、24个音频编码 |

---

## 一、发种规范（来自论坛 topicid=2522）

### 1.1 标签说明

| # | 标签 | 说明 |
|---|------|------|
| 1 | 官方 | HDFans 自己制作的作品 |
| 2 | 微光星辰 | HDFans 特别官方制作组 |
| 3 | 官字组 | HDFans 官方字幕组 |
| 4 | 甄选 | 挑选优质资源 |
| 5 | 驻站 | 驻站个人或小组 |
| 6 | 首发 | 首次在网络上发布（很少用） |
| 7 | 原创 | 发布资源中字幕或其他原创属性 |
| 8 | 禁转 | 禁止转发 |
| 9 | 限转 | 在限定期限内不能转到其他站 |
| 10 | 源站转发 | 从源站转发官方种子（需同时满足：源站+官方种子） |
| 11 | DIY | DIY 原盘（增加字幕、音轨、修改菜单等） |
| 12 | 原生 | 原盘资源未被修改（与 DIY 相反） |
| 13 | 国语 | 含有国语音轨 |
| 14 | 粤语 | 含有粤语音轨 |
| 15 | 中字 | 带有中文字幕（简体繁体都算，含外挂中文字幕） |
| 16 | 中英双语 | 中英双语字幕 |
| 17 | 特效 | 特效字幕 |
| 18 | HDR | 视频具备 HDR 属性 |
| 19 | Dolby Vision | 杜比视界 |
| 20 | Atmos | 音轨有全景声 |
| 21 | 4K | 4K 分辨率 / 2160P |
| 22 | 8K | 8K 分辨率 |
| 23 | CC收藏 | 符合 CC 标准的作品 |
| 24 | 完结 | 电视剧打包全集建议添加 |
| 25 | 刮削 | 经过 Plex/Emby/Jellyfin 适配，或添加了歌词 |
| 26 | AI修复 | 使用软件对老旧视频进行修复 |
| 27 | 保种 | 需要长期做种的资源 |
| 28 | 零魔 | 不计算魔力的资源 |

### 1.2 制作组说明

HDFans 已有 30 个制作组，基本不再新增。列表包括：HDFans, CHDBits, HDC, TTG, WiKi, beAst, CMCT, FRDS, HDS, OurBits, PTer, Audies, Ubits, HHClub, HDHome, PTHome, QHstudIo, Hares, TLF, PTP, BTN/NTb, EPSiLON, FraMeSToR, OpenCD, DIC, Red, GGN, LemonHD 等。

### 1.3 注意事项

使用 auto-feed 或类似转种脚本时，需把标签、编码、音轨、地区、制作组等尽量调整好。

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
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 2.2 质量选择字段

字段名带 `[4]` 后缀（`data-mode='4'`）。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | Documentaries/纪录片 |
| 404 | Education/教育 |
| 405 | Audio Books/有声读物 |
| 406 | Music/音乐 |
| 407 | Music Videos/音乐视频 |
| 408 | Concert/演唱会 |
| 409 | Drama/戏剧 |
| 410 | Others/其他 |
| 416 | TV Shows/综艺 |
| 417 | Animations/动漫、动画 |
| 418 | Sports/体育 |
| 419 | Software/软件 |
| 421 | Games/游戏 |
| 423 | E-Books/电子书 |

#### 媒介（`medium_sel[4]`）— 20个细分选项

| 值 | 显示名称 |
|----|----------|
| 17 | UHD原盘 |
| 18 | UHD DIY |
| 19 | UHD Remux |
| 20 | UHD压制 |
| 21 | BD原盘 |
| 22 | BD DIY |
| 23 | BD Remux |
| 24 | 1080P/i压制 |
| 25 | 720P压制 |
| 26 | MiniSD |
| 5 | WEB-DL |
| 6 | HDTV |
| 7 | DVD |
| 9 | CD |
| 10 | Other |
| 16 | SACD |
| 27 | CD+DVD |
| 28 | 黑胶 |
| 30 | CD+VCD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 2 | x264 |
| 3 | H.265/HEVC |
| 4 | x265 |
| 5 | VC-1 |
| 10 | MPEG-2 |
| 11 | MPEG-4 |
| 12 | Xvid |
| 13 | Other |
| 14 | AV1 |

#### 音频编码（`audiocodec_sel[4]`）— 24个选项

| 值 | 显示名称 |
|----|----------|
| 1 | TrueHD Atmos |
| 2 | DTS |
| 3 | DTS:X |
| 4 | DTS-HDMA |
| 5 | DTS-HD HR |
| 6 | True-HD |
| 7 | LPCM |
| 10 | AC3 |
| 11 | AAC |
| 12 | FLAC |
| 13 | APE |
| 14 | WAV |
| 15 | OGG |
| 16 | TTA |
| 17 | MP3 |
| 18 | Other |
| 19 | Dolby Digital/DD |
| 20 | DSD |
| 21 | DDP/DD+/EAC3 |
| 22 | MPEG |
| 23 | DDP Atmos |
| 24 | m4a |
| 25 | ALAC |

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 8K/4320P |
| 2 | 4K/UHD/2160P |
| 3 | 1080P |
| 4 | 1080i |
| 5 | 720P |
| 6 | SD |
| 7 | Other |

#### 地区（`processing_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | CN/中国大陆 |
| 2 | US/美国 |
| 3 | UK/英国 |
| 4 | HK/香港 |
| 5 | TW/台湾 |
| 6 | JP/日本 |
| 7 | KR/韩国 |
| 8 | EU/欧洲 |
| 9 | Other/其他 |
| 10 | IN/印度 |
| 11 | SG/新加坡 |
| 12 | MY/马来西亚 |

#### 制作组（`team_sel[4]`）— 30个选项

| 值 | 显示名称 |
|----|----------|
| 1 | CHDBits |
| 2 | HDC |
| 3 | WiKi |
| 4 | beAst |
| 5 | CMCT |
| 6 | FRDS |
| 7 | HDS |
| 8 | TLF |
| 9 | HDFans |
| 16 | PTHome |
| 17 | OurBits |
| 18 | HDHome |
| 19 | TTG |
| 20 | PTer |
| 26 | LemonHD |
| 27 | Others |
| 28 | Hares |
| 29 | Audies |
| 30 | EPSiLON |
| 31 | FraMeSToR |
| 32 | BTN/NTb |
| 33 | OpenCD |
| 34 | HHClub |
| 35 | DIC |
| 36 | Red |
| 37 | PTP |
| 39 | GGN |
| 40 | QHstudIo |
| 41 | Ubits |

#### 标签（`tags[4][]`）— 27个多选 checkbox

| 值 | 显示名称 |
|----|----------|
| 2 | 微光星辰 |
| 3 | 禁转 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 限转 |
| 9 | Dolby Vision |
| 10 | 粤语 |
| 12 | Atmos |
| 13 | 原创 |
| 14 | 官字组 |
| 15 | 首发 |
| 16 | 甄选 |
| 17 | 中英双语 |
| 18 | 特效 |
| 19 | 4K |
| 20 | 8K |
| 21 | 原生 |
| 22 | CC收藏 |
| 23 | 完结 |
| 24 | 驻站 |
| 26 | 保种 |
| 27 | AI修复 |
| 28 | 刮削 |
| 29 | 源站转发 |
| 30 | Hi-Res |

---

## 三、与其他站点对比

### 3.1 HDFans 特殊之处

| 维度 | HDFans | 典型 NexusPHP（如 GTK/13City） |
|------|--------|-------------------------------|
| 媒介细分 | 20种（UHD原盘/DIY/Remux/压制分开） | 10-13种（简单列表） |
| 视频编码 | 区分 H.264 和 x264、H.265 和 x265 | 合并为 H.264 或 x264 |
| 音频编码 | 24种（含 DSD、ALAC、TTA 等） | 通常无此字段 |
| 地区字段 | 有（13个选项） | 通常无 |
| 制作组 | 30个（含 PTP、BTN、GGN 等外站组） | 10-12个（以国内组为主） |
| 标签数 | 27个（含 AI修复、刮削、Hi-Res 等） | 6-7个 |
| 分类数 | 16个（含戏剧、演唱会等） | 8-12个 |

### 3.2 关键差异

1. **媒介按分辨率+类型细分** — 不是简单的 "Encode"，而是 "UHD压制"（20）vs "1080P/i压制"（24）vs "720P压制"（25），需要根据分辨率+是否DIY动态选择
2. **编码区分原盘和压制** — H.264/AVC（1）vs x264（2），H.265/HEVC（3）vs x265（4），需要根据媒介类型选择
3. **源站转发标签** — 需同时满足"源站匹配"和"官方种子"两个条件

---

## 四、站点适配器配置参考

```yaml
site:
  id: "hdfans"
  name: "HDFans"
  url: "https://hdfans.org"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "纪录": 403
      "教育": 404
      "有声读物": 405
      "音乐": 406
      "MV": 407
      "演唱会": 408
      "戏剧": 409
      "其他": 410
      "综艺": 416
      "动漫": 417
      "体育": 418
      "软件": 419
      "游戏": 421
      "书籍": 423

    medium_sel:
      "UHD原盘": 17
      "UHD DIY": 18
      "UHD Remux": 19
      "UHD压制": 20
      "BD原盘": 21
      "BD DIY": 22
      "BD Remux": 23
      "1080P/i压制": 24
      "720P压制": 25
      "MiniSD": 26
      "WEB-DL": 5
      "HDTV": 6
      "DVD": 7
      "CD": 9
      "Other": 10
      "SACD": 16
      "CD+DVD": 27
      "黑胶": 28
      "CD+VCD": 30

    codec_sel:
      "H264": 1
      "x264": 2
      "H265": 3
      "x265": 4
      "VC-1": 5
      "MPEG-2": 10
      "MPEG-4": 11
      "Xvid": 12
      "Other": 13
      "AV1": 14

    audiocodec_sel:
      "TrueHD Atmos": 1
      "DTS": 2
      "DTS:X": 3
      "DTS-HDMA": 4
      "DTS-HDHR": 5
      "TrueHD": 6
      "LPCM": 7
      "AC3": 10
      "AAC": 11
      "FLAC": 12
      "APE": 13
      "WAV": 14
      "OGG": 15
      "TTA": 16
      "MP3": 17
      "Other": 18
      "DD": 19
      "DSD": 20
      "DDP": 21
      "MPEG": 22
      "DDP Atmos": 23
      "m4a": 24
      "ALAC": 25

    standard_sel:
      "8K": 1
      "4K": 2
      "1080p": 3
      "1080i": 4
      "720p": 5
      "SD": 6
      "Other": 7

    processing_sel:
      "大陆": 1
      "美国": 2
      "英国": 3
      "香港": 4
      "台湾": 5
      "日本": 6
      "韩国": 7
      "欧洲": 8
      "其他": 9
      "印度": 10
      "新加坡": 11
      "马来西亚": 12

    team_sel:
      "CHDBits": 1
      "HDC": 2
      "WiKi": 3
      "beAst": 4
      "CMCT": 5
      "FRDS": 6
      "HDS": 7
      "TLF": 8
      "HDFans": 9
      "PTHome": 16
      "OurBits": 17
      "HDHome": 18
      "TTG": 19
      "PTer": 20
      "LemonHD": 26
      "Others": 27
      "Hares": 28
      "Audies": 29
      "EPSiLON": 30
      "FraMeSToR": 31
      "BTN/NTb": 32
      "OpenCD": 33
      "HHClub": 34
      "DIC": 35
      "Red": 36
      "PTP": 37
      "GGN": 39
      "QHstudIo": 40
      "Ubits": 41

    tags:
      "微光星辰": 2
      "禁转": 3
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "限转": 8
      "Dolby Vision": 9
      "粤语": 10
      "Atmos": 12
      "原创": 13
      "官字组": 14
      "首发": 15
      "甄选": 16
      "中英双语": 17
      "特效": 18
      "4K": 19
      "8K": 20
      "原生": 21
      "CC收藏": 22
      "完结": 23
      "驻站": 24
      "保种": 26
      "AI修复": 27
      "刮削": 28
      "源站转发": 29
      "Hi-Res": 30

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    audiocodec: "audiocodec_sel[4]"
    standard: "standard_sel[4]"
    processing: "processing_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  hooks:
    medium_subdivision: true
    bilingual_tags: true
    source_site_tag: true
```

---

## 五、Hook 实现要点

### 5.1 媒介细分规则

HDFans 的媒介需要根据分辨率+是否DIY动态选择，不能简单映射：

```
标准媒介 → HDFans 媒介映射逻辑：

UHD + 原盘 + DIY   → 18 (UHD DIY)
UHD + 原盘 + 原生   → 17 (UHD原盘)
UHD + Remux         → 19 (UHD Remux)
UHD + Encode        → 20 (UHD压制)
BD  + 原盘 + DIY    → 22 (BD DIY)
BD  + 原盘 + 原生    → 21 (BD原盘)
BD  + Remux         → 23 (BD Remux)
Encode + 1080p/i    → 24 (1080P/i压制)
Encode + 720p       → 25 (720P压制)
Encode + SD         → 26 (MiniSD)
WEB-DL              → 5
HDTV                → 6
DVD                 → 7
CD                  → 9
```

### 5.2 编码细分规则

HDFans 区分原盘编码和压制编码：

```
原盘/Remux → H.264/AVC(1), H.265/HEVC(3)
压制/Encode → x264(2), x265(4)
```

需根据媒介类型选择正确的编码值。

### 5.3 源站转发标签条件

添加"源站转发"标签需同时满足：
1. 种子来自某个站点的官方发布
2. 转发的是该站点自己的官方种子（不是第三方在该站发布的种子）

例如：从 CHD 转发 CHD 官方种子 → 可以用"源站转发"；从 CHD 转发 HDC 官方种子 → 不能用。

### 5.4 双语标签增强

当种子同时有"国语/粤语"标签和"中英双语字幕"时，自动添加"中英双语"标签。

---

## 六、参考资源

- HDFans 站点：https://hdfans.org
- 发种规范：https://hdfans.org/forums.php?action=viewtopic&forumid=1&topicid=2522

---

*分析时间：2026-04-16*
*数据来源：HDFans 论坛 + upload.php 发布页面 HTML 分析*
