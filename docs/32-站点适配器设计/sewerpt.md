# 下水道 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 下水道 |
| 域名 | sewerpt.com |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是 |
| MediaInfo | 否（在简介中附带） |
| IMDb | 是（url） |
| 豆瓣 | 否 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| PT-Gen | 是（pt_gen） |

## Tracker URL
`https://sewerpt.com/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
| PT-Gen | `pt_gen` | 否 | PT-Gen 链接 |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| 类型 | `type` | 是 | |
| 媒介 | `medium_sel[4]` | 否 | |
| 编码 | `codec_sel[4]` | 否 | |
| 分辨率 | `standard_sel[4]` | 否 | |
| 音频编码 | `audiocodec_sel[4]` | 否 | |
| 制作组 | `team_sel[4]` | 否 | |
| 标签 | `tags[4][]` | 否 | checkbox 多选 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | 电影 / Movies |
| 402 | 电视剧 / TV Series |
| 403 | 综艺 / TV Shows |
| 404 | 纪录片 / Documentaries |
| 405 | 动漫 / Animations |
| 408 | 音乐 / Music |
| 409 | 其他 / Misc |

## 质量字段 (mode=4)

### 媒介 medium_sel[4]

| ID | 名称 |
|----|------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 10 | WEB-DL |

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | HEVC |

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 1 | 1080p/1080i |
| 2 | 480p |
| 3 | 720p |
| 4 | 2K/1440p |
| 5 | 4K/2160p |
| 6 | 8K/4320p |

### 音频编码 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | AC3 |
| 9 | ALAC |
| 10 | WAV |
| 11 | E-AC3 |
| 12 | TrueHD Atmos |
| 13 | TrueHD |
| 14 | DTS-HD MA |
| 15 | DTS:X |
| 16 | LPCM |
| 17 | AV3A |
| 18 | OPUS |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 1 | SewageWeb |
| 5 | Other |

## 标签 tags[4][]（16 个）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官方 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 分集 |
| 9 | 原创 |
| 10 | 原盘 |
| 11 | 冷门/低分 |
| 12 | 完结 |
| 13 | 短剧 |
| 14 | 杜比 |
| 15 | 粤语 |
| 16 | 高码率 |

## 缺失字段

- **无 source_sel**
- **无 processing_sel**
- **无独立 MediaInfo 字段**
- **无豆瓣链接**

## 特殊说明

1. **极简制作组**：仅 SewageWeb(1) 和 Other(5) 两个选项
2. **19 个音频编码**：含 AV3A(17)/ALAC(9)/OPUS(18)，较完整
3. **编码极简**：仅 AVC/HEVC/VC-1/Xvid/MPEG-2/Other 6 种，无 AV1/VP9
4. **无 UHD Blu-ray 媒介**：最高媒介为 Blu-ray(1)，无独立 UHD 选项
5. **分辨率含 2K/1440p 和 8K**：480p(2)/720p(3)/1080p(1080i)(1)/2K(1440p)(4)/4K(2160p)(5)/8K(4320p)(6)
6. **16 个标签**：含冷门/低分(11)/粤语(15)/短剧(13)/高码率(16)/杜比(14) 等特色标签
7. **PT-Gen 独立字段**：有专门的 pt_gen 字段
8. **分类仅 7 个**：无 MV/体育独立分类，无 3D 分类
9. **导航栏特色分区**：首页有"冷门/低分"和"粤语"独立种子浏览入口
