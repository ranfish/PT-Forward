# 乌托邦 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 乌托邦（2026-11-05 由"音乐乌托邦"系统性改名·migration148） |
| 站点地址 | https://www.musopia.vip |
| 站点框架 | NexusPHP（音乐专站·纯音乐内容） |
| Tracker | `https://www.musopia.vip/announce.php`（**同域**·.torrent announce 铁证实证 2026-11-05；tracker_domains=["www.musopia.vip"] 配置正确） |
| 官组体系 | **有**——原抓组 `ZHENGLIFAN@Musopia`（@后缀形态）+制作组字段行（制作组: Musopia）——NP 音乐官组形态（织梦 ZmMusic 同款）；非海豚式无官组 |
| 特殊功能 | 保种区（🌱×N 已保护）·种子认领机制·歌词链接（/lyr） |
| 素材现状 | **29 做种池零种**（seeding/metadata 0 行——纯新增源·适配后从站内拉） |

---

## 一、获取侧摸底（2026-11-05·tid=10072/17109 实证）

### 详情页结构（NP 标准+音乐扩展字段行）

```
标题四段+后缀：艺人 - 专辑 年份 - FLAC 整分轨 - [后缀]
  后缀两形态：官组种=ZHENGLIFAN@Musopia｜转载种=来源站名（- Redacted）
字段行（rowhead 族）：
  副标题     双形态：官组种自由文本（"内嵌专辑封面"）/转载种规格斜杠链
            （"FLAC / Lossless / CD / Log (100%) / Cue"——海豚同款形态）
  标签       多维：首发｜官方｜地区（港台/大陆）｜流派（流行(Pop)·中英混合 rock）
  语言       华语/纯音乐
  基本信息   大小/类型: HQ Audio/媒介: CD（可选）/音频编码: FLAC/制作组: Musopia
  Log score  ★字段行直读（100）——三源最强 log 结构化
  "转载自：" ★字段（转载链结构化——OriginChain 最强源）
独立区：
  抓轨 Log（共 N 碟）——EAC/EZ CD 全文（第三种 log 位置形态实证：
    简介内嵌/MI 槽/独立专区——§九-c 命中）
  音轨(曲目)——曲目区+歌词链接
种名：[乌托邦].xxx (年份) [FLAC] {目录编号}——{CatNo} 花括号形态
```

### MusicMeta 映射评估（22 字段）

| 强项 | 源 | 备注 |
|---|---|---|
| **LogScore** | 字段行直读 | vs 海豚 API/皇后文本解析——NP 站最强 |
| **OriginChain** | "转载自："字段+标题来源后缀 | B4 挂账的最佳白捡站 |
| Tags | 多维标签行 | 首发/官方是乌托邦特色维 |
| TrackList/LogText | 独立区 | NP DOM 解析 |
| Label/CatNo | 种名 {} | 解析层 |
| Artist/Album/Year/Format/AudioMode | 标题四段 | 通用解析 |

ShareRule=**声明文本形态**（"音乐乌托邦原抓组ZHENGLIFAN作品，欢迎转载
各大PT，请在帖子顶部注明出处及作者，并保留后缀"）——欢迎+条件式；
**禁转变体待实证**（源站铁律·下轮补摸）。

### 三源对照（皇后/海豚/乌托邦）

| 维度 | 皇后 | 海豚 | 乌托邦 |
|---|---|---|---|
| 框架 | NP 定制 | Gazelle | NP |
| 官组 | OpenCD | 无 | ZHENGLIFAN@Musopia |
| LogScore | 文本解析 | API 直读 | **字段行直读** |
| 转载链 | 字段行(来自) | 盲区 | **转载自+标题后缀**（最强）|
| log 文本 | divnfo | 无 | 抓轨 Log 区 |
| 曲目 | divtracklist | fileList | 音轨(曲目)区 |

### 待摸遗留

- 禁转种实证（声明文本禁转变体——源站铁律）
- 类型枚举全集（HQ Audio 之外：LP/SACD…分类树）
- 发布侧（表单/值集——音乐目标站适配期）
- RSS/搜索全量形态（素材=0·适配时摸）
