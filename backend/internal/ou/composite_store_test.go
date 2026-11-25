// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// CompositeStoreTestSuite tests the composite OU store functionality.
type CompositeStoreTestSuite struct {
	suite.Suite
	fileStore      organizationUnitStoreInterface
	dbStoreMock    *organizationUnitStoreInterfaceMock
	compositeStore *compositeOUStore
}

// SetupTest sets up the test environment.
func (suite *CompositeStoreTestSuite) SetupTest() {
	// Clear the singleton entity store to avoid state leakage between tests
	_ = entity.GetInstance().Clear()

	// Create NEW file-based store for each test to avoid state leakage
	suite.fileStore, _ = newFileBasedStore()

	// Create mock DB store
	suite.dbStoreMock = newOrganizationUnitStoreInterfaceMock(suite.T())

	// Create composite store
	suite.compositeStore = newCompositeOUStore(suite.fileStore, suite.dbStoreMock)
}

// TestCompositeStore_GetOrganizationUnit tests retrieving OUs from composite store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_GetOrganizationUnit() {
	testCases := []struct {
		name           string
		ouID           string
		setupFileStore func()
		setupDBStore   func()
		want           OrganizationUnit
		wantErr        bool
	}{
		{
			name: "retrieves from DB store - marks as mutable",
			ouID: "db-ou-1",
			setupFileStore: func() {
				// File store doesn't have this OU
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetOrganizationUnit", mock.Anything, "db-ou-1").
					Return(OrganizationUnit{
						ID:     "db-ou-1",
						Handle: "db-handle",
						Name:   "DB OU",
					}, nil).
					Once()
			},
			want: OrganizationUnit{
				ID:     "db-ou-1",
				Handle: "db-handle",
				Name:   "DB OU",
			},
		},
		{
			name: "retrieves from file store - marks as immutable",
			ouID: "file-ou-1",
			setupFileStore: func() {
				// Add OU to file store
				err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
					ID:     "file-ou-1",
					Handle: "file-handle",
					Name:   "File OU",
				})
				suite.NoError(err)
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetOrganizationUnit", mock.Anything, "file-ou-1").
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
					Once()
			},
			want: OrganizationUnit{
				ID:     "file-ou-1",
				Handle: "file-handle",
				Name:   "File OU",
			},
		},
		{
			name: "not found in either store",
			ouID: "nonexistent",
			setupFileStore: func() {
				// OU not in file store
			},
			setupDBStore: func() {
				suite.dbStoreMock.On("GetOrganizationUnit", mock.Anything, "nonexistent").
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
					Once()
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // Fresh setup for each test
			tc.setupFileStore()
			tc.setupDBStore()

			got, err := suite.compositeStore.GetOrganizationUnit(context.Background(), tc.ouID)

			if tc.wantErr {
				suite.Error(err)
				suite.Equal(ErrOrganizationUnitNotFound, err)
			} else {
				suite.NoError(err)
				suite.Equal(tc.want.ID, got.ID)
				suite.Equal(tc.want.Handle, got.Handle)
				suite.Equal(tc.want.Name, got.Name)
			}
		})
	}
}

// TestCompositeStore_CreateOrganizationUnit tests creating OUs.
func (suite *CompositeStoreTestSuite) TestCompositeStore_CreateOrganizationUnit() {
	suite.Run("creates in DB store", func() {
		ou := OrganizationUnit{
			ID:     "new-ou-1",
			Handle: "new-handle",
			Name:   "New OU",
		}

		suite.dbStoreMock.On("CreateOrganizationUnit", mock.Anything, ou).
			Return(nil).
			Once()

		err := suite.compositeStore.CreateOrganizationUnit(context.Background(), ou)
		suite.NoError(err)
	})
}

// TestCompositeStore_UpdateOrganizationUnit tests updating OUs.
func (suite *CompositeStoreTestSuite) TestCompositeStore_UpdateOrganizationUnit() {
	suite.Run("updates DB OU successfully", func() {
		ou := OrganizationUnit{
			ID:     "db-ou-1",
			Handle: "updated-handle",
			Name:   "Updated OU",
		}

		suite.dbStoreMock.On("UpdateOrganizationUnit", mock.Anything, ou).
			Return(nil).
			Once()

		err := suite.compositeStore.UpdateOrganizationUnit(context.Background(), ou)
		suite.NoError(err)
	})
}

// TestCompositeStore_DeleteOrganizationUnit tests deleting OUs.
func (suite *CompositeStoreTestSuite) TestCompositeStore_DeleteOrganizationUnit() {
	suite.Run("deletes DB OU successfully", func() {
		// Children validation is done at service layer, not store layer
		suite.dbStoreMock.On("DeleteOrganizationUnit", mock.Anything, "db-ou-1").
			Return(nil).
			Once()

		err := suite.compositeStore.DeleteOrganizationUnit(context.Background(), "db-ou-1")
		suite.NoError(err)
	})
}

// TestCompositeStore_IsOrganizationUnitExists tests existence checks across both stores.
func (suite *CompositeStoreTestSuite) TestCompositeStore_IsOrganizationUnitExists() {
	suite.Run("exists in DB store", func() {
		suite.dbStoreMock.On("IsOrganizationUnitExists", mock.Anything, "db-ou-1").
			Return(true, nil).
			Once()

		exists, err := suite.compositeStore.IsOrganizationUnitExists(context.Background(), "db-ou-1")
		suite.NoError(err)
		suite.True(exists)
	})

	suite.Run("exists in file store", func() {
		// Add OU to file store
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-ou-1",
			Handle: "file-handle",
			Name:   "File OU",
		})
		suite.NoError(err)

		exists, err := suite.compositeStore.IsOrganizationUnitExists(context.Background(), "file-ou-1")
		suite.NoError(err)
		suite.True(exists)
	})

	suite.Run("not found in either store", func() {
		suite.dbStoreMock.On("IsOrganizationUnitExists", mock.Anything, "nonexistent").
			Return(false, nil).
			Once()

		exists, err := suite.compositeStore.IsOrganizationUnitExists(context.Background(), "nonexistent")
		suite.NoError(err)
		suite.False(exists)
	})
}

// TestCompositeStore_ConflictChecks tests name and handle conflict detection across both stores.
func (suite *CompositeStoreTestSuite) TestCompositeStore_ConflictChecks() {
	suite.Run("detects name conflict in DB store", func() {
		// File store checked first - returns false (no conflict)
		// Then DB store checked - returns true (conflict)
		suite.dbStoreMock.On("CheckOrganizationUnitNameConflict", mock.Anything, "test-name", (*string)(nil)).
			Return(true, nil).
			Once()

		conflict, err := suite.compositeStore.CheckOrganizationUnitNameConflict(context.Background(), "test-name", nil)
		suite.NoError(err)
		suite.True(conflict)
	})

	suite.Run("detects name conflict in file store", func() {
		// Add OU to file store
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-ou-1",
			Handle: "file-handle",
			Name:   "Conflict Name",
		})
		suite.NoError(err)

		// File store checked first - returns true (conflict found)
		// DB store not called since file store found conflict
		// No mock expectation for dbStore

		conflict, err := suite.compositeStore.CheckOrganizationUnitNameConflict(
			context.Background(), "Conflict Name", nil,
		)
		suite.NoError(err)
		suite.True(conflict)
	})

	suite.Run("no name conflict in either store", func() {
		suite.dbStoreMock.On("CheckOrganizationUnitNameConflict", mock.Anything, "unique-name", (*string)(nil)).
			Return(false, nil).
			Once()

		conflict, err := suite.compositeStore.CheckOrganizationUnitNameConflict(
			context.Background(), "unique-name", nil,
		)
		suite.NoError(err)
		suite.False(conflict)
	})
}

// TestCompositeStore_ChildrenOperations tests child OU operations across both stores.
func (suite *CompositeStoreTestSuite) TestCompositeStore_ChildrenOperations() {
	suite.Run("counts children from both stores", func() {
		suite.SetupTest() // Fresh setup
		parentID := testCoverageParentOUID

		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(2, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"file-child-1"}).
			Return([]OrganizationUnitBasic{}, nil).
			Once()

		// Add parent and child to file store
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     parentID,
			Handle: "parent",
			Name:   "Parent",
		})
		suite.NoError(err)
		err = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-child-1",
			Handle: "child1",
			Name:   "Child 1",
			Parent: &parentID,
		})
		suite.NoError(err)

		count, err := suite.compositeStore.GetOrganizationUnitChildrenCount(context.Background(), parentID, nil)
		suite.NoError(err)
		suite.Equal(3, count) // 2 from DB + 1 from file
	})
}

// TestCompositeStore_ListOperations tests list operations include both stores.
func (suite *CompositeStoreTestSuite) TestCompositeStore_ListOperations() {
	suite.Run("GetOrganizationUnitListCount returns count from both stores", func() {
		// Add OUs to file store
		_ = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID: "file-ou-1", Handle: "file-1", Name: "File OU 1",
		})
		_ = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID: "file-ou-2", Handle: "file-2", Name: "File OU 2",
		})

		// Mock DB store count - CompositeMergeCountHelper calls dbStore.GetOrganizationUnitListCount once
		suite.dbStoreMock.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).Return(3, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, mock.Anything).
			Return([]OrganizationUnitBasic{}, nil).Once()

		count, err := suite.compositeStore.GetOrganizationUnitListCount(context.Background(), nil)
		suite.NoError(err)
		suite.Equal(5, count) // 3 from DB + 2 from file
	})

	suite.Run("GetOrganizationUnitList returns merged results", func() {
		// Reset state from previous sub-test by reinitializing stores
		_ = entity.GetInstance().Clear()
		suite.fileStore, _ = newFileBasedStore()
		suite.compositeStore = newCompositeOUStore(suite.fileStore, suite.dbStoreMock)

		// Add 1 OU to file store
		_ = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID: "file-ou-1", Handle: "file-1", Name: "File OU 1",
		})

		// Mock DB store - note that implementation calls count to determine how many to fetch from each
		// dbCount will be called to fetch all from DB
		suite.dbStoreMock.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).Return(2, nil).Once()
		// Then fileCount from fileStore (real store) returns 1
		// Total = 3, so it will try to fetch all 3 (dbCount=2, fileCount=1)
		suite.dbStoreMock.On("GetOrganizationUnitList", mock.Anything, 2, 0, mock.Anything).
			Return([]OrganizationUnitBasic{
				{ID: "db-ou-1", Handle: "db-1", Name: "DB OU 1"},
				{ID: "db-ou-2", Handle: "db-2", Name: "DB OU 2"},
			}, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"file-ou-1"}).
			Return([]OrganizationUnitBasic{}, nil).Once()

		list, err := suite.compositeStore.GetOrganizationUnitList(context.Background(), 10, 0, nil)
		suite.NoError(err)
		suite.Len(list, 3) // 2 from DB + 1 from file

		// Verify all OUs are present and verify IsReadOnly flags
		ids := make(map[string]bool)
		for _, ou := range list {
			ids[ou.ID] = true
			// Verify IsReadOnly flags are correct
			switch ou.ID {
			case "db-ou-1", "db-ou-2":
				suite.False(ou.IsReadOnly, "DB OU %s should have IsReadOnly=false", ou.ID)
			case "file-ou-1":
				suite.True(ou.IsReadOnly, "File OU %s should have IsReadOnly=true", ou.ID)
			}
		}
		suite.True(ids["db-ou-1"])
		suite.True(ids["db-ou-2"])
		suite.True(ids["file-ou-1"])
	})

	suite.Run("GetOrganizationUnitList serves a page past the composite store's limit", func() {
		suite.SetupTest()
		suite.dbStoreMock.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).
			Return(serverconst.MaxCompositeStoreRecords+500, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitList", mock.Anything, 30, 1020, mock.Anything).
			Return(make([]OrganizationUnitBasic, 30), nil).Once()

		result, err := suite.compositeStore.GetOrganizationUnitList(context.Background(), 30, 1020, nil)
		suite.NoError(err)
		suite.Len(result, 30)
	})

	suite.Run("root list drops a file root unit the database also holds and keeps the file store order", func() {
		suite.SetupTest()
		ctx := context.Background()
		for _, ou := range []OrganizationUnit{
			{ID: "file-z", Handle: "z", Name: "Alpha"},
			{ID: "shared", Handle: "shared", Name: "Shared"},
			{ID: "file-a", Handle: "a", Name: "Zeta"},
		} {
			suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, ou))
		}
		// A database row that shares an ID but sits under a parent is not a root duplicate.
		other := testOtherParentOUID
		suite.dbStoreMock.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).Return(1, nil).Twice()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, mock.Anything).
			Return([]OrganizationUnitBasic{{ID: "shared"}, {ID: "file-z", Parent: &other}}, nil).Twice()
		suite.dbStoreMock.On("GetOrganizationUnitList", mock.Anything, 1, 0, mock.Anything).
			Return([]OrganizationUnitBasic{{ID: "shared", Name: "Shared"}}, nil).Once()

		count, err := suite.compositeStore.GetOrganizationUnitListCount(ctx, nil)
		suite.NoError(err)
		suite.Equal(3, count) // database "shared", then the file units "file-a" and "file-z"

		page, err := suite.compositeStore.GetOrganizationUnitList(ctx, 10, 0, nil)
		suite.NoError(err)
		suite.Require().Len(page, 3)
		// The file units keep the file store's ID order, not name order: "file-a" is named Zeta, "file-z" Alpha.
		suite.Equal([]string{"shared", "file-a", "file-z"}, []string{page[0].ID, page[1].ID, page[2].ID})
		suite.False(page[0].IsReadOnly)
		suite.True(page[1].IsReadOnly)
	})
}

// TestCompositeStore_IsOrganizationUnitDeclarative tests checking if an OU is immutable.
func (suite *CompositeStoreTestSuite) TestCompositeStore_IsOrganizationUnitDeclarative() {
	suite.Run("returns true for immutable OU (exists in file store)", func() {
		// Add OU to file store
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "immutable-ou-1",
			Handle: "immutable-handle",
			Name:   "Declarative OU",
		})
		suite.NoError(err)

		isDeclarative := suite.compositeStore.IsOrganizationUnitDeclarative(context.Background(), "immutable-ou-1")
		suite.True(isDeclarative)
	})

	suite.Run("returns false for mutable OU (not in file store)", func() {
		isDeclarative := suite.compositeStore.IsOrganizationUnitDeclarative(context.Background(), "db-ou-1")
		suite.False(isDeclarative)
	})

	suite.Run("returns false for non-existent OU", func() {
		isDeclarative := suite.compositeStore.IsOrganizationUnitDeclarative(context.Background(), "nonexistent")
		suite.False(isDeclarative)
	})
}

// TestCompositeStoreTestSuite runs the test suite.
func TestCompositeStoreTestSuite(t *testing.T) {
	// Initialize entity store for file-based store
	_ = entity.GetInstance()
	suite.Run(t, new(CompositeStoreTestSuite))
}

const (
	testCoverageParentOUID = "parent-ou"
	testCoverageChild1ID   = "child-1"
	testOtherParentOUID    = "other-parent"
)

// CompositeStoreCoverageTestSuite provides additional coverage tests for composite store.
type CompositeStoreCoverageTestSuite struct {
	suite.Suite
	fileStore      organizationUnitStoreInterface
	dbStoreMock    *organizationUnitStoreInterfaceMock
	compositeStore *compositeOUStore
}

func (suite *CompositeStoreCoverageTestSuite) SetupTest() {
	// Clear the singleton entity store to avoid state leakage between tests
	_ = entity.GetInstance().Clear()

	// Create NEW file-based store for each test to avoid state leakage
	suite.fileStore, _ = newFileBasedStore()
	suite.dbStoreMock = newOrganizationUnitStoreInterfaceMock(suite.T())
	suite.compositeStore = newCompositeOUStore(suite.fileStore, suite.dbStoreMock)
}

// TestCompositeStore_GetOrganizationUnitList tests paginated list retrieval.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_GetOrganizationUnitList() {
	suite.Run("retrieves paginated list from DB store only", func() {
		expectedList := []OrganizationUnitBasic{
			{ID: "ou-1", Handle: "handle-1", Name: "OU 1"},
			{ID: "ou-2", Handle: "handle-2", Name: "OU 2"},
		}

		suite.dbStoreMock.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).
			Return(2, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitList", mock.Anything, 2, 0, mock.Anything).
			Return(expectedList, nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitList(context.Background(), 10, 0, nil)
		suite.NoError(err)
		suite.Equal(expectedList, result)
	})

	suite.Run("propagates DB store error", func() {
		dbErr := errors.New("database error")
		suite.dbStoreMock.On("GetOrganizationUnitListCount", mock.Anything, mock.Anything).
			Return(0, dbErr).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitList(context.Background(), 10, 0, nil)
		suite.Error(err)
		suite.Equal(dbErr, err)
		suite.Empty(result)
	})
}

// TestCompositeStore_GetOrganizationUnitByPath tests path-based retrieval.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_GetOrganizationUnitByPath() {
	handles := []string{"level1", "level2"}

	suite.Run("retrieves from DB store by path", func() {
		expectedOU := OrganizationUnit{
			ID:     "ou-path-1",
			Handle: "level2",
			Name:   "Level 2 OU",
		}

		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "level1", (*string)(nil)).
			Return(OrganizationUnit{ID: "db-root", Handle: "level1", Name: "Root"}, nil).
			Once()
		parentID := "db-root"
		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "level2", &parentID).
			Return(expectedOU, nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitByPath(context.Background(), handles)
		suite.NoError(err)
		suite.Equal(expectedOU, result)
	})

	suite.Run("falls back to file store when not in DB", func() {
		suite.SetupTest() // Fresh setup

		// Create parent and child in file store
		parentID := "file-parent"
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     parentID,
			Handle: "level1",
			Name:   "Parent",
		})
		suite.NoError(err)

		err = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-child",
			Handle: "level2",
			Name:   "Child",
			Parent: &parentID,
		})
		suite.NoError(err)

		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "level1", (*string)(nil)).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "level2", &parentID).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitByPath(context.Background(), handles)
		suite.NoError(err)
		suite.Equal("file-child", result.ID)
		suite.Equal("level2", result.Handle)
	})

	suite.Run("resolves mixed ancestry file parent and DB child", func() {
		suite.SetupTest() // Fresh setup

		// Root in file store
		rootID := "file-root"
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     rootID,
			Handle: "default",
			Name:   "Default",
		})
		suite.NoError(err)

		// Child in DB store under file root
		dbChild := OrganizationUnit{
			ID:     "db-child",
			Handle: "thin-ends-make",
			Name:   "Thin Ends Make",
			Parent: &rootID,
		}
		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "default", (*string)(nil)).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "thin-ends-make", &rootID).
			Return(dbChild, nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitByPath(
			context.Background(), []string{"default", "thin-ends-make"},
		)
		suite.NoError(err)
		suite.Equal("db-child", result.ID)
		suite.Equal("thin-ends-make", result.Handle)
	})

	suite.Run("returns not found when in neither store", func() {
		suite.SetupTest() // Fresh setup
		handlesTest := []string{"nonexistent1", "nonexistent2"}

		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "nonexistent1", (*string)(nil)).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitByPath(context.Background(), handlesTest)
		suite.Error(err)
		suite.Equal(ErrOrganizationUnitNotFound, err)
		suite.Empty(result.ID)
	})

	suite.Run("propagates DB errors other than not found", func() {
		dbErr := errors.New("database connection error")
		suite.dbStoreMock.On("GetOrganizationUnitByHandle", mock.Anything, "level1", (*string)(nil)).
			Return(OrganizationUnit{}, dbErr).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitByPath(context.Background(), handles)
		suite.Error(err)
		suite.Equal(dbErr, err)
		suite.Empty(result.ID)
	})
}

// TestCompositeStore_CheckOrganizationUnitHandleConflict tests handle conflict detection.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_CheckOrganizationUnitHandleConflict() {
	suite.Run("detects handle conflict in DB store", func() {
		// File store checked first - returns false (no conflict)
		// Then DB store checked - returns true (conflict)
		suite.dbStoreMock.On("CheckOrganizationUnitHandleConflict", mock.Anything, "test-handle", (*string)(nil)).
			Return(true, nil).
			Once()

		conflict, err := suite.compositeStore.CheckOrganizationUnitHandleConflict(
			context.Background(), "test-handle", nil,
		)
		suite.NoError(err)
		suite.True(conflict)
	})

	suite.Run("detects handle conflict in file store", func() {
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-ou",
			Handle: "conflict-handle",
			Name:   "File OU",
		})
		suite.NoError(err)

		// File store checked first - returns true (conflict found)
		// DB store not called since file store found conflict
		// No mock expectation for dbStore

		conflict, err := suite.compositeStore.CheckOrganizationUnitHandleConflict(
			context.Background(), "conflict-handle", nil,
		)
		suite.NoError(err)
		suite.True(conflict)
	})

	suite.Run("no handle conflict in either store", func() {
		suite.dbStoreMock.On("CheckOrganizationUnitHandleConflict", mock.Anything, "unique-handle", (*string)(nil)).
			Return(false, nil).
			Once()

		conflict, err := suite.compositeStore.CheckOrganizationUnitHandleConflict(
			context.Background(), "unique-handle", nil,
		)
		suite.NoError(err)
		suite.False(conflict)
	})

	suite.Run("propagates DB error", func() {
		dbErr := errors.New("db error")
		suite.dbStoreMock.On("CheckOrganizationUnitHandleConflict", mock.Anything, "test", (*string)(nil)).
			Return(false, dbErr).
			Once()

		conflict, err := suite.compositeStore.CheckOrganizationUnitHandleConflict(context.Background(), "test", nil)
		suite.Error(err)
		suite.Equal(dbErr, err)
		suite.False(conflict)
	})
}

func TestCompositeStoreCoverageTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreCoverageTestSuite))
}

// TestCompositeStore_GetOrganizationUnitChildrenList tests paginated children retrieval.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_GetOrganizationUnitChildrenList() {
	parentID := testCoverageParentOUID

	suite.Run("merges children from both stores with pagination", func() {
		// Setup DB children
		dbChildren := []OrganizationUnitBasic{
			{ID: "db-child-1", Handle: "db1", Name: "DB Child 1"},
			{ID: "db-child-2", Handle: "db2", Name: "DB Child 2"},
		}

		// Setup file store parent and children
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     parentID,
			Handle: "parent",
			Name:   "Parent",
		})
		suite.NoError(err)
		err = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-child-1",
			Handle: "file1",
			Name:   "File Child 1",
			Parent: &parentID,
		})
		suite.NoError(err)

		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(2, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, parentID, 2, 0, mock.Anything).
			Return(dbChildren, nil).
			Once()

		// Request first 2 items
		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(context.Background(), parentID, 2, 0, nil)
		suite.NoError(err)
		suite.Len(result, 2)
	})

	suite.Run("handles offset beyond total count", func() {
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(2, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"file-child-1"}).
			Return([]OrganizationUnitBasic{}, nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), parentID, 10, 100, nil,
		)
		suite.NoError(err)
		suite.Empty(result)
	})

	suite.Run("handles pagination with offset", func() {
		suite.SetupTest() // Fresh setup
		testParentID := "pagination-parent"
		dbChildren := []OrganizationUnitBasic{
			{ID: "db-child-1", Handle: "db1", Name: "DB Child 1"},
			{ID: "db-child-2", Handle: "db2", Name: "DB Child 2"},
			{ID: "db-child-3", Handle: "db3", Name: "DB Child 3"},
		}

		// Setup file store parent
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     testParentID,
			Handle: "parent",
			Name:   "Parent",
		})
		suite.NoError(err)

		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, testParentID, mock.Anything).
			Return(3, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, testParentID, 1, 2, mock.Anything).
			Return(dbChildren[2:], nil).
			Once()

		// Get second page (offset=2, limit=2)
		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), testParentID, 2, 2, nil,
		)
		suite.NoError(err)
		suite.Len(result, 1) // Only 1 item remaining
		suite.Equal("db-child-3", result[0].ID)
	})

	suite.Run("propagates DB count error", func() {
		suite.SetupTest() // Fresh setup
		testParentID := "count-error-parent"
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     testParentID,
			Handle: "parent",
			Name:   "Parent",
		})
		suite.NoError(err)

		dbErr := errors.New("count error")
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, testParentID, mock.Anything).
			Return(0, dbErr).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), testParentID, 10, 0, nil,
		)
		suite.Error(err)
		suite.Equal(dbErr, err)
		suite.Nil(result)
	})

	suite.Run("propagates DB list error", func() {
		suite.SetupTest() // Fresh setup
		testParentID := "error-parent"
		// Create parent in file store
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     testParentID,
			Handle: "parent",
			Name:   "Parent",
		})
		suite.NoError(err)

		dbErr := errors.New("list error")
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, testParentID, mock.Anything).
			Return(2, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, testParentID, 2, 0, mock.Anything).
			Return([]OrganizationUnitBasic{}, dbErr).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), testParentID, 10, 0, nil,
		)
		suite.Error(err)
		suite.Equal(dbErr, err)
		suite.Nil(result)
	})

	suite.Run("serves a small page when the parent has more children than the composite store's limit", func() {
		suite.SetupTest()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, "big-parent", mock.Anything).
			Return(serverconst.MaxCompositeStoreRecords+1, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, "big-parent", 1, 0, mock.Anything).
			Return([]OrganizationUnitBasic{{ID: "child-1", Handle: "c1", Name: "Child 1"}}, nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), "big-parent", 1, 0, nil,
		)
		suite.NoError(err)
		suite.Len(result, 1)
		suite.Equal("child-1", result[0].ID)
	})

	suite.Run("serves a page past the composite store's limit", func() {
		suite.SetupTest()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, "big-parent", mock.Anything).
			Return(serverconst.MaxCompositeStoreRecords+500, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, "big-parent", 30, 1020, mock.Anything).
			Return(make([]OrganizationUnitBasic, 30), nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), "big-parent", 30, 1020, nil,
		)
		suite.NoError(err)
		suite.Len(result, 30)
	})

	suite.Run("lists database children before file children across a page", func() {
		suite.SetupTest()
		boundaryParentID := "boundary-parent"
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID: boundaryParentID, Handle: "parent", Name: "Parent",
		})
		suite.NoError(err)
		err = suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID: "file-child", Handle: "file-handle", Name: "File Child", Parent: &boundaryParentID,
		})
		suite.NoError(err)

		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, boundaryParentID, mock.Anything).
			Return(2, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, boundaryParentID, 1, 1, mock.Anything).
			Return([]OrganizationUnitBasic{{ID: "db-child-2", Handle: "db2", Name: "DB Child 2"}}, nil).
			Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"file-child"}).
			Return([]OrganizationUnitBasic{}, nil).
			Once()

		// offset=1, limit=2: the last database child, then the only file child
		result, err := suite.compositeStore.GetOrganizationUnitChildrenList(
			context.Background(), boundaryParentID, 2, 1, nil,
		)
		suite.NoError(err)
		suite.Require().Len(result, 2)
		suite.Equal("db-child-2", result[0].ID)
		suite.False(result[0].IsReadOnly)
		suite.Equal("file-child", result[1].ID)
		suite.True(result[1].IsReadOnly)
	})
}

// TestPageDatabaseThenFile tests that a page is read from the position it falls in across both stores.
func (suite *CompositeStoreCoverageTestSuite) TestPageDatabaseThenFile() {
	type call struct{ limit, offset int }

	run := func(dbCount, fileCount, limit, offset int) ([]OrganizationUnitBasic, *call, bool, error) {
		var dbCall *call
		fileRead := false
		result, err := pageDatabaseThenFile(limit, offset,
			func() (int, error) { return dbCount, nil },
			func(l, o int) ([]OrganizationUnitBasic, error) {
				dbCall = &call{l, o}
				items := make([]OrganizationUnitBasic, 0, l)
				for i := o; i < min(o+l, dbCount); i++ {
					items = append(items, OrganizationUnitBasic{ID: fmt.Sprintf("db-%d", i)})
				}
				return items, nil
			},
			func() ([]OrganizationUnitBasic, error) {
				fileRead = true
				items := make([]OrganizationUnitBasic, 0, fileCount)
				for i := 0; i < fileCount; i++ {
					items = append(items, OrganizationUnitBasic{ID: fmt.Sprintf("file-%d", i)})
				}
				return items, nil
			})
		return result, dbCall, fileRead, err
	}

	suite.Run("page within the database reads only the database", func() {
		result, dbCall, fileRead, err := run(1000, 1000, 30, 100)
		suite.NoError(err)
		suite.Len(result, 30)
		suite.Equal(&call{30, 100}, dbCall)
		suite.False(fileRead)
	})

	suite.Run("page across the boundary reads the end of the database and the start of the file children", func() {
		result, dbCall, fileRead, err := run(1000, 1000, 30, 990)
		suite.NoError(err)
		suite.Require().Len(result, 30)
		suite.Equal(&call{10, 990}, dbCall)
		suite.True(fileRead)
		suite.Equal("db-990", result[0].ID)
		suite.False(result[0].IsReadOnly)
		suite.Equal("file-0", result[10].ID)
		suite.True(result[10].IsReadOnly)
	})

	suite.Run("page past the database reads only the file children", func() {
		result, dbCall, fileRead, err := run(900, 1000, 30, 990)
		suite.NoError(err)
		suite.Require().Len(result, 30)
		suite.Nil(dbCall)
		suite.True(fileRead)
		suite.Equal("file-90", result[0].ID)
	})

	suite.Run("page running off the end returns what is left", func() {
		result, _, _, err := run(10, 5, 30, 12)
		suite.NoError(err)
		suite.Len(result, 3)
	})

	suite.Run("offset beyond the total returns an empty page", func() {
		result, _, _, err := run(10, 5, 30, 15)
		suite.NoError(err)
		suite.Empty(result)
	})

	suite.Run("a limit of zero returns an empty page without reading any items", func() {
		result, dbCall, fileRead, err := run(10, 5, 0, 0)
		suite.NoError(err)
		suite.NotNil(result)
		suite.Empty(result)
		suite.Nil(dbCall)
		suite.False(fileRead)
	})

	suite.Run("propagates count, list and file children errors", func() {
		boom := errors.New("boom")
		okCount := func() (int, error) { return 1, nil }
		failCount := func() (int, error) { return 0, boom }
		okList := func(int, int) ([]OrganizationUnitBasic, error) { return nil, nil }
		failList := func(int, int) ([]OrganizationUnitBasic, error) { return nil, boom }
		okFiles := func() ([]OrganizationUnitBasic, error) { return nil, nil }
		failFiles := func() ([]OrganizationUnitBasic, error) { return nil, boom }

		_, err := pageDatabaseThenFile(10, 0, failCount, okList, okFiles)
		suite.ErrorIs(err, boom)
		_, err = pageDatabaseThenFile(10, 0, okCount, failList, okFiles)
		suite.ErrorIs(err, boom)
		_, err = pageDatabaseThenFile(10, 1, okCount, okList, failFiles)
		suite.ErrorIs(err, boom)
	})
}

// TestCompositeStore_ChildrenDeduplicatesFileChildren tests that a file child the database also holds under the
// same parent is dropped, the database copy winning, from both the page and the count.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_ChildrenDeduplicatesFileChildren() {
	ctx := context.Background()
	parentID := "dedupe-parent"

	suite.SetupTest()
	suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
		ID: parentID, Handle: "parent", Name: "Parent",
	}))
	for _, id := range []string{"file-a", "shared", "file-b"} {
		suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
			ID: id, Handle: id, Name: id, Parent: &parentID,
		}))
	}
	otherParent := testOtherParentOUID
	sharedInDB := []OrganizationUnitBasic{{ID: "shared", Handle: "shared", Name: "shared", Parent: &parentID}}

	suite.Run("count and page list a duplicated child once", func() {
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(1, nil).Twice()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, mock.Anything).
			Return(sharedInDB, nil).Twice()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, parentID, 1, 0, mock.Anything).
			Return(sharedInDB, nil).Once()

		count, err := suite.compositeStore.GetOrganizationUnitChildrenCount(ctx, parentID, nil)
		suite.NoError(err)
		suite.Equal(3, count) // 1 database child + file-a + file-b

		page, err := suite.compositeStore.GetOrganizationUnitChildrenList(ctx, parentID, 10, 0, nil)
		suite.NoError(err)
		suite.Require().Len(page, 3)
		ids := []string{page[0].ID, page[1].ID, page[2].ID}
		suite.ElementsMatch([]string{"shared", "file-a", "file-b"}, ids)
		suite.Equal("shared", page[0].ID)
		suite.False(page[0].IsReadOnly)
	})

	suite.Run("a database row under another parent does not hide the file child", func() {
		suite.dbStoreMock.ExpectedCalls = nil
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(0, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, mock.Anything).
			Return([]OrganizationUnitBasic{{ID: "shared", Parent: &otherParent}}, nil).Once()

		count, err := suite.compositeStore.GetOrganizationUnitChildrenCount(ctx, parentID, nil)
		suite.NoError(err)
		suite.Equal(3, count)
	})

	suite.Run("a page inside the database part does not look the file children up", func() {
		suite.dbStoreMock.ExpectedCalls = nil
		suite.dbStoreMock.Calls = nil
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(5, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, parentID, 2, 0, mock.Anything).
			Return([]OrganizationUnitBasic{{ID: "d1"}, {ID: "d2"}}, nil).Once()

		page, err := suite.compositeStore.GetOrganizationUnitChildrenList(ctx, parentID, 2, 0, nil)
		suite.NoError(err)
		suite.Len(page, 2)
		suite.dbStoreMock.AssertNotCalled(suite.T(), "GetOrganizationUnitsByIDs", mock.Anything, mock.Anything)
	})

	suite.Run("propagates a database lookup error", func() {
		suite.dbStoreMock.ExpectedCalls = nil
		boom := errors.New("lookup failed")
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, parentID, mock.Anything).
			Return(0, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, mock.Anything).
			Return(nil, boom).Once()

		_, err := suite.compositeStore.GetOrganizationUnitChildrenCount(ctx, parentID, nil)
		suite.ErrorIs(err, boom)
	})
}

// TestCompositeStore_ChildrenFileLevelOverLimit tests that a level with more file children than the composite
// store's record limit is refused when the file children are needed, while a page inside the database part
// does not need them.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_ChildrenFileLevelOverLimit() {
	ctx := context.Background()
	suite.SetupTest()
	bigParent := "big-file-parent"
	suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
		ID: bigParent, Handle: "big", Name: "Big",
	}))
	for i := 0; i <= serverconst.MaxCompositeStoreRecords; i++ {
		suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
			ID: fmt.Sprintf("big-file-%04d", i), Handle: fmt.Sprintf("big-%04d", i),
			Name: fmt.Sprintf("Big %04d", i), Parent: &bigParent,
		}))
	}

	suite.Run("the count is refused", func() {
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, bigParent, mock.Anything).
			Return(0, nil).Once()

		_, err := suite.compositeStore.GetOrganizationUnitChildrenCount(ctx, bigParent, nil)
		suite.ErrorIs(err, ErrResultLimitExceededInCompositeMode)
	})

	suite.Run("a page that reaches the file children is refused", func() {
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, bigParent, mock.Anything).
			Return(5, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, bigParent, 5, 0, mock.Anything).
			Return(make([]OrganizationUnitBasic, 5), nil).Once()

		_, err := suite.compositeStore.GetOrganizationUnitChildrenList(ctx, bigParent, 10, 0, nil)
		suite.ErrorIs(err, ErrResultLimitExceededInCompositeMode)
	})

	suite.Run("a page inside the database part is served", func() {
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, bigParent, mock.Anything).
			Return(50, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenList", mock.Anything, bigParent, 10, 0, mock.Anything).
			Return(make([]OrganizationUnitBasic, 10), nil).Once()

		page, err := suite.compositeStore.GetOrganizationUnitChildrenList(ctx, bigParent, 10, 0, nil)
		suite.NoError(err)
		suite.Len(page, 10)
	})
}

// TestCompositeStore_ChildrenFileLookupInBatches tests that a level with more file children than one batch holds
// is looked up in the database in batches and still counts each duplicate once.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_ChildrenFileLookupInBatches() {
	ctx := context.Background()
	suite.SetupTest()
	batchParent := "batch-parent"
	suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
		ID: batchParent, Handle: "batch", Name: "Batch",
	}))
	fileChildren := idLookupBatchSize + 1
	for i := 0; i < fileChildren; i++ {
		suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
			ID: fmt.Sprintf("batch-file-%03d", i), Handle: fmt.Sprintf("batch-%03d", i),
			Name: fmt.Sprintf("Batch %03d", i), Parent: &batchParent,
		}))
	}

	// The database holds the first two ids of a batch, or the only id of a single-id batch, as children.
	var batchSizes []int
	duplicates := 0
	suite.dbStoreMock.On("GetOrganizationUnitChildrenCount", mock.Anything, batchParent, mock.Anything).
		Return(0, nil).Once()
	suite.dbStoreMock.EXPECT().GetOrganizationUnitsByIDs(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ids []string) ([]OrganizationUnitBasic, error) {
			batchSizes = append(batchSizes, len(ids))
			held := ids[:min(2, len(ids))]
			duplicates += len(held)
			rows := make([]OrganizationUnitBasic, 0, len(held))
			for _, id := range held {
				rows = append(rows, OrganizationUnitBasic{ID: id, Parent: &batchParent})
			}
			return rows, nil
		}).Twice()

	count, err := suite.compositeStore.GetOrganizationUnitChildrenCount(ctx, batchParent, nil)
	suite.NoError(err)
	suite.Equal([]int{idLookupBatchSize, 1}, batchSizes)
	suite.Equal(fileChildren-duplicates, count)
}

// TestMergeAndDeduplicateOUs tests the merge helper function for OUs.
func (suite *CompositeStoreCoverageTestSuite) TestMergeAndDeduplicateOUs() {
	suite.Run("marks DB OUs as mutable and file OUs as immutable", func() {
		const db1 = "db-1"
		const db2 = "db-2"
		const file1 = "file-1"
		dbOUs := []OrganizationUnitBasic{
			{ID: db1, Handle: "db1", Name: "DB 1"},
			{ID: db2, Handle: "db2", Name: "DB 2"},
		}
		fileOUs := []OrganizationUnitBasic{
			{ID: file1, Handle: "file1", Name: "File 1"},
			{ID: "file-2", Handle: "file2", Name: "File 2"},
		}

		result := mergeAndDeduplicateOUs(dbOUs, fileOUs)
		suite.Len(result, 4)

		// Verify IsReadOnly flags
		for _, ou := range result {
			switch ou.ID {
			case db1, db2:
				suite.False(ou.IsReadOnly, "DB OU %s should have IsReadOnly=false", ou.ID)
			case file1, "file-2":
				suite.True(ou.IsReadOnly, "File OU %s should have IsReadOnly=true", ou.ID)
			}
		}
	})

	suite.Run("DB takes precedence for duplicate IDs and marks as mutable", func() {
		dbOUs := []OrganizationUnitBasic{
			{ID: "duplicate", Handle: "db-handle", Name: "DB OU"},
		}
		fileOUs := []OrganizationUnitBasic{
			{ID: "duplicate", Handle: "file-handle", Name: "File OU"},
		}

		result := mergeAndDeduplicateOUs(dbOUs, fileOUs)
		suite.Len(result, 1)
		suite.Equal("db-handle", result[0].Handle)
		suite.False(result[0].IsReadOnly, "DB OU should be marked as mutable (IsReadOnly=false)")
	})

	suite.Run("handles empty slices and sets correct IsReadOnly", func() {
		result := mergeAndDeduplicateOUs([]OrganizationUnitBasic{}, []OrganizationUnitBasic{})
		suite.Empty(result)

		dbOUs := []OrganizationUnitBasic{{ID: "db-1", Handle: "db1", Name: "DB 1"}}
		result = mergeAndDeduplicateOUs(dbOUs, []OrganizationUnitBasic{})
		suite.Len(result, 1)
		suite.False(result[0].IsReadOnly, "DB OU should have IsReadOnly=false")

		fileOUs := []OrganizationUnitBasic{{ID: "file-1", Handle: "file1", Name: "File 1"}}
		result = mergeAndDeduplicateOUs([]OrganizationUnitBasic{}, fileOUs)
		suite.Len(result, 1)
		suite.True(result[0].IsReadOnly, "File OU should have IsReadOnly=true")
	})
}

// TestCompositeStore_GetOrganizationUnitsByIDs tests retrieving OUs by IDs.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_GetOrganizationUnitsByIDs() {
	suite.Run("returns empty slice when ids are empty", func() {
		result, err := suite.compositeStore.GetOrganizationUnitsByIDs(context.Background(), []string{})
		suite.NoError(err)
		suite.Empty(result)
	})

	suite.Run("merges OUs from both stores", func() {
		// Setup DB OUs
		dbOUs := []OrganizationUnitBasic{
			{ID: "db-1", Handle: "db1", Name: "DB 1"},
			{ID: "db-2", Handle: "db2", Name: "DB 2"},
		}

		// Setup file store OU
		err := suite.fileStore.CreateOrganizationUnit(context.Background(), OrganizationUnit{
			ID:     "file-1",
			Handle: "file1",
			Name:   "File 1",
		})
		suite.NoError(err)

		ids := []string{"db-1", "db-2", "file-1", "non-existent"}

		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, ids).
			Return(dbOUs, nil).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitsByIDs(context.Background(), ids)
		suite.NoError(err)
		suite.Len(result, 3) // 2 from DB + 1 from File

		// Verify IsReadOnly flags
		hasDB1, hasDB2, hasFile1 := false, false, false
		for _, ou := range result {
			if ou.ID == "db-1" {
				hasDB1 = true
				suite.False(ou.IsReadOnly)
			}
			if ou.ID == "db-2" {
				hasDB2 = true
				suite.False(ou.IsReadOnly)
			}
			if ou.ID == "file-1" {
				hasFile1 = true
				suite.True(ou.IsReadOnly)
			}
		}
		suite.True(hasDB1)
		suite.True(hasDB2)
		suite.True(hasFile1)
	})

	suite.Run("propagates DB error", func() {
		suite.SetupTest() // Fresh setup
		dbErr := errors.New("db query error")
		ids := []string{"ou-1"}

		suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, ids).
			Return([]OrganizationUnitBasic{}, dbErr).
			Once()

		result, err := suite.compositeStore.GetOrganizationUnitsByIDs(context.Background(), ids)
		suite.Error(err)
		suite.Equal(dbErr, err)
		suite.Empty(result)
	})
}

// The children of a parent in a project are counted and listed across both stores.
func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_ChildrenInProject() {
	ctx := context.Background()
	parentID := testSystemOUID
	suite.Require().NoError(suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
		ID: "file-child", Handle: "file", Name: "File", Parent: &parentID, ProjectID: "project-1",
	}))
	suite.dbStoreMock.On("GetOrganizationUnitChildrenCountInProject", mock.Anything, parentID, "project-1").
		Return(1, nil)
	suite.dbStoreMock.On("GetOrganizationUnitChildrenListInProject", mock.Anything, parentID, "project-1", 1, 0).
		Return([]OrganizationUnitBasic{{ID: "db-child", Parent: &parentID, ProjectID: "project-1"}}, nil)
	suite.dbStoreMock.On("GetOrganizationUnitsByIDs", mock.Anything, []string{"file-child"}).
		Return([]OrganizationUnitBasic{}, nil)

	count, err := suite.compositeStore.GetOrganizationUnitChildrenCountInProject(ctx, parentID, "project-1")
	suite.Require().NoError(err)
	suite.Equal(2, count)

	children, err := suite.compositeStore.GetOrganizationUnitChildrenListInProject(ctx, parentID, "project-1", 10, 0)
	suite.Require().NoError(err)
	suite.Equal([]string{"db-child", "file-child"}, []string{children[0].ID, children[1].ID})
	suite.False(children[0].IsReadOnly)
	suite.True(children[1].IsReadOnly)
}

func (suite *CompositeStoreCoverageTestSuite) TestCompositeStore_ChildrenInProjectErrors() {
	ctx := context.Background()
	suite.Run("fails when the database store cannot list them", func() {
		suite.SetupTest()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCountInProject", mock.Anything, "parent", "project-1").
			Return(1, nil)
		suite.dbStoreMock.On("GetOrganizationUnitChildrenListInProject", mock.Anything, "parent", "project-1", 1, 0).
			Return(nil, errors.New("boom"))

		_, err := suite.compositeStore.GetOrganizationUnitChildrenListInProject(ctx, "parent", "project-1", 10, 0)
		suite.Error(err)
	})

	suite.Run("serves a page past the composite store's limit", func() {
		suite.SetupTest()
		suite.dbStoreMock.On("GetOrganizationUnitChildrenCountInProject", mock.Anything, "parent", "project-1").
			Return(serverconst.MaxCompositeStoreRecords+1, nil)
		suite.dbStoreMock.On("GetOrganizationUnitChildrenListInProject", mock.Anything, "parent", "project-1", 1, 1000).
			Return([]OrganizationUnitBasic{{ID: "last-child"}}, nil)

		result, err := suite.compositeStore.GetOrganizationUnitChildrenListInProject(
			ctx, "parent", "project-1", 10, 1000,
		)
		suite.NoError(err)
		suite.Require().Len(result, 1)
		suite.Equal("last-child", result[0].ID)
	})
}

// A project's organization units are counted and listed across both stores, and a unit whose parent the other
// store holds in the same project is listed under its parent, not at the top.
func (suite *CompositeStoreTestSuite) TestCompositeStore_ProjectListing() {
	ctx := context.Background()
	fileOUID := "file-ou"
	systemOUID := testSystemOUID
	reset := func() {
		_ = entity.GetInstance().Clear()
		suite.fileStore, _ = newFileBasedStore()
		suite.dbStoreMock = newOrganizationUnitStoreInterfaceMock(suite.T())
		suite.compositeStore = newCompositeOUStore(suite.fileStore, suite.dbStoreMock)
		_ = suite.fileStore.CreateOrganizationUnit(ctx, OrganizationUnit{
			ID: fileOUID, Handle: "file", Name: "File", ProjectID: "project-1",
		})
	}
	expectDBTops := func(tops ...OrganizationUnitBasic) {
		suite.dbStoreMock.On("GetOrganizationUnitListCountByProject", mock.Anything, "project-1",
			(*tidcommon.FilterGroup)(nil)).Return(len(tops), nil).Once()
		if len(tops) > 0 {
			suite.dbStoreMock.On("GetOrganizationUnitListByProject", mock.Anything, "project-1", len(tops), 0,
				(*tidcommon.FilterGroup)(nil)).Return(tops, nil).Once()
		}
	}

	suite.Run("counts and lists from both stores", func() {
		reset()
		expectDBTops(OrganizationUnitBasic{ID: "db-ou", Name: "Db"})
		count, err := suite.compositeStore.GetOrganizationUnitListCountByProject(ctx, "project-1", nil)
		suite.NoError(err)
		suite.Equal(2, count)

		expectDBTops(OrganizationUnitBasic{ID: "db-ou", Name: "Db"})
		list, err := suite.compositeStore.GetOrganizationUnitListByProject(ctx, "project-1", 1, 1, nil)
		suite.NoError(err)
		suite.Require().Len(list, 1)
		suite.Equal(fileOUID, list[0].ID)
	})

	suite.Run("lists a database unit under its declared parent of the same project", func() {
		reset()
		expectDBTops(
			OrganizationUnitBasic{ID: "db-child", Name: "Child", Parent: &fileOUID},
			OrganizationUnitBasic{ID: "db-under-system", Name: "Under system", Parent: &systemOUID},
		)
		suite.dbStoreMock.On("GetOrganizationUnit", mock.Anything, fileOUID).
			Return(OrganizationUnit{}, ErrOrganizationUnitNotFound).Once()
		suite.dbStoreMock.On("GetOrganizationUnit", mock.Anything, systemOUID).
			Return(OrganizationUnit{ID: systemOUID}, nil).Once()

		list, err := suite.compositeStore.GetOrganizationUnitListByProject(ctx, "project-1", 10, 0, nil)

		suite.NoError(err)
		ids := make([]string, 0, len(list))
		for _, ou := range list {
			ids = append(ids, ou.ID)
		}
		suite.ElementsMatch([]string{"db-under-system", fileOUID}, ids)
	})

	suite.Run("fails when a parent cannot be read", func() {
		reset()
		expectDBTops(OrganizationUnitBasic{ID: "db-child", Name: "Child", Parent: &systemOUID})
		suite.dbStoreMock.On("GetOrganizationUnit", mock.Anything, systemOUID).
			Return(OrganizationUnit{}, errors.New("boom")).Once()

		_, err := suite.compositeStore.GetOrganizationUnitListByProject(ctx, "project-1", 10, 0, nil)

		suite.Error(err)
	})

	suite.Run("returns no page past the end", func() {
		reset()
		expectDBTops()

		list, err := suite.compositeStore.GetOrganizationUnitListByProject(ctx, "project-1", 10, 5, nil)

		suite.NoError(err)
		suite.Empty(list)
	})

	suite.Run("reports a listing over the limit", func() {
		reset()
		suite.dbStoreMock.On("GetOrganizationUnitListCountByProject", mock.Anything, "project-1",
			(*tidcommon.FilterGroup)(nil)).Return(serverconst.MaxCompositeStoreRecords, nil).Once()

		_, err := suite.compositeStore.GetOrganizationUnitListCountByProject(ctx, "project-1", nil)

		suite.ErrorIs(err, ErrResultLimitExceededInCompositeMode)
	})

	suite.Run("fails when the database store fails", func() {
		reset()
		suite.dbStoreMock.On("GetOrganizationUnitListCountByProject", mock.Anything, "project-1",
			(*tidcommon.FilterGroup)(nil)).Return(0, errors.New("boom")).Once()

		_, err := suite.compositeStore.GetOrganizationUnitListByProject(ctx, "project-1", 10, 0, nil)

		suite.Error(err)
	})

	suite.Run("fails when the database list fails", func() {
		reset()
		suite.dbStoreMock.On("GetOrganizationUnitListCountByProject", mock.Anything, "project-1",
			(*tidcommon.FilterGroup)(nil)).Return(1, nil).Once()
		suite.dbStoreMock.On("GetOrganizationUnitListByProject", mock.Anything, "project-1", 1, 0,
			(*tidcommon.FilterGroup)(nil)).Return(nil, errors.New("boom")).Once()

		_, err := suite.compositeStore.GetOrganizationUnitListByProject(ctx, "project-1", 10, 0, nil)

		suite.Error(err)
	})
}

// A move to another project goes to the database store.
func (suite *CompositeStoreTestSuite) TestCompositeStore_UpdateOrganizationUnitsProject() {
	updatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	suite.dbStoreMock.On("UpdateOrganizationUnitsProject", mock.Anything, []string{"ou-1"}, "project-1", updatedAt).
		Return(nil).Once()

	suite.NoError(suite.compositeStore.UpdateOrganizationUnitsProject(context.Background(), []string{"ou-1"},
		"project-1", updatedAt))
}
