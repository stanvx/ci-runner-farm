#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
ENGINE=src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
source <(sed -n '/^cmd_validate()/,/^}/p' "$ENGINE")
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
NAME_PREFIX=runner; RUNDIR="$tmp"; CACHE_ROOT="$tmp/cache"
check_cache_root() { :; }
ensure_dirs() { :; }
registry_login() { :; }
crf_limit_swap() { [ "$failure" != swap ]; }
build_args() { ARGS=(-d image); }
effective_image() { echo image; }
log() { echo "$*"; }
err() { echo "$*" >&2; }
docker() {
  printf '%s\n' "$*" >> "$tmp/calls"
  case "$1" in
    run) [ "$failure" != run ] ;;
    inspect)
      case "$3" in
        '{{.State.Running}}')
          [ "$failure" != running ] || return 1
          if [ "$failure" = exited ]; then echo false; else echo true; fi ;;
        cpus=*) [ "$failure" != resources ] ;;
        '{{range .Mounts}}'*) [ "$failure" != mounts ] ;;
        '{{json .HostConfig.Tmpfs}}') [ "$failure" != tmpfs ] ;;
        *) return 1 ;;
      esac ;;
    exec) [ "$failure" != exec ] ;;
    rm) [ "$failure" != cleanup ] ;;
    *) return 1 ;;
  esac
}
for failure in none run running exited swap resources mounts tmpfs exec cleanup; do
  : > "$tmp/calls"
  mkdir -p "$CACHE_ROOT/docker/runner-validate"
  passed=false
  if cmd_validate > "$tmp/output" 2>&1; then passed=true; fi
  if [ "$failure" = none ]; then
    $passed && grep -q 'validate: OK' "$tmp/output"
  else
    if $passed || grep -q 'validate: OK' "$tmp/output"; then
      echo "validation falsely passed: $failure" >&2; exit 1
    fi
  fi
  [ "$(grep -c '^rm -f runner-validate$' "$tmp/calls")" = 2 ]
  [ ! -d "$CACHE_ROOT/docker/runner-validate" ]
done
echo 'validate: failure checks and cleanup OK'
