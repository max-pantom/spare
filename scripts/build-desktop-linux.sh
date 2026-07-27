#!/bin/sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RELEASE_VERSION=${VERSION:-0.1.0}

if [ "$(uname -s)" != "Linux" ]; then
  echo "Linux Desktop packages must be built natively on Linux." >&2
  exit 1
fi
case "$(uname -m)" in
  x86_64) TARGET_ARCH=amd64 ;;
  aarch64|arm64) TARGET_ARCH=arm64 ;;
  *)
    echo "Linux Desktop supports only amd64 and arm64." >&2
    exit 1
    ;;
esac

if ! pkg-config --exists gtk+-3.0 ayatana-appindicator3-0.1; then
  echo "Install libgtk-3-dev and libayatana-appindicator3-dev before building Spare Desktop." >&2
  exit 1
fi
WEBKIT_TAGS="desktop production"
if pkg-config --exists webkit2gtk-4.1; then
  WEBKIT_TAGS="$WEBKIT_TAGS webkit2_41"
elif ! pkg-config --exists webkit2gtk-4.0; then
  echo "Install libwebkit2gtk-4.1-dev or libwebkit2gtk-4.0-dev before building Spare Desktop." >&2
  exit 1
fi

TARGET_ROOT="$PROJECT_ROOT/dist/desktop/linux-$TARGET_ARCH"
PACKAGE_ROOT="$TARGET_ROOT/package"
ARCHIVE="$PROJECT_ROOT/dist/desktop/spare-desktop_${RELEASE_VERSION}_linux_${TARGET_ARCH}.tar.gz"
rm -rf "$TARGET_ROOT"
mkdir -p "$PACKAGE_ROOT/bin" "$PACKAGE_ROOT/recipes"

CGO_ENABLED=1 GOARCH="$TARGET_ARCH" go build -trimpath \
  -tags "$WEBKIT_TAGS" \
  -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$PACKAGE_ROOT/Spare" "$PROJECT_ROOT/cmd/spare-desktop"
CGO_ENABLED=1 GOARCH="$TARGET_ARCH" go build -trimpath \
  -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$PACKAGE_ROOT/spared" "$PROJECT_ROOT/cmd/spared"
CGO_ENABLED=1 GOARCH="$TARGET_ARCH" go build -trimpath \
  -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
  -o "$PACKAGE_ROOT/bin/spare" "$PROJECT_ROOT/cmd/spare"

cp "$PROJECT_ROOT/desktop/build/linux/install.sh" "$PACKAGE_ROOT/install.sh"
cp "$PROJECT_ROOT/desktop/build/linux/uninstall.sh" "$PACKAGE_ROOT/uninstall.sh"
cp "$PROJECT_ROOT/desktop/build/linux/spare-mime.xml" "$PACKAGE_ROOT/spare-mime.xml"
chmod 755 "$PACKAGE_ROOT/install.sh" "$PACKAGE_ROOT/uninstall.sh"
printf '%s\n' "$RELEASE_VERSION" > "$PACKAGE_ROOT/VERSION"
for recipe_id in site drop hook; do
  cp "$PROJECT_ROOT/dist/recipes/${recipe_id}_${RELEASE_VERSION}.sp" "$PACKAGE_ROOT/recipes/"
done

rm -f "$ARCHIVE"
tar -C "$PACKAGE_ROOT" -czf "$ARCHIVE" Spare spared bin recipes install.sh uninstall.sh spare-mime.xml VERSION
(
  cd "$PROJECT_ROOT/dist/desktop"
  for desktop_archive in spare-desktop_*.zip spare-desktop_*.tar.gz; do
    [ -f "$desktop_archive" ] || continue
    shasum -a 256 "$desktop_archive"
  done | sort -k2 > checksums.txt
)
echo "Created $ARCHIVE"
echo "Created $PROJECT_ROOT/dist/desktop/checksums.txt"
