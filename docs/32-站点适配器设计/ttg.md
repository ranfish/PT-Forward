# 套套哥 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 套套哥 |
| 域名 | totheglory.im |
| 框架 | TTG 自研（非标准 NexusPHP，有 NP 血统但大量定制） |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（offers.php） |
| PT-Gen | 是 |
| IMDb | 是（imdb_c） |
| 豆瓣 | 是（douban_id） |
| 匿名发布 | 是（anonymity: yes/no） |
| NFO | 是 |
| **标签** | **无标签系统**，"禁转"在列表页显示但详情页不呈现 |

## Tracker URL
需从发布页面确认，种子文件名需去 `[TTG]` 前缀

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `subtitle` | 否 | |
| IMDb | `imdb_c` | 否 | IMDb ID |
| 豆瓣 | `douban_id` | 否 | 豆瓣 ID |
| 简介 | `descr` | 是 | BBCode |
| 描述 | `description` | 否 | |
| 关键词 | `keywords` | 否 | |
| 分类 | `type` | 是 | **分辨率+内容混合分类，共 56 个** |
| 制作组 | `team` | 否 | **与 type 完全相同的选项列表** |
| 匿名 | `anonymity` | 否 | yes/no |
| 禁转 | `nodistr` | 否 | value=yes |
| H&R | `hr` | 否 | **与 type 完全相同的选项列表** |
| 高亮 | `highlight` | 否 | |
| 加粗 | `bold` | 否 | |

## 分类 (type) — 分辨率+内容混合分类体系

### 电影类

| ID | 名称 |
|----|------|
| 51 | 电影DVDRip |
| 52 | 电影720p |
| 53 | 电影1080i/p |
| 54 | BluRay原盘 |
| 108 | 影视2160p |
| 109 | UHD原盘 |

### 剧集类

| ID | 名称 |
|----|------|
| 69 | 欧美剧720p(单集) |
| 70 | 欧美剧1080i/p(单集) |
| 73 | 高清日剧 |
| 74 | 高清韩剧 |
| 75 | 大陆港台剧1080i/p(单集) |
| 76 | 大陆港台剧720p(单集) |
| 87 | 欧美剧包(全集) |
| 88 | 日剧包 |
| 90 | 华语剧包(全集) |
| 99 | 韩剧包 |

### 纪录片

| ID | 名称 |
|----|------|
| 62 | 纪录片720p |
| 63 | 纪录片1080i/p |
| 67 | 纪录片BluRay原盘 |

### 综艺

| ID | 名称 |
|----|------|
| 60 | 高清综艺 |
| 101 | 日本综艺 |
| 103 | 韩国综艺 |

### 动漫

| ID | 名称 |
|----|------|
| 58 | 高清动漫 |
| 111 | 动漫原盘 |

### 音乐

| ID | 名称 |
|----|------|
| 82 | (电影原声&Game)OST |
| 83 | 无损音乐FLAC&APE |
| 84 | 补充音轨 |
| 59 | MV&演唱会 |

### 体育/其他

| ID | 名称 |
|----|------|
| 57 | 高清体育节目 |
| 91 | MiniVideo |

### 游戏类

| ID | 名称 |
|----|------|
| 28 | PC |
| 47 | MAC |
| 5 | XBOX360 |
| 45 | XBOX to XBOX360 |
| 49 | XBLA |
| 105 | XBOX1 |
| 26 | PS2 |
| 46 | PS3 |
| 104 | PS4 |
| 29 | PSP |
| 107 | PSV |
| 110 | SWITCH |
| 44 | NDS |
| 106 | WIIU |
| 27 | WII |
| 43 | NGC |
| 48 | PSP兼容高清&标清 |
| 33 | PS3兼容高清 |
| 30 | Game Video |
| 31 | XBOX360兼容高清 |
| 93 | iPhone/iPad游戏 |

### 软件/书籍/移动端

| ID | 名称 |
|----|------|
| 77 | APPZ |
| 32 | Game Ebook |
| 56 | Ebook |
| 92 | iPhone/iPad视频 |
| 94 | iPad书籍 |
| 95 | iPhone/iPad软件 |

## 质量字段

**无任何标准质量下拉框。** TTG 没有以下字段：
- medium_sel（媒介）
- codec_sel（编码）
- audiocodec_sel（音频编码）
- resolution_sel（分辨率）
- source_sel（来源）
- processing_sel（处理方式）
- tags（标签）

**所有质量信息（分辨率、媒介、编码等）都编码在 type 分类中**，例如：
- `电影1080i/p` = 电影 + 1080 分辨率
- `BluRay原盘` = 电影 + Blu-ray 媒介
- `UHD原盘` = 电影 + UHD 媒介

## 制作组 team

**team 字段的选项列表与 type 完全相同**（56 个选项），这不是制作组选择，而是分类的重复。实际制作组信息从标题后缀解析（如 `-WiKi`、`-TTG`、`-NGB`、`-ARiN`）。

## 禁转标记

- **无标签系统**
- "禁转"通过 `nodistr=yes` 字段标记
- 在资源列表页显示，但资源详情页**不呈现**
- 列表页展示为特殊的"禁转"标识

## 标题属性解析（来自 examples/hdapt_auto_transfer）

TTG 的质量信息需从标题正则解析，参考 `examples/hdapt_auto_transfer/modules/crawler.py`：

### 默认值
```python
{'codec': 'x264', 'audio': 'DTS', 'resolution': '1080p', 'medium': 'Encode', 'team': 'other'}
```

### 编码检测
| 匹配 | 值 |
|------|-----|
| x265/HEVC | x265 |
| 默认 | x264 |

### 音频检测
| 匹配 | 值 |
|------|-----|
| Atmos | TrueHD Atmos |
| TrueHD | TrueHD |
| DD5.1/AC3 | AC3 |
| 默认 | DTS |

### 分辨率检测
| 匹配 | 值 |
|------|-----|
| 2160p/4K | 2160p |
| 1080i | 1080i |
| 720p | 720p |
| 默认 | 1080p |

### 媒介检测
| 匹配 | 值 |
|------|-----|
| HDTV | HDTV |
| WEB-DL/WEB | WEB-DL |
| BluRay + 编码关键词 | Encode |
| BluRay（无编码关键词） | BluRay |
| REMUX | REMUX |
| 默认 | Encode |

### 制作组检测
| 匹配 | 值 |
|------|-----|
| WiKi | WiKi |
| NGB | NGB |
| ARiN | ARiN |
| TTG | TTG |
| 默认 | other |

### TTG→HDA 分类映射（参考）
| TTG 分类 | HDA 分类 |
|----------|---------|
| UHD原盘 / 影视2160p | Movie UHD-4K |
| BluRay原盘 | Movies Blu-ray |
| 电影1080i/p | Movies 1080p |
| 电影720p | Movies 720p |
| 电影DVDRip | Movies DVDRip |
| 欧美剧* / 日剧* / 韩剧* / 华语剧* | TV SERIES |
| 纪录片* | Documentaries |
| (电影原声&Game)OST / 无损音乐FLAC&APE / 补充音轨 | HQ Audio |
| MV&演唱会 | Music Videos |
| 高清体育节目 | SPORTS |
| 高清动漫 / 动漫原盘 | Animations |
| *综艺 | TV SHOWS |

## 文件名清理

TTG 种子文件名需清理：
1. 去除 `[TTG]` 前缀：`re.sub(r'^\[TTG\]\s*', '', filename)`
2. 去除中文字符及之后内容：`re.sub(r'[\u4e00-\u9fa5]+.*\.torrent$', '.torrent', filename)`

## 特殊说明

1. **非标准 NP 框架**：TTG 是自研框架，表单结构与 NexusPHP 完全不同
2. **分类即质量**：56 个分类已包含分辨率+内容类型，无独立质量字段
3. **team 字段语义异常**：team 下拉框与 type 选项完全相同，实际制作组从标题解析
4. **无标签系统**："禁转"通过 nodistr 字段标记，列表页可见详情页不可见
5. **剧集按地区+分辨率细分**：欧美剧/日剧/韩剧/华语剧各有独立分类，分单集和包
6. **游戏分类极多**：PC/MAC/XBOX360/XBOX1/PS2/PS3/PS4/PSP/PSV/SWITCH/NDS/WIIU/WII/NGC 等
7. **Cloudflare 保护**：需要 cf_clearance cookie
8. **种子文件名含 [TTG] 前缀**：需清理
9. **hr 字段**：有 H&R（Hit and Run）标记，选项列表与 type 相同
10. **anonymity 字段**：支持匿名发布（yes/no 选择）
