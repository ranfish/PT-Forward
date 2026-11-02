package db

import (
	"gorm.io/gorm"
)

// migration131 §59.448: PT地带（ptzone.xyz）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（45.1KB 直连）+ docs/32/ptzone.md
// 403 行读全（rules.php 完整+§4.1 音频编码重复定案）。
// **实时 vs 资产大漂移（末日式局部——4 月资产 type/medium/audiocodec
// 三槽值集大变）**：
//   - type 11→12：**408/409 同 label "Others(其它）" 双 value**（§59.444
//     龙PT 同形态——配置对齐 draft 单行口径删 409 留 HTML 首值 408）
//     +410 Ebook→Software+411 Edu→Games（资产后四项全漂移）。
//   - medium 10→10 换血：**HD DVD(2) 新增+UHD(10) 顶替资产 Other(10)**
//     （同 value 不同 label——value 漂移比 label 漂移更严重站方直接
//     拒收；实时 HTML 唯一真源）+Other 消失。
//   - audiocodec 15 大换血：LPCM/ALAC/M4A/AIFF/OPUS 消失+**DTS 3/9
//     同 label 克隆式重复（§59.436 首值锚=HTML 渲染序——配置留 3 删
//     9）**+AC3(8)/DD/AC3(11) 双 value 近语义+DDP/EAC3(12)/DTS-HD(13)/
//     TrueHD(14)/WAV(15) 新增。文档 §4.1 站方语义定案：DTS→3+AC3/DD
//     →11（DD/AC3 全语义）——**AC3(8) 裸项 label-only**（audio.ac3/dd
//     键均归 11——一键多值防线）。
//   - tags 资产 None→实时 8 项（青蛙/学校/海的宝同款资产缺口实时补全）。
//   - codec/standard/team 三槽实时=资产一致（零漂移）。
// 槽位映射：
//   - type 11（配置侧删 409）：404 双键+406 Music=[music,lossless_music]
//     双键（克隆先例——无 HQ Audio 分立）+403=[tv_shows]+407=[sports]+
//     410=[software]+411=[game]+**408 Others label-only**（杂项无键）。
//   - medium 10：UHD(10)=[uhd_bluray]+Blu-ray(1)+**HD DVD(2)=[hddvd]
//     ext 活键**+Remux(3) 双键归并+Encode(7)+WEB-DL(4)
//     双键+HDTV(5)+**DVDR(6)=[dvd,dvdr] 双键宽语义（阿玲先例——无 DVD
//     项用 DVDR 项收 dvd 键）**+CD(8)+**Track(9)=[track] ext 活键**；
//     **无 Other 项**（miss 落空预览人工）。
//   - codec 6：H.264/H.265 配键+VC-1/MPEG-2/**MPEG-4=[mpeg4] ext 活键**
//     +Other——**无 X26x 分立零联动**（§59.380 反证链）+无 AV1/VP9。
//   - audiocodec 14（配置侧删 DTS(9) 克隆）：FLAC/APE/**DTS(3) 留首值**
//     /MP3/OGG(ext)/AAC/Other/**AC3(8) label-only（键归 11）**/
//     DTS-HD MA(10)+**DD/AC3(11)=[ac3,dd] 双键（文档 §4.1 站方语义
//     定案）**+DDP/EAC3(12)=[ddp] 单键（eac3 无键——Nova/萝莉先例）+
//     **DTS-HD(13)=[dts_hd_hr]（家族剩余键——MA 已分立归 10，星湾
//     泛剩余先例）**+TrueHD(14)+WAV(15)。
//   - standard 6（值 5=8K/6=4K 乱序）：1080p/i 分立+720p+SD=[sd]+
//     8K=[r4320p]+4K=[r2160p]；**无 Other 项**（r576p/r480p miss 落空
//     ——SD 统一承接站方语义）。
//   - team 6：PTZWeb 官组 label-only+HDS/CHD/MySiLU/WiKi 外站组
//     label-only+Other(5)=[team.other] 兜底。
//   - tags 8：**禁转(1)/首发(2) 绝不配**（财神雷区）+完结(9)=[complete]
//     +分集(8)=[ongoing]+DIY(4)+国语(5)+中字(6)+**HDR(7)=[hdr10,
//     hdr10_plus,hdr_vivid,hlg] 财神四键泛家族**（站方无细分四键全
//     承接——§59.400 首发 18 站复利）。
//   - 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
//     technical_info+url（IMDb——data-pt-gen 真字段）+pt_gen+uplver；
//     **nfo 不配**（13城先例——发布链不投递）；无 douban 字段（豆瓣
//     走 pt_gen）；url×2 第二个是编辑器按钮（浦园/青蛙同款）。
// 站规人工把关：§3.3 上传者资格+§3.5 不允许的资源+§3.6 Dupe 判定
//   （同媒介+同分辨率+更高/同优先级组——动漫 HDTV/DVD 同级例外）+
//   §3.7 资源打包+§3.8 标题格式规范（电影/电视剧/音轨/游戏分模板）+
//   §4.5 无损音轨只保留一版本（分轨 FLAC 优先）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(131, "ptzone_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'ptzone.xyz' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'ptzone.xyz'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies(电影)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series(电视剧)", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows(综艺)", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries(纪录片)", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations(动漫)", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music(音乐)", "value": "406", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Sports(体育)", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Others(其它）", "value": "408"}, {"label": "Software(软件)", "value": "410", "standard_keys": ["category.software"]}, {"label": "Games(游戏)", "value": "411", "standard_keys": ["category.game"]}], "medium": [{"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "4", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD", "value": "10", "standard_keys": ["medium.uhd_bluray"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "3", "standard_keys": ["video.mpeg2"]}, {"label": "MPEG-4", "value": "4", "standard_keys": ["video.mpeg4"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H.265", "value": "6", "standard_keys": ["video.h265"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "AC3", "value": "8"}, {"label": "DTS-HD MA", "value": "10", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DD/AC3", "value": "11", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/EAC3", "value": "12", "standard_keys": ["audio.ddp"]}, {"label": "DTS-HD", "value": "13", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "TrueHD", "value": "14", "standard_keys": ["audio.truehd"]}, {"label": "WAV", "value": "15", "standard_keys": ["audio.wav"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "8K", "value": "5", "standard_keys": ["resolution.r4320p"]}, {"label": "4K", "value": "6", "standard_keys": ["resolution.r2160p"]}], "team": [{"label": "PTZWeb", "value": "6"}, {"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "完结", "value": "9", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "8", "standard_keys": ["tag.ongoing"]}, {"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
