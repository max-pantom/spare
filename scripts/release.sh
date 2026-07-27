#!/bin/sh
set -eu
LC_ALL=C
export LC_ALL

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RELEASE_DIR="$PROJECT_ROOT/dist/releases"
RELEASE_VERSION=${VERSION:-0.1.0}

mkdir -p "$RELEASE_DIR"
find "$RELEASE_DIR" -mindepth 1 -maxdepth 1 -type f -delete

go run "$PROJECT_ROOT/cmd/spare" recipe pack "$PROJECT_ROOT/recipes/site" \
  --output "$RELEASE_DIR/site_${RELEASE_VERSION}.sp"
go run "$PROJECT_ROOT/cmd/spare" recipe pack "$PROJECT_ROOT/recipes/drop" \
  --output "$RELEASE_DIR/drop_${RELEASE_VERSION}.sp"
go run "$PROJECT_ROOT/cmd/spare" recipe pack "$PROJECT_ROOT/recipes/hook" \
  --output "$RELEASE_DIR/hook_${RELEASE_VERSION}.sp"

for target in \
  darwin/amd64 \
  darwin/arm64 \
  windows/amd64 \
  windows/arm64 \
  linux/amd64 \
  linux/arm64
do
  target_os=${target%/*}
  target_arch=${target#*/}
  package_name="spare_${RELEASE_VERSION}_${target_os}_${target_arch}"
  package_dir=$(mktemp -d)
  binary_suffix=
  if [ "$target_os" = "windows" ]; then
    binary_suffix=.exe
  fi

  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
    go build -trimpath -ldflags="-s -w -X main.version=$RELEASE_VERSION" \
    -o "$package_dir/spare$binary_suffix" "$PROJECT_ROOT/cmd/spare"
  daemon_ldflags="-s -w -X main.version=$RELEASE_VERSION"
  if [ "$target_os" = "windows" ]; then
    daemon_ldflags="$daemon_ldflags -H=windowsgui"
  fi
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
    go build -trimpath -ldflags="$daemon_ldflags" \
    -o "$package_dir/spared$binary_suffix" "$PROJECT_ROOT/cmd/spared"

  cp "$PROJECT_ROOT/README.md" "$package_dir/README.md"
  printf '%s\n' "$RELEASE_VERSION" > "$package_dir/VERSION"
  mkdir -p "$package_dir/recipes"
  cp "$RELEASE_DIR/site_${RELEASE_VERSION}.sp" "$package_dir/recipes/"
  cp "$RELEASE_DIR/drop_${RELEASE_VERSION}.sp" "$package_dir/recipes/"
  cp "$RELEASE_DIR/hook_${RELEASE_VERSION}.sp" "$package_dir/recipes/"
  if [ "$target_os" = "windows" ]; then
    cp "$PROJECT_ROOT/installers/install.ps1" "$package_dir/install.ps1"
    cp "$PROJECT_ROOT/installers/uninstall.ps1" "$package_dir/uninstall.ps1"
    (
      cd "$package_dir"
      zip -qr "$RELEASE_DIR/$package_name.zip" ./*
    )
  else
    cp "$PROJECT_ROOT/installers/install.sh" "$package_dir/install.sh"
    cp "$PROJECT_ROOT/installers/uninstall.sh" "$package_dir/uninstall.sh"
    chmod +x "$package_dir/install.sh" "$package_dir/uninstall.sh"
    tar -C "$package_dir" -czf "$RELEASE_DIR/$package_name.tar.gz" .
  fi
  rm -rf "$package_dir"
done

(
  cd "$RELEASE_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.sp ./*.tar.gz ./*.zip > checksums.txt
  else
    shasum -a 256 ./*.sp ./*.tar.gz ./*.zip > checksums.txt
  fi
)

echo "Created Spare $RELEASE_VERSION release artifacts in $RELEASE_DIR"
