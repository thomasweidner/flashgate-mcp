package config

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

type policyID string

// policyFields is the shared schema for named policies. Each resource type
// resolves the fields it supports before publishing its effective settings.
type policyFields struct {
	resourceFields
	fileTypeFields
	Access         accessFields `toml:"access"`
	Excluded       []string     `toml:"excluded"`
	FollowSymlinks *bool        `toml:"follow_symlinks"`
}

// A nil list means absent. A present empty list explicitly grants no access.
type accessFields []capability.ID

func (a accessFields) resolve(catalog capability.Catalog) (capability.Set, error) {
	if a == nil {
		return catalog.NewSet(capability.FilesRead)
	}
	for _, id := range a {
		if !capability.AppliesToFilesystemRoot(id) {
			return capability.Set{}, errors.New("capability does not apply to filesystem roots")
		}
	}
	return catalog.NewSet(a...)
}

func (s settings) resolveRootDefinitions(catalog capability.Catalog) ([]RootDefinition, error) {
	// Validate every definition, including ones that no resource references.
	policies := make(map[policyID]capability.Set, len(s.Policy))
	exclusions := make(map[policyID]PathExclusions, len(s.Policy))
	for id, policy := range s.Policy {
		if !validConfigID(string(id)) {
			return nil, NewError(CategoryInvalidConfig, errors.New("invalid policy name"))
		}
		access, err := policy.Access.resolve(catalog)
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		policies[id] = access
		excluded, err := compilePathExclusions(policy.Excluded)
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		exclusions[id] = excluded
		if _, err := policy.fileTypeFields.resolveFileTypes(fileTypeFields{}); err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		if _, err := policy.resourceFields.resolve(s.Files.limits(), s.Files.limits()); err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
	}

	roots := make([]RootDefinition, 0, len(s.Files.Roots))
	for id, root := range s.Files.Roots {
		access, err := root.resolveAccess(catalog, policies)
		if err != nil {
			return nil, NewError(CategoryInvalidConfig, err)
		}
		var excluded PathExclusions
		if root.Excluded != nil {
			excluded, err = compilePathExclusions(root.Excluded)
			if err != nil {
				return nil, NewError(CategoryInvalidConfig, err)
			}
		} else if root.Policy != nil {
			excluded = exclusions[*root.Policy]
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
		follow := root.FollowSymlinks
		if follow == nil && root.Policy != nil {
			follow = s.Policy[*root.Policy].FollowSymlinks
		}
		roots = append(roots, RootDefinition{ID: id, Path: filepath.Clean(root.Path), Access: access, Limits: limits, FileTypes: types,
			Excluded: excluded, FollowSymlinks: follow != nil && *follow})
	}
	slices.SortFunc(roots, func(a, b RootDefinition) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
	return roots, nil
}

func (r rootFields) resolveAccess(catalog capability.Catalog, policies map[policyID]capability.Set) (capability.Set, error) {
	if r.Policy == nil {
		return r.Access.resolve(catalog)
	}
	inherited, ok := policies[*r.Policy]
	if !ok {
		return capability.Set{}, errors.New("unknown root policy")
	}
	if r.Access == nil {
		return inherited, nil
	}
	return r.Access.resolve(catalog)
}
