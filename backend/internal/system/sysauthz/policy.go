/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package sysauthz

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// policyDecision is the outcome of a single policy evaluation.
type policyDecision int

const (
	// policyDecisionNotApplicable indicates the policy has no opinion on this
	// context (e.g., the action is not OU-scoped). The next policy in the chain
	// will be consulted. If all policies return NotApplicable, the action is allowed.
	policyDecisionNotApplicable policyDecision = iota
	// policyDecisionAllowed indicates the policy explicitly permits the action.
	policyDecisionAllowed
	// policyDecisionDenied indicates the policy explicitly denies the action.
	policyDecisionDenied
)

// authorizationPolicy defines an authorization rule evaluated after permission checks pass.
// It is the primary extension point for introducing fine-grained access control
// (e.g., attribute-based or relationship-based policies) without changing the
// SystemAuthorizationServiceInterface contract.
//
// A policy mirrors the two methods of SystemAuthorizationServiceInterface at the
// policy layer:
//   - isActionAllowed: called by IsActionAllowed for single-resource operations.
//   - getAccessibleResources: called by GetAccessibleResources for list operations.
type authorizationPolicy interface {
	// isActionAllowed returns the policy decision for the caller in the given context.
	// A non-nil ServiceError signals a policy evaluation failure, not a denial.
	isActionAllowed(ctx context.Context, actionCtx *ActionContext) (policyDecision, *serviceerror.ServiceError)

	// getAccessibleResources reports whether this policy is applicable for the
	// given action and resource type, and if so, the set of resources the caller
	// may access. When applicable is false the policy has no opinion for this
	// resource type and the result must be ignored by the caller.
	// A non-nil ServiceError signals an evaluation failure, not a denial.
	getAccessibleResources(ctx context.Context, action security.Action,
		resourceType security.ResourceType,
	) (applicable bool, result *AccessibleResources, err *serviceerror.ServiceError)
}

// ouMembershipPolicy enforces that the caller's organization unit matches the OU of the
// resource being acted upon. This prevents non-system callers from operating on
// resources that belong to a different OU.
type ouMembershipPolicy struct{}

// isActionAllowed returns:
//   - PolicyDecisionNotApplicable when the action context carries no OUID.
//   - PolicyDecisionAllowed when the caller's OU matches the resource's OU.
//   - PolicyDecisionDenied when the caller's OU does not match.
func (p *ouMembershipPolicy) isActionAllowed(ctx context.Context,
	actionCtx *ActionContext) (policyDecision, *serviceerror.ServiceError) {
	if actionCtx == nil || actionCtx.OUID == "" {
		return policyDecisionNotApplicable, nil
	}
	if security.GetOUID(ctx) == actionCtx.OUID {
		return policyDecisionAllowed, nil
	}
	return policyDecisionDenied, nil
}

// getAccessibleResources constrains list operations by the caller's OU membership:
//   - For non-ResourceTypeOU resource types: not applicable — OU-based filtering
//     for users and groups is applied at the store layer.
//   - For ResourceTypeOU: the caller may only see their own OU.
func (p *ouMembershipPolicy) getAccessibleResources(ctx context.Context, action security.Action,
	resourceType security.ResourceType) (bool, *AccessibleResources, *serviceerror.ServiceError) {
	if resourceType != security.ResourceTypeOU {
		return false, nil, nil
	}
	ouID := security.GetOUID(ctx)
	if ouID == "" {
		return true, &AccessibleResources{AllAllowed: false, IDs: []string{}}, nil
	}
	return true, &AccessibleResources{AllAllowed: false, IDs: []string{ouID}}, nil
}

// ouInheritancePolicy grants read-only access to resources whose OU is an ancestor of
// (or the same as) the caller's OU. This enables child OUs to see resources defined in
// parent OUs without being able to modify them.
type ouInheritancePolicy struct {
	resolver OUHierarchyResolver
}

// isActionAllowed returns:
//   - PolicyDecisionNotApplicable when the action context carries no OUID.
//   - PolicyDecisionAllowed when the resource's OU is the same as or an ancestor of the
//     caller's OU (i.e. the resource was defined at or above the caller's level).
//   - PolicyDecisionDenied when the caller is outside the resource's OU subtree.
func (p *ouInheritancePolicy) isActionAllowed(ctx context.Context,
	actionCtx *ActionContext) (policyDecision, *serviceerror.ServiceError) {
	if actionCtx == nil || actionCtx.OUID == "" {
		return policyDecisionNotApplicable, nil
	}
	callerOUID := security.GetOUID(ctx)
	if callerOUID == "" {
		return policyDecisionDenied, nil
	}
	if callerOUID == actionCtx.OUID {
		return policyDecisionAllowed, nil
	}
	// Allow if the resource's OU is an ancestor of the caller's OU.
	// i.e. the caller belongs to one of its descendants.
	isAncestor, svcErr := p.resolver.IsAncestor(ctx, actionCtx.OUID, callerOUID)
	if svcErr != nil {
		return policyDecisionDenied, svcErr
	}
	if isAncestor {
		return policyDecisionAllowed, nil
	}
	return policyDecisionDenied, nil
}

// getAccessibleResources returns the caller's own OU plus all ancestor OUs, so that
// list queries include inherited resources from parent OUs.
func (p *ouInheritancePolicy) getAccessibleResources(ctx context.Context, action security.Action,
	resourceType security.ResourceType) (bool, *AccessibleResources, *serviceerror.ServiceError) {
	if !inheritanceReadActions[action] {
		return false, nil, nil
	}
	callerOUID := security.GetOUID(ctx)
	if callerOUID == "" {
		return true, &AccessibleResources{AllAllowed: false, IDs: []string{}}, nil
	}
	ancestorIDs, svcErr := p.resolver.GetAncestorOUIDs(ctx, callerOUID)
	if svcErr != nil {
		return true, nil, svcErr
	}

	resultIDs := []string{callerOUID}
	resultIDs = append(resultIDs, ancestorIDs...)

	return true, &AccessibleResources{AllAllowed: false, IDs: resultIDs}, nil
}

// inheritanceReadActions is the set of read-only actions that use OU-inheritance semantics.
// An action listed here gives callers in child OUs visibility into resources defined in
// parent OUs. Write actions must NOT be listed here — child OUs must never be able to
// modify resources owned by a parent OU.
// Each action implicitly encodes its resource type (e.g. ActionReadUserType → EntityType),
// so a separate resource-type map is not needed.
var inheritanceReadActions = map[security.Action]bool{
	security.ActionReadUserType:  true,
	security.ActionListUserTypes: true,
}

// isInheritanceEligible returns true when the action is registered for
// inheritance-based policy evaluation.
func isInheritanceEligible(action security.Action) bool {
	return inheritanceReadActions[action]
}

// selectPolicies returns the effective policy chain for the given action.
// When a pre-built inheritancePolicy is available and the action is eligible,
// that policy is used instead of the default globalPolicies.
func selectPolicies(action security.Action, policies *policies) []authorizationPolicy {
	if policies.inheritancePolicy != nil && isInheritanceEligible(action) {
		return []authorizationPolicy{policies.inheritancePolicy}
	}
	return []authorizationPolicy{policies.membershipPolicy}
}

// isActionAllowedByPolicies runs the effective policy chain for the given action against
// the action context in order.
// - PolicyDecisionDenied from any policy stops the chain and denies the action.
// - PolicyDecisionNotApplicable skips to the next policy.
// - PolicyDecisionAllowed continues to the next policy.
// If all policies return NotApplicable, the action is allowed (permission check already passed).
func isActionAllowedByPolicies(ctx context.Context, policies *policies, action security.Action,
	actionCtx *ActionContext) (bool, *serviceerror.ServiceError) {
	for _, policy := range selectPolicies(action, policies) {
		decision, err := policy.isActionAllowed(ctx, actionCtx)
		if err != nil {
			return false, err
		}
		if decision == policyDecisionDenied {
			return false, nil
		}
	}
	return true, nil
}

// getAccessibleResourcesByPolicies iterates the effective policy chain to compute the
// accessible resource set for list operations. The result of the first applicable policy
// is returned immediately (first-applicable-wins).
//
// NOTE: If multiple policies ever need to be combined for the same resource type in the
// future, this function should be updated to intersect their results.
func getAccessibleResourcesByPolicies(ctx context.Context, policies *policies, action security.Action,
	resourceType security.ResourceType) (*AccessibleResources, *serviceerror.ServiceError) {
	for _, policy := range selectPolicies(action, policies) {
		applicable, result, err := policy.getAccessibleResources(ctx, action, resourceType)
		if err != nil {
			return nil, err
		}
		if applicable {
			return result, nil
		}
	}
	return &AccessibleResources{AllAllowed: true}, nil
}
