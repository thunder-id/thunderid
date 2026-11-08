// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOU   = "ou-own"
	authzOtherOU = "ou-other"
	authzFlowID  = "flow-authz"
)

// expectOwnOUOnly expects each action to be checked, and allows it on the caller's own organization unit
// or in a runtime context. A list action lists the caller's own organization unit alone. Any other
// action is unexpected.
func expectOwnOUOnly(m *sysauthzmock.SystemAuthorizationServiceInterfaceMock, actions ...security.Action) {
	for _, action := range actions {
		if strings.HasSuffix(string(action), ":list") {
			m.On("GetAccessibleResources", mock.Anything, action, security.ResourceTypeOU).
				Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU}}, nil)
			continue
		}
		m.On("IsActionAllowed", mock.Anything, action, mock.Anything).Return(
			func(ctx context.Context, _ security.Action, c *sysauthz.ActionContext) (bool, *tidcommon.ServiceError) {
				return security.IsRuntimeContext(ctx) || c.OUID == authzOwnOU, nil
			})
	}
}

// FlowAuthzTestSuite covers the organization unit checks on the flow management operations.
type FlowAuthzTestSuite struct {
	suite.Suite
	authz   *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store   *flowStoreInterfaceMock
	sharing *sharingmock.SharingServiceInterfaceMock
	service *flowMgtService
}

func TestFlowAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(FlowAuthzTestSuite))
}

func (s *FlowAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.store = newFlowStoreInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.service = newFlowMgtService(s.store, nil, nil, nil, nil, nil, nil, &stubTransactioner{}, nil, nil, nil, nil,
		s.sharing, nil, discardingBus{}, nil, s.authz)
}

func (s *FlowAuthzTestSuite) storedFlow(ouID string) {
	s.store.EXPECT().GetFlowByID(mock.Anything, authzFlowID).
		Return(&providers.CompleteFlowDefinition{ID: authzFlowID, OUID: ouID}, nil).Maybe()
}

func (s *FlowAuthzTestSuite) TestGetFlow_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadFlow)
	s.storedFlow(authzOtherOU)

	flow, svcErr := s.service.GetFlow(context.Background(), authzFlowID)
	s.Nil(flow)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *FlowAuthzTestSuite) TestGetFlow_OtherOUInRuntimeContext() {
	expectOwnOUOnly(s.authz, security.ActionReadFlow)
	s.storedFlow(authzOtherOU)

	flow, svcErr := s.service.GetFlow(security.WithRuntimeContext(context.Background()), authzFlowID)
	s.Require().Nil(svcErr)
	s.Equal(authzFlowID, flow.ID)
}

func (s *FlowAuthzTestSuite) TestDeleteFlow_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteFlow)
	s.storedFlow(authzOtherOU)

	svcErr := s.service.DeleteFlow(context.Background(), authzFlowID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeleteFlow", mock.Anything, mock.Anything)
}

func (s *FlowAuthzTestSuite) TestRestoreFlowVersion_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateFlow)
	s.storedFlow(authzOtherOU)

	_, svcErr := s.service.RestoreFlowVersion(context.Background(), authzFlowID, 7)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "GetFlowVersion", mock.Anything, mock.Anything, mock.Anything)
}

func (s *FlowAuthzTestSuite) TestListFlows_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListFlows)
	s.store.EXPECT().ListFlowsByOU(mock.Anything, authzOwnOU, defaultPageSize, 0, "").
		Return([]BasicFlowDefinition{{ID: "flow-own", OUID: authzOwnOU}}, 1, nil)

	resp, svcErr := s.service.ListFlows(context.Background(), 0, 0, "")
	s.Require().Nil(svcErr)
	s.Require().Len(resp.Flows, 1)
	s.True(*resp.Flows[0].IsOwnedByOU)
	s.store.AssertNotCalled(s.T(), "ListFlows", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.sharing.AssertNotCalled(s.T(), "ListVisibleResourceIDs", mock.Anything, mock.Anything, mock.Anything)
}

func (s *FlowAuthzTestSuite) TestListFlowsForOU_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionListFlows)
	resp, svcErr := s.service.ListFlowsForOU(context.Background(), authzOtherOU, "", 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(0, resp.TotalResults)
	s.store.AssertNotCalled(s.T(), "ListFlowsByOU", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (s *FlowAuthzTestSuite) TestSetVariables_ForOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateFlow)
	s.storedFlow(authzOwnOU)
	s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, authzFlowID, authzOtherOU).Return(true, nil)

	_, svcErr := s.service.SetVariables(context.Background(), authzFlowID, authzOtherOU, SetVariablesRequest{})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *FlowAuthzTestSuite) TestCreateSharingPolicy_InitiatedByOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateFlow)
	s.storedFlow(authzOwnOU)

	_, svcErr := s.service.CreateSharingPolicy(context.Background(), authzFlowID,
		sharing.PolicyRequest{InitiatingOUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}
