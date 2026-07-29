#!/bin/sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RELEASE_VERSION=${VERSION:-0.1.0}

if [ "$(uname -s)" != "Darwin" ]; then
  echo "The macOS desktop package must be built on macOS." >&2
  exit 1
fi

case "${DESKTOP_ARCH:-$(uname -m)}" in
  arm64|aarch64)
    TARGET_ARCH=arm64
    LIPO_ARCH=arm64
    CLANG_ARCH=arm64
    ;;
  amd64|x86_64)
    TARGET_ARCH=amd64
    LIPO_ARCH=x86_64
    CLANG_ARCH=x86_64
    ;;
  *)
    echo "Unsupported macOS desktop architecture: ${DESKTOP_ARCH:-$(uname -m)}" >&2
    exit 1
    ;;
esac

export GOOS=darwin
export GOARCH=$TARGET_ARCH
export CGO_ENABLED=1
case "$(uname -m):$TARGET_ARCH" in
  arm64:arm64|x86_64:amd64)
    ;;
  *)
    export CC="clang -arch $CLANG_ARCH"
    ;;
esac

APP_ROOT="$PROJECT_ROOT/dist/desktop/Spare.app"
CONTENTS="$APP_ROOT/Contents"
MACOS_DIR="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
BIN_RESOURCES="$RESOURCES/bin"
PACKAGE_ROOT="$PROJECT_ROOT/dist/desktop/package"
ARCHIVE="$PROJECT_ROOT/dist/desktop/spare-desktop_${RELEASE_VERSION}_darwin_${TARGET_ARCH}.zip"
ICONSET="$PROJECT_ROOT/dist/desktop/Spare.iconset"

rm -rf "$APP_ROOT" "$PACKAGE_ROOT" "$ICONSET"
rm -f "$ARCHIVE"
mkdir -p "$MACOS_DIR" "$RESOURCES/recipes" "$BIN_RESOURCES" "$PACKAGE_ROOT" "$ICONSET"

go build -trimpath -tags "desktop production" \
  -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$MACOS_DIR/Spare" "$PROJECT_ROOT/cmd/spare-desktop"
go build -trimpath -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$MACOS_DIR/spared" "$PROJECT_ROOT/cmd/spared"
go build -trimpath -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$BIN_RESOURCES/spare" "$PROJECT_ROOT/cmd/spare"

for executable in "$MACOS_DIR/Spare" "$MACOS_DIR/spared" "$BIN_RESOURCES/spare"; do
  if [ "$(lipo -archs "$executable")" != "$LIPO_ARCH" ]; then
    echo "$executable did not produce the requested $TARGET_ARCH executable." >&2
    exit 1
  fi
done

sed "s/__VERSION__/$RELEASE_VERSION/g" \
  "$PROJECT_ROOT/desktop/build/darwin/Info.plist" > "$CONTENTS/Info.plist"
cp "$PROJECT_ROOT/desktop/icons/app-icon.svg" "$RESOURCES/app-icon.svg"
ICON_PREVIEW_DIR=$(mktemp -d)
trap 'rm -rf "$ICON_PREVIEW_DIR"' EXIT INT TERM
ICON_SOURCE="$ICON_PREVIEW_DIR/app-icon.png"
(
  cd "$PROJECT_ROOT/dashboard"
  node scripts/render-svg-icon.mjs \
    "$PROJECT_ROOT/desktop/icons/app-icon.svg" \
    "$ICON_SOURCE" \
    1024
)
if [ ! -f "$ICON_SOURCE" ]; then
  echo "Unable to render the Spare application icon." >&2
  exit 1
fi
for icon_size in 16 32 128 256 512; do
  sips -s format png -z "$icon_size" "$icon_size" \
    "$ICON_SOURCE" \
    --out "$ICONSET/icon_${icon_size}x${icon_size}.png" >/dev/null
  retina_size=$((icon_size * 2))
  sips -s format png -z "$retina_size" "$retina_size" \
    "$ICON_SOURCE" \
    --out "$ICONSET/icon_${icon_size}x${icon_size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$RESOURCES/Spare.icns"
rm -rf "$ICONSET"
rm -rf "$ICON_PREVIEW_DIR"
trap - EXIT INT TERM
cp "$PROJECT_ROOT/desktop/build/darwin/uninstall.sh" "$RESOURCES/uninstall.sh"
chmod 755 "$RESOURCES/uninstall.sh"

for recipe_id in site drop hook; do
  cp "$PROJECT_ROOT/dist/recipes/${recipe_id}_${RELEASE_VERSION}.sp" "$RESOURCES/recipes/"
done

codesign --force --sign - --timestamp=none "$MACOS_DIR/spared"
codesign --force --sign - --timestamp=none "$BIN_RESOURCES/spare"
codesign --force --sign - --timestamp=none "$MACOS_DIR/Spare"
codesign --force --sign - --timestamp=none "$APP_ROOT"
codesign --verify --deep --strict "$APP_ROOT"

ditto "$APP_ROOT" "$PACKAGE_ROOT/Spare.app"
cp "$PROJECT_ROOT/desktop/build/darwin/install.sh" "$PACKAGE_ROOT/install.sh"
chmod 755 "$PACKAGE_ROOT/install.sh"
(
  cd "$PACKAGE_ROOT"
  zip -qry "$ARCHIVE" Spare.app install.sh
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
