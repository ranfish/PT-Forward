package seeding

import (
	"math"
	"testing"
)

func TestCalculateScore_Basic(t *testing.T) {
	input := ScoreInput{
		Seeders:       10,
		Leechers:      50,
		UploadBytes:   1073741824,
		Size:          1073741824,
		SeedTimeHours: 24,
		AgeHours:      48,
		HalfLifeHours: 2,
	}

	result := CalculateScore(input)
	if result.LeecherSeedRatio != 5.0 {
		t.Errorf("expected leecher ratio 5.0, got %f", result.LeecherSeedRatio)
	}
	if result.DecayFactor <= 0 {
		t.Error("decay should be positive")
	}
	if result.EffectiveScore <= 0 {
		t.Error("effective score should be positive")
	}
}

func TestCalculateScore_FreeBonus(t *testing.T) {
	base := ScoreInput{Seeders: 10, Leechers: 50, UploadBytes: 1000, Size: 1000, AgeHours: 1}
	freeBase := base
	freeBase.IsFree = true

	normal := CalculateScore(base)
	free := CalculateScore(freeBase)

	if free.Score <= normal.Score {
		t.Error("free should score higher than normal")
	}
	if math.Abs(free.Score/normal.Score-1.5) > 0.01 {
		t.Errorf("free multiplier should be 1.5x, got %f", free.Score/normal.Score)
	}
}

func TestCalculateScore_Decay(t *testing.T) {
	input := ScoreInput{
		Seeders:       1,
		Leechers:      10,
		AgeHours:      100,
		HalfLifeHours: 10,
	}

	result := CalculateScore(input)
	expected := math.Pow(0.5, 10.0)
	if math.Abs(result.DecayFactor-expected) > 0.001 {
		t.Errorf("decay = %f, want %f", result.DecayFactor, expected)
	}
}

func TestCalculateScore_ZeroSeeders(t *testing.T) {
	input := ScoreInput{Seeders: 0, Leechers: 5}
	result := CalculateScore(input)
	if result.LeecherSeedRatio != 5.0 {
		t.Errorf("should default seeders to 1, got ratio %f", result.LeecherSeedRatio)
	}
}

func TestCleanupScore(t *testing.T) {
	weights := DefaultCleanupWeights()

	candidate := CleanupCandidate{
		ID:            1,
		SeedTimeHours: 1000,
		AgeHours:      500,
		IsFree:        false,
		HasHR:         false,
	}

	score := CalculateCleanupScore(candidate, weights)
	if score <= 0 {
		t.Errorf("long-seeded non-free should have positive score, got %f", score)
	}
}

func TestCleanupScore_HR(t *testing.T) {
	weights := DefaultCleanupWeights()

	normal := CleanupCandidate{SeedTimeHours: 100, AgeHours: 500, HasHR: false}
	withHR := CleanupCandidate{SeedTimeHours: 100, AgeHours: 500, HasHR: true}

	normalScore := CalculateCleanupScore(normal, weights)
	hrScore := CalculateCleanupScore(withHR, weights)

	if hrScore >= normalScore {
		t.Error("HR candidate should have lower cleanup score")
	}
}

func TestShouldCleanup(t *testing.T) {
	tests := []struct {
		name      string
		candidate CleanupCandidate
		result    bool
	}{
		{"too young", CleanupCandidate{AgeHours: 10, Score: -5}, false},
		{"has HR", CleanupCandidate{AgeHours: 200, HasHR: true, Score: -10}, false},
		{"is free", CleanupCandidate{AgeHours: 200, IsFree: true, Score: -10}, false},
		{"eligible", CleanupCandidate{AgeHours: 200, Score: -5}, true},
		{"score too high", CleanupCandidate{AgeHours: 200, Score: 5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCleanup(tt.candidate, -1, 100)
			if got != tt.result {
				t.Errorf("got %v, want %v", got, tt.result)
			}
		})
	}
}

type CleanupCandidateInternal struct {
	ID            uint
	InfoHash      string
	Score         float64
	SeedTimeHours float64
	AgeHours      float64
	IsFree        bool
	HasHR         bool
}

// Re-test with proper types
func TestShouldCleanup_Detailed(t *testing.T) {
	if !ShouldCleanup(CleanupCandidate{AgeHours: 200, Score: -5}, -1, 100) {
		t.Error("eligible candidate should be cleanup-able")
	}
}
