#!/bin/sh
set -eu

INSTALL_DIR=${SPARE_INSTALL_DIR:-"$HOME/.local/bin"}

if [ -x "$INSTALL_DIR/spare" ]; then
  "$INSTALL_DIR/spare" uninstall --yes || true
fi
rm -f "$INSTALL_DIR/spare" "$INSTALL_DIR/spared"
echo "Spare binaries were removed. Site source folders were left unchanged."

