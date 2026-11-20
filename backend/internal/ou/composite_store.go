// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// idLookupBatchSize bounds how many IDs one database lookup carries.
const idLookupBatchSize = 500

// compositeOUStore implements a composite store that combines file-based (immutable) and database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative OUs (from YAML files) cannot be modified or deleted
type compositeOUStore struct {
	fileStore organizationUnitStoreInterface
	dbStore   organizationUnitStoreInterface
}

// newCompositeOUStore creates a new composite store with both file-based and database stores.
func newCompositeOUStore(fileStore, dbStore organizationUnitStoreInterface) *compositeOUStore {
	return &compositeOUStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetOrganizationUnitListCount retrieves the total count of root organization units across both stores. A root
// unit the file store holds under an ID the database also holds is counted once.
func (c *compositeOUStore) GetOrganizationUnitListCount(ctx context.Context, f *tidcommon.FilterGroup) (int, error) {
	return c.countDatabaseAndFileOUs(
		func() (int, error) { return c.dbStore.GetOrganizationUnitListCount(ctx, f) },
		c.fileOUsNotInDatabase(ctx, isRootParent,
			func() (int, error) { return c.fileStore.GetOrganizationUnitListCount(ctx, f) },
			func(limit, offset int) ([]OrganizationUnitBasic, error) {
				return c.fileStore.GetOrganizationUnitList(ctx, limit, offset, f)
			},
		),
	)
}

// GetOrganizationUnitList retrieves root organization units from both stores with pagination, paged as
// GetOrganizationUnitChildrenList pages children.
func (c *compositeOUStore) GetOrganizationUnitList(
	ctx context.Context, limit, offset int, f *tidcommon.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	return pageDatabaseThenFile(
		limit, offset,
		func() (int, error) { return c.dbStore.GetOrganizationUnitListCount(ctx, f) },
		func(limit, offset int) ([]OrganizationUnitBasic, error) {
			return c.dbStore.GetOrganizationUnitList(ctx, limit, offset, f)
		},
		c.fileOUsNotInDatabase(ctx, isRootParent,
			func() (int, error) { return c.fileStore.GetOrganizationUnitListCount(ctx, f) },
			func(limit, offset int) ([]OrganizationUnitBasic, error) {
				return c.fileStore.GetOrganizationUnitList(ctx, limit, offset, f)
			},
		),
	)
}

// GetOrganizationUnitListCountByProject retrieves the count of the top organization units of a project across
// both stores.
func (c *compositeOUStore) GetOrganizationUnitListCountByProject(
	ctx context.Context, projectID string, f *tidcommon.FilterGroup,
) (int, error) {
	tops, err := c.projectTopOUs(ctx, projectID, f)
	if err != nil {
		return 0, err
	}
	return len(tops), nil
}

// GetOrganizationUnitListByProject retrieves the top organization units of a project across both stores with
// pagination. Applies the 1000-record limit in composite mode.
func (c *compositeOUStore) GetOrganizationUnitListByProject(
	ctx context.Context, projectID string, limit, offset int, f *tidcommon.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	tops, err := c.projectTopOUs(ctx, projectID, f)
	if err != nil {
		return nil, err
	}
	if offset >= len(tops) {
		return []OrganizationUnitBasic{}, nil
	}
	return tops[offset:min(offset+limit, len(tops))], nil
}

// projectTopOUs returns the top organization units of a project across both stores. Each store leaves out a
// unit whose parent it holds in the same project, but only this store sees a parent held by the other one: a
// database unit under a declared unit of the same project, or the reverse. Those are listed under their parent.
func (c *compositeOUStore) projectTopOUs(
	ctx context.Context, projectID string, f *tidcommon.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	dbCount, err := c.dbStore.GetOrganizationUnitListCountByProject(ctx, projectID, f)
	if err != nil {
		return nil, err
	}
	fileCount, err := c.fileStore.GetOrganizationUnitListCountByProject(ctx, projectID, f)
	if err != nil {
		return nil, err
	}
	if dbCount+fileCount > serverconst.MaxCompositeStoreRecords {
		return nil, ErrResultLimitExceededInCompositeMode
	}

	var dbOUs, fileOUs []OrganizationUnitBasic
	if dbCount > 0 {
		if dbOUs, err = c.dbStore.GetOrganizationUnitListByProject(ctx, projectID, dbCount, 0, f); err != nil {
			return nil, err
		}
	}
	if fileCount > 0 {
		if fileOUs, err = c.fileStore.GetOrganizationUnitListByProject(ctx, projectID, fileCount, 0, f); err != nil {
			return nil, err
		}
	}

	candidates := mergeAndDeduplicateOUs(dbOUs, fileOUs)
	tops := make([]OrganizationUnitBasic, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Parent != nil {
			parent, err := c.GetOrganizationUnit(ctx, *candidate.Parent)
			if err != nil && !errors.Is(err, ErrOrganizationUnitNotFound) {
				return nil, err
			}
			if err == nil && parent.ProjectID == projectID {
				continue
			}
		}
		tops = append(tops, candidate)
	}
	return tops, nil
}

// GetOrganizationUnitsByIDs retrieves organization units matching the given IDs from both stores.
func (c *compositeOUStore) GetOrganizationUnitsByIDs(
	ctx context.Context, ids []string,
) ([]OrganizationUnitBasic, error) {
	if len(ids) == 0 {
		return []OrganizationUnitBasic{}, nil
	}

	dbOUs, err := c.dbStore.GetOrganizationUnitsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	fileOUs, err := c.fileStore.GetOrganizationUnitsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	return mergeAndDeduplicateOUs(dbOUs, fileOUs), nil
}

// CreateOrganizationUnit creates a new organization unit in the database store only.
// Conflict checking and parent validation are handled at the service layer.
func (c *compositeOUStore) CreateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	return c.dbStore.CreateOrganizationUnit(ctx, ou)
}

// GetOrganizationUnit retrieves an organization unit by ID from either store.
// Checks database store first, then falls back to file store.
func (c *compositeOUStore) GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, error) {
	return declarativeresource.CompositeGetHelper(
		func() (OrganizationUnit, error) { return c.dbStore.GetOrganizationUnit(ctx, id) },
		func() (OrganizationUnit, error) { return c.fileStore.GetOrganizationUnit(ctx, id) },
		ErrOrganizationUnitNotFound,
	)
}

// GetOrganizationUnitByHandle retrieves an organization unit by handle and parent from either store.
// Checks database store first, then falls back to file store.
func (c *compositeOUStore) GetOrganizationUnitByHandle(
	ctx context.Context, handle string, parent *string,
) (OrganizationUnit, error) {
	return declarativeresource.CompositeGetHelper(
		func() (OrganizationUnit, error) {
			return c.dbStore.GetOrganizationUnitByHandle(ctx, handle, parent)
		},
		func() (OrganizationUnit, error) {
			return c.fileStore.GetOrganizationUnitByHandle(ctx, handle, parent)
		},
		ErrOrganizationUnitNotFound,
	)
}

// GetOrganizationUnitByPath retrieves an organization unit by hierarchical path from either store.
func (c *compositeOUStore) GetOrganizationUnitByPath(
	ctx context.Context,
	handles []string,
) (OrganizationUnit, error) {
	if len(handles) == 0 {
		return OrganizationUnit{}, ErrOrganizationUnitNotFound
	}

	current, err := c.findRootByHandle(ctx, handles[0])
	if err != nil {
		return OrganizationUnit{}, err
	}

	for i := 1; i < len(handles); i++ {
		current, err = c.findChildByHandle(ctx, current.ID, handles[i])
		if err != nil {
			return OrganizationUnit{}, err
		}
	}

	return current, nil
}

func (c *compositeOUStore) findRootByHandle(ctx context.Context, handle string) (OrganizationUnit, error) {
	return c.GetOrganizationUnitByHandle(ctx, handle, nil)
}

func (c *compositeOUStore) findChildByHandle(
	ctx context.Context, parentID, handle string,
) (OrganizationUnit, error) {
	return c.GetOrganizationUnitByHandle(ctx, handle, &parentID)
}

// IsOrganizationUnitExists checks if an organization unit exists in either store.
func (c *compositeOUStore) IsOrganizationUnitExists(ctx context.Context, id string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.IsOrganizationUnitExists(ctx, id) },
		func() (bool, error) { return c.dbStore.IsOrganizationUnitExists(ctx, id) },
	)
}

// IsOrganizationUnitDeclarative checks if an organization unit is immutable (exists in file store).
func (c *compositeOUStore) IsOrganizationUnitDeclarative(ctx context.Context, id string) bool {
	return declarativeresource.CompositeIsDeclarativeHelper(
		id,
		func(id string) (bool, error) { return c.fileStore.IsOrganizationUnitExists(ctx, id) },
	)
}

// CheckOrganizationUnitNameConflict checks for name conflicts in both stores.
func (c *compositeOUStore) CheckOrganizationUnitNameConflict(
	ctx context.Context, name string, parent *string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckOrganizationUnitNameConflict(ctx, name, parent) },
		func() (bool, error) { return c.dbStore.CheckOrganizationUnitNameConflict(ctx, name, parent) },
	)
}

// CheckOrganizationUnitHandleConflict checks for handle conflicts in both stores.
func (c *compositeOUStore) CheckOrganizationUnitHandleConflict(
	ctx context.Context, handle string, parent *string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckOrganizationUnitHandleConflict(ctx, handle, parent) },
		func() (bool, error) { return c.dbStore.CheckOrganizationUnitHandleConflict(ctx, handle, parent) },
	)
}

// UpdateOrganizationUnit updates an organization unit in the database store only.
// Immutability checks and parent validation are handled at the service layer.
func (c *compositeOUStore) UpdateOrganizationUnit(ctx context.Context, ou OrganizationUnit) error {
	return c.dbStore.UpdateOrganizationUnit(ctx, ou)
}

// UpdateOrganizationUnitsProject moves a set of organization units to a project in the database store only.
// The service refuses to move a declarative organization unit before it calls this.
func (c *compositeOUStore) UpdateOrganizationUnitsProject(
	ctx context.Context, ids []string, projectID string, updatedAt time.Time,
) error {
	return c.dbStore.UpdateOrganizationUnitsProject(ctx, ids, projectID, updatedAt)
}

// DeleteOrganizationUnit deletes an organization unit from the database store only.
// Immutability and children validation are handled at the service layer.
func (c *compositeOUStore) DeleteOrganizationUnit(ctx context.Context, id string) error {
	return c.dbStore.DeleteOrganizationUnit(ctx, id)
}

// GetOrganizationUnitChildrenCount retrieves the count of child OUs across both stores. A child the file store
// holds under an ID the database also holds as a child of the same parent is counted once.
func (c *compositeOUStore) GetOrganizationUnitChildrenCount(
	ctx context.Context, id string, f *tidcommon.FilterGroup,
) (int, error) {
	return c.countDatabaseAndFileOUs(
		func() (int, error) { return c.dbStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
		c.fileOUsNotInDatabase(ctx, isChildOf(id),
			func() (int, error) { return c.fileStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
			func(limit, offset int) ([]OrganizationUnitBasic, error) {
				return c.fileStore.GetOrganizationUnitChildrenList(ctx, id, limit, offset, f)
			},
		),
	)
}

// GetOrganizationUnitChildrenList retrieves child OUs from both stores with pagination. A level with more than
// 1000 file items is refused with ErrResultLimitExceededInCompositeMode.
func (c *compositeOUStore) GetOrganizationUnitChildrenList(ctx context.Context,
	id string, limit, offset int, f *tidcommon.FilterGroup,
) ([]OrganizationUnitBasic, error) {
	return pageDatabaseThenFile(
		limit, offset,
		func() (int, error) { return c.dbStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
		func(limit, offset int) ([]OrganizationUnitBasic, error) {
			return c.dbStore.GetOrganizationUnitChildrenList(ctx, id, limit, offset, f)
		},
		c.fileOUsNotInDatabase(ctx, isChildOf(id),
			func() (int, error) { return c.fileStore.GetOrganizationUnitChildrenCount(ctx, id, f) },
			func(limit, offset int) ([]OrganizationUnitBasic, error) {
				return c.fileStore.GetOrganizationUnitChildrenList(ctx, id, limit, offset, f)
			},
		),
	)
}

// GetOrganizationUnitChildrenCountInProject retrieves the count of the children of a parent that belong to a project
// across both stores, counting a child both stores hold once.
func (c *compositeOUStore) GetOrganizationUnitChildrenCountInProject(
	ctx context.Context, id, projectID string,
) (int, error) {
	return c.countDatabaseAndFileOUs(
		func() (int, error) { return c.dbStore.GetOrganizationUnitChildrenCountInProject(ctx, id, projectID) },
		c.fileOUsNotInDatabase(ctx, isChildOf(id),
			func() (int, error) { return c.fileStore.GetOrganizationUnitChildrenCountInProject(ctx, id, projectID) },
			func(limit, offset int) ([]OrganizationUnitBasic, error) {
				return c.fileStore.GetOrganizationUnitChildrenListInProject(ctx, id, projectID, limit, offset)
			},
		),
	)
}

// GetOrganizationUnitChildrenListInProject retrieves the children of a parent that belong to a project from both
// stores with pagination. The page is read as GetOrganizationUnitChildrenList reads it.
func (c *compositeOUStore) GetOrganizationUnitChildrenListInProject(
	ctx context.Context, id, projectID string, limit, offset int,
) ([]OrganizationUnitBasic, error) {
	return pageDatabaseThenFile(
		limit, offset,
		func() (int, error) { return c.dbStore.GetOrganizationUnitChildrenCountInProject(ctx, id, projectID) },
		func(limit, offset int) ([]OrganizationUnitBasic, error) {
			return c.dbStore.GetOrganizationUnitChildrenListInProject(ctx, id, projectID, limit, offset)
		},
		c.fileOUsNotInDatabase(ctx, isChildOf(id),
			func() (int, error) { return c.fileStore.GetOrganizationUnitChildrenCountInProject(ctx, id, projectID) },
			func(limit, offset int) ([]OrganizationUnitBasic, error) {
				return c.fileStore.GetOrganizationUnitChildrenListInProject(ctx, id, projectID, limit, offset)
			},
		),
	)
}

// isChildOf matches the parent of a child of the organization unit with the given ID.
func isChildOf(parentID string) func(parent *string) bool {
	return func(parent *string) bool { return parent != nil && *parent == parentID }
}

// isRootParent matches the parent of a root organization unit, which has none.
func isRootParent(parent *string) bool { return parent == nil }

// fileOUsNotInDatabase returns a function that lists one level's file units without those the database also holds
// at that level (isSameLevel checks a database row's parent). It refuses a level over the record limit.
func (c *compositeOUStore) fileOUsNotInDatabase(
	ctx context.Context, isSameLevel func(parent *string) bool,
	countFile func() (int, error),
	listFile func(limit, offset int) ([]OrganizationUnitBasic, error),
) func() ([]OrganizationUnitBasic, error) {
	return func() ([]OrganizationUnitBasic, error) {
		fileCount, err := countFile()
		if err != nil || fileCount == 0 {
			return nil, err
		}
		if fileCount > serverconst.MaxCompositeStoreRecords {
			return nil, ErrResultLimitExceededInCompositeMode
		}
		files, err := listFile(fileCount, 0)
		if err != nil {
			return nil, err
		}

		ids := make([]string, len(files))
		for i := range files {
			ids[i] = files[i].ID
		}
		inDatabase := make(map[string]struct{})
		for start := 0; start < len(ids); start += idLookupBatchSize {
			rows, err := c.dbStore.GetOrganizationUnitsByIDs(ctx, ids[start:min(start+idLookupBatchSize, len(ids))])
			if err != nil {
				return nil, err
			}
			for i := range rows {
				if isSameLevel(rows[i].Parent) {
					inDatabase[rows[i].ID] = struct{}{}
				}
			}
		}

		unique := make([]OrganizationUnitBasic, 0, len(files)-len(inDatabase))
		for i := range files {
			if _, dup := inDatabase[files[i].ID]; !dup {
				unique = append(unique, files[i])
			}
		}
		return unique, nil
	}
}

// countDatabaseAndFileOUs adds the database's count to the number of file units it does not hold.
func (c *compositeOUStore) countDatabaseAndFileOUs(
	countDB func() (int, error), fileChildren func() ([]OrganizationUnitBasic, error),
) (int, error) {
	dbCount, err := countDB()
	if err != nil {
		return 0, err
	}
	files, err := fileChildren()
	if err != nil {
		return 0, err
	}
	return dbCount + len(files), nil
}

// pageDatabaseThenFile serves one page of the database items followed by the file items the database does not
// hold, reading the file items only when the page reaches past the database items.
func pageDatabaseThenFile(
	limit, offset int,
	countDB func() (int, error),
	listDB func(limit, offset int) ([]OrganizationUnitBasic, error),
	fileChildren func() ([]OrganizationUnitBasic, error),
) ([]OrganizationUnitBasic, error) {
	dbCount, err := countDB()
	if err != nil {
		return nil, err
	}

	page := make([]OrganizationUnitBasic, 0)

	dbTake := 0
	if offset < dbCount {
		dbTake = min(limit, dbCount-offset)
	}
	if dbTake > 0 {
		items, err := listDB(dbTake, offset)
		if err != nil {
			return nil, err
		}
		for i := range items {
			items[i].IsReadOnly = false
		}
		page = append(page, items...)
	}

	if fileTake := limit - dbTake; fileTake > 0 {
		files, err := fileChildren()
		if err != nil {
			return nil, err
		}
		fileOffset := max(0, offset-dbCount)
		if fileOffset < len(files) {
			items := files[fileOffset:min(fileOffset+fileTake, len(files))]
			for i := range items {
				items[i].IsReadOnly = true
			}
			page = append(page, items...)
		}
	}

	return page, nil
}

// mergeAndDeduplicateOUs merges root-level OUs from both stores and removes duplicates by ID.
// While duplicates shouldn't exist by design (an OU exists in only one store), this provides
// defensive programming against misconfigurations or bugs.
func mergeAndDeduplicateOUs(dbOUs, fileOUs []OrganizationUnitBasic) []OrganizationUnitBasic {
	seen := make(map[string]bool)
	result := make([]OrganizationUnitBasic, 0, len(dbOUs)+len(fileOUs))

	// Add DB OUs first (they take precedence) - mark as mutable (isReadOnly=false)
	for i := range dbOUs {
		if !seen[dbOUs[i].ID] {
			seen[dbOUs[i].ID] = true
			dbOUs[i].IsReadOnly = false
			result = append(result, dbOUs[i])
		}
	}

	// Add file OUs if not already present - mark as immutable (isReadOnly=true)
	for i := range fileOUs {
		if !seen[fileOUs[i].ID] {
			seen[fileOUs[i].ID] = true
			fileOUs[i].IsReadOnly = true
			result = append(result, fileOUs[i])
		}
	}

	return result
}
