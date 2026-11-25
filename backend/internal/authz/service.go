// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authz provides authorization service functionality.
package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

const loggerComponentName = "AuthorizationService"

// authorizationService is the default implementation of providers.AuthorizationProvider.
type authorizationService struct {
	engine          engine.AuthorizationEngine
	entityService   entity.EntityServiceInterface
	resourceService resource.ResourceServiceInterface
	pdpEngine       engine.AuthorizationEngine
}

// newAuthorizationService creates a new instance of authorizationService.
func newAuthorizationService(
	defaultEngine engine.AuthorizationEngine,
	resourceService resource.ResourceServiceInterface,
	entityService entity.EntityServiceInterface,
	pdpEngine engine.AuthorizationEngine,
) providers.AuthorizationProvider {
	return &authorizationService{
		engine:          defaultEngine,
		entityService:   entityService,
		resourceService: resourceService,
		pdpEngine:       pdpEngine,
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

	responses := make([]providers.AccessEvaluationResponse, len(request.Evaluations))
	pending := providers.AccessEvaluationsRequest{
		Evaluations: make([]providers.AccessEvaluationRequest, 0, len(request.Evaluations)),
	}
	pendingIndexes := make([]int, 0, len(request.Evaluations))
	subjects := make(map[string]*providers.Entity)
	for i, evaluation := range request.Evaluations {
		granted, svcErr := s.grantsSelfServicePermission(ctx, evaluation, subjects)
		if svcErr != nil {
			return nil, svcErr
		}
		if granted {
			responses[i] = providers.AccessEvaluationResponse{Decision: true}
			continue
		}
		pending.Evaluations = append(pending.Evaluations, evaluation)
		pendingIndexes = append(pendingIndexes, i)
	}
	if len(pending.Evaluations) == 0 {
		return &providers.AccessEvaluationsResponse{Evaluations: responses}, nil
	}

	enrichedRequest, svcErr := s.enrichRequest(ctx, pending, subjects)
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

	evaluated := fromEngineAccessEvaluationsResponse(evaluationResp)
	for j, index := range pendingIndexes {
		if j < len(evaluated.Evaluations) {
			responses[index] = evaluated.Evaluations[j]
		}
	}
	return &providers.AccessEvaluationsResponse{Evaluations: responses}, nil
}

// grantsSelfServicePermission reports whether the evaluation asks for a self-service permission of the System
// resource server for an existing user, on no resource or on the user. A user holds those without a role: they
// act only on the user's own account. subjects caches the subjects already read.
func (s *authorizationService) grantsSelfServicePermission(
	ctx context.Context, evaluation providers.AccessEvaluationRequest, subjects map[string]*providers.Entity,
) (bool, *tidcommon.ServiceError) {
	if evaluation.ResourceServer.ID != serverconst.SystemResourceServerID ||
		evaluation.Subject.Category != providers.EntityCategoryUser.String() || evaluation.Subject.ID == "" ||
		(evaluation.ResourceServer.ResourceID != "" && evaluation.ResourceServer.ResourceID != evaluation.Subject.ID) ||
		!security.IsSelfServicePermission(evaluation.Permission.Name) || s.entityService == nil {
		return false, nil
	}
	subject, svcErr := s.resolveSubject(ctx, evaluation.Subject, subjects)
	return subject != nil, svcErr
}

// evaluateWithResolvedEngines routes evaluations to connection-backed or default engines and preserves order.
func (s *authorizationService) evaluateWithResolvedEngines(
	ctx context.Context,
	request engine.AccessEvaluationsRequest,
) (*engine.AccessEvaluationsResponse, error) {
	if s.resourceService == nil {
		return s.engine.EvaluateAccessBatch(ctx, request)
	}
	type evaluationBatch struct {
		request engine.AccessEvaluationsRequest
		indexes []int
	}

	responses := make([]engine.AccessEvaluationResponse, len(request.Evaluations))
	defaultBatch := evaluationBatch{
		indexes: make([]int, 0, len(request.Evaluations)),
	}
	engineBatches := map[engine.AuthorizationEngine]*evaluationBatch{}
	type resolvedRoute struct {
		resourceServer engine.ResourceServer
		engine         engine.AuthorizationEngine
	}
	routes := map[string]resolvedRoute{}

	for index, evaluation := range request.Evaluations {
		resourceServerKey := resourceServerRouteKey(evaluation.ResourceServer)
		resolved, ok := routes[resourceServerKey]
		if ok {
			evaluation.ResourceServer.Identifier = resolved.resourceServer.Identifier
			evaluation.ResourceServer.Engine = resolved.resourceServer.Engine
		} else {
			selectedEngine, err := s.resolveEngine(ctx, &evaluation.ResourceServer)
			if err != nil {
				return nil, err
			}
			resolved = resolvedRoute{
				resourceServer: evaluation.ResourceServer,
				engine:         selectedEngine,
			}
			routes[resourceServerKey] = resolved
		}

		selectedEngine := resolved.engine
		if selectedEngine == nil || selectedEngine == s.engine {
			defaultBatch.request.Evaluations = append(defaultBatch.request.Evaluations, evaluation)
			defaultBatch.indexes = append(defaultBatch.indexes, index)
			continue
		}
		batch := engineBatches[selectedEngine]
		if batch == nil {
			batch = &evaluationBatch{}
			engineBatches[selectedEngine] = batch
		}
		batch.request.Evaluations = append(batch.request.Evaluations, evaluation)
		batch.indexes = append(batch.indexes, index)
	}

	for selectedEngine, batch := range engineBatches {
		if err := evaluateResolvedBatch(
			ctx, selectedEngine.EvaluateAccessBatch, batch.request, batch.indexes, responses); err != nil {
			return nil, err
		}
	}
	if err := evaluateResolvedBatch(
		ctx, s.engine.EvaluateAccessBatch, defaultBatch.request, defaultBatch.indexes, responses); err != nil {
		return nil, err
	}

	return &engine.AccessEvaluationsResponse{Evaluations: responses}, nil
}

// evaluateResolvedBatch evaluates a group of requests and places responses at their original indexes.
func evaluateResolvedBatch(
	ctx context.Context,
	evaluate func(context.Context, engine.AccessEvaluationsRequest) (*engine.AccessEvaluationsResponse, error),
	request engine.AccessEvaluationsRequest,
	indexes []int,
	responses []engine.AccessEvaluationResponse,
) error {
	if len(request.Evaluations) == 0 || evaluate == nil {
		return nil
	}
	engineResponse, err := evaluate(ctx, request)
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

// resolveEngine selects the configured engine and attaches external-PDP routing metadata.
func (s *authorizationService) resolveEngine(
	ctx context.Context,
	requestResourceServer *engine.ResourceServer,
) (engine.AuthorizationEngine, error) {
	if strings.TrimSpace(requestResourceServer.ID) == "" || s.resourceService == nil {
		return s.engine, nil
	}
	resourceServer, svcErr := s.getResourceServer(ctx, *requestResourceServer)
	if svcErr != nil {
		if svcErr.Code == resource.ErrorResourceServerNotFound.Code {
			return s.engine, nil
		}
		return nil, fmt.Errorf("failed to resolve resource server: %s", svcErr.Error.DefaultValue)
	}
	if resourceServer == nil {
		return s.engine, nil
	}
	requestResourceServer.Identifier = resourceServer.Identifier
	requestResourceServer.Engine = engine.EngineConfig{
		Type: resourceServer.AuthorizationEngine.Type,
		Properties: engine.EngineProperties{
			PDPConnectionID: strings.TrimSpace(resourceServer.AuthorizationEngine.Properties.PDPConnectionID),
		},
	}
	switch requestResourceServer.Engine.Type {
	case "", providers.AuthorizationEngineTypeRBAC:
		return s.engine, nil
	case providers.AuthorizationEngineTypeAuthZENPDP:
		return s.pdpEngine, nil
	default:
		return nil, fmt.Errorf("unsupported authorization engine type %q", requestResourceServer.Engine.Type)
	}
}

// resourceServerRouteKey returns the engine-routing key for a resource server.
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

// enrichRequest adds persisted entity attributes to supported subject evaluations. subjects caches the subjects
// already read.
func (s *authorizationService) enrichRequest(
	ctx context.Context,
	request providers.AccessEvaluationsRequest,
	subjects map[string]*providers.Entity,
) (providers.AccessEvaluationsRequest, *tidcommon.ServiceError) {
	if s.entityService == nil {
		return request, nil
	}
	enriched := providers.AccessEvaluationsRequest{
		Evaluations: make([]providers.AccessEvaluationRequest, 0, len(request.Evaluations)),
	}
	for _, evaluation := range request.Evaluations {
		if !supportsSubjectEnrichment(evaluation.Subject.Category) {
			enriched.Evaluations = append(enriched.Evaluations, evaluation)
			continue
		}
		subjectEntity, svcErr := s.resolveSubject(ctx, evaluation.Subject, subjects)
		if svcErr != nil {
			return providers.AccessEvaluationsRequest{}, svcErr
		}
		if subjectEntity != nil {
			properties := map[string]interface{}{}
			for key, value := range evaluation.Subject.Properties {
				properties[key] = value
			}
			if len(subjectEntity.Attributes) > 0 {
				if err := json.Unmarshal(subjectEntity.Attributes, &properties); err != nil {
					return providers.AccessEvaluationsRequest{}, &tidcommon.InternalServerError
				}
			}
			if subjectEntity.OUID != "" {
				properties["ouId"] = subjectEntity.OUID
			}
			evaluation.Subject.Properties = properties
			// Keep the public subject category for routing while retaining the concrete
			// entity type for external PDP attribute mapping.
			evaluation.Subject.Type = subjectEntity.Type
		}
		enriched.Evaluations = append(enriched.Evaluations, evaluation)
	}
	return enriched, nil
}

// resolveSubject reads the subject's entity once per batch. It is nil when the subject does not exist or its
// category differs.
func (s *authorizationService) resolveSubject(ctx context.Context, subject providers.Subject,
	subjects map[string]*providers.Entity) (*providers.Entity, *tidcommon.ServiceError) {
	key := subject.Category + ":" + subject.ID
	if resolved, ok := subjects[key]; ok {
		return resolved, nil
	}
	resolved, err := s.entityService.GetEntityProfile(ctx, subject.ID)
	if err != nil {
		if !errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &tidcommon.InternalServerError
		}
		resolved = nil
	}
	if resolved != nil && resolved.Category.String() != subject.Category {
		resolved = nil
	}
	subjects[key] = resolved
	return resolved, nil
}

// supportsSubjectEnrichment reports whether a subject category has persisted entity attributes.
func supportsSubjectEnrichment(subjectCategory string) bool {
	return subjectCategory == providers.EntityCategoryUser.String() ||
		subjectCategory == providers.EntityCategoryAgent.String()
}

// toEngineResourceServer converts provider routing data into the engine representation.
func toEngineResourceServer(resourceServer providers.AccessEvaluationResourceServer) engine.ResourceServer {
	if resourceServer.ResourceID == "" && len(resourceServer.Properties) == 0 {
		return engine.ResourceServer{ID: resourceServer.ID}
	}
	properties := make(map[string]interface{}, len(resourceServer.Properties)+1)
	for key, value := range resourceServer.Properties {
		properties[key] = value
	}
	if resourceServer.ResourceID != "" {
		properties[engine.DelegatedResourceIDProperty] = resourceServer.ResourceID
	}
	return engine.ResourceServer{ID: resourceServer.ID, Properties: properties}
}

// toEngineAccessEvaluationsRequest converts provider evaluations to engine evaluations.
func toEngineAccessEvaluationsRequest(request providers.AccessEvaluationsRequest) engine.AccessEvaluationsRequest {
	evaluations := make([]engine.AccessEvaluationRequest, 0, len(request.Evaluations))
	for _, evaluation := range request.Evaluations {
		evaluations = append(evaluations, engine.AccessEvaluationRequest{
			Subject: engine.Subject{
				Category:   evaluation.Subject.Category,
				Type:       evaluation.Subject.Type,
				ID:         evaluation.Subject.ID,
				GroupIDs:   evaluation.Subject.GroupIDs,
				RoleIDs:    evaluation.Subject.RoleIDs,
				Properties: evaluation.Subject.Properties,
			},
			ResourceServer: toEngineResourceServer(evaluation.ResourceServer),
			Permission: engine.Permission{
				Name:       evaluation.Permission.Name,
				Properties: evaluation.Permission.Properties,
			},
			OUID:    evaluation.OUID,
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
			Decision:       evaluation.Decision,
			Context:        evaluation.Context,
			GrantedInstead: slices.Clone(evaluation.GrantedInstead),
		})
	}
	return &providers.AccessEvaluationsResponse{Evaluations: evaluations}
}
