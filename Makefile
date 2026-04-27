GO ?= go
BIN := tap
PKG := github.com/bcrisp4/tap

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -X $(PKG)/internal/version.Version=$(VERSION)

.PHONY: build test run tidy clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/tap

test:
	$(GO) test ./...

run: build
	./$(BIN)

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BIN)
