# 天空 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 天空 |
| 域名 | hdsky.me |
| 框架 | NexusPHP |
| Cloudflare | 是 |
| 候选制 | 是（终身影帝以上可直接发布） |
| MediaInfo | 是 |
| IMDb | 是 |
| 豆瓣 | 是（url_douban） |
| 匿名发布 | 是（uplver） |
| NFO | 是 |

## Tracker URL
`https://tracker.hdsky.me/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
| 豆瓣链接 | `url_douban` | 否 | |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| 类型 | `type` | 是 | |
| 媒介 | `medium_sel` | 否 | |
| 编码 | `codec_sel` | 否 | |
| 分辨率 | `standard_sel` | 否 | |
| 音频编码 | `audiocodec_sel` | 否 | |
| 制作组 | `team_sel` | 否 | |
| 标签 | `option_sel[]` | 否 | checkbox 多选 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | Movies/电影 |
| 402 | TV Series/剧集(分集) |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/纪录片 |
| 405 | Animations/动漫 |
| 406 | Music Videos/音乐MV |
| 407 | Sports/体育 |
| 408 | HQ Audio/无损音乐 |
| 409 | Misc/其他 |
| 410 | iPad/iPad影视 |
| 411 | TV Series/剧集(合集) |
| 412 | TV Series/海外剧集(分集) |
| 413 | TV Series/海外剧集(合集) |
| 414 | TV Shows/海外综艺(分集) |
| 415 | TV Shows/海外综艺(合集) |
| 416 | Shortplay/短剧 |

## 质量字段

### 媒介 medium_sel

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
| 9 | Track |
| 11 | WEB-DL |
| 12 | Blu-ray/DIY |
| 13 | UHD Blu-ray |
| 14 | UHD Blu-ray/DIY |
| 15 | SACD |
| 16 | 3D Blu-ray |

### 编码 codec_sel

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 10 | x264 |
| 11 | Other |
| 12 | HEVC |
| 13 | x265 |
| 14 | MVC |
| 15 | ProRes |
| 16 | AV1 |
| 17 | VP9 |

### 分辨率 standard_sel

| ID | 名称 |
|----|------|
| 1 | 2K/1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K/2160p |
| 6 | 8K/4320P |

### 音频编码 audiocodec_sel

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 10 | DTS-HDMA |
| 11 | TrueHD |
| 12 | AC3/DD |
| 13 | LPCM |
| 14 | DTS-HD HR |
| 15 | WAV |
| 16 | DTS-HDMA:X 7.1 |
| 17 | TrueHD Atmos |
| 18 | DSD |
| 19 | PCM |
| 20 | E-AC3 |
| 21 | DDP with Dolby Atmos |
| 22 | Opus |
| 23 | ALAC |

### 制作组 team_sel

| ID | 名称 |
|----|------|
| 1 | HDS |
| 6 | HDSky/原盘DIY小组 |
| 9 | HDSTV |
| 18 | HDSPad |
| 22 | HDSCD |
| 24 | Original |
| 25 | AREA11/韩剧合作小组 |
| 26 | Autoseed |
| 27 | Other |
| 28 | HDS3D |
| 30 | BMDru |
| 31 | HDSWEB |
| 33 | Request |
| 34 | HDSpecial/稀缺资源 |
| 35 | HDSWEB/(网络视频小组合集专用) |
| 36 | HDSAB/有声书小组 |
| 37 | HDSWEB/(补档专用) |

## 标签 option_sel[]（31 个）

| ID | 名称 |
|----|------|
| 1 | 首发 |
| 2 | 禁转 |
| 5 | 国语 |
| 6 | 中字 |
| 9 | HDR10 |
| 11 | 粤语 |
| 12 | 官组 |
| 13 | DIY |
| 14 | 自制 |
| 15 | Dolby Vision |
| 16 | HLG |
| 17 | HDR10+ |
| 18 | SL-HDR1 |
| 19 | 应求 |
| 20 | 特效 |
| 21 | Atmos |
| 22 | 次世代国语 |
| 23 | DTS-X |
| 24 | DoVi+HDR |
| 25 | 限转 |
| 26 | DIY纯净版 |
| 27 | 全景声国语 |
| 28 | 原生原盘 |
| 29 | 临境声国语 |
| 30 | 无广告 |
| 31 | 去头尾广告纯净版 |

## 缺失字段

- **无 source_sel**
- **无 processing_sel**
- **无 tags[]**（使用 option_sel[] 替代）
- **无 PT-Gen**

## 特殊说明

1. **Dupe 规则严格**：Blu-ray/HD DVD > HDTV > DVD > TV 优先级，官组资源不受 Dupe 约束
2. **转发限制**：禁止转发 REMUX 格式电影/剧集
3. **剧集分集限制**：剧集分集仅官方发布员可发
4. **官组保护**：已有官组 HDS/HDSPad 资源不允许再上传其他小组重编码/iPad 资源
5. **豆瓣链接**：支持 url_douban 字段
6. **媒介细分**：区分 Blu-ray(1)/Blu-ray/DIY(12)/UHD Blu-ray(13)/UHD Blu-ray/DIY(14)/3D Blu-ray(16)，含 SACD(15)
7. **标签用 option_sel[]**：不使用标准 tags[]，使用 option_sel[] 复选框，含 HDR10/HDR10+/Dolby Vision/HLG/SL-HDR1/DoVi+HDR 等 HDR 细分标签
8. **编码区分 H.264 与 x264**：H.264/AVC(1) 与 x264(10) 分开，HEVC(12) 与 x265(13) 分开
9. **22 个音频编码**：含 DTS-HDMA:X 7.1(16)/TrueHD Atmos(17)/DDP with Dolby Atmos(21)/DSD(18)/ALAC(23)/Opus(22)
10. **制作组以站组为主**：HDS/HDSky/HDSTV/HDSPad/HDSCD/HDS3D/HDSWEB/HDSAB 等站内组，External 组仅 BMDru/AREA11
11. **分类含海外拆分**：海外剧集(412/413)和海外综艺(414/415)独立分类，短剧独立分类(416)
