# PT地带 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | PT地带|
| 站点地址 | https://ptzone.xyz |
| 站点框架 | NexusPHP |
| 特殊规则 | 标准NexusPHP站点，Cloudflare防护 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 402 | TV Series(电视剧) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录片) |
| 405 | Animations(动漫) |
| 406 | Music(音乐) |
| 407 | Sports(体育) |
| 408 | Others(其它) |
| 409 | Others(其它) |
| 410 | Software(软件) |
| 411 | Games(游戏) |

注意：408 和 409 都显示"其它"，疑似重复分类。

#### 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | UHD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | MPEG-2 |
| 4 | MPEG-4 |
| 5 | Other |
| 6 | H.265 |

注意：编码列表较简单，无 AV1、VP9，不区分原盘/压制。

#### 音频编码（`audiocodec_sel[4]`）— 16个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | AC3 |
| 9 | DTS |
| 10 | DTS-HD MA |
| 11 | DD/AC3 |
| 12 | DDP/EAC3 |
| 13 | DTS-HD |
| 14 | TrueHD |
| 15 | WAV |

注意：值3和值9都显示"DTS"，值8(AC3)和值11(DD/AC3)功能重叠，疑似站点配置冗余。

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 8K |
| 6 | 4K |

注意：8K(5)排在4K(6)前面，值顺序不常规。

#### 制作组（`team_sel[4]`）— 仅6个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | PTZWeb |

#### 标签（`tags[4][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 分集 |
| 9 | 完结 |

### 1.3 缺失字段

- `processing_sel` — 无地区选择

---

## 二、站点适配器配置参考

```yaml
site:
  id: "ptzone"
  name: "PTZone"
  url: "https://ptzone.xyz"
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
      "音乐": 406
      "体育": 407
      "其他": 408
      "软件": 410
      "游戏": 411

    medium_sel:
      "Blu-ray": 1
      "HD DVD": 2
      "Remux": 3
      "WEB-DL": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Track": 9
      "UHD": 10

    codec_sel:
      "H264": 1
      "VC-1": 2
      "MPEG-2": 3
      "MPEG-4": 4
      "Other": 5
      "H265": 6

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS": 3
      "MP3": 4
      "OGG": 5
      "AAC": 6
      "Other": 7
      "AC3": 8
      "DTS-HDMA": 10
      "DD": 11
      "DDP": 12
      "DTS-HD": 13
      "TrueHD": 14
      "WAV": 15

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "8K": 5
      "4K": 6

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "PTZWeb": 6

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "分集": 8
      "完结": 9

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    audiocodec: "audiocodec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  missing_fields:
    - "processing_sel"

  quirks:
    duplicate_dts: "值3和值9都显示DTS，建议使用值3"
    duplicate_others: "type值408和409都显示其它，建议使用408"
    resolution_order: "8K=5在4K=6前面"
```

---

## 三、站点规则（rules.php）

### 3.1 账号保留

| 条件 | 规则 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存账号后不会被删除 |
| 封存账号 | 连续 400 天不登录删除 |
| 未封存账号 | 连续 150 天不登录删除 |
| 无流量账号 | 连续 100 天不登录删除 |

### 3.2 种子促销规则

**随机促销**（种子上传后自动随机设定）：

| 概率 | 类型 |
|------|------|
| 10% | 50%下载 |
| 5% | 免费 |
| 5% | 2x上传 |
| 3% | 50%下载 & 2x上传 |
| 1% | 免费 & 2x上传 |

**自动免费条件**：
- 文件总体积 > 20GB → 免费
- Blu-ray Disk / HD DVD 原盘 → 免费
- 电视剧每季第一集 → 免费

**促销时限**：
- 除"2x上传"外，其余类型限时 7 天（自种子发布起）
- "2x上传"无时限
- 所有种子发布 1 个月后自动永久成为 2x上传

### 3.3 上传者资格

- 任何人都能发布资源
- 游戏类资源：仅上传员及以上等级，或管理组指定用户可自由上传；其他用户须先在候选区提交候选

### 3.4 允许的资源

- 高清视频：蓝光/HD DVD 原盘、Remux、HDTV、高清重编码（≥720p）、高清 DV
- 标清视频：仅限来源于高清媒介的标清重编码（≥480p）、DVDR/DVDISO、DVDRip/CNDVDRip
- 无损音轨（FLAC/APE 等）及 cue 表单
- 5.1 声道或以上的电影/音乐音轨（DTS/DTSCD 镜像等）、评论音轨
- PC 游戏（必须为原版光盘镜像）
- 7 日内高清预告片
- 高清相关软件和文档

### 3.5 不允许的资源

- 总体积 < 100MB（例外：高清软件/文档、单曲专辑）
- 标清 upscale 视频
- CAM/TC/TS/SCR/DVDSCR/R5/R5.Line/HalfCD 等低质量标清
- RealVideo/RMVB/RM/FLV
- 单独样片（应与正片一起上传）
- < 5.1 声道的有损音频（MP3/WMA 等）
- 无正确 cue 的多轨音频
- 硬盘版/高压版游戏、非官方镜像、第三方 mod、小游戏合集、单独破解/补丁
- RAR 等压缩文件
- 重复资源（dupe）
- 色情/敏感政治内容
- 损坏文件、垃圾文件

### 3.6 重复（Dupe）判定

1. **媒介优先级**：Blu-ray/HD DVD > HDTV > DVD > TV
2. 高清版本使标清版本成为 dupe
3. 动漫类特例：HDTV 与 DVD 同优先级
4. 同媒介同分辨率重编码：按发布组优先级判定（参考论坛帖子 "Scene & Internal, from Group to Quality-Degree"）
5. 不同区域/不同配音和字幕的 Blu-ray/HD DVD 原盘不算 dupe
6. 无损音轨原则上只保留一个版本，分轨 FLAC 优先级最高
7. 旧版连续断种 45 天或发布 18 个月以上，新版不受 dupe 约束

### 3.7 资源打包规则（试行）

允许打包：
- 按套装售卖的高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7 日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，高清需同分辨率）
- 同一艺术家音乐（≥5 张专辑方可打包，两年内新专辑可单独发布）
- 分卷发售的动漫剧集/角色歌/广播剧
- 发布组打包资源

打包视频要求：相同媒介、相同分辨率、相同编码（预告片例外）。打包音频要求：相同编码格式。

### 3.8 标题格式规范

| 类型 | 格式 | 示例 |
|------|------|------|
| 电影 | `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS |
| 电视剧 | `[中文名] 名称 [年份] S\*\*E\*\* [发布说明] 分辨率 来源 [音频/]视频编码-发布组` | 越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU |
| 音轨 | `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组]` | 恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC |
| 游戏 | `[中文名] 名称 [年份] [版本] [发布说明][-发布组]` | 红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED |

### 3.9 认领规则

| 项目 | 规则 |
|------|------|
| 可认领时间 | 种子发布 30 天后 |
| 每种子最多认领人数 | 10 人 |
| 每用户最多认领数 | 1000 个 |
| 不达标惩罚 | 删除种子 + 扣 600 魔力（非认领首月） |
| 主动放弃惩罚 | 扣 400 魔力 |
| 达标魔力奖励 | 正常魔力值的 1 倍 |
| 达标标准 | 每月做种 ≥ 300 小时，或上传量 ≥ 体积 2 倍 |

### 3.10 发布者奖励

- 发布者对自己发布的种子获得**双倍上传量**

---

## 四、发布流水线注意事项

### 4.1 音频编码重复

PTZone 的 `audiocodec_sel` 中 DTS 出现两次（值3和值9），AC3 也出现两次（值8 "AC3" 和值11 "DD/AC3"）。建议统一使用：
- DTS → 值3
- AC3/DD → 值11（DD/AC3）
- DTS-HD MA → 值10

### 4.2 制作组映射

PTZone 仅有6个制作组，转种时非 HDS/CHD/MySiLU/WiKi/PTZWeb 的制作组统一选 Other(5)。

### 4.3 Cloudflare 防护

PTZone 使用 Cloudflare（`cf_clearance` cookie），适配器的 HTTP 客户端需能处理 Cloudflare 质询。

### 4.4 标题生成注意事项

- 标题格式遵循 rules.php §3.8 的模板（电影/电视剧/音轨/游戏各有不同格式）
- 不存在 `processing_sel`（无地区选择）
- 视频编码仅 H.264/H.265/VC-1/MPEG-2/MPEG-4/Other，无 AV1/VP9
- 分辨率仅有 1080p/1080i/720p/SD/8K/4K（无 576p/480p 独立选项，SD 统一）

### 4.5 Dupe 检测注意事项

- 转发前需检查目标资源是否已存在（同媒介+同分辨率+更高或同优先级发布组）
- 动漫类资源 HDTV 与 DVD 同优先级，不受常规媒介优先级约束
- 无损音轨只保留一个版本（分轨 FLAC 优先）

---

*分析时间：2026-04-16*
*最后更新：2026-04-22*
*数据来源：https://ptzone.xyz/rules.php + https://ptzone.xyz/upload.php 发布页面 HTML 分析*

### 附：发布适配记录（v0.0.1304 §59.448）

- **form_config**：migration131 分发（七槽 64 项——type 10/medium 10/
  codec 6/audiocodec 14/standard 6/team 6/tags 8）。实时 HTML
  （45.1KB 直连）vs 4 月资产**大漂移（末日式局部）**：
  - **type 11→12**：408/409 同 label "Others(其它）" 双 value（§59.444
    龙PT 同形态——配置对齐 draft 单行口径删 409 留 HTML 首值 408）+
    410 Ebook→Software+411 Edu→Games（资产后四项全漂移）。
  - **medium 换血**：HD DVD(2) 新增+**UHD(10) 顶替资产 Other(10)**
    （同 value 不同 label——value 漂移站方直接拒收；实时 HTML 唯一
    真源）+Other 消失（miss 落空预览人工）。
  - **audiocodec 15 大换血**：LPCM/ALAC/M4A/AIFF/OPUS 消失+**DTS
    3/9 同 label 克隆式重复（配置留首值 3）**+AC3(8)/DD/AC3(11) 双
    value 近语义+DDP/EAC3(12)/DTS-HD(13)/TrueHD(14)/WAV(15) 新增。
  - **tags 资产 None→实时 8 项**（青蛙/学校/海的宝同款缺口补全）。
  - codec/standard/team 三槽零漂移。
- type 10（配置侧删 409）：404 双键+406 Music=[music,lossless_music]
  双键（克隆先例）+408 Others label-only（杂项无键）+410/411 配键。
- medium 10：UHD(10)/Blu-ray(1)/HD DVD(2)=[hddvd] ext 活键+Remux(3)
  双键归并+Encode+WEB-DL(4) 双键+HDTV+**DVDR(6)=[dvd,dvdr] 双键宽
  语义（阿玲先例——无 DVD 项用 DVDR 项收 dvd 键）**+CD+Track(9)=
  [track] ext 活键；无 Other 项。
- codec 6：H.264/H.265 配键+VC-1/MPEG-2/MPEG-4=[mpeg4] ext+Other
  ——无 X26x 分立零联动+无 AV1/VP9。
- **audiocodec 14（配置侧删 DTS(9) 克隆）**：**AC3(8) label-only
  （audio.ac3/dd 键均归 11——文档 §4.1 站方语义定案 DTS→3+AC3/DD→
  11 DD/AC3 全语义，一键多值防线）**+DDP/EAC3(12)=[ddp] 单键（eac3
  无键——Nova/萝莉先例）+**DTS-HD(13)=[dts_hd_hr] 家族剩余键**（MA
  已分立归 10，无 MA 的 DTS-HD 即 HR——星湾泛剩余先例）+TrueHD+
  WAV+FLAC/APE/DTS(3)/MP3/OGG(ext)/AAC/Other。
- standard 6（值 5=8K/6=4K 乱序）：1080p/i 分立+720p+SD=[sd] 统一
  承接（无 576p/480p 独立项）+8K/4K；无 Other。
- team 6：PTZWeb 官组 label-only+HDS/CHD/MySiLU/WiKi 外站组
  label-only+Other(5)=[team.other] 兜底。
- tags 8：**禁转(1)/首发(2) 绝不配**（财神雷区）+完结=[complete]+
  分集=[ongoing]+DIY+国语+中字+**HDR(7)=[hdr10,hdr10_plus,hdr_vivid,
  hlg] 财神四键泛家族**（站方无细分四键全承接）。
- 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
  standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
  technical_info+url（IMDb data-pt-gen 真字段）+pt_gen+uplver+
  anonymous:true；**nfo 不配**（13城先例）；无 douban 字段（豆瓣走
  pt_gen）；url×2 第二个是编辑器按钮（浦园/青蛙同款）。
- **验证**：裸对全槽一致（同 label 去重口径）+formdiff 0 条（一次
  归零）+DryRun 三形态（① WEB-DL 剧集→402+4+1+1+**tags=[国语,中字,
  HDR] hdr10_plus 细分归泛 HDR 承接** ✓/② UHD 原盘→**medium=10 UHD
  乱序正确**+audio=10 DTS-HD MA+standard=6 4K 乱序正确 ✓/③ MI 压制
  →medium=7 Encode+**codec=6 H.265 折叠命中（无 X26x 分立站方语义）**
  ✓）+48 包。
- 站规人工把关：上传者资格（§3.3）+不允许的资源（§3.5）+Dupe 判定
  （§3.6 同媒介+同分辨率+更高/同优先级组——动漫 HDTV/DVD 同级例外）
  +资源打包（§3.7）+标题格式规范（§3.8 分模板）+无损音轨只保留一
  版本（§4.5 分轨 FLAC 优先）+CF 防护（§4.3 cf_clearance）。
- 遗留：首颗真实发布。
