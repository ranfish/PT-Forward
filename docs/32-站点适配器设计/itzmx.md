# 分享站 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 分享站|
| 站点地址 | https://pt.itzmx.com |
| 站点框架 | NexusPHP（极简定制） |
| 特殊功能 | CDN 防护（acw_tc/cdn_sec_tc）、仅目标站 |
| 规则页面 | rules.php |

**站点角色**: 无官组，**只能做目标站（发布站），不能做源站**。

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 极简表单

**这是已采集站点中字段最少的站点**，无模式系统、无 `data-mode` 属性。

### 1.2 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `descr` | textarea | ✓ | 简介（BBCode） |

**重要缺失**:
- **无 `url`（IMDb）字段**
- **无 `pt_gen` 字段**
- **无 `nfo` 文件上传**
- **无 `technical_info`（MediaInfo）字段**
- **无 `uplver`（匿名发布）字段**
- **无标签（tags）字段**

### 1.3 类型字段（`type`）— 8个

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 404 | 漫画 |
| 405 | 动画 |
| 408 | 音乐 |
| 410 | 软件 |
| 411 | 游戏 |
| 414 | 蓝光 |

**注意**: 含漫画(404)、软件(410)、游戏(411)、蓝光(414)——"蓝光"作为独立分类而非媒介。无纪录片/综艺/体育分类。

### 1.4 分辨率（`standard_sel`）— 3个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 720p |
| 3 | 2160p |

**注意**: 仅3个选项，是已采集站点中最少的。无 `data-mode` 属性，字段名无 `[]` 后缀。无 SD/1080i/4320p。

### 1.5 制作组（`team_sel`）— 1个

| 值 | 显示名称 |
|----|----------|
| 1 | Other |

**注意**: 仅1个选项 "Other"，确认无官组。字段名无 `[]` 后缀。

### 1.6 缺失字段汇总

以下常见字段在本站**完全不存在**：
- `medium_sel`（媒介）
- `codec_sel`（编码）
- `audiocodec_sel`（音频编码）
- `tags[]`（标签）
- `url`（IMDb）
- `pt_gen`（PT-Gen）
- `nfo`（NFO 文件）
- `technical_info`（MediaInfo）
- `uplver`（匿名发布）
- `source_sel`（来源）
- `processing_sel`（处理/地区）

---

## 二、发种规则（rules.php）

### 2.1 上传总则

- 上传者必须对上传的文件拥有合法的传播权
- 做种时间不足24小时或故意低速上传将被警告甚至取消上传权限
- **发布者获得和其他用户同等对待的上传量**（非双倍）
- 违规种子不经提醒直接删除

### 2.2 上传者资格

- 任何人都能发布资源
- 游戏类资源只有上传员及以上等级可自由上传

### 2.3 允许的资源

- 高清视频
- 7日内高清预告片
- 标清重编码（来源于高清媒介）

### 2.4 不允许的资源

- CAM/TC/TS/SCR 等低质量视频
- 单独的样片
- 重复（dupe）资源
- 垃圾文件

### 2.5 Dupe 规则

- 优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 高清版本使标清版本被视为 dupe
- 按发布组确定优先级
- 不同区域/配音/字幕的 Blu-ray/HD DVD 不视为 dupe
- 断种45日或已发布18个月以上不受 dupe 约束

### 2.6 种子促销

- 随机促销（自动）
- 文件总体积 > 50GB 自动成为免费&2x上传

---

## 三、字段映射汇总（实际发布用）

### 3.1 类型（`type`）

```json
{
  "电影": 401,
  "电视剧": 402,
  "漫画": 404,
  "动画": 405,
  "音乐": 408,
  "软件": 410,
  "游戏": 411,
  "蓝光": 414
}
```

### 3.2 分辨率（`standard_sel`）

```json
{
  "1080p": 1,
  "720p": 2,
  "2160p": 3
}
```

### 3.3 制作组（`team_sel`）

```json
{
  "Other": 1
}
```

---

## 四、ITZMX 特殊注意事项

### 4.1 仅目标站

无官组，只能做目标站。不能做源站。在 PT-Forward 中应标记为 `SourceEnabled=false`。

### 4.2 已采集站点中最简表单

仅5个可填写字段（file/name/small_descr/descr/type）+ 2个下拉（standard_sel/team_sel）。是所有已采集站点中字段最少的。

### 4.3 无媒介/编码/音频/标签

发布时无需选择媒介、视频编码、音频编码，也无需标签。这大幅简化了适配器的映射逻辑。

### 4.4 无 IMDb/PT-Gen/MediaInfo/匿名

跳过所有这些字段的填写。

### 4.5 "蓝光"作为分类

蓝光(414) 作为独立分类而非媒介选项，与其他站点的分类体系完全不同。

### 4.6 发布者非双倍上传

与其他站点不同，发布者获得**同等**上传量而非双倍。

### 4.7 2160p 值=3

分辨率值排列异常：1080p=1, 720p=2, 2160p=3。2160p 的值最小却在列表最后。

### 4.8 CDN 防护

站点使用 CDN 防护（acw_tc/cdn_sec_tc cookie），需有效的安全 cookie 才能访问。

---

## 五、与其他 NexusPHP 站点对比

| 特征 | ITZMX | 常见 NexusPHP |
|------|-------|---------------|
| 站点角色 | **仅目标站** | 源站/目标站 |
| 表单字段数 | **7个（最少）** | 通常 15-30 个 |
| 媒介 | **无** | medium_sel |
| 编码 | **无** | codec_sel |
| 音频编码 | **无** | audiocodec_sel |
| 标签 | **无** | tags[] |
| IMDb | **无** | url |
| PT-Gen | **无** | pt_gen |
| MediaInfo | **无** | technical_info |
| 匿名发布 | **无** | uplver |
| 分辨率 | 3个（最少） | 通常 5-7 个 |
| 制作组 | 1个（Other） | 通常 3-30 个 |
| 发布者上传量 | **同等**（非双倍） | 通常双倍 |

---

## 六、适配器实现要点

### 6.1 极简发布

```go
func buildITZMXRequest(req *PublishRequest) url.Values {
    form := url.Values{}
    form.Set("name", req.Title)
    form.Set("small_descr", req.Subtitle)
    form.Set("descr", req.Description)
    form.Set("type", strconv.Itoa(mapType(req.Category)))
    form.Set("standard_sel", strconv.Itoa(mapResolution(req.Resolution)))
    form.Set("team_sel", "1") // Other
    return form
}
```

### 6.2 类型映射

"蓝光"(414) 作为独立分类，需特殊处理：

```go
func mapType(category string) int {
    switch category {
    case "Movies": return 401
    case "TV Series": return 402
    case "Animation": return 405
    case "Music": return 408
    case "Software": return 410
    case "Game": return 411
    default: return 401
    }
}
```

### 6.3 跳过所有缺失字段

```go
adapter.SkipIMDb = true
adapter.SkipPTGen = true
adapter.SkipMediaInfo = true
adapter.SkipAnonymous = true
adapter.SkipMedium = true
adapter.SkipCodec = true
adapter.SkipAudioCodec = true
adapter.SkipTags = true
adapter.SkipNFO = true
```

---

*数据来源: upload.php HTML (419行) + rules.php HTML (291行) (2026-04-16)*
*文档创建: 2026-04-16*
