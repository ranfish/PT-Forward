package seeding

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

func BenchmarkCalculateScore(b *testing.B) {
	input := ScoreInput{
		Seeders:       100,
		Leechers:      50,
		AgeHours:      24.0,
		Size:          10 << 30,
		Discount:      model.Discount2xFree,
		HalfLifeHours: 2.0,
		SiteWeight:    1.5,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateScore(input)
	}
}

func BenchmarkCalculateScore_NoDemand(b *testing.B) {
	input := ScoreInput{
		Seeders:       0,
		Leechers:      0,
		AgeHours:      720.0,
		Size:          5 << 30,
		Discount:      model.DiscountNone,
		HalfLifeHours: 2.0,
		SiteWeight:    1.0,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateScore(input)
	}
}

func BenchmarkCalculateCleanupScore(b *testing.B) {
	weights := DefaultCleanupWeights()
	candidate := CleanupCandidate{
		ID:            1,
		InfoHash:      "abc123def456",
		Score:         15.5,
		SeedTimeHours: 720,
		AgeHours:      1440,
		IsFree:        true,
		HasHR:         false,
		Discount:      model.DiscountFree,
		UploadSpeed:   1024 * 1024,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateCleanupScore(candidate, weights)
	}
}

func BenchmarkCalculateCleanupScore_WithHR(b *testing.B) {
	weights := DefaultCleanupWeights()
	freeEnd := time.Now().Add(24 * time.Hour)
	candidate := CleanupCandidate{
		ID:            2,
		InfoHash:      "def789abc012",
		Score:         8.0,
		SeedTimeHours: 48,
		AgeHours:      120,
		IsFree:        false,
		HasHR:         true,
		HRSeedTimeH:   72,
		HRStrategy:    "protect",
		Discount:      model.DiscountNone,
		UploadSpeed:   512 * 1024,
		FreeEndAt:     &freeEnd,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateCleanupScore(candidate, weights)
	}
}

func BenchmarkShouldCleanup(b *testing.B) {
	weights := DefaultCleanupWeights()
	candidate := CleanupCandidate{
		SeedTimeHours: 720,
		AgeHours:      1440,
		IsFree:        true,
		Discount:      model.DiscountFree,
		UploadSpeed:   1024 * 1024,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateCleanupScore(candidate, weights)
	}
}

func BenchmarkCalculateScore_Parallel(b *testing.B) {
	input := ScoreInput{
		Seeders:       100,
		Leechers:      50,
		AgeHours:      24.0,
		Size:          10 << 30,
		Discount:      model.Discount2xUp,
		HalfLifeHours: 2.0,
		SiteWeight:    1.5,
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r := CalculateScore(input)
			if r.Score < 0 {
				b.Fatal("unexpected")
			}
		}
	})
}
