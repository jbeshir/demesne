# Render a React widget headlessly in a sandbox

When you want to render and screenshot a React widget inside a demesne sandbox, use the `browser` image — a demesne-built image that ships Node 22 + the Playwright JS API + headless Chromium/Firefox/WebKit + every required OS dependency so rendering works at `egress=none`. Useful for visual regression, widget snapshots, and agent-evaluated UI output.

## The image

The `browser` image is built locally by demesne the first time `image=browser` is used. The recipe layers the matching Playwright JS npm package (`playwright@1.60.0`) on top of `mcr.microsoft.com/playwright:v1.60.0-noble` (which ships the browser binaries + Node 22 but not the JS API) and sets `NODE_PATH=/usr/lib/node_modules` so `require('playwright')` resolves from any working directory, including the read-only `/in` mount. demesne builds and caches it once, the same way it builds its agent images, so only the first use is slow (it pulls the base). Rendering itself runs at `egress=none`.

`image=browser` is also available from **nested** sandboxes: a child `sandbox_script` (or `sandbox_create`) spawned from inside an agent can request it. Every sandbox is created on the host, so the host builds the image once and serves it to nested callers too — which is what lets an in-sandbox pipeline build a React app in a sibling sandbox and render it in a `browser` child (end-to-end React development).

## Rootless-podman Chromium flags

Four flags are required whenever Chromium runs under rootless podman:

| Flag | Why |
|------|-----|
| `--no-sandbox` | Chromium's inner process sandbox requires kernel namespaces and elevated capabilities unavailable to rootless containers. Without this flag, Chromium exits immediately. |
| `--disable-setuid-sandbox` | Belt-and-suspenders with `--no-sandbox`. Chromium's Linux setuid sandbox helper needs the setuid bit; rootless podman cannot grant that. This flag stops Chromium from trying the helper. |
| `--disable-dev-shm-usage` | Chromium uses `/dev/shm` for large shared-memory regions. Under rootless podman `/dev/shm` may be small or absent, causing render failures. This flag redirects shared memory to `/tmp`. |
| `--disable-gpu` | Headless Chromium still attempts to initialise the GPU process on Linux. No GPU is accessible inside a container, causing hangs or crashes. This flag forces software rendering. |

## Walkthrough

demesne ships a working fixture at `internal/sandbox/testdata/browser-fixture/`. Files:

- `index.html` — self-contained React 18.3.1 widget; loads local UMD bundles, mounts `<h1 id="widget">Hello from React</h1>` into `#root`
- `react.production.min.js` — React 18.3.1 UMD bundle
- `react-dom.production.min.js` — React-DOM 18.3.1 UMD bundle
- `render.cjs` — CommonJS Playwright harness using `require('playwright')`; asserts DOM text, writes `/out/screenshot.png` and `/out/render-ok`. CommonJS (not ESM) because `NODE_PATH=/usr/lib/node_modules` only redirects CommonJS resolution — ESM bare imports would fail to resolve `playwright` from the read-only `/in` mount.

To run it, call `sandbox_script` with the fixture directory mounted:

```
Use sandbox_script with:
  image:       browser
  egress:      none
  directories: ["<your absolute path to internal/sandbox/testdata/browser-fixture>"]
  command:     node /in/browser-fixture/render.cjs
```

demesne mounts each directory listed in `directories` read-only at `/in/<basename>` inside the sandbox. The harness writes `/out/screenshot.png` and `/out/render-ok` to the per-run output directory, whose host path is returned in the tool result as `output_dir`.

## Writing your own widget fixture

- **Self-contained HTML only.** No CDN URLs, no external stylesheets. The entire point of `egress=none` is that the page must load only from disk — any external `<script>` or `<link>` will hang or fail silently.
- Reference scripts relative to the HTML file: `<script src="./your-bundle.js"></script>`. The fixture directory is mounted flat at `/in/<basename>`, so relative paths resolve correctly.
- Put a stable text or DOM marker in the rendered output (`<h1 id="result">done</h1>`) so your harness can assert it deterministically — pixel counting is brittle across Chromium versions.
- Use `waitUntil: 'load'` (not `networkidle`) for `file://` pages. `networkidle` waits for network quiescence, which never fires cleanly for local files.

## Verifying the render

The tool result includes `output_dir` — the host path to the run's output directory. Read `screenshot.png` from there; confirming the file exists and is non-empty is usually sufficient as a smoke test.

For deeper verification, drive a DOM-text assertion inside the harness itself:

```js
const text = await page.textContent('#root');
if (!text.includes('expected text')) { process.exit(1); }
```

The fixture's `render.cjs` does both — read it as a starting template.

## See also

- [`sandbox_script` reference](../reference/tools/sandbox_script.md)
- [Configuration reference](../reference/configuration.md)
