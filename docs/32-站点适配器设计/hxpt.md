# 好学 站点适配器设计

## 站点信息

- **域名**: www.hxpt.org
- **架构**: NexusPHP（NP 同名槽语义全面错位站——全库最极端）
- **定位**: 学习资料资源站——普通影视资源不适用；**仅纪录片（410 特别通道）可发**（用户定案）

## 一、槽位语义错位表（适配核心认知）

| NP 标准槽名 | 好学实际语义 | 410 纪录片通道填法 |
|---|---|---|
| type | 教育阶段（401-406）+410 纪录片 | 410（唯一可发——hxptOnlyDocumentary 拦其余） |
| medium_sel | 内容形态（视频/音频/书籍...） | 视频(1)——全媒介键归一 |
| audiocodec_sel | **年级**（幼儿园~高三+其他 14 项） | 恒"其他"(14)——特化注入 |
| standard_sel | 分辨率（720p/1080p/4K/8K/Other） | 按实际（canonical 直配） |
| processing_sel | **教材出版社版本**（人教版等 28 项） | 恒"其他"(27)——特化注入 |
| codec_sel | **文件格式**（MKV/MP4/PDF 等 26 项） | MI 容器+标题词形→其它"其他" |
| source_sel | **发布年份**（2011-2026+2010 年前+Other） | tp.Year label 动态匹配（三级链） |
| team_sel | Other+HX/HXPT/HXWEB 官组 | [team.other] 兜底（非组员禁选官组） |

## 二、410 纪录片实态验证（tid=23917，2026-11-02）

```
类型=纪录片 / 媒介=视频 / 年级=其他 / 分辨率=1080p/1080i
版本=其他 / 格式=MKV / 制作组=Other
——与用户描述七槽完全一致（摸底实证）
```

## 三、发布适配记录（v0.0.1308 §59.453）

- **站点级拦截**：hxptOnlyDocumentary（碟粉模式——非 documentary/
  documentaries → ineligible"好学仅接受纪录片（410 特别通道）"+
  DryRun 同拦预览可见）。
- **hxptDocumentaryPrefill 四槽特化注入**（用户逐点讨论定案）：
  - 年级恒"其他"(14)（audiocodec 域键）
  - 版本恒"其他"(27)（直写 processing_sel[4]——无域常量）
  - 年份三级链：tp.Year 精确 label（站方加新年份自动适配）→
    **n≤2010 归"2010年前"(2)**（值集 2011 起有独立项无 2010——
    首版 n<2010 会 miss 裸对校准）→ 空不填（非必填不硬造）
  - 格式：MI General 容器（Matroska→MKV(13)/MPEG-4→MP4(4)）
    +标题词形兜底→一律"其他"(26)
- **MI 容器透出通用小增强**：ContainerFormat 字段三处
  （MediaInfoTech/ExtractMediaInfo/TechProfile+merge——§59.235
  局部变量升格）。
- type 410=[documentary,documentaries] 双键唯一配键；medium 全媒介
  键归一 1 视频；standard r1080p+i 双键归 3/p/i 合并项+sd→6；
  processing/source 槽 label 全录零配键（formdiff 零差异——
  htmlDomainRules 不含此二域）；tags 24（古籍 25 实时新增）：
  禁转/首发绝不配+DIY/国语/中字+HDR 泛四键+分集/完结+学科系
  label-only。
- 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
  standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
  technical_info+url(IMDb)+pt_gen+uplver；price（价格）/nfo 不配。
- **挂载点教训**：特化注入首版误挂 pubReq 段（DryRun return 后）——
  DryRun 看不到注入；修正=jobs 段后 DryRun 检查点前。
- 验证：裸对全槽（processing 同 label 克隆 14/20 删+source 1/2 写反
  修正）+formdiff 0 条+DryRun 四形态（拦截/主形态/2005+.mkv/
  MI Matroska）+48 包。

## 四、站规人工把关

- 纪录片特别通道规则（年级/版本恒其它——非纪录片禁止发布）
- HX/HXPT/HXWEB 官组非组员禁选（label-only 保护）
