#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
bundled_version=
if [ -f "$script_dir/VERSION" ]; then
  IFS= read -r bundled_version < "$script_dir/VERSION"
fi
VERSION=${SPARE_VERSION:-${bundled_version:-0.1.0}}
INSTALL_DIR=${SPARE_INSTALL_DIR:-"$HOME/.local/bin"}
BASE_URL=${SPARE_RELEASE_BASE_URL:-"https://github.com/spare-run/spare/releases/download/v$VERSION"}

if [ "${1:-}" = "--uninstall" ]; then
  if command -v spare >/dev/null 2>&1; then
    spare uninstall --yes || true
  fi
  data_home=${XDG_DATA_HOME:-"$HOME/.local/share"}
  rm -f "$data_home/applications/spare-recipe-viewer.desktop"
  rm -f "$data_home/mime/packages/spare-recipe.xml"
  viewer_app="$HOME/Applications/Spare Recipe Viewer.app"
  if [ -d "$viewer_app" ]; then
    launch_services="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
    if [ -x "$launch_services" ]; then
      "$launch_services" -u "$viewer_app" >/dev/null 2>&1 || true
    fi
    rm -rf "$viewer_app"
  fi
  rm -f "$INSTALL_DIR/spare" "$INSTALL_DIR/spared"
  if [ "$(uname -s | tr '[:upper:]' '[:lower:]')" = "darwin" ]; then
    installed_recipes="$HOME/Library/Application Support/Spare/recipes"
  else
    installed_recipes="${XDG_STATE_HOME:-"$HOME/.local/state"}/spare/recipes"
  fi
  rm -f "$installed_recipes/site_${VERSION}.sp"
  rm -f "$installed_recipes/drop_${VERSION}.sp"
  rm -f "$installed_recipes/hook_${VERSION}.sp"
  rmdir "$installed_recipes" >/dev/null 2>&1 || true
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

if [ -x "$script_dir/spare" ] && [ -x "$script_dir/spared" ]; then
  cp "$script_dir/spare" "$INSTALL_DIR/spare"
  cp "$script_dir/spared" "$INSTALL_DIR/spared"
  recipe_source="$script_dir/recipes"
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
  recipe_source="$download_dir/recipes"
fi

chmod 755 "$INSTALL_DIR/spare" "$INSTALL_DIR/spared"

if [ "$target_os" = "darwin" ]; then
  recipe_dir="$HOME/Library/Application Support/Spare/recipes"
else
  state_home=${XDG_STATE_HOME:-"$HOME/.local/state"}
  recipe_dir="$state_home/spare/recipes"
fi
mkdir -p "$recipe_dir"
for recipe_id in site drop hook; do
  recipe_package="$recipe_source/${recipe_id}_${VERSION}.sp"
  if [ ! -f "$recipe_package" ]; then
    echo "The release archive is missing $recipe_package." >&2
    exit 1
  fi
  cp "$recipe_package" "$recipe_dir/${recipe_id}_${VERSION}.sp"
done
echo "Installed the default recipe packages in $recipe_dir."

if [ "$target_os" = "linux" ]; then
  data_home=${XDG_DATA_HOME:-"$HOME/.local/share"}
  mime_packages="$data_home/mime/packages"
  applications="$data_home/applications"
  mkdir -p "$mime_packages" "$applications"
  cat > "$mime_packages/spare-recipe.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">
  <mime-type type="application/vnd.spare.recipe+zip">
    <comment>Spare recipe package</comment>
    <glob pattern="*.sp"/>
  </mime-type>
</mime-info>
EOF
  cat > "$applications/spare-recipe-viewer.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Spare Recipe Viewer
Comment=Inspect a Spare recipe package
Exec="$INSTALL_DIR/spare" view %f
Terminal=false
MimeType=application/vnd.spare.recipe+zip;
Categories=Utility;Development;
NoDisplay=true
EOF
  if command -v update-mime-database >/dev/null 2>&1; then
    update-mime-database "$data_home/mime" >/dev/null 2>&1 || true
  fi
  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$applications" >/dev/null 2>&1 || true
  fi
  if command -v xdg-mime >/dev/null 2>&1; then
    xdg-mime default spare-recipe-viewer.desktop application/vnd.spare.recipe+zip >/dev/null 2>&1 || true
  fi
  echo "Registered .sp files with Spare Recipe Viewer."
fi

if [ "$target_os" = "darwin" ] && command -v osacompile >/dev/null 2>&1 && command -v plutil >/dev/null 2>&1; then
  viewer_app="$HOME/Applications/Spare Recipe Viewer.app"
  viewer_source=$(mktemp)
  escaped_spare=$(printf '%s' "$INSTALL_DIR/spare" | sed 's/\\/\\\\/g; s/"/\\"/g')
  {
    printf 'on open recipeFiles\n'
    printf '  repeat with recipeFile in recipeFiles\n'
    printf '    set recipePath to POSIX path of recipeFile\n'
    printf '    do shell script ((quoted form of "%s") & " view " & (quoted form of recipePath) & " >/dev/null 2>&1 &")\n' "$escaped_spare"
    printf '  end repeat\n'
    printf 'end open\n'
    printf 'on run\n'
    printf '  display dialog "Open a .sp file to inspect it with Spare." buttons {"OK"} default button "OK"\n'
    printf 'end run\n'
  } > "$viewer_source"
  mkdir -p "$(dirname "$viewer_app")"
  rm -rf "$viewer_app"
  if osacompile -o "$viewer_app" "$viewer_source" >/dev/null 2>&1; then
    viewer_plist="$viewer_app/Contents/Info.plist"
    plutil -replace CFBundleIdentifier -string "run.spare.recipe-viewer" "$viewer_plist" >/dev/null 2>&1 ||
      plutil -insert CFBundleIdentifier -string "run.spare.recipe-viewer" "$viewer_plist"
    plutil -replace CFBundleName -string "Spare Recipe Viewer" "$viewer_plist" >/dev/null 2>&1 ||
      plutil -insert CFBundleName -string "Spare Recipe Viewer" "$viewer_plist"
    plutil -remove CFBundleDocumentTypes "$viewer_plist" >/dev/null 2>&1 || true
    plutil -insert CFBundleDocumentTypes -json '[{"CFBundleTypeName":"Spare recipe package","CFBundleTypeExtensions":["sp"],"CFBundleTypeRole":"Viewer","LSHandlerRank":"Owner"}]' "$viewer_plist"
    launch_services="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
    if [ -x "$launch_services" ]; then
      "$launch_services" -f "$viewer_app" >/dev/null 2>&1 || true
    fi
    echo "Registered .sp files with Spare Recipe Viewer."
  else
    rm -rf "$viewer_app"
  fi
  rm -f "$viewer_source"
fi

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add $INSTALL_DIR to PATH before running Spare." ;;
esac

"$INSTALL_DIR/spare" init
