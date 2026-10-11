# ScribeKey CI failure evidence and runner choices — 11 October 2026

At the 17:10 AEDT audit, the ScribeKey queue was waiting for self-hosted provisioning. Earlier failures also include screenshot baselines, terminated Java processes, hosted billing, and Android toolchain installation. Changing one runner setting cannot resolve every category. Later deployment and live acceptance evidence are recorded below and in [the deployment report](runnerfarm-deployment-2026-10-11.md).

Read-only GitHub API and log audit, refreshed at 2026-10-11 06:10 UTC / 17:10 AEDT. This document records an observation, not continuing service health. No jobs were cancelled, rerun, or dispatched during this audit.

## Audit snapshot: 17:10 AEDT

| Item | Verified result |
| --- | --- |
| [ScribeKey PR #1135](https://github.com/scribekey-app/scribekey/pull/1135) | Open, branch `settings/1107-integration`, head `f1ae65706ad7d07e14c5cb9689cf89336b687f73`. |
| [Pre-merge 38116581616](https://github.com/scribekey-app/scribekey/actions/runs/38116581616) | Queued; job `114402557341` requests `unraid-builders`, with no assigned runner name. |
| [Dependency submission 38116577654](https://github.com/scribekey-app/scribekey/actions/runs/38116577654) | Successful on `ubuntu-latest`, runner group `GitHub Actions`, not the Unraid dependency fleet. |
| GitHub runner inventory | One registered runner, `unraid-builders-3174bede6d50`, offline and not busy. Zero online runners in the snapshot. |
| GitHub queued runs | 37 queued runs, all `Pre Merge Checks`. The oldest is [37642532291](https://github.com/scribekey-app/scribekey/actions/runs/37642532291), created 7 October. This is not the historical generated dependency-submission backlog. |
| [RunnerFarm PR #29](https://github.com/stanvx/ci-runner-farm/pull/29) | Merged at 2026-10-11 05:58:53 UTC. Fresh [Build Plugin 38116519244](https://github.com/stanvx/ci-runner-farm/actions/runs/38116519244) and [Lint 38116519268](https://github.com/stanvx/ci-runner-farm/actions/runs/38116519268) succeeded. |
| [RunnerFarm release PR #30](https://github.com/stanvx/ci-runner-farm/pull/30) | Open for 1.9.3 at audit time. Merged source and passing package CI do not establish a published or deployed release. |

ScribeKey's `docs/ci.md` says dependency submission uses the dedicated `dependency-submission` scale set, but the live generated job above used GitHub hosting. Treat this as documentation/routing drift. Changing checked-in build YAML alone does not change GitHub's generated dependency workflow.

## Failure categories and evidence

| Category | Evidence | Interpretation and smallest appropriate response |
| --- | --- | --- |
| Queue/provisioning | At 17:10, PR #1135 run above had no runner; the audit inventory had no online runner. | Inspect listener assignment, admission decision, image availability, Docker creation and runner registration. Authentication or an inert validation pass alone is insufficient. |
| Screenshot baseline failures | [38022496249](https://github.com/scribekey-app/scribekey/actions/runs/38022496249) failed `:app:validateDebugScreenshotTest`, reporting failing tests. | Review actual/diff artifacts against Mac output. Adding RAM cannot repair stale baselines or renderer differences. |
| Terminated screenshot process | [38019332610](https://github.com/scribekey-app/scribekey/actions/runs/38019332610) lost `Gradle Test Executor 1` with exit 137/SIGKILL. | Correlate kernel/cgroup OOM evidence at the failure timestamp. Exit 137 suggests termination, but does not independently establish OOM. |
| Missing Gradle daemon | [37912098226](https://github.com/scribekey-app/scribekey/actions/runs/37912098226), [37994209034](https://github.com/scribekey-app/scribekey/actions/runs/37994209034) report the daemon disappeared unexpectedly. | Historical host investigation found Java cgroup OOM kills; later failures still require individual correlation. Preserve counters/logs before ephemeral containers disappear. |
| Interrupted action/cache setup | [38083235704](https://github.com/scribekey-app/scribekey/actions/runs/38083235704) failed `Setup Gradle` before compilation, during several cache restorations totaling roughly 2.4 GB compressed. | The log ends during restoration, with no diagnostic annotation. Cause remains unproven. Inspect container termination, disk and host logs instead of assuming app compilation or OOM. |
| Hosted billing admission | [37878633284](https://github.com/scribekey-app/scribekey/actions/runs/37878633284) never started; its check annotation says recent account payments failed or the spending limit needs increasing. | Account availability, not source failure. Recent successful hosted dependency jobs mean this historical message cannot be carried forward as the current blanket blocker. |
| Hosted Android toolchain | [38008778321](https://github.com/scribekey-app/scribekey/actions/runs/38008778321) failed configuring `:app`: NDK `27.0.12077973` installation failed and its `source.properties` was absent. | Install and verify the exact toolchain before Gradle configuration. A hosted image's preinstalled Android tools are not an exact project toolchain contract. |

### What a green pre-merge result now means

[38085091785](https://github.com/scribekey-app/scribekey/actions/runs/38085091785) completed successfully on `unraid-builders-3289fb112027` at 2026-10-10 21:24:57 UTC. Compile/build, unit tests, detekt, Android-test compilation and native-package validation succeeded. Its screenshot command still failed and uploaded a roughly 78 MB `screenshot-diffs` artifact; `continue-on-error` allowed the job to succeed.

[PR #1022](https://github.com/scribekey-app/scribekey/pull/1022), linked to its Claude session, deliberately made Linux screenshots report-only. Its evidence distinguishes 74 previews failing on the Mac too from approximately 15 additional Linux renderer mismatches. The Mac baseline gate remains separate. PR CI also deliberately skips Android Lint; it runs on `main`, while the local Mac gate includes lint. Therefore a green PR job does not establish screenshot acceptance, Android Lint acceptance, device behavior, or release readiness.

[Issue #1021](https://github.com/scribekey-app/scribekey/issues/1021) records rescued memory/report changes and their reconciliation. Failed-test report upload already landed through #1022. Current pre-merge flags already cap Gradle at 3 GiB, Kotlin at 2 GiB and workers at two. Avoid duplicating rescued changes or lowering laptop defaults to repair a server resource problem.

## GitHub hosting compared with Unraid

ScribeKey is private. These figures are current official documentation, checked on 11 October 2026; prices are USD and exclude storage, tax and differences in build duration.

| Option | Resources | Execution cost and eligibility | Operational tradeoff |
| --- | --- | --- | --- |
| Standard hosted `ubuntu-24.04` | 2 CPUs, 8 GB RAM, 14 GB SSD | $0.006/minute after the owner's standard allowance. GitHub Free for organizations includes 2,000 minutes/month. | GitHub maintains the VM/image and autoscaling. Less RAM than either the original 12 GiB container or the deployed 16 GiB ceiling; benchmark before migration. |
| Hosted 4-core Ubuntu larger runner | 4 CPUs, 16 GB RAM, 150 GB SSD | $0.012/minute, always billed; included minutes do not apply. Requires GitHub Team or Enterprise Cloud organization and valid payment setup. | Conditional pilot after bounded-render validation: the measured 12.87 GiB container peak leaves less margin on a 16 GB VM that also runs its OS and services. Larger runner assignment can have a cold-start delay. |
| Hosted 8-core Ubuntu larger runner | 8 CPUs, 32 GB RAM, 300 GB SSD | $0.022/minute, always billed; same larger-runner eligibility and payment requirements. | Safer initial hosted pilot for the unchanged full screenshot matrix. Retain explicit Gradle/JVM caps and two workers rather than automatically spending all available RAM/concurrency. |
| Existing Unraid fleet | Original 4-CPU/12-GiB build runner; raised to 16 GiB with actual zero-swap policy during this deployment, live build started 18:04 AEDT | Self-hosted execution currently has no GitHub runner-minute charge. Hardware, electricity, storage and maintenance remain local costs. | Exact prebuilt Android image and local/private hardware access, with host capacity, image lifecycle, credentials and LAN availability owned locally. |

At an assumed 20-minute duration, standard hosted Linux is $0.12 per build beyond included minutes; a 4-core larger runner is $0.24 and an 8-core larger runner $0.44. At 100 such builds, that is $12, $24 or $44 before standard included allowance. These are arithmetic scenarios, not measured forecasts: the 2-core host can take longer, and cache/toolchain installation changes runtime.

Official sources: [hosted specifications](https://docs.github.com/en/actions/reference/runners/github-hosted-runners), [larger runner specifications and payment requirements](https://docs.github.com/en/actions/reference/runners/larger-runners), [runner prices and larger-runner eligibility](https://docs.github.com/en/billing/reference/actions-runner-pricing), [allowances and billing admission](https://docs.github.com/en/billing/concepts/product-billing/github-actions).

For minimal management, ordinary hosted CI removes more responsibilities than hardening a custom autoscaler. Retain Unraid where private devices, local hardware, or an exact persistent image justify it. Use the 32 GB hosted option for an initial unchanged-workload pilot; evaluate the 16 GB option after bounding renderer retention and measuring VM-wide headroom. Do not add automatic cross-provider retry/fallback until both environments can execute the same toolchain contract successfully.

GitHub's [self-hosted routing reference](https://docs.github.com/en/actions/reference/runners/self-hosted-runners) confirms jobs remain queued when no matching online runner exists, and can fail after 24 hours. It also identifies GitHub-hosted autoscaling as a low-maintenance alternative, ARC for Kubernetes environments, and the scale-set client for custom infrastructure. Introducing Kubernetes solely for this farm would add another system to maintain.

## Deterministic Android setup

Pin the hosted OS to `ubuntu-24.04` for a pilot. An annotation on historical run 37878633284 announces `ubuntu-latest` migration to Ubuntu 26 beginning 19 October 2026. An explicit OS version reduces unrelated image changes during acceptance.

The current ScribeKey workflow documents Platform 37, Build Tools 36.0.0, NDK 27, CMake 3.22.1 and Java 21; `app/build.gradle.kts` requires NDK `27.0.12077973`. Verify against the selected ScribeKey SHA before building an image. The common SDK installation contract is:

```sh
sdkmanager 'platform-tools' 'platforms;android-37' 'build-tools;36.0.0' \
  'ndk;27.0.12077973' 'cmake;3.22.1'
test -f "$ANDROID_SDK_ROOT/ndk/27.0.12077973/source.properties"
test -x "$ANDROID_SDK_ROOT/cmake/3.22.1/bin/cmake"
```

Accept SDK licenses through the image/setup process and fail on installation errors. Under Bash `pipefail`, an unguarded `yes | sdkmanager --licenses` can return SIGPIPE/141 from `yes`; handle that pipeline explicitly while preserving SDK manager failure detection. Build the self-hosted toolchain into a versioned image and validate required files before activating it. Hosted setup should provision the same packages before Gradle configuration. Keep native builds, test JVMs, cache extraction and screenshot rendering within the total container memory budget; Java heap sizes alone are not that budget.

Official Android reference: [sdkmanager package installation and licenses](https://developer.android.com/tools/sdkmanager). That page now marks `sdkmanager` deprecated in favor of `android sdk`; retain the existing image's working installer for this repair, and evaluate that migration separately against the same package/file checks. Pin the command-line tools version used to build the image too.

## Deployment acceptance without disturbing other queues

1. **Prefer the existing current-PR run.** Re-read PR #1135's head and checks, then inspect [38116581616](https://github.com/scribekey-app/scribekey/actions/runs/38116581616) if its SHA is still current. Observe queue assignment and container startup after the infrastructure deployment. Do not cancel, delete, force-cancel or blanket-rerun the other 37 runs.
2. **If a fresh manual run is needed, use a unique validation ref.** The workflow's concurrency group uses `github.ref`. Dispatching on `main` shares its group with main pushes; dispatching another run on a branch shares its group with earlier dispatches on that branch. Pull requests use their separate pull-request ref. GitHub can replace a pending run even when `cancel-in-progress` is false. Create a uniquely named validation branch from the freshly verified current PR SHA, then dispatch the existing workflow there. This isolates concurrency without editing ScribeKey's workflow or cancelling unrelated work.
3. **Verify execution, not only listener status.** Record run ID, exact SHA, requested labels, assigned runner, provisioned CPU/RAM/workspace mounts and image identity. Capture kernel/cgroup termination evidence and memory peak/events before cleanup. Require the blocking Gradle/native gates to finish and the ephemeral runner to clean up; retain profile and failure artifacts.
4. **Keep the acceptance scopes explicit.** A manual validation branch behaves as a non-main run, so Android Lint is skipped and screenshots remain report-only. Main lint, Mac screenshot baseline validation, device/product acceptance and signed releases remain separate gates. Do not call those passing based on this build.

Example dispatch commands, to be executed only as part of the deployment validation by the owning operator/agent. This creates a remote validation branch; it does not edit the ScribeKey working tree:

```sh
validation_sha=$(gh pr view 1135 --repo scribekey-app/scribekey --json headRefOid --jq .headRefOid)
validation_branch="codex/runnerfarm-acceptance-$(date -u +%Y%m%dT%H%M%SZ)"
gh api repos/scribekey-app/scribekey/git/refs --method POST \
  -f "ref=refs/heads/$validation_branch" -f "sha=$validation_sha"
gh workflow run pre-merge.yaml --repo scribekey-app/scribekey --ref "$validation_branch"
gh run list --repo scribekey-app/scribekey --workflow pre-merge.yaml \
  --branch "$validation_branch" --limit 1 \
  --json databaseId,headSha,status,conclusion,url
```

A fresh dispatch still waits for the fleet's normal capacity; it does not bypass queue or RAM admission. Record the ref and resulting run in deployment evidence. Removing that remote validation ref later is a separate cleanup action.

Official concurrency behavior: [control workflow concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency).

## Live screenshot OOM: 11 October, 17:59 AEDT

[Run 38090861764](https://github.com/scribekey-app/scribekey/actions/runs/38090861764), exact SHA `498871293ba5c4370804d811ef72c37241ebfe64` on `fix/1039-capture-rtl-large-text`, completed with overall `success` at 18:00 AEDT. Its blocking Gradle build passed in 16m 51s. The screenshot step did not complete successfully: at 17:59:18 it reported 452 tests completed, 24 failed, one skipped, followed by `Gradle Test Executor 3` exiting 137. Ordinary image comparison failures preceded the process termination.

The host kernel independently records a cgroup OOM at 17:59:17 for container `4b7b78d8d03290823a5dd43ada0b17ba1f25ac42344715ba6f820f56ae0bc36c`, killing Java PID 1507305 with anonymous RSS 6,439,812 KiB (6.14 GiB). The operator captured actual cgroup memory limit 12 GiB and `memory.events` values `max=1851`, `oom=3`, `oom_kill=1`; the host OOM counter increased from 73 to 74. This establishes a container memory limit failure, rather than a visual baseline mismatch or an unproven daemon disappearance. The exact killed JVM command line and native allocation breakdown were not retained in this evidence.

The source at that exact run SHA already configures screenshot `Test` tasks with `maxHeapSize = "2g"`. CI separately caps the Gradle daemon at 3g, the Kotlin daemon at 2g, and Gradle workers at two. Heap limits do not cap a process's native layoutlib/image allocations, thread stacks, metaspace, code cache, nor the container's other processes and charged filesystem memory. The observed Java RSS is substantially larger than the configured screenshot heap; effective child JVM arguments must still be captured before assigning the remaining bytes to a particular allocator.

Inspection of the pinned `0.0.1-alpha16` plugin and JUnit engine jars confirms rendering occurs inside the screenshot Test JVM through an isolated renderer classloader. `PreviewScreenshotValidationTask` extends Gradle `Test`, explicitly forces `maxParallelForks` to one, and delegates to the normal Gradle test execution machinery. ScribeKey does not set `forkEvery`, whose [Gradle default is zero](https://docs.gradle.org/current/userguide/java_testing.html): the test process can retain state throughout the complete preview matrix.

The durable resource pilot should use a single screenshot fork, retain the initial 2g heap, and recycle the test JVM after each preview class with `forkEvery = 1`. This bounds retained native renderer state by a preview class instead of the entire matrix. Run the whole matrix, verify that class selection and report generation remain complete, and measure its time overhead. If a single large class still exhausts memory, split that class or run deterministic method groups in separate processes. Do not raise the screenshot heap automatically: that can reduce headroom for native rendering. The new 16 GiB runner ceiling is a reasonable temporary envelope for measurement, but is not evidence that accumulation is fixed. This evidence proves an exhausted container budget; it does not establish a native leak, linear growth, or an unbounded JVM heap heuristic.

For that pilot, capture effective JVM arguments with Gradle `--info`, container `memory.peak`, `memory.stat`, `memory.events` before and after, and periodic JVM RSS. Optional Java Native Memory Tracking can distinguish JVM-managed native categories; it does not account for every allocation made by third-party native libraries. Acceptance requires a complete test matrix, zero new OOM kills, and safe headroom within the 16 GiB container limit. A green workflow with an incomplete screenshot run fails infrastructure acceptance.

The Linux screenshot policy should permit a **completed** image-difference report while treating executor crashes, OOM, render exceptions and missing/incomplete results as failures. The current step's unconditional `continue-on-error: true` masks both categories. Keep Mac baseline correctness as its separate gate; expose Linux infrastructure completeness as a blocking result without accepting Linux rasterization differences as baseline updates.

Android's [screenshot testing documentation](https://developer.android.com/studio/preview/compose-screenshot-testing) acknowledges host-side memory intensity and the experimental engine. It now recommends AGP native test suites starting with AGP 9.5.0-alpha03; ScribeKey is on AGP 9.4.1. Evaluate that migration as a separate toolchain change after the bounded-process pilot, not as an unverified emergency upgrade.

## Completed live acceptance: 18:25 AEDT

[Run 38085953402](https://github.com/scribekey-app/scribekey/actions/runs/38085953402) completed `success` at 18:25:36 AEDT, exact head `24b38a1f692f00b1b0aac4537f35a3954c01e170`, branch `claude/project-thread-20hzq3`, runner `unraid-builders-ec7b100c2797`. It used the deployed controller/image in container `4aee7efb329e2835fecb7618189cee4e7e707db5e1bbd62f52d9f964618b57b8`, started 18:04 AEDT. Its actual cgroup limit was 17,179,869,184 bytes (16 GiB), `memory.swap.max=0`, and swap usage stayed zero.

The blocking Gradle build passed in 15m 26s. The screenshot engine completed its **entire 520-test matrix**, matching the 520 reference PNGs at that exact head. Its XML reports `tests="520" skipped="0" failures="28" errors="0"`; every failure is `com.android.tools.screenshot.ImageComparisonAssertionError`. Logs report 520 tests completed at 18:24:40, followed by ordinary comparison failure, with no executor 137, OOM, unexpected process problem or renderer exception. Native package validation and artifact uploads passed; Android Lint was skipped on this non-main branch. GitHub presents the report-only screenshot step as successful despite its failed comparison command.

Before the container stopped, measured `memory.peak` was **13,824,065,536 bytes (12.875 GiB)**, with `memory.events` `low=0 high=0 max=0 oom=0 oom_kill=0 oom_group_kill=0`. The peak exceeds the previous 12 GiB limit. This supplies direct evidence that 16 GiB allowed this finite workload to finish without memory-limit intervention; it does not establish unlimited headroom for a growing preview matrix.

Live process inspection confirmed screenshot Java PID 2225947 actually ran with **`-Xmx2g`**, while the 3g Gradle and 2g Kotlin daemons remained resident. The screenshot process RSS grew from 1.85 GiB at 18:22:15 to 3.80 GiB at 18:22:55, 4.94 GiB at 18:23:31, 5.98 GiB at 18:24:11 and 6.73 GiB at 18:24:38; it exited normally after the complete matrix. At 18:22:55, cgroup memory comprised approximately 9.47 GiB anonymous memory, 0.40 GiB file memory and 0.09 GiB kernel memory. The dominant pressure was anonymous process memory, not a large tmpfs or page-cache charge. Non-heap renderer/process growth is directly observed; its precise allocator and leak status remain unproven.

Infrastructure acceptance passed for this exact SHA and full Linux rendering workload. Visual acceptance remains 28 image differences and requires its separate baseline review. The bounded-process pilot above remains the durability recommendation: it should reduce dependence on matrix-sized native retention before adding more previews or choosing a smaller hosted VM.
