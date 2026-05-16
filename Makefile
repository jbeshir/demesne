.PHONY: setup-tools
setup-tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install github.com/joho/godotenv/cmd/godotenv@latest

.PHONY: setup-files
setup-files: .env
.env:
	cp .env.dist .env

# ── Validation ────────────────────────────────────────────────

.PHONY: validate
validate: lint test-short build

.PHONY: lint
lint: sidecar-binary
	golangci-lint run --config .golangci.yml ./...

.PHONY: test-short
test-short: sidecar-binary
	go test -v -short ./...

.PHONY: test
test: sidecar-binary
	go test -v ./...

.PHONY: test-integration
test-integration: sidecar-binary .env
	godotenv -f .env go test -v -tags integration ./internal/sandbox/

.PHONY: fmt
fmt:
	go fmt ./...
	goimports -w .

# ── Build ─────────────────────────────────────────────────────

# sidecar-binary cross-compiles the linux/amd64 sidecar into
# internal/sidecar/dist/demesne-sidecar so go:embed picks it up
# before any other `go build` or `go test` runs.
.PHONY: sidecar-binary
sidecar-binary:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-trimpath -ldflags="-s -w" \
		-o internal/sidecar/dist/demesne-sidecar \
		./cmd/demesne-sidecar

.PHONY: build
build: sidecar-binary
	go build -o bin/demesne-mcp ./cmd/demesne-mcp

.PHONY: build-all-platforms
build-all-platforms: sidecar-binary
	GOOS=darwin GOARCH=amd64 go build -o bin/demesne-mcp-darwin-amd64 ./cmd/demesne-mcp
	GOOS=darwin GOARCH=arm64 go build -o bin/demesne-mcp-darwin-arm64 ./cmd/demesne-mcp
	GOOS=linux GOARCH=amd64 go build -o bin/demesne-mcp-linux-amd64 ./cmd/demesne-mcp
	GOOS=windows GOARCH=amd64 go build -o bin/demesne-mcp-windows-amd64.exe ./cmd/demesne-mcp

# ── MCPB Bundle ──────────────────────────────────────────────

# TODO(milestone-2): wire this up once we ship beyond local-dev.
.PHONY: mcpb
mcpb:
	@echo "mcpb packaging deferred to a later milestone"; exit 1

# ── Clean ─────────────────────────────────────────────────────

.PHONY: clean
clean:
	rm -rf bin/ internal/sidecar/dist/demesne-sidecar
