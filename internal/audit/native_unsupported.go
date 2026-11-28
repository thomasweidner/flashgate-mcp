//go:build !windows && !linux

package audit

import "os"

func lockFile(*os.File, bool) (bool, error)     { return false, ErrUnavailable }
func unlockFile(*os.File)                       {}
func ownedNode(*os.File, bool) bool             { return false }
func syncDirectory(*os.Root, string) error      { return ErrUnavailable }
func nativeSamePath(a, b string) bool           { return a == b }
func makeStoreDirectory(*os.Root, string) error { return ErrUnavailable }
func makeStorePartition(*os.Root, string) error { return ErrUnavailable }
func createStoreFile(*os.Root, string) (*os.File, error) {
	return nil, ErrUnavailable
}
func secureOpen(*os.Root, string, int, os.FileMode, bool) (*os.File, error) {
	return nil, ErrUnavailable
}
