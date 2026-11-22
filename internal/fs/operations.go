package fs

import "github.com/thomasweidner/flashgate-mcp/internal/capability"

// Operation names the reviewed filesystem requirements shared by catalog
// admission and dispatch. Copy requires source read and target write. The
// two operands must use the same root ID, so its access must cover both.
type Operation uint8

const (
	ReadOperation Operation = iota
	WriteOperation
	CopyOperation
	operationCount
)

type Operations [operationCount]capability.Operation

func BindOperations(catalog capability.Catalog) (Operations, error) {
	var operations Operations
	for id, requirements := range [operationCount][]capability.ID{
		ReadOperation:  {capability.FilesRead},
		WriteOperation: {capability.FilesWrite},
		CopyOperation:  {capability.FilesRead, capability.FilesWrite},
	} {
		operation, err := catalog.NewOperation(capability.RiskRead, requirements...)
		if err != nil {
			return Operations{}, err
		}
		operations[id] = operation
	}
	return operations, nil
}
