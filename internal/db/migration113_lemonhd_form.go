package db

import (
	"gorm.io/gorm"
)

// migration113 §59.429: 柠檬不甜（lemonhd.net）发布表单配置分发。
// 依据：站方 upload.php?mode=advanced 实时 HTML（41.1KB 直连——默认页
// 13.2KB 简版咖啡/慕雪阁模式）+ docs/32/lemonhd.md 457 行读全。
// 实时 vs 4 月资产全槽一致（零漂移站）。
// 槽位映射：
//   - type 9 非标值集（402=Muisc 音乐/403=Animations 动漫/406=TV Series
//     剧集——非 401/405/407 惯例序）：402/404/405 双键+401/403/406/407/
//     409 直配；**408"3D" label-only**（实态矛盾仲裁：cat408 列表页 90
//     行全音乐 MV 内容（Sia/Beach Boys/FLAC 分轨）——表单 label 与站方
//     实态脱节；3D 电影实态在 401（IMAX 3D/HKG 3D 样本在列）——站方
//     用户不用 408 装 3D，§59.197 Stereo3D 无需路由）。
//   - **medium 10 纯来源形态语义（实态三铁证仲裁）**：Sneakers 2160p
//     UHD Blu-ray 原盘→媒介=Blu-ray(1)+分辨率=4K_UHD；Lock Up UHD
//     Blu-ray Remux→媒介=Remux(3)+分辨率=4K_UHD；Mutiny 1080p Blu-ray
//     原盘→媒介=Blu-ray(1)+分辨率=1080p/i——**UHD 维度由 standard 槽
//     承担，媒介不分 UHD**：Blu-ray=[bluray,uhd_bluray] 双键归并+
//     Remux=[remux,uhd_remux] 双键归并；**4K-UltraHD(6)/8K-UltraHD(7)
//     label-only**（语义存疑不猜——实态证明 UHD 原盘/Remux 均走 1/3）。
//   - codec 5 极简全配：H.264/AVC/H.265/HEVC/VC-1/MPEG-2/Other——无
//     X26x CLI 形态（压制联动零触发=站方语义）。
//   - audiocodec 13 全配键：**Atmos TrueHD(1)/TrueHD(2) 分立**+
//     DTS X(4)=[dtsx]（空格 label 形态——历史 13+ 站单键先例）+
//     AC3/DD=[ac3,dd] 双键+EAC3/DDP=[ddp] 单键（无 eac3 键）+
//     LPCM=[lpcm,pcm] 双键（藏宝阁 PCM=[lpcm] 承接先例——extended
//     活键）+AAC/FLAC/WAV/APE/Other。
//   - standard 5（4K_UHD 置顶乱序）：4K_UHD=[r2160p]+1080p/i=[r1080p,
//     r1080i] 双键+**720p/i=[r720p] 单键（r720i 无键——织梦同款）**+
//     SD=[sd]+Other=[resolution.other]（extended 活键——躺平先例）。
//   - team 6：LemonHD 官组+HDS/CHD/MySiLU/WiKi 外站官组 label-only
//     （组映射层自动命中——车站/学校先例）+Other(5)=[team.other] 兜底。
//   - tags 8：**首发(2) 绝不配键**（财神雷区铁律）+禁转(1) 不配+
//     4K(8) label-only（分辨率类标签零产出——13城/麒麟先例）+
//     DIY=[tag.diy]/国语=[tag.chinese_audio]/中字=[tag.chinese_subtitle]/
//     HDR 四键泛家族（14 站）/原盘=[tag.original_disc]（藏宝阁先例）。
//   - **无 url/pt_gen 真字段**（实时 mode=advanced 页实证——ptgen.js 仅
//     脚本引用；文档 §1.1/§1.2 url+PT-Gen 五来源为漂移/旧版形态，实时
//     为准）；nfo 不配；uplver+anonymous:true。
// 站规人工把关：禁转资源发布前获许可+同名资源≥6 月间隔+官方原盘优先+
// 禁抢发+候选区审核+标题规范+促销自动 2xFree 等。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(113, "lemonhd_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'lemonhd.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'lemonhd.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies(电影)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "Muisc(音乐)", "value": "402", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Animations(动漫/动画)", "value": "403", "standard_keys": ["category.animation"]}, {"label": "Music Videos(音乐视频)", "value": "404", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Documentaries(纪录片)", "value": "405", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "TV Series(电视剧)", "value": "406", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows(综艺)", "value": "407", "standard_keys": ["category.tv_shows"]}, {"label": "3D(3D视频)", "value": "408"}, {"label": "Other(其它)", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "Encode", "value": "2", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "WEB-DL", "value": "4", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "4K-UltraHD", "value": "6"}, {"label": "8K-UltraHD", "value": "7"}, {"label": "DVD", "value": "8", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "9", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "10", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "2", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "3", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "Atmos TrueHD", "value": "1", "standard_keys": ["audio.truehd_atmos"]}, {"label": "TrueHD", "value": "2", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HD MA", "value": "3", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS X", "value": "4", "standard_keys": ["audio.dtsx"]}, {"label": "DTS", "value": "5", "standard_keys": ["audio.dts"]}, {"label": "AC3/DD", "value": "6", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "EAC3/DDP", "value": "7", "standard_keys": ["audio.ddp"]}, {"label": "AAC", "value": "8", "standard_keys": ["audio.aac"]}, {"label": "LPCM", "value": "9", "standard_keys": ["audio.lpcm", "audio.pcm"]}, {"label": "FLAC", "value": "10", "standard_keys": ["audio.flac"]}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}, {"label": "APE", "value": "12", "standard_keys": ["audio.ape"]}, {"label": "Other", "value": "13", "standard_keys": ["audio.other"]}], "standard": [{"label": "4K_UHD", "value": "1", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/i", "value": "2", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p/i", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "5", "standard_keys": ["resolution.other"]}], "team": [{"label": "LemonHD", "value": "6"}, {"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "4K", "value": "8"}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "原盘", "value": "9", "standard_keys": ["tag.original_disc"]}]}}`).Error
	})
}
