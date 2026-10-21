// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
)

type FederatedSyncTestSuite struct {
	suite.Suite
	entityProvider *entityprovidermock.EntityProviderInterfaceMock
	logger         *log.Logger
}

func TestFederatedSyncTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedSyncTestSuite))
}

func (s *FederatedSyncTestSuite) SetupTest() {
	s.entityProvider = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.logger = log.GetLogger()
}

func syncTestIDP() *providers.IDPDTO {
	return &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType: "Person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "mail", LocalAttribute: "email"},
				{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
				{ExternalAttribute: "id", LocalAttribute: "sub"},
				{ExternalAttribute: "phone", LocalAttribute: "mobile"},
			},
		}, {
			UserType: "Contractor",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "contractor_mail", LocalAttribute: "email"},
				{ExternalAttribute: "mail", LocalAttribute: "workEmail"},
			},
		}, {
			UserType:   "Guest",
			Attributes: []providers.AttributeMapping{{ExternalAttribute: "phone", LocalAttribute: "mobile"}},
		}},
		UserTypeResolution: &providers.UserTypeResolution{Default: "Person"},
	}}
}

var syncTestReceived = map[string]interface{}{
	"mail": "new@example.com", "email": "new@example.com", "given_name": "Fed", "firstName": "Fed",
	"id": "fed-sub", "sub": "fed-sub", "picture": "x", "contractor_mail": "contractor@example.com",
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_Attributes() {
	tests := []struct {
		name     string
		mode     providers.SyncMode
		expected map[string]map[string]interface{}
	}{
		{"no sync", providers.SyncModeNoSync, nil},
		{"session only", providers.SyncModeSessionOnly, nil},
		{"override received takes every user type's mapped attributes the provider sent, never sub",
			providers.SyncModeOverrideReceived,
			map[string]map[string]interface{}{
				"Person":     {"email": "new@example.com", "firstName": "Fed"},
				"Contractor": {"email": "contractor@example.com", "workEmail": "new@example.com"},
			}},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			sync := buildFederatedSync(syncTestIDP(), &idp.ResolvedAuthorizationMapping{
				AttributesSyncMode: tc.mode, GroupsAndRolesSyncMode: providers.SyncModeNoSync, Complete: true,
			}, syncTestReceived)
			s.Equal(tc.expected, sync.Entity.AttributesByType)
			s.Nil(sync.Entity.Groups)
			s.Nil(sync.Entity.Roles)
		})
	}
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_GroupsAndRoles() {
	targets := []providers.AuthorizationTarget{
		{Type: providers.AuthorizationTargetGroup, ID: "g1"},
		{Type: providers.AuthorizationTargetRole, ID: "r1"},
		{Type: providers.AuthorizationTargetPermission, Permission: "read"},
	}

	tests := []struct {
		name                 string
		mode                 providers.SyncMode
		mapsGroups, mapsRole bool
		targets              []providers.AuthorizationTarget
		groups, roles        *providers.MembershipSync
	}{
		{"no sync leaves both", providers.SyncModeNoSync, true, true, targets, nil, nil},
		{"session only leaves both", providers.SyncModeSessionOnly, true, true, targets, nil, nil},
		{"override all sets both", providers.SyncModeOverrideAll, true, true, targets,
			&providers.MembershipSync{IDs: []string{"g1"}}, &providers.MembershipSync{IDs: []string{"r1"}}},
		{"override all with nothing resolved clears both", providers.SyncModeOverrideAll, true, true, nil,
			&providers.MembershipSync{}, &providers.MembershipSync{}},
		{"override all leaves roles when the mapping can't resolve roles", providers.SyncModeOverrideAll,
			true, false, targets, &providers.MembershipSync{IDs: []string{"g1"}}, nil},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			sync := buildFederatedSync(syncTestIDP(), &idp.ResolvedAuthorizationMapping{
				AttributesSyncMode: providers.SyncModeNoSync, GroupsAndRolesSyncMode: tc.mode,
				MapsGroups: tc.mapsGroups, MapsRoles: tc.mapsRole, Targets: tc.targets, Complete: true,
			}, syncTestReceived)
			s.Equal(tc.groups, sync.Entity.Groups)
			s.Equal(tc.roles, sync.Entity.Roles)
			s.False(sync.MappingIncomplete)
		})
	}
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_KeepsAmbiguousMatches() {
	ambiguous := []providers.AuthorizationTarget{
		{Type: providers.AuthorizationTargetGroup, ID: "g2"},
		{Type: providers.AuthorizationTargetGroup, ID: "g3"},
		{Type: providers.AuthorizationTargetRole, ID: "r2"},
	}

	sync := buildFederatedSync(syncTestIDP(), &idp.ResolvedAuthorizationMapping{
		AttributesSyncMode: providers.SyncModeNoSync, GroupsAndRolesSyncMode: providers.SyncModeOverrideAll,
		MapsGroups: true, Ambiguous: ambiguous, Complete: true,
		Targets: []providers.AuthorizationTarget{{Type: providers.AuthorizationTargetGroup, ID: "g1"}},
	}, syncTestReceived)
	s.Equal(&providers.MembershipSync{IDs: []string{"g1"}, RetainIDs: []string{"g2", "g3"}}, sync.Entity.Groups)
	s.Nil(sync.Entity.Roles)

	sync = buildFederatedSync(syncTestIDP(), &idp.ResolvedAuthorizationMapping{
		AttributesSyncMode: providers.SyncModeNoSync, GroupsAndRolesSyncMode: providers.SyncModeNoSync,
		MapsGroups: true, MapsRoles: true, Ambiguous: ambiguous, Complete: true,
	}, syncTestReceived)
	s.Nil(sync.Entity.Groups)
	s.Nil(sync.Entity.Roles)
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_IncompleteMapping() {
	sync := buildFederatedSync(syncTestIDP(), &idp.ResolvedAuthorizationMapping{
		AttributesSyncMode: providers.SyncModeNoSync, GroupsAndRolesSyncMode: providers.SyncModeOverrideAll,
		MapsGroups: true,
	}, syncTestReceived)
	s.True(sync.MappingIncomplete)
	s.Nil(sync.Entity.Groups)
}

func syncTestContext() *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background(), RuntimeData: map[string]string{}}
}

func syncTestResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{RuntimeData: map[string]string{}}
}

var syncTestWrite = federatedSync{Entity: providers.FederatedEntitySync{
	AttributesByType: map[string]map[string]interface{}{"Person": {"email": "new@example.com"}},
	Groups:           &providers.MembershipSync{IDs: []string{"g1"}},
}}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_Writes() {
	s.entityProvider.On("SyncFederatedEntity", "user-1", syncTestWrite.Entity).Return(nil).Once()
	execResp := syncTestResponse()

	ok, err := syncFederatedUser(syncTestContext(), execResp, s.entityProvider, "user-1", syncTestWrite, s.logger)
	s.True(ok)
	s.NoError(err)
	s.Empty(execResp.Status)
}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_EmptySyncWritesNothing() {
	ok, err := syncFederatedUser(syncTestContext(), syncTestResponse(), s.entityProvider, "user-1",
		federatedSync{}, s.logger)
	s.True(ok)
	s.NoError(err)
	s.entityProvider.AssertNotCalled(s.T(), "SyncFederatedEntity", mock.Anything, mock.Anything)
}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_IncompleteMappingIsAServerError() {
	ok, err := syncFederatedUser(syncTestContext(), syncTestResponse(), s.entityProvider, "user-1",
		federatedSync{MappingIncomplete: true}, s.logger)
	s.False(ok)
	s.ErrorContains(err, "authorization mapping could not be fully resolved")
}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_RejectedWriteFailsSignIn() {
	s.entityProvider.On("SyncFederatedEntity", "user-1", mock.Anything).Return(
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSyncRejected, "Sync rejected", "denied")).Once()
	execResp := syncTestResponse()

	ok, err := syncFederatedUser(syncTestContext(), execResp, s.entityProvider, "user-1", syncTestWrite, s.logger)
	s.False(ok)
	s.NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(&ErrFederatedUserSyncFailed, execResp.Error)
}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_SystemErrorIsAServerError() {
	s.entityProvider.On("SyncFederatedEntity", "user-1", mock.Anything).Return(
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "System error", "db down")).Once()
	execResp := syncTestResponse()

	ok, err := syncFederatedUser(syncTestContext(), execResp, s.entityProvider, "user-1", syncTestWrite, s.logger)
	s.False(ok)
	s.ErrorContains(err, "db down")
	s.Empty(execResp.Status)
}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_NotImplementedSkipsSync() {
	s.entityProvider.On("SyncFederatedEntity", "user-1", mock.Anything).Return(
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "Not implemented", "")).Once()
	execResp := syncTestResponse()

	ok, err := syncFederatedUser(syncTestContext(), execResp, s.entityProvider, "user-1", syncTestWrite, s.logger)
	s.True(ok)
	s.NoError(err)
	s.Empty(execResp.Status)
	s.Nil(execResp.Error)
}

func (s *FederatedSyncTestSuite) TestSyncOrDeferFederatedUser_DefersUntilTheUserIsResolved() {
	execResp := syncTestResponse()
	ok, err := syncOrDeferFederatedUser(syncTestContext(), execResp, s.entityProvider, "", syncTestWrite, s.logger)
	s.True(ok)
	s.NoError(err)
	s.NotEmpty(execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])

	ctx := syncTestContext()
	ctx.RuntimeData[common.RuntimeKeyPendingFederatedSync] = execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync]
	s.entityProvider.On("SyncFederatedEntity", "user-2", syncTestWrite.Entity).Return(nil).Once()
	ok, err = syncPendingFederatedUser(ctx, syncTestResponse(), s.entityProvider, "user-2", s.logger)
	s.True(ok)
	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncOrDeferFederatedUser_EmptySyncIsNotKept() {
	execResp := syncTestResponse()
	ok, err := syncOrDeferFederatedUser(syncTestContext(), execResp, s.entityProvider, "", federatedSync{}, s.logger)
	s.True(ok)
	s.NoError(err)
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyPendingFederatedSync)
}

func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_NothingPending() {
	ok, err := syncPendingFederatedUser(syncTestContext(), syncTestResponse(), s.entityProvider, "user-1", s.logger)
	s.True(ok)
	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_InvalidPendingSync() {
	ctx := syncTestContext()
	ctx.RuntimeData[common.RuntimeKeyPendingFederatedSync] = "{"
	ok, err := syncPendingFederatedUser(ctx, syncTestResponse(), s.entityProvider, "user-1", s.logger)
	s.False(ok)
	s.Error(err)
}
