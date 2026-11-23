LDFLAGS := -s -w \
  -X github.com/MC5lan/HonestBoy-CLI/internal/cli.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
  -X github.com/MC5lan/HonestBoy-CLI/internal/cli.Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none) \
  -X github.com/MC5lan/HonestBoy-CLI/internal/cli.Date=$(shell powershell -NoProfile -Command "(Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')")

.PHONY: build build-all test bench lint verify-deps clean release-check

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/hb.exe ./cmd/hb

build-all:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/hb-windows-amd64.exe ./cmd/hb
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/hb-windows-arm64.exe ./cmd/hb
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/hb-linux-amd64 ./cmd/hb

test:
	go test ./... -count=1

bench:
	go test ./internal/core -bench . -benchmem -run '^$$'

release-check:
	go build ./...
	go vet ./...
	go test ./... -count=1
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/checkdeps.ps1

lint:
	golangci-lint run ./...

verify-deps:
	bash scripts/checkdeps.sh

clean:
	go clean ./...
	Remove-Item -Recurse -Force bin,dist -ErrorAction SilentlyContinue