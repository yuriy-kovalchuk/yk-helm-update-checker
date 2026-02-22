# Stage 1: Build
FROM golang:1.26.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code explicitly
COPY Makefile ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application using the Makefile to ensure version injection
RUN make build

# Stage 2: Runtime
FROM alpine:3.23.3

# Install runtime dependencies (git is required for cloning, ca-certificates for HTTPS, openssh-client for SSH)
RUN apk add --no-cache git ca-certificates tzdata openssh-client

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/yk-update-checker /app/yk-update-checker

# Create a non-root user with a high UID for security compliance
RUN addgroup -g 10001 appgroup && \
    adduser -D -u 10001 -G appgroup appuser
USER appuser

# Expose the default port
EXPOSE 8080

# Default command
ENTRYPOINT ["/app/yk-update-checker"]
CMD ["-web"]
