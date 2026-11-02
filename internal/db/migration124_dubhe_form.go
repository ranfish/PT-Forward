package db

import (
	"gorm.io/gorm"
)

// migration124 §59.441: 天枢（dubhe.site）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（102.5KB 直连）——**docs/32 无文档
// 站第五例**（咖啡/海的宝/我的PT 路径——实时为源当天建档 dubhe.md）。
// **实时=资产全槽一致（零漂移站）**。
// 槽位映射：
//   - type 11：411 photo=[category.photo] **ext 活键历史首配**（罕见
//     摄影类目）+410 Books=[ebook]+406=[mv,music_video,concert] 三键
//     承接（克隆/优堡/Nova 同款）+408 双键+404 双键。
//   - medium 10（值 1-10 乱序排列）：**Blu-ray(1)=[bluray,uhd_bluray]
//     双键归并（无 UHD 项——奥申同款）**+**DVDR(6)=[dvd,dvdr] 双键
//     宽语义（无 DVD 项——我堡 DVD(6) 单项宽语义同款）**+Remux(3)=
//     [remux,uhd_remux] 双键+WEB-DL(10)=[webdl,webrip] 双键承接+
//     MiniBD/HD DVD/HDTV/Encode/CD/Track；无 Other 项 miss 落空预览
//     人工。
//   - **codec 10 H/X 四分立压制联动第九站（§59.380）**：H.265/HEVC(11)=
//     [h265]+**x.265(10) label-only（联动通道——带点形态三月 §59.414
//     带点增强覆盖同款）**+H.264/AVC(13)=[h264]+**x264(12) label-only
//     （联动通道——百川 X26x label-only 先例）**+MPEG-4/MPEG-2/VC-1/
//     AV1/Xvid/Other 全配。
//   - standard 5 p/i 合并形态（咖啡同款）：**5 2160p/2160i=[r2160p]
//     单键（r2160i 无 ext 键 miss 落空——织梦 r720i 同款）**+1 1080p/
//     1080i=[r1080p,r1080i] 双键+720p/SD/Other=[resolution.other]。
//   - **无 audiocodec 槽（第六站）**（GTK/三月/克隆/奥申/星湾后）——
//     质量信息由 codec/standard/标题传达。
//   - team 6：HDS/CHD/MySiLU/WiKi 外站官组 label-only+Other(5)=
//     [team.other] 兜底+**DubheWeb(6) 站方官组 label-only**。
//   - tags 6 极简：禁转(1)/首发(2) 绝不配（官方专属财神雷区）+DIY/
//     国语/中字/HDR 四键泛家族 四键配。
//   - 字段：url（IMDb 链接 data-pt-gen 标记）+technical_info+uplver+
//     anonymous:true；nfo 不配（13城先例域表无此域）；**price 促销
//     字段不配**；**无 pt_gen**。
// 站规人工把关：上传者资格+H&R+Dupe 规则（无 wiki 文档实时未采集——
//   首颗发布前人工核验站方规则页）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(124, "dubhe_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'dubhe.site' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'dubhe.site'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "Sports", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Misc", "value": "409", "standard_keys": ["category.other"]}, {"label": "Books", "value": "410", "standard_keys": ["category.ebook"]}, {"label": "photo", "value": "411", "standard_keys": ["category.photo"]}], "medium": [{"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "H.265/HEVC", "value": "11", "standard_keys": ["video.h265"]}, {"label": "x.265", "value": "10"}, {"label": "H.264/AVC", "value": "13", "standard_keys": ["video.h264"]}, {"label": "x264", "value": "12"}, {"label": "MPEG-4", "value": "7", "standard_keys": ["video.mpeg4"]}, {"label": "MPEG-2", "value": "6", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "AV1", "value": "9", "standard_keys": ["video.av1"]}, {"label": "Xvid", "value": "8", "standard_keys": ["video.xvid"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "standard": [{"label": "2160p/2160i", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/1080i", "value": "1", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other/其他", "value": "7", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}, {"label": "DubheWeb", "value": "6"}], "tags": [{"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
