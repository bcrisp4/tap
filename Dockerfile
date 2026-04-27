# syntax=docker/dockerfile:1.7

# ─── Stage 1: SvelteKit static build ────────────────────────────────
FROM node:22-alpine AS web

WORKDIR /web

# Cache npm deps separately from sources so dep installs reuse layers
# across iterations.
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY web/ ./
RUN npm run build

# ─── Stage 2: Go binary (pure Go, fully static) ─────────────────────
FROM golang:1-alpine AS gobuild

ARG VERSION=0.0.0-dev
WORKDIR /src

# Cache go module downloads in their own layer.
COPY go.mod go.sum ./
RUN go mod download

# Sources, including the staged SPA build that //go:embed picks up
# (paths are relative to the embed_with_spa.go source file, so the SPA
# must land at /src/internal/web/build).
COPY . .
COPY --from=web /web/build ./internal/web/build

ENV CGO_ENABLED=0
ENV GOFLAGS="-trimpath"

# `-tags embed_spa` flips internal/web onto the real //go:embed path.
# `-s -w` strips DWARF + symbol tables to shrink the binary.
RUN go build \
    -tags embed_spa \
    -ldflags "-s -w -X github.com/bcrisp4/tap/internal/version.Version=${VERSION}" \
    -o /out/tap ./cmd/tap

# ─── Stage 3: distroless runtime ────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=gobuild /out/tap /tap

# distroless/static:nonroot already runs as uid/gid 65532; restate it
# so the directive is visible to anyone reading the Dockerfile.
USER nonroot:nonroot
WORKDIR /data

ENV TAP_DB_PATH=/data/tap.db \
    TAP_LISTEN=0.0.0.0:8080 \
    TAP_PROXY_CACHE_DIR=/data/cache/ \
    TAP_LOG_FORMAT=json \
    TAP_LOG_LEVEL=info

VOLUME ["/data"]
EXPOSE 8080

# distroless/static has no shell, curl, or wget — the binary itself
# carries a `tap healthcheck` subcommand that probes /healthz over the
# loopback. Cheap (single GET, 5 s timeout) and zero extra surface.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/tap", "healthcheck"]

ENTRYPOINT ["/tap"]
CMD ["serve"]
