package adapter

import "mime/multipart"

type fieldWriter struct {
	w   *multipart.Writer
	err error
}

func newFieldWriter(w *multipart.Writer) *fieldWriter {
	return &fieldWriter{w: w}
}

func (fw *fieldWriter) writeField(key, value string) {
	if fw.err != nil {
		return
	}
	fw.err = fw.w.WriteField(key, value)
}

func (fw *fieldWriter) hasError() error {
	return fw.err
}
