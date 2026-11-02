# 星湾 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 星湾 |
| 域名 | xingwan.cc |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（offers.php） |
| MediaInfo | 是（`technical_info` 字段） |
| IMDb | 是（详情页需包含外部信息链接） |
| 豆瓣 | 未确认 |
| 匿名发布 | 是（`uplver` 字段） |
| NFO | 是（`nfo` 字段） |
| PT-Gen | 是（upload.php 引用 ptgen.js） |

## Tracker URL

`https://xingwan.cc/announce.php`

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件（id="torrent"） |
| `name` | text | - | 标题 |
| `small_descr` | text | - | 副标题 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode，rows=20） |
| `technical_info` | textarea | - | MediaInfo / BDInfo（rows=8） |
| `uplver` | checkbox | - | 匿名发布 |

### 1.2 类型选择器（`type`，id="browsecat"，data-mode='4'）

选择分类后通过 JS 显示 `tr[relation=mode_4]` 质量行。

| 值 | 显示名称 |
|----|----------|
| 401 | 🎬 电影 Movies |
| 402 | 📺 电视剧 TV Series |
| 403 | 🎤 综艺 TV Shows |
| 404 | 🌍 纪录片 Documentaries |
| 405 | 🌸 动漫 Animations |
| 406 | 🎵 音乐视频 Music Videos |
| 407 | ⚽ 体育 Sports |
| 408 | 🎧 无损音乐 HQ Audio |
| 409 | 📦 其他杂项 Misc |
| 410 | 📱 短剧 ShortDrama |
| 411 | 🎮 游戏 Games |
| 412 | 💻 软件 Software |
| 413 | 📚 电子书 Ebooks |
| 416 | 📖 漫画 Comics |

### 1.3 媒介选择器（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 10 | Blu-ray / BD（蓝光原盘） |
| 11 | Remux（无损封装） |
| 12 | Encode（压制版） |
| 13 | WEB-DL（官网源） |
| 14 | WEBRip（流媒体录制源） |
| 15 | HDTV（电视台来源） |
| 16 | DVD / DVDR |
| 17 | MiniBD（迷你蓝光压制） |
| 18 | HD DVD |
| 19 | CD（音乐 CD） |
| 20 | Track / Soundtrack（原声音乐/OST） |
| 21 | Other |

### 1.4 编码选择器（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 / AVC |
| 6 | H.265 / HEVC |
| 7 | AV1 |
| 8 | VC-1 |
| 9 | MPEG-2 |
| 10 | VP9 |
| 11 | Xvid |
| 12 | DivX |
| 13 | ProRes |
| 14 | DNxHD / DNxHR |
| 15 | Other |

### 1.5 分辨率选择器（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 5 | SD |
| 6 | 720p |
| 7 | 1080p |
| 8 | 1080i |
| 9 | 2K |
| 10 | 1440p / QHD |
| 11 | 2160p / 4K UHD |
| 12 | 4K Remux |
| 13 | 8K |
| 14 | 3D SBS |
| 15 | 3D HSBS |
| 16 | 3D MVC / Blu-ray 3D |
| 17 | IMAX |
| 18 | VR 180° |
| 19 | VR 360° |
| 20 | Other |

### 1.6 字段后缀规则

字段名带后缀 `[4]`，对应 `data-mode='4'` 的分类组。例如 `medium_sel[4]`、`codec_sel[4]`、`standard_sel[4]`。

---

## 二、种子列表页分析（torrents.php）

### 2.1 表头结构

| 列 | 排序字段 | 说明 |
|----|----------|------|
| 类型 | - | 分类图标（cat=401→c_movies 等） |
| 标题 | sort=1 | 标题 + 促销图标 + 标签 + 副标题 |
| 评论 | sort=3 | 评论数 |
| 存活时间 | sort=4 | 发布时间 |
| 大小 | sort=5 | 文件大小 |
| 种子数 | sort=7 | seeders |
| 下载数 | sort=8 | leechers |
| 完成数 | sort=6 | snatched |
| 发布者 | sort=9 | 上传者（可能匿名） |

### 2.2 下载链接格式

```
download.php?id={torrent_id}
```

带 passkey（如配置）:
```
download.php?id={torrent_id}&passkey={passkey}
```

### 2.3 详情页链接格式

```
details.php?id={torrent_id}&hit=1
```

### 2.4 促销/折扣指示器

| CSS class | 含义 | DOM 提示文字 |
|-----------|------|-------------|
| `img.pro_free` | 免费（0x下载） | `<font class="free">免费</font>` |
| `img.pro_50pctdown` | 50% 下载 | `<font class="halfdown">50%</font>` |
| `img.pro_free2up` | 2X 免费（推测） | - |
| `img.pro_2up` | 2X 上传（推测） | - |
| `img.pro_30pctdown` | 30% 下载（推测） | - |
| `img.pro_custom` | 自定义促销（推测） | - |

促销倒计时格式：`<font color='#0000FF'>剩余时间：X天X小时</font>`

### 2.5 促销筛选器（`spstate`）

| 值 | 显示名称 |
|----|----------|
| 0 | 全部 |
| 1 | 普通 |
| 2 | 免费 |
| 3 | 2X |
| 4 | 2X免费 |
| 5 | 50% |
| 6 | 2X 50% |
| 7 | 30% |

---

## 三、详情页分析（details.php）

### 3.1 Info Hash 位置

```html
<td class="no_border_wide"><b>Hash码:</b>&nbsp;{info_hash}</td>
```

### 3.2 下载链接

- 普通下载：`download.php?id={torrent_id}`
- 种子链接（带认证，当天有效）：`download.php?downhash={user_id}.{JWT_token}`

---

## 四、标签系统

| tag_id | 名称 | 颜色 | 说明 |
|--------|------|------|------|
| 1 | 禁转 | #ff0000 | **禁止转载，转发时需排除** |
| 2 | 首发 | #8F77B5 | - |
| 3 | 官方 | #0000ff | 官组资源 |
| 4 | DIY | #46d5ff | - |
| 5 | 国语 | #6a3906 | - |
| 6 | 中字 | #006400 | - |
| 7 | HDR | #38b03f | - |
| 10 | 电影 | #FF5722 | - |
| 11 | 电视剧 | #9C27B0 | - |
| 12 | 动漫 | - | - |
| 26 | 英字 | #F0F0F0/#000000 | - |
| 27 | HDR10+ | #32CD32 | - |
| 29 | 特效 | #ffcc00/#ffffff | - |
| 30 | 漫画 | #A52A2A | - |
| 31 | 粤语 | #FDE68A/#92400E | - |
| 32 | 4K | #ffcc00 | - |
| 33 | HDRVi | #6A5ACD | - |
| 34 | HDR10bit | #20B2AA | - |
| 35 | 英语 | #1E88E5 | - |

---

## 五、促销规则（站点原始规则）

### 随机促销（种子上传后系统自动设定）

| 概率 | 促销类型 | 对应值 |
|------|----------|--------|
| 10% | 50% 下载 | 50% |
| 5% | 免费 | Free |
| 5% | 2X 上传 | 2xUp |
| 3% | 50%下载 & 2X上传 | 2x50% |
| 1% | 免费 & 2X上传 | 2xFree |

### 自动免费条件

- 文件总体积 > 20GB → 自动免费
- Blu-ray Disk / HD DVD 原盘 → 自动免费
- 电视剧等每季第一集 → 自动免费

### 促销时限

- 除 2X上传外，其余限时 7 天
- 2X上传无时限
- 所有种子发布 1 个月后自动永久 2X上传

---

## 六、站点发布规则要点

### 上传者资格

- 任何人都能发布资源
- 游戏类资源需上传员及以上等级，或先在候选区提交候选

### 允许的资源

- 高清视频（720p+）、蓝光原盘、remux、HDTV
- 标清视频（限于来源于高清媒介的重编码，480p+）
- 无损音轨（FLAC 等）
- 5.1 声道及以上音轨
- PC 游戏（原版光盘镜像）
- 7 日内高清预告片
- 高清相关软件和文档

### 不允许的资源

- 总体积 < 100MB
- 标清 upscale
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD
- RealVideo（RMVB/RM）、FLV
- 有损 MP3/WMA（未达 5.1 声道）
- RAR 压缩文件
- 重复资源（dupe）
- 色情/敏感内容

### 发布者奖励

- 发布者获得双倍上传量

---

## 七、CookieCloud 配置

- 域名匹配：`xingwan.cc`（精确匹配）
- CookieCloud 域名：`xingwan.cc`
- 关键 cookie：`c_secure_pass`、`sl-session`

---

## 八、SiteFieldMapping 转发字段映射

### 转发到星湾时的字段映射

| 源字段 | 目标字段 | 说明 |
|--------|----------|------|
| `cat` | `type` | 分类，值需映射（见分类映射表） |
| `medium_sel` | `medium_sel[4]` | 媒介，值可直接使用 |
| `codec_sel` | `codec_sel[4]` | 编码，值可直接使用 |
| `standard_sel` | `standard_sel[4]` | 分辨率，值可直接使用 |
| `subtitle` | `small_descr` | 副标题 |
| `description` | `descr` | 简介（BBCode） |
| `mediainfo` | `technical_info` | MediaInfo |
| `anonymous` | `uplver` | 匿名发布 |

### 分类值映射（源站 → 星湾）

| 星湾分类 ID | 名称 | 常见源站对应 |
|-------------|------|-------------|
| 401 | 电影 | cat=401/401 |
| 402 | 电视剧 | cat=402/402 |
| 403 | 综艺 | cat=403/403 |
| 404 | 纪录片 | cat=404/404 |
| 405 | 动漫 | cat=405/405 |
| 406 | 音乐视频 | cat=406 |
| 407 | 体育 | cat=407 |
| 408 | 无损音乐 | cat=408 |
| 409 | 其他 | cat=409 |
| 410 | 短剧 | - |
| 411 | 游戏 | cat=411 |
| 412 | 软件 | cat=412 |
| 413 | 电子书 | cat=413 |
| 416 | 漫画 | - |

---

## 九、特殊说明

1. **标准 NexusPHP 站点**：所有上传/下载/解析逻辑均可使用 NexusPHP 通用适配器
2. **字段后缀 `[4]`**：所有质量选择器带 `[4]` 后缀（对应 data-mode='4'）
3. **无制作组选择器**：上传表单没有 `team_sel` 字段
4. **有候选制**：游戏类资源需要先在候选区提交
5. **建设初期**：2025-2026 年新建站，开放注册
6. **标签系统丰富**：支持禁转/首发/官组/DIY/国语/中字/HDR 等标签
7. **有 Telegram 社区群**

### 附：发布适配记录（v0.0.1295 §59.439）

- **form_config**：migration122 分发（六槽 110 项——type 14/medium 12/
  codec 11/standard 16/team 26/tags 31）。实时 HTML（47KB 代理）vs
  文档：**资产两槽缺口实时补全**（team_sel 27 项+tags 31 项 4 月
  完全未捕获——青蛙/学校/海的宝同款）+label 全面漂移（emoji+
  中文说明长形态——实时为准）+**文档 §1.1 称有 uplver——实时
  grep 零次=文档漂移（无匿名字段）**。
- **type 14（emoji label）**：410 短剧第十五站+416 漫画 label-only
  （dict 无 comics 键）+406=[mv,music_video]+408 双键+404 双键。
- **medium 12 全配键（值 10-21 全乱序）**：Blu-ray/BD(10)=[bluray,
  uhd_bluray] 双键归并（无 UHD 分立项）+Remux(11)=[remux,
  uhd_remux] 双键+Encode(12)/WEB-DL(13)/WEBRip(14) 三分立各单键+
  DVD/DVDR(16)=[dvd,dvdr] 站方合并项双键+**MiniBD(17)+HD DVD(18)
  精确键复利（大青虫先例）**+CD/Track/Other。
- **codec 11 全配键罕见格式大满贯**：AV1/VP9/Xvid/**DivX ext 活键**+
  **ProRes ext 活键（专业中间格式全库首配）**；DNxHD/DNxHR(14)
  label-only（ext 无键）；无 X26x 零联动。
- **standard 16 项全库最多级别**（3D 三形态 SBS/HSBS/MVC+IMAX+
  VR 180°/360° 罕见细分）：SD→8K 主链全配+**2K(9) label-only
  （1440p/QHD(10) 独立在表防一键多值——13城分辨率类先例）**+
  **4K Remux(12) label-only（媒介维度混入分辨率槽语义存疑——
  柠檬 4K/8K-UltraHD 同款保守不猜）**+3D/IMAX/VR label-only+
  Other=[resolution.other]；**16 项 label 站方 HTML 原文自带
  截断（"3D MVC / Blu-ray 3D （原生 3D 蓝光画"——§59.384 照抄
  原则）**。
- **team 27 外站官组大表（OKPT 28 组后第二例）**：CHD/WiKi/HDS/
  PTerWEB/FraMeSToR/TrollUHD/CtrlHD/FLUX/JoyHD/HQC/QHstudIo/DOA/
  EBP/HDT/EMP/NTb/TayTO/NGB/QxR/YIFY/CMCT/MWeb/MTeam/HHClub/
  HHWeb/**FRDS(30) 在表 DryRun 直中潜力**/Audiences 全 label-only+
  Other(24)=[team.other] 兜底。
- **tags 31 双区（分类镜像区 19+内容区 12）**：分类镜像区 label-only
  （type 域镜像零产出——TG吃瓜/9KG🈲/已刮削同）+**26 英字=
  [english_subtitle]（藏宝阁首配键——位置在分类区但语义内容标签）**；
  内容区：**1 禁转/2 首发 二绝不配**（官方专属财神雷区——本站无
  原创/官方标签）+**泛 HDR 键分配精细化首发**（7 HDR=[hdr10,hlg]
  泛家族剩余两键——**hdr10_plus 给 27 HDR10+ 独立项/hdr_vivid 给
  33 HDRVi 独立项，防一键多值三站首发**）+4 DIY/5 国语/6 中字/
  31 粤语/35 英语/29 特效=[special_effects_subs] 十二键配+32 4K/
  34 HDR10bit label-only（分辨率类/站方自定义语义无键）。
- 字段：type/medium_sel[4]/codec_sel[4]/standard_sel[4]/team_sel[4]/
  tags[4][]/small_descr/descr/technical_info；**nfo 不配（13城
  先例——域表无 nfo 域配了必 formdiff 假差异，formdiff 抓出即修）**；
  **无 uplver（文档漂移）+无 url/pt_gen/dburl/douban_id+无
  audiocodec 槽（第五站）**——质量信息仅靠选择器+标题传达。
- **验证**：裸对全槽一致+formdiff 0 条（nfo 移除后归零）+DryRun
  三形态（① WEB-DL 剧集→402+13+7+tags=[国语,中字,HDR10+]
  **hdr10_plus 细分正确归 27 独立项** ✓/② UHD 原盘→medium=10
  双键归并+codec=6+standard=11+tags=[HDR] 泛项+原盘标 miss 正确
  （本站无 original_disc 标签）✓/③ MI 压制→medium=12 Encode+
  codec=1 H.264 无 X26x 通用 ✓）+48 包。
- 站规人工把关：随机促销+自动免费（>20GB/原盘/每季首集）+YIFY/
  YTS 低码率"不建议收藏"+上传者资格；9KG🈲 标签=成人区镜像禁发。
- 遗留：首颗真实发布。
