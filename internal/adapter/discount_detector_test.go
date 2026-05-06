package adapter

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestDetectDiscountFromHTML_DefaultFree(t *testing.T) {
	html := `<html><body><span class="pro_free">Free</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_Default2xFree(t *testing.T) {
	html := `<html><body><span class="pro_free2up">2xFree</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xFree, result.Level)
}

func TestDetectDiscountFromHTML_Default2xUp(t *testing.T) {
	html := `<html><body><span class="pro_2up">2xUp</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xUp, result.Level)
}

func TestDetectDiscountFromHTML_Default50p(t *testing.T) {
	html := `<html><body><span class="pro_50p">50%</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountPercent50, result.Level)
}

func TestDetectDiscountFromHTML_DefaultNone(t *testing.T) {
	html := `<html><body><span class="normal">Normal</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountNone, result.Level)
}

func TestDetectDiscountFromHTML_KeywordFree(t *testing.T) {
	html := `<html><body><span>限时免费</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_KeywordFreeleech(t *testing.T) {
	html := `<html><body><span>FreeLeech</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_CustomMapping(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"custom_free_class": "FREE",
			"custom_2xup_class": "2XUP",
		},
	}

	html := `<html><body><span class="custom_free_class">Free</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_CustomMappingNoMatch(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my-site-free": "FREE",
		},
	}

	html := `<html><body><span class="pro_2up">2xUp</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountNone, result.Level)
}

func TestDetectDiscountFromHTML_CustomMappingFallbackToKeywords(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my-site-free": "FREE",
		},
	}

	html := `<html><body><span>限时免费</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_InvalidMappingLevel(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"custom_bad": "INVALID_LEVEL",
		},
	}

	html := `<html><body><span class="custom_bad">Bad</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountNone, result.Level)
}
