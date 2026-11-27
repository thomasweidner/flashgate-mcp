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
import tempfile
import time
import uuid


SMOKE_BUILD_LINUX = 'build/flashgate-mcp'
SMOKE_BUILD_WINDOWS = 'build/flashgate-mcp.exe'
BUILD_SMOKE_GATES = ('build', 'smoke', 'smoke-read', 'smoke-negative', 'startup-negative')
SHELL_MARKDOWN = {'README.md', 'CHANGELOG.md'}
NATIVE_SHELL_FIXED = ('cut', 'grep', 'iconv', 'od', 'sha256sum', 'sort', 'tail', 'timeout', 'tr')
NATIVE_SHELL_SEARCHED = ('awk', 'cat', 'date', 'dirname', 'env', 'grep', 'ln', 'mkdir', 'mktemp',
                         'python3', 'rm', 'rmdir')
NATIVE_SHELL_BIN = Path('/usr/bin')
NATIVE_RUNTIME_ROOTS = (Path('/usr/lib'), Path('/usr/local/lib'))
NATIVE_SHELL_LOADERS = {
    'x86_64': Path('/lib64/ld-linux-x86-64.so.2'),
    'aarch64': Path('/lib/ld-linux-aarch64.so.1'),
}


class Hold(RuntimeError):
    """An unproven binding or process state must not become PASS."""


class ResourceHold(Hold):
    """A protected resource was busy before the next gate started."""


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def identity(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def load(path):
    return parse_json_object(Path(path).read_text(encoding='utf-8'))


def parse_json_object(text):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise Hold('Duplicate JSON property')
            result[key] = value
        return result
    value = json.loads(text, object_pairs_hook=unique)
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


def git(root, *args, stdin=None):
    result = subprocess.run(['git', '-C', str(root), *args], check=True,
                            input=stdin, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
    return result.stdout


def source_parts(name):
    parts = name.split('/')
    if any(part in ('', '.', '..') or part.casefold() == '.git' for part in parts):
        raise Hold('Invalid Git source path')
    if os.name == 'nt' and any(':' in part or '\\' in part or part.rstrip(' .') != part for part in parts):
        raise Hold('Invalid Windows Git source path')
    return parts


def source_boundary(root):
    if os.name == 'nt':
        safe_directory(root)
        return contextlib.nullcontext(None)
    return checkout_descriptor(root)


@contextlib.contextmanager
def source_parent(root, root_fd, parts):
    if root_fd is None:
        parent = root
        for part in parts:
            parent /= part
            meta = parent.lstat()
            if not stat.S_ISDIR(meta.st_mode) or getattr(meta, 'st_file_attributes', 0) & 0x400:
                raise Hold('Source parent is not a link-free directory')
        yield parent
        return
    with contextlib.ExitStack() as descriptors:
        parent = root_fd
        for part in parts:
            try:
                parent = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                                 dir_fd=parent)
            except FileNotFoundError:
                raise
            except OSError as error:
                raise Hold('Source parent is not a link-free directory') from error
            descriptors.callback(os.close, parent)
        yield parent


def source_stat(parent, leaf):
    return ((parent / leaf).lstat() if isinstance(parent, Path)
            else os.stat(leaf, dir_fd=parent, follow_symlinks=False))


def verify_windows_source_handle(descriptor, requested):
    import ctypes
    import msvcrt
    from ctypes import wintypes
    lookup = ctypes.WinDLL('kernel32', use_last_error=True).GetFinalPathNameByHandleW
    lookup.argtypes = (wintypes.HANDLE, wintypes.LPWSTR, wintypes.DWORD, wintypes.DWORD)
    lookup.restype = wintypes.DWORD
    handle = msvcrt.get_osfhandle(descriptor)
    size = lookup(handle, None, 0, 0)
    if not size:
        raise Hold('Cannot verify opened Windows source path')
    buffer = ctypes.create_unicode_buffer(size)
    length = lookup(handle, buffer, size, 0)
    if not length or length >= size:
        raise Hold('Cannot verify opened Windows source path')
    final = buffer.value
    if final.startswith('\\\\?\\UNC\\'):
        final = '\\\\' + final[8:]
    elif final.startswith('\\\\?\\'):
        final = final[4:]
    if os.path.normcase(final) != os.path.normcase(os.path.abspath(requested)):
        raise Hold('Opened Windows source resolved to a different path')


@contextlib.contextmanager
def source_file(root, root_fd, name):
    parts = source_parts(name)
    with source_parent(root, root_fd, parts[:-1]) as parent:
        before = source_stat(parent, parts[-1])
        if not stat.S_ISREG(before.st_mode) or getattr(before, 'st_file_attributes', 0) & 0x400:
            raise Hold('Nonregular source input')
        if root_fd is None:
            requested = parent / parts[-1]
            descriptor = os.open(requested, os.O_RDONLY | os.O_BINARY)
        else:
            descriptor = os.open(parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
                                 dir_fd=parent)
        try:
            opened = os.fstat(descriptor)
            if not stat.S_ISREG(opened.st_mode) or not os.path.samestat(before, opened):
                raise Hold('Source file changed while opening')
            if root_fd is None:
                verify_windows_source_handle(descriptor, requested)
            with os.fdopen(descriptor, 'rb', closefd=False) as stream:
                yield stream, stat.S_IMODE(opened.st_mode)
        finally:
            os.close(descriptor)


def sources(root, *, check_ignored=False, shell_markdown=None):
    names = set(git(root, 'ls-files', '-z').decode().split('\0'))
    names.update(git(root, 'ls-files', '--others', '--exclude-standard', '-z').decode().split('\0'))
    records = []
    with source_boundary(root) as root_fd:
        if check_ignored:
            ignored = set(git(root, 'ls-files', '--others', '--ignored', '--exclude-standard', '-z').split(b'\0'))
            ignored.discard(b'')
            outputs = {SMOKE_BUILD_LINUX.encode(), SMOKE_BUILD_WINDOWS.encode()}
            if ignored - outputs:
                raise Hold('Ignored checkout files may affect candidate evidence; move them outside the checkout')
            for name in sorted(ignored):
                # Smoke uses fresh build outputs. Their existing paths still
                # need the same regular-file and link boundaries as sources.
                with source_file(root, root_fd, name.decode('ascii')):
                    pass
        for name in sorted(names - {''}):
            # A missing path is outside the postimage. Git deletions are
            # validated separately against the historical tree.
            try:
                with source_file(root, root_fd, name) as (stream, mode):
                    hasher = hashlib.sha256()
                    if shell_markdown is not None and name in SHELL_MARKDOWN:
                        prefix = stream.read(2)
                        if prefix == b'# ':
                            shell_markdown.append(name)
                        hasher.update(prefix)
                    records.append([name, hashlib.file_digest(stream, lambda: hasher).hexdigest(), mode])
            except FileNotFoundError:
                continue
    if not records:
        raise Hold('Empty source inventory')
    return records


def git_index(root):
    indexed = {}
    for record in git(root, 'ls-files', '--stage', '-z').split(b'\0'):
        if not record:
            continue
        metadata, path = record.split(b'\t', 1)
        mode, blob, stage = metadata.decode('ascii').split()
        if stage != '0':
            raise Hold('Unmerged Git index entry in source inventory')
        indexed[path.decode('utf-8')] = (mode, blob)
    return indexed


def git_modes(root, postimage):
    indexed = git_index(root)
    honor_executable = git(root, 'config', '--type=bool', '--default=true',
                           '--get', 'core.filemode').strip() == b'true'
    result = []
    for name, _, filesystem_mode in postimage:
        index_mode = indexed[name][0] if name in indexed else '100644'
        if index_mode not in ('100644', '100755'):
            raise Hold('Nonregular Git mode in source inventory: ' + name)
        # Predict the tested mode Git would stage. The separate Git postimage
        # check still detects an incompatible mode already staged or committed.
        mode = ('100755' if filesystem_mode & stat.S_IXUSR else '100644') if honor_executable else index_mode
        result.append([name, mode])
    return result


def project_mirror_blobs(mirror, git_dir, postimage):
    names = [row[0].encode('utf-8') for row in postimage]
    context = ('--git-dir=' + git_dir, '--work-tree=' + str(mirror))
    attrs = git(mirror, *context, 'check-attr', '--stdin', '-z', 'filter',
                stdin=b'\0'.join(names) + b'\0').split(b'\0')
    if (len(attrs) != 3 * len(names) + 1 or attrs[-1] != b''
            or any(path != name or attribute != b'filter' or value not in (b'unspecified', b'unset')
                   for name, path, attribute, value in zip(names, attrs[::3], attrs[1::3], attrs[2::3]))):
        raise Hold('Source Git blob projection requires paths without external clean filters')
    # hash-object expects LF-delimited, C-quoted paths. It applies Git's
    # built-in clean representation without writing blob objects.
    quoted = [b'"' + name.replace(b'\\', b'\\\\').replace(b'"', b'\\"')
              .replace(b'\r', b'\\r').replace(b'\n', b'\\n') + b'"\n' for name in names]
    # check-attr uses the same output for sentinel values and literal driver
    # names. Disable those two driver names and require Git to reject their use.
    filter_guards = [arg for name in ('unspecified', 'unset')
                     for arg in ('-c', f'filter.{name}.clean=', '-c', f'filter.{name}.process=',
                                 '-c', f'filter.{name}.required=true')]
    try:
        blobs = git(mirror, *context, *filter_guards, 'hash-object', '--stdin-paths',
                    stdin=b''.join(quoted)).decode('ascii').splitlines()
    except subprocess.CalledProcessError as error:
        raise Hold('Git could not project source blobs safely') from error
    if len(blobs) != len(names) or any(not re.fullmatch(r'[0-9a-f]{40}|[0-9a-f]{64}', oid) for oid in blobs):
        raise Hold('Git blob projection returned an invalid inventory')
    return [[row[0], oid] for row, oid in zip(postimage, blobs)]


def git_blobs(root, postimage, work):
    work = safe_directory(work)
    root = Path(os.path.abspath(root))
    if work == root or root in work.parents:
        raise Hold('Git projection scratch must be outside source')
    names = {record[0] for record in postimage}
    attribute_paths = {'.gitattributes'}
    for name in names:
        parts = source_parts(name)
        attribute_paths.update('/'.join(parts[:depth] + ['.gitattributes'])
                               for depth in range(1, len(parts)))
    verify_absent(root, attribute_paths - names, 'Unbound Git attribute file')
    git_dir = git(root, 'rev-parse', '--absolute-git-dir').decode().strip()
    # Git's batch interface opens paths itself. Only give it a private copy
    # of bytes obtained through the same guarded reader as the inventory.
    with tempfile.TemporaryDirectory(prefix='v-', dir=work) as scratch:
        mirror = Path(scratch)
        with source_boundary(root) as root_fd:
            for name, expected, mode in postimage:
                target = mirror.joinpath(*source_parts(name))
                target.parent.mkdir(parents=True, exist_ok=True)
                with source_file(root, root_fd, name) as (stream, actual_mode):
                    with target.open('xb') as output:
                        shutil.copyfileobj(stream, output)
                if actual_mode != mode or digest(target) != expected:
                    raise Hold('Source changed before Git blob projection: ' + name)
        return project_mirror_blobs(mirror, git_dir, postimage)


def git_tree(root, commit):
    records = {}
    for record in git(root, 'ls-tree', '-r', '-z', commit).split(b'\0'):
        if record:
            metadata, path = record.split(b'\t', 1)
            mode, _, blob = metadata.decode('ascii').split()
            records[path.decode('utf-8')] = (mode, blob)
    return records


def verify_git_postimage(root, current, base):
    modes = dict(current['git_modes'])
    target = {path: (modes[path], blob) for path, blob in current['git_blobs']}
    head = git(root, 'rev-parse', 'HEAD').decode().strip()
    committed = git_tree(root, head)
    if head != base and committed != target:
        raise Hold('Git commit differs from the tested postimage')
    indexed = git_index(root)
    # Before staging, a path may still have its entry from the bound base.
    # Any other stage-zero entry must be the tested Git representation.
    for path in sorted(indexed.keys() | target.keys() | committed.keys()):
        if indexed.get(path) not in (target.get(path), committed.get(path)):
            raise Hold('Git index differs from the tested postimage: ' + path)


def tracked_at(root, base):
    return set(git(root, 'ls-tree', '-r', '--name-only', '-z', base).decode().split('\0')) - {''}


def verify_absent(root, names, description='A source expected to be deleted'):
    with source_boundary(root) as root_fd:
        for name in names:
            parts = source_parts(name)
            try:
                with source_parent(root, root_fd, parts[:-1]) as parent:
                    source_stat(parent, parts[-1])
            except FileNotFoundError:
                continue
            raise Hold(description + ' still exists: ' + name)


def deleted_sources(root, base, postimage):
    names = sorted(tracked_at(root, base) - {row[0] for row in postimage})
    verify_absent(root, names)
    return names


def verify_source_postimage(root, current, proof):
    base = proof.get('source_base')
    if not isinstance(base, str) or not re.fullmatch('[0-9a-f]{40,64}', base):
        raise Hold('Evidence has no valid historical source base')
    if deleted_sources(root, base, current['sources']) != proof.get('deleted_sources'):
        raise Hold('Evidence deletion inventory differs')
    verify_git_postimage(root, current, base)


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


def native_shell_paths(search):
    paths = {}
    for name in NATIVE_SHELL_FIXED:
        path = NATIVE_SHELL_BIN / name
        paths['shell:fixed:' + name] = str(path) if path.is_file() and os.access(path, os.X_OK) else None
    for name in NATIVE_SHELL_SEARCHED:
        paths['shell:PATH:' + name] = shutil.which(name, path=search) if search is not None else None
    return paths


def native_runtime_file(path):
    original = Path(path)
    resolved = original.resolve()
    # Debian-derived systems may store the standard startup file under /etc.
    startup = (re.fullmatch(r'/usr/lib/python\d+\.\d+/sitecustomize\.py', str(original))
               and resolved == Path('/etc') / original.parent.name / original.name)
    if not original.is_absolute() or not (startup or any(resolved.is_relative_to(root) for root in NATIVE_RUNTIME_ROOTS)):
        raise Hold('Native shell runtime path outside the supported system layout')
    return resolved


def native_shell_loader():
    loader = NATIVE_SHELL_LOADERS.get(platform.machine())
    if loader is None:
        return None
    resolved = native_runtime_file(loader)
    return loader if resolved.is_file() and os.access(resolved, os.X_OK) else None


def native_shell_runtime(root, env, helpers, tools):
    # Probe only the declared system programs used by the native shell gates.
    # The glibc loader lists their resolved objects without running the helpers.
    paths = [helpers.get('bash'), tools.get('git', {}).get('path'),
             *(value for key, value in helpers.items() if key.startswith(('shell:fixed:', 'shell:PATH:')))]
    search = env.get('PATH', '')
    overrides = ('BASH_ENV', 'ENV', 'GCONV_PATH', 'GIT_EXEC_PATH', 'LOCPATH',
                 'PYTHONHOME', 'PYTHONPATH', 'PYTHONSTARTUP', 'PYTHONUSERBASE')
    cache = Path('/etc/ld.so.cache')
    if (not cache.is_file() or os.path.lexists('/etc/ld.so.preload')
            or not all(Path(part).is_absolute() for part in search.split(os.pathsep))
            or any(env.get(key) for key in overrides)
            or any(value for key, value in env.items() if key.startswith(('LD_', 'BASH_FUNC_')))
            or not all(path and Path(path).resolve().parent == NATIVE_SHELL_BIN.resolve() for path in paths)):
        return None
    module_probe = '''import json, os, sys, encodings.utf_8
print(json.dumps([sys.path, sorted({p for m in sys.modules.values()
    for p in (getattr(m, '__file__', None), getattr(m, '__cached__', None)) if p})]))'''
    try:
        loader = native_shell_loader()
        if loader is None:
            return None
        runtime = {'shell:runtime:loader': str(loader), 'shell:runtime:cache': str(cache)}
        interpreter = helpers['shell:PATH:python3']
        python = subprocess.run([interpreter, '-B', '-c', module_probe], cwd=root, env=env,
                                capture_output=True, text=True, check=True, timeout=5)
        module_paths, modules = json.loads(python.stdout)
        extensions = set()
        for path in modules:
            source = native_runtime_file(path)
            runtime['shell:python:' + path] = str(source) if source.is_file() else None
            if source.suffix == '.so':
                extensions.add(source)
        for path in filter(None, module_paths):
            source = native_runtime_file(path)
            if source.is_dir():
                for entry in sorted(source.glob('*.pth')):
                    runtime['shell:python:' + str(entry)] = str(native_runtime_file(entry))
            elif source.suffix == '.zip':
                runtime['shell:python:' + path] = str(source) if source.is_file() else None
            else:
                raise Hold('Unresolved native Python import path')
        for program in sorted({Path(path).resolve() for path in paths} | extensions):
            output = subprocess.run([str(loader), '--list', str(program)], env={**env, 'LC_ALL': 'C'},
                                    capture_output=True, text=True, check=True, timeout=5).stdout
            if output.strip() == 'statically linked':
                # This also covers extension modules with no ELF NEEDED entries.
                runtime['shell:runtime:static:' + str(program)] = str(program)
                continue
            objects = []
            for line in output.splitlines():
                row = line.strip()
                if row.startswith('linux-vdso.so.'):
                    continue
                match = re.fullmatch(r'(?:[^ ]+ => )?(/.+) \(0x[0-9a-f]+\)', row)
                if not match:
                    raise Hold('Unresolved native shell loader dependency')
                library = native_runtime_file(match[1])
                if not library.is_file():
                    raise Hold('Missing native shell loader dependency')
                objects.append(library)
            if not objects:
                raise Hold('Native shell loader dependency list is empty')
            for library in objects:
                runtime['shell:runtime:' + str(program) + ':' + str(library)] = str(library)
    except (OSError, Hold, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        return None
    return runtime


def validation_helpers(root, env, tools):
    bash = Path(r'C:\Program Files\Git\bin\bash.exe') if os.name == 'nt' else Path('/usr/bin/bash')
    paths = {'bash': str(bash) if bash.is_file() else None}
    search = env.get('PATH')
    if search is not None:
        for name in ('gcc', 'g++', 'clang', 'clang++', 'cc', 'c++', 'ar', 'as', 'ld', 'pkg-config'):
            paths[name] = shutil.which(name, path=search)
        for key in ('CC', 'CXX', 'AR', 'PKG_CONFIG'):
            if value := env.get(key):
                paths[key] = shutil.which(value, path=search)
    if 'go' in tools:
        go_root = Path(env.get('GOROOT') or Path(tools['go']['path']).parent.parent)
        for directory in sorted((go_root / 'pkg' / 'tool').glob('*')):
            for name in ('compile', 'link', 'asm', 'cgo', 'vet', 'cover'):
                path = directory / (name + ('.exe' if os.name == 'nt' else ''))
                if path.is_file():
                    paths['go:' + directory.name + '/' + name] = str(path)
    if 'pwsh' in tools:
        directory = Path(tools['pwsh']['path']).parent
        for name in ('System.Management.Automation.dll', 'Microsoft.CodeAnalysis.dll',
                     'Microsoft.CodeAnalysis.CSharp.dll', 'pwsh.runtimeconfig.json'):
            path = directory / name
            paths['pwsh:' + name] = str(path) if path.is_file() else None
    if os.name == 'nt':
        directory = bash.parent.parent / 'usr' / 'bin'
        for name in ('bash.exe', 'msys-2.0.dll', 'msys-iconv-2.dll', 'msys-intl-8.dll'):
            path = directory / name
            paths['git-bash:' + name] = str(path) if path.is_file() else None
        system = Path(env.get('SYSTEMROOT') or env.get('SystemRoot') or 'C:/Windows') / 'System32'
        for name in ('ntdll.dll', 'kernel32.dll', 'ucrtbase.dll'):
            path = system / name
            paths[name] = str(path) if path.is_file() else None
    else:
        paths.update(native_shell_paths(search))
        runtime = native_shell_runtime(root, env, paths, tools)
        if runtime is not None:
            paths.update(runtime)
    resolved = {name: Path(path).resolve() if path else None for name, path in paths.items()}
    hashes = {path: digest(path) for path in set(resolved.values()) if path is not None}
    return {name: [str(path), hashes[path]] if path is not None else None for name, path in resolved.items()}


def binding(root, env, *, check_ignored=True):
    # Conservative whole-checkout binding: docs and tests are not silently
    # excluded. No commit binding: an unchanged commit/transport may change.
    names = {'PATH', 'PATHEXT', 'AR', 'CC', 'CXX', 'FC', 'GCCGO', 'GCCGOTOOLDIR',
             'PKG_CONFIG', 'PKG_CONFIG_PATH', 'PKG_CONFIG_LIBDIR', 'PKG_CONFIG_SYSROOT_DIR',
             'CFLAGS', 'CPPFLAGS', 'CXXFLAGS', 'FFLAGS', 'LDFLAGS', 'CPATH',
             'C_INCLUDE_PATH', 'CPLUS_INCLUDE_PATH', 'LIBRARY_PATH', 'COMPILER_PATH',
             'GCC_EXEC_PREFIX', 'SDKROOT', 'MACOSX_DEPLOYMENT_TARGET', 'INCLUDE', 'LIB', 'LIBPATH',
             'GIT_DIR', 'GIT_WORK_TREE', 'FLASHGATE_TASK_ROOT', 'FLASHGATE_WORK_ROOT',
             'TEMP', 'TMP', 'TMPDIR', 'SystemRoot', 'COMSPEC', 'PSModulePath',
             'DOTNET_ROOT', 'DOTNET_HOME', 'BASH_ENV', 'ENV', 'HOME', 'USERPROFILE',
             'MSYSTEM', 'LANG', 'LC_ALL', 'TZ', 'PYTHONPATH', 'PYTHONHOME'}
    names.update(('AWKPATH', 'AWKLIBPATH', 'BASHOPTS', 'SHELLOPTS', 'POSIXLY_CORRECT',
                  'GCONV_PATH', 'GIT_EXEC_PATH', 'LOCPATH'))
    # Include architecture, cgo security flags and future Go build settings.
    # Record hashes only, including the distinction between absent and empty.
    names.update(name for name in env if name.startswith(
        ('GO', 'CGO_', 'PKG_CONFIG_', 'LD_', 'DYLD_', 'LC_', 'DOTNET_', 'GIT_CONFIG_', 'PYTHON', 'BASH_FUNC_')))
    shell_markdown = []
    postimage = sources(root, check_ignored=check_ignored, shell_markdown=shell_markdown)
    tools = tools_binding(env)
    helpers = validation_helpers(root, env, tools)
    native_shell_reusable = os.name == 'nt' or 'shell:runtime:loader' in helpers
    return {'sources': postimage, 'git_modes': git_modes(root, postimage),
             'git_blobs': git_blobs(root, postimage, env.get('FLASHGATE_WORK_ROOT')), 'tools': tools,
             'helpers': helpers, 'shell_markdown': shell_markdown, 'native_shell_reusable': native_shell_reusable,
            'os': [platform.system(), platform.release(), platform.machine(), platform.version()],
            'python': [sys.version, digest(sys.executable)],
             'environment': {name: identity(env.get(name.upper() if os.name == 'nt' else name))
                             for name in sorted(names)},
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
            if state == 0x102:
                raise ResourceHold('Checkout heavy validation active')
            if state != 0:
                raise Hold('Checkout lock ownership unconfirmed')
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
        except BlockingIOError as error:
            raise ResourceHold('Checkout heavy validation active') from error
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
        raise ResourceHold('Competing Go process observed; no process was stopped')


def changed_paths(root):
    changed = git(root, 'diff', '--name-only', 'HEAD', '-z').decode().split('\0')
    changed += git(root, 'ls-files', '--others', '--exclude-standard', '-z').decode().split('\0')
    return sorted(set(changed) - {''})


def require_fast_selection(changed):
    if not changed or '' in changed:
        raise Hold('FAST needs a changed source or an explicit --changed path')


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
        if os.name == 'nt':
            add('documentation-wrapper', ps + ['scripts/Test-DocumentationConsistency.ps1'])
        else:
            add('documentation', [go, 'run', './cmd/doccheck'])
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
        require_fast_selection(changed)
        # File changes with unknown dependency reach select the full suite.
        go_changes = [p for p in changed if p.endswith('.go')]
        safe = go_changes and all(p.endswith('_test.go') for p in go_changes) and len(go_changes) == len(changed)
        packages = sorted({'./' + str(Path(p).parent).replace('\\', '/') for p in go_changes}) if safe else ['./...']
        add('focused-tests', [go, 'test', '-json', *packages], True)
        return steps
    add('format', [lint, 'fmt', '--diff'])
    add('vet', [go, 'vet', './...'])
    add('lint', [lint, 'run'])
    add('coverage', ps + ['scripts/Test-GoCoverage.ps1', '-PlatformName', host,
                         '-MinimumCoverage', '90', '-OutputRoot', str(work / 'coverage')], True)
    if os.name != 'nt':
        add('race', [go, 'test', '-race', '-json', './...'], True)
    add('mcp', [go, 'run', '-mod=vendor', './cmd/mcpcheck', '-report', str(work / 'mcp.md'),
                '-coverage-evidence', str(work / coverage_log(host))], True)
    binary = root / (SMOKE_BUILD_WINDOWS if os.name == 'nt' else SMOKE_BUILD_LINUX)
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


def verify_evidence(path, expected, current, steps, *, root, native_probe=None):
    with admitted_summary(path, expected) as (proof, raw, base, root_fd):
        verify_evidence_results(proof, current, steps)
        verify_resume_origins(proof, base, root_fd)
        if native_probe:
            native_probe(path, proof)
    verify_source_postimage(root, current, proof)
    return hashlib.sha256(raw).hexdigest(), len(raw)


@contextlib.contextmanager
def admitted_summary(path, expected):
    if not expected:
        raise Hold('Evidence requires an independently retained matching SHA-256')
    path = Path(path).absolute()
    base = path.parent
    with source_boundary(base) as root_fd:
        with evidence_file(base, root_fd, path.name) as stream:
            summary_bytes = stream.read()
        summary_digest = hashlib.sha256(summary_bytes).hexdigest()
        if summary_digest != expected.lower():
            raise Hold('Evidence requires an independently retained matching SHA-256')
        proof = parse_json_object(summary_bytes.decode('utf-8'))
        if evidence_inventory(base, root_fd) != proof.get('artifacts'):
            raise Hold('Evidence artifact missing or manipulated, or inventory changed')
        yield proof, summary_bytes, base, root_fd


def successful_result(row):
    return (row.get('status') == 'PASS' and type(row.get('exit_code')) is int
            and row['exit_code'] == 0 and row.get('timed_out') is False
            and not row.get('output_limit') and row.get('failure_reason') in ('', 'None', None))


def verify_evidence_results(proof, current, steps):
    if proof.get('schema') != 1 or proof.get('status') != 'PASS' or proof.get('binding') != current:
        raise Hold('Evidence failed, incomplete or invalidated')
    if proof.get('plan') != steps:
        raise Hold('Evidence gate/argument scope differs')
    if current.get('native_shell_reusable') is False and any(step['name'] == 'shell' for step in steps):
        raise Hold('Native shell helper/runtime binding incomplete')
    results = proof.get('results', [])
    if not isinstance(results, list) or not all(isinstance(row, dict) for row in results):
        raise Hold('Invalid result matrix')
    if [r.get('name') for r in results] != [s['name'] for s in steps]:
        raise Hold('Evidence matrix incomplete')
    for result in results:
        if not successful_result(result):
            raise Hold('Unsuccessful evidence cannot be reused')
    if not proof.get('artifacts'):
        raise Hold('Evidence has no artifacts')


def phase_binding(binding, gate):
    if gate == 'native-linux':
        # Windows toolchain and process environment do not identify the
        # separately probed native execution. Keep the transport and Git subject.
        return {k: binding[k] for k in ('sources', 'git_modes', 'git_blobs', 'native_config')}
    local = {k: v for k, v in binding.items() if k != 'native_config'}
    if gate == 'shell':
        # Both validators examine first lines for shebangs, even in Markdown.
        # The guarded '# ' prefix rules out that discovery on either platform.
        documents = SHELL_MARKDOWN.intersection(local.get('shell_markdown', []))
        local['sources'] = [[n, None if n in documents else h, m] for n, h, m in local['sources']]
        local['git_blobs'] = [[n, None if n in documents else oid] for n, oid in local['git_blobs']]
    return local


def resume_step(step, run):
    # Normalize only declared run-local paths. The MCP input has its own
    # independently checked producer binding.
    output_args = {'coverage': ('-OutputRoot', 'coverage'), 'mcp': ('-report', 'mcp.md'),
                   'shell-fixtures': ('-WorkingPath', ''), 'snapshot-fixtures': ('-WorkingPath', '')}
    argv = list(step['argv'])
    paths = [output_args[step['name']]] if step['name'] in output_args else []
    if step['name'] == 'mcp':
        paths.append(('-coverage-evidence', coverage_log(platform.system())))
    for option, relative in paths:
        if argv.count(option) == 1:
            index = argv.index(option)
            if argv[index:index + 2] == [option, str(run / relative)]:
                argv[index + 1] = '<run>/' + relative
    return {**step, 'argv': argv}


def coverage_log(system):
    return 'coverage/' + system.lower() + '/test.log'


def uses_coverage(steps):
    return any(row['name'] == 'mcp' and '-coverage-evidence' in row['argv'] for row in steps)


def capture_coverage_input(run, result, system):
    name = coverage_log(system)
    files = dict(result['artifacts'])
    with source_boundary(run) as root_fd:
        summary = 'coverage/' + system.lower() + '/summary.json'
        with evidence_file(run, root_fd, summary) as stream:
            raw = stream.read()
        data = parse_json_object(raw.decode('utf-8'))
        if (hashlib.sha256(raw).hexdigest() != files[summary]
                or data.get('status') != 'PASS' or data.get('platform') != system.lower()
                or data.get('test_log_format') != 'go-test-json'
                or Path(data.get('test_log', '')) != run / name):
            raise Hold('Coverage producer summary does not admit its Go test log')
        with evidence_file(run, root_fd, name) as stream:
            size = os.fstat(stream.fileno()).st_size
    return {'path': name, 'sha256': files[name], 'bytes': size, 'origin': run.name}


def coverage_input(result, system):
    if result is None or not successful_result(result) or result.get('name') != 'coverage':
        raise Hold('Successful Coverage producer required for MCP')
    row = result.get('coverage_input')
    name = coverage_log(system)
    if (not isinstance(row, dict) or set(row) != {'path', 'sha256', 'bytes', 'origin'}
            or row['path'] != name or not isinstance(row['sha256'], str)
            or re.fullmatch('[0-9a-f]{64}', row['sha256']) is None
            or dict(result.get('artifacts', [])).get(name) != row['sha256']
            or type(row['bytes']) is not int or row['bytes'] <= 0
            or not isinstance(row['origin'], str)
            or re.fullmatch('validation-[0-9a-f]{12}', row['origin']) is None):
        raise Hold('Coverage input does not match its gate artifact binding')
    return row


def coverage_arguments(row):
    return ['-coverage-sha256', row['sha256'], '-coverage-bytes', str(row['bytes']),
            '-coverage-origin', row['origin']]


def admit_coverage(run, result, system):
    row = coverage_input(result, system)
    with source_boundary(run) as root_fd, evidence_file(run, root_fd, row['path']) as stream:
        if (os.fstat(stream.fileno()).st_size != row['bytes']
                or hashlib.file_digest(stream, 'sha256').hexdigest() != row['sha256']):
            raise Hold('Coverage evidence drifted before MCP consumption')
    return row


def verify_coverage_link(proof, base, root_fd):
    if not uses_coverage(proof['plan']):
        return
    rows = {row['name']: row for row in proof['results'] if successful_result(row)}
    if 'coverage' not in rows and 'mcp' not in rows:
        return
    coverage = rows.get('coverage')
    system = proof['platform']
    row = coverage_input(coverage, system)
    if coverage.get('reused_from') is None and row['origin'] != base.name:
        raise Hold('Fresh Coverage producer origin differs')
    with evidence_file(base, root_fd, row['path']) as stream:
        if os.fstat(stream.fileno()).st_size != row['bytes']:
            raise Hold('Coverage artifact length differs')
    if 'mcp' in rows and rows['mcp'].get('coverage_input') != row:
        raise Hold('MCP evidence is detached from its Coverage producer')


def required_gate_files(gate, system):
    files = {gate + '.stdout', gate + '.stderr'}
    if system == 'Windows':
        files.update((gate + '.request.json', gate + '.process.json'))
    if gate == 'mcp':
        files.update(('mcp.md', 'mcp.md.tests.jsonl'))
    if gate == 'coverage':
        files.update('coverage/' + system.lower() + '/' + name for name in (
            'coverage.html', 'coverage.out', 'coverage.txt', 'product-packages.txt', 'summary.json', 'test.log'))
    if gate == 'native-linux':
        files.update(('native-evidence/summary.json', 'native-evidence/native-context.json',
                      'native.request.json', 'snapshot.request.json', 'snapshot.result.json',
                      'source.manifest.json', 'source.tar'))
    return files


def gate_artifact_binding(gate, system, inventory):
    required = required_gate_files(gate, system)
    files = dict(inventory)
    if not required <= files.keys():
        raise Hold('Missing required gate artifacts: ' + gate)
    if gate == 'native-linux':
        required.update(name for name in files if name.startswith('native-evidence/'))
    return [[name, files[name]] for name in sorted(required)]


def verify_gate_artifacts(result, proof):
    if result.get('artifacts') != gate_artifact_binding(result['name'], proof['platform'], proof['artifacts']):
        raise Hold('Gate artifact binding differs: ' + result['name'])


def verify_native_projection(base, root_fd, inventory):
    projected = {name.removeprefix('native-evidence/'): expected for name, expected in inventory
                 if name.startswith('native-evidence/')}
    try:
        expected = projected.pop('summary.json')
        projected.pop('native-context.json')
    except KeyError as error:
        raise Hold('Complete native export required') from error
    with evidence_file(base, root_fd, 'native-evidence/summary.json') as stream:
        raw = stream.read()
    if hashlib.sha256(raw).hexdigest() != expected:
        raise Hold('Native exported summary changed')
    original = parse_json_object(raw.decode('utf-8'))
    if projected != dict(original['artifacts']):
        raise Hold('Native export differs from original summary inventory')


def capture_gate_artifacts(run, result, system):
    gate = result['name']
    files = required_gate_files(gate, system)
    with source_boundary(run) as root_fd:
        inventory = []
        for name in sorted(files):
            try:
                with evidence_file(run, root_fd, name) as stream:
                    inventory.append([name, hashlib.file_digest(stream, 'sha256').hexdigest()])
            except FileNotFoundError as error:
                raise Hold('Missing required gate artifact: ' + name) from error
        if gate == 'native-linux':
            with evidence_file(run, root_fd, 'native-linux.stdout') as stream:
                lines = stream.read().decode('utf-8').splitlines()
            if (not lines or parse_json_object(lines[-1]).get('summary_sha256')
                    != dict(inventory)['native-evidence/summary.json']):
                raise Hold('Native export does not match the child original summary')
            nested = artifact_inventory(run / 'native-evidence')
            inventory.extend(['native-evidence/' + name, expected] for name, expected in nested
                             if name != 'native-context.json')
            verify_native_projection(run, root_fd, inventory)
    if not all(row in inventory for row in result['artifacts']):
        raise Hold('Process artifacts changed after gate completion: ' + gate)
    return gate_artifact_binding(gate, system, inventory)


def verify_resume_origins(proof, base, root_fd):
    origins = {}
    for result in proof['results']:
        if successful_result(result):
            verify_gate_artifacts(result, proof)
        key = result.get('reused_from')
        if key is None:
            continue
        if not isinstance(key, str) or not re.fullmatch('[0-9a-f]{64}', key):
            raise Hold('Invalid original gate provenance')
        if key not in origins:
            with evidence_file(base, root_fd, 'origins/' + key + '.json') as stream:
                raw = stream.read()
            if hashlib.sha256(raw).hexdigest() != key:
                raise Hold('Original gate provenance changed')
            origins[key] = parse_json_object(raw.decode('utf-8'))
        original = origins[key]
        terminal = {k: v for k, v in result.items() if k != 'reused_from'}
        if terminal not in original['results']:
            raise Hold('Original result differs from reused gate')
        verify_gate_artifacts(terminal, original)
        if terminal['name'] == 'coverage' and uses_coverage(proof['plan']):
            row = coverage_input(terminal, proof['platform'])
            if original.get('run') != row['origin']:
                raise Hold('Resumed Coverage origin does not match its original summary')
    verify_coverage_link(proof, base, root_fd)


def copy_evidence_artifact(base, root_fd, run, name, expected):
    target = run.joinpath(*source_parts(name))
    target.parent.mkdir(parents=True, exist_ok=True)
    with evidence_file(base, root_fd, name) as origin, target.open('xb') as output:
        shutil.copyfileobj(origin, output)
    if digest(target) != expected:
        raise Hold('Evidence changed before restoration: ' + name)


def retain_native_checkpoint(proof, source_run, root_fd, run):
    artifacts = dict(proof['artifacts'])
    for prefix in ('native-evidence/', 'native-checkpoint/'):
        checkpoint = {}
        for name in ('native-context.json', 'summary.json'):
            source = prefix + name
            if source not in artifacts:
                break
            with evidence_file(source_run, root_fd, source) as stream:
                raw = stream.read()
            if hashlib.sha256(raw).hexdigest() != artifacts[source]:
                raise Hold('Native checkpoint changed')
            checkpoint[name] = raw
        else:
            summary = parse_json_object(checkpoint['summary.json'].decode('utf-8'))
            if summary.get('resumable') is not True or summary.get('status') not in ('HOLD', 'FAIL'):
                continue
            context = parse_json_object(checkpoint['native-context.json'].decode('utf-8'))
            # Keep the partial reference even when a Windows HOLD prevents a
            # native dispatch. The full artifact inventory remains at the
            # original native run and must pass admission before any reuse.
            retained = run / 'native-checkpoint'
            retained.mkdir()
            for name, data in checkpoint.items():
                with (retained / name).open('xb') as output:
                    output.write(data)
            return {'source': context['source'], 'run': context['run'],
                    'sha256': artifacts[prefix + 'summary.json']}
    return None


def resume_results(path, expected, root, work, run, current, steps, base_commit,
                   deletions, *, allow_native, native_probe):
    path = Path(path).absolute()
    source_run = path.parent
    if (path.name != 'summary.json' or source_run.parent != work
            or not re.fullmatch('validation-[0-9a-f]{12}', source_run.name)):
        raise Hold('Resume needs an anchored run in the current task work root')
    with admitted_summary(path, expected) as (proof, raw, source_run, root_fd):
        if (proof.get('schema') != 1 or proof.get('mode') != 'CANDIDATE'
                or proof.get('platform') != platform.system() or proof.get('checkout') != str(root)
                or proof.get('status') not in ('PASS', 'PARTIAL', 'HOLD', 'FAIL')
                or proof.get('resumable') is not True):
            raise Hold('Resume origin or process completion is not proven')
        verify_resume_origins(proof, source_run, root_fd)
        previous = proof['binding']
        if proof.get('binding_sha256') != identity(previous):
            raise Hold('Invalid original candidate binding')
        # A stage or commit may keep the same tested postimage. Base/OID drift
        # cannot transplant the evidence to a different candidate.
        if proof.get('source_base') != base_commit or proof.get('deleted_sources') != deletions:
            raise Hold('Resume base or deletion inventory differs')
        old_steps = {row['name']: resume_step(row, source_run) for row in proof['plan']}
        if len(old_steps) != len(proof['plan']) or list(old_steps) != [step['name'] for step in steps]:
            raise Hold('Resume gate plan identity differs')
        actual_names = [row['name'] for row in proof['results'] if row['name'] != 'coordinator']
        if len(actual_names) != len(set(actual_names)) or actual_names != [
                name for name in old_steps if name in actual_names]:
            raise Hold('Resume origin has invalid gate order')
        reused = {}
        invalidated = {}
        for row in proof['results']:
            name = row['name']
            if name == 'coordinator' or not successful_result(row):
                continue
            step = next(s for s in steps if s['name'] == name)
            if (name in ('shell', 'shell-fixtures', *BUILD_SMOKE_GATES[1:])
                    and current.get('native_shell_reusable') is False):
                invalidated[name] = 'native shell helper/runtime binding incomplete'
                continue
            if phase_binding(previous, name) != phase_binding(current, name):
                invalidated[name] = 'platform binding changed'
                continue
            if old_steps[name] != resume_step(step, run):
                invalidated[name] = 'gate arguments changed'
                continue
            if name == 'coverage' and uses_coverage(steps) and 'coverage_input' not in row:
                invalidated[name] = 'Coverage producer provenance unavailable'
                continue
            if name == 'native-linux':
                if not allow_native:
                    continue
                if not native_probe(path, proof):
                    invalidated[name] = 'native evidence binding changed'
                    continue
            reused[name] = {**row, 'reused_from': row.get('reused_from', expected.lower())}
        build_hashes = {reused[name].get('smoke_binary_sha256')
                        for name in BUILD_SMOKE_GATES if name in reused}
        if (not all(name in reused for name in BUILD_SMOKE_GATES)
                or len(build_hashes) != 1 or None in build_hashes):
            for name in BUILD_SMOKE_GATES:
                if name in reused:
                    del reused[name]
                    invalidated[name] = 'build and smoke must be reused together'
        if uses_coverage(steps) and 'mcp' in reused and 'coverage' not in reused:
            del reused['mcp']
            invalidated['mcp'] = 'MCP needs a valid Coverage producer'
        origins = {row['reused_from'] for row in reused.values()}
        reused_artifacts = {name for row in reused.values() for name, _ in row['artifacts']}
        for name, artifact_hash in proof['artifacts']:
            if (name in reused_artifacts
                    or name in {'origins/' + digest + '.json' for digest in origins}):
                copy_evidence_artifact(source_run, root_fd, run, name, artifact_hash)
        if expected.lower() in origins:
            (run / 'origins').mkdir(exist_ok=True)
            with (run / 'origins' / (expected.lower() + '.json')).open('xb') as original:
                original.write(raw)
        native_checkpoint = None
        if (allow_native and 'native-linux' not in reused
                and phase_binding(previous, 'native-linux') == phase_binding(current, 'native-linux')):
            native_checkpoint = retain_native_checkpoint(proof, source_run, root_fd, run)
        return reused, invalidated, native_checkpoint


@contextlib.contextmanager
def evidence_file(base, root_fd, name):
    with source_file(base, root_fd, name) as (stream, _):
        before = os.fstat(stream.fileno())
        yield stream
        readback = os.fstat(stream.fileno())
        parts = source_parts(name)
        with source_parent(base, root_fd, parts[:-1]) as parent:
            after = source_stat(parent, parts[-1])
        if (not os.path.samestat(before, after) or not stat.S_ISREG(after.st_mode)
                or getattr(after, 'st_file_attributes', 0) & 0x400
                or (before.st_size, before.st_mtime_ns, before.st_ctime_ns)
                != (readback.st_size, readback.st_mtime_ns, readback.st_ctime_ns)
                or (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns)):
            raise Hold('Evidence changed while reading: ' + name)


def evidence_inventory(base, root_fd):
    directories = [()]
    artifacts = []
    while directories:
        parts = directories.pop()
        with source_parent(base, root_fd, parts) as parent:
            for leaf in os.listdir(parent):
                if not parts and leaf == 'summary.json':
                    continue
                name = '/'.join((*parts, leaf))
                meta = source_stat(parent, leaf)
                if getattr(meta, 'st_file_attributes', 0) & 0x400:
                    raise Hold('Evidence contains a reparse point: ' + name)
                if stat.S_ISDIR(meta.st_mode):
                    directories.append((*parts, leaf))
                elif stat.S_ISREG(meta.st_mode):
                    artifacts.append(Path(*parts, leaf))
                else:
                    raise Hold('Evidence contains a nonregular artifact: ' + name)
    inventory = []
    for artifact in sorted(artifacts):
        name = artifact.as_posix()
        with evidence_file(base, root_fd, name) as stream:
            inventory.append([name, hashlib.file_digest(stream, 'sha256').hexdigest()])
    return inventory


def artifact_inventory(work):
    with source_boundary(work) as root_fd:
        return evidence_inventory(work, root_fd)


def smoke_binary_identity(root):
    name = SMOKE_BUILD_WINDOWS if os.name == 'nt' else SMOKE_BUILD_LINUX
    with source_boundary(root) as root_fd, evidence_file(root, root_fd, name) as binary:
        return hashlib.file_digest(binary, 'sha256').hexdigest()


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


def verify_classic_review(path, expected, manifest_path, root, current, summary_identity):
    if not path or not isinstance(expected, str) or not manifest_path or digest(path) != expected.lower():
        raise Hold('Independent Classic decision, matching digest and reviewed manifest required')
    review, manifest = load(path), load(manifest_path)
    validate_classic_outcome(review, manifest, root, current, summary_identity)


def validate_classic_outcome(review, manifest, root, current, summary_identity):
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
    summary_digest, summary_length = summary_identity
    if not any(row['SHA256'] == summary_digest and row['Length'] == summary_length
               for row in technical['PayloadBindings']):
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


def prepare_native(root, run, config, env, current, base, deletions, checkpoint=None):
    request, result = run / 'snapshot.request.json', run / 'snapshot.result.json'
    archive, manifest = run / 'source.tar', run / 'source.manifest.json'
    write(request, {'operation': 'snapshot', 'directory': str(root), 'archive': str(archive), 'manifest': str(manifest)})
    completed = subprocess.run([shutil.which('pwsh', path=env['PATH']), '-NoProfile', '-NonInteractive', '-File',
                                str(root / 'scripts/validation-process.ps1'), '-RequestPath', str(request), '-ResultPath', str(result)],
                               env=env, capture_output=True, timeout=120, check=False)
    if completed.returncode or load(result).get('status') != 'PASS':
        raise Hold('Existing source snapshot producer failed')
    captured = load(manifest)
    if {row['Path']: row['SHA256'] for row in captured['files']} != {
            row[0]: row[1] for row in current['sources']}:
        raise Hold('Snapshot files differ from the bound postimage')
    verify_absent(root, deletions)
    task = dict(config)
    if checkpoint:
        if task.get('source_root', checkpoint['source']) != checkpoint['source']:
            raise Hold('Native checkpoint belongs to a different source root')
        task.update(source_root=checkpoint['source'], resume=checkpoint)
    task.update(archive=wsl_path(archive), manifest=wsl_path(manifest), archive_sha256=digest(archive),
                manifest_sha256=digest(manifest), export_root=wsl_path(run / 'native-evidence'),
                git_dir=wsl_path(git(root, 'rev-parse', '--absolute-git-dir').decode().strip()),
                base=base, deleted_sources=deletions)
    path = run / 'native.request.json'
    write(path, task)
    return ['C:/Windows/System32/wsl.exe', '--distribution', config['distribution'], '--user', config['user'],
            '--cd', config['task_root'], '--exec', '/usr/bin/python3', '-B',
            wsl_path(root / 'scripts/validation.py'), '--native-stage', wsl_path(path)]


def remove_native_deletions(source, names, snapshot):
    # Only verified deletions from this isolated checkout's base reach here.
    with checkout_descriptor(source) as root_fd:
        for name in names:
            path = Path(snapshot.normalize_path(name))
            with contextlib.ExitStack() as descriptors:
                parent_fd = root_fd
                try:
                    for part in path.parts[:-1]:
                        parent_fd = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                                            dir_fd=parent_fd)
                        descriptors.callback(os.close, parent_fd)
                    meta = os.stat(path.name, dir_fd=parent_fd, follow_symlinks=False)
                except FileNotFoundError:
                    continue
                if not stat.S_ISREG(meta.st_mode) or meta.st_nlink != 1:
                    raise Hold('Native deletion target is not a singly linked regular file: ' + name)
                os.unlink(path.name, dir_fd=parent_fd)


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
    try:
        assert_no_competing_go()
    except ResourceHold:
        print(json.dumps({'status': 'HOLD', 'resumable': True, 'stage': 'preflight'}))
        return 2
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
    deletions = sorted(tracked_at(source, request['base']) - set(manifest))
    if deletions != request.get('deleted_sources'):
        raise Hold('Native snapshot deletion paths differ from the bound base')
    if reuse and set(state['sources']) - set(manifest) - set(deletions):
        raise Hold('Existing native source has files absent from the new snapshot')
    remove_native_deletions(source, deletions, module)
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
    argv = [sys.executable, '-B', str(source / 'scripts/validation.py'), '--mode', 'CANDIDATE',
            '--work-root', str(work)]
    if 'resume' in request:
        checkpoint = request['resume']
        prior = safe_directory(checkpoint['run'])
        if prior.parent != work or str(source) != checkpoint['source']:
            raise Hold('Native resume run is not in the bound checkout work root')
        argv += ['--resume', str(prior / 'summary.json'), '--resume-sha256', checkpoint['sha256']]
    result = subprocess.run(argv, cwd=source, env=env, check=False)
    summaries = list(set(work.glob('validation-*/summary.json')) - previous_summaries)
    if len(summaries) != 1:
        raise Hold('Native candidate summary missing or ambiguous')
    original = summaries[0].read_bytes()
    proof = parse_json_object(original.decode('utf-8'))
    export = Path(request['export_root'])
    safe_directory(export.parent)
    if export.exists() or export.is_symlink():
        raise Hold('Native export must be fresh')
    shutil.copytree(summaries[0].parent, export)
    write(export / 'native-context.json', {'source': str(source), 'work': str(work),
                                           'run': str(summaries[0].parent), 'tools': directories})
    print(json.dumps({'status': proof.get('status', 'HOLD'), 'native_root': str(stage),
                       'resumable': proof.get('resumable') is True,
                      'summary_sha256': hashlib.sha256(original).hexdigest()}))
    return 0 if proof.get('status') == 'PASS' and result.returncode == 0 else 2


def native_verify(context_path, summary_path, subject):
    observed = Path(context_path).read_bytes()
    if hashlib.sha256(observed).hexdigest() != subject['context_sha256']:
        raise Hold('Native context identity changed')
    context = parse_json_object(observed.decode('utf-8'))
    if set(context) != {'source', 'work', 'run', 'tools'}:
        raise Hold('Complete native execution context required')
    source, work = safe_directory(context['source']), safe_directory(context['work'])
    run = Path(context['run'])
    task = safe_directory(subject['task_root'])
    if (task not in source.parents or work != source.parent / 'work' or run.parent != work
            or not re.fullmatch('validation-[0-9a-f]{12}', run.name)
            or context['tools'] != subject['tools']
            or any(path.stat().st_uid != os.getuid() for path in (task, source, work))):
        raise Hold('Native reuse is outside the bound actor, task or tool paths')
    env = environment(work, create=False)
    env['PATH'] = os.pathsep.join(context['tools'] + [os.environ['PATH']])
    current = binding(source, env)
    if not current['native_shell_reusable']:
        raise Hold('Native shell helper/runtime binding incomplete')
    if (identity([[name, digest] for name, digest, _ in current['sources']]) != subject['source_hashes']
            or identity(current['git_modes']) != subject['git_modes']
            or identity(current['git_blobs']) != subject['git_blobs']):
        raise Hold('Native evidence does not cover the combined source and Git postimage')
    raw = Path(summary_path).read_bytes()
    if hashlib.sha256(raw).hexdigest() != subject['summary_sha256']:
        raise Hold('Native summary identity changed')
    proof = parse_json_object(raw.decode('utf-8'))
    if (proof.get('platform') != 'Linux' or proof.get('mode') != 'CANDIDATE'
            or proof.get('source_base') != subject['source_base']
            or proof.get('deleted_sources') != subject['deleted_sources']
            or proof.get('checkout') != str(source)):
        raise Hold('Native base, platform, deletion or checkout mismatch')
    verify_evidence_results(proof, current, plan('CANDIDATE', [], source, run, current['tools']))
    with admitted_summary(run / 'summary.json', subject['summary_sha256']) as (original, _, base, root_fd):
        verify_resume_origins(original, base, root_fd)
    verify_source_postimage(source, current, proof)
    print(json.dumps({'status': 'PASS', 'quality_processes_started': 0}))
    return 0


def check_native_reuse(root, summary, config, env, proof):
    base = Path(summary).absolute().parent / 'native-evidence'
    artifacts = dict(proof['artifacts'])
    with source_boundary(base.parent) as root_fd:
        verify_native_projection(base.parent, root_fd, proof['artifacts'])
    try:
        context_hash = artifacts['native-evidence/native-context.json']
        summary_hash = artifacts['native-evidence/summary.json']
    except KeyError as error:
        raise Hold('Complete native artifacts required') from error
    windows = proof['binding']
    subject = {'task_root': config['task_root'],
               'tools': [str(Path(config[key]).parent).replace('\\', '/') for key in ('go', 'lint', 'pwsh')],
               'context_sha256': context_hash, 'summary_sha256': summary_hash,
               'source_hashes': identity([[name, digest] for name, digest, _ in windows['sources']]),
               'git_modes': identity(windows['git_modes']), 'git_blobs': identity(windows['git_blobs']),
               'source_base': proof['source_base'], 'deleted_sources': proof['deleted_sources']}
    command = ['C:/Windows/System32/wsl.exe', '--distribution', config['distribution'], '--user', config['user'],
               '--cd', config['task_root'], '--exec', '/usr/bin/python3', '-B',
               wsl_path(root / 'scripts/validation.py'), '--native-verify',
               wsl_path(base / 'native-context.json'), wsl_path(base / 'summary.json')]
    completed = subprocess.run(command, env=env, capture_output=True, check=False, timeout=90,
                               input=json.dumps(subject, separators=(',', ':')).encode('utf-8'))
    if completed.returncode:
        raise Hold('Native evidence is no longer current')
    try:
        actual = json.loads(completed.stdout)
    except ValueError as error:
        raise Hold('Native verification result unknown') from error
    if actual != {'status': 'PASS', 'quality_processes_started': 0}:
        raise Hold('Native verification result invalid')


def confirmed_native_hold(run, result):
    if (result.get('exit_code') != 2 or result.get('timed_out') is not False
            or result.get('output_limit') or result.get('status') not in ('HOLD', 'FAIL')
            or result.get('failure_reason') not in ('', 'None', 'NonZeroExit', None)):
        return False
    try:
        line = (run / 'native-linux.stdout').read_text(encoding='utf-8').splitlines()[-1]
        state = parse_json_object(line)
        if state == {'status': 'HOLD', 'resumable': True, 'stage': 'preflight'}:
            return not (run / 'native-evidence').exists()
        summary = run / 'native-evidence' / 'summary.json'
        if state.get('status') not in ('HOLD', 'FAIL') or state.get('resumable') is not True:
            return False
        proof = load(summary)
        return (digest(summary) == state.get('summary_sha256')
                and proof.get('status') == state['status'] and proof.get('resumable') is True)
    except (OSError, ValueError, Hold, IndexError):
        return False


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
    parser.add_argument('--platform-phase', choices=['local', 'native'],
                        help='Run only this phase of a Windows candidate, retaining a partial summary')
    parser.add_argument('--resume', help='Retained candidate summary in this task work root')
    parser.add_argument('--resume-sha256', help='Separately retained SHA-256 of the resume summary')
    parser.add_argument('--plan-only', action='store_true')
    args = parser.parse_args(argv)
    if (bool(args.resume) != bool(args.resume_sha256)
            or ((args.resume or args.platform_phase) and
                (args.mode != 'CANDIDATE' or args.evidence or args.plan_only))
            or (args.platform_phase and os.name != 'nt')):
        raise Hold('Resume and phase selection apply only to candidate execution')
    root = Path(__file__).resolve().parent.parent
    work = safe_directory(args.work_root)
    if work == root or root in work.parents:
        raise Hold('Work root must be outside source')
    resource = safe_directory(args.resource_root or work)
    changes = args.changed if args.changed is not None else changed_paths(root)
    if args.mode == 'FAST':
        require_fast_selection(changes)
    env = environment(work, create=args.mode != 'PUBLISH')
    hygiene = source_hygiene(root)
    check_ignored = args.mode != 'FAST'
    current = binding(root, env, check_ignored=check_ignored)
    config = native_config(args.native_config) if args.native_config else None
    if os.name == 'nt' and args.mode in ('CANDIDATE', 'PUBLISH') and not config:
        raise Hold('Complete Windows candidate needs --native-config for the required native Linux gates')
    if config:
        current['native_config'] = config
    mode = 'CANDIDATE' if args.mode == 'PUBLISH' else args.mode
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
        native_probe = (lambda path, proof: check_native_reuse(root, path, config, env, proof)) if config else None
        summary_identity = verify_evidence(args.evidence, args.evidence_sha256, current, steps,
                                           root=root, native_probe=native_probe)
        if args.mode == 'PUBLISH':
            verify_classic_review(args.review, args.review_sha256, args.review_manifest, root, current, summary_identity)
        print(json.dumps({'status': 'PASS', 'reused': True, 'quality_processes_started': 0,
                          'binding_sha256': identity(current), 'external_operations_authorized': False}))
        return 0
    if args.mode == 'PUBLISH':
        raise Hold('PUBLISH requires candidate evidence; it never launches a test matrix')
    source_base = git(root, 'rev-parse', 'HEAD').decode().strip()
    if args.resume:
        # Read only the separately anchored header to recover the original Git
        # base after ordinary staging or committing the tested postimage.
        prior_path = Path(args.resume).absolute()
        if (prior_path.parent.parent != work or prior_path.name != 'summary.json'
                or not re.fullmatch('validation-[0-9a-f]{12}', prior_path.parent.name)):
            raise Hold('Resume needs an anchored run in the current task work root')
        with source_boundary(prior_path.parent) as root_fd:
            with evidence_file(prior_path.parent, root_fd, prior_path.name) as stream:
                original = stream.read()
        if hashlib.sha256(original).hexdigest() != args.resume_sha256.lower():
            raise Hold('Resume summary identity changed')
        original_base = parse_json_object(original.decode('utf-8')).get('source_base')
        if not isinstance(original_base, str) or not re.fullmatch('[0-9a-f]{40,64}', original_base):
            raise Hold('Resume base identity invalid')
        verify_git_postimage(root, current, original_base)
        source_base = original_base
    deletions = deleted_sources(root, source_base, current['sources'])
    run.mkdir()
    results = []
    status = 'FAIL'
    resumable = False
    invalidated = {}
    try:
        reused = {}
        native_checkpoint = None
        smoke_hash = None
        if args.resume:
            def native_probe(path, proof):
                try:
                    check_native_reuse(root, path, config, env, proof)
                except Hold:
                    return False
                return True

            reused, invalidated, native_checkpoint = resume_results(
                args.resume, args.resume_sha256, root, work, run, current, steps, source_base, deletions,
                allow_native=config is not None, native_probe=native_probe)
            results.extend(reused.values())
        # Retain the work root's caches between runs; outputs are always fresh.
        for step in steps:
            name = step['name']
            if name in reused:
                continue
            if ((args.platform_phase == 'local' and name == 'native-linux')
                    or (args.platform_phase == 'native' and name != 'native-linux')):
                continue
            observed = binding(root, env, check_ignored=check_ignored)
            if config:
                observed['native_config'] = config
            if observed != current:
                raise Hold('Source/toolchain/environment changed during validation')
            verify_absent(root, deletions)
            verify_git_postimage(root, observed, source_base)
            print(json.dumps({'status': 'RUNNING', 'gate': step['name']}), flush=True)
            guard = heavy_lock(resource, root) if step['heavy'] else contextlib.nullcontext()
            with guard:
                # The explicit native-only path uses Windows for the guarded
                # snapshot and bridge. The native stage proves its filesystem
                # and checks Go processes before running any Linux gate.
                if step['heavy'] and not (name == 'native-linux' and args.platform_phase == 'native'):
                    assert_no_competing_go()
                if name in BUILD_SMOKE_GATES[1:] and smoke_binary_identity(root) != smoke_hash:
                    raise Hold('Smoke input differs from the completed build')
                command = (prepare_native(root, run, config, env, current, source_base, deletions, native_checkpoint)
                           if step['name'] == 'native-linux' else step['argv'])
                if name == 'mcp':
                    coverage = next((row for row in results if row['name'] == 'coverage'), None)
                    admitted = admit_coverage(run, coverage, platform.system())
                    command = [*command, *coverage_arguments(admitted)]
                result = run_process(root, run, step['name'], command, env,
                                     timeout=10800 if step['name'] == 'native-linux' else 1800)
                if name == 'mcp':
                    result['coverage_input'] = admitted
                if step['name'] == 'native-linux' and result.get('timed_out'):
                    result['status'] = 'HOLD'  # Killing wsl.exe proves no Linux descendant termination.
                if step['name'] == 'native-linux':
                    native_summary = run / 'native-evidence/summary.json'
                    native_state = load(native_summary).get('status') if native_summary.is_file() else 'HOLD'
                    if native_state != 'PASS':
                        result['status'] = native_state if native_state in ('FAIL', 'HOLD') else 'HOLD'
                if name in BUILD_SMOKE_GATES and successful_result(result):
                    binary_hash = smoke_binary_identity(root)
                    if name != 'build' and smoke_hash != binary_hash:
                        raise Hold('Smoke binary changed while the gate was running')
                    smoke_hash = binary_hash
                    result['smoke_binary_sha256'] = binary_hash
            result['name'] = step['name']
            if args.mode == 'CANDIDATE' and successful_result(result):
                result['artifacts'] = capture_gate_artifacts(run, result, platform.system())
                if name == 'coverage':
                    result['coverage_input'] = capture_coverage_input(run, result, platform.system())
            results.append(result)
            if result['status'] != 'PASS':
                status = result['status']
                if name == 'native-linux':
                    resumable = confirmed_native_hold(run, result)
                else:
                    resumable = (status == 'FAIL' and result.get('exit_code') is not None
                                 and not result.get('timed_out')
                                 and result.get('failure_reason') in ('', 'None', 'NonZeroExit', None)
                                 and not result.get('output_limit'))
                break
        else:
            observed = binding(root, env, check_ignored=check_ignored)
            if config:
                observed['native_config'] = config
            if observed != current:
                raise Hold('Candidate drifted during validation')
            verify_absent(root, deletions)
            verify_git_postimage(root, observed, source_base)
            hygiene = source_hygiene(root)
            status = 'PASS' if len(results) == len(steps) else 'PARTIAL'
            resumable = True
    except ResourceHold as error:
        status = 'HOLD'
        resumable = True
        results.append({'name': 'coordinator', 'status': status, 'error': str(error)})
    except (Hold, subprocess.TimeoutExpired, KeyboardInterrupt) as error:
        status = 'HOLD'
        results.append({'name': 'coordinator', 'status': status, 'error': type(error).__name__ + ': ' + str(error)})
    finally:
        system = platform.system()
        inventory = artifact_inventory(run)
        if args.mode == 'CANDIDATE':
            try:
                for row in results:
                    if successful_result(row):
                        verify_gate_artifacts(row, {'platform': system, 'artifacts': inventory})
                with source_boundary(run) as root_fd:
                    verify_coverage_link({'results': results, 'plan': steps, 'platform': system}, run, root_fd)
            except Hold as error:
                status, resumable = 'HOLD', False
                results.append({'name': 'coordinator', 'status': status, 'error': str(error)})
        ordering = {step['name']: index for index, step in enumerate(steps)}
        results.sort(key=lambda row: ordering.get(row['name'], len(steps)))
        summary = run / 'summary.json'
        write(summary, {'schema': 1, 'status': status, 'mode': args.mode, 'platform': system, 'run': run.name,
                        'binding': current, 'plan': steps, 'results': results,
                        'artifacts': inventory, 'binding_sha256': identity(current),
                         'checkout': str(root), 'resumable': resumable, 'invalidated': invalidated,
                        'source_base': source_base, 'deleted_sources': deletions,
                        'source_hygiene': hygiene,
                        'matrix_scope': 'local-platform; Windows and native Linux evidence both required for handoff'})
        print(json.dumps({'status': status, 'report_path': str(summary), 'sha256': digest(summary)}), flush=True)
    return 0 if status == 'PASS' else 2


if __name__ == '__main__':
    try:
        if len(sys.argv) == 3 and sys.argv[1] == '--native-stage':
            raise SystemExit(native_stage(sys.argv[2]))
        if len(sys.argv) == 4 and sys.argv[1] == '--native-verify':
            raise SystemExit(native_verify(sys.argv[2], sys.argv[3], parse_json_object(sys.stdin.read())))
        raise SystemExit(main())
    except (OSError, ValueError, Hold, subprocess.SubprocessError) as error:
        print(json.dumps({'status': 'HOLD', 'error': type(error).__name__ + ': ' + str(error)}))
        raise SystemExit(2)
