#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
ENGINE=src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
source <(sed -n '/^crf_limit_swap()/,/^}/p' "$ENGINE")
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mock_file="$tmp/memory.swap.max"; mock_support=false
crf_swap_file() { printf '%s' "$mock_file"; }
docker() { echo "$mock_support"; }
err() { :; }
echo max > "$mock_file"; crf_limit_swap runner; [ "$(cat "$mock_file")" = 0 ]
rm "$mock_file"; mock_support=true; crf_limit_swap runner
mock_support=false; if crf_limit_swap runner; then exit 1; fi
echo 'swap-limits: OK'
