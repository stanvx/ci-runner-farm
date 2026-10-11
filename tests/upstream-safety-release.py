#!/usr/bin/env python3
"""Run the actual release workflow shell at mocked build/network boundaries."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def shell_step(workflow, name):
    text = (ROOT / '.github/workflows' / workflow).read_text()
    block = text.split('      - name: ' + name + '\n', 1)[1]
    lines = block.split('        run: |\n', 1)[1].splitlines()
    shell = []
    for line in lines:
        if line and not line.startswith('          '):
            break
        shell.append(line[10:] if line else '')
    return '\n'.join(shell)


validate = shell_step('release.yml', 'Validate plugin metadata').split('plg_entity()', 1)[0]
publish = shell_step('release-please.yml', 'Upload validated plugin release artifacts')
assert 'cancel-in-progress: false' in (ROOT / '.github/workflows/release-please.yml').read_text()

with tempfile.TemporaryDirectory() as directory:
    work = Path(directory)
    bindir = work / 'bin'
    bindir.mkdir()
    env = dict(os.environ, PATH=str(bindir) + ':' + os.environ['PATH'],
               RELEASE_TAG='v1.9.3', GITHUB_REPOSITORY='stanvx/ci-runner-farm',
               GITHUB_RUN_ID='123', GITHUB_SERVER_URL='https://github.com')
    git = bindir / 'git'
    git.write_text('#!/bin/sh\nprintf 1789992000\n')
    git.chmod(0o755)
    (work / '.release-please-manifest.json').write_text(json.dumps({'.': '1.9.3'}))
    (work / 'build-plg.sh').write_text('printf "%s %s %s %s\\n" "$DATE" "$BUILD_NUMBER" "$INTERNAL_VERSION" "$REPO" > built\n')
    for _ in range(2):
        result = subprocess.run(['bash', '-c', validate], cwd=work, env=env, capture_output=True)
        assert result.returncode == 0, result.stderr
        assert (work / 'built').read_text() == '2026.09.21.1200 0 1.9.3 stanvx/ci-runner-farm\n'
    (work / 'built').unlink()
    env['RELEASE_TAG'] = 'v9.9.9'
    result = subprocess.run(['bash', '-c', validate], cwd=work, env=env, capture_output=True)
    assert result.returncode != 0 and not (work / 'built').exists(), 'manifest mismatch must fail before building'
    env['RELEASE_TAG'] = 'v1.9.3'

    assets = ['ci-runner-farm-version.tgz', 'crf-scalesetd-version', 'ci-runner-farm.plg']
    fixtures = work / 'fixtures'
    fixtures.mkdir()
    for asset in assets:
        (fixtures / asset).write_text('validated ' + asset)
    mock = '''#!/usr/bin/env python3
import os, pathlib, shutil, sys
root = pathlib.Path.cwd()
args = sys.argv[1:]
if pathlib.Path(sys.argv[0]).name == 'gh':
    if args[:2] == ['run', 'download']:
        shutil.copytree(root / 'fixtures', root / 'release-artifacts', dirs_exist_ok=True)
    else:
        source = pathlib.Path(args[5])
        with (root / 'uploads').open('a') as log: log.write(source.name + '\\n')
        (root / 'published').mkdir(exist_ok=True)
        shutil.copyfile(source, root / 'published' / source.name)
else:
    target = pathlib.Path(args[args.index('--output') + 1])
    source = root / 'published' / args[-1].rsplit('/', 1)[1]
    shutil.copyfile(source, target)
    if source.name == os.environ.get('CORRUPT_ASSET'): target.write_text('corrupt')
'''
    for command in ('gh', 'curl'):
        path = bindir / command
        path.write_text(mock)
        path.chmod(0o755)
    for corrupt in ('', assets[1]):
        (work / 'uploads').unlink(missing_ok=True)
        result = subprocess.run(['bash', '-c', publish], cwd=work,
                                env=dict(env, CORRUPT_ASSET=corrupt), capture_output=True)
        uploaded = (work / 'uploads').read_text().splitlines()
        assert uploaded == (assets[:2] if corrupt else assets), uploaded
        assert (result.returncode != 0) == bool(corrupt), result.stderr
print('upstream-safety: release metadata race, upload order, anonymous byte verification OK')
