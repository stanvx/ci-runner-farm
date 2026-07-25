#!/usr/bin/env bash
# Behavioral tests for multi-fleet resolution in include/runner-farm.sh.
#
# Three things here are high-consequence enough to earn a test, all of them
# invisible until a SECOND fleet exists (so a manual single-fleet check proves
# nothing):
#
#   1. Config layering. A fleet cfg must not be able to set a host-wide key, and
#      the legacy cfg must keep supplying the global layer to every fleet. Get
#      this wrong and one fleet silently repoints another fleet's CACHE_ROOT.
#   2. Derived names. Fleet `default` must resolve to the byte-identical legacy
#      names (that IS the no-op-upgrade invariant); any other fleet must get its
#      own prefix, network and image tag, or two fleets collide on one container.
#   3. The firewall tag matcher. firewall_clear deletes every rule carrying a
#      tag, so a matcher that treats "ci-runner-farm" as a prefix of
#      "ci-runner-farm:build:lan10" would let one fleet's Stop silently strip
#      another strict fleet's egress rules — a security regression with no
#      visible symptom.
#
# Same harness as safe-paths.sh: extract the functions/lines under test from the
# engine (avoiding the script's dispatch and its /boot + docker side effects) and
# table-test them.
set -u
cd "$(dirname "$0")/.."
ENGINE="src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh"

tmpd="$(mktemp -d)"; trap 'rm -rf "$tmpd"' EXIT
pass=0; fail=0
ok()   { pass=$((pass+1)); }
no()   { fail=$((fail+1)); printf 'FAIL  %s\n' "$*"; }
is()   { # <label> <expected> <got>
  if [ "$2" = "$3" ]; then ok; else no "$1: expected '$2' got '$3'"; fi
}

# --- 1. fleet-name validation (the same regex exec.php enforces) -------------
name_ok() { printf '%s' "$1" | grep -qE '^[a-z][a-z0-9-]{0,30}$'; }
for n in default build my-fleet a x9 a-b-c; do
  name_ok "$n" || no "fleet name '$n' should be accepted"
  name_ok "$n" && ok
done
# Rejected: empty, leading digit/dash (would make ci-runner-<fleet>-N ambiguous with
# ci-runner-<index>), uppercase, dots/slashes/spaces (path + label injection), too long.
for n in "" 1build -build Build my.fleet my/fleet "my fleet" '../etc' "$(printf 'a%.0s' $(seq 1 32))"; do
  if name_ok "$n"; then no "fleet name '$n' should be rejected"; else ok; fi
done

# --- 2. cfg_path + the two-layer load --------------------------------------
# shellcheck disable=SC1090  # sourcing an extracted-at-runtime snippet by design
. <(sed -n '/^cfg_path()/,/^}/p;/^load_cfg_file()/,/^}/p;/^load_cfg()/,/^}/p' "$ENGINE")
GLOBAL_KEYS="CACHE_ROOT MIRROR_PORT"
FLEET_KEYS="RUNNER_COUNT RUNNER_LABELS FLEET_MODE"

CFG="$tmpd/ci-runner-farm.cfg"
FLEETDIR="$tmpd/fleets"; mkdir -p "$FLEETDIR"
cat > "$CFG" <<'EOF'
CACHE_ROOT="/mnt/pool/global"
MIRROR_PORT="5001"
RUNNER_COUNT="4"
RUNNER_LABELS="legacy-labels"
EOF
cat > "$FLEETDIR/build.cfg" <<'EOF'
RUNNER_COUNT="9"
FLEET_MODE="legacy"
CACHE_ROOT="/mnt/pool/HIJACKED"
EOF

FLEET=default; is "cfg_path(default) is the legacy cfg" "$CFG" "$(cfg_path)"
FLEET=build;   is "cfg_path(build)" "$FLEETDIR/build.cfg" "$(cfg_path)"

# default: both layers read the same legacy file — that is why an upgrade is a no-op
FLEET=default; CACHE_ROOT=""; MIRROR_PORT=""; RUNNER_COUNT=""; RUNNER_LABELS=""; FLEET_MODE=""
load_cfg
is "default fleet reads its global keys"  "/mnt/pool/global" "$CACHE_ROOT"
is "default fleet reads its fleet keys"   "4"                "$RUNNER_COUNT"

# named fleet: global layer from the legacy cfg, fleet layer from its own cfg
FLEET=build; CACHE_ROOT=""; MIRROR_PORT=""; RUNNER_COUNT=""; RUNNER_LABELS=""; FLEET_MODE=""
load_cfg
is "global key comes from the legacy cfg" "/mnt/pool/global" "$CACHE_ROOT"
is "global key not shadowed by the fleet" "/mnt/pool/global" "$CACHE_ROOT"
is "second global key"                    "5001"             "$MIRROR_PORT"
is "fleet key overrides the global file"  "9"                "$RUNNER_COUNT"
is "fleet mode"                           "legacy"           "$FLEET_MODE"
# RUNNER_LABELS is a FLEET key set only in the legacy cfg, so fleet `build` must NOT
# inherit it — a fleet inherits the box's globals, never another fleet's settings.
is "fleet does not inherit another fleet's key" "" "$RUNNER_LABELS"

# --- 3. derived names -------------------------------------------------------
derive() { # <fleet> -> prints "prefix|network|image|fwtag|legacyfwtag"
  local FLEET="$1" FSFX RUNNER_NETWORK="ci-runner-net" BUILTIN_IMAGE="ci-runner-farm-runner:latest"
  local FW_TAG="ci-runner-farm" LEGACY_FW_TAG NAME_PREFIX
  # shellcheck disable=SC1090
  . <(sed -n '/^FSFX=""/p;/^NAME_PREFIX="ci-runner/p;/^# Names derived per fleet/,/^FW_TAG=/p' "$ENGINE")
  printf '%s|%s|%s|%s|%s' "$NAME_PREFIX" "$RUNNER_NETWORK" "$BUILTIN_IMAGE" "$FW_TAG" "$LEGACY_FW_TAG"
}
is "default keeps every legacy name" \
   "ci-runner|ci-runner-net|ci-runner-farm-runner:latest|ci-runner-farm:default|ci-runner-farm" \
   "$(derive default)"
is "a named fleet gets its own names" \
   "ci-runner-build|ci-runner-net-build|ci-runner-farm-runner-build:latest|ci-runner-farm:build|" \
   "$(derive build)"

# --- 4. firewall tag matching ----------------------------------------------
# Stub iptables: -L prints a fixed listing covering this fleet's rules, another
# fleet's rules, a pre-upgrade untagged-by-fleet rule and an unrelated rule; -D
# records what would be deleted. A tag must match the WHOLE comment.
mkdir -p "$tmpd/bin"
cat > "$tmpd/bin/iptables" <<EOF
#!/usr/bin/env bash
case "\$*" in
  *-L*)
    cat <<'RULES'
num  target  prot opt source   destination
1    DROP    all  --  0.0.0.0/0 0.0.0.0/0 /* ci-runner-farm:default:lan10 */
2    DROP    all  --  0.0.0.0/0 0.0.0.0/0 /* ci-runner-farm:build:lan10 */
3    DROP    all  --  0.0.0.0/0 0.0.0.0/0 /* ci-runner-farm:lan10 */
4    DROP    all  --  0.0.0.0/0 0.0.0.0/0 /* something-else:lan10 */
RULES
    ;;
  *-D*) shift 2; printf '%s\n' "\$2" >> "$tmpd/deleted" ;;
esac
EOF
chmod +x "$tmpd/bin/iptables"
PATH="$tmpd/bin:$PATH"
# shellcheck disable=SC1090
. <(sed -n '/^_firewall_clear()/,/^}/p' "$ENGINE")

fw_case() { # <fleet-tag> <legacy-tag> <expected sorted line numbers, one per chain>
  : > "$tmpd/deleted"
  FW_TAG="$1" LEGACY_FW_TAG="$2" _firewall_clear
  is "firewall_clear tag='$1' legacy='$2'" "$3" "$(sort -u "$tmpd/deleted" | tr '\n' ' ' | sed 's/ $//')"
}
# Fleet `build` deletes only its own rule (rows 1, 3 and 4 must survive).
fw_case "ci-runner-farm:build" "" "2"
# Fleet `default` deletes its own rule AND sweeps the pre-upgrade untagged one —
# but never fleet build's row 2.
fw_case "ci-runner-farm:default" "ci-runner-farm" "1 3"

if [ "$fail" -ne 0 ]; then
  echo "fleet-resolve: FAILED ($fail failed, $pass passed)" >&2
  exit 1
fi
echo "fleet-resolve: OK ($pass checks passed)"
