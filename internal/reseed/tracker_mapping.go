package reseed

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
)

var knownPrefixes = []string{
	"tracker.", "t.", "on.", "announce.", "daisuki.", "relay01.",
	"tracker-public.", "www.", "agsvpt.",
}

type TrackerSiteResolver struct {
	mu          sync.RWMutex
	domainToSite map[string]string
}

func NewTrackerSiteResolver() *TrackerSiteResolver {
	return &TrackerSiteResolver{
		domainToSite: make(map[string]string),
	}
}

func (r *TrackerSiteResolver) BuildIndex(sites []*model.Site) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := make(map[string]string)
	for _, s := range sites {
		siteName := s.Name

		register(m, s.Domain, siteName)

		if s.AlternativeDomains != "" {
			var altDomains []string
			if err := parseJSONStrings(s.AlternativeDomains, &altDomains); err == nil {
				for _, d := range altDomains {
					register(m, d, siteName)
				}
			}
		}

		if s.TrackerDomains != "" {
			var trackerDomains []string
			if err := parseJSONStrings(s.TrackerDomains, &trackerDomains); err == nil {
				for _, td := range trackerDomains {
					register(m, td, siteName)
				}
			}
		}
	}

	r.domainToSite = m
}

func register(m map[string]string, domain, siteName string) {
	d := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "/"))
	if d == "" {
		return
	}
	m[d] = siteName
}

func (r *TrackerSiteResolver) Resolve(trackerURL string) string {
	if trackerURL == "" {
		return ""
	}

	parsed, err := url.Parse(trackerURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if name, ok := r.domainToSite[host]; ok {
		return name
	}

	stripped := host
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(stripped, prefix) {
			stripped = stripped[len(prefix):]
			if name, ok := r.domainToSite[stripped]; ok {
				return name
			}
			break
		}
	}

	return ""
}

func parseJSONStrings(data string, out *[]string) error {
	return json.Unmarshal([]byte(data), out)
}
