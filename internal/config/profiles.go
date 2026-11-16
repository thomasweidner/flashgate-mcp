package config

import (
	"errors"
	"maps"
	"slices"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

type ProfileID string

const (
	ProfileRead  ProfileID = "read"
	ProfileWrite ProfileID = "write"
)

type profileFields struct {
	Active      *ProfileID                            `toml:"active"`
	Include     []ProfileID                           `toml:"include"`
	MaxRisk     string                                `toml:"max_risk"`
	Definitions map[ProfileID]profileDefinitionFields `toml:"definitions"`
}

type profileDefinitionFields struct {
	Grants []capability.ID `toml:"grants"`
}

// EffectiveProfile is the validated, immutable policy of one startup snapshot.
type EffectiveProfile struct {
	id      ProfileID
	catalog capability.Catalog
	policy  capability.Policy
}

func (p EffectiveProfile) ID() ProfileID               { return p.id }
func (p EffectiveProfile) Catalog() capability.Catalog { return p.catalog }
func (p EffectiveProfile) Policy() capability.Policy   { return p.policy }

func (p profileFields) resolve() (EffectiveProfile, error) {
	catalog, err := capability.NewCatalog(capability.BaseDefinitions()...)
	if err != nil {
		return EffectiveProfile{}, err
	}
	definitions := make(map[ProfileID]capability.Set, len(p.Definitions)+2)
	for _, id := range []ProfileID{ProfileRead, ProfileWrite} {
		rights := []capability.ID{capability.FilesRead}
		if id == ProfileWrite {
			rights = append(rights, capability.FilesWrite)
		}
		definitions[id], err = catalog.NewSet(rights...)
		if err != nil {
			return EffectiveProfile{}, err
		}
	}
	for _, id := range slices.Sorted(maps.Keys(p.Definitions)) {
		if !validConfigID(string(id)) || id == ProfileRead || id == ProfileWrite {
			return EffectiveProfile{}, errors.New("invalid or reserved profile ID")
		}
		grants := p.Definitions[id].Grants
		if grants == nil {
			return EffectiveProfile{}, errors.New("profile grants are required")
		}
		definitions[id], err = catalog.NewSet(grants...)
		if err != nil {
			return EffectiveProfile{}, err
		}
	}

	active := ProfileRead
	if p.Active != nil {
		active = *p.Active
	} else if len(p.Include) != 0 {
		return EffectiveProfile{}, errors.New("profile composition requires an explicit base")
	}
	grants, exists := definitions[active]
	if !exists {
		return EffectiveProfile{}, errors.New("unknown active profile")
	}
	selected := map[ProfileID]bool{active: true}
	for _, id := range p.Include {
		component, exists := definitions[id]
		if !exists || selected[id] {
			return EffectiveProfile{}, errors.New("unknown or duplicate profile component")
		}
		selected[id] = true
		grants, err = grants.Union(component)
		if err != nil {
			return EffectiveProfile{}, err
		}
	}
	var ceiling capability.Risk
	switch p.MaxRisk {
	case "read":
		ceiling = capability.RiskRead
	case "mutation":
		ceiling = capability.RiskMutation
	case "high":
		ceiling = capability.RiskHigh
	default:
		return EffectiveProfile{}, capability.ErrInvalidRisk
	}
	policy, err := capability.NewPolicy(grants, ceiling)
	if err != nil {
		return EffectiveProfile{}, err
	}
	return EffectiveProfile{id: active, catalog: catalog, policy: policy}, nil
}
