// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package adminflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
)

const testConfigKey = "roleAssignmentRemovalFlow"

type AdministrationFlowRunnerTestSuite struct {
	suite.Suite
	serverConfig *serverConfigReaderMock
	flows        *flowexecmock.FlowProviderMock
	executor     *flowexecmock.FlowExecServiceInterfaceMock
	runner       AdministrationFlowRunnerInterface
}

func TestAdministrationFlowRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(AdministrationFlowRunnerTestSuite))
}

func (s *AdministrationFlowRunnerTestSuite) SetupTest() {
	s.serverConfig = newServerConfigReaderMock(s.T())
	s.flows = flowexecmock.NewFlowProviderMock(s.T())
	s.executor = flowexecmock.NewFlowExecServiceInterfaceMock(s.T())
	s.runner = newAdministrationFlowRunner(s.serverConfig, s.flows, s.executor)
}

// configure makes the server configuration name the given handle for the role assignment removal flow.
func (s *AdministrationFlowRunnerTestSuite) configure(handle string) {
	s.serverConfig.On("GetMergedConfig", mock.Anything, "flow").Return(flowconfig.FlowSectionConfig{
		RoleAssignmentRemovalFlow: flowconfig.FlowTypeConfig{DefaultHandle: handle},
	}, (*tidcommon.ServiceError)(nil)).Once()
}

func (s *AdministrationFlowRunnerTestSuite) TestNoFlowConfigured() {
	s.configure("")

	ran, svcErr := s.runner.RunAdministrationFlow(context.Background(), testConfigKey, nil)

	s.False(ran)
	s.Nil(svcErr)
	s.flows.AssertNotCalled(s.T(), "GetFlowByHandle", mock.Anything, mock.Anything, mock.Anything)
}

func (s *AdministrationFlowRunnerTestSuite) TestRunsTheConfiguredFlow() {
	inputs := map[string]string{"targetRoleId": "r1", "targetAssigneeId": "u1"}
	s.configure("removal-flow")
	s.flows.On("GetFlowByHandle", mock.Anything, "removal-flow", providers.FlowTypeAdministration).
		Return(&providers.CompleteFlowDefinition{ID: "flow-1"}, (*tidcommon.ServiceError)(nil)).Once()
	s.executor.On("ExecuteByID", mock.Anything, "flow-1", "", false, "", inputs, "",
		[]flowexec.SessionData(nil)).
		Return(&flowexec.FlowStep{Status: providers.FlowStatusComplete}, (*tidcommon.ServiceError)(nil)).Once()

	ran, svcErr := s.runner.RunAdministrationFlow(context.Background(), testConfigKey, inputs)

	s.True(ran)
	s.Nil(svcErr)
}

func (s *AdministrationFlowRunnerTestSuite) TestReadsThePermissionRemovalEntry() {
	s.serverConfig.On("GetMergedConfig", mock.Anything, "flow").Return(flowconfig.FlowSectionConfig{
		RoleAssignmentRemovalFlow: flowconfig.FlowTypeConfig{DefaultHandle: "assignment-flow"},
	}, (*tidcommon.ServiceError)(nil)).Once()

	ran, svcErr := s.runner.RunAdministrationFlow(context.Background(), rolePermissionRemovalFlowKey, nil)

	s.False(ran, "only the assignment removal entry names a flow")
	s.Nil(svcErr)
}

func (s *AdministrationFlowRunnerTestSuite) TestUnknownConfigKey() {
	s.configure("removal-flow")

	ran, svcErr := s.runner.RunAdministrationFlow(context.Background(), "unknownFlow", nil)

	s.False(ran)
	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.flows.AssertNotCalled(s.T(), "GetFlowByHandle", mock.Anything, mock.Anything, mock.Anything)
}

func (s *AdministrationFlowRunnerTestSuite) TestRefusals() {
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "FEX-9"}
	cases := []struct {
		name     string
		prepare  func()
		expected *tidcommon.ServiceError
	}{
		{
			name: "configuration can't be read",
			prepare: func() {
				s.serverConfig.On("GetMergedConfig", mock.Anything, "flow").
					Return(nil, &tidcommon.InternalServerError).Once()
			},
			expected: &tidcommon.InternalServerError,
		},
		{
			name: "configuration has an unexpected type",
			prepare: func() {
				s.serverConfig.On("GetMergedConfig", mock.Anything, "flow").
					Return(map[string]any{}, (*tidcommon.ServiceError)(nil)).Once()
			},
			expected: &tidcommon.InternalServerError,
		},
		{
			name: "configured flow no longer exists",
			prepare: func() {
				s.configure("gone")
				s.flows.On("GetFlowByHandle", mock.Anything, "gone", providers.FlowTypeAdministration).
					Return((*providers.CompleteFlowDefinition)(nil), &flowmgt.ErrorFlowNotFound).Once()
			},
			expected: &ErrorConfiguredFlowMissing,
		},
		{
			name: "flow stops to ask for input",
			prepare: func() {
				s.configure("removal-flow")
				s.flows.On("GetFlowByHandle", mock.Anything, "removal-flow", providers.FlowTypeAdministration).
					Return(&providers.CompleteFlowDefinition{ID: "flow-1"}, (*tidcommon.ServiceError)(nil)).Once()
				s.executor.On("ExecuteByID", mock.Anything, "flow-1", "", false, "", mock.Anything, "", mock.Anything).
					Return(&flowexec.FlowStep{Status: providers.FlowStatusIncomplete},
						(*tidcommon.ServiceError)(nil)).Once()
			},
			expected: &ErrorFlowNeedsInput,
		},
		{
			name: "a step refuses",
			prepare: func() {
				s.configure("removal-flow")
				s.flows.On("GetFlowByHandle", mock.Anything, "removal-flow", providers.FlowTypeAdministration).
					Return(&providers.CompleteFlowDefinition{ID: "flow-1"}, (*tidcommon.ServiceError)(nil)).Once()
				s.executor.On("ExecuteByID", mock.Anything, "flow-1", "", false, "", mock.Anything, "", mock.Anything).
					Return(&flowexec.FlowStep{Status: providers.FlowStatusError, Error: refusal},
						(*tidcommon.ServiceError)(nil)).Once()
			},
			expected: refusal,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.prepare()

			ran, svcErr := s.runner.RunAdministrationFlow(context.Background(), testConfigKey, nil)

			s.False(ran)
			s.Equal(tc.expected, svcErr)
		})
	}
}
