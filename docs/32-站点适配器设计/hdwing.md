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
