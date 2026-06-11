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

package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// External service endpoints.
const (
	consentElementsEndpoint         = "/consent-elements"
	consentElementsValidateEndpoint = "/consent-elements/validate"
	consentPurposesEndpoint         = "/consent-purposes"
	consentsEndpoint                = "/consents"
	consentValidateEndpoint         = "/consents/validate"
)

// clientConfig holds configuration for the consent client.
type clientConfig struct {
	baseURL    string
	timeout    time.Duration
	maxRetries int
}

// defaultClient is the default consentClientInterface implementation, backed by the
// REST API based consent management service.
type defaultClient struct {
	clientConfig clientConfig
	httpClient   httpservice.HTTPClientInterface
	logger       *log.Logger
}

// newDefaultClient creates a new instance of default consent client.
func newDefaultClient(httpClient httpservice.HTTPClientInterface) consentClientInterface {
	return &defaultClient{
		clientConfig: getClientConfig(),
		httpClient:   httpClient,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentClient")),
	}
}

// --- DTO definitions for default client ---

// elementCreateDTO represents the request body for creating a consent element.
type elementCreateDTO struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// elementResponseDTO represents the response body for a consent element.
type elementResponseDTO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Properties  map[string]string `json:"properties"`
}

// elementsCreateResponseDTO represents the response body for creating consent elements in batch.
type elementsCreateResponseDTO struct {
	Data    []elementResponseDTO `json:"data"`
	Message string               `json:"message"`
}

// elementListResponseDTO represents the response body for listing consent elements.
type elementListResponseDTO struct {
	Data []elementResponseDTO `json:"data"`
}

// purposeElementDTO represents a consent element reference within a consent purpose.
type purposeElementDTO struct {
	Name        string `json:"name"`
	IsMandatory bool   `json:"isMandatory"`
}

// purposeCreateDTO represents the request body for creating a consent purpose.
type purposeCreateDTO struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Elements    []purposeElementDTO `json:"elements"`
}

// purposeResponseDTO represents the response body for a consent purpose.
type purposeResponseDTO struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	ClientID    string              `json:"clientId"`
	Elements    []purposeElementDTO `json:"elements"`
	CreatedTime int64               `json:"createdTime"`
	UpdatedTime int64               `json:"updatedTime"`
}

// purposeListResponseDTO represents the response body for listing consent purposes.
type purposeListResponseDTO struct {
	Data []purposeResponseDTO `json:"data"`
}

// authorizationRequestDTO represents an authorization entry in the consent create/update request.
type authorizationRequestDTO struct {
	UserID string `json:"userId"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// purposeItemRequestDTO represents a consent purpose item in the consent create/update request.
type purposeItemRequestDTO struct {
	Name     string                      `json:"name"`
	Elements []elementApprovalRequestDTO `json:"elements"`
}

// elementApprovalRequestDTO represents a consent element approval entry in the consent create/update request.
type elementApprovalRequestDTO struct {
	Name           string   `json:"name"`
	IsUserApproved bool     `json:"isUserApproved"`
	Value          struct{} `json:"value"`
}

// consentCreateDTO represents the request body for creating a consent record.
type consentCreateDTO struct {
	Type                       string                    `json:"type"`
	ValidityTime               int64                     `json:"validityTime"`
	RecurringIndicator         bool                      `json:"recurringIndicator"`
	DataAccessValidityDuration int64                     `json:"dataAccessValidityDuration"`
	Frequency                  int32                     `json:"frequency"`
	Purposes                   []purposeItemRequestDTO   `json:"purposes"`
	Authorizations             []authorizationRequestDTO `json:"authorizations"`
}

// authorizationResponseDTO represents an authorization entry in the consent response.
type authorizationResponseDTO struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	UpdatedTime int64  `json:"updatedTime"`
}

// elementApprovalResponseDTO represents a consent element approval entry in the consent response.
type elementApprovalResponseDTO struct {
	Name           string `json:"name"`
	IsUserApproved bool   `json:"isUserApproved"`
	IsMandatory    bool   `json:"isMandatory"`
}

// purposeItemResponseDTO represents a consent purpose item in the consent response.
type purposeItemResponseDTO struct {
	Name     string                       `json:"name"`
	Elements []elementApprovalResponseDTO `json:"elements"`
}

// consentResponseDTO represents the response body for a consent record.
type consentResponseDTO struct {
	ID                         string                     `json:"id"`
	Type                       string                     `json:"type"`
	ClientID                   string                     `json:"clientId"`
	Status                     string                     `json:"status"`
	ValidityTime               int64                      `json:"validityTime"`
	RecurringIndicator         bool                       `json:"recurringIndicator"`
	DataAccessValidityDuration int64                      `json:"dataAccessValidityDuration"`
	Frequency                  int32                      `json:"frequency"`
	Purposes                   []purposeItemResponseDTO   `json:"purposes"`
	Authorizations             []authorizationResponseDTO `json:"authorizations"`
	CreatedTime                int64                      `json:"createdTime"`
	UpdatedTime                int64                      `json:"updatedTime"`
}

// consentSearchResponseDTO represents the response body for searching consent records.
type consentSearchResponseDTO struct {
	Data []consentResponseDTO `json:"data"`
}

// consentValidateRequestDTO represents the request body for validating a consent record.
type consentValidateRequestDTO struct {
	ConsentID string `json:"consentId"`
}

// consentValidateResponseDTO represents the response body for validating a consent record.
type consentValidateResponseDTO struct {
	IsValid            bool               `json:"isValid"`
	ConsentInformation consentResponseDTO `json:"consentInformation"`
	ErrorCode          string             `json:"errorCode"`
	ErrorMessage       string             `json:"errorMessage"`
}

// consentRevokeDTO represents the request body for revoking a consent record.
type consentRevokeDTO struct {
	Reason string `json:"reason,omitempty"`
}

// consentBackendErrorDTO represents the structured error body returned by the consent service.
type consentBackendErrorDTO struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"`
	TraceID     string `json:"traceId"`
}

// --- Client method implementations ---

// createConsentElements creates one or more consent elements.
func (c *defaultClient) createConsentElements(ctx context.Context, ouID string, elements []ConsentElementInput) (
	[]ConsentElement, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentElementsEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	dtos := make([]elementCreateDTO, 0, len(elements))
	for _, el := range elements {
		dtos = append(dtos, c.consentElementInputToDTO(&el))
	}

	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, "", dtos)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentElementRequest)
	case http.StatusConflict:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentElementAlreadyExists)
	}
	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[elementsCreateResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode create-elements response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	out := make([]ConsentElement, 0, len(result.Data))
	for _, dto := range result.Data {
		out = append(out, c.dtoToConsentElement(&dto))
	}

	return out, nil
}

// listConsentElements retrieves consent elements filtered by optional name.
func (c *defaultClient) listConsentElements(ctx context.Context, ouID string, ns Namespace, nameFilter string) (
	[]ConsentElement, *serviceerror.ServiceError) {
	u, svcErr := c.buildSearchURL(ctx, nameFilter, consentElementsEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	resp, svcErr := c.doRequest(ctx, http.MethodGet, u, ouID, "", nil)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[elementListResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode list-elements response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	out := make([]ConsentElement, 0, len(result.Data))
	for _, dto := range result.Data {
		out = append(out, c.dtoToConsentElement(&dto))
	}

	return out, nil
}

// updateConsentElement updates a consent element by ID.
func (c *defaultClient) updateConsentElement(ctx context.Context, ouID, elementID string,
	element *ConsentElementInput) (*ConsentElement, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentElementsEndpoint, elementID)
	if svcErr != nil {
		return nil, svcErr
	}

	dto := c.consentElementInputToDTO(element)
	resp, svcErr := c.doRequest(ctx, http.MethodPut, u, ouID, "", dto)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentElementRequest)
	case http.StatusNotFound:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentElementNotFound)
	case http.StatusConflict:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentElementAlreadyExists)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[elementResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode update-element response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	el := c.dtoToConsentElement(result)

	return &el, nil
}

// deleteConsentElement deletes a consent element by ID.
func (c *defaultClient) deleteConsentElement(ctx context.Context,
	ouID, elementID string) *serviceerror.ServiceError {
	u, svcErr := c.buildServiceEndpoint(ctx, consentElementsEndpoint, elementID)
	if svcErr != nil {
		return svcErr
	}

	resp, svcErr := c.doRequest(ctx, http.MethodDelete, u, ouID, "", nil)
	if svcErr != nil {
		return svcErr
	}
	defer c.closeBody(ctx, resp)

	if resp.StatusCode == http.StatusNotFound {
		return c.handleClientError(ctx, resp, &ErrorConsentElementNotFound)
	}

	return c.checkStatus(ctx, resp)
}

// validateConsentElements validates a list of consent element names.
func (c *defaultClient) validateConsentElements(ctx context.Context, ouID string, names []string) (
	[]string, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentElementsValidateEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, "", names)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	// A 400 Bad Request indicates no elements matched (or empty array).
	// We handle it gracefully by returning an empty list as expected.
	if resp.StatusCode == http.StatusBadRequest {
		return []string{}, nil
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[[]string](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode validate-elements response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if result == nil {
		return []string{}, nil
	}

	return *result, nil
}

// createConsentPurpose creates a consent purpose.
func (c *defaultClient) createConsentPurpose(ctx context.Context, ouID string, purpose *ConsentPurposeInput) (
	*ConsentPurpose, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	dto := c.consentPurposeInputToDTO(purpose)
	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, purpose.GroupID, dto)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentPurposeRequest)
	case http.StatusConflict:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentPurposeAlreadyExists)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[purposeResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode create-purpose response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	p := c.dtoToConsentPurpose(result)

	return &p, nil
}

// listConsentPurposes retrieves consent purposes for the given organization, optionally filtered by
// group ID (e.g. app ID). If groupID is empty, returns all purposes for the organization.
func (c *defaultClient) listConsentPurposes(ctx context.Context, ouID, groupID string) (
	[]ConsentPurpose, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	if groupID != "" {
		u += "?clientIds=" + url.QueryEscape(groupID)
	}

	resp, svcErr := c.doRequest(ctx, http.MethodGet, u, ouID, "", nil)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[purposeListResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode list-purposes response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	out := make([]ConsentPurpose, 0, len(result.Data))
	for _, dto := range result.Data {
		out = append(out, c.dtoToConsentPurpose(&dto))
	}

	return out, nil
}

// updateConsentPurpose updates a consent purpose by ID.
func (c *defaultClient) updateConsentPurpose(ctx context.Context, ouID, purposeID string,
	purpose *ConsentPurposeInput) (*ConsentPurpose, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint, purposeID)
	if svcErr != nil {
		return nil, svcErr
	}

	dto := c.consentPurposeInputToDTO(purpose)
	resp, svcErr := c.doRequest(ctx, http.MethodPut, u, ouID, purpose.GroupID, dto)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentPurposeRequest)
	case http.StatusNotFound:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentPurposeNotFound)
	case http.StatusConflict:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentPurposeAlreadyExists)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[purposeResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode update-purpose response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	p := c.dtoToConsentPurpose(result)

	return &p, nil
}

// deleteConsentPurpose deletes a consent purpose by ID.
func (c *defaultClient) deleteConsentPurpose(ctx context.Context,
	ouID, purposeID string) *serviceerror.ServiceError {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint, purposeID)
	if svcErr != nil {
		return svcErr
	}

	resp, svcErr := c.doRequest(ctx, http.MethodDelete, u, ouID, "", nil)
	if svcErr != nil {
		return svcErr
	}
	defer c.closeBody(ctx, resp)

	if resp.StatusCode == http.StatusNotFound {
		return c.handleClientError(ctx, resp, &ErrorConsentPurposeNotFound)
	}

	// Handle conflict error for consent purpose deletion due to it being associated with
	// consent records as a client error
	if resp.StatusCode == http.StatusConflict {
		return c.handleClientError(ctx, resp, &ErrorDeletingConsentPurposeWithAssociatedRecords)
	}

	return c.checkStatus(ctx, resp)
}

// createConsent creates a consent record for a user and resource.
func (c *defaultClient) createConsent(ctx context.Context, ouID string, req *ConsentRequest) (
	*Consent, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentsEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	dto := c.consentRequestToDTO(req)
	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, req.GroupID, dto)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	if resp.StatusCode == http.StatusBadRequest {
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentRecordRequest)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[consentResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode create-consent response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	out := c.dtoToConsent(result)

	return &out, nil
}

// searchConsents retrieves consent records filtered by the given criteria.
func (c *defaultClient) searchConsents(ctx context.Context, ouID string, filter *ConsentSearchFilter) (
	[]Consent, *serviceerror.ServiceError) {
	u, svcErr := c.buildConsentSearchURL(ctx, filter)
	if svcErr != nil {
		return nil, svcErr
	}

	resp, svcErr := c.doRequest(ctx, http.MethodGet, u, ouID, "", nil)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	if resp.StatusCode == http.StatusBadRequest {
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentSearchFilter)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[consentSearchResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode search-consents response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Currently the default client does not apply the filtering correctly for some consent statuses (e.g. EXPIRED)
	// due to the limitations in the consent service API. As a workaround, we apply additional filtering logic here
	// based on the validity time and status to ensure the expected results are returned to the service layer.
	// This can be removed once the consent service API is enhanced to support proper filtering by status.
	statusFilter := map[ConsentStatus]bool{}
	if filter != nil {
		for _, status := range filter.ConsentStatuses {
			statusFilter[status] = true
		}
	}
	applyStatusFilter := len(statusFilter) > 0
	nowUnix := time.Now().Unix()

	out := make([]Consent, 0, len(result.Data))
	for _, dto := range result.Data {
		consent := c.dtoToConsent(&dto)

		// Currently the default client doesn't set the expired status for consents based on the validity time
		// due to the limitations in the consent service API. As a workaround, we set the expired status here
		// based on the validity time to ensure the expected results are returned
		if consent.Status == ConsentStatusActive && consent.ValidityTime > 0 && consent.ValidityTime <= nowUnix {
			consent.Status = ConsentStatusExpired
		}

		if applyStatusFilter {
			if _, ok := statusFilter[consent.Status]; !ok {
				continue
			}
		}

		out = append(out, consent)
	}

	return out, nil
}

// validateConsent validates a consent record by ID and returns the validation result
// along with the consent information if valid.
func (c *defaultClient) validateConsent(ctx context.Context, ouID, consentID string) (
	*ConsentValidationResult, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentValidateEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	payload := consentValidateRequestDTO{ConsentID: consentID}
	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, "", payload)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	if resp.StatusCode == http.StatusBadRequest {
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentValidationRequest)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[consentValidateResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode validate-consent response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return c.dtoToValidationResult(result), nil
}

// updateConsent replaces the content of an existing consent record by ID.
func (c *defaultClient) updateConsent(ctx context.Context, ouID, consentID string,
	req *ConsentRequest) (*Consent, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentsEndpoint, consentID)
	if svcErr != nil {
		return nil, svcErr
	}

	dto := c.consentRequestToDTO(req)
	resp, svcErr := c.doRequest(ctx, http.MethodPut, u, ouID, req.GroupID, dto)
	if svcErr != nil {
		return nil, svcErr
	}
	defer c.closeBody(ctx, resp)

	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, c.handleClientError(ctx, resp, &ErrorInvalidConsentUpdateRequest)
	case http.StatusNotFound:
		return nil, c.handleClientError(ctx, resp, &ErrorConsentRecordNotFound)
	}

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return nil, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[consentResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode update-consent response", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	out := c.dtoToConsent(result)

	return &out, nil
}

// revokeConsent revokes a consent record by ID with an optional reason.
func (c *defaultClient) revokeConsent(ctx context.Context, ouID, consentID string,
	payload *ConsentRevokeRequest) *serviceerror.ServiceError {
	u, svcErr := c.buildServiceEndpoint(ctx, consentsEndpoint, consentID, "revoke")
	if svcErr != nil {
		return svcErr
	}

	dto := consentRevokeDTO{}
	if payload != nil && payload.Reason != "" {
		dto.Reason = payload.Reason
	}

	resp, svcErr := c.doRequest(ctx, http.MethodPut, u, ouID, "", dto)
	if svcErr != nil {
		return svcErr
	}
	defer c.closeBody(ctx, resp)

	if resp.StatusCode == http.StatusBadRequest {
		return c.handleClientError(ctx, resp, &ErrorInvalidConsentRevokeRequest)
	}

	return c.checkStatus(ctx, resp)
}

// --- Helper methods ---

// getClientConfig retrieves the client configuration from the system config with validation and defaulting.
func getClientConfig() clientConfig {
	consentCfg := config.GetServerRuntime().Config.Consent

	timeoutSecs := consentCfg.Timeout
	if timeoutSecs <= 0 {
		timeoutSecs = 5
	}
	maxRetries := consentCfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 3
	}

	return clientConfig{
		baseURL:    strings.TrimRight(consentCfg.BaseURL, "/"),
		timeout:    time.Duration(timeoutSecs) * time.Second,
		maxRetries: maxRetries,
	}
}

// buildServiceEndpoint constructs the full URL for a given set of path segments based on the
// baseURL in clientConfig.
func (c *defaultClient) buildServiceEndpoint(
	ctx context.Context, pathSegments ...string) (string, *serviceerror.ServiceError) {
	u, err := url.JoinPath(c.clientConfig.baseURL, pathSegments...)
	if err != nil {
		c.logger.Error(ctx, "Failed to construct endpoint URL for consent operation", log.Error(err))
		return "", &serviceerror.InternalServerError
	}

	return u, nil
}

// buildSearchURL constructs the URL for searching consent elements and purposes with query parameters
// based on the provided filter.
func (c *defaultClient) buildSearchURL(ctx context.Context, nameFilter string, pathSegments ...string) (
	string, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, pathSegments...)
	if svcErr != nil {
		return "", svcErr
	}

	if nameFilter != "" {
		u += "?name=" + url.QueryEscape(nameFilter)
	}

	return u, nil
}

// buildConsentSearchURL assembles the URL for searching consents with query parameters
// based on the provided ConsentSearchFilter.
func (c *defaultClient) buildConsentSearchURL(ctx context.Context, filter *ConsentSearchFilter) (
	string, *serviceerror.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentsEndpoint)
	if svcErr != nil {
		return "", svcErr
	}

	if filter == nil {
		return u, nil
	}

	params := url.Values{}
	if len(filter.ConsentTypes) > 0 {
		consentTypeStrs := make([]string, 0, len(filter.ConsentTypes))
		for _, ct := range filter.ConsentTypes {
			consentTypeStrs = append(consentTypeStrs, string(ct))
		}
		params.Set("consentTypes", strings.Join(consentTypeStrs, ","))
	}
	if len(filter.ConsentStatuses) > 0 {
		consentStatusStrs := make([]string, 0, len(filter.ConsentStatuses))
		for _, cs := range filter.ConsentStatuses {
			consentStatusStrs = append(consentStatusStrs, string(cs))
		}
		params.Set("consentStatuses", strings.Join(consentStatusStrs, ","))
	}
	if len(filter.GroupIDs) > 0 {
		params.Set("clientIds", strings.Join(filter.GroupIDs, ","))
	}
	if len(filter.UserIDs) > 0 {
		params.Set("userIds", strings.Join(filter.UserIDs, ","))
	}
	if len(filter.PurposeNames) > 0 {
		params.Set("purposeNames", strings.Join(filter.PurposeNames, ","))
	}
	if filter.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", filter.Offset))
	}

	if encoded := params.Encode(); encoded != "" {
		u += "?" + encoded
	}

	return u, nil
}

// consentElementInputToDTO converts a ConsentElementInput to elementCreateDTO for API requests.
func (c *defaultClient) consentElementInputToDTO(el *ConsentElementInput) elementCreateDTO {
	// TODO: Map namespace when the support is implemented in consent service
	return elementCreateDTO{
		Name:        el.Name,
		Description: el.Description,
		Type:        "basic",
		Properties:  el.Properties,
	}
}

// dtoToConsentElement converts an elementResponseDTO from the API response to a ConsentElement.
func (c *defaultClient) dtoToConsentElement(dto *elementResponseDTO) ConsentElement {
	// TODO: Map namespace when the support is implemented in consent service
	return ConsentElement{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Properties:  dto.Properties,
	}
}

// consentPurposeInputToDTO converts a ConsentPurposeInput to purposeCreateDTO for API requests.
func (c *defaultClient) consentPurposeInputToDTO(p *ConsentPurposeInput) purposeCreateDTO {
	elements := make([]purposeElementDTO, 0, len(p.Elements))
	for _, el := range p.Elements {
		elements = append(elements, purposeElementDTO{
			Name:        el.Name,
			IsMandatory: el.IsMandatory,
		})
	}

	return purposeCreateDTO{
		Name:        p.Name,
		Description: p.Description,
		Elements:    elements,
	}
}

// dtoToConsentPurpose converts a purposeResponseDTO from the API response to a ConsentPurpose.
func (c *defaultClient) dtoToConsentPurpose(dto *purposeResponseDTO) ConsentPurpose {
	elements := make([]PurposeElement, 0, len(dto.Elements))
	for _, el := range dto.Elements {
		elements = append(elements, PurposeElement{
			Name:        el.Name,
			IsMandatory: el.IsMandatory,
		})
	}

	return ConsentPurpose{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		GroupID:     dto.ClientID,
		Namespace:   NamespaceFromPurposeName(dto.Name),
		Elements:    elements,
		CreatedTime: dto.CreatedTime,
		UpdatedTime: dto.UpdatedTime,
	}
}

// Consent-purpose name prefixes. Each server-owned purpose is named `<prefix><appID>`; the prefix
// doubles as the namespace discriminator on reads since the upstream consent service has no
// `namespace` field on purposes.
const (
	attributesPurposeNamePrefix  = "attributes:"
	permissionsPurposeNamePrefix = "permissions:"
)

// AttributesPurposeName returns the canonical name of the attribute consent purpose for an app.
func AttributesPurposeName(appID string) string {
	return attributesPurposeNamePrefix + appID
}

// PermissionsPurposeName returns the canonical name of the permission consent purpose for an app.
func PermissionsPurposeName(appID string) string {
	return permissionsPurposeNamePrefix + appID
}

// NamespaceFromPurposeName derives the purpose namespace from the name prefix. Returns empty
// for names without a recognized prefix; callers filter such purposes out.
func NamespaceFromPurposeName(name string) Namespace {
	switch {
	case strings.HasPrefix(name, permissionsPurposeNamePrefix):
		return NamespacePermission
	case strings.HasPrefix(name, attributesPurposeNamePrefix):
		return NamespaceAttribute
	default:
		return ""
	}
}

// FilterAttributePurposes returns only the attribute-namespace consent purposes.
func FilterAttributePurposes(purposes []ConsentPurpose) []ConsentPurpose {
	return filterPurposesByNamespace(purposes, NamespaceAttribute)
}

// FilterPermissionPurposes returns only the permission-namespace consent purposes.
func FilterPermissionPurposes(purposes []ConsentPurpose) []ConsentPurpose {
	return filterPurposesByNamespace(purposes, NamespacePermission)
}

// filterPurposesByNamespace returns the subset of purposes whose Namespace matches ns.
func filterPurposesByNamespace(purposes []ConsentPurpose, ns Namespace) []ConsentPurpose {
	out := make([]ConsentPurpose, 0, len(purposes))
	for _, p := range purposes {
		if p.Namespace == ns {
			out = append(out, p)
		}
	}
	return out
}

// consentAuthorizationRequestToDTO converts a ConsentAuthorizationRequest to authorizationRequestDTO for API requests.
func (c *defaultClient) consentAuthorizationRequestToDTO(a *ConsentAuthorizationRequest) authorizationRequestDTO {
	return authorizationRequestDTO{
		UserID: a.UserID,
		Type:   string(a.Type),
		Status: string(a.Status),
	}
}

// consentRequestToDTO converts a ConsentRequest to consentCreateDTO for API requests.
func (c *defaultClient) consentRequestToDTO(req *ConsentRequest) consentCreateDTO {
	purposes := make([]purposeItemRequestDTO, 0, len(req.Purposes))
	for _, p := range req.Purposes {
		elements := make([]elementApprovalRequestDTO, 0, len(p.Elements))
		for _, el := range p.Elements {
			elements = append(elements, elementApprovalRequestDTO{
				Name:           el.Name,
				IsUserApproved: el.IsUserApproved,
			})
		}

		purposes = append(purposes, purposeItemRequestDTO{
			Name:     p.Name,
			Elements: elements,
		})
	}

	auths := make([]authorizationRequestDTO, 0, len(req.Authorizations))
	for _, a := range req.Authorizations {
		auths = append(auths, c.consentAuthorizationRequestToDTO(&a))
	}

	return consentCreateDTO{
		Type:           string(req.Type),
		ValidityTime:   req.ValidityTime,
		Purposes:       purposes,
		Authorizations: auths,
	}
}

// consentAuthorizationDtoToResponse converts an authorizationResponseDTO from the API response
// to a ConsentAuthorization.
func (c *defaultClient) consentAuthorizationDtoToResponse(a *authorizationResponseDTO) ConsentAuthorization {
	return ConsentAuthorization{
		ID:          a.ID,
		UserID:      a.UserID,
		Type:        ConsentAuthorizationType(a.Type),
		Status:      ConsentAuthorizationStatus(a.Status),
		UpdatedTime: a.UpdatedTime,
	}
}

// dtoToConsent converts a consentResponseDTO from the API response to a Consent.
func (c *defaultClient) dtoToConsent(dto *consentResponseDTO) Consent {
	purposes := make([]ConsentPurposeItem, 0, len(dto.Purposes))
	for _, p := range dto.Purposes {
		elements := make([]ConsentElementApproval, 0, len(p.Elements))
		for _, el := range p.Elements {
			elements = append(elements, ConsentElementApproval{
				Name:           el.Name,
				IsUserApproved: el.IsUserApproved,
			})
		}

		purposes = append(purposes, ConsentPurposeItem{
			Name:     p.Name,
			Elements: elements,
		})
	}

	auths := make([]ConsentAuthorization, 0, len(dto.Authorizations))
	for _, a := range dto.Authorizations {
		auths = append(auths, c.consentAuthorizationDtoToResponse(&a))
	}

	return Consent{
		ID:             dto.ID,
		Type:           ConsentType(dto.Type),
		GroupID:        dto.ClientID,
		Status:         ConsentStatus(dto.Status),
		ValidityTime:   dto.ValidityTime,
		Purposes:       purposes,
		Authorizations: auths,
		CreatedTime:    dto.CreatedTime,
		UpdatedTime:    dto.UpdatedTime,
	}
}

// dtoToValidationResult converts a consentValidateResponseDTO from the API response to a ConsentValidationResult.
func (c *defaultClient) dtoToValidationResult(dto *consentValidateResponseDTO) *ConsentValidationResult {
	if !dto.IsValid {
		return &ConsentValidationResult{IsValid: false}
	}
	consent := c.dtoToConsent(&dto.ConsentInformation)

	return &ConsentValidationResult{
		IsValid:            true,
		ConsentInformation: &consent,
	}
}

// setCommonHeaders applies shared headers to every outbound request.
func (c *defaultClient) setCommonHeaders(req *http.Request, ouID, groupID string) {
	req.Header.Set("org-id", ouID)
	if groupID != "" {
		req.Header.Set("TPP-client-id", groupID)
	}
}

// doRequest marshals body (if non-nil), builds an HTTP request, sets common headers,
// and executes it with retry logic for transient errors.
// The caller is responsible for closing resp.Body via closeBody.
func (c *defaultClient) doRequest(ctx context.Context, method, url, ouID, groupID string, body any) (
	*http.Response, *serviceerror.ServiceError) {
	var encodedBody []byte
	if body != nil {
		var merr error
		encodedBody, merr = json.Marshal(body)
		if merr != nil {
			c.logger.Debug(ctx, "Failed to marshal request body", log.Error(merr))
			return nil, &ErrorInvalidRequestFormat
		}
	}

	var lastErr error
	maxAttempts := c.clientConfig.maxRetries + 1
	backoff := time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				c.logger.Warn(ctx, "Consent request cancelled during retry", log.Error(ctx.Err()))
				return nil, &serviceerror.InternalServerError
			case <-time.After(backoff):
			}
			if backoff < 8*time.Second {
				backoff *= 2
			}
		}

		var bodyReader io.Reader
		if encodedBody != nil {
			bodyReader = bytes.NewReader(encodedBody)
		}

		reqCtx, cancel := context.WithTimeout(ctx, c.clientConfig.timeout)
		req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
		if err != nil {
			cancel()
			c.logger.Error(ctx, "Failed to create HTTP request", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}

		if encodedBody != nil {
			req.Header.Set(sysconst.ContentTypeHeaderName, sysconst.ContentTypeJSON)
		}
		c.setCommonHeaders(req, ouID, groupID)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			c.logger.Warn(ctx, "Error reaching consent service, will retry if attempts remain",
				log.Error(err), log.Int("attempt", attempt+1), log.Int("maxAttempts", maxAttempts))
			continue
		}

		// For 5xx responses, retry if attempts remain; on the last attempt return the
		// response so checkStatus can inspect the final status code and body.
		if resp.StatusCode >= http.StatusInternalServerError {
			c.logger.Warn(ctx, "Consent service returned server error, will retry if attempts remain",
				log.Int("status", resp.StatusCode), log.Int("attempt", attempt+1),
				log.Int("maxAttempts", maxAttempts))

			if attempt < maxAttempts-1 {
				c.closeBody(ctx, resp)
				cancel()
				lastErr = fmt.Errorf("consent service returned %d", resp.StatusCode)
				continue
			}

			// Last attempt: wrap body so cancel fires on Close, keeping the context
			// alive while checkStatus/DecodeJSONResponse reads the payload.
			resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}

		// Success or non-retryable error: wrap body so cancel is deferred to Close.
		resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
		return resp, nil
	}

	c.logger.Error(ctx, "All retry attempts exceeded for consent service request",
		log.String("method", method), log.Error(lastErr))

	return nil, &serviceerror.InternalServerError
}

// checkStatus returns nil for 2xx responses. For all other status codes it decodes
// the structured error body and returns the appropriate *serviceerror.ServiceError:
//   - 5xx → InternalServerError
//   - 4xx → ErrorInvalidConsentRequest
//
// Expected per-method status codes (404, 409) must be checked by the caller BEFORE calling
// this method, as their meaning differs per operation and cannot be resolved here.
func (c *defaultClient) checkStatus(ctx context.Context, resp *http.Response) *serviceerror.ServiceError {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Decode the error response body to extract details for logging
	apiErr, decodeErr := sysutils.DecodeJSONResponse[consentBackendErrorDTO](resp)
	if decodeErr != nil {
		c.logger.Error(ctx, "Failed to decode error response from consent service", log.Error(decodeErr))
	}

	if resp.StatusCode >= 500 {
		if decodeErr != nil {
			c.logger.Error(ctx, "Consent service returned server error",
				log.Int("statusCode", resp.StatusCode))
		} else {
			// Handle conflict error for consent element deletion due to it's being associated with a purpose
			// as a client error
			if apiErr.Code == "CE-5009" {
				c.logger.Debug(ctx,
					"Consent service rejected request due to element being associated with a purpose",
					log.Int("statusCode", resp.StatusCode),
					log.String("code", apiErr.Code),
					log.String("description", apiErr.Description))
				return &ErrorDeletingConsentElementWithAssociatedPurpose
			}

			c.logger.Error(ctx, "Consent service returned server error",
				log.Int("statusCode", resp.StatusCode),
				log.String("code", apiErr.Code),
				log.String("description", apiErr.Description))
		}

		return &serviceerror.InternalServerError
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &ErrorConsentServiceReturnedUnauthorized
	case http.StatusForbidden:
		return &ErrorConsentServiceReturnedForbidden
	default:
		if decodeErr != nil {
			c.logger.Debug(ctx, "Consent service rejected request", log.Int("statusCode", resp.StatusCode))
		} else {
			c.logger.Debug(ctx, "Consent service rejected request",
				log.Int("statusCode", resp.StatusCode),
				log.String("code", apiErr.Code),
				log.String("description", apiErr.Description))
		}
		return &ErrorInvalidConsentRequest
	}
}

// closeBody safely closes a response body.
func (c *defaultClient) closeBody(ctx context.Context, resp *http.Response) {
	if resp != nil && resp.Body != nil {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn(ctx, "Failed to close response body", log.Error(err))
		}
	}
}

// handleClientError decodes the error response from the consent service and returns the provided service error.
func (c *defaultClient) handleClientError(ctx context.Context, resp *http.Response,
	svcErr *serviceerror.ServiceError) *serviceerror.ServiceError {
	apiErr, err := sysutils.DecodeJSONResponse[consentBackendErrorDTO](resp)
	if err != nil {
		c.logger.Debug(ctx, "Consent service rejected request", log.Int("statusCode", resp.StatusCode))
	} else {
		c.logger.Debug(ctx, "Consent service rejected request",
			log.Int("statusCode", resp.StatusCode),
			log.String("code", apiErr.Code),
			log.String("description", apiErr.Description))
	}

	return svcErr
}

// cancelOnCloseBody wraps a response body so the associated context cancel function
// is invoked automatically when the body is closed, keeping the request context
// alive for the full duration of the body read (checkStatus, DecodeJSONResponse, etc.).
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

// Close calls the underlying ReadCloser's Close method and then invokes the cancel function.
func (b *cancelOnCloseBody) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}
