# 海胆 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 海胆 |
| 站点地址 | https://www.haidan.cc |
| 站点框架 | NexusPHP |
| Tracker URL | `https://announce.haidan.cc/announce.php` |
| 特殊功能 | season/episode 独立字段、team_suffix 制作组后缀、durl 豆瓣链接、collages 自动收藏、种子置顶/免费（魔力值购买） |
| 规则页面 | rules.php |
| 建站时间 | 2020 年 |

**站点角色**: 无官组，**只能做目标站（发布站），不能做源站**。

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form/data）

### 1.1 无模式系统

无 `data-mode` 属性，字段名无 `[]` 后缀。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（推荐英文 0day 命名法） |
| `small_descr` | text | - | 副标题 |
| `durl` | text | - | 豆瓣链接（独有字段名） |
| `url` | text | - | IMDb 链接 |
| `screenshot_input` | textarea | - | 截图区（一行一条图片链接） |
| `nfo` | file | - | NFO 文件 |
| `nfo_text` | textarea | - | 或粘贴 NFO 文本（文件优先） |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `season` | text | - | 季数（电视剧/综艺/动画/纪录片分集须填写） |
| `episode` | text | - | 集数（0=全季） |
| `team_suffix` | text | ✓ | 制作组后缀（例：sGnB, CMCT...） |
| `collages` | checkbox | - | 自动加入收藏夹（value=1） |
| `uplver` | checkbox | - | 匿名发布 |

**独特字段**:
- **`durl`**: 豆瓣链接，非标准 `pt_gen` 字段
- **`season`/`episode`**: 剧集季集独立文本字段（季数=0 代表不区分季，集数=0 代表全季）
- **`team_suffix`**: 制作组后缀，自由文本输入（必填）
- **`collages`**: 自动加入收藏夹
- **`screenshot_input`**: 截图独立区域（一行一条 URL）
- **`nfo_text`**: NFO 文本粘贴区（文件和文本同时存在时以文件为准）

**缺失**: 无 `pt_gen` 字段、无 `technical_info`（MediaInfo）字段。

### 1.3 类型字段（`type`）— 9个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 402 | TV Series(电视剧) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录片) |
| 405 | Animations(动画片) |
| 406 | Music Videos(MV) |
| 407 | Sports(体育) |
| 408 | HQ Audio(音乐) |
| 409 | Misc(其他) |

**注意**: 分类名使用中英双语。标准9类，无游戏/软件/漫画分类。

### 1.4 媒介（`medium_sel`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 3 | Remux |
| 5 | HDTV |
| 6 | DVD |
| 7 | Encode |
| 8 | CD |
| 9 | UHD Blu-ray |
| 11 | WEB-DL |

**注意**: 值不连续（1,3,5,6,7,8,9,11）。有 UHD Blu-ray(9)。无 Other/WEBRip/Remux DIY。

### 1.5 编码（`codec_sel`）— 6个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/AVC/X264 |
| 2 | VC-1 |
| 4 | MPEG-2 |
| 5 | Other |
| 11 | H.265/HEVC/X265 |
| 13 | MPEG-4/XviD/DivX |

**注意**: 编码名称合并了标准/编码器名（如 "H.264/AVC/X264"）。值不连续。无 AV1。有 MPEG-4/XviD/DivX(13)。

### 1.6 音频编码（`audiocodec_sel`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 6 | AAC |
| 7 | Other |
| 10 | AC3 |
| 11 | LPCM |
| 12 | DTS-HDMA |
| 13 | True-HD |

**注意**: 值不连续。有 DTS(3) 和 DTS-HDMA(12) 分开。无 Atmos、DTS:X、DDP/E-AC-3、WAV。

### 1.7 分辨率（`standard_sel`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | 2160p/4K |
| 2 | 1080p |
| 3 | 1080i |
| 4 | 720p |
| 5 | 540P |

**注意**: 区分 1080p(2) 和 1080i(3)。有 540P(5)（非 SD）。无 SD、4320p、Other。

### 1.8 标签（`tag_list[]`）— 6个

| 值 | 显示名称 | 颜色 |
|----|----------|------|
| 3 | 中字 | 蓝色 #0000ff |
| 4 | DIY | 浅蓝 #0080ff |
| 5 | 国语 | 紫色 #8000ff |
| 7 | 原盘 | 青蓝 #0080c0 |
| 10 | 粤语 | 绿色 #00ff00 |
| 11 | 外语 | 深青 #004040 |

**注意**: 值不连续（3,4,5,7,10,11）。使用彩色 label 样式。字段名为 `tag_list[]`（非 `tags[]`）。无首发/禁转/HDR/DV。

### 1.9 缺失字段

- 无 `team_sel`（制作组下拉）— 使用 `team_suffix` 文本输入代替
- 无 `pt_gen` 字段 — 使用 `durl`（豆瓣链接）代替
- 无 `technical_info`（MediaInfo）
- 无 `processing_sel`（处理/地区）
- 无 `source_sel`（来源）

---

## 二、字段映射汇总（实际发布用）

### 2.1 类型（`type`）

```json
{
  "Movies": 401,
  "TV Series": 402,
  "TV Shows": 403,
  "Documentaries": 404,
  "Animations": 405,
  "Music Videos": 406,
  "Sports": 407,
  "HQ Audio": 408,
  "Misc": 409
}
```

### 2.2 媒介（`medium_sel`）

```json
{
  "Blu-ray": 1,
  "Remux": 3,
  "HDTV": 5,
  "DVD": 6,
  "Encode": 7,
  "CD": 8,
  "UHD Blu-ray": 9,
  "WEB-DL": 11
}
```

### 2.3 编码（`codec_sel`）

```json
{
  "H.264/AVC/X264": 1,
  "VC-1": 2,
  "MPEG-2": 4,
  "Other": 5,
  "H.265/HEVC/X265": 11,
  "MPEG-4/XviD/DivX": 13
}
```

### 2.4 音频编码（`audiocodec_sel`）

```json
{
  "FLAC": 1,
  "APE": 2,
  "DTS": 3,
  "MP3": 4,
  "AAC": 6,
  "Other": 7,
  "AC3": 10,
  "LPCM": 11,
  "DTS-HDMA": 12,
  "True-HD": 13
}
```

### 2.5 分辨率（`standard_sel`）

```json
{
  "2160p/4K": 1,
  "1080p": 2,
  "1080i": 3,
  "720p": 4,
  "540P": 5
}
```

### 2.6 标签（`tag_list[]`）

```json
{
  "中字": 3,
  "DIY": 4,
  "国语": 5,
  "原盘": 7,
  "粤语": 10,
  "外语": 11
}
```

---

## 三、海胆之家特殊注意事项

### 3.1 仅目标站

无官组，只能做目标站。在 PT-Forward 中应标记为 `SourceEnabled=false`。

### 3.2 team_suffix 文本输入

无制作组下拉，使用 `team_suffix` 自由文本输入（60px 宽度）。适配器可填入源资源制作组名称。

### 3.3 season/episode 独立字段

剧集的季/集通过 `season` 和 `episode` 独立文本字段填写，而非从标题中解析。

### 3.4 durl 豆瓣链接

使用 `durl` 字段填写豆瓣链接，非标准的 `pt_gen`。无 PT-Gen 自动获取功能。

### 3.5 标签字段名 tag_list[]

标签字段名为 `tag_list[]`（非常见的 `tags[]`），且使用彩色 label 样式。无首发/禁转/HDR 标签。

### 3.6 无制作组下拉

无 `team_sel` 下拉，使用 `team_suffix` 文本字段。适配器直接填写制作组名称字符串。

### 3.7 Cloudflare 防护

站点使用 Cloudflare 防护，需有效的 `cf_clearance` cookie。

### 3.8 魔力值购买促销/置顶

upload 页面可直接购买：
- **种子置顶**：24h/5000、48h/9000、96h/17000、168h/26000 魔力值
- **种子免费**：24h/3000、48h/5000、168h/18000 魔力值

### 3.9 做种人数自动促销

系统根据做种人数动态设定促销（非发布时固定）：
- 做种 1-7 人 → 免费
- 做种 8-14 人 → 30% 下载
- 做种 14-21 人 → 50% 下载
- 做种 22+ 人 → 普通（无促销）

**动态计算**：同一种子多人下载时，前 7 名免费，后续用户按实时保种人数计算。

### 3.8 collages 自动收藏

`collages` checkbox 可勾选将种子自动加入收藏夹，便于后续打包整理。

---

## 四、与其他 NexusPHP 站点对比

| 特征 | 海胆之家 | 常见 NexusPHP |
|------|----------|---------------|
| 站点角色 | **仅目标站** | 源站/目标站 |
| PT-Gen | **无**（用 durl 豆瓣链接） | 有 |
| MediaInfo | **无** | 通常有 |
| 制作组 | **文本输入**（team_suffix） | 下拉选择 |
| 季/集 | **独立字段**（season/episode） | 从标题解析 |
| 标签字段名 | `tag_list[]` | `tags[]` |
| 标签数量 | 6个（无首发/禁转/HDR） | 通常 3-22 个 |
| 豆瓣链接 | **独有 durl 字段** | 通常无 |
| 分辨率 | 5个（含540P） | 通常 5-7 个 |

---

## 五、适配器实现要点

### 5.1 制作组文本输入

```go
form.Set("team_suffix", req.TeamName) // Free text, e.g. "FRDS"
```

### 5.2 季集字段

```go
if req.Season > 0 {
    form.Set("season", strconv.Itoa(req.Season))
}
if req.Episode > 0 {
    form.Set("episode", strconv.Itoa(req.Episode))
}
```

### 5.3 豆瓣链接

```go
if req.DoubanURL != "" {
    form.Set("durl", req.DoubanURL)
}
```

### 5.4 标签字段名注意

```go
// Use tag_list[] not tags[]
TagsField: "tag_list[]",
```

### 5.5 跳过缺失字段

```go
adapter.SkipPTGen = true
adapter.SkipMediaInfo = true
adapter.SkipTeam = true // Use team_suffix text field instead
```

---

## 六、发种规则（rules.php）

### 6.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 保证上传速度与做种时间——撤种或做种不足 24 小时，或故意低速上传 → 警告甚至取消上传权限
- 发布者获得**双倍上传量**
- 违规但有价值的资源可联系管理组破例
- **任何人都能发布资源**（有些用户需先在候选区提交候选）

### 6.2 允许的资源

- **高清（HD）视频**：蓝光/HD DVD 原碟或 remux、HDTV 流媒体、高清重编码（≥720p）、高清 DV、WEB-DL
- **标清（SD）视频**：仅限高清媒介标清重编码（≥480p）、DVDR/DVDISO、DVDRip/CNDVDRip
- **无损音轨**（及 cue 表单）：FLAC、Monkey's Audio 等
- **5.1 声道或以上**的电影/音乐音轨（DTS、DTS CD 镜像等）、评论音轨
- **PC 游戏**（必须为原版光盘镜像）
- **电子书相关资源**
- 基本上"除禁忌或敏感外，任何你认为有价值的内容"

### 6.3 不允许的资源

- **未经发布者允许的他站禁转资源**
- RAR 等压缩文件
- 重复（dupe）资源
- 禁忌或敏感内容（色情、敏感政治话题等）
- 损坏的文件
- 垃圾文件（病毒/木马/广告/种中种等）
- **体积 < 200MB** 的资源

### 6.4 Dupe 判定规则

- 完全相同的文件 → dupe
- 相同媒介、相同分辨率的高清重编码 → dupe
- 新版本无旧版已确认错误/画质问题，或来源质量更好 → 允许发布，旧版成 dupe
- 旧版连续断种 **45 日以上**或已发布 **18 个月以上** → 新版不受 dupe 约束

### 6.5 资源打包规则（试行）

**允许打包**：
- 按套装售卖的高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7 日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，高清分辨率相同）
- 同一艺术家音乐（5+ 专辑，两年内可单独发布）
- 分卷动漫/角色歌/广播剧等
- 发布组打包资源

**打包要求**：相同媒介/分辨率/编码（预告片例外），音频格式一致

### 6.6 标题格式（0day 命名法）

- 电影：`[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 电视剧：`[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 音轨：`[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 游戏：`[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]`

### 6.7 种子信息要求

- 电影/电视剧/动漫：必须海报/封面，尽量截图+格式详情+演职员+剧情概要
- 体育节目：不得泄露比赛结果
- 音乐：必须专辑封面+曲目列表
- PC 游戏：必须海报/封面
- **外部信息**：电影和电视剧必须包含 IMDb/豆瓣链接

---

## 七、账号保留规则

| 条件 | 处理 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上封存账号 | 不会被删除 |
| 封存账号连续 400 天不登录 | 删除账号 |
| 未封存账号连续 150 天不登录 | 删除账号 |
| 无流量用户（上传/下载都为 0）连续 100 天不登录 | 删除账号 |
| 分享率过低降级为堕落者 | 7 天恢复期，超期封号禁 IP |

---

## 八、字幕区规则

- 允许格式：srt / ssa / ass / cue / zip / rar
- Vobsub（idx+sub）或合集须打包为 zip/rar
- 不允许 lrc 歌词或非字幕/cue 文件
- 不合格字幕扣 100 魔力值，举报奖励 50 魔力值

---

*数据来源: Playwright 采集 rules.php + upload.php (2026-04-22)*
*文档创建: 2026-04-16*
*文档更新: 2026-04-22（补充完整 rules.php 规则、Tracker URL、促销规则、魔力购买、账号保留、字幕区、upload 新增字段）*
