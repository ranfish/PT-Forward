---
name: pt-idx-ops
description: 'PT-IDX 云端指纹服务运维：项目路径/Go module/PostgreSQL DB/端口 8766/systemd 服务/验证部署四步/代码复用规则（bencode+compute 手动复制、pieces_hash 逐字节一致）/数据采集安全规则（不持久化 .torrent、凭证 AES-GCM 加密）。Use when: 操作 PT-IDX 项目、部署或重启 pt-idx 服务、修改 bencode/compute/pieces_hash 相关代码、PT-IDX 数据采集器开发。'
---

# PT-IDX 项目（云端指纹服务）

- **项目路径**：`/home/incast/PT-IDX`
- **Go module**：`github.com/ranfish/pt-idx`
- **DB**：PostgreSQL，本地实例，数据库名 `pt_idx`
- **服务**：`systemctl --user restart pt-idx`（用户级 systemd 服务，端口 8766）
- **CGO**：不需要（纯 Go + pgx 驱动）

## 验证与部署

1. `go vet ./...`
2. `go test ./... -count=1 -timeout 180s`
3. `/home/incast/.local/go/bin/go build -ldflags "-s -w" -o pt-idx ./cmd/pt-idx/`
4. `systemctl --user restart pt-idx && sleep 2 && systemctl --user is-active pt-idx`

## 代码复用规则

- `bencode.go` + `compute.go` 从 PT-Forward **手动复制**，禁止 import PT-Forward 包
- pieces_hash 计算逻辑必须与 PT-Forward **逐字节一致**
- PT-Forward 侧 bencode/compute 如有修改，PT-IDX 侧必须同步

## 数据采集规则

- 批量采集器/RSS 订阅器下载 .torrent 后立即计算 pieces_hash，**丢弃 .torrent 数据**
- 禁止在磁盘上持久化 .torrent 文件
- 站点凭证（cookie/passkey）必须 AES-GCM 加密存 PostgreSQL
- 禁止在日志中出现任何凭证
- 配置文件含敏感信息，必须加入 `.gitignore`
