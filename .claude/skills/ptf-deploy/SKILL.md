---
name: ptf-deploy
description: 'PT-Forward 编译部署流程：先提交后部署铁律、部署检查清单、后端/前端构建脚本（go 全路径/CGO_ENABLED=1/版本号 ldflags）、systemd 用户服务重启、打 tag 发布（触发 Docker 镜像 + GitHub Release）、OTA 兼容（Restart=always）、环境隔离规则（29 可操作/生产只读）。Use when: 编译、部署到开发环境 29、重启服务、发布新版本、打 tag、OTA 更新、检查服务状态、Docker 镜像发布。'
---

# PT-Forward 编译部署

## 部署铁律（最高优先级，违反=事故）

1. **先提交后部署**：`git commit + push` 必须在编译/部署**之前**完成。代码必须在 git 历史中先于部署生效。**禁止先部署后提交**
2. **环境隔离**：
   - 开发只在开发机 **29**（`systemctl --user restart pt-forward`，端口 8765）
   - 可 SSH 到生产环境（249/243/65/12/pt30/242/22）做**只读**诊断（日志/磁盘/Docker 状态/DB 查询）
   - **禁止**在生产环境编译、修改代码或数据；生产更新由用户自行执行（`docker compose pull && up -d` 或 OTA）

## 部署检查清单（每次部署前逐条过，不能跳过）

- [ ] 灵魂四问全部通过？（见 skill: ptf-quality-gate）
- [ ] `git status` 是否有未提交的改动？→ 有则先 commit + push
- [ ] 需要打 tag 吗？→ `git tag vX.X.X && git push origin vX.X.X`
- [ ] 本次是纯后端改动？→ 不需要 vite build
- [ ] 本次涉及 `web/`？→ 必须 vite build → cp dist → go build（三步缺一不可）

## 环境与工具（易错）

| 项 | 值 |
|----|----|
| Go | `/home/incast/.local/go/bin/go`（v1.25，系统 PATH 中无 go，**必须全路径**） |
| Node | `/home/incast/.local/bin/node`（v22，系统 Node 18 不兼容） |
| CGO | 后端编译必须 `CGO_ENABLED=1` |
| DB | `data/pt-forward.db`（SQLite） |
| 服务 | `systemctl --user restart pt-forward`（用户级 systemd，端口 8765） |
| 版本号 | ldflags 必含 `-X main.version=$(git describe --tags --always --dirty)`，源码默认值 `"dev"` |

## 后端验证与部署（纯后端改动）

一条命令完成（vet → test → build → restart）：

```bash
bash .github/skills/ptf-deploy/scripts/build-backend.sh
```

等价手动步骤：
1. `go vet ./internal/... ./cmd/pt-forward/`（**不要**用 `./...`）
2. `go test ./internal/... -count=1 -timeout 180s`
3. `CGO_ENABLED=1 /home/incast/.local/go/bin/go build -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" -o pt-forward ./cmd/pt-forward/`
4. `systemctl --user restart pt-forward && sleep 2 && systemctl --user is-active pt-forward`

## 前端验证与部署（涉及 web/）

前端是 Go embed（`frontend/spa.go` 用 `//go:embed all:dist`），修改前端后必须重新构建并重新编译 Go 二进制才能生效：

```bash
bash .github/skills/ptf-deploy/scripts/build-frontend.sh
```

等价手动步骤：
1. `cd web/ && rm -rf node_modules/.vite && PATH="/home/incast/.local/bin:$PATH" ./node_modules/.bin/vite build`
2. `rm -rf frontend/dist && cp -r web/dist frontend/dist`
3. `CGO_ENABLED=1 /home/incast/.local/go/bin/go build -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" -o pt-forward ./cmd/pt-forward/`
4. `systemctl --user restart pt-forward && sleep 2 && systemctl --user is-active pt-forward`

## 打 tag 发布

```bash
bash .github/skills/ptf-deploy/scripts/release-tag.sh v0.0.XXX
```

- push tag 触发 CI：`docker-publish.yml`（GHCR + Docker Hub 镜像）、`release.yml`（GitHub Release 二进制，OTA 用）
- 镜像：`ghcr.io/ranfish/pt-forward:latest`（国外）/ `ranfish/pt-forward:latest`（国内）
- 生产环境更新由**用户自行执行**：OTA（前端侧栏 → 检查更新 → 立即更新）或 `docker compose pull && docker compose up -d`

## 部署到 243（开发验证环境，Docker）

```bash
bash .claude/skills/ptf-deploy/scripts/deploy-243.sh
```

- 前置：已 commit+push + 已跑 build-backend.sh（前端涉及另跑 build-frontend.sh）
- 流程：gzip+ssh 管道传输 → docker cp + 原子 mv → restart → 版本核对 + 前端资产 hash 比对（不一致即报错退出）

## systemd OTA 兼容（勿改坏）

`~/.config/systemd/user/pt-forward.service` 的 `[Service]` 段必须用 `Restart=always`（**不能**用 `on-failure`）：

- OTA 流程是"下载新二进制 → 校验 → 原子 rename → 主动 `exit 0` 等待 systemd 重启"，`on-failure` 不重启正常退出 → OTA 后服务永久停止、网页连不上
- 修改 service 配置后执行 `systemctl --user daemon-reload`
- `systemctl --user stop` 仍可正常停服务（`Restart=always` 只影响进程退出/崩溃，管理员 `stop` 不触发重启）
