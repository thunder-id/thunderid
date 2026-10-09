// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
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
	s.store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	s.hashService = hashmock.NewHashServiceInterfaceMock(s.T())
	// Default: hashService.Generate returns a deterministic hash for any input.
	s.hashService.On("Generate", mock.Anything).Return(cryptolib.Credential{
		Algorithm: "PBKDF2",
		Hash:      "testhash",
		Parameters: cryptolib.CredParameters{
			Salt: "testsalt", Iterations: 1, KeySize: 32,
		},
	}, nil).Maybe()
	s.svc = newEntityService(s.store, s.hashService, nil, nil, transaction.NewNoOpTransactioner())
	s.ctx = context.Background()
	s.testErr = errors.New("store error")
}

func testEntity(id string) *providers.Entity {
	attrs, _ := json.Marshal(map[string]interface{}{"username": "user-" + id})
	return &providers.Entity{
		ID:         id,
		Category:   providers.EntityCategoryUser,
		Type:       "employee",
		State:      providers.EntityStateActive,
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
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr)
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

// A new entity whose linkedIds name a subject another entity holds is refused before anything is
// written, and a subject nobody holds lets the create through.
func (s *ServiceTestSuite) TestCreateEntity_LinkedAccount() {
	other := "someone-else"
	for name, tc := range map[string]struct {
		holder  *string
		lookErr error
		wantErr error
	}{
		"free":      {lookErr: ErrEntityNotFound},
		"held":      {holder: &other, wantErr: ErrLinkedAccountConflict},
		"ambiguous": {lookErr: ErrAmbiguousEntity, wantErr: ErrLinkedAccountConflict},
	} {
		s.Run(name, func() {
			s.SetupTest()
			e := testEntity("e-linked")
			e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
			s.store.On("ResolveLinkedAccount", mock.Anything, "idp-a", "sub-1").
				Return(tc.holder, tc.lookErr).Once()
			if tc.wantErr == nil {
				s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
					Return(nil).Once()
				s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
			}

			_, err := s.svc.CreateEntity(s.ctx, e, nil)

			if tc.wantErr == nil {
				s.NoError(err)
				return
			}
			s.ErrorIs(err, tc.wantErr)
			s.store.AssertNotCalled(s.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything,
				mock.Anything)
		})
	}
}

func (s *ServiceTestSuite) TestGetEntity_Success() {
	e := testEntity("e4")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.GetEntity(s.ctx, e.ID)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestGetEntity_Error() {
	s.store.On("GetEntity", mock.Anything, "bad").Return(providers.Entity{}, s.testErr)
	_, err := s.svc.GetEntity(s.ctx, "bad")
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_NilEntity() {
	_, err := s.svc.UpdateEntity(s.ctx, "id", nil)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestUpdateEntity_StoreFails() {
	e := testEntity("e5")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_GetAfterUpdateFails() {
	e := testEntity("e6")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr).Once()
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_Success() {
	e := testEntity("e7")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) newSvcWithEntityType() (*entityService, *entitytypemock.EntityTypeServiceInterfaceMock) {
	ets := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	svc := newEntityService(s.store, s.hashService, ets, nil, transaction.NewNoOpTransactioner())
	return svc.(*entityService), ets
}

func (s *ServiceTestSuite) TestStripUndeclaredAttributes_DropsUndeclared() {
	svc, ets := s.newSvcWithEntityType()
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}, {Attribute: "email"}}, nil)

	out, err := svc.stripUndeclaredAttributes(s.ctx, providers.EntityCategoryUser, "employee",
		json.RawMessage(`{"username":"a","email":"b","stale":"c"}`))
	s.NoError(err)
	var m map[string]interface{}
	s.NoError(json.Unmarshal(out, &m))
	s.Equal(map[string]interface{}{"username": "a", "email": "b"}, m)
}

func (s *ServiceTestSuite) TestStripUndeclaredAttributes_PreservesLargeIntWhileDropping() {
	svc, ets := s.newSvcWithEntityType()
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "bigId"}}, nil)

	// bigId is above 2^53; decoding through float64 would round it to 9007254740992.
	out, err := svc.stripUndeclaredAttributes(s.ctx, providers.EntityCategoryUser, "employee",
		json.RawMessage(`{"bigId":9007199254740993,"stale":"x"}`))
	s.NoError(err)
	s.JSONEq(`{"bigId":9007199254740993}`, string(out))
	s.Contains(string(out), "9007199254740993")
}

func (s *ServiceTestSuite) TestStripUndeclaredAttributes_NoDeclaredAttrs_NoOp() {
	svc, ets := s.newSvcWithEntityType()
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{}, nil)

	in := json.RawMessage(`{"anything":"x"}`)
	out, err := svc.stripUndeclaredAttributes(s.ctx, providers.EntityCategoryUser, "employee", in)
	s.NoError(err)
	s.Equal(in, out)
}

func (s *ServiceTestSuite) TestStripUndeclaredAttributes_AllDeclared_NoOp() {
	svc, ets := s.newSvcWithEntityType()
	ets.On("GetAttributes", mock.Anything, mock.Anything, "employee",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}}, nil)

	in := json.RawMessage(`{"username":"x"}`)
	out, err := svc.stripUndeclaredAttributes(s.ctx, providers.EntityCategoryUser, "employee", in)
	s.NoError(err)
	s.Equal(in, out)
}

func (s *ServiceTestSuite) TestUpdateAttributes_DropsUndeclaredBeforeValidateAndStore() {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity("uad1")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	// strip: all declared attributes (username only).
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}}, nil)
	// credential extraction: no credential attributes.
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type, entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, nil)
	// Validation must receive the already-stripped payload, proving strip runs before validate.
	cleaned := json.RawMessage(`{"username":"new"}`)
	ets.On("ValidateEntity", mock.Anything, mock.Anything, e.Type, cleaned, true).
		Return(true, nil)
	ets.On("ValidateEntityUniqueness", mock.Anything, mock.Anything, e.Type, cleaned, mock.Anything).
		Return(true, nil)
	// The stale key is dropped before the write reaches the store.
	s.store.On("UpdateAttributes", mock.Anything, e.ID, cleaned).Return(nil)

	err := svc.UpdateAttributes(s.ctx, e.ID, json.RawMessage(`{"username":"new","stale":"x"}`))
	s.NoError(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_DropsUndeclaredBeforeValidateAndStore() {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity("ue-strip")
	e.Attributes = json.RawMessage(`{"username":"new","stale":"x"}`)
	// strip: only username is declared.
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}}, nil)
	// credential extraction: no credential attributes.
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type, entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, nil)
	// Validation must receive the already-stripped payload, proving strip runs before validate.
	cleaned := json.RawMessage(`{"username":"new"}`)
	ets.On("ValidateEntity", mock.Anything, mock.Anything, e.Type, cleaned, true).
		Return(true, nil)
	ets.On("ValidateEntityUniqueness", mock.Anything, mock.Anything, e.Type, cleaned, mock.Anything).
		Return(true, nil)
	// The stale key is dropped before the full-object update reaches the store.
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, mock.MatchedBy(func(ent *providers.Entity) bool {
		return string(ent.Attributes) == string(cleaned)
	})).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)

	_, err := svc.UpdateEntity(s.ctx, e.ID, e)
	s.NoError(err)
}

func (s *ServiceTestSuite) TestDeleteEntity_Delegates() {
	s.store.On("DeleteEntity", mock.Anything, "del1").Return(nil)
	s.NoError(s.svc.DeleteEntity(s.ctx, "del1"))
}

func (s *ServiceTestSuite) TestUpdateAttributes_GetEntityFails() {
	attrs := json.RawMessage(`{"username":"new"}`)
	s.store.On("GetEntity", mock.Anything, "bad").Return(providers.Entity{}, s.testErr)
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

// A password change stamps the entity so the refresh grant can reject tokens established before it.
// The stamp merges into the existing blob: the column is written wholesale and its other keys belong
// to the service that owns the entity.
func (s *ServiceTestSuite) TestUpdateCredentials_StampsCredentialMarkerPreservingOtherKeys() {
	e := testEntity("e-pw")
	e.SystemAttributes = json.RawMessage(`{"name":"App name"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateCredentials", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).Return(nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Run(func(args mock.Arguments) {
			written, _ = args.Get(2).(json.RawMessage)
		}).Return(nil)

	s.NoError(s.svc.UpdateCredentials(s.ctx, e.ID, json.RawMessage(`{"password":"new-secret"}`)))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	s.Equal("App name", attrs["name"], "unrelated system attributes must survive the stamp")
	s.NotEmpty(attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
}

// A client secret rotation stamps the entity, in the same transaction as the credential write.
func (s *ServiceTestSuite) TestUpdateSystemCredentials_StampsOnClientSecretRotation() {
	e := testEntity("e-cs")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil)
	s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Return(nil)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Return(nil)

	s.NoError(s.svc.UpdateSystemCredentials(s.ctx, e.ID, json.RawMessage(`{"clientSecret":"rotated"}`)))

	s.store.AssertCalled(s.T(), "UpdateSystemAttributes", mock.Anything, e.ID,
		mock.AnythingOfType("json.RawMessage"))
}

// Enrolling a passkey adds an authentication option rather than replacing one, so it must not
// invalidate outstanding tokens. The same holds for the flow secret, which authenticates flow
// initiation rather than the client.
func (s *ServiceTestSuite) TestUpdateSystemCredentials_NoStampForOtherCredentialTypes() {
	for _, creds := range []string{`{"passkey":[{"value":"v1"}]}`, `{"flowSecret":"rotated"}`} {
		e := testEntity("e-other")
		s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
			Return(&entityWithCredentials{Entity: e}, nil).Once()
		s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
			Return(nil).Once()

		s.NoError(s.svc.UpdateSystemCredentials(s.ctx, e.ID, json.RawMessage(creds)))

		s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, e.ID,
			mock.AnythingOfType("json.RawMessage"))
	}
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
	count, err := s.svc.GetEntityListCount(s.ctx, providers.EntityCategoryUser, nil)
	s.NoError(err)
	s.Equal(5, count)
}

func (s *ServiceTestSuite) TestGetEntityList_Delegates() {
	e := testEntity("le1")
	s.store.On("GetEntityList", mock.Anything, "user", 10, 0, mock.Anything).Return([]providers.Entity{*e}, nil)
	list, err := s.svc.GetEntityList(s.ctx, providers.EntityCategoryUser, 10, 0, nil)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *ServiceTestSuite) TestGetEntityListCountByOUIDs_Delegates() {
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(3, nil)
	count, err := s.svc.GetEntityListCountByOUIDs(s.ctx, providers.EntityCategoryUser, []string{"ou1"}, nil)
	s.NoError(err)
	s.Equal(3, count)
}

func (s *ServiceTestSuite) TestGetEntityListByOUIDs_Delegates() {
	e := testEntity("ou-e1")
	s.store.On("GetEntityListByOUIDs", mock.Anything, "user", []string{"ou1"}, 10, 0, mock.Anything).
		Return([]providers.Entity{*e}, nil)
	list, err := s.svc.GetEntityListByOUIDs(s.ctx, providers.EntityCategoryUser, []string{"ou1"}, 10, 0, nil)
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
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"bid1"}).Return([]providers.Entity{*e}, nil)
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
	groups := []providers.EntityGroup{{ID: "g1", Name: "Group1", OUID: "ou1"}}
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
		Category:  providers.EntityCategoryUser,
		Parser: func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
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
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_GetAfterUpdateFails_ViaOldPath() {
	e := testEntity("uc3")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr).Once()
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_Success_ViaOldPath() {
	e := testEntity("uc4")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestSearchEntities_Success() {
	filters := map[string]interface{}{"email": "a@b.com"}
	entities := []providers.Entity{*testEntity("e1"), *testEntity("e2")}
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
	e.State = providers.EntityState("SUSPENDED")
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

func (s *ServiceTestSuite) TestAuthenticateEntityByID_VerifyError() {
	storedCreds := testCredentialsJSON()
	e := testEntity("auth-verify-err-1")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: storedCreds}, nil)
	s.hashService.On("Verify", []byte("password123"), mock.Anything).Return(false, s.testErr)

	_, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "password123"})
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

// --- SetGroupMembershipProvider / GetTransitiveEntityGroups ---

func (s *ServiceTestSuite) TestGetTransitiveEntityGroups_ProviderNil() {
	groups, err := s.svc.GetTransitiveEntityGroups(s.ctx, "user1")

	s.NoError(err)
	s.Empty(groups)
}

func (s *ServiceTestSuite) TestSetGroupMembershipProvider_DelegatesOnCall() {
	provider := NewGroupMembershipProviderMock(s.T())
	expected := []providers.EntityGroup{{ID: "grp1", Name: "Admins", OUID: "ou1"}}
	provider.On("GetTransitiveGroupsForEntity", s.ctx, "user1").Return(expected, nil).Once()

	s.svc.SetGroupMembershipProvider(provider)
	groups, err := s.svc.GetTransitiveEntityGroups(s.ctx, "user1")

	s.NoError(err)
	s.Equal(expected, groups)
}

func (s *ServiceTestSuite) TestGetTransitiveEntityGroups_ProviderError() {
	provider := NewGroupMembershipProviderMock(s.T())
	provider.On("GetTransitiveGroupsForEntity", s.ctx, "user1").Return(nil, s.testErr).Once()

	s.svc.SetGroupMembershipProvider(provider)
	_, err := s.svc.GetTransitiveEntityGroups(s.ctx, "user1")

	s.ErrorIs(err, s.testErr)
}

// A system-attribute blob the server cannot parse must fail the credential write rather than silently
// replacing the blob, which would drop the keys belonging to whichever service owns the entity.
func (s *ServiceTestSuite) TestSetCredentialUpdatedAt_CorruptBlobErrors() {
	_, err := setCredentialUpdatedAt(json.RawMessage(`not json`), time.Now().UTC())
	s.Error(err)
}

// An absent blob is normal for an entity that has never carried system attributes.
func (s *ServiceTestSuite) TestSetCredentialUpdatedAt_EmptyBlobStartsFresh() {
	marked, err := setCredentialUpdatedAt(nil, time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC))
	s.Require().NoError(err)

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(marked, &attrs))
	s.Equal("2026-08-12T10:00:00Z", attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
}

// A service that rebuilds an entity's whole system-attribute blob, such as an application rename,
// must not drop the credential-change marker this package owns.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_PreservesCredentialMarker() {
	e := testEntity("e-preserve")
	e.SystemAttributes = json.RawMessage(`{"name":"Old","credentialUpdatedAt":"2026-08-12T10:00:00Z"}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	s.Equal("New", attrs["name"], "the caller's own keys are replaced")
	s.Equal("2026-08-12T10:00:00Z", attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
}

// UpdateEntity replaces the blob too, so it carries the marker across as well.
func (s *ServiceTestSuite) TestUpdateEntity_PreservesCredentialMarker() {
	stored := testEntity("e-preserve-2")
	stored.SystemAttributes = json.RawMessage(`{"credentialUpdatedAt":"2026-08-12T10:00:00Z"}`)
	s.store.On("LockEntity", mock.Anything, stored.ID).Return(*stored, nil)
	s.store.On("GetEntity", mock.Anything, stored.ID).Return(*stored, nil)
	s.store.On("UpdateEntity", mock.Anything, mock.Anything).Return(nil)

	incoming := testEntity("e-preserve-2")
	incoming.SystemAttributes = json.RawMessage(`{"name":"Renamed"}`)
	_, err := s.svc.UpdateEntity(s.ctx, incoming.ID, incoming)
	s.Require().NoError(err)

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(incoming.SystemAttributes, &attrs))
	s.Equal("Renamed", attrs["name"])
	s.Equal("2026-08-12T10:00:00Z", attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
}

// Linked accounts are what resolves a returning linked user. Dropping them on an
// unrelated profile update would re-provision that user into a unique-attribute conflict on their
// next login, so they survive a wholesale replacement of the blob alongside the credential marker.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_PreservesLinkedAccounts() {
	e := testEntity("e-preserve-fed")
	e.SystemAttributes = json.RawMessage(
		`{"name":"Old","credentialUpdatedAt":"2026-08-12T10:00:00Z",` +
			`"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	s.Equal("New", attrs["name"], "the caller's own keys are replaced")
	s.Equal("2026-08-12T10:00:00Z", attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
	s.Equal(map[string]interface{}{"idp-a": map[string]interface{}{"sub-1": map[string]interface{}{}}},
		attrs[authnprovidercm.SystemAttrLinkedIDs])
}

// A stored blob that does not parse may still hold reserved keys, so the replace fails rather than
// dropping them.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_UnparsableStoredBlobFails() {
	e := testEntity("e-preserve-bad")
	e.SystemAttributes = json.RawMessage(`not json`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	s.Error(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// Links are written only by LinkAccount, which enforces the link limit, so a caller that
// sends linkedIds through a wholesale replace has them dropped when the entity holds none.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_DropsCallerSentLinks() {
	e := testEntity("e-sent-links")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID,
		json.RawMessage(`{"name":"New","linkedIds":{"idp-a":{"sub-x":{}}}}`)))
	s.JSONEq(`{"name":"New"}`, string(written))
}

// A caller's linkedIds never replace the stored links.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_StoredLinksOverrideCallerSentLinks() {
	e := testEntity("e-override-links")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID,
		json.RawMessage(`{"name":"New","linkedIds":{"idp-b":{"sub-x":{}}}}`)))
	s.JSONEq(`{"name":"New","linkedIds":{"idp-a":{"sub-1":{}}}}`, string(written))
}

// linkedIds are indexed whatever the configuration, so the subjects across every connection are held
// to the per-attribute limit on input as well.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_RejectsLinksOverLimit() {
	byIDP := map[string]interface{}{"idp-a": map[string]interface{}{}, "idp-b": map[string]interface{}{}}
	for i := 0; i <= maxIndexedValuesPerAttribute; i++ {
		idp := "idp-a"
		if i%2 == 1 {
			idp = "idp-b"
		}
		byIDP[idp].(map[string]interface{})[fmt.Sprintf("sub-%d", i)] = map[string]interface{}{}
	}
	blob, err := json.Marshal(map[string]interface{}{authnprovidercm.SystemAttrLinkedIDs: byIDP})
	s.Require().NoError(err)

	err = s.svc.UpdateSystemAttributes(s.ctx, "e-over", blob)
	s.ErrorIs(err, ErrIndexedValueLimitExceeded)
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// The credential marker is written over the blob read under the entity's lock, not the copy read
// with the credentials, which can come from a cache filled before a concurrent link committed.
func (s *ServiceTestSuite) TestUpdateCredentials_StampsOverLockedBlob() {
	cached := testEntity("e-pw-lock")
	locked := *cached
	locked.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.store.On("GetEntity", mock.Anything, cached.ID).Return(*cached, nil)
	s.store.On("GetEntityWithCredentials", mock.Anything, cached.ID).
		Return(&entityWithCredentials{Entity: cached}, nil)
	s.store.On("UpdateCredentials", mock.Anything, cached.ID, mock.Anything).Return(nil)
	s.store.On("LockEntity", mock.Anything, cached.ID).Return(locked, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, cached.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateCredentials(s.ctx, cached.ID, json.RawMessage(`{"password":"new-secret"}`)))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	s.Contains(attrs, authnprovidercm.SystemAttrLinkedIDs)
	s.NotEmpty(attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
}

func (s *ServiceTestSuite) TestUpdateSystemCredentials_StampsOverLockedBlob() {
	cached := testEntity("e-cs-lock")
	locked := *cached
	locked.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.store.On("GetEntityWithCredentials", mock.Anything, cached.ID).
		Return(&entityWithCredentials{Entity: cached}, nil)
	s.store.On("UpdateSystemCredentials", mock.Anything, cached.ID, mock.Anything).Return(nil)
	s.store.On("LockEntity", mock.Anything, cached.ID).Return(locked, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, cached.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemCredentials(s.ctx, cached.ID, json.RawMessage(`{"clientSecret":"rotated"}`)))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	s.Contains(attrs, authnprovidercm.SystemAttrLinkedIDs)
}

func (s *ServiceTestSuite) TestLinkAccount_RecordsLink() {
	e := testEntity("e-link")
	s.expectLinkLock(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.JSONEq(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`, string(written))
	s.store.AssertNotCalled(s.T(), "GetEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLinkAccount_AddsSecondConnection() {
	e := testEntity("e-link-2")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-b", "sub-2"))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	links, _ := attrs[authnprovidercm.SystemAttrLinkedIDs].(map[string]interface{})
	s.Len(links, 2, "linking a second connection must not drop the first")
}

// Re-authenticating through an already-linked connection must not churn the entity.
func (s *ServiceTestSuite) TestLinkAccount_SameSubjectIsNoOp() {
	e := testEntity("e-link-3")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e, nil)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A user may hold several accounts at one connection, so a second subject is added alongside the
// first. The subjects share the identifier NAME and differ by VALUE, which the primary key keeps
// apart.
func (s *ServiceTestSuite) TestLinkAccount_AddsSecondSubjectAtSameConnection() {
	e := testEntity("e-link-4")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-other"))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	links, _ := attrs[authnprovidercm.SystemAttrLinkedIDs].(map[string]interface{})
	s.Equal(map[string]interface{}{"sub-1": map[string]interface{}{}, "sub-other": map[string]interface{}{}},
		links["idp-a"],
		"a second account at the same connection must not replace the first")
}

// expectLinkLock expects the link write to lock the entity, reading it as e, and look the pair up,
// answering with holder as the entity that already holds it (nil for none).
func (s *ServiceTestSuite) expectLinkLock(e providers.Entity, holder *string) {
	s.store.On("LockEntity", mock.Anything, e.ID).Return(e, nil).Once()
	if holder == nil {
		s.store.On("ResolveLinkedAccount", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, ErrEntityNotFound).Once()
		return
	}
	s.store.On("ResolveLinkedAccount", mock.Anything, mock.Anything, mock.Anything).
		Return(holder, nil).Once()
}

func (s *ServiceTestSuite) TestLinkAccount_RefusesPairHeldByAnotherEntity() {
	other := "e-other"
	s.expectLinkLock(*testEntity("e-link-5"), &other)

	s.ErrorIs(s.svc.LinkAccount(s.ctx, "e-link-5", "idp-a", "sub-1"), ErrLinkedAccountConflict)
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLinkAccount_RefusesAmbiguousPair() {
	s.store.On("LockEntity", mock.Anything, "e-link-6").Return(*testEntity("e-link-6"), nil).Once()
	s.store.On("ResolveLinkedAccount", mock.Anything, "idp-a", "sub-1").
		Return(nil, ErrAmbiguousEntity).Once()

	s.ErrorIs(s.svc.LinkAccount(s.ctx, "e-link-6", "idp-a", "sub-1"), ErrLinkedAccountConflict)
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// The entity's own pair resolving to itself is a repeat link, not a conflict.
func (s *ServiceTestSuite) TestLinkAccount_OwnPairIsNotAConflict() {
	e := testEntity("e-link-7")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e, &e.ID)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
}

func (s *ServiceTestSuite) TestLinkAccount_LockFailureWritesNothing() {
	lockErr := errors.New("lock failed")
	s.store.On("LockEntity", mock.Anything, "e-link-8").Return(providers.Entity{}, lockErr).Once()

	s.ErrorIs(s.svc.LinkAccount(s.ctx, "e-link-8", "idp-a", "sub-1"), lockErr)
	s.store.AssertNotCalled(s.T(), "ResolveLinkedAccount", mock.Anything, mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// Every subject is an identifier row, so a new link past the limit is refused, while a repeat of a
// subject already held still succeeds.
func (s *ServiceTestSuite) TestLinkAccount_EnforcesLinkLimit() {
	subjects := map[string]interface{}{}
	for i := 0; i < maxIndexedValuesPerAttribute; i++ {
		subjects[fmt.Sprintf("sub-%d", i)] = map[string]interface{}{}
	}
	blob, err := json.Marshal(map[string]interface{}{
		authnprovidercm.SystemAttrLinkedIDs: map[string]interface{}{"idp-a": subjects},
	})
	s.Require().NoError(err)
	e := testEntity("e-link-full")
	e.SystemAttributes = blob

	s.expectLinkLock(*e, nil)
	s.ErrorIs(s.svc.LinkAccount(s.ctx, e.ID, "idp-b", "sub-new"), ErrIndexedValueLimitExceeded)

	s.expectLinkLock(*e, nil)
	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-0"))
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A stored link entry of the wrong shape is refused rather than overwritten, which would silently
// drop whatever it held.
func (s *ServiceTestSuite) TestLinkAccount_RefusesMalformedStoredLinks() {
	for name, blob := range map[string]string{
		"linkedIds not an object":  `{"linkedIds":["idp-a"]}`,
		"connection not an object": `{"linkedIds":{"idp-a":"sub-1"}}`,
	} {
		e := testEntity("e-link-bad")
		e.SystemAttributes = json.RawMessage(blob)
		s.expectLinkLock(*e, nil)
		s.Error(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-2"), name)
	}
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A null entry is the same as no entry.
func (s *ServiceTestSuite) TestLinkAccount_NullLinksStartEmpty() {
	e := testEntity("e-link-null")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":null}`)
	s.expectLinkLock(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.JSONEq(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`, string(written))
}

func (s *ServiceTestSuite) TestLinkAccount_RejectsEmptyArguments() {
	s.ErrorIs(s.svc.LinkAccount(s.ctx, "", "idp-a", "sub-1"), ErrBadAttributesInRequest)
	s.ErrorIs(s.svc.LinkAccount(s.ctx, "e1", "", "sub-1"), ErrBadAttributesInRequest)
	s.ErrorIs(s.svc.LinkAccount(s.ctx, "e1", "idp-a", ""), ErrBadAttributesInRequest)
}

func (s *ServiceTestSuite) TestResolveLinkedAccount_Delegates() {
	expected := "e-resolved"
	s.store.On("ResolveLinkedAccount", mock.Anything, "idp-a", "sub-1").Return(&expected, nil)

	got, err := s.svc.ResolveLinkedAccount(s.ctx, "idp-a", "sub-1")
	s.Require().NoError(err)
	s.Equal(expected, *got)
}

// A subject is unique only within its issuing connection, so both parts are required and a missing
// one is a bad request rather than a broad lookup.
func (s *ServiceTestSuite) TestResolveLinkedAccount_RejectsEmptyArguments() {
	_, err := s.svc.ResolveLinkedAccount(s.ctx, "", "sub-1")
	s.ErrorIs(err, ErrBadAttributesInRequest)
	_, err = s.svc.ResolveLinkedAccount(s.ctx, "idp-a", "")
	s.ErrorIs(err, ErrBadAttributesInRequest)
	s.store.AssertNotCalled(s.T(), "ResolveLinkedAccount", mock.Anything, mock.Anything, mock.Anything)
}

// An entity with no marker recorded is written through untouched.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_NoMarkerPassesThrough() {
	e := testEntity("e-nomarker")
	e.SystemAttributes = json.RawMessage(`{"name":"Old"}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))
	s.JSONEq(`{"name":"New"}`, string(written))
}

// newServiceWithIndexedEmail returns a service whose store indexes email. Its store and entity type
// mocks carry no other expectations, so any schema, uniqueness, or write call fails the test.
func (s *ServiceTestSuite) newServiceWithIndexedEmail() (*entityService, *entityStoreInterfaceMock) {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{"email": true})
	ets := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	svc := newEntityService(store, s.hashService, ets, nil, transaction.NewNoOpTransactioner()).(*entityService)
	return svc, store
}

func (s *ServiceTestSuite) TestCreateEntity_TooManyIndexedValuesRejectedBeforeSchemaValidation() {
	svc, _ := s.newServiceWithIndexedEmail()
	e := testEntity("e1")
	e.Attributes = emailArrayAttrs(maxIndexedValuesPerAttribute + 1)

	_, err := svc.CreateEntity(s.ctx, e, nil)
	s.ErrorIs(err, ErrSchemaValidationFailed)
	s.ErrorIs(err, ErrIndexedValueLimitExceeded)
}

func (s *ServiceTestSuite) TestCreateEntity_TooManyIndexedSystemValuesRejected() {
	svc, _ := s.newServiceWithIndexedEmail()
	e := testEntity("e1")
	e.SystemAttributes = emailArrayAttrs(maxIndexedValuesPerAttribute + 1)

	_, err := svc.CreateEntity(s.ctx, e, nil)
	s.ErrorIs(err, ErrIndexedValueLimitExceeded)
}

func (s *ServiceTestSuite) TestUpdateSystemAttributes_TooManyIndexedValuesRejected() {
	svc, store := s.newServiceWithIndexedEmail()

	err := svc.UpdateSystemAttributes(s.ctx, "e1", emailArrayAttrs(maxIndexedValuesPerAttribute+1))
	s.ErrorIs(err, ErrSchemaValidationFailed)
	s.ErrorIs(err, ErrIndexedValueLimitExceeded)
	store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateSystemAttributes_ValuesAtLimitAccepted() {
	svc, store := s.newServiceWithIndexedEmail()
	attrs := emailArrayAttrs(maxIndexedValuesPerAttribute)
	store.On("LockEntity", mock.Anything, "e1").Return(*testEntity("e1"), nil)
	store.On("UpdateSystemAttributes", mock.Anything, "e1", mock.AnythingOfType("json.RawMessage")).Return(nil)

	s.NoError(svc.UpdateSystemAttributes(s.ctx, "e1", attrs))
}
