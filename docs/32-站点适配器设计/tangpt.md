# 趟平 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 趟平 |
| 域名 | www.tangpt.top |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（offers.php） |
| MediaInfo | 是（technical_info） |
| IMDb | 是 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |

## Tracker URL
`https://www.tangpt.top/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
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

## 分类 (type, mode=4)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 409 | 音乐 |
| 414 | 其他 |

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
| 9 | Other |
| 10 | UHD Blu-ray |
| 11 | WEB-DL |

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265/HEVC |
| 7 | AV1 |
| 10 | VP8/9 |

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 1 | 1080i/1080P |
| 2 | 720i/720P |
| 3 | 480i/480P |
| 4 | 2K/1440i/1440P |
| 5 | 4K/2160i/2160P |
| 6 | 8K/4320i/4320P |
| 7 | Other |

### 音频编码 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | AV3V |
| 9 | TrueHD Atmos |
| 10 | DDP/E-AC3 |
| 11 | DD/AC3 |
| 12 | TrueHD |
| 13 | WAV |
| 14 | DTS |
| 15 | DTS:X |
| 16 | DTS-HD MA |
| 17 | M4A |
| 18 | OPUS |
| 19 | LPCM |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | StarfallWeb |
| 7 | AGSVWEB |
| 8 | FRDS |
| 10 | MWeb |
| 11 | HDSWEB |
| 12 | PTerWEB |
| 14 | HDHome |
| 15 | BtsHD |
| 16 | OurTV |
| 17 | CMCT |
| 19 | FFansDIY |
| 21 | HDArea |
| 23 | beAst |
| 24 | OpenCD |
| 26 | U2 |
| 31 | ADWeb |
| 34 | HHWEB |
| 35 | ZmWeb |
| 36 | UBWEB |
| 37 | QHstudIo |
| 38 | CSWEB |
| 39 | TPWEB |
| 40 | AilMWeb |
| 41 | LUCKDIY |
| 42 | LUCKWEB |
| 43 | LUCKMUSIC |

## 标签 tags[4][]（30 个）

| ID | 名称 |
|----|------|
| 1 | 演唱会 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 特效 |
| 9 | DV |
| 10 | 粤语 |
| 15 | 动画 |
| 19 | 完结 |
| 20 | 大包 |
| 21 | 分集 |
| 22 | 纯享 |
| 23 | 讲座 |
| 24 | 短视频 |
| 27 | 情色 |
| 28 | 自制 |
| 29 | 软件 |
| 30 | 游戏 |
| 31 | 书籍 |
| 32 | 恐怖 |
| 34 | 娱乐 |
| 35 | 玄学 |
| 36 | 自由 |
| 38 | 写真 |
| 39 | 禁转 |
| 40 | cosplay |
| 41 | NSFW |
| 42 | 秀人网 |
| 45 | 首发 |

## 缺失字段

- **无 source_sel**
- **无 processing_sel**
- **无 PT-Gen**

## 特殊说明

1. **standard_sel 替代 resolution_sel**：含 8K/4K/2K/1080/720/480，同时含 i/P 标注
2. **32 个制作组**：含大量 WEB 组（MWeb/HDSWEB/PTerWEB/AGSVWEB/ADWeb/HHWEB/ZmWeb/UBWEB/CSWEB/TPWEB/AilMWeb/LUCKWEB/LUCKMUSIC）+ 传统组（CHD/MySiLU/WiKi/beAst/CMCT/FFansDIY 等）
3. **音频含 AV3V**：支持 AV3V（中国音频编码）+ M4A/OPUS
4. **30 个标签**：含 NSFW/情色/写真/cosplay/秀人网 等成人向标签，短视频/讲座/书籍/游戏/软件 等非影视标签
5. **编码含 AV1**：支持 AV1(7)
6. **站组 TPWEB**：制作组含站组 TPWEB(39)
7. **U2 制作组**：含 U2(26) 制作组
