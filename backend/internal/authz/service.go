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

// Package authz provides authorization service functionality.
package authz

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "AuthorizationService"

// AuthorizationServiceInterface defines the interface for authorization operations.
// This is the public interface exposed to external consumers.
type AuthorizationServiceInterface interface {
	// EvaluateAccess evaluates a single fine-grained access request.
	EvaluateAccess(
		ctx context.Context,
		request AccessEvaluationRequest,
	) (*AccessEvaluationResponse, *tidcommon.ServiceError)

	// EvaluateAccessBatch evaluates multiple fine-grained access requests.
	EvaluateAccessBatch(
		ctx context.Context,
		request AccessEvaluationsRequest,
	) (*AccessEvaluationsResponse, *tidcommon.ServiceError)
}

// authorizationService is the default implementation of AuthorizationServiceInterface.
type authorizationService struct {
	engine engine.AuthorizationEngine
}

// newAuthorizationService creates a new instance of authorizationService.
func newAuthorizationService(engine engine.AuthorizationEngine) AuthorizationServiceInterface {
	return &authorizationService{
		engine: engine,
	}
}

// EvaluateAccess evaluates a single fine-grained access request.
func (s *authorizationService) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, *tidcommon.ServiceError) {
	response, svcErr := s.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{request},
	})
	if svcErr != nil {
		return nil, svcErr
	}
	if len(response.Evaluations) == 0 {
		return &AccessEvaluationResponse{}, nil
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch evaluates multiple fine-grained access requests.
func (s *authorizationService) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Evaluating authorization request",
		log.Int("evaluationCount", len(request.Evaluations)))

	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{
			Evaluations: []AccessEvaluationResponse{},
		}, nil
	}

	// Delegate to engine (engine/underlying service handles validation)
	evaluationResp, err := s.engine.EvaluateAccessBatch(ctx, toEngineAccessEvaluationsRequest(request))
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

func toEngineAccessEvaluationsRequest(request AccessEvaluationsRequest) engine.AccessEvaluationsRequest {
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
				Handle:     evaluation.ResourceServer.Handle,
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

func fromEngineAccessEvaluationsResponse(response *engine.AccessEvaluationsResponse) *AccessEvaluationsResponse {
	if response == nil {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}
	}

	evaluations := make([]AccessEvaluationResponse, 0, len(response.Evaluations))
	for _, evaluation := range response.Evaluations {
		evaluations = append(evaluations, AccessEvaluationResponse{
			Decision: evaluation.Decision,
			Context:  evaluation.Context,
		})
	}
	return &AccessEvaluationsResponse{Evaluations: evaluations}
}
