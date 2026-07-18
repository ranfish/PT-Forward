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
