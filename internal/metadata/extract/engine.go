package extract

import (
	"fmt"
	"time"
)

// Engine 提取器路由（§56.8）。
// 先按 SiteCode/SiteNickname 查特殊提取器，失败或无意义 → fallback public。
type Engine struct {
	public           Extractor
	specialByCode    map[string]Extractor
	specialByNick    map[string]Extractor
}

// NewEngine 创建 Engine。
// specials key 可以是 site_code（如 "pterclub"）或 site_nickname（如 "猫站"）。
func NewEngine(public Extractor, specials map[string]Extractor) *Engine {
	e := &Engine{
		public:        public,
		specialByCode: make(map[string]Extractor),
		specialByNick: make(map[string]Extractor),
	}
	for k, v := range specials {
		if isLikelySiteCode(k) {
			e.specialByCode[k] = v
		} else {
			e.specialByNick[k] = v
		}
	}
	return e
}

// isLikelySiteCode 简单启发式：site_code 全小写+ASCII（如 "pterclub"），
// site_nickname 含中文或大写字母（如 "猫站" / "HDHome"）。
func isLikelySiteCode(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// Extract 执行提取。
// 顺序：specialByCode[input.SiteCode] > specialByNick[input.SiteNickname] > public。
// 任一 special 失败或返回 IsMeaningful()=false，自动 fallback public。
// UsedFallback=true 仅当尝试过 special 但需要回退；无 special 直接用 public 时为 false。
//
// v0.0.238: 末尾统一标准化字段值（不论哪个 extractor 提取，都经过 LookupStandardKey）。
// 这样站点特殊提取器（如 PTer 的 extractPTerCategoryIcons）只需提供原始值，
// 标准化（如 "电视剧 (TV Series)" → "category.tv_series"）由 Engine 统一处理。
func (e *Engine) Extract(input Input) (SeedData, Meta) {
	start := time.Now()
	meta := Meta{}

	// 1. 尝试特殊提取器（在 Normalize 前判断 IsMeaningful，避免 FallbackTitle 干扰）
	triedSpecial := false
	if special := e.lookupSpecial(input); special != nil {
		triedSpecial = true
		seed, err := special.Extract(input)
		if err != nil {
			meta.FallbackReason = fmt.Sprintf("special %q error: %v", special.Name(), err)
		} else if seed.IsMeaningful() {
			// special 有有效结果，但仍 Normalize 裁剪空白 + 补齐默认容器
			seed = seed.NormalizeWithFallback(input.FallbackTitle)
			normalizeSeedFields(&seed)
			meta.ExtractorName = special.Name()
			meta.Duration = time.Since(start)
			return seed, meta
		} else {
			meta.FallbackReason = fmt.Sprintf("special %q returned non-meaningful result", special.Name())
		}
	}

	// 2. Fallback public
	if e.public != nil {
		seed, err := e.public.Extract(input)
		meta.UsedFallback = triedSpecial
		if err != nil {
			meta.Duration = time.Since(start)
			meta.ExtractorName = e.public.Name()
			if meta.FallbackReason == "" {
				meta.FallbackReason = fmt.Sprintf("public %q error: %v", e.public.Name(), err)
			}
			return SeedData{}, meta
		}
		seed = seed.NormalizeWithFallback(input.FallbackTitle)
		normalizeSeedFields(&seed)
		meta.ExtractorName = e.public.Name()
		meta.Duration = time.Since(start)
		return seed, meta
	}

	// 3. 无任何提取器可用
	meta.Duration = time.Since(start)
	if meta.FallbackReason == "" {
		meta.FallbackReason = "no extractor available"
	}
	return SeedData{}, meta
}

// normalizeSeedFields 统一标准化 seed 的所有结构化字段值。
// 把原始值（如 "电视剧 (TV Series)"）映射到标准键（如 "category.tv_series"）。
// 找不到映射时保留原值（不破坏可读性）。
//
// 注意：title/subtitle/intro 不标准化（保留原文），仅标准化 type/medium/codec/resolution/source/team。
func normalizeSeedFields(seed *SeedData) {
	seed.Type = standardizeFieldValue("type", seed.Type)
	seed.Medium = standardizeFieldValue("medium", seed.Medium)
	seed.VideoCodec = standardizeFieldValue("video_codec", seed.VideoCodec)
	seed.AudioCodec = standardizeFieldValue("audio_codec", seed.AudioCodec)
	seed.Resolution = standardizeFieldValue("resolution", seed.Resolution)
	seed.ReleaseGroup = standardizeFieldValue("team", seed.ReleaseGroup)
	seed.Source = standardizeFieldValue("source", seed.Source)
}

// lookupSpecial 按优先级查找特殊提取器（Code 优先于 Nickname）。
func (e *Engine) lookupSpecial(input Input) Extractor {
	if input.SiteCode != "" {
		if ex, ok := e.specialByCode[input.SiteCode]; ok {
			return ex
		}
	}
	if input.SiteNickname != "" {
		if ex, ok := e.specialByNick[input.SiteNickname]; ok {
			return ex
		}
	}
	return nil
}
