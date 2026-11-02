package db

import (
	"gorm.io/gorm"
)

// migration132 §59.449: 他吹吹风（et8.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（48.4KB 代理）+ docs/32/et8.md 410 行
// 读全（TTG 系教育向老牌站——rules 完整+教育/学习类特殊要求）。
// **实时 vs 资产零漂移**（type 9/medium 16/codec 8/standard 5/team 7/
// source_sel 14 全槽一致——零漂移站）；tags 资产 None=站方真无（实证）。
// 站点形态：**裸名字段站**（五二同款——type/source_sel/medium_sel/
// codec_sel/audiocodec_sel/standard_sel/team_sel 全裸名无 [4] 后缀）+
// **无 pt_gen**（实证——站规推荐 PTgen 生成后贴 descr 简介）+**无
// technical_info**（MI 塞简介——站规"视频必须包含截图或 MediaInfo
// 二选一"）+**无 tags 槽**。
// 槽位映射：
//   - **type 9 教育向非标值集**（无电影/剧集/综艺——站规影视剧全站
//     禁止）：624 纪录片双键+628 杂项学习=[education]（教育泛类唯一
//     归位）+629 电子书/小说=[ebook]+632 漫画=[comic]（dict 无
//     manga 用 comic）+633 有声书=[audiobook]；**630 电子书/非小说
//     +634 公开课+635 视频教程 label-only**（ebook/education 键已
//     归位防一键多值——细分无 canonical 键预览人工）+631 杂志
//     label-only（无键）。
//   - medium 16 视频×文档混合（猫站同族）：UHD Bluray/BluRay 分立
//     各单键+Remux 双键归并+Encode+WEB-DL 双键+HDTV+**HDRip(3)
//     label-only（无键）**+**DVDR(7)=[dvd,dvdr] 双键宽语义（阿玲
//     先例——无 DVD 项用 DVDR 项收 dvd 键）**+DVDRip(4)=[dvdrip]
//     （dict 键）+Other+**文档族六项：PDF(12)=[pdf]/AZW3(14)=
//     [azw]/MOBI(15)=[mobi] ext 活键（猫站首配同款）+EPUB(13)/
//     TXT(16)/Pictures(17) label-only（无键）**。
//   - **codec 8 H/X 四分立压制联动站**（天枢同款）：H.264(1)=[h264]
//     +x264(7) label-only（§59.380 联动通道——压制种 x264 联动命中）
//     +H265/HEVC(8)=[h265]+x265(6) label-only（联动通道）+VC-1+
//     Xvid(3)=[xvid]+MPEG-2+Other。
//   - audiocodec 11：**AC3(4)=[ac3,dd,ddp] 三键承接（大青虫先例
//     ——无 DD/DDP 分立项）**+**DTS-HD(8)=[dts_hd_ma,dts_hd_hr]
//     双键承接（合并项无细分——车站/红豆饭先例）**+FLAC/APE/DTS/
//     MP3/AAC/TrueHD/LPCM/WAV/Other。
//   - standard 5：2160/4K(5)=[r2160p]+1080p/i 分立+720p+SD(4)=
//     [sd]；无 Other 项。
//   - team 7：TorrentCCF/TLF/BMDru/CatEDU/MADFOX 老牌组 label-only
//     +**个人原创资源(6) 绝不配**（原创官方专属——我堡"原创/原抓"
//     同款）+其他(7)=[team.other] 兜底。
//   - **source_sel 14=学科槽不配**（**NP 同名字段语义第五变**：产区/
//     文件格式/年份/教育年级/学科——发布链无学科投递能力预览人工；
//     域识别表无 source_sel 解析跳过 formdiff 零差异咖啡同构）。
//   - 字段（裸名站）：type/medium_sel/codec_sel/audiocodec_sel/
//     standard_sel/team_sel/small_descr/descr+url（IMDb——label
//     "IMDb链接"实证）+uplver；**无 pt_gen/technical_info**（实证）
//     +nfo 不配（13城先例）。
// 站规人工把关：**影视剧类全站禁止**（稀有/有价值例外）+FGT 小组
//   黑名单+mp4ba 水印禁+合集优先有合集不发单集+CAM/TS/SCR/R5 低画质
//   禁+RAR 分卷（Scene 影视须解压）+教育类简介开头必须一张图+视频类
//   MI/截图二选一+英文 0day 主标题+中文副标题+首发组专用
//   [TorrentCCF首发] 标记。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(132, "et8_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'et8.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'et8.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Documentaries.纪录片", "value": "624", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Elearning - 杂项学习", "value": "628", "standard_keys": ["category.education"]}, {"label": "Elearning - 电子书/小说", "value": "629", "standard_keys": ["category.ebook"]}, {"label": "Elearning - 电子书/非小说", "value": "630"}, {"label": "Elearning - 杂志", "value": "631"}, {"label": "Elearning - 漫画", "value": "632", "standard_keys": ["category.comic"]}, {"label": "Elearning - 有声书", "value": "633", "standard_keys": ["category.audiobook"]}, {"label": "Elearning - 公开课", "value": "634"}, {"label": "Elearning - 视频教程", "value": "635"}], "medium": [{"label": "UHD Bluray", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "BluRay", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "5", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "11", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "9", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "6", "standard_keys": ["medium.hdtv"]}, {"label": "HDRip", "value": "3"}, {"label": "DVDR", "value": "7", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "DVDRip", "value": "4", "standard_keys": ["medium.dvdrip"]}, {"label": "Other", "value": "8", "standard_keys": ["medium.other"]}, {"label": "PDF", "value": "12", "standard_keys": ["medium.pdf"]}, {"label": "EPUB", "value": "13"}, {"label": "AZW3", "value": "14", "standard_keys": ["medium.azw"]}, {"label": "MOBI", "value": "15", "standard_keys": ["medium.mobi"]}, {"label": "TXT", "value": "16"}, {"label": "Pictures", "value": "17"}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "x265", "value": "6"}, {"label": "x264", "value": "7"}, {"label": "H265/HEVC", "value": "8", "standard_keys": ["video.h265"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "AC3", "value": "4", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "MP3", "value": "5", "standard_keys": ["audio.mp3"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "DTS-HD", "value": "8", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "TrueHD", "value": "9", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "10", "standard_keys": ["audio.lpcm"]}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}], "standard": [{"label": "2160/4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}], "team": [{"label": "TorrentCCF/TCCF", "value": "1"}, {"label": "TLF", "value": "2"}, {"label": "BMDru", "value": "3"}, {"label": "CatEDU", "value": "4"}, {"label": "MADFOX", "value": "5"}, {"label": "个人原创资源(Original)", "value": "6"}, {"label": "其他(other)", "value": "7", "standard_keys": ["team.other"]}]}}`).Error
	})
}
