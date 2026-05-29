package fingerprint

import (
	"encoding/base64"
	"testing"
)

func makeTorrentData() []byte {
	pieces := make([]byte, 20*10)
	for i := range pieces {
		pieces[i] = byte(i)
	}
	return []byte("d" +
		"8:announce" + "36:http://tracker.example.com/announce" +
		"4:info" + "d" +
		"6:length" + "i10737418240e" +
		"4:name" + "12:Big.Movie.2025" +
		"12:piece length" + "i4194304e" +
		"6:pieces" + string(pieces[:200]) +
		"e" +
		"e")
}

func BenchmarkComputeFromTorrent_SingleFile(b *testing.B) {
	data := makeTorrentData()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ComputeFromTorrent(data)
	}
}

func makeMultiFileTorrent() []byte {
	pieces := make([]byte, 20*50)
	for i := range pieces {
		pieces[i] = byte(i % 256)
	}
	files := "l" +
		"d6:lengthi1073741824e4:pathl6:Sample16:sample.mp4ee" +
		"d6:lengthi10737418240e4:pathl10:Big.Movie.202516:big.movie.mkvee" +
		"d6:lengthi536870912e4:pathl6:Behind10:makingof.mp4ee" +
		"d6:lengthi268435456e4:pathl9:Subtitles2:en3:srt8:movie.srtee" +
		"e"
	return []byte("d" +
		"8:announce" + "36:http://tracker.example.com/announce" +
		"4:info" + "d" +
		"5:files" + files +
		"4:name" + "12:Big.Movie.2025" +
		"12:piece length" + "i4194304e" +
		"6:pieces" + string(pieces[:1000]) +
		"e" +
		"e")
}

func BenchmarkComputeFromTorrent_MultiFile(b *testing.B) {
	data := makeMultiFileTorrent()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ComputeFromTorrent(data)
	}
}

func BenchmarkDecodeBencode(b *testing.B) {
	data := makeTorrentData()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = decodeBencode(data)
	}
}

func BenchmarkDecodeBencode_Large(b *testing.B) {
	raw, _ := base64.StdEncoding.DecodeString("ZDY6YW5ub3VuY2U0MDpodHRwczovL3RyYWNrZXIuZXhhbXBsZS5jb20vYW5ub3VuY2U0OmluZm9kMTI6ZG93bmxvYWQtbGlzdGwwOmVuY29kaW5nZjQ6aW5mb2Q2Omxlbmd0aGkxMDczNzQxODI0MGU0Om5hbWUxODpCaWcuTW92aWUuMjAyNS4zODBwMTI6cGllY2UgbGVuZ3RoaTQxOTQzMDRlNjpwaWVjZXM2MDow4IiJiYqLjI6Njo+PkJCRkZmhoeImJqWjI6Q0M0Njo4Ojs8PD4/QEFERkdISUpLTE1OUFFSU1RVVldYWVphYmNkZWZnaGlqa2xtbm9wcXJzdHV2d3h5ejAxMjM0NTY3ODk0Om5vdGUxNTpUaGlzIGlzIGEgdGVzdCB0b3JyZW50ZWU=")
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = decodeBencode(raw)
	}
}

func BenchmarkComputeFromTorrent_Parallel(b *testing.B) {
	data := makeTorrentData()
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ComputeFromTorrent(data)
		}
	})
}
