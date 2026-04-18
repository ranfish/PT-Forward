# 不可羊 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 不可羊|
| 站点地址 | https://tjupt.org |
| 站点框架 | NexusPHP |
| 定位 | 教育网 PT 站（天津大学） |
| 特殊规则 | **极简发布表单**（无媒介/编码/分辨率/制作组选择），蓝光原盘/Remux 需候选审核 |

---

## 一、发布页面表单字段分析

**提交地址**: POST multipart/form-data

**字段名无后缀**（裸名）。

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `external_url` | text | - | **辅助填写**（IMDb/豆瓣/TMDB/Steam/Indienova/Epic/Bangumi 链接，自动生成简介和IMDb链接） |
| `url` | text | - | IMDb 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `uplver` | checkbox | - | 匿名发布 |

注意：有 `external_url` 辅助填写字段，支持 7 种来源（IMDb/豆瓣/TMDB/Steam/Indienova/Epic/Bangumi），是已分析站点中最多的。通过 PT-Gen 后端自动生成简介。

### 1.2 类型选择（`type`）

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 资料 |
| 405 | 动漫 |
| 406 | 音乐 |
| 407 | 体育 |
| 408 | 软件 |
| 409 | 游戏 |
| 410 | 其他 |
| 411 | 纪录片 |
| 412 | 移动视频 |

注意：包含"资料"(404)、"软件"(408)、"移动视频"(412)独特分类。

### 1.3 特性 checkbox（仅3个）

| 字段名 | 显示名称 | 说明 |
|--------|----------|------|
| `exclusive` | 禁止转载 | 禁止转载 |
| `response` | 应求发布 | 求种应求 |
| `chinese` | 中文字幕 | 华语影视不勾选 |

### 1.4 ⚠️ 无质量选择字段

TJUPT 的发布表单**完全没有**以下字段：
- `medium_sel` — 无媒介选择
- `codec_sel` — 无视频编码选择
- `audiocodec_sel` — 无音频编码选择
- `standard_sel` — 无分辨率选择
- `team_sel` — 无制作组选择
- `source_sel` / `processing_sel` — 无地区/来源选择
- `tags[]` — 无标签选择
- `pt_gen` — 无 PTGen 链接
- `technical_info` — 无 MediaInfo 输入框

这是已分析的 **15+ 个站点中表单字段最少的**。所有资源质量信息通过标题和简介传达，不做结构化选择。

---

## 二、标题命名规范

来源：论坛帖子

### 2.1 电影标题格式

```
Titanic 1997 720p BluRay x264 DTS-WiKi
│      │    │    │      │    │   │
│      │    │    │      │    │   └── 压制组
│      │    │    │      │    └───── 音频编码
│      │    │    │      └────────── 视频编码
│      │    │    └───────────────── 压制来源
│      │    └────────────────────── 分辨率
│      └─────────────────────────── 年份
└────────────────────────────────── 英文片名
```

### 2.2 命名要求

- 保持完整的原始文件名称（0Day 名）
- 严禁篡改和伪造 0Day 名
- 禁止带有"求种"、"求置顶"、"求推荐"等主观信息
- 文件格式可在简介中标注（蓝光原盘、REMUX、DVD 原盘等）

### 2.3 WEB-DL 标题

WEB-DL 来源需标明平台（iTunes/AMZN/NF 等），最好提供 source 信息的 NFO。

---

## 三、发布规则

### 3.1 允许的资源

- 高清视频（蓝光重编码仅允许 MKV 封装）
- WEB-DL（非大陆仅接受正规小组，国产仅接受特定组）
- 蓝光原盘/Remux（**全部需候选审核**）
- NFO 文件、Sample 文件、独立音轨

### 3.2 禁止的资源

- RMVB/RM/MP4/AVI/FLV/MOV/3GP 等非 MKV 封装（国产 WEB-DL 允许 MP4）
- 720p 以下蓝光重编码（720p 原则 ≥4000kbps，1080p ≥8000kbps）
- 电影合集（套装售卖除外）
- NC-17 / III 级 / R18 / 18 级影片
- 韩版硬字幕
- 禁转资源
- WEBRip（WEB-DL 质量优于 Bluray 时的 2160p→1080p WEBRip 除外）
- 二次压制的 WEB-DL

### 3.3 黑名单制作组

**禁止的小组**：
SeeHD、Mp4Ba、ZJM、SmY、ZYM、3LT0N、VeryPSP、DWR、PRLxXunlei、RARBG、FGT、BeiTai、DBD

**禁止的片源**：
TC、TS、MiniSD、HDRip、HDTVRip、WEBrip、WEBSCR、DVDSCR、TCRip、HRBD、HR-HDTV、R5、RC、CAM、VCD、DVDR、SCR、MNHD、MicroHD、FULLCD、HalfCD、upscaled

### 3.4 Dupe 规则

- **0Day 名一致即认定为重复**
- 有 Proper/Repack/ReRIP 版本时，原版本删除
- 蓝光原盘/Remux：每部电影不同分辨率各仅接收一版（不接收无中字版本）
- WEB-DL：同分辨率同大小只保留最先发布版本
- **Bluray dupe WEB-DL**：蓝光重编码发布后，WEB-DL 版本不再保留（保留最高画质一版）

### 3.5 时效规则

- 正在上映的国产电影禁止发布（上映期一个月）
- 蓝光未发售前禁止发布低质量版本

### 3.6 促销规则

- 蓝光 Remux → 永久 50%
- 1080p → 永久 50%
- 发布者注明禁转的资源未经同意禁止转载

---

## 四、站点适配器配置参考

```yaml
site:
  id: "tjupt"
  name: "北洋园PT"
  alt_name: "TJUPT"
  url: "https://tjupt.org"
  framework: "nexusphp"
  upload_url: "upload.php"
  wiki_url: "https://tjupt.org/forums.php?action=viewtopic&forumid=5&topicid=3762"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "资料": 404
      "动漫": 405
      "音乐": 406
      "体育": 407
      "软件": 408
      "游戏": 409
      "其他": 410
      "纪录": 411
      "移动视频": 412

    checkboxes:
      exclusive: "exclusive"
      response: "response"
      chinese: "chinese"

  field_names:
    suffix: ""
    external_url: "external_url"
    anonymous: "uplver"

  missing_fields:
    - "medium_sel"
    - "codec_sel"
    - "audiocodec_sel"
    - "standard_sel"
    - "team_sel"
    - "source_sel"
    - "processing_sel"
    - "tags"
    - "pt_gen"
    - "technical_info"

  blacklist_groups:
    - "SeeHD"
    - "Mp4Ba"
    - "ZJM"
    - "SmY"
    - "ZYM"
    - "3LT0N"
    - "VeryPSP"
    - "DWR"
    - "PRLxXunlei"
    - "RARBG"
    - "FGT"
    - "BeiTai"
    - "DBD"

  blacklist_sources:
    - "TC"
    - "TS"
    - "MiniSD"
    - "HDRip"
    - "HDTVRip"
    - "WEBrip"
    - "WEBSCR"
    - "DVDSCR"
    - "TCRip"
    - "HRBD"
    - "HR-HDTV"
    - "R5"
    - "RC"
    - "CAM"
    - "VCD"
    - "DVDR"
    - "SCR"
    - "MNHD"
    - "MicroHD"
    - "FULLCD"
    - "HalfCD"
    - "upscaled"

  quirks:
    minimal_form: "无任何质量选择字段，信息通过标题和简介传达"
    external_url_7_sources: "辅助填写支持7种来源（IMDb/豆瓣/TMDB/Steam/Indienova/Epic/Bangumi）"
    only_3_checkboxes: "仅3个checkbox：禁止转载/应求发布/中文字幕"
    bluray_candidate_only: "蓝光原盘/Remux全部需候选审核"
    0day_name_dupe: "0Day名一致即认定为重复"
    bluray_dupe_webdl: "蓝光重编码发布后WEB-DL不保留"
    mkv_only: "蓝光重编码仅允许MKV封装"
    bitrate_minimum: "720p≥4000kbps，1080p≥8000kbps"
    education_site: "教育网PT站（天津大学）"
```

---

## 五、发布流水线注意事项

### 5.1 极简表单

TJUPT 是已分析站点中表单最简单的——**只有文件上传、标题、副标题、类型选择和简介**。无需填写媒介、编码、分辨率、制作组等结构化字段。

### 5.2 标题质量

因为无结构化字段，所有资源信息通过 0Day 标准标题传达。转种时必须保证标题格式正确。

### 5.3 辅助填写

`external_url` 字段支持 7 种来源，可自动生成简介和 IMDb 链接。转种时应优先使用此功能。

### 5.4 蓝光原盘/Remux 限制

所有蓝光原盘和 Remux 资源**必须通过候选审核**，不能直接发布。

### 5.5 WEB-DL 特殊规则

- 非大陆仅接受正规小组的 iTunes/AMZN/NF 源
- 国产仅接受 RS/NYHD/TTG/OurTV/CMCTV/CHDWEB 等特定组
- 不收 HaresWEB
- 蓝光发布后 WEB-DL 会被删除（保留最高画质一版）

---

*分析时间：2026-04-16*
*数据来源：https://tjupt.org/upload.php + 论坛帖子 topicid=3762/55/56/16548*
