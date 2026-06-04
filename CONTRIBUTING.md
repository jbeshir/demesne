# Contributing to demesne

## Building from source

Building from source requires Go 1.26+ (see `go.mod`).

`go install` route:

```
go install github.com/jbeshir/demesne/cmd/demesne-mcp@latest
```

Local build via `make`:

```
make build
DEMESNE_ALLOWED_PATHS=/tmp/demesne-test \
  OPEN_SANDBOX_DOMAIN=localhost:8080 \
  OPEN_SANDBOX_API_KEY=... \
  ./bin/demesne-mcp
```

The binary speaks JSON-RPC over stdio.

## Validation

```
make lint
make test-short
make build
```

Or run the full gate at once:

```
make validate
```

(which runs `tidy-check → lint → test-short → build`).

### Integration tests

Integration tests in `internal/sandbox/runner_integration_test.go` drive a real OpenSandbox end-to-end. They live behind the `integration` build tag, so the default test path doesn't touch them. To run them:

```
make setup-files     # one-off: copies .env.dist to .env
$EDITOR .env         # fill in OPEN_SANDBOX_API_KEY
make test-integration
```

`make setup-tools` installs the `godotenv` CLI that `test-integration` uses to load `.env`.

The integration suite covers: the `/out` mount round-trip; `egress: "none"` blocks both DNS and raw-IP egress; `egress: "package-managers"` allows pypi.org; the full persistent-sandbox lifecycle (create / exec / upload / exec / download / destroy); and that `sandbox_exec` refreshes the sandbox TTL. The raw-IP assertion requires the `[egress] mode = "dns+nft"` config in `~/.sandbox.toml` (see [Quickstart](docs/tutorial/quickstart.md#step-2-run-a-local-opensandbox)); against a `mode = "dns"` server it will fail.
