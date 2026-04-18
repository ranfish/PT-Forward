# 碟粉 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 碟粉|
| 站点地址 | https://discfan.net |
| 站点框架 | NexusPHP |
| 内容定位 | 华语资源为主、韩日为辅、欧美为补充；原盘/源码为佳，原创压制作辅 |
| 特殊功能 | Cloudflare 防护、地区细分分类（中国大陆/香港/台湾/泰国/日本/韩国/世界）、独立 Wiki |
| 规则页面 | rules.php + Wiki（https://wiki.discfan.net/zh/rule/post） |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 模式系统

使用 `data-mode='4'` 属性控制字段显示，字段名带 `[4]` 后缀。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.3 类型字段（`type`，data-mode=4）— 13个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 - 中国大陆 |
| 404 | 电影 - 中国香港 |
| 405 | 电影 - 中国台湾 |
| 402 | 电影 - 泰国 |
| 403 | 电影 - 日本 |
| 406 | 电影 - 韩国 |
| 410 | 电影 - 世界 |
| 411 | 剧集 |
| 414 | 音乐 |
| 413 | 纪录 |
| 416 | 综艺 |
| 417 | 体育 |
| 419 | 动漫 |

**独特设计**: 电影按**地区细分**为7个子分类（大陆/香港/台湾/泰国/日本/韩国/世界），这是已采集站点中唯一的地区分类模式。

### 1.4 来源字段（`source_sel[4]`）— 11个

**注意**: DiscFan 使用 `source_sel`（来源）代替常见的媒介/编码/分辨率/制作组字段。**这是唯一的质量字段**。

| 值 | 显示名称 |
|----|----------|
| 1 | HDTV |
| 2 | 4K UltraHD |
| 3 | Blu-ray Disc |
| 4 | DVD |
| 5 | SDTV |
| 6 | VCD |
| 7 | LD |
| 8 | VHS |
| 9 | Web-DL |
| 10 | Rip |
| 11 | Book |
| 131 | Remux |

**独特设计**: 
- 无 `medium_sel`、`codec_sel`、`audiocodec_sel`、`standard_sel`、`team_sel` 字段
- 来源字段混合了媒介类型和分辨率（4K UltraHD=2 含分辨率信息）
- 有复古媒介：LD(7)、VHS(8)——已采集站点中唯一包含镭射影碟和录像带的
- 有 VCD(6)、SDTV(5)、Book(11)
- Remux=131 是异常大值
- "Rip"(10) 含义不明，可能泛指重编码

### 1.5 标签（`tags[4][]`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 粤语 |
| 9 | 自购 |
| 10 | DoVi |

**注意**: 值3缺失。有**粤语**(8)——与站点港剧定位一致。有**自购**(9)和**DoVi**(10)。

---

## 二、字段映射汇总（实际发布用）

### 2.1 类型（`type`）

```json
{
  "电影-中国大陆": 401,
  "电影-泰国": 402,
  "电影-日本": 403,
  "电影-中国香港": 404,
  "电影-中国台湾": 405,
  "电影-韩国": 406,
  "电影-世界": 410,
  "剧集": 411,
  "纪录": 413,
  "音乐": 414,
  "综艺": 416,
  "体育": 417,
  "动漫": 419
}
```

### 2.2 来源（`source_sel[4]`）

```json
{
  "HDTV": 1,
  "4K UltraHD": 2,
  "Blu-ray Disc": 3,
  "DVD": 4,
  "SDTV": 5,
  "VCD": 6,
  "LD": 7,
  "VHS": 8,
  "Web-DL": 9,
  "Rip": 10,
  "Book": 11,
  "Remux": 131
}
```

### 2.3 标签（`tags[4][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR": 7,
  "粤语": 8,
  "自购": 9,
  "DoVi": 10
}
```

---

## 三、发种规则（Wiki + 用户补充）

### 3.1 发布总则

- 发布者必须对发布的文件拥有合法的传播权
- 发布者必须保证上传速度与保证出种（至少一个人完成下载），或故意低速上传将被警告甚至封禁
- 发布者获得双倍上传量
- **内容定位**：以华语资源为主、日韩等亚洲资源为辅、其他欧美海外资源为补充；格式以原盘/源码为佳，原创压制为辅，其他格式为补充
- 亚洲资源可直接发布，不需要先提交候选区；其他如欧美等地区需先提交候选区，等待管理员通过或投票通过

### 3.2 发布资格

- 任何人都能发布资源
- 乞丐(Peasant)和凡人(User)需在候选区提交候选
- 炼气(Power User)及以上用户可直接发布允许的资源
- 当用户通过的候选数 ≥ 5 时，可直接发布种子，无需经过候选

### 3.3 候选通过条件

- 当候选支持票比反对票多5票时通过
- 候选添加72小时后未被通过则被删除
- 候选通过后24小时内未发布种子，通过的候选将被删除

### 3.4 允许的资源

- **五年以前的资源**（如2024年可发布2019年及以前资源）
- 视听资源：电影、电视剧、纪录片、综艺、演唱会等；媒介如 BD、LD、VCD、DVD、VHS、WEB-DL、HDTV 等
- 电子书
- 音乐专辑

### 3.5 禁止的资源

- **五年内的资源**（如2024年不可发布2020年及以后的资源）
- **涉及政治、儿童色情、AV 以及各种反人类、反社会的资源**
- 其他站点禁止转载的资源
- 标清 upscale 视频
- CAM、TC、TS、SCR、DVDSCR、R5、R5.Line、HalfCD 等低质量视频
- RealVideo/RMVB、flv 文件
- 单独的样片
- 未达 5.1 声道的有损音频（MP3、WMA 等）
- 无正确 cue 的多轨音频
- **游戏资源**
- 重复（dupe）资源
- 损坏文件、垃圾文件

### 3.6 Dupe 判定规则

- 优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 高清版本使标清版本被判定为 dupe
- **动漫类特例**：HDTV 和 DVD 相同优先级
- 保留一个 DVD5 大小的重编码版本
- 不同区域/配音/字幕的 Blu-ray/HD DVD 不视为 dupe
- 每个无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- **断种45日或已发布18个月**以上不受 dupe 约束

### 3.7 重要时间限制

DiscFan 有独特的**五年规则**：
- 允许发布：五年以前的资源
- 禁止发布：五年内的资源
- 这意味着站点主要收录经典/旧资源，非新资源

---

## 四、DiscFan 特殊注意事项

### 4.1 五年规则

站点独特的**五年规则**：仅允许发布五年以前的资源。适配器在转发到 DiscFan 时需检查资源年份，拒绝五年内的资源。

### 4.2 禁止游戏和 AV

明确禁止游戏资源、AV、涉及政治的内容。发布前需过滤这些类型。

### 4.3 极简表单

表单**无编码/音频编码/分辨率/制作组**字段，仅有 `source_sel` 一个来源下拉。适配器发布时无需映射编码/音频等字段。

### 4.4 地区细分分类

电影按地区细分为7个子分类。适配器需根据资源地区信息（如豆瓣/TMDb 国家）选择正确分类。亚洲资源可直接发布，欧美资源需先提交候选。

### 4.5 复古媒介

包含 LD(7)、VHS(8)、VCD(6) 等复古媒介，与站点聚焦经典/旧资源的定位一致。

### 4.6 Remux 异常值

Remux=131 远超其他选项的值范围（1-11），可能因数据库历史原因。

### 4.7 Cloudflare 防护

站点使用 Cloudflare 防护，需要有效的 `cf_clearance` cookie 才能访问。

### 4.8 粤语标签

独有的**粤语**(8)标签，与站点港剧定位一致。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | DiscFan | 常见 NexusPHP |
|------|---------|---------------|
| 内容定位 | 华语为主/韩日为辅/欧美补充 | 综合影视 |
| 五年规则 | **仅允许五年以前资源** | 无时间限制 |
| 地区分类 | 电影细分7个地区 | 无 |
| 媒介字段 | **无**（用 source_sel 替代） | medium_sel |
| 编码字段 | **无** | codec_sel |
| 音频编码 | **无** | audiocodec_sel |
| 分辨率 | **无**（4K UltraHD 含在来源中） | standard_sel |
| 制作组 | **无** | team_sel |
| 来源字段 | source_sel（11+1个） | 通常无 |
| 禁止内容 | AV、政治、游戏 | 因站而异 |
| 复古媒介 | LD/VHS/VCD | 无 |
| 标签 | 10个（含粤语/自购/DoVi） | 因站而异 |

---

## 六、适配器实现要点

### 6.1 五年规则检查

```go
func canPublishToDiscFan(resourceYear int) bool {
    currentYear := time.Now().Year()
    return resourceYear <= currentYear-5
}
```

### 6.2 内容类型过滤

```go
func isAllowedForDiscFan(req *PublishRequest) bool {
    if req.Category == "Game" || req.Category == "AV" {
        return false
    }
    if req.Year > time.Now().Year()-5 {
        return false
    }
    return true
}
```

### 6.3 地区分类映射

从资源元数据映射到 DiscFan 地区分类：

```go
func mapRegionCategory(country string, baseCategory string) int {
    if baseCategory == "Movies" {
        switch country {
        case "CN": return 401  // 电影 - 中国大陆
        case "HK": return 404  // 电影 - 中国香港
        case "TW": return 405  // 电影 - 中国台湾
        case "TH": return 402  // 电影 - 泰国
        case "JP": return 403  // 电影 - 日本
        case "KR": return 406  // 电影 - 韩国
        default:   return 410  // 电影 - 世界
        }
    }
    switch baseCategory {
    case "TV Series": return 411
    case "Documentary": return 413
    case "Music": return 414
    case "Variety": return 416
    case "Sports": return 417
    case "Animation": return 419
    default: return 410
    }
}
```

### 6.4 source_sel 映射

```go
func mapSourceSel(medium, resolution string) int {
    switch {
    case strings.Contains(resolution, "2160p") && strings.Contains(medium, "Blu-ray"):
        return 2   // 4K UltraHD
    case strings.Contains(medium, "Remux"):
        return 131  // Remux
    case strings.Contains(medium, "Blu-ray"):
        return 3   // Blu-ray Disc
    case strings.Contains(medium, "WEB"):
        return 9   // Web-DL
    case strings.Contains(medium, "HDTV"):
        return 1   // HDTV
    case strings.Contains(medium, "DVD"):
        return 4   // DVD
    case strings.Contains(medium, "Encode"):
        return 10  // Rip
    default:
        return 10  // Rip
    }
}
```

### 6.5 Cloudflare cookie 处理

访问 DiscFan 需要有效的 `cf_clearance` cookie，适配器需支持 cookie 刷新机制。

---

*数据来源: upload.php HTML (514行) + Wiki 规则页 (73行) (2026-04-16)*
*文档创建: 2026-04-16*
