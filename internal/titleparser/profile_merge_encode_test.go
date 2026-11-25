package titleparser

import "testing"

// §59.532 解挂（v0.0.1623·馒头 1298316 冻结案）：Encode 限定透传锚。
// Zhizn 形态：压制铁证（Writing library 压制工具词）∧标题 x265 编码器词
// ∧MI 格式词 HEVC → 保留 x265 不降级（族内精度声明非跨族事实错误）。
func TestMergeMediaInfoIntoEncodePassthrough(t *testing.T) {
	// ① Zhizn 主形态：压制+标题 x265+MI HEVC → x265 保留
	p := &TechProfile{SourceType: "BluRay", VideoCodec: "x265"}
	MergeMediaInfoInto(p, &MediaInfoTech{VideoCodec: "HEVC", Encoded: true})
	if p.VideoCodec != "x265" {
		t.Errorf("压制铁证+标题编码器词应保留: got %q, want x265", p.VideoCodec)
	}
	// ② 原盘门（生死线）：无压制铁证 → MI 格式词覆盖（原盘 raw 带 x265 惯用词防线）
	p2 := &TechProfile{SourceType: "Blu-ray", VideoCodec: "x265"}
	MergeMediaInfoInto(p2, &MediaInfoTech{VideoCodec: "AVC", Encoded: false})
	if p2.VideoCodec != "AVC" {
		t.Errorf("原盘应被 MI 格式词覆盖: got %q, want AVC", p2.VideoCodec)
	}
	// ③ 空隙残留：压制但标题无编码器词 → 格式词照旧（不杜撰编码器）
	p3 := &TechProfile{SourceType: "BluRay", VideoCodec: "HEVC"}
	MergeMediaInfoInto(p3, &MediaInfoTech{VideoCodec: "HEVC", Encoded: true})
	if p3.VideoCodec != "HEVC" {
		t.Errorf("空隙残留应保持格式词: got %q, want HEVC", p3.VideoCodec)
	}
	// ④ WEB 门：流捕获无压制工具词 → 覆盖（v1.05 WEB 词表 H.264/H.265）
	p4 := &TechProfile{SourceType: "WEB-DL", VideoCodec: "x264"}
	MergeMediaInfoInto(p4, &MediaInfoTech{VideoCodec: "AVC", Encoded: false})
	if p4.VideoCodec != "AVC" {
		t.Errorf("WEB 应被 MI 覆盖: got %q, want AVC", p4.VideoCodec)
	}
	// ⑤ MI 自带编码器词（Writing library x265 形态）→ 覆盖（MI 证编码器更准）
	p5 := &TechProfile{SourceType: "BluRay", VideoCodec: "x264"}
	MergeMediaInfoInto(p5, &MediaInfoTech{VideoCodec: "x265", Encoded: true})
	if p5.VideoCodec != "x265" {
		t.Errorf("MI 编码器词应覆盖: got %q, want x265", p5.VideoCodec)
	}
	// ⑥ AVC 族同型：压制+标题 x264+MI AVC → x264 保留
	p6 := &TechProfile{SourceType: "BluRay", VideoCodec: "x264"}
	MergeMediaInfoInto(p6, &MediaInfoTech{VideoCodec: "AVC", Encoded: true})
	if p6.VideoCodec != "x264" {
		t.Errorf("AVC 族同型应保留: got %q, want x264", p6.VideoCodec)
	}
}

// §59.532 解挂：classifyMediumForCodec 连字符区分锚——驼峰 BLURAY=压制
// 写法（v1.05 :34/§59.349 连字符=原盘）→ encode；连字符 BLU-RAY/REMUX=disc。
// codecStyle 无 encode case（default 原样）——空隙残留语义。
func TestClassifyMediumForCodecHyphen(t *testing.T) {
	cases := map[string]string{
		"BluRay":     "encode", // 驼峰=压制写法（composeMedium 走 SourceType 此形态）
		"UHD BluRay": "encode",
		"Blu-ray":    "disc", // 连字符=原盘
		"UHD Blu-ray": "disc",
		"3D Blu-ray": "disc",
		"REMUX":      "disc",
		"BDRip":      "encode",
		"WEB-DL":     "web",
		"HDTV":       "hdtv",
	}
	for medium, want := range cases {
		if got := classifyMediumForCodec(medium); got != want {
			t.Errorf("classify(%q) = %q, want %q", medium, got, want)
		}
	}
}

// hasEncoderCodecWord §59.532 解挂：编码器词判定锚。
func TestHasEncoderCodecWord(t *testing.T) {
	if !hasEncoderCodecWord("x265") || !hasEncoderCodecWord("X264") {
		t.Error("x264/x265 应判编码器词")
	}
	if hasEncoderCodecWord("HEVC") || hasEncoderCodecWord("AVC") || hasEncoderCodecWord("") {
		t.Error("格式词/空不应判编码器词")
	}
}
