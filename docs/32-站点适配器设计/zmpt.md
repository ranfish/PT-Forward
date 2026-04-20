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

## 审核脚本完整逆向分析

### 脚本基本信息

| 项目 | 值 |
|------|-----|
| 名称 | zmpt-check-tool |
| 来源 | Greasyfork #552769 |
| 作者 | anynopt |
| 版本 | 2026.03.08 |
| 大小 | 2696 行 / 117KB |
| 运行页面 | `https://zmpt.cc/details.php?id=*` |
| 权限 | GM_xmlhttpRequest |
| 协议 | MIT |

> **架构特点**：织梦审核脚本是**种审员辅助工具**（非发布页脚本），运行在 `details.php` 详情页。与猫站/不可说/末日/财神等站点的**发布页预审核**脚本不同，它用于种审员在审核已发布的种子时快速检查各项规范。

### 整体架构（两阶段模型）

```
┌─────────────────────────────────────────────────┐
│ Phase 1: 信息提取（DOM解析 + API获取）            │
│  ├─ 页面DOM → 种子名称/官组/副标题/基本信息/标签  │
│  ├─ 简介 fieldsets → MediaInfo/DiscInfo/引用     │
│  ├─ viewfilelist.php → 种子文件列表               │
│  └─ info.php → 种审员业绩                        │
├─────────────────────────────────────────────────┤
│ Phase 2: 校验规则（20+ 项三方交叉验证）           │
│  ├─ 标题格式 → 7项规则                           │
│  ├─ 标签 vs MI/DiscInfo → 6项交叉验证            │
│  ├─ 简介完整性 → 4项检查                         │
│  ├─ 转载来源 → siteSourcesMap(54站)匹配          │
│  └─ 类型特定规则 → 3项                           │
└─────────────────────────────────────────────────┘
```

### 信息提取详细表

| 提取项 | 来源 | 提取方式 | 代码行 |
|--------|------|---------|--------|
| 种子名称 | `h1#top` | `childNodes[0].textContent` | 2550 |
| 官组信息 | 种子名称末尾 | `slice(-10).split('-').pop()` | 2552 |
| 副标题 | 主表格"副标题"行 | XPath: `.//tr[td[1]='副标题']/td[2]` | 2584 |
| 基本信息 | 主表格"基本信息"行 | 正则解析 大小/类型/视频类/分辨率/音频类 | 2569-2575 |
| 标签 | 主表格"标签"行 | `querySelectorAll('span')` → 文本数组 | 2604-2607 |
| 简介 | 主表格"简介"行 | `querySelectorAll('fieldset')` 分类解析 | 2625-2630 |
| MediaInfo | 简介fieldset | 按 General/Video/Audio/Text 分段解析为对象 | 1687-1755 |
| DiscInfo(BDInfo) | 简介fieldset | 按 DISC INFO/VIDEO/AUDIO/SUBTITLES 分段解析 | 1761-1830 |
| DiscInfo(DiscTitle) | 简介fieldset | 按 Disc Title/Video/Audio/Subtitle 行式解析 | 1837-1891 |
| IMDb信息 | 主表格"IMDb信息"行 | XPath | 2664 |
| 发布者/时间 | 主表格发布信息行 | XPath `userdetails.php?id=` + `span[@title]` | 2531-2532 |
| 文件列表 | `viewfilelist.php?id=` | GM_xmlhttpRequest GET → 解析table | 697-790 |

### 基本信息(baseInfoDic)解析逻辑

`parseBaseInfo` 函数（第2465-2511行）从"基本信息"行提取结构化数据：

```
输入文本示例："大小: 2.86 GB 类型: 电影 / Movies 视频类: Blu-ray 分辨率: 1080p/1080i 音频类: DTS"
```

| 键名 | 属性名 | 示例值 | 备注 |
|------|--------|--------|------|
| 大小 | size | 2.86 GB | |
| 类型 | type | 电影 / Movies | 分类全称 |
| 视频类 | videoClass | Blu-ray | 媒介 |
| 分辨率 | resolution | 1080p/1080i | |
| 音频类 | audio | DTS | 如果为"Other"则不提取 |

### 官组识别逻辑

```javascript
// 第2552行：从标题末尾提取官组
const official_group = torrent_name.slice(-10).split('-').pop();

// 判定是否织梦官方：官组名以"zm"开头（不区分大小写）
const isZMOfficial = official_group.toLowerCase().startsWith("zm");
```

> **关键**：织梦官组（ZmWeb/ZmPT/ZmMusic/ZmAudio）的资源无需声明转载来源，且有特殊校验豁免（如中字标签校验跳过）。

### 校验规则完整列表（20项）

| # | 规则 | 校验方式 | 通过条件 | 级别 |
|---|------|---------|---------|------|
| 1 | 种子来源官组 | 标题末尾`-`后提取 | 显示官组名 | 信息 |
| 2 | 种子标题-中文 | `[\u4e00-\u9fa5\uff01-\uff60]` | 无中文/全角字符 | 错误 |
| 3 | 种子标题-分辨率格式 | `/(480\|720\|1080\|2160\|4320)P/` | P小写p | 错误 |
| 4 | 种子标题-4K格式 | `4K` in title | 应改为2160p | 错误 |
| 5 | 种子标题-空格数量 | `(title.match(/ /g) \|\| []).length` | ≥5个空格 | 错误 |
| 6 | 种子标题-HDR格式 | `/(?:^\|\s)(hdr)(?:$\|\s)/i` | HDR全大写 | 错误 |
| 7 | 种子标题-年份 | `/(19\|20)\d{2}/` | 含有效年份 | 错误 |
| 8 | 种子标题-视频编码 | 标题中查找匹配编码 | 含有效视频编码 | 错误 |
| 9 | 分辨率选择校验 | 标题解析 vs baseInfoDic.resolution | 两者匹配 | 错误/正确 |
| 10 | 转载来源检查 | fieldset引用 vs siteSourcesMap(54站) | ZM官组免声明/国内站需声明 | 正确/错误/警告 |
| 11 | MediaInfo/DiscInfo校验 | fieldset分类解析 | 至少存在一种 | 正确/错误 |
| 12 | 中字标签校验 | MI/DiscInfo Text Language vs 标签 | 交叉一致 | 正确/错误/警告 |
| 13 | 杜比标签校验 | MI/DiscInfo Dolby Vision vs 标签 | 交叉一致 | 正确/错误 |
| 14 | HDR标签校验 | MI/DiscInfo HDR信息 vs 标签 | 交叉一致 | 正确/错误 |
| 15 | 简介图片数量 | `introduction_element.querySelectorAll('img')` | ≥2张 | 错误/正确 |
| 16 | 简介图片可访问性 | `img.complete + naturalWidth` | 全部可访问 | 正确/错误 |
| 17 | 电视剧分集/完结 | type含"电视剧"时检查标签 | 必须含"完结"或"分集" | 错误/正确 |
| 18 | 豆瓣&IMDb校验 | 简介文本正则匹配链接 | 至少有一种链接 | 错误 |
| 19 | 电影标签检测 | type含"电影"时检查 | 不含"完结"或"分集" | 错误/正确 |
| 20 | 纯享版标签提示 | 标签含"纯享版" | 仅警告 | 警告 |
| 21 | 国语标签检测 | MI Audio + 简介◎语言 + 副标题 vs 标签 | 三方交叉一致 | 正确/错误/警告 |
| 22 | 年代检测 | 简介◎年代 vs IMDb信息年代 | 显示两者 | 信息 |
| 23 | 原盘/DIY检测 | DiscInfo有无 + 标题Blu-ray + 文件.mkv + 标签 | 交叉一致 | 正确/错误 |

### 视频编码识别表

脚本使用的完整视频编码列表（第2186-2187行）：

**标准编码**（标题校验查找）：

| 编码 | 类型 |
|------|------|
| x264 | 编码（发布端） |
| x265 | 编码（发布端） |
| H.264 | 编码（标准名） |
| H.265 | 编码（标准名） |
| AVC | 编码（别名） |
| HEVC | 编码（别名） |
| VC-1 | 编码 |
| AVS2 | 国产编码 |
| AVS3 | 国产编码 |

**非标准编码**（也在查找范围）：

| 编码 | 标准写法 |
|------|---------|
| H265 | 应为 H.265 |
| H264 | 应为 H.264 |
| MPEG-2 | — |

### 音频编码识别表

脚本使用的音频编码列表（第2185行）：

| 编码 | 说明 |
|------|------|
| FLAC | 无损 |
| DTS | 多声道 |
| AC3 | 杜比数字 |
| DDP 2.0 | Dolby Digital Plus 2.0 |
| TrueHD | 杜比无损 |
| AAC | 有损压缩 |
| LPCM | 无压缩 |

### 分辨率解析逻辑

脚本内置分辨率常量映射（第2417-2424行）：

```javascript
resolution_constant = {
    1: "480p",
    2: "720p",
    3: "1080p/1080i",
    4: "4K/2160p",
    5: "8k/4320p/4320i",
    6: "Other"
};
```

解析函数 `getResolutionKey`（第2430-2451行）从标题提取分辨率：

```
正则匹配优先级：4k/8k > 2160p/4320p > 1080p/720p/480p
匹配规则：独立单词（前后需空格或行首行尾）
480p/480i → key 1
720p/720i → key 2
1080p/1080i → key 3
4K/2160p/2160i → key 4
8K/4320p/4320i → key 5
未匹配 → key 6 (Other)
```

> **注意**：此常量映射用于**校验用户选择是否正确**，与页面 `standard_sel[4]` 的表单ID映射不同（1080p=1, 4K=5, 720p=8, 480p=7 等）。

### HDR检测算法（五层递进）

脚本实现了一套极为完整的HDR检测系统（第1532-1678行），覆盖MediaInfo和BDInfo两种格式：

```
检测层级（优先级从高到低）：
┌─ Level 1: 直接HDR关键词匹配 ─────────────────────────┐
│  字段: HDR format / Format profile / Format/Info      │
│       / Commercial name / Title                       │
│  关键词: HDR Vivid / HDR10 / HDR10+ / Advanced HDR   │
│         / HLG / BT2020_10 / Dolby Vision / SMPTE ST 2086│
├─ Level 2: Transfer Characteristics 映射 ─────────────┤
│  PQ → HDR10                                          │
│  HLG → HLG                                           │
│  SMPTE ST 2084 → HDR10                               │
│  PQ / BT.2100 HLG → HDR10+HLG                        │
├─ Level 3: HDR元数据字段检测 ─────────────────────────┤
│  Maximum Content Light Level (MaxCLL)                 │
│  Maximum Frame-Average Light Level (MaxFALL)          │
│  Mastering display luminance                          │
│  → 检测到任一即判为 HDR10                              │
├─ Level 4: 色彩原色 + 位深组合特征 ───────────────────┤
│  Color primaries 含 BT.2020/BT2020                    │
│  + Bit depth 含 10 bit/10 bits/10                     │
│  → 判为 HDR10（HLG/ST2084可进一步细分）               │
└──────────────────────────────────────────────────────┘
```

### 杜比视界(Dolby Vision)检测算法

```
检测字段（优先级从高到低）：
1. MI Video "HDR format" 含 "Dolby Vision"
2. MI Video "Format profile" 含 "Dolby Vision"
3. MI Video "Format/Info" 含 "Dolby Vision"
4. MI Video "Commercial name" 含 "Dolby Vision"
5. MI Video "Title" 含 "Dolby Vision"
6. DiscInfo Video 行含 "Dolby Vision"
```

### 中文音频检测算法

中文音频语言标识（第1193-1202行，按优先级排序）：

| 标识 | 说明 |
|------|------|
| Chinese | 通用中文 |
| Mandarin | 普通话 |
| 国语 | 简体中文标识 |
| 普通话 | 标准普通话 |
| 中文 | 通用中文 |
| Mandarin (CN) | 中国大陆普通话 |
| Chinese (CN) | 中国大陆中文 |
| Chinese (Simplified) | 简体中文 |

检查字段：`Audio.Language / Audio.language / Audio.Title / Audio.title`（4字段全覆盖）

对 DiscInfo 格式，解析 `Audio:` 行的第一个 `/` 前的语言部分。

### 中文字幕检测算法

中文字幕语言标识（第1377-1392行）：

| 标识 | 类型 |
|------|------|
| Simplified Chinese | 标准简体 |
| Chinese | 通用中文 |
| 简体中文 | 中文 |
| 繁体中文 | 繁体 |
| 繁体 | 繁体简称 |
| 简体 | 简体简称 |
| 简体中字 | 简体 |
| 中文简体 | 简体 |
| 中文（简体） | 简体 |
| Chinese Simplified | 标准简体 |
| Simplified | 简体简称 |
| Chinese (Simplified) | 标准简体 |
| Mandarin | 普通话 |

检查字段：`Text.Language / Text.language / Text.Title / Text.title`（4字段全覆盖）

> **注意**：此列表同时包含简体和繁体标识，脚本不区分简繁，统一判定为"中文字幕"。

### 国语标签三方交叉验证

`checkGYTag` 函数（第1273-1308行）实现了三方交叉验证：

```
验证源 1: 标签是否含"国语"
验证源 2: MediaInfo/DiscInfo 音轨是否含中文语言
验证源 3: 简介◎语言 是否含"汉语普通话"或"国语"
验证源 4: 副标题 是否含"国语"

结果矩阵：
┌───────────┬──────────┬──────────┬──────────────────────┐
│ 标签      │ MI音轨   │ 简介语言 │ 结果                  │
├───────────┼──────────┼──────────┼──────────────────────┤
│ ✓国语     │ ✓中文    │ ✓中文    │ √ 完美通过            │
│ ✓国语     │ ✓中文    │ ✗        │ √ 通过                │
│ ✓国语     │ ✗        │ ✓中文    │ √ 通过                │
│ ✓国语     │ ✗        │ ✗        │ × 标签选中但无来源     │
│ ✗         │ ✓中文    │ —        │ × 需补充国语标签       │
│ ✗         │ —        │ ✓中文    │ × 需补充国语标签       │
│ ✗         │ —        │ —        │ √ 通过（副标题也检查） │
└───────────┴──────────┴──────────┴──────────────────────┘
```

### 原盘/DIY标签检测算法

`checkYPTag` 函数（第913-960行）：

```python
# 核心逻辑（伪代码）
isYPOrDiy = videoIsDiyOrYP OR (NOT torrent_file_has_mkv)
# videoIsDiyOrYP = NOT mediaInfo AND discInfo  (第2693行)
# 即：有DiscInfo无MediaInfo → 原盘；或文件列表无.mkv

should_tag = isYPOrDiy AND (标题含blu-ray/bluray OR 标题/副标题含diy/原盘)

if should_tag:
    if 已选原盘或DIY标签: → √ 通过
    else: → × 请人工确认
else:
    if 已选原盘或DIY标签: → × 不是原盘DIY但选了标签
    else: → √ 通过
```

**关键判定**：`!mediaInfo && !!discInfo` — 即 DiscInfo 存在但 MediaInfo 不存在 = 原盘。这是因为原盘用 BDInfo 而非 MediaInfo 生成媒体信息。

### 转载来源检测（siteSourcesMap — 54站）

脚本内置了 54 个 PT 站点到其官组别名的映射（第1901-1955行）：

| 站点名 | 官组别名 |
|--------|---------|
| 天枢 | dubheweb |
| 好学 | hxweb |
| 幸运PT | LuckAni, luckdiy |
| 三月传媒 | （空数组） |
| 铂金短剧 | （空数组） |
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

**转载来源判定逻辑**：

```
if 官组名以"zm"开头（不区分大小写）:
    → 织梦官方资源，无需声明引用
elif 简介fieldsets中找到引用（含"观组作品"或"感谢原制作者发布"）:
    → 从siteSourcesMap匹配站点名 → 已声明引用，校验通过
elif siteSourcesMap中匹配到国内站:
    → 国内资源站需要声明来源（错误）
else:
    → 疑似国外资源站，不需要声明来源（警告）
```

**引用关键词**（第1958行）：`['观组作品', '感谢原制作者发布']`

### 中字标签校验特殊规则

织梦官方资源有特殊豁免（第2043-2044行）：

```javascript
if (isZMOfficial) {
    // 织梦官方资源，无需校验中字标签，20251111协议
} else {
    // 正常交叉验证 MI字幕 vs 标签
}
```

### 批量审核功能

- **"启动吧，电力！"按钮**：获取未审核种子列表（`torrents.php?approval_status=0&sort=4&type=desc&page=30`），批量打开详情页
- **"一键通过"按钮**：自动选择通过（value=1）并提交
- **"一键拒绝"按钮**：自动选择拒绝（value=2）并聚焦备注框
- **回车键快捷键**：按下Enter键自动点击审核按钮
- **审核弹框**：Layui iframe弹框，脚本监听iframe加载完成后自动操作

### MediaInfo解析器

脚本实现了完整的MediaInfo文本解析器（第1687-1755行）：

```
输入: 纯文本MediaInfo
分段: 按 General/Video/Audio #N/Text #N 分割
解析: 每行按第一个冒号分割为key:value

输出结构:
{
    General: { key: value, ... },
    Video:   { key: value, ... },     // 单对象
    Audio:   [{ index, key: value }, ...],  // 数组
    Text:    [{ index, key: value }, ...]   // 数组
}
```

### DiscInfo解析器（双格式）

脚本支持两种DiscInfo格式：

**格式1: BDInfo标准格式**（第1761-1830行）
```
DISC INFO:
...
VIDEO:
...
AUDIO:
...
SUBTITLES:
...
```

**格式2: DiscTitle格式**（第1837-1891行）
```
Disc Title: ...
Video: ...
Audio: ...
Subtitle: ...
```

两种格式统一解析为相同结构：

```
{
    General: { key: value, ... },
    Video:   [line, ...],   // 数组（每行一条）
    Audio:   [line, ...],   // 数组
    Text:    [line, ...]    // 数组（映射自SUBTITLES/Subtitle）
}
```

### PT-Gen接口

脚本调用了织梦自建的PT-Gen（第996-1003行）：

```
IMDb链接转换: https://ptgen.zmpt.cc?site=douban&sid={imdbId}
其他链接转换: https://ptgen.zmpt.cc?url={encodedUrl}
返回: JSON { this_title, trans_title }
```

### 校验结果分级

脚本使用三级信息分级：

| 级别 | 前缀 | 颜色 | 说明 |
|------|------|------|------|
| 正确 | `√` | #107c10 绿色 | 校验通过 |
| 警告 | `!` | #FF8C00 橙色 | 需人工确认 |
| 错误 | `×` | #d83b01 红色 | 校验不通过 |

## 转载发布自动填写优化方案

> 基于审核脚本逆向分析结果，设计转载发布时的自动填写逻辑。目标是让发布的种子**一次性通过全部20+项校验规则**。

### 1. 标题自动填写

#### 标题格式模板

```
电影：[英文名] [年份] [剪辑版本] [发布说明] [来源] [分辨率] [视频编码] [音频编码]-[组名]
电视剧：[英文名] S**E** [年份] [发布说明] [分辨率] [来源] [视频编码] [音频编码]-[组名]
```

#### 标题清洗规则（对应审核规则 #2-#8）

```python
def clean_title(source_title):
    title = source_title

    # Rule #2: 去除中文/全角字符
    title = re.sub(r'[\u4e00-\u9fa5\uff01-\uff60]+', '', title)

    # Rule #3: 分辨率P → p
    title = re.sub(r'(480|720|1080|2160|4320)P', r'\1p', title)

    # Rule #4: 4K → 2160p
    title = re.sub(r'\b4K\b', '2160p', title, flags=re.IGNORECASE)

    # Rule #6: hdr → HDR
    title = re.sub(r'\bhdr\b', 'HDR', title)

    # Rule #5: 确保空格≥5个（用空格替代点号/下划线/连字符）
    title = title.replace('.', ' ').replace('_', ' ').replace('(', ' ').replace(')', ' ')

    # Rule #7: 确保年份存在且在版本标记前
    year = extract_year(title)
    version_tags = ['PROPER', 'REPACK', 'READ.NFO', 'RERIP']
    for tag in version_tags:
        if tag in title and year:
            ensure_year_before_tag(title, year, tag)

    # Rule #8: 确保包含有效视频编码
    # 标准编码优先: x264, x265, H.264, H.265, AVC, HEVC, VC-1, AVS2, AVS3
    # 非标准也接受: H265, H264, MPEG-2

    # 规范化编码名称
    title = title.replace('HEVC', 'H.265')  # 可选
    title = title.replace('AVC', 'H.264')    # 可选

    return ' '.join(title.split())  # 压缩多余空格
```

#### 标题提取 — 主标题 vs 官组

```python
def split_title_group(torrent_name):
    """与审核脚本一致的标题拆分逻辑（第2088-2095行）"""
    if '-' in torrent_name:
        main_title = torrent_name[:torrent_name.rfind('-')]
        group = torrent_name[torrent_name.rfind('-') + 1:]
    else:
        main_title = torrent_name
        group = ''
    return main_title, group
```

### 2. 副标题自动填写

```python
def generate_subtitle(source_subtitle, douban_title, media_info, category):
    """
    格式: 中文名 | 有价值信息 | [音轨信息] [字幕信息]
    """
    parts = []

    # 1. 中文名（开头必须是中文名）
    chinese_title = douban_title or extract_chinese_from_subtitle(source_subtitle)
    parts.append(chinese_title)

    # 2. 剪辑版本信息（如有）
    if 'PROPER' in source_subtitle or 'REPACK' in source_subtitle:
        parts.append(extract_version_info(source_subtitle))

    # 3. 音轨语言信息
    audio_langs = []
    if media_info and media_info.get('Audio'):
        for track in media_info['Audio']:
            lang = track.get('Language') or track.get('language', '')
            if lang:
                audio_langs.append(lang)
    if audio_langs:
        parts.append(' '.join(set(audio_langs)))

    # 4. 字幕信息
    subtitle_langs = []
    if media_info and media_info.get('Text'):
        for track in media_info['Text']:
            lang = track.get('Language') or track.get('language', '')
            if lang:
                subtitle_langs.append(lang)
    if subtitle_langs:
        parts.append(' '.join(set(subtitle_langs)) + '字幕')

    return ' | '.join(filter(None, parts))
```

> **注意**：如果副标题含"完结"字样且类型为电视剧，审核脚本会提示补充完结标签（第1360-1363行）。如需触发此提示，可在副标题中包含"完结"。

### 3. 分类自动选择

```python
CATEGORY_MAP = {
    # 源站分类关键词 → 织梦 type ID
    'Movie': 401, 'Film': 401, '电影': 401,
    'TV Series': 402, 'Drama': 402, '电视剧': 402,
    'TV Show': 403, 'Variety': 403, '综艺': 403,
    'Anime': 417, 'Animation': 417, '动漫': 417,
    'Documentary': 422, '纪录片': 422,
    'Music': 423, '音乐': 423,
    'Audiobook': 424, '有声书': 424,
    'Software': 425, '软件': 425,
    'Game': 426, '游戏': 426,
    'Short Play': 427, '短剧': 427,
    'Other': 409, '其他': 409, 'Misc': 409,
}
```

> **电视剧特殊处理**（Rule #17）：选择 402 后必须同时添加"完结"或"分集"标签。

### 4. 质量字段自动选择

#### 媒介 (medium_sel[4])

```python
MEDIUM_MAP = [
    # 按优先级排序（关键词 → ID）
    (r'\bBlu-?ray\b|\bBDMV\b', 1),       # Blu-ray
    (r'\bRemux\b', 3),                     # Remux
    (r'\bWEB-?DL\b|\bWebDL\b', 10),       # WEB-DL
    (r'\bHDTV\b', 5),                      # HDTV
    (r'\bDVD(?:R|ISO)?\b', 6),            # DVDR
    (r'\bCD\b', 8),                        # CD
    (r'\bEncode\b', 7),                    # Encode
]
```

> **原盘判定逻辑**（对应 Rule #23）：如果标题含 Blu-ray 且种子文件中无 .mkv → 选 Blu-ray(1) 并勾选"原盘"标签。如果有 DiscInfo 无 MediaInfo → 原盘。

#### 分辨率 (standard_sel[4])

```python
RESOLUTION_MAP = {
    # 关键词 → standard_sel[4] ID（注意：与审核脚本resolution_constant不同！）
    '2160p': 5, '4K': 5, 'UHD': 5,
    '1080p': 1, '1080i': 1, 'FHD': 1,
    '720p': 8, 'HD': 8,
    '480p': 7, 'SD': 7,
    '4320p': 9, '8K': 9,
    '1440p': 6, '2K': 6,
}

def get_resolution(title, media_info):
    """标题优先，MI补充"""
    # 1. 从标题提取
    for pattern, sel_id in RESOLUTION_MAP.items():
        if re.search(r'\b' + pattern + r'\b', title, re.IGNORECASE):
            return sel_id

    # 2. 从MI Width x Height推断
    if media_info and media_info.get('Video'):
        w = int(media_info['Video'].get('Width', 0))
        if w >= 3800: return 9   # 8K
        if w >= 1900: return 5   # 4K
        if w >= 1440: return 6   # 2K
        if w >= 1000: return 1   # 1080p
        if w >= 700:  return 8   # 720p
        return 7                     # 480p

    return 6  # Other → 映射到 2K/1440p
```

> **注意**：织梦的分辨率ID非常规（1080p=1, 4K=5, 720p=8, 480p=7），必须使用上表映射，不能按数值顺序。

#### 音频编码 (audiocodec_sel[4])

```python
AUDIO_CODEC_MAP = {
    # MediaInfo Audio.Format → audiocodec_sel[4] ID
    'FLAC': 1,
    'APE': 2,
    'DTS': 3,
    'MP3': 4,
    'OGG': 5,
    'AAC': 6,
    'Other': 7,
    'AC-3': 8, 'AC3': 8, 'DD': 8,
    'ALAC': 9,
    'WAV': 10,
}

def get_audio_codec(media_info):
    """从MI Audio Format提取"""
    if not media_info or not media_info.get('Audio'):
        return None  # 可不选（审核脚本：未选择不需要校验）
    format_str = media_info['Audio'][0].get('Format', '')
    for key, sel_id in AUDIO_CODEC_MAP.items():
        if key.lower() in format_str.lower():
            return sel_id
    return 7  # Other
```

> **重要**（第1031-1033行）：如果音频编码未选择，审核脚本不报错（"发布者未选择音频格式，不需要主标题校验音频"）。但一旦选择了就不能错（第1055行："发布者选择音频格式未在种子名称中找到"）。

#### 制作组 (team_sel[4])

```python
TEAM_MAP = {
    # 组名关键词 → team_sel[4] ID
    'ZmWeb': 7, 'ZmPT': 6, 'ZmMusic': 8, 'ZmAudio': 11,
    'DYZ-Movie': 9, 'DYZ-WEB': 9,
    'GodDramas': 10,
    'RL4B': 12, 'RL': 12,
}

def get_team(title):
    """从标题末尾提取组名"""
    _, group = split_title_group(title)
    group_lower = group.lower()
    for key, sel_id in TEAM_MAP.items():
        if key.lower() == group_lower:
            return sel_id
    return 5  # Other
```

### 5. IMDb链接自动填写

```python
def get_imdb_url(source_description, source_url_field):
    """从简介或源站url字段提取IMDb链接"""
    # 优先从源站url字段提取
    if source_url_field and 'imdb.com' in source_url_field:
        return source_url_field

    # 从简介中提取
    match = re.search(r'https?://[^\s]*imdb\.com/title/(tt\d+)', source_description)
    if match:
        return f'https://www.imdb.com/title/{match.group(1)}/'

    return ''
```

> **注意**：审核脚本要求简介中至少有豆瓣或IMDb链接之一（Rule #18），且不能有重复链接（同类型>1个=错误）。

### 6. PT-Gen自动填写

```
织梦PT-Gen端点: https://ptgen.zmpt.cc
支持参数:
  - url={encodedUrl}    # 通用链接
  - site=douban&sid={imdbId}  # IMDb→豆瓣转换
返回: JSON { this_title, trans_title, ... }

填写 pt_gen 字段时使用源站的PT-Gen链接或通过织梦PT-Gen重新生成。
```

### 7. 简介自动构建

```python
def build_description(source_desc, pt_gen_data, media_info, disc_info, source_site):
    """
    构建织梦标准简介模板
    """
    desc_parts = []

    # 1. PT-Gen 影片信息（◎格式）
    if pt_gen_data:
        desc_parts.append(format_pt_gen_template(pt_gen_data))

    # 2. MediaInfo / BDInfo（以fieldset引用形式）
    if media_info:
        desc_parts.append(f'[quote]{format_mediainfo(media_info)}[/quote]')
    elif disc_info:
        desc_parts.append(f'[quote]{format_discinfo(disc_info)}[/quote]')

    # 3. 截图（保留源站截图）
    screenshots = extract_screenshots(source_desc)
    desc_parts.extend(screenshots)

    # 4. 转载声明（引用块，含关键词"感谢原制作者发布"或"观组作品"）
    # 注意: 仅非织梦官方资源需要，且国内站（siteSourcesMap中）必须声明
    if source_site and not is_zm_official(source_site):
        desc_parts.append(f'[quote]感谢原制作者发布[/quote]')

    # 5. 确保≥2张图片（Rule #15）
    img_count = count_images(desc_parts)
    if img_count < 2:
        # 需要补充图片
        pass

    return '\n\n'.join(desc_parts)
```

**简介格式模板**（织梦标准）：

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

[quote] MediaInfo/BDInfo原文 [/quote]

[截图1] [截图2] ...

[quote]感谢原制作者发布[/quote]  ← 转载必加，含关键词触发审核脚本引用识别
```

### 8. 标签自动选择

```python
def auto_select_tags(media_info, disc_info, title, subtitle, category, file_list):
    """
    基于审核脚本逆向的标签判定逻辑，确保每个标签都能通过对应校验规则
    """
    tags = []

    has_media_info = media_info is not None
    has_disc_info = disc_info is not None
    check_prompt = 'MediaInfo' if has_media_info else 'DiscInfo'

    # === 中字标签 (Rule #12) ===
    if detect_chinese_subtitle(media_info, disc_info):
        tags.append('中字')

    # === 国语标签 (Rule #21) ===
    if detect_chinese_audio(media_info, disc_info):
        tags.append('国语')

    # === 杜比标签 (Rule #13) ===
    if detect_dolby_vision(media_info, disc_info):
        tags.append('杜比')

    # === HDR标签 (Rule #14) ===
    if detect_hdr(media_info, disc_info):
        tags.append('HDR')

    # === 原盘/DIY标签 (Rule #23) ===
    is_yp_or_diy = (not has_media_info and has_disc_info) or \
                   not any(f.endswith('.mkv') for f in file_list)
    has_bluray = bool(re.search(r'\b(Blu-?ray|Bluray)\b', title, re.IGNORECASE))
    has_diy_keyword = bool(re.search(r'\bdiy\b|\b原盘\b', title + ' ' + subtitle, re.IGNORECASE))

    if is_yp_or_diy and (has_bluray or has_diy_keyword):
        if has_diy_keyword:
            tags.append('DIY')
        else:
            tags.append('原盘')

    # === 电视剧标签 (Rules #17, #19) ===
    if '电视剧' in category or 'TV Series' in category:
        if '完结' in subtitle or '全' in subtitle:
            tags.append('完结')
        else:
            tags.append('分集')

    # 电影不能有完结/分集标签（Rule #19）
    if '电影' in category or 'Movie' in category:
        tags = [t for t in tags if t not in ('完结', '分集')]

    return tags
```

#### 中文音频检测（与审核脚本一致）

```python
CHINESE_AUDIO_IDS = [
    'Chinese', 'Mandarin', '国语', '普通话', '中文',
    'Mandarin (CN)', 'Chinese (CN)', 'Chinese (Simplified)'
]

def detect_chinese_audio(media_info, disc_info):
    """检测MI/DiscInfo音轨是否含中文"""
    if media_info:
        audio_tracks = media_info.get('Audio', [])
        if isinstance(audio_tracks, dict):
            audio_tracks = [audio_tracks]
        for track in audio_tracks:
            for field in ['Language', 'language', 'Title', 'title']:
                val = track.get(field, '')
                if val and any(cid in str(val) for cid in CHINESE_AUDIO_IDS):
                    return True

    if disc_info and disc_info.get('Audio'):
        audio_lines = disc_info['Audio']
        if isinstance(audio_lines, str):
            audio_lines = [audio_lines]
        for line in audio_lines:
            lang_part = line.split('/')[0].replace('Audio:', '').strip()
            if any(cid in lang_part for cid in CHINESE_AUDIO_IDS):
                return True

    return False
```

#### 中文字幕检测（与审核脚本一致）

```python
CHINESE_SUBTITLE_IDS = [
    'Simplified Chinese', 'Chinese', '简体中文', '繁体中文', '繁体',
    '简体', '简体中字', '中文简体', '中文（简体）', 'Chinese Simplified',
    'Simplified', 'Chinese (Simplified)', 'Mandarin'
]

def detect_chinese_subtitle(media_info, disc_info):
    """检测MI/DiscInfo字幕是否含中文"""
    if media_info:
        text_tracks = media_info.get('Text', [])
        if isinstance(text_tracks, dict):
            text_tracks = [text_tracks]
        for track in text_tracks:
            for field in ['Language', 'language', 'Title', 'title']:
                val = track.get(field, '')
                if val and any(sid.lower() in str(val).lower() for sid in CHINESE_SUBTITLE_IDS):
                    return True

    if disc_info and disc_info.get('Text'):
        text_lines = disc_info['Text']
        if isinstance(text_lines, str):
            text_lines = [text_lines]
        pattern = '|'.join(re.escape(sid) for sid in CHINESE_SUBTITLE_IDS)
        for line in text_lines:
            line_str = json.dumps(line) if isinstance(line, dict) else str(line)
            if re.search(pattern, line_str, re.IGNORECASE):
                return True

    return False
```

#### HDR检测（与审核脚本五层逻辑一致）

```python
def detect_hdr(media_info, disc_info):
    """五层递进HDR检测"""
    HDR_KEYWORDS = ['HDR Vivid', 'HDR10', 'HDR10+', 'Advanced HDR', 'HLG',
                    'BT2020_10', 'Dolby Vision', 'SMPTE ST 2086']
    TRANSFER_MAP = {'PQ': 'HDR10', 'HLG': 'HLG', 'SMPTE ST 2084': 'HDR10'}
    HDR_METADATA_FIELDS = ['Maximum Content Light Level', 'Maximum Frame-Average Light Level',
                          'MaxCLL', 'MaxFALL', 'Mastering display luminance']

    if media_info and media_info.get('Video'):
        v = media_info['Video']
        # Level 1: 直接关键词
        for field in ['HDR format', 'Format profile', 'Format/Info', 'Commercial name', 'Title']:
            val = v.get(field, '')
            if val and any(kw in val for kw in HDR_KEYWORDS):
                return True

        # Level 2: Transfer characteristics
        tc = v.get('Transfer characteristics', '')
        for key in TRANSFER_MAP:
            if key in tc:
                return True

        # Level 3: 元数据字段
        for field in HDR_METADATA_FIELDS:
            if v.get(field):
                return True

        # Level 4: BT.2020 + 10bit 组合
        cp = v.get('Color primaries', '')
        bd = v.get('Bit depth', '')
        if cp and bd:
            if ('BT.2020' in cp or 'BT2020' in cp) and ('10 bit' in bd or '10 bits' in bd or bd == '10'):
                return True

    if disc_info and disc_info.get('Video'):
        for line in disc_info['Video']:
            if any(kw in line for kw in HDR_KEYWORDS):
                return True
            for key in TRANSFER_MAP:
                if key in line:
                    return True
            if 'BT.2020' in line and '10 bit' in line:
                return True

    return False
```

### 9. 转载发布全流程校验清单

转载发布后，为确保通过全部20+项审核规则，发布前应自检：

| # | 自检项 | 对应审核规则 | 自动填写保障 |
|---|--------|-------------|-------------|
| 1 | 标题无中文 | #2 | clean_title 去中文 |
| 2 | 分辨率p小写 | #3 | clean_title 规范化 |
| 3 | 无4K用2160p | #4 | clean_title 替换 |
| 4 | 空格≥5个 | #5 | clean_title 保证 |
| 5 | HDR全大写 | #6 | clean_title 规范化 |
| 6 | 含有效年份 | #7 | 标题模板包含 |
| 7 | 含有效视频编码 | #8 | 标题模板包含 |
| 8 | 分辨率与选择一致 | #9 | 同一解析函数 |
| 9 | 含转载引用 | #10 | 简介自动加引用 |
| 10 | 含MI或DiscInfo | #11 | 简介自动包含 |
| 11 | 中字标签正确 | #12 | 标签自动选择 |
| 12 | 杜比标签正确 | #13 | 标签自动选择 |
| 13 | HDR标签正确 | #14 | 标签自动选择 |
| 14 | 图片≥2张 | #15 | 简介模板保证 |
| 15 | 图片可访问 | #16 | 使用推荐图床 |
| 16 | 电视剧标签 | #17 | 分类联动 |
| 17 | 豆瓣/IMDb链接 | #18 | 简介模板包含 |
| 18 | 电影无完结标签 | #19 | 分类联动排除 |
| 19 | 国语标签正确 | #21 | 标签自动选择 |
| 20 | 原盘/DIY标签正确 | #23 | 标签自动选择 |

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
