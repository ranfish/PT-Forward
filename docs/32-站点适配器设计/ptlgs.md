# 劳改所 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 劳改所 |
| 域名 | ptlgs.org |
| 框架 | NexusPHP |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（所有用户需通过10次候选后才能直接发布） |
| 种审制 | 是（种子发布后需审核通过） |
| 站点角色 | 源站 + 发布站 |
| MediaInfo | 是（technical_info 字段，必填） |
| IMDb | 是（通过 pt_gen 字段，支持 IMDb/豆瓣/Bangumi/Indienova） |
| 豆瓣 | 是（pt_gen 字段，**优先豆瓣 > IMDb**，内置豆瓣搜索快捷通道） |
| PT-Gen | 是（pt_gen 字段，支持一键检索豆瓣信息） |
| 匿名发布 | 是（uplver checkbox） |
| NFO | 否 |
| 海报 | 是（cover 字段，必填，支持一键转存官方图床） |
| 截图 | 是（screenshots 字段，必填，至少3张） |
| 建站时间 | 2024年 |

## Tracker URL
`https://ptlgs.org/announce.php`

## 站点特色

- **官方审种脚本**：提供 Greasyfork `ptlgs-Torrent-Assistant` v1.1.43（830行 JS），详情页自动校验
- **官方转种脚本**：提供 `auto_feed.user.js` 转种辅助
- **豆瓣优先建库**：采用豆瓣建库，优先使用豆瓣链接；内置豆瓣搜索代理 `douban-search.ptl.gs`
- **10次候选制**：所有用户需通过10次候选后才能直接发布资源
- **发布者双倍上传**：发布者获得双倍上传量
- **白名单图床**：files.ptlgs.org / cmct.xyz / ssdforum.org / gifyu.com / imgbox.com / pixhost.to / ptpimg.me
- **独立Wiki**：https://wiki.ptlgs.org 发布规则文档
- **9KG/成人内容禁止**：本软件禁止下载和发布此类内容
- **黑名单制作组**：禁止发布 FGT/NSBC/BATWEB/GPTHD/DreamHD/BlackTV/CatWEB/Xiaomi/Huawei/MOMOWEB/DDHDTV/SeeWeb/TagWeb/SonyHD/MiniHD/BitsTV/CTRLHD/ALT/NukeHD/ZeroTV/HotTV/EntTV/GameHD/SmY/SeeHD/VeryPSP/DWR/XLMV/XJCTV/Mp4Ba/GodDramas/FRDS/BeiTai/Ying/VCB-Studio 等31+组的资源
- **字幕组分类**：分类含"字幕组"(411)，为字幕资源专用

## 发布权限

| 用户等级 | 发布权限 |
|---------|---------|
| 任何注册用户 | 可发布资源（需先通过10次候选） |
| 通过10次候选后 | 可直接发布资源 |

## 分类映射 (type)

| ID | 名称 | 备注 |
|----|------|------|
| 401 | 电影 | |
| 402 | 剧集 | |
| 403 | 综艺 | |
| 404 | 纪录片 | |
| 405 | 动漫 | |
| 406 | 音乐 | |
| 407 | 体育 | |
| 409 | 其他 | |
| 411 | 字幕组 | 字幕资源专用 |

> **注意**：上传页面无 410(游戏) 分类，但审核脚本 `cat_constant` 中包含 410。

## 媒介映射 (medium_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 14 | Blu-ray | 蓝光原盘 |
| 8 | Remux | |
| 5 | BDRip | 蓝光重编码 |
| 4 | WEB-DL | 流媒体源码 |
| 7 | WEBRiP | 流媒体重编码 |
| 3 | HDTV | 数字电视 |
| 2 | TVRip | 数字电视重编码 |
| 1 | DVD | DVD原盘 |
| 11 | DVDRiP | DVD重编码 |
| 15 | CD | 音乐CD |
| 12 | Other | |

> **注意**：媒介 ID 非标准排序（Blu-ray=14, Remux=8, DVD=1），与多数 NexusPHP 站点不同。

## 视频编码映射 (codec_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 6 | H.265/HEVC | |
| 7 | H.264/AVC | |
| 2 | VC-1 | |
| 4 | MPEG-2 | |
| 5 | Other | |

> **注意**：无 AV1 编码选项。

## 音频编码映射 (audiocodec_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 10 | DTS-HD | |
| 12 | TrueHD | |
| 8 | LPCM | |
| 19 | DTS | ID=19 非标准 |
| 9 | AC-3 | |
| 11 | AAC | |
| 13 | FLAC | |
| 14 | APE | |
| 1 | WAV | |
| 6 | MP3 | |
| 7 | Other | |

> **注意**：DTS 的 ID=19 非常罕见（标准站通常 DTS=3）。DTS-HD/DTS 独立，无 Atmos/DTS:X 选项。

## 分辨率映射 (standard_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 6 | 2160p | 4K |
| 1 | 1080p | |
| 2 | 1080i | |
| 3 | 720p | |
| 4 | SD | 低于720p |
| 7 | Other | 超过4K或不适用 |

> **注意**：2160p=6, 1080p=1（ID 排序非递增也非递减）。无 8K/1440p 选项。

## 制作组映射 (team_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 17 | DYZ-WEB | DYZ 官组 |
| 15 | DYZ-Movie | DYZ 官组 |
| 14 | DYZ-TV | DYZ 官组 |
| 18 | Eleph | 官组 |
| 9 | beAst | 官组 |
| 11 | ZmWeb | 织梦官组 |
| 13 | Other | 非官组必选 |

> **注意**：非官方发布的种子禁止选择官方制作组，一律选择 Other。

## 标签系统 (tags[4][])

通过 checkbox 多选：

| ID | 名称 | 审核脚本检测 |
|----|------|-------------|
| 22 | 独家 | — |
| 17 | 驻站 | — |
| 1 | 禁转 | 标题含"禁转" |
| 2 | 首发 | — |
| 4 | DIY | — |
| 6 | 中字 | MI字幕语言/字幕区 Chinese 交叉验证 |
| 20 | 3D | — |
| 5 | 国语 | MI音频语言 Chinese 交叉验证 |
| 9 | 原盘 | 副标题含"版原盘" |
| 11 | REMUX | — |
| 14 | WEB-DL | — |
| 8 | DoVi | MI Dolby Vision 交叉验证 |
| 7 | HDR | MI HDR format 交叉验证 |
| 12 | SDR | — |
| 10 | 特效字幕 | — |
| 15 | 完结 | 副标题含"合集/集全/期全" |
| 16 | 分集 | — |
| 18 | 高码率 | — |
| 19 | 高帧率 | — |

## 自定义字段

| 字段名 | 含义 | 必填 | 说明 |
|--------|------|------|------|
| cover | 海报 | 是 | 图床地址，留空用豆瓣/IMDb海报 |
| screenshots | 截图 | 是 | 至少3张原始截图，一行一张 |
| technical_info | MediaInfo | 是 | 英文 MediaInfo/BDInfo |
| descr | 其它信息 | 否 | BBCode，致谢/补充信息 |

> **注意**：无独立的 url(IMDb) 字段。IMDb/豆瓣链接统一通过 pt_gen 字段提交。

## 标题命名规范

### 电影
```
名称.[剪辑版本].[年份].[区域版本].[发布说明].来源.分辨率.视频编码.音频编码.[音频数]-发布组名称
```
示例：`The Tunnel to Summer, the Exit of Goodbyes 2022 BluRay 2160p 10bit x265 HEVC TrueHD5.1 3Audio-Spade4K`

### 电视剧
```
名称.S**E**.[年份].[发布说明].分辨率.来源.视频编码.音频.[音频数]-发布组名称
```
示例：`Swallowed Star S01E139 2020 2160p WEB-DL H.265 AAC5.1-DYZWEB`

### 副标题
- **电影**：`中文片名 (剪辑版本) | 有关影片的有价值信息 [音轨信息] [字幕信息] 发布者附言`
- **电视剧**：`中文名 第X季 第X集`
- 开头必须是资源的中文名（推荐使用豆瓣中文译名）
- 禁止使用全角标点
- 禁止标榜压制组或压制个人

### 标题要求
- 必须全部英文
- 不得出现中括号 `[]`
- 可以使用空格或 `.` 隔开

## 简介规范

### PT-Gen 豆瓣信息（优先）
- 优先级：**豆瓣 > IMDb > 无**
- 内置豆瓣搜索代理（`douban-search.ptl.gs`），可一键检索
- 支持来源：IMDb / douban / bangumi / indienova

### 媒体信息
- **非蓝光原盘**：使用 MediaInfo（英文文本，完整信息）
- **蓝光原盘**：使用 BDInfo（英文文本，完整信息）
- 剧集选第一集的媒体信息
- 审核脚本检查：中文字符检测（`概览`/`概要`开头判定为中文 MI）、空格数量（<30 判定排版错误）

### 截图
- 至少3张原始截图（官组要求 PNG 原图）
- 转种可为缩略图
- 特效字幕种子必须包含至少2张反映特效效果的截图
- 白名单图床：files.ptlgs.org / cmct.xyz / ssdforum.org / gifyu.com / imgbox.com / pixhost.to / ptpimg.me

### 海报
- 必填，图床地址
- 支持一键转存官方图床
- 禁止使用防盗链图床（如 tu.totheglory.im）

## 转种总则

1. 不准转载禁转种
2. 转种种子必须符合资源规则
3. **禁止转载超分处理/补帧资源**
4. **禁止转载黑名单制作组的资源**
5. 不建议转载找不到出处的资源（可加 -NoGroup/-NoGrp，自担责任）
6. **禁止转载机翻资源**
7. 标题和副标题需符合本站规范（即使是转载也需修改）
8. 务必写明种子出处
9. 建议截图放到本站图床
10. 有些片子没有豆瓣收录可不生成豆瓣信息，有的必须生成

## 表单字段

### 提交URL
`POST https://ptlgs.org/takeupload.php`

### 必填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| file | file | 种子文件 |
| type | select | 分类(401-411) |
| name | text | 标题（不填用种子文件名） |
| pt_gen | text | 豆瓣/IMDb/Bangumi/Indienova 链接 |
| cover | text | 海报图床地址 |
| screenshots | textarea | 截图（至少3张，一行一张） |
| technical_info | textarea | MediaInfo/BDInfo（英文完整） |

### 选填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| small_descr | text | 副标题 |
| medium_sel[4] | select | 媒介 |
| codec_sel[4] | select | 视频编码 |
| audiocodec_sel[4] | select | 音频编码 |
| standard_sel[4] | select | 分辨率 |
| team_sel[4] | select | 制作组 |
| tags[4][] | checkbox[] | 标签（多选） |
| descr | textarea(BBCode) | 其它信息 |
| uplver | checkbox | 匿名发布(value="yes") |

### 质量字段联动
质量行通过 `data-mode='4'` 和 `relation="mode_4"` 控制显隐。

## 审核脚本完整逆向分析

### 脚本信息
- **名称**：ptlgs-Torrent-Assistant
- **来源**：https://userscripts.ptl.gs/PTLGS-Torrent-Assistant.user.js
- **版本**：1.1.43
- **大小**：830 行 / 37KB
- **运行页面**：`details.php*` / `offers.php*off_details*` / `torrents.php*`
- **权限**：GM_xmlhttpRequest / GM_setValue / GM_getValue

### 审核模式
- **普通用户模式**（isEditor=false）：基础检查，红色/绿色提示
- **种审员模式**（isEditor=true）：额外激进检查（灰色提示），需配合人工判断

### 信息提取

| 提取项 | 来源 | 提取方式 |
|--------|------|---------|
| 种子名称 | `h1#top > .name` | `.text()` |
| 副标题 | 主表格"副标题"行 | XPath `td` 遍历 |
| 分类/媒介/编码/音频/分辨率/制作组 | `b[title]` 标签 + `span` | `b[title="类型"]` → `next('span').text()` |
| 标签 | 主表格"标签"行 | 正则匹配文字 |
| MediaInfo | `pre` 元素 | `.textContent` 去空白 |
| 海报 | `#kposter img src` | `.attr('src')` |
| 截图 | `#ktorrentscreenshots img` | 遍历 `.attr('src')` |
| 豆瓣信息 | `div.douban-info h2 a` | 链接包含 `douban`/`imdb` |
| 字幕信息 | 主表格"字幕"行 | `img[title="简体中文"]` + 链接含 chs/cht |
| 其它信息 | `#kother` 或 `#kdescr` | `.html()` |
| 匿名 | 主表格"添加"行 | 包含"匿名" |
| 制作组标记 | `span[title="制作组"]` | 是否存在 |

### 标题解析算法

#### 媒介(type)检测（优先级从上到下）

| 优先级 | 匹配模式 | 结果 |
|--------|---------|------|
| 1 | `[.| ]remux` | Remux(8) |
| 2 | `[.| ]bdrip` 或 `bluray/blu-ray` + `x26[45]` | BDRip(5) |
| 3 | `bluray/blu-ray`（无 x264/5） | Blu-ray(14) |
| 4 | `[.| ]webrip` 或 `web[.| ]` + `x26[45]` | WEBRiP(7) |
| 5 | `web-dl/webdl/web[.| ]`（无 x264/5） | WEB-DL(4) |
| 6 | `[.| ]tvrip` | TVRip(2) |
| 7 | `[.| ]hdtv` | HDTV(3) |
| 8 | `[.| ]dvdrip` 或 `dvd` + `x26[45]` | DVDRiP(11) |
| 9 | `dvd`（无 x264/5） | DVD(1) |

#### 编码(codec)检测

| 匹配模式 | 结果 |
|---------|------|
| `x265/h265/h.265/hevc` | H.265/HEVC(6) |
| `x264/h264/h.264/avc` | H.264/AVC(7) |
| `vc-1/vc1` | VC-1(2) |
| `mpeg2/mpeg-2` | MPEG-2(4) |

#### 音频编码检测

| 匹配模式 | 结果 |
|---------|------|
| `dts-hd/dtshd/dts-x/dts:x` | DTS-HD(10) |
| `truehd` | TrueHD(12) |
| `lpcm/pcm` | LPCM(8) |
| `dts`（不含 dts-hd 等） | DTS(19) |
| `ac3/ac-3/ddp/dd+/dd2/dd5/dd.2/dd.5` | AC-3(9) |
| `aac` | AAC(11) |
| `flac` | FLAC(13) |

#### 分辨率检测

| 匹配模式 | 结果 |
|---------|------|
| `2160p` 或 `uhd`(无1080p) 或 `4k[.| ]`（排除 remastered） | 2160p(6) |
| `1080p` | 1080p(1) |
| `1080i` | 1080i(2) |
| `720p` | 720p(3) |
| 其它 | Other(7) |

> **注意**：`remastered` 被排除在 4K 检测之外（防止误判）。

#### 制作组检测

| 匹配模式 | 结果 |
|---------|------|
| `dyz-web` | DYZ-WEB(17) |
| `dyz-movie` | DYZ-Movie(15) |
| `dyz-tv` | DYZ-TV(14) |
| `beast` | beAst(9) |
| `zmweb` | ZmWeb(11) |

### 豆瓣分类判定算法

```
1. 获取豆瓣"类别"字段
   - 包含"真人秀" → isshow=true
   - 包含"纪录片" → isdoc=true
   - 包含"动画" → isani=true

2. 获取豆瓣"类型"字段（取第一个）
   - "电视剧"：
     - isshow → 综艺(403)
     - isdoc → 纪录片(404)
     - 否则 → 剧集(402)
   - 非电视剧：
     - isdoc → 纪录片(404)
     - 否则 → 其他(409)

3. 豆瓣检测分类 vs 用户选择分类交叉验证
```

### 中文字幕检测

```
1. 字幕区 img[title="简体中文"] 存在 → true
2. 字幕区第一个链接文本含 "chs" 或 "cht" → true
3. MediaInfo 含 "字幕.*Chinese" 或 "字幕.*Mandarin" 或 "Subtitle: Chinese" → true
4. 任一条件满足即判定 sub_chinese=true
```

### 中文音轨检测

```
1. 非DVD(type≠1)：MediaInfo 含 "音频:.*chinese.*字幕" → true
2. DVD(type=1)：MediaInfo 含 "Audio:\s?Chinese" → true
```

### 校验规则（普通用户模式）— 共20+项

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 标题包含中文 | `[\u4e00-\u9fa5\uff01-\uff60]` | 错误 |
| 2 | 标题含黑名单制作组 | `-(FGT\|NSBC\|BATWEB\|...)` 正则（31+组） | 错误 |
| 3 | 副标题为空 | `!subtitle` | 错误 |
| 4 | 未选择分类 | `!cat` | 错误 |
| 5 | 未选择媒介 | `!type` | 错误 |
| 6 | 标题媒介与选择不一致 | `title_type !== type` | 错误 |
| 7 | 未选择视频编码 | `!encode` | 错误 |
| 8 | 标题编码与选择不一致 | `title_encode !== encode` | 错误 |
| 9 | 未选择音频编码 | `!audio` | 错误 |
| 10 | 标题音频与选择不一致 | `title_audio !== audio` | 错误 |
| 11 | 未选择分辨率 | `!resolution` | 错误 |
| 12 | 标题分辨率与选择不一致 | `title_resolution !== resolution` | 错误 |
| 13 | 海报使用防盗链图床 | `tu.totheglory.im` | 错误 |
| 14 | 蓝光原盘用 MediaInfo 而非 BDInfo | `type===1 && MediaInfo` | 错误 |
| 15 | 媒体信息未解析 | 短格式 === 原始格式 | 错误 |
| 16 | 有中文字幕但未选"中字"标签 | `sub_chinese && !is_chinese` | 错误 |
| 17 | MI含Dolby Vision但未选"DoVi"标签 | MI匹配 + `!is_dovi` | 错误 |
| 18 | 选了"DoVi"标签但MI无Dolby Vision | `is_dovi && !MI匹配` | 错误 |
| 19 | MI含HDR但未选"HDR"标签 | MI匹配 + `!is_hdr` | 错误 |
| 20 | 选了"HDR"标签但MI无HDR | `is_hdr && !MI匹配` | 错误 |
| 21 | MI含HLG但未选"HLG"标签 | MI匹配 + `!is_hlg` | 错误 |
| 22 | 选了"HLG"标签但MI无HLG | `is_hlg && !MI匹配` | 错误 |
| 23 | 其它信息含非致谢内容（图片/◎符号）且无制作组标记 | `img/◎ + !span[制作组]` | 错误 |
| 24 | MediaInfo 格式错误（<30字符） | `mediainfo_s.length < 30` | 错误(含提示) |
| 25 | 标题检测到制作组但未选择 | `title_group && !group` | 错误 |
| 26 | 截图不足1张 | `imageCount < 1` | 错误 |
| 27 | 截图不在白名单图床 | 域名匹配 pichost_list | 错误 |
| 28 | 豆瓣分类与选择分类不一致 | `douban_cat !== cat` | 错误 |
| 29 | 选了"动画"标签但豆瓣无动画类别 | `is_anime && !isani` | 错误 |

### 校验规则（种审员模式）— 额外10+项

| # | 规则 | 检测方式 | 说明 |
|---|------|---------|------|
| E1 | 标题/副标题无"合集"但选了合集标签 | 正则 + `is_complete` | |
| E2 | MediaInfo 含中文（"概览"/"概要"开头） | 正则 | |
| E3 | 标题含 HDR 但 MI 无 HDR 元数据 | `.hdr./.hdr10.` + MI检查 | |
| E4 | 标题/副标题含"CC"但未选CC标签 | `Criterion/CC` 正则 | |
| E5 | 副标题含"版原盘"但未选"原生"标签 | 正则 | |
| E6 | BDInfo 含 "SUBtitleS:" 字段 | 正则 | |
| E7 | MediaInfo 空格字符过少（<30个双空格） | 正则 `(?!\\S)[ ]{2,}(?!\\S)` | 非蓝光原盘 |
| E8 | Dolby Vision P5 DUPE 参考 | `dvhe.05` | 不含HDR10 |
| E9 | Dolby Vision P7/P8 DUPE 参考 | `dvhe.08/dvhe.07` | 含HDR10 |
| E10 | 选了"中字"标签但未识别到中文字幕 | `!sub_chinese && is_chinese` | |
| E11 | WEB-DL 资源添加字幕后改后缀 | FLUX/HHWEB/HHCLUB + DIY | |
| E12 | 扫描方式为 Progressive 但分辨率选了 1080i | MI Progressive + resolution=3 | |
| E13 | 可替代：音频臃肿 | 高端音频+低分辨率+特定媒介 | |
| E14 | 可替代：无中字或硬字幕 | `!sub_chinese && !is_bd` | |
| E15 | 可替代：x264 10bit 兼容性差 | 标题或MI含x264+10bit | |

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题全部英文（移除中文、全角字符）
2. 确保无中括号 [] （本站规则禁止）
3. 按本站格式重构标题：
   - 电影：名称.年份.来源.分辨率.编码.音频-组名
   - 电视剧：名称.S**E**.年份.分辨率.来源.编码.音频-组名
4. 移除源站前缀标签（如 [馒头]、[HDArea] 等）
```

### 副标题自动处理

```
1. 开头必须是中文名（优先从 PT-Gen/豆瓣获取）
2. 格式：中文名 | 有价值信息 [音轨信息] [字幕信息]
3. 电视剧：中文名 第X季 第X集
4. 禁止标榜压制组
5. 禁止全角标点
```

### 质量字段自动选择

```
从源站标题解析：
1. 媒介：remux→8, bdrip/bluray+x264/5→5, bluray→14, webrip/web+x264/5→7,
         webdl→4, tvrip→2, hdtv→3, dvdrip/dvd+x264/5→11, dvd→1
2. 编码：hevc→6, avc→7, vc1→2, mpeg2→4
3. 音频：dts-hd→10, truehd→12, lpcm→8, dts→19, ac3/ddp→9, aac→11, flac→13
4. 分辨率：2160p/uhd/4k→6, 1080p→1, 1080i→2, 720p→3, SD→4
5. 制作组：dyz-web→17, dyz-movie→15, dyz-tv→14, eleph→18, beast→9, zmweb→11

注意 remastered 排除在 4K 检测之外
```

### 标签自动选择

```
1. 中字：源站 MI 字幕语言含 Chinese → tags[4][]=6
2. DoVi：源站 MI 含 Dolby Vision → tags[4][]=8
3. HDR：源站 MI 含 HDR format（排除 Dolby Vision Profile+） → tags[4][]=7
4. 国语：源站 MI 音频语言含 Chinese → tags[4][]=5
5. 原盘：媒介为 Blu-ray → tags[4][]=9
6. REMUX：媒介为 Remux → tags[4][]=11
7. 完结：副标题含"合集/集全/期全" → tags[4][]=15
8. 禁转：源站标题含"禁转" → tags[4][]=1
9. DIY：源站标题/副标题含 DIY → tags[4][]=4
```

### MediaInfo 处理

```
1. 必须英文（排除含"概览"/"概要"开头的中文 MI）
2. 非蓝光原盘用 MediaInfo，蓝光原盘用 BDInfo
3. 蓝光原盘使用 BDInfo 时检测不应出现 "SUBtitleS:" 字段
4. MediaInfo 双空格数量需 >= 30（排版检查）
```

### 豆瓣信息

```
1. 优先使用豆瓣链接（本站豆瓣优先建库）
2. 通过 pt_gen 字段提交豆瓣链接
3. 截图建议转存到本站官方图床
4. 海报禁止使用防盗链图床
```

---

*文档创建：2026-04-19*
*数据来源：upload.php (70490字节) + Wiki发布规则 (17301字节) + ptlgs-Torrent-Assistant.js v1.1.43 (830行/37KB)*
