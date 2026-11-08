#!/usr/bin/env bash
# Builds Vencord from source with the Roundbar plugin, for Vesktop.
# Vesktop then loads it from: Settings → Vesktop Settings → Vencord Location.
# Re-run after Vencord updates (or to update the plugin).
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
dir="${XDG_DATA_HOME:-$HOME/.local/share}/roundbar/vencord"

if [[ -d $dir/.git ]]; then
  git -C "$dir" pull --ff-only --quiet
else
  mkdir -p "$(dirname "$dir")"
  git clone --depth 1 --quiet https://github.com/Vendicated/Vencord.git "$dir"
fi
mkdir -p "$dir/src/userplugins"
rm -rf "$dir/src/userplugins/roundbar"
cp -r "$here/vencord-plugin/roundbar" "$dir/src/userplugins/roundbar"
cd "$dir"
pnpm install --frozen-lockfile --silent
pnpm build >/dev/null
echo
echo "Built: $dir/dist"
echo "In Vesktop: Settings → Vesktop Settings → Vencord Location → choose that folder, restart Vesktop,"
echo "then enable the Roundbar plugin in Settings → Plugins."
