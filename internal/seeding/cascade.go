package seeding

import (
	"github.com/ranfish/pt-forward/internal/companion"
	"github.com/ranfish/pt-forward/internal/model"
)

func FindRelatedByTagOrPath(torrent *model.TorrentInfo, allTorrents []*model.TorrentInfo, maxDepth int) []string {
	return companion.FindRelatedByTagOrPath(torrent, allTorrents, maxDepth)
}

func ExtractRepostTags(tags []string) []string {
	return companion.ExtractRepostTags(tags)
}

func HasAnyTag(tags []string, targets []string) bool {
	return companion.HasAnyTag(tags, targets)
}
