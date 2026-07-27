#!/bin/sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RELEASE_VERSION=${VERSION:-0.1.0}
TARGET_ROOT="$PROJECT_ROOT/dist/desktop/windows-amd64"
PACKAGE_ROOT="$TARGET_ROOT/package"
ARCHIVE="$PROJECT_ROOT/dist/desktop/spare-desktop_${RELEASE_VERSION}_windows_amd64.zip"

rm -rf "$TARGET_ROOT"
mkdir -p "$PACKAGE_ROOT/bin" "$PACKAGE_ROOT/recipes"

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -tags "desktop production" \
  -ldflags="-s -w -H=windowsgui -X main.version=$RELEASE_VERSION" \
  -o "$PACKAGE_ROOT/Spare.exe" "$PROJECT_ROOT/cmd/spare-desktop"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -H=windowsgui -X main.version=$RELEASE_VERSION" \
  -o "$PACKAGE_ROOT/spared.exe" "$PROJECT_ROOT/cmd/spared"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$PACKAGE_ROOT/bin/spare.exe" "$PROJECT_ROOT/cmd/spare"

cp "$PROJECT_ROOT/desktop/build/windows/install.ps1" "$PACKAGE_ROOT/install.ps1"
cp "$PROJECT_ROOT/desktop/build/windows/uninstall.ps1" "$PACKAGE_ROOT/uninstall.ps1"
printf '%s\n' "$RELEASE_VERSION" > "$PACKAGE_ROOT/VERSION"
for recipe_id in site drop hook; do
  cp "$PROJECT_ROOT/dist/recipes/${recipe_id}_${RELEASE_VERSION}.sp" "$PACKAGE_ROOT/recipes/"
done

for executable in Spare.exe spared.exe bin/spare.exe; do
  if ! file "$PACKAGE_ROOT/$executable" | grep -q "PE32+"; then
    echo "$executable is not a 64-bit Windows executable." >&2
    exit 1
  fi
done
if ! objdump -p "$PACKAGE_ROOT/Spare.exe" | grep -q "Windows GUI"; then
  echo "Spare.exe is not a Windows GUI executable." >&2
  exit 1
fi
if ! objdump -p "$PACKAGE_ROOT/spared.exe" | grep -q "Windows GUI"; then
  echo "spared.exe is not a Windows GUI executable." >&2
  exit 1
fi

rm -f "$ARCHIVE"
(
  cd "$PACKAGE_ROOT"
  zip -qry "$ARCHIVE" Spare.exe spared.exe bin recipes install.ps1 uninstall.ps1 VERSION
)
(
  cd "$PROJECT_ROOT/dist/desktop"
  for desktop_archive in spare-desktop_*.zip spare-desktop_*.tar.gz; do
    [ -f "$desktop_archive" ] || continue
    shasum -a 256 "$desktop_archive"
  done | sort -k2 > checksums.txt
)

echo "Created $ARCHIVE"
echo "Created $PROJECT_ROOT/dist/desktop/checksums.txt"
