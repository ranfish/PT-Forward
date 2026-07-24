package titleparser

import (
	"regexp"
	"strings"
)

// reMusicYear 匹配音乐标题中的 4 位年份。
var reMusicYear = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)

// reMusicFormat 匹配音乐标题中的音频格式。
var reMusicFormat = regexp.MustCompile(`(?i)\b(FLAC|APE|WAV|MP3|AAC|ALAC|OGG|OPUS|DSD|WV|TTA)\b`)

// ParseMusicTitle 从音乐标题解析 TechProfile（§56.35 阶段 3）。
//
// 音乐标题主流格式（docs/32 调研，20+ 站统一）：
//
//	[中文艺术家名 - 中文专辑名] 艺术家名 - 专辑名 年份 音频编码-制作组
//
// 也支持无中文前缀的纯英文格式：
//
//	艺术家名 - 专辑名 年份 音频编码-制作组
func ParseMusicTitle(title string) TechProfile {
	p := TechProfile{}
	if title = strings.TrimSpace(title); title == "" {
		return p
	}

	// 剥离 [中文Artist - 中文Album] 前缀
	if strings.HasPrefix(title, "[") {
		if idx := strings.Index(title, "]"); idx > 0 {
			chinesePart := title[1:idx]
			remaining := strings.TrimSpace(title[idx+1:])
			if parts := strings.SplitN(chinesePart, " - ", 2); len(parts) == 2 {
				p.ChineseArtist = strings.TrimSpace(parts[0])
				p.ChineseAlbum = strings.TrimSpace(parts[1])
			}
			title = remaining
		}
	}

	// 剥离文件扩展名
	title = stripFileExtension(title)

	// 取最后一个 "-" 之后的内容 → Group（注意区分 " - " 和 "-"）
	// 音乐标题中 "Artist - Album" 用 " - " 分隔，Group 用 "-" 连接
	if idx := strings.LastIndex(title, "-"); idx > 0 {
		candidate := strings.TrimSpace(title[idx+1:])
		if isValidMusicGroup(candidate) {
			p.ReleaseGroup = candidate
			title = strings.TrimSpace(title[:idx])
		}
	}

	// 取第一个 " - " 分割 → Artist 和剩余
	if idx := strings.Index(title, " - "); idx > 0 {
		p.Artist = strings.TrimSpace(title[:idx])
		rest := strings.TrimSpace(title[idx+3:])

		// 从 rest 提取 Year（取最后一个匹配，避免专辑名含数字如 "1989" 被误判）
		if indices := reMusicYear.FindAllStringSubmatchIndex(rest, -1); len(indices) > 0 {
			last := indices[len(indices)-1]
			p.Year = rest[last[2]:last[3]]
			rest = strings.TrimSpace(rest[:last[0]] + " " + rest[last[1]:])
		}

		// 从 rest 提取 AudioCodec
		if m := reMusicFormat.FindStringSubmatch(rest); m != nil {
			p.AudioCodec = strings.ToUpper(m[1])
			rest = strings.TrimSpace(reMusicFormat.ReplaceAllString(rest, ""))
		}

		// 剩余部分是 Album
		p.Album = strings.TrimSpace(rest)
	} else {
		// 无 " - " 分隔，整行作为 Artist
		p.Artist = title
	}

	return p
}

// isValidMusicGroup 判断字符串是否是合法的制作组名。
func isValidMusicGroup(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || len(s) > 30 {
		return false
	}
	// 不能是纯数字或年份
	if reMusicYear.MatchString(s) && len(s) == 4 {
		return false
	}
	// 不能是音频格式
	if reMusicFormat.MatchString(s) {
		return false
	}
	return true
}

// reassembleMusic 按音乐范式重组标题（§56.35 阶段 3）。
//
// 主流格式：[中文Artist - 中文Album] Artist - Album Year AudioCodec[-Group]
// 可选追加：BitDepth SampleRate（PTerClub/AGSV 要求）
func reassembleMusic(p TechProfile, tf TitleFormat) string {
	applyParadigm(&tf)
	if tf.Separator == "" {
		tf.Separator = " "
	}
	if tf.GroupConnector == "" {
		tf.GroupConnector = "-"
	}

	var parts []string

	// 中文前缀
	if p.ChineseArtist != "" && p.ChineseAlbum != "" {
		parts = append(parts, "["+p.ChineseArtist+" - "+p.ChineseAlbum+"]")
	} else if p.ChineseArtist != "" {
		parts = append(parts, "["+p.ChineseArtist+"]")
	}

	// Artist - Album
	if p.Artist != "" {
		segment := p.Artist
		if p.Album != "" {
			segment += " - " + p.Album
		}
		parts = append(parts, segment)
	}

	// Year
	if p.Year != "" {
		parts = append(parts, p.Year)
	}

	// AudioCodec
	if p.AudioCodec != "" {
		parts = append(parts, p.AudioCodec)
	}

	// BitDepth + SampleRate（PTerClub/AGSV 要求）
	if p.AudioBitDepth != "" && p.SampleRate != "" {
		parts = append(parts, p.AudioBitDepth+"bit "+p.SampleRate+"kHz")
	}

	result := strings.Join(parts, tf.Separator)

	// separator 为 "." 时，"Artist - Album" 中的 " - " 替换为 "."（HDArea 格式）
	if tf.Separator == "." {
		result = strings.ReplaceAll(result, " - ", ".")
	}

	// Group
	if p.ReleaseGroup != "" {
		result += tf.GroupConnector + p.ReleaseGroup
	}

	// 后处理
	if tf.StripChinese {
		result = normalizeHalfWidth(result)
		result = stripChineseChars(result)
	}
	result = cleanSeparators(result, tf.Separator)

	return strings.TrimSpace(result)
}
