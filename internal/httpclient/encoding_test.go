package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/require"
)

func TestBrotliRoundTripper_AcceptEncodingSet(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Accept-Encoding")
		_, _ = w.Write([]byte("hi"))
	}))
	defer srv.Close()

	c := &http.Client{Transport: newBrotliTransport(http.DefaultTransport)}
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	require.Contains(t, got, "br")
	require.Contains(t, got, "gzip")
}

func TestBrotliRoundTripper_DecodesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Accept-Encoding"), "br")
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		_, _ = bw.Write([]byte("the quick brown fox"))
		require.NoError(t, bw.Close())

		w.Header().Set("Content-Encoding", "br")
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	c := &http.Client{Transport: newBrotliTransport(http.DefaultTransport)}
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "the quick brown fox", string(body))
	require.Empty(t, resp.Header.Get("Content-Encoding"), "header should be stripped after decode")
}

func TestBrotliRoundTripper_PassesThroughIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("plain text"))
	}))
	defer srv.Close()

	c := &http.Client{Transport: newBrotliTransport(http.DefaultTransport)}
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "plain text", strings.TrimSpace(string(body)))
}
