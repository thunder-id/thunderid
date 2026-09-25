// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"time"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// jtiNamespace identifies private_key_jwt client assertions in the shared JTI replay store.
const jtiNamespace = "client_assertion"

// authenticate authenticates the OAuth2 client from the request.
// It extracts credentials, validates them, and returns OAuthClientInfo on success.
// The issuer is the audience value accepted when validating client assertion JWTs.
// Returns an authError on failure.
func authenticate(
	ctx context.Context,
	r *http.Request,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	jtiStore jti.JTIStoreInterface,
	issuer string,
	leeway int64,
) (*OAuthClientInfo, *authError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientAuthMiddleware"))

	// Extract all possible auth fields
	hasAuthHeader := r.Header.Get(serverconst.AuthorizationHeaderName) != ""
	clientIDFromBody := r.FormValue(constants.RequestParamClientID)
	clientSecretFromBody := r.FormValue(constants.RequestParamClientSecret)
	clientAssertionType := r.FormValue(constants.RequestParamClientAssertionType)
	clientAssertion := r.FormValue(constants.RequestParamClientAssertion)

	var detectedMethod providers.TokenEndpointAuthMethod

	// Method 1: Basic Auth (header)
	if hasAuthHeader {
		detectedMethod = providers.TokenEndpointAuthMethodClientSecretBasic
	}

	// Method 2: Client credentials in body
	if clientSecretFromBody != "" {
		if detectedMethod != "" {
			return nil, errMultipleAuthMethods
		}
		detectedMethod = providers.TokenEndpointAuthMethodClientSecretPost
	}

	// Method 3: Client assertion (private_key_jwt)
	if clientAssertionType != "" || clientAssertion != "" {
		if detectedMethod != "" {
			return nil, errMultipleAuthMethods
		}
		detectedMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
	}

	// If no auth method but client_id exists -> public client
	if detectedMethod == "" && clientIDFromBody != "" {
		detectedMethod = providers.TokenEndpointAuthMethodNone
	}

	// Now process based on detected method
	var clientID string
	var clientSecret string

	switch detectedMethod {
	case providers.TokenEndpointAuthMethodClientSecretBasic:
		var err *authError
		clientID, clientSecret, err = extractBasicAuthCredentials(r)
		if err != nil {
			return nil, err
		}

	case providers.TokenEndpointAuthMethodClientSecretPost:
		if clientIDFromBody == "" {
			return nil, errMissingClientID
		}
		clientID = clientIDFromBody
		clientSecret = clientSecretFromBody

	case providers.TokenEndpointAuthMethodPrivateKeyJWT:
		if clientAssertionType != constants.SupportedClientAssertionType {
			logger.Debug(ctx, "Invalid client assertion: unsupported client assertion type")
			return nil, errInvalidClientAssertion
		}
		extracted, err := extractClientIDFromAssertion(ctx, clientAssertion)
		if err != nil {
			return nil, err
		}
		clientID = extracted

	case providers.TokenEndpointAuthMethodNone:
		clientID = clientIDFromBody

	default:
		return nil, errMissingClientID
	}

	if clientIDFromBody != "" && clientID != clientIDFromBody {
		return nil, errClientIDMismatch
	}

	oauthApp, svcErr := actorProvider.GetOAuthClientByClientID(ctx, clientID)
	if svcErr != nil {
		// Resolution is scoped by the organization unit the request named, so a client that exists
		// but may not act for it does not resolve. That is a different answer from a bad credential
		// and has to read as one.
		if svcErr.Code == tidcommon.ErrorUnauthorized.Code {
			accessingOUID := syscontext.GetAccessingOUID(ctx)
			logger.Debug(ctx, "Client is not authorized for the requested organization unit",
				log.MaskedString("clientID", clientID), log.String("ouId", accessingOUID))
			return nil, errClientNotAuthorizedForOU(accessingOUID)
		}
		logger.Error(ctx, "Failed to retrieve OAuth client",
			log.String("error", svcErr.Error.DefaultValue), log.MaskedString("clientID", clientID))
		return nil, errInvalidClientCredentials
	}
	if oauthApp == nil {
		return nil, errInvalidClientCredentials
	}

	if oauthApp.TokenEndpointAuthMethod != detectedMethod {
		// No credentials presented for a client that requires authentication.
		if detectedMethod == providers.TokenEndpointAuthMethodNone {
			return nil, errClientAuthRequired
		}
		return nil, errUnauthorizedAuthMethod
	}

	// Validate credentials based on method
	switch detectedMethod {
	// TODO: Move this to authnProvider.Authenticate
	case providers.TokenEndpointAuthMethodPrivateKeyJWT:
		if err := validateClientAssertion(ctx, oauthApp, jwtService, jtiStore, issuer, clientID,
			clientAssertion, leeway); err != nil {
			logger.Debug(ctx, "Invalid client assertion: "+err.Error())
			return nil, errInvalidClientAssertion
		}
	case providers.TokenEndpointAuthMethodClientSecretBasic,
		providers.TokenEndpointAuthMethodClientSecretPost:
		_, _, authnErr := authnProvider.AuthenticateUser(ctx,
			map[string]interface{}{"clientId": clientID},
			map[string]interface{}{authnprovidercm.CredentialTypeClientSecret: clientSecret},
			nil, nil, providers.AuthUser{})
		if authnErr != nil {
			logger.Debug(ctx, "Client secret authentication failed",
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
func extractClientIDFromAssertion(ctx context.Context, assertion string) (string, *authError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientAuthMiddleware"))

	payload, err := jwt.DecodeJWTPayload(assertion)
	if err != nil {
		logger.Debug(ctx, "Invalid client assertion: failed to decode jwt")
		return "", errInvalidClientAssertion
	}

	subject, ok := payload["sub"].(string)

	if !ok || subject == "" {
		logger.Debug(ctx, "Invalid client assertion: missing 'sub' claim or 'sub' claim is not a string")
		return "", errInvalidClientAssertion
	}

	return subject, nil
}

// validateClientAssertion validates the provided client assertion JWT using the configured certificate and JWT service.
// Per FAPI 2.0 Security Profile Section 5.3.2.1, the assertion's 'aud' claim must be the authorization server's
// issuer identifier.
func validateClientAssertion(ctx context.Context,
	oauthApp *providers.OAuthClient,
	jwtService jwt.JWTServiceInterface,
	jtiStore jti.JTIStoreInterface,
	issuer string,
	clientID, clientAssertion string,
	leeway int64) error {
	if oauthApp.Certificate == nil {
		return fmt.Errorf("no certificate configured for client assertion validation")
	}

	// FAPI 2.0 Security Profile Section 5.3.2.1: the client assertion's 'aud' claim must be the
	// authorization server's issuer identifier as a single string.
	payload, err := jwt.DecodeJWTPayload(clientAssertion)
	if err != nil {
		return fmt.Errorf("failed to decode client assertion payload: %w", err)
	}
	aud, ok := payload[constants.ClaimAud].(string)
	if !ok {
		return fmt.Errorf("client assertion 'aud' claim must be a single string")
	}
	if aud != issuer {
		return fmt.Errorf("client assertion 'aud' claim %q does not match the issuer", aud)
	}

	if err := verifyAssertionSignature(ctx, oauthApp, jwtService, issuer, clientID, clientAssertion); err != nil {
		return err
	}

	// Replay protection: record the assertion's jti so it cannot be reused within its validity window.
	return recordAssertionJTI(ctx, jtiStore, payload, leeway)
}

// verifyAssertionSignature verifies the client assertion's signature against the client's configured
// certificate, resolving the verification key from either a JWKS URI or an inline JWKS.
func verifyAssertionSignature(ctx context.Context,
	oauthApp *providers.OAuthClient,
	jwtService jwt.JWTServiceInterface,
	issuer, clientID, clientAssertion string) error {
	if oauthApp.Certificate.Type == cert.CertificateTypeJWKSURI {
		if err := jwtService.VerifyJWTWithJWKS(ctx, clientAssertion, oauthApp.Certificate.Value, issuer,
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

	if err := jwtService.VerifyJWTWithPublicKey(ctx, clientAssertion, providers.KeyRef{PublicKeyJWK: jwk},
		issuer, clientID); err != nil {
		return fmt.Errorf("client assertion verification failed: %v", err.Error)
	}

	return nil
}

// recordAssertionJTI enforces one-time use of a verified client assertion by recording its jti in
// the shared replay store.
func recordAssertionJTI(ctx context.Context, jtiStore jti.JTIStoreInterface,
	payload map[string]interface{}, leeway int64) error {
	jtiValue, ok := payload[constants.ClaimJTI].(string)
	if !ok || jtiValue == "" {
		return fmt.Errorf("client assertion missing 'jti' claim or 'jti' is not a string")
	}
	exp, ok := payload[constants.ClaimExp].(float64)
	if !ok {
		return fmt.Errorf("client assertion missing 'exp' claim or 'exp' is not a number")
	}

	expiry := time.Unix(int64(exp)+leeway, 0)
	inserted, err := jtiStore.RecordJTI(ctx, jtiNamespace, jtiValue, expiry)
	if err != nil {
		return fmt.Errorf("failed to record client assertion jti: %w", err)
	}
	if !inserted {
		return fmt.Errorf("client assertion replay detected")
	}

	return nil
}
