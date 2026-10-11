# RunnerFarm durability uplift — 11 October 2026

## Decision

Keep the fork's multi-fleet GitHub scale-set architecture and backport tested upstream fixes. Do not replace this installation with upstream's provider/pool implementation: the two projects share the plugin identity but have incompatible configuration and lifecycle assumptions. Prefer GitHub-hosted runners for ordinary CI if minimum operational work is the deciding factor; retain Unraid where private network, hardware or local caches justify owning host capacity.

This is an evidence-backed plan and implementation record. Local tests, deployed runtime, reboot persistence and real workflow acceptance are separate gates.

## 1. Recover host capacity and preserve emulator data

Observed: host 78.31 GiB RAM; approximately 65 GiB used, 12–13 GiB available. The 16 GiB zram device was full and its compressed allocator used about 5.21 GiB physical RAM. Short samples showed little swap traffic, so fullness alone was not evidence of thrashing. The emulator explicitly requested 10 GiB despite a saved 2 GiB AVD setting; its guest had nearly 8 GiB available. Ubuntu and Home Assistant VMs accounted for approximately 18 GiB resident memory; their guest memory samples were stale, so shrinking them is not justified.

Actions: reduce emulator guest to 4 GiB, preserve its actual AVD outside the container's writable layer, make launch arguments and resource limits persistent, and verify app data plus repeated restart. Keep zram as a host safety buffer rather than running swapoff on a full device. Set and verify actual cgroup swap ceilings for CI and emulator: Docker advertises no swap-limit support on this host and silently ignores the desired Compose/CLI setting. Retain the desired Docker limit for hosts that do support it; use explicit writable cgroup-v2 enforcement here.

Acceptance: emulator boots with existing ScribeKey data, restart preserves it, host headroom admits a build, kernel swap controls show the intended values, no new cgroup OOM kills in the validation window. Full-swap history may remain until cold workloads release pages; this is not a reason to force all pages back into RAM.

## 2. Make RunnerFarm recover without manual tuning

Recover missing live hotfixes into tracked source: reconcile GitHub `TotalAssignedJobs` on reconnect and long-poll timeout; count existing containers before provisioning; isolate JIT secret directories by fleet; recover hard engine failure only after successful inert validation. Advertise total scale-set capacity, not spare slots, and do not create idle single-use JIT runners.

Admit jobs under the shared host lock using finite per-runner memory caps and a 2 GiB host reserve. Count only the unused part of existing runner caps against `MemAvailable`; already-resident pages must not be charged twice. Reserve conservatively where kernel usage cannot be read. Refuse provisioning if swap policy cannot be enforced. Use SSD-backed workspaces, max-one build runner and measured JVM limits already present in ScribeKey CI.

Add minute-level process recovery through Unraid's plugin cron integration. Persist explicit Stop intent so cron and reboot cannot resurrect an intentionally stopped fleet. Validate PID identity before signalling, preserve jobs during config/drain operations, and bound runtime/boot logs. Distinguish authenticated readiness, waiting for memory, and a stopped fleet in the UI.

Acceptance: regression coverage for missed/repeated assignments, fleet secrets, memory boundaries, reused PIDs, explicit stop, false validation success and UI state. Live fault injection must demonstrate recovery and respect Stop.

## 3. Adopt useful upstream changes and make releases durable

Audit refs: fork `stanvx/ci-runner-farm`, upstream `unraid/ci-runner-farm` v1.10.3. At audit the branches diverged by 18 fork commits and 59 upstream commits. Scale-set SDK v0.4.0 is already the latest stable version. Installed runner was verified at v2.338.0; do not rebuild large toolchains solely to chase an already-installed runner version.

Selected upstream adaptations:

- Numeric configuration validation: [d24af37 / #57](https://github.com/unraid/ci-runner-farm/pull/57).
- Credential-safe response serialization: [19f4796 / #59](https://github.com/unraid/ci-runner-farm/pull/59).
- Validation prerequisite failures: [7bc43ac / #93](https://github.com/unraid/ci-runner-farm/pull/93).
- Bounded persistent boot logs: [bf42747 / #90](https://github.com/unraid/ci-runner-farm/pull/90).
- Publication safety: [#84](https://github.com/unraid/ci-runner-farm/pull/84), adapted to preserve the fork's separate Go daemon payload.

DinD image readiness must fail after its deadline rather than letting a runner accept jobs without Docker. Android SDK license acceptance must tolerate the producer's SIGPIPE without masking actual sdkmanager failure. Preserve existing saved toolchain selections when migrating the Dockerfile, then build and verify the replacement image. Raw deploy must include the daemon but is explicitly temporary; the published plugin plus both version-pinned payloads is the reboot-persistent delivery.

Release acceptance: source checks; immutable tag/manifest; deterministic tagged builds; XML and both payload checksum checks; source/package equality; publish and anonymously verify payloads before exposing the PLG; install that PLG through Unraid, then compare actual files/binary with release payloads. Do not hand-edit generated version metadata or rewrite published tags.

## 4. Prove useful CI and separate unrelated failures

See [CI failure evidence](ci-failure-evidence-2026-10-11.md) for exact run IDs and [Unraid memory evidence](unraid-memory-evidence-2026-10-11.md) for host attribution and changes. At audit, 37 queued runs were Pre Merge Checks; dependency submission was successfully using GitHub-hosted `ubuntu-latest`, despite older docs claiming a dedicated self-hosted fleet. Do not delete/cancel unrelated runs as a capacity workaround.

Track assignment, registered runner, exact deployed limits/image/toolchain, cgroup events and a completed real ScribeKey build. A fresh run on a unique verification branch avoids replacing unrelated pending concurrency groups. The existing PR #1135 run is a useful separate acceptance target. Linux screenshot checks are report-only; a green build does not establish screenshot or physical-device acceptance.

Historical classes include genuine Java/container OOM kills, queue starvation, account billing blocks, incomplete NDK installation, Linux screenshot differences and an abrupt cache-restore termination with unproven cause. Neither blanket retries nor larger Java heaps resolves all of these.

## 5. Keep management simple

Use existing Fleet/Runner image controls to adopt tools rather than adding another package manager or orchestrator. Continue pinning the runner SDK to deliberate upgrades and keep one GitHub release install path. Keep credentials in the existing private files and the user-controlled token field.

For ordinary CI, a hosted 4-core/16 GiB Linux larger runner matches this build's memory budget more closely than standard private 2-core/8 GiB Ubuntu. At the audited official rates, a 20-minute run is $0.24 on the 4-core larger runner, or $0.12 on standard Linux before standard included allowances. Larger runners require eligible Team/Enterprise billing and are always billed. Cloud removes this host's capacity, LAN, image lifecycle and credential-maintenance burden, but Android toolchain pinning and source/screenshot failures remain. See the evidence document for current primary links and eligibility details. No billing or paid runner provisioning is part of this deployment.

Storage cleanup candidates are documented, not indiscriminately pruned: roughly 5.33 GB build cache and 6.54 GB unused images require reference checks and confirmation before irreversible deletion. Do not remove volumes, VM disks, snapshots, AVD data or shared caches to make the dashboard look greener. The emulator AVD relocation fixes a concrete persistence risk; unrelated storage moves need their own verified source/target/rollback.
