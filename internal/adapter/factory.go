package adapter

import (
	"net/http"

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

func NewHTTPDoerWithSite(proxyURL string, skipSSLVerify bool) *HTTPDoer {
	return &HTTPDoer{
		Client: httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
			ProxyURL:      proxyURL,
			SkipSSLVerify: skipSSLVerify,
		}),
	}
}

// Factory 按框架名创建 adapter 实例。
// §56.13 方案 B: engine 可选注入到 NexusPHP/Unit3D 等 HTML 框架的 adapter。
type Factory struct {
	logger *zap.Logger
	engine *extract.Engine
}

// NewFactory 创建 adapter 工厂。engine 可为 nil（退化为 legacy regexp 提取）。
func NewFactory(logger *zap.Logger, engine *extract.Engine) *Factory {
	return &Factory{logger: logger, engine: engine}
}

// EngineInjector 接口：能接受 Engine 注入的 adapter 实现。
// NexusPHPAdapter / Unit3DAdapter 实现；其他 adapter（MTeam/Gazelle 等）不实现。
type EngineInjector interface {
	SetEngine(*extract.Engine)
}

func (f *Factory) Create(framework string, doer *HTTPDoer) model.SiteAdapter {
	switch framework {
	case "nexusphp":
		a := NewNexusPHPAdapter(doer, f.logger)
		if f.engine != nil {
			a.SetEngine(f.engine)
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
		// Unit3D 暂未实现 SetEngine，先不注入（后续改造）
		_ = f.engine
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
