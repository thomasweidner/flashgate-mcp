//go:build linux

package fs

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLinkPolicyNativeBindMountDenied(t *testing.T) {
	const marker = "FLASHGATE_LINK_MOUNT_TEST"
	if os.Getenv(marker) != "namespace" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command("/usr/bin/unshare", "--user", "--map-root-user", "--mount", executable,
			"-test.run=^TestLinkPolicyNativeBindMountDenied$", "-test.v")
		command.Env = append(os.Environ(), marker+"=namespace", "LC_ALL=C")
		output, err := command.CombinedOutput()
		if err != nil {
			if mountNamespaceUnavailable(err, output) {
				t.Skip("native bind-mount test unavailable: unprivileged mount namespace denied; NOT PASS")
			}
			t.Fatalf("native mount child failed: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	f, root := linkFilesystem(t, true)
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), "outside-marker")
	writeTestFile(t, filepath.Join(root, "source.txt"), "ordinary")
	point := filepath.Join(root, "mount")
	mkdir(t, point)
	if err := syscall.Mount(outside, point, "", syscall.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Unmount(point, 0); err != nil {
			t.Error(err)
		}
	}()
	for _, op := range []func() error{
		func() error { _, err := f.List("mount"); return err }, func() error { _, err := f.Stat("mount/secret.txt"); return err },
		func() error { _, err := f.Read("mount/secret.txt", 100); return err }, func() error { return f.Write("mount/new.txt", nil, false) },
		func() error { _, err := f.Mkdir("mount/new"); return err }, func() error { return f.Delete("mount/secret.txt", false) },
		func() error { return f.Copy("mount/secret.txt", "output.txt", false) }, func() error { return f.Move("mount/secret.txt", "moved.txt", false) },
		func() error { return f.Copy("source.txt", "mount/output.txt", false) }, func() error { return f.Move("source.txt", "mount/moved.txt", false) },
	} {
		if err := op(); err == nil {
			t.Fatal("bind-mount route admitted")
		}
	}
	if b, err := os.ReadFile(filepath.Join(outside, "secret.txt")); err != nil || string(b) != "outside-marker" {
		t.Fatal("mount effect", err)
	}
	for _, p := range []string{"new.txt", "new", "output.txt", "moved.txt"} {
		if _, err := os.Lstat(filepath.Join(outside, p)); !os.IsNotExist(err) {
			t.Fatal("denied mount creation", p, err)
		}
	}
	late := filepath.Join(root, "late")
	mkdir(t, late)
	mounted := false
	defer func() {
		if mounted {
			if err := syscall.Unmount(late, 0); err != nil {
				t.Error(err)
			}
		}
	}()
	f.beforeNative = func(stage string) {
		if stage != "open" || mounted {
			return
		}
		if err := syscall.Mount(outside, late, "", syscall.MS_BIND, ""); err != nil {
			t.Fatal(err)
		}
		mounted = true
	}
	if err := f.Write("late/new.txt", []byte("changed"), false); err == nil || !mounted {
		t.Fatal("late bind mount was not denied", mounted, err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("late mount redirected create", err)
	}
}

func mountNamespaceUnavailable(err error, output []byte) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	// Only exact launcher capability failures are unavailable fixtures. Child
	// assertion failures, including permission errors after launch, still fail.
	for _, message := range []string{
		"unshare: unshare failed: Operation not permitted",
		"unshare: unshare failed: Permission denied",
		"unshare: write failed /proc/self/uid_map: Operation not permitted",
	} {
		if bytes.Equal(bytes.TrimSpace(output), []byte(message)) {
			return true
		}
	}
	return false
}

func TestMountNamespaceUnavailableClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		output string
		want   bool
	}{
		{"success", nil, "unshare: unshare failed: Operation not permitted", false},
		{"missing launcher", os.ErrNotExist, "", true},
		{"namespace denied", os.ErrPermission, "unshare: unshare failed: Operation not permitted\n", true},
		{"namespace permission", os.ErrPermission, "unshare: unshare failed: Permission denied\n", true},
		{"uid mapping denied", os.ErrPermission, "unshare: write failed /proc/self/uid_map: Operation not permitted\n", true},
		{"child assertion", os.ErrPermission, "--- FAIL: TestLinkPolicyNativeBindMountDenied\nOperation not permitted\n", false},
		{"extra child failure", os.ErrPermission, "unshare: write failed /proc/self/uid_map: Operation not permitted\nFAIL\n", false},
		{"unknown launch failure", os.ErrPermission, "unshare: unknown failure\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mountNamespaceUnavailable(tc.err, []byte(tc.output)); got != tc.want {
				t.Fatalf("unavailable=%t, want %t", got, tc.want)
			}
		})
	}
}
