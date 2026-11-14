// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testSharingRoleID  = "role1"
	testSharingPolicy  = "p1"
	testSharingOwnerOU = "ou-owner"
	testSharingChildOU = "ou-child"
)

// RoleSharingAuditTestSuite checks the audit event each change to a sharing policy of a role publishes.
type RoleSharingAuditTestSuite struct {
	suite.Suite
	store     *roleStoreInterfaceMock
	sharing   *sharingmock.SharingServiceInterfaceMock
	published func() []publishedEvent
	service   *roleService
}

func TestRoleSharingAuditTestSuite(t *testing.T) {
	suite.Run(t, new(RoleSharingAuditTestSuite))
}

// SetupTest wires a role owned by ou-owner, and a caller who may change roles of ou-owner only.
func (s *RoleSharingAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}))
	s.store = newRoleStoreInterfaceMock(s.T())
	s.store.On("GetRole", mock.Anything, testSharingRoleID).
		Return(RoleWithPermissions{ID: testSharingRoleID, Name: testAuditRoleName, OUID: testSharingOwnerOU}, nil).
		Maybe()
	s.store.On("GetRole", mock.Anything, "missing").Return(RoleWithPermissions{}, ErrRoleNotFound).Maybe()
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	ownerOnly := func(actionCtx *sysauthz.ActionContext) bool {
		return actionCtx == nil || actionCtx.OUID == "" || actionCtx.OUID == testSharingOwnerOU
	}
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.MatchedBy(ownerOnly)).Return(true, nil).Maybe()
	authz.On("IsActionAllowed", mock.Anything, mock.Anything,
		mock.MatchedBy(func(actionCtx *sysauthz.ActionContext) bool { return !ownerOnly(actionCtx) })).
		Return(false, nil).Maybe()
	ous := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ous.On("GetOrganizationUnit", mock.Anything, testSharingOwnerOU).
		Return(oupkg.OrganizationUnit{ID: testSharingOwnerOU, Handle: "owner"}, nil).Maybe()
	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	s.service = newRoleService(s.store, entitymock.NewEntityServiceInterfaceMock(s.T()),
		groupmock.NewGroupServiceInterfaceMock(s.T()), ous,
		resourcemock.NewResourceServiceInterfaceMock(s.T()), &fakeTransactioner{}, authz, s.sharing, bus,
		nil).(*roleService)
}

func (s *RoleSharingAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *RoleSharingAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// policyOf is a policy of the role made by the initiating organization unit.
func policyOf(initiating string) sharing.Policy {
	return sharing.Policy{ID: testSharingPolicy, ResourceType: RoleSharingType, ResourceID: testSharingRoleID,
		InitiatingOUID: initiating}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin on
// the policy with policyID of the role. A succeeded or denied attempt has the role as subject; a failed one
// has none. Nothing of the policy or the role is recorded.
func (s *RoleSharingAuditTestSuite) requireOnlyEvent(name string, action audit.Action, policyID string,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(audit.Ref{Type: audit.TargetTypeRoleSharingPolicy, ID: policyID}, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(audit.Ref{Type: audit.TargetTypeRole, ID: testSharingRoleID}, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testSharingOwnerOU, testSharingChildOU, testAuditRoleName} {
		s.NotContains(recorded, value, "nothing of the policy or the role is recorded")
	}
}

func (s *RoleSharingAuditTestSuite) TestCreateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name     string
		roleID   string
		request  sharing.PolicyRequest
		arrange  func()
		policyID string
		outcome  audit.Outcome
	}{
		{
			name: "the policy is created", roleID: testSharingRoleID,
			arrange: func() {
				s.sharing.On("CreatePolicy", mock.Anything, RoleSharingType, testSharingRoleID, testSharingOwnerOU,
					mock.Anything).Return(policyOf(testSharingOwnerOU), (*tidcommon.ServiceError)(nil)).Once()
			},
			policyID: testSharingPolicy, outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not act for the initiating unit", roleID: testSharingRoleID,
			request: sharing.PolicyRequest{InitiatingOUID: testSharingChildOU},
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the framework refuses the policy", roleID: testSharingRoleID,
			arrange: func() {
				s.sharing.On("CreatePolicy", mock.Anything, RoleSharingType, testSharingRoleID, testSharingOwnerOU,
					mock.Anything).Return(sharing.Policy{}, &sharing.ErrorPolicyExists).Once()
			},
			outcome: audit.OutcomeFailed("rol_1023"),
		},
		{
			name: "the role does not exist", roleID: "missing",
			arrange: func() {}, outcome: audit.OutcomeFailed("rol_1003"),
		},
		{
			// The role is looked up before the caller is checked, so a role that is not there is a failed
			// attempt whoever the initiating unit is, and records no subject.
			name: "the role does not exist and the caller may not act for the initiating unit", roleID: "missing",
			request: sharing.PolicyRequest{InitiatingOUID: testSharingChildOU},
			arrange: func() {}, outcome: audit.OutcomeFailed("rol_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := s.service.CreateSharingPolicy(adminContext(), tc.roleID, tc.request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventRoleSharingPolicyCreate, audit.ActionCreate, tc.policyID, tc.outcome)
		})
	}
}

func (s *RoleSharingAuditTestSuite) TestUpdateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is updated", policy: policyOf(testSharingOwnerOU),
			arrange: func() {
				s.sharing.On("UpdatePolicy", mock.Anything, testSharingPolicy, mock.Anything).
					Return(policyOf(testSharingOwnerOU), (*tidcommon.ServiceError)(nil)).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "another unit's policy", policy: policyOf(testSharingChildOU),
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the policy belongs to another role",
			policy: sharing.Policy{ID: testSharingPolicy, ResourceType: RoleSharingType, ResourceID: "role2",
				InitiatingOUID: testSharingOwnerOU},
			arrange: func() {}, outcome: audit.OutcomeFailed("rol_1022"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicy).
				Return(tc.policy, (*tidcommon.ServiceError)(nil)).Once()
			tc.arrange()

			_, svcErr := s.service.UpdateSharingPolicy(adminContext(), testSharingRoleID, testSharingPolicy,
				sharing.PolicyRequest{Version: 1})

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventRoleSharingPolicyUpdate, audit.ActionUpdate, testSharingPolicy, tc.outcome)
		})
	}
}

func (s *RoleSharingAuditTestSuite) TestDeleteSharingPolicy_PublishesOneEventPerAttempt() {
	declared := policyOf(testSharingOwnerOU)
	declared.Declared = true
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is deleted", policy: policyOf(testSharingOwnerOU),
			arrange: func() {
				s.sharing.On("DeletePolicy", mock.Anything, testSharingPolicy).
					Return((*tidcommon.ServiceError)(nil)).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "another unit's policy", policy: policyOf(testSharingChildOU), arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "a declared policy", policy: declared, arrange: func() {}, outcome: audit.OutcomeFailed("rol_1025")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicy).
				Return(tc.policy, (*tidcommon.ServiceError)(nil)).Once()
			tc.arrange()

			svcErr := s.service.DeleteSharingPolicy(adminContext(), testSharingRoleID, testSharingPolicy)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventRoleSharingPolicyDelete, audit.ActionDelete, testSharingPolicy, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no policy of the role records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses.
func (s *RoleSharingAuditTestSuite) TestDeleteSharingPolicy_PublishesNothingForAMissingPolicy() {
	cases := []struct {
		name   string
		policy sharing.Policy
		err    *tidcommon.ServiceError
	}{
		{name: "no policy has the ID", err: &sharing.ErrorPolicyNotFound},
		{name: "the policy belongs to another role", policy: sharing.Policy{ID: testSharingPolicy,
			ResourceType: RoleSharingType, ResourceID: "role2", InitiatingOUID: testSharingOwnerOU}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.sharing.On("GetPolicy", mock.Anything, testSharingPolicy).Return(tc.policy, tc.err).Once()

			svcErr := s.service.DeleteSharingPolicy(adminContext(), testSharingRoleID, testSharingPolicy)

			s.Nil(svcErr)
			s.Empty(s.published())
		})
	}
}

// A role that is not there fails the attempt before any policy is read.
func (s *RoleSharingAuditTestSuite) TestUpdateAndDeleteSharingPolicy_FailWhenTheRoleDoesNotExist() {
	s.Run("update", func() {
		_, svcErr := s.service.UpdateSharingPolicy(adminContext(), "missing", testSharingPolicy,
			sharing.PolicyRequest{Version: 1})

		s.Require().NotNil(svcErr)
		s.requireOnlyEvent(audit.EventRoleSharingPolicyUpdate, audit.ActionUpdate, testSharingPolicy,
			audit.OutcomeFailed("rol_1003"))
	})
	s.Run("delete", func() {
		svcErr := s.service.DeleteSharingPolicy(adminContext(), "missing", testSharingPolicy)

		s.Require().NotNil(svcErr)
		s.requireOnlyEvent(audit.EventRoleSharingPolicyDelete, audit.ActionDelete, testSharingPolicy,
			audit.OutcomeFailed("rol_1003"))
	})
}

// A bus that cannot take the event never fails the change it describes.
func (s *RoleSharingAuditTestSuite) TestCreateSharingPolicy_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.sharing.On("CreatePolicy", mock.Anything, RoleSharingType, testSharingRoleID, testSharingOwnerOU,
		mock.Anything).Return(policyOf(testSharingOwnerOU), (*tidcommon.ServiceError)(nil)).Once()

	policy, svcErr := s.service.CreateSharingPolicy(adminContext(), testSharingRoleID, sharing.PolicyRequest{})

	s.Nil(svcErr)
	s.Equal(testSharingPolicy, policy.ID)
}
