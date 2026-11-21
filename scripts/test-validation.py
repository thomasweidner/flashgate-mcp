#!/usr/bin/env python3
"""Focused negative contracts for the project validation entry point."""
import contextlib
import copy
import importlib.util
import json
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('validation', Path(__file__).with_name('validation.py'))
v = importlib.util.module_from_spec(spec)
spec.loader.exec_module(v)


class ValidationTests(unittest.TestCase):
    def setUp(self):
        parent = os.environ.get('FLASHGATE_WORK_ROOT')
        if not parent:
            raise RuntimeError('Explicit FLASHGATE_WORK_ROOT required')
        self.temp = tempfile.TemporaryDirectory(dir=parent, prefix='runner-tests-')
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)

    def proof(self):
        directory = self.work / 'candidate'
        directory.mkdir()
        output = directory / 'actual.log'
        output.write_text('real output', encoding='utf-8')
        steps = [{'name': 'tests', 'argv': ['go', 'test', './...'], 'heavy': True}]
        current = {'source': 'original', 'environment': 'original'}
        data = {'schema': 1, 'status': 'PASS', 'binding': current, 'plan': steps,
                'results': [{'name': 'tests', 'status': 'PASS', 'exit_code': 0, 'timed_out': False}],
                'artifacts': [['actual.log', v.digest(output)]]}
        path = directory / 'summary.json'
        v.write(path, data)
        return path, data, current, steps

    def verify_synthetic_evidence(self, path, expected, current, steps):
        with patch.object(v, 'verify_source_postimage'):
            return v.verify_evidence(path, expected, current, steps, root=self.work)

    def test_unchanged_candidate_reuses_without_process(self):
        path, _, current, steps = self.proof()
        with patch.object(v, 'run_process', side_effect=AssertionError('must not run')):
            self.assertEqual(self.verify_synthetic_evidence(path, v.digest(path), current, steps)['status'], 'PASS')

    def test_missing_and_modified_evidence_rejected(self):
        path, data, current, steps = self.proof()
        original = v.digest(path)
        for anchor in (None, '0' * 64):
            with self.assertRaises(v.Hold):
                self.verify_synthetic_evidence(path, anchor, current, steps)
        data['status'] = 'FAIL'
        path.write_text(json.dumps(data), encoding='utf-8')
        with self.assertRaises(v.Hold):
            self.verify_synthetic_evidence(path, original, current, steps)

    def test_source_test_environment_and_scope_drift_rejected(self):
        path, _, current, steps = self.proof()
        for changed in ({**current, 'source': 'test edited'}, {**current, 'environment': 'toolchain changed'}):
            with self.assertRaises(v.Hold):
                self.verify_synthetic_evidence(path, v.digest(path), changed, steps)
        with self.assertRaises(v.Hold):
            self.verify_synthetic_evidence(path, v.digest(path), current, [])

    def test_inherited_go_build_settings_invalidate_reuse(self):
        path, data, _, steps = self.proof()
        baseline = {'PATH': os.defpath, 'GOAMD64': 'v1'}
        with patch.object(v, 'tools_binding', return_value={}), patch.object(v, 'sources', return_value=[]), \
                patch.object(v, 'git_modes', return_value=[]), patch.object(v, 'git_blobs', return_value=[]):
            original = v.binding(self.work, baseline)
            data['binding'] = original
            path.write_text(json.dumps(data), encoding='utf-8')
            anchor = v.digest(path)
            for name, value in {
                'GOAMD64': 'v4', 'GOARM64': 'v9.5', 'GOROOT': str(self.work / 'go'),
                'GOFIPS140': 'latest', 'GO_EXTLINK_ENABLED': '1',
                'CGO_CFLAGS': '-O0', 'CGO_LDFLAGS': '-L/usr/local/lib',
                'CGO_CFLAGS_ALLOW': '.*', 'PKG_CONFIG_PATH': '/usr/local/lib/pkgconfig',
                'CC': 'clang', 'FC': 'gfortran', 'GCCGO': 'gccgo', 'CPATH': '/usr/local/include',
            }.items():
                with self.subTest(name=name), self.assertRaisesRegex(v.Hold, 'invalidated'):
                    self.verify_synthetic_evidence(path, anchor, v.binding(self.work, {**baseline, name: value}), steps)
            with patch.object(v, 'run_process', side_effect=AssertionError('must not run')):
                self.assertEqual(self.verify_synthetic_evidence(path, anchor, v.binding(self.work, baseline), steps)['status'], 'PASS')

    def test_deleted_postimage_is_stable_across_git_states(self):
        path, data, _, steps = self.proof()
        root = self.work / 'checkout'
        root.mkdir()
        def git(*args):
            return v.git(root, *args)
        git('init', '--quiet')
        for name in ('obsolete', 'keep', 'other'):
            (root / name).write_bytes(name.encode())
        git('add', '.')
        commit = ('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                  '-c', 'commit.gpgsign=false', 'commit', '-qm')
        git(*commit, 'base')
        base = git('rev-parse', 'HEAD').decode().strip()
        (root / 'obsolete').unlink()
        def binding():
            with patch.object(v, 'tools_binding', return_value={}):
                return v.binding(root, {'FLASHGATE_WORK_ROOT': str(self.work)})
        current = binding()
        data.update(binding=current, source_base=base, deleted_sources=['obsolete'])
        path.write_text(json.dumps(data), encoding='utf-8')
        anchor = v.digest(path)
        for state in ('unstaged', 'staged', 'committed'):
            with self.subTest(state=state):
                if state == 'staged':
                    git('add', '-u')
                if state == 'committed':
                    git(*commit, 'deletion')
                else:
                    self.assertEqual(v.changed_paths(root), ['obsolete'])
                actual = binding()
                v.verify_evidence(path, anchor, actual, steps, root=root)

        # Even an ignored reappearance must violate the historical absence.
        (root / '.git/info/exclude').write_text('/obsolete\n', encoding='utf-8')
        (root / 'obsolete').write_bytes(b'obsolete')
        self.assertEqual(v.sources(root), current['sources'])
        with self.assertRaisesRegex(v.Hold, 'still exists'):
            v.verify_source_postimage(root, current, data)
        (root / 'obsolete').unlink()
        (root / 'other').unlink()
        with self.assertRaisesRegex(v.Hold, 'invalidated'):
            v.verify_evidence(path, anchor, binding(), steps, root=root)
        (root / 'other').mkdir()
        with self.assertRaisesRegex(v.Hold, 'Nonregular'):
            v.sources(root)
        if os.name != 'nt':
            (root / 'keep').chmod(0o700)
            (root / 'other').rmdir()
            (root / 'other').write_bytes(b'other')
            with self.assertRaisesRegex(v.Hold, 'invalidated'):
                v.verify_evidence(path, anchor, binding(), steps, root=root)

    def test_git_mode_drift_invalidates_evidence_even_with_identical_filesystem_modes(self):
        path, data, _, steps = self.proof()
        root = self.work / 'checkout'
        root.mkdir()
        def git(*args):
            return v.git(root, *args)
        def binding():
            with patch.object(v, 'tools_binding', return_value={}):
                return v.binding(root, {'FLASHGATE_WORK_ROOT': str(self.work)})
        git('init', '--quiet')
        git('config', 'core.filemode', 'false')
        file = root / 'regular.txt'
        file.write_bytes(b'unchanged\n')
        commit = ('-c', 'core.hooksPath=' + str(self.work / 'no-hooks'),
                  '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                  '-c', 'commit.gpgsign=false', 'commit', '-qm')
        untracked = binding()
        git('add', 'regular.txt')
        self.assertEqual(binding(), untracked)
        git(*commit, 'base')
        self.assertEqual(binding(), untracked)

        for change, expected in (('+x', '100755'), ('-x', '100644')):
            with self.subTest(change=change):
                before = binding()
                data.update(binding=before, source_base=git('rev-parse', 'HEAD').decode().strip(), deleted_sources=[])
                path.write_text(json.dumps(data), encoding='utf-8')
                old_anchor = v.digest(path)
                git('update-index', '--chmod=' + change, 'regular.txt')
                staged = binding()
                self.assertEqual(staged['sources'], before['sources'])
                with self.assertRaisesRegex(v.Hold, 'invalidated'):
                    v.verify_evidence(path, old_anchor, staged, steps, root=root)
                self.assertEqual(staged['git_modes'], [['regular.txt', expected]])
                git(*commit, 'change mode ' + change)
                committed = binding()
                self.assertEqual(committed, staged)
                with self.assertRaisesRegex(v.Hold, 'invalidated'):
                    v.verify_evidence(path, old_anchor, committed, steps, root=root)

                data['binding'] = staged
                path.write_text(json.dumps(data), encoding='utf-8')
                self.assertEqual(v.verify_evidence(path, v.digest(path), committed, steps, root=root)['status'], 'PASS')

    @unittest.skipUnless(os.name == 'posix', 'Native tracked executable-mode contract')
    def test_tracked_chmod_reuses_across_stage_and_commit(self):
        path, data, _, steps = self.proof()
        root = self.work / 'checkout'
        root.mkdir()
        def git(*args):
            return v.git(root, *args)
        def binding():
            with patch.object(v, 'tools_binding', return_value={}):
                return v.binding(root, {'FLASHGATE_WORK_ROOT': str(self.work)})
        git('init', '--quiet')
        git('config', 'core.filemode', 'true')
        file = root / 'checked.sh'
        file.write_bytes(b'#!/bin/sh\nexit 0\n')
        file.chmod(0o644)
        git('add', file.name)
        commit = ('-c', 'core.hooksPath=' + str(self.work / 'no-hooks'),
                  '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                  '-c', 'commit.gpgsign=false', 'commit', '-qm')
        git(*commit, 'base')
        for tested_mode, alternate in ((0o755, '-x'), (0o644, '+x')):
            with self.subTest(mode=oct(tested_mode)):
                base = git('rev-parse', 'HEAD').decode().strip()
                file.chmod(tested_mode)
                tested = binding()
                data.update(binding=tested, source_base=base, deleted_sources=[])
                path.write_text(json.dumps(data), encoding='utf-8')
                anchor = v.digest(path)
                git('add', '--', file.name)
                self.assertEqual(v.verify_evidence(path, anchor, binding(), steps, root=root)['status'], 'PASS')
                git(*commit, 'record tested mode')
                self.assertEqual(v.verify_evidence(path, anchor, binding(), steps, root=root)['status'], 'PASS')
                git('update-index', '--chmod=' + alternate, file.name)
                with self.assertRaises(v.Hold):
                    v.verify_evidence(path, anchor, binding(), steps, root=root)
                git('add', '--', file.name)

    def test_different_index_or_committed_blobs_cannot_reuse_tested_bytes(self):
        path, data, _, steps = self.proof()
        root = self.work / 'checkout'
        root.mkdir()
        def git(*args):
            return v.git(root, *args)
        def binding():
            with patch.object(v, 'tools_binding', return_value={}):
                return v.binding(root, {'FLASHGATE_WORK_ROOT': str(self.work)})
        git('init', '--quiet')
        file = root / 'tested.txt'
        file.write_bytes(b'original\n')
        git('add', file.name)
        commit = ('-c', 'core.hooksPath=' + str(self.work / 'no-hooks'),
                  '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                  '-c', 'commit.gpgsign=false', 'commit', '-qm')
        git(*commit, 'base')
        data.update(binding=binding(), source_base=git('rev-parse', 'HEAD').decode().strip(), deleted_sources=[])
        path.write_text(json.dumps(data), encoding='utf-8')
        anchor = v.digest(path)
        file.write_bytes(b'other content\n')
        git('add', file.name)
        file.write_bytes(b'original\n')
        with self.subTest(state='staged'), self.assertRaisesRegex(v.Hold, 'Git index'):
            v.verify_evidence(path, anchor, binding(), steps, root=root)
        git(*commit, 'commit other content')
        with self.subTest(state='committed'), self.assertRaisesRegex(v.Hold, 'Git commit'):
            v.verify_evidence(path, anchor, binding(), steps, root=root)
        git('add', file.name)
        with self.subTest(state='committed with index restored'), self.assertRaisesRegex(v.Hold, 'Git commit'):
            v.verify_evidence(path, anchor, binding(), steps, root=root)
        git(*commit, 'commit tested content')
        self.assertEqual(v.verify_evidence(path, anchor, binding(), steps, root=root)['status'], 'PASS')

    def test_unchanged_git_postimage_reuses_across_stage_commit_and_line_normalization(self):
        path, data, _, steps = self.proof()
        root = self.work / 'checkout'
        root.mkdir()
        def git(*args):
            return v.git(root, *args)
        def binding():
            with patch.object(v, 'tools_binding', return_value={}):
                return v.binding(root, {'FLASHGATE_WORK_ROOT': str(self.work)})
        git('init', '--quiet')
        git('config', 'core.safecrlf', 'false')
        (root / '.gitattributes').write_bytes(b'*.txt text eol=lf\n')
        existing = root / 'tracked name.txt'
        existing.write_bytes(b'baseline\n')
        git('add', '.')
        commit = ('-c', 'core.hooksPath=' + str(self.work / 'no-hooks'),
                  '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                  '-c', 'commit.gpgsign=false', 'commit', '-qm')
        git(*commit, 'base')
        base = git('rev-parse', 'HEAD').decode().strip()
        existing.write_bytes(b'new content\r\n')
        (root / 'new name.txt').write_bytes(b'new file\r\n')
        current = binding()
        data.update(binding=current, source_base=base, deleted_sources=[])
        path.write_text(json.dumps(data), encoding='utf-8')
        anchor = v.digest(path)
        for state in ('unstaged', 'partially staged', 'staged', 'committed'):
            with self.subTest(state=state):
                if state == 'partially staged':
                    git('add', existing.name)
                elif state == 'staged':
                    git('add', '.')
                elif state == 'committed':
                    git(*commit, 'commit tested postimage')
                self.assertEqual(binding(), current)
                self.assertEqual(v.verify_evidence(path, anchor, current, steps, root=root)['status'], 'PASS')
        self.assertEqual(git('show', 'HEAD:' + existing.name), b'new content\n')
        self.assertEqual(git('show', 'HEAD:new name.txt'), b'new file\n')

    def test_git_projection_holds_before_external_filters(self):
        root = self.work / 'checkout'
        root.mkdir()
        v.git(root, 'init', '--quiet')
        (root / '.gitattributes').write_bytes(b'*.txt filter=external\n')
        (root / 'entry.txt').write_bytes(b'fixture\n')
        with self.assertRaisesRegex(v.Hold, 'external clean filters'):
            v.git_blobs(root, v.sources(root), self.work)
        for name in ('unset', 'unspecified'):
            with self.subTest(driver=name):
                (root / '.gitattributes').write_text('*.txt filter=' + name + '\n', encoding='utf-8')
                v.git(root, 'config', 'filter.' + name + '.clean', 'printf invoked > filter-invoked; cat')
                with self.assertRaisesRegex(v.Hold, 'could not project source blobs safely'):
                    v.git_blobs(root, v.sources(root), self.work)
                self.assertFalse((root / 'filter-invoked').exists())

    def test_ancestor_link_holds_before_source_read_or_git_projection(self):
        root = self.work / 'checkout'
        parent = root / 'parent'
        nested = parent / 'nested'
        nested.mkdir(parents=True)
        name = 'parent/nested/test.txt'
        (root / name).write_bytes(b'local fixture\n')
        v.git(root, 'init', '--quiet')
        v.git(root, 'add', name)
        accepted = v.sources(root)
        spare = self.work / 'spare'
        parent.rename(spare)
        try:
            parent.symlink_to(spare, target_is_directory=True)
        except OSError as error:
            if os.name == 'nt' and error.winerror == 1314:
                self.skipTest('Windows symlink creation is unavailable to the test account')
            raise
        with v.source_boundary(root) as boundary:
            with patch.object(v.os, 'fdopen', side_effect=AssertionError('source opened for reading')):
                with self.assertRaisesRegex(v.Hold, 'Source parent'):
                    with v.source_file(root, boundary, name):
                        self.fail('must not reach the reader')
        with patch.object(v, 'project_mirror_blobs', side_effect=AssertionError('Git projection must not start')):
            with self.assertRaises(v.Hold):
                v.git_blobs(root, accepted, self.work)
        with self.assertRaises(v.Hold):
            v.sources(root)

    def test_git_projection_checks_same_bytes_and_uses_private_copy(self):
        root = self.work / 'checkout'
        root.mkdir()
        v.git(root, 'init', '--quiet')
        file = root / 'test.txt'
        file.write_bytes(b'inspected\n')
        accepted = v.sources(root)
        with patch.object(v, 'project_mirror_blobs', wraps=v.project_mirror_blobs) as project:
            self.assertEqual(v.git_blobs(root, accepted, self.work),
                             [['test.txt', v.git(root, 'hash-object', 'test.txt').decode().strip()]])
            mirror, git_dir, inventory = project.call_args.args
            self.assertEqual(mirror.parent, self.work)
            self.assertFalse(mirror.exists())
            self.assertEqual(git_dir, v.git(root, 'rev-parse', '--absolute-git-dir').decode().strip())
            self.assertEqual(inventory, accepted)
        (root / '.gitattributes').write_bytes(b'*.txt text eol=lf\n')
        with self.assertRaisesRegex(v.Hold, 'Unbound Git attribute file'):
            v.git_blobs(root, accepted, self.work)
        accepted = v.sources(root)
        file.write_bytes(b'changed\n')
        with self.assertRaisesRegex(v.Hold, 'Source changed before Git blob projection'):
            v.git_blobs(root, accepted, self.work)

    @unittest.skipUnless(os.name == 'posix', 'Native snapshot deletion contract')
    def test_native_deletion_preserves_unrelated_files_and_rejects_links(self):
        snapshot_spec = importlib.util.spec_from_file_location(
            'snapshot', Path(__file__).with_name('validate-snapshot.py'))
        snapshot = importlib.util.module_from_spec(snapshot_spec)
        snapshot_spec.loader.exec_module(snapshot)
        (self.work / 'dir').mkdir()
        (self.work / 'dir/obsolete').write_bytes(b'old')
        keep = self.work / 'keep'
        keep.write_bytes(b'keep')
        v.remove_native_deletions(self.work, ['dir/obsolete', 'missing/obsolete'], snapshot)
        self.assertFalse((self.work / 'dir/obsolete').exists())
        self.assertFalse((self.work / 'missing').exists())
        alias = self.work / 'dir/alias'
        alias.symlink_to(keep)
        with self.assertRaisesRegex(v.Hold, 'regular file'):
            v.remove_native_deletions(self.work, ['dir/alias'], snapshot)
        self.assertTrue(alias.is_symlink())
        self.assertEqual(keep.read_bytes(), b'keep')

    def test_missing_and_manipulated_logs_rejected(self):
        path, _, current, steps = self.proof()
        (path.parent / 'actual.log').write_text('forged', encoding='utf-8')
        with self.assertRaises(v.Hold):
            self.verify_synthetic_evidence(path, v.digest(path), current, steps)
        (path.parent / 'actual.log').unlink()
        with self.assertRaises(v.Hold):
            self.verify_synthetic_evidence(path, v.digest(path), current, steps)

    def test_failed_timeout_or_incomplete_matrix_rejected(self):
        path, data, current, steps = self.proof()
        for result in ([], [{'name': 'tests', 'status': 'FAIL', 'exit_code': 1}],
                       [{'name': 'tests', 'status': 'PASS', 'exit_code': 0, 'timed_out': True}]):
            data['results'] = result
            path.write_text(json.dumps(data), encoding='utf-8')
            with self.assertRaises(v.Hold):
                self.verify_synthetic_evidence(path, v.digest(path), current, steps)

    def test_duplicate_json_rejected(self):
        path = self.work / 'duplicate.json'
        path.write_text('{"status":"FAIL","status":"PASS"}', encoding='utf-8')
        with self.assertRaises(v.Hold):
            v.load(path)

    def test_fast_test_only_is_focused_and_unknown_changes_are_conservative(self):
        tools = {name: {'path': name} for name in ('go', 'pwsh', 'golangci-lint')}
        focused = v.plan('FAST', ['internal/fs/read_test.go'], self.work, self.work, tools)
        self.assertEqual(focused[-1]['argv'], ['go', 'test', '-json', './internal/fs'])
        broad = v.plan('FAST', ['internal/fs/read.go'], self.work, self.work, tools)
        self.assertEqual(broad[-1]['argv'][-1], './...')
        with self.assertRaises(v.Hold):
            v.plan('FAST', [], self.work, self.work, tools)

    def test_candidate_retains_distinct_gates(self):
        tools = {name: {'path': name} for name in ('go', 'pwsh', 'golangci-lint')}
        steps = v.plan('CANDIDATE', [], self.work, self.work, tools)
        names = {s['name'] for s in steps}
        self.assertTrue({'coverage', 'mcp', 'vet', 'lint', 'format', 'build', 'smoke-read', 'startup-negative', 'shell-fixtures'} <= names)
        self.assertEqual('race' in names, os.name != 'nt')
        self.assertFalse(next(s for s in steps if s['name'] == 'documentation')['heavy'])
        self.assertEqual('snapshot-fixtures' in names, os.name == 'nt')

    def test_native_configuration_rejects_escaping_reuse_source(self):
        config = dict(distribution='fixture', user='fixture', task_root='/home/fixture/task',
                      go='/home/fixture/go', lint='/home/fixture/lint', pwsh='/home/fixture/pwsh',
                      source_root='/home/fixture/task/stage/source')
        path = self.work / 'native.json'
        v.write(path, config)
        self.assertEqual(v.native_config(path)['source_root'], config['source_root'])
        for unsafe in ('/mnt/c/source', '/home/fixture/task/../other', None):
            config['source_root'] = unsafe
            path.write_text(json.dumps(config), encoding='utf-8')
            with self.assertRaises(v.Hold):
                v.native_config(path)

    def test_fast_interface_dispatches_selected_package(self):
        tools = {name: {'path': name} for name in ('go', 'pwsh', 'golangci-lint')}
        current = {'tools': tools, 'sources': []}
        def execute(root, work, name, argv, env, **options):
            (work / 'actual.log').write_text('executed fixture', encoding='utf-8')
            return {'status': 'PASS', 'exit_code': 0, 'timed_out': False, 'seconds': 0}
        with patch.object(v, 'environment', return_value={}), patch.object(v, 'binding', return_value=current), \
                patch.object(v, 'deleted_sources', return_value=[]), \
                patch.object(v, 'verify_git_postimage'), \
                patch.object(v, 'heavy_lock', return_value=contextlib.nullcontext()), \
                patch.object(v, 'assert_no_competing_go'), patch.object(v, 'run_process', side_effect=execute) as process, \
                contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(v.main(['--mode', 'FAST', '--work-root', str(self.work),
                                     '--changed', 'internal/fs/read_test.go']), 0)
            self.assertEqual(process.call_args.args[3], ['go', 'test', '-json', './internal/fs'])
            config = self.work / 'fast-native.json'
            v.write(config, dict(distribution='fixture', user='fixture', task_root='/home/fixture/task',
                                 go='/home/fixture/go', lint='/home/fixture/lint', pwsh='/home/fixture/pwsh'))
            process.reset_mock()
            self.assertEqual(v.main(['--mode', 'FAST', '--work-root', str(self.work),
                                     '--changed', 'internal/fs/read_test.go', '--native-config', str(config)]), 0)
            self.assertEqual(process.call_count, 1)
            self.assertEqual(process.call_args.args[2], 'focused-tests')

    def test_publish_interface_rejects_synthetic_and_missing_review(self):
        path, data, current, steps = self.proof()
        config = dict(distribution='fixture', user='fixture', task_root='/home/fixture/task',
                      go='/home/fixture/go', lint='/home/fixture/lint', pwsh='/home/fixture/pwsh')
        current.update(tools={}, native_config=config)
        native = {'name': 'native-linux', 'argv': ['native-config', v.identity(config)], 'heavy': True}
        data['plan'] = steps + [native]
        data['results'].append({'name': 'native-linux', 'status': 'PASS', 'exit_code': 0, 'timed_out': False})
        config_path, review_path = self.work / 'native.json', self.work / 'test-only-review.json'
        manifest_path = self.work / 'manifest.json'
        v.write(config_path, config)
        v.write(manifest_path, {})
        v.write(review_path, {'state': 'PASS', 'actor': 'Classic', 'candidate_sha256': v.identity(current), 'TEST_ONLY': True})
        data['artifacts'] = v.artifact_inventory(path.parent)
        path.write_text(json.dumps(data), encoding='utf-8')
        args = ['--mode', 'PUBLISH', '--work-root', str(self.work), '--evidence', str(path),
                '--evidence-sha256', v.digest(path), '--native-config', str(config_path),
                '--review', str(review_path), '--review-sha256', v.digest(review_path)]
        args += ['--review-manifest', str(manifest_path)]
        with patch.object(v, 'environment', return_value={}), patch.object(v, 'binding', return_value=current), \
                patch.object(v, 'changed_paths', return_value=[]), patch.object(v, 'plan', side_effect=lambda *unused: list(steps)), \
                patch.object(v, 'verify_source_postimage'), \
                patch.object(v, 'check_native_reuse') as native_probe, \
                patch.object(v, 'run_process', side_effect=AssertionError('no quality process')), \
                contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(v.Hold, 'test/fixture markers'):
                v.main(args)
            self.assertTrue(native_probe.called)
            with self.assertRaisesRegex(v.Hold, 'Independent Classic decision'):
                v.main(args[:-6])  # No independent decision.
            # The success branch uses a test double at the external decision
            # boundary, never a fabricated decision accepted by the real path.
            with patch.object(v, 'verify_classic_review') as decision_boundary:
                self.assertEqual(v.main(args), 0)
                self.assertEqual(decision_boundary.call_args.args[0], str(review_path))

    def test_external_classic_contract_binds_complete_source_scope_and_evidence(self):
        path, _, _, _ = self.proof()
        source = self.work / 'source.go'
        source.write_text('package fixture\n', encoding='utf-8')
        with patch.object(v, 'git', side_effect=[b'source.go\0obsolete.go\0', b'']):
            current = {'sources': v.sources(self.work)}
        manifest = dict(ContractVersion='1.0.0', TaskId='VALIDATION-123', Entries=[dict(
            Path=str(source), SHA256=v.digest(source), FileType='REGULAR_FILE', Mode=source.stat().st_mode & 0o777,
            PackagePath='source/current/source.go')], TechnicalCandidate=dict(Scope='FULL_WORK_ITEM:VALIDATION-123',
            BaseBindings=[], DependencyBindings=[], PayloadBindings=[dict(Path='evidence/summary.json',
            SHA256=v.digest(path), Length=path.stat().st_size)]))
        # In-memory nominal contract double only. No decision file, PUBLISH
        # success receipt or independently approved candidate is manufactured.
        review = dict(ContractVersion='1.0.0', EvidenceClass='CLASSIC_REVIEW_OUTCOME', TaskId='VALIDATION-123',
            ArtifactClass='TECHNICAL_IMPLEMENTATION', TransitionType='IMPLEMENTATION_TO_INDEPENDENT_FULL_REVIEW',
            HandoffSHA256='a' * 64, ReviewActorClass='CHATGPT_CLASSIC', SelfApproval=False, ReviewOutcome='PASS',
            ClassicFocusedDeltaReviewOutcome='NOT_APPLICABLE', ReviewedPostimageIdentity=v.reviewed_candidate_identity(manifest),
            ReviewedCommitIdentity=None)
        self.assertIsNone(v.validate_classic_outcome(review, manifest, self.work, current, path))
        for field, value in [('TEST_ONLY', True), ('synthetic', True), ('fixture', True), ('self_review', True),
                             ('SelfApproval', True), ('ReviewActorClass', 'CODEX'), ('ReviewOutcome', 'CHANGES_REQUIRED'),
                             ('TaskId', 'OTHER-123'), ('ReviewedPostimageIdentity', '0' * 64),
                             ('ClassicFocusedDeltaReviewOutcome', 'PASS')]:
            altered = {**review, field: value}
            with self.subTest(field=field), self.assertRaises(v.Hold):
                v.validate_classic_outcome(altered, manifest, self.work, current, path)
        for change in ('scope', 'source', 'mode', 'evidence', 'omitted'):
            altered = copy.deepcopy(manifest)
            if change == 'scope':
                altered['TechnicalCandidate']['Scope'] = 'PARTIAL:VALIDATION-123'
            elif change == 'source':
                altered['Entries'][0]['SHA256'] = '0' * 64
            elif change == 'mode':
                altered['Entries'][0]['Mode'] ^= 0o100
            elif change == 'evidence':
                altered['TechnicalCandidate']['PayloadBindings'] = []
            else:
                altered['Entries'] = []
            rebound = {**review, 'ReviewedPostimageIdentity': v.reviewed_candidate_identity(altered)}
            with self.subTest(change=change), self.assertRaises(v.Hold):
                v.validate_classic_outcome(rebound, altered, self.work, current, path)

    def test_extra_evidence_and_foreign_process_are_rejected(self):
        path, _, current, steps = self.proof()
        (path.parent / 'unrecorded.txt').write_text('extra', encoding='utf-8')
        with self.assertRaises(v.Hold):
            self.verify_synthetic_evidence(path, v.digest(path), current, steps)
        if os.name == 'nt':
            fake = subprocess.CompletedProcess([], 0, stdout=b'"go.exe","123"\r\n')
            with patch.object(v.subprocess, 'run', return_value=fake):
                with self.assertRaises(v.Hold):
                    v.assert_no_competing_go()

    def test_scoped_lock_blocks_same_checkout_but_not_independent_checkout(self):
        # A second process is needed: POSIX locks in one process may coalesce.
        code = ('import importlib.util,pathlib,sys; '
                's=importlib.util.spec_from_file_location("v",sys.argv[1]); '
                'v=importlib.util.module_from_spec(s); s.loader.exec_module(v); '
                'c=v.heavy_lock(pathlib.Path(sys.argv[2]),pathlib.Path(sys.argv[3])); '
                'c.__enter__(); c.__exit__(None,None,None)')
        checkout, other_checkout, other_work = (self.work / name for name in ('checkout', 'other-checkout', 'other-work'))
        for directory in (checkout, other_checkout, other_work):
            directory.mkdir()
        with v.heavy_lock(self.work, checkout):
            result = subprocess.run([sys.executable, '-B', '-c', code, v.__file__, str(other_work), str(checkout)],
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'Checkout heavy validation', result.stderr)
            with v.heavy_lock(other_work, other_checkout):
                pass
            with contextlib.nullcontext():
                self.assertTrue(True)  # lightweight gates take no heavy lock
        released = subprocess.run([sys.executable, '-B', '-c', code, v.__file__, str(other_work), str(checkout)],
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15)
        self.assertEqual(released.returncode, 0, released.stderr.decode(errors='replace'))

    @unittest.skipIf(os.name == 'nt', 'Native directory descriptor lock contract')
    def test_linux_lock_never_opens_a_legacy_symlink_or_external_file(self):
        checkout, resource = self.work / 'checkout', self.work / 'resource'
        checkout.mkdir()
        resource.mkdir()
        external = self.work / 'external'
        external.write_bytes(b'')
        legacy_lock = resource / ('heavy-' + v.identity(str(checkout)) + '.lock')
        legacy_lock.symlink_to(external)
        # Any legacy file access fails the regression before a redirected open.
        # Directory-relative os.open and flock remain real native operations.
        with patch.object(Path, 'open', side_effect=AssertionError('Unexpected file access')):
            with v.heavy_lock(resource, checkout):
                pass
        self.assertEqual(external.read_bytes(), b'')
        self.assertTrue(legacy_lock.is_symlink())
        linked_checkout = self.work / 'linked-checkout'
        linked_checkout.symlink_to(checkout, target_is_directory=True)
        with self.assertRaises(v.Hold):
            with v.heavy_lock(resource, linked_checkout):
                self.fail('Symlink checkout was accepted')

    def test_actual_success_failure_timeout_and_unknown_process(self):
        root = Path(__file__).resolve().parent.parent
        env = v.environment(self.work)
        success = v.run_process(root, self.work, 'success', [sys.executable, '-B', '-c', 'print("ok")'], env, 30)
        self.assertEqual((success['status'], success['exit_code']), ('PASS', 0))
        failure = v.run_process(root, self.work, 'failure', [sys.executable, '-B', '-c', 'raise SystemExit(7)'], env, 30)
        self.assertEqual((failure['status'], failure['exit_code']), ('FAIL', 7))
        timeout = v.run_process(root, self.work, 'timeout', [sys.executable, '-B', '-c', 'import time; time.sleep(30)'], env, .2)
        self.assertTrue(timeout['timed_out'])
        self.assertNotEqual(timeout['status'], 'PASS')
        if os.name == 'nt':
            with patch.object(v, 'load', return_value={'status': 'PASS', 'exit_code': None,
                                                     'failure_reason': '', 'timed_out': False}):
                unknown = v.run_process(root, self.work, 'unknown', [sys.executable, '-B', '-c', 'pass'], env, 30)
                self.assertEqual(unknown['status'], 'HOLD')


if __name__ == '__main__':
    unittest.main()
