# 壹吧 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 壹吧 |
| 域名 | 1ptba.com |
| 框架 | NexusPHP |
| Cloudflare | 是 |
| 建站时间 | 2019 年 |
| 后端存储 | Redis |
| 表单提交 | `takeupload.php`，无 CSRF Token |
| 候选制 | 是 |
| IMDb | 是（url字段，支持 PT-Gen 一键获取） |
| 豆瓣 | 否（通过 PT-Gen 间接支持） |
| 匿名发布 | 是（uplver，默认不勾选） |
| NFO | 是 |
| MediaInfo/BDInfo | 是（technical_info 独立字段） |
| YouTube链接 | 是（custom_fields[mode][1]） |
| 认领系统 | 是（0/1000） |
| H&R | 是（5天/24h，≥8次不达标封禁，20000魔力消除） |
| Telegram | https://t.me/ptbar_Chat |

## Tracker URL
`https://1ptba.com/announce.php`

## 站点特色

- **教育特色**：站点描述为"教育视频,课件资源,发布教育类,学习类,纪录片等资源"
- **双分类系统**：种子区（影视/教育类）+ 特别区（9KG/成人内容）
- **特别区禁止转载发布**：只允许发布至种子区（data-mode=4），禁止发布至特别区（data-mode=9）
- **种子最小1GB**：种子小于1GB的资源会被删除
- **source_sel非标准语义**：source_sel用作媒介来源（含原盘/REMUX/Encode/WEB-DL），同时medium_sel也独立存在
- **codec_sel混合音视频**：编码下拉混合了视频编码（H.264/HEVC/VC-1等）和音频编码（FLAC/APE/DTS/AC-3/WAV/MP3/ALAC/AAC）
- **processing_sel极简**：仅Raw/Encode两项
- **8K分辨率**：standard_sel含8K-UHD
- **音乐标签丰富**：含音乐专辑/MV/卡拉OK/LIVE现场/演唱会标签
- **规则与学校/阳光同模板**：rules.php 的上传/Dupe/打包规则完全一致
- **促销规则有差异**：论坛帖（topicid=4726）的促销规则与 rules.php 模板不同，论坛帖更具体
- **H&R规则论坛帖为准**：论坛帖有完整 H&R 规则（5天/24h/≥8次ban/20000魔力），rules.php 未提及
- **发布者双倍上传量**

## ⚠ 特别区禁止规则

特别区（specialcat, data-mode=9）包含16个成人内容分类（AV有码/无码/HD/SD/DVDiSo/Blu-Ray/网站/写真/H-Game/H-Anime/H-Comic/成人电影/Gay等）。

**转载发布规则**：只允许发布至种子区（data-mode=4），**禁止发布至特别区（data-mode=9）**。

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 否 | 规范填写，如 `Blade Runner 1982 Final Cut 720p HDDVD DTS x264-ESiR` |
| 副标题 | `small_descr` | 否 | 如 `银翼杀手 720p @ 4615 kbps - DTS 5.1 @ 1536 kbps` |
| IMDb链接 | `url` | 否 | IMDb URL，支持 PT-Gen 一键获取 |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode，支持 PT-Gen |
| MediaInfo/BDInfo | `technical_info` | 否 | 独立字段，粘贴 MediaInfo/BDInfo 文本 |
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
| 匿名发布 | `uplver` | 否 | 默认不勾选 |

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

### 特别区 (specialcat, data-mode=9) — **禁止发布**

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

> 以上分类全部为成人内容。**转载时禁止选择这些分类。**

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

## 促销规则（论坛帖 topicid=4726 为准）

> rules.php 中的促销规则是通用模板，论坛帖有更详细/更新的数据

### 随机促销（种子上传后系统自动随机设置）

| 概率 | 促销类型 |
|------|---------|
| 50% | 50%下载 |
| 20% | 免费(Free) |
| 25% | 50%下载 & 2x上传 |
| 5% | 免费 & 2x上传 |

> 与 rules.php 模板的6档概率不同，论坛帖只有4档且概率分配差异巨大

### 自动促销规则

| 条件 | 促销类型 |
|------|---------|
| 文件总体积 > **30GB** | 自动免费 |
| Blu-ray Disk / HD DVD 原盘 | 自动免费 & 2x上传 |
| 电视剧等每季第一集 | 自动免费 & 2x上传 |

> 注意：>30GB（非模板的20GB），BR/HD DVD原盘和每季首集为"免费&2x上传"（非模板的仅"免费"）

### 促销时限（论坛帖）

| 促销类型 | 时限 |
|---------|------|
| 免费 & 2x上传 | 3天后变为"50%下载" |
| 免费(Free) | 3天后变为"50%下载" |
| 50%下载 & 2x上传 | 3天后变为"30%下载" |
| 50%下载 | **0天**（立即变为"普通"） |
| 30%下载 | 无时限 |
| 2x上传 | 3天后变为"50%下载" |
| 所有种子发布**2个月**后 | 自动永久成为"2x上传" |

> 与 rules.php 模板（7天时限/1个月永久2X）不同：时限3天、2个月永久2X

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

### 游戏类
- `[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]`
- 例：`红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED`

### 副标题规则
- 不要包含广告或求种/续种请求

### 外部信息
- 电影和电视剧**必须**包含外部信息链接（如IMDb链接）

### 简介要求
- **电影/电视剧/动漫**：必须包含海报/横幅/封面；尽可能包含截图、文件详情、演职员及剧情概要
- **体育节目**：请勿泄漏比赛结果
- **音乐**：必须包含专辑封面和曲目列表
- **PC游戏**：必须包含海报/封面；尽可能包含截图
- NFO图请写入NFO文件，不要粘贴到简介里

## 发布规则

### 重要规则
- **种子最小1GB**：小于1GB的资源会被删除
- 总体积小于100MB的资源禁止
- 禁止RAR等压缩文件
- 禁止色情/敏感政治内容

### Dupe规则（rules.php 通用模板）
- 来源媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 动漫类HDTV与DVD同优先级（特例）
- 断种45日或发布18个月以上可重发
- 官方组发布后同版本视为Dupe

### 资源打包规则（试行）
- 与学校/阳光站完全一致
- 允许打包：电影合集/整季剧/专题纪录片/7日内预告片/同艺术家MV/同艺术家音乐(≥5张)/动漫分卷/发布组打包
- 打包约束：相同媒介/分辨率/编码，电影合集须发布组统一

### H&R规则（论坛帖 topicid=4726）
- 下载完成后**5天内**累计做种**24小时**
- **或者**分享率达到**1.8**
- Power User 及以上用户可发布带 H&R 的种子
- VIP 及以上不受 H&R 限制
- 累计 **≥8个** H&R 不达标 → 禁用账号
- 种子发布后**30天**不再参与 H&R 检查（取消前的记录继续检查）
- 免除 H&R：**20000魔力**/种
- 产生不达标**不再短消息通知**，须主动用魔力值消除，否则影响账户停用

### 认领规则
- 每人最多认领1000个种子

### 账号保留
- Veteran User及以上：永久保留
- Elite User及以上封存后：不删除
- 封存账号：400天未登录删除
- 未封存账号：150天未登录删除
- 无流量用户：100天未登录删除

### 管理组退休待遇
- **上传员 → 养老族**：升职1年以上 + 上传≥200种子
- **上传员 → VIP**：升职6个月以上 + 上传≥100种子
- **管理员 → 养老族**：升职1年以上 + 参加≥2次站务会议 + 参与规则/答疑修订
- **管理员 → VIP**：无条件
- **总管理员及以上**：直接成为养老族

### 字幕区规则
- 允许格式：srt / ssa / ass / cue / zip / rar
- Vobsub格式须打包为 zip/rar
- 禁止上传 lrc 歌词等非字幕/cue 文件
- 不合格字幕直接删除，上传者扣100魔力
- 举报不合格字幕奖励50魔力

## 转载注意事项

1. **source_sel非标准**：用作媒介来源而非地区，转载时需注意映射
2. **codec_sel混合音视频**：编码下拉同时包含视频编码和音频编码，需根据资源类型选择
3. **种子最小1GB**：小于1GB的资源会被删除，转载时需检查资源大小
4. **制作组仅6个**：HDS/CHD/MySiLU/WiKi/Other/1PTBA，大部分资源需选Other
5. **分辨率含8K**：standard_sel包含8K-UHD
6. **音乐标签丰富**：有音乐专辑/MV/卡拉OK/LIVE/演唱会等特色标签
7. **教育特色**：有独立Discovery(紀錄教育)分类和eBook(電子書)分类
8. **YouTube链接字段**：有独立custom_fields用于YouTube链接
9. **双mode系统**：mode=4种子区 / mode=9特别区，字段按mode区分
10. **繁体中文**：分类名称使用繁体中文
11. **禁止发布至特别区**：转载时 type 只能选 browsecat（data-mode=4），不能选 specialcat（data-mode=9）
12. **有 PT-Gen**：支持一键获取 IMDb 简介，url 字段输入 IMDb 链接后点击"获取简介"
13. **有 MediaInfo 字段**：technical_info 独立字段，粘贴 MediaInfo/BDInfo 文本
14. **促销规则以论坛帖为准**：rules.php 是通用模板，论坛帖 topicid=4726 有更准确的数据
