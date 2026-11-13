// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	entitypkg "github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const testAuditNewPassword = "N3w-audit-pa55word!"

// ServiceCredentialAuditTestSuite checks the audit event each attempt to set a user's credentials publishes.
type ServiceCredentialAuditTestSuite struct {
	suite.Suite
	entities    *entitymock.EntityServiceInterfaceMock
	entityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	authz       *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	service     *userService
}

func TestServiceCredentialAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceCredentialAuditTestSuite))
}

func (s *ServiceCredentialAuditTestSuite) SetupTest() {
	s.reset()
}

func (s *ServiceCredentialAuditTestSuite) SetupSubTest() {
	s.reset()
}

func (s *ServiceCredentialAuditTestSuite) reset() {
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.entityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.service = &userService{entityService: s.entities, entityTypeService: s.entityTypes,
		authzService: s.authz, bus: discardingBus{}}
}

// declares makes the user type declare password as its only credential.
func (s *ServiceCredentialAuditTestSuite) declares() {
	s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password", Credential: true}}, nil).Once()
}

// existing makes the entity layer hold svcTestUserID1 as a user of testOrgID, and the caller's access to
// change it answer allowed.
func (s *ServiceCredentialAuditTestSuite) existing(allowed bool) {
	s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).Return(&providers.Entity{
		Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: testOrgID, Type: testUserType,
	}, nil).Once()
	s.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser, mock.Anything).
		Return(allowed, (*tidcommon.ServiceError)(nil)).Once()
	if allowed {
		s.entities.On("IsEntityDeclarative", mock.Anything, svcTestUserID1).Return(false, nil).Once()
		s.authz.On("CanControlEntity", mock.Anything, svcTestUserID1).Return(nil).Once()
	} else {
		s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Once()
	}
}

// requireOnlyEvent checks that exactly one event was published for an attempt by the admin to set the
// credentials of the user with userID, and that it records neither the credential nor its type. A
// succeeded or denied attempt has the user as subject; a failed one has none.
func (s *ServiceCredentialAuditTestSuite) requireOnlyEvent(events []publishedUserEvent, userID string,
	outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	user := audit.Ref{Type: audit.TargetTypeUser, ID: userID}
	s.Equal(audit.EventUserCredentialUpdate, event.name)
	s.Equal(user, event.payload.Target())
	s.Equal(audit.ActionUpdate, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: svcTestAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(user, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	s.NotContains(recorded, testAuditNewPassword, "a credential is never recorded")
	s.NotContains(recorded, "password", "no credential type is recorded")
}

func (s *ServiceCredentialAuditTestSuite) TestUpdateUserCredentials_PublishesOneEventPerAttempt() {
	credentials := json.RawMessage(`{"password":"` + testAuditNewPassword + `"}`)
	cases := []struct {
		name    string
		userID  string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:   "a first password is set",
			userID: svcTestUserID1,
			arrange: func() {
				s.existing(true)
				s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
				s.entities.On("GetHeldCredentialTypes", mock.Anything, svcTestUserID1).Return([]string(nil), nil).Once()
				s.entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:   "a held password is replaced",
			userID: svcTestUserID1,
			arrange: func() {
				s.existing(true)
				s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
				s.entities.On("GetHeldCredentialTypes", mock.Anything, svcTestUserID1).
					Return([]string{"password"}, nil).Once()
				s.entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the caller may not change the user",
			userID:  svcTestUserID1,
			arrange: func() { s.existing(false) },
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name:   "the user does not exist",
			userID: svcTestUserID1,
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).
					Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
			},
			outcome: audit.OutcomeFailed("usr_1003"),
		},
		{
			name:   "the password fails the credential policy",
			userID: svcTestUserID1,
			arrange: func() {
				s.existing(true)
				s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).
					Return(entitypkg.ErrSchemaValidationFailed).Once()
			},
			outcome: audit.OutcomeFailed("usr_1041"),
		},
		{
			name:   "the new password cannot be stored",
			userID: svcTestUserID1,
			arrange: func() {
				s.existing(true)
				s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
				s.entities.On("GetHeldCredentialTypes", mock.Anything, svcTestUserID1).Return([]string(nil), nil).Once()
				s.entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).
					Return(errors.New("database is unavailable")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			svcErr := s.service.UpdateUserCredentials(adminContext(), tc.userID, credentials)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), tc.userID, tc.outcome)
		})
	}
}

// The credentials are already replaced when the post flow fails, so the attempt is recorded as succeeded
// while the caller still gets the failure, as an enrollment that stored its credential is.
func (s *ServiceCredentialAuditTestSuite) TestUpdateUserCredentials_RecordsReplacedCredentialsWhenThePostFlowFails() {
	flows := newSystemFlowProviderMock(s.T())
	s.service.systemFlows = flows
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus
	s.existing(true)
	s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
	s.entities.On("GetHeldCredentialTypes", mock.Anything, svcTestUserID1).Return([]string{"password"}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowUserCredentialUpdatePre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	s.entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowUserCredentialUpdatePost
	})).Return(nil, &tidcommon.InternalServerError).Once()

	svcErr := s.service.UpdateUserCredentials(adminContext(), svcTestUserID1,
		json.RawMessage(`{"password":"`+testAuditNewPassword+`"}`))

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.requireOnlyEvent(published(), svcTestUserID1, audit.OutcomeSucceeded)
}

// A flow that runs on the server's authority, such as a password reset through the recovery flow, keeps the
// subject of the request it came in on, which may be anyone's bearer token sent to the public flow path. The
// actor is the user whose credentials were set, or anonymous when they were not, never that subject.
func (s *ServiceCredentialAuditTestSuite) TestUpdateUserCredentials_FlowActorIsNotTheRequestsSubject() {
	credentials := json.RawMessage(`{"password":"` + testAuditNewPassword + `"}`)
	cases := []struct {
		name      string
		arrange   func()
		outcome   audit.Outcome
		wantActor audit.Actor
	}{
		{
			name: "the password is reset",
			arrange: func() {
				s.existing(true)
				s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
				s.entities.On("GetHeldCredentialTypes", mock.Anything, svcTestUserID1).
					Return([]string{"password"}, nil).Once()
				s.entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
			},
			outcome:   audit.OutcomeSucceeded,
			wantActor: audit.Actor{Type: audit.ActorTypeUser, ID: svcTestUserID1},
		},
		{
			name: "the new password fails the credential policy",
			arrange: func() {
				s.existing(true)
				s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).
					Return(entitypkg.ErrSchemaValidationFailed).Once()
			},
			outcome:   audit.OutcomeFailed("usr_1041"),
			wantActor: audit.Actor{Type: audit.ActorTypeAnonymous},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			// The context carries another user's subject, as a bearer token sent to the public flow path does.
			ctx := security.WithRuntimeContext(adminContext())
			s.service.UpdateUserCredentials(ctx, svcTestUserID1, credentials)

			events := published()
			s.Require().Len(events, 1)
			s.Equal(tc.wantActor, events[0].payload.Actor())
			s.Equal(tc.outcome, events[0].payload.Outcome())
			s.NotEqual(svcTestAdminID, events[0].payload.Actor().ID)
		})
		// An ordinary credential update preserves its recorded caller.
		s.Run(tc.name+" preserves a recorded administrator", func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			ctx := security.WithRecordedCaller(context.Background(), svcTestAdminID, testOrgID, nil)
			s.service.UpdateUserCredentials(ctx, svcTestUserID1, credentials)

			events := published()
			s.Require().Len(events, 1)
			s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: svcTestAdminID}, events[0].payload.Actor())
			s.Equal(tc.outcome, events[0].payload.Outcome())
		})
		// A user completing a reset runs as the administrator who started it, but is the actor.
		s.Run(tc.name+" completing a reset", func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			ctx := WithCredentialResetCompletion(
				security.WithRecordedCaller(context.Background(), svcTestAdminID, testOrgID, nil))
			s.service.UpdateUserCredentials(ctx, svcTestUserID1, credentials)

			events := published()
			s.Require().Len(events, 1)
			s.Equal(tc.wantActor, events[0].payload.Actor())
			s.Equal(tc.outcome, events[0].payload.Outcome())
		})
	}
}

// A request that names no user is recorded with no ID, and the credential it carried is not recorded.
func (s *ServiceCredentialAuditTestSuite) TestUpdateUserCredentials_PublishesARequestWithoutAUser() {
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus

	svcErr := s.service.UpdateUserCredentials(adminContext(), "",
		json.RawMessage(`{"password":"`+testAuditNewPassword+`"}`))

	s.Require().NotNil(svcErr)
	s.requireOnlyEvent(published(), "", audit.OutcomeFailed("usr_1018"))
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceCredentialAuditTestSuite) TestUpdateUserCredentials_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.existing(true)
	s.entities.On("ValidateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()
	s.entities.On("GetHeldCredentialTypes", mock.Anything, svcTestUserID1).Return([]string(nil), nil).Once()
	s.entities.On("UpdateCredentials", mock.Anything, svcTestUserID1, mock.Anything).Return(nil).Once()

	s.Nil(s.service.UpdateUserCredentials(adminContext(), svcTestUserID1,
		json.RawMessage(`{"password":"`+testAuditNewPassword+`"}`)))
}

// Each attempt to invalidate a credential publishes one event naming the credential type, never a value, with
// the outcome of the attempt and the error the caller gets. A refused attempt removes nothing.
func (s *ServiceCredentialAuditTestSuite) TestRemoveUserCredential_PublishesOneEventPerAttempt() {
	const marker = "2026-11-12T10:00:00.123456789Z"
	cases := []struct {
		name       string
		userID     string
		credential string
		arrange    func()
		removes    bool
		wantErr    *tidcommon.ServiceError
		outcome    audit.Outcome
	}{
		{
			name: "the password is invalidated", userID: svcTestUserID1, credential: "password",
			arrange: func() { s.invalidatable("password", nil) },
			removes: true, outcome: audit.OutcomeSucceeded,
		},
		{
			name: "no user is named", credential: "password", arrange: func() {},
			wantErr: &ErrorMissingUserID, outcome: audit.OutcomeFailed("usr_1002"),
		},
		{
			name: "no credential is named", userID: svcTestUserID1, credential: " ", arrange: func() {},
			wantErr: &ErrorInvalidCredential, outcome: audit.OutcomeFailed("usr_1024"),
		},
		{
			name: "a credential the server manages", userID: svcTestUserID1, credential: "passkey",
			arrange: func() {}, wantErr: &ErrorInvalidCredential, outcome: audit.OutcomeFailed("usr_1024"),
		},
		{
			name: "the user does not exist", userID: svcTestUserID1, credential: "password",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).
					Return((*providers.Entity)(nil), entitypkg.ErrEntityNotFound).Once()
			},
			wantErr: &ErrorUserNotFound, outcome: audit.OutcomeFailed("usr_1003"),
		},
		{
			name: "the entity is not a user", userID: svcTestUserID1, credential: "password",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).Return(&providers.Entity{
					Category: providers.EntityCategoryAgent, ID: svcTestUserID1, OUID: testOrgID,
				}, nil).Once()
			},
			wantErr: &ErrorUserNotFound, outcome: audit.OutcomeFailed("usr_1003"),
		},
		{
			name: "the profile cannot be read", userID: svcTestUserID1, credential: "password",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).
					Return((*providers.Entity)(nil), errors.New("database is unavailable")).Once()
			},
			wantErr: &tidcommon.InternalServerError, outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "the caller may not change the user", userID: svcTestUserID1, credential: "password",
			arrange: func() { s.existing(false) },
			wantErr: &tidcommon.ErrorUnauthorized, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the caller may not control the user", userID: svcTestUserID1, credential: "password",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).Return(&providers.Entity{
					Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: testOrgID,
				}, nil).Once()
				s.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser, mock.Anything).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				s.authz.On("CanControlEntity", mock.Anything, svcTestUserID1).
					Return(&tidcommon.ErrorUnauthorized).Once()
			},
			wantErr: &tidcommon.ErrorUnauthorized, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "a declarative user", userID: svcTestUserID1, credential: "password",
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, svcTestUserID1).Return(&providers.Entity{
					Category: providers.EntityCategoryUser, ID: svcTestUserID1, OUID: testOrgID,
				}, nil).Once()
				s.authz.On("IsActionAllowed", mock.Anything, security.ActionUpdateUser, mock.Anything).
					Return(true, (*tidcommon.ServiceError)(nil)).Once()
				s.authz.On("CanControlEntity", mock.Anything, svcTestUserID1).Return(nil).Once()
				s.entities.On("IsEntityDeclarative", mock.Anything, svcTestUserID1).Return(true, nil).Once()
			},
			wantErr: &ErrorCannotModifyDeclarativeResource, outcome: audit.OutcomeFailed("usr_1025"),
		},
		{
			name: "a credential the user's type does not declare", userID: svcTestUserID1, credential: "pin",
			arrange: func() {
				s.existing(true)
				s.declares()
			},
			wantErr: &ErrorInvalidCredential, outcome: audit.OutcomeFailed("usr_1024"),
		},
		{
			name: "the credential cannot be removed", userID: svcTestUserID1, credential: "password",
			arrange: func() { s.invalidatable("password", errors.New("database is unavailable")) },
			removes: true, wantErr: &tidcommon.InternalServerError, outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			got, svcErr := s.service.RemoveUserCredential(adminContext(), tc.userID, tc.credential)

			s.Equal(tc.wantErr, svcErr)
			if tc.wantErr == nil {
				s.Equal(marker, got)
			} else {
				s.Empty(got)
			}
			if !tc.removes {
				s.entities.AssertNotCalled(s.T(), "RemoveCredential", mock.Anything, mock.Anything, mock.Anything)
			}
			events := published()
			s.Require().Len(events, 1)
			event := events[0]
			user := audit.Ref{Type: audit.TargetTypeUser, ID: tc.userID}
			s.Equal(audit.EventUserCredentialInvalidate, event.name)
			s.Equal(user, event.payload.Target())
			s.Equal(audit.ActionUpdate, event.payload.Action())
			s.Equal(tc.outcome, event.payload.Outcome())
			s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: svcTestAdminID}, event.payload.Actor())
			s.Equal(map[string]string{audit.DetailCredentialType: tc.credential}, event.payload.Details())
		})
	}
}

// The credential is already removed when the post flow fails, so the attempt is recorded as succeeded, with
// the flow's execution ID, while the caller gets the failure and no marker.
func (s *ServiceCredentialAuditTestSuite) TestRemoveUserCredential_RecordsTheRemovalWhenThePostFlowFails() {
	flows := newSystemFlowProviderMock(s.T())
	s.service.systemFlows = flows
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus
	s.invalidatable("password", nil)
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowUserCredentialUpdatePre
	})).Return(&flowcommon.SystemFlowResult{}, nil).Once()
	flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.MatchedBy(func(c *flowcommon.SystemFlowContext) bool {
		return c.Handle == systemFlowUserCredentialUpdatePost
	})).Return(nil, &tidcommon.InternalServerError).Once()

	marker, svcErr := s.service.RemoveUserCredential(adminContext(), svcTestUserID1, "password")

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Empty(marker)
	events := published()
	s.Require().Len(events, 1)
	s.Equal(audit.OutcomeSucceeded, events[0].payload.Outcome())
	s.Equal(map[string]string{audit.DetailCredentialType: "password"}, events[0].payload.Details())
}

// The check runs the invalidation's checks, refusing as it does, and publishes and removes nothing.
func (s *ServiceCredentialAuditTestSuite) TestValidateRemoveUserCredential_ChangesNothing() {
	cases := []struct {
		name       string
		credential string
		arrange    func()
		wantErr    *tidcommon.ServiceError
	}{
		{name: "an allowed invalidation", credential: "password", arrange: func() {
			s.existing(true)
			s.declares()
		}},
		{name: "a forbidden invalidation", credential: "password", arrange: func() { s.existing(false) },
			wantErr: &tidcommon.ErrorUnauthorized},
		{name: "an undeclared credential", credential: "pin", arrange: func() {
			s.existing(true)
			s.declares()
		}, wantErr: &ErrorInvalidCredential},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			s.Equal(tc.wantErr, s.service.ValidateRemoveUserCredential(adminContext(), svcTestUserID1,
				tc.credential))

			s.Empty(published())
			s.entities.AssertNotCalled(s.T(), "RemoveCredential", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// invalidatable makes svcTestUserID1 an existing user the admin may change, whose type declares credential,
// and the removal of credential return a marker, or fail with removeErr.
func (s *ServiceCredentialAuditTestSuite) invalidatable(credential string, removeErr error) {
	s.existing(true)
	s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testUserType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: credential, Credential: true}}, nil).Once()
	marker := "2026-11-12T10:00:00.123456789Z"
	if removeErr != nil {
		marker = ""
	}
	s.entities.On("RemoveCredential", mock.Anything, svcTestUserID1, credential).Return(marker, removeErr).Once()
}
