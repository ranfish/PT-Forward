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

**版本**：v0.0.1326（§59.460 附三续 可乐文档回归审核+发版时序修正——①回归七项：RSS 策略表差异标注/getUserTorrentList 已实测+myPeerStatus 端点名存疑（代码 myPeerStatistics vs 站方文档 myPeerStatus 复验再试）/detail 已解决标注/特化方案更新/节流定性修正（非环境层是站方限流器）/双钥匙行（tracker passkey 与 RSS Key 两把不同钥匙）/待挖清单刷新②发版时序修正（用户指正）：v0.0.1325 build 在 tag 前——29 版本号 v0.0.1324-2-g…差一版+数据级验证漏做；修正流程=commit→AGENTS 版本行→release-tag 先行→build（describe 已=tag）→restart→数据级验证 version==tag 闭环③docs/31 §59.460 附三补落盘（1325 轮漏——强制清单第 1 条）。v0.0.1325（§59.460 附三 可乐 RSS 通道接入（用户供链接推翻"无通道"定案）——torrentrss.php?passkey= 完整可用（NexusPHP 兼容自述 200/50 条）；此前 403 cookie_authentication_forbidden=探测方法错误（passkey 通道禁混带 cookie——每通道单一凭证铁律与 x-api-key 同族）；cola RSS 策略表修正（guid=详情 URL 非 hash→IsFakeHash 自动侧载算真 hash 肉丝同款/guid_regex /torrents/(\d+) 提 id——link 路径形态 query_param 恒 miss/RequiresSideLoading）；DetectDiscount 403→Web 面 promotion 降级（download:{num,den} 分数：num==0 FREE——全站免费站免费语义在 promotion 不在 feed）；离线驱动 50 条全解析 ✓；附二限流三层定案：站方=Key 维度请求限流跨端点共享（60/min 突发 20）非端点限流+www CF/IP 层（无 Key 也撞）/我们 GlobalLimiter 按主机分桶默认 30/min 双域——生产链已在保护内烧配额元凶=探针 curl 绕 limiter 非生产；明日冷启动三发收口（Download 端到端/DetectDiscount promotion/RSS 侧载真 hash）。v0.0.1324（§59.460 附二 可乐获取链补全+wiki 八篇全量——①Stats 做种数降级：myPeerStatistics 不在兼容面（403）→行为驱动 fallback getUserTorrentList 翻页计数（条目 {torrent:{size,source}} 嵌套；M-Team 主端点健康零变化）②Detail 降级：/api/torrent/detail 403→Web 面 GET /api/v1/torrents/{id}（cookie 通道字段更全：source_info_hash 双 hash/完整 MI/release_group+official/promotion 分数/审核状态机）③RSS 定案无公开通道（三端点 404/route_not_found/拒认证+roadmap 未列——订阅获取不可用站方上线再接）④wiki 八篇全量落盘：billing-rules 下载全免+24h 发种保护（非发种人辅种 30% 计账）+发种奖励 72/144/360h/site-levels 7 级+分享率考核/客户端白名单 qb4.0~5.2.4 禁增强版/官方转种油猴脚本（auto_feed 衍生——转种友好站）⑤官方插件面契约（前轮）：match hash 反查 500/批 CC cookie 直用/ReelCoreHQ org 公开（主仓库 reelcore 私有+roadmap gh-pages 进度镜像——Go 重写+Torrust 血统）⑥表述治理：可乐与 M-Team 严格区分（站点行为纯可乐视角/无关联显式声明）——用户指正防认知分叉；Download 端到端复验挂明日冷启动首验（修复行为级证据完整）。v0.0.1323（§59.460 附 可乐下载链终修——序列探针实证带 x-api-key 的 GET 到 www /downloads 被 CF 掐连接挂 30s（非 403 响应形态——首版"403 后无头重试"走不到：第一发就网络错）；修法=下载 URL 通道前置判定（按 URL 前缀==resolveBaseURL：API 兼容面同域带头/M-Team 形态 vs 其它域 Web 通道无头——可乐 www /downloads 签名 URL 自含 exp+sig 鉴权+站方 MoviePilot 文档同款下载 GET 不带头；403 无头重试保留兜底；馒头 API 同域带头零变化）；用户问答定位法：三面健康度对照实验（www 1.4s/api 探活/profile 3.2s 全通）推翻"节流"初判→传输栈对照（生产栈/标准栈×首页/下载 URL 四象限全通）→同 client 序列复刻（genDlToken ✓+带头 GET 挂 30s）锁死根因=带头行为本身。v0.0.1322（§59.460 可乐获取链契约三修+架构名称独立 cola——json.Number 双兼容（search id+stats memberCount 四字段：M-Team 字符串/reelcore 真数字同解——纯 string 在数字形态 unmarshal 失败）+resolveBaseURL BaseURL 优先（可乐三面分离 domain=www Web 主机/base_url=api 兼容面——纯 Domain 构造把 profile 打中 reelcore Web API 超时；馒头等 API 即主域站 base==domain 零变化，全库 base_url≠domain 仅可乐一例）+签名 URL 下载 403 无头重试（genDlToken 返回 www 主机 /downloads/<hash>/<id>?exp&sig——带 x-api-key 触发 www 路由 scope 校验 api_key_scope_denied 403，无头裸 GET 即 200 真种子 d8:announce 32894B announce 嵌 tracker.colapt.com+passkey；403 重试一次保馒头带头 200 原路径零变化）；实测终态 Search ✓（Bluey 2 条 id=13115）/Stats ✓（ranfish/86MB/bonus 13.44）/Download 修复后因节流 deadline 未复验挂待冷却；CC 链收官（配置在独立表 cookie_cloud_configs 非 system_settings 查错表一轮+CC 数据可乐 cookie 在 www.colapt.com 域 reelcore_session×2+DomainMatches cdBase==tdBase 本覆盖 www 形态（错误诊断期加的对称加回防御已撤——错误诊断产物不留）+真根因=SyncAll 只遍历 enabled 站（新站骨架铁律设计行为）→用户前端三步→同步 completed 100 站+cookie 落库 201B ✓）；**架构名称独立（用户定案）**：可乐 reelcore 与 M-Team 各自架构不混名——framework=cola（双资产+Factory cola 分支→MTeamAdapter 实现层同构复用架构语义独立+RSS 策略/默认路径/发布配置/API Key 测试端点 cola 分支+detect reelcore 特征→cola+前端 frameworkLabels Cola (ReelCore)（修站点详情页架构标签显示 M-Team 瑕疵）+migration142 存量刷正）；**摸底方法论（用户指出）**：三面分离站摸 Web 面与摸 API 面必须分离——凭证（cookie vs x-api-key）/头（浏览器形态 vs 仅 key 禁 Cookie/Authorization/Origin）/解析（HTML vs JSON）/节流层（CF vs 站方）四维全不同+混合形态警觉（API 签发的下载 URL 指向 www Web 通道——判断通道归属看 URL 主机+路径不看签发来源，403 案即此）详见 docs/32/cola.md 方法论节；待挖：automation 三步字段契约（等勾 scope）/Web 表单值集/站规/官组 is_source/RSS/getUserTorrentList/search 反查 detail 特化（兼容面无 detail 端点）；v0.0.1320 及以前版本过程叙述见 docs/31（§59.459 附三/§59.459/§59.458 索引）

**主线状态一句话**：一种多站收官（§59.149-165）；一站多种已实施+端到端验证（§59.166）；憨憨源站适配收官（§59.167-168——PTGen 资产 batch 路径失败已整体回滚重议 §59.168 教训节，Tab1 20 字段数据源重新对齐中（#1 片名/#2 译名已定案））；手动截图簇传播缺失修复（§59.169）；预览自动保存三连击修复（§59.170 三修生产验证毕——243 复验收官 §59.347）；MI 三合一修复（§59.171——PT31 闭环实证）；PTGen 强制刷新（§59.173）；引用合并采集（§59.172 附八）；RSS 磁盘守卫 blocked 状态机（§59.174）；幸运十连拒修复（§59.175——待站方解禁后重发验证）；发布前置检查公共方法（§59.176）；剧集状态标签两层架构（§59.177）；引用块首行空行修复（§59.178）；搜索关键词中文前缀污染修复（§59.180——strippedCJK回退删除）；副标题匹配路径加固（§59.181——括号剥离+DIY web限制）；优堡@子组署名修复（§59.182——hasGroupSuffix 加 @，中文副标题覆盖英文主标题根治，Just Mercy tid=109323 定案）；站点级种子下载限流可配（§59.183——95/小时 提为 sites.download_hourly_limit（0=不限/默认 95，v0.0.904 语义修订），站点详情-网络入口，优堡 95 误拦定案）；验证层三态门重构+B 补线实施（§59.184——三态门/REPACK 合并/删无组名早退/size 显示等价 v3/Resolution 4K 等价/季标记反驳/中文链内补线+合集序号前缀剥除，v0.0.905-907，B 补线中文链 23 命中实战首捷）；恢复下载链鲁棒三件套+cuhash 自动同步（§59.185——失败落日志/cuhash 门修复+懒刷新/下载三层兜底/搜索退避重试，v0.0.908，keepfrds 429 冻结连坐与城市 cuhash 案定案）；SourceType 血统反驳（§59.185 附——tech 字段表补 ST+连字符变体归一，v0.0.909；挂账 WEB 跨血统不对称）；懒编译正则缓存并发首写 fatal 热修（§59.186——tokenReC2/inferReCache 改 sync.Map，OTA 冷启动批量恢复崩溃实证，v0.0.910）；UBits4 批三修（§59.187——组名变体同族豁免 GroupFamilyResolver/季包 EP 范围签名+文件级回退/混合词归一化降级轮，v0.0.911，28/28 全回优堡三路径实战验证毕）；搜索关键词剥版本词（§59.188——PROPER/REPACK 族词站方标题不带则 AND 0 结果，加勒比 tid=105267 案定案，v0.0.912）；兜底搜索 nil-stats 崩溃热修（§59.189——retrySearchExcluding 传 nil 致 tryL2SearchCore 空指针 panic 重启，入口防御+stats 线程化，v0.0.913）；中英版式词归一+全规格词无标题判定（§59.190——中文修复版≡REMASTERED 规则 B 误杀/浪人 4K修复版丢片名，v0.0.914，SSD4 复测 4/5 回归浪人双修协同实证；挂账：Resolution 双词假冲突）；规则 B 拆分（§59.191——EditionInfo 营销版式词移出版本反驳/REPACK 重发布标记保留，SSD5 反向案三连+titleless 词表扩展，v0.0.915）；关键词 and/& 词形+下划线分隔（§59.192——Queen tid=410220/BOHEMIAN tid=271609 案，v0.0.916；挂账：浪潮站方笔误不修）；UHD 前缀消解+词内连字符组名误提取（§59.193——Queen tid=3020 ST 假冲突/Blu-ray 连字符提取组名 ray，v0.0.917，b 方案六连回归同批实证）；中文关键词净化降级轮（§59.194——版式词残留/词内版式后缀/中点变体，午夜凶铃 tid=270313 等六案，v0.0.918）；获取链 PTGen 空海报门修复（§59.195——Poster!="" 门致流控降级期 Tab2/Tab4 双空，doubaninfo 种子名形态已可用实证，v0.0.919）；元数据获取错挂五修（§59.196——侠女×Valerie 同年同组错挂案：主轮纯 CJK 补 area=1/loose 双盲拒绝/短词锚守卫/恢复链 tid 回写 coverage/Resolution 像素形态优先，v0.0.920）；词内连字符防御双侧证据（§59.196 附一——DTS-CMCT 前段误杀致组名空→源站优先跳过，永安镇故事集未从不可说恢复定案，v0.0.921）；恢复任务终态必落日志（§59.196 附二——SSD5 批 50/50 前端轮询假象定案，task_id+found+elapsed 终态入日志；PT30 compose 缺 restart:always 发现，v0.0.922）；3D 封装反驳（§59.197——300勇士 HSBS×HOU 同体积同组错配预判案：TechProfile 增 Stereo3D+规则 A 双显异值反驳，v0.0.923）；4K修复版 Resolution 假冲突兑现（§59.190 挂账——午夜凶铃六站候选全灭实战现形：版式语境 4K 不进 Resolution 交 size 仲裁，v0.0.924）；Phase-1 年份剥离降级轮（§59.198——站方年份笔误第四案拾芳 tid=278834 搜索层解锁：StripYearToken 导出+最末兜底轮，生死格斗=PT30 批次静默空返回非 bug/黑潮=纯配额/CMCTf 不修/area-1 补线零案例挂账，v0.0.925）；搜索异常页检测（§59.199——并发限流 503 轻量页静默 0 行盲区：24 并发 17×503 实证，0 行无空标记=站点错误+退避重试，生死格斗 Phase-2 全盲定案，v0.0.926）；E+数字合集序号前缀剥除（§59.200——007 合集 E09 金枪人案：站方标题无集号词 AND 0 结果，正则统一纯数字/E 形态，v0.0.927）；数字片名家族三守卫（§59.201——65.2023 秒拒案：2-3 位数字+紧邻年份=片名，前缀剥除/全规格判定/行首短数字三处豁免；BBC王朝=跨语言不可译手动恢复 tid=77856，v0.0.928）；Phase-1 原始全名兜底轮（§59.202——优堡丢弃数字词形态：65 探针四形态实证 tid=42 被埋，全名窄化首行命中，l2:priority-rawname，v0.0.929）；SSD8 批三修（§59.203——错误注入对：REPACK 年前形态边界锚吞词致规则 B 失明+recheck incomplete 判失败清错种+fallback 实际站点回传；绝望的牛仔=海豹虚标 size 骗 verify 注入校验兜住实证；色即是空 I.II 关键词丢片名挂账，v0.0.930）；SSD8 收官（§59.203——活着=规则 B 拒 REPACK 版直中非重发版 tid=77804/克拉之膝=CC 版 recheck 判败清错种+次优站大青虫递补成功，三修复实战验证毕）；关键词结构标记三修（§59.204——BBC王朝全五集阻断词/色即是空区间阻断词+罗马合集结构词：集数中文数字/年份区间剥除/结构词不构成标题触发中文回退，v0.0.931）；多部合集续集号对（§59.205——色即是空 I.II 合集 sequel 误杀：提取策略不对称 CJK路I=1×英文路II=2，罗马对命中返回0不反驳；数字对假阳性不检测挂账；PT30账号限制类不可见实证，v0.0.932）；字母数字粘连拆分+词数上限剥规格词（§59.206——Aliens2 案：纯数字尾粘连拆分（分辨率后缀形态不拆——拆出独立1080P实测致0）+>5词剥规格词（站方词数上限5实证），v0.0.933）；碟片发行商品牌血统反驳（§59.207——克拉之膝 CC/FLAC×DTS 双版同size同组案：Criterion/MoC/WAC 品牌词回归规则 B（营销版式词保持移出），tid=73609 源站直中，v0.0.934）；挂账清理（§59.196 附三——错挂受害面扫描零嫌疑收官+降级轮五轮 rc 日志销账 §59.194，243 已自动升 v0.0.934，v0.0.935）；A 组挂账清理（§59.209——异常页结构签名通道（人人 div 主题实证+spring 503 判别验证）/size 虚标定案不修（真相在 .torrent 内三层已闭环）/数字合集定案不修（零案例），v0.0.936）；pieces_hash 批量循环 ctx 过期短路（§59.210——PT31 1.28 万条 GORM 洪峰：预载循环漏 ctx 检查；对账附记：3746 行零错挂/日志静默=PTF_LOG_LEVEL=error，v0.0.937）；@子组署名官方键回退（§59.211——243 三种无源站映射案：OfficialGroupKey cXcY@FRDS→FRDS 单点+LookupGroup/GroupFamilyResolver/getSitePriority 三消费方接入，v0.0.938）；@子组署名终案（§59.211 用户方案 B——@ 后段即官方组：util £ 规则跳后段 FRDS 与 dash 语义统一+titleparser £ 优先修 dash 垃圾段，v0.0.939）；retryBlocked 免费重校验检测补跑（§59.212——243 四种 3 分钟 expired 案：blocked 项被 seen 门跳过检测从未运行→零值误判 free expired，状态机生产端到端从未成功；补跑检测+不可得保持 blocked；243 pt3/pt5 仍 100% 待清至 50GB+，v0.0.940）；全站搜索调试日志+searchprobe 工具（§59.213——SSD9 批不走官站调查：MINBD↔MiniBD 词形变体定案/密阳 app 进程恒 0 行谜团待 req/resp 日志收口/雅尼括号名+0 0兆赫关键词缺陷挂账，v0.0.941）；品牌反驳相对化（§59.214——密阳假杀案：搜索恒正常 parsed=7 实证，vr=1=tid=2670 被 §59.207 绝对品牌反驳，改集合级兄弟判定；本地复刻假阴性=门6 短路测试方法论教训，v0.0.942）；全角冒号副题段降级轮（§59.215——柯南剧场版16案：站方副标题只含副题"第11位前锋"，主关键词整段 AND 灭，ColonSubSegment+Phase-1 降级轮；柯南12/14=版本消亡定案，v0.0.943）；行首短数字守卫越权修复（§59.216——0 0兆赫案：守卫仅在其余 token 无标题时生效；杀人回忆=同规格异 mux 定案手动（tid=5174 名/size 三合 recheck 0%），v0.0.944）；CJK 后挂括号注记剥离（§59.217——雅尼案：注记被中文前缀整段带走致 keyword 残括号丢片名，stripCJKParenNote 前邻 CJK+内容 CJK 限定剥离，tid=77372 同版本在列，v0.0.945）；注记噪声词+版式后缀循环剥净（§59.218——勇敢的心/死亡诗社/角斗士三案：音轨注记 国英双语 主路径剥/蓝光版入词表/两连后缀循环剥净，tid=4260/4609/4209 全同版本在列，v0.0.946）；批期空页限流重试+BFI 词典归位（§59.219——并发期带标记真空页绕过异常检测（容器探针 6/6 恒 18 行实证）空页免费重试一次；BFI 误入 platform 流媒体槽致单侧反驳移回品牌位，v0.0.947）；规格词变体归一+组名前缀回退（§59.220——MINBD→MiniBD 有界词汇归一；CMCTf→CMCT 前缀回退三消费方（LookupGroup/getSitePriority/GroupFamilyResolver），§59.198 不修定案重议撤销，v0.0.948）；§59.220 附：GroupFamilyResolver 前缀回退漏装补丁（批量编辑断言中止静默漏文件——生死格斗 group_refute=5 实证选站侧已生效唯验证层漏；教训：断言通过也要 grep 验证数量，v0.0.949）；SSD9 支线终局（生死格斗 tid=32973 不可说 priority 命中收官——20 孤儿 19 自动恢复+1 定案手动，16 个官站 priority，§59.211-221 十一修全数实战验证毕 09-14）；MTeam 成人区双模式搜索（§59.222——3dsvr 六连案：mode:"normal" 硬编码搜索层一刀切，改双模式+Adult 标记+发布层天然隔离，成人不限制孤儿恢复/辅种但不可发布（用户定案），v0.0.950）；MTeam API 搜索可观测性（§59.223——savr 六连空结果案：searchViaAPI 三级日志 req/resp/API 错误码对齐 §59.213 NexusPHP 标准，v0.0.951）；MTeam API code≠0 退避重试（§59.224——savr 六连批量限流案：code=4 并发窗口瞬时错误单独重试即恢复，code≠0 从返回空结果改为返回错误进入 searchWithBackoff 退避链，v0.0.952）；JAV 番号专用恢复路径（§59.225——复用 compliance.DetectAdult 成人识别基建：番号即关键词（馒头标题=纯编号），优先馒头 adult 未中全站兜底；MDVR-400.8K/SVRT-079_4K 规格词问题根治，v0.0.953）；刷流 72h 兜底失联对账（§59.227——249 生产案：record 状态机与下载器失联无对账（archived×qb做种/无record孤儿）→ 对账重激活（>10min 防抖+deleted/deleting 纳入）+ syncUnmanaged 三修（DB 真相源/LOWER 归一/comment 详情页 tid 回填），v0.0.954）；朋友站头区引用/鸣谢采集收官（§59.172 主修+附二~七——锚三形态/门槛豁免/引用=干净文本全剥图+OrigFull 双份修复；PT31 Top250 全量实证 190/190，10352 站方禁转定案不修）。Tab1 数据源对齐收官并实施（§59.226-228——21 字段三分区/三源重构/canonical 单点）；刷流 72h 兜底失联对账（§59.227——249 案实战闭环）；fnos DB 治理+retention 守卫修复（§59.229——23G→1.7G）；无映射筛选语义修正（§59.230——站方标题重判）；①b 搜索降级链补齐（§59.231——刘老庄案簇兜底首捷）；region 剥离 dash 锚保护（§59.232）；组名识别统一（§59.233——@锚+资产扫描+锚兜底三层+@复合词条资产化 21 形态）。

**下一步**：
0. **NP 架构目标站适配告一段落（2026-11-02 用户定案）**——62 站收官经验已落盘 docs/31 §59.450（资产可信度谱系/槽位语义陷阱/媒介六形态/codec 两极/音频承接库/HDR 三模式/tags 雷区/键双体系/结构异常/双零验证/抓取谱系/站点分型与 SPA 边界十二节知识库）；二师兄（SPA 改版）终止留档 docs/32/piggo.md 附（未来重启前置=渲染 DOM+提交端核验）；62 站首颗真实发布均遗留+§59.433 三站人工审核挂账照旧；新架构站（SPA/API 化）适配待用户指令立项
1. **孤儿恢复优化线收官观察（§59.182-225 三十一修，09-10~14 十二批实战验证毕——优堡57/Disney83/UBits/SSD4-9/TTG/馒头成人区 MT9）**；遗留观察：辅种侧三态口径回流、柯南站方资源形态
2. **辅种 L2 三态门口径实战回流观察**（§59.184-185 Disney 批 83/83 全正确处置已闭环；遗留观察：refute-neutral 口径辅种侧回流、中文链 l2:*-cn 命中形态、预存 HDR 严格性 DoVi HDR vs HDR10、keepfrds 用户下载配额阈值）
3. **Tab1 数据源对齐收官并实施完成（§59.226-228——v0.0.956）**；组名识别统一三层+@复合资产化（§59.233——v0.0.963/964；cqkyjj2 及 @ 复合 21 形态已 seed 化随版发布，无手动项）；Tab1 表格优化自检四修（§59.235 P1-P4——composeMedium 单点/PTGen 后置提取/BDAV 容器门/分发方词表，v0.0.967-972，P2 终修后引出 X 重构定案）；**X 重构已实施（§59.236——作品信息单通道 v0.0.973：runMainlinePTGen 主链必经/kdouban 逆向删除/desc 站方原文切断/四键 JSON 派生/§59.235 P2 补丁架构性取代）**；译名陷阱修正+缓存 enrich 补提（§59.237——v0.0.974/975）；UI 五轮优化（§59.238-242——v0.0.976-980：冗余行删除/标签表头紫/筛选下拉直选/预览区序号重排+色系终局）；fnos SCP 更新流程实证；译名陷阱修正（§59.237——TranslatedTitles 唯一真源：端点 foreign_title 恒=中文陷阱实证/bbcode 提取独立正则/kdouban 反置语义发现，v0.0.974）；fnos OTA 重获后数据级验证已收官（§59.346——PTGen 族键/desc 同源/簇跳过三绿，2026-10-25）
4. 发布配置 diff 维度补 StandardKeys 对比（低优先）

**已销账（2026-09-14）**：~~PT31 批量重获治愈 223 簇存量~~（DB 实证 mi=0/ss=0 均零——09-08 批量重获 11232 行已治愈）；~~幸运审核实战观察~~（advisory 引擎稳定运行，§59.166）；~~修道院新种站方审核观察~~（tid=1850-56 观察期毕）；~~team 域词条建设~~（R3-5/R3-6 遗留——用户销账放弃，SHB931 形态分类已归纳备查）

**活跃观察项（未闭环）**：
- **我堡/五二/财神目标站适配结果人工审核（§59.433 挂账——用户指令）**：三站均降级验证发版（五二/财神 cookie 失效+我堡资质门 5/10——form_config 值集未经站方当前实态实时确认）——人工审核发现漂移/拒收回传 upload HTML 走 formdiff 定位链闭环；各自前置：五二凭证恢复实时比对+首颗发布+候选权限/财神适配终态实发核验+上传者资格/我堡候选通道或权限达标补验
- §59.208 降级链不对称观察（fetch 无 yearless/rawname｜recovery 无 area=1）——按需生长纪律：零案例不预实施；第 3 次跨链不对称实证触发轮次库抽取重构
- ~~build-backend go test 偶发"首跑 FAIL 重跑过"~~（09-20 销账——时序 flake 族三案全闭：recordMap CreatedAt 零值×2+EventBus 异步断言×1，单核轰击验证零复现）

**已销观察项**：LuckAudit 过审率（09-02 收官——除 Suspiria MI 误判外全部正常过审，幸运专线 §59.155-163 实战验证完成）；阮玲玉两态放行（09-02 双路闭环 ✓）；243 错加无关种（用户已删）；keepfrds 列表页容量假设（朋友站日发种个位数，消解）

**行为铁律（全文保留——约束每次开发）**：
- **全量覆盖端点禁止稀疏 payload 实测**（2026-10-06 §59.318 附五教训）：导入（sites/import）语义=整行替换，`Select(siteImportFields)` 强制写 35 列含零值——缺省字段=清零非部分更新。实测必须用完整导出文件或测试专建站点，禁止手造稀疏 JSON 打真实 DB（29 两站 35 列清零事故，243 真值回填恢复）。
- **生产站实测禁止直接上传**（2026-09-26 用户定案）：验证发布链路必须走 DryRun/正常发布流程——手工 takeupload 会创建真实种子且可能无权删除（修道院 id=3620 复现事故）。
- **发布链资产消费白名单**：发布=纯消费种子配置落库资产。网络动作仅允许 ①目标站交互（pre-audit/dedup/上传/下载新种）②下载器 RPC ③**发布投递所需图片下载**（§59.458/§59.459 附二：封面 PosterURL+Tab3 截图 URL——现场下载→内存 base64→投递完即弃零持久化；数量门=站方 images 上限合计 ≤6；逐张失败降级跳过不阻塞；不开放其它图床下载）——其余外部 API 一律禁止（发布用 assembleDescription 纯本地组装，禁止复用采集组件）
- **发布四终态**：pushed/pushed_existing（302 权威 ID 自动推种）/existing（信文案不信页面 ID，不推走辅种）/failed——已存在=终态非失败；重试=人工幂等再执行，无自动重试引擎
- **上传判定二元化**：仅接受成功页种子（302 最终 URL+uploaded=1）；body 通用提取禁止（推荐位误推）；"其它页面的种子是辅种业务的工作"
- **幸运判据八条**（§59.151 终版：MI 唯一真相/HDR Profile 语义/语言四通道/粤语复合/媒介 title 词优先/音频组合键/英语条件映射/映射完备）——全文见 docs/31 §59.151 终局节
- **站点下线=删除记录+移除支持（全站通用铁律）**：migration DELETE sites 记录（全系统不可见）+ supported_sites.json 移除条目（系统不再认识——复活时按新站重新走接入流程，而非解禁旧配置）（2026-09-04 用户定案）
- **新站骨架永不默认启用（全站通用铁律）**：migration/seed 默认增加的支持站点一律 `enabled=false`（落"未启用"列表）——只有用户自行添加认证凭证并启用的站点才进入"我的站点"（2026-09-04 用户定案；migration 33 修道院 Enabled=true 反例教训）
- **系统级禁发剧集分集（§59.424 用户定案·行为铁律层）**：所有站点仅允许发布合集/完结/全集形态剧集——范围 {tv_series, animation, tv_shows}（动漫电影形态无 S/E 信号天然豁免）；放行信号任一：源站采集标签 complete ∨ 主标中英词形（\bcomplete\b/完结/全集/全季）∨ 副标中英词形直查 ∨ 全集一集（E01∧eps=1）；挂归一后 PushOnly 分支之后（补推豁免——形态恒定 vs flags 时变挂载策略不同）；纯代码规则零配置
- **转存发布禁勾"首发"标签（全站通用铁律）**：首发=PT 圈首次发布，站管/压制组官方专属，转载人员不允许使用（2026-09-02 用户权威定义）
- **站点代理解析公共单点**：`site.ResolveSiteProxy(db,ctx,site)` + `httpclient.NewSiteHTTPClient` 组合，禁止裸 client（§59.156）
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
- **设计决策**：所有设计决策记录到 `docs/31-模块设计决策记录.md`（grep "§59.X" 定位）
- **敏感信息**：禁止保存 cookie/passkey/apikey/token
- **适配器文档**：`docs/32-站点适配器设计/<站点>.md`；站点数据原样写入不杜撰
- **删除代码后**：跑 `go vet ./internal/... ./cmd/pt-forward/`（不用 `./...`——cmd/verify-pieces-hash 有已知冲突）
- **Git 提交**：禁止提交 `data/`、`PT0/`～`PT8/`、`*.torrent`、`logs/`、`*.db*`（详见 .gitignore+pre-commit hook 双保险）
- **版本号**：编译必须 `-X main.version=$(git describe --tags --always --dirty)`
- **环境隔离铁律**：开发只在 29；生产（249/242/22 等）只读诊断+scp 二进制，禁止直接改生产；生产更新由用户执行
- **部署铁律**：先提交后部署（commit+push 必须在编译部署前）；前端涉及必走三步（vite build → cp dist → go build）；完整流程见 skill `ptf-deploy`
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

**实施**：临时程序放 `cmd/tmp-<name>/`（编译验证后即删，不提交）；需长期保留的探测工具放 `cmd/`（如 searchprobe 先例）；生产 DB 数据（cookie 等）只读取出后本地使用。


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
