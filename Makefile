.PHONY: build test clean

BINARY_NAME=yk-update-checker
VERSION?=1.0.0
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS=-ldflags "-X yk-update-checker/internal/version.Version=$(VERSION) -X yk-update-checker/internal/version.Commit=$(COMMIT) -X yk-update-checker/internal/version.BuildDate=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/yk-update-checker

test:
	go test -v ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf test-repo
