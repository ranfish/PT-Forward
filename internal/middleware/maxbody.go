package middleware

import (
	"net/http"
	"strings"
)

const (
	maxBodySize       = 1 << 20  // 1MB default
	maxBodySizeUpload = 32 << 20 // 32MB for multipart form uploads (torrent files)
)

func MaxBodySize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			limit := int64(maxBodySize)
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				limit = maxBodySizeUpload
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}
