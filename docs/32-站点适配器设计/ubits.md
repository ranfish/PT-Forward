# 优堡 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 优堡 |
| 域名 | ubits.club |
| 框架 | NexusPHP |
| Cloudflare | 是 |
| 候选制 | 是（asoffer字段可选提交候选） |
| 站点角色 | **仅源站**（官组非常活跃，不适合做发布站） |
| MediaInfo/BDInfo | 是（technical_info） |
| IMDb | 是（url） |
| 豆瓣 | 是（pt_gen，PT-Gen获取简介） |
| 匿名发布 | 是（uplver） |
| NFO | 否 |
| 种审制 | 是（发布后需审核） |
| 建站时间 | 2023年 |

## Tracker URL
`https://t.ubits.club/announce.php`

## 站点特色

- **官组非常活跃**：站组UBits（原盘DIY组）、UBWEB（流媒体组）、UBTV（电视录制组），官种量大
- **仅源站角色**：官组活跃+种审制，不适合做发布站
- **种审制**：发布后需审核通过才能正式显示
- **编码含AVS**：codec_sel包含国产AVS编码
- **音频编码细分**：Atmos/DTS:X/TrueHD/DTS-HD MA/HR/DD+/DD独立选项，共14种
- **分辨率含1440p**：standard_sel包含1440p
- **地区10个**：含中国大陆/港/台/欧美/日/韩/泰/印/俄/其它
- **标签含原生原盘**：有"原生原盘"标签区分原盘和DIY
- **标签含高分国剧**：有"高分国剧"标签
- **标签含菜单修改**：有"菜单修改"标签
- **双语规则**：论坛发种规则中英双语
- **转载需标来源**：必须用[quote]标签标注来源站
- **禁止删除原站标识**：转载时保留原站标识（如DIY@UBits），删除者删种+警告+取消发布权限
- **发布者双倍上传量**

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 否 | 主标题不含中文，保留原发布站标识 |
| 副标题 | `small_descr` | 否 | 中文名称+附加信息 |
| IMDb链接 | `url` | 否 | data-pt-gen="url" |
| 豆瓣/PT-Gen | `pt_gen` | 否 | data-pt-gen="pt_gen"，有获取简介按钮 |
| 简介 | `descr` | 是 | BBCode，转载需用[quote]标注来源 |
| MediaInfo/BDInfo | `technical_info` | 否 | MediaInfo和BDInfo都支持 |
| 类型 | `type` | 是 | data-mode=4 |
| 媒介 | `medium_sel[4]` | 否 | |
| 视频编码 | `codec_sel[4]` | 否 | |
| 音频编码 | `audiocodec_sel[4]` | 否 | |
| 分辨率 | `standard_sel[4]` | 否 | |
| 地区 | `source_sel[4]` | 否 | |
| 制作组 | `team_sel[4]` | 否 | |
| 标签 | `tags[4][]` | 否 | checkbox多选，17个 |
| 提交候选 | `asoffer` | 否 | 勾选则由管理员审核 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | 电影(Movie) |
| 404 | 纪录片(Documentaries) |
| 405 | 动漫(Animations) |
| 402 | 电视剧(TV Series) |
| 403 | 综艺（TV Shows） |
| 409 | 演唱会(Misic Videos) |
| 407 | 体育节目(Sports) |
| 406 | 音乐CD(Music CD Tracker) |
| 408 | Other |

> 无游戏/软件/电子书/短剧分类

## 质量字段

### 媒介 medium_sel[4]

| ID | 名称 |
|----|------|
| 10 | 4K UHD原盘(UltraHD Blu-ray) |
| 1 | 蓝光原盘(Blu-ray) |
| 4 | 流媒体(WEB-DL) |
| 3 | REMUX |
| 7 | 压制(Encode) |
| 2 | HD DVD |
| 5 | HDTV |
| 6 | DVDR |
| 8 | Lossless Music |
| 9 | Track |

> 无MiniBD/Blu-ray DIY/UHDTV

### 视频编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 7 | H265(HEVC/x265) |
| 1 | H264(AVC/x264) |
| 11 | AV1 |
| 2 | VC-1 |
| 4 | MPEG-2 |
| 10 | AVS |
| 3 | Xvid |
| 9 | MPEG-4 |
| 5 | Other |

> 含AVS（国产编码），含MPEG-4，无VP9

### 音频编码 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 8 | Dolby Atmos |
| 9 | DTS:X |
| 10 | TrueHD |
| 11 | DTS-HD MA/HR |
| 13 | LPCM |
| 12 | DD+(Dolby Digital Plus) |
| 14 | DD(AC3) |
| 3 | DTS |
| 6 | AAC |
| 1 | FLAC |
| 2 | APE |
| 5 | OGG |
| 4 | MP3 |
| 7 | Other |

> 14种音频编码，Atmos/DTS:X/TrueHD/DTS-HD MA/HR/DD+全部独立

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 6 | 4320p |
| 5 | 2160p |
| 7 | 1440p |
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |

> 含1440p（2K），1080p/i分开

### 地区 source_sel[4]

| ID | 名称 |
|----|------|
| 1 | 中国大陆(China Mainland) |
| 2 | 中国香港(China HK) |
| 3 | 中国台湾(China Taiwan) |
| 4 | 欧美(Euro/American) |
| 5 | 日本(Japanese) |
| 6 | 韩国(Korea) |
| 7 | 泰国(Thailand) |
| 8 | 印度(India) |
| 9 | 俄罗斯(Russia) |
| 11 | 其它(Other) |

> 10个地区，含泰国/印度/俄罗斯，港/台独立

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 1 | UBits |
| 6 | UBWEB |
| 7 | UBTV |
| 5 | Other |

> 仅4个制作组，3个站组+Other。非管理组禁止选择UBits/UBWEB/UBTV

## 标签 tags[4][]

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 3 | 官方 |
| 17 | 原生原盘 |
| 4 | DIY |
| 5 | 国语 |
| 11 | 粤语 |
| 6 | 中字 |
| 21 | 高分国剧 |
| 12 | 特效字幕 |
| 8 | 杜比视界 |
| 10 | HDR10+ |
| 18 | 菁彩HDR |
| 9 | HDR10 |
| 23 | 连载 |
| 20 | 合集 |
| 19 | HLG |
| 22 | 自购 |
| 14 | 菜单修改 |

> 17个标签，含原生原盘/高分国剧/特效字幕/菜单修改等特色标签

## 标题命名规范（来自论坛topicid=269）

### 通用规则
- 主标题不含中文，直接使用原发布站主标题
- 转载时必须保留原站标识（如"DIY@UBits"或"UBits"）
- 禁止删除原站标识，违者删种+警告+取消发布权限一周

### 副标题
- 影片中文名称，如"特别响，非常近 / 心灵钥匙(台) / 咫尺浩劫(港)"
- 可附加"蓝光原盘"、"4K UHD"等信息

### 简介要求
- 必须包含：海报、简介文字、资源参数（BDinfo或Mediainfo）
- MediaInfo和BDInfo必须填写在种子发布页面对应位置
- 转载自其他站点，须在简介上方用[quote]标签标注来源

## 发布规则

### 种审制
- 发布后需审核通过
- 待审核状态不影响其他用户连接做种
- 不合格可修改后重新提交审核

### 禁止内容
- 总体积小于100MB的资源
- 标清upscale视频
- CAM/TC/TS/SCR等低质量视频
- RMVB/RM/flv文件
- RAR等压缩文件
- 色情/敏感政治内容

### 转载规则
- 必须用[quote]标签标注来源站
- 必须保留原站标识
- 禁止删除原站标识

## 转载注意事项

1. **仅源站角色**：官组非常活跃，不适合做发布站
2. **种审制**：发布后需等待审核，增加发布延迟
3. **禁止删除原站标识**：转载时保留原站标识，违者严惩
4. **制作组仅4个**：UBits/UBWEB/UBTV/Other，转载只能选Other
5. **编码含AVS**：有国产AVS编码选项
6. **1440p分辨率**：standard_sel包含1440p
7. **地区含泰国/印度/俄罗斯**：10个地区选项
8. **标签含原生原盘**：区分原盘和DIY
9. **标签含高分国剧**：特色标签
10. **IMDb+豆瓣**：同时支持IMDb链接和PT-Gen（豆瓣）获取简介
