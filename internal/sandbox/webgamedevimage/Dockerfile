# syntax=docker/dockerfile:1
# The syntax directive selects the BuildKit Dockerfile frontend, which this
# image needs for the `RUN cat > file <<EOF` heredocs that author the game
# template inline (podman/buildah parse heredocs natively and ignore it).

# The Playwright base tag is an ARG (consumed by FROM) so the whole base —
# Chromium/Firefox/WebKit + Node — is version-pinned and overridable; the
# default joins the embedded-Dockerfile content hash, so a bump forces a
# rebuild.
ARG PLAYWRIGHT_TAG=v1.60.0-noble
FROM mcr.microsoft.com/playwright:${PLAYWRIGHT_TAG}

# Playwright JS + Chromium/Firefox/WebKit + Node, identical to the browser
# image, so the offline HTML5-game playtest harness gets Chromium and the
# `playwright` package via `require('playwright')` from any cwd (including
# the read-only /in mount). The npm package version tracks the base tag and
# is pinned via its own ARG for the same rebuild-on-bump reason.
ARG PLAYWRIGHT_VERSION=1.60.0
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
RUN npm install -g playwright@${PLAYWRIGHT_VERSION}
ENV NODE_PATH=/usr/lib/node_modules

# A minimal but real Phaser + Vite + TypeScript project, baked at
# /opt/game-template so an agent can copy it, drop in a game, and build +
# playtest it entirely offline. Versions are ARGs so the cache key tracks
# them; the only deps are phaser/vite/typescript to keep the image lean.
ARG PHASER_VERSION=4.2.0
ARG VITE_VERSION=8.1.0
ARG TYPESCRIPT_VERSION=6.0.3

RUN mkdir -p /opt/game-template/src

# package.json — version specifiers are interpolated from the ARGs above
# (unquoted heredoc), so the baked template pins exactly what the image
# advertises.
RUN cat > /opt/game-template/package.json <<JSON
{
  "name": "demesne-game-template",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "phaser": "^${PHASER_VERSION}"
  },
  "devDependencies": {
    "typescript": "^${TYPESCRIPT_VERSION}",
    "vite": "^${VITE_VERSION}"
  }
}
JSON

# vite.config.ts — base: './' emits relative asset paths so the built game
# opens from a file:// URL or any subpath during offline playtesting.
RUN cat > /opt/game-template/vite.config.ts <<'TS'
import { defineConfig } from 'vite';

// Relative base so the production bundle is openable from file:// or any
// subpath — the offline playtest harness loads it without a dev server.
export default defineConfig({
  base: './',
});
TS

# tsconfig.json — bundler module resolution, suitable for Vite + TS.
RUN cat > /opt/game-template/tsconfig.json <<'JSON'
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "moduleDetection": "force",
    "isolatedModules": true,
    "strict": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "noEmit": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "types": []
  },
  "include": ["src", "vite.config.ts"]
}
JSON

# index.html — Vite's entry point; <html data-game-state> starts at "boot"
# and the game updates it as it progresses.
RUN cat > /opt/game-template/index.html <<'HTML'
<!doctype html>
<html lang="en" data-game-state="boot">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>demesne webgamedev template</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
HTML

# src/main.ts — a one-scene Phaser game. After the first scene's create()
# runs it flips <html data-game-state> to "playing" and appends an element
# with id="game-ready", giving the offline playtest harness two DOM signals
# (a coarse state machine and a first-frame marker) it can poll via
# Playwright without inspecting pixels.
RUN cat > /opt/game-template/src/main.ts <<'TS'
import Phaser from 'phaser';

// Coarse game state mirrored onto <html data-game-state> so a Playwright
// playtest can observe progress without reading the canvas.
type GameState = 'boot' | 'playing' | 'won' | 'lost';

function setGameState(state: GameState): void {
  document.documentElement.dataset.gameState = state;
}

setGameState('boot');

class MainScene extends Phaser.Scene {
  constructor() {
    super('main');
  }

  create(): void {
    this.add.text(20, 20, 'demesne webgamedev template', {
      color: '#ffffff',
      fontSize: '24px',
    });

    setGameState('playing');

    // First-frame readiness marker for the playtest harness.
    const ready = document.createElement('div');
    ready.id = 'game-ready';
    ready.style.display = 'none';
    document.body.appendChild(ready);
  }
}

new Phaser.Game({
  type: Phaser.AUTO,
  width: 800,
  height: 600,
  parent: 'app',
  backgroundColor: '#1d1d1d',
  scene: [MainScene],
});
TS

# Warm node_modules and prove the template builds. npm install (no lockfile)
# resolves the pinned deps; npm run build runs `vite build`, warming Vite's
# cache and producing dist/ so the first offline build is fast.
RUN cd /opt/game-template \
    && npm install \
    && npm run build
