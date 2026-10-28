// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

// systemFlowRuns stands in for the execution service's ExecuteSystemFlow, which a test in this package
// cannot reach through the generated mock: flowexec imports the executors, which import this package.
// It records each run and answers with err.
type systemFlowRuns struct {
	handles []string
	inputs  []map[string]string
	err     *tidcommon.ServiceError
	onRun   func()
}

func (r *systemFlowRuns) execute(_ context.Context, handle string,
	inputs map[string]string) *tidcommon.ServiceError {
	r.handles = append(r.handles, handle)
	r.inputs = append(r.inputs, inputs)
	if r.onRun != nil {
		r.onRun()
	}
	return r.err
}

type UserDeletionSystemFlowTestSuite struct {
	suite.Suite
	entities *entitymock.EntityServiceInterfaceMock
	service  *userService
	runs     *systemFlowRuns
	order    []string
}

func TestUserDeletionSystemFlowTestSuite(t *testing.T) {
	suite.Run(t, new(UserDeletionSystemFlowTestSuite))
}

// SetupTest builds a service around a user that may be deleted, recording in order what the deletion
// does: the system flow, the sharing purge and the entity delete.
func (s *UserDeletionSystemFlowTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)

	s.order = nil
	s.runs = &systemFlowRuns{onRun: func() { s.order = append(s.order, "flow") }}
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(&providers.Entity{
		ID: "user-1", Category: providers.EntityCategoryUser, OUID: "ou-1",
	}, nil).Maybe()
	s.entities.On("IsEntityDeclarative", mock.Anything, "user-1").Return(false, nil).Maybe()
	s.entities.On("DeleteEntity", mock.Anything, "user-1").
		Run(func(mock.Arguments) { s.order = append(s.order, "entity") }).Return(nil).Maybe()
	sharing := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharing.On("DeletePoliciesForResource", mock.Anything, UserSharingType, "user-1").
		Run(func(mock.Arguments) { s.order = append(s.order, "sharing") }).
		Return((*tidcommon.ServiceError)(nil)).Maybe()
	s.service = &userService{
		entityService:      s.entities,
		authzService:       newAllowAllAuthz(s.T()),
		dependencyRegistry: newNoBlockingDepsRegistry(),
		sharingService:     sharing,
		executeSystemFlow:  s.runs.execute,
	}
}

// A deletion runs the user deletion system flow, with the user as its subject, before anything of the
// user is removed.
func (s *UserDeletionSystemFlowTestSuite) TestDeletionRevokesFirst() {
	s.Nil(s.service.DeleteUser(context.Background(), "user-1"))

	s.Equal([]string{systemFlowUserDelete}, s.runs.handles)
	s.Equal(map[string]string{systemFlowInputSubject: "user-1"}, s.runs.inputs[0])
	s.Equal([]string{"flow", "sharing", "entity"}, s.order, "something was removed before the revocation")
}

// A refusal from the system flow is the caller's answer, and the user stays as it was.
func (s *UserDeletionSystemFlowTestSuite) TestARefusedRevocationDeletesNothing() {
	s.runs.err = &tidcommon.ServiceError{Code: "USR-1999", Type: tidcommon.ClientErrorType}

	s.Equal(s.runs.err, s.service.DeleteUser(context.Background(), "user-1"))

	s.Equal([]string{"flow"}, s.order)
}

// A server that never wired an executor refuses to delete rather than deleting with the user's tokens
// left valid.
func (s *UserDeletionSystemFlowTestSuite) TestWithoutAnExecutorTheDeletionIsRefused() {
	s.service.executeSystemFlow = nil

	s.Equal(&tidcommon.InternalServerError, s.service.DeleteUser(context.Background(), "user-1"))

	s.Empty(s.order)
}

// A user that is not there has nothing to revoke.
func (s *UserDeletionSystemFlowTestSuite) TestAMissingUserRunsNoFlow() {
	entities := entitymock.NewEntityServiceInterfaceMock(s.T())
	entities.On("GetEntityProfile", mock.Anything, "gone").Return(nil, entity.ErrEntityNotFound)
	s.service.entityService = entities

	s.Equal(&ErrorUserNotFound, s.service.DeleteUser(context.Background(), "gone"))

	s.Empty(s.runs.handles)
}
