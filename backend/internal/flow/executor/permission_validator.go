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

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

func getDefaultRequiredScope() string {
	if p := security.GetSystemPermissions(); p != nil {
		return p.Root
	}
	return security.UninitializedPermissionSentinel
}

// permissionValidator validates that the request has the required permission/scope to access the next node.
type permissionValidator struct {
	core.ExecutorInterface
	logger *log.Logger
}

// newPermissionValidator creates a new permission validator executor.
func newPermissionValidator(flowFactory core.FlowFactoryInterface) *permissionValidator {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PermissionValidator"))
	base := flowFactory.CreateExecutor(
		ExecutorNamePermissionValidator,
		common.ExecutorTypeUtility,
		[]common.Input{},
		[]common.Input{},
	)
	return &permissionValidator{
		ExecutorInterface: base,
		logger:            logger,
	}
}

// Execute validates that the request has the required permission/scope to access the next node.
func (e *permissionValidator) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// Get required scopes from node properties.
	requiredScopes := e.getRequiredScopes(ctx)

	logger.Debug("Checking scope protection", log.Any("requiredScopes", requiredScopes))

	// Check if context exists
	if ctx.Context == nil {
		logger.Debug("No context available - blocking access")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Insufficient permissions"
		return execResp, nil
	}

	// Extract permissions from request context
	userPermissions := security.GetPermissions(ctx.Context)
	logger.Debug("Extracted permissions from context",
		log.Int("permissionCount", len(userPermissions)),
		log.String("permissions", strings.Join(userPermissions, ", ")))

	// Check if any of the required permissions are present
	if !slices.ContainsFunc(requiredScopes, func(reqScope string) bool {
		return slices.Contains(userPermissions, reqScope)
	}) {
		logger.Debug("Request lacks required scope",
			log.Any("requiredScopes", requiredScopes))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Insufficient permissions"
		return execResp, nil
	}

	logger.Debug("Scope protection passed", log.Any("requiredScopes", requiredScopes))
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// getRequiredScopes retrieves the required scopes from the node context properties.
func (e *permissionValidator) getRequiredScopes(ctx *core.NodeContext) []string {
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
