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
- **部署确认**：浏览器地址栏 GET→405 应用层错误页（非 CF 非 SPA 壳）；浏览器同源 POST ping→`{"code":0,"data":{"ok":true}}`；/me→user 对象（id=22479 <作者>·can_upload=true·has_passkey=false 印证 Tracker 换令牌）。
- **CF 破解=TLS 指纹**：curl-impersonate chrome116 POST→200（同 IP 同秒裸 curl→403 铁证）；**无需 cf_clearance**；主域 /v1/api/* 仍 nginx 静态 405（未代理）——生产走 api.piggo.me。
- **实测链**：ping ✓ / me ✓（Bearer+IPv6 出口 2408:...）/ torrents 搜索 ✓（q=Dune→60 条真实结果全字段：name/small_descr/cover/size/section/category/taxonomy/media{douban,imdb}/seeders/times_completed/added_at/page_url/tag_ids·分页 total_capped）。
- **生产架构方向**：Go 客户端需浏览器 TLS 指纹（bogdanfinn/tls-client 或 utls 方案）——CF 指纹墙唯一障碍已破；Bearer+POST JSON+统一信封+九端点全契约在手。
