# 野马（YemaPT）站点适配器设计

## 站点信息

| 项目 | 值 |
|------|-----|
| 站点名称 | 野马（YemaPT） |
| 域名 | www.yemapt.org |
| 框架 | 自研 SPA（Umi.js + Ant Design Vue），归类 `generic` |
| 前端技术 | Umi.js + Ant Design，前端静态资源托管于 static-ui.yemapt.org |
| 后端 API | REST JSON，所有接口返回 `{"success":bool,"showType":int,"data":...}` |
| CookieCloud 域名 | `yemapt.org`（非 www） |
| 认证方式 | Cookie（`auth` 字段） |
| Cloudflare | 是（Turnstile 验证） |
| 候选制 | 是（Level 6 及以上或发布种子数 > 10 可开启免候选） |
| MediaInfo | 否（无独立字段） |
| IMDb | 是（`imdb` 字段，仅填 ID） |
| 豆瓣 | 是（`douban` 字段，仅填 ID） |
| 匿名发布 | 是（`uploadUserAnonymous` radio） |
| NFO | 否 |
| PT-Gen | 否 |
| 封面图 | 是（`picture` 字段，URL） |
| 简介格式 | **Markdown**（非 BBCode，提供转换功能） |
| HR 惩罚 | 是（发布者可选开启，不可修改） |
| 做种要求 | 上架后必须有人做种满 24h，否则删除并处罚 |
| 盒子规则 | 有详细盒子限制 |

## Tracker URL

`https://www.yemapt.org/announce`

## 发布 API

种子发布通过 REST API 而非传统表单 POST：
- **分类选项**：`GET /api/category/fetchCategorySelectOptions`
- **发布描述配置**：`GET /api/torrent/fetchUploadDesc`
- **用户信息/权限**：`GET /api/user/profile`（含 `accessFlag.canTorrentAdd` 等）
- **发布提交**：推测为 `POST /api/torrent/add`（JSON body）

## 发布页面字段

| 字段 | name | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| 种子文件 | `files` | file | 是 | 最大 3MB |
| 标题 | `showName` | text | 是 | |
| 副标题 | `shortDesc` | text | 否 | |
| 预览图 | `picture` | text | 否 | 未填则从 IMDb 信息解析 |
| 简介 | Markdown 编辑器 | textarea | 否 | Markdown 格式，支持 BBCode 转换 |
| 分类 | `categoryId` | select | 是 | 数字 ID |
| 匿名上传 | `uploadUserAnonymous` | radio | 否 | 是/否 |
| IMDb | `imdb` | text | 否 | 仅填 ID（如 tt1234567） |
| 豆瓣 | `douban` | text | 否 | 仅填 ID |
| 媒介 | `medium` | select | 否 | |
| 分辨率 | `standard` | select | 否 | |
| 视频编码 | `codec` | select | 否 | |
| 音频编码 | `audiocodec` | select | 否 | |
| 地区 | `region` | select | 否 | |
| 制作组 | `team` | select | 否 | |
| 标签 | `tagList` | multi-select | 否 | |
| HR 惩罚 | `hrSetPunish` | checkbox | 否 | 发布者可选开启，提交后不可修改 |

## 分类（categoryId）— 4 大类 17 小类

### 影视（大类 1）

| ID | 名称 |
|----|------|
| 4 | 电影 |
| 5 | 剧集 |
| 13 | 综艺 |
| 14 | 动漫 |
| 15 | 纪录片 |
| 17 | 体育 |
| 6 | 短剧 |
| 16 | MV/演唱会 |

### 综合（大类 2）

| ID | 名称 |
|----|------|
| 3 | 软件 |
| 10 | 游戏 |
| 12 | 书籍 |
| 22 | 其他 |

### 音频（大类 7）

| ID | 名称 |
|----|------|
| 8 | 音乐 |
| 9 | 广播剧 |

### 教育（大类 18）

| ID | 名称 |
|----|------|
| 19 | 教育书籍 |
| 20 | 教育音频 |
| 21 | 教育视频 |

## 质量字段

### 媒介（medium）— 10 个

| 值 | 显示名称 |
|----|----------|
| Web-dl | WEB-DL |
| Blu-ray | Blu-ray |
| Blu-rayUHD | UHD Blu-ray |
| Remux | Remux |
| Encode | Encode |
| HDTV/TV | HDTV |
| DVDrip | DVDRip |
| CD | CD |
| DVD | DVD |
| Other | Other |

### 分辨率（standard）— 9 个

| 值 | 显示名称 |
|----|----------|
| 720i | 720i |
| 720p | 720p |
| 1080i | 1080i |
| 1080p | 1080p |
| SD | SD |
| 1440p/2K | 2K |
| 2160p/4K | 4K |
| 8K | 8K |
| Other | Other |

### 视频编码（codec）— 10 个

| 值 | 显示名称 |
|----|----------|
| H.264(x264/AVC) | H.264 |
| H.265(x265/HEVC) | HEVC |
| Bluray(VC1) | VC-1 |
| Bluray(AVC) | Blu-ray AVC |
| Bluray(HEVC) | Blu-ray HEVC |
| MPEG-2 | MPEG-2 |
| Xvid | Xvid |
| AV1 | AV1 |
| H.266/VVC | VVC |
| Other | Other |

### 音频编码（audiocodec）— 15 个

| 值 | 显示名称 |
|----|----------|
| AAC | AAC |
| AC3 | AC3 |
| DTS | DTS |
| DTS-HD MA | DTS-HD MA |
| E-AC3(DDP) | DDP |
| E-AC3 Atoms | E-AC3 Atmos |
| TrueHD | TrueHD |
| TrueHD Atoms | TrueHD Atmos |
| LPCM | LPCM |
| FLAC | FLAC |
| APE | APE |
| MP3 | MP3 |
| OGG | OGG |
| Opus | Opus |
| Other | Other |

### 地区（region）— 8 个

| 值 | 显示名称 |
|----|----------|
| CN(中国) | 中国大陆 |
| HK/CN(香港) | 香港 |
| TW/CN(台湾) | 台湾 |
| US(美国) | 美国 |
| EU(欧洲) | 欧洲 |
| JP(日本) | 日本 |
| KR(韩国) | 韩国 |
| Other | 其他 |

### 制作组（team）— 11 个

| 值 | 显示名称 |
|----|----------|
| OurBits | OurBits |
| BtsHD | BtsHD |
| BtsTV | BtsTV |
| HDChina | HDChina |
| CMCT | CMCT |
| HHWEB | HHWEB |
| FRDS | FRDS |
| MTeam | MTeam |
| QHstudio | QHstudio |
| UBits | UBits |
| Other | Other |

### 标签（tagList）— 12 个

| 值 | 显示名称 |
|----|----------|
| 禁转 | 禁转 |
| 首发 | 首发 |
| 官方 | 官方 |
| 自制 | 自制 |
| 国语 | 国语 |
| 中字 | 中字 |
| 粤语 | 粤语 |
| 英字 | 英字 |
| HDR10 | HDR10 |
| 杜比视界 | 杜比视界 |
| 分集 | 分集 |
| 完结 | 完结 |

## 缺失字段

- **无 source_sel** — 无来源字段
- **无 NFO 上传**
- **无 PT-Gen 字段**
- **无独立 MediaInfo 字段**
- **无 processing_sel** — 地区通过 `region` 字段实现

## 禁转检测

标签中有"禁转"选项，发布时可选。

## 基本规则

### 允许的资源

- 合法传播权的文件
- 体积 ≥ 100MB 的资源
- 短剧类需发布全集

### 禁止的资源

- 体积 < 100MB 的资源（管理员保留删除权）
- 网络游戏客户端（私服除外）
- 单独的 Sample/预告片
- 禁忌/敏感/色情/政治内容
- 含商业网站链接或他站推广的种子
- 压缩包形式上传（rar/zip/7z），漫画和电子书/游戏/图包除外
- 带密码的压缩包
- 处于禁转期或标明禁止转载的资源
- 偷拍影片
- 低码率转高码率影片
- 预告片

### 标题要求

- 各站/制作组标题要求不尽相同，转种时保留原标题/副标题即可
- 非标内容暂无要求

### 重复判定（Dupe）

- 旧版本有错误/重复等问题 → 允许新版本发布
- 旧版本断种或发布超过 18 个月 → 允许新版本发布
- 发布新版本需在评论区注明原因

## 促销规则

### 上传促销

- 1.5 倍上传
- 2 倍上传

### 下载促销

- 50% 下载
- FREE（免费）

### 促销触发

- 新发布种子随机开启 Free，优惠时长以站内实际情况为准
- 不定期开启全站 Free
- 发布者可用积分兑换促销

## H&R 规则

### 考核机制

- 所有种子都进行 HR 考核
- 下载完成后自动开始考核
- **7 天内**完成 **24 小时**做种，或分享率 > 1
- 系统统计时检查做种时间或分享率是否达标

### HR 惩罚

- 发布者发布种子时选择是否开启 HR 惩罚（开启后不可修改）
- 未达标者在首页看到重要提示
- 免罪需花费 **20000 积分**
- 发布者获得免罪积分的 **30% 回馈**
- 当前管理可能关闭 HR 惩罚设置功能（见公告）

### 终极惩罚

- 系统每天凌晨统计带惩罚的考核未达标数
- 未达标数超过 **10 个** → 自动 ban 号

### 终极惩罚免疫规则

1. 野马之友权限（PT 工具圈开发者，管理手动添加）
2. Level 10 及以上
3. 参与当月契约
4. 管理认为需要关闭终极惩罚时

> 免疫不代表不需要接受惩罚，请时刻保持未达标数为 0。

## 盒子规则

### 盒子定义

- 使用服务器/seedbox/VPN 向 tracker 汇报的 IP
- 家宽上传速度超过 **12.5MB/s** 的 IP

### 盒子限制

1. **不享受上传促销**：包含种子上传促销和发布者限定小时内双倍上传促销，下载促销不受限制
2. **非自己发布的种子** + 盒子上传量超过种子文件大小 **3 倍** + 未参加当月契约 → 本次汇报上传量计 **0**

## 发布奖励

### 积分奖励

- 发布种子并通过候选后获得积分
- 前 **30 个**种子每个获得 **500 发布奖励积分**
- 奖励积分在种子上架 **3 天后**由系统自动发放
- 一天最多提交 **100 个**种子

### 发布人做种积分奖励

| 时间段 | 每小时额外积分 |
|--------|---------------|
| 上架后 24 小时内 | 10 |
| 上架 30 天内 | 5 |
| 上架 365 天内 | 1 |

三个时间段的积分奖励不累加，取最高值。前 30 个种子可获得此奖励。

### 发布人上传奖励

种子上架后特定时间内，发布人上传量计 **双倍**（不与其他上传促销叠加）。

## 等级体系

| 等级 | 称号 | 特权 | 升级条件 |
|------|------|------|----------|
| Level 0 | 乱民 | 无 | 下载>10GB 且分享率<0.3，次日 0 点禁用 |
| Level 1 | 小卒 | 无 | 新注册默认；降级条件见下方，持续 7 天未改善禁用 |
| Level 2 | 教谕 | 查看站点统计 | 不符合其他规则时默认 |
| Level 3 | 登仕郎 | 查看种子 TOP10 | 下载>100GB，分享率>1 |
| Level 4 | 修职郎 | 查看种子 TOP250、用户 TOP10 | 下载>200GB，分享率>2，注册>30天，月均下载>5GB |
| Level 5 | 文林郎 | 查看用户 TOP250、匿名发帖 | 下载>400GB，分享率>3，注册>60天，月均下载>10GB |
| Level 6 | 忠武校尉 | **免候选发布** | 下载>500GB，分享率>4，注册>90天，月均下载>15GB |
| Level 7 | 承信将军 | 发帖可禁止评论 | 下载>800GB，分享率>5，注册>120天，月均下载>20GB |
| Level 8 | 武毅将军 | — | 下载>1TB，分享率>6，注册>150天，月均下载>25GB |
| Level 9 | 武节将军 | **永久保留账号** | 下载>2TB，分享率>7，注册>180天，月均下载>50GB |
| Level 10 | 显威将军 | HR 终极惩罚免疫 | 下载>2.5TB，分享率>8，注册>210天，月均下载>60GB |
| Level 11 | 宣武将军 | — | 下载>3TB，分享率>9，注册>240天，月均下载>70GB |
| Level 12-17 | 将军系列 | — | 更高要求 |

### Level 1 降级条件

| 下载量 | 分享率低于 |
|--------|-----------|
| > 10 GB | 0.6 |
| > 50 GB | 0.7 |
| > 100 GB | 0.8 |
| > 200 GB | 0.9 |
| > 500 GB | 1.0 |

## 客户端白名单

| 系列 | 版本 |
|------|------|
| qBittorrent | 4.0.x-4.6.x、5.0.x |
| Transmission | 2.9.x、3.0.x、4.0.x |
| uTorrent | 2.x、3.x |
| BitComet | 2.x |
| rTorrent | 0.9.8 |
| BitTorrent | 7.x |
| BiglyBT | 3.x.x.x |
| webtorrent | 0.0.2.4 |
| libtorrent | 0.16.17.0 |
| LibreTorrent | 2.0-2.1 |
| tixati | 3.29 |

## 积分体系

- 货币名称：积分（整数，无小数）
- 获取途径：签到、发布种子、做种、被赠送、娱乐场、契约
- 赠送积分收取 30% 手续费

## 适配器实现要点

### 1. REST API 发布

野马不使用传统 NexusPHP 表单 POST，而是通过 REST JSON API 发布。适配器需要：

```
POST /api/torrent/add
Content-Type: multipart/form-data 或 application/json

字段：
- files: 种子文件
- showName: 标题
- shortDesc: 副标题
- categoryId: 分类 ID（数字）
- medium: 媒介（字符串值）
- standard: 分辨率（字符串值）
- codec: 视频编码（字符串值）
- audiocodec: 音频编码（字符串值）
- region: 地区（字符串值）
- team: 制作组（字符串值）
- tagList: 标签（字符串值数组）
- imdb: IMDb ID
- douban: 豆瓣 ID
- picture: 预览图 URL
- uploadUserAnonymous: 是否匿名
- descr: 简介（Markdown）
```

### 2. CookieCloud 域名

精确匹配 `yemapt.org`，非 `www.yemapt.org`。需设置 `CookieCloudDomain: "yemapt.org"`。

### 3. 简介格式为 Markdown

与其他 NexusPHP 站点的 BBCode 不同，野马使用 Markdown。适配器需要将 BBCode 转为 Markdown 后提交。

### 4. 种子文件大小限制

最大 **3MB**。超大的种子文件需要先去除介绍，提交成功后再修改。

### 5. 字段值为字符串

除 `categoryId` 为数字外，其余质量字段（medium/standard/codec/audiocodec/region/team）的值均为**字符串**（如 `"Web-dl"` 而非数字 ID）。

### 6. H&R 标记在 RSS 中

RSS 标题末尾带 `[HR]` 表示该种子有 HR 惩罚。

## 特殊说明

1. **自研 SPA 架构**：完全不同于 NexusPHP 的传统 HTML 表单，所有操作通过 REST API
2. **简介格式 Markdown**：非 BBCode，需要格式转换
3. **字段值类型**：categoryId 为数字，其他质量字段为字符串（与 NexusPHP 数字 ID 不同）
4. **CookieCloud 域名**：`yemapt.org` 而非 `www.yemapt.org`
5. **HR 惩罚由发布者控制**：发布时可选开启，不可修改
6. **盒子限制严格**：不享受上传促销，非自发布种子盒子上传超 3 倍计 0
7. **发布做种强制**：上架 72h 内必须有人做种满 24h，否则删除并处罚
8. **Level 6 免候选**：达到条件后可在个人设定中开启
9. **短剧分类独特**：有独立的"短剧"分类(6)
10. **教育大类**：含教育书籍(19)、教育音频(20)、教育视频(21) 三个子分类
11. **RSS HR 标记**：标题末尾 `[HR]` 标识带 HR 惩罚的种子

*数据来源: Playwright 采集 upload 页面 + wiki.yemapt.org 规则页面（2026-05-04）*
