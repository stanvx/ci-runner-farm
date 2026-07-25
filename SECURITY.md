# Security Policy

## Reporting a vulnerability

Please report security issues privately, not as a public issue.

Use GitHub's private vulnerability reporting: **Security → Report a
vulnerability** on <https://github.com/stanvx/ci-runner-farm/security/advisories/new>.

Include the plugin version (Settings → Utilities → CI Runner Farm shows it, or
read `<!ENTITY pluginVersion>` in the installed `.plg`), your Unraid version, and
enough detail to reproduce.

This is a fork. If the issue also affects
[unraid/ci-runner-farm](https://github.com/unraid/ci-runner-farm), please report
it upstream as well — fixes here do not reach upstream users.

## Supported versions

Only the latest release is supported. Fixes ship forward as a new release rather
than as patches to older tags.

## Scope

The plugin installs and runs as root on an Unraid server, manages Docker
containers, and stores a GitHub personal access token. Reports about any of the
following are in scope:

- Command injection or privilege escalation via the web UI, `include/exec.php`,
  or `include/runner-farm.sh`
- CSRF or authentication bypass on the plugin's endpoints
- Token or credential disclosure — in logs, the config file, the UI, container
  environments, or the `nchan` push channel
- Path traversal or destructive filesystem operations reachable from
  web-settable values
- Escape from a runner container to the host beyond what the configured
  isolation settings imply

Known and documented by design, not vulnerabilities on their own:

- The `nchan` channel `/sub` endpoint is unauthenticated on the LAN. It carries
  only aggregate counts (`count`, `up`, `busy`, `idle`) for that reason.
- `SHARE_DOCKER_SOCK=true` and `RUN_AS_ROOT=true` deliberately weaken isolation.
  Both default to `false`. Reports amounting to "enabling this is unsafe" are
  expected behaviour; reports that these settings fail to isolate when left at
  their defaults are not.
- Self-hosted runners executing workflow code from repositories you grant them
  is the entire purpose of the tool. Use them only with trusted repositories —
  see GitHub's own guidance on self-hosted runners and public repositories.
