# CI Runner Farm

An Unraid plugin that runs GitHub Actions self-hosted runners as Docker
containers on one box. This glossary fixes the vocabulary used across the engine,
the webGUI and the issue tracker.

## Language

### Core

**Fleet**:
A named, independently configured group of runners within one plugin install.
Fleet `default` is the original single-fleet installation.
_Avoid_: pool, group, cluster, farm

**Runner**:
One container that executes GitHub Actions jobs.
_Avoid_: worker, agent, node, executor

**Fleet mode**:
Whether a fleet obtains its runners by registration token (legacy) or through a
GitHub scale set (scale-set). A fleet is one or the other, never both.
_Avoid_: fleet type, runner kind

### Scale sets

**Scale set**:
The GitHub-side object a workflow targets by name in `runs-on`. Its name is
operator configuration and is referenced from repositories this plugin does not
control.
_Avoid_: runner set, pool

**Runner group**:
A GitHub access-control boundary deciding which repositories may schedule onto a
runner. A distinct concept from a scale set — a scale set is what work is sent
to, a runner group is who is allowed to send it.
_Avoid_: using interchangeably with scale set

**JIT runner**:
A runner created from a just-in-time configuration to serve exactly one job, then
discarded. It cannot be re-registered or reused.
_Avoid_: ephemeral runner, one-shot runner, disposable runner

**Listener session**:
The long-lived connection a scale-set fleet holds to GitHub in order to receive
job assignments.
_Avoid_: connection, subscription, poller, watcher

**Assignment**:
GitHub allocating a queued job to a scale set. An assignment exists before any
runner exists to serve it.
_Avoid_: dispatch, allocation

### Lifecycle

**Drain**:
Waiting for in-flight jobs to finish before removing their runners, rather than
interrupting them.
_Avoid_: graceful shutdown, quiesce, cordon

**Sweep**:
Removing runner containers that have exited.
_Avoid_: reap, garbage collect, prune

**Orphan**:
A GitHub runner record with no matching container, or a container GitHub no
longer has a record for.
_Avoid_: zombie, stray, ghost

**Stale runner**:
A running runner still built from a previous configuration generation, awaiting
migration onto the current one.
_Avoid_: outdated runner, old runner

### Access

**Credential**:
A named set of GitHub authentication material — either a personal access token,
or a GitHub App id plus private key. Any number of fleets may point at one
credential.
_Avoid_: token, secret, auth (each names only part of what a credential can be)
