package titleparser

import "testing"

// TestMergeMediaInfoInto_HDRAuthoritative §59.541 附十 修3 回归锚：
// HDR 族「MI 在场即权威」——Video 段在场×hdr format 行无=SDR 铁证，
// 标题/声明的 HDR 词形不再存留（piggo 57291 拒案：fetch 时态标题 HDR
// 词×本地 MI 无 HDR 行→DB hdr=HDR10→站方按 MI 判 SDR 拒收）。
func TestMergeMediaInfoInto_HDRAuthoritative(t *testing.T) {
	// ① MI Video 段在场×无 HDR 行 → 标题 HDR 值清空（SDR 铁证）
	p := TechProfile{HDR: "HDR10"}
	MergeMediaInfoInto(&p, &MediaInfoTech{HasVideo: true, Resolution: "1080p"})
	if p.HDR != "" {
		t.Fatalf("① MI 在场无 HDR 行应清空标题值，got %q", p.HDR)
	}
	// ② MI Video 段在场×HDR 行有值 → MI 权威覆盖
	p = TechProfile{HDR: "HDR10"}
	MergeMediaInfoInto(&p, &MediaInfoTech{HasVideo: true, HDR: "DoVi HDR"})
	if p.HDR != "DoVi HDR" {
		t.Fatalf("② MI 有值应覆盖，got %q", p.HDR)
	}
	// ③ MI 无 Video 段（数据缺失）×HDR 值空 → 标题值保留（回退语义）
	p = TechProfile{HDR: "HDR10"}
	MergeMediaInfoInto(&p, &MediaInfoTech{})
	if p.HDR != "HDR10" {
		t.Fatalf("③ MI 缺失应保留标题值，got %q", p.HDR)
	}
	// ④ MI 无 Video 段但 HDR 提取有值（宽松输入形态）→ 覆盖（原语义）
	p = TechProfile{HDR: "SDR-ish"}
	MergeMediaInfoInto(&p, &MediaInfoTech{HDR: "HDR10"})
	if p.HDR != "HDR10" {
		t.Fatalf("④ MI HDR 有值无 Video 段应覆盖，got %q", p.HDR)
	}
}

// TestMergeMediaInfoInto_SpecEncodeMutex §59.541 附十 修3 回归锚：
// MIEncoded（Writing library 压制铁证）×Remux/Untouched 物理互斥
// ——铁证在→降格 Encode（§59.496 tags 层同语义延伸到 specification 层
// ——piggo 57292 拒案：源站错标 Remux×MI x265 压制→发布投 Remux 被拒）。
func TestMergeMediaInfoInto_SpecEncodeMutex(t *testing.T) {
	// ① MIEncoded×Remux → Encode
	p := TechProfile{Specification: "Remux"}
	MergeMediaInfoInto(&p, &MediaInfoTech{Encoded: true})
	if p.Specification != "Encode" {
		t.Fatalf("① 压制铁证×Remux 应降格 Encode，got %q", p.Specification)
	}
	// ② MIEncoded×无 spec → 不变（无值可降）
	p = TechProfile{}
	MergeMediaInfoInto(&p, &MediaInfoTech{Encoded: true})
	if p.Specification != "" {
		t.Fatalf("② 无 spec 不应凭空产出，got %q", p.Specification)
	}
	// ③ 无铁证×Remux → 保留（证据缺席忠实提取）
	p = TechProfile{Specification: "Remux"}
	MergeMediaInfoInto(&p, &MediaInfoTech{})
	if p.Specification != "Remux" {
		t.Fatalf("③ 无铁证应保留 Remux，got %q", p.Specification)
	}
	// ④ MIEncoded×Encode → 不变
	p = TechProfile{Specification: "Encode"}
	MergeMediaInfoInto(&p, &MediaInfoTech{Encoded: true})
	if p.Specification != "Encode" {
		t.Fatalf("④ Encode 应保持，got %q", p.Specification)
	}
	// ⑤ MIEncoded×WEB-DL → 不变（互斥只针对 Remux/Untouched 族）
	p = TechProfile{Specification: "WEB-DL"}
	MergeMediaInfoInto(&p, &MediaInfoTech{Encoded: true})
	if p.Specification != "WEB-DL" {
		t.Fatalf("⑤ WEB-DL 不在互斥族应保持，got %q", p.Specification)
	}
}

// TestExtractMediaInfo_HasVideoFlag §59.541 附十 修3: Video 段级探测置位。
func TestExtractMediaInfo_HasVideoFlag(t *testing.T) {
	// MI 含 Video 段 → HasVideo=true（即使提取 miss）
	miWithVideo := "General\nComplete name : x.mkv\nFormat : Matroska\nVideo\nFormat : HEVC\nWidth : 3840 pixels\n"
	tech := ExtractMediaInfo(miWithVideo)
	if !tech.HasVideo {
		t.Fatal("含 Video 段的 MI 应置位 HasVideo")
	}
	// MI 无 Video 段 → HasVideo=false
	miNoVideo := "General\nComplete name : x.mkv\nFormat : Matroska\nAudio\nFormat : FLAC\n"
	tech = ExtractMediaInfo(miNoVideo)
	if tech.HasVideo {
		t.Fatal("无 Video 段的 MI 不应置位 HasVideo")
	}
	// 空 MI → false
	if tech := ExtractMediaInfo(""); tech.HasVideo {
		t.Fatal("空 MI 不应置位 HasVideo")
	}
}
