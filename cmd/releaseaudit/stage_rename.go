package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// stageRename is confined to new candidate/evidence directory promotion. It
// never owns an existing target or the current alias. No identity handle stays
// open across rename; callers must keep their local roots free of concurrent writes.
type stageRename struct {
	stage, target string
	identity      os.FileInfo
	parents       []stageParent
	retained      bool
	promoted      bool
}

type stageParent struct {
	path string
	info os.FileInfo
}

type stageRenameOps struct {
	rename    func(string, string) error
	now       func() time.Time
	sleep     func(time.Duration)
	transient func(error) bool
	maximum   int
	window    time.Duration
}

// Bind immediately after MkdirTemp, before populating the owned stage.
func bindStageRename(stage, target string) (*stageRename, error) {
	stage, err := filepath.Abs(stage)
	if err != nil {
		return nil, err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if stage == target || filepath.Dir(stage) != filepath.Dir(target) {
		return nil, errors.New("stage promotion requires distinct siblings")
	}
	b := &stageRename{stage: stage, target: target}
	for path := filepath.Dir(stage); ; path = filepath.Dir(path) {
		info, err := stageDirectory(path)
		if err != nil {
			return nil, fmt.Errorf("stage parent binding %q: %w; stage retained at %q", path, err, stage)
		}
		b.parents = append(b.parents, stageParent{path, info})
		if filepath.Dir(path) == path {
			break
		}
	}
	b.identity, err = stageDirectory(stage)
	if err == nil {
		err = b.preState(nil)
	}
	if err != nil {
		return nil, fmt.Errorf("stage binding: %w; stage retained at %q", err, stage)
	}
	return b, nil
}

func (b *stageRename) parentState() error {
	for _, parent := range b.parents {
		info, err := stageDirectory(parent.path)
		if err != nil {
			return fmt.Errorf("parent %q: %w", parent.path, err)
		}
		if !os.SameFile(parent.info, info) {
			return fmt.Errorf("parent identity changed: %q", parent.path)
		}
	}
	return nil
}

func stageAbsent(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("expected absent path: %q", path)
}

func (b *stageRename) sameDirectory(path string) error {
	info, err := stageDirectory(path)
	if err != nil {
		return err
	}
	if !os.SameFile(b.identity, info) {
		return fmt.Errorf("stage identity changed at %q", path)
	}
	return nil
}

func (b *stageRename) preState(verify func(string) error) error {
	if err := b.parentState(); err != nil {
		return err
	}
	if err := b.sameDirectory(b.stage); err != nil {
		return err
	}
	if err := stageAbsent(b.target); err != nil {
		return err
	}
	if verify != nil {
		if err := verify(b.stage); err != nil {
			return fmt.Errorf("stage integrity: %w", err)
		}
		// Content verification opens files. Recheck identity and ancestry after
		// those handles have closed, immediately before the follow-up operation.
		return b.preState(nil)
	}
	return nil
}

func (b *stageRename) ownedTarget() error {
	if err := b.parentState(); err != nil {
		return err
	}
	if err := stageAbsent(b.stage); err != nil {
		return err
	}
	return b.sameDirectory(b.target)
}

func (b *stageRename) cleanup() error {
	if b.retained {
		return fmt.Errorf("stage recovery retained: stage=%q target=%q", b.stage, b.target)
	}
	if b.promoted {
		return stageAbsent(b.stage)
	}
	if err := b.preState(nil); err != nil {
		return fmt.Errorf("stage cleanup ownership unproven; retained: %w", err)
	}
	return os.RemoveAll(b.stage)
}

func (b *stageRename) rename(verify func(string) error) error {
	return b.renameWithOps(verify, platformStageRenameOps())
}

func (b *stageRename) renameWithOps(verify func(string) error, ops stageRenameOps) error {
	attempts := 0
	var first, last error
	fail := func(phase string, err error) error {
		return fmt.Errorf("stage promotion attempts=%d stage=%q target=%q %s: %w", attempts, b.stage, b.target, phase, errors.Join(first, last, err))
	}
	if verify == nil || ops.maximum < 1 {
		return fail("invalid contract", errors.New("missing verifier or attempt budget"))
	}
	if err := b.preState(nil); err != nil {
		b.retained = true
		return fail("pre-state unproven; recovery retained", err)
	}
	start := ops.now()
	for {
		attempts++
		err := ops.rename(b.stage, b.target)
		if err == nil {
			if err := b.ownedTarget(); err != nil {
				b.retained = true
				return fail("ambiguous post-state; recovery retained", err)
			}
			b.promoted = true
			return nil // Caller retains its full original post-promotion verifier.
		}
		last = stageRenameDiagnostic(err)
		if first == nil {
			first = last
		}
		// Even an error may have moved the directory. Never retry or clean an
		// ambiguous result, including an error returned after a completed move.
		if err := b.preState(nil); err != nil {
			b.retained = true
			return fail("ambiguous failed-rename state; recovery retained", err)
		}
		if !ops.transient(err) || attempts >= ops.maximum {
			return fail("rename rejected or attempt budget exhausted; pre-state intact", nil)
		}
		remaining := ops.window - ops.now().Sub(start)
		delay := time.Duration(attempts) * 20 * time.Millisecond
		if remaining <= delay {
			return fail("retry window exhausted; pre-state intact", nil)
		}
		ops.sleep(delay)
		if !ops.now().Before(start.Add(ops.window)) {
			return fail("retry window exhausted; pre-state intact", nil)
		}
		if err := b.preState(verify); err != nil {
			b.retained = true
			return fail("follow-up pre-state unproven; recovery retained", err)
		}
		if !ops.now().Before(start.Add(ops.window)) {
			return fail("retry window exhausted after verification; pre-state intact", nil)
		}
	}
}
