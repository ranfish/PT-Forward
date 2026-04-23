# 百川 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 百川|
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

#### 分辨率（`standard_sel[4]`）— 8个

| 值 | 显示名称 |
|----|----------|
| 7 | 8K |
| 5 | 4K |
| 6 | 1440p |
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 8 | Other |

注意：含独特的 1440p(6) 和 Other(8) 选项。

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

### 2.1 各类型标题格式（Wiki 14 类）

| 类型 | 主标题格式 | 副标题格式 | 示例 |
|------|----------|-----------|------|
| 电影 | 英文片名/外文名 年代 分辨率 来源 编码 制作组 | 中文片名 | `A Good Rain Knows 2009 BluRay iPad 720p AAC x264-CHDPAD` |
| 电视剧 | 英文片名/外文名 年代 季度信息 分辨率 编码 制作组 | 中文名称 (补充信息) | `Breaking Bad S05E01 PROPER 720p HDTV x264-ORENJI` |
| 动漫 | 英文名/罗马音/日文名 年代 季度信息 编码 制作组 | 中文名称 (补充信息) | `Sword Art Online II 1080p BluRay FLAC x265 10bit - VCB-Studio` |
| 综艺 | 节目英文名.季数.期数.时间.分辨率.来源.编码.制作组 | 中文名 第几季第几期 | `DragonTV Go Fighting S02E12 20160703 1080i HDTV H264-NGB` |
| 演唱会 | 英文名 时间 分辨率 来源 编码 压制组 | 中文名 时间 | `Girls Generation 4th Tour Phantasia in Japan 2016 BluRay 1080p x264 DTSHD-NoVA` |
| 体育 | 英文名称 时间 参赛场次及队伍 分辨率 来源 编码 制作组 | 中文名 时间 场次 参赛队伍 | `EURO 2016 07 04 Quarter-finals France vs Iceland 720p HDTV 50fps x264-90oo` |
| 音乐 | 艺术家 - 官方专辑名称 发行年份 - 资源格式(分轨/整轨) - 发布源 | 艺术家 专辑名称 | `JayChou - Jay Chou's Bedtime Stories 2016 Flac-HerrWu@iMusic` |
| MV | 艺术家-名称 发行年份-分辨率_编码格式-资源格式-发布源 | 艺术家 MV名称 | |
| 游戏 | (发行公司)游戏英文名(DLC名称)(版本号)语言.类别.完整性.压制组 | 游戏中文名 特别说明 | `Far Cry 4 Valley of the Yetis Addon v1.9 CHS ENG Full Rip-GBT` |
| 纪录片 | 英文名称.(次名称.)日期.来源.分辨率.格式 | 名称 次名称 | |
| 教学视频 | 授课学校,(授课年份,)课程名称,(教学平台) | | |
| 电子文档 | 文档名称.出版日期(若有).语言.格式 | 名称 | |
| 软件资源 | (公司名)软件英文名(主版本号)架构 资源格式-破解组或来源 | 中文名 发行语言 特别说明 | `Adobe CC Family 20160210 Master Collection v5.9 ISO` |

注意：
- 国内电视剧/综艺若没有英文名，用拼音即可
- 音乐标题严禁使用别名、译名，必须与官方发行信息相同
- 标题禁止带有诱导性词语
- 国内动漫合集：字幕组、片源必须统一，禁止混合字幕组/格式/分辨率的合集

### 2.2 简介要求

电影/电视剧/纪录片/动漫：必须包含海报 + ◎片名/译名/年代/国家/类别/语言/字幕/上映日期/IMDb/豆瓣/片长/导演/主演/简介 + 视频参数 + 截图
演唱会：海报 + 演唱会简介 + 曲目列表
体育：海报 + 简介（禁止泄漏比赛结果）
音乐：专辑封面 + 专辑名称 + 艺人 + 发行年份 + 资源类型 + 曲目列表
MV：海报/截图 + MV 简介（不少于 30 字）
游戏：封面海报 + 基本信息 + 介绍 + 配置要求 + 安装方法
软件：软件介绍 + 截图 + 安装说明 + 更新内容

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

### 3.6 账号保留规则

| 条件 | 规则 |
|------|------|
| Veteran User 及以上 | 永远保留 |
| Elite User 及以上 | 封存账号后不会被删除 |
| 封存账号 | 连续 **500** 天不登录删除 |
| 未封存账号 | 连续 **300** 天不登录删除 |
| 无流量账号 | 连续 100 天不登录删除 |

注意：百川账号保留期比大多数站点更长（封存 500 天、未封存 300 天）。

### 3.7 评论区规则

- **评论区禁止求字幕**：一次警告一周，两次封号

### 3.8 认领规则

| 项目 | 规则 |
|------|------|
| 可认领时间 | 种子发布 30 天后 |
| 每种子认领上限 | 25 人 |
| 每用户认领上限 | 25 个 |
| 达标标准 | 每月做种 ≥ 300 小时，或上传量 ≥ 体积 2 倍 |
| 达标奖励 | 2 倍魔力值 |
| 不达标惩罚 | 自动移除 + 扣 50 魔力 |
| 首月特殊 | 认领首月不会因不达标被移除和扣魔力 |
| 手动取消 | 不扣魔力 |

### 3.9 捐赠规则

非营利站点，不接受主动索要赞助。自愿捐赠 N 元赠送：
- 上传量：N×10 GB
- 魔力值：N×1000
- 邀请码：N/20 取整
- VIP 时长：N/20 取整月（N>500 永久 VIP）
- N≥10 时：星星标记 + 双倍魔力值做种奖励

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
      "1080i": 2
      "720p": 3
      "SD": 4
      "2160p": 5
      "1440p": 6
      "8K": 7
      "Other": 8

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
*最后更新：2026-04-22*
*数据来源：https://wiki.hitpt.com/zh/classics/Specification + https://www.hitpt.com/rules.php + https://www.hitpt.com/upload.php*
