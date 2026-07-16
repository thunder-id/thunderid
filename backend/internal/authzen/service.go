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

// Package authzen exposes AuthZEN authorization API endpoints backed by Thunder authorization.
package authzen

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// AuthZENServiceInterface defines AuthZEN access evaluation operations.
type AuthZENServiceInterface interface {
	// EvaluateAccess evaluates a single AuthZEN access request.
	EvaluateAccess(ctx context.Context, request AccessEvaluationRequest) (
		*AccessEvaluationResponse, *tidcommon.ServiceError)
	// EvaluateAccessBatch evaluates multiple AuthZEN access requests.
	EvaluateAccessBatch(ctx context.Context, request AccessEvaluationsRequest) (
		*AccessEvaluationsResponse, *tidcommon.ServiceError)
	// SearchActions returns the actions allowed for a subject and resource.
	SearchActions(ctx context.Context, request AccessActionSearchRequest) (
		*AccessSearchResponse, *tidcommon.ServiceError)
}

// authzenService adapts AuthZEN requests to Thunder authorization services.
type authzenService struct {
	authzService    providers.AuthorizationProvider
	entityProvider  entityprovider.EntityProviderInterface
	resourceService resource.ResourceServiceInterface
	logger          *log.Logger
}

// newService creates an AuthZEN service with its dependent services.
func newService(
	authzService providers.AuthorizationProvider,
	entityProvider entityprovider.EntityProviderInterface,
	resourceService resource.ResourceServiceInterface,
) AuthZENServiceInterface {
	return &authzenService{
		authzService:    authzService,
		entityProvider:  entityProvider,
		resourceService: resourceService,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthZENService")),
	}
}

// EvaluateAccess evaluates one AuthZEN access request against Thunder authorization.
func (s *authzenService) EvaluateAccess(ctx context.Context, request AccessEvaluationRequest) (
	*AccessEvaluationResponse, *tidcommon.ServiceError) {
	if svcErr := validateEvaluationRequest(request); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := s.validateSubject(ctx, request.Subject); svcErr != nil {
		return nil, svcErr
	}

	resourceServerID, svcErr := s.resolveResourceServerID(ctx, request.Resource.Type)
	if svcErr != nil {
		if svcErr.Code == ErrorInvalidResource.Code {
			return &AccessEvaluationResponse{
				Decision: false,
				Context:  buildEvaluationErrorContext("Resource not found"),
			}, nil
		}
		return nil, svcErr
	}

	if svcErr := s.validateAction(ctx, resourceServerID, request.Action.Name); svcErr != nil {
		if svcErr.Code == ErrorInvalidAction.Code {
			return &AccessEvaluationResponse{
				Decision: false,
				Context:  buildInvalidActionContext(request.Action.Name),
			}, nil
		}
		return nil, svcErr
	}

	groupIDs, svcErr := s.resolveGroupIDs(ctx, request.Subject.ID)
	if svcErr != nil {
		return nil, svcErr
	}

	authzResp, svcErr := s.authzService.EvaluateAccess(
		ctx, toAuthzAccessEvaluationRequest(request, groupIDs, resourceServerID))
	if svcErr != nil {
		s.logger.Error(ctx, "Authorization evaluation failed",
			log.MaskedString(log.LoggerKeyUserID, request.Subject.ID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	return &AccessEvaluationResponse{
		Decision: authzResp.Decision,
		Context:  buildDecisionContext(authzResp.Decision, authzResp.Context),
	}, nil
}

// EvaluateAccessBatch evaluates multiple AuthZEN access requests and preserves request order.
func (s *authzenService) EvaluateAccessBatch(ctx context.Context, request AccessEvaluationsRequest) (
	*AccessEvaluationsResponse, *tidcommon.ServiceError) {
	if len(request.Evaluations) == 0 {
		return nil, &ErrorMissingEvaluations
	}

	authzEvaluations := make([]providers.AccessEvaluationRequest, 0, len(request.Evaluations))
	responses := make([]AccessEvaluationResponse, len(request.Evaluations))
	authzEvaluationIndexes := make([]int, 0, len(request.Evaluations))
	groupIDsBySubject := make(map[string][]string)
	resourceServerIDsByIdentifier := make(map[string]string)
	validSubjects := make(map[string]struct{})
	validActions := make(map[string]struct{})
	for i, evaluation := range request.Evaluations {
		if svcErr := validateEvaluationRequest(evaluation); svcErr != nil {
			responses[i] = AccessEvaluationResponse{
				Decision: false,
				Context:  buildEvaluationErrorContext(svcErr.Error.DefaultValue),
			}
			continue
		}

		resourceServerID, ok := resourceServerIDsByIdentifier[evaluation.Resource.Type]
		if !ok {
			resolvedResourceServerID, svcErr := s.resolveResourceServerID(ctx, evaluation.Resource.Type)
			if svcErr != nil {
				if svcErr.Code == ErrorInvalidResource.Code {
					responses[i] = AccessEvaluationResponse{
						Decision: false,
						Context:  buildEvaluationErrorContext("Resource not found"),
					}
					continue
				}
				return nil, svcErr
			}
			resourceServerIDsByIdentifier[evaluation.Resource.Type] = resolvedResourceServerID
			resourceServerID = resolvedResourceServerID
		}

		subjectKey := evaluation.Subject.Type + ":" + evaluation.Subject.ID
		if _, ok := validSubjects[subjectKey]; !ok {
			if svcErr := s.validateSubject(ctx, evaluation.Subject); svcErr != nil {
				if svcErr.Code == ErrorInvalidSubject.Code {
					responses[i] = AccessEvaluationResponse{
						Decision: false,
						Context:  buildEvaluationErrorContext(svcErr.Error.DefaultValue),
					}
					continue
				}
				return nil, svcErr
			}
			validSubjects[subjectKey] = struct{}{}
		}

		actionKey := resourceServerID + ":" + evaluation.Action.Name
		if _, ok := validActions[actionKey]; !ok {
			if svcErr := s.validateAction(ctx, resourceServerID, evaluation.Action.Name); svcErr != nil {
				if svcErr.Code == ErrorInvalidAction.Code {
					responses[i] = AccessEvaluationResponse{
						Decision: false,
						Context:  buildInvalidActionContext(evaluation.Action.Name),
					}
					continue
				}
				return nil, svcErr
			}
			validActions[actionKey] = struct{}{}
		}

		groupIDs, ok := groupIDsBySubject[evaluation.Subject.ID]
		if !ok {
			resolvedGroupIDs, svcErr := s.resolveGroupIDs(ctx, evaluation.Subject.ID)
			if svcErr != nil {
				responses[i] = AccessEvaluationResponse{
					Decision: false,
					Context:  buildEvaluationErrorContext(svcErr.Error.DefaultValue),
				}
				continue
			}
			groupIDsBySubject[evaluation.Subject.ID] = resolvedGroupIDs
			groupIDs = resolvedGroupIDs
		}
		authzEvaluations = append(
			authzEvaluations, toAuthzAccessEvaluationRequest(evaluation, groupIDs, resourceServerID))
		authzEvaluationIndexes = append(authzEvaluationIndexes, i)
	}

	if len(authzEvaluations) == 0 {
		return &AccessEvaluationsResponse{
			Evaluations: responses,
		}, nil
	}

	authzResp, svcErr := s.authzService.EvaluateAccessBatch(ctx, providers.AccessEvaluationsRequest{
		Evaluations: authzEvaluations,
	})
	if svcErr != nil {
		s.logger.Error(ctx, "Authorization batch evaluation failed",
			log.Int("evaluationCount", len(request.Evaluations)),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	for i, evaluation := range authzResp.Evaluations {
		if i < len(authzEvaluationIndexes) {
			responses[authzEvaluationIndexes[i]] = AccessEvaluationResponse{
				Decision: evaluation.Decision,
				Context:  buildDecisionContext(evaluation.Decision, evaluation.Context),
			}
		}
	}

	return &AccessEvaluationsResponse{
		Evaluations: responses,
	}, nil
}

// SearchActions returns actions the subject is authorized to perform on the resource.
func (s *authzenService) SearchActions(ctx context.Context, request AccessActionSearchRequest) (
	*AccessSearchResponse, *tidcommon.ServiceError) {
	if strings.TrimSpace(request.Subject.ID) == "" {
		return nil, &ErrorMissingSubject
	}
	if strings.TrimSpace(request.Resource.Type) == "" {
		return nil, &ErrorMissingResource
	}

	resourceServerID, svcErr := s.resolveResourceServerID(ctx, request.Resource.Type)
	if svcErr != nil {
		return nil, svcErr
	}

	groupIDs, svcErr := s.resolveGroupIDs(ctx, request.Subject.ID)
	if svcErr != nil {
		return nil, svcErr
	}

	actions, svcErr := s.getAllPermissionActions(ctx, resourceServerID)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to retrieve resource server actions",
			log.String("resourceServerID", resourceServerID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	requestedPermissions := make([]string, 0, len(actions))
	actionByPermission := make(map[string]Action, len(actions))
	for _, action := range actions {
		if action.Permission == "" {
			continue
		}
		requestedPermissions = append(requestedPermissions, action.Permission)
		actionByPermission[action.Permission] = Action{Name: action.Permission}
	}

	authzEvaluations := make([]providers.AccessEvaluationRequest, 0, len(requestedPermissions))
	for _, permission := range requestedPermissions {
		authzEvaluations = append(authzEvaluations, providers.AccessEvaluationRequest{
			Subject: providers.Subject{
				ID:         request.Subject.ID,
				Type:       request.Subject.Type,
				GroupIDs:   groupIDs,
				Properties: request.Subject.Properties,
			},
			ResourceServer: providers.AccessEvaluationResourceServer{
				ID:         resourceServerID,
				Properties: request.Resource.Properties,
			},
			Permission: providers.Permission{
				Name: permission,
			},
			Context: request.Context,
		})
	}

	authzResp, svcErr := s.authzService.EvaluateAccessBatch(ctx, providers.AccessEvaluationsRequest{
		Evaluations: authzEvaluations,
	})
	if svcErr != nil {
		s.logger.Error(ctx, "Authorization action search failed",
			log.MaskedString(log.LoggerKeyUserID, request.Subject.ID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	results := make([]Action, 0, len(authzResp.Evaluations))
	for i, evaluation := range authzResp.Evaluations {
		if evaluation.Decision && i < len(requestedPermissions) {
			if action, ok := actionByPermission[requestedPermissions[i]]; ok {
				results = append(results, action)
			}
		}
	}
	return &AccessSearchResponse{Results: results}, nil
}

// getAllPermissionActions returns unique permission-backed actions for a resource server.
func (s *authzenService) getAllPermissionActions(
	ctx context.Context,
	resourceServerID string,
) ([]providers.Action, *tidcommon.ServiceError) {
	actions := make([]providers.Action, 0)
	permissions := make(map[string]struct{})

	rsLevelActions, svcErr := s.getAllActions(ctx, resourceServerID, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	actions = appendUniquePermissionActions(actions, permissions, rsLevelActions)

	resources, svcErr := s.getAllResources(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	for _, res := range resources {
		resourceActions, svcErr := s.getAllActions(ctx, resourceServerID, &res.ID)
		if svcErr != nil {
			return nil, svcErr
		}
		actions = appendUniquePermissionActions(actions, permissions, resourceActions)
	}

	return actions, nil
}

// getAllActions retrieves every action page for a resource server or resource.
func (s *authzenService) getAllActions(
	ctx context.Context,
	resourceServerID string,
	resourceID *string,
) ([]providers.Action, *tidcommon.ServiceError) {
	actions := make([]providers.Action, 0)
	for offset := 0; ; {
		actionList, svcErr := s.resourceService.GetActionList(
			ctx, resourceServerID, resourceID, "", serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		if actionList == nil {
			return actions, nil
		}

		actions = append(actions, actionList.Actions...)
		if !hasNextPage(actionList.TotalResults, actionList.Count, len(actionList.Actions), offset) {
			break
		}
		offset += pageSize(actionList.Count, len(actionList.Actions))
	}
	return actions, nil
}

// getAllResources retrieves every resource page for a resource server.
func (s *authzenService) getAllResources(
	ctx context.Context,
	resourceServerID string,
) ([]providers.Resource, *tidcommon.ServiceError) {
	resources := make([]providers.Resource, 0)
	for offset := 0; ; {
		resourceList, svcErr := s.resourceService.GetResourceList(
			ctx, resourceServerID, nil, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		if resourceList == nil {
			return resources, nil
		}

		resources = append(resources, resourceList.Resources...)
		if !hasNextPage(resourceList.TotalResults, resourceList.Count, len(resourceList.Resources), offset) {
			break
		}
		offset += pageSize(resourceList.Count, len(resourceList.Resources))
	}
	return resources, nil
}

// hasNextPage reports whether another page should be requested.
func hasNextPage(totalResults int, count int, itemCount int, offset int) bool {
	size := pageSize(count, itemCount)
	if size == 0 {
		return false
	}
	if totalResults <= 0 {
		return itemCount == serverconst.MaxPageSize
	}
	return offset+size < totalResults
}

// pageSize returns the response count, falling back to the actual item count.
func pageSize(count int, itemCount int) int {
	if count > 0 {
		return count
	}
	return itemCount
}

// appendUniquePermissionActions appends actions whose permissions were not already seen.
func appendUniquePermissionActions(
	actions []providers.Action,
	permissions map[string]struct{},
	newActions []providers.Action,
) []providers.Action {
	for _, action := range newActions {
		if action.Permission == "" {
			continue
		}
		if _, ok := permissions[action.Permission]; ok {
			continue
		}
		permissions[action.Permission] = struct{}{}
		actions = append(actions, action)
	}
	return actions
}

// validateSubject verifies that the subject exists and matches its type.
func (s *authzenService) validateSubject(ctx context.Context, subject Subject) *tidcommon.ServiceError {
	if s.entityProvider == nil {
		return nil
	}
	if strings.TrimSpace(subject.Type) == "" {
		return nil
	}

	entity, err := s.entityProvider.GetEntity(subject.ID)
	if err != nil {
		if err.Code == entityprovider.ErrorCodeNotImplemented {
			return nil
		}
		if err.Code == entityprovider.ErrorCodeEntityNotFound {
			return &ErrorInvalidSubject
		}
		s.logger.Error(ctx, "Failed to validate subject",
			log.MaskedString(log.LoggerKeyUserID, subject.ID),
			log.String("error", err.Error()))
		return &tidcommon.InternalServerError
	}

	if entity == nil || entity.Category.String() != subject.Type {
		return &ErrorInvalidSubject
	}
	return nil
}

// validateAction verifies that an action is registered on the resource server.
func (s *authzenService) validateAction(
	ctx context.Context,
	resourceServerID string,
	actionName string,
) *tidcommon.ServiceError {
	if s.resourceService == nil {
		return nil
	}

	invalidPermissions, svcErr := s.resourceService.ValidatePermissions(ctx, resourceServerID, []string{actionName})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to validate action",
			log.String("resourceServerID", resourceServerID),
			log.String("error", svcErr.Error.DefaultValue))
		return &tidcommon.InternalServerError
	}
	if len(invalidPermissions) > 0 {
		return &ErrorInvalidAction
	}
	return nil
}

// validateEvaluationRequest validates required fields for a single access evaluation.
func validateEvaluationRequest(request AccessEvaluationRequest) *tidcommon.ServiceError {
	if strings.TrimSpace(request.Subject.ID) == "" {
		return &ErrorMissingSubject
	}
	if strings.TrimSpace(request.Resource.Type) == "" {
		return &ErrorMissingResource
	}
	if strings.TrimSpace(request.Action.Name) == "" {
		return &ErrorMissingAction
	}
	return nil
}

// resolveResourceServerID resolves a resource server identifier to its internal ID.
func (s *authzenService) resolveResourceServerID(ctx context.Context, resourceServerIdentifier string) (
	string, *tidcommon.ServiceError) {
	if strings.TrimSpace(resourceServerIdentifier) == "" {
		return "", &ErrorMissingResource
	}

	if s.resourceService == nil {
		return resourceServerIdentifier, nil
	}

	resourceServer, svcErr := s.resourceService.GetResourceServerByIdentifier(ctx, resourceServerIdentifier)
	if svcErr != nil {
		if svcErr.Code == resource.ErrorResourceServerNotFound.Code {
			return "", &ErrorInvalidResource
		}
		s.logger.Error(ctx, "Failed to retrieve resource server by identifier",
			log.String("resourceServerIdentifier", resourceServerIdentifier),
			log.String("error", svcErr.Error.DefaultValue))
		return "", &tidcommon.InternalServerError
	}

	return resourceServer.ID, nil
}

// resolveGroupIDs returns the transitive group IDs for an entity.
func (s *authzenService) resolveGroupIDs(ctx context.Context, entityID string) ([]string, *tidcommon.ServiceError) {
	if s.entityProvider == nil {
		return []string{}, nil
	}

	groups, err := s.entityProvider.GetTransitiveEntityGroups(entityID)
	if err != nil {
		if err.Code == entityprovider.ErrorCodeNotImplemented {
			return []string{}, nil
		}
		s.logger.Error(ctx, "Failed to resolve entity groups",
			log.MaskedString(log.LoggerKeyUserID, entityID),
			log.String("error", err.Error()))
		return nil, &tidcommon.InternalServerError
	}

	groupIDs := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.ID != "" && !slices.Contains(groupIDs, group.ID) {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	return groupIDs, nil
}

// toAuthzAccessEvaluationRequest converts an AuthZEN request to an authorization request.
func toAuthzAccessEvaluationRequest(
	request AccessEvaluationRequest,
	groupIDs []string,
	resourceServerID string,
) providers.AccessEvaluationRequest {
	return providers.AccessEvaluationRequest{
		Subject: providers.Subject{
			Type:       request.Subject.Type,
			ID:         request.Subject.ID,
			GroupIDs:   groupIDs,
			Properties: request.Subject.Properties,
		},
		ResourceServer: providers.AccessEvaluationResourceServer{
			ID:         resourceServerID,
			Properties: request.Resource.Properties,
		},
		Permission: providers.Permission{
			Name:       request.Action.Name,
			Properties: request.Action.Properties,
		},
		Context: request.Context,
	}
}

// buildDecisionContext returns the engine context or a default denial context.
func buildDecisionContext(decision bool, context map[string]interface{}) map[string]interface{} {
	if context != nil {
		return context
	}
	if decision {
		return nil
	}
	return map[string]interface{}{
		"reason": "Subject is not authorized to perform the requested action",
	}
}

// buildInvalidActionContext returns denial context for an action missing from the resource server.
func buildInvalidActionContext(actionName string) map[string]interface{} {
	return buildEvaluationErrorContext(
		fmt.Sprintf("Action %s is not registered on the resource server", actionName))
}

// buildEvaluationErrorContext returns a structured error context for an evaluation response.
func buildEvaluationErrorContext(message string) map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
		},
	}
}
