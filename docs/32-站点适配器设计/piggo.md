# 猪猪（Piggo）站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 猪猪（Piggo） |
| 站点地址 | https://piggo.me |
| 站点框架 | NexusPHP（深度定制） |
| 口号 | 童话里做英雄！ |
| 特殊功能 | 儿童专区（IMDB 分级过滤）、3D专题、独立 Tracker 子域名、画质字段（DV/HDR10+/HDR10/SDR）、Cloudflare 防护 |
| 规则页面 | rules.php |
| Tracker URL | https://tracker.piggo.me/announce.php |
| 建站时间 | 2022 年 |
| **转发定位** | **仅发布站**（禁止做源站） |
| **转发范围** | **仅限儿童动画资源** |
| **官组资源** | **全站官种禁转，禁止转出官组资源** |

---

## 一、发布页面表单字段分析（儿童专区模式）

**注意**: 当前抓取到的是**儿童专区** upload 页面（`data-mode='5'`），主种子区（`data-mode='4'`）的表单未抓取。以下字段均带 `[5]` 后缀。

**提交地址**: `takeupload.php`（POST multipart/form-data）

### 1.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `file` | file | ✓ | 种子文件 |
| `name` | text | - | 标题（英文0day命名法） |
| `small_descr` | text | - | 副标题 |
| `url` | text | - | IMDb 链接（带"获取简介"按钮） |
| `pt_gen` | text | - | 豆瓣链接（标签显示"豆瓣链接"，仅支持 douban） |
| `descr` | textarea | ✓ | 简介（BBCode，20行） |
| `technical_info` | textarea | - | MediaInfo/BDInfo |
| `screenshots[]` | text(URL) | - | 截图 URL（JS 动态添加行，支持 jpg/jpeg/png/gif/webp） |

**注意**: 无 `uplver`（匿名发布）字段。无 `nfo` 字段。

### 1.2 类型字段（`type`，data-mode=5，儿童专区）— 6个

| 值 | 显示名称 |
|----|----------|
| 907 | 纪录片 |
| 910 | 读书绘本 |
| 911 | 音乐 |
| 905 | 有声读物 |
| 909 | 儿童电影 |
| 908 | 儿童剧集 |

**注意**: 这是儿童专区分类。主种子区分类（Movies/TV Series 等，data-mode=4）**未抓取**，需补充。

### 1.3 来源字段（`source_sel[5]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 1 | Blu-ray |
| 3 | DVD |
| 5 | TV |
| 6 | Other |
| 7 | WEB-DL |

### 1.4 媒介字段（`medium_sel[5]`）— 5个

| 值 | 显示名称 |
|----|----------|
| 3 | Remux |
| 5 | DIY |
| 7 | Encode |
| 8 | Other |
| 11 | Untouched |

**注意**: WEB-DL/TV 类资源的媒介应选择 **Untouched**（而非原盘专用的含义）。DIY 适用于原盘及 Encode/Remux 的自定义制作。

### 1.5 编码字段（`codec_sel[5]`）— 3个

| 值 | 显示名称 |
|----|----------|
| 1 | H.264/X.264 |
| 5 | Other |
| 6 | H.265/X.265 |

**注意**: H.264 和 x264 合并为一个选项（"H.264/X.264"），不区分标准名与编码器名。只有3个选项（较其他站点少很多）。

### 1.6 音频编码（`audiocodec_sel[5]`）— 10个

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS |
| 4 | MP3 |
| 6 | AAC |
| 7 | Other |
| 8 | AC-3 |
| 9 | DTS-HD MA |
| 10 | TrueHD |
| 11 | LPCM |

**注意**: DTS 统一包含 DTS/DTS-ES/DTS-HiRes。DD/AC3 包含 DD/AC3/DDP/DDP Atmos。

### 1.7 分辨率（`standard_sel[5]`）— 4个

| 值 | 显示名称 |
|----|----------|
| 1 | 1080p/i |
| 3 | 720p |
| 5 | 4K |
| 6 | other |

### 1.8 制作组（`team_sel[5]`）— 4个官组

| 值 | 显示名称 |
|----|----------|
| 5 | Other |
| 7 | PigoHD |
| 8 | PigoWeb |
| 9 | PigoNF |
| 10 | PigoAD |

**注意**: 非本站小组成员**禁止勾选官方标签**。

### 1.9 画质字段（`processing_sel[5]`）— 4个

| 值 | 显示名称 |
|----|----------|
| 1 | SDR |
| 2 | HDR10 |
| 3 | HDR10+ |
| 4 | DV（杜比视界） |

**独特设计**: 独立的**画质**下拉字段，而非用标签代替。DV(4)/HDR10+(3)/HDR10(2)/SDR(1) 四级。

### 1.10 标签（`tags[5][]`）— 18个

| 值 | 显示名称 |
|----|----------|
| 1 | 禁转 |
| 5 | 国语 |
| 6 | 中字 |
| 7 | HDR 10 |
| 8 | 3D |
| 9 | Dolby Vision |
| 10 | 特效 |
| 11 | 合集 |
| 12 | Dolby Atmos |
| 13 | 完结 |
| 14 | 追更 |
| 15 | 怀旧计划 |
| 16 | 应求 |
| 17 | 独占 |
| 18 | 分集 |
| 20 | 自购 |
| 21 | 中英双语 |

**注意**: 
- **合集**(11) = 多季度合集包，**完结**(13) = 单季度合集包，**分集**(18) = 单集
- **中英双语**(21): 同时存在国语+英语音轨时必须勾选，**禁止勾选国语标签**
- **自购**(20): 需在简介内附购买凭证图片
- **应求**(16): 求种区应求发布
- **怀旧计划**(15)、**追更**(14)、**3D**(8) 为特色标签

### 1.11 自定义字段（`custom_fields[5][2]`）— 观看分级

**必填**字段，IMDB 观看分级：

| 值 | 说明 |
|----|------|
| G级 | TV-Y/TV-7/TV-G/G级，所有年龄均可观看 |
| PG级 | TV-PG/PG级，建议在父母陪伴下观看 |

---

## 二、儿童专区规则

**来源**: Forum topic 102（关于儿童专区资源范围(暂行）的公告）

1. 儿童区资源的总原则是**适合12岁（含）以下儿童观看**的资源
2. 有 IMDB 评级的，按 IMDB 评级执行：**G、PG、TV-Y、TV-Y7、TV-G、TV-PG** 六个等级可发布到儿童区
3. 无 IMDB 评级的，由发种者自行评估适合年龄段
4. 有 IMDB 评级的 TV 资源：TV-Y/TV-7/TV-G 统一选择 G 级，TV-PG 选择 PG 级
5. 举报错误分区奖励 **500 魔力/种**
6. 发布者可提出异议，争议通过社区投票决定

---

## 三、媒介/标签/编码选择释义

**来源**: Forum topic 183（视频媒介/标签/编码选择释义）

### 3.1 媒介类型详解

| 媒介 | 说明 |
|------|------|
| UHD Blu-ray | 4K蓝光原盘，仅破解/解除锁区，未做任何改动（Untouched） |
| UHD BluRay/DIY | 对4K蓝光原盘自定义操作（添加/删减音轨字幕等） |
| Blu-ray原盘 | 1080P/1080i蓝光原盘，未做任何改动 |
| BluRay/DIY | 对1080P蓝光原盘自定义操作，**miniBD 也归此类** |
| Remux | 对原盘正片/花絮重封装为单独 mkv/mp4，视频音轨质量等同原盘 |
| Encode | 对原始视频有损重编码（压制），标题可见 x264/x265/Rip |
| Web-DL | 从流媒体平台下载的资源（国内编码标 H264/H265，国外标 x264/x265） |
| HDTV | 特殊设备提取的数字源高清电视资源 |
| DVD | DVD原盘，最高分辨率 720p |

**原盘类**需具备完整蓝光结构文件夹或打包为 ISO。**Remux/Encode** 无蓝光结构，为单独 mkv/mp4 文件。

### 3.2 标签使用规则

| 标签 | 规则 |
|------|------|
| 原创 | 自己购买的原盘/DIY/Encode/Remux/流媒体下载等自己获取的资源 |
| 国语/粤语 | 正片只有一条混合语言音轨时，根据主要语言确定 |
| 官字组 | 仅适用于 Audies 官组资源或本站字幕组帮助制作的资源 |
| 中字 | 简体/繁体/简繁双语字幕 |
| Dolby Vision | 具备双视频轨道或 MEL/FEL 视频轨道的原盘/重封装 |
| HDR10 | 含 HDR10 标准（BT2020，HDR10 compatible） |
| HDR10+ | 含三星 HDR10+ 标准（BT2020，HDR10+ compatible），较罕见 |
| 禁转 | 禁止转载他站的资源 |
| 限转 | 限时禁止转载，标签消失前不可转载 |
| DIY | 仅适用于原盘资源的 DIY 制作 |
| 首发 | **仅适用于蓝光原盘的首次发布（全网内）** |
| 应求 | 求种区应求发布的资源 |
| 动画 | 非真人的动画/动漫类视频资源 |

### 3.3 音频编码选择

| 编码 | 包含范围 |
|------|----------|
| DTS-HD MA | 次世代无损音轨 |
| TrueHD | 次世代无损杜比音轨 |
| DTS-X | 次世代无损临境音轨 |
| TrueHD Atmos | 次世代无损杜比全景声 |
| DD/AC3 | DD/AC3/DDP/DDP Atmos |
| DTS | DTS/DTS-ES/DTS-HiRes |

### 3.4 用户补充规则（3楼 WhiteLycoris）

1. **WEB-DL/TV 资源**: 来源选 WEB-DL/TV，**媒介选 Untouched**
2. **中英双语标签**: 资源同时存在国语+英语音轨时，**必须勾选"中英双语"，禁止勾选"国语"**
3. **合集 vs 完结**: 合集 = 多季度合集包，完结 = 单季度合集包
4. **Encode/Remux 的 DIY 标签**: 如果对 Encode/Remux 做了自定义修改（如追加字幕），也需要添加 DIY 标签

---

## 四、资源发布新手教学

**来源**: Forum topic 394（资源发布新手教学[2023.08]）

### 4.1 命名规范

**单集资源**（电影、动漫、综艺等）：
- 标题：`英文译名.年份.分辨率.来源.视频编码.音频编码-发布组` 或用空格分隔
- 示例：`Drunken Master 1978 1080p WEB-DL H264 AAC-PigoWeb`
- 副标题：`中文译名/又名 类型 音轨/字幕`
- 示例：`醉拳 喜剧/动作 国语/中字`

**多集资源**（剧集、动漫、纪录片等）：
- 标题：`英文译名.季数集数.年份.分辨率.来源.视频编码.音频编码-发布组`
- 示例：`Archives S01E01-S01E05 2009 1080p WEB-DL H264 AAC-CMCTV`
- 副标题：`中文译名/又名 季数 集数 类型 音轨/字幕`
- **多集资源无论单季还是多季，均需加入季数标识 `S0X`**

### 4.2 类别选择

- 有 IMDB 评级且符合要求的儿童资源可直接勾选发布至儿童专区
- 其他资源请选择普通影视区对应类别分类

### 4.3 标签选择

- 根据资源属性勾选对应类别
- **勾选自购标签需在简介内以图片形式附上购买凭证**
- **非 Pigo 官方小组成员禁止勾选官方标签**

### 4.4 简介要求

1. 引用发布、转载来源、致谢等信息（**转载内站资源必须标明来源**）
2. 引用需特别说明的信息（不同编码/分辨率等）
3. 海报图片（推荐 PicGo 图床）
4. 资源详情、简介（站内工具一键生成）
5. 视频源码截图（单图 ≥ 3 张或拼图 1 张）

### 4.5 MediaInfo 要求

- 使用 PotPlayer 生成
- 多集**完结**资源仅需提供第一集 INFO
- 多集**未完结追更**资源需在每次更新时复制当前首集 INFO

### 4.6 上传者资格

- 任何人都能发布资源
- 有些用户需先在候选区提交候选
- 游戏类资源只有上传员及以上等级可自由上传

---

## 五、Upload 页面特别提示

### 5.1 分辨率限制

**本站不接受低于 1080p 的资源**（例外：纪录片 | 原盘 | 儿童区）。

### 5.2 大包零魔

**500G 以上大包将被赋予零魔 TAG**。

### 5.3 转载脚本

推荐使用 Greasyfork 脚本 `auto-feed` 进行转载。

---

## 六、Piggo 特殊注意事项

### 6.1 儿童专区体系

站点有独立的**儿童专区**，使用 IMDB 分级过滤（G/PG 六个等级），有独立分类（儿童电影/儿童剧集等），观看分级为必填字段。适配器转发儿童资源时需检查 IMDB 分级。

### 6.2 画质字段（processing_sel）

独立于标签的画质下拉字段：DV(4)/HDR10+(3)/HDR10(2)/SDR(1)。其他站点通常用标签代替。

### 6.3 WEB-DL/TV 媒介为 Untouched

WEB-DL/TV 来源的资源，媒介字段应选 **Untouched**（而非其他），这是猪猪独特的用法。

### 6.4 中英双语标签规则

资源同时有国语+英语音轨时，必须选**中英双语**(21)，**禁止选国语**(5)。

### 6.5 合集/完结/分集标签

- 合集(11) = 多季度合集包
- 完结(13) = 单季度合集包
- 分集(18) = 单集

### 6.6 官组保护

非 Pigo 官方小组成员**禁止勾选官方标签**。官组：PigoWeb(8)/PigoHD(7)/PigoNF(9)/PigoAD(10)。

### 6.7 无匿名发布

表单中无 `uplver` 字段。

### 6.8 豆瓣链接字段

PT-Gen 字段标签显示为"**豆瓣链接**"，仅支持 douban（不支持 imdb/bangumi/indienova 多源）。

### 6.9 独立 Tracker 子域名

Tracker URL 使用独立子域名 `https://tracker.piggo.me/announce.php`。

### 6.10 截图为 URL 输入

截图使用 URL 文本输入（`screenshots[]`），非文件上传。JS 动态添加行。

### 6.11 Cloudflare 防护

站点使用 Cloudflare 防护，CookieCloud 可能无法获取有效 `cf_clearance`，需手动提供 HTML。

---

## 七、转发定位（重要）

### 7.1 仅发布站，禁止做源站

- 猪猪为**儿童动画特色站**
- **全站官种都禁转**——禁止从猪猪转载官组资源到其他站点
- **只能转入儿童资源**——仅允许从其他站点转发符合条件的儿童动画资源到猪猪
- **禁止转出官组资源**——PigoWeb/PigoHD/PigoNF/PigoAD 的资源不可转载至其他站点
- 适配器需标记此站为 `publish_only: true, source: false`
- 转发到猪猪前必须检查：IMDB 分级为 G/PG 且适合 12 岁以下儿童

### 7.2 转发条件

```go
func canPublishToPiggo(req *PublishRequest) bool {
    if req.IMDBRating != "G" && req.IMDBRating != "PG" &&
       req.IMDBRating != "TV-Y" && req.IMDBRating != "TV-Y7" &&
       req.IMDBRating != "TV-G" && req.IMDBRating != "TV-PG" {
        return false
    }
    if !isChildrenContent(req) {
        return false
    }
    return true
}
```

---

## 八、待补充数据

以下数据**尚未采集**，需用户补充：

| 待补充项 | 说明 | 优先级 |
|----------|------|--------|
| **主种子区 upload 表单** | data-mode=4 的分类、标签、制作组等 | 中 |
| **rules.php** | 完整站点规则 | 低 |
| **促销规则** | 免费/下载优惠比例和规则 | 低 |
| **账号保留规则** | 不登录禁用/删除的时间限制 | 低 |

以下数据因站点定位为**仅发布站**，暂不需要补充：
- 主种子区详细字段映射（转发时不需从猪猪抓取元数据）
- 官组后缀格式（禁止转出，无需识别）

---

*数据来源: upload.php HTML（儿童专区模式）+ Forum 3帖（topicid=102/183/394）(2026-04-22)*
*文档创建: 2026-04-22*
*定位: 仅发布站（儿童动画资源），禁止做源站，全站官种禁转*

### 附二：搬家改版重新摸站（2026-11-16·§59.538）

**触发**：站方搬家（新旧站数据迁移中）+ 公告启用「存取令牌」体系。

**公告四要点**（用户提供原文）：
1. 数据搬家完成一部分（新站为 SPA·个别地方还空着属正常）
2. **禁止脚本直接请求 SPA 页面**（站方明令"小身板扛不住"·慎用油猴·刷猛限流封号）
3. **令牌 API 加急开发中·未上线**——未来「个人中心 → 存取令牌」生成令牌接新接口（用户已可生成 32 位 hex 令牌·接口等上线）
4. **Tracker 将停用 passkey 汇报·只接受下载令牌**——旧种需到详情页重下 .torrent 换种重做（个人中心有「仍用密钥汇报的种子」打包临时入口）

**渲染 DOM 分析**（用户提供 214KB 首页 outerHTML·`app-ready` 完整渲染态）：
- **架构**：Vite 构建 SPA（CSR 无 SSR/hydration·纯 XHR 取数）+ `/v1/` 命名空间 + **插件体系**（`/v1/web/plugin-assets/`：红包/tgbot/菠菜竞猜/iyuu/iyuu-find/抽奖/telegram-login/work-group——iyuu 插件在场=辅种生态）
- **路由**：`/torrents/<分类>`（电影/电视剧/少儿/动漫/综艺/纪录片/体育/音乐/图文 picbook/有声）·`/upload`·`/wiki`·`/userdetails/<uid>`——REST 风格无 .php
- **首页形态**：豆瓣风格卡片流（分类+分辨率徽章+标题+年份·genre+三列评分）——与传统 NP 表格完全断裂·4 月文档的全部表单字段分析已过时
- **自建服务**：`ptgen.piggo.me`（站方自营 PTGen）
- **个人数据面**：饲料（魔力）/免流/邀请/分享率/上传下载/活动上下——语义字段齐
- 旧 NP 会话 cookie 已消失（DB cookie 仅 `sl-session`×2+`SITE_TOTAL_ID`+cf_clearance）

**API 现状实证**（用户浏览器+我方多出口矩阵）：
- `api.piggo.me` → 404（子域已建·服务未就绪）；`piggo.me/api` → 跳回首页（无路由）——**印证令牌 API 未上线**
- 旧 NP 端点（torrents.php 等）随改版消亡

**CF 拦截矩阵**（自动化穷尽·全灭·方法论沉淀）：

| 出口 | piggo.me 主站 | api.piggo.me |
|------|--------------|--------------|
| 29 直连（curl/wget/http1.1） | 托管挑战 403 | CF 硬封 403 |
| 29 代理 | 托管挑战 403 | CF 硬封 403 |
| 243 | 403 封锁 | TCP 挂起 000 |
| DB cf_clearance+双 UA | 403（清障绑用户 IP+UA） | — |
| Playwright headless 直连/代理 | 挑战 30s 未过 | — |
| Playwright headful（xvfb+反检测+turnstile 点击） | **挑战未过**（"navigation"系挑战页自刷新·首版误判教训） | — |

→ **用户提供渲染 DOM 是唯一可靠摸站通道**（附·重启前置条件①兑现）。

**Tracker 改制资产影响**：243 库 10 颗·29 库 656 颗 piggo 种 100% 做种中——旧 passkey tracker 停用后汇报即断（公告"上传量白干"）。**换种=用户手动**（个人中心打包入口/详情页重下·SPA 操作不可自动化）。

**令牌 API 契约（2026-11-16 用户供文档 /tmp/piggo-api.txt·「实验室接口」）**：
- **认证**：`Authorization: Bearer <32位hex令牌>`（个人中心→存取令牌生成·等同账号密码）；根路径 `/v1/api`；`/ping` 免令牌探活
- **查询类用 JSON 正文**（POST·无参发 `{}`）；发种 `POST /torrents/publish`（字段 file/name/section_id 等）
- **端点与限流**：/ping(—)·/me(—)·/me/torrents(30/min 本人发布/做种/下载/完成/未完成)·/catalog(60/min 分区/分类/二级分类/标签)·/torrents(30/min 搜索)·/torrents/pieces-hash(30/min)·/torrents/publish(10/min)·/torrents/{id}(60/min 详情)·/torrents/{id}/download-link(20/min)
- **信封**：`{code:0, data:{}, msg:"", time:0.012}`·业务字段在 data·成功外层 code=0
- **公告门特殊形态**：未读必读公告时除 /ping 外全 HTTP 403 但**外层 code 仍 0**——判 `data.code=="notice_unread"`（含 notice_gate.id/title/body_md/read_wait·前台读完才能用·实验室不能标已读）
- **错误**：200 成功/401 令牌无效或通道关/403 无权或公告未读/404 不存在或不区分/422 参数无效（如有 tmdb_id 无 tmdb_kind）/429 超限退避/500 错误
- **可见范围**：仅当前账号默认可见分区分类种子；详情无权=404 不区分；搜索/详情带 page_url（前台 /detail/{id}）；无简介/无 passkey/无权限列表

**API 部署状态实证（2026-11-16·用户浏览器九点测绘交叉校验）**：
- **两层封锁同时存在**：①CF 边缘对非浏览器客户端挑战（curl/wget/curl-impersonate chrome116/edge101 + 三精确 UA + 新鲜 cf_clearance 重放全灭——API 路径规则比主站严·JA4 需精确匹配真浏览器）②**源站 nginx 无 /v1/api 路由**——真浏览器穿透 CF 后 GET /v1/api/* 落 SPA try_files fallback 壳（200+HTML）/POST 405（nginx 静态页特征）
- **定案：令牌 API 文档先行、接口未部署**（对应公告「加急开发中」）
- 主站 SPA 路由（/torrents/* 等）对**任何客户端开放**（裸 curl 200 壳）——CF 仅保护 index.php 等老路径与 /v1/api/*
- **上线判据**：GET `https://piggo.me/v1/api/ping` 返回 JSON（而非 HTML 壳）= API 已部署
- 预期：站方部署 API 时会同步放开 CF 对非浏览器客户端的限制（否则第三方程序全灭与 API 目的矛盾）——适配器建好后等阀门即通

**适配定案（2026-11-16）**：
- **等令牌 API 上线再适配**（获取链+发布链 API 直连·形态待站方放出；届时以 `/v1/` 命名空间为首要假设）
- **现在不爬 SPA**（站方明令+CF 技术双锁）·获取链猪猪本就 publish-only 不受影响
- DB 现状保持：is_source=0 ✓·is_target=0 ✓·form_config 空=未开闸 ✓（11-02 零代码落盘定案延续）·framework=nexusphp 标签过时但无消费面不预改
- 令牌（用户已生成）待 API 上线后配置 auth_type=apikey 通道

### 附：目标站适配停止记录（2026-11-02）

- **用户指令停止**（考古阶段终止——零代码落盘：无 migration/未开闸）。
- **停止动因：站方已改 SPA 前端框架**（实时铁证：upload.php 仅 3766B
  Vue/React 壳——`<div id="app-boot">` 加载中+`/assets/css/DgVlznA.css`
  Vite 构建哈希资源——表单完全由 JavaScript 动态渲染，view-source
  静态源码无任何表单控件；4 月资产+本文档的 NP 传统表单结构已全面
  过时）。
- **实时抓取受阻实录**：29 直连 IP+代理 IP 双被 CF WAF 硬拦（IP 级
  block"You have been blocked"——curl 直连/代理+完整头/Playwright
  真浏览器直连/代理+首页预热四手段穷尽全 403；cf_clearance 在
  CookieCloud cookie 中但绑签发 IP 无法跨 IP 使用——百川同款）。
- **未来重启前置条件**（若再次适配）：
  1. 用户提供**渲染后 DOM**（F12 Elements → html → Copy outerHTML
     ——非 view-source）——SPA 表单字段名/值集唯一可靠源；
  2. 核验提交端形态：`<form action="takeupload.php">` 传统提交保留
     与否——若 API 化（JSON REST）则 adapter 提交链需新适配（逆向
     API 协议），适配复杂度与 NP 站完全不同级；
  3. formdiff 解析器对 SPA 页面无效（解析静态 HTML）——需渲染 DOM
     作为输入。
- 站点定位不变（§7.1：仅发布站+禁做源站+全站官种禁转——is_source=
  false 已落库验证 ✓）。

**API 上线实测打通（2026-11-16 晚·用户供新版文档 366 行）**：
- **文档升级**：完整端点示例（发布 422 dedup 带 torrent_id、download-link 24h 签发、pieces-hash 批量 100 反查、/catalog taxonomies 二级分类树、/me can_upload 等真实形态）——权威端点域名=api.piggo.me。
- **部署确认**：浏览器地址栏 GET→405 应用层错误页（非 CF 非 SPA 壳）；浏览器同源 POST ping→`{"code":0,"data":{"ok":true}}`；/me→user 对象（id=22479 <用户名>·can_upload=true·has_passkey=false 印证 Tracker 换令牌）。
- **CF 破解=TLS 指纹**：curl-impersonate chrome116 POST→200（同 IP 同秒裸 curl→403 铁证）；**无需 cf_clearance**；主域 /v1/api/* 仍 nginx 静态 405（未代理）——生产走 api.piggo.me。
- **实测链**：ping ✓ / me ✓（Bearer+IPv6 出口 2408:...）/ torrents 搜索 ✓（q=Dune→60 条真实结果全字段：name/small_descr/cover/size/section/category/taxonomy/media{douban,imdb}/seeders/times_completed/added_at/page_url/tag_ids·分页 total_capped）。
- **生产架构方向**：Go 客户端需浏览器 TLS 指纹（bogdanfinn/tls-client 或 utls 方案）——CF 指纹墙唯一障碍已破；Bearer+POST JSON+统一信封+九端点全契约在手。

**九端点全量实测（2026-11-16 晚·chrome116 指纹+cURL 官方形态）**：

| 端点 | 实测 |
|------|------|
| /ping | ✓ {"ok":true} |
| /me | ✓ <用户名> id=22479·can_upload=true·has_passkey=false |
| /me/torrents | ✓ kind=seeding 真实做种（57178 金鱼公主 FRDS·uploaded/seed_seconds/ratio 个人统计字段） |
| /catalog | ✓ 12 分区（电影1/电视剧2/综艺3/少儿5/纪录6/动漫7/音乐8/图文9/有声10/体育11/其他12）+81 分类+7 taxonomies（地区17/来源5/媒介5/编码8/音频编码11/分辨率4/处理4/**制作组5**）+**22 tags（id=2 禁转**/1 官方/15 独占/9 合集/11 完结/3 国语/4 中字/5 HDR10/6 3D/17 零魔/18 自购/21 未审） |
| /torrents 搜索 | ✓ q+section_id+分页（Dune 60 条·字段全） |
| /torrents/{id} 详情 | ✓ 57277/57178 完整（慢响应 3.2s 偶现·管道截断假象教训） |
| /torrents/{id}/download-link | ✓ 24h 签发真 URL（piggo.me/v1/web/.../dl/base64·不含 passkey·announce 后作废） |
| /torrents/pieces-hash | ✓ **243 真指纹精确命中** id=56292（L0 辅种通道验证） |
| /torrents/publish | 不真发（铁律）——契约=cURL form 权威形态（category_ids=[3,5] JSON 字符串数组·非 NP 重复字段）+422 dedup 带 torrent_id |

- **禁转体系在 API 面**：tags id=2=禁转——tag_ids 含 2 即禁转种（§59.539 复检通用路径天然覆盖：catalog 翻译 tag_ids→名称「禁转」→extractFlags 命中）。
- 官方请求头三元组：Accept+Content-Type application/json+Bearer。
- 文档用户版 v3=625 行（含每端点 cURL 权威形态）。

**curl-impersonate 方案复验（2026-11-16·定案版）**：
- **tls-client 库全灭**：Go 内嵌指纹伪装全 Chrome profile（120/124/131/133/PSK 族）对猪猪 CF 皆 403——CF 检测已深入 Go 程序化 TLS 全族（H2 帧序层），只有真 BoringSSL 二进制（curl-impersonate）能过。
- **猪猪复验**：POST 3 连发全 200（4.7~9s 偏慢·生产超时配 15s+）；**下载链端到端通**（签发→主域 GET→真 torrent 25.6KB bencode）；api 子域 GET 403 无生产影响（API 全 POST·下载走主域 piggo.me/v1/web/）。
- **好大结论修正：不迁移**——243 复验 3 连发 200/200/25s 超时——CF 判罚=「指纹+频率」组合，curl-impersonate 单发改善但**连发仍触发静默挂起**；好大维持既有「全局代理+批量预查+熔断」体系（§59.1487 工作正常）。**curl-impersonate=猪猪专属**。
- **兼容性定案（用户拍板）**：x86_64 only（ARM 不支持）；glibc 最高引用 2.17（CentOS 7 级·2013 后所有 x86 兼容·J1900 老机覆盖）；动态库仅 zlib/pthread/libc（BoringSSL 静态编入近乎自包含）；CA 路径自适应（Debian /etc/ssl/certs/ca-certificates.crt vs RH 系 /etc/pki/tls/certs/ca-bundle.crt——243 Rocky 实证）；Docker 镜像预装零感知·systemd 版随发行二进制分发·缺工具环境猪猪功能 fail-fast 不拖垮主程序。

**§59.538 规则文档吸收（2026-11-16 晚·用户供 /tmp/piggo-rules.txt·发布+RSS+H&R 三章）**：

**发布规则关键**：
- **审核制**：发完先进审核队·过审前他人不可下（发布终态=已提交待审·非即时可见）
- **info_hash 重写**：站方清外站 tracker→改本站 tracker+private+来源标记→**info_hash 与本机不同**——发布链推种必须用**站方下载的 .torrent**（download-link 拉取）做种；同 hash 已有种 422（=API dedup 同源）
- **Markdown only 禁 BBCode**：简介组装需 Markdown 翻译层（现有 assembleDescription 是 BBCode 体系）·**截图必不可少**（简介内嵌）
- 分类自动纠正：站方按 MI 识别**盖住手填**（来源/媒介/编码/音频/分辨率/处理以 MI 为准·制作组优先标题·地区按元数据）——识别规则=去标点最长命中（2160P/4K·H265/HEVC 同档）——我们 taxonomy 映射照做·站方兜底纠正
- 种子文件：v1 only（v2/混合拒）·4MB·文件≤4096
- 发布资格：上传开关+分区等级+**拒绝态 2 颗冻结**（文案透出即可·API 无资格端点）
- 收费/促销/置顶/H&R 挡位=权限限定·普通人促销抽奖——**发布恒不预设**

**RSS 章**：
- **订阅制**：RSS 管理建订阅·每条独立 rss_key（≤10 条）·可按分区/分类/标签/体积/促销等过滤·条数≤100——**验证时 1 条=用户该订阅的过滤配置**·适配时建议用户建「全量 100 条」订阅
- 限额：XML 60/h·enclosure 50/h·**单 IP 120/h**·停用倍数 5——RSS 引擎调度对齐
- 条目链接=24h HMAC 短链·收费种默认无 enclosure·轮换 key 即失效

**H&R**：
- 考核 enabled·挡位制（一档 72h 做种/480h 窗/0.2·二档 120h/480h/0.2）·各分区现均未设挡位→回退窗口 72h+完成率 1
- **发布者不记**（发种无 H&R ✓）·下载他人种（辅种场景）会记——长期做种天然满足
- 停下载阈值 10 条·饲料消一条 100000

**§59.538 回归审核+补齐定案（2026-11-16 晚·用户逐项拍板）**：
- **P1 RSS 全链路接 impersonate 执行层**（回归审核实锤：feed XML+enclosure 下载裸 Go 栈全 403·impersonate 全 200——RSS 引擎对 framework=piggo 走站级执行层通道·原「标准消费」设计作废）。
- **收费种扣费**：用户定案「RSS 由用户创建·风险自担·系统只消费不判断」。
- **待审种自己下载**：按常规可以·首颗实测验证项。
- **官组映射**：catalog 制作组 5 项与用户给 4 官组完全吻合（PigoWeb=1/PigoHD=2/PigoNF=3/PigoAD=4/Other=5 兜底非官组）→ is_source=1 需 release_group_mappings 四行 is_official=1。
- **publish 三件定案**：成功判定=201 Created（非 NP 302 习惯）；422 errors 字典逐项透出（不笼统报错）；media_info 以 API 文档必填为准·素材 MI 缺失发布前 fail-fast（规则页「非必填」矛盾以 API 为准）。
- **taxonomy 处理维度**：4 选项 SDR/HDR10/HDR10+/DV 对 standard_keys（hdr10/hdr10plus/dolby_vision）一一配对·catalog 快照核对后进 migration。
- **H&R 系统零处理（用户定案）**：辅种不考核·RSS 下载用户自担·只消费不判断→adapter HR 恒 false；真实规则备查=HR3（480h 窗保 72h）/HR5（480h 窗保 120h）·带对应标签的种子才考核。
- **好大偶发挂起补录**：间隔 1s 十连发 2 挂（第 2/4 发·后 6 发全过）=随机 ~20% 黑洞节点非频率惩罚·好大维持不迁移·若未来迁移重试链可兜。

**§59.538 第三轮回归（2026-11-16）**：
- **media_info 特判撤销**（用户纠正）：审核门 9 字段含 MI（§59.500 完整才推理）——MI 缺失过不了审核发不出·猪猪特判多余。
- 新遗漏四项定处理：①执行层内置站级限流（exec 绕过 GlobalLimiter·API 各端点 30-60/min）②做种体积分页成本高——首版 total 一发做种数·体积按需③音乐种不发 piggo（正确表述·用户纠正：音乐发布链目标站=已做音乐发布适配的站·目前仅乌托邦 §59.485——piggo 音乐分区未做音乐发布适配故音乐链不路由到它；未来按需单独立项（乌托邦同款流程）·非「piggo 走影视链」）④small_descr 必填——审核门不含副标题·空副标题 422 字典透出。
- 顺路确认：图片零下载（MD 直链内嵌·不触发图片白名单流程）；tags 完结 11/合集 9 入映射表·分集永不勾；exec 数组参数注入免疫。

**§59.540 架构反转定案（2026-11-16 晚·用户拍板）**：
- **猪猪 tls 通道=azuretls chromeTransport**（§59.334 附二既有基建·RoundTripper·Session 复用·重试内置）——实测全绿：ping/搜索+Bearer/RSS feed GET（29KB）/稳定 3 连发 0.1s 级。**curl-impersonate 执行层方案整体撤销**（零新包零二进制零打包）。差异原因：azuretls 自研 HTTP/2 实现未踩中 CF 检测特征（tls-client 的 fhttp 已被识破）。备胎记录：若未来 azuretls 被识破切回 curl-impersonate（验证数据在本档）。
- **猪猪接入形态**：adapter client 走 TLSPretend="chrome"（§59.333 既有配置通道·sites.tls_pretend 列）；RSS 同列配置——纯配置级。
- **好大迁移定案（用户 b）**：243 现场实证 azuretls 3 连发 200（§59.1487 挂起现场根治）→ migration172 hdarea tls_pretend='chrome'（消费面零改动·adapter+RSS 全链自动生效·现状 proxy_url 空 use_global_proxy=0 即无代理直连·迁移后指纹直连根治）。

**§59.541 值集刀定案（2026-11-17·用户逐项拍板·catalog 实测明细快照 /tmp/opencode/piggo-catalog.json 2026-11-16 版）**：

**发布四层结构（站方种实测定锚）**：
- `section_id` 单值分区：1 电影/2 电视剧/3 综艺/5 少儿/6 纪录片/7 动漫/8 音乐/9 图文/10 有声/11 体育/12 其他
- `category_ids` 纯体裁多选数组（无分区前缀·id 跨区复用）：实测 Ronin=[3,7] 动作+惊悚·Fiksik=[5,6,11,14,15] 奇幻科幻+动画+其他+益智启蒙+冒险·Rocky III=[11] 单项兜底合法
- `taxonomy_ids` 八维扁平数组各一项（id 段区分归属）：Ronin=[35,28,25,21,11,9,5,40]=Blu-ray+Remux+H.265+DTS-HD MA+4K+DV+Other组+美国
- `tag_ids` 多选数组：Ronin=[4,5,7,19] 中字+HDR10+DV+中英双语

**A. 动画分区组合判据（方案二·样本 4/4 实证）**：
```
产地=日本 ∧ animation          → 7 动漫
PTGen genre 含 Family/Kids     → 5 少儿
其余 animation                 → 7 动漫（默认）
```
摸站实证：千与千寻/你的名字（日漫剧场）→7·蜘蛛侠平行宇宙（西方青少年向）→7·玩具总动员/小黄人（合家欢低幼）→5·诡秘之主/黑色四叶草/工作细胞（泛年龄剧集）→7。分界本质=受众年龄感人工判断·判据纯度<100% 容忍（站方审核制人工可挪）。

**B. 体裁 category_ids（b1 全映射）**：PTGen genre 词全映射（命中几个挂几个·分区感知有效集）+零命中兜底「11=其他」（电影/剧集/综艺/纪录/少儿/体育区通用）。

**C. tags 22 项接线（12 接+8 不接+2 挂账）**：
| 接（键） | 不接（理由） |
|---------|-------------|
| 3 国语←tag.chinese_audio | 1 官方（转存铁律） |
| 4 中字←tag.chinese_subtitle | 2 禁转（铁律） |
| 5 HDR10←tag.hdr10 | 15 独占（站方权限） |
| 7 DV←tag.dolby_vision | 17 零魔（运营） |
| 10 Atmos←tag.dolby_atmos | 18 自购（转存语义） |
| 6 3D←tag.three_d | 21 未审（站方内部） |
| 8 特效←tag.effects_subtitle | 13 怀旧计划（运营） |
| 9 合集←tag.collection | 14 应求（运营） |
| 11 完结←tag.complete | **20 备胎（挂账——组映射补全轮再议）** |
| 12 追更←tag.ongoing | **22 丐帮（挂账——同上）** |
| 16 分集←键不配（§59.424 禁发永不触发） | |
| 19 中英双语←tag.bilingual_subtitle_chs_eng | |

**HDR 双通道定案（Ronin 实证对齐）**：taxonomy 处理单选（SDR/HDR10/HDR10+/DV）+tags 多选（5+7 可同勾）**两边都写**——DoVi 盘 taxonomy=DV+tags=[5,7]·HDR10 盘 taxonomy=HDR10+tags=[5]·SDR 盘 taxonomy=SDR+无 HDR tags。非财神式同值误投（站方实务即此·taxonomy 单选 vs tags 多选语义分层）。Atmos 走 tags（10）·DDP 走 taxonomy 音频（62）——分域不重叠。

**D. 副标题 small_descr**：直传源站副标题（保真哲学·Fiksik「螺丝钉1-4季全集 1080P 国语中字」实态已可用）。

**E. Markdown 层**：BBCode→MD 纯函数转换（[img]→![]()·[url]→[]()·[b]/[i]·[quote]→>·[size]/[color] 剥壳留内容）——转换层非重写模板·未来其它 MD 站复用。

**taxonomy 8 维值集表（catalog 权威）**：
- 来源(1)：DVD=32/TV=33/WEB-DL=34/Blu-ray=35/Other=36
- 媒介(2)：Untouched=27/Remux=28/Encode=29/DIY=30/Other=31（与 medium canonical 完美对齐）
- 编码(3)：H.264/X.264=24/H.265/X.265=25/H.266/VVC=63/MPEG-2=64/VC-1=65/VP9=66/AV1=67/Other=26
- 音频编码(4)：FLAC=14/APE=15/DTS=16/MP3=17/AAC=18/Other=19/AC-3=20/DTS-HD MA=21/TrueHD=22/LPCM=23/DDP=62
- 分辨率(5)：other=10/4K=11/1080p·i=12/720p=13
- 处理(6)：SDR=6/HDR10=7/HDR10+=8/DV=9（§59.538 已定案键配对）
- 制作组(7)：PigoWeb=1/PigoHD=2/PigoNF=3/PigoAD=4/Other=5
- 地区(8)：内地=37/香港=38/台湾=39/美国=40/英国=41/韩国=42/日本=43/法国=44/德国=45/意大利=46/西班牙=47/加拿大=48/澳大利亚=49/印度=50/泰国=51/俄罗斯=52/其他=53

**miss 兜底哲学**：taxonomy 八维全填（素材缺→Other/SDR 兜底）——站方 MI 纠正盖住手填（Ronin 八维全填实证·站方识别规则=去标点最长命中）；体裁分类站方不纠正（填错就错·审核人工兜底）。

**实施分解**：
1. Markdown 转换层（internal/description·纯函数+锚矩阵）
2. executor 猪猪特化：piggoSectionOf（A 判据·animation 覆写）+piggoCategoryIds（B·genre 词→分区感知分类表）——PublishRequest 无 genre·组装段 meta 上下文生成
3. migration174：form_config（type→section_id+8 taxonomy 虚拟字段+tags 12 项）+is_target/publish_supported 开闸
4. PiggoAdapter 组装：8 虚拟字段合并 taxonomy_ids JSON 串+tag_ids 数组串+descr MD 转换挂载
5. 测试锚：A 判据三态+B 映射+MD 矩阵
6. DryRun 借壳（243 Fiksik 素材现成）→首颗实测（待用户）

### §59.541 附·首颗实战 422 根修（2026-11-17·v0.0.1512）

- **首颗**：Obsession 2025 UHD BluRay（簇代表 1d6392d1·源 朋友）→ 二师兄，10:33 执行。
- **现象**：`failed · 上传失败: piggo 发布 422: 请填写副标题`（publish_result_records id=4847）。
- **根因**：素材副标题**在场**（`【痴迷/爱你致死不渝(港/台)/痴爱成魔】mUHD作品 4k 杜比视界版本 英语 评论音轨 简繁英双语字幕`）但 form_fields 未配 `small_descr` 域 → setForm 守卫跳过 → executor :641 `pubReq.Subtitle = form[small_descr]` 恒空 → adapter `addField("small_descr","")` → 站方必填校验拒收。**与 description 空键（migration176）同族第二例：form_fields 缺域=文本域断链**。
- **修**：migration174 源码补 `"small_descr":"small_descr"` + migration177 json_set 刷存量 + FreshInstallChain ④-b 锚（piggo 契约必填文本域 small_descr/description 缺域即 CI 红——防同族第三例）。
- **残留边缘**：源副标题为空的素材仍走站方 422 透出（摸站已知行为·piggo.md §新遗漏④——不拦不合成，保真哲学）。
- **方法论**：契约必填文本域≠「adapter 读了 req.Subtitle 就行」——**executor→form→pubReq 的域映射是唯一通道**，新站接入 checklist 补一条「grep adapter 契约必填文本域 ↔ form_fields 配域对照」。

### §59.541 附三·MD 转换层实战 422 根修（2026-11-17·v0.0.1514）

- 首发二阶段（网络恢复后）：连发 5 颗全 422「简介含有 BBCode，本站仅支持 Markdown」——转换器双漏点：Quote 模式无 (?s)（多行引用块整个漏过）+嵌套 [quote] 单遍非贪婪残留。修=18 模式补 (?is)+quote 内层优先循环+残骸清扫（详见 docs/31 §59.541 附三）。**站方 MD 检测器是严格白名单**——任何残留 BBCode token 即 422。

### §59.541 附四·{{ptgen}} 模板机制（2026-11-17·v0.0.1515）

- **站方校验**：已填豆瓣/TMDB/IMDb 任一字段时，简介**必须**包含 `{{ptgen}}` 占位符——站方用自营 PTGen 服务（ptgen.piggo.me）在该处服务端渲染简介块。
- **实现**：piggoDescrPtgen（executor :558 后·Framework==piggo）——自制 PTGen 正文整体替换为占位符（防双份）；无正文前置占位符；无链接零变化。
- **首次 422 记录**：4865-4869 五颗（v0.0.1514 重发批）。

### §59.541 附七·推种断链根修+API 差异记录（2026-11-17·v0.0.1516）

- **piggo 上传响应不包含 .torrent 数据**——与好大（return_torrent_base64）不同。adapter 不得回填 `TorrentData = req.TorrentData`（本地数据≠站方终版）。
- **piggo 上传后重新生成 .torrent**：加 `source: "[piggo.me] 猪猪"` 字段 → info_hash 变化。实证 tid=57289：本地 hash fa2de723ce → piggo hash aa21ecbd。
- **announce 使用 token 认证**（停 passkey·只接受下载令牌——piggo.md :422 记载）。
- **新 API 站接入三问**：①上传响应是否含 .torrent？②站方是否重写（source 字段）？③info_hash 是否与上传时一致？

### §59.541 附九·download-link 端点完整行为指令（文档漏读补录）

> **站方文档原文（用户供 API 文档 v3·625 行）**：
> "签发约 24 小时的种子文件 URL（不含 passkey）。无权 403，看不见 404。成功 announce 后作废。收费种签发时检查余额，真正拉文件时才计次、扣费。**拿到 url 后用普通 GET 下载，不要把令牌带到 Tracker。**"

**关键行为指令**：
1. 返回的 URL 用**普通 GET** 下载——不需要 Bearer/chrome TLS（签名 URL 自含鉴权）
2. **不要把令牌带到 Tracker**——API 令牌不能出现在 announce URL 中
3. 签发 24h 有效·成功 announce 后作废
4. 收费种：签发时查余额·拉文件时扣费

**教训**：API 文档摘要只记端点清单（「有什么」）不够——**行为指令（「怎么用」）同等重要**。今后新 API 站接入，每个端点的完整描述段必须通读，不能只看路由表。
