# 他吹吹风 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 他吹吹风 |
| 域名 | et8.org |
| 站点全称 | TorrentCCF (TCCF) |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（offers.php，Power User 以下需先候选） |
| PT-Gen | 是（推荐使用，多源） |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| IMDb | 是 |

## 上传表单

**提交地址**: `takeupload.php`（POST multipart/form-data）

> **ET8 为教育特色站**（TorrentCCF），所有字段名均为裸名（无 `[N]` 后缀）。`source_sel` 语义为**学科分类**而非来源媒介。分类 ID 为非标准编号（624-635）。

### 基础字段

| 字段名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `file` | file | 是 | 种子文件 |
| `name` | text | 是 | 标题 |
| `small_descr` | text | 否 | 副标题 |
| `url` | text | 否 | IMDb 链接 |
| `nfo` | file | 否 | NFO 文件 |
| `descr` | textarea | 是 | 简介（BBCode） |
| `type` | select | 是 | 分类 |
| `uplver` | checkbox | 否 | 匿名发布 |

### 分类 `type`（9 个，非标准 ID）

| ID | 名称 |
|----|------|
| 624 | 纪录片 |
| 628 | Elearning - 杂项学习 |
| 629 | Elearning - 电子书/小说 |
| 630 | Elearning - 电子书/非小说 |
| 631 | Elearning - 杂志 |
| 632 | Elearning - 漫画 |
| 633 | Elearning - 有声书 |
| 634 | Elearning - 公开课 |
| 635 | Elearning - 视频教程 |

> 9 个分类中 8 个为 Elearning 系列，仅 624 为纪录片。无电影/剧集/综艺等常规影视分类。

### 学科分类 `source_sel`（14 个，语义=学科非来源媒介）

| ID | 名称 |
|----|------|
| 1 | 信息技术 |
| 2 | 自然科学 |
| 3 | 社会科学 |
| 4 | 哲学 |
| 5 | 法律 |
| 6 | 军事政治 |
| 7 | 经济 |
| 8 | 文体教育/少儿教育 |
| 9 | 文体教育/非少儿教育 |
| 10 | 语言文字 |
| 11 | 文学艺术 |
| 12 | 历史地理 |
| 13 | 医学卫生 |
| 14 | 其他 |

### 媒介 `medium_sel`（17 个，含电子书格式）

| ID | 名称 |
|----|------|
| 1 | BluRay |
| 3 | HDRip |
| 4 | DVDRip |
| 5 | Remux |
| 6 | HDTV |
| 7 | DVDR |
| 8 | Other |
| 9 | WEB-DL |
| 10 | UHD Bluray |
| 11 | Encode |
| 12 | PDF |
| 13 | EPUB |
| 14 | AZW3 |
| 15 | MOBI |
| 16 | TXT |
| 17 | Pictures |

### 编码 `codec_sel`

| ID | 名称 |
|----|------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | x265 |
| 7 | x264 |
| 8 | H265/HEVC |

### 音频编码 `audiocodec_sel`

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | AC3 |
| 5 | MP3 |
| 6 | AAC |
| 7 | Other |
| 8 | DTS-HD |
| 9 | TrueHD |
| 10 | LPCM |
| 11 | WAV |

### 分辨率 `standard_sel`（字段名非 resolution_sel）

| ID | 名称 |
|----|------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160/4K |

### 制作组 `team_sel`

| ID | 名称 |
|----|------|
| 1 | TorrentCCF/TCCF |
| 2 | TLF |
| 3 | BMDru |
| 4 | CatEDU |
| 5 | MADFOX |
| 6 | 个人原创 |
| 7 | 其他 |

> ET8 所有质量字段均为裸名（`source_sel`/`medium_sel`/`codec_sel`/`audiocodec_sel`/`standard_sel`/`team_sel`），无 `[N]` 后缀。注意 `source_sel` 语义为学科分类而非来源媒介。

## Tracker URL
`https://t.et8.org/announce.php`

## 站点特色

1. **教育学习特色站**：推荐发布教育类、学习类、纪录片等资源
2. **全站禁止发布影视剧类资源**（含电影、剧集、动画等；管理组认为稀有或有价值的可特许发布）
3. **老牌站点**：站名 TorrentCCF，历史悠久
4. **source_sel 语义重定义**：`source_sel` 不是来源媒介，而是**学科分类**（14 个学科领域）
5. **standard_sel 替代 resolution_sel**：使用 `standard_sel` 字段名表示分辨率/清晰度
6. **medium_sel 含电子书格式**：除传统媒介外，还包含 PDF/EPUB/AZW3/MOBI/TXT/Pictures

## 发布规范摘要

### 允许的资源
- 教育类、学习类、纪录片
- 高清/标清视频文件
- 无损或多声道音乐
- Scene 组资源
- PC 游戏
- 高清相关软件和文档

### 禁止的资源
- **影视剧类资源**（全站禁止，除非稀有或有价值）
- XXX 内容及擦边内容
- **FGT 小组**（黑名单制作组）
- **mp4ba 等有水印的影视作品**
- 政治/体制内容
- CAM、TS、SCR、DVDSRC、R5 低画质
- RAR 分卷压缩（Scene 组影视须解压，其他可直接发）
- 合集优先，有合集不发单集

### 标题命名规范
- 主标题：英文 0day 格式，单词间用 `.` 分隔
- 副标题：中文名及介绍语
- 仅允许首发组使用 `[TorrentCCF首发]`
- 分隔符只用 `.` 和空格，可使用 `/` 转义

### 简介要求
- 推荐使用 PTgen 生成规范介绍
- 必须包含海报/封面
- 视频：必须包含截图或 MediaInfo（二选一）
- 音乐：必须包含 CD 封面和曲目列表
- 电影/电视剧：必须包含豆瓣或 IMDB 外部链接

### 教育/学习类资源特殊要求
- 鼓励英文标题 + 中文副标题
- 定期发布资源须注明时期
- 简介开头必须提供一张图片
- 视频类必须包含 MediaInfo 和截图
- 图文类必须包含至少一张预览图
- 简介正文必须有适当文字描述

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| 类型 | `type` | 是 | |
| 学科分类 | `source_sel` | 否 | **语义=学科分类（非来源媒介）** |
| 媒介 | `medium_sel` | 否 | 含电子书格式 |
| 编码 | `codec_sel` | 否 | |
| 音频编码 | `audiocodec_sel` | 否 | |
| 分辨率 | `standard_sel` | 否 | **字段名=standard_sel（非resolution_sel）** |
| 制作组 | `team_sel` | 否 | |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 624 | Documentaries.纪录片 |
| 628 | Elearning - 杂项学习 |
| 629 | Elearning - 电子书/小说 |
| 630 | Elearning - 电子书/非小说 |
| 631 | Elearning - 杂志 |
| 632 | Elearning - 漫画 |
| 633 | Elearning - 有声书 |
| 634 | Elearning - 公开课 |
| 635 | Elearning - 视频教程 |

## source_sel（学科分类，非来源媒介！）

| ID | 名称 |
|----|------|
| 1 | 信息技术 |
| 2 | 自然科学 |
| 3 | 社会科学 |
| 4 | 哲学 |
| 5 | 法律 |
| 6 | 军事政治 |
| 7 | 经济 |
| 8 | 文体教育/少儿教育 |
| 9 | 文体教育/非少儿教育 |
| 10 | 语言文字 |
| 11 | 文学艺术 |
| 12 | 历史地理 |
| 13 | 医学卫生 |
| 14 | 其他 |

## 媒介 medium_sel

| ID | 名称 |
|----|------|
| 1 | BluRay |
| 3 | HDRip |
| 4 | DVDRip |
| 5 | Remux |
| 6 | HDTV |
| 7 | DVDR |
| 8 | Other |
| 9 | WEB-DL |
| 10 | UHD Bluray |
| 11 | Encode |
| 12 | PDF |
| 13 | EPUB |
| 14 | AZW3 |
| 15 | MOBI |
| 16 | TXT |
| 17 | Pictures |

## 编码 codec_sel

| ID | 名称 |
|----|------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | x265 |
| 7 | x264 |
| 8 | H265/HEVC |

## 音频编码 audiocodec_sel

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | AC3 |
| 5 | MP3 |
| 6 | AAC |
| 7 | Other |
| 8 | DTS-HD |
| 9 | TrueHD |
| 10 | LPCM |
| 11 | WAV |

## 分辨率 standard_sel（字段名非 resolution_sel）

| ID | 名称 |
|----|------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160/4K |

## 制作组 team_sel

| ID | 名称 |
|----|------|
| 1 | TorrentCCF/TCCF |
| 2 | TLF |
| 3 | BMDru |
| 4 | CatEDU |
| 5 | MADFOX |
| 6 | 个人原创资源(Original) |
| 7 | 其他(other) |

## 缺失字段

- **无 resolution_sel**（使用 standard_sel 替代）
- **无 processing_sel**
- **无标签 (tags)**
- **无 PT-Gen 专用字段**（通过 IMDb 链接 + 手动 PTgen 生成）
- **无 MediaInfo 专用字段**

## 黑名单制作组

- **FGT**（严禁发布）
- **mp4ba** 等有水印的影视作品

## 字段语义重定义（重要）

| 字段名 | 标准含义 | 本站含义 |
|--------|---------|---------|
| `source_sel` | 来源媒介 | **学科分类**（14 个学科领域） |
| `standard_sel` | （非标准字段） | **分辨率/清晰度**（替代 resolution_sel） |
| `medium_sel` | 媒介 | 媒介 + **电子书格式**（PDF/EPUB/AZW3/MOBI/TXT/Pictures） |

## 站点规则（来源：rules.php）

### 总则

- 绝对服从管理员安排和判罚
- 管理组保留对规则未列但对站点有危害行为进行处罚的权力
- 一切作弊/贩卖邀请/多账号/共享账号会被封
- 第一次捣乱警告，第二次永久封禁
- **全站禁止发布影视剧类资源**（管理组认为稀有或有价值的可特许）
- 单种最大上传速度 125MB/s，超速自动封禁且原则上不予解封

### 账号保留规则

| 条件 | 处理 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Insane User 及以上封存账号 | 不被删除 |
| 封存账号连续 365 天不登录 | 禁用 |
| 未封存账号连续 90 天不登录 | 禁用 |
| 无流量用户连续 30 天不登录 | 禁用 |

**长期未登录抵消机制**：
- 有邀请名额：每消耗 1 个邀请名额延长 10 天
- 无邀请名额：每消耗 1/10 购买邀请名额的魔力延长 1 天
- 无邀请名额也无足够魔力：账号封禁
- 全自动处理，原则上不人工解封

### 发布者资格

- **Power User 及以上**可直接上传
- Power User 以下需先在候选区提交候选

### 做种要求

- 发布后必须做种至少 24 小时且有至少 3 个做种者后方可撤种
- 发布后 7 天无人下载可自行斟酌是否留种
- 发布者获得双倍上传量

### Dupe 规则

- 当前最佳画质来源经重编码而成的 DVD5 大小（4.3GB）版本永远允许发布
- 新版本允许发布条件（旧版本被视为 Dupe）：
  - 旧版本已连续断种 7 日以上
  - 新版本没有旧版本中已确认的错误/画质问题
  - 旧版本已发布 18 个月以上
- 不同区域/配音/字幕的 Blu-ray/HD DVD 版本不被视为 Dupe

### 促销规则

- 所有种子有一定概率随机促销
- 内容优质的种子（由 MOD 定夺）将成为免费
- 不定期开启全站免费

## 特殊说明

1. **教育特色**：9 个分类中 8 个是 Elearning 系列，1 个纪录片
2. **全站禁影视**：禁止发布影视剧类资源（管理特许除外），适合转发教育/纪录片类
3. **source_sel = 学科**：不是来源媒介，是 14 个学科分类（信息技术/自然科学/社会科学等）
4. **standard_sel = 分辨率**：用 standard_sel 而非 resolution_sel，含 2160/4K
5. **medium_sel 含电子书**：除传统媒介外还含 PDF/EPUB/AZW3/MOBI/TXT/Pictures 共 6 种电子书格式
6. **制作组以教育为主**：CatEDU、BMDru、MADFOX 为教育相关制作组
7. **老牌站**：站名 TorrentCCF (TCCF)，单种最大上传速度限制 125MB/s
8. **发布者 2X 上传**：发布者获得双倍上传量
9. **推荐 PTgen**：推荐使用 PTgen 生成规范介绍，支持多个 PTgen 源
10. **趣味盒已关闭**
11. **封存线为 Insane User**（非 Elite User），且封禁线 365 天（非 400 天）
12. **邀请抵消封禁**：有邀请名额可抵消长期未登录封禁（每邀请=10天）
