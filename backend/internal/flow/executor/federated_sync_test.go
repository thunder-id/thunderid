// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

var syncTestSchema = []entitytype.AttributeInfo{
	{Attribute: "username"}, {Attribute: "email"}, {Attribute: "firstName"},
	{Attribute: "mobile"}, {Attribute: "sub"}, {Attribute: "employeeId", Required: true},
	{Attribute: "age", Type: entitytypemodel.TypeNumber},
	{Attribute: "email_verified", Type: entitytypemodel.TypeString},
}

type FederatedSyncTestSuite struct {
	suite.Suite
	entityProvider *entityprovidermock.EntityProviderInterfaceMock
	entityTypes    *entitytypemock.EntityTypeServiceInterfaceMock
	users          *usermock.UserServiceInterfaceMock
	syncer         *federatedSyncer
}

func TestFederatedSyncTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedSyncTestSuite))
}

func (s *FederatedSyncTestSuite) SetupTest() {
	s.entityProvider = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.entityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.users = usermock.NewUserServiceInterfaceMock(s.T())
	s.syncer = &federatedSyncer{
		entityProvider: s.entityProvider, entityTypeService: s.entityTypes, userService: s.users,
	}
}

func (s *FederatedSyncTestSuite) TestPlanFederatedSync_Attributes() {
	current := map[string]interface{}{
		"username": "local", "email": "old@example.com", "mobile": "0771234567", "sub": "local-sub",
		"employeeId": "E1",
	}
	received := map[string]interface{}{
		"username": "fed", "email": "new@example.com", "firstName": "Fed", "sub": "fed-sub", "picture": "x",
	}

	tests := []struct {
		name     string
		mode     providers.SyncMode
		expected map[string]interface{}
	}{
		{"no sync", providers.SyncModeNoSync, nil},
		{"session only", providers.SyncModeSessionOnly, nil},
		{"override received returns only the changed attributes", providers.SyncModeOverrideReceived,
			map[string]interface{}{"username": "fed", "email": "new@example.com", "firstName": "Fed"}},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			plan := planFederatedSync(tc.mode, providers.SyncModeNoSync, syncTestSchema, current, received,
				false, false, nil, nil)
			s.Equal(tc.expected, plan.attributes)
			s.Nil(plan.groupIDs)
			s.Nil(plan.roleIDs)
		})
	}
}

func (s *FederatedSyncTestSuite) TestPlanFederatedSync_UnchangedAttributesAreNotWritten() {
	current := map[string]interface{}{"username": "fed", "age": float64(5), "sub": "sub-1"}
	received := map[string]interface{}{"username": "fed", "age": 5}

	plan := planFederatedSync(providers.SyncModeOverrideReceived, providers.SyncModeNoSync, syncTestSchema,
		current, received, false, false, nil, nil)

	s.Nil(plan.attributes)
}

func (s *FederatedSyncTestSuite) TestPlanFederatedSync_ConvertsReceivedValuesToSchemaTypes() {
	received := map[string]interface{}{"age": "30", "email_verified": true}

	plan := planFederatedSync(providers.SyncModeOverrideReceived, providers.SyncModeNoSync, syncTestSchema,
		map[string]interface{}{}, received, false, false, nil, nil)

	s.Equal(map[string]interface{}{"age": float64(30), "email_verified": "true"}, plan.attributes)
}

func (s *FederatedSyncTestSuite) TestPlanFederatedSync_KeepsArraysAndObjects() {
	schema := []entitytype.AttributeInfo{
		{Attribute: "tags", Type: entitytypemodel.TypeArray},
		{Attribute: "address", Type: entitytypemodel.TypeObject},
	}
	received := map[string]interface{}{
		"tags":    []interface{}{"a", "b"},
		"address": map[string]interface{}{"city": "Kandy"},
	}

	plan := planFederatedSync(providers.SyncModeOverrideReceived, providers.SyncModeNoSync, schema,
		map[string]interface{}{"tags": []interface{}{"a"}, "address": map[string]interface{}{"city": "Kandy"}},
		received, false, false, nil, nil)

	s.Equal(map[string]interface{}{"tags": []interface{}{"a", "b"}}, plan.attributes)
}

func (s *FederatedSyncTestSuite) TestPlanFederatedSync_GroupsAndRoles() {
	s.Run("override all with a mapping sets the mapped groups and roles", func() {
		plan := planFederatedSync(providers.SyncModeNoSync, providers.SyncModeOverrideAll, nil, nil, nil,
			true, true, []string{"g1"}, nil)
		s.Equal([]string{"g1"}, plan.groupIDs)
		s.Equal([]string{}, plan.roleIDs)
	})
	s.Run("override all leaves roles alone when the mapping resolves only groups", func() {
		plan := planFederatedSync(providers.SyncModeNoSync, providers.SyncModeOverrideAll, nil, nil, nil,
			true, false, []string{"g1"}, nil)
		s.Equal([]string{"g1"}, plan.groupIDs)
		s.Nil(plan.roleIDs)
	})
	s.Run("override all without a mapping leaves memberships alone", func() {
		plan := planFederatedSync(providers.SyncModeNoSync, providers.SyncModeOverrideAll, nil, nil, nil,
			false, false, []string{"g1"}, []string{"r1"})
		s.Nil(plan.groupIDs)
		s.Nil(plan.roleIDs)
	})
	s.Run("no sync leaves memberships alone", func() {
		plan := planFederatedSync(providers.SyncModeNoSync, providers.SyncModeNoSync, nil, nil, nil,
			true, true, []string{"g1"}, []string{"r1"})
		s.Nil(plan.groupIDs)
		s.Nil(plan.roleIDs)
	})
}

func (s *FederatedSyncTestSuite) TestMappingTargetTypes() {
	permission := providers.AuthorizationTarget{Type: providers.AuthorizationTargetPermission, Permission: "read"}
	tests := []struct {
		name                  string
		mapping               *providers.AuthorizationMapping
		wantGroups, wantRoles bool
	}{
		{"no mapping", nil, false, false},
		{"permissions only", &providers.AuthorizationMapping{
			Rules: []providers.AuthorizationRuleMapping{{Claim: "level", Values: []providers.AuthorizationRule{
				{Value: "1", Targets: []providers.AuthorizationTarget{permission}}}}},
			Direct: []providers.AuthorizationDirectMapping{
				{Claim: "scopes", TargetType: providers.AuthorizationTargetPermission}},
		}, false, false},
		{"rule role and direct group", &providers.AuthorizationMapping{
			Rules: []providers.AuthorizationRuleMapping{{Claim: "level", Values: []providers.AuthorizationRule{
				{Value: "1", Targets: []providers.AuthorizationTarget{
					{Type: providers.AuthorizationTargetRole, ID: "r1"}}}}}},
			Direct: []providers.AuthorizationDirectMapping{
				{Claim: "groups", TargetType: providers.AuthorizationTargetGroup}},
		}, true, true},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			groups, roles := mappingTargetTypes(tc.mapping)
			s.Equal(tc.wantGroups, groups)
			s.Equal(tc.wantRoles, roles)
		})
	}
}

func (s *FederatedSyncTestSuite) TestMappedAttributes() {
	idpDTO := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{UserType: "person", Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "mail", LocalAttribute: "email"},
				{ExternalAttribute: "address.city", LocalAttribute: "city"},
				{ExternalAttribute: "phone", LocalAttribute: "mobile"},
			}},
			{UserType: "employee", Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
			}},
		},
	}}
	received := map[string]interface{}{
		"mail": "fed@example.com", "address": map[string]interface{}{"city": "Kandy"},
		"given_name": "Fed", "username": "unmapped",
	}

	s.Equal(map[string]interface{}{"email": "fed@example.com", "city": "Kandy"},
		mappedAttributes(idpDTO, "person", received))
	s.Empty(mappedAttributes(&providers.IDPDTO{}, "person", received))
}

func syncTestIDP(attributes, groupsAndRoles providers.SyncMode) *providers.IDPDTO {
	return &providers.IDPDTO{ID: "idp-1", AttributeConfiguration: &providers.AttributeConfiguration{
		Sync: &providers.SyncConfiguration{Attributes: attributes, GroupsAndRoles: groupsAndRoles},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{UserType: "person",
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "username", LocalAttribute: "username"},
				{ExternalAttribute: "email", LocalAttribute: "email"},
			}}},
		AuthorizationMapping: &providers.AuthorizationMapping{
			Direct: []providers.AuthorizationDirectMapping{
				{Claim: "groups", TargetType: "group"}, {Claim: "roles", TargetType: "role"},
			},
		},
	}}
}

func (s *FederatedSyncTestSuite) syncTestContext() *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background()}
}

func (s *FederatedSyncTestSuite) expectUser() {
	s.entityProvider.On("GetEntity", "user-1").Return(&providers.Entity{
		ID: "user-1", Type: "person", Attributes: json.RawMessage(`{"username":"local","sub":"sub-1"}`),
	}, nil).Once()
	s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).Return(syncTestSchema, nil).Once()
}

func (s *FederatedSyncTestSuite) TestSyncAccount_OverridesAttributesGroupsAndRoles() {
	s.expectUser()
	s.users.On("SyncFederatedUser", mock.Anything, "user-1",
		map[string]interface{}{"email": "fed@example.com", "username": "fed"},
		[]string{"mapped-group"}, []string{"mapped-role"}).Return((*tidcommon.ServiceError)(nil)).Once()

	err := s.syncer.syncAccount(s.syncTestContext(),
		syncTestIDP(providers.SyncModeOverrideReceived, providers.SyncModeOverrideAll), "user-1",
		map[string]interface{}{"username": "fed", "email": "fed@example.com", "firstName": "Unmapped"},
		[]string{"mapped-group"}, []string{"mapped-role"}, true, log.GetLogger())

	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncAccount_UnmappedAttributesWriteNothing() {
	s.expectUser()

	err := s.syncer.syncAccount(s.syncTestContext(),
		syncTestIDP(providers.SyncModeOverrideReceived, providers.SyncModeNoSync), "user-1",
		map[string]interface{}{"username": "local", "firstName": "Unmapped"}, nil, nil, true, log.GetLogger())

	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncAccount_NoSyncWritesNothing() {
	err := s.syncer.syncAccount(s.syncTestContext(),
		syncTestIDP(providers.SyncModeNoSync, providers.SyncModeSessionOnly), "user-1",
		map[string]interface{}{"username": "fed"}, []string{"g1"}, nil, true, log.GetLogger())

	s.NoError(err)
}

func (s *FederatedSyncTestSuite) TestSyncAccount_IncompleteMappingFails() {
	err := s.syncer.syncAccount(s.syncTestContext(),
		syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll), "user-1",
		nil, nil, nil, false, log.GetLogger())

	s.ErrorContains(err, "authorization mapping could not be fully resolved")
}

func (s *FederatedSyncTestSuite) TestSyncAccount_WriteFailureFails() {
	s.expectUser()
	s.users.On("SyncFederatedUser", mock.Anything, "user-1", mock.Anything,
		[]string(nil), []string(nil)).Return(&tidcommon.ServiceError{
		Error: tidcommon.I18nMessage{DefaultValue: "schema validation failed"}}).Once()

	err := s.syncer.syncAccount(s.syncTestContext(),
		syncTestIDP(providers.SyncModeOverrideReceived, providers.SyncModeNoSync), "user-1",
		map[string]interface{}{"email": "fed@example.com"}, nil, nil, true, log.GetLogger())

	s.ErrorContains(err, "schema validation failed")
}

func (s *FederatedSyncTestSuite) TestSyncFederatedUser_FailureFailsSignIn() {
	execResp := &providers.ExecutorResponse{RuntimeData: map[string]string{}}

	ok := s.syncer.syncFederatedUser(s.syncTestContext(), execResp,
		syncTestIDP(providers.SyncModeNoSync, providers.SyncModeOverrideAll), "user-1", nil, false,
		log.GetLogger())

	s.False(ok)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(&ErrFederatedUserSyncFailed, execResp.Error)
}
