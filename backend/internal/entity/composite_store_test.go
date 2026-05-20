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

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	dbStore   *entityStoreInterfaceMock
	fileStore *entityStoreInterfaceMock
	store     *entityCompositeStore
	ctx       context.Context
	testErr   error
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (s *CompositeStoreTestSuite) SetupTest() {
	s.dbStore = newEntityStoreInterfaceMock(s.T())
	s.fileStore = newEntityStoreInterfaceMock(s.T())
	s.store = newEntityCompositeStore(s.fileStore, s.dbStore)
	s.ctx = context.Background()
	s.testErr = errors.New("store error")
}

func compEntity(id, ouID string) Entity {
	return Entity{ID: id, Category: EntityCategoryUser, OUID: ouID}
}

func (s *CompositeStoreTestSuite) TestCreateEntity_DelegatesToDB() {
	e := compEntity("e1", "ou1")
	s.dbStore.On("CreateEntity", mock.Anything, e, json.RawMessage(nil), json.RawMessage(nil)).Return(nil)
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.NoError(err)
}

func (s *CompositeStoreTestSuite) TestGetEntity_DBFound() {
	e := compEntity("e1", "ou1")
	s.dbStore.On("GetEntity", mock.Anything, "e1").Return(e, nil)
	got, err := s.store.GetEntity(s.ctx, "e1")
	s.NoError(err)
	s.Equal("e1", got.ID)
	s.False(got.IsReadOnly)
}

func (s *CompositeStoreTestSuite) TestGetEntity_DBNotFound_FileFound() {
	e := compEntity("e2", "ou1")
	s.dbStore.On("GetEntity", mock.Anything, "e2").Return(Entity{}, ErrEntityNotFound)
	s.fileStore.On("GetEntity", mock.Anything, "e2").Return(e, nil)
	got, err := s.store.GetEntity(s.ctx, "e2")
	s.NoError(err)
	s.True(got.IsReadOnly)
}

func (s *CompositeStoreTestSuite) TestGetEntity_DBError() {
	s.dbStore.On("GetEntity", mock.Anything, "e3").Return(Entity{}, s.testErr)
	_, err := s.store.GetEntity(s.ctx, "e3")
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntity_BothNotFound() {
	s.dbStore.On("GetEntity", mock.Anything, "e4").Return(Entity{}, ErrEntityNotFound)
	s.fileStore.On("GetEntity", mock.Anything, "e4").Return(Entity{}, ErrEntityNotFound)
	_, err := s.store.GetEntity(s.ctx, "e4")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *CompositeStoreTestSuite) TestGetEntityWithCredentials_DBFound() {
	e := compEntity("c1", "ou1")
	creds := json.RawMessage(`{"p":"h"}`)
	s.dbStore.On("GetEntityWithCredentials", mock.Anything, "c1").
		Return(&entityWithCredentials{Entity: &e, SchemaCredentials: creds, SystemCredentials: nil}, nil)
	result, err := s.store.GetEntityWithCredentials(s.ctx, "c1")
	s.NoError(err)
	s.Equal("c1", result.Entity.ID)
	s.Equal(string(creds), string(result.SchemaCredentials))
}

func (s *CompositeStoreTestSuite) TestGetEntityWithCredentials_DBNotFound_FileFound() {
	e := compEntity("c2", "ou1")
	s.dbStore.On("GetEntityWithCredentials", mock.Anything, "c2").
		Return(nil, ErrEntityNotFound)
	s.fileStore.On("GetEntityWithCredentials", mock.Anything, "c2").
		Return(&entityWithCredentials{Entity: &e, SchemaCredentials: nil, SystemCredentials: nil}, nil)
	result, err := s.store.GetEntityWithCredentials(s.ctx, "c2")
	s.NoError(err)
	s.True(result.Entity.IsReadOnly)
}

func (s *CompositeStoreTestSuite) TestGetEntityWithCredentials_DBError() {
	s.dbStore.On("GetEntityWithCredentials", mock.Anything, "c3").
		Return(nil, s.testErr)
	_, err := s.store.GetEntityWithCredentials(s.ctx, "c3")
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityWithCredentials_BothNotFound() {
	s.dbStore.On("GetEntityWithCredentials", mock.Anything, "c4").
		Return(nil, ErrEntityNotFound)
	s.fileStore.On("GetEntityWithCredentials", mock.Anything, "c4").
		Return(nil, ErrEntityNotFound)
	_, err := s.store.GetEntityWithCredentials(s.ctx, "c4")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *CompositeStoreTestSuite) TestUpdateEntity_Delegates() {
	e := compEntity("u1", "ou1")
	s.dbStore.On("UpdateEntity", mock.Anything, &e).Return(nil)
	s.NoError(s.store.UpdateEntity(s.ctx, &e))
}

func (s *CompositeStoreTestSuite) TestUpdateSystemAttributes_Delegates() {
	s.dbStore.On("UpdateSystemAttributes", mock.Anything, "u1", mock.Anything).Return(nil)
	s.NoError(s.store.UpdateSystemAttributes(s.ctx, "u1", nil))
}

func (s *CompositeStoreTestSuite) TestUpdateCredentials_Delegates() {
	s.dbStore.On("UpdateCredentials", mock.Anything, "u1", mock.Anything).Return(nil)
	s.NoError(s.store.UpdateCredentials(s.ctx, "u1", nil))
}

func (s *CompositeStoreTestSuite) TestUpdateSystemCredentials_Delegates() {
	s.dbStore.On("UpdateSystemCredentials", mock.Anything, "u1", mock.Anything).Return(nil)
	s.NoError(s.store.UpdateSystemCredentials(s.ctx, "u1", nil))
}

func (s *CompositeStoreTestSuite) TestDeleteEntity_Delegates() {
	s.dbStore.On("DeleteEntity", mock.Anything, "d1").Return(nil)
	s.NoError(s.store.DeleteEntity(s.ctx, "d1"))
}

func (s *CompositeStoreTestSuite) TestGetGroupCountForEntity_Delegates() {
	s.dbStore.On("GetGroupCountForEntity", mock.Anything, "e1").Return(3, nil)
	count, err := s.store.GetGroupCountForEntity(s.ctx, "e1")
	s.NoError(err)
	s.Equal(3, count)
}

func (s *CompositeStoreTestSuite) TestGetEntityGroups_Delegates() {
	groups := []EntityGroup{{ID: "g1"}}
	s.dbStore.On("GetEntityGroups", mock.Anything, "e1", 10, 0).Return(groups, nil)
	got, err := s.store.GetEntityGroups(s.ctx, "e1", 10, 0)
	s.NoError(err)
	s.Len(got, 1)
}

func (s *CompositeStoreTestSuite) TestGetIndexedAttributes_Delegates() {
	attrs := map[string]bool{"email": true}
	s.dbStore.On("GetIndexedAttributes").Return(attrs)
	s.Equal(attrs, s.store.GetIndexedAttributes())
}

func (s *CompositeStoreTestSuite) TestIdentifyEntity_DBFound() {
	id := "found"
	filters := map[string]interface{}{"email": "a@b.com"}
	s.dbStore.On("IdentifyEntity", mock.Anything, filters).Return(&id, nil)
	got, err := s.store.IdentifyEntity(s.ctx, filters)
	s.NoError(err)
	s.Equal(&id, got)
}

func (s *CompositeStoreTestSuite) TestIdentifyEntity_DBNotFound_FileFallback() {
	id := "file-id"
	filters := map[string]interface{}{"email": "a@b.com"}
	s.dbStore.On("IdentifyEntity", mock.Anything, filters).Return((*string)(nil), ErrEntityNotFound)
	s.fileStore.On("IdentifyEntity", mock.Anything, filters).Return(&id, nil)
	got, err := s.store.IdentifyEntity(s.ctx, filters)
	s.NoError(err)
	s.Equal(&id, got)
}

func (s *CompositeStoreTestSuite) TestIdentifyEntity_DBError() {
	filters := map[string]interface{}{"email": "a@b.com"}
	s.dbStore.On("IdentifyEntity", mock.Anything, filters).Return((*string)(nil), s.testErr)
	_, err := s.store.IdentifyEntity(s.ctx, filters)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityListCount_MergesStores() {
	e1 := compEntity("e1", "ou1")
	e2 := compEntity("e2", "ou1")
	e3 := compEntity("e3", "ou1") // unique to file
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(2, nil)
	s.fileStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(1, nil)
	s.dbStore.On("GetEntityList", mock.Anything, "user", 2, 0, mock.Anything).Return([]Entity{e1, e2}, nil)
	s.fileStore.On("GetEntityList", mock.Anything, "user", 1, 0, mock.Anything).Return([]Entity{e3}, nil)

	count, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.NoError(err)
	s.Equal(3, count)
}

func (s *CompositeStoreTestSuite) TestGetEntityListCount_DBCountError() {
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(0, s.testErr)
	_, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityListCount_FileCountError() {
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(0, nil)
	s.fileStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(0, s.testErr)
	_, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityListCount_DBListError() {
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(2, nil)
	s.fileStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(0, nil)
	s.dbStore.On("GetEntityList", mock.Anything, "user", 2, 0, mock.Anything).Return(nil, s.testErr)
	_, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityListCount_FileListError() {
	e1 := compEntity("e1", "ou1")
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(1, nil)
	s.fileStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(1, nil)
	s.dbStore.On("GetEntityList", mock.Anything, "user", 1, 0, mock.Anything).Return([]Entity{e1}, nil)
	s.fileStore.On("GetEntityList", mock.Anything, "user", 1, 0, mock.Anything).Return(nil, s.testErr)
	_, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityList_Success() {
	e1 := compEntity("e1", "ou1")
	e2 := compEntity("e2", "ou1")
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(1, nil)
	s.fileStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(1, nil)
	s.dbStore.On("GetEntityList", mock.Anything, "user", 1, 0, mock.Anything).Return([]Entity{e1}, nil)
	s.fileStore.On("GetEntityList", mock.Anything, "user", 1, 0, mock.Anything).Return([]Entity{e2}, nil)

	list, err := s.store.GetEntityList(s.ctx, "user", 10, 0, nil)
	s.NoError(err)
	s.Len(list, 2)
}

func (s *CompositeStoreTestSuite) TestGetEntityList_LimitExceeded() {
	limit := serverconst.MaxCompositeStoreRecords + 1

	// When total count exceeds the hard limit, CompositeMergeListHelperWithLimit short-circuits
	// and returns errResultLimitExceededInCompositeMode without calling the fetchers.
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(limit, nil)
	s.fileStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(0, nil)

	_, err := s.store.GetEntityList(s.ctx, "user", limit, 0, nil)
	s.ErrorIs(err, errResultLimitExceededInCompositeMode)
}

func (s *CompositeStoreTestSuite) TestGetEntityList_Error() {
	s.dbStore.On("GetEntityListCount", mock.Anything, "user", mock.Anything).Return(0, s.testErr)
	_, err := s.store.GetEntityList(s.ctx, "user", 10, 0, nil)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntityListCountByOUIDs_MergesStores() {
	e1 := compEntity("e1", "ou1")
	e2 := compEntity("e2", "ou1")
	ouIDs := []string{"ou1"}
	s.dbStore.On("GetEntityListCountByOUIDs", mock.Anything, "user", ouIDs, mock.Anything).Return(1, nil)
	s.fileStore.On("GetEntityListCountByOUIDs", mock.Anything, "user", ouIDs, mock.Anything).Return(1, nil)
	s.dbStore.On("GetEntityListByOUIDs", mock.Anything, "user", ouIDs, 1, 0, mock.Anything).Return([]Entity{e1}, nil)
	s.fileStore.On("GetEntityListByOUIDs", mock.Anything, "user", ouIDs, 1, 0, mock.Anything).Return([]Entity{e2}, nil)

	count, err := s.store.GetEntityListCountByOUIDs(s.ctx, "user", ouIDs, nil)
	s.NoError(err)
	s.Equal(2, count)
}

func (s *CompositeStoreTestSuite) TestGetEntityListByOUIDs_LimitExceeded() {
	ouIDs := []string{"ou1"}
	limit := serverconst.MaxCompositeStoreRecords + 1

	// When total count exceeds the hard limit, CompositeMergeListHelperWithLimit short-circuits
	// and returns errResultLimitExceededInCompositeMode without calling the fetchers.
	s.dbStore.On("GetEntityListCountByOUIDs", mock.Anything, "user", ouIDs, mock.Anything).Return(limit, nil)
	s.fileStore.On("GetEntityListCountByOUIDs", mock.Anything, "user", ouIDs, mock.Anything).Return(0, nil)

	_, err := s.store.GetEntityListByOUIDs(s.ctx, "user", ouIDs, limit, 0, nil)
	s.ErrorIs(err, errResultLimitExceededInCompositeMode)
}

func (s *CompositeStoreTestSuite) TestGetEntityListByOUIDs_Error() {
	ouIDs := []string{"ou1"}
	s.dbStore.On("GetEntityListCountByOUIDs", mock.Anything, "user", ouIDs, mock.Anything).Return(0, s.testErr)
	_, err := s.store.GetEntityListByOUIDs(s.ctx, "user", ouIDs, 10, 0, nil)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDs_AllValid() {
	e1 := compEntity("v1", "ou1")
	s.dbStore.On("GetEntity", mock.Anything, "v1").Return(e1, nil)
	invalid, err := s.store.ValidateEntityIDs(s.ctx, []string{"v1"})
	s.NoError(err)
	s.Empty(invalid)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDs_SomeInvalid() {
	e1 := compEntity("v1", "ou1")
	s.dbStore.On("GetEntity", mock.Anything, "v1").Return(e1, nil)
	s.dbStore.On("GetEntity", mock.Anything, "missing").Return(Entity{}, ErrEntityNotFound)
	s.fileStore.On("GetEntity", mock.Anything, "missing").Return(Entity{}, ErrEntityNotFound)
	invalid, err := s.store.ValidateEntityIDs(s.ctx, []string{"v1", "missing"})
	s.NoError(err)
	s.Equal([]string{"missing"}, invalid)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDs_StoreError() {
	s.dbStore.On("GetEntity", mock.Anything, "err-id").Return(Entity{}, s.testErr)
	_, err := s.store.ValidateEntityIDs(s.ctx, []string{"err-id"})
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntitiesByIDs_Empty() {
	list, err := s.store.GetEntitiesByIDs(s.ctx, []string{})
	s.NoError(err)
	s.Empty(list)
}

func (s *CompositeStoreTestSuite) TestGetEntitiesByIDs_DBError() {
	s.dbStore.On("GetEntitiesByIDs", mock.Anything, []string{"id1"}).Return(nil, s.testErr)
	_, err := s.store.GetEntitiesByIDs(s.ctx, []string{"id1"})
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntitiesByIDs_FileError() {
	s.dbStore.On("GetEntitiesByIDs", mock.Anything, []string{"id1"}).Return([]Entity{}, nil)
	s.fileStore.On("GetEntitiesByIDs", mock.Anything, []string{"id1"}).Return(nil, s.testErr)
	_, err := s.store.GetEntitiesByIDs(s.ctx, []string{"id1"})
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestGetEntitiesByIDs_MergeDedup() {
	e1 := compEntity("e1", "ou1")
	e2 := compEntity("e2", "ou1")
	// e1 exists in both — DB takes precedence
	s.dbStore.On("GetEntitiesByIDs", mock.Anything, []string{"e1", "e2"}).Return([]Entity{e1}, nil)
	s.fileStore.On("GetEntitiesByIDs", mock.Anything, []string{"e1", "e2"}).Return([]Entity{e1, e2}, nil)

	list, err := s.store.GetEntitiesByIDs(s.ctx, []string{"e1", "e2"})
	s.NoError(err)
	s.Len(list, 2)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDsInOUs_EmptyEntityIDs() {
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{}, []string{"ou1"})
	s.NoError(err)
	s.Empty(out)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDsInOUs_EmptyOUIDs() {
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"e1", "e2"}, []string{})
	s.NoError(err)
	s.Equal([]string{"e1", "e2"}, out)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDsInOUs_InScope() {
	e := compEntity("in1", "ou-A")
	s.dbStore.On("GetEntity", mock.Anything, "in1").Return(e, nil)
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"in1"}, []string{"ou-A"})
	s.NoError(err)
	s.Empty(out)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDsInOUs_OutOfScope() {
	e := compEntity("out1", "ou-B")
	s.dbStore.On("GetEntity", mock.Anything, "out1").Return(e, nil)
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"out1"}, []string{"ou-A"})
	s.NoError(err)
	s.Equal([]string{"out1"}, out)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDsInOUs_NotFound() {
	s.dbStore.On("GetEntity", mock.Anything, "missing").Return(Entity{}, ErrEntityNotFound)
	s.fileStore.On("GetEntity", mock.Anything, "missing").Return(Entity{}, ErrEntityNotFound)
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"missing"}, []string{"ou-A"})
	s.NoError(err)
	s.Equal([]string{"missing"}, out)
}

func (s *CompositeStoreTestSuite) TestValidateEntityIDsInOUs_StoreError() {
	s.dbStore.On("GetEntity", mock.Anything, "err-id").Return(Entity{}, s.testErr)
	_, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"err-id"}, []string{"ou-A"})
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestIsEntityDeclarative_TrueFromFile() {
	s.fileStore.On("IsEntityDeclarative", mock.Anything, "decl1").Return(true, nil)
	ok, err := s.store.IsEntityDeclarative(s.ctx, "decl1")
	s.NoError(err)
	s.True(ok)
}

func (s *CompositeStoreTestSuite) TestIsEntityDeclarative_FalseInFile_CheckDB() {
	s.fileStore.On("IsEntityDeclarative", mock.Anything, "mut1").Return(false, nil)
	s.dbStore.On("IsEntityDeclarative", mock.Anything, "mut1").Return(false, nil)
	ok, err := s.store.IsEntityDeclarative(s.ctx, "mut1")
	s.NoError(err)
	s.False(ok)
}

func (s *CompositeStoreTestSuite) TestIsEntityDeclarative_FileError() {
	s.fileStore.On("IsEntityDeclarative", mock.Anything, "err1").Return(false, s.testErr)
	_, err := s.store.IsEntityDeclarative(s.ctx, "err1")
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestMergeAndDeduplicateEntities() {
	db1 := Entity{ID: "shared", IsReadOnly: false}
	file1 := Entity{ID: "shared", IsReadOnly: false}
	file2 := Entity{ID: "file-only"}

	result := mergeAndDeduplicateEntities([]Entity{db1}, []Entity{file1, file2})
	s.Len(result, 2)

	idMap := make(map[string]Entity)
	for _, e := range result {
		idMap[e.ID] = e
	}
	s.False(idMap["shared"].IsReadOnly)
	s.True(idMap["file-only"].IsReadOnly)
}

func (s *CompositeStoreTestSuite) TestSearchEntities_DBOnly() {
	filters := map[string]interface{}{"email": "a@b.com"}
	entities := []Entity{compEntity("e1", "ou1"), compEntity("e2", "ou1")}
	s.dbStore.On("SearchEntities", mock.Anything, filters).Return(entities, nil)
	s.fileStore.On("SearchEntities", mock.Anything, filters).Return(nil, ErrEntityNotFound)
	got, err := s.store.SearchEntities(s.ctx, filters)
	s.NoError(err)
	s.Len(got, 2)
}

func (s *CompositeStoreTestSuite) TestSearchEntities_FileOnly() {
	filters := map[string]interface{}{"email": "a@b.com"}
	entities := []Entity{compEntity("f1", "ou1")}
	s.dbStore.On("SearchEntities", mock.Anything, filters).Return(nil, ErrEntityNotFound)
	s.fileStore.On("SearchEntities", mock.Anything, filters).Return(entities, nil)
	got, err := s.store.SearchEntities(s.ctx, filters)
	s.NoError(err)
	s.Len(got, 1)
}

func (s *CompositeStoreTestSuite) TestSearchEntities_BothStores() {
	filters := map[string]interface{}{"email": "a@b.com"}
	dbEntities := []Entity{compEntity("e1", "ou1")}
	fileEntities := []Entity{compEntity("f1", "ou1")}
	s.dbStore.On("SearchEntities", mock.Anything, filters).Return(dbEntities, nil)
	s.fileStore.On("SearchEntities", mock.Anything, filters).Return(fileEntities, nil)
	got, err := s.store.SearchEntities(s.ctx, filters)
	s.NoError(err)
	s.Len(got, 2)
}

func (s *CompositeStoreTestSuite) TestSearchEntities_BothNotFound() {
	filters := map[string]interface{}{"email": "a@b.com"}
	s.dbStore.On("SearchEntities", mock.Anything, filters).Return(nil, ErrEntityNotFound)
	s.fileStore.On("SearchEntities", mock.Anything, filters).Return(nil, ErrEntityNotFound)
	_, err := s.store.SearchEntities(s.ctx, filters)
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *CompositeStoreTestSuite) TestSearchEntities_DBError() {
	filters := map[string]interface{}{"email": "a@b.com"}
	s.dbStore.On("SearchEntities", mock.Anything, filters).Return(nil, s.testErr)
	_, err := s.store.SearchEntities(s.ctx, filters)
	s.Error(err)
}

func (s *CompositeStoreTestSuite) TestSearchEntities_FileError() {
	filters := map[string]interface{}{"email": "a@b.com"}
	dbEntities := []Entity{compEntity("e1", "ou1")}
	s.dbStore.On("SearchEntities", mock.Anything, filters).Return(dbEntities, nil)
	s.fileStore.On("SearchEntities", mock.Anything, filters).Return(nil, s.testErr)
	_, err := s.store.SearchEntities(s.ctx, filters)
	s.Error(err)
}
