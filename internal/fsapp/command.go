package fsapp

import (
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

const (
	ListOperation   execution.OperationID = "filesystem.list"
	ReadOperation   execution.OperationID = "filesystem.read"
	StatOperation   execution.OperationID = "filesystem.stat"
	WriteOperation  execution.OperationID = "filesystem.write"
	MkdirOperation  execution.OperationID = "filesystem.mkdir"
	DeleteOperation execution.OperationID = "filesystem.delete"
	CopyOperation   execution.OperationID = "filesystem.copy"
	MoveOperation   execution.OperationID = "filesystem.move"

	listDirectory = ListOperation
	readFile      = ReadOperation
	pathInfo      = StatOperation
	writeFile     = WriteOperation
	makeDirectory = MkdirOperation
	deletePath    = DeleteOperation
	copyPath      = CopyOperation
	movePath      = MoveOperation

	overwriteVariant execution.VariantID = "overwrite"
	recursiveVariant execution.VariantID = "recursive"
)

// operand contains only domain data. In-process result admission is separate
// from the command and cannot be serialized as a delegated operation.
type operand struct {
	root config.RootID
	path string
}

func fileOperand(path fs.Operand) operand { return operand{root: path.RootID, path: path.Path} }

type command struct {
	name      execution.OperationID
	path      operand
	target    operand
	maxBytes  int64
	content   []byte
	overwrite bool
	recursive bool
}

type result struct {
	entries                 []fs.Entry
	content                 []byte
	info                    fs.Metadata
	missing                 bool
	created                 bool
	readBytes, writtenBytes *uint64
}

type requestedResource struct {
	operand
	role   execution.OperandRole
	action fs.Operation
}

func (c command) key() execution.OperationKey {
	variant := execution.DefaultVariant
	switch c.name {
	case writeFile, copyPath, movePath:
		if c.overwrite {
			variant = overwriteVariant
		}
	case deletePath:
		if c.recursive {
			variant = recursiveVariant
		}
	}
	return execution.OperationKey{Name: c.name, Variant: variant}
}

func (c command) declaration() (fs.Operation, []requestedResource, error) {
	switch c.name {
	case listDirectory, readFile, pathInfo:
		return fs.ReadOperation, []requestedResource{{c.path, execution.Single, fs.ReadOperation}}, nil
	case writeFile, makeDirectory, deletePath:
		return fs.WriteOperation, []requestedResource{{c.path, execution.Single, fs.WriteOperation}}, nil
	case copyPath:
		return fs.CopyOperation, []requestedResource{{c.path, execution.Source, fs.ReadOperation}, {c.target, execution.Target, fs.WriteOperation}}, nil
	case movePath:
		return fs.WriteOperation, []requestedResource{{c.path, execution.Source, fs.WriteOperation}, {c.target, execution.Target, fs.WriteOperation}}, nil
	default:
		return 0, nil, execution.ErrUnsupported
	}
}
