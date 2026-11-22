package capability

import "errors"

// Risk is the minimum risk classification for a definition or operation.
type Risk uint8

const (
	RiskRead Risk = iota + 1
	RiskMutation
	RiskHigh
)

func (r Risk) valid() bool { return r >= RiskRead && r <= RiskHigh }

var ErrInvalidRisk = errors.New("invalid risk class")

// Operation binds all required rights and the derived risk to one catalog.
type Operation struct {
	requirement Requirement
	risk        Risk
}

// NewOperation resolves risk as the highest capability floor or operation
// floor. The floor is supplied by the reviewed operation declaration.
func (c Catalog) NewOperation(floor Risk, ids ...ID) (Operation, error) {
	if !floor.valid() {
		return Operation{}, ErrInvalidRisk
	}
	requirement, err := c.NewRequirement(ids...)
	if err != nil {
		return Operation{}, err
	}
	for _, id := range requirement.required.ids {
		definition, _ := c.Lookup(id)
		floor = max(floor, definition.MinimumRisk)
	}
	return Operation{requirement: requirement, risk: floor}, nil
}

func (o Operation) Risk() Risk { return o.risk }

func (o Operation) Requirement() Requirement { return o.requirement }

// Policy is the immutable functional and risk input for an effective profile.
// Passing it establishes those inputs only. Resource and execution policies
// are applied separately by the relevant domain and host.
type Policy struct {
	grants  Set
	ceiling Risk
}

func NewPolicy(grants Set, ceiling Risk) (Policy, error) {
	if grants.snapshot == nil {
		return Policy{}, ErrInvalidCatalog
	}
	if !ceiling.valid() {
		return Policy{}, ErrInvalidRisk
	}
	return Policy{grants: grants, ceiling: ceiling}, nil
}

func (p Policy) Grants() Set   { return p.grants }
func (p Policy) Ceiling() Risk { return p.ceiling }

func (p Policy) Allows(operation Operation) bool {
	return operation.risk.valid() && operation.risk <= p.ceiling && operation.requirement.SatisfiedBy(p.grants)
}
