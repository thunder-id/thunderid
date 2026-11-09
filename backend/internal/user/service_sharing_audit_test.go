// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

// ServiceSharingAuditTestSuite checks the audit event each change to a sharing policy of a user publishes.
type ServiceSharingAuditTestSuite struct {
	suite.Suite
	fixture *sharingFixture
}

func TestServiceSharingAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceSharingAuditTestSuite))
}

// SetupSubTest wires a fixture whose caller may change users of the owning unit only.
func (s *ServiceSharingAuditTestSuite) SetupSubTest() {
	s.fixture = newSharingFixture(s.T(), map[security.Action][]string{security.ActionUpdateUser: {shareOwnerOU}})
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin on
// the policy with policyID of the fixture's user. A succeeded or denied attempt has the user as subject; a
// failed one has none. Nothing of the policy's contents is recorded.
func (s *ServiceSharingAuditTestSuite) requireOnlyEvent(events []publishedUserEvent, name string,
	action audit.Action, policyID string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(audit.Ref{Type: audit.TargetTypeUserSharingPolicy, ID: policyID}, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: svcTestAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(audit.Ref{Type: audit.TargetTypeUser, ID: svcTestUserID1}, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{shareOwnerOU, shareChildOU} {
		s.NotContains(recorded, value, "no organization unit a policy names is recorded")
	}
}

// anotherUsersPolicy is a policy the owning unit made for a user other than the fixture's.
func anotherUsersPolicy() sharing.Policy {
	foreign := userPolicy(shareOwnerOU)
	foreign.ResourceID = "user-2"
	return foreign
}

func (s *ServiceSharingAuditTestSuite) TestCreateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name     string
		userID   string
		request  sharing.PolicyRequest
		arrange  func()
		policyID string
		outcome  audit.Outcome
	}{
		{
			name: "the policy is created", userID: svcTestUserID1,
			arrange: func() {
				s.fixture.sharing.On("CreatePolicy", mock.Anything, UserSharingType, svcTestUserID1, shareOwnerOU,
					mock.Anything).Return(userPolicy(shareOwnerOU), (*tidcommon.ServiceError)(nil)).Once()
			},
			policyID: sharePolicyID, outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not act for the initiating unit", userID: svcTestUserID1,
			request: sharing.PolicyRequest{InitiatingOUID: shareChildOU},
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the framework refuses the policy", userID: svcTestUserID1,
			arrange: func() {
				s.fixture.sharing.On("CreatePolicy", mock.Anything, UserSharingType, svcTestUserID1, shareOwnerOU,
					mock.Anything).Return(sharing.Policy{}, &sharing.ErrorPolicyExists).Once()
			},
			outcome: audit.OutcomeFailed("usr_1031"),
		},
		{
			name: "the user does not exist", userID: "missing-user",
			arrange: func() {}, outcome: audit.OutcomeFailed("usr_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.fixture.service.bus = bus
			tc.arrange()

			_, svcErr := s.fixture.service.CreateSharingPolicy(adminContext(), tc.userID, tc.request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventUserSharingPolicyCreate, audit.ActionCreate, tc.policyID,
				tc.outcome)
		})
	}
}

func (s *ServiceSharingAuditTestSuite) TestUpdateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name       string
		initiating string
		arrange    func()
		outcome    audit.Outcome
	}{
		{
			name: "the policy is updated", initiating: shareOwnerOU,
			arrange: func() {
				s.fixture.sharing.On("UpdatePolicy", mock.Anything, sharePolicyID, mock.Anything).
					Return(userPolicy(shareOwnerOU), (*tidcommon.ServiceError)(nil)).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "another unit's policy", initiating: shareChildOU,
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the framework fails", initiating: shareOwnerOU,
			arrange: func() {
				s.fixture.sharing.On("UpdatePolicy", mock.Anything, sharePolicyID, mock.Anything).
					Return(sharing.Policy{}, &tidcommon.InternalServerError).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.fixture.service.bus = bus
			s.fixture.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
				Return(userPolicy(tc.initiating), (*tidcommon.ServiceError)(nil)).Once()
			tc.arrange()

			_, svcErr := s.fixture.service.UpdateSharingPolicy(adminContext(), svcTestUserID1, sharePolicyID,
				sharing.PolicyRequest{Version: 1})

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventUserSharingPolicyUpdate, audit.ActionUpdate, sharePolicyID,
				tc.outcome)
		})
	}
}

// A policy of another user answers as one the user does not hold, so its update is recorded as failed.
func (s *ServiceSharingAuditTestSuite) TestUpdateSharingPolicy_PublishesAFailureForAnotherUsersPolicy() {
	s.Run("the policy belongs to another user", func() {
		bus, published := newRecordingBus(s.T())
		s.fixture.service.bus = bus
		s.fixture.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
			Return(anotherUsersPolicy(), (*tidcommon.ServiceError)(nil)).Once()

		_, svcErr := s.fixture.service.UpdateSharingPolicy(adminContext(), svcTestUserID1, sharePolicyID,
			sharing.PolicyRequest{Version: 1})

		s.Equal(&ErrorSharingPolicyNotFound, svcErr)
		s.requireOnlyEvent(published(), audit.EventUserSharingPolicyUpdate, audit.ActionUpdate, sharePolicyID,
			audit.OutcomeFailed("usr_1030"))
	})
}

func (s *ServiceSharingAuditTestSuite) TestDeleteSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		policy  func() sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:   "the policy is deleted",
			policy: func() sharing.Policy { return userPolicy(shareOwnerOU) },
			arrange: func() {
				s.fixture.sharing.On("DeletePolicy", mock.Anything, sharePolicyID).
					Return((*tidcommon.ServiceError)(nil)).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "another unit's policy",
			policy:  func() sharing.Policy { return userPolicy(shareChildOU) },
			arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "a declared policy",
			policy: func() sharing.Policy {
				declared := userPolicy(shareOwnerOU)
				declared.Declared = true
				return declared
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("usr_1033"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.fixture.service.bus = bus
			s.fixture.sharing.On("GetPolicy", mock.Anything, sharePolicyID).
				Return(tc.policy(), (*tidcommon.ServiceError)(nil)).Once()
			tc.arrange()

			svcErr := s.fixture.service.DeleteSharingPolicy(adminContext(), svcTestUserID1, sharePolicyID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventUserSharingPolicyDelete, audit.ActionDelete, sharePolicyID,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no policy of the user records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses.
func (s *ServiceSharingAuditTestSuite) TestDeleteSharingPolicy_PublishesNothingForAMissingPolicy() {
	cases := []struct {
		name   string
		policy sharing.Policy
		err    *tidcommon.ServiceError
	}{
		{name: "no policy has the ID", err: &sharing.ErrorPolicyNotFound},
		{name: "the policy belongs to another user", policy: anotherUsersPolicy()},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.fixture.service.bus = bus
			s.fixture.sharing.On("GetPolicy", mock.Anything, sharePolicyID).Return(tc.policy, tc.err).Once()

			svcErr := s.fixture.service.DeleteSharingPolicy(adminContext(), svcTestUserID1, sharePolicyID)

			s.Nil(svcErr)
			s.Empty(published())
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceSharingAuditTestSuite) TestCreateSharingPolicy_SucceedsWhenTheBusRefusesTheEvent() {
	s.Run("the bus refuses the event", func() {
		failing := eventbusmock.NewEventBusInterfaceMock(s.T())
		failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
		s.fixture.service.bus = failing
		s.fixture.sharing.On("CreatePolicy", mock.Anything, UserSharingType, svcTestUserID1, shareOwnerOU,
			mock.Anything).Return(userPolicy(shareOwnerOU), (*tidcommon.ServiceError)(nil)).Once()

		policy, svcErr := s.fixture.service.CreateSharingPolicy(adminContext(), svcTestUserID1,
			sharing.PolicyRequest{})

		s.Nil(svcErr)
		s.Equal(sharePolicyID, policy.ID)
	})
}
