---
status: accepted
---

# One scale-set daemon process per fleet

Scale-set fleets need a long-lived listener session, and we run one
`crf-scalesetd` process per scale-set fleet rather than a single process holding
every fleet's session as a goroutine. Every runtime path in the engine is
already fleet-scoped — pid, lock, log and state files all carry the fleet suffix
— so per-fleet processes inherit that isolation for free, one wedged session
cannot stop another fleet acquiring work, and each process holds exactly one
credential.

The cost is N processes at roughly 15-30 MB each. On the target hardware that is
noise, and we preferred it to reimplementing per-fleet serialisation, per-fleet
log demultiplexing, and multi-credential isolation inside one address space.

Resolves part of #10. A privilege-split daemon pair was separately rejected in #3.
