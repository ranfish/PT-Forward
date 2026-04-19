# 星陨阁 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 星陨阁 |
| 域名 | pt.xingyungept.org |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是 |
| MediaInfo | 是（technical_info） |
| IMDb | 是（url） |
| 豆瓣 | 否 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| PT-Gen | 是（pt_gen） |

## Tracker URL
`https://tracker.xingyungept.org/announce.php`

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
| MediaInfo | `technical_info` | 否 | |
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
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音频 |
| 409 | 其他 |
| 410 | 短剧 |

## 质量字段 (mode=4)

### 媒介 medium_sel[4]

| ID | 名称 |
|----|------|
| 1 | Blu-ray |
| 2 | UHD Blu-ray |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | DVD |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | Other |

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 2 | H.265/HEVC |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | AV1 |
| 6 | Other |

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 1 | 480p/480i |
| 2 | 720p/720i |
| 3 | 1080p/1080i |
| 4 | 4K/2160p/2160i |
| 5 | 8k/4320p/4320i |
| 6 | Other |

### 音频编码 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | MP3 |
| 3 | WAV |
| 4 | M4A |
| 5 | DTS |
| 6 | DTS-HD MA |
| 7 | DTS:X |
| 8 | TrueHD |
| 9 | LPCM |
| 10 | DD/AC3 |
| 11 | DDP/E-AC3 |
| 12 | TrueHD Atmos |
| 13 | APE |
| 14 | AAC |
| 15 | ALAC |
| 16 | Other |
| 17 | OPUS |
| 18 | AV3V |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | rain |
| 7 | rainweb |
| 8 | StarfallWeb |
| 9 | AGSVWEB |
| 10 | Starfall |
| 11 | NatureWeb |
| 12 | Pure@StarfallWeb |

## 标签 tags[4][]（24 个）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 杜比视界 |
| 9 | 特效字幕 |
| 10 | 分集 |
| 11 | 完结 |
| 12 | 粤语 |
| 13 | 美剧 |
| 14 | 韩剧 |
| 15 | 英字 |
| 16 | 应求 |
| 17 | 大包 |
| 18 | 特效 |
| 19 | 超分 |
| 20 | 补帧 |
| 23 | 驻站 |
| 24 | 原生 |
| 25 | 去头尾广告纯净版 |
| 26 | 高帧率 |
| 27 | 高码率 |

## 缺失字段

- **无 source_sel**
- **无 processing_sel**
- **无豆瓣链接**

## 特殊说明

1. **媒介区分 FHD/UHD**：Blu-ray(1) 与 UHD Blu-ray(2) 独立
2. **编码极简**：仅 6 种（H.264/H.265/VC-1/MPEG-2/AV1/Other），无 x264/x265 区分，无 VP9
3. **19 个音频编码**：含 AV3V(18)/ALAC(15)/OPUS(17)/M4A(4)
4. **分辨率含 8K 和 480p**：480p(1)/720p(2)/1080p(3)/4K(4)/8K(5)/Other(6)
5. **12 个制作组**：站组系列 Starfall(10)/StarfallWeb(8)/Pure@StarfallWeb(12)/NatureWeb(11)，rain/rainweb，传统组 HDS/CHD/MySiLU/WiKi，加 AGSVWEB
6. **24 个标签**：含超分(19)/补帧(20)/高帧率(26)/高码率(27)/去头尾广告纯净版(25) 等特色标签
7. **标签含地区/类型标签**：美剧(13)/韩剧(14) 按地区，原生(24) 标签
8. **短剧独立分类**：短剧(410)
9. **PT-Gen 独立字段**：有专门的 pt_gen 字段
10. **标准 NexusPHP 规则**：Dupe/打包/保留规则均为标准模板
11. **Tracker 独立子域名**：tracker.xingyungept.org
