#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi

TAG="${1:-}"
if [[ -z "$TAG" ]]; then
  echo "Usage: scripts/release.sh [--dry-run] TAG" >&2
  exit 1
fi

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "Error: TAG must match ^v[0-9]+\.[0-9]+\.[0-9]+ (e.g. v0.2.0)" >&2
  exit 1
fi

VERSION="${TAG#v}"

# Clean dist/ for predictability
rm -rf dist
mkdir -p dist

# Build all five platform binaries
make build-all-platforms

# Stage each platform and create archives
ARCHIVES=()

build_archive() {
  local os="$1" arch="$2" src="$3" dst="$4" archive_type="$5"
  local platform="${os}_${arch}"
  local stage="dist/.stage/${platform}"
  mkdir -p "$stage"
  cp "$src" "$stage/$dst"
  cp LICENSE README.md SECURITY.md "$stage/"
  local archive_base="demesne-mcp_${VERSION}_${platform}"
  if [[ "$archive_type" == "zip" ]]; then
    (cd "dist/.stage/${platform}" && zip -r "../../${archive_base}.zip" .)
    ARCHIVES+=("dist/${archive_base}.zip")
  else
    tar -C "dist/.stage/${platform}" -czf "dist/${archive_base}.tar.gz" .
    ARCHIVES+=("dist/${archive_base}.tar.gz")
  fi
}

# The sandbox-side helper binary embedded by `go:embed` is linux/amd64, so the
# host's container runtime must run linux/amd64 containers. We ship native
# host binaries for the OS/arch combos where that runtime path is standard:
# linux/amd64 natively, darwin/amd64 + windows/amd64 via the Docker/Podman
# Machine VM, darwin/arm64 via Rosetta. linux/arm64 needs `qemu-user-static`
# binfmt and is not shipped. See README §Requirements.
build_archive linux   amd64 bin/demesne-mcp-linux-amd64        demesne-mcp     tar
build_archive darwin  amd64 bin/demesne-mcp-darwin-amd64       demesne-mcp     tar
build_archive darwin  arm64 bin/demesne-mcp-darwin-arm64       demesne-mcp     tar
build_archive windows amd64 bin/demesne-mcp-windows-amd64.exe  demesne-mcp.exe zip

# Clean up staging dirs
rm -rf dist/.stage

# Determine checksum command
if command -v sha256sum &>/dev/null; then
  sha_digest() { sha256sum "$@"; }
elif command -v shasum &>/dev/null; then
  sha_digest() { shasum -a 256 "$@"; }
else
  echo "Error: neither sha256sum nor shasum is available" >&2
  exit 1
fi

# Generate checksums (sha256sum-compatible: "<hash>  <basename>")
CHECKSUM_FILE="dist/demesne-mcp_${VERSION}_checksums.txt"
: > "$CHECKSUM_FILE"
for archive in "${ARCHIVES[@]}"; do
  name="$(basename "$archive")"
  hash=$(sha_digest "$archive" | awk '{print $1}')
  printf '%s  %s\n' "$hash" "$name" >> "$CHECKSUM_FILE"
done

# Generate release notes
NOTES_FILE="dist/release-notes.md"
NOTES_BODY=""

if [[ -f "CHANGELOG.md" ]]; then
  NOTES_BODY=$(awk -v ver="$VERSION" '
    $0 ~ "^## \\[?" ver "\\]?" { found=1; next }
    found && /^## / { exit }
    found { print }
  ' CHANGELOG.md | sed '/^[[:space:]]*$/d')
fi

if [[ -z "$NOTES_BODY" ]]; then
  NOTES_BODY=$(git tag -l --format='%(contents)' "$TAG" 2>/dev/null || true)
  NOTES_BODY=$(printf '%s' "$NOTES_BODY" | sed '/^[[:space:]]*$/d')
fi

if [[ -z "$NOTES_BODY" ]]; then
  NOTES_BODY="Release $TAG"
fi

{
  printf '%s\n' "$NOTES_BODY"
  printf '\n## Caveats\n\n'
  printf -- '- This is a pre-1.0 release; APIs and the tool surface may change.\n'
} > "$NOTES_FILE"

if [[ "$DRY_RUN" == 1 ]]; then
  echo "=== Dry run: planned command ==="
  printf 'gh release create "%s" --title "Release %s" --notes-file dist/release-notes.md \\\n' "$TAG" "$TAG"
  printf '  dist/demesne-mcp_%s_*.tar.gz \\\n' "$VERSION"
  printf '  dist/demesne-mcp_%s_*.zip \\\n' "$VERSION"
  printf '  dist/demesne-mcp_%s_checksums.txt\n' "$VERSION"
  echo ""
  echo "=== Staged files ==="
  for archive in "${ARCHIVES[@]}"; do
    echo "  $archive"
  done
  echo "  $CHECKSUM_FILE"
  exit 0
fi

if [[ -z "${GH_TOKEN:-}" ]]; then
  echo "Error: GH_TOKEN must be set to publish a release" >&2
  exit 1
fi

gh release create "$TAG" \
  --title "Release $TAG" \
  --notes-file dist/release-notes.md \
  dist/demesne-mcp_${VERSION}_*.tar.gz \
  dist/demesne-mcp_${VERSION}_*.zip \
  dist/demesne-mcp_${VERSION}_checksums.txt
