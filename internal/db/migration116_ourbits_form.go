package db

import (
	"gorm.io/gorm"
)

// migration116 §59.432: 我堡（ourbits.club）发布表单配置分发。
// 依据：docs/32/ourbits.md 465 行（Playwright 完整采集）+ sites.json
// 4 月资产双源互验一致。
// **实时 upload.php 不可达（资质门铁证）**：29 凭证"通过审核 10 种才可
// 直接发布（当前 5/10）"——完整表单页返回候选提示页；发布走候选制
// （offers 提交）——**首颗真实发布须走候选通道或待权限达标**（挂账
// 用户知晓）。降级验证先例：五二 §59.377（双源互验+DryRun 先行，
// 权限达标后实时比对补验）。
// 槽位映射：
//   - type 11：**402"Movies-3D" label-only**（柠檬 408 同款保守——
//     cat402 实态验证参数不生效两页同题无法仲裁，Stereo3D 无 type
//     路由消费，电影走 401 通用）；**412 电视剧单集 label-only
//     （§59.424 系统禁分集——槽位设计即站规执行 13城同款）**+
//     tv_series 全走 405 TV-Pack（整季/合集）；419=[concert]/414/416
//     双键+410 双键。
//   - medium 8：UHD/FHD Blu-ray 分立各单键+**UHDTV=[uhdtv]**（extended
//     活键——农场/藏宝阁先例）+WEB-DL=[webdl,webrip]+DVD=[dvd,dvdr]。
//   - codec 7 **值 12 起跳号**（12/14/15/16/17/19/18——乱序跳号）：
//     H.264/HEVC/MPEG-2/VC-1/Xvid/AV1/Other 全配键无 X26x（零联动）。
//   - audiocodec 13 乱序（14/21/1/2/4/5/13/12/7/6/11/32/33）：Atmos=
//     [atmos,truehd_atmos]（慕雪阁模式——站方 Atmos 泛项+TrueHD 分立）+
//     DTS X=[dtsx]+**AC3=[ac3,dd,ddp] 三键承接**（无 DDP 分立——大青虫
//     式）+MPEG/OPUS 配键（青蛙先例）+全独立。
//   - standard 5：2160p 置尾乱序全配键。
//   - **team 1 单项"原创/原抓" label-only**（官方专属绝不配键——转种
//     恒非原创首发同族雷区；miss→nil 不填=站方默认态藏宝阁 NONE 同款）。
//   - processing_sel=产区槽不配（发布链无投递域历史定案）。
//   - **tags 11 字符串值形态（家园后第二例——sf/diy/gy 值非数字）**：
//     **sf 首发/jz 禁转绝不配**+yq 应求 label-only+diy/gy/zz 配键+
//     **HDR 五族细分独立（优堡/天空/家园后第四站）**：db=[dolby_vision]/
//     hdrvivid=[hdr_vivid]/hdr=[hdr10]/hdrp=[hdr10_plus]/hlg=[hlg]。
//   - 字段：url（IMDb 必填）+**dburl（豆瓣必填——FieldDomainDoubanURL
//     §59.159 域南洋/葡萄先例）**+uplver+anonymous:true；picture/offer
//     （候选关联）不配；无 pt_gen/nfo/technical_info（文档实证）。
// 站规人工把关：编码矩阵（FHD Blu-ray×H.264=720p/1080p、UHD Blu-ray
// ×H.264=×、HEVC 含 HDR 信息）+官方组优先 Dupe+黑名单组（文档 §38）
// +电视剧必须整季包+标题中文副标题规范+候选制。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(116, "ourbits_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'ourbits.club' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'ourbits.club'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "tags": "tags[]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "dburl": "dburl", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "Movies-3D/3D电影", "value": "402"}, {"label": "Concert/演唱会", "value": "419", "standard_keys": ["category.concert"]}, {"label": "TV-Episode/电视剧单集", "value": "412"}, {"label": "TV-Pack/电视剧包", "value": "405", "standard_keys": ["category.tv_series"]}, {"label": "TV-Show/综艺节目", "value": "413", "standard_keys": ["category.tv_shows"]}, {"label": "Documentary/纪录片", "value": "410", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animation/动漫", "value": "411", "standard_keys": ["category.animation"]}, {"label": "Sports/体育", "value": "415", "standard_keys": ["category.sports"]}, {"label": "Music-Video/MV", "value": "414", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Music/音乐", "value": "416", "standard_keys": ["category.music", "category.lossless_music"]}], "medium": [{"label": "UHD Blu-ray", "value": "12", "standard_keys": ["medium.uhd_bluray"]}, {"label": "FHD Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "9", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "UHDTV", "value": "13", "standard_keys": ["medium.uhdtv"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}], "codec": [{"label": "H.264", "value": "12", "standard_keys": ["video.h264"]}, {"label": "HEVC", "value": "14", "standard_keys": ["video.h265"]}, {"label": "MPEG-2", "value": "15", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "16", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "17", "standard_keys": ["video.xvid"]}, {"label": "AV1", "value": "19", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "18", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "Atmos", "value": "14", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS X", "value": "21", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HDMA", "value": "1", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "TrueHD", "value": "2", "standard_keys": ["audio.truehd"]}, {"label": "DTS", "value": "4", "standard_keys": ["audio.dts"]}, {"label": "LPCM", "value": "5", "standard_keys": ["audio.lpcm"]}, {"label": "FLAC", "value": "13", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "12", "standard_keys": ["audio.ape"]}, {"label": "AAC", "value": "7", "standard_keys": ["audio.aac"]}, {"label": "AC3", "value": "6", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}, {"label": "MPEG", "value": "32", "standard_keys": ["audio.mpeg"]}, {"label": "OPUS", "value": "33", "standard_keys": ["audio.opus"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}], "team": [{"label": "原创/原抓", "value": "41"}], "tags": [{"label": "首发", "value": "sf"}, {"label": "DIY", "value": "diy", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "gy", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "zz", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "应求", "value": "yq"}, {"label": "禁转", "value": "jz"}, {"label": "杜比视界", "value": "db", "standard_keys": ["tag.dolby_vision"]}, {"label": "菁彩HDR", "value": "hdrvivid", "standard_keys": ["tag.hdr_vivid"]}, {"label": "HDR10", "value": "hdr", "standard_keys": ["tag.hdr10"]}, {"label": "HDR10+", "value": "hdrp", "standard_keys": ["tag.hdr10_plus"]}, {"label": "HLG", "value": "hlg", "standard_keys": ["tag.hlg"]}]}}`).Error
	})
}
