#!/usr/bin/env bash
# Removes what get.sh installed. Settings are kept.
set -euo pipefail
DATA="${XDG_DATA_HOME:-$HOME/.local/share}"
pkill -x base-station-manager 2>/dev/null || true
rm -rf "$DATA/base-station-manager" "$DATA/applications/base-station-manager.desktop" "$HOME/.local/bin/base-station-manager"
echo "Base Station Manager removed. / Видалено."
