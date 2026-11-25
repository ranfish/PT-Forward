package description

import (
	"strings"
	"testing"
)

// §59.577 刀2: 声明三辨门锚——官组/外站组/未知各归各模板·nil checker 官组兼容。
func TestComposeStatementThreeWayGate(t *testing.T) {
	// nil checker → 全按官组（兼容缺省——缺口 3 甲）
	stmt := ComposeStatement("", "家园", "Some Movie 2020 1080p-MAMA")
	if !strings.Contains(stmt, "MAMA官组作品") {
		t.Errorf("nil checker 应官组兼容: %q", stmt)
	}
	if !strings.Contains(stmt, ThanksCoreText) {
		t.Errorf("尾句（互珍互重）应在: %q", stmt)
	}

	// 外站组 → 小组模板
	SetGroupKindChecker(func(g string) GroupKind {
		if g == "MAMA" {
			return GroupKindExternal
		}
		if g == "HHWEB" {
			return GroupKindOfficial
		}
		return GroupKindUnknown
	})
	defer SetGroupKindChecker(nil)

	stmt2 := ComposeStatement("", "家园", "Some Movie 2020 1080p-MAMA")
	if !strings.Contains(stmt2, "MAMA小组作品，感谢原制作者发布。") {
		t.Errorf("外站组应小组模板: %q", stmt2)
	}
	if !strings.Contains(stmt2, ThanksCoreText) {
		t.Errorf("尾句应保留（只换第一句）: %q", stmt2)
	}
	if strings.Contains(stmt2, "官组作品") {
		t.Errorf("外站组不应官组称谓: %q", stmt2)
	}

	// 官组 → 官组模板（现状不动）
	stmt3 := ComposeStatement("", "憨憨", "Some Movie 2020 1080p-HHWEB")
	if !strings.Contains(stmt3, "HHWEB官组作品，感谢原制作者发布。") {
		t.Errorf("官组应官组模板: %q", stmt3)
	}

	// 未知 → 不合成（诚实）
	stmt4 := ComposeStatement("原始声明", "家园", "Some Movie 2020 1080p-UNKNOWNGRP")
	if stmt4 != "原始声明" {
		t.Errorf("未知组应不合成: %q", stmt4)
	}

	// 已含我方致谢（双锚）→ 不双加
	existing := BuildExternalGroupThanks("家园", "MAMA")
	stmt5 := ComposeStatement(existing, "家园", "Some Movie 2020 1080p-MAMA")
	if strings.Count(stmt5, "小组作品") != 1 {
		t.Errorf("已含小组致谢不应双加: %q", stmt5)
	}
	existing6 := BuildOfficialThanks("憨憨", "HHWEB")
	stmt6 := ComposeStatement(existing6, "家园", "Some Movie 2020 1080p-MAMA")
	if strings.Count(stmt6, "小组作品") != 0 || strings.Count(stmt6, "官组作品") != 1 {
		t.Errorf("已含官组致谢时外站组不应再加: %q", stmt6)
	}
}

// §59.577 刀2: 双锚判别锚——官组作品∨小组作品。
func TestHasOfficialThanksDualAnchor(t *testing.T) {
	if !HasOfficialThanks("[quote]XX官组作品，感谢原制作者发布。[/quote]") {
		t.Error("官组形态应命中")
	}
	if !HasOfficialThanks("[quote]MAMA小组作品，感谢原制作者发布。[/quote]") {
		t.Error("小组形态应命中（刀2 新形态）")
	}
	if HasOfficialThanks("普通声明无锚词") {
		t.Error("无锚词不应命中")
	}
}

// §59.577 刀2: 小组模板形态锚——与官组块同构（只换第一句）。
func TestBuildExternalGroupThanksForm(t *testing.T) {
	got := BuildExternalGroupThanks("家园", "MAMA")
	if !strings.Contains(got, "[quote][b][color=blue][size=5]MAMA小组作品，感谢原制作者发布。[/size][/color][/b][/quote]") {
		t.Errorf("head 形态不符: %q", got)
	}
	if !strings.Contains(got, "[quote][b][color=red][size=5]"+ThanksCoreText+"[/size][/color][/b][/quote]") {
		t.Errorf("tail 形态不符: %q", got)
	}
}
