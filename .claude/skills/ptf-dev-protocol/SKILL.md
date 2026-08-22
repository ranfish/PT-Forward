---
name: ptf-dev-protocol
description: 'PT-Forward 开发前强制协议：动手前先查设计文档（docs/31-模块设计决策记录.md、docs/32-站点适配器设计/<站点>.md），再查已有基础设施、优先复用公共函数。Use when: 开始任何开发、修复、优化、重构、站点适配任务之前；提出新方案之前；查找 Detector/Fetcher/Searcher/Resolver/Matcher/Service/Engine 等已有设施；判断问题是"能力缺失"还是"接入遗漏"；需要制作组提取/搜索关键词/种子分类等公共函数。'
---

# PT-Forward 开发前协议（设计文档优先 + 复用优先）

任何开发、修复、优化，动手前必须先走本协议。核心教训（v0.0.606）：跳过调研直接写码，修复成本远超先查文档的成本。

## 流程总览

```
问题/需求
  ↓ ① 先查设计文档（见下）
  ↓   有记录 → 按设计实施；实现与设计不符 → 修正实现（不是绕过设计）
  ↓   无记录 → 与用户讨论设计 → 设计落盘 → 再实施
  ↓ ② 实施开发（复用优先，见下）
  ↓ ③ 灵魂四问 + 回归审核 → 见 skill: ptf-quality-gate
  ↓ ④ 设计文档更新（实施中的偏差与新发现回写设计文档）
```

## 第一步：查设计文档（权威源）

| 文档 | 内容 | 查法 |
|------|------|------|
| `docs/31-模块设计决策记录.md` | 模块设计决策（§55 起，67000+ 行） | grep 关键词定位章节，按需读 50-100 行上下文，**不要一次读全文** |
| `docs/32-站点适配器设计/<站点>.md` | 站点行为/标记/规则的权威定义 | 按站点名找文件 |

### 决策分支

- **有记录** → 按设计实施
- **实现与设计不符** → 修正实现（不是绕过设计）
- **无记录** → 与用户讨论设计 → 设计落盘（写入 docs/31 或 docs/32）→ 再实施

### 反面案例（v0.0.606）

extractFlags 用 `Contains("禁转")` 扫声明文本，偏离 §32/keepfrds.md 六章定义的"站点文字标记"权威源；在其上叠加致谢模板（含"禁转PTT"）时未查设计 → 跨词伪命中 + 定向禁转误判 + 误标 3 个种子禁转。

## 第二步：查已有基础设施（复用优先）

**核心原则**：遇到新问题先查已有基础设施是否有可用数据或方法，再决定是否新建。PT-Forward 基础设施已经很完善，真正的瓶颈往往是**接入遗漏**而非能力缺失。

### 调研清单（提"新方案"前必须过一遍）

1. `grep -rn 'func.*Xxx' internal/` 查全项目是否已有同名/同义函数；`grep` 已有的 `Detector`/`Fetcher`/`Searcher`/`Resolver`/`Matcher`/`Service`/`Engine`（命名约定：能力以接口/结构体名暴露）
2. **对比同类 handler** 的做法（如 `publish_torrents_handler` vs `manual_forward_handler`，一个接入了某设施，另一个可能遗漏）
3. **查数据表内容**：`release_group_mappings`、`SiteCoverageCache`、`torrent_metadata` 等表可能已有你需要的数据（`sqlite3 data/pt-forward.db "SELECT ..."` 确认）
4. 确认是"**能力缺失**"（需新建）还是"**接入遗漏**"（已有设施没调用）

典型案例（§56.33）：tid 反查（`reseed.SearchAndVerifyMatch` 已有）、源站识别（`SourceSiteDetector.Detect` 已有）、三源字段合并（`metadata.Merge` 已有）——六个看似需要"新建"的问题，实际全是"接入遗漏"。

### 典型公共函数清单（禁止重复造轮子）

| 函数 | 用途 |
|------|------|
| `util.ExtractGroupName` | 制作组名提取（曾有 publish/reseed 两份副本，改一处漏一处导致 bug，已合并） |
| `reseed.ExtractSearchKeyword` | 搜索关键词提取 |
| `reseed.SearchAndVerifyMatch` | 搜索+验证匹配（tid 反查用它） |
| `reseed.VerifyMatchWithTruncationCheckAndSource` | 匹配验证核心 |
| `reseed.ValidateInjection` | 注入前校验 |
| `util.ClassifyTorrent` / `util.ClassifyFromDir` | 种子类型分类（movie/tv_series/music + season_pack/partial_pack/single_episode） |
| `util.DetectContentType` | 音乐/视频检测原语 |
| `formatBytes` / `formatSpeed`（`web/src/utils/format.ts`） | 前端字节/速率格式化（ESLint 硬约束禁止私有副本） |

### 私有函数提升规则

已有函数是私有的且跨包需要时，**先考虑提升为公共函数**（改首字母大写或委托到 `util` 包），而非在新包重写一份。

## 完成开发后

→ 进入 `ptf-quality-gate`（灵魂四问 + 回归审核），再进入 `ptf-deploy`（编译部署）。
