package fingerprint

// BuildSingleFileTorrent 构造单文件 .torrent（bencode 自洽——测试/工具复用）。
// §59.252 教训：手写字节串 announce 长度差 1 字节即致 info 键解析错乱——
// 统一走 encodeBencode 保证语义。
func BuildSingleFileTorrent(announce, name string, length, pieceLen int, pieces []byte) ([]byte, error) {
	return encodeBencode(map[string]any{
		"announce": announce,
		"info": map[string]any{
			"length":       length,
			"name":         name,
			"piece length": pieceLen,
			"pieces":       string(pieces),
		},
	})
}
