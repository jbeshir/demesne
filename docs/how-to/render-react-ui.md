# Render a React widget headlessly in a sandbox

When you want to render and screenshot a React widget, ask your agent to do it in a demesne sandbox on the `browser` image — a demesne-built image with Node 22, the Playwright JS API, and headless Chromium/Firefox/WebKit, so rendering works with no network access. Useful for visual regression, widget snapshots, and agent-evaluated UI output.

## The image

demesne builds the `browser` image locally the first time it's used — layering the matching Playwright npm package onto Microsoft's official Playwright base (which ships the browser binaries and Node) — and caches it, so only the first run is slow. The cache is shared between the host and any nested sandboxes, so an agent running a larger pipeline can build a React app in one sandbox and render it in a `browser` child. Rendering needs no internet, so it runs with egress disabled.

## Rootless-podman Chromium flags

If your agent drives Chromium itself, four launch flags are required under rootless podman:

| Flag | Why |
|------|-----|
| `--no-sandbox` | Chromium's inner process sandbox requires kernel namespaces and elevated capabilities unavailable to rootless containers. Without this flag, Chromium exits immediately. |
| `--disable-setuid-sandbox` | Belt-and-suspenders with `--no-sandbox`. Chromium's Linux setuid sandbox helper needs the setuid bit; rootless podman cannot grant that. This flag stops Chromium from trying the helper. |
| `--disable-dev-shm-usage` | Chromium uses `/dev/shm` for large shared-memory regions. Under rootless podman `/dev/shm` may be small or absent, causing render failures. This flag redirects shared memory to `/tmp`. |
| `--disable-gpu` | Headless Chromium still attempts to initialise the GPU process on Linux. No GPU is accessible inside a container, causing hangs or crashes. This flag forces software rendering. |

## A working example

demesne ships a runnable fixture at `internal/sandbox/testdata/browser-fixture/` — a self-contained React 18 widget (loading local UMD bundles, no CDN) plus a Playwright harness, `render.cjs`, that loads it, asserts the rendered text, and writes a screenshot. Point your agent at it to see the whole flow work, or have it copy the harness as a starting template.

## Writing your own widget fixture

When you're rendering your own widget, a few things have to hold for it to work at `egress=none`:

- **Self-contained HTML only.** No CDN URLs, no external stylesheets — the page must load entirely from disk, or any external `<script>`/`<link>` will hang or fail silently. Reference your bundles relative to the HTML file (`<script src="./your-bundle.js"></script>`).
- **Put a stable text or DOM marker in the output** (`<h1 id="result">done</h1>`) so the render can be asserted deterministically — pixel counting is brittle across Chromium versions.
- **Use `waitUntil: 'load'`, not `networkidle`,** for local pages — `networkidle` waits for network quiescence that never fires cleanly for files on disk.
- **The harness must use CommonJS `require('playwright')`, not an ESM `import`** — the image exposes Playwright on `NODE_PATH`, which only redirects CommonJS resolution.

## Verifying the render

Your agent reads the screenshot back from the run's output directory; confirming it exists and is non-empty is usually enough as a smoke test. For deeper verification, have the harness assert the rendered DOM text itself — the fixture's `render.cjs` does both and reads well as a template.

## See also

- [`sandbox_script` reference](../reference/tools/sandbox_script.md)
- [Configuration reference](../reference/configuration.md)
