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
2. `docs/32-站点适配器设计/<站点>.md`（站点行为/标记/规则的权威定义——**涉站点问题必查该站文档**：§59.289 教训——修道院 type 修复时未查 xdy.md 的 type 映射权威表，虽文档未覆盖 standard_key 翻译层，过程仍属违规）
3. 站点表单映射相关改动：README"映射命名铁律"——standard_key 必须对齐 dict canonical，新建映射后全集 diff 比对

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

> **分支任务·仓库安全专项（2026-11-17 起·docs/31 §仓库安全-1~10）**：第三刀已完成（gitleaks 双门+自建比对层·§仓库安全-11）、第四刀已完成（R-02 轮换 CLI+密钥出 DB·e03a4e43·现役环境下次 OTA 首启自动迁移密钥——数据级验证点）。刀序（§仓库安全-9）：①②③④⑤✅（⑤大二进制三留五出·77d79607·CI+Docker 双绿）→ ⑥✅（docs 敏感审查 34 处改写·含第八刀替换清单）→ ⑦✅（脱敏管道 pre-commit 自动消毒·§仓库安全-17）→ ⑧✅（filter-repo 七轮重写：①36值②68值③身份词→<作者>（1510处uid关联·三段式洗牌保护import路径）④IYUU token⑤系统用户名路径+域名⑥系统用户名独立词（§仓库安全-22·联合回归 P1 定案·工具环境变量化 PTF_DB/LUCK_USERNAME）⑦message 层清洗（--message-callback：commit message 的 <user>×6+PT站用户名×2+自托管域名×2·blob 层洗法补全）·CI 首验修复（16处=3真+13误报→allowlist）·gitleaks 全历史 0 findings——用户侧全部销案 2026-11-18：token/ghcr/R-08/PT 操作表 63 站轮换/备份 mirror 删除）→ ⑨✅（发布物审计四项全绿·§仓库安全-20·aedaba1b——strings 10类零·ldflags 纯版本·embed 18 文件零·Docker 层干净）→ **转 public 前置条件全部满足**

**九刀全清单（§仓库安全-9 刀序·2026-11-18 收官）**：
①✅ 删敏感文件+口令清除 ②✅ .env 体系+变量化 ③✅ gitleaks 双门+自建比对层 ④✅ R-02 轮换 CLI+密钥出 DB ⑤✅ 大二进制三留五出 ⑥✅ docs+skills+scripts 敏感审查 ⑦✅ 夹具脱敏管道 ⑧✅ filter-repo 七轮重写（blob 六轮+message 一轮·历史敏感值清零·含 <user> 身份关联）⑨✅ 发布物审计

**待用户收尾**：无——**仓库安全专项 2026-11-18 全部销案收官**（IYUU token 重置+ghcr 可见性+R-08 网络加固+PT 操作表 A/B/C 63 站轮换+备份 mirror 735MB 删除+/tmp 工作产物清理；Support 工单放弃：Free 无通道·自然 GC+值轮换根治）。token：用户填 .env 的 GH_TOKEN（查 CI/第八刀 API 用）。

**版本**：v0.0.1586（§59.563 审计 02 刀A=刀3+刀4 合并〔低风险先行〕——①StartObservingCleanup 接 baseCtx〔原 Background+24h ticker 无 stop 永久 goroutine→避峰/ticker 双挂 Done+清理幂等型派生〕②siteBatch 循环顶接 baseCtx〔与 StopRequested 同语义位·颗内 Background 保留=发布站侧事实动作 graceful〕。v0.0.1585（§59.562 刀4 seeding rootCtx 收敛——审计 01 §3.4 P2：Engine 加 rootCtx〔Start 注入〕+refreshCtx 派生根 ctx〔停机/Stop 双通道收敛〕+transferRecord 改派生巡检 ctx〔transferring 中断回退=既有设计〕·锚两枚〔隔离 Stop 通道纯证 root 派生链+单源存储〕·同族残留两处留 02 篇——**审计 01 篇全案收官（刀1-5 全落地）**。v0.0.1584（§59.562 刀3 后台任务接根 ctx——审计 01 §3.1 P1 四处：NewRouter 加 ctx 首参穿透〔main 根 ctx→两 handler+StatsSync 双实例〕·语义分型三型接线〔循环型=baseCtx.Done 挂循环顶与 stopRequested 同位·graceful 当前颗跑完颗内 Background 保留/幂等型=直接派生停机重查/工件型=截图中止重试〕·锚两枚〔取消链快速终止/nil 兜底〕。v0.0.1583（§59.562 刀2 eventbus consumer 超时边界——审计 01 §3.2 P1：OnPushed 裸 Background 无取消链→NewEventBus 加 baseCtx〔main 根 ctx 接线〕+Task ctx=WithTimeout(60s 命名常量 eventBusConsumerTimeout)·停机 cancel 传播+单消费者超时双保护+超时五字段 Warn·锚三枚〔deadline 到点/停机传播/nil 兜底〕·Close 零调用面发现留 02 篇。v0.0.1582（§59.562 Context 审计整改刀1+5——①client_handler 三处 reload 静默失败→Warn 日志化〔op/client_id/client_name 字段〕②附录A-Context使用规范落盘〔四类 ctx 约定+六规则+评审清单〕+AGENTS 短指针+00-索引状态流转·刀2〔eventbus 60s〕/刀3〔后台任务接根 ctx〕/刀4〔seeding rootCtx〕逐刀待发。v0.0.1581（§59.561 英字误投双根因修复〔圣克莱尔案·站方审核「检查是否有硬英字字幕」质询=pterclub:1435 规则「MI 无英字轨但标签有英字」〕——根因一 cat 层 ensub 裸「双语」宽词探〔中德双语≠英字〕·根因二 dict infer_pattern `英.{0,12}字幕` 12 字符窗跨词伪命中〔英语␣简繁德双语字幕 7 字符·canonical 污染〕；双层刀：窗口收 {0,2}〔真值词形全保·简繁英双语字幕=2〕+ensub 重构 canonical 优先+紧词形·撤双语/中英·双层六锚。v0.0.1580（§59.559 终 代码质量报告两低风险备注收敛——①parseBoolQuery 注释明确「有意放宽」契约〔"1" 也真·非逐字节零变更·防维护者误判〕②__existing__ 哨兵→命名常量 existingStatusSentinel〔两处消费零行为变更·裸字符串清零·struct 升级留再动上传段时〕·**代码质量后续优化报告全部销案**）。v0.0.1579（§59.560 追记二 禁转标签升格系统级定义〔用户权威纠错：非猫站专属——站点/原发布者专属权力声明·转载者在任何目标站无权勾选·与采集侧禁转标志同名不同物〕·AGENTS 行为铁律新增〔首发标签同族〕+四层防线代码实证〔拦截链六处+无 canonical 键+55 站 label-only+特化不投〕·pterclub.md 改实例指向·纯文档刀零代码）。v0.0.1578（§59.560 追记 jinzhuan/diy 权利属性二分〔用户权威定义：禁转=官组原始发布专属·普通用户无权——权利声明类 pr/bim/guanfang 同族不投；diy=资源属性转载者有权有义务勾〕+catTagCheckboxFields 补 diy 块〔双门：DIY/自制关键词+SourceType 碟形态·五键自动投递〕·五态锚。v0.0.1577（§59.560 猫站 team 域产区化+标签 checkbox 投递——站方审核三问题〔地区不一致+缺国语/中字标签〕双根因：team_sel 字段名=制作组实际=地区选择器〔ExtractGroupName→FRDS→Other=8 错投〕+标签 10 独立 checkbox 非数组〔form_fields 无 tags 域→TagArrayFields=0〕；cat_site.go 四函数+executor 两处接线〔动态检测值集含「大陆」特征·PTGen region→team_sel 值·canonical 键+副标词探双通道〕·四锚全绿。v0.0.1576（§59.559 PR-F adapter 三文件细化——generic 1564→四〔主 682+upload_special 539+probe 239+stats 164〕·mteam 1552→四〔主 394+detail 178+slhr 567+upload 476〕·userstats 898→两〔fetch 797+parse 115〕·声明集合 35/51/17 零丢失；**§59.559 全案收官·四轮代码质量专项（P0/P1/P2/§557-559）25 连发全部销案**）。v0.0.1575（§59.559 PR-E 高风险编排特征锚——publish 五锚〔音乐分流/GPW 截图三要件+内容门白名单制/HasMusic〕+reseed 五锚〔titleKeywordState 三态/分辨率+源类型+编码等价/size 舍入窗〕·踩坑四枚〔同名去重/签名/参数序/白名单断言反〕·方法论=写锚先读签名+契约方向不能想当然；PR-F adapter 按需——§59.559 全案销账）。v0.0.1574（§59.559 PR-D query 解析窄契约——三 helper〔parseBoolQuery 宽松 1/true·parseUintQuery/parseIntQuery 缺省回落〕+契约十七态锚+窄落地 5 文件 6 处〔seed_list/reseed/client/rss〕·56 处存量不强制〔报告禁项+P1-05 定案〕·seed_list 十态锚复验绿；PR-F adapter 按需——§59.559 专项收官）。v0.0.1573（§59.559 PR-C Execute 上传段抽取——438→287 行·executor_upload.go 两函数〔executeUploadAndDedup 八路 +finalizeExecuteResult 加种落库〕·existing 哨兵通道·四阶段全抽齐〔PR-5 三段+PR-C 两段〕；前置锚三枚+踩坑两枚〔同名锚去重/跨包私有不可引〕；§59.559 P1 三刀收官·PR-D/F 按需）。v0.0.1572（§59.559 PR-B engine_match 拆文件——2235 行→五文件〔主 438 编排+决策/seedfeature 290/legacy 185/l0 358 九函数/verify 1044 三十八块〕·纯移动·172 现有测试=天然锚〔集成级锚记录观察项·函数级时再补〕·声明集合 56=56；PR-C Execute 上传段待续）。v0.0.1571（§59.559 PR-A batch_fetch 职责收敛——11 块归位〔seed-detail 三 helper/BDInfo 三 handler/后处理五函数→两新文件〕+batchFetchItem 命名类型〔主文件已有六字段版复用·踩坑=grep 全包同名先行〕；PR-B reseed/PR-C Execute 上传段待续〔前置测试锚〕）。v0.0.1570（§59.558 终 PR-6 余量三文件——site_handler 2858→四文件〔主 733+crud 1101+manage 371+search 719〕·seeding_handler 1809→五文件〔主 255+config 536+records 453+stats 417+scoring 216〕·adapter probe 1298→两文件〔probe 418+userstats 898〕·声明集合 72/35/28 三重零丢失；**§59.557+558 拆分专项全收官**：God File 五刀+函数级六刀十三连发 v0.0.1558-1570·全部零行为变更）。v0.0.1569（§59.558 续 PR-5 Execute 拆分——744 行→executor_stages.go 三函数一结构〔PushOnly 通道 45 行/表单段 273 行〔executeFormBuild 十产物+闭包跨函数方案 fail·failRec 留主函数·失败经 status+msg+useFailRec 返回〕/DryRun 检查点 36 行〕·⑦ 段归位教训〔preAudit/localFindings 主函数消费·卷进表单函数 unused〕·主函数剩余准备/审计/上传/落库四段〔早返回交织最深·按需〕；PR-6 余量待续）。v0.0.1568（§59.558 续 PR-4 RunTask 编排拆分——273 行→主函数〔锁/defer 链/panic 三敏感段〕+engine_match_prepare.go 三 helper 三结构〔源收集/参数解析/预载族顺序零变化〕·上下文指针 defer 留主函数〔闭包不跨函数〕·scan 族内部抽取未动〔报告「优先纯准备/汇总」原则〕；PR-5~6 待续）。v0.0.1567（§59.558 续 PR-3 fetchSingleTorrent 拆分——537 行 goto 跨越函数→seed_fetch.go 八函数两结构〔coverage 保证/音乐路由/源站选择/获取降级链函数化——goto fetched 语义等价消除/本地工件/后处理三件/终态 Profile〕·编排 1 屏·日志 key 与时序零变化；类型踩坑三连=跨包类型名编译器兜底；PR-4~5 待续）。v0.0.1566（§59.558 续 PR-2 handleListSeeds 拆分——前置测试锚十态先行〔用户边界·拆分前后双绿〕·682 行→seed_list.go 十函数一结构〔参数聚合/六阶段装载/行组装五级过滤/尾段收尾〕·编排 1 屏；PR-3~5 待续）。v0.0.1565（§59.558 函数级拆分第一刀——用户方案 A+边界：两 commit〔PR-1 职责对齐 12 块纯移动·区间卷入修正〔batch_fetch 3439→2009+新 seed_detail 1502〕+engine_title 1921→三文件〔title 509+keyword 863+music 576〕〕·声明集合双证明〔api 157=157/reseed 113=113〕；PR-2~5 逐刀另起·PR-2 前先补 handleListSeeds 筛选参数组合测试锚——用户边界定案）。v0.0.1564（§59.557 补：拆分触发 CI 严检五 error 修复——存量代码进新文件触发「新增代码全量严检」：ST1020×2〔导出方法注释首词须函数名·历史形态〕+unused×3〔keepfrds 三正则真死代码·使用者历史已删正则残留〕——注释形态修正+死代码删除·tag 后补丁打新 tag〔§59.1507〕——教训=拆分让存量代码获得严检视野·严检是拆分的免费体检）。v0.0.1563（§59.557 终 adapter_nexusphp 拆分——3616 行 102 块→四文件〔主 715+detail 838+upload 835+probe 1306〕·**God File 专项五刀收官**：24231 行全拆约 26 文件零行为变更五连发〔1558-1563〕·方法资产沉淀〔块解析器+分配法两型+声明集合校验+豁免前缀化〕）。v0.0.1562（§59.557 续 seeding 拆分——3562 行 103 块→五文件〔主 332+sync 1018+evaluate 997+patrol 470+stats 848〕·校验器假单行 regex 跨行吞噬 bug 破案〔\s+/[^)]* 跨行·^var/^const 块首吞段〕→行内语义修正+前三刀回补复验全绿；全库 gofmt 历史格式债 315 文件一次性清零；剩 adapter 最后一刀）。v0.0.1561（§59.557 续 reseed 拆分——6708 行 230 块最大一刀→六文件〔主 321+preload 921+match 2400+inject 730+store 556+title 1941〕·校验升级跨行签名兼容〔参数换行形态单行 regex 误报丢失·多行形态补丁〕·cloud_fp/errors 已有拆分零触碰；剩 seeding/adapter 两刀）。v0.0.1560（§59.557 续 executor 拆分——3699 行→五文件〔主 1347+helpers 324+mapping 647+site_rules 1308+prefill 233〕·行号集合分配法〔子域散布非区间·doc 职责定归属+表外默认 main〕+块解析器三形态修复〔注释后 line 刷新/type struct 无括号/var map 字面量〕+声明集合 103=103〔校验 glob 卷入 test 假新增 21 的口径教训〕）。v0.0.1559（§59.557 补：sqlcheck 行值 IN 豁免改前缀匹配——§59.316 豁免按精确文件路径匹配·拆分后 SQL 迁至 _seeddata.go 豁免失效·build 门实证拦截〔suspects=1〕·改 HasPrefix 覆盖子域文件·tag 后补丁打新 tag〔§59.1507 纪律〕——教训=按文件路径的豁免/规则对拆分天然脆弱·前缀匹配是拆分兼容形态）。v0.0.1558（§59.557 God File 拆分试水——用户方案交叉验证〔五文件规模数字精确+锚点在册+零遗漏〕·试水 api/publish_torrents_handler 6746 行→六文件〔主 557+torrents 1284+seeddata 571+batch_fetch 3446+execute 684+music 389〕·python 顶层声明切分+声明集合零丢失校验〔157=157 机械证明〕；后续四文件按同流程铺开；方法论=声明集合相等是纯搬移最强机械校验+区间分配法零行为变更下足够）。v0.0.1557（§59.556 ServeHTTP 路由异常修复纯 B——用户审计两异常：recompute-profiles case 空体〔命中空响应〕+fix-stale-dolby 双 handler 串联〔双写风险+全量 recompute 负载〕；blame 时间线重构坐实：08-30 同步版→11-11 §59.539 异步化清空体未删 case 行→11-21 §59.529 新 case 顶上收编老调用行；用户定案纯 B=删 case+删调用行+删 81 行死代码函数——404 比命中空响应诚实·与异步化意图一致；锚两态〔退役 404+task 健康/单 JSON 双写检测〕；方法论=路由 case 空体是改造未删净高发残骸·空壳比删除危险·长 switch 插入要查邻位孤行归属）。v0.0.1556（§59.555 朱雀复测修正两件——①note 声明合成层：用户抓漏「两行系统声明」=ComposeStatement 消费时合成（§59.542 追记十二）·tnodeNoteOf 直取原始列漏合成层·修复=改走同单点+纯文本化②致谢句号双形态锚：ThanksCoreText 句号化〔工作区改动·显示对称〕破坏生成/剥离单源——句号锚漏剥存量无句号形态回流治三测试红·修复=Legacy 锚〔TrimSuffix 派生·子串双兼容〕+正则可选句号·单源保持；第三颗复测实证 55835=pushed+TMDb 在位+note 正常+推种成功——三刀全生效收官；方法论=单源常量改措辞必审计派生锚消费方向+子串关系是双形态锚最省实现+站点特化必须复用通用链合成单点）。v0.0.1555（§59.554 朱雀发布链三刀——首两颗真实发布复盘：问题一 ID 丢失链〔上传成功但响应 id 数字形态炸 string 字段→html fallback 静默丢 ID→推种空 ID 404→终态停 uploaded·五环证据链闭合〕+问题二 tmdbid 恒不投〔挂账实锤为我方未投·适配器槽位在〕+问题三 note 内容错位〔站方 TMDb 渲染简介·应投纯文本声明+IMDb/豆瓣链接行三件套〕；摸站=243 key+密文 cookie 内存管道→SPA 壳→chunk-vendors 反混淆还原 findByImdb 端点→实测 200〔tt0342778→寻枪 83277〕；三刀=anyIDToString 三态断言+fallback warn 日志+executor tnode 特化 note 三件套〔判据 site.Framework 与工厂路由同源〕+fillTMDBByImdb 反查〔fail fast/无词条两分〕；回归四轮实录：抓 json.Number 自身 bug+MTeam 张冠李戴被用户纠错+tnode 三口径独占核实；方法论=站方前端函数是 API 直发规格书+降级路径必须可观测+五环证据链无日志事故走法）。v0.0.1554（§59.553 P2 渐进治理四提交——P2-01 解码单点 103 处〔91 标准+12 复合拆分=DryRun 教训泛化·五种文案漂移统一「请求格式错误」·6 处刻意保留定性清单〕+P2-06 source_detector 九处凭证条件内聚〔报告漏 382/420〕+P2-02 download nil,err 统一+P2-08/10/07 小清理〔前端 downloadBlob/formatDateForFilename/methodNotAllowed 125 处〕；实施期事故=methodNotAllowed 批量替换吞定义体→自调用爆栈·全量测试抓出·教训=全局替换必须排除 helper 定义文件；挂账=P2-03 分批/P2-04/P2-05/P2-09 渐进；方法论=审计报告与实施时序差产生重叠提案必查近期项防重复建设+code 与响应函数风格是契约边界不动）。v0.0.1553（§59.552 P1 契约先行与语义统一——复审 6/6+三深挖〔分页三档分裂/page_size 负数直通 SQLite 无限制全表=真实防御缺口/Sscanf 42abc 静默接受+手写 parseUint 溢出回绕〕；用户三定案：统一 200 有意放宽+键优先级测试锁定/P1-05 窄版契约 57 处不强制/保守收敛保文案哨兵；十三处分页迁移+两处契约冲突保留〔§59.509 clamp/§59.166 上限——审计报告不知情的用户历史定案·P1 与 P0 本质差异=契约统一必须逐处核对既有定案〕+util.ParseUint+api parseUintStrict 三 helper 委托+五函数改名隔离〔toInt64/extractDomain/normalizeDomain 同名异义清零〕；方法论=契约冲突即停三案例+实态≠契约锚+==0 陷阱〔只判零不判负〕）。v0.0.1552（§59.551 P0 低风险重复收敛——用户审计报告复审 7/7 属实+四处漏项〔containsAnyWord/image_check_test/tmp-hashtest 纪律残留/同包隐蔽消费者 tag_inferer+artifact_test 8 处——grep 定义看不见跨文件同包消费者·编译器最后防线〕；三分歧用户定案：P0-02 slices.Contains 标准库直调〔第四副本防线〕/DedupStrings 手写 map 保行为等价〔空 key 契约注释固化〕/1-2 主题提交；util 三新函数〔ContainsAny/ExtractHost/DedupStrings·零 internal 依赖〕+七包 90+ 调用点收敛+min/max 八处删除〔内建接管〕+agsvpt 手写子串搜索退役）。v0.0.1551（§59.550 补 cmd/pt-forward 第三处前缀回退漏删——build-backend 门抓出：internal/ 全绿但 cmd 域漏查〔本轮 vet 只跑 internal/ 忘了全量纪律〕·main.go :329 验证层组门家族豁免的同款回退残留·同批删除+注释收编·tag 后补丁打新 tag〔§59.1507 纪律〕。v0.0.1550（§59.550 官组身份零推导原则+前缀回退收编退役——组名摸底审计〔20190 标题→21 组名 vs 映射表 252 官组〕发现 PigoWEB/DSWeb miss·用户实态核查：PigoWEB=大小写失真·DSWeb=dstudio 深度官组〔全官组源站搜索箱未列·搜索有大量种子实证〕；用户原则定案「系统不能推断官组·必须显式映射开发者决定」与 §59.546 跳过=替用户做决定同构；实态修正=大小写 LOWER 归一已在生产链〔审计工具精确匹配假阳性〕+前缀回退是现存推导〔两处消费〕；用户选 A 收编：migration181 DSWeb 显式收编〔退役唯一受影响面·CMCTf 22 行现已无数据〕+前缀回退两处删除+FuzzyGroupPrefixKey 函数删除+新锚三态〔零推导/LOWER 命中/显式收编〕；方法论=审计工具匹配语义与生产链同源+机制退役先摸数据受影响面+显式映射是数据纪律非性能优化）。v0.0.1549（§59.549 劳改所媒介源保真——Quacks AMZN WEBRip 案·站方标题检测 WEBRiP·媒介槽投 BDRip〔TYPE_MISMATCH〕：根因=劳改所媒介按「源」四分立〔BDRip/WEBRiP/TVRip/DVDRiP·适配期精确分立槽〕vs 通用翻译按「形态」归并〔Encode 大类·§59.166 语境=站方无 WEBRip 槽〕两套体系错位——encode 键绑 BDRip〔上次 BD 源压制定案〕致 WEB 源压制跟投；用户点破「这个站点的媒介和其它站不一样」；实施=applyMediumSourceFidelity 统一源保真〔executor 两处接线·仅改写 medium.encode 保守面·双通道判源：spec 显式 webrip/tvrip ∨ 血统隐式 encode+SourceType=WEB·值集有源槽→精确键无槽→回退通用·BD 源压制零触碰——蓝光压制要求 BDRip 定案锚⑥锁定·其它源分立站查键动态受益〕；锚六态〔含值集 StandardKeys=全键形态工程坑——手写裸键共享误解〕；全矩阵回归 11 形态〔挂账 HDTV 空投+DVDRip 键名错位零实证〕；方法论=源分立站与形态归并站体系错位——适配期识别站方分立维度+查键动态判定泛化非站名硬编码+同族双实现两处都接）。
v0.0.1548 及更早（§59.548/§59.547/§59.546/…/§59.132→§59 全部过程细节见 docs/31-模块设计决策记录.md——grep "§59.X" 定位章节·docs/32 站点文档同索引。

**主线状态一句话**：NP 架构 62 站目标站适配收官（§59.450 知识库落盘）；音乐发布链已建成（皇后/海豚/乌托邦三源+MDP+翻译层·§59.466-492）；辅种覆盖体系六刀收官（§59.535）；发布链四维治理完成（终态单点 §59.546/三态三面四色 §59.547/禁转复检实态化 §59.548/媒介源保真 §59.549/官组零推导 §59.550）；当前活跃焦点见下方「下一步」与「活跃观察项」。（§59.149-§59.233 历史演进全录见 docs/31——grep "§59.X" 定位）

**下一步**：
0. **NP 架构目标站适配告一段落（2026-11-02 用户定案）**——62 站收官经验已落盘 docs/31 §59.450（资产可信度谱系/槽位语义陷阱/媒介六形态/codec 两极/音频承接库/HDR 三模式/tags 雷区/键双体系/结构异常/双零验证/抓取谱系/站点分型与 SPA 边界十二节知识库）；二师兄（SPA 改版）终止留档 docs/32/piggo.md 附（未来重启前置=渲染 DOM+提交端核验）；62 站首颗真实发布均遗留+§59.433 三站人工审核挂账照旧；新架构站（SPA/API 化）适配待用户指令立项
1. **孤儿恢复优化线收官观察（§59.182-225 三十一修，09-10~14 十二批实战验证毕——优堡57/Disney83/UBits/SSD4-9/TTG/馒头成人区 MT9）**；遗留观察：辅种侧三态口径回流、柯南站方资源形态
2. **辅种 L2 三态门口径实战回流观察**（§59.184-185 Disney 批 83/83 全正确处置已闭环；遗留观察：refute-neutral 口径辅种侧回流、中文链 l2:*-cn 命中形态、预存 HDR 严格性 DoVi HDR vs HDR10、keepfrds 用户下载配额阈值）
3. **Tab1 数据源对齐收官并实施完成（§59.226-228——v0.0.956）**；组名识别统一三层+@复合资产化（§59.233——v0.0.963/964；cqkyjj2 及 @ 复合 21 形态已 seed 化随版发布，无手动项）；Tab1 表格优化自检四修（§59.235 P1-P4——composeMedium 单点/PTGen 后置提取/BDAV 容器门/分发方词表，v0.0.967-972，P2 终修后引出 X 重构定案）；**X 重构已实施（§59.236——作品信息单通道 v0.0.973：runMainlinePTGen 主链必经/kdouban 逆向删除/desc 站方原文切断/四键 JSON 派生/§59.235 P2 补丁架构性取代）**；译名陷阱修正+缓存 enrich 补提（§59.237——v0.0.974/975）；UI 五轮优化（§59.238-242——v0.0.976-980：冗余行删除/标签表头紫/筛选下拉直选/预览区序号重排+色系终局）；fnos SCP 更新流程实证；译名陷阱修正（§59.237——TranslatedTitles 唯一真源：端点 foreign_title 恒=中文陷阱实证/bbcode 提取独立正则/kdouban 反置语义发现，v0.0.974）；fnos OTA 重获后数据级验证已收官（§59.346——PTGen 族键/desc 同源/簇跳过三绿，2026-10-25）
4. 发布配置 diff 维度补 StandardKeys 对比（低优先）

**已销账（2026-09-14）**：~~PT31 批量重获治愈 223 簇存量~~（DB 实证 mi=0/ss=0 均零——09-08 批量重获 11232 行已治愈）；~~幸运审核实战观察~~（advisory 引擎稳定运行，§59.166）；~~修道院新种站方审核观察~~（tid=1850-56 观察期毕）；~~team 域词条建设~~（R3-5/R3-6 遗留——用户销账放弃，SHB931 形态分类已归纳备查）

**活跃观察项（未闭环）**：
- ~~**特征辅种 API 站案（§59.543·2026-11-20 全清单闭环）**~~：任务 2 对好大/二师兄零结果→五项全闭环（项1 SearchByPiecesHash 1534·项2 BatchMatchHashes 删除 1535·项3 导入 1:1 镜像定案·项5 import-precheck 工具 1537·项6 销案〔piecesHashSites=0 为熔断轮瞬态·21:40 健康轮铁证 queried=825 matched=15 piecesHashSites=1·追记七〕）+下载 403 修复（1536·UA 判罚根因终裁·追记四五）；生产成果：L0 31 hits+特征批量 15 命中+5 注入成功
- ~~**猪猪令牌 API 适配（2026-11-16 挂账·§59.538）**~~（**2026-11-17 销账**：API 已部署上线·PiggoAdapter 全链建成（§59.540-541 获取+值集刀+发布）·**首战五连发收官**（§59.541 附五：五颗 pushed tid 57289-57293+做种 100%·五层洋葱 1512-1515 全录）——残留观察：站方 {{ptgen}} 渲染块形态+MI 双通道接受度）
- **海豹/朱雀/可乐首颗真实发布（2026-11-04 用户定案统一挂账——用户另行安排时间测试）**：
  - 海豹：五刀+三前置+回归审核修+DryRun 全绿工程就绪（v0.0.1341 终态）；前置=用户一次性设置（设置→截图格式切 png→目标素材 Tab3 重截）+首颗两项假设终验（fetchMovieData 覆盖行为/genre 英文键接受度）
  - ~~朱雀~~（**2026-11-22 销账：首测期收官**——§59.554-555 三刀+复测修正·六连 pushed tid 55835-55840 全完美〔用户确认〕·用户连发五颗批量流畅；残留：存量两颗 5001/5002 站方编辑页手动补 note+tmdbid）
  - 可乐：获取链全通收官（§59.460）；发布侧等用户发种机权限+automation scope 勾选后探三步字段契约
- **我堡/五二目标站适配结果人工审核（§59.433 挂账——用户指令）**：两站均降级验证发版（form_config 值集未经站方当前实态实时确认）——人工审核发现漂移/拒收回传 upload HTML 走 formdiff 定位链闭环；各自前置：五二首颗发布+候选权限/我堡候选通道或权限达标补验；~~财神~~（2026-11-14 销账：实发 106 颗·84 过审+22 拒全破案修复（§59.529 杜比单键+值去重+Atmos MI 铁证）+67 站实态审计财神 diff=0——实发核验完成）
- ~~批量补推生产验证挂账（§59.509 附六）~~（2026-11-15 销账：API 小批量子项已被 OKPT 案实战覆盖——45563 案 7 颗批量补推全 pushed·新判据版；浏览器全流程为同端点低风险形态随案关闭·§59.534 附三）
- **好学重适配（§59.533 进行中·2026-11-15）**：站方改版离开 NP（Next.js+/api/v1 REST）·摸底两刀+收官完成（发布 FormData 15 字段+CSRF double-submit flux_csrf↔x-csrf-token/section-dict 值集 70 制作组含 FRDS/tags 9/详情 /detail 端点/download ✓/RSS 无）；四定案=内容全放开+旧特化（拦截/年级教材注入）废+新 framework hxpt（API 直发）+price 恒不传+anonymous 系统默认；刀2 获取链完成（HxptAdapter+factory+migration169 framework 刷+publish 临时关+双 JSON+端到端七链全绿·v0.0.1479 已发）；刀2-5 工程全链完成（v0.0.1479 获取链/1480 发布链+值集/1481 DryRun 借壳验证+tags 断链修复·五键全中 dry_run_ok）；**剩首颗实测——前置：站方上传权限（账号 403 code 2003 全分类实证·web 手动传同样 403·联系站方开通后再发；§59.533 附四认知更正：OKPT≠好学——OKPT=www.okpt.net 独立老站·其 download 超时案=243 网络瞬态补推自愈·与好学无关）**——观察项：source 域缺失站方容忍度/重复发种形态/promotion 非 null 值形态；~~上传权限 403~~（§59.533 附三已实证=账号权限非系统问题）
- **压制编码器词组装死锁（§59.532 挂账·2026-11-15）**：Zhizn 案（ShanaEncoder 压制·MI 只证 HEVC 证不到 x265）→重组输出 BluRay HEVC 混用违规；方案已定案（Encode 限定透传+原盘门+classifyMediumForCodec BluRay/disc 分类同修）——**>3 同类种子再修**（现 1 例）；另 Dr Strangelove 简介嵌 MI 案挂账（原站嵌套引用·用户定后处理）；引用块消费面两理论窗口零实证挂账（statement 内嵌来源规格行+desc_raw 国语/粤语/短剧未剥 quote——§59.540 附七盘点·设计无影响·误报系人工分析非系统）
- **我方声明回流治（§59.542 三刀已实施·2026-11-19）**：存量清洗待执行——243 实证 295 颗回流+13 颗双份累积·待 v0.0.1520 到 243 后前端点「简介引用清洗」backfill 一遍；获取链 DeclFilter 回调挂点已补（fetcher 直落路径）·观察新重获 statement 干净度；source_description 的 ptgen 套娃层（12172 字形态）单独立案未动
- §59.208 降级链不对称观察（fetch 无 yearless/rawname｜recovery 无 area=1）——按需生长纪律：零案例不预实施；第 3 次跨链不对称实证触发轮次库抽取重构
- ~~build-backend go test 偶发"首跑 FAIL 重跑过"~~（09-20 销账——时序 flake 族三案全闭：recordMap CreatedAt 零值×2+EventBus 异步断言×1，单核轰击验证零复现）

**已销观察项**：LuckAudit 过审率（09-02 收官——除 Suspiria MI 误判外全部正常过审，幸运专线 §59.155-163 实战验证完成）；阮玲玉两态放行（09-02 双路闭环 ✓）；243 错加无关种（用户已删）；keepfrds 列表页容量假设（朋友站日发种个位数，消解）

**行为铁律（全文保留——约束每次开发）**：
- **调 API 一律用 ops token 变量（2026-11-20 用户指令定案）**：操作各环境 API 必须用 `.env` 的 `OPS_TOKEN_<环境>`（Bearer 头·一环境一个·用户生成配置）。**禁止走 admin 密码登录通道·禁止 reset-password**（历史事故：Agent reset 与用户改密互相覆盖·双方密码轮番作废——v0.0.1523 案）。未配 token 的环境需要调 API 时·先请用户按三步配置（宿主机生成→compose environment 加 PTF_OPS_TOKEN→up -d→串入 .env）·不走密码捷径。
- **全量覆盖端点禁止稀疏 payload 实测**（2026-10-06 §59.318 附五教训）：导入（sites/import）语义=整行替换，`Select(siteImportFields)` 强制写 35 列含零值——缺省字段=清零非部分更新。实测必须用完整导出文件或测试专建站点，禁止手造稀疏 JSON 打真实 DB（29 两站 35 列清零事故，243 真值回填恢复）。
- **生产站实测禁止直接上传**（2026-09-26 用户定案）：验证发布链路必须走 DryRun/正常发布流程——手工 takeupload 会创建真实种子且可能无权删除（修道院 id=3620 复现事故）。
- **发布链资产消费白名单**：发布=纯消费种子配置落库资产。网络动作仅允许 ①目标站交互（pre-audit/dedup/上传/下载新种）②下载器 RPC ③**发布投递所需图片下载**（§59.458/§59.459 附二：封面 PosterURL+Tab3 截图 URL——现场下载→内存 base64→投递完即弃零持久化；数量门=站方 images 上限合计 ≤6；逐张失败降级跳过不阻塞；不开放其它图床下载）——其余外部 API 一律禁止（发布用 assembleDescription 纯本地组装，禁止复用采集组件）
- **发布四终态**：pushed/pushed_existing（302 权威 ID 自动推种）/existing（信文案不信页面 ID，不推走辅种）/failed——已存在=终态非失败；重试=人工幂等再执行，无自动重试引擎
- **上传判定二元化**：仅接受成功页种子（302 最终 URL+uploaded=1）；body 通用提取禁止（推荐位误推）；"其它页面的种子是辅种业务的工作"
- **幸运判据八条**（§59.151 终版：MI 唯一真相/HDR Profile 语义/语言四通道/粤语复合/媒介 title 词优先/音频组合键/英语条件映射/映射完备）——全文见 docs/31 §59.151 终局节
- **站点下线=双 JSON 移除条目随版本生效（全站通用铁律·§59.512 修订）**：supported_sites.json（唯一权威·启动对账判定源）+ data/sites.json 同步移除（漏删=启动断言 fail-fast 显性拦截）；对账每次启动强制 DB 收敛于支持集（migration DELETE 一次性语义防不住数据恢复路径·已废弃——243 pt.cdy.skin 镜像重构复活案）；导入导出双过滤为入口/出口防线；复活时按新站重新走接入流程（2026-09-04 用户定案+2026-11-10 §59.512 三防线）
- **新站骨架永不默认启用（全站通用铁律）**：migration/seed 默认增加的支持站点一律 `enabled=false`（落"未启用"列表）——只有用户自行添加认证凭证并启用的站点才进入"我的站点"（2026-09-04 用户定案；migration 33 修道院 Enabled=true 反例教训）
- **系统级禁发剧集分集（§59.424 用户定案·行为铁律层）**：所有站点仅允许发布合集/完结/全集形态剧集——范围 {tv_series, animation, tv_shows}（动漫电影形态无 S/E 信号天然豁免）；放行信号任一：源站采集标签 complete ∨ 主标中英词形（\bcomplete\b/完结/全集/全季）∨ 副标中英词形直查 ∨ 全集一集（E01∧eps=1）；挂归一后 PushOnly 分支之后（补推豁免——形态恒定 vs flags 时变挂载策略不同）；纯代码规则零配置
- **转存发布禁勾"首发"标签（全站通用铁律）**：首发=PT 圈首次发布，站管/压制组官方专属，转载人员不允许使用（2026-09-02 用户权威定义）
- **context 使用遵循 `docs/代码质量审计/附录A-Context使用规范.md`**（四类约定：Request=r.Context / Task=serviceCtx+超时 / Service=根 ctx 停机总闸 / Finalize=Background+短超时+注释；后台任务必须接入停机·超时用命名常量·副作用错误不静默）
- **转存发布禁勾"禁转"类标签（全站通用铁律·2026-11-23 用户权威定义）**：禁转=「这个资源只在本站发布，不允许转载发布到其它站点」——站点/原发布者专属权力声明，转载者不能替代任何人行使，在任何目标站都无权勾选。与采集侧「禁转标志」同名不同物：标志=源站采集输入信号（标签/标题说明）由拦截链消费（§59.369 禁转必检+§59.548 复检实态化），真禁转种走不到发布这步；标签=发布侧投递动作，我们永远不投（四层防线：拦截链六处+tag_inferer 无禁转 canonical 键+55 站值集 label-only+站点特化不投——§59.560 追记二代码实证）
- **站点代理解析公共单点**：`site.ResolveSiteProxy(db,ctx,site)` + `httpclient.NewSiteHTTPClient` 组合，禁止裸 client（§59.156）
- **测试工具镜像禁入生产环境（2026-11-10 用户指令）**：Playwright/浏览器工具等测试镜像只在开发环境 29 拉取运行——29 容器 --network host 访问生产环境才是正确工作方式；生产环境（243/249/12/22/65 等）禁止 docker pull 测试工具镜像（243 曾有 17 个月前遗留 playwright 3.6GB 已清理）
- **新增发布终态纪律（§59.546·v0.0.1544）**：新增 publish_result_records 终态 → 先在 `model/publish.go` 两常量登记（判断语义归属：List=记录在案口径含 existing / Skip=发布事实口径不含 existing——existing 不入 Skip 是用户定案〔保留推种选择权〕）→ 改锚测试 `publish_status_sets_test.go` 内容锁定 → 再 grep 消费面接线。**禁止任何查询点手写裸终态清单**（源码扫描锚 `TestNoBarePublishStatusList` 会拦：`"pushed", "pushed_existing"` 相邻指纹只许出现在常量定义处）。
- **clients 表查询铁律（§59.251/v0.0.995 教训）**：按 UID 查下载器必须 `Where("id = ?", uid)`（uint 列）；`Where("name = ?", ...)` 仅限真名字场景——**uint 变量查 name 列=恒 miss 静默错**（isLocal/role 五处漏网实证，mock 测试不掩盖 DB 查询语义）。clients.id 带 AUTOINCREMENT 永不复用；悬空引用（rss 订阅/转移目标）由消费方失败处理容忍（§59.251 保留面定案）

**历史详情**：§55-§59.161 全过程见 `docs/31-模块设计决策记录.md`（grep "§59.X" 定位章节）。

**⚠️ 本段维护铁律**：任务焦点只保留——版本/下一步/未闭环观察项/行为铁律。完成的章节一律一句索引行（`§59.X 见 docs/31`），**禁止累积过程叙事**（本段曾膨胀至 11.4k token——2026-09-01 治理）。


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

## 站点角色术语定义（§59.369 权威）

- **站点**=系统内置全集；**源站**=必须有官组（/sites"作为源站"✓，亚型：纯官组/混合）；**目标站**=官组可选（/sites"作为目标站"✓，无官组=转载站）；源站/目标站可交叉
- **源站适配=站点适配的自然结果**（能正确采集+有官组 ⟹ 即好），不独立工程；**目标站适配=发布适配**（一站一指令+长期维护）——未适配的站目标站开关用户不可开（§59.318 publish_supported 门槛即此语义）
- **源站适配必含禁转必检**（§59.369 附，用户权威）：源站必有禁转规则——必须能采集禁转类标签/特征（详情页标签 + 主/副标题头尾部文字标注），适配验收用站内已知禁转种实测采集链命中；docs/32 文档必须含禁转标记形态章节
- 纯官组/混合不数据区分（区分点=目标站门槛自然收敛）

**settings 表键名（§59.368 定案）**：新键一律 **snake_case**（Go 常量 `setting.Key*` 惯例）；camel 遗留清单锁定（`httpProxy`/`otaUseProxy`/`socksProxy`/`cookieCloudUrl`/`cookieCloudUuid`/`cookieCloudPassword`/`websocketEnabled`——§55-56 前端直传时代产物）——**仅在因功能触碰时顺路迁移，禁止新增 camel 键**。

## 环境信息

> **§仓库安全-1**：基础设施信息不入库——实际值见本地 `.env`（不入库）·模板见 `.env.example`

- **gitleaks**：`~/.local/bin/gitleaks`（v8.24.3·第三刀防线·新克隆环境需 `git config core.hooksPath .githooks` 激活本地 hook）——pre-commit 自动调用·缺失时 warning 放行（CI 硬门兜底）。误报处理：`.gitleaks.toml` allowlist 值级精确豁免+commit 说明理由·**禁止 `--no-verify` 绕过**。CI 三流（ci/release/docker-publish）均有 secrets job（baseline 模式）·第八刀历史重写后删 `.gitleaks-baseline.json` 重新生成（应为空）。
- **夹具消毒管道**：`scripts/sanitize-fixture.py`（第七刀）——testdata HTML 变更时 pre-commit 自动消毒+重暂存（passkey/JWT/绑定 token→SANITIZED·幂等）·原始 HTML 直接 add 即可零人工。

- **Go**：`/home/<user>/.local/go/bin/go`（v1.25，系统 PATH 中无 go，必须用全路径）
- **Node**：`/home/<user>/.local/bin/node`（v22，系统 Node 18 不兼容，必须用此路径）
- **CGO**：后端编译必须 `CGO_ENABLED=1`
- **DB**：`data/pt-forward.db`（SQLite）
- **服务**：`systemctl --user restart pt-forward`（用户级 systemd 服务，端口 8765）
- **开发环境**：`$DEV_HOST`（见 .env），Agent 唯一可操作的机器（编译/测试/部署/DB）
- **生产环境**：各主机 IP 见 .env 的 `PROD_HOST_*` 变量，**禁止 Agent 直接操作**
- **243 环境**：`$PROD_HOST_243`（见 .env），SSH 密钥路径 `$SSH_KEY`，**禁止 Agent 直接操作**·只读诊断+scp 二进制
- **代理**：`$PROXY`（见 .env·curl/docker pull 等访问外部网络时可用）
- **Docker**：`sg docker -c "docker ..."`（当前用户不在 docker 组，需 sg 切换组）

## 通用规则

- **语言**：与用户沟通用中文
- **设计决策**：所有设计决策记录到 `docs/31-模块设计决策记录.md`（grep "§59.X" 定位）
- **敏感信息**：禁止保存 cookie/passkey/apikey/token
- **适配器文档**：`docs/32-站点适配器设计/<站点>.md`；站点数据原样写入不杜撰
- **删除代码后**：跑 `go vet ./internal/... ./cmd/pt-forward/`（不用 `./...`——cmd/verify-pieces-hash 有已知冲突）
- **Git 提交**：禁止提交 `data/`、`PT0/`～`PT8/`、`*.torrent`、`logs/`、`*.db*`（详见 .gitignore+pre-commit hook 双保险）
- **版本号**：编译必须 `-X main.version=$(git describe --tags --always --dirty)`
- **环境隔离铁律**：开发只在 29；生产（249/242/22 等）只读诊断+scp 二进制，禁止直接改生产；生产更新由用户执行
- **部署铁律**：先提交后部署（commit+push 必须在编译部署前）；前端涉及必走三步（vite build → cp dist → go build）；完整流程见 skill `ptf-deploy`
- **部署脚本输出禁 grep/tail 等一切管道**（§59.462 附七+§59.514 附八 P6 教训）：管道（`| grep`/`| tail`/`| head`）会吞掉脚本拦截的 exit 1 造成「假部署绿」——tail 曾漏网三轮（build-backend sqlcheck 门 exit 1 被吞·版本由 build-frontend 的 go build 掩盖正确部署）；调用部署脚本必须裸跑完整输出，或 `set -o pipefail` + 显式退出码检查；输出里的陌生标记（如 suspects=N）=红旗必查
- **临时诊断代码禁 commit 进 main**：诊断用 `cmd/tmp-*/` 离线驱动（§59.338，编译即删）或本地改动（build 前 stash）；确需部署验证的诊断改动，验证后必须 rebase squash 清理历史再 push（v0.0.1339 诊断期 5 个临时 commit 进远端 main 后 force push 清理的返工教训）
- **部署后必须数据级验证**（版本号/migration 记录/功能端点实测——脚本退出码≠生效，§59.155 竞态教训）
- **设计文档改动后必须 `wc -l` 行数验证**（docs/31 曾被清空未察觉——§59.161 教训）


## 方法论：复用已有基础设施

**核心**：新问题先查已有基建（Detector/Fetcher/Searcher/Resolver 等命名约定+同类 handler 对比+DB 表内容），区分"能力缺失"vs"接入遗漏"；**优先公共函数**（grep 同名/同义函数，私有跨包先提升公共——重复造轮子是 §59.26/§59.156/§59.159 三次事故同型）。完整调研清单与典型清单见 skill `ptf-dev-protocol`。

## 方法论：空态三问（§59.403 固化——条件操作防静默失效）

**定义**：条件操作（后端 `UPDATE...WHERE` / migration 扫行 / 前端 `if (x && ...)` 状态门）对"目标数据不存在"的默认语义是 **skip（静默跳过）**——当数据缺失是**意外状态**（首配/重装/清库/时序未到）而非预期的"无事可做"时，skip 就成了 bug。本质=update-only 语义误用在需要 upsert 语义的场景。

**三问**（设计/审查条件更新时逐条过）：
1. **目标不存在走哪条路？**——skip / 创建（upsert）/ 报错，三者必须是有意识的选择而非默认
2. **skip 是预期语义还是意外静默？**——穷举状态矩阵必须含**空态**（无行/未配置/数据未到），空态与"已存在态的子状态"是不同维度
3. **失败可见吗？**——静默 no-op 是否有日志/提示/测试锚兜底；migration 返回 nil 与表单"保存成功"提示都是假阳性信号

**两案索引**：
- §59.378（migration 时机）：UPDATE 扫 0 行 no-op+执行记录已写入→永不重跑（**固化损伤**——时序问题：数据之后会存在，挪序即治）；机制防线已固化=§59.379 TestFreshInstallChain（publish_supported 站配置缺失即 CI 红）
- §59.403（首配写入）：前端 `masked && ...` 首态要求已配置→首配 DB 无行恒短路，真值静默丢弃（**可重试不固化**——状态门问题：数据从未存在，须补状态分支）；前端无单测基建是已知缺口——按需生长：第三案出现时立项

**教训短句**：状态机条件首项即门槛时（`x && ...`），穷举矩阵必须含 x 为空的状态。


## 方法论：真实组件离线驱动测试法（§59.338 首创）

**定义**：测试程序直接 `import internal/*` 生产包、用真实组件构造（如 `httpclient.NewSiteHTTPClient` 完整栈），在 29 离线驱动目标链路——**除业务链路外与生产完全同栈**。

**适用场景**：网络行为/传输栈/限流/连接复用等依赖真实基础设施的验证——mock 单测栈行为失真、生产实测禁碰（环境隔离铁律）的场景。

**三优势**：①完整栈真实（smart dial/limiter/breaker 全生效）②29 合规（零生产触碰）③数据级基线（可复现——§59.338 基线复测 20/20 即此法产出，发版最强证据）。

**实施**：临时程序放 `cmd/tmp-<name>/`（编译验证后即删，不提交）；需长期保留的探测工具放 `cmd/`（如 searchprobe 先例）或 `tools/`（摸站探针 `tools/siteprobe` 先例·2026-11-11 用户指令工具化——**主模块成员身份**（无独立 go.mod）以导入 internal/ 生产链：三模式 `-cred`（凭证解密→bash 管道）/`-fetch`（页面抓取·curl stdin 注 cookie 不落盘）/`-verify`（货架 HTML→生产 adapter+Engine 离线验证））；生产 DB 数据（cookie 等）只读取出后本地使用。


## 回归审核第一项（每次审核先查）

- **AGENTS.md 任务焦点同步**：`git log --oneline -10` 的版本推进与"当前主线"描述对照——主线/支线完成项是否已反映？滞后即先补（历史教训：v0.0.631、v0.0.642、v0.0.670 三次同类违规）。

## 灵魂四问（每次代码改动后必须逐条审核）

1. **nil 安全**：所有指针返回值是否检查了 nil？map 查找是否有 ok 判断？type assertion 是否用了 comma-ok 模式？
2. **边界安全**：空输入/空 DB/context 取消/并发锁竞争等边界情况是否处理？`context.WithTimeout` 后是否都调了 `cancel()`？锁是否有嵌套导致死锁风险？
3. **回归通过**：`go vet` + `go test` + `vue-tsc -b` + `eslint` 是否全部通过？
   - **`vue-tsc` 必须带 `-b`**（build 模式/project references）——根 tsconfig 是 solution 文件，裸 `--noEmit` 会静默跳过全部子项目检查（v0.0.988 CI 全红教训：8 处类型错本地漏检）
   - **改 `dict/*.json` 后必跑 `go run ./cmd/gen-dict` 并提交生成的 `web/src/generated/dict.ts`**——CI 有 drift check，漏生成必红（§59.219/§59.235 教训）
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
