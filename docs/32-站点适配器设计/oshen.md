# 奥申 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 奥申|
| 站点地址 | https://www.oshen.win |
| 站点框架 | NexusPHP |
| 主题 | BlueGene |
| 定位 | 通用分享站，标准 HD 站规则 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名） |
| `small_descr` | text | - | 副标题 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode，20行） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀，单模式（mode=4）。

#### 类型（`type`）— 必填，data-mode='4'

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 连续剧 |
| 403 | 综艺节目 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV(音乐视频) |
| 407 | 体育 |
| 408 | 高清音乐 |
| 409 | 音乐 |
| 410 | 游戏/Game |

注意：区分"高清音乐"(408)和"音乐"(409)两个分类。

#### 媒介（`medium_sel[4]`）— 9个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |

注意：有 MiniBD(4)，无 WEB-DL、UHD 独立选项。

#### 视频编码（`codec_sel[4]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 10 | H.265/HEVC |

注意：H.265/HEVC 值为10（非连续），保留 Xvid(3)。无 AV1。

#### 分辨率（`standard_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K/UHD |

#### 制作组（`team_sel[4]`）— 11个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | CMCT |
| 7 | SCSG |
| 8 | HDTIME |
| 9 | PTHOME |
| 10 | OshenPT |
| 11 | 52pt |

注意：制作组较多，包含 OshenPT(10) 官方组和 52pt(11)，以及 CMCT(6)、SCSG(7)、HDTIME(8)、PTHOME(9) 等常见 PT 站组。

#### 标签（`tags[4][]`）— 7个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官方 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

### 1.3 缺失字段

- `audiocodec_sel` — 无音频编码选择
- `processing_sel` — 无地区选择
- `technical_info` — 无 MediaInfo 输入框
- `pt_gen` — 无 PTGen 链接输入框
- `url` — 无 IMDb 链接输入框
- `uplver` — 无匿名发布选项

---

## 二、标题命名规范

来源：`rules.php` → 种子信息

### 2.1 标题格式

| 类型 | 格式 | 示例 |
|------|------|------|
| 电影 | `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS |
| 电视剧 | `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU |
| 音轨 | `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组]` | 恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC |
| 游戏 | `[中文名] 名称 [年份] [版本] [发布说明][-发布组]` | 红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED |

### 2.2 简介要求

- 电影/电视剧/动漫：必须包含海报/封面，尽可能包含截图、MediaInfo、演职员和剧情概要
- NFO 写入 NFO 文件而非粘贴到简介
- 体育节目：禁止泄漏比赛结果
- 音乐：必须包含专辑封面和曲目列表
- 游戏：必须包含封面，尽可能包含截图

---

## 三、发布规则

### 3.1 允许的资源

- 高清视频（Blu-ray/HD DVD 原盘、Remux、HDTV、720p+ 重编码）
- 标清视频（仅限高清媒介重编码 480p+、DVDR/DVDISO/DVDRip/CNDVDRip）
- 无损音轨（FLAC、Monkey's Audio 等）
- 5.1声道+ 音轨（DTS、DTSCD 等）
- PC游戏（必须原版光盘镜像）
- 7日内高清预告片
- 高清相关软件和文档（<100MB）

### 3.2 禁止的资源

- 总体积 < 100MB
- 标清 upscale 视频
- CAM、TC、TS、SCR、DVDSCR、R5、HalfCD 等低质量
- RealVideo/RMVB/RM/FLV
- 单独样片
- < 5.1声道有损音频（MP3、WMA）
- 无正确 cue 的多轨音频
- 游戏硬盘版/高压版/非官方镜像/第三方 mod
- RAR 压缩文件
- 重复资源
- 涉及禁忌或敏感内容
- 损坏文件、垃圾文件

### 3.3 Dupe 规则（标准 HD 站规则）

媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 同一视频高优先级使低优先级被判定为重复
- 动漫特例：HDTV 和 DVD 同优先级
- 按发布组优先级判定（参考论坛帖子）
- 断种45日+ 或发布18月+ → 可重发
- 不同区域/配音/字幕的原盘不视为重复
- 无损音轨只保留一个版本（分轨 FLAC 优先级最高）

### 3.4 资源打包规则

- 按套装售卖的电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7日内高清预告片
- 同一艺术家 MV（标清 MV 只允许 DVD 打包）
- 同一艺术家音乐（5张+或两年内专辑可单独发布）
- 分卷发售的动漫剧集/角色歌/广播剧
- 发布组打包资源

### 3.5 促销规则

随机促销（上传后自动触发）：
- 10% → 50%下载
- 5% → 免费
- 5% → 2x上传
- 3% → 50%下载 & 2x上传
- 1% → 免费 & 2x上传

固定促销：
- 文件总体积 > 20GB → 自动免费
- Blu-ray Disk/HD DVD 原盘 → 自动免费
- 电视剧每季第一集 → 自动免费
- 所有种子发布1个月后 → 永久2x上传
- 促销限时7天（2x上传无时限）

### 3.6 游戏发布限制

游戏类资源只有**上传员**及以上等级用户才能自由上传，其他用户必须先在候选区提交候选。

---

## 四、站点适配器配置参考

```yaml
site:
  id: "oshen"
  name: "OshenPT"
  url: "https://www.oshen.win"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动漫": 405
      "MV": 406
      "体育": 407
      "高清音乐": 408
      "音乐": 409
      "游戏": 410

    medium_sel:
      "Blu-ray": 1
      "HD DVD": 2
      "Remux": 3
      "MiniBD": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Track": 9

    codec_sel:
      "H264": 1
      "VC-1": 2
      "Xvid": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 10

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "2160p": 5

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "CMCT": 6
      "SCSG": 7
      "HDTIME": 8
      "PTHOME": 9
      "OshenPT": 10
      "52pt": 11

    tags:
      "禁转": 1
      "首发": 2
      "官方": 3
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"

  missing_fields:
    - "audiocodec_sel"
    - "processing_sel"
    - "technical_info"
    - "pt_gen"
    - "url"
    - "uplver"

  quirks:
    codec_h265_value: "H.265/HEVC值为10（非连续）"
    dual_music_category: "区分高清音乐(408)和音乐(409)"
    game_restricted: "游戏类资源仅上传员及以上可自由发布"
    no_webdl_medium: "无WEB-DL媒介选项"
```

---

## 五、发布流水线注意事项

### 5.1 制作组映射

OshenPT 有11个制作组，涵盖常见国内 PT 站组：
- HDS(1)/CHD(2)/MySiLU(3)/WiKi(4) — 标准四组
- CMCT(6)/SCSG(7)/HDTIME(8)/PTHOME(9) — 其他常见组
- OshenPT(10) — 官方组
- 52pt(11)
- 非以上制作组统一选 Other(5)

### 5.2 编码字段注意

H.265/HEVC 的值为 **10**（而非6），转种映射时需注意。

### 5.3 缺失字段处理

- 无 `url`：IMDb 链接写入简介
- 无 `pt_gen`：无 PTGen 辅助
- 无 `technical_info`：MediaInfo 写入简介
- 无 `uplver`：无法匿名发布

---

*分析时间：2026-04-16*
*数据来源：https://www.oshen.win/upload.php + rules.php*
