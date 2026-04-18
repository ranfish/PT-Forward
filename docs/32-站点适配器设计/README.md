# 站点适配器设计文档

> 本目录包含各个 PT 站点的特异化适配器设计文档，每个站点独立一个文档，便于维护和扩展。

## 目录

| 站点 | 文档 | 状态 |
|------|------|------|
| 青蛙 | [qingwapt.md](./qingwapt.md) | ✅ 完成 |
| 红豆饭 | [hdfans.md](./hdfans.md) | ✅ 完成 |
| 13城 | [13city.md](./13city.md) | ✅ 完成 |
| GTK | [gtk.md](./gtk.md) | ✅ 完成 |
| HDVideo | [hdvideo.md](./hdvideo.md) | ✅ 完成 |
| Nova高清 | [novahd.md](./novahd.md) | ✅ 完成 |
| OKPT | [okpt.md](./okpt.md) | ✅ 完成 |
| PTFans | [ptfans.md](./ptfans.md) | ✅ 完成 |
| SBPT | [sbpt.md](./sbpt.md) | ✅ 完成 |
| TLF | [eastgame.md](./eastgame.md) | ✅ 完成 |
| 阿玲 | [aling.md](./aling.md) | ✅ 完成 |
| 奥申 | [oshen.md](./oshen.md) | ✅ 完成 |
| 百川 | [hitpt.md](./hitpt.md) | ✅ 完成 |
| 包子 | [baozi.md](./baozi.md) | ✅ 完成 |
| 铂金家 | [pthome.md](./pthome.md) | ✅ 完成 |
| 不可杜 | [hddolby.md](./hddolby.md) | ✅ 完成 |
| 不可说 | [springsunday.md](./springsunday.md) | ✅ 完成 |
| 不可羊 | [tjupt.md](./tjupt.md) | ✅ 完成 |
| 财神 | [cspt.md](./cspt.md) | ✅ 完成 |
| 彩虹岛 | [chdbits.md](./chdbits.md) | ✅ 完成 |
| 城市 | [hdcity.md](./hdcity.md) | ✅ 完成 |
| 藏宝阁 | [cangbao.md](./cangbao.md) | ✅ 完成 |
| 车站 | [carpt.md](./carpt.md) | ✅ 完成 |
| 传道院 | [cdy.md](./cdy.md) | ✅ 完成 |
| 大青虫 | [cyanbug.md](./cyanbug.md) | ✅ 完成 |
| 碟粉 | [discfan.md](./discfan.md) | ✅ 完成 |
| 冬樱 | [wintersakura.md](./wintersakura.md) | ✅ 完成 |
| 分享站 | [itzmx.md](./itzmx.md) | ✅ 完成（仅目标站） |
| 轨道炮 | [railgunpt.md](./railgunpt.md) | ✅ 完成（仅目标站） |
| 海胆 | [haidan.md](./haidan.md) | ✅ 完成（仅目标站） |
| 海豚 | [dicmusic.md](./dicmusic.md) | ✅ 完成（Gazelle框架·纯音乐） |
| 憨憨 | [hhanclub.md](./hhanclub.md) | ✅ 完成（仅源站·全站官种） |
| 馒头 | [mteam.md](./mteam.md) | ✅ 完成（mTorrent自研SPA+API） |
| 好大 | [hdarea.md](./hdarea.md) | ✅ 完成（目标站·29音频编码） |
| 好多油 | [hdupt.md](./hdupt.md) | ✅ 完成（媒介TV/电影分开·UHD独立） |
| 好学 | [hxpt.md](./hxpt.md) | ✅ 完成（教育专题·字段全部重定义） |
| 皇后 | [opencd.md](./opencd.md) | ✅ 完成（NexusPHP定制·纯音乐·候选制） |
| 家园 | [hdhome.md](./hdhome.md) | ✅ 完成（双区域·8K分类·候选制·豆瓣ID） |
| 咖啡 | [ptcafe.md](./ptcafe.md) | ✅ 完成（source_sel=地区·30制作组·18音频·OPUS/OGG） |
| 克隆 | [hdclone.md](./hdclone.md) | ✅ 完成（极简字段·无source/audio·短剧·AV1） |
| 库非 | [kufei.md](./kufei.md) | ✅ 完成（Cloudflare·16分类·17媒介·22音频·游戏/电子书） |
| 昆仑 | [yhpp.md](./yhpp.md) | ✅ 完成（processing_sel=地区·19媒介·23音频·29制作组·19标签） |
| 垃圾堆 | [lajidui.md](./lajidui.md) | ✅ 完成（Cloudflare·processing_sel=文件格式·source_sel=地区·16分类·2K分辨率） |
| 聆音 | [soulvoice.md](./soulvoice.md) | ✅ 完成（双模式影视+阅听·电子书/有声书·字段语义按模式切换） |
| 龙之家 | [dragonhd.md](./dragonhd.md) | ✅ 完成（繁体中文·AV分类·无标签·2K/1440p·极简字段） |
| 萝莉 | [xloli.md](./xloli.md) | ✅ 完成（动漫向·双区域综合+9KG·禁止9KG·13动漫制作组·舞台演出·OPUS） |
| 末日 | [agsv.md](./agsv.md) | ✅ 完成（Cloudflare·种审制·27黑名单·双区域综合+学习·大包规则·ALAC/M4A） |
| 慕雪阁 | [muxuege.md](./muxuege.md) | ✅ 完成（HDR10编码·TXT/PDF编码·540p·47制作组·31标签·无音频编码） |
| 南洋 | [nanyangpt.md](./nanyangpt.md) | ✅ 完成（NYPT框架·极简发布·无质量下拉框·禁止蓝光原盘·剧集dupe·豆瓣链接） |
| 柠檬不甜 | [lemonhd.md](./lemonhd.md) | ✅ 完成（双语分类·4K/8K独立媒介·3D分类·PT-Gen四来源·匿名发布·5倍上传） |
| 农场 | [farmm.md](./farmm.md) | ✅ 完成（Cloudflare·双区域种子+特别·source_sel=地区·processing_sel=年级/分级·17媒介·15编码·20音频·1440p·儿童教育特色） |
| 朋友 | [keepfrds.md](./keepfrds.md) | ✅ 完成（**仅源站**·全站官种·Cloudflare·HEVC细分5级·8K·19分类·转载须24h后·黑名单制作组） |
| 葡萄 | [sjtu.md](./sjtu.md) | ✅ 完成（教育网·28分类按地区细分·编码含音频·禁止HEVC/10bit·黑名单组·豆瓣链接·校园原创） |
| 浦园 | [njtupt.md](./njtupt.md) | ✅ 完成（教育网·演出分类·资料分类·MediaInfo字段·PT-Gen四来源·极简质量字段·标准规则） |
| 麒麟 | [hdkyl.md](./hdkyl.md) | ✅ 完成（种审制·27黑名单组·processing_sel=年份·source_sel=地区19个·19音频·2K/480p·官种/驻站标签·MediaInfo·短剧） |
| 人人 | [audiences.md](./audiences.md) | ✅ 完成（Cloudflare·候选制·0day命名·无制作组/来源/地区字段·HDR三标签·Trump共存规则·Web-DL/WebRip·爆米花系统） |
| 朱雀 | [zhuque.md](./zhuque.md) | ✅ 完成（**TNode框架**·Vue SPA+REST API·CSRF Token·TMDb必填·H264/x264四分·ID分段体系·无音频编码·标签逗号分隔） |
| 肉丝 | [rousi.md](./rousi.md) | ✅ 完成（**自研框架**·Vue SPA+REST JSON API·Passkey认证·UUID种子·Base64截图·Markdown描述·动态属性·11分类·9KG专区·wiki规则待补充） |
| 幸运 | [luckpt.md](./luckpt.md) | 🚧 待完成 |
| 拾刻 | [ptskit.md](./ptskit.md) | 🚧 待完成 |

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
│   ├── audiences.go                  // AudiencesHook 人人 站点
│   ├── zhuque.go                    // ZhuqueHook 朱雀站点
│   ├── haidan.go                    // HaidanHook 海胆站点
│   ├── baozi.go                     // BaoziHook 包子站点
│   ├── luckpt.go                    // LuckPTHook 幸运 站点
│   ├── ptskit.go                    // PTSKitHook 拾刻 站点
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
*最后更新：2026-04-17*
