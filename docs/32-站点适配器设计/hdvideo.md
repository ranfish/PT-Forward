# HDVideo 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | HDVideo|
| 站点地址 | https://hdvideo.top |
| 站点框架 | NexusPHP |
| 特殊规则 | HDR 细分标签、音乐/演唱会专属标签、仅3个制作组 |
| Cloudflare | 否 |
| 候选制 | 是（Peasant 及以上可候选，User 及以上可直接发布） |
| MediaInfo | 是（放入简介 descr，用引用标签包裹） |
| BDInfo | 是（原盘必须用 BDInfo） |
| IMDb | 是（url 字段） |
| 豆瓣 | 是（pt_gen 字段） |
| NFO | 是（独立 nfo 文件上传字段） |
| 匿名发布 | 是（uplver） |
| 官组后缀 | HDV / HDVWEB / HDVMV |

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
| `hr[4]` | input | - | HR（不明字段） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | 音轨 |
| 407 | 体育 |
| 408 | 音乐MV |

#### 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 10 | UHD Blu-ray |
| 11 | FHD Blu-ray |
| 12 | Remux |
| 13 | Encode |
| 14 | WEB-DL |
| 16 | UHDTV/HDTV |
| 18 | DVD |
| 19 | Other |
| 20 | CD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 6 | HEVC/H.265/x265 |
| 7 | AVC/H.264/x264 |
| 8 | VC-1 |
| 9 | MPEG-2 |
| 10 | VP8/9 |
| 11 | Other |
| 12 | AV1/S |
| 14 | ProRes |

#### 音频编码（`audiocodec_sel[4]`）— 21个选项

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS_HD\|MA |
| 5 | Opus |
| 7 | Other |
| 8 | LPCM/PCM |
| 9 | DD/AC3 |
| 10 | DDP/EAC3 |
| 11 | TrueHD |
| 12 | WAV |
| 14 | TrueHD_Atmos |
| 15 | DTS_HD\|HR |
| 16 | DTS_X |
| 17 | DTS |
| 18 | MP2/3 |
| 19 | TAA |
| 20 | OGG |
| 21 | AAC |
| 22 | MPEG |
| 23 | DDP Atmos/EAC3 |

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 5 | 4320p/8K |
| 6 | 2160p/4K |
| 8 | 1080p |
| 9 | 1080i |
| 10 | 720p |
| 11 | Other |

#### 制作组（`team_sel[4]`）— 4个

| 值 | 显示名称 |
|----|----------|
| 1 | HDVWEB |
| 2 | HDVMV |
| 4 | Other |
| 5 | HDV |

#### 标签（`tags[4][]`）— 25个多选 checkbox

| 值 | 显示名称 | 类别 |
|----|----------|------|
| 1 | 禁转 | 转载控制 |
| 3 | 官方 | 身份标识 |
| 5 | 国语 | 音轨 |
| 6 | 中字 | 字幕 |
| 8 | 源码 | 技术 |
| 9 | 限转 | 转载控制 |
| 10 | 粤语 | 音轨 |
| 11 | HDR10 | HDR |
| 12 | Dolby Vision | HDR |
| 13 | HLG | HDR |
| 15 | 零魔 | 系统 |
| 16 | 金种 | 系统 |
| 17 | 完结 | 状态 |
| 18 | 原创 | 身份标识 |
| 19 | 特效 | 字幕 |
| 20 | HDR10+ | HDR |
| 21 | 应求 | 状态 |
| 22 | DIY | 类型 |
| 23 | MV | 音乐 |
| 24 | LIVE现场 | 音乐 |
| 25 | 音乐专辑 | 音乐 |
| 26 | 演唱会 | 音乐 |
| 27 | HDR Vivid | HDR |
| 28 | 连载 | 状态 |
| 29 | 首发 | 身份标识 |

### 1.3 缺失字段

- `technical_info` — 无 MediaInfo/BDInfo 专用字段
- `processing_sel` — 无地区选择
- 分辨率缺少 SD 选项

---

## 二、与其他站点对比

### 2.1 HDVideo 特色

| 维度 | HDVideo 特色 |
|------|-------------|
| 制作组分隔符 | 电影使用 `--`（双横线）分隔制作组，剧集/MV/音乐使用 `-`（单横线） |
| 帧率位置 | 可选字段，位于分辨率之后、来源之前 |
| 地区/平台 | 可选字段，如 `HKG`（香港）、`NF`（Netflix） |
| 色深 | 可选字段，如 `10bit` |
| 色彩空间 | 可选字段 |
| Complete 标记 | 剧集完结时使用 `Complete`（位于季数之后） |
| 3D 格式 | 如 `3D HSBS`（位于年份之后） |

### 2.2 关键差异

1. **媒介分 UHD/FHD Blu-ray** — UHD Blu-ray(10) vs FHD Blu-ray(11)，而非按 DIY/Remux 细分
2. **制作组极少** — 仅 HDVWEB、HDVMV、Other，转种时几乎都选 Other(4)
3. **HDR 标签细分** — 5种 HDR 标签：HDR10、HDR10+、Dolby Vision、HLG、HDR Vivid
4. **音乐专属标签** — MV、LIVE现场、音乐专辑、演唱会
5. **无 MediaInfo 专用字段** — 只能放在简介 `descr` 中
6. **分辨率值非标准** — 1080p=8, 720p=10（多数站点为 1-4）

---

## 三、站点适配器配置参考

```yaml
site:
  id: "hdvideo"
  name: "HDVideo"
  url: "https://hdvideo.top"
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
      "音轨": 406
      "体育": 407
      "MV": 408

    medium_sel:
      "UHD Blu-ray": 10
      "FHD Blu-ray": 11
      "Remux": 12
      "Encode": 13
      "WEB-DL": 14
      "HDTV": 16
      "DVD": 18
      "Other": 19
      "CD": 20

    codec_sel:
      "H265": 6
      "H264": 7
      "VC-1": 8
      "MPEG-2": 9
      "VP9": 10
      "Other": 11
      "AV1": 12
      "ProRes": 14

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS-HDMA": 3
      "Opus": 5
      "Other": 7
      "LPCM": 8
      "AC3": 9
      "DDP": 10
      "TrueHD": 11
      "WAV": 12
      "TrueHD Atmos": 14
      "DTS-HDHR": 15
      "DTS-X": 16
      "DTS": 17
      "MP3": 18
      "AAC": 21
      "DDP Atmos": 23

    standard_sel:
      "8K": 5
      "4K": 6
      "1080p": 8
      "1080i": 9
      "720p": 10
      "Other": 11

    team_sel:
      "HDVWEB": 1
      "HDVMV": 2
      "Other": 4
      "HDV": 5

    tags:
      "禁转": 1
      "官方": 3
      "国语": 5
      "中字": 6
      "源码": 8
      "限转": 9
      "粤语": 10
      "HDR10": 11
      "Dolby Vision": 12
      "HLG": 13
      "零魔": 15
      "金种": 16
      "完结": 17
      "原创": 18
      "特效": 19
      "HDR10+": 20
      "应求": 21
      "DIY": 22
      "MV": 23
      "LIVE现场": 24
      "音乐专辑": 25
      "演唱会": 26
      "HDR Vivid": 27
      "连载": 28
      "首发": 29

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
    - "technical_info"
```

---

## 四、发布流水线注意事项

### 4.1 制作组映射

HDVideo 有 4 个制作组选项（HDVWEB、HDVMV、HDV、Other）。转种时：
- 源站是 HDVideo 官组 WEB 资源 → 映射到 HDVWEB(1)
- 源站是 HDVideo 官组 MV 资源 → 映射到 HDVMV(2)
- 源站是 HDVideo 官组其他资源 → 映射到 HDV(5)
- 其他所有情况 → 使用 Other(4)

### 4.2 媒介映射逻辑

HDVideo 区分 UHD Blu-ray(10) 和 FHD Blu-ray(11)：
- 标准媒介为 UHD + 原盘 → 10 (UHD Blu-ray)
- 标准媒介为 Blu-ray + 1080p → 11 (FHD Blu-ray)
- 需要根据分辨率选择正确的 Blu-ray 子类型

### 4.3 HDR 标签选择

HDVideo 的 HDR 标签细分 5 种，需要从 MediaInfo 或标题中精确识别：
- HDR10 → 标签 11
- HDR10+ → 标签 20
- Dolby Vision → 标签 12
- HLG → 标签 13
- HDR Vivid → 标签 27

### 4.4 MediaInfo 处理

HDVideo 无专用 `technical_info` 字段，MediaInfo 需嵌入 `descr` 简介中（通常用 `[code]` 标签包裹）。

---

*分析时间：2026-04-16*
*数据来源：https://hdvideo.top/upload.php 发布页面 HTML 分析*
*文档更新：2026-04-22 — 补充 rules.php 完整规则 + 论坛发种规范 + 盒子规则 + 发布页 Playwright 验证*

---

## 五、发布页字段验证（2026-04-22 Playwright 实际采集）

> 用户 ranfish 已登录，页面标题 `HDVideo :: 发布 - Powered by NexusPHP`。

与现有文档逐一对比，采集结果完全一致：

| 对比项 | 原文档 | 采集结果 | 一致 |
|--------|--------|---------|------|
| 分类 type | 8 个 (401-408) | 8 个 (401-408) | ✅ |
| 媒介 medium_sel[4] | 9 个 | 9 个 | ✅ |
| 视频编码 codec_sel[4] | 8 个 | 8 个 | ✅ |
| 音频编码 audiocodec_sel[4] | 21 个 | 21 个 | ✅ |
| 分辨率 standard_sel[4] | 6 个 | 6 个 | ✅ |
| 制作组 team_sel[4] | 3 个 (HDVWEB/HDVMV/Other) | 4 个（新增 HDV=5） | ⚠️ |
| 标签 tags[4][] | 25 个 | 25 个 + uplver | ✅ |

**发现差异**：制作组新增 `HDV`（id=5），原有文档未记录。制作组完整列表更新为：

| 值 | 显示名称 |
|----|----------|
| 1 | HDVWEB |
| 2 | HDVMV |
| 4 | Other |
| 5 | HDV |

---

## 六、站点规则（rules.php 完整采集 2026-04-22）

### 6.1 总则

- 禁止发送垃圾信息
- 禁止注册马甲账号
- 禁止使用站点名称作为用户名
- 禁止将本站种子上传到其他 Tracker
- 禁止将 PT 资源以非 PT 方式公开分享（网盘/论坛/BT 网站等）
- 禁止发布违规删改的资源（篡改命名、删改文件、变更种子目录）
- 一切作弊账号封禁

### 6.2 账号保留规则

| 等级 | 保留条件 |
|------|---------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存后不会被删除 |
| 封存账号 | 连续 400 天不登录删除 |
| 未封存账号 | 连续 150 天不登录删除 |
| 新注册 | 七天无流量自动封禁 |

### 6.3 下载规则

- 分享率过低会禁止下载/警告/封禁
- **种子促销**：90% 概率免费，10% 概率 2x 免费
- 体积 > 80GB 自动免费
- H&R：目前未明确开启，但规则提及 H&R 考核
- **允许的客户端**：qBittorrent, Transmission, Deluge, Azureus, Rufus, MLDonkey, RTorrent, SymTorrent, uTorrent
- **禁用客户端**：Transmission 3.x（需 4.0.5+）

### 6.4 上传规则

#### 上传资格

- 任何人都能发布资源
- Peasant 及以上需先在候选区提交候选
- User 及以上可直接发布

#### 允许的资源

| 类型 | 说明 |
|------|------|
| 高清视频 | UHD/FHD Blu-ray 原盘/Remux/Encode, UHDTV/HDTV/WEB-DL |
| 标清视频 | 仅限来源于高清媒介的 720p+ Encode；无高清片源时允许 DVDRip/DVDISO |
| PC 游戏 | 必须为原版光盘镜像 |
| 特许资源 | 发布前咨询管理组 |

#### 禁止的资源

| 类型 | 说明 |
|------|------|
| 体积 < 100MB | 除高清软件/文档、单曲专辑外 |
| Upscale 视频 | AI 放大及无意义堆高码率 |
| 公网组视频 | 使用 HDR10plus_tool/DoVi_tool 制作的视频；公网小组一切视频 |
| 劣质视频 | CAM/TC/TS/HC/SCR/WP/DVDSCR/R5/R5.Line/HalfCD/韩版硬字幕 |
| RMVB/RM/Flash | RealVideo 编码和 Flash 格式 |
| 单独样片 | 样片须与正片一起上传 |
| 无 CUE 多轨音频 | 无正确 cue 表单 |
| 压缩文件 | RAR 等压缩文件 |
| 重复资源 | 详见 DUPE 规则 |
| 敏感内容 | 禁忌/敏感宗教/政治话题 |
| 损坏文件 | 读取/回放错误 |
| 不受信任发布组 | 无法确认来源或来自公网 |
| 带广告水印 | 平台水印除外 |
| 垃圾文件 | 病毒/木马/广告/种子内含种子 |

#### DUPE 判定规则

**来源优先级**（高→低）：

```
UHD Blu-ray / Blu-ray ≥ UHDTV > HDTV > DVD
WEB-DL > WEBRip
```

- 动漫类 HDTV 和 DVD 同优先级（特例）
- 金种（PTP Golden Popcorn）Encode 享有最高优先级
- 基于无损截图/参数对比，高质量版本使低质量版本视为重复
- 高优先级发布组版本使低优先级组版本视为重复
- 不同区域/平台/不同配音或字幕 → 不视为重复
- 无损音轨每种原则上只保留一个版本（分轨 FLAC 优先级最高）

**不受 DUPE 约束的条件**：
- 旧版本连续断种 ≥ 45 天
- 旧版本已发布 ≥ 18 个月
- 本站工作组及合作制作组作品不受限制

#### 资源打包规则

**禁止**：非发行/出品方打包的合集、私自打包任何制作组作品

**允许打包**：
- 按套装售卖的合集
- 单季完结的剧集/综艺/动漫
- 同一艺术家 5 张以上专辑
- 分卷发售的动漫/角色歌/广播剧
- 打包视频必须同媒介/同分辨率/同编码

### 6.5 标题命名规范（rules.php 原文）

**标题必须全部是英文，不得出现非必要内容。**

#### 电影/演唱会/动漫/纪录片

```
英文名称 年份 [版本说明] 分辨率 [帧率] [地区/平台] 来源 视频编码 [色深] [色彩空间] 音频编码[声道]--发布组
```

示例：
```
Keeper of Darkness 2015 1080p HKG Blu-ray AVC TrueHD 7.1
Ekipazh 2016 3D HSBS 1080p 50fps WEB-DL HEVC AAC
```

#### 剧集/综艺/纪录/动漫

```
英文名称 年份 S**E** [版本说明] 分辨率 [帧率] [地区/平台] 来源 视频编码 [色深] [色彩空间] 音频编码[声道]-发布组
```

示例：
```
City of Streamer 2022 S01 Complete 1080p 50fps WEB-DL HEVC 10bit HDR AAC
Justice Served 2022 S01E01 1080p NF WEB-DL AVC DDP5.1
```

#### MV 类

```
歌手 - 英文歌名 年份 [版本说明] 分辨率 [帧率] [地区/平台] 来源 视频编码 [色深] [色彩空间] 音频编码[声道]-发布组名称
```

示例：
```
G.E.M. - Gloria 2022 2K WEB-DL ProRes PCM-HDVWEB
Jay Chou - You Are The Firework I Missed 2022 2160p 60fps WEB-DL AVC AAC
```

#### 音乐

```
艺术家名 - 专辑名 年份 [版本说明] 音频编码[-发布组名称]
```

示例：
```
Enya - And Winter Came 2008 FLAC
```

#### 副标题

```
中文名（多版本译名）、字幕信息，音轨信息等
禁止：广告、求种/续种请求
```

### 6.6 简介要求

所有种子必须：海报 → 影片简介 → MediaInfo/BDInfo → 截图（≥3 张）

- MediaInfo 用引用标签包裹（`[quote]`）
- NFO 写入 NFO 文件上传，不粘贴到简介
- 截图使用原图链接，不使用缩略图
- 体育节目禁止泄露比赛结果
- 音乐必须包含专辑封面和曲目列表

---

## 七、论坛发种规范（forums.php topicid=68 完整采集 2026-04-22）

> 来源：https://hdvideo.top/forums.php?action=viewtopic&topicid=68&page=p286#pid286
> 作者：茶小二（维护开发员）

### 7.1 媒介分类详解

| 媒介 | 说明 |
|------|------|
| UHD Blu-ray | 4K 蓝光原盘，未做任何改动（Untouched）。需检查 HDR/DV 并勾选标签 |
| UHD BluRay/DIY | 对 4K 蓝光原盘自定义操作（增删音轨/字幕/菜单/导评等） |
| Blu-ray 原盘 | 1080P/1080i 蓝光原盘，未做任何改动（Untouched） |
| BluRay/DIY | 对 1080P 蓝光原盘自定义操作。miniBD 归为此类 |
| Remux | 对 UHD/BD 原盘正片/花絮增删音轨字幕，制作为单独 mkv/mp4 |
| Encode | 对原始视频有损重编码（压制），标题常见 x264/x265/Rip 字样 |
| WEB-DL | 从流媒体平台直接下载。国内平台编码命名 H264/H265，国外 x264/x265 |
| HDTV | 特殊设备提取的数字源高清电视资源。UHDTV 可能有 HDR |
| DVD | DVD 原盘，最高分辨率 720p |

> 原盘（UHD/FHD/DIY）需具备完整蓝光结构文件夹或打包成 ISO。

### 7.2 标签使用规范

| 标签 | 说明 |
|------|------|
| 原创 | 自己购买/制作/下载的资源（原盘/DIY/Encode/Remux/流媒体/音乐等） |
| 国语 | 国语配音。正片只有一条混合语言音轨时，根据主要语言确定 |
| 粤语 | 粤语配音 |
| 官字组 | HDV 官组资源或本站字幕组介入制作的资源 |
| 中字 | 简体/繁体/简繁双语字幕 |
| Dolby Vision | 原盘双视频轨道或 MEL/FEL 单层/双层。DV 一定包含 HDR |
| HDR10 | MediaInfo: BT2020, HDR10 compatible |
| HDR10+ | MediaInfo: BT2020, HDR10+ compatible（比较少见） |
| HLG | 可归类至 HDR10（画面可见 HLG 标准的 HDR 资源） |
| 禁转 | 禁止转载他站。一般不适用于从友站转载来的资源 |
| 限转 | 限时禁止转载，标签消失前不可转载 |
| DIY | 仅适用于原盘资源的 DIY 制作 |
| 首发 | 仅适用于蓝光原盘的首次发布（全网，不仅是本站） |
| 应求 | 为应求求种区发布的资源 |
| 动画 | 非真人的动画/动漫类视频资源 |
| 金种 | 副标题有 PTP Golden Popcorn 字样 |
| 源码 | 源码视频 |
| 特效 | 特效字幕 |

### 7.3 音频编码选择规范

| 编码 | 分类 | 说明 |
|------|------|------|
| DTS-HD MA | 次世代无损 | 有对应标签 |
| TrueHD | 次世代无损杜比 | 有对应标签 |
| DTS-X | 次世代无损临境音 | 有对应标签 |
| TrueHD Atmos | 次世代无损杜比全景声 | 有对应标签 |
| DD/AC3 | 有损杜比 | 包含 DD/AC3/DDP，流媒体 DDP Atmos 也归此类 |
| DTS | 有损 DTS | 包含 DTS/DTS-ES/DTS-HiRes |

### 7.4 HDR 检测方法（审核员指南）

| HDR 类型 | BDInfo 检测 | MediaInfo 检测 |
|---------|------------|---------------|
| HDR10/HDR10+ | 视频轨道查找 HDR10/HDR10+ | HDR format 一栏查找 HDR10/HDR10 compatible/HDR10+ |
| DV | 视频轨道查找 Dolby Vision | HDR format 一栏查找 Dolby Vision |
| HLG | 视频轨道查找 HLG | Transfer characteristics 一栏查找 HLG |

> info 信息不全时结合截图判断：截图发绿 = DV，截图发灰 = HDR。冲突时以 MediaInfo 为准。

### 7.5 发布检查清单（审核员用）

1. 主副标题确认完整
2. 标签勾选：官方/金种/国语粤语/中字/DIY/类型/媒介/编码/音频编码/分辨率/制作组
3. 海报、简介、info、截图四要素完整
4. IMDb 信息栏验证
5. HDR/DV/HLG 标签检查（结合 info + 截图 + 制作组 + 媒介判断）
6. 语言标签检查（MI Chinese/Mandarin/Cantonese）
7. 中字标签检查（MI CHS/CHT/Chinese）
8. 注意：remux 外的 4K 原盘选 UHD Blu-ray，1080P 原盘选 FHD Blu-ray
9. 来源于 BluRay/DVD 的压制选 Encode
10. 注意区分 DDP/EAC3 vs DD/AC3

### 7.6 音乐种子发布规范

**标题格式**：
```
艺术家本国名 - 专辑本国名 发行年份 - 文件格式 分辨率(可选) - 制作组(如有)
```

**副标题格式**：
```
艺术家译名 - 专辑译名(可选) | 发行类别 | 厂牌 目录号 版本等其他信息 | 转载信息
```

**简介内容顺序**：专辑封面 → 音乐专辑信息（含曲目） → NFO → 频谱图

### 7.7 常见不合格种子（12 类）

1. 主标题含中文/括号/点号/封装格式
2. 副标题胡乱填写
3. 未补充 IMDb 链接
4. 海报缺失或防盗链
5. 影片简介缺失
6. 无完整 BDInfo/MediaInfo
7. 截图少于 3 张/使用缩略图/无截图
8. info 未使用引用标签
9. 未根据 info 补充媒介/编码/分辨率/语言/字幕/DIY/HDR 标签
10. 未经授权使用"官组"标签
11. 音乐种未按要求发布
12. 未按"海报→简介→info→截图"顺序
13. 添加广告信息

---

## 八、盒子规则（rules.php）

| 规则 | 说明 |
|------|------|
| 盒子报备 | 必须在控制面板登记 IPv4 和 IPv6 地址 |
| 未报备后果 | 未标记的盒子用户将被**取消下载权限**（全站白盒） |
| 限速建议 | 建议 100Mb/s 限速规则 |
| 48h 限制 | 种子发布后 48h 内，盒子最多计算 3 倍种子体积的上传流量 |
| 误标记反馈 | 家宽/教育网被误标为盒子的，通过管理组信箱反馈 |

> **全站白盒模式**：未在控制面板登记盒子地址的用户将被取消下载权限，这是强制要求。

> **对 PT-Forward 的影响**：PT-Forward 部署在盒子服务器时，必须确保已在 HDVideo 控制面板登记盒子 IP 地址。48h 上传量限制对转发行为无直接影响（转发是发布新种子，不是做种下载）。
