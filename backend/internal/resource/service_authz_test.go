// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOU   = "ou-own"
	authzOtherOU = "ou-other"
	authzRSID    = "rs-authz"
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

// ResourceAuthzTestSuite covers the organization unit checks on the resource server management operations.
type ResourceAuthzTestSuite struct {
	suite.Suite
	authz   *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store   *resourceStoreInterfaceMock
	service ResourceServiceInterface
}

func TestResourceAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceAuthzTestSuite))
}

func (s *ResourceAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	config.ResetServerRuntime()
	require.NoError(s.T(), config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)

	s.store = newResourceStoreInterfaceMock(s.T())
	var err error
	s.service, err = newResourceService(new(oumock.OrganizationUnitServiceInterfaceMock), s.store,
		&fakeTransactioner{}, nil, nil, nil, discardingBus{}, s.authz)
	s.Require().NoError(err)
}

// storedResourceServer stores the resource server in the given organization unit.
func (s *ResourceAuthzTestSuite) storedResourceServer(ouID string) {
	s.store.On("GetResourceServer", mock.Anything, authzRSID).
		Return(providers.ResourceServer{ID: authzRSID, Name: "Orders", Identifier: "https://orders.example.com",
			OUID: ouID, Delimiter: ":"}, nil).Maybe()
}

func (s *ResourceAuthzTestSuite) TestGetResourceServer_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)
	s.storedResourceServer(authzOtherOU)

	rs, svcErr := s.service.GetResourceServer(context.Background(), authzRSID)
	s.Nil(rs)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ResourceAuthzTestSuite) TestGetResourceServer_OtherOUInRuntimeContext() {
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)
	s.storedResourceServer(authzOtherOU)

	rs, svcErr := s.service.GetResourceServer(security.WithRuntimeContext(context.Background()), authzRSID)
	s.Require().Nil(svcErr)
	s.Equal(authzRSID, rs.ID)
}

func (s *ResourceAuthzTestSuite) TestDeleteResourceServer_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteResourceServer)
	s.storedResourceServer(authzOtherOU)
	s.store.On("IsResourceServerDeclarative", authzRSID).Return(false).Maybe()

	svcErr := s.service.DeleteResourceServer(context.Background(), authzRSID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeleteResourceServer", mock.Anything, mock.Anything)
}

func (s *ResourceAuthzTestSuite) TestCreateResource_UnderOtherOUServer() {
	expectOwnOUOnly(s.authz, security.ActionUpdateResourceServer)
	s.storedResourceServer(authzOtherOU)

	_, svcErr := s.service.CreateResource(context.Background(), authzRSID,
		providers.Resource{Name: "Orders", Handle: "orders"})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "CreateResource", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (s *ResourceAuthzTestSuite) TestGetResourceServerList_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListResourceServers)
	s.store.On("GetResourceServerListCountByOUID", mock.Anything, authzOwnOU).Return(1, nil)
	s.store.On("GetResourceServerListByOUID", mock.Anything, authzOwnOU, 10, 0).
		Return([]providers.ResourceServer{{ID: "rs-own", OUID: authzOwnOU}}, nil)

	list, svcErr := s.service.GetResourceServerList(context.Background(), 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(1, list.TotalResults)
	s.store.AssertNotCalled(s.T(), "GetResourceServerList", mock.Anything, mock.Anything, mock.Anything)
}

// A caller may list the resource servers of its own organization unit and of the ones above it, in
// that order, so a page can span both.
func (s *ResourceAuthzTestSuite) TestGetResourceServerList_OwnThenParentOU() {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListResourceServers,
		security.ResourceTypeOU).Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU, "ou-parent"}}, nil)
	s.store.On("GetResourceServerListCountByOUID", mock.Anything, authzOwnOU).Return(2, nil)
	s.store.On("GetResourceServerListCountByOUID", mock.Anything, "ou-parent").Return(2, nil)
	s.store.On("GetResourceServerListByOUID", mock.Anything, authzOwnOU, 2, 1).
		Return([]providers.ResourceServer{{ID: "rs-own-2", OUID: authzOwnOU}}, nil)
	s.store.On("GetResourceServerListByOUID", mock.Anything, "ou-parent", 1, 0).
		Return([]providers.ResourceServer{{ID: "rs-parent-1", OUID: "ou-parent"}}, nil)

	list, svcErr := s.service.GetResourceServerList(context.Background(), 2, 1)
	s.Require().Nil(svcErr)
	s.Equal(4, list.TotalResults)
	s.Require().Len(list.ResourceServers, 2)
	s.Equal("rs-own-2", list.ResourceServers[0].ID)
	s.Equal("rs-parent-1", list.ResourceServers[1].ID)
}

func (s *ResourceAuthzTestSuite) TestGetResourceServerListByOUID_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionListResourceServers)
	list, svcErr := s.service.GetResourceServerListByOUID(context.Background(), authzOtherOU, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(0, list.TotalResults)
	s.store.AssertNotCalled(s.T(), "GetResourceServerListByOUID", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (s *ResourceAuthzTestSuite) TestUpdateResourceServer_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateResourceServer)
	s.storedResourceServer(authzOtherOU)

	_, svcErr := s.service.UpdateResourceServer(context.Background(), authzRSID, providers.ResourceServer{
		Name: "Orders", Identifier: "https://orders.example.com"})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "UpdateResourceServer", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ResourceAuthzTestSuite) TestUpdateResourceServer_MoveToOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateResourceServer)
	s.storedResourceServer(authzOwnOU)
	s.store.On("IsResourceServerDeclarative", authzRSID).Return(false).Maybe()

	_, svcErr := s.service.UpdateResourceServer(context.Background(), authzRSID, providers.ResourceServer{
		Name: "Orders", Identifier: "https://orders.example.com", OUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "UpdateResourceServer", mock.Anything, mock.Anything, mock.Anything)
}

// Deleting an action is authorized as updating its resource server; reading the action needs no view check.
func (s *ResourceAuthzTestSuite) TestValidateDeleteAction_ReadsActionWithoutViewCheck() {
	expectOwnOUOnly(s.authz, security.ActionUpdateResourceServer)
	s.storedResourceServer(authzOwnOU)
	s.store.On("IsResourceServerDeclarative", authzRSID).Return(false)
	s.store.On("GetAction", mock.Anything, "act-1", authzRSID, (*string)(nil)).
		Return(providers.Action{ID: "act-1", Permission: "orders:read"}, nil)

	target, svcErr := s.service.ValidateDeleteAction(context.Background(), authzRSID, nil, "act-1")
	s.Require().Nil(svcErr)
	s.Require().Len(target.Scopes, 1)
	s.Equal("orders:read", target.Scopes[0].Scope)
	s.authz.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, security.ActionReadResourceServer, mock.Anything)
}
