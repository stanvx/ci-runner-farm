# ScribeKey CI failure evidence and runner choices — 11 October 2026

The current ScribeKey queue is waiting for self-hosted provisioning. Earlier failures also include screenshot baselines, terminated Java processes, hosted billing, and Android toolchain installation. Changing one runner setting cannot resolve every category.

Read-only GitHub API and log audit, refreshed at 2026-10-11 06:10 UTC / 17:10 AEDT. This document records an observation, not continuing service health. No jobs were cancelled, rerun, or dispatched during this audit.

## Current state

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
| Queue/provisioning | PR #1135 run above has no runner; current inventory has no online runner. | Inspect listener assignment, admission decision, image availability, Docker creation and runner registration. Authentication or an inert validation pass alone is insufficient. |
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
| Standard hosted `ubuntu-24.04` | 2 CPUs, 8 GB RAM, 14 GB SSD | $0.006/minute after the owner's standard allowance. GitHub Free for organizations includes 2,000 minutes/month. | GitHub maintains the VM/image and autoscaling. Less RAM than the current 12 GiB build container; benchmark before migration. |
| Hosted 4-core Ubuntu larger runner | 4 CPUs, 16 GB RAM, 150 GB SSD | $0.012/minute, always billed; included minutes do not apply. Requires GitHub Team or Enterprise Cloud organization and valid payment setup. | Closest simple hosted replacement for the current build budget. Larger runner assignment can have a cold-start delay. |
| Existing Unraid fleet | Currently intended as one 4-CPU/12-GiB build runner; verify live configuration at deployment | Self-hosted execution currently has no GitHub runner-minute charge. Hardware, electricity, storage and maintenance remain local costs. | Exact prebuilt Android image and local/private hardware access, with host capacity, image lifecycle, credentials and LAN availability owned locally. |

At an assumed 20-minute duration, standard hosted Linux is $0.12 per build beyond included minutes; a 4-core larger runner is $0.24. At 100 such builds, that is $12 or $24 before standard included allowance. These are arithmetic scenarios, not measured forecasts: the 2-core host can take longer, and cache/toolchain installation changes runtime.

Official sources: [hosted specifications](https://docs.github.com/en/actions/reference/runners/github-hosted-runners), [larger runner specifications and payment requirements](https://docs.github.com/en/actions/reference/runners/larger-runners), [runner prices and larger-runner eligibility](https://docs.github.com/en/billing/reference/actions-runner-pricing), [allowances and billing admission](https://docs.github.com/en/billing/concepts/product-billing/github-actions).

For minimal management, ordinary hosted CI removes more responsibilities than hardening a custom autoscaler. Retain Unraid where private devices, local hardware, or an exact persistent image justify it. A hosted 4-core pilot is a more credible fit for the existing memory budget than assuming an 8 GB standard VM is interchangeable. Do not add automatic cross-provider retry/fallback until both environments can execute the same toolchain contract successfully.

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
