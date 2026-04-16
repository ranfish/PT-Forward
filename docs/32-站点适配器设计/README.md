# 站点适配器设计文档

> 本目录包含各个 PT 站点的特异化适配器设计文档，每个站点独立一个文档，便于维护和扩展。

## 目录

| 站点 | 文档 | 状态 |
|------|------|------|
| 青蛙（QingWAP） | [qingwapt.md](./qingwapt.md) | ✅ 完成 |
| HDFans | [hdfans.md](./hdfans.md) | ✅ 完成 |
| 13City | [13city.md](./13city.md) | ✅ 完成 |
| GTK | [gtk.md](./gtk.md) | ✅ 完成 |
| HDVideo | [hdvideo.md](./hdvideo.md) | ✅ 完成 |
| NovaHD | [novahd.md](./novahd.md) | ✅ 完成 |
| OKPT | [okpt.md](./okpt.md) | ✅ 完成 |
| PTFans | [ptfans.md](./ptfans.md) | ✅ 完成 |
| SBPT | [sbpt.md](./sbpt.md) | ✅ 完成 |
| TLFBits (EastGame) | [eastgame.md](./eastgame.md) | ✅ 完成 |
| alingPT | [aling.md](./aling.md) | ✅ 完成 |
| OshenPT | [oshen.md](./oshen.md) | ✅ 完成 |
| 百川PT (HITPT) | [hitpt.md](./hitpt.md) | ✅ 完成 |
| 包子 (BaoZi) | [baozi.md](./baozi.md) | ✅ 完成 |
| PTHome (铂金家) | [pthome.md](./pthome.md) | ✅ 完成 |
| HDDolby | [hddolby.md](./hddolby.md) | ✅ 完成 |
| SpringSunday (CMCT) | [springsunday.md](./springsunday.md) | ✅ 完成 |
| 北洋园PT (TJUPT) | [tjupt.md](./tjupt.md) | ✅ 完成 |
| 财神PT (CSPT) | [cspt.md](./cspt.md) | ✅ 完成 |
| CHDBits | [chdbits.md](./chdbits.md) | ✅ 完成 |
| HDCiTY | [hdcity.md](./hdcity.md) | ✅ 完成 |
| 藏宝阁 (CBG) | [cangbao.md](./cangbao.md) | ✅ 完成 |
| CarPT | [carpt.md](./carpt.md) | ✅ 完成 |
| 传道院·PT (CDY) | [cdy.md](./cdy.md) | ✅ 完成 |
| PTerClub | [pterclub.md](./pterclub.md) | 🚧 待完成 |
| 朱雀 | [zhuque.md](./zhuque.md) | 🚧 待完成 |
| 海胆 | [haidan.md](./haidan.md) | 🚧 待完成 |
| 包子 | [baozi.md](./baozi.md) | 🚧 待完成 |
| LuckPT | [luckpt.md](./luckpt.md) | 🚧 待完成 |
| PTSKit | [ptskit.md](./ptskit.md) | 🚧 待完成 |

## 设计原则

### 1. 站点特异化处理
每个站点可能有特殊的发布规则、字段映射、标题格式要求等，需要通过 SitePublishHook 接口实现特异化处理。

### 2. 模块化设计
- 每个站点独立一个 Hook 文件
- 统一的 SitePublishHook 接口
- 集中式注册表管理

### 3. 代码复用
- 公共辅助函数提取到 `site_hooks/helpers.go`
- 避免重复代码，提高可维护性

### 4. 配置驱动
- 尽可能通过配置文件实现站点适配
- 代码 Hook 仅处理无法通过配置实现的逻辑

## 接口定义

```go
// SitePublishHook 站点发布钩子（发布前/后执行自定义逻辑）
type SitePublishHook interface {
    // BeforePublish 发布前钩子（修改发布请求）
    BeforePublish(ctx context.Context, req *PublishRequest) error

    // AfterPublish 发布后钩子（处理特殊后续动作）
    AfterPublish(ctx context.Context, result *PublishResult) error
}
```

## 包结构

```
internal/publish/
├── site_hooks/
│   ├── interface.go                 // SitePublishHook 接口定义
│   ├── helpers.go                   // 公共辅助函数
│   ├── registry.go                   // 钩子注册表
│   ├── qingwapt.go                  // QingWapHook 青蛙站点
│   ├── hdfans.go                    // HDFansHook HDFans 站点
│   ├── pterclub.go                  // PTerClubHook PTerClub 站点
│   ├── zhuque.go                    // ZhuqueHook 朱雀站点
│   ├── haidan.go                    // HaidanHook 海胆站点
│   ├── baozi.go                     // BaoziHook 包子站点
│   ├── luckpt.go                    // LuckPTHook LuckPT 站点
│   ├── ptskit.go                    // PTSKitHook PTSKit 站点
│   └── ...                          // 其他站点
└── ...
```

## 开发指南

### 新增站点适配器

1. 在 `internal/publish/site_hooks/` 创建新文件（如 `newsite.go`）
2. 实现 `SitePublishHook` 接口
3. 在 `registry.go` 中注册钩子
4. 在 `docs/32-站点适配器设计/` 创建对应文档

### 文档模板

每个站点适配器文档应包含：

1. **站点信息**
   - 站点名称
   - 站点框架（NexusPHP/UNIT3D/Gazelle 等）
   - 特殊规则说明

2. **核心规范**
   - 标题命名规范
   - 发布规则
   - 文件规范
   - 自查流程

3. **Hook 实现**
   - BeforePublish 逻辑
   - AfterPublish 逻辑
   - 关键辅助函数

4. **配置示例**
   - 字段映射配置
   - 标签映射配置
   - 其他站点配置

5. **测试用例**
   - 功能测试
   - 边界测试
   - 错误处理测试

## 参考资源

- PTNexus 站点适配器实现：`examples/PTNexus/server/internal/service/publish/publisher/sites/`
- 发布流水线设计：`docs/31-模块设计决策记录.md §11`
- 站点管理模块：`docs/31-模块设计决策记录.md §13`

## 维护说明

- 新增站点时，在此目录添加对应文档
- 站点规则变更时，及时更新对应文档
- 定期检查文档与实际代码的一致性

---

*文档维护：PT-Forward 开发团队*
*最后更新：2026-04-16*
