package adapter

import (
	"net/http"

	"github.com/ranfish/pt-forward/internal/browser"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type HTTPDoer struct {
	Client *http.Client
}

func NewHTTPDoer() *HTTPDoer {
	return &HTTPDoer{
		Client: httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{}),
	}
}

func NewHTTPDoerWithSite(proxyURL string, skipSSLVerify bool, tlsPretend ...string) *HTTPDoer {
	cfg := httpclient.SiteHTTPConfig{
		ProxyURL:      proxyURL,
		SkipSSLVerify: skipSSLVerify,
	}
	if len(tlsPretend) > 0 {
		cfg.TLSPretend = tlsPretend[0]
	}
	return &HTTPDoer{
		Client: httpclient.NewSiteHTTPClient(cfg),
	}
}

// Factory 按框架名创建 adapter 实例。
// §56.13 方案 B: engine 可选注入到 NexusPHP/Unit3D 等 HTML 框架的 adapter。
// §59.336: browser 可选注入到 NexusPHP adapter（详情页 fallback）。
type Factory struct {
	logger  *zap.Logger
	engine  *extract.Engine
	browser *browser.Fetcher
}

// NewFactory 创建 adapter 工厂。engine/browser 可为 nil
// （engine nil 退化为 legacy regexp；browser nil 不启用 fallback）。
func NewFactory(logger *zap.Logger, engine *extract.Engine) *Factory {
	return &Factory{logger: logger, engine: engine}
}

// SetBrowserFetcher 注入浏览器获取器（§59.336）。
// 容器无 Chromium 时保持 nil——fallback 优雅降级为纯 HTTP。
func (f *Factory) SetBrowserFetcher(b *browser.Fetcher) { f.browser = b }

func (f *Factory) Create(framework string, doer *HTTPDoer) model.SiteAdapter {
	switch framework {
	case "nexusphp":
		a := NewNexusPHPAdapter(doer, f.logger)
		if f.engine != nil {
			a.SetEngine(f.engine)
		}
		if f.browser != nil {
			a.SetBrowserFetcher(f.browser)
		}
		return a
	case "tnode":
		return NewTNodeAdapter(doer, f.logger)
	case "yemapt":
		return NewYemaptAdapter(doer, f.logger)
	case "mteam":
		return NewMTeamAdapter(doer, f.logger)
	case "unit3d":
		a := NewUnit3DAdapter(doer, f.logger)
		if f.engine != nil {
			a.SetEngine(f.engine)
		}
		return a
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
	"yemapt": {
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
