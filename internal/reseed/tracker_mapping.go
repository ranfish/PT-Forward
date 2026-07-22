package reseed

import (
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
)

// TrackerSiteResolver 将 tracker URL 解析为站点名。
// v0.0.267: 统一架构 — 内部代理 site.TrackerMatcher，消除 3 套独立匹配逻辑。
type TrackerSiteResolver struct {
	matcher *site.TrackerMatcher
}

func NewTrackerSiteResolver() *TrackerSiteResolver {
	return &TrackerSiteResolver{
		matcher: site.NewTrackerMatcherFromSites(nil),
	}
}

func (r *TrackerSiteResolver) BuildIndex(sites []*model.Site) {
	list := make([]model.Site, len(sites))
	for i, s := range sites {
		list[i] = *s
	}
	r.matcher = site.NewTrackerMatcherFromSites(list)
}

func (r *TrackerSiteResolver) Resolve(trackerURL string) string {
	return r.matcher.Match(trackerURL)
}
