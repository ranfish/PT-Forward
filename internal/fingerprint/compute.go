package fingerprint

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type TorrentMeta struct {
	InfoHash    string
	PiecesHash  string
	TotalSize   int64
	FileCount   int
	FileTree    map[string]int64
	LargestFile int64
	FilesHash   string
}

func ComputeFromTorrent(data []byte) (*TorrentMeta, error) {
	if len(data) == 0 {
		return nil, fpError(ErrFPCompute, "empty torrent data", nil)
	}

	d, err := decodeBencode(data)
	if err != nil {
		return nil, fpError(ErrFPCompute, "bencode decode", err)
	}

	root, ok := d.(map[string]any)
	if !ok {
		return nil, fpError(ErrFPCompute, "torrent root is not a dictionary", nil)
	}

	info, ok := root["info"]
	if !ok {
		return nil, fpError(ErrFPCompute, "missing info dictionary", nil)
	}

	infoDict, ok := info.(map[string]any)
	if !ok {
		return nil, fpError(ErrFPCompute, "info is not a dictionary", nil)
	}

	infoHash, err := computeInfoHash(infoDict)
	if err != nil {
		return nil, fpError(ErrFPCompute, "info hash", err)
	}

	piecesHash := computePiecesHash(infoDict)
	fileTree := extractFileTree(infoDict)
	totalSize := computeTotalSize(fileTree)
	fileCount := len(fileTree)
	largestFile := int64(0)
	for _, sz := range fileTree {
		if sz > largestFile {
			largestFile = sz
		}
	}
	filesHash := computeFilesHash(fileTree)

	return &TorrentMeta{
		InfoHash:    infoHash,
		PiecesHash:  piecesHash,
		TotalSize:   totalSize,
		FileCount:   fileCount,
		FileTree:    fileTree,
		LargestFile: largestFile,
		FilesHash:   filesHash,
	}, nil
}

func computeInfoHash(infoDict map[string]any) (string, error) {
	encoded, err := encodeBencode(infoDict)
	if err != nil {
		return "", err
	}
	h := sha1.Sum(encoded)
	return hex.EncodeToString(h[:]), nil
}

func computePiecesHash(infoDict map[string]any) string {
	pieces, ok := infoDict["pieces"].(string)
	if !ok {
		return ""
	}
	h := sha1.Sum([]byte(pieces))
	return hex.EncodeToString(h[:])
}

func extractFileTree(infoDict map[string]any) map[string]int64 {
	files := make(map[string]int64)

	if length, ok := infoDict["length"]; ok {
		name := getStr(infoDict, "name")
		if name != "" {
			files[name] = toInt64(length)
		}
		return files
	}

	fileList, ok := infoDict["files"].([]any)
	if !ok {
		return files
	}

	for _, f := range fileList {
		fd, ok := f.(map[string]any)
		if !ok {
			continue
		}
		length := toInt64(fd["length"])
		pathParts, ok := fd["path"].([]any)
		if !ok {
			continue
		}
		var parts []string
		for _, p := range pathParts {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		path := strings.Join(parts, "/")
		if path != "" {
			files[path] = length
		}
	}

	return files
}

func computeTotalSize(fileTree map[string]int64) int64 {
	var total int64
	for _, sz := range fileTree {
		total += sz
	}
	return total
}

func computeFilesHash(fileTree map[string]int64) string {
	keys := make([]string, 0, len(fileTree))
	for k := range fileTree {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteByte(0)
		fmt.Fprintf(&buf, "%d", fileTree[k])
		buf.WriteByte(0)
	}

	h := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(h[:])
}

func getStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
