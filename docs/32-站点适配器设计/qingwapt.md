# 青蛙 站点适配器设计

> 青蛙（QingWAP）站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 青蛙|
| 站点地址 | https://www.qingwapt.com |
| 站点框架 | NexusPHP |
| 特殊规则 | 三套规范性指南 |

## 核心规范

青蛙站点有三套完整的规范性指南，对发布有严格要求：

1. **视频标题命名规范 v1.05**
2. **青蛙发种规范 v1.02**
3. **转种自查指南 v0.1**

### 1. 视频标题命名规范（0day 命名法）

#### 基本格式

```
剧名 年份 其他信息 分辨率 地区码 来源类型 规格 HDR类型 bit信息 视频编码 音频编码 声道数 对象信息 音轨数-制作组
```

#### 示例

```
The Last 10 Years 2022 1080p BluRay x265 DD 5.1-CMCT
（其他信息为空；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x265 不写 bit 信息）

Sidonia no Kishi 2014-2015 1080p BluRay Hi10P x264 FLAC 2.0-VCB-Studio
（其他信息为空；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x264 10bit 要写为"Hi10P x264"）

Clannad S01-S02+MOVIE REPACK 1080p / 480p BluRay / DVDRip Hi10P x264 FLAC 5.1-mawen1250&fch1993
（其他信息为 REPACK；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x264 10bit 要写为"Hi10P x264"）
（此为多季合集有 DVD 有 BD，多个分辨率压制的情况）

Saki S01-S03 BluRay 1080p / 720p Hi10P x264 FLAC 2.0-VCB-Studio
（无年份可以省略；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x264 10bit 要写为"Hi10P x264"）
```

#### 标题各组成部分详细规则

##### 剧名

- 主标题推荐使用英文名（可从 IMDB 或豆瓣又名中获取）
- 动画可使用 MyAnimeList 的罗马音标题
- 别名与中文名填入副标题

##### 季集信息

- 单季合集：S01（即使只有1季也必须写）
- 多季合集：S01-S05
- 分集：S01E03 或 S01E03-E05
- 每季剧名后缀不同时写为 `XXX Series`
- 完结剧集**不要**在标题写 Complete，请**勾选完结**标签

##### 年份

- 有重名时必须填写
- 无重名时可省略
- 多季合集填写 `最早年份-最晚年份`（如 2015-2022）
- 单集综艺/体育节目填写完整年月日（如 20240326）

##### 其他信息（按优先级检查，属于"不一定有"的项目）

| 优先级 | 项目 | 说明 |
|--------|------|------|
| 1 | 剪辑版本 | Director's Cut、Uncensored、Extended、Unrated、Uncut 等（需官方盖章） |
| 2 | 2in1 | 同时含2个剪辑版本（如影院版+导演剪辑版），3in1 同理 |
| 3 | 版本 | 20th Anniversary Edition、Remaster、4K Remaster、Limited Edition |
| 4 | 特殊比例 | IMAX、Open Matte、MAR |
| 5 | Hybrid | 多源混合（如添加其他源的 DV 层信息） |
| 6 | REPACK/V2 | 文件有改动但主要视频未重新转码（增补CD、扫图等） |
| 7 | RERIP | 主要视频文件被重新转码 |
| 8 | PROPER | 原盘抓取有问题重新发布（仅原盘资源） |
| 9 | MiniBD | CMCT 特色资源 |

##### 分辨率

从 MediaInfo 的 Video - Height 和 Video - Scan type 组合：

| 分辨率 | 说明 |
|--------|------|
| 4320p | 8K |
| 2160p | 4K |
| 1440p | 2K |
| 1080p | Height=1080, Progressive |
| 1080i | Height=1080, Interlaced |
| 720p | Height=720 |
| 576p/576i | PAL |
| 480p/480i | NTSC |
| SD | DVD 类可由制式（NTSC/PAL）代替 |

注意：不要将 p 写成 P，不要使用 4k/2k 字样。黑边裁切导致非标准分辨率（如716p）时可写720p或实际值。

##### 地区码（仅原盘类）

ITA、USA、JPN、HKG、TWN 等，无法判定时可省略。

##### 内容分发方

- 原盘：标准收藏（CC）、电影大师（MOC）、华纳档案馆（MAC）等
- WEB-DL：流媒体厂商缩写（见 wiki 流媒体缩写页）
- HDTV：电视台缩写（无法找到来源时可省略）

##### 片源类型

**原盘类**：Blu-ray / 3D Blu-ray / UHD Blu-ray / Modded Blu-ray / Custom BluRay / NTSC DVD5 / NTSC DVD9 / PAL DVD5 / PAL DVD9 / HD DVD

**压制类**：BluRay / 3D BluRay / UHD BluRay / DVDRip / HDDVDRip

注意：原盘为 `Blu-ray`（带连字符），压制为 `BluRay`（无连字符）。

**WEB类**：省略此项，填写流媒体厂商缩写名。
**HDTV类**：填写电视台名称，无法找到来源时可省略。

##### 规格

Remux / WEB-DL / WEBRip / HDTV / UHDTV / HOU / HSBS

原盘类（Blu-ray/DVD/CD）此项不填。

##### HDR类型

| 类型 | 说明 |
|------|------|
| HDR | 通用 HDR |
| HDR10+ | 动态 HDR |
| DV | Dolby Vision（也可写作 DoVi） |
| DV HDR | Dolby Vision + HDR10 |
| DV HDR10+ | Dolby Vision + HDR10+ |
| HLG | Hybrid Log-Gamma |
| PQ10 | PQ HDR |
| HDR vivid | 国产 HDR 标准 |

SDR 资源不填此项。从 MediaInfo 的 Video - HDR Format 字段判断。

##### bit 信息

仅当 Format=AVC 且 Bit depth=10 bit 时写 `Hi10P`。

常见错误：x265 10bit 只写 x265 即可（不写10bit）；x264 10bit 必须写为 `Hi10P x264`。

##### 视频编码（按媒介严格区分）

**蓝光原盘/REMUX**：AVC / HEVC / MPEG-2 / VC-1
**WEB**：H.264 / H.265 / MPEG-2 / VC-1 / VP9 / AVS+ / AVS3 / AV1
**HDTV**：H264 / H265 / MPEG2 / VP9 / AVS+ / AVS3 / AV1
**压制(Encode)**：x264 / x265 / AV1 / MPEG-2 / VP9

注意：
- 原盘用 AVC/HEVC，压制用 x264/x265，**严禁混用**
- AVC = H.264，HEVC = H.265
- 如果 MediaInfo 的 Writing library 明确有 x264/x265，WEB/HDTV 也可以写 x264/x265

##### 音频编码和声道数

音频编码查看 MediaInfo 的 Audio - Format，多音轨仅标最高规格。

声道数查看 Audio - Channel layout，L/R 有几个单词就是几个声道，LFE 为 0.1 个。

| 标题写法 | 对应格式 |
|----------|----------|
| DD | Dolby Digital (AC3) |
| DDP | Dolby Digital Plus (E-AC3, DD+) |
| DTS | DTS |
| DTS-HD MA | DTS-HD Master Audio |
| DTS-HD HR | DTS-HD High Resolution |
| DTS:X | DTS:X（标题中不需要额外标注） |
| TrueHD | Dolby TrueHD |
| Atmos | Dolby Atmos（基于对象的，写在声道数前） |
| FLAC | FLAC |
| LPCM | LPCM |
| AAC | AAC（2.0 时可省略声道数） |
| MP2/MP3 | MP2/MP3（2.0 时可省略声道数） |
| APE | APE |
| WAV | WAV |

**常见错误**：
- DD 和 DDP 搞混：Dolby Digital Plus = DDP = DD+ = E-AC3；Dolby Digital = DD = AC3
- 标题中不应写作 AC3/E-AC3，应写作 DD/DDP
- AAC/MP2/MP3 且声道为 2.0 时可省略声道数
- Atmos 写在声道数之后（如 `TrueHD Atmos 7.1`）
- DTS:X 不需额外标注
- 评论音轨不计入音轨数

##### 对象信息

Atmos / Auro3D（DTS:X 不需要标注）。省略则视为不存在。

##### 音轨数

正片有多条音轨时写 XAudio（X为数字），评论音轨不计入。仅对压制(Encode)类强制要求。

##### 制作组

- 本站首发：`-你的名字@QingWa`
- 多站官组转种：`-组名`（如 `-VCB-Studio`）
- 个人用户他站转种：`-发布者@原站名`
- 匿名用户他站转种：`-Anonymous@原站名`
- 来源不明：留空或 `-NOGROUP` / `-NoGroup` / `-NoGrp`
- Scene 资源：可省略

#### MediaInfo 生成

1. 打开 MediaInfoOnline（https://mediaarea.net/MediaInfoOnline）
2. 拖入视频文件（多集选任意一集，建议第一集）
3. 点击 **Copy to clipboard** 复制到 MediaInfo 栏

**蓝光原盘请使用 BDInfo 而非 MediaInfo**（https://www.videohelp.com/software/BDInfo）：
1. 打开 BDInfo.exe
2. 点击 Browse 选择蓝光文件夹
3. 勾选主视频 MPLS（一般是最长的）
4. 点击 Scan Bitrates → View Report
5. 复制 quick summary 部分填入

注意：转种时不推荐使用原贴中的 MediaInfo（可能被简化或为中文）。

#### 自查助手

推荐安装油猴插件 [不可蛙-种审/发种自查助手](https://greasyfork.org/zh-CN/scripts/490095-qingwa-torrent-assistant)，用于错误自查，减少种子出错风险。

### 2. 发种规范

#### 重复判定总则

- **完结资源代替分集资源** — 已有完结的种子禁止发布单集资源
- **跨季资源代替相同来源和品质的单季资源** — S01-S08 代替 S01-S07
- **跨季资源代替季数被覆盖的跨季资源**
- **高清资源代替完全重复的低清资源**
- **各个压制版允许共存**
- **压制版与原盘允许共存**
- **各个 DIY 原盘资源允许共存**
- **蓝光资源代替更低清晰度且完全重复的 WEB/HDTV 资源**

**特殊情况**：
- 动漫区如果蓝光对 TV 版作出了极大幅度的修正，TV 资源将予以保留
- 某些上古旧番的高清版仅有 WEB-DL，此情况下 WEB-DL 也将和蓝光或 DVD 一起保留

#### 合集打包规则

- **电影类资源** — 只允许发行商官方原盘合集
  - 目前允许在此基础上的衍生品（DIY、Remux 和 Encode）
  - 发布发行商官方原盘合集及其衍生资源建议简介区域加上蓝光合集的封面
- **严禁私人合集** — 诸如导演、演员、IMDb Top 250、豆瓣 Top 250 等
  - 违反此条可能给予处罚
- **禁止跨季不同分辨率资源打包发布**
  - VCB 资源豁免此条

#### 转种总则

- **当然不准转载禁转种**
- **已有完结的种子禁止发布单集资源**
- **禁止转载超分处理/补帧资源**
- **禁止转载黑名单制作组的资源**
- **不建议转载找不到出处的资源** — 可以加 -NoGroup 或者 -NoGrp，但请自担责任
- **不建议转载灰名单制作组的资源**
- **禁止转载机翻资源**
- **请直接上传原种** — 以便辅种
- **编辑需要** — 如果确有编辑需要，请在简介中明确注明增补内容
  - 注意，"添加了字幕""修正了不规范的名字"并不是合理的"编辑需要"
- **标题和副标题需符合本站规范** — 即使是转载，标题也应修改成本站对应的格式
- **务必写明种子出处** — 实在找不到的也不强求

原出处简介推荐用 quote 标签括起来：
```
[quote]资源简介[/quote]
```

#### 制种总则

- **合法传播权** — 你需要对上传的文件拥有合法的传播权（免责用的废话，别在意）
- **禁止发布超分处理/补帧资源**
- **违规但有价值的资源** — 如果有一些违规但却有价值的资源，请将详细情况告知管理组，我们可能破例允许其发布
- **制种时请不要加入** — 广告文件、病毒、木马、种子中种子、无关文件
- **违法资源** — 不允许发布包括但不限于涉及暴恐、肢解、虐待、色情、政治的违法资源
- **文件名和目录名** — 不建议过长，请控制在 100 中文/200 英文之内
- **特殊符号** — 文件和文件夹名字里不可包含特殊符号（比如斜杠和单双引号）
- **点号** — 点号（.）不可出现在文件或文件夹末尾（Windows 的祖传 bug）
- **分块大小** — 种子的分块大小请控制在 16MB 以下
  - 过大的分块会让某些旧软件无法读取，而且会很卡
- **文件夹包装** — 即使发布的是单文件，也仍然推荐套一层文件夹，将文件夹制种
- **文件夹名字** — 文件夹的名字请好好取，不要用"新建文件夹""新增文件夹"这类摆烂名

### 3. 文件规范

- **分块大小** ≤ 16MB
- **文件名/目录名长度** ≤ 100 中文/200 英文
- **特殊符号** — 不可包含（斜杠、引号等）
- **点号结尾** — 点号不能出现在文件/文件夹末尾

### 4. 自查流程

1. 安装站内大佬写的自查插件
2. **允许的资源** — 检查是否为允许的资源类型
3. **种子文件** — 检查种子文件格式
4. **主标题** — 检查主标题是否符合规范
5. **副标题 & IMDB & PT-Gen & NFO** — 检查副标题和元数据
6. **简介** — 检查简介内容
7. **MediaInfo** — 检查 MediaInfo 信息
8. **类型 & 质量 & 标签** — 检查类型、质量和标签

### 5. 各分区发布要求

#### 所有视频分区

- 压制资源必须提供完整 MediaInfo（推荐 MediaInfoOnline）
- 多集资源提供任意一集的 MediaInfo 即可
- 蓝光原盘必须提供 BDInfo（使用 quick summary）
- 正确选择媒介、视频编码、音频编码、分辨率、制作组
- 副标题可包含：片名中译（建议用豆瓣名）、片名原名、包含内容等
- 简介要求：资源引用 + 海报 + 介绍（可用 PTGen）+ **至少三张视频截图**
- **简介中禁止包含 MediaInfo**（原种有也需删除）
- 禁止发布过短视频（含抖音短视频合集，短剧除外）

#### 音乐区（暂不审核）

- 禁止发布有损音乐
- 允许合集类资源（歌手/公司/组合/社团/作曲者合集），禁止个人精选
- 标题包含码率、作者、专辑名即可
- 简介需有预览图（如专辑封面）
- Log/频谱图至少包含一项（优先 Log）

#### 其他区（暂不审核）

- 游戏、软件、电子书等资源
- 转种可直接使用原标题
- 软件/游戏务必带版本号，零碎文件打成压缩包
- 严禁夹带私货

### 发布页面分析

#### 基本信息

| 项目 | 内容 |
|------|------|
| 表单地址 | https://www.qingwapt.com/takeupload.php |
| 表单方法 | POST multipart/form-data |
| 表单字段数 | 49 个 |
| 站点框架 | NexusPHP |

#### 关键字段说明

| 字段名称 | 类型 | 说明 |
|----------|------|------|
| `name` | text | 主标题（必须符合视频标题命名规范） |
| `small_descr` | text | 副标题 |
| `type` | select-one | 分类类型（电影/剧集/动画等） |
| `url` | text | IMDB 链接 |
| `pt_gen` | text | PT-Gen 链接或内容 |
| `descr` | textarea | 简介/描述正文 |
| `tagcount` | int | 标签数量 |
| `color` | int | 标题颜色 |
| `font` | int | 标题字体 |
| `size` | int | 种子大小（字节） |
| `IMG` | file | 图片上传 |
| `list` | file | 图片列表上传 |
| `quote` | textarea | 引用内容 |
| `md` | textarea | Markdown 内容 |
| `b` | checkbox | 匿名发布（NexusPHP uplver） |
| `i` | checkbox | IMDB 图片上传 |
| `u` | checkbox | 上传中 |
| `upload-descr-btn-preview` | button | 描述预览按钮 |

#### 发布页面表单字段完整列表

```go
// 青蛙站点发布表单字段完整列表
const QingWapPublishFields = []string{
    // 必填字段
    "file",           // 种子文件（必须）
    "name",          // 主标题（必须）
    "type",          // 分类（必须）
    "small_descr",   // 副标题
    "descr",         // 简介/描述正文（必须）
    
    // 可选字段
    "url",           // IMDB 链接
    "pt_gen",       // PT-Gen 内容或链接
    "nfo",           // NFO 文件
    "tagcount",      // 标签数量
    "color",         // 标题颜色
    "font",          // 标题字体
    "size",          // 种子大小
    "list",          // 图片列表上传
    "quote",         // 引用内容
    "md",            // Markdown 内容
    
    // 发布控制字段
    "b",             // 匿名发布（NexusPHP uplver）
    "i",             // IMDB 图片上传
    "u",             // 上传中
    "upload-descr-btn-preview", // 描述预览按钮
}
```

#### 与标准 NexusPHP 表单字段对比

| 标准 NexusPHP 字段 | 青蛙对应字段 | 说明 |
|---------------------|--------------|------|
| `file` | `file` | 完全相同 |
| `name` | `name` | 完全相同 |
| `small_descr` | `small_descr` | 完全相同 |
| `type` | `type` | 完全相同 |
| `url` | `url` | 完全相同 |
| `pt_gen` | `pt_gen` | 完全相同 |
| `descr` | `descr` | 完全相同 |
| `nfo` | `nfo` | 完全相同 |
| `b` | `b` | 完全相同 |
| `i` | `i` | 完全相同 |
| `u` | `u` | 完全相同 |

#### 青蛙站点特殊字段

| 字段 | 特殊说明 |
|------|----------|
| `tagcount` | 青蛙特有字段，控制标签显示数量 |
| `color` | 标题颜色 |
| `font` | 标题字体 |
| `list` | 图片列表上传（青蛙特有） |
| `quote` | 引用内容（青蛙特有） |
| `md` | Markdown 内容（青蛙特有） |

#### 发布页面截图说明

青蛙站点发布页面截图已获取（已清理）：
- `/tmp/qingwapt_upload_page.png` - 发布页面基础截图
- `/tmp/qingwapt_upload_page_detailed.png` - 发布页面完整截图

#### 发布流程与表单映射

发布流水线中的字段映射阶段（§11.10 字段映射）：

1. **标准化参数映射** — `standardized_params.type` → `type` 字段
2. **站点配置映射** — 根据 `site_config.mappings.type` 映射到具体分类值
3. **表单字段构造** — 构建完整的 `formFields` map[string]string
4. **发布表单提交** - 提交到 `takeupload.php`
5. **匿名发布处理** — 应用 `b` 字段（§21.14 匿名发布配置）

#### 示例表单字段值

```go
// 发布表单字段示例
formFields := map[string]string{
    "name":          "The Last 10 Years 2022 1080p BluRay x265 DD 5.1-CMCT",
    "small_descr":   "2022 1080p BluRay x265 DD 5.1-CMCT",
    "type":          "movie",                    // 根据 mapping 映射到具体值
    "url":           "https://www.imdb.com/title/tt0090080/",
    "pt_gen":       "https://qingwapt.com/ptgen?title=The+Last+10+Years&year=2022",
    "descr":         "简介内容...",
    "nfo":           "NFO 内容...",
    "b":             "yes",                   // 匿名发布
    "i":             "",                     // 不上传 IMDB 图片
    "u":             "",                     // 不显示上传中状态
    "tagcount":      "3",                   // 标签数量
    "color":         "0",                    // 标题颜色（0=默认）
    "font":          "0",                    // 标题字体（0=默认）
    "size":          "0",                    // 自动计算
}
```

---

#### 发布流程与表单映射

发布流水线中的字段映射阶段（§11.10 字段映射 + §11.11 SitePublisher 接口）：

```
┌─ 步骤 1: 标准化参数生成
├→ standard_params.type → 根据站点配置映射到具体分类值
└→ standard_params.resolution → 映射到具体分辨率值

┌─ 步骤 2: 表单字段映射（根据青蛙站点发布页面分析）
│
├── 标准化映射
│   ├── standardized_params.type → formFields["type"]
│   ├── standardized_params.resolution → formFields["standard_sel"]
│   ├── standardized_params.medium → formFields["medium_sel"]
│   ├── standardized_params.video_codec → formFields["codec_sel"]
│   └── standardized_params.audio_codec → formFields["audiocodec_sel"]
│
└── 站点特殊字段
    ├── req.Title → formFields["name"]
    ├── req.Subtitle → formFields["small_descr"]
    ├── req.IMDbLink → formFields["url"]
    ├── req.DoubanLink → formFields["pt_gen"]
    ├── req.Description → formFields["descr"]
    ├── req.PieceSize → formFields["size"]
    └── req.ExtraFields["b"] → formFields["b"]
```

#### 标签列表（来自 Wiki 标签规则页）

以下为青蛙站点的完整标签定义，用于发布时勾选：

| 标签 | 说明 | 自动/手动 |
|------|------|-----------|
| 官方 | 官方组成员发布带官方后缀的资源 | 自动（普通用户无需选） |
| 驻站 | 驻站组成员发布带驻站小组后缀的资源 | 自动（普通用户无需选） |
| VCB-Studio | VCB-Studio 小组作品 | 手动 |
| 国语 | 含国语（普通话）音轨 | 手动 |
| 粤语 | 含粤语音轨 | 手动 |
| 中字 | 内嵌中文硬字幕、内封中文软字幕或包含外挂中文字幕 | 手动 |
| 特效字幕 | 带有特效字幕的视频 | 手动 |
| DIY | DIY 资源（Custom Disc） | 手动 |
| 原生原盘 | 未经修改的原盘资源（蓝光原盘、DVD原盘） | 手动 |
| Remux | 由原盘提取未经重编码的 Remux 资源 | 手动 |
| 分集 | 连载中剧集的某几集（需申请） | 手动 |
| 完结 | 完结的**剧集**（电影类不要选择） | 手动 |
| 杜比视界 | 含有杜比视界的视频 | 手动 |
| HDR | 含有 HDR10（静态）的视频 | 手动 |
| HDR10+ | 含有 HDR10+（动态）的视频，**必须同时选择 HDR 标签** | 手动 |
| 儿童片 | 适合儿童观看的家庭片与教育片 | 手动 |
| 禁转 | 官方组/驻站组/发布者自制不希望被转载的资源 | 手动 |
| 系列合集 | 多季合一或系列电影打包的资源 | 手动 |
| 零魔 | 系统自动判断：做种数/完成数>=3 且做种数>50 | 自动 |

#### 黑名单和灰名单制作组（禁止/不建议转载）

**黑名单（禁止转载）**：

| 制作组 | 原因 |
|--------|------|
| DBD-Raws | 盗用资源、超分、劣迹斑斑 |
| Skymoon/天月/HKACG | 反华组 |
| c.c动漫 | 改名组 |
| 猎户发布组/orion origin、爪爪字幕组/ZhuaZhuaStudio | 机翻组 |

**黑名单（盗转/改名发布组，禁止转载）**：

FGT, NSBC, BATWEB, GPTHD, DreamHD, BlackTV, CatWEB, Xiaomi, Huawei, MOMOWEB, DDHDTV, SeeWeb, TagWeb, SonyHD, MiniHD, BitsTV, ALT, LelveTV, NukeHD, ZeroTV, HotTV, EntTV, GameHD, SmY, SeeHD, ParkHD, VeryPSP, DWR, XLMV, XJCTV, Mp4Ba, Huluwa, CTRLHD(非CtrlHD), HotWEB, TBMaxUB, BestWEB

**灰名单（不建议转载）**：

| 制作组 | 原因 |
|--------|------|
| 异域11番小队、加刘景长 | 低码率 |
| Reinforce | 高体积渣画质 |

#### 发布流程

```go
// QingWapPublishAction 发布动作实现
func (a *QingWapPublishAction) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
    // 步骤 1: 验证标题命名规范
    if err := a.validateTitleNaming(req); err != nil {
        return nil, fmt.Errorf("标题命名规范验证失败: %w", err)
    }

    // 步骤 2: 验证文件规范
    if err := a.validateFileSpecs(req); err != nil {
        return nil, fmt.Errorf("文件规范验证失败: %w", err)
    }

    // 步骤 3: 处理重复判定逻辑
    if err := a.handleDuplicateRules(req); err != nil {
        return nil, fmt.Errorf("重复判定规则处理失败: %w", err)
    }

    // 步骤 4: 执行自查流程
    if err := a.runSelfCheck(req); err != nil {
        return nil, fmt.Errorf("自查流程失败: %w", err)
    }

    // 步骤 5: 构建发布表单
    formFields, err := a.buildPublishForm(req)
    if err != nil {
        return nil, err
    }

    // 步骤 6: 执行发布
    result, err := a.executePublish(ctx, req, formFields)
    if err != nil {
        return nil, err
    }

    // 步骤 7: 记录发布日志
    a.logPublishResult(ctx, req, result)

    return result, nil
}

// buildPublishForm 构建发布表单
func (a *QingWapPublishAction) buildPublishForm(req *PublishRequest) (map[string]string, error) {
    siteConfig := a.loadSiteConfig(req.SiteCode)
    if siteConfig == nil {
        return nil, errors.New("站点配置未找到")
    }

    // 构建表单字段
    formFields := make(map[string]string)

    // 基本字段
    formFields["name"] = req.Title
    formFields["small_descr"] = req.Subtitle
    formFields["url"] = req.IMDbLink
    formFields["pt_gen"] = req.DoubanLink

    // 描述（包含 MediaInfo、截图等）
    formFields["descr"] = a.buildDescription(req)

    // 文件大小
    if req.PieceSize != nil {
        formFields["size"] = fmt.Sprintf("%d", *req.PieceSize)
    }

    // 分类映射
    if standardizedType := req.StandardizedParams["type"]; standardizedType != "" {
        formFields["type"] = siteConfig.Mappings["type"][standardizedType]
    }

    // 分辨率映射
    if resolution := req.StandardizedParams["resolution"]; resolution != "" {
        formFields["standard_sel"] = siteConfig.Mappings["resolution"][resolution]
    }

    // 媒介映射
    if medium := req.StandardizedParams["medium"]; medium != "" {
        formFields["medium_sel"] = siteConfig.Mappings["medium"][medium]
    }

    // 视频编码映射
    if codec := req.StandardizedParams["video_codec"]; codec != "" {
        formFields["codec_sel"] = siteConfig.Mappings["video_codec"][codec]
    }

    // 音频编码映射
    if audioCodec := req.StandardizedParams["audio_codec"]; audioCodec != "" {
        formFields["audiocodec_sel"] = siteConfig.Mappings["audio_codec"][audioCodec]
    }

    // 来源地区映射
    if sourceArea := req.StandardizedParams["source_area"]; sourceArea != "" {
        formFields["source_area_sel"] = siteConfig.Mappings["source_area"][sourceArea]
    }

    // 匿名发布处理
    if siteConfig.Anonymous != nil && *siteConfig.Anonymous.EnabledValue != "" {
        formFields["uplver"] = *siteConfig.Anonymous.EnabledValue
    }

    // 发布参数落盘（调试支持）
    if os.Getenv("DEV_ENV") == "true" {
        a.dumpPublishParams(req, formFields)
    }

    return formFields, nil
}

// buildDescription 构建描述内容
func (a *QingWapPublishAction) buildDescription(req *PublishRequest) string {
    var parts []string

    // 1. PTGen 元数据
    if req.DoubanLink != "" {
        parts = append(parts, fmt.Sprintf("豆瓣链接: %s\n", req.DoubanLink))
    }

    // 2. MediaInfo（青蛙站点必须）
    if req.MediaInfo != "" {
        parts = append(parts, "\n")
        parts = append(parts, "MediaInfo:\n")
        parts = append(parts, req.MediaInfo)
    }

    // 3. 截图
    if req.Screenshots != "" {
        parts = append(parts, "\n")
        parts = append(parts, "截图:\n")
        parts = append(parts, req.Screenshots)
    }

    // 4. 简介/备注
    if req.Description != "" {
        parts = append(parts, "\n")
        parts = append(parts, "简介:\n")
        parts = append(parts, req.Description)
    }

    return strings.Join(parts, "\n")
}

// loadSiteConfig 加载站点配置
func (a *QingWapPublishAction) loadSiteConfig(siteCode string) (*SitePublishConfig, error) {
    // 实现站点配置加载逻辑
    return nil, nil
}
```

#### 发布状态机

青蛙站点发布流程状态机：

```
[PENDING] → [CHECKING] → [CHECK_FAILED] → [VALIDATING] → [PUBLISHING] → [PUBLISHED] → [FAILED] → [CANCELLED]
                                      ↓
                                  [DELETED]（管理员或系统删除）
```

---

## Hook 实现

### QingWapHook 结构

```go
// QingWapHook 青蛙站点特异化钩子
type QingWapHook struct{}

func (h *QingWapHook) BeforePublish(ctx context.Context, req *PublishRequest) error {
    if req.ExtraFields == nil {
        req.ExtraFields = make(map[string]string)
    }

    // 步骤 1: 验证标题命名规范
    if err := h.validateTitleNaming(req); err != nil {
        return fmt.Errorf("标题命名规范验证失败: %w", err)
    }

    // 步骤 2: 验证文件规范
    if err := h.validateFileSpecs(req); err != nil {
        return fmt.Errorf("文件规范验证失败: %w", err)
    }

    // 步骤 3: 处理重复判定逻辑
    if err := h.handleDuplicateRules(req); err != nil {
        return fmt.Errorf("重复判定规则处理失败: %w", err)
    }

    // 步骤 4: 执行自查流程
    if err := h.runSelfCheck(req); err != nil {
        return fmt.Errorf("自查流程失败: %w", err)
    }

    return nil
}

func (h *QingWapHook) AfterPublish(ctx context.Context, result *PublishResult) error {
    return nil
}
```

### 验证函数

#### validateTitleNaming 验证标题命名规范

```go
// validateTitleNaming 验证标题命名规范
func (h *QingWapHook) validateTitleNaming(req *PublishRequest) error {
    title := strings.TrimSpace(req.Title)
    if title == "" {
        return errors.New("标题不能为空")
    }

    // 检查剧集季度编号格式
    if h.isSeries(req.StandardizedParams) {
        if !h.hasSeasonNumber(title) {
            return errors.New("剧集必须有季度编号（S01 格式）")
        }
    }

    // 检查压制版关键字
    if h.isEncode(req.StandardizedParams) {
        if strings.Contains(title, "规格") {
            return errors.New("压制版不应包含'规格'字段")
        }
        if !strings.Contains(title, "HDR") && strings.Contains(title, "HDR") {
            return errors.New("非 HDR 不应写 HDR 类型")
        }
    }

    // 检查 x264 10bit 格式
    if strings.Contains(strings.ToUpper(title), "X264") {
        if !strings.Contains(title, "Hi10P x264") && h.has10Bit(req.StandardizedParams) {
            return errors.New("x264 10bit 必须写为'Hi10P x264'")
        }
    }

    return nil
}
```

#### validateFileSpecs 验证文件规范

```go
// validateFileSpecs 验证文件规范
func (h *QingWapHook) validateFileSpecs(req *PublishRequest) error {
    // 检查分块大小
    if req.PieceSize != nil && *req.PieceSize > 16*1024*1024 {
        return errors.New("分块大小不能超过 16MB")
    }

    // 检查文件名长度
    if req.FileName != "" {
        // 中文 100 字，英文 200 字
        chineseCount := h.countChinese(req.FileName)
        englishCount := len([]rune(req.FileName)) - chineseCount
        if chineseCount > 100 || englishCount > 200 {
            return errors.New("文件名过长（≤100 中文/200 英文）")
        }

        // 检查特殊符号
        if h.hasSpecialChars(req.FileName) {
            return errors.New("文件名包含特殊符号（斜杠、引号等）")
        }

        // 检查点号结尾
        if strings.HasSuffix(req.FileName, ".") {
            return errors.New("文件名不能以点号结尾")
        }
    }

    return nil
}
```

#### handleDuplicateRules 处理重复判定逻辑

```go
// handleDuplicateRules 处理重复判定逻辑
func (h *QingWapHook) handleDuplicateRules(req *PublishRequest) error {
    // 步骤 1: 检查是否有完结资源
    if h.hasCompletedResource(req) {
        return h.checkIfShouldReplaceSingleEpisodes(req)
    }

    // 步骤 2: 检查跨季资源
    if h.isCrossSeason(req) {
        return h.checkCrossSeasonReplacement(req)
    }

    // 步骤 3: 检查高清资源
    if h.isHighDefinition(req) {
        return h.checkLowDefinitionReplacement(req)
    }

    return nil
}
```

#### runSelfCheck 执行自查流程

```go
// runSelfCheck 执行自查流程
func (h *QingWapHook) runSelfCheck(req *PublishRequest) error {
    // 检查点 1: 检查允许的资源
    if err := h.checkAllowedResource(req); err != nil {
        return fmt.Errorf("允许的资源检查失败: %w", err)
    }

    // 检查点 2: 检查种子文件
    if err := h.checkTorrentFile(req); err != nil {
        return fmt.Errorf("种子文件检查失败: %w", err)
    }

    // 检查点 3: 检查主标题
    if err := h.checkMainTitle(req); err != nil {
        return fmt.Errorf("主标题检查失败: %w", err)
    }

    // 检查点 4: 检查副标题 & IMDB & PT-Gen & NFO
    if err := h.checkSubtitleAndMetadata(req); err != nil {
        return fmt.Errorf("副标题&元数据检查失败: %w", err)
    }

    // 检查点 5: 检查简介
    if err := h.checkDescription(req); err != nil {
        return fmt.Errorf("简介检查失败: %w", err)
    }

    // 检查点 6: 检查 MediaInfo
    if err := h.checkMediaInfo(req); err != nil {
        return fmt.Errorf("MediaInfo 检查失败: %w", err)
    }

    // 检查点 7: 检查类型 & 质量 & 标签
    if err := h.checkTypeQualityTags(req); err != nil {
        return fmt.Errorf("类型&质量&标签检查失败: %w", err)
    }

    return nil
}
```

### 辅助方法

```go
// isSeries 判断是否为剧集
func (h *QingWapHook) isSeries(standardized map[string]any) bool {
    if standardized == nil {
        return false
    }
    type_ := strings.ToLower(toStringAny(standardized["type"], ""))
    return type_ == "category.tv_series" || type_ == "category.animation"
}

// hasSeasonNumber 检查季度编号格式
func (h *QingWapHook) hasSeasonNumber(title string) bool {
    return regexp.MustCompile(`S\d{2}`).MatchString(title)
}

// isEncode 判断是否为压制版
func (h *QingWapHook) isEncode(standardized map[string]any) bool {
    if standardized == nil {
        return false
    }
    medium := strings.ToLower(toStringAny(standardized["medium"], ""))
    return strings.Contains(medium, "encode")
}

// has10Bit 检查是否为 10bit
func (h *QingWapHook) has10Bit(standardized map[string]any) bool {
    if standardized == nil {
        return false
    }
    bit := strings.ToLower(toStringAny(standardized["bit_depth"], ""))
    return bit == "10bit"
}

// countChinese 统计中文字符数
func (h *QingWapHook) countChinese(s string) int {
    var count int
    for _, r := range s {
        if unicode.Is(unicode.Han, r) {
            count++
        }
    }
    return count
}

// hasSpecialChars 检查特殊符号
func (h *QingWapHook) hasSpecialChars(s string) bool {
    specialChars := []string{`/`, `\`, `"`, `'`, "`", "*"}
    for _, char := range specialChars {
        if strings.Contains(s, char) {
            return true
        }
    }
    return false
}

// hasCompletedResource 检查是否为完结资源
func (h *QingWapHook) hasCompletedResource(req *PublishRequest) bool {
    // 检查是否为完结资源（通过标题或元数据判断）
    title := strings.ToLower(req.Title)
    return strings.Contains(title, "complete") ||
           strings.Contains(title, "完结") ||
           strings.Contains(title, "全集")
}

// isCrossSeason 检查是否为跨季资源
func (h *QingWapHook) isCrossSeason(req *PublishRequest) bool {
    title := strings.ToLower(req.Title)
    return regexp.MustCompile(`S\d{2}-S\d{2}`).MatchString(title)
}

// isHighDefinition 检查是否为高清资源
func (h *QingWapHook) isHighDefinition(req *PublishRequest) bool {
    // 检查是否为高清资源（≥720p）
    resolution := strings.ToLower(req.StandardizedParams["resolution"])
    return strings.Contains(resolution, "1080p") ||
           strings.Contains(resolution, "2160p")
}

// checkAllowedResource 检查是否为允许的资源
func (h *QingWapHook) checkAllowedResource(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkTorrentFile 检查种子文件
func (h *QingWapHook) checkTorrentFile(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkMainTitle 检查主标题
func (h *QingWapHook) checkMainTitle(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkSubtitleAndMetadata 检查副标题 & IMDB & PT-Gen & NFO
func (h *QingWapHook) checkSubtitleAndMetadata(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkDescription 检查简介
func (h *QingWapHook) checkDescription(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkMediaInfo 检查 MediaInfo
func (h *QingWapHook) checkMediaInfo(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkTypeQualityTags 检查类型 & 质量 & 标签
func (h *QingWapHook) checkTypeQualityTags(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}
```

## 配置示例

青蛙站点使用标准的 NexusPHP 配置，无需特殊配置。

## 测试用例

### 标题命名规范测试

```go
func TestQingWapHook_ValidateTitleNaming(t *testing.T) {
    tests := []struct {
        name     string
        title    string
        wantErr  bool
    }{
        {"正常标题", "The Last 10 Years 2022 1080p BluRay x265 DD 5.1-CMCT", false},
        {"剧集无季度", "Game of Thrones 2022 1080p BluRay x265 DD 5.1-CMCT", true},
        {"压制版有规格", "Test 2022 Encode 1080p 规格 BluRay x265 DD 5.1-CMCT", true},
        {"x264 10bit", "Test 2022 1080p BluRay Hi10P x264 DD 5.1-CMCT", false},
        {"x264 10bit 错误", "Test 2022 1080p BluRay x264 DD 5.1-CMCT", true},
    }

    hook := &QingWapHook{}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := &PublishRequest{
                Title:            tt.title,
                StandardizedParams: map[string]any{},
            }

            err := hook.validateTitleNaming(req)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateTitleNaming() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 文件规范测试

```go
func TestQingWapHook_ValidateFileSpecs(t *testing.T) {
    tests := []struct {
        name     string
        fileName string
        wantErr  bool
    }{
        {"正常文件名", "测试文件名.txt", false},
        {"文件名过长（中文）", strings.Repeat("中", 101) + ".txt", true},
        {"文件名过长（英文）", strings.Repeat("a", 201) + ".txt", true},
        {"文件名有特殊符号", "测试/文件名.txt", true},
        {"文件名以点结尾", "测试文件名.", true},
    }

    hook := &QingWapHook{}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := &PublishRequest{
                FileName: tt.fileName,
            }

            err := hook.validateFileSpecs(req)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateFileSpecs() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## 参考资源

- 青蛙站点：https://www.qingwapt.com
- 青蛙 Wiki：https://wiki.qingwapt.org/docs
- 发种规则（标题命名规范）：https://wiki.qingwapt.org/docs/rules/content-rules/upload-title
- 重复和合集规则：https://wiki.qingwapt.org/docs/rules/content-rules/duplicate-collection
- 制种和转种规则：https://wiki.qingwapt.org/docs/rules/content-rules/torrent-transfer
- 黑名单和灰名单：https://wiki.qingwapt.org/docs/rules/content-rules/blacklist
- 标签规则：https://wiki.qingwapt.org/docs/rules/content-rules/tags
- 分区发布规则：https://wiki.qingwapt.org/docs/rules/content-rules/category-rules
- MediaInfo和BDInfo教程：https://wiki.qingwapt.org/docs/rules/content-rules/upload-tutorials
- 影片参数详解：https://wiki.qingwapt.org/docs/guides/content-creation/mediainf
- 杜比视界：https://wiki.qingwapt.org/docs/guides/content-creation/dv
- 自查助手（油猴插件）：https://greasyfork.org/zh-CN/scripts/490095-qingwa-torrent-assistant

## 审核脚本完整逆向分析

### 脚本信息

| 项目 | 内容 |
|------|------|
| 名称 | qingwa-torrent-assistant |
| 来源 | Greasyfork #490095 |
| 版本 | 1.1.1 |
| 作者 | QingWaPT-Official |
| 致谢 | 不可说-Torrent-Assistant, 末日-Torrent-Assistant |
| 大小 | 1886 行 / 82KB（含 Greasyfork HTML 包装，实际 JS 约 1350 行） |
| 运行页面 | `upload.php*`（发布页） |
| 权限 | GM_xmlhttpRequest / GM_setValue / GM_getValue |

> **基于不可说/末日审核脚本改写**，结构与 Agsv-Torrent-Assistant 高度相似，但规则针对青蛙站定制。

### 常量映射

#### 分类 (cat_constant)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | 其他 |
| 412 | 短剧 |

> **注意**：音乐(408)和其他(409)分类**跳过大部分校验规则**（仅音乐额外检查采样频率和比特率）。

#### 媒介 (type_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | UHD Blu-ray | `uhd.*blu-?ray` |
| 8 | Blu-ray | `blu-?ray`（无 x264/5、非 remux） |
| 9 | Remux | `remux` |
| 10 | Encode | `bluray/blu-ray` + `x26[45]`（Encode 格式） |
| 7 | WEB-DL | `web-?dl`（无 x264/5） |
| 4 | HDTV | `hdtv` |
| 11 | MiniBD | `minibd` |
| 2 | DVD | `dvd`（无 x264/5） |
| 3 | CD | - |
| 5 | Track | - |
| 6 | Other | - |

> **关键区分**：原盘 `Blu-ray`（连字符）vs 压制 `BluRay`（无连字符）。Encode 类标题须用 `BluRay`。

#### 视频编码 (encode_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | H.264/AVC | `x264\|h264\|h.264\|avc` |
| 6 | H.265/HEVC | `x265\|h265\|h.265\|hevc` |
| 2 | VC-1 | `vc-?1` |
| 4 | MPEG-2 | `mpeg-?2` |
| 7 | AV1 | `av1` |
| 3 | MPEG-4 | `mpeg-?4` |
| 8 | VP9 | `vp9` |
| 5 | Other | - |

> **命名规范严格**：原盘用 AVC/HEVC，WEB 用 H.264/H.265，Encode 用 x264/x265。HEVC≠H.265 视媒介类型区分。

#### 音频编码 (audio_constant)

| ID | 名称 | 标题匹配关键词 |
|----|------|--------------|
| 6 | DTS:X | `dts.x\|dts-x\|dtsx` |
| 11 | DDP | `ddp\|dd+\|e-?ac3\|dolby digital plus` |
| 14 | TrueHD Atmos | `truehd.*atmos` |
| 12 | TrueHD | `truehd` |
| 19 | Atmos | `atmos` |
| 16 | DTS-HD MA | `dts-hd.?ma\|dtshdma` |
| 17 | DTS-HD HR | `dts-hd.?hr\|dtshdhr` |
| 2 | DTS | `dts`（排除 dts-hd 等） |
| 1 | DD | `ac-?3\|dd[^p+\|dolby digital[^+]` |
| 4 | FLAC | `flac` |
| 3 | AAC | `aac` |
| 5 | LPCM | `lpcm\|pcm` |
| 7 | ALAC | `alac` |
| 8 | WAV | `wave?\b` |
| 9 | OGG | `ogg` |
| 10 | OPUS | `opus` |
| 13 | MPEG | `mp[23]` |
| 15 | AV3A | `av3a` |
| 18 | USAC | `usac` |
| 20 | M4A | `m4a` |
| 21 | ADPCM | `adpcm` |
| 22 | Other | - |

> **DD vs DDP 严格区分**：AC3=DD，E-AC3=DDP。标题禁止写 AC3/E-AC3，必须写 DD/DDP。

#### 分辨率 (resolution_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 6 | 8K | `8k\|4320p` |
| 7 | 4K | `2160p\|4k[.\| ]`（排除 remastered） |
| 1 | 1080p | `1080p` |
| 2 | 1080i | `1080i` |
| 3 | 720p | `720p` |
| 4 | SD | `480p\|576p\|sd` |
| 5 | Other | - |

> **P 小写强制**：标题中 `1080P` → 错误，应为 `1080p`。`4K` 在标题中应改为 `2160p`。

#### 制作组 (group_constant)

| ID | 名称 | 匹配 |
|----|------|------|
| 6 | FROG | 标题含 `frog` |
| 7 | FROGE | 标题含 `froge` |
| 8 | FROGWeb | 标题含 `frogweb` |
| 9 | GodDramas | 标题含 `goddramas` |
| 5 | Other | 其他 |

> **官组检测**：标题含 `frog`/`froge`/`frogweb`/`Loong@QingWa` → `officialSeed=true`。

### 标题解析算法

#### 解析流程

```
1. 标题转小写 → title_lowercase
2. 排除年份/季集后的干扰
3. 位置约束校验：
   ├── 分辨率 须在 来源 前 → 否则报错
   ├── HDR 须在 视频编码 前 → 否则报错
   └── 视频编码 须在 音频 前 → 否则报错
4. 正则匹配链（按优先级）：
   ├── 分辨率 → resolution_constant
   ├── 来源类型 → type_constant
   ├── 视频编码 → encode_constant
   ├── 音频编码 → audio_constant（取最高规格）
   └── 制作组 → group_constant
```

#### HDR 检测与交叉验证

```
1. 标题含 hdr10+ → 标签须含 hdr10+
2. 标题含 hdr（非 hdr10+）→ 标签须含 hdr
3. 标签含 hdr10+ → 标题须含 hdr10+
4. 标签含 hdr → 标题须含 hdr
5. MediaInfo HDR Format 字段与标题/标签双向验证
```

### 校验规则 — 共 44+ 项

#### 分类级特殊规则

| 分类 | 规则 |
|------|------|
| 音乐(408) | 跳过大部分校验；主标题须含采样频率(`khz`)；主标题须含比特率(`bit`) |
| 其他(409) | 跳过所有校验规则 |
| 官方音乐种子 | 跳过所有检查（`officialSeed && cat===408`） |

#### 标题校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 标题含中文 | `[\u4e00-\u9fa5]` | 错误 |
| 2 | Complete 需删除（非蓝光原盘） | `complete` 在标题中 | 错误 |
| 3 | 季集应在年份前 | `S\d{2}` 位置检查 | 错误 |
| 4 | HDR10 应为 HDR | `hdr10`（非 HDR10+）→ 应写 HDR | 错误 |
| 5 | HDR10+ 标签与标题交叉验证 | 标签 vs 标题 | 错误 |
| 6 | HDR 标签与标题交叉验证 | 标签 vs 标题 | 错误 |
| 7 | WEB 资源 HEVC→H.265 | `hevc` in WEB 类型 | 错误 |
| 8 | 电影类别不应含 S**E** | `s\d{2}e\d{2}` + cat=401 | 错误 |
| 9 | 剧集类别必须含 S**E** | `!s\d{2}e\d{2}` + cat=402 | 错误 |
| 10 | WEB 资源 AVC→H.264 | `avc` in WEB 类型 | 错误 |
| 11 | HDTV 资源 HEVC→H265 | `hevc` in HDTV 类型 | 错误 |
| 12 | HDTV 资源 AVC→H264 | `avc` in HDTV 类型 | 错误 |
| 13 | 禁发小组（28+组） | 正则匹配黑名单 | 错误 |
| 14 | 分辨率 P→p | `\d+P` 大写检测 | 错误 |
| 15 | AC3→DD | `ac3` in 标题 | 错误 |
| 16 | 删除 HQ/FPS/EDR/SDR/10bit/4K(→2160p) | 关键词检测 | 错误 |
| 17 | 来源须在编码前 | 位置校验 | 错误 |
| 18 | 分辨率须在来源前 | 位置校验 | 错误 |
| 19 | 缺少分辨率/来源/编码/音频 | 完整性检查 | 错误 |
| 20 | 视频编码须在音频前 | 位置校验 | 错误 |
| 21 | 蓝光原盘标题格式 | `Blu-ray` vs `BluRay` | 错误 |
| 22 | Encode 标题格式 | `BluRay`（无连字符） | 错误 |
| 23 | Atmos 应在声道后 | `atmos` 位置校验 | 错误 |
| 24 | 声道数标示错误 | `7\.1`/`5\.1` 格式检查 | 错误 |

#### 字段选择校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 25 | 副标题为空 | `!subtitle` | 错误 |
| 26 | 未选分类 | `!cat` | 错误 |
| 27 | 未选媒介 | `!type` | 错误 |
| 28 | 未选编码 | `!encode` | 错误 |
| 29 | 未选音频 | `!audio` | 错误 |
| 30 | 未选分辨率 | `!resolution` | 错误 |
| 31 | 标题与选择字段不一致（媒介） | 标题解析 vs 用户选择 | 错误 |
| 32 | 标题与选择字段不一致（编码） | 标题解析 vs 用户选择 | 错误 |
| 33 | 标题与选择字段不一致（音频） | 标题解析 vs 用户选择 | 错误 |
| 34 | 标题与选择字段不一致（分辨率） | 标题解析 vs 用户选择 | 错误 |
| 35 | 官种标签/制作组双向验证 | 官组检测 vs 选择 | 错误 |
| 36 | 未选择制作组 | `!group` | 错误 |

#### 标签与 MI 交叉校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 37 | HLG 需添加 HDR 标签 | 标签检查 | 错误 |
| 38 | 国语/粤语/中字 标签与 MI 交叉验证 | MI 语言检测 vs 标签 | 错误 |
| 39 | HDR/HDR10+/杜比视界 标签与 MI 交叉验证 | MI HDR Format vs 标签 | 错误 |
| 40 | VCB-Studio 标签校验 | 制作组 vs 标签 | 错误 |
| 41 | Remux 标签校验 | 媒介 vs 标签 | 错误 |
| 42 | 完结/分集/合集标签与季集交叉验证 | 标题 S**E** vs 标签 | 错误 |

#### 简介与媒体信息校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 43 | 简介无 IMDb/豆瓣链接 | 链接检测 | 警告 |
| 44 | MediaInfo 含 BBCode | `[b]\|[color]` 等标签 | 错误 |
| 45 | 简介含 MediaInfo（应在独立栏） | `#kdescr` 内容检测 | 错误 |
| 46 | 原盘 MI 用 BDInfo/非原盘用 MediaInfo | 媒介类型 vs MI 格式 | 错误 |
| 47 | 蓝光原盘必须选 DIY 或原生原盘标签 | 媒介=原盘 + `!diy && !native` | 错误 |
| 48 | 缺少海报/截图 | `#kposter img` + `#ktorrentscreenshots img` | 错误 |
| 49 | MediaInfo 栏为空/不正确 | 空值或格式校验 | 错误 |
| 50 | 官组标题编码应为 x264/x265 | `officialSeed && !x264/x265` | 错误 |

#### 警告类规则

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| W1 | SD/Other 分辨率 | resolution=4 or 5 | 警告 |
| W2 | 简介无 IMDb/豆瓣链接 | 链接检测 | 警告 |
| W3 | 异常图片（高度≤24px） | `img.height() <= 24` | 警告 |

### 禁发制作组（28+组）

```
FGT, NSBC, BATWEB, GPTHD, DreamHD, BlackTV, CatWEB, Xiaomi, Huawei,
MOMOWEB, DDHDTV, SeeWeb, TagWeb, SonyHD, MiniHD, BitsTV, ALT,
LelveTV, NukeHD, ZeroTV, HotTV, EntTV, GameHD, SmY, SeeHD, ParkHD,
VeryPSP, DWR, XLMV, XJCTV, Mp4Ba, Huluwa, CTRLHD(非CtrlHD),
HotWEB, TBMaxUB, BestWEB, DBD-Raws, Skymoon/天月/HKACG, c.c动漫,
猎户发布组/orion origin, 爪爪字幕组/ZhuaZhuaStudio
```

### UI 功能

| 功能 | 说明 |
|------|------|
| 错误提示框 | 红色背景 `#EA2027`，显示所有错误 |
| 通过提示框 | 绿色背景 `#8BC34A`，显示"此种子未检测到错误" |
| 警告提示框 | 黄色，显示警告信息 |
| 图片加载等待 | 30 秒超时，加载完成后检查异常图片 |
| 快捷键 F4 | 一键通过/驳回（根据是否有错误） |
| 快捷键 F3 | 关闭窗口 |
| 审核页自动操作 | 自动点击通过/驳回按钮，自动填写错误信息 |
| 错误信息翻译 | 自动将技术错误转为用户友好提示 |

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题全部英文（移除中文、全角字符）
2. 按青蛙命名规范重构标题：
   剧名 年份 其他信息 分辨率 来源类型 规格 HDR bit 视频编码 音频 声道数 Atmos 音轨数-制作组
3. 原盘用 Blu-ray（连字符），Encode 用 BluRay（无连字符）
4. 原盘编码用 AVC/HEVC，WEB 用 H.264/H.265，Encode 用 x264/x265
5. P 小写（1080p 非 1080P），4K 改为 2160p
6. AC3→DD，E-AC3→DDP
7. 移除源站前缀标签（如 [馒头]、[HDArea] 等）
8. Complete 需删除（非蓝光原盘）
9. x264 10bit 须写为 Hi10P x264
```

### 副标题自动处理

```
1. 禁止为空（必填）
2. 建议格式：中文名 | 外文名 | 包含内容等
3. 优先从 PT-Gen/豆瓣获取中文名
```

### 质量字段自动选择

```
从源站标题解析：
1. 媒介(type)：
   UHD Blu-ray → 1, Blu-ray → 8, Remux → 9, Encode → 10,
   WEB-DL → 7, HDTV → 4, MiniBD → 11, DVD → 2, CD → 3, Track → 5, Other → 6
2. 编码(encode)：
   H.264/AVC → 1, H.265/HEVC → 6, VC-1 → 2, MPEG-2 → 4,
   AV1 → 7, MPEG-4 → 3, VP9 → 8, Other → 5
3. 音频(audio)：按匹配优先级
   DTS:X → 6, TrueHD Atmos → 14, DDP → 11, TrueHD → 12,
   DTS-HD MA → 16, DTS-HD HR → 17, DTS → 2, DD → 1,
   FLAC → 4, AAC → 3, LPCM → 5, ALAC → 7, WAV → 8,
   OPUS → 10, AV3A → 15, USAC → 18, Other → 22
4. 分辨率(resolution)：
   8K → 6, 4K/2160p → 7, 1080p → 1, 1080i → 2,
   720p → 3, SD → 4, Other → 5
5. 制作组(group)：
   FROG → 6, FROGE → 7, FROGWeb → 8, GodDramas → 9, Other → 5

注意 remastered 排除在 4K 检测之外
```

### 标签自动选择

```
1. HDR：MI 含 HDR Format（非 DV）→ 勾选 HDR
2. HDR10+：MI 含 HDR10+ → 勾选 HDR10+（必须同时选 HDR）
3. 杜比视界：MI 含 Dolby Vision → 勾选杜比视界
4. 国语：MI 音频语言含 Chinese → 勾选国语
5. 粤语：MI 音频语言含 Chinese/Yue/Cantonese → 勾选粤语
6. 中字：MI 字幕语言含 Chinese → 勾选中字
7. VCB-Studio：制作组含 VCB-Studio → 勾选
8. DIY：标题/副标题含 DIY → 勾选
9. 原生原盘：媒介为原盘且未修改 → 勾选
10. Remux：媒介为 Remux → 勾选
11. 完结：标题含 S01-S** 或季集完整 → 勾选
12. 分集：标题仅含部分集数 → 勾选（需申请）
13. 系列合集：多季/多部合集 → 勾选
```

### MediaInfo 处理

```
1. 非蓝光原盘用 MediaInfo，蓝光原盘用 BDInfo
2. MediaInfo 须英文（禁止中文 MI）
3. MediaInfo 禁止包含 BBCode 标签
4. MediaInfo 应在独立栏位，禁止放入简介
5. 简介中禁止包含 MediaInfo 内容
```

### 简介自动构建

```
1. IMDb/豆瓣链接（至少一个，否则警告）
2. 海报图片
3. PT-Gen 生成的简介内容
4. 至少 3 张视频截图
5. 蓝光原盘使用 BDInfo quick summary
6. 原出处简介用 quote 标签包裹：
   [quote]资源简介[/quote]
```

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-19*
*数据来源：upload.php + Wiki发布规则 + qingwa-torrent-assistant.js v1.1.1 (1886行/82KB)*
