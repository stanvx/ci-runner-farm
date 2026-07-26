---
status: accepted
---

# No watchdog for the scale-set daemon

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
