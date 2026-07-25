#!/usr/bin/env bash
# Re-sync references/ from upstream community docs (MIT, mstrhakr/plugin-docs).
set -euo pipefail

SK="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

git clone --depth 1 https://github.com/mstrhakr/plugin-docs "$TMP/src"

rsync -a --delete "$TMP/src/docs/" "$SK/references/docs/"
rsync -a --delete --exclude '.git' "$TMP/src/validation/" "$SK/references/validation/"

# MIT requires the copyright notice ride along with the copy.
cp "$TMP/src/LICENSE" "$SK/references/LICENSE.plugin-docs"

# Jekyll {% link docs/foo.md %} -> docs/foo.md, so cross-refs are real paths.
find "$SK/references" -name '*.md' -exec perl -pi -e 's/\{%\s*link\s+(\S+?)\s*%\}/$1/g' {} +

echo "synced $(cd "$TMP/src" && git log -1 --format='%h %cs')"
