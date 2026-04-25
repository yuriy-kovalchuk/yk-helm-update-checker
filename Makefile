.PHONY: build test clean

BINARY   := bin/yk-update-checker
VERSION  ?= dev
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE     := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
PKG      := github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config

LDFLAGS := -ldflags "-s -w \
  -X $(PKG).Version=$(VERSION) \
  -X $(PKG).Commit=$(COMMIT) \
  -X $(PKG).BuildDate=$(DATE)"

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BINARY) ./cmd/yk-update-checker

test:
	go test -v ./...

clean:
	rm -rf bin/
