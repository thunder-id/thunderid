// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// stubPolicy is a configurable authorizationPolicy for testing. It allows independent
// control of isActionAllowed and getAccessibleResources behavior.
type stubPolicy struct {
	// isActionAllowed response fields.
	decision  policyDecision
	actionErr *tidcommon.ServiceError

	// getAccessibleResources response fields.
	applicable  bool
	result      *AccessibleResources
	resourceErr *tidcommon.ServiceError
}

func (p *stubPolicy) isActionAllowed(_ context.Context,
	_ *ActionContext) (policyDecision, *tidcommon.ServiceError) {
	return p.decision, p.actionErr
}

func (p *stubPolicy) getAccessibleResources(_ context.Context, _ security.Action,
	_ security.ResourceType) (bool, *AccessibleResources, *tidcommon.ServiceError) {
	return p.applicable, p.result, p.resourceErr
}

// stubOUHierarchyResolver is a configurable OUHierarchyResolver for testing.
type stubOUHierarchyResolver struct {
	// IsAncestor response fields.
	isAncestorResult bool
	isAncestorErr    *tidcommon.ServiceError

	// GetAncestorOUIDs response fields.
	ancestorIDs    []string
	ancestorIDsErr *tidcommon.ServiceError
}

func (r *stubOUHierarchyResolver) IsAncestor(
	_ context.Context, _, _ string,
) (bool, *tidcommon.ServiceError) {
	return r.isAncestorResult, r.isAncestorErr
}

func (r *stubOUHierarchyResolver) GetAncestorOUIDs(
	_ context.Context, _ string,
) ([]string, *tidcommon.ServiceError) {
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
	errSvc := &tidcommon.ServiceError{
		Code:  "ERR-100",
		Error: tidcommon.I18nMessage{DefaultValue: "policy evaluation error"},
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
	errSvc := &tidcommon.ServiceError{
		Code:  "ERR-200",
		Error: tidcommon.I18nMessage{DefaultValue: "resource policy error"},
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
	errSvc := &tidcommon.ServiceError{
		Code:  "ERR-300",
		Error: tidcommon.I18nMessage{DefaultValue: "hierarchy resolver error"},
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
	errSvc := &tidcommon.ServiceError{
		Code:  "ERR-400",
		Error: tidcommon.I18nMessage{DefaultValue: "ancestor lookup error"},
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
		{"AgentType_Read_Eligible", security.ActionReadAgentType, true},
		{"AgentType_List_Eligible", security.ActionListAgentTypes, true},
		{"AgentType_Create_NotEligible", security.ActionCreateAgentType, false},
		{"AgentType_Update_NotEligible", security.ActionUpdateAgentType, false},
		{"AgentType_Delete_NotEligible", security.ActionDeleteAgentType, false},
		{"OUType_Read_Eligible", security.ActionReadOUType, true},
		{"OUType_List_Eligible", security.ActionListOUTypes, true},
		{"OUType_Create_NotEligible", security.ActionCreateOUType, false},
		{"OUType_Update_NotEligible", security.ActionUpdateOUType, false},
		{"OUType_Delete_NotEligible", security.ActionDeleteOUType, false},
		{"NotificationTemplate_Read_Eligible", security.ActionReadNotificationTemplate, true},
		{"NotificationTemplate_List_Eligible", security.ActionListNotificationTemplates, true},
		{"NotificationTemplate_Create_NotEligible", security.ActionCreateNotificationTemplate, false},
		{"NotificationTemplate_Update_NotEligible", security.ActionUpdateNotificationTemplate, false},
		{"NotificationTemplate_Delete_NotEligible", security.ActionDeleteNotificationTemplate, false},
		{"VC_Read_Eligible", security.ActionReadVC, true},
		{"VC_List_Eligible", security.ActionListVCs, true},
		{"VC_Create_NotEligible", security.ActionCreateVC, false},
		{"VC_Update_NotEligible", security.ActionUpdateVC, false},
		{"VC_Delete_NotEligible", security.ActionDeleteVC, false},
		{"ResourceServer_Read_Eligible", security.ActionReadResourceServer, true},
		{"ResourceServer_List_Eligible", security.ActionListResourceServers, true},
		{"ResourceServer_Update_NotEligible", security.ActionUpdateResourceServer, false},
		{"ResourceServer_Delete_NotEligible", security.ActionDeleteResourceServer, false},
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

// ---------------------------------------------------------------------------
// ouChildPolicy
// ---------------------------------------------------------------------------

func TestOuChildPolicy_IsActionAllowed(t *testing.T) {
	notFound := &tidcommon.ServiceError{Code: "OU-1003", Type: tidcommon.ClientErrorType}
	tests := []struct {
		name         string
		includeSelf  bool
		target       string
		resolver     *stubOUHierarchyResolver
		wantDecision policyDecision
		wantErr      bool
	}{
		{name: "NoTarget_NotApplicable", target: "", resolver: &stubOUHierarchyResolver{},
			wantDecision: policyDecisionNotApplicable},
		{name: "DirectChild_Allowed", target: "child", resolver: &stubOUHierarchyResolver{
			ancestorIDs: []string{"ou1", "root"}}, wantDecision: policyDecisionAllowed},
		{name: "Grandchild_Denied", target: "grandchild", resolver: &stubOUHierarchyResolver{
			ancestorIDs: []string{"child", "ou1", "root"}}, wantDecision: policyDecisionDenied},
		{name: "Sibling_Denied", target: "sibling", resolver: &stubOUHierarchyResolver{
			ancestorIDs: []string{"root"}}, wantDecision: policyDecisionDenied},
		{name: "Root_Denied", target: "root", resolver: &stubOUHierarchyResolver{ancestorIDs: []string{}},
			wantDecision: policyDecisionDenied},
		{name: "OwnUnitForAWrite_Denied", target: "ou1", resolver: &stubOUHierarchyResolver{},
			wantDecision: policyDecisionDenied},
		{name: "OwnUnitForARead_Allowed", includeSelf: true, target: "ou1", resolver: &stubOUHierarchyResolver{},
			wantDecision: policyDecisionAllowed},
		{name: "UnknownUnit_Denied", target: "gone", resolver: &stubOUHierarchyResolver{ancestorIDsErr: notFound},
			wantDecision: policyDecisionDenied},
		{name: "ResolverFailure_DeniedWithError", target: "child", resolver: &stubOUHierarchyResolver{
			ancestorIDsErr: &tidcommon.InternalServerError}, wantDecision: policyDecisionDenied, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &ouChildPolicy{resolver: tc.resolver, includeSelf: tc.includeSelf}
			decision, err := p.isActionAllowed(buildCtxWithOU("", "ou1"), &ActionContext{OUID: tc.target})
			assert.Equal(t, tc.wantDecision, decision)
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}

	decision, err := (&ouChildPolicy{resolver: &stubOUHierarchyResolver{}, includeSelf: true}).isActionAllowed(
		context.Background(), &ActionContext{OUID: "child"})
	assert.Nil(t, err)
	assert.Equal(t, policyDecisionDenied, decision, "a caller naming no unit reaches none")
}

func TestOuChildPolicy_GetAccessibleResources_NotApplicable(t *testing.T) {
	applicable, result, err := (&ouChildPolicy{}).getAccessibleResources(
		buildCtxWithOU("", "ou1"), security.ActionListChildOUs, security.ResourceTypeOU)
	assert.False(t, applicable)
	assert.Nil(t, result)
	assert.Nil(t, err)
}

func TestSelectPolicies_OUManagementUsesTheChildPolicies(t *testing.T) {
	read := &ouChildPolicy{includeSelf: true}
	write := &ouChildPolicy{}
	membership := &ouMembershipPolicy{}
	p := &policies{membershipPolicy: membership, ouReadPolicy: read, ouWritePolicy: write}

	assert.Equal(t, []authorizationPolicy{read}, selectPolicies(security.ActionReadOU, p))
	assert.Equal(t, []authorizationPolicy{write}, selectPolicies(security.ActionUpdateOU, p))
	assert.Equal(t, []authorizationPolicy{write}, selectPolicies(security.ActionDeleteOU, p))
	assert.Equal(t, []authorizationPolicy{membership}, selectPolicies(security.ActionListChildOUs, p),
		"children are listed only for the caller's own unit")
	assert.Equal(t, []authorizationPolicy{membership}, selectPolicies(security.ActionCreateOU, p),
		"a child is created only beneath the caller's own unit")
}
