# 百川PT (HITPT) 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 百川PT (HITPT) |
| 站点地址 | https://www.hitpt.com |
| 站点框架 | NexusPHP |
| 主题 | BlueGene + 自定义 |
| 别名 | 百川PT |
| Wiki | https://wiki.hitpt.com/zh/classics/Specification |
| 特殊规则 | 宽松 dupe（高清/标清可共存，不同 iNT 组可共存），Cloudflare 防护 |

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（若不填使用种子文件名） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接 |
| `pt_gen` | text | - | PT-Gen 链接 |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode，20行） |
| `technical_info` | textarea | - | MediaInfo/BDInfo（8行） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

**双模式系统**：种子区（mode=4）和特别区（mode=7），互斥选择。

#### 类型（`type`）— 必填，双选择器互斥

**种子区** (mode=4)：

| 值 | 显示名称 |
|----|----------|
| 401 | 高清电影 |
| 402 | 高清剧集 |
| 403 | 抢鲜或标清 |
| 405 | 动漫 |
| 407 | 体育 |
| 413 | 纪录片 |
| 416 | 综艺 |
| 415 | Music Video |

**特别区** (mode=7)：

| 值 | 显示名称 |
|----|----------|
| 404 | 教学视频 |
| 406 | 音乐 |
| 408 | 工程软件 |
| 409 | 其他 |
| 410 | 游戏 |
| 411 | 电子文档 |
| 417 | 电子书 |
| 418 | 网络课程 |

注意：使用 `onchange="disableother('browsecat','specialcat')"` 实现互斥。种子区有"抢鲜或标清"(403)独特分类。特别区有"教学视频"(404)、"工程软件"(408)、"电子文档"(411)、"电子书"(417)、"网络课程"(418)等独特分类。

#### 来源（`source_sel[4]`）— 用作媒介，11个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 2 | BDrip |
| 3 | DVD |
| 4 | HDTV |
| 5 | TV |
| 7 | CD |
| 8 | Other |
| 9 | Web |
| 10 | 保种资源 |
| 11 | UHD |
| 12 | Remux |

注意：字段名为 `source_sel` 但实际用作媒介选择。有"保种资源"(10)独特选项。UHD(11)单独列出。

#### 视频编码（`codec_sel[4]`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 10 | H.265 |
| 11 | VP9 |
| 12 | MPEG-4 |
| 13 | X264 |
| 14 | X265 |

注意：**区分原盘/压制编码**——H.264(1) vs X264(13)、H.265(10) vs X265(14)。与青蛙、HDFans 类似。包含 VP9(11)。

#### 音频编码（`audiocodec_sel[4]`）— 17个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | AC3 |
| 11 | WAV |
| 12 | TrueHD |
| 13 | Atmos |
| 14 | LPCM |
| 15 | DTS-X |
| 16 | DTS-HD |
| 17 | DTS-HDMR |
| 18 | DTS-HDMA |
| 19 | DTS-HDMA:X 7.1 |

注意：DTS 家族极为细分——DTS(3)、DTS-HD(16)、DTS-HDMR(17)、DTS-HDMA(18)、DTS-X(15)、DTS-HDMA:X 7.1(19)共6个级别。含 Atmos(13)、LPCM(14)、TrueHD(12)。

#### 分辨率（`standard_sel[4]`）— 含 1440p

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 5 | 4K |
| 6 | 1440p |
| 7 | 8K |

注意：被截断，完整列表应包含 1080i、720p、SD 等。含独特的 1440p(6) 分辨率选项。

#### 制作组（`team_sel[4]`）— 14个

| 值 | 显示名称 |
|----|----------|
| 3 | HIT内部资料 |
| 4 | 其他 |
| 5 | CMCT |
| 6 | HDWinG |
| 8 | CHDBits |
| 13 | WiKi |
| 14 | beAst |
| 16 | MTeam |
| 17 | 百川自制 |
| 18 | HDDolby |
| 19 | OurBits |
| 20 | FRDS |
| 21 | HSPT |
| 22 | PTer |

注意：制作组数量较多，包含百川自制(17)、HIT内部资料(3)，以及 MTeam(16)、OurBits(19)、PTer(22)、FRDS(20) 等知名外站组。

#### 标签（`tags[4][]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 粤语 |
| 9 | 杜比 |

注意：种子区(mode=4)和特别区(mode=7)标签完全相同。

### 1.3 缺失字段

- `processing_sel` — 无地区选择

---

## 二、标题命名规范

来源：`rules.php` + Wiki (https://wiki.hitpt.com/zh/classics/Specification)

发布细则指引至 Wiki 页面，规则页面未包含详细的标题格式要求。标题禁止带有诱导性词语。

---

## 三、发布规则

### 3.1 允许的资源

**种子区**：
- 高清/标清影视资源
- 学校内活动的影像录像或相关资料宣传片
- 质量上乘的枪版在一周内发布（管理员可随时删除）

**特别区**：
- 软件安装程序、开发环境
- 中文硬盘压制版、光盘镜像版游戏
- Steam 预载文件及 Origin 完整文件（副标题需注明）
- 无损音乐、m4a 分轨音乐
- 电子书
- 教学视频

### 3.2 禁止的资源

- 总体积 < 100MB（电子书/小型软件除外）
- 标清 upscale 视频
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD（质量上乘枪版一周内例外）
- RealVideo/RMVB/RM/FLV
- 单独样片
- 重复资源
- 涉及禁忌或敏感内容
- 损坏文件
- 老师明确不允许分享的 PPT/资料
- 需要特殊播放器的格式（kux、qsv）
- 非特别允许的压缩文件（zip、rar）
- 垃圾文件
- 水印严重的视频

### 3.3 Dupe 规则（宽松）

百川PT 的 dupe 规则比标准 HD 站更宽松：

- **高清与标清不构成 dupe**（720p 和 1080p 可共存）
- **不同 iNT 小组的资源可同时共存**
- 同一影视作品，片源/音轨一样且码率相近 → 构成重复，只保留先发版本
- 片源优先级：Blu-ray = HD DVD > DTheater > HDTV > DVD > PDTV > TV
- 不同区域/配音/字幕的原盘不视为重复
- 无损音轨只保留一个版本（分轨 FLAC 优先级最高）
- 游戏：光盘镜像版和中文硬盘版可共存，其余视为多余
- 游戏/软件：正式版发布后 Beta 版视为多余

### 3.4 促销规则

**新种促销**：上传7日内免费

**随机促销**（上传后自动触发，30天后降级）：
- 10% → 50%下载，30天后 → 2x上传
- 5% → 免费，30天后 → 30%下载
- 5% → 2x上传，30天后 → 50%下载
- 3% → 免费&2x上传，30天后 → 30%下载
- 4% → 50%下载&2x上传，30天后 → 50%下载
- 2% → 30%下载，30天后 → 50%下载

**固定促销**：
- 总体积 > 50GB → 自动免费
- Blu-ray 原盘 → 免费（提醒管理员手动设定）
- 电视剧每季第一集 → 免费（提醒管理员手动设定）
- 未参与促销的种子180天后 → 永久2x上传

### 3.5 游戏发布限制

游戏类资源只有**发布员**及以上等级用户可自由上传，其他用户需在候选区提交候选。

---

## 四、站点适配器配置参考

```yaml
site:
  id: "hitpt"
  name: "百川PT"
  alt_name: "HITPT"
  url: "https://www.hitpt.com"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"
  wiki_url: "https://wiki.hitpt.com/zh/classics/Specification"

  dual_mode:
    primary:
      name: "种子区"
      mode: 4
      field_suffix: "[4]"
    secondary:
      name: "特别区"
      mode: 7
      field_suffix: "[7]"
      types: [404, 406, 408, 409, 410, 411, 417, 418]
    mutual_exclusive: true

  mappings:
    type_seeds:
      "电影": 401
      "剧集": 402
      "标清": 403
      "动漫": 405
      "体育": 407
      "纪录": 413
      "综艺": 416
      "MV": 415

    type_special:
      "教学视频": 404
      "音乐": 406
      "工程软件": 408
      "其他": 409
      "游戏": 410
      "电子文档": 411
      "电子书": 417
      "网络课程": 418

    source_sel:
      "Blu-ray": 1
      "BDrip": 2
      "DVD": 3
      "HDTV": 4
      "TV": 5
      "CD": 7
      "Other": 8
      "WEB-DL": 9
      "保种资源": 10
      "UHD": 11
      "Remux": 12

    codec_sel:
      "H264": 1
      "VC-1": 2
      "Xvid": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 10
      "VP9": 11
      "MPEG-4": 12
      "x264": 13
      "x265": 14

    audiocodec_sel:
      "FLAC": 1
      "APE": 2
      "DTS": 3
      "MP3": 4
      "OGG": 5
      "AAC": 6
      "Other": 7
      "AC3": 8
      "WAV": 11
      "TrueHD": 12
      "Atmos": 13
      "LPCM": 14
      "DTS:X": 15
      "DTS-HD": 16
      "DTS-HDMR": 17
      "DTS-HDMA": 18
      "DTS-HDMA:X": 19

    standard_sel:
      "1080p": 1
      "2160p": 5
      "1440p": 6
      "8K": 7

    team_sel:
      "HIT内部": 3
      "Other": 4
      "CMCT": 5
      "HDWinG": 6
      "CHDBits": 8
      "WiKi": 13
      "beAst": 14
      "MTeam": 16
      "百川自制": 17
      "HDDolby": 18
      "OurBits": 19
      "FRDS": 20
      "HSPT": 21
      "PTer": 22

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7
      "粤语": 8
      "杜比": 9

  field_names:
    suffix: "[4]"
    source: "source_sel[4]"
    codec: "codec_sel[4]"
    audiocodec: "audiocodec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    technical_info: "technical_info"
    pt_gen: "pt_gen"
    anonymous: "uplver"

  missing_fields:
    - "processing_sel"

  quirks:
    dual_mode: "种子区(mode=4)和特别区(mode=7)互斥选择"
    source_as_medium: "source_sel用作媒介，含保种资源(10)和UHD(11)"
    codec_split: "区分原盘H.264/H.265和压制x264/x265"
    dts_family: "DTS家族6个级别细分"
    relaxed_dupe: "高清/标清可共存，不同iNT组可共存"
    cloudflare: "使用Cloudflare防护"
    wiki_rules: "详细发布规范在Wiki"
    1440p: "分辨率含1440p选项"
```

---

## 五、发布流水线注意事项

### 5.1 双模式处理

百川PT 有种子区和特别区两个互斥选择器。转种时需先判断资源类型选择正确的模式：
- 视频/动漫/体育/MV → 种子区 (mode=4)
- 音乐/游戏/软件/电子书/教学 → 特别区 (mode=7)
- 两个 `type` 选择器使用相同的 `name`，但不同 `id`（`browsecat` vs `specialcat`），需确保只提交一个。

### 5.2 编码区分原盘/压制

H.264(1) vs X264(13)、H.265(10) vs X265(14)：
- 原盘/Remux → H.264(1) 或 H.265(10)
- 压制/Encode → X264(13) 或 X265(14)

### 5.3 制作组映射

14个制作组，是已分析站点中较多的。包含外站组 MTeam(16)、OurBits(19)、PTer(22)、FRDS(20) 等。

### 5.4 Dupe 规则宽松

百川PT 的 dupe 比大部分站点宽松，适合作为发布站：
- 高清/标清可共存
- 不同 iNT 组可共存
- 仅在片源/音轨/码率都相近时才构成重复

---

*分析时间：2026-04-16*
*数据来源：https://www.hitpt.com/upload.php + rules.php*
