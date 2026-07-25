---
name: add-config-key
description: Add, rename, or remove a ci-runner-farm config key across the three files that must stay in sync. Use whenever a new setting is introduced or an existing default changes.
---

Defaults are hand-written in three files. `tests/config-parity.sh` parses all
three textually and fails CI on any drift. Touch all three, in this order.

Key name (or the change requested): $ARGUMENTS

## 1. Engine — runtime authority

`src/usr/local/emhttp/plugins/ci-runner-farm/include/runner-farm.sh`, inside the
`# ---- defaults` block (it ends at the next `# ----` rule line). Add
`KEY="value"` with a trailing `# comment` explaining why the default is what it
is. The parity test reads only lines between those two markers.

## 2. Operator docs

`src/usr/local/emhttp/plugins/ci-runner-farm/default.cfg`. Reference only — it is
never copied to flash. Same key and same value as the engine.

## 3. UI

`src/usr/local/emhttp/plugins/ci-runner-farm/RunnerFarmSettings.page`:

- Add `'KEY' => 'value',` to the `$defaults` array (~line 22). This array is
  authoritative for the form fallback and the Reset button, and is serialized to
  the frontend as `CRF_DEFAULTS`.
- Add the matching form field, using `crf_g($cfg,'KEY')` for text inputs or
  `crf_sel($cfg,'KEY',$val)` for selects.

If the key is genuinely engine-only and must have no UI field, add it to
`ENGINE_ONLY_IN_CFG` in `tests/config-parity.sh` instead of adding a field.

## 4. Wire it up and verify

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
php -l src/usr/local/emhttp/plugins/ci-runner-farm/RunnerFarmSettings.page
```
