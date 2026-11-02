package db

import (
	"gorm.io/gorm"
)

// migration133 §59.451: 馒头（api.m-team.cc）发布表单配置分发——**首个
// API 架构目标站**（非 NP 表单：MTeamAdapter.uploadViaAPI 已在——multipart
// POST /api/torrent/createOredit+x-api-key；form_fields 值=API JSON 字段
// 名（camelCase），executor 域→字段名通用链直接工作）。
// 依据：docs/32/mteam.md 1694 行 API 文档读全+在线 swagger spec 实时对比
// （2026-11-02：TorrentUploadForm 28 字段——bangumi 字段新增（发布链无
// 域不配）+List 八端点实时值集（真实组件离线驱动——29 库 apiKey 解密
// 调 /api/torrent/*List）：**team 25 vs 资产 23——3 增 1 删**（+63 ZZH/
// +64 DStudio/+65 Aisha/-34 QHstudIo）；category 22 非成人叶/medium 10/
// codec 8/audiocodec 15/standard 6/processing 6 全零漂移+source label 微调
// （5 HDTV→HDTV/TV））。
// 结构要点：
//   - **category 树=类型×媒介×分辨率三维融合**（电影族 5 叶/影剧综艺族
//     4 叶按媒介细分——好大/时光同族）→ mteamCategoryValue typeMappingOf
//     第十站特化路由（影视族×mediumStdKeyOf×分辨率；动画原盘→453）；
//     影视族 9 叶+453 label-only（路由独占）+448 TV 游戏 label-only
//     （game 键归 423——平台细分无判定维度）；其余 13 类标准键直配。
//   - medium 10：1 Blu-ray=[bluray,uhd_bluray] 双键归并（无 UHD 项——
//     奥申同款）+6 DVDR=[dvd,dvdr] 双键宽语义（无 DVD 项——阿玲先例）+
//     Remux/WEB-DL 双键+MiniBD/HD DVD/Track ext 活键。
//   - codec 8 合并命名零联动：1 H.264(x264/AVC)=[h264]+16 H.265(x265/
//     HEVC)=[h265]——站方合并项语义（§59.380 反证链）+AVS ext 活键。
//   - audiocodec 15 全配键：8 AC3(DD)=[ac3,dd] 双键+12 E-AC3(DDP)=
//     [ddp] 单键（eac3 无键 Nova 先例）+13 E-AC3 Atmos=[ddp_atmos]
//     （红豆饭先例——atmos 泛键归 10）+10 TrueHD Atmos=[atmos,
//     truehd_atmos] 双键（慕雪阁模式）+11 DTS-HD MA+14 LPCM/PCM 双键
//     （咖啡）+4 MP2/3 双键+FLAC/APE/DTS/OGG(ext)/AAC/WAV/Other。
//   - standard 6：5 SD=[sd]+6 4K=[r2160p]+7 8K=[r4320p] 乱序。
//   - team 26（实时新值集——25 实项含漏录修正 TnP 后 26）全 label-only：MTeam/MWeb/TPTV 官组+外站
//     官组大表（含实时新增 ZZH/DStudio/Aisha）；**无 Other 项**（miss→
//     nil 预览人工——克隆同款）。
//   - **processing 产区槽不配**（CN/US-EU/HK-TW/JP/KR/OT——咖啡定案）
//     +**source 来源槽不配**（category 融合已覆盖媒介维度+独立槽非必填
//     系统自动推断——§10.2 注释）+countries 不配（发布链无产区投递）。
//   - **labels 位掩码**（bit0=DIY/bit1=中配/bit2=中字）：无 tags checkbox
//     槽——mteamLabelsBits 站方特化经 ExtraFields 通道翻译投递（executor
//     装配段注入+adapter 透传）。
//   - **anonymous 必填 boolean**（spec required）：adapter 已修（false
//     显式写——缺字段 API 报必填错）。
//   - 字段：category/medium/videoCodec/audioCodec/standard/team/
//     smallDescr/descr/mediainfo+imdb+douban（dburl 域）；**无 uplver**
//     （anonymous 走 API boolean 非 NP checkbox）；bangumi（spec 新增）
//     发布链无域不配（熊猫 tmdb 定案）；offer 候选 ID 非候选流程不写；
//     nfo/tags 自定义字符串不投递。
// 站规人工把关：上传规则（§15——双倍上传/候选审核/软件游戏病毒扫描）+
//   标题规范（§14 wiki 分模板）+100GB 音乐区发布码+成人 15 叶发布层
//   拦截在前（§59.222）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(133, "mteam_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'api.m-team.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'api.m-team.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "category", "medium": "medium", "codec": "videoCodec", "audiocodec": "audioCodec", "standard": "standard", "team": "team", "small_descr": "smallDescr", "description": "descr", "technical_info": "mediainfo", "imdb_url": "imdb", "dburl": "douban"}, "framework": "mteam", "value_mappings": {"type": [{"label": "电影/SD", "value": "401"}, {"label": "电影/HD", "value": "419"}, {"label": "电影/DVDiSo", "value": "420"}, {"label": "电影/BluRay", "value": "421"}, {"label": "电影/Remux", "value": "439"}, {"label": "影剧/综艺/HD", "value": "402"}, {"label": "影剧/综艺/SD", "value": "403"}, {"label": "影剧/综艺/DVDiSo", "value": "435"}, {"label": "影剧/综艺/BluRay", "value": "438"}, {"label": "动画/BluRay", "value": "453"}, {"label": "TV遊戲", "value": "448"}, {"label": "纪录", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "演唱", "value": "406", "standard_keys": ["category.concert"]}, {"label": "运动", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Music(无损)", "value": "434", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Misc(其他)", "value": "409", "standard_keys": ["category.other"]}, {"label": "软件", "value": "422", "standard_keys": ["category.software"]}, {"label": "電子書", "value": "427", "standard_keys": ["category.ebook"]}, {"label": "有聲書", "value": "442", "standard_keys": ["category.audiobook"]}, {"label": "教育影片", "value": "451", "standard_keys": ["category.education"]}, {"label": "PC游戏", "value": "423", "standard_keys": ["category.game"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "Web-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "H.264(x264/AVC)", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265(x265/HEVC)", "value": "16", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "AV1", "value": "19", "standard_keys": ["video.av1"]}, {"label": "VP8/9", "value": "21", "standard_keys": ["video.vp9"]}, {"label": "AVS", "value": "22", "standard_keys": ["video.avs"]}], "audiocodec": [{"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "AC3(DD)", "value": "8", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "DTS-HD MA", "value": "11", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "E-AC3(DDP)", "value": "12", "standard_keys": ["audio.ddp"]}, {"label": "E-AC3 Atmos(DDP Atmos)", "value": "13", "standard_keys": ["audio.ddp_atmos"]}, {"label": "TrueHD", "value": "9", "standard_keys": ["audio.truehd"]}, {"label": "TrueHD Atmos", "value": "10", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "LPCM/PCM", "value": "14", "standard_keys": ["audio.lpcm", "audio.pcm"]}, {"label": "WAV", "value": "15", "standard_keys": ["audio.wav"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "MP2/3", "value": "4", "standard_keys": ["audio.mp3", "audio.mp2"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "5", "standard_keys": ["resolution.sd"]}, {"label": "4K", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "8K", "value": "7", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "MTeam", "value": "9"}, {"label": "TnP", "value": "23"}, {"label": "MWeb", "value": "44"}, {"label": "TPTV", "value": "43"}, {"label": "BMDru", "value": "6"}, {"label": "CNHK", "value": "19"}, {"label": "Pack", "value": "8"}, {"label": "CatEDU", "value": "25"}, {"label": "ARiC", "value": "26"}, {"label": "7³ACG", "value": "30"}, {"label": "JKCT", "value": "31"}, {"label": "G00DB0Y", "value": "35"}, {"label": "D0", "value": "36"}, {"label": "ZZH", "value": "63"}, {"label": "DST", "value": "57"}, {"label": "HBO", "value": "40"}, {"label": "REE", "value": "41"}, {"label": "Aisha", "value": "65"}, {"label": "CTRL", "value": "45"}, {"label": "StarfallWeb", "value": "59"}, {"label": "ZTR", "value": "48"}, {"label": "126811", "value": "49"}, {"label": "RRS", "value": "61"}, {"label": "lijiang-tv", "value": "62"}, {"label": "DStudio", "value": "64"}]}}`).Error
	})
}
