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
	"testing"

	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	entitystore "github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	store *entityFileBasedStore
	ctx   context.Context
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entitystore.KeyTypeEntity)
	s.store = &entityFileBasedStore{GenericFileBasedStore: genericStore}
	s.ctx = context.Background()
}

func makeTestEntity(id, category, ouID string) Entity {
	attrs, _ := json.Marshal(map[string]interface{}{"username": id + "-user", "email": id + "@test.com"})
	return Entity{
		ID:         id,
		Category:   EntityCategory(category),
		Type:       "employee",
		State:      EntityStateActive,
		OUID:       ouID,
		Attributes: json.RawMessage(attrs),
	}
}

func (s *FileBasedStoreTestSuite) seedEntity(e Entity) {
	entry := &entityStoreEntry{Entity: e}
	s.Require().NoError(s.store.GenericFileBasedStore.Create(e.ID, entry))
}

func (s *FileBasedStoreTestSuite) TestCreate_InvalidType() {
	err := s.store.Create("id1", "not-an-entity-store-entry")
	s.Error(err)
}

func (s *FileBasedStoreTestSuite) TestCreate_ValidType() {
	e := makeTestEntity("e1", "user", "ou1")
	entry := &entityStoreEntry{Entity: e}
	err := s.store.Create("e1", entry)
	s.NoError(err)
}

func (s *FileBasedStoreTestSuite) TestCreateEntity() {
	e := makeTestEntity("e2", "user", "ou1")
	err := s.store.CreateEntity(s.ctx, e, nil, nil)
	s.NoError(err)

	got, err := s.store.GetEntity(s.ctx, "e2")
	s.NoError(err)
	s.Equal(e.ID, got.ID)
}

func (s *FileBasedStoreTestSuite) TestGetEntity_NotFound() {
	_, err := s.store.GetEntity(s.ctx, "nonexistent")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *FileBasedStoreTestSuite) TestGetEntity_CorruptedData() {
	s.Require().NoError(s.store.GenericFileBasedStore.Create("bad-id", "not-an-entry"))
	_, err := s.store.GetEntity(s.ctx, "bad-id")
	s.Error(err)
}

func (s *FileBasedStoreTestSuite) TestGetEntityWithCredentials_Found() {
	e := makeTestEntity("e3", "user", "ou1")
	creds := json.RawMessage(`{"password":"hashed"}`)
	sysCreds := json.RawMessage(`{"token":"abc"}`)
	s.Require().NoError(s.store.CreateEntity(s.ctx, e, creds, sysCreds))

	result, err := s.store.GetEntityWithCredentials(s.ctx, "e3")
	s.NoError(err)
	s.Equal(e.ID, result.Entity.ID)
	s.Equal(string(creds), string(result.SchemaCredentials))
	s.Equal(string(sysCreds), string(result.SystemCredentials))
}

func (s *FileBasedStoreTestSuite) TestGetEntityWithCredentials_NotFound() {
	_, err := s.store.GetEntityWithCredentials(s.ctx, "missing")
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *FileBasedStoreTestSuite) TestGetEntityWithCredentials_Corrupted() {
	s.Require().NoError(s.store.GenericFileBasedStore.Create("bad2", "corrupted"))
	_, err := s.store.GetEntityWithCredentials(s.ctx, "bad2")
	s.Error(err)
}

func (s *FileBasedStoreTestSuite) TestUnsupportedMutations() {
	e := makeTestEntity("e4", "user", "ou1")

	s.Error(s.store.UpdateEntity(s.ctx, &e))
	s.Error(s.store.UpdateSystemAttributes(s.ctx, "e4", nil))
	s.Error(s.store.UpdateCredentials(s.ctx, "e4", nil))
	s.Error(s.store.UpdateSystemCredentials(s.ctx, "e4", nil))
	s.Error(s.store.DeleteEntity(s.ctx, "e4"))
}

func (s *FileBasedStoreTestSuite) TestIdentifyEntity_NoMatch() {
	s.seedEntity(makeTestEntity("e5", "user", "ou1"))
	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "nobody@nowhere.com"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *FileBasedStoreTestSuite) TestIdentifyEntity_OneMatch() {
	e := makeTestEntity("e6", "user", "ou1")
	s.seedEntity(e)
	id, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "e6@test.com"})
	s.NoError(err)
	s.Equal("e6", *id)
}

func (s *FileBasedStoreTestSuite) TestIdentifyEntity_MultipleMatches() {
	attrs1, _ := json.Marshal(map[string]interface{}{"email": "dup@test.com"})
	attrs2, _ := json.Marshal(map[string]interface{}{"email": "dup@test.com"})
	e1 := Entity{ID: "dup1", Category: EntityCategoryUser, Attributes: json.RawMessage(attrs1)}
	e2 := Entity{ID: "dup2", Category: EntityCategoryUser, Attributes: json.RawMessage(attrs2)}
	s.seedEntity(e1)
	s.seedEntity(e2)

	_, err := s.store.IdentifyEntity(s.ctx, map[string]interface{}{"email": "dup@test.com"})
	s.Error(err)
}

func (s *FileBasedStoreTestSuite) TestGetEntityListCount_WithCategoryAndFilter() {
	s.seedEntity(makeTestEntity("u1", "user", "ou1"))
	s.seedEntity(makeTestEntity("u2", "user", "ou1"))
	s.seedEntity(makeTestEntity("a1", "app", "ou1"))

	count, err := s.store.GetEntityListCount(s.ctx, "user", nil)
	s.NoError(err)
	s.Equal(2, count)

	count, err = s.store.GetEntityListCount(s.ctx, "user", map[string]interface{}{"email": "u1@test.com"})
	s.NoError(err)
	s.Equal(1, count)
}

func (s *FileBasedStoreTestSuite) TestGetEntityList_Pagination() {
	for i := 0; i < 5; i++ {
		s.seedEntity(makeTestEntity("p"+string(rune('0'+i)), "user", "ou1"))
	}

	all, err := s.store.GetEntityList(s.ctx, "user", 0, 0, nil)
	s.NoError(err)
	s.Len(all, 5)

	page, err := s.store.GetEntityList(s.ctx, "user", 2, 1, nil)
	s.NoError(err)
	s.Len(page, 2)

	empty, err := s.store.GetEntityList(s.ctx, "user", 2, 100, nil)
	s.NoError(err)
	s.Empty(empty)

	negLimit, err := s.store.GetEntityList(s.ctx, "user", -1, 0, nil)
	s.NoError(err)
	s.Empty(negLimit)
}

func (s *FileBasedStoreTestSuite) TestGetEntityListCountByOUIDs() {
	s.seedEntity(makeTestEntity("ou1e1", "user", "ou-A"))
	s.seedEntity(makeTestEntity("ou2e1", "user", "ou-B"))
	s.seedEntity(makeTestEntity("ou3e1", "user", "ou-C"))

	count, err := s.store.GetEntityListCountByOUIDs(s.ctx, "user", []string{"ou-A", "ou-B"}, nil)
	s.NoError(err)
	s.Equal(2, count)
}

func (s *FileBasedStoreTestSuite) TestGetEntityListByOUIDs_WithFilter() {
	s.seedEntity(makeTestEntity("flt1", "user", "ou-X"))
	s.seedEntity(makeTestEntity("flt2", "user", "ou-X"))

	filter := map[string]interface{}{"email": "flt1@test.com"}
	list, err := s.store.GetEntityListByOUIDs(s.ctx, "user", []string{"ou-X"}, 0, 0, filter)
	s.NoError(err)
	s.Len(list, 1)
	s.Equal("flt1", list[0].ID)
}

func (s *FileBasedStoreTestSuite) TestGetGroupCountForEntity() {
	count, err := s.store.GetGroupCountForEntity(s.ctx, "any-id")
	s.NoError(err)
	s.Equal(0, count)
}

func (s *FileBasedStoreTestSuite) TestGetEntityGroups() {
	groups, err := s.store.GetEntityGroups(s.ctx, "any-id", 10, 0)
	s.NoError(err)
	s.Empty(groups)
}

func (s *FileBasedStoreTestSuite) TestValidateEntityIDs() {
	s.seedEntity(makeTestEntity("v1", "user", "ou1"))
	s.seedEntity(makeTestEntity("v2", "user", "ou1"))

	invalid, err := s.store.ValidateEntityIDs(s.ctx, []string{"v1", "v2", "missing"})
	s.NoError(err)
	s.Equal([]string{"missing"}, invalid)
}

func (s *FileBasedStoreTestSuite) TestGetEntitiesByIDs_Empty() {
	result, err := s.store.GetEntitiesByIDs(s.ctx, []string{})
	s.NoError(err)
	s.Empty(result)
}

func (s *FileBasedStoreTestSuite) TestGetEntitiesByIDs_MixedExists() {
	s.seedEntity(makeTestEntity("ge1", "user", "ou1"))

	result, err := s.store.GetEntitiesByIDs(s.ctx, []string{"ge1", "nope"})
	s.NoError(err)
	s.Len(result, 1)
	s.Equal("ge1", result[0].ID)
}

func (s *FileBasedStoreTestSuite) TestValidateEntityIDsInOUs_EmptyEntityIDs() {
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{}, []string{"ou1"})
	s.NoError(err)
	s.Empty(out)
}

func (s *FileBasedStoreTestSuite) TestValidateEntityIDsInOUs_EmptyOUIDs() {
	out, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"e1", "e2"}, []string{})
	s.NoError(err)
	s.Equal([]string{"e1", "e2"}, out)
}

func (s *FileBasedStoreTestSuite) TestValidateEntityIDsInOUs_Mixed() {
	s.seedEntity(makeTestEntity("in1", "user", "ou-A"))
	s.seedEntity(makeTestEntity("out1", "user", "ou-B"))

	outOfScope, err := s.store.ValidateEntityIDsInOUs(s.ctx, []string{"in1", "out1", "missing"}, []string{"ou-A"})
	s.NoError(err)
	s.ElementsMatch([]string{"out1", "missing"}, outOfScope)
}

func (s *FileBasedStoreTestSuite) TestIsEntityDeclarative() {
	s.seedEntity(makeTestEntity("decl1", "user", "ou1"))

	ok, err := s.store.IsEntityDeclarative(s.ctx, "decl1")
	s.NoError(err)
	s.True(ok)

	ok, err = s.store.IsEntityDeclarative(s.ctx, "nonexistent")
	s.NoError(err)
	s.False(ok)
}

func (s *FileBasedStoreTestSuite) TestGetIndexedAttributes() {
	s.Nil(s.store.GetIndexedAttributes())
}

func (s *FileBasedStoreTestSuite) TestApplyPagination() {
	entities := []Entity{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}}

	s.Len(applyPagination(entities, 2, 0), 2)
	s.Len(applyPagination(entities, 0, 0), 5)
	s.Len(applyPagination(entities, 10, 3), 2)
	s.Empty(applyPagination(entities, 2, 10))
	s.Empty(applyPagination(entities, -1, 0))
	s.Len(applyPagination(entities, 3, -1), 3)
}

func (s *FileBasedStoreTestSuite) TestMatchesFilters() {
	attrs := json.RawMessage(`{"email":"a@b.com","nested":{"key":"val"},"count":1}`)

	s.True(matchesFilters(attrs, nil))
	s.True(matchesFilters(attrs, map[string]interface{}{"email": "a@b.com"}))
	s.False(matchesFilters(attrs, map[string]interface{}{"email": "wrong@b.com"}))
	s.True(matchesFilters(attrs, map[string]interface{}{"nested.key": "val"}))
	s.False(matchesFilters(attrs, map[string]interface{}{"nested.missing": "val"}))
	s.False(matchesFilters(nil, map[string]interface{}{"email": "a@b.com"}))
	s.False(matchesFilters(json.RawMessage(`invalid-json`), map[string]interface{}{"k": "v"}))
}

func (s *FileBasedStoreTestSuite) TestGetNestedValue() {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "found",
		},
		"flat": "value",
	}

	val, ok := getNestedValue(data, "a.b")
	s.True(ok)
	s.Equal("found", val)

	_, ok = getNestedValue(data, "a.missing")
	s.False(ok)

	_, ok = getNestedValue(data, "flat.sub")
	s.False(ok)
}

func (s *FileBasedStoreTestSuite) TestValuesEqual() {
	s.True(valuesEqual(float64(42), int64(42)))
	s.True(valuesEqual(float64(3.14), float64(3.14)))
	s.True(valuesEqual(float64(5), int(5)))
	s.False(valuesEqual(float64(5), int64(6)))
	s.True(valuesEqual("hello", "hello"))
	s.False(valuesEqual("hello", "world"))
	s.True(valuesEqual(true, true))
	s.False(valuesEqual(true, false))
	s.False(valuesEqual(float64(1), "1"))
	s.False(valuesEqual("x", 1))
	s.False(valuesEqual(true, "true"))
}

func (s *FileBasedStoreTestSuite) TestSearchEntities_NoMatch() {
	s.seedEntity(makeTestEntity("s1", "user", "ou1"))
	_, err := s.store.SearchEntities(s.ctx, map[string]interface{}{"email": "nobody@nowhere.com"})
	s.ErrorIs(err, ErrEntityNotFound)
}

func (s *FileBasedStoreTestSuite) TestSearchEntities_OneMatch() {
	s.seedEntity(makeTestEntity("s2", "user", "ou1"))
	got, err := s.store.SearchEntities(s.ctx, map[string]interface{}{"email": "s2@test.com"})
	s.NoError(err)
	s.Len(got, 1)
	s.Equal("s2", got[0].ID)
}

func (s *FileBasedStoreTestSuite) TestSearchEntities_MultipleMatches() {
	attrs, _ := json.Marshal(map[string]interface{}{"email": "shared@test.com"})
	e1 := Entity{ID: "m1", Category: EntityCategoryUser, Attributes: json.RawMessage(attrs)}
	e2 := Entity{ID: "m2", Category: EntityCategoryUser, Attributes: json.RawMessage(attrs)}
	s.seedEntity(e1)
	s.seedEntity(e2)
	got, err := s.store.SearchEntities(s.ctx, map[string]interface{}{"email": "shared@test.com"})
	s.NoError(err)
	s.Len(got, 2)
}
