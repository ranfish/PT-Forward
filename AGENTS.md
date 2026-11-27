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

> **已销案索引**：仓库安全专项九刀（2026-11-18 收官·gitleaks 双门/密钥出 DB/大二进制/docs 审查/夹具消毒/filter-repo 七轮/发布物审计——全案见 docs/31 grep「§仓库安全」）。

**版本**：v0.0.1660（代码质量审计 15 规则引擎条件字段语义契约精读落盘——全库盘点据实立项·基线 6aad0a38·纯文档零代码：V-01（中危·确凿功能缺陷）`downloadUploadRatio` 零上传语义反转〔docs31:7757 规定上传=0 为 +∞·:7674 黑车检测核心需求·rule/evaluator.go:137-141 与 seeding/rule_evaluator.go:133-139 返回 "0"·离线实测零上传 value=0 对照有上传 5.0000·黑车「只下不传」下 >阈值 恰好不命中·另加未声明 TotalSize>0 门槛〕+V-02（中危·expr 路径字段缺失）两个 ExprEnv/buildExprEnv 均无该键·真实 expr.Compile 实测 unknown name（对照 ratio 通过）·文档 42/42 全覆盖失实+F-01（低危·需取证）未知操作符 default 放行致规则静默生效；与 01-14 正交；三刀待实施；证据 evidence/rule-field-contract-repro.txt）。v0.0.1659（14 全案三刀——前置验证〔master UserResource 源码权威+monika 404 探针+v9.0.0 版本考据——/api/user 是 v9 新增端点·老版站恒 404〕+刀1 /api/user 形态重写〔string 带单位+flexFloat 双兼容+API 失败降级 HTML 覆盖两路径〕+刀2 零值契约〔PublishSupported+Gazelle 位去 omitempty〕+刀3 flexFloat+ParseSizeToBytes 收敛单点〔PB 补齐+多小数点根除〕+三层锚三阴性〔A 形态退回/B 删降级恰复现原案两失败路径/C omitempty 键消失〕——**14 全案收官**·；**15 规则引擎条件字段语义契约精读完成**〔v0.0.1660·V-01 零上传语义反转 + V-02 expr 字段缺失·三刀待实施〕——审计系列 01-15 十五篇）。v0.0.1658（代码质量审计 14 外部响应解析与序列化契约治理精读落盘——全库盘点 1055 文件据实立项·基线 32145be3·纯文档零代码：V-01（中危·入站契约确凿功能缺陷）UNIT3D `/api/user` 响应形态与解析结构体不匹配〔docs/34:344-347 记录 uploaded:"50.00 GiB"/ratio:"50.00"·代码声明 flexInt/float64·真实 Unmarshal 实测 err=cannot unmarshal string into float64 且 uploaded 静默归零·FetchUserStats:803-809 无 HTML 降级→配置 APIKey 站点统计整体失败·形态来源止于摸站记录标注未验证〕+V-02（低危·出站契约失真·当前无功能损坏）PublishSupported 值类型+omitempty false 时键消失〔同结构体 8 个 *bool 正例·前端类型非 optional·同型 torrent/publish_form_config/site〕+F-01 flexInt 静默吞错+F-02 第二套字节换算多小数点吸收与上界漏网〔13 篇 F-02 同型延伸〕；与 03 错误日志/04 入参契约/08 适配器复用/11 整数溢出/13 尺寸上界正交；整改三刀待实施〔api/user 形态重写+零值契约字段+三层锚〕；证据 evidence/response-contract-repro.txt）。v0.0.1657（13 补刀〔用户复审两缺口〕——①settings ListByPrefix 前缀过滤通配符劫持根修：settings_handler:95 用户输入 prefix 直达 repository 裸拼〔白名单登记失实撤除——登记仅凭定义行猜测未 grep 调用方〕+前缀锚定转义+四态锚+阴性红·白名单登记纪律修正=理由必附调用方清单②F-02 default 分支（B 单位）补上界守卫——教训：白名单豁免=攻面转移·豁免核验深度必须等于拦截核验深度）。v0.0.1656（13 全案三刀——刀1 util.LikeEscape 公共单点〔V-01 根修·12 处收编含 pipeline:572 漏网补遗·转义与 ESCAPE 成对铁律〕+刀2 ESCAPE 补齐〔V-02·ptgen 两条+site_handler_crud 第一条〕+刀3 F-01 黑名单空串守卫/F-02 尺寸上界+三层锚〔单元+端到端+静态白名单登记制 12 条〕+三阴性全红〔A 恒等全命中/B 删 ESCAPE 漏匹配复现 V-02 原案/C 静态锚点名〕——**13 全案收官**·审计系列 01-13 十三篇全毕）。v0.0.1655（代码质量审计 13 字符串匹配边界与查询语义精读落盘——全库盘点 628 文件据实立项·基线 e9d27978·纯文档零代码：V-01 LIKE 通配符未转义〔3 处已转义 vs 9+ 搜索端点裸拼 %+用户输入+%·内存 SQLite 实测 %/_ 输入命中全表·搜索过滤失效〕+V-02 转义值缺 ESCAPE 子句〔ptgen_handler.go:84「正例」\% 无 ESCAPE 反漏匹配字面 %·实测返回空 vs 正确 100%done〕+F-01 黑名单空串恒命中+F-02 尺寸 float→int64 溢出〔未实测〕；与 04 入参契约/05 DB 纪律正交；整改三刀待实施〔公共转义单点+ESCAPE 成对+查询语义锚〕；证据 evidence/like-wildcard-repro.txt）。v0.0.1654（12 全案三刀——刀1 OriginKey 单一规范化〔V-01+F-01 根修·CVSS 5.3·四套手搓 key 口径收编+第四处 freezeMap 分裂+启动自检〕+刀2 并发槽计数式信号量〔V-02 根修·CVSS 7.5·世代唤醒 channel·取消锁内归还无阻塞〕+刀3 三锚+三阴性〔四形态域键契约/竞态 -race/取消不阻塞·阴性 A 恒等 FAIL/B 去 RACE/C 幽灵计数全红〕+三探针反转实证——**12 全案收官**·审计系列 01-12 十二篇全毕）。v0.0.1653（代码质量审计 12 限流器状态机契约与同步治理精读落盘——全库盘点 626 文件据实立项·基线 c1931e1c·纯文档零代码：V-01 域键口径分裂〔主抓取路径 Acquire 用 https://host 桶 vs fetch_rps 写裸 domain·配置失效·keepfrds 7.5 倍超速·CVSS 5.3·离线探针实证〕+V-02 concurrentCh 锁外替换〔:83 持锁写 vs :94/:107/:130/:152 无锁读·-race 实测 DATA RACE+取消路径永久阻塞·CVSS 7.5〕+F-01 第三套 key 口径；同包 circuit_breaker 正确持锁为正例；与 09 限流算法排除/10 map fatal 正交；整改三刀待实施〔统一 key+并发槽计数化+三锚阴性对照〕；证据 evidence/domain-limiter-repro.txt）。v0.0.1652（11 全案三刀——刀1 findBencodeEnd 先判长度再加法+computeInfoHash 纵深断言〔V-01 根修·CVSS 7.5〕+刀2 decodeString 同型同批〔V-02〕+刀3 防回归锚〔四态×两处+正常路径〕+阴性对照双层实证〔A=断言拦截·B=双退 panic 复现原案〕+F-01 时间解析第三层兜底——**11 全案收官**·审计系列 01-11 十一篇全毕）。v0.0.1651（代码质量审计 11 外部结构化输入数值边界治理精读落盘——全库盘点 626 文件据实立项·基线 c31b3840·纯文档零代码：高危 V-01 `qbittorrent/client.go:355` findBencodeEnd 长度串 Atoi 得 MaxInt 加法回绕为负绕过上界检查、`:314` computeInfoHash 负索引 panic〔离线驱动实测 slice bounds out of range [:-9223372036854775782]·CVSS 7.5·HTTP 用户可控+多个后台 worker 可达〕；同型潜伏 V-02 `fingerprint/bencode.go:131` decodeString 同类溢出〔实测 [:-9223372036854775789]〕；次要 F-01 时间解析忽略 err→cleanup_in_days 静默归零；盘点确认动态正则/除零/goroutine/资源关闭/JSON 标签健康不作主导；整改三刀待实施〔先判长度再加法+同型补齐+防回归锚〕；证据 evidence/bencode-overflow-repro.txt）。v0.0.1650（10 全案三刀——刀1 compileRegex 补写锁 double-check〔P1 fatal 根修·rss/engine.go 范式·§59.186 同型补齐〕+刀2 防回归锚〔并发压测 -race×10 绿+阴性对照 4×DATA RACE 实证+CI race 层加入 filter〕+刀3 同类核验〔rss×2+titleparser+httpclient 四处全绿〕——**10 全案收官**·审计系列 01-10 十篇全毕）。v0.0.1649（代码质量审计 10 共享状态与缓存并发治理精读落盘——承接 09 §6 点名候选·基线 263c4ceb·纯文档零代码：真 bug=P1 filter/engine.go:20-44 compileRegex 读侧 RLock 放锁后裸写 regexCache map〔:38-43 淘汰+写入无锁〕·离线驱动 8 goroutine 压测实证 fatal error concurrent map read and map write〔栈顶 engine.go:22〕+concurrent map writes〔栈顶 :43〕·map 并发写不可 recover 直接杀进程；漏网定性=rss/fetcher+engine/titleparser§59.186/httpclient 四处同类缓存均正确 double-check+写锁或 sync.Map·唯 filter 漏改；全库 32 处并发单点健康治理=补该点非重写；整改三刀待实施〔补写锁 double-check+并发锚+同类核验〕；授权与范围.md 落位 L0/I0·证据存 evidence/）。v0.0.1648（09 刀4 附录H+静态锚——三条铁律〔client 工厂+例外登记/ReadBody+DrainBody+err/NewRequestWithContext+BrowserUserAgent〕+TestNoBareHTTPBoundaryPatterns 四形态源码扫描〔阴性对照全红实证〕+Walk root SkipDir 工程坑修复+browser/fetcher UA 兜底顺修——**09 全案四刀收官**〔1644 transportSwitcher/1646 ReadAll/1647 构造/1648 锚〕·附录A-H 八约定）。v0.0.1647（09 刀3 请求构造收编+UA 单点+IYUU 合并——9 处裸 NewRequest→WithContext〔含 site_handler_crud 签名穿透 8 函数〕+9 处 UA 硬编码→BrowserUserAgent+IYUU 双 client 合并〔handler 自建删→公共工厂〕——三项残留全 0·09 刀4 附录H+静态锚待续）。v0.0.1646（09 刀2 响应体读上限与排空收编——17 处 io.ReadAll(resp.Body)→httpclient.ReadBody〔10MiB 上限〕·3 处 err 忽略处置〔主链报错/enrich 显式忽略〕+download_handler ctx 接入+imagehost 无界排空套 LimitReader+存量中文括号预扫——残留 0·09 刀3 请求构造/刀4 附录H 待续）。v0.0.1645（09 刀1 附 CI lint 根修——锚注释中文括号空格〔1636/1629 同款〕·proxy_switcher_test.go:107 gocritic commentFormatting）。v0.0.1644（09 刀1 消除全局 DefaultTransport 替换——P1 真 bug 根修：transportSwitcher〔RWMutex 动态切换·Register 一次性注入 host client 零 race 面〕+SetProxy 锁内切 rt+旧值 CloseIdleConnections〔连接悬挂顺修〕+withProxy 删+6 调用点去包装+接口 SetTransport+downloadURL 传 rt〔下载源图同代理链〕+锚三断言三方并发〔-race×10 绿〕——09 刀2 ReadAll 收编/刀3 请求构造/刀4 附录H 待续）。v0.0.1643（09 HTTP 边界资源治理精读落盘——承接 08 §5/§6 留作独立方向·基线 678a61fe：真 bug=P1 imagehost.Manager.withProxy 锁外替换进程级 http.DefaultTransport〔并发交叉污染·transport 注入修〕+17 处无上限 io.ReadAll/P1 响应体破裂〔download_handler/publish/executor·只 Close 不 DrainBody 且忽略 err〕+口径漂移 P2〔24 自建 client/9 裸 NewRequest/UA 硬编码/IYUU 双 client〕；公共单点健康〔NewSiteHTTPClient/ReadBody/DrainBody/UA〕·堵绕过非重写；四刀+新增即拦·存量登记·渐进收敛；后续点名 10 篇候选=filter/engine.go:20-44 裸写 map〔基线实测 fatal error 栈顶 engine.go:22〕·纯文档零代码）。v0.0.1642（08 回归审核四修〔用户实证复审〕——①锚注释诚实化：委托后 baseURLOf 已规范·BuildNexus 兜底成死路径·四态锚「防兜底被改/防新消费点」声明不成立〔临时测试实证〕——补 TestBuildNexusDownloadURL_Idempotence 独立锚〔真能触发〕②附录G §2.1-2.3 与委托后实态同步③model_test 函数名引用错④00-总索引状态滞后——方法论=锚防回归声明必须逐条可触发验证〔删兜底/加消费点攻击实验〕）。v0.0.1641（08 刀3 FrameworkLabels 补全+测试单一来源——5 常量〔值与 Factory.Create 逐字一致〕+Labels 13 项+model/factory 两测试手抄→map 派生〔Defaults 6→10 项全/AllFrameworks 6→13 项全〕+类型锚〔default 兜底回显输入·回显锚无效须断具体类型·luminance 无专属 adapter 白名单〕——**08 全案三刀收官**〔1639 附录G/1640 委托/1641 补全〕·锚边界诚实记录=factory⊃Labels 方向靠 checklist 纪律+基数锁兜）。v0.0.1640（08 刀2 baseURLOf 委托 resolveBaseURL——消费点端到端等价〔两消费点全经 BuildNexus 幂等兜底·四态最终 URL 一致〕+resolveBaseURL 注释修正〔补协议+去斜杠实态·「纯 Domain」标注历史叙述〕+回归锚 TestBaseURLOfEndToEndEquivalence〔验证口径=最终 URL·防 BuildNexus 兜底被改/新增直用消费点〕——08 刀3 FrameworkLabels 补全待实施）。v0.0.1639（08 刀1 附录G 站点适配器复用边界与框架标识规约——framework 权威来源+七处消费点事实表+np/nexusphp 双词汇域+FrameworkDefaults 有意不注册+URL 基址三 helper 实态〔siteBaseURL↔resolveBaseURL 三态等价/baseURLOf 裸取·BuildNexus 幂等兜底〕+「不凭函数头注释判语义」核对纪律+SiteConfig 构造约定域收编〔§59.574/575 病灶〕+骨架四族下沉边界+三次法则+维护触发表·00-总索引附录A-G 同步——纯文档零代码·08 精读收官刀2/3 待实施）。v0.0.1638（§59.578 附 BDInfo 结构铁证正向通道——Death Ship 案：original_disc 推理断点〔文案 pattern 中文「原盘」vs 站方规范 title「UHD Blu-ray」恒 miss+原盘无 MI 双 miss·dict note 挂账兑现〕：QUICK SUMMARY 锚〔build_profile §59.319 附十六同源〕→产出·三重防线〔tech.Encoded 前置不回加/MiniBD 排除导出单源/文案并集〕+锚四态——海的宝站方值集无原盘标签=站方限制非我方可修）。v0.0.1637（§59.578 刀3 收录关闭+可解释性——收录通道严格 migration-only〔前端按钮+函数+API 全清·后端 POST/DELETE 撤 GET 留·多环境单源〕+fetch_error 可解释性〔seed_list 透出+unsupported② tooltip+fetchSingle 失败刷新〕——**§59.578 状态模型重构全案三刀收官**〔1634 原子切换/1635 烙印闭环/1637 收录关闭〕·生命周期四路径全通）。v0.0.1636（§59.578 刀2 附 CI lint 红根修——gocritic commentFormatting 注释空格×2〔1629 同款·中文括号紧跟〕+改动行触发 staticcheck SA1012 nil ctx×2〔签名加参算新行·nil→context.TODO〕——lint --new-from-rev 严检把「改动行」当新行·存量 nil 形态被翻出）。v0.0.1635（§59.578 刀2 烙印+闭环——写点〔口径A 瞬态外全烙·三不烙边界：重获 fresh 基线/瞬态扩四形态/DB 失败 Warn〕+成功清印+清除清印〔deleteSeed 同步清·无行也执行〕+弹窗排除〔任一 hash 烙印整 name 排除〕+classify②接入〔snap 直读零查询〕+前端 unsupported 视图批量重置按钮〔§59.479C 先例〕+行内清除放行+锚四件——测试姿势坑=GORM First 复用结构体 NULL 指针不覆盖·断言查询用 fresh 变量）。v0.0.1634（§59.578 刀1 种子配置页状态模型切换——用户五轮讨论定案重设计：①收录列表二元身份（官组∪外站注册表）×获取结果正交 ②未获取=收录命中∨音乐·隐含未走链 ③禁转收窄纯化（原「禁转/无映射」no_mapping 剥离）④不支持=①不认识∪②获取终态失败〔隐含不支持转载发布·烙印刀2〕⑤no_mapping/external_group 状态消亡·外站组概念收回系统内部〔tag 留〕+classify 重构+seed_list 特判删+快照加烙印列〔不写不读〕+前端三页原子切换+筛选 value 容错+值域锚 TestSeedStatusValueSet〔§59.546 纪律平移〕——刀2 烙印闭环/刀3 收录关闭待续）。v0.0.1633（§59.577 附 编码日志剥离+清洗通道收编——猫 802365 SPHD 案用户指正：x265 [INFO] 帧统计混入 Statement〔<200 rune 启发式误纳〕两道门修〔extract 层 IsEncoderLogQuote 不入 Statement 保留 Body+词表签名纳入「简介引用清洗」存量零 migration〕+刀2 消费面漏改即修〔backfill buildThanks 无三辨门→ComposeStatement 单点·外站组行错 append 官组块防回归〕——方法论=形态像声明的非声明内容靠内容签名挡·新过滤语义双落点=分类层+词表层）。v0.0.1632（§59.577 附 external_release_groups 去重+唯一索引——243 复验发现 760 行〔crash loop 期间 INSERT OR IGNORE 无唯一索引无效·每轮+19×40 轮〕：migration185 去重〔MIN(id) 优先〕+idx_ext_group_name 唯一索引·不注册 AutoMigrate〔脏数据上建索引炸启动——migration 拥有 schema 权威〕——方法论=INSERT OR IGNORE 幂等依赖约束存在·新表交付=建表+索引+INSERT 三件套）。v0.0.1631（§59.577 附 migration184 空串炸库根修·243 crash loop——json_extract('') 对空串报错非返 NULL·UPDATE backfill 加三重守卫〔NULL/空串/json_valid〕——243 单颗脏行锁死生产·29 零空串纯运气——方法论=批量 SQL 消费 JSON 函数必须带守卫·探针过滤条件须与目标语句逐字对齐）。v0.0.1630（§59.577 附 刀2 回归审核收尾——SeedListItem 类型补 external_group 字段+status 枚举注释修正+checkbox 门注释更新+TemplateOverride 休眠通道挂账〔{{thanks_quote}} 不带三辨门·触发条件=首站配置该占位符〕）。v0.0.1629（§59.577 附 1628 CI 红根修——gocritic commentFormatting 注释空格+gofmt 三文件归位+watch 假绿二案〔watch 对已完成 failed run 退出码 0——CI 核验终态须 gh run view --json conclusion 直查〕）。v0.0.1628（§59.577 刀2 外站组转发——①已获取外站组走状态流〔§59.230 原则·external_group 状态仅未获取态·身份并行字段 tag〕发布门由此自开 ②声明三辨门〔官组/外站组/未知〕+小组模板「{group}小组作品，感谢原制作者发布。」〔只换第一句〕+SetGroupKindChecker 闭包注入 ③回流四点〔双锚/词表/形态锚天然兼容〕——刀1 回归审核观察 #2/#3/#4 全部随之消失）。v0.0.1627（§59.577 附 刀1 回归审核 P2 即修——PublishData/PublishTorrents 双页 STATUS_META 补 external_group 条目〔共用 listSeeds 数据源漏补·SeedConfig 已有〕——教训=新增状态值消费面清单须 grep 全部 statusLabel 消费方）。v0.0.1626（§59.577 附 audit flake 第三修——TestLogger_StartAndFlushOnCancel/FlushLoopBatchSize 同步化：cancel→Stop 精确同步+直断言零轮询〔前两修在窗口加码·本修消灭窗口〕·count=10 0.166s——方法论=异步组件测试找完成同步原语〔wg.Wait〕·轮询+超时是原语缺失的妥协）。v0.0.1625（§59.577 刀1 外站组识别——external_release_groups 新表+19 组冷启动+PigGoWEB 容错行+release_group 升列+三级链分类〔Ⅰ级救回/Ⅱ级标题/Ⅲ级快照〕+external_group 新状态+LookupExternalGroup〔@回退〕+CRUD 策展通道+前端六处+独立筛选桶+.iso 顺修——发布门刀1 保持关·刀2 声明扩展后开闸）。v0.0.1624（§59.576 附 1623 CI 红根修——staticcheck QF1001 命名布尔正形态+gofmt 五文件归位〔含 1613 存量漂移〕+watch 竞态教训〔push 后新 run 未注册时 list 查旧 run 假绿——CI 核验必须按 head SHA 对齐〕）。v0.0.1623（§59.576 馒头两案——§59.532 解挂〔Encode 透传门：mi.Encoded 压制铁证∧标题编码器词∧MI 格式词→保留 x264/x265·原盘/WEB/空隙三门天然覆盖〕+classify 连字符区分〔驼峰 BLURAY=encode〕+labels 消费断链〔assembleTags 放行位掩码键——馒头无 tags 表单域被 allowed 全杀→labels 恒 0〕+副标中字窄词表通道——馒头规则 vs v1.05=同构不冲突·真冲突=实现违反两规范）。v0.0.1622（§59.575 好学统计同步双 scheme 根修——hxpt base() 手搓 https://+Domain 改复用公共 siteBaseURL〔三态兼容〕·统计同步链 Domain=BaseURL 带 scheme 形态炸出 https://https//..·Token 修复前被掩盖+全 adapter 12 处拼接点扫描=其余 11 处有 HasPrefix 守卫仅 hxpt 裸拼——病灶记 08 篇：stats_sync 与 provider 对 config.Domain 两种约定并存）。v0.0.1621（§59.574 城市 cuhash 同步断链根修——stats_sync 手搓 config 漏 DownloadMode/DownloadURLTemplate 两字段→cuhashEnabled 恒 false→scrapeCuhash 永不执行〔§59.185 门修了但输入没接·设计实现脱节〕+两凭证实证区分〔tracker passkey 嵌 .torrent·cuhash=下载端点轮换凭证存 passkey 列〕——离线驱动验证 Passkey=站上当前 cuhash ✓——病灶记入 08 篇：手搓 config 副本漏字段）。v0.0.1620（§59.573 附 CI 首红根修——BDInfo 队列懒启动竞态：Enqueue 懒启动撤→显式 Start〔main 装配期·幂等〕+测试1 零消费确定性+测试2 WaitFor 轮询清空——race×10+CI race 层五包全绿——方法论=懒启动让测试与生产共享调度竞态·worker 生命周期显式化）。v0.0.1619（§59.573 TR 自动补挂 tracker 全撤——用户定案「一挂多=双计统计=禁·系统零此路径」：isDup→trackerAdd 补挂块撤除〔§59.541 附七②方向性错误·正解=站方注入 source〕+extractAnnounces 死代码清理+qB 零路径确认+全库残留复核=0——isDup 保留如实上报+日志）。v0.0.1618（§59.572 好学首颗三根因+关闸——①刀1 Success 标志〔终态误判〕②刀2 元数据走 query 参数〔multipart 只留 file·契约对齐〕③站方 API 不注入 source〔同 hash 无法做种·我方不可修〕→migration183 软关闸等站方·二师兄案复核=干净路径非补挂〔§59.541 附七①才是当年根因〕——TR 自动补挂撤留挂账待定）。v0.0.1617（§59.571 附 回归审核加固——闭包 strings.Contains→json.Unmarshal〔migration 四先例对齐·带空格漏判+嵌套伪命中两理论缺口封堵〕+好学任务焦点行同步——方法论=字符串匹配 JSON 布尔位=把数据形态当格式契约）。v0.0.1616（§59.570 附 好学 Token 字段错位修正——apiCall/Download/Upload 三处 config.AuthKey→config.APIKey〔api_key 列·馒头先例同款〕+注释两处+hxpt.md 同步——方法论=字段选型先查同 auth_type 先例读写闭环·DB 实态为准）。v0.0.1615（§59.571 IsPublishSupported 双源化——五站 publish_supported 漏标根修〔乌托邦/朱雀/海豹/肉丝/野马〕+双源〔json 声明∪form_config 派生〕+白名单前置+闭包注入〔site 包零 DB 依赖〕+锚两枚五态——方法论=migration 交付即权威·手工 json 是副本）。v0.0.1614（§59.570 补 前端 API Key placeholder 馒头提示移除——zh+en 两 locale·通用控件不硬编码站方说明）。v0.0.1613（§59.570 好学全切 Token——站方 API 文档驱动〔fxo_/fxr_/fxk_ 三凭据体系〕·cookie+CSRF→Authorization Token〔AuthKey 字段·apikey 复用〕·全端点迁移〔搜索=NP 兼容层/详情=开放 API/下载=fxk_ 两步/用户=ptppUserInfo/发种=open torrents Token+multipart 幂等〕·adapter 全文重写 587 行·CSRF 全链删除。v0.0.1612（§59.569 撤 stripLeadingPosterImage·瓷器简介海报保留——§59.541 附十一通用剥离误伤好学/瓷器〔cover 域≠海报要从简介剥〕·劳改所 DescrQuoteOnly 已覆盖冗余可删·恢复正常 NP 投递序〔简介含 [img] 海报+cover 框复制投递〕·锚三态反转。v0.0.1611（§59.568 刀1 补注+07 复审微瑕——:208 90s 唯一漏注补齐〔检查正则误匹配 §59.49〕+附录E 清单修正〔22→23/6→7/4+19〕+GetIntDefault 现状标注〔零生产调用·附录F §4.5〕。v0.0.1610（§59.568 刀4 补修 audit flake——WaitFor 3s→10s〔观测窗>busy_timeout 5s·build 门二拦闭环〕。v0.0.1609（§59.568 刀4 GetIntDefault+锚——缺省/解析失败/越界三路回退+Warn·max=0 哨兵·锚六态〔TTL 缓存需 InvalidateAll 踩坑〕——**审计 07 篇收官（刀1-4·全审计系列 01-07 完毕）**。v0.0.1608（§59.568 刀3 魔法值收敛窄版——365 族跨包命名化〔fingerprint/coverage/reseed 三常量·第五处哨兵定性〕+24h/5min 族分类盘点〔多语义局部·保留〕——方法论=治理对象是跨文件同语义族非同数值）。v0.0.1607（§59.568 刀2 调参清单+资源预算——附录F〔三层分层约定+config/setting/const 三清单+资源预算表+维护规则〕。v0.0.1606（§59.568 刀1 超时治理约定——附录E〔四类预算分类+注释三件规约+批量预算 helper 约定〕+23 处 Background 派生点注释补齐〔6 前注+17 本刀〕·残留零〔:273 复核抓漏补〕。v0.0.1605（§59.567 刀B 补修三高风险七文件 :memory: 收编——回归审核最严口径〔全库 59 文件余量→并发真实性核定 7 文件 8 DSN→unique+cache=shared〕·余 52 文件零 goroutine DB 渐进——方法论=修坑后必扫同类余量。v0.0.1604（§59.567 刀B 补修二 integration 三连修——用户复审 P1〔-race 复现〕：①setupDB unique+cache=shared〔同 audit 根因〕②mock 标志 atomic.Bool〔原 join 掩住的竞争〕③WaitFor 条件并入 DB 终态〔flag 中段置位·status=pending flake〕——count=10+整包 race×2 全绿。v0.0.1603（§59.567 刀B 补修 audit 并发连接分裂——build 门真实拦截〔WaitFor 轮询与 flush INSERT 并发→池二连→:memory: 分裂空库·原 sleep 串行化掩住〕→unique+cache=shared 惯例·-count=10 连跑绿。v0.0.1602（§59.567 刀C 编排不变量锚矩阵——附录D〔六不变量×组件·14 现有锚+2 语义锚+1 渐进〕+缺口补锚 3 枚〔重复投递 at-least-once/Engine+Watcher Stop 幂等〕+维护规则四条——**审计 06 篇收官（刀A/A补/B/C）**。v0.0.1601（§59.567 刀B sleep 治理——testutil.WaitFor helper〔锚三态〕+点名 6 处转换〔audit×2 场景×3·固定 300ms→条件轮询再 Stop〕+A 类注释定性 9 处·41 处全分类·B 类余 14 渐进清单。v0.0.1600（§59.567 刀A补 race 层 7 包对齐 06 篇建议——补 publish〔16.5s〕+rss〔13.5s〕·预跑验证可接受·06 交叉核验八组样例全中归档。v0.0.1599（§59.567 刀A CI race 层+三层分层——test-race job〔五高风险包 -race 每 push·进 build 门〕+test-full-race〔每日定时全量 -race 无 -short·覆盖 4 跳过项〕+fast 现行保留·本地五包预跑全绿。v0.0.1598（§59.566 刀1 补刀二轮多行链 2 处——回归审核最严口径〔裸 h.db. 行级〕再抓 filter/rss 两处独行链·账目闭合 builder 20+多行 2=22——最严口径终态 0·方法论=清零复核逐轮加严直至形态无关）。v0.0.1597（§59.566 刀1 补刀 builder 链 WithContext 22 处——用户复审发现「残留零」系词表限定伪清零〔直链 141 之外漏 Order/Select/Raw 等 builder 起点〕·宽词表分类器补插 19+helper ctx 穿参+DB() 豁免·宽口径残留 0——方法论=清零声明口径必须与词表绑定）。v0.0.1596（§59.566 刀4 热路径索引 EXPLAIN 实测——243 生产只读九查询族·七命中索引/一全量语义预期/一 LIKE 固有·观察项=publish target_site 排序 TEMP B-TREE〔80 行级可忽略〕——**审计 05 篇收官（刀1-4·无需新增索引）**。v0.0.1595（§59.566 刀3 多步写原子性盘点——38 候选四类全分类〔A 事务先例 7/B 周期自愈 9+代表注释/C 单实体+副作用 10/D 迁移幂等 3〕·结论=无需新增事务〔自愈通道优先于事务·client CRUD 先例已覆盖唯一真实形态〕）。v0.0.1594（§59.566 刀2 无界查询定性——三站点型〔窗口合规注释/做种索引全量语义+5 万增长观测 Warn/小表定性〕·五点刀1 已覆盖 ctx——方法论=防御是增长可见非提前截断）。v0.0.1593（§59.566 审计 05 刀1 API 层 WithContext 收敛——141 处全清〔签名分类器五步法：30 处 `_` 重命名+helper ctx 穿参+139 批量插入+1 闭包人工定案=任务 ctx〕·残留零·dashboard/torrents/client 三大头收编。v0.0.1592（§59.565 刀2 D2 三处 limit 收敛——parseLimitQuery 单点〔默认 20·clamp 200·Warn 观测〕+三接线〔publish 旧模式真缺口补上限/seeding 两处静默 clamp 收编单点〕·锚六态——**审计 04 篇收官（D1-D4 全落地）**；核验纠错=seeding 原有 clamp 我漏看。v0.0.1591（§59.565 审计 04 刀1=D1+D3a+D4〔零行为合并〕——D1 五域 21 处 5xx err 透传收敛〔稳定文案+日志·detail 不透传原始 err·双清零复核〕+D3a decodeJSONBodyStrict〔DisallowUnknownFields+单值检查·锚三态·新接口用〕+D4 附录C 错误码语义约定〔status=协议/code=分支·message 五条契约〕·D2 三处 limit clamp 独立刀待发。v0.0.1590（§59.564 刀B 5xx 业务日志窄版——三文件 17 处「有 err 无日志」补 Error〔op+标识+err·execute/batch_fetch 实态零落点=未初始化 503 不合边界〕——**审计 03 篇收官（刀A/B/C/E·刀D 归 04）**+任务焦点清理〔仓库安全收索引+版本行瘦身〕）。v0.0.1589（§59.564 审计 03 刀A+C+E——createResultLogged 发布落库 Warn 九字段〔不反转终态〕+副作用日志化五处〔bootstrap std log 兜底〕+附录B 日志字段约定）。v0.0.1588（§59.563 刀C flush/free_end 残留 ctx 清账——**审计 02 篇收官**）。v0.0.1587（§59.563 刀B EventBus 停机接线——closed 守卫+排空+Stop 链补位）。v0.0.1586（§59.563 刀A 低风险合并——清理/站批接 baseCtx）。
v0.0.1585 及更早（§59.562-548/§仓库安全/§59.560-559/…→§59 全部过程细节见 docs/31-模块设计决策记录.md——grep "§59.X" 定位章节·docs/32 站点文档同索引。

**主线状态一句话**：NP 架构 62 站目标站适配收官（§59.450 知识库落盘）；音乐发布链已建成（皇后/海豚/乌托邦三源+MDP+翻译层·§59.466-492）；辅种覆盖体系六刀收官（§59.535）；发布链四维治理完成（终态单点 §59.546/三态三面四色 §59.547/禁转复检实态化 §59.548/媒介源保真 §59.549/官组零推导 §59.550）；**代码质量审计系列 01-07 全七篇收官**（§59.562-568·docs/代码质量审计/·v0.0.1582-1613 三十二连发·附录A-F 六约定·WithContext 163/5xx 透传 21/limit 三处/Stop 链+drain/EXPLAIN/CI race 三层/sleep 治理/超时注释 23/365 族命名化/GetIntDefault——07 复审通过）；**08 站点适配器复用治理全案收官**〔三刀 v0.0.1639-1641：附录G 规约/baseURLOf 委托+四态锚/FrameworkLabels 补全+单一来源+类型锚——附录A-G 七约定〕；**09 HTTP 边界资源治理全案收官**〔四刀 v0.0.1644-1648：transportSwitcher 根修 P1/17 处 ReadAll 收编/9 处构造+UA+IYUU/附录H+静态锚——附录A-H 八约定〕；**10 共享状态与缓存并发治理全案收官**〔三刀 v0.0.1650：compileRegex 写锁根修+并发锚+CI race 层〕；**11 外部结构化输入数值边界治理全案收官**〔三刀 v0.0.1652：bencode 先判长度再加法+同型+锚+阴性双层〕；**12 限流器状态机契约与同步治理全案收官**〔三刀 v0.0.1654：OriginKey 单一规范化+计数式信号量+三锚三阴性〕；**13 字符串匹配边界与查询语义全案收官**〔三刀 v0.0.1656：LikeEscape 公共单点+ESCAPE 补齐+三层锚三阴性〕；**14 外部响应解析与序列化契约治理全案收官**〔三刀 v0.0.1659：形态重写+降级+零值契约+收敛单点——前置验证销案〕——审计系列 01-14 十四篇全毕·附录A-H 八约定；当前活跃焦点见下方「下一步」与「活跃观察项」。（§59.149-§59.233 历史演进全录见 docs/31——grep "§59.X" 定位）

**下一步**：
0. **代码质量审计 15 规则引擎条件字段语义契约（精读完成·三刀待实施 v0.0.1660）**——全库盘点据实立项（非预设·避开 01-14）；V-01（中危·确凿功能缺陷）docs31:7757 规定 `downloadUploadRatio` 上传=0 为 +∞、:7674 黑车检测核心需求，而 rule/evaluator.go:137-141 与 seeding/rule_evaluator.go:133-139 零上传返回 "0"（另加未声明 TotalSize>0 门槛）——离线实测零上传 value="0"、有上传对照 5.0000，黑车「只下不传」下 `>阈值` 恰好不命中；scoring.go:60-71 零除数正确处理构成遗漏佐证。V-02（中危）两个 ExprEnv（rule/evaluator.go:338-377、seeding/context.go:12-56）与 buildExprEnv 均无该键，真实 expr.Compile 实测 `unknown name downloadUploadRatio`（对照 ratio 通过），文档 42/42 全覆盖失实；前端 SeedingRuleList.vue:381/601 暴露该字段、实战 20/24 条用 expr。F-01（低危·需取证）未知操作符 `default: return true` 致拼错规则静默生效。三刀：①零上传 +∞ 语义+去门槛+契约锚 ②两个 expr 环境补字段+编译锚+42/42 覆盖锚 ③默认行为显式化+静态锚+三阴性对照。与 01-14 正交。授权与范围.md 已更新（L0/I0）；证据 evidence/rule-field-contract-repro.txt。第 16 篇候选已登记：Truncate(24h) 日界偏移（9 处·依赖部署时区形态）。按 15 §4 验证。
0. **代码质量审计 09 HTTP 边界资源治理（已收官·四刀 v0.0.1644-1648+联合回归 165fb854）**——承接 08 §5/§6 留作独立方向；真 bug=P1 `withProxy` 全局 transport 替换+P1 响应体 17 处无上限 `ReadAll`；口径漂移 P2=24 client/9 裸 `NewRequest`/UA/IYUU 双 client，公共单点健康（`NewSiteHTTPClient`/`ReadBody`/`DrainBody`/UA）——堵绕过非重写。四刀全落地：①transportSwitcher 根修 ②ReadAll 收编（`ReadBody` 12→29）③请求构造+UA+IYUU ④附录H+静态锚；文档三处状态同步完成（09 篇状态/00 索引/docs31 §59.579）。附录A-H 八约定；09 及 01-08 全九篇收官；00-总索引驱动逐篇推进
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
- **好学重适配（§59.533 进行中·2026-11-15）**：站方改版离开 NP（Next.js+/api/v1 REST）·摸底两刀+收官完成（发布 FormData 15 字段+CSRF double-submit flux_csrf↔x-csrf-token/section-dict 值集 70 制作组含 FRDS/tags 9/详情 /detail 端点/download ✓/RSS 无）；四定案=内容全放开+旧特化（拦截/年级教材注入）废+新 framework hxpt（API 直发）+price 恒不传+anonymous 系统默认；刀2 获取链完成（HxptAdapter+factory+migration169 framework 刷+publish 临时关+双 JSON+端到端七链全绿·v0.0.1479 已发）；刀2-5 工程全链完成（v0.0.1479 获取链/1480 发布链+值集/1481 DryRun 借壳验证+tags 断链修复·五键全中 dry_run_ok）；**§59.572 关闸等站方（2026-11-25 用户定案）**：首颗实测三根因全破案——①Success 标志+②query 参数契约已修〔v0.0.1618〕·③站方 open API 不注入 source（web 路径注入·指纹库实证）→同 info_hash 下载器去重无法做种〔我方不可修〕→migration183 软关闸（is_target=0+form_config.enabled=false·json 声明保留可重开）——**重启前置=站方 API 补 source 注入+用户举报删 23941 反馈 bug**；观察项：站方修复通知/重开后契约复验（含 descr 通道刀3 nfo 定案）——观察项：source 域缺失站方容忍度/重复发种形态/promotion 非 null 值形态；~~上传权限 403~~（§59.533 附三已实证=账号权限非系统问题）
- ~~**压制编码器词组装死锁（§59.532 挂账）**~~（**2026-11-25 销账：§59.576 解挂提前实施**——用户定案（馒头 1298316 冻结案=第二次咬人）·Encode 透传门（mi.Encoded 铁证信号替代原 SourceType 启发式）+classify 连字符区分+六态锚·ground truth Zhizn 复跑 HEVC→x265 ✓）；另 Dr Strangelove 简介嵌 MI 案挂账（原站嵌套引用·用户定后处理）；引用块消费面两理论窗口零实证挂账（statement 内嵌来源规格行+desc_raw 国语/粤语/短剧未剥 quote——§59.540 附七盘点·设计无影响·误报系人工分析非系统）
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
