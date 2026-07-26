# Community Applications listing

Optional metadata for listing **CI Runner Farm** on Unraid
[Community Applications](https://docs.unraid.net/unraid-os/using-unraid-to/run-docker-containers/community-applications/)
(CA). CA scrapes a maintainer's template repo: one `*.xml` per plugin plus a
single `ca_profile.xml`. These files are served raw from this repo if the fork
is submitted to CA; they are not required for the direct plugin install.

## Files here

| File | Purpose |
|---|---|
| [`ci-runner-farm.xml`](ci-runner-farm.xml) | CA plugin template — name, description, category, icon, `PluginURL`, support/project links. |
| [`ca_profile.xml`](ca_profile.xml) | Maintainer profile shown next to the listing. |
| [`ci-runner-farm.png`](ci-runner-farm.png) | 256×256 listing icon (source: [`ci-runner-farm.svg`](ci-runner-farm.svg)). |
| [`DESCRIPTION.md`](DESCRIPTION.md) | Copy for the CA listing and the forum support thread. |

`PluginURL` points at this fork's `releases/latest/download/ci-runner-farm.plg`,
so CA would install the newest release from `stanvx/ci-runner-farm`. Unraid's
"check for updates" resolves from the same URL.

## Prerequisites (must be true before CA can list this)

1. **This fork must be public.** CA fetches the `.plg`, the template XML, the
   icon, and the screenshot over unauthenticated HTTPS. Every URL below targets
   `stanvx/ci-runner-farm`; nothing works until that repository is public.
2. **At least one published GitHub Release** so `releases/latest/download/…`
   resolves. release-please cuts these; confirm the `.plg` asset is attached.
3. **A dedicated support thread on the Unraid forums.** CA submissions require it.
   Create the thread, then:
   - set `<Support>` in `ci-runner-farm.xml` to the thread URL (currently the
     GitHub issues URL as a stand-in), and
   - set `<Forum>` in `ca_profile.xml` to the maintainer's forums.unraid.net
     profile (currently a placeholder — see the TODO comment).

## Submit

Use the Community Applications submission flow
(https://unraid.net/community/apps — "Submit"). It parses the template XML,
validates `ca_profile.xml`, checks for duplicates, and previews the listing.
Point it at the raw URLs:

- Template: `https://raw.githubusercontent.com/stanvx/ci-runner-farm/main/community-applications/ci-runner-farm.xml`
- Profile:  `https://raw.githubusercontent.com/stanvx/ci-runner-farm/main/community-applications/ca_profile.xml`

The CA moderation team then vets it for security, functionality, and design
before it goes live.

## Regenerating the icon

```bash
rsvg-convert -w 256 -h 256 ci-runner-farm.svg -o ci-runner-farm.png
```

## Pre-submission checklist

- [ ] Repo is **public**
- [ ] A GitHub Release exists with `ci-runner-farm.plg` attached
- [ ] Installed the released `.plg` on a clean Unraid box and verified it works
- [ ] Forum support thread created; `<Support>` + `<Forum>` updated to real URLs
- [ ] Raw URLs for the template, profile, icon, and screenshot all load in a browser
- [ ] Submitted via the CA portal and passed the preview/validation
