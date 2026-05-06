package seeding

import "github.com/ranfish/pt-forward/internal/model"

func HasSameFileTorrent(torrent *model.TorrentInfo, allTorrents []*model.TorrentInfo) bool {
	for _, t := range allTorrents {
		if t.Hash == torrent.Hash {
			continue
		}
		if t.Name == torrent.Name && t.TotalSize == torrent.TotalSize && t.SavePath == torrent.SavePath {
			return true
		}
	}
	return false
}
