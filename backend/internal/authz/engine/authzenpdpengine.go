// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/authzen"
	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
)

// newAuthZENPDPEngine creates an AuthZEN PDP authorization engine.
func newAuthZENPDPEngine(
	connectionService authZENPDPConnectionService,
	client httpservice.HTTPClientInterface,
) AuthorizationEngine {
	return &authZENPDPEngine{
		connectionService: connectionService,
		client:            client,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthZENPDP")),
	}
}

// EvaluateAccess evaluates a single authorization request with the AuthZEN PDP.
func (p *authZENPDPEngine) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := p.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{request},
	})
	if err != nil {
		return nil, err
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch evaluates multiple authorization requests using AuthZEN's batch endpoint.
func (p *authZENPDPEngine) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}, nil
	}
	if p.client == nil {
		return nil, fmt.Errorf("HTTP client is required")
	}
	groups, err := groupAuthZENEvaluations(request)
	if err != nil {
		return nil, err
	}
	responses := make([]AccessEvaluationResponse, len(request.Evaluations))
	for key, group := range groups {
		settings, err := p.settingsForConnection(ctx, key.connectionID, key.resourceType)
		if err != nil {
			return nil, err
		}
		response, err := p.evaluateGroup(ctx, settings, group.request)
		if err != nil {
			return nil, err
		}
		for index, evaluation := range response.Evaluations {
			responses[group.indexes[index]] = evaluation
		}
	}
	return &AccessEvaluationsResponse{Evaluations: responses}, nil
}

// evaluateGroup uses the batch endpoint when configured, otherwise evaluates each request at the single endpoint.
func (p *authZENPDPEngine) evaluateGroup(
	ctx context.Context,
	settings *authZENPDPSettings,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if settings.batchEndpoint != "" {
		return p.evaluateBatch(ctx, settings, request)
	}

	responses := make([]AccessEvaluationResponse, 0, len(request.Evaluations))
	for _, evaluation := range request.Evaluations {
		response, err := p.evaluateSingle(ctx, settings, evaluation)
		if err != nil {
			return nil, err
		}
		responses = append(responses, *response)
	}
	return &AccessEvaluationsResponse{Evaluations: responses}, nil
}

// groupAuthZENEvaluations groups requests by their PDP connection and resource type.
func groupAuthZENEvaluations(request AccessEvaluationsRequest) (map[authZENPDPBatchKey]*authZENPDPBatch, error) {
	groups := make(map[authZENPDPBatchKey]*authZENPDPBatch)
	for index, evaluation := range request.Evaluations {
		connectionID, resourceType, err := authZENPDPRoute(evaluation)
		if err != nil {
			return nil, err
		}
		key := authZENPDPBatchKey{connectionID: connectionID, resourceType: resourceType}
		group, found := groups[key]
		if !found {
			group = &authZENPDPBatch{}
			groups[key] = group
		}
		group.request.Evaluations = append(group.request.Evaluations, evaluation)
		group.indexes = append(group.indexes, index)
	}
	return groups, nil
}

// authZENPDPRoute returns the PDP connection and resource-server identifiers for an evaluation.
func authZENPDPRoute(evaluation AccessEvaluationRequest) (string, string, error) {
	connectionID := strings.TrimSpace(evaluation.ResourceServer.Engine.Properties.PDPConnectionID)
	if connectionID == "" {
		return "", "", fmt.Errorf("AuthZEN PDP connection ID is required")
	}
	return connectionID, strings.TrimSpace(evaluation.ResourceServer.Identifier), nil
}

// settingsForConnection loads and normalizes settings for a PDP connection.
func (p *authZENPDPEngine) settingsForConnection(
	ctx context.Context,
	connectionID string,
	resourceType string,
) (*authZENPDPSettings, error) {
	if p.connectionService == nil {
		return nil, fmt.Errorf("AuthZEN PDP connection service is required")
	}
	connection, svcErr := p.connectionService.GetAuthZENPDP(ctx, connectionID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve AuthZEN PDP connection: %s", svcErr.Code)
	}
	if connection == nil {
		return nil, fmt.Errorf("AuthZEN PDP connection %q was not found", connectionID)
	}
	settings, err := prepareAuthZENPDPSettingsWithTimeout(AuthZENPDPConfig{
		ResourceType:             resourceType,
		Endpoint:                 connection.Endpoint,
		BatchEndpoint:            connection.BatchEndpoint,
		Timeout:                  time.Duration(connection.TimeoutMS) * time.Millisecond,
		RetryCount:               connection.RetryCount,
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	}, time.Second)
	if err != nil {
		return nil, err
	}
	authenticationConfig, err := connection.OutboundAuthenticationConfig()
	if err != nil {
		return nil, err
	}
	authenticator, err := outboundauthn.NewRequestAuthenticator(authenticationConfig)
	if err != nil {
		return nil, err
	}
	settings.authenticator = authenticator
	return settings, nil
}

// evaluateBatch converts ThunderID evaluations into AuthZEN batch payloads and maps responses back in order.
func (p *authZENPDPEngine) evaluateBatch(
	ctx context.Context,
	settings *authZENPDPSettings,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	started := time.Now()
	batchEndpoint := settings.batchEndpoint
	payload := authzen.AccessEvaluationsRequest{
		Evaluations: make([]authzen.AccessEvaluationRequest, 0, len(request.Evaluations)),
	}
	for _, evaluation := range request.Evaluations {
		converted := toAuthZENEvaluationRequest(
			evaluation,
			settings.resourceType,
			settings.subjectAttributeMappings,
		)
		payload.Evaluations = append(payload.Evaluations, authzen.AccessEvaluationRequest{
			Subject:  converted.Subject,
			Resource: converted.Resource,
			Action:   converted.Action,
			Context:  evaluation.Context,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode AuthZEN batch request: %w", err)
	}

	var response authzen.AccessEvaluationsResponse
	if err := p.post(ctx, settings, batchEndpoint, body, &response); err != nil {
		p.logger.Error(ctx, "AuthZEN batch evaluation failed",
			log.String("pdp_endpoint", batchEndpoint),
			log.Int("evaluation_count", len(request.Evaluations)),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
			log.Error(err),
		)
		return nil, err
	}
	if len(response.Evaluations) != len(request.Evaluations) {
		return nil, fmt.Errorf("AuthZEN PDP returned %d evaluations for %d requests",
			len(response.Evaluations), len(request.Evaluations))
	}

	results := make([]AccessEvaluationResponse, 0, len(response.Evaluations))
	for _, evaluation := range response.Evaluations {
		results = append(results, AccessEvaluationResponse(evaluation))
	}
	return &AccessEvaluationsResponse{Evaluations: results}, nil
}

// evaluateSingle sends one ThunderID authorization request to the configured AuthZEN PDP.
func (p *authZENPDPEngine) evaluateSingle(
	ctx context.Context,
	settings *authZENPDPSettings,
	evaluation AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	evaluationEndpoint := settings.endpoint
	request := toAuthZENEvaluationRequest(
		evaluation,
		settings.resourceType,
		settings.subjectAttributeMappings,
	)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode AuthZEN request: %w", err)
	}

	var responseDecision authzen.AccessEvaluationResponse
	if err := p.post(ctx, settings, evaluationEndpoint, payload, &responseDecision); err != nil {
		p.logger.Error(ctx, "AuthZEN evaluation failed",
			log.String("pdp_endpoint", evaluationEndpoint),
			log.String("resource_server_id", evaluation.ResourceServer.ID),
			log.String("action", evaluation.Permission.Name),
			log.Error(err),
		)
		return nil, err
	}

	return &AccessEvaluationResponse{
		Decision: responseDecision.Decision,
		Context:  responseDecision.Context,
	}, nil
}

// post sends an AuthZEN JSON request and retries transient PDP or network failures.
func (p *authZENPDPEngine) post(
	ctx context.Context,
	settings *authZENPDPSettings,
	endpoint string,
	payload []byte,
	result interface{},
) error {
	attempts := settings.retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, settings.timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create AuthZEN request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if settings.authenticator != nil {
			settings.authenticator.ApplyAuthentication(req)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			cancel()
			if ctx.Err() != nil || attempt == attempts-1 {
				return fmt.Errorf("AuthZEN PDP request failed: %w", err)
			}
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			statusCode := resp.StatusCode
			_ = resp.Body.Close()
			cancel()
			if statusCode >= http.StatusInternalServerError && attempt < attempts-1 {
				continue
			}
			return fmt.Errorf("AuthZEN PDP returned HTTP %d", statusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			_ = resp.Body.Close()
			cancel()
			return fmt.Errorf("failed to decode AuthZEN response: %w", err)
		}
		_ = resp.Body.Close()
		cancel()
		return nil
	}
	return fmt.Errorf("AuthZEN PDP request failed after retries")
}

// toAuthZENEvaluationRequest maps ThunderID's internal authorization request to AuthZEN's wire format.
func toAuthZENEvaluationRequest(
	evaluation AccessEvaluationRequest,
	configuredResourceType string,
	subjectAttributeMappings []authzenpdp.SubjectAttributeMapping,
) authzen.AccessEvaluationRequest {
	resourceType := strings.TrimSpace(configuredResourceType)
	resourceID, _ := evaluation.ResourceServer.Properties[DelegatedResourceIDProperty].(string)
	if strings.TrimSpace(resourceType) == "" {
		resourceType = evaluation.ResourceServer.ID
	}
	if strings.TrimSpace(resourceID) == "" {
		resourceID = evaluation.ResourceServer.ID
	}

	resourceProperties := make(map[string]interface{}, len(evaluation.ResourceServer.Properties))
	for key, value := range evaluation.ResourceServer.Properties {
		if key == DelegatedResourceIDProperty {
			continue
		}
		resourceProperties[key] = value
	}

	return authzen.AccessEvaluationRequest{
		Subject: authzen.Subject{
			Type: evaluation.Subject.Category,
			ID:   evaluation.Subject.ID,
			Properties: mapAuthZENSubjectProperties(
				evaluation.Subject,
				subjectAttributeMappings,
			),
		},
		Resource: authzen.Resource{
			Type:       resourceType,
			ID:         resourceID,
			Properties: resourceProperties,
		},
		Action: authzen.Action{
			Name:       evaluation.Permission.Name,
			Properties: evaluation.Permission.Properties,
		},
		Context: evaluation.Context,
	}
}

// mapAuthZENSubjectProperties filters and optionally renames subject attributes for the PDP.
func mapAuthZENSubjectProperties(
	subject Subject,
	subjectAttributeMappings []authzenpdp.SubjectAttributeMapping,
) map[string]interface{} {
	properties := make(map[string]interface{}, len(subject.Properties)+1)
	allowedSubjectProperties, subjectPropertyMappings := subjectMappingForSubject(
		subject.Type, subjectAttributeMappings,
	)
	for key, value := range subject.Properties {
		if _, allowed := allowedSubjectProperties[key]; !allowed {
			continue
		}
		propertyName := key
		if mappedName, mapped := subjectPropertyMappings[key]; mapped {
			propertyName = mappedName
		}
		properties[propertyName] = value
	}
	if len(subject.GroupIDs) > 0 {
		if _, allowed := allowedSubjectProperties[subjectGroupsProperty]; allowed {
			propertyName := subjectGroupsProperty
			if mappedName, mapped := subjectPropertyMappings[subjectGroupsProperty]; mapped {
				propertyName = mappedName
			}
			if _, exists := properties[propertyName]; !exists {
				properties[propertyName] = append([]string(nil), subject.GroupIDs...)
			}
		}
	}
	if len(properties) == 0 {
		return nil
	}
	return properties
}

// subjectMappingForSubject returns the released and renamed attributes for a concrete subject.
func subjectMappingForSubject(
	entityType string,
	subjectAttributeMappings []authzenpdp.SubjectAttributeMapping,
) (map[string]struct{}, map[string]string) {
	resultProperties := make(map[string]struct{})
	resultMappings := make(map[string]string)
	trimmedEntityType := strings.TrimSpace(entityType)
	for _, group := range subjectAttributeMappings {
		if strings.TrimSpace(group.EntityType) != trimmedEntityType {
			continue
		}
		for _, row := range group.Attributes {
			attribute := strings.TrimSpace(row.Attribute)
			if attribute == "" {
				continue
			}
			resultProperties[attribute] = struct{}{}
			if pdpAttribute := strings.TrimSpace(row.PDPAttribute); pdpAttribute != "" {
				resultMappings[attribute] = pdpAttribute
			}
		}
		break
	}
	return resultProperties, resultMappings
}

// normalizeSubjectAttributeMappings copies and removes empty subject attribute mappings.
func normalizeSubjectAttributeMappings(
	groups []authzenpdp.SubjectAttributeMapping,
) []authzenpdp.SubjectAttributeMapping {
	if len(groups) == 0 {
		return nil
	}
	normalized := make([]authzenpdp.SubjectAttributeMapping, 0, len(groups))
	for _, group := range groups {
		attributes := make([]authzenpdp.SubjectAttributeRow, 0, len(group.Attributes))
		for _, row := range group.Attributes {
			attribute := strings.TrimSpace(row.Attribute)
			pdpAttribute := strings.TrimSpace(row.PDPAttribute)
			if attribute == "" {
				continue
			}
			if pdpAttribute == "" {
				pdpAttribute = attribute
			}
			attributes = append(attributes, authzenpdp.SubjectAttributeRow{
				Attribute:    attribute,
				PDPAttribute: pdpAttribute,
			})
		}
		if len(attributes) == 0 {
			continue
		}
		normalized = append(normalized, authzenpdp.SubjectAttributeMapping{
			EntityType: strings.TrimSpace(group.EntityType),
			Attributes: attributes,
		})
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// validateAuthZENEndpoint accepts only absolute HTTP or HTTPS PDP endpoints.
func validateAuthZENEndpoint(endpoint string) error {
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Host == "" ||
		(parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") {
		return fmt.Errorf("endpoint must be an absolute URL")
	}
	return nil
}

// prepareAuthZENPDPSettings validates and normalizes settings local to one evaluation.
func prepareAuthZENPDPSettings(config AuthZENPDPConfig) (*authZENPDPSettings, error) {
	return prepareAuthZENPDPSettingsWithTimeout(config, time.Second)
}

// prepareAuthZENPDPSettingsWithTimeout validates settings with a caller-provided timeout.
func prepareAuthZENPDPSettingsWithTimeout(
	config AuthZENPDPConfig,
	defaultTimeout time.Duration,
) (*authZENPDPSettings, error) {
	configuredEndpoint := strings.TrimSpace(config.Endpoint)
	if err := validateAuthZENEndpoint(configuredEndpoint); err != nil {
		return nil, fmt.Errorf("invalid AuthZEN access evaluation endpoint: %w", err)
	}

	batchEndpoint := strings.TrimSpace(config.BatchEndpoint)
	if batchEndpoint != "" {
		if err := validateAuthZENEndpoint(batchEndpoint); err != nil {
			return nil, fmt.Errorf("invalid AuthZEN access evaluations endpoint: %w", err)
		}
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	subjectAttributeMappings := normalizeSubjectAttributeMappings(config.SubjectAttributeMappings)

	return &authZENPDPSettings{
		resourceType:             strings.TrimSpace(config.ResourceType),
		endpoint:                 configuredEndpoint,
		batchEndpoint:            batchEndpoint,
		retryCount:               max(config.RetryCount, 0),
		subjectAttributeMappings: subjectAttributeMappings,
		timeout:                  timeout,
	}, nil
}
