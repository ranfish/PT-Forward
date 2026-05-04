package seeding

import (
	"math"
	"time"
)

type ScoreInput struct {
	Seeders       int
	Leechers      int
	UploadBytes   int64
	SeedTimeHours float64
	AgeHours      float64
	Size          int64
	IsFree        bool
	HalfLifeHours float64
}

type ScoreResult struct {
	Score            float64
	LeecherSeedRatio float64
	DecayFactor      float64
	EffectiveScore   float64
}

func CalculateScore(input ScoreInput) ScoreResult {
	if input.Seeders <= 0 {
		input.Seeders = 1
	}

	leecherRatio := float64(input.Leechers) / float64(input.Seeders)

	uploadWeight := 1.0
	if input.UploadBytes > 0 && input.Size > 0 {
		uploadWeight = float64(input.UploadBytes) / float64(input.Size)
	}

	decay := 1.0
	if input.HalfLifeHours > 0 && input.AgeHours > 0 {
		decay = math.Pow(0.5, input.AgeHours/input.HalfLifeHours)
	}

	freeMultiplier := 1.0
	if input.IsFree {
		freeMultiplier = 1.5
	}

	rawScore := leecherRatio * uploadWeight * freeMultiplier
	effectiveScore := rawScore * decay

	return ScoreResult{
		Score:            rawScore,
		LeecherSeedRatio: leecherRatio,
		DecayFactor:      decay,
		EffectiveScore:   effectiveScore,
	}
}

type CleanupCandidate struct {
	ID            uint
	InfoHash      string
	Score         float64
	SeedTimeHours float64
	AgeHours      float64
	IsFree        bool
	HasHR         bool
}

type CleanupWeights struct {
	SeedHours   float64
	UploadSpeed float64
	Ratio       float64
	DiskUsage   float64
}

func DefaultCleanupWeights() CleanupWeights {
	return CleanupWeights{
		SeedHours:   0.3,
		UploadSpeed: 0.3,
		Ratio:       0.2,
		DiskUsage:   0.2,
	}
}

func CalculateCleanupScore(candidate CleanupCandidate, weights CleanupWeights) float64 {
	seedScore := 0.0
	if candidate.SeedTimeHours > 720 {
		seedScore = 1.0
	} else {
		seedScore = candidate.SeedTimeHours / 720.0
	}

	freeBonus := 0.0
	if candidate.IsFree {
		freeBonus = 0.5
	}

	hrPenalty := 0.0
	if candidate.HasHR {
		hrPenalty = 1.0
	}

	score := weights.SeedHours*seedScore +
		weights.Ratio*freeBonus -
		hrPenalty -
		weights.DiskUsage*(1.0/(1.0+candidate.AgeHours/168.0))

	return score
}

func ShouldCleanup(candidate CleanupCandidate, minScore float64, minAgeHours float64) bool {
	if candidate.AgeHours < minAgeHours {
		return false
	}
	if candidate.HasHR {
		return false
	}
	if candidate.IsFree {
		return false
	}
	return candidate.Score < minScore
}

type TimeWindow struct {
	StartHour int
	EndHour   int
	Days      []string
}

func (tw TimeWindow) Contains(t time.Time) bool {
	hour := t.Hour()
	if hour < tw.StartHour || hour >= tw.EndHour {
		return false
	}
	if len(tw.Days) == 0 {
		return true
	}
	day := t.Format("Mon")
	for _, d := range tw.Days {
		if d == day {
			return true
		}
	}
	return false
}
