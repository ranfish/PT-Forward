# PTFans 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | PTFans|
| 站点地址 | https://ptfans.cc |
| 站点框架 | NexusPHP |
| 特殊规则 | 综合PT站（含种子区 mode=4 + 9KG特别区 mode=5），仅允许向种子区转发，禁止向特别区转发（§30.3 屏蔽）、全站HR、盒子限速+备案制、收费种子 |
| Cloudflare | 否 |
| 候选制 | 是（游戏类须候选） |
| MediaInfo | 是（独立 `technical_info` 字段） |
| IMDb | 是（url 字段） |
| 豆瓣 | 是（pt_gen 字段） |
| NFO | 是（独立 nfo 文件上传字段） |
| 匿名发布 | 否（未发现 uplver 字段） |
| H&R | 全站强制，35 天内做种 7 天，3 个 HR 封号 |
| 盒子规则 | 限速 100Mbps + 新种头 3 天最多 3 倍上传 + 需备案 |

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
| `qch_bt_rainbow_enable` | checkbox | - | 彩虹种（站点特色功能） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 双分类模式

PTFans 有两套分类，通过 `data-mode` 区分：

- **mode=4（综合区）**：影视、学习、书籍等常规资源 — **PT-Forward 只使用此模式**
- **mode=5（特别区/9KG）**：成人内容 — ⛔ **全局禁止（§30.3）**，YAML `skip_modes: ["5"]`

### 1.3 综合区类型（`type` mode=4）— 18个

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 404 | TV Series(电视剧) |
| 405 | TV Shows(综艺) |
| 406 | Documentaries(纪录片) |
| 407 | Music(音乐、专辑、MV、演唱会) |
| 408 | Art(曲艺、相声、小品、戏曲、舞蹈、歌剧、评书等) |
| 409 | Games(游戏及相关) |
| 410 | Science(科学、知识、技能) |
| 411 | School(应试、考级、职称、初中以上教育) |
| 412 | Book(书籍、杂志、报刊、有声书) |
| 413 | Code(IT技术、建模、编程、信息技术、大数据、人工智能) |
| 414 | Animate(3D动画、2.5次元) |
| 415 | ACGN(二次元、漫画) |
| 416 | Baby(婴幼、儿童、早教、小学及相关) |
| 417 | Resource(素材、数据、图片、文档、模板) |
| 418 | Software(软件、系统、程序、APP等) |
| 403 | Sport(体育、竞技、武术及相关) |

注意：分类偏重**教育和学习资源**（Science/School/Book/Code/ACGN/Baby 等），与常规影视站不同。

### 1.4 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | CD |
| 2 | DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | Web-DL |
| 6 | Blu-ray |
| 7 | Encode |
| 8 | Encode |
| 9 | Other |

注意：值7和值8都显示 "Encode"，疑似站点配置错误。

### 1.5 视频编码（`codec_sel[4]`）— 区分原盘/压制

| 值 | 显示名称 |
|----|----------|
| 1 | H.264(x264/AVC) |
| 2 | H.265(x265/HEVC) |
| 3 | Bluray(VC-1) |
| 4 | Bluray(AVC) |
| 5 | Bluray(HEVC) |
| 6 | MPEG-2 |
| 7 | Xvid |
| 8 | AV1 |
| 9 | Other |

注意：编码区分原盘和压制 — Bluray(AVC)(4) vs H.264(x264/AVC)(1)，Bluray(HEVC)(5) vs H.265(x265/HEVC)(2)。

### 1.6 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4K |
| 6 | 8K |

### 1.7 制作组（`team_sel[4]`）— 仅6个

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |

### 1.8 标签（`tags[4][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | DoVi |

### 1.9 缺失字段

- `processing_sel` — 无地区选择
- `audiocodec_sel` — 无音频编码选择
- `team_sel` 仅5个国内老牌组，转种基本都选 Other(5)

---

## 二、与其他站点对比

| 维度 | PTFans | OKPT | HDFans |
|------|--------|------|--------|
| 分类 | 18（偏教育/学习） | 19（双模式） | 16 |
| 制作组 | 5（最少） | 29 | 30 |
| 标签 | 8（最少） | 18 | 27 |
| 编码 | 区分原盘/压制 | 合并 | 区分 H.264/x264 |
| 音频编码 | 无 | 含整轨/分轨 | 24种 |
| 地区 | 无 | 8种 | 13种 |
| 特色 | 教育资源+9KG区 | 双模式+制作组映射 | 媒介细分20种 |

### 关键差异

1. **分类偏重教育** — 18个分类中约一半与学习/教育相关（Science/School/Book/Code/ACGN/Baby 等），影视分类使用非标准ID（电影=401，电视剧=404而非402）
2. **编码区分原盘/压制** — Bluray(AVC)(4) vs H.264(x264)(1)，Bluray(HEVC)(5) vs H.265(x265)(2)
3. **制作组极少** — 仅 HDS/CHD/MySiLU/WiKi/Other，都是国内老牌组
4. **媒介值7和8重复** — 都显示 "Encode"，需注意避免选错
5. **9KG特别区** — mode=5 的12个分类全为成人内容

---

## 三、站点适配器配置参考

```yaml
site:
  id: "ptfans"
  name: "PTFans"
  url: "https://ptfans.cc"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  restricted_modes:
    - 5  # 9KG特别区

  mappings:
    type:
      "电影": 401
      "剧集": 404
      "综艺": 405
      "纪录": 406
      "音乐": 407
      "体育": 403
      "游戏": 409
      "其他": 418

    medium_sel:
      "CD": 1
      "DVD": 2
      "Remux": 3
      "MiniBD": 4
      "WEB-DL": 5
      "Blu-ray": 6
      "Encode": 7
      "Other": 9

    codec_sel:
      "H264": 1
      "H265": 2
      "VC-1": 3
      "Bluray AVC": 4
      "Bluray HEVC": 5
      "MPEG-2": 6
      "Xvid": 7
      "AV1": 8
      "Other": 9

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "4K": 5
      "8K": 6

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "DoVi": 8

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  missing_fields:
    - "audiocodec_sel"
    - "processing_sel"

  codec_mode_map:
    disc_media: ["Blu-ray", "Remux"]
    disc_codecs:
      "AVC": 4
      "HEVC": 5
      "VC-1": 3
    encode_codecs:
      "H264": 1
      "H265": 2
      "AV1": 8
      "Other": 9
```

---

## 四、发布流水线注意事项

### 4.1 编码按媒介类型选择

PTFans 的编码字段区分原盘和压制：
- 原盘/Remux → Bluray(AVC)(4)、Bluray(HEVC)(5)、Bluray(VC-1)(3)
- 压制/WEB → H.264(x264)(1)、H.265(x265)(2)、AV1(8)

适配器需根据媒介类型选择正确的编码值。

### 4.2 媒介值冲突

medium_sel 的值7和值8都显示 "Encode"，建议使用值7。

### 4.3 分类ID非标准

PTFans 的分类ID不遵循常见规律：
- 电影=401（标准）
- 电视剧=404（非常规，通常是402）
- 综艺=405（非常规，通常是403）

### 4.4 9KG特别区 — ⛔ 全局禁止（§30.3）

PTFans mode=5 包含 11 个成人内容分类：

| ID | 名称 |
|----|------|
| 420 | 步兵(无码) |
| 421 | 骑兵(有码) |
| 422 | 三级(三级片、限制级) |
| 423 | H漫(动漫、漫画) |
| 424 | H游(游戏) |
| 425 | H书(书籍、有声书) |
| 426 | H图(写真、图片) |
| 427 | H音(ASMR、音频) |
| 428 | H综(综艺、综合) |
| 429 | H同(男同、女同) |
| 419 | Other(其它) |

> 禁止下载和发布这些分类的资源。YAML `skip_categories` + `skip_modes: ["5"]`。
> 启动时由 RequiredSkips 审计（§30.4.2）。

---

*分析时间：2026-04-16*
*数据来源：https://ptfans.cc/upload.php 发布页面 HTML 分析*
*文档更新：2026-04-22 — Playwright 3 页采集：rules.php 完整规则 + details.php 站点特色规则 + 发布页验证*

---

## 三、站点规则（rules.php 完整采集 2026-04-22）

> 来源：https://ptfans.cc/rules.php

### 3.1 总则

- 禁止发送垃圾信息
- 禁止注册多账号
- 禁止将种子上传到其他 Tracker
- 一切作弊封禁
- 首次捣乱警告，第二次永久封禁

### 3.2 账号保留规则

| 等级 | 保留条件 |
|------|---------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存后不删除 |
| 封存账号 | 连续 180 天不登录删除 |
| 未封存账号 | 连续 90 天不登录删除 |
| 无流量用户 | 连续 7 天不登录删除 |

### 3.3 下载规则

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

### 3.4 上传规则

#### 允许的资源

高清视频（Blu-ray/HD DVD 原盘/Remux, HDTV, ≥720p 重编码）、标清视频、无损音轨、≥5.1 声道音轨、PC 游戏原版镜像、7 日内高清预告片、高清软件/文档。

#### 禁止的资源

体积 <100MB（除高清软件/文档、单曲专辑）、Upscale 视频、CAM/TC/TS/SCR/DVDSCR/R5/HalfCD、RMVB/RM/FLV、单独样片、<5.1 有损音频、无 CUE 多轨音频、游戏硬盘版/高压版/非官方镜像、RAR 压缩包、DUPE、色情/敏感内容、损坏文件、垃圾文件。

#### DUPE 判定

来源优先级：Blu-ray/HD DVD > HDTV > DVD > TV

- 高清版本使标清视为重复
- 动漫类 HDTV 和 DVD 同优先级
- 不同区域原盘不视为重复
- 无损音轨每种只保留一个版本（分轨 FLAC 优先级最高）
- 旧版本断种 ≥45 天或已发布 ≥18 个月 → 不受 DUPE 约束

#### 资源打包规则

允许：按套装售卖合集、整季电视剧/综艺/动漫、同专题纪录片、同艺术家 MV/音乐（≥5 张打包，两年内可单独）、发布组打包。

### 3.5 标题命名规范

| 类型 | 格式 |
|------|------|
| 电影 | `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组` |
| 电视剧 | `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组` |
| 音轨 | `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组]` |
| 游戏 | `[中文名] 名称 [年份] [版本] [发布说明][-发布组]` |

### 3.6 简介要求

- NFO 写入 NFO 文件上传，不粘贴到简介
- 影视：海报 + 截图 + 编码信息 + 演职员/剧情
- 体育：禁止泄露比赛结果
- 音乐：专辑封面 + 曲目列表
- 游戏：海报/封面 + 截图

---

## 四、站点特色规则（details.php?id=68 采集）

> 来源：https://ptfans.cc/details.php?id=68（新人教程种子详情页）

### 4.1 全站 H&R 规则

| 规则 | 说明 |
|------|------|
| 范围 | 全站 HR，每个下载量超过 100% 的种子 |
| 达标条件 | 35 天内做种 **7 天** |
| 处罚 | **3 个 HR 自动封号** |
| 消除方式 | 魔力清除 HR / 重新下载校验 |

> 误删除不达标的种子只能重新下载。孤种用置顶促销，死种用魔力清除 HR。

### 4.2 盒子规则

| 规则 | 说明 |
|------|------|
| 限速 | 上传限速 **100Mbps**（≈11.92 MiBps） |
| 超速处罚 | 几分钟内超速 3 次封号 |
| 备案 | **不备案超速不会解封** |
| 新种限制 | 头 3 天盒子最多计算 **3 倍上传** |
| 下载优惠 | 盒子下载**不享受任何优惠** |

> 企业宽带限速规则同盒子。

### 4.3 收费种子规则

| 规则 | 说明 |
|------|------|
| 资格 | ≥Power User 可设置收费种子 |
| 机制 | 下载 torrent 文件即扣魔力（非下载完成后扣） |
| 上限 | 最高 10 万魔力，扣税三成 |
| 发种人责任 | 不做种又设收费 → 按收费价格×下载人数双倍处罚+警告 |
| 违规 | 违规 3 次封号 |
| 风险 | 下载收费孤种/死种风险自担，扣费不返还 |

### 4.4 新人考核

- 考核内容：下载量 + 平均做种时间
- 7 天无流量自动封号
- 被封不解封，需重新注册

### 4.5 僵尸清理

| 条件 | 处理 |
|------|------|
| 新注册 7 天无流量 | 自动封号 |
| 无流量用户 7 天不登录 | 封号 |
| 未封存 90 天不登录 | 封号 |
| 封存 180 天不登录 | 封号 |
| 只签到辅种无流量 | 封号 |
| 分享率警告 | 30 天内周末发种改善，1 周后自动撤销 |

---

## 五、发布页验证（2026-04-22 Playwright 实际采集）

> 用户 ranfish 已登录，页面标题 `PTFans :: 发布 - Powered by NexusPHP`。

与现有文档对比：

| 对比项 | 原文档 | 采集结果 | 一致 |
|--------|--------|---------|------|
| 基础字段 | file/name/small_descr/url/pt_gen/nfo/descr/technical_info | 完全一致 | ✅ |
| 分类 type | 11 个成人分类 | 完全一致 | ✅ |
| 媒介 medium_sel | 10 个 | 完全一致 | ✅ |
| 编码 codec_sel | 9 个 | 完全一致 | ✅ |
| 分辨率 standard_sel | 6 个 | 完全一致 | ✅ |
| 制作组 team_sel | 5 个 (HDS/CHD/MySiLU/WiKi/Other) | 完全一致 | ✅ |
| 标签 tags | 7 个 + uplver | 无 uplver，有彩虹种 checkbox | ⚠️ |

**差异**：无 `uplver` 匿名发布字段；有 `qch_bt_rainbow_enable` 彩虹种 checkbox（扣 100 魔力，有效期 30 天）。

> **对 PT-Forward 的影响**：
> - PTFans 是综合 PT 站，有正常种子区（mode=4）和 9KG 特别区（mode=5）
> - **允许向种子区发布转载，禁止向特别区发布转载**
> - §30.3 已屏蔽特别区 mode=5 全部 11 个分类（419-429）
> - H&R 全站强制（35 天内做种 7 天），转发后须保持做种
> - 盒子限速 100Mbps，PT-Forward 部署在盒子时需备案且限速
> - 新种头 3 天盒子仅 3 倍上传，下载无优惠
> - 无匿名发布选项

### 附：发布适配记录（v0.0.1292 §59.436）

- **form_config**：migration119 分发（六槽 52 项——type 17/medium 8/
  codec 9/standard 6/team 5/tags 7）。实时 HTML（48.3KB 直连）vs 资产
  vs 文档全槽一致（零漂移站）。
- 双模式站（mode=4 综合区/5 9KG 成人区禁止——§59.372+§59.388 双防线
  自动护；发布链走综合区；9KG 区 12 分类全局禁止成人拦截在前天然
  不可达）。
- type 17 **教育/知识向大分类站**（Science/School/Book/Code/ACGN/
  Baby/Resource 罕见细分）+非标值集（404 电视剧/405 综艺/406 纪录片
  错位）：**407 Music=[music,lossless_music,mv,music_video,concert]
  五键合并承接**（音乐/专辑/MV/演唱会一大类——历史三键承接放大）
  +412 Book=[ebook,audiobook] 双键+414 Animate=[animation]+411
  School=[education]+418 Software；408 Art/410 Science/413 Code/
  415 ACGN/416 Baby/417 Resource label-only（dict 无键）。
- medium 9：**Encode 7/8 重复值站方配置错误（克隆式去重第五例）**——
  配置只留 **HTML 首值 8**（§59.394 蟹黄堡口径——首版误留资产序首值
  7，formdiff 抓出 changed cur=7 draft=8 即修）+Other(9) 首乱序+
  Blu-ray=[bluray,uhd_bluray]/Remux=[remux,uhd_remux] 双键归并。
- **★codec 原盘/压制编码双维站——blurayCodecOption 特化挂
  codecMappingOf**（站方 §4.1 明文：原盘/Remux→Bluray(AVC/HEVC/
  VC-1) 项、压制/WEB→H.26x 项）：**动态检测 "Bluray(" 前缀非站名
  硬编码**（原盘族媒介∧有原盘项才分发——§59.407 diyMediumOption
  同款通用投资未来同形态站自动生效）；Bluray 三项 label-only 特化
  独占（防 h264 键一键多值）+H.264(x264/AVC)/H.265(x265/HEVC)
  合并命名零联动+MPEG-2/Xvid/AV1/Other 配键；测试锚八态。
- standard 4K(5)/8K(6) 置尾全配键；**无 audiocodec 槽第五站**
  （GTK/克隆/三月/奥申后）。
- team 5：HDS/CHD/MySiLU/WiKi 老牌四组 label-only（文档 §1.9
  "转种基本都选 Other"）+Other=[team.other] 兜底。
- tags 7：首发/禁转绝不配（财神雷区）+DoVi=[dolby_vision]+DIY/
  国语/中字+HDR 四键泛家族（17 站）。
- 字段：url+**pt_gen 真字段**+technical_info+**无 uplver**（实时
  实证——控件全列无匿名字段 anonymous 无承载）；nfo 不配；
  qch_bt_rainbow_enable（站点插件）不配。
- **验证**：裸对全槽一致+formdiff 0 条（Encode 首值修正后归零）+
  DryRun 四形态（① UHD 原盘 AVC→401+6+**codec=4 Bluray(AVC)
  特化**+tags=[DoVi,中字,HDR] 三标 ✓/② MI 压制 AVC→medium=8+
  codec=1 H.264 通用**不触发特化** ✓/③ WEB-DL 剧集→404 非标+5 ✓/
  ④ 演唱会→407 五键+codec=4 Bluray 特化 ✓——**原盘/压制双向分发
  实证**）+48 包全绿。
- 站规人工把关：标题命名规范+鼓励保种+吸血处罚+转载注明来源+9KG
  区禁止（全局）+教育类内容健康要求。
- 遗留：首颗真实发布。
