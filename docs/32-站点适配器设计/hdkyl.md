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

## 二、种审脚本逆向分析

### 2.1 黑名单制作组（27 个关键词）

从 Greasyfork 油猴脚本 `hdkylin-torrent-assistant` 逆向提取：

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

### 2.2 group_constant 映射（脚本内部）

| 值 | 制作组名 |
|----|----------|
| 1 | HDKMV |
| 2 | HDKTV |
| 3 | HDKDIY |
| 5 | Other |
| 6 | HDK |
| 7 | HDKWeb |
| 8 | HDKGame |
| 9 | GodDramas |
| 10 | CatEDU |

### 2.3 种审检查项

脚本在审核时检查以下项目：

1. **简介必须包含 MediaInfo**（NFO/文件名/体积/Release Date/Source 等信息）
2. **禁止转载标记**：简介含"禁止转载"时必须选"禁转"标签
3. **制作组选择**：必须选择制作组
4. **标签一致性**：
   - 音频含国语 → 必须选"国语"标签
   - 字幕含中文 → 必须选"中字"标签
   - 字幕含英文 → 必须选"英字"标签
   - 体积 >1TB → 必须选"大包"标签（脚本中引用但页面上未见此标签）
5. **官种标签**：非官方组不能选官种标签
6. **剧集分集**：电视剧标题含集号时必须选"分集"标签
7. **黑名单制作组检测**：标题/简介含黑名单关键词时警告
8. **简介冗余图片检测**：检测是否包含不必要的影片参数图片
9. **IMDb/豆瓣/TMDB 链接检查**：验证是否包含外部信息链接

### 2.4 官种/驻站体系

- **官种**（标签值=3）：站方组发布的种子
- **驻站**（标签值=15）：长期驻站的短剧组种子
- 非官种选择官种标签会被审核拒绝

---

## 三、关键适配器设计要点

### 3.1 种审制

HDKylin 使用种审制（油猴脚本辅助审核），所有种子需经过审核。适配器发布后种子可能被审核退回。

### 3.2 processing_sel = 年份

`processing_sel` 在 HDKylin 表示资源年份（2025 倒序到"更早"），适配器需从标题或元数据提取年份。

### 3.3 source_sel = 地区（18 个）

地区选项非常详细（含欧洲/俄罗斯/巴西/瑞典/丹麦等），适配器需精确匹配地区。

### 3.4 黑名单制作组（27 个）

适配器转发前需检查制作组是否在黑名单中，黑名单组的资源将被拒绝。

### 3.5 标签一致性检查

种审脚本会检查标签与内容的一致性：
- 国语音轨 → 必须选"国语"标签
- 中文字幕 → 必须选"中字"标签
- 英文字幕 → 必须选"英字"标签

### 3.6 简介必须包含 MediaInfo

种审要求简介中必须包含 MediaInfo 信息（NFO/文件名/体积等）。适配器应确保 `technical_info` 字段和 `descr` 中包含完整信息。

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

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
