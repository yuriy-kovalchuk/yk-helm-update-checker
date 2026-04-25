# syntax=docker/dockerfile:1

# ─── Build ────────────────────────────────────────────────────────────────────
# BUILDPLATFORM pins the builder to the host's native architecture so the Go
# toolchain runs at full speed. TARGETOS/TARGETARCH are injected by Buildx and
# tell the Go compiler which platform to cross-compile for.
FROM --platform=$BUILDPLATFORM golang:1.26.0-alpine AS builder

WORKDIR /app

# Download modules first; this layer is cached until go.mod or go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/      ./cmd/
COPY internal/ ./internal/

# Override at build time: docker build --build-arg VERSION=1.2.3 ...
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Buildx sets TARGETOS and TARGETARCH automatically for each platform variant.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
        -trimpath \
        -ldflags "-s -w \
            -X github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config.Version=${VERSION} \
            -X github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config.Commit=${COMMIT} \
            -X github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config.BuildDate=${BUILD_DATE}" \
        -o /yk-update-checker \
        ./cmd/yk-update-checker

# ─── Runtime ──────────────────────────────────────────────────────────────────
# Distroless static: no shell, no package manager, includes CA certificates
# and /tmp. Runs as non-root user 65532 by default.
# Buildx selects the correct arch variant of this image automatically.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="yk-update-checker" \
      org.opencontainers.image.description="Scan Helm and FluxCD repositories for chart updates" \
      org.opencontainers.image.source="https://github.com/yuriy-kovalchuk/yk-helm-update-checker" \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder /yk-update-checker /yk-update-checker

EXPOSE 8080

ENTRYPOINT ["/yk-update-checker"]
CMD ["-web"]
