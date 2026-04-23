# 好大 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 好大|
| 站点地址 | https://hdarea.club |
| 站点框架 | NexusPHP（基于 NexusPHP 1.5 修改） |
| 特殊功能 | Cloudflare 防护、29 种音频编码（最全之一）、盒子规则、候选制、字幕区 |
| Tracker URL | https://tracker.hdarea.club/announce.php |
| 规则页面 | rules.php |
| 公告页面 | forums.php?topicid=6122（新手发种教程）、topicid=6126（盒子规则）、topicid=6123（完结剧规则） |
| 发布页面 | upload.php → takeupload.php |

**站点角色**: 目标站（发布站）。HDApt Auto Transfer 项目已实现 M-Team/TTG → HDArea 的完整自动转发。

---

## 一、发布页面表单字段（upload.php）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `file` | file | 种子文件 |
| `name` | text | 主标题（英文） |
| `small_descr` | text | 副标题（中文） |
| `url` | text | IMDb 链接 |
| `dburl` | text | 豆瓣链接（ID 数字） |
| `descr` | textarea | 简介（BBCode） |
| `uplver` | checkbox | 匿名发布（value=yes） |

**无** PT-Gen 字段、**无** MediaInfo 独立字段、**无** NFO 字段、**无** 标签字段、**无** source_sel 字段。

### 1.2 分类（`type`）— 18个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 300 | Movie UHD-4K | UHD 4K 电影 |
| 401 | Movies Blu-ray | 蓝光原盘电影 |
| 415 | Movies REMUX | Remux 电影 |
| 416 | Movies 3D | 3D 电影 |
| 410 | Movies 1080p | 1080p 电影 |
| 411 | Movies 720p | 720p 电影 |
| 414 | Movies DVD | DVD 电影 |
| 412 | Movies WEB-DL | WEB-DL 电影 |
| 413 | Movies HDTV | HDTV 电影 |
| 417 | Movies iPad | iPad 电影 |
| 404 | Documentaries | 纪录片 |
| 405 | Animations | 动漫 |
| 402 | TV Series | 剧集 |
| 403 | TV Shows | 综艺 |
| 406 | Music Videos | MV/演唱会 |
| 407 | Sports | 体育 |
| 409 | Misc | 其他 |
| 408 | HQ Audio | 高品质音频 |

**注意**: 分类值编号跨度大（300, 401-417），非连续编号。

### 1.3 媒介（`medium_sel`）— 9个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 3 | REMUX |
| 7 | Encode |
| 9 | WEB-DL |
| 4 | MiniBD |
| 5 | HDTV |
| 2 | HD DVD |
| 6 | DVDR |
| 8 | CD |

### 1.4 视频编码（`codec_sel`）— 10个

| 值 | 显示名称 |
|----|----------|
| 7 | H.264(x264/AVC) |
| 1 | MPEG-4 |
| 6 | H.265(x265/HEVC) |
| 4 | MPEG-2 |
| 3 | Xvid |
| 2 | VC-1 |
| 5 | Other |
| 8 | AV1 |
| 9 | VP8/9 |
| 10 | AVS |

### 1.5 音频编码（`audiocodec_sel`）— 29个

| 值 | 显示名称 | 类型 |
|----|----------|------|
| 6 | AAC | 有损 |
| 5 | DD5.1/AC-3 | 有损 |
| 3 | DTS | 有损/无损 |
| 4 | DTS-HD MA/DTS XLL | 无损 |
| 12 | DTS:X | 对象音频 |
| 13 | DTS-HD HR/HRA | 有损 |
| 7 | TrueHD | 无损 |
| 10 | TrueHD Atmos | 对象音频 |
| 15 | DDP Atmos | 对象音频 |
| 16 | DDP/E-AC-3 | 有损 |
| 11 | DD2.0/AC-3 | 有损 |
| 8 | LPCM | 无损 |
| 9 | WAV | 无损 |
| 1 | FLAC | 无损音频 |
| 2 | APE | 无损音频 |
| 14 | DSD | 无损音频 |
| 21 | MP3 | 有损 |
| 25 | Opus | 有损 |
| 18 | Vorbis | 有损 |
| 22 | ALAC | 无损 |
| 26 | WMA | 有损 |
| 27 | AC-4 | 新格式 |
| 28 | MPEG-H | 新格式 |
| 29 | MQA | 无损编码 |
| 19 | TTA | 无损 |
| 20 | AV3A | 中国标准 |
| 17 | MPEG | 有损 |
| 24 | Other | 兜底 |

**注意**: 已采集站点中音频编码最多的站点之一（29个），涵盖 DTS:X、DSD、AC-4、MPEG-H、MQA、AV3A 等罕见编码。值编号有跳跃（无 23）。

### 1.6 分辨率（`standard_sel`）— 5个

| 值 | 显示名称 |
|----|----------|
| 3 | 720p |
| 1 | 1080p |
| 4 | SD |
| 2 | 1080i |
| 5 | 4K |

### 1.7 制作组（`team_sel`）— 13个

| 值 | 显示名称 |
|----|----------|
| 1 | EPiC |
| 2 | HDArea |
| 3 | HDWING |
| 4 | WiKi |
| 5 | TTG |
| 6 | other |
| 7 | MTeam |
| 8 | HDApad |
| 9 | CHD |
| 10 | HDAccess |
| 11 | HDATV |
| 12 | cXcY |
| 13 | CMCT |

---

## 二、完整站点规则（rules.php）

> 数据来源：rules.php（45088 字节）+ 论坛公告 #6126 + #6123

### 2.1 总则

- 英简繁三版规则有冲突的以**简体中文版**为准
- 不允许发送垃圾信息
- 一切作弊帐号会被封
- 注册多账号将被禁止
- 不要把本站种子上传到其他 Tracker
- 第一次捣乱警告，第二次永久封禁

### 2.2 账号保留规则

| 等级/状态 | 保留条件 |
|-----------|----------|
| Veteran User 及以上 | 永远保留 |
| Insane User 及以上（封存） | 封存后不会被删除 |
| 封存账号 | 连续 150 天不登录删除 |
| 未封存账号 | 连续 60 天不登录删除 |
| 新注册 | 7 天无流量（上传/下载都为 0）删除 |

### 2.3 下载规则与促销

**种子促销类型**：

| 类型 | 说明 |
|------|------|
| 50% | 按 50% 计下载量 |
| 30% | 按 30% 计下载量 |
| 免费/FREE | 不计下载量 |
| 2x | 按 2 倍计上传量 |
| 2X 50% | 50% 下载 + 2x 上传 |
| 2X FREE | 不计下载 + 2x 上传 |

**随机促销概率**（种子上传后系统自动）：

| 概率 | 促销类型 |
|------|----------|
| 10% | 50% 下载 |
| 5% | 免费/FREE |
| 5% | 2x 上传 |
| 3% | 50% 下载 & 2x 上传 |
| 1% | 免费 & 2x 上传 |

**官种限时 FREE 后阶梯**：

| 官种类型 | 限时 FREE 结束后 |
|----------|-----------------|
| 官方 Blu-ray 原盘 | → 30% |
| 官方 1080P | → 30% |
| 官方 720P | → 50% |

**其他促销规则**：
- 电视剧等每季第一集 → 免费/FREE
- 关注度高的种子可由管理员设为促销
- 不定期全站免费活动
- "限时免费/FREE"的资源只有在限时时段内下载完成才不计下载流量

### 2.4 上传总则

- 上传者必须对上传文件拥有合法传播权
- 做种时间不足 24h 或故意低速上传将被警告甚至取消上传权限
- **发布者获得双倍上传量**
- 请遵守各类资源发种细则，认真完整填写发种界面各项信息

### 2.5 上传者资格

- 任何人都能发布资源
- 部分用户需先在候选区提交候选（详见 FAQ 用户等级说明）

**候选区规则**：
- 提交前确定无重复、符合上传规则、能成功发布并做种到出种
- 简介须详细介绍资源
- 禁止无差别无理由投反对票

### 2.6 允许的资源

- 高清视频：蓝光原碟/HD DVD 原碟/remux/HDTV/重编码(≥720p)/高清 DV
- 标清视频：高清媒介重编码(≥480p)/DVDR/DVDISO/DVDRip/CNDVDRip
- 无损音轨及 cue 表单（FLAC、APE 等）
- 5.1 声道或以上的电影/音乐音轨（DTS、DTS CD 镜像等）、评论音轨
- PC 游戏（必须原版光盘镜像）
- 7 日内高清预告片
- 高清相关软件和文档

### 2.7 不允许的资源

- 做种文件夹含 `*.torrent`、`*.url` 等无关文件
- rmvb/rm 等无来源无编码资源，CAM/TC/TS 枪版
- 硬水印视频
- xv/bhd/qsv 等专用播放器格式
- 不规范转发（修改文件名/结构/增删文件）
- 总体积 <100MB（学习类/文档类/音乐类除外）
- 标清 upscale 视频
- RealVideo 编码/flv 文件
- 单独样片（须和正片一起上传）
- 未达 5.1 声道的有损音频（MP3/WMA 等）
- 无正确 cue 的多轨音频
- 硬盘版/高压版游戏、非官方镜像、第三方 mod、小游戏合集、单独破解/补丁
- RAR 等压缩文件（须用文件夹直接发布）
- Dupe 资源
- 色情/敏感政治内容
- 损坏文件、垃圾文件

### 2.8 Dupe 规则

- **本站官方制作组已发布作品时，后发布同一资源视为 dupe 会被删除**
- 在本站官方小组作品未发布前，同一作品只保留 1 个版本

### 2.9 打包规则

允许打包：
- 套装售卖的高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7 日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，高清须同分辨率）
- 同一艺术家音乐（5 张以上专辑方可打包，两年内专辑可单独发布）
- 分卷发售的动漫剧集/角色歌/广播剧
- 发布组打包发布的资源

打包要求：
- 视频须相同媒介、相同分辨率、相同编码（预告片例外）
- 电影合集须同一发布组
- 音频须相同编码格式（如全部 FLAC）
- 打包发布后视情况删除单独种子

### 2.10 种子标题命名规范

**电影**：`英文名称 年份 [剪辑版本] [发布说明] 分辨率 来源 音频编码 视频编码-发布组`

示例：`Zootopia 2016 1080p BluRay DTS-HD MA 7.1 2Audio x264-EPiC`

**电视剧**：`英文名称 [年份] S**E** [发布说明] 分辨率 来源 [音频编码] 视频编码-发布组`

示例：`Game of Thrones S06E07 Hybrid 720p HDTV x264-DON`

**音轨**：`艺术家名.专辑名.[年份].[版本].[发布说明].音频编码[-发布组名称]`

示例：`Arcade Fire - Reflektor 2013 FLAC`

**副标题**：开头必须是资源中文名，其后填写最有价值的信息。

### 2.11 简介要求

- 电影/电视剧/动漫：须含海报、截图、文件详情（格式/时长/编码/码率/分辨率/语言/字幕）、演职员/剧情概要
- 体育：勿泄露比赛结果
- 音乐：须含专辑封面和曲目列表
- PC 游戏：须含海报/封面、截图

### 2.12 质量选项规范

- 蓝光原盘 → "Blu-Ray"
- DVD 原盘 → "DVD"
- REMUX → "REMUX"
- HDTV → "HDTV"
- 重编码 → "Encode"
- HEVC 即 H.265
- 分辨率低于 720p 选 "SD"，4K(3840×2160) 选 "4K"

---

## 二B、盒子（Seedbox）规则（论坛 #6126）

> 数据来源：forums.php?topicid=6126（30729 字节，中英双语公告）

**注意：盒子规则仅适用于普通会员，VIP 及以上级别不适用。**

### 2B.1 取消下载优惠

- 所有被系统标记为盒子的 IP，下载量严格按 **100%** 计算
- 不再享受任何下载促销/优惠（Free、50% 等）

### 2B.2 72 小时上传限额

- 种子发布后最初 72h 内，单个种子最多只计算**总体积 3 倍**的上传量
- 72h 后恢复正常计算

### 2B.3 发布人豁免

- 种子的原始发布人（Uploader）**不受 72h/3x 上传限额限制**

### 2B.4 盒子识别与申诉

- 系统自动识别盒子 IP，无需手动提交
- 盒子 IP 范围动态调整
- 家庭 IP 被误识别可申诉：https://hdarea.club/forums.php?action=viewtopic&forumid=1&topicid=6125

---

## 二C、完结剧规则（论坛 #6123）

> 数据来源：forums.php?topicid=6123（27574 字节）

- **已完结的电视剧将删除全部分集资源，只保留合集资源**
- **已完结的电视剧禁止再上传分集资源**

---

## 二D、字幕区规则

- 允许格式：srt/ssa/ass/cue/zip/rar
- Vobsub(idx+sub)或合集须打包为 zip/rar
- 不允许 lrc 歌词或非字幕/cue 文件
- 不合格字幕（不匹配/不同步/打包错误/编码错误/语种标识错误/命名不明确/重复）直接删除
- 举报不合格字幕奖励 50 魔力值
- 上传不合格字幕扣 100 魔力值

---

## 二E、简介（BBCode）结构参考

```
[quote][b][color=Blue]转自xxx，感谢原作者发布[/color][/b][/quote]  ← 转载信息
[img]海报URL[/img]                                                    ← 海报
PT-Gen 生成的豆瓣/IMDb 简介                                            ← 简介
[img]截图1[/img][img]截图2[/img][img]截图3[/img]                      ← 截图（≥3张）
[quote]MediaInfo文本[/quote]                                           ← Info信息
```

---

## 二F、特殊注意事项汇总

- **Cloudflare 防护**: 站点使用 Cloudflare，需先 GET `upload.php` 预热 session
- **种子文件名**: 必须为 ASCII（非 ASCII 文件名会出错）
- **4 字节 emoji 禁止**: NexusPHP MySQL utf8（非 utf8mb4），4 字节字符会导致截断
- **种子格式**: qBittorrent libtorrent ≥2.0 需使用 V1 格式
- **查重**: 发布前必须查重
- **豆瓣链接必填**: `dburl` 字段为必填项
- **Tracker URL**: `https://tracker.hdarea.club/announce.php`
- **发布者双倍上传**: 发布者获得该种子双倍上传量
- **盒子规则**: 普通会员盒子下载 100% 计量，72h 内上传限 3 倍
- **完结剧**: 已完结剧只保留合集，禁止分集

---

## 三、HDApt 字段映射参考

HDApt Auto Transfer（`examples/hdapt_auto_transfer/`）已实现完整的源站→HDArea 字段映射，映射架构为：

```
源站原始数据 → 中间属性名（字符串） → config.yaml 映射表 → HDArea 表单 ID
```

### 3.1 M-Team 分类映射示例

| M-Team cat ID | 中文名 | → hda_type_key | → HDA type ID |
|---------------|--------|----------------|---------------|
| 419 | 电影/HD | Movies 1080p（默认） | 410 |
| 419 + 标题含 2160p/4K | | Movie UHD-4K | 300 |
| 419 + 标题含 720p | | Movies 720p | 411 |
| 421 | 电影/Blu-Ray | Movies Blu-ray | 401 |
| 439 | 电影/Remux | Movies REMUX | 415 |
| 401 | 电影/SD | Movies 720p / DVD | 411/414 |
| 403,402,438,435 | 影剧/综艺 | TV Series | 402 |

### 3.2 视频编码映射示例

| 源 | 内部名称 | → HDA codec_sel ID |
|----|----------|-------------------|
| M-Team videoCodec=1 | x264 | 7 (H.264) |
| M-Team videoCodec=16 | x265 | 6 (H.265) |
| MediaInfo HEVC/H265 | H.265(x265/HEVC) | 6 |
| MediaInfo AV1 | AV1 | 8 |
| 标题正则 x265/HEVC | x265 | 6 |

### 3.3 音频编码映射示例

| 源 | 内部名称 | → HDA audiocodec_sel ID |
|----|----------|------------------------|
| M-Team audioCodec=11 | DTS-HD MA | 4 |
| MediaInfo DTS+HD+MA | DTS-HD MA/DTS XLL | 4 |
| MediaInfo E-AC-3+Atmos | DDP Atmos | 15 |
| MediaInfo TrueHD+Atmos | TrueHD Atmos | 10 |
| M-Team audioCodec=6 | AAC | 6 |

### 3.4 媒介映射示例

| 判断逻辑 | 内部名称 | → HDA medium_sel ID |
|----------|----------|-------------------|
| M-Team cat=421（强制） | BluRay | 1 |
| M-Team cat=439（强制） | REMUX | 3 |
| 标题含 REMUX | REMUX | 3 |
| 标题含 BluRay + 编码名 | Encode | 7 |
| 标题含 BluRay 无编码名 | BluRay | 1 |
| 标题含 WEB-DL/WEB | WEB-DL | 9 |
| 标题含 HDTV | HDTV | 5 |
| 默认 | Encode | 7 |

### 3.5 映射设计要点

1. **config.yaml 外置映射表**: 映射关系在配置文件中定义，支持别名（如 `x264` 和 `H.264(x264/AVC)` 都指向 ID 7），无需改代码即可调整
2. **三级覆盖**: 源站 API/正则 → MediaInfo 覆盖 codec 和 audio → 标题正则保留 resolution
3. **分辨率用标题不用 MediaInfo**: 裁剪视频像素不标准
4. **编码/音频用 MediaInfo 覆盖**: 文件元数据比标题更准确
5. **默认值**: type→410(1080p), codec→7(H.264), audio→3(DTS), medium→7(Encode), standard→1(1080p), team→6(other)

> 完整映射表见 `docs/11-HDApt自动转发工具.md §10`。

---

## 四、与其他 NexusPHP 站点对比

| 特征 | HDArea | 常见 NexusPHP |
|------|--------|---------------|
| 分类 | **18个**（含 UHD-4K=300、3D、iPad） | 通常 5-15 个 |
| 视频编码 | **10个**（含 AVS、VP8/9、AV1） | 通常 5-7 个 |
| 音频编码 | **29个**（最全：DTS:X/DSD/MQA/AC-4/MPEG-H/AV3A） | 通常 6-15 个 |
| 制作组 | 13个 | 通常 3-30 个 |
| source_sel | **无** | 部分站有 |
| 标签 | **无** | 部分站有 |
| IMDb/豆瓣 | 均支持（**豆瓣必填**） | 大多支持 |
| Cloudflare | 是 | 部分站 |
| 盒子规则 | **100%计量 + 72h/3x上传限额** | 少数站有 |
| 完结剧 | **删除分集只保留合集** | 通常不限制 |
| 候选制 | 是（部分用户需候选） | 部分站有 |
| 发布者奖励 | **双倍上传** | 因站而异 |
| 官种促销阶梯 | **原盘→30%/1080P→30%/720P→50%** | 因站而异 |

---

## 五、适配器实现要点

### 5.1 上传流程

```go
func (a *HDAreaAdapter) Upload(ctx context.Context, req *PublishRequest) error {
    // 1. 预热 session: GET upload.php
    // 2. 构建 multipart form
    // 3. POST takeupload.php
    // 4. 检查响应：302 + id=NNN = 成功
    // 5. 200 + 错误文本 = 失败
    // 6. 重定向到 survey-smiles.com = cookie 失效
}
```

### 5.2 字段映射

```go
func mapHDAreaCategory(standardCat string) int {
    switch standardCat {
    case "Movie/UHD":    return 300
    case "Movie/BluRay": return 401
    case "Movie/Remux":  return 415
    case "Movie/1080p":  return 410
    case "Movie/720p":   return 411
    case "Movie/DVD":    return 414
    case "Movie/WEBDL":  return 412
    case "TV/Series":    return 402
    case "TV/Show":      return 403
    case "Doc":          return 404
    case "Anime":        return 405
    case "Music/Video":  return 406
    case "Sport":        return 407
    case "Misc":         return 409
    case "Audio/HQ":     return 408
    default:             return 410
    }
}
```

### 5.3 注意事项

- Cloudflare 防护：需先 GET upload.php 预热 session
- 种子文件名必须为 ASCII
- 所有文本字段需去除 4 字节 emoji
- 无 source_sel 字段，媒介判断尤为重要
- 无标签字段，转载信息写入简介 BBCode

---

*数据来源: upload.php HTML (68458字节) + rules.php HTML (45088字节) + 论坛#6126盒子规则 (30729字节) + 论坛#6123完结剧规则 (27574字节) + HDApt Auto Transfer 源码分析 (2026-04-17/2026-04-22)*
*文档创建: 2026-04-17*
*文档更新: 2026-04-22 — 补充完整 rules.php 规则、盒子规则(#6126)、完结剧规则(#6123)、字幕区规则、Tracker URL*
