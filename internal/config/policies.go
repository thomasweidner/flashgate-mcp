package config

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

type policyID string

// policyFields is the shared schema for named policies. Each resource type
// resolves the fields it supports before publishing its effective settings.
type policyFields struct {
	resourceFields
	fileTypeFields
	Access accessFields `toml:"access"`
}

// A nil list means absent. A present empty list explicitly grants no access.
type accessFields []string

// RootAccess is the effective set of filesystem rights for one root ID.
// Its zero value denies all access.
type RootAccess uint8

const (
	RootRead RootAccess = 1 << iota
	RootWrite
)

func (a RootAccess) Allows(required RootAccess) bool {
	return required != 0 && a&required == required
}

func (a accessFields) resolve() (RootAccess, error) {
	if a == nil {
		return RootRead, nil
	}
	var access RootAccess
	for _, value := range a {
		var right RootAccess
		switch value {
		case "read":
			right = RootRead
		case "write":
			right = RootWrite
		default:
			return 0, errors.New("invalid filesystem right")
		}
		if access&right != 0 {
			return 0, errors.New("duplicate filesystem right")
		}
		access |= right
	}
	return access, nil
}

func (s settings) resolveRootDefinitions() ([]RootDefinition, error) {
	// Validate every definition, including ones that no resource references.
	policies := make(map[policyID]RootAccess, len(s.Policy))
	for id, policy := range s.Policy {
		if !validConfigID(string(id)) {
			return nil, NewError(CategoryInvalidConfig, errors.New("invalid policy name"))
		}
		access, err := policy.Access.resolve()
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		policies[id] = access
		if _, err := policy.fileTypeFields.resolveFileTypes(fileTypeFields{}); err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		if _, err := policy.resourceFields.resolve(s.Files.limits(), s.Files.limits()); err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
	}

	roots := make([]RootDefinition, 0, len(s.Files.Roots))
	for id, root := range s.Files.Roots {
		access, err := root.resolveAccess(policies)
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		limits := s.Files.limits()
		if root.Policy != nil {
			limits, err = s.Policy[*root.Policy].resourceFields.resolve(limits, s.Files.limits())
			if err != nil {
				return nil, NewError(CategoryInvalidConfig, err)
			}
		}
		limits, err = root.resourceFields.resolve(limits, s.Files.limits())
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		var inherited fileTypeFields
		if root.Policy != nil {
			inherited = s.Policy[*root.Policy].fileTypeFields
		}
		types, err := root.fileTypeFields.resolveFileTypes(inherited)
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		roots = append(roots, RootDefinition{ID: id, Path: filepath.Clean(root.Path), Access: access, Limits: limits, FileTypes: types})
	}
	slices.SortFunc(roots, func(a, b RootDefinition) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
	return roots, nil
}

func (r rootFields) resolveAccess(policies map[policyID]RootAccess) (RootAccess, error) {
	if r.Policy == nil {
		return r.Access.resolve()
	}
	inherited, ok := policies[*r.Policy]
	if !ok {
		return 0, errors.New("unknown root policy")
	}
	if r.Access == nil {
		return inherited, nil
	}
	return r.Access.resolve()
}
