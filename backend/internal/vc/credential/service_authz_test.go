// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOU    = "ou-own"
	authzOtherOU  = "ou-other"
	authzConfigID = "cfg-authz"
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

// ConfigurationAuthzTestSuite covers the organization unit checks on the credential configuration
// management operations.
type ConfigurationAuthzTestSuite struct {
	suite.Suite
	authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store *credentialStoreInterfaceMock
	svc   *configurationService
}

func TestConfigurationAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationAuthzTestSuite))
}

func (s *ConfigurationAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.store = newCredentialStoreInterfaceMock(s.T())
	s.svc = newCredentialConfigurationService(s.store, nil, s.authz, nil, false, discardingBus{})
}

func (s *ConfigurationAuthzTestSuite) stored(ouID string) {
	s.store.On("GetCredentialConfigurationByID", mock.Anything, authzConfigID).
		Return(&CredentialConfigurationDTO{ID: authzConfigID, Handle: "cfg", VCT: "vct", OUID: ouID}, nil).Maybe()
}

func (s *ConfigurationAuthzTestSuite) TestGet_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadVC)
	s.stored(authzOtherOU)

	_, svcErr := s.svc.GetCredentialConfiguration(context.Background(), authzConfigID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ConfigurationAuthzTestSuite) TestUpdate_MoveToOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateVC)
	s.stored(authzOwnOU)

	_, svcErr := s.svc.UpdateCredentialConfiguration(context.Background(), authzConfigID,
		&CredentialConfigurationDTO{Handle: "cfg", VCT: "vct", OUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "UpdateCredentialConfiguration", mock.Anything, mock.Anything)
}

func (s *ConfigurationAuthzTestSuite) TestDelete_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteVC)
	s.stored(authzOtherOU)

	svcErr := s.svc.DeleteCredentialConfiguration(context.Background(), authzConfigID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeleteCredentialConfiguration", mock.Anything, mock.Anything)
}

func (s *ConfigurationAuthzTestSuite) TestListSummaries_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListVCs)
	s.store.On("ListCredentialConfigurationSummariesByOUID", mock.Anything, authzOwnOU).
		Return([]CredentialConfigurationList{
			{ID: "cfg-own", OUID: authzOwnOU},
		}, nil)

	summaries, svcErr := s.svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().Nil(svcErr)
	s.Require().Len(summaries, 1)
	s.Equal("cfg-own", summaries[0].ID)
	s.store.AssertNotCalled(s.T(), "ListCredentialConfigurationSummaries", mock.Anything)
}

func (s *ConfigurationAuthzTestSuite) TestGetByHandle_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadVC)
	s.store.On("GetCredentialConfigurationByHandle", mock.Anything, "cfg").
		Return(&CredentialConfigurationDTO{ID: authzConfigID, OUID: authzOtherOU}, nil)

	_, svcErr := s.svc.GetCredentialConfigurationByHandle(context.Background(), "cfg")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *ConfigurationAuthzTestSuite) TestGetByHandle_OtherOUInRuntimeContext() {
	expectOwnOUOnly(s.authz, security.ActionReadVC)
	s.store.On("GetCredentialConfigurationByHandle", mock.Anything, "cfg").
		Return(&CredentialConfigurationDTO{ID: authzConfigID, OUID: authzOtherOU}, nil)

	dto, svcErr := s.svc.GetCredentialConfigurationByHandle(security.WithRuntimeContext(context.Background()), "cfg")
	s.Require().Nil(svcErr)
	s.Equal(authzConfigID, dto.ID)
}

func (s *ConfigurationAuthzTestSuite) TestListSummaries_OwnAndParentOUs() {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListVCs, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU, "ou-parent"}}, nil)
	s.store.On("ListCredentialConfigurationSummaries", mock.Anything).
		Return([]CredentialConfigurationList{
			{ID: "cfg-own", OUID: authzOwnOU},
			{ID: "cfg-parent", OUID: "ou-parent"},
			{ID: "cfg-other", OUID: authzOtherOU},
		}, nil)

	summaries, svcErr := s.svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().Nil(svcErr)
	s.Require().Len(summaries, 2)
	s.Equal("cfg-own", summaries[0].ID)
	s.Equal("cfg-parent", summaries[1].ID)
}
