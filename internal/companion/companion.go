package companion

import (
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

func FindRelatedByTagOrPath(torrent *model.TorrentInfo, allTorrents []*model.TorrentInfo, maxDepth int) []string {
	if maxDepth <= 0 {
		maxDepth = 1
	}

	visited := map[string]bool{torrent.Hash: true}
	currentLevel := []*model.TorrentInfo{torrent}
	var result []string

	for depth := 0; depth < maxDepth; depth++ {
		var nextLevel []*model.TorrentInfo
		for _, t := range currentLevel {
			related := findDirectlyRelated(t, allTorrents, visited)
			for _, r := range related {
				if !visited[r.Hash] {
					visited[r.Hash] = true
					nextLevel = append(nextLevel, r)
					result = append(result, r.Hash)
				}
			}
		}
		if len(nextLevel) == 0 {
			break
		}
		currentLevel = nextLevel
	}
	return result
}

func findDirectlyRelated(torrent *model.TorrentInfo, allTorrents []*model.TorrentInfo, visited map[string]bool) []*model.TorrentInfo {
	repTags := ExtractRepostTags(torrent.Tags)
	var related []*model.TorrentInfo
	for _, t := range allTorrents {
		if visited[t.Hash] {
			continue
		}
		if HasAnyTag(t.Tags, repTags) {
			related = append(related, t)
			continue
		}
		if t.SavePath == torrent.SavePath && t.Name == torrent.Name && t.Name != "" {
			related = append(related, t)
		}
	}
	return related
}

func ExtractRepostTags(tags []string) []string {
	var repost []string
	for _, tag := range tags {
		if strings.HasPrefix(tag, "REPOST_") {
			repost = append(repost, tag)
		}
	}
	return repost
}

func HasAnyTag(tags []string, targets []string) bool {
	if len(targets) == 0 {
		return false
	}
	targetSet := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetSet[t] = true
	}
	for _, tag := range tags {
		if targetSet[tag] {
			return true
		}
	}
	return false
}
