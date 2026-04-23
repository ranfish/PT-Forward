# 肉丝 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 肉丝 |
| 站点地址 | https://rousi.pro |
| 站点框架 | **自研框架**（Vue.js SPA + REST JSON API，非 NexusPHP/UNIT3D/TNode） |
| 认证方式 | **Passkey Bearer Token**（无需 cookie） |
| 种子标识 | **UUID**（非数字 ID） |
| Tracker 域名 | rousipt（与主域名不同） |
| 特殊规则 | 无 cookie 认证；JSON API 发布；截图 base64 上传（最多6张）；BBCode→Markdown 转换；**禁止向 9KG 分类发布资源** |

---

## 一、发布 API 分析

**发布接口**: `POST /api/v1/torrents`（`application/json`）

**选项数据接口**: `GET /api/v1/categories`（JSON，含完整分类 + 属性 + 选项）

**认证方式**: `Authorization: Bearer {passkey}`（passkey 从 `https://rousi.pro/account?tab=passkey` 获取）

### 1.1 请求头

```
Content-Type: application/json
Authorization: Bearer {passkey}
Origin: https://rousi.pro
Referer: https://rousi.pro/
User-Agent: {标准浏览器 UA}
```

### 1.2 请求体（JSON）

```json
{
  "torrent": "{base64编码的.torrent文件}",
  "title": "标题",
  "description": "Markdown描述（从BBCode转换）",
  "subtitle": "副标题",
  "category": "movie",
  "anonymous": false,
  "media_info": "MediaInfo文本",
  "images": ["data:image/jpeg;base64,..."],
  "attributes": {
    "genre": ["剧情", "动作"],
    "resolution": "4K / 2160p",
    "region": "大陆",
    "source": "UHD Blu-ray",
    "tmdb": "tmdb_id_or_url",
    "imdb": "imdb_id_or_url",
    "douban": "douban_url"
  }
}
```

### 1.3 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `torrent` | string | ✓ | Base64 编码的 .torrent 文件 |
| `title` | string | ✓ | 标题（0day 命名法） |
| `category` | string | ✓ | 分类标识（如 "movie"、"tv"） |
| `description` | string | - | Markdown 格式描述（原 BBCode 需转换） |
| `subtitle` | string | - | 副标题 |
| `anonymous` | boolean | - | 匿名发布 |
| `media_info` | string | - | MediaInfo 文本 |
| `images` | string[] | - | 截图（JPEG base64 data URL，最多6张，单张≤5MB，总计≤20MB） |
| `attributes` | object | - | 分类相关属性（genre/resolution/region/source/tmdb/imdb/douban） |

注意：
- **无音频编码字段**——与 TNode（朱雀）类似。
- **无制作组下拉**——靠标题中的小组名标识。
- **截图以 base64 data URL 上传**——非外链图床 URL，需下载图片转 JPEG 并 base64 编码。
- 种子文件以 **Base64 字符串**提交，非 multipart file。
- 描述使用 **Markdown**，非 BBCode。

### 1.4 响应格式

成功：
```json
{ "code": 0, "message": "success", "data": { "UUID": "...", "status": "..." } }
```

详情页 URL：`/torrent/{UUID}`
下载 URL：`/api/torrent/{UUID}/download/{passkey}`

---

## 二、分类体系（11个分类）

> 数据来源：`GET /api/v1/categories`

肉丝使用**分类 + 动态属性**体系，不同分类有不同的属性字段。

### 2.1 分类列表

| ID | name | 显示名称 | 属性数 |
|----|------|----------|--------|
| 1 | movie | 电影 | 7（genre×29 + region×18 + resolution×6 + source×7 + imdb/tmdb/douban） |
| 2 | tv | 电视剧 | 7（genre×21 + region×18 + resolution×6 + source×7 + imdb/tmdb/douban） |
| 3 | documentary | 纪录片 | 7（genre×13 + region×10 + resolution×6 + source×7 + imdb/tmdb/douban） |
| 4 | animation | 动漫 | 8（genre×25 + region×14 + resolution×5 + source×7 + bangumi/imdb/tmdb/douban） |
| 5 | music | 音乐 | 2（genre×16 + format×7） |
| 6 | variety | 综艺 | 6（genre×12 + region×8 + resolution×6 + source×4 + imdb/douban） |
| 7 | sports | 体育 | 4（genre×13 + resolution×6 + source×3 + imdb/douban） |
| 8 | software | 软件 | 2（platform×7 + genre×9） |
| 9 | ebook | 电子书 | 2（genre×15 + format×6） |
| 10 | other | 其它 | 0 |
| 11 | 9kg | 9KG | 4（genre×7 + themes×32 + behaviors×34 + resolution×6 + source×17） |

### 2.2 分辨率（resolution）— 多数分类共用

| 值 | 说明 |
|----|------|
| 4K / 2160p | 4K |
| 1080p | 1080p |
| 1080i | 1080i |
| 720p | 720p |
| SD | 标清 |
| 其它 | 其他 |

动漫分类无 1080i 选项。使用**字符串值**（非数字 ID）。

### 2.3 来源（source）— 按分类不同

**电影/电视剧/纪录片/动漫**（7个）：

| 值 | 说明 |
|----|------|
| Blu-ray | 蓝光 |
| UHD Blu-ray | 超高清蓝光 |
| WEB-DL | 网页下载 |
| HDTV | 高清电视 |
| DVDRip | DVD 翻录 |
| CAM | 枪版 |
| 其它 | 其他 |

**综艺**（4个）：WEB-DL、HDTV、DVDRip、其它

**体育**（3个）：WEB-DL、HDTV、其它

**9KG**（17个）：Blu-Ray、UHD Blu-Ray、Remux、Web-DL、Webrip、母带流出、AI破解、Onlyfans、ManyVids、Fansly、fantia、Patreon、Pornhub、电报、推特/X、转载、其他

### 2.4 地区（region）— 按分类不同

**电影/电视剧**（18个）：大陆、香港、台湾、日本、韩国、美国、英国、法国、德国、意大利、西班牙、俄罗斯、新西兰、加拿大、印度、泰国、澳大利亚、其它

**纪录片**（10个）：大陆、香港、台湾、日本、韩国、美国、英国、法国、德国、其它

**动漫**（14个）：大陆、香港、台湾、日本、韩国、美国、英国、法国、德国、意大利、西班牙、俄罗斯、印度、泰国

**综艺**（8个）：大陆、香港、台湾、日本、韩国、美国、英国、其它

### 2.5 类型（genre）— 多选，按分类不同

**电影**（29个）：剧情、喜剧、动作、爱情、科幻、悬疑、惊悚、恐怖、犯罪、动画、奇幻、冒险、灾难、战争、传记、历史、运动、音乐、歌舞、家庭、儿童、纪录、短片、真人秀、脱口秀、西部、武侠、古装、其它

**电视剧**（21个）：剧情、喜剧、动作、爱情、科幻、悬疑、惊悚、恐怖、犯罪、动画、奇幻、冒险、战争、历史、家庭、儿童、纪录、真人秀、武侠、古装、都市

**动漫**（25个）：剧情、动画、热血、冒险、搞笑、恋爱、爱情、同性、校园、后宫、百合、治愈、萌系、悬疑、科幻、机战、奇幻、战斗、运动、竞技、历史、社会、恐怖、致郁、其它

**纪录片**（13个）：自然、历史、科技、人文、社会、传记、探险、美食、旅行、体育、音乐、艺术、其它

**音乐**（16个）：流行、摇滚、电子、古典、爵士、蓝调、乡村、民谣、说唱、R&B、金属、朋克、新世纪、原声、世界音乐、其它

**综艺**（12个）：真人秀、脱口秀、选秀、访谈、音乐、喜剧、游戏、美食、旅行、情感、亲子、其它

**体育**（13个）：足球、篮球、网球、F1、WWE、UFC、拳击、高尔夫、棒球、冰球、橄榄球、电竞、其它

**软件**（9个）：系统工具、办公软件、图形设计、影音处理、开发工具、网络工具、安全软件、游戏、其它

**电子书**（15个）：小说、文学、历史、哲学、经济、管理、心理、科技、计算机、教育、艺术、生活、漫画、杂志、其它

### 2.6 特殊属性

**音乐分类**——格式（format，7个）：FLAC、APE、WAV、DSD、MP3、AAC、其它

**电子书分类**——格式（format，6个）：PDF、EPUB、MOBI、AZW3、TXT、其它

**软件分类**——平台（platform，7个）：Windows、macOS、Linux、Android、iOS、跨平台、其它

**动漫分类**——额外支持 **Bangumi**（ptgen 类型）

### 2.7 元数据属性（ptgen 类型）

| 字段 | 适用分类 | 类型 |
|------|----------|------|
| imdb | 电影/电视剧/纪录片/综艺/体育 | ptgen |
| tmdb | 电影/电视剧/纪录片/动漫 | ptgen |
| douban | 电影/电视剧/纪录片/动漫/综艺/体育 | ptgen |
| bangumi | 仅动漫 | ptgen |

---

## 三、缺失字段（对比 NexusPHP）

| 字段 | 说明 |
|------|------|
| `codec_sel` | 无视频编码下拉（靠标题推断） |
| `audiocodec_sel` | 无音频编码字段 |
| `team_sel` | 无制作组下拉 |
| `processing_sel` | 无处理方式字段 |
| 标签 checkbox | 无传统标签（通过 genre 多选替代） |
| `pt_gen` | 无 PT-Gen（通过 ptgen 类型属性替代：imdb/tmdb/douban/bangumi） |

---

## 四、与其他框架对比

| 特征 | NexusPHP | 肉丝自研框架 |
|------|----------|-------------|
| 发布方式 | HTML 表单 POST | REST JSON API |
| 认证 | Cookie | Bearer Token（passkey） |
| 种子文件 | multipart file upload | Base64 字符串 |
| 截图 | 外链 URL | base64 data URL |
| 描述格式 | BBCode | Markdown |
| 种子标识 | 数字 ID | UUID |
| 选项值类型 | 数字 ID | 字符串（如 "movie"、"4K / 2160p"） |
| 分类属性 | 固定字段 | 按分类动态变化 |
| 类型标签 | 独立 tags 字段 | 嵌入 attributes.genre |

---

## 五、适配器设计注意事项

### 5.1 认证流程

1. 获取 passkey（用户从 `https://rousi.pro/account?tab=passkey` 手动复制）
2. 请求头携带 `Authorization: Bearer {passkey}`
3. **无需 cookie、无需 CSRF token**

### 5.2 字段映射特点

| 特点 | 说明 |
|------|------|
| 字符串值 | 分类用 "movie" 而非数字，分辨率用 "4K / 2160p" |
| 动态属性 | 不同 category 有不同的 attributes，需按分类加载 |
| genre 多选 | 类型以数组形式提交，非 checkbox |
| Base64 种子 | .torrent 文件需 Base64 编码后放入 JSON |
| Base64 截图 | 截图需转 JPEG→base64 data URL（ffmpeg 辅助） |
| Markdown 描述 | BBCode→Markdown 转换（来源站通常是 BBCode） |
| 无视频编码 | 需从标题推断 source（Blu-ray/WEB-DL/HDTV 等） |

### 5.3 来源推断

PTNexus 实现中使用正则从标题推断 source：
- 标题含 "UHD Blu-ray" → source = "UHD Blu-ray"
- 标题含 "WEB-DL"/"WEBRip" → source = "WEB-DL"
- 标题含 "HDTV" → source = "HDTV"
- 标题含 "DVDRip"/"DVD" → source = "DVDRip"
- 标题含 "Blu-ray" → source = "Blu-ray"

### 5.4 PTNexus 参考实现

- 发布器：`examples/PTNexus/server/internal/service/publish/publisher/sites/rousi.go`（1506 行）
- 配置文件：`examples/PTNexus/server/configs/rousi.yaml`（230 行）
- 站点数据：`examples/PTNexus/server/sites_data.json`

---

## 六、9KG 专区

> ⛔ **全局禁止（§30.5）**：**禁止向肉丝 9KG 分类发布任何资源。**
> category = "9kg"（id=11）被全局策略屏蔽。
> YAML 配置中 `skip_categories: ["11"]`，MappingResolver 在映射阶段前置拦截。
> 启动时由 RequiredSkips 审计该配置存在性，缺失则拒绝启动。

9KG 分类有独立的属性体系和标题规范（仅供逆向参考，代码层面不可触及）：

### 6.1 属性体系

- **类型**（genre，单选，7个）：日本有码、日本无码、FC2、订阅平台、欧美无码、HACG、写真
- **主题**（themes，多选，32个）
- **行为**（behaviors，多选，34个）
- **分辨率**（6个）
- **来源**（17个，含 Onlyfans/Fansly/fantia/Patreon/Pornhub/电报/推特/X 等平台）

### 6.2 9KG 标题规范（参考）

> 来源：`https://rousi.pro/wiki/9kg-specifications`

**总则**：
- 严禁发布任何形式的**幼女/恐虐**相关视频，违者连坐封号
- 除资源原标题自带的【】外，主标题/副标题中禁止使用括号
- 9KG 区暂时不接受任何合集类资源

**日本有码**：主标题 `品番 影片完整日文名称`，副标题 `[AI破解] 附加信息`
- 也可用：`品番 年份 分辨率 来源 视频编码 音频编码-压制组`
- 番号统一大写+连字符：`MUKC-110`，不使用空格或直接拼接

**日本无码**：主标题 `厂商 品番 影片完整日文名称`

**FC2 类**：主标题 `FC2-PPV-XXXXXXX 日文名称`，副标题 `[有码/无码/AI破解] 卖家ID 附加信息`

**订阅制平台**：主标题 `[平台] 作者ID 日期YY/MM/DD 标题`，平台全小写
- 包括：onlyfans、manyvids、fansly、糖心vlog、pornhub、X 等

**欧美成人**：主标题 `[厂商] 标题`，副标题 `演员 附加信息`

**HACG 类**（里番/3D动画/本子/ASMR/黄油）：各有独立格式

**写真类**：主标题 `[R16/R18] 作者ID 标题/主题`

---

## 七、发布规则要点

> 来源：`https://rousi.pro/wiki/user-rules-and-usage-guide` 和 `https://rousi.pro/wiki/title-specifications`

### 7.1 禁止上传的内容

- 当地法律法规明确禁止的内容
- 含病毒、木马、恶意脚本的文件
- 虚假、欺骗性资源
- 重复资源（无合理补充说明）
- 明显灌水、测试、无意义资源
- 禁止拆分无意义的小体积种子
- 文件列表禁止出现 URL、快捷方式、二维码等外站链接/广告

### 7.2 发布须知

- 资源必须完整，可正常校验
- 发布后需履行基本做种义务，**禁止即发即跑**
- 转载/搬运种子，如原种标题中未声明出处，请在简介中标明来源
- 游戏类资源发布前需自行下载运行确保可正常运行；如破解需用户自行操作，请在简介中写明操作步骤或附带操作文档

### 7.3 标题规范

- 主标题**不要添加中文**，中文电影/电视剧使用官方英文名称。
- 除教育类、软件类、游戏类资源，**未经允许禁止在主标题中使用任何括号**。
- 不要添加文件格式字样（.MKV、.mp3、.exe 等）。
- 除视频参数（编码、音频格式等）外，主标题**勿使用特殊符号**（如 !;.#$%【】）。
- 转种时如原标题用 `.` 作分隔符，应替换为空格（H.264、5.1 等参数除外）。
- 禁止在标题/副标题/内容中添加与资源无关的信息（外站链接、广告等）。
- [非强制] 主标题需在视频标题后添加年份。

#### 标题格式示例

**电影**：
- 主标题：`Avengers Infinity War 2018 BluRay 1080p H.264 DTS-CMCT`
- 副标题：`复仇者联盟3：无限战争[中英字幕]`

**电视剧**：
- 主标题：`Best Choice Ever 2024 S01E11-E12 1080p WEB-DL H.264 AAC 2.0-QHstudIo`
- 副标题：`承欢记 第1季第11-12集`

**动漫**：
- 主标题：`Kinsou no Vermeil S01 2022 1080p Blu-ray Remux AVC FLAC-7³ACG@OurBits`
- 副标题：`金装的维尔梅 第1季01-12 日语+簡繁字幕`

**音乐**：
- 主标题：`Kidney - Better Late Than Never 2014 FLAC 分轨`
- 副标题：`腰乐队 相见恨晚 专辑`

**游戏**：
- 主标题：`[Windows] [RPG] Pacific Drive-RUNE`
- 副标题：`[简中] 超自然车旅 版本 v1.1.1 2024`
- 如已破解开包即用，可添加 全DLC、全解锁、绿色版 等关键字

**软件**：
- 主标题：`[安卓] Bilibili Global v3.20.4`
- 副标题：`Bilibili 国际版`

**其他**：
- 主标题：`[黑马程序员] [Python] 黑马Python+大数据14阶段`
- 副标题：`网络搬运 课程内容完整无加密`

### 7.4 大包规则

除以下情况外，原则上不接受大包，请拆分：
- 同系列电影/写真集（如钢铁侠1~3）
- 同一部电视剧/动漫（可多季多集）

### 7.5 分类与标签

- 全部资源**必填**分类和标签。
- 根据内容选择分类（电影/电视剧/纪录片/动漫等）。
- 根据内容选择类型、地区、分辨率、来源等标签（无对应标签选"其它"）。

### 7.6 链接（元数据）

- IMDb、TMDB、豆瓣：最好补全 3 个，**最少填 1 个**（优先豆瓣）。
- 解析 1 次链接即可，请检查描述不要多次重复解析。

### 7.7 截图要求

**视频资源**：
- 电影/电视剧/动漫：**必须上传 1 张官方封面（宣传图）+ 2 张内容截图**（多合一截图可 1 张），单集资源可附带视频截图便于预览。
- 纪录片/综艺/演唱会等：**必须上传 1 张官方封面（宣传图）**。

**非视频资源**：
- 漫画/电子书：至少 1 张封面图。
- 音乐：至少 1 张专辑封面。
- 其他：至少 1 张相应封面。

截图文件大小 < 5MB。

### 7.8 简介

- 全部资源**必填**简介。
- 简介只填与资源相关的文字信息，**勿填外站链接、图床链接**（图片请直接在编辑器内上传）。
- MediaInfo/BDInfo 填写到对应位置，不要放在简介中。
- 转载种子如原种未声明出处，请在简介中标明来源。
- 软件/游戏资源需在简介中附带**查毒链接**（推荐 virscan.org 或 virustotal.com）。

### 7.9 MediaInfo

- 视频类**必填**，合集填任意一个视频的 MediaInfo 即可。
- **必须在英文模式下获取**（中文模式输出信息系统无法解析）。

---

## 八、账号与社区规则

> 来源：`https://rousi.pro/wiki/user-rules-and-usage-guide`

### 8.1 新手考核（30 天）

新手考核自注册时立即开始，页面顶部显示剩余时间。以下条件**任意一项未达标即视为考核失败**，考核失败直接封禁：

- 下载量 ≥ 30 GB
- 上传量 ≥ 100 GB
- 魔力值 ≥ 2333.00
- 数据以站内统计为准（Wiki 可能滞后，以站内信通知为准）

**分享率考察**：当下载量 ≥ 20 GB 时触发——分享率 < 0.4 → 7 天观察期，观察期结束仍 < 0.4 → 封禁。

### 8.2 不活跃惩罚

| 条件 | 处理 |
|------|------|
| > 30 天未登录 | 邮件提醒 |
| > 60 天未登录 | 每日扣 5000 魔力值，扣至 0 封禁 |

**不活跃保护**：预付 150000 魔力值可保护 60 天不活跃而不受惩罚（约为正常惩罚成本的 50%）。具体操作在账户设置→危险操作中查看。

### 8.3 警告与处罚机制

触发警告后**立即执行**以下惩罚：
- ⚠️ 上传量**不计入**账户统计
- ⬇️ 下载倍率 **×4**

**考察期**：
| 警告次数 | 考察期 |
|----------|--------|
| 第一次 | 24 小时 |
| 第二次 | 12 小时 |
| 第三次 | 6 小时 |

**上传超速累计 3 次 → 直接封号**。首次封号可通过捐献方式解除封禁。

考察期内请勿再次违规，否则将升级处罚。

### 8.4 禁止事项

触发以下行为将进入考察期：
- 上传超速
- 恶意刷流量 / 刷魔力值
- 长时间占用连接但不做种
- 使用非白名单 BT 客户端
- 其他破坏站点公平性的行为

### 8.5 做种要求

每个已完成下载的种子需满足**至少一项**：
- 做种 ≥ 24 小时
- **或** 分享率 ≥ 1.0

未满足即视为 H&R。**当前暂无 HR 要求，但必须满足基本做种要求。**

H&R 处理（包括但不限于）：记录 H&R 次数、警告、扣除魔力值、限制下载权限、严重或多次违规者封禁账号。

### 8.6 下载规则

- 允许本地下载及使用 Seedbox 等辅助工具
- **严禁**使用网盘离线下载、代抓 BT、离线种子
- 禁止出借账号、分享下载链接/PassKey、非本人操作账号

### 8.7 上传限速与流量统计

| 用户类型 | 发种限速 | 流量统计 |
|----------|----------|----------|
| 普通用户发种 | 200 Mbps | ×2（家宽） |
| 盒子发种 | 600 Mbps | ×0.5 |
| 盒子上传 | 400 Mbps | ×0.5 |

用户可在用户详情页查看在线 BT 客户端，并手动申请登记为 Seedbox。

上传超速累计 **3 次直接封号**。

### 8.8 BT 客户端白名单

**允许**：
- qBittorrent 5.x（推荐）、4.x（推荐）
- Transmission 4.x、3.x、2.x
- µTorrent 3.5.x、3.6.x、2.2.x、2.0.x（不推荐，后续可能调整策略）

**禁止**：修改 User-Agent、魔改/伪装客户端、模拟 BT 行为。违规视为**严重作弊行为**。

客户端通过 Peer ID 与 User-Agent 识别。

### 8.9 多设备 / IP 使用规则

**允许**：
- 同一账号在多台本人设备上使用
- 本地设备与 Seedbox 同时在线
- 正常 IP 变化（家宽拨号、移动网络切换、IPv4/IPv6 共存）

**禁止**（视为严重违规或作弊，直接封禁且不接受申诉）：
- 多人共用同一账号
- 出租、出借账号
- 共享 PassKey
- 代下载、代做种
- 使用脚本或自动化方式模拟客户端行为

### 8.10 管理裁量权

管理组有权不经事先警告直接介入处理：
- 利用规则漏洞牟取不当利益
- 明显破坏站点公平性与正常秩序
- 自动化脚本、异常流量、异常行为
- 其他未明确列出但被认定为不合理的行为
