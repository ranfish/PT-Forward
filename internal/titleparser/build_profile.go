package titleparser

// BuildTechProfile 三源分层合并 → TechProfile（§56.34 决策 4 的公共入口）。
//
// 统一供 runAnalyze（手动转发 ⑫）和 fetchSingleTorrent（种子配置页）调用，
// 消除两条管线的 TechProfile 解析断裂（§59.26 管线统一）。
//
// 合并优先级：
//   - 技术参数 8 字段：MediaInfo > 标题
//   - 标题特有 8 字段：标题（唯一源）
//   - 媒介/分类 2 字段：DOM > 标题
func BuildTechProfile(title, mediaInfo string, domMedium, domResolution, domVideoCodec, domAudioCodec string) TechProfile {
	profile := ParseTitleTech(title)
	if mediaInfo != "" {
		miTech := ExtractMediaInfo(mediaInfo)
		MergeMediaInfoInto(&profile, &miTech)
	}
	MergeDOMInto(&profile, domMedium, domResolution, domVideoCodec, domAudioCodec)
	return profile
}
