.PHONY: dev build run docker clean test

GO   ?= go
PNPM ?= pnpm
BIN  := bin/tap
ADDR ?= 127.0.0.1:8080
DATA ?= ./data

dev:
	@echo "Starting Go on :8080 and Vite on :5173 — open http://localhost:5173"
	@$(GO) run ./cmd/tap & \
	 GO_PID=$$!; \
	 trap "kill $$GO_PID 2>/dev/null" EXIT; \
	 $(PNPM) --dir web dev

# CGO_ENABLED=0 keeps the binary truly static so it runs on a distroless image
# without glibc — same flag the Dockerfile sets, kept here so `make build`
# and `make docker` produce equivalent artefacts.
build: web/dist/index.html
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/tap

web/dist/index.html: $(shell find web/src -type f) web/index.html web/package.json
	$(PNPM) --dir web install --frozen-lockfile
	$(PNPM) --dir web build

run: build
	mkdir -p $(DATA)
	$(BIN) -addr $(ADDR) -data $(DATA)

docker: build
	docker build -t tap:dev .

test: web/dist/index.html
	$(GO) test ./... -race

clean:
	rm -rf bin/ web/dist/
