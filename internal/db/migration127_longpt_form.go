package db

import (
	"gorm.io/gorm"
)

// migration127 §59.444: 龙PT（longpt.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（56.9KB 直连）+ docs/32/longpt.md
// 426 行读全（促销+上传+Dupe+打包规则完整采集）。
// **实时 vs 资产差异**：**tags 18 项实时新增 官方(3)+零魔(18)**
// （资产 16 项——官方绝不配+零魔 label-only 麒麟站方标记同款）+
// standard label 实时扩充 i 形态（4K/2160p→4K/2160p/2160i 三元
// 合并）+medium 顺序漂移；**type 409/411 双"其他"同 label 双 value
//     重复项**（label 重复新形态——历史 FRDS 是同 value 双 label；
//     draft 解析器同 label 去重保 HTML 首见 409，配置对齐 draft
//     单行口径删 411——formdiff 抓出 changed 即修）。
// 槽位映射：
//   - type 11：410 有声书+408 音频=[music,lossless_music] 双键+
//     406 音乐视频=[mv,music_video] 双键+404 双键（409=[other]
//     配键，411 重复项删除）。
//   - **medium 11 原盘/Remux 双单键分流（双键路径各自承接无一键
//     多值）**：UHD Blu-ray(2)=[uhd_bluray]+Blu-ray(1)=[bluray]
//     原盘分立+**UHD Blu-ray Remux(11)=[uhd_remux]+Blu-ray
//     Remux(3)=[remux] 双单键分流**（DryRun 形态二实测：UHD
//     Remux 标题判定层"UHD REMUX→remux"归一损失（§59.420 家园
//     注释明证）→命中 3 历史站普遍形态；11 承接 uhd_remux 键
//     精确路径——两键无冲突，Remux 双键归并反而制造一键多值）+WEB-DL(4)=
//     [webdl,webrip] 双键承接+**DVD(6)=[dvd,dvdr] 双键宽语义**
//     （无 DVDR 项垃圾堆同款）+Encode/HDTV/CD/Track/Other。
//   - codec 6 全配键（合并命名+AV1）——无 X26x 零联动。
//   - **standard 7 p/i 三元合并形态**（2K/1440p/1440i 等全槽合并
//     ——咖啡 p/i 合并放大版）：1080p/1080i=[r1080p,r1080i] 双键
//     +4K/8K/2K/720p/480p 主键单键（i 变体无 ext 键 miss 落空——
//     织梦 r720i 同款）+Other=[resolution.other]。
//   - **audiocodec 18 全配键**：TrueHD Atmos(9)=[atmos,
//     truehd_atmos] 双键+TrueHD(19) 分立+**E-AC3(DDP)(10)=[ddp]
//     单键+AC3(15)=[ac3] 单键（DDP/AC3 分立不互承接）**+DTS:X(12)
//     = [dtsx]+DTS(13)+DTS-HD MA(3)+LPCM+**M4A/ALAC/AV3A ext
//     活键**（躺平/麒麟先例）+FLAC/APE/MP3/OGG/AAC/WAV/Other。
//   - team 8：**LongA/LongWeb/LongPT 站方官组 label-only**+WiKi/
//     RL/CMCT/HHWEB 外站 label-only+Other(5)=[team.other] 兜底。
//   - **tags 18 长尾键丰收第二站**：**臻彩MAX(14)=[hdr_vivid]
//     （HDR Vivid 中文品牌名第四站——优堡"菁彩HDR"/慕雪阁"真彩"/
//     熊猫"菁彩HDR"同键不同译名先例）+高码(15)=[high_bitrate]+
//     高帧(16)=[high_frame_rate]**（星陨阁/劳改所/Nova 复利）+
//     杜比(10)=[dolby_vision]+特效(11)=[special_effects_subs]+
//     英字(9)+完结(8)+分集(12)+DIY/国语/中字/**HDR=[hdr10,
//     hdr10_plus,hlg] 三键泛剩余（hdr_vivid 让位臻彩MAX 独立项
//     ——包子/星湾精细化分配同款防一键多值）** 十键配；**禁转(1)/首发(2)/官方(3) 三绝不配**（财神雷区）+**零魔
//     (18) label-only（麒麟零魔站方标记同款）+高分(13) label-only
//     （评分语义无键）+去广告纯享版(17) label-only（无键）**。
//   - 字段：type/medium_sel[4]/codec_sel[4]/standard_sel[4]/
//     audiocodec_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
//     technical_info+url（IMDb）+uplver+anonymous:true；nfo 不配
//     （13城先例）；**无 pt_gen**；action/nonce/mode/redirect/
//     amount（魔改 UI 字段发布链不处理）+pos_state/pos_state_until
//     促销字段不配。
// 站规人工把关：上传者资格+Dupe 媒介优先级（§4.1）+同媒介同分辨率
//   重编码+豁免条款+资源打包规则（试行）+例外规则。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(127, "longpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'longpt.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'longpt.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音频", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "有声书", "value": "410", "standard_keys": ["category.audiobook"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "2", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray Remux", "value": "11", "standard_keys": ["medium.uhd_remux"]}, {"label": "Blu-ray Remux", "value": "3", "standard_keys": ["medium.remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "4", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "Other", "value": "10", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "2", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "3", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AV1", "value": "5", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "6", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS-HD MA", "value": "3", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "M4A", "value": "8", "standard_keys": ["audio.m4a"]}, {"label": "TrueHD Atmos", "value": "9", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "E-AC3(DDP)", "value": "10", "standard_keys": ["audio.ddp"]}, {"label": "Other", "value": "11", "standard_keys": ["audio.other"]}, {"label": "DTS:X", "value": "12", "standard_keys": ["audio.dtsx"]}, {"label": "DTS", "value": "13", "standard_keys": ["audio.dts"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "AC3", "value": "15", "standard_keys": ["audio.ac3"]}, {"label": "ALAC", "value": "16", "standard_keys": ["audio.alac"]}, {"label": "WAV", "value": "17", "standard_keys": ["audio.wav"]}, {"label": "AV3A", "value": "18", "standard_keys": ["audio.av3a"]}, {"label": "TrueHD", "value": "19", "standard_keys": ["audio.truehd"]}], "standard": [{"label": "2K/1440p/1440i", "value": "1", "standard_keys": ["resolution.r1440p"]}, {"label": "1080p/1080i", "value": "2", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p/720i", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "480p/480i", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "4K/2160p/2160i", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320p/4320i", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "7", "standard_keys": ["resolution.other"]}], "team": [{"label": "LongA", "value": "1"}, {"label": "LongWeb", "value": "2"}, {"label": "LongPT", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}, {"label": "RL", "value": "6"}, {"label": "CMCT", "value": "7"}, {"label": "HHWEB", "value": "8"}], "tags": [{"label": "禁转", "value": "1"}, {"label": "英字", "value": "9", "standard_keys": ["tag.english_subtitle"]}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hlg"]}, {"label": "零魔", "value": "18"}, {"label": "去广告纯享版", "value": "17"}, {"label": "高帧", "value": "16", "standard_keys": ["tag.high_frame_rate"]}, {"label": "高码", "value": "15", "standard_keys": ["tag.high_bitrate"]}, {"label": "臻彩MAX", "value": "14", "standard_keys": ["tag.hdr_vivid"]}, {"label": "高分", "value": "13"}, {"label": "分集", "value": "12", "standard_keys": ["tag.ongoing"]}, {"label": "特效", "value": "11", "standard_keys": ["tag.special_effects_subs"]}, {"label": "杜比", "value": "10", "standard_keys": ["tag.dolby_vision"]}, {"label": "完结", "value": "8", "standard_keys": ["tag.complete"]}]}}`).Error
	})
}
