// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authz provides authorization service functionality.
package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/resource"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	userpkg "github.com/thunder-id/thunderid/internal/user"
)

const loggerComponentName = "AuthorizationService"

// authorizationService is the default implementation of providers.AuthorizationProvider.
type authorizationService struct {
	engine          engine.AuthorizationEngine
	userService     userpkg.UserServiceInterface
	resourceService resource.ResourceServiceInterface
	httpClient      httpservice.HTTPClientInterface
}

// newAuthorizationService creates a new instance of authorizationService.
func newAuthorizationService(
	engine engine.AuthorizationEngine,
	resourceService resource.ResourceServiceInterface,
	userService userpkg.UserServiceInterface,
	httpClient httpservice.HTTPClientInterface,
) providers.AuthorizationProvider {
	return &authorizationService{
		engine:          engine,
		userService:     userService,
		resourceService: resourceService,
		httpClient:      httpClient,
	}
}

// EvaluateAccess evaluates a single fine-grained access request.
func (s *authorizationService) EvaluateAccess(
	ctx context.Context,
	request providers.AccessEvaluationRequest,
) (*providers.AccessEvaluationResponse, *tidcommon.ServiceError) {
	response, svcErr := s.EvaluateAccessBatch(ctx, providers.AccessEvaluationsRequest{
		Evaluations: []providers.AccessEvaluationRequest{request},
	})
	if svcErr != nil {
		return nil, svcErr
	}
	if len(response.Evaluations) == 0 {
		return &providers.AccessEvaluationResponse{}, nil
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch evaluates multiple fine-grained access requests.
func (s *authorizationService) EvaluateAccessBatch(
	ctx context.Context,
	request providers.AccessEvaluationsRequest,
) (*providers.AccessEvaluationsResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Evaluating authorization request",
		log.Int("evaluationCount", len(request.Evaluations)))

	if len(request.Evaluations) == 0 {
		return &providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{},
		}, nil
	}
	enrichedRequest, svcErr := s.enrichRequest(ctx, request)
	if svcErr != nil {
		return nil, svcErr
	}

	evaluationResp, err := s.evaluateWithResolvedEngines(ctx, toEngineAccessEvaluationsRequest(enrichedRequest))
	if err != nil {
		logger.Error(ctx, "Authorization evaluation failed",
			log.Int("evaluationCount", len(request.Evaluations)),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Authorization evaluation completed",
		log.Int("evaluationCount", len(request.Evaluations)))

	return fromEngineAccessEvaluationsResponse(evaluationResp), nil
}

// evaluateWithResolvedEngines routes evaluations to external or default engines and preserves order.
func (s *authorizationService) evaluateWithResolvedEngines(
	ctx context.Context,
	request engine.AccessEvaluationsRequest,
) (*engine.AccessEvaluationsResponse, error) {
	if s.resourceService == nil {
		return s.engine.EvaluateAccessBatch(ctx, request)
	}
	responses := make([]engine.AccessEvaluationResponse, len(request.Evaluations))
	defaultRequest := engine.AccessEvaluationsRequest{}
	defaultIndexes := make([]int, 0, len(request.Evaluations))
	externalRequests := map[engine.AuthorizationEngine]engine.AccessEvaluationsRequest{}
	externalIndexes := map[engine.AuthorizationEngine][]int{}
	resolvedExternalEngines := map[string]engine.AuthorizationEngine{}
	resolvedExternalIdentifiers := map[string]string{}
	resolvedRouteKeys := map[string]bool{}

	for index, evaluation := range request.Evaluations {
		resourceServerKey := resourceServerRouteKey(evaluation.ResourceServer)
		externalEngine := resolvedExternalEngines[resourceServerKey]
		resourceServerIdentifier := resolvedExternalIdentifiers[resourceServerKey]
		ok := externalEngine != nil
		if !resolvedRouteKeys[resourceServerKey] {
			var err error
			externalEngine, resourceServerIdentifier, ok, err =
				s.resolveEngine(ctx, evaluation.ResourceServer)
			if err != nil {
				return nil, err
			}
			resolvedRouteKeys[resourceServerKey] = true
			if ok {
				resolvedExternalEngines[resourceServerKey] = externalEngine
				resolvedExternalIdentifiers[resourceServerKey] = resourceServerIdentifier
			}
		}
		if !ok {
			defaultRequest.Evaluations = append(defaultRequest.Evaluations, evaluation)
			defaultIndexes = append(defaultIndexes, index)
			continue
		}
		evaluation.ResourceServer.Identifier = resourceServerIdentifier
		externalRequest := externalRequests[externalEngine]
		externalRequest.Evaluations = append(externalRequest.Evaluations, evaluation)
		externalRequests[externalEngine] = externalRequest
		externalIndexes[externalEngine] = append(externalIndexes[externalEngine], index)
	}

	for externalEngine, externalRequest := range externalRequests {
		indexes := externalIndexes[externalEngine]
		if err := evaluateResolvedBatch(ctx, externalEngine, externalRequest, indexes, responses); err != nil {
			return nil, err
		}
	}
	if err := evaluateResolvedBatch(ctx, s.engine, defaultRequest, defaultIndexes, responses); err != nil {
		return nil, err
	}

	return &engine.AccessEvaluationsResponse{Evaluations: responses}, nil
}

// evaluateResolvedBatch evaluates a group of requests and places responses at their original indexes.
func evaluateResolvedBatch(
	ctx context.Context,
	authorizationEngine engine.AuthorizationEngine,
	request engine.AccessEvaluationsRequest,
	indexes []int,
	responses []engine.AccessEvaluationResponse,
) error {
	if len(request.Evaluations) == 0 || authorizationEngine == nil {
		return nil
	}
	engineResponse, err := authorizationEngine.EvaluateAccessBatch(ctx, request)
	if err != nil {
		return err
	}
	for index, evaluation := range engineResponse.Evaluations {
		if index < len(indexes) {
			responses[indexes[index]] = evaluation
		}
	}
	return nil
}

// resolveEngine resolves the authorization engine configured for a resource server.
func (s *authorizationService) resolveEngine(
	ctx context.Context,
	resourceServer engine.ResourceServer,
) (engine.AuthorizationEngine, string, bool, error) {
	resourceServerKey := resourceServerRouteKey(resourceServer)
	if resourceServerKey == "" {
		return nil, "", false, nil
	}
	return s.resolveConnectionExternalEngine(ctx, resourceServer)
}

// resolveConnectionExternalEngine creates an external AuthZEN engine from a resource server connection.
func (s *authorizationService) resolveConnectionExternalEngine(
	ctx context.Context,
	requestResourceServer engine.ResourceServer,
) (engine.AuthorizationEngine, string, bool, error) {
	if s.resourceService == nil {
		return nil, "", false, nil
	}
	resourceServer, svcErr := s.getResourceServer(ctx, requestResourceServer)
	if svcErr != nil {
		if svcErr.Code == resource.ErrorResourceServerNotFound.Code {
			return nil, "", false, nil
		}
		return nil, "", false,
			fmt.Errorf("failed to resolve resource server: %s", svcErr.Error.DefaultValue)
	}
	if resourceServer == nil ||
		resourceServer.AuthorizationEngine.Type != providers.AuthorizationEngineTypeExternalAuthZENPDP {
		return nil, "", false, nil
	}
	connectionID := strings.TrimSpace(resourceServer.AuthorizationEngine.Properties.PDPConnectionID)
	if connectionID == "" {
		return nil, "", false, nil
	}
	externalEngine, ok, err := engine.NewAuthZENPDP(ctx, connectionID, s.httpClient)
	return externalEngine, strings.TrimSpace(resourceServer.Identifier), ok, err
}

// resourceServerRouteKey returns the external routing key for a resource server.
func resourceServerRouteKey(resourceServer engine.ResourceServer) string {
	return strings.TrimSpace(resourceServer.ID)
}

// getResourceServer retrieves the persisted resource server used for engine resolution.
func (s *authorizationService) getResourceServer(
	ctx context.Context,
	resourceServer engine.ResourceServer,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	return s.resourceService.GetResourceServer(ctx, strings.TrimSpace(resourceServer.ID))
}

// enrichRequest adds persisted user attributes to user subject evaluations.
func (s *authorizationService) enrichRequest(
	ctx context.Context,
	request providers.AccessEvaluationsRequest,
) (providers.AccessEvaluationsRequest, *tidcommon.ServiceError) {
	if s.userService == nil {
		return request, nil
	}
	enriched := providers.AccessEvaluationsRequest{
		Evaluations: make([]providers.AccessEvaluationRequest, 0, len(request.Evaluations)),
	}
	users := make(map[string]*providers.User)
	for _, evaluation := range request.Evaluations {
		if evaluation.Subject.Type != providers.EntityCategoryUser.String() {
			enriched.Evaluations = append(enriched.Evaluations, evaluation)
			continue
		}
		user, ok := users[evaluation.Subject.ID]
		if !ok {
			var svcErr *tidcommon.ServiceError
			user, svcErr = s.userService.GetUser(ctx, evaluation.Subject.ID, false)
			if svcErr != nil {
				if svcErr.Code != userpkg.ErrorUserNotFound.Code {
					return providers.AccessEvaluationsRequest{}, &tidcommon.InternalServerError
				}
				user = nil
			}
			users[evaluation.Subject.ID] = user
		}
		if user != nil {
			properties := map[string]interface{}{}
			if len(user.Attributes) > 0 {
				if err := json.Unmarshal(user.Attributes, &properties); err != nil {
					return providers.AccessEvaluationsRequest{}, &tidcommon.InternalServerError
				}
			}
			if user.OUID != "" {
				properties["ouId"] = user.OUID
			}
			for key, value := range evaluation.Subject.Properties {
				properties[key] = value
			}
			evaluation.Subject.Properties = properties
		}
		enriched.Evaluations = append(enriched.Evaluations, evaluation)
	}
	return enriched, nil
}

// toEngineAccessEvaluationsRequest converts provider evaluations to engine evaluations.
func toEngineAccessEvaluationsRequest(request providers.AccessEvaluationsRequest) engine.AccessEvaluationsRequest {
	evaluations := make([]engine.AccessEvaluationRequest, 0, len(request.Evaluations))
	for _, evaluation := range request.Evaluations {
		evaluations = append(evaluations, engine.AccessEvaluationRequest{
			Subject: engine.Subject{
				Type:       evaluation.Subject.Type,
				ID:         evaluation.Subject.ID,
				GroupIDs:   evaluation.Subject.GroupIDs,
				Properties: evaluation.Subject.Properties,
			},
			ResourceServer: engine.ResourceServer{
				ID:         evaluation.ResourceServer.ID,
				ResourceID: evaluation.ResourceServer.ResourceID,
				Properties: evaluation.ResourceServer.Properties,
			},
			Permission: engine.Permission{
				Name:       evaluation.Permission.Name,
				Properties: evaluation.Permission.Properties,
			},
			Context: evaluation.Context,
		})
	}
	return engine.AccessEvaluationsRequest{Evaluations: evaluations}
}

// fromEngineAccessEvaluationsResponse converts engine responses to provider responses.
func fromEngineAccessEvaluationsResponse(
	response *engine.AccessEvaluationsResponse) *providers.AccessEvaluationsResponse {
	if response == nil {
		return &providers.AccessEvaluationsResponse{Evaluations: []providers.AccessEvaluationResponse{}}
	}

	evaluations := make([]providers.AccessEvaluationResponse, 0, len(response.Evaluations))
	for _, evaluation := range response.Evaluations {
		evaluations = append(evaluations, providers.AccessEvaluationResponse{
			Decision: evaluation.Decision,
			Context:  evaluation.Context,
		})
	}
	return &providers.AccessEvaluationsResponse{Evaluations: evaluations}
}
