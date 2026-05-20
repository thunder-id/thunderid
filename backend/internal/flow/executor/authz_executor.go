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

package executor

import (
	"encoding/json"
	"errors"

	authzsvc "github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	authzLoggerComponentName = "AuthorizationExecutor"
	authorizedPermissionsKey = "authorized_permissions"
	requestedPermissionsKey  = "requested_permissions"
)

// authorizationExecutor implements the ExecutorInterface for performing authorization checks
// during flow execution. It enriches the flow context with authorized permissions.
type authorizationExecutor struct {
	core.ExecutorInterface
	authzService   authzsvc.AuthorizationServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*authorizationExecutor)(nil)

// newAuthorizationExecutor creates a new instance of AuthorizationExecutor.
func newAuthorizationExecutor(
	flowFactory core.FlowFactoryInterface,
	authZService authzsvc.AuthorizationServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *authorizationExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, authzLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAuthorization))

	base := flowFactory.CreateExecutor(ExecutorNameAuthorization, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{})

	return &authorizationExecutor{
		ExecutorInterface: base,
		authzService:      authZService,
		entityProvider:    entityProvider,
		logger:            logger,
	}
}

// Execute executes the authorization logic by determining required permissions based on context,
// calling the authorization service, and storing authorized permissions in runtime data.
func (a *authorizationExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing authorization executor")

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	if !ctx.AuthenticatedUser.IsAuthenticated && ctx.FlowType == common.FlowTypeRegistration {
		logger.Debug("Sending executor complete response for unauthenticated user in registration flow")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	if !ctx.AuthenticatedUser.IsAuthenticated {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = failureReasonUserNotAuthenticated
		return execResp, nil
	}

	// Determine required permissions
	requestedPerms := extractRequestedPermissions(ctx)

	if len(requestedPerms) == 0 {
		logger.Debug("No permissions to check, returning empty permissions")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	logger.Debug("Determined required permissions", log.Int("count", len(requestedPerms)))

	// Extract user ID and group IDs
	userID := ctx.AuthenticatedUser.UserID
	groupIDs, err := a.extractGroupIDs(ctx)
	if err != nil {
		return nil, errors.Join(errors.New("Failed to extract group IDs"), err)
	}

	logger.Debug("Calling authorization service",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.Int("groupCount", len(groupIDs)),
		log.Int("permissionCount", len(requestedPerms)))

	// Call authorization service
	authzReq := authzsvc.GetAuthorizedPermissionsRequest{
		EntityID:             userID,
		GroupIDs:             groupIDs,
		RequestedPermissions: requestedPerms,
	}

	authzResp, svcErr := a.authzService.GetAuthorizedPermissions(ctx.Context, authzReq)
	if svcErr != nil {
		logger.Error("Authorization service call failed", log.String("error", svcErr.Error.DefaultValue))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Authorization validation failure"
		return execResp, nil
	}

	setAuthorizedPermissions(execResp, authzResp.AuthorizedPermissions)
	logger.Debug("Authorization completed successfully",
		log.Int("authorizedCount", len(authzResp.AuthorizedPermissions)))

	execResp.Status = common.ExecComplete
	return execResp, nil
}

// extractRequestedPermissions extracts requested permissions from the context.
func extractRequestedPermissions(ctx *core.NodeContext) []string {
	requestedPermissions := ctx.RuntimeData[requestedPermissionsKey]
	if requestedPermissions != "" {
		return utils.ParseStringArray(requestedPermissions, " ")
	}
	requestedPermissions = ctx.UserInputs[requestedPermissionsKey]
	return utils.ParseStringArray(requestedPermissions, " ")
}

// setAuthorizedPermissions sets the authorized permissions in the executor response's runtime data.
func setAuthorizedPermissions(execResp *common.ExecutorResponse, authorizedPermissions []string) {
	execResp.RuntimeData[authorizedPermissionsKey] = utils.StringifyStringArray(authorizedPermissions, " ")
}

// extractGroupIDs extracts group IDs from the authenticated user's attributes or runtime data.
// If neither provides group information, it fetches them using the user service.
func (a *authorizationExecutor) extractGroupIDs(ctx *core.NodeContext) ([]string, error) {
	// Try to get groups from authenticated user attributes
	if groupsAttr, ok := ctx.AuthenticatedUser.Attributes[userAttributeGroups]; ok {
		// Handle different group attribute formats
		switch v := groupsAttr.(type) {
		case []string:
			return v, nil
		case []interface{}:
			groups := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					groups = append(groups, str)
				}
			}
			return groups, nil
		case string:
			// Single group as string
			return []string{v}, nil
		}
	}

	// Try to get groups from runtime data (JSON array string)
	if groupsJSON, ok := ctx.RuntimeData[userAttributeGroups]; ok && groupsJSON != "" {
		var groups []string
		if err := json.Unmarshal([]byte(groupsJSON), &groups); err == nil {
			return groups, nil
		}
	}

	// If no groups found in context, fetch transitive groups from entity provider
	if a.entityProvider != nil && ctx.AuthenticatedUser.UserID != "" {
		a.logger.Debug("Groups not found in context, fetching transitive groups from entity provider",
			log.MaskedString(log.LoggerKeyUserID, ctx.AuthenticatedUser.UserID))

		groups, err := a.entityProvider.GetTransitiveEntityGroups(ctx.AuthenticatedUser.UserID)
		if err != nil {
			return nil, err
		}
		groupIDs := make([]string, 0, len(groups))
		for _, g := range groups {
			groupIDs = append(groupIDs, g.ID)
		}
		return groupIDs, nil
	}

	// No groups found
	return []string{}, nil
}
