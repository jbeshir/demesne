# Demesne host requirements

Everything that must be true of your host before demesne will run.

## Container runtime

Demesne embeds a `linux/amd64` helper binary that runs inside every sandbox container, so the host's container runtime must be able to execute `linux/amd64` containers. This is the standard path on:

- **linux/amd64** — native.
- **darwin/amd64** and **windows/amd64** — via the Docker/Podman Machine Linux VM.
- **darwin/arm64 (Apple Silicon)** — via Rosetta, which Docker Desktop enables by default and Podman supports via `podman machine init --rosetta`.

Releases are published for `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`. **Only `linux/amd64` is actively tested**; the other platforms build cleanly but are best-effort. linux/arm64 is reachable with `qemu-user-static` binfmt but no native binary is shipped.

## Build (Go 1.26+ for `go install`)

Building from source via `go install github.com/jbeshir/demesne/cmd/demesne-mcp@latest` requires Go 1.26 or later (see `go.mod`). Pre-built release binaries have no Go toolchain requirement.

## OpenSandbox configuration

Demesne talks to a long-running [OpenSandbox](https://github.com/alibaba/OpenSandbox) lifecycle server (typically via `uvx opensandbox-server --config ~/.sandbox.toml`). The packaged docker example defaults are too permissive for use as a security boundary. Change three settings in `~/.sandbox.toml` before starting the server:

- **`[egress] mode = "dns+nft"`** (default `"dns"`). The default only filters egress at DNS lookup; raw-IP outbound traffic still succeeds, so `egress: "none"` in `sandbox_script` does not actually deny network. The `dns+nft` mode adds nftables-based IP filtering and makes `none` mean none.
- **`[server] api_key = "<some-secret>"`** (default is empty). With an empty key, the server requires either an interactive `YES` at startup or `OPENSANDBOX_INSECURE_SERVER=YES` in the environment.
- **`[storage] allowed_host_paths = ["/tmp", "/home/<you>/code"]`** (or whichever directories you want bind-mountable). The example sets `[]` with a comment saying "all paths allowed", but empirically empty means *nothing* is allowed — every bind mount fails with `VOLUME::HOST_PATH_NOT_ALLOWED`.

### Allowed paths (both sides)

Both OpenSandbox's `allowed_host_paths` and demesne's `DEMESNE_ALLOWED_PATHS` must include each host path you intend to mount. When only one side is configured, bind mounts fail with `VOLUME::HOST_PATH_NOT_ALLOWED` from OpenSandbox.

## Rootless Podman pipe-page cap

If OpenSandbox runs on rootless podman (including podman serving the Docker-compatible API), set the per-user pipe-page soft cap to unlimited:

```bash
sudo sysctl -w fs.pipe-user-pages-soft=0                                     # now
echo 'fs.pipe-user-pages-soft = 0' | sudo tee /etc/sysctl.d/99-demesne.conf  # persist
```

Each sandbox's `pasta` network helper holds several 1 MiB pipes, so a fan-out of ~10+ concurrent sandboxes (routine for multi-agent pipelines) exceeds the default cap. Above it the kernel shrinks every new pipe to 8 KiB — too small for buildah's image-distribution copier, which then fails with `DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED: passing bulk input to subprocess`. `0` disables the soft cap; the hard cap is unlimited by default.
