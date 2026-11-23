package fsapp

import (
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

const (
	listDirectory execution.OperationID = "filesystem.list"
	readFile      execution.OperationID = "filesystem.read"
	pathInfo      execution.OperationID = "filesystem.stat"
	writeFile     execution.OperationID = "filesystem.write"
	makeDirectory execution.OperationID = "filesystem.mkdir"
	deletePath    execution.OperationID = "filesystem.delete"
	copyPath      execution.OperationID = "filesystem.copy"
	movePath      execution.OperationID = "filesystem.move"
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
	entries []fs.Entry
	content []byte
	info    fs.Metadata
	created bool
}

type requestedResource struct {
	operand
	role   execution.OperandRole
	action fs.Operation
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
