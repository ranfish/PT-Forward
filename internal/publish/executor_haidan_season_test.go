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

// §59.471 回归审核补锚：跨季判定边界（首版手写条件 S1-2 漏判 +
// S01E02-03 集区间误判——正则化后锚定）。
func TestHaidanCrossSeasonEdge(t *testing.T) {
	cases := []struct {
		se   string
		want string // collages 期望值
	}{
		{"S01-S02", "1"},   // 标准跨季
		{"S01-02", "1"},    // 尾季无 S 短形态
		{"S1-2", "1"},      // 单位数跨季（首版漏判形态）
		{"S04E02-03", ""},  // 集区间（首版误判形态——非跨季）
		{"S04", ""},        // 纯季不勾
	}
	for _, c := range cases {
		form := map[string]string{}
		cfg := &model.PublishFormConfig{FormFields: map[string]string{
			model.FieldDomainSeason: "season", model.FieldDomainEpisode: "episode", model.FieldDomainCollages: "collages",
		}}
		haidanSeasonEpisodeInject(form, cfg, &model.TorrentMetadata{Category: "category.tv_series"}, &titleparser.TechProfile{SeasonEpisode: c.se})
		if got := form["collages"]; got != c.want {
			t.Errorf("se=%q collages=%q, want %q", c.se, got, c.want)
		}
	}
}
