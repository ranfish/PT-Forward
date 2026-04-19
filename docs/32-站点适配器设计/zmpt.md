# 织梦 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 织梦 |
| 域名 | zmpt.cc |
| 框架 | NexusPHP |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（部分用户需先候选，游戏类仅上传员可自由上传） |
| 种审制 | 是（种子发布后需审核通过） |
| 站点角色 | 源站 + 发布站 |
| MediaInfo | 是（简介中fieldsets，系统自动解析） |
| IMDb | 是（url字段 + IMDb信息行自动解析） |
| 豆瓣 | 是（PT-Gen简介中） |
| PT-Gen | 是（url + pt_gen双字段，按钮获取简介） |
| 匿名发布 | 是（checkbox） |
| NFO | 否（简介BBCode） |
| Bangumi | 是（PT-Gen支持） |
| 建站时间 | 2010年 |

## Tracker URL
`https://zmpt.cc/announce.php`

## 站点特色

- **种审制**：种子发布后需审核通过，配合第三方Greasyfork审核协助工具使用
- **宝可梦主题**：用户等级命名为训练家（传说训练家/道馆训练家等）
- **电力值经济系统**：魔力值称为"电力值"，支持银行/借贷/双色球/快三/21点等
- **认领系统**：每种最多5人认领，每人最多2000种，达标魔力2倍
- **全站随机促销**：100%概率免费（2天），10%概率免费&2x上传（1天），>25GB自动免费
- **站组体系**：ZmWeb/ZmPT/ZmMusic/ZmAudio/DYZ-Movie/GodDramas/RL(RL4B)
- **9KG/成人内容禁止**：本软件禁止下载和发布此类内容
- **单种限速100MB/s**

## 发布权限

| 用户等级 | 发布权限 |
|---------|---------|
| 任何注册用户 | 可发布资源（部分需先候选） |
| 上传员及以上 | 游戏类可自由上传 |
| 其他用户 | 游戏类需先候选 |

## 分类映射 (type)

| ID | 名称 | 备注 |
|----|------|------|
| 401 | 电影 / Movies | |
| 402 | 电视剧 / TV Series | 需要完结/分集标签 |
| 403 | 综艺 / TV Shows | |
| 417 | 动漫 / Anime | |
| 422 | 纪录片 / documentary | |
| 423 | 音乐 / Music | 歌手字段 |
| 424 | 有声书 / Audiobook | |
| 425 | 软件 / Software | |
| 426 | 游戏 / Game | 仅上传员可自由上传 |
| 427 | 短剧 / Short Play | |
| 409 | 其他 / Misc | |

## 媒介映射 (medium_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | Blu-ray | 蓝光原盘 |
| 2 | HD DVD | |
| 3 | Remux | |
| 4 | MiniBD | |
| 5 | HDTV | |
| 6 | DVDR | |
| 7 | Encode | 重编码 |
| 8 | CD | 音乐CD |
| 10 | WEB-DL | |
| 12 | 其他资料 | |

## 分辨率映射 (standard_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | 1080p/1080i | 合并选项 |
| 5 | 4K/2160p | |
| 6 | 2K/1440p | |
| 7 | 480p | |
| 8 | 720p | |
| 9 | 8K/4320p | |

> **注意**：分辨率ID非标准排序（1080p=1, 4K=5, 720p=8, 480p=7），与多数NexusPHP站点不同。

## 音频编码映射 (audiocodec_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | FLAC | |
| 2 | APE | |
| 3 | DTS | |
| 4 | MP3 | |
| 5 | OGG | |
| 6 | AAC | |
| 7 | Other | |
| 8 | AC3 | |
| 9 | ALAC | |
| 10 | WAV | |

> **注意**：音频编码ID排序非字母序（AC3=8, AAC=6），与标准NexusPHP不同。

## 制作组映射 (team_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 5 | Other | |
| 6 | ZmPT | 织梦官组 |
| 7 | ZmWeb | 织梦WEB官组 |
| 8 | ZmMusic | 织梦音乐官组 |
| 9 | DYZ-Movie | DYZ电影组 |
| 10 | GodDramas | GodDramas组 |
| 11 | ZmAudio | 织梦音频官组 |
| 12 | RL/RL4B | RL组 |

## 自定义字段

| 字段名 | 含义 | 分类 |
|--------|------|------|
| custom_fields[4][1] | 歌手 | 音乐类 |

## 标签系统

标签通过checkbox选择（非标准tags[]数组），审核脚本中识别的关键标签：

| 标签名 | 检测逻辑 | 来源 |
|--------|---------|------|
| 国语 | 与MediaInfo音频语言/简介语言交叉验证 | 简介◎语言 + MI Audio Language |
| 中字 | 与MediaInfo Text字幕语言交叉验证 | MI Text Language |
| 杜比 | 与MediaInfo Dolby Vision信息交叉验证 | MI HDR format/Format profile |
| HDR | 与MediaInfo HDR信息交叉验证 | MI HDR format/Transfer characteristics |
| 纯享版 | 仅警告提示人工检查 | — |
| 完结 | 电视剧必需（与分集互斥） | 副标题文字 |
| 分集 | 电视剧必需（与完结互斥） | — |
| DIY | 与种子名称/副标题DIY关键词交叉验证 | 标题+副标题+文件扩展名 |
| 原盘 | 与种子名称Blu-ray/文件结构交叉验证 | 标题+文件列表(.mkv) |

> **注意**：电影不允许包含"完结"或"分集"标签。

## 标题命名规范

### 电影/演唱会
```
英文或者拼音名称.[剪辑版本].[年份].[发布说明].来源.分辨率.视频编码.[音频编码]-后缀
```
示例：`The Witch 2015 COMPLETE UHD BLURAY-TERMiNAL`

### 电视剧/动漫
```
英文或者拼音名称.S**E**.[年份].[发布说明].分辨率.来源.视频编码.[音频]-后缀
```
示例：`DARK S01 2017 1080p Netflix WebDL H264 - PTDream`

### 副标题
- **电影**：`中文片名 (剪辑版本) | 有价值信息 | [音轨信息] [字幕信息] 发布者附言`
- **电视剧**：`中文名 第X季 第X集 | 发布者留言`
- 副标题开头必须是资源的中文名

### 标题校验规则（审核脚本提取）

| 规则 | 检测内容 |
|------|---------|
| 中文检查 | 标题不能包含中文或全角字符 |
| 分辨率格式 | `1080P`应改为`1080p`（P小写） |
| 4K格式 | `4K`应改为`2160p` |
| 空格数量 | 标题中空格不能少于5个 |
| HDR格式 | `hdr`应改为`HDR`（全大写） |
| 年份检查 | 必须包含4位年份（19xx或20xx） |
| 年份/版本顺序 | 年份应在版本标记（PROPER/REPACK等）前面 |
| 视频编码检查 | 必须包含有效视频编码（x264/x265/H.264/H.265/AVC/HEVC/VC-1/AVS2/AVS3等） |

### 禁止发布
- 来自网盘/BT的公网资源
- 未被IMDb/豆瓣收录的影片
- 禁转资源
- FGT/RARTV/RARBG/MP4BA/DreamHD/DDHDTV等公网盗窃组资源

## 简介规范

### 必须包含
1. **海报/封面**（必须）
2. **演职员名单 + 剧情概要**
3. **MediaInfo或BDInfo信息**（必须正确，系统自动解析）
4. **画面截图**（尽量包含）

### 简介格式模板
```
[海报图片]

◎译　　名　xxx
◎片　　名　xxx
◎年　　代　2023
◎产　　地　中国大陆
◎类　　别　剧情 / 悬疑
◎语　　言　汉语普通话
◎上映日期　2023-08-25
◎IMDb评分　6.0/10
◎IMDb链接　https://www.imdb.com/title/ttxxxx/
◎豆瓣评分　6.2/10
◎豆瓣链接　https://movie.douban.com/subject/xxxxx/
◎片　　长　112分钟
◎导　　演　xxx
◎演　　员　xxx
◎简　　介　xxx

[引用] MediaInfo/BDInfo [/引用]

[截图]
```

### 图片要求
- 简介图片不能少于2张
- 图床推荐：image.zmpt.cc

## 表单字段

### 提交URL
`POST https://zmpt.cc/takeupload.php`

### 必填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| file | file | 种子文件 |
| name | text | 标题（不填用种子文件名） |
| descr | textarea(BBCode) | 简介 |
| type | select | 分类(401-426) |

### 选填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| small_descr | text | 副标题 |
| url | text | IMDb链接 |
| pt_gen | text | PT-Gen链接 |
| medium_sel[4] | select | 媒介 |
| standard_sel[4] | select | 分辨率 |
| audiocodec_sel[4] | select | 音频编码 |
| team_sel[4] | select | 制作组 |
| custom_fields[4][1] | text | 歌手（音乐类） |

### 质量字段联动
质量行通过`data-mode`和`relation`属性控制显隐，分类选择`type`时通过JS控制`tr[relation=mode_4]`的显示。

## 种子审核协助工具逆向分析

### 工具信息
- **名称**：zmpt-check-tool
- **来源**：Greasyfork #552769
- **作者**：anynopt
- **版本**：2026.03.08
- **运行页面**：`https://zmpt.cc/details.php?id=*`
- **权限**：GM_xmlhttpRequest

### 审核流程

工具在种子详情页运行，自动提取以下信息并进行校验：

#### 1. 信息提取

| 提取项 | 来源 | 提取方式 |
|--------|------|---------|
| 种子名称 | `h1#top` | `childNodes[0].textContent` |
| 官方组名 | 种子名称末尾 | `slice(-10).split('-').pop()` |
| 副标题 | 主表格"副标题"行 | XPath |
| 基本信息 | 主表格"基本信息"行 | 正则解析 大小/类型/视频类/分辨率/音频类 |
| 标签 | 主表格"标签"行 | `span`元素数组 |
| 简介内容 | 主表格"简介"行 | fieldsets解析 |
| MediaInfo | 简介中fieldset | 按General/Video/Audio/Text分段解析 |
| DiscInfo | 简介中fieldset | 按DISC INFO/VIDEO/AUDIO/SUBTITLES分段解析 |
| IMDb信息 | 主表格"IMDb信息"行 | XPath |
| 发布者 | 主表格 | XPath `userdetails.php?id=` |
| 文件列表 | `viewfilelist.php` API | GM_xmlhttpRequest |

#### 2. 校验规则（17项）

| # | 规则 | 校验方式 | 通过条件 |
|---|------|---------|---------|
| 1 | 种子来源官组 | 标题末尾`-`后提取 | 显示官组名 |
| 2 | 种子标题格式 | 正则检测 | 无中文+有分辨率+有年份+有编码+空格≥5 |
| 3 | 分辨率选择校验 | 标题解析 vs 用户选择 | 两者匹配 |
| 4 | 转载来源检查 | fieldset引用 vs siteSourcesMap | ZM官组无需引用/国内站需声明 |
| 5 | MediaInfo/DiscInfo校验 | fieldset分类解析 | 至少存在一种 |
| 6 | 中字标签校验 | MI Text Language vs 标签 | 标签与MI一致 |
| 7 | 杜比标签校验 | MI HDR format含"Dolby Vision" vs 标签 | 标签与MI一致 |
| 8 | HDR标签校验 | MI HDR关键词/Transfer characteristics vs 标签 | 标签与MI一致 |
| 9 | 简介图片数量 | 简介中`<img>`元素 | ≥2张 |
| 10 | 简介图片可访问性 | `img.complete` + `naturalWidth` | 全部可访问 |
| 11 | 电视剧分集/完结标签 | 类型=电视剧时检查标签 | 必须含"完结"或"分集" |
| 12 | 豆瓣&IMDb校验 | 简介文本正则匹配链接 | 至少有一种链接 |
| 13 | 电影标签检测 | 类型=电影时检查 | 不含"完结"或"分集" |
| 14 | 纯享版标签提示 | 标签含"纯享版" | 仅警告 |
| 15 | 国语标签检测 | MI Audio Language/简介◎语言/副标题 vs 标签 | 标签与来源一致 |
| 16 | 年代检测 | 简介◎年代 vs IMDb信息年代 | 显示两者 |
| 17 | 原盘/DIY标签检测 | MI/DiscInfo+标题Blu-ray+文件列表 vs 标签 | 标签与来源一致 |

#### 3. HDR检测逻辑（深度）

```
检测优先级：
1. MI Video "HDR format" 字段含关键词
2. MI Video "Format profile" 含 "Dolby Vision"
3. MI Video "Transfer characteristics" 含 PQ/HLG/ST 2084
4. MI Video HDR元数据字段（MaxCLL/MaxFALL等）
5. BT.2020 + 10bit 组合特征

HDR关键词：HDR Vivid, HDR10, HDR10+, Advanced HDR, HLG, Dolby Vision, SMPTE ST 2086
```

#### 4. 中文音频检测逻辑

```
音频语言标识（按优先级）：
Chinese, Mandarin, 国语, 普通话, 中文, Mandarin (CN), Chinese (CN), Chinese (Simplified)
检查字段：Audio.Language, Audio.language, Audio.Title, Audio.title
```

#### 5. 中文字幕检测逻辑

```
字幕语言标识：
Simplified Chinese, Chinese, 简体中文, 繁体中文, 繁体, 简体, 简体中字,
中文简体, 中文（简体）, Chinese Simplified, Simplified, Chinese (Simplified), Mandarin
检查字段：Text.Language, Text.language, Text.Title, Text.title
```

#### 6. 转载来源检测（siteSourcesMap）

| 站点名 | 官组别名 |
|--------|---------|
| 天枢 | dubheweb |
| 好学 | hxweb |
| 幸运PT | LuckAni, luckdiy |
| 财神 | csweb |
| 回声 | hsweb |
| 柠檬 | LeagueWEB, lhd |
| 蟹黄堡 | crab |
| 青蛙 | FROGWeb |
| 末日 | agsvweb |
| 麒麟 | hdkweb, kylin |
| 肉丝 | rousiweb |
| 熊猫 | ailmweb, panda |
| U堡 | ubits, ubweb |
| 不可梦 | zmweb |
| 憨憨 | hhweb |
| 车站 | carpt |
| 人人 | ADWeb, ADE |
| 不可比 | qhstudio, dbtv |
| 猫站 | pterweb, ptertv |
| 我堡 | ourbits, ourtv, ilovetv |
| 家园 | hdhome, hdhweb |
| 月月 | frds |
| 馒头 | mweb, mteam |
| 不可说 | cmct, CMCTV |
| TTG | ttg |
| U2 | u2 |
| 星陨阁 | starfallweb |
| 天空 | hdsky, hdsweb |
| 彩虹岛 | chdbits, chdweb |
| 猪猪 | pigoweb |
| 朱雀 | zwex |
| 52pt | 52pt |
| 下水道 | SewageWeb |
| 北洋园 | tjupt |
| 吐鲁番 | TLF |
| 铂金家 | PTHome, PTHWeb, PTH |
| 时光 | HDT, HDTime |
| 日日 | Sunny |
| 幸运 | LuckWeb, LuckDIY |
| 自然 | NatureWeb |
| 火鸟 | ZWEX |
| 劳改所 | DYZ-WEB, DYZ-Movie, DYZ-TV |
| 兽 | beAst |
| 学校 | BTS, BTSchool |
| 白兔俱乐部 | Hares, HaresWeb, HaresTV |
| 备胎 | BeiTai |
| 烧包乐园 | FFansDIY@至尊宝, FFansWeb, FFansTV |
| 瓷器 | HDChina, HDC, HDCTV |
| 蝶粉 | DiscFan |
| 爱好多 | DIY@iHDBits, iHDBits, iHD |
| 好多油 | HDU, DIY@HDU |

#### 7. 批量审核功能

- "启动吧，电力！"按钮：获取未审核种子列表（`torrents.php?approval_status=0`），批量打开详情页
- "一键通过"按钮：自动选择通过（value=1）并提交
- "一键拒绝"按钮：自动选择拒绝（value=2）并聚焦备注框
- 回车键快速审核支持

#### 8. 分辨率解析逻辑

```
从标题提取分辨率 → 映射到standard_sel ID：
480p/480i → 7(480p)
720p/720i → 8(720p)
1080p/1080i → 1(1080p/1080i)
4K/2160p → 5(4K/2160p)
8K/4320p → 9(8K/4320p)
其他 → 6(2K/1440p)
```

## 转载发布自动填写优化方案

### 1. 标题自动填写

```
源标题解析 → 重构为织梦标题格式：
电影：[英文名] [年份] [剪辑版本] [发布说明] [来源] [分辨率] [视频编码] [音频编码]-[组名]
电视剧：[英文名] S**E** [年份] [发布说明] [分辨率] [来源] [视频编码] [音频编码]-[组名]

清洗规则：
- 去除中文/全角字符
- 4K → 2160p
- 1080P → 1080p（P小写）
- hdr → HDR（全大写）
- 确保年份在版本标记前
- 空格分隔，无下划线
```

### 2. 副标题自动填写

```
提取源站中文标题 + 豆瓣中文名 → 副标题
格式：[中文名] | [有价值信息] | [音轨信息] [字幕信息]

音轨信息：从MediaInfo Audio提取语言列表
字幕信息：从MediaInfo Text提取语言列表
```

### 3. 分类自动选择

```
标题/源站分类 → 织梦type ID映射：
Movie/Film → 401
TV Series/Drama → 402
Anime/Animation → 417
Documentary/Doc → 422
Music → 423
TV Show/Variety → 403
Game → 426
Audiobook → 424
Software → 425
Short Play/短剧 → 427
Other → 409
```

### 4. 质量字段自动选择

```
媒介：标题关键词映射 → medium_sel[4]
Blu-ray/Blu-ray/BDMV → 1(Blu-ray)
Remux → 3(Remux)
WEB-DL/WebDL/Web-DL → 10(WEB-DL)
HDTV → 5(HDTV)
Encode/x264/x265/HEVC/H.264/H.265 → 7(Encode)
DVD/DVDR → 6(DVDR)
CD → 8(CD)

分辨率：标题关键词映射 → standard_sel[4]
4K/2160p/UHD → 5(4K/2160p)
1080p/1080i/FHD → 1(1080p/1080i)
720p/HD → 8(720p)
480p/SD → 7(480p)
8K/4320p → 9(8K/4320p)
1440p/2K → 6(2K/1440p)

音频编码：MediaInfo Audio Format映射 → audiocodec_sel[4]
FLAC → 1
APE → 2
DTS → 3
AC-3/AC3/DD → 8
AAC → 6
MP3 → 4
OGG → 5
ALAC → 9
WAV → 10
Other → 7

制作组：标题末尾组名映射 → team_sel[4]
ZmWeb/ZmPT/ZmMusic/ZmAudio → 对应ID
DYZ-Movie → 9
GodDramas → 10
RL/RL4B → 12
其他 → 5(Other)
```

### 5. IMDb链接自动填写

```
源站IMDb链接 → url字段
格式：https://www.imdb.com/title/ttXXXXXXX/
```

### 6. PT-Gen自动填写

```
源站PT-Gen链接 → pt_gen字段
支持来源：IMDb/豆瓣/Bangumi/indienova
```

### 7. 简介自动构建

```
1. PT-Gen获取影片信息（海报/演职员/剧情）
2. 格式化为织梦标准简介模板（◎译名/片名/年代/产地...）
3. 保留源站MediaInfo/BDInfo的fieldset引用块
4. 保留源站截图
5. 转载声明：添加"观组作品"或"感谢原制作者发布"引用
6. 确保图片≥2张
```

### 8. 标签自动选择

```
基于MediaInfo + 标题 + 副标题自动判断：

中字标签：MI Text Language含Chinese/Simplified Chinese → 选中字
国语标签：MI Audio Language含Chinese/Mandarin/国语 → 选国语
HDR标签：MI Video含HDR format/HDR10/Dolby Vision/HDR Vivid → 选HDR
杜比标签：MI Video含Dolby Vision → 选杜比
原盘标签：标题含Blu-ray且文件列表无.mkv → 选原盘
DIY标签：标题/副标题含DIY → 选DIY
完结标签：电视剧+副标题含"完结"/"全" → 选完结
分集标签：电视剧+非完结 → 选分集
```

## H&R规则

- **已取消全站H&R**
- 官种默认实行H&R考察
- 非官种随机进行H&R考察
- H&R标准：336小时(14天)内做种满72小时 或 分享率>1
- 消除规则：10000魔力值消除1条，累计10条未达标封禁

## 认领规则

- 种子发布后即可认领
- 每种最多5个用户认领
- 每人最多认领2000个种子（当前显示为1000）
- 达标标准：每月做种≥300小时，或上传量≥体积2倍
- 达标种子魔力奖励为正常2倍
- 不达标扣除600魔力值（非首月）
- 主动放弃扣除300魔力值

## Dupe规则

- 来源媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 同媒介同分辨率按发布组优先级判定
- 旧种断种45天或发布18个月以上可重发
- 动漫类HDTV和DVD优先级相同（特例）
- 无损音轨只保留一个版本，分轨FLAC优先级最高

## 上传规则要点

### 允许的资源
- 高清视频（≥720p标准）
- 标清视频（≥480p标准，来源于高清媒介）
- DVDR/DVDISO/DVDRip/CNDVDRip
- 无损音轨（FLAC/APE等 + cue表单）
- 5.1声道或以上音轨
- PC游戏（必须原版光盘镜像）
- 7日内高清预告片
- 高清相关软件和文档

### 不允许的资源
- 总体积<100MB（高清软件/文档/单曲专辑除外）
- 标清upscale视频
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD等
- RMVB/RM/FLV
- 有损MP3/WMA（<5.1声道）
- RAR压缩文件
- 损坏文件/垃圾文件
- 禁转资源

### 打包规则
- 整季电视剧/综艺/动漫
- 同套装电影合集
- 同一艺术家MV（≥5张专辑）
- 7日内预告片合集

## SeedBox规则

- 所有非家用网络客户端视为盒子
- 必须主动报备
- 未报备可能警告
- 已报备盒子：不享受优惠，发布48小时内3倍上传量

## 账号保留

| 等级 | 保留条件 |
|------|---------|
| 传说训练家(NM)及以上 | 永远保留 |
| 道馆训练家(IU)及以上 | 封存后不删除 |
| 封存账号 | 180天不登录封禁 |
| 未封存账号 | 45天不登录封禁 |
| 无流量账号 | 7天不登录封禁 |

## Cloudflare注意事项

- 需要有效的`cf_clearance` cookie
- 建议使用`curl -k --tlsv1.2`绕过TLS指纹
- 参见 `docs/31-模块设计决策记录.md §29`
