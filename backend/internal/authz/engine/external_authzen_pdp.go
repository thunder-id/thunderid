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
	"github.com/thunder-id/thunderid/internal/connection"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// AuthZENPDPConfig configures an external AuthZEN PDP access-evaluation endpoint.
type AuthZENPDPConfig struct {
	Endpoint                 string
	BatchEndpoint            string
	Timeout                  time.Duration
	RetryCount               int
	SubjectProperties        []string
	SubjectPropertyMappings  map[string]string
	SubjectAttributeMappings []SubjectAttributeMapping
}

// NewAuthZENPDP creates an external AuthZEN PDP engine from a saved connection.
func NewAuthZENPDP(
	ctx context.Context,
	connectionID string,
	client httpservice.HTTPClientInterface,
) (AuthorizationEngine, bool, bool, error) {
	config, err := connection.GetAuthZENPDPRuntimeConfig(ctx, strings.TrimSpace(connectionID))
	if err != nil {
		return nil, false, false, err
	}
	if config == nil {
		return nil, false, false, nil
	}

	pdpEngine, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:                 config.Endpoint,
		BatchEndpoint:            config.BatchEndpoint,
		Timeout:                  time.Duration(config.TimeoutMS) * time.Millisecond,
		RetryCount:               config.RetryCount,
		SubjectProperties:        config.SubjectProperties,
		SubjectPropertyMappings:  config.SubjectPropertyMappings,
		SubjectAttributeMappings: toSubjectAttributeMappings(config.SubjectAttributeMappings),
	}, client)
	return pdpEngine, config.FailOpen, true, err
}

func toSubjectAttributeMappings(
	groups []connection.AuthZENPDPSubjectAttributeMapping,
) []SubjectAttributeMapping {
	if len(groups) == 0 {
		return nil
	}
	result := make([]SubjectAttributeMapping, 0, len(groups))
	for _, group := range groups {
		attributes := make([]SubjectAttributeRow, 0, len(group.Attributes))
		for _, attribute := range group.Attributes {
			attributes = append(attributes, SubjectAttributeRow{
				Attribute:    attribute.Attribute,
				PDPAttribute: attribute.PDPAttribute,
			})
		}
		result = append(result, SubjectAttributeMapping{
			UserType:   group.UserType,
			Attributes: attributes,
		})
	}
	return result
}

// SubjectAttributeMapping maps a subject type to PDP subject attribute names.
type SubjectAttributeMapping struct {
	UserType   string
	Attributes []SubjectAttributeRow
}

// SubjectAttributeRow identifies one PDP subject attribute mapping.
type SubjectAttributeRow struct {
	Attribute    string
	PDPAttribute string
}

type authZENPDP struct {
	endpoint                 string
	batchEndpoint            string
	retryCount               int
	subjectProperties        map[string]struct{}
	subjectPropertyMappings  map[string]string
	subjectAttributeMappings []SubjectAttributeMapping
	timeout                  time.Duration
	client                   httpservice.HTTPClientInterface
	logger                   *log.Logger
}

const (
	subjectGroupsProperty    = "groups"
	defaultAuthZENPDPTimeout = 5 * time.Second
)

type authZENEvaluationRequest = authzen.AccessEvaluationRequest
type authZENSubject = authzen.Subject
type authZENResource = authzen.Resource
type authZENAction = authzen.Action
type authZENEvaluationResponse = authzen.AccessEvaluationResponse
type authZENBatchEvaluationRequest = authzen.AccessEvaluationsRequest
type authZENBatchEvaluation = authzen.AccessEvaluationRequest
type authZENBatchEvaluationResponse = authzen.AccessEvaluationsResponse

// newAuthZENPDP creates an AuthorizationEngine from resolved external AuthZEN PDP settings.
func newAuthZENPDP(
	config AuthZENPDPConfig,
	client httpservice.HTTPClientInterface,
) (AuthorizationEngine, error) {
	configuredEndpoint := strings.TrimSpace(config.Endpoint)
	if err := validateAuthZENEndpoint(configuredEndpoint); err != nil {
		return nil, fmt.Errorf("invalid AuthZEN access evaluation endpoint: %w", err)
	}

	batchEndpoint := strings.TrimSpace(config.BatchEndpoint)
	if batchEndpoint == "" {
		return nil, fmt.Errorf("AuthZEN access evaluations endpoint is required")
	}
	if err := validateAuthZENEndpoint(batchEndpoint); err != nil {
		return nil, fmt.Errorf("invalid AuthZEN access evaluations endpoint: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("HTTP client is required")
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultAuthZENPDPTimeout
	}

	subjectProperties := make(map[string]struct{}, len(config.SubjectProperties))
	for _, property := range config.SubjectProperties {
		property = strings.TrimSpace(property)
		if property != "" {
			subjectProperties[property] = struct{}{}
		}
	}

	subjectPropertyMappings := make(map[string]string, len(config.SubjectPropertyMappings))
	for source, target := range config.SubjectPropertyMappings {
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source == "" || target == "" {
			return nil, fmt.Errorf("subject property mappings must have non-empty names")
		}
		if _, allowed := subjectProperties[source]; !allowed {
			return nil, fmt.Errorf("subject property %q has a mapping but is not allowed", source)
		}
		subjectPropertyMappings[source] = target
	}
	subjectAttributeMappings := normalizeSubjectAttributeMappings(config.SubjectAttributeMappings)

	return &authZENPDP{
		endpoint:                 configuredEndpoint,
		batchEndpoint:            batchEndpoint,
		retryCount:               max(config.RetryCount, 0),
		subjectProperties:        subjectProperties,
		subjectPropertyMappings:  subjectPropertyMappings,
		subjectAttributeMappings: subjectAttributeMappings,
		timeout:                  timeout,
		client:                   client,
		logger:                   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExternalAuthZENPDP")),
	}, nil
}

// EvaluateAccess evaluates a single authorization request with the external AuthZEN PDP.
func (p *authZENPDP) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := p.evaluateSingle(ctx, request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// EvaluateAccessBatch evaluates multiple authorization requests using AuthZEN's batch endpoint.
func (p *authZENPDP) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}, nil
	}
	return p.evaluateBatch(ctx, request)
}

// evaluateBatch converts ThunderID evaluations into AuthZEN batch payloads and maps responses back in order.
func (p *authZENPDP) evaluateBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	started := time.Now()
	batchEndpoint := p.batchEndpoint
	payload := authZENBatchEvaluationRequest{
		Evaluations: make([]authZENBatchEvaluation, 0, len(request.Evaluations)),
	}
	for _, evaluation := range request.Evaluations {
		converted := toAuthZENEvaluationRequest(
			evaluation,
			p.subjectProperties,
			p.subjectPropertyMappings,
			p.subjectAttributeMappings,
		)
		payload.Evaluations = append(payload.Evaluations, authZENBatchEvaluation{
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

	var response authZENBatchEvaluationResponse
	if err := p.post(ctx, batchEndpoint, body, &response); err != nil {
		p.logger.Error(ctx, "External AuthZEN batch evaluation failed",
			log.String("pdp_endpoint", batchEndpoint),
			log.Int("evaluation_count", len(request.Evaluations)),
			log.Any("evaluation_request", auditAuthZENEvaluations(payload.Evaluations)),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
			log.Error(err),
		)
		return nil, err
	}
	if len(response.Evaluations) != len(request.Evaluations) {
		return nil, fmt.Errorf("AuthZEN PDP returned %d evaluations for %d requests",
			len(response.Evaluations), len(request.Evaluations))
	}

	for index, evaluation := range request.Evaluations {
		converted := payload.Evaluations[index]
		fields := authZENEvaluationLogFields(converted)
		fields = append(fields,
			log.MaskedMap("response_context", response.Evaluations[index].Context),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
		)
		p.logger.Info(ctx, "External AuthZEN evaluation completed",
			append([]log.Field{
				log.String("pdp_endpoint", batchEndpoint),
				log.String("resource_server_id", evaluation.ResourceServer.ID),
				log.String("action", evaluation.Permission.Name),
				log.Bool("decision", response.Evaluations[index].Decision),
			}, fields...)...,
		)
	}

	results := make([]AccessEvaluationResponse, 0, len(response.Evaluations))
	for _, evaluation := range response.Evaluations {
		results = append(results, AccessEvaluationResponse(evaluation))
	}
	return &AccessEvaluationsResponse{Evaluations: results}, nil
}

// evaluateSingle sends one ThunderID authorization request to the configured AuthZEN PDP.
func (p *authZENPDP) evaluateSingle(
	ctx context.Context,
	evaluation AccessEvaluationRequest,
) (response *AccessEvaluationResponse, err error) {
	started := time.Now()
	evaluationEndpoint := p.endpoint
	decision := false
	var auditRequest authZENEvaluationRequest
	var responseContext map[string]interface{}
	defer func() {
		fields := []log.Field{
			log.String("pdp_endpoint", evaluationEndpoint),
			log.String("resource_server_id", evaluation.ResourceServer.ID),
			log.String("action", evaluation.Permission.Name),
			log.Bool("decision", decision),
			log.Int("latency_ms", int(time.Since(started).Milliseconds())),
			log.MaskedMap("response_context", responseContext),
		}
		if auditRequest.Subject.ID != "" {
			fields = append(fields, authZENEvaluationLogFields(auditRequest)...)
		} else {
			fields = append(fields, log.MaskedString("subject_id", evaluation.Subject.ID))
		}
		if err != nil {
			p.logger.Error(ctx, "External AuthZEN evaluation failed", append(fields, log.Error(err))...)
			return
		}
		p.logger.Info(ctx, "External AuthZEN evaluation completed", fields...)
	}()

	auditRequest = toAuthZENEvaluationRequest(
		evaluation,
		p.subjectProperties,
		p.subjectPropertyMappings,
		p.subjectAttributeMappings,
	)
	payload, err := json.Marshal(auditRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to encode AuthZEN request: %w", err)
	}

	var responseDecision authZENEvaluationResponse
	if err := p.post(ctx, evaluationEndpoint, payload, &responseDecision); err != nil {
		return nil, err
	}

	decision = responseDecision.Decision
	responseContext = responseDecision.Context
	return &AccessEvaluationResponse{
		Decision: responseDecision.Decision,
		Context:  responseDecision.Context,
	}, nil
}

// authZENEvaluationLogFields returns request fields safe for audit logs. Subject, resource, action,
// and context attributes are masked where their values may contain sensitive data.
func authZENEvaluationLogFields(request authZENEvaluationRequest) []log.Field {
	fields := []log.Field{
		log.MaskedString("subject_id", request.Subject.ID),
		log.String("subject_type", request.Subject.Type),
		log.String("resource_type", request.Resource.Type),
		log.MaskedString("resource_id", request.Resource.ID),
		log.String("action_name", request.Action.Name),
	}
	if request.Subject.Properties != nil {
		fields = append(fields, log.MaskedMap("subject_properties", request.Subject.Properties))
	}
	if request.Resource.Properties != nil {
		fields = append(fields, log.MaskedMap("resource_properties", request.Resource.Properties))
	}
	if request.Action.Properties != nil {
		fields = append(fields, log.MaskedMap("action_properties", request.Action.Properties))
	}
	if request.Context != nil {
		fields = append(fields, log.MaskedMap("evaluation_context", request.Context))
	}
	return fields
}

func auditAuthZENEvaluations(evaluations []authZENBatchEvaluation) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(evaluations))
	for _, evaluation := range evaluations {
		result = append(result, map[string]interface{}{
			"subject": map[string]interface{}{
				"type":       evaluation.Subject.Type,
				"id":         maskedAuditString(evaluation.Subject.ID),
				"properties": maskedAuditMap(evaluation.Subject.Properties),
			},
			"resource": map[string]interface{}{
				"type":       evaluation.Resource.Type,
				"id":         maskedAuditString(evaluation.Resource.ID),
				"properties": maskedAuditMap(evaluation.Resource.Properties),
			},
			"action": map[string]interface{}{
				"name":       evaluation.Action.Name,
				"properties": maskedAuditMap(evaluation.Action.Properties),
			},
			"context": maskedAuditMap(evaluation.Context),
		})
	}
	return result
}

func maskedAuditString(value string) string {
	return log.MaskedString("value", value).Value.(string)
}

func maskedAuditMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}
	return log.MaskedMap("values", values).Value.(map[string]interface{})
}

// post sends an AuthZEN JSON request and retries transient PDP or network failures.
func (p *authZENPDP) post(
	ctx context.Context,
	endpoint string,
	payload []byte,
	result interface{},
) error {
	attempts := p.retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, p.timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create AuthZEN request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

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

func validateAuthZENEndpoint(endpoint string) error {
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Host == "" ||
		(parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") {
		return fmt.Errorf("endpoint must be an absolute URL")
	}
	return nil
}

// toAuthZENEvaluationRequest maps ThunderID's internal authorization request to AuthZEN's wire format.
func toAuthZENEvaluationRequest(
	evaluation AccessEvaluationRequest,
	allowedSubjectProperties map[string]struct{},
	subjectPropertyMappings map[string]string,
	subjectAttributeMappings []SubjectAttributeMapping,
) authZENEvaluationRequest {
	resourceType := evaluation.ResourceServer.Type
	resourceID := evaluation.ResourceServer.ResourceID
	if strings.TrimSpace(resourceType) == "" {
		resourceType = evaluation.ResourceServer.ID
	}
	if strings.TrimSpace(resourceID) == "" {
		resourceID = evaluation.ResourceServer.ID
	}

	resourceProperties := make(map[string]interface{}, len(evaluation.ResourceServer.Properties))
	for key, value := range evaluation.ResourceServer.Properties {
		resourceProperties[key] = value
	}

	return authZENEvaluationRequest{
		Subject: authZENSubject{
			Type: evaluation.Subject.Type,
			ID:   evaluation.Subject.ID,
			Properties: authZENSubjectProperties(
				evaluation.Subject,
				allowedSubjectProperties,
				subjectPropertyMappings,
				subjectAttributeMappings,
			),
		},
		Resource: authZENResource{
			Type:       resourceType,
			ID:         resourceID,
			Properties: resourceProperties,
		},
		Action: authZENAction{
			Name:       evaluation.Permission.Name,
			Properties: evaluation.Permission.Properties,
		},
		Context: evaluation.Context,
	}
}

// authZENSubjectProperties filters and optionally renames subject attributes before sending them to the PDP.
func authZENSubjectProperties(
	subject Subject,
	allowedSubjectProperties map[string]struct{},
	subjectPropertyMappings map[string]string,
	subjectAttributeMappings []SubjectAttributeMapping,
) map[string]interface{} {
	allowedSubjectProperties, subjectPropertyMappings = subjectMappingForSubject(
		subject.Type,
		allowedSubjectProperties,
		subjectPropertyMappings,
		subjectAttributeMappings,
	)
	properties := make(map[string]interface{}, len(allowedSubjectProperties)+1)
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

func subjectMappingForSubject(
	subjectType string,
	allowedSubjectProperties map[string]struct{},
	subjectPropertyMappings map[string]string,
	subjectAttributeMappings []SubjectAttributeMapping,
) (map[string]struct{}, map[string]string) {
	if len(subjectAttributeMappings) == 0 {
		return allowedSubjectProperties, subjectPropertyMappings
	}
	resultProperties := cloneSubjectPropertySet(allowedSubjectProperties)
	resultMappings := cloneSubjectPropertyMapping(subjectPropertyMappings)
	trimmedSubjectType := strings.TrimSpace(subjectType)
	for _, group := range subjectAttributeMappings {
		if strings.TrimSpace(group.UserType) != trimmedSubjectType {
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

func normalizeSubjectAttributeMappings(groups []SubjectAttributeMapping) []SubjectAttributeMapping {
	if len(groups) == 0 {
		return nil
	}
	normalized := make([]SubjectAttributeMapping, 0, len(groups))
	for _, group := range groups {
		attributes := make([]SubjectAttributeRow, 0, len(group.Attributes))
		for _, row := range group.Attributes {
			attribute := strings.TrimSpace(row.Attribute)
			pdpAttribute := strings.TrimSpace(row.PDPAttribute)
			if attribute == "" {
				continue
			}
			if pdpAttribute == "" {
				pdpAttribute = attribute
			}
			attributes = append(attributes, SubjectAttributeRow{
				Attribute:    attribute,
				PDPAttribute: pdpAttribute,
			})
		}
		if len(attributes) == 0 {
			continue
		}
		normalized = append(normalized, SubjectAttributeMapping{
			UserType:   strings.TrimSpace(group.UserType),
			Attributes: attributes,
		})
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func cloneSubjectPropertySet(values map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(values))
	for key := range values {
		clone[key] = struct{}{}
	}
	return clone
}

func cloneSubjectPropertyMapping(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
