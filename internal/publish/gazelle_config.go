// Package publish Gazelle 音乐站配置（§56.28）。
//
// 数据来源：examples/Gazelle/ 源码（lib/config.php + misc/migrations + templates/upload/music.twig）
// 海豚(dicmusic.com) / 海豹(greatposterwall.com) 共用 Gazelle 框架。
package publish

// GazelleReleaseTypes 发布类型（16 种，源码 release_type 迁移）。
var GazelleReleaseTypes = map[int]string{
	1:  "Album",
	3:  "Soundtrack",
	5:  "EP",
	6:  "Anthology",
	7:  "Compilation",
	8:  "Sampler",
	9:  "Single",
	10: "Demo",
	11: "Live album",
	12: "Split",
	13: "Remix",
	14: "Bootleg",
	15: "Interview",
	16: "Mixtape",
	21: "Unknown",
}

// GazelleReleaseTypeByName 按名称查 ID。
var GazelleReleaseTypeByName = map[string]int{
	"Album":      1,
	"Soundtrack": 3,
	"EP":         5,
	"Anthology":  6,
	"Compilation": 7,
	"Sampler":    8,
	"Single":     9,
	"Demo":       10,
	"Live album": 11,
	"Split":      12,
	"Remix":      13,
	"Bootleg":    14,
	"Interview":  15,
	"Mixtape":    16,
	"Unknown":    21,
}

// GazelleFormats 音频格式（6 种，源码 FORMAT 常量）。
var GazelleFormats = []string{
	"MP3", "FLAC", "Ogg Vorbis", "AAC", "AC3", "DTS",
}

// GazelleMedia 介质类型（9 种，源码 MEDIA 常量）。
var GazelleMedia = []string{
	"CD", "WEB", "Vinyl", "DVD", "BD", "Soundboard", "SACD", "DAT", "Cassette",
}

// GazelleEncodings 编码/码率（15 种，源码 ENCODING 常量）。
var GazelleEncodings = []string{
	"Lossless", "24bit Lossless",
	"V0 (VBR)", "V1 (VBR)", "V2 (VBR)",
	"320", "256", "192", "160", "128", "96", "64",
	"APS (VBR)", "APX (VBR)", "q8.x (VBR)",
	"Other",
}

// GazelleArtistTypes 艺术家角色（8 种，源码 ARTIST_TYPE 常量）。
var GazelleArtistTypes = map[int]string{
	1: "Main",
	2: "Guest",
	3: "Remixer",
	4: "Composer",
	5: "Conductor",
	6: "DJ",
	7: "Producer",
	8: "Arranger",
}

// GazelleArtistTypeByName 按名称查 ID。
var GazelleArtistTypeByName = map[string]int{
	"Main":      1,
	"Guest":     2,
	"Remixer":   3,
	"Composer":  4,
	"Conductor": 5,
	"DJ":        6,
	"Producer":  7,
	"Arranger":  8,
}

// GazelleUploadFields 发布表单字段名（源码 templates/upload/music.twig）。
// 用于 adapter_generic 的 multipart 表单填充。
var GazelleUploadFields = map[string]string{
	"release_type":   "releasetype",       // 发布类型 select
	"artist":         "artists[]",          // 艺术家数组
	"importance":     "importance[]",       // 艺术家角色数组
	"title":          "title",              // 专辑标题
	"year":           "year",               // 发行年份
	"format":         "format",             // 音频格式 select
	"encoding":       "bitrate",            // 编码/码率 select
	"media":          "media",              // 介质 select
	"scene":          "scene",              // scene 发布 checkbox
	"vanity_house":   "vanity_house",       // vanity house checkbox
	"remaster_title": "remaster_title",     // 重发行标题
	"remaster_year":  "remaster_year",      // 重发行年份
	"record_label":   "remaster_record_label", // 唱片厂牌
	"catalog_number": "remaster_catalogue_number", // 目录号
	"other_bitrate":  "other_bitrate",      // 自定义码率输入
	"flac_log":       "flac_log",           // EAC/XLD log 文件
	"flac_cue":       "flac_cue",           // cue 文件
	"vbr":            "vbr",                // VBR 标记
	"unknown":        "unknown",            // 未知码率
	"group_remasters": "groupremasters",    // 组重发行
	"image":          "image",              // 封面图片
	"desc":           "desc",               // 描述/简介
}
