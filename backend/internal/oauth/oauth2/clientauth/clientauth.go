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

// Package clientauth provides shared client authentication logic for OAuth2 endpoints.
package clientauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// authenticate authenticates the OAuth2 client from the request.
// It extracts credentials, validates them, and returns OAuthClientInfo on success.
// The endpointURL is used as the expected audience when validating client assertion JWTs.
// Returns an authError on failure.
func authenticate(
	ctx context.Context,
	r *http.Request,
	inboundClient inboundclient.InboundClientServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	jwtService jwt.JWTServiceInterface,
	endpointURL string,
) (*OAuthClientInfo, *authError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientAuthMiddleware"))

	// Extract all possible auth fields
	hasAuthHeader := r.Header.Get(serverconst.AuthorizationHeaderName) != ""
	clientIDFromBody := r.FormValue(constants.RequestParamClientID)
	clientSecretFromBody := r.FormValue(constants.RequestParamClientSecret)
	clientAssertionType := r.FormValue(constants.RequestParamClientAssertionType)
	clientAssertion := r.FormValue(constants.RequestParamClientAssertion)

	var detectedMethod constants.TokenEndpointAuthMethod

	// Method 1: Basic Auth (header)
	if hasAuthHeader {
		detectedMethod = constants.TokenEndpointAuthMethodClientSecretBasic
	}

	// Method 2: Client credentials in body
	if clientSecretFromBody != "" {
		if detectedMethod != "" {
			return nil, errMultipleAuthMethods
		}
		detectedMethod = constants.TokenEndpointAuthMethodClientSecretPost
	}

	// Method 3: Client assertion (private_key_jwt)
	if clientAssertionType != "" || clientAssertion != "" {
		if detectedMethod != "" {
			return nil, errMultipleAuthMethods
		}
		detectedMethod = constants.TokenEndpointAuthMethodPrivateKeyJWT
	}

	// If no auth method but client_id exists -> public client
	if detectedMethod == "" && clientIDFromBody != "" {
		detectedMethod = constants.TokenEndpointAuthMethodNone
	}

	// Now process based on detected method
	var clientID string
	var clientSecret string

	switch detectedMethod {
	case constants.TokenEndpointAuthMethodClientSecretBasic:
		var err *authError
		clientID, clientSecret, err = extractBasicAuthCredentials(r)
		if err != nil {
			return nil, err
		}

	case constants.TokenEndpointAuthMethodClientSecretPost:
		if clientIDFromBody == "" {
			return nil, errMissingClientID
		}
		clientID = clientIDFromBody
		clientSecret = clientSecretFromBody

	case constants.TokenEndpointAuthMethodPrivateKeyJWT:
		if clientAssertionType != constants.SupportedClientAssertionType {
			logger.Debug("Invalid client assertion: unsupported client assertion type")
			return nil, errInvalidClientAssertion
		}
		extracted, err := extractClientIDFromAssertion(clientAssertion)
		if err != nil {
			return nil, err
		}
		clientID = extracted

	case constants.TokenEndpointAuthMethodNone:
		clientID = clientIDFromBody

	default:
		return nil, errMissingClientID
	}

	if clientIDFromBody != "" && clientID != clientIDFromBody {
		return nil, errClientIDMismatch
	}

	oauthApp, err := inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		logger.Error("Failed to retrieve OAuth client", log.Error(err), log.MaskedString("clientID", clientID))
		return nil, errInvalidClientCredentials
	}
	if oauthApp == nil {
		return nil, errInvalidClientCredentials
	}

	if !oauthApp.IsAllowedTokenEndpointAuthMethod(detectedMethod) {
		return nil, errUnauthorizedAuthMethod
	}

	// Validate credentials based on method
	switch detectedMethod {
	// TODO: Move this to authnProvider.Authenticate
	case constants.TokenEndpointAuthMethodPrivateKeyJWT:
		if err := validateClientAssertion(oauthApp, jwtService, endpointURL, clientID,
			clientAssertion); err != nil {
			logger.Debug("Invalid client assertion: " + err.Error())
			return nil, errInvalidClientAssertion
		}
	case constants.TokenEndpointAuthMethodClientSecretBasic,
		constants.TokenEndpointAuthMethodClientSecretPost:
		_, _, authnErr := authnProvider.AuthenticateUser(ctx,
			map[string]interface{}{"clientId": clientID},
			map[string]interface{}{"clientSecret": clientSecret},
			nil, nil, authnprovidermgr.AuthUser{})
		if authnErr != nil {
			logger.Debug("Client secret authentication failed",
				log.MaskedString("clientID", clientID))
			return nil, errInvalidClientCredentials
		}
	}

	return &OAuthClientInfo{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		OAuthApp:     oauthApp,
	}, nil
}

// extractBasicAuthCredentials extracts the basic authentication credentials from the request header.
func extractBasicAuthCredentials(r *http.Request) (string, string, *authError) {
	authHeader := r.Header.Get(serverconst.AuthorizationHeaderName)
	if !utils.HasPrefixFold(authHeader, serverconst.AuthSchemeBasic) {
		return "", "", errInvalidAuthorizationHeader
	}

	encodedCredentials := utils.TrimPrefixFold(authHeader, serverconst.AuthSchemeBasic)
	decodedCredentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return "", "", errInvalidAuthorizationHeader
	}

	credentials := strings.SplitN(string(decodedCredentials), ":", 2)
	if len(credentials) != 2 {
		return "", "", errInvalidAuthorizationHeader
	}
	if credentials[0] == "" {
		return "", "", errMissingClientID
	}
	if credentials[1] == "" {
		return "", "", errInvalidAuthorizationHeader
	}

	// URL-decode client credentials.
	clientID, idErr := url.QueryUnescape(credentials[0])
	if idErr != nil {
		return "", "", errInvalidAuthorizationHeader
	}
	clientSecret, secretErr := url.QueryUnescape(credentials[1])
	if secretErr != nil {
		return "", "", errInvalidAuthorizationHeader
	}

	return clientID, clientSecret, nil
}

// extractClientIDFromAssertion extracts the client_id from the JWT assertion's 'sub' claim.
// This parses the JWT WITHOUT signature verification to extract the subject.
func extractClientIDFromAssertion(assertion string) (string, *authError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientAuthMiddleware"))

	payload, err := jwt.DecodeJWTPayload(assertion)
	if err != nil {
		logger.Debug("Invalid client assertion: failed to decode jwt")
		return "", errInvalidClientAssertion
	}

	subject, ok := payload["sub"].(string)

	if !ok || subject == "" {
		logger.Debug("Invalid client assertion: missing 'sub' claim or 'sub' claim is not a string")
		return "", errInvalidClientAssertion
	}

	return subject, nil
}

// validateClientAssertion validates the provided client assertion JWT using the configured certificate and JWT service.
// The endpointURL is used as the expected audience for JWT validation.
func validateClientAssertion(
	oauthApp *inboundmodel.OAuthClient,
	jwtService jwt.JWTServiceInterface,
	endpointURL string,
	clientID, clientAssertion string) error {
	if oauthApp.Certificate == nil {
		return fmt.Errorf("no certificate configured for client assertion validation")
	}

	if oauthApp.Certificate.Type == cert.CertificateTypeJWKSURI {
		if err := jwtService.VerifyJWTWithJWKS(clientAssertion, oauthApp.Certificate.Value, endpointURL,
			clientID); err != nil {
			return fmt.Errorf("client assertion verification with JWKS URI failed: %v", err.Error)
		}
		return nil
	}

	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal([]byte(oauthApp.Certificate.Value), &jwks); err != nil {
		return fmt.Errorf("invalid JWKS certificate format: %w", err)
	}

	var kid string
	if header, err := jwt.DecodeJWTHeader(clientAssertion); err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	} else if k, ok := header["kid"].(string); !ok || k == "" {
		return fmt.Errorf("JWT header missing 'kid' claim or 'kid' is not a string")
	} else {
		kid = k
	}

	var jwk map[string]any
	for _, key := range jwks.Keys {
		if keyID, ok := key["kid"].(string); ok && keyID == kid {
			jwk = key
			break
		}
	}
	if jwk == nil {
		return fmt.Errorf("no matching key found in JWKS for kid: %v", kid)
	}

	pubKey, err := jws.JWKToPublicKey(jwk)
	if err != nil {
		return fmt.Errorf("failed to convert JWK to public key: %w", err)
	}

	if err := jwtService.VerifyJWTWithPublicKey(clientAssertion, pubKey, endpointURL, clientID); err != nil {
		return fmt.Errorf("client assertion verification failed: %v", err.Error)
	}

	return nil
}
