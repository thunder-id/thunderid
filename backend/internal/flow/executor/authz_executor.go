/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
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
	authnProvider  authnprovidermgr.AuthnProviderManagerInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*authorizationExecutor)(nil)

// newAuthorizationExecutor creates a new instance of AuthorizationExecutor.
func newAuthorizationExecutor(
	flowFactory core.FlowFactoryInterface,
	authZService authzsvc.AuthorizationServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
) *authorizationExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, authzLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAuthorization))

	base := flowFactory.CreateExecutor(ExecutorNameAuthorization, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{})

	return &authorizationExecutor{
		ExecutorInterface: base,
		authzService:      authZService,
		entityProvider:    entityProvider,
		authnProvider:     authnProvider,
		logger:            logger,
	}
}

// Execute executes the authorization logic by determining required permissions based on context,
// calling the authorization service, and storing authorized permissions in runtime data.
func (a *authorizationExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing authorization executor")

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
		AuthUser:    ctx.AuthUser,
	}

	if !execResp.AuthUser.IsAuthenticated() && ctx.FlowType == common.FlowTypeRegistration {
		logger.Debug(ctx.Context,
			"Sending executor complete response for unauthenticated user in registration flow")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	if !execResp.AuthUser.IsAuthenticated() {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	authUser, entityRef, svcErr := a.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}

	// Determine required permissions
	requestedPerms := extractRequestedPermissions(ctx)

	if len(requestedPerms) == 0 {
		logger.Debug(ctx.Context, "No permissions to check, returning empty permissions")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	logger.Debug(ctx.Context, "Determined required permissions", log.Int("count", len(requestedPerms)))

	// Extract user ID and group IDs
	userID := entityRef.EntityID
	groupIDs, err := a.extractGroupIDs(ctx, userID)
	if err != nil {
		return nil, errors.Join(errors.New("Failed to extract group IDs"), err)
	}

	logger.Debug(ctx.Context, "Calling authorization service",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.Int("groupCount", len(groupIDs)),
		log.Int("permissionCount", len(requestedPerms)))

	authzResp, svcErr := a.authzService.EvaluateAccessBatch(ctx.Context,
		a.buildAccessEvaluationsRequest(userID, groupIDs, requestedPerms))
	if svcErr != nil {
		logger.Error(ctx.Context, "Authorization service call failed",
			log.String("error", svcErr.Error.DefaultValue))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrAuthorizationFailed
		return execResp, nil
	}

	authorizedPermissions := a.filterAuthorizedPermissions(requestedPerms, authzResp.Evaluations)
	setAuthorizedPermissions(execResp, authorizedPermissions)
	logger.Debug(ctx.Context, "Authorization completed successfully",
		log.Int("authorizedCount", len(authorizedPermissions)))

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

// buildAccessEvaluationsRequest builds the authorization service request for the requested permissions.
func (a *authorizationExecutor) buildAccessEvaluationsRequest(
	entityID string,
	groupIDs []string,
	requestedPermissions []string,
) authzsvc.AccessEvaluationsRequest {
	evaluations := make([]authzsvc.AccessEvaluationRequest, 0, len(requestedPermissions))
	for _, permission := range requestedPermissions {
		evaluations = append(evaluations, authzsvc.AccessEvaluationRequest{
			Subject: authzsvc.Subject{
				ID:       entityID,
				GroupIDs: groupIDs,
			},
			Permission: authzsvc.Permission{Name: permission},
		})
	}
	return authzsvc.AccessEvaluationsRequest{Evaluations: evaluations}
}

// filterAuthorizedPermissions returns the requested permissions that were allowed by the authorization service.
func (a *authorizationExecutor) filterAuthorizedPermissions(
	requestedPermissions []string,
	evaluations []authzsvc.AccessEvaluationResponse,
) []string {
	authorizedPermissions := make([]string, 0, len(evaluations))
	for i, evaluation := range evaluations {
		if evaluation.Decision && i < len(requestedPermissions) {
			authorizedPermissions = append(authorizedPermissions, requestedPermissions[i])
		}
	}
	return authorizedPermissions
}

// extractGroupIDs extracts group IDs from the authenticated user's attributes or runtime data.
// If neither provides group information, it fetches them using the user service.
func (a *authorizationExecutor) extractGroupIDs(ctx *core.NodeContext, userID string) ([]string, error) {
	// Try to get groups from runtime data (JSON array string)
	if groupsJSON, ok := ctx.RuntimeData[userAttributeGroups]; ok && groupsJSON != "" {
		var groups []string
		if err := json.Unmarshal([]byte(groupsJSON), &groups); err == nil {
			return groups, nil
		}
	}

	// If no groups found in context, fetch transitive groups from entity provider
	if a.entityProvider != nil && userID != "" {
		a.logger.Debug(ctx.Context,
			"Groups not found in context, fetching transitive groups from entity provider",
			log.MaskedString(log.LoggerKeyUserID, userID))

		groups, err := a.entityProvider.GetTransitiveEntityGroups(userID)
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
