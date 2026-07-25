# Source and attribution

Everything under `docs/` and `validation/` is vendored from
**[mstrhakr/plugin-docs](https://github.com/mstrhakr/plugin-docs)**, the community-maintained
Unraid plugin development documentation, published at
[plugin-docs.mstrhakr.com](https://plugin-docs.mstrhakr.com).

**License: MIT** — Copyright (c) 2026 Unraid Plugin Documentation Contributors.
Full text in `LICENSE.plugin-docs`, retained unmodified as the license requires.

Note this differs from the license of the repository these files sit in
(ci-runner-farm is Apache-2.0). The MIT terms govern `docs/` and `validation/` only.

## Modifications

The copy is verbatim except for one mechanical rewrite: Jekyll `{% link docs/foo.md %}` tags
are replaced with plain relative paths (`docs/foo.md`) so cross-references resolve as real
files when read off disk instead of through the Jekyll site build.

Synced from upstream commit `23496fd` (2026-05-05). Re-sync with `../scripts/update-docs.sh`.

## Disclaimer, carried over from upstream

This documentation is unofficial and community-maintained. Verify critical details against
official sources and against a live server before relying on them.

Unraid® is a registered trademark of Lime Technology, Inc. Neither this skill nor the upstream
documentation is affiliated with, endorsed by, or sponsored by Lime Technology, Inc.
