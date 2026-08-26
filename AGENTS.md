# PT-Forward 项目 Agent 指令

## Skills 索引（按需加载，正文在 .claude/skills/）

| skill | 用途 | 触发时机 |
|-------|------|---------|
| `ptf-dev-protocol` | 开发前协议：设计文档优先 + 复用优先 | 任何开发/修复/优化动手前 |
| `ptf-quality-gate` | 灵魂四问 + 回归审核 + 强制清单 | 任何代码改动后、commit 前 |
| `ptf-deploy` | 编译部署脚本 + 打 tag 发布 | 部署到 29、发版时 |
| `ptf-infra-notes` | mpv 编译/截图引擎/采集策略/CookieCloud/Playwright | 涉及基础设施时 |
| `pt-idx-ops` | PT-IDX 云端指纹服务运维 | 操作 PT-IDX 时 |

> ⚠️ 强制流程内容（铁律/灵魂四问/强制清单）已内嵌本文件，**必须执行**；skills 是其扩展细节，加载 skill 不替代本文件的强制项。

## 开发流程铁律（最高优先级，v0.0.606 教训）

**任何开发、修复、优化，动手前必须先查设计文档**：

1. `docs/31-模块设计决策记录.md`（grep 关键词定位章节，按需读 50-100 行上下文）
2. `docs/32-站点适配器设计/<站点>.md`（站点行为/标记/规则的权威定义）

```
问题/需求
  ↓ ①先查设计文档
  ↓   有记录 → 按设计实施；实现与设计不符 → 修正实现（不是绕过设计）
  ↓   无记录 → 与用户讨论设计 → 设计落盘 → 再实施
  ↓ ②实施开发
  ↓ ③灵魂四问 + 回归审核
  ↓ ④设计文档更新（实施中的偏差与新发现回写设计文档）
```

**反面案例（v0.0.606）**：extractFlags 用 `Contains("禁转")` 扫声明文本，偏离 §32/keepfrds.md 六章定义的"站点文字标记"权威源；在其上叠加致谢模板（含"禁转PTT"）时未查设计 → 跨词伪命中 + 定向禁转误判 + 误标 3 个种子禁转。修复耗时远超先查文档的成本。

## 每次代码改动后强制清单（不可跳过）

1. **设计落盘** — 改动记录到 `docs/31-模块设计决策记录.md` 或 `docs/32-站点适配器设计/`
2. **灵魂四问** — nil 安全 / 边界安全 / `go vet`+`go test` / 前端构建（如涉及 `web/`）
3. **先提交后部署** — `git commit + push` → 再编译部署
4. **打 tag** — 如需发布新版本（`git tag vX.X.X && git push origin vX.X.X`）
5. **AGENTS.md 更新** — 如任务状态变化

## 当前任务焦点（新会话必读）

**版本**：v0.0.631（已发布）。

**当前主线**：**§59.62-111 编辑器打磨 + 标签/资产体系 + 发布预览收官（v0.0.705-767，63 版）**。两条线：①编辑器细节——§59.62 本地 MI Complete name 去路径（v0.0.705）、§59.63 簇截图链接缓存·观察期复用（v0.0.706，两段验证 148 簇：清簇重取截图环节 50min→秒级零 mpv 零上传；settings `screenshot_cache_days` 默认 30 天）、§59.64 组名提取剥尾部促销标记（v0.0.707，克隆站 NBSP+`[2X 50%]` 连坐修复）、§59.65 获取降级链重排（v0.0.708，直达失败不再内嵌 IYUU 兜底抢跑：①直达→②同站搜索→③②a簇内其他comment直达→④IYUU末位兜底）、§59.66 quote 引用剥站内布局图（v0.0.709，trans.gif 小黑点）、§59.67 引用三细节（v0.0.710，嵌套引用重复采集修复 topLevelQuotes）、§59.68 致谢模板去"转自来源"段（v0.0.711，中英同步）、**§59.72 前端资产 cp 陷阱事故**（v0.0.720，手动 `cp -r` 未带 rm 致 frontend/dist 拷入 dist/dist 子目录、8/21 起前端资产停滞——**铁律：前端部署永远走 build-frontend.sh，禁止手动 cp**）。②**标签推导体系（dict 49 词条 9 分组）**——§59.69 高码/高帧五判据（v0.0.712，MI Overall≥15Mb/s@≥4K/≥9Mb/s@<4K、Frame rate≥60 且 59.940 不算）、§59.70 高分（v0.0.713，豆瓣评分≥8.0 + PTGen 落库后 t2 重推 refreshInferredTags）、§59.71 B1 分集/合集/完结（v0.0.715-716，藏宝阁 7.6 判据+neg 抑制+互斥组；PT5/PT7 1168 种子名实测零漏判，Ep2861 四位集数边界修复）、§59.72 B2 连载/大包（v0.0.717，ongoing=分集&&!完结&&!合集 组合判据、big_pack=size>1TB AGSV 硬规则，TagInput 加 Size/Statement 字段）、§59.73 特效字幕+直采标签归一（v0.0.718，statement_raw 第二源——description 是 PTGen 剧情简介不承载声明；NormalizeTagDisplay 显示名→canonical 21 词条）、§59.74 MergeTags 合并单点（v0.0.719，跨源冲突仲裁于合并结果：直采惰性归一+直采在前互斥直采赢+ApplyTagRules 输入序）。§59.75-77 资产三补：§59.75 region/genre 系统资产域（v0.0.721，dict/region.json 27 国+dict/genre.json 30 豆瓣词+persistPTGenSource 接入遗漏修复——ptgen_source_json 落库 Region/Genre，Tab1 产地/类型只读 tag 展示 {keys,labels} 双形态；30 站地区表单五型承载备忘：source_sel/processing_sel/region_id/tag/category 同名字段不同站语义不同）、§59.76 v1.05 资产四断点（v0.0.722，REPACK/PROPER 合并进 EditionInfo“步骤3”注释承诺落地+MoC/WAC 发行商品牌 pattern+audio_tracks 列 migration 22+Tab1 行；分发方行为伪缺口）、§59.77 评论音轨副标题扣减（v0.0.723，AdjustCommentaryTracks `[双|三]?评论音轨` 提取扣减+MI≤1 防御+下限钳 0——v1.05 XAudio 语义闭环；243 实证 68 部）。**v1.05 十八字段四层（提取/落库/展示/重组）全通**；系统资产三支柱：tag 49/region 27/genre 30 词条 + dict 11 域。§59.78-111 收官批次：§59.78-80 声明区三连（MI 碎片结构化剔除 div.mediainfo/音轨明细/空声明前导空行）、§59.81 发布预览六点重构（PTNexus/easy-upload 融合：v1.05 全字段/标签着色/四段渲染/源码切换/截图放大）、§59.83-84 站点媒介合成修复+媒介写法三缺口（3D 点分隔/HDDVDRip/裸 DVD）、§59.85 标签九词条+TagSelector 弹窗化、§59.86-92 预览页七卡片化+精修+滚动完成门槛（scroll 祖先监听）、§59.93-94 reviewed 簇同步三写点统一（clusterKeyOf/syncByIDs 公共方法）、§59.95 死代码清理 452 行、§59.96-98 主标题统一边界锚（extractBoundaryAnchor：季集>年份锚、锚左侧全片名、右侧技术区不回填——2046 双年份/Blade2049/HKG 消歧）、§59.99 运营标记过滤（[中性种子(NL)]）、§59.100 BBCode size 档位渲染、§59.101 预览键盘滚动（scroll 宿主 focus）、§59.102-104 预览零计算原则（PUT/GET 同源+PTGen 缓存兜底）、§59.105 站点标签体系全量审计（140 显示名六类分流+语言族补缺）、§59.106-110 标签显示名后端权威化（tag_labels 双形态+utils/tagDisplay 公共单点+28 词条 standard_key 补齐）、§59.107-108 titleComponents 消费端完整性（audio_tracks/medium 漏映射）、§59.109-111 语言组判据补齐+10bit 维持+dubbed 删除（语言组定态六类：国/粤/英/日/韩/双语）。**dict 终态：tag 57 + region 27 + genre 30 词条。下一步：发布页重构（tag_config 激活，§56.22）grilling Round 3（Q7-Q12）**。前主线（v0.0.694-695）**：Resolution 闸门修复（时空奇旅错配根因）+ 243 全量 148 组重取六 Tab 全绿 + §59.47 同族写侧第五处换行残留修复（model.FormatScreenshotColumn/NormalizeScreenshotColumn 单点写手，四处替换）。下一步：§59.61 comment 直达收官验证（v0.0.697-699 管道/解析器+层选+簇共享全实施，②a 直接生效定案；剩：148 组端到端回归验证）+ 发布页重构 grilling Round 3。字典（§59.35）已完成后回线，近期完成：§59.42 海报可信图源白名单+PTGen fallback（doubaninfo/pixhost 家族+pixho.st，Tab2 闭环）；§59.43 全站字节显示统一（KiB/MiB/GiB+两位）+ ESLint 硬约束防私有副本；§59.44 资源视图（虚拟种子）——(client,path,name) 三元组键 + ResourceResolver 正式接口，修复 49% 编辑 404（158/158 组全通）；§59.45 Tab4 简介对齐 PTGen 为准——朋友站 kdouban 框逆向提取（139 MI 污染修复面，8 发布者自贴保留）；§59.46 Tab4 PTGen 四项修复——doubaninfo format 解析/清空 BBCode 缓存/展示兜底/kdouban 空 body 回退（行业基准：PT-Gen 数据类型 = format BBCode，examples 四项目实证）；§59.47 screenshots 写读格式分裂修复（ParseScreenshotColumn 四消费点，Tab3 数据层）；§59.48 keepfrds 截图 URL 三层展开（缩略图 403 → imgfetch 代理 → picgo 原图直链，采集层归一；v0.0.675 补丁：NormalizeImageURL 保留 path 大小写——base64 段被 lower 破坏是二次根因）；§59.49 采集时图片探活——海报死链清空（§59.42 链③改语义）+ 截图清死留活/全死全清，字段列诚实化（获取时点存活状态即绿红依据）。**Tab3 打磨进行中**（243 为本地环境）：§59.50 mpv 按钮接线 + §59.51 后台截图任务+轮询已完成（v0.0.678：长任务脱离 HTTP 生命周期——243 实测上传 4/5 context canceled 根因修复；API 冒烟守卫/进度端点通过，待 243 OTA 端到端）；§59.52 预览全量化（纵向平铺+点击放大）+ 按钮文案简化；§59.53 auto 截图策略重构七点定案落地（白名单逐张保留/非白名单转存/差额 mpv 补足/无图全量/远程只转存无图留空/checkRequiredFields ≥3 门槛/采集链双跑）；§59.54 批量粘贴+恢复引用完成（三形态解析/转存前快照还原，纯前端）。v0.0.684 补丁：auto 截图上限 8 张（129 张异常案例）+ ptgen_dead 探活语义核对确认（v0.0.676 已正确）。243 全量 148 种子已获取验证（五策略形态全触发/六 Tab 数据完整/142+6 重试全通）；§59.55 获取链 PTGen 双字段消费（一次调用产海报+简介——Tab4 落库与 PTGen 查询不一致的根因修复，29 实测 description kdouban 484 字 → format 1735 字）；§59.56 本地 MI 落库列名笔误修复（v0.0.686-687 同族四处终清：fetchSingleTorrent + handleSaveSeedData 两处列名 + 前端静默 catch——243 实测 760 次静默失败、148 组本地 MI 全灭根因；GORM Updates/Update 必查 Error 并入规范）；§59.57 截图探活与策略竞态修复（v0.0.688：purge 内联 strategy 前序串行化——原双 goroutine 竞态致 strategy 读 purge 前死链→same 早退→永不 mpv 补图，ptpimg.me 全死链 8 组复现）。**243 全量 148 组终态全绿：MI 148/148 local、截图 148/148 ≥3 张——Tab3 端到端总验证完成，编辑器六 Tab 全部收官**。遗留观察：批量链并发截图限流已实施（§59.58 信号量 5 路，v0.0.689）。编辑器六 Tab 已闭环：Tab2 海报/Tab4 简介/Tab5 MI。**Tab 3 已收官（§59.47-58 九轮：数据层三重修复/mpv 接线/后台任务/预览平铺/auto 策略/批量粘贴/恢复引用/竞态修复/并发限流，243 全量 148 组终态全绿）。下一步：预览→审核闭环全流程 → 发布页重构（tag_config 激活，§56.22）。**

**支线完成**：幽灵种子巡检 v2（§59.31）；Token Registry P1-P3（§59.27）；清 seen 重推（§59.33）；侧栏信息架构重排（§59.32）；Remux 规格解析修复（v0.0.629）；Tab1 片源写法与 Encode 判定 + DOM key 归一化（§59.34，v0.0.632-634，两轮审计）。

**支线插曲（v0.0.642-643）**：compliance 关键词层防误伤（§56.2x 修正）——"The Pornographers" 被 "Porn" 子串误标系统禁转；修复 = ASCII 关键词词边界匹配（compliance.MatchKeyword 单一入口，checker/reseed/pipeline 三套统一）+ 中文误报排除补实现（成人+教育/高考/大学/学院，设计稿落地）。已知边界：xXx 由 qui 层白名单覆盖、Adult Swim 待案例。

**遗留**：发布页重构（含 tag_config 激活，§56.22，下一步主线）；MarkStatus 跨订阅写（§59.17 违反，预存在）；runAnalyze 分级（读 DB 跳过重拉）；integration TestE2E_SeedingFullChain flaky（eng2.Start 竞态，§59.35 P3 记录）。已完结：批量截图并发限流（§59.58 信号量 5 实施，观察期无新问题，2026-08-26 关闭）。

### ✅ 已完成（v0.0.228 → v0.0.547）

**Phase 1（v0.0.228~v0.0.310）：转载/发布/标题重组/技术特征模型** — 详见 §56 全系列。

**Phase 2（v0.0.522~v0.0.547）：辅种/孤儿/RSS/配置表**

- ✅ **辅种误匹配修复栈**（§59.1~§59.9）：
  - ValidateInjection 公共方法 + 孤儿恢复接入（v0.0.522）
  - 续集号比较 extractSequelNumber（I~IX，v0.0.528~v0.0.529）
  - 标题关键词词长 4→3 + 停用词（v0.0.531）
  - truncation retry 组名泄漏修复（v0.0.541）
  - negative cache 确定性失败 + 覆盖→追加 bug（v0.0.530）
- ✅ **孤儿恢复重构**（§59.14~§59.16）：
  - ClassifyTorrent 统一分类系统（movie/tv_series/music + season_pack/partial_pack/single_episode）
  - 按分类选择搜索策略 + 文件级大小匹配修复（v0.0.537~v0.0.539）
  - 同站扩展：主恢复命中后在同一站点搜索其他集（v0.0.540）
  - 剧集孤儿修复（目录总大小→单集文件大小，v0.0.537）
- ✅ **RSS recheck 状态机重构**（§59.12）：
  - 移除 rss_recheck_waiting 全局任务（v0.0.523）
  - 废弃 NonFreeRecheck/PendingScoringQueue/skipped_nonstate（v0.0.524）
  - FreeWaitRecheckSec/MinRemain 生效 + wait-queue API（v0.0.525~v0.0.527）
- ✅ **配置表合并**（§59.13）：
  - seeding_client_configs + download_client_configs → 统一表（阶段 A/B/C，v0.0.535~v0.0.546）
  - Role 字段区分 seeding/download，评分清理只对 seeding 生效（v0.0.534）
- ✅ **RSS seen 每订阅独立**（§59.17）：UNIQUE 约束加 subscription_id + IsSeen 每订阅 + MarkSeen UPSERT（v0.0.543~v0.0.545）
- ✅ **DryRun 缓存修复**（§59.18）：试运行从 rss_torrent_seen 恢复缓存的免费状态（v0.0.546）
- ✅ **死代码清理**：删除 ExtractType（v0.0.547）、海胆 pieces_hash_api 修正（v0.0.542）

### ✅ 种子配置页（§59.19~§59.21，全部完成）

**Phase 3 基础设施（v0.0.548~v0.0.551）**：
- ✅ torrent_snapshots 快照表 + syncer 扩展 + snapshot-paths API（v0.0.549）
- ✅ torrent_metadata 加 14 TechProfile 平铺字段 + category/form（v0.0.550）
- ✅ CrossSeedPanel maintenanceOnly prop + fillFormFromPreset + saveOnly（v0.0.551）
- ✅ SeedConfig.vue 种子配置页框架（mock 数据）（v0.0.548）

**后端实施（v0.0.552~v0.0.553，20/20 完成）**：
- ✅ torrent_metadata 加 statement + bdinfo 列 + 22 个无映射站点关闭 is_source（migration #7）
- ✅ Detect() Step 2 加 cookie 检查 + SelectFetchSite() 新函数（制作组优先）
- ✅ GET/PUT /publish/seeds 列表/读写 API（含 compliance + 9 字段校验 + 5 态标注）
- ✅ GET /downloads/snapshot-unconfigured + POST batch-fetch 异步批量获取 + 进度轮询
- ✅ GET/PUT /publish/fetch-priority（获取数据站点优先级）
- ✅ GenerateThanksQuote 加 {group_name} + 渲染器始终添加致谢
- ✅ handleRefresh('mediainfo') 联动 BDInfo + handleRefresh('intro') 不再返回 subtitle
- ✅ CheckPublishEligibility 增加源站映射检查（第 0 层合规）
- ✅ persistAnalysis 写入 statement + bdinfo
- ✅ MainDataCron 列名 bug 修复 + /seeding role 过滤（v0.0.553）

**前端实施（v0.0.552~v0.0.555，17/17 完成）**：
- ✅ SeedConfig.vue 接真实 API + snake_case + 5 态标签 + 禁转/无映射只读（v0.0.552）
- ✅ CrossSeedPanel 六 Tab 改造（18 字段只读 / 海报 / 截图左右分栏 / 简介 / 媒体信息 / 已过滤声明）（v0.0.552）
- ✅ CrossSeedPanel 维护模式 GET/PUT API + 编辑→预览→完成三步流程（v0.0.554）
- ✅ BatchFetchPanel.vue 批量获取弹窗 + 进度轮询（v0.0.552）
- ✅ BBCode→HTML parseBBCode 工具函数（v0.0.554）
- ✅ 发布预览页面（方案 A：保存即预览）（v0.0.554）
- ✅ 截图左右分栏布局（URL 列表 + 大图预览）（v0.0.555）
- ✅ 路由 /publish/exclusions → /publish/rules（4 Tab 含声明过滤 + 小组映射只读）（v0.0.552）
- ✅ 导航简化（隐藏一站多种/一种多站）（v0.0.552）

**回归审核修复（v0.0.556）**：
- ✅ batch-fetch runBatchFetch 加 defer recover()
- ✅ Migration #8 错误不再忽略
- ✅ handleRefresh('mediainfo') BDInfo 改调 bdinfoScanner.ScanIfBD

**is_local 本地发布 vs 转种上盒（§59.21，v0.0.557）**：
- ✅ ClientConfig 加 IsLocal bool（用户添加下载器时必填）
- ✅ batch-fetch：is_local=true 追加本地 mediainfo，is_local=false 仅源站采集
- ✅ GET API 通过 client_id 查 is_local 返回前端（后端是唯一真相源）
- ✅ handleRefresh：is_local=false 时从源站重抓（mediainfo/screenshots）
- ✅ 下载器编辑表单加 is_local 必填 radio（本地发布/转种上盒）
- ✅ CrossSeedPanel 按钮文案根据 is_local 切换

**暂缓**：
- ⏸ 遗漏 3（一种多站/一站多种重构），待种子配置页有数据后再讨论

### ✅ 野马 OpenAPI 适配（§59.22，v0.0.558~v0.0.560）

- ✅ pieces_hash 接入（`SearchByPiecesHash` 调 `/openApi/torrent/fetchTorrentIdWithPiecesHash.json`，APIKey 认证）
- ✅ generateDownloadKey 迁移 POST OpenAPI（优先 APIKey + fallback Cookie）
- ✅ fetchBasicInfo GET→POST（原 GET 已废弃）
- ✅ 搜索迁移 Open API（`/openApi/torrent/fetchOpenTorrentList.json`）
- ✅ migration #9-#12：supports_pieces_hash_api + auth_type=apikey
- ✅ updateSiteRequest 补 SupportsPiecesHashAPI 字段（camelCase + nil 守卫）
- ✅ Cookie/APIKey 双输入框显示（show_cookie override）

### ✅ 下载器连接管理修复（§59.22，v0.0.561~v0.0.562）

- ✅ Reload 用 `context.Background()` 替代 `r.Context()`（3 处 create/update/delete）
- ✅ update handler 改用 `ReloadClient(name)` 只重连被编辑的客户端
- ✅ is_local 字段持久化遗漏修复（Updates map 缺 is_local 列）
- ✅ JSON 命名规范加入 AGENTS.md + 强制清单

### ✅ P2 snake_case 统一（v0.0.563~v0.0.567）

- ✅ 后端 4 个文件 34 个 struct 106 个字段 snake_case → camelCase
  - publish_torrents_handler.go / manual_forward_handler.go / metadata_handler.go / download_handler.go
- ✅ 前端 types.ts + publish.ts + downloads.ts + image-host.ts API 层全量对齐
- ✅ 前端 7 个 Vue 组件 API 调用参数 + 响应类型 + 模板引用全量对齐
- ✅ 5 轮回归审核，修复 13 处遗漏（vue-tsc 无法检测的运行时 JSON key 不匹配）
- ✅ 保留 snake_case：Go model JSON tag / raw map 响应 / DB 列名 / URL query param

### ✅ 野马 pieces_hash bencode 辅种（§59.24-§59.25，v0.0.570~v0.0.579）

- ✅ 辅种移除"源站"概念（§59.24，与发布业务解耦）
- ✅ 野马 pieces_hash bencode 算法分支（§59.25，SHA1(bencode(info.pieces))）
- ✅ ContentFingerprint 加 PiecesHashBencode + 懒计算回写
- ✅ 引擎按 adapter.Framework() 选择 hash 格式
- ✅ verifyL0Size 搜索失败时降级放行（方案 A，注入阶段 ValidateInjection 兜底）
- ✅ 端到端验证通过（249 环境，4 个种子 matched + injected）

### 端到端验收状态

| 端点/功能 | 状态 |
|------|------|
| 辅种引擎（L0/L1/L2 + 注入校验 + 续集号 + 标题关联性）| ✅ |
| 孤儿恢复（分类 + 文件级搜索 + 同站扩展 + 注入校验）| ✅ |
| RSS 订阅（每订阅独立 seen + FreeWait + DryRun 缓存）| ✅ |
| 配置表统一（seeding + download 合并，阶段 A/B/C 完成）| ✅ |
| 种子配置页（基础设施 + 设计）| ✅ |
| 种子配置页（前后端实施）| ✅ v0.0.552-555 |
| 种子配置页 is_local（本地发布 vs 转种上盒）| ✅ v0.0.557 |
| 野马 OpenAPI 适配（pieces_hash + 下载 + 搜索 + 认证）| ✅ v0.0.558-560 |
| 下载器连接管理（Reload context + 单客户端重连）| ✅ v0.0.561-562 |
| P2 snake_case 统一（106 字段 + 5 轮审核）| ✅ v0.0.563-567 |
| 辅种移除源站概念 + 目标站不按 isTarget 过滤 | ✅ v0.0.570-571 |
| 野马 pieces_hash bencode 辅种（端到端验证通过）| ✅ v0.0.576-579 |

**关键事实**：
- 开发环境 29（systemctl --user pt-forward，端口 8765）
- 多生产环境 Docker（249/243/65/12/pt30），端口 8765
- 辅种引擎三层匹配：L0 pieces_hash、L1 fingerprint、L2 search_verify
- 孤儿恢复：ClassifyFromDir 分类 → 按类型选择搜索策略 → 同站扩展
- 辅种引擎与孤儿恢复共享 VerifyMatchWithTruncationCheckAndSource + ValidateInjection
- 配置表合并后统一表 seeding_client_configs（Role=seeding/download 区分）
- RSS seen 去重已改为每订阅独立（UNIQUE: site_name + torrent_id + subscription_id）
- 下载器是去重的唯一权威（同 info_hash 拒绝），RSS 引擎不做跨订阅去重
- 种子配置页六 Tab：种子详情/海报与声明/视频截图/简介详情/媒体信息/已过滤声明，仅截图和简介可编辑
- 种子 5 态：禁转(flags)/系统禁转(compliance)/无源站映射/已审核/待审核+配置不完整
- 源站约束：制作组不在 release_group_mappings 的种子不可转发，无映射站点不可开启 is_source
- 发布预览采用方案 A（保存即预览）：PUT 同时保存+标准化+9字段校验+渲染
- is_local 区分本地发布（本地 mediainfo/mpv）和转种上盒（源站数据引用），后端是唯一真相源
- 野马 OpenAPI 认证：APIKey 优先（Header `Authorization`，无 Bearer），fallback Cookie
- 下载器编辑触发 ReloadClient（单客户端重连），create/delete 触发全量 Reload

**待办**：
- 端到端测试（需有本地下载器的环境验证 is_local=true 全流程）
- 生产环境部署（用户 OTA / docker compose pull）

**完整设计文档**：`docs/31-模块设计决策记录.md` §55-§59.22（67000+ 行，按需读特定章节，不要一次读全文）。

## 前端格式化函数规范（§59.43）

- **禁止页面私有字节/速率格式化函数**：所有体积/速率展示必须 `import { formatBytes / formatSpeed } from '@/utils/format'`（单点维护，防 toFixed 位数/进制/后缀漂移）
- ESLint 硬约束已启用（no-restricted-syntax）：私有 formatSize/formatBytes 同名副本 + toFixed×字节单位拼接形态直接报 error；utils/format.ts 本体豁免；非字节场景 `eslint-disable-next-line` 注释豁免
- 历史教训：§59.26 snake_case、§59.43 字节副本——"知道该统一"不等于"不会漏"，靠机制不靠记忆

## JSON 字段命名规范

| 场景 | 风格 | 示例 |
|------|------|------|
| Go model JSON tag | snake_case | `json:"supports_pieces_hash_api"` |
| API 专用请求/响应 struct（site/downloader 等） | **camelCase** | `json:"supportsPiecesHashApi"` |
| API raw map 响应（publish/seeds 等新 handler） | snake_case | `"info_hash": hash` |
| 前端 TS 类型 | 跟 API（struct 端点 camelCase，raw map 端点 snake_case） | — |

**核心规则**：在已使用 camelCase 的专用 struct（如 `updateSiteRequest`、`siteResponse`、`downloaderResponse`）中新增字段时，**必须用 camelCase**，否则前端反序列化失败。

## 环境信息

- **Go**：`/home/incast/.local/go/bin/go`（v1.25，系统 PATH 中无 go，必须用全路径）
- **Node**：`/home/incast/.local/bin/node`（v22，系统 Node 18 不兼容，必须用此路径）
- **CGO**：后端编译必须 `CGO_ENABLED=1`
- **DB**：`data/pt-forward.db`（SQLite）
- **服务**：`systemctl --user restart pt-forward`（用户级 systemd 服务，端口 8765）
- **开发环境 29**：10.0.0.29，Agent 唯一可操作的机器（编译/测试/部署/DB）
- **生产环境 249**：10.0.0.249 Docker（docker compose），端口 8765，**禁止 Agent 直接操作**
- **pt30 环境**：pt30.ranfish.uk（192.168.100.30）Docker，**禁止 Agent 直接操作**
- **65 环境**：10.0.0.65 Docker，**禁止 Agent 直接操作**
- **243 环境**：10.0.0.243 Docker，SSH root/~/.ssh/xsy，**禁止 Agent 直接操作**
- **12 环境**：10.0.0.12 Docker，**禁止 Agent 直接操作**
- **PT8 环境 242**：10.0.0.242 Docker，**禁止 Agent 直接操作**
- **22 环境 22**：10.0.0.22 Docker，**禁止 Agent 直接操作**
- **代理**：`http://10.0.2.5:7897`（curl/docker pull 等访问外部网络时可用）
- **Docker**：`sg docker -c "docker ..."`（当前用户不在 docker 组，需 sg 切换组）

## 通用规则

- **语言**：与用户沟通用中文
- **设计决策**：所有设计决策记录到 `docs/31-模块设计决策记录.md`
- **敏感信息**：禁止在本机或云端保存 cookie、passkey、apikey、token 等敏感信息
- **适配器文档**：放在 `docs/32-站点适配器设计/` 目录下
- **站点数据**：官组命名、规则等站点原始数据必须原样写入，不能杜撰
- **Git 提交**：禁止提交 `data/`、`PT0/`～`PT8/`、`*.torrent`、`logs/`、`*.db`（详见 `.gitignore`）
- **删除代码后**：必须跑 `go vet ./internal/... ./cmd/pt-forward/...`（**不要**用 `go vet ./...`，`cmd/verify-pieces-hash` 有已知冲突会报错），确保测试文件不引用已删符号
- **编译部署前**：必须灵魂四问、回归审核，然后再进行编译和部署
- **版本号**：编译命令必须包含 `-X main.version=$(git describe --tags --always --dirty)`，源码默认值为 `"dev"`
- **⚠️ 环境隔离铁律（最高优先级，违反=事故）**：
  1. **开发只在开发机（29）上进行**：所有代码编写、编译、测试、部署仅在开发环境 29 执行（`systemctl --user restart pt-forward`）。
  2. **可以 SSH 到生产环境查看状态**：可以 SSH 到生产机（249/242/22）查看日志、磁盘空间、Docker 状态、DB 数据等（只读诊断）；可以 scp 二进制到生产环境；但**禁止**：在生产环境编译、在生产环境直接修改代码或数据。
     - **⚠️ 部分 Docker 生产环境 scp 不可用**（exit=1），改用 SSH 管道传输二进制：`gzip -c pt-forward | ssh <user>@<host> 'cat > /tmp/pt-forward.gz && gunzip -f /tmp/pt-forward.gz && chmod +x /tmp/pt-forward'`，然后用 `sudo docker cp /tmp/pt-forward <container>:/usr/local/bin/pt-forward && sudo docker restart <container>` 部署。
  3. **生产环境由用户自行更新**：生产环境的 Docker 镜像更新（`docker compose pull && docker compose up -d`）由用户自行执行。Agent 的职责是提交代码、打 tag、推送（触发 CI/CD 构建镜像），不负责生产部署。
  4. **Agent 可以做的**：在 29 上编译部署验证 → commit + push → 打 tag → 告知用户生产环境需更新。SSH 到生产环境做只读诊断（磁盘/日志/DB 查询）。
- **⚠️ 部署铁律（最高优先级，违反=事故）**：
  1. **先提交后部署**：`git commit + push` 必须在编译/部署**之前**完成。代码必须在 git 历史中先于部署生效。**禁止先部署后提交**。
  2. **部署检查清单**（每次部署前逐条过一遍，不能跳过）：
     - [ ] 灵魂四问全部通过？
     - [ ] `git status` 是否有未提交的改动？→ 有则先 commit + push
     - [ ] 需要打 tag 吗？→ `git tag vX.X.X && git push origin vX.X.X`
     - [ ] 本次是纯后端改动？→ 不需要 vite build
     - [ ] 本次涉及 `web/`？→ 必须 vite build → cp dist → go build（三步缺一不可）

## 方法论：复用已有基础设施

- **核心原则**：遇到新问题**先查已有基础设施**是否有可用数据或方法，再决定是否新建。PT-Forward 经过多次迭代，基础设施已经很完善（甚至有冗余），真正的瓶颈往往是**接入遗漏**而非能力缺失。
- **调研清单**（提"新方案"前必须过一遍）：
  1. `grep` 已有的 `Detector`/`Fetcher`/`Searcher`/`Resolver`/`Matcher`/`Service`/`Engine`（命名约定：能力以接口/结构体名暴露）
  2. **对比同类 handler** 的做法（如 `publish_torrents_handler` vs `manual_forward_handler`，一个接入了某设施，另一个可能遗漏）
  3. **查数据表内容**（`release_group_mappings`、`SiteCoverageCache`、`torrent_metadata` 等表可能已有你需要的数据，用 `sqlite3 data/pt-forward.db "SELECT ..."` 确认）
  4. 确认是"**能力缺失**"（需新建）还是"**接入遗漏**"（已有设施没调用）
- **典型案例**（§56.33 讨论）：tid 反查（`reseed.SearchAndVerifyMatch` 已有，orphan/recovery 已调用）、源站识别（`SourceSiteDetector.Detect` 已有，publish_torrents 已接入）、三源字段合并（`metadata.Merge` 已有，handleMerge 已调用）—— 六个看似需要"新建"的问题，实际全是"接入遗漏"。
- **⚠️ 优先使用公共函数（最高优先级）**：遇到功能需求时，**必须先 grep 查是否已有公共实现**，禁止重复造轮子。
  1. `grep -rn 'func.*Xxx' internal/` 查全项目是否已有同名/同义函数
  2. 特别注意跨包重复：`publish.ExtractGroupName` 和 `reseed.ExtractGroupName` 曾经是两份副本，改一处漏一处导致 bug（已合并到 `util.ExtractGroupName`）
  3. **典型清单**：`util.ExtractGroupName`（制作组名提取）、`reseed.ExtractSearchKeyword`（搜索关键词提取）、`reseed.SearchAndVerifyMatch`（搜索+验证匹配）、`reseed.VerifyMatchWithTruncationCheckAndSource`（匹配验证核心）、`reseed.ValidateInjection`（注入前校验）、`util.ClassifyTorrent`/`util.ClassifyFromDir`（种子类型分类）、`util.DetectContentType`（音乐/视频检测原语）
  4. 如果已有函数是私有的且跨包需要，**先考虑提升为公共函数**（改首字母大写或委托到 `util` 包），而非在新包重写一份

## 回归审核第一项（每次审核先查）

- **AGENTS.md 任务焦点同步**：`git log --oneline -10` 的版本推进与"当前主线"描述对照——主线/支线完成项是否已反映？滞后即先补（历史教训：v0.0.631、v0.0.642、v0.0.670 三次同类违规）。

## 灵魂四问（每次代码改动后必须逐条审核）

1. **nil 安全**：所有指针返回值是否检查了 nil？map 查找是否有 ok 判断？type assertion 是否用了 comma-ok 模式？
2. **边界安全**：空输入/空 DB/context 取消/并发锁竞争等边界情况是否处理？`context.WithTimeout` 后是否都调了 `cancel()`？锁是否有嵌套导致死锁风险？
3. **回归通过**：`go vet` + `go test` + `vue-tsc` + `eslint` 是否全部通过？
4. **前端构建**：本次改动是否涉及 `web/` 目录？如果是，是否执行了 `vite build → cp -r web/dist frontend/dist`？`vue-tsc`/`eslint` 只是验证，**不是构建**。`go build` embed 的是 `frontend/dist`，不是 `web/src`。跳过构建 = 前端改动不生效。

## 验证与部署

后端/前端验证部署完整流程与脚本：见 skill `ptf-deploy`（`bash .claude/skills/ptf-deploy/scripts/build-backend.sh` / `build-frontend.sh` / `release-tag.sh vX.X.X`）。关键点：Go/Node 必须全路径（系统 PATH 无）；CGO_ENABLED=1；版本号 ldflags；前端三步缺一不可（vite build → cp dist → go build）；systemd 必须 Restart=always（OTA 兼容）。

<!-- 镜像地址/Dockerfile/本地构建/CI 发布/用户部署：已抽出至 skill ptf-deploy（.claude/skills/ptf-deploy/SKILL.md）-->

<!-- mpv 编译/截图工具引擎/数据采集策略/CookieCloud/Playwright 注意事项：已抽出至 skill ptf-infra-notes（.claude/skills/ptf-infra-notes/SKILL.md）-->

<!-- PT-IDX 云端指纹服务（路径/DB/部署/代码复用/采集规则）：已抽出至 skill pt-idx-ops（.claude/skills/pt-idx-ops/SKILL.md）-->

## 版本管理

- **编译部署前**：必须先提交并推送代码，然后在 commit message 中注明版本号和变更内容
- **禁止先部署后提交**：代码必须在 git 历史中先于部署生效
- **打 tag 发版**：`git tag v0.0.x && git push origin v0.0.x`（触发 Docker 镜像 + GitHub Release 自动发布）
