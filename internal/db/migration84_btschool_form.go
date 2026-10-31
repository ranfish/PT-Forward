package db

import (
	"gorm.io/gorm"
)

// migration84 §59.398: 学校（pt.btschool.club）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（39.3KB 直连抓取）+ docs/32/btschool.md
// 338 行读全（含标题命名规范/促销精确数据/打包规则试行）。
// 实时 HTML 与 sites.json 4 月资产全槽零漂移（第八个零漂移站）；
// **tags 资产缺口实时补全**（sites.json tags=None——青蛙站同款，
// span[] 6 项实时捕获）。
// NP 标准裸字段名（无 [4] 后缀——五二同款）；单模式站。
// 槽位特化：
//   - type 11 项**非标准值集**（电影=405 非 401！连续剧=406/综艺=
//     412/软件=404/资料=402=category.education/游戏=411/音乐=409/
//     体育=410/其他=415）；408 纪录双键
//   - medium 9 项乱序：Blu-ray(1) 与 Blu-rayUHD(12) 分立两键
//     （车站同款）+WEB-DL 承接 webrip+Remux 归并 uhd_remux+
//     DVDRip 精确键（站方无 DVDR 原盘项——文档权威注记）
//   - codec 6 项合并命名（H.264/AVC/x264 一项——压制联动 label
//     精确匹配不触发 ✓ 车站同款）；无 AV1/VP9；Xvid 配键
//   - audiocodec 9 项：DTS-HD/DTS(3) 合并项承接 DTS 家族四键
//     [dts,dts_hd_ma,dts_hd_hr,dtsx]（车站同款）+AC3(10) 承接
//     [ac3,dd,ddp] 三键；无 wav/DDP/Atmos 独立项
//   - standard 5 项：4K_UHD(5)/1080p/i(1) 双键/720p/i(3) 单键
//     （r720i 无键）/SD(4)/Other(6)——与车站同构值集不同
//   - team 12 项全 label-only：BTSCHOOL/Zone/BtsHD/BtsTV/BtsPAD
//     官组（站规非管理组禁选）+**六外站官组 WiKi/HDChina/HDB_iNT/
//     M-team/CMCT/Ourbits**（label 直配自动命中——车站首例的
//     六倍扩展）
//   - **tags 字段名 span[]（非标准 tags[]——全库唯一）**：6 项；
//     禁转/国语/中字 配键+首发/自制 label-only+**官方(3) 绝不
//     配键**（财神官种案同款雷区——站方双向校验，外站组转发
//     恒非官方）
//   - **imdbid/doubanid 编号字段不配**（站方仅收编号 tt4154756/
//     纯数字——发布链 imdb_url 域填完整 URL 语义不匹配；非必填
//     字段；挂账首颗发布观察站方审核是否要求编号——若要求需
//     编号提取功能另案实施）；无 pt_gen/url/technical_info
//   - 标题只能英文/数字/拼音禁中文（v1.05 英文重组天然合规——
//     大青虫同款）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(84, "btschool_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.btschool.club' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.btschool.club'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "tags": "span[]", "small_descr": "small_descr", "description": "descr", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影/Movies", "value": "405", "standard_keys": ["category.movie"]}, {"label": "连续剧/TV-Series", "value": "406", "standard_keys": ["category.tv_series"]}, {"label": "动漫/Animation", "value": "407", "standard_keys": ["category.animation"]}, {"label": "纪录片/Documentary", "value": "408", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "综艺/TV-Show", "value": "412", "standard_keys": ["category.tv_shows"]}, {"label": "软件/Software", "value": "404", "standard_keys": ["category.software"]}, {"label": "资料/Education", "value": "402", "standard_keys": ["category.education"]}, {"label": "游戏/Game", "value": "411", "standard_keys": ["category.game"]}, {"label": "音乐/Music", "value": "409", "standard_keys": ["category.music"]}, {"label": "体育/Sports", "value": "410", "standard_keys": ["category.sports"]}, {"label": "其他/Other", "value": "415", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Blu-rayUHD", "value": "12", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDRip", "value": "6", "standard_keys": ["medium.dvdrip"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "11", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC/x265", "value": "10", "standard_keys": ["video.h265"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD", "value": "11", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HD/DTS", "value": "3", "standard_keys": ["audio.dts", "audio.dts_hd_ma", "audio.dts_hd_hr", "audio.dtsx"]}, {"label": "AC3", "value": "10", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "LPCM", "value": "5", "standard_keys": ["audio.lpcm"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "4K_UHD", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/i", "value": "1", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p/i", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "6", "standard_keys": ["resolution.other"]}], "team": [{"label": "BTSCHOOL", "value": "1"}, {"label": "Zone", "value": "13"}, {"label": "BtsHD", "value": "2"}, {"label": "BtsTV", "value": "3"}, {"label": "BtsPAD", "value": "4"}, {"label": "WiKi", "value": "5"}, {"label": "HDChina", "value": "6"}, {"label": "HDB_iNT", "value": "7"}, {"label": "M-team", "value": "9"}, {"label": "CMCT", "value": "10"}, {"label": "Ourbits", "value": "11"}, {"label": "Other", "value": "12"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "自制", "value": "4"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}]}}`).Error
	})
}
