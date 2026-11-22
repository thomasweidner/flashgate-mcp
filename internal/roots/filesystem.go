package roots

import "github.com/thomasweidner/flashgate-mcp/internal/fs"

var _ fs.FileSystem = (*Registry)(nil)

func (r *Registry) List(path fs.Operand) ([]fs.Entry, error) {
	backend, err := r.backendFor(path.RootID, fs.ReadOperation)
	if err != nil {
		return nil, err
	}
	return backend.List(path.Path)
}

func (r *Registry) Read(path fs.Operand, maxBytes int64) ([]byte, error) {
	backend, err := r.backendFor(path.RootID, fs.ReadOperation)
	if err != nil {
		return nil, err
	}
	return backend.Read(path.Path, maxBytes)
}

func (r *Registry) Stat(path fs.Operand) (fs.Metadata, error) {
	backend, err := r.backendFor(path.RootID, fs.ReadOperation)
	if err != nil {
		return fs.Metadata{}, err
	}
	return backend.Stat(path.Path)
}

func (r *Registry) Write(path fs.Operand, content []byte, overwrite bool) error {
	backend, err := r.backendFor(path.RootID, fs.WriteOperation)
	if err != nil {
		return err
	}
	return backend.Write(path.Path, content, overwrite, path.AdmitEffect)
}

func (r *Registry) Mkdir(path fs.Operand) (bool, error) {
	backend, err := r.backendFor(path.RootID, fs.WriteOperation)
	if err != nil {
		return false, err
	}
	return backend.Mkdir(path.Path, path.AdmitEffect)
}

func (r *Registry) Delete(path fs.Operand, recursive bool) error {
	backend, err := r.backendFor(path.RootID, fs.WriteOperation)
	if err != nil {
		return err
	}
	return backend.Delete(path.Path, recursive, path.AdmitEffect)
}

func (r *Registry) Move(source, target fs.Operand, overwrite bool) error {
	backend, err := r.pairBackend(source, target, fs.WriteOperation)
	if err != nil {
		return err
	}
	return backend.Move(source.Path, target.Path, overwrite, target.AdmitEffect)
}

func (r *Registry) Copy(source, target fs.Operand, overwrite bool) error {
	backend, err := r.pairBackend(source, target, fs.CopyOperation)
	if err != nil {
		return err
	}
	return backend.Copy(source.Path, target.Path, overwrite, target.AdmitEffect)
}

func (r *Registry) pairBackend(source, target fs.Operand, action fs.Operation) (*fs.LocalFileSystem, error) {
	if source.RootID != target.RootID {
		_, sourceKnown := r.entries[source.RootID]
		_, targetKnown := r.entries[target.RootID]
		if !sourceKnown || !targetKnown {
			return nil, fs.ErrUnknownRoot
		}
		return nil, fs.ErrCrossRootOperation
	}
	// Both operands have the same ID. Require all rights of the pair before dispatch.
	return r.backendFor(source.RootID, action)
}
