package adapter

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type HTTPDoer struct {
	Client *http.Client
}

func NewHTTPDoer() *HTTPDoer {
	return &HTTPDoer{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func NewHTTPDoerWithSite(proxyURL string, skipSSLVerify bool) *HTTPDoer {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipSSLVerify,
		},
	}
	if proxyURL != "" {
		if pu, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	return &HTTPDoer{
		Client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

type Factory struct {
	logger *zap.Logger
}

func NewFactory(logger *zap.Logger) *Factory {
	return &Factory{logger: logger}
}

func (f *Factory) Create(framework string, doer *HTTPDoer) model.SiteAdapter {
	switch framework {
	case "nexusphp":
		return NewNexusPHPAdapter(doer, f.logger)
	case "tnode":
		return NewTNodeAdapter(doer, f.logger)
	case "mteam":
		return NewMTeamAdapter(doer, f.logger)
	case "unit3d":
		return NewUnit3DAdapter(doer, f.logger)
	case "gazelle":
		return NewGazelleAdapter(doer, f.logger)
	case "rousi":
		return NewRousiAdapter(doer, f.logger)
	case "luminance":
		return NewGenericAdapter("luminance", doer, f.logger)
	default:
		return NewGenericAdapter(framework, doer, f.logger)
	}
}

type FrameworkDefault struct {
	HashStrategy        string
	SizeStrategy        string
	IDStrategy          string
	IDPattern           string
	DownloadURLTemplate string
	RequiresSideLoading bool
}

var FrameworkDefaults = map[string]FrameworkDefault{
	"nexusphp": {
		HashStrategy:        "guid",
		SizeStrategy:        "enclosure",
		IDStrategy:          "query_param",
		IDPattern:           "id",
		DownloadURLTemplate: "download.php?id={id}&passkey={passkey}",
	},
	"unit3d": {
		HashStrategy:        "fake_from_id",
		SizeStrategy:        "desc_regex",
		IDStrategy:          "link_regex",
		IDPattern:           `/torrent/(\d+)`,
		RequiresSideLoading: true,
	},
	"gazelle": {
		HashStrategy:        "bencode",
		SizeStrategy:        "bencode",
		IDStrategy:          "link_regex",
		IDPattern:           `torrents\.php\?torrentid=(\d+)`,
		RequiresSideLoading: true,
	},
	"mteam": {
		HashStrategy: "guid",
		SizeStrategy: "enclosure",
		IDStrategy:   "query_param",
		IDPattern:    "id",
	},
	"luminance": {
		HashStrategy:        "xml_tag",
		SizeStrategy:        "xml_tag",
		IDStrategy:          "query_param",
		IDPattern:           "id",
		RequiresSideLoading: true,
	},
	"tnode": {
		HashStrategy:        "guid",
		SizeStrategy:        "enclosure",
		IDStrategy:          "query_param",
		IDPattern:           "id",
		DownloadURLTemplate: "download.php?id={id}&passkey={passkey}",
	},
	"rousi": {
		HashStrategy: "guid",
		SizeStrategy: "enclosure",
		IDStrategy:   "path_segment",
		IDPattern:    "uuid",
	},
	"generic": {
		HashStrategy: "guid",
		SizeStrategy: "enclosure",
		IDStrategy:   "query_param",
		IDPattern:    "id",
	},
}
