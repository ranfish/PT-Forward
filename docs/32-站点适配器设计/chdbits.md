# CHDBits 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | CHDBits |
| 站点地址 | https://ptchdbits.co |
| 站点框架 | NexusPHP（老版本，CSS时间戳 202012051845） |
| 主题 | BlueGene（经典蓝色主题） |
| 口号 | CHDBits - 为会员服务 |
| 特殊功能 | 候选区(offers)、复活区(renew)、字幕区(subtitles)、菠菜(bet)、HR制度 |
| 规则页面 | rules.php |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `torrentfile` | file | ✓ | 种子文件（**注意：非 `file`**） |
| `name` | text | - | 标题（若不填使用种子文件名，不允许出现中文，要求0day命名） |
| `small_descr` | text | - | 副标题（**必须包含影片中文译名**） |
| `url` | text | - | IMDb 链接（有 AutoDesc 按钮） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

**重要差异**:
- 种子文件字段名为 `torrentfile`，而非其他站点的 `file`
- 无 `pt_gen` 字段，但有 AutoDesc 按钮（JavaScript ptgen() 函数）
- 无 `technical_info`（MediaInfo）独立字段——MediaInfo 应写在简介的引用框中
- 副标题必须包含中文译名
- 主标题不允许出现中文

### 1.2 类型字段（`type`）— 必填

**注意**: 无 `data-mode` 属性，无 `[4]` 后缀，无多模式系统。

| 值 | 显示名称 |
|----|----------|
| 401 | Movies |
| 402 | TV Series |
| 403 | TV Shows |
| 404 | Documentaries |
| 405 | Animations |
| 406 | Music |
| 407 | Sports |
| 408 | HQ Audio |
| 409 | Demo |
| 410 | Game |

**注意**: 分类名使用英文（非中文），与大多数 NexusPHP 站点不同。

### 1.3 质量选择字段

**所有质量字段均无 `[4]` 后缀，无模式系统。**

#### 来源（`source_sel`）— 4个

**注意**: 这是 CHDBits 独有字段！不是媒介类型，而是来源类型。

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | 官方 | 官方发布 |
| 7 | 转载 | 转载资源 |
| 8 | 复活区 | 复活区资源 |
| 9 | 原创 | 原创资源 |

#### 媒介（`medium_sel`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 19 | UHD Blu-ray |
| 3 | Remux |
| 4 | Encode |
| 6 | HDTV |
| 18 | WEB-DL |
| 8 | CD |

**注意**: 值不连续（1,19,3,4,6,18,8），UHD Blu-ray=19 是特殊大值。无 DVD、Other 选项。

#### 视频编码（`codec_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 5 | H.265 |
| 6 | MPEG-4 |
| 4 | MPEG-2 |
| 2 | VC-1 |
| 3 | AV1 |

**注意**: 值不按顺序（1,5,6,4,2,3）。有 MPEG-4（6）但无 MPEG-4 在多数其他站点出现。H.265 显示为 "H.265"（非 "H.265/HEVC"）。

#### 音频编码（`audiocodec_sel`）— 10个

| 值 | 显示名称 |
|----|----------|
| 3 | DTS |
| 7 | AC3 |
| 10 | DTS-HD |
| 11 | True-HD |
| 13 | LPCM |
| 1 | FLAC |
| 2 | APE |
| 12 | WAV |
| 6 | AAC |
| 14 | ALAC |

**注意**: 值不连续，无 DTS:X、DTS-HD MA（仅有 DTS-HD）、DDP/E-AC3、MP3、M4A、Other。音频编码数量较少（10个）。

#### 分辨率（`standard_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 5 | Other |
| 7 | 8K |
| 6 | 4K |

**注意**: 值不按分辨率排列（1,2,3,5,7,6）。1080p 和 1080i 分开。无 480p、2K/1440p。

#### 处理（`processing_sel`）— 11个

**注意**: CHDBits 的 `processing_sel` 不是地区，而是剧集类型分类！

| 值 | 显示名称 |
|----|----------|
| 1 | 3D |
| 3 | 美剧 |
| 4 | 日剧 |
| 5 | 港剧 |
| 6 | 韩剧 |
| 7 | 英剧 |
| 8 | 国剧 |
| 9 | 台剧 |
| 10 | 新剧 |
| 11 | 马剧 |
| 13 | 合集 |

### 1.4 制作组（`team_sel`）— 15个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 3 | 压制组 | 通用压制组 |
| 14 | CHDBits | 官组 |
| 13 | SGNB | 官组 |
| 1 | REMUX | 官组Remux |
| 2 | CHDTV | 官组 |
| 15 | CHDPAD | 官组 |
| 12 | CHDWEB | 官组 |
| 11 | CHDHKTV | 官组 |
| 8 | OneHD | 官组 |
| 16 | blucook | 官组 |
| 19 | KAN | 驻站组 |
| 22 | JKCT | 驻站组 |
| 23 | BMDru | 驻站组 |
| 25 | Destiny | 驻站组 |
| 26 | SP | 驻站组 |

**注意**: 制作组分为官组（CHD系列）和驻站组。有"申请驻站"按钮跳转 `resident.php`。

### 1.5 标签 — 8个独立 checkbox

**注意**: 标签不使用 `tags[]` 数组，而是每个标签有独立字段名，且使用 GIF 图标显示！

| 字段名 | value | 图标文件 | 含义 |
|--------|-------|----------|------|
| `perent` | yes | chd-gf.gif | 官方（Official） |
| `first` | yes | chd-sf.gif | 首发（First） |
| `oneself` | yes | chd-dz.gif | 自制（Self-made） |
| `cnlang` | yes | chd-gy.gif | 国语（Chinese Audio） |
| `diy` | yes | chd-diy.gif | DIY |
| `cnsub` | yes | chd-cnsub.gif | 中字（Chinese Subtitle） |
| `limited` | yes | chd-limited.gif | 禁转（Limited） |
| `txsub` | yes | chd-txsub.gif | 特效字幕（Special Effect Sub） |

**重要**: 所有标签的 value 都是 `yes`，需通过字段名区分。

### 1.6 字段命名汇总（与其他站点对比）

| 字段 | CHDBits | 常见 NexusPHP |
|------|---------|---------------|
| 种子文件 | `torrentfile` | `file` |
| 来源 | `source_sel` | 无此字段 |
| 媒介 | `medium_sel` | `medium_sel[4]` |
| 编码 | `codec_sel` | `codec_sel[4]` |
| 音频 | `audiocodec_sel` | `audiocodec_sel[4]` |
| 分辨率 | `standard_sel` | `standard_sel[4]` |
| 处理 | `processing_sel` | `processing_sel[4]`（通常为地区） |
| 制作组 | `team_sel` | `team_sel[4]` |
| 标签 | 独立字段名 | `tags[4][]` |
| PT-Gen | 无（有AutoDesc） | `pt_gen` |
| MediaInfo | 无独立字段 | `technical_info` |

---

## 二、发种规则（rules.php）

### 2.1 上传总则

- 上传者必须对文件拥有合法传播权
- 保证上传速度与做种时间，至少三人完成前不可撤种，做种不少于24小时
- 故意低速上传将被警告甚至取消上传权限
- 发布者获得双倍上传量

### 2.2 上传者资格

- 任何人都能发布资源
- 部分用户需先在**候选区**(offers.php)提交候选
- **游戏类资源**：仅上传员及以上等级可自由上传，其他用户须先提交候选

### 2.3 允许的资源

- 高清视频（Blu-ray/HD DVD 原碟、Remux、HDTV、重编码 ≥ 720p、高清 DV）
- 无损音轨及 CUE（FLAC、APE 等）
- 5.1 声道及以上音轨（DTS、DTSCD 镜像等）、评论音轨
- PC 游戏（必须为原版光盘镜像）
- 7 日内高清预告片
- 高清相关软件和文档

### 2.4 不允许的资源

- DVD/miniBD 资源
- 总体积 < 100MB（例外：高清软件/文档、单曲专辑）
- 标清 upscale
- CAM/TC/TS/SCR/DVDSCR/R5/R5.Line/HalfCD
- RealVideo（RMVB/RM）、FLV
- 单独样片
- < 5.1 声道有损音频（MP3/WMA）
- 无正确 CUE 的多轨音频
- 游戏硬盘版/高压版/非官方镜像/mod/破解补丁
- RAR 等压缩文件
- 禁忌/敏感内容
- 损坏文件、垃圾文件

### 2.5 Dupe 判定规则

"质量重于数量"：

1. **媒介优先级**: Blu-ray/HD DVD > HDTV > DVD > TV
2. 高清版本代替标清
3. **动漫特例**: HDTV 和 DVD 有相同优先级
4. 相同媒介+分辨率：按发布组确定优先级（Scene & Internal 规则）
5. 保留一个 DVD5 大小的重编码版本
6. 不同区域/配音/字幕的原盘不视为重复
7. 无损音轨只保留一个版本（分轨 FLAC 优先级最高）
8. 断种 45 日或发布 18 个月以上的可重新发布
9. 新版本发布后旧版本保留至断种

### 2.6 打包规则

- 套装高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7 日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，高清同分辨率打包）
- 同一艺术家音乐（≥ 5 专辑，两年内新专辑可单独发布）
- 分卷发售的动漫/角色歌/广播剧
- 发布组打包

打包要求：相同媒介、相同分辨率、编码一致、发布组统一（电影）。

### 2.7 标题命名规范

**主标题**（0day 命名，不含中文）：
- 电影：`[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组`
  - 例：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS`
  - 官组例：`Infernal Affairs II 2003 2160p FRA UHD Blu-ray HDR HEVC DTS-HD MA 5.1-Beans@CHDBits`
- 电视剧：`[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组`
  - 例：`越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU`
- 音轨：`[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组]`
- 游戏：`[中文名] 名称 [年份] [版本] [发布说明][-发布组]`

**副标题**: 资源简略信息（中文名、字幕信息、视频格式等）

**简介三部分**:
1. 资源简介（海报+简介）
2. 媒体信息（原盘用 BDInfo，其他用 MediaInfo，用引用框包裹）
3. 至少三张原分辨率截图

### 2.8 转载规则

- 禁止发布网课/教辅资料
- 不得转发无法确认来源或疑似公网的资源
- 不得转发其他站点禁转资源
- 不得在原站未出种前转发
- 不得修改文件结构和命名
- 不得发布跨季或合集打包（官组除外）
- 非官组成员任何情况不得使用 CHDBits 或 CHD 后缀

### 2.9 HR 制度

- H3 种子：20 天内保种 ≥ 72 小时
- H5 种子：20 天内保种 ≥ 120 小时
- 未达标扣 1 HP，HP=0 失去下载权限
- HP 初始值/封顶值 = 5
- 黄星标志及 VIP 免除 HR

### 2.10 SeedBox 规则

- 盒子下载量按黑种计算（VIP/工作组除外）
- 发布 72 小时内，非 VIP 上传量上限为种子体积 3 倍
- 单种上传速率 ≤ 100 MB/s
- 种子发布者始终获得双倍上传量

### 2.11 复活区

- 发布超过 14 天且做种 < 10 的种子
- 可用 2W 做种积分复活
- 复活种子享受 7 天免费且无盒子规则限制

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "Movies": 401,
  "TV Series": 402,
  "TV Shows": 403,
  "Documentaries": 404,
  "Animations": 405,
  "Music": 406,
  "Sports": 407,
  "HQ Audio": 408,
  "Demo": 409,
  "Game": 410
}
```

### 3.2 来源（`source_sel`）

```json
{
  "官方": 1,
  "转载": 7,
  "复活区": 8,
  "原创": 9
}
```

### 3.3 媒介（`medium_sel`）

```json
{
  "Blu-ray": 1,
  "UHD Blu-ray": 19,
  "Remux": 3,
  "Encode": 4,
  "HDTV": 6,
  "WEB-DL": 18,
  "CD": 8
}
```

### 3.4 视频编码（`codec_sel`）

```json
{
  "H.264/AVC": 1,
  "H.265": 5,
  "MPEG-4": 6,
  "MPEG-2": 4,
  "VC-1": 2,
  "AV1": 3
}
```

### 3.5 音频编码（`audiocodec_sel`）

```json
{
  "DTS": 3,
  "AC3": 7,
  "DTS-HD": 10,
  "True-HD": 11,
  "LPCM": 13,
  "FLAC": 1,
  "APE": 2,
  "WAV": 12,
  "AAC": 6,
  "ALAC": 14
}
```

### 3.6 分辨率（`standard_sel`）

```json
{
  "1080p": 1,
  "1080i": 2,
  "720p": 3,
  "Other": 5,
  "8K": 7,
  "4K": 6
}
```

### 3.7 处理（`processing_sel`）

```json
{
  "3D": 1,
  "美剧": 3,
  "日剧": 4,
  "港剧": 5,
  "韩剧": 6,
  "英剧": 7,
  "国剧": 8,
  "台剧": 9,
  "新剧": 10,
  "马剧": 11,
  "合集": 13
}
```

### 3.8 制作组（`team_sel`）

```json
{
  "压制组": 3,
  "CHDBits": 14,
  "SGNB": 13,
  "REMUX": 1,
  "CHDTV": 2,
  "CHDPAD": 15,
  "CHDWEB": 12,
  "CHDHKTV": 11,
  "OneHD": 8,
  "blucook": 16,
  "KAN": 19,
  "JKCT": 22,
  "BMDru": 23,
  "Destiny": 25,
  "SP": 26
}
```

### 3.9 标签

```json
{
  "官方": {"field": "perent", "value": "yes"},
  "首发": {"field": "first", "value": "yes"},
  "自制": {"field": "oneself", "value": "yes"},
  "国语": {"field": "cnlang", "value": "yes"},
  "DIY": {"field": "diy", "value": "yes"},
  "中字": {"field": "cnsub", "value": "yes"},
  "禁转": {"field": "limited", "value": "yes"},
  "特效字幕": {"field": "txsub", "value": "yes"}
}
```

---

## 四、CHDBits 特殊注意事项

### 4.1 字段命名差异大

CHDBits 是老版本 NexusPHP，字段命名与大多数站点不同：
- 种子文件: `torrentfile`（非 `file`）
- 无 `[4]` 后缀，无模式系统
- 标签使用独立字段名（非 `tags[]` 数组）

### 4.2 source_sel 是来源而非媒介

CHDBits 独有"来源"字段（官方/转载/复活区/原创），其他站点通常无此字段。转载种子应选 `source_sel=7`。

### 4.3 processing_sel 是剧集类型

不同于大多数站点的"地区"处理字段，CHDBits 的 processing_sel 用于剧集地区分类（美剧/日剧/港剧等）+ 3D + 合集。

### 4.4 候选区制度

部分用户需先在候选区提交候选后才能发布。游戏类资源仅上传员及以上等级可自由上传。

### 4.5 HR 种子

CHDBits 有严格的 HR 制度（H3/H5），HP 系统可能导致用户失去下载权限。转发资源时需注意目标种子是否为 HR 种子。

### 4.6 SeedBox 黑种

盒子下载量按黑种计算，发布 72 小时内上传上限为体积 3 倍，单种速率 ≤ 100 MB/s。这些限制可能影响辅种。

### 4.7 非官组命名限制

非官组成员**任何情况**不得使用 CHDBits 或 CHD 后缀。转发资源如标题含 CHD 后缀需修改。

### 4.8 简介格式要求严格

简介必须分三部分：海报+简介 → BDInfo/MediaInfo（引用框） → ≥ 3 张原分辨率截图。

### 4.9 标签使用 GIF 图标

标签在页面上显示为 GIF 图标而非文字标签，但提交时仍使用字段名+yes值。

---

## 五、适配器实现要点

### 5.1 字段命名

```go
TorrentFileField: "torrentfile",  // 非 file！
SourceSelField:   "source_sel",   // 来源，非媒介
MediumSelField:   "medium_sel",   // 无 [4] 后缀
CodecSelField:    "codec_sel",
AudioSelField:    "audiocodec_sel",
StandardSelField: "standard_sel",
ProcessingSelField: "processing_sel", // 剧集类型，非地区
TeamSelField:     "team_sel",
TypeField:        "type",
```

### 5.2 标签处理

标签需使用独立字段名而非数组：

```go
Tags: map[string]string{
    "perent":  "yes",  // 官方
    "first":   "yes",  // 首发
    "oneself": "yes",  // 自制
    "cnlang":  "yes",  // 国语
    "diy":     "yes",  // DIY
    "cnsub":   "yes",  // 中字
    "limited": "yes",  // 禁转
    "txsub":   "yes",  // 特效字幕
}
```

### 5.3 来源字段

转发资源时 `source_sel` 应设为 7（转载）。

### 5.4 标题格式

主标题使用 0day 命名格式（英文，不含中文），副标题必须包含中文译名。

---

## 六、与其他 NexusPHP 站点对比

| 特征 | CHDBits | 常见 NexusPHP |
|------|---------|---------------|
| 种子文件字段 | `torrentfile` | `file` |
| 模式系统 | 无 | 通常有 mode=4 |
| 字段后缀 | 无 `[4]` | 通常有 `[4]` |
| 来源字段 | 有（官方/转载/复活区/原创） | 无 |
| 媒介数量 | 7 | 通常 6-13 |
| 编码 | 有 MPEG-4 | 通常无 |
| 音频编码 | 10（无 DTS:X/DDP） | 通常 15-24 |
| 分辨率 | 6（1080i独立） | 通常 5-7 |
| 处理字段 | 剧集类型（11个） | 通常为地区 |
| 制作组 | 15（大量官组） | 通常 3-30 |
| 标签 | 8个独立字段+GIF图标 | 通常 checkbox 数组 |
| PT-Gen | 无（有AutoDesc） | 通常有 |
| MediaInfo | 无独立字段 | 通常有 |
| 分类名 | 英文 | 通常中文 |
| HR 制度 | 有（HP 系统） | 部分站点有 |
| SeedBox 规则 | 严格（黑种/限速） | 少见 |

---

*数据来源: upload.php HTML + rules.php HTML (2026-04-16)*
*文档创建: 2026-04-16*
