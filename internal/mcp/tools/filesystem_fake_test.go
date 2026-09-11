package tools

import "github.com/thomasweidner/flashgate-mcp/internal/fs"

type fakeFileSystem struct {
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
	editPath        string
	editStart       int64
	editEnd         int64
	editContent     []byte
	editSize        int64
	editErr         error
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

func newFakeFileSystem() *fakeFileSystem { return &fakeFileSystem{} }
func (f *fakeFileSystem) List(path string) ([]fs.Entry, error) {
	f.listPath = path
	return f.entries, f.err
}
func (f *fakeFileSystem) Read(path string, maxBytes int64) ([]byte, error) {
	f.readPath, f.readMaxBytes = path, maxBytes
	return f.readContent, f.readErr
}
func (f *fakeFileSystem) Stat(path string) (fs.Metadata, error) {
	f.statPath = path
	return f.statMetadata, f.statErr
}
func (f *fakeFileSystem) Write(path string, content []byte, overwrite bool) error {
	f.writePath, f.writeContent, f.writeOverwrite = path, append([]byte(nil), content...), overwrite
	return f.writeErr
}
func (f *fakeFileSystem) EditRange(path string, startByte, endByte int64, content []byte) (int64, error) {
	f.editPath, f.editStart, f.editEnd = path, startByte, endByte
	f.editContent = append([]byte(nil), content...)
	return f.editSize, f.editErr
}
func (f *fakeFileSystem) Mkdir(path string) (bool, error) {
	f.mkdirPath = path
	return f.mkdirCreated, f.mkdirErr
}
func (f *fakeFileSystem) Delete(path string, recursive bool) error {
	f.deletePath, f.deleteRecursive = path, recursive
	return f.deleteErr
}
func (f *fakeFileSystem) Move(source string, target string, overwrite bool) error {
	f.moveSource, f.moveTarget, f.moveOverwrite = source, target, overwrite
	return f.moveErr
}
func (f *fakeFileSystem) Copy(source string, target string, overwrite bool) error {
	f.copySource, f.copyTarget, f.copyOverwrite = source, target, overwrite
	return f.copyErr
}
