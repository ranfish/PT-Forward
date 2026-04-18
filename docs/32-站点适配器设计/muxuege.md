# 慕雪阁 站点适配器设计

> 慕雪阁站点特异化适配器设计文档

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 慕雪阁|
| 站点地址 | https://pt.muxuege.org |
| 站点框架 | NexusPHP |
| 特殊规则 | 官种命名自定义格式、编码含 HDR 10/TXT/PDF、分辨率含 540p、31 标签（含珍宝楼/乐府）、47 制作组、候选制、PT-Gen 仅 IMDb |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://pt.muxuege.org/announce.php` |

---

## 一、发布页面表单字段分析

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件（id="torrent"） |
| `name` | text | - | 标题（规范填写，如 `Blade Runner 1982 Final Cut 720p HDDVD DTS x264-ESiR`） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（带 `data-pt-gen="url"` 属性） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode） |

### 1.2 缺失字段

以下常见字段在慕雪阁 **不存在**：

- **`source_sel`**：无来源字段
- **`processing_sel`**：无处理方式字段
- **`audiocodec_sel`**：无音频编码字段
- **`uplver`**：无匿名发布选项
- **`pt_gen`**：无独立 PT-Gen 字段（仅 IMDb 输入框带 `data-pt-gen` 属性）
- **`douban_id`**：无豆瓣字段
- **`technical_info`**：无 MediaInfo 独立字段

### 1.3 类型（`type`）— 18 个分类

`<select name="type" id="browsecat" data-mode='4'>`

| 值 | 显示名称 |
|----|----------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | Music Videos |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | 其他 |
| 410 | 系统镜像 |
| 411 | 游戏 |
| 412 | 电子书 |
| 413 | 教育 |
| 414 | 图片 |
| 415 | 软件 |
| 416 | 短剧 |
| 417 | 有声书 |
| 418 | 广播剧 |

**特点**：
- 分类编号 401-418 连续递增（无 419+）
- 有 **广播剧**（418）和 **有声书**（417），音频/文学类分类丰富
- 有 **短剧**（416）、**系统镜像**（410）
- 仅 `data-mode='4'`，单模式发布

### 1.4 媒介（`medium_sel[4]`）— 10 个

`<select name="medium_sel[4]" data-mode="medium_4">`

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
| 10 | web |

**特点**：
- Encode 值=7（非标准，通常 Encode 值较大）
- web 值=10（小写 w）
- 有 MiniBD（值=4）
- 无 UHD Blu-ray 独立选项

### 1.5 编码（`codec_sel[4]`）— 11 个

`<select name="codec_sel[4]" data-mode="codec_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 & AVC |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265 & HEVC |
| 7 | AV1 |
| 8 | VP9 |
| 9 | HDR 10 |
| 10 | TXT |
| 11 | PDF |

**特点**：
- **HDR 10 被列为编码选项**（值=9），而非标签或媒介属性——这在 NexusPHP 站点中较罕见
- **TXT**（值=10）和 **PDF**（值=11）作为编码选项出现，用于电子书/有声书等文本类资源
- 视频编码与文本编码混在同一字段，适配器需根据分类选择合适的编码值
- H.264 和 AVC 合并为一个选项（`H.264 & AVC`）
- H.265 和 HEVC 合并为一个选项（`H.265 & HEVC`）

### 1.6 分辨率（`standard_sel[4]`）— 5 个

`<select name="standard_sel[4]" data-mode="standard_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p |
| 6 | 540p |

**特点**：
- 有 **540p**（值=6），较少见，用于低质量或移动端资源
- 无 1080i 独立选项
- 值编号不连续（1→3→4→5→6，缺 2）
- 1080p=1, 2160p=5（非标准映射）

### 1.7 制作组（`team_sel[4]`）— 47 个

`<select name="team_sel[4]" data-mode="team_4">`

| 值 | 显示名称 |
|----|----------|
| 1 | HDSky 高清天空 |
| 2 | CHDbits |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | StarfallWeb星陨阁 |
| 7 | MxWeb慕雪阁 |
| 8 | MTeam |
| 9 | 3Mweb 三月传媒 |
| 10 | AiMWeb 熊猫 |
| 11 | Tmp |
| 12 | TPTV |
| 13 | Mweb |
| 14 | UBWEB |
| 15 | Ubits |
| 16 | QHstudIo |
| 17 | DBTV |
| 18 | Qurbits |
| 19 | NatureWeb |
| 20 | CHD |
| 21 | BeyondHD |
| 22 | HDHome |
| 23 | BLU |
| 24 | DISC |
| 25 | CiNEFiLE |
| 26 | FraMeSToR |
| 27 | FLUX |
| 28 | EPSiLON |
| 29 | BLUTONIUM |
| 30 | TAoE |
| 31 | DECADE |
| 32 | TEPES |
| 33 | SWTYBLZ |
| 34 | NTb |
| 35 | PLAYERS |
| 36 | AMA |
| 37 | CM |
| 38 | BTN |
| 39 | BBC |
| 40 | MvGroup |
| 41 | zmWeb |
| 42 | AGSVWEB |
| 43 | TLF HALFCD TeaM |
| 44 | ADweb |
| 45 | Audies |
| 46 | TPAudio躺平 |
| 47 | NovaHD |

**特点**：
- **47 个制作组**，是目前分析站点中制作组数量最多的之一
- 含多个 Web 组：StarfallWeb、MxWeb、zmWeb、AGSVWEB、ADweb、UBWEB、AiMWeb、Mweb、3Mweb
- 含国际知名组：FraMeSToR、FLUX、EPSiLON、BLUTONIUM、NTb、BTN、BBC、PLAYERS
- 含音频组：Audies、TPAudio躺平
- 含老牌中文组：HDSky、CHDbits、MySiLU、WiKi、CHD
- 站方组：**MxWeb慕雪阁**（值=7）、**StarfallWeb星陨阁**（值=6）

### 1.8 标签（`tags[4][]`）— 31 个

`<input type="checkbox" name="tags[4][]" value="XX" />`

| 值 | 显示名称 |
|----|----------|
| 8 | 完结国漫 |
| 10 | 国风音乐 |
| 11 | 完结日漫 |
| 12 | 分集 |
| 25 | 杜比视界 |
| 26 | 完结 |
| 27 | 原盘 |
| 30 | 乐府 |
| 32 | 大包 |
| 33 | 珍宝楼 |
| 34 | 杜比视频 |
| 35 | 杜比全景声 |
| 36 | 压制 |
| 37 | DIY |
| 38 | 国语 |
| 40 | 生肉 |
| 41 | 高码率 |
| 42 | 特效字慕 |
| 45 | 自购 |
| 46 | 菜单修改 |
| 47 | 古装 |
| 48 | HDR 真彩 |
| 49 | 纯净版 |
| 50 | 1080p |
| 51 | 4k |
| 52 | H.264 |
| 53 | H.265 |
| 54 | Mac |
| 55 | Windows |
| 56 | linux |
| 57 | NovaHD |

**特点**：
- **31 个标签**，数量中等偏多
- 含分类标签：**珍宝楼**（33）、**乐府**（30）——站内特色版块标签
- 含杜比系列：**杜比视界**（25）、**杜比视频**（34）、**杜比全景声**（35）
- 含分辨率标签：**4k**（51）、**1080p**（50）——与分辨率下拉重复
- 含编码标签：**H.264**（52）、**H.265**（53）——与编码下拉重复
- 含平台标签：**Mac**（54）、**Windows**（55）、**linux**（56）
- 含动漫标签：**完结国漫**（8）、**完结日漫**（11）、**分集**（12）
- 含质量标签：**高码率**（41）、**纯净版**（49）、**原盘**（27）、**压制**（36）
- 含制作组标签：**NovaHD**（57）

---

## 二、关键适配器设计要点

### 2.1 编码字段混合用途

编码（`codec_sel`）字段同时包含视频编码（H.264/H.265/AV1/VP9/VC-1/MPEG-2/Xvid）和文本编码（TXT/PDF），以及 HDR 10。适配器需根据分类类型选择合适的值：
- 影视类：选择视频编码
- 电子书/有声书/广播剧：选择 TXT 或 PDF
- HDR 内容：可额外选择 HDR 10（值=9）

### 2.2 540p 分辨率

分辨率含 540p（值=6），适配器需处理源站无此分辨率时的映射策略。

### 2.3 无音频编码字段

慕雪阁没有独立的 `audiocodec_sel` 字段。音频信息只能通过标题或简介传递。与其他有音频编码下拉的站点（如 HDFans 24 种、YHPP 23 种）形成鲜明对比。

### 2.4 47 个制作组

制作组数量较多，适配器需建立完整的映射表。特别注意站方组 MxWeb（7）和 StarfallWeb（6）。

### 2.5 标签与下拉字段重复

分辨率（4k/1080p）和编码（H.264/H.265）同时出现在标签和下拉框中，发布时需同步填写。

### 2.6 官种命名格式

官种命名格式非 Scene 标准：
```
中文名,介绍,英文名 季度集数 年份 分辨率 来源 编码 音频-组名
例：永生.无尽仙途.再遇师姐.Yong.Sheng.S01E12.2022.1080p.WEB-DL.H264.AAC-2.0-likesnowweb
```

### 2.7 PT-Gen 集成

IMDb 输入框带 `data-pt-gen="url"` 属性，表明站点集成了 PT-Gen 自动填充功能，但仅限 IMDb 链接。

### 2.8 候选制

站点有候选系统（offers.php），游戏类资源需先提交候选。

### 2.9 dupe 判定规则

视频资源按来源媒介优先级：`Blu-ray/HD DVD > HDTV > DVD > TV`。同一视频高优先级版本使低优先级被判定为 dupe。旧版本连续断种 45 日或发布 18 个月以上，发布新版本不受 dupe 约束。

---

## 三、发布字段与通用模型的映射

### 3.1 类型映射（type）

| 通用类型 | 慕雪阁 type 值 |
|---------|---------------|
| 电影 | 401 |
| 电视剧 | 402 |
| 综艺 | 403 |
| 纪录片 | 404 |
| 动漫 | 405 |
| MV | 406 |
| 体育 | 407 |
| 音乐 | 408 |
| 其他 | 409 |
| 系统镜像 | 410 |
| 游戏 | 411 |
| 电子书 | 412 |
| 教育 | 413 |
| 图片 | 414 |
| 软件 | 415 |
| 短剧 | 416 |
| 有声书 | 417 |
| 广播剧 | 418 |

### 3.2 媒介映射（medium_sel）

| 通用媒介 | 慕雪阁 medium_sel 值 |
|---------|---------------------|
| Blu-ray | 1 |
| HD DVD | 2 |
| Remux | 3 |
| MiniBD | 4 |
| HDTV | 5 |
| DVDR | 6 |
| Encode | 7 |
| CD | 8 |
| Track | 9 |
| Web | 10 |

### 3.3 编码映射（codec_sel）

| 通用编码 | 慕雪阁 codec_sel 值 |
|---------|---------------------|
| H.264/AVC | 1 |
| VC-1 | 2 |
| Xvid | 3 |
| MPEG-2 | 4 |
| Other | 5 |
| H.265/HEVC | 6 |
| AV1 | 7 |
| VP9 | 8 |
| HDR 10 | 9 |
| TXT | 10 |
| PDF | 11 |

### 3.4 分辨率映射（standard_sel）

| 通用分辨率 | 慕雪阁 standard_sel 值 |
|-----------|----------------------|
| 1080p | 1 |
| 720p | 3 |
| SD | 4 |
| 2160p | 5 |
| 540p | 6 |

### 3.5 音频编码映射（audiocodec_sel）

**慕雪阁无音频编码字段。**

### 3.6 制作组映射（team_sel）

| 值 | 显示名称 |
|----|----------|
| 1 | HDSky 高清天空 |
| 2 | CHDbits |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | StarfallWeb星陨阁 |
| 7 | MxWeb慕雪阁 |
| 8 | MTeam |
| 9 | 3Mweb 三月传媒 |
| 10 | AiMWeb 熊猫 |
| 11 | Tmp |
| 12 | TPTV |
| 13 | Mweb |
| 14 | UBWEB |
| 15 | Ubits |
| 16 | QHstudIo |
| 17 | DBTV |
| 18 | Qurbits |
| 19 | NatureWeb |
| 20 | CHD |
| 21 | BeyondHD |
| 22 | HDHome |
| 23 | BLU |
| 24 | DISC |
| 25 | CiNEFiLE |
| 26 | FraMeSToR |
| 27 | FLUX |
| 28 | EPSiLON |
| 29 | BLUTONIUM |
| 30 | TAoE |
| 31 | DECADE |
| 32 | TEPES |
| 33 | SWTYBLZ |
| 34 | NTb |
| 35 | PLAYERS |
| 36 | AMA |
| 37 | CM |
| 38 | BTN |
| 39 | BBC |
| 40 | MvGroup |
| 41 | zmWeb |
| 42 | AGSVWEB |
| 43 | TLF HALFCD TeaM |
| 44 | ADweb |
| 45 | Audies |
| 46 | TPAudio躺平 |
| 47 | NovaHD |

### 3.7 标签映射（tags）

| 值 | 显示名称 |
|----|----------|
| 8 | 完结国漫 |
| 10 | 国风音乐 |
| 11 | 完结日漫 |
| 12 | 分集 |
| 25 | 杜比视界 |
| 26 | 完结 |
| 27 | 原盘 |
| 30 | 乐府 |
| 32 | 大包 |
| 33 | 珍宝楼 |
| 34 | 杜比视频 |
| 35 | 杜比全景声 |
| 36 | 压制 |
| 37 | DIY |
| 38 | 国语 |
| 40 | 生肉 |
| 41 | 高码率 |
| 42 | 特效字慕 |
| 45 | 自购 |
| 46 | 菜单修改 |
| 47 | 古装 |
| 48 | HDR 真彩 |
| 49 | 纯净版 |
| 50 | 1080p |
| 51 | 4k |
| 52 | H.264 |
| 53 | H.265 |
| 54 | Mac |
| 55 | Windows |
| 56 | linux |
| 57 | NovaHD |

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-17*
