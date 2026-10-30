// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
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

func syncTestIDP(attributes, groupsAndRoles providers.SyncMode) *providers.IDPDTO {
	return &providers.IDPDTO{ID: "idp-1", AttributeConfiguration: &providers.AttributeConfiguration{
		Sync: &providers.SyncConfiguration{Attributes: attributes, GroupsAndRoles: groupsAndRoles},
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
	}}
}

var syncTestReceived = map[string]interface{}{
	"mail": "new@example.com", "email": "new@example.com", "given_name": "Fed", "firstName": "Fed",
	"id": "fed-sub", "sub": "fed-sub", "picture": "x", "contractor_mail": "contractor@example.com",
}

var syncTestLink = providers.LinkedAccount{IdpID: "idp-1", Sub: "fed-sub"}

// Neither mode overriding the account means nothing is synced, not even the link's sync time.
func (s *FederatedSyncTestSuite) TestBuildFederatedSync_NoOverride() {
	for _, mode := range []providers.SyncMode{providers.SyncModeNoSync, providers.SyncModeSessionOnly} {
		s.Nil(buildFederatedSync(syncTestIDP(mode, mode), &idp.ResolvedAuthorizationMapping{Complete: true},
			syncTestReceived, "fed-sub"))
	}
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_Attributes() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeOverrideReceived, providers.SyncModeNoSync),
		&idp.ResolvedAuthorizationMapping{Complete: true}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.Equal(syncTestLink, sync.Entity.Link)
	s.Equal(map[string]map[string]interface{}{
		"Person":     {"email": "new@example.com", "firstName": "Fed"},
		"Contractor": {"email": "contractor@example.com", "workEmail": "new@example.com"},
	}, sync.Entity.AttributesByType)
	s.Nil(sync.Entity.Groups)
	s.Nil(sync.Entity.Roles)
}

// Groups and roles alone still sync, to stamp the link, without writing any attribute.
func (s *FederatedSyncTestSuite) TestBuildFederatedSync_GroupsAndRolesWithoutAttributes() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll),
		&idp.ResolvedAuthorizationMapping{Complete: true}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.Equal(syncTestLink, sync.Entity.Link)
	s.Nil(sync.Entity.AttributesByType)
	s.Nil(sync.Entity.Groups)
	s.Nil(sync.Entity.Roles)
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
		{"override all sets both", providers.SyncModeOverrideAll, true, true, targets,
			&providers.MembershipSync{IDs: []string{"g1"}}, &providers.MembershipSync{IDs: []string{"r1"}}},
		{"override all with nothing resolved clears both", providers.SyncModeOverrideAll, true, true, nil,
			&providers.MembershipSync{}, &providers.MembershipSync{}},
		{"override all leaves roles when the mapping can't resolve roles", providers.SyncModeOverrideAll,
			true, false, targets, &providers.MembershipSync{IDs: []string{"g1"}}, nil},
		{"no sync leaves both", providers.SyncModeNoSync, true, true, targets, nil, nil},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			sync := buildFederatedSync(syncTestIDP(providers.SyncModeOverrideReceived, tc.mode),
				&idp.ResolvedAuthorizationMapping{
					MapsGroups: tc.mapsGroups, MapsRoles: tc.mapsRole, Targets: tc.targets, Complete: true,
				}, syncTestReceived, "fed-sub")
			s.Require().NotNil(sync)
			s.Equal(tc.groups, sync.Entity.Groups)
			s.Equal(tc.roles, sync.Entity.Roles)
			s.False(sync.MappingIncomplete)
		})
	}
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_KeepsAmbiguousMatches() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll),
		&idp.ResolvedAuthorizationMapping{
			MapsGroups: true, Complete: true,
			Targets: []providers.AuthorizationTarget{{Type: providers.AuthorizationTargetGroup, ID: "g1"}},
			Ambiguous: []providers.AuthorizationTarget{
				{Type: providers.AuthorizationTargetGroup, ID: "g2"},
				{Type: providers.AuthorizationTargetGroup, ID: "g3"},
				{Type: providers.AuthorizationTargetRole, ID: "r2"},
			},
		}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.Equal(&providers.MembershipSync{IDs: []string{"g1"}, RetainIDs: []string{"g2", "g3"}}, sync.Entity.Groups)
	s.Nil(sync.Entity.Roles)
}

func (s *FederatedSyncTestSuite) TestBuildFederatedSync_IncompleteMapping() {
	sync := buildFederatedSync(syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll),
		&idp.ResolvedAuthorizationMapping{MapsGroups: true}, syncTestReceived, "fed-sub")

	s.Require().NotNil(sync)
	s.True(sync.MappingIncomplete)
	s.Nil(sync.Entity.Groups)
}

func syncTestContext(pending string) *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background(),
		RuntimeData: map[string]string{common.RuntimeKeyPendingFederatedSync: pending}}
}

func syncTestResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{RuntimeData: map[string]string{}}
}

var syncTestWrite = federatedSync{Entity: providers.FederatedEntitySync{
	Link:             syncTestLink,
	AttributesByType: map[string]map[string]interface{}{"Person": {"email": "new@example.com"}},
	Groups:           &providers.MembershipSync{IDs: []string{"g1"}},
}}

func encodedSync(s *FederatedSyncTestSuite, sync federatedSync) string {
	encoded, err := json.Marshal(sync)
	s.Require().NoError(err)
	return string(encoded)
}

// The pending sync is written once, then cleared so a later node doesn't write it again.
func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_WritesAndClears() {
	s.entityProvider.On("SyncFederatedEntity", "user-1", syncTestWrite.Entity).Return(nil).Once()
	execResp := syncTestResponse()

	ok, err := syncPendingFederatedUser(syncTestContext(encodedSync(s, syncTestWrite)), execResp,
		s.entityProvider, "user-1", s.logger)
	s.True(ok)
	s.NoError(err)
	s.Empty(execResp.Status)
	s.Equal("", execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])
	s.Contains(execResp.RuntimeData, common.RuntimeKeyPendingFederatedSync)
}

// A sync the same node just kept is read from its response before the flow's runtime data.
func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_ReadsTheResponseFirst() {
	execResp := syncTestResponse()
	s.Require().NoError(setPendingFederatedSync(execResp, nil))

	ok, err := syncPendingFederatedUser(syncTestContext(encodedSync(s, syncTestWrite)), execResp,
		s.entityProvider, "user-1", s.logger)
	s.True(ok)
	s.NoError(err)
	s.entityProvider.AssertNotCalled(s.T(), "SyncFederatedEntity", mock.Anything, mock.Anything)
}

func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_NothingToSync() {
	for _, tc := range []struct{ name, pending, userID string }{
		{"nothing pending", "", "user-1"},
		{"no user settled", "{}", ""},
	} {
		s.Run(tc.name, func() {
			ok, err := syncPendingFederatedUser(syncTestContext(tc.pending), syncTestResponse(), s.entityProvider,
				tc.userID, s.logger)
			s.True(ok)
			s.NoError(err)
		})
	}
	s.entityProvider.AssertNotCalled(s.T(), "SyncFederatedEntity", mock.Anything, mock.Anything)
}

func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_InvalidPendingSync() {
	ok, err := syncPendingFederatedUser(syncTestContext("{"), syncTestResponse(), s.entityProvider, "user-1",
		s.logger)
	s.False(ok)
	s.Error(err)
}

func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_IncompleteMappingIsAServerError() {
	ok, err := syncPendingFederatedUser(syncTestContext(encodedSync(s, federatedSync{MappingIncomplete: true})),
		syncTestResponse(), s.entityProvider, "user-1", s.logger)
	s.False(ok)
	s.ErrorContains(err, "authorization mapping could not be fully resolved")
}

func (s *FederatedSyncTestSuite) TestSyncPendingFederatedUser_ProviderErrors() {
	tests := []struct {
		name       string
		code       entityprovider.ErrorCode
		ok, server bool
		failed     bool
	}{
		{"rejected write fails sign-in", entityprovider.ErrorCodeSyncRejected, false, false, true},
		{"conflicting attribute fails sign-in", entityprovider.ErrorCodeAttributeConflict, false, false, true},
		{"system error is a server error", entityprovider.ErrorCodeSystemError, false, true, false},
		{"not implemented skips sync", entityprovider.ErrorCodeNotImplemented, true, false, false},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.entityProvider.On("SyncFederatedEntity", "user-1", mock.Anything).Return(
				entityprovider.NewEntityProviderError(tc.code, "failed", "detail")).Once()
			execResp := syncTestResponse()

			ok, err := syncPendingFederatedUser(syncTestContext(encodedSync(s, syncTestWrite)), execResp,
				s.entityProvider, "user-1", s.logger)
			s.Equal(tc.ok, ok)
			s.Equal(tc.server, err != nil)
			if tc.failed {
				s.Equal(providers.ExecFailure, execResp.Status)
				s.Equal(&ErrFederatedUserSyncFailed, execResp.Error)
			} else {
				s.Empty(execResp.Status)
			}
		})
	}
}

func (s *FederatedSyncTestSuite) TestSetPendingFederatedSync() {
	execResp := syncTestResponse()
	s.Require().NoError(setPendingFederatedSync(execResp, &syncTestWrite))
	s.JSONEq(encodedSync(s, syncTestWrite), execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])

	s.Require().NoError(setPendingFederatedSync(execResp, nil))
	s.Equal("", execResp.RuntimeData[common.RuntimeKeyPendingFederatedSync])
}

// pendingSync is the runtime data entry of a federated sign-in that synced the link idp-a, sub-1.
func pendingSync() string {
	encoded, _ := json.Marshal(federatedSync{Entity: providers.FederatedEntitySync{
		Link: providers.LinkedAccount{IdpID: "idp-a", Sub: "sub-1"},
	}})
	return string(encoded)
}

// syncedLink matches a sync through the link idpID, sub.
func syncedLink(idpID, sub string) interface{} {
	return mock.MatchedBy(func(sync providers.FederatedEntitySync) bool {
		return sync.Link == providers.LinkedAccount{IdpID: idpID, Sub: sub}
	})
}
