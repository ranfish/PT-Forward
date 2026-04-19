# 我堡 站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 我堡 |
| 域名 | ourbits.club |
| 框架 | NexusPHP |
| Cloudflare | 是 |
| 候选制 | 是（部分用户需提交候选） |
| MediaInfo | 是（简介中附带，非独立字段） |
| IMDb | 是（url） |
| 豆瓣 | 是（外部信息要求必填） |
| 匿名发布 | 是（uplver） |
| NFO | 否 |
| PT-Gen | 否 |

## Tracker URL
`https://ourbits.club/announce.php`

## 发布页面字段

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | 是 | |
| 标题 | `name` | 是 | |
| 副标题 | `small_descr` | 否 | |
| IMDb链接 | `url` | 否 | 电影和电视剧必填 |
| 简介 | `descr` | 是 | BBCode |
| 类型 | `type` | 是 | |
| 媒介 | `medium_sel` | 否 | |
| 编码 | `codec_sel` | 否 | |
| 分辨率 | `standard_sel` | 否 | |
| 音频编码 | `audiocodec_sel` | 否 | |
| 处理方式/地区 | `processing_sel` | 否 | 实际为地区 |
| 制作组 | `team_sel` | 否 | 仅"原创/原抓" |
| 标签 | `tags[]` | 否 | checkbox 多选，字符串值 |
| 匿名发布 | `uplver` | 否 | |
| 候选关联 | `offer` | 否 | 下拉选择关联候选 |

## 分类 (type)

| ID | 名称 |
|----|------|
| 401 | Movies / 电影 |
| 402 | Movies-3D / 3D电影 |
| 419 | Concert / 演唱会 |
| 412 | TV-Episode / 电视剧 |
| 405 | TV-Pack / 电视剧包 |
| 413 | TV-Show / 综艺节目 |
| 410 | Documentary / 纪录片 |
| 411 | Animation / 动漫 |
| 415 | Sports / 体育 |
| 414 | Music-Video / MV |
| 416 | Music / 音乐 |

## 质量字段

### 媒介 medium_sel

| ID | 名称 |
|----|------|
| 1 | FHD Blu-ray |
| 2 | DVD |
| 5 | HDTV |
| 7 | Encode |
| 8 | CD |
| 9 | WEB-DL |
| 12 | UHD Blu-ray |
| 13 | UHDTV |

### 编码 codec_sel

| ID | 名称 |
|----|------|
| 12 | H.264 |
| 14 | HEVC |
| 15 | MPEG-2 |
| 16 | VC-1 |
| 17 | Xvid |
| 18 | Other |
| 19 | AV1 |

### 分辨率 standard_sel

| ID | 名称 |
|----|------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p |

### 音频编码 audiocodec_sel

| ID | 名称 |
|----|------|
| 1 | DTS-HDMA |
| 2 | TrueHD |
| 4 | DTS |
| 5 | LPCM |
| 6 | AC3 |
| 7 | AAC |
| 11 | WAV |
| 12 | APE |
| 13 | FLAC |
| 14 | Atmos |
| 21 | DTS X |
| 32 | MPEG |
| 33 | OPUS |

### 处理方式/地区 processing_sel

| ID | 名称 |
|----|------|
| 1 | CN(中国大陆) |
| 2 | US/EU(欧美) |
| 3 | HK/TW(港台) |
| 4 | JP(日) |
| 5 | KR(韩) |
| 6 | OT(其他) |

### 制作组 team_sel

| ID | 名称 |
|----|------|
| 41 | 原创/原抓 |

## 标签 tags[]（11 个，字符串值）

| 值 | 名称 |
|----|------|
| sf | 首发 |
| diy | DIY |
| gy | 国语 |
| zz | 中字 |
| yq | 应求 |
| jz | 禁转 |
| db | 杜比视界 |
| hdrvivid | 菁彩HDR |
| hdr | HDR10 |
| hdrp | HDR10+ |
| hlg | HLG |

## 缺失字段

- **无 source_sel**
- **无 NFO 上传**
- **无 PT-Gen**
- **无独立 MediaInfo 字段**（在简介中附带）
- **team_sel 仅"原创/原抓"**（无外部制作组列表）

## 特殊说明

1. **processing_sel = 地区**：字段名为处理方式，实际值为中国大陆/欧美/港台/日/韩/其他
2. **媒介区分 FHD/UHD**：FHD Blu-ray(1) 与 UHD Blu-ray(12) 独立，含 UHDTV(13)
3. **标签使用字符串值**：tags[] 使用拼音缩写值（sf/diy/gy/zz 等），非数字 ID
4. **11 个标签含 HDR 细分**：杜比视界(db)/菁彩HDR(hdrvivid)/HDR10(hdr)/HDR10+(hdrp)/HLG(hlg)
5. **制作组仅"原创/原抓"**：无外部制作组下拉列表，官方组通过标题后缀标识
6. **官方组体系完整**：OurBits/PbK/OurTV/iLoveTV/Ao/MGs/OurPad/HosT/iLoveHD + 合作组FLTTH
7. **黑名单制作组**：禁止 FRDS 小组、FGT 小组的资源
8. **禁止 REMUX/WebRip**：非官方组 remux 禁止发布，WebRip 文件禁止
9. **3D 电影独立分类**：Movies-3D(402)，3D 原盘必须是 ISO 格式
10. **电视剧分集/合集拆分**：TV-Episode(412) 和 TV-Pack(405) 独立分类
11. **Dupe 规则严格**：官方组在 Dupe 中有优先权；Encode 只允许 Scene 与 P2P 组各一个版本
12. **IMDb 和豆瓣必填**：电影和电视剧外部信息链接要求必填
13. **编码矩阵规定编码-分辨率映射**：FHD Blu-ray→H.264(720p/1080p)/HEVC(1080p)；UHD Blu-ray→仅HEVC(1080p/2160p)
14. **候选关联字段 offer**：发布时可关联候选条目
15. **H&R 规则**：14 天内做种 48 小时或分享率 1.0，累计 10 次 HR 警告封禁
16. **盒子限速规则**：发布 72 小时内盒子上传限 3 倍种子大小，下载按优惠降级计算
