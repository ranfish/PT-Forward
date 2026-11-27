# 16 本地日界与 Truncate 对齐语义

- **状态**：精读完成，整改建议待实施
- **日期**：2026-11-27
- **基线**：`17230e8a`（15 三刀 + 补刀落地后，`origin/main` 最新）
- **审计范围**：日期分桶口径——「今日」统计窗口与趋势图日期标签的对齐语义，以及 `time.Time.Truncate(24h)` 在 `Location=Asia/Shanghai` 下的边界行为。
- **审计方法**：全库盘点后据实立项（非预设），避开 01-15 覆盖维度；批量定位 `Truncate(24h)` 使用点 → 逐处精读 → 用真实 Go 1.25 的 `time.Truncate`/`time.Date` 本地离线复现；核验仓库内部署时区证据链；对照同库正确口径正例与已核验健康的相邻维度划清正交边界。
- **证据文件**：`evidence/tool-versions.txt`（工具版本+基线+本机时区）、`evidence/local-day-boundary-repro.txt`（部署时区证据摘录 + 离线复现真实输出 + 使用点清单）。

> 本文为代码质量定点精读结论，不是安全评估结论。行号为基线快照，实施前请重新定位。所有证据均来自本地离线复现，未请求真实站点、未接触用户数据。

## 1. 结论摘要

- 本篇是全库盘点后新立的独立方向。上一轮盘点曾登记此候选并标注「需上游时区形态取证」；本轮在仓库内取得四处独立部署时区证据（`docker-compose.yml:23` `TZ=Asia/Shanghai`、`Dockerfile:39` tzdata、`notification/service.go:26-27` 注释「本地 CST」、本机 `date` 输出 CST），取证缺口消除，成为一个三者兼备（未覆盖 + 可离线复现 + 确凿功能失真）的方向。
- **确认两处确凿功能性缺陷（V-01/V-02，中危）**：同源于 `Truncate(24h)` 的绝对时间对齐，但表现为两条独立坏路径。
  - **V-01（日界偏移）**：`Truncate(24h)` 按自零时刻起的绝对持续时间对齐、忽略 `Location`，UTC+8 下对齐点落在本地 `08:00`（前一日）。8 处生产点用 `time.Now().Truncate(24h)` 取"今日"，使今日窗口多算前一日 `08:00–24:00` 共 16 小时记录；本地 `00:00–08:00` 的操作被排除出"今日"。
  - **V-02（趋势图标签错配）**：`seeding_handler_stats.go:215-230` 把 `Truncate(24h)` 与 `AddDate(0,0,1)` 混用，并用 `dayStart.Format("2006-01-02")` 作标签——本地凌晨记录标签显示前一天，相邻日窗口边界错位，图表数据与日期标签双重错配。
- **次要项（F-01，低危）**：`engine_test.go:909` 用同型 `Truncate(24h)` 生成期望值，测试与生产同错、失去对照价值；另记录残余不确定性——非 UTC+8 部署形态（如裸跑且系统时区 UTC、容器未传 `TZ`）下缺陷不显现，但仓库已明确声明主流形态为 UTC+8。
- **正面结论**：盘点独立复核确认小时对齐 `Truncate(time.Hour)`、相对天数 cutoff `AddDate(0,0,-N)`、出站 `UTC().Format`、本地时间口径正例（同库 ≥6 处 `time.Date(..., now.Location())` / `time.ParseInLocation(..., time.Local)`）均健康，本篇不重复主张正例本身。
- **明确不做**：请求真实站点、碰靶机、改业务语义；本篇只做精读与整改建议。

## 2. 重点发现

### V-01 · `Truncate(24h)` 取「今日」导致 UTC+8 下日界偏移 8 小时（中危 · 日界对齐语义）

- **级别**：中危（阅读档「中危」；技术档参照 `AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:L/A:N`）。统计与展示层失真，用户可见；无越权、无数据破坏。
- **证据**：

```go
// internal/api/dashboard_handler.go:100（另 :431/:565/:611 同型）
today := time.Now().Truncate(24 * time.Hour)
// ... Where("publish_status = ? AND updated_at >= ?", "done", today)

// internal/api/seeding_handler_stats.go:52/111/313、internal/seeding/engine_stats.go:636 同型

// 同库正确口径正例（dashboard_handler.go:282）
dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
```

  Go 的 `Time.Truncate(d)` 按自零时刻起的绝对持续时间向下对齐，不使用 `Location`。UTC+8 下 24 小时对齐点落在本地 `08:00`（前一日）。
- **影响**：8 处生产点覆盖用户高频页面——Dashboard 今日发布数/今日辅种数（`:100`）、今日删除数（`:431/:565`）、最近发布结果（`:611`）；Seeding overview 今日新增/删除（`:52`）、by-site 今日统计（`:111`）、今日上传流量查询（`:313`）；engine 每日站点流量归档（`:636`）。UTC+8 下本地 `00:00–08:00` 的操作被排除出"今日"，同时前一日 `08:00–24:00` 的记录被计入——窗口错开 8 小时。`site_traffic_daily` 归档值亦偏移。
- **复现**（本地真实 Go 1.25，见证据文件）：

```
对齐点: 本地 11-27 08:00 epoch=1795737600 可被86400整除=true
本地 11-27 00:30 → Truncate(24h)=11-26 08:00 | 本地午夜口径=11-27 00:00
本地 11-27 07:30 → Truncate(24h)=11-26 08:00 | 本地午夜口径=11-27 00:00
本地 11-27 08:30 → Truncate(24h)=11-27 08:00 | 本地午夜口径=11-27 00:00
本地 11-27 09:30 → Truncate(24h)=11-27 08:00 | 本地午夜口径=11-27 00:00
```

- **形态取证（非缺口）**：`docker-compose.yml:23` 明确 `- TZ=Asia/Shanghai`；`Dockerfile:39` 安装 tzdata 使 `TZ` 生效 → `time.Local = Asia/Shanghai`；`internal/notification/service.go:26-27` 注释记录项目曾因「CI UTC 03:2x 命中窗口而本地 CST 不中」修过一次时区飘移；本机 `date` 输出 `CST 2026`。四处证据独立印证 UTC+8 部署形态，上一轮标注的「需上游形态取证」已销案。
- **整改**：将 8 处生产点收敛到统一的本地午夜口径单点（如 `util.LocalDayStart(t)`，复用 `dashboard_handler.go:282` 的 `time.Date(..., now.Location())` 写法），"今日"查询一律走该单点；新增日界锚（`TZ=Asia/Shanghai` 下 `00:30` 归当日、`07:30` 归当日、对齐点为本地 `00:00` 而非 `08:00`）。

### V-02 · 趋势图窗口与日期标签双重错配（中危 · 同族第二坏路径）

- **级别**：中危（同上）。用户可见的图表数据与标签错配，可能误导运营判断。
- **证据**（`internal/api/seeding_handler_stats.go:215-230`）：

```go
for i := days; i >= 0; i-- {
    day := now.AddDate(0, 0, -i)
    dayStart := day.Truncate(24 * time.Hour)      // 本地 08:00（前一日）
    dayEnd := dayStart.AddDate(0, 0, 1)           // 下一本地 08:00
    // ... Where("created_at >= ? AND created_at < ?", site, dayStart, dayEnd)
    points = append(points, map[string]interface{}{
        "date":  dayStart.Format("2006-01-02"),   // 标签跟着错到前一日
        "count": count,
    })
}
```

  `Truncate(24h)` 与 `AddDate(0,0,1)` 混用：前者按绝对零点对齐到 08:00，后者按日历日加一天。二者组合使窗口边界为 `[本地 08:00, 次日 08:00)`，而标签用 `dayStart.Format` 显示为前一日日期。
- **影响**：趋势图每个点的 `date` 标签比其实际统计日早一天（UTC+8 下），且相邻点的窗口边界从本地午夜错到 08:00——前端 `web/src/views/seeding/SeedingStats.vue:34/64-65` 直接展示该标签与计数，形成"标签与数据双错"。
- **复现**（证据文件 B 段末行）：本地 `11-27 07:30` 的记录落在窗口 `[11-26 08:00, 11-27 08:00)`，`date` 标签为 `2026-11-26`——与正确口径 `2026-11-27 00:00` 起算的「今日」完全错位。
- **整改**：`dayStart` 改用本地午夜单点（同 V-01），`dayEnd := dayStart.AddDate(0,0,1)` 保持不变（此时两者口径一致）；标签改用 `day.Format("2006-01-02")`（取目标日本身）而非 `dayStart.Format`。新增趋势图锚：断言连续两日窗口首尾相接于本地午夜、标签与窗口所属日一致。

### F-01 · 测试同型失准与残余不确定性（低危）

- **证据**：`internal/seeding/engine_test.go:909` 用 `now.Truncate(24*time.Hour)` 生成期望值，与生产同型——若生产改为本地午夜口径而测试仍用 `Truncate`，测试会失去对照价值；反之若两者同错，测试无法发现问题。
- **残余不确定性**：非 UTC+8 部署形态（systemd 裸跑且系统时区为 UTC，或容器未传 `TZ`）下缺陷不显现。这是时区类问题的固有属性；仓库已明确声明主流部署形态为 UTC+8，故本篇按「已取证形态」处理，不挂账为缺口。若后续新增非 UTC+8 部署形态，需重新评估影响面。
- **整改**：测试期望值改用与生产一致的本地午夜单点；锚在 `TZ=Asia/Shanghai` 下验证行为，并可在 `TZ=UTC` 下做一次对照确认锚不误报。

### 正面结论：经核验健康的维度

- 小时对齐：`stats/traffic_aggregator.go:23` 的 `Truncate(time.Hour)`、`seeding_handler_stats.go:370-391` 小时分桶——时区偏移为整小时，整点边界不受影响。
- 相对天数 cutoff：`fingerprint/repository.go:254`、`rss/repository.go:111`、`seeding/engine_evaluate.go:15`、`engine_stats.go:801/814`、`ptgen/provider.go:583/618`、`notification/repository.go:74`、`dashboard_handler.go:400/689` 的 `AddDate(0,0,-N)` 族——相对窗口无日界问题。
- 出站时间戳：`api/ws_handler.go:31/272`、`publish/executor.go:561` 用 `UTC().Format`，明确 UTC 语义。
- 本地时间口径正例：`dashboard_handler.go:282/341-342`、`publish_torrents_handler_seeddata.go:172` 的 `time.Date(..., now.Location())`，`publish_torrents_handler_seed_detail.go:75`、`adapter/adapter_nexusphp_detail.go:366`、`adapter/adapter_hdarea.go:245/372` 的 `time.ParseInLocation(..., time.Local)`，`cron.WithLocation(time.Local)`，`engine_preload.go:41` 本地日限额——证明作者已掌握正确写法，本篇 9 处属漏改。
- 历史篇沿用：除零点守卫全健康（`preview_builder.go:162-165`、`seeding_handler_stats.go:79-84`、`traffic_aggregator.go:71-74`）、atomic 类型化 API、`sync.Once`、类型断言守卫、`%w` 错误链、SQL 非 LIKE 白名单、分页边界、`strings.Index -1` 守卫、TTL 边界、retry_transport、maskToken 长度守卫、迁移版本纪律、OTA 单飞门、规则引擎白名单静态集合相等锚。

## 3. 整改建议

- **刀1（V-01）**：新增本地午夜口径单点（如 `util.LocalDayStart(t)`），把 8 处生产 `Truncate(24h)` 全部收编；保留相对天数 `AddDate` 的正确用法。新增日界锚（本地 `00:30`/`07:30` 归当日、对齐点本地 `00:00`）。
- **刀2（V-02）**：趋势图 `dayStart` 走同一单点、`dayEnd` 保持 `AddDate(0,0,1)`、标签改 `day.Format`；新增趋势图窗口/标签一致性锚。
- **刀3（F-01 + 三层锚）**：测试期望值改用同单点；静态锚登记「时间分桶一律走本地午夜单点、`Truncate(24h)` 禁用」纪律（登记须附调用方与形态证据核验，沿用 13/14/15 补刀教训）。三阴性对照：改回 `Truncate(24h)` → 日界锚 FAIL、标签改回 `dayStart.Format` → 标签锚 FAIL、删单点调用 → 静态锚 FAIL。

## 4. 验证建议

- 三刀后跑全量 `go test ./...` 与 `go vet ./internal/...`（frontend/cmd 三处基线既有失败与本篇无关）。
- 三阴性对照须全红（08 篇方法论：声明逐条可触发验证，杜绝虚拟安全感）。
- 锚在 `TZ=Asia/Shanghai` 下验证主行为，可在 `TZ=UTC` 下做一次对照确认不误报；形态证据引用仓库内 `docker-compose.yml:23` 与 `Dockerfile:39`，不请求真实站点。

## 5. 本次未覆盖

- 未请求真实站点 API；部署时区形态依据仓库内声明与本机时区，未做容器运行时取证（标注「已取证形态」，非缺口）。
- 未审计所有相对天数 cutoff（盘点已核验健康，见 §2 正面结论）。
- 盘点候选「动态正则编译失败静默 `return false`」与 15 篇规则引擎边界重叠、无确凿损坏证据，本篇不立项。
- 未审计前端展示层的日期格式化（本篇聚焦后端分桶口径）。

## 6. 后续方向

- 按 §3 三刀整改后复审；复审须做三阴性对照与双时区对照。
- 下一方向待本篇收官后再据实盘点，不预设。
- 与本系列关系：本篇由盘点据实立项（非预设）；与 07（魔法值）、15（规则字段语义）相邻但正交——本篇管「时间分桶口径」。
