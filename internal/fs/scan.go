package fs

import (
	"io"
	"os"
	"path/filepath"

	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

// scanTreeWithMeter visits each directory entry once without eagerly allocating an
// entire directory. Names (including filtered/link entries) consume bytes;
// entries independently consume the operation's traversal bound. No contents
// are read and directory symlinks are never followed.
func (f *LocalFileSystem) scanTreeWithMeter(root string, maxEntries int, visit func(string, os.DirEntry) error, meter *resource.Meter) error {
	if meter == nil {
		var err error
		meter, err = resource.NewMeter(f.limits.ResourceLimits())
		if err != nil {
			return err
		}
	}
	remaining := maxEntries
	rootInfo, err := effectInfo(root)
	if err != nil {
		return err
	}
	if rootInfo == nil || !rootInfo.IsDir() {
		return ErrPathIsNotDirectory
	}
	type directorySnapshot struct {
		path       string
		info       os.FileInfo
		structural bool
	}
	queue := []directorySnapshot{{root, rootInfo, false}}
	queueRemaining := maxListResultCost - len(root) - listEntryCost
	for len(queue) != 0 {
		next := queue[len(queue)-1]
		queue[len(queue)-1] = directorySnapshot{}
		queue = queue[:len(queue)-1]
		directory := next.path
		queueRemaining += len(directory) + listEntryCost
		operand, err := f.inspectEffectContext(directory, true, next.structural)
		if err != nil {
			return err
		}
		if !sameEffectInfo(operand.info, next.info) {
			return ErrMovePathChanged
		}
		if err := f.recheckTreeEntry(treeEffect{effectPath: operand}); err != nil {
			return err
		}
		file, err := f.openNative(operand, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		err = func() error {
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return err
			}
			if !sameEffectInfo(info, operand.info) {
				return ErrMovePathChanged
			}
			if err := f.recheckTreeEntry(treeEffect{effectPath: operand}); err != nil {
				return err
			}
			for {
				batch, readErr := file.ReadDir(min(listBatchSize, max(1, remaining)))
				if readErr != nil && readErr != io.EOF {
					return readErr
				}
				if len(batch) > remaining {
					return ErrLimitExceeded
				}
				remaining -= len(batch)
				for _, e := range batch {
					if err := meter.Charge(resource.Scan, int64(len(e.Name()))); err != nil {
						return err
					}
					p := filepath.Join(directory, e.Name())
					if err := visit(p, e); err != nil {
						return err
					}
					if e.IsDir() {
						child, err := f.inspectPath(p)
						if err != nil {
							return err
						}
						info := child.info
						if info == nil || !info.IsDir() {
							return ErrMovePathChanged
						}
						cost := len(p) + listEntryCost
						if cost > queueRemaining {
							return ErrLimitExceeded
						}
						queueRemaining -= cost
						queue = append(queue, directorySnapshot{p, info, true})
					}
				}
				if readErr == io.EOF {
					return f.recheckTreeEntry(treeEffect{effectPath: operand})
				}
			}
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
