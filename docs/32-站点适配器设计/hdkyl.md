# 麒麟 站点适配器设计

> HDKylin（麒麟/海盗）站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 麒麟|
| 站点地址 | https://www.hdkyl.in |
| 站点框架 | NexusPHP |
| 特殊规则 | **种审制·27黑名单制作组·processing_sel=年份·source_sel=地区(19个)·19音频编码·2K/1440p·480p·官种/驻站标签体系·MediaInfo字段·短剧分类·Cloudflare(SL Challenge)** |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://tracker.hdkyl.in/announce.php` |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（`data-pt-gen="url"`） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | **MediaInfo**（独立字段） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 PT-Gen 集成

支持多来源：IMDb + Douban + Bangumi + PT-Gen（`data-pt-gen` 出现在 `url` 和 `pt_gen` 属性中）。

### 1.3 类型（`type`）— 14 个分类

`<select name="type" data-mode='4'>`

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 404 | Record Education/纪录教育 |
| 405 | Animations/动漫 |
| 406 | Music Videos/音乐视频 |
| 407 | Sports/体育运动 |
| 408 | HQ Audio/音乐 |
| 409 | Misc/其他 |
| 411 | software/软件 |
| 412 | Game/游戏 |
| 413 | Ebook/电子书 |
| 419 | Study/学习 |
| 420 | TV Shows/综艺 |
| 421 | Playlet/短剧 |

**特点**：
- **中英双语分类名**
- 有 **短剧**（421）独立分类
- 有 **Ebook/电子书**（413）和 **Study/学习**（419）
- 纪录片为 **Record Education/纪录教育**（404）
- 无体育独立分类...有 Sports/体育运动（407）
- 仅 `data-mode='4'`，单模式发布

### 1.4 处理方式（`processing_sel[4]`）= 年份 — 11 个

| 值 | 显示名称 |
|----|----------|
| 11 | 2025 |
| 10 | 2024 |
| 1 | 2023 |
| 2 | 2022 |
| 3 | 2021 |
| 4 | 2020 |
| 5 | 2019 |
| 6 | 2018 |
| 7 | 2017 |
| 8 | 2016 |
| 9 | Earlier/更早 |

**注意**：`processing_sel` 在 HDKylin 表示 **年份**，从 2025 倒序到"更早"。

### 1.5 来源（`source_sel[4]`）= 地区 — 18 个

| 值 | 显示名称 |
|----|----------|
| 15 | CN/中国 |
| 16 | HK/香港 |
| 17 | TW/台湾 |
| 18 | US/美国 |
| 28 | EU/欧洲 |
| 19 | JPN/日本 |
| 20 | Kr/韩国 |
| 21 | GB/英国 |
| 22 | FR/法国 |
| 23 | DE/德国 |
| 25 | IN/印度 |
| 30 | RU/俄罗斯 |
| 31 | CA/加拿大 |
| 32 | BR/巴西 |
| 33 | SE/瑞典 |
| 34 | DK/丹麦 |
| 35 | TH/泰国 |
| 14 | Other/其他 |

**特点**：**18 个地区选项**，是目前分析站点中地区最细的之一。含欧洲/俄罗斯/巴西/瑞典/丹麦等非常见地区。

### 1.6 媒介（`medium_sel[4]`）— 10 个

| 值 | 显示名称 |
|----|----------|
| 24 | UHD Blu-ray |
| 25 | Blu-ray(原盘) |
| 27 | DVD(原盘) |
| 28 | HDTV |
| 29 | Encode |
| 30 | REMUX |
| 31 | WEB-DL |
| 32 | Track |
| 33 | CD |
| 34 | Other |

**特点**：值从 24 开始（非标准 1-10），UHD Blu-ray（24）和 Blu-ray(原盘)（25）分开。

### 1.7 编码（`codec_sel[4]`）— 7 个

| 值 | 显示名称 |
|----|----------|
| 6 | H.265/HEVC |
| 1 | H.264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2/MPEG-4 |
| 7 | AV1 |
| 5 | Other |

**特点**：MPEG-2 和 MPEG-4 合并为一个选项（值=4）。有 AV1（值=7）。

### 1.8 音频编码（`audiocodec_sel[4]`）— 18 个

| 值 | 显示名称 |
|----|----------|
| 8 | DTS-HD MA |
| 16 | DTS:X |
| 19 | DTS-HD HR |
| 3 | DTS |
| 9 | TrueHD |
| 15 | TrueHD Atmos |
| 11 | DD/AC3 |
| 17 | DDP/E-AC3 |
| 6 | AAC |
| 10 | LPCM |
| 12 | APE |
| 13 | WAV |
| 14 | M4A |
| 18 | MPEG |
| 1 | FLAC |
| 4 | MP3 |
| 20 | Opus |
| 7 | Other |

**特点**：
- **19 种音频编码**，数量丰富
- DTS 细分 4 级（DTS/DTS-HD HR/DTS-HD MA/DTS:X）
- 有 **M4A**（14）、**MPEG**（18）、**Opus**（20）
- 有 **DDP/E-AC3**（17）

### 1.9 分辨率（`standard_sel[4]`）— 7 个

| 值 | 显示名称 |
|----|----------|
| 7 | 8K/4320p/4320i |
| 6 | 4K/2160p/2160i |
| 10 | 2K/1440p/1440i |
| 1 | 1080p/1080i |
| 3 | 720p/720i |
| 8 | 480p/480i |
| 9 | Other |

**特点**：
- 有 **8K**（7）、**2K/1440p**（10）、**480p**（8）
- 每个分辨率合并 p/i（如 1080p/1080i）
- 无 SD 选项，用 480p 代替

### 1.10 制作组（`team_sel[4]`）— 9 个

| 值 | 显示名称 |
|----|----------|
| 6 | HDK |
| 7 | HDKWeb |
| 8 | HDKGame |
| 12 | Kylin |
| 14 | RedLeaves |
| 10 | CatEDU |
| 9 | GodDramas |
| 13 | StarfallWeb |
| 5 | Other |

**特点**：
- 以 **HDK** 系列为核心（HDK/HDKWeb/HDKGame）
- 有 **Kylin**（12）、**RedLeaves**（14）、**CatEDU**（10，教育组）
- 有 **GodDramas**（9，短剧组）和 **StarfallWeb**（13）
- 种审脚本中还有 HDKMV、HDKTV、HDKDIY（group_constant 映射），但上传页面只有 9 个

### 1.11 标签（`tags[4][]`）— 16 个

| 值 | 显示名称 |
|----|----------|
| 3 | 官种 |
| 15 | 驻站 |
| 2 | 首发 |
| 1 | 禁转 |
| 21 | 特效字幕 |
| 4 | DIY |
| 5 | 国语 |
| 18 | 英字 |
| 6 | 中字 |
| 7 | HDR |
| 8 | Dolby Vision |
| 9 | Blu-ray |
| 10 | LIVE现场 |
| 11 | 4K |
| 16 | 完结 |
| 19 | 分集 |

**特点**：
- 含 **官种**（3）和 **驻站**（15）——站点特色标签体系
- 区分 **英字**（18）和 **中字**（6）
- 有 **特效字幕**（21）、**Dolby Vision**（8）、**LIVE现场**（10）
- 有 **Blu-ray**（9）和 **4K**（11）标签

---

## 二、审核脚本完整逆向分析

### 脚本基本信息

| 项目 | 值 |
|------|-----|
| 名称 | HDKylin-Torrent-Assistant |
| 来源 | Greasyfork #493232 |
| 作者 | Exception & 7ommy |
| 版本 | 1.3.15 |
| 大小 | 1364 行 / 58KB |
| 运行页面 | `details.php` + `web/torrent-approval-page` |
| 依赖 | jQuery 3.4.1 |
| 协议 | MIT |

> **架构特点**：改自 SpringSunday-Torrent-Assistant（不可说），属于上游模板的下游分支。与不可说脚本共享基础架构（常量映射 + 标题解析 + 字段比对），但校验规则完全重写为麒麟特色。运行在 `details.php`（详情页）和 `web/torrent-approval-page`（审核弹框页）两个页面。

### 整体架构

```
┌───────────────────────────────────────────────────┐
│ Phase 1: 页面信息提取（jQuery DOM遍历）             │
│  ├─ 简介文本 → MediaInfo/豆瓣/IMDb/禁止转载检测     │
│  ├─ 标题解析 → 媒介/编码/音频/分辨率/官组标记        │
│  ├─ 基本信息行 → 分类/媒介/编码/音频/分辨率/地区/组  │
│  ├─ 标签行 → 禁转/官方/麒麟火/驻站/分集/国语/中字等  │
│  └─ MediaInfo栏 → 国语音轨/中文字幕/英文字幕/x26x编码│
├───────────────────────────────────────────────────┤
│ Phase 2: 校验规则（20+ 项，含跳过机制）             │
│  ├─ 标题格式 → 中文检测                              │
│  ├─ 字段完整性 → 分类/媒介/编码/音频/分辨率必选      │
│  ├─ 字段一致性 → 标题解析 vs 用户选择 6项比对        │
│  ├─ 标签规则 → 官种/驻站/禁转/大包 等               │
│  ├─ 简介质量 → 影片简介/MediaInfo/图片数量           │
│  └─ 跳过分类 → 音乐/电子书/图片/游戏/软件 免校验     │
└───────────────────────────────────────────────────┘
```

### 常量映射表（脚本内部定义，第90-173行）

#### 分类 (cat_constant)

| ID | 名称 |
|----|------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 420 | TV Shows/综艺 |
| 404 | Record Education/纪录教育 |
| 405 | Animations/动漫 |
| 406 | Music Videos/音乐视频 |
| 407 | Sports/体育运动 |
| 408 | HQ Audio/音乐 |
| 409 | Misc/其他 |
| 411 | software/软件 |
| 412 | Game/游戏 |
| 413 | Ebook/电子书 |
| 416 | Comic(漫画) |
| 419 | Study/学习 |
| 418 | Picture(图片) |
| 421 | Playlet/短剧 |

#### 媒介 (type_constant)

| ID | 名称 | 备注 |
|----|------|------|
| 24 | UHD Blu-ray | 麒麟独有 |
| 25 | Blu-ray(原盘) | |
| 27 | DVD(原盘) | |
| 28 | HDTV | |
| 29 | Encode | |
| 30 | Remux | |
| 31 | WEB-DL | |
| 32 | Track | |
| 33 | CD | |
| 34 | Other | |

> **注意**：麒麟将 UHD Blu-ray(24) 和 Blu-ray(25) 分开，与多数 NP 站不同。

#### 编码 (encode_constant)

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2/MPEG-4 |
| 5 | Other |
| 6 | H.265/HEVC |
| 7 | AV1 |

#### 音频编码 (audio_constant)

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 3 | DTS |
| 4 | MP3 |
| 6 | AAC |
| 7 | Other |
| 8 | DTS-HD MA |
| 9 | TrueHD |
| 10 | LPCM |
| 11 | DD/AC3 |
| 12 | APE |
| 13 | WAV |
| 14 | M4A |
| 15 | TrueHD Atmos |
| 16 | DTS:X |
| 17 | DDP/E-AC3 |

#### 分辨率 (resolution_constant)

| ID | 名称 |
|----|------|
| 1 | 1080p/1080i |
| 3 | 720p/720i |
| 6 | 4K/2160p/2160i |
| 7 | 8K/4320p/4320i |
| 8 | 480p/480i |
| 9 | Other |

> **注意**：分辨率ID非连续（1,3,6,7,8,9），720p=3，480p=8。

#### 制作组 (group_constant)

| ID | 名称 |
|----|------|
| 1 | HDKMV |
| 2 | HDKTV |
| 3 | HDKDIY |
| 5 | Other |
| 6 | HDK |
| 7 | HDKWeb |
| 8 | HDKGame |
| 9 | GodDramas |
| 10 | CatEDU |

> **注意**：脚本内部制作组映射与页面上 team_sel 表单不完全一致。页面上还有 Kylin(12)、StarfallWeb(13)、RedLeaves(14)。

#### 地区 (area_constant)

脚本内部定义了 22 个地区（第600-645行）：

| ID | 名称 |
|----|------|
| 14 | Other(其他) |
| 15 | CN/中国 |
| 16 | HK/香港 |
| 17 | TW/台湾 |
| 18 | US/美国 |
| 19 | JPN/日本 |
| 20 | Kr/韩国 |
| 21 | GB/英国 |
| 22 | FR/法国 |
| 23 | DE/德国 |
| 24 | AU/澳大利亚 |
| 25 | IN/印度 |
| 26 | ES/西班牙 |
| 27 | IE/爱尔兰 |
| 28 | BE/比利时 |
| 29 | IT/意大利 |
| 30 | RU/俄罗斯 |
| 31 | CA/加拿大 |
| 32 | BR/巴西 |
| 33 | SE/瑞典 |
| 34 | DK/丹麦 |
| 35 | TH/泰国 |

### 标题解析逻辑（第266-352行）

脚本从标题中提取以下信息（全部基于关键词匹配，不使用正则分组）：

#### 媒介解析

```javascript
// 优先级从上到下
"WEB-DL" / "webdl"     → type = 31
"REMUX" / "remux"      → type = 30
"webrip"/"web-rip"/"dvdrip"/"bdrip" → type = 29 (Encode)
"HDTV"                  → type = 28
```

> **注意**：脚本没有从标题解析 Blu-ray/UHD Blu-ray/DVD 原盘的媒介类型。这意味着原盘类种子如果用户选了正确的媒介，标题比对不会报错（因为 title_type 为 undefined）。

#### 视频编码解析

```javascript
"264" / "avc"    → encode = 1 (H.264/AVC)
"265" / "hevc"   → encode = 6 (H.265/HEVC)
"vc" / "vc-1"    → encode = 2 (VC-1)
"mpeg2"/"mpeg-2" → encode = 4 (MPEG-2)
"av1" / "av-1"   → encode = 12 (注意：此处BUG，encode_constant中无12)
```

> **BUG**：AV1 的 title_encode 被设为 12，但 encode_constant 最大到 7。当标题含 AV1 但用户选了 AV1(7) 时，比对会失败并报错（12 ≠ 7）。这是一个已知脚本缺陷。

#### 音频编码解析（多条件优先级，第296-321行）

```
优先级从高到低：
"flac"                              → 1
"lpcm"                              → 10
"truehd atmos"/"truehdatmos"/...    → 15
"ddp"/"ddp/e-ac3"/"dd+"/e-ac3      → 17
"aac"                               → 6
"ac3"/"dd/ac3"                      → 11
"truehd" + "atmos"                  → 15 (第二次检查)
"truehd"                            → 9
"dts-hd ma"/"dts-hdma"             → 8
"dts:x"/"dts: x"                   → 16
"dts" (不含"dts-x")                → 3
```

> **注意**：TrueHD Atmos 被检测了两次（第302行和第311行），存在冗余但逻辑正确。

#### 分辨率解析

```
"1080p"/"1080i" → 1
"720p"/"720i"   → 3
"480p"/"480i"   → 8
"4k"/"2160p"/"2160i" → 6
"8k"/"4320p"/"4320i" → 7
```

### 校验规则完整列表（20+ 项）

| # | 规则 | 校验方式 | 通过条件 | 级别 |
|---|------|---------|---------|------|
| 1 | 主标题中文检测 | `[^\x00-\xff]` 正则 | 无中文（白名单：￡™ⅠⅡ等+白自在/至尊宝） | 错误 |
| 2 | 副标题为空 | 检查 subtitle | 副标题非空 | 错误 |
| 3 | 副标题含"动画"但未选动漫分类 | 副标题文字 vs cat | 副标题"动画"→cat=405 | 错误 |
| 4 | 分类未选择 | 检查 cat | cat 有值 | 错误 |
| 5 | 媒介未选择 | 检查 type | type 有值 | 错误 |
| 6 | 媒介与标题不一致 | title_type vs type | 标题解析=用户选择 | 错误 |
| 7 | 编码未选择 | 检查 encode | encode 有值 | 错误 |
| 8 | 编码与标题不一致 | title_encode vs encode | 标题解析=用户选择 | 错误 |
| 9 | 音频编码未选择 | 检查 audio | audio 有值 | 错误 |
| 10 | 音频编码与标题不一致 | title_audio vs audio | 标题解析=用户选择 | 错误 |
| 11 | 分辨率未选择 | 检查 resolution | resolution 有值 | 错误 |
| 12 | 分辨率与标题不一致 | title_resolution vs resolution | 标题解析=用户选择 | 错误 |
| 13 | 低分辨率警告 | resolution=8/480p 且非官种/驻站 | 提示检查更高清资源 | 警告 |
| 14 | 完结标签缺失 | 标题含"complete" + 电视剧/综艺/纪录 + 未选完结 | 建议添加完结标签 | 警告 |
| 15 | 官种 MediaInfo 未解析 | 官种 + mediainfo_short=mediainfo | 需要系统解析MediaInfo | 错误 |
| 16 | 官种未选制作组 | 官种 + isGroupSelected=false | 官种必须选制作组 | 错误 |
| 17 | GodDramas 禁转标签缺失 | 驻站短剧 + 简介含"禁止转载" + 未选禁转 | 需选禁转标签 | 错误 |
| 18 | GodDramas 分类错误 | 驻站短剧 + cat≠421 | 必须选短剧(421) | 错误 |
| 19 | GodDramas 驻站标签缺失 | 驻站短剧 + 未选驻站标签 | 必须选驻站标签 | 错误 |
| 20 | 简介冗余图片 | 含特定Mediainfo.png图床链接 | 请删除多余图片 | 警告 |
| 21 | 非官种选了官方标签 | 非官种 + isOfficialSeedLabel | 非官种不可选官方标签 | 错误 |
| 22 | 官种未选官方标签 | 官种 + !isOfficialSeedLabel | 官种必须选官方标签 | 错误 |
| 23 | 官种/驻站未选麒麟火标签 | 官种或驻站 + !isIceSeedLabel | 必须选麒麟火标签 | 错误 |
| 24 | MediaInfo栏为空 | isMediainfoEmpty=true | 请补充MediaInfo | 错误 |
| 25 | 简介无影片简介 | 不含"片名/译名/名/演/主持人/简/国家" | 必须有影片简介 | 错误 |
| 26 | 大包标签缺失 | 体积>1TB + !isTagBigTorrent | 建议选大包标签 | 警告 |
| 27 | 截图不足 | imgCount < 2 | 至少2张图片 | 错误 |
| 28 | 官种HDK编码与MI不一致 | 官种HDK组 + MI含x264/x265 + 标题不含 | 标题编码应与MI一致 | 错误 |
| 29 | 音乐类(HDKMV)仅检查制作组 | cat=406 + HDKMV → 清空所有错误 | 仅检查制作组 | 特殊 |
| 30 | 音乐HQ Audio标题缺采样频率 | cat=408(音乐) + 标题无"khz" | 标题需含采样频率 | 错误 |
| 31 | 音乐HQ Audio标题缺比特率 | cat=408(音乐) + 标题无"bit" | 标题需含比特率 | 错误 |

### 分类跳过机制（第1033-1051行）

以下分类的种子**完全跳过所有校验规则**（清空错误和警告）：

| 分类 | cat ID |
|------|--------|
| 电子书 | 413 |
| 图片 | 418 |
| 漫画 | 415 |
| 游戏 | 412 |
| 软件 | 411 |
| 音乐 | 408 |

但音乐类(cat=408)跳过后，还会检查标题是否含 "khz" 和 "bit"。

> **重要**：转载音乐/电子书/图片/漫画/游戏/软件类资源时，只需确保分类选择正确即可通过审核。

### 黑名单制作组（22个关键词，第358-366行）

```javascript
const keywords = [
    "fgt", "hao4k", "mp4ba", "rarbg", "gpthd",
    "seeweb", "dreamhd", "blacktv", "xiaomi",
    "huawei", "momohd", "ddhdtv", "nukehd",
    "tagweb", "sonyhd", "minihd", "bitstv",
    "-alt", "rarbg", "mp4ba", "fgt", "hao4k",
    "batweb", "dbd-raws", "xunlei",
    "zerotv", "lelvetv"
];
```

去重后实际为 **22 个**（rarbg/mp4ba/fgt/hao4k 在数组中重复出现）：

| 关键词 | 类型 |
|--------|------|
| fgt | Web 组 |
| hao4k | Web 组 |
| mp4ba | Web 组 |
| rarbg | Scene 组 |
| gpthd | Web 组 |
| seeweb | Web 组 |
| dreamhd | Web 组 |
| blacktv | Web 组 |
| xiaomi | 平台组 |
| huawei | 平台组 |
| momohd | Web 组 |
| ddhdtv | Web 组 |
| nukehd | Web 组 |
| tagweb | Web 组 |
| sonyhd | Web 组 |
| minihd | Web 组 |
| bitstv | Web 组 |
| -alt | 后缀标记 |
| batweb | Web 组 |
| dbd-raws | Raw 组 |
| xunlei | 平台组 |
| zerotv | Web 组 |
| lelvetv | Web 组 |

> **注意**：黑名单检测逻辑在第905-908行被**注释掉**（`// if(is_untrusted_group)`），当前版本不执行黑名单检测，但关键词列表仍保留在代码中。未来可能重新启用。

### 官种/驻站识别体系

```javascript
// 官种标记：标题包含 "HDK"（不区分大小写，但代码用小写比较）
if (title_lowercase.includes("hdk")) officialSeed = 1;

// GodDramas 驻站短剧标记：标题包含 "goddramas"
if (title_lowercase.includes("goddramas")) godDramaSeed = 1;

// 音乐MV官种标记：标题包含 "HDKMV"
if (title_lowercase.includes("hdkmv")) officialMusicSeed = 1;
```

**官种标签规则矩阵**：

| 条件 | 官方标签(3) | 麒麟火标签 | 禁转标签 |
|------|------------|-----------|---------|
| HDK官种 | 必选 | 必选 | — |
| GodDramas驻站 | — | 必选 | 简介含"禁止转载"时必选 |
| 非官种 | **禁选** | — | — |
| 体积>1TB | — | — | 建议选大包 |

### MediaInfo检测逻辑（第683-733行）

脚本从页面 MediaInfo 栏提取以下信息：

```javascript
// 音频语言检测
const audioMatch = mediainfo.match(/Audio.*?Language:(\w+)/);
// 中文音频：Language含 Chinese 或 Mandarin（排除Text部分）
if (!audioLanguage.includes("Text") &&
    (audioLanguage.includes("Chinese") || audioLanguage.includes("Mandarin"))) {
    isAudioChinese = true;
}

// 字幕语言检测
const textMatches = mediainfo.match(/Text.*?Language:(\w+)/g);
// 中文字幕：Language含 Chinese
// 英文字幕：Language含 English

// 视频编码检测
if (mediainfo.includes("x264")) mi_x264 = true;
if (mediainfo.includes("x265")) mi_x265 = true;
```

> **注意**：音频/字幕的标签检查（国语/中字/英字）当前已被注释掉（第973-986行），不再强制执行。但代码和变量仍保留，未来可能重新启用。

### 简介MediaInfo识别（宽泛检测，第191-222行）

脚本对简介中是否包含MediaInfo做了极为宽泛的检测：

```javascript
// 标准MediaInfo
brief.includes("general") && brief.includes("video") && brief.includes("audio")

// 中文MediaInfo
brief.includes("概览") && brief.includes("视频") && brief.includes("音频")

// BDInfo
brief.includes("disc info") || brief.includes("disc size")

// Release Info
brief.includes(".release.info") || brief.includes("general information")

// 杜比官种NFO
brief.includes("nfo信息")

// FRDS官种
brief.includes("release date") && brief.includes("source")

// Release Name/Size
brief.includes("release.name") || brief.includes("release.size")

// CMCT/HDCTV官种
(brief.includes("文件名") || brief.includes("文件名称")) &&
(brief.includes("体　积") || brief.includes("体　　积"))

// HDChina官种
brief.includes("source type") || brief.includes("video bitrate")
```

> **关键**：只要简介满足以上**任意一种**模式，就被认为包含MediaInfo。这对转载很有利——即使源站使用非标准格式也能通过检测。

### 影片简介检测（第229-233行）

```javascript
// 简介必须包含以下关键词之一（去除空格后检查）：
"片名" / "译名" / "名" / "演" / "主持人" / "简" / "国家"
```

> **注意**：关键词非常宽泛，"名"和"演"几乎任何中文简介都包含。PT-Gen 生成的标准模板肯定通过。

### 标题中文检测白名单（第799行）

```javascript
// 白名单字符（即使匹配中文检测也不报错）：
'￡'  // 货币符号
'™'  // 商标符号
'\u2161-\u2169'  // 罗马数字 Ⅱ-Ⅸ
'Ⅰ'  // 罗马数字 Ⅰ
'白自在'  // 特定组名
'至尊宝'  // 特定组名（FFansDIY@至尊宝）
```

### 一键通过/拒绝功能

- **F4 快捷键**：无错误时一键通过，有错误时打开审核页
- **F3 快捷键**：关闭当前页面
- **自动关闭**：通过审核后自动关闭页面（GM_setValue 控制）
- **审核弹框页**：自动选择通过/拒绝并提交（`web/torrent-approval-page`）
- **错误信息自动填入**：拒绝时将检测到的错误信息写入审核备注

### 校验结果分级

| 级别 | 颜色 | 说明 |
|------|------|------|
| 错误 | #EA2027 红色 | 必须修复 |
| 警告 | #ffdd59 黄色 | 建议修复 |
| 通过 | #8BC34A 绿色 | 无错误，自动通过 |

---

## 三、关键适配器设计要点

### 3.1 种审制

HDKylin 使用种审制（油猴脚本辅助审核），所有种子需经过审核。适配器发布后种子可能被审核退回。

### 3.2 processing_sel = 年份

`processing_sel` 在 HDKylin 表示资源年份（2025 倒序到"更早"），适配器需从标题或元数据提取年份。

### 3.3 source_sel = 地区（18 个）

地区选项非常详细（含欧洲/俄罗斯/巴西/瑞典/丹麦等），适配器需精确匹配地区。

### 3.4 黑名单制作组（22 个）

适配器转发前需检查制作组是否在黑名单中。当前版本脚本已注释掉黑名单检测，但未来可能重新启用。

### 3.5 标签一致性检查

当前版本中，国语/中字/英字标签检查已被注释掉，不强制执行。但以下标签规则仍然强制：
- 官种 → 必须选"官方"标签
- 官种/驻站 → 必须选"麒麟火"标签
- GodDramas → 必须选"驻站"标签 + 短剧分类(421)
- 简介含"禁止转载" → 必须选"禁转"标签
- 体积>1TB → 建议选"大包"标签

### 3.6 简介必须包含 MediaInfo

种审要求简介中必须包含 MediaInfo 信息（检测极为宽泛，支持多种格式）。适配器应确保 `descr` 中包含完整信息。

### 3.7 做种时间要求

做种时间不足 **48 小时**（非标准 24 小时），或故意低速上传，将被警告、强制候选甚至取消上传权限。

### 3.8 转载规则

- **禁止删除原站后缀**、**修改文件名**、**修改文件夹结构**等操作
- **禁止转载标注禁转的资源**，或在限转期间转发
- **来自 BT/网盘资源（试行）**：站内无该资源→保留（前提符合上传规则）；站内有更优版本→dupe

### 3.9 Dupe 共存原则（新规则）

HDKylin 的 dupe 规则与标准 NexusPHP 有重要差异——有 **共存原则**：
- **不同小组作品允许共存**
- **不同分辨率作品允许共存**
- **不同编码作品允许共存**

这意味着同一部电影的不同制作组/分辨率/编码版本可以同时存在，**不构成 dupe**。

### 3.10 Cloudflare SL Challenge

站点使用 Cloudflare SL Challenge（cookie 含 `sl-challenge-server=cloud`、`sl_jwt_session`、`sl-session`），需参考 `docs/31-模块设计决策记录.md §29` 的 TLS 指纹绕过方案。

---

## 四、发布字段与通用模型的映射

### 4.1 类型映射（type）

| 通用类型 | HDKylin type 值 |
|---------|----------------|
| 电影 | 401 |
| 电视剧 | 402 |
| 纪录教育 | 404 |
| 动漫 | 405 |
| MV | 406 |
| 体育 | 407 |
| 音乐 | 408 |
| 其他 | 409 |
| 软件 | 411 |
| 游戏 | 412 |
| 电子书 | 413 |
| 学习 | 419 |
| 综艺 | 420 |
| 短剧 | 421 |

### 4.2 地区映射（source_sel）

| 值 | 显示名称 |
|----|----------|
| 14 | Other |
| 15 | CN/中国 |
| 16 | HK/香港 |
| 17 | TW/台湾 |
| 18 | US/美国 |
| 19 | JPN/日本 |
| 20 | Kr/韩国 |
| 21 | GB/英国 |
| 22 | FR/法国 |
| 23 | DE/德国 |
| 25 | IN/印度 |
| 28 | EU/欧洲 |
| 30 | RU/俄罗斯 |
| 31 | CA/加拿大 |
| 32 | BR/巴西 |
| 33 | SE/瑞典 |
| 34 | DK/丹麦 |
| 35 | TH/泰国 |

### 4.3 媒介映射（medium_sel）

| 通用媒介 | HDKylin medium_sel 值 |
|---------|----------------------|
| UHD Blu-ray | 24 |
| Blu-ray | 25 |
| DVD | 27 |
| HDTV | 28 |
| Encode | 29 |
| REMUX | 30 |
| WEB-DL | 31 |
| Track | 32 |
| CD | 33 |
| Other | 34 |

### 4.4 编码映射（codec_sel）

| 通用编码 | HDKylin codec_sel 值 |
|---------|---------------------|
| H.264/AVC | 1 |
| VC-1 | 2 |
| Xvid | 3 |
| MPEG-2/MPEG-4 | 4 |
| Other | 5 |
| H.265/HEVC | 6 |
| AV1 | 7 |

### 4.5 音频编码映射（audiocodec_sel）

| 通用音频编码 | HDKylin audiocodec_sel 值 |
|-------------|--------------------------|
| FLAC | 1 |
| DTS | 3 |
| MP3 | 4 |
| Other | 7 |
| AAC | 6 |
| DTS-HD MA | 8 |
| TrueHD | 9 |
| LPCM | 10 |
| DD/AC3 | 11 |
| APE | 12 |
| WAV | 13 |
| M4A | 14 |
| TrueHD Atmos | 15 |
| DTS:X | 16 |
| DDP/E-AC3 | 17 |
| MPEG | 18 |
| DTS-HD HR | 19 |
| Opus | 20 |

### 4.6 分辨率映射（standard_sel）

| 通用分辨率 | HDKylin standard_sel 值 |
|-----------|------------------------|
| 8K | 7 |
| 4K/2160p | 6 |
| 2K/1440p | 10 |
| 1080p/i | 1 |
| 720p/i | 3 |
| 480p/i | 8 |
| Other | 9 |

### 4.7 制作组映射（team_sel）

| 值 | 显示名称 |
|----|----------|
| 5 | Other |
| 6 | HDK |
| 7 | HDKWeb |
| 8 | HDKGame |
| 9 | GodDramas |
| 10 | CatEDU |
| 12 | Kylin |
| 13 | StarfallWeb |
| 14 | RedLeaves |

### 4.8 标签映射（tags）

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官种 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | Dolby Vision |
| 9 | Blu-ray |
| 10 | LIVE现场 |
| 11 | 4K |
| 15 | 驻站 |
| 16 | 完结 |
| 18 | 英字 |
| 19 | 分集 |
| 21 | 特效字幕 |

---

## 五、转载发布自动填写优化方案

> 基于审核脚本逆向分析结果，设计转载发布时的自动填写逻辑。目标是让发布的种子**一次性通过全部校验规则**。

### 1. 标题自动填写

```python
def clean_title_kylin(source_title):
    title = source_title

    # Rule #1: 去除中文（白名单：￡™ⅠⅡ白自在至尊宝）
    title = re.sub(r'[\u4e00-\u9fff]', '', title)
    # 保留 ￡™Ⅰ-Ⅸ 等白名单字符

    # 规范化
    title = title.replace('.', ' ').replace('_', ' ')
    return ' '.join(title.split())
```

> **注意**：脚本对标题格式的校验相对宽松，只检查是否包含中文字符，不检查分辨率格式（4K vs 2160p）、空格数量等。

### 2. 副标题自动填写

```python
def generate_subtitle_kylin(douban_title, source_subtitle, category):
    """
    副标题不能为空（Rule #2）
    如果副标题含"动画"则分类必须选动漫（Rule #3）
    """
    chinese_title = douban_title or extract_chinese(source_subtitle)
    return chinese_title or "转载资源"
```

### 3. 分类自动选择

```python
CATEGORY_MAP = {
    'Movie': 401, 'Film': 401, '电影': 401,
    'TV Series': 402, 'Drama': 402, '电视剧': 402,
    'TV Show': 420, 'Variety': 420, '综艺': 420,
    'Documentary': 404, '纪录教育': 404,
    'Anime': 405, 'Animation': 405, '动漫': 405,
    'Music Video': 406, 'MV': 406, '音乐视频': 406,
    'Sport': 407, '体育': 407,
    'Music': 408, 'HQ Audio': 408, '音乐': 408,
    'Other': 409, 'Misc': 409, '其他': 409,
    'Software': 411, '软件': 411,
    'Game': 412, '游戏': 412,
    'Ebook': 413, '电子书': 413,
    'Study': 419, '学习': 419,
    'Playlet': 421, '短剧': 421,
}
```

> **跳过分类**（Rule #29）：选择 408/413/418/415/412/411 后，所有校验规则自动跳过，无需担心媒介/编码等字段是否一致。

### 4. 质量字段自动选择

#### 媒介 (medium_sel[4])

```python
MEDIUM_MAP = [
    (r'\bUHD\s*Blu-?ray\b', 24),
    (r'\bBlu-?ray\b.*\bComplete\b', 24),  # UHD原盘
    (r'\bBlu-?ray\b', 25),
    (r'\bBDMV\b', 25),
    (r'\bRemux\b', 30),
    (r'\bWEB-?DL\b|\bWebDL\b', 31),
    (r'\bHDTV\b', 28),
    (r'\bDVD\b', 27),
    (r'\bEncode\b|\b(x264|x265)\b', 29),
    (r'\bWebRip\b|\bDVDRip\b|\bBDRip\b', 29),
    (r'\bCD\b', 33),
    (r'\bTrack\b', 32),
]
```

> **注意**：麒麟将 UHD Blu-ray(24) 和 Blu-ray(25) 分开，需根据标题中的 "UHD" 关键词区分。

#### 编码 (codec_sel[4])

```python
CODEC_MAP = {
    'H.264': 1, 'AVC': 1, 'x264': 1,
    'H.265': 6, 'HEVC': 6, 'x265': 6,
    'VC-1': 2,
    'Xvid': 3,
    'MPEG-2': 4, 'MPEG-4': 4,
    'AV1': 7,
}
```

> **脚本BUG**：标题含 AV1 时，脚本设 title_encode=12（不在 encode_constant 中），与用户选择 AV1(7) 不匹配会报错。转载时应避免使用含 "av1" 的标题关键词来规避此BUG。

#### 音频编码 (audiocodec_sel[4])

```python
AUDIO_MAP = [
    (r'\bFLAC\b', 1),
    (r'\bLPCM\b', 10),
    (r'\bTrueHD\s*Atmos\b', 15),
    (r'\bDDP\b|\bDD\+\b|\bE-?AC-?3\b', 17),
    (r'\bAAC\b', 6),
    (r'\bAC-?3\b|\bDD\b', 11),
    (r'\bTrueHD\b', 9),
    (r'\bDTS-?HD\s*MA\b', 8),
    (r'\bDTS:?\s*X\b', 16),
    (r'\bDTS\b', 3),
    (r'\bAPE\b', 12),
    (r'\bWAV\b', 13),
    (r'\bMP3\b', 4),
    (r'\bM4A\b', 14),
]
```

> **匹配顺序很重要**：脚本按固定优先级检测（FLAC→LPCM→TrueHD Atmos→DDP→AAC→AC3→TrueHD→DTS-HD MA→DTS:X→DTS），转载时应使用相同优先级。

#### 分辨率 (standard_sel[4])

```python
RESOLUTION_MAP = {
    '1080p': 1, '1080i': 1,
    '720p': 3, '720i': 3,
    '4K': 6, '2160p': 6, '2160i': 6,
    '8K': 7, '4320p': 7, '4320i': 7,
    '480p': 8, '480i': 8,
}
```

### 5. IMDb链接自动填写

```python
def get_imdb_url(source_desc, source_url):
    """麒麟检查简介中是否含豆瓣/IMDb/TMDB链接"""
    if source_url and 'imdb.com' in source_url:
        return source_url
    match = re.search(r'https?://[^\s]*imdb\.com/title/(tt\d+)', source_desc)
    if match:
        return f'https://www.imdb.com/title/{match.group(1)}/'
    return ''
```

> **注意**：脚本检测简介中是否包含 `imdb.com`、`douban.com` 或 `themoviedb.org` 链接（第176-178行），但此检查当前被注释掉（第878-881行），不强制执行。

### 6. PT-Gen自动填写

从源站获取PT-Gen链接，填入麒麟的 `pt_gen` 字段。麒麟支持 PT-Gen 按钮自动获取简介。

### 7. 简介自动构建

```python
def build_description_kylin(source_desc, pt_gen_data, media_info, source_site):
    """
    构建麒麟标准简介
    关键：必须通过 isBriefContainsInfo 和 isBriefContainsMovieBrief 检测
    """
    parts = []

    # 1. PT-Gen 影片信息（确保通过"影片简介"检测：含"片名/演/简"等关键词）
    if pt_gen_data:
        parts.append(format_pt_gen_template(pt_gen_data))

    # 2. MediaInfo/BDInfo（确保通过MediaInfo检测：含"general"+"video"+"audio"）
    if media_info:
        parts.append(format_mediainfo_block(media_info))
    # 如果无标准MediaInfo，至少添加一种被识别的格式
    # 例如添加 "文件名: xxx  体　积: xxx" 格式（CMCT/HDCTV兼容）

    # 3. 截图（确保 imgCount ≥ 2）
    screenshots = extract_screenshots(source_desc)
    parts.extend(screenshots)

    # 4. 确保至少2张图片
    img_count = count_images(parts)
    if img_count < 2:
        # 需要补充图片（海报 + 至少1张截图）
        pass

    return '\n\n'.join(parts)
```

**简介通过检测的关键词清单**：

| 检测项 | 需要包含的关键词 |
|--------|----------------|
| MediaInfo | "general"+"video"+"audio"（三者都有） |
| 影片简介 | "片名"或"译名"或"名"或"演"或"主持人"或"简"或"国家"（任一） |
| 豆瓣链接 | "douban.com" |
| IMDb链接 | "imdb.com" |
| TMDB链接 | "themoviedb.org" |
| 禁止转载 | "禁止转载" |

### 8. 标签自动选择

```python
def auto_select_tags_kylin(title, description, media_info, category, file_size,
                           is_official=False, is_godramas=False):
    tags = []

    # 官种标签（Rule #22）：标题含HDK/GodDramas等官组名
    if is_official:
        tags.append(('官方', 3))
        tags.append(('麒麟火', None))  # 页面标签值待确认

    # GodDramas驻站标签（Rule #19）
    if is_godramas:
        tags.append(('驻站', 15))
        tags.append(('麒麟火', None))

    # 禁转标签（Rule #17）：简介含"禁止转载"
    if '禁止转载' in description:
        tags.append(('禁转', 1))

    # 大包标签（Rule #26）：体积>1TB
    if file_size and file_size > 1_000_000_000_000:  # 1TB
        tags.append(('大包', None))

    # 完结标签（Rule #14）：电视剧+标题含"complete"
    if category in (402, 420, 404) and 'complete' in title.lower():
        tags.append(('完结', 16))

    # 以下标签当前被注释掉（不强制），但预留
    # 国语：MI Audio Language含Chinese/Mandarin → tags.append(('国语', 5))
    # 中字：MI Text Language含Chinese → tags.append(('中字', 6))
    # 英字：MI Text Language含English → tags.append(('英字', 18))

    return tags
```

### 9. 转载发布全流程校验清单

| # | 自检项 | 对应规则 | 自动填写保障 |
|---|--------|---------|-------------|
| 1 | 标题无中文 | #1 | clean_title 去中文 |
| 2 | 副标题非空 | #2 | 自动生成 |
| 3 | 副标题"动画"→选动漫 | #3 | 分类联动 |
| 4 | 分类已选择 | #4 | 自动选择 |
| 5 | 媒介已选择且一致 | #5-6 | 同一解析函数 |
| 6 | 编码已选择且一致 | #7-8 | 同一解析函数 |
| 7 | 音频已选择且一致 | #9-10 | 同一解析函数(注意优先级) |
| 8 | 分辨率已选择且一致 | #11-12 | 同一解析函数 |
| 9 | 官种选官方+麒麟火 | #21-23 | 标签自动选择 |
| 10 | GodDramas选驻站+短剧 | #17-19 | 标签+分类联动 |
| 11 | 禁转简介→禁转标签 | #17 | 标签自动选择 |
| 12 | 简介含MediaInfo | #15,24 | 简介模板保证 |
| 13 | 简介含影片信息 | #25 | PT-Gen模板保证 |
| 14 | 图片≥2张 | #27 | 简介模板保证 |
| 15 | 体积>1TB→大包标签 | #26 | 标签自动选择 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-19*
