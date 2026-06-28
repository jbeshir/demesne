# The Playwright base tag is an ARG (consumed by FROM) so the whole base —
# Chromium/Firefox/WebKit + Node — is version-pinned and overridable; the
# default joins the embedded-Dockerfile content hash, so a bump forces a
# rebuild.
ARG PLAYWRIGHT_TAG=v1.60.0-noble
FROM mcr.microsoft.com/playwright:${PLAYWRIGHT_TAG}

# Playwright JS + Chromium/Firefox/WebKit + Node, identical to the browser
# image, so the offline interactive-fiction playtest harness gets Chromium
# and the `playwright` package via `require('playwright')` from any cwd
# (including the read-only /in mount). The npm package version tracks the base
# tag and is pinned via its own ARG for the same rebuild-on-bump reason.
ARG PLAYWRIGHT_VERSION=1.60.0
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
RUN npm install -g playwright@${PLAYWRIGHT_VERSION}
ENV NODE_PATH=/usr/lib/node_modules

# curl + unzip fetch and unpack the Tweego release archive; ca-certificates
# lets curl verify the GitHub TLS chain.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        unzip \
    && rm -rf /var/lib/apt/lists/*

# Tweego is the Twee/Twine compiler. Its prebuilt linux-x64 archive carries
# the `tweego` binary at the top level next to a `storyformats/` directory
# bundling the standard formats (Harlowe 1-3, SugarCube 1-2, Snowman,
# Chapbook, Paperthin). Pinned via ARG so the cache key tracks the version.
ARG TWEEGO_VERSION=2.1.1
RUN set -eux; \
    cd /tmp; \
    curl -fsSL -o tweego.zip \
        "https://github.com/tmedwards/tweego/releases/download/v${TWEEGO_VERSION}/tweego-${TWEEGO_VERSION}-linux-x64.zip"; \
    mkdir -p /opt/tweego; \
    unzip -q tweego.zip -d /opt/tweego; \
    rm tweego.zip; \
    install -m 0755 /opt/tweego/tweego /usr/local/bin/tweego

# TWEEGO_PATH points Tweego at the story formats from any cwd.
ENV TWEEGO_PATH=/opt/tweego/storyformats

# Tweego 2.1.1's bundled Harlowe (3.1.0) and SugarCube (2.30.0) are years stale.
# Overlay the current format.js for each — taken from the latest Twine 2 release
# that ships them — onto the matching major-version directory. Tweego keys
# formats by the directory name (`harlowe-3` / `sugarcube-2`), so replacing the
# file in place upgrades the format the author compiles against. Each version is
# an ARG so the cache key tracks a bump; the grep asserts the downloaded file is
# the expected version, failing the build on a bad URL or empty fetch.
ARG HARLOWE_VERSION=3.3.9
ARG SUGARCUBE_VERSION=2.37.3
ARG TWINEJS_TAG=2.12.0
RUN set -eux; \
    base="https://raw.githubusercontent.com/klembot/twinejs/refs/tags/${TWINEJS_TAG}/public/story-formats"; \
    curl -fsSL -o "${TWEEGO_PATH}/harlowe-3/format.js" \
        "${base}/harlowe-${HARLOWE_VERSION}/format.js"; \
    curl -fsSL -o "${TWEEGO_PATH}/sugarcube-2/format.js" \
        "${base}/sugarcube-${SUGARCUBE_VERSION}/format.js"; \
    grep -F "${HARLOWE_VERSION}" "${TWEEGO_PATH}/harlowe-3/format.js" > /dev/null; \
    grep -F "${SUGARCUBE_VERSION}" "${TWEEGO_PATH}/sugarcube-2/format.js" > /dev/null

# Prove the compiler runs and the pinned version is the one baked in.
# `tweego --version` prints its banner but exits non-zero (no compilation
# was requested), so assert on the banner via grep, which exits 0 on a match.
RUN tweego --version 2>&1 | grep -F "version ${TWEEGO_VERSION}"
