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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// stubPolicy is a configurable authorizationPolicy for testing. It allows independent
// control of isActionAllowed and getAccessibleResources behavior.
type stubPolicy struct {
	// isActionAllowed response fields.
	decision  policyDecision
	actionErr *serviceerror.ServiceError

	// getAccessibleResources response fields.
	applicable  bool
	result      *AccessibleResources
	resourceErr *serviceerror.ServiceError
}

func (p *stubPolicy) isActionAllowed(_ context.Context,
	_ *ActionContext) (policyDecision, *serviceerror.ServiceError) {
	return p.decision, p.actionErr
}

func (p *stubPolicy) getAccessibleResources(_ context.Context, _ security.Action,
	_ security.ResourceType) (bool, *AccessibleResources, *serviceerror.ServiceError) {
	return p.applicable, p.result, p.resourceErr
}

// stubOUHierarchyResolver is a configurable OUHierarchyResolver for testing.
type stubOUHierarchyResolver struct {
	// IsAncestor response fields.
	isAncestorResult bool
	isAncestorErr    *serviceerror.ServiceError

	// GetAncestorOUIDs response fields.
	ancestorIDs    []string
	ancestorIDsErr *serviceerror.ServiceError
}

func (r *stubOUHierarchyResolver) IsAncestor(
	_ context.Context, _, _ string,
) (bool, *serviceerror.ServiceError) {
	return r.isAncestorResult, r.isAncestorErr
}

func (r *stubOUHierarchyResolver) GetAncestorOUIDs(
	_ context.Context, _ string,
) ([]string, *serviceerror.ServiceError) {
	return r.ancestorIDs, r.ancestorIDsErr
}

// ---------------------------------------------------------------------------
// ouMembershipPolicy.isActionAllowed
// ---------------------------------------------------------------------------

func TestOuMembershipPolicy_IsActionAllowed(t *testing.T) {
	policy := &ouMembershipPolicy{}

	tests := []struct {
		name         string
		ctx          context.Context
		actionCtx    *ActionContext
		wantDecision policyDecision
	}{
		{
			name:         "NilActionCtx_NotApplicable",
			ctx:          context.Background(),
			actionCtx:    nil,
			wantDecision: policyDecisionNotApplicable,
		},
		{
			name:         "EmptyOUID_NotApplicable",
			ctx:          context.Background(),
			actionCtx:    &ActionContext{OUID: ""},
			wantDecision: policyDecisionNotApplicable,
		},
		{
			name:         "MatchingOU_Allowed",
			ctx:          buildCtxWithOU("", "ou1"),
			actionCtx:    &ActionContext{OUID: "ou1"},
			wantDecision: policyDecisionAllowed,
		},
		{
			name:         "MismatchedOU_Denied",
			ctx:          buildCtxWithOU("", "ou2"),
			actionCtx:    &ActionContext{OUID: "ou1"},
			wantDecision: policyDecisionDenied,
		},
		{
			name:         "NoOuInContext_Denied",
			ctx:          context.Background(),
			actionCtx:    &ActionContext{OUID: "ou1"},
			wantDecision: policyDecisionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := policy.isActionAllowed(tt.ctx, tt.actionCtx)
			assert.Nil(t, err)
			assert.Equal(t, tt.wantDecision, decision)
		})
	}
}

// ---------------------------------------------------------------------------
// ouMembershipPolicy.getAccessibleResources
// ---------------------------------------------------------------------------

func TestOuMembershipPolicy_GetAccessibleResources(t *testing.T) {
	policy := &ouMembershipPolicy{}

	tests := []struct {
		name           string
		ctx            context.Context
		resourceType   security.ResourceType
		wantApplicable bool
		wantAllAllowed bool
		wantIDs        []string
	}{
		{
			name:           "UserResource_NotApplicable",
			ctx:            context.Background(),
			resourceType:   security.ResourceTypeUser,
			wantApplicable: false,
		},
		{
			name:           "GroupResource_NotApplicable",
			ctx:            context.Background(),
			resourceType:   security.ResourceTypeGroup,
			wantApplicable: false,
		},
		{
			name:           "OUResource_EmptyOUIDInContext_RestrictedEmpty",
			ctx:            context.Background(),
			resourceType:   security.ResourceTypeOU,
			wantApplicable: true,
			wantAllAllowed: false,
			wantIDs:        []string{},
		},
		{
			name:           "OUResource_NonEmptyOUID_RestrictedToOU",
			ctx:            buildCtxWithOU("", "ou1"),
			resourceType:   security.ResourceTypeOU,
			wantApplicable: true,
			wantAllAllowed: false,
			wantIDs:        []string{"ou1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applicable, result, err := policy.getAccessibleResources(tt.ctx, security.ActionListOUs, tt.resourceType)
			assert.Nil(t, err)
			assert.Equal(t, tt.wantApplicable, applicable)
			if tt.wantApplicable {
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantAllAllowed, result.AllAllowed)
				assert.ElementsMatch(t, tt.wantIDs, result.IDs)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isActionAllowedByPolicies
// ---------------------------------------------------------------------------

func TestIsActionAllowedByPolicies(t *testing.T) {
	errSvc := &serviceerror.ServiceError{
		Code:  "ERR-100",
		Error: i18ncore.I18nMessage{DefaultValue: "policy evaluation error"},
	}

	tests := []struct {
		name        string
		policy      authorizationPolicy
		wantAllowed bool
		wantErr     bool
	}{
		{
			// Policy has no opinion → allowed (permission check already passed).
			name:        "NotApplicable_DefaultAllowed",
			policy:      &stubPolicy{decision: policyDecisionNotApplicable},
			wantAllowed: true,
		},
		{
			name:        "PolicyDenied_ReturnsFalse",
			policy:      &stubPolicy{decision: policyDecisionDenied},
			wantAllowed: false,
		},
		{
			name:        "PolicyAllowed_ReturnsTrue",
			policy:      &stubPolicy{decision: policyDecisionAllowed},
			wantAllowed: true,
		},
		{
			name:        "PolicyError_ReturnsFalseAndError",
			policy:      &stubPolicy{actionErr: errSvc},
			wantAllowed: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &policies{membershipPolicy: tt.policy}
			allowed, err := isActionAllowedByPolicies(context.Background(), p, security.ActionCreateOU, nil)
			assert.Equal(t, tt.wantAllowed, allowed)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getAccessibleResourcesByPolicies
// ---------------------------------------------------------------------------

func TestGetAccessibleResourcesByPolicies(t *testing.T) {
	errSvc := &serviceerror.ServiceError{
		Code:  "ERR-200",
		Error: i18ncore.I18nMessage{DefaultValue: "resource policy error"},
	}

	tests := []struct {
		name           string
		policy         authorizationPolicy
		wantAllAllowed bool
		wantIDs        []string
		wantErr        bool
	}{
		{
			// Policy has no opinion on this resource type → AllAllowed fallback.
			name:           "NotApplicable_AllAllowed",
			policy:         &stubPolicy{applicable: false},
			wantAllAllowed: true,
		},
		{
			name: "ApplicableResult_Returned",
			policy: &stubPolicy{
				applicable: true,
				result:     &AccessibleResources{AllAllowed: false, IDs: []string{"ou1", "ou2"}},
			},
			wantAllAllowed: false,
			wantIDs:        []string{"ou1", "ou2"},
		},
		{
			name:    "PolicyError_ReturnsNilAndError",
			policy:  &stubPolicy{applicable: true, resourceErr: errSvc},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &policies{membershipPolicy: tt.policy}
			result, err := getAccessibleResourcesByPolicies(
				context.Background(), p, security.ActionListOUs, security.ResourceTypeOU)
			if tt.wantErr {
				assert.NotNil(t, err)
				assert.Nil(t, result)
				return
			}
			assert.Nil(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.wantAllAllowed, result.AllAllowed)
			if tt.wantIDs != nil {
				assert.ElementsMatch(t, tt.wantIDs, result.IDs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ouInheritancePolicy.isActionAllowed
// ---------------------------------------------------------------------------

func TestOuInheritancePolicy_IsActionAllowed(t *testing.T) {
	errSvc := &serviceerror.ServiceError{
		Code:  "ERR-300",
		Error: i18ncore.I18nMessage{DefaultValue: "hierarchy resolver error"},
	}

	tests := []struct {
		name         string
		ctx          context.Context
		actionCtx    *ActionContext
		resolver     *stubOUHierarchyResolver
		wantDecision policyDecision
		wantErr      bool
	}{
		{
			name:         "NilActionCtx_NotApplicable",
			ctx:          context.Background(),
			actionCtx:    nil,
			resolver:     &stubOUHierarchyResolver{},
			wantDecision: policyDecisionNotApplicable,
		},
		{
			name:         "EmptyOUID_NotApplicable",
			ctx:          context.Background(),
			actionCtx:    &ActionContext{OUID: ""},
			resolver:     &stubOUHierarchyResolver{},
			wantDecision: policyDecisionNotApplicable,
		},
		{
			name:         "NoCallerOU_Denied",
			ctx:          context.Background(),
			actionCtx:    &ActionContext{OUID: "parent-ou"},
			resolver:     &stubOUHierarchyResolver{isAncestorResult: true},
			wantDecision: policyDecisionDenied,
		},
		{
			// Caller is in the same OU as the resource (ancestor of self).
			name:         "SameOU_ResolverReturnsTrue_Allowed",
			ctx:          buildCtxWithOU("", "ou1"),
			actionCtx:    &ActionContext{OUID: "ou1"},
			resolver:     &stubOUHierarchyResolver{isAncestorResult: true},
			wantDecision: policyDecisionAllowed,
		},
		{
			// Caller is in a child OU; resource's OU is an ancestor → allowed (inherited visibility).
			name:         "CallerInChildOU_ResolverReturnsTrue_Allowed",
			ctx:          buildCtxWithOU("", "child-ou"),
			actionCtx:    &ActionContext{OUID: "parent-ou"},
			resolver:     &stubOUHierarchyResolver{isAncestorResult: true},
			wantDecision: policyDecisionAllowed,
		},
		{
			// Caller is in an unrelated OU; resource's OU is not an ancestor → denied.
			name:         "CallerInUnrelatedOU_ResolverReturnsFalse_Denied",
			ctx:          buildCtxWithOU("", "other-ou"),
			actionCtx:    &ActionContext{OUID: "parent-ou"},
			resolver:     &stubOUHierarchyResolver{isAncestorResult: false},
			wantDecision: policyDecisionDenied,
		},
		{
			// Resolver returns an error → denied + error propagated.
			name:         "ResolverError_DeniedWithError",
			ctx:          buildCtxWithOU("", "child-ou"),
			actionCtx:    &ActionContext{OUID: "parent-ou"},
			resolver:     &stubOUHierarchyResolver{isAncestorErr: errSvc},
			wantDecision: policyDecisionDenied,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &ouInheritancePolicy{resolver: tt.resolver}
			decision, err := policy.isActionAllowed(tt.ctx, tt.actionCtx)
			assert.Equal(t, tt.wantDecision, decision)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ouInheritancePolicy.getAccessibleResources
// ---------------------------------------------------------------------------

func TestOuInheritancePolicy_GetAccessibleResources(t *testing.T) {
	errSvc := &serviceerror.ServiceError{
		Code:  "ERR-400",
		Error: i18ncore.I18nMessage{DefaultValue: "ancestor lookup error"},
	}

	tests := []struct {
		name           string
		ctx            context.Context
		resourceType   security.ResourceType
		action         security.Action
		resolver       *stubOUHierarchyResolver
		wantApplicable bool
		wantAllAllowed bool
		wantIDs        []string
		wantErr        bool
	}{
		{
			// Non-inheritable resource type → not applicable.
			name:           "OUResource_NotApplicable",
			ctx:            buildCtxWithOU("", "ou1"),
			resourceType:   security.ResourceTypeOU,
			action:         security.ActionListOUs,
			resolver:       &stubOUHierarchyResolver{},
			wantApplicable: false,
		},
		{
			// Non-inheritable resource type → not applicable.
			name:           "UserResource_NotApplicable",
			ctx:            buildCtxWithOU("", "ou1"),
			resourceType:   security.ResourceTypeUser,
			action:         security.ActionListUsers,
			resolver:       &stubOUHierarchyResolver{},
			wantApplicable: false,
		},
		{
			// EntityType resource, no caller OU → applicable, empty IDs.
			name:           "EntityTypeResource_EmptyCallerOU_RestrictedEmpty",
			ctx:            context.Background(),
			resourceType:   security.ResourceTypeUserType,
			action:         security.ActionListUserTypes,
			resolver:       &stubOUHierarchyResolver{},
			wantApplicable: true,
			wantAllAllowed: false,
			wantIDs:        []string{},
		},
		{
			// EntityType resource, caller in child OU → resolver returns self + ancestors.
			name:           "EntityTypeResource_CallerInChildOU_ReturnsAncestors",
			ctx:            buildCtxWithOU("", "child-ou"),
			resourceType:   security.ResourceTypeUserType,
			action:         security.ActionListUserTypes,
			resolver:       &stubOUHierarchyResolver{ancestorIDs: []string{"parent-ou", "root-ou"}},
			wantApplicable: true,
			wantAllAllowed: false,
			wantIDs:        []string{"child-ou", "parent-ou", "root-ou"},
		},
		{
			// Resolver error for GetAncestorOUIDs → applicable true, nil result, error returned.
			name:           "EntityTypeResource_ResolverError_PropagatedAsError",
			ctx:            buildCtxWithOU("", "ou1"),
			resourceType:   security.ResourceTypeUserType,
			action:         security.ActionListUserTypes,
			resolver:       &stubOUHierarchyResolver{ancestorIDsErr: errSvc},
			wantApplicable: true,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &ouInheritancePolicy{resolver: tt.resolver}
			applicable, result, err := policy.getAccessibleResources(tt.ctx, tt.action, tt.resourceType)
			assert.Equal(t, tt.wantApplicable, applicable)
			if tt.wantErr {
				assert.NotNil(t, err)
				assert.Nil(t, result)
				return
			}
			assert.Nil(t, err)
			if tt.wantApplicable {
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantAllAllowed, result.AllAllowed)
				assert.ElementsMatch(t, tt.wantIDs, result.IDs)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isInheritanceEligible + selectPolicies
// ---------------------------------------------------------------------------

func TestIsInheritanceEligible(t *testing.T) {
	tests := []struct {
		name   string
		action security.Action
		want   bool
	}{
		{"EntityType_Read_Eligible", security.ActionReadUserType, true},
		{"EntityType_List_Eligible", security.ActionListUserTypes, true},
		{"EntityType_Create_NotEligible", security.ActionCreateUserType, false},
		{"EntityType_Update_NotEligible", security.ActionUpdateUserType, false},
		{"EntityType_Delete_NotEligible", security.ActionDeleteUserType, false},
		{"OU_Read_NotEligible", security.ActionReadOU, false},
		{"User_List_NotEligible", security.ActionListUsers, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// resourceType is not consulted; eligibility is determined solely by action.
			assert.Equal(t, tt.want, isInheritanceEligible(tt.action))
		})
	}
}

func TestSelectPolicies_InheritanceEligible_UsesInheritancePolicy(t *testing.T) {
	inh := &ouInheritancePolicy{resolver: &stubOUHierarchyResolver{}}
	p := &policies{
		membershipPolicy:  &ouMembershipPolicy{},
		inheritancePolicy: inh,
	}
	chain := selectPolicies(security.ActionReadUserType, p)
	assert.Len(t, chain, 1)
	_, ok := chain[0].(*ouInheritancePolicy)
	assert.True(t, ok, "expected ouInheritancePolicy for inheritance-eligible action")
}

func TestSelectPolicies_NilInheritance_UsesMembershipPolicy(t *testing.T) {
	membership := &ouMembershipPolicy{}
	p := &policies{membershipPolicy: membership}
	chain := selectPolicies(security.ActionReadUserType, p)
	assert.Len(t, chain, 1)
	assert.Equal(t, membership, chain[0])
}

func TestSelectPolicies_NonEligibleAction_UsesMembershipPolicy(t *testing.T) {
	membership := &ouMembershipPolicy{}
	p := &policies{
		membershipPolicy:  membership,
		inheritancePolicy: &ouInheritancePolicy{resolver: &stubOUHierarchyResolver{}},
	}
	// Write action on EntityType → not in inheritanceReadActions → membership policy.
	chain := selectPolicies(security.ActionCreateUserType, p)
	assert.Len(t, chain, 1)
	assert.Equal(t, membership, chain[0])
}

func TestSelectPolicies_NonEligibleResourceType_UsesMembershipPolicy(t *testing.T) {
	membership := &ouMembershipPolicy{}
	p := &policies{
		membershipPolicy:  membership,
		inheritancePolicy: &ouInheritancePolicy{resolver: &stubOUHierarchyResolver{}},
	}
	// ActionReadOU is not in inheritanceReadActions → membership policy regardless.
	chain := selectPolicies(security.ActionReadOU, p)
	assert.Len(t, chain, 1)
	assert.Equal(t, membership, chain[0])
}
