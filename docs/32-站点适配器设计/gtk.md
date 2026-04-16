# GTK 站点适配器

## 站点信息

- **站点名称**: PT GTK
- **站点地址**: https://pt.gtkpw.xyz
- **框架**: NexusPHP
- **域名变更**: 旧域名 pt.gtk.pw，当前域名 pt.gtkpw.xyz
- **登录特性**: Challenge-Response 认证（SHA256 + HMAC-SHA256），支持两步验证 + 验证码

---

## 一、发布页面表单字段分析

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | ✓ | 标题（若不填将使用种子文件名） |
| `small_descr` | text | - | 副标题（显示在种子标题下方） |
| `url` | text | - | IMDb 链接（带 PT-Gen "获取简介"按钮） |
| `pt_gen` | text | - | PT-Gen 链接（带 "获取简介"按钮） |
| `nfo` | file | - | NFO 文件 |
| `descr` | textarea | ✓ | 简介（BBCode 编辑器，20行） |
| `technical_info` | textarea | - | MediaInfo/BDInfo（8行） |
| `uplver` | checkbox | - | 匿名发布（value="yes"） |

### 1.2 质量选择字段

质量字段名带 `[4]` 后缀（如 `medium_sel[4]`），`data-mode='4'` 表示当前分类模式。

#### 类型（`type`）— 必填

| 值 | 显示名称 |
|----|----------|
| 401 | Movies(电影) |
| 402 | TV Series(剧集) |
| 403 | TV Shows(综艺) |
| 404 | Documentaries(纪录片) |
| 405 | Animations(动画) |
| 406 | Music Videos(MV) |
| 407 | Sports(运动题材) |
| 408 | HQ Audio(高清音频) |
| 409 | Misc(其他) |
| 410 | Book(图书) |
| 411 | Music Album(音乐专辑) |
| 412 | Education(资料) |

#### 媒介（`medium_sel[4]`）

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
| 10 | UHD |
| 11 | WEB-DL |

#### 视频编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265/HEVC |
| 7 | AV1 |
| 8 | VP9 |

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p |
| 2 | 1080i |
| 3 | 720p |
| 4 | SD |
| 5 | 2160p/4K |
| 6 | 4320p/8K |

#### 制作组（`team_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | HDS |
| 2 | CHD |
| 3 | MySiLU |
| 4 | WiKi |
| 5 | Other |
| 6 | CMCT |
| 7 | MARK |
| 8 | MTeam |
| 9 | FRDS |
| 10 | PTHome |
| 11 | beAst |

#### 标签（`tags[4][]`）— 多选 checkbox

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 2 | 首发 |
| 4 | DIY |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR |

### 1.3 缺失字段

与标准 NexusPHP 相比，GTK 发布页**缺少**以下常见字段：

- `audiocodec_sel` — 无音频编码选择
- `processing_sel` — 无地区/来源选择

---

## 二、与其他站点对比

### 2.1 与 13City 对比

| 维度 | GTK | 13City |
|------|-----|--------|
| 分类ID | 401-412（12类） | 401-413（8类，无纪录片404） |
| 媒介值 | UHD=10, WEB-DL=11 | WEB-DL=10, BluRay=11, WEBRip=12, Other=13 |
| 编码值 | H.265/HEVC=6, AV1=7, VP9=8 | AVC/H.264/x264=1, HEVC/H.265/x265=2 |
| 分辨率值 | 2160p/4K=5, 4320p/8K=6 | 1080p=1, 1080i=2, 720p=3, SD=4 |
| 制作组 | HDS/CHD/MySiLU/WiKi/CMCT/MARK/MTeam/FRDS/PTHome/beAst | — |
| 标签 | 禁转/首发/DIY/国语/中字/HDR | — |
| 音频编码 | 无 | 有 |

### 2.2 字段映射注意事项

- GTK 的分类ID体系与多数 NexusPHP 站点一致（401-412 范围）
- 媒介值分配不标准：UHD=10, WEB-DL=11（多数站点 UHD 和 WEB-DL 的值不同）
- 编码字段将 H.264 和 x264 合并为 "H.264"（值1），没有区分原盘和压制编码
- 制作组列表偏向国内老牌组（HDS, CHD, MySiLU, WiKi），转种时常需选 "Other"(5)
- 质量字段名含模式后缀 `[4]`，需要动态拼接字段名

---

## 三、站点适配器配置参考

```yaml
site:
  id: "gtk"
  name: "PT GTK"
  url: "https://pt.gtkpw.xyz"
  framework: "nexusphp"
  upload_url: "upload.php"
  upload_action: "takeupload.php"

  mappings:
    type:
      "电影": 401
      "剧集": 402
      "综艺": 403
      "纪录": 404
      "动漫": 405
      "MV": 406
      "体育": 407
      "高清音频": 408
      "其他": 409
      "书籍": 410
      "音乐": 411
      "学习": 412

    medium_sel:
      "Blu-ray": 1
      "HD DVD": 2
      "Remux": 3
      "MiniBD": 4
      "HDTV": 5
      "DVDR": 6
      "Encode": 7
      "CD": 8
      "Track": 9
      "UHD": 10
      "WEB-DL": 11

    codec_sel:
      "H264": 1
      "VC-1": 2
      "Xvid": 3
      "MPEG-2": 4
      "Other": 5
      "H265": 6
      "AV1": 7
      "VP9": 8

    standard_sel:
      "1080p": 1
      "1080i": 2
      "720p": 3
      "SD": 4
      "4K": 5
      "8K": 6

    team_sel:
      "HDS": 1
      "CHD": 2
      "MySiLU": 3
      "WiKi": 4
      "Other": 5
      "CMCT": 6
      "MARK": 7
      "MTeam": 8
      "FRDS": 9
      "PTHome": 10
      "beAst": 11

    tags:
      "禁转": 1
      "首发": 2
      "DIY": 4
      "国语": 5
      "中字": 6
      "HDR": 7

  field_names:
    suffix: "[4]"
    medium: "medium_sel[4]"
    codec: "codec_sel[4]"
    standard: "standard_sel[4]"
    team: "team_sel[4]"
    tags: "tags[4][]"
    anonymous: "uplver"

  missing_fields:
    - "audiocodec_sel"
    - "processing_sel"
```

---

## 四、发布流水线注意事项

### 4.1 字段名动态拼接

GTK 的质量字段带模式后缀 `[4]`（对应 type 的 `data-mode='4'`）。发布时需拼接：

```
medium_sel[4], codec_sel[4], standard_sel[4], team_sel[4], tags[4][]
```

如果 GTK 未来新增分类模式（如音乐分类用 `data-mode='5'`），字段名会变为 `medium_sel[5]` 等。适配器需根据 `type` 字段的 `data-mode` 属性动态确定后缀。

### 4.2 制作组映射策略

GTK 的制作组列表偏向国内老牌压制组。转种时：
- 源站制作组在列表中 → 直接映射
- 源站制作组不在列表中 → 使用 "Other"(5)
- 制作组字段无法自动判断时默认选 "Other"

### 4.3 缺失字段处理

GTK 无 `audiocodec_sel` 和 `processing_sel`：
- 适配器在构建表单时应跳过这两个字段
- 不应因这两个字段缺失而报错

### 4.4 PT-Gen 集成

GTK 发布页内置了 PT-Gen 获取按钮（`btn-get-pt-gen`），支持从 IMDb/Douban 链接自动填充简介。适配器可直接利用此功能或自行调用 PT-Gen API。

---

*分析时间：2026-04-16*
*数据来源：https://pt.gtkpw.xyz/upload.php 发布页面 HTML 分析*
