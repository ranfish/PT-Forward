# 天枢 站点适配器设计

## 站点信息

- **站名**：天枢 | **域名**：dubhe.site
- **框架**：NexusPHP（字段后缀 [4]）
- **建档**：2026-11-02（**docs/32 无文档站第五例**——咖啡/海的宝/我的PT 路径：实时 HTML 为源当天建档）
- **官组**：DubheWeb

## 一、发布页面表单（upload.php 实时 102.5KB 2026-11-02）

**提交地址**：`takeupload.php`（POST multipart/form-data）

### 1.1 字段

| 字段名 | 类型 | 说明 |
|--------|------|------|
| `file` | file | 种子文件 |
| `name` / `small_descr` | text | 标题/副标题 |
| `url` | text | IMDb 链接（data-pt-gen="url"） |
| `nfo` | file | NFO 文件 |
| `price` | select | 促销（不配） |
| `descr` | textarea | 简介 |
| `technical_info` | textarea | MediaInfo/BDInfo |
| `uplver` | checkbox | 匿名发布（value="yes"） |

无 pt_gen；无 audiocodec 槽（第六站）。

### 1.2 type（11 项）

401 Movies/402 TV Series/403 TV Shows/404 Documentaries/405 Animations/406 Music Videos/407 Sports/408 HQ Audio/409 Misc/410 Books/411 photo

### 1.3 medium_sel[4]（10 项，值 1-10 乱序排列）

1 Blu-ray/2 HD DVD/3 Remux/4 MiniBD/5 HDTV/6 DVDR/7 Encode/8 CD/9 Track/10 WEB-DL——无 UHD/DVD/Other 项

### 1.4 codec_sel[4]（10 项，H/X 四分立）

11 H.265/HEVC、10 x.265（带点）、13 H.264/AVC、12 x264、7 MPEG-4、6 MPEG-2、2 VC-1、9 AV1、8 Xvid、5 Other

### 1.5 standard_sel[4]（5 项，p/i 合并形态）

5 2160p/2160i、1 1080p/1080i、3 720p、4 SD、7 Other/其他

### 1.6 team_sel[4]（6 项）

1 HDS/2 CHD/3 MySiLU/4 WiKi/5 Other/6 DubheWeb（站方官组）

### 1.7 tags[4][]（6 项极简）

1 禁转、2 首发、4 DIY、5 国语、6 中字、7 HDR

## 二、发布适配记录（v0.0.1297 §59.441）

- **form_config**：migration124 分发（六槽 48 项）。实时=资产全槽
  一致（零漂移站）。
- **type 11**：**411 photo=[category.photo] ext 活键历史首配**（罕见
  摄影类目）+410 Books=[ebook]+406=[mv,music_video,concert] 三键
  承接+408/404 双键。
- **medium 10 乱序**：Blu-ray(1)=[bluray,uhd_bluray] 双键归并（无
  UHD 项奥申同款）+**DVDR(6)=[dvd,dvdr] 双键宽语义（无 DVD 项——
  我堡单项宽语义同款）**+Remux/WEB-DL 双键+MiniBD/HD DVD/HDTV/
  Encode/CD/Track；无 Other 项 miss 落空预览人工。
- **codec 10 H/X 四分立压制联动第九站（§59.380）**：x.265(10)
  带点 label-only（联动通道——三月带点增强同款）+x264(12)
  label-only 联动通道+H.265(11)/H.264(13) 配键+MPEG-4/MPEG-2/
  VC-1/AV1/Xvid/Other 全配。
- standard 5 p/i 合并（咖啡同款）：2160p/i(5)=[r2160p] 单键
  （r2160i 无 ext 键 miss 落空——织梦 r720i 同款）+1080p/i(1)=
  [r1080p,r1080i] 双键+720p/SD/Other。
- team 6：HDS/CHD/MySiLU/WiKi 外站 label-only+Other(5)=[team.
  other] 兜底+DubheWeb(6) 站方官组 label-only。
- tags 6 极简：禁转/首发绝不配（财神雷区）+DIY/国语/中字/HDR
  四键泛家族 四键配。
- 字段：url（IMDb）+technical_info+uplver+anonymous:true；nfo
  不配（13城先例）；price 促销不配；无 pt_gen。
- **验证**：裸对全槽一致+formdiff 0 条（一次归零）+DryRun 三形态
  （① WEB-DL 剧集→402+10+13+1+tags=[国语,中字,HDR] ✓/② UHD
  原盘→medium=1 双键归并+codec=11+standard=5 ✓/③ MI 压制→
  medium=7 Encode+**codec=10 x.265 带点联动通道命中实证** ✓）+
  48 包。
- 站规人工把关：上传者资格+H&R+Dupe 规则（无 wiki 文档实时未
  采集——首颗发布前人工核验站方规则页）。
- 遗留：首颗真实发布。
