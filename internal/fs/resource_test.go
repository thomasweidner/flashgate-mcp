package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestContentResourceBounds(t *testing.T) {
	for _, dim := range []string{"file", "write", "scan"} {
		t.Run(dim, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "source"), "12345")
			l := DefaultLimits()
			switch dim {
			case "file":
				l.MaxFileSize = 4
			case "write":
				l.MaxWriteSize = 4
			case "scan":
				l.MaxScanSize = 4
			}
			f := mustNewLocalFileSystemWithLimits(t, root, l)
			if err := f.Copy("source", "copy", false); !errors.Is(err, ErrLimitExceeded) {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "copy")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("copy denial wrote target")
			}
			if dim != "scan" {
				if err := f.Write("write", []byte("12345"), false); !errors.Is(err, ErrLimitExceeded) {
					t.Fatal(err)
				}
			}
			if dim != "write" {
				if _, err := f.Read("source", 100); !errors.Is(err, ErrFileTooLarge) {
					t.Fatal(err)
				}
			}
			writeTestFile(t, filepath.Join(root, "source"), "1234")
			if err := f.Copy("source", "exact", false); err != nil {
				t.Fatal(err)
			}
			if dim != "write" {
				if data, err := f.Read("source", 4); err != nil || string(data) != "1234" {
					t.Fatal(string(data), err)
				}
			}
		})
	}
}

func TestScansChargeFilteredNamesAndPreflightTrees(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".hidden"), "content is not scanned")
	l := DefaultLimits()
	l.MaxScanSize = int64(len(".hidden"))
	f := mustNewLocalFileSystemWithLimits(t, root, l)
	if entries, err := f.List("."); err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	l.MaxScanSize--
	f = mustNewLocalFileSystemWithLimits(t, root, l)
	if _, err := f.List("."); !errors.Is(err, ErrLimitExceeded) {
		t.Fatal("filtered name not charged", err)
	}
	if err := os.Mkdir(filepath.Join(root, "tree"), 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "tree", "longname"), "x")
	for _, operation := range []string{"delete", "move"} {
		var err error
		if operation == "delete" {
			err = f.Delete("tree", true)
		} else {
			err = f.Move("tree", "moved", false)
		}
		if !errors.Is(err, ErrLimitExceeded) {
			t.Fatal(operation, err)
		}
		if _, err := os.Stat(filepath.Join(root, "tree", "longname")); err != nil {
			t.Fatal("tree changed on denial", err)
		}
	}
	l.MaxScanSize = 8
	f = mustNewLocalFileSystemWithLimits(t, root, l)
	if err := f.Delete("tree", true); err != nil {
		t.Fatal("exact tree scan", err)
	}
}

func TestRootConstructorLimitBinding(t *testing.T) {
	if _, err := NewProtectedRootFileSystems(nil, []Limits{DefaultLimits()}); err == nil {
		t.Fatal("binding mismatch")
	}
	l := DefaultLimits()
	l.MaxTempSize = 0
	if _, err := NewProtectedRootFileSystems(nil, []Limits{l}); err == nil {
		t.Fatal("invalid limit binding")
	}
}

func TestEffectAdmissionPrecedesMutations(t *testing.T) {
	for _, name := range []string{"write", "copy", "move", "delete", "mkdir"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "source"), "safe")
			f := mustNewLocalFileSystem(t, root)
			calls := 0
			deny := func() error { calls++; return ErrLimitExceeded }
			var err error
			switch name {
			case "write":
				err = f.Write("target", []byte("new"), false, deny)
			case "copy":
				err = f.Copy("source", "target", false, deny)
			case "move":
				err = f.Move("source", "target", false, deny)
			case "delete":
				err = f.Delete("source", false, deny)
			case "mkdir":
				_, err = f.Mkdir("parent/target", deny)
			}
			if !errors.Is(err, ErrLimitExceeded) || calls != 1 {
				t.Fatal(name, calls, err)
			}
			data, err := os.ReadFile(filepath.Join(root, "source"))
			if err != nil || string(data) != "safe" {
				t.Fatal("denial changed source")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatal("denial created effects")
			}
			if err := f.Write("../escape", []byte("new"), false, deny); err == nil || calls != 1 {
				t.Fatal("admission before path validation")
			}
		})
	}
	if admitEffect([]func() error{nil, nil}) == nil {
		t.Fatal("multiple admissions")
	}
}

func TestMoveInspectionKeepsImplicitHiddenChildren(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tree", ".nested"), 0700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "tree", ".nested", "file"), "payload is not scanned")
	l := DefaultLimits()
	l.MaxDeleteEntries = 1
	// Both actual inspections spend the same operation allowance. This test
	// isolates structural hidden descendants and the independent deletion cap.
	l.MaxScanSize = 2 * int64(len(".nested")+len("file"))
	f := mustNewLocalFileSystemWithLimits(t, root, l)
	if err := f.Move("tree", "moved", false); err != nil {
		t.Fatal("rename acquired a deletion or hidden-selection limit", err)
	}
	if _, err := os.Stat(filepath.Join(root, "moved", ".nested", "file")); err != nil {
		t.Fatal(err)
	}
}
