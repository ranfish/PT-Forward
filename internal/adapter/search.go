package adapter

import (
	"net/url"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

type searchDefaults struct {
	Browse  string
	Param   string
	Extra   string
}

var siteSearchDefaults = map[string]searchDefaults{
	"totheglory.im":  {Browse: "/browse.php", Param: "search_field"},
	"hdroute.org":    {Browse: "/browse.php", Param: "s", Extra: "dp=0&add=0&action=s&or=1&imdb="},
}

func buildSearchURL(config *model.SiteConfig, framework, keyword string) string {
	browsePath := config.Paths.Browse
	searchParam := config.Paths.SearchParam

	if browsePath == "" || searchParam == "" {
		if d, ok := siteSearchDefaults[config.Domain]; ok {
			if browsePath == "" {
				browsePath = d.Browse
			}
			if searchParam == "" {
				searchParam = d.Param
			}
		}
	}

	if browsePath == "" {
		switch framework {
		case "unit3d":
			browsePath = "/torrents"
		default:
			browsePath = "/torrents.php"
		}
	}
	if searchParam == "" {
		searchParam = "search"
	}

	u := resolveBaseURL(config) + browsePath
	suffix := searchParam + "=" + url.QueryEscape(keyword)
	if d, ok := siteSearchDefaults[config.Domain]; ok && d.Extra != "" {
		suffix += "&" + d.Extra
	}
	if strings.Contains(u, "?") {
		u += "&" + suffix
	} else {
		u += "?" + suffix
	}
	return u
}
