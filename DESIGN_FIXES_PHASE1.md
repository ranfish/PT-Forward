# Phase 1 代码改造计划（设计审计结果落地）

**审计时间**：2026-04-30  
**总问题数**：22（P0: 7, P1: 9, P2: 6）  
**预计改造位置**：50+ locations  
**预计工作量**：2-4h（含验证）  

---

## 一、快速状态对照表

| 问题 | 类别 | 严重度 | Canonical | 改造点 | 状态 |
|------|------|--------|-----------|--------|------|
| **DB-1** | SubscriptionID | P0 | ✅ 已定义 §33.1.14a | 6-7 | 就绪改造 |
| **DB-2** | ClientID | P0 | ✅ 已定义 §33.1.15a | 4-5 | 就绪改造 |
| **DB-3** | SiteName | P0 | ✅ 已定义 §33.1.15c | 8-10 | 就绪改造 |
| **SA-1** | TorrentEvent.HRSeedTimeH | P0 | ✅ 已补齐 §33.1.1 | 0 | **✅ 已完成** |
| **PP-1** | Step 18 Dupe InfoHash | P0 | ⏳ 需完善 | 1 | 待设计 |
| **CR-1** | 磁盘预算检查 | P0 | ✅ 已有方案 §33.8.17 | 1 | 就绪改造 |
| **ER-1** | 崩溃恢复机制 | P0 | ✅ 已有方案 §33.8.16 | 2 | 就绪改造 |
| **P1-1** | PublishCandidate 场景不一致 | P1 | ✅ 部分 | 3-4 | 待完善 |
| **P1-2/3/4-9** | 其他 P1 问题 | P1 | ⚠️ 部分 | 15+ | 分阶段 |

---

## 二、P0-1: SubscriptionID 规范化（6-7 处改造）

**Canonical 源**：§33.1.14a SubscriptionIDConverter

### 改造清单

#### [改造点 1.1] 位置：L3954（postPush Scene B）
**当前代码**（假设）：
```go
publishCandidate.SubscriptionID = fmt.Sprintf("%d", sub.ID)
```

**改为**：
```go
publishCandidate.SubscriptionID = SubscriptionIDConverter{}.FromRSSSubscriptionID(sub.ID)
```

**验证**：所有 PublishCandidate 创建必须用 Converter

#### [改造点 1.2] 位置：L4000（postPush Scene C）
**当前**：`fmt.Sprintf("%d", sub.ID)`  
**改为**：`SubscriptionIDConverter{}.FromRSSSubscriptionID(sub.ID)`

#### [改造点 1.3] 位置：L4198（postPush）
**同上**

#### [改造点 1.4] 位置：L35040（postPush）
**同上**

#### [改造点 1.5] 位置：L35074（postPush）
**同上**

#### [改造点 1.6] 位置：L35495、L35547（postPush）
**同上**

#### [改造点 1.7] 位置：L10150（submit API 手动发布）
**当前代码**（假设）：
```go
publishCandidate.SubscriptionID = "manual:" + req.UUID
```

**改为**：
```go
publishCandidate.SubscriptionID = SubscriptionIDConverter{}.FromManualUUID(req.UUID)
```

### 验证清单
- [ ] `grep "fmt.Sprintf.*%d.*ID"` 在这 7 个位置已替换
- [ ] 不存在 `"manual:" + uuid` 的字符串拼接
- [ ] 编译通过：类型检查

---

## 三、P0-2: ClientID 规范化（4-5 处改造）

**Canonical 源**：§33.1.15a ClientIDConverter

### 问题背景

系统中下载器有两种标识：
- **string 标识**（业务用）：`client.GetName()`，99% 代码用这个
- **uint 标识**（数据库用）：`client.GetID()`，仅 FK 字段用这个

异常表：`ClientPathMapping` 和 `ClientPublishTarget` 中使用 uint FK，但赋值时无转换。

### 改造清单

#### [改造点 2.1] ClientPathMapping.SourceClientID 赋值
**位置**：§15.5 Step 6 辅种注入（查询路径映射时）

**当前代码**（假设）：
```go
mapping := &ClientPathMapping{
    SourceClientID: sourceClient.GetID(),  // ❌ 直接用 uint
}
```

**改为**：
```go
mapping := &ClientPathMapping{
    SourceClientID: ClientIDConverter{}.ToUintID(sourceClient),
}
```

#### [改造点 2.2] ClientPathMapping.ReseedClientID 赋值
**位置**：同上

**改为**：
```go
mapping.ReseedClientID = ClientIDConverter{}.ToUintID(reseedClient)
```

#### [改造点 2.3] ClientPublishTarget.ClientID 赋值
**位置**：§21 PublishPipeline Step 1（查询发布目标配置时）

**改为**：
```go
target := &ClientPublishTarget{
    ClientID: ClientIDConverter{}.ToUintID(targetClient),
}
```

#### [改造点 2.4] 反向查询时的转换
**位置**：从数据库加载 ClientPathMapping/ClientPublishTarget 后

**当前代码**（假设）：
```go
mapping := &ClientPathMapping{}
db.First(mapping, ...)
sourceClient := dm.Get(mapping.SourceClientID)  // ❌ uint 入参，但 Get() 期望 string
```

**改为**：
```go
mapping := &ClientPathMapping{}
db.First(mapping, ...)
sourceClientID := ClientIDConverter{}.FromClientID(dm.GetByID(mapping.SourceClientID))
sourceClient := dm.Get(sourceClientID)
```

或者优化方案（改为 JOIN）：
```go
db.Joins("JOIN clients ON clients.id = client_path_mappings.source_client_id").
   Where("client_path_mappings.id = ?", mappingID).
   Select("clients.name AS source_client_id").
   First(mapping, ...)
// 然后 sourceClient := dm.Get(mapping.SourceClientID) ✅
```

### 验证清单
- [ ] ClientPathMapping / ClientPublishTarget 所有赋值都用 ToUintID()
- [ ] 所有反向查询都用 Converter 转换
- [ ] 没有 `mapping.SourceClientID = client.GetID()` 的直接赋值
- [ ] 编译通过

---

## 四、P0-3: SiteName 规范化（8-10 处改造）

**Canonical 源**：§33.1.15c SiteNameConverter

### 问题背景

站点有三种标识，使用规范不清：
- **string domain**（业务）："hdsky.me"，业务代码应统一使用
- **string name**（显示）："HDSky"，仅前端展示
- **uint id**（数据库 PK）：PublishTask.SourceSiteID 使用，业务代码不应用

### 改造清单

#### [改造点 3.1-3.3] 业务路由键转换
**位置**：任何涉及站点查询的模块（发布、辅种、RSS）

**原则**：业务代码永远用 `site.Domain`，从不用 `site.ID`

**当前代码**（假设）：
```go
if site.ID == targetSiteID { ... }  // ❌
events := getEventsBySiteID(site.ID)  // ❌
```

**改为**：
```go
if site.Domain == targetSiteDomain { ... }  // ✅
events := getEventsBySiteDomain(site.Domain)  // ✅
```

#### [改造点 3.4-3.5] API 返回修复
**位置**：所有 API response 结构

**当前**：
```json
{
  "id": 42,
  "name": "HDSky",
  "domain": "hdsky.me"
}
```

**改为**：
```json
{
  "name": "HDSky",           // 显示名
  "domain": "hdsky.me",      // 业务标识
  // ✗ 不返回 "id": 42
}
```

#### [改造点 3.6-3.8] 日志和监控
**位置**：所有日志记录、监控指标中

**当前**：
```go
log.Printf("Publishing to site %d", site.ID)
```

**改为**：
```go
log.Printf("Publishing to site %s", site.Domain)
```

#### [改造点 3.9-3.10] 数据库 FK 使用（特例）
**位置**：PublishTask.SourceSiteID 赋值

**原则**：FK 仅用于 JOIN，获取后立即转为 domain

**当前**：
```go
publishTask.SourceSiteID = site.ID  // 赋值用 uint
// 后续使用 publishTask.SourceSiteID 做业务逻辑 ❌
```

**改为**：
```go
publishTask.SourceSiteID = SiteNameConverter{}.ToUintID(site)  // 赋值用 Converter

// 使用时先 JOIN，获取 domain
var task PublishTask
db.Joins("LEFT JOIN sites ON sites.id = publish_tasks.source_site_id").
   Select("publish_tasks.*, sites.domain AS source_site_domain").
   First(&task, ...)
// 然后业务逻辑用 task.SourceSiteDomain（string）✅
```

### 验证清单
- [ ] 业务代码不存在 `site.ID` 的使用（仅 FK 赋值除外）
- [ ] API 返回不暴露 `id` 字段
- [ ] 日志都用 `site.Domain` 或 `SiteNameConverter{}.FromSite(site)`
- [ ] 编译通过

---

## 五、P0-4: TorrentEvent.HRSeedTimeH 补齐

**状态**：✅ **已完成**（在 Sprint 91 已添加）

Canonical 已在 §33.1.1 补齐：
```go
HRSeedTimeH  int           `json:"hr_seed_time_h" gorm:"default:0"` // 决策 A-1
```

**需要的改造**（1-2 处）：
- [ ] SiteAdapter.DetectHR() 返回时填充 hrSeedTimeH
- [ ] RSS 解析后 SiteAdapter.EnrichEvent() 调用补充 HRSeedTimeH

---

## 六、P0-5: Step 18 Dupe 场景 InfoHash 缺失修复

**Canonical 源**：§33.8.19 Sprint 69 PP-1 修复方案

### 问题

Dupe 发布时，targetMember.InfoHash 为空，导致 PublishTracker 无法验证种子。

### 改造清单

#### [改造点 5.1] PublishPipeline Step 18
**位置**：§21.11 INJECTING_DUPE step 处理

**改造方案**：
```go
if publishResult.InfoHash == "" {
    // 方案 A（推荐）：查询目标站 Torrent API 获取 hash
    torrentInfo, err := siteAdapter.GetTorrentInfo(ctx, site, publishResult.TorrentID)
    if err == nil && torrentInfo.InfoHash != "" {
        publishResult.InfoHash = torrentInfo.InfoHash
    }
    
    // 方案 B（备选）：下载 .torrent 文件解析 hash
    if publishResult.InfoHash == "" {
        torrentData, err := siteAdapter.DownloadTorrent(ctx, site, publishResult.TorrentID)
        if err == nil {
            publishResult.InfoHash = parseTorrentHash(torrentData)
        }
    }
}

// 补齐状态
targetMember.InfoHash = publishResult.InfoHash
targetMember.Status = "seeding_confirmed"  // 而非空值
```

### 验证清单
- [ ] Step 18 后 targetMember.InfoHash 非空（或标记失败）
- [ ] PublishTracker 能成功查询种子
- [ ] 无超时错误

---

## 七、P0-6: 辅种注入磁盘预算检查

**Canonical 源**：§33.8.17 Sprint 67 CR-1 修复方案

### 改造清单

#### [改造点 6.1] §15.5 Step 6 注入前的检查
**位置**：ReseedEngine.InjectTorrents() 或等效方法

**改造**：
```go
for _, match := range matchesToInject {
    // 新增：磁盘检查
    effectiveFree := diskBudget.EffectiveFreeGB(match.ClientID)
    minFreeGB := diskBudget.MinFreeGB(match.ClientID)
    if effectiveFree < minFreeGB {
        match.Status = "skipped"
        match.FailReason = fmt.Sprintf(
            "disk_budget_exceeded: %.1fGB < %.1fGB",
            effectiveFree, minFreeGB,
        )
        continue  // 跳过注入，不记录为错误
    }
    
    // 原有逻辑
    result, err := client.AddFromFile(ctx, match.TorrentPath, opts)
    ...
}
```

### 验证清单
- [ ] 所有注入操作前都有磁盘检查
- [ ] 无磁盘的种子标记 "skipped" 而非 error
- [ ] 大规模辅种（1000+ 种子）不会磁盘超额

---

## 八、P0-7: 辅种引擎崩溃恢复机制

**Canonical 源**：§33.8.16 Sprint 66 ER-1 修复方案

### 改造清单

#### [改造点 7.1] ReseedEngine.Start() 前增加恢复
**位置**：ReseedEngine 初始化

**改造**：
```go
func (e *ReseedEngine) Start(ctx context.Context) error {
    // 第 1 步：恢复被卡住的任务
    if err := e.recoverStuckTasks(ctx); err != nil {
        log.Warnf("Failed to recover stuck tasks: %v", err)
    }
    
    // 第 2 步：清理孤立的中间态 matches
    if err := e.recoverOrphanMatches(ctx); err != nil {
        log.Warnf("Failed to recover orphan matches: %v", err)
    }
    
    // 第 3 步：启动主循环
    e.mainLoop(ctx)
}

// 恢复被卡住的任务（in_progress > 10min）
func (e *ReseedEngine) recoverStuckTasks(ctx context.Context) error {
    cutoff := time.Now().Add(-10 * time.Minute)
    result := e.db.Model(&ReseedTask{}).
        Where("status = ? AND updated_at < ?", "in_progress", cutoff).
        Updates(map[string]any{
            "status":      "failed",
            "fail_reason": "crash_recovery: stuck in progress",
        })
    return result.Error
}

// 清理孤立的 matches（pending 超过 1h 未被 task 认领）
func (e *ReseedEngine) recoverOrphanMatches(ctx context.Context) error {
    cutoff := time.Now().Add(-1 * time.Hour)
    result := e.db.Model(&ReseedMatch{}).
        Where("status = ? AND created_at < ?", "pending", cutoff).
        Where("task_id NOT IN (SELECT id FROM reseed_tasks WHERE status = ?)", "pending").
        Updates(map[string]any{
            "status":      "skipped",
            "fail_reason": "crash_recovery: orphan match",
        })
    return result.Error
}
```

#### [改造点 7.2] 定时崩溃检测（可选）
**用途**：如果恢复机制不够，可定时检查

```go
func (e *ReseedEngine) startCrashDetector(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        cutoff := time.Now().Add(-10 * time.Minute)
        var count int64
        e.db.Model(&ReseedTask{}).
            Where("status = ? AND updated_at < ?", "in_progress", cutoff).
            Count(&count)
        if count > 0 {
            log.Warnf("Detected %d stuck tasks, recovering...", count)
            e.recoverStuckTasks(ctx)
        }
    }
}
```

### 验证清单
- [ ] ReseedEngine 启动前调用恢复函数
- [ ] 系统崩溃后任务能正确恢复（测试）
- [ ] 无无限等待的 "in_progress" 任务

---

## 九、P1 问题改造清单（高优先级）

### [P1-1] PublishCandidate 四场景字段不一致

**场景对照**（决策 #214 附表）：

| 字段 | Scene B (postPush) | Scene C (postPush) | Scene C/D (onWatchCompleted) | Scene F (submit) |
|------|-------------------|-------------------|------------------------------|-----------------|
| **SubscriptionID** | ✅ Sub.ID | ✅ 同 B | ✅ 保持 | ✅ `manual:uuid` |
| **SourceClientID** | ❌ 空 | ❌ 空 | ⚠️ 无更新 | ✅ `req.ClientID` |
| **ClientID** | ✅ 目标 | ✅ 同 B | ✅ 保持 | ✅ `req.TargetClientID` |
| **Role** | ✅ "download" | ❌ "source"(!) | ⚠️ "source" | ✅ "manual" |
| **LocalSavePath** | ✅ 设置 | ✅ 同 B | ❌ 缺 CC-1 | ✅ 设置 |

**改造**：
- [ ] Scene C: Role 改回 "download"（应重用 Scene B 的 candidate，而非创建新）
- [ ] Scene F: SourceClientID 赋值 req.ClientID
- [ ] Scene C/D onWatchCompleted: 补齐 LocalSavePath 等 4 字段更新（CC-1 遗漏）

### [P1-2] PP-3 — UserOverrides 未传递

**位置**：PublishPipeline Step 16（FIELD_MAPPING）

**改造**：在映射前解析用户覆盖
```go
if candidate.UserOverrides != "" {
    var overrides UserEditedFields
    json.Unmarshal(candidate.UserOverrides, &overrides)
    for k, v := range overrides.ExtraFields {
        if v != "" {
            mappedFields[k] = v  // 覆盖自动值
        }
    }
}
```

### [P1-3] PP-4 — Dupe-update 的 InfoHash 缺失
**改造**：同 P0-5（补查 hash）

### [P1-4~9] 其他 P1 问题
详见审计报告完整版，分阶段执行

---

## 十、完整改造执行步骤

### 第 1 阶段：设计检查（0.5h）
- [ ] 验证所有 Canonical 定义已完整（§33.1.1/14a/15a/15c）
- [ ] 确认本文档中的"改造点"位置准确性

### 第 2 阶段：Converter 实现（0.5h）
- [ ] 实现 SubscriptionIDConverter（Go struct + 5 方法）
- [ ] 实现 ClientIDConverter（Go struct + 5 方法）
- [ ] 实现 SiteNameConverter（Go struct + 4 方法）
- [ ] 单元测试：覆盖所有转换场景

### 第 3 阶段：应用改造（2h）
按优先级改造代码位置：

**P0-1 优先**（最关键，影响 7 处）：
```bash
# 搜索并替换 fmt.Sprintf("%d", sub.ID) → Converter
grep -rn "fmt.Sprintf.*%d.*ID" src/
```

**P0-2 + P0-3**（各 5-10 处）：
```bash
grep -rn "ClientPathMapping\|ClientPublishTarget" src/
grep -rn "site\.ID\|Site\.ID" src/
```

**P0-6 + P0-7**（各 1-2 处）：
逐位置手工验证

### 第 4 阶段：验证（1h）
- [ ] 编译无误：`go build ./...`
- [ ] 类型检查：`go vet ./...`
- [ ] 单元测试：`go test ./... -v`
- [ ] 集成测试：验证 RSS 推送 → PublishCandidate → 发布的完整链路
- [ ] 回归测试：旧功能无破坏

### 第 5 阶段：Code Review（1h）
- [ ] 每个改造点都有清晰的 Converter 调用
- [ ] 无遗漏的 fmt.Sprintf 或字符串拼接
- [ ] 错误处理完整（特别是 Converter 的 error 返回）
- [ ] 日志信息清晰（使用 domain/name 而非 ID）

---

## 十一、风险评估和应急方案

| 风险 | 影响 | 应急 |
|------|------|------|
| Converter 实现有 bug | 所有转换失败 | 单元测试覆盖 100%，特别是边界情况 |
| 遗漏改造点导致类型错误 | 编译失败或运行时崩溃 | grep 验证所有模式都已替换 |
| 反向查询 JOIN 性能下降 | 高频查询变慢 | 添加数据库索引（FK 字段） |
| 数据库迁移遗漏 | 旧数据不兼容 | HRSeedTimeH 默认值为 0（兼容） |

---

## 十二、预期验收指标

**编码完成后**：
- ✅ 0 个 `fmt.Sprintf` + SubscriptionID
- ✅ 0 个 ClientPathMapping FK 直接赋值（所有用 Converter）
- ✅ 0 个业务代码用 Site.ID
- ✅ 所有 API 返回不暴露 uint ID
- ✅ 编译通过 + 全量测试通过

**设计闭合**：
- ✅ 22 个审计问题已分类
- ✅ 50+ 改造位置已定位
- ✅ 可逐个执行的改造清单

---

## 附录 A：改造点速查表

| ID | 类别 | 位置 | 优先级 | 影响 |
|----|------|------|--------|------|
| 1.1-1.7 | DB-1 | L3954/4000/4198/35040/35074/35495/35547/10150 | 🔴 P0 | RSS 推送 + 手动发布 |
| 2.1-2.4 | DB-2 | ClientPathMapping/ClientPublishTarget CRUD | 🔴 P0 | 辅种引擎 + 发布管道 |
| 3.1-3.10 | DB-3 | 业务路由、API、日志、FK | 🔴 P0 | 全系统 |
| 4.0 | SA-1 | 无（已完成） | ✅ | - |
| 5.1 | PP-1 | PublishPipeline Step 18 | 🔴 P0 | Dupe 发布 |
| 6.1 | CR-1 | ReseedEngine Step 6 | 🔴 P0 | 大规模辅种 |
| 7.1-7.2 | ER-1 | ReseedEngine.Start() | 🔴 P0 | 系统崩溃恢复 |
| 8.1-8.4 | P1-1 | PublishCandidate 四场景 | 🟠 P1 | 发布管道 |
| 9.1 | P1-2 | Step 16 UserOverrides | 🟠 P1 | 手动发布体验 |
| 10.1 | P1-3 | PP-4 Dupe-update | 🟠 P1 | 编辑发布 |

---

## 附录 B：后续改进项（P2 问题）

### P2-1: 同类问题未全量处理
- [ ] 检查是否还有其他 uint↔string 混用的类型（如 TorrentID、RuleID 等）
- [ ] 建立类型转换的通用模式库

### P2-2: DEPRECATED 定义清理
- [ ] 标注所有旧的 IsFree/DiscountLevel 定义为 DEPRECATED
- [ ] 编码前删除这些定义

### P2-3: 字段 tag 不一致
- [ ] 统一 JSON tag 命名（snake_case vs camelCase）
- [ ] 确保 gorm tag 正确

### P2-4: 错误处理遗漏
- [ ] 审查所有 Converter 调用，验证 error 处理
- [ ] 日志记录 + 告警

### P2-5: 文档缺陷
- [ ] 补齐 API 文档（Domain 的用途）
- [ ] 补齐开发者手册（Converter 使用规范）

---

**文档版本**：v1.0（Sprint 91 审计版本）  
**最后更新**：2026-04-30  
**维护者**：设计审计组  
**下一步**：立即执行 P0-1 → P0-2 → P0-3 → P0-5 → P0-6 → P0-7
