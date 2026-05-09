# syntax=docker/dockerfile:1.7

# ---- web build stage ----
FROM node:22-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---- go build stage ----
FROM golang:1.24-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) \
    go build -trimpath -ldflags="-s -w" -o /out/tap ./cmd/tap

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/tap /tap
USER nonroot:nonroot
VOLUME ["/data"]
EXPOSE 8080
ENV TAP_DATA_DIR=/data
ENTRYPOINT ["/tap", "-addr", "0.0.0.0:8080"]
