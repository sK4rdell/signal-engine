# ---- build -----------------------------------------------------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies first so they cache independently of source changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN rm -rf old

# Deterministic static binaries: no cgo, no build paths, no debug symbols.
# Build metadata (VCS revision) is embedded by the Go toolchain via
# runtime/debug when the source is a git checkout.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

# ---- runtime ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /out/worker /out/migrate /app/

USER nonroot:nonroot
WORKDIR /app

ENV HTTP_ADDR=:8080
EXPOSE 8080

# Same image runs every role:
#   api     -> /app/api
#   worker  -> /app/worker
#   migrate -> /app/migrate up
ENTRYPOINT ["/app/api"]
