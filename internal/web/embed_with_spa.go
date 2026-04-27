//go:build embed_spa

// This file is only compiled when -tags embed_spa is set. It owns the
// real //go:embed directive that pulls web/build/ into the binary.
//
// Without this build tag, buildFS (declared in embed.go) stays empty
// and Handler() falls back to the placeholder page. That's what makes
// `go build ./...` succeed against a fresh checkout that hasn't run
// `npm run build` yet.
package web

import "embed"

//go:embed all:build
var realFS embed.FS

func init() {
	buildFS = realFS
}
