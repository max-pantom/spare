#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
app_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)

case "$app_root" in
  "$HOME/Applications/Spare.app"|"/Applications/Spare.app")
    ;;
  *)
    echo "Refusing to remove an unexpected application path: $app_root" >&2
    exit 1
    ;;
esac

sleep 1
rm -f "$HOME/.local/bin/spare" "$HOME/.local/bin/spared"
rm -rf "$app_root"
echo "Spare was removed. Selected recipe folders were left unchanged."
