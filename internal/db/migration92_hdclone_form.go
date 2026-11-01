package db

import (
	"gorm.io/gorm"
)

// migration92 §59.408: 克隆（pt.hdclone.top）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（38.3KB 直连抓取）+ docs/32/hdclone.md
// 423 行读全（促销/dupe/打包/盒子/认领规则）。实时 HTML 与 4 月资产
// 全槽一致；[4] 后缀标准形态；单模式站。
// 槽位特化：
//   - **无 audiocodec_sel 槽**（站方真无——GTK 同款第二站）；无
//     pt_gen/nfo/source_sel/processing_sel
//   - **uplver 实时存在**（文档称无——文档漂移配 anonymous；另 offers
//     候选字段文档有实时无——漂移二）
//   - type 9 项：**409 Playlet=short_drama 第六站**+**410 MV/演唱会
//     三键承接 [mv,music_video,concert]**（MV 与演唱会站方合并项
//     ——三键归一）+408 Music=[music,lossless_music] 双键+407
//     Othes（慎选）=other；404 纪录双键
//   - medium 10 项**全 canonical 配键**（第五个零 label-only 媒介站
//     ——大青虫/末日/农场/GTK 后）：UHD 分立+Remux 归并+WEB-DL
//     承接 webrip+DVD 归并 [dvdr,dvd]+MiniBD/Track 全配
//   - codec 6 项合并命名（H.264/x264/AVC 一项——无独立 X26x 联动
//     零触发）+**AV1 配键**（站方前卫）；无 Xvid/MPEG-4
//   - standard 6 项乱序（4320p/8K=6 首位）：1080p/1080i 合并双键+
//     SD/720p以下=sd+Othes=resolution.other
//   - team 14 项 label-only：**FRDS(6) 在表直中第五站**+外站十三组
//     （M-Team/CHD/BeiTai/WiKi/AGSV/TLF/CMCT/beAst/HDSky/HDHome/
//     TTG/UBits/PTer）；**无 Other 项**（组名 miss→nil 不填预览
//     人工——文本框无兜底）
//   - tags 5 项（无首发✓）：禁转/DIY/国语/中字配键+**HDR 四键泛
//     家族第五站**
//   - url IMDb 真字段（url×2 按钮同款）+technical_info；price 促销
//     字段不配
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(92, "hdclone_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.hdclone.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.hdclone.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动漫、动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Playlet/短剧", "value": "409", "standard_keys": ["category.short_drama"]}, {"label": "MV/演唱会", "value": "410", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "Music/音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Othes/其他（慎选）", "value": "407", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "9", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "6", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "DVD", "value": "10", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "HDTV", "value": "2", "standard_keys": ["medium.hdtv"]}, {"label": "Track", "value": "5", "standard_keys": ["medium.track"]}], "codec": [{"label": "H.264/x264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H265/HEVC/x265", "value": "6", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "AV1", "value": "3", "standard_keys": ["video.av1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "standard": [{"label": "4320p/8K", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "2160p/4K", "value": "1", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/1080i", "value": "2", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD/720p以下", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Othes（慎选）", "value": "5", "standard_keys": ["resolution.other"]}], "team": [{"label": "FRDS", "value": "6"}, {"label": "beAst", "value": "9"}, {"label": "CMCT", "value": "8"}, {"label": "TLF", "value": "7"}, {"label": "WiKi", "value": "4"}, {"label": "UBits", "value": "13"}, {"label": "PTer", "value": "14"}, {"label": "M-Team", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "BeiTai", "value": "3"}, {"label": "AGSV", "value": "5"}, {"label": "HDSky", "value": "10"}, {"label": "HDHome", "value": "11"}, {"label": "TTG", "value": "12"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
