// Package description 音乐站描述模板（§56.28 Q5）。
//
// 独立于影视模板，专辑信息格式（Artist/Year/Label/Catalog/Tracklist）。
// 用于 Gazelle 框架音乐站（海豚/海豹）。
package description

import (
	"strings"
)

// MusicAlbumData 专辑数据（音乐站描述渲染输入）。
type MusicAlbumData struct {
	Artists       []MusicArtist // 艺术家+角色
	Title         string        // 专辑标题
	Year          string        // 发行年份
	RecordLabel   string        // 唱片厂牌
	CatalogNumber string        // 目录号
	ReleaseType   string        // 发布类型（Album/EP/Single/...）
	Format        string        // 音频格式（FLAC/MP3/...）
	Encoding      string        // 编码/码率（Lossless/320/...）
	Media         string        // 介质（CD/Vinyl/WEB/...）
	Tracklist     []MusicTrack  // 曲目列表
	PosterURL     string        // 封面 URL
	Description   string        // 专辑简介
	Scene         bool          // 是否 scene 发布
}

// MusicArtist 艺术家。
type MusicArtist struct {
	Name       string // 艺术家名
	Importance string // 角色（Main/Guest/Composer/...）
}

// MusicTrack 曲目。
type MusicTrack struct {
	Number int    // 曲号
	Title  string // 曲名
	Duration string // 时长
}

// FormatMusicDescription 渲染音乐站描述（BBCode）。
func FormatMusicDescription(data *MusicAlbumData) string {
	if data == nil {
		return ""
	}
	var b strings.Builder

	// 封面
	if data.PosterURL != "" {
		b.WriteString("[img]")
		b.WriteString(data.PosterURL)
		b.WriteString("[/img]\n\n")
	}

	// 艺术家（按角色分组）
	mainArtists := filterByImportance(data.Artists, "Main")
	if len(mainArtists) > 0 {
		names := make([]string, 0, len(mainArtists))
		for _, a := range mainArtists {
			names = append(names, a.Name)
		}
		writeMusicField(&b, "Artist", strings.Join(names, " / "))
	}

	// 专辑信息
	writeMusicField(&b, "Album", data.Title)
	writeMusicField(&b, "Year", data.Year)
	writeMusicField(&b, "Release Type", data.ReleaseType)
	writeMusicField(&b, "Record Label", data.RecordLabel)
	writeMusicField(&b, "Catalog Number", data.CatalogNumber)

	// 发布信息
	writeMusicField(&b, "Format", data.Format)
	writeMusicField(&b, "Encoding", data.Encoding)
	writeMusicField(&b, "Media", data.Media)
	if data.Scene {
		writeMusicField(&b, "Scene", "Yes")
	}

	// 曲目列表
	if len(data.Tracklist) > 0 {
		b.WriteString("\n[align=center][b]Tracklist[/b][/align]\n")
		for _, track := range data.Tracklist {
			b.WriteString(formatTrackLine(track))
			b.WriteString("\n")
		}
	}

	// 简介
	if data.Description != "" {
		b.WriteString("\n")
		b.WriteString(data.Description)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// writeMusicField 写入字段（跳过空值）。
func writeMusicField(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("[b]")
	b.WriteString(label)
	b.WriteString(":[/b] ")
	b.WriteString(value)
	b.WriteString("\n")
}

// formatTrackLine 格式化曲目行。
func formatTrackLine(track MusicTrack) string {
	var b strings.Builder
	b.WriteString(formatTrackNumber(track.Number))
	b.WriteString(". ")
	b.WriteString(track.Title)
	if track.Duration != "" {
		b.WriteString(" (")
		b.WriteString(track.Duration)
		b.WriteString(")")
	}
	return b.String()
}

// formatTrackNumber 格式化曲号（补零）。
func formatTrackNumber(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return formatInt(n)
}

// formatInt 简单整数转字符串。
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// filterByImportance 按角色过滤艺术家。
func filterByImportance(artists []MusicArtist, importance string) []MusicArtist {
	var result []MusicArtist
	for _, a := range artists {
		if a.Importance == importance {
			result = append(result, a)
		}
	}
	return result
}
