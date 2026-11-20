#!/usr/bin/env python3
"""FlashGate's local test entry point; composes existing project gates."""
from __future__ import annotations

import argparse
import contextlib
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import shutil
import stat
import subprocess
import sys
import time
import uuid


class Hold(RuntimeError):
    """An unproven binding or process state must not become PASS."""


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def identity(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def load(path):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise Hold('Duplicate JSON property')
            result[key] = value
        return result
    value = json.loads(Path(path).read_text(encoding='utf-8'), object_pairs_hook=unique)
    if not isinstance(value, dict):
        raise Hold('JSON object required')
    return value


def write(path, value):
    with Path(path).open('x', encoding='utf-8', newline='\n') as stream:
        json.dump(value, stream, sort_keys=True, indent=2)
        stream.write('\n')


def safe_directory(path):
    path = Path(os.path.abspath(path))
    for item in (path, *path.parents):
        meta = item.lstat()
        if not stat.S_ISDIR(meta.st_mode) or item.is_symlink() or getattr(meta, 'st_file_attributes', 0) & 0x400:
            raise Hold('Work path crosses a non-directory or link')
    return path


def git(root, *args):
    result = subprocess.run(['git', '-C', str(root), *args], check=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
    return result.stdout


def sources(root):
    names = set(git(root, 'ls-files', '-z').decode().split('\0'))
    names.update(git(root, 'ls-files', '--others', '--exclude-standard', '-z').decode().split('\0'))
    records = []
    for name in sorted(names - {''}):
        path = root / name
        # Deletions also affect identity and package selection.
        if not path.exists() and not path.is_symlink():
            records.append([name, 'DELETED'])
            continue
        meta = path.lstat()
        if not stat.S_ISREG(meta.st_mode) or path.is_symlink() or getattr(meta, 'st_file_attributes', 0) & 0x400:
            raise Hold('Nonregular source input')
        records.append([name, digest(path), stat.S_IMODE(meta.st_mode)])
    if not records:
        raise Hold('Empty source inventory')
    return records


def source_hygiene(root):
    forbidden = []
    for name in ('scripts', 'cmd', 'internal'):
        for path in (root / name).rglob('*'):
            if path.name == '__pycache__' or path.suffix in ('.pyc', '.pyo', '.syso') or path.name.endswith(('.test', '.test.exe')):
                forbidden.append(path.relative_to(root).as_posix())
    if forbidden:
        raise Hold('Generated artifacts in persistent source: ' + ', '.join(forbidden[:10]))
    return {'generated_source_artifact_count': 0}


def environment(work, create=True):
    env = os.environ.copy()
    for key, child in [('GOCACHE', 'go-build'), ('GOMODCACHE', 'modules'), ('GOPATH', 'gopath'),
                       ('GOTMPDIR', 'go-tmp'), ('TEMP', 'tmp'), ('TMP', 'tmp'), ('TMPDIR', 'tmp'),
                       ('GOLANGCI_LINT_CACHE', 'lint-cache'), ('XDG_CACHE_HOME', 'xdg-cache'),
                       ('XDG_CONFIG_HOME', 'xdg-config'), ('XDG_DATA_HOME', 'xdg-data'),
                       ('DOTNET_CLI_HOME', 'dotnet')]:
        path = work / child
        if create:
            path.mkdir(exist_ok=True)
        safe_directory(path)
        env[key] = str(path)
    env.update(GOTOOLCHAIN='local', GOENV='off', GOWORK='off', GOPROXY='off', GOSUMDB='off',
               GOTELEMETRY='off', GOFLAGS='-mod=vendor -p=1 -count=1', GOEXPERIMENT='',
               FLASHGATE_WORK_ROOT=str(work), PYTHONDONTWRITEBYTECODE='1',
               POWERSHELL_TELEMETRY_OPTOUT='1', GIT_OPTIONAL_LOCKS='0',
               DOTNET_SKIP_FIRST_TIME_EXPERIENCE='1')
    return env


def tools_binding(env):
    result = {}
    for name in ('go', 'golangci-lint', 'pwsh', 'git'):
        executable = shutil.which(name, path=env['PATH'])
        if not executable:
            raise Hold('Required executable unavailable: ' + name)
        result[name] = {'path': str(Path(executable).resolve()), 'sha256': digest(executable)}
    return result


def binding(root, env):
    # Conservative whole-checkout binding: docs and tests are not silently
    # excluded. No commit binding: an unchanged commit/transport may change.
    names = ('PATH', 'GOOS', 'GOARCH', 'CGO_ENABLED', 'CC', 'CXX', 'GOFLAGS',
             'GOEXPERIMENT', 'GOTOOLCHAIN', 'GOENV', 'GOWORK', 'GOPROXY', 'GOSUMDB',
             'GODEBUG', 'GOMAXPROCS', 'GIT_DIR', 'GIT_WORK_TREE', 'FLASHGATE_TASK_ROOT')
    names += ('TEMP', 'TMP', 'TMPDIR', 'GOTMPDIR', 'FLASHGATE_WORK_ROOT')
    return {'sources': sources(root), 'tools': tools_binding(env),
            'os': [platform.system(), platform.release(), platform.machine(), platform.version()],
            'python': [sys.version, digest(sys.executable)],
            'environment': {name: identity(env.get(name, '')) for name in names},
            'actor': os.getuid() if hasattr(os, 'getuid') else os.environ.get('USERNAME', '')}


@contextlib.contextmanager
def checkout_descriptor(root):
    """Open the actual directory through nonfollowing, directory-relative FDs."""
    root = Path(os.path.abspath(root))
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
    descriptor = None
    try:
        try:
            descriptor = os.open(root.anchor, flags)
            for component in root.parts[1:]:
                child = os.open(component, flags, dir_fd=descriptor)
                os.close(descriptor)
                descriptor = child
            if not stat.S_ISDIR(os.fstat(descriptor).st_mode):
                raise Hold('Checkout lock requires a directory')
        except OSError as error:
            raise Hold('Nonfollowing checkout directory lock unavailable') from error
        yield descriptor
    finally:
        if descriptor is not None:
            os.close(descriptor)


@contextlib.contextmanager
def heavy_lock(resource, root):
    # Linux locks the existing checkout directory inode without creating or
    # writing a lock file. Separate work roots share that same kernel lock.
    # OS locks release on death; independent checkouts remain separate.
    if os.name == 'nt':
        import ctypes
        from ctypes import wintypes
        kernel = ctypes.WinDLL('kernel32', use_last_error=True)
        kernel.CreateMutexW.argtypes = [ctypes.c_void_p, wintypes.BOOL, wintypes.LPCWSTR]
        kernel.CreateMutexW.restype = wintypes.HANDLE
        kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
        kernel.CloseHandle.argtypes = [wintypes.HANDLE]
        kernel.ReleaseMutex.argtypes = [wintypes.HANDLE]
        # No shared filesystem write or machine-wide lock. Every Windows
        # task for this checkout, including its native stage, uses this key.
        handle = kernel.CreateMutexW(None, False, 'Local\\FlashGateValidation-' + identity(str(root).casefold()))
        if not handle:
            raise Hold('Checkout mutex unavailable')
        acquired = False
        try:
            state = kernel.WaitForSingleObject(handle, 0)
            acquired = state in (0, 0x80)
            if state != 0:
                raise Hold('Checkout heavy validation active or previous owner unconfirmed')
            yield
        finally:
            if acquired:
                kernel.ReleaseMutex(handle)
            kernel.CloseHandle(handle)
        return
    import fcntl
    with checkout_descriptor(root) as descriptor:
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as error:
            raise Hold('Checkout heavy validation active or filesystem lock unavailable') from error
        try:
            with checkout_descriptor(root) as current:
                before, after = os.fstat(descriptor), os.fstat(current)
                if (before.st_dev, before.st_ino) != (after.st_dev, after.st_ino):
                    raise Hold('Checkout directory binding changed while acquiring its lock')
            yield
        finally:
            fcntl.flock(descriptor, fcntl.LOCK_UN)


def run_process(root, work, name, argv, env, timeout=1800):
    stdout, stderr = work / (name + '.stdout'), work / (name + '.stderr')
    start = time.monotonic()
    if os.name == 'nt':
        request, result = work / (name + '.request.json'), work / (name + '.process.json')
        write(request, {'executable': argv[0], 'arguments': argv[1:], 'directory': str(root),
                        'stdout': str(stdout), 'stderr': str(stderr), 'timeout_ms': int(timeout * 1000)})
        # Reuse the C# Windows process-tree owner, not taskkill/PID discovery.
        bridge = root / 'scripts/validation-process.ps1'
        child = subprocess.run([shutil.which('pwsh', path=env['PATH']), '-NoProfile', '-NonInteractive',
                                '-File', str(bridge), '-RequestPath', str(request), '-ResultPath', str(result)],
                               env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               timeout=timeout + 120, check=False)
        if child.returncode or not result.is_file():
            raise Hold('Windows process bridge did not confirm completion')
        process = load(result)
    else:
        # Reuse the existing Linux session/start-identity owner directly. Its
        # CLI's short fixture budget is independent of this suite budget.
        spec = importlib.util.spec_from_file_location('bounded_validation', root / 'scripts/bounded-process-runner.py')
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        args = argparse.Namespace(command=argv, working_directory=str(root), timeout_seconds=timeout,
                                  maximum_bytes=33554432, stdout_file=str(stdout), stderr_file=str(stderr))
        previous = os.environ.copy()
        try:
            os.environ.update(env)
            fields = module._run_bounded_process(args)
        finally:
            os.environ.clear()
            os.environ.update(previous)
        process = {'status': fields[1], 'exit_code': None if fields[2] == 'none' else int(fields[2]),
                   'timed_out': fields[3] == 'true', 'failure_reason': fields[5], 'output_limit': fields[4] == 'true'}
    process['seconds'] = round(time.monotonic() - start, 3)
    process['artifacts'] = [[p.name, digest(p)] for p in (stdout, stderr)]
    reason = process['failure_reason']
    if process['exit_code'] is None or any(word in reason for word in ('Unknown', 'Unconfirmed', 'CleanupFailure', 'CleanupUnconfirmed')):
        process['status'] = 'HOLD'
    return process


def assert_no_competing_go():
    """Observe names only; process discovery never authorizes termination."""
    if os.name == 'nt':
        import csv
        executable = str(Path(os.environ.get('SystemRoot', 'C:/Windows')) / 'System32/tasklist.exe')
        result = subprocess.run([executable, '/FO', 'CSV', '/NH'], capture_output=True, timeout=15, check=False)
        if result.returncode:
            raise Hold('Competing process state unavailable')
        names = [row[0].casefold() for row in csv.reader(result.stdout.decode(errors='replace').splitlines()) if row]
        busy = any(name == 'go.exe' or name.endswith('.test.exe') for name in names)
    else:
        busy = False
        for entry in Path('/proc').iterdir():
            if not entry.name.isdecimal():
                continue
            try:
                name = (entry / 'comm').read_text().strip()
                busy = busy or name == 'go' or name.endswith('.test')
            except (FileNotFoundError, ProcessLookupError):
                continue
            except PermissionError as error:
                raise Hold('Competing process state unavailable') from error
    if busy:
        raise Hold('Competing Go process observed; no process was stopped')


def changed_paths(root):
    changed = git(root, 'diff', '--name-only', 'HEAD', '-z').decode().split('\0')
    changed += git(root, 'ls-files', '--others', '--exclude-standard', '-z').decode().split('\0')
    return sorted(set(changed) - {''})


def plan(mode, changed, root, work, tools):
    go, pwsh, lint = (tools[name]['path'] for name in ('go', 'pwsh', 'golangci-lint'))
    ps = [pwsh, '-NoProfile', '-NonInteractive', '-File']
    host = 'windows' if os.name == 'nt' else 'linux'
    steps = []
    def add(name, args, heavy=False):
        steps.append({'name': name, 'argv': args, 'heavy': heavy})
    docs = mode == 'CANDIDATE' or any(p.endswith('.md') for p in changed)
    shell = mode == 'CANDIDATE' or any(p.startswith(('scripts/', '.github/')) for p in changed)
    if docs:
        add('documentation', [go, 'run', './cmd/doccheck'])
        if os.name == 'nt':
            add('documentation-wrapper', ps + ['scripts/Test-DocumentationConsistency.ps1'])
    if shell:
        add('runner-regressions', [sys.executable, '-B', 'scripts/test-validation.py'])
        if os.name == 'nt':
            add('shell', ps + ['scripts/Test-ShellScripts.ps1'])
            add('shell-fixtures', ps + ['scripts/Test-ShellScripts.Tests.ps1', '-WorkingPath', str(work)])
        else:
            add('shell', ['/usr/bin/bash', 'scripts/test-shell-scripts.sh'])
            add('shell-fixtures', ['/usr/bin/bash', 'scripts/test-shell-scripts.tests.sh'])
    if os.name == 'nt' and mode == 'CANDIDATE':
        add('snapshot-fixtures', ps + ['scripts/Test-GitSnapshotSecurity.ps1', '-WorkingPath', str(work)])
    if mode == 'FAST':
        # File changes with unknown dependency reach select the full suite.
        go_changes = [p for p in changed if p.endswith('.go')]
        safe = go_changes and all(p.endswith('_test.go') for p in go_changes) and len(go_changes) == len(changed)
        packages = sorted({'./' + str(Path(p).parent).replace('\\', '/') for p in go_changes}) if safe else ['./...']
        if not changed:
            raise Hold('FAST needs a changed source or an explicit --changed path')
        add('focused-tests', [go, 'test', '-json', *packages], True)
        return steps
    add('format', [lint, 'fmt', '--diff'])
    add('vet', [go, 'vet', './...'])
    add('lint', [lint, 'run'])
    add('coverage', ps + ['scripts/Test-GoCoverage.ps1', '-PlatformName', host,
                         '-MinimumCoverage', '90', '-OutputRoot', str(work / 'coverage')], True)
    if os.name != 'nt':
        add('race', [go, 'test', '-race', '-json', './...'], True)
    add('mcp', [go, 'run', '-mod=vendor', './cmd/mcpcheck', '-report', str(work / 'mcp.md')], True)
    binary = root / 'build' / ('flashgate-mcp.exe' if os.name == 'nt' else 'flashgate-mcp')
    # Existing smoke entry points consume this established ignored build path.
    add('build', [go, 'build', '-o', str(binary), './cmd/server'], True)
    if os.name == 'nt':
        for name, script, extra in [('smoke', 'smoke-jsonrpc.ps1', []), ('smoke-read', 'smoke-jsonrpc.ps1', ['-ReadOnly']),
                                    ('smoke-negative', 'smoke-jsonrpc-negative.ps1', []), ('startup-negative', 'smoke-startup-negative.ps1', [])]:
            add(name, ps + ['scripts/' + script] + extra)
    else:
        for name, script, extra in [('smoke', 'smoke-jsonrpc.sh', []), ('smoke-read', 'smoke-jsonrpc.sh', ['--read-only']),
                                    ('smoke-negative', 'smoke-jsonrpc-negative.sh', []), ('startup-negative', 'smoke-startup-negative.sh', [])]:
            add(name, ['/usr/bin/bash', 'scripts/' + script] + extra)
    return steps


def verify_evidence(path, expected, current, steps):
    if not expected or digest(path) != expected.lower():
        raise Hold('Evidence requires an independently retained matching SHA-256')
    proof = load(path)
    if proof.get('schema') != 1 or proof.get('status') != 'PASS' or proof.get('binding') != current:
        raise Hold('Evidence failed, incomplete or invalidated')
    if proof.get('plan') != steps:
        raise Hold('Evidence gate/argument scope differs')
    base = Path(path).parent
    results = proof.get('results', [])
    if not isinstance(results, list) or not all(isinstance(row, dict) for row in results):
        raise Hold('Invalid result matrix')
    if [r.get('name') for r in results] != [s['name'] for s in steps]:
        raise Hold('Evidence matrix incomplete')
    for result in results:
        if result.get('status') != 'PASS' or result.get('exit_code') != 0 or result.get('timed_out'):
            raise Hold('Unsuccessful evidence cannot be reused')
    for name, sha in proof.get('artifacts', []):
        item = Path(name)
        if item.is_absolute() or '..' in item.parts or not (base / item).is_file() or (base / item).is_symlink() or digest(base / item) != sha:
            raise Hold('Evidence artifact missing or manipulated')
        safe_directory((base / item).parent)
    if not proof.get('artifacts'):
        raise Hold('Evidence has no artifacts')
    if artifact_inventory(base) != proof['artifacts']:
        raise Hold('Evidence inventory changed')
    return proof


def artifact_inventory(work):
    return [[p.relative_to(work).as_posix(), digest(p)] for p in sorted(work.rglob('*'))
            if p.is_file() and p != work / 'summary.json']


def wsl_path(path):
    value = str(Path(path).resolve()).replace('\\', '/')
    if len(value) < 3 or value[1:3] != ':/':
        raise Hold('WSL transport requires a local drive path')
    return '/mnt/' + value[0].lower() + value[2:]


def reviewed_candidate_identity(manifest):
    # Portable content identity used by the external Classic outcome contract.
    # No private workflow module or reviewer authentication service is needed.
    entries = []
    for row in manifest['Entries']:
        row = dict(row)
        if 'TargetPath' in row:
            row['Path'] = row.pop('TargetPath')
        entries.append(row)
    technical = manifest['TechnicalCandidate']
    subject = {**manifest, 'Entries': sorted(entries, key=lambda row: row['Path']),
               'TechnicalCandidate': {**technical, 'PayloadBindings': sorted(technical['PayloadBindings'], key=lambda row: row['Path'])}}
    return hashlib.sha256(json.dumps(subject, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode('utf-8')).hexdigest()


def verify_classic_review(path, expected, manifest_path, root, current, evidence):
    if not path or not isinstance(expected, str) or not manifest_path or digest(path) != expected.lower():
        raise Hold('Independent Classic decision, matching digest and reviewed manifest required')
    review, manifest = load(path), load(manifest_path)
    validate_classic_outcome(review, manifest, root, current, evidence)


def validate_classic_outcome(review, manifest, root, current, evidence):
    fields = {'ContractVersion', 'EvidenceClass', 'TaskId', 'ArtifactClass', 'TransitionType',
              'HandoffSHA256', 'ReviewActorClass', 'SelfApproval', 'ReviewOutcome',
              'ClassicFocusedDeltaReviewOutcome', 'ReviewedPostimageIdentity', 'ReviewedCommitIdentity'}
    if not isinstance(review, dict) or set(review) != fields:
        raise Hold('Independent Classic outcome contract required; test/fixture markers are forbidden')
    contracts = {'TECHNICAL_IMPLEMENTATION': ('IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW', 'NOT_APPLICABLE'),
                 'FINDING_CORRECTION': ('BUNDLED_CORRECTION_TO_FOCUSED_DELTA_REVIEW', 'PASS')}
    expected_contract = contracts.get(review['ArtifactClass']) if isinstance(review['ArtifactClass'], str) else None
    if (review['ContractVersion'] != '1.0.0' or review['EvidenceClass'] != 'CLASSIC_REVIEW_OUTCOME'
            or review['ReviewActorClass'] != 'CHATGPT_CLASSIC' or review['SelfApproval'] is not False
            or review['ReviewOutcome'] != 'PASS' or expected_contract is None
            or (review['TransitionType'], review['ClassicFocusedDeltaReviewOutcome']) != expected_contract
            or review['ReviewedCommitIdentity'] is not None
            or not isinstance(review['HandoffSHA256'], str) or not re.fullmatch('[0-9a-f]{64}', review['HandoffSHA256'])):
        raise Hold('Independent Classic PASS required; synthetic or self approval is not a review')
    if (set(manifest) != {'ContractVersion', 'TaskId', 'Entries', 'TechnicalCandidate'}
            or manifest['ContractVersion'] != '1.0.0' or not isinstance(manifest['TaskId'], str)
            or not re.fullmatch(r'[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)*-[0-9]+(?:-[A-Z0-9]+)*', manifest['TaskId'])
            or manifest['TaskId'] != review['TaskId'] or not isinstance(manifest['TechnicalCandidate'], dict)):
        raise Hold('Reviewed work-item identity differs')
    technical = manifest['TechnicalCandidate']
    if (set(technical) != {'Scope', 'BaseBindings', 'DependencyBindings', 'PayloadBindings'}
            or technical['Scope'] != 'FULL_WORK_ITEM:' + review['TaskId']
            or not all(isinstance(value, list) for value in (manifest['Entries'], technical['BaseBindings'],
                                                            technical['DependencyBindings'], technical['PayloadBindings']))):
        raise Hold('Complete reviewed work-item scope required')
    # The actual reviewed manifest must cover this complete current checkout.
    # Current bytes/modes come from the fresh binding, not caller assertions.
    def key(path):
        if not isinstance(path, str) or not Path(path).is_absolute() or '..' in Path(path).parts:
            raise Hold('Invalid reviewed source path')
        return os.path.normcase(os.path.abspath(path))
    actual = {}
    for row in current['sources']:
        if len(row) != 3 or row[1] == 'DELETED':
            raise Hold('Complete current source inventory required')
        actual[key(str(root / row[0]))] = (row[1], row[2])
    if not actual:
        raise Hold('Empty reviewed checkout')
    supplied = {}
    for row in manifest['Entries'] + technical['DependencyBindings']:
        if not isinstance(row, dict) or not isinstance(row.get('SHA256'), str):
            raise Hold('Invalid reviewed source binding')
        entry = row in manifest['Entries']
        fields = {'Path', 'SHA256', 'FileType', 'Mode', 'PackagePath'} if entry else {'Path', 'SHA256'}
        if set(row) != fields and not (entry and set(row) == fields | {'TargetPath'}):
            raise Hold('Unexpected reviewed source fields')
        name = key(row.get('TargetPath', row.get('Path')))
        if name in supplied or name not in actual or row['SHA256'] != actual[name][0]:
            raise Hold('Reviewed source missing, duplicated, outside checkout or changed')
        if entry and (row.get('FileType') != 'REGULAR_FILE'
                or type(row.get('Mode')) is not int or row['Mode'] != actual[name][1]):
            raise Hold('Reviewed source type or mode differs')
        supplied[name] = row['SHA256']
    if set(supplied) != set(actual):
        raise Hold('Review does not cover the complete checkout')
    for rows, fields in ((technical['BaseBindings'], {'Path', 'SHA256'}),
                         (technical['PayloadBindings'], {'Path', 'SHA256', 'Length'})):
        if any(not isinstance(row, dict) or set(row) != fields for row in rows):
            raise Hold('Unexpected reviewed base or payload fields')
    if not any(isinstance(row, dict) and row.get('SHA256') == digest(evidence)
               and row.get('Length') == Path(evidence).stat().st_size for row in technical['PayloadBindings']):
        raise Hold('Actual validation evidence is absent from the reviewed payload')
    try:
        candidate = reviewed_candidate_identity(manifest)
    except (KeyError, TypeError) as error:
        raise Hold('Invalid reviewed candidate manifest') from error
    if review['ReviewedPostimageIdentity'] != candidate:
        raise Hold('Independent review does not bind this candidate and scope')


def native_config(path):
    config = load(path)
    if set(config) - {'source_root'} != {'distribution', 'user', 'task_root', 'go', 'lint', 'pwsh'}:
        raise Hold('Native config needs distribution, user, task_root, go, lint, pwsh only')
    for key in ('task_root', 'go', 'lint', 'pwsh', *(['source_root'] if 'source_root' in config else [])):
        if not isinstance(config[key], str) or not config[key].startswith('/home/') or '..' in Path(config[key]).parts:
            raise Hold('Native paths must be explicit native /home paths')
    return config


def prepare_native(root, run, config, env):
    request, result = run / 'snapshot.request.json', run / 'snapshot.result.json'
    archive, manifest = run / 'source.tar', run / 'source.manifest.json'
    write(request, {'operation': 'snapshot', 'directory': str(root), 'archive': str(archive), 'manifest': str(manifest)})
    completed = subprocess.run([shutil.which('pwsh', path=env['PATH']), '-NoProfile', '-NonInteractive', '-File',
                                str(root / 'scripts/validation-process.ps1'), '-RequestPath', str(request), '-ResultPath', str(result)],
                               env=env, capture_output=True, timeout=120, check=False)
    if completed.returncode or load(result).get('status') != 'PASS':
        raise Hold('Existing source snapshot producer failed')
    task = dict(config)
    task.update(archive=wsl_path(archive), manifest=wsl_path(manifest), archive_sha256=digest(archive),
                manifest_sha256=digest(manifest), export_root=wsl_path(run / 'native-evidence'),
                git_dir=wsl_path(git(root, 'rev-parse', '--absolute-git-dir').decode().strip()),
                base=git(root, 'rev-parse', 'HEAD').decode().strip())
    path = run / 'native.request.json'
    write(path, task)
    return ['C:/Windows/System32/wsl.exe', '--distribution', config['distribution'], '--user', config['user'],
            '--cd', config['task_root'], '--exec', '/usr/bin/python3', '-B',
            wsl_path(root / 'scripts/validation.py'), '--native-stage', wsl_path(path)]


def native_stage(path):
    request = load(path)
    task = safe_directory(request['task_root'])
    if task.stat().st_uid != os.getuid() or not str(task).startswith('/home/'):
        raise Hold('Native task ownership or root mismatch')
    # Bind the actual longest mount; /mnt and network-backed sources are not
    # authoritative Linux validation filesystems.
    mounts = [line.split() for line in Path('/proc/mounts').read_text().splitlines()]
    mount = max((row for row in mounts if task == Path(row[1]) or Path(row[1]) in task.parents), key=lambda row: len(row[1]))
    if mount[2] not in ('ext4', 'xfs', 'btrfs'):
        raise Hold('Native filesystem not proven')
    if digest(request['archive']) != request['archive_sha256'] or digest(request['manifest']) != request['manifest_sha256']:
        raise Hold('Native source transport drift')
    reuse = 'source_root' in request
    if reuse:
        source = safe_directory(request['source_root'])
        if task not in source.parents or source.stat().st_uid != os.getuid():
            raise Hold('Existing native source is outside the owned task root')
        stage = safe_directory(source.parent)
        state = load(stage / 'source-state.json')
        head = git(source, 'rev-parse', 'HEAD').decode().strip()
        if state.get('base') != request['base'] or head != request['base']:
            raise Hold('Existing native source base changed')
        if {row[0]: row[1] for row in sources(source)} != state.get('sources'):
            raise Hold('Existing native source changed outside the validator')
    else:
        stage = task / ('validation-' + uuid.uuid4().hex[:12])
        stage.mkdir()
        source = stage / 'source'
    work, extracted = stage / 'work', stage / ('verified-' + uuid.uuid4().hex[:10])
    work.mkdir(exist_ok=True)
    safe_directory(work)
    spec = importlib.util.spec_from_file_location('snapshot', Path(__file__).with_name('validate-snapshot.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    import tarfile
    manifest = module.load_manifest(Path(request['manifest']))
    with tarfile.open(request['archive'], 'r:') as archive:
        members = module.validate_archive(archive, manifest)
        module.extract_regular_files(archive, members, extracted)
    module.verify_extracted(extracted, manifest)
    # This is an isolated validation checkout, never the leading repository.
    # Callers must authorize creating its local Git context before execution.
    clone_env = os.environ.copy()
    clone_env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL='/dev/null', GIT_TERMINAL_PROMPT='0', GIT_OPTIONAL_LOCKS='0')
    if not reuse:
        subprocess.run(['/usr/bin/git', '-c', 'core.hooksPath=/dev/null', 'clone', '--no-hardlinks', '--no-checkout',
                        '--quiet', request['git_dir'], str(source)], env=clone_env, check=True, timeout=120)
        subprocess.run(['/usr/bin/git', '-c', 'core.hooksPath=/dev/null', '-C', str(source), 'checkout', '--detach',
                        '--quiet', request['base']], env=clone_env, check=True, timeout=120)
    if reuse and set(state['sources']) - set(manifest):
        raise Hold('Existing native source has files absent from the new snapshot')
    module.overlay_verified_files(extracted, source, manifest)
    actual = {row[0]: row[1] for row in sources(source)}
    if actual != {name: value[1] for name, value in manifest.items()}:
        raise Hold('Native checkout does not exactly match the source snapshot')
    # Retain every previous state. The current pointer changes only after the
    # exact verified overlay, never after a failed or ambiguous preparation.
    state_record = dict(schema=1, base=request['base'], sources=actual)
    state_path = stage / ('source-state-' + uuid.uuid4().hex[:10] + '.json')
    write(state_path, state_record)
    (stage / 'source-state.json').write_bytes(state_path.read_bytes())
    env = environment(work)
    directories = [str(Path(request[key]).parent) for key in ('go', 'lint', 'pwsh')]
    for key in ('go', 'lint', 'pwsh'):
        if not Path(request[key]).is_file():
            raise Hold('Native tool missing: ' + key)
    env.update(PATH=os.pathsep.join(directories + [os.environ['PATH']]))
    previous_summaries = set(work.glob('validation-*/summary.json'))
    result = subprocess.run([sys.executable, '-B', str(source / 'scripts/validation.py'), '--mode', 'CANDIDATE',
                             '--work-root', str(work)], cwd=source, env=env, check=False)
    summaries = list(set(work.glob('validation-*/summary.json')) - previous_summaries)
    if len(summaries) != 1:
        raise Hold('Native candidate summary missing or ambiguous')
    proof = load(summaries[0])
    export = Path(request['export_root'])
    safe_directory(export.parent)
    if export.exists() or export.is_symlink():
        raise Hold('Native export must be fresh')
    shutil.copytree(summaries[0].parent, export)
    write(export / 'native-context.json', {'source': str(source), 'work': str(work), 'tools': directories})
    print(json.dumps({'status': proof.get('status', 'HOLD'), 'native_root': str(stage),
                      'summary_sha256': digest(export / 'summary.json')}))
    return 0 if proof.get('status') == 'PASS' and result.returncode == 0 else 2


def native_verify(context_path, summary_path):
    context = load(context_path)
    source, work = safe_directory(context['source']), safe_directory(context['work'])
    env = environment(work, create=False)
    env['PATH'] = os.pathsep.join(context['tools'] + [os.environ['PATH']])
    proof = load(summary_path)
    if proof.get('status') != 'PASS' or binding(source, env) != proof.get('binding'):
        raise Hold('Native source, toolchain or environment drifted')
    print(json.dumps({'status': 'PASS', 'quality_processes_started': 0}))
    return 0


def check_native_reuse(root, summary, config, env):
    base = Path(summary).resolve().parent / 'native-evidence'
    command = ['C:/Windows/System32/wsl.exe', '--distribution', config['distribution'], '--user', config['user'],
               '--cd', config['task_root'], '--exec', '/usr/bin/python3', '-B',
               wsl_path(root / 'scripts/validation.py'), '--native-verify',
               wsl_path(base / 'native-context.json'), wsl_path(base / 'summary.json')]
    completed = subprocess.run(command, env=env, capture_output=True, check=False, timeout=90)
    if completed.returncode:
        raise Hold('Native evidence is no longer current')
    try:
        actual = json.loads(completed.stdout)
    except ValueError as error:
        raise Hold('Native verification result unknown') from error
    if actual != {'status': 'PASS', 'quality_processes_started': 0}:
        raise Hold('Native verification result invalid')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=['FAST', 'CANDIDATE', 'PUBLISH'], default='FAST')
    parser.add_argument('--work-root', required=True, help='Existing external task-owned root; never OS temp')
    parser.add_argument('--resource-root', help='Optional existing external resource directory; never selects the checkout lock domain')
    parser.add_argument('--changed', action='append', help='Explicit relative path, may repeat')
    parser.add_argument('--evidence', help='Existing candidate summary.json')
    parser.add_argument('--evidence-sha256', help='Digest retained outside the evidence being verified')
    parser.add_argument('--review', help='Independent Classic decision JSON; PUBLISH only')
    parser.add_argument('--review-sha256', help='Digest of the actual independent decision')
    parser.add_argument('--review-manifest', help='Actual content-bound manifest covered by the independent Classic outcome')
    parser.add_argument('--native-config', help='Explicit native task root and existing tool paths; Windows candidate requires it')
    parser.add_argument('--plan-only', action='store_true')
    args = parser.parse_args(argv)
    root = Path(__file__).resolve().parent.parent
    work = safe_directory(args.work_root)
    if work == root or root in work.parents:
        raise Hold('Work root must be outside source')
    resource = safe_directory(args.resource_root or work)
    env = environment(work, create=args.mode != 'PUBLISH')
    hygiene = source_hygiene(root)
    current = binding(root, env)
    config = native_config(args.native_config) if args.native_config else None
    if os.name == 'nt' and args.mode in ('CANDIDATE', 'PUBLISH') and not config:
        raise Hold('Complete Windows candidate needs --native-config for the required native Linux gates')
    if config:
        current['native_config'] = config
    mode = 'CANDIDATE' if args.mode == 'PUBLISH' else args.mode
    changes = args.changed if args.changed is not None else changed_paths(root)
    run = work / ('validation-' + uuid.uuid4().hex[:12])
    # Bind stable artifact paths to the original run for evidence verification.
    plan_work = Path(args.evidence).resolve().parent if args.evidence else run
    steps = plan(mode, changes, root, plan_work, current['tools'])
    if config and mode == 'CANDIDATE':
        steps.append({'name': 'native-linux', 'argv': ['native-config', identity(config)], 'heavy': True})
    if args.plan_only:
        print(json.dumps({'mode': args.mode, 'binding_sha256': identity(current), 'plan': steps}, indent=2))
        return 0
    if args.evidence:
        verify_evidence(args.evidence, args.evidence_sha256, current, steps)
        if config:
            check_native_reuse(root, args.evidence, config, env)
        if args.mode == 'PUBLISH':
            verify_classic_review(args.review, args.review_sha256, args.review_manifest, root, current, args.evidence)
        print(json.dumps({'status': 'PASS', 'reused': True, 'quality_processes_started': 0,
                          'binding_sha256': identity(current), 'external_operations_authorized': False}))
        return 0
    if args.mode == 'PUBLISH':
        raise Hold('PUBLISH requires candidate evidence; it never launches a test matrix')
    run.mkdir()
    results = []
    status = 'FAIL'
    try:
        # Retain the work root's caches between runs; outputs are always fresh.
        for step in steps:
            observed = binding(root, env)
            if config:
                observed['native_config'] = config
            if observed != current:
                raise Hold('Source/toolchain/environment changed during validation')
            print(json.dumps({'status': 'RUNNING', 'gate': step['name']}), flush=True)
            guard = heavy_lock(resource, root) if step['heavy'] else contextlib.nullcontext()
            with guard:
                if step['heavy']:
                    assert_no_competing_go()
                command = prepare_native(root, run, config, env) if step['name'] == 'native-linux' else step['argv']
                result = run_process(root, run, step['name'], command, env,
                                     timeout=10800 if step['name'] == 'native-linux' else 1800)
                if step['name'] == 'native-linux' and result.get('timed_out'):
                    result['status'] = 'HOLD'  # Killing wsl.exe proves no Linux descendant termination.
                if step['name'] == 'native-linux':
                    native_summary = run / 'native-evidence/summary.json'
                    native_state = load(native_summary).get('status') if native_summary.is_file() else 'HOLD'
                    if native_state != 'PASS':
                        result['status'] = native_state if native_state in ('FAIL', 'HOLD') else 'HOLD'
            result['name'] = step['name']
            results.append(result)
            if result['status'] != 'PASS':
                status = result['status']
                break
        else:
            observed = binding(root, env)
            if config:
                observed['native_config'] = config
            if observed != current:
                raise Hold('Candidate drifted during validation')
            hygiene = source_hygiene(root)
            status = 'PASS'
    except (Hold, subprocess.TimeoutExpired, KeyboardInterrupt) as error:
        status = 'HOLD'
        results.append({'name': 'coordinator', 'status': status, 'error': type(error).__name__ + ': ' + str(error)})
    finally:
        summary = run / 'summary.json'
        write(summary, {'schema': 1, 'status': status, 'mode': args.mode, 'platform': platform.system(),
                        'binding': current, 'plan': steps, 'results': results,
                        'artifacts': artifact_inventory(run), 'binding_sha256': identity(current),
                        'source_hygiene': hygiene,
                        'matrix_scope': 'local-platform; Windows and native Linux evidence both required for handoff'})
        print(json.dumps({'status': status, 'report_path': str(summary), 'sha256': digest(summary)}), flush=True)
    return 0 if status == 'PASS' else 2


if __name__ == '__main__':
    try:
        if len(sys.argv) == 3 and sys.argv[1] == '--native-stage':
            raise SystemExit(native_stage(sys.argv[2]))
        if len(sys.argv) == 4 and sys.argv[1] == '--native-verify':
            raise SystemExit(native_verify(sys.argv[2], sys.argv[3]))
        raise SystemExit(main())
    except (OSError, ValueError, Hold, subprocess.SubprocessError) as error:
        print(json.dumps({'status': 'HOLD', 'error': type(error).__name__ + ': ' + str(error)}))
        raise SystemExit(2)
