package client

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadTorrentFile reads a .torrent file from torrent_dir by infoHash.
// Tries both lowercase and original case filenames (QB uses uppercase, TR uses lowercase).
func ReadTorrentFile(torrentDir, infoHash string) ([]byte, error) {
	if torrentDir == "" || infoHash == "" {
		return nil, os.ErrNotExist
	}
	path := filepath.Join(torrentDir, strings.ToLower(infoHash)+".torrent")
	data, err := os.ReadFile(path) //nolint:gosec // controlled by admin config
	if err == nil {
		return data, nil
	}
	path = filepath.Join(torrentDir, infoHash+".torrent")
	return os.ReadFile(path) //nolint:gosec // controlled by admin config
}
