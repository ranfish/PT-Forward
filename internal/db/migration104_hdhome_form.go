package db

import (
	"gorm.io/gorm"
)

// migration104 §59.420: 家园（hdhome.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（48.1KB 代理抓取）+ docs/32/hdhome.md
// 770 行读全（rules.php+H&R §三+候选制+标题规范+杂项概不接受）。
// 实时 vs 文档+资产：全槽一致；**文档 §6.2"49 个"计数错误**（种子区
// 实际 63 项——实时与文档表格逐项一致，标题计数错）；type 双区
// （id=browsecat 种子区 63+specialcat LIVE 27 同 name=type——§59.372+
// §59.388 双防线自动护首见种子区）；**无 pt_gen/technical_info**（实证）；
// **tags 字符串值**（yc/gy/db 等非数字 ID——第三个非标准值形态）。
// 槽位映射：
//   - **type 63 项全 label-only + hdhomeCategoryValue 三维词形路由**
//     （typeMappingOf 第四站——好大模式第三例放大版）：站方 type=类型×
//     媒介×分辨率三维编码（label "{Prefix} {Medium/Res}" 统一结构），
//     form_config 单键单值不可表达——label 词形路由（非硬编码 value：
//     站方改值不破+错拼容错 TVSereis(453)+TVShows(452) 复数+Sport(511)
//     单数）；prefix 9 类（Movies/TVSer/TVShow/Doc/Anime/Musics/Sport/
//     MV）+suffix 矩阵（原盘 8K/4K/BD 三分+Remux 按分辨率分发 UHD/8K
//     维度+压制按分辨率+音乐 CD 子路由 FLAC/APE）；education/ebook/
//     software/game/concert 无独立项→通用链落空（杂项站方概不接受+
//     演唱会无分类——预览人工）；测试锚 24 态。
//   - medium 8 项全配键：UHD Blu-ray 独立/Remux=[remux,uhd_remux]
//     双键归并（medium 无独立 UHD Remux 项）/WEB-DL 承接 webrip/
//     MiniBD 精确键；无 DVD/DVDR/HD DVD/Track/Other（miss 落空）。
//   - codec 5 项全配键：**AVC/H264/x264 三重合并命名**（label 含 x264
//     ——压制联动 label 精确匹配不触发=合并语义站反证第六站）+
//     HEVC/H265/x265 合并+VC-1/MPEG-2/Other；无 AV1/VP9/Xvid。
//   - standard 6 项 p/i 分立全配键：2160p/4K=[r2160p]+4320p/8K=
//     [r4320p]+1080p/1080i/720p/SD（r4320p 首次位于 standard 槽位）。
//   - audiocodec 13 项：AAC/AC3-DD=[ac3,dd] 双键/APE/WAV/FLAC/DTS/
//     TrueHD/LPCM/DTS-HDMA/DTS-HDHRA=TrueHD Atmos=[atmos,
//     truehd_atmos] 双键/**DTS-HDMA:X 7.1 label-only**（组合粒度
//     保守——好大/冬樱先例）/Other。
//   - team 14 项 label-only：HDHome 系官组（HDHome/HDH/HDHTV/HDHPad/
//     HDHWEB）+TTG/M-Team+**Other(11) 兜底**；FRDS 不在表。
//   - source_sel 6 项不配（媒介来源同名异义——咖啡定案；与 medium_sel
//     双槽冗余）；processing_sel 2 项 Raw/Encode 不配（壹吧先例）。
//   - **tags 18 项字符串值配键 10**：国语/中字/粤语/完结/连载=[ongoing]/
//     DIY/**DOLBY VISION=[dolby_vision] 第九站**/**HDR10/HDR10+ 独立
//     配键**（HDR 三族细分独立标签站——优堡五族先例泛家族归一不适用：
//     站方无泛 HDR 项，hdr_vivid/hlg 无对应 miss 落空）/禁转=[restricted]；
//     首发 sf 铁律 label-only+原创/官字（财神雷区）/特效/原生/限转/
//     **Criterion label-only（dict tag.criterion 死键——extended 零
//     产出验证不配）**/应求 label-only。
//   - 字段：url+**douban_id**（豆瓣 ID/链接双形态——配 dburl 完整链接
//     域投递）+uplver；**offers 候选制不勾**（优堡铁律——Crazy User
//     以下候选流程人工）；nfo 不配（13城教训）；无 pt_gen/technical_info。
// 站规人工把关：≥1080i 硬要求（§5.2 分辨率未达 1080i 不允许——特化
// 路由保留 720p/SD 分支映射正确性、站规预览把关）+<100MB 禁+upscale
// 禁+RMVB 禁+杂项概不接受+H&R 标志种 60 天/336h 考核+候选制+Dupe
// 媒介优先级。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(104, "hdhome_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdhome.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdhome.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "tags": "tags[]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "douban_url": "douban_id", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies 8K UHD BD", "value": "506"}, {"label": "Movies UHD Blu-ray", "value": "499"}, {"label": "Movies UHD REMUX", "value": "518"}, {"label": "Movies Bluray", "value": "450"}, {"label": "Movies REMUX", "value": "415"}, {"label": "Movies 8K/4320p", "value": "505"}, {"label": "Movies 2160p", "value": "416"}, {"label": "Movies 1080p", "value": "414"}, {"label": "Movies 720p", "value": "413"}, {"label": "Movies SD", "value": "411"}, {"label": "Movies IPad", "value": "412"}, {"label": "TVSeries 8KUHD", "value": "523"}, {"label": "TVSeries 4K Bluray", "value": "502"}, {"label": "Doc Bluray", "value": "451"}, {"label": "Doc REMUX", "value": "421"}, {"label": "TVSeries 4320p", "value": "526"}, {"label": "TVShow 2160p", "value": "431"}, {"label": "TVSeries IPad", "value": "433"}, {"label": "TVSeries 720p", "value": "434"}, {"label": "TVSeries 1080i", "value": "435"}, {"label": "TVSeries 1080p", "value": "436"}, {"label": "TVSeries REMUX", "value": "437"}, {"label": "TVSereis Bluray", "value": "453"}, {"label": "TVSeries 2160p", "value": "438"}, {"label": "Musics APE", "value": "439"}, {"label": "TVSeries SD", "value": "432"}, {"label": "Musics FLAC", "value": "440"}, {"label": "Musics MV", "value": "441"}, {"label": "Musics Bluray", "value": "503"}, {"label": "Sports 720p", "value": "442"}, {"label": "Anime 8K UHD BD", "value": "510"}, {"label": "Sports 1080i", "value": "443"}, {"label": "Anime SD", "value": "444"}, {"label": "Anime IPad", "value": "445"}, {"label": "Anime 720p", "value": "446"}, {"label": "Anime 1080p", "value": "447"}, {"label": "Anime REMUX", "value": "448"}, {"label": "Anime Bluray", "value": "454"}, {"label": "Anime UHD REMUX", "value": "531"}, {"label": "Misc", "value": "409"}, {"label": "Anime 2160p", "value": "449"}, {"label": "Anime 8K/4320p", "value": "509"}, {"label": "Anime UHD Blu-ray", "value": "501"}, {"label": "Sports 2160p", "value": "504"}, {"label": "Sport 8K/4320p", "value": "511"}, {"label": "Doc 8K UHD BD", "value": "508"}, {"label": "Doc 8K UHD BD REMUX", "value": "529"}, {"label": "Doc UHD Blu-ray", "value": "500"}, {"label": "Doc 8K/4320p", "value": "507"}, {"label": "Doc 2160p", "value": "422"}, {"label": "Doc 1080p", "value": "420"}, {"label": "Doc 720p", "value": "419"}, {"label": "Doc SD", "value": "417"}, {"label": "Doc IPad", "value": "418"}, {"label": "TVMusic 1080i", "value": "424"}, {"label": "TVMusic 720p", "value": "423"}, {"label": "TVShows Bluray", "value": "452"}, {"label": "TVShow REMUX", "value": "430"}, {"label": "TVShow 1080p", "value": "429"}, {"label": "TVShow 1080i", "value": "428"}, {"label": "TVShow 720p", "value": "427"}, {"label": "TVShow SD", "value": "425"}, {"label": "TVShow IPad", "value": "426"}], "medium": [{"label": "UHD Blu-ray", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "AVC/H264/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "HEVC/H265/x265", "value": "2", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "3", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "AC3/DD", "value": "15", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "16", "standard_keys": ["audio.wav"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "TrueHD", "value": "13", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "DTS-HDMA", "value": "11", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HDHRA", "value": "18", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "TrueHD Atmos", "value": "12", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS-HDMA:X 7.1", "value": "17"}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "2160p/4K", "value": "1", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "2", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "3", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "4", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "5", "standard_keys": ["resolution.sd"]}, {"label": "4320p/8K", "value": "10", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "HDHome", "value": "1"}, {"label": "HDH", "value": "2"}, {"label": "HDHTV", "value": "3"}, {"label": "HDHPad", "value": "4"}, {"label": "HDHWEB", "value": "12"}, {"label": "3201", "value": "20"}, {"label": "SHMA", "value": "17"}, {"label": "TVman", "value": "21"}, {"label": "ARiN", "value": "19"}, {"label": "TTG", "value": "6"}, {"label": "M-Team", "value": "7"}, {"label": "Other", "value": "11"}, {"label": "969154968", "value": "22"}, {"label": "BMDru", "value": "23"}], "tags": [{"label": "原创", "value": "yc"}, {"label": "国语", "value": "gy", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "yy", "standard_keys": ["tag.cantonese_audio"]}, {"label": "官字", "value": "gz"}, {"label": "中字", "value": "zz", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "特效", "value": "tx"}, {"label": "原生", "value": "ybyp"}, {"label": "连载", "value": "lz", "standard_keys": ["tag.ongoing"]}, {"label": "完结", "value": "wj", "standard_keys": ["tag.complete"]}, {"label": "DIY", "value": "diy", "standard_keys": ["tag.diy"]}, {"label": "DOLBY VISION", "value": "db", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10", "value": "hdr10", "standard_keys": ["tag.hdr10"]}, {"label": "HDR10+", "value": "hdrm", "standard_keys": ["tag.hdr10_plus"]}, {"label": "Criterion", "value": "cc"}, {"label": "禁转", "value": "jz", "standard_keys": ["tag.restricted"]}, {"label": "限转", "value": "xz"}, {"label": "首发", "value": "sf"}, {"label": "应求", "value": "yq"}]}}`).Error
	})
}
