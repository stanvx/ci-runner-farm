---
status: accepted
---

# Docker labels are the only reconciliation state

The scale-set daemon persists nothing. Container labels carry fleet identity and
the GitHub runner name, and every reconciliation decision is derived fresh at
startup by intersecting `docker ps` with the scale-set API: exited containers are
swept, running ones are left alone to finish the job they already hold, and
GitHub records with no matching container are deregistered.

Labels are already the source of truth in this engine, so this adds one label to
an existing namespace rather than a new storage mechanism. There is no schema to
migrate across plugin upgrades, nothing to corrupt on an unclean kill, and no
second authority that can disagree with Docker — because when a store and
`docker ps` disagree, `docker ps` wins anyway, which makes the store a cache
pretending to be an authority.

## Considered options

A state file on tmpfs would close the window between "GitHub assigned a job" and
"container exists", but it vanishes on reboot exactly when containers may not, so
the label diff is still required and the file adds a second mechanism without
removing the first. An embedded database on flash was rejected on three counts:
writing runtime state to the USB stick is a wear antipattern this project
avoids everywhere else, it needs migrations at every upgrade, and it does not
resolve the disagreement problem above.

## Consequences

Because state is derived rather than remembered, a runner container that Docker
stopped is indistinguishable from one that finished its job — both are simply
exited, and both are swept. This is why a scale-set fleet sweeps rather than
restarts containers in place when Docker returns: a restarted JIT runner holds an
assignment that has long since timed out, so it would idle forever while
occupying a capacity slot.

## Amendment: half the diff is currently unreachable

Implementation (#22) found that `github.com/actions/scaleset` v0.4.0 exposes no
list-runners call — only `GetRunnerByName`. The intersection this decision
describes is therefore only computable in one direction. A GitHub record can be
removed when its container is still present to name it, because the container
label carries the runner name; a record whose container has vanished entirely
cannot be enumerated and so cannot be reconciled at all.

This does not change the decision. Labels remain the only state we keep, and
adding a store would not help: the orphan is on the GitHub side, so no local
record makes it enumerable. It does mean the "GitHub records with no matching
container are deregistered" clause above overstates what the library permits
today, and orphaned records accumulate until the API grows a list call or we
drop to the REST runners endpoint the legacy path already uses.

Resolves part of #10, and supplies the scale-set replacement for legacy runner
deregistration that #5 identified as necessary.
