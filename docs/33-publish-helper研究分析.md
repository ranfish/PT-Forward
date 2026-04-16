# Publish Helper 研究分析

> 来源：`examples/publish-helper/`
> 分析时间：2026-04-16
> 用途：为 PT-Forward 站点适配器设计提供参考

---

## 一、项目概览

Publish Helper 是一个 Python 编写的 PT 资源发布辅助工具，提供 PyQt6 桌面 GUI 和 Flask REST API 双前端。

### 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Python 3.9+ |
| GUI | PyQt6 |
| API | Flask + flask-cors |
| MediaInfo | pymediainfo（封装 libmediainfo） |
| 截图 | OpenCV + Pillow + numpy |
| 种子创建 | torf |
| PT-Gen | requests HTTP 调用 |
| 图床 | 6种图床插件（lsky-pro、bohe、chevereto 等） |
| 部署 | Docker + nginx 反向代理 |

### 架构

```
main_gui.py / main_api.py          ← 双入口
    │
    ├── gui/startgui.py (2279行)    ← PyQt6 GUI
    ├── api/startapi.py (2379行)    ← Flask REST API (20+ 端点)
    │
    └── core/                        ← 核心业务逻辑
        ├── tool.py (976行)          ← 设置、缩写映射、PT-Gen 解析、中文处理
        ├── rename.py (490行)        ← 视频信息提取、模板命名、文件操作
        ├── mediainfo.py (209行)     ← MediaInfo 格式化输出
        ├── ptgen.py                 ← PT-Gen API 客户端
        ├── screenshot.py           ← 视频截图 + 缩略图
        ├── picturebed.py           ← 图床上传（6种）
        ├── autofeed.py             ← Auto-Feed 链接生成
        └── settings_tool.py        ← 配置管理
```

---

## 二、值得借鉴的设计

### 2.1 MediaInfo 缩写映射表 (`abbreviation.json`)

将 MediaInfo 输出的原始字符串映射为 PT 站通用缩写，覆盖 6 大类。

#### 分辨率（精确 + 模糊二级）

精确匹配：
```
"7 680 pixels"  → "4320p"
"3 840 pixels"  → "2160p"
"2 560 pixels"  → "1440p"
"1 920 pixels"  → "1080p"
"1 440 pixels"  → "720p"
"1 280 pixels"  → "720p"
"720 pixels"    → "480p"
"640 pixels"    → "480p"
```

模糊匹配（`min_widths` 阈值，用于精确匹配未命中时）：
```
宽度 ≥ 9600 → "8640p"
宽度 ≥ 4608 → "4320p"
宽度 ≥ 3200 → "2160p"
宽度 ≥ 2240 → "1440p"
宽度 ≥ 1600 → "1080p"
宽度 ≥ 900  → "720p"
宽度 ≥ 533  → "480p"
```

调用流程（`rename.py:200-204`）：
1. 取较长边（width vs height）
2. 精确查 `abbreviation.json`
3. 若结果以 `" pixels"` 结尾 → 未命中精确映射
4. 调用 `approximate_resolution_by_width()` 用 `min_widths` 阈值模糊匹配

#### 视频编码

```
"HEVC" → "HEVC"    ← 原盘
"AVC"  → "AVC"     ← 原盘
"x264" → "x264"    ← 实际走 writing_library 检测，不从这里
"x265" → "x265"
"x266" → "x266"
"AV1"  → "AV1"
```

关键逻辑（`rename.py:167-173`）—— x264/x265 判定优先走 `writing_library`：
```python
if track.writing_library:
    if 'x264' in track.writing_library:
        video_codec = 'x264'    # 覆盖之前的 AVC
    if 'x265' in track.writing_library:
        video_codec = 'x265'    # 覆盖之前的 HEVC
    if 'x266' in track.writing_library:
        video_codec = 'x266'
```

这是区分原盘（AVC/HEVC）和压制（x264/x265）的正确做法。

#### HDR 格式

```
"Dolby Vision, Version 1.0, dvhe.05.06, BL+RPU"                → "DV"
"SMPTE ST 2094 App 4, Version 1, HDR10+ Profile B compatible"   → "HDR10+"
"SMPTE ST 2086, HDR10 compatible"                                → "HDR10"
"HDR Vivid, Version 1"                                           → "HDR"
```

#### 帧率

```
"120.000 FPS" → "120FPS"    ← 高帧率保留
"60.000 FPS"  → "60FPS"
"50.000 FPS"  → "50FPS"
"48.000 FPS"  → "48FPS"
"30.000 FPS"  → ""          ← 常规帧率省略
"29.970 FPS"  → ""
"24.000 FPS"  → ""
"23.976 FPS"  → ""
```

#### 音频编码

```
"Dolby Digital Plus with Dolby Atmos" → "Atmos DDP"
"Dolby TrueHD with Dolby Atmos"       → "Atmos TrueHD"
"DTS-HD Master Audio"                 → "DTS-HD MA"
"Dolby Digital Plus"                  → "DDP"
"Dolby Digital"                       → "DD"
"HE-AAC"                              → "AAC"
```

使用 `commercial_name` 字段（`rename.py:178-179`）。

#### 声道布局

```
"L R C LFE Ls Rs Lb Rb"   → "7.1"
"C L R LS RS LFE"          → "5.1"
"L R C LFE Ls Rs"          → "5.1"
"C L R Ls Rs LFE"          → "5.1"
"L R"                      → "2.0"
```

4 种不同的 5.1 声道排列都映射到 `"5.1"`。

### 2.2 PT-Gen 正则提取 (`rename.py:13-127`)

PT-Gen 生成的描述使用固定全角空格格式：
```
◎片　　名　Spider-Man: No Way Home
◎译　　名　蜘蛛侠：英雄无归 / 蜘蛛侠：不战无归
◎年　　代　2021
◎产　　地　美国
◎类　　别　动作 / 冒险 / 奇幻
◎主　　演　汤姆·赫兰德 / 赞达亚
◎集　　数　12
```

#### 标题提取（`rename.py:26-57`）

```python
pattern = r'◎片　　名　(.*?)\n|◎译　　名　(.*?)\n'
matches = re.findall(pattern, description)
```

然后用正则分离出三类标题：
- 英文标题：`^[A-Za-z\-\:\s\(\)\'\.\d]+$`（纯 ASCII）
- 中文原始标题：含 `\u4e00-\u9fa5`
- 其他别名：排除前两者

#### 类别提取（`rename.py:19`）

```python
categories_match = re.search(r'◎类　　别\s*([^\n]*)', description)
```

#### 季数提取（`rename.py:100-118`）——支持 6 种格式

```python
r'Season (\d+)|season (\d+)| (\d+)st|第(\d+)季|第([零一二三四五六七八九十百千万]+)季'
```

中文数字通过 `chinese_to_int()`（`tool.py:755-797`）转换。

#### 集数/年份

```python
episodes_match = re.search(r'◎集　　数\s*(\d+)', description)
year_match = re.search(r'◎年　　代\s*(\d{4})', description)
if not year_match:
    year_match = re.search(r'◎上映日期\s*(\d{4})', description)  # 回退
```

#### 演员（最多 5 个中文名）

```python
r'(◎主　　演|◎演　　员)\s*((?:[\s　]*.*?(?:\n|$))*)'
# 逐行提取 [\u4e00-\u9fa5·]+
```

### 2.3 启发式编码识别 (`tool.py:833-964`)

从标题字符串中用 `in` 判断，**结合 PT-Gen 描述和 MediaInfo**。

#### 分类（基于 PT-Gen 描述）

```
'纪录'    → 纪录
'体育'    → 体育
'动画'    → 动画
'综艺'/'脱口秀' → 综艺
'短片'    → 短剧
其他      → 默认传入值（电影/剧集）
```

#### 地区（基于 PT-Gen 描述）

```
'美国'/'英国'/'德国'/'法国' → 欧美
'大陆'                       → 大陆
'香港'/'台湾'                 → 港台
'日本'                       → 日本
'韩国'                       → 韩国
'印度'                       → 印度
```

#### 分辨率（从标题匹配）

```
'3840p'/'3840P'/'3840i' → '8K'      ← ⚠️ 有误，3840 是 4K
'2160p'/'2160P'/'2160i' → '4K'
'1080p'/'1080P'         → '1080p'
'1080i'                 → '1080i'
'720p'/'720P'           → '720p'
'480p'/'480P'           → '480p'
```

#### 音频编码（从标题匹配，if 链非 elif）

```
'AAC'                              → 'AAC'
'AC3'/'DD'                         → 'AC3'
'EAC3'/'E-AC3'/'DDP'/'DD+'        → 'EAC3'
'DTS' + 'HD' + 'MA'                → 'DTS-HDMA'
'DTS' (其他)                       → 'DTS'
'Atmos'/'ATMOS'                    → 'Atmos'
'TrueHD'/'TRUEHD'                  → 'TrueHD'
'Flac'/'FLAC'                      → 'Flac'
```

#### 视频编码（从标题匹配，if 链非 elif）

```
'H264'/'H.264'/'h264'/'AVC'  → 'H264'
'H265'/'H.265'/'h265'/'HEVC' → 'H265'
'H266'/'H.266'/'VVC'         → 'H266'
'X264'/'x264'                 → 'X264'  ← 后出现会覆盖 H264
'X265'/'x265'                 → 'X265'  ← 后出现会覆盖 H265
'X266'/'x266'                 → 'X266'
'AV1'/'av1'                   → 'AV1'
```

#### 媒介推断（最复杂，三层）

```python
# 第一层：WEB-DL（10种写法）
if 'WEB-DL' in source or title: medium = 'WEB-DL'

# 第二层：Blu-ray → 根据编码和容器细分
if 'Blu-ray' in source or title:
    if 'X26*' in video_codec:     medium = 'Encode'    # 压制
    elif 'Remux' in title:        medium = 'Remux'     # Remux
    elif 'mkv' in media_info:     medium = 'Remux'     # mkv 容器 → Remux
    # else: 不赋值（原盘，medium 为空）

# 第三层：HDTV / DVD
if 'HDTV' in title: medium = 'HDTV'
if 'DVD' in title:  medium = 'DVD'
```

### 2.4 其他值得借鉴的模式

#### PT-Gen 双 API 竞速（`startgui.py:262-274`）

同时启动两个 PT-Gen 线程（主/备 API），谁先返回用谁：
```python
self.get_pt_gen_thread = GetPtGenThread(primary_url, resource_url)
self.get_pt_gen_backup_thread = GetPtGenThread(backup_url, resource_url)
# 两个都 start()，第一个成功后设 flag，第二个被忽略
```

#### 配置自愈 + 三层优先级（`settings_tool.py:115-142`）

```
环境变量 > settings.json > 默认值
```
JSON 配置文件缺失时自动创建默认值，老版本字段自动迁移。

#### 图床插件模式（`picturebed.py`）

每种图床是独立函数，通过 JSON 配置映射 URL → 图床类型。新增图床只需：加 JSON 条目 + 写函数 + 加 elif。

#### Auto-Feed 链接系统（`autofeed.py`）

用户配置模板 URL，含 `{主标题}`、`{媒介}` 等占位符，替换后 base64 编码拼接到站点 URL。浏览器打开后油猴脚本 `auto_feed.js` 解析并填充表单。**无需在工具中维护认证逻辑**。

---

## 三、发现的问题

### 3.1 严重问题

| # | 问题 | 位置 | 影响 |
|---|------|------|------|
| 1 | 音频编码 if 链覆盖 | `tool.py:912-930` | 标题含多个编码关键字时，最后一个覆盖前面。如 `DTS-HD.MA.Atmos` 最终取 Atmos 而非 DTS-HD MA + Atmos |
| 2 | `3840p` 映射到 `8K` | `tool.py:894-895` | 3840×2160 是 4K/UHD，非 8K。`7680p` 才是 8K |
| 3 | `720i` 条件重复 | `tool.py:904-909` | 第二次 `720i` 赋值 `480i` 覆盖了第一次的 `720i` |
| 4 | DV 只映射一个 Profile | `abbreviation.json:32` | Dolby Vision 有多种 Profile（5/7/8），只有 dvhe.05.06 能精确匹配 |

### 3.2 设计缺陷

| # | 问题 | 位置 | 影响 |
|---|------|------|------|
| 5 | Blu-ray 原盘不赋值 | `tool.py:952-957` | Blu-ray 非Encode非Remux时 medium 为空字符串 |
| 6 | `'mkv' in media_info` 太粗糙 | `tool.py:956` | mkv 容器不一定就是 Remux，原盘 BDMV 也有 mkv |
| 7 | 未区分 UHD 和非 UHD | `tool.py:952` | UHD Blu-ray 和普通 Blu-ray 混在一起 |
| 8 | 无多站点适配 | 架构层面 | 只做单站 auto-feed，无跨站点字段映射 |
| 9 | 无 dupe 检查 | 缺失功能 | 不检查站内是否已有资源 |
| 10 | 无 BDInfo | 缺失功能 | 只支持 MediaInfo，无法处理原盘 |
| 11 | abbreviation.json 不支持正则 | `tool.py:480` | `abbreviation_map.get()` 只能精确匹配，无法匹配 DV 的多种变体 |
| 12 | 分类用 if 非 elif | `tool.py:862-871` | 类别含多个关键字时（如体育纪录片），取最后一个而非最精确的 |

### 3.3 代码质量问题

| # | 问题 | 位置 |
|---|------|------|
| 13 | 全局变量线程同步 | `startgui.py:29-33`，用 global bool 做 QThread 通信 |
| 14 | `print()` 代替 logger | 所有 core 模块 |
| 15 | `(bool, str)` 返回值 | 所有 core 模块，应使用异常 |
| 16 | 简单 `str.replace` 模板 | `rename.py:289-310`，非结构化，空值会留下占位符 |

---

## 四、对 PT-Forward 的借鉴方案

### 4.1 两级映射架构

publish-helper 只做了 Level 1 且不完善，我们需要设计两级：

```
Level 1: MediaInfo/标题 原始值 → 标准中间值
         ("Dolby Vision, Version 1.0, dvhe.05.06, BL+RPU" → "DV")
         ("3 840 pixels" → "2160p")
         ("DTS-HD Master Audio" → "DTS-HDMA")

Level 2: 标准中间值 → 站点具体表单值
         ("DV" → HDFans tag value 12)
         ("2160p" → HDVideo standard_sel value 8)
         ("DTS-HDMA" → HDDolby audiocodec_sel value 1)
```

### 4.2 Level 1 改进点

| 改进 | 说明 |
|------|------|
| 用正则匹配替代精确匹配 | DV 有多种 Profile，需要 `Dolby Vision, Version \d+` 正则 |
| 用 elif 替代 if | 编码识别链不应后覆盖前 |
| 加入 MediaInfo 语言检测 | 区分音频语言（国语/粤语/英语）和字幕语言（中字/英字） |
| 加入 UHD 判定 | 结合分辨率 + 媒介：2160p + Blu-ray → UHD Blu-ray |
| 补全编码映射 | DTS:X、LPCM、DSD、APE、OPUS、WAV 等 |
| 补全声道布局 | 6.1、7.1.4、Atmos 对象数 |

### 4.3 Level 2 站点映射

Level 2 映射数据已在各站点文档中采集完毕（`docs/32-站点适配器设计/*.md`），每个站点的 YAML 配置中的 `mappings` 部分就是 Level 2 映射。

### 4.4 可直接复用的代码

| 模块 | 来源文件 | 用途 |
|------|----------|------|
| `chinese_to_int()` | `tool.py:755-797` | 季数中文数字转换 |
| `writing_library` 检测 x26\* | `rename.py:167-173` | 区分原盘/压制编码 |
| `commercial_name` 取音频 | `rename.py:178-179` | MediaInfo 音频商业名称 |
| `min_widths` 阈值表 | `abbreviation.json:2-10` | 模糊分辨率匹配 |
| PT-Gen 描述正则 | `rename.py:13-127` | 片名/年份/类别/演员/季数完整提取 |
| 季数多格式正则 | `rename.py:101` | 6种季数格式 + 中文数字支持 |
| MediaInfo 格式化 | `mediainfo.py:10-209` | 5种轨道类型的格式化输出 |

---

*研究完成，待 PT-Forward 开发到站点适配器阶段时参考*
