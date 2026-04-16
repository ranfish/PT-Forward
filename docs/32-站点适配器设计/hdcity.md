# HDCiTY (HDCity) 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | HDCiTY（高清城市） |
| 站点地址 | https://hdcity.city（多镜像：hdcity.leniter.org / hdcity.work） |
| 站点框架 | NexusPHP（深度定制，非标准） |
| 主题 | ProCity（Material Design 风格） |
| 口号 | An Advanced City For Entertainment / User is god, admin is disciple |
| 生成器 | HDCiTY Team（非标准 NexusPHP） |
| 特殊功能 | 两步上传、候选区、NoVA/NoPA/NoTA/NoXA 官组体系、字幕区、音乐流派标签 |
| 规则页面 | /rules |

---

## 一、发布页面表单字段分析

### 1.1 两步上传流程

**HDCiTY 使用独特的两步上传流程**：

**第一步**: 上传 .torrent 文件到 `/upload`（无查询参数），获得：
- `tfu`: torrent file upload hash
- `tn`: torrent name
- `f`: file count
- `s`: resource size
- `n`: ???
- `ih`: infohash
- `citeid`: citation id

**第二步**: 带参数跳转到 `/upload?tfu=...&tn=...&f=...&s=...&n=...&ih=...&citeid=0`，填写详细信息。

第二步的表单**无文件上传字段**，通过 hidden fields 传递种子信息：
```html
<input type="hidden" name="infohash" value="..." />
<input type="hidden" name="listtype" value="1" />
<input type="hidden" name="ressize" value="11649894200" />
<input type="hidden" name="filecount" value="1" />
```

**注意**: 表单 action 为空字符串（`action=""`），提交到当前 URL（含查询参数）。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `infohash` | hidden | 自动 | 种子 infohash |
| `listtype` | hidden | 自动 | 列表类型（值=1） |
| `ressize` | hidden | 自动 | 资源大小（字节） |
| `filecount` | hidden | 自动 | 文件数量 |
| `name` | text | ✓ | 种子名称（从种子文件名自动获取） |
| `bigname` | text | ✓ | **资源标题/片名**（最多64字符） |
| `small_descr` | text | - | 简短介绍（最多100字符） |
| `url` | text | - | IMDb（支持只填 tt 后的数字） |
| `posterimg` | text | - | 海报图片 URL（覆盖 IMDb 海报） |
| `descr` | textarea | ✓ | 资源介绍（BBCode） |
| `mediainfo` | textarea | - | iNFO（NFO 内容或编码信息） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

**注意**:
- `bigname` 字段是 HDCiTY 独有的"资源标题/片名"，不同于 `name`（种子名称）
- `mediainfo` 字段名非标准（其他站点通常为 `technical_info`）
- IMDb 支持只填数字（如 `1234567`），自动补全为完整 URL
- 无 `pt_gen` 字段，无 `nfo` 文件上传

### 1.3 类型字段（`type`）

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | Series/剧集 |
| 403 | Shows/节目 |
| 404 | Doc/档案记录 |
| 405 | Anim/动漫 |
| 406 | MV/音乐视频 |
| 407 | Sports/体育 |
| 408 | Audio/音频 |
| 409 | Other/其他 |
| 727 | XXX/家长指引 |
| 728 | Edu/文档/教材 |
| 729 | Soft/软件 |

**注意**: 有 XXX(727)、Edu(728)、Soft(729) 独特分类。分类名使用双语（中/英）。

### 1.4 媒介（`medium_sel`）— 13个

| 值 | 显示名称 |
|----|----------|
| 1 | BD/蓝光原盘 |
| 2 | HDDVD原盘 |
| 3 | Remux/重混流 |
| 4 | MiniBD/微蓝光 |
| 5 | HDTV/SNG/原始录制 |
| 6 | DVD原盘 |
| 7 | Encode/重编码 |
| 8 | CD/音乐/有声读物 |
| 9 | Track/外挂音轨 |
| 10 | Ebook/文档/图库 |
| 11 | Rec/视频教材 |
| 12 | Joy/游戏 |
| 13 | Prog/程序 |

**注意**: 媒介范围极广，包含 Ebook/Rec/Joy/Prog 非视频类型。有 HDDVD原盘(2) 和 MiniBD(4)。值 1-13 中跳过了部分数字（无14+）。

### 1.5 编码（`codec_sel`）— 17个（含音频编码）

**注意**: HDCiTY 的编码下拉**混合了视频和音频编码**。

| 值 | 显示名称 | 类型 |
|----|----------|------|
| 1 | H.264/AVC | 视频 |
| 13 | H.265/HEVC | 视频 |
| 4 | MPEG-2 | 视频 |
| 3 | DivX/XviD | 视频 |
| 2 | WMV/VC-1 | 视频 |
| 16 | AV1 | 视频 |
| 17 | WebM/VP | 视频 |
| 14 | WMA/WMA-LL | 音频 |
| 5 | FLAC | 音频 |
| 6 | APE | 音频 |
| 7 | DTS/DTS-ES | 音频 |
| 8 | Dolby AC3 | 音频 |
| 10 | WAV/Raw | 音频 |
| 11 | MP3/MP2 | 音频 |
| 12 | AAC/M4A | 音频 |
| 15 | TrueHD/Atmos | 音频 |
| 9 | Other | 其他 |

**注意**: 无 DTS-HD MA、DTS:X、DDP/E-AC3、LPCM。只有一个编码字段，不区分视频编码和音频编码。

### 1.6 分辨率（`standard_sel`）— 12个

| 值 | 显示名称 |
|----|----------|
| 11 | 8K-4320p |
| 10 | 8K-4320i |
| 9 | 4K-2160p |
| 8 | 4K-2160i |
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 7 | 720i |
| 6 | 540p |
| 5 | 480p |
| 4 | SD |

**注意**: 值不按分辨率排列（11,10,9,8,1,2,3,7,6,5,4）。有 8K(11/10)、720i(7)、540p(6)。

### 1.7 处理（`processing_sel`）— 3D 类型

| 值 | 显示名称 |
|----|----------|
| 1 | 3D H-OU/上下半宽 |
| 2 | 3D H-SBS/左右半宽 |
| 3 | 3D Interleaved/交织 |
| 4 | 3D Red-blue/红蓝 |
| 5 | 3D Alt/其他3D |

**注意**: processing_sel 用于 3D 类型而非地区！

### 1.8 制作组（`team_sel`）— 6个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | HDCITY-NoVA | 官组（Video/Audio） |
| 14 | HDCITY-NoPA | 官组（PAD/Mobile） |
| 15 | HDCITY-NoTA | 官组（TV/SNG） |
| 17 | HDCITY-NoXA | 官组（未知） |
| 9 | 0DAY | Scene 0day |
| 0 | （不选） | |

**注意**: 只有官组 + 0DAY 选项，无 Other。非官组成员不得使用 HDCITY 前缀。

### 1.9 标签（`tag1ing` + `tag2ing`）— 两个下拉

**标签分两个下拉框**，value 为**中英文字符串**（非数字ID）。

#### tag1ing — 影视题材标签

动作/Action, 科幻/Sci-Fi, 爱情/Romance, 剧情/Drama, 喜剧/Comedy, 家庭/Family, 伦理/Ethics, 惊悚/Thriller, 文艺/Artistic, 冒险/Adventure, 魔幻/Fantasy, 灾难/Disaster, 犯罪/Crime, 悬疑/Mystery, 记录/Documentary, 战争/War, 历史/History, 传记/Biography, 歌舞/Music, 动画/Animation, 西部/Western, 古装/Costume, 武侠/仙侠, 运动/Sports, 微电影/MiniVideo, 少儿不宜/Adult, Cult

#### tag2ing — 音乐流派标签（在 "-- Genres --" 分隔符后）

Ambient/氛围, Anime/动漫, Ballad/浪漫, Bass/贝斯, Beat/打击乐, Blues/蓝调, ChineseStyle/古风, Chorus/合唱, Classic Rock/古典摇滚, Classical/古典, Country/乡村乐, Dance/舞曲, Disco/迪斯科, Electronic/电子乐, Folk/民谣, Heavy Metal/重金属, Hip-Hop/嘻哈, Instrument/器乐, Jazz/爵士乐, Latin/拉丁, Meditative/冥想, Metal/金属乐, Musical/音乐剧, New Age/新世纪, Opera/歌剧, Pop/流行, Porn Groove/情色, ProgressiveRock/前卫摇滚, Psychadelic/迷幻, PsychedelicRock/迷幻摇滚, Punk/朋克, Punk Rock/朋克摇滚, Pure Music/纯音乐, R&B/节奏蓝调, Rap/说唱, Rave/锐舞, Retro/怀旧, Revival/复兴, RhythmicSoul/节奏灵魂乐, Rock/摇滚, Rock&Roll/摇滚, Samba/森巴, Slow Rock/慢摇, Soul/灵魂乐, Symphony/交响乐, Synth/合成器, Techno/数码, Tribal/部落

### 1.10 字幕情况 — 4个 checkbox

| 字段名 | value | 含义 |
|--------|-------|------|
| `sub_soft` | 1 | 软字幕 |
| `sub_hard` | 1 | 硬字幕 |
| `sub_incl` | 1 | 种子含字幕 |
| `chnize` | 1 | 中字 |

---

## 二、发种规则（rules.php）

### 2.1 上传总则

- 上传者必须对文件拥有合法传播权
- 做种时间达到 24 小时且有至少 3 个做种者后方可撤种
- 违规种子不经提醒直接删除

### 2.2 上传者资格

- 任何人都能上传
- 堕落天使(Fallen Angel)发种前需先经过候选区

### 2.3 允许的资源

- HD 视频文件（BD/HD DVD 原盘、HDTV 录制、高清重编码）
- SD 视频文件（DVDR、高清来源的重编码 480p/MiniSD；高清/MiniSD 已存在时禁止 DVDRip）
- 无损/多声道音乐（FLAC、DTS、APE 等）
- 评论/配音/原声音轨
- 提前的高清预告片、游戏/多媒体预告片/CG 宣传片
- 中大型软件、操作系统、资料、文献/文档、有声读物

### 2.4 不允许的资源

- Dupe 版本
- RealVideo（RMVB/RM）
- CAM/TS/TC/SCR/R5 等低画质（有更高分辨率后禁止）
- flv/3gp/asf 等低质量格式
- 同分辨率已有高码率版本时的 0day/枪版
- 无 CUE 的 CD 映像
- 无关 txt/srt/url/.torrent 文件
- 政治/不适/病毒/垃圾内容

### 2.5 Dupe 规则

- DVD5 大小的重编码版本永远允许
- 新版本可发布条件：
  - 旧版本连续断种 7 日以上
  - 新版本无旧版错误/画质更好
  - 旧版本已发布 18 个月以上
- 不同区域/配音/字幕的 BD/HD DVD 不视为 Dupe

### 2.6 标题命名规范

- 电影：`英文名称 [剪辑版本] [年份] [发布说明] 分辨率 来源 视频编码 [音频编码]-制作者`
- 电视剧：`英文名称 [剪辑版本] S**E** [年份] [发布说明] 分辨率 来源 视频编码 [音频编码]-制作者`
- 音乐：`艺术家名 - 专辑名 [版本] 年份 [发布说明] 音频编码-[制作者]`

副标题：中文名称或其他说明。

### 2.7 制作组规则

非 HDCITY 工作组成员**不得使用 HDCITY- 前缀**。0day 资源可选择 0DAY，不确定则不选。

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "Movies": 401,
  "Series": 402,
  "Shows": 403,
  "Doc": 404,
  "Anim": 405,
  "MV": 406,
  "Sports": 407,
  "Audio": 408,
  "Other": 409,
  "XXX": 727,
  "Edu": 728,
  "Soft": 729
}
```

### 3.2 媒介（`medium_sel`）

```json
{
  "BD": 1,
  "HDDVD": 2,
  "Remux": 3,
  "MiniBD": 4,
  "HDTV": 5,
  "DVD": 6,
  "Encode": 7,
  "CD": 8,
  "Track": 9,
  "Ebook": 10,
  "Rec": 11,
  "Joy": 12,
  "Prog": 13
}
```

### 3.3 编码（`codec_sel`）— 视频+音频混合

```json
{
  "H.264/AVC": 1,
  "H.265/HEVC": 13,
  "MPEG-2": 4,
  "DivX/XviD": 3,
  "WMV/VC-1": 2,
  "AV1": 16,
  "WebM/VP": 17,
  "WMA/WMA-LL": 14,
  "FLAC": 5,
  "APE": 6,
  "DTS/DTS-ES": 7,
  "Dolby AC3": 8,
  "WAV/Raw": 10,
  "MP3/MP2": 11,
  "AAC/M4A": 12,
  "TrueHD/Atmos": 15,
  "Other": 9
}
```

### 3.4 分辨率（`standard_sel`）

```json
{
  "8K-4320p": 11,
  "8K-4320i": 10,
  "4K-2160p": 9,
  "4K-2160i": 8,
  "1080p": 1,
  "1080i": 2,
  "720p": 3,
  "720i": 7,
  "540p": 6,
  "480p": 5,
  "SD": 4
}
```

### 3.5 处理/3D（`processing_sel`）

```json
{
  "3D H-OU": 1,
  "3D H-SBS": 2,
  "3D Interleaved": 3,
  "3D Red-blue": 4,
  "3D Alt": 5
}
```

### 3.6 制作组（`team_sel`）

```json
{
  "HDCITY-NoVA": 1,
  "HDCITY-NoPA": 14,
  "HDCITY-NoTA": 15,
  "HDCITY-NoXA": 17,
  "0DAY": 9
}
```

### 3.7 标签

标签 value 为字符串（非数字），通过两个下拉框选择。

### 3.8 字幕

```json
{
  "软字幕": {"field": "sub_soft", "value": "1"},
  "硬字幕": {"field": "sub_hard", "value": "1"},
  "种子含字幕": {"field": "sub_incl", "value": "1"},
  "中字": {"field": "chnize", "value": "1"}
}
```

---

## 四、HDCiTY 特殊注意事项

### 4.1 两步上传流程

HDCiTY 是目前采集的唯一一个需要**先上传 .torrent 文件再填写信息**的站点。适配器必须实现：
1. POST .torrent 文件到 `/upload`
2. 解析重定向 URL 中的参数（tfu/tn/f/s/n/ih/citeid）
3. 携带参数 GET `/upload?...`
4. 填写表单并 POST

### 4.2 表单 action 为空

表单 `action=""`，提交时 POST 到当前 URL（含所有查询参数）。

### 4.3 无 file 字段

第二步表单中无文件上传字段，种子信息通过 hidden fields 传递。

### 4.4 编码字段混合视频和音频

`codec_sel` 混合了视频编码（H.264/H.265/AV1 等）和音频编码（FLAC/DTS/AC3 等），适配器需根据资源类型选择正确的编码。

### 4.5 processing_sel 是 3D 类型

不同于大多数站点的"地区"处理字段，HDCiTY 的 processing_sel 用于 3D 类型选择。

### 4.6 bigname 独有字段

`bigname` 是 HDCiTY 独有的"资源标题/片名"字段，与 `name`（种子名称）不同。

### 4.7 标签使用字符串 value

标签的 value 是中英文字符串（如"动作/Action"），非数字 ID。

### 4.8 制作组命名限制

非 HDCITY 工作组成员不得使用 HDCITY- 前缀。转载资源需注意去除或修改标题中的 HDCITY 前缀。

### 4.9 URL 路径非标准

HDCiTY 使用简短路径（`/pt` 而非 `torrents.php`，`/upload` 而非 `upload.php`），无 `.php` 后缀。

### 4.10 多镜像域名

站点有多个镜像域名（hdcity.city / hdcity.leniter.org / hdcity.work），适配器需支持多域名。

### 4.11 种子名称含中文

从示例看，种子名称（`name`）可以包含中文（如 `[追恶]`），不像 CHDBits 严格要求不含中文。

---

## 五、适配器实现要点

### 5.1 两步上传实现

```go
// Step 1: Upload torrent file
resp1 := POST("/upload", multipart{
    "torrent": torrentFile,
})
// Parse redirect URL or response for tfu/tn/ih etc.

// Step 2: GET form page with params
uploadURL := fmt.Sprintf("/upload?tfu=%s&tn=%s&f=%d&s=%d&n=%d&ih=%s&citeid=0", ...)

// Step 3: Fill and POST form
resp2 := POST(uploadURL, form{
    "infohash":  ih,
    "listtype":  "1",
    "ressize":   fmt.Sprintf("%d", size),
    "filecount": fmt.Sprintf("%d", count),
    "name":      title,
    "bigname":   bigName,
    // ... other fields
})
```

### 5.2 字段命名

```go
BigNameField:     "bigname",      // 独有字段
MediainfoField:   "mediainfo",    // 非 technical_info
PosterImgField:   "posterimg",    // 独有字段
Tag1Field:        "tag1ing",      // 影视题材
Tag2Field:        "tag2ing",      // 音乐流派
SubSoftField:     "sub_soft",
SubHardField:     "sub_hard",
SubInclField:     "sub_incl",
ChnizeField:      "chnize",
InfohashField:    "infohash",     // hidden
ListtypeField:    "listtype",     // hidden
RessizeField:     "ressize",     // hidden
FilecountField:   "filecount",   // hidden
```

### 5.3 编码选择逻辑

需根据媒介类型选择编码：
- 视频（BD/Remux/Encode/HDTV 等）→ 选择视频编码
- 音频（CD/Track）→ 选择音频编码

---

## 六、与其他 NexusPHP 站点对比

| 特征 | HDCiTY | 常见 NexusPHP |
|------|--------|---------------|
| 上传流程 | 两步上传 | 单步 |
| 表单 action | 空（含查询参数） | takeupload.php |
| 种子文件 | hidden fields（infohash） | file input |
| URL 风格 | 简短路径（/pt, /upload） | .php 后缀 |
| 大标题字段 | `bigname`（独有） | 无 |
| 海报字段 | `posterimg`（独有） | 无 |
| 编码字段 | 视频+音频混合（17个） | 通常分开 |
| 处理字段 | 3D 类型 | 通常为地区 |
| 制作组 | 官组体系（NoVA/NoPA/NoTA/NoXA） | 通常更多样 |
| 标签 | 两个下拉，字符串 value | checkbox 数组 |
| 字幕 | 4个独立 checkbox | 无或包含在标签中 |
| 分类 | 含 XXX/Edu/Soft | 通常无 |
| 媒介 | 13个（含 Ebook/Rec/Joy/Prog） | 通常 6-10 |
| 分辨率 | 含 8K/540p/SD | 通常 5-7 |
| 音乐流派 | 有（46种） | 通常无 |
| IMDb | 支持只填数字 | 通常需完整 URL |

---

*数据来源: upload.php HTML + rules.php HTML (2026-04-16)*
*文档创建: 2026-04-16*
