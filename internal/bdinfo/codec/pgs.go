package codec

import "github.com/ranfish/pt-forward/internal/bdinfo/stream"

func ScanPGS(g *stream.GraphicsStream, _ []byte) {
	if g.IsInitialized {
		return
	}
	g.IsInitialized = true
}
