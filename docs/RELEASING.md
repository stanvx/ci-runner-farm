# Releasing

Releases are fully automated. Nobody tags, builds, or uploads by hand. The only
human actions are **writing conventional commits** and **merging the release
PR**.

## Source of truth

`.release-please-manifest.json` holds the SemVer version. Everything else is
derived from it:

| File | Owner | Notes |
| --- | --- | --- |
| `.release-please-manifest.json` | release-please | the version |
| `CHANGELOG.md` | release-please | newest section is copied into the `.plg` `<CHANGES>` block |
| `VERSION` | `prepare-plugin-release-pr` job | mirror for tooling |
| `ci-runner-farm.plg` | `prepare-plugin-release-pr` job | committed, version-stamped |
| `ci-runner-farm.tgz` | never committed | rebuilt reproducibly at validate + publish |
| `crf-scalesetd` | never committed | Go listener, rebuilt reproducibly at validate + publish |

**Never hand-edit `VERSION` or `ci-runner-farm.plg`.** A release-PR job
overwrites both, and `release.yml` fails the release if they disagree with the
tag. To change a version, change the commit types you merge — not these files.

## The flow

1. **Merge conventional commits to `main`.** `feat:` → minor, `fix:` → patch,
   `feat!:` / `BREAKING CHANGE:` → major. Anything else (`docs:`, `chore:`,
   `refactor:`, `test:`) does not move the version. Squash-merge, so the squashed
   subject is what release-please parses — a good PR body with a bad subject
   produces a wrong bump.
2. **release-please opens or updates the release PR** (`release-please.yml`),
   bumping the manifest and `CHANGELOG.md`.
3. **`prepare-plugin-release-pr` commits the derived metadata onto that PR**: it
   reads the manifest, writes `VERSION`, runs `INTERNAL_VERSION=$version
   ./build-plg.sh`, and pushes `VERSION` + `ci-runner-farm.plg` as
   `chore(release): prepare plugin metadata`.
4. **Review the release PR** — see the checklist below.
5. **Merge it.** release-please tags `vX.Y.Z` and cuts the GitHub Release.
6. **`validate-plugin-release`** runs `release.yml` against the tag. It refuses
   the release unless every invariant below holds.
7. **`publish-plugin-release`** rebuilds the `.tgz` and `crf-scalesetd`, re-checks
   both MD5s against the committed `.plg`, and uploads `ci-runner-farm.plg`, the
   version-pinned `ci-runner-farm-<version>.tgz`, and the version-pinned
   `crf-scalesetd-<version>` to the Release.

## Invariants enforced by `release.yml`

A release cannot publish unless all of these hold at the tag:

- tag matches `^v[0-9]+\.[0-9]+\.[0-9]+(-…)?$`
- `<!ENTITY pluginVersion>` == the tag without its leading `v`
- `<!ENTITY releaseTag>` == the tag
- `<!ENTITY version>` matches `YYYY.MM.DD.HHMM.BUILD-<semver>` and ends in that exact SemVer
- `VERSION` == the tag without its leading `v`
- the `.plg` parses as XML
- the `.tgz` rebuilt from the tagged `src/` has the MD5 the committed `.plg` advertises
- `diff -r` of that `.tgz` against `src/usr/local/emhttp/plugins/ci-runner-farm` is clean
- `crf-scalesetd` rebuilt from the tagged `daemon/` has the MD5 the committed `.plg` advertises

The last three are the important ones: together they prove the published `.plg`
fetches a package and a binary whose bytes are exactly the tagged source. They
only hold because `build-plg.sh` builds reproducibly — pinned `--sort=name`,
`--mtime`, ownership, and `gzip -9n`. Those flags are applied **only under GNU
tar**, so a `.plg` built on macOS must never be committed. Let CI build it.

## The two release assets

Neither build output is committed and neither lives in `src/`. Both ship as
GitHub Release assets that the `.plg` fetches by URL and verifies by MD5 — the
standard Unraid `<FILE>` pattern, used twice:

| asset | entity pair | installed to |
| --- | --- | --- |
| `ci-runner-farm-<version>.tgz` | `packageURL` / `packageMD5` | extracted over `/usr/local/emhttp/plugins/ci-runner-farm` |
| `crf-scalesetd-<version>` | `daemonURL` / `daemonMD5` | `$PLGDIR/bin/crf-scalesetd`, mode 0755 |

The listener is a compiled binary, so keeping it out of the tarball is what lets
`src/` stay the text-only image `diff -r` checks and keeps git binary-free. It
gets its own `install -m 0755` in the `.plg` because the install block's blanket
`chmod 0644` pass only walks what the tarball extracted — without it the plugin
would ship a non-executable daemon.

Both are built by `build-plg.sh` (`--tgz-only` / `--daemon-only` rebuild one
each). The listener is built
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w -buildid='`
for exactly one target — Unraid 6.12+ is x86_64 and the daemon only shells out to
`runner-farm.sh` and speaks HTTPS, so it needs no libc and no build matrix. The
toolchain is pinned by the `toolchain` directive in `daemon/go.mod` plus a
SHA-pinned `actions/setup-go`.

`package-plugins.yml`'s `daemon` job proves reproducibility directly: it builds
the listener, rebuilds it from a different absolute path with a cold build cache,
and `cmp`s the two. Carrying the binary between workflow runs as an artifact was
rejected — that is exactly the shortcut by which a `.tgz` ends up committed.

Dependabot watches `daemon/` but **ignores `github.com/actions/scaleset`**: it is
a public-preview module whose interfaces move, so an automated bump lands a PR
that does not compile. Upgrade it deliberately, in a PR that touches code.

## Reviewing the release PR

- The version bump matches the intent of the merged commits.
- `CHANGELOG.md`'s newest section is accurate — it is verbatim what Unraid's
  plugin manager shows in `<CHANGES>`.
- The diff to `ci-runner-farm.plg` is version entities only. Any change to the
  install/remove `<INLINE>` scripts means a source change rode along; those
  scripts run as root on every user's server. `packageMD5` and `daemonMD5` move
  on every release by design — they are content hashes, not version stamps.
- If `default.cfg`, the engine defaults, `include/crf-config.php` or
  `include/crf-fields.php` changed,
  `config-parity` passed (see `/add-config-key`).
- `pluginURL` / `packageURL` point at the repo you intend to publish from
  (`build-plg.sh` takes `REPO` from `${{ github.repository }}` in CI).

Note that the metadata commit is pushed with `GITHUB_TOKEN`, and GitHub does not
re-trigger workflows for token-authored pushes. **The `VERSION` + `.plg` commit is
therefore not covered by the PR's own CI run** — `release.yml` re-checks it after
tagging instead, but review that diff rather than trusting a green PR.

## Fork releases

This repository is `stanvx/ci-runner-farm`, a fork. Releases are distributed by
URL from this repo's GitHub Releases only — Community Applications is not part of
this fork's current distribution. The optional `community-applications/` metadata
also points at this fork, but does not make the plugin CA-listed.

`REPO` defaults to `${{ github.repository }}` in CI, so releases cut here build
`stanvx` URLs automatically. Local `build-plg.sh` builds use the same fork by
default; set `REPO=unraid/ci-runner-farm` only when intentionally building the
upstream repository.

The committed `.plg` must point at `stanvx/ci-runner-farm` before publishing. The
release job regenerates those URLs from `github.repository`; inspect them on the
release PR rather than assuming a fork inherited the correct owner.

Two repository settings must be enabled on a fork before release automation works:

- **Settings → Actions → General → Workflow permissions → Allow GitHub Actions to
  create and approve pull requests.** Without it release-please cannot open the
  release PR.
- **Read and write permissions** for the workflow token, so the metadata commit
  can be pushed.

`UNRAID_BOT_GITHUB_ADMIN_TOKEN` is optional; the workflows fall back to
`github.token`.

## Recovering from a bad release

Do not delete or move a published tag — installed plugins resolve
`releases/latest/download/ci-runner-farm.plg`, and a rewritten tag breaks MD5
verification on machines that already downloaded the package. Ship a forward fix:
merge a `fix:` commit and let the next release supersede it.

If validation fails after tagging, the Release exists but has no assets attached,
so nothing is installable yet. Fix `main`, then re-run `release.yml` via
`workflow_dispatch` with the tag, or cut a new patch release.
