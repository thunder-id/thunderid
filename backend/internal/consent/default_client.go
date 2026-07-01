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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/config"
	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// External service endpoints.
const (
	consentElementsEndpoint = "/consent-elements"
	consentPurposesEndpoint = "/consent-purposes"
	consentsEndpoint        = "/consents"
	consentValidateEndpoint = "/consents/validate"
)

// revokeActionBy is the value sent in the revoke payload's required actionBy field. The
// internal ConsentRevokeRequest does not carry actor identity yet; once the
// interface gains an actor field, source it from there instead.
const revokeActionBy = "user"

// elementTypeBasic is the consent server wire type for elements with simple string values.
// only `basic` is used currently; `json` and `xml` types are not used.
const elementTypeBasic = "basic"

// The `versions` path segment is used in the consent service API to create a new version
const versions = "versions"

// Statuses in bulk element-create results.
const (
	bulkResultStatusSuccess = "SUCCESS"
	bulkResultStatusFailed  = "FAILED"
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
	Namespace   string            `json:"namespace,omitempty"`
	Type        string            `json:"type"`
	DisplayName string            `json:"displayName,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// elementResponseDTO represents the response body for a consent element.
type elementResponseDTO struct {
	ElementID   string            `json:"elementId"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Type        string            `json:"type"`
	Version     string            `json:"version"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Properties  map[string]string `json:"properties"`
	CreatedTime int64             `json:"createdTime"`
}

// bulkElementResultDTO is one entry in the elements bulk-create results array. On
// status=="SUCCESS", Element is populated; on status=="FAILED", Error carries a
// free-form message from the consent service.
type bulkElementResultDTO struct {
	Status  string              `json:"status"`
	Element *elementResponseDTO `json:"element,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// elementsCreateResponseDTO represents the partial-success bulk-create response.
type elementsCreateResponseDTO struct {
	Results []bulkElementResultDTO `json:"results"`
}

// listMetadataDTO is the pagination metadata block on list responses.
type listMetadataDTO struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
	Limit  int `json:"limit"`
}

// elementListResponseDTO represents the response body for listing consent elements.
type elementListResponseDTO struct {
	Data     []elementResponseDTO `json:"data"`
	Metadata listMetadataDTO      `json:"metadata"`
}

// purposeElementDTO represents a consent element reference within a consent purpose.
type purposeElementDTO struct {
	ElementID string `json:"elementId,omitempty"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
	Mandatory bool   `json:"mandatory"`
}

// purposeCreateDTO represents the request body for creating a consent purpose. GroupID is
// not in the body — it is carried in the group-id header.
type purposeCreateDTO struct {
	Name        string              `json:"name"`
	DisplayName string              `json:"displayName,omitempty"`
	Description string              `json:"description,omitempty"`
	Properties  map[string]string   `json:"properties,omitempty"`
	Elements    []purposeElementDTO `json:"elements"`
}

// purposeVersionCreateDTO represents the request body for creating a new version of an
// existing consent purpose. Name is immutable and is therefore omitted from the body.
type purposeVersionCreateDTO struct {
	DisplayName string              `json:"displayName,omitempty"`
	Description string              `json:"description,omitempty"`
	Properties  map[string]string   `json:"properties,omitempty"`
	Elements    []purposeElementDTO `json:"elements"`
}

// purposeResponseDTO represents the response body for a consent purpose.
type purposeResponseDTO struct {
	PurposeID   string              `json:"purposeId"`
	Name        string              `json:"name"`
	GroupID     string              `json:"groupId"`
	Version     string              `json:"version"`
	DisplayName string              `json:"displayName"`
	Description string              `json:"description"`
	Properties  map[string]string   `json:"properties"`
	Elements    []purposeElementDTO `json:"elements"`
	CreatedTime int64               `json:"createdTime"`
	UpdatedTime int64               `json:"updatedTime"`
}

// purposeListResponseDTO represents the response body for listing consent purposes.
type purposeListResponseDTO struct {
	Data     []purposeResponseDTO `json:"data"`
	Metadata listMetadataDTO      `json:"metadata"`
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
	Version  string                      `json:"version,omitempty"`
	Elements []elementApprovalRequestDTO `json:"elements"`
}

// elementApprovalRequestDTO represents a consent element approval entry in the consent create/update request.
type elementApprovalRequestDTO struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Approved  bool   `json:"approved"`
}

// consentCreateDTO represents the request body for creating a consent record.
type consentCreateDTO struct {
	Type                       string                    `json:"type"`
	ExpirationTime             int64                     `json:"expirationTime"`
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
	ElementID string `json:"elementId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Approved  bool   `json:"approved"`
	Mandatory bool   `json:"mandatory"`
}

// purposeItemResponseDTO represents a consent purpose item in the consent response.
type purposeItemResponseDTO struct {
	PurposeID string                       `json:"purposeId"`
	Name      string                       `json:"name"`
	Version   string                       `json:"version"`
	Elements  []elementApprovalResponseDTO `json:"elements"`
}

// consentResponseDTO represents the response body for a consent record.
type consentResponseDTO struct {
	ID                         string                     `json:"id"`
	Type                       string                     `json:"type"`
	GroupID                    string                     `json:"groupId"`
	Status                     string                     `json:"status"`
	ExpirationTime             int64                      `json:"expirationTime"`
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
	Data     []consentResponseDTO `json:"data"`
	Metadata listMetadataDTO      `json:"metadata"`
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
	ActionBy         string `json:"actionBy"`
	RevocationReason string `json:"revocationReason,omitempty"`
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
	[]ConsentElement, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	out := make([]ConsentElement, 0, len(result.Results))
	for i, r := range result.Results {
		if r.Status == bulkResultStatusFailed {
			c.logger.Debug(ctx, "Consent service rejected element in bulk-create",
				log.Int("index", i),
				log.String("error", r.Error))
			return nil, bulkResultErrorToServiceError(r.Error)
		}
		if r.Element == nil {
			c.logger.Error(ctx, "Bulk-create result missing element for SUCCESS item",
				log.Int("index", i))
			return nil, &tidcommon.InternalServerError
		}
		out = append(out, c.dtoToConsentElement(r.Element))
	}

	return out, nil
}

// bulkResultErrorToServiceError maps a per-item bulk failure to a Thunder service error.
// The consent service returns a free-form message — substrings indicating a duplicate
// element map to the conflict error; everything else falls back to the generic
// invalid-element-request error.
func bulkResultErrorToServiceError(msg string) *tidcommon.ServiceError {
	if strings.Contains(msg, "already exists") {
		return &ErrorConsentElementAlreadyExists
	}
	return &ErrorInvalidConsentElementRequest
}

// listConsentElements retrieves consent elements filtered by optional name.
func (c *defaultClient) listConsentElements(
	ctx context.Context, ouID string, ns providers.Namespace, nameFilter string) (
	[]ConsentElement, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	out := make([]ConsentElement, 0, len(result.Data))
	for _, dto := range result.Data {
		out = append(out, c.dtoToConsentElement(&dto))
	}

	return out, nil
}

// deleteConsentElement deletes a consent element by ID.
func (c *defaultClient) deleteConsentElement(ctx context.Context,
	ouID, elementID string) *tidcommon.ServiceError {
	u, svcErr := c.buildServiceEndpoint(ctx, consentElementsEndpoint, elementID, versions, "v1")
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
	[]string, *tidcommon.ServiceError) {
	valid := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		matched, svcErr := c.elementNameExists(ctx, ouID, name)
		if svcErr != nil {
			return nil, svcErr
		}
		if matched {
			valid = append(valid, name)
		}
	}

	return valid, nil
}

// elementNameExists returns true if at least one consent element in the organization has
// the given exact name.
func (c *defaultClient) elementNameExists(ctx context.Context, ouID, name string) (
	bool, *tidcommon.ServiceError) {
	u, svcErr := c.buildSearchURL(ctx, name, consentElementsEndpoint)
	if svcErr != nil {
		return false, svcErr
	}

	resp, svcErr := c.doRequest(ctx, http.MethodGet, u, ouID, "", nil)
	if svcErr != nil {
		return false, svcErr
	}
	defer c.closeBody(ctx, resp)

	if svcErr := c.checkStatus(ctx, resp); svcErr != nil {
		return false, svcErr
	}

	result, err := sysutils.DecodeJSONResponse[elementListResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode list-elements response during validate",
			log.Error(err))
		return false, &tidcommon.InternalServerError
	}

	for _, dto := range result.Data {
		if dto.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// createConsentPurpose creates a consent purpose.
func (c *defaultClient) createConsentPurpose(ctx context.Context, ouID string, purpose *ConsentPurposeInput) (
	*ConsentPurpose, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}
	p := c.dtoToConsentPurpose(result)

	return &p, nil
}

// listConsentPurposes retrieves consent purposes for the given organization, optionally filtered by
// group ID (e.g. app ID). If groupID is empty, returns all purposes for the organization.
func (c *defaultClient) listConsentPurposes(ctx context.Context, ouID, groupID string) (
	[]ConsentPurpose, *tidcommon.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint)
	if svcErr != nil {
		return nil, svcErr
	}

	if groupID != "" {
		u += "?groupIds=" + url.QueryEscape(groupID)
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
		return nil, &tidcommon.InternalServerError
	}

	// v0.3.0 list omits per-purpose elements; fetch each by ID to populate them. Callers
	// (consent prompt building, attribute-purpose sync) compare against Elements, so an
	// empty slice would silently break those flows.
	out := make([]ConsentPurpose, 0, len(result.Data))
	for _, dto := range result.Data {
		full, svcErr := c.fetchConsentPurpose(ctx, ouID, dto.PurposeID)
		if svcErr != nil {
			return nil, svcErr
		}
		out = append(out, c.dtoToConsentPurpose(full))
	}

	return out, nil
}

// fetchConsentPurpose retrieves a single consent purpose by ID, including its elements.
func (c *defaultClient) fetchConsentPurpose(ctx context.Context, ouID, purposeID string) (
	*purposeResponseDTO, *tidcommon.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint, purposeID)
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

	result, err := sysutils.DecodeJSONResponse[purposeResponseDTO](resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to decode get-purpose response", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return result, nil
}

// updateConsentPurpose updates a consent purpose by ID. In v0.3.0 the underlying call
// creates a new immutable version (POST /consent-purposes/{id}/versions) — Thunder callers
// see the new version as the latest, preserving previous edit-in-place read semantics.
func (c *defaultClient) updateConsentPurpose(ctx context.Context, ouID, purposeID string,
	purpose *ConsentPurposeInput) (*ConsentPurpose, *tidcommon.ServiceError) {
	u, svcErr := c.buildServiceEndpoint(ctx, consentPurposesEndpoint, purposeID, versions)
	if svcErr != nil {
		return nil, svcErr
	}

	dto := c.consentPurposeInputToVersionDTO(purpose)
	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, purpose.GroupID, dto)
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
		return nil, &tidcommon.InternalServerError
	}
	p := c.dtoToConsentPurpose(result)

	return &p, nil
}

// createConsent creates a consent record for a user and resource.
func (c *defaultClient) createConsent(ctx context.Context, ouID string, req *ConsentRequest) (
	*providers.Consent, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}
	out := c.dtoToConsent(result)

	return &out, nil
}

// searchConsents retrieves consent records filtered by the given criteria.
func (c *defaultClient) searchConsents(ctx context.Context, ouID string, filter *ConsentSearchFilter) (
	[]providers.Consent, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	// Currently the default client does not apply the filtering correctly for some consent statuses (e.g. EXPIRED)
	// due to the limitations in the consent service API. As a workaround, we apply additional filtering logic here
	// based on the validity time and status to ensure the expected results are returned to the service layer.
	// This can be removed once the consent service API is enhanced to support proper filtering by status.
	statusFilter := map[providers.ConsentStatus]bool{}
	if filter != nil {
		for _, status := range filter.ConsentStatuses {
			statusFilter[status] = true
		}
	}
	applyStatusFilter := len(statusFilter) > 0
	nowUnix := time.Now().Unix()

	out := make([]providers.Consent, 0, len(result.Data))
	for _, dto := range result.Data {
		consent := c.dtoToConsent(&dto)

		// Currently the default client doesn't set the expired status for consents based on the validity time
		// due to the limitations in the consent service API. As a workaround, we set the expired status here
		// based on the validity time to ensure the expected results are returned
		if consent.Status == providers.ConsentStatusActive &&
			consent.ValidityTime > 0 && consent.ValidityTime <= nowUnix {
			consent.Status = providers.ConsentStatusExpired
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
	*ConsentValidationResult, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	return c.dtoToValidationResult(result), nil
}

// updateConsent replaces the content of an existing consent record by ID.
func (c *defaultClient) updateConsent(ctx context.Context, ouID, consentID string,
	req *ConsentRequest) (*providers.Consent, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}
	out := c.dtoToConsent(result)

	return &out, nil
}

// revokeConsent revokes a consent record by ID with an optional reason.
func (c *defaultClient) revokeConsent(ctx context.Context, ouID, consentID string,
	payload *ConsentRevokeRequest) *tidcommon.ServiceError {
	u, svcErr := c.buildServiceEndpoint(ctx, consentsEndpoint, consentID, "revoke")
	if svcErr != nil {
		return svcErr
	}

	dto := consentRevokeDTO{ActionBy: revokeActionBy}
	if payload != nil && payload.Reason != "" {
		dto.RevocationReason = payload.Reason
	}

	resp, svcErr := c.doRequest(ctx, http.MethodPost, u, ouID, "", dto)
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
	ctx context.Context, pathSegments ...string) (string, *tidcommon.ServiceError) {
	u, err := url.JoinPath(c.clientConfig.baseURL, pathSegments...)
	if err != nil {
		c.logger.Error(ctx, "Failed to construct endpoint URL for consent operation", log.Error(err))
		return "", &tidcommon.InternalServerError
	}

	return u, nil
}

// buildSearchURL constructs the URL for searching consent elements and purposes with query parameters
// based on the provided filter.
func (c *defaultClient) buildSearchURL(ctx context.Context, nameFilter string, pathSegments ...string) (
	string, *tidcommon.ServiceError) {
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
	string, *tidcommon.ServiceError) {
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
		params.Set("groupIds", strings.Join(filter.GroupIDs, ","))
	}
	if len(filter.UserIDs) > 0 {
		params.Set("userIds", strings.Join(filter.UserIDs, ","))
	}
	if filter.PurposeName != "" {
		params.Set("purposeName", filter.PurposeName)
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
	return elementCreateDTO{
		Name:        el.Name,
		Namespace:   string(el.Namespace),
		Type:        elementTypeBasic,
		DisplayName: el.Description,
		Description: el.Description,
		Properties:  el.Properties,
	}
}

// dtoToConsentElement converts an elementResponseDTO from the API response to a ConsentElement.
func (c *defaultClient) dtoToConsentElement(dto *elementResponseDTO) ConsentElement {
	return ConsentElement{
		ID:          dto.ElementID,
		Name:        dto.Name,
		Description: dto.Description,
		Namespace:   providers.Namespace(dto.Namespace),
		Properties:  dto.Properties,
	}
}

// consentPurposeInputToDTO converts a ConsentPurposeInput to purposeCreateDTO for API requests.
func (c *defaultClient) consentPurposeInputToDTO(p *ConsentPurposeInput) purposeCreateDTO {
	return purposeCreateDTO{
		Name:        p.Name,
		DisplayName: p.Description,
		Description: p.Description,
		Elements:    c.purposeElementsInputToDTO(p.Elements),
	}
}

// consentPurposeInputToVersionDTO converts a ConsentPurposeInput to purposeVersionCreateDTO
// for the POST /consent-purposes/{id}/versions endpoint. Name is immutable and is not sent
// in the body.
func (c *defaultClient) consentPurposeInputToVersionDTO(p *ConsentPurposeInput) purposeVersionCreateDTO {
	return purposeVersionCreateDTO{
		DisplayName: p.Description,
		Description: p.Description,
		Elements:    c.purposeElementsInputToDTO(p.Elements),
	}
}

// purposeElementsInputToDTO maps the Thunder PurposeElement slice to the wire shape.
func (c *defaultClient) purposeElementsInputToDTO(in []PurposeElement) []purposeElementDTO {
	out := make([]purposeElementDTO, 0, len(in))
	for _, el := range in {
		out = append(out, purposeElementDTO{
			Name:      el.Name,
			Namespace: string(el.Namespace),
			Mandatory: el.IsMandatory,
		})
	}
	return out
}

// dtoToConsentPurpose converts a purposeResponseDTO from the API response to a ConsentPurpose.
func (c *defaultClient) dtoToConsentPurpose(dto *purposeResponseDTO) ConsentPurpose {
	elements := make([]PurposeElement, 0, len(dto.Elements))
	for _, el := range dto.Elements {
		elements = append(elements, PurposeElement{
			Name:        el.Name,
			Namespace:   providers.Namespace(el.Namespace),
			IsMandatory: el.Mandatory,
		})
	}

	return ConsentPurpose{
		ID:          dto.PurposeID,
		Name:        dto.Name,
		Description: dto.Description,
		GroupID:     dto.GroupID,
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
func NamespaceFromPurposeName(name string) providers.Namespace {
	switch {
	case strings.HasPrefix(name, permissionsPurposeNamePrefix):
		return providers.NamespacePermission
	case strings.HasPrefix(name, attributesPurposeNamePrefix):
		return providers.NamespaceAttribute
	default:
		return ""
	}
}

// FilterAttributePurposes returns only the attribute-namespace consent purposes.
func FilterAttributePurposes(purposes []ConsentPurpose) []ConsentPurpose {
	return filterPurposesByNamespace(purposes, providers.NamespaceAttribute)
}

// FilterPermissionPurposes returns only the permission-namespace consent purposes.
func FilterPermissionPurposes(purposes []ConsentPurpose) []ConsentPurpose {
	return filterPurposesByNamespace(purposes, providers.NamespacePermission)
}

// filterPurposesByNamespace returns the subset of purposes whose Namespace matches ns.
func filterPurposesByNamespace(purposes []ConsentPurpose, ns providers.Namespace) []ConsentPurpose {
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
				Name:      el.Name,
				Namespace: string(el.Namespace),
				Approved:  el.IsUserApproved,
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
		ExpirationTime: req.ValidityTime,
		Purposes:       purposes,
		Authorizations: auths,
	}
}

// consentAuthorizationDtoToResponse converts an authorizationResponseDTO from the API response
// to a ConsentAuthorization.
func (c *defaultClient) consentAuthorizationDtoToResponse(a *authorizationResponseDTO) providers.ConsentAuthorization {
	return providers.ConsentAuthorization{
		ID:          a.ID,
		UserID:      a.UserID,
		Type:        providers.ConsentAuthorizationType(a.Type),
		Status:      providers.ConsentAuthorizationStatus(a.Status),
		UpdatedTime: a.UpdatedTime,
	}
}

// dtoToConsent converts a consentResponseDTO from the API response to a Consent.
func (c *defaultClient) dtoToConsent(dto *consentResponseDTO) providers.Consent {
	purposes := make([]providers.ConsentPurposeItem, 0, len(dto.Purposes))
	for _, p := range dto.Purposes {
		elements := make([]providers.ConsentElementApproval, 0, len(p.Elements))
		for _, el := range p.Elements {
			elements = append(elements, providers.ConsentElementApproval{
				Name:           el.Name,
				Namespace:      providers.Namespace(el.Namespace),
				IsUserApproved: el.Approved,
			})
		}

		purposes = append(purposes, providers.ConsentPurposeItem{
			Name:     p.Name,
			Elements: elements,
		})
	}

	auths := make([]providers.ConsentAuthorization, 0, len(dto.Authorizations))
	for _, a := range dto.Authorizations {
		auths = append(auths, c.consentAuthorizationDtoToResponse(&a))
	}

	return providers.Consent{
		ID:             dto.ID,
		Type:           providers.ConsentType(dto.Type),
		GroupID:        dto.GroupID,
		Status:         providers.ConsentStatus(dto.Status),
		ValidityTime:   dto.ExpirationTime,
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
		req.Header.Set("group-id", groupID)
	}
}

// doRequest marshals body (if non-nil), builds an HTTP request, sets common headers,
// and executes it with retry logic for transient errors.
// The caller is responsible for closing resp.Body via closeBody.
func (c *defaultClient) doRequest(ctx context.Context, method, url, ouID, groupID string, body any) (
	*http.Response, *tidcommon.ServiceError) {
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
				return nil, &tidcommon.InternalServerError
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
			return nil, &tidcommon.InternalServerError
		}

		if encodedBody != nil {
			req.Header.Set(sysconst.ContentTypeHeaderName, sysconst.ContentTypeJSON)
		}
		req.Header.Set(sysconst.CorrelationIDHeaderName, sysContext.GetTraceID(reqCtx))
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

	return nil, &tidcommon.InternalServerError
}

// checkStatus returns nil for 2xx responses. For all other status codes it decodes
// the structured error body and returns the appropriate *tidcommon.ServiceError:
//   - 5xx → InternalServerError
//   - 4xx → ErrorInvalidConsentRequest
//
// Expected per-method status codes (404, 409) must be checked by the caller BEFORE calling
// this method, as their meaning differs per operation and cannot be resolved here.
func (c *defaultClient) checkStatus(ctx context.Context, resp *http.Response) *tidcommon.ServiceError {
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

		return &tidcommon.InternalServerError
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
	svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
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
