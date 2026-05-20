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

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	systemhttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// restAuthnProvider is an authentication provider that communicates with an external service via REST.
type restAuthnProvider struct {
	baseURL    string
	apiKey     string
	httpClient systemhttp.HTTPClientInterface
	logger     *log.Logger
}

// AuthenticateRequest is the request body for the authentication endpoint.
type AuthenticateRequest struct {
	Identifiers map[string]interface{}         `json:"identifiers"`
	Credentials map[string]interface{}         `json:"credentials"`
	Metadata    *authnprovidercm.AuthnMetadata `json:"metadata"`
}

// GetAttributesRequest is the request body for the attributes endpoint.
type GetAttributesRequest struct {
	Token               string                                 `json:"token"`
	RequestedAttributes *authnprovidercm.RequestedAttributes   `json:"requestedAttributes"`
	Metadata            *authnprovidercm.GetAttributesMetadata `json:"metadata"`
}

type apiErrorResponse struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"`
}

// newRestAuthnProvider creates a new REST authentication provider.
func newRestAuthnProvider(baseURL, apiKey string, httpClient systemhttp.HTTPClientInterface) AuthnProviderInterface {
	return &restAuthnProvider{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RestAuthnProvider")),
	}
}

// Authenticate authenticates a user.
func (p *restAuthnProvider) Authenticate(ctx context.Context, identifiers, credentials map[string]interface{},
	metadata *authnprovidercm.AuthnMetadata) (*authnprovidercm.AuthnResult, *serviceerror.ServiceError) {
	reqBody := AuthenticateRequest{
		Identifiers: identifiers,
		Credentials: credentials,
		Metadata:    metadata,
	}
	return postAndDecode[authnprovidercm.AuthnResult](p, ctx, p.baseURL+"/authenticate", reqBody)
}

// GetAttributes retrieves the attributes of a user.
func (p *restAuthnProvider) GetAttributes(ctx context.Context, token string,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata) (
	*authnprovidercm.GetAttributesResult, *serviceerror.ServiceError) {
	reqBody := GetAttributesRequest{
		Token:               token,
		RequestedAttributes: requestedAttributes,
		Metadata:            metadata,
	}
	return postAndDecode[authnprovidercm.GetAttributesResult](p, ctx, p.baseURL+"/attributes", reqBody)
}

// postAndDecode marshals reqBody as JSON, posts it to url, and decodes the response into T.
func postAndDecode[T any](p *restAuthnProvider, ctx context.Context, url string,
	reqBody interface{}) (*T, *serviceerror.ServiceError) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, p.logAndReturnServerError("Failed to marshal request", log.String("error", err.Error()))
	}

	resp, err := p.doRequest(ctx, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, p.logAndReturnServerError("Failed to send request", log.String("error", err.Error()))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		var result T
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, p.logAndReturnServerError("Failed to decode response", log.String("error", err.Error()))
		}
		return &result, nil
	}

	return nil, p.decodeError(resp.Body, resp.StatusCode)
}

func (p *restAuthnProvider) logAndReturnServerError(msg string, fields ...log.Field) *serviceerror.ServiceError {
	p.logger.Error(msg, fields...)
	err := serviceerror.InternalServerError
	return &err
}

func isClientError(statusCode int, code string) bool {
	return (statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError) ||
		code == authnprovidercm.ErrorCodeAuthenticationFailed ||
		code == authnprovidercm.ErrorCodeUserNotFound ||
		code == authnprovidercm.ErrorCodeInvalidToken ||
		code == authnprovidercm.ErrorCodeInvalidRequest
}

func (p *restAuthnProvider) decodeError(body io.Reader, statusCode int) *serviceerror.ServiceError {
	var apiErr apiErrorResponse
	if err := json.NewDecoder(body).Decode(&apiErr); err != nil {
		return p.logAndReturnServerError("Failed to decode error response from authn provider",
			log.String("error", err.Error()))
	}

	errorType := serviceerror.ServerErrorType
	if isClientError(statusCode, apiErr.Code) {
		errorType = serviceerror.ClientErrorType
	}

	if errorType == serviceerror.ServerErrorType {
		return p.logAndReturnServerError("Authn provider returned server error",
			log.String("code", apiErr.Code), log.String("message", apiErr.Message),
			log.String("description", apiErr.Description))
	}

	return &serviceerror.ServiceError{
		Type: errorType,
		Code: apiErr.Code,
		Error: core.I18nMessage{
			Key:          "error.authnproviderservice." + apiErr.Code,
			DefaultValue: apiErr.Message,
		},
		ErrorDescription: core.I18nMessage{
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
	if p.apiKey != "" {
		req.Header.Set("API-KEY", p.apiKey)
	}
	return p.httpClient.Do(req)
}
