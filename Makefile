SHELL := /bin/sh

VERSION ?= 0.1.0
GO_LDFLAGS := -s -w -X main.version=$(VERSION)
DAEMON_LDFLAGS := $(GO_LDFLAGS)

ifeq ($(OS),Windows_NT)
DAEMON_LDFLAGS += -H=windowsgui
endif

.PHONY: dashboard build test test-ui smoke recipes release clean

dashboard:
	cd dashboard && npm ci && npm run build

build: dashboard
	mkdir -p bin
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o bin/spare ./cmd/spare
	go build -trimpath -ldflags="$(DAEMON_LDFLAGS)" -o bin/spared ./cmd/spared

test: dashboard
	go test ./...
	go vet ./...

test-ui:
	cd dashboard && npm run test:e2e

smoke: build
	./scripts/smoke.sh

recipes:
	mkdir -p dist/recipes
	go run ./cmd/spare recipe pack ./recipes/site --output dist/recipes/site_$(VERSION).sp
	go run ./cmd/spare recipe pack ./recipes/drop --output dist/recipes/drop_$(VERSION).sp
	go run ./cmd/spare recipe pack ./recipes/hook --output dist/recipes/hook_$(VERSION).sp

release: dashboard
	VERSION=$(VERSION) ./scripts/release.sh

clean:
	go clean
