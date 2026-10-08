#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIAO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
version="${MIAO_VERSION:-$(git -C "$MIAO_ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)}"
[[ "$version" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]] || { echo "Invalid MIAO_VERSION" >&2; exit 1; }
goos="${GOOS:-$(go env GOOS)}"
goarch="${GOARCH:-$(go env GOARCH)}"
case "$goos/$goarch" in linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;; *) echo "Unsupported platform: $goos/$goarch" >&2; exit 1 ;; esac
stage="$(mktemp -d "${TMPDIR:-/tmp}/miao-package.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
MIAO_VERSION="$version" GOOS="$goos" GOARCH="$goarch" bash "$SCRIPT_DIR/build.sh" "$stage/miao"
mkdir -p "$stage/scripts" "$MIAO_ROOT/dist"
for command_script in runtime install start stop status logs backup restore run-miao; do cp "$SCRIPT_DIR/$command_script.sh" "$stage/scripts/"; done
cp "$MIAO_ROOT/ecosystem.config.cjs" "$stage/"
cp "$MIAO_ROOT/package.json" "$stage/"
archive="miao_${version}_${goos}_${goarch}.tar.gz"
tar -czf "$MIAO_ROOT/dist/$archive" -C "$stage" miao scripts ecosystem.config.cjs package.json
(cd "$MIAO_ROOT/dist"; if command -v sha256sum >/dev/null; then sha256sum "$archive" > "$archive.sha256"; else shasum -a 256 "$archive" > "$archive.sha256"; fi)
echo "Packaged: $MIAO_ROOT/dist/$archive"
