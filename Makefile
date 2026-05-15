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
lint:
	golangci-lint run --config .golangci.yml ./...

.PHONY: test-short
test-short:
	go test -v -short ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: test-integration
test-integration: .env
	godotenv -f .env go test -v -tags integration ./internal/sandbox/

.PHONY: fmt
fmt:
	go fmt ./...
	goimports -w .

# ── Build ─────────────────────────────────────────────────────

.PHONY: build
build:
	go build -o bin/demesne-mcp ./cmd/demesne-mcp

.PHONY: build-all-platforms
build-all-platforms:
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
	rm -rf bin/
