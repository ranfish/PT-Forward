# PTFans 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | PTFans |
| 站点地址 | https://ptfans.cc |
| 站点框架 | NexusPHP |
| 特殊规则 | 双分类模式（综合区mode=4 / 特别区9KG mode=5），**不下载也不发布到特别区** |

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
| `qch_bt_rainbow_enable` | checkbox | - | 彩虹种（站点特色功能） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 双分类模式

PTFans 有两套分类，通过 `data-mode` 区分：

- **mode=4（综合区）**：影视、学习、书籍等常规资源 — **PT-Forward 只使用此模式**
- **mode=5（特别区/9KG）**：成人内容 — **PT-Forward 不下载也不发布**

### 1.3 综合区类型（`type` mode=4）— 18个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 404 | TV Series(电视剧) |
| 405 | TV Shows(综艺) |
| 406 | Documentaries(纪录片) |
| 407 | Music(音乐、专辑、MV、演唱会) |
| 408 | Art(曲艺、相声、小品、戏曲、舞蹈、歌剧、评书等) |
| 409 | Games(游戏及相关) |
| 410 | Science(科学、知识、技能) |
| 411 | School(应试、考级、职称、初中以上教育) |
| 412 | Book(书籍、杂志、报刊、有声书) |
| 413 | Code(IT技术、建模、编程、信息技术、大数据、人工智能) |
| 414 | Animate(3D动画、2.5次元) |
| 415 | ACGN(二次元、漫画) |
| 416 | Baby(婴幼、儿童、早教、小学及相关) |
| 417 | Resource(素材、数据、图片、文档、模板) |
| 418 | Software(软件、系统、程序、APP等) |
| 403 | Sport(体育、竞技、武术及相关) |

注意：分类偏重**教育和学习资源**（Science/School/Book/Code/ACGN/Baby 等），与常规影视站不同。

### 1.4 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | CD |
| 2 | DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | Web-DL |
| 6 | Blu-ray |
| 7 | Encode |
| 8 | Encode |
| 9 | Other |

注意：值7和值8都显示 "Encode"，疑似站点配置错误。

### 1.5 视频编码（`codec_sel[4]`）— 区分原盘/压制

| 值 | 显示名称 |
|----|----------|
| 1 | H.264(x264/AVC) |
| 2 | H.265(x265/HEVC) |
| 3 | Bluray(VC-1) |
| 4 | Bluray(AVC) |
| 5 | Bluray(HEVC) |
| 6 | MPEG-2 |
| 7 | Xvid |
| 8 | AV1 |
| 9 | Other |

注意：编码区分原盘和压制 — Bluray(AVC)(4) vs H.264(x264/AVC)(1)，Bluray(HEVC)(5) vs H.265(x265/HEVC)(2)。

### 1.6 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K |
| 6 | 8K |

### 1.7 制作组（`team_sel[4]`）— 仅6个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |

### 1.8 标签（`tags[4][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | DoVi |

### 1.9 缺失字段

- `processing_sel` — 无地区选择
- `audiocodec_sel` — 无音频编码选择
- `team_sel` 仅5个国内老牌组，转种基本都选 Other(5)

---

## 二、与其他站点对比

| 维度 | PTFans | OKPT | HDFans |
|------|--------|------|--------|
| 分类 | 18（偏教育/学习） | 19（双模式） | 16 |
| 制作组 | 5（最少） | 29 | 30 |
| 标签 | 8（最少） | 18 | 27 |
| 编码 | 区分原盘/压制 | 合并 | 区分 H.264/x264 |
| 音频编码 | 无 | 含整轨/分轨 | 24种 |
| 地区 | 无 | 8种 | 13种 |
| 特色 | 教育资源+9KG区 | 双模式+制作组映射 | 媒介细分20种 |

### 关键差异

1. **分类偏重教育** — 18个分类中约一半与学习/教育相关（Science/School/Book/Code/ACGN/Baby 等），影视分类使用非标准ID（电影=401，电视剧=404而非402）
2. **编码区分原盘/压制** — Bluray(AVC)(4) vs H.264(x264)(1)，Bluray(HEVC)(5) vs H.265(x265)(2)
3. **制作组极少** — 仅 HDS/CHD/MySiLU/WiKi/Other，都是国内老牌组
4. **媒介值7和8重复** — 都显示 "Encode"，需注意避免选错
5. **9KG特别区** — mode=5 的12个分类全为成人内容，PT-Forward 不应触碰

---

## 三、站点适配器配置参考

```yaml
site:
  id: "ptfans"
  name: "PTFans"
  url: "https://ptfans.cc"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  restricted_modes:
    - 5  # 9KG特别区 - 不下载也不发布

  mappings:
    type:
      "电影": 401
      "剧集": 404
      "综艺": 405
      "纪录": 406
      "音乐": 407
      "体育": 403
      "游戏": 409
      "其他": 418

    medium_sel:
      "CD": 1
      "DVD": 2
      "Remux": 3
      "MiniBD": 4
      "WEB-DL": 5
      "Blu-ray": 6
      "Encode": 7
      "Other": 9

    codec_sel:
      "H264": 1
      "H265": 2
      "VC-1": 3
      "Bluray AVC": 4
      "Bluray HEVC": 5
      "MPEG-2": 6
      "Xvid": 7
      "AV1": 8
      "Other": 9

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "4K": 5
      "8K": 6

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "DoVi": 8

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  missing_fields:
    - "audiocodec_sel"
    - "processing_sel"

  codec_mode_map:
    disc_media: ["Blu-ray", "Remux"]
    disc_codecs:
      "AVC": 4
      "HEVC": 5
      "VC-1": 3
    encode_codecs:
      "H264": 1
      "H265": 2
      "AV1": 8
      "Other": 9
```

---

## 四、发布流水线注意事项

### 4.1 编码按媒介类型选择

PTFans 的编码字段区分原盘和压制：
- 原盘/Remux → Bluray(AVC)(4)、Bluray(HEVC)(5)、Bluray(VC-1)(3)
- 压制/WEB → H.264(x264)(1)、H.265(x265)(2)、AV1(8)

适配器需根据媒介类型选择正确的编码值。

### 4.2 媒介值冲突

medium_sel 的值7和值8都显示 "Encode"，建议使用值7。

### 4.3 分类ID非标准

PTFans 的分类ID不遵循常见规律：
- 电影=401（标准）
- 电视剧=404（非常规，通常是402）
- 综艺=405（非常规，通常是403）

### 4.4 9KG特别区过滤

PT-Forward 在 RSS 订阅和发布时必须过滤 mode=5 的内容：
- RSS 中识别到的 9KG 分类种子应跳过
- 发布目标站点不应选择 PTFans 的 mode=5 分类
- 种子详情页如果属于特别区也应跳过

---

*分析时间：2026-04-16*
*数据来源：https://ptfans.cc/upload.php 发布页面 HTML 分析*
