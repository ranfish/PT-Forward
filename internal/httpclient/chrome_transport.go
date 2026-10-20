package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	fhttp "github.com/Noooste/fhttp"
	fhttp2 "github.com/Noooste/fhttp/http2"
	utls "github.com/Noooste/utls"
	"math/rand"
)

// §59.334 附二: Chrome 指纹传输层——拆解 azuretls，分层组合
// （用户定案：智能拨号+Chrome 指纹理想叠加）。
//
// 替代 azuretlsTransport（azuretls 黑盒注入受限——ProxyDialer 私有/
// ModifyDialer 只能改属性），直接用 azuretls 底层组件构建：
//   dialContextSmart（智能拨号）→ TCP
//   utls.UClient（Chrome TLS 指纹）→ TLS
//   fhttp2（Chrome H2 指纹）→ HTTP/2
//   fhttp.Transport（有序头/伪头序）→ HTTP
//
// 每层独立可控——智能拨号与指纹伪装完全叠加。

// chromeTransport §59.334 附二: Chrome 指纹传输层
type chromeTransport struct {
	inner *fhttp.Transport
}

var (
	chromeTransports   = map[string]*fhttp.Transport{}
	chromeTransportsMu sync.Mutex
)

// getChromeTransport §59.334 附二: 站点级 Transport 复用
func getChromeTransport(domain string) *fhttp.Transport {
	chromeTransportsMu.Lock()
	defer chromeTransportsMu.Unlock()
	if t, ok := chromeTransports[domain]; ok {
		return t
	}

	baseDial := (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	t := &fhttp.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// ① 智能拨号（dialContextSmart：健康缓存+并发竞速）
			conn, err := dialContextSmart(baseDial)(ctx, network, addr)
			if err != nil {
				return nil, fmt.Errorf("smart dial: %w", err)
			}

			// ② TLS 升级（utls Chrome 指纹）
			hostname, _, splitErr := net.SplitHostPort(addr)
			if splitErr != nil {
				hostname = addr
			}
			return chromeHandshake(ctx, conn, hostname)
		},
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// ③ HTTP/2 指纹（fhttp2 Chrome 设置）
	applyChromeH2(t)

	chromeTransports[domain] = t
	return t
}

// chromeHandshake §59.334 附二: utls Chrome TLS 握手
func chromeHandshake(ctx context.Context, conn net.Conn, hostname string) (net.Conn, error) {
	config := &utls.Config{ServerName: hostname}
	tlsConn := utls.UClient(conn, config, utls.HelloCustom)

	spec := lastChromeSpec()
	tlsConn.ApplyPreset(&spec)

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("chrome TLS handshake: %w", err)
	}
	return tlsConn, nil
}

// lastChromeSpec §59.334 附二: Chrome 最新版 ClientHelloSpec
// （对齐 azuretls GetLastChromeVersion）
func lastChromeSpec() utls.ClientHelloSpec {
	return utls.ClientHelloSpec{
		CipherSuites: []uint16{
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
		},
		CompressionMethods: []byte{0},
		Extensions:         shuffledChromeExtensions(),
	}
}

// shuffledChromeExtensions §59.334 附二: Chrome 110+ 扩展乱序
func shuffledChromeExtensions() []utls.TLSExtension {
	exts := []utls.TLSExtension{
		&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
		&utls.SNIExtension{},
		&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519, utls.CurveP256, utls.CurveP384}},
		&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
			utls.ECDSAWithP256AndSHA256,
			utls.PSSWithSHA256,
			utls.PKCS1WithSHA256,
			utls.ECDSAWithP384AndSHA384,
			utls.PSSWithSHA384,
			utls.PKCS1WithSHA384,
			utls.PSSWithSHA512,
			utls.PKCS1WithSHA512,
		}},
		&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
		&utls.KeyShareExtension{KeyShares: []utls.KeyShare{
			{Group: utls.X25519},
		}},
		&utls.SupportedVersionsExtension{Versions: []uint16{
			utls.VersionTLS13, utls.VersionTLS12,
		}},
		&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
		&utls.ExtendedMasterSecretExtension{},
		&utls.SCTExtension{},
		&utls.StatusRequestExtension{},
		&utls.UtlsPaddingExtension{PaddingLen: 0},
		&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
		&utls.SessionTicketExtension{},
	}
	// Chrome 110+ shuffle
	rand.Shuffle(len(exts), func(i, j int) { exts[i], exts[j] = exts[j], exts[i] })
	return exts
}

// applyChromeH2 §59.334 附二: fhttp2 Chrome HTTP/2 指纹
func applyChromeH2(t *fhttp.Transport) {
	tr, err := fhttp2.ConfigureTransports(t)
	if err != nil {
		return
	}
	// Chrome SETTINGS（对齐 azuretls defaultHeaderSettings Chrome 分支）
	tr.Settings = map[fhttp2.SettingID]uint32{
		fhttp2.SettingHeaderTableSize:   65536,
		fhttp2.SettingEnablePush:        0,
		fhttp2.SettingInitialWindowSize: 6291456,
		fhttp2.SettingMaxHeaderListSize: 262144,
	}
	tr.SettingsOrder = []fhttp2.SettingID{
		fhttp2.SettingHeaderTableSize,
		fhttp2.SettingEnablePush,
		fhttp2.SettingInitialWindowSize,
		fhttp2.SettingMaxHeaderListSize,
	}
	tr.ConnectionFlow = 15663105
	tr.HeaderPriority = &fhttp2.PriorityParam{Weight: 254, StreamDep: 0, Exclusive: false}
	tr.StrictMaxConcurrentStreams = true
}

// RoundTrip 实现 http.RoundTripper——标准↔fhttp 双向转换
// （fhttp 是 net/http fork，字段兼容但类型不同）
func (t *chromeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fReq := &fhttp.Request{
		Method: req.Method,
		URL:    req.URL,
		Body:   req.Body,
		Header: make(fhttp.Header),
		Host:   req.Host,
	}
	for k, vs := range req.Header {
		fReq.Header[fhttp.CanonicalHeaderKey(k)] = vs
	}
	if fReq.Header.Get("Host") == "" && req.Host != "" {
		fReq.Header.Set("Host", req.Host)
	}

	fResp, err := t.inner.RoundTrip(fReq)
	if err != nil {
		return nil, err
	}

	// fhttp Response → 标准 Response
	resp := &http.Response{
		Status:        fResp.Status,
		StatusCode:    fResp.StatusCode,
		Proto:         fResp.Proto,
		ProtoMajor:    fResp.ProtoMajor,
		ProtoMinor:    fResp.ProtoMinor,
		Header:        make(http.Header),
		Body:          fResp.Body,
		ContentLength: fResp.ContentLength,
		Request:       req,
	}
	for k, vs := range fResp.Header {
		resp.Header[k] = vs
	}
	return resp, nil
}

func (t *chromeTransport) CloseIdleConnections() {
	t.inner.CloseIdleConnections()
}

