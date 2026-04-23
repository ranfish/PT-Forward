# 幸运 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 幸运 |
| 域名 | pt.luckpt.de |
| 框架 | NexusPHP |
| Cloudflare | 是 |
| 候选制 | 部分（游戏资源需候选） |
| MediaInfo | 是（technical_info） |
| IMDb | 是（url，data-pt-gen） |
| 豆瓣 | 是（通过 PT-Gen） |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| PT-Gen | 是（支持 IMDB / Douban / Bangumi / Indienova） |
| LuckAudit | 是（**预审核系统**，自动驳回+人工复核） |

## Tracker URL
`https://tracker.luckpt.de/announce`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | **最小 1GB** |
| 标题 | `name` | 是 | 0DAY 命名规范 |
| 副标题 | `small_descr` | 是 | LuckAudit 检查非空 |
| IMDb链接 | `url` | 否 | data-pt-gen="url" |
| PT-Gen | `pt_gen` | 否 | data-pt-gen="pt_gen" |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode，要求至少 2 张图片 |
| MediaInfo | `technical_info` | 是 | LuckAudit 强制检查格式完整性 |
| 类型 | `type` | 是 | data-mode='4' |
| 媒介 | `medium_sel[4]` | 是 | |
| 编码 | `codec_sel[4]` | 是 | |
| 分辨率 | `standard_sel[4]` | 是 | |
| 音频编码 | `audiocodec_sel[4]` | 是 | |
| 制作组 | `team_sel[4]` | 否 | |
| 标签 | `tags[4][]` | 是 | checkbox 多选 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 电视剧 |
| 405 | 动画 |
| 406 | MV |
| 408 | 音乐 |
| 409 | 其他 |
| 410 | 综艺 |
| 411 | 纪录片 |
| 412 | 体育 |
| 413 | 短剧 |

## 质量字段

### 媒介 medium_sel[4]（13 个）

| ID | 名称 |
|----|------|
| 1 | Blu-ray |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVD |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | UHD Blu-ray |
| 11 | WEB-DL |
| 13 | Other |

### 编码 codec_sel[4]（8 个）

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 2 | AV1 |
| 3 | VC-1 |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265/HEVC |
| 12 | MPEG-4/XviD |

### 分辨率 standard_sel[4]（8 个）

| ID | 名称 |
|----|------|
| 1 | 1080p/1080i |
| 3 | 720p/720i |
| 4 | 480p/480i |
| 5 | 2K/1440p/1440i |
| 6 | 4K/2160p/2160i |
| 7 | 8K/4320p/4320i |
| 8 | Other |

### 音频编码 audiocodec_sel[4]（16 个）

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | DD/AC3 |
| 11 | TrueHD Atmos |
| 12 | DDP/E-AC3 |
| 13 | LPCM |
| 14 | TrueHD |
| 15 | DTS:X |
| 16 | DTS-HD MA |
| 17 | M4A |
| 18 | WAV |
| 19 | PCM |

### 制作组 team_sel[4]（9 个）

| ID | 名称 | 备注 |
|----|------|------|
| 5 | Other | |
| 7 | LuckWeb | 站组 |
| 8 | LuckMusic | 站组 |
| 9 | FRDS | |
| 10 | StarfallWeb | |
| 11 | LuckAni | 站组 |
| 12 | LuckDIY | 站组 |
| 13 | LuckDocu | 站组 |

### 缺失字段

- **无 source_sel**
- **无 processing_sel**

## 标签 tags[4][]（17 个）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 9 | 连载 |
| 10 | 完结 |
| 11 | 大包 |
| 14 | 粤语 |
| 16 | 特效 |
| 17 | 合集 |
| 18 | HDR10+ |
| 19 | HDR10 |
| 20 | Dolby Vision |
| 21 | 菁彩HDR (Vivid HDR) |
| 22 | 英语 |
| 23 | 原创 |

## LuckAudit 预审核系统

### 概述

LuckAudit 是幸运站的**自动预审核系统**，嵌入在 upload.php 页面中，通过外部 JS (`js/luckaudit.js`) 加载。发布种子时会自动调用预审核 API (`https://admin.luckpt.de/api/torrents/pre-audit`)，对种子信息进行评分。**未通过预审核的种子会被自动驳回**，需修改后重新提交或申请人工审核。

### 评分机制

- **满分 100 分**，需达到 100 分才能通过自动审核
- 每个检查项发现问题时**扣减对应分数**
- 存在 ERROR/WARNING 级别问题时无法通过

### 审核级别

| 级别 | 图标 | 说明 |
|------|------|------|
| ERROR | 🚫 | 错误，必须修复 |
| WARNING | ⚠️ | 警告，需要处理 |
| SUSPICIOUS | 🔍 | 可疑，需关注 |

### 已发现的审核规则

#### 基础字段规则 (ruleType: SUBTITLE)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `SUBTITLE_EMPTY` | 副标题为空 | ERROR | 5 | 副标题不能为空，需含中文名/类型/演员等 |

#### 分类规则 (ruleType: CATEGORY)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `CATEGORY_NOT_SELECTED` | 未选择分类 | ERROR | 10 | 必须选择分类 |

#### 技术字段规则 (ruleType: TECHNICAL)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `TYPE_NOT_SELECTED` | 未选择媒介 | ERROR | 8 | 必须选择媒介类型 |
| `ENCODE_NOT_SELECTED` | 未选择编码 | ERROR | 8 | 必须选择编码 |
| `AUDIO_NOT_SELECTED` | 未选择音频 | ERROR | 8 | 必须选择音频编码 |
| `RESOLUTION_NOT_SELECTED` | 未选择分辨率 | ERROR | 8 | 必须选择分辨率 |

#### 标签规则 (ruleType: TAGS)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `TAGS_EMPTY` | 未选择任何标签 | ERROR | 6 | 需人工复核 |
| `TAGS_WRONGLY_SELECTED_CHINESE_SUBTITLE` | 错误勾选中字标签 | ERROR | 10 | MediaInfo 中未检测到中文字幕但勾选了中字 |

#### 简介内容规则 (ruleType: CONTENT)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `DESCRIPTION_MISSING_LINKS` | 简介缺少 IMDb/豆瓣/TMDB 链接 | ERROR | 6 | 简介中必须包含外部信息链接 |
| `DESCRIPTION_MISSING_BASIC_INFO` | 简介缺少影片基础信息 | WARNING | 5 | 简介内容过于简单 |
| `MEDIAINFO_EMPTY` | MediaInfo 为空 | ERROR | 9 | 必须填写 MediaInfo |

#### MediaInfo 规则 (ruleType: MEDIAINFO)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `MEDIAINFO_FORMAT_INVALID` | MediaInfo 格式不规范 | ERROR | 8 | 缺少必需的 Video 区块 |
| `MEDIAINFO_INCOMPLETE` | MediaInfo 信息不完整 | WARNING | 4 | 缺少 Duration/Width/Height 等字段 |

#### 图片规则 (ruleType: IMAGES)

| errorCode | message | level | 扣分 | 说明 |
|-----------|---------|-------|------|------|
| `IMAGES_INSUFFICIENT` | 图片数量不足 | ERROR | 5 | 当前: 0，要求: 至少 2 张 |

### LuckAudit API 详情

- **端点**: `POST https://admin.luckpt.de/api/torrents/pre-audit`
- **Content-Type**: `application/json`
- **请求体**: 表单收集的种子信息 JSON（含 name/small_descr/imdb_url/description/technical_info/type/quality/tags）
- **响应体结构**:
  ```json
  {
    "success": true,
    "message": "预审核完成",
    "data": {
      "passed": false,
      "status": "未通过",
      "totalScore": 62,
      "errorCount": 4,
      "warningCount": 2,
      "suspiciousCount": 1,
      "details": [
        {
          "ruleType": "TAGS",
          "errorCode": "...",
          "message": "...",
          "level": "ERROR",
          "score": 10,
          "details": "...",
          "suggestion": "..."
        }
      ],
      "suggestions": ["..."],
      "auditTime": "2026-04-19 15:20:24"
    }
  }
  ```

### 驳回与人工审核

- 未通过预审核的种子**自动驳回**
- 用户修改后可重新提交预审核
- 如确信无误，可发布后被自动驳回，然后**点击举报按钮申请人工审核**

## 货币体系

- 货币名称：**幸运星**

## 导航栏分类入口

- 官种 / 动漫 / 电影 / 电视剧 / 短剧 / 纪录片 / 综艺
- 认领上限：1000

## 发种规则摘要

### 资源大小
- **最小 500MB**（rules.php 原文：总体积小于 500MB 的资源禁止发布）

### 标题命名
- 电影: `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 电视剧: `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 音轨: `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 游戏: `[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]`

### 允许的资源
- HD 视频（720p+）：Blu-ray/HD DVD 原盘、Remux、HDTV、重编码
- SD 视频（仅限）：HD 源重编码（480p+）、DVDR/DVDISO/DVDRip
- 无损音轨+cue：FLAC/APE 等
- 5.1 声道及以上电影音轨、DTS CD 镜像等
- PC 游戏（仅限原盘镜像）
- 7 天内 HD 预告片
- HD 相关软件和文档

### 禁止的资源
- < 500MB 的资源
- **已完结剧集不允许发布分集**（有合集后删分集）
- SD 拉升视频
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD
- RMVB/RM/FLV
- 无损音轨以下的有损音频（普通 MP3/WMA）
- 无正确 cue 的多轨音频
- 硬盘版/高压缩游戏/非官方镜像/Mod/小游戏合集
- RAR 等压缩包
- 禁忌或敏感内容（色情/政治等）
- Dupe 重复资源

### Dupe 规则
- 来源优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- HD 版本使 SD 版本成为 Dupe
- 同源同分辨率重编码：按制作组优先级，高质量替代低质量
- 不同区 Blu-ray 原盘（不同配音/字幕）不算 Dupe
- 无损音频：原则上只保留一个版本，分轨 FLAC 优先级最高
- 死种 45+ 天或发布 18+ 月的资源不受 Dupe 限制

### 促销规则

**随机促销（上传时自动触发）**:
| 概率 | 类型 |
|------|------|
| 10% | 50% Download（半价） |
| 5% | Free |
| 5% | 2x Upload |
| 3% | 50% Download + 2x Upload |
| 1% | Free + 2x Upload |

**自动促销**:
- 总文件大小 > 20GB → Free
- Blu-ray/HD DVD 原盘 → Free
- 每季首集 → Free
- 发布 1 个月后永久 2x Upload

**促销时限**:
- 除 2x Upload 外所有促销限 7 天
- 2x Upload 无时限

### 副标题规则
- 禁止广告或求种内容

### 账号保留

| 等级/条件 | 规则 |
|----------|------|
| Ultimate User 及以上 | 永远保留 |
| Veteran User 及以上 | 封存账号后保留，连续 400 天不登录封禁 |
| 未封存账号 | 连续 150 天不登录封禁 |
| 无流量用户 | 连续 60 天不登录封禁 |

> 本站**没有删除账号**功能，被封禁将永远无法进入。

### 发布者上传量

- 发布者获**一倍上传量**（非双倍）

## 特殊说明

1. **LuckAudit 预审核系统**：发布页内置自动审核面板，对种子信息进行全面评分（100分制），通过 `admin.luckpt.de` 外部 API 实现
2. **Cloudflare 防护**：站点使用 Cloudflare，需 `-k --tlsv1.2` 绕过 TLS 指纹检测
3. **最小 500MB 限制**：rules.php 原文为 500MB
4. **6 个站组**：LuckWeb/LuckMusic/LuckAni/LuckDIY/LuckDocu + StarfallWeb
5. **短剧分类**：type=413 短剧分类
6. **2K 分辨率**：含 2K/1440p 选项
7. **8K 分辨率**：含 8K/4320p 选项
8. **AV1 编码**：支持 AV1(2)
9. **TrueHD Atmos**：音频含 TrueHD Atmos(11)，独立于 TrueHD(14) 和 DTS:X(15)
10. **HDR 标签三连**：HDR10(19)/HDR10+(18)/Dolby Vision(20) + 菁彩HDR Vivid(21)
11. **中字标签智能检测**：LuckAudit 会解析 MediaInfo 中的字幕语言，验证中字标签是否正确
12. **图片最低要求**：简介中至少 2 张图片（`[img]` BBCode）
13. **简介外部链接要求**：简介中必须包含 IMDb/豆瓣/TMDB 链接
14. **MediaInfo 完整性检查**：要求包含 General(Duration)/Video(Width/Height)/Audio 区块
15. **填写质量按钮**：发布页有"填写质量"按钮，可从标题自动解析填充质量字段
16. **无 source_sel、无 processing_sel**：只有标准 5 个质量下拉框

---

## LuckAudit 预审核系统深度分析

### 已发现的全部审核规则

通过多次 API 测试（`POST https://admin.luckpt.de/api/torrents/pre-audit`），已确认以下审核规则：

#### A. 基础字段规则

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `SUBTITLE_EMPTY` | 副标题为空 | ERROR | 5 | `small_descr` 为空字符串 |
| `CATEGORY_NOT_SELECTED` | 未选择分类 | ERROR | 10 | `type` 为 null 或 id="0" |
| `TAGS_EMPTY` | 未选择任何标签 | ERROR | 6 | `tags` 数组为空 |

#### B. 技术字段规则 (TECHNICAL)

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `TYPE_NOT_SELECTED` | 未选择媒介 | ERROR | 8 | `quality.medium` 缺失 |
| `ENCODE_NOT_SELECTED` | 未选择编码 | ERROR | 8 | `quality.codec` 缺失 |
| `AUDIO_NOT_SELECTED` | 未选择音频 | ERROR | 8 | `quality.audiocodec` 缺失 |
| `RESOLUTION_NOT_SELECTED` | 未选择分辨率 | ERROR | 8 | `quality.standard` 缺失 |

**注意**: 制作组 `team_sel` 未选择不触发 ERROR，仅为可选字段。

#### C. 标题-MediaInfo交叉验证规则 (MEDIAINFO_TITLE_MISMATCH)

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `MEDIAINFO_TITLE_MISMATCH` | 分辨率不匹配 | WARNING | 5 | MI 的 Width/Height 与标题中的分辨率标识(1080p/2160p等)不一致 |
| `MEDIAINFO_TITLE_MISMATCH` | HDR标识缺失 | WARNING | 5 | MI 检测到 Dolby Vision/HDR 但标题中未包含 HDR/DV 标识 |
| `MEDIAINFO_TITLE_MISMATCH` | 1080分辨率未在标题体现 | WARNING | 5 | MI 显示 1920x1080 但标题中无 1080p/1080i |

#### D. 媒介-标题交叉验证规则 (TYPE_MISMATCH)

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `TYPE_MISMATCH` | 媒介与标题不匹配 | ERROR | 7 | 标题中检测到的来源与选择的 medium 不一致 |

**已确认的标题来源检测逻辑**:
- 标题含 `REMUX` / `Remux` → 应选 Remux(3)
- 标题含 `WEB-DL` / `WEBDL` → 应选 WEB-DL(11)
- 标题含 `HDTV` → 应选 HDTV(5)
- 标题含 `BluRay` 但不含 REMUX 且不含 x264/x265/HEVC/AVC **或文件较小** → 判定为 Encode(7)
- 标题含 `BluRay` + REMUX → Remux(3)
- 标题含 `BluRay` 无 REMUX → Encode(7)（非原盘即编码）
- **重要**: 仅有 `BluRay x264-GRP` 格式时，审核 API 判定为 Encode(7)，不判定为 Blu-ray(1)

#### E. 标签智能检测规则

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `TAGS_WRONGLY_SELECTED_CHINESE_SUBTITLE` | 错误勾选中字标签 | ERROR | 10 | 勾选了中字(6)但 MI 中未检测到 Chinese 字幕 |
| `TAGS_HDR_WRONGLY_SELECTED` | DV标签错误选择 | WARNING | 5 | 勾选了 DV(20) 但 MI 中 HDR format 不含 Dolby Vision |
| `TAGS_MISSING_HDR` | HDR10标签缺失 | WARNING | 4 | MI 检测到 Dolby Vision（含 HDR10 compatible）但未勾选 HDR10(19) |

**MI 语言检测逻辑**:
- 解析 MI Text 区块的 `Language` 字段
- `Language: Chinese` / `Language: Chinese (Simplified)` / `Language: Chinese (Traditional)` → 检测为中文
- MI 中有中文 Text 区块 → 允许勾选中字标签

**MI HDR 检测逻辑**:
- `HDR format` 字段含 `Dolby Vision` → 允许 DV(20) 标签
- `HDR format` 字段含 `HDR10` → 允许/建议 HDR10(19) 标签
- `HDR format` 字段含 `HDR10+` → 允许 HDR10+(18) 标签
- DV Profile 8 含 `HDR10 compatible` → 同时建议 HDR10(19) 标签

#### F. 简介内容规则 (CONTENT)

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `MEDIAINFO_EMPTY` | MediaInfo为空 | ERROR | 9 | `technical_info` 为空 |
| `DESCRIPTION_MISSING_LINKS` | 简介缺少外部链接 | ERROR | 6 | 简介中未找到 `imdb.com` / `douban.com` / `themoviedb.org` / `tmdb.org` 链接 |
| `DESCRIPTION_MISSING_BASIC_INFO` | 简介缺少基础信息 | WARNING | 5 | 简介内容过短（< ~100字符）或缺少关键信息 |

#### G. MediaInfo 格式规则

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `MEDIAINFO_FORMAT_INVALID` | 格式不规范 | ERROR | 8 | 缺少 Video 区块 |
| `MEDIAINFO_INCOMPLETE` | General缺少Duration | WARNING | 4 | General 区块无 Duration 字段 |
| `MEDIAINFO_INCOMPLETE` | Video缺少Width | WARNING | 4 | Video 区块无 Width 字段 |
| `MEDIAINFO_INCOMPLETE` | Video缺少Height | WARNING | 4 | Video 区块无 Height 字段 |

**MI 解析逻辑**: API 按 `General` / `Video` / `Audio` / `Text` 区块解析，通过关键字段（Duration/Format/Width/Height/Channel(s)/Language/HDR format）进行验证。

#### H. 图片规则 (IMAGES)

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `IMAGES_INSUFFICIENT` | 图片数量不足 | ERROR | 5 | `[img]...[/img]` 标签数量 < 2 |

**图片计数逻辑**: 统计 description 中 `[img]` BBCode 标签出现次数。需要**完整的 URL 格式**（`[img]https://...[/img]`），而非简写（`[img]a[/img]` 不计数或超时无法处理）。

#### I. 分类特殊规则 (CATEGORY)

| errorCode | message | level | 扣分 | 检测逻辑 |
|-----------|---------|-------|------|----------|
| `MUSIC_NOT_ALLOWED` | 暂时禁止发布音乐资源 | ERROR | 100 | type=408(音乐) → 直接 0 分 |

**注意**: 音乐分类当前被完全禁止（-100分），所有音乐资源都无法通过预审核。

#### J. 可疑标记 (SUSPICIOUS) — 已确认触发条件

通过对比测试确认：`suspiciousCount` 在**标题非空**时始终为 2，在标题为空时为 0。后端对标题内容进行了额外可疑性检测（如标题格式是否符合 0DAY 规范），但检测结果不计入 details 明细，仅影响 suspiciousCount。

**确认**: SUSPICIOUS 计数不影响总分（仅用于提示），且不阻碍审核通过。

#### K. BDInfo 格式不被支持

测试确认：BDInfo 格式（以 `Disc Title:` 开头）**不被 LuckAudit 识别为有效的 MediaInfo 格式**，会触发:
- `MEDIAINFO_FORMAT_INVALID`: 缺少必需的 General 区块 (-8)
- `MEDIAINFO_INCOMPLETE`: Video 区块缺少 Width/Height (-4×2)

BDInfo 使用 `Resolution: 1920x1080` 而非 MediaInfo 的 `Width: 1920 pixels` / `Height: 1080 pixels` 格式，导致解析失败。

**解决方案**: 发布原盘资源时，必须将 BDInfo **转换为 MediaInfo 格式**或同时提供两种格式（以 MediaInfo 格式为主）。

#### L. 分类审核结果汇总

| 分类 | ID | 审核结果 | 特殊规则 |
|------|-----|---------|---------|
| 电影 | 401 | ✅ 正常审核 | 无 |
| 电视剧 | 402 | ✅ 正常审核 | 无 |
| 动画 | 405 | ✅ 正常审核 | 无 |
| MV | 406 | ✅ 正常审核 | 无 |
| 音乐 | 408 | ❌ **直接禁止** (-100) | `MUSIC_NOT_ALLOWED` |
| 其他 | 409 | ✅ 正常审核 | 无 |
| 综艺 | 410 | ✅ 正常审核 | 无 |
| 纪录片 | 411 | ✅ 正常审核 | 无 |
| 体育 | 412 | ✅ 正常审核 | 无 |
| 短剧 | 413 | ✅ 正常审核 | 无 |

#### M. HDR 标签审核规则（已修正）

低频测试发现了更精确的 HDR 标签规则：

| 场景 | 建议标签 | 审核结果 |
|------|---------|---------|
| MI 无 HDR format | 不勾选任何 HDR 标签 | ✅ 通过 |
| MI 有 `Dolby Vision ... HDR10 compatible` | DV(20) + **HDR10(19)** | ✅ 通过（缺 HDR10 则 WARNING -4） |
| MI 有纯 `SMPTE ST 2086, HDR10 compatible`（无 DV） | **不勾选** HDR10 标签 | ✅ 通过（90分） |
| 勾选 HDR10(19) + MI 仅有 HDR10（无 DV） | ❌ WARNING -5 | `TAGS_HDR_WRONGLY_SELECTED` |
| 勾选 Vivid HDR(21) + MI 有 `HLG` | ❌ WARNING -5 | `TAGS_HDR_WRONGLY_SELECTED` |
| 勾选 DV(20) + MI 无 Dolby Vision | ❌ WARNING -5 | `TAGS_HDR_WRONGLY_SELECTED` |

**关键发现**: HDR10(19) 标签的审核逻辑是**仅在 DV 内容包含 HDR10 compatible 时才建议勾选**。纯 HDR10 内容（无 DV）**不应勾选 HDR10 标签**，否则触发 WARNING。

#### N. 媒介-标题匹配规则（已修正）

| 标题格式 | 审核判定的媒介 | 正确选择 |
|---------|--------------|---------|
| `...BluRay REMUX...` | Remux | Remux(3) |
| `...UHD BluRay...` | UHD Blu-ray | UHD Blu-ray(10) |
| `...BluRay x264-GRP`（无REMUX） | **Encode** | Encode(7) |
| `...WEB-DL...` | WEB-DL | WEB-DL(11) |
| `...HDTV...` | HDTV | HDTV(5) |
| 选择 Blu-ray(1) + 标题含 BluRay 但无 REMUX | ❌ ERROR -7 | `TYPE_MISMATCH: 标题显示为Encode，但选择了Blu-ray` |
| 选择 Encode(7) + 标题含 BluRay 但无 REMUX | ✅ 通过 | 正确 |

**关键规则**: API 将 `BluRay x264-GRP` 格式（非原盘，无 REMUX 标识）判定为 **Encode**，不判定为 Blu-ray。Blu-ray(1) 仅适用于**完整原盘**发布（BDMV/ISO 结构）。

### 审核规则与字段映射完整列表

#### 按字段分组的审核扣分汇总

| 字段 | 最高可能扣分 | 关键审核点 |
|------|-------------|-----------|
| 分类 (type) | 10+100 | 必选 + 音乐禁止 |
| 媒介 (medium) | 8+7 | 必选 + 与标题匹配 |
| 编码 (codec) | 8 | 必选 |
| 分辨率 (standard) | 8 | 必选 |
| 音频 (audiocodec) | 8 | 必选 |
| 副标题 (small_descr) | 5 | 不能为空 |
| 标签 (tags) | 6+10+5+4 | 必选 + 中字智能 + HDR标签 |
| MediaInfo | 9+8+4×3+5 | 非空 + 格式 + 完整性 + 与标题匹配 |
| 简介 (descr) | 6+5+5 | 外部链接 + 内容质量 + 图片数量 |
| 标题 (name) | 5+5+7 | 与MI匹配 + 与媒介匹配 |

---

## 转载发布自动填写优化方案

### 概述

针对 LuckAudit 预审核系统，转载发布时需对每个字段进行优化填写，确保一次性通过 100 分审核。以下按字段逐一说明自动填写策略。

### 1. 标题 (name) 自动构造

**规则**: 0DAY 命名规范 + 与 MI 交叉验证

**自动构造逻辑**:

```
输入: 源站标题 + MediaInfo解析结果
输出: 符合0DAY规范的标题

算法:
1. 从源站标题提取基础信息（中文名、英文名、年份、季集号）
2. 从 MediaInfo 提取技术信息:
   - 分辨率 → 1080p / 2160p / 720p 等
   - 编码格式 → x264 / x265 / HEVC / AVC
   - 音频格式 → DTS / DTS-HD MA / TrueHD / Atmos / AAC / FLAC
   - HDR格式 → Dolby Vision / HDR10 / HDR10+
   - 音轨数 → 多音轨时标注 2Audios / 3Audios
3. 组装标题:
   电影: {英文名} {年份} {分辨率} {来源} {音频} {视频编码}-{制作组}
         例: The.Dark.Knight.2008.1080p.BluRay.DTS.x264-REFLECTIONS
   电视剧: {英文名}.S{季}E{集} {年份} {分辨率} {来源} {音频} {视频编码}-{制作组}
   UHD+DV: {英文名} {年份} 2160p UHD.BluRay.HEVC.{TrueHD.7.1.Atmos/DTS-HD.MA.7.1}-{制作组}
         注意: DV内容应在标题中体现（审核会检查）
4. 注意事项:
   - REMUX 必须大写出现在标题中（否则会被判为 Encode）
   - UHD Blu-ray 来源必须包含 UHD 标识
   - DV 内容标题中应包含 Dolby.Vision 或 DV 标识
```

**LuckAudit 交叉验证点**:
- 标题分辨率标识 ↔ MI Width/Height（WARNING -5分）
- 标题来源标识（BluRay/HDTV/WEB-DL/REMUX）↔ medium_sel 选择（ERROR -7分）
- 标题 HDR/DV 标识 ↔ MI HDR format（WARNING -5分）

### 2. 副标题 (small_descr) 自动填写

**规则**: 不能为空（ERROR -5分）

**自动填写逻辑**:

```
1. 优先使用源站副标题（如有中文内容）
2. 若源站副标题为空或纯英文:
   a. 从 PT-Gen/简介中提取中文标题
   b. 格式: {中文名} / {英文名备选}
   c. 可追加: {导演} / {主演} / {类型}
3. 确保非空字符串
```

### 3. IMDb 链接 (url) 自动填写

**规则**: 非必填但简介中需要外部链接

**自动填写逻辑**:

```
1. 从源站种子提取 IMDb ID（从 url 字段或简介中正则匹配）
2. 构造标准 URL: https://www.imdb.com/title/tt{ID}/
3. 若无 IMDb ID，尝试从 PT-Gen 结果中获取
```

### 4. PT-Gen (pt_gen) 自动填写

**规则**: 非必填，但简介需要外部链接

**自动填写逻辑**:

```
1. 从源站提取 PT-Gen 链接（如有）
2. 若源站无 PT-Gen，尝试通过 IMDb ID 调用 PT-Gen API 获取
3. 支持的来源: IMDB / Douban / Bangumi / Indienova
```

### 5. 简介 (descr) 自动构造

**规则**: 必须含 ≥2 张图片 + 外部链接 + 基础信息

**自动构造逻辑**:

```
1. 从源站复制简介 BBCode
2. 图片处理（关键！）:
   a. 统计现有 [img] 标签数量
   b. 如果 < 2 张:
      - 从源站种子详情页提取海报/截图图片
      - 从 PT-Gen 返回的数据中提取海报图片
      - 补充到简介顶部作为海报
   c. 确保所有 [img] 使用完整 URL（非短路径）
3. 外部链接处理:
   a. 检查简介中是否包含 imdb.com / douban.com / tmdb.org 链接
   b. 如果缺少，用 IMDb URL 字段值构造 BBCode 链接:
      [url=https://www.imdb.com/title/ttXXXX/]IMDb[/url]
   c. 如果有豆瓣链接，一并提供
4. 基础信息补充:
   a. 检查简介长度（建议 > 100 字符）
   b. 应包含: 导演/主演/简介/海报 等基本信息
   c. 可从 PT-Gen 自动获取这些信息
```

**简介模板（电影）**:

```bbcode
[img]{海报URL}[/img]

[b]导演:[/b] {导演}
[b]主演:[/b] {主演}
[b]类型:[/b] {类型}
[b]国家:[/b] {国家}
[b]年份:[/b] {年份}

[b]简介:[/b]
{剧情简介}

[b]MediaInfo:[/b]
{MediaInfo内容}

[url={IMDb链接}]IMDb[/url] | [url={豆瓣链接}]豆瓣[/url]

[b]Screenshots:[/b]
[img]{截图1}[/img]
[img]{截图2}[/img]
[img]{截图3}[/img]
```

### 6. MediaInfo (technical_info) 自动处理

**规则**: 非空 + 必须含 General/Video/Audio 区块 + 关键字段完整

**自动处理逻辑**:

```
1. 从源站提取 MediaInfo（从 technical_info 字段或简介中）
2. 格式验证:
   a. 检查是否包含 "General"、"Video"、"Audio" 区块标题
   b. General 区块: 确保包含 Duration 字段
   c. Video 区块: 确保包含 Width、Height、Format 字段
   d. Audio 区块: 确保包含 Format、Channel(s) 字段
3. 如果从源站 .torrent 内文件获取:
   a. 下载种子中的视频文件头部（通过 HTTP range 请求）
   b. 使用 mediainfo CLI 工具解析
   c. 输出完整 MediaInfo 文本
4. 关键: MI 是标签智能检测的数据源
   - Text 区块的 Language 字段 → 中字标签验证
   - Video 区块的 HDR format 字段 → HDR 标签验证
   - Video 区块的 Width/Height → 分辨率标签验证
   - Audio 区块的 Format → 音频编码验证
5. BDInfo 格式转换（原盘资源）:
   - BDInfo 格式不被 LuckAudit 识别（触发 FORMAT_INVALID）
   - 必须转换为 MediaInfo 格式或同时提供两种格式
   - BDInfo: `Resolution: 1920x1080` → MediaInfo: `Width: 1920 pixels` + `Height: 1080 pixels`
   - BDInfo: `Codec: AVC` → MediaInfo: `Format: AVC`
   - BDInfo: `Length: 2:15:30` → MediaInfo: `Duration: 2 h 15 min`
   - BDInfo: 无 General 区块 → MediaInfo: 添加 `General\nFormat: BDAV\nDuration: ...`
```

**完整 MediaInfo 模板（确保通过审核）**:

```
General
Complete name: {filename}.mkv
Format: Matroska
Duration: {duration}
File size: {size}

Video
Format: {codec}
Format/Info: {codec_info}
Width: {width} pixels
Height: {height} pixels
Frame rate: {fps} FPS
{HDR_format_if_exists}HDR format: {hdr_format}{/HDR_format_if_exists}

Audio
Format: {audio_codec}
Channel(s): {channels} channels
Sampling rate: {sample_rate} kHz
{language_if_exists}Language: {audio_language}{/language_if_exists}

Text #1
Format: {subtitle_format}
Language: {subtitle_language}

Text #2
Format: {subtitle_format}
Language: {subtitle_language}
```

### 7. 分类 (type) 自动选择

**自动选择逻辑**:

```
1. 从源站种子分类映射:
   - 源站"电影" → 401
   - 源站"电视剧" → 402
   - 源站"动画/动漫" → 405
   - 源站"综艺" → 410
   - 源站"纪录片" → 411
   - 源站"体育" → 412
   - 源站"短剧" → 413
   - 源站"MV" → 406
   - 源站"音乐" → 408 (⚠️ 当前被禁!)
2. 辅助判断:
   - 标题含 S**E** → 电视剧(402) 或 动画(405)
   - 标题含 EP** → 短剧(413) 或 电视剧(402)
   - PT-Gen 返回 type=movie/tv/anime → 对应分类
3. 特殊规则:
   - 音乐(408) 当前被禁止发布（-100分），需跳过
```

### 8. 质量字段自动选择

#### 8.1 媒介 (medium_sel) 自动选择

**自动选择逻辑（关键！标题交叉验证）**:

```
解析标题和 MediaInfo:
1. 标题含 "REMUX" 或 "Remux" → Remux(3)
2. 标题含 "UHD.BluRay" 或 "UHD-BluRay" → UHD Blu-ray(10)
3. 标题含 "BluRay" (不含REMUX) → 
   a. MI File size > 20GiB + 单文件 → 可能是原盘 Encode(7)
      但 API 会判为 Encode(7)
   b. 实际规则: 标题仅有 BluRay+x264 无 REMUX → Encode(7)
4. 标题含 "WEB-DL" 或 "WEBDL" → WEB-DL(11)
5. 标题含 "HDTV" → HDTV(5)
6. 标题含 "DVD" → DVD(6)
7. 标题含 "Encode" → Encode(7)
8. 音频资源 → CD(8) 或 Track(9)

安全策略:
- Blu-ray(1) 仅用于原盘发布（整盘ISO/BDMV结构）
- 标题含 BluRay.x264-GRP 格式时选 Encode(7) 而非 Blu-ray(1)
- Remux(3) 必须标题含 REMUX
- 注意: API 会从标题反推媒介类型，BluRay+x264-GRP = Encode
```

#### 8.2 编码 (codec_sel) 自动选择

```
从 MediaInfo Video.Format 解析:
- AVC / H.264 / x264 → H.264/AVC(1)
- HEVC / H.265 / x265 → H.265/HEVC(6)
- AV1 → AV1(2)
- VC-1 → VC-1(3)
- MPEG-2 → MPEG-2(4)
- MPEG-4 / XviD → MPEG-4/XviD(12)
- 其他 → Other(5)
```

#### 8.3 分辨率 (standard_sel) 自动选择

```
从 MediaInfo Video.Width/Height 解析:
- Width ≥ 3840 或 Height ≥ 2160 → 4K/2160p(6)
- Width ≥ 2560 或 Height ≥ 1440 → 2K/1440p(5)
- Width ≥ 1920 或 Height ≥ 1080 → 1080p/1080i(1)
- Width ≥ 1280 或 Height ≥ 720 → 720p/720i(3)
- 其他 → 480p/480i(4) 或 Other(8)
```

#### 8.4 音频编码 (audiocodec_sel) 自动选择

```
从 MediaInfo Audio.Format 解析:
- TrueHD + Atmos (Channel≥8) → TrueHD Atmos(11)
- TrueHD → TrueHD(14)
- DTS XLL + Channel≥8 → DTS-HD MA(16) 
- DTS:X → DTS:X(15)
- DTS + Channel≥6 → DTS(3)
- E-AC-3 / DDP → DDP/E-AC3(12)
- AC-3 / DD → DD/AC3(8)
- FLAC → FLAC(1)
- APE → APE(2)
- AAC → AAC(6)
- MP3 → MP3(4)
- OGG → OGG(5)
- LPCM → LPCM(13)
- WAV → WAV(18)
- PCM → PCM(19)
- M4A → M4A(17)
```

### 9. 标签 (tags) 自动选择

**自动选择逻辑（智能检测核心！）**:

```
1. 从源站标签映射:
   - 源站"禁转" → 禁转(1)
   - 源站"首发" → 首发(2)  [转载时不勾选]
   - 源站"原创" → 原创(23) [转载时不勾选]
   - 源站"DIY" → DIY(4)
   - 源站"国语" → 国语(5)
   - 源站"粤语" → 粤语(14)
   - 源站"英语" → 英语(22)
   - 源站"连载" → 连载(9)
   - 源站"完结" → 完结(10)
   - 源站"合集" → 合集(17)
   - 源站"大包" → 大包(11)

2. HDR 标签智能选择（从 MediaInfo）:
   IF MI.Video."HDR format" contains "Dolby Vision":
     勾选 Dolby Vision(20)
     IF MI.Video."HDR format" contains "HDR10 compatible":
       同时勾选 HDR10(19)   ← API 要求! (-4分)
   ELIF MI.Video."HDR format" contains "HDR10+" (无 DV):
     勾选 HDR10+(18)        ← 需验证
   ELSE:
     不勾选任何 HDR 标签    ← 纯 HDR10 不应勾选 HDR10 标签!
   
   注意: 
   - 纯 HDR10（无 DV）不应勾选 HDR10(19)，否则 WARNING -5
   - Vivid HDR/菁彩HDR(21) 需特定 MI 格式支持（非标准 HLG）
   - DV Profile 8 (HDR10 compatible) 必须同时勾选 DV + HDR10

3. 中字标签智能选择（从 MediaInfo）:
   IF MI 中存在 Text 区块 且 Language 包含 "Chinese":
     勾选 中字(6)
   ELSE:
     不勾选（否则 ERROR -10分）

4. 国语标签智能选择（从 MediaInfo）:
   IF MI 中存在 Audio 区块 且 Language 包含 "Chinese":
     勾选 国语(5)
   ELSE:
     不勾选

5. 转载默认标签:
   - 至少勾选 1 个标签（否则 ERROR -6分）
   - 如果无特殊标签可勾选，默认勾选 禁转(1)
```

### 10. 完整自动转载流程

```
输入: 源站种子信息 + .torrent 文件

Step 1: 解析源站信息
  ├─ 提取标题、副标题、简介、MediaInfo
  ├─ 提取分类、质量字段、标签
  └─ 提取 IMDb ID、PT-Gen 链接

Step 2: 构造标题 (name)
  ├─ 基于源站标题 + MediaInfo 技术信息
  ├─ 遵循 0DAY 命名规范
  ├─ 确保分辨率/来源/HDR标识与 MI 一致
  └─ REMUX 大写、UHD Blu-ray 标识

Step 3: 填写副标题 (small_descr)
  └─ 中文名 + 备选英文名，确保非空

Step 4: 填写 IMDb/PT-Gen (url, pt_gen)
  └─ 直接复制源站值

Step 5: 构造简介 (descr)
  ├─ 复制源站简介
  ├─ 确保图片 ≥ 2 张（不足则从 PT-Gen 获取海报补充）
  ├─ 确保含 IMDb/豆瓣 外部链接
  └─ 确保内容长度 > 100 字符

Step 6: 处理 MediaInfo (technical_info)
  ├─ 优先使用源站 MI
  ├─ 格式验证：General(Duration) + Video(Width/Height) + Audio(Format/Channels)
  └─ 缺失字段从 .torrent 内文件提取

Step 7: 选择分类 (type)
  ├─ 从源站分类映射
  └─ 跳过音乐(408)分类

Step 8: 选择质量字段
  ├─ medium: 从标题+MI判断（REMUX/UHD BluRay/BluRay→Encode/WEB-DL/HDTV）
  ├─ codec: 从 MI Video.Format 映射
  ├─ standard: 从 MI Video.Width/Height 映射
  ├─ audiocodec: 从 MI Audio.Format 映射
  └─ team: 从标题制作组映射，无匹配选 Other(5)

Step 9: 智能选择标签
  ├─ 基础标签: 从源站映射（禁转/连载/完结/合集等）
  ├─ HDR标签: 从 MI HDR format 智能选择（DV/HDR10/HDR10+）
  ├─ 中字标签: 从 MI Text.Language 判断
  ├─ 国语标签: 从 MI Audio.Language 判断
  └─ 确保至少 1 个标签

Step 10: 提交预审核（可选）
  ├─ POST 到 admin.luckpt.de/api/torrents/pre-audit
  ├─ 检查 passed=true
  └─ 如未通过，根据 details 修复后重试
```

### 11. 通过审核的最低要求清单

以下条件**必须全部满足**才能通过预审核（100分）：

- [x] 标题: 非空（空标题不触发 SUSPICIOUS 但不建议），与 MI 技术参数一致（分辨率/来源/HDR标识）
- [x] 副标题: 非空
- [x] 分类: 已选择（音乐 408 禁止，其余分类无特殊限制）
- [x] 媒介: 已选择且与标题来源标识一致（BluRay+x264-GRP = Encode(7)，非 Blu-ray(1)）
- [x] 编码: 已选择
- [x] 分辨率: 已选择
- [x] 音频: 已选择
- [x] 标签: 至少 1 个；中字标签需 MI 中文 Text 支持；DV 标签需 MI DV 支持
- [x] MediaInfo: 非空，**必须是 MediaInfo 格式**（BDInfo 不被识别），含 General(Duration) + Video(Width/Height/Format) + Audio(Format/Channels)
- [x] 简介: ≥2 张完整 URL 图片 + 含 IMDb/豆瓣链接 + 内容 > 100 字符
- [x] HDR: MI 有 DV 且含 HDR10 compatible 时需同时勾选 DV(20)+HDR10(19)；纯 HDR10（无 DV）**不勾选** HDR10 标签
- [x] SUSPICIOUS=2 是正常现象（标题分析触发），不影响审核通过

---

*数据来源: upload.php HTML (48298字节) + rules.php HTML (34082字节) + luckaudit.js (67658字节) + pre-audit API 30+次测试*
*文档创建: 2026-04-19*
*最后更新: 2026-04-23 (rules.php/rules 采集更新)*
