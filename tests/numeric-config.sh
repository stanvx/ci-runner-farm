#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
ENGINE=src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
source <(sed -n '/^CFG_NUMERIC_KEYS=/p;/^load_cfg_file()/,/^}/p' "$ENGINE")
tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'CFG'
RUNNER_COUNT="a[$(touch /tmp/crf-injection)]"
AUTOSCALE_MAX="08"
SCALESET_DRAIN_TIMEOUT="99999999999999999999999"
AUTOSCALE_MIN="0"
AUTOSCALE_STEP="2"
CFG
RUNNER_COUNT=4; AUTOSCALE_MAX=16; SCALESET_DRAIN_TIMEOUT=600; AUTOSCALE_MIN=2; AUTOSCALE_STEP=1
load_cfg_file "$tmp" 'RUNNER_COUNT AUTOSCALE_MAX SCALESET_DRAIN_TIMEOUT AUTOSCALE_MIN AUTOSCALE_STEP' 2>/dev/null
[ "$RUNNER_COUNT" = 4 ] && [ "$AUTOSCALE_MAX" = 16 ] && [ "$SCALESET_DRAIN_TIMEOUT" = 600 ]
[ "$AUTOSCALE_MIN" = 0 ] && [ "$AUTOSCALE_STEP" = 2 ]
echo 'numeric-config: OK'
