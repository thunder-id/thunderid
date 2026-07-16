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

package executor

import (
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

func getDefaultRequiredScope() string {
	return security.GetSystemRootPermission()
}

// permissionValidator validates that the request has the required permission/scope to access the next node.
type permissionValidator struct {
	providers.Executor
	logger *log.Logger
}

// newPermissionValidator creates a new permission validator executor.
func newPermissionValidator(flowFactory core.FlowFactoryInterface) *permissionValidator {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PermissionValidator"))
	base := flowFactory.CreateExecutor(
		ExecutorNamePermissionValidator,
		providers.ExecutorTypeUtility,
		[]providers.Input{},
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyRequiredScopes},
			},
		},
	)
	return &permissionValidator{
		Executor: base,
		logger:   logger,
	}
}

// Execute validates that the request has the required permission/scope to access the next node.
func (e *permissionValidator) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// Get required scopes from node properties.
	requiredScopes := e.getRequiredScopes(ctx)

	logger.Debug(ctx.Context, "Checking scope protection", log.Any("requiredScopes", requiredScopes))

	// Check if context exists
	if ctx.Context == nil {
		logger.Debug(ctx.Context, "No context available - blocking access")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInsufficientPermissions
		return execResp, nil
	}

	// Extract permissions from request context
	userPermissions := security.GetPermissions(ctx.Context)
	logger.Debug(ctx.Context, "Extracted permissions from context",
		log.Int("permissionCount", len(userPermissions)),
		log.String("permissions", strings.Join(userPermissions, ", ")))

	// Check if any of the required permissions are satisfied, using hierarchical
	// scope matching (e.g. a "system" permission satisfies a "system:user" requirement).
	if !slices.ContainsFunc(requiredScopes, func(reqScope string) bool {
		return security.HasSufficientPermission(userPermissions, reqScope)
	}) {
		logger.Debug(ctx.Context, "Request lacks required scope",
			log.Any("requiredScopes", requiredScopes))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInsufficientPermissions
		return execResp, nil
	}

	logger.Debug(ctx.Context, "Scope protection passed", log.Any("requiredScopes", requiredScopes))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// getRequiredScopes retrieves the required scopes from the node context properties.
func (e *permissionValidator) getRequiredScopes(ctx *providers.NodeContext) []string {
	requiredScopes := []string{getDefaultRequiredScope()}

	if ctx.NodeProperties != nil {
		if val, exists := ctx.NodeProperties[propertyKeyRequiredScopes]; exists {
			if v, ok := val.([]interface{}); ok {
				scopes := make([]string, 0, len(v))
				for _, item := range v {
					if s, ok := item.(string); ok && s != "" {
						scopes = append(scopes, s)
					}
				}

				if len(scopes) > 0 {
					requiredScopes = scopes
				}
			}
		}
	}

	return requiredScopes
}
