//go:build !windows && !linux

package fs

import "os"

// Other ports cannot silently inherit Windows/Linux enforcement claims.
type nativePath struct{}

func bindNativePath(*os.Root, string, string) (*nativePath, error) { return nil, os.ErrPermission }
func (*nativePath) close()                                         {}
func (*nativePath) parentInfo() (os.FileInfo, error)               { return nil, os.ErrPermission }
func (*nativePath) info() (os.FileInfo, error)                     { return nil, os.ErrPermission }
func (*nativePath) matches(os.FileInfo, os.FileInfo) error         { return os.ErrPermission }
func (*nativePath) open(int, os.FileMode) (*os.File, error)        { return nil, os.ErrPermission }
func (*nativePath) mkdir(os.FileMode) error                        { return os.ErrPermission }
func (*nativePath) remove(bool) error                              { return os.ErrPermission }
func (*nativePath) rename(*nativePath) error                       { return os.ErrPermission }
