# 熊猫 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 熊猫 |
| 域名 | pandapt.net |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（部分用户可直接发布） |
| MediaInfo | 是 |
| TMDB | 是 |
| IMDb | 是 |
| 豆瓣 | 是（PT-Gen获取简介） |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| 认领系统 | 是（新种1天后可认领，3倍魔力） |
| H&R | 手动模式（发布者自行决定是否勾选） |

## 上传表单

**提交地址**: `takeupload.php`（POST multipart/form-data）

**双分类选择器**: 种子区（`browsecat`，data-mode=4）+ 特别区（`specialcat`，data-mode=5）

**字段后缀**: `[4]`（种子区）/ `[5]`（特别区），由 `data-mode` 属性联动切换

### 种子区字段（data-mode=4）

| 字段 | name | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| 种子文件 | `file` | file | 是 | |
| 标题 | `name` | text | 否 | 不填使用种子文件名 |
| 副标题 | `small_descr` | text | 否 | 自定义标签用[]包裹 |
| TMDB链接 | `tmdb` | text | 否 | data-pt-gen="tmdb" |
| IMDb链接 | `url` | text | 否 | data-pt-gen="url" |
| 豆瓣ID/链接 | `pt_gen` | text | 否 | data-pt-gen="pt_gen"，有获取简介按钮 |
| NFO文件 | `nfo` | file | 否 | |
| 简介 | `descr` | textarea | 是 | BBCode |
| MediaInfo | `technical_info` | textarea | 否 | |
| 类型 | `type` | select | 是 | 双 select 联动（browsecat + specialcat） |
| 媒介 | `medium_sel[4]` | select | 否 | |
| 分辨率 | `standard_sel[4]` | select | 否 | 仅种子区 |
| 编码 | `codec_sel[4]` | select | 否 | |
| 音频编码 | `audiocodec_sel[4]` | select | 否 | |
| 地区 | `source_sel[4]` | select | 否 | |
| 制作组 | `team_sel[4]` | select | 否 | |
| 标签 | `tags[4][]` | checkbox | 否 | 多选 |
| 匿名发布 | `uplver` | checkbox | 否 | |

> **注意**：
> - 有独立 `tmdb` 字段（非标准 NexusPHP 字段）
> - `pt_gen` 字段带自动获取简介按钮（data-pt-gen="pt_gen"）
> - 种子区/特别区通过 `browsecat`/`specialcat` 双 select 联动，切换时 `data-mode` 变化导致所有质量字段后缀改变

## Tracker URL
`https://tracker.pandapt.net/announce.php`

## 站点特色

- **站免池**：站点有站免池系统，当前进度77.2%
- **阅听区**：独立特别区（special.php），包含电子书/有声书/MV/音乐
- **复活区**：独立复活区（rescue.php）
- **双分类系统**：种子区（影视类）+ 特别区（音乐/书籍类），表单字段按mode切换
- **官方工作组导航**：顶部菜单"官方"按媒介类型筛选（DIY/Encode/REMUX/WEB-DL/HDTV/Upscale）
- **无AV1编码**：编码选项中无AV1
- **无单独分辨率字段**：分辨率使用standard_sel标准字段

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 否 | 若不填使用种子文件名，要求规范填写 |
| 副标题 | `small_descr` | 否 | 自定义标签用[]包裹 |
| TMDB链接 | `tmdb` | 否 | data-pt-gen="tmdb" |
| IMDb链接 | `url` | 否 | data-pt-gen="url" |
| 豆瓣ID/链接 | `pt_gen` | 否 | data-pt-gen="pt_gen"，有获取简介按钮 |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| MediaInfo | `technical_info` | 否 | MediaInfo文本 |
| 类型 | `type` | 是 | 双select（种子区browsecat + 特别区specialcat） |
| 媒介 | `medium_sel[mode]` | 否 | mode=4种子区 / mode=5特别区 |
| 分辨率 | `standard_sel[4]` | 否 | 仅种子区 |
| 编码 | `codec_sel[mode]` | 否 | mode=4种子区 / mode=5特别区 |
| 音频编码 | `audiocodec_sel[mode]` | 否 | mode=4种子区 / mode=5特别区 |
| 地区 | `source_sel[mode]` | 否 | mode=4种子区 / mode=5特别区 |
| 制作组 | `team_sel[mode]` | 否 | mode=4种子区 / mode=5特别区 |
| 文件格式 | `processing_sel[5]` | 否 | 仅特别区 |
| 标签 | `tags[mode][]` | 否 | checkbox多选，mode=4种子区 / mode=5特别区 |
| 候选发布 | `approval` | 否 | 勾选则发布到候选区 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

### 种子区 (browsecat, data-mode=4)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 电视剧 |
| 415 | 短剧 |
| 405 | 动漫 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 407 | 体育 |
| 412 | 软件 |
| 411 | 游戏 |
| 413 | 演唱会/音乐会 |
| 409 | 其他 |

### 特别区 (specialcat, data-mode=5)

| ID | 名称 |
|----|------|
| 410 | 电子书 |
| 414 | 有声书 |
| 406 | MV (音乐短片) |
| 408 | 音乐 |

## 质量字段

### 媒介 medium_sel

#### 种子区 medium_sel[4]

| ID | 名称 |
|----|------|
| 11 | UHD Blu-ray |
| 1 | Blu-ray |
| 3 | Remux |
| 7 | Encode |
| 9 | Track |
| 10 | WEB-DL |
| 8 | CD |
| 6 | DVDR |
| 5 | HDTV |
| 12 | UHDTV |
| 4 | MiniBD |
| 2 | HD DVD |

#### 特别区 medium_sel[5]

| ID | 名称 |
|----|------|
| 9 | Track |
| 10 | WEB-DL |
| 8 | CD |
| 6 | DVDR |

### 分辨率 standard_sel[4]（仅种子区）

| ID | 名称 |
|----|------|
| 6 | 4320k/8K |
| 5 | 2160p/4K |
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 7 | SD |

### 编码 codec_sel

#### 种子区 codec_sel[4]

| ID | 名称 |
|----|------|
| 6 | HEVC/H.265/x265 |
| 1 | AVC/H.264/x264 |
| 7 | VP8/VP9 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |

#### 特别区 codec_sel[5]

| ID | 名称 |
|----|------|
| 6 | HEVC/H.265/x265 |
| 1 | AVC/H.264/x264 |

### 文件格式 processing_sel[5]（仅特别区）

| ID | 名称 |
|----|------|
| 1 | EPUB/MOBI/PDF/AZW |
| 2 | EPUB |
| 3 | MOBI |
| 4 | PDF |
| 5 | Other |

### 音频编码 audiocodec_sel

#### 种子区 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 19 | AV3A |
| 8 | TrueHD Atmos |
| 1 | TrueHD |
| 2 | DTS-X |
| 3 | DTS-HD |
| 18 | DTS-HR |
| 4 | DTS |
| 5 | DD/AC3 |
| 6 | DDP/EAC3 |
| 7 | AAC |
| 11 | FLAC |
| 9 | LPCM/PCM |
| 16 | Other |

#### 特别区 audiocodec_sel[5]

| ID | 名称 |
|----|------|
| 7 | AAC |
| 11 | FLAC |
| 10 | MP3 |
| 12 | APE |
| 14 | M4A |
| 15 | WAV |
| 16 | Other |

### 地区 source_sel

#### 种子区 source_sel[4]

| ID | 名称 |
|----|------|
| 7 | CHN(中国大陆) |
| 1 | EU/US(欧美) |
| 2 | HK/MAC/TW(港澳台地区) |
| 3 | JPN(日本) |
| 4 | KOR(韩国) |
| 9 | IND(印度) |
| 8 | SEA(东南亚) |
| 6 | Other(其他) |

#### 特别区 source_sel[5]

与种子区相同。

### 制作组 team_sel

#### 种子区 team_sel[4]

| ID | 名称 |
|----|------|
| 6 | Panda(原盘diy组) |
| 10 | Panda(原盘Remux组) |
| 1 | Panda(压制组) |
| 7 | AilMWeb(流媒体组) |
| 8 | AilMTV(电视录制组) |
| 14 | AilMUpscale(超分视频组) |
| 15 | CatEDU |
| 22 | Red Leaves (红叶) |
| 5 | Other |

#### 特别区 team_sel[5]

| ID | 名称 |
|----|------|
| 25 | Panda(原盘diy组) |
| 28 | Panda(原盘Remux组) |
| 23 | Panda(压制组) |
| 26 | AilMWeb(流媒体组) |
| 27 | AilMTV(电视录制组) |
| 29 | AilMUpscale(超分视频组) |
| 30 | CatEDU |
| 21 | Red Leaves (红叶) |
| 24 | Other |

> **注意**：种子区和特别区的制作组名称相同但ID不同，转载时需根据mode选择正确的ID。

## 标签 tags[mode][]

### 种子区 tags[4][]

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 20 | 驻站 |
| 16 | 纯净版 |
| 4 | DIY |
| 10 | 完结 |
| 17 | 分集 |
| 5 | 国语 |
| 13 | 粤语 |
| 6 | 中字 |
| 12 | 特效 |
| 8 | 杜比视界 |
| 9 | HDR10+ |
| 7 | HDR10 |
| 15 | 菁彩HDR |
| 22 | HLG |
| 2 | 横屏HOR |
| 23 | 竖屏VER |
| 11 | 应求 |
| 14 | 自购 |

### 特别区 tags[5][]

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 20 | 驻站 |
| 22 | HLG |
| 2 | 横屏HOR |
| 23 | 竖屏VER |
| 11 | 应求 |
| 18 | 漫画 |
| 19 | 网络小说 |
| 21 | ASMR |
| 14 | 自购 |

## 标题命名规范

### 电影类
- **主标题**：英文名 年份 分辨率 介质 视频编码 音频编码-制作组
- **副标题**：中文名 导演 演员 音轨/字幕信息 其他
- **示例**：`The Northman 2022 1080p Blu-ray AVC TrueHD 7.1 Atoms-Panda`
- **副标题示例**：`北欧人/北方人(台)/北族人(港)| 导演：罗伯特·艾格斯 | 主演：亚历山大·斯卡斯加德/妮可·基德曼/克拉斯·邦等领衔 | 类别：剧情 | 音频:英语 | 字幕:英/简英/繁英/简/繁`

### 剧集类
- **主标题**：英文名 年代 季数/集数 分辨率 介质 视频编码 音频编码-制作组
- **副标题**：中文名 季数/集数 演员 音轨/字幕信息 其他
- **示例**：`Playing Go 2025 S01 Complete 2160p 60fps WEB-DL H.265 HDR Vivid DDP5.1 Atmos-AilMWeb`

### 音乐类
- **主标题**：艺术家英文名 - 专辑名 发行年份 - 文件格式 采样位深 采样频率 - 制作组
- **副标题**：艺术家本国名 - 专辑本国名 - 年份 | 其他可选信息
- **示例**：`Aaron Kwok - True Legend 101 2013 - FLAC 16bit 44.1kHz - Panda`

### 命名规则要点
- 主标题不得包含中文（制作小组/站点名称特殊除外）
- 主标题各词之间用空格隔开，音轨5.1/编码H.265的点号必须保留
- 副标题必须以中文名起始
- 副标题中需标注剧集季/集信息
- 副标题中导演/演员等至多展示三名
- 剧集分集：S01EXX格式 / 连续分集：S01EXX-XX格式 / 完结：只标季数

## 发布规则

### 简介格式（影视类）
必须包含且排序为：海报 → 简介 → info信息 → 截图
- IMDB及豆瓣链接必须至少填写一个
- 简介可使用PT-Gen一键获取
- 原盘类资源请使用BDinfo
- 压制类/WEB类请使用MediaInfo

### 转载规则
- 转载保持原内容不变，不得随意修改他站官组作品的文件名及文件结构
- 不接受他站分集资源（2025年5月1日起不再接受非官组分集资源发布）
- WEB类资源不再接受未知来源作品
- 未知来源必须添加NoGroup作为后缀（不含原盘资源）
- 自制资源必须添加自己ID作为后缀，并在简介中注明为原创
- 自有原盘必须添加含有光盘及手写ID纸条的图片作为证明

### 禁止内容
- 禁止DHT网络
- 禁止院线在映作品
- 禁止压缩包形式上传
- 禁止"滚雪球"形式发布
- 禁止单种限速超过100MB/S

### 黑名单制作组
GPTHD、SeeWeb、DreamHD、BlackTV、Xiaomi、Huawei、MOMOHD、DDHDTV、Nukehd、TagWeb、CTRLHD、SonyHD、MiniHD、BestTV、BitsTV、ALT、RARBG、mp4ba、FGT、Hao4K、BATWEB、PandaMoon、LelveTV、ColorTV

2025年6月20日追加：FRDS小组的电影合集及剧集合季资源

### H&R规则
- 模式：手动（发布者自行决定是否勾选）
- 考核时间：下载完成100%后168小时
- 达标要求：做种48小时 或 分享率≥1
- 惩罚：H&R数量达到10个时封禁

### 认领规则
- 新种发布1天后才可认领
- 每种最多20人认领，每人最多5000种
- 达标标准：做种≥300小时/月 或 上传量≥种子体积2倍
- 达标认领种获得3倍魔力
- 未达标扣除100魔力/种

## 转载注意事项

1. **双mode字段映射**：种子区(mode=4)和特别区(mode=5)的字段名相同但使用`[mode]`后缀区分，制作组ID在两个mode下不同
2. **TMDB优先**：有独立TMDB链接字段，建议填写TMDB
3. **地区8个选项**：包含印度和东南亚，比标准NexusPHP多
4. **标签mode区分**：种子区19个标签，特别区10个标签，部分重叠
5. **无AV1编码**：编码选项中无AV1，AV1资源只能选Other
6. **短剧独立分类**：有独立短剧分类(ID=415)
7. **横竖屏标签**：有横屏HOR/竖屏VER标签，适合短视频/短剧资源
8. **黑名单制作组多**：25+个黑名单组，转载前需检查
9. **候选区修改时限**：收到站务要求修改后7天内完成，否则删种扣200魔力
10. **简介排序严格要求**：海报-简介-info-截图的固定顺序
