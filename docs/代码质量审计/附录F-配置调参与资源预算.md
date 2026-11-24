# 附录F 配置调参清单与资源预算表

- **状态**：生效（随 07-配置与魔法值治理 整改建立）
- **日期**：2026-11-24
- **来源**：[07-配置与魔法值治理](./07-配置与魔法值治理.md) §2.1/§2.3 落地
- **用途**：调参前先查本清单——「该改哪层、默认值是什么、怎么生效」

## 1. 三层配置分层约定

| 层 | 适用 | 生效方式 | 例子 |
|---|---|---|---|
| **启动配置**（config） | 进程级·需重启 | YAML / `PTF_*` env（`internal/config`） | 端口/DB/日志/加密密钥 |
| **运行时设置**（setting） | 业务策略·可热更 | DB `system_settings` 表（`internal/setting`·30s TTL 缓存） | 截图/清理/限流/图床 |
| **代码不变量**（const） | 协议/安全上限·需发版 | 命名常量 | eventBusConsumerTimeout |

新增可调值判断序：能热更进 setting → 影响进程进 config → 协议/安全不变量留 const。

## 2. 启动配置清单（config.go DefaultConfig）

| 键 | 默认值 | 说明 |
|---|---|---|
| server.host/port | 0.0.0.0 / 8765 | 监听地址 |
| server.mode | release | gin 模式 |
| database.driver/sqlite_path | sqlite / ./data/pt-forward.db | DB |
| database.max_open/idle_conns | 10 / 5 | 连接池 |
| log.directory/level/max_age_days | ./logs / info / 30 | 日志 |
| security.encryption_key | （启动生成） | §仓库安全-3 |
| env 覆盖 | PTF_LOG_LEVEL / PTF_DATABASE_LOGLEVEL | BindEnv ×2 |

## 3. 运行时设置清单（setting DefaultSeeds·节选高频）

| 键 | 默认 | 边界建议 |
|---|---|---|
| rate_limit.enabled/global/write/download | true / 600 / 200 / 50 | 请求窗口·调大需评估并发容量 |
| screenshot.enabled/count/min_interval/jpeg_quality/cache_days | false / 6 / 60s / 85 / 30 | 截图链·count 受站方 images 上限 |
| data_cleanup.*_days | 30（PTGen/Archive/Audit 90） | 清理保留天数 |
| login_lockout.* | false / 5 次 / 5min | 登录锁定 |
| transfer_cooldown_seconds / seeding_delete_rules_global | — / false | 刷流策略 |
| image_host_strategy | （空=默认） | 图床选择 |

## 4. 代码不变量清单（命名常量·节选）

| 常量 | 值 | 位置 | 语义 |
|---|---|---|---|
| eventBusConsumerTimeout | 60s | pusher/eventbus | 消费者超时（附录A §3） |
| EventBusDrainTimeout | 10s | pusher/eventbus | 停机排空 join 上限 |
| preloadBatchSize | 500 | reseed/engine_preload | 预载分批 |
| seedRecordsIndexWarnThreshold | 50000 | reseed/engine_match_prepare | 做种索引增长观测 |
| defaultMaxConcurrent | 2 | httpclient/domain_limiter | 单域并发 |
| RetryConfig（MaxRetries/Base/Max） | 3 / 500ms / 30s | httpclient/retry_transport | HTTP 重试 |
| parseLimitQuery（默认/上限） | 20 / 200 | api/pagination | top-N limit 契约 |
| parsePagination（默认/上限/maxPage） | 20 / 200 / 10000 | api/pagination | 分页契约 |

## 4.5 GetIntDefault 使用现状

`GetIntDefault(key, def, min, max)` 已就位（`internal/setting/runtime.go`）但**当前零生产调用点**——现有 `GetInt` 4 处（artifact_generator ×3/syncer ×1）均有本地默认回退（如 `cfg.Count == 0 → 6`），无静默掩盖风险。**新代码一律 GetIntDefault**；存量 GetInt 触达时顺路迁移。

## 5. 资源预算表（并发/队列容量·§2.3 盘点）

| 资源 | 值 | 位置 | 理由 |
|---|---|---|---|
| **外站·域名级** | 并发 2 / 30req/60s / 冻结封顶 10min | domain_limiter | 站方封禁规避（§keepfrds 流控探究） |
| **外站·任务级** | stats 同步 5/10·截图后处理 6·发布 2·孤儿恢复 10 | 各 handler | 单任务批并发·与域名 limiter 正交 |
| **DB·写队列** | bufSize 参数化（write_queue） | db/write_queue | 异步写聚合 |
| **事件** | eventbus 256 / audit 1000 / side_load 1000 / cloud_fp 1000-5000 | 各组件 | 消费速率×突发余量 |
| **进程·内存** | MemoryConfig（config）+ memMonitor | config/db | 峰值观测 |

调优规则：外站并发优先走 domain_limiter/站点配置（非 goroutine 信号量）；调大任何并发前查本表同行资源上限。

## 6. 维护规则

1. 新增 setting 键必须同步 §3；新增命名常量同步 §4；新增并发/容量同步 §5。
2. 同类值禁止跨层重复定义（如超时既在 config 又在 setting）。
3. env 覆盖入口统一登记：变量名/默认值/允许范围（当前仅 PTF_* ×2 + BDINFO_WORKERS）。
