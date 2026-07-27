SHELL := /bin/sh

VERSION ?= 0.1.0
GO_LDFLAGS := -s -w -X main.version=$(VERSION)
DAEMON_LDFLAGS := $(GO_LDFLAGS)

ifeq ($(OS),Windows_NT)
DAEMON_LDFLAGS += -H=windowsgui
endif

.PHONY: dashboard build desktop desktop-package desktop-package-arm64 desktop-package-amd64 desktop-windows-package desktop-linux-package test test-ui smoke recipes release clean

dashboard:
	cd dashboard && npm ci && npm run build

build: dashboard
	mkdir -p bin
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o bin/spare ./cmd/spare
	go build -trimpath -ldflags="$(DAEMON_LDFLAGS)" -o bin/spared ./cmd/spared

desktop: dashboard
	mkdir -p bin
	go build -trimpath -tags "desktop production" -ldflags="$(GO_LDFLAGS)" -o bin/spare-desktop ./cmd/spare-desktop

desktop-package: dashboard recipes
	VERSION=$(VERSION) ./scripts/build-desktop.sh

desktop-package-arm64: dashboard recipes
	DESKTOP_ARCH=arm64 VERSION=$(VERSION) ./scripts/build-desktop.sh

desktop-package-amd64: dashboard recipes
	DESKTOP_ARCH=amd64 VERSION=$(VERSION) ./scripts/build-desktop.sh

desktop-windows-package: dashboard recipes
	VERSION=$(VERSION) ./scripts/build-desktop-windows.sh

desktop-linux-package: dashboard recipes
	VERSION=$(VERSION) ./scripts/build-desktop-linux.sh

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
