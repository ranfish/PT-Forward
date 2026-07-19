package sites

import (
	"regexp"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// hdDolbyExtractor 不可杜站特殊提取器。
// PTNexus sites/hddolby.go（220 行）。
type hdDolbyExtractor struct{}

func newHDDolbyExtractor() *hdDolbyExtractor { return &hdDolbyExtractor{} }

func (e *hdDolbyExtractor) Name() string { return "hddolby_special" }

// div#kimdb 中 IMDb 链接
var kimdbImdbRe = regexp.MustCompile(`(?is)id="kimdb"[^>]*>.*?imdb\.com/title/(tt\d+)`)

func (e *hdDolbyExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// HDDolby 特殊: div#kimdb 独立容器提取 IMDb（非简介中）
	if seed.IMDbLink == "" {
		if m := kimdbImdbRe.FindStringSubmatch(input.PageHTML); len(m) > 1 {
			seed.IMDbLink = "https://www.imdb.com/title/" + m[1]
		}
	}
	// l_ratio_poster 海报已在公共提取器的海报关键词中（doubanio）
	return seed, nil
}
