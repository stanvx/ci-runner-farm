---
status: accepted
---

# The scale set is found-or-created and never automatically deleted

Starting a scale-set fleet creates the GitHub scale set if it is absent and
attaches to it otherwise. Stopping the fleet closes the listener session and
leaves the scale set in place. Deleting it is only ever an explicit operator
action.

A scale set is named by the operator and referenced from `runs-on:` in
repositories this plugin does not control, which makes it part of the user's
configuration surface rather than our runtime state. Deleting it on stop would
turn an ordinary stop into a breaking change for every workflow naming it, and
jobs queued during the gap would fail outright instead of waiting for the fleet
to return.

## Consequences

Uninstalling the plugin leaves the scale set behind in GitHub, because the
removal path stops fleets and stopping does not delete. This needs either
documenting or an explicit prompt in the plugin's remove step; it is not yet
handled.

Resolves part of #10.
