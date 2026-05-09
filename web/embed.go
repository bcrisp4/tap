// Package web exposes the built SPA bundle to the Go server as an embedded
// filesystem, so the Tap binary needs no external static assets at runtime.
//
// This file lives in web/ rather than internal/server/ because go:embed paths
// cannot escape the package directory.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
