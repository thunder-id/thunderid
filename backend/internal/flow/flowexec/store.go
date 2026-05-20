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

package flowexec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// flowStoreInterface defines the methods for flow context storage operations.
type flowStoreInterface interface {
	StoreFlowContext(ctx context.Context, dbModel FlowContextDB, expirySeconds int64) error
	GetFlowContext(ctx context.Context, executionID string) (*FlowContextDB, error)
	UpdateFlowContext(ctx context.Context, dbModel FlowContextDB) error
	DeleteFlowContext(ctx context.Context, executionID string) error
}

// flowStore implements the FlowStoreInterface for managing flow contexts.
type flowStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newFlowStore creates a new instance of FlowStore.
func newFlowStore(dbProvider provider.DBProviderInterface) flowStoreInterface {
	return &flowStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// StoreFlowContext stores the complete flow context in the database.
func (s *flowStore) StoreFlowContext(ctx context.Context, dbModel FlowContextDB, expirySeconds int64) error {
	expiryTime := time.Now().UTC().Add(time.Duration(expirySeconds) * time.Second)

	return withRuntimeDBClientContext(ctx, s.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, QueryCreateFlowContext,
			dbModel.ExecutionID, s.deploymentID, dbModel.Context, expiryTime)
		return err
	})
}

// GetFlowContext retrieves the flow context from the database.
func (s *flowStore) GetFlowContext(ctx context.Context, executionID string) (*FlowContextDB, error) {
	var result *FlowContextDB

	err := withRuntimeDBClientContext(ctx, s.dbProvider, func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, QueryGetFlowContext,
			executionID, s.deploymentID, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}

		if len(results) == 0 {
			return nil
		}

		if len(results) != 1 {
			return fmt.Errorf("unexpected number of results: %d", len(results))
		}

		row := results[0]
		var buildErr error
		result, buildErr = s.buildFlowContextFromResultRow(row)
		return buildErr
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UpdateFlowContext updates the flow context in the database.
func (s *flowStore) UpdateFlowContext(ctx context.Context, dbModel FlowContextDB) error {
	return withRuntimeDBClientContext(ctx, s.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, QueryUpdateFlowContext,
			dbModel.ExecutionID, dbModel.Context, s.deploymentID)
		return err
	})
}

// DeleteFlowContext removes the flow context from the database.
func (s *flowStore) DeleteFlowContext(ctx context.Context, executionID string) error {
	return withRuntimeDBClientContext(ctx, s.dbProvider, func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, QueryDeleteFlowContext, executionID, s.deploymentID)
		return err
	})
}

// withRuntimeDBClientContext is a helper to execute a function with a runtime database client.
func withRuntimeDBClientContext(_ context.Context, dbProvider provider.DBProviderInterface,
	fn func(provider.DBClientInterface) error) error {
	dbClient, err := dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	return fn(dbClient)
}

// buildFlowContextFromResultRow builds a FlowContextDB from a database result row.
func (s *flowStore) buildFlowContextFromResultRow(row map[string]interface{}) (*FlowContextDB, error) {
	id, ok := row["flow_id"].(string)
	if !ok {
		return nil, errors.New("failed to parse id as string")
	}

	contextStr := s.parseRequiredString(row["context"])
	if contextStr == nil {
		return nil, errors.New("failed to parse context as string")
	}

	expiryTime, err := s.parseTimeField(row["expiry_time"], "expiry_time")
	if err != nil {
		return nil, err
	}

	return &FlowContextDB{
		ExecutionID: id,
		Context:     *contextStr,
		ExpiryTime:  expiryTime,
	}, nil
}

// parseRequiredString parses a required string field from the database row.
func (s *flowStore) parseRequiredString(value interface{}) *string {
	if value == nil {
		return nil
	}
	if str, ok := value.(string); ok {
		return &str
	}
	// Handle []byte type (PostgreSQL may return TEXT/JSON as []byte)
	if bytes, ok := value.([]byte); ok {
		str := string(bytes)
		return &str
	}
	return nil
}

// parseTimeField safely parses a time field from the database row handling multiple formats.
// This follows the pattern used in other stores for consistency.
func (s *flowStore) parseTimeField(field interface{}, fieldName string) (time.Time, error) {
	const customTimeFormat = "2006-01-02 15:04:05.999999999"

	switch v := field.(type) {
	case string:
		// Handle SQLite datetime strings
		trimmedTime := s.trimTimeString(v)
		parsedTime, err := time.Parse(customTimeFormat, trimmedTime)
		if err != nil {
			// Try alternative ISO 8601 format as fallback
			parsedTime, err = time.Parse(time.RFC3339, v)
			if err != nil {
				return time.Time{}, fmt.Errorf("error parsing %s: %w", fieldName, err)
			}
		}
		return parsedTime, nil
	case time.Time:
		return v, nil
	case nil:
		return time.Time{}, fmt.Errorf("%s is nil", fieldName)
	default:
		return time.Time{}, fmt.Errorf("unexpected type for %s: %T", fieldName, field)
	}
}

// trimTimeString trims extra information from a time string to match the expected format.
func (s *flowStore) trimTimeString(timeStr string) string {
	parts := strings.SplitN(timeStr, " ", 3)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return timeStr
}
