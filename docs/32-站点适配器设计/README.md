# 站点适配器设计文档

> 本目录包含各个 PT 站点的特异化适配器设计文档，每个站点独立一个文档，便于维护和扩展。

## 目录

| 站点 | 文档 | 状态 |
|------|------|------|
| 青蛙 | [qingwapt.md](./qingwapt.md) | ✅ 完成（**种审制·审核脚本完整逆向(qingwa-torrent-assistant v1.1.1 1886行/82KB→44+项校验规则)**·22音频含DTS:X/DDP/Atmos/AV3A/USAC/OPUS·官组FROG/FROGE/FROGWeb·音乐分类跳过校验·28+禁发制作组·**含完整转载自动填写优化方案**） |
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
| 不可说 | [springsunday.md](./springsunday.md) | ✅ 完成（**种审制·审核脚本完整逆向(SpringSunday-Torrent-Assistant v1.1.67 2727行/135KB→普通30+规则+种审60+规则)**·9地区·12+音频含OPUS/AV3A·官组CMCT/CMCTA/CMCTV·40+禁发组+7不受信组·白名单图床8个·截图分辨率验证·Encode编码限制(x265/2160p+x264/1080pSDR)·豆瓣分类+地区+年份+语言+动画5维交叉验证·**含完整转载自动填写优化方案**） |
| 不可羊 | [tjupt.md](./tjupt.md) | ✅ 完成 |
| 财神 | [cspt.md](./cspt.md) | ✅ 完成（**审核脚本完整逆向(CS-Torrent-Assistant-New v1.5.11 1615行/73KB→30+校验规则)**·**全部字段ID重编(媒介10-19/编码6-11/音频8-23/分辨率5-11)**·API直接审核模式(CSRF Token)·16音频含ALAC/M4A·2K/1440p独立·WiKi/MySiLU/HDS/CHD跨站制作组·禁止图床5个·儿童/喜剧标签-简介交叉验证·Quark/mp4/mkv不信任·**含完整转载自动填写优化方案**） |
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
| 末日 | [agsv.md](./agsv.md) | ✅ 完成（Cloudflare·种审制·**审核脚本完整逆向(Agsv-Torrent-Assistant v1.4.7 1480行/65KB→30+校验规则)**·27黑名单·双区域综合+学习·大包规则·ALAC/M4A·18分类含短剧/漫画/图片·官组AGSVPT/AGSVMUSIC/Hares/RL/BeiTai·方舟计划标签·冰种标签·Music/Audio/Game等7分类跳过校验·**含完整转载自动填写优化方案**） |
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
| 肉丝 | [rousi.md](./rousi.md) | ✅ 完成（**自研框架**·Vue SPA+REST JSON API·Passkey认证·UUID种子·Base64截图·Markdown描述·动态属性·11分类·9KG专区禁止） |
| 三月 | [duckboobee.md](./duckboobee.md) | ✅ 完成（user可发种·HEVC≠H.265双选项·8K/576P/480P分辨率·3MWEB官组·21标签含6音乐标签·dupe来源优先级·无音频编码） |
| 时光 | [hdtime.md](./hdtime.md) | ✅ 完成（Cloudflare·候选制·PT-Gen四来源·极简质量·无分辨率·AV1/VVC/AVS3·Audio Vivid） |
| 时间 | [pttime.md](./pttime.md) | ⛔ 禁止发布（PTT-NP框架·极简发布·无质量字段·标签字符串值·16分类·9KG双区·候选制） |
| 蝴蝶 | [hamster.md](./hamster.md) | ✅ 完成 |
| PT地带 | [ptzone.md](./ptzone.md) | ✅ 完成 |
| 幸运 | [luckpt.md](./luckpt.md) | ✅ 完成（Cloudflare·**LuckAudit预审核系统100分制·含完整审核规则逆向分析+转载自动填写优化方案**·17条审核规则·6站组·短剧分类·8K·AV1·最小1GB·HDR四标签·中字/国语智能检测·标题-MI交叉验证） |
| 猫 | [pterclub.md](./pterclub.md) | ✅ 完成（**极简字段3下拉框·种子检查脚本完整逆向185KB JS→23项验证规则**·标题命名规范7种类型·质量/分类/地区判定算法·音频语言/字幕检测·DUPE规则·图床黑名单·独立Wiki·标签checkbox非tags数组） |
| 套套哥 | [ttg.md](./ttg.md) | ✅ 完成（自研框架·56混合分类·无质量字段无标签·禁转=nodistr·team=type重复·标题正则解析·[TTG]前缀清理） |
| 趟平 | [tangpt.md](./tangpt.md) | ✅ 完成（standard_sel含8K·32制作组·19音频含AV3V·30标签含NSFW·AV1·站组TPWEB） |
| 太乙 | [tey.md](./tey.md) | ✅ 完成（韩国资源特色·standard_sel含8K·极简媒介5种·Atmos三分·站组Tey·PT-Gen+MediaInfo） |
| 他吹吹风 | [et8.md](./et8.md) | ✅ 完成（教育特色·全站禁影视·source_sel=学科·standard_sel=分辨率·medium含电子书格式·Elearning 8分类） |
| 思齐 | [siqi.md](./siqi.md) | ⛔ 不适合（图像书特色站·无类似站点·Power User发种·豆瓣集成·无质量字段） |
| 瞬间 | [momentpt.md](./momentpt.md) | ⛔ 不适合（纯图片特色站·无类似站点·禁止视频/音频/软件/游戏） |
| 拾刻 | [ptskit.md](./ptskit.md) | ✅ 完成（仅源站·短剧特色·NexusPHP·禁止短剧/动态漫/擦边剧转出·开放电影/电视剧/动漫） |
| 天空 | [hdsky.md](./hdsky.md) | ✅ 完成（Cloudflare·候选制·16媒介·22音频·31标签·option_sel替代tags·Dupe严格·禁转REMUX·豆瓣链接·海外分类独立） |
| 天枢 | [dubhe.md](./dubhe.md) | ✅ 完成（极简字段·无音频编码·编码ID非标准·6标签·6制作组·站组DubheWeb·价格系统·Books/photo分类·游戏限制·PT-Gen） |
| 图88 | [tu88.md](./tu88.md) | ⛔ 禁止发布（9KG/成人图片站·无质量字段·分辨率在标签中·34标签·5分类全成人·繁体中文·HTTP Tracker） |
| 忘年桥 | [ptlao.md](./ptlao.md) | ⛔ 禁止发布（9KG/成人向·4分类全成人·双mode质量字段·无音频编码·站组PTL·PT-Gen独立字段·价格系统·禁止下载特别区） |
| 我堡 | [ourbits.md](./ourbits.md) | ✅ 完成（Cloudflare·processing_sel=地区·FHD/UHD媒介独立·标签字符串值·HDR五标签·仅原创/原抓制作组·官方组优先Dupe·禁REMUX/WebRip·禁FRDS/FGT·3D分类·IMDb+豆瓣必填） |
| 五二 | [52pt.md](./52pt.md) | ✅ 完成（Cloudflare·媒介语义自定义(有/无中文/中字)·8K·编码含重复·标签中文值·3D分辨率·戏曲/18+分类·跨站制作组·全站H&R·官组免Dupe） |
| 下水道 | [sewerpt.md](./sewerpt.md) | ✅ 完成（极简制作组2个·19音频含AV3A/ALAC/OPUS·无AV1·2K/8K分辨率·16标签含冷门低分/粤语/短剧/高码率·PT-Gen独立字段·冷门低分+粤语特色分区） |
| 蟹黄堡 | [crabpt.md](./crabpt.md) | ✅ 完成（种审制·source_sel替代medium_sel·双mode影视+书籍·mode6文档编码EPUB/AZW3·37制作组含重复·37影视标签+34书籍标签含网文·processing_sel=地区含印度·H.266/VVC·0DAY英文标题强制·站组XHB·独立Wiki） |
| 杏坛 | [xingtan.md](./xingtan.md) | ✅ 完成（学术资源专题·90+学科分类·source_sel=载体类型·processing_sel=文件格式27种含CAJ/UVZ/DJVU·无影视质量字段·新增author/publisher/ISBN字段·无匿名发布） |
| 星空 | [starspace.md](./starspace.md) | ✅ 完成（**FireFly自研框架**·双发布系统视频+音乐·字段tr_前缀·来源层级编码s11-s54·分类字符串ID·HDR独立下拉·Gazelle风格音乐·禁止DIY/Remux·压制仅WiKi/CMCT·站组Ying系列） |
| 星湾 | [xingwan.md](./xingwan.md) | ✅ 完成（**仅下载站**·建设初期·NexusPHP·开放注册·无发布权限·不做源站和目标站） |
| 星陨阁 | [xingyungept.md](./xingyungept.md) | ✅ 完成（FHD/UHD媒介独立·编码极简6种含AV1·19音频含AV3V/ALAC/OPUS·8K·站组Starfall系列+rain·24标签含超分/补帧/高帧率/高码率·短剧分类·PT-Gen·Tracker独立子域名） |
| 熊猫 | [pandapt.md](./pandapt.md) | ✅ 完成（双mode种子区+特别区·制作组ID按mode不同·12媒介含UHDTV·13音频含AV3A·8地区含印度/东南亚·25+黑名单制作组·认领系统3倍魔力·H&R手动模式·短剧分类·横竖屏标签·TMDB独立字段·候选区修改7天时限） |
| 学校 | [btschool.md](./btschool.md) | ✅ 完成（Cloudflare·教育特色·极简字段无source/地区·标签用span[]仅6个·9媒介含DVDRip·9音频DTS-HD/DTS合并·分辨率1080p/i合并·12制作组含跨站组·游戏类仅上传员直发·IMDb/豆瓣仅填编号·无PT-Gen·发布者双倍上传·认领系统·H&R 10天20小时·限速25MB/s） |
| 阳光 | [sunnypt.md](./sunnypt.md) | ✅ 完成（**仅源站**·原盘特效站·无直接发布权限User需候选·规则与学校站高度相似·表单结构未获取） |
| 壹吧 | [1ptba.md](./1ptba.md) | ✅ 完成（Cloudflare·教育特色·**特别区9KG禁止**·种子最小1GB·source_sel=媒介来源·codec_sel混合音视频·8K·processing_sel仅Raw/Encode·6制作组HDS/CHD/MySiLU/WiKi·21标签含特效/音乐系列·双mode种子区+特别区·认领1000种·H&R 5天24小时） |
| 樱花 | [ying.md](./ying.md) | ✅ 完成（新站2025·单mode·无音频编码字段·无地区·**媒介ID非标准UHD=1/BD=2**·**分辨率ID非标准4K=1/1080p=2**·编码含AV1·分辨率含8K·6制作组含YHWeb·16标签含韩剧/超分/零魔·PT-Gen·MediaInfo） |
| 优堡 | [ubits.md](./ubits.md) | ✅ 完成（**仅源站**·Cloudflare·官组极活跃UBits/UBWEB/UBTV·种审制·9分类·10媒介·9编码含AVS·14音频Atmos/DTS:X独立·分辨率含1440p·10地区含泰/印/俄·4制作组·17标签含原生原盘/高分国剧·IMDb+PT-Gen·禁止删除原站标识） |
| 幼儿园 | [u2.md](./u2.md) | ✅ 完成（**仅源站**·动漫站·**深度定制NexusPHP无标准下拉菜单**·自由文本输入框·标题系统自动拼接·四大分类anime/manga/music/other·候选制严格·UCoin经济系统·AniDB集成·音频逐级缩进·媒介含BDRip/DVDRip/Remux/Live·编码含AV1/HEVC/AVC/VCE/NVENC/QSV·分辨率含1080i/SD·全站无PT-Gen/豆瓣/IMDb） |
| 在脚下 | [hdroute.md](./hdroute.md) | ✅ 完成（**仅源站**·**自研框架**·老牌原盘/高清站·**普通用户仅能发原盘**·Uploader制·独特"最高音轨"字段·三段式标题(中文+一句+英文)·9分类含电影音轨·7媒介·7编码含MVC·5分辨率ID不连续含4K2K·9音频含LPCM·音轨复选框(国语/粤语/中字/DIY)·无标签·无制作组下拉·拒绝转发标记·IMDb得分+编号·预告片仅优酷/土豆·CKEditor·2013建站） |
| 织梦 | [zmpt.md](./zmpt.md) | ✅ 完成（源站+发布站·Cloudflare·种审制·**审核脚本完整逆向(Greasyfork #552769 v2026.03.08 17项校验规则)**·7站组ZmWeb/ZmPT/ZmMusic/ZmAudio/DYZ-Movie/GodDramas/RL·9分类含短剧/有声书/游戏·10媒介含MiniBD·6分辨率含8K/2K·10音频含ALAC/WAV/OGG·7制作组·11分类ID非连续(401-427)·分辨率ID非标准(1080p=1/4K=5/720p=8)·转载来源检测Map(50+站别名)·HDR/杜比/中字/国语标签与MI交叉验证·标题严格校验(中文/分辨率P小写/年份/编码)·认领2000种·全站随机促销·宝可梦主题·电力值经济系统·**含完整转载自动填写优化方案**） |
| 自然 | [zrpt.md](./zrpt.md) | ⚠️ 部分完成（纪录片特色站·建站2025·规则与织梦同模板·**上传表单未获取(审核拒绝达上限)**·9分类为纪录片题材(军事/生物/美食/人文/宇宙/音乐/体育/其它)·分类ID非连续(401-412)·全站Free(2025-09~2026-06)·发布者双倍上传） |
| 劳改所 | [ptlgs.md](./ptlgs.md) | ✅ 完成（源站+发布站·Cloudflare·**种审制·官方审核脚本完整逆向(ptlgs-Torrent-Assistant v1.1.43 830行 29+项校验规则·含种审员模式额外15项)**·10次候选制·豆瓣优先建库·7官组DYZ-WEB/DYZ-Movie/DYZ-TV/Eleph/beAst/ZmWeb·字幕组分类(411)·11媒介ID非标准(Blu-ray=14)·5编码无AV1·11音频DTS=19非标准·6分辨率无8K·19标签含HLG/SDR/3D/驻站/独家·**31+黑名单制作组**·白名单图床7个·独立Wiki·禁止超分/补帧/机翻·**含完整转载自动填写优化方案**） |
| 我爱电影 | [52movie.md](./52movie.md) | ⚠️ 部分完成（**规则与织梦/自然完全同模板(模板B组)**·Cloudflare·候选制·**上传表单未获取(账户无发布权限)**·独立字幕区·认领1000种·签到500魔力·游戏类仅上传员·建站2024） |
| 龙PT | [longpt.md](./longpt.md) | ✅ 完成（**规则与织梦/自然/我爱电影同模板(模板B组)**·Cloudflare·候选制·11媒介含**UHD Blu-ray独立**+**UHD Remux独立**·6编码含AV1·18音频含DDP/AV3A/DTS:X/Atmos/ALAC/WAV/OGG/M4A·7分辨率含8K/2K·8制作组LongA/LongWeb/LongPT/WiKi/RL/CMCT/HHWEB·16标签含英字/臻彩MAX/去广告纯享版·无匿名/无PT-Gen豆瓣/无地区·签到1000魔力·建站2024） |

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
*最后更新：2026-04-19*
