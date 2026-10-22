package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stageTestBinding(t *testing.T) (*stageRename, func(string) error) {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "parent")
	stage := filepath.Join(parent, "stage")
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := bindStageRename(stage, filepath.Join(parent, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "bytes"), []byte("verified bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	return b, func(path string) error {
		data, err := os.ReadFile(filepath.Join(path, "bytes"))
		if err != nil {
			return err
		}
		if string(data) != "verified bytes" {
			return errors.New("bound bytes changed")
		}
		return nil
	}
}

func stageTestOps(rename func(string, string) error) (stageRenameOps, *time.Time, *int) {
	now := time.Unix(100, 0)
	sleeps := 0
	return stageRenameOps{rename, func() time.Time { return now }, func(d time.Duration) {
		sleeps++
		now = now.Add(d)
	}, func(error) bool { return true }, 8, 2 * time.Second}, &now, &sleeps
}

func TestStageRenameSuccessAndBudgets(t *testing.T) {
	for _, failures := range []int{0, 1, 4, 8} {
		t.Run(fmt.Sprint(failures), func(t *testing.T) {
			b, verify := stageTestBinding(t)
			calls, verifications := 0, 0
			injected := errors.New("injected transient")
			ops, _, sleeps := stageTestOps(func(from, to string) error {
				calls++
				if calls <= failures {
					return injected
				}
				return os.Rename(from, to)
			})
			err := b.renameWithOps(func(path string) error { verifications++; return verify(path) }, ops)
			if failures < 8 {
				if err != nil || calls != failures+1 || *sleeps != failures || verifications != failures {
					t.Fatalf("calls=%d sleeps=%d verifies=%d: %v", calls, *sleeps, verifications, err)
				}
				if err := errors.Join(b.ownedTarget(), verify(b.target), b.cleanup()); err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, injected) || calls != 8 || *sleeps != 7 || !strings.Contains(err.Error(), "attempts=8") {
					t.Fatalf("calls=%d sleeps=%d: %v", calls, *sleeps, err)
				}
				if err := b.cleanup(); err != nil {
					t.Fatal(err)
				}
				if err := errors.Join(stageAbsent(b.stage), stageAbsent(b.target)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestStageRenameTimeBudget(t *testing.T) {
	for _, phase := range []string{"rename", "sleep", "verify", "insufficient_backoff"} {
		t.Run(phase, func(t *testing.T) {
			b, verify := stageTestBinding(t)
			calls := 0
			injected := errors.New("slow transient")
			ops, now, _ := stageTestOps(nil)
			ops.rename = func(string, string) error {
				calls++
				if phase == "rename" {
					*now = now.Add(3 * time.Second)
				}
				if phase == "insufficient_backoff" {
					*now = now.Add(1990 * time.Millisecond)
				}
				return injected
			}
			if phase == "sleep" {
				ops.sleep = func(time.Duration) { *now = now.Add(2 * time.Second) }
			}
			err := b.renameWithOps(func(path string) error {
				if phase == "verify" {
					*now = now.Add(2 * time.Second)
				}
				return verify(path)
			}, ops)
			if calls != 1 || !errors.Is(err, injected) || !strings.Contains(err.Error(), "window exhausted") {
				t.Fatalf("calls=%d: %v", calls, err)
			}
		})
	}
}

func TestStageRenameDriftRetainsRecovery(t *testing.T) {
	for _, mutation := range []string{"missing", "exchanged", "content", "target_dir", "target_file", "target_link", "stage_link", "parent", "parent_link", "error_after_move", "success_without_move", "success_wrong_target", "success_stage_reappears"} {
		t.Run(mutation, func(t *testing.T) {
			b, verify := stageTestBinding(t)
			calls := 0
			injected := errors.New("transient with drift")
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			ops, _, _ := stageTestOps(func(from, to string) error {
				calls++
				switch mutation {
				case "missing":
					must(os.RemoveAll(from))
				case "exchanged":
					must(os.Rename(from, from+"-original"))
					must(os.Mkdir(from, 0o700))
				case "content":
					must(os.WriteFile(filepath.Join(from, "bytes"), []byte("changed"), 0o600))
				case "target_dir":
					must(os.Mkdir(to, 0o700))
				case "target_file":
					must(os.WriteFile(to, []byte("foreign"), 0o600))
				case "target_link":
					must(os.Symlink(from, to))
				case "stage_link":
					must(os.Rename(from, from+"-original"))
					must(os.Symlink(from+"-original", from))
				case "parent", "parent_link":
					parent := filepath.Dir(from)
					must(os.Rename(parent, parent+"-original"))
					if mutation == "parent_link" {
						must(os.Symlink(parent+"-original", parent))
					} else {
						must(os.Mkdir(parent, 0o700))
						must(os.Mkdir(from, 0o700))
					}
				case "error_after_move":
					must(os.Rename(from, to))
				case "success_without_move":
					return nil
				case "success_wrong_target":
					must(os.Rename(from, from+"-original"))
					must(os.Mkdir(to, 0o700))
					return nil
				case "success_stage_reappears":
					must(os.Rename(from, to))
					must(os.Mkdir(from, 0o700))
					return nil
				}
				return injected
			})
			err := b.renameWithOps(verify, ops)
			if err == nil || calls != 1 || !b.retained || b.promoted {
				t.Fatalf("calls=%d retained=%t promoted=%t: %v", calls, b.retained, b.promoted, err)
			}
			if !strings.HasPrefix(mutation, "success_") && !errors.Is(err, injected) {
				t.Fatal("original error lost", err)
			}
			// Cleanup must report retention and leave the mutated state intact.
			if err := b.cleanup(); err == nil {
				t.Fatal("ambiguous cleanup succeeded")
			}
			if mutation == "target_file" {
				data, err := os.ReadFile(b.target)
				if err != nil || string(data) != "foreign" {
					t.Fatal("foreign target changed", err)
				}
			}
			if mutation == "exchanged" {
				if _, err := os.Stat(b.stage); err != nil {
					t.Fatal("foreign stage removed", err)
				}
			}
		})
	}
}

func TestStageRenameBindingAndCleanupFailClosed(t *testing.T) {
	b, _ := stageTestBinding(t)
	if _, err := bindStageRename(b.stage, b.stage); err == nil {
		t.Fatal("same path accepted")
	}
	if _, err := bindStageRename(b.stage, filepath.Join(t.TempDir(), "target")); err == nil {
		t.Fatal("non-sibling accepted")
	}
	if err := os.WriteFile(b.target, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := bindStageRename(b.stage, b.target); err == nil {
		t.Fatal("existing target accepted")
	}
	if err := b.cleanup(); err == nil {
		t.Fatal("cleanup accepted changed pre-state")
	}
	if _, err := os.Stat(b.stage); err != nil {
		t.Fatal("stage evidence removed", err)
	}
}
