# 深度 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 深度（Depth Studio）|
| 站点地址 | https://dstudio.me |
| 站点框架 | NexusPHP（新版 details-* 前端组件）|
| 认证方式 | Cookie（c_secure_pass 加密 cookie 体系）|
| 角色 | **仅源站**（全站官种，不能发布）|
| 官方组 | DS、DStudio、DepWeb、DSAudio |
| Tracker | `https://dstudio.me/announce.php`（与主域同域）|
| 下载模板 | `download.php?id={id}&passkey={passkey}` |
| L0 API | 支持 pieces_hash（HTTP 401 验证）|
| **网络要求** | **直连极慢（25s/32KB 截断）——必须配置代理** |

---

## 一、站点特点

- 全站官种（DStudio 压制的蓝光原盘为主——详情页 BDInfo 是发布链刚需）
- 详情页使用 NP 新版前端组件（`details-*` class 命名空间），与标准 NP 的 `kdescr`/`dt-dd` 结构差异大
- 促销标记在标题行独立 span（`details-title-status--promotion`）

## 二、页面结构与提取映射（§59.366 实证）

### 详情页结构（与标准 NP 的差异点）

| 区块 | 深度站结构 | 标准 NP | 提取适配 |
|------|-----------|---------|---------|
| 标题 | `<h1 class="details-title">`（干净）+ 促销 span（兄弟节点）| `<h1 id="top">` | `#top` 在外层 header（容器）——extractTitle 容器守卫跳过落 h1 |
| 副标题 | `<p class="details-subtitle">`（中文名/译名/类型/主演）| kdescr 区/独立元素 | engine 默认锚兼容 ✓ |
| 简介 | 非标准区（无 kdescr）| `<div class="kdescr">` | engine 另一锚命中 ✓ |
| **MI/BDInfo** | **结构化字段区**：rowhead "MediaInfo/BDInfo" 的 rowfollow 内 `nexus-media-info-raw` div 的 `<pre>`（锚 `DISC INFO:`）| kdescr 简介区 | fillMediaFromRawBlock fallback（§59.366 新增）|
| 大小 | `<span class="details-meta__label">大小：</span><strong class="details-meta__value">46.26 GB</strong>` | dt/dd 或 th/td | findSizeInRows 第三分支（§59.366 新增）|
| 类型 | details-meta 结构"电影-Movies" | dt/dd | standardizeFieldValue → category.movie ✓ |

### 搜索页

标准 NP `torrents.php` 结构——通用 adapter 直接兼容（50 行验证）。

## 三、深度适配三缺口修复（§59.366，v0.0.1207）

1. **Title 容器污染**：`#top` 在外层 header → `.Text()` 拼接标题+促销+副题——extractTitle 加标题标签守卫（非 h1/h2/h3 跳过继续回退链）
2. **MI/BDInfo 字段区提取**：fillMediaFromRawBlock——Description 双空时扫 `nexus-media-info-raw` 的 `<pre>`，`DISC INFO:`/`PLAYLIST REPORT:` 锚判型填 BDInfo（1619B 实证）
3. **Size details-meta 分支**：findSizeInRows 加 span label→strong value 分支（"46.26 GB"→49671296778 精确字节实证）

## 四、运营注意事项

- **代理是硬需求**（直连 25s 截断不可用）——各环境站点配置需设 proxy_url
- CookieCloud 域匹配：dstudio.me
- 全站官种——发布链无该站表单（is_target=false），仅做源站转发

## 五、禁转规则（§59.369 附二，rules.php 权威）

### 规则原文

> 带有限转标签的种子，发布72小时之内严禁将其转载至任何其他PT站点或公开网络。
> 限转解除后，此类资源仍然禁转至 PTT、Rousi、HDhome。

**站名映射**：PTT=时间、Rousi=肉丝、HDhome=家园

### 两态语义模型

| 态 | 规则 | 系统实现 |
|---|---|---|
| 发布 72h 内 | 严禁转载至任何站 | NoTransferUntil = PublishedAt+72h（pipeline 窗内拦）|
| 72h 后（限转解除） | 永久禁转至 时间/肉丝/家园 | siteDirectedNoTransferTargets 名单（pipeline 目标命中拦）|

### 标记载体与采集

- **标签**：结构化字段区 rowhead"标签"→ rowfollow 内
  `<span class="torrent-content-tag">官方/限转/原生/国语/中字</span>`
- **采集链**：extractTags span 分支（§59.369 附二）→ extractFlags
  tags 并入 → "限转"关键词命中 → Flags=["限转"]
- **发布时间**：`发布于<span title='绝对时间'>相对文本</span>`——
  title 属性即精确时间戳（fillPublishedAt 提取）
- **列表页注意**：tag_id=1 筛选器的"限转"是站方标签体系入口，
  非种子行内标记（§59.167 筛选器陷阱同款区分）

### 实证（tid=5073 后会无期）

```
probe 生产同栈：Tags=[原生 官方 限转 国语 中字] | Flags=[限转]
             | PublishedAt=2026-10-29 19:35（title 绝对时间）
消费三态锚：名单内拦（时间/肉丝/家园 无论窗）/ 窗内拦（名单外 72h 内）
          / 窗外放行（名单外 72h 后）/ until 缺失放行（名单已拦+缺失
          不无限保守=老种防永久误拦）
```
