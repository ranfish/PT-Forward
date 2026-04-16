# 财神PT (CSPT) 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 财神PT (CSPT) |
| 站点地址 | https://cspt.top |
| 站点框架 | NexusPHP |
| 主题 | BambooGreen（自定义金色主题） |
| 关联站点 | 无 |
| 特殊规则 | 官组 csweb/cspt、短剧 GodDramas、音乐 AGSVMUS、方舟 Hares |
| Wiki 规则 | https://wiki.cspt.top/zh/uploadrule |
| 油猴脚本 | CS-Torrent-Assistant-New v1.5.11（审种助手） |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名，要求规范填写） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（data-pt-gen="url"） |
| `pt_gen` | text | - | PT-Gen（data-pt-gen="pt_gen"，支持 imdb/douban/bangumi/indienova 4种来源） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |
| `technical_info` | textarea | - | MediaInfo |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

**注意**: PT-Gen 支持4种来源（imdb/douban/bangumi/indienova），比一般站点多了 bangumi 和 indienova。有"获取简介"按钮自动填充。

### 1.2 类型字段（`type`）— 必填，data-mode='4'

单模式发布，所有分类共享同一套质量字段。

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | HQ音乐 |
| 409 | 其他 |

**注意**: 油猴脚本中有额外分类映射（仅审种逻辑使用，不在 upload.php 表单中）：
- 410: 短剧（用于 GodDramas 官种）
- 411: Music（用于音乐分类检测）
- 412: Software
- 413: Game
- 415: E-Book
- 416: Comic（用于 Pack 制作组）
- 417: Education
- 418: Picture
- 419: Playlet（用于 GodDramas 短剧审种）

### 1.3 质量选择字段

字段名带 `[4]` 后缀，对应 mode=4 综合区。

#### 媒介（`source_sel[4]`）— 10个

**注意**: 字段名为 `source_sel` 而非 `medium_sel`。

| HTML值 | 显示名称 | 脚本映射值 | 说明 |
|--------|----------|------------|------|
| 7 | Blu-ray | 11 | 值不一致！HTML=7, 脚本=11 |
| 8 | UHD Blu-ray | 10 | 值不一致！HTML=8, 脚本=10 |
| 9 | Remux | 12 | 值不一致！HTML=9, 脚本=12 |
| 10 | Encode | 13 | 值不一致！HTML=10, 脚本=13 |
| 11 | WEB-DL | 14 | 值不一致！HTML=11, 脚本=14 |
| 12 | HDTV | 15 | 值不一致！HTML=12, 脚本=15 |
| 13 | DVD | 16 | 值不一致！HTML=13, 脚本=16 |
| 14 | CD | 17 | 值不一致！HTML=14, 脚本=17（有前导空格/tab） |
| 15 | Track | 18 | 值不一致！HTML=15, 脚本=18 |
| 16 | Other | 19 | 值不一致！HTML=16, 脚本=19 |

**⚠️ 关键发现**: 油猴脚本中的 `type_constant` 映射值（10-19）与 upload.php HTML 表单中的实际值（7-16）完全不同！脚本在审核详情页从文本匹配得到脚本内部值，在提交时需要转换。**实际发布时应使用 HTML 表单值（7-16）**。

脚本映射用于 details.php 审核页面显示，不用于 upload.php 提交。

#### 视频编码（`codec_sel[4]`）— 6个

| HTML值 | 显示名称 | 脚本映射值 | 说明 |
|--------|----------|------------|------|
| 1 | H.264/AVC | 6 | 值不一致 |
| 2 | H.265/HEVC | 7 | 值不一致 |
| 3 | VC-1 | 8 | 值不一致 |
| 4 | MPEG-2 | 9 | 值不一致 |
| 6 | AV1 | 10 | 值不一致，HTML跳过5 |
| 5 | Other | 11 | 值不一致 |

**注意**: HTML值不连续（4→6→5），AV1 的 HTML 值为 6 而非 5。

#### 音频编码（`audiocodec_sel[4]`）— 16个

| HTML值 | 显示名称 | 脚本映射值 |
|--------|----------|------------|
| 8 | ALAC | 8 |
| 9 | AAC | 9 |
| 10 | APE | 10 |
| 11 | TrueHD Atmos | 11 |
| 12 | DDP/E-AC3 | 12 |
| 13 | DD/AC3 | 13 |
| 14 | LPCM | 14 |
| 15 | TrueHD | 15 |
| 16 | DTS:X | 16 |
| 17 | DTS-HD MA | 17 |
| 18 | DTS | 18 |
| 19 | M4A | 19 |
| 20 | WAV | 20 |
| 21 | MP3 | 21 |
| 22 | FLAC | 22 |
| 23 | Other | 23 |

**注意**: 音频编码是唯一一组 HTML 值与脚本映射值完全一致的字段。

#### 分辨率（`standard_sel[4]`）— 7个

| HTML值 | 显示名称 | 脚本映射值 |
|--------|----------|------------|
| 4 | 480p/480i | 5 | 值不一致 |
| 5 | 720p/720i | 6 | 值不一致 |
| 6 | 1080p/1080i | 7 | 值不一致 |
| 10 | 2K/1440p/1440i | 11 | 值不一致 |
| 7 | 4K/2160p/2160i | 8 | 值不一致 |
| 8 | 8K/4320p/4320i | 9 | 值不一致 |
| 9 | Other | 10 | 值不一致 |

**注意**: HTML 值不按分辨率顺序排列（4,5,6,10,7,8,9）。

#### 制作组（`team_sel[4]`）— 5个

| HTML值 | 显示名称 | 脚本映射值 |
|--------|----------|------------|
| 19 | CSPT | - | 官组 |
| 18 | CSWEB | - | 官组 |
| 8 | HSPT | - | 官组 |
| 9 | HSWEB | - | 官组 |
| 5 | Other | 5 | |

**注意**: upload.php 表单中只有5个制作组选项（4个官组 + Other），但油猴脚本的 `group_constant` 中有8个制作组（HDS/CHD/MySiLU/WiKi/rain/rainweb/Tangweb/Other），这是审核详情页的历史映射，upload.php 已更新为新的官组体系。

**特殊情况**: 油猴脚本审核逻辑还会检测以下官组并映射到特定制作组：
- `AGSVPT`(6), `AGSVMUSIC`(20), `AGSVWEB`(21), `Hares`(23), `GodDramas`(24), `Pack`(16)

这些映射用于审核页面（details.php）显示，不在 upload.php 表单的 team_sel 中出现。

### 1.4 标签（`tags[4][]`）— 24个 checkbox

| HTML值 | 显示名称 |
|--------|----------|
| 23 | 驻站 |
| 22 | 零魔 |
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官种 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 13 | 独家 |
| 14 | 自压 |
| 15 | 重制 |
| 16 | 外挂字幕 |
| 10 | Remux |
| 17 | 大包 |
| 18 | 超分 |
| 19 | 补帧 |
| 12 | 粤语 |
| 20 | 特效 |
| 11 | 杜比 |
| 8 | 喜剧 |
| 21 | 分集 |
| 9 | 完结 |
| 24 | 儿童 |

**特殊标签说明**:
- **驻站(23)**: GodDramas 短剧官种必选
- **零魔(22)**: 独有标签
- **官种(3)**: 官种种子必选，非官种不可选
- **大包(17)**: 种子体积 > 1TB 时建议选择
- **喜剧(8)**: 从 PT-Gen 简介中"类别"字段自动检测（喜剧/搞笑/comedy/funny）
- **儿童(24)**: 从 PT-Gen 简介中"类别"字段自动检测（儿童/animated/child/kids）
- **分集(21)**: 标题含 S01E01 模式或副标题含"第X集"时必选
- **完结(9)**: 标题含"complete"且分类为电视剧/综艺/纪录片/动漫时必选

### 1.5 缺失字段

| 字段 | 状态 |
|------|------|
| `processing_sel[4]`（地区） | ❌ 无此字段 |

CSPT 无地区选择下拉。但油猴脚本审核逻辑中有地区映射（用于显示检测）：
- 1: Mainland(大陆), 2: Hongkong(香港), 3: Taiwan(台湾), 4: West(欧美), 5: Japan(日本), 6: Korea(韩国), 7: India(印度), 8: Russia(俄国), 9: Thailand(泰国), 99: Other(其他)

这些地区值仅在 details.php 审核页面显示，upload.php 中无此字段。

---

## 二、映射值对照表（HTML表单 vs 油猴脚本）

**核心结论**: 油猴脚本的常量映射值与 HTML 表单值完全不同（除音频编码外）。脚本用于审核详情页（details.php）的文本匹配显示，实际发布到 upload.php 时必须使用 HTML 表单值。

### 完整对照表

| 字段 | HTML表单值 | 脚本映射值 | 显示名称 |
|------|-----------|-----------|----------|
| **媒介** | | | |
| Blu-ray | 7 | 11 | Blu-ray |
| UHD Blu-ray | 8 | 10 | UHD Blu-ray |
| Remux | 9 | 12 | Remux |
| Encode | 10 | 13 | Encode |
| WEB-DL | 11 | 14 | WEB-DL |
| HDTV | 12 | 15 | HDTV |
| DVD | 13 | 16 | DVD |
| CD | 14 | 17 | CD |
| Track | 15 | 18 | Track |
| Other | 16 | 19 | Other |
| **编码** | | | |
| H.264/AVC | 1 | 6 | H.264/AVC |
| H.265/HEVC | 2 | 7 | H.265/HEVC |
| VC-1 | 3 | 8 | VC-1 |
| MPEG-2 | 4 | 9 | MPEG-2 |
| AV1 | 6 | 10 | AV1 |
| Other | 5 | 11 | Other |
| **音频** | | | |
| ALAC→Other | 8→23 | 8→23 | 一致 |
| **分辨率** | | | |
| 480p | 4 | 5 | 480p/480i |
| 720p | 5 | 6 | 720p/720i |
| 1080p | 6 | 7 | 1080p/1080i |
| 2K | 10 | 11 | 2K/1440p/1440i |
| 4K | 7 | 8 | 4K/2160p/2160i |
| 8K | 8 | 9 | 8K/4320p/4320i |
| Other | 9 | 10 | Other |

---

## 三、发种规则（Wiki）

### 3.1 做种规则

- 上传者必须实际拥有文件（本地或盒子）
- 上传速度不低于 1024 KB/s
- 做种时间至少 24 小时；若 24 小时内未出种（保种 ≥ 3），需持续做种到 48 小时
- 违规三次警告，再犯封禁

### 3.2 不允许的资源和文件

**影视**:
- 标清低质：CAM、TC、TS、SCR、DVDSCR、R5、R5.Line、HalfCD
- 禁止格式：RealVideo（RMVB/RM）、FLV
- 禁止带 BT 站点信息的资源
- 剧集（含动漫/番）：禁止跨季不同分辨率打包、禁止完结剧集拆分、禁止未完结剧集超过1周的分集
- 电影：严禁私人合集打包（导演/演员/IMDb Top 250/豆瓣 Top 250），仅允许专业官组

**音乐**:
- 未达 5.1 声道标准的有损音频（MP3、WMA 等）
- 无正确 CUE 表单的多轨音频
- 自定义歌单（仅允许艺术家正式专辑）

**其他**:
- 软体（游戏、办公软件等）
- RAR 等压缩文件
- 禁忌/敏感内容
- 垃圾文件

### 3.3 Dupe 判定规则

"质量重于数量"：

1. 完结资源代替分集资源
2. 跨季资源代替季数被覆盖的跨季资源（S01-S08 代替 S01-S07）
3. 高清代替完全重复的低清
4. 各个压制版允许共存
5. 压制版与原盘允许共存
6. 各个 DIY 原盘资源允许共存
7. 蓝光代替更低清晰度且完全重复的 WEB/HDTV
8. 动漫区例外：蓝光对 TV 版大幅修正时 TV 保留；仅有 WEB-DL 的高清旧番也保留

**被代替资源将会被删除。**

### 3.4 打包规则

- 电影：仅允许发行商官方原盘合集（允许 DIY/Remux/Encode 衍生品）
- 电视剧/综艺/动漫：整季打包
- 纪录片：同一专题打包
- 音乐：5张或以上专辑打包
- 发布组打包发布的资源

打包要求：相同媒介、相同分辨率、编码一致、发布组统一（电影合集）。

### 3.5 制种规则

- 禁止非官方超分处理/补帧资源
- 禁止广告文件、病毒、木马、种中种
- 文件名 ≤ 100中文/200英文，无特殊符号
- 分块大小 ≤ 16MB
- 推荐单文件也套一层文件夹制种

### 3.6 转种规则

- 不准转载禁转种
- 禁止转载黑/灰名单制作组资源
- 不建议转载找不到出处的资源（可加 -NoGroup/-NoGrp，自担责任）
- 禁止转载简易机翻
- 直接上传原种以便辅种
- 标题/副标题需改成本站对应格式
- 务必写明种子出处
- 建议预览图下载后重新上传

**推荐图床**: Freeimage, Postimages, imgbox, pixhost

---

## 四、黑名单与灰名单

### 4.1 黑名单

**绝对黑名单**:
- DBD-Raws（盗用资源、超分、劣迹斑斑）
- Skymoon/天月/HKACG（反华组）
- c.c动漫（改名组）
- 猎户发布组/orion origin、爪爪字幕组/ZhuaZhuaStudio（机翻组）

**盗转/改名发布组（展开列表，40+个）**:
ALT, BATWEB, BestWEB, BitsTV, BlackTV, CatWEB, CTRLHD(非CtrlHD), CnSCG, DDHDTV, DreamHD, DWR, EntTV, FGT, GameHD, GPTHD, HotTV, HotWEB, Huawei, Huluwa, LelveTV, MiniHD, MOMOWEB, Mp4Ba, NSBC, NukeHD, PandaMoon, ParkHD, SeeHD, SeeWeb, SmY, SonyHD, TagWeb, TBMaxUB, VeryPSP, Xiaomi, xunlei, XJCTV, XLMV, ZeroTV

**油猴脚本检测关键词**（与黑名单重叠但更广）:
fgt, hao4k, mp4ba, rarbg, gpthd, seeweb, dreamhd, blacktv, xiaomi, huawei, momohd, ddhdtv, nukehd, tagweb, sonyhd, minihd, bitstv, -alt, batweb, dbd-raws, xunlei, zerotv, lelvetv, Quark, mp4, mkv

**注意**: 脚本检测 `mp4` 和 `mkv` 作为关键词，可能误判标题中包含这些扩展名的合法种子。

### 4.2 灰名单

- 异域11番小队、加刘景长（低码率）
- Reinforce（高体积渣画质）

---

## 五、油猴审种脚本分析

### 5.1 官方组识别

```
config.official_tags = ["csweb", "cspt"]
```

检测标题中是否包含 csweb 或 cspt（不区分大小写）。

### 5.2 特殊官种类型

| 标题关键词 | 类型 | 特殊要求 |
|-----------|------|----------|
| `goddramas` | 短剧(GodDramas) | 必须选择分类=419、禁转标签、驻站标签 |
| `agsvmus` | 音乐官种(AGSVMUS) | 仅检查制作组是否选择，其他检查跳过 |
| `hares` | 方舟计划(Hares) | 必须选择方舟标签+制作组=Hares(23) |

### 5.3 审核检查项

脚本对 details.php 页面执行以下审核检查：

**错误（必须修复）**:
- 主标题包含中文/中文字符
- 副标题为空
- 标签为空
- 标签与简介类别不一致（儿童/喜剧）
- 未选择分类/媒介/编码/音频/分辨率
- 标题检测值与选择值不匹配（媒介/编码/音频/分辨率）
- 完结剧集未添加完结标签
- 媒体信息未解析（官种）
- MediaInfo 中含有 BBCode
- 官种未选择制作组/未选择官种标签
- 非官种不可选择官种标签
- 检测为未信任制作组
- GodDramas 短剧需选择短剧类型+驻站标签
- 方舟标签与种子不匹配
- 简介中包含 MediaInfo 文本
- 未填写影片简介（需含"片名"或"译名"）
- 缺少海报或截图（< 2张图片）
- Mediainfo 栏为空
- 官种主标题编码与 MediaInfo 不一致（x264/x265）

**警告（建议修复）**:
- 4K/8K 资源请检查是否有更高清资源
- 未选择中字标签（MediaInfo 检测到中文字幕）
- 未选择大包标签（体积 > 1TB）
- 多余的影片参数/MediaInfo 图片
- 图片加载失败
- 页面图片加载 30 秒超时

**豁免分类**: 当分类为 413(Game)/418(Picture)/415(E-Book)/412(Software)/411(Music)/406(MV)/408(HQ Audio) 时，跳过大部分检查。

### 5.4 禁止图床

```
banPictureBed = ['rains3.com', 'img.m-team.cc', 'totheglory.im/details', 'i.miji.bid', 'duan.red']
```

### 5.5 MediaInfo 语言检测

脚本从 MediaInfo 文本中提取：
- 音频语言 → 检测是否为国语（Chinese/Mandarin）
- 字幕语言 → 检测是否为中字（Chinese）/英字（English）
- 编码格式 → 检测 x264/x265

---

## 六、字段映射汇总（实际发布用）

> 以下为 upload.php 表单中的实际值，适配器实现时应使用这些值。

### 6.1 类型（`type`）

```json
{
  "电影": 401,
  "电视剧": 402,
  "综艺": 403,
  "纪录片": 404,
  "动漫": 405,
  "MV": 406,
  "体育": 407,
  "HQ音乐": 408,
  "其他": 409
}
```

### 6.2 媒介（`source_sel[4]`）

```json
{
  "Blu-ray": 7,
  "UHD Blu-ray": 8,
  "Remux": 9,
  "Encode": 10,
  "WEB-DL": 11,
  "HDTV": 12,
  "DVD": 13,
  "CD": 14,
  "Track": 15,
  "Other": 16
}
```

### 6.3 视频编码（`codec_sel[4]`）

```json
{
  "H.264/AVC": 1,
  "H.265/HEVC": 2,
  "VC-1": 3,
  "MPEG-2": 4,
  "AV1": 6,
  "Other": 5
}
```

### 6.4 音频编码（`audiocodec_sel[4]`）

```json
{
  "ALAC": 8,
  "AAC": 9,
  "APE": 10,
  "TrueHD Atmos": 11,
  "DDP/E-AC3": 12,
  "DD/AC3": 13,
  "LPCM": 14,
  "TrueHD": 15,
  "DTS:X": 16,
  "DTS-HD MA": 17,
  "DTS": 18,
  "M4A": 19,
  "WAV": 20,
  "MP3": 21,
  "FLAC": 22,
  "Other": 23
}
```

### 6.5 分辨率（`standard_sel[4]`）

```json
{
  "480p": 4,
  "720p": 5,
  "1080p": 6,
  "2K": 10,
  "4K": 7,
  "8K": 8,
  "Other": 9
}
```

### 6.6 制作组（`team_sel[4]`）

```json
{
  "CSPT": 19,
  "CSWEB": 18,
  "HSPT": 8,
  "HSWEB": 9,
  "Other": 5
}
```

### 6.7 标签（`tags[4][]`）

```json
{
  "禁转": 1,
  "首发": 2,
  "官种": 3,
  "DIY": 4,
  "国语": 5,
  "中字": 6,
  "HDR": 7,
  "喜剧": 8,
  "完结": 9,
  "Remux": 10,
  "杜比": 11,
  "粤语": 12,
  "独家": 13,
  "自压": 14,
  "重制": 15,
  "外挂字幕": 16,
  "大包": 17,
  "超分": 18,
  "补帧": 19,
  "特效": 20,
  "分集": 21,
  "零魔": 22,
  "驻站": 23,
  "儿童": 24
}
```

---

## 七、CSPT 特殊注意事项

### 7.1 脚本值与表单值不一致

这是 CSPT 最关键的特殊点。油猴审种脚本的 `type_constant`/`encode_constant`/`resolution_constant` 中的值与 upload.php HTML 表单 `<option value="...">` 中的值**完全不同**（音频编码除外）。

原因推测：脚本改编自 SpringSunday 审种助手（脚本注释"改自SpringSunday-Torrent-Assistant"），保留了原站点的映射值体系，而 CSPT 的 NexusPHP 数据库中 source_sel/codec_sel/standard_sel 使用了不同的自增值序列。

**适配器实现建议**: 完全忽略油猴脚本中的映射值，只使用 upload.php HTML 表单值。

### 7.2 source_sel 而非 medium_sel

CSPT 媒介字段名为 `source_sel[4]` 而非其他站点常见的 `medium_sel[4]`。

### 7.3 短剧分类

CSPT 有短剧（GodDramas）特殊分类，标题含 `goddramas` 的种子需：
- 分类选择 419（Playlet）
- 标签选择：禁转 + 驻站
- 制作组选择

### 7.4 方舟计划

标题含 `hares` 的种子需选择方舟相关标签和制作组。

### 7.5 音乐官种豁免

`AGSVMUS` 官方音乐种子会清空大部分审核检查，仅保留制作组检查。

### 7.6 标题规范

- 主标题不允许包含中文（除少数白名单如 ￡、™、罗马数字等）
- 标题格式示例：`Blade Runner 1982 Final Cut 720p HDDVD DTS x264-ESiR`
- 副标题示例：`银翼杀手 720p @ 4615 kbps - DTS 5.1 @ 1536 kbps`

### 7.7 禁止的超分/补帧

规则禁止非官方超分处理/补帧资源，但标签中有"超分(18)"和"补帧(19)"，说明官方组发布的超分/补帧是允许的。

### 7.8 删除种子规则

- 禁止种子：立即删除
- 分集种子：月初删除完结剧集的分集
- 无做种无出种：3天后删除
- 需修改种子：15天未修改则删除

---

## 八、适配器实现要点

### 8.1 字段命名

```go
SourceSelField: "source_sel[4]",   // 注意：非 medium_sel
CodecSelField:  "codec_sel[4]",
AudioSelField:  "audiocodec_sel[4]",
StandardSelField: "standard_sel[4]",
TeamSelField:   "team_sel[4]",
TagsField:      "tags[4][]",
TypeField:      "type",
```

### 8.2 映射值

所有映射值使用 HTML 表单值（参见第六节），**不要使用油猴脚本中的常量值**。

### 8.3 标签逻辑

标签需从多个来源综合判断：
1. **标题分析**: 禁转/官种/分集/完结
2. **MediaInfo 分析**: 国语/中字/x264/x265
3. **体积判断**: 大包（>1TB）
4. **PT-Gen 类别**: 喜剧/儿童
5. **特殊官种**: 驻站（GodDramas）

### 8.4 黑名单检测

发布前需检查标题是否包含黑名单制作组关键词。注意 `mp4` 和 `mkv` 关键词可能误判。

---

## 九、与其他 NexusPHP 站点对比

| 特征 | CSPT | 常见 NexusPHP |
|------|------|---------------|
| 媒介字段名 | `source_sel` | `medium_sel` |
| 媒介数量 | 10 | 通常 6-13 |
| 制作组数量 | 5（仅官组） | 通常 3-30 |
| 标签数量 | 24 | 通常 3-21 |
| 地区字段 | 无 | 通常有 |
| 分辨率顺序 | 非顺序（4,5,6,10,7,8,9） | 通常按分辨率排列 |
| 短剧分类 | 有（419） | 少见 |
| 方舟计划 | 有 | 独有 |
| PT-Gen 来源 | 4种（imdb/douban/bangumi/indienova） | 通常 2-3种 |
| 脚本值≠表单值 | 是（全部字段除音频） | 通常一致 |

---

*数据来源: upload.php HTML (2026-04-16), Wiki规则 (2025-11-05更新), 油猴脚本 v1.5.11*
*文档创建: 2026-04-16*
