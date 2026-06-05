package site

import (
	"encoding/json"
	"fmt"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type SiteFormConfig struct {
	Category      []SelectOption `json:"category"`
	MediumSel     []SelectOption `json:"medium_sel"`
	CodecSel      []SelectOption `json:"codec_sel"`
	StandardSel   []SelectOption `json:"standard_sel"`
	AudioCodec    []SelectOption `json:"audio_codec"`
	TeamSel       []SelectOption `json:"team_sel"`
	ProcessingSel []SelectOption `json:"processing_sel"`
	SourceSel     []SelectOption `json:"source_sel"`
	Tags          []SelectOption `json:"tags"`

	MusicFormatSel  []SelectOption `json:"music_format_sel"`
	MusicMediumSel  []SelectOption `json:"music_medium_sel"`
	MusicPublishSel []SelectOption `json:"music_publish_sel"`
	MusicTags       []SelectOption `json:"music_tags"`
}

type SiteSeedData struct {
	Domain                string         `json:"domain"`
	Name                  string         `json:"name"`
	BaseURL               string         `json:"base_url"`
	Framework             string         `json:"framework"`
	AuthType              string         `json:"auth_type"`
	DownloadMode          string         `json:"download_mode"`
	IsSource              bool           `json:"is_source"`
	IsTarget              bool           `json:"is_target"`
	CookieCloudDomain     string         `json:"cookiecloud_domain"`
	AlternativeDomains    string         `json:"alternative_domains"`
	SupportsPiecesHashAPI *bool          `json:"supports_pieces_hash_api,omitempty"`
	Form                  SiteFormConfig `json:"form"`
}

type seedData struct {
	Sites      []SiteSeedData `json:"sites"`
	Exclusions []struct {
		TargetSite string `json:"target_site"`
		SourceSite string `json:"source_site"`
	} `json:"exclusions"`
	Overrides []struct {
		SiteName   string `json:"site_name"`
		FieldPath  string `json:"field_path"`
		FieldValue string `json:"field_value"`
	} `json:"overrides"`
}

var loaded *seedData

func loadSeedData() *seedData {
	if loaded != nil {
		return loaded
	}
	var data seedData
	if err := json.Unmarshal(sitesJSON, &data); err != nil {
		panic(fmt.Sprintf("failed to parse embedded sites.json: %v", err))
	}
	loaded = &data
	return loaded
}

func seedSites() []SiteSeedData {
	return loadSeedData().Sites
}

func SeedSites(db *gorm.DB) error {
	for _, s := range seedSites() {
		var existing model.Site
		err := db.Where("domain = ?", s.Domain).First(&existing).Error
		if err == nil {
			continue
		}

		defs, ok := adapter.FrameworkDefaults[s.Framework]
		if !ok {
			defs = adapter.FrameworkDefaults["generic"]
		}

		site := &model.Site{
			Domain:             s.Domain,
			Name:               s.Name,
			BaseURL:            s.BaseURL,
			Framework:          s.Framework,
			AuthType:           defaultAuthType(s.AuthType),
			Enabled:            true,
			IsSource:           s.IsSource,
			IsTarget:           s.IsTarget,
			CookieCloudDomain:  s.CookieCloudDomain,
			AlternativeDomains: s.AlternativeDomains,

			HashStrategy:        defs.HashStrategy,
			SizeStrategy:        defs.SizeStrategy,
			IDStrategy:          defs.IDStrategy,
			IDPattern:           defs.IDPattern,
			DownloadMode:        defaultDownloadMode(s.DownloadMode),
			DownloadURLTemplate: defs.DownloadURLTemplate,
			RequiresSideLoading: defs.RequiresSideLoading,
		}

		if s.SupportsPiecesHashAPI != nil {
			site.SupportsPiecesHashAPI = *s.SupportsPiecesHashAPI
		}

		if err := db.Create(site).Error; err != nil {
			return siteError(ErrSiteSeed, fmt.Sprintf("create site %s", s.Domain), err)
		}
	}

	return nil
}

func defaultAuthType(s string) string {
	if s != "" {
		return s
	}
	return "cookie"
}

func defaultDownloadMode(s string) string {
	if s != "" {
		return s
	}
	return "template"
}

func SeedFieldMappings(db *gorm.DB) error {
	for _, s := range seedSites() {
		fieldTypes := map[string][]SelectOption{
			"cat":               s.Form.Category,
			"medium_sel":        s.Form.MediumSel,
			"codec_sel":         s.Form.CodecSel,
			"standard_sel":      s.Form.StandardSel,
			"audiocodec_sel":    s.Form.AudioCodec,
			"team_sel":          s.Form.TeamSel,
			"processing_sel":    s.Form.ProcessingSel,
			"source_sel":        s.Form.SourceSel,
			"tags":              s.Form.Tags,
			"music_format_sel":  s.Form.MusicFormatSel,
			"music_medium_sel":  s.Form.MusicMediumSel,
			"music_publish_sel": s.Form.MusicPublishSel,
			"music_tags":        s.Form.MusicTags,
		}

		for fieldType, options := range fieldTypes {
			for _, opt := range options {
				if opt.Value == "" || opt.Label == "" {
					continue
				}

				var existing model.SiteFieldMapping
				err := db.Where("site_name = ? AND field_type = ? AND source_value = ?",
					s.Name, fieldType, opt.Label).First(&existing).Error
				if err == nil {
					continue
				}

				mapping := &model.SiteFieldMapping{
					SiteName:    s.Name,
					FieldType:   fieldType,
					SourceValue: opt.Label,
					TargetValue: opt.Value,
				}
				if err := db.Create(mapping).Error; err != nil {
					return siteError(ErrSiteSeed, fmt.Sprintf("create mapping %s/%s/%s", s.Name, fieldType, opt.Label), err)
				}
			}
		}
	}

	return nil
}

func SeedExclusions(db *gorm.DB) error {
	for _, e := range loadSeedData().Exclusions {
		var existing model.PublishExclusion
		err := db.Where("target_site = ? AND source_site = ?", e.TargetSite, e.SourceSite).First(&existing).Error
		if err == nil {
			continue
		}

		exclusion := &model.PublishExclusion{
			TargetSite: e.TargetSite,
			SourceSite: e.SourceSite,
		}
		if err := db.Create(exclusion).Error; err != nil {
			return siteError(ErrSiteSeed, fmt.Sprintf("create exclusion %s↔%s", e.TargetSite, e.SourceSite), err)
		}
	}
	return nil
}

func SeedFormFieldOverrides(db *gorm.DB) error {
	for _, o := range loadSeedData().Overrides {
		var existing model.SiteConfigOverride
		err := db.Where("site_name = ? AND field_path = ?", o.SiteName, o.FieldPath).First(&existing).Error
		if err == nil {
			continue
		}

		override := &model.SiteConfigOverride{
			SiteName:   o.SiteName,
			FieldPath:  o.FieldPath,
			FieldValue: o.FieldValue,
		}
		if err := db.Create(override).Error; err != nil {
			return siteError(ErrSiteSeed, fmt.Sprintf("create override %s/%s", o.SiteName, o.FieldPath), err)
		}
	}
	return nil
}
