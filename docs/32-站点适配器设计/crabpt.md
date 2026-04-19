# 蟹黄堡 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 蟹黄堡 |
| 域名 | crabpt.vip |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（种审制，非影视/音乐/游戏需 Elite User） |
| MediaInfo | 是（technical_info） |
| IMDb | 是（url） |
| 豆瓣 | 是（简介中要求包含） |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| PT-Gen | 是（简介要求使用 PT-Gen 获取） |
| Wiki | 是（独立 wiki 站点 wiki.crabpt.vip） |

## Tracker URL
`https://crabpt.vip/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | 0DAY 规范，强制英文 |
| 副标题 | `small_descr` | 是 | 不能为空 |
| IMDb链接 | `url` | 否 | 电影电视剧要求必填 |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode，要求 PT-Gen 生成 |
| MediaInfo | `technical_info` | 否 | 电影资源必须 |
| 类型 | `type` | 是 | |
| 来源媒介 | `source_sel[4]` | 否 | 替代 medium_sel |
| 编码 | `codec_sel[4]` / `codec_sel[6]` | 否 | mode 6 为文档编码 |
| 分辨率 | `standard_sel[4]` | 否 | |
| 音频编码 | `audiocodec_sel[4]` / `audiocodec_sel[6]` | 否 | 双 mode |
| 制作组 | `team_sel[4]` / `team_sel[6]` | 否 | 双 mode |
| 地区 | `processing_sel[4]` / `processing_sel[6]` | 否 | 双 mode |
| 标签 | `tags[4][]` / `tags[6][]` | 否 | checkbox 多选，双 mode |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 410 | 电子书 / Ebook |
| 411 | 有声书 / Audiobook |
| 412 | 学习 / Study |
| 414 | 游戏 / Game |
| 415 | 漫画 / Cartoon |

> **注意**：影视/音乐等分类通过种子浏览区筛选实现，不在 type 下拉中。upload.php 仅显示非影视分类。

## 质量字段

### 来源媒介 source_sel[4]（替代 medium_sel）

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | BluRay |
| 3 | UHD Blu-ray |
| 4 | Remux |
| 5 | Encode |
| 6 | WEB-DL |
| 7 | HDTV |
| 8 | CD |
| 9 | MVC |
| 10 | ProRes |
| 11 | Xvid |

### 编码 codec_sel[4]（影视）

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | AVC/H.264/x264 |
| 3 | HEVC/H.265/x265 |
| 4 | H.266/VVC |
| 5 | VP9 |
| 6 | AV1 |
| 14 | VC-1 |
| 15 | MPEG |

### 编码 codec_sel[6]（文档）

| ID | 名称 |
|----|------|
| 1 | Other |
| 7 | TXT |
| 8 | EPUB |
| 9 | AZW3 |
| 10 | MOBI |
| 11 | PDF |
| 12 | ZIP |
| 13 | EPUB/ZAW3/MOBI |

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | 720p |
| 3 | 1080p/1080i |
| 4 | 4K/2160p |
| 5 | 8K |

### 音频编码 audiocodec_sel[4]（影视，21 个）

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | AAC |
| 3 | DD/AC3 |
| 4 | DDP/E-AC3 |
| 5 | DTS |
| 6 | TrueHD |
| 7 | LPCM |
| 8 | DTS:X |
| 9 | MPEG |
| 10 | FLAC |
| 11 | WAV |
| 12 | APE |
| 15 | DTS-HD |
| 16 | ALAC |
| 17 | DTS-HD MA |
| 21 | OGG |
| 22 | DTS-HD |
| 23 | DSD |
| 24 | Opus |
| 26 | Atmos |

### 音频编码 audiocodec_sel[6]（有声书，9 个）

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | AAC |
| 14 | OGG |
| 13 | M4A |
| 15 | DTS-HD |
| 17 | DTS-HD MA |
| 23 | DSD |
| 26 | Atmos |

### 地区 processing_sel[4] / processing_sel[6]（8 个）

| ID | 名称 |
|----|------|
| 1 | 其他（Other） |
| 2 | 中国大陆（CN） |
| 3 | 港台（HK/TW） |
| 4 | 欧美（EU/US） |
| 5 | 日本（JP） |
| 6 | 韩国（KR） |
| 7 | 印度（India） |

### 制作组 team_sel[4]（37 个） / team_sel[6]（34 个）

| ID | 名称 | 备注 |
|----|------|------|
| 1 | Other | |
| 2 | CHD | |
| 3 | HDS | |
| 4 | WiKi | |
| 5 | OurBits | |
| 6 | XHB | 站组 |
| 7 | FRDS | |
| 8 | HHWEB | |
| 9 | UBits | |
| 10 | Audiences | |
| 11 | DYZ-WEB | |
| 12 | DYZ-Movie | |
| 13 | DYZ-TV | |
| 15 | AGSVWEB | |
| 16 | ZmWeb | |
| 17 | Pter | |
| 18 | FFans | |
| 19 | UBWEB | |
| 20 | HDFans | |
| 21 | PigoHD | |
| 22 | PigoWeb | |
| 23 | PiGoNF | |
| 24 | QHstudIo | |
| 25 | ADWeb | |
| 26 | WiKi | 重复 ID 4 |
| 27 | CMCT | |
| 28 | FRDS | 重复 ID 7 |
| 29 | FROG | |
| 30 | FROGWeb | |
| 31 | ZmWeb | 重复 ID 16 |
| 32 | tlf | |
| 33 | beAst | |
| 34 | QHstudIo | 重复 ID 24 |
| 35 | HDHome | |
| 36 | HDVWEB | |
| 37 | MTeam | |
| 38 | 红叶 | mode 6 独有 |

## 标签

### tags[4][]（37 个，影视 mode）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 自购 |
| 4 | 原盘DIY |
| 5 | 国语 |
| 6 | 粤语 |
| 7 | 中字 |
| 8 | 完结 |
| 9 | 分集 |
| 10 | 特效字幕 |
| 11 | HDR |
| 12 | Dolby Vision |
| 13 | Dolby Atmos |
| 19 | 合集大包 |
| 20 | 伦理 |
| 22 | 驻站 |
| 23 | Audiences |
| 24 | AGSV |
| 25 | 短剧 |
| 32 | 剧情 |
| 33 | 喜剧 |
| 34 | 动作 |
| 35 | 爱情 |
| 36 | 科幻 |
| 37 | 动画 |
| 38 | 悬疑 |
| 39 | 惊悚 |
| 40 | 恐怖 |
| 41 | 纪录片 |
| 42 | 历史 |
| 43 | 战争 |
| 44 | 犯罪 |
| 45 | 奇幻 |
| 46 | 冒险 |
| 47 | 灾难 |
| 48 | 武侠 |
| 50 | ASMR |
| 59 | 未完结 |

### tags[6][]（34 个，书籍/有声书 mode）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 自购 |
| 7 | 中字 |
| 8 | 完结 |
| 9 | 分集 |
| 14 | 历史架空 |
| 15 | 网文 |
| 16 | 财经 |
| 17 | 知识普及 |
| 18 | 军事 |
| 19 | 合集大包 |
| 21 | 条漫 |
| 26 | 奇幻玄幻 |
| 27 | 游戏竞技 |
| 28 | 科幻末日 |
| 29 | 武侠仙侠 |
| 30 | 灵异悬疑 |
| 31 | 都市异能 |
| 36 | 科幻 |
| 37 | 动画 |
| 44 | 犯罪 |
| 47 | 灾难 |
| 51 | 传统戏曲 |
| 52 | 电台节目 |
| 53 | 历史军事 |
| 54 | 轻小说 |
| 55 | 外语有声 |
| 56 | 文学出版 |
| 57 | 相声评书 |
| 58 | 言情小说 |
| 59 | 未完结 |
| 60 | 传记名著 |
| 61 | 穿越重生 |
| 62 | 无限流 |

## 缺失字段

- **无 medium_sel**（使用 source_sel 替代）
- **无豆瓣独立字段**（在简介中要求包含）

## 特殊说明

1. **source_sel 替代 medium_sel**：字段名为 source_sel，值包含 UHD Blu-ray(3)/MVC(9)/ProRes(10)/Xvid(11) 等
2. **双 mode 质量字段体系**：mode 4（影视）和 mode 6（书籍/有声书），编码/音频/制作组/地区/标签各不相同
3. **mode 6 编码为文档格式**：EPUB/AZW3/MOBI/PDF/TXT/ZIP，非视频编码
4. **mode 6 音频为有声书格式**：AAC/M4A/OGG/DSD 等
5. **processing_sel = 地区**：含中国大陆/港台/欧美/日本/韩国/印度
6. **37 个制作组含重复**：WiKi(4/26)、FRDS(7/28)、ZmWeb(16/31)、QHstudIo(24/34) 出现两次
7. **站组 XHB**：制作组 XHB(6) 为站内组
8. **37 个影视标签含完整类型标签**：动作/喜剧/爱情/科幻/悬疑/惊悚/恐怖/战争/犯罪/奇幻/冒险/灾难/武侠 等
9. **34 个书籍标签含网文分类**：穿越重生/无限流/都市异能/武侠仙侠/奇幻玄幻/灵异悬疑 等
10. **种审制**：所有种子需审核，非影视/音乐/游戏资源需 Elite User 等级
11. **禁止 9KG**：明确禁止 NC-17/III级/R18/18分级及露点资源
12. **标题强制 0DAY 英文规范**：主标题不使用中文，无英文名使用拼音
13. **PT-Gen + MediaInfo 必填**：电影电视剧简介要求使用 PT-Gen，电影要求 MediaInfo
14. **已完结剧集必须合集发布**：限制单集发布
15. **音频标记规范**：EAC3=DDP，AC3=DD，多音轨标注 2Audios/3Audios
16. **编码含 H.266/VVC**：支持 VVC(4)
17. **独立 Wiki 站点**：wiki.crabpt.vip 提供详细发布规则和标题命名规范
18. **附加服务**：EMBY/MC 服务器/在线影视/图床/音乐等
