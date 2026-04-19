# 樱花 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 樱花 |
| 域名 | pt.ying.us.kg |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是 |
| MediaInfo | 是（technical_info） |
| IMDb | 是（url，data-pt-gen="url"） |
| 豆瓣 | 否 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| 建站时间 | 2025年 |

## Tracker URL
`https://pt.ying.us.kg/announce.php`

## 站点特色

- **新站（2025年建）**：站点还在发展初期
- **单mode设计**：仅种子区（mode=4），无特别区/音乐区
- **无音频编码字段**：质量区域仅有媒介/编码/分辨率/制作组，无独立音频编码下拉
- **无地区字段**：无source_sel/processing_sel
- **媒介ID非标准**：UHD Blu-ray=1, Blu-ray=2（标准站通常是反的）
- **编码含AV1**：codec_sel包含AV1
- **分辨率含8K**：standard_sel包含8K/4320p
- **制作组含YHWeb**：6个制作组，含站组YHWeb
- **标签含韩剧**：tags中有"韩剧"标签，暗示韩剧资源特色
- **标签含超分/零魔**：有"超分"、"零魔"、"大包"、"合并"、"杜比"等特色标签
- **PT-Gen**：有PT-Gen按钮（data-pt-gen="url"）
- **规则与学校/阳光/壹吧同模板**
- **发布者双倍上传量**

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 否 | 有"填写质量"按钮 |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | data-pt-gen="url" |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode，有预览功能 |
| MediaInfo | `technical_info` | 否 | MediaInfo文本 |
| 类型 | `type` | 是 | data-mode=4 |
| 媒介 | `medium_sel[4]` | 否 | |
| 编码 | `codec_sel[4]` | 否 | |
| 分辨率 | `standard_sel[4]` | 否 | |
| 制作组 | `team_sel[4]` | 否 | |
| 标签 | `tags[4][]` | 否 | checkbox多选，16个 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | 电影 |
| 402 | 电视剧 |
| 403 | 综艺 |
| 404 | 纪录片 |
| 405 | 动漫 |
| 406 | MV |
| 407 | 体育 |
| 408 | 音频 |
| 410 | 短剧 |
| 409 | 其他 |

> 无游戏/软件/电子书分类

## 质量字段

### 媒介 medium_sel[4]

| ID | 名称 |
|----|------|
| 1 | UHD Blu-ray |
| 2 | Blu-ray |
| 3 | Remux |
| 4 | WEB-DL |
| 5 | HDTV |
| 6 | DVD |
| 7 | Encode |
| 8 | CD |
| 9 | Track |
| 10 | Other |

> **媒介ID非标准**：UHD Blu-ray=1, Blu-ray=2（标准站通常是1=Blu-ray, 11/13=UHD Blu-ray）

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | H.264/AVC |
| 7 | H.265/HEVC |
| 2 | VC-1 |
| 4 | MPEG-2 |
| 6 | AV1 |
| 5 | Other |

> 含AV1，无Xvid/VP9

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 4 | 480p/480i |
| 3 | 720p/720i |
| 2 | 1080p/1080i |
| 1 | 4K/2160p/2160i |
| 5 | 8K/4320p/4320i |
| 6 | Other |

> **分辨率ID非标准**：4K=1, 1080p=2, 720p=3（与标准NexusPHP完全相反）

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 4 | WiKi |
| 3 | MySiLU |
| 1 | HDS |
| 2 | CHD |
| 5 | Other |
| 6 | YHWeb |

> 含经典压制组HDS/CHD/MySiLU/WiKi，站组YHWeb

## 标签 tags[4][]

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 粤语 |
| 9 | 杜比 |
| 10 | 合并 |
| 11 | 零魔 |
| 12 | 超分 |
| 13 | 大包 |
| 14 | 应求 |
| 15 | 完结 |
| 16 | 分集 |
| 17 | 英字 |
| 18 | 韩剧 |

> 16个标签，含韩剧/超分/零魔/大包/合并/杜比等特色标签，无独立HDR10/HDR10+/DV标签

## 标题命名规范（来自rules.php）

### 电影类
- `[中文名] 名称 [年份] [剪辑版本] [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 例：`蝙蝠侠:黑暗骑士 The Dark Knight 2008 PROPER 720p BluRay x264-SiNNERS`

### 剧集类
- `[中文名] 名称 [年份] S**E** [发布说明] 分辨率 来源 [音频/]视频编码-发布组名称`
- 例：`越狱 Prison Break S04E01 PROPER 720p HDTV x264-CTU`

### 音轨类
- `[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 [年份] [版本] [发布说明] 音频编码[-发布组名称]`
- 例：`恩雅 - 冬季降临 Enya - And Winter Came 2008 FLAC`

## 发布规则

### 禁止内容
- 总体积小于100MB的资源
- 标清upscale视频
- CAM/TC/TS/SCR/DVDSCR/R5/HalfCD
- RMVB/RM/flv文件
- 有损MP3/有损WMA（未达5.1声道标准）
- RAR等压缩文件
- 色情/敏感政治内容

### Dupe规则
- 来源媒介优先级：Blu-ray/HD DVD > HDTV > DVD > TV
- 断种45日或发布18个月以上可重发

### 账号保留
- Veteran User及以上：永久保留
- Elite User及以上封存后：不删除
- 封存账号：400天未登录删除
- 未封存账号：150天未登录删除
- 无流量用户：100天未登录删除

## 转载注意事项

1. **媒介ID非标准**：UHD Blu-ray=1, Blu-ray=2，与标准NexusPHP相反，转载映射时需特别注意
2. **分辨率ID非标准**：4K=1, 1080p=2, 720p=3, 480p=4，也与标准NexusPHP相反
3. **无音频编码字段**：没有audiocodec_sel下拉，音频编码信息无法通过下拉选择传递
4. **无地区字段**：没有source_sel/processing_sel，无法选择地区
5. **单mode**：仅mode=4种子区，无特别区/音乐区
6. **新站2025年**：站点规则和字段配置可能后续有调整
7. **制作组仅6个**：HDS/CHD/MySiLU/WiKi/Other/YHWeb
8. **标签含韩剧特色**：有韩剧/英字标签，暗示韩剧资源特色
9. **无单独HDR细分**：仅有HDR标签，无HDR10/HDR10+/DV细分
10. **规则与多站同模板**：学校/阳光/壹吧/樱花使用完全相同的规则模板
