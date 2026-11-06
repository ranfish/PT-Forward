package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.472: 海胆季/集/合集注入锚——五形态表驱动（From S04 案）。
func TestHaidanSeasonEpisodeInject(t *testing.T) {
	cfg := &model.PublishFormConfig{
		FormFields: map[string]string{
			model.FieldDomainSeason:   "season",
			model.FieldDomainEpisode:  "episode",
			model.FieldDomainCollages: "collages",
		},
	}
	series := "category.tv_series"
	movie := "category.movie"

	cases := []struct {
		name     string
		category string
		se       string
		season   string
		episode  string
		collages string
	}{
		{"S04 纯季（From 案·全季）", series, "S04", "4", "0", ""},
		{"S01-S02 跨季（勾合集）", series, "S01-S02", "0", "0", "1"},
		{"S04E01 单集（全集豁免形态）", series, "S04E01", "4", "1", ""},
		{"剧集无季信号（国产剧全集）", series, "", "0", "0", ""},
		{"电影（范围外不填）", movie, "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			form := map[string]string{}
			meta := &model.TorrentMetadata{Category: c.category}
			tp := titleparser.TechProfile{SeasonEpisode: c.se}
			haidanSeasonEpisodeInject(form, cfg, meta, &tp)
			if got := form["season"]; got != c.season {
				t.Errorf("season = %q, want %q", got, c.season)
			}
			if got := form["episode"]; got != c.episode {
				t.Errorf("episode = %q, want %q", got, c.episode)
			}
			if got := form["collages"]; got != c.collages {
				t.Errorf("collages = %q, want %q", got, c.collages)
			}
		})
	}
	// 未配置 season 键（其它站）零动作
	t.Run("未配置站零动作", func(t *testing.T) {
		form := map[string]string{}
		empty := &model.PublishFormConfig{FormFields: map[string]string{}}
		haidanSeasonEpisodeInject(form, empty, &model.TorrentMetadata{Category: series}, &titleparser.TechProfile{SeasonEpisode: "S04"})
		if len(form) != 0 {
			t.Errorf("未配置站应零动作, got %v", form)
		}
	})
}
