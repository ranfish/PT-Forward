// Package publish Gazelle 音乐站配置（§56.28）。
//
// 数据来源：
//   海豚（dicmusic.com）真实采集（2026-07-19）— 通过 PT-Forward DB cookie 解密访问 upload.php
//   海豹（greatposterwall.com）可能不同，需单独验证（TODO）
//
// 与 Gazelle 源码（examples/Gazelle/）的差异：
//   - releasetype 用中文名称 + 自定义 ID（17/18/19 海豚新增）
//   - format 多了 DSD（源码无）
//   - media 用 Blu-ray（源码用 BD）+ Unknown Media
//   - importance 用中文名称（DJ/编曲 合并了源码的 DJ+Arranger）
package publish

// ===== 海豚站真实数据（dicmusic.com 采集 2026-07-19）=====

// DicmusicReleaseTypes 海豚发布类型（16 种，中文）。
var DicmusicReleaseTypes = map[int]string{
	1:  "专辑",
	3:  "原声",
	5:  "EP",
	6:  "精选",
	7:  "合辑",
	9:  "单曲专辑",
	11: "现场专辑",
	13: "重混音",
	14: "私录专辑",
	15: "访谈",
	16: "私制合辑",
	17: "录音样带",
	18: "音乐会录音",
	19: "DJ混音",
	21: "未知",
}

// DicmusicReleaseTypeByName 按名称查 ID（中文 + 英文别名）。
var DicmusicReleaseTypeByName = map[string]int{
	"专辑": 1, "Album": 1,
	"原声": 3, "Soundtrack": 3,
	"EP": 5,
	"精选": 6,
	"合辑": 7,
	"单曲专辑": 9, "Single": 9,
	"现场专辑": 11, "Live album": 11,
	"重混音": 13, "Remix": 13,
	"私录专辑": 14, "Bootleg": 14,
	"访谈": 15, "Interview": 15,
	"私制合辑": 16, "Mixtape": 16,
	"录音样带": 17, "Sampler": 17,
	"音乐会录音": 18,
	"DJ混音": 19,
	"未知": 21, "Unknown": 21,
}

// DicmusicFormats 海豚音频格式（6 种）。
var DicmusicFormats = []string{
	"FLAC", "DSD", "MP3", "AAC", "AC3", "DTS",
}

// DicmusicMedia 海豚介质类型（10 种）。
var DicmusicMedia = []string{
	"CD", "DVD", "Vinyl", "Soundboard", "SACD",
	"Blu-ray", "DAT", "Cassette", "WEB", "Unknown Media",
}

// DicmusicEncodings 海豚编码/码率（12 种）。
var DicmusicEncodings = []string{
	"192", "APS (VBR)", "V2 (VBR)", "V1 (VBR)",
	"256", "APX (VBR)", "V0 (VBR)", "q8.x (VBR)",
	"320", "Lossless", "24bit Lossless", "Other",
}

// DicmusicArtistTypes 海豚艺术家角色（7 种，中文）。
var DicmusicArtistTypes = map[int]string{
	1: "主要",
	2: "客座",
	3: "重混",
	4: "作曲",
	5: "指挥",
	6: "DJ／编曲",
	7: "制作人",
}

// DicmusicArtistTypeByName 按名称查 ID。
var DicmusicArtistTypeByName = map[string]int{
	"主要": 1, "Main": 1,
	"客座": 2, "Guest": 2,
	"重混": 3, "Remixer": 3,
	"作曲": 4, "Composer": 4,
	"指挥": 5, "Conductor": 5,
	"DJ／编曲": 6, "DJ": 6,
	"制作人": 7, "Producer": 7,
}

// DicmusicSampleRates 海豚采样率（6 种）。
var DicmusicSampleRates = []string{
	"44.1kHz", "48kHz", "88.2kHz", "96kHz", "176.4kHz", "192kHz",
}

// DicmusicUploadFields 海豚发布表单字段名（真实采集）。
// 与 Gazelle 源码的差异标注 § 差异。
var DicmusicUploadFields = map[string]string{
	"release_type":   "releasetype",
	"artist":         "artists[]",
	"importance":     "importance[]",
	"title":          "title",
	"year":           "year",
	"format":         "format",
	"encoding":       "bitrate",
	"media":          "media",
	"scene":          "scene",
	"vanity_house":   "vanity_house",
	"remaster_title": "remaster_title",
	"remaster_year":  "remaster_year",
	"record_label":   "remaster_record_label",
	"catalog_number": "remaster_catalogue_number",
	"other_bitrate":  "other_bitrate",
	"image":          "image",
	"desc":           "desc",
	// § 海豚特有字段
	"subtitle":       "subtitle",       // 副标题
	"genre_tags":     "genre_tags",     // 曲风标签下拉
	"sample_rate":    "sample_rate",    // 采样率
	"tags":           "tags",           // 自由标签
	"diy":            "diy",            // DIY 标记
	"jinzhuan":       "jinzhuan",       // 金砖标记
	"buy":            "buy",            // 购买链接
	"logfiles":       "logfiles[]",     // log 文件数组
	"remaster":       "remaster",       // 重发行标记
	"vbr":            "vbr",
	"unknown":        "unknown",
}

// ===== 向后兼容别名（原 Gazelle 源码版本，保留供海豹站使用）=====

// GazelleReleaseTypes Gazelle 框架默认（源码 release_type 迁移，16 种英文）。
// 海豚站请用 DicmusicReleaseTypes。
var GazelleReleaseTypes = map[int]string{
	1: "Album", 3: "Soundtrack", 5: "EP", 6: "Anthology",
	7: "Compilation", 8: "Sampler", 9: "Single", 10: "Demo",
	11: "Live album", 12: "Split", 13: "Remix", 14: "Bootleg",
	15: "Interview", 16: "Mixtape", 21: "Unknown",
}

// GazelleReleaseTypeByName Gazelle 框架默认反查。
var GazelleReleaseTypeByName = map[string]int{
	"Album": 1, "Soundtrack": 3, "EP": 5, "Anthology": 6,
	"Compilation": 7, "Sampler": 8, "Single": 9, "Demo": 10,
	"Live album": 11, "Split": 12, "Remix": 13, "Bootleg": 14,
	"Interview": 15, "Mixtape": 16, "Unknown": 21,
}

// GazelleFormats Gazelle 框架默认格式。
var GazelleFormats = []string{"MP3", "FLAC", "Ogg Vorbis", "AAC", "AC3", "DTS"}

// GazelleMedia Gazelle 框架默认介质。
var GazelleMedia = []string{"CD", "WEB", "Vinyl", "DVD", "BD", "Soundboard", "SACD", "DAT", "Cassette"}

// GazelleEncodings Gazelle 框架默认编码。
var GazelleEncodings = []string{
	"Lossless", "24bit Lossless", "V0 (VBR)", "V1 (VBR)", "V2 (VBR)",
	"320", "256", "192", "160", "128", "96", "64",
	"APS (VBR)", "APX (VBR)", "q8.x (VBR)", "Other",
}

// GazelleArtistTypes Gazelle 框架默认角色。
var GazelleArtistTypes = map[int]string{
	1: "Main", 2: "Guest", 3: "Remixer", 4: "Composer",
	5: "Conductor", 6: "DJ", 7: "Producer", 8: "Arranger",
}

// GazelleArtistTypeByName Gazelle 框架默认反查。
var GazelleArtistTypeByName = map[string]int{
	"Main": 1, "Guest": 2, "Remixer": 3, "Composer": 4,
	"Conductor": 5, "DJ": 6, "Producer": 7, "Arranger": 8,
}

// GazelleUploadFields Gazelle 框架默认表单字段（保留供海豹或其他 Gazelle 站使用）。
var GazelleUploadFields = map[string]string{
	"release_type": "releasetype", "artist": "artists[]", "importance": "importance[]",
	"title": "title", "year": "year", "format": "format", "encoding": "bitrate",
	"media": "media", "scene": "scene", "vanity_house": "vanity_house",
	"remaster_title": "remaster_title", "remaster_year": "remaster_year",
	"record_label": "remaster_record_label", "catalog_number": "remaster_catalogue_number",
	"other_bitrate": "other_bitrate", "flac_log": "flac_log", "flac_cue": "flac_cue",
	"vbr": "vbr", "unknown": "unknown", "group_remasters": "groupremasters",
	"image": "image", "desc": "desc",
}
