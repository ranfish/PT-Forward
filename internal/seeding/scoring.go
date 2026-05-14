package seeding

import (
	"math"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

type ScoreInput struct {
	Seeders       int
	Leechers      int
	AgeHours      float64
	Size          int64
	Discount      model.DiscountLevel
	HalfLifeHours float64
	SiteWeight    float64
}

type ScoreResult struct {
	Score          float64
	DemandScore    float64
	UploadValue    float64
	RecencyFactor  float64
	EffectiveScore float64
}

func uploadValueFromDiscount(d model.DiscountLevel) float64 {
	switch d {
	case model.DiscountFree:
		return 1.0
	case model.Discount2xFree:
		return 2.0
	case model.Discount2xUp:
		return 1.5
	case model.Discount2x50:
		return 1.75
	case model.DiscountPercent25:
		return 0.875
	case model.DiscountPercent30:
		return 0.85
	case model.DiscountPercent50:
		return 0.75
	case model.DiscountPercent70:
		return 0.65
	case model.DiscountPercent75:
		return 0.625
	default:
		return 0.5
	}
}

func CalculateScore(input ScoreInput) ScoreResult {
	if input.HalfLifeHours <= 0 {
		input.HalfLifeHours = 2.0
	}
	if input.SiteWeight <= 0 {
		input.SiteWeight = 1.0
	}

	var demandScore float64
	switch {
	case input.Leechers == 0 && input.Seeders == 0:
		demandScore = 0
	case input.Seeders == 0 && input.Leechers > 0:
		demandScore = 99999
	case input.Leechers == 0:
		demandScore = 0
	default:
		demandScore = float64(input.Leechers) / float64(input.Seeders)
	}

	uploadValue := uploadValueFromDiscount(input.Discount)

	recencyFactor := 1.0 / (1.0 + input.AgeHours/input.HalfLifeHours)

	rawScore := demandScore * uploadValue * input.SiteWeight
	effectiveScore := rawScore * recencyFactor

	return ScoreResult{
		Score:          rawScore,
		DemandScore:    demandScore,
		UploadValue:    uploadValue,
		RecencyFactor:  recencyFactor,
		EffectiveScore: effectiveScore,
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
	HRSeedTimeH   int
	HRStrategy    string
	Discount      model.DiscountLevel
	UploadSpeed   int64
	FreeEndAt     *time.Time
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
	maxSeedHours := 720.0
	if candidate.SeedTimeHours >= maxSeedHours {
		seedScore = 1.0
	} else {
		seedScore = candidate.SeedTimeHours / maxSeedHours
	}

	uploadValue := uploadValueFromDiscount(candidate.Discount)

	hrPenalty := 0.0
	if candidate.HasHR && !hrReleased(candidate) {
		hrPenalty = 1.0
	}

	speedScore := 0.0
	if candidate.SeedTimeHours > 0 {
		speedScore = math.Min(float64(candidate.UploadSpeed)/1024.0/1024.0, 1.0)
	}

	score := weights.SeedHours*seedScore +
		weights.UploadSpeed*speedScore +
		weights.Ratio*uploadValue -
		hrPenalty -
		weights.DiskUsage*(1.0/(1.0+candidate.AgeHours/168.0))

	return score
}

func hrReleased(candidate CleanupCandidate) bool {
	if !candidate.HasHR {
		return true
	}
	if candidate.HRStrategy == "ignore" {
		return true
	}
	if candidate.HRSeedTimeH <= 0 {
		return false
	}
	return candidate.SeedTimeHours >= float64(candidate.HRSeedTimeH)
}

func ShouldCleanup(candidate CleanupCandidate, minScore float64, minAgeHours float64) bool {
	if candidate.AgeHours < minAgeHours {
		return false
	}
	if candidate.HasHR && !hrReleased(candidate) {
		return false
	}
	if candidate.IsFree {
		if candidate.FreeEndAt == nil || candidate.FreeEndAt.After(time.Now()) {
			return false
		}
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
