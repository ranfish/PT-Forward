# Nova高清 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | Nova高清|
| 站点地址 | https://pt.novahd.top |
| 站点框架 | NexusPHP |
| 特殊规则 | 分辨率含帧率细分（60FPS/120FPS）、番剧分类、17个制作组 |
| Cloudflare | 否 |
| 候选制 | 是（游戏类须候选，其他资源部分用户需候选） |
| MediaInfo | 是（独立 `technical_info` 字段） |
| IMDb | 是（url 字段） |
| 豆瓣 | 是（pt_gen 字段） |
| NFO | 是（独立 nfo 文件上传字段） |
| 匿名发布 | 是（uplver） |
| 官组后缀 | NHDWeb / NDJWEB |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 标题 |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

字段名带 `[4]` 后缀。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | Movies/电影 |
| 402 | TV Series/电视剧 |
| 403 | TV Shows/综艺 |
| 404 | Documentaries/记录片 |
| 405 | Animations/动画 |
| 406 | MV/演唱会 |
| 407 | Sports/体育 |
| 409 | Music/音乐 |
| 410 | Othes/其他 |
| 411 | Short Play/短剧 |
| 412 | Anime/动漫 |
| 413 | Anime Series/番剧 |
| 414 | Game/游戏 |

#### 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | UHD Blu-ray |
| 11 | WEB-DL |
| 12 | DVD |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H264/x264/AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H265/HEVC/x265 |

#### 音频编码（`audiocodec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | ALAC |
| 8 | TrueHD Atmos |
| 9 | DDP/E-AC3 |
| 10 | DD/AC3 |
| 11 | LPCM |
| 12 | TrueHD |
| 13 | DTS-HD MA |
| 14 | DTS:X |
| 15 | Other |

#### 分辨率（`standard_sel[4]`）— 含帧率细分

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p/4K |
| 6 | 4320p/8K |
| 7 | 2160p/4K 60Fps |
| 8 | 2160p/4K 120Fps |

#### 制作组（`team_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | HDSky |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | FRDS |
| 7 | beAst |
| 8 | CMCT |
| 9 | TLF |
| 10 | M-Team |
| 11 | BeiTai |
| 12 | AGSV |
| 13 | HDHome |
| 14 | TTG |
| 15 | NHDWeb |
| 16 | NDJWEB |

#### 标签（`tags[4][]`）— 18个多选 checkbox

| 值 | 显示名称 |
|----|----------|
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 驻站 |
| 9 | 分集 |
| 10 | 完结 |
| 11 | 英字 |
| 12 | 应求 |
| 13 | 大包 |
| 14 | 杜比 |
| 15 | 特效 |
| 17 | 番组 |
| 18 | 连载 |
| 19 | 高码 |
| 20 | 10Bit |
| 21 | 60FPS |

### 1.3 缺失字段

- `processing_sel` — 无地区选择

---

## 二、与其他站点对比

### 2.1 NovaHD 特色

| 维度 | NovaHD | HDVideo | HDFans |
|------|--------|---------|--------|
| 分类 | 13（含番剧/短剧/游戏） | 8 | 16 |
| 分辨率 | 含帧率（60FPS/120FPS） | 标准列表 | 标准列表 |
| 编码 | 合并原盘+压制 | 合并 | 区分 H.264/x264 |
| 音频 | 15种 | 21种 | 24种 |
| 制作组 | 17（含 AGSV/M-Team/BeiTai） | 3 | 30 |
| 标签 | 18（含 60FPS/10Bit/高码/番组） | 25 | 27 |
| MediaInfo | 有 | 无 | 有 |

### 2.2 关键差异

1. **分辨率含帧率** — 4K 60FPS(7) 和 4K 120FPS(8) 是独立选项，需要从 MediaInfo 的 FrameRate 字段判断
2. **番剧分类** — 区分 Anime(412) 和 Anime Series(413)，还有 Short Play(411)
3. **10Bit 标签** — 有独立的 10Bit 标签(20)，需从 MediaInfo BitDepth 判断
4. **60FPS 标签** — 有独立的 60FPS 标签(21)，与分辨率选项 60FPS 对应
5. **自制组** — NHDWeb(15) 和 NDJWEB(16) 是 NovaHD 自家制作组

---

## 三、站点适配器配置参考

```yaml
site:
  id: "novahd"
  name: "NovaHD"
  url: "https://pt.novahd.top"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动画": 405
      "MV": 406
      "体育": 407
      "音乐": 409
      "其他": 410
      "短剧": 411
      "动漫": 412
      "番剧": 413
      "游戏": 414

    medium_sel:
      "Blu-ray": 1
      "HD DVD": 2
      "Remux": 3
      "MiniBD": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Track": 9
      "UHD": 10
      "WEB-DL": 11
      "DVD": 12

    codec_sel:
      "H264": 1
      "VC-1": 2
      "Xvid": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 6

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS": 3
      "MP3": 4
      "OGG": 5
      "AAC": 6
      "ALAC": 7
      "TrueHD Atmos": 8
      "DDP": 9
      "AC3": 10
      "LPCM": 11
      "TrueHD": 12
      "DTS-HDMA": 13
      "DTS-X": 14
      "Other": 15

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "4K": 5
      "8K": 6
      "4K 60Fps": 7
      "4K 120Fps": 8

    team_sel:
      "HDSky": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "FRDS": 6
      "beAst": 7
      "CMCT": 8
      "TLF": 9
      "MTeam": 10
      "BeiTai": 11
      "AGSV": 12
      "HDHome": 13
      "TTG": 14
      "NHDWeb": 15
      "NDJWEB": 16

    tags:
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "驻站": 8
      "分集": 9
      "完结": 10
      "英字": 11
      "应求": 12
      "大包": 13
      "杜比": 14
      "特效": 15
      "番组": 17
      "连载": 18
      "高码": 19
      "10Bit": 20
      "60FPS": 21

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    audiocodec: "audiocodec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  missing_fields:
    - "processing_sel"
```

---

## 四、发布流水线注意事项

### 4.1 分辨率帧率判断

NovaHD 的分辨率选项含帧率细分，需要从 MediaInfo 提取：

```
FrameRate >= 120 → 8 (2160p/4K 120Fps)
FrameRate >= 60  → 7 (2160p/4K 60Fps)
其他 4K          → 5 (2160p/4K)
```

同时需勾选对应的 60FPS 标签(21)。

### 4.2 10Bit 标签

从 MediaInfo BitDepth 判断：
- BitDepth = 10 → 勾选 10Bit 标签(20)

### 4.3 番剧分类

NovaHD 区分三个动漫相关分类：
- Animations(405) — 动画电影
- Anime(412) — 动漫（单季/单集）
- Anime Series(413) — 番剧（多季合集）

### 4.4 制作组映射

NovaHD 有 17 个制作组，含自家组 NHDWeb(15) 和 NDJWEB(16)。转种时非列表内制作组选 Other(5)。

---

*分析时间：2026-04-16*
*数据来源：https://pt.novahd.top/upload.php 发布页面 HTML 分析*
*文档更新：2026-04-22 — 补充 rules.php 完整规则 + 发布页 Playwright 验证 + 站点信息完善*

---

## 五、发布页字段验证（2026-04-22 Playwright 实际采集）

> 用户 ranfish 已登录，页面标题 `NovaHD :: 发布 - Powered by NexusPHP`。

与现有文档逐一对比，**所有字段完全一致**，无差异：

- 分类 13 个 ✅
- 媒介 12 个 ✅
- 视频编码 6 个 ✅
- 音频编码 15 个 ✅
- 分辨率 8 个（含 60FPS/120FPS）✅
- 制作组 16 个 ✅
- 标签 18 个 + uplver ✅
- 独立 `technical_info` 字段 ✅

---

## 六、站点规则（rules.php 完整采集 2026-04-22）

> 来源：https://pt.novahd.top/rules.php（Playwright 认证抓取）

### 6.1 总则

- 禁止发送垃圾信息
- 禁止注册多账号
- 禁止将种子上传到其他 Tracker
- 一切作弊账号封禁
- 首次捣乱警告，第二次永久封禁

### 6.2 账号保留规则

| 等级 | 保留条件 |
|------|---------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存后不删除 |
| 封存账号 | 连续 400 天不登录删除 |
| 未封存账号 | 连续 150 天不登录删除 |
| 无流量用户 | 连续 100 天不登录删除 |

### 6.3 下载规则

- 分享率过低会禁止下载/封禁
- **种子促销**（随机）：

| 概率 | 促销类型 |
|------|---------|
| 10% | 50% 下载 |
| 5% | 免费 |
| 5% | 2x 上传 |
| 3% | 50% 下载 & 2x 上传 |
| 1% | 免费 & 2x 上传 |

- 体积 > 20GB 自动免费
- Blu-ray / HD DVD 原盘自动免费
- 电视剧每季第一集免费
- 促销时限：除 2x 上传外限时 7 天；2x 上传无时限
- 发布 1 个月后自动永久 2x 上传

### 6.4 上传规则

#### 上传资格

- 任何人都能发布
- 部分用户需先候选
- **游戏类**：仅上传员及以上等级可自由上传，其他须候选

#### 允许的资源

| 类型 | 说明 |
|------|------|
| 高清视频 | Blu-ray/HD DVD 原盘/Remux, HDTV, 高清重编码（≥720p）, 高清 DV |
| 标清视频 | 高清媒介标清重编码（≥480p）, DVDR/DVDISO, DVDRip/CNDVDRip |
| 无损音轨 | FLAC, APE 等（含 cue 表单） |
| 多声道音轨 | ≥5.1 标准（DTS/DTSCD 等），评论音轨 |
| PC 游戏 | 必须原版光盘镜像 |
| 高清预告片 | 7 日内发布 |
| 高清软件/文档 | 与高清相关 |

#### 禁止的资源

| 类型 | 说明 |
|------|------|
| 体积 < 100MB | 除高清软件/文档、单曲专辑外 |
| Upscale 视频 | 标清 upscale 或部分 upscale |
| 劣质视频 | CAM/TC/TS/SCR/DVDSCR/R5/R5.Line/HalfCD |
| RMVB/RM/FLV | RealVideo 编码和 Flash 格式 |
| 单独样片 | 须与正片一起上传 |
| 有损音频 | <5.1 声道的有损 MP3/WMA |
| 无 CUE 多轨音频 | 无正确 cue 表单 |
| 游戏相关 | 硬盘版/高压版/非官方镜像/第三方 mod/破解补丁 |
| 压缩文件 | RAR 等 |
| 重复资源 | 详见 DUPE 规则 |
| 敏感内容 | 禁忌/色情/政治敏感 |
| 损坏文件 | 读取/回放错误 |
| 垃圾文件 | 病毒/木马/广告/种子内含种子 |

#### DUPE 判定规则

**来源优先级**（高→低）：

```
Blu-ray / HD DVD > HDTV > DVD > TV
```

- 高清版本使标清版本视为重复
- 动漫类 HDTV 和 DVD 同优先级（特例）
- 相同媒介相同分辨率的重编码：按发布组优先级判定
- 总会保留一个 DVD5 大小（~4.38GB）的最佳画质重编码版本
- 不同区域的 Blu-ray/HD DVD 原盘（不同配音/字幕）不视为重复
- 无损音轨每种只保留一个版本（分轨 FLAC 优先级最高）

**不受 DUPE 约束**：
- 旧版本连续断种 ≥ 45 天
- 旧版本已发布 ≥ 18 个月
- 新版本无旧版已确认错误，或来源质量更好

#### 资源打包规则

**允许打包**：
- 按套装售卖的高清电影合集
- 整季电视剧/综艺/动漫
- 同一专题纪录片
- 7 日内高清预告片
- 同一艺术家 MV（标清按 DVD 打包，高清分辨率相同）
- 同一艺术家 5 张以上专辑（两年内可单独发布）
- 分卷发售的动漫/角色歌/广播剧
- 发布组打包

**打包要求**：视频须同媒介/同分辨率/同编码；音频须同编码格式。

### 6.5 标题命名规范

**标题格式**：

#### 电影

```
[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```

示例：
```
蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS
```

#### 电视剧

```
[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```

示例：
```
越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU
```

#### 音轨

```
[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]
```

示例：
```
恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC
```

#### 游戏

```
[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]
```

示例：
```
红色警戒3:起义时刻 Command And Conquer Red Alert 3 Uprising-RELOADED
```

#### 副标题

- 不要包含广告或求种/续种请求

#### 外部信息

- 电影和电视剧必须包含 IMDb 链接（如果存在）

#### 简介要求

| 分类 | 必须 |
|------|------|
| 影视 | 海报/封面、简介、截图、编码信息 |
| 体育 | 禁止泄露比赛结果 |
| 音乐 | 专辑封面、曲目列表 |
| 游戏 | 海报/封面、截图 |

- NFO 写入 NFO 文件上传，不粘贴到简介
- 尽量使用原始发布信息

### 6.6 无盒子规则

rules.php 中**没有盒子/SeedBox 规则**章节。

> **对 PT-Forward 的影响**：NovaHD 对盒子无特殊限制，PT-Forward 转发行为不受盒子规则约束。站点促销策略宽松（90% 种子有促销），发布后自动获得双倍上传量。
