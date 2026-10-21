// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

var syncTestSchema = []entitytype.AttributeInfo{
	{Attribute: "username"}, {Attribute: "email"}, {Attribute: "age", Type: entitytypemodel.TypeNumber},
	{Attribute: "verified", Type: entitytypemodel.TypeBoolean}, {Attribute: "tags", Type: entitytypemodel.TypeArray},
}

func (s *ServiceTestSuite) newSyncSvc() (
	*entityService, *entitytypemock.EntityTypeServiceInterfaceMock, *GroupMembershipProviderMock,
	*RoleMembershipProviderMock,
) {
	svc, ets := s.newSvcWithEntityType()
	groups := NewGroupMembershipProviderMock(s.T())
	roles := NewRoleMembershipProviderMock(s.T())
	svc.SetGroupMembershipProvider(groups)
	svc.SetRoleMembershipProvider(roles)
	return svc, ets, groups, roles
}

func syncTestUser(attributes string) providers.Entity {
	return providers.Entity{ID: "u1", Category: providers.EntityCategoryUser, Type: "employee",
		Attributes: json.RawMessage(attributes)}
}

// expectAttributeWrite stubs the schema lookups and the store write UpdateAttributes performs for
// written.
func (s *ServiceTestSuite) expectAttributeWrite(ets *entitytypemock.EntityTypeServiceInterfaceMock, written string) {
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).Return(syncTestSchema, nil)
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, nil)
	ets.On("ValidateEntity", mock.Anything, mock.Anything, "employee", mock.Anything, true).Return(true, nil)
	ets.On("ValidateEntityUniqueness", mock.Anything, mock.Anything, "employee", mock.Anything, mock.Anything).
		Return(true, nil)
	s.store.On("UpdateAttributes", mock.Anything, "u1", mock.MatchedBy(func(got json.RawMessage) bool {
		return s.JSONEq(written, string(got))
	})).Return(nil).Once()
}

func (s *ServiceTestSuite) expectSchema(ets *entitytypemock.EntityTypeServiceInterfaceMock) {
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowNonCredential: true}).Return(syncTestSchema, nil).Once()
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_WritesChangedAttributesGroupsAndRoles() {
	svc, ets, groups, roles := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").
		Return(syncTestUser(`{"username":"local","email":"old@example.com","age":30}`), nil)
	s.expectSchema(ets)
	s.expectAttributeWrite(ets,
		`{"username":"local","email":"new@example.com","age":30,"verified":true,"tags":["a"]}`)
	groups.On("ReplaceEntityGroups", mock.Anything, "u1", providers.EntityCategoryUser, []string{"g1"},
		[]string{"g2"}).Return(nil).Once()
	roles.On("ReplaceEntityRoles", mock.Anything, "u1", providers.EntityCategoryUser, []string{},
		[]string{"r2"}).Return(nil).Once()

	err := svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{
		AttributesByType: map[string]map[string]interface{}{
			"employee": {
				"email": "new@example.com", "age": "30", "verified": "true", "tags": []interface{}{"a"},
				"undeclared": "x", "username": "",
			},
			"contractor": {"email": "contractor@example.com", "username": "contractor"},
		},
		Groups: &providers.MembershipSync{IDs: []string{"g1"}, RetainIDs: []string{"g2"}},
		Roles:  &providers.MembershipSync{IDs: []string{}, RetainIDs: []string{"r2"}},
	})
	s.NoError(err)
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_UnchangedAttributesWriteNothing() {
	svc, ets, _, _ := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").Return(syncTestUser(`{"email":"same@example.com","age":30}`), nil)
	s.expectSchema(ets)

	err := svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{
		AttributesByType: map[string]map[string]interface{}{
			"employee": {"email": "same@example.com", "age": 30},
		},
	})
	s.NoError(err)
	s.store.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// An entity whose user type has no entry keeps its attributes, even when another type's values were
// received.
func (s *ServiceTestSuite) TestSyncFederatedEntity_OtherUserTypeWritesNothing() {
	svc, _, _, _ := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").Return(syncTestUser(`{"email":"old@example.com"}`), nil)

	err := svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{
		AttributesByType: map[string]map[string]interface{}{"contractor": {"email": "new@example.com"}},
	})
	s.NoError(err)
	s.store.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_ReadOnlyEntityKeepsAttributes() {
	svc, _, groups, _ := s.newSyncSvc()
	user := syncTestUser(`{"email":"old@example.com"}`)
	user.IsReadOnly = true
	s.store.On("GetEntity", mock.Anything, "u1").Return(user, nil)
	groups.On("ReplaceEntityGroups", mock.Anything, "u1", providers.EntityCategoryUser, []string{"g1"}, []string(nil)).
		Return(nil).Once()

	err := svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{
		AttributesByType: map[string]map[string]interface{}{"employee": {"email": "new@example.com"}},
		Groups:           &providers.MembershipSync{IDs: []string{"g1"}},
	})
	s.NoError(err)
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_RejectedWriteIsMarked() {
	svc, _, groups, roles := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").Return(syncTestUser(`{}`), nil)
	groups.On("ReplaceEntityGroups", mock.Anything, "u1", providers.EntityCategoryUser, []string{"g1"}, []string(nil)).
		Return(nil).Once()
	roles.On("ReplaceEntityRoles", mock.Anything, "u1", providers.EntityCategoryUser, []string{"r1"}, []string(nil)).
		Return(&tidcommon.ErrorUnauthorized).Once()

	err := svc.SyncFederatedEntity(s.ctx, "u1",
		providers.FederatedEntitySync{
			Groups: &providers.MembershipSync{IDs: []string{"g1"}},
			Roles:  &providers.MembershipSync{IDs: []string{"r1"}},
		})
	s.ErrorIs(err, ErrSyncRejected)
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_ServerErrorIsNotMarked() {
	svc, _, groups, _ := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").Return(syncTestUser(`{}`), nil)
	groups.On("ReplaceEntityGroups", mock.Anything, "u1", providers.EntityCategoryUser, []string{"g1"}, []string(nil)).
		Return(&tidcommon.InternalServerError).Once()

	err := svc.SyncFederatedEntity(s.ctx, "u1",
		providers.FederatedEntitySync{Groups: &providers.MembershipSync{IDs: []string{"g1"}}})
	s.Error(err)
	s.NotErrorIs(err, ErrSyncRejected)
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_EntityNotFound() {
	svc, _, _, _ := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").Return(providers.Entity{}, ErrEntityNotFound)

	err := svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{Groups: &providers.MembershipSync{}})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_SchemaLookupFails() {
	svc, ets, _, _ := s.newSyncSvc()
	s.store.On("GetEntity", mock.Anything, "u1").Return(syncTestUser(`{}`), nil)
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowNonCredential: true}).Return(nil, &tidcommon.InternalServerError)

	err := svc.SyncFederatedEntity(s.ctx, "u1",
		providers.FederatedEntitySync{
			AttributesByType: map[string]map[string]interface{}{"employee": {"email": "a@example.com"}},
		})
	s.ErrorContains(err, "failed to get schema attributes")
}

func (s *ServiceTestSuite) TestSyncFederatedEntity_NoWriterRegistered() {
	s.store.On("GetEntity", mock.Anything, "u1").Return(syncTestUser(`{}`), nil)

	s.Error(s.svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{Groups: &providers.MembershipSync{}}))
	s.Error(s.svc.SyncFederatedEntity(s.ctx, "u1", providers.FederatedEntitySync{Roles: &providers.MembershipSync{}}))
}
