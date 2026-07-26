#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

ENGINE=src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
source <(sed -n '/^timeout_value()/,/^}/p' "$ENGINE")

[ "$(timeout_value '' 600)" = 600 ]
[ "$(timeout_value bad 3600)" = 3600 ]
[ "$(timeout_value 0 600)" = 0 ]
[ "$(timeout_value 42 600)" = 42 ]
echo "timeout-fallback: OK"
