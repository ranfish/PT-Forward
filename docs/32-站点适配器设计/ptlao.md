# 忘年桥 站点适配器设计

> **⛔ 禁止下载"特——别"版块内容、禁止发布"特别区"类型。**
>
> 本站含成人向分类（old/middle/men and women/youth）。

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 忘年桥 |
| 域名 | www.ptlao.top |
| 框架 | NexusPHP |
| Cloudflare | 否 |
| 候选制 | 是 |
| MediaInfo | 是（technical_info） |
| IMDb | 是（url） |
| 豆瓣 | 是 |
| 匿名发布 | 是（uplver） |
| NFO | 是 |
| PT-Gen | 是（pt_gen） |
| 价格系统 | 是（price） |

## Tracker URL
`https://ptlao.top/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | |
| PT-Gen | `pt_gen` | 否 | PT-Gen 链接/ID |
| NFO文件 | `nfo` | 否 | |
| 价格 | `price` | 否 | |
| 简介 | `descr` | 是 | BBCode |
| MediaInfo | `technical_info` | 否 | |
| 类型 | `type` | 是 | |
| 媒介 | `medium_sel[4]` / `medium_sel[5]` | 否 | 双 mode |
| 编码 | `codec_sel[4]` / `codec_sel[5]` | 否 | 双 mode |
| 分辨率 | `standard_sel[4]` / `standard_sel[5]` | 否 | 双 mode |
| 制作组 | `team_sel[4]` / `team_sel[5]` | 否 | 双 mode |
| 标签 | `tags[4][]` / `tags[5][]` | 否 | checkbox 多选，双 mode |
| 匿名发布 | `uplver` | 否 | |

## 分类 (type)

| ID | 名称 |
|----|------|
| 410 | old |
| 411 | middle |
| 412 | men and women |
| 413 | youth |

## 质量字段 (mode=4 和 mode=5 相同)

### 媒介 medium_sel

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
| 10 | 4K UHD Remux |
| 11 | WEB-DL |

### 编码 codec_sel

| ID | 名称 |
|----|------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 8 | H.265(HEVC) |

### 分辨率 standard_sel

| ID | 名称 |
|----|------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 4k/2016p |

### 制作组 team_sel

| ID | 名称 |
|----|------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | PTL |

## 标签

### tags[4][]（10 个，mode 4）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官方 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 12 | 首发禁转 |
| 13 | 原创 |
| 14 | 同性 |

### tags[5][]（10 个，mode 5）

| ID | 名称 |
|----|------|
| 1 | 禁转 |
| 2 | 首发 |
| 3 | 官方 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |
| 8 | 男男 |
| 9 | 男女 |
| 12 | 首发禁转 |
| 13 | 原创 |

## 缺失字段

- **无 audiocodec_sel**
- **无 source_sel**
- **无 processing_sel**

## 特殊说明

1. **4 个分类全部为成人向**：old(410)/middle(411)/men and women(412)/youth(413)，无标准影视分类
2. **双 mode 质量字段**：mode=4 和 mode=5 字段完全相同（媒介/编码/分辨率/制作组），但标签不同
3. **标签按 mode 区分**：mode 4 含"同性"(14)，mode 5 含"男男"(8)/"男女"(9)
4. **分辨率写法错误**：4K 写为 "4k/2016p"（疑似应为 2160p）
5. **仅 6 个制作组**：含站组 PTL(6)，传统组仅 HDS/CHD/MySiLU/WiKi
6. **极简编码**：仅 6 种（H.264/H.265/VC-1/Xvid/MPEG-2/Other），无 AV1/VP9
7. **PT-Gen 独立字段**：有专门的 pt_gen 字段
8. **媒介含 4K UHD Remux**：独立于 Blu-ray，但无独立 UHD Blu-ray
9. **禁止暴力/血腥/恋童/未成年人内容**：特别区规则明确禁止
10. **综合区禁止露点自制内容**：正规影片有少量允许
