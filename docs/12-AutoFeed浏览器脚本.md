# Auto Feed 站点适配规则深度分析

## 一、架构设计核心思想

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          架构设计核心思想                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌──────────────┐      ┌──────────────┐      ┌──────────────┐             │
│   │   源站点 A   │      │   源站点 B   │      │   源站点 C   │             │
│   │  (PTP/HDB)  │      │  (TTG/MTeam) │      │  (BLU/ACM)  │             │
│   └──────┬───────┘      └──────┬───────┘      └──────┬───────┘             │
│          │                     │                     │                     │
│          │ 不同DOM结构         │ 不同提取逻辑        │ 不同API格式          │
│          ▼                     ▼                     ▼                     │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │                    raw_info 统一数据结构                         │      │
│   │   { name, type, medium_sel, codec_sel, standard_sel, ... }      │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│                              │                                              │
│                              │ 统一格式                                      │
│                              ▼                                              │
│   ┌──────────────┐      ┌──────────────┐      ┌──────────────┐             │
│   │  目标站点 X   │      │  目标站点 Y   │      │  目标站点 Z   │             │
│   │  type_dict   │      │  type_dict   │      │  type_dict   │             │
│   │  {'电影':1}  │      │  {'电影':401}│      │  {'电影':'mo'}│             │
│   └──────────────┘      └──────────────┘      └──────────────┘             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**核心价值**: 一套数据结构适配100+站点，通过字典映射实现站点间差异转换。

---

## 二、统一数据结构 `raw_info`

### 2.1 完整字段定义 (lines 1539-1580)

```javascript
var raw_info = {
    // ========== 填充类信息 ==========
    'name': '',              // 主标题
    'small_descr': '',       // 副标题
    'url': '',               // IMDB 链接
    'dburl': '',             // 豆瓣链接
    'descr': '',             // 简介 (BBCode格式)
    'images': [],            // 截图数组
    
    // ========== 音乐站专用 ==========
    'log_info': '',          // Log 信息 (RED/OPS)
    'tracklist': '',         // 曲目列表
    'music_type': '',        // 音乐类型
    'music_media': '',       // 音乐媒介
    'music_name': '',        // 专辑名
    'music_author': '',      // 艺术家
    'edition_info': '',      // 版本信息
    
    // ========== 动漫站专用 ==========
    'animate_info': '',      // 动漫信息 (BYR/U2)
    'anidb': '',             // AniDB 链接
    'torrentName': '',       // 种子名称辅助字段
    
    // ========== 种子文件 ==========
    'torrent_name': '',      // 种子文件名
    'torrent_url': '',       // 种子下载链接
    
    // ========== 选择类信息 (核心映射字段) ==========
    'type': '',              // 类型: 电影/剧集/动漫/综艺/音乐/纪录/体育/软件/学习/书籍/MV
    'source_sel': '',        // 来源: 欧美/大陆/香港/台湾/日本/韩国/印度
    'standard_sel': '',      // 分辨率: 8K/4K/1080p/1080i/720p/SD
    'audiocodec_sel': '',    // 音频编码: DTS-HDMA/Atmos/TrueHD/DTS/AC3/AAC/Flac/APE
    'codec_sel': '',         // 视频编码: H264/H265/X264/X265/VC-1/MPEG-2/AV1
    'medium_sel': '',        // 媒介: UHD/Blu-ray/Remux/Encode/HDTV/WEB-DL/DVD/CD
    
    // ========== 元信息 ==========
    'origin_site': '',       // 源站点名称
    'origin_url': '',        // 源站点URL (用于跳转识别)
    'golden_torrent': false, // 金种子标记 (皮转柠檬)
    'mediainfo_cmct': '',    // 春天专用MediaInfo
    'imgs_cmct': '',         // 春天专用截图
    'full_mediainfo': '',    // 完整MediaInfo (铂金家/猫/春天)
    'subtitles': [],         // 字幕文件 (皮转海豹)
    'youtube_url': '',       // YouTube链接 (iTS发布)
    'ptp_poster': '',        // PTP海报 (iTS发布)
    'comparisons': '',       // 对比图 (海豹)
    'version_info': '',      // 版本信息 (海豹)
    'multi_mediainfo': '',   // 多MediaInfo (海豹)
    'labels': 0,             // 标签位图
    'disctributor': '',      // 发行商 (unit3d架构)
    'region': ''             // 地区码 (unit3d架构)
};
```

### 2.2 字段分类说明

| 类别 | 字段 | 用途 |
|------|------|------|
| **填充类** | name, small_descr, url, dburl, descr | 直接填充到表单输入框 |
| **选择类** | type, medium_sel, codec_sel, standard_sel, audiocodec_sel | 通过字典映射转换为站点ID |
| **音乐专用** | log_info, tracklist, music_type, music_author | 音乐站(RED/OPS)特殊处理 |
| **动漫专用** | animate_info, anidb | 动漫站(BYR/U2)特殊处理 |
| **元信息** | origin_site, origin_url | 用于跨页面状态保持 |

---

## 三、站点识别机制

### 3.1 源站点判断 (`judge_if_the_site_as_source`)

通过 URL 正则匹配返回不同模式代码：

```javascript
function judge_if_the_site_as_source() {
    // 返回值含义:
    // 0 - 上传页面 (目标站点)
    // 1 - 详情页面 (源站点)
    // 2 - HDCity 特殊上传页
    // 4 - KG 上传页
    // 5 - BTN 上传页
    // 6 - AvistaZ 系上传页
    // 7 - 本地服务
    
    if (site_url.match(/^https:\/\/karagarga.in\/upload\.php.*/)) {
        return 4;  // KG upload page
    }
    if (site_url.match(/^https:\/\/(broadcasthe.net|backup.landof.tv)\/upload.php.*/)) {
        return 5;  // BTN upload page
    }
    if (site_url.match(/^https?:\/\/.*\/.*(upload|create|offer|viewoffers).*?(php)?#separator#/i)) {
        return 0;  // Upload page (target site)
    }
    if (site_url.match(/^https:\/\/(www.)?(darkland.top|eiga.moi|...|reelflix.xyz)\/torrents\/\d+$/)){
        return 1;  // Detail page (source site) - Laravel架构站点
    }
    // ... 40+ URL patterns for different sites
}
```

### 3.2 国内站点判断 (`judge_if_the_site_in_domestic`)

```javascript
function judge_if_the_site_in_domestic() {
    var domain, reg, key;
    for (key in o_site_info){
        if (key != 'FRDS' && key != 'BYR' && key != 'U2'){  // 排除特殊站点
            domain = o_site_info[key].split('//')[1].replace('/', '');
            reg = new RegExp(domain, 'i');
            if (site_url.split('#separator#')[0].match(reg)){
                return 0;  // 国内站点
            }
        }
    }
    return 1;  // 国外站点
}
```

### 3.3 站点识别返回值对照表

| 返回值 | 含义 | 示例站点 |
|--------|------|----------|
| `0` | 上传页面 | 国内站点 upload.php |
| `1` | 详情页面 | PTP, HDB, TTG, MTeam 等 |
| `2` | HDCity 特殊 | hdcity.city/upload |
| `4` | KG 上传页 | karagarga.in/upload.php |
| `5` | BTN 上传页 | broadcasthe.net/upload.php |
| `6` | AvistaZ 系 | avistaz.to/upload/torrent |
| `7` | 本地服务 | IP:5678 |

---

## 四、站点映射字典体系

### 4.1 类型映射 `type_dict`

不同站点的类型ID完全不同，需要通过字典映射：

| 站点 | 电影 | 剧集 | 动漫 | 综艺 | 音乐 | 纪录 | 体育 | 软件 | 书籍 | MV |
|------|------|------|------|------|------|------|------|------|------|-----|
| PTer | 401 | 404 | 403 | 405 | 406 | 402 | 407 | 410 | 408 | 413 |
| CMCT | 501 | 502 | - | 505 | 508 | 503 | 506 | 509 | 509 | 507 |
| HDSky | 'mo' | 'tv' | 'an' | 'ot' | - | 'do' | 'sp' | - | - | 'mv' |
| CHDBits | 1 | 4 | 3 | 5 | 6 | 2 | 7 | - | - | 6 |
| BHD | 1 | 2 | 3 | 4 | 8 | 5 | - | - | - | - |
| TJUPT | 401 | 402 | 405 | 403 | 408 | 404 | 407 | 411 | 419 | 406 |

**代码示例**:

```javascript
// PTer站点
var type_dict = {'电影': 401, '剧集': 404, '动漫': 403, '综艺': 405, 
                 '音乐': 406, '纪录': 402, '体育': 407, '软件': 410, 
                 '学习': 411, '书籍': 408, 'MV': 413};

// HDSky站点 (字符串类型)
var type_dict = {'电影': 'mo', '剧集': 'tv', '动漫': 'an', 
                 '综艺': 'ot', '纪录': 'do', '体育': 'sp', 'MV': 'mv'};
```

### 4.2 分辨率映射 `standard_dict`

| 站点 | 8K | 4K | 1080p | 1080i | 720p | SD |
|------|-----|-------|-------|-------|------|-----|
| PTer | - | 1 | 2 | 3 | 4 | 5 |
| HDSky | 'r5' | 'r4' | 'r3' | 'r3' | 'r2' | 'r1' |
| CHDBits | 5 | 6 | 1 | 2 | 3 | 4 |
| BHD | - | 1 | 2 | 3 | 4 | 5 |
| MTeam | 需组合 | 需组合 | - | - | - | - |

**MTeam特殊处理** (React架构，需异步选择):

```javascript
async function runSequence(standard_sel, videoCodec, audioCodec, type_code) {
    await selectDropdownOption('standard', 0, standard_sel);
    await selectDropdownOption('videoCodec', 1, videoCodec);
    await selectDropdownOption('audioCodec', 2, audioCodec);
    await selectDropdownOption('category', 3, type_code);
}
```

### 4.3 视频编码映射 `codec_dict`

| 站点 | H264/X264 | H265/X265 | VC-1 | MPEG-2 | Xvid | AV1 |
|------|-----------|-----------|------|--------|------|-----|
| CHDBits | 1 | 2 | 5 | 4 | - | - |
| BHD | 1 | 2 | 5 | 6 | 7 | - |
| HDU | 1 | 2 | 4 | 3 | 5 | - |
| TCCF | 1 | 6 | 2 | 4 | 3 | - |

**代码示例**:

```javascript
// CHDBits站点
var codec_dict = { 'H264': 1, 'X265': 2, 'X264': 1, 'H265': 2, 
                   'VC-1': 5, 'MPEG-2': 4 };

// BHD站点
var codec_dict = {'H264': 1, 'X265': 2, 'X264': 3, 'H265': 4, 
                  'VC-1': 5, 'MPEG-2': 6, 'Xvid': 7, '': 8 };
```

### 4.4 音频编码映射 `audiocodec_dict`

| 站点 | DTS-HDMA | Atmos | TrueHD | DTS | AC3 | AAC | Flac | APE |
|------|----------|-------|--------|-----|-----|-----|------|-----|
| CHDBits | 3 | 4 | 4 | 1 | 2 | 9 | 6 | 7 |
| BHD | 10 | 17 | 11 | 3 | 12 | 6 | 1 | 2 |
| HDU | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |

### 4.5 媒介映射 `medium_dict`

| 站点 | UHD | Blu-ray | Remux | Encode | HDTV | WEB-DL | DVD | CD |
|------|-----|---------|-------|--------|------|--------|-----|-----|
| CHDBits | 2 | 1 | 3 | 4 | 5 | 6 | - | 7 |
| BHD | 1 | 3 | 5 | 6 | 7 | 12 | - | 9 |
| HDU | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |

---

## 五、数据流转完整过程

### 5.1 源站点提取 → raw_info

不同站点使用不同的DOM选择器，但结果统一存入raw_info：

```javascript
// 国内站点提取逻辑
if (origin_site == 'TTG' || origin_site == 'PuTao' || origin_site == 'OpenCD') {
    title = document.getElementsByTagName("h1")[0];
    descr = document.getElementById("kt_d");
} else if (origin_site == 'HUDBT') {
    title = document.getElementById('page-title');
    descr = document.getElementById("kdescr");
} else if (origin_site == 'BYR') {
    raw_info.name = $('.index:eq(0)').text();
} else {
    title = document.getElementById("top");
    descr = document.getElementById("kdescr");
}

// 最终都存入 raw_info.name, raw_info.descr
for (i = 0; i < title.childNodes.length; i++) {
    raw_info.name = raw_info.name + title.childNodes[i].textContent;
}
raw_info.descr = walkDOM(descr);
```

### 5.2 raw_info 智能补全 (`fill_raw_info`)

当某些字段缺失时，从其他字段智能推断：

```javascript
function fill_raw_info(raw_info, forward_site){
    // 从标题推断媒介
    if (raw_info.medium_sel == ''){
        raw_info.medium_sel = raw_info.name.medium_sel();
        if (!raw_info.medium_sel && raw_info.descr.match(/mpls/i)) {
            raw_info.medium_sel = 'Blu-ray';
        }
    }
    
    // 从标题推断编码
    if (raw_info.codec_sel == ''){
        raw_info.codec_sel = raw_info.name.codec_sel();
    }
    
    // 从MediaInfo推断分辨率
    if (raw_info.standard_sel == ''){
        raw_info.standard_sel = raw_info.name.standard_sel();
    }
    if (raw_info.standard_sel == '') {
        try {
            var height = raw_info.descr.match(/Height.*?:(.*?)pixels/i)[1].trim();
            if (height == '480' || height == '576') {
                raw_info.standard_sel = 'SD';
            } else if (height == '720') {
                raw_info.standard_sel = '720p';
            } else if (height == '1 080') {
                raw_info.standard_sel = '1080p';
                if (raw_info.descr.match(/Scan.*?type.*?(Interleaved|Interlaced)/i)) {
                    raw_info.standard_sel = '1080i';
                }
            } else if (height == '2 160') {
                raw_info.standard_sel = '4K';
            }
        } catch(err) {}
    }
    
    // 从简介推断编码
    if (!raw_info.codec_sel || forward_site == 'PTer') {
        if (raw_info.descr.match(/Writing library.*(x264|x265)/)) {
            raw_info.codec_sel = raw_info.descr.match(/Writing library.*(x264|x265)/)[1].toUpperCase();
        } else if (raw_info.descr.match(/Video[\s\S]*?Format.*?HEVC/i)){
            raw_info.codec_sel = 'H265';
        } else if (raw_info.descr.match(/Video[\s\S]*?Format.*?AVC/i)){
            raw_info.codec_sel = 'H264';
        }
    }
}
```

### 5.3 目标站点填充 (字典映射)

```javascript
// PTer站点填充示例
if (forward_site == 'PTer'){
    var type_dict = {'电影': 401, '剧集': 404, '动漫': 403, '综艺': 405, 
                     '音乐': 406, '纪录': 402, '体育': 407, '软件': 410, 
                     '学习': 411, '书籍': 408, 'MV': 413};
    if (type_dict.hasOwnProperty(raw_info.type)){
        var index = type_dict[raw_info.type];
        $('select[name="type"]').val(index);
    }
    
    var source_box = $('select[name=source_sel]');
    switch(raw_info.medium_sel){
        case 'UHD': source_box.val(1); break;
        case 'Blu-ray': source_box.val(2); break;
        case 'Remux': source_box.val(3); break;
        case 'HDTV': source_box.val(4); break;
        case 'WEB-DL': source_box.val(5); break;
        case 'Encode': source_box.val(6); break;
        case 'DVD': source_box.val(7);
    }
}

// CHDBits站点填充示例
else if (forward_site == 'CHDBits') {
    var type_dict = {'电影': 1, '剧集': 4, '动漫': 3, '综艺': 5, 
                     '音乐': 6, 'MV': 6, '纪录': 2, '体育': 7};
    var standard_dict = {'8K': 5, '4K': 6, '1080p': 1, '1080i': 2, 
                         '720p': 3, 'SD': 4, '': 4};
    var codec_dict = { 'H264': 1, 'X265': 2, 'X264': 1, 'H265': 2, 
                       'VC-1': 5, 'MPEG-2': 4 };
    var medium_dict = { 'UHD': 2, 'Blu-ray': 1, 'Encode': 4, 'HDTV': 5, 
                        'WEB-DL': 6, 'Remux': 3, 'CD': 7 };
    
    // 逐个映射填充
    if (type_dict.hasOwnProperty(raw_info.type)) {
        browsecat.options[type_dict[raw_info.type]].selected = true;
    }
    if (standard_dict.hasOwnProperty(raw_info.standard_sel)) {
        standard_box.options[standard_dict[raw_info.standard_sel]].selected = true;
    }
}
```

---

## 六、标签自动勾选规则

### 6.1 标签识别 (`get_label`)

从标题和简介中识别标签：

```javascript
var labels = {
    'gy': false,      // 国语 - 匹配 "国语/国配"
    'yy': false,      // 粤语 - 匹配 "粤语/粤配"
    'zz': false,      // 中字 - 匹配 "中字/简繁字幕"
    'diy': false,     // DIY - 匹配 "DIY" + 制作组
    'hdr10': false,   // HDR10
    'hdr10plus': false, // HDR10+
    'db': false,      // 豆瓣
    'yz': false,      // 英字
    'en': false,      // 英语音轨
    'yp': false,      // 原盘
    'complete': false // 完整
};
```

### 6.2 站点标签映射

每个站点有独立的标签ID映射：

```javascript
// HDU站点标签
case 'HDU':
    if (labels.gy){ check_label(document.getElementsByName('tags[4][]'), '5'); }
    if (labels.zz){ check_label(document.getElementsByName('tags[4][]'), '6'); }
    if (labels.diy){ check_label(document.getElementsByName('tags[4][]'), '4'); }
    if (labels.hdr10) { check_label(document.getElementsByName('tags[4][]'), '7'); }
    break;

// TJUPT站点标签
case 'TJUPT':
    if (labels.gy){ check_label(document.getElementsByName('tags[4][]'), '5'); }
    if (labels.yy){ check_label(document.getElementsByName('tags[4][]'), '10'); }
    if (labels.zz){ check_label(document.getElementsByName('tags[4][]'), '6'); }
    if (labels.diy){ check_label(document.getElementsByName('tags[4][]'), '4'); }
    if (labels.hdr10) { check_label(document.getElementsByName('tags[4][]'), '11'); }
    if (labels.hdr10plus) { check_label(document.getElementsByName('tags[4][]'), '20'); }
    break;

// PThome站点标签
case 'PThome':
    if (labels.gy){ check_label(document.getElementsByName('tags[]'), 'gy'); }
    if (labels.yy){ check_label(document.getElementsByName('tags[]'), 'yy'); }
    if (labels.zz){ check_label(document.getElementsByName('tags[]'), 'zz'); }
    if (labels.diy){ check_label(document.getElementsByName('tags[]'), 'diy'); }
    if (labels.hdr10) { check_label(document.getElementsByName('tags[]'), 'hdr10'); }
    break;
```

---

## 七、制作组识别规则

### 7.1 官组正则匹配 (`reg_team_name`)

```javascript
const reg_team_name = {
    'MTeam': /-(.*mteam|mpad|tnp|BMDru|MWEB)/i,
    'CMCT': /-(CMCT|cmctv)/i,
    'HDSky': /-(hds|.*@HDSky)/i,
    'CHDBits': /-(CHD|.*@CHDBits)|@CHDWEB/i,
    'OurBits': /(-Ao|-.*OurBits|-FLTTH|-IloveTV|OurTV|-IloveHD|OurPad|-MGs)$/i,
    'TTG': /-(WiKi|DoA|.*TTG|NGB|ARiN)/i,
    'HDChina': /-(HDC)/i,
    'PTer': /-(Pter|.*Pter)/i,
    'HDHome': /(-hdh|.*@HDHome)/i,
    'Audiences': /(-Audies|.*@Audies|-ADE|-ADWeb)/i,
    'FRDS': /-FRDS|@FRDS/i,
    'HDDolby': /-DBTV|-QHstudIo|Dream$/i,
    'PigGo': /PigoHD|PigoWeb|PiGoNF/i,
    'CarPt': /CarPT/i,
    'HDVideo': /(-HDVWEB|-HDVMV)/i,
    'HDfans': /HDFans/i,
    'WT-Sakura': /SakuraWEB|SakuraSUB|WScode/i,
    'HHClub': /HHWEB/i,
    'HaresClub': /Hares?WEB|HaresTV|DIY@Hares|-hares/i,
    'Panda': /AilMWeb|-PANDA|@Panda/i,
    'UBits': /@UBits|-UBits|-UBWEB/i,
    'PTCafe': /CafeWEB|CafeTV|DIY@PTCafe/i,
    '影': /Ying(WEB|DIY|TV|MV|MUSIC)?$/i,
    'DaJiao': /DJWEB|DJTV/i,
    'OKPT': /OK(WEB|Web)?$/i,
    'AGSV': /AGSV(PT|E|WEB|REMUX|Rip|TV|DIY|MUS)?$/i,
    'TJUPT': /TJUPT$/,
    'FileList': /Play(HD|SD|WEB|TV)$/i,
    'CrabPt': /XHBWeb$/i,
    '红叶': /(RLWEB|RLeaves|RLTV|-R²)$/i,
    'QingWa': /(FROG|FROGE|FROGWeb)$/i,
    'ZMPT': /ZmWeb|ZmPT/i,
    'ptsbao': /-(FFans|sBao|FHDMV|OPS)/i,
    '麒麟': /-HDK(WEB|TV|MV|Game|DIY|ylin)/i,
    '13City': /-(13City|.*13City)/i,
};
```

### 7.2 制作组选择 (`check_team`)

```javascript
function check_team(raw_info, s_name, forward_site) {
    // 特殊处理: MTeam转HDHome
    if (raw_info.name.match(/MTeam/) && forward_site == 'HDHome') {
        $(`select[name="team_sel"]>option:eq(11)`).attr('selected', true);
        return;
    }
    
    $(`select[name="${s_name}"]>option`).map(function(index, e){
        // 从标题中提取制作组名 (去掉年份后的部分)
        var name = raw_info.name.split(/(19|20)\d{2}/).pop();
        
        // 特殊站点处理
        if (forward_site == '慕雪阁') {
            name = raw_info.name.split(/Blu-ray/i).pop();
        }
        
        // 匹配制作组
        if (name.toLowerCase().match(e.innerText.toLowerCase())) {
            // 排除误判情况
            if ((name.match(/PSY|LCHD/) && e.innerText == 'CHD') || 
                (name.match(/PandaMoon/) && e.innerText == 'Panda') || 
                e.innerText == 'DIY' || e.innerText == 'REMUX') {
                return;  // 跳过误判
            } else if (name.match(/HDSpace/i) && e.innerText.match(/HDS/i)) {
                return;
            } else if (name.match(/HDClub/i) && e.innerText.match(/HDC/i)) {
                return;
            } else if (name.match(/REPACK/i) && e.innerText.match(/PACK/i)) {
                return;
            } else {
                $(`select[name^="${s_name}"]>option:eq(${index})`).attr('selected', true);
            }
        }
    });
}
```

### 7.3 官种感谢声明 (`add_thanks`)

```javascript
function add_thanks(descr) {
    const thanks_str = "[quote][b][color=blue]{site}官组作品，感谢原制作者发布。[/color][/b][/quote]\n\n{descr}";
    for (var key in reg_team_name) {
        if (raw_info.name.match(reg_team_name[key]) && 
            !raw_info.name.match(/PandaMoon|HDSpace|HDClub|LCHD/i)) {
            descr = thanks_str.format({'site': key, 'descr': descr});
        }
    }
    return descr;
}
```

---

## 八、特殊站点处理

### 8.1 MTeam (React架构)

MTeam使用React框架，需要模拟React事件：

```javascript
// React组件需要特殊事件触发
function setValue(input, value) {
    let lastValue = input.value;
    input.value = value;
    let tracker = input._valueTracker;
    if (tracker) {
        tracker.setValue(lastValue);
    }
    input.dispatchEvent(new Event("change", { bubbles: true, cancelable: false }));
}

// 异步下拉选择
async function selectDropdownOption(tid, index, targetTitle) {
    var clickEvent = document.createEvent('MouseEvents');
    clickEvent.initEvent('mousedown', true, true);
    document.getElementById(tid).dispatchEvent(clickEvent);
    
    await new Promise(resolve => setTimeout(resolve, 100));
    
    const listHolder = document.querySelectorAll('.rc-virtual-list-holder')[index];
    const option = listHolder.querySelector(`.ant-select-item-option[title="${targetTitle}"]`);
    if (option) {
        option.click();
    }
}

// MTeam类型组合逻辑
var type_code = '電影/HD';
switch (raw_info.type){
    case '电影':
        if (raw_info.medium_sel == 'Blu-ray' || raw_info.medium_sel == 'UHD'){
            type_code = '電影/Blu-Ray';
        } else if (raw_info.medium_sel == 'Remux'){
            type_code = '電影/Remux';
        } else if (raw_info.medium_sel == 'DVD' || raw_info.medium_sel == 'DVDRip'){
            type_code = '電影/DVDiSo';
        } else {
            type_code = raw_info.standard_sel != 'SD' ? '電影/HD' : '電影/SD';
        }
        break;
    case '剧集': case '综艺':
        if (raw_info.medium_sel == 'Blu-ray' || raw_info.medium_sel == 'UHD'){
            type_code = '影劇/綜藝/BD';
        } else if (raw_info.medium_sel == 'DVD' || raw_info.medium_sel == 'DVDRip'){
            type_code = '影劇/綜藝/DVDiSo';
        } else {
            type_code = raw_info.standard_sel != 'SD' ? '影劇/綜藝/HD' : '影劇/綜藝/SD';
        }
        break;
}
```

### 8.2 音乐站 (RED/OPS/lztr)

音乐站使用API获取JSON数据：

```javascript
// API获取种子信息
getJson(`https://redacted.sh/ajax.php?action=torrent&id=${torrent_id}`, null, function(data){
    raw_info.json = JSON.stringify(data);
    var group = data['response']['group'];
    var torrent = data['response']['torrent'];
    
    // 艺术家
    if (group.artists) {
        raw_info.music_author = Array.from(group.artists.map((e)=>{
            return e.name;
        })).join(' & ');
    }
    
    // 专辑名
    raw_info.music_name = group.name.replace(/&quot;/g, '');
    
    // 标签
    if (group.tags) {
        raw_info.music_type = group.tags.join(',');
    }
    
    // 副标题 (格式/编码/媒介/Log分数)
    raw_info.small_descr = torrent['format'] + ' / ' + torrent['encoding'] + ' / ' + torrent['media'];
    if (torrent.logScore !== undefined && torrent.logScore > 0) {
        raw_info.small_descr += ` / Log (${torrent.logScore}%)`
    }
    if (torrent.hasCue !== undefined && torrent.hasCue) {
        raw_info.small_descr += ` / Cue`
    }
});

// 音乐类型映射
var type_dict = {
    "electronic": "电子(Electronic)",
    "blues": "蓝调(Blues)",
    "classical": "古典(Classical)",
    "country": "乡村(Country)",
    "folk": "民间(Folk)",
    "jazz": "爵士(Jazz)",
    "new.age": "新世纪(NewAge)",
    "soul": "天籁(Soul)",
    "reggae": "雷鬼(Reggae)",
    "hip.hop": "嘻哈(Hip Hop)",
    "soundtrack": "原声(OST)",
    "japanese": "日韩", 
    "korean": "日韩",
    "chinese": "大陆",
    "english": "欧美",
};
```

### 8.3 Laravel架构站点 (BLU/ACM/Monika/Tik)

Laravel架构站点使用统一的选择器：

```javascript
// 统一的提取方式
var mediainfo = $('code[x-ref="mediainfo"]').text().trim();
if (!mediainfo) {
    mediainfo = $('code[x-ref="bdinfo"]').text().trim();
}

// 统一的填充方式
$('#autocat').val("1");  // 电影
$('#autocat').val("2");  // 剧集
$('#autoimdb').val(imdb_id);
$('#autores').val(resolution_id);
```

### 8.4 PTP (PassThePopcorn)

PTP有特殊的页面结构：

```javascript
// PTP特殊处理
function walk_ptp(n) {
    // 处理spoiler
    if (n.nodeName == 'DIV' && n.className == 'spoiler') {
        var head = n.querySelector('.spoiler_head');
        var body = n.querySelector('.spoiler_body');
        var title = head ? head.innerHTML : '';
        var content = body ? body.innerHTML : '';
        n.innerHTML = '[quote=' + title.trim() + ']' + content.trim() + '[/quote]';
    }
    
    // 处理comparison
    if (n.className == 'comparison') {
        // 特殊处理对比图
    }
}
```

---

## 九、架构优势总结

| 特性 | 说明 |
|------|------|
| **解耦** | 源站点提取与目标站点填充完全分离，互不影响 |
| **可扩展** | 新增站点只需添加映射字典，无需修改核心逻辑 |
| **智能推断** | `fill_raw_info` 自动从已有信息推断缺失字段 |
| **统一格式** | 所有站点使用相同的 `raw_info` 结构，便于维护 |
| **容错性强** | 字典映射使用 `hasOwnProperty` 检查，避免未定义错误 |

**核心公式**: `目标站点值 = dict[raw_info.字段]`

---

## 十、扩展新站点指南

### 10.1 添加新源站点

1. 在 `judge_if_the_site_as_source()` 添加URL识别规则
2. 在 `find_origin_site()` 添加域名映射
3. 在源站点提取逻辑中添加DOM选择器

### 10.2 添加新目标站点

1. 在 `used_site_info` 添加站点信息
2. 在目标站点填充逻辑中添加映射字典：
   - `type_dict` - 类型映射
   - `standard_dict` - 分辨率映射
   - `codec_dict` - 编码映射
   - `medium_dict` - 媒介映射
   - `audiocodec_dict` - 音频编码映射
3. 添加标签映射 (如有)
4. 添加制作组映射 (如有)

### 10.3 代码模板

```javascript
// 新目标站点模板
else if (forward_site == 'NewSite') {
    // 类型映射
    var type_dict = {'电影': 1, '剧集': 2, '动漫': 3, '综艺': 4, 
                     '音乐': 5, '纪录': 6, '体育': 7};
    if (type_dict.hasOwnProperty(raw_info.type)){
        $('select[name="type"]').val(type_dict[raw_info.type]);
    }
    
    // 分辨率映射
    var standard_dict = {'4K': 1, '1080p': 2, '1080i': 3, '720p': 4, 'SD': 5};
    if (standard_dict.hasOwnProperty(raw_info.standard_sel)){
        $('select[name="standard_sel"]').val(standard_dict[raw_info.standard_sel]);
    }
    
    // 编码映射
    var codec_dict = {'H264': 1, 'H265': 2, 'X264': 1, 'X265': 2};
    if (codec_dict.hasOwnProperty(raw_info.codec_sel)){
        $('select[name="codec_sel"]').val(codec_dict[raw_info.codec_sel]);
    }
    
    // 媒介映射
    var medium_dict = {'UHD': 1, 'Blu-ray': 2, 'Remux': 3, 'Encode': 4, 
                       'HDTV': 5, 'WEB-DL': 6};
    if (medium_dict.hasOwnProperty(raw_info.medium_sel)){
        $('select[name="medium_sel"]').val(medium_dict[raw_info.medium_sel]);
    }
    
    // 填充简介
    $('#descr').val(raw_info.descr);
    
    // 填充IMDB
    $('#url').val(raw_info.url);
}
```

---

## 十一、关键代码位置索引

| 功能 | 行号范围 |
|------|----------|
| `raw_info` 定义 | 1539-1580 |
| `reg_team_name` 制作组正则 | 1894-1937 |
| `judge_if_the_site_as_source()` | 2203-2323 |
| `judge_if_the_site_in_domestic()` | 2324-2336 |
| `fill_raw_info()` 智能补全 | 3052-3250 |
| `check_label()` 标签勾选 | 3255-3265 |
| `check_team()` 制作组选择 | 4767-4795 |
| 源站点提取逻辑 | 8960-9500 |
| 目标站点填充逻辑 | 16550-22000+ |
| 音乐站处理 | 14290-14450 |

---

*分析完成于 2026-04-11*
