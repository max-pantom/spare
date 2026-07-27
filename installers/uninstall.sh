#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
bundled_version=
if [ -f "$script_dir/VERSION" ]; then
  IFS= read -r bundled_version < "$script_dir/VERSION"
fi
VERSION=${SPARE_VERSION:-${bundled_version:-0.1.0}}
INSTALL_DIR=${SPARE_INSTALL_DIR:-"$HOME/.local/bin"}

if [ -x "$INSTALL_DIR/spare" ]; then
  "$INSTALL_DIR/spare" uninstall --yes || true
fi

data_home=${XDG_DATA_HOME:-"$HOME/.local/share"}
rm -f "$data_home/applications/spare-recipe-viewer.desktop"
rm -f "$data_home/mime/packages/spare-recipe.xml"
if command -v update-mime-database >/dev/null 2>&1 && [ -d "$data_home/mime" ]; then
  update-mime-database "$data_home/mime" >/dev/null 2>&1 || true
fi
if command -v update-desktop-database >/dev/null 2>&1 && [ -d "$data_home/applications" ]; then
  update-desktop-database "$data_home/applications" >/dev/null 2>&1 || true
fi

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
echo "Spare binaries were removed. Site source folders were left unchanged."
