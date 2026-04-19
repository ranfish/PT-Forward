# 壹吧 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 壹吧 |
| 域名 | 1ptba.com |
| 框架 | NexusPHP |
| Cloudflare | 是 |
| 候选制 | 是 |
| IMDb | 是（url字段） |
| 豆瓣 | 否 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| YouTube链接 | 是（custom_fields[mode][1]） |
| 认领系统 | 是（0/1000） |
| H&R | 是（5天内做种24小时，8个不达标封禁） |

## Tracker URL
`https://1ptba.com/announce.php`（推测）

## 站点特色

- **教育特色**：站点描述为"教育视频,课件资源,发布教育类,学习类,纪录片等资源"
- **双分类系统**：种子区（影视/教育类）+ 特别区（**9KG/成人内容，禁止下载和发布**）
- **特别区禁止规则**：禁止下载"特 區"版块资源，禁止发布资源到"特别区"
- **种子最小1GB**：种子小于1GB的资源会被删除
- **source_sel非标准语义**：source_sel用作媒介来源（含原盘/REMUX/Encode/WEB-DL），同时medium_sel也独立存在
- **codec_sel混合音视频**：编码下拉混合了视频编码（H.264/HEVC/VC-1等）和音频编码（FLAC/APE/DTS/AC-3/WAV/MP3/ALAC/AAC）
- **processing_sel极简**：仅Raw/Encode两项
- **8K分辨率**：standard_sel含8K-UHD
- **音乐标签丰富**：含音乐专辑/MV/卡拉OK/LIVE现场/演唱会标签
- **规则与学校/阳光同模板**：上传/Dupe/打包/促销规则完全一致
- **发布者双倍上传量**

## ⚠ 特别区禁止规则

**本软件禁止下载和发布 9KG/色情/成人类内容。**

特别区（specialcat, data-mode=9）包含16个成人内容分类（AV有码/无码/HD/SD/DVDiSo/Blu-Ray/网站/写真/H-Game/H-Anime/H-Comic/成人电影/Gay等），**严禁访问、下载或发布该区域任何内容**。

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 否 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| 类型 | `type` | 是 | 双select（种子区browsecat + 特别区specialcat） |
| 来源媒介 | `source_sel[mode]` | 否 | mode=4/9，含原盘/REMUX/Encode/WEB-DL |
| 媒介 | `medium_sel[mode]` | 否 | mode=4/9 |
| 编码(音视频混合) | `codec_sel[mode]` | 否 | mode=4/9，混合视频+音频编码 |
| 音频编码 | `audiocodec_sel[4]` | 否 | 仅种子区 |
| 分辨率 | `standard_sel[mode]` | 否 | mode=4/9 |
| 处理方式 | `processing_sel[4]` | 否 | 仅种子区，Raw/Encode |
| 制作组 | `team_sel[mode]` | 否 | mode=4/9 |
| YouTube链接 | `custom_fields[mode][1]` | 否 | mode=4/9 |
| 标签 | `tags[mode][]` | 否 | checkbox多选，mode=4/9 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

### 种子区 (browsecat, data-mode=4)

| ID | 名称 |
|----|------|
| 401 | Movie(電影) |
| 402 | TV Series(電視影劇) |
| 403 | TV Shows(電視綜藝) |
| 404 | Discovery(紀錄教育) |
| 405 | Cartoon(卡通動漫) |
| 406 | Music Videos(音樂短片/演唱會) |
| 407 | Sports(體育賽事) |
| 408 | HQ Audio(高品质音频) |
| 410 | Software(軟體) |
| 411 | Games(電子遊戲) |
| 412 | eBook(電子書) |
| 409 | Misc(其他) |

### 特别区 (specialcat, data-mode=9) — **禁止访问**

| ID | 名称 |
|----|------|
| 610 | AV(有碼)/HD Censored |
| 611 | AV(無碼)/HD Uncensored |
| 612 | AV(有碼)/SD Censored |
| 613 | AV(無碼)/SD Uncensored |
| 614 | AV(無碼)/DVDiSo Uncensored |
| 615 | AV(有碼)/DVDiSo Censored |
| 616 | AV(有碼)/Blu-Ray Censored |
| 617 | AV(無碼)/Blu-Ray Uncensored |
| 618 | AV(網站)/0Day |
| 619 | IV(寫真影集)/Video Collection |
| 620 | IV(寫真圖集)/Picture Collection |
| 621 | H-Game(遊戲) |
| 622 | H-Anime(動畫) |
| 623 | H-Comic(漫畫) |
| 624 | Adult film(成人電影) |
| 625 | AV(Gay)/HD |

> **以上分类全部为成人内容，本软件禁止下载和发布。**

## 质量字段

### 来源媒介 source_sel[mode]（非标准语义）

> source_sel在此站用作媒介来源，而非标准NexusPHP的地区含义

| ID | 名称 |
|----|------|
| 1 | Blu-ray(原盘) |
| 2 | DVD(原盘) |
| 4 | HDTV |
| 6 | Other |
| 16 | UHD Blu-ray |
| 17 | UHD Blu-ray/DIY |
| 19 | Blu-ray/DIY |
| 20 | REMUX |
| 22 | encode |
| 23 | WEB-DL |
| 25 | CD |
| 26 | Track |

### 媒介 medium_sel[mode]

| ID | 名称 |
|----|------|
| 9 | Track |
| 8 | CD |
| 6 | DVDR |
| 5 | HDTV |
| 4 | MiniBD |
| 7 | Encode |
| 3 | Remux |
| 2 | HD DVD |
| 1 | Blu-ray(原盘) |
| 16 | UHD Blu-ray |
| 17 | UHD Blu-ray/DIY |
| 19 | Blu-ray/DIY |

### 编码 codec_sel[mode]（音视频混合）

| ID | 名称 | 类型 |
|----|------|------|
| 1 | H.264(AVC) | 视频 |
| 2 | VC-1 | 视频 |
| 3 | Xvid | 视频 |
| 4 | MPEG-2 | 视频 |
| 5 | Other | 通用 |
| 18 | H.265(HEVC) | 视频 |
| 19 | FLAC | 音频 |
| 20 | APE | 音频 |
| 21 | DTS | 音频 |
| 22 | AC-3 | 音频 |
| 23 | WAV | 音频 |
| 24 | MP3 | 音频 |
| 25 | ALAC | 音频 |
| 26 | AAC | 音频 |

> codec_sel混合了视频编码和音频编码，转载时需根据资源类型选择正确的编码

### 音频编码 audiocodec_sel[4]（仅种子区）

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 31 | TrueHD |

> 仅种子区有独立音频编码下拉，特别区无

### 分辨率 standard_sel[mode]

| ID | 名称 |
|----|------|
| 1 | 1080p-HD(逐行)/1920×1080 |
| 2 | 1080i-HD(隔行)/1920×1080 |
| 3 | 720p-HD(逐行)/1280×720 |
| 4 | SD(标清)/720p×576p |
| 16 | 4K-UHD(超高清)/3840×2160 |
| 17 | 8K-UHD(超高清)/7680×4320 |

### 处理方式 processing_sel[4]（仅种子区）

| ID | 名称 |
|----|------|
| 1 | Raw |
| 2 | Encode |

### 制作组 team_sel[mode]

| ID | 名称 |
|----|------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 20 | 1PTBA |

> 6个制作组，含经典压制组HDS/CHD/MySiLU/WiKi

## 标签 tags[mode][]

### 种子区 tags[4][]（21个）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 17 | 限转 |
| 18 | 原创 |
| 2 | 首发 |
| 5 | 国配 |
| 19 | 粤配 |
| 6 | 中字 |
| 20 | 官字组 |
| 4 | DIY |
| 21 | Dolby Vision |
| 7 | HDR10 |
| 22 | HDR10+ |
| 23 | 应求 |
| 24 | 特效 |
| 25 | 音乐专辑 |
| 26 | Music Video |
| 27 | 卡拉OK |
| 28 | LIVE现场 |
| 29 | 演唱会 |
| 31 | AI修复 |
| 30 | 最佳影片 |

### 特别区 tags[9][]

与种子区相同（21个标签）。

## 标题命名规范（来自rules.php）

### 电影类
- `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 例：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS`

### 剧集类
- `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 例：`越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU`

### 音轨类
- `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 例：`恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC`

## 发布规则

### 重要规则
- **种子最小1GB**：小于1GB的资源会被删除
- 总体积小于100MB的资源禁止
- 禁止RAR等压缩文件
- 禁止色情/敏感政治内容

### Dupe规则
- 来源媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 断种45日或发布18个月以上可重发

### H&R规则
- 下载完成后5天内做种24小时
- H&R不达标需主动用魔力值消除
- 否则影响账户停用

### 认领规则
- 每人最多认领1000个种子

### 促销规则
- 随机促销（同学校/阳光模板）
- >20GB自动Free
- 蓝光原盘自动Free
- 1个月后永久2X

## 转载注意事项

1. **特别区禁止**：禁止下载和发布特别区任何内容
2. **source_sel非标准**：用作媒介来源而非地区，转载时需注意映射
3. **codec_sel混合音视频**：编码下拉同时包含视频编码和音频编码，需根据资源类型选择
4. **种子最小1GB**：小于1GB的资源会被删除，转载时需检查资源大小
5. **制作组仅6个**：HDS/CHD/MySiLU/WiKi/Other/1PTBA，大部分资源需选Other
6. **分辨率含8K**：standard_sel包含8K-UHD
7. **音乐标签丰富**：有音乐专辑/MV/卡拉OK/LIVE/演唱会等特色标签
8. **教育特色**：有独立Discovery(紀錄教育)分类和eBook(電子書)分类
9. **YouTube链接字段**：有独立custom_fields用于YouTube链接
10. **双mode系统**：mode=4种子区 / mode=9特别区，字段按mode区分
11. **繁体中文**：分类名称使用繁体中文
