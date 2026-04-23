# 青蛙 站点适配器设计

> 青蛙（QingWAP）站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 青蛙|
| 站点地址 | https://www.qingwapt.com |
| 站点框架 | NexusPHP |
| 特殊规则 | 三套规范性指南 |

## 核心规范

青蛙站点有三套完整的规范性指南，对发布有严格要求：

1. **视频标题命名规范 v1.05**
2. **青蛙发种规范 v1.02**
3. **转种自查指南 v0.1**

### 1. 视频标题命名规范（0day 命名法）

#### 基本格式

```
剧名 季集信息 年份 其他信息 分辨率 地区码 内容分发方 片源类型 规格 HDR类型 bit信息 视频编码 音频编码 声道数 对象信息 音轨数-制作组
```

#### 示例

```
The Last 10 Years 2022 1080p BluRay x265 DD 5.1-CMCT
（其他信息为空；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x265 不写 bit 信息）

Sidonia no Kishi 2014-2015 1080p BluRay Hi10P x264 FLAC 2.0-VCB-Studio
（其他信息为空；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x264 10bit 要写为"Hi10P x264"）

Clannad S01-S02+MOVIE REPACK 1080p / 480p BluRay / DVDRip Hi10P x264 FLAC 5.1-mawen1250&fch1993
（其他信息为 REPACK；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x264 10bit 要写为"Hi10P x264"）
（此为多季合集有 DVD 有 BD，多个分辨率压制的情况）

Saki S01-S03 BluRay 1080p / 720p Hi10P x264 FLAC 2.0-VCB-Studio
（无年份可以省略；由于是 Encode，因此规格为空；非 HDR 不写 HDR 类型；x264 10bit 要写为"Hi10P x264"）
```

#### 标题各组成部分详细规则

##### 剧名

- 主标题推荐使用英文名（可从 IMDB 或豆瓣又名中获取）
- 动画可使用 MyAnimeList 的罗马音标题
- 别名与中文名填入副标题

##### 季集信息

- 单季合集：S01（即使只有1季也必须写）
- 多季合集：S01-S05
- 分集：S01E03 或 S01E03-E05
- 每季剧名后缀不同时写为 `XXX Series`
- 完结剧集**不要**在标题写 Complete，请**勾选完结**标签

##### 年份

- 有重名时必须填写
- 无重名时可省略
- 多季合集填写 `最早年份-最晚年份`（如 2015-2022）
- 单集综艺/体育节目填写完整年月日（如 20240326）

##### 其他信息（按优先级检查，属于"不一定有"的项目）

| 优先级 | 项目 | 说明 |
|--------|------|------|
| 1 | 剪辑版本 | Director's Cut、Uncensored、Extended、Unrated、Uncut 等（需官方盖章） |
| 2 | 2in1 | 同时含2个剪辑版本（如影院版+导演剪辑版），3in1 同理 |
| 3 | 版本 | 20th Anniversary Edition、Remaster、4K Remaster、Limited Edition |
| 4 | 特殊比例 | IMAX、Open Matte、MAR |
| 5 | Hybrid | 多源混合（如添加其他源的 DV 层信息） |
| 6 | REPACK/V2 | 文件有改动但主要视频未重新转码（增补CD、扫图等） |
| 7 | RERIP | 主要视频文件被重新转码 |
| 8 | PROPER | 原盘抓取有问题重新发布（仅原盘资源） |
| 9 | MiniBD | CMCT 特色资源 |

##### 分辨率

从 MediaInfo 的 Video - Height 和 Video - Scan type 组合：

| 分辨率 | 说明 |
|--------|------|
| 4320p | 8K |
| 2160p | 4K |
| 1440p | 2K |
| 1080p | Height=1080, Progressive |
| 1080i | Height=1080, Interlaced |
| 720p | Height=720 |
| 576p/576i | PAL |
| 480p/480i | NTSC |
| SD | DVD 类可由制式（NTSC/PAL）代替 |

注意：不要将 p 写成 P，不要使用 4k/2k 字样。黑边裁切导致非标准分辨率（如716p）时可写720p或实际值。

##### 地区码（仅原盘类）

ITA、USA、JPN、HKG、TWN 等，无法判定时可省略。

##### 内容分发方

- 原盘：标准收藏（CC）、电影大师（MOC）、华纳档案馆（MAC）等
- WEB-DL：流媒体厂商缩写（见 wiki 流媒体缩写页）
- HDTV：电视台缩写（无法找到来源时可省略）

##### 片源类型

**原盘类**：Blu-ray / 3D Blu-ray / UHD Blu-ray / Modded Blu-ray / Custom BluRay / NTSC DVD5 / NTSC DVD9 / PAL DVD5 / PAL DVD9 / HD DVD

**压制类**：BluRay / 3D BluRay / UHD BluRay / DVDRip / HDDVDRip

注意：原盘为 `Blu-ray`（带连字符），压制为 `BluRay`（无连字符）。

**WEB类**：省略此项，填写流媒体厂商缩写名。
**HDTV类**：填写电视台名称，无法找到来源时可省略。

##### 规格

Remux / WEB-DL / WEBRip / HDTV / UHDTV / HOU / HSBS

原盘类（Blu-ray/DVD/CD）此项不填。

##### HDR类型

| 类型 | 说明 |
|------|------|
| HDR | 通用 HDR |
| HDR10+ | 动态 HDR |
| DV | Dolby Vision（也可写作 DoVi） |
| DV HDR | Dolby Vision + HDR10 |
| DV HDR10+ | Dolby Vision + HDR10+ |
| HLG | Hybrid Log-Gamma |
| PQ10 | PQ HDR |
| HDR vivid | 国产 HDR 标准 |

SDR 资源不填此项。从 MediaInfo 的 Video - HDR Format 字段判断。

##### bit 信息

仅当 Format=AVC 且 Bit depth=10 bit 时写 `Hi10P`。

常见错误：x265 10bit 只写 x265 即可（不写10bit）；x264 10bit 必须写为 `Hi10P x264`。

##### 视频编码（按媒介严格区分）

**蓝光原盘/REMUX**：AVC / HEVC / MPEG-2 / VC-1
**WEB**：H.264 / H.265 / MPEG-2 / VC-1 / VP9 / AVS+ / AVS3 / AV1
**HDTV**：H264 / H265 / MPEG2 / VP9 / AVS+ / AVS3 / AV1
**压制(Encode)**：x264 / x265 / AV1 / MPEG-2 / VP9

注意：
- 原盘用 AVC/HEVC，压制用 x264/x265，**严禁混用**
- AVC = H.264，HEVC = H.265
- 如果 MediaInfo 的 Writing library 明确有 x264/x265，WEB/HDTV 也可以写 x264/x265

##### 音频编码和声道数

音频编码查看 MediaInfo 的 Audio - Format，多音轨仅标最高规格。

声道数查看 Audio - Channel layout，L/R 有几个单词就是几个声道，LFE 为 0.1 个。

| 标题写法 | 对应格式 |
|----------|----------|
| DD | Dolby Digital (AC3) |
| DDP | Dolby Digital Plus (E-AC3, DD+) |
| DTS | DTS |
| DTS-HD MA | DTS-HD Master Audio |
| DTS-HD HR | DTS-HD High Resolution |
| DTS:X | DTS:X（标题中不需要额外标注） |
| TrueHD | Dolby TrueHD |
| Atmos | Dolby Atmos（基于对象的，写在声道数前） |
| FLAC | FLAC |
| LPCM | LPCM |
| AAC | AAC（2.0 时可省略声道数） |
| MP2/MP3 | MP2/MP3（2.0 时可省略声道数） |
| APE | APE |
| WAV | WAV |

**常见错误**：
- DD 和 DDP 搞混：Dolby Digital Plus = DDP = DD+ = E-AC3；Dolby Digital = DD = AC3
- 标题中不应写作 AC3/E-AC3，应写作 DD/DDP
- AAC/MP2/MP3 且声道为 2.0 时可省略声道数
- Atmos 写在声道数之后（如 `TrueHD Atmos 7.1`）
- DTS:X 不需额外标注
- 评论音轨不计入音轨数

##### 对象信息

Atmos / Auro3D（DTS:X 不需要标注）。省略则视为不存在。

##### 音轨数

正片有多条音轨时写 XAudio（X为数字），评论音轨不计入。仅对压制(Encode)类强制要求。

##### 制作组

- 本站首发：`-你的名字@QingWa`
- 多站官组转种：`-组名`（如 `-VCB-Studio`）
- 个人用户他站转种：`-发布者@原站名`
- 匿名用户他站转种：`-Anonymous@原站名`
- 来源不明：留空或 `-NOGROUP` / `-NoGroup` / `-NoGrp`
- Scene 资源：可省略

#### MediaInfo 生成

1. 打开 MediaInfoOnline（https://mediaarea.net/MediaInfoOnline）
2. 拖入视频文件（多集选任意一集，建议第一集）
3. 点击 **Copy to clipboard** 复制到 MediaInfo 栏

**蓝光原盘请使用 BDInfo 而非 MediaInfo**（https://www.videohelp.com/software/BDInfo）：
1. 打开 BDInfo.exe
2. 点击 Browse 选择蓝光文件夹
3. 勾选主视频 MPLS（一般是最长的）
4. 点击 Scan Bitrates → View Report
5. 复制 quick summary 部分填入

注意：转种时不推荐使用原贴中的 MediaInfo（可能被简化或为中文）。

#### 自查助手

推荐安装油猴插件 [不可蛙-种审/发种自查助手](https://greasyfork.org/zh-CN/scripts/490095-qingwa-torrent-assistant)，用于错误自查，减少种子出错风险。

### 2. 发种规范

#### 重复判定总则

- **完结资源代替分集资源** — 已有完结的种子禁止发布单集资源
- **跨季资源代替相同来源和品质的单季资源** — S01-S08 代替 S01-S07
- **跨季资源代替季数被覆盖的跨季资源**
- **高清资源代替完全重复的低清资源**
- **各个压制版允许共存**
- **压制版与原盘允许共存**
- **各个 DIY 原盘资源允许共存**
- **蓝光资源代替更低清晰度且完全重复的 WEB/HDTV 资源**

**特殊情况**：
- 动漫区如果蓝光对 TV 版作出了极大幅度的修正，TV 资源将予以保留
- 某些上古旧番的高清版仅有 WEB-DL，此情况下 WEB-DL 也将和蓝光或 DVD 一起保留
- **动漫区额外规定**：已发售并放流蓝光盘的日本动画，不允许再发布 WEB-DL 和 WEBRip 资源
- **WEB-DL 特殊规则**：同一个影视资源，在有官种（官方首发）的情况下，将不接受分辨率/质量相同或更低的 WEB-DL 种子资源

#### 合集打包规则

- **电影类资源** — 只允许发行商官方原盘合集
  - 目前允许在此基础上的衍生品（DIY、Remux 和 Encode）
  - 发布发行商官方原盘合集及其衍生资源建议简介区域加上蓝光合集的封面
- **严禁私人合集** — 诸如导演、演员、IMDb Top 250、豆瓣 Top 250 等
  - 违反此条可能给予处罚
- **禁止跨季不同分辨率资源打包发布**
  - VCB 资源豁免此条

#### 转种总则

- **当然不准转载禁转种**
- **已有完结的种子禁止发布单集资源**
- **禁止转载发布未完结分集** — 未完结剧集只接受增量包，不接受分集转载
- **禁止转载超分处理/补帧资源**
- **禁止转载黑名单制作组的资源**
- **不建议转载找不到出处的资源** — 可以加 -NoGroup 或者 -NoGrp，但请自担责任
- **不建议转载灰名单制作组的资源**
- **禁止转载简易机翻资源**
- **请直接上传原种** — 以便辅种
- **编辑需要** — 如果确有编辑需要，请在简介中明确注明增补内容
  - 注意，"添加了字幕""修正了不规范的名字"并不是合理的"编辑需要"
- **标题和副标题需符合本站规范** — 即使是转载，标题也应修改成本站对应的格式
- **务必写明种子出处** — 实在找不到的也不强求

**重要补充规则**：
- **本站禁止发布剧集的分集**（PT-Forward 适配器层面强制执行，即使 Wiki 标签规则说"需要申请"可使用分集标签，自动转发不应涉及分集资源）
- 分集标签(18)仅限手动申请通过后使用，自动转发流程中**不得勾选**
- 建议把预览图下载到自己电脑重新上传到网站附件或图床
- 源站简介如果无法达到本站对应分区的简介标准，则按要求补充简介

原出处简介推荐用 quote 标签括起来：
```
[quote]资源简介[/quote]
```

#### 制种总则

- **合法传播权** — 你需要对上传的文件拥有合法的传播权（免责用的废话，别在意）
- **禁止发布超分处理/补帧资源**
- **违规但有价值的资源** — 如果有一些违规但却有价值的资源，请将详细情况告知管理组，我们可能破例允许其发布
- **制种时请不要加入** — 广告文件、病毒、木马、种子中种子、无关文件
- **违法资源** — 不允许发布包括但不限于涉及暴恐、肢解、虐待、色情、政治的违法资源
- **文件名和目录名** — 不建议过长，请控制在 100 中文/200 英文之内
- **特殊符号** — 文件和文件夹名字里不可包含特殊符号（比如斜杠和单双引号）
- **点号** — 点号（.）不可出现在文件或文件夹末尾（Windows 的祖传 bug）
- **分块大小** — 种子的分块大小请控制在 16MB 以下
  - 过大的分块会让某些旧软件无法读取，而且会很卡
- **文件夹包装** — 即使发布的是单文件，也仍然推荐套一层文件夹，将文件夹制种
- **文件夹名字** — 文件夹的名字请好好取，不要用"新建文件夹""新增文件夹"这类摆烂名

### 3. 文件规范

- **分块大小** ≤ 16MB
- **文件名/目录名长度** ≤ 100 中文/200 英文
- **特殊符号** — 不可包含（斜杠、引号等）
- **点号结尾** — 点号不能出现在文件/文件夹末尾

### 3.5 压缩包规则

- **禁止使用非常见压缩格式**（以标准版 7-zip 能解压为限，含 zstd）
- **禁止带密码**
- **禁止"假装是 zip"的格式**（快压/好压等）
- 不推荐 RAR5 格式（兼容性问题）
- 不推荐 zip 格式（unicode 支持不好）
- 超过 4GB 压缩包建议分卷（非硬性要求）
- RAR 恢复记录设置在 **5% 以下**

### 3.6 做种规则

- 上传者必须实际拥有上传文件（可存本地或盒子）
- **本站对盒子暂无任何限制，也无需报备**
- **最小上传速度 300KB/s**，故意低速将警告/封禁
- **免费期 48 小时**内不可撤种；完成数 ≥ 3 后方可撤种
- 基本出种前（做种数 ≥ 3 并持续 **6 小时**），非做种状态不超过 24 小时
- 发布后 **7 天内无人下载**，允许撤种
- **禁止将视频打压缩包发布**
- 软件类资源请打压缩包发布（尤其小文件多的）

### 4. 自查流程

1. 安装站内大佬写的自查插件
2. **允许的资源** — 检查是否为允许的资源类型
3. **种子文件** — 检查种子文件格式
4. **主标题** — 检查主标题是否符合规范
5. **副标题 & IMDB & PT-Gen & NFO** — 检查副标题和元数据
6. **简介** — 检查简介内容
7. **MediaInfo** — 检查 MediaInfo 信息
8. **类型 & 质量 & 标签** — 检查类型、质量和标签

### 5.5 转种自查指南（Forum 197）

**来源**: topicid=197（转种自查指南v0.1，作者 playboy）

#### 检查一：允许的资源
- 官种转种（非禁转）大部分可直接转
- BT/网盘资源一般不被允许（稀缺资源可 PM 管理组）
- **跨季打包和多分辨率打包不被允许**
- 已完结剧集**禁止发布单集**
- 未完结剧集从第一集开始发
- **动漫区额外规定**：已发售并放流蓝光盘的，不允许再发布 WEB-DL 和 WEBRip 资源（适用于日本动画）

#### 检查二：种子文件
- 可直接上传源站点种子，站点会自动洗种

#### 检查三：主标题
- 按标题规范重新修改

#### 检查四：副标题 & IMDb & PT-Gen & NFO
- 前两项（副标题/IMDb）复制源站点
- NFO 不需要填写

#### 检查五：简介（错误最多）
- **顺序**：感谢词 → 说明 → PT-Gen 内容（海报→基本信息→简介）→ 截图
- 感谢词用 `[quote]` 框起来
- 说明单独用 `[quote]` 框起来，位于感谢词后海报前
- VCB 资源：感谢词替代为官网作品链接，说明替代为官网作品说明
- 截图确保未裂，至少一张，推荐 PNG 原图
- 推荐图床：pixhost / imgbox / ImgBB / freeimage
- 多余内容删除（如源站广告）
- **MediaInfo 不要放在简介里**（删除并移至专属栏位）

#### 检查六：MediaInfo
- 压制资源：重新生成 MediaInfo（不推荐引用源站的，可能简化或中文导致排版异常）
- 原盘资源：使用 BDInfo 生成
- 留空将百分百被打回

#### 检查七：类型 & 质量 & 标签
- 类型选对应资源（**动漫电影也算动漫**）
- 媒介辨别：
  - UHD Blu-ray：来源 UHD 且编码 HEVC/AVC，分辨率一般 4K
  - Blu-ray：来源 Blu-ray 且编码 AVC，分辨率一般 1080p
  - Remux：标题含 Remux
  - WEB-DL：标题含 WEB-DL
  - Encode：来源 BluRay 且编码 x264/x265

### 5.6 流媒体厂商缩写名（Forum 208 附件1）

**来源**: topicid=208 附件1（~170 个厂商）

常见缩写：

| 缩写 | 厂商 |
|------|------|
| AMZN | Amazon/Prime Video |
| NF | Netflix |
| DSNP | Disney+ |
| HMAX | HBO Max |
| HBO | HBO |
| ATVP | Apple TV+ |
| HULU | Hulu |
| PCOK | Peacock |
| PMTP | Paramount+ |
| CR | Crunchyroll |
| iT | iTunes |
| iP | BBC iPlayer |
| DSCP | Discovery+ |
| MAX | Max（原 HBO Max 更名） |
| STAR | Disney+ Hotstar |
| MY5 | Channel 5 |
| CRAV | Crave |
| HIDI | Hidive |
| CC | Criterion Channel |
| MUBI | Mubi |
| STAN | Stan |
| BFI | BFI Player |
| ALL4 | Channel 4 |
| RED | YouTube Premium/Red |

> 完整列表见 Forum 208 附件1。适配器解析标题时需匹配这些缩写来识别 WEB 来源。

#### 所有视频分区

- 压制资源必须提供完整 MediaInfo（推荐 MediaInfoOnline）
- 多集资源提供任意一集的 MediaInfo 即可
- 蓝光原盘必须提供 BDInfo（使用 quick summary）
- 正确选择媒介、视频编码、音频编码、分辨率、制作组
- 副标题可包含：片名中译（建议用豆瓣名）、片名原名、包含内容等
- 简介要求：资源引用 + 海报 + 介绍（可用 PTGen）+ **至少三张视频截图**
- **简介中禁止包含 MediaInfo**（原种有也需删除）
- 禁止发布过短视频（含抖音短视频合集，短剧除外）

#### 音乐区（暂不审核）

- 禁止发布有损音乐
- 允许合集类资源（歌手/公司/组合/社团/作曲者合集），禁止个人精选
- 标题包含码率、作者、专辑名即可
- 简介需有预览图（如专辑封面）
- Log/频谱图至少包含一项（优先 Log）

#### 其他区（暂不审核）

- 游戏、软件、电子书等资源
- 转种可直接使用原标题
- 软件/游戏务必带版本号，零碎文件打成压缩包
- 严禁夹带私货

### 发布页面分析（2026-04-22 Playwright 实际采集）

> 用户 ranfish 已登录，页面标题 `青蛙 :: 发布 - Powered by NexusPHP`，表单共 49 个元素。

#### 基本信息

| 项目 | 内容 |
|------|------|
| 表单地址 | https://www.qingwapt.com/takeupload.php |
| 表单方法 | POST multipart/form-data |
| 站点框架 | NexusPHP |
| Tracker URL | https://tracker.qingwapt.com/announce.php |

#### 核心字段（8 个数据字段）

| 字段 name | 类型 | 必填 | 说明 |
|-----------|------|------|------|
| `file` | file | 是 | 种子文件（id=torrent） |
| `name` | text | 是 | 主标题（id=name，必须符合 0day 命名规范） |
| `small_descr` | text | 否 | 副标题（建议填写，审核脚本检查为空报错） |
| `url` | text | 否 | IMDb 链接 |
| `pt_gen` | text | 否 | PT-Gen 链接（支持 imdb/douban/tmdb/bangumi/indienova） |
| `nfo` | file | 否 | NFO 文件上传 |
| `descr` | textarea | 是 | 简介正文（id=descr，BBCode 编辑器） |
| `technical_info` | textarea | 否 | **MediaInfo/BDInfo 独立栏位**（无 id，与 descr 分离） |

> **重要发现**：`technical_info` 是独立于 `descr` 的 textarea，用于填写 MediaInfo 或 BDInfo。审核脚本检查此栏位为空或不含正确格式时报错。这比猫站（MediaInfo 放在 descr 的 hide 标签中）更清晰。

#### 下拉选择字段（6 个 + 3 个格式字段）

**分类 type（10 个选项）**：

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 405 | 动漫 |
| 404 | 纪录片 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音乐 |
| 412 | 短剧 |
| 409 | 其他 |

> 注意：403=综艺，405=动漫，与猫站 403=动画、405=综艺 **不同**。青蛙 402=剧集、404=纪录片，猫站 402=纪录片、404=电视剧。

**媒介 source_sel[4]（11 个选项）**：

| ID | 名称 |
|----|------|
| 1 | UHD Blu-ray |
| 8 | Blu-ray |
| 9 | Remux |
| 10 | Encode |
| 7 | WEB-DL |
| 4 | HDTV |
| 2 | DVD |
| 3 | CD |
| 11 | MiniBD |
| 5 | Track |
| 6 | Other |

> **注意**：字段名带 `[4]` 后缀（如 `source_sel[4]`），是 NexusPHP 动态字段——`[4]` 表示当前默认分类（电影）。切换分类时后缀会变化。

**视频编码 codec_sel[4]（8 个选项）**：

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 6 | H.265/HEVC |
| 2 | VC-1 |
| 4 | MPEG-2 |
| 7 | AV1 |
| 3 | MPEG-4 |
| 8 | VP9 |
| 5 | Other |

**音频编码 audiocodec_sel[4]（18 个选项）**：

| ID | 名称 |
|----|------|
| 9 | DTS:X |
| 14 | DTS |
| 10 | DTS-HD MA |
| 21 | DTS-HD HRA |
| 11 | TrueHD Atmos |
| 12 | TrueHD |
| 13 | LPCM |
| 15 | DD/AC3 |
| 16 | DDP/E-AC3 |
| 1 | FLAC |
| 17 | AAC |
| 18 | APE |
| 19 | WAV |
| 4 | MP3 |
| 8 | M4A |
| 20 | OPUS |
| 22 | AV3A |
| 7 | Other |

**分辨率 standard_sel[4]（8 个选项）**：

| ID | 名称 |
|----|------|
| 6 | 8K/4320p |
| 7 | 4K/2160p |
| 8 | 2K/1440p |
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | Other |

**制作组 team_sel[4]（5 个选项）**：

| ID | 名称 |
|----|------|
| 6 | FROG |
| 7 | FROGE |
| 8 | FROGWeb |
| 10 | CatEDU |
| 5 | Other |

### 官组后缀

FROG / FROGE / FROGWeb

> 注意：审核脚本中提到 GodDramas（id=9），但发布页面制作组选项中**没有** GodDramas。标题含 `frog`/`froge`/`frogweb` → `officialSeed=true`。

#### 标签 tags[4][]（16 个 checkbox）

| value | 标签名 | 对应 Wiki 标签 |
|-------|--------|---------------|
| 2 | VCB-Studio | VCB-Studio 小组作品 |
| 17 | 儿童片 | 适合儿童观看 |
| 19 | LGBTQ+ | — |
| 1 | 禁转 | 不希望被转载 |
| 6 | 中字 | 内嵌/内封/外挂中文字幕 |
| 20 | 特效字幕 | 带特效字幕 |
| 5 | 国语 | 含国语（普通话）音轨 |
| 8 | 粤语 | 含粤语音轨 |
| 14 | 完结 | 完结剧集 |
| 10 | 系列合集 | 多季/系列电影打包 |
| 11 | 原生原盘 | 未经修改的原盘 |
| 4 | DIY | DIY 资源 |
| 15 | 杜比视界 | 含杜比视界 |
| 12 | HDR | 含 HDR10 |
| 13 | HDR10+ | 含 HDR10+（须同时选 HDR） |
| 7 | Remux | Remux 资源 |

> Wiki 标签页列出的"官方"、"驻站"、"零魔"标签不在 checkbox 中（系统自动判断）。

#### 其他字段

| 字段 | 说明 |
|------|------|
| `uplver` | 匿名发布 checkbox（value=yes） |
| `color` | 标题颜色下拉（40 色） |
| `font` | 标题字体下拉（20 种） |
| `size` | 标题字号下拉（1-7） |
| `tagcount` | BBCode 编辑器"关闭所有标签"按钮 |
| `b/i/u/url/IMG/list/quote/md` | BBCode 编辑器工具栏按钮 |

#### 与猫站发布页的关键差异

| 对比项 | 猫站 (PTerClub) | 青蛙 (QingWaPT) |
|--------|-----------------|-----------------|
| MediaInfo | 放在 `descr` 的 `[hide=MediaInfo]` 中 | **独立 `technical_info` 字段** |
| 豆瓣链接 | 独立 `douban` 字段 | 无独立字段，用 `pt_gen` |
| 地区选择 | `team_sel`（大陆/香港/台湾/欧美/韩国/日本/印度/其他） | **无地区选择字段** |
| 质量/媒介 | `source_sel`（15 选项，含 FLAC/WAV/ISO/PDF 等） | `source_sel[4]`（11 选项，纯视频媒介） |
| 视频编码 | **无独立字段** | `codec_sel[4]`（8 选项） |
| 音频编码 | **无独立字段** | `audiocodec_sel[4]`（18 选项） |
| 分辨率 | **无独立字段** | `standard_sel[4]`（8 选项） |
| 制作组 | **无独立字段** | `team_sel[4]`（5 选项） |
| 标签 | 11 个独立 name checkbox | `tags[4][]` 数组 checkbox（16 个） |
| 引用发布 | `referid` 字段 | **无引用字段** |
| 分类 ID | 402=纪录片，404=电视剧 | 402=剧集，404=纪录片 |
| NFO | **无** | 有 `nfo` file 字段 |

> **核心差异**：猫站只有 3 个下拉框（type/source_sel/team_sel），标题中的编码、分辨率、音频等纯靠文本匹配审核。青蛙站有 7 个下拉框，标题和下拉必须同时匹配。

---

#### 发布流程与表单映射

发布流水线中的字段映射阶段（§11.10 字段映射 + §11.11 SitePublisher 接口）：

```
┌─ 步骤 1: 标准化参数生成
├→ standard_params.type → 根据站点配置映射到具体分类值
└→ standard_params.resolution → 映射到具体分辨率值

┌─ 步骤 2: 表单字段映射（根据青蛙站点发布页面分析）
│
├── 标准化映射
│   ├── standardized_params.type → formFields["type"]
│   ├── standardized_params.resolution → formFields["standard_sel"]
│   ├── standardized_params.medium → formFields["medium_sel"]
│   ├── standardized_params.video_codec → formFields["codec_sel"]
│   └── standardized_params.audio_codec → formFields["audiocodec_sel"]
│
└── 站点特殊字段
    ├── req.Title → formFields["name"]
    ├── req.Subtitle → formFields["small_descr"]
    ├── req.IMDbLink → formFields["url"]
    ├── req.DoubanLink → formFields["pt_gen"]
    ├── req.Description → formFields["descr"]
    ├── req.PieceSize → formFields["size"]
    └── req.ExtraFields["b"] → formFields["b"]
```

#### 标签列表（来自 Wiki 标签规则页）

以下为青蛙站点的完整标签定义，用于发布时勾选：

| 标签 | 说明 | 自动/手动 |
|------|------|-----------|
| 官方 | 官方组成员发布带官方后缀的资源 | 自动（普通用户无需选） |
| 驻站 | 驻站组成员发布带驻站小组后缀的资源 | 自动（普通用户无需选） |
| VCB-Studio | VCB-Studio 小组作品 | 手动 |
| 国语 | 含国语（普通话）音轨 | 手动 |
| 粤语 | 含粤语音轨 | 手动 |
| 中字 | 内嵌中文硬字幕、内封中文软字幕或包含外挂中文字幕 | 手动 |
| 特效字幕 | 带有特效字幕的视频 | 手动 |
| DIY | DIY 资源（Custom Disc） | 手动 |
| 原生原盘 | 未经修改的原盘资源（蓝光原盘、DVD原盘） | 手动 |
| Remux | 由原盘提取未经重编码的 Remux 资源 | 手动 |
| 分集 | 连载中剧集的某几集（需申请） | 手动 |
| 完结 | 完结的**剧集**（电影类不要选择） | 手动 |
| 杜比视界 | 含有杜比视界的视频 | 手动 |
| HDR | 含有 HDR10（静态）的视频 | 手动 |
| HDR10+ | 含有 HDR10+（动态）的视频，**必须同时选择 HDR 标签** | 手动 |
| 儿童片 | 适合儿童观看的家庭片与教育片 | 手动 |
| 禁转 | 官方组/驻站组/发布者自制不希望被转载的资源 | 手动 |
| 系列合集 | 多季合一或系列电影打包的资源 | 手动 |
| 零魔 | 系统自动判断：做种数/完成数>=3 且做种数>50 | 自动 |

#### 黑名单和灰名单制作组（禁止/不建议转载）

**黑名单（禁止转载）**：

| 制作组 | 原因 |
|--------|------|
| DBD-Raws | 盗用资源、超分、劣迹斑斑 |
| Skymoon/天月/HKACG | 反华组 |
| c.c动漫 | 改名组 |
| 猎户发布组/orion origin、爪爪字幕组/ZhuaZhuaStudio | 机翻组 |

**黑名单（盗转/改名发布组，禁止转载）**：

FGT, NSBC, BATWEB, GPTHD, DreamHD, BlackTV, CatWEB, Xiaomi, Huawei, MOMOWEB, DDHDTV, SeeWeb, TagWeb, SonyHD, MiniHD, BitsTV, ALT, LelveTV, NukeHD, ZeroTV, HotTV, EntTV, GameHD, SmY, SeeHD, ParkHD, VeryPSP, DWR, XLMV, XJCTV, Mp4Ba, Huluwa, CTRLHD(非CtrlHD), HotWEB, TBMaxUB, BestWEB

**灰名单（不建议转载）**：

| 制作组 | 原因 |
|--------|------|
| 异域11番小队、加刘景长 | 低码率 |
| Reinforce | 高体积渣画质 |

#### 发布流程

```go
// QingWapPublishAction 发布动作实现
func (a *QingWapPublishAction) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
    // 步骤 1: 验证标题命名规范
    if err := a.validateTitleNaming(req); err != nil {
        return nil, fmt.Errorf("标题命名规范验证失败: %w", err)
    }

    // 步骤 2: 验证文件规范
    if err := a.validateFileSpecs(req); err != nil {
        return nil, fmt.Errorf("文件规范验证失败: %w", err)
    }

    // 步骤 3: 处理重复判定逻辑
    if err := a.handleDuplicateRules(req); err != nil {
        return nil, fmt.Errorf("重复判定规则处理失败: %w", err)
    }

    // 步骤 4: 执行自查流程
    if err := a.runSelfCheck(req); err != nil {
        return nil, fmt.Errorf("自查流程失败: %w", err)
    }

    // 步骤 5: 构建发布表单
    formFields, err := a.buildPublishForm(req)
    if err != nil {
        return nil, err
    }

    // 步骤 6: 执行发布
    result, err := a.executePublish(ctx, req, formFields)
    if err != nil {
        return nil, err
    }

    // 步骤 7: 记录发布日志
    a.logPublishResult(ctx, req, result)

    return result, nil
}

// buildPublishForm 构建发布表单
func (a *QingWapPublishAction) buildPublishForm(req *PublishRequest) (map[string]string, error) {
    siteConfig := a.loadSiteConfig(req.SiteCode)
    if siteConfig == nil {
        return nil, errors.New("站点配置未找到")
    }

    // 构建表单字段
    formFields := make(map[string]string)

    // 基本字段
    formFields["name"] = req.Title
    formFields["small_descr"] = req.Subtitle
    formFields["url"] = req.IMDbLink
    formFields["pt_gen"] = req.DoubanLink

    // 描述（包含 MediaInfo、截图等）
    formFields["descr"] = a.buildDescription(req)

    // 文件大小
    if req.PieceSize != nil {
        formFields["size"] = fmt.Sprintf("%d", *req.PieceSize)
    }

    // 分类映射
    if standardizedType := req.StandardizedParams["type"]; standardizedType != "" {
        formFields["type"] = siteConfig.Mappings["type"][standardizedType]
    }

    // 分辨率映射
    if resolution := req.StandardizedParams["resolution"]; resolution != "" {
        formFields["standard_sel"] = siteConfig.Mappings["resolution"][resolution]
    }

    // 媒介映射
    if medium := req.StandardizedParams["medium"]; medium != "" {
        formFields["medium_sel"] = siteConfig.Mappings["medium"][medium]
    }

    // 视频编码映射
    if codec := req.StandardizedParams["video_codec"]; codec != "" {
        formFields["codec_sel"] = siteConfig.Mappings["video_codec"][codec]
    }

    // 音频编码映射
    if audioCodec := req.StandardizedParams["audio_codec"]; audioCodec != "" {
        formFields["audiocodec_sel"] = siteConfig.Mappings["audio_codec"][audioCodec]
    }

    // 来源地区映射
    if sourceArea := req.StandardizedParams["source_area"]; sourceArea != "" {
        formFields["source_area_sel"] = siteConfig.Mappings["source_area"][sourceArea]
    }

    // 匿名发布处理
    if siteConfig.Anonymous != nil && *siteConfig.Anonymous.EnabledValue != "" {
        formFields["uplver"] = *siteConfig.Anonymous.EnabledValue
    }

    // 发布参数落盘（调试支持）
    if os.Getenv("DEV_ENV") == "true" {
        a.dumpPublishParams(req, formFields)
    }

    return formFields, nil
}

// buildDescription 构建描述内容
func (a *QingWapPublishAction) buildDescription(req *PublishRequest) string {
    var parts []string

    // 1. PTGen 元数据
    if req.DoubanLink != "" {
        parts = append(parts, fmt.Sprintf("豆瓣链接: %s\n", req.DoubanLink))
    }

    // 2. MediaInfo（青蛙站点必须）
    if req.MediaInfo != "" {
        parts = append(parts, "\n")
        parts = append(parts, "MediaInfo:\n")
        parts = append(parts, req.MediaInfo)
    }

    // 3. 截图
    if req.Screenshots != "" {
        parts = append(parts, "\n")
        parts = append(parts, "截图:\n")
        parts = append(parts, req.Screenshots)
    }

    // 4. 简介/备注
    if req.Description != "" {
        parts = append(parts, "\n")
        parts = append(parts, "简介:\n")
        parts = append(parts, req.Description)
    }

    return strings.Join(parts, "\n")
}

// loadSiteConfig 加载站点配置
func (a *QingWapPublishAction) loadSiteConfig(siteCode string) (*SitePublishConfig, error) {
    // 实现站点配置加载逻辑
    return nil, nil
}
```

#### 发布状态机

青蛙站点发布流程状态机：

```
[PENDING] → [CHECKING] → [CHECK_FAILED] → [VALIDATING] → [PUBLISHING] → [PUBLISHED] → [FAILED] → [CANCELLED]
                                      ↓
                                  [DELETED]（管理员或系统删除）
```

---

## Hook 实现

### QingWapHook 结构

```go
// QingWapHook 青蛙站点特异化钩子
type QingWapHook struct{}

func (h *QingWapHook) BeforePublish(ctx context.Context, req *PublishRequest) error {
    if req.ExtraFields == nil {
        req.ExtraFields = make(map[string]string)
    }

    // 步骤 1: 验证标题命名规范
    if err := h.validateTitleNaming(req); err != nil {
        return fmt.Errorf("标题命名规范验证失败: %w", err)
    }

    // 步骤 2: 验证文件规范
    if err := h.validateFileSpecs(req); err != nil {
        return fmt.Errorf("文件规范验证失败: %w", err)
    }

    // 步骤 3: 处理重复判定逻辑
    if err := h.handleDuplicateRules(req); err != nil {
        return fmt.Errorf("重复判定规则处理失败: %w", err)
    }

    // 步骤 4: 执行自查流程
    if err := h.runSelfCheck(req); err != nil {
        return fmt.Errorf("自查流程失败: %w", err)
    }

    return nil
}

func (h *QingWapHook) AfterPublish(ctx context.Context, result *PublishResult) error {
    return nil
}
```

### 验证函数

#### validateTitleNaming 验证标题命名规范

```go
// validateTitleNaming 验证标题命名规范
func (h *QingWapHook) validateTitleNaming(req *PublishRequest) error {
    title := strings.TrimSpace(req.Title)
    if title == "" {
        return errors.New("标题不能为空")
    }

    // 检查剧集季度编号格式
    if h.isSeries(req.StandardizedParams) {
        if !h.hasSeasonNumber(title) {
            return errors.New("剧集必须有季度编号（S01 格式）")
        }
    }

    // 检查压制版关键字
    if h.isEncode(req.StandardizedParams) {
        if strings.Contains(title, "规格") {
            return errors.New("压制版不应包含'规格'字段")
        }
        if !strings.Contains(title, "HDR") && strings.Contains(title, "HDR") {
            return errors.New("非 HDR 不应写 HDR 类型")
        }
    }

    // 检查 x264 10bit 格式
    if strings.Contains(strings.ToUpper(title), "X264") {
        if !strings.Contains(title, "Hi10P x264") && h.has10Bit(req.StandardizedParams) {
            return errors.New("x264 10bit 必须写为'Hi10P x264'")
        }
    }

    return nil
}
```

#### validateFileSpecs 验证文件规范

```go
// validateFileSpecs 验证文件规范
func (h *QingWapHook) validateFileSpecs(req *PublishRequest) error {
    // 检查分块大小
    if req.PieceSize != nil && *req.PieceSize > 16*1024*1024 {
        return errors.New("分块大小不能超过 16MB")
    }

    // 检查文件名长度
    if req.FileName != "" {
        // 中文 100 字，英文 200 字
        chineseCount := h.countChinese(req.FileName)
        englishCount := len([]rune(req.FileName)) - chineseCount
        if chineseCount > 100 || englishCount > 200 {
            return errors.New("文件名过长（≤100 中文/200 英文）")
        }

        // 检查特殊符号
        if h.hasSpecialChars(req.FileName) {
            return errors.New("文件名包含特殊符号（斜杠、引号等）")
        }

        // 检查点号结尾
        if strings.HasSuffix(req.FileName, ".") {
            return errors.New("文件名不能以点号结尾")
        }
    }

    return nil
}
```

#### handleDuplicateRules 处理重复判定逻辑

```go
// handleDuplicateRules 处理重复判定逻辑
func (h *QingWapHook) handleDuplicateRules(req *PublishRequest) error {
    // 步骤 1: 检查是否有完结资源
    if h.hasCompletedResource(req) {
        return h.checkIfShouldReplaceSingleEpisodes(req)
    }

    // 步骤 2: 检查跨季资源
    if h.isCrossSeason(req) {
        return h.checkCrossSeasonReplacement(req)
    }

    // 步骤 3: 检查高清资源
    if h.isHighDefinition(req) {
        return h.checkLowDefinitionReplacement(req)
    }

    return nil
}
```

#### runSelfCheck 执行自查流程

```go
// runSelfCheck 执行自查流程
func (h *QingWapHook) runSelfCheck(req *PublishRequest) error {
    // 检查点 1: 检查允许的资源
    if err := h.checkAllowedResource(req); err != nil {
        return fmt.Errorf("允许的资源检查失败: %w", err)
    }

    // 检查点 2: 检查种子文件
    if err := h.checkTorrentFile(req); err != nil {
        return fmt.Errorf("种子文件检查失败: %w", err)
    }

    // 检查点 3: 检查主标题
    if err := h.checkMainTitle(req); err != nil {
        return fmt.Errorf("主标题检查失败: %w", err)
    }

    // 检查点 4: 检查副标题 & IMDB & PT-Gen & NFO
    if err := h.checkSubtitleAndMetadata(req); err != nil {
        return fmt.Errorf("副标题&元数据检查失败: %w", err)
    }

    // 检查点 5: 检查简介
    if err := h.checkDescription(req); err != nil {
        return fmt.Errorf("简介检查失败: %w", err)
    }

    // 检查点 6: 检查 MediaInfo
    if err := h.checkMediaInfo(req); err != nil {
        return fmt.Errorf("MediaInfo 检查失败: %w", err)
    }

    // 检查点 7: 检查类型 & 质量 & 标签
    if err := h.checkTypeQualityTags(req); err != nil {
        return fmt.Errorf("类型&质量&标签检查失败: %w", err)
    }

    return nil
}
```

### 辅助方法

```go
// isSeries 判断是否为剧集
func (h *QingWapHook) isSeries(standardized map[string]any) bool {
    if standardized == nil {
        return false
    }
    type_ := strings.ToLower(toStringAny(standardized["type"], ""))
    return type_ == "category.tv_series" || type_ == "category.animation"
}

// hasSeasonNumber 检查季度编号格式
func (h *QingWapHook) hasSeasonNumber(title string) bool {
    return regexp.MustCompile(`S\d{2}`).MatchString(title)
}

// isEncode 判断是否为压制版
func (h *QingWapHook) isEncode(standardized map[string]any) bool {
    if standardized == nil {
        return false
    }
    medium := strings.ToLower(toStringAny(standardized["medium"], ""))
    return strings.Contains(medium, "encode")
}

// has10Bit 检查是否为 10bit
func (h *QingWapHook) has10Bit(standardized map[string]any) bool {
    if standardized == nil {
        return false
    }
    bit := strings.ToLower(toStringAny(standardized["bit_depth"], ""))
    return bit == "10bit"
}

// countChinese 统计中文字符数
func (h *QingWapHook) countChinese(s string) int {
    var count int
    for _, r := range s {
        if unicode.Is(unicode.Han, r) {
            count++
        }
    }
    return count
}

// hasSpecialChars 检查特殊符号
func (h *QingWapHook) hasSpecialChars(s string) bool {
    specialChars := []string{`/`, `\`, `"`, `'`, "`", "*"}
    for _, char := range specialChars {
        if strings.Contains(s, char) {
            return true
        }
    }
    return false
}

// hasCompletedResource 检查是否为完结资源
func (h *QingWapHook) hasCompletedResource(req *PublishRequest) bool {
    // 检查是否为完结资源（通过标题或元数据判断）
    title := strings.ToLower(req.Title)
    return strings.Contains(title, "complete") ||
           strings.Contains(title, "完结") ||
           strings.Contains(title, "全集")
}

// isCrossSeason 检查是否为跨季资源
func (h *QingWapHook) isCrossSeason(req *PublishRequest) bool {
    title := strings.ToLower(req.Title)
    return regexp.MustCompile(`S\d{2}-S\d{2}`).MatchString(title)
}

// isHighDefinition 检查是否为高清资源
func (h *QingWapHook) isHighDefinition(req *PublishRequest) bool {
    // 检查是否为高清资源（≥720p）
    resolution := strings.ToLower(req.StandardizedParams["resolution"])
    return strings.Contains(resolution, "1080p") ||
           strings.Contains(resolution, "2160p")
}

// checkAllowedResource 检查是否为允许的资源
func (h *QingWapHook) checkAllowedResource(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkTorrentFile 检查种子文件
func (h *QingWapHook) checkTorrentFile(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkMainTitle 检查主标题
func (h *QingWapHook) checkMainTitle(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkSubtitleAndMetadata 检查副标题 & IMDB & PT-Gen & NFO
func (h *QingWapHook) checkSubtitleAndMetadata(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkDescription 检查简介
func (h *QingWapHook) checkDescription(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkMediaInfo 检查 MediaInfo
func (h *QingWapHook) checkMediaInfo(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}

// checkTypeQualityTags 检查类型 & 质量 & 标签
func (h *QingWapHook) checkTypeQualityTags(req *PublishRequest) error {
    // 实现具体检查逻辑
    return nil
}
```

## 配置示例

青蛙站点使用标准的 NexusPHP 配置，无需特殊配置。

## 测试用例

### 标题命名规范测试

```go
func TestQingWapHook_ValidateTitleNaming(t *testing.T) {
    tests := []struct {
        name     string
        title    string
        wantErr  bool
    }{
        {"正常标题", "The Last 10 Years 2022 1080p BluRay x265 DD 5.1-CMCT", false},
        {"剧集无季度", "Game of Thrones 2022 1080p BluRay x265 DD 5.1-CMCT", true},
        {"压制版有规格", "Test 2022 Encode 1080p 规格 BluRay x265 DD 5.1-CMCT", true},
        {"x264 10bit", "Test 2022 1080p BluRay Hi10P x264 DD 5.1-CMCT", false},
        {"x264 10bit 错误", "Test 2022 1080p BluRay x264 DD 5.1-CMCT", true},
    }

    hook := &QingWapHook{}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := &PublishRequest{
                Title:            tt.title,
                StandardizedParams: map[string]any{},
            }

            err := hook.validateTitleNaming(req)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateTitleNaming() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 文件规范测试

```go
func TestQingWapHook_ValidateFileSpecs(t *testing.T) {
    tests := []struct {
        name     string
        fileName string
        wantErr  bool
    }{
        {"正常文件名", "测试文件名.txt", false},
        {"文件名过长（中文）", strings.Repeat("中", 101) + ".txt", true},
        {"文件名过长（英文）", strings.Repeat("a", 201) + ".txt", true},
        {"文件名有特殊符号", "测试/文件名.txt", true},
        {"文件名以点结尾", "测试文件名.", true},
    }

    hook := &QingWapHook{}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := &PublishRequest{
                FileName: tt.fileName,
            }

            err := hook.validateFileSpecs(req)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateFileSpecs() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## 参考资源

- 青蛙站点：https://www.qingwapt.com
- 青蛙 Wiki：https://wiki.qingwapt.org/docs
- 发种规则（标题命名规范）：https://wiki.qingwapt.org/docs/rules/content-rules/upload-title
- 重复和合集规则：https://wiki.qingwapt.org/docs/rules/content-rules/duplicate-collection
- 制种和转种规则：https://wiki.qingwapt.org/docs/rules/content-rules/torrent-transfer
- 黑名单和灰名单：https://wiki.qingwapt.org/docs/rules/content-rules/blacklist
- 发布和做种规则：https://wiki.qingwapt.org/docs/rules/content-rules/publish-seed
- 标签规则：https://wiki.qingwapt.org/docs/rules/content-rules/tags
- 分区发布规则：https://wiki.qingwapt.org/docs/rules/content-rules/category-rules
- MediaInfo和BDInfo教程：https://wiki.qingwapt.org/docs/rules/content-rules/upload-tutorials
- 影片参数详解：https://wiki.qingwapt.org/docs/guides/content-creation/mediainf
- 杜比视界：https://wiki.qingwapt.org/docs/guides/content-creation/dv
- 流媒体厂商缩写：https://wiki.qingwapt.org/docs/guides/content-creation/streaming
- Forum 205: 发种规范 v1.02（Dupe/制种/转种/压缩包/发布/做种/分区规则）
- Forum 208: 视频标题命名规范 v1.05（含 ~170 流媒体厂商缩写附件）
- Forum 197: 转种自查指南 v0.1（7 步自查流程）
- 自查助手（油猴插件）：https://greasyfork.org/zh-CN/scripts/490095-qingwa-torrent-assistant

## 审核脚本完整逆向分析

### 脚本信息

| 项目 | 内容 |
|------|------|
| 名称 | qingwa-torrent-assistant |
| 来源 | Greasyfork #490095 |
| 版本 | 1.1.1 |
| 作者 | QingWaPT-Official |
| 致谢 | 不可说-Torrent-Assistant, 末日-Torrent-Assistant |
| 大小 | 1353 行 |
| 运行页面 | `details.php*`（种子详情/审核页） |
| 权限 | GM_xmlhttpRequest / GM_setValue / GM_getValue |

> **基于不可说/末日审核脚本改写**，结构与 Agsv-Torrent-Assistant 高度相似，但规则针对青蛙站定制。

### 常量映射

#### 分类 (cat_constant)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | 其他 |
| 412 | 短剧 |

> **注意**：音乐(408)和其他(409)分类**跳过大部分校验规则**（仅音乐额外检查采样频率和比特率）。

#### 媒介 (type_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | UHD Blu-ray | `uhd.*blu-?ray` |
| 8 | Blu-ray | `blu-?ray`（无 x264/5、非 remux） |
| 9 | Remux | `remux` |
| 10 | Encode | `bluray/blu-ray` + `x26[45]`（Encode 格式） |
| 7 | WEB-DL | `web-?dl`（无 x264/5） |
| 4 | HDTV | `hdtv` |
| 11 | MiniBD | `minibd` |
| 2 | DVD | `dvd`（无 x264/5） |
| 3 | CD | - |
| 5 | Track | - |
| 6 | Other | - |

> **关键区分**：原盘 `Blu-ray`（连字符）vs 压制 `BluRay`（无连字符）。Encode 类标题须用 `BluRay`。

#### 视频编码 (encode_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 1 | H.264/AVC | `x264\|h264\|h.264\|avc` |
| 6 | H.265/HEVC | `x265\|h265\|h.265\|hevc` |
| 2 | VC-1 | `vc-?1` |
| 4 | MPEG-2 | `mpeg-?2` |
| 7 | AV1 | `av1` |
| 3 | MPEG-4 | `mpeg-?4` |
| 8 | VP9 | `vp9` |
| 5 | Other | - |

> **命名规范严格**：原盘用 AVC/HEVC，WEB 用 H.264/H.265，Encode 用 x264/x265。HEVC≠H.265 视媒介类型区分。

#### 音频编码 (audio_constant)

| ID | 名称 | 标题匹配关键词 |
|----|------|--------------|
| 9 | DTS:X | `dts.x`/`dts-x`/`dtsx` |
| 11 | TrueHD Atmos | `truehd.*atmos` |
| 12 | TrueHD | `truehd`（排除 atmos） |
| 10 | DTS-HD MA | `dts-hd.?ma`/`dtshdma` |
| 3 | DTS-HD HRA | `dts-hd.?hr`/`dtshdhr` |
| 14 | DTS | `dts`（排除 dts-hd 等） |
| 16 | DDP/E-AC3 | `ddp`/`dd+`/`e-?ac3`/`dolby digital plus` |
| 15 | DD/AC3 | `ac-?3`/`dd[^p]`/`dolby digital[^+]` |
| 13 | LPCM | `lpcm`/`pcm` |
| 1 | FLAC | `flac` |
| 17 | AAC | `aac` |
| 18 | APE | `ape` |
| 19 | WAV | `wav` |
| 4 | MP3 | `mp3` |
| 8 | M4A | `m4a` |
| 20 | OPUS | `opus` |
| 22 | AV3A | `av3a` |
| 23 | USAC | `usac`（映射到 AAC 组检测） |
| 7 | Other | - |

> **DD vs DDP 严格区分**：AC3=DD，E-AC3=DDP。标题禁止写 AC3/E-AC3，必须写 DD/DDP。

#### 分辨率 (resolution_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 6 | 8K | `8k\|4320p` |
| 7 | 4K | `2160p\|4k[.\| ]`（排除 remastered） |
| 1 | 1080p | `1080p` |
| 2 | 1080i | `1080i` |
| 3 | 720p | `720p` |
| 4 | SD | `480p\|576p\|sd` |
| 5 | Other | - |

> **P 小写强制**：标题中 `1080P` → 错误，应为 `1080p`。`4K` 在标题中应改为 `2160p`。

#### 制作组 (group_constant)

| ID | 名称 | 匹配 |
|----|------|------|
| 6 | FROG | 标题含 `frog` |
| 7 | FROGE | 标题含 `froge` |
| 8 | FROGWeb | 标题含 `frogweb` |
| 9 | GodDramas | 标题含 `goddramas` |
| 5 | Other | 其他 |

> **官组检测**：标题含 `frog`/`froge`/`frogweb`/`Loong@QingWa` → `officialSeed=true`。

### 标题解析算法

#### 解析流程

```
1. 标题转小写 → title_lowercase
2. 排除年份/季集后的干扰
3. 位置约束校验：
   ├── 分辨率 须在 来源 前 → 否则报错
   ├── HDR 须在 视频编码 前 → 否则报错
   └── 视频编码 须在 音频 前 → 否则报错
4. 正则匹配链（按优先级）：
   ├── 分辨率 → resolution_constant
   ├── 来源类型 → type_constant
   ├── 视频编码 → encode_constant
   ├── 音频编码 → audio_constant（取最高规格）
   └── 制作组 → group_constant
```

#### HDR 检测与交叉验证

```
1. 标题含 hdr10+ → 标签须含 hdr10+
2. 标题含 hdr（非 hdr10+）→ 标签须含 hdr
3. 标签含 hdr10+ → 标题须含 hdr10+
4. 标签含 hdr → 标题须含 hdr
5. MediaInfo HDR Format 字段与标题/标签双向验证
```

### 校验规则 — 共 44+ 项

#### 分类级特殊规则

| 分类 | 规则 |
|------|------|
| 音乐(408) | 跳过大部分校验；主标题须含采样频率(`khz`)；主标题须含比特率(`bit`) |
| 其他(409) | 跳过所有校验规则 |
| 官方音乐种子 | 跳过所有检查（`officialSeed && cat===408`） |

#### 标题校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 标题含中文或全角字符 | `[\u4e00-\u9fa5\uff01-\uff60]` | 错误 |
| 2 | Complete 需删除（非蓝光原盘） | `complete` 在标题中 | 错误 |
| 3 | 季集应在年份前 | `S\d{2}` 位置检查 | 错误 |
| 4 | HDR10 应为 HDR | `hdr10`（非 HDR10+）→ 应写 HDR | 错误 |
| 5 | HDR10+ 标签与标题交叉验证 | 标签 vs 标题 | 错误 |
| 6 | HDR 标签与标题交叉验证 | 标签 vs 标题 | 错误 |
| 7 | WEB 资源 HEVC→H.265 | `hevc` in WEB 类型 | 错误 |
| 8 | 电影类别不应含 S**E** | `s\d{2}e\d{2}` + cat=401 | 错误 |
| 9 | 剧集类别必须含 S**E** | `!s\d{2}e\d{2}` + cat=402 | 错误 |
| 10 | WEB 资源 AVC→H.264 | `avc` in WEB 类型 | 错误 |
| 11 | HDTV 资源 HEVC→H265 | `hevc` in HDTV 类型 | 错误 |
| 12 | HDTV 资源 AVC→H264 | `avc` in HDTV 类型 | 错误 |
| 13 | 禁发小组（28+组） | 正则匹配黑名单 | 错误 |
| 14 | 分辨率 P→p | `\d+P` 大写检测 | 错误 |
| 15 | AC3→DD | `ac3` in 标题 | 错误 |
| 16 | 删除 HQ/FPS/EDR/SDR/10bit/4K(→2160p) | 关键词检测 | 错误 |
| 17 | 来源须在编码前 | 位置校验 | 错误 |
| 18 | 分辨率须在来源前 | 位置校验 | 错误 |
| 19 | 缺少分辨率/来源/编码/音频 | 完整性检查 | 错误 |
| 20 | 视频编码须在音频前 | 位置校验 | 错误 |
| 21 | 蓝光原盘标题格式 | `Blu-ray` vs `BluRay` | 错误 |
| 22 | Encode 标题格式 | `BluRay`（无连字符） | 错误 |
| 23 | Atmos 应在声道后 | `atmos` 位置校验 | 错误 |
| 24 | 声道数标示错误 | `7\.1`/`5\.1` 格式检查 | 错误 |

#### 字段选择校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 25 | 副标题为空 | `!subtitle` | 错误 |
| 26 | 未选分类 | `!cat` | 错误 |
| 27 | 未选媒介 | `!type` | 错误 |
| 28 | 未选编码 | `!encode` | 错误 |
| 29 | 未选音频 | `!audio` | 错误 |
| 30 | 未选分辨率 | `!resolution` | 错误 |
| 31 | 标题与选择字段不一致（媒介） | 标题解析 vs 用户选择 | 错误 |
| 32 | 标题与选择字段不一致（编码） | 标题解析 vs 用户选择 | 错误 |
| 33 | 标题与选择字段不一致（音频） | 标题解析 vs 用户选择 | 错误 |
| 34 | 标题与选择字段不一致（分辨率） | 标题解析 vs 用户选择 | 错误 |
| 35 | 官种标签/制作组双向验证 | 官组检测 vs 选择 | 错误 |
| 36 | 未选择制作组 | `!group` | 错误 |

#### 标签与 MI 交叉校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 37 | HLG 需添加 HDR 标签 | 标签检查 | 错误 |
| 38 | 国语/粤语/中字 标签与 MI 交叉验证 | MI 语言检测 vs 标签 | 错误 |
| 39 | HDR/HDR10+/杜比视界 标签与 MI 交叉验证 | MI HDR Format vs 标签 | 错误 |
| 40 | VCB-Studio 标签校验 | 制作组 vs 标签 | 错误 |
| 41 | Remux 标签校验 | 媒介 vs 标签 | 错误 |
| 42 | 完结/分集/合集标签与季集交叉验证 | 标题 S**E** vs 标签 | 错误 |

#### 简介与媒体信息校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 43 | 简介无 IMDb/豆瓣链接 | 链接检测 | 警告 |
| 44 | MediaInfo 含 BBCode | `[b]\|[color]` 等标签 | 错误 |
| 45 | 简介含 MediaInfo（应在独立栏） | `#kdescr` 内容检测 | 错误 |
| 46 | 原盘 MI 用 BDInfo/非原盘用 MediaInfo | 媒介类型 vs MI 格式 | 错误 |
| 47 | 蓝光原盘必须选 DIY 或原生原盘标签 | 媒介=原盘 + `!diy && !native` | 错误 |
| 48 | 缺少海报/截图 | `#kposter img` + `#ktorrentscreenshots img` | 错误 |
| 49 | MediaInfo 栏为空/不正确 | 空值或格式校验 | 错误 |
| 50 | 官组标题编码应为 x264/x265 | `officialSeed && !x264/x265` | 错误 |

#### 审核脚本额外规则（从实际代码逆向）

| # | 规则 | 说明 |
|---|------|------|
| 51 | DVD+720p 交叉检查 | 标题含 DVD 且分辨率=720p → 警告"请检查分辨率是否错标" |
| 52 | USAC→AAC 分组映射 | `title.includes('USAC')` 映射到 AAC 组（title_audio=17） |
| 53 | `officialMusicSeed` 独立路径 | 标题含 `frogmus` → 清空所有错误，仅检查制作组选择（预留功能） |
| 54 | 分辨率匹配含隔行变体 | `720i`/`2160i`/`4320i`/`uhd`（裸关键词）也有匹配逻辑 |
| 55 | 标签校验精确逻辑 | `title_ES>=1`（无 S##E##）→须有完结标签；`title_ES==0`（有 S##E##）→须有分集标签；非多季→不得有合集标签 |

#### 警告类规则

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| W1 | SD/Other 分辨率 | resolution=4 or 5 | 警告 |
| W2 | 简介无 IMDb/豆瓣链接 | 链接检测 | 警告 |
| W3 | 异常图片（高度≤24px） | `img.height() <= 24` | 警告 |

### 禁发制作组（28+组）

```
FGT, NSBC, BATWEB, GPTHD, DreamHD, BlackTV, CatWEB, Xiaomi, Huawei,
MOMOWEB, DDHDTV, SeeWeb, TagWeb, SonyHD, MiniHD, BitsTV, ALT,
LelveTV, NukeHD, ZeroTV, HotTV, EntTV, GameHD, SmY, SeeHD, ParkHD,
VeryPSP, DWR, XLMV, XJCTV, Mp4Ba, Huluwa, CTRLHD(非CtrlHD),
HotWEB, TBMaxUB, BestWEB, RARBG, Hao4K, MOMOHD,
DBD-Raws, Skymoon/天月/HKACG, c.c动漫,
猎户发布组/orion origin, 爪爪字幕组/ZhuaZhuaStudio
```

### UI 功能

| 功能 | 说明 |
|------|------|
| 错误提示框 | 红色背景 `#EA2027`，显示所有错误 |
| 通过提示框 | 绿色背景 `#8BC34A`，显示"此种子未检测到错误" |
| 警告提示框 | 黄色，显示警告信息 |
| 图片加载等待 | 30 秒超时，加载完成后检查异常图片 |
| 快捷键 F4 | 一键通过/驳回（根据是否有错误） |
| 快捷键 F3 | 关闭窗口 |
| 审核页自动操作 | 自动点击通过/驳回按钮，自动填写错误信息 |
| 错误信息翻译 | 自动将技术错误转为用户友好提示 |

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题全部英文（移除中文、全角字符）
2. 按青蛙命名规范重构标题：
   剧名 年份 其他信息 分辨率 来源类型 规格 HDR bit 视频编码 音频 声道数 Atmos 音轨数-制作组
3. 原盘用 Blu-ray（连字符），Encode 用 BluRay（无连字符）
4. 原盘编码用 AVC/HEVC，WEB 用 H.264/H.265，Encode 用 x264/x265
5. P 小写（1080p 非 1080P），4K 改为 2160p
6. AC3→DD，E-AC3→DDP
7. 移除源站前缀标签（如 [馒头]、[HDArea] 等）
8. Complete 需删除（非蓝光原盘）
9. x264 10bit 须写为 Hi10P x264
```

### 副标题自动处理

```
1. 禁止为空（必填）
2. 建议格式：中文名 | 外文名 | 包含内容等
3. 优先从 PT-Gen/豆瓣获取中文名
```

### 质量字段自动选择

```
从源站标题解析：
1. 媒介(type)：
   UHD Blu-ray → 1, Blu-ray → 8, Remux → 9, Encode → 10,
   WEB-DL → 7, HDTV → 4, MiniBD → 11, DVD → 2, CD → 3, Track → 5, Other → 6
2. 编码(encode)：
   H.264/AVC → 1, H.265/HEVC → 6, VC-1 → 2, MPEG-2 → 4,
   AV1 → 7, MPEG-4 → 3, VP9 → 8, Other → 5
3. 音频(audio)：按匹配优先级
   DTS:X → 6, TrueHD Atmos → 14, DDP → 11, TrueHD → 12,
   DTS-HD MA → 16, DTS-HD HR → 17, DTS → 2, DD → 1,
   FLAC → 4, AAC → 3, LPCM → 5, ALAC → 7, WAV → 8,
   OPUS → 10, AV3A → 15, USAC → 18, Other → 22
4. 分辨率(resolution)：
   8K → 6, 4K/2160p → 7, 1080p → 1, 1080i → 2,
   720p → 3, SD → 4, Other → 5
5. 制作组(group)：
   FROG → 6, FROGE → 7, FROGWeb → 8, GodDramas → 9, Other → 5

注意 remastered 排除在 4K 检测之外
```

### 标签自动选择

```
1. HDR：MI 含 HDR Format（非 DV）→ 勾选 HDR
2. HDR10+：MI 含 HDR10+ → 勾选 HDR10+（必须同时选 HDR）
3. 杜比视界：MI 含 Dolby Vision → 勾选杜比视界
4. 国语：MI 音频语言含 Chinese → 勾选国语
5. 粤语：MI 音频语言含 Chinese/Yue/Cantonese → 勾选粤语
6. 中字：MI 字幕语言含 Chinese → 勾选中字
7. VCB-Studio：制作组含 VCB-Studio → 勾选
8. DIY：标题/副标题含 DIY → 勾选
9. 原生原盘：媒介为原盘且未修改 → 勾选
10. Remux：媒介为 Remux → 勾选
11. 完结：标题含 S01-S** 或季集完整 → 勾选
12. 分集：标题仅含部分集数 → 勾选（需申请）
13. 系列合集：多季/多部合集 → 勾选
```

### MediaInfo 处理

```
1. 非蓝光原盘用 MediaInfo，蓝光原盘用 BDInfo
2. MediaInfo 须英文（禁止中文 MI）
3. MediaInfo 禁止包含 BBCode 标签
4. MediaInfo 应在独立栏位，禁止放入简介
5. 简介中禁止包含 MediaInfo 内容
```

### 简介自动构建

```
1. IMDb/豆瓣链接（至少一个，否则警告）
2. 海报图片
3. PT-Gen 生成的简介内容
4. 至少 3 张视频截图
5. 蓝光原盘使用 BDInfo quick summary
6. 原出处简介用 quote 标签包裹：
   [quote]资源简介[/quote]
```

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-22 — 补充 W.10 盒子规则 + 下载规则摘要（Playwright 采集）*
*数据来源：upload.php + Wiki发布规则 + qingwa-torrent-assistant.js v1.1.1 (1886行/82KB) + 官方 Wiki Playwright 采集*

---

## 官方 Wiki 采集（2026-04-22 Playwright 抓取）

> 来源：`https://wiki.qingwapt.org` — Nuxt Content SPA，需 Playwright 渲染才能获取页面内容。
> CookieCloud 对接程序从 `.qingwapt.org` 域获取 cookie，但 Wiki 页面为公开内容，无需登录。

### W.1 发种规则总览和标题命名规范

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/upload-title`

**基本要求**：
- 原盘、DIY 资源请务必带 BDInfo，WEB、TV、压制资源请务必带 MediaInfo
- 标题要符合下面的 0day 规范
- 违规种子将被打回修改
- 转种请勿重新制种
- 种子通过审核后其他用户才能够连接上该种 tracker，未审和拒绝状态只有发种人可以连接

**视频标题命名规范（0day 命名法）**：

青蛙使用 0day 命名法（+少量改动）进行命名，这是一套较为严格的命名法。

**基本格式**：

```
剧名 季集信息 年份 其他信息 分辨率 地区码 内容分发方 片源类型 规格 HDR类型 bit信息 视频编码 音频编码 声道数 对象信息 音轨数-制作组
```

**官方示例**：

```
Dan Da Dan S01 2024 1080p BluRay x265 FLAC 2.0-FROGE
（Encode资源，规格为空，非HDR不写HDR类型，x265不写bit信息）

Gannibal S02 2025 2160p DSNP WEB-DL DV H.265 DDP 5.1-FROGE
（WEB-DL资源，流媒体产商简写DSNP，HDR类型为DV，视频编码为H.265）

Maze Runner: The Scorch Trials 2015 1080p FRA Blu-ray AVC DTS-HD MA 7.1-GMB
（Blu-ray资源，地区码为FRA，视频编码为AVC）

Sidonia no Kishi 2014-2015 1080p BluRay Hi10P x264 FLAC 2.0-VCB-Studio
（x264 10bit要写为【Hi10P x264】）

Clannad S01-S02+MOVIE REPACK 1080p / 480p BluRay / DVDRip Hi10P x264 FLAC 5.1-mawen1250&fch1993
（多季合集有DVD有BD，多个分辨率压制的情况）

Saki S01-S03 BluRay 1080p / 720p Hi10P x264 FLAC 2.0-VCB-Studio
（无重名可以省略年份）
```

**剧名**：
- 主标题推荐使用英文名，别名与中文名填入副标题
- 英文名可从 IMDB 获取（搜索框搜索后在条目页面查看）
- 豆瓣条目的"又名"中也有对应的英文名可供参考
- 动画可使用 MyAnimeList 的罗马音式标题

**季集信息**：
- 单季合集：加季度编号（即使只有1季也得写上 S01）
- 多季合集：S0a-S0b（季度≥10不写前导0，如 S08-S12）
- 分集：S##E## 或 S01E03-E05
- 每季剧名后缀不同时写为 `XXX Series`（多见于动漫区，如 Monogatati Series）

**年份**：
- 有重名时**必须**填写
- 无重名时可省略
- 多季合集填写 `最早年份-最晚年份`（必填）
- 单集综艺/体育节目填写完整年月日（如 20240326）

**其他信息（按优先级检查）**：

| 优先级 | 项目 | 说明 |
|--------|------|------|
| 1 | 剪辑版本 | Director's Cut、Uncensored、Extended、Unrated、Uncut 等。**需官方盖章**，如不是完结合集都能叫 Complete Edition 得官方确认 |
| 2 | 2in1 | 同时含2个剪辑版本，3in1 同理（多见于原盘资源） |
| 3 | 版本 | 20th Anniversary Edition、Remaster、4K Remaster、Limited Edition |
| 4 | 特殊比例 | IMAX、Open Matte、MAR |
| 5 | Hybrid | 仅当由两个或更多来源组成时注明（如不含 DV 层的原始视频添加了其他源的 DV 层） |
| 6 | REPACK/V2 | 文件有改动+主要视频**未**重新转码（增补CD/扫图/重压特典等） |
| 7 | RERIP | 主要视频文件**被**重新转码 |
| 8 | PROPER | 原盘抓取有问题，重新发布（仅原盘资源） |
| 9 | MiniBD | CMCT 特色资源（大小写不做强制规定） |

**分辨率**：
- 从 MediaInfo 的 Video - Height 和 Scan type 组合
- 注意：p 小写（1080p 非 1080P），不要使用 4K/2K
- 黑边裁切导致非标准分辨率（如716p）时可写 720p 或实际值
- DVD 原盘/Remux 可由制式（NTSC/PAL）代替

**地区码**：
- 仅原盘类填写（ITA/USA/JPN/HKG/TWN 等）
- 无法判定时可省略，不做强制要求

**内容分发方**：
- 原盘：标准收藏（CC）、电影大师（MOC）、华纳档案馆（WAC）等（详见 Wiki 发行商页）
- WEB-DL：流媒体厂商缩写名（详见 Wiki 流媒体缩写页）
- HDTV：电视台缩写（无法找到来源时可省略）

**片源类型（按媒介严格区分）**：

| 媒介 | 允许值 |
|------|--------|
| 原盘 | Blu-ray / 3D Blu-ray / UHD Blu-ray / Modded Blu-ray / Custom BluRay / NTSC DVD5 / NTSC DVD9 / PAL DVD5 / PAL DVD9 / HD DVD |
| 压制(Encode) | BluRay / 3D BluRay / UHD BluRay / DVDRip / HDDVDRip |
| WEB | **此项省略**，填写流媒体厂商缩写名 |
| HDTV | 填写电视台名称，无法找到来源时可省略 |

> **关键区分**：原盘 `Blu-ray`（连字符）vs 压制 `BluRay`（无连字符）。通行规则。

**规格**：Remux / WEB-DL / WEBRip / HDTV / UHDTV / HOU / HSBS。原盘类不填。Remux/WEBRip 大小写不做强制。

**HDR 类型**：HDR / HDR10+ / DV / DV HDR / DV HDR10+ / HLG / PQ10 / HDR vivid。SDR 不填。DV 也可写作 DoVi。

**bit 信息**：仅当 Format=AVC 且 Bit depth=10bit 时写 `Hi10P`。x265 10bit 只写 x265（不写 10bit）。

**视频编码（按媒介严格区分）**：

| 媒介 | 允许值 |
|------|--------|
| 蓝光原盘/REMUX | AVC / HEVC / MPEG-2 / VC-1 |
| DVD 原盘 | 可省略 |
| WEB | H.264 / H.265 / MPEG-2 / VC-1 / VP9 / AVS+ / AVS3 / AV1 |
| HDTV | H264 / H265 / MPEG2 / VP9 / AVS+ / AVS3 / AV1 |
| 压制(Encode) | x264 / x265 / AV1 / MPEG-2 / VP9 |

> **严禁混用**：原盘 AVC/HEVC，压制 x264/x265。WEB/HDTV 若 MediaInfo Writing library 明确有 x264/x265 也可写。
> H264=H.264，H265=H.265，MPEG2=MPEG-2。

**音频编码和声道数**：
- 多音轨仅标最高规格的音轨
- 声道数保留1位小数（L R=2.0，L R LFE=2.1，L R C LFE Ls Rs=5.1）
- AAC/MP2/MP3 且 2.0 时可省略声道数
- 合集中多部影片音轨规格不一可用 `/` 分开
- 评论音轨不计入

**标题写法对照**：DD=AC3, DDP=E-AC3/DD+。标题中**不应**写作 AC3/E-AC3，必须写 DD/DDP。

**对象信息**：Atmos / Auro3D。DTS:X 不需额外标注。省略视为不存在。

**音轨数**：正片多条音轨时写 XAudio（X为数字），仅 1 条不写。仅对压制(Encode)类强制要求。

**制作组**：
- 本站首发：`-你的名字@QingWa`
- 多站官组转种：`-组名`（如 `-VCB-Studio`）
- 个人他站转种：`-发布者@原站名`
- 匿名他站转种：`-Anonymous@原站名`
- 来源不明：留空或 `-NOGROUP` / `-NoGroup` / `-NoGrp`
- Scene 资源：可省略
- **严正警告**：务必认真对待，没写/写 NoGroup 后正主找上门将无条件处罚

**对 PT-Forward 的影响**：
- Wiki 确认了完整字段顺序：剧名→季集→年份→其他→分辨率→地区码→**内容分发方**→片源类型→规格→HDR→bit→视频编码→音频编码→声道→对象→音轨数→制作组
- 注意 **内容分发方** 在片源类型**前**，这是之前文档未明确强调的（§31.10.26 规则 #38 SourcePlatform 在 Medium 前已覆盖）
- 剧名中的冒号必须保留（如 `Maze Runner: The Scorch Trials`）
- 完结剧集不在标题写 Complete，用标签
- REPACK vs RERIP vs PROPER 三种更新场景已明确区分

### W.2 标签规则

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/tags`

| 标签 | 说明 |
|------|------|
| 官方 | 官方组成员发布带官方后缀的资源，普通用户无需选择 |
| 驻站 | 驻站组成员发布带驻站小组后缀的资源，普通用户无需选择 |
| VCB-Studio | VCB-Studio 小组的作品 |
| 国语 | 含国语（普通话）音轨 |
| 粤语 | 含粤语音轨 |
| 中字 | 内嵌中文硬字幕、内封中文软字幕或种子文件内包含外挂中文字幕 |
| 特效字幕 | 带有特效字幕的视频 |
| DIY | DIY 资源（Custom Disc） |
| 原生原盘 | 未经修改的原盘资源（蓝光原盘、DVD原盘） |
| Remux | 由原盘提取未经重编码的 Remux 资源 |
| 分集 | 连载中剧集的其中一集或某几集（需申请） |
| 完结 | 完结的**剧集**，电影类不要选择 |
| 杜比视界 | 含有杜比视界的视频 |
| HDR | 含有 HDR10（静态）的视频 |
| HDR10+ | 含有 HDR10+（动态）的视频，**必须同时选择 HDR 标签** |
| 儿童片 | 适合儿童观看的家庭片与教育片 |
| 禁转 | 官方组、驻站组或发布者自制的不希望被他人转载的资源 |
| 系列合集 | 多季合一或系列电影打包的资源 |
| 零魔 | 系统自动判断（做种数/完成数≥3 且做种数>50），不参与魔力计算 |

### W.3 重复和合集规则

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/duplicate-collection`

**重复判定总则**：
- 被代替资源将会被删除
- 完结资源将代替分集资源
- 跨季资源将代替相同来源和品质的单季资源
- 跨季资源将代替季数被覆盖的跨季资源（如 S01-S08 代替 S01-S07）
- 高清资源代替完全重复的低清资源
- 各个压制版允许共存
- 压制版与原盘允许共存
- 各个 DIY 原盘资源允许共存
- 蓝光资源代替更低清晰度且完全重复的 WEB/HDTV 资源
- **特殊**：动漫区蓝光对 TV 版极大幅度修正时 TV 资源保留；上古旧番高清仅有 WEB-DL 时也保留
- **WEB-DL 特殊规则**：同一个影视资源，在有官种（官方首发）的情况下，将不接受分辨率/质量相同或更低的 WEB-DL 种子资源

**合集打包规则**：
- 电影类只允许发行商官方原盘合集，允许衍生品（DIY/Remux/Encode）
- 发布官方原盘合集及其衍生建议简介加蓝光合集封面
- **严禁**导演/演员/IMDb Top 250/豆瓣 Top 250 等私人合集（违反可能处罚）
- 禁止跨季不同分辨率打包发布（VCB 豁免）

### W.4 制种和转种规则

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/torrent-transfer`

**制种总则**：
- 禁止发布非官方的超分处理/补帧资源
- 违规但有价值的资源可联系管理组破例
- 制种时不要加入广告文件、病毒、木马、种中种、无关文件
- 不允许发布涉及暴恐/肢解/虐待/色情/政治的违法资源
- 文件名/目录名控制在 100 中文/200 英文之内
- 文件和文件夹名字不可包含特殊符号（斜杠、单双引号），点号不可出现在末尾
- 分块大小控制在 16MB 以下
- 即使单文件也推荐套一层文件夹制种
- 文件夹名字不要用"新建文件夹"等摆烂名

**转种总则**：
- 不准转载禁转种
- 已有完结的种子禁止发布同质单集资源
- **禁止转载发布未完结分集** — 未完结剧集只接受增量包，不接受分集转载
- 禁止转载非官方超分/补帧资源
- 禁止转载黑名单制作组资源
- 不建议转载找不到出处的资源（可加 -NoGroup，自担责任）
- 不建议转载灰名单制作组资源
- 禁止转载简易机翻资源
- 请直接上传原种以便辅种
- 编辑需要须在简介注明增补内容（"添加字幕"/"修正名字"不是合理编辑需要）
- 标题和副标题需符合本站规范
- 务必写明种子出处
- 原出处简介推荐用 `[quote]资源简介[/quote]`
- 建议把预览图下载后重新传到网站附件或图床
- 源站简介无法达到分区标准则按要求补充

### W.5 发布和做种规则

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/publish-seed`

**压缩包总则**：
- 禁止使用 zstd 等非常见压缩格式（以标准版 7-zip 能解压为限）
- 禁止带密码
- 禁止"假装是 zip 但只能以特定压缩软件解压"的格式（如快压/好压）
- 不推荐 RAR5 格式（会导致一系列问题）
- 不推荐 zip 格式（Unicode 支持不好）
- 超过 4GB 的压缩包建议分卷（非硬性要求）
- RAR 恢复记录请设置在 5% 以下

**发布总则**：
- 标题请使用 0day 命名法
- 副标题请填写任何有助于搜索的信息（多语言译名、作品特征 tag）
- 必须有至少 1 张预览图（建议用海报，禁止用缩略图作第一张）
- 注意第一张预览图会作为海报加载到首页

**做种总则**：
- 上传者必须实际拥有所上传的文件
- 上传速度至少 300KB/s（故意低速将警告甚至封禁）
- 做种时间须满足免费期 48 小时
- 在其他人完成前（完成数≥3）撤种将处罚
- 发布后基本保证出种（做种数≥3 持续 12h）前，非做种状态须在 24h 以内
- 7 天内无人下载允许撤种

**资源打包规则**：
- 禁止将视频打压缩包发布
- 软件类资源请打压缩包（特别是小文件极多的情况）

### W.6 分区发布规则

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/category-rules`

**所有视频分区**：
- 压制资源须在 MediaInfo 栏提供完整 MediaInfo
- 蓝光原盘须在 MediaInfo 栏提供 BDInfo（建议 quick summary）
- 正确选择媒介、视频编码、音频编码、分辨率、制作组
- 标题使用 0day 命名法（参照标题规范）
- **副标题可包含**：片名中译（建议用豆瓣名，可含民间名）| 片名原名（如日语名）| 包含内容 | 其他有价值信息 | 槽点
- 简介须包含：资源引用 + 海报 + 介绍（可用 PTGen）+ **至少 3 张视频截图**
- **简介中禁止包含 MediaInfo**（原种有也需删除）
- 禁止发布过短视频（抖音短视频合集，短剧除外）

**音乐区（暂不审核）**：
- 禁止发布有损音乐
- 允许合集（歌手/公司/组合/社团/作曲者），禁止个人精选
- 标题无格式要求，包含码率、作者、专辑名即可
- 简介须有预览图（如专辑封面）
- Log/频谱图至少含一项（优先 Log）

**其他区（暂不审核）**：
- 游戏/软件/电子书等
- 转种可用原标题，非转种自行处理
- 软件/游戏务必带版本号，零碎文件打压缩包
- 严禁夹带私货

**其他问题**：
- 管理员可能根据规范编辑种子信息
- 请勿改变或去除管理员修改
- 不规范种子可能被删除

### W.7 黑名单和灰名单

> 来源：`https://wiki.qingwapt.org/docs/rules/content-rules/blacklist`

**黑名单（禁止转载）**：

| 制作组 | 原因 |
|--------|------|
| DBD-Raws | 盗用资源、超分、劣迹斑斑 |
| Skymoon/天月/HKACG | 反华组 |
| c.c动漫 | 改名组 |
| 猎户发布组/orion origin、爪爪字幕组/ZhuaZhuaStudio | 机翻组 |
| 盗转/改名发布组 | FGT/NSBC/BATWEB 等 28+ 组（详见审核脚本） |

**灰名单（不建议转载）**：

| 制作组 | 原因 |
|--------|------|
| 异域11番小队、加刘景长 | 低码率 |
| Reinforce | 高体积渣画质 |

### W.8 流媒体厂商缩写名（参考数据）

> 来源：`https://wiki.qingwapt.org/docs/guides/content-creation/streaming`

共 170+ 条流媒体缩写映射，以下列出常见项：

| 流媒体 | 标题写法 |
|--------|---------|
| Amazon/Prime Video | AMZN |
| Apple TV+ | ATVP |
| BBC iPlayer | iP |
| Crunchy Roll | CR |
| Disney+ | DSNP |
| HBO Max | HMAX |
| HBO | HBO |
| Hulu | HULU |
| iTunes | iT |
| Netflix | NF |
| Paramount+ | PMTP |
| Peacock | PCOK |
| Showtime | SHO |
| Star+ | STRP |
| Starz | STZ |

### W.9 常见碟片发行商（参考数据）

> 来源：`https://wiki.qingwapt.org/docs/guides/content-creation/distributor`

| 发行商 | 系列/品牌 | 写作 | 中文名 | 简写 |
|--------|----------|------|--------|------|
| The Criterion Collection | - | Criterion | 标准收藏 | CC |
| Eureka Entertainment | Masters of Cinema | - | 电影大师 | MoC |
| Warner Bros. | Warner Archive Collection | - | 华纳档案 | WAC |

以下一般不出现在标题，但建议在副标题注明：Arrow Films（箭影）、Curzon（人造眼）、BFI（英国电影协会）、Vinegar Syndrome（醋酸综合征）、Shout! Factory（尖叫工厂）、Studio Canal（映欧嘉纳）、Kino Lorber 等。

### W.10 盒子规则

> 来源：`https://wiki.qingwapt.org/docs/rules/download-rules#盒子规则`

| 规则 | 说明 |
|------|------|
| 无限制 | 站点目前对盒子的使用**没有任何限制**，也无需报备 |
| 未来可能变化 | 不排除将来对盒子有所限制的可能性 |
| 禁止大量跳车 | 无论家宽还是盒子，大量跳车行为不允许 |
| 禁止恶意限速 | 无论家宽还是盒子，恶意限速行为不允许 |
| 分享率上限 | 账号等级提升有分享率上限限制，过于依赖短期盒子产生高上传会影响等级提升 |

#### 下载规则摘要（同页面采集）

| 项目 | 规则 |
|------|------|
| 分享率 | 要求参考账号规则，过低会禁止下载权限/警告/封禁 |
| H&R | 目前**未开启** H&R 考核，但建议尽可能多保种 |
| 新种促销 | 发布后 **2 天之内为 Free**（不计下载量） |
| 禁用客户端 | **Transmission 3.x 禁用**，需升级至 4.0.5+ |
| 推荐客户端 | qBittorrent 或 Transmission（不推荐最新版） |

> **对 PT-Forward 的影响**：青蛙站对盒子无限制，PT-Forward 转发行为不受盒子规则约束。但需注意 Transmission 版本限制——如果 PT-Forward 使用 Transmission 做种，版本必须在 4.0.5 以上或使用 qBittorrent。
