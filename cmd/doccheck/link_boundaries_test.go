package main

import "testing"

func TestRuntimeLinkBoundaryExceptions(t *testing.T) {
	for _, tc := range []struct {
		path, target, body string
		allow              bool
	}{
		{"internal/security/policy_windows.go", "windows/amd64", `var _=syscall.NewLazyDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")`, true},
		{"internal/security/policy_windows.go", "windows/arm64", `var _=syscall.NewLazyDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")`, true},
		{"internal/fs/native_windows.go", "windows/amd64", `var _=syscall.NewLazyDLL("kernel32.dll").NewProc("CreateProcessW")`, false},
		{"internal/security/policy_windows.go", "windows/amd64", `var _=syscall.NewLazyDLL("other.dll").NewProc("GetFileInformationByHandleEx")`, false},
		{"internal/security/policy_windows.go", "linux/amd64", `var _=syscall.NewLazyDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")`, false},
		{"other.go", "windows/amd64", `var _=syscall.NewLazyDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")`, false},
		{"internal/fs/native_linux.go", "linux/amd64", `func nativeOpenAt(){syscall.Syscall6(437,0,0,0,0,0,0)}`, true},
		{"internal/fs/native_linux.go", "linux/arm64", `func nativeOpenAt(){syscall.Syscall6(437,0,0,0,0,0,0)}`, true},
		{"internal/fs/native_linux.go", "linux/amd64", `func nativeOpenAt(){syscall.Syscall6(59,0,0,0,0,0,0)}`, false},
		{"internal/fs/native_linux.go", "linux/amd64", `func other(){syscall.Syscall6(437,0,0,0,0,0,0)}`, false},
		{"other.go", "linux/amd64", `func nativeOpenAt(){syscall.Syscall6(437,0,0,0,0,0,0)}`, false},
		{"internal/fs/native_linux.go", "windows/amd64", `func nativeOpenAt(){syscall.Syscall6(437,0,0,0,0,0,0)}`, false},
		{"internal/fs/native_linux.go", "linux/amd64", `var _=syscall.Syscall6`, false},
		{"internal/fs/native_linux.go", "linux/amd64", `func(p *nativePath)remove(){syscall.Syscall(syscall.SYS_UNLINKAT,0,0,0)}`, true},
		{"internal/fs/native_linux.go", "linux/amd64", `func(p *nativePath)remove(){syscall.Syscall(syscall.SYS_EXECVE,0,0,0)}`, false},
	} {
		source := []byte("package fixture\nimport \"syscall\"\ntype nativePath struct{}\n" + tc.body)
		if got := inspectRuntimeSource(tc.path, tc.target, source); (len(got) == 0) != tc.allow {
			t.Fatalf("%+v: %+v", tc, got)
		}
	}
}
