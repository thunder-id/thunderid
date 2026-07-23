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

package restprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	systemhttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// restAuthnProvider is an authentication provider that communicates with an external service via REST.
type restAuthnProvider struct {
	baseURL             string
	apiKey              string
	correlationIDHeader string
	httpClient          systemhttp.HTTPClientInterface
	logger              *log.Logger
}

// AuthenticateRequest is the request body for the authentication endpoint.
type AuthenticateRequest struct {
	Identifiers map[string]interface{}   `json:"identifiers"`
	Credentials map[string]interface{}   `json:"credentials"`
	Metadata    *providers.AuthnMetadata `json:"metadata"`
}

// GetAttributesRequest is the request body for the attributes endpoint.
type GetAttributesRequest struct {
	Token               any                              `json:"token"`
	RequestedAttributes *providers.RequestedAttributes   `json:"requestedAttributes"`
	Metadata            *providers.GetAttributesMetadata `json:"metadata"`
}

// InitiateRequest is the request body for the initiate-authentication and initiate-enrollment endpoints.
type InitiateRequest struct {
	CredentialType string                   `json:"credentialType"`
	InitData       any                      `json:"initData"`
	Metadata       *providers.AuthnMetadata `json:"metadata"`
}

// EnrollRequest is the request body for the enrollment endpoint.
type EnrollRequest struct {
	Identifiers map[string]interface{}   `json:"identifiers"`
	Credentials map[string]interface{}   `json:"credentials"`
	Metadata    *providers.AuthnMetadata `json:"metadata"`
}

type apiErrorResponse struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"`
}

// newRestAuthnProvider creates a new REST authentication provider.
func newRestAuthnProvider(baseURL, apiKey, correlationIDHeader string,
	httpClient systemhttp.HTTPClientInterface) provider.AuthnProviderInterface {
	return &restAuthnProvider{
		baseURL:             baseURL,
		apiKey:              apiKey,
		correlationIDHeader: correlationIDHeader,
		httpClient:          httpClient,
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RestAuthnProvider")),
	}
}

// InitiateAuthentication initiates authentication for a credential type via the external service.
// The response payload is provider-defined, so it is passed through as raw JSON for the caller to decode.
func (p *restAuthnProvider) InitiateAuthentication(ctx context.Context, credentialType string, initData any,
	metadata *providers.AuthnMetadata) (any, *tidcommon.ServiceError) {
	reqBody := InitiateRequest{
		CredentialType: credentialType,
		InitData:       initData,
		Metadata:       metadata,
	}
	result, svcErr := postAndDecode[json.RawMessage](p, ctx, p.baseURL+"/initiate-authentication", reqBody)
	if svcErr != nil {
		return nil, svcErr
	}
	return *result, nil
}

// Authenticate authenticates a user.
func (p *restAuthnProvider) Authenticate(ctx context.Context, identifiers, credentials map[string]interface{},
	metadata *providers.AuthnMetadata) (*providers.AuthnResult, *tidcommon.ServiceError) {
	reqBody := AuthenticateRequest{
		Identifiers: identifiers,
		Credentials: credentials,
		Metadata:    metadata,
	}
	return postAndDecode[providers.AuthnResult](p, ctx, p.baseURL+"/authenticate", reqBody)
}

// GetEntityReference retrieves the entity reference from the external service.
func (p *restAuthnProvider) GetEntityReference(ctx context.Context, entityReferenceToken any,
) (*providers.EntityReference, *tidcommon.ServiceError) {
	return postAndDecode[providers.EntityReference](p, ctx, p.baseURL+"/entity-reference",
		entityReferenceToken)
}

// GetAttributes retrieves the attributes of a user.
func (p *restAuthnProvider) GetAttributes(ctx context.Context, token any,
	requestedAttributes *providers.RequestedAttributes,
	metadata *providers.GetAttributesMetadata) (
	*providers.AttributesResponse, *tidcommon.ServiceError) {
	reqBody := GetAttributesRequest{
		Token:               token,
		RequestedAttributes: requestedAttributes,
		Metadata:            metadata,
	}
	return postAndDecode[providers.AttributesResponse](p, ctx, p.baseURL+"/attributes", reqBody)
}

// InitiateEnrollment initiates credential enrollment for a credential type via the external service.
// The response payload is provider-defined, so it is passed through as raw JSON for the caller to decode.
func (p *restAuthnProvider) InitiateEnrollment(ctx context.Context, credentialType string, initData any,
	metadata *providers.AuthnMetadata) (any, *tidcommon.ServiceError) {
	reqBody := InitiateRequest{
		CredentialType: credentialType,
		InitData:       initData,
		Metadata:       metadata,
	}
	result, svcErr := postAndDecode[json.RawMessage](p, ctx, p.baseURL+"/initiate-enrollment", reqBody)
	if svcErr != nil {
		return nil, svcErr
	}
	return *result, nil
}

// Enroll enrolls a credential for a user via the external service.
func (p *restAuthnProvider) Enroll(ctx context.Context, identifiers, credentials map[string]interface{},
	metadata *providers.AuthnMetadata) (*providers.AuthnResult, *tidcommon.ServiceError) {
	reqBody := EnrollRequest{
		Identifiers: identifiers,
		Credentials: credentials,
		Metadata:    metadata,
	}
	return postAndDecode[providers.AuthnResult](p, ctx, p.baseURL+"/enroll", reqBody)
}

// postAndDecode marshals reqBody as JSON, posts it to url, and decodes the response into T.
func postAndDecode[T any](p *restAuthnProvider, ctx context.Context, url string,
	reqBody interface{}) (*T, *tidcommon.ServiceError) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to marshal request", log.String("error", err.Error()))
	}

	resp, err := p.doRequest(ctx, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, p.logAndReturnServerError(ctx, "Failed to send request", log.String("error", err.Error()))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		var result T
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, p.logAndReturnServerError(ctx, "Failed to decode response", log.String("error", err.Error()))
		}
		return &result, nil
	}

	return nil, p.decodeError(ctx, resp.Body, resp.StatusCode)
}

func (p *restAuthnProvider) logAndReturnServerError(
	ctx context.Context, msg string, fields ...log.Field) *tidcommon.ServiceError {
	p.logger.Error(ctx, msg, fields...)
	err := tidcommon.InternalServerError
	return &err
}

func isClientError(statusCode int, code string) bool {
	return (statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError) ||
		code == authnprovidercm.ErrorCodeAuthenticationFailed ||
		code == authnprovidercm.ErrorCodeEnrollmentFailed ||
		code == authnprovidercm.ErrorCodeUserNotFound ||
		code == authnprovidercm.ErrorCodeInvalidToken ||
		code == authnprovidercm.ErrorCodeInvalidRequest
}

func (p *restAuthnProvider) decodeError(
	ctx context.Context, body io.Reader, statusCode int) *tidcommon.ServiceError {
	var apiErr apiErrorResponse
	if err := json.NewDecoder(body).Decode(&apiErr); err != nil {
		return p.logAndReturnServerError(ctx, "Failed to decode error response from authn provider",
			log.String("error", err.Error()))
	}

	errorType := tidcommon.ServerErrorType
	if isClientError(statusCode, apiErr.Code) {
		errorType = tidcommon.ClientErrorType
	}

	if errorType == tidcommon.ServerErrorType {
		return p.logAndReturnServerError(ctx, "Authn provider returned server error",
			log.String("code", apiErr.Code), log.String("message", apiErr.Message),
			log.String("description", apiErr.Description))
	}

	return &tidcommon.ServiceError{
		Type: errorType,
		Code: apiErr.Code,
		Error: tidcommon.I18nMessage{
			Key:          "error.authnproviderservice." + apiErr.Code,
			DefaultValue: apiErr.Message,
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnproviderservice." + apiErr.Code + "_description",
			DefaultValue: apiErr.Description,
		},
	}
}

func (p *restAuthnProvider) doRequest(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(p.correlationIDHeader, sysContext.GetTraceID(ctx))
	if p.apiKey != "" {
		req.Header.Set("API-KEY", p.apiKey)
	}
	return p.httpClient.Do(req)
}
