BINARY_NAME=dd-license-attribution
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X github.com/DataDog/dd-license-attribution/cmd.Version=$(VERSION)"

.PHONY: build test lint clean install cross-compile release snapshot verify

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

install:
	go install $(LDFLAGS) .

test:
	go test ./internal/... -v -count=1

test-cover:
	go test ./internal/... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html

test-contract:
	go test -tags contract ./internal/adaptor/... -v -count=1

lint:
	go vet ./...

clean:
	rm -rf bin/ dist/ coverage.out coverage.html

cross-compile:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .

# GoReleaser targets
release:
	goreleaser release --clean

snapshot:
	goreleaser release --snapshot --clean

# Verification: compare Go and Python outputs
# Usage: make verify REPO=https://github.com/DataDog/dd-license-attribution TOKEN=ghp_...
verify:
	@./scripts/verify.sh "$(REPO)" "$(TOKEN)"
