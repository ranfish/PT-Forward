# MonikaDesign 站点适配器设计

> MonikaDesign 站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | MonikaDesign |
| 站点地址 | https://monikadesign.uk |
| 站点框架 | UNIT3D (Laravel) |
| 特殊规则 | 聚焦日本动画/电影/剧集/ACG 音乐；按内容类型分别有独立发布细则（动画/日本电影/日剧/现场演出/无损音乐）；动画不接受 Remux；黑名单发布组制度；命名规范极为详细 |
| 发布页面 | `https://monikadesign.uk/upload/{category_id}` |
| 提交地址 | POST `https://monikadesign.uk/upload` (multipart/form-data) |
| Tracker | 发布页右侧显示 Tracker URL |
| 规则页面 | `pages/1`(站点规则) / `pages/4`(发布指南) / `pages/8`(动画细则) / `pages/12`(日本电影) / `pages/13`(日剧) / `pages/14`(现场演出) / `pages/15`(无损音乐) |
| 黑名单 | `pages/23`(发布组黑/白名单) |
| 认证方式 | Cookie (monikadesign_session / XSRF-TOKEN / remember_web_*) |
| Cloudflare | 有防护，直连可能被拦截，需 Cookie 认证 |

---

## 一、站点规则（General Rules）

> 数据来源: `pages/1` 站点规则

### 1.1 主要规则

1. 管理组对规则有最终解释权
2. 禁止抨击站点、侮辱他人、阴阳怪气、散布谣言
3. **禁止交易、共享账号或出售、互换邀请** → 永久封禁
4. 禁止创建马甲账号
5. 严禁公开用户私人信息
6. 不活跃账号可能被自动修剪
7. 禁止交易、出售、分享或赠送账户
8. 禁止作弊
9. 禁止利用漏洞
10. **禁止将内部组资源发布到公网 BT/论坛/网盘**，除非原发布者准许
11. **尊重发布者意愿**：包括禁止/限制转载、REMUX、DIY
12. **禁止出售从 MonikaDesign 获取的资源**

### 1.2 做种要求

- H&R 系统目前**处于关闭状态**
- 所有种子至少做种 **2 天（48 小时）**
- 下载文件 < 10% 总体积时不计算 H&R
- Free 种子 H&R 规则仍然适用
- 停止做种超 24h + 未达最低时间 → 48h 内恢复可避免警告
- 停止做种超 72h + 未达最低时间 → 直接 H&R 警告

### 1.3 H&R 警告阶梯

| 生效中警告数 | 后果 |
|-------------|------|
| 1 | 无事发生 |
| 2 | 禁用下载和悬赏请求 |
| 3 | **自动封禁账号** |

> 每次警告持续 **14 天**后自动过期，记录永久保留。

### 1.4 盒子规则

- **本站目前对盒子无任何限制**

### 1.5 转载规则

- 转载资源原则上**不得修改、增加、删除文件**；必须修改时，须在简介内说明理由
- 转载资源应**保留原发布者的说明或注意事项**
- 转载资源应确保为**非盗转内容**，同时应**取得原发布者的转载许可**
- 如果转载了其他网站的独占内容，种子将被删除并可能警告甚至封禁

---

## 二、发布页面表单字段

> 数据来源: `upload/8` 页面采集 (2026-04-23 Cookie 认证)
> 表单: method=POST, enctype=multipart/form-data, action=`https://monikadesign.uk/upload`

### 2.1 完整字段列表

| 字段名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `torrent` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 主标题 |
| `subhead` | text | - | 副标题 |
| `nfo` | file | - | NFO 文件 |
| `cover` | file | - | 封面图片 (JPG/JPEG，音乐种无 TMDB 时使用) |
| `banner` | file | - | Banner 图片 (JPG/JPEG，音乐种无 TMDB 时使用) |
| `category_id` | select | ✓ | 分类 (11 个选项) |
| `type_id` | select | ✓ | 规格 (11 个选项) |
| `resolution_id` | select | ✓ | 分辨率 (14 个选项) |
| `season_number` | text | 条件 | 季数 (多季合集填 0) |
| `season_title` | text | - | 季标题 |
| `episode_number` | text | 条件 | 集数 (合集填 0) |
| `distributor_id` | select | 条件 | 发行商 (181 个选项，仅原生原盘) |
| `region_id` | select | 条件 | 发行地区 (21 个选项，仅原生原盘/非日版 CD) |
| `tmdb_id` | text | ✓ | TMDB ID (必须填写) |
| `mal_id` | text | 条件 | MAL ID (动画必须填写) |
| `bgm_id` | text | - | Bangumi ID |
| `imdb_id` | text | 条件 | IMDB ID (强烈建议) |
| `tvdb_id` | text | - | TVDB ID (选填，不填填 0) |
| `keywords` | text | 条件 | 关键词 (原盘专属) |
| `subtitle_tag` | text | - | 字幕标签 |
| `description` | textarea | - | 描述 (BBCode) |
| `mediainfo` | textarea | 条件 | MediaInfo (非 BD 原盘影视必填) |
| `bdinfo` | textarea | 条件 | BDInfo (Full Disc 必填) |
| `anonymous` | checkbox | - | 匿名发布 |
| `exclusive` | checkbox | - | 独占 |
| `scans_included` | checkbox | - | 附扫图 |
| `cds_included` | checkbox | - | 附 CD |
| `personal_release` | checkbox | - | 自购自抓自扫自压 |
| `auto_approve` | checkbox | - | 自动审批 |
| `_token` | hidden | ✓ | CSRF Token |

### 2.2 分类（category_id）— 11 个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 8 | Anime TV | 动画剧集，以 TMDB 分类为准 |
| 6 | Anime Movie | 动画电影，以 TMDB 分类为准 |
| 2 | TV | 日剧/纪录片，以 TMDB 分类为准 |
| 1 | Movie | 日本电影/纪录片，以 TMDB 分类为准 |
| 9 | Music of TV | 影视相关音乐(TV)，以 TMDB 分类为准 |
| 3 | Music of Movie | 影视相关音乐(Movie)，以 TMDB 分类为准 |
| 7 | Anime Live | 动画形式 Live 演唱会 |
| 5 | Action Live | 真人形式 Live 演唱会 |
| 4 | Game | 游戏（暂未开放） |
| 11 | Airing Anime TV | 放送中动画（新番自动发布机器人使用） |

### 2.3 规格（type_id）— 11 个

#### 视频规格

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | Full Disc | 原生原盘、DIY 原盘、RBD |
| 2 | Remux | 原盘重新封装为 MKV |
| 3 | Encode | 原盘压制 |
| 4 | WEB-DL | 流媒体下载 |
| 5 | WEBRip | WEB-DL 压制 |
| 6 | HDTV | 录制档及其重编码 |

#### 音频规格

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 7 | ALBUM | 专辑 |
| 14 | SINGLE | 包括 OP/ED 等 |
| 15 | OST | 原声大碟 |
| 16 | DRAMA | 广播剧等 |

### 2.4 分辨率（resolution_id）— 14 个

#### 视频分辨率

| 值 | 显示名称 |
|----|----------|
| 1 | 4320p |
| 2 | 2160p |
| 3 | 1080p |
| 4 | 1080i |
| 5 | 720p |
| 6 | 576p |
| 7 | 576i |
| 8 | 480p |
| 9 | 480i |
| 10 | Other |

#### 音频质量

| 值 | 显示名称 |
|----|----------|
| 11 | Lossless |
| 12 | Hi-Res |
| 13 | Lossy |

### 2.5 发行商（distributor_id）— 181 个

> 太多不逐一列出。主要包含：Aniplex / Bandai Visual / KADOKAWA / Kyoto Animation / Pony Canyon / Sony Music / Toei / Toho / Crunchyroll / FUNimation / Sentai Filmworks / Criterion / Arrow / BFI / Muse(木棉花) / 采昌 / 洲立影視 / 镭射 / 安樂影片 / 普威尔 等。
>
> **仅原生原盘需要填写。**

### 2.6 发行地区（region_id）— 21 个

| 值 | 显示名称 |
|----|----------|
| 112 | JPN (日本) |
| 229 | USA (美国) |
| 78 | GBR (英国) |
| 45 | CHN (中国) |
| 95 | HKG (香港) |
| 223 | TWN (台湾) |
| 118 | KOR (韩国) |
| 73 | FRA (法国) |
| 80 | GER (德国) |
| 108 | ITA (意大利) |
| 14 | AUS (澳大利亚) |
| 243 | EUR (欧洲) |
| 33 | BRA (巴西) |
| 41 | CEE (中东欧) |
| 69 | ESP (西班牙) |
| 200 | MEX (墨西哥) |
| 201 | NLD (荷兰) |
| 184 | RUS (俄罗斯) |
| 226 | UKR (乌克兰) |

> **仅原生原盘 & 非日版 CD 需要填写。**

---

## 三、动画发布细则（Anime Upload Rules）

> 数据来源: `pages/8`

### 3.1 总体要求

- 发布后至少做种 **3 天**，或直到有 **3 个以上做种者**
- 发布后至少在 **48h 内**开始做种，否则删除种子
- 转载不得修改文件，须保留原发布说明，须为非盗转
- **动画应仅以日本动画为主**，不建议国产/韩国/欧美动画

### 3.2 内容要求

- Anime 包括 "Anime TV" 和 "Anime Movie" 两个分类（以 TMDB 为准）
- 声优访谈、广播剧、真人电影应发布至 Movie 或 Music 分类
- **禁止一切 H Anime（成人）内容**

### 3.3 允许的片源

| 片源 | 说明 |
|------|------|
| Full Disc | Untouched Blu-ray / HD DVD / DVD / Modded Blu-ray / RBD |
| Encode | 以 Full Disc 或 Remux 为源的重编码 |
| WEB-DL / WEBRip | 流媒体下载及其重编码 |
| HDTV / HDTVRip | HDTV/UHDTV 录制及其重编码 |

### 3.4 禁止的片源

| 类型 | 说明 |
|------|------|
| 带广告的 | 含广告内容 |
| 低质量重编码 | Portable/mini encodes |
| 以重编码为源的重编码 | Encode of Encode |
| 不合理的超分/补帧 | unreasonable upscale/interpolation |
| 低质量翻译 | 如猎户不鸽组、爪爪字幕组等 |
| 无日语配音的日本动画 | 特殊情况可申请 |
| **黑名单内的发布组** | 见 §七 |
| **Remux** | **动画不再接受 Remux** |
| 非白名单 PT 组 Encode | 白名单外国内 PT 压制组（PTP 金种例外） |
| 已有 JPN BD 的 WEB-DL/HDTV | 例外：画质高于 BD 或含 BD 没有的画面 |

### 3.5 视频编码要求

| 格式 | 编码 |
|------|------|
| Full Disc | MPEG-2 / VC-1 / AVC / HEVC |
| Encode/WEBRip/HDTVRip | x264 / x265 / AV1 |
| WEB-DL/HDTV | H264 / H265 / VP9 / MPEG-2 |
| DVD 原盘 | 省略 |

### 3.6 音频编码

允许: LPCM / DTS:X / DTS-HD MA / DTS-ES / DTS-HD HR / DTS / TrueHD (Atmos) / DD+ (E-AC-3) (Atmos) / DD (AC-3) (EX) / FLAC / ALAC / AAC / OPUS

### 3.7 取代与共存规则

| 片源 | 规则 |
|------|------|
| Full Disc | 不同版本共存（须含独特部分）；高质量取代旧种；合集取代分卷 |
| Encode | 全部共存，除非 REPACK/RERIP、Rev 关系、单季→多季合并 |
| WEBRip | 全部共存，除非 REPACK、合集取代分集、Encode 取代 WEBRip |
| WEB-DL | **仅保留质量最高的一个**（画质>日语音轨>中字>国粤台配音） |
| HDTV | **仅保留质量最高的一个** |
| HDTVRip | 全部共存，类似 WEBRip |

### 3.8 简介要求

- **UNIT3D 架构不需要在描述内填写**: 海报、豆瓣/bangumi 简介、MediaInfo、BDInfo
- 自购内容须提供证明
- 转载须保留原始发布说明
- Full Disc: 建议商品图 + 简介信息
- Encode: 原创须提供对比截图 + MediaInfo + 原盘来源
- 字幕组作品允许不提供截图

### 3.9 命名规范

**通用模板**：
```
Name [Year] S##E## [Cut] [Hybrid] [REPACK|RERIP] [PROPER] Resolution [CROPPED] [Region] [3D] Source [TYPE] [HDR] [Hi10P] VideoCodec AudioCodec Channels[-Tag]
```

**名词解释**：

| 字段 | 说明 |
|------|------|
| Name | MAL 罗马音主标题，含标点 |
| Year | Anime Movie 必填；多部用 起始年-终止年 |
| S##E## | 详见站内 Wiki |
| Resolution | 1080p/2160p 等；DVD 原盘不写 |
| Source | BluRay / UHD BluRay / DVDRip / 流媒体缩写 |
| TYPE | WEB-DL/WEBRip/HDTV（Full Disc/Encode 省略） |
| HDR | HDR/HDR10+/DV 等（SDR 省略） |
| Hi10P | 10bit AVC 时填写 |
| Audio Codec | 以最高质量音轨为准 |
| Channels | 1.0/2.0/5.1/7.1 等 |
| Tag | 发布组或发布者名；匿名用 Anonymous@Site |

---

## 四、日本电影/剧集发布细则

> 数据来源: `pages/12`(日本电影) / `pages/13`(日剧)

### 4.1 日本电影（Movie 分类）

- 包含日本制作的电影、纪录片、舞台剧、真人版动画、声优访谈
- 禁止 H 内容
- **允许 Remux**（与动画不同）
- 允许的片源：Full Disc / Remux / Encode / WEB-DL / WEBRip / HDTV
- 禁止：带广告、低质量重编码、以重编码为源的重编码、机翻、黑名单发布组

**命名模板**（Full Disc）：
```
Name [AKA Original] Year [Cut] [Hybrid] [PROPER] Resolution [Edition] [Region] [3D] SOURCE [TYPE] [HDR] VideoCodec AudioCodec Channels[-Tag]
```

**命名模板**（Remux/Encode/WEB-DL/HDTV）：
```
Name [AKA Original] Year [S##E##] [Cut] [Hybrid] [REPACK|RERIP] Resolution [Edition] SOURCE [TYPE] [HDR] [Hi10P] VideoCodec AudioCodec Channels [Object] [Hardsubs]-Tag
```

### 4.2 日剧（TV 分类）

- 包含日本电视剧、纪录片剧集、真人版动画、声优访谈
- 与电影规则基本一致
- S##E## 语法详细定义了分集/分卷/分Part/合集等格式
- 多季合集时 Season Number 和 Episode Number 均**填 0**

---

## 五、现场演出发布细则

> 数据来源: `pages/14`

- Live 应与 ACG 相关或 ACG 歌手相关；允许包含日本影视主题曲的 Live
- Anime Live（动画形式）/ Action Live（真人形式）
- 特典 CD 不强制要求 Log 文件
- 命名使用**发售日期**（YYMMDD 格式）替代年份

---

## 六、无损音乐发布细则

> 数据来源: `pages/15`

### 6.1 内容要求

- 仅包括与影视资源或 ACG 相关的音乐
- 广播剧等纯音频内容也发布于无损音乐分类
- Music of TV / Music of Movie 两个分类（以 TMDB 为准）
- **禁止盗版、饭制品等非官方发行资源**
- **禁止发布 ASMR**
- **禁止发布音乐合集**（官方发行套装除外）
- Game 分类开放前暂不开放 Music of Game

### 6.2 来源要求

| 允许 | 说明 |
|------|------|
| Full Disc | CD / SACD 等实体发行 |
| WEB | Mora 等 Hi-Res 音源；广播剧允许最高质量（无损或有损均可） |

| 禁止 | 说明 |
|------|------|
| 有损音乐 | - |
| 假无损 | - |
| 盗版/饭制品 | 非官方发行 |

### 6.3 格式要求

- Full Disc: WAV / FLAC / APE / TAK 等无损格式
- CD **要求有 Log 文件**（特典 CD 不强制）
- 整轨 CD 要求有 **UTF-8 编码的 CUE 文件**

### 6.4 音乐命名规范

```
艺人/发行公司 - 专辑名 品番 发售日期 音质 文件类型 [版本]
```

范例：
- `Aimer - 残響散歌/朝が来る VVCL-1953-4 220112 44.1kHz 16bit FLAC+CUE+LOG+ISO+PNG [初回生産限定盤]`
- `Aimer - 残響散歌/朝が来る 220112 96kHz 24bit FLAC+Cover`

### 6.5 发布页填写说明

| 字段 | 音乐发布要求 |
|------|-------------|
| 规格 | SINGLE(OP/ED/角色歌) / ALBUM(精选集) / OST / DRAMA |
| 分辨率 | Lossless / Hi-Res / Lossy |
| 发行商 | 留空 |
| 发行地区 | 仅实体发行物需填写，日版可省略 |
| TMDB/IMDB/TVDB/MAL | 有相关影视作品时填写，无则填 0 |
| Mediainfo | 仅当包含非 BDMV 视频文件时填写 |
| BDInfo | 仅当包含 BDMV 时填写 |

---

## 七、发布组黑名单与白名单

> 数据来源: `pages/23`

### 7.1 国内 PT 压制组白名单（仅 Anime 受此限制）

- PTer
- WiKi

### 7.2 公网压制组黑名单

- DBD-Raws

### 7.3 字幕组黑名单

- c.c 动漫
- 猎户发布组 (-orion origin)
- 爪爪字幕组 (-ZhuaZhuaStudio)

---

## 八、关键适配器设计要点

### 8.1 UNIT3D 架构特点

- 不需要在描述内填写海报、豆瓣简介、MediaInfo、BDInfo（各有独立字段）
- 描述使用 BBCode
- 需要 CSRF Token（`_token` 字段）
- 提交后种子进入**审核队列**

### 8.2 分类选择策略

- 以 **TMDB 分类为准**：动画电影→Anime Movie，动画剧集→Anime TV
- 日本电影→Movie，日剧→TV
- 音乐按关联影视的 TMDB 分类选 Music of TV 或 Music of Movie
- Live 按动画/真人选 Anime Live / Action Live

### 8.3 TMDB / MAL ID 必填

- **TMDB**: 必须填写
- **MAL**: 动画必须填写（非动画 bug 时填 0）
- **IMDB**: 强烈建议填写
- **TVDB**: 选填（不填填 0）

### 8.4 动画特殊规则

- **不接受 Remux**（动画分类）
- **不接受非白名单 PT 组的 Encode**（仅 PTer/WiKi）
- **不接受已有 JPN BD 的 WEB-DL/HDTV**（除非画质更高）
- 裁剪黑边须在名称添加 CROPPED 后缀
- 命名使用 MAL 罗马音

### 8.5 独占/禁转检查

- 转载须确保为**非盗转**
- 须**取得原发布者转载许可**
- 转载独占内容→删种+可能警告/封禁

### 8.6 发布后做种

- 至少 3 天或有 3+ 做种者
- 48h 内必须开始做种，否则删种

---

## 九、全局转载策略对 MonikaDesign 的影响

| 规则 | MonikaDesign 情况 | PT-Forward 处理 |
|------|-------------------|-----------------|
| §30.5 禁止 9KG/成人 | 明确禁止 H Anime/H 内容 | 源站带成人标签→不转发 |
| §30.5 禁止禁转/独占 | 严禁盗转，需获原发布者许可 | 源站带禁转标签→不转发 |
| TMDB 必填 | TMDB ID 必须，MAL 动画必须 | 转发前须查询 TMDB/MAL |
| 动画命名规范 | 极详细的 0DAY 命名规则 | 按规范重写标题 |
| 动画禁止 Remux | 动画分类不接受 Remux | 动画 Remux 不转发 |
| 发布组限制 | 动画 Encode 仅白名单 PT 组 | 非白名单组的动画 Encode 不转发 |
| 描述不含海报/简介 | UNIT3D 有独立字段 | 描述仅保留发布说明 |
| MediaInfo 独立字段 | 不在描述中填写 | 从源站提取 MediaInfo 填入独立字段 |
| 黑名单发布组 | 3 个字幕组 + 1 个公网组 | 检查源站发布组是否在黑名单 |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-23*
*数据来源：monikadesign.uk pages/1,4,8,12,13,14,15,23 + upload/8 表单 + forums/topics/288*
