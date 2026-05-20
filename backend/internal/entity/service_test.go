/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
)

type ServiceTestSuite struct {
	suite.Suite
	store       *entityStoreInterfaceMock
	hashService *hashmock.HashServiceInterfaceMock
	svc         EntityServiceInterface
	ctx         context.Context
	testErr     error
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.store = newEntityStoreInterfaceMock(s.T())
	s.hashService = hashmock.NewHashServiceInterfaceMock(s.T())
	// Default: hashService.Generate returns a deterministic hash for any input.
	s.hashService.On("Generate", mock.Anything).Return(hash.Credential{
		Algorithm: "PBKDF2",
		Hash:      "testhash",
		Parameters: hash.CredParameters{
			Salt: "testsalt", Iterations: 1, KeySize: 32,
		},
	}, nil).Maybe()
	s.svc = newEntityService(s.store, s.hashService, nil, nil, transaction.NewNoOpTransactioner())
	s.ctx = context.Background()
	s.testErr = errors.New("store error")
}

func testEntity(id string) *Entity {
	attrs, _ := json.Marshal(map[string]interface{}{"username": "user-" + id})
	return &Entity{
		ID:         id,
		Category:   EntityCategoryUser,
		Type:       "employee",
		State:      EntityStateActive,
		OUID:       "ou-1",
		Attributes: json.RawMessage(attrs),
	}
}

func (s *ServiceTestSuite) TestCreateEntity_NilEntity() {
	_, err := s.svc.CreateEntity(s.ctx, nil, nil)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestCreateEntity_StoreCreateFails() {
	e := testEntity("e1")
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(s.testErr)
	_, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Error(err)
}

func (s *ServiceTestSuite) TestCreateEntity_GetAfterCreateFails() {
	e := testEntity("e2")
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(Entity{}, s.testErr)
	_, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Error(err)
}

func (s *ServiceTestSuite) TestCreateEntity_Success() {
	e := testEntity("e3")
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestGetEntity_Success() {
	e := testEntity("e4")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.GetEntity(s.ctx, e.ID)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestGetEntity_Error() {
	s.store.On("GetEntity", mock.Anything, "bad").Return(Entity{}, s.testErr)
	_, err := s.svc.GetEntity(s.ctx, "bad")
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_NilEntity() {
	_, err := s.svc.UpdateEntity(s.ctx, "id", nil)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestUpdateEntity_StoreFails() {
	e := testEntity("e5")
	s.store.On("UpdateEntity", mock.Anything, e).Return(s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_GetAfterUpdateFails() {
	e := testEntity("e6")
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(Entity{}, s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_Success() {
	e := testEntity("e7")
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestDeleteEntity_Delegates() {
	s.store.On("DeleteEntity", mock.Anything, "del1").Return(nil)
	s.NoError(s.svc.DeleteEntity(s.ctx, "del1"))
}

func (s *ServiceTestSuite) TestUpdateAttributes_GetEntityFails() {
	attrs := json.RawMessage(`{"username":"new"}`)
	s.store.On("GetEntity", mock.Anything, "bad").Return(Entity{}, s.testErr)
	err := s.svc.UpdateAttributes(s.ctx, "bad", attrs)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateAttributes_StoreFails() {
	e := testEntity("ua1")
	attrs := json.RawMessage(`{"username":"new"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateAttributes", mock.Anything, e.ID, attrs).Return(s.testErr)
	err := s.svc.UpdateAttributes(s.ctx, e.ID, attrs)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateAttributes_Success() {
	e := testEntity("ua2")
	attrs := json.RawMessage(`{"username":"new"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateAttributes", mock.Anything, e.ID, attrs).Return(nil)
	err := s.svc.UpdateAttributes(s.ctx, e.ID, attrs)
	s.NoError(err)
}

func (s *ServiceTestSuite) TestUpdateSystemCredentials_Delegates() {
	creds := json.RawMessage(`{"token":"x"}`)
	// Fetch existing (empty), hash new, merge, store.
	existingEntity := testEntity("e1")
	s.store.On("GetEntityWithCredentials", mock.Anything, "e1").
		Return(&entityWithCredentials{Entity: existingEntity, SchemaCredentials: nil, SystemCredentials: nil}, nil)
	s.store.On("UpdateSystemCredentials", mock.Anything, "e1", mock.AnythingOfType("json.RawMessage")).Return(nil)
	s.NoError(s.svc.UpdateSystemCredentials(s.ctx, "e1", creds))
}

func (s *ServiceTestSuite) TestGetCredentialsByType_NoCredentials() {
	e := testEntity("ecreds")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: nil, SystemCredentials: nil}, nil)
	creds, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "passkey")
	s.NoError(err)
	s.Nil(creds)
}

func (s *ServiceTestSuite) TestGetCredentialsByType_FromSystemColumn() {
	e := testEntity("ecreds")
	sysCreds := json.RawMessage(`{"passkey":[{"value":"v1"},{"value":"v2"}],"otp":[{"value":"o1"}]}`)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: nil, SystemCredentials: sysCreds}, nil)
	creds, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "passkey")
	s.NoError(err)
	s.Len(creds, 2)
	s.Equal("v1", creds[0].Value)
	s.Equal("v2", creds[1].Value)
}

func (s *ServiceTestSuite) TestGetCredentialsByType_FromSchemaColumn() {
	e := testEntity("ecreds")
	schemaCreds := json.RawMessage(`{"password":[{"value":"hashed-pw"}]}`)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: schemaCreds, SystemCredentials: nil}, nil)
	creds, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "password")
	s.NoError(err)
	s.Len(creds, 1)
	s.Equal("hashed-pw", creds[0].Value)
}

func (s *ServiceTestSuite) TestGetCredentialsByType_SystemOverridesSchema() {
	e := testEntity("ecreds")
	schemaCreds := json.RawMessage(`{"password":[{"value":"schema-pw"}]}`)
	sysCreds := json.RawMessage(`{"password":[{"value":"system-pw"}]}`)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: schemaCreds, SystemCredentials: sysCreds}, nil)
	creds, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "password")
	s.NoError(err)
	s.Len(creds, 1)
	s.Equal("system-pw", creds[0].Value, "system column should take precedence")
}

func (s *ServiceTestSuite) TestGetCredentialsByType_TypeAbsent() {
	e := testEntity("ecreds")
	sysCreds := json.RawMessage(`{"otp":[{"value":"o1"}]}`)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: nil, SystemCredentials: sysCreds}, nil)
	creds, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "passkey")
	s.NoError(err)
	s.Empty(creds)
}

func (s *ServiceTestSuite) TestGetCredentialsByType_StoreError() {
	s.store.On("GetEntityWithCredentials", mock.Anything, "bad").
		Return(nil, s.testErr)
	_, err := s.svc.GetCredentialsByType(s.ctx, "bad", "passkey")
	s.Error(err)
}

func (s *ServiceTestSuite) TestGetCredentialsByType_MalformedSystemJSON() {
	e := testEntity("ecreds")
	sysCreds := json.RawMessage(`not json`)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: nil, SystemCredentials: sysCreds}, nil)
	_, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "passkey")
	s.Error(err)
}

func (s *ServiceTestSuite) TestGetCredentialsByType_MalformedSchemaJSON() {
	e := testEntity("ecreds")
	schemaCreds := json.RawMessage(`not json`)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: schemaCreds, SystemCredentials: nil}, nil)
	_, err := s.svc.GetCredentialsByType(s.ctx, e.ID, "password")
	s.Error(err)
}

func (s *ServiceTestSuite) TestIdentifyEntity_Delegates() {
	filters := map[string]interface{}{"email": "x@y.com"}
	id := "found-id"
	s.store.On("IdentifyEntity", mock.Anything, filters).Return(&id, nil)
	got, err := s.svc.IdentifyEntity(s.ctx, filters)
	s.NoError(err)
	s.Equal(&id, got)
}

func (s *ServiceTestSuite) TestGetEntityListCount_Delegates() {
	s.store.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(5, nil)
	count, err := s.svc.GetEntityListCount(s.ctx, EntityCategoryUser, nil)
	s.NoError(err)
	s.Equal(5, count)
}

func (s *ServiceTestSuite) TestGetEntityList_Delegates() {
	e := testEntity("le1")
	s.store.On("GetEntityList", mock.Anything, "user", 10, 0, mock.Anything).Return([]Entity{*e}, nil)
	list, err := s.svc.GetEntityList(s.ctx, EntityCategoryUser, 10, 0, nil)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *ServiceTestSuite) TestGetEntityListCountByOUIDs_Delegates() {
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(3, nil)
	count, err := s.svc.GetEntityListCountByOUIDs(s.ctx, EntityCategoryUser, []string{"ou1"}, nil)
	s.NoError(err)
	s.Equal(3, count)
}

func (s *ServiceTestSuite) TestGetEntityListByOUIDs_Delegates() {
	e := testEntity("ou-e1")
	s.store.On("GetEntityListByOUIDs", mock.Anything, "user", []string{"ou1"}, 10, 0, mock.Anything).
		Return([]Entity{*e}, nil)
	list, err := s.svc.GetEntityListByOUIDs(s.ctx, EntityCategoryUser, []string{"ou1"}, 10, 0, nil)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *ServiceTestSuite) TestValidateEntityIDs_Delegates() {
	s.store.On("ValidateEntityIDs", mock.Anything, []string{"id1", "id2"}).Return([]string{}, nil)
	invalid, err := s.svc.ValidateEntityIDs(s.ctx, []string{"id1", "id2"})
	s.NoError(err)
	s.Empty(invalid)
}

func (s *ServiceTestSuite) TestGetEntitiesByIDs_Delegates() {
	e := testEntity("bid1")
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"bid1"}).Return([]Entity{*e}, nil)
	list, err := s.svc.GetEntitiesByIDs(s.ctx, []string{"bid1"})
	s.NoError(err)
	s.Len(list, 1)
}

func (s *ServiceTestSuite) TestValidateEntityIDsInOUs_Delegates() {
	s.store.On("ValidateEntityIDsInOUs", mock.Anything, []string{"id1"}, []string{"ou1"}).
		Return([]string{}, nil)
	out, err := s.svc.ValidateEntityIDsInOUs(s.ctx, []string{"id1"}, []string{"ou1"})
	s.NoError(err)
	s.Empty(out)
}

func (s *ServiceTestSuite) TestGetGroupCountForEntity_Delegates() {
	s.store.On("GetGroupCountForEntity", mock.Anything, "e1").Return(2, nil)
	count, err := s.svc.GetGroupCountForEntity(s.ctx, "e1")
	s.NoError(err)
	s.Equal(2, count)
}

func (s *ServiceTestSuite) TestGetEntityGroups_Delegates() {
	groups := []EntityGroup{{ID: "g1", Name: "Group1", OUID: "ou1"}}
	s.store.On("GetEntityGroups", mock.Anything, "e1", 10, 0).Return(groups, nil)
	got, err := s.svc.GetEntityGroups(s.ctx, "e1", 10, 0)
	s.NoError(err)
	s.Len(got, 1)
}

func (s *ServiceTestSuite) TestIsEntityDeclarative_Delegates() {
	s.store.On("IsEntityDeclarative", mock.Anything, "e1").Return(true, nil)
	ok, err := s.svc.IsEntityDeclarative(s.ctx, "e1")
	s.NoError(err)
	s.True(ok)
}

func (s *ServiceTestSuite) TestLoadDeclarativeResources_MutableStore_NoOp() {
	cfg := DeclarativeLoaderConfig{
		Directory: "users",
		Category:  EntityCategoryUser,
		Parser: func(data []byte) (*Entity, json.RawMessage, json.RawMessage, error) {
			return nil, nil, nil, nil
		},
	}
	// store is a mock (not file/composite) → fileStore == nil → returns nil immediately
	err := s.svc.LoadDeclarativeResources(cfg)
	s.NoError(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_NilEntity_ViaOldPath() {
	_, err := s.svc.UpdateEntity(s.ctx, "id", nil)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestUpdateEntity_UpdateFails_ViaOldPath() {
	e := testEntity("uc1")
	s.store.On("UpdateEntity", mock.Anything, e).Return(s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_GetAfterUpdateFails_ViaOldPath() {
	e := testEntity("uc3")
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(Entity{}, s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_Success_ViaOldPath() {
	e := testEntity("uc4")
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestSearchEntities_Success() {
	filters := map[string]interface{}{"email": "a@b.com"}
	entities := []Entity{*testEntity("e1"), *testEntity("e2")}
	s.store.On("SearchEntities", mock.Anything, filters).Return(entities, nil)
	got, err := s.svc.SearchEntities(s.ctx, filters)
	s.NoError(err)
	s.Len(got, 2)
}

func (s *ServiceTestSuite) TestSearchEntities_Error() {
	filters := map[string]interface{}{"email": "a@b.com"}
	s.store.On("SearchEntities", mock.Anything, filters).Return(nil, s.testErr)
	_, err := s.svc.SearchEntities(s.ctx, filters)
	s.Error(err)
}

func testCredentialsJSON() json.RawMessage {
	return json.RawMessage(`{"password":[{` +
		`"value":"testhash","storageAlgo":"PBKDF2",` +
		`"storageAlgoParams":{"salt":"testsalt","iterations":1,"keySize":32}}]}`)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_Success() {
	storedCreds := testCredentialsJSON()
	e := testEntity("auth-id-1")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: storedCreds}, nil)
	s.hashService.On("Verify", []byte("password123"), mock.Anything).Return(true, nil)

	result, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "password123"})
	s.NoError(err)
	s.Equal(e.ID, result.EntityID)
	s.Equal(e.Category, result.EntityCategory)
	s.Equal(e.Type, result.EntityType)
	s.Equal(e.OUID, result.OUID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_EmptyID() {
	_, err := s.svc.AuthenticateEntityByID(s.ctx, "", map[string]interface{}{"password": "p"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_EmptyCredentials() {
	_, err := s.svc.AuthenticateEntityByID(s.ctx, "some-id", map[string]interface{}{})
	s.ErrorIs(err, ErrAuthenticationFailed)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_EntityNotFound() {
	s.store.On("GetEntityWithCredentials", mock.Anything, "missing").
		Return(nil, ErrEntityNotFound)

	_, err := s.svc.AuthenticateEntityByID(s.ctx, "missing", map[string]interface{}{"password": "p"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_InactiveEntity() {
	e := testEntity("inactive-1")
	e.State = EntityState("SUSPENDED")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)

	_, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "p"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_WrongCredentials() {
	storedCreds := testCredentialsJSON()
	e := testEntity("auth-fail-1")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: storedCreds}, nil)
	s.hashService.On("Verify", []byte("wrong"), mock.Anything).Return(false, nil)

	_, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
}

func (s *ServiceTestSuite) TestAuthenticateEntity_DelegatesToByID() {
	id := "delegate-1"
	filters := map[string]interface{}{"username": "user1"}
	storedCreds := testCredentialsJSON()
	e := testEntity(id)

	s.store.On("IdentifyEntity", mock.Anything, filters).Return(&id, nil)
	s.store.On("GetEntityWithCredentials", mock.Anything, id).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: storedCreds}, nil)
	s.hashService.On("Verify", []byte("pass"), mock.Anything).Return(true, nil)

	result, err := s.svc.AuthenticateEntity(s.ctx, filters, map[string]interface{}{"password": "pass"})
	s.NoError(err)
	s.Equal(id, result.EntityID)
}
