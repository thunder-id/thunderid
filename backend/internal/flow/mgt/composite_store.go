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

package flowmgt

import (
	"context"

	"github.com/thunder-id/thunderid/internal/flow/common"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeFlowStore implements a composite store that combines file-based (immutable) and database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative flows (from YAML files) cannot be modified or deleted
type compositeFlowStore struct {
	fileStore flowStoreInterface
	dbStore   flowStoreInterface
}

// newCompositeFlowStore creates a new composite store with both file-based and database stores.
func newCompositeFlowStore(fileStore, dbStore flowStoreInterface) *compositeFlowStore {
	return &compositeFlowStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// ListFlows retrieves flows from both stores, merges and deduplicates them, then applies pagination.
// Database flows are marked as mutable (IsReadOnly=false), file-based flows as immutable (IsReadOnly=true).
// This method fetches all flows from both stores, computes the deduplicated total, then applies
// pagination to the merged result to ensure accurate total count and correct pagination behavior.
func (c *compositeFlowStore) ListFlows(ctx context.Context, limit, offset int, flowType string) (
	[]BasicFlowDefinition, int, error) {
	// Fetch all flows from both stores (use a large limit to get all results)
	// We use 10000 as a practical "unlimited" sentinel - in practice, no deployment should have this many flows
	const unlimitedSentinel = 10000

	dbFlows, _, err := c.dbStore.ListFlows(ctx, unlimitedSentinel, 0, flowType)
	if err != nil {
		return nil, 0, err
	}

	fileFlows, _, err := c.fileStore.ListFlows(ctx, unlimitedSentinel, 0, flowType)
	if err != nil {
		return nil, 0, err
	}

	// Merge and deduplicate to get the full union
	mergedAll := mergeAndDeduplicateFlows(dbFlows, fileFlows)
	total := len(mergedAll)

	// Apply pagination to the merged result
	start := offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	paginatedResult := mergedAll[start:end]
	return paginatedResult, total, nil
}

// CreateFlow creates a new flow in the database store only.
func (c *compositeFlowStore) CreateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*CompleteFlowDefinition, error) {
	return c.dbStore.CreateFlow(ctx, flowID, flow)
}

// GetFlowByID retrieves a flow by ID from either store.
// Checks database store first, then falls back to file store.
// Flows from the file store are marked as read-only (IsReadOnly=true).
func (c *compositeFlowStore) GetFlowByID(ctx context.Context, flowID string) (*CompleteFlowDefinition, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*CompleteFlowDefinition, error) { return c.dbStore.GetFlowByID(ctx, flowID) },
		func() (*CompleteFlowDefinition, error) {
			flow, err := c.fileStore.GetFlowByID(ctx, flowID)
			if err != nil {
				return nil, err
			}
			if flow != nil {
				flow.IsReadOnly = true
			}
			return flow, nil
		},
		errFlowNotFound,
	)
}

// GetFlowByHandle retrieves a flow by handle from either store.
// Checks database store first, then falls back to file store.
// Flows from the file store are marked as read-only (IsReadOnly=true).
func (c *compositeFlowStore) GetFlowByHandle(ctx context.Context, handle string,
	flowType common.FlowType) (*CompleteFlowDefinition, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*CompleteFlowDefinition, error) { return c.dbStore.GetFlowByHandle(ctx, handle, flowType) },
		func() (*CompleteFlowDefinition, error) {
			flow, err := c.fileStore.GetFlowByHandle(ctx, handle, flowType)
			if err != nil {
				return nil, err
			}
			if flow != nil {
				flow.IsReadOnly = true
			}
			return flow, nil
		},
		errFlowNotFound,
	)
}

// UpdateFlow updates a flow in the database store only.
// Immutability checks are handled at the service layer.
func (c *compositeFlowStore) UpdateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*CompleteFlowDefinition, error) {
	return c.dbStore.UpdateFlow(ctx, flowID, flow)
}

// DeleteFlow deletes a flow from the database store only.
// Immutability checks are handled at the service layer.
func (c *compositeFlowStore) DeleteFlow(ctx context.Context, flowID string) error {
	return c.dbStore.DeleteFlow(ctx, flowID)
}

// ListFlowVersions retrieves versions from the database store only.
func (c *compositeFlowStore) ListFlowVersions(ctx context.Context, flowID string) ([]BasicFlowVersion, error) {
	return c.dbStore.ListFlowVersions(ctx, flowID)
}

// GetFlowVersion retrieves a specific flow version from the database store only.
func (c *compositeFlowStore) GetFlowVersion(ctx context.Context, flowID string, version int) (*FlowVersion, error) {
	return c.dbStore.GetFlowVersion(ctx, flowID, version)
}

// RestoreFlowVersion restores a flow version in the database store only.
func (c *compositeFlowStore) RestoreFlowVersion(ctx context.Context, flowID string, version int) (
	*CompleteFlowDefinition, error) {
	return c.dbStore.RestoreFlowVersion(ctx, flowID, version)
}

// IsFlowExistsByHandle checks if a flow exists by handle in either store.
func (c *compositeFlowStore) IsFlowExistsByHandle(ctx context.Context, handle string,
	flowType common.FlowType) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.IsFlowExistsByHandle(ctx, handle, flowType) },
		func() (bool, error) { return c.dbStore.IsFlowExistsByHandle(ctx, handle, flowType) },
	)
}

// mergeAndDeduplicateFlows merges flows from both stores and removes duplicates by ID.
// Database flows are marked as mutable (IsReadOnly=false), file-based flows as immutable (IsReadOnly=true).
// While duplicates shouldn't exist by design, this provides defensive programming.
func mergeAndDeduplicateFlows(dbFlows, fileFlows []BasicFlowDefinition) []BasicFlowDefinition {
	seen := make(map[string]bool)
	result := make([]BasicFlowDefinition, 0, len(dbFlows)+len(fileFlows))

	// Add DB flows first (they take precedence) - mark as mutable (IsReadOnly=false)
	for i := range dbFlows {
		if !seen[dbFlows[i].ID] {
			seen[dbFlows[i].ID] = true
			dbFlows[i].IsReadOnly = false
			result = append(result, dbFlows[i])
		}
	}

	// Add file flows if not already present - mark as immutable (IsReadOnly=true)
	for i := range fileFlows {
		if !seen[fileFlows[i].ID] {
			seen[fileFlows[i].ID] = true
			fileFlows[i].IsReadOnly = true
			result = append(result, fileFlows[i])
		}
	}

	return result
}
