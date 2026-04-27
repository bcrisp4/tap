package httpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/httpclient"
)

func TestOptions_AppliesUACookieAuth(t *testing.T) {
	var seen http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
	}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{Timeout: 2 * time.Second, MaxBodyBytes: 1024, AllowPrivate: true})
	resp, err := c.Get(context.Background(), srv.URL, &httpclient.Options{
		UserAgent: "MyAgent/9.9",
		Cookie:    "session=abc",
		Username:  "u",
		Password:  "p",
	})
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "MyAgent/9.9", seen.Get("User-Agent"))
	require.Equal(t, "session=abc", seen.Get("Cookie"))
	require.NotEmpty(t, seen.Get("Authorization"))
}

func TestOptions_DefaultUAUsedWhenEmpty(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{
		Timeout:      2 * time.Second,
		MaxBodyBytes: 1024,
		AllowPrivate: true,
		UserAgent:    "Tap/test",
	})
	resp, err := c.Get(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "Tap/test", seen)
}
