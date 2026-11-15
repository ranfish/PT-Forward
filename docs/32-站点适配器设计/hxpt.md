# 好学 站点适配器设计

> **v2（2026-11-15 重适配）**：站方彻底改版离开 NexusPHP（Next.js SPA + /api/v1 REST）——旧 NP 适配（§59.453 v0.0.1308 槽位错位特化）全部作废。本文档以新架构为权威；旧 NP 错位表见文末历史存档。

## 站点信息

- **域名**: www.hxpt.org
- **架构**: hxpt（Next.js + /api/v1 REST·2026-11 改版）
- **鉴权**: cookie 同域直用（SPA 流）；POST/PUT 需 CSRF double-submit（cookie `flux_csrf` 值 ↔ header `x-csrf-token`·GET 免）
- **定位**: 综合站全品类（用户定案 2026-11-15 全放开——旧「仅纪录片」基于旧站教育定位作废）
- **RSS**: 无（404·获取走 API·可乐先例同构）

## 一、获取链 API 契约（2026-11-15 活体探针实测）

| 能力 | 端点 | 形态 |
|------|------|------|
| 搜索 | GET `/api/v1/torrents?search=&page=&limit=` | cursor 分页 next_cursor；**唯一搜索参数 `search`**（keyword/q/name 阴性对照实证被忽略）；条目含 info_hash/pieces_hash/sec_names/promotion/hr_policy |
| 元数据 | GET `/api/v1/torrents/{id}` | category_id/info_hash/pieces_hash/hr_policy/promotion/created_at/seeders/owner_name |
| 内容 | GET `/api/v1/torrents/{id}/detail` | descr BBCode+**mediainfo 独立字段**+sections 实例（dict_id/field_type/label/name/values）+numfiles+price |
| 下载 | GET `/api/v1/torrents/{id}/download` | 真 bencode（tracker.hxpt·cookie 态） |
| 用户 | GET `/api/v1/me` | username/class_name/uploaded/downloaded/seeding/uploads |
| 详情页 | `/torrent/{id}` | SPA 路由（DetailURL 拼接形态） |

### 关键字段形态

- **统一信封**: `{code, message, data}`；code===0 成功
- **hr_policy**: `{days:14, enabled:true, seed_hours:96}`——HR 判定直读
- **promotion**: 全站当前 null（无促销实证）；非 null 值形态未实证——adapter 宽容字符串匹配+日志留痕·首见后修正
- **created_at**: RFC3339 纳秒精度（`2026-11-06T01:27:35.361831Z`）
- **分类树**: 1电影/2电视剧/3短剧/4音乐/5游戏/6软件/7动漫/8纪录片/9电子书/10有声书/11体育/12综艺/13教育资源（+101 幼儿园 grade 维度）

## 二、发布契约（刀3 摸底定案+实施完成）

- **上传**: POST `/api/v1/torrents`（FormData·cookie+CSRF double-submit）
- **恒发字段**: category_id（必填·1-13 分类树）/anonymous（"true"/"false"）/file（种子文件）
- **条件发字段**（非空才发·对齐站方 l() 语义）: name/imdb/douban/tmdb/small_descr/descr/poster/mediainfo/sections/tags
- **恒不发字段**（用户定案/站方管理功能）: price（恒免费）/pos_state/pick_type/hr_punish/log/nfo
- **sections 提交形态**: `{"media":443,"codec":435,...}` **纯数字 dict_id**（select 类型不包装——UploadForm JS 逆向；multiselect=`{dict_ids:[]}` 站内无实例不实现）；四维=media/codec/source/team **全站统一值集**（五分类对照实证·section-dict 参数化只是形式）
- **tags 提交形态**: `[9,10]` 数值 id 数组 JSON（tags-dict 9 项：6首发/8禁转/9国语/10中字/11DIY/3官种/4合集/14完结/15独家）
- **响应**: code=0→`data.id`（detail=`/torrent/{id}`）；1002→data=errors map `{field:[msgs]}`；group_suggest.same_source=同组提示（不拦上传）
- **detail 端点的 sections（filetype/standard/year）**=站方自动分析回填维度（非用户提交面——最新种子实证）
- **用户定案**: price 恒不传（免费发布）+ anonymous 系统级默认匿名
- **形态未实证**（首颗暴露）: 重复发种 code/HTTP 形态；上传权限 403 文案

## 三、值集（section-dict 按分类拉取）

GET `/api/v1/section-dict?category_id=N`（分类 8 纪录片实测）：
- **media** 媒介 8（视频/音频/文档/图片/软件/书籍/游戏/其他）
- **codec** 编码 8（H.264/H.265/AV1/XviD/FLAC/MP3/APE/其他）
- **source** 来源 9（WEB-DL/WEBRip/BluRay/HDTV/DVDRip/CD/SACD/自制/其他）
- **team** 制作组 70（含 FRDS@469/CHDBits/TTG/CMCT/HDSky…）
- **全部非必填**（miss 不再是风险——与 NP 站必填槽语义根本不同）
- tags-dict 9 标签（国语/中字/DIY/合集/完结+人工类首发/禁转/官种/独家）
- PTGen 站方内置（/api/v1/ptgen?url=）
- modes 15 分类模式体系（默认模式/电子书/漫画/影视·电影/纪录片/动漫/教育/…）

## 四、适配实施记录

### 刀2 获取链（2026-11-15·§59.533）

- **HxptAdapter**（internal/adapter/adapter_hxpt.go）：framework=hxpt·API-only（回退阀=sites.framework 应急回 nexusphp）
- **factory 注册** + migration169（framework 刷 hxpt + publish_form_config.enabled→false 临时关闭——旧值集全错位防误发·刀4 重造后 re-enable）
- **双 JSON 同步**（supported_sites.json + data/sites.json framework=hxpt）
- **端到端七链验证**（§59.338 离线驱动·真实站点数据）：搜索 8 条/详情双端点合并/HR 96h/折扣 NONE/stats ratio 124.51/下载真 bencode 7657B/VerifyExists 404→false ✓
- 测试锚八枚（adapter_hxpt_test.go·消毒夹具 testdata/hxpt/ 实测形态）

### 刀3 发布链（2026-11-15·§59.533）

- **UploadTorrent**：FormData 契约全实施（恒发/条件发/恒不发三段）+CSRF+sections dict_id 数值化+tags 数组聚合+1002 校验错误透出
- **摸底定案**：sections=纯数字 dict_id（select 不包装）；四维全站统一（五分类对照）；detail 的 filetype/standard/year=站方自动分析维度
- 测试锚五枚（捕获型 RoundTripper 形态断言）——好学锚累计 13 枚
- 与刀4 合并发版（publish enabled=false 期间无用户可见变化）

### 刀4-5（待实施）

- 刀3：发布链 adapter（FormData 契约+CSRF+sections 组装）
- 刀4：值集键映射 form_config 重造+旧特化清理（hxptOnlyDocumentary/hxptDocumentaryPrefill）+re-enable
- 刀5：DryRun 借壳+首颗实测

## 五、行为注意

- **GetTorrentDetail 双端点合并**：/{id} 元数据 + /{id}/detail 内容——detail 失败时 meta 兜底（Title+InfoHash 过真空检查）
- **组名提取**：API 无独立组字段——标题尾 `-GROUP` 惯用形态解析（hxptGroupOfName）
- **促销形态未知**：DetectDiscount null→NONE 实证；非 null 宽容匹配+日志（首见修正）
- **上传权限**：site-profile 无权限字段——DryRun/首颗 403 自然暴露

---

## 历史存档：旧 NP 适配（v0.0.1308 §59.453·已作废）

> 站方改版前为全库最极端的 NP 槽位错位站。特化代码（hxptOnlyDocumentary 内容拦截+hxptDocumentaryPrefill 四槽注入）在刀4 清理，此处保留认知存档。

| NP 标准槽名 | 旧好学实际语义 |
|---|---|
| type | 教育阶段（401-406）+410 纪录片 |
| audiocodec_sel | **年级**（幼儿园~高三+其他 14 项） |
| processing_sel | **教材出版社版本**（人教版等 28 项） |
| codec_sel | **文件格式**（MKV/MP4/PDF 等 26 项） |
| source_sel | **发布年份**（2011-2026+2010 年前+Other） |
