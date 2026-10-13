package titleparser

import "testing"

// §59.319 附三十二: General 段重编码器词表——ShanaEncoder 等压制前端的
// Writing library 写在 General 段（Video 段扫描漏检——雀起乡案）。
func TestExtractMediaInfo_GeneralReencoder(t *testing.T) {
	// 雀起乡 MI 实录形态：General 段 ShanaEncoder，Video 段无 writing library
	mi := `General
Unique ID                                : 173356742009558425531373210285870817254
Complete name                            : /home/pt/雀起乡/lark.mkv
Format                                   : Matroska
Writing application                      : mkvmerge v8.3.0 ('Over the Horizon') 64bit
Writing library                          : libebml v1.3.1 + libmatroska v1.4.2 / ShanaEncoder

Video
ID                                       : 1
Format                                   : HEVC
Format/Info                              : High Efficiency Video Coding
Codec ID                                 : V_MPEGH/ISO/HEVC
Bit depth                                : 10 bits

Audio
ID                                      : 2
Format                                  : AC-3
Codec ID                                : A_AC3
Channel(s)                              : 2 channels`
	tech := ExtractMediaInfo(mi)
	if !tech.Encoded {
		t.Fatal("General 段 ShanaEncoder 应触发 MIEncoded 铁证（雀起乡案）")
	}
}

func TestExtractMediaInfo_MuxerNotReencoder(t *testing.T) {
	// mkvmerge/libebml 封装器（Remux 重封装同样写入）不触发
	mi := `General
Format                                   : Matroska
Writing application                      : mkvmerge v8.3.0 ('Over the Horizon') 64bit
Writing library                          : libebml v1.3.1 + libmatroska v1.4.2

Video
Format                                  : HEVC
Bit depth                               : 10 bits`
	tech := ExtractMediaInfo(mi)
	if tech.Encoded {
		t.Fatal("纯封装器 Writing library 不应判重编码（Remux 误伤）")
	}
}

func TestExtractMediaInfo_VideoLayerStillWorks(t *testing.T) {
	// Video 段 writing library 铁证（§59.151 原路径）叠加语义
	mi := `General
Format                                   : Matroska

Video
Format                                  : AVC
Writing library                          : x264 core 148`
	tech := ExtractMediaInfo(mi)
	if !tech.Encoded {
		t.Fatal("Video 段 x264 writing library 铁证应保留")
	}
}
