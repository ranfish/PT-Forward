# 蝴蝶 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 蝴蝶 (HUDBT)|
| 站点地址 | https://zeus.hamsters.space |
| 站点框架 | NexusPHP（深度定制，老牌教育网站） |
| Tracker URL | https://hudbt.hust.edu.cn/announce.php |
| 特殊功能 | **32 个分类**（按地区+类型细分）、无质量下拉框、dl-url 上传方式、720p 码率标准(4000kbps)、严格的 Dupe 规则（Scene/iNT 分组） |
| 规则页面 | rules.php |
| 公告页面 | topicid=30100（电影规则）、topicid=29664（剧集规则）、topicid=30138（纪录片规则）、topicid=28759（综艺规则）、topicid=28174（音乐规则）、topicid=7277（游戏规则）、topicid=13271（学习/软件/其他）、topicid=21199（游戏视频） |

---

## 一、发布页面表单字段（upload.php）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 说明 |
|--------|----------|------|
| `file` | file | 种子文件 |
| `dl-url` | url | 下载链接（替代上传种子文件） |
| `name` | text | 主标题 |
| `small_descr` | text | 副标题 |
| `url` | text | IMDb 链接 |
| `nfo` | file | NFO 文件 |
| `descr` | textarea | 简介（BBCode） |
| `uplver` | checkbox | 匿名发布（value=yes） |

**独有字段**: `dl-url`（通过 URL 链接上传种子，替代文件上传）。**无** `pt_gen` 字段、**无** MediaInfo 独立字段、**无** 标签字段、**无** source_sel 字段、**无** medium_sel 字段、**无** codec_sel 字段、**无** audiocodec_sel 字段、**无** team_sel 字段、**无** processing_sel 字段。

### 1.2 分类（`type`）— 32个

**按地区+类型双重细分**，是已采集站点中分类最多的之一：

#### 电影（5个）

| 值 | 显示名称 |
|----|----------|
| 401 | 大陆电影 |
| 413 | 港台电影 |
| 414 | 亚洲电影 |
| 415 | 欧美电影 |
| 430 | iPad |
| 433 | 抢先视频 |

#### 剧集（4个）

| 值 | 显示名称 |
|----|----------|
| 402 | 大陆剧集 |
| 417 | 港台剧集 |
| 416 | 亚洲剧集 |
| 418 | 欧美剧集 |

#### 综艺（4个）

| 值 | 显示名称 |
|----|----------|
| 403 | 大陆综艺 |
| 419 | 港台综艺 |
| 420 | 亚洲综艺 |
| 421 | 欧美综艺 |

#### 音乐（5个）

| 值 | 显示名称 |
|----|----------|
| 408 | 华语音乐 |
| 422 | 日韩音乐 |
| 423 | 欧美音乐 |
| 424 | 古典音乐 |
| 425 | 原声音乐 |

#### 动漫（4个）

| 值 | 显示名称 |
|----|----------|
| 405 | 完结动漫 |
| 427 | 连载动漫 |
| 428 | 剧场OVA |
| 429 | 动漫周边 |

#### 其他（10个）

| 值 | 显示名称 |
|----|----------|
| 404 | 纪录片 |
| 407 | 体育 |
| 406 | 音乐MV |
| 409 | 其他 |
| 432 | 电子书 |
| 410 | 游戏 |
| 431 | 游戏视频 |
| 411 | 软件 |
| 412 | 学习 |
| 426 | MAC |
| 1037 | HUST |

### 1.3 分辨率（`standard_sel`）— 8个

| 值 | 显示名称 | 说明 |
|----|----------|------|
| 1 | 1080p | |
| 2 | 1080i | |
| 3 | 720p | |
| 4 | SD | 标清 |
| 6 | Lossy | 有损音频 |
| 7 | 2160p/4K | |
| 5 | Lossless | 无损音频 |

**注意**: 含 `Lossy`(6) 和 `Lossless`(5) 用于音乐分类，分辨率和音频质量共用同一字段。

---

## 二、发种规则摘要

### 2.1 标题命名规范

**主标题**（英文，用空格连接，禁止 `[]` 和 `.` 连接）：

```
# 电影
英文名 年代 分辨率 介质 音频 编码-制作组

# 剧集（大陆/港台/亚洲）
英文名 播出时间 分辨率 介质 音频 编码-制作组

# 欧美剧集
英文名 S季E集 分辨率 介质 音频 编码-制作组

# 综艺
英文名 播出时间(YYYYMMDD) 分辨率 介质 音频 编码-制作组

# 无损音乐
艺术家名 - 专辑名 发行年代 音频格式

# 有损音乐
艺术家名 专辑名 发行日期 音频格式 码率
```

**副标题**（简体中文，可加入补充信息，用空格连接，禁止 `.` 连接）：

```
中文名 / 其他语言名 可加入补充信息
```

### 2.2 Dupe（重复）规则

**电影**：
- 不同分辨率之间不构成 Dupe
- WEB-DL/HDTV 与 Blu-ray 构成 Dupe，保留 Blu-ray
- 断种超过 3 个月允许重新发布

**剧集**：
- 完结后合集发布 → 单集变 Dupe 并删除
- 高清版本不使已存在的标清版本构成 Dupe
- 同介质高清只保留一个版本

**音乐**：
- 同专辑不同格式构成 Dupe，保留先发布的
- 不同版本（如欧版/大陆版）不构成 Dupe
- 无损与有损不构成 Dupe

### 2.3 黑名单制作组

以下制作组的资源**禁止发布**：

| 制作组 | 禁止范围 |
|--------|----------|
| CnSCG, WOFEI, CNXP | 有水印的 720p 电影 |
| Verypsp | iPad 视频 |
| CkreleaSe | WEBRip, DVDRip |
| xiabd | 任何资源 |
| PublicHD | 重编码 |
| EVO, Mp4Ba, SeeHD, FGT | 任何资源 |
| STUTTERSHIT 等 | 韩语硬字幕资源 |

### 2.4 禁止的资源格式

- RMVB, RM, FLV, 3GP, ASF, XV（无压制组信息的低质量视频）
- HR-HDTV（半高清）
- 腾讯视频/爱奇艺/优酷 WEBRip
- 分卷压缩包
- 单曲（音乐类必须整张专辑，≥100MB）

### 2.5 制作种子规则

- 文件夹不能包含 `*.torrent`, `*.url`, `*.txt` 等无关文件
- 不能含海报封面
- 外挂字幕（srt/ass/ssa）不能放在资源文件夹，需单独上传
- idx+sub 字幕可与资源一同做种
- 不允许分卷形式做种

---

## 三、Hamster 特殊注意事项

### 3.1 无质量下拉框

Hamster 的 upload.php **没有** medium_sel、codec_sel、audiocodec_sel、team_sel、processing_sel、source_sel 等质量相关下拉框。这是已采集站点中极罕见的——所有质量信息仅通过分类选择和标题命名传达。

### 3.2 dl-url 字段

提供 `dl-url` URL 字段作为种子文件的替代上传方式。适配器可选使用文件上传或 URL 提交。

### 3.3 分类地区细分

电影/剧集/综艺均按**地区**（大陆/港台/亚洲/欧美）细分，是已采集站点中最细的地区分类。适配器需要根据资源地区信息选择正确的分类。

### 3.4 严格的黑名单

黑名单涵盖 FGT/Mp4Ba/EVO/PublicHD 等常见公网制作组，源站消费时需过滤。

### 3.5 分类包含 HUST

分类中包含 `HUST`(1037)，可能是站点特殊分类（华中科技大学相关？）。

---

## 四、与其他 NexusPHP 站点对比

| 特征 | Hamster | 常规 NexusPHP |
|------|---------|---------------|
| 分类 | **32个（地区+类型细分）** | 通常 5-15 个 |
| 质量下拉框 | **无**（全靠标题） | 通常 5-7 个字段 |
| 分辨率 | 7个（含 Lossy/Lossless） | 通常 4-6 个 |
| 地区细分 | **分类内地区区分** | 部分站有 processing_sel |
| 黑名单 | FGT/Mp4Ba/EVO/PublicHD 等 | 因站而异 |
| dl-url | **有**（URL 上传） | 无 |
| 媒介/编码/音频下拉 | **无** | 通常有 |

---

## 五、适配器实现要点

### 5.1 地区分类映射

```go
func mapHamsterCategory(standardCat string, region string) int {
    switch standardCat {
    case "Movie":
        switch region {
        case "CN":    return 401  // 大陆电影
        case "HK/TW": return 413  // 港台电影
        case "AS":    return 414  // 亚洲电影
        default:      return 415  // 欧美电影
        }
    case "TV/Series":
        switch region {
        case "CN":    return 402  // 大陆剧集
        case "HK/TW": return 417  // 港台剧集
        case "AS":    return 416  // 亚洲剧集
        default:      return 418  // 欧美剧集
        }
    case "TV/Show":
        switch region {
        case "CN":    return 403  // 大陆综艺
        case "HK/TW": return 419  // 港台综艺
        case "AS":    return 420  // 亚洲综艺
        default:      return 421  // 欧美综艺
        }
    case "Doc":          return 404
    case "Sport":        return 407
    case "Music/Video":  return 406
    case "Anime/Done":   return 405
    case "Anime/Airing": return 427
    case "Anime/OVA":    return 428
    case "Audio/CN":     return 408
    case "Audio/JP/KR":  return 422
    case "Audio/US/EU":  return 423
    case "Audio/Classic":return 424
    case "Audio/OST":   return 425
    default:             return 409
    }
}
```

### 5.2 无质量字段的简化上传

```go
func (a *HamsterAdapter) Upload(req *PublishRequest) error {
    payload := map[string]string{
        "name":        req.Title,       // 主标题（英文）
        "small_descr": req.Subtitle,    // 副标题（中文）
        "url":         req.IMDbURL,     // IMDb 链接
        "descr":       req.Description,  // BBCode 简介
        "type":        fmt.Sprintf("%d", mapHamsterCategory(req.Category, req.Region)),
        "standard_sel": mapHamsterStandard(req.Resolution),
        "uplver":      "yes",
    }
    // 只需 type + standard_sel，无需 medium/codec/audio/team
}
```

---

---

## 六、完整站点规则（rules.php + 论坛公告）

> 数据来源：rules.php（6889字符）+ 8 个论坛公告帖子

### 6.1 账号保留规则

| 等级/状态 | 保留条件 |
|-----------|----------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上（封存） | 封存后不会被删除 |
| 封存账号 | 连续 240 天不登录删除 |
| 未封存账号 | 连续 120 天不登录删除 |
| 无流量用户 | 连续 30 天不登录或注册满 60 天删除 |
| 分享率不达标警告 | 15 天内提升分享率，否则禁用 |

### 6.2 促销规则

- Blu-ray Disk 原盘 → **30%**
- 电视剧每季第一集 → **永久免费**
- 热门资源由管理员设置为限时免费
- 不定期全站免费活动
- **注意**：不允许一个种子夹杂 HUDBT Tracker 与其他 PT 站 Tracker 一起上传（视为作弊，直接 Ban）

### 6.3 上传总则

- 做种时间不足 24h 或故意低速上传将被警告甚至取消上传权限
- **发布者获得 1.5 倍上传量**（非双倍）
- <100MB 资源原则上应发到网盘共享区
- 需解压才能使用的资源应用文件夹直接发布；免安装软件用压缩包发布

### 6.4 制作种子规则

- 文件夹不能含 `*.torrent`、`*.url`、`*.txt` 等无关文件
- 不能以分卷形式做种
- 不要把海报封面放在资源文件夹中
- srt 字幕勿放在资源文件夹，须单独上传；idx+sub 字幕可与资源一同做种
- **例外**：CMCT 压制组文件夹里有海报或图片，无需移除，保持原样

---

## 七、电影 Dupe 细则（#30100 最新版）

> 数据来源：topicid=30100（2017-04-23 起）

### 7.1 基本规则

- **不同分辨率之间互不重复**
- **断种超过 3 个月**，允许另行发布其他制作组版本或重发原版本
- **制片方宣布系列电影彻底完结前禁止不断更新合集**

### 7.2 制作组分类

| 类别 | 代表制作组 |
|------|-----------|
| Scene (0day) | SPARKS, GECKOS, AMIABLE, DRONES, SiNNERS 等 |
| iNT 国内组 | EPiC, MTeam, WiKi, beAst, HDChina, CHD 等 |
| iNT 国外组 | CtrlHD, TayTO, DON, Ebp, IDE, ZQ, decibeL, HiFi, BMF, D-Z0N3, NCmt, HANDJOB 等 |

### 7.3 Encode 720P

- **Blu-ray 版本可同时存在多个**
- WEB-DL/HDTV 版本与 Blu-ray 版本构成 Dupe，只保留 Blu-ray
- 当 Blu-ray 保种版本确定后，其他版本断种 3 个月后人为删种

### 7.4 Encode 1080P

- **可同时存在多个 Blu-ray 版本**

### 7.5 其他

- 原盘/Remux 各允许一个版本
- iPad 视频仅允许一个版本
- 标清：DVDrip 和 MiniSD 不互为 Dupe，但每个版本只留一个压制组

### 7.6 720P 码率标准

| 类型 | 最低码率 |
|------|----------|
| 普通 720p | **4000 kbps**（可放宽至 3000 kbps） |
| iPad 720p | **2000 kbps** |

例外：Scene 组 0day（定体积超长电影）、动画电影、WEB-DL

### 7.7 合集规则

- 允许：连续性合集（哈利波特、魔戒等）
- 禁止：IMDB TOP250、李小龙合集等无关联合集
- 合集中分辨率+制作组须全部相同
- 新合集不与已存在单集构成 Dupe，但已存在于合集中的单集构成 Dupe

---

## 八、剧集 Dupe 细则（#29664）

### 8.1 通用规则

- **完结后合集发布 → 单集变 Dupe 并删除**
- 只允许一个 SD 版本
- **已存在高清版本时不允许发布标清**，但后发高清不会使先发标清变 Dupe
- iPad 版本只保留第一个
- 同分辨率合集：WEB-DL/HDTV 与 Blu-ray 不构成 Dupe，HDTV 和 WEB-DL 各择优保留一个
- 同介质合集：高清各只保留一个版本

### 8.2 欧美剧集特殊规则

- Youtube 版本 WEB-DL 不适用
- Scene 组 PROPER/RERIP 使该组之前版本构成 Dupe
- **仅允许整季打包，不允许冬歇半季合集，不允许未完结多季合集**
- 合集质量须相同且介质编码一致，允许不同 Scene 制作组

### 8.3 亚洲/港台/大陆剧集特殊规则

- 合集必须来自同一录制组/压制组/字幕组
- 无正规 HDTV/WEB-DL 源时允许字幕组二压合集
- 无标清资源时特允许 HR-HDTV 半高清打包发布，无其他资源亦允许 RM 打包
- 接受发布组原打包形式（两集/四集包），不允许另行打包或拆包

---

## 九、音乐发布规则（#28174）

### 9.1 允许格式

| 类型 | 允许格式 |
|------|----------|
| 无损 | WAV 整轨/分轨、DTS 整轨、**FLAC 分轨**、ALAC 分轨、AIFF 分轨 |
| 有损 | MP3（≥320kbps）、AAC（≥256kbps） |
| 镜像 | SACD、DVD-A、BD-AUDIO（音频编码限 LPCM/TrueHD/DTS-HD MA） |
| 蓝光 | BDMV 文件夹格式 |

### 9.2 禁止格式

- **FLAC 整轨、APE、TTA**（动漫周边类可放宽）
- OGG、WMA、M4A 等低码率
- <320kbps 的 MP3、<256kbps 的 AAC
- 无正确 cue/m3u 的整轨音频
- **所有音乐不允许压缩包发布**
- 单曲或 <100MB 专辑
- 个人编辑作品（自选集、混音、从视频提取音频等）

### 9.3 Dupe 规则

- 同专辑不同格式构成 Dupe，**保留先发布的版本**
- 不同版本（欧版/大陆版）不构成 Dupe
- 无损与有损不构成 Dupe
- 断种 3 个月以上可不受 Dupe 约束

---

## 十、综艺发布规则（#28759）

### 10.1 HDTV 录制源限制

仅允许：**CHDTV / NGB / HDWTV / BYRTV** 等高清录制源作品或以此的一次压制版本

### 10.2 WEB-DL 限制

仅允许官方电视台在 **Youtube** 或 **iTunes** 发布的高清版本。**禁止**腾讯视频/爱奇艺/优酷源及其传至 Youtube 的版本

### 10.3 Dupe 规则

- 只允许一个 SD 版本；已有高清则禁止发标清
- 高清(720p+1080i)各只保留第一个版本
- HDTV 和 WEB-DL 不构成 Dupe，各保留一个
- 日韩综艺有字幕和无字幕不构成 Dupe
- 已有合集则不再接受单集

---

## 十一、纪录片发布规则（#30138）

- Blu-ray 版本可同时存在多个
- WEB-DL/HDTV 与 Blu-ray 构成 Dupe，只保留 Blu-ray
- 国外纪录片：确定保种版本与含国语音轨版本不构成 Dupe
- **仅允许整季打包，不允许多季合集**
- 完结后单集与合集构成 Dupe，删除单集

---

## 十二、游戏发布规则（#7277）

### 12.1 Dupe 规则

- 同一游戏最多允许 0day 光盘镜像版 + 中文硬盘版两个版本
- 正式版后 Beta 版视为多余
- 零售光盘破解版后，Steam 解锁版视为多余
- 修正破解光盘版后，原光盘版视为多余
- 硬盘版只取一个版本

### 12.2 促销政策

| 体积 | 优惠 |
|------|------|
| 0-5GB | 无优惠 |
| 5-10GB | 50% |
| >10GB | 30% |

---

## 十三、学习/软件/其他发布规则（#13271）

### 13.1 软件类

- 截图必有
- **Windows 系统仅允许发布原版**，修改版/Ghost 版不允许
- 软件 ISO/MDF/NRG 等以镜像原样发种，勿解压
- 主标题用英文，含：软件名+版本号+语言+架构+破解方式

### 13.2 学习及其他类

- 发布课件须经老师同意
- 电子书合集须给出所有书名作为目录
- 不推荐发布合集类资源
- 禁止"四六级收藏""考研大全"等模糊标题

---

## 十四、电影黑名单制作组（完整版）

| 制作组 | 禁止范围 | 来源 |
|--------|----------|------|
| CnSCG, WOFEI, CNXP | 有水印的 720p 电影 | #13622/#30100 |
| BMDruCHinYaN, YYets | 清晰度介于高清与标清之间 | #13622 |
| Verypsp | iPad 视频 | #13622/#30100 |
| CkreleaSe | WEBRip, DVDRip | #13622/#30100 |
| xiabd | 任何资源 | #13622/#30100 |
| PublicHD | 重编码 | #13622/#30100 |
| EVO, Mp4Ba, SeeHD | 任何资源 | #13622/#30100 |
| FGT | 任何资源 | #30100 |
| STUTTERSHIT 等 | 韩语硬字幕资源 | #30100 |

---

*数据来源: upload.php HTML (18099字节) + rules.php (6889字符) + 论坛#30100/#29664/#30138/#28759/#28174/#7277/#13271/#21199/#27100/#13622 (2026-04-17/2026-04-22)*
*文档创建: 2026-04-17*
*文档更新: 2026-04-22 — 补充 Tracker URL、完整 rules.php、电影/剧集/纪录片/综艺/音乐/游戏/学习各类发种细则、Scene/iNT Dupe 规则、720p 码率标准、音乐格式限制、综艺 HDTV 录制源限制、黑名单制作组完整版、合集规则*
