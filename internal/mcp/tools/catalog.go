package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type CatalogHandler func(handlers.Context, execution.CatalogInvocation, json.RawMessage) (ToolResult, error)

// CatalogSupport is an exact combination confirmed at trusted composition.
// A representation names the operations its handler can execute in that scope.
type CatalogSupport struct {
	Revision       execution.Protocol
	Backend        execution.BackendID
	Origin         execution.Origin
	Platform       string
	Representation execution.RepresentationID
}

type CatalogRepresentation struct {
	Support  CatalogSupport
	Metadata Tool
	Variants []execution.OperationKey
	Outcomes []CatalogOutcome
}

// CatalogOutcome binds a declared operation to a result it can produce.
type CatalogOutcome struct {
	Operation execution.OperationKey
	Result    string
}

// CatalogTool is a canonical registration reference. Consumers can reuse the
// same reference when combining components. A different reference with the
// same public name is a conflict.
type CatalogTool struct {
	Route           execution.CatalogRouteID
	Representations []CatalogRepresentation
	Handle          CatalogHandler
}

type catalogRegistration struct {
	route           execution.CatalogRouteID
	representations map[CatalogSupport]registeredTool
}

type masterCatalog struct {
	bind     func(handlers.Context) (execution.CatalogContext, error)
	validate func(*CatalogView) error
	tools    []catalogRegistration
}

// CatalogView is an immutable projection supplied to the activation validator.
// The registry publishes its composition only after its trusted validator passes.
type CatalogView struct {
	composition execution.CatalogComposition
	entries     map[string]registeredTool
	order       []string
}

func NewCatalog(bind func(handlers.Context) (execution.CatalogContext, error), validate func(*CatalogView) error, definitions ...*CatalogTool) (*Registry, error) {
	if bind == nil || validate == nil || len(definitions) == 0 {
		return nil, execution.ErrMismatch
	}
	master := &masterCatalog{bind: bind, validate: validate}
	seen := make(map[*CatalogTool]bool, len(definitions))
	names := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		if seen[definition] {
			continue
		}
		name, representations, err := compileRegistrations(definition)
		if err != nil {
			return nil, err
		}
		if names[name] {
			return nil, fmt.Errorf("conflicting tool registration %q: %w", name, execution.ErrMismatch)
		}
		names[name] = true
		seen[definition] = true
		master.tools = append(master.tools, catalogRegistration{definition.Route, representations})
	}
	return &Registry{master: master, sealed: true}, nil
}

func compileRegistrations(definition *CatalogTool) (string, map[CatalogSupport]registeredTool, error) {
	if definition == nil || definition.Handle == nil || len(definition.Representations) == 0 {
		return "", nil, execution.ErrMismatch
	}
	var name string
	representations := make(map[CatalogSupport]registeredTool, len(definition.Representations))
	for _, representation := range definition.Representations {
		support := representation.Support
		if _, duplicate := representations[support]; duplicate || support.Revision == "" ||
			support.Backend == "" || support.Origin == "" || support.Platform == "" ||
			missingTool(representation.Metadata) || len(representation.Variants) == 0 {
			return "", nil, execution.ErrMismatch
		}
		tool, err := snapshotCatalogMetadata(representation.Metadata)
		if err != nil {
			return "", nil, fmt.Errorf("register %s: %w", representation.Metadata.Name(), err)
		}
		if name != "" && name != tool.name {
			return "", nil, execution.ErrMismatch
		}
		name = tool.name
		keys := make(map[execution.OperationKey]bool, len(representation.Variants))
		for _, key := range representation.Variants {
			if key.Name == "" || key.Variant == "" || keys[key] {
				return "", nil, execution.ErrMismatch
			}
			keys[key] = true
		}
		outcomes, err := compileCatalogOutcomes(keys, tool.contract, representation.Outcomes)
		if err != nil {
			return "", nil, err
		}
		representations[support] = registeredTool{
			tool: tool, contract: tool.contract, bound: definition.Handle,
			variants: slices.Clone(representation.Variants), outcomes: outcomes,
		}
	}
	return name, representations, nil
}

func compileCatalogOutcomes(keys map[execution.OperationKey]bool, contract ResultContract, outcomes []CatalogOutcome) ([]CatalogOutcome, error) {
	if len(outcomes) == 0 {
		return nil, execution.ErrMismatch
	}
	seen := make(map[CatalogOutcome]bool, len(outcomes))
	operations := make(map[execution.OperationKey]bool)
	results := make(map[string]bool)
	for _, outcome := range outcomes {
		if !keys[outcome.Operation] || seen[outcome] ||
			!slices.ContainsFunc(contract, func(result ResultVariant) bool { return result.Name == outcome.Result }) {
			return nil, execution.ErrMismatch
		}
		seen[outcome], operations[outcome.Operation], results[outcome.Result] = true, true, true
	}
	if len(operations) != len(keys) || len(results) != len(contract) {
		return nil, execution.ErrMismatch
	}
	return slices.Clone(outcomes), nil
}

func missingTool(tool Tool) bool {
	if tool == nil {
		return true
	}
	value := reflect.ValueOf(tool)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func (r *Registry) IsCatalog() bool { return r.master != nil }

func (r *Registry) PrepareView(ctx handlers.Context) (*CatalogView, error) {
	scope, err := r.bindCatalogContext(ctx)
	if err != nil {
		return nil, err
	}
	composition := scope.Composition()
	view := &CatalogView{composition: composition, entries: make(map[string]registeredTool)}
	for _, registration := range r.master.tools {
		route, found := composition.Route(registration.route)
		if !found {
			continue
		}
		support := CatalogSupport{
			Revision: composition.Protocol(), Backend: route.Backend(), Origin: composition.Origin(),
			Platform: composition.Platform(), Representation: route.Representation(),
		}
		entry, found := registration.representations[support]
		if !found {
			continue
		}
		keys := make([]execution.OperationKey, 0, len(entry.variants))
		for _, key := range entry.variants {
			if !route.Declared(key) {
				return nil, execution.ErrMismatch
			}
			if route.Eligible(key) {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			continue
		}
		entry.variants = keys
		entry.route = registration.route
		entry.outcomes = slices.DeleteFunc(slices.Clone(entry.outcomes), func(outcome CatalogOutcome) bool {
			return !slices.Contains(keys, outcome.Operation)
		})
		name := entry.tool.Name()
		view.order = append(view.order, name)
		view.entries[name] = entry
	}
	return view, nil
}

// Activate validates all compositions as a unit. Fresh trusted requests can
// consume a validated composition regardless of which principal validated it.
func (r *Registry) Activate(contexts ...handlers.Context) error {
	if r.master == nil || len(contexts) == 0 {
		return execution.ErrMismatch
	}
	admitted := make(map[execution.CatalogComposition]*CatalogView, len(contexts))
	var first *CatalogView
	for _, ctx := range contexts {
		view, err := r.PrepareView(ctx)
		if err != nil {
			return err
		}
		if admitted[view.composition] != nil {
			return execution.ErrMismatch
		}
		if err := r.master.validate(view); err != nil {
			return err
		}
		admitted[view.composition] = view
		if first == nil {
			first = view
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.admitted != nil {
		return execution.ErrMismatch
	}
	r.admitted, r.firstView = admitted, first
	return nil
}

func (r *Registry) bindCatalogContext(ctx handlers.Context) (execution.CatalogContext, error) {
	if r.master == nil || ctx.Context == nil || ctx.Context.Err() != nil {
		return execution.CatalogContext{}, execution.ErrMismatch
	}
	scope, err := r.master.bind(ctx)
	if err != nil || !scope.Valid() || string(scope.Composition().Protocol()) != ctx.ProtocolVersion {
		return execution.CatalogContext{}, execution.ErrMismatch
	}
	return scope, nil
}

func (r *Registry) activeView(ctx handlers.Context) (*CatalogView, execution.CatalogContext, error) {
	scope, err := r.bindCatalogContext(ctx)
	if err != nil {
		return nil, execution.CatalogContext{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if view := r.admitted[scope.Composition()]; view != nil {
		return view, scope, nil
	}
	return nil, execution.CatalogContext{}, execution.ErrMismatch
}

func (v *CatalogView) Composition() execution.CatalogComposition { return v.composition }

func (v *CatalogView) List() []Tool {
	result := make([]Tool, 0, len(v.order))
	for _, name := range v.order {
		result = append(result, v.entries[name].tool)
	}
	return result
}

func (v *CatalogView) wireDefinitions() []json.RawMessage {
	result := make([]json.RawMessage, 0, len(v.order))
	for _, name := range v.order {
		metadata := v.entries[name].tool.(catalogMetadata)
		result = append(result, json.RawMessage(metadata.definition))
	}
	return result
}

func (v *CatalogView) ResultContract(name string) ResultContract {
	return copyResultContract(v.entries[name].contract)
}

func (v *CatalogView) OperationKeys(name string) []execution.OperationKey {
	return slices.Clone(v.entries[name].variants)
}

func (v *CatalogView) Outcomes(name string) []CatalogOutcome {
	return slices.Clone(v.entries[name].outcomes)
}

type catalogMetadata struct {
	name        string
	description string
	definition  string
	contract    ResultContract
}

func snapshotCatalogMetadata(source Tool) (catalogMetadata, error) {
	definition := source.Definition()
	if definition.Name == "" || definition.Name != source.Name() ||
		strings.TrimSpace(definition.Name) != definition.Name || definition.InputSchema == nil {
		return catalogMetadata{}, execution.ErrMismatch
	}
	contract, err := source.ResultContract().validatedCopy()
	if err != nil {
		return catalogMetadata{}, err
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return catalogMetadata{}, err
	}
	return catalogMetadata{definition.Name, definition.Description, string(encoded), contract}, nil
}

func (m catalogMetadata) Name() string        { return m.name }
func (m catalogMetadata) Description() string { return m.description }
func (m catalogMetadata) InputSchema() any    { return m.Definition().InputSchema }

func (m catalogMetadata) Definition() protocol.Tool {
	var definition protocol.Tool
	// The bytes were encoded from this exact type during trusted composition.
	// Preserve exact numbers in schema constants and bounds.
	decoder := json.NewDecoder(strings.NewReader(m.definition))
	decoder.UseNumber()
	_ = decoder.Decode(&definition)
	return definition
}

func (m catalogMetadata) ResultContract() ResultContract { return copyResultContract(m.contract) }

func (catalogMetadata) Execute(context.Context, json.RawMessage) (ToolResult, error) {
	return ToolResult{}, execution.ErrMismatch
}
