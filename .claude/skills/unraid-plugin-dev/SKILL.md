---
name: unraid-plugin-dev
description: Reference and best practices for building Unraid OS plugins - .plg installer XML, .page files, the Dynamix webGUI, CSRF tokens, event hooks, .cfg settings storage, nchan/WebSocket, dashboard tiles, txz packaging and Community Applications distribution. Use this skill whenever the work touches an Unraid plugin, an Unraid webGUI page, /usr/local/emhttp, /boot/config/plugins, a .plg or .page file, an emhttp event handler, or the Dynamix framework - including when the user only says "my Unraid plugin", "the plugin page", "the settings tab", or names a plugin repo, and does not explicitly ask for docs. Also use before reviewing or shipping plugin changes, since most Unraid plugin bugs are convention violations this reference catches.
---

# Unraid plugin development

Unraid has no official plugin SDK and no official developer docs. Almost everything is
convention, learned from reverse-engineering system plugins. That is why this skill exists:
guessing at an Unraid convention looks fine locally and breaks on a live server, usually at
array-start or after a reboot when nothing is watching.

`references/` is a full offline copy of the community documentation at
[plugin-docs.mstrhakr.com](https://plugin-docs.mstrhakr.com), much of it validated against a
live Unraid 7.2.3 box. Read the specific file you need before writing code — the rules below
are the shape of the thing, not a substitute for the detail.

Those docs are the work of the **[mstrhakr/plugin-docs](https://github.com/mstrhakr/plugin-docs)**
contributors, vendored here under the MIT License — see `references/SOURCE.md` and
`references/LICENSE.plugin-docs`. Credit them when their material informs an answer, and point
people upstream rather than pasting large excerpts. They are unofficial and not affiliated with
Lime Technology; Unraid® is a Lime Technology trademark.

## The five that actually bite

These cause most real plugin bugs. Check them on every change, even when the user asked
about something else.

**1. RAM vs flash.** Unraid boots into RAM from USB. `/usr/local/emhttp/plugins/<name>/` is
rebuilt from the `.txz` every boot — anything written there is gone after a reboot.
`/boot/config/plugins/<name>/` is the USB stick and survives, but it is a flash drive: writing
caches, state, or logs there on a timer wears it out and is the classic plugin sin. Persistent
config on flash, hot/volatile state in tmpfs (`/var/local/emhttp/<name>/`).

**2. CSRF on every POST.** Unraid rejects POSTs without a valid token, and a PHP endpoint that
does not check it is the plugin's main attack surface — it runs as root inside the webGUI.
Emit `$var['csrf_token']` as a hidden field or AJAX param, and validate it server-side with
`hash_equals()` against the token in `/var/local/emhttp/var.ini`. `references/docs/core/csrf-tokens.md`.

**3. Executable bit and LF line endings on shipped scripts.** Event handlers and `rc.d` scripts
need `chmod 755`; a CRLF anywhere gives `/bin/bash^M: bad interpreter` at runtime, not build
time. Fix both in the build script, not by hand. `references/docs/advanced/debugging-techniques.md`.

**4. Scope your CSS and JS selectors.** Plugin pages share a DOM and a jQuery context with the
parent Unraid page. Unscoped selectors like `.advanced`, `.updatecolumn`, `tr.sortable` collide
with Docker/VM tabs and reinitialize each other's widgets. Namespace everything with a plugin
prefix. `references/docs/ui/tab-pages.md`, `references/docs/ui/javascript-patterns.md`.

**5. Unraid 7.2 broke the old DOM.** The webGUI was refactored for responsive layout. Title-bar
injection no longer works, dashboard tile structure changed, and DOM hacks written for 6.x
silently render wrong. Check `references/docs/ui/responsive-design.md` before touching layout,
and set `min=` on `<PLUGIN>` honestly.

## Where to look

Read one file, not the whole tree. Everything below is under `references/`.

| Need | File |
| --- | --- |
| What a plugin *is*, install flow | `docs/introduction.md` |
| First plugin, end to end | `docs/getting-started.md` |
| Which directory, and does it survive reboot | `docs/filesystem.md` |
| `.plg` XML: entities, `<FILE>`, `<INLINE>`, install/remove | `docs/plg-file.md` |
| `.page` headers, `Menu=`, `Cond=`, form markup | `docs/page-files.md` |
| Event hooks and firing order | `docs/events.md`, `docs/reference/event-types-reference.md` |
| `$Dynamix`, themes, shared globals | `docs/core/dynamix-framework.md` |
| `.cfg` files, `parse_plugin_cfg()`, defaults | `docs/core/plugin-settings-storage.md` |
| CSRF | `docs/core/csrf-tokens.md` |
| Live updates without polling | `docs/core/nchan-websocket.md` |
| User-facing alerts | `docs/core/notifications-system.md` |
| Scheduled work | `docs/core/cron-jobs.md` |
| Daemons / services | `docs/core/rc-d-scripts.md` |
| Translations | `docs/core/multi-language-support.md` |
| Form controls, switches, selects | `docs/ui/form-controls.md` |
| jQuery/AJAX conventions | `docs/ui/javascript-patterns.md` |
| Icons, CSS vars, buttons | `docs/ui/icons-and-styling.md` |
| Multi-tab pages | `docs/ui/tab-pages.md` |
| Dashboard tiles | `docs/ui/dashboard-tiles.md` |
| 7.2 responsive migration | `docs/ui/responsive-design.md` |
| Container control from a plugin | `docs/advanced/docker-integration.md` |
| Disks, shares, spin-up avoidance | `docs/advanced/array-disk-access.md` |
| Input sanitizing | `docs/security/input-validation.md` |
| Ownership and modes | `docs/security/file-permissions.md` |
| Failing gracefully | `docs/security/error-handling.md` |
| `.txz` build, MD5, CI release | `docs/build-and-packaging.md` |
| `plugin` CLI (install/update/remove/checkall) | `docs/plugin-command.md` |
| Getting into Community Applications | `docs/distribution/community-applications.md` |
| Hosting the `.plg` and support thread | `docs/distribution/hosting.md`, `docs/distribution/support.md` |
| Real plugins worth copying from | `docs/examples.md` |
| Exact paths / `$var` keys / PHP helpers | `docs/reference/` |

Thin stubs — skim, then fall back to reading a real plugin: `docs/advanced/testing.md`,
`docs/advanced/update-mechanisms.md`, `docs/advanced/package-management.md`,
`docs/advanced/user-scripts-integration.md`.

## Ground truth beats the docs

These docs are community-maintained and explicitly unofficial. When something is load-bearing
and you are not certain, verify instead of asserting:

- `references/validation/results/unraid-7.2.3.md` — 74 claims checked against a live 7.2.3
  server (events, paths, `$var` keys, PHP helpers). If a fact is in here, it is real.
- `references/validation/scripts/` — runnable `validate-*.sh` checks. Copy one to a server to
  confirm a path, event, or `$var` key exists on *that* Unraid version.
- `references/validation/plugin/source/emhttp/` — a small working plugin: `.page` file,
  `default.cfg`, all 16 event handlers, and `build.sh`. The fastest correct answer to "what
  should this file look like" is usually here.
- The installed plugins on a real box (`/usr/local/emhttp/plugins/dynamix*/`) are the actual
  spec. Unraid's own code wins over any doc.

Pages with a `✅ Validated against Unraid 7.2.3` note near the top are verified; pages without
one are somebody's best understanding.

## Writing a plugin

Follow the existing repo's conventions first — a project `CLAUDE.md` or an established layout
outranks anything here. For greenfield work the shape is:

1. Source tree that mirrors the target filesystem byte for byte, so the build is `tar`, not
   a copy script with logic in it.
2. `build.sh` producing `<name>-<version>.txz` plus its MD5, with LF conversion and `chmod`
   baked in. `docs/build-and-packaging.md`.
3. A `.plg` that downloads the `.txz` by URL and verifies the MD5 — do not embed the payload.
4. Version as `YYYY.MM.DD`, matching the `.plg`, the `.txz` name, and the release tag.
5. Any non-zero exit from an `<INLINE>` block aborts the install. Handle non-fatal failures
   and exit 0 deliberately.

## Reviewing plugin changes

Beyond the five above: secrets in `/boot/config/plugins/<name>/` with mode 600 and never in a
`.cfg` that the UI renders; user input reaching a shell command must be escaped
(`docs/security/input-validation.md`); defaults duplicated between shell, `.cfg`, and PHP need
a parity check or they will drift; disk reads that spin up idle array drives should be avoided
or cached (`docs/advanced/array-disk-access.md`).

## Refreshing this reference

`scripts/update-docs.sh` re-syncs `references/` from upstream. Run it if a doc looks stale or
the user mentions a newer Unraid release. It preserves `LICENSE.plugin-docs`; keep it that way.

## Attribution

The reference material here is not ours. It is the community documentation project
[mstrhakr/plugin-docs](https://github.com/mstrhakr/plugin-docs), used under the MIT License,
Copyright (c) 2026 Unraid Plugin Documentation Contributors. The license text travels with the
docs in `references/LICENSE.plugin-docs` and must stay there — MIT requires the copyright
notice be retained in copies, which is exactly what `references/` is. Provenance, the list of
modifications, and the upstream disclaimer are in `references/SOURCE.md`.

The surrounding repository is Apache-2.0; that does not extend over `references/`.
