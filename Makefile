GO ?= go
BIN := tap
PKG := github.com/bcrisp4/tap

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -X $(PKG)/internal/version.Version=$(VERSION)

.PHONY: web-build web-stage build test test-all run tidy clean

# Build the SvelteKit SPA into web/build/.
web-build:
	cd web && npm ci && npm run build

# Stage the SPA build into the internal/web/ package so //go:embed can
# pick it up. //go:embed paths are relative to the source file's
# package, so the build output must live alongside embed_with_spa.go.
web-stage: web-build
	rm -rf internal/web/build
	cp -r web/build internal/web/build

build: web-stage
	CGO_ENABLED=0 $(GO) build -trimpath -tags embed_spa \
		-ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/tap

# Default test run — works against a fresh checkout, no SPA needed.
test:
	$(GO) test ./...

# Full test run — also exercises the embedded-SPA path.
test-all: web-stage
	$(GO) test -tags embed_spa ./...

run: build
	./$(BIN)

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BIN)
	rm -rf internal/web/build
