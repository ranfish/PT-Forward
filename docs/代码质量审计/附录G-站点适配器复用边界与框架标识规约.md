# 附录G 站点适配器复用边界与框架标识规约

- **状态**：生效（随 08-站点适配器复用治理 刀1 建立）
- **日期**：2026-11-26
- **来源**：[08-站点适配器复用治理](./08-站点适配器复用治理.md) §2/§3 刀1 落地（基线 `9d1e4239`·行号为快照·实施前重新定位）
- **用途**：动 adapter/framework/SiteConfig 构造之前先读本附录——「framework 合法取值在哪定义、URL 基址该用哪个 helper、config 副本怎么搓不漏字段」

## 1. framework 标识规约

### 1.1 权威来源与七处消费点（事实表）

**权威来源：`internal/adapter/factory.go` `Factory.Create` 的 switch（13 项）**。其余六处是消费/展示副本，漂移不报错：

| 位置 | 形式 | 项数 | 覆盖情况 |
|---|---|---|---|
| `factory.go` `Factory.Create` | switch | **13（权威）** | nexusphp/tnode/yemapt/mteam/cola/piggo/hdarea/hxpt/unit3d/gazelle/rousi/luminance + default 兜 generic |
| `factory.go` `FrameworkDefaults` | map | 10 | 见 §1.3 有意不注册规则 |
| `model/config_types.go:277` `FrameworkLabels`+常量 | map | 8 | 缺 cola/yemapt/piggo/hdarea/hxpt |
| `model/model_test.go:335` `frameworks` | 测试手抄 | 8 | 与 FrameworkLabels 相同（手抄=锚基准失真） |
| `adapter/factory_test.go:25/:58` | 测试手抄 | 6+6 | 子集 |
| `web/src/views/sites/SiteDetail.vue:447` | 前端手抄 | 11 | **缺 piggo、yemapt**（现成漂移例） |

`ValidFramework`/`FrameworkLabels`/8 个类型常量在生产代码**零调用**（只与自己的测试对话）——补全/改它们不改变运行时行为（08 刀3 范畴）。

### 1.2 两套词汇域：`nexusphp` 与 `np`

| 域 | 取值 | 消费面 |
|---|---|---|
| adapter 路由域（`sites.framework`） | `"nexusphp"` | `Factory.Create` 只认全称·无 `np` 分支 |
| 发布表单域（`PublishFormConfig.Framework`） | `"np"`（另有 4 处 `nexusphp`） | migration 表单 JSON/`form_config_diff.go` 草稿默认值 |

**边界规则**：两域各自闭合·当前无直传（`np` 进 `Factory.Create` 会落 default 分支得到 `GenericAdapter("np")`——能力静默降级）。**未来跨边界传递必须走显式归一化函数**，禁止散写字符串比较。

### 1.3 `FrameworkDefaults` 有意不注册规则（不是遗漏）

`hdarea`/`hxpt`/`piggo` 不注册是**设计决策**（`docs/31` §59.533 刀2 与联合回归项 C）：

- hdarea/hxpt：API-only 无 RSS 站——`HashStrategy` 等 Defaults 字段全为 RSS 消费·注册无意义
- piggo：由 migration173 在站点行直接写 `hash_strategy=fake_from_id`/`id_strategy=path_segment` 覆盖

**禁止机械补齐**。新增 API-only 站时同样不注册。

### 1.4 新增 framework 更新清单（checklist）

新增一个 framework 时**逐处核对**（漏改无测试变红）：

1. `factory.go` `Factory.Create` switch 加 case（**权威来源·必改**）
2. `model/config_types.go` 常量 + `FrameworkLabels` 条目（08 刀3 落地后与测试单一来源对齐）
3. `web/.../SiteDetail.vue` `frameworkLabels` 前端标签（走前端三步构建）
4. RSS 站评估 `FrameworkDefaults` 注册（API-only 站不注册·§1.3）
5. 测试手抄清单（刀3 落地后由单一来源派生·不再手抄）

## 2. URL 基址 helper 规约

### 2.1 三套 helper 事实表（代码实态·不凭函数头注释判语义）

| helper | 位置 | 语义（实态） | 消费面 |
|---|---|---|---|
| `siteBaseURL` | `adapter/errors.go:142` | BaseURL 非空→去尾斜杠；Domain 含协议→直用去斜杠；裸域→补 `https://` | 5 处（hdarea/hdcity/hxpt/nexusphp_edit/piggo）·§59.533 定为公共单点·§59.575 收编 hxpt 手搓 |
| `resolveBaseURL` | `adapter_mteam_upload.go:276` | BaseURL→Domain + `HasPrefix("http")` 补协议 + `TrimRight` 去斜杠（**§59.559 创建起即带·git log -L 零修改**） | 12 文件（gazelle/cola/mteam 族/rousi/tnode/yemapt 等） |
| `baseURLOf` | `adapter_generic.go:676` | **委托 `resolveBaseURL`**（08 刀2·v0.0.1640）——输出规范（补协议+去尾斜杠）；刀2 前曾是裸取旧形态 | 2 处（generic:109/nexusphp_upload:57）喂 `BuildNexusDownloadURL`——委托后兜底成死路径（幂等分支恒不触发·保留为公共契约） |

### 2.2 语义分层与选用规则

- **标准三态**（BaseURL 含协议/Domain 含协议/裸 Domain）下 `siteBaseURL` ↔ `resolveBaseURL` 输出逐字一致——可互相委托的一对。第四态（BaseURL 非空但不含协议）两者不同·正常配置不出现。
- ~~`baseURLOf` 是裸取旧形态~~（刀2 前历史）——**刀2（v0.0.1640）委托 `resolveBaseURL` 后自身已规范**·helper 层与 resolveBaseURL 等价（别名）。委托前裸 Domain 返回 `pt.site.com` 的差异已消失·下游 `BuildNexusDownloadURL` 兜底成死路径（幂等分支恒不触发——保留为公共契约·`TestBuildNexusDownloadURL_Idempotence` 独立锚锁〔08 回归审核修正补〕）。
- **选用**：新代码一律 `siteBaseURL`（公共单点）；`resolveBaseURL` 消费面暂不动（行为变更需专项授权）；`baseURLOf` 已规范——**新增消费点无需评估**（曾需评估的「裸取喂下游」前提随委托消失）。

### 2.3 等价性核对纪律

**以代码本体 + `git log -L` 为准·不据函数头注释判语义**——`resolveBaseURL` 的函数头注释讲「纯 Domain 构造」是修复前的问题叙述·代码早已带补协议（08 精读初稿即被此误导·后经 git log -L 实证纠正）。本纪律同样适用于**测试锚注释**：08 回归审核实证——baseURLOf 委托后兜底成死路径·四态快照锚的「防兜底被改/防新消费点」声明不成立（虚拟安全感·与 §2.1 批评的同款错误）——已诚实化+补幂等独立锚（TestBuildNexusDownloadURL_Idempotence）。

## 3. SiteConfig 构造约定域（历史病灶收编）

### 3.1 `config.Domain` 两种形态与消费链

同一 `model.SiteConfig.Domain` 字段·两条构造链塞入两种形态（§59.575 病灶实证·`https://https//...` 双 scheme 炸点）：

| 构造链 | Domain 形态 | 位置 |
|---|---|---|
| provider 单点 | **裸域**（`s.Domain`） | `site/provider.go:461` 等 |
| 统计同步手搓 | **完整 URL**（`site.BaseURL`） | `site/stats_sync.go:202` |

现状：`siteBaseURL` 三态兼容兜底已就位·但**无成文约定**——新构造链仍可能踩同种坑。

### 3.2 构造规则

1. **优先走 `provider.go` 单点**构造 config（形态有权威源）
2. 手搓副本**必须逐字段对齐清单**（§59.574 病灶：`stats_sync.go:202` 手搓漏 `DownloadMode`/`DownloadURLTemplate` → `cuhashEnabled` 恒 false → `scrapeCuhash` 永不执行——门修了输入没接·设计实现脱节）
3. 手搓副本的 Domain 形态**必须声明**塞的是裸域还是完整 URL·并核对消费方预期（adapter 的 URL 基址 helper 三态兼容·但 `stats_sync` 链历史形态是 BaseURL 完整 URL——改前先查全链消费）

## 4. API 请求骨架与复用边界

### 4.1 骨架重复现状（四族·按同签名）

`detailViaAPI`×3 / `slViaAPI`×3 / `fetchUserStatsAPI`×4 / `apiCall`×2 同签名（hdarea/hxpt·piggo 为变体不属同签名组）。骨架一致：`NewRequestWithContext → 设头 → Do → drainBody → readBody → JSON`；差异全在方法/URL/头/响应结构（**方言**）。

### 4.2 机制下沉边界

沿用 `adapter/apihelper.go` 成文铁律：**「共享机制·不共享方言」**——只可下沉 `Do → drainBody → readBody → Unmarshal → parseError` 纯机制段·请求构造与响应结构留在方言层。鉴权头/限流特判（如好学 429）属方言**不得合并**。下沉前逐点核对错误包装等价性（有的用 `networkError`/`parseError`·有的原始 err）。

## 5. 三次法则与小 helper 升级路径

跨包同名/同语义小 helper（`extractDoubanID`×2 实现不同/`gpwFirst`↔`firstNonEmpty`）**维持现状**：出现包外消费者或第三站证明流程稳定时再升公共 util·不机械搬迁。adapter 包 153 个包内私有函数散落属常态·非治理对象。

## 6. 维护规则（触发场景）

| 变更场景 | 必读节 |
|---|---|
| 新增 framework | §1.4 checklist 逐处核对 |
| 构造 URL / 动 URL 基址 | §2.2 选用规则 |
| 判 helper 语义 / 等价性 | §2.3 核对纪律（代码本体+git log -L） |
| 构造 SiteConfig 副本 | §3.2 逐字段清单 |
| 下沉 API 请求骨架 | §4.2 机制/方言边界 |
| 跨域传 framework 值 | §1.2 显式归一化 |
