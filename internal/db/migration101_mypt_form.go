package db

import (
	"gorm.io/gorm"
)

// migration101 §59.417: 我的PT（cc.mypt.cc）发布表单配置分发。
// **docs/32 无文档站第三例**（§59.385 咖啡/§59.406 海的宝路径——
// sites.json form 资产+站方实时 HTML 为源直接建档 docs/32/mypt.md）。
// 实时 HTML（49.3KB 代理抓取）关键发现：
//   - **双区互斥站**（type×2 同 name：mode=4 影视区 10 项+mode=5
//     资料区 11 项倒序）——§59.372 后覆盖陷阱形态+§59.388 模式过滤
//     自动护（[5] 槽跳过）；**sites.json 资产 19 项 type/14 项
//     audio_codec/6 项 source_sel/6 项 processing_sel 是两区混录**
//     （audiocodec_sel[5]=资料区文件格式槽：app/ipa+exe/win+iso/esd
//     +pdf/ppt 等非音频——影视区无 audiocodec 槽，GTK 后第四站视角）。
//   - 发布链走 mode=4 影视区：type/medium_sel[4]/codec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]（17 项）+url/technical_
//     info/uplver；无 pt_gen（控件实证）；资料类种子（ebook/software
//     等键在 [5] 区 type）发布时 type miss 落空预览人工（双区站
//     固有限制——影视区专用表单）。
// 槽位映射（mode=4 影视区）：
//   - type 10 项：409 Opera/戏曲 label-only+404 纪录双键+**406 MTV/
//     演唱会/唱片三键第八站**+408 Playlet 短剧第十站+401/402/403/
//     405/407 全配。
//   - medium 11 项乱序全 canonical（大青虫式又一全配键站）：无 UHD
//     独立 Blu-ray=[bluray,uhd_bluray] 双键归并第五例+Remux 双键+
//     WEB-DL 承接 webrip+CD/Track 全配。
//   - codec 7 项：H.264/H.265 分立（无 X26x 联动零触发）+Xvid/
//     MPEG-2+MP4/AVI=[mpeg4]+**RM/RMVB label-only**（无键）；无
//     AV1/VP9/VC-1（miss 落空）。
//   - standard 8 项乱序：other=[resolution.other]+1080i/P 双键+
//     720p/2k/4k/8k 全配+**3D/DTS label-only**（语义怪异无 canonical
//     键）+360p/480p=[sd]。
//   - **无 audiocodec 槽**（影视区——资料区文件格式槽不配）。
//   - team 5 项 label-only：HDS/CHD/MySiLU/MY/发布者+Other/转载(5)
//     兜底（FRDS 不在表）。
//   - tags[4][] 17 项配键 8：**严禁转载=[restricted]**（extended
//     活键）+DIY资源/简繁中字=[chinese_subtitle]/国语/**HDR(原盘)=
//     四键泛家族第十站**/英语=[english_audio]（幸运先例——本站无
//     排斥站规证据）/已完结/粤语；**官方首发 label-only（铁律）**+
//     日韩剧/火种(零魔)/长期保种/恐怖暴力/少儿不宜/欧美剧/国产剧/
//     港奥台 9 项 label-only。
//   - pos_state/pos_state_until 促销字段不配（GTK 同款）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(101, "mypt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'cc.mypt.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'cc.mypt.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Opera/戏曲", "value": "409"}, {"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/节目", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "DOCU/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动画片", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MTV/演唱会/唱片", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "Sports/运动/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Playlet/短剧", "value": "408", "standard_keys": ["category.short_drama"]}], "medium": [{"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265", "value": "2", "standard_keys": ["video.h265"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "MP4/AVI", "value": "5", "standard_keys": ["video.mpeg4"]}, {"label": "RM/RMVB", "value": "6"}, {"label": "Other", "value": "7", "standard_keys": ["video.other"]}], "standard": [{"label": "other/其它", "value": "1", "standard_keys": ["resolution.other"]}, {"label": "1080i/P", "value": "2", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "2k/高清", "value": "4", "standard_keys": ["resolution.r1440p"]}, {"label": "4k/超清", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8k/蓝光", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "3D/DTS", "value": "7"}, {"label": "360p/480p", "value": "8", "standard_keys": ["resolution.sd"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "MY/发布者", "value": "4"}, {"label": "Other/转载", "value": "5"}], "tags": [{"label": "官方首发", "value": "2"}, {"label": "严禁转载", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "日韩剧", "value": "11"}, {"label": "火种(零魔)", "value": "23"}, {"label": "长期保种", "value": "3"}, {"label": "DIY资源", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "简繁中字", "value": "5", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "国语", "value": "6", "standard_keys": ["tag.chinese_audio"]}, {"label": "HDR(原盘)", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "英语", "value": "16", "standard_keys": ["tag.english_audio"]}, {"label": "恐怖/暴力", "value": "15"}, {"label": "少儿不宜", "value": "14"}, {"label": "已完结", "value": "13", "standard_keys": ["tag.complete"]}, {"label": "粤语", "value": "12", "standard_keys": ["tag.cantonese_audio"]}, {"label": "欧美剧", "value": "10"}, {"label": "国产剧", "value": "9"}, {"label": "港/奥/台", "value": "8"}]}}`).Error
	})
}
