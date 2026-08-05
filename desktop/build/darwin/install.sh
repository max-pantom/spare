#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
source_app="$script_dir/Spare.app"
target_root="$HOME/Applications"
target_app="$target_root/Spare.app"

if [ ! -d "$source_app" ]; then
  echo "Spare.app is missing from this installer." >&2
  exit 1
fi

host_arch=$(uname -m)
app_archs=$(lipo -archs "$source_app/Contents/MacOS/Spare" 2>/dev/null || true)
case "$host_arch" in
  x86_64) required_arch=x86_64 ;;
  arm64) required_arch=arm64 ;;
  *)
    echo "This Mac uses an unsupported processor architecture: $host_arch" >&2
    exit 1
    ;;
esac
case " $app_archs " in
  *" $required_arch "*)
    ;;
  *)
    if [ "$required_arch" = "x86_64" ]; then
      package_name="darwin_amd64"
      processor_name="Intel"
    else
      package_name="darwin_arm64"
      processor_name="Apple Silicon"
    fi
    echo "This Spare application does not support this $processor_name Mac." >&2
    echo "Use the Spare Desktop archive ending in ${package_name}.zip." >&2
    exit 1
    ;;
esac

mkdir -p "$target_root"
if [ -d "$target_app" ]; then
  stop_installed_process() {
    executable=$1
    process_ids=$(ps -Ao pid=,comm= | awk -v executable="$executable" '$2 == executable {print $1}')
    if [ -z "$process_ids" ]; then
      return
    fi
    kill $process_ids 2>/dev/null || true
    attempts=0
    while [ "$attempts" -lt 20 ]; do
      process_ids=$(ps -Ao pid=,comm= | awk -v executable="$executable" '$2 == executable {print $1}')
      if [ -z "$process_ids" ]; then
        return
      fi
      attempts=$((attempts + 1))
      sleep 0.1
    done
    kill -KILL $process_ids 2>/dev/null || true
  }

  stop_installed_process "$target_app/Contents/MacOS/Spare"
  stop_installed_process "$target_app/Contents/MacOS/spared"
  archive="$HOME/.Trash/Spare-previous-$(date +%Y%m%d-%H%M%S).app"
  mkdir -p "$HOME/.Trash"
  mv "$target_app" "$archive"
  echo "Moved the previous application to $archive."
fi

ditto "$source_app" "$target_app"
chmod 755 "$target_app/Contents/MacOS/Spare"
chmod 755 "$target_app/Contents/MacOS/spared"
chmod 755 "$target_app/Contents/Resources/bin/spare"
mkdir -p "$HOME/.local/bin"
ln -sf "$target_app/Contents/Resources/bin/spare" "$HOME/.local/bin/spare"
ln -sf "$target_app/Contents/MacOS/spared" "$HOME/.local/bin/spared"
open "$target_app"

echo "Installed Spare in $target_app."
echo "Spare is opening now and will initialize this user automatically."
