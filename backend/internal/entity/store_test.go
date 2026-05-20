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

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/log"
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
		deploymentID:      "dep1",
		indexedAttributes: map[string]bool{},
		dbProvider:        s.provider,
		logger:            log.GetLogger(),
	}
	s.ctx = context.Background()
	s.testErr = errors.New("db error")
}

func (s *DBStoreTestSuite) expectClient() {
	s.provider.On("GetUserDBClient").Return(s.client, nil).Once()
}

func (s *DBStoreTestSuite) expectClientError() {
	s.provider.On("GetUserDBClient").Return(nil, s.testErr).Once()
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
	e := Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestCreateEntity_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	e := Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestCreateEntity_Success_NoIdentifiers() {
	s.expectClient()
	s.onExecAny(1, nil)
	// SyncAttributeIdentifiers: no indexed attributes → no DB call for sync
	e := Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestCreateEntity_WithSystemAttrsAndCreds() {
	s.expectClient()
	s.onExecAny(1, nil)
	// SyncAttributeIdentifiers: no indexed attributes → no DB call for sync
	e := Entity{
		ID:               "e1",
		Attributes:       json.RawMessage(`{}`),
		SystemAttributes: json.RawMessage(`{"key":"val"}`),
	}
	err := s.store.CreateEntity(s.ctx, e, json.RawMessage(`{"p":"h"}`), json.RawMessage(`{"t":"t"}`))
	s.NoError(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_ProviderError() {
	s.expectClientError()
	e := &Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_ExecuteError() {
	s.expectClient()
	s.onExecAny(0, s.testErr)
	e := &Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_NotFound() {
	s.expectClient()
	s.onExecAny(0, nil)
	e := &Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *DBStoreTestSuite) TestUpdateEntity_ReloadError() {
	s.expectClient()
	s.onExecAny(1, nil).Once()          // update entity succeeds
	s.expectClient()                    // for reload (GetEntity)
	s.onQueryAny(nil, s.testErr).Once() // reload query fails
	e := &Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
	err := s.store.UpdateEntity(s.ctx, e)
	s.Error(err)
}

func (s *DBStoreTestSuite) TestUpdateEntity_DeleteIdentifiersError() {
	s.expectClient()
	s.onExecAny(1, nil).Once()                                        // update entity succeeds
	s.expectClient()                                                  // for reload (GetEntity)
	s.onQueryAny([]map[string]interface{}{dbEntityRow()}, nil).Once() // reload succeeds
	s.onExecAny(0, s.testErr).Once()                                  // delete identifiers fails
	e := &Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
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
	e := &Entity{ID: "e1", Attributes: json.RawMessage(`{}`)}
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
	s.onExecAny(1, nil)
	err := s.store.UpdateSystemAttributes(s.ctx, "e1", json.RawMessage(`{}`))
	s.NoError(err)
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
	s.Equal(EntityCategoryUser, e.Category)
	s.Equal("employee", e.Type)
	s.Equal(EntityStateActive, e.State)
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

func (s *StoreHelpersTestSuite) TestAttrValueToString() {
	s.Equal("hello", attrValueToString("hello"))
	s.Equal("3.14", attrValueToString(float64(3.14)))
	s.Equal("42", attrValueToString(int(42)))
	s.Equal("100", attrValueToString(int64(100)))
	s.Equal("true", attrValueToString(true))
	s.Equal("", attrValueToString([]string{"unsupported"}))
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
