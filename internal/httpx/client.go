package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// Opts configures the shared HTTP client. Zero values get defaults: 30s
// timeout, 4 PerHostInflight, no UserAgent header injection.
type Opts struct {
	Timeout         time.Duration
	PerHostInflight int
	SSRF            SSRFPolicy
	UserAgent       string
}

// NewClient builds the shared HTTP client. Composition top-down:
//   - http.Client (Timeout, CheckRedirect)
//   - hostLimiter RoundTripper (per-host concurrency cap)
//   - uaRoundTripper (User-Agent injection, only if Opts.UserAgent != "")
//   - http.Transport (connection pool + dialer)
//   - net.Dialer with ControlContext (SSRF check post-DNS)
//
// The dialer chooses between strict (with SSRF check) and permissive based
// on the URL hostname so suffix-allowlisted hosts can resolve to otherwise-
// rejected IPs (e.g. a Tailscale tailnet hostname → 100.x.y.z).
func NewClient(opts Opts) *http.Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.PerHostInflight <= 0 {
		opts.PerHostInflight = 4
	}

	permissive := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	strict := &net.Dialer{
		Timeout:        30 * time.Second,
		KeepAlive:      30 * time.Second,
		ControlContext: makeSSRFControl(opts.SSRF),
	}

	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if opts.SSRF.AllowHostname(host) {
			return permissive.DialContext(ctx, network, address)
		}
		return strict.DialContext(ctx, network, address)
	}

	transport := &http.Transport{
		DialContext:         dial,
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	var rt http.RoundTripper = transport
	if opts.UserAgent != "" {
		rt = &uaRoundTripper{ua: opts.UserAgent, inner: rt}
	}
	rt = newHostLimiter(rt, opts.PerHostInflight)

	return &http.Client{
		Timeout:       opts.Timeout,
		Transport:     rt,
		CheckRedirect: opts.SSRF.CheckRedirect,
	}
}

type uaRoundTripper struct {
	ua    string
	inner http.RoundTripper
}

func (u *uaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		cloned := req.Clone(req.Context())
		cloned.Header = req.Header.Clone()
		cloned.Header.Set("User-Agent", u.ua)
		return u.inner.RoundTrip(cloned)
	}
	return u.inner.RoundTrip(req)
}

func makeSSRFControl(policy SSRFPolicy) func(ctx context.Context, network, address string, c syscall.RawConn) error {
	return func(ctx context.Context, network, address string, c syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			return err
		}
		return policy.AllowAddr(addr)
	}
}
