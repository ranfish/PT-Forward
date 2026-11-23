package reseed

import (
	"testing"
)

// §59.559 PR-E: reseed 匹配决策门特征锚——PR-B 拆分产物（verify/l0 文件）
// 的判定单元锁定。

// TestTitleKeywordStateConfirm §59.184: 标题门三态——确认/反驳/中性。
// meaningfulWords 命中候选标题=确认；CJK 源标题不命中=反驳；无信号=中性。
func TestTitleKeywordStateConfirm(t *testing.T) {
	// 确认：关键词在候选标题中
	state := titleKeywordState(
		[]string{"stand", "me"}, []string{},
		"Stand by Me 1986", "Stand.By.Me.1986 1080p",
	)
	if state != titleStateConfirm {
		t.Errorf("关键词命中应 confirm: %v", state)
	}
	// 反驳：源标题有明确续集号差异（续集号提取在首参 sourceTitle）
	state2 := titleKeywordState(
		[]string{}, []string{},
		"Show S01 2020", "Show S02 2021",
	)
	if state2 != titleStateRefute {
		t.Errorf("续集号差异应 refute: %v", state2)
	}
	// 中性：无信号
	state3 := titleKeywordState(
		nil, nil, "A", "B",
	)
	if state3 != titleStateNeutral {
		t.Errorf("无信号应 neutral: %v", state3)
	}
}

// TestResolutionEquivalent §59.184: 分辨率显示变体同义。
func TestResolutionEquivalent(t *testing.T) {
	if !resolutionEquivalent("4K", "2160p") {
		t.Error("4K≡2160p 应 true")
	}
	if !resolutionEquivalent("8K", "4320p") {
		t.Error("8K≡4320p 应 true")
	}
	if resolutionEquivalent("1080p", "2160p") {
		t.Error("1080p≠2160p 应 false")
	}
	if resolutionEquivalent("", "1080p") {
		t.Error("空≠1080p 应 false")
	}
}

// TestSourceTypeEquivalent §59.185: 源类型等价（剥连字符/空格+大小写不敏感）。
func TestSourceTypeEquivalent(t *testing.T) {
	if !sourceTypeEquivalent("WEB-DL", "webdl") {
		t.Error("WEB-DL≡webdl 应 true")
	}
	if !sourceTypeEquivalent("Blu-ray", "bluray") {
		t.Error("Blu-ray≡bluray 应 true")
	}
	if sourceTypeEquivalent("WEB-DL", "BDRip") {
		t.Error("WEB-DL≠BDRip 应 false")
	}
}

// TestCodecEquivalent: 编码等价。
func TestCodecEquivalent(t *testing.T) {
	if !codecEquivalent("x264", "x264") {
		t.Error("x264≡x264 应 true（精确匹配）")
	}
	if codecEquivalent("x264", "x265") {
		t.Error("x264≠x265 应 false")
	}
	if !codecEquivalent(" H.264 ", "H.264") {
		t.Error("trim 后应 true")
	}
}

// TestCompareSizeDisplay §59.184 v3: 显示舍入等价+1MB 地板（int64 字节）。
func TestCompareSizeDisplay(t *testing.T) {
	// 等价：舍入半窗内（1.36GB 显示档的同源字节差）
	if !CompareSizeDisplay(1468006400, 1468006401) {
		t.Error("1 字节差应等价（舍入窗内）")
	}
	// 不等价：差异超 1MB 地板
	if CompareSizeDisplay(734003200, 1500000000) {
		t.Error("700MB vs 1.4GB 应不等价")
	}
}
