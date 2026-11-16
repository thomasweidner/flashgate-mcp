package capability_test

import (
	"errors"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

func TestOperationRiskAndPolicy(t *testing.T) {
	catalog := mustCatalog(t, capability.BaseDefinitions()...)
	other := mustCatalog(t, capability.BaseDefinitions()...)
	read := capability.FilesRead
	write := capability.FilesWrite
	manage := capability.ProcessesManage
	control := capability.ProcessesControl

	for _, tc := range []struct {
		name    string
		require []capability.ID
		grant   []capability.ID
		floor   capability.Risk
		ceiling capability.Risk
		derived capability.Risk
		allow   bool
	}{
		{"all rights", []capability.ID{read, write}, []capability.ID{read, write}, capability.RiskRead, capability.RiskMutation, capability.RiskMutation, true},
		{"partial rights", []capability.ID{read, write}, []capability.ID{write}, capability.RiskRead, capability.RiskMutation, capability.RiskMutation, false},
		{"tightened ceiling", []capability.ID{write}, []capability.ID{write}, capability.RiskRead, capability.RiskRead, capability.RiskMutation, false},
		{"raised floor", []capability.ID{read}, []capability.ID{read}, capability.RiskHigh, capability.RiskMutation, capability.RiskHigh, false},
		{"managed process", []capability.ID{manage}, []capability.ID{manage}, capability.RiskRead, capability.RiskMutation, capability.RiskMutation, true},
		{"external process risk", []capability.ID{control}, []capability.ID{control}, capability.RiskRead, capability.RiskMutation, capability.RiskHigh, false},
		{"external process explicit high-risk grant", []capability.ID{control}, []capability.ID{control}, capability.RiskRead, capability.RiskHigh, capability.RiskHigh, true},
		{"managed grant cannot control external process", []capability.ID{control}, []capability.ID{manage}, capability.RiskRead, capability.RiskHigh, capability.RiskHigh, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			operation, err := catalog.NewOperation(tc.floor, tc.require...)
			if err != nil || operation.Risk() != tc.derived {
				t.Fatalf("operation risk = %v, error = %v", operation.Risk(), err)
			}
			policy, err := capability.NewPolicy(mustSet(t, catalog, tc.grant...), tc.ceiling)
			if err != nil || policy.Allows(operation) != tc.allow {
				t.Fatalf("unexpected operation decision: %v", err)
			}
			foreign, err := capability.NewPolicy(mustSet(t, other, tc.require...), capability.RiskHigh)
			if err != nil || foreign.Allows(operation) || policy.Allows(capability.Operation{}) || (capability.Policy{}).Allows(operation) {
				t.Fatal("missing or unrelated snapshot satisfied the operation")
			}
		})
	}
	if _, err := catalog.NewOperation(0, read); !errors.Is(err, capability.ErrInvalidRisk) {
		t.Fatalf("invalid operation floor: %v", err)
	}
	if _, err := catalog.NewOperation(capability.RiskRead); !errors.Is(err, capability.ErrEmptyRequirement) {
		t.Fatalf("empty operation: %v", err)
	}
	if _, err := capability.NewPolicy(capability.Set{}, capability.RiskHigh); !errors.Is(err, capability.ErrInvalidCatalog) {
		t.Fatalf("invalid grants: %v", err)
	}
	if _, err := capability.NewPolicy(mustSet(t, catalog), 0); !errors.Is(err, capability.ErrInvalidRisk) {
		t.Fatalf("invalid risk ceiling: %v", err)
	}
	definitions := capability.BaseDefinitions()
	conflict := definitions[0]
	conflict.MinimumRisk = capability.RiskHigh
	if _, err := capability.NewCatalog(append(definitions, conflict)...); !errors.Is(err, capability.ErrConflictingDefinition) {
		t.Fatalf("conflicting risk definition: %v", err)
	}
}
