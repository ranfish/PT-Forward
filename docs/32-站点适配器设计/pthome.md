# 铂金家 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 铂金家|
| 站点地址 | https://pthome.net |
| 站点框架 | NexusPHP |
| 主题 | 自定义 |
| 特殊规则 | **与 HDHome（高清家园）互斥**，禁止互相转载；FGT/HDHome官组/Mp4Ba/RARBG 黑名单 |

> ⚠️ **互斥站点**：PTHome 与 HDHome 互相禁止转载作品。转种时需检查来源站，**禁止将 HDHome 的资源转发到 PTHome**。

---

## 一、发布页面表单字段分析

PTHome 有**两个独立的发布页面**：
- `upload.php` — 综合资源（电影/剧集/综艺/纪录/动漫/体育/游戏/软件/学习/其他/有声书/电子书）
- `upload_music.php` — 音乐专用页面（完全不同的表单结构）

### 1.1 综合发布页 (`upload.php`)

**提交地址**: `takeupload.php`（POST multipart/form-data）

**字段名无后缀**（裸名，如 `medium_sel` 而非 `medium_sel[4]`）。

#### 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介 |
| `uplver` | checkbox | - | 匿名发布 |

注意：综合页无 `pt_gen`、`technical_info` 字段。

#### 类型（`type`）— 单选择器

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | 其他 |
| 410 | 游戏 |
| 411 | 软件 |
| 412 | 学习 |
| 507 | 有声书 |
| 508 | 电子书 |

注意：包含有声书(507)、电子书(508)、学习(412)独特分类。值507/508超出常规NexusPHP 4xx范围。

#### 媒介（`medium_sel`）— 11个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray(原盘) |
| 2 | DVD(原盘) |
| 3 | REMUX |
| 5 | HDTV |
| 8 | CD |
| 9 | Track |
| 10 | WEB-DL |
| 11 | Other |
| 12 | UHD Blu-ray |
| 13 | UHD Blu-ray/DIY |
| 14 | Blu-ray/DIY |
| 15 | encode |

注意：Blu-ray 细分为原盘(1)和DIY(14)，UHD Blu-ray 分为原盘(12)和DIY(13)。

#### 视频编码（`codec_sel`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264(AVC) |
| 2 | VC-1 |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265(HEVC) |

#### 音频编码（`audiocodec_sel`）— 13个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 6 | AAC |
| 7 | Other |
| 18 | DD/AC3 |
| 19 | DTS-HD MA |
| 20 | TrueHD |
| 21 | LPCM |
| 22 | WAV |
| 23 | MP3 |
| 24 | M4A |

#### 分辨率（`standard_sel`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K |
| 10 | 8K |
| 11 | None |

#### 制作组（`team_sel`）— 7个

| 值 | 显示名称 |
|----|----------|
| 5 | Other |
| 19 | PTHome |
| 20 | PTHweb |
| 21 | PTH |
| 22 | PTHtv |
| 23 | PTHAudio |
| 24 | PTHeBook |
| 25 | PTHmusic |

注意：制作组以 PTHome 官方系列为主，含按类型细分的子组（PTHweb/PTHtv/PTHAudio/PTHeBook/PTHmusic）。外部组统一选 Other(5)。

#### 标签

综合发布页**无标签选择器**。标签通过种子浏览页的导航链接实现（音乐专辑、MV、演唱会、LIVE等），非发布时选择。

---

### 1.2 音乐发布页 (`upload_music.php`)

**完全独立的表单结构**，非标准 NexusPHP upload 表单。

#### 音乐专用字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `file` | file | 种子文件 |
| `name` | text | 标题 |
| `small_descr` | text | 副标题 |
| `artists` | text | 艺术家 |
| `album` | text | 专辑名 |
| `year` | text | 年份 |
| `cover_url` | text | 封面URL |
| `descr` | textarea | 简介 |
| `uplver` | checkbox | 匿名发布 |
| `group_id` | select | 制作组 |

#### 格式类型（`format_type`）— 4个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC分轨 |
| 2 | WAV整轨 |
| 3 | DSD |
| 400 | ISO |

#### 媒介类型（`medium_type`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | CD |
| 2 | DVD |
| 3 | 黑胶 |
| 4 | WEB |
| 5 | 未知媒介 |
| 6 | SACD |
| 7 | Blu-ray |

#### 发布类型（`publish_type`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 专辑 |
| 2 | EP |
| 3 | 单曲 |
| 4 | 精选 |
| 5 | 集锦 |
| 6 | 音乐会 |
| 7 | 重混 |
| 8 | 原声 |

#### 音乐制作组（`group_id`）— 3个

| 值 | 显示名称 |
|----|----------|
| 19 | PTHome |
| 25 | PTHmusic |
| 99 | Other |

#### 音乐标签（`tag[]`）— 28个

**地区标签**：
| 值 | 显示名称 |
|----|----------|
| 大陆 | 大陆 |
| 欧美 | 欧美 |
| 港台 | 港台 |
| 日韩 | 日韩 |
| 其它地区 | 其它地区 |

**风格标签**：
| 值 | 显示名称 |
|----|----------|
| 流行Pop | 流行Pop |
| 乡村Country | 乡村Country |
| 古典Classical | 古典Classical |
| 摇滚Rock | 摇滚Rock |
| 电子Electronic | 电子Electronic |
| 轻音乐 | 轻音乐 |
| 发烧Hifi | 发烧Hifi |
| 原声OST | 原声OST |
| 民间Folk | 民间Folk |
| 天籁Soul | 天籁Soul |
| 新世纪NewAge | 新世纪NewAge |
| 蓝调Blues | 蓝调Blues |
| 爵士Jazz | 爵士Jazz |
| 金属Metal | 金属Metal |
| 朋克Punk | 朋克Punk |
| 儿童Children's | 儿童Children's |
| 宗教Religion | 宗教Religion |
| 雷鬼Reggae | 雷鬼Reggae |
| 贝斯Drum&Bass | 贝斯Drum&Bass |
| 说唱Rap | 说唱Rap |
| 音乐剧musical | 音乐剧musical |
| 其他 | 其他 |

注意：音乐标签使用**字符串值**（非数字），是已分析站点中唯一的。

---

## 二、标题命名规范

来源：`rules.php` → 种子信息

标准 HD 站标题格式（电影/电视剧/音轨/游戏），与 OshenPT/SBPT 等站点一致。

---

## 三、发布规则

### 3.1 允许的资源

标准 HD 站允许列表（高清/标清/无损音轨/PC游戏/高清预告片等）。

### 3.2 ⚠️ 黑名单制作组

**以下制作组所有资源禁止发布**：
- **FGT** 小组
- **HDHome 任一官组**（与 HDHome 互斥）
- **Mp4Ba** 小组
- **RARBG** 小组

> ⚠️ **互斥规则**：PTHome 与 HDHome 互相禁止转载。转种时需检查源站，若资源来自 HDHome 或由 HDHome 官组发布，则**禁止转发到 PTHome**。

### 3.3 禁止的资源

- 总体积 < 100MB
- 标清 upscale 视频
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD
- RealVideo/RMVB/RM/FLV
- 单独样片
- < 5.1声道有损音频
- 无正确 cue 的多轨音频
- 游戏硬盘版/高压版
- RAR 压缩文件
- 重复资源
- 涉及禁忌或敏感内容
- **禁止将本站资源以非PT方式公开共享**（网盘、BT网站等）

### 3.4 Dupe 规则（标准 HD 站规则）

媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 按发布组优先级判定
- 断种45日+ 或发布18月+ → 可重发
- 不同区域/配音/字幕的原盘不视为重复
- 无损音轨只保留一个版本（分轨 FLAC 优先级最高）

---

## 四、站点适配器配置参考

```yaml
site:
  id: "pthome"
  name: "PTHome"
  alt_name: "铂金家"
  url: "https://pthome.net"
  framework: "nexusphp"
  
  mutual_exclusion:
    - site_id: "hdhome"
      reason: "PTHome与HDHome互相禁止转载"

  blacklist_groups:
    - "FGT"
    - "HDHome"  # HDHome任一官组
    - "Mp4Ba"
    - "RARBG"

  upload_pages:
    general:
      url: "upload.php"
      action: "takeupload.php"
      description: "综合资源发布"
    music:
      url: "upload_music.php"
      description: "音乐专用发布页"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动漫": 405
      "体育": 407
      "音乐": 408
      "其他": 409
      "游戏": 410
      "软件": 411
      "学习": 412
      "有声书": 507
      "电子书": 508

    medium_sel:
      "Blu-ray": 1
      "DVD": 2
      "Remux": 3
      "HDTV": 5
      "CD": 8
      "Track": 9
      "WEB-DL": 10
      "Other": 11
      "UHD": 12
      "UHD DIY": 13
      "DIY": 14
      "Encode": 15

    codec_sel:
      "H264": 1
      "VC-1": 2
      "MPEG-2": 4
      "Other": 5
      "H265": 6

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS": 3
      "AAC": 6
      "Other": 7
      "DD": 18
      "DTS-HDMA": 19
      "TrueHD": 20
      "LPCM": 21
      "WAV": 22
      "MP3": 23
      "M4A": 24

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "2160p": 5
      "8K": 10
      "None": 11

    team_sel:
      "Other": 5
      "PTHome": 19
      "PTH": 21
      "PTHweb": 20
      "PTHtv": 22
      "PTHAudio": 23
      "PTHeBook": 24
      "PTHmusic": 25

    music_format_type:
      "FLAC分轨": 1
      "WAV整轨": 2
      "DSD": 3
      "ISO": 400

    music_medium_type:
      "CD": 1
      "DVD": 2
      "黑胶": 3
      "WEB": 4
      "未知": 5
      "SACD": 6
      "Blu-ray": 7

    music_publish_type:
      "专辑": 1
      "EP": 2
      "单曲": 3
      "精选": 4
      "集锦": 5
      "音乐会": 6
      "重混": 7
      "原声": 8

    music_group:
      "PTHome": 19
      "PTHmusic": 25
      "Other": 99

  field_names:
    suffix: ""
    medium: "medium_sel"
    codec: "codec_sel"
    audiocodec: "audiocodec_sel"
    standard: "standard_sel"
    team: "team_sel"
    anonymous: "uplver"

  missing_fields:
    - "tags"
    - "technical_info"
    - "pt_gen"
    - "processing_sel"

  quirks:
    dual_upload_page: "综合资源和音乐使用不同的发布页面"
    no_field_suffix: "质量字段名无后缀，使用裸名"
    hdhome_mutual_exclusion: "与HDHome互斥，禁止互相转载"
    blacklist_groups: "禁止FGT/HDHome官组/Mp4Ba/RARBG"
    music_string_tags: "音乐页标签使用字符串值（非数字）"
    music_special_fields: "音乐页有独立字段：artists/album/year/format_type/medium_type/publish_type"
    pth_official_teams: "制作组以PTH系列为主（PTHweb/PTHtv/PTHAudio/PTHeBook/PTHmusic）"
    unique_categories: "有有声书(507)/电子书(508)/学习(412)独特分类"
    cloudflare: "使用Cloudflare防护"
```

---

## 五、发布流水线注意事项

### 5.1 双页面路由

转种时需根据资源类型选择正确的发布页面：
- 电影/剧集/综艺/纪录/动漫/体育/游戏/软件/学习/有声书/电子书 → `upload.php`
- 音乐 → `upload_music.php`（完全不同的表单结构）

### 5.2 互斥站点检查

发布前必须检查：
1. 资源是否来自 HDHome → 拒绝发布
2. 制作组是否为 HDHome 官组 → 拒绝发布
3. 制作组是否为 FGT/Mp4Ba/RARBG → 拒绝发布

### 5.3 音乐页特殊处理

音乐发布页需要填写额外字段：
- `artists`（艺术家）、`album`（专辑名）、`year`（年份）
- `format_type`（FLAC分轨/WAV整轨/DSD/ISO）
- `medium_type`（CD/DVD/黑胶/WEB/SACD/Blu-ray）
- `publish_type`（专辑/EP/单曲/精选/集锦/音乐会/重混/原声）
- `tag[]`（地区+风格标签，字符串值，可多选）

### 5.4 制作组映射

综合页：非 PTH 系列组统一选 Other(5)。
音乐页：非 PTHome/PTHmusic 统一选 Other(99)。

---

*分析时间：2026-04-16*
*数据来源：https://pthome.net/upload.php + upload_music.php + rules.php*
