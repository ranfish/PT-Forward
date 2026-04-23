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

来源：`rules.php` + 论坛 topicid=40 "PTHome站内发种格式标准" + topicid=8179 "标签的解释，及媒介分类的选择"

### 2.1 标题格式

**电影/演唱会**：`English_or_Pinyin_Name [Edit_Version] [Year] [Region] [Notes] Source Resolution VideoCodec [AudioCodec]-Group`
- 例：`Police Story REMASTERED 1985 GBR BluRay 1080p x264 DTS 2Audios-CMCT`
- 例：`Harry Potter 8-Film Collection 2160P UHD Blu-Ray HEVC DTS-HD MA 7.1 - user@PThome`

**电视剧/动漫**：`English_or_Pinyin_Name S**E** [Year] [Notes] Resolution Source VideoCodec [Audio]-Group`
- 例：`Always With You 2018 S01E20 1080p HDTV H264 AAC - PThome`
- 例：`HERO MASK S01 Complete 1080p NF WEB-DL DDP5.1 x264 - user@PThome`

**音轨**：`Artist - Album [Year] [Version] [Notes] AudioCodec[-Group]`

**音乐**：`Artist Year Album ResourceType Format`
- 例：`Jay Chou 2008 Capricorn Album ape`

**游戏**：`Game English Name [Version Type] [Resource Type] [Version Number] [Additional Info] [-Release Group]`

注意：主标题**仅用英文/拼音**，中文信息放副标题。

### 2.2 副标题格式

**电影**：`Chinese_Name (Edit_Version) | Valuable_Info | [Audio_Info] [Subtitle_Info] *Publisher_Notes*`
- 例：`阿凡达/化身/异次元战神 (加长版) | 导演: 詹姆斯·卡梅隆 | [国英双语] [中英字幕] | *精制特效字幕*`

**电视剧**：`Chinese_Name SeasonX EpisodeX | Publisher_Notes`

### 2.3 简介要求

1. 海报（全尺寸，上传到图床）
2. 影片信息（导演/演员/年份/评分/剧情概要）
3. 视频参数（MediaInfo/BDInfo，用 `[code]` 包裹）
4. 截图（≥3 张）
5. 外部链接（IMDb + 豆瓣）
6. 质量/来源信息如实选择

### 2.4 关键约束

- **禁止删除资源小组后缀**
- 禁止在标题中使用中文（电影/电视剧）
- 禁止在副标题中使用英文片名
- 至少保持与源站相同的信息量，不能缺斤少两

---

## 三、发布规则

### 3.1 允许的资源

标准 HD 站允许列表（高清/标清/无损音轨/PC游戏/高清预告片等）。

### 3.2 ⚠️ 黑名单制作组

**以下制作组所有资源禁止发布**：
- **FGT** — 全部禁止
- **HDHome 任一官组** — 全部禁止（与 HDHome 互斥）
- **Mp4Ba** — 全部禁止
- **RARBG** — 全部禁止

> ⚠️ **互斥规则**：PTHome 与 HDHome 互相禁止转载。转种时需检查源站，若资源来自 HDHome 或由 HDHome 官组发布，则**禁止转发到 PTHome**。

### 3.3 禁止的资源

除标准 HD 站禁止列表外，PTHome 额外禁止：
- **二压资源**（对已有 Encode 再次重编码，MiniSD/移动端除外）
- DivX/XviD (.AVI) 格式
- 不完整 BDMV 结构、拆分发布
- 硬水印视频
- 非规范转发（改名/改结构/增删文件）
- **禁止以非 PT 方式公开共享**（网盘、BT 网站等）
- 单种子上传速度 > **100MB/s** 自动封号

### 3.4 Dupe 规则

媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 按发布组优先级判定
- 断种 45 日+ 或发布 18 月+ → 可重发
- 不同区域/配音/字幕的原盘不视为重复
- 无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- **动漫特例**：HDTV 和 DVD 同优先级

### 3.5 账号保留规则

| 条件 | 规则 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存账号后不会被删除 |
| 封存账号 | 连续 **180** 天不登录删除 |
| 未封存账号 | 连续 **90** 天不登录删除 |
| 无流量账号 | 连续 **30** 天不登录删除 |

### 3.6 新用户警告

- 下载 > 100GB 且分享率 < 0.2 → **直接封号**

### 3.7 上传者资格

- 发布 < 5 个种子的用户必须通过**候选区**审核
- 通过 5 个合格发布后解锁直接发布
- 错误太多或拒绝修正 → 发布计数清零，回到候选区
- 游戏：仅上传员及以上可自由发布

### 3.8 H&R 规则

- **适用**：所有非 VIP 用户（发布 > 30 天的种子除外）
- 下载完成后生成 H&R 记录
- **7 天内做种 ≥ 48 小时 或 分享率 ≥ 1.0** → 消除 H&R
- **10 次 H&R 违规 = 自动封号**
- 消除 1 次 H&R：花费 **8,888 魔力值**

### 3.9 促销规则

**随机促销**（上传后自动触发）：

| 概率 | 类型 |
|------|------|
| 60% | 50% 下载 |
| 16% | 免费 |
| 1% | 2x 上传 |
| 5% | 50% 下载 & 2x 上传 |
| 3% | 30% 下载 |
| 3% | 免费 & 2x 上传 |

**自动促销**：
- 种子体积 > **100GB** → 发布时自动免费
- 所有种子 **108 天后** → 永久 50% 下载 & 2x 上传

**促销衰减链**：
```
Free & 2X →(1天)→ Free →(2天)→ Normal →(3天)→ 50% →(30天)→ 30% →(30天)→ 2X & 50% →(45天)→ 永久
```

### 3.10 盒子/SeedBox 规则（论坛 topicid=7545）

**定义**：专业供应商提供的设备或安装在独立服务器/VPS 上的远程下载/上传设备。

| 规则 | 说明 |
|------|------|
| 下载 | 100% 计量，无促销优惠（全黑种） |
| 上传（前 72 小时） | 按实际上传量计算，无 2x/2xFree，**上限种子体积 3 倍** |
| 上传（72 小时后） | 享受正常促销待遇 |
| 豁免 | VIP 用户、自己发布的种子、转载发布员/官组/保种组 |
| 共享 IP | **不再允许** |

### 3.11 做种/断种规则

- 撤种前必须保证他人完成下载，做种时间不足 **48 小时** → 警告/取消上传权限
- 初始做种阶段连续 **24 小时**断线 → 种子被删除
- 发布奖励：新种 **500 魔力**（< 2GB 仅 100 魔力）、新字幕 20 魔力、发布者双倍上传量

### 3.12 认领规则

| 项目 | 规则 |
|------|------|
| 条件 | 种子体积 > 1GB，发布 > 30 天 |
| 每种子上限 | 5 人 |
| 每用户上限 | 30 个 |
| 达标标准 | 300 小时/月 或 10 倍体积上传/月 |
| 奖励 | 普通 200 魔力/种子/月，官方 800 魔力/种子/月 |
| 放弃惩罚 | 400 魔力/种子 |

### 3.13 流量控制

| 用户等级 | 限制 | 超限后果 |
|----------|------|----------|
| 至 得力助手 | 40 个种子/小时 | 4 小时冷却 |
| 保种员+ | 200 个种子/小时 | 4 小时冷却 |
| < 1GB 种子 | 不受限制 | — |

### 3.14 标签定义（论坛 topicid=8179）

| 标签 | 定义 | 对转发工具的影响 |
|------|------|-----------------|
| 官方 | **仅限** PTHOME 官方组资源 | — |
| 原创 | 自购/自制/自编码/自下载 | — |
| 国语/粤语 | 国语/粤语配音 | — |
| 官字组 | **仅限** PTHOME 官方字幕组 | — |
| 中字 | 简体/繁体中文字幕 | — |
| Dolby Vision | 杜比视界（单层/双层），**不是** Dolby Atmos | — |
| HDR10 | 含 HDR10 标准 | — |
| HDR10+ | 三星 HDR10+ 标准（罕见） | — |
| **禁转** | 禁止转载到其他站点 | **必须跳过** |
| **限转** | 限时禁止转载，标签消失前不可转载 | **必须跳过** |
| DIY | **仅限**原盘 DIY 资源 | — |
| 首发 | **仅限**全球首发蓝光原盘 | — |
| 应求 | 应求区回应请求发布 | — |

### 3.15 游戏发布规则（论坛 topicid=8799，试行版）

**标题**：全英文 — `Game English Name [Version Type] [Resource Type] [Version Number] [Additional Info] [-Release Group]`
**副标题**：`Chinese Name & Aliases [Version Number] | Platform | Language | File Format [| Resource Type] [| Other Info]`
**质量字段**：统一选 "Other" 或 "None"
**标签**：游戏资源**不使用任何标签**
**禁止**：DRM 保护、纯网游、含病毒、政府禁发资源
**Dupe 规则**（游戏特例）：
- 稳定版替换 alpha/beta
- v1.0/v1.x/最新版可共存
- 无 v1.0 时（Early Access），最近 3 个版本共存
- 信任源资源替换个人自制，个人优秀资源可与信任源共存

### 3.16 禁止的行为

- 账号交易/买卖/交换 → 封号 + 连坐
- 多账号 → 封号
- 作弊 → 封号
- 不尊重上传者（删除后缀/违反禁转/修改源资源）→ 警告或封号

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
