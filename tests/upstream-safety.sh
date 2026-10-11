#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
php tests/upstream-safety.php
python3 - <<'PY'
import os
from pathlib import Path
import re
import shlex
import signal
import subprocess
import tempfile

runtime = Path('src/usr/local/emhttp/plugins/ci-runner-farm')
dockerfile = (runtime / 'default.Dockerfile').read_text()
block = dockerfile.split('RUN printf ', 1)[1].split('> /usr/local/bin/wait-docker.sh', 1)[0]
script = '\n'.join(shlex.split(block.replace('\\\n', ''))[1:]) + '\n'
with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    for name, body in {
        'docker': 'exit "${MOCK_DOCKER_RC:-1}"',
        'service': 'exit 0',
        'sleep': 'if [ "$1" = 3 ]; then /bin/sleep 30; fi',
        'job': f'touch "{tmp}/job-started"',
    }.items():
        path = root / name
        path.write_text('#!/usr/bin/env bash\n' + body + '\n')
        path.chmod(0o755)
    wrapper = root / 'wait-docker.sh'
    wrapper.write_text(script)
    for rc, dind in ((1, True), (0, True), (1, False)):
        marker = root / 'job-started'
        marker.unlink(missing_ok=True)
        env = dict(os.environ, PATH=tmp + ':' + os.environ['PATH'], MOCK_DOCKER_RC=str(rc), START_DOCKER_SERVICE=str(dind).lower())
        with (root / 'output').open('w') as output:
            process = subprocess.Popen(['bash', str(wrapper), 'job'], env=env, stdout=output, stderr=output, start_new_session=True)
            try:
                result = process.wait(timeout=5)
            finally:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
        assert (result == 0) == (rc == 0 or not dind), (rc, dind, result)
        assert marker.exists() == (rc == 0 or not dind), 'unready Docker must never accept a job'

    page = (runtime / 'RunnerFarmImage.page').read_text()
    pipeline = re.search(r' && ([^\n]*sdkmanager --licenses >/dev/null)', page).group(1)
    for rc in (0, 23):
        sdk = root / 'sdkmanager'
        sdk.write_text(f'#!/bin/sh\nexit {rc}\n')
        sdk.chmod(0o755)
        result = subprocess.run(['bash', '-euo', 'pipefail', '-c', pipeline], env=env, timeout=5)
        assert result.returncode == rc, ('license status must survive producer SIGPIPE', rc, result.returncode)
print('upstream-safety: Docker readiness and Android licenses OK')
PY
python3 tests/upstream-safety-release.py
