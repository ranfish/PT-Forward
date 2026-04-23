# GTK 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | PT GTK |
| 域名 | pt.gtkpw.xyz（旧域名 pt.gtk.pw） |
| 框架 | NexusPHP |
| Tracker URL | https://t.myaltbox.com/announce.php |
| Cloudflare | 否 |
| 候选制 | 是（游戏类必须候选，其他资源任何人可发） |
| MediaInfo | 独立 `technical_info` textarea |
| IMDb | 是（`url` 字段） |
| 豆瓣/PT-Gen | 是（`pt_gen` 字段，支持 imdb/douban/bangumi/indienova） |
| 匿名发布 | 是（`uplver`） |
| NFO | 是（`nfo` file 字段，Power User 以下不可查看） |
| 登录特性 | Challenge-Response 认证（SHA256 + HMAC-SHA256），支持两步验证 + 验证码 |
| 缺失字段 | 无 `audiocodec_sel`、无 `processing_sel` |

---

## 一、站点规则（2026-04-22 Playwright 实际采集 rules.php）

### 1.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 上传者必须保证上传速度与做种时间。如果在其他人完成前撤种或做种时间不足 24 小时，或故意低速上传，将被警告甚至取消上传权限
- 发布者获得双倍上传量
- 违规但有价值的资源可联系管理组破例

### 1.2 允许的资源

- 高清（HD）视频：蓝光原碟/HD DVD/remux/HDTV/重编码（至少 720p）/高清 DV
- 标清（SD）视频：高清媒介标清重编码（至少 480p）/DVDR/DVDISO/DVDRip/CNDVDRip
- 无损音轨（FLAC、Monkey's Audio 等）及 cue 表单
- 5.1 声道或以上的电影/音乐音轨（DTS、DTSCD 镜像等）、评论音轨
- PC 游戏（必须原版光盘镜像）
- 7 日内高清预告片
- 高清相关软件和文档

### 1.3 不允许的资源

- 总体积小于 100MB（例外：<100MB 的高清软件/文档、单曲专辑）
- 标清 upscale 视频
- CAM/TC/TS/SCR/DVDSCR/R5/R5.Line/HalfCD 等低质量标清
- RealVideo/RMVB/RM/FLV
- 单独样片（须与正片一起上传）
- 未达 5.1 声道的 MP3/WMA 有损音频（例外：2.0+ 国语/粤语音轨允许）
- 无正确 cue 的多轨音频
- 硬盘版/高压版/非官方游戏镜像/mod/小游戏合集/单独破解补丁
- RAR 等压缩文件
- 重复资源
- 禁忌/敏感内容
- 损坏文件/垃圾文件

### 1.4 重复（dupe）判定规则

- 来源媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 高清版本使标清版本被判定为重复
- **动漫特例**：HDTV 和 DVD 有相同优先级
- 相同媒介相同分辨率的重编码：按发布组优先级判定
- 总会保留一个 DVD5 大小（~4.38 GB）的最佳画质版本
- 基于无损截图对比，高质量版本使低质量版本被视为重复
- 不同配音/字幕的 blu-ray/HD DVD 原盘不被视为重复
- 每个无损音轨原则上只保留一个版本（分轨 FLAC 优先级最高）
- 新版本允许发布条件：无旧版错误/来源质量更好
- 旧版连续断种 45 日以上或已发布 18 个月以上 → 新版不受 dupe 约束
- 新版发布后旧版保留至断种

### 1.5 资源打包规则（试行）

允许打包：
- 按套装售卖的高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7 日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，不允许单曲 MV；高清分辨率需相同）
- 同一艺术家音乐（5 张以上专辑方可打包；两年内新专辑可单独发布）
- 分卷发售的动漫剧集/角色歌/广播剧
- 发布组打包发布的资源

打包要求：视频须相同媒介/相同分辨率/相同编码（预告片例外）；音频须相同编码。

### 1.6 标题格式（官方示例）

**电影**：
```
[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```
例：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS`

**电视剧**：
```
[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```
例：`越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU`

**音轨**：
```
[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]
```
例：`恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC`

**游戏**：
```
[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]
```
例：`红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED`

### 1.7 副标题 / 简介 / 种子信息要求

- **副标题**：不含广告或求种/续种请求
- **外部信息**：电影/电视剧必须包含 IMDb 链接（如存在）
- **简介**：
  - NFO 图写入 NFO 文件，不粘贴到简介
  - 电影/电视剧/动漫：海报/封面 + 截图 + 文件详情 + 演职员/剧情概要
  - 体育节目：不得泄露比赛结果
  - 音乐：专辑封面 + 曲目列表
  - PC 游戏：海报/封面 + 截图
- **杂项**：正确选择类型和质量信息
- 管理员可编辑种子信息，上传者可修正错误但不得改动管理员修改
- 资源原始信息合规时尽量沿用

### 1.8 促销规则

- 随机促销：10% 概率 50%下载，5% 免费，5% 2x上传，3% 50%&2x，1% 免费&2x
- 文件总体积 >20GB → 自动免费
- Blu-ray/HD DVD 原盘 → 自动免费
- 电视剧每季第一集 → 自动免费
- 促销限时 7 天（2x上传无时限）
- 发布 1 个月后自动永久 2x上传

---

## 二、发布页面表单字段分析（2026-04-22 Playwright 实际采集验证）

> 用户 ranfish 已登录，页面标题 `PT GTK :: 发布 BT|电影|... - Powered by NexusPHP`，表单共 37 个元素。以下数据已与实际页面逐一验证一致。

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 标题（若不填将使用种子文件名） |
| `small_descr` | text | - | 副标题（显示在种子标题下方） |
| `url` | text | - | IMDb 链接（带 PT-Gen "获取简介"按钮） |
| `pt_gen` | text | - | PT-Gen 链接（带 "获取简介"按钮） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode 编辑器，20行） |
| `technical_info` | textarea | - | MediaInfo/BDInfo（8行） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

质量字段名带 `[4]` 后缀（如 `medium_sel[4]`），`data-mode='4'` 表示当前分类模式。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 402 | TV Series(剧集) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录片) |
| 405 | Animations(动画) |
| 406 | Music Videos(MV) |
| 407 | Sports(运动题材) |
| 408 | HQ Audio(高清音频) |
| 409 | Misc(其他) |
| 410 | Book(图书) |
| 411 | Music Album(音乐专辑) |
| 412 | Education(资料) |

#### 媒介（`medium_sel[4]`）

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
| 10 | UHD |
| 11 | WEB-DL |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265/HEVC |
| 7 | AV1 |
| 8 | VP9 |

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p/4K |
| 6 | 4320p/8K |

#### 制作组（`team_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | CMCT |
| 7 | MARK |
| 8 | MTeam |
| 9 | FRDS |
| 10 | PTHome |
| 11 | beAst |

#### 标签（`tags[4][]`）— 多选 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

### 1.3 缺失字段

与标准 NexusPHP 相比，GTK 发布页**缺少**以下常见字段：

- `audiocodec_sel` — 无音频编码选择
- `processing_sel` — 无地区/来源选择

---

## 二、与其他站点对比

### 2.1 与 13City 对比

| 维度 | GTK | 13City |
|------|-----|--------|
| 分类ID | 401-412（12类） | 401-413（8类，无纪录片404） |
| 媒介值 | UHD=10, WEB-DL=11 | WEB-DL=10, BluRay=11, WEBRip=12, Other=13 |
| 编码值 | H.265/HEVC=6, AV1=7, VP9=8 | AVC/H.264/x264=1, HEVC/H.265/x265=2 |
| 分辨率值 | 2160p/4K=5, 4320p/8K=6 | 1080p=1, 1080i=2, 720p=3, SD=4 |
| 制作组 | HDS/CHD/MySiLU/WiKi/CMCT/MARK/MTeam/FRDS/PTHome/beAst | — |
| 标签 | 禁转/首发/DIY/国语/中字/HDR | — |
| 音频编码 | 无 | 有 |

### 2.2 字段映射注意事项

- GTK 的分类ID体系与多数 NexusPHP 站点一致（401-412 范围）
- 媒介值分配不标准：UHD=10, WEB-DL=11（多数站点 UHD 和 WEB-DL 的值不同）
- 编码字段将 H.264 和 x264 合并为 "H.264"（值1），没有区分原盘和压制编码
- 制作组列表偏向国内老牌组（HDS, CHD, MySiLU, WiKi），转种时常需选 "Other"(5)
- 质量字段名含模式后缀 `[4]`，需要动态拼接字段名

---

## 三、站点适配器配置参考

```yaml
site:
  id: "gtk"
  name: "PT GTK"
  url: "https://pt.gtkpw.xyz"
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
      "高清音频": 408
      "其他": 409
      "书籍": 410
      "音乐": 411
      "学习": 412

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
      "UHD": 10
      "WEB-DL": 11

    codec_sel:
      "H264": 1
      "VC-1": 2
      "Xvid": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 6
      "AV1": 7
      "VP9": 8

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "4K": 5
      "8K": 6

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "CMCT": 6
      "MARK": 7
      "MTeam": 8
      "FRDS": 9
      "PTHome": 10
      "beAst": 11

    tags:
      "禁转": 1
      "首发": 2
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
    anonymous: "uplver"

  missing_fields:
    - "audiocodec_sel"
    - "processing_sel"
```

---

## 四、发布流水线注意事项

### 4.1 字段名动态拼接

GTK 的质量字段带模式后缀 `[4]`（对应 type 的 `data-mode='4'`）。发布时需拼接：

```
medium_sel[4], codec_sel[4], standard_sel[4], team_sel[4], tags[4][]
```

如果 GTK 未来新增分类模式（如音乐分类用 `data-mode='5'`），字段名会变为 `medium_sel[5]` 等。适配器需根据 `type` 字段的 `data-mode` 属性动态确定后缀。

### 4.2 制作组映射策略

GTK 的制作组列表偏向国内老牌压制组。转种时：
- 源站制作组在列表中 → 直接映射
- 源站制作组不在列表中 → 使用 "Other"(5)
- 制作组字段无法自动判断时默认选 "Other"

### 4.3 缺失字段处理

GTK 无 `audiocodec_sel` 和 `processing_sel`：
- 适配器在构建表单时应跳过这两个字段
- 不应因这两个字段缺失而报错

### 4.4 PT-Gen 集成

GTK 发布页内置了 PT-Gen 获取按钮（`btn-get-pt-gen`），支持从 IMDb/Douban 链接自动填充简介。适配器可直接利用此功能或自行调用 PT-Gen API。

---

*分析时间：2026-04-16*
*最后更新：2026-04-22*
*数据来源：https://pt.gtkpw.xyz/upload.php 发布页面 HTML 分析*

