# 好多油 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 好多油|
| 站点地址 | https://pt.hdupt.com |
| 站点框架 | NexusPHP |
| 特殊功能 | 媒介区分 TV/电影、UHD Blu-ray/UHD Remux 独立选项、processing 地区 8 类 |
| 发布页面 | upload.php |

---

## 一、发布页面表单字段（upload.php）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `file` | file | 种子文件 |
| `name` | text | 主标题 |
| `small_descr` | text | 副标题 |
| `nfo` | file | NFO 文件 |
| `descr` | textarea | 简介（BBCode） |
| `uplver` | checkbox | 匿名发布（value=yes） |

**无** IMDb 字段、**无** 豆瓣字段、**无** PT-Gen 字段、**无** MediaInfo 独立字段、**无** 标签字段、**无** source_sel 字段。

### 1.2 分类（`type`）— 10个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/纪录片 |
| 405 | Animations/动画 |
| 406 | Music Videos/音乐 MV |
| 407 | Sports/体育 |
| 408 | HQ Audio/无损音乐 |
| 411 | Misc/其他 |
| 410 | Games/游戏 |

### 1.3 媒介（`medium_sel`）— 15个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | Blu-ray | FHD 蓝光原盘 |
| 11 | UHD Blu-ray | UHD 蓝光原盘 |
| 5 | HDTV | 高清电视 |
| 6 | DVD | DVD |
| 3 | Remux | FHD Remux |
| 15 | UHD Remux | UHD Remux（电影） |
| 16 | UHD Remux TV | UHD Remux（剧集） |
| 12 | Remux TV | FHD Remux（剧集） |
| 7 | Encode | 压制（电影） |
| 14 | Encode TV | 压制（剧集） |
| 10 | WEB-DL/WEBRip | WEB-DL/WEBRip（电影） |
| 13 | WEB-DL/WEBRip TV | WEB-DL/WEBRip（剧集） |
| 4 | MiniBD | MiniBD |
| 8 | CD | CD |
| 9 | Track | 单曲音轨 |

**重要**: 媒介细分 TV/电影——同一类型有不同的电影和剧集选项（如 `Encode`(7) vs `Encode TV`(14)，`WEB-DL`(10) vs `WEB-DL/WEBRip TV`(13)）。UHD Blu-ray 和 UHD Remux 为独立选项。这是已采集站点中媒介细分最详细的之一。

### 1.4 视频编码（`codec_sel`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 14 | H.265/HEVC |
| 2 | VC-1 |
| 16 | x264 |
| 3 | Xvid |
| 18 | MPEG/MPEG-2 |
| 5 | Other |

**注意**: H.264/AVC(1) 和 x264(16) 为独立选项（区分原盘编码和压制编码），无 AV1、VP8/9、AVS。

### 1.5 音频编码（`audiocodec_sel`）— 13个

| 值 | 显示名称 |
|----|----------|
| 16 | DTS:X |
| 1 | DTS-HDMA |
| 3 | TrueHD |
| 11 | LPCM |
| 4 | DTS |
| 2 | AC3/EAC3 |
| 6 | AAC |
| 7 | FLAC |
| 10 | APE |
| 17 | WAV |
| 18 | MPEG |
| 13 | Other |

**注意**: 无 TrueHD Atmos、DDP Atmos、DDP/E-AC-3 独立区分；AC3 和 EAC3 合并为 AC3/EAC3(2)。DTS-HDMA(1) 无 "/DTS XLL" 后缀。无 MP3、Opus、DSD、AV3A、AC-4、MPEG-H、MQA。

### 1.6 分辨率（`standard_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 5 | 4K/2160p |
| 3 | 720p |
| 4 | SD |
| 6 | iPad |

**注意**: 含 iPad(6) 分辨率选项。

### 1.7 地区（`processing_sel`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | CN/中国内地 |
| 3 | HK/TW/港台 |
| 2 | US/EU/欧美 |
| 4 | JP/日本 |
| 5 | KR/韩国 |
| 6 | India/印度 |
| 8 | SEA/东南亚 |
| 7 | Other |

**注意**: 含 India(6) 和 SEA(8)，是已采集站点中地区分类较细的之一。

### 1.8 制作组（`team_sel`）— 3个

| 值 | 显示名称 |
|----|----------|
| 2 | HDU |
| 5 | Other |

仅 2 个有效选项（本站官组 HDU + Other），是已采集站点中最少的。

---

## 二、HDUPT 特殊注意事项

### 2.1 媒介 TV/电影区分

发布时必须根据分类（Movies vs TV Series）选择对应媒介：
- 电影类: Encode(7), WEB-DL/WEBRip(10), Remux(3), UHD Remux(15)
- 剧集类: Encode TV(14), WEB-DL/WEBRip TV(13), Remux TV(12), UHD Remux TV(16)
- 通用: Blu-ray(1), UHD Blu-ray(11), HDTV(5), DVD(6), MiniBD(4), CD(8), Track(9)

适配器需要根据分类自动选择正确的媒介值。

### 2.2 H.264 vs x264 区分

- `H.264/AVC`(1): 用于原盘/Remux（未经再次编码）
- `x264`(16): 用于 Encode（使用 x264 编码器压制）

适配器需要根据媒介类型判断使用哪个值。

### 2.3 无 IMDb/豆瓣/PT-Gen 字段

发布表单无 IMDb、豆瓣、PT-Gen 输入框，影视链接信息只能写入简介 BBCode 中。

### 2.4 UHD 独立媒介

UHD Blu-ray(11) 和 UHD Remux(15)/UHD Remux TV(16) 为独立选项，而非通过分辨率+媒介组合表示。适配器需根据分辨率判断是否使用 UHD 媒介。

---

## 三、与其他 NexusPHP 站点对比

| 特征 | HDUPT | 常见 NexusPHP |
|------|--------|---------------|
| 分类 | 10个（含 Games） | 通常 5-15 个 |
| 媒介 | **15个（TV/电影分开+UHD独立）** | 通常 8-10 个 |
| 视频编码 | 7个（H.264/x264 分开） | 通常 5-10 个 |
| 音频编码 | 13个（AC3/EAC3 合并） | 通常 6-15 个 |
| 分辨率 | 6个（含 iPad） | 通常 4-6 个 |
| 地区 | **8个（含 India/SEA）** | 通常无或 6 个 |
| 制作组 | **2个（最少）** | 通常 3-30 个 |
| IMDb/豆瓣 | **无** | 大多支持 |
| source_sel | 无 | 部分站有 |

---

## 四、适配器实现要点

### 4.1 TV/电影媒介选择

```go
func mapHDUPTMedium(baseMedium string, category int) int {
    tvCategories := map[int]bool{402: true, 403: true, 404: true, 405: true}
    isTV := tvCategories[category]
    
    switch baseMedium {
    case "Encode":
        if isTV { return 14 } else { return 7 }
    case "WEB-DL":
        if isTV { return 13 } else { return 10 }
    case "Remux":
        if isTV { return 12 } else { return 3 }
    case "UHD Remux":
        if isTV { return 16 } else { return 15 }
    case "Blu-ray":
        return 1
    case "UHD Blu-ray":
        return 11
    case "HDTV":
        return 5
    case "DVD":
        return 6
    default:
        if isTV { return 14 } else { return 7 }
    }
}
```

### 4.2 编码器选择

```go
func mapHDUPTCodec(codec string, medium int) int {
    // 原盘/Remux 使用 H.264/AVC(1)，Encode 使用 x264(16)
    isOriginal := medium == 1 || medium == 11 || medium == 3 || medium == 15 || medium == 12 || medium == 16
    
    switch codec {
    case "H.264", "AVC", "x264":
        if isOriginal { return 1 } else { return 16 }
    case "H.265", "HEVC", "x265":
        return 14
    case "VC-1":
        return 2
    case "Xvid":
        return 3
    case "MPEG-2", "MPEG":
        return 18
    default:
        return 5
    }
}
```

---

*数据来源: upload.php HTML (45687字节) (2026-04-17)*
*文档创建: 2026-04-17*
