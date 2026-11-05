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
| 素材现状 | **PT2 做种 131 颗**（2026-11-05 transmission 实测·快照 6/6 命中——素材入口通；seeding_torrent_records 0 行=**误报已销**：该表为刷流/RSS 经手流水账非全量镜像（皇后/海豚同为 0 行·红豆饭 504 行全 imported 实证）；种名含中文方括号变体"\[1994\] 许景淳 - xxx\[台湾\]"——标题解析新形态） |

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

### 禁转规则实证（2026-11-05 四路摸底·源站铁律合规）

**结论：乌托邦无站级禁转体系**（转载文化开放）——四路实证：
rules.php 零转载条款/faq.php 零命中/tags.php 标签总表零禁转类/
详情页+站内搜索零"禁转"字样（唯一命中=误匹配）。

**ShareRule 两形态**（无禁转变体）：
- 官组种：声明文本（"欢迎转载各大PT+注明出处保留后缀"——欢迎+条件式）
- 转载种：**"转载自："=完整源种 URL 直链**（https://redacted.sh/torrents.
  php?id=…&torrentid=…——OriginChain 最强形态：直达源种页面，比站名/
  文本链精细一个量级）+"来源站点"字段（站名）

§59.369 附铁律合规记录：实证无禁转=无检可做（四路搜索过程即证据）；
适配时 ShareRule=声明文本判定。

**入站规则补全（用户发现·forums.php topicid=3）**："严禁转载其他PT站
禁转的资源"——乌托邦出站开放+**入站要求上游非禁转**：与我们系统级
R7（源站禁转必检）完全同构（站方背书）；且构成 **R7 禁转二级传递性**
（12.5-b 讨论点⑥）的具体案例——乌托邦转载种的"转载自 URL"指向源头
站，源头禁转约束传导（该 URL 兼具传递性检查线索价值）。

### adapter 音乐特化实施（2026-11-05·§59.469 附五）

**策略=enrich 模式**（`internal/adapter/adapter_utopia.go`）：基础版面=
标准 NP（Engine 提取照走），音乐字段后处理 enrichUtopiaMusic 挂
getTorrentDetailInner 两分支（内部域判定·非乌托邦零开销）。

- 标题双形态解析：三段（官组·中段粘连）/四段（转载·保种区尾巴剥+
  繁体分軌归一简体）；AudioMode 词表皇后同款词序
- 字段行双/单引号 class 兼容；基本信息嵌套 b 键值串（nbsp 跳过）
- 17 字段中 14 填：TrackList 不提（无结构化区·desc 原样）/
  OtherVersions 站方无此字段/HasCue=副标题规格链 grep
- 测试锚三枚：TestUtopiaOfficialMusicMeta（官组三段）/TestUtopiaTransfer
  MusicMeta（转载四段+Origin URL）/TestUtopiaDomain

### 待摸遗留

- ~~禁转种实证~~ ✓（2026-11-05 四路实证无禁转体系——上方节）
- 类型枚举全集（HQ Audio 之外：LP/SACD…分类树）
- 发布侧（表单/值集——音乐目标站适配期）
- RSS/搜索全量形态（素材=0·适配时摸）
