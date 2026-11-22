package util

// ChunkStrings §59.547: 字符串切片分批（SQLite IN 变量上限防越——500/批）。
// 自 reseed/engine.go chunkStrings 提升 public（api 列表端点四色预查消费——
// 「私有跨包先提升公共」纪律）。
func ChunkStrings(slice []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}
