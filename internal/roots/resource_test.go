package roots

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

func TestRootResourceSnapshotsAndEffects(t *testing.T) {
	work := t.TempDir()
	source := filepath.Join(work, "source")
	if err := os.WriteFile(source, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	r := loadRegistry(t, filepath.Join(t.TempDir(), "config.toml"),
		"[files]\ndefault_id='work'\n"+rootTable(t, "work", work)+"policy='small'\n"+
			rootTable(t, "alias", work)+"access=['files.read','files.write']\nmax_temp_size=4\n"+
			"[policy.small]\naccess=['files.read','files.write']\nmax_file_size=4\nmax_write_size=4\nmax_scan_size=4\nmax_temp_size=4\n")
	l, err := r.LimitsForRoot("work")
	if err != nil {
		t.Fatal(err)
	}
	l.File = 100
	if got, _ := r.LimitsForRoot("work"); got.File != 4 {
		t.Fatal("mutable snapshot")
	}
	if _, err := r.Read(fs.Operand{RootID: "work", Path: "source"}, 100); !errors.Is(err, fs.ErrFileTooLarge) {
		t.Fatal(err)
	}
	if b, err := r.Read(fs.Operand{RootID: "alias", Path: "source"}, 100); err != nil || string(b) != "12345" {
		t.Fatal(string(b), err)
	}
	if err := r.Write(fs.Operand{RootID: "work", Path: "new"}, []byte("12345"), false); !errors.Is(err, fs.ErrLimitExceeded) {
		t.Fatal(err)
	}
	if err := r.Copy(fs.Operand{RootID: "work", Path: "source"}, fs.Operand{RootID: "work", Path: "copy"}, false); !errors.Is(err, fs.ErrLimitExceeded) {
		t.Fatal(err)
	}
	if err := r.Copy(fs.Operand{RootID: "work", Path: "source"}, fs.Operand{RootID: "alias", Path: "copy"}, false); !errors.Is(err, fs.ErrCrossRootOperation) {
		t.Fatal(err)
	}
	if _, err := r.LimitsForRoot("Work"); !errors.Is(err, fs.ErrUnknownRoot) {
		t.Fatal(err)
	}
	if _, err := r.ReserveTemp("unknown", 1); !errors.Is(err, fs.ErrUnknownRoot) {
		t.Fatal(err)
	}
	release, err := r.ReserveTemp("work", 4)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := r.ReserveTemp("work", 1); !errors.Is(err, fs.ErrLimitExceeded) {
				t.Error("concurrent aggregate bypass", err)
			}
		})
	}
	wg.Wait()
	other, err := r.ReserveTemp("alias", 4)
	if err != nil {
		t.Fatal("separate root budget", err)
	}
	other()
	release()
	release()
	release, err = r.ReserveTemp("work", 4)
	if err != nil {
		t.Fatal("release leaked", err)
	}
	release()
}

type externalResourceFixture struct {
	hard    bool
	limits  resource.Limits
	effects int
}

func (b *externalResourceFixture) RequireHardLimits(l resource.Limits, required []resource.Dimension) error {
	if !b.hard {
		return resource.ErrNotEnforceable
	}
	for _, d := range required {
		if d != resource.File && d != resource.Write && d != resource.Scan {
			return resource.ErrNotEnforceable
		}
	}
	b.limits = l
	return nil
}
func (b *externalResourceFixture) execute(n int64, dimensions ...resource.Dimension) error {
	if err := resource.Admit(b, b.limits, dimensions...); err != nil {
		return err
	}
	for _, d := range dimensions {
		if err := b.limits.Check(d, n); err != nil {
			return err
		}
	}
	b.effects++
	return nil
}

func TestGoAndSimulatedExternalResourceParity(t *testing.T) {
	root := t.TempDir()
	r := loadRegistry(t, filepath.Join(t.TempDir(), "config.toml"), "[files]\ndefault_id='work'\n"+
		rootTable(t, "work", root)+"access=['files.read','files.write']\nmax_file_size=4\nmax_write_size=4\nmax_scan_size=4\n")
	l, _ := r.LimitsForRoot("work")
	for _, op := range []string{"read", "write", "copy"} {
		for _, n := range []int{3, 4, 5} {
			content := make([]byte, n)
			source := fmt.Sprintf("%s%d", op, n)
			if err := os.WriteFile(filepath.Join(root, source), content, 0600); err != nil {
				t.Fatal(err)
			}
			var goErr error
			dimensions := []resource.Dimension{resource.File, resource.Write}
			switch op {
			case "read":
				_, goErr = r.Read(fs.Operand{RootID: "work", Path: source}, 100)
				dimensions = []resource.Dimension{resource.File, resource.Scan}
			case "write":
				goErr = r.Write(fs.Operand{RootID: "work", Path: source + "target"}, content, false)
			case "copy":
				goErr = r.Copy(fs.Operand{RootID: "work", Path: source}, fs.Operand{RootID: "work", Path: source + "target"}, false)
				dimensions = append(dimensions, resource.Scan)
			}
			external := &externalResourceFixture{hard: true, limits: l}
			externalErr := external.execute(int64(n), dimensions...)
			if (goErr == nil) != (externalErr == nil) {
				t.Fatal("backend parity", op, n, goErr, externalErr)
			}
			if n > 4 && external.effects != 0 {
				t.Fatal("external denial had effects")
			}
			unavailable := &externalResourceFixture{limits: l}
			if err := unavailable.execute(int64(n), dimensions...); !errors.Is(err, resource.ErrNotEnforceable) || unavailable.effects != 0 {
				t.Fatal("configured-only backend dispatched")
			}
		}
	}
}
