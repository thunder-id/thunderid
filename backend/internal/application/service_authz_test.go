// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOU   = "ou-own"
	authzOtherOU = "ou-other"
	authzAppID   = "app-authz"
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

// ApplicationAuthzTestSuite covers the organization unit checks on the application management operations.
type ApplicationAuthzTestSuite struct {
	suite.Suite
	authz    *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	inbound  *inboundclientmock.InboundClientServiceInterfaceMock
	ous      *oumock.OrganizationUnitServiceInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
	sharing  *sharingmock.SharingServiceInterfaceMock
	svc      *applicationService
}

func TestApplicationAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationAuthzTestSuite))
}

func (s *ApplicationAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.authz.On("CanControlEntity", mock.Anything, mock.Anything).Return(nil).Maybe()
	s.inbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
	governance.EXPECT().ReadEntityAccess(mock.Anything, mock.Anything).Return(identitygovernance.AccessRead{
		Status: providers.AccessStatusReport{Value: providers.AccessStatusActive},
	}).Maybe()
	s.svc = &applicationService{
		logger:               log.GetLogger(),
		inboundClientService: s.inbound,
		entityService:        s.entities,
		ouService:            s.ous,
		sharingService:       s.sharing,
		governance:           governance,
		authzService:         s.authz,
		bus:                  discardingBus{},
	}
}

// storedApplication makes the application readable as one the given organization unit owns.
func (s *ApplicationAuthzTestSuite) storedApplication(ouID string) {
	s.inbound.On("GetInboundClientByEntityID", mock.Anything, authzAppID).
		Return(&inboundmodel.InboundClient{ID: authzAppID}, nil).Maybe()
	s.inbound.On("GetOAuthProfileByEntityID", mock.Anything, authzAppID).
		Return((*providers.OAuthProfile)(nil), nil).Maybe()
	entity := &providers.Entity{ID: authzAppID, Category: providers.EntityCategoryApp, OUID: ouID}
	s.entities.On("GetEntity", mock.Anything, authzAppID).Return(entity, nil).Maybe()
	s.entities.On("GetEntityProfile", mock.Anything, authzAppID).Return(entity, nil).Maybe()
}

func (s *ApplicationAuthzTestSuite) TestGetApplication_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
	s.storedApplication(authzOtherOU)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Nil(app)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ApplicationAuthzTestSuite) TestDeleteApplication_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteApplication)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
	s.storedApplication(authzOtherOU)

	svcErr := s.svc.DeleteApplication(context.Background(), authzAppID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.entities.AssertNotCalled(s.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

func (s *ApplicationAuthzTestSuite) TestCreateSharingPolicy_InitiatedByOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateApplication)
	s.storedApplication(authzOwnOU)

	_, svcErr := s.svc.CreateSharingPolicy(context.Background(), authzAppID,
		sharing.PolicyRequest{InitiatingOUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.sharing.AssertNotCalled(s.T(), "CreatePolicy", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (s *ApplicationAuthzTestSuite) TestListSharingPolicies_OnlyOwnInitiated() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.storedApplication(authzOtherOU)
	s.sharing.On("ListPolicies", mock.Anything, ApplicationSharingType, authzAppID).Return([]sharing.Policy{
		{ID: "p-other", InitiatingOUID: authzOtherOU},
		{ID: "p-own", InitiatingOUID: authzOwnOU},
	}, nil)

	list, svcErr := s.svc.ListSharingPolicies(context.Background(), authzAppID, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(1, list.TotalResults)
	s.Require().Len(list.Policies, 1)
	s.Equal("p-own", list.Policies[0].ID)
}

func (s *ApplicationAuthzTestSuite) TestGetApplicationList_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListApplications)
	s.entities.On("GetEntityPageInOU", mock.Anything, providers.EntityCategoryApp, authzOwnOU,
		entity.OUEntityScopeAll, mock.Anything, 0, mock.Anything).Return(&entity.OUEntityPage{}, nil)

	resp, svcErr := s.svc.GetApplicationList(context.Background(), nil)
	s.Require().Nil(svcErr)
	s.Equal(0, resp.TotalResults)
	s.entities.AssertNotCalled(s.T(), "GetEntityList", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

// updateRequest is an update of the stored application that names the given organization unit.
func (s *ApplicationAuthzTestSuite) updateRequest(ouID string) *model.ApplicationDTO {
	s.entities.On("IsEntityDeclarative", mock.Anything, authzAppID).Return(false, nil).Maybe()
	s.entities.On("IdentifyEntity", mock.Anything, mock.Anything).Return((*string)(nil), entity.ErrEntityNotFound).
		Maybe()
	s.inbound.On("ResolveInboundAuthProfileHandles", mock.Anything, mock.Anything).Return(nil).Maybe()
	s.ous.On("IsOrganizationUnitExists", mock.Anything, mock.Anything).Return(true, nil).Maybe()
	return &model.ApplicationDTO{Name: "app", OUID: ouID}
}

func (s *ApplicationAuthzTestSuite) TestUpdateApplication_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateApplication)
	s.authz.On("GetBoundary", mock.Anything).Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment}).Maybe()
	s.storedApplication(authzOtherOU)

	_, svcErr := s.svc.UpdateApplication(context.Background(), authzAppID, s.updateRequest(""))
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.inbound.AssertNotCalled(s.T(), "UpdateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (s *ApplicationAuthzTestSuite) TestUpdateApplication_MoveToOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateApplication)
	s.storedApplication(authzOwnOU)

	_, svcErr := s.svc.UpdateApplication(context.Background(), authzAppID, s.updateRequest(authzOtherOU))
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.inbound.AssertNotCalled(s.T(), "UpdateInboundClient", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

// confinedToOwnOU confines the caller to its own organization unit, into which the stored application is
// shared or not.
func (s *ApplicationAuthzTestSuite) confinedToOwnOU(shared bool) {
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU}).Maybe()
	s.sharing.On("IsVisible", mock.Anything, ApplicationSharingType, authzAppID, authzOwnOU).
		Return(shared, nil)
}

func (s *ApplicationAuthzTestSuite) TestGetApplication_OwnOU() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.storedApplication(authzOwnOU)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Require().Nil(svcErr)
	s.Equal(authzOwnOU, app.OUID)
	s.Require().NotNil(app.IsOwnedByOU)
	s.True(*app.IsOwnedByOU)
	s.False(app.IsReadOnly)
}

func (s *ApplicationAuthzTestSuite) TestGetApplication_SharedIntoOwnOU() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.storedApplication(authzOtherOU)
	s.confinedToOwnOU(true)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Require().Nil(svcErr)
	s.Equal(authzOwnOU, app.OUID, "a shared application is read under the unit it is shared into")
	s.Require().NotNil(app.IsOwnedByOU)
	s.False(*app.IsOwnedByOU)
	s.True(app.IsReadOnly)
}

func (s *ApplicationAuthzTestSuite) TestGetApplication_NotSharedIntoOwnOU() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.storedApplication(authzOtherOU)
	s.confinedToOwnOU(false)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Nil(app)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ApplicationAuthzTestSuite) TestGetApplication_SharingCheckFails() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.storedApplication(authzOtherOU)
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
	s.sharing.On("IsVisible", mock.Anything, ApplicationSharingType, authzAppID, authzOwnOU).
		Return(false, &tidcommon.InternalServerError)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Nil(app)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A shared application is read only by a caller that may read applications in the organization unit it is shared
// into.
func (s *ApplicationAuthzTestSuite) TestGetApplication_SharedButNotReadableInOwnOU() {
	s.authz.On("IsActionAllowed", mock.Anything, security.ActionReadApplication, mock.Anything).Return(false, nil)
	s.storedApplication(authzOtherOU)
	s.confinedToOwnOU(true)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Nil(app)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

// A token naming an organization unit that no longer exists has nothing shared into it, so it is refused rather than
// answered with a server error.
func (s *ApplicationAuthzTestSuite) TestGetApplication_TheTokensUnitIsGone() {
	expectOwnOUOnly(s.authz, security.ActionReadApplication)
	s.storedApplication(authzOtherOU)
	s.authz.On("GetBoundary", mock.Anything).
		Return(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
	s.sharing.On("IsVisible", mock.Anything, ApplicationSharingType, authzAppID, authzOwnOU).
		Return(false, &oupkg.ErrorOrganizationUnitNotFound)

	app, svcErr := s.svc.GetApplication(context.Background(), authzAppID)
	s.Nil(app)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ApplicationAuthzTestSuite) TestWritesToASharedApplicationAreReadOnly() {
	writes := map[string]struct {
		action security.Action
		write  func() *tidcommon.ServiceError
	}{
		"update": {action: security.ActionUpdateApplication, write: func() *tidcommon.ServiceError {
			_, svcErr := s.svc.UpdateApplication(context.Background(), authzAppID, s.updateRequest(""))
			return svcErr
		}},
		"delete": {action: security.ActionDeleteApplication, write: func() *tidcommon.ServiceError {
			return s.svc.DeleteApplication(context.Background(), authzAppID)
		}},
	}
	for name, tc := range writes {
		for _, shared := range []bool{true, false} {
			s.Run(fmt.Sprintf("%s shared=%t", name, shared), func() {
				s.SetupTest()
				expectOwnOUOnly(s.authz, tc.action)
				s.storedApplication(authzOtherOU)
				s.confinedToOwnOU(shared)

				svcErr := tc.write()

				s.Require().NotNil(svcErr)
				want := tidcommon.ErrorUnauthorized.Code
				if shared {
					want = ErrorSharedApplicationReadOnly.Code
				}
				s.Equal(want, svcErr.Code)
				s.inbound.AssertNotCalled(s.T(), "UpdateInboundClient", mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything)
				s.entities.AssertNotCalled(s.T(), "DeleteEntity", mock.Anything, mock.Anything)
			})
		}
	}
}
