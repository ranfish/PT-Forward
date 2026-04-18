# 蝴蝶 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 蝴蝶|
| 站点地址 | https://zeus.hamsters.space |
| 站点框架 | NexusPHP（深度定制） |
| 特殊功能 | **32 个分类**（按地区+类型细分）、黑名单制作组多、严格的 Dupe 规则 |
| 规则页面 | topicid=5211（总规则）、30138（纪录片）、30100（电影）、29664（剧集）、28759（综艺）、28174（音乐） |

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

*数据来源: upload.php HTML (18099字节) + forums.php 规则页 6 篇 (2026-04-17)*
*文档创建: 2026-04-17*
