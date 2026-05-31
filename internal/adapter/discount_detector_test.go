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
	html := `<html><body><span>限时freeleech活动</span></body></html>`
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
	assert.Equal(t, model.Discount2xUp, result.Level)
}

func TestDetectDiscountFromHTML_CustomMappingFallbackToKeywords(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my-site-free": "FREE",
		},
	}

	html := `<html><body><span>freeleech限时</span></body></html>`
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

func TestDetectDiscountFromHTML_Default30p(t *testing.T) {
	html := `<html><body><span class="pro_30p">30%</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountPercent30, result.Level)
}

func TestDetectDiscountFromHTML_Default2x50pctdown(t *testing.T) {
	html := `<html><body><span class="pro_2x50pctdown">2x50</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2x50, result.Level)
}

func TestDetectDiscountFromHTML_KeywordFreeLower(t *testing.T) {
	html := `<html><body><span>This is freeleech download</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_Keyword2xFreeChinese_MatchesFreeFirst(t *testing.T) {
	html := `<html><body><span>2x免费</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xFree, result.Level)
}

func TestDetectDiscountFromHTML_Keyword2xFreeSpace_Matches2xFreeFirst(t *testing.T) {
	html := `<html><body><span>2x Free</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xFree, result.Level)
}

func TestDetectDiscountFromHTML_Keyword2xFreeNoSpace_Matches2xFreeFirst(t *testing.T) {
	html := `<html><body><span>2xFree</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xFree, result.Level)
}

func TestDetectDiscountFromHTML_KeywordDoubleUpload(t *testing.T) {
	html := `<html><body><span>Double Upload</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xUp, result.Level)
}

func TestDetectDiscountFromHTML_Keyword2xUpload(t *testing.T) {
	html := `<html><body><span>2x Upload</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.Discount2xUp, result.Level)
}

func TestDetectDiscountFromHTML_Keyword50pFree(t *testing.T) {
	html := `<html><body><span>50% Free</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountPercent50, result.Level)
}

func TestDetectDiscountFromHTML_KeywordHalfDownload(t *testing.T) {
	html := `<html><body><span>Half Download</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountPercent50, result.Level)
}

func TestDetectDiscountFromHTML_ClassMapping2xUp(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my_2xup": "2XUP",
		},
	}
	html := `<html><body><span class="my_2xup">2x</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.Discount2xUp, result.Level)
}

func TestDetectDiscountFromHTML_ClassMapping2xFree(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my_2xfree": "2XFREE",
		},
	}
	html := `<html><body><span class="my_2xfree">2xfree</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.Discount2xFree, result.Level)
}

func TestDetectDiscountFromHTML_ClassMappingPercent50(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my_half": "PERCENT_50",
		},
	}
	html := `<html><body><span class="my_half">50%</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountPercent50, result.Level)
}

func TestDetectDiscountFromHTML_ClassMappingPercent30(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"my_30": "PERCENT_30",
		},
	}
	html := `<html><body><span class="my_30">30%</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountPercent30, result.Level)
}

func TestDetectDiscountFromHTML_EmptyHTML(t *testing.T) {
	result := DetectDiscountFromHTML("", nil)
	assert.Equal(t, model.DiscountNone, result.Level)
}

func TestDetectDiscountFromHTML_NilConfigDefaultPath(t *testing.T) {
	html := `<html><body><span class="pro_free">Free</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_EmptyMappingFallsToKeywords(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{},
	}
	html := `<html><body><span class="pro_free">Free</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_ClassMappingKeywordFallback(t *testing.T) {
	cfg := &model.SiteDiscountDetectionConfig{
		DiscountClassMapping: map[string]string{
			"nonexistent_class": "FREE",
		},
	}
	html := `<html><body><span>double upload</span></body></html>`
	result := DetectDiscountFromHTML(html, cfg)
	assert.Equal(t, model.Discount2xUp, result.Level)
}

func TestDetectDiscountFromDetailsPage_ExtractsH1Only(t *testing.T) {
	html := `<html><body><h1>Title <img class="pro_free" src="x.gif" alt="Free" /></h1><table><tr><td><img class="pro_50p" /></td></tr></table></body></html>`
	result := DetectDiscountFromDetailsPage(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromDetailsPage_IgnoresSimilarTorrents(t *testing.T) {
	html := `<html><body><h1>Target Torrent Title</h1><table class="details"><tr><td>info</td></tr></table><h2>Similar</h2><table><tr><td><img class="pro_free" src="x.gif" /></td></tr></table></body></html>`
	result := DetectDiscountFromDetailsPage(html, nil)
	assert.Equal(t, model.DiscountNone, result.Level)
}

func TestDetectDiscountFromDetailsPage_FreeInH1(t *testing.T) {
	html := `<html><body><h1>Torrent <img class="pro_free" src="x.gif" alt="Free" title="免费" /> (限时)</h1><table><tr><td>no discount here</td></tr></table></body></html>`
	result := DetectDiscountFromDetailsPage(html, nil)
	assert.Equal(t, model.DiscountFree, result.Level)
}

func TestDetectDiscountFromHTML_NoBareFreeKeyword(t *testing.T) {
	html := `<html><body><span>Free Space: 100GB</span></body></html>`
	result := DetectDiscountFromHTML(html, nil)
	assert.Equal(t, model.DiscountNone, result.Level)
}
