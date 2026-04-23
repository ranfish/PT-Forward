# 财神 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 财神|
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
| `uplver` | checkbox | - | 匿名发布（value="yes"，默认未勾选） |

**注意**: PT-Gen 支持4种来源（imdb/douban/bangumi/indienova），比一般站点多了 bangumi 和 indienova。有"获取简介"按钮自动填充。

**隐含规则**: 简介中包含 IMDb 链接时，`url` 字段（IMDb 链接）**必须填写**。Wiki 标注红星必填。

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

### 7.8 删除种子规则（暂实施）

- 禁止种子：种审看到将立刻删除
- 分集种子：月初删除完结剧集的分集
- 无做种无出种：3天后删除
- 需修改种子：15天内未修改则删除

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

## 十、Wiki 发种教程（howtoupload）

> 来源：https://wiki.cspt.top/zh/howtoupload（2025-10-11 更新，编辑者 riunc）

### 10.1 标题命名规范

#### 电影类

**主标题**：`英文名 年份 分辨率 介质 视频编码 音频编码-制作组`

**副标题**：`中文名 演员（可选） 音轨字幕信息（可选） 点评（可选）`

**示例**：
- 主标题：`Flying Colors 2015 1080p BluRay x265 DTS 5.1-PTer`
- 副标题：`垫底辣妹 | 导演:土井裕泰 主演:有村架纯 [日语] [简繁英字+章节]`

#### 剧集类

**主标题**：`电视台名（可选） 英文名 季数/集数 年代 分辨率 介质 视频编码 音频编码-制作组`

**副标题**：`中文名 季数/集数 演员（可选） 音轨字幕信息（可选） 点评(可选)`

**示例**：
- 主标题：`The Learning Curve Of A Warlord S01 2018 1080p WEB-DL H264 AAC 2Audios-PTer`
- 副标题：`大帅哥 全集| 主演: 张卫健 蔡思贝 [国粤双语] [内嵌简体中字]`

#### 综艺类

**主标题**：`电视台名（可选） 英文名 播出日期 集数 分辨率 介质 视频编码 音频编码-制作组`

**副标题**：`中文名 参与人员（可选） 音轨字幕信息（可选） 点评（可选）`

**示例**：
- 主标题：`BTV New Year's concert 20181231 1080p WEB-DL H264 AAC-PTerWEB`
- 副标题：`北京卫视2019环球跨年冰雪盛典 | 腾格尔版硬核日不落`

#### 音乐类

中国艺术家用中文名，外国用英语名。

**主标题**：`艺术家名 - 专辑名 发行年份 - 文件格式 采样位深 采样频率 - 制作组`

**副标题**：`艺术家其他名 - 专辑其他名 | 发行类别 | 厂牌（可选） 目录号（可选） 版本等其他信息（可选） | 转载信息（可选）`

**示例 1**：
- 主标题：`林俊杰 - 学不会 2011 - FLAC 16bit 44.1kHz - PTerMUSIC`
- 副标题：`JJ Lin - Lost N Found 2011 | Warner Music CD / Lossless / Log (100%) / Cue`

**示例 2**：
- 主标题：`Taylor Swift - Lover 2019 - WAV 16bit 44.1khz`
- 副标题：`泰勒·斯威夫特 | Republic Records / 602577928222 / Deluxe Edition`

### 10.2 标题细节要素

#### 必选要素

| 要素 | 说明 |
|------|------|
| 片名 | 按 IMDb 命名，除非不正确或不存在；中文内容可在豆瓣找到，中文名置于副标题开头；所有标点必须保留，大小写参照 IMDb |
| S##E## | 单集 S##E##，单季合集 S##，连续多季 S##-S## |
| 年份 | 发行/放映年份，参考 IMDb/豆瓣 |
| 分辨率 | 480i/480p/576i/576p/720p/1080i/1080p/2160p/4320p，DVD 可省略 |

#### 条件要素

| 要素 | 适用条件 | 可选值 |
|------|---------|--------|
| 剪辑 | 省略=剧场版 | Director's Cut、Extended、Special Edition、Unrated、Uncut、Super Duper Cut、FanRes 等 |
| 比例 | 省略=原始纵横比 | IMAX、Open Matte、MAR |
| HYBRID | 多源混合时 | 如含 DV 层信息来自其他源 |
| REPACK | 重新发布时递增 | REPACK、REPACK2 |
| PROPER | 修正发布时递增 | PROPER、PROPER2 |
| 版本 | 非必需 | 20th Anniversary Edition、Remaster、4K Remaster、Criterion Collection、Limited Edition、60FPS 等 |
| 地区 | 仅原盘 | CEE、GER、HKG 等 |
| HDR | 省略=SDR | HDR10、HDR10+、DV、DV HDR10、DV HDR10+、HLG、PQ10、HDR Vivid |
| Hi10P | 10-bit AVC/H264/x264 | DVD 原盘可省略 |

#### 来源（介质）详细映射

| 来源类型 | 标题中写法 | 说明 |
|---------|-----------|------|
| 原盘（Blu-ray） | BluRay、3D BluRay、UHD BluRay | 也可写 NTSC DVD5/DVD9、PAL DVD5/DVD9、HDDVD、Blu-ray、3D Blu-ray、UHD Blu-ray |
| Encode（压制） | BluRay、DVDRip 等 | 来源 DVD 时写 DVDRip |
| REMUX（重封装） | BluRay REMUX、UHD BluRay REMUX 等 | 去除冗余内容的原盘重封装 |
| WEB-DL | WEB-DL | 流媒体解密重混流，与原始质量相同；外国平台可加前缀：NF（Netflix）、AMZN（Amazon）、DSNP（Disney+） |
| WEBRip | WEBRip | HDMI 采集或 WEB-DL 重编码，编码器/分辨率可能改变；cXcY@FRDS 是典型 WEBRip 发布者 |
| HDTV | HDTV、UHDTV | 电视台录制（含台标/广告）；HDTV 的 Encode 仍写 HDTV |

#### 视频编码规范

| 来源 | 应写 | 禁止写 |
|------|------|--------|
| 原盘/REMUX | AVC、HEVC、MPEG-2、VC-1 | x264、x265 |
| WEB-DL/HDTV | H264、H265、x264、x265、MPEG-2、VP9、AV1、AVS+、AVS3 | AVC、HEVC |
| Encode/WEBRip | x264、x265、MPEG-2、VP9、AV1、Xvid、DivX | AVC、HEVC |

> **Wiki 明确规定**：压制重编码作品使用 x264 或 H.264 / x265 或 H.265，**不要使用 AVC、HEVC**。蓝光原盘使用 AVC、HEVC。

#### 音频编码与声道

**音频编码**：DD（或 AC3）、DD EX、DDP（或 DD+、EAC3）、DD+ EX、TrueHD、DTS、DTS-ES、DTS-HD MA、DTS-HD HRA、DTS:X、PCM、LPCM、FLAC、ALAC、AAC、OPUS、MP3、MP2

**声道数**（默认 2.0 省略）：1.0、2.0、3.0、4.0、5.0、5.1、5.1.4、6.1、7.1、9.1、11.1

**音频对象**（省略=无）：Atmos、Auro3D（注：DTS:X 既是编码也是对象，写在编码部分即可）

**音轨**：双音频/多语言配音时标明，如 `2Audio`

**多音轨优先级**：高码率优先；优先使用 DDP 和 DD，而非 EAC3 和 AC3

### 10.3 简介栏要求

**固定顺序**（不可调换）：

1. **引用框** — 必须放在最顶部，用 `[quote]` 包裹，含声明/源文件信息/感谢信息等
2. **海报** — 使用免费图床
3. **简介内容** — 非必要不使用英文版
4. **截图** — 3 张截图或 1 张清晰的九宫格；有字幕的最好截取到带字幕的画面

**转载要求**：禁止删除引用信息（除非含联系信息和 BT 站点等非 PT 内容）

**推荐图床**：Freeimage、Postimages、imgbox、pixhost

### 10.4 MediaInfo 要求

- 一定不要放到简介栏中
- 软件手动获取的使用英文原版
- 转载的（大站自定义精简的）需删除 BBCode 后粘贴到对应栏目

### 10.5 标签详细说明

**常用标签**：

| 标签 | 说明 |
|------|------|
| 国语 | 包含国语音轨（配音或音乐） |
| 中字 | 内嵌中文硬字幕、内封中文软字幕或种子内含外挂中文字幕；种子内禁止二次添加外挂字幕；若种子无字幕但发种人另行上传字幕区的也适用 |
| 粤语 | 包含粤语音轨（配音或音乐） |
| 喜剧 | 资源内容为喜剧类型 |
| 分集 | 资源为分集形式发布 |
| 完结 | 剧集/动画/系列已全部更新完毕，一整季也可选完结 |
| 儿童 | 适合儿童观看的内容 |
| 官种 | 本站工作组直接发布，标准化命名和高质量保证 |

**不常用标签**：

| 标签 | 说明 |
|------|------|
| 驻站 | 长期合作的资源发布者 |
| 禁转 | 官方组或自制资源，禁止未授权转载 |
| 首发 | 本站首次发布 |
| DIY | 重新编辑/优化/添加内容（如字幕、音轨） |
| HDR | 支持高动态范围技术 |
| 独家 | 仅在本站发布 |
| 自压 | 发布者自行压制（Encode） |
| 重制 | 对旧版修复/优化/重新编码 |
| 外挂字幕 | 附带单独的外挂字幕文件 |
| Remux | 无损提取原盘重封装（主标题必含 REMUX 字样） |
| 大包 | 多部作品/多季/大量相关资源的合集包 |
| 超分 | AI/算法提升分辨率或画质 |
| 补帧 | 插帧技术提高流畅度（如 24fps→60fps） |
| 特效 | 包含特效字幕（动态/注释/多风格） |
| 杜比 | 支持杜比视界或杜比全景声 |

---

## 审核脚本完整逆向分析

### 脚本信息

| 项目 | 内容 |
|------|------|
| 名称 | CS-Torrent-Assistant-New |
| 来源 | Greasyfork #544521 |
| 版本 | 1.5.11 |
| 作者 | Exception & 7ommy & wiiii & shun |
| 大小 | 1615 行 / 73KB |
| 运行页面 | `details.php*`（详情页）+ `edit.php*`（编辑页）+ 审核弹窗页 |
| 权限 | GM_setValue / GM_getValue / GM_registerMenuCommand / GM_xmlhttpRequest |
| 基于上游 | SpringSunday-Torrent-Assistant（经 Agsv-Torrent-Assistant 改写） |
| 域名 | cspt.top / cspt.cc / cspt.date（三域名通用） |

> **注意**：该脚本基于末日脚本改写，结构高度相似但**所有字段 ID 完全不同**（媒介/编码/音频/分辨率 ID 全部重编）。

### 常量映射

#### 分类 (categoryMapping) — 18+2 个

| ID | 英文名称 | 中文名称 |
|----|---------|---------|
| 401 | Movie | 电影 |
| 402 | TV Series | 电视剧 |
| 403 | TV Shows | 综艺 |
| 404 | Documentaries | 纪录片 |
| 405 | Anime | 动漫/动画 |
| 406 | MV | - |
| 407 | Sports | 体育 |
| 408 | Audio | 音频 |
| 409 | Misc | 其他 |
| 411 | Music | - |
| 412 | Software | - |
| 413 | Game | - |
| 415 | E-Book | - |
| 416 | Comic | - |
| 417 | Education | - |
| 418 | Picture | - |
| 419 | Playlet | 短剧 |
| 410 | - | 短剧（中文名映射） |

> **注意**：支持英文名和中文名双向匹配。419(Playlet)和410(短剧)两个 ID 都映射到短剧。410 仅中文名匹配使用。

#### 媒介 (type_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 10 | UHD Blu-ray | `uhd blu-ray`/`uhd bluray`/` uhd ` |
| 11 | Blu-ray | `blu-ray`/`bluray`（无 uhD/264/265） |
| 12 | Remux | `remux` |
| 13 | Encode | `av1`/`encode`/`264`/`265`/`bluray`+编码词 |
| 14 | WEB-DL | `web-dl`/`webdl`/`webrip`/`web-rip` |
| 15 | HDTV | `hdtv` |
| 16 | DVD | `dvd` |
| 17 | CD | `cd` |
| 18 | Track | `track` |
| 19 | Other | 默认值 |

> **关键差异**：ID 从 10 起始（非标准 1）。WEB-DL 和 WEBRip 合并为同一 ID(14)。AV1 标题优先检测为 Encode(13)。含 264/265 的标题检测为 Encode 而非 Blu-ray。

#### 视频编码 (encode_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 6 | H.264/AVC | `264`/`avc` |
| 7 | H.265/HEVC | `265`/`hevc` |
| 8 | VC-1 | `vc`/`vc-1` |
| 9 | MPEG-2 | `mpeg2`/`mpeg-2` |
| 10 | AV1 | `av1`/`av-1` |
| 11 | Other | - |

#### 音频编码 (audio_constant) — 16 个

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 22 | FLAC | `flac` |
| 14 | LPCM | `lpcm` |
| 12 | DDP/E-AC3 | ` ddp`/` dd+`/`E-?AC-?3` |
| 9 | AAC | `aac` |
| 13 | DD/AC3 | ` ac3`/` dd` |
| 11 | TrueHD Atmos | `truehd`+`atmos` |
| 17 | DTS-HD MA | `dts-hd ma`/`dts-hdma`/`dts-hd` |
| 16 | DTS:X | `dts:x`/`dtsx`/`dts-x` |
| 18 | DTS | `dts`（排除 dts-x） |
| 8 | ALAC | `alac` |
| 10 | APE | `ape` |
| 15 | TrueHD | `truehd`（无 atmos） |
| 19 | M4A | `m4a` |
| 20 | WAV | `wav` |
| 21 | MP3 | `mp3` |
| 23 | Other | 默认值 |

> **注意**：ID 编号与所有其他站点完全不同。FLAC=22, DTS=18, AAC=9。ALAC(8) 和 APE(10) 在 DTS 之前匹配（与末日不同）。

#### 分辨率 (resolution_constant)

| ID | 名称 | 标题匹配 |
|----|------|---------|
| 5 | 480p/480i | `480p`/`480i` |
| 6 | 720p/720i | `720p`/`720i` |
| 7 | 1080p/1080i | `1080p`/`1080i` |
| 8 | 4K/2160p/2160i | `4k`/`2160p`/`2160i`/`uhd` |
| 9 | 8K/4320p/4320i | `8k`/`4320p`/`4320i` |
| 10 | Other | 默认值 |
| 11 | 2K/1440p/1440i | `1440p`/`1440i` |

> **注意**：ID 从 5 起始，非连续。2K/1440p(11) 独立 ID。480p(5) 在 720p(6) 之前检测。

#### 地区 (area_constant) — 空定义（同末日）

```
area_constant = {} // 空对象，不检测地区
```

#### 制作组 (group_constant) — 8 个

| ID | 名称 |
|----|------|
| 4 | WiKi |
| 3 | MySiLU |
| 1 | HDS |
| 2 | CHD |
| 5 | Other |
| 7 | rainweb |
| 6 | rain |
| 8 | Tangweb |

> **注意**：跨站制作组（WiKi/MySiLU/HDS/CHD 为知名压制组）。rain/rainweb/Tangweb 为财神相关官组。

### 官组检测逻辑

```
标题含 "csweb" 或 "cspt" → officialSeed (AGSVPT 官种)
标题含 "goddramas"         → godDramaSeed (驻站短剧)
标题含 "agsvmus"           → officialMusicSeed (音乐官种)
标题含 "hares"             → haresSeed (白兔/方舟)
```

> **注意**：official_tags 可配置（`config.official_tags = ["csweb", "cspt"]`）。财神复用了末日的 AGSVPT/AGSVMUSIC/GodDramas/Hares 检测代码但含义可能不同。

### 禁止图床

```
rains3.com, img.m-team.cc, totheglory.im/details, i.miji.bid, duan.red
```

### 简介内容检测

```
简介含 IMDb/豆瓣/TMDb 链接 → dbUrl=true（检查已注释，不强制）
简介含 MediaInfo 关键词 → isBriefContainsInfo=true
简介含 "禁止转载" → isBriefContainsForbidReseed=true
简介含 "片名" 或 "译名" → isBriefContainsMovieBrief=true（缺失则报错）
简介类别字段 → 检测儿童/喜剧关键词 → 与标签交叉验证
```

### 标签-简介类别交叉验证（财神独有）

```
1. 从简介"类别"或"Keywords"字段提取关键词
2. 检测是否含儿童类关键词：儿童/animated/child/kids
3. 检测是否含喜剧类关键词：喜剧/搞笑/comedy/funny
4. 与标签区的"儿童"/"喜剧"标签交叉验证
5. 不一致 → 错误
```

### 标题解析算法

```
1. 获取 h1#top 文本
2. 清除后缀标签（同末日）
3. 检测「禁转」→ exclusive=1
4. 标题转小写 → title_lowercase
5. 正则匹配链：
   ├── 媒介(type)：
   │   WEB-DL/WEBRip→14, Remux→12, AV1→Encode(13),
   │   bluray(无uhd/264/265)→Blu-ray(11), HDTV→15,
   │   encode/264/265→Encode(13), UHD Blu-ray→10,
   │   DVD→16, CD→17, Track→18, Other→19
   ├── 编码(encode)：264/avc→6, 265/hevc→7, vc-1→8, mpeg2→9, av1→10
   ├── 音频(audio)：flac→22, lpcm→14, ddp/eac3→12, aac→9, ac3/dd→13,
   │   truehd+atmos→11, dts-hd ma→17, dts:x→16, dts→18,
   │   alac→8, ape→10, truehd→15, m4a→19, wav→20, mp3→21, other→23
   ├── 分辨率(resolution)：
   │   1440p→11, 1080p/i→7, 720p/i→6, 480p/i→5,
   │   4k/2160p/uhd→8, 8k/4320p→9, Other→10
   ├── 完结：`complete` → title_is_complete
   ├── 分集：`S\d+E\d+` → title_is_episode
   └── 制作组检测：csweb/cspt/goddramas/agsvmus/hares
```

> **关键差异**：媒介检测优先级与末日完全不同。WEB-DL/WEBRip 优先级最高(14)。AV1 关键词触发 Encode(13)。含 264/265 的标题优先判定为 Encode 而非 Blu-ray。

### 校验规则 — 共 30+ 项

#### 标题校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 1 | 禁止图床图片 | 域名匹配 banPictureBed | 错误 |
| 2 | 标题含中文/非ASCII | `[^\x00-\xff]`（排除特殊符号） | 错误 |
| 3 | 不信任制作组 | 关键词列表（含 Quark/mp4/mkv） | 错误 |

#### 副标题/标签/分类校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 4 | 副标题为空 | `!subtitle` | 错误 |
| 5 | 标签为空 | `hasBQ` | 错误 |
| 6 | 儿童 标签与简介类别不一致 | `bqByz1` | 错误 |
| 7 | 喜剧 标签与简介类别不一致 | `bqByz2` | 错误 |
| 8 | 副标题含"动画"但未选 Anime | `isSubtitleAnime && cat!==405` | 错误 |
| 9 | 未选择分类 | `!cat` | 错误 |
| 10 | 完结剧集未选完结标签 | `title_is_complete && !is_complete` | 错误 |

#### 字段选择校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 11 | 未选择媒介 | `!type` | 错误 |
| 12 | 标题媒介与选择不一致 | `title_type !== type` | 错误 |
| 13 | 未选择视频编码 | `!encode` | 错误 |
| 14 | 标题编码与选择不一致 | `title_encode !== encode` | 错误 |
| 15 | 未选择音频编码 | `!audio` | 错误 |
| 16 | 标题音频与选择不一致 | `title_audio !== audio` | 错误 |
| 17 | 未选择分辨率 | `!resolution` | 错误 |
| 18 | 标题分辨率与选择不一致 | `title_resolution !== resolution` | 错误 |

#### 简介与媒体信息校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 19 | 未填写影片简介 | `!isBriefContainsMovieBrief`（缺"片名"/"译名"） | 错误 |
| 20 | 官种媒体信息未解析 | `officialSeed && short===full` | 错误 |
| 21 | MediaInfo 含 BBCode | `[/b]/[/color]` 等标签 | 错误 |
| 22 | 简介含 MediaInfo | `isBriefContainsInfo` | 错误 |
| 23 | 缺少海报或截图 | `imgCount < 2` | 错误 |
| 24 | MediaInfo 栏为空 | `isMediainfoEmpty` | 错误 |
| 25 | 官种 MI 含 x264 但标题无 x264 | MI vs 标题交叉验证 | 错误 |
| 26 | 官种 MI 含 x265 但标题无 x265 | MI vs 标题交叉验证 | 错误 |

#### 标签与制作组校验

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 27 | 官种未选制作组 | `officialSeed && !isGroupSelected` | 错误 |
| 28 | GodDramas 禁转但未选禁转标签 | 简介含"禁止转载" | 错误 |
| 29 | GodDramas 未选短剧分类 | `godDramaSeed && cat!==419` | 错误 |
| 30 | GodDramas 未选驻站标签 | `godDramaSeed && !isTagResident` | 错误 |
| 31 | 非官种选了官方标签 | `!officialSeed && isOfficialSeedLabel` | 错误 |
| 32 | 官种未选官方标签 | `officialSeed && !isOfficialSeedLabel` | 错误 |
| 33 | 分集未选分集标签 | `!isEpisode && (title_is_episode\|subtitle_is_episode)` | 错误 |
| 34 | 非方舟选了方舟标签 | `isTagArcProj && !haresSeed` | 错误 |
| 35 | 方舟未选方舟标签 | `!isTagArcProj && haresSeed` | 错误 |
| 36 | Hares 制作组选择错误 | `haresSeed && category!==23` | 错误 |
| 37 | 非 Hares 选了 Hares | `!haresSeed && category===23` | 错误 |

#### 音乐分类特殊规则

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| 38 | 音乐标题缺采样频率 | `cat===411 && !khz` | 错误 |
| 39 | 音乐标题缺比特率 | `cat===411 && !bit` | 错误 |

#### 警告类规则

| # | 规则 | 检测方式 | 错误等级 |
|---|------|---------|---------|
| W1 | 低分辨率请检查高清版 | `resolution∈{8,4}` 且非官种 | 警告 |
| W2 | MI 含中字但未选中字标签 | `isTextChinese && !isTagTextChinese` | 警告 |
| W3 | 大于 1T 未选大包标签 | `isBiggerThan1T && !isTagBigTorrent` | 警告 |
| W4 | 简介含冗余图片 | 特定图片 URL 检测 | 警告 |
| W5 | 图片加载失败 | `height <= 24` | 警告 |
| W6 | 图片加载 30 秒超时 | 计时器 | 警告 |

### 分类级跳过校验

```
以下分类清空所有错误和警告：
- 413(Game) / 418(Picture) / 415(E-Book) / 412(Software) / 411(Music) / 408(Audio) / 406(MV)

音乐官种 (officialMusicSeed)：
- 清空所有错误，仅检查"是否选择了制作组"

Music(411) 在清空后额外检查：
- 主标题缺少采样频率(khz)
- 主标题缺少比特率(bit)
```

### 不信任制作组（22+关键词）

```
FGT, Hao4K, Mp4Ba, RARBG, GPTHD, SeeWeb, DreamHD, BlackTV, Xiaomi,
Huawei, MomoHD, DDHDTV, NukeHD, TagWeb, SonyHD, MiniHD, BitsTV,
ALT, BATWEB, DBD-Raws, XunLei, ZeroTV, LelveTV, Quark, mp4, mkv
```

> **注意**：Quark（夸克网盘转存组）、mp4、mkv 被列为不信任关键词，这是财神站的特殊规则。

### UI 功能

| 功能 | 说明 |
|------|------|
| 错误/警告双提示框 | 红色(#F44336)错误 + 黄色(#ffdd59)警告 |
| 提示框位置可配置 | 3种位置 |
| 一键通过按钮（API 模式） | 通过 GM_xmlhttpRequest 直接 POST API，无需打开弹窗 |
| CSRF Token 自动获取 | GET 审核页获取 `_token`，POST 到 `/web/torrent-approval` |
| 审核按钮放大 | 可切换 |
| 快捷键 F4 | 一键通过 |
| 快捷键 F3 | 关闭窗口 |
| 图片链接显示 | 种审模式在图片前添加可点击链接 |
| 自动关闭页面 | 可切换 |
| 清空备注按钮 | 审核弹窗中添加 |
| 移动端适配 | 自动调整按钮大小 |
| 脚本菜单 | 3个菜单命令 |
| edit.php 支持 | 编辑页面也可运行检查 |
| 图片加载检测 | 30秒超时 + 异常高度(≤24px)检测 |

> **关键差异**：财神使用**API 直接审核**（POST JSON），而非其他站的弹窗点击模式。审核接口为 `/web/torrent-approval`，需 CSRF `_token`。

## 转载发布自动填写优化方案

### 标题自动处理

```
1. 确保标题无中文/非ASCII字符
2. 移除源站后缀标签
3. WEB-DL 和 WEBRip 均选 WEB-DL(14)
4. AV1 关键词 → Encode(13) 媒介
5. 含 x264/x265 的标题 → Encode(13) 而非 Blu-ray(11)
6. 纯 bluray（无编码词/无UHD）→ Blu-ray(11)
7. UHD Blu-ray(10) 独立媒介
8. 检测禁转标记并选禁转标签
```

### 副标题自动处理

```
1. 禁止为空（必填）
2. 副标题含"动画" → 分类须选 Anime(405)
3. 副标题含"第X集" → 须选分集标签
4. 优先从 PT-Gen/豆瓣获取中文名
```

### 质量字段自动选择

```
从源站标题解析（注意所有 ID 与其他站完全不同）：
1. 媒介(type)：
   WEB-DL/WEBRip→14, Remux→12, AV1→Encode(13),
   Blu-ray(无编码词)→11, HDTV→15, Encode(含264/265)→13,
   UHD Blu-ray→10, DVD→16, CD→17, Track→18, Other→19
2. 编码(encode)：
   H.264/AVC→6, H.265/HEVC→7, VC-1→8, MPEG-2→9, AV1→10, Other→11
3. 音频(audio)：按匹配优先级
   FLAC→22, LPCM→14, DDP/E-AC3→12, AAC→9, DD/AC3→13,
   TrueHD Atmos→11, DTS-HD MA→17, DTS:X→16, DTS→18,
   ALAC→8, APE→10, TrueHD→15, M4A→19, WAV→20, MP3→21, Other→23
4. 分辨率(resolution)：
   480p/i→5, 720p/i→6, 1080p/i→7, 4K/2160p/UHD→8,
   8K/4320p→9, 2K/1440p→11, Other→10
5. 制作组(group)：
   WiKi→4, MySiLU→3, HDS→1, CHD→2, rain→6, rainweb→7,
   Tangweb→8, Other→5
```

### 标签自动选择

```
1. 官方：标题含 csweb/cspt → 勾选官方标签
2. 驻站：标题含 goddramas → 勾选驻站标签
3. 方舟：标题含 hares → 勾选方舟标签
4. 中字：MI Text Language 含 Chinese → 警告级建议勾选
5. 大包：种子体积 > 1TB → 警告级建议勾选
6. 分集：标题含 S**E** 或副标题含"第X集" → 勾选
7. 完结：标题含 complete + cat∈{402,403,404} → 勾选
8. 禁转：标题含"禁转" 或 GodDramas 含"禁止转载" → 勾选
9. 儿童：简介类别含儿童关键词 → 勾选（交叉验证）
10. 喜剧：简介类别含喜剧关键词 → 勾选（交叉验证）
```

### MediaInfo 处理

```
1. MediaInfo 须放入独立栏位（禁止在简介中包含）
2. MediaInfo 禁止含 BBCode 标签
3. 官种必须解析（短格式≠原始格式）
4. 从 MI 提取音频/字幕语言用于标签自动选择
5. 简介中禁止包含冗余图片
6. 禁止使用 rains3.com/img.m-team.cc/totheglory.im/i.miji.bid/duan.red 图床
```

### 简介要求

```
1. 简介必须包含"片名"或"译名"字段（影片简介必填）
2. 简介禁止包含 MediaInfo 内容
3. 至少 2 张图片（海报+截图）
4. IMDb/豆瓣/TMDb 链接检查已注释（不强制）
```

---

*数据来源: upload.php HTML (2026-04-22), Wiki uploadrule (2025-11-05 更新), Wiki howtoupload (2025-10-11 更新), CS-Torrent-Assistant-New v1.5.11 (1615行/73KB)*
*文档更新: 2026-04-22*
