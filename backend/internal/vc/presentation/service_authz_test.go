// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

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
	authzConfigID = "pd-authz"
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

// DefinitionAuthzTestSuite covers the organization unit checks on the presentation definition
// management operations.
type DefinitionAuthzTestSuite struct {
	suite.Suite
	authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store *definitionStoreInterfaceMock
	svc   *definitionService
}

func TestDefinitionAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionAuthzTestSuite))
}

func (s *DefinitionAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.store = newDefinitionStoreInterfaceMock(s.T())
	s.svc = newPresentationDefinitionService(s.store, nil, s.authz, nil, false, discardingBus{})
}

func (s *DefinitionAuthzTestSuite) stored(ouID string) {
	s.store.On("GetPresentationDefinitionByID", mock.Anything, authzConfigID).
		Return(&PresentationDefinitionDTO{ID: authzConfigID, Handle: "cfg", VCT: "vct", OUID: ouID}, nil).Maybe()
}

func (s *DefinitionAuthzTestSuite) TestGet_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadVC)
	s.stored(authzOtherOU)

	_, svcErr := s.svc.GetPresentationDefinition(context.Background(), authzConfigID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *DefinitionAuthzTestSuite) TestUpdate_MoveToOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateVC)
	s.stored(authzOwnOU)

	_, svcErr := s.svc.UpdatePresentationDefinition(context.Background(), authzConfigID,
		&PresentationDefinitionDTO{Handle: "cfg", VCT: "vct", OUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "UpdatePresentationDefinition", mock.Anything, mock.Anything)
}

func (s *DefinitionAuthzTestSuite) TestDelete_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionDeleteVC)
	s.stored(authzOtherOU)

	svcErr := s.svc.DeletePresentationDefinition(context.Background(), authzConfigID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeletePresentationDefinition", mock.Anything, mock.Anything)
}

func (s *DefinitionAuthzTestSuite) TestListSummaries_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListVCs)
	s.store.On("ListPresentationDefinitionSummariesByOUID", mock.Anything, authzOwnOU).
		Return([]PresentationDefinitionList{
			{ID: "cfg-own", OUID: authzOwnOU},
		}, nil)

	summaries, svcErr := s.svc.ListPresentationDefinitionSummaries(context.Background())
	s.Require().Nil(svcErr)
	s.Require().Len(summaries, 1)
	s.Equal("cfg-own", summaries[0].ID)
	s.store.AssertNotCalled(s.T(), "ListPresentationDefinitionSummaries", mock.Anything)
}

func (s *DefinitionAuthzTestSuite) TestGetByHandle_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadVC)
	s.store.On("GetPresentationDefinitionByHandle", mock.Anything, "cfg").
		Return(&PresentationDefinitionDTO{ID: authzConfigID, OUID: authzOtherOU}, nil)

	_, svcErr := s.svc.GetPresentationDefinitionByHandle(context.Background(), "cfg")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *DefinitionAuthzTestSuite) TestGetByHandle_OtherOUInRuntimeContext() {
	expectOwnOUOnly(s.authz, security.ActionReadVC)
	s.store.On("GetPresentationDefinitionByHandle", mock.Anything, "cfg").
		Return(&PresentationDefinitionDTO{ID: authzConfigID, OUID: authzOtherOU}, nil)

	dto, svcErr := s.svc.GetPresentationDefinitionByHandle(security.WithRuntimeContext(context.Background()), "cfg")
	s.Require().Nil(svcErr)
	s.Equal(authzConfigID, dto.ID)
}

func (s *DefinitionAuthzTestSuite) TestListSummaries_OwnAndParentOUs() {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListVCs, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU, "ou-parent"}}, nil)
	s.store.On("ListPresentationDefinitionSummaries", mock.Anything).
		Return([]PresentationDefinitionList{
			{ID: "cfg-own", OUID: authzOwnOU},
			{ID: "cfg-parent", OUID: "ou-parent"},
			{ID: "cfg-other", OUID: authzOtherOU},
		}, nil)

	summaries, svcErr := s.svc.ListPresentationDefinitionSummaries(context.Background())
	s.Require().Nil(svcErr)
	s.Require().Len(summaries, 2)
	s.Equal("cfg-own", summaries[0].ID)
	s.Equal("cfg-parent", summaries[1].ID)
}
