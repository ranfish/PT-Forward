# CarPT 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | CarPT |
| 站点地址 | https://carpt.net |
| 站点框架 | NexusPHP |
| 主题 | Classic（经典蓝色主题） |
| 口号 | CarPT - =链接@分享= |
| 特殊功能 | 候选区(offers)、认领制度(claim)、H&R规则、置顶促销(sticky-promotion)、签到 |
| 规则页面 | rules.php |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 模式系统

使用 `data-mode='4'` 属性控制字段显示，字段名带 `[4]` 后缀。标准综合区模式。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名，要求规范填写） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（`data-pt-gen="url"`） |
| `pt_gen` | text | - | PT-Gen 链接（`data-pt-gen="pt_gen"`） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

**PT-Gen 支持**: 支持4种来源——imdb / douban / bangumi / indienova

**注意**: 无 `technical_info`（MediaInfo）独立字段，MediaInfo 应写在简介中。

### 1.3 类型字段（`type`，data-mode=4）— 7个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影/Movies |
| 402 | 连续剧/TV-Series |
| 403 | 动漫/Animation |
| 404 | 纪录片/Documentary |
| 405 | 综艺/TV-Show |
| 406 | 音乐/Music |
| 407 | 其他/Other |

**注意**: 分类名使用中英双语。无"体育"分类（体育归入"其他"）。

### 1.4 媒介（`medium_sel[4]`）— 9个

| 值 | 显示名称 |
|----|----------|
| 1 | Encode |
| 2 | WEB |
| 3 | HDTV |
| 4 | DVDRip |
| 5 | CD |
| 6 | Other |
| 7 | Blu-ray |
| 8 | Blu-rayUHD |
| 9 | Remux |

**注意**: 值不按质量排列（1=Encode, 7=Blu-ray, 8=Blu-rayUHD, 9=Remux）。WEB=2 而非常见 WEB-DL。区分 Blu-ray(7) 和 Blu-rayUHD(8)。有 DVDRip(4) 和 CD(5)。

### 1.5 编码（`codec_sel[4]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC/x264 |
| 2 | H.265/HEVC/x265 |
| 3 | MPEG-2 |
| 4 | VC-1 |
| 5 | Xvid |
| 6 | Other |

**注意**: 无 AV1 选项。编码名称合并了编码标准名称和编码器名称（如 "H.264/AVC/x264"）。

### 1.6 音频编码（`audiocodec_sel[4]`）— 11个

| 值 | 显示名称 |
|----|----------|
| 1 | TrueHD |
| 2 | DTS-HD/DTS |
| 3 | AC3 |
| 4 | LPCM |
| 5 | FLAC |
| 6 | mp3 |
| 7 | AAC |
| 8 | APE |
| 9 | Other |
| 10 | wav |

**注意**: 值10（wav）跳过了值9（Other）后面。有 LPCM(4) 和 APE(8)。DTS-HD 和 DTS 合并为 DTS-HD/DTS(2)。

### 1.7 分辨率（`standard_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | 4K_UHD |
| 2 | 1080p/i |
| 3 | 720p/i |
| 4 | SD |
| 5 | Other |

**注意**: 极简分辨率列表，仅5个选项。1080p 和 1080i 合并为 "1080p/i"，同理 720p/i。无 1440p、540p 等中间分辨率。4K 表示为 "4K_UHD"。

### 1.8 制作组（`team_sel[4]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | CarPT |
| 2 | WiKi |
| 3 | CMCT |
| 4 | M-team |
| 5 | Other |

**注意**: 仅5个选项。包含其他站点的官组名（WiKi、CMCT、M-team），这些是非本站官组的知名制作组。非管理组发布的资源**禁止选择 CarPT 制作组**。

### 1.9 标签（`tags[4][]`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

**注意**: 仅6个标签，是已采集站点中最少的之一。值3缺失（跳过）。无"完结"、"分集"、"特效"、"DV"等常见标签。

### 1.10 匿名发布

`uplver` checkbox，value="yes"，默认未勾选。

---

## 二、发种规则（rules.php）

### 2.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 上传者必须保证上传速度与做种时间：做种时间不足24小时或故意低速上传将被警告甚至取消上传权限
- 发布者获得双倍上传量
- **非网站管理组发布的种子资源时，禁止用官方标签或制作组选择 CarPT**
- **禁止发布色情(含三级)资源**
- 不要发布小于10MB资源的种子

### 2.2 上传者资格

- 任何人都能发布资源
- 有些用户需要先在候选区提交候选
- 游戏类资源只有上传员及以上等级用户可自由上传，其他用户必须先候选

### 2.3 允许的资源

- 高清（HD）视频：Blu-rayUHD、Blu-ray、x264/x265、WEB、HDTV（**不能发枪版与含广告的**）
- HDTV 流媒体
- 高清重编码（至少 720p）
- 标清重编码（至少 480p，来源于高清媒介）
- DVDR/DVDISO、DVDRip、CNDVDRip
- 无损音轨（FLAC、Monkey's Audio 等）及 cue 表单
- 5.1 声道及以上音轨、评论音轨
- PC 游戏（原版光盘镜像）
- 7日内高清预告片
- 高清相关软件和文档

### 2.4 不允许的资源

- 总体积小于 100MB 的资源
- 标清 upscale 视频
- CAM、TC、TS、SCR、DVDSCR、R5、R5.Line、HalfCD 等低质量标清
- RealVideo 编码（RMVB/RM）、flv 文件
- 单独的样片
- 未达 5.1 声道的有损音频（MP3、WMA 等）
- 无正确 cue 的多轨音频
- 硬盘版/高压版游戏、非官方游戏镜像、第三方 mod、单独破解/补丁
- RAR 等压缩文件
- 重复（dupe）资源
- 色情/敏感政治内容
- 损坏文件、垃圾文件

### 2.5 Dupe 规则

- 优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 同一视频高清版本使标清版本被视为 dupe
- **动漫类特例**：HDTV 和 DVD 版本有相同优先级
- 相同媒介+分辨率的重编码，按发布组确定优先级（参考论坛帖子）
- 总会保留一个 DVD5 大小（~4.38GB）的重编码版本
- 不同区域/配音/字幕的 Blu-ray/HD DVD 不视为 dupe
- 每个无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- 旧版本连续断种 45 日或已发布 18 个月以上时，新版本不受 dupe 约束
- 新版本无旧版错误/画质问题，或来源质量更好时允许发布

### 2.6 资源打包规则（试行）

允许打包：
- 高清电影合集（套装盒装）
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，不允许单曲单独发布）
- 同一艺术家音乐（5张以上专辑打包，两年内新专辑可单独发布）
- 发布组打包发布的资源

打包视频须：相同媒介类型、相同分辨率、相同编码格式。打包音频须：相同编码格式。

### 2.7 标题命名规范

- 电影：`[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 电视剧：`[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 音轨：`[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 游戏：`[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]`

### 2.8 简介要求

- 电影/电视剧/动漫：必须包含海报/封面，尽可能包含截图、详细文件信息、演职员和剧情
- 体育节目：禁止在简介/截图/文件名中泄漏比赛结果
- 音乐：必须包含专辑封面和曲目列表
- PC 游戏：必须包含海报/封面，尽可能包含截图
- NFO 图应写入 NFO 文件，不粘贴到简介

### 2.9 H&R 规则

- 下载完成后 **10天内**做种时间须达 **24小时**
- 上传量 ≥ 10倍种子体积则直接通过
- HR 未达标数 ≥ 20 将被封号
- 免除一个 HR 记录需 20000 魔力

### 2.10 种子认领规则

- 种子发布7天后可认领
- 每个种子最多20人认领，每人最多认领1000个
- 达标标准：每月做种 ≥ 300小时（12.5天），或上传量 ≥ 5倍体积
- 达标种子魔力奖励为正常值的2倍

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "电影": 401,
  "连续剧": 402,
  "动漫": 403,
  "纪录片": 404,
  "综艺": 405,
  "音乐": 406,
  "其他": 407
}
```

### 3.2 媒介（`medium_sel[4]`）

```json
{
  "Encode": 1,
  "WEB": 2,
  "HDTV": 3,
  "DVDRip": 4,
  "CD": 5,
  "Other": 6,
  "Blu-ray": 7,
  "Blu-rayUHD": 8,
  "Remux": 9
}
```

### 3.3 编码（`codec_sel[4]`）

```json
{
  "H.264/AVC/x264": 1,
  "H.265/HEVC/x265": 2,
  "MPEG-2": 3,
  "VC-1": 4,
  "Xvid": 5,
  "Other": 6
}
```

### 3.4 音频编码（`audiocodec_sel[4]`）

```json
{
  "TrueHD": 1,
  "DTS-HD/DTS": 2,
  "AC3": 3,
  "LPCM": 4,
  "FLAC": 5,
  "mp3": 6,
  "AAC": 7,
  "APE": 8,
  "Other": 9,
  "wav": 10
}
```

### 3.5 分辨率（`standard_sel[4]`）

```json
{
  "4K_UHD": 1,
  "1080p/i": 2,
  "720p/i": 3,
  "SD": 4,
  "Other": 5
}
```

### 3.6 制作组（`team_sel[4]`）

```json
{
  "CarPT": 1,
  "WiKi": 2,
  "CMCT": 3,
  "M-team": 4,
  "Other": 5
}
```

### 3.7 标签（`tags[4][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR": 7
}
```

---

## 四、CarPT 特殊注意事项

### 4.1 制作组选择限制

非管理组发布的资源**禁止选择 CarPT 制作组**。转载资源应选 Other(5) 或根据实际制作组选择 WiKi(2)/CMCT(3)/M-team(4)。

### 4.2 禁止色情内容

规则明确"禁止发布色情(含三级)的资源"，比多数站点更严格。

### 4.3 最小体积限制

不允许发布小于 10MB 的资源种子。允许的总体积最小为 100MB（部分例外：高清软件/文档、单曲专辑）。

### 4.4 WEB 媒介命名

媒介使用 "WEB"(2) 而非常见的 "WEB-DL"，编码器合并显示（如 "H.264/AVC/x264"）。

### 4.5 分辨率极简化

仅5个分辨率选项，1080p/i 和 720p/i 各合并为一个选项，无中间分辨率。

### 4.6 标签极简化

仅6个标签（已采集站点中最少之一），无"完结"、"分集"、"特效"、"DV"等常见标签。标签值3缺失。

### 4.7 无 AV1 编码

视频编码选项中无 AV1，只有 H.264、H.265、MPEG-2、VC-1、Xvid、Other。

### 4.8 无 MediaInfo 独立字段

表单中无 `technical_info` 字段，MediaInfo 应写在简介中。

### 4.9 动漫 dupe 特例

动漫类视频资源的 HDTV 和 DVD 版本有相同优先级，不受常规 dupe 规则约束。

### 4.10 Dupe 断种时限

旧版本连续断种 **45日**以上才允许发布新版本（多数站点为7日或30日）。已发布 **18个月**以上也不受 dupe 约束。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | CarPT | 常见 NexusPHP |
|------|--------|---------------|
| 模式系统 | data-mode=4，字段带[4]后缀 | 多种模式 |
| 类型数量 | 7个（无体育） | 通常 8-10 个 |
| 媒介 | 9个，WEB(2)非WEB-DL，区分Blu-ray/Blu-rayUHD | 通常 6-10 个 |
| 编码 | 6个，无AV1，合并编码器名 | 通常 5-9 个 |
| 音频编码 | 11个，含LPCM/APE | 通常 6-12 个 |
| 分辨率 | 5个（极简），合并p/i | 通常 5-7 个 |
| 制作组 | 5个（含其他站官组） | 通常 3-30 个 |
| 标签 | 6个（最少） | 通常 3-21 个 |
| MediaInfo | 无独立字段 | 通常有 technical_info |
| PT-Gen | 4种来源 | 通常 1-3 种 |
| Dupe 断种 | 45日 | 通常 7-30 日 |
| 色情内容 | 禁止（含三级） | 因站而异 |
| 最小体积 | 10MB/100MB | 因站而异 |

---

## 六、适配器实现要点

### 6.1 字段名

```go
TorrentFileField: "file",                    // 标准
TitleField:      "name",                     // 标准
SubtitleField:   "small_descr",              // 标准
PtGenField:      "pt_gen",                   // 标准
DescrField:      "descr",                    // 标准
AnonField:       "uplver",                   // 标准
TypeField:       "type",                     // 标准（无后缀）
MediumField:     "medium_sel[4]",            // 带 [4] 后缀
CodecField:      "codec_sel[4]",             // 带 [4] 后缀
AudioCodecField: "audiocodec_sel[4]",        // 带 [4] 后缀
ResolutionField: "standard_sel[4]",          // 带 [4] 后缀
TeamField:       "team_sel[4]",              // 带 [4] 后缀
TagsField:       "tags[4][]",                // 带 [4] 后缀，数组
```

### 6.2 制作组映射

转载资源不应使用 CarPT(1)，应映射为 Other(5)，除非确认源制作组为 WiKi/CMCT/M-team：

```go
func mapTeam(sourceTeam string) int {
    switch {
    case strings.Contains(sourceTeam, "WiKi"):
        return 2
    case strings.Contains(sourceTeam, "CMCT"):
        return 3
    case strings.Contains(sourceTeam, "M-team") || strings.Contains(sourceTeam, "MT"):
        return 4
    default:
        return 5 // Other
    }
}
```

### 6.3 媒介映射注意

WEB-DL / WEBRip 在 CarPT 映射为 WEB(2)，不是常见的 WEB-DL：

```go
func mapMedium(sourceMedium string) int {
    switch {
    case containsAny(sourceMedium, "Remux"):
        return 9
    case containsAny(sourceMedium, "UHD", "4K", "2160p") && containsAny(sourceMedium, "Blu-ray", "BluRay"):
        return 8  // Blu-rayUHD
    case containsAny(sourceMedium, "Blu-ray", "BluRay"):
        return 7  // Blu-ray
    case containsAny(sourceMedium, "WEB-DL", "WEBRip", "WEB"):
        return 2  // WEB
    // ...
    }
}
```

### 6.4 分辨率合并

1080p 和 1080i 在 CarPT 合并为 "1080p/i"(2)，无需区分逐行/隔行扫描。

---

*数据来源: upload.php HTML (561行) + rules.php HTML (474行) (2026-04-16)*
*文档创建: 2026-04-16*
