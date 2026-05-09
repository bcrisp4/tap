package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Per-format minimal magic byte fixtures. http.DetectContentType only
// inspects the first 512 bytes; these prefixes are sufficient.
var fixtures = map[string][]byte{
	"image/png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG signature
	"image/jpeg": {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
	"image/gif":  []byte("GIF89a"),
	"image/webp": append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 4)...),
	"image/avif": append([]byte{0x00, 0x00, 0x00, 0x20}, []byte("ftypavif")...),
}

func TestValidateImage_AcceptsAllowlistedFormats(t *testing.T) {
	t.Parallel()
	for want, body := range fixtures {
		want, body := want, body
		t.Run(want, func(t *testing.T) {
			t.Parallel()
			got, ok := validateImage(body, want)
			require.True(t, ok, "fixture must validate")
			require.Equal(t, want, got, "sniffed type must match expected")
		})
	}
}

func TestValidateImage_RejectsTextHTML(t *testing.T) {
	t.Parallel()
	body := []byte("<!doctype html><html></html>")
	_, ok := validateImage(body, "text/html")
	require.False(t, ok)
}

func TestValidateImage_RejectsHeaderMismatch(t *testing.T) {
	t.Parallel()
	// Real PNG bytes but origin claims text/html — drop.
	_, ok := validateImage(fixtures["image/png"], "text/html")
	require.False(t, ok)
}

func TestValidateImage_AcceptsHeaderWithCharset(t *testing.T) {
	t.Parallel()
	// Origins sometimes append parameters — strip them before compare.
	got, ok := validateImage(fixtures["image/png"], "image/png; charset=binary")
	require.True(t, ok)
	require.Equal(t, "image/png", got)
}

func TestValidateImage_RejectsSVG(t *testing.T) {
	t.Parallel()
	body := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	_, ok := validateImage(body, "image/svg+xml")
	require.False(t, ok, "SVG must be rejected — not in M3 allowlist")
}
