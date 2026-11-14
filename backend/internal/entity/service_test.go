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
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
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

// impl returns the service under test as its implementation.
func (s *ServiceTestSuite) impl() *entityService {
	return s.svc.(*entityService)
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
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(), nil, nil)
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
	_, err := s.svc.CreateEntity(s.ctx, nil, nil, nil)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestCreateEntity_StoreCreateFails() {
	e := testEntity("e1")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(s.testErr)
	_, err := s.svc.CreateEntity(s.ctx, e, nil, nil)
	s.Error(err)
}

func (s *ServiceTestSuite) TestCreateEntity_GetAfterCreateFails() {
	e := testEntity("e2")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr)
	_, err := s.svc.CreateEntity(s.ctx, e, nil, nil)
	s.Error(err)
}

func (s *ServiceTestSuite) TestCreateEntity_Success() {
	e := testEntity("e3")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	got, err := s.svc.CreateEntity(s.ctx, e, nil, nil)
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

// A new entity's link is written as its linkedIds whether or not other entities hold the subject.
func (s *ServiceTestSuite) TestCreateEntity_LinkedAccount() {
	e := testEntity("e-linked")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound).Once()
	s.store.On("CreateEntity", mock.Anything, mock.MatchedBy(func(got providers.Entity) bool {
		return string(got.SystemAttributes) == `{"linkedIds":{"idp-a":{"sub-1":{}}}}`
	}), json.RawMessage(nil), json.RawMessage(nil)).Return(nil).Once()
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()

	_, err := s.svc.CreateEntity(s.ctx, e, nil, &providers.LinkedAccount{IdpID: "idp-a", Sub: "sub-1"})
	s.NoError(err)
	s.store.AssertNotCalled(s.T(), "ResolveLinkedAccounts", mock.Anything, mock.Anything, mock.Anything)
}

// A caller-supplied id a declarative account holds is refused, so its runtime row is not reset.
func (s *ServiceTestSuite) TestCreateEntity_RefusesADeclarativeID() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("declarative-id")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(true, nil)

	_, err := svc.CreateEntity(s.ctx, e, nil, nil)

	s.ErrorIs(err, ErrAttributeConflict)
	s.store.AssertNotCalled(s.T(), "CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	runtime.AssertNotCalled(s.T(), "ResetRuntimeData", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestCreateEntity_DeclarativeCheckFails() {
	e := testEntity("declarative-check")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, s.testErr)

	_, err := s.svc.CreateEntity(s.ctx, e, nil, nil)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestCreateEntity_GeneratedIDSkipsTheDeclarativeCheck() {
	e := testEntity("")
	s.store.On("CreateEntity", mock.Anything, mock.Anything, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil)
	s.store.On("GetEntity", mock.Anything, mock.Anything).Return(*testEntity("generated"), nil)

	_, err := s.svc.CreateEntity(s.ctx, e, nil, nil)

	s.NoError(err)
	s.store.AssertNotCalled(s.T(), "IsEntityDeclarative", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestCreateEntity_RuntimeResetFailureIsNotReturned() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("reset-fails")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("ResetRuntimeData", mock.Anything, e.ID, providers.EntityStateActive).Return(s.testErr)

	created, err := svc.CreateEntity(s.ctx, e, nil, nil)

	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, created.State)
}

// A created entity is returned with the runtime state just written, without reading it back, so a
// committed create never reports a failure.
func (s *ServiceTestSuite) TestCreateEntity_DoesNotReadTheRuntimeRowBack() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("create-no-read")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).Return(nil)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("ResetRuntimeData", mock.Anything, e.ID, providers.EntityStateActive).Return(nil)

	created, err := svc.CreateEntity(s.ctx, e, nil, nil)

	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, created.State)
	s.JSONEq(`{}`, string(created.RuntimeAttributes))
	runtime.AssertNotCalled(s.T(), "GetRuntimeData", mock.Anything, mock.Anything)
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

// A read reports a missing runtime row as the row the entity starts with, and writes nothing.
func (s *ServiceTestSuite) TestGetEntity_MissingRuntimeRowReadsAsActiveWithoutSeeding() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-unseeded")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil)

	got, err := svc.GetEntity(s.ctx, e.ID)

	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, got.State)
	s.JSONEq(`{}`, string(got.RuntimeAttributes))
	runtime.AssertNotCalled(s.T(), "InitializeRuntimeData", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestGetGovernedEntity_RuntimeSeedFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-seed-fails")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil)
	runtime.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(s.testErr)

	_, err := svc.GetGovernedEntity(s.ctx, e.ID, true)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestGetGovernedEntity_RuntimeReadAfterSeedFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-reread-fails")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil).Once()
	runtime.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr).Once()

	_, err := svc.GetGovernedEntity(s.ctx, e.ID, true)

	s.ErrorIs(err, s.testErr)
}

func (s *ServiceTestSuite) TestGetGovernedEntity_RuntimeRowMissingAfterSeed() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("get-row-missing")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil)
	runtime.On("InitializeRuntimeData", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.GetGovernedEntity(s.ctx, e.ID, true)

	s.Error(err)
}

// Without seedMissing, a missing runtime row reads as the state the entity starts with, the
// declarative one included, and no row is created.
func (s *ServiceTestSuite) TestGetGovernedEntity_WithoutSeedingAMissingRowReadsAsTheStartingState() {
	for name, state := range map[string]providers.EntityState{
		"database":    "",
		"declarative": providers.EntityStateSuspended,
	} {
		svc, runtime := s.newServiceWithRuntimeMock()
		e := testEntity("unseeded-" + name)
		e.State = state
		s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
		runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{}, nil)

		got, err := svc.GetGovernedEntity(s.ctx, e.ID, false)

		s.Require().NoError(err, name)
		want := state
		if want == "" {
			want = providers.EntityStateActive
		}
		s.Equal(want, got.State, name)
		runtime.AssertNotCalled(s.T(), "InitializeRuntimeData", mock.Anything, mock.Anything)
	}
}

func (s *ServiceTestSuite) TestGetGovernedEntity_WithoutSeedingReadsTheStoredRow() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("unseeded-row")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{
		e.ID: {ID: e.ID, State: providers.EntityStateActive, Revision: 7,
			Attributes: json.RawMessage(`{"accessState":{"suspend":{"suspendedAt":"2026-10-19T10:00:00Z"}}}`)},
	}, nil)

	got, err := svc.GetGovernedEntity(s.ctx, e.ID, false)

	s.Require().NoError(err)
	s.Equal(providers.EntityStateActive, got.State)
	s.Require().NotNil(got.AccessState.Suspend, "the stored suspension member must be read")
	s.Equal(int64(7), got.Revision, "the stored row's revision must be carried")
}

func (s *ServiceTestSuite) TestGetGovernedEntity_WithoutSeedingFailures() {
	svc, runtime := s.newServiceWithRuntimeMock()
	s.store.On("GetEntity", mock.Anything, "unseeded-missing").Return(providers.Entity{}, ErrEntityNotFound)
	governed, err := svc.GetGovernedEntity(s.ctx, "unseeded-missing", false)
	s.NoError(err)
	s.Nil(governed, "an unknown entity has nothing to govern")

	e := testEntity("unseeded-unreadable")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{
		e.ID: {ID: e.ID, State: providers.EntityStateActive, Attributes: json.RawMessage(`{"accessState":`)},
	}, nil)
	governed, err = svc.GetGovernedEntity(s.ctx, e.ID, false)
	s.Error(err, "an unreadable document is a fault, not a hold")
	s.Nil(governed)
}

// A failed runtime read fails the update before anything is written.
func (s *ServiceTestSuite) TestUpdateEntity_RuntimeReadFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("update-read-fails")
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr).Once()

	updated, err := svc.UpdateEntity(s.ctx, e.ID, e)

	s.ErrorIs(err, s.testErr)
	s.Nil(updated)
	s.store.AssertNotCalled(s.T(), "LockEntity", mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "UpdateEntity", mock.Anything, mock.Anything)
}

// The runtime row is read once, before the transaction, and the updated entity carries it; an entity
// with no row reads as the row it starts with.
func (s *ServiceTestSuite) TestUpdateEntity_AppliesTheRuntimeRowReadBeforeTheTransaction() {
	for name, tc := range map[string]struct {
		rows  map[string]entityRuntimeData
		state providers.EntityState
		attrs string
	}{
		"stored row": {map[string]entityRuntimeData{"update-runtime": {ID: "update-runtime",
			State: providers.EntityStateSuspended, Attributes: json.RawMessage(`{"accessState":{"suspend":{}}}`)}},
			providers.EntityStateSuspended, `{"accessState":{"suspend":{}}}`},
		"no row": {map[string]entityRuntimeData{}, providers.EntityStateActive, `{}`},
	} {
		s.Run(name, func() {
			s.SetupTest()
			e := testEntity("update-runtime")
			runtime := newEntityRuntimeStoreInterfaceMock(s.T())
			read := runtime.On("GetRuntimeData", s.ctx, []string{e.ID}).Return(tc.rows, nil).Once()
			transactioner := transactionmock.NewTransactionerMock(s.T())
			transactioner.EXPECT().Transact(s.ctx, mock.Anything).RunAndReturn(
				func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
				NotBefore(read).Once()
			stored := *e
			stored.State = ""
			s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
			s.store.On("UpdateEntity", mock.Anything, e).Return(nil).Once()
			s.store.On("GetEntity", mock.Anything, e.ID).Return(stored, nil).Once()
			svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transactioner, nil, nil)

			updated, err := svc.UpdateEntity(s.ctx, e.ID, e)

			s.Require().NoError(err)
			s.Equal(tc.state, updated.State)
			s.JSONEq(tc.attrs, string(updated.RuntimeAttributes))
		})
	}
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

// A governed read seeds a missing row with the state the entity starts with, a declarative one kept
// even when this build does not define it, and returns the row read back, so a row another request
// wrote first wins over the seed.
func (s *ServiceTestSuite) TestGetGovernedEntity_SeedsAMissingRowAndReadsItBack() {
	for name, tc := range map[string]struct{ declared, seeded providers.EntityState }{
		"database":              {"", providers.EntityStateActive},
		"declarative suspended": {providers.EntityStateSuspended, providers.EntityStateSuspended},
		"declarative unknown":   {"SOME_FUTURE_STATE", "SOME_FUTURE_STATE"},
	} {
		s.Run(name, func() {
			s.SetupTest()
			svc, runtime := s.newServiceWithRuntimeMock()
			e := testEntity("seeded")
			e.State = tc.declared
			s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
			read := runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).
				Return(map[string]entityRuntimeData{}, nil).Once()
			seed := runtime.On("InitializeRuntimeData", mock.Anything, []entityRuntimeData{
				{ID: e.ID, State: tc.seeded, Attributes: json.RawMessage(`{}`)},
			}).Return(nil).NotBefore(read).Once()
			runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(map[string]entityRuntimeData{
				e.ID: {ID: e.ID, State: providers.EntityStateActive, Revision: 4,
					Attributes: json.RawMessage(`{"accessState":{"lastLoginAt":"2026-09-08T10:00:00Z"}}`)},
			}, nil).NotBefore(seed).Once()

			governed, err := svc.GetGovernedEntity(s.ctx, e.ID, true)

			s.Require().NoError(err)
			s.Equal(providers.EntityStateActive, governed.State, "the row read back wins over the seed")
			s.Equal(int64(4), governed.Revision)
			s.Equal("2026-09-08T10:00:00Z", governed.AccessState.LastLoginAt)
		})
	}
}

// A list read hydrates every entity from one runtime read and never seeds: an entity with no row reads
// as the row it starts with, a declarative state included.
func (s *ServiceTestSuite) TestListReads_HydrateFromOneReadWithoutSeeding() {
	svc, runtime := s.newServiceWithRuntimeMock()
	stored, unseeded, declarative := testEntity("l-stored"), testEntity("l-unseeded"), testEntity("l-declarative")
	stored.State, unseeded.State = "", ""
	declarative.State = "SOME_FUTURE_STATE"
	ids := []string{stored.ID, unseeded.ID, declarative.ID}
	s.store.On("GetEntitiesByIDs", mock.Anything, ids).
		Return([]providers.Entity{*stored, *unseeded, *declarative}, nil).Once()
	runtime.On("GetRuntimeData", mock.Anything, ids).Return(map[string]entityRuntimeData{
		stored.ID: {ID: stored.ID, State: providers.EntityStateSuspended,
			Attributes: json.RawMessage(`{"accessState":{"suspend":{}}}`)},
	}, nil).Once()

	entities, err := svc.GetEntitiesByIDs(s.ctx, ids)

	s.Require().NoError(err)
	s.Require().Len(entities, 3)
	s.Equal(providers.EntityStateSuspended, entities[0].State)
	s.JSONEq(`{"accessState":{"suspend":{}}}`, string(entities[0].RuntimeAttributes))
	s.Equal(providers.EntityStateActive, entities[1].State)
	s.JSONEq(`{}`, string(entities[1].RuntimeAttributes))
	s.Equal(providers.EntityState("SOME_FUTURE_STATE"), entities[2].State)
	runtime.AssertNotCalled(s.T(), "InitializeRuntimeData", mock.Anything, mock.Anything)
}

// An empty list reads no runtime state.
func (s *ServiceTestSuite) TestListReads_AnEmptyListReadsNoRuntimeState() {
	svc, runtime := s.newServiceWithRuntimeMock()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"none"}).Return([]providers.Entity{}, nil).Once()

	entities, err := svc.GetEntitiesByIDs(s.ctx, []string{"none"})

	s.Require().NoError(err)
	s.Empty(entities)
	runtime.AssertNotCalled(s.T(), "GetRuntimeData", mock.Anything, mock.Anything)
}

// A failed runtime read fails the governed read before anything is seeded.
func (s *ServiceTestSuite) TestGetGovernedEntity_RuntimeReadFails() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("governed-read-fails")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr).Once()

	governed, err := svc.GetGovernedEntity(s.ctx, e.ID, true)

	s.ErrorIs(err, s.testErr)
	s.Nil(governed)
	runtime.AssertNotCalled(s.T(), "InitializeRuntimeData", mock.Anything, mock.Anything)
}

// Each governance write reaches the runtime store with its arguments and returns the store's answer.
func (s *ServiceTestSuite) TestGovernanceWritesGoToTheRuntimeStore() {
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	observed := identitygovernance.LockEntry{FailureCount: 5, Revision: 3}
	episode := identitygovernance.LockEpisode{LockCount: 1, UnlockAt: "2026-09-08T10:05:00Z"}
	hold := &identitygovernance.AccountHold{UnlockAt: testPermanentUnlockAt}
	methods := []providers.AuthenticationMethod{providers.AuthenticationMethodCredential}
	for name, tc := range map[string]struct {
		expect func(runtime *entityRuntimeStoreInterfaceMock)
		call   func(svc EntityServiceInterface) (any, error)
		want   any
	}{
		"increment": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("IncrementAccessFailure", s.ctx, "e1", providers.AuthenticationMethodOTP, now,
					now.Add(-time.Minute)).
					Return(observed, true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) {
				lock, recorded, err := svc.IncrementFailure(s.ctx, "e1", providers.AuthenticationMethodOTP, now,
					now.Add(-time.Minute))
				return []any{lock, recorded}, err
			},
			[]any{observed, true},
		},
		"form lock": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("FormAccessLock", s.ctx, "e1", providers.AuthenticationMethodOTP, observed, episode, now).
					Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) {
				return svc.FormLock(s.ctx, "e1", providers.AuthenticationMethodOTP, observed, episode, now)
			},
			true,
		},
		"clear": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("ClearAccessState", s.ctx, "e1", methods).Return(s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) { return nil, svc.ClearAccessState(s.ctx, "e1", methods) },
			nil,
		},
		"guarded clear": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("ClearAccessStateIfUnchanged", s.ctx, "e1", providers.AuthenticationMethodOTP, int64(3)).
					Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) {
				return svc.ClearAccessStateIfUnchanged(s.ctx, "e1", providers.AuthenticationMethodOTP, 3)
			},
			true,
		},
		"record login": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("RecordAccessLogin", s.ctx, "e1", now, now.Add(-time.Hour)).Return(s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) {
				return nil, svc.RecordLogin(s.ctx, "e1", now, now.Add(-time.Hour))
			},
			nil,
		},
		"suspend": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("SetAccessSuspension", s.ctx, "e1", now, "note").Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) { return svc.SetSuspension(s.ctx, "e1", now, "note") },
			true,
		},
		"owner suspend": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("SetAccessOwnerSuspension", s.ctx, "e1", now).Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) { return svc.SetOwnerSuspension(s.ctx, "e1", now) },
			true,
		},
		"owner release": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("ClearAccessOwnerSuspension", s.ctx, "e1").Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) { return svc.ClearOwnerSuspension(s.ctx, "e1") },
			true,
		},
		"cascade suspend": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("SetAccessCascadeSuspension", s.ctx, "e1", now).Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) { return svc.SetCascadeSuspension(s.ctx, "e1", now) },
			true,
		},
		"cascade release": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("ClearAccessCascadeSuspension", s.ctx, "e1").Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) { return svc.ClearCascadeSuspension(s.ctx, "e1") },
			true,
		},
		"release": {
			func(r *entityRuntimeStoreInterfaceMock) {
				r.On("ClearAccessSuspension", s.ctx, "e1", hold, "2026-09-08T09:00:00Z").Return(true, s.testErr).Once()
			},
			func(svc EntityServiceInterface) (any, error) {
				return svc.ClearSuspension(s.ctx, "e1", hold, "2026-09-08T09:00:00Z")
			},
			true,
		},
	} {
		s.Run(name, func() {
			svc, runtime := s.newServiceWithRuntimeMock()
			tc.expect(runtime)

			got, err := tc.call(svc)

			s.ErrorIs(err, s.testErr)
			s.Equal(tc.want, got)
		})
	}
}

// transactionMarker is the key passingTransactioner marks its transaction's context with.
type transactionMarker struct{}

// passingTransactioner runs one transaction with a marked context. It records what the transaction
// returned, which a database would commit or roll back on, and appends "transaction ended" to events.
func (s *ServiceTestSuite) passingTransactioner(result *error,
	events *[]string) (*transactionmock.TransactionerMock, context.Context) {
	txCtx := context.WithValue(s.ctx, transactionMarker{}, true)
	transactioner := transactionmock.NewTransactionerMock(s.T())
	transactioner.EXPECT().Transact(s.ctx, mock.Anything).RunAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			*result = fn(txCtx)
			*events = append(*events, "transaction ended")
			return *result
		}).Once()
	return transactioner, txCtx
}

// The runtime row is written after the create's transaction ends, always ACTIVE whatever the caller
// sends.
func (s *ServiceTestSuite) TestCreateEntity_WritesTheRuntimeRowAfterTheCommit() {
	var committed error
	var events []string
	transactioner, txCtx := s.passingTransactioner(&committed, &events)
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transactioner, nil, nil)
	e := testEntity("create-after")
	e.State = providers.EntityStateSuspended
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound).Once()
	s.store.On("CreateEntity", txCtx, mock.Anything, json.RawMessage(nil), json.RawMessage(nil)).
		Return(nil).Once()
	s.store.On("GetEntity", txCtx, e.ID).Return(*e, nil).Once()
	runtime.On("ResetRuntimeData", s.ctx, e.ID, providers.EntityStateActive).
		Run(func(mock.Arguments) { events = append(events, "runtime row written") }).Return(nil).Once()

	created, err := svc.CreateEntity(s.ctx, e, nil, nil)

	s.Require().NoError(err)
	s.NoError(committed)
	s.Equal([]string{"transaction ended", "runtime row written"}, events)
	s.Equal(providers.EntityStateActive, created.State)
}

// A failed create writes no runtime row.
func (s *ServiceTestSuite) TestCreateEntity_FailedCreateWritesNoRuntimeRow() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("create-fails")
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, *e, json.RawMessage(nil), json.RawMessage(nil)).Return(s.testErr)

	_, err := svc.CreateEntity(s.ctx, e, nil, nil)

	s.ErrorIs(err, s.testErr)
	runtime.AssertNotCalled(s.T(), "ResetRuntimeData", mock.Anything, mock.Anything, mock.Anything)
}

// The runtime row is deleted after the entity delete's transaction ends, outside it.
func (s *ServiceTestSuite) TestDeleteEntity_DeletesTheRuntimeRowAfterTheCommit() {
	var committed error
	var events []string
	transactioner, txCtx := s.passingTransactioner(&committed, &events)
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transactioner, nil, nil)
	s.store.On("DeleteEntity", txCtx, "del-after").Return(nil).Once()
	runtime.On("DeleteRuntimeData", s.ctx, "del-after").
		Run(func(mock.Arguments) { events = append(events, "runtime row deleted") }).Return(nil).Once()

	s.Require().NoError(svc.DeleteEntity(s.ctx, "del-after"))
	s.NoError(committed)
	s.Equal([]string{"transaction ended", "runtime row deleted"}, events)
}

// newServiceWithRuntimeMock returns a service over s.store with a runtime store mock that has no
// expectations, for tests that make one runtime call fail.
func (s *ServiceTestSuite) newServiceWithRuntimeMock() (EntityServiceInterface, *entityRuntimeStoreInterfaceMock) {
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(), nil, nil)
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
	svc := newEntityService(s.store, s.runtime, s.hashService, ets, nil, transaction.NewNoOpTransactioner(), nil, nil)
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
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	// strip: all declared attributes (username only).
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}}, nil)
	// credential extraction: no credential attributes.
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type, entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{}, nil)
	// verifiable attributes: none.
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type, entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}}, nil)
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
	// verifiable attributes: none.
	ets.On("GetAttributes", mock.Anything, mock.Anything, e.Type, entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "username"}}, nil)
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
	s.store.AssertNotCalled(s.T(), "LockEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateAttributes_LockEntityFails() {
	e := testEntity("ua-lock")
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("LockEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr).Once()
	err := s.svc.UpdateAttributes(s.ctx, e.ID, json.RawMessage(`{"username":"new"}`))
	s.ErrorIs(err, s.testErr)
	s.store.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateAttributes_RefusesATypeChangedBeforeTheLock() {
	e := testEntity("ua-type")
	locked := *e
	locked.Type = "other"
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("LockEntity", mock.Anything, e.ID).Return(locked, nil).Once()
	err := s.svc.UpdateAttributes(s.ctx, e.ID, json.RawMessage(`{"username":"new"}`))
	s.ErrorIs(err, ErrSchemaValidationFailed)
	s.store.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateEntity_LockEntityFails() {
	e := testEntity("ue-lock")
	s.store.On("LockEntity", mock.Anything, e.ID).Return(providers.Entity{}, s.testErr).Once()
	_, err := s.svc.UpdateEntity(s.ctx, e.ID, e)
	s.ErrorIs(err, s.testErr)
	s.store.AssertNotCalled(s.T(), "UpdateEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateSystemAttributes_LockEntityFails() {
	s.store.On("LockEntity", mock.Anything, "usa-lock").Return(providers.Entity{}, s.testErr).Once()
	err := s.svc.UpdateSystemAttributes(s.ctx, "usa-lock", json.RawMessage(`{"keep":true}`))
	s.ErrorIs(err, s.testErr)
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateAttributes_StoreFails() {
	e := testEntity("ua1")
	attrs := json.RawMessage(`{"username":"new"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateAttributes", mock.Anything, e.ID, attrs).Return(s.testErr)
	err := s.svc.UpdateAttributes(s.ctx, e.ID, attrs)
	s.Error(err)
}

func (s *ServiceTestSuite) TestUpdateAttributes_Success() {
	e := testEntity("ua2")
	attrs := json.RawMessage(`{"username":"new"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
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
	s.store.On("UpdateSystemCredentials", mock.Anything, "e1", mock.IsType(json.RawMessage(nil))).Return(nil)
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
	s.store.On("UpdateCredentials", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).Return(nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Run(func(args mock.Arguments) {
			written, _ = args.Get(2).(json.RawMessage)
		}).Return(nil)

	s.NoError(s.svc.UpdateCredentials(s.ctx, e.ID, json.RawMessage(`{"password":"new-secret"}`)))

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(written, &attrs))
	s.Equal("App name", attrs["name"], "unrelated system attributes must survive the stamp")
	s.NotEmpty(attrs[authnprovidercm.SystemAttrCredentialUpdatedAt])
}

// testCredentialEntityType is the entity type the credential schema tests validate against.
const testCredentialEntityType = "employee"

func (s *ServiceTestSuite) TestUpdateCredentials_RejectsValueFailingSchema() {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity("e-weak")
	e.Category = providers.EntityCategoryUser
	e.Type = testCredentialEntityType
	creds := json.RawMessage(`{"password":"1"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	ets.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testCredentialEntityType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password", Credential: true}}, nil)
	ets.On("ValidateEntityValues", mock.Anything, entitytype.TypeCategoryUser, testCredentialEntityType, creds).
		Return(false, nil)

	err := svc.UpdateCredentials(s.ctx, e.ID, creds)

	s.ErrorIs(err, ErrSchemaValidationFailed)
	s.store.AssertNotCalled(s.T(), "UpdateCredentials", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateCredentials_SchemaLookupFails() {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity("e-lookup")
	e.Category = providers.EntityCategoryUser
	e.Type = testCredentialEntityType
	creds := json.RawMessage(`{"password":"long-enough"}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	ets.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testCredentialEntityType,
		entitytype.AttributeFilter{AllowCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "password", Credential: true}}, nil)
	ets.On("ValidateEntityValues", mock.Anything, entitytype.TypeCategoryUser, testCredentialEntityType, creds).
		Return(false, &tidcommon.InternalServerError)

	err := svc.UpdateCredentials(s.ctx, e.ID, creds)

	s.Error(err)
	s.NotErrorIs(err, ErrSchemaValidationFailed)
	s.store.AssertNotCalled(s.T(), "UpdateCredentials", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestValidateCredentials() {
	// reaches is how far each payload gets: 0 stops at parsing, 1 checks the keys against the entity's
	// type, and 2 also checks the values against it.
	for name, test := range map[string]struct {
		creds   string
		reaches int
		valid   bool
		wantIs  error
	}{
		"valid value":       {creds: `{"password":"long-enough"}`, reaches: 2, valid: true},
		"value fails":       {creds: `{"password":"1"}`, reaches: 2, wantIs: ErrSchemaValidationFailed},
		"undeclared key":    {creds: `{"pin":"1234"}`, reaches: 1, wantIs: ErrInvalidCredential},
		"empty value":       {creds: `{"password":" "}`, wantIs: ErrInvalidCredential},
		"unparseable input": {creds: `not json`, wantIs: ErrInvalidCredential},
	} {
		s.Run(name, func() {
			s.SetupTest()
			svc, ets := s.newSvcWithEntityType()
			e := testEntity("e-validate")
			e.Category = providers.EntityCategoryUser
			e.Type = testCredentialEntityType
			creds := json.RawMessage(test.creds)
			if test.reaches >= 1 {
				s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
				ets.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, testCredentialEntityType,
					entitytype.AttributeFilter{AllowCredential: true}).
					Return([]entitytype.AttributeInfo{{Attribute: "password", Credential: true}}, nil).Once()
			}
			if test.reaches >= 2 {
				ets.On("ValidateEntityValues", mock.Anything, entitytype.TypeCategoryUser, testCredentialEntityType,
					creds).Return(test.valid, nil).Once()
			}

			err := svc.ValidateCredentials(s.ctx, e.ID, creds)

			if test.wantIs == nil {
				s.NoError(err)
			} else {
				s.ErrorIs(err, test.wantIs)
			}
			s.store.AssertNotCalled(s.T(), "UpdateCredentials", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A client secret rotation stamps the entity, in the same transaction as the credential write.
func (s *ServiceTestSuite) TestUpdateSystemCredentials_StampsOnClientSecretRotation() {
	e := testEntity("e-cs")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil)
	s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Return(nil)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Return(nil)

	s.NoError(s.svc.UpdateSystemCredentials(s.ctx, e.ID, json.RawMessage(`{"clientSecret":"rotated"}`)))

	s.store.AssertCalled(s.T(), "UpdateSystemAttributes", mock.Anything, e.ID,
		mock.IsType(json.RawMessage(nil)))
}

// Enrolling a passkey adds an authentication option rather than replacing one, so it must not
// invalidate outstanding tokens. The same holds for the flow secret, which authenticates flow
// initiation rather than the client.
func (s *ServiceTestSuite) TestUpdateSystemCredentials_NoStampForOtherCredentialTypes() {
	for _, creds := range []string{`{"passkey":[{"value":"v1"}]}`, `{"flowSecret":"rotated"}`} {
		e := testEntity("e-other")
		s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
			Return(&entityWithCredentials{Entity: e}, nil).Once()
		s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
			Return(nil).Once()

		s.NoError(s.svc.UpdateSystemCredentials(s.ctx, e.ID, json.RawMessage(creds)))

		s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, e.ID,
			mock.IsType(json.RawMessage(nil)))
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

// testUserSharingType is the sharing resource type the page tests give users.
const testUserSharingType sharing.ResourceType = "user"

// withUserSharing makes users shareable through a sharing service mock, which the caller sets up.
func (s *ServiceTestSuite) withUserSharing() *sharingmock.SharingServiceInterfaceMock {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(),
		sharingService, nil)
	s.svc.RegisterSharedCategory(providers.EntityCategoryUser, testUserSharingType)
	return sharingService
}

// shareUsers makes the sharing service answer with sharedIDs for the users shared into ou1.
func (s *ServiceTestSuite) shareUsers(sharedIDs []string, svcErr *tidcommon.ServiceError) {
	s.withUserSharing().On("ListVisibleResourceIDs", mock.Anything, testUserSharingType, "ou1").
		Return(sharedIDs, svcErr).Once()
}

func (s *ServiceTestSuite) TestGetSharedResourceType() {
	s.svc.RegisterSharedCategory(providers.EntityCategoryUser, testUserSharingType)

	resourceType, ok := s.svc.GetSharedResourceType(providers.EntityCategoryUser)
	s.True(ok)
	s.Equal(testUserSharingType, resourceType)

	_, ok = s.svc.GetSharedResourceType(providers.EntityCategoryAgent)
	s.False(ok, "a category nothing registered is not shareable")
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_PageSpansOwnedAndShared() {
	s.shareUsers([]string{"shared-1", "shared-2"}, nil)
	owned := testEntity("owned-3")
	shared := testEntity("shared-1")
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(3, nil).Once()
	s.store.On("GetEntityListByOUIDs", mock.Anything, "user", []string{"ou1"}, 1, 2, mock.Anything).
		Return([]providers.Entity{*owned}, nil).Once()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"shared-1"}).
		Return([]providers.Entity{*shared}, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeAll, 2, 2, nil)

	s.Require().NoError(err)
	s.Equal(5, page.TotalResults)
	s.Require().Len(page.Owned, 1)
	s.Equal("owned-3", page.Owned[0].ID)
	s.Require().Len(page.Shared, 1)
	s.Equal("shared-1", page.Shared[0].ID)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_PastOwned_ReadsOnlyShared() {
	s.shareUsers([]string{"shared-a", "shared-b", "shared-app", "shared-c"}, nil)
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(2, nil).Once()
	app := testEntity("shared-app")
	app.Category = providers.EntityCategoryApp
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"shared-b", "shared-app", "shared-c"}).
		Return([]providers.Entity{*testEntity("shared-c"), *app, *testEntity("shared-b")}, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeAll, 3, 3, nil)

	s.Require().NoError(err)
	s.Equal(6, page.TotalResults)
	s.Empty(page.Owned)
	s.Require().Len(page.Shared, 2, "an entity of another category is left out")
	s.Equal([]string{"shared-b", "shared-c"}, []string{page.Shared[0].ID, page.Shared[1].ID})
	s.store.AssertNotCalled(s.T(), "GetEntityListByOUIDs", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_OwnedScope_ListsNoShared() {
	s.withUserSharing()
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(1, nil).Once()
	s.store.On("GetEntityListByOUIDs", mock.Anything, "user", []string{"ou1"}, 1, 0, mock.Anything).
		Return([]providers.Entity{*testEntity("owned-1")}, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeOwned, 10, 0, nil)

	s.Require().NoError(err)
	s.Equal(1, page.TotalResults)
	s.Len(page.Owned, 1)
	s.Empty(page.Shared)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_SharedScope_ListsNoOwned() {
	s.shareUsers([]string{"shared-1"}, nil)
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"shared-1"}).
		Return([]providers.Entity{*testEntity("shared-1")}, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeShared, 10, 0, nil)

	s.Require().NoError(err)
	s.Equal(1, page.TotalResults)
	s.Empty(page.Owned)
	s.Len(page.Shared, 1)
	s.store.AssertNotCalled(s.T(), "GetEntityListCountByOUIDs", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

// A category with no sharing resource type is never shared, so only its owned entities are listed.
func (s *ServiceTestSuite) TestGetEntityPageInOU_UnsharedCategory_ListsOwned() {
	s.withUserSharing()
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "agent", []string{"ou1"}, mock.Anything).
		Return(0, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryAgent, "ou1", OUEntityScopeAll, 10, 0, nil)

	s.Require().NoError(err)
	s.Zero(page.TotalResults)
	s.Empty(page.Owned)
	s.Empty(page.Shared)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_UnknownOU_ListsNothing() {
	s.shareUsers(nil, &ou.ErrorOrganizationUnitNotFound)
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(0, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeAll, 10, 0, nil)

	s.Require().NoError(err)
	s.Zero(page.TotalResults)
	s.Empty(page.Owned)
	s.Empty(page.Shared)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_SharingFailure() {
	s.shareUsers(nil, &tidcommon.InternalServerError)
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, mock.Anything).
		Return(0, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeAll, 10, 0, nil)

	s.Error(err)
	s.Nil(page)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_FiltersNarrowOwned() {
	filters := map[string]interface{}{"username": "alice"}
	s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", []string{"ou1"}, filters).
		Return(1, nil).Once()
	s.store.On("GetEntityListByOUIDs", mock.Anything, "user", []string{"ou1"}, 1, 0, filters).
		Return([]providers.Entity{*testEntity("owned-1")}, nil).Once()

	page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeOwned, 10, 0,
		filters)

	s.Require().NoError(err)
	s.Equal(1, page.TotalResults)
	s.Len(page.Owned, 1)
	s.Empty(page.Shared)
}

// Filters cannot narrow shared entities yet, so a scope that lists them refuses filters rather than
// listing shared entities a filter would have left out.
func (s *ServiceTestSuite) TestGetEntityPageInOU_FiltersRefusedWithShared() {
	s.withUserSharing()
	filters := map[string]interface{}{"username": "alice"}
	for _, scope := range []OUEntityScope{OUEntityScopeAll, OUEntityScopeShared} {
		page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", scope, 10, 0, filters)

		s.ErrorIs(err, errSharedEntityFilterUnsupported)
		s.Nil(page)
	}
	s.store.AssertNotCalled(s.T(), "GetEntityListCountByOUIDs", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *ServiceTestSuite) TestGetEntityPageInOU_Errors() {
	for _, tc := range []struct {
		name    string
		arrange func()
	}{
		{"count", func() {
			s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", mock.Anything, mock.Anything).
				Return(0, s.testErr).Once()
		}},
		{"owned list", func() {
			s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", mock.Anything, mock.Anything).
				Return(1, nil).Once()
			s.store.On("GetEntityListByOUIDs", mock.Anything, "user", mock.Anything, 1, 0, mock.Anything).
				Return(nil, s.testErr).Once()
		}},
		{"shared load", func() {
			s.shareUsers([]string{"shared-1"}, nil)
			s.store.On("GetEntityListCountByOUIDs", mock.Anything, "user", mock.Anything, mock.Anything).
				Return(0, nil).Once()
			s.store.On("GetEntitiesByIDs", mock.Anything, []string{"shared-1"}).Return(nil, s.testErr).Once()
		}},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.arrange()

			page, err := s.svc.GetEntityPageInOU(s.ctx, providers.EntityCategoryUser, "ou1", OUEntityScopeAll,
				10, 0, nil)

			s.ErrorIs(err, s.testErr)
			s.Nil(page)
		})
	}
}

func (s *ServiceTestSuite) TestGetEntityListBySystemAttributes_Delegates() {
	sysQuery := SystemAttributeQuery{SystemAttributes: map[string]interface{}{"owner": "u1"}}
	e := testEntity("sys-e1")
	s.store.On("GetEntityListCountBySystemAttributes", mock.Anything, "agent", sysQuery).Return(1, nil)
	s.store.On("GetEntityListBySystemAttributes", mock.Anything, "agent", sysQuery, 10, 0).
		Return([]providers.Entity{*e}, nil)

	count, err := s.svc.GetEntityListCountBySystemAttributes(s.ctx, providers.EntityCategoryAgent, sysQuery)
	s.NoError(err)
	s.Equal(1, count)
	list, err := s.svc.GetEntityListBySystemAttributes(s.ctx, providers.EntityCategoryAgent, sysQuery, 10, 0)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *ServiceTestSuite) TestGetEntityListBySystemAttributes_Errors() {
	sysQuery := SystemAttributeQuery{SystemAttributes: map[string]interface{}{"owner": "u1"}}
	s.store.On("GetEntityListBySystemAttributes", mock.Anything, "agent", sysQuery, 10, 0).
		Return(nil, s.testErr).Once()
	_, err := s.svc.GetEntityListBySystemAttributes(s.ctx, providers.EntityCategoryAgent, sysQuery, 10, 0)
	s.ErrorIs(err, s.testErr, "store error")

	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("sys-read-fails")
	s.store.On("GetEntityListBySystemAttributes", mock.Anything, "agent", sysQuery, 10, 0).
		Return([]providers.Entity{*e}, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)
	_, err = svc.GetEntityListBySystemAttributes(s.ctx, providers.EntityCategoryAgent, sysQuery, 10, 0)
	s.ErrorIs(err, s.testErr, "runtime read error")
}

func (s *ServiceTestSuite) TestGetEntityListBySystemAttributesAndOUIDs_Delegates() {
	sysQuery := SystemAttributeQuery{SystemAttributes: map[string]interface{}{"owner": "u1"}}
	ouIDs := []string{"ou1"}
	e := testEntity("sys-ou-e1")
	s.store.On("GetEntityListCountBySystemAttributesAndOUIDs", mock.Anything, "agent", sysQuery, ouIDs).
		Return(1, nil)
	s.store.On("GetEntityListBySystemAttributesAndOUIDs", mock.Anything, "agent", sysQuery, ouIDs, 10, 0).
		Return([]providers.Entity{*e}, nil)

	count, err := s.svc.GetEntityListCountBySystemAttributesAndOUIDs(
		s.ctx, providers.EntityCategoryAgent, sysQuery, ouIDs)
	s.NoError(err)
	s.Equal(1, count)
	list, err := s.svc.GetEntityListBySystemAttributesAndOUIDs(
		s.ctx, providers.EntityCategoryAgent, sysQuery, ouIDs, 10, 0)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *ServiceTestSuite) TestGetEntityListBySystemAttributesAndOUIDs_Errors() {
	sysQuery := SystemAttributeQuery{SystemAttributes: map[string]interface{}{"owner": "u1"}}
	ouIDs := []string{"ou1"}
	s.store.On("GetEntityListBySystemAttributesAndOUIDs", mock.Anything, "agent", sysQuery, ouIDs, 10, 0).
		Return(nil, s.testErr).Once()
	_, err := s.svc.GetEntityListBySystemAttributesAndOUIDs(
		s.ctx, providers.EntityCategoryAgent, sysQuery, ouIDs, 10, 0)
	s.ErrorIs(err, s.testErr, "store error")

	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("sys-ou-read-fails")
	s.store.On("GetEntityListBySystemAttributesAndOUIDs", mock.Anything, "agent", sysQuery, ouIDs, 10, 0).
		Return([]providers.Entity{*e}, nil)
	runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)
	_, err = svc.GetEntityListBySystemAttributesAndOUIDs(s.ctx, providers.EntityCategoryAgent, sysQuery, ouIDs, 10, 0)
	s.ErrorIs(err, s.testErr, "runtime read error")
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

// testAppSharingType is the sharing resource type the boundary tests give applications.
const testAppSharingType sharing.ResourceType = "application"

// withBoundarySharing makes users and applications shareable through a sharing service mock, which the
// caller sets up.
func (s *ServiceTestSuite) withBoundarySharing() *sharingmock.SharingServiceInterfaceMock {
	sharingService := s.withUserSharing()
	s.svc.RegisterSharedCategory(providers.EntityCategoryApp, testAppSharingType)
	app := userIn("app-1", "ou-app")
	app.Category = providers.EntityCategoryApp
	s.store.On("GetEntity", mock.Anything, "app-1").Return(app, nil).Maybe()
	return sharingService
}

// inBoundary returns the suite context carrying a sign-in boundary for the application app-1.
func (s *ServiceTestSuite) inBoundary(ouID string) context.Context {
	return authnprovidercm.WithSignInBoundary(s.ctx, authnprovidercm.SignInBoundary{OUID: ouID, ApplicationID: "app-1"})
}

// boolPtr returns a pointer to b.
func boolPtr(b bool) *bool { return &b }

// userIn returns a user an organization unit owns.
func userIn(id, ouID string) providers.Entity {
	e := testEntity(id)
	e.OUID = ouID
	return *e
}

var aliceFilters = map[string]interface{}{"username": "alice"}

func (s *ServiceTestSuite) TestIdentifyEntity_WithoutBoundaryAsksTheStoreOnly() {
	s.withBoundarySharing()
	id := "u1"
	s.store.On("IdentifyEntity", mock.Anything, aliceFilters).Return(&id, nil).Once()

	got, err := s.svc.IdentifyEntity(s.ctx, aliceFilters)

	s.NoError(err)
	s.Equal(&id, got)
	s.store.AssertNotCalled(s.T(), "SearchEntities", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestSearchEntities_WithoutBoundaryKeepsEveryUser() {
	s.withBoundarySharing()
	users := []providers.Entity{userIn("u1", "ou-a"), userIn("u2", "ou-b")}
	s.store.On("SearchEntities", mock.Anything, aliceFilters).Return(users, nil).Once()

	got, err := s.svc.SearchEntities(s.ctx, aliceFilters)

	s.NoError(err)
	s.Len(got, 2)
}

func (s *ServiceTestSuite) TestSearchEntities_FixedOUBoundary() {
	cases := []struct {
		name   string
		user   providers.Entity
		shared *bool
		kept   bool
	}{
		{"owned is kept", userIn("u1", "ou-a"), nil, true},
		{"shared in is kept", userIn("u2", "ou-b"), boolPtr(true), true},
		{"foreign is dropped", userIn("u3", "ou-c"), boolPtr(false), false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			sharingService := s.withBoundarySharing()
			s.store.On("SearchEntities", mock.Anything, aliceFilters).Return([]providers.Entity{tc.user}, nil).Once()
			if tc.shared != nil {
				sharingService.On("IsVisible", mock.Anything, testUserSharingType, tc.user.ID, "ou-a").
					Return(*tc.shared, nil).Once()
			}

			got, err := s.svc.SearchEntities(s.inBoundary("ou-a"), aliceFilters)

			if tc.kept {
				s.NoError(err)
				s.Equal([]string{tc.user.ID}, entityIDs(got))
				return
			}
			s.ErrorIs(err, ErrEntityNotFound)
		})
	}
}

// A sign-in that has not fixed its OU searches deployment-wide; the OU resolver or the assertion step refuses a
// user the application can't be used with.
func (s *ServiceTestSuite) TestSearchEntities_BoundaryWithoutAFixedOUKeepsEveryUser() {
	s.withBoundarySharing()
	users := []providers.Entity{userIn("u1", "ou-a"), userIn("u2", "ou-b")}
	s.store.On("SearchEntities", mock.Anything, aliceFilters).Return(users, nil).Once()

	got, err := s.svc.SearchEntities(s.inBoundary(""), aliceFilters)

	s.NoError(err)
	s.Equal([]string{"u1", "u2"}, entityIDs(got))
}

func (s *ServiceTestSuite) TestIdentifyEntity_BoundaryWithoutAFixedOUAsksTheStore() {
	s.withBoundarySharing()
	id := "u1"
	s.store.On("IdentifyEntity", mock.Anything, aliceFilters).Return(&id, nil).Once()

	got, err := s.svc.IdentifyEntity(s.inBoundary(""), aliceFilters)

	s.NoError(err)
	s.Equal(&id, got)
	s.store.AssertNotCalled(s.T(), "SearchEntities", mock.Anything, mock.Anything)
}

// Every category is kept to the boundary's OU: an entity of a shareable category also where it is shared, one of
// any other category (an agent) in its own OU only.
func (s *ServiceTestSuite) TestSearchEntities_BoundaryKeepsEveryCategoryToTheOU() {
	sharingService := s.withBoundarySharing()
	sharedApp := userIn("app-x", "ou-z")
	sharedApp.Category = providers.EntityCategoryApp
	agentElsewhere := userIn("agent-x", "ou-z")
	agentElsewhere.Category = providers.EntityCategoryAgent
	agentInOU := userIn("agent-a", "ou-a")
	agentInOU.Category = providers.EntityCategoryAgent
	user := userIn("u1", "ou-z")
	s.store.On("SearchEntities", mock.Anything, aliceFilters).
		Return([]providers.Entity{sharedApp, agentElsewhere, agentInOU, user}, nil).Once()
	sharingService.On("IsVisible", mock.Anything, testAppSharingType, sharedApp.ID, "ou-a").Return(true, nil).Once()
	sharingService.On("IsVisible", mock.Anything, testUserSharingType, user.ID, "ou-a").Return(false, nil).Once()

	got, err := s.svc.SearchEntities(s.inBoundary("ou-a"), aliceFilters)

	s.NoError(err)
	s.Equal([]string{sharedApp.ID, agentInOU.ID}, entityIDs(got))
}

func (s *ServiceTestSuite) TestIdentifyEntity_SameUsernameInTwoOUsResolvesInsideTheBoundary() {
	sharingService := s.withBoundarySharing()
	outside, inside := userIn("u1", "ou-a"), userIn("u2", "ou-b")
	s.store.On("SearchEntities", mock.Anything, aliceFilters).
		Return([]providers.Entity{outside, inside}, nil).Once()
	sharingService.On("IsVisible", mock.Anything, testUserSharingType, outside.ID, "ou-b").Return(false, nil).Once()

	got, err := s.svc.IdentifyEntity(s.inBoundary("ou-b"), aliceFilters)

	s.NoError(err)
	s.Equal(inside.ID, *got)
	s.store.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestIdentifyEntity_BoundaryJudgesWhatRemains() {
	cases := []struct {
		name    string
		filters map[string]interface{}
		arrange func(*sharingmock.SharingServiceInterfaceMock)
		wantErr error
	}{
		{
			name:    "nothing matches",
			filters: aliceFilters,
			arrange: func(*sharingmock.SharingServiceInterfaceMock) {
				s.store.On("SearchEntities", mock.Anything, aliceFilters).Return(nil, ErrEntityNotFound).Once()
			},
			wantErr: ErrEntityNotFound,
		},
		{
			name:    "every match is outside",
			filters: aliceFilters,
			arrange: func(sharingService *sharingmock.SharingServiceInterfaceMock) {
				s.store.On("SearchEntities", mock.Anything, aliceFilters).
					Return([]providers.Entity{userIn("u1", "ou-c")}, nil).Once()
				sharingService.On("IsVisible", mock.Anything, testUserSharingType, "u1", "ou-a").
					Return(false, nil).Once()
			},
			wantErr: ErrEntityNotFound,
		},
		{
			name:    "two remain",
			filters: aliceFilters,
			arrange: func(sharingService *sharingmock.SharingServiceInterfaceMock) {
				s.store.On("SearchEntities", mock.Anything, aliceFilters).
					Return([]providers.Entity{userIn("u1", "ou-a"), userIn("u2", "ou-b")}, nil).Once()
				sharingService.On("IsVisible", mock.Anything, testUserSharingType, "u2", "ou-a").
					Return(true, nil).Once()
			},
			wantErr: ErrAmbiguousEntity,
		},
		{
			name:    "no filters identify nobody",
			filters: map[string]interface{}{},
			arrange: func(*sharingmock.SharingServiceInterfaceMock) {},
			wantErr: ErrEntityNotFound,
		},
		{
			name:    "a sharing check failure is returned",
			filters: aliceFilters,
			arrange: func(sharingService *sharingmock.SharingServiceInterfaceMock) {
				s.store.On("SearchEntities", mock.Anything, aliceFilters).
					Return([]providers.Entity{userIn("u1", "ou-c")}, nil).Once()
				sharingService.On("IsVisible", mock.Anything, testUserSharingType, "u1", "ou-a").
					Return(false, &tidcommon.InternalServerError).Once()
			},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.arrange(s.withBoundarySharing())

			got, err := s.svc.IdentifyEntity(s.inBoundary("ou-a"), tc.filters)

			s.Nil(got)
			if tc.wantErr == nil {
				s.Error(err)
				return
			}
			s.ErrorIs(err, tc.wantErr)
		})
	}
}

func (s *ServiceTestSuite) TestEntityOUs_OwnFirstThenSharedInSorted() {
	sharingService := s.withUserSharing()
	s.store.On("GetEntity", mock.Anything, "u1").Return(userIn("u1", "ou-own"), nil).Once()
	sharingService.On("ListResourceSharedOUIDs", mock.Anything, testUserSharingType, "u1").
		Return([]string{"ou-c", "ou-b"}, nil).Once()

	got, found, err := s.svc.EntityOUs(s.ctx, "u1")

	s.NoError(err)
	s.True(found)
	s.Equal([]string{"ou-own", "ou-b", "ou-c"}, got)
}

func (s *ServiceTestSuite) TestEntityOUs_UnshareableUsersHaveTheirOwnOUOnly() {
	s.store.On("GetEntity", mock.Anything, "u1").Return(userIn("u1", "ou-own"), nil).Once()

	got, found, err := s.svc.EntityOUs(s.ctx, "u1")

	s.NoError(err)
	s.True(found)
	s.Equal([]string{"ou-own"}, got)
}

func (s *ServiceTestSuite) TestEntityOUs_AnEntityTheStoreDoesNotHoldIsNotFound() {
	sharingService := s.withUserSharing()
	s.store.On("GetEntity", mock.Anything, "u1").Return(providers.Entity{}, ErrEntityNotFound).Once()

	got, found, err := s.svc.EntityOUs(s.ctx, "u1")

	s.NoError(err)
	s.False(found)
	s.Nil(got)
	sharingService.AssertNotCalled(s.T(), "ListResourceSharedOUIDs", mock.Anything, mock.Anything, mock.Anything)
}

// An agent cannot be shared, so it is in its own organization unit only.
func (s *ServiceTestSuite) TestEntityOUs_AnAgentIsInItsOwnOUOnly() {
	sharingService := s.withUserSharing()
	agent := userIn("agent-1", "ou-a")
	agent.Category = providers.EntityCategoryAgent
	s.store.On("GetEntity", mock.Anything, "agent-1").Return(agent, nil).Once()

	got, found, err := s.svc.EntityOUs(s.ctx, "agent-1")

	s.NoError(err)
	s.True(found)
	s.Equal([]string{"ou-a"}, got)
	sharingService.AssertNotCalled(s.T(), "ListResourceSharedOUIDs", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEntityOUs_Failures() {
	cases := []struct {
		name    string
		arrange func(*sharingmock.SharingServiceInterfaceMock)
	}{
		{"the user lookup fails", func(*sharingmock.SharingServiceInterfaceMock) {
			s.store.On("GetEntity", mock.Anything, "u1").Return(providers.Entity{}, s.testErr).Once()
		}},
		{"the sharing lookup fails", func(sharingService *sharingmock.SharingServiceInterfaceMock) {
			s.store.On("GetEntity", mock.Anything, "u1").Return(userIn("u1", "ou-own"), nil).Once()
			sharingService.On("ListResourceSharedOUIDs", mock.Anything, testUserSharingType, "u1").
				Return(nil, &tidcommon.InternalServerError).Once()
		}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.arrange(s.withUserSharing())

			got, found, err := s.svc.EntityOUs(s.ctx, "u1")

			s.Error(err)
			s.False(found)
			s.Nil(got)
		})
	}
}

// Uniqueness spans every organization unit, so a sign-in boundary does not hide a conflicting user.
func (s *ServiceTestSuite) TestValidateEntityType_UniquenessIgnoresTheSignInBoundary() {
	svc, ets := s.newSvcWithEntityType()
	attrs := json.RawMessage(`{"username":"alice"}`)
	taken := "u-elsewhere"
	ets.On("ValidateEntity", mock.Anything, entitytype.TypeCategoryUser, "employee", attrs, false).
		Return(true, nil).Once()
	ets.On("ValidateEntityUniqueness", mock.Anything, entitytype.TypeCategoryUser, "employee", attrs, mock.Anything).
		Return(func(_ context.Context, _ entitytype.TypeCategory, _ string, _ json.RawMessage,
			exists func(map[string]interface{}) (bool, error)) (bool, *tidcommon.ServiceError) {
			found, err := exists(aliceFilters)
			s.NoError(err)
			return !found, nil
		}).Once()
	s.store.On("IdentifyEntity", mock.Anything, aliceFilters).Return(&taken, nil).Once()

	err := svc.validateEntityType(s.inBoundary("ou-a"), providers.EntityCategoryUser, "employee",
		attrs, "", false)

	s.ErrorIs(err, ErrAttributeConflict)
	s.store.AssertNotCalled(s.T(), "SearchEntities", mock.Anything, mock.Anything)
}

// entityIDs returns the IDs of entities in order.
func entityIDs(entities []providers.Entity) []string {
	ids := make([]string, 0, len(entities))
	for _, e := range entities {
		ids = append(ids, e.ID)
	}
	return ids
}

func testCredentialsJSON() json.RawMessage {
	return json.RawMessage(`{"password":[{` +
		`"value":"testhash","storageAlgo":"PBKDF2",` +
		`"storageAlgoParams":{"salt":"testsalt","iterations":1,"keySize":32}}]}`)
}

// twoCandidateCredentialsJSON stores two password candidates, as an entity mid-rehash holds.
func twoCandidateCredentialsJSON() json.RawMessage {
	return json.RawMessage(`{"password":[` +
		`{"value":"brokenhash","storageAlgo":"LEGACY",` +
		`"storageAlgoParams":{"salt":"s","iterations":1,"keySize":32}},` +
		`{"value":"testhash","storageAlgo":"PBKDF2",` +
		`"storageAlgoParams":{"salt":"testsalt","iterations":1,"keySize":32}}]}`)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_Success() {
	storedCreds := testCredentialsJSON()
	e := testEntity("auth-id-1")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: storedCreds}, nil)
	s.hashService.On("Verify", []byte("password123"), mock.Anything).Return(true, nil)

	result, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "password123"})
	s.NoError(err)
	s.Empty(mismatchedEntityID, "a successful verification produces no failure evidence")
	s.Equal(e.ID, result.EntityID)
	s.Equal(e.Category, result.EntityCategory)
	s.Equal(e.Type, result.EntityType)
	s.Equal(e.OUID, result.OUID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_EmptyID() {
	_, _, err := s.svc.AuthenticateEntityByID(s.ctx, "", map[string]interface{}{"password": "p"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_EmptyCredentials() {
	_, _, err := s.svc.AuthenticateEntityByID(s.ctx, "some-id", map[string]interface{}{})
	s.ErrorIs(err, ErrAuthenticationFailed)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_EntityNotFound() {
	s.store.On("GetEntityWithCredentials", mock.Anything, "missing").
		Return(nil, ErrEntityNotFound)

	_, _, err := s.svc.AuthenticateEntityByID(s.ctx, "missing", map[string]interface{}{"password": "p"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_DoesNotCheckProfileState() {
	e := testEntity("inactive-1")
	e.State = providers.EntityState("SUSPENDED")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)

	s.hashService.On("Verify", []byte("p"), mock.Anything).Return(true, nil)
	result, _, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "p"})
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

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Equal(e.ID, mismatchedEntityID, "a compared and rejected secret is attributable")
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_VerifyError() {
	storedCreds := testCredentialsJSON()
	e := testEntity("auth-verify-err-1")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: storedCreds}, nil)
	s.hashService.On("Verify", []byte("password123"), mock.Anything).Return(false, s.testErr)

	_, _, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID, map[string]interface{}{"password": "password123"})
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

	result, _, err := s.svc.AuthenticateEntity(s.ctx, filters, map[string]interface{}{"password": "pass"})
	s.NoError(err)
	s.Equal(id, result.EntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntity_EmptyIdentifiers() {
	_, mismatchedEntityID, err := s.svc.AuthenticateEntity(s.ctx, map[string]interface{}{},
		map[string]interface{}{"password": "p"})
	s.ErrorIs(err, ErrEntityNotFound)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntity_EmptyCredentials() {
	_, mismatchedEntityID, err := s.svc.AuthenticateEntity(s.ctx, map[string]interface{}{"username": "user1"},
		map[string]interface{}{})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Empty(mismatchedEntityID)
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

func (s *ServiceTestSuite) TestGetTransitiveEntityGroups_BoundaryKeepsOnlyTheOUsGroups() {
	ctx := s.inBoundary("ou-decided")
	provider := NewGroupMembershipProviderMock(s.T())
	provider.On("GetTransitiveGroupsForEntity", ctx, "user1").Return([]providers.EntityGroup{
		{ID: "grp-home", Name: "Admins", OUID: "ou-home"},
		{ID: "grp-decided", Name: "Editors", OUID: "ou-decided"},
	}, nil).Once()

	s.svc.SetGroupMembershipProvider(provider)
	groups, err := s.svc.GetTransitiveEntityGroups(ctx, "user1")

	s.NoError(err)
	s.Equal([]providers.EntityGroup{{ID: "grp-decided", Name: "Editors", OUID: "ou-decided"}}, groups)
}

func (s *ServiceTestSuite) TestGetTransitiveEntityGroups_BoundaryWithNoGroupInTheOUReturnsNone() {
	ctx := s.inBoundary("ou-decided")
	provider := NewGroupMembershipProviderMock(s.T())
	provider.On("GetTransitiveGroupsForEntity", ctx, "user1").
		Return([]providers.EntityGroup{{ID: "grp-home", Name: "Admins", OUID: "ou-home"}}, nil).Once()

	s.svc.SetGroupMembershipProvider(provider)
	groups, err := s.svc.GetTransitiveEntityGroups(ctx, "user1")

	s.NoError(err)
	s.Empty(groups)
}

func (s *ServiceTestSuite) TestGetTransitiveEntityGroups_BoundaryWithoutADecidedOUKeepsEveryGroup() {
	ctx := s.inBoundary("")
	expected := []providers.EntityGroup{
		{ID: "grp-home", Name: "Admins", OUID: "ou-home"},
		{ID: "grp-other", Name: "Editors", OUID: "ou-other"},
	}
	provider := NewGroupMembershipProviderMock(s.T())
	provider.On("GetTransitiveGroupsForEntity", ctx, "user1").Return(expected, nil).Once()

	s.svc.SetGroupMembershipProvider(provider)
	groups, err := s.svc.GetTransitiveEntityGroups(ctx, "user1")

	s.NoError(err)
	s.Equal(expected, groups)
}

func (s *ServiceTestSuite) TestGetTransitiveEntityGroups_BoundaryProviderError() {
	ctx := s.inBoundary("ou-decided")
	provider := NewGroupMembershipProviderMock(s.T())
	provider.On("GetTransitiveGroupsForEntity", ctx, "user1").Return(nil, s.testErr).Once()

	s.svc.SetGroupMembershipProvider(provider)
	_, err := s.svc.GetTransitiveEntityGroups(ctx, "user1")

	s.ErrorIs(err, s.testErr)
}

// A system-attribute blob the server cannot parse must fail the credential write rather than silently
// replacing the blob, which would drop the keys belonging to whichever service owns the entity.
func (s *ServiceTestSuite) TestWithSystemAttribute_CorruptBlobErrors() {
	_, err := withSystemAttribute(json.RawMessage(`not json`), authnprovidercm.SystemAttrCredentialUpdatedAt,
		"2026-08-12T10:00:00Z")
	s.Error(err)
}

// An absent blob is normal for an entity that has never carried system attributes.
func (s *ServiceTestSuite) TestWithSystemAttribute_EmptyBlobStartsFresh() {
	marked, err := withSystemAttribute(nil, authnprovidercm.SystemAttrCredentialUpdatedAt, "2026-08-12T10:00:00Z")
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
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
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

// An agent's lifespan is changed only through SetValidUntil, so a generic write keeps the stored
// value whether it leaves validUntil out or sends another one.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_PreservesValidUntil() {
	e := testEntity("e-preserve-lifespan")
	e.SystemAttributes = json.RawMessage(`{"name":"Old","validUntil":"2027-03-31T00:00:00Z"}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	for _, incoming := range []string{`{"name":"New"}`, `{"name":"New","validUntil":"9999-12-31T23:59:59Z"}`} {
		s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(incoming)))

		var attrs map[string]interface{}
		s.Require().NoError(json.Unmarshal(written, &attrs))
		s.Equal("New", attrs["name"])
		s.Equal("2027-03-31T00:00:00Z", attrs[authnprovidercm.SystemAttrValidUntil])
	}
}

// SetValidUntil changes the lifespan alone and keeps every other key, reserved or not.
func (s *ServiceTestSuite) TestSetValidUntil_ChangesOnlyValidUntil() {
	e := testEntity("e-set-lifespan")
	e.SystemAttributes = json.RawMessage(
		`{"owner":"o-1","credentialUpdatedAt":"2026-08-12T10:00:00Z","validUntil":"2027-03-31T00:00:00Z"}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.SetValidUntil(s.ctx, e.ID, "2028-01-01T00:00:00Z"))

	s.JSONEq(`{"owner":"o-1","credentialUpdatedAt":"2026-08-12T10:00:00Z","validUntil":"2028-01-01T00:00:00Z"}`,
		string(written))
}

func (s *ServiceTestSuite) TestSetValidUntil_Errors() {
	s.store.On("LockEntity", mock.Anything, "e-missing").Return(providers.Entity{}, ErrEntityNotFound)
	s.ErrorIs(s.svc.SetValidUntil(s.ctx, "e-missing", "2028-01-01T00:00:00Z"), ErrEntityNotFound)

	corrupt := testEntity("e-corrupt-lifespan")
	corrupt.SystemAttributes = json.RawMessage(`not json`)
	s.store.On("LockEntity", mock.Anything, corrupt.ID).Return(*corrupt, nil)
	s.Error(s.svc.SetValidUntil(s.ctx, corrupt.ID, "2028-01-01T00:00:00Z"))
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
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
// sends linkedIds through a wholesale replace has them dropped when the account holds none.
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
	s.expectLinkLock(*e)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.JSONEq(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`, string(written))
	s.store.AssertNotCalled(s.T(), "GetEntity", mock.Anything, mock.Anything)
}

// linkedIds a caller writes into the system attributes are dropped, so a link is created only through
// the typed link.
func (s *ServiceTestSuite) TestCreateEntity_CallerLinkedIDsDropped() {
	e := testEntity("e-raw-links")
	e.SystemAttributes = json.RawMessage(`{"name":"x","linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, mock.MatchedBy(func(got providers.Entity) bool {
		return string(got.SystemAttributes) == `{"name":"x"}`
	}), json.RawMessage(nil), json.RawMessage(nil)).Return(nil).Once()
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()

	_, err := s.svc.CreateEntity(s.ctx, e, nil, nil)
	s.NoError(err)
}

// A link missing its connection or subject names nothing, so the create is refused before anything
// is looked up or written.
func (s *ServiceTestSuite) TestCreateEntity_IncompleteLinkRejected() {
	for name, link := range map[string]providers.LinkedAccount{
		"no connection": {Sub: "sub-1"},
		"no subject":    {IdpID: "idp-a"},
	} {
		s.Run(name, func() {
			_, err := s.svc.CreateEntity(s.ctx, testEntity("e-linked"), nil, &link)
			s.ErrorIs(err, ErrBadAttributesInRequest)
		})
	}
}

func (s *ServiceTestSuite) TestLinkAccount_AddsSecondConnection() {
	e := testEntity("e-link-2")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e)

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
	s.expectLinkLock(*e)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A user may hold several accounts at one connection, so a second subject is added alongside the
// first. The subjects share the identifier NAME and differ by VALUE, which the primary key keeps
// apart.
func (s *ServiceTestSuite) TestLinkAccount_AddsSecondSubjectAtSameConnection() {
	e := testEntity("e-link-4")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e)

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

// expectLinkLock expects the link write to lock the entity, reading it as e.
func (s *ServiceTestSuite) expectLinkLock(e providers.Entity) {
	s.store.On("LockEntity", mock.Anything, e.ID).Return(e, nil).Once()
}

// Several entities may hold one subject, so linking never looks up who else holds it.
func (s *ServiceTestSuite) TestLinkAccount_DoesNotCheckOtherHolders() {
	e := testEntity("e-link-5")
	s.expectLinkLock(*e)
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).Return(nil).Once()

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.store.AssertNotCalled(s.T(), "ResolveLinkedAccounts", mock.Anything, mock.Anything, mock.Anything)
}

// The entity's own pair is a repeat link, so nothing is written.
func (s *ServiceTestSuite) TestLinkAccount_OwnPairIsNotRewritten() {
	e := testEntity("e-link-7")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	s.expectLinkLock(*e)

	s.NoError(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-1"))
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLinkAccount_LockFailureWritesNothing() {
	lockErr := errors.New("lock failed")
	s.store.On("LockEntity", mock.Anything, "e-link-8").Return(providers.Entity{}, lockErr).Once()

	s.ErrorIs(s.svc.LinkAccount(s.ctx, "e-link-8", "idp-a", "sub-1"), lockErr)
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

	s.expectLinkLock(*e)
	s.ErrorIs(s.svc.LinkAccount(s.ctx, e.ID, "idp-b", "sub-new"), ErrIndexedValueLimitExceeded)

	s.expectLinkLock(*e)
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
		s.expectLinkLock(*e)
		s.Error(s.svc.LinkAccount(s.ctx, e.ID, "idp-a", "sub-2"), name)
	}
	s.store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// A null entry is the same as no entry.
func (s *ServiceTestSuite) TestLinkAccount_NullLinksStartEmpty() {
	e := testEntity("e-link-null")
	e.SystemAttributes = json.RawMessage(`{"linkedIds":null}`)
	s.expectLinkLock(*e)

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

func (s *ServiceTestSuite) TestResolveLinkedAccounts_Delegates() {
	expected := []string{"e-resolved", "e-other"}
	s.store.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return(expected, nil)

	got, err := s.svc.ResolveLinkedAccounts(s.ctx, "idp-a", "sub-1")
	s.Require().NoError(err)
	s.Equal(expected, got)
}

// A subject is unique only within its issuing connection, so both parts are required and a missing
// one is a bad request rather than a broad lookup.
func (s *ServiceTestSuite) TestResolveLinkedAccounts_RejectsEmptyArguments() {
	_, err := s.svc.ResolveLinkedAccounts(s.ctx, "", "sub-1")
	s.ErrorIs(err, ErrBadAttributesInRequest)
	_, err = s.svc.ResolveLinkedAccounts(s.ctx, "idp-a", "")
	s.ErrorIs(err, ErrBadAttributesInRequest)
	s.store.AssertNotCalled(s.T(), "ResolveLinkedAccounts", mock.Anything, mock.Anything, mock.Anything)
}

// Every holder of the subject is returned with its OU handle, for the user to pick between them.
func (s *ServiceTestSuite) TestGetLinkedEntities_ReturnsHoldersWithOUHandles() {
	ouService := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, ouService,
		transaction.NewNoOpTransactioner(), nil, nil)
	first, second := testEntity("e-holder-1"), testEntity("e-holder-2")
	second.OUID = "ou-2"
	s.store.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").
		Return([]string{first.ID, second.ID}, nil).Once()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{first.ID, second.ID}).
		Return([]providers.Entity{*first, *second}, nil).Once()
	ouService.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{"ou-1", "ou-2"}).
		Return(map[string]string{"ou-1": "sales", "ou-2": "support"}, nil).Once()

	got, err := s.svc.GetLinkedEntities(s.ctx, "idp-a", "sub-1")
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal("sales", got[0].OUHandle)
	s.Equal("support", got[1].OUHandle)
}

func (s *ServiceTestSuite) TestGetLinkedEntities_NoHolders() {
	s.store.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return([]string{}, nil).Once()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{}).Return([]providers.Entity{}, nil).Once()

	got, err := s.svc.GetLinkedEntities(s.ctx, "idp-a", "sub-1")
	s.Require().NoError(err)
	s.Empty(got)
}

func (s *ServiceTestSuite) TestGetLinkedEntities_GetEntitiesFails() {
	s.store.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return([]string{"e-1"}, nil).Once()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"e-1"}).Return(nil, s.testErr).Once()

	_, err := s.svc.GetLinkedEntities(s.ctx, "idp-a", "sub-1")
	s.ErrorIs(err, s.testErr)
}

// linkHoldersInsideAndOutside links sub-1 to a user ou-a owns and to a user only ou-c holds.
func (s *ServiceTestSuite) linkHoldersInsideAndOutside() {
	sharingService := s.withBoundarySharing()
	inside, outside := userIn("u-inside", "ou-a"), userIn("u-outside", "ou-c")
	s.store.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").
		Return([]string{inside.ID, outside.ID}, nil).Once()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{inside.ID, outside.ID}).
		Return([]providers.Entity{inside, outside}, nil).Once()
	sharingService.On("IsVisible", mock.Anything, testUserSharingType, outside.ID, "ou-a").Return(false, nil).Once()
}

func (s *ServiceTestSuite) TestGetLinkedEntities_BoundaryLeavesOutHoldersOutside() {
	s.linkHoldersInsideAndOutside()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{"u-inside"}).
		Return([]providers.Entity{userIn("u-inside", "ou-a")}, nil).Once()

	got, err := s.svc.GetLinkedEntities(s.inBoundary("ou-a"), "idp-a", "sub-1")

	s.Require().NoError(err)
	s.Equal([]string{"u-inside"}, entityIDs(got))
}

func (s *ServiceTestSuite) TestResolveLinkedAccounts_BoundaryLeavesOutHoldersOutside() {
	s.linkHoldersInsideAndOutside()

	got, err := s.svc.ResolveLinkedAccounts(s.inBoundary("ou-a"), "idp-a", "sub-1")

	s.Require().NoError(err)
	s.Equal([]string{"u-inside"}, got)
}

func (s *ServiceTestSuite) TestResolveLinkedAccounts_BoundaryWithEveryHolderOutsideFindsNone() {
	sharingService := s.withBoundarySharing()
	outside := userIn("u-outside", "ou-c")
	s.store.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return([]string{outside.ID}, nil).Once()
	s.store.On("GetEntitiesByIDs", mock.Anything, []string{outside.ID}).
		Return([]providers.Entity{outside}, nil).Once()
	sharingService.On("IsVisible", mock.Anything, testUserSharingType, outside.ID, "ou-a").Return(false, nil).Once()

	got, err := s.svc.ResolveLinkedAccounts(s.inBoundary("ou-a"), "idp-a", "sub-1")

	s.Require().NoError(err)
	s.Empty(got)
}

// An entity with no marker recorded is written through untouched.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_NoMarkerPassesThrough() {
	e := testEntity("e-nomarker")
	e.SystemAttributes = json.RawMessage(`{"name":"Old"}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)

	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))
	s.JSONEq(`{"name":"New"}`, string(written))
}

// --- Failure evidence ---

func (s *ServiceTestSuite) TestAuthenticateEntityByID_WrongCredentialProducesEvidence() {
	e := testEntity("count-1")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)
	s.hashService.On("Verify", []byte("wrong"), mock.Anything).Return(false, nil)

	result, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Nil(result, "a refusal returns no authenticate result")
	s.Equal(e.ID, mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntity_EvidenceNamesTheComparedEntity() {
	id := "count-2"
	e := testEntity(id)
	filters := map[string]interface{}{"username": "someone"}
	s.store.On("IdentifyEntity", mock.Anything, filters).Return(&id, nil)
	s.store.On("GetEntityWithCredentials", mock.Anything, id).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)
	s.hashService.On("Verify", mock.Anything, mock.Anything).Return(false, nil)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntity(s.ctx, filters,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Equal(id, mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntity_UnknownIdentifierProducesNoEvidence() {
	filters := map[string]interface{}{"username": "ghost"}
	s.store.On("IdentifyEntity", mock.Anything, filters).Return(nil, ErrEntityNotFound)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntity(s.ctx, filters,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrEntityNotFound)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_NoStoredCredentialOfThatTypeProducesNoEvidence() {
	e := testEntity("count-3")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"pin": "1234"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_NoStoredCredentialsProducesNoEvidence() {
	e := testEntity("count-4")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_CorruptCredentialBlobProducesNoEvidence() {
	corrupt := json.RawMessage(`not json`)
	cases := []struct {
		name   string
		stored *entityWithCredentials
	}{
		{name: "schema credentials", stored: &entityWithCredentials{SchemaCredentials: corrupt}},
		{name: "system credentials", stored: &entityWithCredentials{SystemCredentials: corrupt}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			e := testEntity("count-5")
			tc.stored.Entity = e
			s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).Return(tc.stored, nil)

			_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
				map[string]interface{}{"password": "wrong"})
			s.Require().Error(err)
			s.NotErrorIs(err, ErrAuthenticationFailed)
			s.Empty(mismatchedEntityID)
		})
	}
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_IndeterminateHashServiceProducesNoEvidence() {
	e := testEntity("count-6")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)
	s.hashService.On("Verify", mock.Anything, mock.Anything).
		Return(false, errors.New("hash service unavailable"))

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_MatchingCandidateWinsOverAnIndeterminateOne() {
	e := testEntity("count-7")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: twoCandidateCredentialsJSON()}, nil)
	s.hashService.On("Verify", mock.Anything, mock.MatchedBy(func(c cryptolib.Credential) bool {
		return c.Hash == "brokenhash"
	})).Return(false, errors.New("unsupported algorithm"))
	s.hashService.On("Verify", mock.Anything, mock.MatchedBy(func(c cryptolib.Credential) bool {
		return c.Hash == "testhash"
	})).Return(true, nil)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "right"})
	s.Require().NoError(err)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_NoMatchWithAnIndeterminateCandidateProducesNoEvidence() {
	e := testEntity("count-8")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: twoCandidateCredentialsJSON()}, nil)
	s.hashService.On("Verify", mock.Anything, mock.MatchedBy(func(c cryptolib.Credential) bool {
		return c.Hash == "brokenhash"
	})).Return(false, errors.New("unsupported algorithm"))
	s.hashService.On("Verify", mock.Anything, mock.MatchedBy(func(c cryptolib.Credential) bool {
		return c.Hash == "testhash"
	})).Return(false, nil)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{"password": "wrong"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Empty(mismatchedEntityID)
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_ProvenMismatchDecidesOverAnIndeterminateSibling() {
	s.hashService.On("Verify", []byte("wrong"), mock.Anything).Return(false, nil)
	cases := []struct {
		name string
		id   string
		// sibling is the indeterminate credential type; credentials are compared in sorted type order.
		sibling string
		reached bool
	}{
		{name: "sibling compared after the mismatch", id: "count-10", sibling: "pin", reached: false},
		{name: "sibling compared before the mismatch", id: "count-11", sibling: "passcode", reached: true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			e := testEntity(tc.id)
			s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
				Return(&entityWithCredentials{
					Entity: e,
					SchemaCredentials: json.RawMessage(`{` +
						`"password":[{"value":"testhash","storageAlgo":"PBKDF2",` +
						`"storageAlgoParams":{"salt":"testsalt","iterations":1,"keySize":32}}],` +
						`"` + tc.sibling + `":[{"value":"brokenhash","storageAlgo":"LEGACY",` +
						`"storageAlgoParams":{"salt":"s","iterations":1,"keySize":32}}]}`),
				}, nil).Once()
			if tc.reached {
				s.hashService.On("Verify", []byte("0000"), mock.Anything).
					Return(false, errors.New("unsupported algorithm")).Once()
			}

			_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
				map[string]interface{}{"password": "wrong", tc.sibling: "0000"})

			s.ErrorIs(err, ErrAuthenticationFailed)
			s.Equal(e.ID, mismatchedEntityID)
		})
	}
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_CorrectSecretBesideAnIndeterminateOneProducesNoEvidence() {
	e := testEntity("count-12")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{
			Entity: e,
			SchemaCredentials: json.RawMessage(`{` +
				`"password":[{"value":"testhash","storageAlgo":"PBKDF2",` +
				`"storageAlgoParams":{"salt":"testsalt","iterations":1,"keySize":32}}],` +
				`"pin":[{"value":"brokenhash","storageAlgo":"LEGACY",` +
				`"storageAlgoParams":{"salt":"s","iterations":1,"keySize":32}}]}`),
		}, nil)
	s.hashService.On("Verify", []byte("right"), mock.Anything).Return(true, nil)
	s.hashService.On("Verify", []byte("0000"), mock.Anything).
		Return(false, errors.New("unsupported algorithm"))

	for range 8 {
		_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
			map[string]interface{}{"password": "right", "pin": "0000"})
		s.ErrorIs(err, ErrAuthenticationFailed)
		s.Empty(mismatchedEntityID)
	}
}

func (s *ServiceTestSuite) TestAuthenticateEntityByID_StepThatVerifiesNoSecretProducesNoEvidence() {
	e := testEntity("count-9")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: testCredentialsJSON()}, nil)

	_, mismatchedEntityID, err := s.svc.AuthenticateEntityByID(s.ctx, e.ID,
		map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: "count-9"})
	s.ErrorIs(err, ErrAuthenticationFailed)
	s.Empty(mismatchedEntityID)
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
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
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
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.Anything).Return(nil).Once()

	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, json.RawMessage(`{"name":"New"}`)))
}

// An accessState key in system attributes is profile data and does not reach the runtime row.
func (s *ServiceTestSuite) TestUpdateSystemAttributes_DoesNotTouchRuntimeState() {
	e := testEntity("e-runtime")
	e.SystemAttributes = json.RawMessage(`{"name":"Old"}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil)
	var written json.RawMessage
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Run(func(args mock.Arguments) { written, _ = args.Get(2).(json.RawMessage) }).Return(nil)

	sent := json.RawMessage(`{"name":"New","accessState":{"suspend":{}}}`)
	s.NoError(s.svc.UpdateSystemAttributes(s.ctx, e.ID, sent))

	s.JSONEq(string(sent), string(written))
	s.runtime.AssertNotCalled(s.T(), "SetAccessSuspension", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
	s.runtime.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

// installRuntimeFixture lets the runtime store succeed with no rows, for tests about the profile.
// Runtime behavior is tested with newServiceWithRuntimeMock.
func installRuntimeFixture(store *entityRuntimeStoreInterfaceMock) {
	store.On("GetRuntimeData", mock.Anything, mock.Anything).Return(map[string]entityRuntimeData{}, nil).Maybe()
	store.On("ResetRuntimeData", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	store.On("DeleteRuntimeData", mock.Anything, mock.Anything).Return(nil).Maybe()
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
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(), nil, nil)
}

// The governed read carries the category, the parsed lock state and the runtime row's revision.
func (s *ServiceTestSuite) TestGetGovernedEntityReadsTheRuntimeRow() {
	e := testEntity("e-gov")
	e.OUID = "ou-1"
	s.governedRead(e, providers.EntityStateActive, `{"accessState":{"lock":{"authenticationMethods":`+
		`{"credential":{"failureCount":3,"lockCount":2,"unlockAt":"2026-09-08T10:15:00Z",`+
		`"lastFailedAt":"2026-09-08T10:00:00Z"}}}}}`)

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
	s.Require().NoError(err)
	s.Equal(e.ID, governed.ID)
	s.Equal(e.Category, governed.Category)
	s.Equal(e.Type, governed.Type)
	s.Equal("ou-1", governed.OUID)
	s.Equal(providers.EntityStateActive, governed.State)

	credential := governed.AccessState.Lock.AuthenticationMethods[providers.AuthenticationMethodCredential]
	s.Equal(int64(7), governed.Revision)
	s.Equal(3, credential.FailureCount)
	s.Equal(2, credential.LockCount)
	s.Equal("2026-09-08T10:15:00Z", credential.UnlockAt)
	s.Equal("2026-09-08T10:00:00Z", credential.LastFailedAt)
}

func (s *ServiceTestSuite) TestGetGovernedEntityParsesSuspensionAndAccountHold() {
	e := testEntity("e-gov-held")
	s.governedRead(e, providers.EntityStateSuspended, `{"accessState":{"suspend":`+
		`{"suspendedAt":"2026-09-10T11:02:00Z","operatorNote":"Credential exposure"},`+
		`"lock":{"account":{"unlockAt":"9999-12-31T23:59:59Z","reason":"POST_SUSPENDED"}},`+
		`"lastLoginAt":"2026-09-09T18:22:04Z"}}`)

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
	s.Require().NoError(err)
	s.Equal(providers.EntityStateSuspended, governed.State)
	s.Require().NotNil(governed.AccessState.Suspend)
	s.Equal("2026-09-10T11:02:00Z", governed.AccessState.Suspend.SuspendedAt)
	s.Equal("Credential exposure", governed.AccessState.Suspend.OperatorNote)
	s.Equal("2026-09-09T18:22:04Z", governed.AccessState.LastLoginAt)
	s.Require().NotNil(governed.AccessState.Lock.Account)
	s.Equal(int64(7), governed.Revision)
	s.Equal(testPermanentUnlockAt, governed.AccessState.Lock.Account.UnlockAt)
	s.Equal(testPostSuspensionReason, governed.AccessState.Lock.Account.Reason)
	s.Empty(governed.AccessState.Lock.AuthenticationMethods)
}

func (s *ServiceTestSuite) TestGetGovernedEntityWithNoAccessState() {
	for _, attrs := range []string{"", `{"name":"only"}`} {
		e := testEntity("e-gov-empty")
		s.governedRead(e, providers.EntityStateActive, attrs)

		governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
		s.Require().NoError(err)
		s.Equal(identitygovernance.AccessState{}, governed.AccessState)
	}
}

// An unknown method is kept and an unknown member is ignored.
func (s *ServiceTestSuite) TestGetGovernedEntityToleratesUnknownKeys() {
	e := testEntity("e-gov-future")
	s.governedRead(e, providers.EntityStateActive, `{"accessState":{"somethingLater":{"x":1},`+
		`"lock":{"authenticationMethods":{"some_future_method":{"failureCount":2}}}}}`)

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
	s.Require().NoError(err)
	s.Equal(2, governed.AccessState.Lock.AuthenticationMethods["some_future_method"].FailureCount)
}

// An access-state document that cannot be parsed fails the read.
func (s *ServiceTestSuite) TestGetGovernedEntityRefusesAnUnparsableDocument() {
	e := testEntity("e-gov-bad")
	s.governedRead(e, providers.EntityStateActive, `not json`)

	_, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
	s.Error(err)
}

// The lifespan end is read from the system attributes, so governance can refuse an expired agent.
func (s *ServiceTestSuite) TestGetGovernedEntityReadsTheLifespanEnd() {
	for name, tc := range map[string]struct {
		systemAttributes string
		want             string
	}{
		"bounded":     {`{"validUntil":"2026-10-30T00:00:00Z"}`, "2026-10-30T00:00:00Z"},
		"with offset": {`{"validUntil":"2026-10-30T05:30:00+05:30"}`, "2026-10-30T00:00:00Z"},
		"unbounded":   {`{"validUntil":"` + authnprovidercm.UnboundedValidUntil + `"}`, ""},
		"absent":      {`{"owner":"o1"}`, ""},
	} {
		e := testEntity("e-gov-lifespan-" + name)
		e.SystemAttributes = json.RawMessage(tc.systemAttributes)
		s.governedRead(e, providers.EntityStateActive, "")

		governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
		s.Require().NoError(err, name)
		s.Equal(tc.want, governed.ValidUntil, name)
	}
}

// A lifespan end that does not parse is handed over as an error rather than failing the read, so a
// lifecycle operation such as a suspension can still read the entity.
func (s *ServiceTestSuite) TestGetGovernedEntityReportsAnUnparsableLifespanEnd() {
	e := testEntity("e-gov-bad-lifespan")
	e.SystemAttributes = json.RawMessage(`{"validUntil":"not-a-timestamp"}`)
	s.governedRead(e, providers.EntityStateActive, "")

	governed, err := s.svc.GetGovernedEntity(s.ctx, e.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(governed)
	s.Error(governed.ValidUntilErr)
	s.Empty(governed.ValidUntil)
}

// An unknown identity reads as no entity, not an error, and its runtime row is neither read nor seeded.
func (s *ServiceTestSuite) TestGetGovernedEntityReportsAnUnknownIdentity() {
	svc, runtime := s.newServiceWithRuntimeMock()
	s.store.On("GetEntity", mock.Anything, "missing").
		Return(providers.Entity{}, ErrEntityNotFound).Once()

	governed, err := svc.GetGovernedEntity(s.ctx, "missing", true)

	s.NoError(err)
	s.Nil(governed)
	runtime.AssertNotCalled(s.T(), "GetRuntimeData", mock.Anything, mock.Anything)
	runtime.AssertNotCalled(s.T(), "InitializeRuntimeData", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestGetGovernedEntityPropagatesAReadFailure() {
	readErr := errors.New("store unavailable")
	s.store.On("GetEntity", mock.Anything, "e1").Return(providers.Entity{}, readErr).Once()

	governed, err := s.svc.GetGovernedEntity(s.ctx, "e1", true)
	s.ErrorIs(err, readErr)
	s.Nil(governed)
}

// A profile read touches no runtime state and carries none, not even a declarative file's state.
func (s *ServiceTestSuite) TestGetEntityProfileDoesNotHydrateTheRuntimeRow() {
	svc, runtime := s.newServiceWithRuntimeMock()
	e := testEntity("e-category")
	e.Category = providers.EntityCategoryAgent
	e.State = providers.EntityStateSuspended
	e.RuntimeAttributes = json.RawMessage(`{"accessState":{}}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()

	profile, err := svc.GetEntityProfile(s.ctx, e.ID)

	s.Require().NoError(err)
	s.Equal(providers.EntityCategoryAgent, profile.Category)
	s.Empty(profile.State)
	s.Nil(profile.RuntimeAttributes)
	runtime.AssertNotCalled(s.T(), "GetRuntimeData", mock.Anything, mock.Anything)
	runtime.AssertNotCalled(s.T(), "InitializeRuntimeData", mock.Anything, mock.Anything)
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
		transaction.NewNoOpTransactioner(), nil, nil).(*entityService)
	return svc, store
}

func (s *ServiceTestSuite) TestCreateEntity_TooManyIndexedValuesRejectedBeforeSchemaValidation() {
	svc, _ := s.newServiceWithIndexedEmail()
	e := testEntity("e1")
	e.Attributes = emailArrayAttrs(maxIndexedValuesPerAttribute + 1)

	_, err := svc.CreateEntity(s.ctx, e, nil, nil)
	s.ErrorIs(err, ErrSchemaValidationFailed)
	s.ErrorIs(err, ErrIndexedValueLimitExceeded)
}

func (s *ServiceTestSuite) TestCreateEntity_TooManyIndexedSystemValuesRejected() {
	svc, _ := s.newServiceWithIndexedEmail()
	e := testEntity("e1")
	e.SystemAttributes = emailArrayAttrs(maxIndexedValuesPerAttribute + 1)

	_, err := svc.CreateEntity(s.ctx, e, nil, nil)
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
	store.On("UpdateSystemAttributes", mock.Anything, "e1", mock.IsType(json.RawMessage(nil))).Return(nil)

	s.NoError(svc.UpdateSystemAttributes(s.ctx, "e1", attrs))
}

// declaresCredentials sets the attributes an entity type declares as credentials.
func declaresCredentials(ets *entitytypemock.EntityTypeServiceInterfaceMock, category entitytype.TypeCategory,
	entityType string, attributes ...string) {
	infos := make([]entitytype.AttributeInfo, 0, len(attributes))
	for _, attribute := range attributes {
		infos = append(infos, entitytype.AttributeInfo{Attribute: attribute, Credential: true})
	}
	ets.On("GetAttributes", mock.Anything, category, entityType,
		entitytype.AttributeFilter{AllowCredential: true}).Return(infos, (*tidcommon.ServiceError)(nil))
}

// Only what the type declares and the entity holds is reported, whichever column holds it. A client
// secret the entity also holds is not a declared credential.
func (s *ServiceTestSuite) TestGetHeldCredentialTypes_ReportsTheDeclaredCredentialsHeld() {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity("e-held")
	e.Category = providers.EntityCategoryAgent
	e.Type = "default"
	declaresCredentials(ets, entitytype.TypeCategoryAgent, "default", "pin", "password", "otp")
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).Return(&entityWithCredentials{
		Entity:            e,
		SchemaCredentials: json.RawMessage(`{"password":[{"value":"h1"}],"otp":[]}`),
		SystemCredentials: json.RawMessage(`{"clientSecret":[{"value":"h2"}],"pin":[{"value":"h3"}]}`),
	}, nil)

	types, err := svc.GetHeldCredentialTypes(s.ctx, e.ID)

	s.NoError(err)
	s.ElementsMatch([]string{"pin", "password"}, types)
}

func (s *ServiceTestSuite) TestGetHeldCredentialTypes_NoneDeclared() {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity("e-none")
	declaresCredentials(ets, entitytype.TypeCategoryUser, e.Type)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).Return(&entityWithCredentials{
		Entity: e, SchemaCredentials: json.RawMessage(`{"password":[{"value":"h1"}]}`),
	}, nil)

	types, err := svc.GetHeldCredentialTypes(s.ctx, e.ID)

	s.NoError(err)
	s.Empty(types)
}

// An application has no type to declare credentials, so it holds none an export could carry.
func (s *ServiceTestSuite) TestGetHeldCredentialTypes_CategoryWithoutATypeHoldsNone() {
	svc, _ := s.newSvcWithEntityType()
	e := testEntity("e-app")
	e.Category = providers.EntityCategoryApp
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).Return(&entityWithCredentials{
		Entity: e, SystemCredentials: json.RawMessage(`{"clientSecret":[{"value":"h1"}]}`),
	}, nil)

	types, err := svc.GetHeldCredentialTypes(s.ctx, e.ID)

	s.NoError(err)
	s.Empty(types)
}

func (s *ServiceTestSuite) TestGetHeldCredentialTypes_Failures() {
	cases := []struct {
		name  string
		setup func(svc *entityService, ets *entitytypemock.EntityTypeServiceInterfaceMock) string
		want  error
	}{
		{name: "store error", setup: func(_ *entityService, _ *entitytypemock.EntityTypeServiceInterfaceMock) string {
			s.store.On("GetEntityWithCredentials", mock.Anything, "e-store").Return(nil, s.testErr).Once()
			return "e-store"
		}, want: s.testErr},
		{name: "no entity", setup: func(_ *entityService, _ *entitytypemock.EntityTypeServiceInterfaceMock) string {
			s.store.On("GetEntityWithCredentials", mock.Anything, "e-nil").
				Return(&entityWithCredentials{}, nil).Once()
			return "e-nil"
		}, want: ErrEntityNotFound},
		{name: "type error", setup: func(_ *entityService, ets *entitytypemock.EntityTypeServiceInterfaceMock) string {
			e := testEntity("e-type")
			s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
				Return(&entityWithCredentials{Entity: e}, nil).Once()
			ets.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, e.Type,
				entitytype.AttributeFilter{AllowCredential: true}).
				Return(nil, &tidcommon.ServiceError{Code: "ETS-5000", Type: tidcommon.ServerErrorType}).Once()
			return e.ID
		}},
		{name: "type not found", setup: func(
			_ *entityService, ets *entitytypemock.EntityTypeServiceInterfaceMock) string {
			e := testEntity("e-gone")
			s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
				Return(&entityWithCredentials{Entity: e}, nil).Once()
			ets.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, e.Type,
				entitytype.AttributeFilter{AllowCredential: true}).
				Return(nil, &tidcommon.ServiceError{Code: "USRS-1002", Type: tidcommon.ClientErrorType}).Once()
			return e.ID
		}, want: ErrSchemaValidationFailed},
		{name: "malformed column", setup: func(
			_ *entityService, ets *entitytypemock.EntityTypeServiceInterfaceMock) string {
			e := testEntity("e-bad")
			declaresCredentials(ets, entitytype.TypeCategoryUser, e.Type, "password")
			s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).Return(&entityWithCredentials{
				Entity: e, SchemaCredentials: json.RawMessage(`{"password":"not-a-list"}`),
			}, nil).Once()
			return e.ID
		}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			svc, ets := s.newSvcWithEntityType()
			id := tc.setup(svc, ets)

			types, err := svc.GetHeldCredentialTypes(s.ctx, id)

			s.Error(err)
			if tc.want != nil {
				s.ErrorIs(err, tc.want)
			}
			s.Nil(types)
		})
	}
}

// The values come from the declared credentials alone; an empty one and an undeclared attribute are
// left out.
func (s *ServiceTestSuite) TestGetDeclaredCredentialValues_ReadsTheDeclaredCredentials() {
	svc, ets := s.newSvcWithEntityType()
	declaresCredentials(ets, entitytype.TypeCategoryAgent, "default", "password", "pin", "otp")

	values, err := svc.GetDeclaredCredentialValues(s.ctx, providers.EntityCategoryAgent, "default",
		json.RawMessage(`{"name":"bot","password":"s3cret","pin":"","otp":7,"secret":"not declared"}`))

	s.NoError(err)
	s.Equal(map[string]string{"password": "s3cret"}, values)
}

func (s *ServiceTestSuite) TestGetDeclaredCredentialValues_NoAttributesAskNothing() {
	svc, _ := s.newSvcWithEntityType()

	values, err := svc.GetDeclaredCredentialValues(s.ctx, providers.EntityCategoryUser, "employee", nil)

	s.NoError(err)
	s.Empty(values)
}

func (s *ServiceTestSuite) TestGetDeclaredCredentialValues_Failures() {
	s.Run("type error", func() {
		svc, ets := s.newSvcWithEntityType()
		ets.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "employee",
			entitytype.AttributeFilter{AllowCredential: true}).
			Return(nil, &tidcommon.ServiceError{Code: "ETS-1001"})

		_, err := svc.GetDeclaredCredentialValues(s.ctx, providers.EntityCategoryUser, "employee",
			json.RawMessage(`{"password":"s3cret"}`))

		s.Error(err)
	})
	s.Run("malformed attributes", func() {
		svc, ets := s.newSvcWithEntityType()
		declaresCredentials(ets, entitytype.TypeCategoryUser, "employee", "password")

		_, err := svc.GetDeclaredCredentialValues(s.ctx, providers.EntityCategoryUser, "employee",
			json.RawMessage(`[1]`))

		s.Error(err)
	})
}
func (s *ServiceTestSuite) TestProjectMethods_Delegate() {
	e := testEntity("project-entity")
	s.store.On("GetEntityListCountByProject", mock.Anything, "app", testProjectID, []string(nil)).Return(1, nil)
	s.store.On("GetEntityListByProject", mock.Anything, "app", testProjectID, []string(nil), 10, 0).
		Return([]providers.Entity{*e}, nil)
	s.store.On("ProjectHasEntities", mock.Anything, testProjectID).Return(true, nil)
	s.store.On("UpdateOwnership", mock.Anything, e.ID, "ou-2", testProjectID).Return(nil)

	count, err := s.svc.GetEntityListCountByProject(s.ctx, providers.EntityCategoryApp, testProjectID, nil)
	s.NoError(err)
	s.Equal(1, count)
	list, err := s.svc.GetEntityListByProject(s.ctx, providers.EntityCategoryApp, testProjectID, nil, 10, 0)
	s.NoError(err)
	s.Len(list, 1)
	found, err := s.svc.ProjectHasEntities(s.ctx, testProjectID)
	s.NoError(err)
	s.True(found)
	s.NoError(s.svc.UpdateOwnership(s.ctx, e.ID, "ou-2", testProjectID))
}

func (s *ServiceTestSuite) TestGetEntityListByProject_Errors() {
	s.Run("store error", func() {
		s.SetupTest()
		s.store.On("GetEntityListByProject", mock.Anything, "app", testProjectID, []string(nil), 10, 0).
			Return(nil, s.testErr)

		_, err := s.svc.GetEntityListByProject(s.ctx, providers.EntityCategoryApp, testProjectID, nil, 10, 0)

		s.ErrorIs(err, s.testErr)
	})
	s.Run("runtime read error", func() {
		s.SetupTest()
		svc, runtime := s.newServiceWithRuntimeMock()
		e := testEntity("project-runtime")
		s.store.On("GetEntityListByProject", mock.Anything, "app", testProjectID, []string(nil), 10, 0).
			Return([]providers.Entity{*e}, nil)
		runtime.On("GetRuntimeData", mock.Anything, []string{e.ID}).Return(nil, s.testErr)

		_, err := svc.GetEntityListByProject(s.ctx, providers.EntityCategoryApp, testProjectID, nil, 10, 0)

		s.ErrorIs(err, s.testErr)
	})
}
func (s *ServiceTestSuite) TestCreateEntityDropsIncomingVerification() {
	e := testEntity("create-verification")
	e.SystemAttributes = json.RawMessage(`{"keep":true,"attributeVerification":` +
		`{"email":{"verifiedAt":"forged"}}}`)
	s.store.On("IsEntityDeclarative", mock.Anything, e.ID).Return(false, ErrEntityNotFound)
	s.store.On("CreateEntity", mock.Anything, mock.MatchedBy(func(created providers.Entity) bool {
		return string(created.SystemAttributes) == `{"keep":true}`
	}), json.RawMessage(nil), json.RawMessage(nil)).Return(nil).Once()
	stored := *e
	stored.SystemAttributes = json.RawMessage(`{"keep":true}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(stored, nil)
	created, err := s.svc.CreateEntity(s.ctx, e, nil, nil)
	s.Require().NoError(err)
	s.JSONEq(`{"keep":true}`, string(created.SystemAttributes))
}

func (s *ServiceTestSuite) TestUpdateSystemAttributesPreservesVerification() {
	e := testEntity("update-system-verification")
	e.SystemAttributes = json.RawMessage(`{"attributeVerification":{"email":{"verifiedAt":"2026-10-14T00:00:00Z"}}}`)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.MatchedBy(func(raw json.RawMessage) bool {
		entries, err := ReadRecordedAttributeVerifications(raw)
		return err == nil && entries["email"] == (providers.VerifiedAttribute{VerifiedAt: "2026-10-14T00:00:00Z"})
	})).Return(nil).Once()
	err := s.svc.UpdateSystemAttributes(s.ctx, e.ID,
		json.RawMessage(`{"attributeVerification":{"email":{"verifiedAt":"forged"}}}`))
	s.NoError(err)
}

func (s *ServiceTestSuite) TestUpdateAttributesRefusesUnreadableVerification() {
	e := testEntity("malformed-verification")
	e.SystemAttributes = json.RawMessage(`{"attributeVerification":{"email":{"verifiedAt":5}}}`)
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Once()
	err := s.svc.UpdateAttributes(s.ctx, e.ID, json.RawMessage(`{"username":"new"}`))
	s.Error(err)
	s.store.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "UpdateEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestVerificationSurvivesUnrelatedMetadataWrites() {
	original := json.RawMessage(`{"attributeVerification":{"email":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)
	merged, err := s.impl().mergeReservedAttributes(original, json.RawMessage(`{"keep":true}`))
	s.Require().NoError(err)
	credential, err := withSystemAttribute(original, authnprovidercm.SystemAttrCredentialUpdatedAt,
		time.Now().UTC().Format(time.RFC3339))
	s.Require().NoError(err)
	linked, err := withLinkedSubject(original, "connection-1", "subject-1")
	s.Require().NoError(err)
	for _, value := range []json.RawMessage{merged, credential, linked} {
		entries, err := ReadRecordedAttributeVerifications(value)
		s.Require().NoError(err)
		s.Equal("2026-10-15T00:00:00Z", entries["email"].VerifiedAt)
	}
}

// declaredVerifiableAttributes are the verifiable attributes contactTypeService declares.
var declaredVerifiableAttributes = map[string]string{"workEmail": "email", "mobile": "mobile"}

func contactTypeService(t *testing.T) *entitytypemock.EntityTypeServiceInterfaceMock {
	t.Helper()
	return schemaService(t, []entitytype.AttributeInfo{
		{Attribute: "workEmail", Type: "email"}, {Attribute: "mobile", Type: "mobile"},
		{Attribute: "nested", Type: "object"}, {Attribute: "many", Type: "array"},
		{Attribute: "nickname", Type: "string"},
	})
}

// schemaService is a user type schema declaring the given attributes.
func schemaService(t *testing.T, attributes []entitytype.AttributeInfo) *entitytypemock.EntityTypeServiceInterfaceMock {
	t.Helper()
	return declareSchema(entitytypemock.NewEntityTypeServiceInterfaceMock(t), attributes)
}

// declareSchema stubs the schema reads and validations of a type declaring the given attributes, after
// any expectation the schema already has.
func declareSchema(schema *entitytypemock.EntityTypeServiceInterfaceMock,
	attributes []entitytype.AttributeInfo) *entitytypemock.EntityTypeServiceInterfaceMock {
	schema.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything,
		entitytype.AttributeFilter{AllowNonCredential: true}).Return(attributes, nil).Maybe()
	schema.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).Return(attributes, nil).Maybe()
	schema.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything,
		entitytype.AttributeFilter{AllowCredential: true}).Return([]entitytype.AttributeInfo{}, nil).Maybe()
	schema.On("ValidateEntity", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, true).Return(true, nil).Maybe()
	schema.On("ValidateEntityUniqueness", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return(true, nil).Maybe()
	return schema
}

// verifiedWorkEmail is stored verification metadata with workEmail verified.
const verifiedWorkEmail = `{"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`

// verificationUser is user-1 in ou1 with the given attributes and system attributes.
func verificationUser(attributes, systemAttributes string) providers.Entity {
	return providers.Entity{ID: "user-1", OUID: "ou1", Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(attributes), SystemAttributes: json.RawMessage(systemAttributes)}
}

// recordWorkEmail records user-1's person workEmail at a@example.com.
func recordWorkEmail(svc *entityService, at time.Time) (bool, error) {
	return svc.RecordAttributeVerification(context.Background(), AttributeVerificationRecord{EntityID: "user-1",
		EntityCategory: providers.EntityCategoryUser, EntityType: "person",
		Attribute: "workEmail", Value: "a@example.com", VerifiedAt: at})
}

// txKey marks a context as inside a markingTransactioner transaction.
type txKey struct{}

// markingTransactioner runs each transaction inline on a context marked as inside it.
func markingTransactioner(t *testing.T) *transactionmock.TransactionerMock {
	m := transactionmock.NewTransactionerMock(t)
	m.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, txFunc func(context.Context) error) error {
			return txFunc(context.WithValue(ctx, txKey{}, true))
		}).Maybe()
	return m
}

// inTransaction matches a context inside a markingTransactioner transaction.
func inTransaction() interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txKey{}) != nil })
}

// outsideTransaction matches a context outside a markingTransactioner transaction.
func outsideTransaction() interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txKey{}) == nil })
}

// expectAttributeUpdate stubs the read of an attribute update before its transaction and the locked
// read inside it.
func expectAttributeUpdate(store *entityStoreInterfaceMock, current providers.Entity) {
	store.On("GetEntity", outsideTransaction(), current.ID).Return(current, nil).Once()
	store.On("LockEntity", inTransaction(), current.ID).Return(current, nil).Once()
}

// withoutVerification matches system attributes that record no verification.
func withoutVerification() interface{} {
	return mock.MatchedBy(func(raw json.RawMessage) bool {
		entries, err := ReadRecordedAttributeVerifications(raw)
		return err == nil && len(entries) == 0
	})
}

// verificationService is a service over the store and the runtime fixture whose transactions mark
// their context.
func (s *ServiceTestSuite) verificationService(store *entityStoreInterfaceMock,
	schema entitytype.EntityTypeServiceInterface) *entityService {
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	return &entityService{store: store, runtimeStore: runtime, entityTypeService: schema,
		transactioner: markingTransactioner(s.T()), logger: log.GetLogger()}
}

func (s *ServiceTestSuite) TestCreateAndGenericWritesCannotSetVerification() {
	incoming := json.RawMessage(`{"attributeVerification":` +
		`{"workEmail":{"verifiedAt":"forged"}},"keep":true}`)
	stripped, err := s.impl().withoutCreateProtectedAttributes(incoming)
	s.Require().NoError(err)
	s.Require().JSONEq(`{"keep":true}`, string(stripped))
	merged, err := s.impl().mergeReservedAttributes(nil, incoming)
	s.Require().NoError(err)
	s.Require().JSONEq(`{"keep":true}`, string(merged))
	current := json.RawMessage(`{"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-14T00:00:00Z"}}}`)
	merged, err = s.impl().mergeReservedAttributes(current, incoming)
	s.Require().NoError(err)
	s.Require().JSONEq(`{"keep":true,"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-14T00:00:00Z"}}}`,
		string(merged))
}

func (s *ServiceTestSuite) TestUpdateEntityInvalidatesFromTheLockedProfile() {
	t := s.T()
	store := newEntityStoreInterfaceMock(t)
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	current := providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"old@example.com"}`),
		SystemAttributes: json.RawMessage(
			`{"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-16T00:00:00Z"}}}`),
	}
	store.On("LockEntity", mock.Anything, current.ID).Return(current, nil).Once()
	store.On("UpdateEntity", mock.Anything, mock.MatchedBy(func(next *providers.Entity) bool {
		entries, err := ReadRecordedAttributeVerifications(next.SystemAttributes)
		return err == nil && len(entries) == 0
	})).Return(nil).Once()
	store.On("GetEntity", mock.Anything, current.ID).Return(current, nil).Once()
	attributes := []entitytype.AttributeInfo{{Attribute: "workEmail", Type: "email"}}
	schema := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schema.On("GetAttributes", outsideTransaction(), mock.Anything, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).Return(attributes, nil).Once()
	schema.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).Return(attributes, nil).Maybe()
	schema.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything,
		entitytype.AttributeFilter{AllowCredential: true}).Return([]entitytype.AttributeInfo{}, nil).Maybe()
	schema.On("ValidateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, true).
		Return(true, nil).Maybe()
	schema.On("ValidateEntityUniqueness", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(true, nil).Maybe()
	svc := s.verificationService(store, schema)
	next := current
	next.Attributes = json.RawMessage(`{"workEmail":"new@example.com"}`)
	next.SystemAttributes = json.RawMessage(`{"attributeVerification":` +
		`{"workEmail":{"verifiedAt":"forged"}}}`)
	_, err := svc.UpdateEntity(context.Background(), current.ID, &next)
	s.Require().NoError(err)
}

// The verifiable attributes are read before the transaction, so an unreadable schema refuses the write before
// the lock.
func (s *ServiceTestSuite) TestInvalidationRefusesAnUnreadableSchema() {
	failingSchema := func() *entitytypemock.EntityTypeServiceInterfaceMock {
		schema := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
		schema.On("GetAttributes", outsideTransaction(), mock.Anything, "person",
			entitytype.AttributeFilter{AllowNonCredential: true}).
			Return(nil, &tidcommon.InternalServerError).Once()
		return declareSchema(schema, []entitytype.AttributeInfo{{Attribute: "workEmail", Type: "email"}})
	}
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)

	s.Run("update attributes", func() {
		store := newEntityStoreInterfaceMock(s.T())
		store.On("GetEntity", outsideTransaction(), current.ID).Return(current, nil).Once()
		svc := s.verificationService(store, failingSchema())

		err := svc.UpdateAttributes(context.Background(), current.ID, json.RawMessage(`{}`))

		s.ErrorContains(err, tidcommon.InternalServerError.Code)
	})
	s.Run("update entity", func() {
		svc := s.verificationService(newEntityStoreInterfaceMock(s.T()), failingSchema())
		next := current

		_, err := svc.UpdateEntity(context.Background(), current.ID, &next)

		s.ErrorContains(err, tidcommon.InternalServerError.Code)
	})
}

func (s *ServiceTestSuite) TestUpdateAttributesValidatesBeforeTheTransaction() {
	store := newEntityStoreInterfaceMock(s.T())
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).Return(nil).Once()
	schema := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	schema.On("GetAttributes", outsideTransaction(), mock.Anything, "person",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "workEmail", Type: "email"}}, nil).Once()
	schema.On("GetAttributes", outsideTransaction(), mock.Anything, "person",
		entitytype.AttributeFilter{AllowCredential: true}).Return([]entitytype.AttributeInfo{}, nil).Once()
	schema.On("GetAttributes", outsideTransaction(), mock.Anything, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "workEmail", Type: "email"}}, nil).Once()
	schema.On("ValidateEntity", outsideTransaction(), mock.Anything, mock.Anything, mock.Anything, true).
		Return(true, nil).Once()
	schema.On("ValidateEntityUniqueness", outsideTransaction(), mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(true, nil).Once()
	svc := s.verificationService(store, schema)

	err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{"workEmail":"b@example.com"}`))

	s.NoError(err)
}

// lockedUser is a store that returns the given user under the lock and expects no write.
func (s *ServiceTestSuite) lockedUser(current providers.Entity) (*entityService, *entityStoreInterfaceMock) {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("LockEntity", mock.Anything, current.ID).Return(current, nil).Once()
	return &entityService{store: store, entityTypeService: contactTypeService(s.T()),
		transactioner: transaction.NewNoOpTransactioner(), logger: log.GetLogger()}, store
}

func (s *ServiceTestSuite) TestRecordRefusesAUserDeletedBeforeTheLock() {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("LockEntity", mock.Anything, "user-1").Return(providers.Entity{}, ErrEntityNotFound).Once()
	svc := &entityService{store: store, entityTypeService: contactTypeService(s.T()),
		transactioner: transaction.NewNoOpTransactioner(), logger: log.GetLogger()}

	applied, err := recordWorkEmail(svc, time.Now())

	s.NoError(err)
	s.False(applied)
}

// The caller pinned the type and canonical value before the lock; a record whose locked user no longer
// matches writes nothing.
func (s *ServiceTestSuite) TestRecordRefusesAChangedEntity() {
	user := func(userType, attributes string) providers.Entity {
		return providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser, Type: userType,
			Attributes: json.RawMessage(attributes)}
	}
	for name, current := range map[string]providers.Entity{
		"other type":    user("employee", `{"workEmail":"a@example.com"}`),
		"other value":   user("person", `{"workEmail":"b@example.com"}`),
		"other local":   user("person", `{"workEmail":"A@example.com"}`),
		"removed":       user("person", `{}`),
		"no attributes": user("person", ``),
		"not a string":  user("person", `{"workEmail":42}`),
		"not a contact": user("person", `{"nickname":"a@example.com"}`),
		"other category": {ID: "user-1", Category: providers.EntityCategoryAgent, Type: "person",
			Attributes: json.RawMessage(`{"workEmail":"a@example.com"}`)},
	} {
		s.Run(name, func() {
			svc, _ := s.lockedUser(current)

			applied, err := recordWorkEmail(svc, time.Now())

			s.NoError(err)
			s.False(applied)
		})
	}
}

// Recording compares canonical values, as invalidation does, so a value that differs only in the case of
// its email domain or surrounding whitespace is still the pinned contact.
func (s *ServiceTestSuite) TestRecordMatchesTheCanonicalValue() {
	svc, store := s.lockedUser(verificationUser(`{"workEmail":" a@EXAMPLE.com "}`, `{}`))
	store.On("UpdateSystemAttributes", mock.Anything, "user-1", mock.Anything).Return(nil).Once()

	applied, err := recordWorkEmail(svc, time.Now())

	s.NoError(err)
	s.True(applied)
}

// An attribute the user type no longer declares as a contact is not recorded.
func (s *ServiceTestSuite) TestRecordRefusesAnAttributeThatIsNoLongerAContact() {
	svc, _ := s.lockedUser(verificationUser(`{"workEmail":"a@example.com"}`, `{}`))
	svc.entityTypeService = schemaService(s.T(), []entitytype.AttributeInfo{{Attribute: "workEmail", Type: "string"}})

	applied, err := recordWorkEmail(svc, time.Now())

	s.NoError(err)
	s.False(applied)
}

func (s *ServiceTestSuite) TestRecordRefusesAnEmptyValue() {
	svc := &entityService{logger: log.GetLogger()}

	applied, err := svc.RecordAttributeVerification(context.Background(), AttributeVerificationRecord{
		EntityID: "user-1", EntityType: "person", Attribute: "workEmail", VerifiedAt: time.Now()})

	s.NoError(err)
	s.False(applied)
}

func (s *ServiceTestSuite) TestRecordPersistenceFailureRecordsNothing() {
	svc, store := s.lockedUser(verificationUser(`{"workEmail":"a@example.com"}`, `{}`))
	writeErr := errors.New("write failed")
	store.On("UpdateSystemAttributes", mock.Anything, "user-1", mock.Anything).Return(writeErr).Once()

	applied, err := recordWorkEmail(svc, time.Now())

	s.ErrorIs(err, writeErr)
	s.False(applied)
}

// captureEvents returns an observability service whose events land in the returned slice.
func (s *ServiceTestSuite) captureEvents() (*observabilitymock.ObservabilityServiceInterfaceMock,
	*[]*providers.Event) {
	published := []*providers.Event{}
	obs := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	obs.On("IsEnabled").Return(true).Maybe()
	obs.On("PublishEvent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { published = append(published, args.Get(1).(*providers.Event)) }).Maybe()
	return obs, &published
}

func (s *ServiceTestSuite) TestAUserTypeChangePublishesWithTheCaller() {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	current := providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "person",
		Attributes:       json.RawMessage(`{"workEmail":"a@example.com"}`),
		SystemAttributes: json.RawMessage(verifiedWorkEmail)}
	store.On("LockEntity", mock.Anything, current.ID).Return(current, nil).Once()
	store.On("UpdateEntity", mock.Anything, mock.Anything).Return(nil).Once()
	store.On("GetEntity", mock.Anything, current.ID).Return(current, nil).Once()
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	obs, published := s.captureEvents()
	svc := &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
		transactioner: transaction.NewNoOpTransactioner(), observability: obs, logger: log.GetLogger()}
	next := current
	next.Type = "employee"
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("operator", "ou1", "", nil, nil))

	_, err := svc.UpdateEntity(ctx, current.ID, &next)

	s.Require().NoError(err)
	s.Require().Len(*published, 1)
	s.Equal(string(verificationUserTypeChanged),
		(*published)[0].Data[event.DataKey.InvalidationReason])
	s.Equal("operator", (*published)[0].Data[event.DataKey.ActorSub])
}

func (s *ServiceTestSuite) TestUpdateAttributesClearsAChangedContactInItsTransaction() {
	store := newEntityStoreInterfaceMock(s.T())
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"old@example.com"}`, verifiedWorkEmail))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).Return(nil).Once()
	svc := s.verificationService(store, contactTypeService(s.T()))

	err := svc.UpdateAttributes(context.Background(), "user-1",
		json.RawMessage(`{"workEmail":"new@example.com"}`))

	s.NoError(err)
}

// An agent has no verifiable contacts: its update reads no user contact schema and writes no
// verification metadata, whatever its system attributes hold.
func (s *ServiceTestSuite) TestUpdateAttributesOfAnAgentLeavesVerificationAlone() {
	store := newEntityStoreInterfaceMock(s.T())
	agent := verificationUser(`{"workEmail":"old@example.com"}`, verifiedWorkEmail)
	agent.Category = providers.EntityCategoryAgent
	expectAttributeUpdate(store, agent)
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	schema := contactTypeService(s.T())
	svc := s.verificationService(store, schema)

	err := svc.UpdateAttributes(context.Background(), "user-1",
		json.RawMessage(`{"workEmail":"new@example.com"}`))

	s.NoError(err)
	schema.AssertNotCalled(s.T(), "GetAttributes", mock.Anything, entitytype.TypeCategoryUser, mock.Anything,
		mock.Anything)
	store.AssertNotCalled(s.T(), "UpdateSystemAttributes", mock.Anything, mock.Anything, mock.Anything)
}

// Both writes run in one transaction, so a failed metadata write rolls the contact write back.
func (s *ServiceTestSuite) TestAFailedMetadataWriteFailsTheContactWrite() {
	store := newEntityStoreInterfaceMock(s.T())
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	writeErr := errors.New("metadata write failed")
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", mock.Anything).Return(writeErr).Once()
	obs, published := s.captureEvents()
	svc := s.verificationService(store, contactTypeService(s.T()))
	svc.observability = obs

	err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{"workEmail":"b@example.com"}`))

	s.ErrorIs(err, writeErr)
	s.Empty(*published)
}

func (s *ServiceTestSuite) TestAFailedCommitPublishesNothing() {
	store := newEntityStoreInterfaceMock(s.T())
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)
	store.On("GetEntity", mock.Anything, "user-1").Return(current, nil).Once()
	store.On("LockEntity", mock.Anything, "user-1").Return(current, nil).Once()
	store.On("UpdateAttributes", mock.Anything, "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", mock.Anything, "user-1", withoutVerification()).Return(nil).Once()
	obs, published := s.captureEvents()
	svc := s.verificationService(store, contactTypeService(s.T()))
	svc.observability = obs
	commitErr := errors.New("commit failed")
	tx := transactionmock.NewTransactionerMock(s.T())
	tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, txFunc func(context.Context) error) error {
			if err := txFunc(ctx); err != nil {
				return err
			}
			return commitErr
		}).Once()
	svc.transactioner = tx

	err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{"workEmail":"b@example.com"}`))

	s.ErrorIs(err, commitErr)
	s.Empty(*published)
}

// schemaReadOutsideTransaction is contactTypeService read only outside a markingTransactioner transaction.
func schemaReadOutsideTransaction(t *testing.T) *entitytypemock.EntityTypeServiceInterfaceMock {
	schema := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schema.On("GetAttributes", outsideTransaction(), entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "workEmail", Type: "email"}}, nil).Once()
	return schema
}

func (s *ServiceTestSuite) TestRecordFaultsAreErrors() {
	readErr := errors.New("read failed")
	for name, locked := range map[string]func(*entityStoreInterfaceMock){
		"schema read": nil,
		"entity lock": func(store *entityStoreInterfaceMock) {
			store.On("LockEntity", mock.Anything, "user-1").Return(providers.Entity{}, readErr).Once()
		},
		"malformed attributes": func(store *entityStoreInterfaceMock) {
			store.On("LockEntity", mock.Anything, "user-1").
				Return(verificationUser(`[`, verifiedWorkEmail), nil).Once()
		},
		"malformed metadata": func(store *entityStoreInterfaceMock) {
			store.On("LockEntity", mock.Anything, "user-1").Return(
				verificationUser(`{"workEmail":"a@example.com"}`, `{"attributeVerification":[]}`), nil).Once()
		},
	} {
		s.Run(name, func() {
			store := newEntityStoreInterfaceMock(s.T())
			schema := entitytype.EntityTypeServiceInterface(contactTypeService(s.T()))
			if locked == nil {
				failing := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
				failing.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, &tidcommon.InternalServerError).Once()
				schema = failing
			} else {
				locked(store)
			}
			svc := s.verificationService(store, schema)

			applied, err := recordWorkEmail(svc, time.Now())

			s.Error(err)
			s.False(applied)
		})
	}
}

func (s *ServiceTestSuite) TestRecordWritesOnlyTheVerificationUnderTheEntityLock() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, `{"keep":true}`)
	store := newEntityStoreInterfaceMock(s.T())
	store.On("LockEntity", inTransaction(), "user-1").Return(current, nil).Once()
	var written json.RawMessage
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", mock.Anything).
		Run(func(args mock.Arguments) { written = args.Get(2).(json.RawMessage) }).Return(nil).Once()
	svc := s.verificationService(store, schemaReadOutsideTransaction(s.T()))

	applied, err := recordWorkEmail(svc, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))

	s.Require().NoError(err)
	s.True(applied)
	s.JSONEq(`{"keep":true,"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-16T12:00:00Z"}}}`,
		string(written))
}

func (s *ServiceTestSuite) TestRecordAgainRefreshesVerifiedAt() {
	svc, store := s.lockedUser(verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail))
	var written json.RawMessage
	store.On("UpdateSystemAttributes", mock.Anything, "user-1", mock.Anything).
		Run(func(args mock.Arguments) { written = args.Get(2).(json.RawMessage) }).Return(nil).Once()
	at := time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC)

	applied, err := recordWorkEmail(svc, at)

	s.Require().NoError(err)
	s.True(applied)
	entries, err := ReadRecordedAttributeVerifications(written)
	s.Require().NoError(err)
	s.Equal(providers.VerifiedAttribute{VerifiedAt: at.Format(time.RFC3339Nano)}, entries["workEmail"])
}

func (s *ServiceTestSuite) TestAChangeWhileIneligibleClearsTheVerification() {
	store := newEntityStoreInterfaceMock(s.T())
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).Return(nil).Once()
	svc := s.verificationService(store,
		schemaService(s.T(), []entitytype.AttributeInfo{{Attribute: "workEmail", Type: "string"}}))

	err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{"workEmail":"A@example.com"}`))

	s.NoError(err)
}

func (s *ServiceTestSuite) TestACommittedUpdatePublishesEachClearedVerification() {
	store := newEntityStoreInterfaceMock(s.T())
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"a@example.com","mobile":"+94771234567"}`,
		verifiedWorkEmail))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).Return(nil).Once()
	obs, published := s.captureEvents()
	svc := s.verificationService(store, contactTypeService(s.T()))
	svc.observability = obs

	err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`))

	s.Require().NoError(err)
	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Equal(string(event.EventTypeAttributeVerificationInvalidated), evt.Type)
	s.Equal("user-1", evt.Data[event.DataKey.Subject])
	s.Equal(event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	s.Equal("workEmail", evt.Data[event.DataKey.Attribute])
	s.Equal(string(verificationValueRemoved), evt.Data[event.DataKey.InvalidationReason])
	s.NotContains(evt.Data, event.DataKey.ActorSub, "an update without a caller names no actor")
	for key, value := range evt.Data {
		s.NotContains(fmt.Sprint(value), "a@example.com", "field %s carries the contact", key)
	}
}

func (s *ServiceTestSuite) TestMalformedMetadataRefusesTheContactWrite() {
	s.Run("self update", func() {
		store := newEntityStoreInterfaceMock(s.T())
		expectAttributeUpdate(store, verificationUser(`{"workEmail":"a@example.com"}`, `{"attributeVerification":[]}`))
		svc := s.verificationService(store, contactTypeService(s.T()))

		err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{"workEmail":"b@example.com"}`))

		s.Require().Error(err)
		store.AssertNotCalled(s.T(), "UpdateAttributes", mock.Anything, mock.Anything, mock.Anything)
	})
	s.Run("administrator update", func() {
		current := verificationUser(`{"workEmail":"a@example.com"}`,
			`{"attributeVerification":{"workEmail":"forged"}}`)
		store := newEntityStoreInterfaceMock(s.T())
		store.On("LockEntity", inTransaction(), current.ID).Return(current, nil).Once()
		svc := s.verificationService(store, contactTypeService(s.T()))
		next := current
		next.Attributes = json.RawMessage(`{"workEmail":"b@example.com"}`)

		_, err := svc.UpdateEntity(context.Background(), current.ID, &next)

		s.Require().Error(err)
		store.AssertNotCalled(s.T(), "UpdateEntity", mock.Anything, mock.Anything)
	})
}

func (s *ServiceTestSuite) TestInvalidationRemovesTheEntryOfAChangedValue() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"User@EXAMPLE.COM","mobile":"+94771234567"}`),
		SystemAttributes: json.RawMessage(`{"keep":true,"attributeVerification":` +
			`{"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`),
	}
	tests := []struct {
		name, attributes, nextType string
		verified, changed          bool
	}{
		{"canonical equality", `{"workEmail":" User@example.com ","mobile":"+94771234567"}`, "person", true, false},
		{"local part changed", `{"workEmail":"user@example.com","mobile":"+94771234567"}`, "person", false, true},
		{"email removed", `{"mobile":"+94771234567"}`, "person", false, true},
		{"user type changed", `{"workEmail":"User@EXAMPLE.COM","mobile":"+94771234567"}`, "employee", false, true},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			result, cleared, err := s.impl().invalidateVerification(current, test.nextType,
				declaredVerifiableAttributes, json.RawMessage(test.attributes), current.SystemAttributes)
			s.Require().NoError(err)
			s.Require().Equal(test.changed, len(cleared) > 0)
			entries, err := ReadRecordedAttributeVerifications(result)
			s.Require().NoError(err)
			_, verified := entries["workEmail"]
			s.Require().Equal(test.verified, verified)
			s.Require().Contains(string(result), `"keep":true`)
		})
	}
	// A -> B -> A does not bring the verification back.
	for _, value := range []string{`"other@example.com"`, `"User@EXAMPLE.COM"`} {
		next := json.RawMessage(`{"workEmail":` + value + `,"mobile":"+94771234567"}`)
		result, _, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes, next,
			current.SystemAttributes)
		s.Require().NoError(err)
		current.Attributes, current.SystemAttributes = next, result
	}
	entries, err := ReadRecordedAttributeVerifications(current.SystemAttributes)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *ServiceTestSuite) TestInvalidationComparesAnIneligibleAttributeExactly() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"formerEmail":"a@example.com"}`),
		SystemAttributes: json.RawMessage(
			`{"attributeVerification":{"formerEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`),
	}
	updated, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"formerEmail":"A@example.com"}`), current.SystemAttributes)
	s.Require().NoError(err)
	s.Require().Equal([]clearedVerification{
		{attribute: "formerEmail", reason: verificationValueChanged}}, cleared)
	entries, err := ReadRecordedAttributeVerifications(updated)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *ServiceTestSuite) TestInvalidationWritesNothingWithoutAVerification() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"a@example.com"}`)}
	updated, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":"new@example.com"}`), json.RawMessage(`{"keep":true}`))
	s.Require().NoError(err)
	s.Require().Empty(cleared)
	s.Require().JSONEq(`{"keep":true}`, string(updated))
}

func (s *ServiceTestSuite) TestInvalidationCanReplaceAnOldNonStringContact() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes:       json.RawMessage(`{"workEmail":42}`),
		SystemAttributes: json.RawMessage(verifiedWorkEmail),
	}
	updated, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":"new@example.com"}`), current.SystemAttributes)
	s.Require().NoError(err)
	s.Require().NotEmpty(cleared)
	entries, err := ReadRecordedAttributeVerifications(updated)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *ServiceTestSuite) TestInvalidationReportsAnEmptyValueAsRemoved() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)

	_, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":" "}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Equal([]clearedVerification{
		{attribute: "workEmail", reason: verificationValueRemoved}}, cleared)
}

func (s *ServiceTestSuite) TestInvalidationReportsANullValueAsRemoved() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)

	_, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":null}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Equal([]clearedVerification{
		{attribute: "workEmail", reason: verificationValueRemoved}}, cleared)
}

func (s *ServiceTestSuite) TestInvalidationLeavesANonUserAlone() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)
	current.Category = providers.EntityCategoryAgent

	updated, cleared, err := s.impl().invalidateVerification(current, "other", nil,
		json.RawMessage(`{"workEmail":"b@example.com"}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Empty(cleared)
	s.JSONEq(verifiedWorkEmail, string(updated))
}

func (s *ServiceTestSuite) TestUserTypeChangeClearsEveryVerification() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"a@example.com","mobile":"+94771234567"}`),
		SystemAttributes: json.RawMessage(`{"attributeVerification":{` +
			`"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"},"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)}

	updated, cleared, err := s.impl().invalidateVerification(current, "employee", declaredVerifiableAttributes,
		current.Attributes, current.SystemAttributes)

	s.Require().NoError(err)
	s.Equal([]clearedVerification{{attribute: "mobile", reason: verificationUserTypeChanged},
		{attribute: "workEmail", reason: verificationUserTypeChanged}}, cleared,
		"cleared in attribute order")
	entries, err := ReadRecordedAttributeVerifications(updated)
	s.Require().NoError(err)
	s.Empty(entries)
}

// Changed values are cleared in attribute order, so their events are published in a fixed order.
func (s *ServiceTestSuite) TestChangedValuesAreClearedInAttributeOrder() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"e":"1","b":"1","d":"1","a":"1","c":"1"}`),
		SystemAttributes: json.RawMessage(`{"attributeVerification":{"e":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
			`"b":{"verifiedAt":"2026-10-15T00:00:00Z"},"d":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
			`"a":{"verifiedAt":"2026-10-15T00:00:00Z"},"c":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)}

	_, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"e":"2","b":"2","d":"2","a":"2","c":"2"}`), current.SystemAttributes)

	s.Require().NoError(err)
	names := make([]string, 0, len(cleared))
	for _, c := range cleared {
		names = append(names, c.attribute)
	}
	s.Equal([]string{"a", "b", "c", "d", "e"}, names)
}

func (s *ServiceTestSuite) TestAnUnrelatedUpdateLeavesVerificationAlone() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes:       json.RawMessage(`{"workEmail":"a@example.com","nickname":"a"}`),
		SystemAttributes: json.RawMessage(verifiedWorkEmail)}

	updated, cleared, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":"a@example.com","nickname":"b"}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Empty(cleared)
	s.JSONEq(string(current.SystemAttributes), string(updated))
}

func (s *ServiceTestSuite) TestRemovedAndRestoredMobileIsUnverified() {
	current := verificationUser(`{"mobile":"+94771234567"}`,
		`{"attributeVerification":{"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)

	for _, next := range []string{`{}`, `{"mobile":"+94771234567"}`} {
		result, _, err := s.impl().invalidateVerification(current, current.Type, declaredVerifiableAttributes,
			json.RawMessage(next), current.SystemAttributes)
		s.Require().NoError(err)
		current.Attributes, current.SystemAttributes = json.RawMessage(next), result
	}

	entries, err := ReadRecordedAttributeVerifications(current.SystemAttributes)
	s.Require().NoError(err)
	s.Empty(entries)
}

func (s *ServiceTestSuite) TestInvalidationRefusesMalformedJSON() {
	stored := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)
	malformedMetadata := stored
	malformedMetadata.SystemAttributes = json.RawMessage(`{`)
	malformedAttributes := stored
	malformedAttributes.Attributes = json.RawMessage(`[`)
	for name, test := range map[string]struct {
		current providers.Entity
		next    string
	}{
		"stored metadata":   {malformedMetadata, `{"workEmail":"b@example.com"}`},
		"stored attributes": {malformedAttributes, `{"workEmail":"b@example.com"}`},
		"next attributes":   {stored, `[`},
	} {
		s.Run(name, func() {
			_, _, err := s.impl().invalidateVerification(test.current, test.current.Type, declaredVerifiableAttributes,
				json.RawMessage(test.next), test.current.SystemAttributes)

			s.Error(err)
		})
	}
}

// An invalidation names the attribute and its reason, never the value, and the caller as actor only
// outside a runtime context.
func (s *ServiceTestSuite) TestNewVerificationInvalidatedEvent() {
	caller := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("operator", "ou1", "", nil, nil))

	evt := s.impl().newVerificationInvalidatedEvent(caller, "user-a",
		clearedVerification{attribute: "email", reason: verificationValueChanged})

	s.Equal(string(event.EventTypeAttributeVerificationInvalidated), evt.Type)
	s.Equal(event.ComponentEntityService, evt.Component)
	s.Equal("user-a", evt.Data[event.DataKey.Subject])
	s.Equal(event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	s.Equal("operator", evt.Data[event.DataKey.ActorSub])
	s.Equal("email", evt.Data[event.DataKey.Attribute])
	s.Equal("value_changed", evt.Data[event.DataKey.InvalidationReason])

	runtime := s.impl().newVerificationInvalidatedEvent(security.WithRuntimeContext(caller), "user-a",
		clearedVerification{attribute: "email", reason: verificationValueChanged})
	s.NotContains(runtime.Data, event.DataKey.ActorSub)
}

func (s *ServiceTestSuite) TestWithVerifiedAttributesReplacesOnlyTheEntries() {
	entries := map[string]providers.VerifiedAttribute{"email": {VerifiedAt: "2026-10-16T12:00:00Z"}}
	for _, stored := range []string{``, `{"keep":true,"attributeVerification":{"mobile":{"verifiedAt":"x"}}}`} {
		written, err := s.impl().withVerifiedAttributes(json.RawMessage(stored), entries)
		s.Require().NoError(err)
		read, err := ReadRecordedAttributeVerifications(written)
		s.Require().NoError(err)
		s.Equal(entries, read)
		if stored != "" {
			s.JSONEq(`{"keep":true,"attributeVerification":{"email":{"verifiedAt":"2026-10-16T12:00:00Z"}}}`,
				string(written))
		}
	}
	_, err := s.impl().withVerifiedAttributes(json.RawMessage(`[`), entries)
	s.Error(err)
}

func (s *ServiceTestSuite) TestGetEntityListByType() {
	e := testEntity("type-e1")
	s.store.On("GetEntityListByType", mock.Anything, "agent", "tools", 10).
		Return([]providers.Entity{*e}, nil).Once()
	list, err := s.svc.GetEntityListByType(s.ctx, providers.EntityCategoryAgent, "tools", 10)
	s.NoError(err)
	s.Len(list, 1)

	s.store.On("GetEntityListByType", mock.Anything, "agent", "tools", 10).Return(nil, s.testErr).Once()
	_, err = s.svc.GetEntityListByType(s.ctx, providers.EntityCategoryAgent, "tools", 10)
	s.ErrorIs(err, s.testErr, "store error")

	svc, runtime := s.newServiceWithRuntimeMock()
	failing := testEntity("type-read-fails")
	s.store.On("GetEntityListByType", mock.Anything, "agent", "tools", 10).
		Return([]providers.Entity{*failing}, nil).Once()
	runtime.On("GetRuntimeData", mock.Anything, []string{failing.ID}).Return(nil, s.testErr)
	_, err = svc.GetEntityListByType(s.ctx, providers.EntityCategoryAgent, "tools", 10)
	s.ErrorIs(err, s.testErr, "runtime read error")
}

// flowRecorder records the system flows an entity service runs, in order with the store writes the tests
// add to it, failing the run of the handle named failing.
type flowRecorder struct {
	flows   *SystemFlowRunnerInterfaceMock
	order   []string
	runs    []*flowcommon.SystemFlowContext
	failing string
}

func (s *ServiceTestSuite) recordFlows(svc *entityService, failing string) *flowRecorder {
	r := &flowRecorder{flows: NewSystemFlowRunnerInterfaceMock(s.T()), failing: failing}
	r.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			r.runs = append(r.runs, sysCtx)
			r.order = append(r.order, sysCtx.Handle)
			if sysCtx.Handle == r.failing {
				return nil, &tidcommon.InternalServerError
			}
			return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": sysCtx.Handle}}, nil
		}).Maybe()
	svc.SetSystemFlowRunner(r.flows)
	return r
}

// Deleting an entity revokes what was issued to it before the row goes, and moves the cutoff past the delete
// with what the first flow shared.
func (s *ServiceTestSuite) TestDeleteEntity_RevokesAroundTheDelete() {
	svc := s.svc.(*entityService)
	r := s.recordFlows(svc, "")
	s.store.On("DeleteEntity", mock.Anything, "del-flows").
		Run(func(mock.Arguments) { r.order = append(r.order, "deleted") }).Return(nil).Once()

	s.Require().NoError(svc.DeleteEntity(s.ctx, "del-flows"))

	inputs := map[string]string{systemFlowInputEntity: "del-flows"}
	s.Equal([]*flowcommon.SystemFlowContext{
		{Handle: systemFlowEntityDeletePre, Inputs: inputs},
		{Handle: systemFlowRevocationPost, Inputs: inputs,
			RuntimeData: map[string]string{"plan": systemFlowEntityDeletePre}},
	}, r.runs)
	s.Equal([]string{systemFlowEntityDeletePre, "deleted", systemFlowRevocationPost}, r.order)
}

// A revocation that fails before the delete deletes nothing.
func (s *ServiceTestSuite) TestDeleteEntity_RevocationFailureDeletesNothing() {
	svc := s.svc.(*entityService)
	s.recordFlows(svc, systemFlowEntityDeletePre)

	s.ErrorIs(svc.DeleteEntity(s.ctx, "del-refused"), ErrRevocationFailed)
	s.store.AssertNotCalled(s.T(), "DeleteEntity", mock.Anything, mock.Anything)
}

// A revocation that fails after the delete is reported, though the entity is gone.
func (s *ServiceTestSuite) TestDeleteEntity_FailedPostFlowIsReported() {
	svc := s.svc.(*entityService)
	s.recordFlows(svc, systemFlowRevocationPost)
	s.store.On("DeleteEntity", mock.Anything, "del-post").Return(nil).Once()

	s.ErrorIs(svc.DeleteEntity(s.ctx, "del-post"), ErrRevocationIncomplete)
}

// A failed delete runs no flow after it.
func (s *ServiceTestSuite) TestDeleteEntity_FailedDeleteRunsNoPostFlow() {
	svc := s.svc.(*entityService)
	r := s.recordFlows(svc, "")
	s.store.On("DeleteEntity", mock.Anything, "del-store").Return(s.testErr).Once()

	s.ErrorIs(svc.DeleteEntity(s.ctx, "del-store"), s.testErr)
	s.Equal([]string{systemFlowEntityDeletePre}, r.order)
}

// credentialUpdateFixture is an agent whose type declares a password, holding the stored credentials given.
func (s *ServiceTestSuite) credentialUpdateFixture(id string, stored json.RawMessage) (*entityService, *flowRecorder,
	json.RawMessage) {
	svc, ets := s.newSvcWithEntityType()
	e := testEntity(id)
	e.Category = providers.EntityCategoryAgent
	e.Type = testCredentialEntityType
	creds := json.RawMessage(`{"password":"long-enough"}`)
	declaresCredentials(ets, entitytype.TypeCategoryAgent, testCredentialEntityType, "password")
	ets.On("ValidateEntityValues", mock.Anything, entitytype.TypeCategoryAgent, testCredentialEntityType, creds).
		Return(true, (*tidcommon.ServiceError)(nil))
	s.store.On("GetEntity", mock.Anything, e.ID).Return(*e, nil)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e, SchemaCredentials: stored}, nil)
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Maybe()
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Return(nil).Maybe()
	r := s.recordFlows(svc, "")
	s.store.On("UpdateCredentials", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Run(func(mock.Arguments) { r.order = append(r.order, "written") }).Return(nil).Maybe()
	return svc, r, creds
}

// Replacing a credential the entity holds revokes what was issued to it before the write, and once it is
// written ends what was issued while it ran.
func (s *ServiceTestSuite) TestUpdateCredentials_ReplacingAHeldCredentialRevokesAroundTheWrite() {
	svc, r, creds := s.credentialUpdateFixture("e-replace", json.RawMessage(`{"password":[{"value":"h1"}]}`))

	s.Require().NoError(svc.UpdateCredentials(s.ctx, "e-replace", creds))

	inputs := map[string]string{systemFlowInputEntity: "e-replace"}
	s.Equal([]*flowcommon.SystemFlowContext{
		{Handle: systemFlowCredentialUpdatePre, Inputs: inputs},
		{Handle: systemFlowCredentialUpdatePost, Inputs: inputs,
			RuntimeData: map[string]string{"plan": systemFlowCredentialUpdatePre}},
	}, r.runs)
	s.Equal([]string{systemFlowCredentialUpdatePre, "written", systemFlowCredentialUpdatePost}, r.order)
}

// A credential set for the first time replaces nothing, so nothing is revoked.
func (s *ServiceTestSuite) TestUpdateCredentials_FirstCredentialRevokesNothing() {
	svc, r, creds := s.credentialUpdateFixture("e-first", nil)

	s.Require().NoError(svc.UpdateCredentials(s.ctx, "e-first", creds))

	s.Equal([]string{"written"}, r.order)
}

// A revocation that fails before the write writes nothing; one that fails after it is reported as
// incomplete, since the credential is stored by then.
func (s *ServiceTestSuite) TestUpdateCredentials_RevocationFailures() {
	svc, r, creds := s.credentialUpdateFixture("e-fail", json.RawMessage(`{"password":[{"value":"h1"}]}`))

	r.failing = systemFlowCredentialUpdatePre
	s.ErrorIs(svc.UpdateCredentials(s.ctx, "e-fail", creds), ErrRevocationFailed)
	s.Equal([]string{systemFlowCredentialUpdatePre}, r.order)

	r.order, r.failing = nil, systemFlowCredentialUpdatePost
	s.ErrorIs(svc.UpdateCredentials(s.ctx, "e-fail", creds), ErrRevocationIncomplete)
	s.Equal([]string{systemFlowCredentialUpdatePre, "written", systemFlowCredentialUpdatePost}, r.order)
}

// regenerationFixture is an entity service with a holder for agents, whose system flows and secret write
// are recorded in order.
type regenerationFixture struct {
	svc    *entityService
	holder *ClientSecretHolderInterfaceMock
	flows  *SystemFlowRunnerInterfaceMock
	order  []string
	runs   []*flowcommon.SystemFlowContext
	stored json.RawMessage
}

const testSecretEntityID = "agent-1"

// secretRotationInputs is what the flows around the agent's secret rotation are given.
var secretRotationInputs = map[string]string{systemFlowInputEntity: testSecretEntityID}

func (s *ServiceTestSuite) newRegenerationFixture() *regenerationFixture {
	f := &regenerationFixture{svc: s.svc.(*entityService)}
	f.holder = NewClientSecretHolderInterfaceMock(s.T())
	f.svc.RegisterClientSecretHolder(providers.EntityCategoryAgent, f.holder)
	f.flows = NewSystemFlowRunnerInterfaceMock(s.T())
	f.svc.SetSystemFlowRunner(f.flows)

	e := testEntity(testSecretEntityID)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil).Maybe()
	s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Run(func(args mock.Arguments) {
			f.order = append(f.order, "secret")
			f.stored, _ = args.Get(2).(json.RawMessage)
		}).Return(nil).Maybe()
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Maybe()
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Return(nil).Maybe()
	return f
}

// allows has the holder allow the regeneration.
func (f *regenerationFixture) allows() {
	f.holder.EXPECT().ValidateClientSecretRegeneration(mock.Anything, testSecretEntityID).Return(nil)
}

// recordsFlows records every system flow run, failing the run of the handle named failing with failure.
func (f *regenerationFixture) recordsFlows(failing string, failure *tidcommon.ServiceError) {
	f.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			f.runs = append(f.runs, sysCtx)
			f.order = append(f.order, sysCtx.Handle)
			if sysCtx.Handle == failing {
				return nil, failure
			}
			return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": sysCtx.Handle}}, nil
		})
}

// A regeneration revokes what the old secret issued before the new one is written, and once it is written
// moves the cutoff past the write, with what the first flow shared.
func (s *ServiceTestSuite) TestRegenerateClientSecret_RevokesAroundTheWrite() {
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows("", nil)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.NotEmpty(regenerated.ClientSecret)
	s.True(regenerated.Revoked)
	s.Nil(regenerated.RevocationError)
	s.Equal([]*flowcommon.SystemFlowContext{
		{Handle: systemFlowSecretRotatePre, Inputs: secretRotationInputs},
		{Handle: systemFlowRevocationPost, Inputs: secretRotationInputs,
			RuntimeData: map[string]string{"plan": systemFlowSecretRotatePre}},
	}, f.runs)
	s.Equal([]string{systemFlowSecretRotatePre, "secret", systemFlowRevocationPost}, f.order,
		"the secret was written before the revocation")
	s.NotContains(string(f.stored), regenerated.ClientSecret, "the secret is stored hashed")
}

// A refusal of the service owning the entity runs no flow and writes nothing.
func (s *ServiceTestSuite) TestRegenerateClientSecret_HolderRefusalWritesNothing() {
	f := s.newRegenerationFixture()
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AGT-1001"}
	f.holder.EXPECT().ValidateClientSecretRegeneration(mock.Anything, testSecretEntityID).Return(refusal)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(refusal, svcErr)
	s.Nil(regenerated)
	s.Empty(f.order)
}

// A refusal of the flow run before the write writes nothing.
func (s *ServiceTestSuite) TestRegenerateClientSecret_PreFlowRefusalWritesNothing() {
	f := s.newRegenerationFixture()
	f.allows()
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "FET-1088"}
	f.recordsFlows(systemFlowSecretRotatePre, refusal)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(refusal, svcErr)
	s.Nil(regenerated)
	s.Equal([]string{systemFlowSecretRotatePre}, f.order, "something was written after the refusal")
}

// A revocation that fails once the new secret is written still returns the secret, which is stored by then
// and can never be read again, with the failure for the caller to report.
func (s *ServiceTestSuite) TestRegenerateClientSecret_FailedPostFlowStillReturnsTheSecret() {
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows(systemFlowRevocationPost, &tidcommon.InternalServerError)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.NotEmpty(regenerated.ClientSecret)
	s.Equal(&tidcommon.InternalServerError, regenerated.RevocationError)
	s.False(regenerated.Revoked)
	s.Equal([]string{systemFlowSecretRotatePre, "secret", systemFlowRevocationPost}, f.order)
}

// A secret that cannot be stored is reported as a server error, and no flow runs after the failed write.
func (s *ServiceTestSuite) TestRegenerateClientSecret_FailedWriteRunsNoPostFlow() {
	s.store.On("UpdateSystemCredentials", mock.Anything, testSecretEntityID, mock.IsType(json.RawMessage(nil))).
		Return(s.testErr).Once()
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows("", nil)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Nil(regenerated)
	s.Equal([]string{systemFlowSecretRotatePre}, f.order, "a flow ran after the failed write")
}

// With system flows off, as on a control plane, the secret is still regenerated.
func (s *ServiceTestSuite) TestRegenerateClientSecret_WithSystemFlowsOff() {
	f := s.newRegenerationFixture()
	f.allows()
	f.svc.systemFlows = nil

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.NotEmpty(regenerated.ClientSecret)
	s.False(regenerated.Revoked, "nothing was revoked with system flows off")
	s.Equal([]string{"secret"}, f.order)
}

// A category no service decides for is a wiring fault: nothing runs and nothing is written.
func (s *ServiceTestSuite) TestRegenerateClientSecret_CategoryWithoutAHolder() {
	f := s.newRegenerationFixture()

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryUser, testSecretEntityID)

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Nil(regenerated)
	s.Empty(f.order)
}

// A rotation whose write fails runs no flow after it, and reports the write's failure.
func (s *ServiceTestSuite) TestRotateClientSecret_FailedWriteRunsNoPostFlow() {
	f := s.newRegenerationFixture()
	f.recordsFlows("", nil)

	svcErr, revocationErr := f.svc.RotateClientSecret(s.ctx, testSecretEntityID, func() *tidcommon.ServiceError {
		f.order = append(f.order, "write")
		return &tidcommon.InternalServerError
	})

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Nil(revocationErr)
	s.Equal([]string{systemFlowSecretRotatePre, "write"}, f.order)
}

// With system flows off, as on a control plane, only the change runs.
func (s *ServiceTestSuite) TestRevokeAround_WithSystemFlowsOff() {
	svc := s.svc.(*entityService)
	ran := false

	s.NoError(svc.revokeAround(s.ctx, "e-off", systemFlowEntityDeletePre, systemFlowRevocationPost,
		func() error { ran = true; return nil }))
	s.True(ran)
}
