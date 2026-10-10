// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type DBStoreTestSuite struct {
	suite.Suite
	provider *providermock.DBProviderInterfaceMock
	client   *providermock.DBClientInterfaceMock
	store    *entityDBStore
	ctx      context.Context
	testErr  error
}

func TestDBStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DBStoreTestSuite))
}

func (s *DBStoreTestSuite) SetupTest() {
	s.provider = providermock.NewDBProviderInterfaceMock(s.T())
	s.client = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &entityDBStore{
		indexedAttributes: map[string]bool{},
		dbProvider:        s.provider,
		logger:            log.GetLogger(),
	}
	s.ctx = context.Background()
	s.testErr = errors.New("db error")
}

func (s *DBStoreTestSuite) expectClient() {
	s.provider.On("GetEntityDBClient").Return(s.client, nil).Once()
}

func (s *DBStoreTestSuite) expectClientError() {
	s.provider.On("GetEntityDBClient").Return(nil, s.testErr).Once()
}

// onExecAny registers an ExecuteContext expectation that matches any args (up to 14).
// ExecuteContext is variadic; using 14 Anything matchers covers the widest call (CreateEntity).
// Extra Anything matchers silently pass when fewer actual args are provided.
func (s *DBStoreTestSuite) onExecAny(ret int64, err error) *mock.Call {
	return s.client.On("ExecuteContext",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	).Return(ret, err)
}

// onQueryAny registers a QueryContext expectation that matches any args (up to 8).
// QueryContext is variadic; 8 Anything matchers covers the widest call in this package.
func (s *DBStoreTestSuite) onQueryAny(ret []map[string]interface{}, err error) *mock.Call {
	return s.client.On("QueryContext",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(ret, err)
}

func dbEntityRow() map[string]interface{} {
	return map[string]interface{}{
		"id":                "e1",
		"ou_id":             "ou-1",
		"category":          "user",
		"type":              "employee",
		"state":             "ACTIVE",
		"attributes":        `{"email":"a@b.com"}`,
		"system_attributes": nil,
	}
}

func dbEntityRowWithCreds() map[string]interface{} {
	row := dbEntityRow()
	row["credentials"] = `{"password":"hashed"}`
	row["system_credentials"] = `{"token":"tok"}`
	return row
}

func (s *DBStoreTestSuite) TestGetEntity_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntity_QueryError() {
	s.expectClient()
	s.onQueryAny(nil, s.testErr)
	_, err := s.store.GetEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntity_NotFound() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil)
	_, err := s.store.GetEntity(s.ctx, "e1")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestGetEntity_MultipleResults() {
	s.expectClient()
	rows := []map[string]interface{}{dbEntityRow(), dbEntityRow()}
	s.onQueryAny(rows, nil)
	_, err := s.store.GetEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntity_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil)
	e, err := s.store.GetEntity(s.ctx, "e1")
	s.NoError(err)
	s.Equal("e1", e.ID)
}

func (s *DBStoreTestSuite) TestGetEntityWithCredentials_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntityWithCredentials(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityWithCredentials_NotFound() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil)
	_, err := s.store.GetEntityWithCredentials(s.ctx, "e1")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestGetEntityWithCredentials_MultipleResults() {
	s.expectClient()
	rows := []map[string]interface{}{dbEntityRowWithCreds(), dbEntityRowWithCreds()}
	s.onQueryAny(rows, nil)
	_, err := s.store.GetEntityWithCredentials(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityWithCredentials_BadRow() {
	s.expectClient()
	bad := map[string]interface{}{"id": 123} // wrong type for id
	s.onQueryAny([]map[string]interface{}{bad}, nil)
	_, err := s.store.GetEntityWithCredentials(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityWithCredentials_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRowWithCreds()}, nil)
	result, err := s.store.GetEntityWithCredentials(s.ctx, "e1")
	s.NoError(err)
	s.Equal("e1", result.Entity.ID)
	s.NotNil(result.SchemaCredentials)
	s.NotNil(result.SystemCredentials)
}

func (s *DBStoreTestSuite) TestCreateEntity_ProviderError() {
	s.expectClientError()
	e := providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestCreateEntity_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	e := providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestCreateEntity_Success_NoIdentifiers() {
	s.expectClient()
	s.onExecAny(1, nil)
	// SyncAttributeIdentifiers: no indexed attributes → no DB call for sync
	e := providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestCreateEntity_WithSystemAttrsAndCreds() {
	s.expectClient()
	s.onExecAny(1, nil)
	// SyncAttributeIdentifiers: no indexed attributes → no DB call for sync
	e := providers.Entity{
		ID:               "e1",
		Attributes:       json.RawMessage(`{}`),
		SystemAttributes: json.RawMessage(`{"key":"val"}`),
	}
	err := s.store.CreateEntity(s.ctx, e, json.RawMessage(`{"p":"h"}`), json.RawMessage(`{"t":"t"}`))
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_ProviderError() {
	s.expectClientError()
	e := &providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	e := &providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	e := &providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestUpdateEntity_ReloadError() {
	s.expectClient()
	s.onExecAny(1, nil).Once()          // update entity succeeds
	s.expectClient()                    // for reload (GetEntity)
	s.onQueryAny(nil, s.testErr).Once() // reload query fails
	e := &providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_DeleteIdentifiersError() {
	s.expectClient()
	s.onExecAny(1, nil).Once()                                        // update entity succeeds
	s.expectClient()                                                  // for reload (GetEntity)
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil).Once() // reload succeeds
	s.onExecAny(0, s.testErr).Once()                                  // delete identifiers fails
	e := &providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_Success() {
	// SyncAttributeIdentifiers with no indexed attrs returns nil without a DB call.
	s.expectClient()
	s.onExecAny(1, nil).Once()                                        // update entity
	s.expectClient()                                                  // for reload (GetEntity)
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil).Once() // reload succeeds
	s.onExecAny(1, nil).Once()                                        // delete identifiers
	e := &providers.Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestUpdateSystemAttributes_ProviderError() {
	s.expectClientError()
	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{}`))
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateSystemAttributes_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{}`))
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateSystemAttributes_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{}`))
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestUpdateSystemAttributes_Success() {
	s.expectClient()
	s.onExecAny(1, nil).Once()                                        // update system attributes
	s.expectClient()                                                  // for reload (GetEntity)
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil).Once() // reload succeeds
	s.onExecAny(1, nil).Once()                                        // delete identifiers
	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{}`))
	s.NoError(err)
}

// identifierWrites returns the query IDs of all ExecuteContext calls and the string args of the last one.
func (s *DBStoreTestSuite) identifierWrites() ([]string, []string) {
	queryIDs := make([]string, 0, len(s.client.Calls))
	var lastArgs []string
	for _, call := range s.client.Calls {
		if call.Method != "ExecuteContext" {
			continue
		}
		queryIDs = append(queryIDs, call.Arguments.Get(1).(dbmodel.DBQuery).ID)
		lastArgs = nil
		for _, arg := range call.Arguments[2:] {
			if str, ok := arg.(string); ok {
				lastArgs = append(lastArgs, str)
			}
		}
	}
	return queryIDs, lastArgs
}

// expectIdentifierResync sets up an entity update whose reload returns the given stored attributes.
func (s *DBStoreTestSuite) expectIdentifierResync(attributes, systemAttributes string) {
	s.store.indexedAttributes = map[string]bool{"email": true}
	row := dbEntityRow()
	row["attributes"] = attributes
	row["system_attributes"] = systemAttributes
	s.provider.On("GetEntityDBClient").Return(s.client, nil)
	s.onQueryAny([]map[string]interface{}{row}, nil).Once()
	s.onExecAny(1, nil)
}

// The linked-ID unique index refusing an insert means another entity holds the subject, so it is
// reported as a conflict on both databases. Any other insert failure stays a server error.
func (s *DBStoreTestSuite) TestSyncAttributeIdentifiers_LinkedIDConflict() {
	sysAttrs := json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	for name, tc := range map[string]struct {
		execErr  error
		conflict bool
	}{
		"postgres":           {execErr: &pq.Error{Code: "23505", Constraint: linkedIDIndexName}, conflict: true},
		"other unique index": {execErr: &pq.Error{Code: "23505", Constraint: "entity_identifier_pkey"}},
		"other failure":      {execErr: s.testErr},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectClient()
			s.onExecAny(0, tc.execErr)

			err := s.store.syncAttributeIdentifiers(s.ctx, "e1", nil, sysAttrs, map[string]bool{})

			s.Require().Error(err)
			s.Equal(tc.conflict, errors.Is(err, ErrLinkedAccountConflict))
		})
	}
}

// Against the shipped SQLite schema, a second entity linking a held subject is refused as a conflict,
// while the same subject at another deployment links freely.
func (s *DBStoreTestSuite) TestSyncAttributeIdentifiers_LinkedIDUniqueOnSQLite() {
	schema, err := os.ReadFile("../../dbscripts/entitydb/sqlite.sql")
	s.Require().NoError(err)
	db, err := sql.Open("sqlite", ":memory:")
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.Require().NoError(db.Close()) })
	_, err = db.Exec(string(schema))
	s.Require().NoError(err)

	s.provider.On("GetEntityDBClient").Return(s.client, nil)
	// One identifier row binds six values.
	s.client.EXPECT().ExecuteContext(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, q dbmodel.DBQuery, args ...interface{}) (int64, error) {
			res, execErr := db.Exec(q.GetQuery("sqlite"), args...)
			if execErr != nil {
				return 0, execErr
			}
			return res.RowsAffected()
		})
	link := json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`)
	otherCtx := deployment.WithID(s.ctx, "other-deployment")

	s.Require().NoError(s.store.syncAttributeIdentifiers(s.ctx, "e1", nil, link, map[string]bool{}))
	s.ErrorIs(s.store.syncAttributeIdentifiers(s.ctx, "e2", nil, link, map[string]bool{}),
		ErrLinkedAccountConflict)
	s.NoError(s.store.syncAttributeIdentifiers(otherCtx, "e3", nil, link, map[string]bool{}))
}

func (s *DBStoreTestSuite) TestUpdateAttributes_ResyncAppliesSystemPrecedence() {
	s.expectIdentifierResync(`{"email":"schema@b.com"}`, `{"email":"sys@b.com"}`)

	err := s.store.UpdateAttributes(s.ctx, "e1", json.RawMessage(`{"email":"schema@b.com"}`))
	s.NoError(err)

	queryIDs, inserted := s.identifierWrites()
	s.Equal([]string{QueryUpdateAttributes.ID, QueryDeleteIdentifiersByEntity.ID,
		QueryBatchInsertIdentifiers.ID}, queryIDs)
	s.Contains(inserted, "sys@b.com")
	s.NotContains(inserted, "schema@b.com")
}

func (s *DBStoreTestSuite) TestUpdateSystemAttributes_ResyncDropsOverriddenSchemaValue() {
	s.expectIdentifierResync(`{"email":"schema@b.com"}`, `{"email":"sys@b.com"}`)

	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{"email":"sys@b.com"}`))
	s.NoError(err)

	queryIDs, inserted := s.identifierWrites()
	s.Equal([]string{QueryUpdateSystemAttributes.ID, QueryDeleteIdentifiersByEntity.ID,
		QueryBatchInsertIdentifiers.ID}, queryIDs)
	s.Contains(inserted, "sys@b.com")
	s.NotContains(inserted, "schema@b.com")
}

func (s *DBStoreTestSuite) TestUpdateSystemAttributes_ResyncRestoresSchemaValue() {
	s.expectIdentifierResync(`{"email":"schema@b.com"}`, `{}`)

	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{}`))
	s.NoError(err)

	_, inserted := s.identifierWrites()
	s.Contains(inserted, "schema@b.com")
}

func (s *DBStoreTestSuite) TestLockEntity_ProviderError() {
	s.expectClientError()
	_, err := s.store.LockEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestLockEntity_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	_, err := s.store.LockEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestLockEntity_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	_, err := s.store.LockEntity(s.ctx, "e1")
	s.ErrorIs(err, ErrEntityNotFound)
}

// The entity is read from the database under the lock, so the caller modifies the committed value.
func (s *DBStoreTestSuite) TestLockEntity_ReadsEntityUnderLock() {
	s.expectClient()
	s.onExecAny(1, nil)
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil)
	e, err := s.store.LockEntity(s.ctx, "e1")
	s.Require().NoError(err)
	s.Equal("e1", e.ID)
}

func (s *DBStoreTestSuite) TestUpdateCredentials_ProviderError() {
	s.expectClientError()
	err := s.store.UpdateCredentials(s.ctx, "e1", json.RawMessage(`{}`))
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateCredentials_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	err := s.store.UpdateCredentials(s.ctx, "e1", json.RawMessage(`{}`))
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestUpdateCredentials_Success() {
	s.expectClient()
	s.onExecAny(1, nil)
	err := s.store.UpdateCredentials(s.ctx, "e1", json.RawMessage(`{}`))
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestUpdateSystemCredentials_ProviderError() {
	s.expectClientError()
	err := s.store.UpdateSystemCredentials(s.ctx, "e1", json.RawMessage(`{}`))
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateSystemCredentials_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	err := s.store.UpdateSystemCredentials(s.ctx, "e1", json.RawMessage(`{}`))
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestUpdateSystemCredentials_Success() {
	s.expectClient()
	s.onExecAny(1, nil)
	err := s.store.UpdateSystemCredentials(s.ctx, "e1", json.RawMessage(`{}`))
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestDeleteEntity_ProviderError() {
	s.expectClientError()
	err := s.store.DeleteEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestDeleteEntity_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	err := s.store.DeleteEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestDeleteEntity_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	err := s.store.DeleteEntity(s.ctx, "e1")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestDeleteEntity_Success() {
	s.expectClient()
	s.onExecAny(1, nil)
	err := s.store.DeleteEntity(s.ctx, "e1")
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_ProviderError() {
	s.expectClientError()
	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.Error(err)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_FastPath_SingleResult() {
	id := "e1"
	s.expectClient()
	// Fast path query (identifiers table)
	s.onQueryAny([]map[string]interface{}{{"id": id}}, nil).Once()
	got, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.NoError(err)
	s.Equal(id, *got)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_FastPath_Empty_FallbackToJSON() {
	s.expectClient()
	// Fast path returns no results
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	// Fallback JSON query
	s.onQueryAny([]map[string]interface{}{{"id": "e1"}}, nil).Once()
	got, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.NoError(err)
	s.Equal("e1", *got)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_NotFound() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_MultipleResults() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	rows := []map[string]interface{}{{"id": "e1"}, {"id": "e2"}}
	s.onQueryAny(rows, nil).Once()
	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.Error(err)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_FastPath_MultipleEntitiesIndexed_Ambiguous() {
	s.store.indexedAttributes = map[string]bool{"email": true}
	s.expectClient()
	// Two entities hold the item in the identifier table. No fallback query is expected, since the
	// JSON fallback cannot match an array item and would hide the ambiguity.
	rows := []map[string]interface{}{{"id": "e1"}, {"id": "e2"}}
	s.onQueryAny(rows, nil).Once()
	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.ErrorIs(err, ErrAmbiguousEntity)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_FastPath_MultipleEntitiesNonIndexed_FallsBack() {
	s.store.indexedAttributes = map[string]bool{"email": true}
	s.expectClient()
	// A filter on a non-indexed name may match leftover identifier rows, so the JSON query decides.
	rows := []map[string]interface{}{{"id": "e1"}, {"id": "e2"}}
	s.onQueryAny(rows, nil).Once()
	s.onQueryAny([]map[string]interface{}{{"id": "e1"}}, nil).Once()
	got, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com", "username": "u1"})
	s.NoError(err)
	s.Equal("e1", *got)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_BadIDType() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	s.onQueryAny([]map[string]interface{}{{"id": 123}}, nil).Once()
	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "a@b.com"})
	s.Error(err)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_HybridQuery_IndexedAndNonIndexed() {
	s.store.indexedAttributes = map[string]bool{"email": true}
	s.expectClient()
	// fast path via identifier table fails (empty)
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	// hybrid query
	s.onQueryAny([]map[string]interface{}{{"id": "e1"}}, nil).Once()
	filters := map[string]interface{}{"email": "a@b.com", "username": "u1"}
	got, err := s.store.IdentifyEntity(s.ctx, filters)
	s.NoError(err)
	s.Equal("e1", *got)
}

func (s *DBStoreTestSuite) TestIdentifyEntity_FastPath_Empty_FallbackSearchesBothColumns() {
	s.expectClient()
	// Fast path (ENTITY_IDENTIFIER table) returns nothing.
	s.onQueryAny([]map[string]interface{}{}, nil).Once()
	// Fallback COALESCE query (searches both ATTRIBUTES and SYSTEM_ATTRIBUTES) finds the entity.
	s.onQueryAny([]map[string]interface{}{{"id": "app-entity-1"}}, nil).Once()

	got, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"clientId": "my-client"})
	s.NoError(err)
	s.Equal("app-entity-1", *got)
}

func (s *DBStoreTestSuite) TestGetEntityListCount_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityListCount_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"total": int64(5)}}, nil)
	count, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.NoError(err)
	s.Equal(5, count)
}

func (s *DBStoreTestSuite) TestGetEntityListCount_BadTotalType() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"total": "not-an-int"}}, nil)
	_, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityList_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntityList(s.ctx, "user", 10, 0, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityList_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil)
	list, err := s.store.GetEntityList(s.ctx, "user", 10, 0, nil)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *DBStoreTestSuite) TestGetEntityListCountByOUIDs_EmptyOUIDs() {
	count, err := s.store.GetEntityListCountByOUIDs(s.ctx, "user", []string{}, nil)
	s.NoError(err)
	s.Equal(0, count)
}

func (s *DBStoreTestSuite) TestGetEntityListCountByOUIDs_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntityListCountByOUIDs(s.ctx, "user", []string{"ou1"}, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityListCountByOUIDs_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"total": int64(2)}}, nil)
	count, err := s.store.GetEntityListCountByOUIDs(s.ctx, "user", []string{"ou1"}, nil)
	s.NoError(err)
	s.Equal(2, count)
}

func (s *DBStoreTestSuite) TestGetEntityListByOUIDs_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntityListByOUIDs(s.ctx, "user", []string{"ou1"}, 10, 0, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityListByOUIDs_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil)
	list, err := s.store.GetEntityListByOUIDs(s.ctx, "user", []string{"ou1"}, 10, 0, nil)
	s.NoError(err)
	s.Len(list, 1)
}

func (s *DBStoreTestSuite) TestValidateEntityIDs_Empty() {
	invalid, err := s.store.ValidateEntityIDs(s.ctx, []string{})
	s.NoError(err)
	s.Empty(invalid)
}

func (s *DBStoreTestSuite) TestValidateEntityIDs_ProviderError() {
	s.expectClientError()
	_, err := s.store.ValidateEntityIDs(s.ctx, []string{"e1"})
	s.Error(err)
}

func (s *DBStoreTestSuite) TestValidateEntityIDs_SomeInvalid() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"id": "e1"}}, nil)
	invalid, err := s.store.ValidateEntityIDs(s.ctx, []string{"e1", "missing"})
	s.NoError(err)
	s.Equal([]string{"missing"}, invalid)
}

func (s *DBStoreTestSuite) TestGetEntitiesByIDs_Empty() {
	list, err := s.store.GetEntitiesByIDs(s.ctx, []string{})
	s.NoError(err)
	s.Empty(list)
}

func (s *DBStoreTestSuite) TestGetEntitiesByIDs_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntitiesByIDs(s.ctx, []string{"e1"})
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntitiesByIDs_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil)
	list, err := s.store.GetEntitiesByIDs(s.ctx, []string{"e1"})
	s.NoError(err)
	s.Len(list, 1)
}

func (s *DBStoreTestSuite) TestValidateEntityIDsInOUs_EmptyEntityIDs() {
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{}, []string{"ou1"})
	s.NoError(err)
	s.Empty(out)
}

func (s *DBStoreTestSuite) TestValidateEntityIDsInOUs_EmptyOUIDs() {
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"e1", "e2"}, []string{})
	s.NoError(err)
	s.Equal([]string{"e1", "e2"}, out)
}

func (s *DBStoreTestSuite) TestValidateEntityIDsInOUs_ProviderError() {
	s.expectClientError()
	_, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"e1"}, []string{"ou1"})
	s.Error(err)
}

func (s *DBStoreTestSuite) TestValidateEntityIDsInOUs_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"id": "e1"}}, nil)
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"e1", "e2"}, []string{"ou1"})
	s.NoError(err)
	s.Equal([]string{"e2"}, out)
}

func (s *DBStoreTestSuite) TestGetGroupCountForEntity_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetGroupCountForEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetGroupCountForEntity_EmptyResults() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil)
	count, err := s.store.GetGroupCountForEntity(s.ctx, "e1")
	s.NoError(err)
	s.Equal(0, count)
}

func (s *DBStoreTestSuite) TestGetGroupCountForEntity_BadType() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"total": "wrong"}}, nil)
	_, err := s.store.GetGroupCountForEntity(s.ctx, "e1")
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetGroupCountForEntity_Success() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{{"total": int64(3)}}, nil)
	count, err := s.store.GetGroupCountForEntity(s.ctx, "e1")
	s.NoError(err)
	s.Equal(3, count)
}

func (s *DBStoreTestSuite) TestGetEntityGroups_ProviderError() {
	s.expectClientError()
	_, err := s.store.GetEntityGroups(s.ctx, "e1", 10, 0)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestGetEntityGroups_Success() {
	s.expectClient()
	groupRow := map[string]interface{}{"id": "g1", "name": "GroupA", "ou_id": "ou1"}
	s.onQueryAny([]map[string]interface{}{groupRow}, nil)
	groups, err := s.store.GetEntityGroups(s.ctx, "e1", 10, 0)
	s.NoError(err)
	s.Len(groups, 1)
	s.Equal("g1", groups[0].ID)
}

func (s *DBStoreTestSuite) TestGetEntityGroups_BadGroupRow() {
	s.expectClient()
	bad := map[string]interface{}{"id": 123} // wrong type
	s.onQueryAny([]map[string]interface{}{bad}, nil)
	_, err := s.store.GetEntityGroups(s.ctx, "e1", 10, 0)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestIsEntityDeclarative_AlwaysFalse() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil)
	ok, err := s.store.IsEntityDeclarative(s.ctx, "e1")
	s.NoError(err)
	s.False(ok)
}

func (s *DBStoreTestSuite) TestIsEntityDeclarative_EntityError() {
	s.expectClient()
	s.onQueryAny([]map[string]interface{}{}, nil)
	_, err := s.store.IsEntityDeclarative(s.ctx, "missing")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestGetIndexedAttributes() {
	s.store.indexedAttributes = map[string]bool{"email": true}
	s.Equal(map[string]bool{"email": true}, s.store.GetIndexedAttributes())
}

func (s *DBStoreTestSuite) TestExecuteCountQuery_QueryError() {
	s.onQueryAny(nil, s.testErr)
	_, err := executeCountQuery(s.client, s.ctx, dbmodel.DBQuery{ID: "test", Query: "SELECT 1"}, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestExecuteCountQuery_EmptyResults() {
	s.onQueryAny([]map[string]interface{}{}, nil)
	count, err := executeCountQuery(s.client, s.ctx, dbmodel.DBQuery{ID: "test", Query: "SELECT 1"}, nil)
	s.NoError(err)
	s.Equal(0, count)
}

func (s *DBStoreTestSuite) TestExecuteCountQuery_BadType() {
	s.onQueryAny([]map[string]interface{}{{"total": "bad"}}, nil)
	_, err := executeCountQuery(s.client, s.ctx, dbmodel.DBQuery{ID: "test", Query: "SELECT 1"}, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestExecuteCountQuery_Success() {
	s.onQueryAny([]map[string]interface{}{{"total": int64(7)}}, nil)
	count, err := executeCountQuery(s.client, s.ctx, dbmodel.DBQuery{ID: "test", Query: "SELECT 1"}, nil)
	s.NoError(err)
	s.Equal(7, count)
}

// A schema attribute that happens to be named like a link indexes under the same identifier name as
// a recorded link, but it is user-owned. Resolving it would sign the linked account in as that
// user without the linking verification step, so only server-owned rows may resolve. The rows are
// written through the real identifier write path and read back with the real query on SQLite.
func (s *DBStoreTestSuite) TestResolveLinkedAccount_IgnoresAttributeSourcedRows() {
	db, err := sql.Open("sqlite", ":memory:")
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.Require().NoError(db.Close()) })
	_, err = db.Exec(`CREATE TABLE "ENTITY_IDENTIFIER" (
		DEPLOYMENT_ID TEXT NOT NULL, ENTITY_ID TEXT NOT NULL, NAME TEXT NOT NULL, VALUE TEXT NOT NULL,
		SOURCE TEXT NOT NULL, CREATED_AT TEXT NOT NULL,
		PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID, NAME, VALUE))`)
	s.Require().NoError(err)

	linkName := linkedIdentifierName("idp-a")
	indexed := map[string]bool{linkName: true}
	insert := func(entityID string, attrs, sysAttrs json.RawMessage) {
		q, args, qErr := prepareIdentifierQuery(entityID, attrs, sysAttrs, indexed, deployment.Resolve(s.ctx))
		s.Require().NoError(qErr)
		_, qErr = db.Exec(q.GetQuery("sqlite"), args...)
		s.Require().NoError(qErr)
	}

	s.provider.On("GetEntityDBClient").Return(s.client, nil)
	s.client.EXPECT().QueryContext(mock.Anything, QueryResolveIdentifier,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, q dbmodel.DBQuery, args ...interface{}) (
			[]map[string]interface{}, error) {
			rows, qErr := db.Query(q.GetQuery("sqlite"), args...)
			if qErr != nil {
				return nil, qErr
			}
			defer func() { _ = rows.Close() }()
			var out []map[string]interface{}
			for rows.Next() {
				var id string
				if qErr := rows.Scan(&id); qErr != nil {
					return nil, qErr
				}
				out = append(out, map[string]interface{}{"id": id})
			}
			return out, rows.Err()
		})

	insert("attacker", json.RawMessage(fmt.Sprintf(`{%q:"sub-1"}`, linkName)), nil)

	_, err = s.store.ResolveLinkedAccount(s.ctx, "idp-a", "sub-1")
	s.ErrorIs(err, ErrEntityNotFound, "an attribute-sourced row must not resolve as a link")

	insert("victim", nil, json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{}}}}`))

	got, err := s.store.ResolveLinkedAccount(s.ctx, "idp-a", "sub-1")
	s.Require().NoError(err, "an attribute-sourced row must not make the recorded link ambiguous")
	s.Equal("victim", *got)
}

func (s *DBStoreTestSuite) TestResolveLinkedAccount_Ambiguous() {
	s.provider.On("GetEntityDBClient").Return(s.client, nil)
	s.client.EXPECT().QueryContext(mock.Anything, QueryResolveIdentifier,
		linkedIdentifierName("idp-a"), "sub-1", identifierSourceSystem, mock.Anything).
		Return([]map[string]interface{}{{"id": "e1"}, {"id": "e2"}}, nil)

	_, err := s.store.ResolveLinkedAccount(s.ctx, "idp-a", "sub-1")
	s.ErrorIs(err, ErrAmbiguousEntity)
}

type StoreHelpersTestSuite struct {
	suite.Suite
}

func TestStoreHelpersTestSuite(t *testing.T) {
	suite.Run(t, new(StoreHelpersTestSuite))
}

func goodRow() map[string]interface{} {
	return map[string]interface{}{
		"id":                "entity-1",
		"ou_id":             "ou-1",
		"category":          "user",
		"type":              "employee",
		"state":             "ACTIVE",
		"attributes":        `{"email":"a@b.com"}`,
		"system_attributes": `{"key":"val"}`,
	}
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_Success() {
	e, err := buildEntityFromResultRow(goodRow())
	s.NoError(err)
	s.Equal("entity-1", e.ID)
	s.Equal(providers.EntityCategoryUser, e.Category)
	s.Equal("employee", e.Type)
	s.Equal(providers.EntityStateActive, e.State)
	s.Equal("ou-1", e.OUID)
	s.NotNil(e.Attributes)
	s.NotNil(e.SystemAttributes)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_AttributesAsBytes() {
	row := goodRow()
	row["attributes"] = []byte(`{"email":"a@b.com"}`)
	row["system_attributes"] = []byte(`{"k":"v"}`)
	e, err := buildEntityFromResultRow(row)
	s.NoError(err)
	s.Equal("entity-1", e.ID)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_MissingID() {
	row := goodRow()
	delete(row, "id")
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_MissingOUID() {
	row := goodRow()
	delete(row, "ou_id")
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_MissingCategory() {
	row := goodRow()
	delete(row, "category")
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_MissingType() {
	row := goodRow()
	delete(row, "type")
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_MissingState() {
	row := goodRow()
	delete(row, "state")
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_BadAttributes() {
	row := goodRow()
	row["attributes"] = 12345 // unknown type
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntityFromResultRow_InvalidAttributesJSON() {
	row := goodRow()
	row["attributes"] = `not-valid-json`
	_, err := buildEntityFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildGroupFromResultRow_Success() {
	row := map[string]interface{}{"id": "g1", "name": "GroupA", "ou_id": "ou1"}
	g, err := buildGroupFromResultRow(row)
	s.NoError(err)
	s.Equal("g1", g.ID)
	s.Equal("GroupA", g.Name)
	s.Equal("ou1", g.OUID)
}

func (s *StoreHelpersTestSuite) TestBuildGroupFromResultRow_MissingID() {
	row := map[string]interface{}{"name": "GroupA", "ou_id": "ou1"}
	_, err := buildGroupFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildGroupFromResultRow_MissingName() {
	row := map[string]interface{}{"id": "g1", "ou_id": "ou1"}
	_, err := buildGroupFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildGroupFromResultRow_MissingOUID() {
	row := map[string]interface{}{"id": "g1", "name": "GroupA"}
	_, err := buildGroupFromResultRow(row)
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestBuildEntitiesFromResults_Empty() {
	entities, err := buildEntitiesFromResults([]map[string]interface{}{})
	s.NoError(err)
	s.Empty(entities)
}

func (s *StoreHelpersTestSuite) TestBuildEntitiesFromResults_Success() {
	entities, err := buildEntitiesFromResults([]map[string]interface{}{goodRow()})
	s.NoError(err)
	s.Len(entities, 1)
}

func (s *StoreHelpersTestSuite) TestBuildEntitiesFromResults_Error() {
	bad := goodRow()
	delete(bad, "id")
	_, err := buildEntitiesFromResults([]map[string]interface{}{bad})
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_String() {
	row := map[string]interface{}{"col": `{"k":"v"}`}
	v := parseJSONColumn(row, "col")
	s.NotNil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_EmptyString() {
	row := map[string]interface{}{"col": ""}
	v := parseJSONColumn(row, "col")
	s.Nil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_EmptyObject() {
	row := map[string]interface{}{"col": "{}"}
	v := parseJSONColumn(row, "col")
	s.NotNil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_Bytes() {
	row := map[string]interface{}{"col": []byte(`{"k":"v"}`)}
	v := parseJSONColumn(row, "col")
	s.NotNil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_BytesEmpty() {
	row := map[string]interface{}{"col": []byte(``)}
	v := parseJSONColumn(row, "col")
	s.Nil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_BytesEmptyObject() {
	row := map[string]interface{}{"col": []byte(`{}`)}
	v := parseJSONColumn(row, "col")
	s.NotNil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_Missing() {
	v := parseJSONColumn(map[string]interface{}{}, "col")
	s.Nil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_Nil() {
	row := map[string]interface{}{"col": nil}
	v := parseJSONColumn(row, "col")
	s.Nil(v)
}

func (s *StoreHelpersTestSuite) TestParseJSONColumn_UnknownType() {
	row := map[string]interface{}{"col": 12345}
	v := parseJSONColumn(row, "col")
	s.Nil(v)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_NoIndexedAttrs() {
	attrs := json.RawMessage(`{"email":"a@b.com"}`)
	query, args, err := prepareIdentifierQuery("e1", attrs, nil, map[string]bool{}, "dep1")
	s.NoError(err)
	s.Nil(query)
	s.Nil(args)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_WithIndexedAttr() {
	attrs := json.RawMessage(`{"email":"a@b.com","username":"user1"}`)
	indexed := map[string]bool{"email": true}
	query, args, err := prepareIdentifierQuery("e1", attrs, nil, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query)
	s.NotEmpty(args)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_Deduplication() {
	// Same key in both schema and system attributes; system should win.
	attrs := json.RawMessage(`{"email":"schema@b.com"}`)
	sysAttrs := json.RawMessage(`{"email":"system@b.com"}`)
	indexed := map[string]bool{"email": true}
	query, args, err := prepareIdentifierQuery("e1", attrs, sysAttrs, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query)
	// Find the email value in args — system value should be present
	found := false
	for _, arg := range args {
		if str, ok := arg.(string); ok && str == "system@b.com" {
			found = true
		}
	}
	s.True(found, "system attribute email should win over schema attribute")
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_LinkedIDsIndexedUnconditionally() {
	// Linked accounts are server-owned. Gating them on user.indexed_attributes would let a missing
	// config line silently break sign-in through a link, so no indexed attributes are configured here.
	sysAttrs := json.RawMessage(
		`{"linkedIds":{"idp-a":{"sub-1":{}},"idp-b":{"sub-2":{}}}}`)
	query, args, err := prepareIdentifierQuery("e1", nil, sysAttrs, map[string]bool{}, "dep1")
	s.NoError(err)
	s.NotNil(query)

	rows := identifierArgPairs(args)
	s.Len(rows, 2)
	s.Contains(rows, identifierRow{linkedIdentifierName("idp-a"), "sub-1"})
	s.Contains(rows, identifierRow{linkedIdentifierName("idp-b"), "sub-2"})
}

// One connection can hold several accounts for the same user. The subjects share the connection's
// identifier name and differ by value, which the primary key on (ENTITY_ID, DEPLOYMENT_ID, NAME,
// VALUE) keeps apart.
func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_LinkedIDsMultipleSubjectsPerIDP() {
	sysAttrs := json.RawMessage(`{"linkedIds":{"idp-a":{"sub-1":{},"sub-2":{}}}}`)
	query, args, err := prepareIdentifierQuery("e1", nil, sysAttrs, map[string]bool{}, "dep1")
	s.NoError(err)
	s.NotNil(query)

	rows := identifierArgPairs(args)
	s.Len(rows, 2, "both accounts at the connection must be indexed")
	s.Contains(rows, identifierRow{linkedIdentifierName("idp-a"), "sub-1"})
	s.Contains(rows, identifierRow{linkedIdentifierName("idp-a"), "sub-2"})
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_LinkedIDsMalformedEntriesSkipped() {
	// A malformed entry must not fail an otherwise valid system attribute write.
	sysAttrs := json.RawMessage(`{"linkedIds":{` +
		`"idp-a":{"sub-1":{}},` +
		`"idp-b":["sub-2"],` +
		`"idp-c":"sub-3",` +
		`"idp-d":{"":{}},` +
		`"":{"sub-4":{}}}}`)
	query, args, err := prepareIdentifierQuery("e1", nil, sysAttrs, map[string]bool{}, "dep1")
	s.NoError(err)
	s.NotNil(query)
	s.Len(args, 6, "only the well-formed link should be indexed")
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_LinkedIDsNonMapIgnored() {
	sysAttrs := json.RawMessage(`{"linkedIds":"nonsense"}`)
	query, args, err := prepareIdentifierQuery("e1", nil, sysAttrs, map[string]bool{}, "dep1")
	s.NoError(err)
	s.Nil(query)
	s.Nil(args)
}

func (s *StoreHelpersTestSuite) TestlinkedIdentifierName_BoundedAndDistinct() {
	// NAME is VARCHAR(255). The subject is the row's value, not part of the name, so even the
	// 255-character subject OIDC permits leaves the name well within the column.
	idpID := "0195f0a1-2b3c-7d4e-8f90-a1b2c3d4e5f6"

	name := linkedIdentifierName(idpID)
	s.Less(len(name), 256)
	s.Equal("linkedIds."+idpID, name,
		"the connection id stays in the clear so links are enumerable by prefix")
	s.NotEqual(name, linkedIdentifierName("other-idp"))
}

// identifierRow is one (name, value) pair read back out of a batch insert's args.
type identifierRow struct {
	name  string
	value string
}

// identifierArgPairs lists the (name, value) pairs in a batch insert's args. A name can repeat with
// different values, so this is a list rather than a map. Rows are six placeholders wide: entity id,
// name, value, source, deployment id, created at.
func identifierArgPairs(args []interface{}) []identifierRow {
	var rows []identifierRow
	for i := 0; i+2 < len(args); i += 6 {
		name, _ := args[i+1].(string)
		value, _ := args[i+2].(string)
		rows = append(rows, identifierRow{name: name, value: value})
	}
	return rows
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_InvalidAttributesJSON() {
	_, _, err := prepareIdentifierQuery("e1", json.RawMessage(`invalid`), nil, map[string]bool{"email": true}, "dep1")
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_InvalidSystemAttributesJSON() {
	attrs := json.RawMessage(`{"email":"a@b.com"}`)
	_, _, err := prepareIdentifierQuery("e1", attrs, json.RawMessage(`bad`), map[string]bool{"email": true}, "dep1")
	s.Error(err)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_NumericAndBoolValues() {
	attrs := json.RawMessage(`{"score":99,"active":true,"nested":{"k":"v"}}`)
	indexed := map[string]bool{"score": true, "active": true, "nested": true}
	query, args, err := prepareIdentifierQuery("e1", attrs, nil, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query) // score and active indexed; nested is a map (skipped)
	_ = args
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_MultipleValuesForSameName() {
	attrs := json.RawMessage(`{"email":["a@b.com","c@d.com"]}`)
	indexed := map[string]bool{"email": true}
	query, args, err := prepareIdentifierQuery("e1", attrs, nil, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query)

	var values []string
	for _, arg := range args {
		if str, ok := arg.(string); ok && (str == "a@b.com" || str == "c@d.com") {
			values = append(values, str)
		}
	}
	s.ElementsMatch([]string{"a@b.com", "c@d.com"}, values)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_DuplicateValuesDeduped() {
	attrs := json.RawMessage(`{"email":["a@b.com","a@b.com"]}`)
	indexed := map[string]bool{"email": true}
	query, args, err := prepareIdentifierQuery("e1", attrs, nil, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query)

	count := 0
	for _, arg := range args {
		if str, ok := arg.(string); ok && str == "a@b.com" {
			count++
		}
	}
	s.Equal(1, count, "duplicate values for the same name should be inserted once")
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_SystemArrayOverridesSchemaArray() {
	attrs := json.RawMessage(`{"email":["schema@b.com"]}`)
	sysAttrs := json.RawMessage(`{"email":["sys1@b.com","sys2@b.com"]}`)
	indexed := map[string]bool{"email": true}
	query, args, err := prepareIdentifierQuery("e1", attrs, sysAttrs, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query)

	var values []string
	for _, arg := range args {
		if str, ok := arg.(string); ok {
			values = append(values, str)
		}
	}
	s.Contains(values, "sys1@b.com")
	s.Contains(values, "sys2@b.com")
	s.NotContains(values, "schema@b.com")
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_EmptySystemValueKeepsSchemaValue() {
	attrs := json.RawMessage(`{"email":"schema@b.com"}`)
	indexed := map[string]bool{"email": true}
	for _, sysAttrs := range []string{`{"email":""}`, `{"email":null}`, `{"email":[]}`, `{"email":{"k":"v"}}`} {
		query, args, err := prepareIdentifierQuery("e1", attrs, json.RawMessage(sysAttrs), indexed, "dep1")
		s.NoError(err)
		s.NotNil(query, sysAttrs)
		s.Contains(args, "schema@b.com", sysAttrs)
	}
}

func (s *StoreHelpersTestSuite) TestAttrValueToString() {
	s.Equal("hello", attrValueToString("hello"))
	s.Equal("3.14", attrValueToString(float64(3.14)))
	s.Equal("42", attrValueToString(int(42)))
	s.Equal("100", attrValueToString(int64(100)))
	s.Equal("true", attrValueToString(true))
	s.Equal("", attrValueToString([]string{"unsupported"}))
}

func (s *StoreHelpersTestSuite) TestAttrValueToStrings() {
	s.Equal([]string{"hello"}, attrValueToStrings("hello"))
	s.Equal([]string{"42"}, attrValueToStrings(int(42)))
	s.Nil(attrValueToStrings(map[string]interface{}{"k": "v"}))
	s.Equal([]string{"a@b.com", "c@d.com"},
		attrValueToStrings([]interface{}{"a@b.com", "c@d.com"}))
	s.Equal([]string{"a@b.com"},
		attrValueToStrings([]interface{}{"a@b.com", map[string]interface{}{"k": "v"}}))
}

func (s *StoreHelpersTestSuite) TestValidateIndexedAttributesConfig_WithinLimit() {
	attrs := make([]string, MaxIndexedAttributesCount)
	err := validateIndexedAttributesConfig(attrs)
	s.NoError(err)
}

func (s *StoreHelpersTestSuite) TestValidateIndexedAttributesConfig_ExceedsLimit() {
	attrs := make([]string, MaxIndexedAttributesCount+1)
	err := validateIndexedAttributesConfig(attrs)
	s.Error(err)
}

func emailArrayAttrs(count int) json.RawMessage {
	emails := make([]string, count)
	for i := range emails {
		emails[i] = fmt.Sprintf("user%d@example.com", i)
	}
	raw, _ := json.Marshal(map[string]interface{}{"email": emails})
	return raw
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_ValuesAtLimitIndexed() {
	indexed := map[string]bool{"email": true}
	query, _, err := prepareIdentifierQuery("e1", emailArrayAttrs(maxIndexedValuesPerAttribute), nil, indexed, "dep1")
	s.NoError(err)
	s.NotNil(query)
}

func (s *StoreHelpersTestSuite) TestPrepareIdentifierQuery_StoredValuesOverLimitIndexed() {
	// The limit applies to input only. Stored data over it is re-indexed in full, so an entity
	// stored before the limit existed can still be updated.
	indexed := map[string]bool{"email": true}
	over := emailArrayAttrs(maxIndexedValuesPerAttribute + 1)

	_, args, err := prepareIdentifierQuery("e1", over, nil, indexed, "dep1")
	s.NoError(err)
	s.Len(args, (maxIndexedValuesPerAttribute+1)*6)

	_, args, err = prepareIdentifierQuery("e1", nil, over, indexed, "dep1")
	s.NoError(err)
	s.Len(args, (maxIndexedValuesPerAttribute+1)*6, "system attributes are indexed in full too")
}

func (s *StoreHelpersTestSuite) TestValidateIndexedValueCounts_OverLimitReturnsLimitError() {
	var attrMap map[string]interface{}
	s.Require().NoError(json.Unmarshal(emailArrayAttrs(maxIndexedValuesPerAttribute+1), &attrMap))
	err := validateIndexedValueCounts(attrMap, map[string]bool{"email": true})
	s.ErrorIs(err, ErrIndexedValueLimitExceeded)
	s.ErrorContains(err, "indexed attribute 'email' has more than")
}

func (s *StoreHelpersTestSuite) TestValidateIndexedValueCounts_IgnoresNonIndexedNames() {
	var attrMap map[string]interface{}
	s.Require().NoError(json.Unmarshal(emailArrayAttrs(maxIndexedValuesPerAttribute+1), &attrMap))
	s.NoError(validateIndexedValueCounts(attrMap, map[string]bool{"username": true}))
}
