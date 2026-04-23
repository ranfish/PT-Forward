# 家园 站点适配器设计

> HDHome 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 家园|
| 站点地址 | https://hdhome.org |
| 备用域名 | https://hdbiger.org |
| 站点框架 | NexusPHP |
| 特殊规则 | 候选制（Crazy User 及以上免候选）、双区域发布（种子区/LIVE 区）、8K 分类、豆瓣 ID 字段 |
| 互斥站点 | **铂金家**（禁止互相转载发布） |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://t.hdhome.org/announce.php` 或 `https://hdbiger.org/announce.php` |
| 规则页面 | `forums.php?action=viewtopic&forumid=14&topicid=601`（发种规则）、`topicid=8847`（资源格式规范） |

---

## 一、总则

- 禁止在任何公开场所讨论本站（论坛、贴吧、各种群聊等），违者警告，严重者禁用
- 注册多个 HDHome 账号的用户将被禁止，出借、出租、出售账号也被禁止
- 不要把本站的种子文件上传到其他 Tracker
- 禁止将本站资源或他站 PT 资源以非 PT 方式进行网络公开共享（百度网盘、GD 网盘、115 盘、BT 网站等）
- 以下行为第一次警告，第二次永久无缘 HDHome：
  1. 在论坛或服务器中的捣乱行为（灌水、辱骂他人等）
  2. 向管理组发送本站常见问题&规则已列出的问题，或因吸血考核等要求宽限
  3. 种子评论区出现：求 free、等 free、不 free 不下等字样

### 1.1 账号保留规则

| 条件 | 保留规则 |
|------|----------|
| Nexus Master 及以上等级 **或 200 万做种积分** | 永久保留 |
| Veteran User 及以上等级 | 封存账号后不会被删除或禁用 |
| 封存账号的用户 | 连续 **120天** 不登录将被封禁 |
| 未封存账号的用户 | 连续 **60天** 不登录将被封禁 |
| 注册 **5天** 内无上下载流量 | 账号会被删除 |
| 无流量用户（上传/下载都为0） | 连续 **7天** 不登录或注册满7天将被删除 |
| 黄星捐赠会员 | 永久保留 |
| 持有永久保号卡用户 | 永久保留（仅特定活动中获取） |

### 1.2 防吸血规则

- 下载超过 **200GB**，且分享率低于 **0.2** 时，账号将被系统直接封禁
- 分享率 < 1.0 会导致账号被封禁

### 1.3 进站限制

- 进站 **30天** 内，禁止使用各种盒子(SEEDBOX)，违者视做代刷处理（贵宾不受限制）

---

## 二、种子促销规则

### 2.1 随机促销（种子上传后系统自动随机设定）

| 概率 | 促销类型 |
|------|----------|
| 10% | 50% 下载 |
| 2% | 免费(Free) |
| 5% | 2x 上传 |
| 20% | 30% 下载 |

### 2.2 管理员促销

- Blu-ray Disk、HD DVD 原盘有一定机率由管理员设置成"免费"
- **HDH 原创资源**会有较大机率设置成"免费"
- 关注度高的种子将被设置为促销（由管理员定夺）

### 2.3 促销类型说明

| 促销类型 | 说明 |
|----------|------|
| 普通 | 下载量、上传量均按正常计算 |
| Free | 不计算下载量，上传量按正常计算 |
| 2X | 下载量正常计算，上传量按2倍计算 |
| 2XFree | 不计算下载量，上传量按2倍计算 |
| 2X50% | 下载量按50%计算，上传量按2倍计算 |
| 50% | 下载量按50%计算，上传量按正常计算 |
| 30% | 下载量按30%计算，上传量按正常计算 |

---

## 三、H&R 规则

- 有 H&R 标志的种子 **60天** 之内须做种 **336小时** 或分享率达到 **1**，未达要求则记录 1 个 H&R
- 累计得到 **10个** H&R 即禁止账号
- 使用魔力可消除 H&R，**20,000 魔力** 消除一个 H&R
- 种子发布 **60天** 之后自动取消种子 H&R 标识（取消 H&R 之前开始的下载记录仍然检测，直到合格）
- 下载开始即开始计算 H&R，即使没有完成也会计算，60天内完成即为合格
- **24小时** 检测一次 H&R
- 发布员以上等级发种时才有权限是否启用 H&R
- **贵宾以上等级不受 H&R 约束**

---

## 四、盒子(SEEDBOX)规则

### 4.1 基本规则

- 允许使用盒子，但**禁止使用共享类型**的盒子（共享 IP、硬盘等，如 FH 提供商）
- 新手考核期内（注册后30天），所有非捐赠会员禁止使用盒子
- 使用盒子的会员需要到**备案专贴**和**控制面板-个人说明**进行备案，备案内容包括：
  1. 盒子的 IPV4 地址、IPV6 地址
  2. 使用的开始时间和结束时间
  3. 盒子的带宽、BT 客户端（包括版本号）
  4. 是否独立 IP，独自使用

### 4.2 盒子注意事项

- **VPS（虚拟服务器）类型**的盒子不允许使用，发现立即禁止账号
- 使用交易平台，如果商家在本站判定有代刷嫌疑的，发现立即禁止账号
- 请勿删除备案过的盒子信息，保留盒子购买凭证

### 4.3 盒子流量统计

- 盒子下载的数据按 **100% 下载流量**计算，不享受任何优惠待遇（**全部黑种**）
- 盒子上传的流量按实际流量计算，不享受 **2X、2XFREE** 待遇
- 盒子上传的流量，**72小时内**最多记录**种子体积的三倍**大小（如种子 10GB，上传最多记录 30GB），超过 72 小时流量正常计算
- **VIP 用户组不受此限制**（但必须备案，且使用时间不低于 3 个月）
- **自己发布的种子不受此限制**
- 种子发布一个月之后，如果做种人数 **≤3**，则享受正常优惠，不受规则限制

### 4.4 全局单种限速

- 单种上传限速 **25M/S**
- 超过限速，**72小时内**只记录**种子体积的三倍**上传，超过 72 小时不受限制
- **VIP 用户组不受此限制**

### 4.5 流量控制

- 普通会员限制 **60个种子/小时**
- VIP 及以上等级 **90个种子/小时**
- 超出限制将被**黑名单 4 小时**（Out of Limit!）

---

## 五、发种规范

### 5.1 候选制

- **Crazy User（营长）及以上**：可直接发布种子，无需经过候选
- **Peasant（俘虏）及以上**：可以添加候选
- 候选在投票通过或被管理通过后会收到通知短信，显示为【允许】

### 5.2 不允许的资源

- 分辨率未达 **1080i** 或以上
- 总体积小于 **100MB** 的资源
- 标清视频 upscale 或部分 upscale 而成的视频
- 质量较差的视频文件（CAM、TC、TS、SCR、DVDSCR、R5、R5.Line、HalfCD 等）
- RealVideo 编码（RMVB/RM）、flv 文件
- 单独的样片（样片请和正片一起上传）
- 未达到 5.1 声道标准的有损音频（有损 MP3、有损 WMA 等）
- 无正确 cue 表单的多轨音频文件
- RAR 等压缩文件
- 重复（dupe）资源
- 禁忌或敏感内容
- 损坏的文件、垃圾文件
- **杂项（课程、电子书、软件、漫画、墙纸、素材等）概不接受**

### 5.3 标题命名规则

**电影**：
```
[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```
范例：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 1080p BluRay x264-SiNNERS`

**电视剧**：
```
[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```
范例：`越狱 Prison Break S04E01 PROPER 1080p HDTV x264-CTU`

**音轨**：
```
[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]
```
范例：`恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC`

**标题注意事项**（来自 topicid=8847）：
- 主标题不能含有中文名（除了片名）
- DTS-HD 不能出现其他标点符号
- 末尾不能出现文件后缀名
- 副标题不需要重复出现和主标题相同英文片名
- 转载来源注明在简介中即可，副标题不能出现

### 5.4 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 上传者必须保证上传速度与做种时间。做种时间不足 24 小时或故意低速上传将被警告甚至取消上传权限
- 发布者将获得**双倍上传量**
- 副标题不能出现转载与站点信息，否则可能会在没有通知的情况下删除种子
- 对于**游戏类资源**，只有上传员及以上等级的用户，或者是管理组特别指定的用户，才能上传

### 5.5 重复（Dupe）判定规则

**原则：质量重于数量**

**来源媒介优先级**：
```
Blu-ray/HD DVD > HDTV > DVD > TV
```
- 高优先级版本将使低优先级版本被判定为重复

**同一来源的 Dupe 规则**：
- 同一区域来源类型已发布的情况下，之后发布的相同介质的将判定为重复（来源相同的字幕音轨）
- 同一区域来源类型已发布的情况下，之后发布的包含了**不同的音轨、字幕等**不会被判定重复

**同媒介同分辨率重编码**：
- 参考"Scene & Internal, from Group to Quality-Degree"按发布组确定优先级
- 高优先级发布组版本将使低优先级或相同优先级版本被判定为重复
- 基于无损截图对比，高质量版本将使低质量版本被视为重复

**跨区域原盘**：
- 来自其他区域，包含不同配音和/或字幕的 Blu-ray/HD DVD 原盘版本**不被视为重复**

**无损音轨 Dupe**：
- 每个无损音轨资源原则上只保留一个版本，其余不同格式的版本将被视为重复
- **分轨 FLAC 格式**有最高的优先级

**Dupe 豁免条件**：
- 新版本没有旧版本中已确认的错误/画质问题，或新版本的来源有更好的质量 → 允许发布，旧版视为重复
- 旧版本已经连续断种 **60日以上** 或已经发布 **12个月以上** → 发布新版本不受 Dupe 约束

**老种保留**：
- 新版本发布后，旧的、重复的版本将被保留，直至断种

### 5.6 资源打包规则

原则上只允许以下资源打包：
- 按套装售卖的高清电影合集（如 The Ultimate Matrix Collection Blu-ray Box）
- **整季**的电视剧/综艺节目/动漫
- 同一专题的纪录片
- 同一艺术家的音乐：**5张或5张以上**专辑方可打包；**两年内**发售的专辑可单独发布；打包时应剔除站内已有资源或全部包括
- 分卷发售的动漫剧集、角色歌、广播剧等
- 发布组打包发布的资源

**打包要求**：
- 视频资源必须来源于**相同类型的媒介**、**相同分辨率水平**、**编码格式一致**（预告片例外）
- 电影合集的发布组也必须统一
- 音频资源必须编码格式一致（如全部为分轨 FLAC）
- 打包发布后，将视情况删除相应单独的种子

### 5.7 例外规则

- 允许发布小于 100MB 的高清相关软件和文档
- 允许发布小于 100MB 的单曲专辑
- 允许发布 **2.0 声道或以上**标准的国语/粤语音轨
- 允许在发布的资源中附带字幕、游戏破解与补丁、字体、包装扫描图（必须统一打包或统一不打包）
- 允许在发布音轨时附带附赠 DVD 的相关文件

### 5.8 杂项分类

- 杂项分类（Misc, type=409）：包括但不限于课程、电子书、软件、漫画、墙纸、素材等，**概不接受**（除获管理组邀请）

---

## 六、发布页面表单字段分析

### 6.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（不填则使用种子文件名，要求规范填写） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `douban_id` | text | - | 豆瓣 ID 或链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |

**注意**：有 `douban_id` 字段（非标准 NexusPHP），可输入豆瓣影视 ID 或链接。

### 6.2 类型（`type`）— 双区域

HDHome 有两个 `type` 下拉框，**只选其中之一**：

#### 种子区 — 49 个分类

| 值 | 显示名称 |
|----|----------|
| 506 | Movies 8K UHD BD |
| 499 | Movies UHD Blu-ray |
| 518 | Movies UHD REMUX |
| 450 | Movies Bluray |
| 415 | Movies REMUX |
| 505 | Movies 8K/4320p |
| 416 | Movies 2160p |
| 414 | Movies 1080p |
| 413 | Movies 720p |
| 411 | Movies SD |
| 412 | Movies IPad |
| 523 | TVSeries 8KUHD |
| 502 | TVSeries 4K Bluray |
| 451 | Doc Bluray |
| 421 | Doc REMUX |
| 526 | TVSeries 4320p |
| 431 | TVShow 2160p |
| 433 | TVSeries IPad |
| 434 | TVSeries 720p |
| 435 | TVSeries 1080i |
| 436 | TVSeries 1080p |
| 437 | TVSeries REMUX |
| 453 | TVSereis Bluray |
| 438 | TVSeries 2160p |
| 439 | Musics APE |
| 432 | TVSeries SD |
| 440 | Musics FLAC |
| 441 | Musics MV |
| 503 | Musics Bluray |
| 442 | Sports 720p |
| 510 | Anime 8K UHD BD |
| 443 | Sports 1080i |
| 444 | Anime SD |
| 445 | Anime IPad |
| 446 | Anime 720p |
| 447 | Anime 1080p |
| 448 | Anime REMUX |
| 454 | Anime Bluray |
| 531 | Anime UHD REMUX |
| 409 | Misc |
| 449 | Anime 2160p |
| 509 | Anime 8K/4320p |
| 501 | Anime UHD Blu-ray |
| 504 | Sports 2160p |
| 511 | Sport 8K/4320p |
| 508 | Doc 8K UHD BD |
| 529 | Doc 8K UHD BD REMUX |
| 500 | Doc UHD Blu-ray |
| 507 | Doc 8K/4320p |
| 422 | Doc 2160p |
| 420 | Doc 1080p |
| 419 | Doc 720p |
| 417 | Doc SD |
| 418 | Doc IPad |
| 424 | TVMusic 1080i |
| 423 | TVMusic 720p |
| 452 | TVShows Bluray |
| 430 | TVShow REMUX |
| 429 | TVShow 1080p |
| 428 | TVShow 1080i |
| 427 | TVShow 720p |
| 425 | TVShow SD |
| 426 | TVShow IPad |

#### LIVE 区 — 27 个分类

| 值 | 显示名称 |
|----|----------|
| 494 | Movies Bluray |
| 495 | Doc Bluray |
| 469 | TVMusic 1080i |
| 472 | TVShow 720p |
| 473 | TVShow 1080i |
| 474 | TVShow 1080p |
| 475 | TVShow REMUX |
| 496 | TVShows Bluray |
| 476 | TVShow 2160p |
| 477 | TVSeries SD |
| 478 | TVSeries IPad |
| 479 | TVSeries 720p |
| 480 | TVSeries 1080p |
| 481 | TVSeries REMUX |
| 497 | TVSereis Bluray |
| 482 | TVSeries 2160p |
| 483 | Musics APE |
| 484 | Musics FLAC |
| 486 | Sports 720p |
| 487 | Sports 1080i |
| 488 | Anime SD |
| 489 | Anime IPad |
| 490 | Anime 720p |
| 491 | Anime 1080p |
| 492 | Anime REMUX |
| 498 | Anime Bluray |
| 493 | Anime 2160p |

**分类特点**：
- 分辨率编码在分类值中（如 414=Movies 1080p），与 HDFans 类似的 data-mode 方式
- 有完整的 8K/4320p 分类体系（505/506/507/509/511/523/526/529/531）
- TV 系列细分为 TVShow（综艺）和 TVSeries（剧集）两大类
- 有 IPad 分类（小屏设备专用编码）
- LIVE 区为实时直播内容

### 6.3 来源（`source_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 9 | UHD Blu-ray |
| 1 | Blu-ray |
| 4 | HDTV |
| 3 | DVD |
| 7 | WEB-DL |
| 8 | Other |

### 6.4 媒介（`medium_sel`）— 8 个

| 值 | 显示名称 |
|----|----------|
| 10 | UHD Blu-ray |
| 1 | Blu-ray |
| 3 | Remux |
| 7 | Encode |
| 5 | HDTV |
| 8 | CD |
| 4 | MiniBD |
| 11 | WEB-DL |

### 6.5 编码（`codec_sel`）— 5 个

| 值 | 显示名称 |
|----|----------|
| 1 | AVC/H264/x264 |
| 2 | HEVC/H265/x265 |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | Other |

### 6.6 音频编码（`audiocodec_sel`）— 13 个

| 值 | 显示名称 |
|----|----------|
| 6 | AAC |
| 15 | AC3/DD |
| 2 | APE |
| 16 | WAV |
| 1 | FLAC |
| 3 | DTS |
| 13 | TrueHD |
| 14 | LPCM |
| 11 | DTS-HDMA |
| 18 | DTS-HDHRA |
| 12 | TrueHD Atmos |
| 17 | DTS-HDMA:X 7.1 |
| 7 | Other |

### 6.7 分辨率（`standard_sel`）— 6 个

| 值 | 显示名称 |
|----|----------|
| 1 | 2160p/4K |
| 2 | 1080p |
| 3 | 1080i |
| 4 | 720p |
| 5 | SD |
| 10 | 4320p/8K |

### 6.8 处理（`processing_sel`）— 2 个

| 值 | 显示名称 |
|----|----------|
| 1 | Raw |
| 2 | Encode |

### 6.9 制作组（`team_sel`）— 14 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | HDHome | 站方制作组 |
| 2 | HDH | 站方简称 |
| 3 | HDHTV | 站方 TV 组 |
| 4 | HDHPad | 站方 Pad 组 |
| 12 | HDHWEB | 站方 WEB 组 |
| 20 | 3201 | 制作组 |
| 17 | SHMA | 制作组 |
| 21 | TVman | 制作组 |
| 19 | ARiN | 制作组 |
| 6 | TTG | TTG |
| 7 | M-Team | 馒头 |
| 11 | Other | 其他 |
| 22 | 969154968 | 制作组 |
| 23 | BMDru | 制作组 |

### 6.10 标签（`tags[]`）— 18 个 checkbox

| 值 | 显示名称 | CSS 类 |
|----|----------|--------|
| yc | 原创 | tyc |
| gy | 国语 | tgy |
| yy | 粤语 | tyy |
| gz | 官字 | tgz |
| zz | 中字 | tzz |
| tx | 特效 | ttx |
| ybyp | 原生 | tybyp |
| lz | 连载 | tlz |
| wj | 完结 | twj |
| diy | DIY | tdiy |
| db | DOLBY VISION | tdb |
| hdr10 | HDR10 | thdr10 |
| hdrm | HDR10+ | thdrm |
| cc | Criterion | tcc |
| jz | 禁转 | tjz |
| xz | 限转 | txz |
| sf | 首发 | tsf |
| yq | 应求 | tyq |

**注意**：标签使用字符串值（如 `yc`、`gy`），非数字 ID。

### 6.11 其他字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `offers` | checkbox | 候选发布（value="yes"） |
| `uplver` | checkbox | 匿名发布（value="yes"） |

---

## 七、关键适配器设计要点

### 7.1 双区域 type 字段

发布页面有两个同名的 `type` 下拉框（`browsecat` 和 `specialcat`），选择一个时另一个自动禁用。适配器需注意：
- 正常转发使用 **种子区**（`browsecat`）
- LIVE 区一般不用于转发
- 两个下拉框的 `name` 都是 `type`，提交时只发选中的那个

### 7.2 分类编码包含分辨率和类型

与 HDFans 类似，type 值本身编码了内容类型+分辨率信息。例如：
- 414 = Movies + 1080p
- 438 = TVSeries + 2160p
- 506 = Movies + 8K UHD BD

适配器需根据内容类型和分辨率组合来确定正确的 type 值。

### 7.3 豆瓣 ID 字段

`douban_id` 字段支持输入豆瓣影视 ID 或链接（如 `https://movie.douban.com/subject/35050809/` 或 `35050809`）。

### 7.4 标题规则特殊要求

- 主标题不能含中文名（除了片名）
- DTS-HD 不能出现其他标点符号
- 末尾不能出现文件后缀名
- 副标题不能重复主标题英文片名
- 副标题不能出现转载来源（来源注明在简介中）

### 7.5 候选制

非 Crazy User 等级的用户需先提交候选，通过后才能发布。适配器可设置 `offers=yes` 参数来候选发布。

### 7.6 资源简介要求

- 必须包含海报、横幅或封面
- 尽可能包含画面截图
- 尽可能包含文件详细信息（格式、时长、编码、码率、分辨率、语言、字幕）
- 尽可能包含演职员名单和剧情概要
- 无 NFO 文件时必须填写编码信息

---

## 八、发布字段与通用模型的映射

### 8.1 来源映射（source_sel）

| 通用来源 | HDHome source_sel 值 |
|----------|---------------------|
| UHD Blu-ray | 9 |
| Blu-ray | 1 |
| HDTV | 4 |
| DVD | 3 |
| WEB-DL | 7 |
| Other | 8 |

### 8.2 媒介映射（medium_sel）

| 通用媒介 | HDHome medium_sel 值 |
|----------|---------------------|
| UHD Blu-ray | 10 |
| Blu-ray | 1 |
| Remux | 3 |
| Encode | 7 |
| HDTV | 5 |
| CD | 8 |
| MiniBD | 4 |
| WEB-DL | 11 |

### 8.3 编码映射（codec_sel）

| 通用编码 | HDHome codec_sel 值 |
|----------|---------------------|
| AVC/x264 | 1 |
| HEVC/x265 | 2 |
| VC-1 | 3 |
| MPEG-2 | 4 |
| Other | 5 |

### 8.4 音频编码映射（audiocodec_sel）

| 通用音频编码 | HDHome audiocodec_sel 值 |
|-------------|-------------------------|
| FLAC | 1 |
| APE | 2 |
| DTS | 3 |
| AAC | 6 |
| Other | 7 |
| TrueHD | 13 |
| LPCM | 14 |
| AC3/DD | 15 |
| WAV | 16 |
| DTS-HDMA | 11 |
| DTS-HDHRA | 18 |
| TrueHD Atmos | 12 |
| DTS-HDMA:X 7.1 | 17 |

### 8.5 分辨率映射（standard_sel）

| 通用分辨率 | HDHome standard_sel 值 |
|-----------|----------------------|
| 4320p/8K | 10 |
| 2160p/4K | 1 |
| 1080p | 2 |
| 1080i | 3 |
| 720p | 4 |
| SD | 5 |

### 8.6 处理映射（processing_sel）

| 通用处理 | HDHome processing_sel 值 |
|---------|------------------------|
| Raw | 1 |
| Encode | 2 |

### 8.7 type 分类映射（种子区主要分类）

| 内容类型 | 分辨率/媒介 | type 值 |
|---------|------------|---------|
| Movies | 8K UHD BD | 506 |
| Movies | UHD Blu-ray | 499 |
| Movies | UHD REMUX | 518 |
| Movies | Bluray | 450 |
| Movies | REMUX | 415 |
| Movies | 8K/4320p | 505 |
| Movies | 2160p | 416 |
| Movies | 1080p | 414 |
| Movies | 720p | 413 |
| Movies | SD | 411 |
| Movies | IPad | 412 |
| TVSeries | 8KUHD | 523 |
| TVSeries | 4K Bluray | 502 |
| TVSeries | 4320p | 526 |
| TVSeries | 2160p | 438 |
| TVSeries | 1080p | 436 |
| TVSeries | 1080i | 435 |
| TVSeries | 720p | 434 |
| TVSeries | SD | 432 |
| TVSeries | IPad | 433 |
| TVSeries | REMUX | 437 |
| TVSeries | Bluray | 453 |
| TVShow | 2160p | 431 |
| TVShow | 1080p | 429 |
| TVShow | 1080i | 428 |
| TVShow | 720p | 427 |
| TVShow | SD | 425 |
| TVShow | IPad | 426 |
| TVShow | REMUX | 430 |
| TVShows | Bluray | 452 |
| Doc | 8K UHD BD | 508 |
| Doc | 8K UHD BD REMUX | 529 |
| Doc | UHD Blu-ray | 500 |
| Doc | 8K/4320p | 507 |
| Doc | Bluray | 451 |
| Doc | REMUX | 421 |
| Doc | 2160p | 422 |
| Doc | 1080p | 420 |
| Doc | 720p | 419 |
| Doc | SD | 417 |
| Doc | IPad | 418 |
| Anime | 8K UHD BD | 510 |
| Anime | UHD Blu-ray | 501 |
| Anime | UHD REMUX | 531 |
| Anime | 8K/4320p | 509 |
| Anime | Bluray | 454 |
| Anime | REMUX | 448 |
| Anime | 1080p | 447 |
| Anime | 720p | 446 |
| Anime | IPad | 445 |
| Anime | SD | 444 |
| Anime | 2160p | 449 |
| Musics | APE | 439 |
| Musics | FLAC | 440 |
| Musics | MV | 441 |
| Musics | Bluray | 503 |
| Sports | 2160p | 504 |
| Sports | 8K/4320p | 511 |
| Sports | 720p | 442 |
| Sports | 1080i | 443 |
| TVMusic | 1080i | 424 |
| TVMusic | 720p | 423 |
| Misc | - | 409 |

---

## 九、特殊注意事项

### 9.1 分类体系特点

HDHome 的分类是 **内容类型 × 分辨率/媒介** 的矩阵式编码：
- 内容大类：Movies、TVSeries（剧集）、TVShow（综艺）、Doc、Anime、Musics、Sports、TVMusic、Misc
- 分辨率/媒介：8K UHD BD、UHD Blu-ray、Bluray、REMUX、8K/4320p、2160p、1080p、1080i、720p、SD、IPad
- 所有分类值均以 4xx/5xx 编号，无明显规律需查表

### 9.2 LIVE 区

LIVE 区是 HDHome 的特色功能，用于实时直播内容。一般转发场景不涉及 LIVE 区。

### 9.3 质量字段

`source_sel`、`medium_sel`、`codec_sel`、`audiocodec_sel`、`standard_sel`、`processing_sel` 均为非必填字段，但建议填写以提高种子质量。

### 9.4 最低分辨率要求

所有视频资源分辨率必须达到 **1080i** 或以上。SD 资源仅限于非视频内容。

### 9.5 杂项分类（Misc）

Misc 分类（type=409）存在但规则中明确表示「课程、电子书、软件、漫画、墙纸、素材等概不接受」，实际用途有限。

### 9.6 CSS 标签样式映射

标签在种子列表中有对应的颜色样式：

| 标签值 | CSS 类 | 背景色 |
|--------|--------|--------|
| gf | tgf | #06c |
| yc | tyc | #085 |
| gz | tgz | #530 |
| db | tdb | #358 |
| hdr10 | thdr10 | #9a3 |
| hdrm | thdrm | #9b5 |
| gy | tgy | #f96 |
| yy | tyy | #f66 |
| zz | tzz | #9c0 |
| tx | ttx | #f38 |
| jz | tjz | #903 |
| xz | txz | #c03 |
| diy | tdiy | #993 |
| sf | tsf | #339 |
| yq | tyq | #f90 |
| m0 | tm0 | #096 |
| bz | tbz | #333 |
| ybyp | tybyp | #5C44BB |
| xy | txy | #553300 |
| cc | tcc | #4488bb |
| wj | twj | #f02498 |
| lz | tlz | #d064e8 |

### 9.7 互斥站点

**家园与铂金家互斥**，禁止互相转载发布。适配器转发时需检查源站/目标站是否为铂金家，若是则拒绝转发。

### 9.8 字幕区规则

- 允许的文件格式：srt/ssa/ass/cue/zip/rar
- Vobsub 格式（idx+sub）或其他格式/合集需打包为 zip/rar 后上传
- 音轨对应 cue 表单文件可上传，多个 cue 需打包
- 不允许 lrc 歌词或其他非字幕/cue 文件
- 不合格字幕判定：不匹配/不同步/打包错误/编码错误/语种标识错误/标题命名不明确/重复
- 举报不合格字幕奖励 **50魔力**，上传者扣除 **100魔力**

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-22*
