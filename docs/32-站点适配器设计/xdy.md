# 修道院 站点适配器设计

> 状态：**接入设计阶段**（2026-09-02 实抓 upload.php / rules.php / index.php 分析，cookie 已由 CookieCloud 同步）。本档为站点行为权威定义，适配实施以本档为准。

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 修道院 |
| 域名 | xdypt.vip |
| 站点地址 | https://xdypt.vip |
| 站点框架 | NexusPHP |
| 直连性 | 243 Docker 直连可达（302 login.php → 登录态 200） |
| 上传端点 | `takeupload.php`（POST multipart/form-data，form name="upload"） |
| Tracker | `https://xdypt.vip/announce.php`（**同域 announce**，与幸运分域 tracker 不同） |
| 用户 | ranfish（user_id=10318，c_secure_pass 单 cookie 登录态有效） |
| 候选制 | 无强制（rules："任何人都能发布资源"；游戏类需候选或上传员等级） |
| 全站活动 | 2X Free 至 2026-09-30；转种/发种大赛（500 种=6 个月 VIP；站点种子总量目标 10000——**发展期站点，欢迎转存**） |

## 与幸运的关键差异（适配要点）

| 维度 | 幸运 | 修道院 | 适配影响 |
|------|------|--------|---------|
| 音频编码 | `audiocodec_sel[4]` 独立字段 | **无 audiocodec 字段**（select 仅 type/medium/codec/standard/team 五个） | form_config 映射少一维；幸运判据八条中"音频组合键"不适用 |
| 中文名 | 无独立字段（并入 name） | **`cnname` 独立字段** | 标题组装需拆分中英文名 |
| type 形态 | radio + data-mode 联动 | **select 无 data-mode**（质量字段名硬编码 `[4]` 电影后缀） | 表单解析按静态 select 处理；非电影类的质量字段名待实测（`upload.php?type=` 参数或 JS 注入） |
| standard 号段 | 4K=6 | **4K=8 / 2K=7 / 8K=9 / 1080p=6 / 720p=11 / 480p=12** | **ValueMappings 必须按站重做**（dict 号段不可复用幸运） |
| medium 号段 | 13 个（UHD BD=10 等） | **11 个**：1 Blu-ray / 2 HD DVD / 3 Remux / 4 MiniBD / 5 HDTV / 6 DVDR / 7 Encode / 8 CD / 9 Track / **10 WEB-DL / 11 ISO**（无 UHD BD 独立项） | 同上 |
| 发布组 | team_sel（号段未采） | **team_sel 预置下拉**：WiKi=16 / MTeam=26 / **FRDS=25** / ADWeb=24 / HHWEB=23 / ZmWeb=22 / UBWeb=21 / AGSVWEB=20 / CSWEB=19 / StarfallWeb=18 / TPWEB=6 / MySiLU=15 / Other=17 | FRDS 直选 25（朋友站 FRDS 种转存友好） |
| 标签形态 | tags（dict 词条） | **`tags[4][]` checkbox 数组**：1 禁转 / 2 首发 / 3 官方 / 4 DIY / 5 国语 / 6 中字 / 7 HDR | TagArrayFields 数组参数直传；**发布转存种不勾"禁转"(1)**，FRDS 转存惯例勾"首发"(2)待用户定 |
| 审核系统 | LuckAudit 预审（pre-audit API） | **无预审 API**（规则页人工管理；"违规种子不经提醒直接删除"） | 无 pre-audit 步骤——executor 该步跳过（§59.156 pre-audit 本就是可选步骤） |
| MediaInfo | technical_info 独立必填字段 | **无 technical_info 字段**（MI 写入 descr 或 NFO） | MI 资产投递目标=descr 内或省略（见"发布组装"） |
| IMDb | url（data-pt-gen） | url（data-pt-gen）✓ 同 | — |

## 发布页面字段（upload.php 实抓）

| 字段 | name | 必填 | 说明 |
|------|------|------|------|
| 种子文件 | `file` | ★ | 红星标记 |
| 标题 | `name` | ★ | 英文/主标题（服务端强制，页面红星未标但 NP 必填） |
| 中文名 | `cnname` | 否 | **独立字段**（幸运无） |
| 副标题 | `small_descr` | 否 | |
| IMDb 链接 | `url` | 否 | `data-pt-gen="url"` |
| NFO 文件 | `nfo` | 否 | |
| 简介 | `descr` | ★ | BBCode 编辑器（b/i/u/IMG/list/quote 按钮组） |
| 类型 | `type` | ★ | select 401-410 |
| 媒介 | `medium_sel[4]` | 是 | 11 项 |
| 编码 | `codec_sel[4]` | 是 | 7 项 |
| 分辨率 | `standard_sel[4]` | 是 | 7 项 |
| 制作组 | `team_sel[4]` | 否 | 13 项预置 |
| 标签 | `tags[4][]` | 否 | checkbox 多选（禁转/首发/官方/DIY/国语/中字/HDR） |
| 匿名发布 | `uplver` | 否 | checkbox value=yes |
| 提交 | `upload` | — | submit（"我已经阅读过规则/发布"按钮，无隐藏确认字段） |

**无 audiocodec / 无 pt_gen / 无 technical_info / 无隐藏字段**（generator/list/u/i/b 为编辑器按钮 name，非数据字段）。

## 分类 (type select)

| ID | 名称 |
|----|------|
| 401 | 电影（Movies） |
| 402 | 电视剧（TV Series） |
| 403 | 综艺（TV Shows） |
| 404 | 纪录片（Documentaries） |
| 405 | 动漫（Animations） |
| 406 | Music Videos |
| 407 | 体育（Sports） |
| 408 | HQ Audio |
| 409 | 音乐（Misc） |
| 410 | 游戏（Games） |

（401-405 与幸运 401-405 同号段同名；幸运 411-413 短剧等扩展本站无）

## 质量字段

### 媒介 medium_sel[4]

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
| 10 | WEB-DL |
| 11 | ISO |

### 编码 codec_sel[4]

| ID | 名称 |
|----|------|
| 1 | H.264 |
| 2 | VC-1 |
| 3 | Xvid |
| 4 | MPEG-2 |
| 5 | Other |
| 6 | H.265 |
| 7 | VP8/9 |

（无 DV/AV1 独立项——DoVi 种按 H.265+HDR 标签表达）

### 分辨率 standard_sel[4]（⚠️ 号段与幸运不同）

| ID | 名称 |
|----|------|
| 6 | 1080i/1080P |
| 7 | 2K/1440i/1440P |
| 8 | 4K/2160i/2160P |
| 9 | 8K/4320i/4320P |
| 10 | Other |
| 11 | 720i/720P |
| 12 | 480i/480P |

### 制作组 team_sel[4]

| ID | 名称 |
|----|------|
| 16 | WiKi |
| 26 | MTeam |
| 25 | FRDS |
| 24 | ADWeb |
| 23 | HHWeb |
| 22 | ZmWeb |
| 21 | UBWeb |
| 20 | AGSVWEB |
| 19 | CSWEB |
| 18 | StarfallWeb |
| 17 | Other |
| 6 | TPWEB |
| 15 | MySiLU |

## 上传规则要点（rules.php）

- **允许**：HD 视频（原碟/Remux/HDTV/重编码≥720p）、SD（源自 HD 的≥480p/DVDR）、无损音轨、≥5.1 音轨、游戏镜像、7 日内预告片、HD 相关软件文档
- **禁止**：<100MB（例外：软件文档/单曲）、SD upscale、CAM/TC/TS/SCR 等枪版、RMVB/FLV、有损音频（<5.1，国语粤语 2.0+ 例外）、RAR 压缩包、内嵌种子文件、违禁内容
- **dupe 判定**：媒介优先级 Blu-ray > HDTV > DVD > TV；同媒介同分辨率重编码按发布组优先级（Scene/Internal→Group→Quality-Degree）；**断种 45 日或发布 18 个月以上可重发不受 dupe 限制**（转存窗口判断依据）
- **简介要求**：影视必须海报/横幅/封面 + 尽可能截图 + MediaInfo 详情（格式/时长/编码/码率/分辨率/语言/字幕）+ 演职员/剧情——**MI 写入 descr（无独立字段）**；NFO 图写 NFO 不贴简介
- **标题规范**：`[中文名] 名称 [年份] [剪辑] [发布说明] 分辨率 来源 [音频/]视频编码-发布组`（与幸运同源 HDChina 系模板）
- **匿名**：uplver=yes 不显示发布者用户名
- 做种要求：≥24h / 他人完成前不撤种

## 发布组装与执行适配（executor 接入清单）

1. **form_config**：HTML 上传 diff 流（§59.157 FormConfigPanel）——`ParsePublishFormHTML` 直接解析本站表单（select 静态无 data-mode，解析器兼容性待验证）；ValueMappings 按**本档号段表**落库（不复制幸运 dict）
2. **资产投递**：quote/descr 组装复用 assembleDescription（**MI 全角化防御对本站无谓但无害**——本站无 MI-in-descr 检测；MI 资产可附 descr 末尾，站规"尽可能包含"非强制）；url=IMDb ✓；无 pt_gen/technical_info 字段
3. **cnname**：新字段——中文名拆分投递（meta 的中文标题→cnname，主标题→name）
4. **tags[4][]**：数组形态（TagArrayFields 已支持 §59.159）；首发勾选待用户定夺
5. **pre-audit**：本站无 → executor 跳过（可选步骤设计兼容）
6. **上传判定**：NP 标准 takeupload 成功=302 → details.php?id=N（redirect 权威 + upload_classify.go 公共单点已覆盖）
7. **新种下载**：BuildNexusDownloadURL 标准 `download.php?id=N&passkey=`（243 实测确认 passkey 形态）
8. **推种**：下载器 RPC 通用（无站特判）

## 待实测项（适配实施时验证）

- [ ] 非电影类（402-410）质量字段名是否仍为 `[4]` 后缀（upload.php 无 type 参数默认电影表单）
- [ ] download.php passkey 下载链接实测
- [ ] takeupload 302 成功页与失败页形态（信文案不信页面 ID——§59.160 原则）
- [ ] 首发标签（tags[4][2]）转存种是否勾选——用户经验定夺
- [ ] descr 中 MI 文本是否触发人工审查（无自动检测，风险低）
- [ ] 种子最小体积限制（幸运 1GB，本站规则仅见 100MB 下限——验证是否有上限/下限服务端校验）
