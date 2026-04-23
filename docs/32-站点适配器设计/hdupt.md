# 好多油 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 好多油|
| 站点地址 | https://pt.hdupt.com |
| 站点框架 | NexusPHP |
| Tracker URL | https://pt.hdupt.com/announce.php |
| 特殊功能 | 媒介区分 TV/电影、UHD Blu-ray/UHD Remux 独立选项、processing 地区 8 类、HR 规则(72h/2周)、速度监控 200MiB/s |
| 规则页面 | rules.php |
| 公告页面 | forums.php?topicid=304（HDUPT 发布总则【试行】— 含电影/剧集/纪录片/综艺/动漫/体育/游戏/音轨 8 类细则） |
| 发布页面 | upload.php |

---

## 一、发布页面表单字段（upload.php）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `file` | file | 种子文件 |
| `name` | text | 主标题 |
| `small_descr` | text | 副标题 |
| `nfo` | file | NFO 文件 |
| `descr` | textarea | 简介（BBCode） |
| `uplver` | checkbox | 匿名发布（value=yes） |

**无** IMDb 字段、**无** 豆瓣字段、**无** PT-Gen 字段、**无** MediaInfo 独立字段、**无** 标签字段、**无** source_sel 字段。

### 1.2 分类（`type`）— 10个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/纪录片 |
| 405 | Animations/动画 |
| 406 | Music Videos/音乐 MV |
| 407 | Sports/体育 |
| 408 | HQ Audio/无损音乐 |
| 411 | Misc/其他 |
| 410 | Games/游戏 |

### 1.3 媒介（`medium_sel`）— 15个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | Blu-ray | FHD 蓝光原盘 |
| 11 | UHD Blu-ray | UHD 蓝光原盘 |
| 5 | HDTV | 高清电视 |
| 6 | DVD | DVD |
| 3 | Remux | FHD Remux |
| 15 | UHD Remux | UHD Remux（电影） |
| 16 | UHD Remux TV | UHD Remux（剧集） |
| 12 | Remux TV | FHD Remux（剧集） |
| 7 | Encode | 压制（电影） |
| 14 | Encode TV | 压制（剧集） |
| 10 | WEB-DL/WEBRip | WEB-DL/WEBRip（电影） |
| 13 | WEB-DL/WEBRip TV | WEB-DL/WEBRip（剧集） |
| 4 | MiniBD | MiniBD |
| 8 | CD | CD |
| 9 | Track | 单曲音轨 |

**重要**: 媒介细分 TV/电影——同一类型有不同的电影和剧集选项（如 `Encode`(7) vs `Encode TV`(14)，`WEB-DL`(10) vs `WEB-DL/WEBRip TV`(13)）。UHD Blu-ray 和 UHD Remux 为独立选项。这是已采集站点中媒介细分最详细的之一。

### 1.4 视频编码（`codec_sel`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC |
| 14 | H.265/HEVC |
| 2 | VC-1 |
| 16 | x264 |
| 3 | Xvid |
| 18 | MPEG/MPEG-2 |
| 5 | Other |

**注意**: H.264/AVC(1) 和 x264(16) 为独立选项（区分原盘编码和压制编码），无 AV1、VP8/9、AVS。

### 1.5 音频编码（`audiocodec_sel`）— 13个

| 值 | 显示名称 |
|----|----------|
| 16 | DTS:X |
| 1 | DTS-HDMA |
| 3 | TrueHD |
| 11 | LPCM |
| 4 | DTS |
| 2 | AC3/EAC3 |
| 6 | AAC |
| 7 | FLAC |
| 10 | APE |
| 17 | WAV |
| 18 | MPEG |
| 13 | Other |

**注意**: 无 TrueHD Atmos、DDP Atmos、DDP/E-AC-3 独立区分；AC3 和 EAC3 合并为 AC3/EAC3(2)。DTS-HDMA(1) 无 "/DTS XLL" 后缀。无 MP3、Opus、DSD、AV3A、AC-4、MPEG-H、MQA。

### 1.6 分辨率（`standard_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 5 | 4K/2160p |
| 3 | 720p |
| 4 | SD |
| 6 | iPad |

**注意**: 含 iPad(6) 分辨率选项。

### 1.7 地区（`processing_sel`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | CN/中国内地 |
| 3 | HK/TW/港台 |
| 2 | US/EU/欧美 |
| 4 | JP/日本 |
| 5 | KR/韩国 |
| 6 | India/印度 |
| 8 | SEA/东南亚 |
| 7 | Other |

**注意**: 含 India(6) 和 SEA(8)，是已采集站点中地区分类较细的之一。

### 1.8 制作组（`team_sel`）— 3个

| 值 | 显示名称 |
|----|----------|
| 2 | HDU |
| 5 | Other |

仅 2 个有效选项（本站官组 HDU + Other），是已采集站点中最少的。

---

## 二、完整站点规则（rules.php + 论坛 #304）

> 数据来源：rules.php（5349 字符）+ forums.php?topicid=304 发布总则（6704 字符）

### 2.1 总则

- 转载种子禁止修改原资源文件名、文件夹名、目录结构
- 禁止为赚取魔力刷评论/赞
- 系统速度监控上行 **200MiB/s**，盒子用户请及时限速，下行无限制
- 盒子使用需备案（IP 变动专贴）
- 转载或使用站内资源须感谢原后缀并注明来源

### 2.2 账号保留规则

| 等级/状态 | 保留条件 |
|-----------|----------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上（封存） | 封存后不会被删除 |
| 封存账号 | 连续 365 天不登录删除 |
| 未封存账号 | 连续 120 天不登录删除 |
| 无流量用户 | 连续 3 天不登录删除 |

### 2.3 H&R 保种规则

- 单种分享率达到 **1:1**，**或者**在两周内累计做种 **72 小时**（以个人页面完成种子为准）
- 严重违规者将被警告或封禁
- **为逃避 H&R 而有意不完成种子的用户将被直接封禁**

### 2.4 促销规则

**自动促销**：
- 官方组种子 → 自动免费
- 自购/自压/自录等自主资源 → 免费
- BD50 untouched 原盘 → 免费
- 经典资源 → 免费

**促销时限阶梯**：

| 促销类型 | 时限 | 结束后 |
|----------|------|--------|
| 2x 免费 | 1 天 | → 50% |
| 免费 | 3 天 | → 50% |
| 2x 50% | 3 天 | → 30% |
| 50% | 永久 | — |
| 30% | 5 天 | → 50% |
| 2X | 60 天 | → 50% |
| 普通（黑种） | 40 天 | → 2X |

### 2.5 上传总则

- 上传前须先搜索种子是否已存在或违反重复规则
- 做种时间不足 72h 或故意低速上传将被警告甚至取消上传权限

### 2.6 上传者资格

- User 级别及以上可发布资源
- 新人需先在候选区提交候选，联系管理组查看

### 2.7 重复（Dupe）规则

- **已有蓝光重编码(BluRay Encode)的情况下，禁止发布 WEB-DL、HDTV 资源**
- 除共版原盘外，其他区原盘可共存（德版+法版等），DIY 原盘与 untouched 原盘可共存，不同小组作品可共存
- **例外：2160p/1080p/720p 的 WEB-DL/WEBRip/HDTV 资源，264/265 编码及 HDR/SDR 下各只允许一份**
- **每季剧集完结后禁止再发布单集资源**
- 新剧须按顺序发布（若前 8 集都没有，不允许先发第 9 集）
- **WEB-DL 更优质源可替代先出源**（亚马逊替代 iTunes）
- **官方小组作品优先一切其它资源**，不受上述 Dupe 规则限制

### 2.8 允许资源

- 蓝光原盘/蓝光 DVD 及其重编码
- HDTV 录制及其重编码
- 蓝光原盘/蓝光 DVD Remux 及其重编码
- DVD 5/9
- WEB-DL 流
- SD/720p/1080p/2160p 分辨率资源（压制源须为蓝光/HDTV）

### 2.9 禁止资源

- 纯肉类 xxx
- **二次编码（BR/HR-HDTV）、WEB-DL 重编码、DVD 重编码**
- BDMV 解压文件夹形式的 3D 资源
- 枪版（CAM/TS/TC/SCR/R5）
- 来源不明的资源
- 未解压的 RAR 资源
- 低质量资源（超小体积/超低码率/rmvb/rm/flv/3gp/asf）
- **非官方打包合集**（官方定义：1.发行商发布如剧场版/导演剪辑版二合一原盘；2.HDU 官方小组）

---

## 三、各类型资源发种细则（论坛 #304）

### 3.1 制作种子注意事项（通用）

- 禁止擅自改动原文件名发布
- 不能以压缩分卷形式制作种子
- 文件夹中不能包含海报封面等无关文件
- 文件夹中不能包含 `*.torrent`、`*.url`、`*.txt` 等无关文件
- 文件夹中不能包含 srt/ass/ssa 格式字幕文件（须单独上传），但可包含 idx+sub 字幕文件

### 3.2 电影

**标题格式**：`英文名称 年代 分辨率 媒介 音频编码-制作组`

**副标题**：中文名称 [补充说明]（半角符号）

**允许**：正规发行版 Bluray/HDDVD/DVD/VCD 非重编码 + 一次重编码、HDTV 非重编码 + 一次重编码、iTunes 等优质视频供应商非重编码、iPad 平台视频、DVDRip/MiniSD

**禁止**：其他站独占/禁转资源、无压制组且无编码参数的视频、低码率高清、upscale 高清、枪版(CAM/TS/TC/SCR/R5)、非正规碟片压制源、rmvb/rm/flv/3gp/asf、HR-HDTV(1024x\*\*\*/960x\*\*\*)

**合集**：仅允许压制组官方打包的同一系列完结电影合集（哈利波特、魔戒等连续性合集），不允许 IMDB TOP100 等无关联合集，合集须分辨率+制作组全部相同

**简介要求**：须有 IMDb 链接 + 海报/封面 + 影片资料/剧情介绍 + 编码信息或 NFO 至少一个 + 推荐 4 张截图

### 3.3 电视剧集

**标题格式**：`英文名称 第几季第几集 年代 分辨率 介质 音频 编码-制作组`

**副标题**：中文名称 + 补充信息（无英文名可用拼音）

**合集**：质量相同（分辨率/介质/编码一致），仅允许整季打包，不允许未完结多季合集

**简介要求**：须有海报/封面 + 影片资料/剧情介绍，推荐编码信息+截图

### 3.4 纪录片

**标题格式**：`英文名称 第几季第几集 年代 分辨率 介质 音频 编码-制作组`

**副标题**：中文名称 + 补充信息

**简介要求**：须有纪录片资料，推荐截图

### 3.5 综艺

**标题格式**：`英文名称 年代 分辨率 介质 音频 编码-制作组`（无英文名用拼音）

**副标题**：中文名称 + 补充信息

**特殊说明**：MV/音乐会/演唱会/颁奖礼归类为综艺类；未发布的早期季播综艺可打包发布

### 3.6 动漫

**标题格式**：`英文名称/罗马音 年代 分辨率 介质 音频 编码-制作组`

**副标题**：中文名称 + 补充信息

**简介要求**：转载须尽量保留原发布者所有信息（含字幕组海报等）

### 3.7 体育

**标题格式**：`英文名称 节目日期 分辨率 介质 音频 编码-制作组`

**副标题**：中文名称 + 补充信息

### 3.8 游戏

**标题格式**：`游戏名 年份 公司名-破解组或来源`

**副标题**：语言 版本 其他

**简介要求**：须有游戏封面 + 简介含安装说明+配置要求 + 至少 2 张截图

**禁止**：国家/地区禁止游戏、小体积无分享意义游戏/补丁、与标题不符资源、不可运行版本（预载版本除外需注明）

### 3.9 音轨

**标题格式**：`艺术家英文名 - 专辑英文名 发行年代 音频格式`

**副标题**：中文艺术家名 - 中文专辑名 + 补充信息

**简介要求**：须有专辑封面、简介、曲目列表

---

## 四、HDUPT 特殊注意事项

### 4.1 媒介 TV/电影区分

发布时必须根据分类（Movies vs TV Series）选择对应媒介：
- 电影类: Encode(7), WEB-DL/WEBRip(10), Remux(3), UHD Remux(15)
- 剧集类: Encode TV(14), WEB-DL/WEBRip TV(13), Remux TV(12), UHD Remux TV(16)
- 通用: Blu-ray(1), UHD Blu-ray(11), HDTV(5), DVD(6), MiniBD(4), CD(8), Track(9)

适配器需要根据分类自动选择正确的媒介值。

### 4.2 H.264 vs x264 区分

- `H.264/AVC`(1): 用于原盘/Remux（未经再次编码）
- `x264`(16): 用于 Encode（使用 x264 编码器压制）

适配器需要根据媒介类型判断使用哪个值。

### 4.3 无 IMDb/豆瓣/PT-Gen 字段

发布表单无 IMDb、豆瓣、PT-Gen 输入框，影视链接信息只能写入简介 BBCode 中。

### 4.4 UHD 独立媒介

UHD Blu-ray(11) 和 UHD Remux(15)/UHD Remux TV(16) 为独立选项，而非通过分辨率+媒介组合表示。适配器需根据分辨率判断是否使用 UHD 媒介。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | HDUPT | 常见 NexusPHP |
|------|--------|---------------|
| 分类 | 10个（含 Games） | 通常 5-15 个 |
| 媒介 | **15个（TV/电影分开+UHD独立）** | 通常 8-10 个 |
| 视频编码 | 7个（H.264/x264 分开） | 通常 5-10 个 |
| 音频编码 | 13个（AC3/EAC3 合并） | 通常 6-15 个 |
| 分辨率 | 6个（含 iPad） | 通常 4-6 个 |
| 地区 | **8个（含 India/SEA）** | 通常无或 6 个 |
| 制作组 | **2个（最少）** | 通常 3-30 个 |
| IMDb/豆瓣 | **无** | 大多支持 |
| source_sel | 无 | 部分站有 |
| H&R | **72h/2周，逃避者直接封禁** | 因站而异 |
| 速度监控 | **上行 200MiB/s** | 通常无 |
| Dupe | **官组优先+BR优先于WEB+完结剧禁单集** | 因站而异 |
| 禁止二次编码 | **BR/HR-HDTV/WEB-DL重编码全禁** | 部分站有 |
| 非官方合集 | **禁止** | 因站而异 |
| 促销阶梯 | **7级（普通40天→2X→50%）** | 因站而异 |

---

## 六、适配器实现要点

### 6.1 TV/电影媒介选择

```go
func mapHDUPTMedium(baseMedium string, category int) int {
    tvCategories := map[int]bool{402: true, 403: true, 404: true, 405: true}
    isTV := tvCategories[category]
    
    switch baseMedium {
    case "Encode":
        if isTV { return 14 } else { return 7 }
    case "WEB-DL":
        if isTV { return 13 } else { return 10 }
    case "Remux":
        if isTV { return 12 } else { return 3 }
    case "UHD Remux":
        if isTV { return 16 } else { return 15 }
    case "Blu-ray":
        return 1
    case "UHD Blu-ray":
        return 11
    case "HDTV":
        return 5
    case "DVD":
        return 6
    default:
        if isTV { return 14 } else { return 7 }
    }
}
```

### 6.2 编码器选择

```go
func mapHDUPTCodec(codec string, medium int) int {
    // 原盘/Remux 使用 H.264/AVC(1)，Encode 使用 x264(16)
    isOriginal := medium == 1 || medium == 11 || medium == 3 || medium == 15 || medium == 12 || medium == 16
    
    switch codec {
    case "H.264", "AVC", "x264":
        if isOriginal { return 1 } else { return 16 }
    case "H.265", "HEVC", "x265":
        return 14
    case "VC-1":
        return 2
    case "Xvid":
        return 3
    case "MPEG-2", "MPEG":
        return 18
    default:
        return 5
    }
}
```

---

*数据来源: upload.php HTML (68458字节) + rules.php (5349字符) + 论坛#304发布总则 (6704字符) (2026-04-17/2026-04-22)*
*文档创建: 2026-04-17*
*文档更新: 2026-04-22 — 补充完整 rules.php 规则、论坛#304 八类资源发种细则(电影/剧集/纪录片/综艺/动漫/体育/游戏/音轨)、H&R规则、促销阶梯、Dupe规则、Tracker URL*
