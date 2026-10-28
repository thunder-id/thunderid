// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// systemFlowRuns stands in for the execution service's ExecuteSystemFlow, which a test in this package
// cannot reach through the generated mock: flowexec imports the executors, which import this package.
// It records each run and answers with err.
type systemFlowRuns struct {
	handles []string
	inputs  []map[string]string
	err     *tidcommon.ServiceError
}

func (r *systemFlowRuns) execute(_ context.Context, handle string,
	inputs map[string]string) *tidcommon.ServiceError {
	r.handles = append(r.handles, handle)
	r.inputs = append(r.inputs, inputs)
	return r.err
}

// deletableGroup returns a service around a group that may be deleted, with runs as its executor and
// cascadeErr from the cascade, so a test sees whether the deletion went past the flow.
func (suite *GroupServiceTestSuite) deletableGroup(runs *systemFlowRuns, cascadeErr error) (
	*groupService, *groupStoreInterfaceMock) {
	storeMock := newGroupStoreInterfaceMock(suite.T())
	storeMock.On("IsGroupDeclarative", mock.Anything, "grp-001").Return(false, nil).Once()
	storeMock.On("GetGroup", mock.Anything, "grp-001").
		Return(GroupDAO{ID: "grp-001", OUID: testOUID1}, nil).Once()
	service := &groupService{
		authzService:       newAllowAllAuthz(suite.T()),
		groupStore:         storeMock,
		transactioner:      &stubTransactioner{},
		dependencyRegistry: noopDepRegistry{cascadeErr: cascadeErr},
	}
	if runs != nil {
		service.executeSystemFlow = runs.execute
	}
	return service, storeMock
}

// A group deletion runs the system flow with the group as input, and goes on to the cascade only after.
func (suite *GroupServiceTestSuite) TestDeleteGroup_RunsTheSystemFlowBeforeTheCascade() {
	runs := &systemFlowRuns{}
	service, storeMock := suite.deletableGroup(runs, errors.New("cascade reached"))

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Equal(&tidcommon.InternalServerError, err, "the cascade must run after the flow")
	suite.Equal([]string{systemFlowGroupDelete}, runs.handles)
	suite.Equal([]map[string]string{{systemFlowInputGroup: "grp-001"}}, runs.inputs)
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

// A refused system flow leaves the group, its role assignments and its memberships as they were.
func (suite *GroupServiceTestSuite) TestDeleteGroup_RefusedFlowRemovesNothing() {
	refusal := &tidcommon.ServiceError{Code: "FEX-1", Type: tidcommon.ClientErrorType}
	// A cascade error would surface as a server error, so the refusal coming back proves it never ran.
	service, storeMock := suite.deletableGroup(&systemFlowRuns{err: refusal}, errors.New("cascade reached"))

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Same(refusal, err)
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

// Without an executor the deletion is refused, so a server that forgot the wiring fails closed.
func (suite *GroupServiceTestSuite) TestDeleteGroup_WithoutAnExecutorIsRefused() {
	service, storeMock := suite.deletableGroup(nil, nil)

	err := service.DeleteGroup(context.Background(), "grp-001")

	suite.Equal(&tidcommon.InternalServerError, err)
	storeMock.AssertNotCalled(suite.T(), "DeleteGroup", mock.Anything, mock.Anything)
}
