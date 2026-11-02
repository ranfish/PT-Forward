package db

import (
	"gorm.io/gorm"
)

// migration114 §59.430: OKPT（www.okpt.net）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（53.2KB 直连）+ docs/32/okpt.md
// 880 行读全（论坛 topicid=86 站规完整采集+发种规范五帖）。
// 实时 vs 4 月资产三处漂移（实时为准）：medium 新增 Web(20)+team 新增
// Audiences(13)/Other(14)+**无 uplver**（文档 §2.1 称有——实时零命中
// checkbox 全为 tags；无匿名字段 anonymous 无承载点）。
// 槽位映射（双模式站 mode=4 影视/5 音乐——§59.372 id=specialcat 判别
// +§59.388 模式互斥过滤自动护；发布链走 mode=4 影视区，音乐类种子
// type miss 落空预览人工——我的PT 同款定案）：
//   - type 13：**440 短剧第十三站**+434 电子书=[ebook]+432 有声书=
//     [audiobook]+431 软件+413 游戏+404 双键+436 漫画书 label-only
//     （dict 无 comic 键——分享站同款）。
//   - medium 15 乱序（16 Other 首+Web(20) 尾实时新增）：Remux=[remux,
//     uhd_remux] 双键归并（无独立 UHD 项——好大/浦园先例）+SACD=
//     [sacd]+Vinyl=[vinyl]（extended 活键——农场/躺平先例）+HDCD/
//     HI-RES/Web label-only（无键/语义存疑——Web 与 WEB-DL 区分不明）。
//   - codec 13 视频编码+文档类混入：HEVC/H.265/x265=[h265]+AVC/H.264/
//     x264=[h264]（合并命名+CLI 词在 label——压制联动精确匹配不触发=
//     站方合并项语义，好大第六站反证同款）+VVC/AV1/VP9/Other 配键+
//     **文档类 7 项 label-only**（EPUB/AZW3/PDF/TXT/ZIP——电子书编码
//     发布链零产出）。
//   - audiocodec 17 影视+音乐系混入：DTS:X=[dtsx]+DTS-HD=[dts_hd_ma,
//     dts_hd_hr] 双键（优堡/垃圾堆先例）+TrueHD=[truehd,truehd_atmos]
//     双键家族承接（柠檬/好多油同款）+DD/DD+=[ac3,dd,ddp] 三键合并+
//     LPCM=[lpcm,pcm] 双键承接+MP3/MPEG/APE/WAV/FLAC 分轨/DTS/AAC/
//     Other 配键+**整轨/分轨 3 项 label-only**（镜像整轨/WAV 整轨/DSF
//     分轨——音乐形态发布链无产出）。
//   - standard 6：8K/4K/1080p·i=[r1080p,r1080i] 双键合并（文档 §4.3）
//     /720p/Other 全配。
//   - team 29 **外站官组表最大站**（26 外站组+OKPT/Audiences）：全部
//     label-only（组映射层 label 直配自动命中——学校站 6 组四倍扩展）
//     +Other(14)=[team.other] 兜底（实时新增——资产漏）；**黑名单 9
//     组人工规避**（FGT/RARBG/Mp4Ba/xiaomi/huawei/BlackTV/BitsTV/
//     DreamHD/Hao4K——文档 §4.6）。
//   - tags 18 配键 11：完结=[complete]/分集=[ongoing]（三站先例）/
//     国语/粤语/英语/中字/英字/DIY/特效=[special_effects_subs]（星陨
//     阁首配+天空定案）/HDR 四键泛家族（15 站）/DoVi=[dolby_vision]
//     （11 站）；**禁转不配+驻站/自购 label-only**（官方专属——§1.8
//     转载不得勾禁转/首发/官方）+日语/韩语 label-only（推断链零产出
//     ——历史无站配）+Atmos 不配（extended 零产出——红豆饭实证定案）+
//     资料教程 label-only。
//   - processing_sel[4]=产区槽不配（8 产区含印度/东南亚——发布链无
//     投递域历史定案持续；预览人工）。
//   - 字段：url（data-pt-gen IMDb 真字段）+technical_info（MediaInfo
//     独立字段）+无 pt_gen/nfo/dburl（文档实证）。
// 站规人工把关：无重复判定（多小组共存）+H&R 全站强制+吸血降级
// Peasant+简介严格顺序+转载出处按制作组源站+黑名单组禁转。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(114, "okpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.okpt.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.okpt.net'`, `{"enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "technical_info": "technical_info"}, "framework": "np", "value_mappings": {"type": [{"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "短剧", "value": "440", "standard_keys": ["category.short_drama"]}, {"label": "动漫/动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "综艺/真人秀", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "电子书", "value": "434", "standard_keys": ["category.ebook"]}, {"label": "有声书", "value": "432", "standard_keys": ["category.audiobook"]}, {"label": "漫画书", "value": "436"}, {"label": "游戏", "value": "413", "standard_keys": ["category.game"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "软件", "value": "431", "standard_keys": ["category.software"]}, {"label": "其它", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "其他（Other）", "value": "16", "standard_keys": ["medium.other"]}, {"label": "UHD Blu-ray", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "SACD", "value": "15", "standard_keys": ["medium.sacd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Vinyl", "value": "17", "standard_keys": ["medium.vinyl"]}, {"label": "HDCD", "value": "18"}, {"label": "HI-RES", "value": "19"}, {"label": "Web", "value": "20"}], "codec": [{"label": "EPUB/AZW3/MOBI", "value": "21"}, {"label": "AZW3/MOBI", "value": "19"}, {"label": "PDF", "value": "16"}, {"label": "EPUB", "value": "17"}, {"label": "ZIP", "value": "20"}, {"label": "TXT", "value": "15"}, {"label": "HEVC/H.265/x265", "value": "11", "standard_keys": ["video.h265"]}, {"label": "AVC/H.264/x264", "value": "2", "standard_keys": ["video.h264"]}, {"label": "H.266/VVC", "value": "10", "standard_keys": ["video.vvc"]}, {"label": "AV1", "value": "7", "standard_keys": ["video.av1"]}, {"label": "VP9", "value": "12", "standard_keys": ["video.vp9"]}, {"label": "Other", "value": "14", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS:X", "value": "22", "standard_keys": ["audio.dtsx"]}, {"label": "MPEG", "value": "14", "standard_keys": ["audio.mpeg"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "APE", "value": "5", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "20", "standard_keys": ["audio.wav"]}, {"label": "FLAC 分轨", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "LPCM", "value": "16", "standard_keys": ["audio.lpcm", "audio.pcm"]}, {"label": "TrueHD", "value": "19", "standard_keys": ["audio.truehd", "audio.truehd_atmos"]}, {"label": "DTS-HD", "value": "7", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "DD/DD+", "value": "15", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "镜像(Mirror) 整轨", "value": "23"}, {"label": "Other", "value": "21", "standard_keys": ["audio.other"]}, {"label": "WAV 整轨", "value": "24"}, {"label": "DSF 分轨", "value": "25"}], "standard": [{"label": "8K", "value": "1", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/2160p", "value": "2", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/1080i", "value": "3", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p", "value": "4", "standard_keys": ["resolution.r720p"]}, {"label": "Other", "value": "10", "standard_keys": ["resolution.other"]}], "team": [{"label": "ZmPT", "value": "29"}, {"label": "Ying", "value": "28"}, {"label": "U2", "value": "32"}, {"label": "UBits", "value": "27"}, {"label": "TTG", "value": "26"}, {"label": "Rousi", "value": "24"}, {"label": "PTHome", "value": "22"}, {"label": "Red Leaves", "value": "23"}, {"label": "PterClub", "value": "21"}, {"label": "Panda", "value": "5"}, {"label": "OurBits", "value": "2"}, {"label": "OKPT", "value": "3"}, {"label": "M-Team", "value": "12"}, {"label": "LemonHD", "value": "11"}, {"label": "HD4FANS", "value": "1"}, {"label": "HDSky", "value": "10"}, {"label": "HDHome", "value": "6"}, {"label": "HDDollby", "value": "9"}, {"label": "HDChina", "value": "8"}, {"label": "HHClub", "value": "4"}, {"label": "FRDS", "value": "20"}, {"label": "DaJiao", "value": "19"}, {"label": "CMCT", "value": "16"}, {"label": "CHDBits", "value": "17"}, {"label": "BTSchool", "value": "18"}, {"label": "BeiTai", "value": "31"}, {"label": "Audiences", "value": "13"}, {"label": "Other", "value": "14", "standard_keys": ["team.other"]}], "tags": [{"label": "禁转", "value": "1"}, {"label": "驻站", "value": "56"}, {"label": "自购", "value": "11"}, {"label": "完結", "value": "51", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "50", "standard_keys": ["tag.ongoing"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "45", "standard_keys": ["tag.cantonese_audio"]}, {"label": "英语", "value": "57", "standard_keys": ["tag.english_audio"]}, {"label": "日语", "value": "28"}, {"label": "韩语", "value": "25"}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "英字", "value": "58", "standard_keys": ["tag.english_subtitle"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "特效", "value": "12", "standard_keys": ["tag.special_effects_subs"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "DoVi", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "Atmos", "value": "53"}, {"label": "资料教程", "value": "31"}]}}`).Error
	})
}
