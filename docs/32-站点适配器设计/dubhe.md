# 天枢 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 天枢 |
| 域名 | dubhe.site |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（部分用户需提交候选；游戏资源仅上传员及以上可自由发布） |
| MediaInfo | 是（technical_info） |
| IMDb | 是（url，含 PT-Gen data-pt-gen 属性） |
| 豆瓣 | 否 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| 价格系统 | 是（price，税率 30%） |

## Tracker URL
`https://dubhe.site/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | 含 PT-Gen data-pt-gen 属性 |
| NFO文件 | `nfo` | 否 | |
| 价格 | `price` | 否 | 下载时收费，税率 30% |
| 简介 | `descr` | 是 | BBCode |
| MediaInfo | `technical_info` | 否 | |
| 类型 | `type` | 是 | |
| 媒介 | `medium_sel[4]` | 否 | |
| 编码 | `codec_sel[4]` | 否 | |
| 分辨率 | `standard_sel[4]` | 否 | |
| 制作组 | `team_sel[4]` | 否 | |
| 标签 | `tags[4][]` | 否 | checkbox 多选 |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type, mode=4)

| ID | 名称 |
|----|------|
| 401 | Movies |
| 402 | TV Series |
| 403 | TV Shows |
| 404 | Documentaries |
| 405 | Animations |
| 406 | Music Videos |
| 407 | Sports |
| 408 | HQ Audio |
| 409 | Misc |
| 410 | Books |
| 411 | photo |

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
| 11 | H.265/HEVC |
| 10 | x.265 |
| 13 | H.264/AVC |
| 12 | x264 |
| 7 | MPEG-4 |
| 6 | MPEG-2 |
| 2 | VC-1 |
| 9 | AV1 |
| 8 | Xvid |
| 5 | Other |

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 5 | 2160p/2160i |
| 1 | 1080p/1080i |
| 3 | 720p |
| 4 | SD |
| 7 | Other/其他 |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | DubheWeb |

## 标签 tags[4][]（6 个）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

## 缺失字段

- **无 audiocodec_sel**
- **无 source_sel**
- **无 processing_sel**
- **无豆瓣链接**

## 特殊说明

1. **极简质量字段**：无音频编码、无来源、无处理方式，仅媒介/编码/分辨率/制作组四项
2. **编码 ID 非标准**：不从 1 开始，使用 5-13 范围（如 H.265/HEVC=11, x.265=10, H.264/AVC=13, x264=12）
3. **无 UHD Blu-ray 媒介**：最高媒介为 Blu-ray(1)，无 UHD Blu-ray 独立选项
4. **无 8K 分辨率**：最高分辨率为 2160p/2160i(5)
5. **仅 6 个标签**：极简标签集（禁转/首发/DIY/国语/中字/HDR）
6. **仅 6 个制作组**：含站组 DubheWeb(6)，传统组仅 HDS/CHD/MySiLU/WiKi
7. **特殊分类**：Books(410) 和 photo(411) 独立分类，无 Games 分类
8. **游戏资源限制**：PC 游戏仅上传员及以上等级可自由发布，其他用户需先提交候选
9. **价格系统**：seed 可设价格，下载时收费，税率 30%
10. **Dupe 规则**：Blu-ray/HD DVD > HDTV > DVD > TV 优先级；动漫 HDTV 和 DVD 同优先级
11. **PT-Gen 集成**：url 字段含 data-pt-gen="url" 属性，支持 PT-Gen 自动填充
12. **编码区分 HEVC/x265 和 AVC/x264**：H.265/HEVC(11) 与 x.265(10) 分开，H.264/AVC(13) 与 x264(12) 分开
