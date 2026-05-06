package seeding

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestCalculateScore_Basic(t *testing.T) {
	input := ScoreInput{
		Seeders:       10,
		Leechers:      50,
		AgeHours:      1,
		Size:          1024,
		Discount:      model.DiscountNone,
		HalfLifeHours: 2,
	}
	result := CalculateScore(input)
	if result.DemandScore != 5.0 {
		t.Errorf("demandScore = %v, want 5.0", result.DemandScore)
	}
	if result.UploadValue != 0.5 {
		t.Errorf("uploadValue = %v, want 0.5", result.UploadValue)
	}
	if result.EffectiveScore <= 0 {
		t.Errorf("effectiveScore should be positive, got %v", result.EffectiveScore)
	}
}

func TestCalculateScore_FreeBonus(t *testing.T) {
	base := ScoreInput{Seeders: 10, Leechers: 50, AgeHours: 1, HalfLifeHours: 2}
	freeBase := base
	freeBase.Discount = model.DiscountFree

	normal := CalculateScore(base)
	free := CalculateScore(freeBase)

	if free.UploadValue <= normal.UploadValue {
		t.Errorf("free uploadValue (%v) should be > normal (%v)", free.UploadValue, normal.UploadValue)
	}

	twoXFree := base
	twoXFree.Discount = model.Discount2xFree
	result2x := CalculateScore(twoXFree)
	if result2x.UploadValue <= free.UploadValue {
		t.Errorf("2XFREE (%v) should be > FREE (%v)", result2x.UploadValue, free.UploadValue)
	}
}

func TestCalculateScore_Decay(t *testing.T) {
	input := ScoreInput{
		Seeders:       10,
		Leechers:      50,
		AgeHours:      2,
		HalfLifeHours: 2,
		Discount:      model.DiscountNone,
	}
	result := CalculateScore(input)
	if result.RecencyFactor <= 0 || result.RecencyFactor > 0.5 {
		t.Errorf("recencyFactor at half-life should be ~0.5, got %v", result.RecencyFactor)
	}

	oldInput := input
	oldInput.AgeHours = 100
	oldResult := CalculateScore(oldInput)
	if oldResult.RecencyFactor >= result.RecencyFactor {
		t.Errorf("older seed should have smaller recencyFactor")
	}
}

func TestCalculateScore_ZeroSeeders(t *testing.T) {
	input := ScoreInput{Seeders: 0, Leechers: 5, Discount: model.DiscountNone}
	result := CalculateScore(input)
	if result.DemandScore != 99999 {
		t.Errorf("demandScore with 0 seeders and leechers>0 should be 99999, got %v", result.DemandScore)
	}
}

func TestCalculateScore_ZeroLeechers(t *testing.T) {
	input := ScoreInput{Seeders: 10, Leechers: 0, Discount: model.DiscountNone}
	result := CalculateScore(input)
	if result.DemandScore != 0 {
		t.Errorf("demandScore with 0 leechers should be 0, got %v", result.DemandScore)
	}
}

func TestCalculateScore_BothZero(t *testing.T) {
	input := ScoreInput{Seeders: 0, Leechers: 0, Discount: model.DiscountNone}
	result := CalculateScore(input)
	if result.DemandScore != 0 {
		t.Errorf("demandScore with 0 seeders and 0 leechers should be 0, got %v", result.DemandScore)
	}
}

func TestCalculateScore_SiteWeight(t *testing.T) {
	base := ScoreInput{Seeders: 10, Leechers: 50, AgeHours: 1, Discount: model.DiscountNone, SiteWeight: 1.0}
	weighted := base
	weighted.SiteWeight = 2.0

	normal := CalculateScore(base)
	boosted := CalculateScore(weighted)

	if boosted.Score <= normal.Score {
		t.Errorf("siteWeight=2.0 should double the raw score")
	}
}

func TestCleanupScore(t *testing.T) {
	weights := DefaultCleanupWeights()
	candidate := CleanupCandidate{
		SeedTimeHours: 360,
		AgeHours:      500,
		Discount:      model.DiscountFree,
	}
	score := CalculateCleanupScore(candidate, weights)
	if score <= 0 {
		t.Errorf("cleanup score should be positive for free seed, got %v", score)
	}
}

func TestCleanupScore_HR(t *testing.T) {
	weights := DefaultCleanupWeights()
	normal := CleanupCandidate{SeedTimeHours: 100, AgeHours: 500, Discount: model.DiscountNone}
	withHR := CleanupCandidate{SeedTimeHours: 10, AgeHours: 500, HasHR: true, HRSeedTimeH: 72, Discount: model.DiscountNone}

	normalScore := CalculateCleanupScore(normal, weights)
	hrScore := CalculateCleanupScore(withHR, weights)

	if hrScore >= normalScore {
		t.Errorf("HR penalty should reduce score: normal=%v, hr=%v", normalScore, hrScore)
	}
}

func TestCleanupScore_HR_Released(t *testing.T) {
	weights := DefaultCleanupWeights()
	released := CleanupCandidate{SeedTimeHours: 80, AgeHours: 500, HasHR: true, HRSeedTimeH: 72, Discount: model.DiscountNone}
	normal := CleanupCandidate{SeedTimeHours: 80, AgeHours: 500, HasHR: false, Discount: model.DiscountNone}

	releasedScore := CalculateCleanupScore(released, weights)
	normalScore := CalculateCleanupScore(normal, weights)

	if releasedScore < normalScore {
		t.Errorf("released HR should not have penalty: released=%v, normal=%v", releasedScore, normalScore)
	}
}

func TestCleanupScore_UploadSpeed(t *testing.T) {
	weights := DefaultCleanupWeights()
	slow := CleanupCandidate{SeedTimeHours: 100, AgeHours: 500, UploadSpeed: 100 * 1024, Discount: model.DiscountNone}
	fast := CleanupCandidate{SeedTimeHours: 100, AgeHours: 500, UploadSpeed: 2 * 1024 * 1024, Discount: model.DiscountNone}

	slowScore := CalculateCleanupScore(slow, weights)
	fastScore := CalculateCleanupScore(fast, weights)

	if fastScore <= slowScore {
		t.Errorf("higher upload speed should have higher score: fast=%v, slow=%v", fastScore, slowScore)
	}
}

func TestShouldCleanup(t *testing.T) {
	tests := []struct {
		name      string
		candidate CleanupCandidate
		want      bool
	}{
		{"too young", CleanupCandidate{AgeHours: 10, Score: -5}, false},
		{"has HR not released", CleanupCandidate{AgeHours: 200, HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 10, Score: -10}, false},
		{"has HR released", CleanupCandidate{AgeHours: 200, HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 80, Score: -5}, true},
		{"has HR ignore strategy", CleanupCandidate{AgeHours: 200, HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 10, HRStrategy: "ignore", Score: -5}, true},
		{"is free", CleanupCandidate{AgeHours: 200, IsFree: true, Score: -10}, false},
		{"is free expired", CleanupCandidate{AgeHours: 200, IsFree: true, Score: -10, FreeEndAt: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }()}, true},
		{"eligible", CleanupCandidate{AgeHours: 200, Score: -5}, true},
		{"score too high", CleanupCandidate{AgeHours: 200, Score: 5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCleanup(tt.candidate, -1, 100)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHRReleased(t *testing.T) {
	tests := []struct {
		name      string
		candidate CleanupCandidate
		want      bool
	}{
		{"no HR", CleanupCandidate{HasHR: false}, true},
		{"HR not enough seed time", CleanupCandidate{HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 50}, false},
		{"HR exact seed time", CleanupCandidate{HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 72}, true},
		{"HR more seed time", CleanupCandidate{HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 100}, true},
		{"HR zero required", CleanupCandidate{HasHR: true, HRSeedTimeH: 0, SeedTimeHours: 100}, false},
		{"HR ignore strategy", CleanupCandidate{HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 10, HRStrategy: "ignore"}, true},
		{"HR protect strategy", CleanupCandidate{HasHR: true, HRSeedTimeH: 72, SeedTimeHours: 10, HRStrategy: "protect"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hrReleased(tt.candidate)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldCleanup_Detailed(t *testing.T) {
	if !ShouldCleanup(CleanupCandidate{AgeHours: 200, Score: -5}, -1, 100) {
		t.Error("old low-score seed should be cleaned up")
	}
}

func TestUploadValueFromDiscount(t *testing.T) {
	tests := []struct {
		discount model.DiscountLevel
		want     float64
	}{
		{model.DiscountFree, 1.0},
		{model.Discount2xFree, 2.0},
		{model.Discount2xUp, 1.5},
		{model.Discount2x50, 1.75},
		{model.DiscountPercent50, 0.75},
		{model.DiscountPercent30, 0.85},
		{model.DiscountPercent70, 0.65},
		{model.DiscountNone, 0.5},
	}
	for _, tt := range tests {
		got := uploadValueFromDiscount(tt.discount)
		if got != tt.want {
			t.Errorf("uploadValueFromDiscount(%s) = %v, want %v", tt.discount, got, tt.want)
		}
	}
}

func TestDiscountLevel_Methods(t *testing.T) {
	if !model.DiscountFree.IsFree() {
		t.Error("FREE should be free")
	}
	if model.DiscountNone.IsFree() {
		t.Error("NONE should not be free")
	}
	if !model.Discount2xUp.IsFreeOrDiscount() {
		t.Error("2XUP should be free or discount")
	}
	if model.Discount2xFree.PriorityValue() <= model.DiscountFree.PriorityValue() {
		t.Error("2XFREE should have higher priority than FREE")
	}
	if !model.DiscountNone.IsValid() {
		t.Error("NONE should be valid")
	}
	if model.DiscountLevel("INVALID").IsValid() {
		t.Error("INVALID should not be valid")
	}
	if model.NewDiscountLevelFromBool(true) != model.DiscountFree {
		t.Error("NewDiscountLevelFromBool(true) should be FREE")
	}
	if model.NewDiscountLevelFromBool(false) != model.DiscountNone {
		t.Error("NewDiscountLevelFromBool(false) should be NONE")
	}
}
