#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
ENGINE=src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
source <(sed -n '/^crf_memory_bytes()/,/^}/p;/^host_memory_ok()/,/^}/p' "$ENGINE")
err() { :; }
MANAGED_LABEL=managed
RUNNER_MEMORY=12g
crf_available_memory() { echo "$mock_available"; }
crf_resident_memory() { echo "$mock_resident"; }
docker() {
  case "$1" in ps) echo "$mock_ids";; inspect) printf '%s\n' "$mock_caps";; esac
}
[ "$(crf_memory_bytes 12g)" = 12884901888 ]
[ "$(crf_memory_bytes 0012G)" = 12884901888 ]
for bad in 0 2.5g -1g 99999999999999 2p 'a[$(touch /tmp/crf-injection)]'; do
  if crf_memory_bytes "$bad" >/dev/null; then echo "accepted invalid memory: $bad"; exit 1; fi
done
mock_ids=''; mock_caps=''; mock_resident=0
mock_available=$((14*1024*1024*1024)); host_memory_ok
mock_available=$((14*1024*1024*1024-1)); if host_memory_ok; then exit 1; fi
# Existing resident pages already reduce MemAvailable. Reserve only their unused cap.
mock_ids=runner; mock_caps="8589934592 123"; mock_resident=$((6*1024*1024*1024))
mock_available=$((16*1024*1024*1024)); host_memory_ok
mock_available=$((16*1024*1024*1024-1)); if host_memory_ok; then exit 1; fi
# Unknown cgroup usage reserves the whole existing cap, never assumes it is free.
mock_resident=''; mock_available=$((22*1024*1024*1024)); host_memory_ok
mock_available=$((22*1024*1024*1024-1)); if host_memory_ok; then exit 1; fi
mock_caps='0 123'; if host_memory_ok; then exit 1; fi
RUNNER_MEMORY=''; if host_memory_ok; then exit 1; fi
# Exercise the actual kernel-stat reader with a cgroup fixture; file cache and
# reclaimable slab must not be credited as already charged against MemAvailable.
(
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  printf 'anon 100\nshmem 20\nkernel 50\nslab_reclaimable 30\nfile 1000\n' > "$tmp/stat"
  awk() {
    if [ "$1" = -F: ]; then printf '/docker/fixture'; else command awk "$1" "$tmp/stat"; fi
  }
  source <(sed -n '/^crf_resident_memory()/,/^}/p' "$ENGINE" | sed 's|"/sys/fs/cgroup${cg}/memory.stat"|"$tmp/stat"|g')
  [ "$(crf_resident_memory 123)" = 140 ]
)
echo 'host-memory: OK'
