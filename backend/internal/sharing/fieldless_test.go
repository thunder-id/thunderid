// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const fieldlessType = ResourceType("application")

// fieldlessDeclaration is a resource type that shares visibility and nothing else: no field is
// exposed to a policy, so there is nothing a target organization unit could be handed to edit.
//
// Applications are onboarded this way. Their configuration is global to the application rather than
// per-organization-unit state, so being reached by a policy means only that the organization unit
// may be named, never that it holds any part of the resource.
type fieldlessDeclaration struct{ owner string }

func (d *fieldlessDeclaration) ResourceType() ResourceType { return fieldlessType }
func (d *fieldlessDeclaration) Fields() []FieldDeclaration { return nil }
func (d *fieldlessDeclaration) OwningOUID(_ context.Context, _ string) (string, *tidcommon.ServiceError) {
	return d.owner, nil
}

type FieldlessTypeTestSuite struct {
	suite.Suite
	svc ServiceInterface
}

func TestFieldlessTypeTestSuite(t *testing.T) {
	suite.Run(t, new(FieldlessTypeTestSuite))
}

func (s *FieldlessTypeTestSuite) SetupTest() {
	resolver := testResolver()
	s.svc = newService(newFakeStore(), newDeclarativePolicyStore(), resolver, resolver,
		&fakeTransactioner{}, nil, nil, false)
	s.svc.RegisterResourceType(&fieldlessDeclaration{owner: ownerOU})
}

// declare records a declarative policy, which is how an application's policies always arrive: from
// a file, held in memory, re-seeded on every start.
func (s *FieldlessTypeTestSuite) declare(scope TargetOUScope) *tidcommon.ServiceError {
	_, svcErr := s.svc.CreateDeclarativePolicy(
		context.Background(), fieldlessType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: scope})
	return svcErr
}

// The two scopes an application may express, against the real engine rather than a stub.
func (s *FieldlessTypeTestSuite) TestBlanketScopesReachTheirTargets() {
	tests := []struct {
		name    string
		scope   TargetOUScope
		visible map[string]bool
	}{
		{
			name:  "every organization unit",
			scope: TargetOUScope{AllOUs: true},
			// otherOU sits in a tree of its own, which is the case one registration per customer
			// exists to avoid.
			visible: map[string]bool{rootOU: true, childOU: true, grandOU: true, otherOU: true},
		},
		{
			name:    "the owner's subtree, with one branch carved out",
			scope:   TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{childOU}},
			visible: map[string]bool{rootOU: true, otherOU: true, childOU: false, grandOU: false},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.Require().Nil(s.declare(tt.scope))

			for ouID, want := range tt.visible {
				visible, svcErr := s.svc.IsVisible(context.Background(), fieldlessType, testResource, ouID)
				s.Require().Nil(svcErr)
				s.Equal(want, visible, "organization unit %s", ouID)
			}
		})
	}
}

// A carve-out takes the branch beneath it too, which is what lets one customer be excluded without
// enumerating every other.
func (s *FieldlessTypeTestSuite) TestACarveOutTakesTheBranchBeneathIt() {
	s.Require().Nil(s.declare(TargetOUScope{AllOUs: true, ExcludedOUIDs: []string{childOU}}))

	visible, svcErr := s.svc.IsVisible(context.Background(), fieldlessType, testResource, grandOU)

	s.Require().Nil(svcErr)
	s.False(visible, "excluding an organization unit excludes everything beneath it")
}

// The owner holds its own resource whether or not any policy exists, which is why an application
// used in its own organization unit needs no policy at all.
func (s *FieldlessTypeTestSuite) TestTheOwnerNeedsNoPolicy() {
	visible, svcErr := s.svc.IsVisible(context.Background(), fieldlessType, testResource, ownerOU)

	s.Require().Nil(svcErr)
	s.True(visible)
}

// Being reached confers visibility and nothing else. A type that declares no field has no rule to
// resolve, so a reached organization unit holds no part of the resource.
func (s *FieldlessTypeTestSuite) TestBeingReachedHandsOverNoField() {
	s.Require().Nil(s.declare(TargetOUScope{AllOUs: true}))

	resolved, svcErr := s.svc.ResolveOverlayRules(context.Background(), fieldlessType, testResource, otherOU)

	s.Require().Nil(svcErr)
	s.True(resolved.Visible)
	s.False(resolved.Owned)
	s.Empty(resolved.Rules, "a type declaring no field hands over nothing")
}

// A declared policy is held in memory, so it cannot be deleted through the API: the file it came
// from is the only thing that decides it exists.
func (s *FieldlessTypeTestSuite) TestADeclaredPolicyCannotBeDeleted() {
	policy, svcErr := s.svc.CreateDeclarativePolicy(
		context.Background(), fieldlessType, testResource, ownerOU,
		PolicyRequest{TargetOUScope: TargetOUScope{AllOUs: true}})
	s.Require().Nil(svcErr)

	deleteErr := s.svc.DeletePolicy(context.Background(), policy.ID)

	s.Require().NotNil(deleteErr)
	s.Equal(ErrorPolicyDeclared.Code, deleteErr.Code)
}

// One policy per resource per initiating organization unit holds for declared policies too. Startup
// replay does not rely on this being an upsert: declared policies live in memory, so every start
// begins with an empty store. A second declaration within one start means two documents said the
// same thing, and is refused rather than merged.
func (s *FieldlessTypeTestSuite) TestASecondDeclarationForTheSameInitiatorIsRefused() {
	s.Require().Nil(s.declare(TargetOUScope{AllOUs: true}))

	svcErr := s.declare(TargetOUScope{AllOUs: true})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorPolicyExists.Code, svcErr.Code)

	policies, listErr := s.svc.ListPolicies(context.Background(), fieldlessType, testResource)
	s.Require().Nil(listErr)
	s.Len(policies, 1, "the refused declaration must not have been appended")
}
