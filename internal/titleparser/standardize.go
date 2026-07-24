package titleparser

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed data/standard_mappings.json
var standardMappingsJSON []byte

// StandardParams 标准化后的参数（标准键）
type StandardParams struct {
	Type       string `json:"type"`        // category.movie
	Medium     string `json:"medium"`      // medium.bluray
	VideoCodec string `json:"video_codec"` // video.x264
	AudioCodec string `json:"audio_codec"` // audio.dts_hd_ma
	Resolution string `json:"resolution"`  // resolution.r1080p
	HDR        string `json:"hdr"`         // hdr.dv
	Source     string `json:"source"`      // source.china
	Team       string `json:"team"`        // team.cmct
}

type standardMappings struct {
	Type       map[string]string `json:"type"`
	Medium     map[string]string `json:"medium"`
	VideoCodec map[string]string `json:"video_codec"`
	AudioCodec map[string]string `json:"audio_codec"`
	Resolution map[string]string `json:"resolution"`
	HDR        map[string]string `json:"hdr"`
	Source     map[string]string `json:"source"`
}

var (
	mappings     standardMappings
	reverseMap   map[string]string // standard_key → canonical display
	mappingsOnce sync.Once
	loadErr      error
)

// canonicalDisplay 每个标准键的规范显示名（逆向映射优先使用）
var canonicalDisplay = map[string]string{
	// resolution
	"resolution.r4320p": "2160p",
	"resolution.r2160p": "2160p",
	"resolution.r1080p": "1080p",
	"resolution.r1080i": "1080i",
	"resolution.r720p":  "720p",
	"resolution.r480p":  "480p",
	// video_codec
	"video.x264":  "x264",
	"video.h264":  "H.264",
	"video.x265":  "x265",
	"video.h265":  "HEVC",
	"video.av1":   "AV1",
	"video.vp9":   "VP9",
	"video.mpeg2": "MPEG-2",
	"video.vc1":   "VC-1",
	"video.avs2":  "AVS2",
	// audio_codec
	"audio.dts_hd_ma": "DTS-HD MA",
	"audio.dts_x":     "DTS:X",
	"audio.dts_hd_hr": "DTS-HD HR",
	"audio.dts":       "DTS",
	"audio.truehd":    "TrueHD",
	"audio.ddp":       "DDP",
	"audio.dd":        "DD",
	"audio.flac":      "FLAC",
	"audio.aac":       "AAC",
	"audio.alac":      "ALAC",
	"audio.ape":       "APE",
	"audio.wav":       "WAV",
	"audio.opus":      "Opus",
	"audio.lpcm":      "LPCM",
	"audio.mp3":       "MP3",
	"audio.dsd":       "DSD",
	// medium
	"medium.bluray":      "Blu-ray",
	"medium.uhd_bluray":  "UHD Blu-ray",
	"medium.bluray_3d":   "3D Blu-ray",
	"medium.remux":       "Remux",
	"medium.uhd_remux":   "UHD Blu-ray Remux",
	"medium.encode":      "Encode",
	"medium.webdl":       "WEB-DL",
	"medium.webrip":      "WEBRip",
	"medium.hdtv":        "HDTV",
	"medium.uhdtv":       "UHDTV",
	"medium.dvdrip":      "DVDRip",
	"medium.bdrip":       "BDRip",
	"medium.tvrip":       "TVRip",
	"medium.dvd":         "DVD",
	// hdr
	"hdr.dv_hdr10plus": "DoVi HDR10+",
	"hdr.dv_hdr":       "DoVi HDR",
	"hdr.dv":           "DoVi",
	"hdr.hdr10plus":    "HDR10+",
	"hdr.hdr_vivid":    "HDR Vivid",
	"hdr.hdr10":        "HDR10",
	"hdr.hdr":          "HDR",
	"hdr.hlg":          "HLG",
	"hdr.sdr":          "SDR",
	// type
	"category.movie":       "电影",
	"category.tv_series":   "电视剧",
	"category.tv_shows":    "综艺",
	"category.animation":   "动漫",
	"category.documentary": "纪录片",
	"category.music":       "音乐",
	"category.sports":      "体育",
	"category.other":       "其他",
}

func ensureMappings() {
	mappingsOnce.Do(func() {
		if err := json.Unmarshal(standardMappingsJSON, &mappings); err != nil {
			loadErr = fmt.Errorf("parse standard_mappings.json: %w", err)
			return
		}
		// 构建逆向映射：标准键 → 规范显示名
		reverseMap = make(map[string]string)
		// 优先用 canonicalDisplay
		for k, v := range canonicalDisplay {
			reverseMap[k] = v
		}
		// 补充 canonicalDisplay 中没有的（从 forward map 取第一个）
		for _, m := range []struct {
			name string
			data map[string]string
		}{
			{"type", mappings.Type},
			{"medium", mappings.Medium},
			{"video_codec", mappings.VideoCodec},
			{"audio_codec", mappings.AudioCodec},
			{"resolution", mappings.Resolution},
			{"hdr", mappings.HDR},
			{"source", mappings.Source},
		} {
			for display, key := range m.data {
				if _, exists := reverseMap[key]; !exists {
					reverseMap[key] = display
				}
			}
		}
	})
}

func buildReverse(forward map[string]string) {
	for display, key := range forward {
		if _, exists := reverseMap[key]; !exists {
			reverseMap[key] = display
		}
	}
}

func standardizeMedium(medium string) string {
	if medium == "" {
		return ""
	}
	// 先尝试整体匹配
	if key := lookupStandard(mappings.Medium, medium); key != "" {
		return key
	}
	// 拆分为子词，逐个匹配，优先级：整体 > 部分
	parts := strings.Fields(medium)
	for _, part := range parts {
		if key := lookupStandard(mappings.Medium, part); key != "" {
			return key
		}
	}
	return ""
}

func lookupStandard(m map[string]string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	// 精确匹配
	if key, ok := m[value]; ok {
		return key
	}
	// 大小写不敏感匹配
	lower := strings.ToLower(value)
	for k, v := range m {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}

// ReverseLookup 标准键 → 规范显示名
func ReverseLookup(standardKey string) string {
	ensureMappings()
	if loadErr != nil {
		return ""
	}
	return reverseMap[standardKey]
}

// ReverseLookupWithFallback 标准键 → 规范显示名，带降级链
func ReverseLookupWithFallback(standardKey string, fallbackChains map[string][]string) string {
	display := ReverseLookup(standardKey)
	if display != "" {
		return display
	}
	// 降级链：category.animation → [category.movie, category.other]
	if chain, ok := fallbackChains[standardKey]; ok {
		for _, fb := range chain {
			if d := ReverseLookup(fb); d != "" {
				return d
			}
		}
	}
	return ""
}
