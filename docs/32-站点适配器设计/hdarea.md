# 好大 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 好大|
| 站点地址 | https://hdarea.club |
| 站点框架 | NexusPHP |
| 特殊功能 | Cloudflare 防护、29 种音频编码（最全之一）、MQA 禁止 |
| 规则页面 | forums.php?action=viewtopic&forumid=4&topicid=5622（新手发种教程） |
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

## 二、发种规则摘要

### 2.1 标题命名规范

主标题组成（按顺序）：

```
资源英文名称 发行年份 [S季E集] [REPACK] 分辨率 来源 [HDR] [色深] 视频编码 [xAudio] 音频编码 音频通道-制作组
```

示例：`Ruozhiba Featured Joke Clips 2024 S1E1 REPACK 1080p BluRay DV HDR 10bit x265 4Audio DTS-HD MA 5.1-JQY@Ruozhiba`

**副标题**：中文名 + 源语言名 + 季/集 + 字幕/音频信息 + 转载信息。

### 2.2 简介（BBCode）结构

```
[quote][b][color=Blue]转自xxx，感谢原作者发布[/color][/b][/quote]  ← 转载信息
[img]海报URL[/img]                                                    ← 海报
PT-Gen 生成的豆瓣/IMDb 简介                                            ← 简介
[img]截图1[/img][img]截图2[/img][img]截图3[/img]                      ← 截图（≥3张）
[quote]MediaInfo文本[/quote]                                           ← Info信息
```

### 2.3 特殊注意事项

- **Cloudflare 防护**: 站点使用 Cloudflare，需先 GET `upload.php` 预热 session
- **种子文件名**: 必须为 ASCII（非 ASCII 文件名会出错）
- **4 字节 emoji 禁止**: NexusPHP MySQL utf8（非 utf8mb4），4 字节字符会导致截断
- **种子格式**: qBittorrent libtorrent ≥2.0 需使用 V1 格式
- **查重**: 发布前必须查重

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
| IMDb/豆瓣 | 均支持 | 大多支持 |
| Cloudflare | 是 | 部分站 |

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

*数据来源: upload.php HTML (70944字节) + forums.php 规则页 (149547字节) + HDApt Auto Transfer 源码分析 (2026-04-17)*
*文档创建: 2026-04-17*
