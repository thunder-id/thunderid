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

package flowmgt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Database column names
const (
	colFlowID        = "id"
	colHandle        = "handle"
	colName          = "name"
	colFlowType      = "flow_type"
	colActiveVersion = "active_version"
	colNodes         = "nodes"
	colCreatedAt     = "created_at"
	colUpdatedAt     = "updated_at"
	colVersion       = "version"
	colCount         = "count"
)

var getDBProvider = provider.GetDBProvider

// flowStoreInterface defines the interface for flow store operations.
type flowStoreInterface interface {
	ListFlows(ctx context.Context, limit, offset int, flowType string) ([]BasicFlowDefinition, int, error)
	CreateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (*CompleteFlowDefinition, error)
	GetFlowByID(ctx context.Context, flowID string) (*CompleteFlowDefinition, error)
	GetFlowByHandle(ctx context.Context, handle string, flowType common.FlowType) (*CompleteFlowDefinition, error)
	UpdateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (*CompleteFlowDefinition, error)
	DeleteFlow(ctx context.Context, flowID string) error
	ListFlowVersions(ctx context.Context, flowID string) ([]BasicFlowVersion, error)
	GetFlowVersion(ctx context.Context, flowID string, version int) (*FlowVersion, error)
	RestoreFlowVersion(ctx context.Context, flowID string, version int) (*CompleteFlowDefinition, error)
	IsFlowExistsByHandle(ctx context.Context, handle string, flowType common.FlowType) (bool, error)
}

// flowStore is the default implementation of flowStoreInterface.
type flowStore struct {
	dbProvider        provider.DBProviderInterface
	deploymentID      string
	maxVersionHistory int
	logger            *log.Logger
}

// newFlowStore creates a new instance of flowStore.
func newFlowStore() (flowStoreInterface, transaction.Transactioner, error) {
	dbProvider := getDBProvider()
	transactioner, err := dbProvider.GetConfigDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &flowStore{
		dbProvider:        dbProvider,
		deploymentID:      config.GetServerRuntime().Config.Server.Identifier,
		maxVersionHistory: getMaxVersionHistory(),
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowStore")),
	}, transactioner, nil
}

// ListFlows retrieves a paginated list of flow definitions with optional filtering by flow type.
func (s *flowStore) ListFlows(ctx context.Context, limit, offset int, flowType string) (
	[]BasicFlowDefinition, int, error) {
	var flows []BasicFlowDefinition
	var totalCount int

	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		var countResults, results []map[string]interface{}
		var err error

		if flowType != "" {
			countResults, err = dbClient.QueryContext(ctx, queryCountFlowsWithType, flowType, s.deploymentID)
			if err != nil {
				return fmt.Errorf("failed to count flows: %w", err)
			}

			results, err = dbClient.QueryContext(ctx, queryListFlowsWithType, flowType, s.deploymentID, limit, offset)
			if err != nil {
				return fmt.Errorf("failed to list flows: %w", err)
			}
		} else {
			countResults, err = dbClient.QueryContext(ctx, queryCountFlows, s.deploymentID)
			if err != nil {
				return fmt.Errorf("failed to count flows: %w", err)
			}

			results, err = dbClient.QueryContext(ctx, queryListFlows, s.deploymentID, limit, offset)
			if err != nil {
				return fmt.Errorf("failed to list flows: %w", err)
			}
		}

		totalCount, err = s.parseCountResult(countResults)
		if err != nil {
			return err
		}

		flows = make([]BasicFlowDefinition, 0, len(results))
		for _, row := range results {
			flow, err := s.buildBasicFlowDefinitionFromRow(row)
			if err != nil {
				return fmt.Errorf("failed to build flow: %w", err)
			}
			flows = append(flows, flow)
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return flows, totalCount, nil
}

// CreateFlow creates a new flow definition with version 1.
func (s *flowStore) CreateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*CompleteFlowDefinition, error) {
	nodesJSON, err := json.Marshal(flow.Nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	err = s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryCreateFlow, flowID, flow.Handle,
			flow.Name, flow.FlowType, int64(1), s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to create flow: %w", err)
		}

		_, err = dbClient.ExecuteContext(ctx, queryInsertFlowVersion, flowID, 1, string(nodesJSON), s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to create flow version: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetFlowByID(ctx, flowID)
}

// GetFlowByID retrieves the active version of a flow definition by its ID.
func (s *flowStore) GetFlowByID(ctx context.Context, flowID string) (*CompleteFlowDefinition, error) {
	var flow *CompleteFlowDefinition
	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetFlow, flowID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get flow: %w", err)
		}

		if len(results) == 0 {
			return errFlowNotFound
		}

		flow, err = s.buildCompleteFlowDefinitionFromRow(results[0])
		return err
	})

	return flow, err
}

// GetFlowByHandle retrieves a flow definition by handle and flow type.
func (s *flowStore) GetFlowByHandle(ctx context.Context, handle string, flowType common.FlowType) (
	*CompleteFlowDefinition, error) {
	var flow *CompleteFlowDefinition
	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetFlowByHandle, handle, string(flowType), s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get flow by handle: %w", err)
		}

		if len(results) == 0 {
			return errFlowNotFound
		}

		flow, err = s.buildCompleteFlowDefinitionFromRow(results[0])
		return err
	})

	return flow, err
}

// UpdateFlow updates a flow definition by creating a new version.
// Automatically deletes oldest versions if the count exceeds max_version_history.
func (s *flowStore) UpdateFlow(ctx context.Context, flowID string, flow *FlowDefinition) (
	*CompleteFlowDefinition, error) {
	nodesJSON, err := json.Marshal(flow.Nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	err = s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		flowResults, err := dbClient.QueryContext(ctx, queryGetFlow, flowID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get flow metadata: %w", err)
		}
		if len(flowResults) == 0 {
			return errFlowNotFound
		}

		currentFlow, err := s.buildCompleteFlowDefinitionFromRow(flowResults[0])
		if err != nil {
			return errFlowNotFound
		}

		newVersion := currentFlow.ActiveVersion + 1

		// Insert the new version first to ensure it succeeds before updating the flow
		if err := s.pushToVersionStack(ctx, dbClient, flowID, newVersion, string(nodesJSON)); err != nil {
			return err
		}

		_, err = dbClient.ExecuteContext(ctx, queryUpdateFlow, flowID, flow.Name, newVersion, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to update flow: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetFlowByID(ctx, flowID)
}

// DeleteFlow deletes a flow definition and all its version history.
func (s *flowStore) DeleteFlow(ctx context.Context, flowID string) error {
	return s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryDeleteFlow, flowID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to delete flow: %w", err)
		}
		return nil
	})
}

// IsFlowExistsByHandle checks if a flow exists with the given handle and flow type.
func (s *flowStore) IsFlowExistsByHandle(ctx context.Context, handle string, flowType common.FlowType) (bool, error) {
	var exists bool
	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryCheckFlowExistsByHandle,
			handle, string(flowType), s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to check flow existence by handle: %w", err)
		}

		exists = len(results) > 0
		return nil
	})

	return exists, err
}

// ListFlowVersions retrieves all versions of a flow definition.
func (s *flowStore) ListFlowVersions(ctx context.Context, flowID string) ([]BasicFlowVersion, error) {
	var versions []BasicFlowVersion

	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryListFlowVersions, flowID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to list flow versions: %w", err)
		}

		versions = make([]BasicFlowVersion, 0, len(results))
		for _, row := range results {
			version, err := s.buildBasicFlowVersionFromRow(row)
			if err != nil {
				return fmt.Errorf("failed to build flow version: %w", err)
			}
			versions = append(versions, version)
		}

		return nil
	})

	return versions, err
}

// GetFlowVersion retrieves a specific version of a flow definition.
func (s *flowStore) GetFlowVersion(ctx context.Context, flowID string, version int) (*FlowVersion, error) {
	var flowVersion *FlowVersion

	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetFlowVersionWithMetadata, flowID, version, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get flow version: %w", err)
		}
		if len(results) == 0 {
			return errVersionNotFound
		}

		flowVersion, err = s.buildFlowVersionFromRow(results[0])
		return err
	})

	return flowVersion, err
}

// RestoreFlowVersion restores a specified version as the active version.
// This creates a new version by copying the configuration from the specified version.
// Automatically deletes oldest versions if the count exceeds max_version_history.
func (s *flowStore) RestoreFlowVersion(ctx context.Context, flowID string, version int) (
	*CompleteFlowDefinition, error) {
	err := s.withDBClientContext(ctx, func(dbClient provider.DBClientInterface) error {
		flowResults, err := dbClient.QueryContext(ctx, queryGetFlow, flowID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get flow metadata: %w", err)
		}
		if len(flowResults) == 0 {
			return errFlowNotFound
		}

		currentFlow, err := s.buildCompleteFlowDefinitionFromRow(flowResults[0])
		if err != nil {
			return errFlowNotFound
		}

		versionResults, err := dbClient.QueryContext(ctx, queryGetFlowVersion, flowID, version, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get version to restore: %w", err)
		}
		if len(versionResults) == 0 {
			return errVersionNotFound
		}

		nodesJSON, err := s.getString(versionResults[0], colNodes)
		if err != nil {
			return errVersionNotFound
		}

		newVersion := currentFlow.ActiveVersion + 1

		// Insert the new version first to ensure it succeeds before updating the flow
		if err := s.pushToVersionStack(ctx, dbClient, flowID, newVersion, nodesJSON); err != nil {
			return err
		}

		_, err = dbClient.ExecuteContext(ctx, queryUpdateFlow, flowID, currentFlow.Name, newVersion, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to update flow: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetFlowByID(ctx, flowID)
}

// pushToVersionStack adds a new version to the version history and removes the oldest version
// if the count exceeds max_version_history.
func (s *flowStore) pushToVersionStack(ctx context.Context, dbClient provider.DBClientInterface,
	flowID string, version int, nodesJSON string) error {
	_, err := dbClient.ExecuteContext(ctx, queryInsertFlowVersion, flowID, version, nodesJSON, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to insert flow version: %w", err)
	}

	countResults, err := dbClient.QueryContext(ctx, queryCountFlowVersions, flowID, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to count versions: %w", err)
	}

	versionCount, err := s.parseCountResult(countResults)
	if err != nil {
		return err
	}

	if versionCount > s.maxVersionHistory {
		if _, err := dbClient.ExecuteContext(ctx, queryDeleteOldestVersion, flowID, s.deploymentID); err != nil {
			return fmt.Errorf("failed to delete oldest version: %w", err)
		}
	}

	return nil
}

// getConfigDBClient retrieves the configuration database client.
func (s *flowStore) getConfigDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// withDBClientContext executes a function with a DB client, threading the context through.
func (s *flowStore) withDBClientContext(_ context.Context, fn func(provider.DBClientInterface) error) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}
	return fn(dbClient)
}

// parseCountResult parses a count result from database query.
func (s *flowStore) parseCountResult(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, nil
	}

	countVal, ok := results[0][colCount]
	if !ok {
		return 0, fmt.Errorf("count field not found in result")
	}

	switch v := countVal.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected count type: %T", countVal)
	}
}

// getString safely extracts a string value from a database row.
// Handles both string (SQLite) and []byte (PostgreSQL) types.
func (s *flowStore) getString(row map[string]interface{}, key string) (string, error) {
	val := row[key]
	switch v := val.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("%s field is missing or invalid", key)
	}
}

// getTimestamp safely extracts a timestamp value from a database row.
// Handles both string (SQLite) and time.Time (PostgreSQL) types.
func (s *flowStore) getTimestamp(row map[string]interface{}, key string) (string, error) {
	val := row[key]
	switch v := val.(type) {
	case string:
		return v, nil
	case time.Time:
		// Convert time.Time to RFC3339 format for consistency
		return v.Format(time.RFC3339), nil
	default:
		return "", fmt.Errorf("%s field is missing or invalid", key)
	}
}

// getInt64 safely extracts an int64 value from a database row.
func (s *flowStore) getInt64(row map[string]interface{}, key string) (int64, error) {
	if val, ok := row[key].(int64); ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s field is missing or invalid", key)
}

// buildBasicFlowDefinitionFromRow builds a BasicFlowDefinition from a database row.
func (s *flowStore) buildBasicFlowDefinitionFromRow(row map[string]interface{}) (
	BasicFlowDefinition, error) {
	flowID, err := s.getString(row, colFlowID)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	handle, err := s.getString(row, colHandle)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	name, err := s.getString(row, colName)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	flowTypeStr, err := s.getString(row, colFlowType)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	activeVersion, err := s.getInt64(row, colActiveVersion)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	createdAt, err := s.getTimestamp(row, colCreatedAt)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	updatedAt, err := s.getTimestamp(row, colUpdatedAt)
	if err != nil {
		return BasicFlowDefinition{}, err
	}

	return BasicFlowDefinition{
		ID:            flowID,
		Handle:        handle,
		Name:          name,
		FlowType:      common.FlowType(flowTypeStr),
		ActiveVersion: int(activeVersion),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}

// buildCompleteFlowDefinitionFromRow builds a CompleteFlowDefinition from a database row.
func (s *flowStore) buildCompleteFlowDefinitionFromRow(row map[string]interface{}) (
	*CompleteFlowDefinition, error) {
	flowID, err := s.getString(row, colFlowID)
	if err != nil {
		return nil, err
	}

	handle, err := s.getString(row, colHandle)
	if err != nil {
		return nil, err
	}

	name, err := s.getString(row, colName)
	if err != nil {
		return nil, err
	}

	flowTypeStr, err := s.getString(row, colFlowType)
	if err != nil {
		return nil, err
	}

	activeVersion, err := s.getInt64(row, colActiveVersion)
	if err != nil {
		return nil, err
	}

	createdAt, err := s.getTimestamp(row, colCreatedAt)
	if err != nil {
		return nil, err
	}

	updatedAt, err := s.getTimestamp(row, colUpdatedAt)
	if err != nil {
		return nil, err
	}

	nodesJSON, err := s.getString(row, colNodes)
	if err != nil {
		return nil, err
	}

	flow := &CompleteFlowDefinition{
		ID:            flowID,
		Handle:        handle,
		Name:          name,
		FlowType:      common.FlowType(flowTypeStr),
		ActiveVersion: int(activeVersion),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}

	if err := json.Unmarshal([]byte(nodesJSON), &flow.Nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	return flow, nil
}

// buildBasicFlowVersionFromRow builds a BasicFlowVersion from a database row.
func (s *flowStore) buildBasicFlowVersionFromRow(row map[string]interface{}) (BasicFlowVersion, error) {
	version, err := s.getInt64(row, colVersion)
	if err != nil {
		return BasicFlowVersion{}, err
	}

	createdAt, err := s.getTimestamp(row, colCreatedAt)
	if err != nil {
		return BasicFlowVersion{}, err
	}

	activeVersion, err := s.getInt64(row, colActiveVersion)
	if err != nil {
		return BasicFlowVersion{}, err
	}

	return BasicFlowVersion{
		Version:   int(version),
		CreatedAt: createdAt,
		IsActive:  int(version) == int(activeVersion),
	}, nil
}

// buildFlowVersionFromRow builds a FlowVersion from a single joined database row.
func (s *flowStore) buildFlowVersionFromRow(row map[string]interface{}) (*FlowVersion, error) {
	flowID, err := s.getString(row, colFlowID)
	if err != nil {
		return nil, err
	}

	handle, err := s.getString(row, colHandle)
	if err != nil {
		return nil, err
	}

	name, err := s.getString(row, colName)
	if err != nil {
		return nil, err
	}

	flowTypeStr, err := s.getString(row, colFlowType)
	if err != nil {
		return nil, err
	}

	version, err := s.getInt64(row, colVersion)
	if err != nil {
		return nil, err
	}

	createdAt, err := s.getTimestamp(row, colCreatedAt)
	if err != nil {
		return nil, err
	}

	activeVersion, err := s.getInt64(row, colActiveVersion)
	if err != nil {
		return nil, err
	}

	nodesJSON, err := s.getString(row, colNodes)
	if err != nil {
		return nil, err
	}

	flowVersion := &FlowVersion{
		ID:        flowID,
		Handle:    handle,
		Name:      name,
		FlowType:  flowTypeStr,
		Version:   int(version),
		IsActive:  int(version) == int(activeVersion),
		CreatedAt: createdAt,
	}

	if err := json.Unmarshal([]byte(nodesJSON), &flowVersion.Nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	return flowVersion, nil
}

// getMaxVersionHistory retrieves the maximum version history size from configuration.
// If not set or invalid, returns the default value.
func getMaxVersionHistory() int {
	flowConfig := config.GetServerRuntime().Config.Flow
	if flowConfig.MaxVersionHistory <= 0 {
		return defaultVersionHistory
	}
	if flowConfig.MaxVersionHistory > maxAllowedVersionHistory {
		return maxAllowedVersionHistory
	}

	return flowConfig.MaxVersionHistory
}
