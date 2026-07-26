---
name: add-config-key
description: Add, rename, or remove a ci-runner-farm config key across the three files that must stay in sync. Use whenever a new setting is introduced or an existing default changes.
---

Defaults are hand-written in three files, and the form field that surfaces a key
lives in a fourth. `tests/config-parity.sh` parses all four textually and fails CI
on any drift. Touch them in this order.

Key name (or the change requested): $ARGUMENTS

## 1. Engine — runtime authority

`src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh`, inside the
`# ---- defaults` block (it ends at the next `# ----` rule line). Add
`KEY="value"` with a trailing `# comment` explaining why the default is what it
is. The parity test reads only lines between those two markers.

## 2. Operator docs

`src/usr/local/emhttp/plugins/ci-runner-farm/default.cfg`. Reference only — it is
never copied to flash. Same key and same value as the engine.

## 3. UI default

`src/usr/local/emhttp/plugins/ci-runner-farm/include/crf-config.php`: add
`'KEY' => 'value',` to the `$crf_defaults` array. It is authoritative for the form
fallback and the Reset button, and is serialized to the frontend as `CRF_DEFAULTS`.

## 4. UI field

`src/usr/local/emhttp/plugins/ci-runner-farm/include/crf-fields.php`: add a row to
`crf_fields()`, **in the table matching the key's engine layer** — `$global` for a
`GLOBAL_KEYS` key (host-wide; renders on the Settings tab) or `$fleet` for a
`FLEET_KEYS` key (per fleet; renders in the Fleet tab's configuration column).
A field in the wrong table writes to the wrong cfg file and the engine's
other-layer read never sees it — the value lands on flash and nothing reports it.
`tests/config-parity.sh` checks both tables against the engine's key lists.

Row shape: `['KEY','Label','text|number|select',['help'=>'…', …]]`. `select` takes
`'options'=>['value'=>'label', …]`; `number` takes `'min'`/`'max'`; text takes
`'placeholder'`. Help is HTML (single-quoted PHP — use `&rsquo;`, never a raw `'`).

Also add the key to the engine's `GLOBAL_KEYS` or `FLEET_KEYS` allowlist in
`runner-farm.sh` — a key absent from both is never read off flash at all.

If the key is genuinely engine-only and must have no UI field, add it to
`ENGINE_ONLY_IN_CFG` in `tests/config-parity.sh` instead of adding a field.

## 5. Wire it up and verify

- Consume the key in `runner-farm.sh` where it matters; if it affects a running
  fleet, check whether `reconcile-config` needs to handle it.
- If it is a path used for `rm -rf`, `chown -R`, or a bind mount, it must pass
  `crf_safe_cache_root` / `crf_safe_mount_subdir` — add a case to
  `tests/safe-paths.sh`.
- Secrets do not go in the `.cfg`; they go in
  `/boot/config/plugins/ci-runner-farm/<name>` at mode 600.

Then run:

```sh
bash tests/config-parity.sh
bash tests/safe-paths.sh
bash -n src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh
# php -l on every *.php and .page containing <?php (CI runs this; php is
# usually absent on macOS)
```
