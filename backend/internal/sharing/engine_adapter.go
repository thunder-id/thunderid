// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing/policyengine"
)

// The engine deals in its own value types so that it depends on nothing, which means the service
// converts at the boundary. The two shapes are deliberately kept in step.

// toEngineRule converts a stored rule into the engine's form.
func toEngineRule(r OverlayRule) policyengine.Rule {
	return policyengine.Rule{
		Editable:       r.Editable,
		Value:          copyStrings(r.Value),
		AllowedValues:  copyStrings(r.AllowedValues),
		ExcludedValues: copyStrings(r.ExcludedValues),
	}
}

// fromEngineRule converts an engine rule back into the stored form.
func fromEngineRule(r policyengine.Rule) OverlayRule {
	return OverlayRule{
		Editable:       r.Editable,
		Value:          copyStrings(r.Value),
		AllowedValues:  copyStrings(r.AllowedValues),
		ExcludedValues: copyStrings(r.ExcludedValues),
	}
}

// toEnginePolicy converts a policy into the subset chain evaluation needs.
func toEnginePolicy(p Policy) policyengine.Policy {
	targets := make([]policyengine.Target, 0, len(p.Targets))
	for _, t := range p.Targets {
		targets = append(targets, policyengine.Target{
			ID: t.ID, Scope: policyengine.TargetScope(t.Scope), OUID: t.OUID,
		})
	}
	return policyengine.Policy{
		ID:             p.ID,
		OwningOUID:     p.OwningOUID,
		InitiatingOUID: p.InitiatingOUID,
		Stage:          policyengine.PolicyStage(p.Stage),
		Targets:        targets,
		ExcludedOUIDs:  p.ExcludedOUIDs,
		Declared:       p.Declared,
	}
}

// toEnginePolicies converts a set of policies.
func toEnginePolicies(in []Policy) []policyengine.Policy {
	out := make([]policyengine.Policy, 0, len(in))
	for _, p := range in {
		out = append(out, toEnginePolicy(p))
	}
	return out
}

// requestFromPolicy rebuilds the request that recreates a policy, for export and replay. The
// resolved rules are exported rather than the requested ones, so replaying reaches the same
// result and the export is a fixed point.
func requestFromPolicy(p Policy) PolicyRequest {
	scope := TargetOUScope{ExcludedOUIDs: p.ExcludedOUIDs}
	for _, t := range p.Targets {
		switch t.Scope {
		case TargetScopeAllOUs:
			scope.AllOUs = true
		case TargetScopeAllRoots:
			scope.AllRoots = true
		case TargetScopeRoot:
			scope.RootOUIDs = append(scope.RootOUIDs, t.OUID)
		case TargetScopeAllChildren:
			scope.AllChildren = true
		case TargetScopeOU, TargetScopeOUSubtree:
			scope.ChildOUIDs = append(scope.ChildOUIDs, TargetEntry{
				OUID:         t.OUID,
				AllChildren:  t.Scope == TargetScopeOUSubtree,
				OverlayRules: p.TargetRules(t.ID),
			})
		}
	}

	return PolicyRequest{
		InitiatingOUID: p.InitiatingOUID,
		TargetOUScope:  scope,
		OverlayRules:   p.PolicyLevelRules(),
		Version:        p.Version,
	}
}

// mapEngineError turns a narrowing failure into the service error naming the offending field.
func mapEngineError(err error, fieldKey string) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, policyengine.ErrWidens):
		return withDetail(ErrorRuleWidens, fieldKey)
	case errors.Is(err, policyengine.ErrValueOutsideAllowed):
		return withDetail(ErrorValueOutsideAllowed, fieldKey)
	case errors.Is(err, policyengine.ErrPinnedWithAllowed):
		return withDetail(ErrorPinnedWithAllowed, fieldKey)
	default:
		return &tidcommon.InternalServerError
	}
}

// mapEditError turns an edit-validation failure into its service error.
func mapEditError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, policyengine.ErrBlanketScopeNarrowOnly),
		errors.Is(err, policyengine.ErrScopeFamilyChange):
		return &ErrorBlanketNarrowOnly
	case errors.Is(err, policyengine.ErrDeclaredImmutable):
		return &ErrorPolicyDeclared
	case errors.Is(err, policyengine.ErrVersionMismatch):
		return &ErrorVersionMismatch
	default:
		return &tidcommon.InternalServerError
	}
}
