SHELL := /bin/sh

VERSION ?= 0.1.0
GO_LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: dashboard build test test-ui smoke release clean

dashboard:
	cd dashboard && npm ci && npm run build

build: dashboard
	mkdir -p bin
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o bin/spare ./cmd/spare
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o bin/spared ./cmd/spared

test: dashboard
	go test ./...
	go vet ./...

test-ui:
	cd dashboard && npm run test:e2e

smoke: build
	./scripts/smoke.sh

release: dashboard
	VERSION=$(VERSION) ./scripts/release.sh

clean:
	go clean

