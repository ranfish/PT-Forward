# 17 ORM Upsert 赋值语义契约

- **状态**：已完成（三刀 v0.0.1668 落地）
- **日期**：2026-11-27
- **基线**：`8ec12cab`（16 篇三刀收官后，远端另新增瓷器规则页回摸与 §59.580 禁转黑名单三刀）
- **审计范围**：GORM `Assign(attrs).FirstOrCreate(dest)` 的别名语义——当 attrs 与 dest 指向同一对象时，「已存在行刷新」是否真正生效，以及覆盖缓存写入链上的两处遗留点。
- **审计方法**：全库盘点后据实立项（非预设），避开 01-16 覆盖维度；批量定位 `Assign` + `FirstOrCreate` 组合 → 逐处精读 → 读 GORM v1.31.1 源码确认机制 → 用内存 SQLite 与真实 `model.SiteCoverageCache` 本地离线复现；核验项目自身 §56.33 修复提交作为权威定性；追业务写入路径。
- **证据文件**：`evidence/tool-versions.txt`（工具版本+基线+采集时间）、`evidence/gorm-upsert-alias-repro.txt`（项目自证、GORM 源码摘录、两处确凿点、离线复现真实输出、业务路径、调用面、正例对照）。

> 本文为代码质量定点精读结论，不是安全评估结论。行号为基线快照，实施前请重新定位。所有证据均来自本地离线复现与仓库内提交记录，未请求真实站点、未接触用户数据。

## 1. 结论摘要

- 本篇是全库盘点后新立的独立方向。找到了一个三者兼备（未被 01-16 覆盖 + 可纯本地离线复现 + 确凿功能性缺陷）的缺陷族：**GORM `Assign(x).FirstOrCreate(x)` 同指针别名导致已存在行刷新静默失效**。
- **确认两处确凿缺陷（V-01/V-02，中危）**：`coverage.Service.upsertCoverage` 与 `metadata.fetcher.persistCoverageHits` 均以同一指针同时作 attrs 与 dest；实测已存在行写入后仍保持旧值（`confirmed_not`/`tracker`/空 `TorrentID`/时间 1000），而值副本正例全部刷新为新值。
- **项目自证最强**：提交 `13b17321a`（§56.33，v0.0.282）已对同型缺陷做过权威定性与修复，并留下「attrs 用值副本、dest 用本体」的注释；本篇两处正是该修复未推广到的遗留点。同库另有三处正例（值+指针不同对象、独立 map、主键 `Save`），构成「作者已认知、后续新增漏改」的完整叙事。
- **业务路径取证（F-01）**：`publish_torrents_handler_torrents.go:388-415` 明示「① 辅种引擎查询先写（带 `TorrentID`）→ ② tracker 映射后写覆盖同名」，对同一 `(InfoHash, SiteName)` 二次写入且字段变化——设计意图是后写生效，同指针缺陷使后写失效。诚实边界：这是**确凿语义缺陷 + 可复现的行为失真**；至于实际业务损害程度，取决于上游是否真出现「同名二次写入且字段变化」，不夸大为已发生的生产事故。
- **正面结论**：盘点独立复核确认 §59.580 黑名单主链、切片复用、`RowsAffected`/唯一索引幂等、sort comparator、数字/时间边界、URL 拼接均健康，本篇不重复主张。候选「黑名单双键查询与前缀字节切片严谨性」三阴性当前均不成立，标注「当前无功能损坏」不立项。
- **明确不做**：请求真实站点、碰靶机、改业务语义；本篇只做精读与整改建议。

## 2. 重点发现

### V-01 · `coverage.Service.upsertCoverage` 同指针致已存在行刷新静默失效（中危 · ORM 别名语义）

- **级别**：中危（阅读档「中危」；技术档参照 `AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:L/A:N`）。缓存状态与源站点信息失真，用户可见的辅种判定可能受影响；无越权、无数据破坏。
- **证据**（`internal/coverage/service.go:359-364`）：

```go
func (s *Service) upsertCoverage(ctx context.Context, record *model.SiteCoverageCache) error {
	return s.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", record.InfoHash, record.SiteName).
		Assign(record).          // attrs = record 指针
		FirstOrCreate(record).Error  // dest = 同一 record 指针
}
```

  GORM v1.31.1 `finisher_api.go:354` 先执行 `result := queryTx.Find(dest, conds...)`，把数据库旧值写回 `dest`；`RowsAffected > 0` 分支（`:375`）用 `BuildCondition(db.Statement.assigns[0], …)` 构造 `Updates`。因 attrs 与 dest 为同一指针，被 `Find` 覆写后的 attrs 已是数据库旧值——「刷新」等价于用旧值更新。
- **影响**：`upsertCoverage` 是覆盖缓存写入的唯一收口，调用面 10+ 处：`publish_torrents_handler_torrents.go:275/330/392/413/499/538/557`、`seed_fetch.go:50`、`orphan/recovery.go:1384`、`coverage/service.go:197`（批量写）及 `UpsertCoverage` 公开入口（`:366`）。受影响字段为 `Status`/`Source`/`TorrentID`/`DetailURL`/`QueriedAt`/`ExpiresAt`/`TargetInfoHash`——辅种覆盖判定、源站探测、簇跳过均读此表。唯一索引 `idx_coverage_hash_site` 保证同一 `(InfoHash, SiteName)` 只有一行，二次写入必然落入「已存在」分支，因此缺陷在二次写入路径上稳定触发。
- **复现**（本地真实 Go 1.25，内存 SQLite + 真实 `model.SiteCoverageCache`，见证据文件 D 段）：

```
A 同指针 : err=<nil> rows=1 | Status="confirmed_not" Source="tracker" TorrentID="" QueriedAt=1000
  期望    : Status="confirmed_has" Source="iyuu" TorrentID="t-new" QueriedAt=2000
B 值副本 : err=<nil> rows=1 | Status="confirmed_has" Source="iyuu" TorrentID="t-new" QueriedAt=2000
结论: 同指针刷新失效已复现——已存在行保持旧值；值副本正常刷新
```

- **整改**：仿照 §56.33 修复形态，用 `attrs := *record`（值副本）作 `Assign(&attrs)`，dest 保持 `record` 本体；新增刷新锚——先插旧行，再以变化字段调用 `upsertCoverage`，断言新值落库（含 `Status`/`Source`/`TorrentID`/`QueriedAt`）；阴性对照：退回同指针 → 锚 FAIL。

### V-02 · `metadata.fetcher.persistCoverageHits` 同指针且丢弃错误（中危 · 同族第二遗留点）

- **级别**：中危（同上）。
- **证据**（`internal/metadata/fetcher.go:738-747`）：

```go
row := model.SiteCoverageCache{ /* InfoHash, SiteName, Status=confirmed_has,
    Source=iyuu, TorrentID=h.TorrentID, Confidence=1.0, QueriedAt=now, ExpiresAt=ttl */ }
f.db.WithContext(ctx).
	Where("info_hash = ? AND site_name = ?", infoHash, h.SiteName).
	Assign(&row).FirstOrCreate(&row)   // 同一 &row，且 .Error 未接收
```

  与 V-01 同型；更严重的是 `.Error` 完全丢弃——即便数据库报错也无任何日志或返回。
- **影响**：经 `PersistCoverageHit`（`:928`）被 `manual_forward_handler.go:57`（接口声明）、`publish_torrents_handler_music.go:349` 调用，写入「handler 簇兄弟标题桥成功路径」的皇后 `tid` 缓存。该路径写入的 `TorrentID` 若因同指针缺陷无法刷新，未来重获直达的缓存化目的落空。
- **复现**：与 V-01 同机制、同模型；可用同一刷新锚函数覆盖两处（分别注入旧行并断言新值）。
- **整改**：同样改为 `attrs := row` 值副本 + `Assign(&attrs).FirstOrCreate(&row)`，并接收 `.Error` 记日志（与同文件 `store` 的错误处理对齐）；补第二处刷新锚。

### F-01 · 业务二次写入路径与影响边界（低危 · 诚实标注）

- **证据**（`internal/api/publish_torrents_handler_torrents.go:388-415`，见证据文件 E 段）：

```go
// ① 辅种引擎查询 → 🟡（先写·带 TorrentID）
UpsertCoverage(ctx, &model.SiteCoverageCache{
    InfoHash: req.InfoHash, SiteName: hit.SiteName,
    Status: confirmed_has, Source: source /* IYUU 或 pieces_hash */,
    TorrentID: hit.TorrentID, QueriedAt: now, ExpiresAt: ttl })
// ② tracker 映射 → 🟢 做种中（后写，覆盖同名 🟡）
UpsertCoverage(ctx, &model.SiteCoverageCache{
    InfoHash: req.InfoHash, SiteName: sn,
    Status: confirmed_has, Source: model.CoverageSourceTracker,
    Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl })
```

  两处对同一 `(InfoHash, SiteName)` 二次写入，注释明确「后写覆盖同名」，且 `Source` 与 `TorrentID` 字段发生变化。另有 `coverage/service.go:197` 批量链与 `metadata.fetcher.persistCoverageHits` 的交叉写入可能命中同一行。
- **诚实边界**：缺陷语义与行为失真已由 V-01/V-02 离线实证；「已观测到的业务事故」需要验证上游是否真实出现同名二次写入及字段变化——本篇**不宣称已发生生产事故**，只标注路径存在。
- **整改**：修复 V-01/V-02 后补一条端到端锚，模拟「先写 IYUU 带 TorrentID → 后写 tracker」并断言后写字段落库。

### 正面结论：经核验健康的维度与正例

- **项目自证**：`git show 13b17321a`（§56.33，v0.0.282）提交说明完整记录同型根因、现象、诊断与修复；`fetcher.go:884-894` 现态 `attrs := *meta` + `Assign(&attrs).FirstOrCreate(meta)` 是权威正例。
- **同库正例**：`stats/traffic_aggregator.go:78-95`（`Assign(record)` 值与 `FirstOrCreate(&record)` 指针——不同对象）、`orphan/recovery.go:1100-1109`（独立 map 作 attrs）、`coverage/service.go:593-597`（主键 `Save(&state)`）。
- **盘点健康维度**：§59.580 黑名单主链（`BannedGroupsConfig.Match` 精确 EqualFold + 前缀、批次早退/一站多种跳过/`engine_match.go:204-218` 过滤三处消费点、三枚锚测试）；切片复用 `[:0]`（tag_inferer、cloud_fp、engine_preload、companion、seed_list 均为同 slice 过滤后回赋或同步消费，无别名污染）；`RowsAffected`/`INSERT OR IGNORE`/`FirstOrCreate` 幂等依赖 `idx_coverage_hash_site` 唯一索引成立；sort comparator 34 处抽样均为严格序；`ParseRelativeAge`、日期解析、`time.Parse` 失败保守放行无新确凿失真；URL 查询用 `url.Values`/`QueryEscape`。
- **历史篇沿用**：01-16 已覆盖主题与已核验健康维度不重复主张。

## 3. 整改建议

- **刀1（V-01）**：`internal/coverage/service.go:359-364` 改为值副本 attrs + dest 本体；新增刷新锚（旧行 → 新值写入 → 断言新值落库，覆盖 `Status`/`Source`/`TorrentID`/`QueriedAt`）。
- **刀2（V-02）**：`internal/metadata/fetcher.go:738-747` 同修，并接收 `.Error` 记日志；补第二处刷新锚。
- **刀3（F-01 + 静态锚）**：补端到端二次写入锚（IYUU 先写 → tracker 后写 → 断言后写字段落库）；新增静态锚扫描全库 `Assign(x).FirstOrCreate(x)` 同指针形态（检测 `Assign(` 与 `FirstOrCreate(` 参数文本相同，注释行截断防误报，测试文件豁免），防止后续新增代码重犯。三阴性对照：V-01 退回同指针 → 刷新锚 FAIL、V-02 退回同指针 → 第二锚 FAIL、注入新同指针形态 → 静态锚点名 FAIL。

## 4. 验证建议

- 三刀后跑全量 `go test ./...`、`go vet ./internal/...`、`gofmt -l`（`cmd/pt-forward`、`frontend`、`cmd/verify-pieces-hash` 三处基线既有失败与本篇无关）。
- 三阴性对照须全红（08 篇方法论：声明逐条可触发验证，杜绝虚拟安全感）。
- 静态锚须做阴性对照并点名文件行号（沿用 13/14/15/16 补刀教训：防回归锚本身也要被锚）。
- 复现与锚均用内存 SQLite，不请求真实站点、不接触用户数据。

## 5. 本次未覆盖

- 未请求真实站点 API；业务损害程度按「路径存在但上游形态待取证」标注，不夸大。
- 未审计所有 ORM 写路径（盘点已核验 `RowsAffected`/唯一索引幂等、主键 `Save` 等健康，见 §2 正面结论）。
- 盘点候选「黑名单双键查询与前缀字节切片严谨性」三阴性当前均不成立（105 站 `name_cn` 无重复、前缀为 ASCII、忽略 `Scan` 错误属保守放行），本篇不立项。
- 未审计前端展示层（本篇聚焦后端 ORM 写入语义）。

## 6. 后续方向

- 按 §3 三刀整改后复审；复审须做三阴性对照与刷新锚实跑。
- 下一方向待本篇收官后再据实盘点，不预设。
- 与本系列关系：本篇由盘点据实立项（非预设）；与 05（数据库访问纪律）相邻但正交——本篇管「ORM Upsert 的 attrs/dest 别名语义」。
