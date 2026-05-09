package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
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
