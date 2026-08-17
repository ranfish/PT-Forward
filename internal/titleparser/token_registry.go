// Package titleparser Token Registry（§59.27 → §59.35 v2）。
//
// v1.05 标题规范的技术 token 注册表。数据源：dict/*.json 分域文件（go:embed，
// 见 dict_loader.go）；本文件只保留派生出口与消费函数。
//
// 消费方（全部从 registry 派生，禁止再手写变体）：
//   - extractVideoCodec/extractAudio：首个命中（registry 顺序即优先级）
//   - removeVideoCodecTokens/removeAudioCodecTokens：从 Pattern 合成移除正则
//   - mediainfo_extractor codecFromMI/audioFromMI：MI 值 → Canonical
//   - ReassembleFromTechProfile：Canonical → TitleForm
//
// 核心不变量（round-trip，dict_consistency_test.go 持续护航）：
//   parse(reassemble(profile)) == profile
package titleparser


// compileToken 编译 token pattern（长 token 在前由 dict 文件顺序保证）。
// re() 方法在 dict_loader.go（v2：CaseSensitive 支持）。

// === 标题解析 registry（§59.35：dict 分域数据填充，顺序即提取优先级）===

var (
	videoCodecRegistry = DictTokens("video_codec")
	audioCodecRegistry = DictTokens("audio_codec")
	hdrTokenRegistry   = DictTokens("hdr")
	platformRegistry   = DictTokens("platform")
)

// lookupToken 从 registry 找首个命中 token（顺序即优先级）。
func lookupToken(registry []TokenDef, s string) string {
	for _, t := range registry {
		if t.matchesWithRequires(s, false) {
			return t.Canonical
		}
	}
	return ""
}

// lookupTokenWebContext 同 lookupToken，但 requires=web 的词条按 webContext 启用。
// webContext = 标题已证实为 WEB 类资源（含 WEB/HDTV/UHDTV token）。
func lookupTokenWebContext(registry []TokenDef, s string, webContext bool) string {
	for _, t := range registry {
		if t.matchesWithRequires(s, webContext) {
			return t.Canonical
		}
	}
	return ""
}

// titleFormOfToken canonical → 重组形式（§59.27 P3 重组显式出口）。
// registry 未收录的值原样返回（EditionInfo 等自由文本不经过 registry）。
func titleFormOfToken(registry []TokenDef, canonical string) string {
	if canonical == "" {
		return ""
	}
	for _, t := range registry {
		if t.Canonical == canonical {
			return t.titleForm()
		}
	}
	return canonical
}

// AudioTitleForm / VideoTitleForm 对外出口（重组器使用）。
func AudioTitleForm(canonical string) string { return titleFormOfToken(audioCodecRegistry, canonical) }
func VideoTitleForm(canonical string) string { return titleFormOfToken(videoCodecRegistry, canonical) }

// removeAllTokenPatterns 从 registry 合成移除正则（替代手写 remove*Tokens）。
// 返回移除所有变体后的字符串。requires=web 词条仅在 webContext 时移除
//（否则不提取也不剥词——2 字符缩写误命中不得污染主标题）。
func removeAllTokenPatterns(registry []TokenDef, s string, webContext bool) string {
	for _, t := range registry {
		if t.matchesWithRequires(s, webContext) {
			s = t.re().ReplaceAllString(s, " ")
		}
	}
	return s
}
