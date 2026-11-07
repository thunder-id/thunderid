// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type UserDeletionSystemFlowTestSuite struct {
	suite.Suite
	entities *entitymock.EntityServiceInterfaceMock
	sharing  *sharingmock.SharingServiceInterfaceMock
	authz    *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	service  *userService
	flows    *systemFlowProviderMock
	order    []string
}

func TestUserDeletionSystemFlowTestSuite(t *testing.T) {
	suite.Run(t, new(UserDeletionSystemFlowTestSuite))
}

// SetupTest builds a service around user-1 with no expectations set: each test states the calls its
// deletion makes, so a missing or extra one fails it.
func (s *UserDeletionSystemFlowTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)

	s.order = nil
	s.flows = newSystemFlowProviderMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.service = &userService{
		entityService:      s.entities,
		authzService:       s.authz,
		dependencyRegistry: newNoBlockingDepsRegistry(),
		sharingService:     s.sharing,
		systemFlows:        s.flows,
		bus:                discardingBus{},
	}
}

// expectUser makes user-1 exist, and the caller allowed or refused its deletion.
func (s *UserDeletionSystemFlowTestSuite) expectUser(allowed bool) {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(&providers.Entity{
		ID: "user-1", Category: providers.EntityCategoryUser, OUID: "ou-1",
	}, nil).Once()
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Once()
}

// expectDeclarative reports whether user-1 is declarative.
func (s *UserDeletionSystemFlowTestSuite) expectDeclarative(declarative bool) {
	s.entities.On("IsEntityDeclarative", mock.Anything, "user-1").Return(declarative, nil).Once()
}

// expectRemoval records, in order, the sharing purge and the entity delete of user-1.
func (s *UserDeletionSystemFlowTestSuite) expectRemoval() {
	s.sharing.On("DeletePoliciesForResource", mock.Anything, UserSharingType, "user-1").
		Run(func(mock.Arguments) { s.order = append(s.order, "sharing") }).
		Return((*tidcommon.ServiceError)(nil)).Once()
	s.entities.On("DeleteEntity", mock.Anything, "user-1").
		Run(func(mock.Arguments) { s.order = append(s.order, "entity") }).Return(nil).Once()
}

// A deletion runs the user deletion pre flow, with the user as its subject, before anything of the user
// is removed. It revokes everything the user holds with no cutoff, so no flow runs after the write.
func (s *UserDeletionSystemFlowTestSuite) TestDeletionRevokesFirst() {
	s.expectUser(true)
	s.expectDeclarative(false)
	s.expectRemoval()
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, &flowcommon.SystemFlowContext{
		Handle: systemFlowUserDeletePre, Inputs: map[string]string{systemFlowInputSubject: "user-1"},
	}).Run(func(context.Context, *flowcommon.SystemFlowContext) { s.order = append(s.order, "flow") }).
		Return(&flowcommon.SystemFlowResult{}, nil).Once()

	s.Nil(s.service.DeleteUser(context.Background(), "user-1"))

	s.Equal([]string{"flow", "sharing", "entity"}, s.order)
}

// A refusal from the system flow is the caller's answer, and the user stays as it was.
func (s *UserDeletionSystemFlowTestSuite) TestARefusedRevocationDeletesNothing() {
	s.expectUser(true)
	s.expectDeclarative(false)
	refusal := &tidcommon.ServiceError{Code: "USR-1999", Type: tidcommon.ClientErrorType}
	s.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).Return(nil, refusal).Once()

	s.Equal(refusal, s.service.DeleteUser(context.Background(), "user-1"))

	s.Empty(s.order)
}

// A user that is not there has nothing to revoke.
func (s *UserDeletionSystemFlowTestSuite) TestAMissingUserRunsNoFlow() {
	s.entities.On("GetEntityProfile", mock.Anything, "gone").Return(nil, entity.ErrEntityNotFound).Once()

	s.Equal(&ErrorUserNotFound, s.service.DeleteUser(context.Background(), "gone"))
}

// A caller refused the deletion revokes nothing.
func (s *UserDeletionSystemFlowTestSuite) TestARefusedAccessCheckRunsNoFlow() {
	s.expectUser(false)

	s.Equal(&tidcommon.ErrorUnauthorized, s.service.DeleteUser(context.Background(), "user-1"))
}

// A declarative user cannot be deleted, so nothing is revoked.
func (s *UserDeletionSystemFlowTestSuite) TestADeclarativeUserRunsNoFlow() {
	s.expectUser(true)
	s.expectDeclarative(true)

	s.Equal(&ErrorCannotModifyDeclarativeResource, s.service.DeleteUser(context.Background(), "user-1"))
}

// A user other resources still depend on is kept, so nothing is revoked.
func (s *UserDeletionSystemFlowTestSuite) TestABlockingDependencyRunsNoFlow() {
	s.expectUser(true)
	s.expectDeclarative(false)
	s.service.dependencyRegistry = newBlockingDepsRegistry()

	svcErr := s.service.DeleteUser(context.Background(), "user-1")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorUserHasBlockingDependencies.Code, svcErr.Code)
}

// A service with no provider runs no system flows and still deletes the user.
func (s *UserDeletionSystemFlowTestSuite) TestNoProviderRunsNoFlow() {
	s.expectUser(true)
	s.expectDeclarative(false)
	s.expectRemoval()
	s.service.systemFlows = nil

	s.Nil(s.service.DeleteUser(context.Background(), "user-1"))

	s.Equal([]string{"sharing", "entity"}, s.order)
}

// The provider is bound to the service the package built, and refused for any other implementation.
func (s *UserDeletionSystemFlowTestSuite) TestSetSystemFlowProvider() {
	service := &userService{}
	s.Require().NoError(SetSystemFlowProvider(service, s.flows))
	s.Equal(s.flows, service.systemFlows)

	s.Error(SetSystemFlowProvider(NewUserServiceInterfaceMock(s.T()), s.flows))
}
