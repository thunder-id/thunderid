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

package environmentvariable

import (
	"context"
	"errors"
	"regexp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const environmentVariableLoggerComponentName = "EnvironmentVariableService"

// environmentVariableKeyPattern restricts keys to valid environment-variable names so they can back
// the declarative placeholders (for example MY_APP_REDIRECT_URL) they resolve.
var environmentVariableKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// EnvironmentVariableServiceInterface defines environment variable management operations.
type EnvironmentVariableServiceInterface interface {
	CreateEnvironmentVariable(ctx context.Context, envID string,
		request CreateEnvironmentVariableRequest) (*EnvironmentVariable, *tidcommon.ServiceError)
	GetEnvironmentVariable(ctx context.Context, envID,
		id string) (*EnvironmentVariable, *tidcommon.ServiceError)
	GetEnvironmentVariableList(ctx context.Context, envID string, limit,
		offset int) (*EnvironmentVariableListResponse, *tidcommon.ServiceError)
	UpdateEnvironmentVariable(ctx context.Context, envID, id string,
		request UpdateEnvironmentVariableRequest) (*EnvironmentVariable, *tidcommon.ServiceError)
	DeleteEnvironmentVariable(ctx context.Context, envID, id string) *tidcommon.ServiceError
	// ResolveEnvironmentVariables returns every key mapped to its value for one environment. Used by
	// the config export/apply tooling to substitute declarative placeholders for a Data Plane.
	ResolveEnvironmentVariables(ctx context.Context,
		envID string) (map[string]string, *tidcommon.ServiceError)
}

// environmentVariableService is the default implementation of EnvironmentVariableServiceInterface.
type environmentVariableService struct {
	store environmentVariableStoreInterface
}

// newEnvironmentVariableService creates a new environmentVariableService.
func newEnvironmentVariableService(store environmentVariableStoreInterface) EnvironmentVariableServiceInterface {
	return &environmentVariableService{store: store}
}

// CreateEnvironmentVariable validates and stores a new environment variable.
func (s *environmentVariableService) CreateEnvironmentVariable(ctx context.Context, envID string,
	request CreateEnvironmentVariableRequest) (*EnvironmentVariable, *tidcommon.ServiceError) {
	if !environmentVariableKeyPattern.MatchString(request.Key) {
		return nil, &ErrorInvalidEnvironmentVariableRequest
	}

	existing, err := s.store.GetEnvironmentVariableByKey(ctx, envID, request.Key)
	if err != nil && !errors.Is(err, errEnvironmentVariableNotFound) {
		return nil, s.internalError(ctx, "failed to check environment variable key uniqueness", err)
	}
	if err == nil && existing.ID != "" {
		return nil, &ErrorEnvironmentVariableKeyConflict
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return nil, s.internalError(ctx, "failed to generate environment variable id", err)
	}

	created := EnvironmentVariable{
		ID:          id,
		Key:         request.Key,
		Value:       request.Value,
		Description: request.Description,
	}
	if err := s.store.CreateEnvironmentVariable(ctx, envID, created); err != nil {
		return nil, s.internalError(ctx, "failed to create environment variable", err)
	}

	return &created, nil
}

// GetEnvironmentVariable returns an environment variable by id, including its value.
func (s *environmentVariableService) GetEnvironmentVariable(ctx context.Context,
	envID, id string) (*EnvironmentVariable, *tidcommon.ServiceError) {
	stored, err := s.store.GetEnvironmentVariableByID(ctx, envID, id)
	if err != nil {
		if errors.Is(err, errEnvironmentVariableNotFound) {
			return nil, &ErrorEnvironmentVariableNotFound
		}
		return nil, s.internalError(ctx, "failed to get environment variable", err)
	}
	return &stored, nil
}

// GetEnvironmentVariableList returns a paginated list of environment variables.
func (s *environmentVariableService) GetEnvironmentVariableList(ctx context.Context, envID string,
	limit, offset int) (*EnvironmentVariableListResponse, *tidcommon.ServiceError) {
	total, err := s.store.GetEnvironmentVariableCount(ctx, envID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to count environment variables", err)
	}

	variables, err := s.store.GetEnvironmentVariableList(ctx, envID, limit, offset)
	if err != nil {
		return nil, s.internalError(ctx, "failed to list environment variables", err)
	}

	return &EnvironmentVariableListResponse{
		TotalResults:         total,
		Count:                len(variables),
		EnvironmentVariables: variables,
	}, nil
}

// UpdateEnvironmentVariable updates an environment variable's value and description.
func (s *environmentVariableService) UpdateEnvironmentVariable(ctx context.Context, envID, id string,
	request UpdateEnvironmentVariableRequest) (*EnvironmentVariable, *tidcommon.ServiceError) {
	err := s.store.UpdateEnvironmentVariableByID(ctx, envID, id, request.Description, request.Value)
	if err != nil {
		if errors.Is(err, errEnvironmentVariableNotFound) {
			return nil, &ErrorEnvironmentVariableNotFound
		}
		return nil, s.internalError(ctx, "failed to update environment variable", err)
	}

	return s.GetEnvironmentVariable(ctx, envID, id)
}

// DeleteEnvironmentVariable removes an environment variable by id.
func (s *environmentVariableService) DeleteEnvironmentVariable(ctx context.Context,
	envID, id string) *tidcommon.ServiceError {
	if err := s.store.DeleteEnvironmentVariableByID(ctx, envID, id); err != nil {
		if errors.Is(err, errEnvironmentVariableNotFound) {
			return &ErrorEnvironmentVariableNotFound
		}
		return s.internalError(ctx, "failed to delete environment variable", err)
	}
	return nil
}

// ResolveEnvironmentVariables returns every key mapped to its value for one environment.
func (s *environmentVariableService) ResolveEnvironmentVariables(ctx context.Context,
	envID string) (map[string]string, *tidcommon.ServiceError) {
	values, err := s.store.GetEnvironmentVariableValues(ctx, envID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to read environment variable values", err)
	}
	return values, nil
}

// internalError logs the underlying error and returns the generic server-side ServiceError.
func (s *environmentVariableService) internalError(ctx context.Context, msg string,
	err error) *tidcommon.ServiceError {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, environmentVariableLoggerComponentName))
	logger.Error(ctx, msg, log.Error(err))
	return &ErrorInternalServer
}
