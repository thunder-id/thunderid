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
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/hashmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
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
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(), nil)
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
			svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transactioner, nil)

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
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transactioner, nil)
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
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transactioner, nil)
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
	svc := newEntityService(s.store, runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(), nil)
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
	svc := newEntityService(s.store, s.runtime, s.hashService, ets, nil, transaction.NewNoOpTransactioner(), nil)
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
		sharingService)
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
		transaction.NewNoOpTransactioner(), nil)
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
	s.svc = newEntityService(s.store, s.runtime, s.hashService, nil, nil, transaction.NewNoOpTransactioner(), nil)
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
		transaction.NewNoOpTransactioner(), nil).(*entityService)
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
