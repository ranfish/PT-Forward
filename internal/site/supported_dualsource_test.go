package site

import "testing"

// §59.571 锚：IsPublishSupported 双源。
func TestIsPublishSupportedDualSource(t *testing.T) {
	// ① 源一：supported_sites.json 声明（存量 69 站）
	if !IsPublishSupported("pterclub.net") {
		t.Error("json 声明站应 true（pterclub 有 publish_supported=true）")
	}
	// ② 源二：json 未声明但注入 formConfigChecker → true
	var checkedDomains []string
	SetFormConfigChecker(func(domain string) bool {
		checkedDomains = append(checkedDomains, domain)
		return domain == "www.musopia.vip" // 模拟乌托邦 form_config enabled=true
	})
	defer SetFormConfigChecker(nil)

	if !IsPublishSupported("www.musopia.vip") {
		t.Error("form_config enabled 站应 true（源二派生）")
	}
	if !IsPublishSupported("pterclub.net") {
		t.Error("双源不互相覆盖——json 声明站仍 true")
	}
	// ③ 双源都无 → false
	if IsPublishSupported("nonexistent.example.com") {
		t.Error("白名单外+无 form_config 应 false")
	}
	// ④ 注入 nil 后退回仅源一（乌托邦 json 已补 true→仍 true）
	SetFormConfigChecker(nil)
	if !IsPublishSupported("www.musopia.vip") {
		t.Error("checker=nil 退回 json 源——乌托邦 json 已补 true 应 true")
	}
	// ⑤ 白名单外且 checker 有值 → 仍 false（强白名单前置）
	SetFormConfigChecker(func(domain string) bool { return true })
	defer SetFormConfigChecker(nil)
	if IsPublishSupported("nonexistent.example.com") {
		t.Error("白名单外站恒 false（强白名单前置门）")
	}
	_ = checkedDomains // 引用防 unused
}

// §59.571 锚：SetFormConfigChecker 注入/清退。
func TestSetFormConfigChecker(t *testing.T) {
	// 默认 nil
	SetFormConfigChecker(nil)
	if siteFormConfigEnabled("any.domain") {
		t.Error("nil checker 应 false")
	}
	// 注入后生效
	SetFormConfigChecker(func(domain string) bool { return domain == "test.com" })
	defer SetFormConfigChecker(nil)
	if !siteFormConfigEnabled("test.com") {
		t.Error("注入后应生效")
	}
	if siteFormConfigEnabled("other.com") {
		t.Error("注入后非目标域应 false")
	}
}
