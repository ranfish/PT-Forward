package db

import "gorm.io/gorm"

// migration170 §59.533 刀4: 好学（www.hxpt.org）发布配置 v2——新 API 架构
// 值集重造+re-enable（刀2 临时关的防误发闸门开启）。
//
// 旧 NP 配置（migration135·年级/教材/年份错位语义）整体替换。新值集来源：
// section-dict?category_id=8 实拉（全站统一值集·五分类对照实证）+
// tags-dict 实拉（9 项）。键映射对齐 dict canonical（type.json/medium.json/
// video_codec.json/tag.json 全集比对·§59.528 三守则核对：无跨域双投）。
//
// 值集语义要点：
//   - type→category_id（13 分类·music 大键集归并 mv/concert/lossless）
//   - medium→media（内容形态·视频 443=全部影视媒介键归并——旧站 migration135
//     「视频=全媒介键归一」同构）
//   - codec（H.264←h264+x264·H.265←h265+x265——编码器实现归并格式词条）
//   - team 70 组 label-only+其他←team.other（财神「能对应对应」先例）
//   - tags：国语/中字/DIY/合集/完结五键；首发/禁转/官种/独家不接键
//     （转存禁勾铁律+§59.524 禁转族零键=正确设计先例）
//   - source 域不配置（站方语义=处理来源 WEB-DL/BluRay——medium 键已在
//     media 域消费·一键双域=§59.528 违规；全站非必填 miss 不投·首颗实测
//     观察站方容忍度）
//   - cover→poster 复用 §59.510 瓷器通道（executor setForm(Cover) 通用投递）
func init() {
	RegisterMigration(170, "hxpt_form_config_v2", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.hxpt.org'").Scan(&count)
		if count == 0 {
			return nil
		}
		if err := gormDB.Exec(`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = 'www.hxpt.org'`, `{"anonymous":true,"enabled":true,"framework":"hxpt","form_fields":{"type":"category_id","medium":"media","codec":"codec","team":"team","tags":"tags","cover":"poster"},"value_mappings":{"type":[{"label":"电影","value":"1","standard_keys":["category.movie"]},{"label":"电视剧","value":"2","standard_keys":["category.tv_series"]},{"label":"短剧","value":"3","standard_keys":["category.short_drama"]},{"label":"音乐","value":"4","standard_keys":["category.music","category.lossless_music","category.mv","category.concert"]},{"label":"游戏","value":"5","standard_keys":["category.game"]},{"label":"软件","value":"6","standard_keys":["category.software"]},{"label":"动漫","value":"7","standard_keys":["category.animation"]},{"label":"纪录片","value":"8","standard_keys":["category.documentary"]},{"label":"电子书","value":"9","standard_keys":["category.ebook"]},{"label":"有声书","value":"10","standard_keys":["category.audiobook"]},{"label":"体育","value":"11","standard_keys":["category.sports"]},{"label":"综艺","value":"12","standard_keys":["category.tv_shows"]},{"label":"教育资源","value":"13","standard_keys":["category.education"]}],"medium":[{"label":"视频","value":"443","standard_keys":["medium.bluray","medium.uhd_bluray","medium.bluray_3d","medium.remux","medium.uhd_remux","medium.encode","medium.webdl","medium.webrip","medium.hdtv","medium.uhdtv","medium.dvdrip","medium.bdrip","medium.tvrip","medium.dvd"]},{"label":"音频","value":"444"},{"label":"文档","value":"445"},{"label":"图片","value":"446"},{"label":"软件","value":"447"},{"label":"书籍","value":"448"},{"label":"游戏","value":"449"},{"label":"其他","value":"450"}],"codec":[{"label":"H.264","value":"435","standard_keys":["video.h264","video.x264"]},{"label":"H.265","value":"436","standard_keys":["video.h265","video.x265"]},{"label":"AV1","value":"437","standard_keys":["video.av1"]},{"label":"XviD","value":"438","standard_keys":["video.xvid"]},{"label":"FLAC","value":"439"},{"label":"MP3","value":"440"},{"label":"APE","value":"441"},{"label":"其他","value":"442"}],"team":[{"label":"HX","value":"460"},{"label":"HXWEB","value":"461"},{"label":"HXPT","value":"462"},{"label":"CSPT","value":"463"},{"label":"CSWEB","value":"464"},{"label":"ZmWeb","value":"465"},{"label":"FT","value":"466"},{"label":"WiKi","value":"467"},{"label":"CMCT","value":"468"},{"label":"FRDS","value":"469"},{"label":"HDSky","value":"470"},{"label":"HDChina","value":"471"},{"label":"CHDBits","value":"472"},{"label":"TTG","value":"473"},{"label":"OurBits","value":"474"},{"label":"HDHome","value":"475"},{"label":"HDArea","value":"476"},{"label":"HDRoute","value":"477"},{"label":"HDSpace","value":"478"},{"label":"HDTime","value":"479"},{"label":"HDZone","value":"480"},{"label":"HDCity","value":"481"},{"label":"HDStar","value":"482"},{"label":"HDRoad","value":"483"},{"label":"HD001","value":"484"},{"label":"HDSource","value":"485"},{"label":"HDCTV","value":"486"},{"label":"HDFans","value":"487"},{"label":"HDU","value":"488"},{"label":"HDKing","value":"489"},{"label":"HDB","value":"490"},{"label":"HDW","value":"491"},{"label":"HDC","value":"492"},{"label":"CHD","value":"493"},{"label":"MySiLU","value":"494"},{"label":"HDS","value":"495"},{"label":"HDA","value":"496"},{"label":"HDT","value":"497"},{"label":"TLF","value":"498"},{"label":"NGB","value":"499"},{"label":"QHstudIo","value":"500"},{"label":"SF","value":"501"},{"label":"HDV","value":"502"},{"label":"HDM","value":"503"},{"label":"3E","value":"504"},{"label":"DiY","value":"505"},{"label":"SubHD","value":"506"},{"label":"FLUX","value":"507"},{"label":"PandaMoon","value":"508"},{"label":"HDClub","value":"509"},{"label":"LCHD","value":"510"},{"label":"BMDru","value":"511"},{"label":"MPAD","value":"512"},{"label":"TNP","value":"513"},{"label":"MWEB","value":"514"},{"label":"LoliHouse","value":"515"},{"label":"SweetSub","value":"516"},{"label":"VCB-Studio","value":"517"},{"label":"Moozzi2","value":"518"},{"label":"ANi","value":"519"},{"label":"Airota","value":"520"},{"label":"Ohys-Raws","value":"521"},{"label":"Beatrice-Raws","value":"522"},{"label":"ReinForce","value":"523"},{"label":"Kagura","value":"524"},{"label":"ANK-Raws","value":"525"},{"label":"AI-RAWS","value":"526"},{"label":"Nekomoe kissaten","value":"527"},{"label":"Sakurato","value":"528"},{"label":"其他","value":"529","standard_keys":["team.other"]}],"tags":[{"label":"国语","value":"9","standard_keys":["tag.chinese_audio"]},{"label":"中字","value":"10","standard_keys":["tag.chinese_subtitle"]},{"label":"DIY","value":"11","standard_keys":["tag.diy"]},{"label":"合集","value":"4","standard_keys":["tag.collection"]},{"label":"完结","value":"14","standard_keys":["tag.complete"]},{"label":"首发","value":"6"},{"label":"禁转","value":"8"},{"label":"官种","value":"3"},{"label":"独家","value":"15"}]}}`).Error; err != nil {
			return err
		}
		return nil
	})
}
