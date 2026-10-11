---
status: accepted
---

# Minute supervision replaces the original no-watchdog decision

## Accepted amendment, 11 October 2026

The original decision below is superseded. A plugin-owned Unraid cron entry
checks each fleet every minute. A configured scale-set fleet that is intended to
run is relaunched when its listener is absent, after Docker and its cache root
are ready. The fleet lock serializes recovery against Start, Stop, rename and
delete; process discovery compares the full binary path and fleet arguments,
so a missing or reused pidfile cannot hide a listener or signal another service.

An explicit Stop creates a small marker on flash. Boot and supervision honor it;
an explicit successful Start clears it. Docker shutdown only stops processes,
so returning Docker still recovers fleets that were previously enabled. Renaming
an empty fleet first stops its listener, then transfers the stopped intent or
lets supervision start the new name. Deleting an empty fleet stops its daemons
and removes its marker with the configuration. Rename and delete acquire locks
in the existing fleet-then-host order.

The original assumption that permanent unattended failure was acceptable did not
survive operation: earlier provisioning faults and resource contention left jobs
queued, zero-runner fleets displayed as stopped while their listeners were alive,
and development hotfixes disappeared on reinstall. The operator now explicitly
requires unattended reliability. Session reconnect and transient backoff remain
inside the Go listener; process recovery covers crashes and installation gaps.
Hard provisioning failures still pause acquisition until inert validation passes.

Recovery is bounded to one attempt per minute, logs each restart and does not
alter credentials or erase configuration errors. The UI still exposes listener
absence and capacity pressure. Runtime fleet logs are trimmed in place after
4 MiB to retain the newest 256 KiB, preserving open append handles and bounding
ordinary retry-log accumulation on tmpfs. This is a periodic ceiling: a burst can
temporarily exceed it, and a few concurrent log lines can be lost during trimming.
An ephemeral readiness file exists only while an authenticated message session is
open; the UI combines it with process identity to distinguish Starting from Ready.
It is telemetry, never a second authority for runner reconciliation.

## Original decision, retained for history

Nothing automatically restarts a dead `crf-scalesetd`. The daemon owns session
reconnection internally with backoff and does not exit on transient failure; it
exits only when configuration says stop or when authentication is
unrecoverable. Process-level supervision is exactly what the autoscale daemon
already does — pid file, liveness check, relaunched by fleet start and by the
Docker-started event — and nothing more.

The failure that actually happens is the network session dropping, and the
client can handle that internally with far better information than an external
liveness poll ever gets. Process death after that is a genuine bug, and a
supervisor that silently respawns a crashlooping daemon converts a visible fault
into an invisible one.

## Consequences

An unattended crash leaves that fleet not acquiring work until an operator looks
at the UI or Docker restarts. This is accepted deliberately: the UI reports the
listener as degraded, which we consider better than a fleet that appears healthy
while restarting every sixty seconds.

Resolves part of #10.
