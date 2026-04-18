# 皇后 站点适配器设计

> OpenCD 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 皇后|
| 站点地址 | https://open.cd |
| 站点框架 | NexusPHP（定制音乐版） |
| 内容定位 | **纯音乐站点**，禁止有损音频、APE、CD镜像、非官方合集 |
| 特殊功能 | 候选制发布、Log Checker、频谱图验证、转载规则标记、候选人投票 |
| 发布页面 | `plugin_upload.php`（非标准 `upload.php`） |
| 提交地址 | `plugin_upload_save.php`（POST multipart/form-data） |
| Tracker | `https://tracker.open.cd/announce.php` |

---

## 一、发种规范（来自论坛 topicid=7984）

### 1.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 发布者必须在做种累计时间达到 **24 小时**且有至少 **3 个做种者**后方可撤种，若 **72 小时**后无人下载可撤种
- 发布者将获得 **双倍上传量**
- 违规资源不经提醒直接删除

### 1.2 允许的资源

- 无损音乐：只允许 **FLAC 分轨**、**WAV 整轨**
- 镜像类：只允许 SACD、DVDV、DVDA、Blu-ray
- 高清 MV（HD）视频
- 标清 MV（SD）视频

### 1.3 不允许的资源

- 总体积小于 **50MB**（单曲专辑除外）
- 有损音频（MP3 等需移步外站）
- **APE** 音频文件（明确禁止）
- CD 镜像
- 非官方合集
- 无正确 cue 表单的整轨音频文件
- 压缩文件（RAR/ZIP 等）
- DVDrip upscale（分辨率大于 480p 的 DVDrip）
- 对各种 rip 版本的再次压制版本
- 损坏的文件、垃圾文件、涉及禁忌内容

### 1.4 重复判定规则（Dupe）

**总体规则**：
1. 相同或相似资源，后发布构成重复被删除
2. 已存在资源除非有明显缺陷，否则不因重复被删除
3. 后发布资源如果级别高于已存在资源，两者可同时存在

**音频类具体规则**：
- 单专辑与合集之间不构成 dupe
- 同一专辑不同版本不构成 dupe（欧版与大陆版之间不构成 dupe）
- OpenCD 制作组对外不受 dupe 规则约束，但组内成员作品之间依然受约束

**例外**：
- 先上传资源有明显质量缺陷 → 允许上传其他版本
- 原有资源连续断种 **30 日以上** → 发布新资源不受 dupe 约束

### 1.5 标题规则

#### 原抓组成员发布

中文专辑：
```
艺术家 - 官方专辑名称 年份 - 格式整分轨 - 发布人@制作组
```
范例：`刘欢 - 经典20年珍藏锦集 2004 - FLAC 分轨 - luoluo@OpenCD`

其他语言专辑：
```
艺术家 - 官方专辑名称 年份 - 格式整分轨 - 发布人@制作组
```
范例：`Jerry Goldsmith - Total Recall [The Deluxe Edition] 2000 - FLAC 分轨 - barca008@OpenCD`

#### 会员自抓专辑发布

中文专辑主标题：`艺术家 - 官方专辑名`

**标题自动生成规则**（由 JS 函数 `titlepreview()` 实现）：
```
艺术家 - 专辑名称 年份 - 格式 [整轨|分轨] - 原创者 - 来源
```
字段通过 `split` 属性拼接：artist（` - `）、resource_name（` - `）、year（` `）、standard、audio_mode、pgname（` - `）、frname（` - `）

### 1.6 新手考核

新人注册起第一个月必须满足（否则 ban）：
- 上传 > 10G
- 下载 > 10G
- 魔力值 > 1000
- 分享率 > 1
- 做种率 > 6.0

---

## 二、发布页面表单字段分析

### 2.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `type` | select | ✓ | 类型/分类 |
| `artist` | text | ✓（音乐类） | 艺术家名 |
| `resource_name` | text | ✓（音乐类） | 专辑名称 |
| `year` | text | ✓（音乐类） | 发行年份（maxlength=4） |
| `name` | text | ✓ | 标题（音乐类只读，自动生成） |
| `name_search` | hidden | - | 搜索用标题（含繁简翻译） |
| `cover` | text + iframe | ✓ | 封面图片（通过 attachment.php 上传或填 URL） |
| `small_descr` | text | - | 副标题 |
| `descr` | textarea | ✓ | 简介（BBCode，通过 AJAX 加载编辑器） |
| `track_list` | textarea | ✓ | 曲目列表 |
| `uplver` | checkbox | - | 匿名发布（value="yes"，默认选中） |
| `albumId` | hidden | - | 专辑 ID（用于关联 Wiki 专辑） |
| `tags` | hidden | - | 标签 ID 列表（逗号分隔） |

### 2.2 类型（`type`）— 4 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 408 | 音乐(Music) | 默认模式，启用艺术家/专辑名/年份字段，标题自动生成 |
| 402 | 演唱会(Vocal Concert) | 标题手动输入 |
| 405 | 戏剧(Drama) | 标题手动输入 |
| 409 | 其他(Other) | 标题手动输入 |

**类型切换行为**（`typechange()` JS 函数）：
- 选择 `408`（音乐）：显示 `.type_music` 行，标题字段只读
- 选择其他：隐藏 `.type_music` 行，标题字段可编辑

### 2.3 类别/音乐流派（`source`）— 17 个

| 值 | 显示名称 |
|----|----------|
| 2 | 流行(Pop) |
| 3 | 古典(Classical) |
| 11 | 器乐(Instrumental) |
| 4 | 原声(OST) |
| 5 | 摇滚(Rock) |
| 8 | 爵士(Jazz) |
| 12 | 新世纪(NewAge) |
| 13 | 舞曲(Dance) |
| 14 | 电子(Electronic) |
| 15 | 民谣(Folk) |
| 16 | 独立(Indie) |
| 17 | 嘻哈(Hip Hop) |
| 18 | 音乐剧(Musical) |
| 19 | 乡村(Country) |
| 20 | 另类(Alternative) |
| 21 | 世界音乐(World) |
| 9 | 其他(Others) |

### 2.4 格式（`standard`）— 8 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | 镜像(Mirror) | 镜像文件 |
| 2 | WAV | 选择 WAV 时自动设置 audio_mode=整轨 |
| 4 | FLAC | 选择 FLAC 时自动设置 audio_mode=分轨 |
| 15 | DTS | DTS 格式 |
| 17 | DFF | DSD 交错格式 |
| 18 | DSF | DSD 索引格式 |
| 19 | DST | DST 压缩格式 |
| 10 | 其它(Other) | 其他格式 |

**注意**：值编号不连续（缺少 3/5-9/11-14/16），可能是历史原因。

### 2.5 媒介（`medium`）— 22 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | CD | 标准 CD |
| 2 | 24KCD | 24K 金 CD |
| 3 | DSD | DSD 媒介 |
| 4 | LPCD | LPCD |
| 5 | HDCD | HDCD |
| 6 | SACD | Super Audio CD |
| 7 | SRCD | SRCD |
| 8 | K2CD | K2CD |
| 9 | DTS | DTS 光盘 |
| 10 | DAT | 数字录音带 |
| 11 | Blu-ray | 蓝光 |
| 12 | HD DVD | HD DVD |
| 13 | HDTV | 高清电视录制 |
| 14 | DVD | DVD |
| 16 | HQCD | HQCD |
| 17 | XRCD | XRCD |
| 18 | SHM-CD | SHM-CD |
| 19 | Blu-spec | Blu-spec CD |
| 20 | Vinyl | 黑胶唱片 |
| 21 | Web | 网络下载 |
| 22 | HI-RES | 高解析度音频 |
| 15 | Other | 其他 |

### 2.6 整/分轨（`audio_mode`）— 3 个

| 值 | 显示名称 |
|----|----------|
| single | 整轨 |
| multi | 分轨 |
| none | 无 |

### 2.7 制作组（`team`）— 5 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 7 | OpenCD | 站方制作组 |
| 8 | LLM | 制作组 |
| 9 | TSxD | 制作组 |
| 6 | KHQ | 制作组 |
| 5 | 其他(Other) | 其他制作组 |

### 2.8 来源（`boardid`）— 2 个（单选 radio）

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 2 | 原抓 | 选择后隐藏来源名称输入框 |
| 1 | 转载（默认选中） | 显示来源名称输入框 `frname` |

配套字段：
- `frname`：来源名称（转载时填写来源站点/发布者）

### 2.9 原创者（`pgname`）— 文本输入

### 2.10 转载规则（`share_rule`）— 4 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | 禁止转载 | |
| 2 | 未经允许不得转载 | |
| 3 | 欢迎转载 | |
| 255 | 其他情况 | 参考副标题及种子简介的标注语 |

### 2.11 其他字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `has_artwork[]` | checkbox | 完整扫图（value=1） |
| `nfo1`~`nfoN` | file | Log 文件（可动态添加多个，通过 `nfocnt` 计数） |
| `nfocnt` | hidden | Log 文件数量（默认 1） |
| `audition` | text + iframe | 试听音频（通过 attachment.php 上传） |
| `spectrogram` | text + iframe | 频谱图（通过 attachment.php 上传） |
| `status` | checkbox | 候选状态（value="pending"，默认选中且 disabled） |

### 2.12 频谱图提交校验逻辑

```
如果没有上传 Log 文件：
  如果格式不是 镜像(1)/DFF(17)/DSF(18)：
    如果没有上传频谱图 → 拒绝提交，提示"没有Log时必须上传频谱图"
```

### 2.13 重复检测

提交时通过 AJAX 调用 `plugin_upload.php?cmd=checkTorrent&search=<资源名>` 检查是否已存在相似资源。如检测到重复，弹出确认对话框。

---

## 三、分类标签（AJAX 动态加载）

标签通过 `plugin_upload.php?cmd=gettags` 获取 JSON 数据，分为两组：

### 3.1 地区(Area)标签 — 5 个

| ID | 名称 |
|----|------|
| 1 | 大陆 |
| 2 | 欧美 |
| 3 | 港台 |
| 4 | 日韩 |
| 5 | 其它地区 |

### 3.2 风格(Style)标签 — 22 个

| ID | 名称 |
|----|------|
| 6 | 流行(Pop) |
| 7 | 发烧(HiFi) |
| 8 | 汽车(garage) |
| 9 | 古典(Classical) |
| 10 | 民族(National) |
| 11 | 摇滚(rock) |
| 12 | 原声(OST) |
| 13 | 民间(Folk) |
| 14 | 乡村(Country) |
| 15 | 灵魂(Soul) |
| 16 | 新世纪(NewAge) |
| 17 | 蓝调(Blues) |
| 18 | 爵士(Jazz) |
| 19 | 金属(Metal) |
| 20 | 朋克(Punk) |
| 21 | 电子(Electronic) |
| 22 | 儿童(Children's) |
| 23 | 宗教(Religion) |
| 24 | 雷鬼(Reggae) |
| 25 | 贝斯(Drum&Bass) |
| 26 | 说唱(Rap) |
| 27 | 音乐剧(musical) |

**标签选择方式**：点击标签高亮添加，已选标签显示在"已选择的标签列表"区域，可点击移除。选中标签的 ID 以逗号分隔存入 `tags` 隐藏字段。

---

## 四、与 DIC Music 的对比

| 维度 | OpenCD | DIC Music |
|------|--------|-----------|
| 框架 | NexusPHP（定制） | Gazelle |
| 提交地址 | `plugin_upload_save.php` | 标准路由 |
| 标题 | 音乐类自动生成 | 手动输入 |
| 艺术家字段 | 单一 `artist` | `artists[]` + `importance[]`（支持多人） |
| 分类系统 | `type`(4) + `source`(17) | `releasetype`(15) |
| 格式 | `standard`(8) | 无（Gazelle 用 Torrent Group） |
| 媒介 | `medium`(22) | `media`(10，字符串值) |
| 制作组 | `team`(5) | 无独立字段（通过 release_desc 标注） |
| 整/分轨 | `audio_mode`(3) | 无（通过文件内容判断） |
| 来源 | `boardid` radio | `scene`/`diy` checkbox |
| 转载规则 | `share_rule`(4) | 无独立字段 |
| 标签 | AJAX 加载，27 个（地区5+风格22） | 自动完成，用户自定义 |
| Log 文件 | 支持多文件 | 支持多文件 + Log Score |
| 频谱图 | 必填（无 Log 时） | 无 |
| 封面 | 必填（上传或 URL） | 必填（URL） |
| 曲目列表 | 必填 | 无独立字段（在描述中） |
| 禁止格式 | APE、有损、CD 镜像 | MQA |
| 候选机制 | 默认候选（`status=pending`） | 无 |

---

## 五、关键适配器设计要点

### 5.1 标题自动生成

音乐类（type=408）的标题由前端 JS 自动拼接，字段顺序：
```
{artist} - {resource_name} {year} - {standard名称} {audio_mode名称} - {pgname} - {frname}
```

适配器需要 **在后端重现此逻辑**，生成正确格式的标题。

### 5.2 候选制发布

所有种子默认以候选状态发布（`status=pending`，disabled 且 checked），需要通过投票审核后才能正式出现在种子列表。**Elite User 及以上**等级可直接发布无需候选。

### 5.3 频谱图/Log 校验

提交前校验逻辑：
- 有 Log → 直接通过
- 无 Log 且格式为镜像(1)/DFF(17)/DSF(18) → 通过
- 无 Log 且其他格式 → **必须上传频谱图**

适配器需根据格式类型判断是否需要频谱图。

### 5.4 特殊字段名差异

| 通用 NexusPHP 字段 | OpenCD 字段 | 说明 |
|--------------------|-------------|------|
| `torrent`/`torrentfile` | `file` | 种子文件字段名不同 |
| `upload.php` | `plugin_upload.php` | 发布页面路径不同 |
| `takeupload.php` | `plugin_upload_save.php` | 提交地址不同 |
| 无 | `source` | 音乐流派（其他站用作 source_sel） |
| 无 | `standard` | 音频格式 |
| 无 | `medium` | 物理媒介 |
| 无 | `audio_mode` | 整轨/分轨 |
| 无 | `artist` | 艺术家 |
| 无 | `resource_name` | 专辑名 |
| 无 | `track_list` | 曲目列表 |
| 无 | `audition` | 试听音频 |
| 无 | `spectrogram` | 频谱图 |
| 无 | `share_rule` | 转载规则 |
| 无 | `boardid`/`frname`/`pgname` | 来源/原创者信息 |
| 无 | `cover` | 封面（独立字段，非描述内） |

### 5.5 重复检测 API

```
GET plugin_upload.php?cmd=checkTorrent&search=<资源名>
POST rows=100
```

返回 JSON：`{"total": N, "rows": [{"id": "...", "name": "..."}]}`

适配器可在发布前调用此 API 检查重复。

### 5.6 标签 API

```
GET plugin_upload.php?cmd=gettags
```

返回 JSON：`{"items": [{"id": "1", "name": "大陆", "parentId": "1", "parentName": "地区(Area)"}, ...]}`

### 5.7 用户等级与发布权限

- **Elite User 及以上**（正六品）：无需候选，可直接发布
- **Power User 及以下**：需先提交候选，通过投票后正式发布
- **发布员(Uploader)**：免除自动降级，可查看匿名用户

---

## 六、发布字段与通用模型的映射

### 6.1 类型映射（type）

| 通用类型 | OpenCD type 值 |
|----------|----------------|
| 音乐 | 408 |
| 演唱会 | 402 |
| 戏剧 | 405 |
| 其他 | 409 |

### 6.2 格式映射（standard）

| 通用格式 | OpenCD standard 值 |
|----------|-------------------|
| FLAC | 4 |
| WAV | 2 |
| DTS | 15 |
| DFF | 17 |
| DSF | 18 |
| DST | 19 |
| 镜像/ISO | 1 |
| 其他 | 10 |

### 6.3 媒介映射（medium）

| 通用媒介 | OpenCD medium 值 |
|----------|-----------------|
| CD | 1 |
| SACD | 6 |
| Blu-ray | 11 |
| DVD | 14 |
| Vinyl | 20 |
| Web | 21 |
| HI-RES | 22 |
| HDCD | 5 |
| HQCD | 16 |
| XRCD | 17 |
| SHM-CD | 18 |
| Blu-spec | 19 |
| DAT | 10 |
| 24KCD | 2 |
| LPCD | 4 |
| K2CD | 8 |
| SRCD | 7 |
| DSD | 3 |
| HD DVD | 12 |
| HDTV | 13 |
| DTS | 9 |
| Other | 15 |

### 6.4 音乐流派映射（source）

| 通用流派 | OpenCD source 值 |
|----------|-----------------|
| 流行 | 2 |
| 古典 | 3 |
| 原声 | 4 |
| 摇滚 | 5 |
| 爵士 | 8 |
| 其他 | 9 |
| 器乐 | 11 |
| 新世纪 | 12 |
| 舞曲 | 13 |
| 电子 | 14 |
| 民谣 | 15 |
| 独立 | 16 |
| 嘻哈 | 17 |
| 音乐剧 | 18 |
| 乡村 | 19 |
| 另类 | 20 |
| 世界 | 21 |

### 6.5 制作组映射（team）

| 制作组 | OpenCD team 值 |
|--------|----------------|
| OpenCD | 7 |
| LLM | 8 |
| TSxD | 9 |
| KHQ | 6 |
| 其他 | 5 |

---

## 七、特殊注意事项

### 7.1 发布页面路径

OpenCD 使用 **定制插件发布页** `plugin_upload.php`，而非标准 NexusPHP 的 `upload.php`。提交地址为 `plugin_upload_save.php`，也非标准的 `takeupload.php`。适配器必须使用正确的路径。

### 7.2 封面上传

封面通过 `attachment.php?target=cover` iframe 上传，支持 URL 输入和文件上传。适配器需要处理封面 URL 或通过附件 API 上传。

### 7.3 试听音频和频谱图

均有对应的 `attachment.php?target=audition` 和 `attachment.php?target=spectrogram` iframe 上传接口。

### 7.4 无 IMDb / PT-Gen 字段

纯音乐站无视频相关字段（无 IMDb 链接、无 PT-Gen、无 MediaInfo）。

### 7.5 无视频编码/分辨率/音频编码字段

所有质量相关字段完全不同于视频站：
- 无 `codec_sel`（视频编码）
- 无 `standard_sel`（分辨率）
- 无 `audiocodec_sel`（音频编码）
- 使用 `standard`（音频格式）+ `medium`（物理媒介）+ `audio_mode`（整/分轨）

### 7.6 转载规则必填

`share_rule` 为必填字段，适配器需为转发资源选择合适的转载规则值。建议：
- 源站禁止转载 → 值=1
- 源站未明确 → 值=2（未经允许不得转载）
- 源站欢迎转载 → 值=3
- 不确定 → 值=255

### 7.7 候选状态

新发布的种子默认为候选状态（`status=pending`），需管理组/投票审核。如果发布者等级足够（Elite User+），可直接发布。

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
