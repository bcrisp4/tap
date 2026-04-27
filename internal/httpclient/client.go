// Package httpclient provides Tap's shared outbound HTTP client.
package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	xproxy "golang.org/x/net/proxy"
)

// Config holds the global outbound client config (TAP_HTTP_*).
type Config struct {
	UserAgent    string
	Timeout      time.Duration
	MaxBodyBytes int64
	AllowPrivate bool
	Allowlist    AllowedHosts
}

// Client is Tap's shared HTTP client.
type Client struct {
	cfg       Config
	protected *http.Transport
	unguarded *http.Transport
	proxies   sync.Map // key: proxy URL string ; val: http.RoundTripper (already brotli-wrapped)
}

// NewClient constructs the shared client. Cheap; one per process.
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 20 * time.Second
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 10 * 1024 * 1024
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Tap/0.1 (+https://github.com/bcrisp4/tap)"
	}

	prot := newBaseTransport()
	prot.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 15 * time.Second,
		Control:   newSSRFControl(SSRFConfig{AllowPrivate: cfg.AllowPrivate, Allowlist: cfg.Allowlist}),
	}).DialContext

	un := newBaseTransport()
	un.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 15 * time.Second,
	}).DialContext

	return &Client{cfg: cfg, protected: prot, unguarded: un}
}

func newBaseTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        50,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
}

// Options holds per-request overrides. nil means "use defaults".
type Options struct {
	UserAgent       string
	Cookie          string
	Username        string
	Password        string
	ProxyURL        string
	DisableHTTP2    bool
	AllowSelfSigned bool
}

// Get is a convenience wrapper around Do.
func (c *Client) Get(ctx context.Context, rawURL string, opts *Options) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req, opts)
}

// Do dispatches a request through the appropriate transport, honoring
// per-feed overrides. The returned response's body is wrapped in
// MaxBytesReader and must be Closed by the caller.
func (c *Client) Do(req *http.Request, opts *Options) (*http.Response, error) {
	if opts == nil {
		opts = &Options{}
	}
	rt, viaUnguarded, err := c.transportFor(req.URL, opts)
	if err != nil {
		return nil, err
	}
	applyHeaders(req, c.cfg.UserAgent, opts)

	cli := &http.Client{
		Transport:     rt,
		Timeout:       c.cfg.Timeout,
		CheckRedirect: c.checkRedirect(viaUnguarded),
	}
	resp, err := cli.Do(req)
	if err != nil {
		return resp, err
	}
	resp.Body = http.MaxBytesReader(nil, resp.Body, c.cfg.MaxBodyBytes)
	return resp, nil
}

// transportFor picks the right transport for the request and reports
// whether it is the unguarded one — callers need that to install a
// matching redirect policy.
func (c *Client) transportFor(u *url.URL, opts *Options) (rt http.RoundTripper, viaUnguarded bool, err error) {
	switch {
	case opts.ProxyURL != "":
		rt, err = c.transportForProxy(opts.ProxyURL, opts)
		return rt, false, err
	case c.cfg.Allowlist.MatchesHost(u.Hostname()):
		return newBrotliTransport(c.cloneTransport(c.unguarded, opts)), true, nil
	default:
		return newBrotliTransport(c.cloneTransport(c.protected, opts)), false, nil
	}
}

// checkRedirect returns a CheckRedirect callback that reproduces the
// SSRF guarantees across redirect hops:
//   - For the protected transport, the SSRF dialer runs on every TCP
//     connect (including each redirect), so no extra check is needed.
//   - For the unguarded transport (used when the original hostname
//     matched the suffix allowlist), redirects to a non-allowlisted
//     host would silently skip the SSRF block. Reject those unless
//     AllowPrivate is set.
//
// Returns nil for the protected/proxy paths so net/http uses its
// default redirect policy.
func (c *Client) checkRedirect(viaUnguarded bool) func(*http.Request, []*http.Request) error {
	if !viaUnguarded || c.cfg.AllowPrivate {
		return nil
	}
	return func(req *http.Request, _ []*http.Request) error {
		if !c.cfg.Allowlist.MatchesHost(req.URL.Hostname()) {
			return fmt.Errorf("ssrf: redirect from allowlisted host to %q would bypass SSRF guard", req.URL.Hostname())
		}
		return nil
	}
}

// cloneTransport returns base unchanged when no per-request override
// touches the transport, otherwise returns a deep clone with overrides
// applied. Callers that *will* mutate the transport (e.g. to set Proxy
// or DialContext) must clone unconditionally — see transportForProxy.
func (c *Client) cloneTransport(base *http.Transport, opts *Options) *http.Transport {
	if !opts.DisableHTTP2 && !opts.AllowSelfSigned {
		return base
	}
	t := base.Clone()
	applyTLSAndHTTP2(t, opts)
	return t
}

// applyTLSAndHTTP2 mutates t in place to honour the per-request TLS /
// HTTP/2 toggles. Caller must own t (i.e. it must already be a clone).
func applyTLSAndHTTP2(t *http.Transport, opts *Options) {
	if opts.DisableHTTP2 {
		t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		t.ForceAttemptHTTP2 = false
	}
	if opts.AllowSelfSigned {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{} //nolint:gosec
		}
		t.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec
	}
}

// proxyCacheKey discriminates cached proxy transports by all per-feed
// fields that change the transport's behaviour, not just the proxy URL.
// Without this, a request that needs strict TLS could reuse a cached
// transport built earlier with InsecureSkipVerify=true (or HTTP/2 off).
type proxyCacheKey struct {
	URL             string
	DisableHTTP2    bool
	AllowSelfSigned bool
}

// transportForProxy returns (or builds) a brotli-wrapped transport
// keyed by (proxy URL, TLS / HTTP2 options). Supports http://,
// https://, and socks5:// URIs.
func (c *Client) transportForProxy(proxyURL string, opts *Options) (http.RoundTripper, error) {
	key := proxyCacheKey{URL: proxyURL, DisableHTTP2: opts.DisableHTTP2, AllowSelfSigned: opts.AllowSelfSigned}
	if cached, ok := c.proxies.Load(key); ok {
		return cached.(http.RoundTripper), nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("httpclient: parse proxy_url: %w", err)
	}
	// Always build a fresh clone — the switch below mutates Proxy /
	// DialContext, which would corrupt c.protected if cloneTransport
	// short-circuited and returned the shared base.
	t := c.protected.Clone()
	applyTLSAndHTTP2(t, opts)
	switch u.Scheme {
	case "http", "https":
		t.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		dialer, err := xproxy.FromURL(u, &net.Dialer{Timeout: 10 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("httpclient: build SOCKS5 dialer: %w", err)
		}
		ctxDialer, ok := dialer.(interface {
			DialContext(ctx context.Context, network, addr string) (net.Conn, error)
		})
		if !ok {
			return nil, errors.New("httpclient: SOCKS5 dialer lacks DialContext")
		}
		t.DialContext = ctxDialer.DialContext
	default:
		return nil, fmt.Errorf("httpclient: unsupported proxy scheme %q", u.Scheme)
	}
	wrapped := newBrotliTransport(t)
	actual, _ := c.proxies.LoadOrStore(key, wrapped)
	return actual.(http.RoundTripper), nil
}

func applyHeaders(req *http.Request, defaultUA string, o *Options) {
	ua := o.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	req.Header.Set("User-Agent", ua)
	if o.Cookie != "" {
		req.Header.Set("Cookie", o.Cookie)
	}
	if o.Username != "" || o.Password != "" {
		req.SetBasicAuth(o.Username, o.Password)
	}
}
