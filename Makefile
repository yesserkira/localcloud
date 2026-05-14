VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := localcloud
GOFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test vet check studio-install studio-build studio-typecheck clean

## Build the CLI binary
build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/localcloud

## Run Go tests
test:
	go test -race -count=1 ./...

## Run go vet
vet:
	go vet ./...

## Install Studio dependencies
studio-install:
	cd studio && npm ci

## Build Studio UI
studio-build:
	cd studio && npx vite build

## Type-check Studio UI
studio-typecheck:
	cd studio && npx tsc --noEmit

## Run all checks (Go + Studio)
check: vet test studio-typecheck studio-build

## Build release binary for the current platform (CGO required for go-sqlite3).
## For cross-platform builds, use CI with per-platform runners or goreleaser.
DIST := dist

release: check
	@echo "Building release $(VERSION) for $$(go env GOOS)/$$(go env GOARCH)"
	@mkdir -p $(DIST)
	CGO_ENABLED=1 go build $(GOFLAGS) -o $(DIST)/$(BINARY)-$$(go env GOOS)-$$(go env GOARCH)$$(go env GOEXE) ./cmd/localcloud
	@cd $(DIST) && sha256sum * > checksums.txt
	@echo "Artifacts in $(DIST)/"

## Remove build artifacts
clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(DIST) studio/dist
