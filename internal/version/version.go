// Package version exposes the build-time version of the tap binary.
package version

// Version is overridable at link time via
// -ldflags "-X github.com/bcrisp4/tap/internal/version.Version=<v>".
var Version = "0.0.0-dev"

// String returns the current version string.
func String() string { return Version }
