#!/usr/bin/env bash
# Config-parity guard.
#
# The plugin's defaults + a couple of shared literals are written by hand in THREE
# places that can silently drift apart:
#   1. include/runner-farm.sh  — the bash engine (the runtime authority)
#   2. default.cfg             — the documented cfg shipped/referenced for operators
#   3. RunnerFarmSettings.page — the UI $defaults array (form fallback + "Reset")
# Nothing else asserts they agree, and they HAVE drifted before (SHARED_IMAGE_CACHE
# once existed in the engine + cfg but was missing from the UI). This script fails
# CI on any value mismatch, any UI field with no engine backing, any cfg key not
# surfaced in the UI (unless explicitly engine-only), and any skew in the shared
# `64` scale cap / runner-name prefix between the engine and exec.php.
#
# Since the engine became multi-fleet the allowlist is split in two (GLOBAL_KEYS =
# host-wide, FLEET_KEYS = per-fleet), so this also asserts that split is well-formed:
# every allowlisted key has an engine default, and no key sits in both layers (which
# would make "is this setting the box's or this fleet's?" unanswerable).
#
# Values here are the project's own trusted source — parsed textually, never eval'd.
set -euo pipefail
cd "$(dirname "$0")/.."
D="src/usr/local/emhttp/plugins/ci-runner-farm"
ENGINE="$D/include/runner-farm.sh"
CFG="$D/default.cfg"
UI="$D/RunnerFarmSettings.page"
EXEC="$D/include/exec.php"

fail=0
bad() { printf 'PARITY FAIL: %s\n' "$*" >&2; fail=1; }

# Keys that legitimately live in the engine/cfg but are NOT user-editable form
# fields (fixed infrastructure names), so they are exempt from the UI-coverage check.
ENGINE_ONLY_IN_CFG=" RUNNER_NETWORK MIRROR_PORT FLEET_MODE "

# Normalize the RHS of a KEY=VALUE line: take the double-quoted value if quoted,
# else the token up to the first whitespace (dropping any trailing inline comment).
parse_val() {
  local rhs="${1#*=}"
  if [ "${rhs#\"}" != "$rhs" ]; then rhs="${rhs#\"}"; rhs="${rhs%%\"*}"; else rhs="${rhs%%[[:space:]]*}"; fi
  printf '%s' "$rhs"
}

declare -A ENG CFGV UIV

# --- engine defaults: the block from the "# ---- defaults" header to its closing
#     pure-dashes divider (the "# ---- image auto-update ----" sub-header has text,
#     so only the final all-dashes line matches and bounds the block) ---
while IFS= read -r line; do
  case "$line" in [A-Z_]*=*) ENG["${line%%=*}"]="$(parse_val "$line")" ;; esac
done < <(awk '/^# ---- defaults/{f=1;next} f&&/^# -+$/{exit} f' "$ENGINE")

# --- default.cfg (plain KEY="value") ---
while IFS= read -r line; do
  case "$line" in \#*|'') continue ;; [A-Z_]*=*) CFGV["${line%%=*}"]="$(parse_val "$line")" ;; esac
done < "$CFG"

# --- UI $defaults array ('KEY'=>'VALUE') ---
while IFS= read -r pair; do
  k="${pair%%\'=>*}"; k="${k#\'}"
  v="${pair#*=>\'}"; v="${v%\'}"
  UIV["$k"]="$v"
done < <(grep -oE "'[A-Za-z_][A-Za-z0-9_]*'[[:space:]]*=>[[:space:]]*'[^']*'" "$UI")

# 1. every UI form field must have an engine default, with the same value
for k in "${!UIV[@]}"; do
  if [ -z "${ENG[$k]+x}" ]; then bad "UI \$defaults has '$k' but the engine has no such default"
  elif [ "${UIV[$k]}" != "${ENG[$k]}" ]; then bad "'$k' differs: UI='${UIV[$k]}' engine='${ENG[$k]}'"; fi
done

# 2. every cfg key must exist in the engine with the same value
for k in "${!CFGV[@]}"; do
  if [ -z "${ENG[$k]+x}" ]; then bad "default.cfg has '$k' but the engine has no such default"
  elif [ "${CFGV[$k]}" != "${ENG[$k]}" ]; then bad "'$k' differs: cfg='${CFGV[$k]}' engine='${ENG[$k]}'"; fi
done

# 3. every cfg key must be surfaced in the UI (or be explicitly engine-only) — the
#    check that would have caught the SHARED_IMAGE_CACHE drift
for k in "${!CFGV[@]}"; do
  [ -n "${UIV[$k]+x}" ] && continue
  case "$ENGINE_ONLY_IN_CFG" in *" $k "*) continue ;; esac
  bad "default.cfg exposes '$k' but it is missing from the UI \$defaults (add a form field, or allowlist it as engine-only)"
done

# 4. the fleet/global key split: every allowlisted key must have an engine default,
#    and no key may appear in both layers.
read_keylist() { sed -n "/^$1=\"/,/[^\\\\]$/p" "$ENGINE" | tr -d '\\\\' | sed -e "s/^$1=//" -e 's/"//g'; }
GLOBALK="$(read_keylist GLOBAL_KEYS)"
FLEETK="$(read_keylist FLEET_KEYS)"
[ -n "$GLOBALK" ] || bad "could not parse GLOBAL_KEYS out of the engine"
[ -n "$FLEETK" ] || bad "could not parse FLEET_KEYS out of the engine"
for k in $GLOBALK $FLEETK; do
  [ -n "${ENG[$k]+x}" ] || bad "'$k' is on the engine's key allowlist but has no engine default"
done
for k in $GLOBALK; do
  case " $(echo $FLEETK) " in *" $k "*) bad "'$k' is in BOTH GLOBAL_KEYS and FLEET_KEYS — it must belong to exactly one layer" ;; esac
done

# 5. shared literals: the manual scale cap and runner-name prefix duplicated across
#    the engine and exec.php must agree.
eng_cap="$(grep -oE 'HARD_MAX=[0-9]+' "$ENGINE" | head -1 | grep -oE '[0-9]+')"
php_cap="$(grep -oE 'min\([0-9]+' "$EXEC" | head -1 | grep -oE '[0-9]+')"
[ -n "$eng_cap" ] && [ "$eng_cap" = "$php_cap" ] || bad "scale hard-cap differs: engine HARD_MAX='$eng_cap' vs exec.php min='$php_cap'"

# NAME_PREFIX is now DERIVED per fleet (NAME_PREFIX="ci-runner${FSFX}"), so there is no
# literal to grep on either side any more — exec.php builds the same regex from its own
# $namePrefix literal. The drift this check exists to catch is now between those two
# literals: the engine's base prefix and exec.php's copy of it.
prefix="$(grep -oE 'NAME_PREFIX="[^"$]+\$\{FSFX\}"' "$ENGINE" | head -1 | sed -E 's/NAME_PREFIX="([^"$]+)\$\{FSFX\}"/\1/')"
php_prefix="$(grep -oE "\\\$namePrefix = '[^']+'" "$EXEC" | head -1 | sed -E "s/.*'([^']+)'/\1/")"
if [ -z "$prefix" ]; then
  bad "could not parse the engine's derived NAME_PREFIX (expected NAME_PREFIX=\"<base>\${FSFX}\")"
elif [ "$prefix" != "$php_prefix" ]; then
  bad "runner-name prefix differs: engine='$prefix' exec.php \$namePrefix='$php_prefix'"
fi

if [ "$fail" -ne 0 ]; then
  echo "config-parity: FAILED — reconcile the defaults/literals above." >&2
  exit 1
fi
echo "config-parity: OK — engine, default.cfg, and the UI \$defaults agree ($(( ${#ENG[@]} )) engine keys, ${#UIV[@]} UI fields, ${#CFGV[@]} cfg keys)."
