# 乌托邦 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 乌托邦（2026-11-05 由"音乐乌托邦"系统性改名·migration148） |
| 站点地址 | https://www.musopia.vip |
| 站点框架 | NexusPHP（音乐专站·纯音乐内容） |
| Tracker | `https://www.musopia.vip/announce.php`（**同域**·.torrent announce 铁证实证 2026-11-05；tracker_domains=["www.musopia.vip"] 配置正确） |
| 官组体系 | **有**——原抓组 `ZHENGLIFAN@Musopia`（@后缀形态）+制作组字段行（制作组: Musopia）——NP 音乐官组形态（织梦 ZmMusic 同款）；非海豚式无官组 |
| 特殊功能 | 保种区（🌱×N 已保护）·种子认领机制·歌词链接（/lyr） |
| 素材现状 | **PT2 做种 131 颗**（2026-11-05 transmission 实测·快照 6/6 命中——素材入口通；seeding_torrent_records 0 行=**误报已销**：该表为刷流/RSS 经手流水账非全量镜像（皇后/海豚同为 0 行·红豆饭 504 行全 imported 实证）；种名含中文方括号变体"\[1994\] 许景淳 - xxx\[台湾\]"——标题解析新形态） |

---

## 一、获取侧摸底（2026-11-05·tid=10072/17109 实证）

### 详情页结构（NP 标准+音乐扩展字段行）

```
标题四段+后缀：艺人 - 专辑 年份 - FLAC 整分轨 - [后缀]
  后缀两形态：官组种=ZHENGLIFAN@Musopia｜转载种=来源站名（- Redacted）
字段行（rowhead 族）：
  副标题     双形态：官组种自由文本（"内嵌专辑封面"）/转载种规格斜杠链
            （"FLAC / Lossless / CD / Log (100%) / Cue"——海豚同款形态）
  标签       多维：首发｜官方｜地区（港台/大陆）｜流派（流行(Pop)·中英混合 rock）
  语言       华语/纯音乐
  基本信息   大小/类型: HQ Audio/媒介: CD（可选）/音频编码: FLAC/制作组: Musopia
  Log score  ★字段行直读（100）——三源最强 log 结构化
  "转载自：" ★字段（转载链结构化——OriginChain 最强源）
独立区：
  抓轨 Log（共 N 碟）——EAC/EZ CD 全文（第三种 log 位置形态实证：
    简介内嵌/MI 槽/独立专区——§九-c 命中）
  音轨(曲目)——曲目区+歌词链接
种名：[乌托邦].xxx (年份) [FLAC] {目录编号}——{CatNo} 花括号形态
```

### MusicMeta 映射评估（22 字段）

| 强项 | 源 | 备注 |
|---|---|---|
| **LogScore** | 字段行直读 | vs 海豚 API/皇后文本解析——NP 站最强 |
| **OriginChain** | "转载自："字段+标题来源后缀 | B4 挂账的最佳白捡站 |
| Tags | 多维标签行 | 首发/官方是乌托邦特色维 |
| TrackList/LogText | 独立区 | NP DOM 解析 |
| Label/CatNo | 种名 {} | 解析层 |
| Artist/Album/Year/Format/AudioMode | 标题四段 | 通用解析 |

ShareRule=**声明文本形态**（"音乐乌托邦原抓组ZHENGLIFAN作品，欢迎转载
各大PT，请在帖子顶部注明出处及作者，并保留后缀"）——欢迎+条件式；
**禁转变体待实证**（源站铁律·下轮补摸）。

### 三源对照（皇后/海豚/乌托邦）

| 维度 | 皇后 | 海豚 | 乌托邦 |
|---|---|---|---|
| 框架 | NP 定制 | Gazelle | NP |
| 官组 | OpenCD | 无 | ZHENGLIFAN@Musopia |
| LogScore | 文本解析 | API 直读 | **字段行直读** |
| 转载链 | 字段行(来自) | 盲区 | **转载自+标题后缀**（最强）|
| log 文本 | divnfo | 无 | 抓轨 Log 区 |
| 曲目 | divtracklist | fileList | 音轨(曲目)区 |

### 禁转规则实证（2026-11-05 四路摸底·源站铁律合规）

**结论：乌托邦无站级禁转体系**（转载文化开放）——四路实证：
rules.php 零转载条款/faq.php 零命中/tags.php 标签总表零禁转类/
详情页+站内搜索零"禁转"字样（唯一命中=误匹配）。

**ShareRule 两形态**（无禁转变体）：
- 官组种：声明文本（"欢迎转载各大PT+注明出处保留后缀"——欢迎+条件式）
- 转载种：**"转载自："=完整源种 URL 直链**（https://redacted.sh/torrents.
  php?id=…&torrentid=…——OriginChain 最强形态：直达源种页面，比站名/
  文本链精细一个量级）+"来源站点"字段（站名）

§59.369 附铁律合规记录：实证无禁转=无检可做（四路搜索过程即证据）；
适配时 ShareRule=声明文本判定。

**入站规则补全（用户发现·forums.php topicid=3）**："严禁转载其他PT站
禁转的资源"——乌托邦出站开放+**入站要求上游非禁转**：与我们系统级
R7（源站禁转必检）完全同构（站方背书）；且构成 **R7 禁转二级传递性**
（12.5-b 讨论点⑥）的具体案例——乌托邦转载种的"转载自 URL"指向源头
站，源头禁转约束传导（该 URL 兼具传递性检查线索价值）。

### adapter 音乐特化实施（2026-11-05·§59.469 附五）

**策略=enrich 模式**（`internal/adapter/adapter_utopia.go`）：基础版面=
标准 NP（Engine 提取照走），音乐字段后处理 enrichUtopiaMusic 挂
getTorrentDetailInner 两分支（内部域判定·非乌托邦零开销）。

- 标题双形态解析：三段（官组·中段粘连）/四段（转载·保种区尾巴剥+
  繁体分軌归一简体）；AudioMode 词表皇后同款词序
- 字段行双/单引号 class 兼容；基本信息嵌套 b 键值串（nbsp 跳过）
- 17 字段中 14 填：TrackList 不提（无结构化区·desc 原样）/
  OtherVersions 站方无此字段/HasCue=副标题规格链 grep
- 测试锚三枚：TestUtopiaOfficialMusicMeta（官组三段）/TestUtopiaTransfer
  MusicMeta（转载四段+Origin URL）/TestUtopiaDomain

### 待摸遗留

- ~~禁转种实证~~ ✓（2026-11-05 四路实证无禁转体系——上方节；**修正：
  upload tags[4] 值 1=禁转——发种人自主标记面存在（站方不强制）**）
- 类型枚举全集（HQ Audio 之外：LP/SACD…分类树）
- ~~发布侧（表单/值集——音乐目标站适配期）~~ ✓（2026-11-07 摸底——下方节）
- RSS/搜索全量形态（素材=0·适配时摸）

---

## 二、发布侧摸底（2026-11-07·§59.485 音乐发布链第一站）

### 发布资格

- User 级：仅周六 12:00 ~ 周日 23:59 可发布；Power User+ 直发
- uplver=yes 匿名发布（§59.485 定案：系统级默认匿名——music/影视统一）
- 出站开放（"独占"资源除外）·入站要求上游非禁转（forums topicid=3
  ——与我们 R7 同构）

### 表单契约（takeupload.php·multipart·§59.485 九定案）

| 字段 | 值集/规范 | 定案 |
|---|---|---|
| file | .torrent | 必填 |
| name | 四段式 `歌手 - 专辑 年份 - 格式 分轨 - 来源`（来源可省）站方例：`Henry Mancini - Essential Henry Mancini 2024 - FLAC 分轨 - Redacted` | MusicMeta 重组·album 空标题中段兜底·AudioMode 简繁归一·有官组填组名段无则省 |
| descr | 简介 | `[quote]转载{源站}资源，感谢[/quote]`+[img]pixhost 封面[/img]+正文（剥首图+截 LOG 段——log 走 logtexts 独立通道） |
| type | 408 HQ Audio（种子区）/406·403（特别区 MV） | 音乐恒 408 |
| medium_sel[4] | 8 CD/9 SACD/10 DVD/11 VCD/12 Other | Media 值映射 |
| audiocodec_sel[4] | 19 值（1 FLAC/8 PCM/WAV/11 DSD/DSF/16 Hi-Res PCM…） | Format 值映射 |
| team_sel[4] | 3 Musopia（官组）/5 Other/HDS/CHD/WiKi | 组名识别→命中选·其它 Other（MusopiaBAK 不配——淘汰组） |
| language_id | 13 值（40 纯音乐/41 华语/42 欧美/43 日韩…） | 地区推导 |
| tags[4][] | 44 checkbox（**1 禁转/2 首发**/6 港台/4 大陆/5 欧美/7 日韩/流派 9-43/30 Redacted/31 OpenCD/36 Orpheus/37 Jpopsuki/47·44 加速） | 流派+地区+来源站（有官组才打）；禁转(1)恒不勾（R7 上游拦）·首发(2)恒不勾（铁律）·加速恒不勾 |
| small_descr | 副标题 | 规格链 `{Format} / Lossless / {Media} / Log (%) / Cue`（缺项跳过·对齐海豚同款形态） |
| logfiles[] | .log 文件多选 | 不用（文本通道） |
| logtexts[] | 动态 textarea·每框一张 CD | LogText EAC/XLD 头拆 CD·剥皇后 Score 标注行·段数超种内 .log 数站方自动忽略（守恒律） |
| rip_score | number | LogScore 直通+LogText Score 行兜底 |
| resource_count | 合集填·单资源留空 | 首版留空（定案） |
| nfo | NFO 文件 | 不传 |

### log 数量守恒律（站方深度定制）

站方前端 POST /api/parse_torrent.php 服务端解析 .torrent → log_count；
上传 log（文件+文本）总数 ≤ 种内 .log 文件数——超出服务端忽略；种内
无 .log 传了全忽略。

**⚠ 实测修正（2026-11-08 朱孝天案·推翻下方"缺 log 不违规"预判）**：
种内无 .log 时 takeupload 硬拦——"音乐种子包内未找到EAC抓轨LOG文件，
必须附带 .log 抓轨日志才能发布"。log 门=预检必检项（见六节）。

### 官种副标题实证（三颗 ZHENGLIFAN@Musopia）

副标题恒=「内嵌专辑封面」——官组模板文案（音频文件 tag 内嵌图·文件
层信息·与转发无关·页面无此数据）。我们转发副标题=规格链（转载种
实践实证：`FLAC / Lossless / CD / Log (100%) / Cue`）。

### 结构化音乐数据库

站方有艺术家/唱片公司/专辑/曲子/作品/音轨/歌词/乐队独立页+
logcheck.php 日志评分体系（log 质量站内评分输入——缺 log 不违规
但评分吃亏）。

### 预检与 R7

- 预检最小集（§59.485 定案）：R7 禁转拦+reviewed 门
- **log 硬拦（2026-11-08 朱孝天案实证——首颗观察期兑现）**：种内
  无 .log 硬拦·预检门待实施（见六节）
- R7 矩阵：皇后=ShareRule 文本/海豚=VH∨scene 拦·双 false 放
  （§59.485 torrentgroup 补采）/乌托邦=放
- 站方规则页：rules.php 空模板·forums 参数 GET 被拦（空壳响应）
  ——无集中版规·规范=表单提示+已发种实践

---

## 三、发布侧实测与首颗复盘（2026-11-08·§59.491）

| 事件 | 行为 | 判定 |
|------|------|------|
| 首颗 Old Crow（tid=25174） | 上传 120s 超时·请求到达站方发布成功·超时复查同抖动未命中→failed→重试在线 dedup 命中→duplicate→PushOnly 补推→tr 校验 100% 做种 | 假失败三步恢复全系统内闭环 ✓ |
| Monk（tid=15141） | 在线 dedup 命中站方老种 | 防重正确 ✓ |
| 朱孝天 | 站方拦截：种内无 EAC log 硬拦 | 预检缺口→六节 log 门 |

- 上传链路：12→musopia 直连慢（首颗 120s 卡线）；用户已配代理
  （2026-11-08）·后续观察
- pieces-hash 反查（dedup）：**实证可用**（两案在线命中）

## 四、实时值集复核（2026-11-09·刀1·label+value 双源实时 HTML）

与 2026-11-07 摸底快照逐值比对——**全槽位零漂移**：

| 槽位 | 值数 | 复核 |
|------|------|------|
| tags[4][] | 44 | 一致（1 禁转/2 首发/31 OpenCD/30 Redacted/37 Jpopsuki/47·44 加速…） |
| medium_sel[4] | 6 | 一致（8 CD/9 SACD/10 DVD/11 VCD/12 Other） |
| audiocodec_sel[4] | 19 | 一致（1 FLAC/8 PCM/WAV/11 DSD/DSF/16 Hi-Res PCM/17 MQA/18 LDAC…） |
| team_sel[4] | 7 | 一致（3 Musopia/5 Other/6 MusopiaBAK 不配=淘汰组） |
| language_id | 13 | 一致 |

## 五、parse_torrent.php 契约（2026-11-09 刀1 摸通·站方沙盒端点）

```
POST /api/parse_torrent.php
multipart 字段名: torrent（tfile/file 均拒——"未上传种子文件"）
鉴权: cookie（c_secure_pass）
返回 JSON:
  name        种子名
  size        总字节
  file_count  文件数
  log_count   .log 文件数（log 硬拦预检权威源）
  has_cover   封面在场
  files[]     逐文件 {path, size, ext}
```

实测（tid=25174 自己的种）：log_count=1·files=5×flac+cue+log+Cover.jpg。

限制：手工构造的 bencode 被拒（"Invalid integer format"——严格解析
器·非标准形态敏感）；真实 .torrent 正常解析。**只解析不产生种子
=安全可调**（DryRun L4 站方校验通道）。

## 六、预检规则集（渐进式积累——音乐站规则资产）

影视站规则=一次性摸齐；音乐站=**拦一次记一条**回灌积累：

| # | 规则 | 来源 | 实施状态 |
|---|------|------|---------|
| 1 | log 硬拦：种内 .log 数=0 拒发 | 朱孝天案拦截文案 | 待实施（bencode 本地数+parse_torrent 双验） |
| 2 | dedup：pieces-hash 反查站方已有 | 机制已有（§59.470/§59.485） | ✓ 已在线 |

回灌机制：每次 failed 上传的站方拦截文案 → 本表新增（人工确认入册）。

## 七、DryRun 设计（四层·§59.492 刀3 实施）

```
L1 表单全览   dry_run_ok Form+Tags（§59.485 已有）
L2 值集校验   组装值 vs 四节实时值集表——miss 即红
L3 站规预检   bencode 数 .log（六节规则集）+ reviewed 门 + R7
L4 站方解析   parse_torrent.php（五节契约）——log_count/has_cover 双验
```

## 八、验证矩阵与素材矩阵（刀4）

素材形态全覆盖 DryRun：整轨/分轨/有 log（预期过）/无 log（预期 L3 拦）/
多 CD（logtexts 多框）/大合集。

## 九、适配流程对照（§59.450 标准流程·重走定案 2026-11-09）

| 步骤 | 状态 |
|------|------|
| ① 摸底 | ✓（一/二节）+ 刀1 实时复核（四/五节） |
| ② 方案落盘 | ✓ 本文档（重组标准结构） |
| ③ 获取链 | ✓（§59.469 附五 enrich 模式） |
| ④ 发布链 | ✓ 翻译层（§59.485 刀2）；预检 log 门=刀2 待实施 |
| ⑤ 配置开闸 | ✓ migration153 |
| ⑥ 验证矩阵 | DryRun 四层=刀3；素材矩阵=刀4 |
| ⑦ 发版 | ✓ 持续（→v0.0.1384） |
| ⑧ 首颗发布 | ✓ tid=25174 发布+做种闭环（三节） |
| ⑨ 回归+回写 | 进行中（本文档重组=刀1） |
