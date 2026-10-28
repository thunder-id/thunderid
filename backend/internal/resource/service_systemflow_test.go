// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/stretchr/testify/mock"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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

// existingAction readies action-123 on rs-123 to be deleted.
func (suite *ResourceServiceTestSuite) existingAction() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(true, nil)
}

// An action deletion runs the system flow with the action as input before anything is removed.
func (suite *ResourceServiceTestSuite) TestDeleteAction_RunsTheSystemFlowFirst() {
	suite.existingAction()
	var order []string
	suite.runs.onRun = func() { order = append(order, "flow") }
	suite.mockStore.On("DeleteAction", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(nil).
		Run(func(mock.Arguments) { order = append(order, "delete") })

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err)
	suite.Equal([]string{systemFlowActionDelete}, suite.runs.handles)
	suite.Equal([]map[string]string{{
		systemFlowInputResourceServer: "rs-123", systemFlowInputAction: "action-123",
	}}, suite.runs.inputs)
	suite.Equal([]string{"flow", "delete"}, order, "the action was removed before the revocation")
}

// An action under a resource names the resource to the flow as well.
func (suite *ResourceServiceTestSuite) TestDeleteAction_UnderAResourceNamesTheResource() {
	resourceID := testResourceID
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("GetResource", mock.Anything, resourceID, "rs-123").Return(providers.Resource{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", &resourceID).Return(true, nil)
	suite.mockStore.On("DeleteAction", mock.Anything, "action-123", "rs-123", &resourceID).Return(nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", &resourceID, "action-123")

	suite.Nil(err)
	suite.Equal([]map[string]string{{
		systemFlowInputResourceServer: "rs-123", systemFlowInputResource: resourceID,
		systemFlowInputAction: "action-123",
	}}, suite.runs.inputs)
}

// A refused system flow leaves the action as it was.
func (suite *ResourceServiceTestSuite) TestDeleteAction_RefusedFlowRemovesNothing() {
	suite.existingAction()
	suite.runs.err = &tidcommon.InternalServerError

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteAction", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// Without an executor the deletion is refused, so a server that forgot the wiring fails closed.
func (suite *ResourceServiceTestSuite) TestDeleteAction_WithoutAnExecutorIsRefused() {
	suite.existingAction()
	suite.service.SetSystemFlowExecutor(nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Equal(&tidcommon.InternalServerError, err)
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteAction", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// An action that is already gone runs no flow.
func (suite *ResourceServiceTestSuite) TestDeleteAction_MissingActionRunsNoFlow() {
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(providers.ResourceServer{}, nil)
	suite.mockStore.On("IsActionExist", mock.Anything, "action-123", "rs-123", (*string)(nil)).Return(false, nil)

	err := suite.service.DeleteAction(context.Background(), "rs-123", nil, "action-123")

	suite.Nil(err)
	suite.Empty(suite.runs.handles)
}
