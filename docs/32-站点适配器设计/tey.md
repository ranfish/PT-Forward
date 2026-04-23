# 太乙 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 太乙 |
| 域名 | pt.tey.cc |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是（offers.php） |
| PT-Gen | 是 |
| MediaInfo | 是（technical_info） |
| IMDb | 是 |
| 豆瓣 | 是 |
| NFO | 是 |
| 匿名发布 | 否（无 uplver 字段） |
| **资源限制** | **韩国资源特色站，不接受其它产地资源** |

## Tracker URL
`https://tracker.tey.cc/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
| PT-Gen | `pt_gen` | 否 | |
| NFO文件 | `nfo` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| MediaInfo | `technical_info` | 否 | |
| 类型 | `type` | 是 | |
| 分辨率 | `standard_sel[4]` | 否 | |
| 媒介 | `medium_sel[4]` | 否 | |
| 编码 | `codec_sel[4]` | 否 | |
| 音频编码 | `audiocodec_sel[4]` | 否 | |
| 制作组 | `team_sel[4]` | 否 | |
| 标签 | `tags[4][]` | 否 | checkbox 多选 |

## 分类 (type, mode=4)

| ID | 名称 |
|----|------|
| 401 | Movies \| 电影 |
| 402 | TV Series \| 剧集 |
| 403 | TV Shows \| 综艺 |
| 404 | Documentaries \| 纪录 |
| 405 | Animations \| 动漫 |
| 406 | Music Videos \| MV |
| 407 | Sports \| 体育 |
| 408 | HQ Audio \| 无损 |
| 409 | Other \| 其他 |

## 质量字段 (mode=4)

### 分辨率 standard_sel[4]

| ID | 名称 |
|----|------|
| 1 | 8K |
| 2 | 4K |
| 3 | 1080p |
| 4 | 1080i |
| 5 | 720p |

### 媒介 medium_sel[4]

| ID | 名称 |
|----|------|
| 1 | WEB-DL |
| 2 | HDTV |
| 6 | DVDR |
| 8 | CD |
| 9 | Track |

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | H.264(x264/AVC) |
| 3 | H.265(x265/HEVC) |
| 4 | MPEG-2 |
| 5 | VP8/9 |

### 音频编码 audiocodec_sel[4]

| ID | 名称 |
|----|------|
| 1 | Other |
| 2 | DTS-HDMA:X 7.1 |
| 3 | DTS-HDMA |
| 4 | DTS |
| 5 | TrueHD Atmos |
| 6 | TrueHD |
| 7 | E-AC3 Atmos(DDP Atmos) |
| 8 | E-AC3(DDP) |
| 9 | AC3(DD) |
| 10 | AAC |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 1 | Tey |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |

## 标签 tags[4][]（14 个）

| ID | 名称 |
|----|------|
| 2 | 完结 |
| 3 | 连载 |
| 4 | 禁转 |
| 5 | 首发 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 短剧 |
| 9 | 零魔 |
| 10 | 10bit |
| 11 | 杜比 |
| 12 | DIY |
| 13 | 国配 |
| 14 | 粤语 |
| 15 | 纯享 |

## 缺失字段

- **无 source_sel**（来源）
- **无 processing_sel**（处理方式）
- **无匿名发布（uplver）**
- **无 description/keywords 字段**
- **媒介极简**：仅 5 种（WEB-DL/HDTV/DVDR/CD/Track），无 Blu-ray/Remux/Encode
- **编码极简**：仅 5 种，无 AV1/VVC
- **制作组极少**：仅 5 个（Tey/CHD/MySiLU/WiKi/Other）

## 站点规则

### 盒子规则

- 使用盒子需及时登记备案（IPv4/IPv6 都需登记）
- **普通用户默认限速 200Mbps**
- 超速会被系统自动禁用下载权限

### H&R 规则

从用户信息可见：`H&R: [种子区: 0/0/10 特别区: 0/0/10]`
- 种子区和特别区各有独立的 H&R 记录，上限各 10 个

### 认领系统

从用户信息可见：`认领: [0/1000]`
- 每人最多认领 1000 个种子

### 促销类型（spstate）

| 值 | 名称 |
|----|------|
| 1 | 普通 |
| 2 | 免费 |
| 3 | 2X |
| 4 | 2X免费 |
| 5 | 50% |
| 6 | 2X 50% |
| 7 | 30% |

### 种子审核系统

有审核流程：approval_status（未审/通过/拒绝）

## 特殊说明

1. **韩国资源特色站**：只接受韩国产地的资源，不接受其它产地
2. **极简媒介**：无 Blu-ray/Remux/Encode/HD DVD/MiniBD，韩国资源以 WEB-DL 和 HDTV 为主
3. **分辨率含 8K**：最高支持 8K，无 SD/480p 等低分辨率
4. **standard_sel 替代 resolution_sel**：字段名为 standard_sel
5. **音频编码 Atmos 细分**：区分 DTS-HDMA:X 7.1、TrueHD Atmos、E-AC3 Atmos 三种 Atmos 格式
6. **站组 Tey**：制作组含站组 Tey(1)
7. **标签含韩国特色**：短剧、零魔、国配、粤语、纯享
8. **PT-Gen + MediaInfo 齐全**：有 pt_gen 和 technical_info 字段
9. **盒子限速 200Mbps**：普通用户默认限速，盒子需登记备案
10. **双区 H&R**：种子区和特别区各有独立 H&R 配额
