package site

import "testing"

// §59.580: 站点级禁转小组黑名单锚——数据存在性+双键匹配+Match
// 语义（瓷器 rules.php §6.2 站方明文 12 组·硬编码 supported_sites.json）。
//
// 阴性对照：supported_sites.json 删黑名单字段/Match 退恒 false →
// 锚 FAIL。
func TestBannedGroups_Hdwing(t *testing.T) {
	// 锚1: 数据存在性（domain 直查）
	bg := BannedGroupsForSite("hdwing.org")
	if bg == nil {
		t.Fatal("瓷器（hdwing.org）应配置 banned_groups——supported_sites.json 数据缺失")
	}
	if len(bg.Exact) < 11 || len(bg.Prefixes) < 2 {
		t.Errorf("黑名单数据不完整：exact=%d prefixes=%d（站方 12 组+别名展开应 11+2）", len(bg.Exact), len(bg.Prefixes))
	}

	// 锚2: 双键（站名「瓷器」回查）
	bg2 := BannedGroupsForSite("瓷器")
	if bg2 == nil {
		t.Error("站名「瓷器」应能回查到黑名单（辅种 SiteInfo 持站名形态）")
	}

	// 锚3: exact 精确（大小写不敏感）
	for _, g := range []string{"SeeHD", "seehd", "RARBG", "Mp4Ba", "BitsTV", "HQC"} {
		if !bg.Match(g) {
			t.Errorf("精确组 %q 应命中黑名单", g)
		}
	}

	// 锚4: 别名展开（HDArea↔EPiC 站方写法「HDArea（EPiC）」）
	if !bg.Match("HDArea") || !bg.Match("EPiC") {
		t.Error("HDArea 与 EPiC 双形态都应命中（站方括号别名展开）")
	}

	// 锚5: 前缀族（宁严不宽——彩虹岛官组映射 6 组全 CHD 前缀实证）
	for _, g := range []string{"CHD", "CHDBits", "CHDPAD", "CHDTV", "CHDWEB", "CHDHKTV", "CHDRandom"} {
		if !bg.Match(g) {
			t.Errorf("前缀族组 %q 应命中（CHD* 宁严不宽）", g)
		}
	}
	if !bg.Match("Hyperay") || !bg.Match("HyperayX") {
		t.Error("Hyperay* 前缀族应命中")
	}

	// 锚6: 非命中（合法组不受影响）
	for _, g := range []string{"HDChina", "HDWinG", "FRDS", "WiLDCAT", "PTer"} {
		if bg.Match(g) {
			t.Errorf("合法组 %q 不应命中黑名单（误拦）", g)
		}
	}

	// 锚7: 空值防御
	if bg.Match("") {
		t.Error("空组名不应命中")
	}
	var nilCfg *BannedGroupsConfig
	if nilCfg.Match("CHD") {
		t.Error("nil 配置不应命中")
	}

	// 锚8: 非黑名单站
	if GroupBannedOnSite("CHDBits", "hdarea.club") {
		t.Error("hdarea（未配置黑名单）不应拦")
	}
	if GroupBannedOnSite("CHDBits", "不存在的站") {
		t.Error("未知站不应拦")
	}

	// 锚9: GroupBannedOnSite 组合便捷
	if !GroupBannedOnSite("CHDBits", "瓷器") || !GroupBannedOnSite("CHDBits", "hdwing.org") {
		t.Error("GroupBannedOnSite 双键组合应命中")
	}
}
