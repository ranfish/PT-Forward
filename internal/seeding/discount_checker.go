package seeding

import (
	"context"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

type SiteDiscountChecker struct {
	siteProvider model.SiteInfoProvider
}

func NewSiteDiscountChecker(sp model.SiteInfoProvider) *SiteDiscountChecker {
	return &SiteDiscountChecker{siteProvider: sp}
}

func (c *SiteDiscountChecker) CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, error) {
	adapter, err := c.siteProvider.GetAdapter(ctx, siteName)
	if err != nil {
		return model.DiscountNone, err
	}
	cfg, err := c.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil {
		return model.DiscountNone, err
	}
	result, err := adapter.DetectDiscount(ctx, cfg, torrentID)
	if err != nil {
		return model.DiscountNone, err
	}
	if result == nil {
		return model.DiscountNone, nil
	}
	return result.Level, nil
}

func IsWithinActiveWindow(windows string) bool {
	windows = strings.TrimSpace(windows)
	if windows == "" {
		return true
	}

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	for _, w := range strings.Split(windows, ",") {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}

		parts := strings.SplitN(w, "-", 2)
		if len(parts) != 2 {
			continue
		}

		startMins := parseTimeOfDay(strings.TrimSpace(parts[0]))
		endMins := parseTimeOfDay(strings.TrimSpace(parts[1]))
		if startMins < 0 || endMins < 0 {
			continue
		}

		if startMins <= endMins {
			if currentMinutes >= startMins && currentMinutes <= endMins {
				return true
			}
		} else {
			if currentMinutes >= startMins || currentMinutes <= endMins {
				return true
			}
		}
	}

	return false
}

func parseTimeOfDay(s string) int {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h := atoiSimple(parts[0])
	m := atoiSimple(parts[1])
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

func atoiSimple(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}
