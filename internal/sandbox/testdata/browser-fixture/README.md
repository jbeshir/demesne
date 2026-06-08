# browser-fixture

React-widget render fixture for the `browser` sandbox image (`mcr.microsoft.com/playwright:v1.60.0-noble`).

## Files

- `index.html` — self-contained HTML page that mounts a React 18.3.1 widget rendering "Hello from React" into `#root`
- `react.production.min.js` — React 18.3.1 UMD bundle (~10.5 KB)
- `react-dom.production.min.js` — React-DOM 18.3.1 UMD bundle (~129 KB)
- `render.mjs` — Playwright/Chromium harness that loads the page, asserts the widget text, and writes `/out/screenshot.png` + `/out/render-ok`

## Updating React bundles

```
npm pack react@18.3.1 react-dom@18.3.1
tar xf react-18.3.1.tgz && cp package/umd/react.production.min.js .
tar xf react-dom-18.3.1.tgz && cp package/umd/react-dom.production.min.js .
```

## Integration test usage

The test mounts this directory read-only at `/in/browser-fixture`, runs `node /in/browser-fixture/render.mjs`, and expects `/out/screenshot.png` and `/out/render-ok` to exist with non-zero size on success.
