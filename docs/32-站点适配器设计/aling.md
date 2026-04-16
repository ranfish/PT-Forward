# alingPT 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | alingPT |
| 站点地址 | https://pt.aling.de |
| 站点框架 | NexusPHP |
| 主题 | Classic |
| 特殊规则 | 标准 dupe 规则，分类较少（仅4个），地区细分详细 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名） |
| `small_descr` | text | - | 副标题 |
| `pt_gen` | text | - | PT-Gen 链接（支持 IMDb/豆瓣/Bangumi/indienova） |
| `descr` | textarea | ✓ | 简介（BBCode，20行） |
| `technical_info` | textarea | - | MediaInfo/BDInfo（8行） |
| `uplver` | checkbox | - | 匿名发布（未在表单中看到，但规则未明确排除） |

注意：alingPT 无 `url`（IMDb链接）和 `nfo` 字段。PT-Gen 支持4种来源（IMDb/豆瓣/Bangumi/indienova），是已分析站点中最多的。MediaInfo/BDInfo 有详细的使用说明。

### 1.2 质量选择字段

字段名带 `[4]` 后缀，单模式（mode=4）。

#### 类型（`type`）— 必填，data-mode='4'，仅5个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 404 | 纪录片 |
| 405 | 动画 |

注意：仅4个分类，是已分析站点中最少的。无综艺、体育、音乐、游戏、软件等分类。

#### 媒介（`medium_sel[4]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | Blu-ray DIY |
| 3 | Remux |
| 4 | TV |
| 5 | WEB-DL |
| 6 | DVD 原盘 |
| 7 | Encode |
| 8 | Other |

注意：有 Blu-ray DIY(2) 选项。TV(4) 代替 HDTV。无 UHD 独立选项。

#### 视频编码（`codec_sel[4]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264(AVC) |
| 2 | H.265(HEVC) |
| 3 | AV1 |
| 4 | MPEG-2 |
| 5 | VC-1 |
| 6 | Other |

注意：包含 AV1(3)。H.264/H.265 不区分原盘/压制。

#### 分辨率（`standard_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | 8K |
| 2 | 4K |
| 3 | 1080p |
| 4 | 720p |
| 5 | SD |

注意：8K(1) 排在最前面。

#### 制作组（`team_sel[4]`）— 仅3个

| 值 | 显示名称 |
|----|----------|
| 1 | aling |
| 2 | alingWEB |
| 3 | Other |

注意：以 aling 官方组为核心，外部组统一选 Other(3)。

#### 地区（`source_sel[4]`）— 11个

| 值 | 显示名称 |
|----|----------|
| 1 | 内地 |
| 2 | 香港 |
| 3 | 台湾 |
| 4 | 日本 |
| 5 | 朝鲜（韩国） |
| 6 | 印度 |
| 7 | 印尼 |
| 8 | 泰国 |
| 9 | 苏联 |
| 10 | 欧米（欧美） |
| 11 | 其他 |

注意：地区选项极为细分，包含印度(6)、印尼(7)、泰国(8)、苏联(9)，是已分析站点中最详细的地区分类。`source_sel` 在此站点用作地区而非来源类型。

#### 标签（`tags[4][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 4 | DIY |
| 6 | 中字 |
| 7 | HDR |
| 10 | 其他中国方言 |
| 12 | 国语 |
| 13 | 粤语 |
| 14 | Dolby Vision |

注意：标签以语言/地区为主（国语、粤语、其他中国方言），辅以 HDR(7) 和 Dolby Vision(14)。

### 1.3 缺失字段

- `audiocodec_sel` — 无音频编码选择
- `processing_sel` — 无（地区使用 `source_sel[4]`）
- `nfo` — 无 NFO 文件上传
- `url` — 无 IMDb 链接输入框（通过 PT-Gen 生成）

---

## 二、标题命名规范

来源：`rules.php` → 种子信息

### 2.1 标题格式

| 类型 | 格式 | 示例 |
|------|------|------|
| 电影 | `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS |
| 电视剧 | `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU |

### 2.2 简介要求

**必须包含**：
- 海报、横幅或 BD/HDDVD/DVD 封面
- IMDb 链接（电影和电视剧）

**尽可能包含**：
- 画面截图或缩略图和链接
- 文件详细情况（格式、时长、编码、码率、分辨率、语言、字幕等）
- 演职员名单以及剧情概要

### 2.3 原始信息优先

- 如果资源的原始发布信息基本符合规范，尽量使用原始发布信息

---

## 三、发布规则

### 3.1 允许的资源

- 高清视频：Blu-ray/HD DVD 原盘、Remux、HDTV 流媒体、720p+ 重编码
- 标清视频：仅限来源于高清媒介的重编码（480p+）、DVDR/DVDISO/DVDRip/CNDVDRip

### 3.2 禁止的资源

- 总体积 < 100MB
- 标清 upscale 视频
- CAM、TC、TS、SCR、DVDSCR、R5、R5.Line、HalfCD 等低质量
- RAR 压缩文件
- 重复资源
- 涉及禁忌或敏感内容
- 损坏文件、垃圾文件

### 3.3 Dupe 规则（标准 HD 站规则）

**媒介优先级**：Blu-ray/HD DVD > HDTV > DVD > TV

- 同一视频高优先级版本使低优先级版本被判定为重复
- 高清版本使标清版本被判定为重复
- **动漫特例**：HDTV 和 DVD 同优先级
- 相同媒介+相同分辨率：按发布组优先级判定（参考论坛帖子）
- 基于无损截图对比，高质量版本使低质量版本被判定为重复
- 不同区域/不同配音/字幕的 Blu-ray/HD DVD 原盘**不视为重复**
- 无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- 断种45日+ 或发布18月+ → 可重发不受 dupe 约束
- 保留一个 DVD5 大小的最佳画质版本

### 3.4 资源打包规则

- 按套装售卖的高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 分卷发售的动漫剧集/角色歌/广播剧
- 发布组打包资源
- 打包要求：相同媒介、相同分辨率、相同编码格式、相同发布组

### 3.5 促销规则

- **所有新种一律 free 3天**
- 关注度高的种子由管理员设为促销

### 3.6 认领规则

- 发布30天后可认领
- 一个种子30个上限
- 最多认领无限
- 达标标准：180小时 / 10倍体积
- 达标奖励1倍，不达标扣300，主动放弃扣200

---

## 四、站点适配器配置参考

```yaml
site:
  id: "aling"
  name: "alingPT"
  url: "https://pt.aling.de"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  quirks:
    standard_dupe: "标准HD站dupe规则，按媒介+制作组优先级"
    minimal_categories: "仅4个分类，无综艺/体育/音乐/游戏"
    minimal_teams: "仅3个制作组，外部组统一选Other"
    source_as_region: "source_sel用作地区选择，含11个细分地区"
    new_seed_free: "所有新种free 3天"
    pt_gen_multi_source: "PT-Gen支持IMDb/豆瓣/Bangumi/indienova四种来源"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "纪录": 404
      "动漫": 405

    medium_sel:
      "Blu-ray": 1
      "DIY": 2
      "Remux": 3
      "TV": 4
      "WEB-DL": 5
      "DVD": 6
      "Encode": 7
      "Other": 8

    codec_sel:
      "H264": 1
      "H265": 2
      "AV1": 3
      "MPEG-2": 4
      "VC-1": 5
      "Other": 6

    standard_sel:
      "8K": 1
      "2160p": 2
      "1080p": 3
      "720p": 4
      "SD": 5

    team_sel:
      "aling": 1
      "alingWEB": 2
      "Other": 3

    source_sel:
      "大陆": 1
      "香港": 2
      "台湾": 3
      "日本": 4
      "韩国": 5
      "印度": 6
      "印尼": 7
      "泰国": 8
      "苏联": 9
      "欧美": 10
      "其他": 11

    tags:
      "禁转": 1
      "DIY": 4
      "中字": 6
      "HDR": 7
      "其他中国方言": 10
      "国语": 12
      "粤语": 13
      "Dolby Vision": 14

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    source: "source_sel[4]"
    tags: "tags[4][]"
    technical_info: "technical_info"
    pt_gen: "pt_gen"

  missing_fields:
    - "audiocodec_sel"
    - "nfo"
    - "url"
```

---

## 五、发布流水线注意事项

### 5.1 Dupe 规则

发布前需检查站内是否已有：
1. 同一视频的更高媒介优先级版本
2. 相同媒介+相同分辨率的更高发布组优先级版本
3. 动漫特例：HDTV 和 DVD 同优先级

### 5.2 制作组映射

仅3个选项，非 aling/alingWEB 的制作组统一选 Other(3)。

### 5.3 地区字段

`source_sel[4]` 在此站点用作地区选择（非来源类型），是已分析站点中地区最细分的：
- 东亚：内地/香港/台湾/日本/朝鲜（韩国）
- 南亚/东南亚：印度/印尼/泰国
- 其他：苏联/欧米/其他

### 5.4 分类限制

仅4个分类（电影/电视剧/纪录片/动画），综艺、体育、音乐、游戏等类型无对应分类，发布时需选择最接近的分类或避免发布。

### 5.5 PT-Gen 集成

alingPT 的 PT-Gen 支持最广泛的来源：
- IMDb — 电影/电视剧
- 豆瓣 — 电影/音乐/图书
- Bangumi — 动漫/游戏
- indienova — 独立游戏

---

*分析时间：2026-04-16*
*数据来源：https://pt.aling.de/upload.php + rules.php*
