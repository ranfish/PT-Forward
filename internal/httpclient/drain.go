package httpclient

import (
	"io"
	"net/http"
)

const maxDrainSize = 1 << 20

const MaxBodySize = 10 << 20

func DrainBody(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainSize))
	_ = resp.Body.Close()
}

func ReadBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, MaxBodySize))
}
