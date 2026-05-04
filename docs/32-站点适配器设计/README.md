# 站点适配器设计文档

> 本目录包含各个 PT 站点的特异化适配器设计文档，每个站点独立一个文档，便于维护和扩展。

## 目录

| 站点 | 文档 | 状态 |
|------|------|------|
| 青蛙 | [qingwapt.md](./qingwapt.md) | ✅ 完成（**种审制·审核脚本完整逆向(qingwa-torrent-assistant v1.1.1 1886行/82KB→44+项校验规则)**·22音频含DTS:X/DDP/Atmos/AV3A/USAC/OPUS·官组FROG/FROGE/FROGWeb·音乐分类跳过校验·28+禁发制作组·**含完整转载自动填写优化方案**） |
| 红豆饭 | [hdfans.md](./hdfans.md) | ✅ 完成（**全站H&R V2.0(下载100%后720h/保种72h/分享率1:1/10HP/99,999魔力消除)**·**盒子100%黑种+72h/3x上传限额+25MB/s限速+羊毛盒封禁(GCP/AWS/Azure/Oracle等)**·**11制作组黑名单(Hao4K/Mp4ba/rarbg/FGT/xiaomi/huawei/GPTHD等)**·**HDR后缀禁止转载**·印度/新加坡/马来西亚须候选·媒介20种(UHD/BD分开·原盘/DIY/Remux/压制分开)·30制作组·27标签·24音频·认领100种(3倍魔力)·来源优先级Dupe·发布者双倍上传·PT-Gen四来源） |
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
| 龟站 | [kamept.md](./kamept.md) | ⛔ 不做支持（9KG/成人内容为主·双区域系统(种子区19+特别区8)·字段语义完全重定义(source_sel=马赛克类型/team_sel=中文字幕/medium_sel=原始语言/processing_sel=同人·官方)·无Tracker URL·KameGen工具·候选制·认领系统） |
| 轨道炮 | [railgunpt.md](./railgunpt.md) | ✅ 完成（**仅目标站·只允许发布至种子区(mode=4)·禁止特别区(mode=5)**·14种子分类含下架视频备份(420)/漫画(419)/学习(411)·无官组·游戏类需候选·完整Dupe规则·10随机促销+2自动促销·认领1000种·全站Free限免中） |
| 海胆 | [haidan.md](./haidan.md) | ✅ 完成（仅目标站·独立Tracker子域名·durl豆瓣链接·season/episode独立字段·team_suffix文本输入·tag_list[]字段名·540P分辨率·做种人数动态促销(1-7人免费)·魔力购买置顶/免费·禁止<200MB资源·电子书允许） |
| 海豹 | [gpw.md](./gpw.md) | ⚠️ 仅规则记录（**GazellePW框架**·纯电影站(PTP模式)·**槽位制(25种可替代标记)**·禁止剧集/体育/MV·40+制作组黑名单·版本信息标签体系(3D/CC/MOC/HDR10+/DV/Atmos/DTS:X)·外挂字幕系统·SPA+MDX规则页面·分享率动态计算·做种积分公式） |
| 海棠 | [htpt.md](./htpt.md) | ⚠️ 仅规则记录（**戏曲类特色站·不适合做源站和发布站**·8戏曲分类(相声/评书/戏曲/鼓琴/小曲/小品/二人转/小剧种)·codec_sel含MP3/M4A·仅官组HTPT·标签拼音缩写(jz/gf/yq/gq/mp3/mp4/gc)·须转MP4/MP3格式(唱戏机)·新发100%免费7天后永久50%·官组禁转·禁止VIP付费内容） |
| 海豚 | [dicmusic.md](./dicmusic.md) | ✅ 完成（**Gazelle框架·纯音乐**·15音乐分类·10媒介(字符串值)·6格式·12比特率·70流派标签·艺人+重要性系统·Log Checker·MQA严格禁止·GSC/SAE/SPGM/AIGM垃圾规则·SACD禁止ISO·WEB须购买截图·FLAC须lv8压缩·首种<2GB免惩·完整有损替代优先级链） |
| 憨憨 | [hhanclub.md](./hhanclub.md) | ✅ 完成（**仅源站·全站官种**·盒子100%计量(黑种)·72h内上传限3倍·超速100MB/s封禁·6大云服务商封禁·H&R 20天/72h·5次警告封禁·消除需50万憨豆·制作组黑名单FGT/Hao4K/Mp4Ba/RARBG·禁止未完结分集·禁止AI插帧·禁止HDR+DV合并·音乐仅官组·标题0day命名·憨豆货币） |
| 馒头 | [mteam.md](./mteam.md) | ✅ 完成（mTorrent自研SPA+API） |
| 好大 | [hdarea.md](./hdarea.md) | ✅ 完成（目标站·Cloudflare·**盒子100%计量+72h/3x上传限额(VIP+豁免)**·**完结剧删除分集只保留合集**·29音频含DTS:X/DSD/MQA/AC-4/MPEG-H/AV3A·候选制·豆瓣必填·发布者双倍上传·官种促销阶梯(原盘→30%/1080P→30%/720P→50%)·字幕区·18分类含UHD-4K(300)/3D/iPad·13制作组·无标签·HDApt字段映射参考） |
| 好多油 | [hdupt.md](./hdupt.md) | ✅ 完成（目标站·**媒介TV/电影分开15种(UHD独立)**·官组HDU·**HR 72h/2周逃避直接封禁**·速度监控上行200MiB/s·**Dupe:官组优先+BR优先WEB+完结剧禁单集+优质源替代**·**禁止二次编码/非官方合集**·8地区含India/SEA·8类资源发种细则·促销7级阶梯(普通40天→2X→50%)·H.264/x264分开·iPad分辨率） |
| 好学 | [hxpt.md](./hxpt.md) | ✅ 完成（目标站·**纯教育专题·禁止连续剧和纯娱乐电影·仅允许教育纪录片/教育影音**·字段全部重定义(medium=载体/codec=文件格式/audiocodec=年级/processing=教材版本/source=年份)·7分类(学前→纪录片)·26文件格式含CAJ/DJVU/AZW3·14年级·27教材版本·13学科标签·火花货币·价格系统30%税率·PT-Gen四来源(imdb/douban/bangumi/indienova)·所有种子1个月后永久2x·Dupe来源优先级+45天/18月豁免·认领1000种·JWT认证） |
| 皇后 | [opencd.md](./opencd.md) | ✅ 完成（NexusPHP定制·**纯音乐**·**候选制(≤3种强制候选)**·**H&R全局30天/36h/10次ban/10000魔力消除**·**6档随机促销+3天过期**·**6大类Dupe(CD/WEB/Hi-res/DSD/DTS/转录)独立规则**·7资源类别(CD/Web/Hi-res/SACD+DSD/DTS/转录/Other)·**MQA以展开前规格为准**·**满分LOG永久保留**·转载4级(禁转/须经允许/欢迎/其他)·OpenCD/LLM原抓免Dupe·发布者双倍上传·plugin_upload定制路径·22媒介·8格式·17流派·5制作组·27标签(5地区+22风格)·Log Checker·频谱图校验） |
| 家园 | [hdhome.md](./hdhome.md) | ✅ 完成（**双区域·8K分类·候选制**·豆瓣ID·**与铂金家互斥禁止互相转载**·**H&R 60天/336h/10次ban/20000魔力消除**·**盒子100%黑种+72h/3x限额+VPS禁止**·**单种限速25M/s**·**防吸血200GB/0.2封禁**·促销4档随机(30%/50%/Free/2X)+7种类型·Dupe来源优先级(BR>HDTV>DVD>TV)+发布组优先级+无损截图对比+断种60天/12月豁免·资源打包规则(电影合集/整季/5张专辑)·游戏仅上传员可发·49种子区+27LIVE区分类·14制作组(HDHome/HDH/HDHTV/HDHPad/HDHWEB/3201/SHMA/TVman/ARiN/TTG/M-Team/BMDru)·18标签(原创/国语/粤语/DV/HDR10/HDR10+/Criterion/禁转/限转/首发/应求等)·字幕区规则） |
| 咖啡 | [ptcafe.md](./ptcafe.md) | ✅ 完成（**source_sel=地区(7个)**·**禁止R18**·**发布者1.5倍上传**·30制作组(PTCafe/CafeTV/CafeWEB)·18音频(OPUS/OGG)·13媒介(UHD/BD原盘/DIY分开)·11编码(H265/x265分开)·16标签·data-mode='4'·**促销:90%随机Free+>20GB自动Free+阶梯时限(Free→7天→50%→90天)**·**超速100MB/s三次封禁**·**盒子试行(>100MB/s须登记)**·**认领:10人/月500h/不达标扣3000豆**·**已完结剧禁止分集**·**官种严格命名(主标题+副标题+文件名+文件夹名)**·**咖啡豆货币**·转载原样/禁删后缀·MediaInfo独立字段） |
| 克隆 | [hdclone.md](./hdclone.md) | ✅ 完成（**极简字段·无source/audio·短剧·AV1**·5标签·14制作组(无Other)·**促销:5档随机+>20GB免费+Blu-ray免费+每季首集免费+7天时限+1个月永久2x**·Dupe优先级(BR>HDTV>DVD/TV)+动漫HDTV=DVD特例+DVD5保留+断种45天/18月豁免·**盒子正常享优惠(非黑种)**·**认领8人/2000种/月420h/5倍魔力**·游戏仅上传员·资源打包规则·字幕区规则） |
| 可撸 | [kelu.md](./kelu.md) | ⛔ 暂不支持（男同向成人站·双区域(种子区6分类+伪娘区1)·**极简表单无任何质量字段**·类型按地区分类(国产/日韩/欧美/南亚/中东/其他)·23+13标签·data-mode='4'/'5'·**50%随机Free**·**1个月后永久2x**·Dupe断种45天/18月豁免·认领50人/500种·9级魔法师等级体系） |
| 库非 | [kufei.md](./kufei.md) | ✅ 完成（**Cloudflare**·16分类(含游戏/电子书/软件/教育/有声读物)·17媒介(UHD/BD各分原盘/DIY/Remux/压制+MiniSD+SACD)·22音频(DSD/TTA/MPEG/OGG×2)·10编码(含AV1)·**促销:5档随机+>20GB免费+BR原盘免费+每季首集免费**·Dupe优先级(BR>HDTV>DVD/TV)+动漫HDTV=DVD特例+DVD5保留+断种45天/18月豁免·游戏仅上传员·**认领1000种**·保种组退休待遇(10月→养老族/8月→VIP)·PT-Gen四来源·5制作组(HDS/CHD/MySiLU/WiKi)·12标签） |
| 昆仑 | [yhpp.md](./yhpp.md) | ✅ 完成（**processing_sel=地区(12个CN/HK/TW/US/JP/KR/UK/EU/IN/SG/MY)**·19媒介(UHD/BD各四级+黑胶/CD+VCD/CD+DVD/SACD)·23音频(ALAC/m4a/DSD/TTA)·29制作组(含PTP/BTN/FraMeSToR/EPSiLON)·19标签(HDR/DV/Atmos/4K/8K/Hi-Res/AI修复)·**促销:90%随机Free+>1GB自动免费+5天时限**·Dupe优先级(BR>HDTV>DVD/TV)+动漫HDTV=DVD特例+DVD5保留+断种45天/18月豁免·Peasant降级阶梯(50GB/0.4→800GB/0.8)·字幕区规则） |
| 垃圾堆 | [lajidui.md](./lajidui.md) | ✅ 完成（Cloudflare·processing_sel=文件格式·source_sel=地区·16分类·2K分辨率·**促销:5档随机+>20GB免费+BR免费+每季首集免费+7天时限+1个月永久2x**·Dupe优先级(BR>HDTV>DVD>TV)+动漫HDTV=DVD特例+DVD5保留+断种45天/18月豁免·**发布者双倍上传**·游戏仅上传员·资源打包规则·种子信息命名规范·字幕区规则·管理组退休待遇） |
| 聆音 | [soulvoice.md](./soulvoice.md) | ✅ 完成（**双模式影视+阅听**·**阶梯促销:新种免费→7天50%→14天普通·>20GB免费&2x**·**H&R:72h/30天/10封禁/20000魔力消除/200元解封**·电子书/有声书·字段语义按模式切换(source=语言/audio=电子书格式/processing=书籍分类)·13标签含HLG/杜比·极简影视字段(无source/audio/processing)·5媒介(APE+FLAC合并/DSD独立)·3编码·4分辨率(无720p)·5制作组·游戏仅上传员·Dupe+动漫/学习特例+DVD5保留+断种45天/18月豁免·资源打包规则·管理组退休待遇·**禁止Adult Video**·**20条封禁行为**） |
| 龙之家 | [dragonhd.md](./dragonhd.md) | ✅ 完成（**繁体中文**·标准NexusPHP规则模板·AV分类·无标签·2K/1440p·极简字段(无source/processing)·9媒介UHD独立·6编码含VP9·15音频DTS细分4级+DD/DD+/AC3合并·6分辨率·9制作组DragonHD/LeagueHD·**促销:5档随机+>20GB免费+BR免费+每季首集免费+7天时限+1个月永久2x**·Dupe优先级(BR>HDTV>DVD>TV)+动漫特例+DVD5保留+断种45天/18月豁免·资源打包规则·字幕区规则·游戏仅上传员·发布者双倍上传·建站2020） |
| 萝莉 | [xloli.md](./xloli.md) | ✅ 完成（动漫向·双区域综合+9KG·禁止9KG·13动漫制作组·舞台演出·OPUS） |
| 末日 | [agsv.md](./agsv.md) | ✅ 完成（Cloudflare·种审制·**审核脚本完整逆向(Agsv-Torrent-Assistant v1.4.7 1480行/65KB→30+校验规则)**·27黑名单·双区域综合+学习·大包规则·ALAC/M4A·18分类含短剧/漫画/图片·官组AGSVPT/AGSVMUSIC/Hares/RL/BeiTai·方舟计划标签·冰种标签·Music/Audio/Game等7分类跳过校验·**含完整转载自动填写优化方案**） |
| 慕雪阁 | [muxuege.md](./muxuege.md) | ✅ 完成（HDR10编码·TXT/PDF编码·540p·47制作组·31标签·无音频编码） |
| 南洋 | [nanyangpt.md](./nanyangpt.md) | ✅ 完成（NYPT框架·极简发布·无质量下拉框·禁止蓝光原盘·剧集dupe·豆瓣链接） |
| 奶昔 | [nicept.md](./nicept.md) | ⛔ 禁止发布（**纯 9KG/成人内容站·不适合做源站和发布站**·NexusPHP·8分类(日本有码/日本无码/欧美/动漫限制级/写真套图/真人秀限制级/SM调教限制级/其他限制级)·极简表单(无媒介/无音频编码/无制作组/无PT-Gen/无海报/无截图字段)·来源6·编码6含HEVC·分辨率5含4K(非标ID)·6标签·H&R 600h/99h/分享率>2/20封禁·促销5档随机+>200GB免费+BR免费+国产无优惠·打包须候选·认领1000种·建站2019） |
| 柠檬不甜 | [lemonhd.md](./lemonhd.md) | ✅ 完成（双语分类·4K/8K独立媒介·3D分类·PT-Gen四来源·匿名发布·5倍上传） |
| 农场 | [farmm.md](./farmm.md) | ✅ 完成（Cloudflare·双区域种子+特别·source_sel=地区·processing_sel=年级/分级·17媒介·15编码·20音频·1440p·儿童教育特色） |
| 朋友 | [keepfrds.md](./keepfrds.md) | ✅ 完成（**仅源站**·全站官种·Cloudflare·HEVC细分5级·8K·19分类·转载须24h后·黑名单制作组） |
| 葡萄 | [sjtu.md](./sjtu.md) | ✅ 完成（教育网·28分类按地区细分·编码含音频·禁止HEVC/10bit·黑名单组·豆瓣链接·校园原创） |
| 浦园 | [njtupt.md](./njtupt.md) | ✅ 完成（教育网·演出分类·资料分类·MediaInfo字段·PT-Gen四来源·极简质量字段·标准规则） |
| 麒麟 | [hdkyl.md](./hdkyl.md) | ✅ 完成（种审制·**审核脚本完整逆向 HDKylin-Torrent-Assistant v1.3.15 1364行/58KB→20+校验规则·改自不可说SpringSunday脚本·22黑名单组(当前注释)·UHD Blu-ray独立(24)·AV1脚本BUG(encode=12不存在)·6分类跳过全部校验(音乐/电子书/图片/漫画/游戏/软件)·官种+驻站+麒麟火三标签联动·GodDramas驻站短剧特殊规则·MediaInfo宽泛检测(9种格式)·processing_sel=年份·source_sel=地区22个·19音频含Atmos/DTS:X/DDP·**含完整转载自动填写优化方案**） |
| 人人 | [audiences.md](./audiences.md) | ✅ 完成（Cloudflare·候选制·0day命名·无制作组/来源/地区字段·HDR三标签·Trump共存规则·Web-DL/WebRip·爆米花系统） |
| 朱雀 | [zhuque.md](./zhuque.md) | ✅ 完成（**TNode框架**·Vue SPA+REST API·CSRF Token·TMDb必填·H264/x264四分·ID分段体系(编码1xx/媒介3xx/分辨率4xx/分类5xx/标签6xx)·无音频编码·标签逗号分隔·**修仙等级体系(筑基→真仙/走火入魔/自动封禁500GiB+0.5)**·**抽卡系统(四星13%/五星1.6%/收益衰减)**·**认领规则(7天后/7人/240h/灵石奖励)**·**无HR规则**·全站单种限速50MiB/s·种审制(7天修改/超期扣上传)·禁止合集/跨季打包·非官组禁止分集·转载无需重制种·禁止修改种子文件·连坐规则[2023-12-28]·允许4客户端·新种24h↑1x↓0x·发布者↑2x） |
| 肉丝 | [rousi.md](./rousi.md) | ✅ 完成（**自研框架**·Vue SPA+REST JSON API·Passkey认证·UUID种子·Base64截图·Markdown描述·动态属性·11分类·9KG专区禁止） |
| 三月 | [duckboobee.md](./duckboobee.md) | ✅ 完成（user可发种·HEVC≠H.265双选项·8K/576P/480P分辨率·3MWEB官组·21标签含6音乐标签·dupe来源优先级·无音频编码） |
| 时光 | [hdtime.md](./hdtime.md) | ✅ 完成（Cloudflare·候选制·PT-Gen四来源·极简质量·无分辨率·AV1/VVC/AVS3·Audio Vivid） |
| 时间 | [pttime.md](./pttime.md) | ⛔ 禁止发布（PTT-NP框架·极简发布·无质量字段·标签字符串值·16分类·9KG双区·候选制） |
| 蝴蝶 | [hamster.md](./hamster.md) | ✅ 完成（**老牌教育网站(华中科技大学)**·**无质量下拉框(全靠标题)**·**Tracker=hudbt.hust.edu.cn**·32分类按地区细分(大陆/港台/亚洲/欧美)·dl-url上传·**电影Scene/iNT Dupe(720P多版/1080P多版/断种3月豁免)**·**720P码率≥4000kbps**·**剧集:完结后单集→Dupe+欧美仅整季**·**音乐:禁止FLAC整轨/APE/TTA/OGG/WMA/M4A+须≥320kbps+整张专辑**·**综艺HDTV限CHDTV/NGB/HDWTV/BYRTV**·9+黑名单制作组·发布者1.5x上传·HUST分类） |
| PT地带 | [ptzone.md](./ptzone.md) | ✅ 完成 |
| 幸运 | [luckpt.md](./luckpt.md) | ✅ 完成（Cloudflare·**LuckAudit预审核系统100分制·含完整审核规则逆向分析+转载自动填写优化方案**·17条审核规则·6站组·短剧分类·8K·AV1·最小1GB·HDR四标签·中字/国语智能检测·标题-MI交叉验证） |
| 猫 | [pterclub.md](./pterclub.md) | ✅ 完成（**极简字段3下拉框·PTerClub Torrent Checker v1.0.22 完整逆向2245行/141KB→40+校验规则·TORRENT_INFO结构化对象三方交叉验证(标题+MI+豆瓣)·分辨率从宽高差值推断·制作组白名单决定质量等级·多音轨完整流解析·标题命名7种类型·音频格式映射表·17制作组白名单·9编码组·DUPE规则·图床黑名单·独立Wiki·标签checkbox非tags数组·含转载方案**） |
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
| 影 | [starspace.md](./starspace.md) | ✅ 完成（**FireFly自研框架**·双发布系统视频+音乐·字段tr_前缀·来源层级编码s11-s54·分类字符串ID·HDR独立下拉·Gazelle风格音乐·禁止DIY/Remux·压制仅WiKi/CMCT·站组Ying系列） |
| 星湾 | [xingwan.md](./xingwan.md) | ✅ 完成（**仅下载站**·建设初期·NexusPHP·开放注册·无发布权限·不做源站和目标站） |
| 星陨阁 | [xingyungept.md](./xingyungept.md) | ✅ 完成（FHD/UHD媒介独立·编码极简6种含AV1·19音频含AV3V/ALAC/OPUS·8K·站组Starfall系列+rain·24标签含超分/补帧/高帧率/高码率·短剧分类·PT-Gen·Tracker独立子域名） |
| 熊猫 | [pandapt.md](./pandapt.md) | ✅ 完成（双mode种子区+特别区·制作组ID按mode不同·12媒介含UHDTV·13音频含AV3A·8地区含印度/东南亚·25+黑名单制作组·认领系统3倍魔力·H&R手动模式·短剧分类·横竖屏标签·TMDB独立字段·候选区修改7天时限） |
| 学校 | [btschool.md](./btschool.md) | ✅ 完成（Cloudflare·教育特色·极简字段无source/地区·标签用span[]仅6个·12制作组含跨站组·1080p/i合并·DTS-HD/DTS合并·**促销6档随机概率(10%/5%/5%/3%/1%)·>20GB自动Free·BR原盘Free·每季首集Free·7天时限·1个月永久2X**·**H&R 10天/20h·≥10次ban·20000魔力消除**·**限速25MB/s·非发布者超速3倍扣罚·100MB/s自动封号**·资源打包规则·字幕区规则·认领20人/100种·游戏仅上传员·IMDb/豆瓣仅填编号·发布者双倍上传） |
| 阳光 | [sunnypt.md](./sunnypt.md) | ✅ 完成（**仅源站**·原盘特效站·建站2024·Redis后端·User需候选·规则与学校站同模板·**无H&R·无认领系统·无官组禁止选择**·标题含中文前缀`[中文名] 英文名...`·促销6档随机·>20GB/BR原盘/每季首集自动Free·1个月永久2X·资源打包规则·字幕区规则·管理组退休待遇(上传员→养老族/VIP)·IMDb必填·发布者双倍上传） |
| 壹吧 | [1ptba.md](./1ptba.md) | ✅ 完成（Cloudflare·教育特色·**特别区9KG禁止发布·只允许种子区mode=4**·种子最小1GB·Redis后端·source_sel=媒介来源·codec_sel混合音视频·8K·processing_sel仅Raw/Encode·6制作组HDS/CHD/MySiLU/WiKi·21标签含特效/音乐系列·双mode种子区+特别区·认领1000种·**H&R 5天/24h·分享率≥1.8免·≥8次ban·20000魔力消除·30天停止检查**·**促销4档随机(50%/20%/25%/5%)·>30GB免费·BR原盘免费&2x·3天时限·2个月永久2X·PT-Gen·MediaInfo独立字段·YouTube链接·IMDb必填·管理组退休待遇·字幕区规则·发布者双倍上传） |
| 樱花 | [ying.md](./ying.md) | ✅ 完成（新站2025·Redis后端·单mode无特别区·无音频编码字段·无地区·**媒介ID非标准UHD=1/BD=2**·**分辨率ID非标准4K=1/1080p=2**·编码含AV1·分辨率含8K·6制作组含YHWeb·18标签含韩剧/超分/零魔·PT-Gen·MediaInfo独立字段·**促销6档随机(10%/5%/5%/3%/1%)·>20GB/BR原盘/每季首集自动Free·7天时限·1个月永久2X**·资源打包规则·字幕区规则·管理组退休待遇·签到500魔力·发布者双倍上传·uplver默认勾选） |
| 优堡 | [ubits.md](./ubits.md) | ✅ 完成（**仅源站**·Cloudflare·Redis后端·**静默模式已开启**·官组极活跃UBits/UBWEB/UBTV·种审制·**候选积分制(<10自动候选)·2025.04.11后类型/MediaInfo/质量区域必填**·9分类·10媒介·9编码含AVS·14音频Atmos/DTS:X独立·分辨率含1440p·10地区含泰/印/俄·4制作组·18标签含原生原盘/高分国剧/特效字幕·**标签互斥:原生原盘/DIY/菜单修改三选一·连载/合集二选一·DV可与任意HDR共存**·IMDb+豆瓣必填+PT-Gen·**促销:官种2x&Free48h/非官种Free12h/原生原盘2x&Free24h·2天后永久50%或2x&50%**·**与不可说互斥禁止互相转载**·**黑名单:HDSky/CMCT全系列禁止**·**盒子5x分享率上限+125MB/s限速**·禁止ISO封装非3D·禁止拆包/跳车刷流·账号保留90/60/45天·认领1090/10000·签到500魔力·独立Tracker子域名t.ubits.club·发布者双倍上传·禁止删除原站标识） |
| 野马 | [yemapt.md](./yemapt.md) | ✅ 完成（**自研SPA框架(Umi.js+Ant Design)**·REST JSON API发布·**简介Markdown非BBCode**·字段值字符串非数字ID·17分类(影视8+综合4+音频2+教育3)·10媒介·9分辨率含2K/8K·10视频编码含VVC·15音频含Atmos/DDP/Opus·8地区·11制作组·12标签·**H&R发布者可选开启(不可修改)·7天/24h考核·免罪20000积分·发布者30%回馈·≥10次ban·免疫规则(L10/契约/野马之友)**·**盒子:不享受上传促销·非自发3x上传限额计0**·**发布做种强制:上架72h内须做种24h否则处罚**·**发布奖励:前30种500积分·发布人24h/30d/365d做种积分10/5/1·发布人双倍上传**·Level 6免候选·Level 9永久保留·一天100种限制·种子最大3MB·CookieCloud域名yemapt.org） |
| 幼儿园 | [u2.md](./u2.md) | ✅ 完成（**仅源站**·动漫外番特色站·**深度定制NexusPHP无标准下拉菜单**·自由文本输入框·标题系统自动拼接·四大分类anime/manga/music/other·**候选制严格·自动通过投票评分系统**·UCoin经济系统·魔法促销·**促销仅1%/1%随机无自动促销**·多语言界面(中/英/俄/繁)·AniDB必填·NFO支持·海报poster字段·API支持·PT适宜原则·**发布者禁限条款三要素体系(适用对象/适用条件/豁免条件)**·**Dupe:质量提升/质量相同/无法修正三类**·合集规则5.1-5.8·原盘须BDMV文件夹(3D/MGVC除外BDISO)·转载须保留原文件名·蓝光无用文件须移除·奖惩条例·字幕zip/rar/7z英文文件名·账号保留600/90/120天·反作弊联盟·建站2008） |
| 在脚下 | [hdroute.md](./hdroute.md) | ✅ 完成（**仅源站（仅采集数据，不作为源站和发布站）**·**自研框架**·老牌原盘/高清站·**所有HDR/R2HD后缀及官方资源均为禁转（不论是否标注）**·普通用户仅能发原盘·Uploader制·独特"最高音轨"字段·三段式标题(中文+一句+英文)·9分类含电影音轨·7媒介·7编码含MVC·5分辨率ID不连续含4K2K·9音频含LPCM·音轨复选框(国语/粤语/中字/DIY)·6种优惠类型·无标签·无制作组下拉·IMDb得分+编号·预告片仅优酷/土豆·CKEditor·2013建站） |
| 织梦 | [zmpt.md](./zmpt.md) | ✅ 完成（源站+发布站·Cloudflare·种审制·**审核脚本完整逆向 zmpt-check-tool v2026.03.08 2697行/117KB→20+校验规则·五层递进HDR检测·54站转载来源Map·国语标签三方交叉验证·原盘DIY文件结构检测·中文字幕13种标识·中文音频8种标识·PT-Gen自建端点**·7站组·9分类含短剧/有声书/游戏·10媒介含MiniBD·6分辨率含8K/2K·10音频含ALAC/WAV/OGG·7制作组·11分类ID非连续·分辨率ID非标准·**16标签含完整ID映射(驻站/禁转/粤语/无损/首发/短剧/汉化/原创)·注意:上传页无'原盘'标签但审种脚本检测原盘**·**含完整转载自动填写优化方案·标题清洗9规则·标签自动选择8项·简介自动构建含引用关键词**·**字幕区规则(srt/ssa/ass/cue/zip/rar)·管理组退休待遇·超速封禁细化**·认领1000种·全站随机促销·宝可梦主题·电力值经济系统） |
| 自然 | [zrpt.md](./zrpt.md) | ✅ 完成（**纪录片特色站·仅支持发布纪录片**·建站2025·Redis后端·**规则与织梦同模板**·13分类为纪录片题材(军事/生物/美食/人文/宇宙/音乐/体育/旅游/纪录片+官组Nature/NatureWeb)·分类ID非连续(401-416)·**12媒介(UHD独立/Track独立/无HD DVD)**·9编码含AV1/VP9·**8分辨率(8K=1/4K=2/1080p=3/SD=6·与织梦完全不同)**·**20音频(DTS细分4级/Atmos/DDP/AV3A/USAC·与织梦完全不同)**·7制作组(HDS/CHD/MySiLU/WiKi/ZRPT)·19标签(含英语/连载/HDR10+/Remux)·technical_info独立字段·全站Free(2025-09~2026-06)·字幕区规则·管理组退休待遇·发布者双倍上传） |
| 劳改所 | [ptlgs.md](./ptlgs.md) | ✅ 完成（源站+发布站·Cloudflare·**种审制·官方审核脚本完整逆向(ptlgs-Torrent-Assistant v1.1.43 830行 29+项校验规则·含种审员模式额外15项·**含脚本差异分析(标签名不一致/死代码/电影豆瓣无映射/截图检查宽松)**)**·10次候选制·豆瓣优先建库·7官组DYZ-WEB/DYZ-Movie/DYZ-TV/Eleph/beAst/ZmWeb·字幕组分类(411)·11媒介ID非标准(Blu-ray=14)·5编码无AV1·11音频DTS=19非标准·6分辨率无8K·19标签·**HR规则:72h做种/360h缓冲/10HP封禁线**·**上传总则:48h做种/双倍上传/刷流判定/禁止违法资源(3次封禁)**·**31+黑名单制作组**·白名单图床9个·独立Wiki·禁止超分/补帧/机翻·**含完整转载自动填写优化方案**） |
| 我爱电影 | [52movie.md](./52movie.md) | ⚠️ 部分完成（**规则与织梦/自然完全同模板(模板B组)**·Cloudflare·候选制·**上传表单未获取(账户无发布权限)**·独立字幕区·认领1000种·签到500魔力·游戏类仅上传员·建站2024） |
| 龙PT | [longpt.md](./longpt.md) | ✅ 完成（**标准NexusPHP规则模板(与垃圾堆/克隆/库非/昆仑同源)**·Cloudflare·候选制·11媒介含**UHD Blu-ray独立**+**UHD Remux独立**·6编码含AV1·18音频含DDP/AV3A/DTS:X/Atmos/ALAC/WAV/OGG/M4A·7分辨率含8K/2K(非标ID)·8制作组LongA/LongWeb/LongPT/WiKi/RL/CMCT/HHWEB·16标签含英字/臻彩MAX/去广告纯享版·**促销:5档随机+>20GB免费+BR免费+每季首集免费+7天时限+1个月永久2x**·Dupe优先级(BR>HDTV>DVD>TV)+动漫特例+DVD5保留+断种45天/18月豁免·资源打包规则·字幕区规则·管理组退休待遇·游戏仅上传员·发布者双倍上传·签到1000魔力·建站2024） |
| 莫妮卡 | [monikadesign.md](./monikadesign.md) | ✅ 完成（**Unit3D(Laravel)框架**·聚焦日本动画/电影/剧集/ACG音乐·TMDB ID必填·MAL ID动画必填·CSRF Token·动画不接受Remux·PT Encode白名单(PTer/WiKi)·黑名单发布组·181发行商·21地区·独立海报/横幅/MediaInfo/BDInfo字段·发布后审核队列·分类按内容类型(Anime TV/Anime Movie/TV/Movie/Game)） |
| 猪猪 | [piggo.md](./piggo.md) | ✅ 完成（**仅发布站·禁止做源站·仅限儿童动画资源·全站官种禁转**·NexusPHP深度定制·儿童专区(IMDB分级过滤)·3D专题·独立Tracker子域名·画质字段(DV/HDR10+/HDR10/SDR)·Cloudflare·建站2022） |
| 烧包 | [ptsbao.md](./ptsbao.md) | ✅ 完成（**source_sel有80个细分来源**·processing_sel区分处理方式·80+级用户等级(宫廷风命名)·官组FFans系列·H&R规则·禁止猪猪字幕组动漫资源·data-mode=1） |
| UltraHD | [ultrahd.md](./ultrahd.md) | ⛔ 不适合（**几乎全站禁转**·仅接受韩国产地影视作品·内容范围过窄·转发价值极低） |

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
*最后更新：2026-05-04*

## UploadTorrent 适配器实现状态

| 站点 | 适配器文件 | UploadTorrent 状态 | 特殊处理 |
|------|-----------|-------------------|---------|
| 朱雀 | adapter_tnode.go | ✅ 已完成 | REST API `POST /api/torrent/upload`，CSRF Token，camelCase 字段名(category/medium/videoCoding/resolution)，TMDb，标签逗号分隔 |
| 莫妮卡 | adapter_unit3d.go | ✅ 已完成 | CSRF `_token`，`category_id`/`type_id`/`resolution_id`，TMDB/MAL ID，181发行商，21地区 |
| 海豚 | adapter_gazelle.go | ✅ 已完成 | 字符串值字段(media/format/bitrate)，Gazelle艺人系统，remaster版本，Log Checker |
| 套套哥 | adapter_generic.go (uploadTTG) | ✅ 已完成 | 仅 `type` 分类(58个)，无媒介/编码/分辨率字段，`file`字段名，`nodistr`禁转，`imdb_c`/`douban_id` |
| 影 | adapter_generic.go (uploadStarSpace) | ✅ 已完成 | 双发布系统(video_upload.php + music_upload.php)，`tr_`前缀字段名，独立checkbox标签，HDR下拉 |
| 野马 | adapter_generic.go (uploadYemaPT) | ✅ 已完成 | REST JSON API `POST /api/torrent/add`，`showName`/`shortDesc`/`categoryId`，Markdown简介，字符串值字段 |
