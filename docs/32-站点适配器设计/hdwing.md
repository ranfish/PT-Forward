# HDWinG 瓷器适配器设计

> 站点：瓷器 HDWinG · hdwing.org · tracker.hdwing.org
> 状态：源站接入完成（§59.510·2026-11-10·v0.0.1424·五链端到端验证毕）
> 摸底账号：ranfish_hdc（UID 268618 · UltimateUser 等级 · CC cookie 有效）

## 1. 站点实态档案

### 1.1 框架与形态

- **NexusPHP 定制版**（xajh @ 2015.09.18 风格改版 · 页脚 "Powered by NexusPHP"）
- 列表/详情/搜索骨架=标准 NP，但单元格结构有定制差异（见 §3）
- **Cloudflare Turnstile 闸门**（sitekey 在场 · appearance=interaction-only——仅 CF 挑战时交互；当前 cookie/IP 信誉正常零挑战）

### 1.2 下载链（关键特化）

**hash 签名链接独占，标准 id 形态禁用**：

```
列表/详情页提取: download.php?hash=MQo4MzgzMDMKMjY4NjE4.aRSFTOhrcUFQkQtO3b1fO8kJ2MCN54AEu9PAVLQPwZE
                 前段=base64("1\n{tid}\n{uid}") · 后段=站方签名
标准形态 download.php?id={tid}（不带/带 passkey 均 404）——站方禁用
GET hash 链接 → 200 application/x-bittorrent ✓
```

**tracker credential 机制**（种子个性化）：

```
announce = https://tracker.hdwing.org/announce?credential={hex64}&uid={uid}
```

- per-user per-torrent 专用密钥（详情页明示：泄漏可单独重置该种子密钥）
- **每次下载实时取**——不可缓存种子文件、不可转交他人（credential 绑定账号）
- tracker 地址与用户提供的 tracker.hdwing.org 吻合 ✓

### 1.3 列表页结构（torrents.php）

标准参数工作（search/search_area/search_mode · Wimbledon 搜索 105 命中）。
单页 100 行。行结构：

```html
<tr>
  <td class="t_pin">置顶标记</td>
  <td><h3>标题</h3><h4>副标题</h4>[下载进度条(自己做种中)]</td>
  <td class="discount"><span class="sp_state_placeholder" id="{tid}"></span></td>   ← 优惠 JS 动态填充
  <td class="share_rule"><img src='/pic/share_rule_N.gif' title='规则文本'/></td>  ← 转载规则独立列
  <td class="act">hash 签名下载链接 + 收藏 + 下载筐</td>
</tr>
```

**完整列结构（用户纠正+用户样本 /tmp/hdw.html 实证）**：普通行 15 类
td 全在——t_cat/t_name(嵌套)/t_pin/discount/share_rule/act/t_comment/
t_time/t_size/t_torrents/t_leech/t_completed/t_uploader。**置顶行是精简
形态（无后段列）——首行采样曾误判"无 size 列"（摸底错误已修正）**。
- size：`<td class="t_size">799.12&nbsp;GB</td>`（&nbsp; 分隔）
- seeders：`dllist=1#seeders">N</a>`（0 值=纯数字无链接）
- 发布时间：`<span title="2026-11-10 09:13:51">5时17分</span>`（绝对时间在 title）

### 1.4 转载规则（禁转必检 §59.369 附·站内实测全形态）

`share_rule` 独立列（列表行 img title 直读·**详情页无此标记**）：

| 编号 | title 文本 | 实测样本 | 语义 |
|------|-----------|---------|------|
| share_rule_1 | 禁止转载 | tid 837787/438018 | **硬禁转** |
| share_rule_2 | 未经允许不得转载 | tid 822791（副标"未允禁转"） | **硬禁转**（首版漏判已修） |
| share_rule_3 | 欢迎转载 | 32% | 可转 |
| share_rule_255 | 其他情况(参考副标题及种子简介的标注语) | 66% | **软规则——副标题+简介标注语扫描** |

**255 态标注语四形态正例**（search=禁转 109 命中实测·"禁转"子串全覆盖）：

- 副标「自购**禁转**」（837787·share_rule_1 行）
- 副标「**未允禁转**」（822791·share_rule_2 行）
- 副标「发布者授权转载，**禁转**」（601877·255 行）
- 副标「**【禁转】**有声小说…」（438018·share_rule_1 行）

**反例**：255 行无标注语=默认可转（838381 大醉侠·副标纯内容描述+简介豆瓣式
零禁转词——"其他情况"≠禁转·扫描判定方向正确）。

**站规明文**（rules.php）：
- 「未经许可，转发 HDWinG 独占资源，第一次严重警告，并限时删除转发资源，
  如拒绝整改账号将立刻被封禁」——HDWinG 独占资源（官组种）禁转
- 「个人会员未经许可，一律不得使用 HDChina/HDCTV/LU9998/iHD 等相关官方
  小组后缀」——官组后缀禁用清单（groups 配置依据）
- DUPE 规则：官方组作品不受 dupe 约束·优先保留

### 1.5 优惠状态（JS 动态）

列表页静态 HTML 零优惠信息（占位 span）。批量端点：

```
POST ajax_promotion.php
body: ids[]={tid}&ids[]={tid}...&csrf={csrf}
csrf: 页面 <meta name="x-csrf" content="...">（GET 列表页时提取）
头: X-Requested-With: XMLHttpRequest · Referer

响应: {"status":200,"message":{"{tid}":{"sp_state":"<img class=\"pro_free\" ... title=\"免费\"/>","timeout":""},...}}
```

抽查三颗全部 pro_free（免费）。timeout 字段=限时优惠截止。

### 1.6 官组与源站资格

getrss.php 制作组下拉 40+ 组：HDWinG（自组）/HDWTV/HDChina/HDCTV/iHD/KiSHD/
OpenMV/HDC/DIY/KHQ/ExREN/JOMA/Anonymous/CrsS/DON/ESiR/WiKi/beAst/CMCT/NGB/U2...
**HDWinG 自组在场 → 源站资格 ✓**

### 1.7 RSS 通道（全通·2026-11-10 实测）

- **passkey 独立凭证机制**：tracker credential（做种）与 passkey（RSS）是两套
- 链接生成：POST getrss.php（csrf+筛选参数）→ 输出
  `https://hdwing.org/torrentrss.php?rows=20&search=X&passkey=***`
- 裸访/错 key → 15B "invalid passkey"
- **RSS 输出**（180KB XML 实测）：
  - `<guid>`=**40 位 hex infohash 真身**（无需侧载·与可乐 guid=URL 需侧载不同）
  - `<link>`=details.php?id=N 详情页（**非直接下载链接**——下载需走
    详情页 hash 签名链提取，adapter DownloadTorrent 特化覆盖）
  - title/pubDate 标准形态
- getrss.php 配置页=xajh 定制多维筛选（类型/媒介/编码/音频/分辨率/制作组/
  收藏/下载筐/关键字和或精确/条数）

## 2. 引擎兼容性验证（离线驱动·纯函数解析器）

### 2.1 列表页 parseNexusPHPBrowseLimit

| 项 | 结果 |
|----|------|
| 行数 | 100/100 ✓（与 class="download" 图标数吻合） |
| tid/标题/副标题 | ✓ 干净 |
| size | **0 ✗**——`reNexusBrowseSize` class 分支无 `t_size`（加分支后 100/100 全中·用户样本实测） |
| seeders/leechers | 锚点 `#seeders/#leechers` 兼容 ✓（0 值纯数字形态 miss=可接受默认 0） |

### 2.2 详情页 legacyExtractDetail（生产走 extract 引擎·此处粗验）

| 项 | 结果 |
|----|------|
| size | 799.12GB ✓ 精确 |
| category/codec/source | 体育节目(Sports) / x265/HEVC / HDTV ✓ |
| poster | ✓（简介首图） |
| resolution | 空 ✗（标签是"格式：4K"非标准"分辨率"） |
| team/audiocodec | 标签边界污染（"HDCTV 行为 下载种子..."粘连） |
| upload_time | 零值 ✗ |

## 3. 引擎适配（§59.510 已实施·端到端五链验证毕）

| 缺陷 | 根因 | 修复 |
|------|------|------|
| 列表 size=0 | `reNexusBrowseSize` class 分支无 `t_size` | 加分支（用户样本 100/100） |
| 详情 size 错 57.5TB | 引擎四结构 miss→fallback 正则错抓用户上传量 | extractSize 加 `<i><b>大小：</b>值</i>` 分支 |
| 详情技术字段全空 | li 聚合结构（非 td/dt/th） | fillBasicInfoFieldsWithCode 加模式 1.5（li） |
| "格式"标签歧义 | hdwing=分辨率 vs SSD/碟粉=媒介（default） | matchFieldWithLabels site-key 优先层 |
| title 污染/team 粘连 | legacy 正则边界 | 生产 engine 路径根治（canonical 全对） |

canonical 实测（tid 838303）：category.sports / video.h265 / medium.hdtv /
team.hdctv / resolution.r2160p（格式 4K 映射）。

## 4. 接入实施记录（§59.510·v0.0.1424）

- 双 JSON 条目（sites.json：signed_hash/is_source=true/groups 7 官组/
  tracker.hdwing.org；supported_sites.json：verified）
- DownloadMode="signed_hash"（新值）+resolveHashDownloadURL
- enrichHdwingTransferFlags（share_rule 四态·255 软规则扫描域限定）
- detectDiscountHdwing（ajax_promotion csrf 两步）
- hdwing.org.json source_keys+standard_keys 全值集
- RSS 零改动（guid=infohash+enclosure=hash 直链）
- 测试锚七枚·五链端到端 29 实测绿

## 5. 发布适配摸底（2026-11-10·待实施）

### 5.1 表单契约（upload.php 实测·55KB）

```
POST takeupload.php · multipart/form-data · 零 CSRF token（老 NP·无 hidden _token）
Tracker: https://tracker.hdwing.org/announce
```

| 字段 | 类型 | 说明 |
|------|------|------|
| file | file 必填* | 种子文件 |
| type | select 必填* | 27 类（双轴混维：老编号+401+ 体系） |
| share_rule | select | **转载规则声明**（发布者自选：1 禁止/2 未经允许/3 欢迎/255 其他）——与列表采集同体系闭环 |
| standard_sel | select 必填* | 格式（分辨率）9 项 |
| medium_sel | select 必填* | 媒介 14 项 |
| codec_sel | select 必填* | 编码 9 项 |
| audiocodec_sel | select 必填* | 音频编码 16 项 |
| name | text 必填* | 标题 |
| small_descr | text | 副标题（≤30 字·建议中英对译） |
| cover | text | 海报外链 URL 直填（非 BBCode） |
| url | text | IMDb 链接 |
| dburl | text | 豆瓣链接 |
| nfo | file | NFO/LOG 文件（.nfo/.log·多 LOG 以 8+ 连续 * 分隔——音乐 LOG 体系·影视链不传） |
| descr | textarea 必填* | 简介（BBCode 编辑器） |
| team_sel | select | 制作组 22 项 |
| is3d | checkbox | 3D 标记 |
| status | checkbox | **候选开关**（value=pending·勾=候选·不勾=直发） |
| uplver | checkbox | 匿名（value=yes） |
| board | select | 版块：1 普通区/2 HDCTV/3 HDChina（官组通道） |
| refid+uploadref | text+submit | 引用发布（details.php?id= 引用他人种） |

### 5.2 值集全表（实时·与 getrss 配置页一致）

- **type 27 项**：0 请选/20 原盘(Full BD)/17 电影1080p/16 电影1080i/9 电影720p/
  13 欧美剧/25 中港台剧集/26 韩剧/24 日剧/21 欧美剧集包/22 中港台剧集包/
  23 日韩剧集包/27 iPad视频/5 纪录片/15 体育节目/14 动画片/401 综艺/
  402 演唱会/406 MV/408 音乐/19 补充音轨/405 戏剧/404 书籍/409 其他/
  410 4K UltraHD/411 旅游/412 饮食
  ⚠ type 混维（20 原盘/410 4K 是媒介分辨率语义混入类型轴）——映射需
  站内样本验证惯例（实施刀 DryRun 前抽样定案）
- **share_rule**：1 禁止转载/2 未经允许不得转载/3 欢迎转载/255 其他情况
- **standard_sel**：19=8K/17=4K/18=2K/11=1080P/12=1080I/13=720P/15=SD/10=其它
- **medium_sel**：11 Blu-ray/12 HD DVD/6 Remux/19 Blu-spec/2 MiniBD/5 Encode/
  13 HDTV/21 WEB-DL/14 DVD/4 DVDR/10 DAT/1 CD/15 Other
- **codec_sel**：1 H.264/AVC/2 VC-1/4 MPEG-2/6 x264/10 x265/HEVC/3 Xvid/11 AVS/5 Other
  ⚠ H.264 与 x264 分立（§59.380 压制联动站·H/X 四分立同族）
- **audiocodec_sel**：14 DTS X/15 Atmos/12 DTS-HDMA/13 True-HD/11 LPCM/3 DTS/
  8 AC3/9 WAVE/1 FLAC/2 APE/6 AAC/5 OGG/10 TTA/4 MP2/3/7 Other
- **team_sel 22 项**：iHD/KiSHD/OpenMV/HDC/DIY/KHQ/ExREN/JOMA/Anonymous/CrsS/
  Ebp/DON/ESiR/TrollHD/WiKi/beAst/CMCT/NGB/LU9998/TAiCHi/U2/其他(5)
  ⚠ 无 HDWinG/HDWTV 选项（主官组走 board 版块）；站规明文个人会员未经
  许可不得使用 HDChina/HDCTV/LU9998/iHD 等官组后缀——**转载恒"其他(Other)"**
  （乌托邦 team 恒 Other 零冒认同款定案预期）

### 5.3 发布规则（站规已摸）

- 资格：UltimateUser 免候选（status 不勾=直发）；金牌经纪以下需候选（10 正票）
- 合格资源：海报+IMDb 链接+影片简介+编码信息+截图（≥3 张 png 源截图）
- 合集：电影类仅发行商官方合集；动漫整季（未完结禁 <50 集小合集）；剧集整季；
  音乐最小单位专辑
- DUPE：官方组不受 dupe 约束优先保留；后上传重复版本无提醒直接删除
- 官组后缀禁用清单（HDChina/HDCTV/LU9998/iHD 等）——转载不得使用
- 发种 2X 上传（不受 SeedBox 规则限制）

### 5.4 四点定案（§59.511·2026-11-10 用户讨论式定案）

1. **share_rule 恒 3「欢迎转载」**——权利链定案：R7 保证源头开放+
   转载者无权禁止（1/2 越权）+255 自锁规避（enrich 扫描域已收窄
   Subtitle-only·任何值下自锁不存在）；与「互珍互重禁转PTT」声明
   两层各表（文本=礼仪立场·结构化=权限事实）
2. **type 混维矩阵**：4K 大桶（r2160p/r4320p→410·按我们标签消费）→
   电影媒介分桶（原盘 20/Remux·压制 17/1080i 16/720p 9）→剧集产区
   轴（PTGen Region·多季包桶·缺失兜底 13）→直映射类通用链
3. **is3d**=Stereo3D 在场勾 ｜ **board** 恒 1 普通区
4. **team_sel** 能对应对应（§59.273 teamMapping 既有语义零改动）+
   miss→Other；值集排除官组三槽（HDC/LU9998/iHD）双保险

### 5.5 官组与规则⑥（§59.511）

- 瓷器官组 8 组（用户确认+站规点名）：HDChina/HDCTV/HDWinG/HDWTV/
  OpenMV/HDC/LU9998/iHD（KiSHD 站规未点名=非官组已修正移除）
- **规则⑥ checkOfficialGroupBackflow（preflightNoTransfer 第⑥项·
  全站通用）**：种子组→mappings(is_official=1) 归属站==目标站→拦
  （官组种回流=DUPE+站规双风险·251 组自动生效）

### 5.6 实施记录（v0.0.1426）

migration157（form_config 七域+官组修正+is_target 开闸）+executor
三挂载（hdwingCategoryValue 第十一站/hdwingPublishInject/preflight⑥）
+ enrich 扫描域修正（Subtitle-only·用户回摸纠偏：系统禁转判定不读
简介声明——extractFlags 两道防线实证）+测试锚三枚+49 包绿。
成功判定形态待首颗实测（标准 NP 302 预期）。
