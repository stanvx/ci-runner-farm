---
status: accepted
---

# Stop drains a scale-set fleet with a bounded deadline

Stopping a scale-set fleet advertises zero capacity, stops acquiring new work,
lets in-flight jobs finish, and force-removes whatever is still running when the
deadline expires. The default deadline is 600 seconds. Force stop skips the wait
entirely, as does the Docker-stopping event, where the host is shutting down and
will terminate the containers regardless.

The deadline is deliberately far shorter than the one guarding image rollover,
which waits an hour. That longer wait protects persistent runners whose
interrupted work is simply lost. A job interrupted on a scale-set runner is
re-queued by GitHub instead, so the deadline buys politeness rather than
preventing data loss, and a bounded stop that always terminates is worth more
than an unbounded one that appears hung.

## Consequences

Stop can take up to ten minutes on a fleet running long jobs. An unbounded drain
was rejected because array shutdown terminates the process well before any
generous deadline elapses, making the guarantee fictional precisely when it
would matter.

Resolves part of #10.
