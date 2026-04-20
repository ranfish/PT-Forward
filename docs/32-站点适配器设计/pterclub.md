# 猫 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 猫 |
| 域名 | pterclub.net |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 否 |
| MediaInfo | 是（放入简介 descr，使用 `[hide=MediaInfo]` 包裹） |
| IMDb | 是（url 字段） |
| 豆瓣 | 是（douban 字段，独立字段） |
| 匿名发布 | 是（uplver） |
| NFO | 否（无独立字段） |
| PT-Gen | 是（外部工具，结果粘贴到简介） |
| 种子检查脚本 | 是（Greasyfork: pterclub-torrent-checker v1.0.22） |

## Tracker URL
`https://tracker.pterclub.net/announce`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 引用ID | `referid` | 否 | 转载引用 |
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | 0DAY 英文命名规范 |
| 副标题 | `small_descr` | 是 | 中文名 + 附加信息 |
| IMDb链接 | `url` | 是（非华语区） | |
| 豆瓣链接 | `douban` | 是 | 独立字段 |
| 简介 | `descr` | 是 | BBCode（WYSIBB编辑器） |
| 类型 | `type` | 是 | |
| 质量/来源 | `source_sel` | 是 | |
| 地区 | `team_sel` | 是 | 字段名为 team_sel 但实际含义是地区 |
| 禁转 | `jinzhuan` | 否 | checkbox |
| 官方 | `guanfang` | 否 | **disabled**（仅管理员） |
| 国语 | `guoyu` | 否 | checkbox |
| 粤语 | `yueyu` | 否 | checkbox |
| 中字 | `zhongzi` | 否 | checkbox |
| 英字 | `ensub` | 否 | checkbox |
| 应求 | `yingqiu` | 否 | checkbox |
| DIY原盘 | `diy` | 否 | checkbox |
| 原创 | `pr` | 否 | checkbox |
| 自购 | `bim` | 否 | checkbox |
| MV母盘 | `mp` | 否 | checkbox |
| 匿名发布 | `uplver` | 否 | |

### 缺失字段

- **无 medium_sel / codec_sel / standard_sel / audiocodec_sel / processing_sel** — 全站仅 3 个下拉框
- **无 NFO 独立字段**
- **无 PT-Gen 独立字段** — 结果粘贴到简介
- **无 MediaInfo 独立字段** — 粘贴到简介用 `[hide=MediaInfo]` 包裹
- **无 BDInfo 独立字段** — 粘贴到简介用 `[hide=BDInfo]` 包裹

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 纪录片 |
| 403 | 动画 |
| 404 | 电视剧 |
| 405 | 综艺 |
| 406 | 音乐 |
| 407 | 体育 |
| 408 | 电子书 |
| 410 | 软件 |
| 411 | 学习 |
| 412 | 其它 |
| 413 | 音乐短片 (MV) |
| 418 | 舞台演出 |

## 质量字段

### 质量/来源 source_sel（16 个）

| ID | 名称 | 适用 |
|----|------|------|
| 1 | UHD Discs | 4K UHD 蓝光原盘 |
| 2 | BD Discs | 标准蓝光原盘（含 3D） |
| 3 | Remux | Remux |
| 4 | HDTV | HDTV/UHDTV |
| 5 | WEB-DL | WEB-DL |
| 6 | Encode | 编码压制（BluRay Encode/DVDRip/WEBRip/HDTVRip 等） |
| 7 | DVD Discs | DVD 原盘 |
| 8 | FLAC | 音乐 FLAC |
| 9 | WAV | 音乐 WAV |
| 10 | ISO | 音乐 ISO |
| 11 | PDF | 电子书 PDF |
| 12 | PUB | 电子书 EPUB |
| 13 | AZW | 电子书 AZW3 |
| 14 | MOBI | 电子书 MOBI |
| 15 | Other | 其他 |

### 地区 team_sel（8 个）

| ID | 名称 | 覆盖范围 |
|----|------|---------|
| 1 | 大陆 | 中国大陆 |
| 2 | 香港 | 中国香港 |
| 3 | 台湾 | 中国台湾 |
| 4 | 欧美 | 美国/加拿大/欧洲/澳洲/新西兰等 80+ 国家 |
| 5 | 韩国 | 韩国 |
| 6 | 日本 | 日本 |
| 7 | 印度 | 印度 |
| 8 | 其它 | 阿联酋/约旦/尼日利亚/阿富汗/柬埔寨等 |

### 无编码/分辨率/音频编码下拉框

PterClub 是极简字段设计，所有技术细节通过标题和简介中的 MediaInfo 表达，不需要下拉选择。

## 标签（11 个 checkbox）

| name | 名称 | 说明 |
|------|------|------|
| `jinzhuan` | 禁转 | 官组资源或发布者自制资源 |
| `guanfang` | 官方 | **disabled**，仅管理组 |
| `guoyu` | 国语 | 含国语音轨 |
| `yueyu` | 粤语 | 含粤语音轨 |
| `zhongzi` | 中字 | 内嵌/内封/外挂中文字幕 |
| `ensub` | 英字 | 内嵌/内封/外挂英文字幕（非必填） |
| `yingqiu` | 应求 | 满足求种 |
| `diy` | DIY原盘 | 仅用于自制原盘 |
| `pr` | 原创 | 上传者原创资源（官组不勾） |
| `bim` | 自购 | 自购正版资源 |
| `mp` | MV母盘 | MV 母盘资源 |

## 标题命名规范

### 通用格式（原盘/REMUX）

```
片名(English) AKA原名 年份 S##E## 剪辑版 比例 HYBRID REPACK PROPER 分辨率 地区 介质 HDR DV Hi10P 视频编码 配音 音频编码 音频通道 音频对象 -制作组
```

### 通用格式（Encode/WEB-DL/HDTV）

```
片名 AKA原名 年份 S##E## 剪辑版 比例 HYBRID REPACK PROPER 分辨率 介质 HDR DV Hi10P 音频编码 音频通道 音频对象 视频编码 -制作组
```

### 各类型示例

**电影**:
```
Flying Colors 2015 1080p BluRay x265 10bit DTS 5.1-PTer
```

**电视剧**:
```
The Learning Curve Of A Warlord 2018 S01E01-E25 1080p WEB-DL H264 AAC 2Audios-PTer
```

**综艺**:
```
BTV New Year's Concert 20181231 1080p WEB-DL H264 AAC-PTerWEB
```

**MV**:
```
Fang Wu & Kevin Hsieh - Precious 2017 2160p WEB-DL VP9 Opus-PTerMV
```

**音乐**:
```
JJ Lin - Lost N Found 2011 - FLAC 16bit 44.1kHz - PTerMUSIC
```

**体育**:
```
ESPN NBA Playoffs 2020-2021 R1G1 20210524 LAL VS PHX 720p WEB-DL 60fps H264 AAC-PTer
```

**DVD原盘**:
```
The Dream of the Red Chamber 1962 NTSC DVD9
```

### 命名规则要点

1. **仅英文**：主标题只使用英文（音乐/电子书除外）
2. **空格分隔**：各部分用空格分隔，不用点号
3. **大小写敏感**：1080p（非 1080P）、HEVC（非 hevc）、x264（非 X264）、H265（非 h265）
4. **编码区分**：
   - 原盘/REMUX → AVC/HEVC/MPEG-2/VC-1
   - WEB-DL/HDTV → H264/H265/x264/x265/VP9/AV1
   - Encode → x264/x265/VP9/AV1/Xvid/DivX
   - MV → 使用 AVC/HEVC（例外）
5. **x264 vs H264 判断**：查 MediaInfo "Writing library" 字段含 "x264" 则用 x264，否则 H264
6. **禁止使用**：BDMV/BDISO/BDBOX/DVDISO → 替换为 Blu-ray/BluRay/DVD
7. **BDRip 必须改为 BluRay**（动漫常见错误）

### 分辨率标准

| 标识 | 最大尺寸 |
|------|---------|
| 2160p (4K) | 4096×2160 |
| 1440p (2K) | 2560×1440 |
| 1080p | 1920×1080 |
| 720p | 1280×720 |
| 576p | 1024×576 |
| 480p | 854×480 |

### HDR 标识

HDR10, HDR10+, DV, DV HDR10, DV HDR10+, HLG, PQ10, HDR Vivid

### 音频编码参考

**无损**: TrueHD, DTS:X, DTS-HD MA, LPCM(PCM), FLAC
**有损**: DTS-HD HR, AAC, AC3/DD, E-AC3/DD+/DDP, DTS, MP3, MP2, Opus, WMA

多音轨时选**最主要**的音轨（Default:Yes / 原始语言 / 最多声道 / 最佳编码 / 无损优先）

## 质量判定规则（来源：种子检查脚本逆向分析）

### 从标题和 MediaInfo 判定质量

| 条件 | 判定质量 |
|------|---------|
| 标题含 REMUX | Remux |
| 来源=Blu-ray + 编码=x264/x265/AV1 | Encode |
| 来源=Blu-ray + 制作组∈{FRDS,beAst,WScode,Dream,WiKi,CMCT,ANK-Raws,TLF,HDH,HDS} | Encode |
| 来源=WEB-DL + 制作组=FRDS | Encode |
| 来源=WEB-DL | WEB-DL |
| 来源=WEBRip | Encode |
| 来源=HDTVRip | Encode |
| 来源=HDTV | HDTV |
| 来源=DVDRip 或 编码=XviD/DivX | Encode |
| 来源=DVD + 文件含 .VOB/.ISO | DVD |
| MiniBD 标记 | Encode |

### 分类判定（按顺序，首次匹配）

| 优先级 | 分类 | 条件 |
|--------|------|------|
| 1 | 纪录片 | 简介类型含"纪录片" |
| 2 | 舞台演出 | 副标题含"演唱会" |
| 3 | 动画 | 简介类型含"动画" |
| 4 | 综艺 | 简介类型含"综艺/真人秀/脱口秀" |
| 5 | 电视剧 | 有集数 或 副标题含"短剧" |
| 6 | 电影 | 默认 |

### 地区判定（从简介"制片/产地/国家/地区"字段）

| 选择 | 匹配的简介文本 |
|------|--------------|
| 大陆 | 大陆, 中国, China |
| 香港 | 香港, Hong Kong |
| 台湾 | 台湾, Taiwan |
| 日本 | 日本, Japan |
| 韩国 | 韩国, Korean |
| 印度 | 印度, India |
| 欧美 | US/United States + 80+ 西方国家名 |
| 其它 | 阿联酋/约旦/尼日利亚等 ~20 国 |

### 音频语言检测（从 MediaInfo）

| MediaInfo 检测 | 判定 |
|---------------|------|
| Audio Title 含 "国语/普通话/国配/台配/Mandarin" | 国语 = true |
| Audio Title 含 "粤语/粵語/粤配/Cantonese" | 粤语 = true |
| Audio Language = Chinese/Mandarin | 国语 = true |
| Audio Language = Cantonese | 粤语 = true |

### 字幕检测（从 MediaInfo）

| MediaInfo 检测 | 判定 |
|---------------|------|
| Text Language 含 Chinese/Mandarin | 中字 = true |
| Text Title 含 "cht&eng/中英/chs&eng" | 中字 + 英字 = true |
| Text Language 含 English | 英字 = true |
| Text Title 含 "简繁/双语" | 中字 = true |

## DUPE 规则

| 场景 | DUPE? | 允许? |
|------|-------|-------|
| 同组 + 同分辨率 | 是 | 否 |
| 同组 + 同分辨率 + 增删字幕/NFO 改hash | 是 | 否 |
| 不同组 + 同分辨率 | 否 | 允许共存 |
| 同组 + 不同分辨率 | 否 | 允许共存 |
| 同组 + 同分辨率 + 不同编码 | 否 | 允许共存 |
| 不同区原盘（不同音轨/字幕） | 否 | 允许共存 |
| 充分保种完结 Web-DL + 同分辨率/编码/同源 | 是 | 否（Helper 裁量） |
| 同组 Repack 修复确认错误 | 旧版=DUPE | 新版替换旧版 |

### 死种重发条件（必须同时满足）

- 连续断种 ≥ 45 天
- 已发布 ≥ 6 个月

## 禁止内容

- SD 拉升视频（AI Upscale 需声明）
- CAM/TC/TS/SCR/DVDSCR/R5
- RMVB/RM/FLV
- 无正确 CUE 的整盘音频
- 9KG/色情/成人内容
- 损坏文件/垃圾文件
- 密码保护的压缩包
- 带论坛水印的宣传视频
- FGT/RARBG/Mp4Ba 等黑名单组
- 国家版权局重点保护预警名单作品

## 禁止图床

imgur.com, loli.net, ibb.co, ax1x.com, picgd.com, p.sda1.dev, gifyu.com, i.duan.red, z4a.net, helloimg.com, chdbits.co, ubitspho.top, ik.jcwsr.top, stonestudio2015.com, img.m-team.cc, cmct.xyz

## 推荐图床

1. s3.pterclub.net（站内图床）
2. pixhost.to
3. imgbox.com

## 简介要求

### 影视类（必须）

1. **海报** — 高清无水印
2. **简介** — 使用 PT-Gen 获取
3. **技术信息** — 原盘用 BDInfo，其他用 MediaInfo
4. **截图** — 强烈推荐

顺序：海报 → 简介 → MediaInfo/BDInfo → 截图

### 音乐类（必须）

1. 海报/封面
2. 简介（含曲目列表）
3. Log 或 频谱（至少一个，Log 优先）
4. 来源信息

### 体育类

- 技术信息必须
- 截图至少 3 张（不能含比赛结果）
- 海报/简介非必须

### BBCode 工具

WYSIBB 编辑器支持: `bold, underline, justifycenter, table, fontcolor, fontsize, fontfamily, img, video, link, code, hide, quote, hidemediainfo, hidebdinfo, comparison`

## 种子检查脚本逆向分析摘要

PterClub 使用 Greasyfork 脚本 "pterclub-torrent-checker" v1.0.22 进行种子质量检查。该脚本在 `details.php` 页面运行，执行以下检查：

### 检查项目（23 项通过条件）

1. 标题含来源（source）
2. 标题含视频编码（非 DVD）
3. 标题含分辨率（非 DVD）
4. 标题无 BDRip/BDMV/非 ASCII 字符
5. 标题无多余点号
6. 标题无连续空格
7. 标题片名不为空
8. 电影类标题含年份
9. 有 MediaInfo 或 BDInfo
10. 标题分辨率与 MI 一致
11. 标题视频编码与 MI 一致
12. 标题音频编码与 MI 一致
13. 标题音频通道与 MI 一致
14. 分类选择正确
15. 质量选择正确
16. 地区选择正确
17. 语言标签正确（国语/粤语）
18. 字幕标签正确（中字/英字）
19. DIY 标签正确
20. IMDb 链接非空且一致
21. 豆瓣链接非空且一致
22. 无黑名单图床
23. 无多余文件

### 制作组白名单（免部分文件检查）

FRDS, CMCT, EPiC, WiKi, TTG, QHstudIo, DBTV, CHD, HDH, PbK, MTeam, HDChina, Dream, TLF, BMDru, PuTao, GodDramas, OPS

### 编码组（Blu-ray 来源强制判为 Encode）

FRDS, beAst, WScode, Dream, WiKi, CMCT, ANK-Raws, TLF, HDH, HDS

### 合作组（标记提示）

AdBlue, AREY, BdC, BMDru, CatEDU, c0kE, Dave, doraemon, iFT, JKCT, KMX, Lislander, MZABI, nLiBRA, RO, Telesto, XPcl, ZTR, GodDramas

## 转载发布自动填写优化方案

### 1. 标题 (name) 自动构造

```
输入: 源站标题 + MediaInfo
输出: 0DAY 英文标题

规则:
1. 提取英文片名（去除中文）
2. 提取年份
3. 提取/从 MI 获取: 分辨率、来源、编码、音频
4. 判定编码名:
   - 原盘/REMUX → AVC/HEVC/MPEG-2/VC-1
   - WEB-DL/HDTV → H264/x264/H265/x265（查 MI Writing library 判 x/H）
   - Encode → x264/x265/AV1/Xvid
5. 格式:
   电影: {EnglishName} {Year} {Resolution} {Source} {VideoCodec} {AudioCodec} {AudioCh}-{Group}
   电视: {EnglishName} {Year} S{SS}E{EE}-E{EE} {Resolution} {Source} {VideoCodec} {AudioCodec} {AudioCh}-{Group}
   综艺: {Station} {EnglishName} {YYYYMMDD} {Resolution} {Source} {VideoCodec} {AudioCodec}-{Group}
   音乐: {ArtistEN} - {AlbumEN} {Year} - {Format} {BitDepth}bit {SampleRate}kHz -{Group}
6. 注意:
   - 空格分隔，不用点号
   - BDRip → BluRay
   - BDMV/BDISO → BluRay
   - DVDISO → DVD
```

### 2. 副标题 (small_descr) 自动构造

```
格式: {中文名} {季/集信息} {音轨信息} {字幕信息}
示例: 垫底辣妹 | 导演:土井裕泰 主演:有村架纯 [日语] [简繁英字+章节]
注意: 不使用全角标点
```

### 3. IMDb/豆瓣链接自动填写

```
IMDb: 从源站提取 → url 字段
豆瓣: 从源站提取 → douban 字段（独立字段！）
检查脚本会验证: 表格链接 == 简介中的链接
```

### 4. 简介 (descr) 自动构造

```
模板（影视类）:
[img]{海报URL}[/img]

[b]◎简　　介[/b]
{剧情简介}

[b]◎影片信息[/b]
{PT-Gen 生成的内容}

[hide=MediaInfo]{MediaInfo文本}[/hide]

[img]{截图1}[/img]
[img]{截图2}[/img]
[img]{截图3}[/img]

注意:
- 图床必须使用推荐图床，禁用黑名单图床
- MediaInfo 用 [hide=MediaInfo] 包裹
- BDInfo 用 [hide=BDInfo] 包裹
- 至少一个 IMDb 或豆瓣链接
```

### 5. 分类 (type) 自动选择

```
判定逻辑（按优先级）:
1. 简介类型含"纪录片" → 402(纪录片)
2. 副标题含"演唱会" → 418(舞台演出)
3. 简介类型含"动画" → 403(动画)
4. 简介类型含"综艺/真人秀/脱口秀" → 405(综艺)
5. 有集数 或 副标题含"短剧" → 404(电视剧)
6. 默认 → 401(电影)
```

### 6. 质量 (source_sel) 自动选择

```
从标题+MI 判定:
- UHD 原盘(BDMV+2160p+HEVC) → 1(UHD Discs)
- BD 原盘(BDMV+1080p+AVC) → 2(BD Discs)
- REMUX → 3(Remux)
- HDTV → 4(HDTV)
- WEB-DL → 5(WEB-DL)
- Encode(BluRay x264/x265/WEBRip/DVDRip) → 6(Encode)
- DVD 原盘(VOB/ISO) → 7(DVD Discs)
- 音乐 FLAC → 8(FLAC)
- 音乐 WAV → 9(WAV)
- 电子书 PDF → 11(PDF)
- 电子书 EPUB → 12(PUB)
- 电子书 AZW3 → 13(AZW)
- 电子书 MOBI → 14(MOBI)
```

### 7. 地区 (team_sel) 自动选择

```
从简介"制片/产地/国家/地区"字段匹配:
- 大陆/中国/China → 1(大陆)
- 香港/Hong Kong → 2(香港)
- 台湾/Taiwan → 3(台湾)
- 美国/US/Europe/Australia/... → 4(欧美) [80+国家]
- 韩国/Korean → 5(韩国)
- 日本/Japan → 6(日本)
- 印度/India → 7(印度)
- 其他 → 8(其它)
注意: 按制片地区选择，不按语言
```

### 8. 标签自动选择

```
从 MediaInfo 智能检测:
IF MI Audio 有 Chinese/Mandarin → 勾选 guoyu(国语)
IF MI Audio 有 Cantonese → 勾选 yueyu(粤语)
IF MI Text 有 Chinese → 勾选 zhongzi(中字)
IF MI Text 有 English → 可选勾选 ensub(英字)（非必须）
IF 原盘 DIY → 勾选 diy(DIY原盘)
转载时:
- 不勾选 jinzhuan(禁转) — 非原创
- 不勾选 pr(原创) — 转载
- 不勾选 bim(自购) — 非自购

地区+标签交叉验证:
- 香港地区 + 无国语/粤语标签 → WARNING
- 大陆/台湾地区 + 无国语标签 → WARNING
```

---

## 审核脚本完整逆向分析

### 脚本信息

| 项目 | 内容 |
|------|------|
| 名称 | PTerClub Torrent Checker |
| 来源 | Greasyfork #522428 |
| 版本 | 1.0.22 |
| 作者 | PTerClub-Helpers |
| 大小 | 2245 行 / 141KB |
| 运行页面 | `details.php?id=*`（详情页，支持 pterclub.com/net） |
| 权限 | GM_xmlhttpRequest / GM_setValue / GM_getValue |
| 外部连接 | greasyfork.org（版本检查） |

> **注意**：这是目前分析过的**最大审核脚本**（141KB），也是架构最独特的 — 使用结构化 TORRENT_INFO 对象存储所有解析数据，而非其他站的简单变量对比。

### 架构设计（与其他站完全不同）

```
TORRENT_INFO = {
    titleinfo:  { /* 从标题解析的所有字段 */ },
    tableinfo:  { /* 从页面表格提取的所有字段 */ },
    descrinfo:  { /* 从简介提取的所有字段 */ },
    mediainfo:  { /* 从 MediaInfo 解析的所有字段 */ },
    bdinfo:     { /* 从 BDInfo 解析的所有字段 */ },
    results:    { /* 综合判断后的最终结果 */ }
}
```

**核心差异**：不使用常量映射表（cat_constant/type_constant 等），而是通过 MediaInfo/BDInfo 深度解析 + 标题正则提取 + 简介豆瓣信息 三方交叉验证来确定质量/分类/地区。

### 白名单制作组

```
FRDS, CMCT, EPiC, WiKi, TTG, QHstudIo, DBTV, CHD, HDH, PbK, MTeam,
HDChina, Dream, TLF, BMDru, PuTao, GodDramas, OPS
```

### 禁止图床（15+ 个）

```
imgur.com, loli.net, ibb.co, ax1x.com, picgd.com, p.sda1.dev,
gifyu.com, i.duan.red, z4a.net, helloimg.com, chdbits.co,
ubitspho.top, ik.jcwsr.top, stonestudio2015.com, img.m-team.cc, cmct.xyz
```

> **gifyu 限制**：首图禁止使用 gifyu 图床（其他位置可以）。

### 标题解析算法（最复杂）

```
1. 获取 h1#top 文本，分离主标题和免费信息
2. 台标检测：CCTV-4K/8K, CHC, CWJDTV, Jade(粤语标签时)
3. REMUX 检测
4. 媒介(Source)检测（优先级）：
   Blu-ray → WEBRip → WEB-DL/WEB → HDTVRip → U?HDTV → DVDRip
   → DVD+PAL/NTSC → DVD+Remux
5. 视频编码检测：HEVC/AVC/x264/x265/H.264/H.265/Xvid/VC-1/MPEG-2/AV1/VP9/AVS2/AVS3/AVS+
6. 分辨率检测：480p-4320p → 8K→4K → 480i/576i/1080i
7. 音频对象检测：Atmos/DDPA
8. 标题拆分（以 Source 为界）：
   title[0] = 片名+年份+季数+集数+日期
   title[1] = 制作组+音频编码+音频通道+HDR+HQ+FPS
9. 音频编码检测（从 title[1]）：DTS-HD MA/HR/DDP/LPCM/DTS:X/MP2/EAC3/FLAC/TrueHD/AAC/OPUS → DTS/DD/PCM/AC3
10. 音频通道检测：X.Y 格式
11. 制作组检测：从 title[1] 以 - 分隔取最后段，特殊处理 ￡FRDS
12. 日期/季数/集数/片名/年份 逐步提取
13. 后置媒介检测：片名中含 WEB → WEB-DL
14. FPS/HDR(HDR10+/HDR Vivid/HDR10/HLG)/DV/10bit/MiniBD/3D 检测
```

### 质量判定算法（核心逻辑）

```
BDInfo 存在时：
  MiniBD → Encode
  BDInfo 2160p → UHD (Blu-ray)
  BDInfo 1080p → BD (Blu-ray)

MediaInfo 存在时（多级判断）：
  Remux → REMUX
  Blu-ray + x264/x265/AV1 → Encode
  Blu-ray + 白名单组(FRDS/beAst/WScode/Dream/WiKi/CMCT/...) → Encode
  WEB-DL + FRDS → Encode
  WEB-DL → WEB-DL
  WEBRip → Encode
  HDTVRip → Encode
  HDTV → HDTV
  DVDRip / Xvid/DivX → Encode
  DVD + VOB/ISO → DVD
```

> **关键差异**：猫站通过制作组白名单判断 Encode，而非简单依赖编码关键词。FRDS 等组即使标题写 Blu-ray 也判定为 Encode。

### 分类判定算法（从简介豆瓣信息）

```
1. 简介"类型"含"纪录片" → 纪录片
2. 副标题含"演唱会" → 舞台演出
3. 简介"类型"含"动画" → 动画
4. 简介"类型"含"综艺/真人秀/脱口秀" → 综艺
5. 简介有集数 / 副标题含"短剧" / 有分集 → 电视剧
6. 其他 → 电影
```

### 地区判定算法（从简介产地 vs 页面选择交叉验证）

```
从简介"制片/产地/国家/地区"字段提取产地，与页面"地区"下拉选择交叉验证：
- 大陆 ← 中国/中国大陆/China
- 香港 ← 香港/Hong Kong
- 台湾 ← 台湾/Taiwan
- 欧美 ← 60+欧美国家名
- 日本 ← 日本/Japan
- 韩国 ← 韩国/Korean
- 印度 ← 印度/India
- 其它 ← 阿联酋/约旦/泰国/苏联/南非/埃及/马来西亚/新加坡等
```

### MediaInfo 深度解析

#### 视频解析
```
Format → AVC/HEVC/AV1/VP9/VC-1/MPEG-2/AVS2/AVS3/CAVS
HDR Format → DV/HDR10+/HDR Vivid/HDR10/HLG
Transfer characteristics → PQ=HDR10, HLG
Bit rate → kbps 转换
Frame rate → 24/25/30/60/120 FPS
Width/Height → 像素级分辨率
Bit depth → 8/10 bit
Scan type → Interlaced/MBAFF → i 后缀
Writing library → x264/x265/XviD/DivX
Standard → NTSC/PAL
```

#### 分辨率推断（从宽高差值）
```
width - height > (4096-1248) → 4320p
width - height > (1920-672) 或 height==2160 → 2160p
width - height > (1280-480) 或 height==1080 → 1080p
width - height > (1024-520) 或 width∈(1260,1280] 或 height==720 → 720p
height ∈ (480,576] → 576p
height ∈ (350,480] → 480p
+ 扫描类型 → p/i 后缀
```

#### 音频解析（完整流解析，支持多音轨）
```
MLP FBA 16-ch → TrueHD Atmos
DTS XLL X → DTS:X 7.1
MLP FBA → TrueHD
DTS XLL/ES XLL → DTS-HD MA
DTS XBR → DTS-HD HR
DTS LBR → DTSE
E-AC-3 JOC → DDP Atmos
E-AC-3 → DDP
AC-3 → DD
PCM → LPCM
AV3A → AV3A
Opus → Opus
FLAC → FLAC
AAC → AAC
DTS → DTS
MPEG Audio + Layer 2/3 → MP2/MP3

Channel layout → 逐步计数 (L/R/C/LFE/Ls/Rs/Lb/Rb/Cb/Lss/Rss) → X.Y
音轨语言 Title/Language → 国语/粤语/Chinese/Cantonese/Mandarin
```

#### 字幕解析
```
Text Language Chinese/Mandarin → 中字
Text Language English → 英字
Title 含 cht&eng/中英/chs&eng/简*双语/繁*双语 → 中字+英字
```

### BDInfo 深度解析

```
Video 行 → AVC/HEVC/VC-1/MPEG-2 + kbps + resolution + HDR/HDR10+
DV 行 → Dolby Vision + kbps
Subtitle 行 → Chinese/Mandarin/English
Audio 行 → TrueHD Atmos/DTS-HD MA/DTS-HD HR/DTS/DDP/DD/LPCM
DIY 检测 → 副标题含 "DIY"
```

### 简介信息提取

```
片名/名字 → moviename
译名/又名/别名 → moviename（追加）
IMDb 链接 → imdburl
豆瓣链接 → doubanurl
制片/产地/国家/地区 → area（<30字符）
语言 → lang
集数 → chapters（纯数字）
类型/类别 → categorys（所有类别）
首映/上映日期/年代/年份 → publishdate（提取4位年份）
```

### 校验规则 — 共 40+ 项

#### 标题完整性校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 主标题缺少来源 | `titleinfo.source == ''` | 错误 |
| 2 | 主标题缺少视频编码 | `titleinfo.vcodec == ''`（DVD 豁免） | 错误 |
| 3 | 主标题缺少分辨率 | `titleinfo.resolution == ''`（DVD 豁免） | 错误 |
| 4 | 首图是 gifyu 图床 | 首图 URL 匹配 | 错误 |

#### 类型/质量/地区校验（"必有"系列）

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 5 | 必有 1：类型选择错误 | 豆瓣推断 vs 页面选择 | 错误 |
| 6 | 必有 2：质量选择错误 | MI/BDInfo 推断 vs 页面选择 | 错误 |
| 7 | 必有 3：地区不一致 | 简介产地 vs 页面选择 | 错误 |

#### 标题命名规范校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 8 | 标题含 BDRip/BDMV/中文 | 正则匹配（排除制作组） | 错误 |
| 9 | 标题含多余点号 | `.` in 残余标题 | 错误 |
| 10 | 音频通道错误 | 2.05.1 | 错误 |
| 11 | 音频对象错误 | TrueHD 非 7.1 + Atmos | 错误 |
| 12 | 制作组含空格 | 疑似含扩展名 | 错误 |
| 13 | 标题含多余括号 | `(.*?)` | 错误 |
| 14 | 主标题含连续空格 | `\s{2,}` | 错误 |
| 15 | 片名与简介不匹配 | 简介片名 vs 标题片名 | 错误 |
| 16 | 标题缺少年份（电影） | 电影 + `!year` | 错误 |
| 17 | 年份/季数/日期至少缺一个 | 全空 | 错误 |
| 18 | 季数不一致 | 标题 vs 副标题 | 错误 |
| 19 | 集数不一致 | 标题 vs 副标题 | 错误 |

#### MI/BDInfo 交叉验证

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 20 | 分辨率不一致 | 标题 vs MI/BDInfo | 错误 |
| 21 | 视频编码不一致 | 标题 vs MI/BDInfo | 错误 |
| 22 | 音频编码不一致 | 标题 vs MI/BDInfo | 错误 |
| 23 | 音频通道不一致 | 标题 vs MI/BDInfo | 错误 |
| 24 | HDR 不一致 | 标题 vs MI | 错误 |
| 25 | DV 信息缺失 | 标题有 DV 但 MI 无 | 错误 |
| 26 | 10bit 不一致 | 标题 vs MI | 错误 |
| 27 | FPS 不一致 | 标题 vs MI | 错误 |
| 28 | DVD 制式不一致 | 标题 vs MI Standard | 错误 |
| 29 | 媒介与质量不匹配 | Source vs Quality 交叉验证 | 错误 |
| 30 | 缺少 MediaInfo/BDInfo | 两者+infosp 全空 | 错误 |
| 31 | Info 中含有图片 | img 标签检测 | 错误 |

#### 标签交叉验证

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 32 | 粤语标签错误 | MI 粤语 vs 标签 | 错误 |
| 33 | 缺少粤语标签 | MI 有粤语但无标签 | 错误 |
| 34 | 港产片缺国语/粤语标签 | 地区=香港 | 错误 |
| 35 | 国语标签错误 | MI 国语 vs 标签 | 错误 |
| 36 | 缺少国语标签 | MI 有国语或地区=大陆/台湾 | 错误 |
| 37 | 缺少语言标签 | 地区=大陆/港/台但无国语/粤语 | 错误 |
| 38 | 中字标签缺失/错误 | MI 字幕 vs 标签（区分原盘/非原盘） | 错误 |
| 39 | 英字标签检查 | MI 英字 vs 标签 | 警告 |
| 40 | DIY 标签缺失/错误 | BDInfo DIY vs 标签 | 错误 |

#### 链接与内容校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 41 | IMDb 链接为空（非华语） | 非大陆/港/台地区 | 错误 |
| 42 | IMDb 链接不一致 | 页面 vs 简介 | 错误 |
| 43 | 豆瓣链接为空 | 页面为空 | 错误 |
| 44 | 豆瓣链接不一致 | 页面 vs 简介 | 错误 |

#### 文件与重复校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 45 | 错误文件数量 | 文件数 vs 集数（非原盘/非GodDramas） | 错误 |
| 46 | 包含多余文件 | 原盘:非clpi/mpls/m2ts/pad; DVD:非vob/iso/ifo/bup; 其他:非mkv/mp4等 | 错误 |
| 47 | 重复种子 | 相关资源表:大小+制作组+3D格式 | 错误 |
| 48 | BDInfo 码率为 0 | BDInfo video bitrates | 错误 |
| 49 | REPACK 种子 | 标题含 REPACK | 提示 |
| 50 | 黑名单图床 | 简介图片 URL 匹配 | 错误 |

### 合作组提示

```
AdBlue, AREY, BdC, BMDru, CatEDU, c0kE, Dave, doraemon, iFT, JKCT,
KMX, Lislander, MZABI, nLiBRA, RO, Telesto, XPcl, ZTR, GodDramas
```

> 这些组发布时脚本会显示"合作组"提示，但不一定报错。

### 单集豁免

```
副标题含"第X集/话/期"（非范围）→ error.push("不审核单集") → 脚本直接 return
```

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题无中文、无 BDRip/BDMV、无全角字符
2. 确保标题以空格分隔（非点号）
3. 确保包含来源(Blu-ray/WEB-DL/HDTV等)
4. 确保包含视频编码（DVD 豁免）
5. 确保包含分辨率（DVD 豁免）
6. 电影标题必须包含年份
7. 剧集标题必须包含季数（S01）或日期
8. REMUX 必须大写
9. Atmos 须配合 7.1
10. 4K→2160p, 8K→4320p
11. 移除多余括号、连续空格、多余点号
12. 制作组后缀不含空格
13. 台标前置：CCTV-4K/8K/CHC/CWJDTV/Jade(粤语)
14. 检测 REPACK 标记
```

### 副标题自动处理

```
1. 禁止为空（必填）
2. 副标题含集数范围 → 须在标题体现
3. 副标题含"演唱会" → 分类为舞台演出
4. 副标题含"短剧" → 分类为电视剧
5. 季数从副标题"第X季"提取（支持中文数字 1-25）
6. 优先从 PT-Gen/豆瓣获取中文名
```

### 质量字段自动选择

```
通过 MI/BDInfo 深度解析确定（非简单标题匹配）：
1. 质量(zhiliang)：
   UHD(BDInfo 2160p) → BD(BDInfo 1080p) → Encode(MiniBD/Blu-ray+x264/x265)
   → Encode(Blu-ray+白名单组) → Encode(WEB-DL+FRDS) → WEB-DL
   → Encode(WEBRip) → Encode(HDTVRip) → HDTV → Encode(DVDRip) → DVD
   → REMUX(Remux标记)
2. 来源(source)：
   Blu-ray → WEBRip → WEB-DL → HDTVRip → HDTV → DVDRip → DVD
3. 编码(vcodec)：
   原盘: AVC/HEVC/VC-1/MPEG-2
   Encode: x264/x265/AV1/VP9/VC-1/AVS2/AVS3/AVS+/XviD
   特殊: MPEG-2/AV1/VP9/VC-1 直接用格式名
4. 分辨率：从 MI width-height 差值推断 + scan type → p/i
5. 音频：从 MI Audio 流逐一解析 format+channels+object+language
```

### 标签自动选择

```
1. 国语：MI/BDInfo 音频含 Chinese/Mandarin 或标题含国配/普通话 + 非大陆地区
2. 粤语：MI/BDInfo 音频含 Cantonese 或标题含粤语/粤配
3. 中字：MI 字幕含 Chinese + 非原盘时地区=大陆/港/台也触发
4. 英字：MI 字幕含 English
5. DIY：副标题含 DIY + 原盘
6. 语言标签必选：地区=大陆/港/台 必须有国语或粤语
7. 字幕检查：无任何字幕信息时提示检查
```

### MediaInfo/BDInfo 处理

```
1. 原盘用 BDInfo，非原盘用 MediaInfo（强制）
2. 两者+infosp 全空 → 错误
3. Info 中禁止包含图片
4. BDInfo 码率不能为 0
5. 支持多种 MediaInfo 格式：标准/中文/FRDS NFO/CMCT NFO/TLF NFO 等
6. 支持多种 BDInfo 格式：Disc Title/Disc Label/DISC INFO
7. 支持非标准 info（infosp）：小组发布信息/General Information 等
```

### 链接处理

```
1. IMDb 链接：非华语片必填，页面与简介须一致
2. 豆瓣链接：必填（GodDramas 豁免），页面与简介须一致
3. 两者都为空时 → 错误
4. IMDb 豁免条件：地区=大陆/台湾/香港
```

### 文件结构检查

```
原盘(BD/UHD)：仅允许 .clpi/.mpls/.m2ts（BDMV 目录内）
DVD：仅允许 .vob/.iso/.ifo/.bup
其他：仅允许 .mkv/.mp4/.vob/.m2ts/.ts/.avi/.mov/.nfo/.md5
白名单组额外允许：.jpg/.png/.txt/.ass
文件数量须与集数匹配（非原盘/非GodDramas）
```

---

*数据来源: upload.php (92140字节) + Wiki上传规则 (12281字节) + Wiki DUPE规则 (11334字节) + 种子检查脚本 PTerClub Torrent Checker v1.0.22 (2245行/141KB) + 论坛规则 (112756字节)*
*文档更新: 2026-04-19*
