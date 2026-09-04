package db

import (
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.167 · migration 33: 站点支持数据操作代码化（OTA 同步——243 的建站/下线
// 数据操作不随二进制分发，OTA 实例缺同步；用户定案：migration 随版本带走）。
// ① 传道院（pt.cdy.skin）关站下线：enabled=0/is_target=0（幂等）。
// ② 修道院（xdypt.vip）建站骨架 upsert：含 form_config 基线（站方表单结构
//    非敏感可代码化——243 快照 2026-09-03）；凭证不带（敏感不进代码——OTA 实例
//    用户填 passkey 或 CookieCloud 自动同步）。
func syncXdyAndCdySites(gormDB *gorm.DB) error {
	// ① 传道院下线
	if err := gormDB.Exec("UPDATE sites SET enabled = 0, is_target = 0, updated_at = CURRENT_TIMESTAMP WHERE domain = ? AND enabled = 1", "pt.cdy.skin").Error; err != nil {
		return err
	}
	// ② 修道院骨架 upsert（不存在则建；存在只补 form_config 空缺——不覆盖用户已配置）
	var cnt int64
	gormDB.Model(&model.Site{}).Where("domain = ?", "xdypt.vip").Count(&cnt)
	if cnt == 0 {
		site := model.Site{
			Name: "修道院", Domain: "xdypt.vip", BaseURL: "https://xdypt.vip",
			Framework: "nexusphp", AuthType: "cookie",
			// §59.167 用户定案：骨架不默认启用（新装实例无凭证的站不该出现在
			// "我的站点"——骨架落"未启用"列表，用户配置凭证后自行启用）
			Enabled: false, IsTarget: true,
			CookieCloudSync: true, CookieCloudDomain: "xdypt.vip",
			HRStrategy: "protect", DownloadMode: "template",
			DownloadURLTemplate: "download.php?id={id}&passkey={passkey}",
			HashStrategy: "guid", SizeStrategy: "enclosure",
			IDStrategy: "query_param", IDPattern: "id",
		}
		site.PublishFormConfig = xdyFormConfigBaseline
		if err := gormDB.Create(&site).Error; err != nil {
			return err
		}
	} else {
		// 已存在（如 243）：form_config 为空才补基线（不覆盖用户实配）
		gormDB.Exec("UPDATE sites SET publish_form_config = ? WHERE domain = ? AND (publish_form_config IS NULL OR publish_form_config = '')",
			xdyFormConfigBaseline, "xdypt.vip")
	}
	return nil
}

// xdyFormConfigBaseline 修道院发布配置基线（243 快照 2026-09-03——含 StandardKeys/
// cnname/tags/首发 id=2 禁勾语义见 form_config 词条；站方改版走 HTML 上传重刷）。
const xdyFormConfigBaseline = `{"enabled":true,"framework":"np","anonymous":true,"form_fields":{"cnname":"cnname","codec":"codec_sel[4]","description":"descr","imdb_url":"url","medium":"medium_sel[4]","small_descr":"small_descr","standard":"standard_sel[4]","tags":"tags[4][]","team":"team_sel[4]","type":"type","uplver":"uplver"},"value_mappings":{"codec":[{"label":"H.264","value":"1","standard_keys":["video.h264"]},{"label":"VC-1","value":"2","standard_keys":["video.vc1"]},{"label":"Xvid","value":"3","standard_keys":["video.xvid"]},{"label":"MPEG-2","value":"4","standard_keys":["video.mpeg2"]},{"label":"Other","value":"5","standard_keys":["video.other"]},{"label":"H.265","value":"6","standard_keys":["video.h265"]},{"label":"VP8/9","value":"7","standard_keys":["video.vp9"]}],"medium":[{"label":"Track","value":"9","standard_keys":["medium.track"]},{"label":"CD","value":"8","standard_keys":["medium.cd"]},{"label":"DVDR","value":"6","standard_keys":["medium.dvdr"]},{"label":"HDTV","value":"5","standard_keys":["medium.hdtv"]},{"label":"MiniBD","value":"4","standard_keys":["medium.minibd"]},{"label":"Encode","value":"7","standard_keys":["medium.encode"]},{"label":"Remux","value":"3","standard_keys":["medium.remux"]},{"label":"HD DVD","value":"2","standard_keys":["medium.hddvd"]},{"label":"Blu-ray","value":"1","standard_keys":["medium.bluray"]},{"label":"WEB-DL","value":"10","standard_keys":["medium.webdl"]},{"label":"ISO","value":"11","standard_keys":["medium.iso"]}],"standard":[{"label":"1080i/1080P","value":"6","standard_keys":["resolution.r1080p"]},{"label":"2K/1440i/1440P","value":"7","standard_keys":["resolution.r1440p"]},{"label":"4K/2160i/2160P","value":"8","standard_keys":["resolution.r2160p"]},{"label":"8K/4320i/4320P","value":"9","standard_keys":["resolution.r4320p"]},{"label":"Other","value":"10","standard_keys":["resolution.other"]},{"label":"720i/720P","value":"11","standard_keys":["resolution.r720p"]},{"label":"480i/480P","value":"12","standard_keys":["resolution.r480p"]}],"tags":[{"label":"禁转","value":"1","standard_keys":["tag.restricted"]},{"label":"首发","value":"2"},{"label":"官方","value":"3","standard_keys":["tag.official"]},{"label":"DIY","value":"4","standard_keys":["tag.diy"]},{"label":"国语","value":"5","standard_keys":["tag.chinese_audio"]},{"label":"中字","value":"6","standard_keys":["tag.chinese_subtitle"]},{"label":"HDR","value":"7"}],"team":[{"label":"WiKi","value":"16","standard_keys":["team.wiki"]},{"label":"MTeam","value":"26","standard_keys":["team.mteam"]},{"label":"FRDS","value":"25","standard_keys":["team.frds"]},{"label":"ADWeb","value":"24","standard_keys":["team.adweb"]},{"label":"HHWEB","value":"23","standard_keys":["team.hhweb"]},{"label":"ZmWeb","value":"22","standard_keys":["team.zmweb"]},{"label":"UBWeb","value":"21","standard_keys":["team.ubweb"]},{"label":"AGSVWEB","value":"20","standard_keys":["team.agsvweb"]},{"label":"CSWEB","value":"19","standard_keys":["team.csweb"]},{"label":"StarfallWeb","value":"18","standard_keys":["team.starfallweb"]},{"label":"Other","value":"17","standard_keys":["team.other"]},{"label":"TPWEB","value":"6"},{"label":"MySiLU","value":"15","standard_keys":["team.mysilu"]},{"label":"U2","value":"14","standard_keys":["team.u2"]},{"label":"HDS","value":"13","standard_keys":["team.hds"]},{"label":"CHD","value":"12","standard_keys":["team.chd"]},{"label":"QHstudio","value":"11","standard_keys":["team.qhstudio"]},{"label":"AllMWeb","value":"10","standard_keys":["team.mweb"]},{"label":"LUCKDIY","value":"9","standard_keys":["team.luckdiy"]},{"label":"LUCKWEB","value":"8","standard_keys":["team.luckweb"]},{"label":"LUCKMUSIC","value":"7","standard_keys":["team.luckmusic"]}],"type":[{"label":"电影（Movies）","value":"401","standard_keys":["category.movie"]},{"label":"电视剧（TV Series）","value":"402","standard_keys":["category.tv_series"]},{"label":"综艺（TV Shows）","value":"403","standard_keys":["category.tv_shows"]},{"label":"纪录片（Documentaries）","value":"404","standard_keys":["category.documentaries"]},{"label":"动漫（Animations）","value":"405","standard_keys":["category.animation"]},{"label":"Music Videos","value":"406","standard_keys":["category.mv"]},{"label":"体育（Sports）","value":"407","standard_keys":["category.sports"]},{"label":"HQ Audio","value":"408","standard_keys":["category.music"]},{"label":"音乐（Misc）","value":"409","standard_keys":["category.music"]},{"label":"游戏（Games）","value":"410","standard_keys":["category.game"]}]}}`

func init() {
	RegisterMigration(33, "sync_xdy_cdy_sites", syncXdyAndCdySites)
}
