//go:build linux

package managedprocess

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxAdapterRequiresDelegatedControllers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte("memory"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := LinuxAdapter{CgroupRoot: root}
	if leaf, directory, err := adapter.prepare(ResourceLimits{CPURate: 100}); err == nil || leaf != nil || directory != "" {
		t.Fatalf("prepare() = (%v, %q, %v), want controller denial", leaf, directory, err)
	}
}

func TestLinuxAdapterRejectsMissingDelegation(t *testing.T) {
	adapter := LinuxAdapter{}
	launch := validTestLaunch()
	launch.Resources = ResourceLimits{MemoryBytes: 1024}
	if started, err := adapter.Start(launch, io.Discard, io.Discard); started != nil || err == nil {
		t.Fatalf("Start() = (%v, %v), want fail-closed denial", started, err)
	}
}

func TestLinuxAdapterPreparesPortableControls(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte("cpu memory"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := LinuxAdapter{CgroupRoot: root}
	leaf, directory, err := adapter.prepare(ResourceLimits{CPURate: 1250, MemoryBytes: 64 << 20})
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	defer os.RemoveAll(directory)
	defer leaf.Close()
	for name, want := range map[string]string{
		"cpu.max":          "125000 1000000",
		"memory.max":       "67108864",
		"memory.oom.group": "1",
	} {
		contents, readErr := os.ReadFile(filepath.Join(directory, name))
		if readErr != nil || string(contents) != want {
			t.Errorf("%s = %q, %v; want %q", name, contents, readErr, want)
		}
	}
}

func TestLinuxAdapterRejectsUnrepresentableCPURate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte("cpu"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := LinuxAdapter{CgroupRoot: root}
	if leaf, directory, err := adapter.prepare(ResourceLimits{CPURate: 1}); !errors.Is(err, ErrInvalidResourceLimits) || leaf != nil || directory != "" {
		t.Fatalf("prepare() = (%v, %q, %v), want representation denial", leaf, directory, err)
	}
}
