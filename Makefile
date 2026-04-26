.PHONY: build run lint fmt vet tidy test test-cover docker-build docker-push clean help

BINARY     := bin/yk-update-checker
IMAGE      ?= ghcr.io/yuriy-kovalchuk/yk-helm-update-checker
VERSION    ?= dev
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE       := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
PKG        := github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config
PLATFORMS  ?= linux/amd64,linux/arm64

LDFLAGS := -ldflags "-s -w \
  -X $(PKG).Version=$(VERSION) \
  -X $(PKG).Commit=$(COMMIT) \
  -X $(PKG).BuildDate=$(DATE)"

## build: compile the binary for the current platform
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BINARY) ./cmd/yk-update-checker

## run: build and run the web server using config.yaml
run: build
	$(BINARY) -web -config config.yaml

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	gofmt -w -s .

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## test: run all tests
test:
	go test -race -timeout 120s ./...

## test-cover: run tests with coverage report
test-cover:
	go test -race -timeout 120s -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## docker-build: build multi-arch Docker image (requires buildx)
docker-build:
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(DATE) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

## docker-push: build and push multi-arch Docker image
docker-push:
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(DATE) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		--push \
		.

## clean: remove build artefacts and coverage reports
clean:
	rm -rf bin/ coverage.out coverage.html

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## //' | column -t -s ':'
