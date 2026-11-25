// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
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

func (s *FlowAuthzTestSuite) TestGetFlowView_Owned() {
	expectOwnOUOnly(s.authz, security.ActionReadFlow)
	s.storedFlow(authzOwnOU)

	view, svcErr := s.service.GetFlowView(context.Background(), authzFlowID)
	s.Require().Nil(svcErr)
	s.Require().NotNil(view.Owned)
	s.Nil(view.Shared)
	s.Equal(authzFlowID, view.Owned.ID)
	s.True(view.Owned.IsOwnedByOU)
}

// A caller refused in the organization unit that owns the flow reads it as the unit it is confined to sees it, when
// the flow is shared there, and stays refused otherwise.
func (s *FlowAuthzTestSuite) TestGetFlowView_ForTheCallersUnit() {
	for _, shared := range []bool{true, false} {
		s.Run(fmt.Sprintf("shared %t", shared), func() {
			s.SetupTest()
			expectOwnOUOnly(s.authz, security.ActionReadFlow)
			s.authz.On("GetBoundary", mock.Anything).
				Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
			s.storedFlow(authzOtherOU)
			s.sharing.EXPECT().IsVisible(mock.Anything, FlowSharingType, authzFlowID, authzOwnOU).Return(shared, nil)

			view, svcErr := s.service.GetFlowView(context.Background(), authzFlowID)
			if !shared {
				s.Nil(view)
				s.Require().NotNil(svcErr)
				s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
				return
			}
			s.Require().Nil(svcErr)
			s.Require().NotNil(view.Shared)
			s.Nil(view.Owned)
			s.Equal(authzOwnOU, view.Shared.OUID)
			s.False(view.Shared.IsOwnedByOU)
			s.True(view.Shared.IsReadOnly)
		})
	}
}

func (s *FlowAuthzTestSuite) TestGetFlowView_RefusedCallerNotConfinedToAUnit() {
	expectOwnOUOnly(s.authz, security.ActionReadFlow)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})
	s.storedFlow(authzOtherOU)

	view, svcErr := s.service.GetFlowView(context.Background(), authzFlowID)
	s.Nil(view)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.sharing.AssertNotCalled(s.T(), "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *FlowAuthzTestSuite) TestDeleteFlow_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteFlow)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
	s.storedFlow(authzOtherOU)

	svcErr := s.service.DeleteFlow(context.Background(), authzFlowID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeleteFlow", mock.Anything, mock.Anything)
}

// A flow shared into the organization unit the caller is confined to is read only to it, and one that is not stays
// refused as any flow of another unit is.
func (s *FlowAuthzTestSuite) TestSharedFlowIsReadOnlyToTheCallersUnit() {
	writes := map[security.Action]func(service *flowMgtService) *tidcommon.ServiceError{
		security.ActionUpdateFlow: func(service *flowMgtService) *tidcommon.ServiceError {
			_, svcErr := service.UpdateFlow(context.Background(), authzFlowID, &FlowDefinition{})
			return svcErr
		},
		security.ActionDeleteFlow: func(service *flowMgtService) *tidcommon.ServiceError {
			return service.DeleteFlow(context.Background(), authzFlowID)
		},
	}
	for action, write := range writes {
		for _, shared := range []bool{true, false} {
			s.Run(fmt.Sprintf("%s shared %t", action, shared), func() {
				s.SetupTest()
				validator := NewFlowValidatorInterfaceMock(s.T())
				validator.EXPECT().ValidateFlowDefinition(mock.Anything, mock.Anything).Return(nil).Maybe()
				s.service.flowValidator = validator
				expectOwnOUOnly(s.authz, action)
				s.authz.On("GetBoundary", mock.Anything).
					Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
				s.storedFlow(authzOtherOU)
				s.sharing.EXPECT().IsVisible(mock.MatchedBy(security.IsRuntimeContext), FlowSharingType,
					authzFlowID, authzOwnOU).Return(shared, nil)

				svcErr := write(s.service)

				s.Require().NotNil(svcErr)
				if shared {
					s.Equal(ErrorSharedFlowReadOnly.Code, svcErr.Code)
				} else {
					s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
				}
				s.store.AssertNotCalled(s.T(), "UpdateFlow", mock.Anything, mock.Anything, mock.Anything)
				s.store.AssertNotCalled(s.T(), "DeleteFlow", mock.Anything, mock.Anything)
			})
		}
	}
}

// A token naming an organization unit that no longer exists has nothing shared into it, so a change is refused
// rather than answered with a server error.
func (s *FlowAuthzTestSuite) TestSharedFlowRefusalWhenTheTokensUnitIsGone() {
	expectOwnOUOnly(s.authz, security.ActionDeleteFlow)
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
	s.storedFlow(authzOtherOU)
	s.sharing.EXPECT().IsVisible(mock.MatchedBy(security.IsRuntimeContext), FlowSharingType, authzFlowID,
		authzOwnOU).Return(false, &ou.ErrorOrganizationUnitNotFound)

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
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, FlowSharingType, authzOwnOU).
		Return([]string{"flow-shared"}, nil)
	s.store.EXPECT().ListFlowsByIDs(mock.Anything, []string{"flow-shared"}, "").
		Return([]BasicFlowDefinition{{ID: "flow-shared", OUID: authzOtherOU}}, nil)

	resp, svcErr := s.service.ListFlows(context.Background(), 0, 0, "")
	s.Require().Nil(svcErr)
	s.Require().Len(resp.Flows, 2)
	s.True(*resp.Flows[0].IsOwnedByOU)
	s.False(*resp.Flows[1].IsOwnedByOU)
	s.Equal(authzOwnOU, resp.Flows[1].OUID, "a shared flow is listed under the unit it is shared into")
	s.store.AssertNotCalled(s.T(), "ListFlows", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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
