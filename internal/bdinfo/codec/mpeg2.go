package codec

import "github.com/ranfish/pt-forward/internal/bdinfo/stream"

func ScanMPEG2(v *stream.VideoStream, _ []byte) {
	if v.IsInitialized {
		return
	}
	v.IsVBR = true
	v.IsInitialized = true
}
