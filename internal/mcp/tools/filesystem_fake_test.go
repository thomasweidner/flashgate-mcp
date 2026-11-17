package tools

import (
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

type fakeFileSystem struct {
	limits          *resource.Limits
	entries         []fs.Entry
	err             error
	listPath        string
	readPath        string
	readMaxBytes    int64
	readContent     []byte
	readErr         error
	statPath        string
	statMetadata    fs.Metadata
	statErr         error
	writePath       string
	writeContent    []byte
	writeOverwrite  bool
	writeErr        error
	mkdirPath       string
	mkdirCreated    bool
	mkdirErr        error
	deletePath      string
	deleteRecursive bool
	deleteErr       error
	moveSource      string
	moveTarget      string
	moveOverwrite   bool
	moveErr         error
	copySource      string
	copyTarget      string
	copyOverwrite   bool
	copyErr         error
}

func (f *fakeFileSystem) LimitsForRoot(config.RootID) (resource.Limits, error) {
	if f.limits != nil {
		return *f.limits, nil
	}
	return fs.DefaultLimits().ResourceLimits(), nil
}

func newFakeFileSystem() *fakeFileSystem { return &fakeFileSystem{} }
func (f *fakeFileSystem) List(path fs.Operand) ([]fs.Entry, error) {
	f.listPath = path.Path
	return f.entries, f.err
}
func (f *fakeFileSystem) Read(path fs.Operand, maxBytes int64) ([]byte, error) {
	f.readPath, f.readMaxBytes = path.Path, maxBytes
	return f.readContent, f.readErr
}
func (f *fakeFileSystem) Stat(path fs.Operand) (fs.Metadata, error) {
	f.statPath = path.Path
	return f.statMetadata, f.statErr
}
func (f *fakeFileSystem) Write(path fs.Operand, content []byte, overwrite bool) error {
	if path.AdmitEffect != nil && f.writeErr == nil {
		if err := path.AdmitEffect(); err != nil {
			return err
		}
	}
	f.writePath, f.writeContent, f.writeOverwrite = path.Path, append([]byte(nil), content...), overwrite
	return f.writeErr
}
func (f *fakeFileSystem) Mkdir(path fs.Operand) (bool, error) {
	if path.AdmitEffect != nil && f.mkdirErr == nil {
		if err := path.AdmitEffect(); err != nil {
			return false, err
		}
	}
	f.mkdirPath = path.Path
	return f.mkdirCreated, f.mkdirErr
}
func (f *fakeFileSystem) Delete(path fs.Operand, recursive bool) error {
	if path.AdmitEffect != nil && f.deleteErr == nil {
		if err := path.AdmitEffect(); err != nil {
			return err
		}
	}
	f.deletePath, f.deleteRecursive = path.Path, recursive
	return f.deleteErr
}
func (f *fakeFileSystem) Move(source fs.Operand, target fs.Operand, overwrite bool) error {
	if target.AdmitEffect != nil && f.moveErr == nil {
		if err := target.AdmitEffect(); err != nil {
			return err
		}
	}
	f.moveSource, f.moveTarget, f.moveOverwrite = source.Path, target.Path, overwrite
	return f.moveErr
}
func (f *fakeFileSystem) Copy(source fs.Operand, target fs.Operand, overwrite bool) error {
	if target.AdmitEffect != nil && f.copyErr == nil {
		if err := target.AdmitEffect(); err != nil {
			return err
		}
	}
	f.copySource, f.copyTarget, f.copyOverwrite = source.Path, target.Path, overwrite
	return f.copyErr
}
