#!/bin/sh
set -eu

SOURCE_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
INSTALL_ROOT=${SPARE_DESKTOP_INSTALL_DIR:-"$HOME/.local/lib/spare"}
BIN_ROOT="$HOME/.local/bin"
APPLICATIONS_ROOT="${XDG_DATA_HOME:-"$HOME/.local/share"}/applications"
case "$INSTALL_ROOT" in
  ""|"/"|"$HOME")
    echo "Refusing unsafe Spare install directory: $INSTALL_ROOT" >&2
    exit 1
    ;;
esac

for required in Spare spared bin/spare uninstall.sh spare-mime.xml; do
  if [ ! -f "$SOURCE_ROOT/$required" ]; then
    echo "The Spare Desktop package is missing $required." >&2
    exit 1
  fi
done

mkdir -p "$INSTALL_ROOT/bin" "$INSTALL_ROOT/recipes" "$BIN_ROOT" "$APPLICATIONS_ROOT"
cp "$SOURCE_ROOT/Spare" "$INSTALL_ROOT/Spare"
cp "$SOURCE_ROOT/spared" "$INSTALL_ROOT/spared"
cp "$SOURCE_ROOT/bin/spare" "$INSTALL_ROOT/bin/spare"
cp "$SOURCE_ROOT/uninstall.sh" "$INSTALL_ROOT/uninstall.sh"
cp "$SOURCE_ROOT/spare-mime.xml" "$INSTALL_ROOT/spare-mime.xml"
cp "$SOURCE_ROOT/VERSION" "$INSTALL_ROOT/VERSION"
cp "$SOURCE_ROOT"/recipes/*.sp "$INSTALL_ROOT/recipes/"
chmod 755 "$INSTALL_ROOT/Spare" "$INSTALL_ROOT/spared" "$INSTALL_ROOT/bin/spare" "$INSTALL_ROOT/uninstall.sh"
ln -sf "$INSTALL_ROOT/bin/spare" "$BIN_ROOT/spare"
ln -sf "$INSTALL_ROOT/spared" "$BIN_ROOT/spared"

cat > "$APPLICATIONS_ROOT/run.spare.desktop.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Spare
Comment=Give this computer a job
Exec="$INSTALL_ROOT/Spare" %f
Icon=folder-publicshare-symbolic
Terminal=false
Categories=Utility;Network;
MimeType=application/vnd.spare.recipe+zip;application/vnd.spare.backup+zip;
StartupNotify=true
EOF

if command -v xdg-mime >/dev/null 2>&1; then
  xdg-mime install --mode user "$INSTALL_ROOT/spare-mime.xml"
  xdg-mime default run.spare.desktop.desktop application/vnd.spare.recipe+zip
  xdg-mime default run.spare.desktop.desktop application/vnd.spare.backup+zip
fi

"$INSTALL_ROOT/Spare" >/dev/null 2>&1 &
echo "Spare Desktop was installed in $INSTALL_ROOT."
echo "The app will initialize its per-user background service automatically."
