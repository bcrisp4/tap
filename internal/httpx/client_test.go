package httpx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_RejectsLoopbackByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := NewClient(Opts{
		Timeout: 5 * time.Second,
		SSRF:    SSRFPolicy{},
	})
	_, err := client.Get(srv.URL)
	if err == nil {
		t.Fatal("expected SSRF reject, got nil")
	}
	if !strings.Contains(err.Error(), "ssrf") {
		t.Errorf("expected ssrf error, got %v", err)
	}
}

func TestNewClient_AllowlistAcceptsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := NewClient(Opts{
		Timeout: 5 * time.Second,
		SSRF: SSRFPolicy{
			AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		},
	})
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q; want ok", body)
	}
}

func TestNewClient_DisabledAcceptsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	client := NewClient(Opts{
		SSRF: SSRFPolicy{Disabled: true},
	})
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
}

func TestNewClient_UserAgentApplied(t *testing.T) {
	var sawUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUA = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	client := NewClient(Opts{
		SSRF:      SSRFPolicy{Disabled: true},
		UserAgent: "tap-test/1.0",
	})
	_, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if sawUA != "tap-test/1.0" {
		t.Errorf("server saw UA = %q; want tap-test/1.0", sawUA)
	}
}

func TestNewClient_RedirectReChecks(t *testing.T) {
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer blocked.Close()

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.2:1/", http.StatusFound)
	}))
	defer srv1.Close()

	client := NewClient(Opts{
		SSRF: SSRFPolicy{
			AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
		},
	})
	_, err := client.Get(srv1.URL)
	if err == nil {
		t.Fatal("expected SSRF reject on redirect, got nil")
	}
}

// TestNewClient_RedirectCallbackFiresBeforeDial pins down that NewClient
// wires opts.SSRF.CheckRedirect onto the http.Client. With the redirect
// target at a literal private IP that is NOT in 127.x (the dialer-allowed
// range), the callback rejects with the wrapped ErrSSRFBlocked BEFORE any
// dial is attempted. If somebody removes CheckRedirect from NewClient, the
// dialer would still reject — but the error message would contain "dial tcp"
// and not be wrapped via ErrSSRFBlocked from the redirect callback path.
// Asserting both errors.Is(err, ErrSSRFBlocked) and "dial tcp" absence
// proves the callback fired first.
func TestNewClient_RedirectCallbackFiresBeforeDial(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://192.168.99.99:1/", http.StatusFound)
	}))
	defer srv1.Close()

	client := NewClient(Opts{
		SSRF: SSRFPolicy{
			AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		},
	})
	_, err := client.Get(srv1.URL)
	if err == nil {
		t.Fatal("expected SSRF reject on redirect, got nil")
	}
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected wrapped ErrSSRFBlocked, got %v", err)
	}
	if strings.Contains(err.Error(), "dial tcp 192.168.99.99") {
		t.Errorf("CheckRedirect should have rejected before any dial; got %v", err)
	}
}

// TestNewClient_UserAgentAppliedOverRedirect pins that the centrally-set
// User-Agent is injected on every hop, including after a redirect. If UA
// injection lived only on the initial request (e.g. via Client.Header rather
// than a RoundTripper), the second hop would lose it.
func TestNewClient_UserAgentAppliedOverRedirect(t *testing.T) {
	var (
		ua1 atomic.Value // string
		ua2 atomic.Value // string
	)

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua2.Store(r.Header.Get("User-Agent"))
	}))
	defer srv2.Close()

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua1.Store(r.Header.Get("User-Agent"))
		http.Redirect(w, r, srv2.URL, http.StatusFound)
	}))
	defer srv1.Close()

	client := NewClient(Opts{
		SSRF:      SSRFPolicy{Disabled: true},
		UserAgent: "tap-redirect-test/2.0",
	})
	resp, err := client.Get(srv1.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	got1, _ := ua1.Load().(string)
	got2, _ := ua2.Load().(string)
	if got1 != "tap-redirect-test/2.0" {
		t.Errorf("srv1 saw UA = %q; want tap-redirect-test/2.0", got1)
	}
	if got2 != "tap-redirect-test/2.0" {
		t.Errorf("srv2 (after redirect) saw UA = %q; want tap-redirect-test/2.0", got2)
	}
}
