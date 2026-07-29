#!/bin/sh
set -eu

FROM_APP=0
if [ "${1:-}" = "--from-app" ]; then
  FROM_APP=1
fi

INSTALL_ROOT=${SPARE_DESKTOP_INSTALL_DIR:-"$HOME/.local/lib/spare"}
BIN_ROOT="$HOME/.local/bin"
APPLICATIONS_ROOT="${XDG_DATA_HOME:-"$HOME/.local/share"}/applications"
ICONS_ROOT="${XDG_DATA_HOME:-"$HOME/.local/share"}/icons/hicolor/scalable/apps"
case "$INSTALL_ROOT" in
  ""|"/"|"$HOME")
    echo "Refusing unsafe Spare install directory: $INSTALL_ROOT" >&2
    exit 1
    ;;
esac

if [ "$FROM_APP" -eq 0 ] && [ -x "$INSTALL_ROOT/bin/spare" ]; then
  "$INSTALL_ROOT/bin/spare" uninstall --yes
fi

rm -f "$HOME/.config/autostart/spare-desktop.desktop"
rm -f "$APPLICATIONS_ROOT/run.spare.desktop.desktop"
rm -f "$ICONS_ROOT/run.spare.desktop.svg"
if command -v xdg-mime >/dev/null 2>&1 && [ -f "$INSTALL_ROOT/spare-mime.xml" ]; then
  xdg-mime uninstall --mode user "$INSTALL_ROOT/spare-mime.xml" || true
fi
if [ -L "$BIN_ROOT/spare" ] && [ "$(readlink "$BIN_ROOT/spare")" = "$INSTALL_ROOT/bin/spare" ]; then
  rm -f "$BIN_ROOT/spare"
fi
if [ -L "$BIN_ROOT/spared" ] && [ "$(readlink "$BIN_ROOT/spared")" = "$INSTALL_ROOT/spared" ]; then
  rm -f "$BIN_ROOT/spared"
fi
rm -rf "$INSTALL_ROOT"
echo "Spare Desktop was removed. Recipe folders and received files were left unchanged."
