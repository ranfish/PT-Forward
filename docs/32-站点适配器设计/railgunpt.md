# 轨道炮 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 轨道炮|
| 站点地址 | https://bilibili.download |
| 站点框架 | NexusPHP |
| 特殊功能 | 下架视频备份分类、漫画分类、学习分类、双模式(mode 4/5) |
| 规则页面 | 无独立 rules.php（规则待确认） |

**站点角色**: 无官组，**只能做目标站（发布站），不能做源站**。

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 模式系统

使用 `data-mode='4'`（影视模式）和 mode=5（可能对应非影视模式）。两个字段集共享相同的值。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布 |

**注意**: 无 `technical_info`（MediaInfo）独立字段。

### 1.3 类型字段（`type`）— 13个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | misc |
| 410 | 软件 |
| 411 | 学习 |
| 412 | 游戏 |
| 419 | 漫画 |
| 420 | 下架视频备份 |

**独特分类**:
- **下架视频备份**(420) — 与站点域名 bilibili.download 定位一致
- **漫画**(419) — 独立分类
- **学习**(411) — 教育资源分类

### 1.4 媒介（`medium_sel[4]`/`medium_sel[5]`）— 9个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | UHD |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | DVD |
| 7 | Encode |
| 8 | CD |
| 9 | Track |

**注意**: 值从大到小倒序排列（1=Blu-ray, 2=UHD）。UHD(2) 独立于 Blu-ray(1)。有 Track(9)。

### 1.5 编码（`codec_sel[4]`/`codec_sel[5]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H264 |
| 2 | H265 |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | XVID |
| 6 | Other |

**注意**: 无 AV1。编码名称简洁（H264/H265 非 H.264/H.265）。有 XVID(5)。

### 1.6 音频编码（`audiocodec_sel[4]`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | TrueHD/Atmos |
| 2 | DTS-HD/DTS-HDMA |
| 3 | AC3 |
| 4 | LPCM |
| 5 | Flac |
| 6 | MP3 |
| 7 | AAC |
| 8 | APE |
| 9 | Other |
| 10 | WAV |

**注意**: TrueHD 和 Atmos 合并为 TrueHD/Atmos(1)。DTS-HD 和 DTS-HD MA 合并为 DTS-HD/DTS-HDMA(2)。无 DTS（基础）、DDP/E-AC-3。

### 1.7 分辨率（`standard_sel[4]`/`standard_sel[5]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 4K |
| 2 | 1080p/i |
| 3 | 720p |
| 4 | SD |
| 5 | Other |
| 6 | 2K |

**注意**: 1080p 和 1080i 合并为 1080p/i(2)。有 4K(1)、2K(6)。mode 5 无 2K(6)。

### 1.8 标签（`tags[4][]`/`tags[5][]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官方 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

**注意**: 值连续（1-7），无缺失。有"官方"(3)。无 Dolby Vision、HDR10+ 等细分标签。

### 1.9 缺失字段

- 无 `team_sel`（制作组）
- 无 `technical_info`（MediaInfo）
- 无 `source_sel`（来源）

---

## 二、字段映射汇总（实际发布用）

### 2.1 类型（`type`）

```json
{
  "电影": 401,
  "剧集": 402,
  "综艺": 403,
  "纪录片": 404,
  "动漫": 405,
  "MV": 406,
  "体育": 407,
  "音乐": 408,
  "misc": 409,
  "软件": 410,
  "学习": 411,
  "游戏": 412,
  "漫画": 419,
  "下架视频备份": 420
}
```

### 2.2 媒介（`medium_sel[4]`）

```json
{
  "Blu-ray": 1,
  "UHD": 2,
  "Remux": 3,
  "WEB-DL": 4,
  "HDTV": 5,
  "DVD": 6,
  "Encode": 7,
  "CD": 8,
  "Track": 9
}
```

### 2.3 编码（`codec_sel[4]`）

```json
{
  "H264": 1,
  "H265": 2,
  "VC-1": 3,
  "MPEG-2": 4,
  "XVID": 5,
  "Other": 6
}
```

### 2.4 音频编码（`audiocodec_sel[4]`）

```json
{
  "TrueHD/Atmos": 1,
  "DTS-HD/DTS-HDMA": 2,
  "AC3": 3,
  "LPCM": 4,
  "Flac": 5,
  "MP3": 6,
  "AAC": 7,
  "APE": 8,
  "Other": 9,
  "WAV": 10
}
```

### 2.5 分辨率（`standard_sel[4]`）

```json
{
  "4K": 1,
  "1080p/i": 2,
  "720p": 3,
  "SD": 4,
  "Other": 5,
  "2K": 6
}
```

### 2.6 标签（`tags[4][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "官方": 3,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR": 7
}
```

---

## 三、RailgunPT 特殊注意事项

### 3.1 仅目标站

无官组，只能做目标站。在 PT-Forward 中应标记为 `SourceEnabled=false`。

### 3.2 下架视频备份分类

独特的"下架视频备份"(420)分类，与站点 bilibili.download 域名定位一致。适配器可将来自己知下架平台的资源归入此分类。

### 3.3 无制作组字段

表单中无 `team_sel`，发布时无需选择制作组。

### 3.4 无 MediaInfo 字段

无 `technical_info` 独立字段，MediaInfo 应写在简介中。

### 3.5 音频编码合并

TrueHD + Atmos 合并为 TrueHD/Atmos(1)，DTS-HD + DTS-HD MA 合并为 DTS-HD/DTS-HDMA(2)。无独立 Atmos、DTS:X、DDP 选项。

### 3.6 双模式

mode 4（影视）有额外 `audiocodec_sel` 字段，mode 5（非影视）无音频编码字段。其他字段（媒介/编码/分辨率）值完全相同。

### 3.7 媒介值倒序

媒介值从 1（Blu-ray，最高质量）到 9（Track），非按数字大小排列质量等级。

---

## 四、与其他 NexusPHP 站点对比

| 特征 | RailgunPT | ITZMX | 常见 NexusPHP |
|------|-----------|-------|---------------|
| 站点角色 | **仅目标站** | **仅目标站** | 源站/目标站 |
| 类型数量 | 13个（含下架备份/漫画/学习） | 8个 | 通常 8-10 个 |
| 制作组 | **无** | **1个（Other）** | 通常 3-30 个 |
| MediaInfo | **无** | **无** | 通常有 |
| 媒介 | 9个 | **无** | 通常 6-10 个 |
| 编码 | 6个 | **无** | 通常 5-9 个 |
| 音频编码 | 10个 | **无** | 通常 6-12 个 |
| 分辨率 | 6个（含2K） | 3个 | 通常 5-7 个 |
| 标签 | 7个 | **无** | 通常 3-21 个 |
| 独特分类 | 下架视频备份 | 蓝光 | 无 |

---

## 五、适配器实现要点

### 5.1 无制作组

```go
// No team_sel field — skip team mapping entirely
adapter.SkipTeam = true
```

### 5.2 下架视频备份分类

```go
func mapType(category string, isDelisted bool) int {
    if isDelisted {
        return 420 // 下架视频备份
    }
    switch category {
    case "Movies": return 401
    case "TV Series": return 402
    // ...
    }
}
```

### 5.3 音频编码合并匹配

```go
func mapAudioCodec(codec string) int {
    switch {
    case strings.Contains(codec, "Atmos") || strings.Contains(codec, "TrueHD"):
        return 1  // TrueHD/Atmos
    case strings.Contains(codec, "DTS-HD MA") || strings.Contains(codec, "DTS-HD"):
        return 2  // DTS-HD/DTS-HDMA
    case strings.Contains(codec, "DTS"):
        return 2   // DTS 也归入 DTS-HD/DTS-HDMA
    case strings.Contains(codec, "AC3") || strings.Contains(codec, "DD"):
        return 3
    case strings.Contains(codec, "FLAC"):
        return 5
    case strings.Contains(codec, "MP3"):
        return 6
    case strings.Contains(codec, "AAC"):
        return 7
    default:
        return 9  // Other
    }
}
```

---

*数据来源: upload.php HTML (501行) (2026-04-16)*
*文档创建: 2026-04-16*
