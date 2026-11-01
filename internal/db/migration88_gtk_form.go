package db

import (
	"gorm.io/gorm"
)

// migration88 §59.402: GTK（pt.gtkpw.xyz）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（46.6KB 代理抓取——站方已配代理）+
// docs/32/gtk.md 405 行读全（Playwright 采集验证/dupe/打包规则）。
// 实时 HTML 与 sites.json 4 月资产全槽一致；**tags 资产缺口实时补全**
// （sites.json 6 项→实时 7 项：官方(3) 新增——青蛙/学校同款缺口形态；
// 官方标签绝不配键——财神官种案雷区，外站组转发恒非官方）。
// 单模式站（type 仅 1 个）；[4] 后缀标准形态。
// 槽位特化：
//   - type 12 项（实时乱序排列 412 前 401 后——值对照不受影响）：
//     406 MV 双键+408 HQ Audio=lossless_music+411 Music Album=
//     category.music+410 Book=ebook+412 Education=education
//     +409 Misc=other；404 纪录双键
//   - **无 audiocodec_sel 槽**（音频编码站方真无——南洋极简形态
//     的一部分；form_fields 不配该域）
//   - medium 11 项**全 canonical 配键**（第四个零 label-only 媒介站）
//   - codec 8 项全配：H.264(1) 与 x264 合并命名（无 X26x——压制
//     联动零触发=站方合并语义）+AV1/VP9 配键
//   - standard 6 项**p/i 分立双值**（1080p(1)/1080i(2) 独立——农场
//     同款）+2160p/4K(5)+4320p/8K(6)
//   - team 11 项全 label-only：**FRDS(9) 在表**（大青虫/冬樱/聆音
//     同款 FRDS 种子 label 直配直中）+外站官组 HDS/CHD/MySiLU/
//     WiKi/CMCT/MARK/MTeam/PTHome/beAst（label 直配——车站/学校
//     先例复利）
//   - tags 7 项：HDR→[hdr10,hdr10_plus,hdr_vivid,hlg] 四键泛家族
//     （财神/农场同款第三站）+禁转/国语/中字/DIY 配键+首发 label-only
//     铁律+官方 label-only（雷区）
//   - 字段：url（IMDb data-pt-gen）+pt_gen+nfo+technical_info+uplver
//     全齐；price/pos_state/pos_state_until 促销字段不配（站方默认值）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(88, "gtk_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.gtkpw.xyz' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.gtkpw.xyz'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies(电影)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series(剧集)", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows(综艺)", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries(纪录片)", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations(动画)", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos(MV)", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports(运动题材)", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio(高清音频)", "value": "408", "standard_keys": ["category.lossless_music"]}, {"label": "Misc(其他)", "value": "409", "standard_keys": ["category.other"]}, {"label": "Book(图书)", "value": "410", "standard_keys": ["category.ebook"]}, {"label": "Music Album(音乐专辑)", "value": "411", "standard_keys": ["category.music"]}, {"label": "Education(资料)", "value": "412", "standard_keys": ["category.education"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "UHD", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H.265/HEVC", "value": "6", "standard_keys": ["video.h265"]}, {"label": "AV1", "value": "7", "standard_keys": ["video.av1"]}, {"label": "VP9", "value": "8", "standard_keys": ["video.vp9"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "2160p/4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "4320p/8K", "value": "6", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5"}, {"label": "CMCT", "value": "6"}, {"label": "MARK", "value": "7"}, {"label": "MTeam", "value": "8"}, {"label": "FRDS", "value": "9"}, {"label": "PTHome", "value": "10"}, {"label": "beAst", "value": "11"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
