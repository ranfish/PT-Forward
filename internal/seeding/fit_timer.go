package seeding

import "github.com/ranfish/pt-forward/internal/rule"

type FitTimer = rule.FitTimer

func NewFitTimer() *FitTimer {
	return rule.NewFitTimer()
}
