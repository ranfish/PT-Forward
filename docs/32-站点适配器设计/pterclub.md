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

*数据来源: upload.php (92140字节) + Wiki上传规则 (12281字节) + Wiki DUPE规则 (11334字节) + 种子检查脚本 (185066字节) + 论坛规则 (112756字节)*
*文档创建: 2026-04-19*
