// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entitytype"
	governancemodel "github.com/thunder-id/thunderid/internal/identitygovernance/model"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

type ServiceTestSuite struct {
	suite.Suite
	store       *entityStoreInterfaceMock
	runtime     *entityRuntimeStoreInterfaceMock
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
	s.runtime = newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(s.runtime)
	s.hashService = hashmock.NewHashServiceInterfaceMock(s.T())
	// Default: hashService.Generate returns a deterministic hash for any input.
	s.hashService.On("Generate", mock.Anything).Return(cryptolib.Credential{
		Algorithm: "PBKDF2",
		Hash:      "testhash",
		Parameters: cryptolib.CredParameters{
			Salt: "testsalt", Iterations: 1, KeySize: 32,
		},
	}, nil).Maybe()
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner())
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
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(s.testErr)
	_, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Error(err)
}

func (s *ServiceTestSuite) TestCreateEntity_GetAfterCreateFails() {
	e := testEntity("e2")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr)
	_, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.Error(err)
}

func (s *ServiceTestSuite) TestCreateEntity_Success() {
	e := testEntity("e3")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.CreateEntity(s.ctx, e, nil)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *ServiceTestSuite) TestCreateEntity_RefusesADeclarativeID() {
	e := testEntity("declarative-id")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(true, nil)

	_, err := s.svc.CreateEntity(s.ctx, e, nil)

	s.ErrorIs(err, ErrAttributeConflict)
	s.store.AssertNotCalled(s.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestCreateEntity_DeclarativeCheckFails() {
	e := testEntity("declarative-check")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, s.testErr)

	_, err := s.svc.CreateEntity(s.ctx, e, nil)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestCreateEntity_GeneratedIDSkipsTheDeclarativeCheck() {
	e := testEntity("")
	s.store.On("CreateEntity", mock.Anything, mock.Anything, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, mock.Anything).Return(*testEntity("generated"), nil)

	_, err := s.svc.CreateEntity(s.ctx, e, nil)

	s.NoError(err)
	s.store.AssertNotCalled(s.T(), "IsEntityDeclarative", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestCreateEntity_RuntimeResetFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("reset-fails")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).Return(nil)
	runtime.On("ResetRuntimeData", mock.Anything, e.ID, providers.EntityStateActive).Return(s.testErr)

	_, err := svc.CreateEntity(s.ctx, e, nil)

	s.ErrorIs(err, s.testErr)
	s.store.AssertNotCalled(s.T(), "GetEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestCreateEntity_RuntimeReadFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("create-read-fails")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("ResetRuntimeData", mock.Anything, e.ID, providers.EntityStateActive).Return(nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)

	_, err := svc.CreateEntity(s.ctx, e, nil)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestDeleteEntity_RuntimeDeleteFailureIsNotReturned() {
	svc, runtime := s.newServiceWithRuntimeMock()
	s.store.On("DeleteEntity", mock.Anything, "del-runtime").Return(nil)
	runtime.On("DeleteRuntimeData", mock.Anything, "del-runtime").Return(s.testErr)

	s.NoError(svc.DeleteEntity(s.ctx, "del-runtime"), "the entity is already deleted")
}

func (s *ServiceTestSuite) TestDeleteEntity_EntityDeleteFailureKeepsTheRuntimeRow() {
	svc, runtime := s.newServiceWithRuntimeMock()
	s.store.On("DeleteEntity", mock.Anything, "del-fails").Return(s.testErr)

	s.ErrorIs(svc.DeleteEntity(s.ctx, "del-fails"), s.testErr)
	runtime.AssertNotCalled(s.T(), "DeleteRuntimeData", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestGetEntity_RuntimeReadFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-read-fails")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)

	_, err := svc.GetEntity(s.ctx, e.ID)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestGetEntity_RuntimeSeedFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-seed-fails")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil)
	runtime.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(s.testErr)

	_, err := svc.GetEntity(s.ctx, e.ID)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestGetEntity_RuntimeReadAfterSeedFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-reread-fails")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil).Once()
	runtime.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr).Once()

	_, err := svc.GetEntity(s.ctx, e.ID)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestGetEntity_RuntimeRowMissingAfterSeed() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-row-missing")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil)
	runtime.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.GetEntity(s.ctx, e.ID)

	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_RuntimeReadFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("update-read-fails")
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)

	_, err := svc.UpdateEntity(s.ctx, e.ID, e)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestListReads_RuntimeReadFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("list-read-fails")
	filters := map[string]interface{}{"email": "a@b.com"}
	s.store.On("GetEntityList", mock.Anything, "user", 10, 0, mock.Anything).Return([]providers.Entity{*e}, nil)
	s.store.On("GetEntityListByOUIDs", mock.Anything, "user", []string{"ou1"}, 10, 0, mock.Anything).
		Return([]providers.Entity{*e}, nil)
	s.store.On("SearchEntities", mock.Anything, filters).Return([]providers.Entity{*e}, nil)
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{e.ID}).Return([]providers.Entity{*e}, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)

	_, err := svc.GetEntityList(s.ctx, providers.EntityCategoryUser, 10, 0, nil)
	s.ErrorIs(err, s.testErr, "list")
	_, err = svc.GetEntityListByOUIDs(s.ctx, providers.EntityCategoryUser, []string{"ou1"}, 10, 0, nil)
	s.ErrorIs(err, s.testErr, "list by OU")
	_, err = svc.SearchEntities(s.ctx, filters)
	s.ErrorIs(err, s.testErr, "search")
	_, err = svc.GetEntitiesByIDs(s.ctx, []string{e.ID})
	s.ErrorIs(err, s.testErr, "get by IDs")
}

// newServiceWithRuntimeMock returns a service over s.store with a runtime store mock that has no
// expectations, for tests that make one runtime call fail.
func (s *ServiceTestSuite) newServiceWithRuntimeMock() (EntityServiceInterface, *entityRuntimeStoreInterfaceMock) {
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner())
	return svc, runtime
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
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_GetAfterUpdateFails() {
	e := testEntity("e6")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr).Once()
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

func (s *ServiceTestSuite) newSvcWithEntityType() (*entityService, *entitytypemock.EntityTypeServiceInterfaceMock) {
	ets := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	svc := newEntityService(s.store, s.runtime, s.hashService, ets, nil, transaction.NewNoOpTransactioner())
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
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(s.testErr)
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateEntity_GetAfterUpdateFails_ViaOldPath() {
	e := testEntity("uc3")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateEntity", mock.Anything, e).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr).Once()
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

func (s *ServiceTestSuite) TestAuthenticateEntityByID_DoesNotCheckProfileState() {
	e := testEntity("inactive-1")
	e.State = providers.EntityState("SUSPENDED")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)

	s.hashService.On("Verify", []byte("p"), mock.Anything).Return(true, nil)
	result, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "p"})
	s.NoError(err)
	s.Require().NotNil(result)
	s.Equal(e.ID, result.EntityID)
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
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)

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

// An entity with no marker recorded is written through untouched.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_NoMarkerPassesThrough() {
	e := testEntity("e-nomarker")
	e.SystemAttributes = json.RawMessage(`{"name":"Old"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))
	s.JSONEq(`{"name":"New"}`, string(written))
}

// suspendedEntity returns a suspended entity.
func suspendedEntity(id string) *providers.Entity {
	e := testEntity(id)
	e.State = providers.EntityStateSuspended
	return e
}

// A suspended record stays writable, so an operator can remediate it.
func (s *ServiceTestSuite) TestSuspendedEntityStillAcceptsWrites() {
	e := suspendedEntity("e-held")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Maybe()
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil).Maybe()
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).Return(nil)
	s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.Anything).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)),
		"an operator must be able to correct a held record")
	s.NoError(s.svc.UpdateSystemCredentials(s.ctx, e.ID, json.RawMessage(`{"clientSecret":"s3cret"}`)),
		"rotating the exposed secret is the remediation the hold exists to allow")
}

func (s *ServiceTestSuite) TestActiveEntityStillAcceptsWrites() {
	e := testEntity("e-open")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Maybe()
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).Return(nil).Once()

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))
}

// An accessState key in system attributes is profile data and does not reach the runtime row.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_DoesNotTouchRuntimeState() {
	e := testEntity("e-runtime")
	e.SystemAttributes = json.RawMessage(`{"name":"Old"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.AnythingOfType("json.RawMessage")).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	sent := json.RawMessage(`{"name":"New","accessState":{"suspend":{}}}`)
	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, sent))

	s.JSONEq(string(sent), string(written))
	s.runtime.AssertNotCalled(s.T(), "SetAccessSuspension", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
	s.runtime.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

// installRuntimeFixture stubs the runtime store for service tests. SQL behavior is tested against
// SQLite.
func installRuntimeFixture(store *entityRuntimeStoreInterfaceMock) {
	records := map[string]entityRuntimeData{}
	store.On("GetRuntimeData", mock.Anything, mock.Anything).Return(
		func(_ context.Context, ids []string) (map[string]entityRuntimeData, error) {
			result := map[string]entityRuntimeData{}
			for _, id := range ids {
				if row, ok := records[id]; ok {
					result[id] = row
				}
			}
			return result, nil
		}).Maybe()
	store.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(
		func(_ context.Context, rows []entityRuntimeData) error {
			for _, row := range rows {
				if _, ok := records[row.ID]; !ok {
					records[row.ID] = row
				}
			}
			return nil
		}).Maybe()
	store.On("ResetRuntimeData", mock.Anything, mock.Anything, mock.Anything).Return(
		func(_ context.Context, id string, state providers.EntityState) error {
			if state == "" {
				state = providers.EntityStateActive
			}
			records[id] = entityRuntimeData{ID: id, State: state, Attributes: json.RawMessage(`{}`)}
			return nil
		}).Maybe()
	store.On("DeleteRuntimeData", mock.Anything, mock.Anything).Return(
		func(_ context.Context, id string) error {
			delete(records, id)
			return nil
		}).Maybe()
}

// governedRead stubs the profile row and a runtime row at revision 7 for one GetGovernedEntity.
func (s *ServiceTestSuite) governedRead(e *providers.Entity, state providers.EntityState, runtimeAttrs string) {
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	row := entityRuntimeData{ID: e.ID, State: state, Revision: 7}
	if runtimeAttrs != "" {
		row.Attributes = json.RawMessage(runtimeAttrs)
	}
	s.runtime = newEntityRuntimeStoreInterfaceMock(s.T())
	s.runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).
		Return(map[string]entityRuntimeData{e.ID: row}, nil).Once()
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner())
}

// The governed read carries the category, the parsed lock state and the runtime row's revision.
func (s *ServiceTestSuite) TestGetGovernedEntityReadsTheRuntimeRow() {
	e := testEntity("e-gov")
	e.OUID = "ou-1"
	s.governedRead(e, providers.EntityStateActive, `{"accessState":{"lock":{"authenticationMethods":`+
		`{"credential":{"failureCount":3,"lockCount":2,"unlockAt":"2026-09-08T10:15:00Z",`+
		`"lastFailedAt":"2026-09-08T10:00:00Z"}}}}}`)

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(e.ID, governed.ID)
	s.Equal(e.Category, governed.Category)
	s.Equal(e.Type, governed.Type)
	s.Equal("ou-1", governed.OUID)
	s.Equal(providers.EntityStateActive, governed.State)

	credential := governed.AccessState.Lock.AuthenticationMethods[governancemodel.AccessScopeCredential]
	s.Equal(int64(7), governed.Revision)
	s.Equal(3, credential.FailureCount)
	s.Equal(2, credential.LockCount)
	s.Equal("2026-09-08T10:15:00Z", credential.UnlockAt)
	s.Equal("2026-09-08T10:00:00Z", credential.LastFailedAt)
}

func (s *ServiceTestSuite) TestGetGovernedEntityParsesSuspensionAndEntityHold() {
	e := testEntity("e-gov-held")
	s.governedRead(e, providers.EntityStateSuspended, `{"accessState":{"suspend":`+
		`{"suspendedAt":"2026-09-10T11:02:00Z","operatorNote":"Credential exposure"},`+
		`"lock":{"entity":{"unlockAt":"9999-12-31T23:59:59Z","reason":"POST_SUSPENDED"}},`+
		`"lastLoginAt":"2026-09-09T18:22:04Z"}}`)

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateSuspended, governed.State)
	s.Require().NotNil(governed.AccessState.Suspend)
	s.Equal("2026-09-10T11:02:00Z", governed.AccessState.Suspend.SuspendedAt)
	s.Equal("Credential exposure", governed.AccessState.Suspend.OperatorNote)
	s.Equal("2026-09-09T18:22:04Z", governed.AccessState.LastLoginAt)
	s.Require().NotNil(governed.AccessState.Lock.Entity)
	s.Equal(int64(7), governed.Revision)
	s.Equal(governancemodel.PermanentUnlockAt, governed.AccessState.Lock.Entity.UnlockAt)
	s.Equal(governancemodel.ReasonPostSuspension, governed.AccessState.Lock.Entity.Reason)
	s.Empty(governed.AccessState.Lock.AuthenticationMethods)
}

func (s *ServiceTestSuite) TestGetGovernedEntityWithNoAccessState() {
	for _, attrs := range []string{"", `{"name":"only"}`} {
		e := testEntity("e-gov-empty")
		s.governedRead(e, providers.EntityStateActive, attrs)

		governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID)
		s.Require().NoError(err)
		s.Equal(governancemodel.AccessState{}, governed.AccessState)
	}
}

// An unknown scope is kept and an unknown member is ignored.
func (s *ServiceTestSuite) TestGetGovernedEntityToleratesUnknownKeys() {
	e := testEntity("e-gov-future")
	s.governedRead(e, providers.EntityStateActive, `{"accessState":{"somethingLater":{"x":1},`+
		`"lock":{"authenticationMethods":{"some_future_method":{"failureCount":2}}}}}`)

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID)
	s.Require().NoError(err)
	s.Equal(2, governed.AccessState.Lock.AuthenticationMethods["some_future_method"].FailureCount)
}

// An access-state document that cannot be parsed fails the read.
func (s *ServiceTestSuite) TestGetGovernedEntityRefusesAnUnparsableDocument() {
	e := testEntity("e-gov-bad")
	s.governedRead(e, providers.EntityStateActive, `not json`)

	_, err := s.svc.GetGovernedEntity(s.ctx, e.ID)
	s.Error(err)
}

func (s *ServiceTestSuite) TestGetGovernedEntityPropagatesAReadFailure() {
	s.store.On("GetEntity", mock.Anything, "missing").
		Return(providers.Entity{}, ErrEntityNotFound).Once()

	_, err := s.svc.GetGovernedEntity(s.ctx, "missing")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestGetEntityProfileDoesNotHydrateTheRuntimeRow() {
	e := testEntity("e-category")
	e.Category = providers.EntityCategoryAgent
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()

	profile, err := s.svc.GetEntityProfile(s.ctx, e.ID)

	s.Require().NoError(err)
	s.Equal(providers.EntityCategoryAgent, profile.Category)
	s.runtime.AssertNotCalled(s.T(), "GetRuntimeData", mock.Anything, mock.Anything)
}

// A read failure is returned as an error, never as a zero-value profile.
func (s *ServiceTestSuite) TestGetEntityProfilePropagatesAReadFailure() {
	s.store.On("GetEntity", mock.Anything, "missing").
		Return(providers.Entity{}, ErrEntityNotFound).Once()

	_, err := s.svc.GetEntityProfile(s.ctx, "missing")

	s.Require().ErrorIs(err, ErrEntityNotFound)
}

// newServiceWithIndexedEmail returns a service whose store indexes email. Its store and entity type
// mocks carry no other expectations, so any schema, uniqueness, or write call fails the test.
func (s *ServiceTestSuite) newServiceWithIndexedEmail() (*entityService, *entityStoreInterfaceMock) {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{"email": true})
	ets := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	svc := newEntityService(store, runtime, s.hashService, ets, nil,
		transaction.NewNoOpTransactioner()).(*entityService)
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
	store.On("GetEntity", mock.Anything, "e1").Return(*testEntity("e1"), nil)
	store.On("UpdateSystemAttributes", mock.Anything, "e1", mock.AnythingOfType("json.RawMessage")).Return(nil)

	s.NoError(svc.UpdateSystemAttributes(s.ctx, "e1", attrs))
}
