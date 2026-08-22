---
name: ptf-quality-gate
description: 'PT-Forward 代码改动后质量门禁：灵魂四问（nil 安全/边界安全/go vet+go test 回归/前端构建）+ 回归审核（AGENTS.md 任务焦点同步）+ 代码改动后强制清单（设计落盘/先提交后部署/打 tag/AGENTS.md 更新）。Use when: 完成任何代码修改之后、git commit 之前、代码审核或回归审核时、排查改动是否遗漏检查项、GORM 错误处理与 JSON 命名规范疑问。'
---

# PT-Forward 质量门禁（灵魂四问 + 回归审核）

每次代码改动后必须逐条过完本门禁，不可跳过。全部通过后才可进入部署（见 skill: ptf-deploy）。

## 代码改动后强制清单（不可跳过）

1. **设计落盘** — 改动记录到 `docs/31-模块设计决策记录.md` 或 `docs/32-站点适配器设计/`
2. **灵魂四问** — 见下文，逐条审核
3. **先提交后部署** — `git commit + push` → 再编译部署（**禁止先部署后提交**）
4. **打 tag** — 如需发布新版本（`git tag vX.X.X && git push origin vX.X.X`）
5. **AGENTS.md 更新** — 如任务状态变化

## 回归审核第一项（每次审核先查）

**AGENTS.md 任务焦点同步**：`git log --oneline -10` 的版本推进与 AGENTS.md"当前主线"描述对照——主线/支线完成项是否已反映？滞后即先补。

历史教训：v0.0.631、v0.0.642、v0.0.670 三次同类违规。

## 灵魂四问

### 1. nil 安全

- 所有指针返回值是否检查了 nil？
- map 查找是否有 ok 判断？
- type assertion 是否用了 comma-ok 模式？

### 2. 边界安全

- 空输入 / 空 DB / context 取消 / 并发锁竞争等边界情况是否处理？
- `context.WithTimeout` 后是否都调了 `cancel()`？
- 锁是否有嵌套导致死锁风险？

### 3. 回归通过

`go vet` + `go test` + `vue-tsc` + `eslint` 是否全部通过？

- 后端：`go vet ./internal/... ./cmd/pt-forward/` + `go test ./internal/... -count=1 -timeout 180s`
  - **不要**用 `go vet ./...`（`cmd/verify-pieces-hash` 有已知冲突会报错）
  - **删除代码后**必跑 vet，确保测试文件不引用已删符号
- 前端：`vue-tsc -b --noEmit` + `npx eslint src/`（在 `web/` 下执行，PATH 前置 `/home/incast/.local/bin`）

### 4. 前端构建

本次改动是否涉及 `web/` 目录？

- 如果是 → 必须 `vite build → cp -r web/dist frontend/dist → go build`，三步缺一不可
- `vue-tsc`/`eslint` 只是验证，**不是构建**。`go build` embed 的是 `frontend/dist`，不是 `web/src`。跳过构建 = 前端改动不生效

## 常见易错点（历史教训）

| 场景 | 规则 |
|------|------|
| GORM `Updates`/`Update` | 必查 `.Error` 并处理（243 实测 760 次静默失败的教训，§59.56） |
| API 专用 struct 新增字段 | 在已用 camelCase 的 struct（`updateSiteRequest`、`siteResponse`、`downloaderResponse` 等）中必须用 camelCase，否则前端反序列化失败；Go model JSON tag 保持 snake_case |
| 前端字节/速率显示 | 必须 `import { formatBytes / formatSpeed } from '@/utils/format'`，ESLint 硬约束禁止私有副本（§59.43） |
| 长任务（截图/批量获取） | 脱离 HTTP 生命周期，否则 context canceled（§59.51） |

## 全部通过后

→ 进入 `ptf-deploy`（编译部署流程）。
