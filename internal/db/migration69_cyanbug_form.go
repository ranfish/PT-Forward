package db

import (
	"gorm.io/gorm"
)

// migration69 §59.383: 大青虫（cyanbug.net）发布表单配置分发。
// 依据 docs/32/cyanbug.md（519 行读全）+ sites.json form 资产。
// 背景：§59.369 刷正转载站（18 制作组全为它站官组 is_source=false）——
// 目标站适配官组可选 ✓。
// 槽位特点（本站为标准 medium_sel 形态+全配站）：
//   - **medium_sel 12 项全部有 canonical 键**（首个零 label-only 媒介站）：
//     含 MiniBD(4)→medium.minibd（§59.325 精确键先例）/HD DVD(2)/DVDR(6)/
//     Track(9)/Other(12)——唯一留白的是语义已全覆盖
//   - codec 仅 7 项：无 AV1（不配）/无 X26x CLI 形态（§59.380 压制联动
//     自然零触发）——MPEG-4(7) 独立项
//   - audiocodec 13 项**无 DDP/E-AC3 项**（§4.7）——DDP 并入 DD/AC3(8)
//     [dd,ac3,ddp] 三键；DTS-HDMA:X 7.1(11) 组合形态双键 [dts_hd_ma,dtsx]
//   - standard 仅 5 项无 1440p/8K/Other——resolution.r1440p/r4320p/other
//     miss 落空（发布链 lookup miss 不填——站方无对应选项可接受）
//   - team 18 项（FRDS/HHWEB/ADWeb 等外站组丰富）全 label-only
//   - tags 7 项：首发(2) label-only（铁律）+HDR 10(7)/Dolby Vision(8)
//     label-only（无 canonical 键）；禁转/国语/中字/DIY 配键
//   - **标题禁止中文**（§4.1 站规）——发布链重组 v1.05 英文标题天然合规
//   - type 408 音轨→category.lossless_music（无音乐类——音轨即无损音乐）
//   - 分叉族双键即防：404 纪录片+406 MV
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(69, "cyanbug_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'cyanbug.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'cyanbug.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音轨", "value": "408", "standard_keys": ["category.lossless_music"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "UHD Blu-ray", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Other", "value": "12", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H.265", "value": "6", "standard_keys": ["video.h265"]}, {"label": "MPEG-4", "value": "7", "standard_keys": ["video.mpeg4"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "DD/AC3", "value": "8", "standard_keys": ["audio.dd", "audio.ac3", "audio.ddp"]}, {"label": "WAV", "value": "9", "standard_keys": ["audio.wav"]}, {"label": "TrueHD", "value": "10", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HDMA:X 7.1", "value": "11", "standard_keys": ["audio.dts_hd_ma", "audio.dtsx"]}, {"label": "DTS-HDMA", "value": "12", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "Atmos", "value": "13", "standard_keys": ["audio.atmos", "audio.truehd_atmos", "audio.ddp_atmos"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "DTS-HD", "value": "15", "standard_keys": ["audio.dts_hd"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "CMCT", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5"}, {"label": "MTean", "value": "6"}, {"label": "PTer", "value": "7"}, {"label": "FRDS", "value": "8"}, {"label": "Audies", "value": "9"}, {"label": "BeiTai", "value": "10"}, {"label": "HHWEB", "value": "11"}, {"label": "HDC", "value": "12"}, {"label": "OurBits", "value": "13"}, {"label": "HDH", "value": "14"}, {"label": "Pter", "value": "15"}, {"label": "LeagueWEB", "value": "16"}, {"label": "SharkWEB", "value": "17"}, {"label": "ADWeb", "value": "18"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR 10", "value": "7"}, {"label": "Dolby Vision", "value": "8"}]}}`).Error
	})
}
