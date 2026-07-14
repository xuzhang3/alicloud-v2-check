BINARY      := alicloud-v2-check
PKG         := github.com/xuzhang3/alicloud-v2-check
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# os/arch distribution matrix
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/386 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64 \
	windows/386

.PHONY: all build test vet fmt clean build-all $(PLATFORMS) snapshot

all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Cross-compile every platform into dist/
build-all: $(PLATFORMS)

$(PLATFORMS):
	@os=$(word 1,$(subst /, ,$@)); arch=$(word 2,$(subst /, ,$@)); \
	ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
	out=dist/$(BINARY)_$${os}_$${arch}$${ext}; \
	echo "==> $$out"; \
	CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
		go build -trimpath -ldflags '$(LDFLAGS)' -o $$out .

# Local GoReleaser dry-run (no publishing)
snapshot:
	goreleaser build --snapshot --clean

clean:
	rm -rf bin dist
