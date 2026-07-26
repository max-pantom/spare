#!/bin/sh
set -eu

VERSION=${SPARE_VERSION:-0.1.0}
INSTALL_DIR=${SPARE_INSTALL_DIR:-"$HOME/.local/bin"}
BASE_URL=${SPARE_RELEASE_BASE_URL:-"https://github.com/spare-run/spare/releases/download/v$VERSION"}

if [ "${1:-}" = "--uninstall" ]; then
  if command -v spare >/dev/null 2>&1; then
    spare uninstall --yes || true
  fi
  rm -f "$INSTALL_DIR/spare" "$INSTALL_DIR/spared"
  echo "Spare binaries were removed."
  exit 0
fi

os_name=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os_name" in
  darwin) target_os=darwin ;;
  linux) target_os=linux ;;
  *) echo "Spare supports macOS and Linux with this installer." >&2; exit 1 ;;
esac

machine=$(uname -m)
case "$machine" in
  x86_64|amd64) target_arch=amd64 ;;
  arm64|aarch64) target_arch=arm64 ;;
  *) echo "Spare supports amd64 and arm64 computers." >&2; exit 1 ;;
esac

mkdir -p "$INSTALL_DIR"
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ -x "$script_dir/spare" ] && [ -x "$script_dir/spared" ]; then
  cp "$script_dir/spare" "$INSTALL_DIR/spare"
  cp "$script_dir/spared" "$INSTALL_DIR/spared"
else
  archive="spare_${VERSION}_${target_os}_${target_arch}.tar.gz"
  download_dir=$(mktemp -d)
  trap 'rm -rf "$download_dir"' EXIT INT TERM
  curl -fsSL "$BASE_URL/$archive" -o "$download_dir/$archive"
  curl -fsSL "$BASE_URL/checksums.txt" -o "$download_dir/checksums.txt"
  expected=$(awk -v name="$archive" '$2 == "./" name || $2 == name { print $1 }' "$download_dir/checksums.txt")
  if [ -z "$expected" ]; then
    echo "The release checksum is missing." >&2
    exit 1
  fi
  actual=$(shasum -a 256 "$download_dir/$archive" | awk '{print $1}')
  if [ "$actual" != "$expected" ]; then
    echo "The Spare archive checksum did not match." >&2
    exit 1
  fi
  tar -xzf "$download_dir/$archive" -C "$download_dir"
  cp "$download_dir/spare" "$INSTALL_DIR/spare"
  cp "$download_dir/spared" "$INSTALL_DIR/spared"
fi

chmod 755 "$INSTALL_DIR/spare" "$INSTALL_DIR/spared"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add $INSTALL_DIR to PATH before running Spare." ;;
esac

"$INSTALL_DIR/spare" init

