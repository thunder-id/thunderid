/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package ou

import (
	"context"

	"strconv"
	"testing"
	"time"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/filter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	testParentOUID = "parent-1"
	testRootOUID   = "root-1"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	store *fileBasedStore
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	// Create a file-based store with test instance
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeOU)
	s.store = &fileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

func (s *FileBasedStoreTestSuite) TestCreateOrganizationUnit() {
	ou := OrganizationUnit{
		ID:          "test-ou-1",
		Handle:      "test",
		Name:        "Test OU",
		Description: "Test organization unit",
		Parent:      nil,
	}

	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	assert.NoError(s.T(), err)

	// Verify it was created
	retrieved, err := s.store.GetOrganizationUnit(context.Background(), "test-ou-1")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), ou.ID, retrieved.ID)
	assert.Equal(s.T(), ou.Name, retrieved.Name)
	assert.Equal(s.T(), ou.Handle, retrieved.Handle)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitNotFound() {
	_, err := s.store.GetOrganizationUnit(context.Background(), "non-existent")
	assert.Error(s.T(), err)
	assert.ErrorIs(s.T(), err, ErrOrganizationUnitNotFound)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitList() {
	// Create root OUs
	ou1 := OrganizationUnit{
		ID:     "root-1",
		Handle: "root1",
		Name:   "Root 1",
		Parent: nil,
	}
	ou2 := OrganizationUnit{
		ID:     "root-2",
		Handle: "root2",
		Name:   "Root 2",
		Parent: nil,
	}

	err := s.store.CreateOrganizationUnit(context.Background(), ou1)
	assert.NoError(s.T(), err)
	err = s.store.CreateOrganizationUnit(context.Background(), ou2)
	assert.NoError(s.T(), err)

	// Create child OU (should not be in root list)
	parentID := testRootOUID
	child := OrganizationUnit{
		ID:     "child-1",
		Handle: "child1",
		Name:   "Child 1",
		Parent: &parentID,
	}
	err = s.store.CreateOrganizationUnit(context.Background(), child)
	assert.NoError(s.T(), err)

	// Get list should only return root OUs
	list, err := s.store.GetOrganizationUnitList(context.Background(), 10, 0, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), list, 2)
}

func (s *FileBasedStoreTestSuite) TestUpdateNotSupported() {
	ou := OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
		Name:   "Test OU",
	}

	err := s.store.UpdateOrganizationUnit(context.Background(), ou)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestDeleteNotSupported() {
	err := s.store.DeleteOrganizationUnit(context.Background(), "test-ou-1")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported")
}

func (s *FileBasedStoreTestSuite) TestCheckOrganizationUnitNameConflict() {
	ou := OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
		Name:   "Test OU",
		Parent: nil,
	}

	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	assert.NoError(s.T(), err)

	// Check for conflict with same name and parent
	conflict, err := s.store.CheckOrganizationUnitNameConflict(context.Background(), "Test OU", nil)
	assert.NoError(s.T(), err)
	assert.True(s.T(), conflict)

	// No conflict with different name
	conflict, err = s.store.CheckOrganizationUnitNameConflict(context.Background(), "Different Name", nil)
	assert.NoError(s.T(), err)
	assert.False(s.T(), conflict)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitChildren() {
	// Create parent
	parent := OrganizationUnit{
		ID:     testParentOUID,
		Handle: "parent",
		Name:   "Parent OU",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), parent)
	assert.NoError(s.T(), err)

	// Create children
	parentID := testParentOUID
	child1 := OrganizationUnit{
		ID:     "child-1",
		Handle: "child1",
		Name:   "Child 1",
		Parent: &parentID,
	}
	child2 := OrganizationUnit{
		ID:     "child-2",
		Handle: "child2",
		Name:   "Child 2",
		Parent: &parentID,
	}

	err = s.store.CreateOrganizationUnit(context.Background(), child1)
	assert.NoError(s.T(), err)
	err = s.store.CreateOrganizationUnit(context.Background(), child2)
	assert.NoError(s.T(), err)

	// Get children
	children, err := s.store.GetOrganizationUnitChildrenList(context.Background(), testParentOUID, 10, 0, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), children, 2)

	// Get children count
	count, err := s.store.GetOrganizationUnitChildrenCount(context.Background(), testParentOUID, nil)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitByPath() {
	// Create hierarchy: root -> engineering -> backend
	root := OrganizationUnit{
		ID:     "root-1",
		Handle: "root",
		Name:   "Root",
		Parent: nil,
	}
	rootID := testRootOUID
	engineering := OrganizationUnit{
		ID:     "eng-1",
		Handle: "engineering",
		Name:   "Engineering",
		Parent: &rootID,
	}
	engID := "eng-1"
	backend := OrganizationUnit{
		ID:     "backend-1",
		Handle: "backend",
		Name:   "Backend",
		Parent: &engID,
	}

	err := s.store.CreateOrganizationUnit(context.Background(), root)
	assert.NoError(s.T(), err)
	err = s.store.CreateOrganizationUnit(context.Background(), engineering)
	assert.NoError(s.T(), err)
	err = s.store.CreateOrganizationUnit(context.Background(), backend)
	assert.NoError(s.T(), err)

	// Test getting by path
	ou, err := s.store.GetOrganizationUnitByPath(context.Background(), []string{"root"})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "root-1", ou.ID)

	ou, err = s.store.GetOrganizationUnitByPath(context.Background(), []string{"root", "engineering"})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "eng-1", ou.ID)

	ou, err = s.store.GetOrganizationUnitByPath(context.Background(), []string{"root", "engineering", "backend"})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "backend-1", ou.ID)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitByPath_NotFound() {
	root := OrganizationUnit{
		ID:     "root-1",
		Handle: "root",
		Name:   "Root",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), root)
	assert.NoError(s.T(), err)

	// Test invalid path
	_, err = s.store.GetOrganizationUnitByPath(context.Background(), []string{"root", "nonexistent"})
	assert.Error(s.T(), err)
	assert.ErrorIs(s.T(), err, ErrOrganizationUnitNotFound)

	// Test completely invalid path
	_, err = s.store.GetOrganizationUnitByPath(context.Background(), []string{"invalid"})
	assert.Error(s.T(), err)
	assert.ErrorIs(s.T(), err, ErrOrganizationUnitNotFound)
}

func (s *FileBasedStoreTestSuite) TestIsOrganizationUnitExists() {
	ou := OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
		Name:   "Test OU",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	assert.NoError(s.T(), err)

	// Test existing OU
	exists, err := s.store.IsOrganizationUnitExists(context.Background(), "test-ou-1")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)

	// Test non-existent OU
	exists, err = s.store.IsOrganizationUnitExists(context.Background(), "non-existent")
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestCheckOrganizationUnitHandleConflict() {
	ou := OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test-handle",
		Name:   "Test OU",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	assert.NoError(s.T(), err)

	// Check for conflict with same handle and parent
	conflict, err := s.store.CheckOrganizationUnitHandleConflict(context.Background(), "test-handle", nil)
	assert.NoError(s.T(), err)
	assert.True(s.T(), conflict)

	// No conflict with different handle
	conflict, err = s.store.CheckOrganizationUnitHandleConflict(context.Background(), "different-handle", nil)
	assert.NoError(s.T(), err)
	assert.False(s.T(), conflict)

	// Test with parent context
	parentID := testParentOUID
	child := OrganizationUnit{
		ID:     "child-1",
		Handle: "child-handle",
		Name:   "Child",
		Parent: &parentID,
	}
	err = s.store.CreateOrganizationUnit(context.Background(), child)
	assert.NoError(s.T(), err)

	conflict, err = s.store.CheckOrganizationUnitHandleConflict(context.Background(), "child-handle", &parentID)
	assert.NoError(s.T(), err)
	assert.True(s.T(), conflict)

	// Different parent, same handle should not conflict
	differentParent := "different-parent"
	conflict, err = s.store.CheckOrganizationUnitHandleConflict(context.Background(), "child-handle", &differentParent)
	assert.NoError(s.T(), err)
	assert.False(s.T(), conflict)
}

func (s *FileBasedStoreTestSuite) TestCheckOrganizationUnitNameConflict_WithParent() {
	parentID := testParentOUID
	parent := OrganizationUnit{
		ID:     parentID,
		Handle: "parent",
		Name:   "Parent",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), parent)
	assert.NoError(s.T(), err)

	child := OrganizationUnit{
		ID:     "child-1",
		Handle: "child",
		Name:   "Child Name",
		Parent: &parentID,
	}
	err = s.store.CreateOrganizationUnit(context.Background(), child)
	assert.NoError(s.T(), err)

	// Same name, same parent - should conflict
	conflict, err := s.store.CheckOrganizationUnitNameConflict(context.Background(), "Child Name", &parentID)
	assert.NoError(s.T(), err)
	assert.True(s.T(), conflict)

	// Same name, different parent - should not conflict
	differentParent := "different-parent"
	conflict, err = s.store.CheckOrganizationUnitNameConflict(context.Background(), "Child Name", &differentParent)
	assert.NoError(s.T(), err)
	assert.False(s.T(), conflict)

	// Same name, nil parent vs actual parent - should not conflict
	conflict, err = s.store.CheckOrganizationUnitNameConflict(context.Background(), "Child Name", nil)
	assert.NoError(s.T(), err)
	assert.False(s.T(), conflict)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitListCount() {
	// Initially empty
	count, err := s.store.GetOrganizationUnitListCount(context.Background(), nil)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)

	// Add root OUs
	root1 := OrganizationUnit{
		ID:     "root-1",
		Handle: "root1",
		Name:   "Root 1",
		Parent: nil,
	}
	root2 := OrganizationUnit{
		ID:     "root-2",
		Handle: "root2",
		Name:   "Root 2",
		Parent: nil,
	}
	err = s.store.CreateOrganizationUnit(context.Background(), root1)
	assert.NoError(s.T(), err)
	err = s.store.CreateOrganizationUnit(context.Background(), root2)
	assert.NoError(s.T(), err)

	// Should count only root OUs
	count, err = s.store.GetOrganizationUnitListCount(context.Background(), nil)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)

	// Add child OU (should not be counted)
	parentID := testRootOUID
	child := OrganizationUnit{
		ID:     "child-1",
		Handle: "child",
		Name:   "Child",
		Parent: &parentID,
	}
	err = s.store.CreateOrganizationUnit(context.Background(), child)
	assert.NoError(s.T(), err)

	// Count should still be 2
	count, err = s.store.GetOrganizationUnitListCount(context.Background(), nil)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitList_Pagination() {
	// Create multiple root OUs
	for i := 1; i <= 5; i++ {
		iStr := strconv.Itoa(i)
		ou := OrganizationUnit{
			ID:     "root-" + iStr,
			Handle: "root" + iStr,
			Name:   "Root " + iStr,
			Parent: nil,
		}
		err := s.store.CreateOrganizationUnit(context.Background(), ou)
		assert.NoError(s.T(), err)
	}

	// Test pagination - first page
	list, err := s.store.GetOrganizationUnitList(context.Background(), 2, 0, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), list, 2)

	// Test pagination - second page
	list, err = s.store.GetOrganizationUnitList(context.Background(), 2, 2, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), list, 2)

	// Test pagination - last page
	list, err = s.store.GetOrganizationUnitList(context.Background(), 2, 4, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), list, 1)

	// Test offset beyond range
	list, err = s.store.GetOrganizationUnitList(context.Background(), 10, 100, nil)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), list)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitChildrenList_Pagination() {
	// Create parent
	parent := OrganizationUnit{
		ID:     testParentOUID,
		Handle: "parent",
		Name:   "Parent",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), parent)
	assert.NoError(s.T(), err)

	// Create multiple children
	parentID := testParentOUID
	for i := 1; i <= 5; i++ {
		iStr := strconv.Itoa(i)
		child := OrganizationUnit{
			ID:     "child-" + iStr,
			Handle: "child" + iStr,
			Name:   "Child " + iStr,
			Parent: &parentID,
		}
		err := s.store.CreateOrganizationUnit(context.Background(), child)
		assert.NoError(s.T(), err)
	}

	// Test pagination
	children, err := s.store.GetOrganizationUnitChildrenList(context.Background(), testParentOUID, 2, 0, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), children, 2)

	children, err = s.store.GetOrganizationUnitChildrenList(context.Background(), testParentOUID, 2, 2, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), children, 2)

	// Test offset beyond range
	children, err = s.store.GetOrganizationUnitChildrenList(context.Background(), testParentOUID, 10, 100, nil)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), children)
}

func (s *FileBasedStoreTestSuite) TestCreate_StorerInterface() {
	ou := &OrganizationUnit{
		ID:     "test-ou-1",
		Handle: "test",
		Name:   "Test OU",
		Parent: nil,
	}

	// Test the Create method from Storer interface
	err := s.store.Create("test-ou-1", ou)
	assert.NoError(s.T(), err)

	// Verify it was created
	retrieved, err := s.store.GetOrganizationUnit(context.Background(), "test-ou-1")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), ou.ID, retrieved.ID)
}

func (s *FileBasedStoreTestSuite) TestCreateAndRetrieveWithDesignFields() {
	ou := OrganizationUnit{
		ID:       "design-ou-1",
		Handle:   "design-test",
		Name:     "Design Test OU",
		Parent:   nil,
		ThemeID:  "theme-123",
		LayoutID: "layout-456",
		LogoURL:  "https://example.com/logo.png",
	}

	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	assert.NoError(s.T(), err)

	// Verify design fields are preserved on retrieval
	retrieved, err := s.store.GetOrganizationUnit(context.Background(), "design-ou-1")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "theme-123", retrieved.ThemeID)
	assert.Equal(s.T(), "layout-456", retrieved.LayoutID)
	assert.Equal(s.T(), "https://example.com/logo.png", retrieved.LogoURL)
}

func (s *FileBasedStoreTestSuite) TestListIncludesDesignFields() {
	ou := OrganizationUnit{
		ID:       "design-list-1",
		Handle:   "design-list",
		Name:     "Design List OU",
		Parent:   nil,
		ThemeID:  "theme-abc",
		LayoutID: "layout-def",
		LogoURL:  "https://example.com/list-logo.png",
	}

	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	assert.NoError(s.T(), err)

	list, err := s.store.GetOrganizationUnitList(context.Background(), 10, 0, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), list, 1)
	assert.Equal(s.T(), "https://example.com/list-logo.png", list[0].LogoURL)
}

func (s *FileBasedStoreTestSuite) TestChildrenListIncludesDesignFields() {
	parentID := "design-parent"
	parent := OrganizationUnit{
		ID:     parentID,
		Handle: "parent",
		Name:   "Parent",
		Parent: nil,
	}
	child := OrganizationUnit{
		ID:       "design-child-1",
		Handle:   "child",
		Name:     "Child",
		Parent:   &parentID,
		ThemeID:  "child-theme",
		LayoutID: "child-layout",
		LogoURL:  "https://example.com/child-logo.png",
	}

	err := s.store.CreateOrganizationUnit(context.Background(), parent)
	assert.NoError(s.T(), err)
	err = s.store.CreateOrganizationUnit(context.Background(), child)
	assert.NoError(s.T(), err)

	children, err := s.store.GetOrganizationUnitChildrenList(context.Background(), parentID, 10, 0, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), children, 1)
	assert.Equal(s.T(), "https://example.com/child-logo.png", children[0].LogoURL)
}

func (s *FileBasedStoreTestSuite) TestNewFileBasedStore() {
	// Test that newFileBasedStore creates a valid store
	store, _ := newFileBasedStore()
	assert.NotNil(s.T(), store)

	// Verify it implements the interface by using it
	fbStore, ok := store.(*fileBasedStore)
	assert.True(s.T(), ok)
	assert.NotNil(s.T(), fbStore.GenericFileBasedStore)
}

func (s *FileBasedStoreTestSuite) TestFileBasedStore_GetOrganizationUnitsByIDs() {
	// Create some OUs
	ou1 := OrganizationUnit{
		ID:     "ou-1",
		Handle: "handle-1",
		Name:   "Name 1",
		Parent: nil,
	}
	ou2 := OrganizationUnit{
		ID:     "ou-2",
		Handle: "handle-2",
		Name:   "Name 2",
		Parent: nil,
	}
	ou3 := OrganizationUnit{
		ID:     "ou-3",
		Handle: "handle-3",
		Name:   "Name 3",
		Parent: nil,
	}

	err := s.store.CreateOrganizationUnit(context.Background(), ou1)
	s.Require().NoError(err)
	err = s.store.CreateOrganizationUnit(context.Background(), ou2)
	s.Require().NoError(err)
	err = s.store.CreateOrganizationUnit(context.Background(), ou3)
	s.Require().NoError(err)

	// Test empty ids
	result, err := s.store.GetOrganizationUnitsByIDs(context.Background(), []string{})
	s.Require().NoError(err)
	s.Require().Empty(result)

	// Test existing ids
	result, err = s.store.GetOrganizationUnitsByIDs(context.Background(), []string{"ou-1", "ou-3"})
	s.Require().NoError(err)
	s.Require().Len(result, 2)

	validIDs := map[string]bool{"ou-1": true, "ou-3": true}
	for _, ou := range result {
		s.Require().True(validIDs[ou.ID])
	}

	// Test partial matching ids
	result, err = s.store.GetOrganizationUnitsByIDs(context.Background(), []string{"ou-2", "non-existent"})
	s.Require().NoError(err)
	s.Require().Len(result, 1)
	s.Require().Equal("ou-2", result[0].ID)
}

func (s *FileBasedStoreTestSuite) TestFileBasedStore_IsOrganizationUnitDeclarative() {
	// Create an OU
	ou := OrganizationUnit{
		ID:     "ou-decl",
		Handle: "handle-decl",
		Name:   "Name Decl",
		Parent: nil,
	}
	err := s.store.CreateOrganizationUnit(context.Background(), ou)
	s.Require().NoError(err)

	// Test existing OU
	isDecl := s.store.IsOrganizationUnitDeclarative(context.Background(), "ou-decl")
	s.Require().True(isDecl)

	// Test non-existent OU
	isDecl = s.store.IsOrganizationUnitDeclarative(context.Background(), "non-existent")
	s.Require().False(isDecl)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnit_CorruptedData() {
	err := s.store.GenericFileBasedStore.Create("bad-ou", "not-an-ou")
	s.Require().NoError(err)

	_, err = s.store.GetOrganizationUnit(context.Background(), "bad-ou")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "organization unit data corrupted")
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitByHandle() {
	root := OrganizationUnit{ID: "root-1", Handle: "root", Name: "Root", Parent: nil}
	rootID := testRootOUID
	child := OrganizationUnit{ID: "child-1", Handle: "child", Name: "Child", Parent: &rootID}

	err := s.store.CreateOrganizationUnit(context.Background(), root)
	s.Require().NoError(err)
	err = s.store.CreateOrganizationUnit(context.Background(), child)
	s.Require().NoError(err)

	ou, err := s.store.GetOrganizationUnitByHandle(context.Background(), "root", nil)
	s.Require().NoError(err)
	s.Require().Equal("root-1", ou.ID)

	ou, err = s.store.GetOrganizationUnitByHandle(context.Background(), "child", &rootID)
	s.Require().NoError(err)
	s.Require().Equal("child-1", ou.ID)

	_, err = s.store.GetOrganizationUnitByHandle(context.Background(), "child", nil)
	s.Require().ErrorIs(err, ErrOrganizationUnitNotFound)
}

func (s *FileBasedStoreTestSuite) TestGetOrganizationUnitByPath_EmptyPath() {
	_, err := s.store.GetOrganizationUnitByPath(context.Background(), []string{})
	s.Require().ErrorIs(err, ErrOrganizationUnitNotFound)
}

// singleFilterGroup builds a one-clause FilterGroup for test brevity.
func singleFilterGroup(attr string, op filter.Operator, val interface{}) *filter.FilterGroup {
	return &filter.FilterGroup{Clauses: []filter.FilterClause{
		{Expr: filter.FilterExpression{Attribute: attr, Operator: op, Value: val}},
	}}
}

func TestMatchesOUFilter(t *testing.T) {
	baseTime := time.Date(2025, time.January, 1, 10, 0, 0, 0, time.UTC)
	ou := &OrganizationUnit{
		ID:          "ou-1",
		Handle:      "finance",
		Name:        "Finance",
		Description: "Finance OU",
		CreatedAt:   baseTime,
		UpdatedAt:   baseTime.Add(2 * time.Hour),
	}

	tests := []struct {
		name string
		f    *filter.FilterGroup
		want bool
	}{
		{
			name: "nil filter",
			f:    nil,
			want: true,
		},
		{
			name: "name eq case insensitive",
			f:    singleFilterGroup("name", filter.OperatorEq, "finance"),
			want: true,
		},
		{
			name: "handle eq",
			f:    singleFilterGroup("handle", filter.OperatorEq, "finance"),
			want: true,
		},
		{
			name: "description eq",
			f:    singleFilterGroup("description", filter.OperatorEq, "Finance OU"),
			want: true,
		},
		{
			name: "createdAt gt",
			f:    singleFilterGroup("createdAt", filter.OperatorGt, "2025-01-01T09:59:59Z"),
			want: true,
		},
		{
			name: "updatedAt lt",
			f:    singleFilterGroup("updatedAt", filter.OperatorLt, "2025-01-01T12:00:01Z"),
			want: true,
		},
		{
			name: "unknown attribute",
			f:    singleFilterGroup("id", filter.OperatorEq, "ou-1"),
			want: false,
		},
		{
			name: "non string value",
			f: &filter.FilterGroup{Clauses: []filter.FilterClause{
				{Expr: filter.FilterExpression{Attribute: "name", Operator: filter.OperatorEq, Value: 10}},
			}},
			want: false,
		},
		{
			name: "unsupported operator",
			f:    singleFilterGroup("name", filter.Operator("co"), "Finance"),
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, matchesOUFilter(ou, tc.f))
		})
	}
}
