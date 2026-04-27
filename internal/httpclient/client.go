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
	rt, err := c.transportFor(req.URL, opts)
	if err != nil {
		return nil, err
	}
	applyHeaders(req, c.cfg.UserAgent, opts)

	cli := &http.Client{Transport: rt, Timeout: c.cfg.Timeout}
	resp, err := cli.Do(req)
	if err != nil {
		return resp, err
	}
	resp.Body = http.MaxBytesReader(nil, resp.Body, c.cfg.MaxBodyBytes)
	return resp, nil
}

func (c *Client) transportFor(u *url.URL, opts *Options) (http.RoundTripper, error) {
	switch {
	case opts.ProxyURL != "":
		return c.transportForProxy(opts.ProxyURL, opts)
	case c.cfg.Allowlist.MatchesHost(u.Hostname()):
		return newBrotliTransport(c.cloneTransport(c.unguarded, opts)), nil
	default:
		return newBrotliTransport(c.cloneTransport(c.protected, opts)), nil
	}
}

func (c *Client) cloneTransport(base *http.Transport, opts *Options) *http.Transport {
	if !opts.DisableHTTP2 && !opts.AllowSelfSigned {
		return base
	}
	t := base.Clone()
	if opts.DisableHTTP2 {
		t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		t.ForceAttemptHTTP2 = false
	}
	if opts.AllowSelfSigned {
		// base.Clone() above already deep-clones TLSClientConfig (or
		// leaves it nil), so it's safe to mutate t.TLSClientConfig
		// without affecting the shared base transport.
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{} //nolint:gosec
		}
		t.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec
	}
	return t
}

// transportForProxy returns (or builds) a brotli-wrapped transport
// keyed by proxy URL. Supports http://, https://, and socks5:// URIs.
func (c *Client) transportForProxy(proxyURL string, opts *Options) (http.RoundTripper, error) {
	if cached, ok := c.proxies.Load(proxyURL); ok {
		return cached.(http.RoundTripper), nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("httpclient: parse proxy_url: %w", err)
	}
	t := c.cloneTransport(c.protected, opts)
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
	actual, _ := c.proxies.LoadOrStore(proxyURL, wrapped)
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
