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
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
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
	m.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
}

// ResourceAuthzTestSuite covers the organization unit checks on the resource server management operations.
type ResourceAuthzTestSuite struct {
	suite.Suite
	authz   *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store   *resourceStoreInterfaceMock
	sharing *sharingmock.SharingServiceInterfaceMock
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
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	var err error
	s.service, err = newResourceService(new(oumock.OrganizationUnitServiceInterfaceMock), s.store,
		&fakeTransactioner{}, nil, nil, nil, discardingBus{}, s.authz, s.sharing)
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

func (s *ResourceAuthzTestSuite) TestGetDeclaredPermissions_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)
	s.storedResourceServer(authzOtherOU)

	permissions, svcErr := s.service.GetDeclaredPermissions(context.Background(), authzRSID, []string{"reports"})

	s.Nil(permissions)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ResourceAuthzTestSuite) TestGetDeclaredPermissions_OtherOUInRuntimeContext() {
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)
	s.storedResourceServer(authzOtherOU)
	s.store.On("GetDeclaredPermissions", mock.Anything, authzRSID, []string{"reports"}, "").
		Return([]string{"reports"}, nil).Once()

	permissions, svcErr := s.service.GetDeclaredPermissions(security.WithRuntimeContext(context.Background()),
		authzRSID, []string{"reports"})

	s.Require().Nil(svcErr)
	s.Equal([]string{"reports"}, permissions)
}

// confinedToOwnOU makes the caller one confined to its own organization unit, with the resource server stored in
// another one and shared into the caller's unit or not.
func (s *ResourceAuthzTestSuite) confinedToOwnOU(shared bool, svcErr *tidcommon.ServiceError) {
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU}).Maybe()
	s.storedResourceServer(authzOtherOU)
	s.sharing.EXPECT().IsVisible(mock.Anything, ResourceServerSharingType, authzRSID, authzOwnOU).
		Return(shared, svcErr).Maybe()
}

// A caller confined to an organization unit a resource server is shared with reads it there, read only.
func (s *ResourceAuthzTestSuite) TestGetResourceServer_SharedIntoCallerOU() {
	s.confinedToOwnOU(true, nil)
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)

	rs, svcErr := s.service.GetResourceServer(context.Background(), authzRSID)

	s.Require().Nil(svcErr)
	s.Equal(authzOwnOU, rs.OUID)
	s.True(rs.IsReadOnly)
	s.Equal(new(false), rs.IsOwnedByOU)
}

// A caller who may not read resource servers in its own organization unit does not read one shared into it.
func (s *ResourceAuthzTestSuite) TestGetResourceServer_SharedIntoCallerOUWithoutReadThere() {
	s.confinedToOwnOU(true, nil)
	s.authz.On("IsActionAllowed", mock.Anything, security.ActionReadResourceServer, mock.Anything).
		Return(false, nil)

	rs, svcErr := s.service.GetResourceServer(context.Background(), authzRSID)

	s.Nil(rs)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ResourceAuthzTestSuite) TestGetResourceServer_NotSharedIntoCallerOU() {
	s.confinedToOwnOU(false, nil)
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)

	rs, svcErr := s.service.GetResourceServer(context.Background(), authzRSID)

	s.Nil(rs)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ResourceAuthzTestSuite) TestGetResourceServer_SharingLookupFails() {
	s.confinedToOwnOU(false, &tidcommon.InternalServerError)
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)

	_, svcErr := s.service.GetResourceServer(context.Background(), authzRSID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// The resources and actions of a resource server shared into the caller's organization unit are readable there.
func (s *ResourceAuthzTestSuite) TestGetDeclaredPermissions_SharedIntoCallerOU() {
	s.confinedToOwnOU(true, nil)
	expectOwnOUOnly(s.authz, security.ActionReadResourceServer)
	s.store.On("GetDeclaredPermissions", mock.Anything, authzRSID, []string{"reports"}, "").
		Return([]string{"reports"}, nil).Once()

	permissions, svcErr := s.service.GetDeclaredPermissions(context.Background(), authzRSID, []string{"reports"})

	s.Require().Nil(svcErr)
	s.Equal([]string{"reports"}, permissions)
}

// A resource server shared into the caller's organization unit cannot be changed there.
func (s *ResourceAuthzTestSuite) TestCreateResource_UnderServerSharedIntoCallerOU() {
	s.confinedToOwnOU(true, nil)
	expectOwnOUOnly(s.authz, security.ActionUpdateResourceServer)

	_, svcErr := s.service.CreateResource(context.Background(), authzRSID,
		providers.Resource{Name: "Orders", Handle: "orders"})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "CreateResource", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
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
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, ResourceServerSharingType, authzOwnOU).
		Return(nil, nil)
	s.store.On("GetResourceServerListCountByOUID", mock.Anything, authzOwnOU).Return(1, nil)
	s.store.On("GetResourceServerListByOUID", mock.Anything, authzOwnOU, 1, 0).
		Return([]providers.ResourceServer{{ID: "rs-own", OUID: authzOwnOU}}, nil)

	list, svcErr := s.service.GetResourceServerList(context.Background(), 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(1, list.TotalResults)
	s.store.AssertNotCalled(s.T(), "GetResourceServerList", mock.Anything, mock.Anything, mock.Anything)
}

// A caller whose list scope sees no organization unit gets an empty list, without reading the store or the sharing
// policies.
func (s *ResourceAuthzTestSuite) TestGetResourceServerList_ScopeSeesNothing() {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListResourceServers, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{}, nil)

	list, svcErr := s.service.GetResourceServerList(context.Background(), 10, 0)

	s.Require().Nil(svcErr)
	s.Equal(0, list.TotalResults)
	s.Empty(list.ResourceServers)
	s.store.AssertNotCalled(s.T(), "GetResourceServerList", mock.Anything, mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "GetResourceServerListCountByOUID", mock.Anything, mock.Anything)
	s.sharing.AssertNotCalled(s.T(), "ListVisibleResourceIDs", mock.Anything, mock.Anything, mock.Anything)
}

// A caller confined to an organization unit lists the resource servers it owns, then the ones shared with it, and
// none of the other resource servers of the organization units above it.
func (s *ResourceAuthzTestSuite) TestGetResourceServerList_OwnThenShared() {
	expectOwnOUOnly(s.authz, security.ActionListResourceServers)
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, ResourceServerSharingType, authzOwnOU).
		Return([]string{"rs-shared"}, nil)
	s.store.On("GetResourceServerListCountByOUID", mock.Anything, authzOwnOU).Return(2, nil)
	s.store.On("GetResourceServerListByOUID", mock.Anything, authzOwnOU, 1, 1).
		Return([]providers.ResourceServer{{ID: "rs-own-2", OUID: authzOwnOU}}, nil)
	s.store.On("GetResourceServersByIDs", mock.Anything, []string{"rs-shared"}).
		Return([]providers.ResourceServer{{ID: "rs-shared", OUID: "ou-parent"}}, nil)

	list, svcErr := s.service.GetResourceServerList(context.Background(), 2, 1)
	s.Require().Nil(svcErr)
	s.Equal(3, list.TotalResults)
	s.Require().Len(list.ResourceServers, 2)
	s.Equal("rs-own-2", list.ResourceServers[0].ID)
	s.Equal("rs-shared", list.ResourceServers[1].ID)
	s.Equal(authzOwnOU, list.ResourceServers[1].OUID, "a shared resource server is listed under the caller's unit")
	s.Equal(new(false), list.ResourceServers[1].IsOwnedByOU)
	for _, link := range list.Links {
		s.NotContains(link.Href, "filter", "an unfiltered list's links name no filter")
	}
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
