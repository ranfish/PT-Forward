# 时间 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 时间 |
| 域名 | www.pttime.org |
| 框架 | PTT-NP（NexusPHP 变种，generator="PTT-NP"） |
| Cloudflare | 是（cf_clearance） |
| 候选制 | 是（offers.php，非 Power User 需先候选） |
| PT-Gen | 否（无 PT-Gen 字段） |
| MediaInfo | 否（无 technical_info 字段） |
| 匿名发布 | 是（anonymous） |
| 9KG专区 | 是（adults.php + upload_adults.php，独立发布页） |
| **角色** | **禁止发布（仅记录及设计参考）** |

## 重要声明

**本站禁止发布任何资源。仅做记录及设计参考用途。**

## 发布规范摘要

来源：`forums.php?action=viewtopic&forumid=3&topicid=16`（规则&教程）及 `forums.php?action=viewtopic&topicid=9`（发种及三严说明）

### 发种规则
1. 非 Power User 需先在候选区添加候选，管理员手动通过后才能发布
2. 清晰度要求：教程 480P 起，其它影视资源 720P 起（720P 应不低于 1G，1080P 应不低于 2G）
3. 清晰度较低不要发布，包括但不限于 HC、CAM、TS、小于 480P
4. 不带广告（澳门赌场、APP 二维码、非发行方的第三方网址等）
5. 不改动文件结构、文件名，直接复制它站相关信息或插件搬运
6. 可以打包为合集
7. 删除本地资源需至少 5 个做种者，建议 10 个

### 禁止发布类
- 院线在映、三严、它站禁转、非必要合集/分集/单本书
- **不建议上传**后缀为 Mp4ba、rag、rarbg、rartv 和 FGT 的资源（黑名单制作组）

### 三严（严重违法）
1. 儿童（或疑似儿童）色情
2. 国内私拍、偷拍、迷奸
3. 枪支弹药制造、毒品
4. 颠覆国家政权、恐怖活动
5. 人兽、屎尿、非科研用途解剖等超重口味
6. 公民信息及重要机密
7. 虚假信息
8. 巨大争议或严重歧视信息
9. 计算机病毒
10. AI 明星换脸的 9KG

### 发布奖励
- 教育类资源（社科、考试、编程）：3X 上传
- 其他资源：2X 上传
- 9KG：1X 上传

## 发布页面字段

### 表单字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | file input |
| 类型 | `type` | 是 | 下拉选择 |
| 标签 | `tags[]` | 否 | checkbox 多选，按分类显示不同标签组 |
| 演员 | `actor` | 否 | |
| 标题 | `name` | 否 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `imdb_id` | 否 | IMDb ID |
| 豆瓣电影链接 | `dburl` | 否 | 豆瓣链接 |
| 封面图 | `pt_img` | 否 | 封面图片 |
| 视频链接 | `pt_video` | 否 | |
| 简介 | `descr` | 是 | BBCode 编辑器 |
| 多选/匿名 | `anonymous` | 否 | checkbox |
| 工厂 | `factory` | 否 | |

### 表单提交
- action: `takeupload.php`
- 方法: POST + multipart/form-data

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | Movies（电影、电影短片，不含动漫） |
| 402 | TV Series（连续剧、连续短剧） |
| 403 | TV Shows（综艺） |
| 404 | Documentaries（纪录片） |
| 405 | Sport（体育、健身、运动） |
| 406 | ACG（动漫、卡通、二次元、漫画及相关） |
| 407 | Baby（婴幼、儿童、早教、小学以下） |
| 408 | Music（音乐、专辑、MV、演唱会） |
| 409 | Art（曲艺、相声、小品、戏曲、舞蹈、歌剧、评书等） |
| 411 | Knowledge（社科、文学、知识、技能、书刊、有声书等） |
| 412 | School（应试、考级、职称、初中以上） |
| 420 | Code（编程技术&教程、AI、信息技术） |
| 421 | Games（游戏及相关） |
| 422 | Software（软件、系统、程序、APP等） |
| 423 | Resource（素材、数据、图片、文档、模板） |
| 490 | Other（其它） |

## 质量字段

**无质量下拉框。** 没有 medium_sel、codec_sel、audiocodec_sel、team_sel、resolution_sel、source_sel、processing_sel 等标准 NexusPHP 质量字段。

## 标签 tags[]（按分类动态显示）

### 影视类标签（电影/剧集/综艺/纪录片/体育/动漫）

| 值 | 名称 |
|----|------|
| jz | 禁转 |
| xz | 限转 |
| xpt | 限转PT |
| yq | 应求 |
| xs | 新手 |
| dwj | 原盘或ISO |
| sk | 4K(+) |
| vr | 3D或VR |
| dbsj | 杜比视界 |
| zz | 中字 |
| zy | 中英双字 |
| yz | 英字 |
| qt | 其他字幕 |
| wzz | 无中字 |
| hj | 合集 |
| diy | DIY/自压/原创 |
| short | 短剧 |

### 音乐类标签

| 值 | 名称 |
|----|------|
| jzm | 禁转 |
| xzm | 限转 |
| xptm | 限转PT |
| yqm | 应求 |
| xsm | 新手 |
| yym | 英语 |
| yuem | 粤语 |
| fym | 方言 |
| hym | 韩语 |
| rym | 日语 |
| eym | 俄语 |
| ydym | 印语 |
| wym | 其他音轨 |
| hj2 | 合辑(Compilation) |
| jx | 精选(Anthology) |
| zj | 专辑(Album) |
| dq | 单曲(Single) |
| ep | 迷你专辑(EP) |
| zhy | 重混音(Remix) |
| sz | 私制唱片(Bootleg) |
| ly | 录音样带(Demo) |
| qtlx | 其他(Other) |
| mp3 | MP3 |
| flac | FLAC |
| ape | APE |
| wav | WAV |
| dsd | DSD/DTS/DSF/DFF/SACD(Hi-Res) |
| pcm | PCM(Hi-Res) |
| mqa | MQA |
| dbqjs | 杜比全景声 |
| mv | MV |
| ych | 演唱会/音乐会 |
| ysjq | 影视金曲 |
| wlrq | 网络热曲 |
| zhg | 整轨 |
| fg | 分轨 |
| dwjm | 原盘或ISO |

### 黑名单制作组（不建议发布）

Mp4ba、rag、rarbg、rartv、FGT

## 缺失字段

- **无媒介 (medium_sel)**
- **无编码 (codec_sel)**
- **无音频编码 (audiocodec_sel)**
- **无制作组 (team_sel)**
- **无分辨率 (resolution_sel)**
- **无来源 (source_sel)**
- **无处理方式 (processing_sel)**
- **无 PT-Gen**
- **无 MediaInfo**
- **无 NFO 文件上传**

## 特殊说明

1. **PTT-NP 框架**：非标准 NexusPHP，是自研变种（generator="PTT-NP"），UI 和结构与标准 NexusPHP 差异较大
2. **极简发布**：只有分类和标签两个下拉/多选，无任何质量下拉框
3. **标签使用字符串值**：tags[] 的值是拼音缩写字符串（如 jz、xz、dbsj），非数字 ID
4. **标签按分类动态显示**：影视类和音乐类各有不同标签组
5. **9KG 双区**：综合区(torrents.php) + 9KG区(adults.php)，发布页分开（upload.php / upload_adults.php）
6. **候选制**：非 Power User 必须先候选，管理通过后才能发布
7. **双链接**：同时支持 IMDb 链接(imdb_id)和豆瓣电影链接(dburl)
8. **封面图独立字段**：有 pt_img 封面图字段
9. **16 个分类**：涵盖电影/剧集/综艺/纪录片/体育/动漫/婴幼/音乐/曲艺/知识/应试/编程/游戏/软件/素材/其它
10. **错误分类处罚**：错误分入奖励类将删除资源、关闭发布权限、扣减上传量
