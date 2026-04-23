# SBPT 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | SBPT|
| 站点地址 | https://sbpt.link |
| 站点框架 | NexusPHP |
| 主题 | BlueGene |
| 定位 | 中型转载站，目标种子量5-20万，易入门 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名，要求规范填写） |
| `small_descr` | text | - | 副标题 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode，20行） |

注意：SBPT 无 `url`（IMDb链接）、`pt_gen`（PTGen链接）、`technical_info`（MediaInfo）字段。但页面引用了 `js/ptgen.js`，可能通过"填写质量"按钮或 PTGen 辅助填写标题。

### 1.2 质量选择字段

字段名带 `[4]` 后缀，单模式（mode=4）。

#### 类型（`type`）— 必填，data-mode='4'

| 值 | 显示名称 |
|----|----------|
| 401 | 电影(Movie) |
| 402 | 电视剧(TV Series) |
| 403 | 综艺(TV Show) |
| 404 | 纪录片(Documentary) |
| 405 | 动画(Animation) |
| 406 | 音乐短片(MV)Music Videos |
| 407 | 体育(Sport) |
| 408 | 音乐(Music) |
| 409 | 其他 |

#### 媒介（`medium_sel[4]`）— 12个

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
| 10 | WEB-DL |
| 11 | WEBRip |
| 12 | BDRip |

注意：SBPT 有 MiniBD(4)、WEBRip(11)、BDRip(12) 三个在多数 NexusPHP 站点中不常见的媒介选项。无 UHD 独立选项（靠标签区分）。

#### 视频编码（`codec_sel[4]`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 / AVC |
| 2 | VC-1 |
| 3 | MPEG-4 Part 2 (如 Xvid/DivX) |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265 / HEVC |
| 7 | VP8 |
| 8 | VP9 |
| 9 | ProRes |
| 10 | H.266 / VVC |

注意：
- H.264 和 H.265 不区分原盘（AVC/HEVC）和压制（x264/x265），合并为 H.264/AVC 和 H.265/HEVC
- 包含 H.266/VVC(10) 前瞻性编码
- 包含 VP8(7)、VP9(8)、ProRes(9) 专业/网页编码

#### 分辨率（`standard_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p |

#### 制作组（`team_sel[4]`）— 仅5个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |

#### 标签（`tags[4][]`）— 14个

| 值 | 显示名称 |
|----|----------|
| 1 | 中英双语字幕 |
| 2 | 原盘BDMV |
| 4 | DIY |
| 5 | 国语 |
| 6 | 粤语 |
| 7 | 3D |
| 8 | mUHD |
| 9 | UHD Blu-ray |
| 10 | Remux |
| 12 | WEB-DL |
| 13 | CC |
| 14 | 特效字幕 |
| 15 | 合集 |
| 16 | 原盘ISO |

注意：
- 值3和值11缺失，可能已弃用
- 标签中包含多个媒介/质量相关标签（原盘BDMV、Remux、WEB-DL、UHD Blu-ray、原盘ISO、mUHD），与媒介选择有一定功能重叠
- mUHD(8) 可能指 mini UHD 或移动端 UHD
- CC(13) 指 Criterion Collection

### 1.3 缺失字段

- `audiocodec_sel` — 无音频编码选择
- `processing_sel` — 无地区选择
- `technical_info` — 无 MediaInfo/BDInfo 输入框
- `url` — 无 IMDb 链接输入框
- `pt_gen` — 无 PTGen 链接输入框（但有 ptgen.js 辅助）
- `uplver` — 无匿名发布选项

---

## 二、标题命名规范

来源：`rules.php` → 种子信息

### 2.1 标题格式

| 类型 | 格式 |
|------|------|
| 电影 | `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组` |
| 电视剧 | `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组` |
| 音轨 | `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组]` |
| 游戏 | `[中文名] 名称 [年份] [版本] [发布说明][-发布组]` |

### 2.2 标题示例

- 电影：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS`
- 电视剧：`越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU`
- 音轨：`恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC`
- 游戏：`红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED`

### 2.3 简介要求

- 电影/电视剧/动漫：必须包含海报/封面，尽可能包含截图、MediaInfo、演职员和剧情概要
- NFO 应写入 NFO 文件而非粘贴到简介
- 体育节目：禁止在简介中泄漏比赛结果
- 音乐：必须包含专辑封面和曲目列表

---

## 三、发布规则

### 3.1 允许的资源

- 高清视频（Blu-ray/HD DVD 原碟、Remux、HDTV、720p+ 重编码）
- 标清视频（仅限来源于高清媒介的重编码，至少480p）
- DVDR/DVDISO/DVDRip/CNDVDRip
- 无损音轨（FLAC、Monkey's Audio等）
- 5.1声道+ 音轨（DTS、DTSCD等）
- PC游戏（必须原版光盘镜像）
- 7日内高清预告片
- 高清相关软件和文档（<100MB）

### 3.2 禁止的资源

- 总体积 < 100MB
- 标清 upscale 视频
- CAM、TC、TS、SCR、DVDSCR、R5、HalfCD 等低质量
- RealVideo/RMVB/RM/FLV
- 单独样片
- < 5.1声道有损音频（MP3、WMA等）
- RAR 压缩文件
- 含可执行文件（.exe、.bat、.sh）— 一经发现直接封禁
- 1990年以前作品（特殊原因需在详情页说明）
- 非官方 AI 修复作品
- 短剧作品（暂行）
- 有声书作品（暂行）
- 电子书：仅限 PDF（需正规来源，结构完整，禁止水印）

### 3.3 重复（Dupe）判定

- 媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 动漫特例：HDTV 和 DVD 同优先级
- 断种45日+ 或发布18月+ 可重发
- 无损音轨只保留一个版本（分轨FLAC优先级最高）
- 发布者获得双倍上传量

### 3.4 资源打包规则

- 按套装售卖的电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 同一艺术家MV（标清MV只允许DVD打包，禁止单曲单独发布）
- 5张+专辑或两年内专辑可单独发布
- 发布组打包资源

### 3.5 促销规则

随机促销（上传后自动触发）：
- 40% 概率 → 50%下载
- 20% 概率 → 免费
- 10% 概率 → 2x上传
- 10% 概率 → 50%下载 & 2x上传
- 10% 概率 → 免费 & 2x上传
- 总体积 > 20GB → 自动免费
- 关注度高的种子由管理员设为促销

### 3.6 账号保留规则

| 条件 | 规则 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存账号后不会被删除 |
| 封存账号 | 连续 400 天不登录删除 |
| 未封存账号 | 连续 **365** 天不登录删除 |
| 无流量账号 | 连续 **60** 天不登录删除 |

### 3.7 站点特殊政策

- 允许一人多IP、多账号
- 账号允许交易（但论坛内禁止交易相关内容）
- 盒子限速 2Gbps，无需备案
- 一次性邮箱可注册
- 不发邮件、不需邮箱验证
- 禁止使用纯数字 QQ 邮箱或拼音可读邮箱（保护隐私防连坐）
- 用 @qq.com 注册暂未封禁，但规则中已列出
- 作弊行为封号，但魔力/HR 系统宽松，不作弊也能轻松生存
- 站点定位：中型转载站，目标种子量 5-20 万，易入门

### 3.8 禁止发布规定汇总（论坛 topicid=8）

| 禁止内容 | 说明 |
|----------|------|
| 1990 年以前作品 | 需在详情页写理由（如周年纪念、NAS珍藏等） |
| 非官方 AI 修复作品 | 完全禁止 |
| 短剧作品 | 暂行规定 |
| 有声书作品 | 暂行规定 |
| 种子内含可执行文件 | .exe/.bat/.sh 等，发现即封号 |
| 电子书非 PDF | 仅限 PDF，须正规来源、结构完整、有目录书签、可搜索、无水印 |
| 站内论坛账号交易 | 禁止（站外允许） |
| 公开平台发送站点链接/截图 | 禁止 |

---

## 四、站点适配器配置参考

```yaml
site:
  id: "sbpt"
  name: "SBPT"
  url: "https://sbpt.link"
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
      "音乐": 408
      "其他": 409

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
      "WEB-DL": 10
      "WEBRip": 11
      "BDRip": 12

    codec_sel:
      "H264": 1
      "VC-1": 2
      "MPEG4": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 6
      "VP8": 7
      "VP9": 8
      "ProRes": 9
      "H266": 10

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

    tags:
      "中英双语字幕": 1
      "原盘BDMV": 2
      "DIY": 4
      "国语": 5
      "粤语": 6
      "3D": 7
      "mUHD": 8
      "UHD Blu-ray": 9
      "Remux": 10
      "WEB-DL": 12
      "CC": 13
      "特效字幕": 14
      "合集": 15
      "原盘ISO": 16

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
    - "url"
    - "pt_gen"
    - "uplver"

  quirks:
    relaxed_rules: "中型转载站，规则较宽松，无严格命名校验"
    no_anonymous: "无匿名发布选项"
    no_imdb_input: "无IMDb链接输入框，外部信息写入简介"
    tag_value_gap: "标签值3和11缺失"
    mini_uhd_tag: "mUHD标签含义待确认"
```

---

## 五、发布流水线注意事项

### 5.1 制作组映射

SBPT 仅有5个制作组（HDS/CHD/MySiLU/WiKi/Other），转种时非上述制作组统一选 Other(5)。

### 5.2 缺失字段处理

- 无 `audiocodec_sel`：不需要提交音频编码
- 无 `technical_info`：MediaInfo 应写入简介（`descr`）
- 无 `url`/`pt_gen`：IMDb 链接等外部信息写入简介
- 无 `uplver`：无法匿名发布

### 5.3 标题格式

SBPT 使用标准 HD 站标题格式（类似 NexusHD 规范），转种时需按规则格式化：
- 保留原始英文名称和发布组
- 分辨率和来源使用标准格式（720p、1080p、BluRay、HDTV等）
- 规则要求"尽量使用原始发布信息"

### 5.4 禁止内容过滤

发布前需检查：
- 发布年份 ≥ 1990（除非有理由）
- 非 AI 修复作品
- 非短剧、非有声书
- 种子内无可执行文件
- 电子书仅 PDF

---

*分析时间：2026-04-16*
*最后更新：2026-04-22*
*数据来源：https://sbpt.link/forums.php?action=viewtopic&forumid=1&topicid=8 + https://sbpt.link/rules.php + https://sbpt.link/upload.php 发布页面 HTML 分析*
