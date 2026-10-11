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

commit = shell_step('release-please.yml', 'Commit plugin release metadata')
dispatch = shell_step('release-please.yml', 'Validate prepared release branch')
workflow = (ROOT / '.github/workflows/release-please.yml').read_text()
prepare_job = workflow.split('  prepare-plugin-release-pr:\n', 1)[1].split('  validate-plugin-release:\n', 1)[0]
assert '    permissions:\n      actions: write\n      contents: write\n' in prepare_job
assert workflow.count('actions: write') == 1, 'dispatch permission must remain scoped to metadata job'
assert prepare_job.index('Commit plugin release metadata') < prepare_job.index('Validate prepared release branch')
assert 'GH_TOKEN: ${{ github.token }}' in prepare_job, 'dispatch uses the trusted workflow token'
assert prepare_job.count('RELEASE_BRANCH: ${{ fromJSON(needs.release-please.outputs.pr).headBranchName }}') == 2
assert '${{' not in commit + dispatch, 'branch expressions must enter shell through environment values'

with tempfile.TemporaryDirectory() as directory:
    work = Path(directory)
    bindir = work / 'bin'
    bindir.mkdir()
    # Deliberate shell syntax proves the branch is one literal argument, never code.
    branch = 'release-please--branches--main;$(touch injected)'
    env = dict(os.environ, PATH=str(bindir) + ':' + os.environ['PATH'],
               RELEASE_BRANCH=branch, GITHUB_REPOSITORY='stanvx/ci-runner-farm', GH_TOKEN='test-only-token')
    mock = '''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
command = pathlib.Path(sys.argv[0]).name
with pathlib.Path('calls').open('a') as log: log.write(json.dumps([command] + args) + '\\n')
if command == 'git':
    if args[0] == 'status' and os.environ['DIRTY'] == '1': print(' M VERSION')
    if args[0] == 'push' and os.environ['FAIL_PUSH'] == '1': sys.exit(1)
elif args[2] == 'lint.yml' and os.environ['FAIL_DISPATCH'] == '1': sys.exit(1)
'''
    for command in ('git', 'gh'):
        path = bindir / command
        path.write_text(mock)
        path.chmod(0o755)
    for dirty, fail_push, fail_dispatch in ((1, 0, 0), (0, 0, 0), (1, 1, 0), (1, 0, 1)):
        (work / 'calls').unlink(missing_ok=True)
        case_env = dict(env, DIRTY=str(dirty), FAIL_PUSH=str(fail_push), FAIL_DISPATCH=str(fail_dispatch))
        result = subprocess.run(['bash', '-c', commit], cwd=work, env=case_env, capture_output=True)
        if result.returncode == 0:
            result = subprocess.run(['bash', '-c', dispatch], cwd=work, env=case_env, capture_output=True)
        calls = [json.loads(line) for line in (work / 'calls').read_text().splitlines()]
        dispatches = [call for call in calls if call[0] == 'gh']
        expected = [] if fail_push else ['lint.yml'] if fail_dispatch else ['lint.yml', 'package-plugins.yml']
        assert [call[3] for call in dispatches] == expected, calls
        for call in dispatches:
            assert call == ['gh', 'workflow', 'run', call[3], '--repo', 'stanvx/ci-runner-farm', '--ref', branch], call
        if dirty and not fail_push:
            push = ['git', 'push', 'origin', 'HEAD:' + branch]
            assert push in calls and calls.index(push) < calls.index(dispatches[0]), calls
        assert (result.returncode != 0) == bool(fail_push or fail_dispatch), result.stderr
        assert not (work / 'injected').exists(), 'branch value was executed as shell code'
print('upstream-safety: release metadata, verified payloads, automatic prepared-branch dispatch OK')
