# 星空 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 星空 |
| 域名 | star-space.net |
| 框架 | FireFly（自研框架，非 NexusPHP） |
| Cloudflare | 否 |
| 候选制 | 否 |
| MediaInfo | 否（在简介中附带） |
| IMDb | 是（imdb_url） |
| 豆瓣 | 是（douban_url） |
| 匿名发布 | 否 |
| NFO | 是（tr_nfo） |
| PT-Gen | 否 |

## Tracker URL
需从站点获取（页面中未直接显示）

## 架构特点

- **自研 FireFly 框架**，非 NexusPHP
- **双独立发布系统**：视频发布（p_torrent/video_upload.php）和音乐发布（p_music/music_upload.php）完全分离
- **字段命名完全不同于 NexusPHP**：使用 `tr_` 前缀（tr_team/tr_category/tr_source/tr_video_codec 等）
- **分类使用字符串 ID**：如 `mo`/`tv`/`an`/`do`，非数字
- **来源使用层级字符串 ID**：如 `s41`=BD Encode / `s42`=BD Remux / `s43`=BD DIY / `s44`=BD ISO

## 视频发布页面 (video_upload.php)

### 字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子ID | `tid` | 隐藏 | 值=0 为新发布 |
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_desc` | 否 | |
| 豆瓣链接 | `douban_url` | 否 | 要求必须有豆瓣或IMDb |
| IMDb链接 | `imdb_url` | 否 | |
| 简介 | `descr` | 是 | BBCode（KindEditor） |
| NFO | `tr_nfo` | 否 | |
| 截图 | `screen` | 否 | textarea |
| 封面URL | `tr_cover_url` | 否 | |
| 分类 | `tr_category` | 是 | |
| 来源 | `tr_source` | 是 | |
| 视频编码 | `tr_video_codec` | 否 | |
| 音频编码 | `tr_audio_codec` | 否 | |
| 分辨率 | `tr_resolution` | 否 | |
| HDR | `tr_hdr` | 否 | |
| 制作组 | `tr_team` | 否 | |
| 标签 | tag_* 复选框 | 否 | 独立命名字段 |

### 分类 tr_category（字符串 ID）

| ID | 名称 |
|----|------|
| mo | 电影 |
| tv | 剧集 |
| an | 动画 |
| do | 纪录片 |
| mv | MV |
| sp | 体育 |
| ot | 综艺 |

### 来源媒介 tr_source（层级字符串 ID，15 个）

| ID | 名称 |
|----|------|
| s11 | Other |
| s13 | Web-DL |
| s21 | HDTV Encode |
| s22 | HDTV |
| s31 | DVD Encode |
| s32 | DVD Remux |
| s33 | DVD ISO |
| s41 | BD Encode |
| s42 | BD Remux |
| s43 | BD DIY |
| s44 | BD ISO |
| s51 | UHD Encode |
| s52 | UHD Remux |
| s53 | UHD DIY |
| s54 | UHD ISO |

### 视频编码 tr_video_codec

| ID | 名称 |
|----|------|
| 1 | H264 (AVC) |
| 2 | H265 (HEVC) |
| 3 | MPEG |
| 4 | Other |
| 5 | VC-1 |
| 6 | x264 |
| 7 | x265 |
| 8 | Xvid |

### 音频编码 tr_audio_codec（16 个）

| ID | 名称 |
|----|------|
| 1 | AAC |
| 2 | APE |
| 3 | DD/DD+/AC3 |
| 4 | DTS |
| 5 | DTS-HD HR |
| 6 | DTS-HD MA |
| 7 | DTS-X |
| 8 | FLAC |
| 9 | LPCM |
| 10 | M4A |
| 11 | MP3 |
| 12 | OGG |
| 13 | Other |
| 14 | TrueHD |
| 15 | TrueHD Atmos |
| 16 | WAV |

### 分辨率 tr_resolution

| ID | 名称 |
|----|------|
| r1 | SD |
| r2 | 720 |
| r3 | 1080 |
| r4 | 2160 |
| r5 | 4320 |

### HDR tr_hdr

| ID | 名称 |
|----|------|
| h1 | DV |
| h2 | HDR |
| h3 | HDR+DV |

### 制作组 tr_team

| ID | 名称 |
|----|------|
| 1 | Ying |
| 2 | YingWEB |
| 3 | YingDIY |
| 4 | YingMV |
| 5 | Other |
| 6 | YingHDTV |
| 7 | YingMUSIC |
| 8 | CatEDU |
| 9 | Telesto |

### 标签（独立 checkbox 字段）

| name | 名称 |
|------|------|
| tag_gf | 官方 |
| tag_xiaozu | 驻站组 |
| tag_jz | 禁转 |
| tag_3d | 3D |
| tag_chs_sub | 中字 |
| tag_chs_lang | 国语 |
| tag_yueyu | 粤语 |
| tag_eng_sub | 英字 |
| tag_eng_lang | 英语 |
| tag_ep | 分集 |
| tag_complete | 完结 |

## 音乐发布页面 (music_upload.php)

### 字段（Gazelle 风格）

| 字段 | name | 说明 |
|------|------|------|
| 种子ID | `tid` | 隐藏 |
| 种子文件 | `file` | |
| 艺术家 | `artist` | |
| 标题 | `title` | |
| 年份 | `year` | |
| 重制年份 | `remaster_year` | |
| 重制标题 | `remaster_title` | |
| 厂牌 | `remaster_record_label` | |
| 目录号 | `remaster_catalogue_number` | |
| 音乐标签 | `musicTagIds` | 隐藏字段 |
| 封面 | `image` | URL |
| 发行类型 | `release_type` | |
| 格式 | `format` | |
| 比特率 | `bitrate` | |
| 媒介 | `media` | |

### 发行类型 release_type

| ID | 名称 |
|----|------|
| 1 | 专辑 |
| 3 | 电影游戏原声 |
| 5 | EP/迷你专辑/单曲 |
| 11 | 演唱会歌剧录音 |
| 21 | 其它 |

### 格式 format

| 值 | 名称 |
|----|------|
| FLAC | FLAC |
| DTS | DTS |
| DFF | DFF |
| DSF | DSF |
| DST | DST |
| Other | Other |

### 比特率 bitrate

| 值 | 名称 |
|----|------|
| LL | Lossless |
| 24BL | 24bit Lossless |
| DSD64 | DSD64 |
| DSD128 | DSD128 |
| DSD256 | DSD256 |
| DSD512 | DSD512 |
| Other | Other |

### 媒介 media

| 值 | 名称 |
|----|------|
| CD | CD |
| DVD | DVD |
| Vinyl | Vinyl |
| SB | Soundboard |
| SACD | SACD |
| DAT | DAT |
| CASS | Cassette |
| WEB | WEB |
| BD | Blu-Ray |

## 缺失字段

- **无 source_sel**（视频使用 tr_source 替代）
- **无 processing_sel**
- **无独立 MediaInfo 字段**
- **无匿名发布**
- **无 PT-Gen**

## 特殊说明

1. **FireFly 自研框架**：完全不同于 NexusPHP，字段命名、URL 路径、表单结构均独立
2. **双独立发布系统**：视频和音乐发布页面完全分离，URL 路径不同
3. **视频来源层级编码**：tr_source 使用 s11-s54 编码，按 Encode/Remux/DIY/ISO × HDTV/DVD/BD/UHD 组织
4. **分类使用字符串 ID**：mo/tv/an/do/mv/sp/ot，非数字
5. **分辨率使用 r1-r5 编码**，HDR 使用 h1-h3 编码
6. **标签使用独立命名字段**：tag_gf/tag_xiaozu/tag_jz 等，非 tags[] 数组
7. **HDR 独立下拉框**：DV/HDR/HDR+DV 三选项，非标签
8. **制作组以站组为主**：Ying/YingWEB/YingDIY/YingMV/YingHDTV/YingMUSIC + CatEDU/Telesto
9. **音乐系统为 Gazelle 风格**：艺术家/标题/年份/厂牌/目录号/发行类型/格式/比特率/媒介
10. **音乐支持 DSD**：DSD64/DSD128/DSD256/DSD512
11. **禁止发布 DIY 和 Remux 资源**（视频）
12. **压制仅接受 WiKi/CMCT**：其他小组压制作品不允许
13. **分集仅驻站组和官方组可发布**
14. **豆瓣或 IMDb 必填**
15. **蓝光原盘替换规则严格**：BD100 > BD66 > BD50 > BD25，带中字/国语可替换
16. **重复判定**：分辨率 + 媒介 + 视频编码 + HDR 组合唯一
17. **禁止羊毛盒子**
