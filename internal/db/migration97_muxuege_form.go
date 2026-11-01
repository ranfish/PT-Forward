package db

import (
	"gorm.io/gorm"
)

// migration97 §59.413: 慕雪阁（pt.muxuege.org）发布表单配置分发。
// 依据：站方 upload.php?mode=advanced 实时 HTML（54.1KB 直连——**咖啡式
// 简版首例复现**：默认页 17KB 仅基础字段，完整表单在 mode=advanced）+
// docs/32/muxuege.md 600 行读全。**实时 vs 文档大改版**（4 月→11 月）：
//   - medium 10→6 项大砍（**无 Encode/DVDR/CD/Track**——encode/dvdr/
//     dvd/cd/track 键 miss 落空预览人工）+HDDVD label 无空格化
//   - team 47→7 项重组（站方全自家组：MxWeb 网络视频搬运组/慕雪阁
//     音乐组/软件搬运组/教育资源搬运组/压缩组/乐府+**Other 兜底**——
//     FRDS 不在表落 Other）
//   - **audiocodec 新增槽**（文档称无——漂移）：9 项含 **AAC 6/14 重复
//     值条目**（克隆式去重留首值 6）+杜比全景声(15)/全景声(19) 同语义
//     双项（15 配 [atmos,truehd_atmos]+19 label-only 防一键多值）；
//     **无 FLAC/TrueHD/DDP/AC3**（miss 落空）
//   - standard 5 项：**新增 4320p(7)**+删 SD（sd miss 落空）+540p
//     label-only（无键）；无 1080i
//   - type 19 项：**新增 421 源码**（label-only）+416 短剧第九站+
//     406 Music Videos 三键+418 广播剧/414 图片/410 系统镜像 label-only
//   - codec 9 项：H264/H265 简短命名（站方去 & 后缀——无 X26x 联动
//     零触发）+AV1/VP9+HDR 10 label-only（画质属性非编码——codec 域
//     不产出 HDR 键）；**TXT/PDF 已删**
//   - **processing_sel=年份槽**（第四种语义特化站：咖啡地区/YHPP 地区/
//     垃圾堆文件格式/慕雪阁发布年代 90 年代之前→2027 逐年）——发布链
//     无年份投递维度不配（按需生长）
//   - tags 34 项（实时新增禁转 60/60Fps 59/Autodesk 58）：配键 8——
//     **禁转=[restricted]**+**杜比视界=[dolby_vision] 第四站**+
//     **原盘=[original_disc]**+**HDR 真彩=[hdr_vivid]**（真彩=Vivid
//     中文译名）+DIY/国语/完结/分集；label-only 26 项
//   - 字段：url+**pt_gen（文档称无——漂移修正）**+**technical_info
//     （文档称无——漂移修正）**+descr；nfo 不配（13城教训）；无 uplver。
// 站规同垃圾堆系（<100MB 禁/upscale 禁/RMVB 禁/游戏候选制/Dupe
// 媒介优先级+断种豁免）——发布预览人工把关。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(97, "muxuege_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.muxuege.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.muxuege.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "系统镜像", "value": "410"}, {"label": "游戏", "value": "411", "standard_keys": ["category.game"]}, {"label": "电子书", "value": "412", "standard_keys": ["category.ebook"]}, {"label": "教育", "value": "413", "standard_keys": ["category.education"]}, {"label": "图片", "value": "414"}, {"label": "软件", "value": "415", "standard_keys": ["category.software"]}, {"label": "短剧", "value": "416", "standard_keys": ["category.short_drama"]}, {"label": "有声书", "value": "417", "standard_keys": ["category.audiobook"]}, {"label": "广播剧", "value": "418"}, {"label": "源码", "value": "421"}], "medium": [{"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HDDVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "H264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H265", "value": "6", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "AV1", "value": "7", "standard_keys": ["video.av1"]}, {"label": "VP9", "value": "8", "standard_keys": ["video.vp9"]}, {"label": "HDR 10", "value": "9"}], "audiocodec": [{"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts", "audio.dtsx", "audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "杜比全景声", "value": "15", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "全景声", "value": "19"}], "standard": [{"label": "540p", "value": "6"}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "4320p", "value": "7", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "MxWeb 网络视频搬运组", "value": "7"}, {"label": "慕雪阁音乐组", "value": "48"}, {"label": "慕雪阁软件搬运组", "value": "49"}, {"label": "慕雪阁教育资源搬运组", "value": "50"}, {"label": "慕雪阁压缩组", "value": "51"}, {"label": "乐府", "value": "53"}, {"label": "Other", "value": "52"}], "tags": [{"label": "珍宝楼", "value": "33"}, {"label": "乐府", "value": "30"}, {"label": "高码率", "value": "41"}, {"label": "DIY", "value": "37", "standard_keys": ["tag.diy"]}, {"label": "压制", "value": "36"}, {"label": "原盘", "value": "27", "standard_keys": ["tag.original_disc"]}, {"label": "杜比全景声", "value": "35"}, {"label": "杜比视频", "value": "34"}, {"label": "杜比视界", "value": "25", "standard_keys": ["tag.dolby_vision"]}, {"label": "完结", "value": "26", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "12", "standard_keys": ["tag.ongoing"]}, {"label": "完结日漫", "value": "11"}, {"label": "完结国漫", "value": "8"}, {"label": "禁转", "value": "60", "standard_keys": ["tag.restricted"]}, {"label": "60Fps", "value": "59"}, {"label": "Autodesk", "value": "58"}, {"label": "NovaHD", "value": "57"}, {"label": "linux", "value": "56"}, {"label": "Windows", "value": "55"}, {"label": "Mac", "value": "54"}, {"label": "H.265", "value": "53"}, {"label": "H.264", "value": "52"}, {"label": "4k", "value": "51"}, {"label": "1080p", "value": "50"}, {"label": "纯净版", "value": "49"}, {"label": "HDR 真彩", "value": "48", "standard_keys": ["tag.hdr_vivid"]}, {"label": "古装", "value": "47"}, {"label": "菜单修改", "value": "46"}, {"label": "自购", "value": "45"}, {"label": "特效字慕", "value": "42"}, {"label": "生肉", "value": "40"}, {"label": "国语", "value": "38", "standard_keys": ["tag.chinese_audio"]}, {"label": "大包", "value": "32"}, {"label": "国风音乐", "value": "10"}]}}`).Error
	})
}
