# auto_feed 站点映射参考

> 本文档从 auto_feed 油猴脚本源码中提取站点配置、字段映射、转种逻辑等关键信息，
> 为 PT-Forward 站点适配器设计提供参考。
>
> 源码来源：https://greasyfork.org/zh-CN/scripts/424132-auto-feed
> 本地文件：`examples/auto_feed/auto_feed.js`
> 分析时间：2026-04-16

---

## 一、站点配置表

### 1.1. default_site_info（141个站点）

| # | 站点ID | URL | 备注 |
|---|--------|-----|------|
| 1 | 13City | https://13city.org/ | |
| 2 | 1PTBA | https://1ptba.com/ | |
| 3 | 52PT | https://52pt.site/ | |
| 4 | ACM | https://eiga.moi/ | |
| 5 | AGSV | https://www.agsvpt.com/ | URL受保护，不自动更新 |
| 6 | AFUN | https://www.ptlover.cc/ | |
| 7 | Aither | https://aither.cc/ | |
| 8 | ALing | https://pt.aling.de/ | |
| 9 | ANT | https://anthelion.me/ | |
| 10 | Audiences | https://audiences.me/ | |
| 11 | avz | https://avistaz.to/ | |
| 12 | BaoZi | https://p.t-baozi.cc/ | |
| 13 | BHD | https://beyond-hd.me/ | |
| 14 | BLU | https://blutopia.cc/ | |
| 15 | BTN | https://broadcasthe.net/ | 备用域名 backup.landof.tv |
| 16 | BTSchool | https://pt.btschool.club/ | |
| 17 | BYR | https://byr.pt/ | |
| 18 | CarPt | https://carpt.net/ | |
| 19 | CDFile | https://pt.cdfile.org/ | |
| 20 | CG | http://cinemageddon.net/ | |
| 21 | CHDBits | https://ptchdbits.co/ | 备用域名 {region_code}.chddiy.xyz |
| 22 | CMCT | https://springsunday.net/ | |
| 23 | CNZ | https://cinemaz.to/ | |
| 24 | CrabPt | https://crabpt.vip/ | |
| 25 | CyanBug | https://cyanbug.net/ | |
| 26 | DarkLand | https://darkland.top/ | |
| 27 | DevTracker | https://www.devtracker.me/ | |
| 28 | DiscFan | https://discfan.net/ | |
| 29 | Dragon | https://www.dragonhd.xyz/ | |
| 30 | ECUST | https://pt.ecust.pp.ua/ | |
| 31 | FNP | https://fearnopeer.com/ | |
| 32 | FreeFarm | https://pt.0ff.cc/ | |
| 33 | GGPT | https://www.gamegamept.com/ | |
| 34 | GPW | https://greatposterwall.com/ | |
| 35 | GTK | https://pt.gtk.pw/ | |
| 36 | HaiDan | https://www.haidan.video/ | |
| 37 | HDArea | https://hdarea.club/ | |
| 38 | HDB | https://hdbits.org/ | |
| 39 | HDBAO | https://hdbao.cc/ | |
| 40 | HDCity | https://hdcity.city/ | |
| 41 | HDClone | https://pt.hdclone.top/ | |
| 42 | HDDolby | https://www.hddolby.com/ | |
| 43 | HDF | https://hdf.world/ | 法语界面汉化 |
| 44 | HDfans | http://hdfans.org/ | |
| 45 | HDHome | https://hdhome.org/ | |
| 46 | HD-Only | https://hd-only.org/ | |
| 47 | HDRoute | http://hdroute.org/ | |
| 48 | HDSky | https://hdsky.me/ | |
| 49 | HDSpace | https://hd-space.org/ | |
| 50 | HDT | https://hd-torrents.org/ | 备用域名 hdts.ru |
| 51 | HDTime | https://hdtime.org/ | |
| 52 | HDU | https://pt.upxin.net/ | |
| 53 | HDVideo | https://hdvideo.top/ | |
| 54 | HITPT | https://www.hitpt.com/ | |
| 55 | HUDBT | https://hudbt.hust.edu.cn/ | 教育网 |
| 56 | ICC | https://www.icc2022.com/ | |
| 57 | iloli | https://mua.xloli.cc/ | |
| 58 | iTS | https://shadowthein.net/ | |
| 59 | ITZMX | https://pt.itzmx.com/ | |
| 60 | JoyHD | https://www.joyhd.net/ | |
| 61 | KG | https://karagarga.in/ | |
| 62 | KuFei | https://kufei.org/ | |
| 63 | LaJiDui | https://pt.lajidui.top/ | |
| 64 | LongPT | https://longpt.org/ | |
| 65 | LuckPT | https://pt.luckpt.de/ | |
| 66 | MARCH | https://duckboobee.org/ | |
| 67 | Monika | https://monikadesign.uk/ | |
| 68 | MTeam | https://kp.m-team.cc/ | 备用域名 zp.m-team.io; URL受保护 |
| 69 | MTV | https://www.morethantv.me/ | |
| 70 | MyPT | https://cc.mypt.cc/ | |
| 71 | NanYang | https://nanyangpt.com/ | |
| 72 | NBL | https://nebulance.io/ | |
| 73 | NexusHD | https://www.nexushd.org/ | 备用域名 v6.nexushd.org |
| 74 | NJTUPT | https://njtupt.top/ | |
| 75 | NPUPT | https://npupt.com/ | |
| 76 | OKPT | https://www.okpt.net/ | |
| 77 | OnlyEncodes | https://onlyencodes.cc/ | |
| 78 | OpenCD | https://open.cd/ | |
| 79 | OPS | https://orpheus.network/ | |
| 80 | Oshen | http://www.oshen.win/ | |
| 81 | OurBits | https://ourbits.club/ | |
| 82 | Panda | https://pandapt.net/ | |
| 83 | PHD | https://privatehd.to/ | |
| 84 | PigGo | https://piggo.me/ | |
| 85 | PTCafe | https://ptcafe.club/ | |
| 86 | PTer | https://pterclub.net/ | |
| 87 | PTFans | https://ptfans.cc/ | |
| 88 | PThome | https://www.pthome.net/ | |
| 89 | PTLGS | https://ptlgs.org/ | |
| 90 | PTP | https://passthepopcorn.me/ | |
| 91 | PTSkit | https://www.ptskit.org/ | |
| 92 | PTsbao | https://ptsbao.club/ | |
| 93 | PTT | https://www.pttime.org/ | |
| 94 | PTzone | https://ptzone.xyz/ | |
| 95 | PuTao | https://pt.sjtu.edu.cn/ | 教育网 |
| 96 | QingWa | https://qingwapt.com/ | URL受保护，不自动更新 |
| 97 | RailgunPT | https://bilibili.download/ | |
| 98 | ReelFliX | https://reelflix.xyz/ | |
| 99 | RED | https://redacted.sh/ | |
| 100 | RouSi | https://rousi.pro/ | |
| 101 | RS | https://resource.xidian.edu.cn/ | 教育网 |
| 102 | SBPT | https://sbpt.link/ | |
| 103 | SC | https://secret-cinema.pw/ | |
| 104 | SoulVoice | https://pt.soulvoice.club/ | |
| 105 | TCCF | https://et8.org/ | |
| 106 | Tik | https://cinematik.net/ | |
| 107 | TJUPT | https://www.tjupt.org/ | |
| 108 | TLFbits | http://pt.eastgame.org/ | |
| 109 | Tokyo | https://www.tokyopt.xyz/ | |
| 110 | TTG | https://totheglory.im/ | |
| 111 | TVV | http://tv-vault.me/ | |
| 112 | UBits | https://ubits.club/ | |
| 113 | UltraHD | https://ultrahd.net/ | |
| 114 | WT-Sakura | https://wintersakura.net/ | |
| 115 | xthor | https://xthor.tk/ | |
| 116 | YDY | https://pt.hdbd.us/ | |
| 117 | YemaPT | https://www.yemapt.org/ | |
| 118 | YHPP | https://www.yhpp.cc/ | |
| 119 | ZHUQUE | https://zhuque.in/ | |
| 120 | ZMPT | https://zmpt.cc/ | |
| 121 | 52MOVIE | https://www.52movie.top/ | |
| 122 | 52PT | https://52pt.site/ | |
| 123 | 下水道 | https://sewerpt.com/ | |
| 124 | 天枢 | https://dubhe.site/ | |
| 125 | 好学 | https://www.hxpt.org/ | |
| 126 | 星陨阁 | https://pt.xingyungept.org/ | |
| 127 | 柠檬不甜 | https://lemonhd.net/ | |
| 128 | 未来幻境 | https://nex.jivon.de/ | |
| 129 | 杏林 | https://xingtan.one/ | |
| 130 | 藏宝阁 | https://cangbao.ge/ | |
| 131 | 唐门 | https://tmpt.top/ | |
| 132 | 墓雪阁 | https://pt.muxuege.org/ | |
| 133 | 我好闲 | http://whax.net/ | |
| 134 | 樱花 | http://pt.ying.us.kg/ | |
| 135 | 海棠 | https://www.htpt.cc/ | |
| 136 | 自然 | http://zrpt.cc/ | |
| 137 | 躺平 | https://www.tangpt.top/ | |
| 138 | 财神 | https://cspt.top/ | |
| 139 | 麒麟 | https://www.hdkyl.in/ | |
| 140 | 影 | https://star-space.net/ | |
| 141 | 雨 | https://raingfh.top/ | |
| 142 | 财神 | https://cspt.top/ | |
| 143 | NovaHD | https://pt.novahd.top/ | |

### 1.2. o_site_info（仅用于源站解析的额外站点）

以下站点不在 `default_site_info` 中，仅作为源站出现在 `o_site_info` 中：

| 站点ID | URL | 类型 |
|--------|-----|------|
| FRDS | https://pt.keepfrds.com/ | NexusPHP |
| TorrentLeech | https://www.torrentleech.org/ | 多域名 (.org/.me/.cc/tlgetin.cc) |
| FileList | https://filelist.io/ | NexusPHP |
| U2 | https://u2.dmhy.org/ | NexusPHP |
| jpop | https://jpopsuki.eu/ | Gazelle |
| HDOli | https://hd-olimpo.club/ | NexusPHP |
| IPT | https://iptorrents.com/ | NexusPHP |
| torrentseeds | https://torrentseeds.org/ | NexusPHP |
| IN | https://nzbs.in/ | NZB |
| HOU | https://house-of-usenet.com/ | NZB |
| OMG | https://omgwtfnzbs.org/ | NZB |
| digitalcore | https://digitalcore.club/ | NexusPHP |
| BlueBird | https://bluebird-hd.org/ | NexusPHP |
| bwtorrents | https://bwtorrents.tv/ | NexusPHP |
| lztr | https://lztr.me/ | Gazelle |
| DICMusic | https://dicmusic.com/ | Gazelle |
| bib | https://bibliotik.me/ | Gazelle |
| mam | https://www.myanonamouse.net | Gazelle |
| bit-hdtv | https://www.bit-hdtv.com/ | NexusPHP |
| SugoiMusic | https://sugoimusic.me/ | Gazelle |
| DTR | https://torrent.desi/ | UNIT3D |
| HONE | https://hawke.uno/ | UNIT3D |
| SpeedApp | https://speedapp.io/ | NexusPHP |
| HHClub | https://hhanclub.net/ | NexusPHP (多域名 hhan.club/hhanclub.net) |
| SportsCult | https://sportscult.org/ | NexusPHP |

### 1.3. 签到不支持站点列表

```javascript
var unsupported_sites = ['digitalcore', 'HD-Only', 'HOU', 'OMG', 'TorrentLeech', 'MTeam', 'UBits', 'PigGo'];
```

---

## 二、特殊域名处理

| 站点 | 默认URL | 备用URL | 触发条件 |
|------|---------|---------|----------|
| CHDBits | https://ptchdbits.co/ | https://{region_code}.chddiy.xyz/ | `chd_use_backup_url` 启用 |
| NexusHD | https://www.nexushd.org/ | https://v6.nexushd.org/ | `nhd_use_v6_url` 启用 |
| HDT | https://hd-torrents.org/ | https://hdts.ru/ | URL自动匹配 |
| MTeam | https://kp.m-team.cc/ | https://zp.m-team.io/ | 上传时使用备用域名 |
| BTN | https://broadcasthe.net/ | https://backup.landof.tv | 备用域名 |
| AGSV | - | - | URL受保护，用户自定义URL不被覆盖 |
| QingWa | - | - | URL受保护，用户自定义URL不被覆盖 |
| MTeam | - | - | URL受保护，用户自定义URL不被覆盖 |
| TorrentLeech | https://www.torrentleech.org/ | .org/.me/.cc/tlgetin.cc | `tldomain` 变量(0-3) |
| HHClub | https://hhanclub.net/ | https://hhan.club/ | URL自动匹配 |

---

## 三、源站解析逻辑

### 3.1. 国内站点判断

`judge_if_the_site_in_domestic()` 遍历 `o_site_info`，如果当前URL匹配其中的非 `FRDS`/`BYR`/`U2` 条目则返回0（国外站），否则返回1（国内站）。

### 3.2. 国内 NexusPHP 站点解析流程

1. **标题获取**：根据站点从 `h1`、`#top`、`#page-title`、`.index:eq(0)` 等元素提取
2. **简介获取**：从 `#kdescr` 元素通过 `walkDOM()` 转换为 BBCode
3. **IMDb/豆瓣链接**：从 `#kimdb`、`#kdouban` 等元素提取
4. **种子下载链接**：匹配 `download.php`、`种子链接`、`下载直链` 等模式
5. **MediaInfo**：从 `.codemain`、`.nexus-media-info-raw`、`#kmediainfo` 提取

**各站点标题提取方式**：

| 站点 | 标题元素 |
|------|----------|
| TTG/PuTao/OpenCD/HDArea | `document.getElementsByTagName("h1")[0]` |
| HUDBT | `document.getElementById('page-title')` |
| BYR | `$('.index:eq(0)').text()` |
| NPUPT | `document.getElementsByClassName('jtextfill')[0]` |
| 其他国内站 | `document.getElementById("top")` |

### 3.3. 国外站点解析

#### UNIT3D 框架站点（BLU, BHD, Tik, Aither, Monika, DarkLand, FNP, OnlyEncodes, ReelFliX）

- IMDb/TMDB/TVDB ID：从 `ul.meta__ids` HTML 提取
- 标题：从页面标题元素
- 简介：页面body通过 `walkDOM()` 转换
- 类型：从分类结构分析

#### Gazelle 框架站点（PTP, BTN, GPW, SC, HD-Only, NBL, TVV, MTV, RED, OPS, jpop, SugoiMusic）

- **PTP**：torrent_id从URL获取；IMDb从 `#imdb-title-link`；海报从 `.sidebar-cover-image`；高质量标记从 "High quality torrent"
- **BTN**：类型强制为 `剧集`；IMDb/TVDB异步获取
- **RED/OPS**：音乐站特殊字段 - `releasetype`, `music_type`, `music_media`, `tracklist`, `log_info`, `file_list`, `music_author`, `music_name`
- **jpop**：`edition_info` 从 `h2`；`music_author` 从艺术家链接

#### 特殊站点处理

| 站点 | 特殊处理 |
|------|----------|
| HDB | `hdb_hide_douban` 设置控制是否显示豆瓣信息 |
| BLU | 搜索页使用AJAX异步检索 |
| HHClub | 自定义 `get_next_text()` 辅助函数 |
| SportsCult | 类型强制为 `体育`，medium强制为 `WEB-DL` |
| MTeam/ZHUQUE/YemaPT | React/Ant Design 表单，使用 `setValue()` 和 `selectDropdownOption()` |

---

## 四、字段映射函数

以下函数定义在 `String.prototype` 上，用于从种子名称/描述中提取字段值。

### 4.1. medium_sel() — 媒介/来源类型

源码位置：auto_feed.js:2420

| 匹配规则 | 返回值 |
|----------|--------|
| `Webdl\|Web-dl\|WEB[\. ]` (排除 webrip) | `WEB-DL` |
| `UHDTV` | `UHDTV` |
| `HDTV` | `HDTV` |
| `Remux` (排除 Encode) | `Remux` |
| `Blu-ray\|.MPLS\|Bluray原盘` (排除 Encode) | `Blu-ray` |
| `UHD\|UltraHD` (排除 Encode) | `UHD` |
| `Encode\|BDRIP\|webrip\|BluRay` | `Encode` |
| `DVDRip\|DVD` | `DVD` |
| `TV` | `TV` |
| `VHS` | `VHS` |
| `格式: CD\|媒介: CD` | `CD` |

### 4.2. codec_sel() — 视频编码

源码位置：auto_feed.js:2450

| 匹配规则 | 返回值 |
|----------|--------|
| `H264\|H\.264\|AVC` | `H264` |
| `HEVC\|H265\|H\.265` | `H265` |
| `VVC\|H266\|H\.266` | `H266` |
| `X265` | `X265` |
| `X264` | `X264` |
| `VC-1` | `VC-1` |
| `MPEG-2` | `MPEG-2` |
| `MPEG-4` | `MPEG-4` |
| `XVID` | `XVID` |
| `VP9` | `VP9` |
| `DIVX` | `DIVX` |

### 4.3. audiocodec_sel() — 音频编码

源码位置：auto_feed.js:2480

| 匹配规则 | 返回值 |
|----------|--------|
| `DTS-HDMA:X 7\.1\|DTS.?X.?7\.1` | `DTS-HDMA:X 7.1` |
| `DTS-HD.?MA` | `DTS-HDMA` |
| `DTS-HD.?HR` | `DTS-HDHR` |
| `DTS-HD` | `DTS-HD` |
| `DTS.?X[^2]` | `DTS-X` |
| `LPCM` | `LPCM` |
| `OPUS` | `OPUS` |
| `[ \.]DD\|AC3\|AC-3\|Dolby Digital` | `AC3` |
| `Atmos` + `True.?HD` | `Atmos` |
| `AAC` | `AAC` |
| `TrueHD` | `TrueHD` |
| `DTS` | `DTS` |
| `Flac` | `Flac` |
| `APE` | `APE` |
| `MP3` | `MP3` |
| `WAV` | `WAV` |
| `OGG` | `OGG` |

### 4.4. standard_sel() — 分辨率

源码位置：auto_feed.js:2526

| 匹配规则 | 返回值 |
|----------|--------|
| `4320p\|8k` | `8K` |
| `2160p\|4k` | `4K` |
| `1440p` | `1440p` |
| `1080p\|2K` | `1080p` |
| `1080i` | `1080i` |
| `720p` | `720p` |
| `576[pi]\|480[pi]` | `SD` |

### 4.5. get_type() — 内容类型

源码位置：auto_feed.js:2550

| 匹配规则 | 返回值 |
|----------|--------|
| `Movie\|电影\|UHD原盘\|films\|電影\|剧场` | `电影` |
| `Animation\|动漫\|動畫\|动画\|Anime\|Cartoons?` | `动漫` |
| `TV.*Show\|综艺` | `综艺` |
| `Docu\|纪录\|Documentary` | `纪录` |
| `短剧` | `短剧` |
| `TV.*Series\|影劇\|剧\|TV-PACK\|TV-Episode\|TV` | `剧集` |
| `Music Videos\|音乐短片\|MV(演唱)` | `MV` |
| `有声小说\|Audio(有声)\|有声书\|有聲書` | `有声小说` |
| `Music\|音乐` | `音乐` |
| `Sport\|体育\|運動` | `体育` |
| `学习\|资料\|Study` | `学习` |
| `Software\|软件\|軟體` | `软件` |
| `Game\|游戏\|PC遊戲` | `游戏` |
| `eBook\|電子書\|电子书\|书籍\|book` | `书籍` |

### 4.6. source_sel() — 来源地区

源码位置：auto_feed.js:2586

| 匹配规则 | 返回值 |
|----------|--------|
| `大陆\|China\|中国\|CN\|chinese` | `大陆` |
| `HK&TW\|港台\|thai` | `港台` |
| `EU&US\|欧美\|US/EU\|英美` | `欧美` |
| `JP&KR\|日韩\|japanese\|korean` | `日韩` |
| `香港` | `香港` |
| `台湾` | `台湾` |
| `日本\|JP` | `日本` |
| `韩国\|KR` | `韩国` |
| `印度` | `印度` |

### 4.7. get_label() — 标签检测

源码位置：auto_feed.js:2614

返回对象包含布尔标志：
- `gy`：国语配音
- `yy`：粤语配音
- `zz`：中文字幕（简繁字幕）
- `yz`：英文字幕
- `diy`：DIY发布（匹配制作组名 + mpls）
- `yp`：原盘（DISC INFO 或 mpls 但非 DIY）
- `hdr10plus`：HDR10+
- `hdr10`：HDR10
- `hdr`：HDR
- `db`：Dolby Vision
- `en`：英文音轨

---

## 五、目标站点填充逻辑

### 5.1. 通用填充流程

1. `raw_info` 从 URL hash 反序列化（`stringToDict()`）
2. `fill_raw_info()` 规范化/补充字段
3. 设置匿名上传复选框
4. 获取种子文件并注入新 announce URL
5. 应用站点特定字段映射

### 5.2. fill_raw_info() 规范化逻辑

源码位置：auto_feed.js:3052

- 若 `small_descr` 为空，从描述中派生
- 根据描述关键词纠正类型（纪录片→`纪录`，动画→`动漫`）
- 从描述提取缺失的 IMDb/豆瓣 URL
- 从描述地区信息派生 `source_sel`
- 从种子名派生 `medium_sel`（回退到描述中检测 mpls）
- 从种子名派生 `codec_sel`（回退到描述中 `Writing library` 或 `Video Format`）
- 从种子名派生 `audiocodec_sel`
- 从种子名派生 `standard_sel`（回退到描述中 `Height` 像素值）
- 特殊覆盖：名含 "Remux" → medium=Remux；名含 "webrip" → medium=WEB-DL

### 5.3. 各站点 type_dict 映射示例

#### PTer（源码:16580）

```javascript
type_dict = {
    '电影': 401, '剧集': 404, '动漫': 403, '综艺': 405,
    '音乐': 406, '纪录': 402, '体育': 407, '软件': 410,
    '学习': 411, '书籍': 408, 'MV': 413
};
medium_dict = {'UHD': 1, 'Blu-ray': 2, 'Remux': 3, 'HDTV': 4, 'WEB-DL': 5, 'Encode': 6, 'DVD': 7};
source_dict = {'欧美': 4, '大陆': 1, '香港': 2, '台湾': 3, '日本': 6, '韩国': 5, '印度': 7};
audio_dict = {'Flac': 8, 'WAV': 9};
```

#### CMCT（源码:16788）

```javascript
type_dict = {
    '电影': 501, '剧集': 502, '综艺': 505, '音乐': 508,
    '纪录': 503, '有声小说': 510, '体育': 506, '软件': 509,
    '学习': 509, '': 509, 'MV': 507
};
medium_dict = {
    'UHD': 1, 'Blu-ray': 1, 'Remux': 4, 'HDTV': 5,
    'WEB-DL': 7, 'WEBRip': 8, 'Encode': 6, 'DVD': 3,
    'DVDRip': 10, 'CD': 11
};
codec_dict = {'H265/X265': 1, 'H264/X264': 2, 'VC-1': 3, 'MPEG-2': 4, 'AV1': 5};
audio_dict = {
    'DTS-HD*': 1, 'TrueHD/Atmos': 2, 'LPCM': 6, 'DTS': 3,
    'AC3': 4, 'DD+': 11, 'AAC': 5, 'Flac': 7, 'APE': 8, 'WAV': 9
};
standard_dict = {'4K': 1, '1080p': 2, '1080i': 3, '720p': 4, 'SD': 5};
source_dict = {'欧美': 4, '大陆': 1, '香港': 2, '台湾': 3, '日本': 5, '韩国': 6, '印度': 7, '泰国': 9};
```

#### HDCity（源码:16925）

```javascript
type_dict = {
    '电影': 'mo', '剧集': 'tv', '动漫': 'an', '综艺': 'ot',
    '纪录': 'do', '体育': 'sp', 'MV': 'mv'
};
```

#### ZHUQUE（源码:17024）

```javascript
type_dict = {
    '电影': '电影', '剧集': '剧集', '动漫': '动画', '综艺': '节目',
    '音乐': '其它', '纪录': '其它', '体育': '其它', '软件': '其它',
    '学习': '其它', '': '其它', 'MV': '其它', '书籍': '其它'
};
medium_dict = {
    'UHD': 'UHD Blu-ray', 'Blu-ray': 'Blu-ray',
    'Remux': 'Remux', 'HDTV': 'HDTV', 'WEB-DL': 'WEB-DL',
    'Encode': 'Encode', 'DVD': 'Other'
};
standard_dict = {'4K': '2160p', '1080p': '1080p', '1080i': '1080i', '720p': '720p', 'SD': 'Other'};
```

#### YemaPT（源码:17202）

```javascript
type_dict = {
    '电影': '电影', '剧集': '剧集', '动漫': '动漫', '综艺': '综艺',
    '纪录': '纪录片', '音乐': '音乐', 'MV': 'MV/演唱会'
};
medium_dict = {
    'UHD': 'Blu-rayUHD', 'Blu-ray': 'Blu-ray', 'Remux': 'Remux',
    'HDTV': 'HDTV/TV', 'WEB-DL': 'Web-dl', 'Encode': 'Encode', 'CD': 'CD'
};
standard_dict = {'4K': '2160p/4K', '1080p': '1080p', '1080i': '1080i', '720p': '720p', 'SD': 'SD'};
```

#### BHD UNIT3D（源码:19944）

```javascript
type_dict = {'电影': 1, '剧集': 2, '纪录': 2, '综艺': 2};
// autosource: Remux/UHD/Blu-ray/Encode → 1, DVD → 5, HDTV → 4, WEB-DL → 3
// autotype: 基于文件大小的复杂选择逻辑
//   4K+Remux → 4, 4K+Encode → 8, 1080p+Blu-ray → 5/6, 1080p+Remux → 7
// Edition: Collector→1, Director→2, Extended→3, Limited→4, Special→5,
//          Theatrical→6, Uncut→7, Unrated→8
// Tags: HDR10, HDR10P, DV, WEBDL, WEBRip, 4kRemaster, 2in1, Commentary, 3D
```

#### BLU/ACM/Monika/FNP/OnlyEncodes UNIT3D（源码:22330）

```javascript
// Category: '剧集'/'综艺'/'纪录' → 2 (含季集解析), else → 1
// 分辨率映射因站点而异:
// BLU: {'4K': 1, '1080p': 2, '1080i': 3, '720p': 5, 'SD': 0, '8K': 11}
// ACM: {'4K': 1, '1080p': 2, '1080i': 2, '720p': 3, 'SD': 4}
// Monika: {'4K': 2, '1080p': 3, '1080i': 4, '720p': 5, 'SD': 10, '8K': 1}
// FNP:  {'4K': 2, '1080p': 3, '1080i': 11, '720p': 5, 'SD': 10, '8K': 1}
```

### 5.4. 上传URL模式

源码位置：auto_feed.js:4156 (`set_jump_href`)

| 站点类型 | URL模式 |
|----------|---------|
| Monika | `upload/1` |
| HDCity/BHD/HDB | `upload` |
| BYR | `upload.php?type=408/401/405/402/404/410` (按类型) |
| ACM/BLU | `torrents/create?category_id=1` 或 `2` |
| avz/CNZ/PHD | `upload/movie` 或 `upload/tv` |
| Aither/FNP/OnlyEncodes/DarkLand/ReelFliX | `torrents/create?category_id={type_id}` |
| ZHUQUE | `torrent/upload` |
| YemaPT | `#/torrent/add?` |
| 默认 NexusPHP | `upload.php` |

---

## 六、组名提取算法

源码位置：auto_feed.js:385 `get_group_name(name, torrent_info)`

```
1. 移除方括号内容 [...]、web-dl、dts-hd、Blu-ray、MPEG-2、MPEG-4
2. 按 .mkv/.mp4/.iso/.avi/.ts/.m2ts/.flac/.flv/.mp4 分割取第一部分
3. 检查末尾已知组名：KJNU, tomorrow505, KG, BMDru, BobDobbs, Dusictv, AFKI
4. 否则按 '-' 分割取最后一段
5. 若最后段匹配编码关键词（AC3, DD, AAC, x264, x265 等）：
   - Scene发布 → 取第一段
   - 否则继续分割或返回 Null
6. 已知别名修正：
   - Z0N3 → D-Z0N3
   - AVC.ZONE → ZONE
   - CultFilms → CultFilms™
7. 过滤规则（返回 Null）：
   - 含 ™ 但非 CultFilms
   - 匹配 [_\.! ] 或 Extras
   - 长度为1
   - 纯数字
   - .nfo 后缀 → 去除
```

---

## 七、标签映射

### 7.1. PTP 标签字典

源码位置：auto_feed.js:25997

```javascript
tag_dict = {
    'Masters of Cinema': 'masters_of_cinema',
    'The Criterion Collection': 'the_criterion_collection',
    'Warner Archive Collection': 'warner_archive_collection',
    "Director's Cut": 'director_s_cut',
    'Extended Edition': 'extended_edition',
    'Rifftrax': 'rifftrax',
    'Theatrical Cut': 'theatrical_cut',
    'Uncut': 'uncut',
    'Unrated': 'unrated',
    '2-Disc Set': '2_disc_set',
    '2in1': '2_in_1',
    '2D/3D Edition': '2d_3d_edition',
    '3D Anaglyph': '3d_anaglyph',
    '3D Full SBS': '3d_full_sbs',
    '3D Half OU': '3d_half_ou',
    '3D Half SBS': '3d_half_sbs',
    '4K Restoration': '4k_restoration',
    '10-bit': '10_bit',
    'DTS:X': 'dts_x',
    'Dolby Atmos': 'dolby_atmos',
    'Dolby Vision': 'dolby_vision',
    'Dual Audio': 'dual_audio',
    'English Dub': 'english_dub',
    'Extras': 'extras',
    'HDR10': 'hdr10',
    'HDR10+': 'hdr10plus',
    'With Commentary': 'with_commentary',
}
```

### 7.2. ZHUQUE 标签

源码位置：auto_feed.js:17182

| 标签值 | 含义 | 对应标志 |
|--------|------|----------|
| 603 | 国语 | gy |
| 604 | 中字 | zz |
| 613 | HDR10/HDR10+ | hdr10/hdr10plus |
| 611 | Dolby Vision | db |
| 621 | 合集 | - |
| 622 | 单集 (E\d+) | - |
| 614 | 特效字幕 | - |

### 7.3. BHD 标签

HDR10, HDR10P, DV, WEBDL, WEBRip, 4kRemaster, 2in1, Commentary, 3D

### 7.4. OpenCD 音乐类型映射

源码位置：auto_feed.js:14297

```javascript
type_dict = {
    "electronic": "电子(Electronic)",
    "blues": "蓝调(Blues)",
    "classical": "古典(Classical)",
    "country": "乡村(Country)",
    "folk": "民间(Folk)",
    "drum.and.bass": "贝斯(Drum Bass)",
    "jazz": "爵士(Jazz)",
    "new.age": "新世纪(NewAge)",
    "soul": "天籁(Soul)",
    "reggae": "雷鬼(Reggae)",
    "hip.hop": "嘻哈(Hip Hop)",
    "soundtrack": "原声(OST)",
    "japanese": "日韩",
    "korean": "日韩",
    "chinese": "大陆",
}
```

### 7.5. RED 发布类型

源码位置：auto_feed.js:14733

```javascript
JSONReleaseTypes = {
    '1': 'Album', '3': 'Soundtrack', '5': 'EP',
    '6': 'Anthology', '7': 'Compilation', '9': 'Single',
    '11': 'Live album', '13': 'Remix', '14': 'Bootleg',
    '15': 'Interview', '16': 'Mixtape', '17': 'Sampler',
    '21': 'Unknown', '22': 'Demo', '23': 'DJ Mix',
    '24': 'Concert Recording'
}
```

---

## 八、转种URL构建逻辑

源码位置：auto_feed.js:4156 `set_jump_href(raw_info, mode)`

### 8.1. 上传模式（mode=1）

1. 将 `raw_info` 序列化为 base64（`dictToString()`）
2. 追加到目标站URL后以 `#separator#` 分隔
3. 构建所有已启用站点的转种链接列表
4. 渲染到页面上的转发按钮

### 8.2. 搜索模式（mode!=1）

使用 IMDb ID 或搜索名构建各站点的搜索URL，替换 `{imdbid}`、`{imdbno}`、`{search_name}` 占位符。

### 8.3. 链接更新

`rebuild_href(raw_info)` 在 raw_info 变更时更新所有转种链接。

---

## 九、豆瓣信息获取

源码位置：auto_feed.js:8510 `get_douban_info()`

### 9.1. 完整信息获取流程

`getInfo(doc, raw_info)` 函数（源码:8330）从豆瓣页面提取：

| 字段 | 提取方式 |
|------|----------|
| 标题 | `doc.title.replace(/\(豆瓣\)$/, '')` |
| 原标题 | `$('#content h1>span[property]', doc)` |
| 又名 | `$('#info span.pl:contains("又名")', doc)` 下一个兄弟节点 |
| 年份 | `$('#content>h1>span.year', doc)` |
| 地区 | `$('#info span.pl:contains("制片国家/地区")', doc)` |
| 类型 | `$('#info span[property="v:genre"]', doc)` |
| 语言 | `$('#info span.pl:contains("语言")', doc)` |
| 上映日期 | `$('#info span[property="v:initialReleaseDate"]', doc)` |
| 片长 | `$('span[property="v:runtime"]', doc)` |
| 豆瓣评分 | `$('#interest_sectl [property="v:average"]', doc)` |
| 评分人数 | `$('#interest_sectl [property="v:votes"]', doc)` |
| 海报 | `$('#mainpic img', doc)` 转换为 `img9.doubanio.com` URL |
| 简介 | `$('#link-report-intra>[property="v:summary"]', doc)` |
| 导演 | `$('#info span.pl:contains("导演")', doc)` |
| 编剧 | `$('#info span.pl:contains("编剧")', doc)` |
| 主演 | `$('#info span.pl:contains("主演")', doc)` |
| IMDb ID | `$('#info span.pl:contains("IMDb:")', doc)` |
| 获奖 | `getAwards()` 从 `/awards/` 页面 |
| 演职员 | `getCelebrities()` 从 `/celebrities/` 页面 |

### 9.2. 豆瓣信息格式化

`formatInfo(info)` 函数（源码:8415）将信息格式化为 BBCode：

```
◎译　　名　{translatedTitle} / {alsoKnownAsTitles}
◎片　　名　{originalTitle}
◎年　　代　{year}
◎产　　地　{regions}
◎类　　别　{genres}
◎语　　言　{languages}
◎上映日期　{releaseDates}
◎IMDb评分　{IMDbScore}/10 from {ratingCount} users
◎IMDb链接　https://www.imdb.com/title/tt{IMDbID}/
◎豆瓣评分　{doubanScore}/10 from {votes} users
◎豆瓣链接　https://movie.douban.com/subject/{doubanID}/
◎片　　长　{durations}
◎简　　介　{description}
◎获奖情况　{awards}
```

---

## 十、图片托管服务

### 10.1. 支持的图床

| 服务 | API URL | 用途 |
|------|---------|------|
| PTPImg | https://ptpimg.me/upload.php | 主要图床 |
| PixHost | https://pixhost.to/new-upload/ | 备用图床（豆瓣海报） |
| Catbox | https://catbox.moe/user/api.php | 无需API Key |
| ImgBB | https://api.imgbb.com/1/upload | 需API Key |
| FreeImage | https://freeimage.host/api/1/upload | 需API Key |
| Gifyu | https://gifyu.com/api/1/upload | 需API Key |

---

## 十一、API集成

### 11.1. TMDB API

- 基础URL: `https://api.themoviedb.org/3/`
- 端点: `/movie/{id}`, `/tv/{id}`, `/find/{imdb_id}`
- 参数: `api_key`, `append_to_response` (external_ids, credits, images, videos)

### 11.2. OMDB API

- 基础URL: `https://www.omdbapi.com/`
- 端点: `?apikey={key}&i={imdb_id}&plot=full&tomatoes=true&r=json`
- 返回: Metacritic评分、Rotten Tomatoes评分、导演、编剧、演员

### 11.3. IMDb评分

- URL: `http://p.media-imdb.com/static-content/documents/v1/title/tt{ID}/ratings%3Fjsonp=imdb.rating.run:imdb.api.title.ratings/data.json`

### 11.4. 豆瓣API

- 搜索: `https://m.douban.com/search/?query={imdb_id}&type=movie`
- 详情: `https://movie.douban.com/subject/{id}/`

---

## 十二、签到站点

源码位置：auto_feed.js:6738

### 12.1. attendance.php 签到站点

PThome, HDHome, HDDolby, Audiences, PTLGS, SoulVoice, OKPT, UltraHD, CarPt, ECUST, iloli, HDClone, HDVideo, HDTime, FreeFarm, HDfans, PTT, ZMPT, CrabPt, QingWa, ICC, 1PTBA, HDBAO, AFUN, 星陨阁, CyanBug, 杏林, 海棠, Panda, KuFei, PTCafe, GTK, HHClub, 麒麟, AGSV, Oshen, PTFans, PTzone, 雨, 唐门, 天枢, 财神, DevTracker, CDFile, 柠檬不甜, ALing, LongPT, BaoZi

### 12.2. 特殊签到方式

| 站点 | 签到方式 |
|------|----------|
| HDArea | POST `sign_in.php` with `action=sign_in` |
| PTer | AJAX `attendance-ajax.php` |
| HDU | POST `added.php` |
| CHDBits | 手动 `bakatest.php` |
| U2 | 手动 `showup.php` |
| HDSky | 手动签到 |
| TJUPT | 手动 `attendance.php` |
| 52PT | 手动 `bakatest.php` |
| WT-Sakura | 手动 `attendance.php` |
| OurBits | 手动 `attendance.php` |
| PigGo | 手动 `attendance.php` |
| OpenCD | 手动签到 |
| UBits | 手动签到 |

---

## 参考资料

- auto_feed 源码: `examples/auto_feed/auto_feed.js`
- Greasyfork: https://greasyfork.org/zh-CN/scripts/424132-auto-feed
- GitHub: https://github.com/tomorrow505/auto_feed_js
- PT-Forward 站点适配器设计: `docs/32-站点适配器设计/README.md`
- NexusPHP API 指南: `docs/26-NexusPHP-API完整指南.md`

---

*最后更新：2026-04-16*
