#!/usr/bin/env bash
# Builds the Windows release of Drifthold from any machine with Go installed
# (cross-compiling needs no C compiler) and packs it into a zip.
#
#   scripts/build-windows.sh [version]
#
# Output: dist/Drifthold.exe and dist/Drifthold-<version>-windows-x64.zip
set -euo pipefail
cd "$(dirname "$0")/.."

version="${1:-1.0.0}"
out=dist
mkdir -p "$out"

# Icon, manifest (DPI awareness, Windows 10/11) and version info.
go run ./tools/winres -out cmd/drifthold/rsrc_windows_amd64.syso -version "$version"

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
	-ldflags "-H=windowsgui -s -w" \
	-o "$out/Drifthold.exe" ./cmd/drifthold

go run ./tools/winres -check "$out/Drifthold.exe" -version "$version" >/dev/null

pkg="$out/Drifthold-$version-windows-x64"
rm -rf "$pkg" "$pkg.zip"
mkdir -p "$pkg"
cp "$out/Drifthold.exe" "$pkg/"
cp README.md "$pkg/README.txt"
cp THIRD_PARTY_LICENSES.md "$pkg/THIRD_PARTY_LICENSES.txt"
if command -v zip >/dev/null; then
	(cd "$out" && zip -qr "$(basename "$pkg").zip" "$(basename "$pkg")")
	echo "built $pkg.zip"
else
	echo "built $pkg/ (zip not installed; folder left unpacked)"
fi
