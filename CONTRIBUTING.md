# Contributing

## Layout rule

`src/usr/local/emhttp/plugins/ci-runner-farm/` is a literal image of the install
path on Unraid. Anything you add under it ships; anything outside it does not.
CI enforces this — the release job extracts the built package and `diff -r`s it
against that tree.

## Local checks

There is no build step, package manager, or lockfile. Run what CI runs:

```sh
bash tests/config-parity.sh    # defaults agree across engine, default.cfg, and the UI
bash tests/safe-paths.sh       # path-guard unit tests
bash tests/fleet-resolve.sh    # fleet config layering, derived names, firewall tags
bash -n <file>.sh              # every shell script must parse
php -l <file>.php              # every PHP file and .page containing <?php must parse
```

`tests/safe-paths.sh` needs GNU `realpath -m` and prints `SKIP:` on macOS. It
runs for real in CI, so a change to `crf_safe_cache_root` or
`crf_safe_mount_subdir` is not verified until CI runs it.

## Changing a config default

Defaults are maintained by hand in three places and drift is a CI failure:

1. `include/runner-farm.sh` — the `# ---- defaults` block, the runtime authority
2. `default.cfg` — operator-facing reference (never copied to flash)
3. `RunnerFarmSettings.page` — the `$defaults` array plus the form field

Add all three, or add the key to `ENGINE_ONLY_IN_CFG` in
`tests/config-parity.sh` if it is deliberately engine-only.

## Trying it on a real box

```sh
./build-plg.sh                 # writes ci-runner-farm.plg + ci-runner-farm.tgz
./deploy.sh root@tower         # fast iteration on a dev Unraid host
```

`deploy.sh` copies a hand-listed subset of files, not the whole tree. Install the
built `.plg` to test anything it does not cover. Do not commit a locally built
`.plg` — it is generated during the release and is only byte-reproducible under
GNU tar.

## Commits and pull requests

- [Conventional Commits](https://www.conventionalcommits.org). Release automation
  parses them, so the type is load-bearing: `feat:` → minor, `fix:` → patch,
  `feat!:` or `BREAKING CHANGE:` → major. `docs:`, `chore:`, `refactor:`, `test:`
  do not release. Scope where it helps (`fix(autoscale):`, `feat(ui):`).
- PRs are squash-merged, so the **PR title** becomes the released commit message.
  Get that right even if the branch history is messy.
- Keep PRs focused. One behavioural change per PR.
- Say how you tested. "Installed the `.plg` on 6.12.10 and scaled to 4 runners" is
  worth more than a green CI badge here, because CI cannot exercise Unraid.

## Things to be careful with

This plugin runs as root on other people's servers, manages Docker, and handles
GitHub tokens. Extra care around:

- The install and remove `<INLINE>` scripts in `build-plg.sh` — they execute as
  root on every install.
- Anything feeding `rm -rf`, `chown -R`, or a bind mount. Web-settable paths must
  pass the `crf_safe_*` guards.
- Secrets. Tokens live in `/boot/config/plugins/ci-runner-farm/{token,registry-token}`
  at mode 600, never in the `.cfg` and never in logs.
- `include/exec.php` — every request is CSRF-checked. Keep it that way.
- The `nchan` channel publishes aggregate counts only; `/sub` is unauthenticated
  on the LAN.

Releases are automated end to end — see [docs/RELEASING.md](docs/RELEASING.md).
