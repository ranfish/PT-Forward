# 龙PT 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 龙PT |
| 域名 | longpt.org |
| 框架 | NexusPHP |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（部分用户需先候选） |
| 种审制 | 否（未提及） |
| 站点角色 | 源站 + 发布站 |
| MediaInfo | 是（technical_info 字段） |
| IMDb | 是（url 字段，带 PT-Gen "获取简介"按钮） |
| 豆瓣 | 否（无 pt_gen 字段） |
| PT-Gen | 否（无独立 pt_gen 字段，仅 IMDb url 的"获取简介"按钮） |
| 匿名发布 | 否（无 uplver 字段） |
| NFO | 是（nfo 文件上传） |
| 字幕区 | 是（独立字幕上传系统） |
| 签到系统 | 是（签到已得 1000 魔力值） |
| 建站时间 | 2024年 |

## Tracker URL
`https://longpt.org/announce.php`

## 站点特色

- **规则与织梦/自然/我爱电影完全同模板**（模板B组）
- **UHD Blu-ray 独立媒介**：Blu-ray 和 UHD Blu-ray 分开，Remux 也有 UHD 版本
- **编码含 AV1**：支持 H.264/AVC、H.265/HEVC、VC-1、MPEG-2、AV1、Other
- **音频含 DDP/AV3A/DTS:X/Atmos/ALAC/WAV/OGG/M4A**：18 个音频编码选项
- **分辨率含 8K/2K**：7 个分辨率级别，含 8K 和 2K/1440p
- **无匿名发布、无 PT-Gen 豆瓣**：只有 IMDb url 字段
- **官组体系**：LongA/LongWeb/LongPT/WiKi/RL/CMCT/HHWEB
- **发布者双倍上传**
- **9KG/成人内容禁止**：本软件禁止下载和发布此类内容

## 发布权限

| 用户等级 | 发布权限 |
|---------|---------|
| 任何注册用户 | 可发布资源（部分需先候选） |
| 上传员及以上 | 游戏类可自由上传 |

## 分类映射 (type)

| ID | 名称 | 备注 |
|----|------|------|
| 401 | 电影 | |
| 402 | 剧集 | |
| 403 | 综艺 | |
| 404 | 纪录片 | |
| 405 | 动画 | |
| 406 | 音乐视频 | |
| 407 | 体育 | |
| 408 | 音频 | |
| 409 | 其他 | |
| 410 | 有声书 | |
| 411 | 其他 | 第二个"其他" |

> **注意**：分类含 408(音频) 和 410(有声书)，无标准游戏分类。

## 媒介映射 (medium_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | Blu-ray | 蓝光原盘 |
| 2 | UHD Blu-ray | **UHD 蓝光原盘独立** |
| 3 | Blu-ray Remux | |
| 11 | UHD Blu-ray Remux | **UHD Remux 独立** |
| 4 | WEB-DL | |
| 5 | HDTV | |
| 6 | DVD | |
| 7 | Encode | 重编码 |
| 8 | CD | 音乐CD |
| 9 | Track | 单曲 |
| 10 | Other | |

> **注意**：UHD Blu-ray(2) 和 UHD Blu-ray Remux(11) 独立于标准 Blu-ray(1)，这在 NexusPHP 站点中较少见。

## 视频编码映射 (codec_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | H.264/AVC | |
| 2 | H.265/HEVC | |
| 3 | VC-1 | |
| 4 | MPEG-2 | |
| 5 | AV1 | |
| 6 | Other | |

## 音频编码映射 (audiocodec_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | FLAC | |
| 2 | APE | |
| 3 | DTS-HD MA | |
| 4 | MP3 | |
| 5 | OGG | |
| 6 | AAC | |
| 8 | M4A | |
| 9 | TrueHD Atmos | Atmos 独立 |
| 10 | DDP | Dolby Digital Plus |
| 11 | Other | |
| 12 | DTS:X | |
| 13 | DTS | |
| 14 | LPCM | |
| 15 | AC3 | |
| 16 | ALAC | |
| 17 | WAV | |
| 18 | AV3A | Audio Vivid 3D Audio |

> **注意**：18 个音频编码选项，含 AV3A（Audio Vivid 3D Audio）、TrueHD Atmos、DDP、DTS:X、ALAC、WAV。DTS-HD MA 独立于 DTS(13)。

## 分辨率映射 (standard_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | 2K/1440p/1440i | |
| 2 | 1080p/1080i | |
| 3 | 720p/720i | |
| 4 | 480p/480i | |
| 5 | 4K/2160p/2160i | |
| 6 | 8K/4320p/4320i | |
| 7 | Other | |

> **注意**：分辨率 ID 排序非递增也非递减（2K=1, 1080p=2, 720p=3, 480p=4, 4K=5, 8K=6）。

## 制作组映射 (team_sel[4])

| ID | 名称 | 备注 |
|----|------|------|
| 1 | LongA | 龙PT官组 |
| 2 | LongWeb | 龙PT WEB官组 |
| 3 | LongPT | 龙PT官组 |
| 4 | WiKi | 知名压制组 |
| 5 | Other | |
| 6 | RL | |
| 7 | CMCT | |
| 8 | HHWEB | |

## 标签系统 (tags[4][])

通过 checkbox 多选（16 个）：

| ID | 名称 | 备注 |
|----|------|------|
| 1 | 禁转 | |
| 2 | 首发 | |
| 4 | DIY | |
| 5 | 国语 | |
| 6 | 中字 | |
| 7 | HDR | |
| 8 | 完结 | |
| 9 | 英字 | |
| 10 | 杜比 | Dolby Vision |
| 11 | 特效 | 特效字幕 |
| 12 | 分集 | |
| 13 | 高分 | 高分资源 |
| 14 | 臻彩MAX | |
| 15 | 高码 | 高码率 |
| 16 | 高帧 | 高帧率 |
| 17 | 去广告纯享版 | |

> **注意**：标签含"英字"(9)、"臻彩MAX"(14)、"去广告纯享版"(17) 等非常规标签，无标准 HDR/SDR 分离。

## 标题命名规范

### 电影
```
[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```

### 电视剧
```
[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称
```

### 音轨
```
[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]
```

### 游戏
```
[中文名] 名称 [年份] [版本] [发布说明][-发布组名称]
```

## Dupe判定规则

与织梦/自然/我爱电影站**完全一致**（模板B组）。

## 打包规则

与织梦/自然/我爱电影站**完全一致**（模板B组）。

## 表单字段

### 提交URL
`POST https://longpt.org/takeupload.php`

### 必填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| file | file | 种子文件 |
| type | select | 分类(401-411) |
| descr | textarea(BBCode) | 简介 |

### 选填字段

| 字段 | 类型 | 说明 |
|------|------|------|
| name | text | 标题（不填用种子文件名） |
| small_descr | text | 副标题 |
| url | text | IMDb 链接（带"获取简介"按钮） |
| nfo | file | NFO 文件 |
| technical_info | textarea | MediaInfo |
| medium_sel[4] | select | 媒介 |
| codec_sel[4] | select | 视频编码 |
| audiocodec_sel[4] | select | 音频编码 |
| standard_sel[4] | select | 分辨率 |
| team_sel[4] | select | 制作组 |
| tags[4][] | checkbox[] | 标签（多选） |

> **注意**：无 pt_gen（豆瓣）字段、无 uplver（匿名发布）字段、无 source_sel（地区）字段、无 processing_sel 字段。

### 质量字段联动
质量行通过 `data-mode='4'` 和 `relation="mode_4"` 控制显隐。

## 注意事项

1. **规则与织梦/自然/我爱电影完全同模板**（模板B组）
2. **UHD Blu-ray 独立**：媒介区分 Blu-ray(1) 和 UHD Blu-ray(2)，Remux 也有 UHD 版本(11)
3. **无匿名发布、无 PT-Gen**：相比其他同模板站点少了这两个功能
4. **无 source_sel/processing_sel**：无地区和处理方式字段
5. **音频编码丰富**：18 个选项含 AV3A/DDP/DTS:X/Atmos/ALAC/WAV/OGG/M4A
6. **分辨率 ID 非标准排序**：2K=1, 1080p=2, 720p=3, 480p=4, 4K=5, 8K=6

---

*文档创建：2026-04-19*
*数据来源：rules.php (45787字节) + upload.php (49257字节)*
