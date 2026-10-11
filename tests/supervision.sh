#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
ENGINE=src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
source <(sed -n '/^scaleset_pids()/,/^}/p;/^scaleset_status()/,/^}/p;/^scaleset_stop()/,/^}/p;/^cmd_supervise()/,/^}/p;/^_supervise_one()/,/^}/p;/^trim_farm_log()/,/^}/p;/^cmd_boot_autostart()/,/^}/p;/^_boot_start_if_enabled()/,/^}/p;/^cmd_fleet_rename()/,/^}/p;/^cmd_fleet_delete()/,/^}/p' "$ENGINE")
tmpd="$(mktemp -d)"; trap 'rm -rf "$tmpd"' EXIT
CFGDIR="$tmpd"; FLEETDIR="$tmpd/fleets"; mkdir -p "$FLEETDIR"
FLEET=build; FLEET_MODE=scale-set; SCALESET_NAME=builders
SCALESETD=/usr/local/emhttp/plugins/ci-runner-farm/bin/crf-scalesetd
SCALESET_PID="$tmpd/listener.pid"; STOPPED_FILE="$tmpd/stopped-build"; FARM_LOG="$tmpd/farm.log"
ps() { printf '%s\n' "$mock_ps"; }
log() { :; }; err() { :; }
timeout_value() { echo 0; }
kill() { printf '%s\n' "$*" >> "$tmpd/signals"; mock_ps=''; }
mock_ps="101 unrelated-service
202 $SCALESETD run --fleet build2
303 $SCALESETD run --fleet build"
echo 101 > "$SCALESET_PID"
[ "$(scaleset_status)" = 'running (pid 303)' ]
rm "$SCALESET_PID"
[ "$(scaleset_status)" = 'running (pid 303)' ]
echo 101 > "$SCALESET_PID"
scaleset_stop force
[ "$(cat "$tmpd/signals")" = '-KILL 303' ]
mock_ps="101 unrelated-service
202 $SCALESETD run --fleet build2"
[ "$(scaleset_status)" = stopped ]

cfg_path() { echo "$FLEETDIR/${1:-$FLEET}.cfg"; }
drain_target() { cat "$FLEETDIR/${1:-$FLEET}.draining" 2>/dev/null || true; }
check_cache_root() { return 0; }; docker() { return 0; }
with_fleet_lock() { shift; "$@"; }
cmd_start() { echo started >> "$tmpd/starts"; }
echo configured > "$FLEETDIR/build.cfg"
: > "$STOPPED_FILE"
cmd_supervise; cmd_boot_autostart
[ ! -f "$tmpd/starts" ]
rm "$STOPPED_FILE"
cmd_supervise
[ "$(wc -l < "$tmpd/starts" | tr -d ' ')" = 1 ]
echo target > "$FLEETDIR/build.draining"
cmd_supervise
[ "$(wc -l < "$tmpd/starts" | tr -d ' ')" = 1 ]
rm "$FLEETDIR/build.draining"
# A Stop that lands during boot's readiness wait must still win under the lock.
: > "$STOPPED_FILE"
_boot_start_if_enabled
[ "$(wc -l < "$tmpd/starts" | tr -d ' ')" = 1 ]

(
  source <(sed -n '/^cmd_stop()/,/^}/p' "$ENGINE")
  autoscale_stop() { :; }; imageupdate_stop() { :; }
  scaleset_stop() { [ -f "$STOPPED_FILE" ]; }
  managed_names() { :; }; firewall_clear() { :; }
  NETWORK_ISOLATION=off; MIRROR_NAME=fixture
  rm -f "$STOPPED_FILE"
  cmd_stop
  [ -f "$STOPPED_FILE" ]
  cmd_supervise; cmd_boot_autostart
  [ "$(wc -l < "$tmpd/starts" | tr -d ' ')" = 1 ]
)

# Trim through the existing inode: a listener with an open append fd keeps writing.
dd if=/dev/zero of="$FARM_LOG" bs=1024 count=4200 2>/dev/null
exec 5>>"$FARM_LOG"
trim_farm_log
[ "$(wc -c < "$FARM_LOG" | tr -d ' ')" = 262144 ]
printf 'still-open\n' >&5
[ "$(tail -c 11 "$FARM_LOG")" = still-open ]
exec 5>&-

fleet_container_count() { echo 0; }
autoscale_stop() { echo autoscale >> "$tmpd/teardown"; }
imageupdate_stop() { echo image >> "$tmpd/teardown"; }
scaleset_stop() { echo listener >> "$tmpd/teardown"; mock_ps=''; }
json_escape() { cat; }
mock_ps="303 $SCALESETD run --fleet build"
cmd_fleet_rename build renamed >/dev/null
[ ! -f "$FLEETDIR/build.cfg" ] && [ -f "$FLEETDIR/renamed.cfg" ]
[ ! -f "$CFGDIR/stopped-build" ] && [ -f "$CFGDIR/stopped-renamed" ]
[ "$(cat "$tmpd/teardown")" = "autoscale
image
listener" ]
FLEET=renamed; STOPPED_FILE="$CFGDIR/stopped-renamed"
cmd_fleet_delete renamed >/dev/null
[ ! -f "$FLEETDIR/renamed.cfg" ] && [ ! -f "$STOPPED_FILE" ]
# Active fleets keep their intent to run under the new name, without old daemons.
FLEET=build; STOPPED_FILE="$CFGDIR/stopped-build"
echo configured > "$FLEETDIR/build.cfg"
mock_ps="303 $SCALESETD run --fleet build"
cmd_fleet_rename build active-renamed >/dev/null
[ ! -f "$FLEETDIR/build.cfg" ] && [ -f "$FLEETDIR/active-renamed.cfg" ]
[ ! -f "$CFGDIR/stopped-build" ] && [ ! -f "$CFGDIR/stopped-active-renamed" ]

# Explicit Start, including the direct scaleset-start verb, clears Stop intent.
(
  source <(sed -n '/^scaleset_start()/,/^}/p' "$ENGINE")
  SCALESETD="$tmpd/fake-listener"; : > "$SCALESETD"; chmod +x "$SCALESETD"
  nohup() { :; }
  : > "$STOPPED_FILE"
  scaleset_start
  [ ! -f "$STOPPED_FILE" ]
)
# Uninstall must stop every configured fleet before removing its runtime.
grep -q 'runner-farm.sh" stop-all' build-plg.sh
[ "$(sed -n 's/^  stop-all) *\(.*\) ;;$/\1/p' "$ENGINE")" = 'for_each_fleet stop' ]
echo 'supervision: OK' 
