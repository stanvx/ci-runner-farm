# CLAUDE.md

Unraid 6.12+ webGUI plugin that runs GitHub Actions self-hosted runners as Docker
containers. Bash engine + PHP endpoint + Dynamix `.page` tabs. No compiler, no
package manager, no lockfile.

## src/ is a filesystem image

`src/usr/local/emhttp/plugins/ci-runner-farm/` mirrors the install path on the
target byte for byte. `build-plg.sh` tars it wholesale and CI runs `diff -r` of
the extracted tarball against it. Anything added under `src/` ships; anything
outside it does not.

## Adding or changing a config key

Defaults are hand-written in three places and `tests/config-parity.sh` fails CI
if they drift:

1. `include/runner-farm.sh` — the `# ---- defaults` block (runtime authority)
2. `default.cfg` — operator-facing documentation (reference only, never copied to flash)
3. `include/crf-config.php` — the `$crf_defaults` PHP array (form fallback + Reset)

A key must also be rendered as a form field in `include/crf-fields.php`, in the
table matching its engine layer: `global` (host-wide, Settings tab) or `fleet`
(per fleet, Fleet tab). The parity test checks each table against the engine's
`GLOBAL_KEYS` / `FLEET_KEYS`, because a field in the wrong table writes to the
wrong cfg file and is then silently ignored.

Engine-only keys must be added to `ENGINE_ONLY_IN_CFG` in the test. See
`/add-config-key`.

## Config is two layers, and neither form uses `/update.php`

`GLOBAL_KEYS` live in the legacy cfg; `FLEET_KEYS` live in `fleets/<name>.cfg`.
Fleet `default` IS the legacy cfg, so that one file holds **both** layers — which
is why saves go through `exec.php`'s `save-config` (which merges) instead of
Unraid's `/update.php` (one hardcoded `#file`, rebuilt from the posted fields).
A whole-file rewrite from either form would delete the other layer.

## Commands

```sh
bash tests/config-parity.sh     # defaults must agree across the three files
bash tests/safe-paths.sh        # SKIPs on macOS (needs GNU realpath -m); runs in CI
./build-plg.sh                  # writes ci-runner-farm.plg + ci-runner-farm.tgz
./deploy.sh root@tower          # dev iteration against a live Unraid host
```

Lint is exactly what CI runs: `bash -n` on every `*.sh` plus shebanged files
under `nchan/` and `event/`, and `php -l` on every `*.php` and `.page` containing
`<?php`. No shellcheck, shfmt, or formatter config exists.

## Release

Full process: @docs/RELEASING.md. The rules that matter while editing:

- `.release-please-manifest.json` is the version. `VERSION` and the committed
  `ci-runner-farm.plg` are **generated** by the release PR job — never hand-edit
  either, and never tag or upload assets manually.
- The version moves only via conventional-commit types on squashed PR subjects.
- The build is byte-reproducible only under GNU tar, so a locally built `.plg`
  must never be committed.

## Gotchas

- The `.plg` embeds nothing: it downloads `ci-runner-farm.tgz` by URL and verifies
  it by MD5. `README.md`'s layout block is otherwise stale — trust `build-plg.sh`.
- Caches go to `RUNDIR=/var/local/emhttp/ci-runner-farm` (tmpfs), never flash —
  writing 60-300s caches to USB is a flash-wear antipattern.
- Secrets live in `/boot/config/plugins/ci-runner-farm/{token,registry-token}`,
  mode 600, never in the `.cfg`.
- `exec.php` gates every request on `hash_equals` against `csrf_token` from
  `var.ini`. `run()` merges stderr (log is the payload); `run_json()` discards it
  so a docker warning can't corrupt a JSON body.
- The `nchan/ci_runner_farm` channel publishes aggregate counts only — `/sub` is
  unauthenticated on the LAN.

## Style

- 2-space indent in bash and PHP. Frontend prefix `crf` / `.crf-*`, shared via
  `include_once include/crf-core.php`.
- Comments explain *why*, naming the specific failure being prevented, not what
  the line does. Match that density.
- `runner-farm.sh` runs `set -uo pipefail` deliberately without `-e`; scripts at
  the repo root use `set -euo pipefail`.
- Mutating engine subcommands go through `with_fleet_lock wait`.

## Agent skills

### Issue tracker

GitHub Issues on `stanvx/ci-runner-farm` (this fork), via the `gh` CLI. See
`docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, each label string equal to its name. See
`docs/agents/triage-labels.md`.

### Domain docs

Single-context — `CONTEXT.md` + `docs/adr/` at the repo root. See
`docs/agents/domain.md`.

## Git

This is a fork. `origin` is `stanvx/ci-runner-farm`; the parent is
`unraid/ci-runner-farm`. **All PRs target `stanvx/ci-runner-farm`, never
upstream.** `gh repo set-default stanvx/ci-runner-farm` is already configured, so
`gh pr create` resolves here — but pass `--repo stanvx/ci-runner-farm` explicitly
if there is any doubt, and never `--base` a branch on the parent. Same for
issues. `build-plg.sh` defaults `REPO=stanvx/ci-runner-farm`; set
`REPO=unraid/ci-runner-farm` only when intentionally building the upstream
repository so the plugin URLs point at the intended release assets.

Conventional commits (release-please depends on them), scoped e.g. `feat(ui):`,
`fix(autoscale):`. PR-based, squash-merged, linear history. Actions are SHA-pinned
and use `persist-credentials: false` unless a push is required.
