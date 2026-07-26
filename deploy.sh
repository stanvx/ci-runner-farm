#!/bin/bash
# Safe RAW deploy of the plugin runtime to an Unraid host (dev iteration only).
# ALWAYS chowns root:root + sets explicit perms after copying, so a raw deploy
# can never leave non-root files that would break the GUI/SSH. For real installs
# use the .plg (build-plg.sh) — this is just for fast iteration on a dev box.
set -euo pipefail

HOST="${1:?usage: ./deploy.sh root@host}"
NAME="ci-runner-farm"
SRC="src/usr/local/emhttp/plugins/${NAME}"
DEST="/usr/local/emhttp/plugins/${NAME}"
cd "$(dirname "$0")"

echo "[deploy] syncing $SRC -> $HOST:$DEST"
ssh "$HOST" "mkdir -p '$DEST/include' '$DEST/nchan' '$DEST/event'"
# Globs, not a hand-maintained list: this used to name five files, so editing any other
# tab and deploying silently shipped nothing. src/ is a byte-for-byte image of $DEST
# (build-plg.sh tars it wholesale), so copying all of it is also the accurate thing.
scp -q "$SRC"/*.page "$SRC"/default.cfg "$SRC"/default.Dockerfile "$SRC"/README.md "$HOST:$DEST/"
scp -q "$SRC"/nchan/* "$HOST:$DEST/nchan/"
scp -q "$SRC"/include/* "$HOST:$DEST/include/"
scp -q "$SRC"/event/* "$HOST:$DEST/event/"

echo "[deploy] enforcing root:root + perms on $HOST"
ssh "$HOST" "
  chown -R root:root '$DEST'
  find '$DEST' -type d -exec chmod 0755 {} +
  find '$DEST' -type f -exec chmod 0644 {} +
  chmod 0755 '$DEST/include/runner-farm.sh'
  find '$DEST/nchan' -type f -exec chmod 0755 {} + 2>/dev/null || true  # monitor_nchan execs the publisher
  find '$DEST/event' -type f -exec chmod 0755 {} + 2>/dev/null || true  # emhttp_event execs these
  echo '[deploy] done:'; ls -la '$DEST'
"
