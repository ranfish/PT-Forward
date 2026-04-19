# 时光 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 时光 |
| 域名 | hdtime.org |
| 框架 | NexusPHP |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（offers.php） |
| PT-Gen | 是（imdb/douban/bangumi/indienova） |
| MediaInfo | 是 |
| 匿名发布 | 是（uplver=yes） |

## 发布规范摘要

来源：`forums.php?action=viewtopic&forumid=1&topicid=83`

### 转发规则
1. 个人会员未经许可不得使用 HDTime/PadTime 等标识或后缀发布作品
2. 转发资源不得随意去掉或篡改原创小组的标识、后缀（如 CHD/CMCT）
3. 严禁转发其他站点的独占、禁转、限转（未解除限制前）资源
4. 禁止随意修改原始文件、文件名或自行增减内容后重新做种发布
5. 发布前先搜索站内是否有重复资源
6. 严格遵守命名及格式规范

### 标题命名规范

**视频/电影类：**
- 主标题：英文片名 版本信息 上映(发行)年代 发布说明 原始媒介 分辨率 音频信息 视频编码-制作人/发布组
- 副标题：中文片名(日韩文片名) [附加说明]
- 例：`The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS` / `蝙蝠侠:黑暗骑士 [导演剪辑版] [国/英] [修正版]`

**剧集类：**
- 主标题：英文剧集名 上映(发行)年代 季数集数 发布说明 分辨率 原始媒介 音频信息 视频编码-制作人/发布组
- 副标题：中文剧集名(日韩文剧集名) (季数 集数) [附加说明]
- 例：`Prison Break S04E01 PROPER 720p HDTV x264-CTU` / `越狱 (第四季 第1集) [修正版]`

**音乐类：**
- 主标题：英文艺术家名 - 英文专辑名 版本信息 发行年代 发布说明 音频编码-发布组
- 副标题：中文艺术家名 - 中文专辑名 版本信息 [附加说明]
- 例：`Enya - And Winter Came Deluxe Edition 2008 APE` / `恩雅 - 冬季降临 [豪华收藏版]`

### 内容格式规范
- 视频电影类必须包含海报/封面，尽量包含画面截图和资源详情（年代、国家、导演、主演、语言、字幕、格式、时长、分辨率等）
- 体育类请勿在文字介绍或截图/文件名/文件大小/时长中泄漏比赛结果
- 音乐类必须包含专辑封面和曲目列表

## 发布页面字段

### 表单字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | file input |
| 标题 | `name` | 否（推荐） | 若不填使用种子文件名 |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | data-pt-gen="url" |
| PT-Gen | `pt_gen` | 否 | data-pt-gen="pt_gen"，来源 imdb/douban/bangumi/indienova |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode编辑器，附件上传 |
| MediaInfo | `technical_info` | 否 | textarea |
| 类型 | `type` | 是 | 下拉选择，data-mode='4' |
| 质量 | 多个 select | 否 | 媒介/编码/音频编码/制作组，仅 mode=4 |
| 标签 | `tags[4][]` | 否 | checkbox 多选 |
| 匿名发布 | `uplver` | 否 | checkbox，value="yes" |

### Tracker URL
`https://tracker.hdtime.org/announce.php`

## 分类 (type, mode=4)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 剧集 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音乐 |
| 409 | 其他 |
| 410 | 游戏 |
| 411 | 文档 |
| 414 | 软件 |
| 424 | Blu-Ray原盘 |

## 质量字段 (mode=4)

### 媒介 medium_sel[4]

| ID | 名称 |
|----|------|
| 1 | Blu-ray |
| 2 | HD DVD |
| 3 | Remux |
| 4 | MiniBD |
| 5 | HDTV |
| 6 | DVDR |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | WEB-DL |

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | AVC/H.264/x264 |
| 2 | VC-1 |
| 3 | xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 11 | x264-10bit |
| 12 | HEVC/H.265/x265 |
| 14 | VP8/VP9 |
| 15 | AV1 |
| 16 | VVC/H.266/x266 |
| 17 | AVS3 |

### 音频编码 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | Other |
| 8 | TrueHD Atmos |
| 9 | TrueHD |
| 10 | DTS-HD |
| 11 | DD/AC3 |
| 12 | DDP/EAC3 |
| 13 | LPCM/PCM |
| 14 | Audio Vivid/AV3A |
| 15 | OPUS |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 2 | CHD |
| 3 | beAst |
| 4 | WiKi |
| 5 | Other |
| 6 | HDTime |
| 7 | PADTime |
| 8 | CMCT |
| 9 | 个人原创 |
| 12 | HDT |
| 15 | VTime |
| 16 | QHstudIo |
| 17 | AilMWeb |
| 18 | HHWEB |

## 标签 tags[4][]

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官方 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 杜比视界 |

## 缺失字段

- **无分辨率 (resolution_sel)**
- **无来源 (source_sel)**
- **无处理方式 (processing_sel)**
- **无地区选择**

## 特殊说明

1. **极简质量字段**：仅有媒介/编码/音频编码/制作组四个下拉框，无分辨率、来源、处理方式、地区等常见字段
2. **候选制**：导航栏有"候选"(offers.php)入口，存在候选发布机制
3. **PT-Gen 四来源**：支持 imdb/douban/bangumi/indienova 四个来源获取简介
4. **Cloudflare 保护**：需要 cf_clearance cookie
5. **置顶促销**：支持魔力值置顶促销功能
6. **附件上传**：简介区域有 iframe 附件上传（attachment.php）
7. **分类含 Blu-Ray原盘**：424 为独立分类而非质量选项
8. **制作组含站组**：HDTime(6)、HDT(12)、PADTime(7)、VTime(15) 为站点相关制作组
9. **编码含新一代编码**：支持 AV1(15)、VVC/H.266(16)、AVS3(17)、Audio Vivid/AV3A(14)
