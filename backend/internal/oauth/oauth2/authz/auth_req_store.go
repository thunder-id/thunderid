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

package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// authRequestContext holds OAuth authorization request information.
type authRequestContext struct {
	OAuthParameters model.OAuthParameters
}

// authorizationRequestStoreInterface defines the interface for authorization request storage.
type authorizationRequestStoreInterface interface {
	AddRequest(ctx context.Context, value authRequestContext) (string, error)
	GetRequest(ctx context.Context, key string) (bool, authRequestContext, error)
	ClearRequest(ctx context.Context, key string) error
}

// authorizationRequestStore provides the authorization request store functionality using database.
type authorizationRequestStore struct {
	dbProvider     provider.DBProviderInterface
	validityPeriod time.Duration
	deploymentID   string
}

// newAuthorizationRequestStore creates a new instance of authorizationRequestStore with injected dependencies.
func newAuthorizationRequestStore() authorizationRequestStoreInterface {
	return &authorizationRequestStore{
		dbProvider:     provider.GetDBProvider(),
		validityPeriod: 10 * time.Minute,
		deploymentID:   config.GetServerRuntime().Config.Server.Identifier,
	}
}

// AddRequest adds an authorization request context entry to the store.
func (authzRS *authorizationRequestStore) AddRequest(ctx context.Context, value authRequestContext) (string, error) {
	dbClient, err := authzRS.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return "", fmt.Errorf("failed to get database client: %w", err)
	}

	key, err := utils.GenerateUUIDv7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	// Calculate expiry based on current time
	requestInitiatedTime := time.Now()
	expiryTime := requestInitiatedTime.Add(authzRS.validityPeriod)

	// Serialize authRequestContext to JSON
	jsonDataBytes, err := authzRS.getJSONDataBytes(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request context to JSON: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryInsertAuthRequest, key, jsonDataBytes, expiryTime, authzRS.deploymentID)
	if err != nil {
		return "", fmt.Errorf("failed to insert authorization request: %w", err)
	}

	return key, nil
}

// GetRequest retrieves an authorization request context entry from the store.
func (authzRS *authorizationRequestStore) GetRequest(
	ctx context.Context, key string) (bool, authRequestContext, error) {
	if key == "" {
		return false, authRequestContext{}, nil
	}

	dbClient, err := authzRS.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return false, authRequestContext{}, fmt.Errorf("failed to get database client: %w", err)
	}

	// Check expiry by comparing with current time
	now := time.Now()
	results, err := dbClient.QueryContext(ctx, queryGetAuthRequest, key, now, authzRS.deploymentID)
	if err != nil {
		return false, authRequestContext{}, fmt.Errorf("failed to query authorization request: %w", err)
	}

	if len(results) == 0 {
		return false, authRequestContext{}, nil
	}

	row := results[0]
	authRequestCtx, err := authzRS.buildAuthRequestContextFromResultRow(row)
	if err != nil {
		return false, authRequestContext{}, fmt.Errorf("failed to build authorization request context: %w", err)
	}

	return true, authRequestCtx, nil
}

// ClearRequest removes a specific authorization request context entry from the store.
func (authzRS *authorizationRequestStore) ClearRequest(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}

	dbClient, err := authzRS.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteAuthRequest, key, authzRS.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete authorization request: %w", err)
	}

	return nil
}

// getJSONDataBytes prepares the JSON data bytes for the authorization request context.
func (authzRS *authorizationRequestStore) getJSONDataBytes(authRequestCtx authRequestContext) ([]byte, error) {
	jsonData := map[string]interface{}{
		jsonKeyState:               authRequestCtx.OAuthParameters.State,
		jsonKeyClientID:            authRequestCtx.OAuthParameters.ClientID,
		jsonKeyRedirectURI:         authRequestCtx.OAuthParameters.RedirectURI,
		jsonKeyResponseType:        authRequestCtx.OAuthParameters.ResponseType,
		jsonKeyStandardScopes:      authRequestCtx.OAuthParameters.StandardScopes,
		jsonKeyPermissionScopes:    authRequestCtx.OAuthParameters.PermissionScopes,
		jsonKeyCodeChallenge:       authRequestCtx.OAuthParameters.CodeChallenge,
		jsonKeyCodeChallengeMethod: authRequestCtx.OAuthParameters.CodeChallengeMethod,
		jsonKeyResource:            authRequestCtx.OAuthParameters.Resources,
		jsonKeyClaimsLocales:       authRequestCtx.OAuthParameters.ClaimsLocales,
		jsonKeyNonce:               authRequestCtx.OAuthParameters.Nonce,
	}

	// Add claims_request if present
	if authRequestCtx.OAuthParameters.ClaimsRequest != nil {
		jsonData[jsonKeyClaimsRequest] = authRequestCtx.OAuthParameters.ClaimsRequest
	}

	jsonDataBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request context to JSON: %w", err)
	}
	return jsonDataBytes, nil
}

// buildAuthRequestContextFromResultRow builds an authRequestContext from a database result row.
func (authzRS *authorizationRequestStore) buildAuthRequestContextFromResultRow(
	row map[string]interface{},
) (authRequestContext, error) {
	var dataJSON string
	if val, ok := row[dbColumnRequestData].(string); ok && val != "" {
		dataJSON = val
	} else if val, ok := row[dbColumnRequestData].([]byte); ok && len(val) > 0 {
		dataJSON = string(val)
	} else {
		return authRequestContext{}, fmt.Errorf("%s is missing or of unexpected type", dbColumnRequestData)
	}

	var requestDataMap map[string]interface{}
	if err := json.Unmarshal([]byte(dataJSON), &requestDataMap); err != nil {
		return authRequestContext{}, fmt.Errorf("failed to unmarshal %s JSON: %w", dbColumnRequestData, err)
	}

	// Build OAuthParameters from JSON
	oauthParams := model.OAuthParameters{}
	oauthParams.StandardScopes = []string{}
	oauthParams.PermissionScopes = []string{}

	if state, ok := requestDataMap[jsonKeyState].(string); ok {
		oauthParams.State = state
	}
	if clientID, ok := requestDataMap[jsonKeyClientID].(string); ok {
		oauthParams.ClientID = clientID
	}
	if redirectURI, ok := requestDataMap[jsonKeyRedirectURI].(string); ok {
		oauthParams.RedirectURI = redirectURI
	}
	if responseType, ok := requestDataMap[jsonKeyResponseType].(string); ok {
		oauthParams.ResponseType = responseType
	}
	// Handle standard_scopes
	if standardScopes, ok := requestDataMap[jsonKeyStandardScopes].([]interface{}); ok {
		oauthParams.StandardScopes = convertToStringArray(standardScopes)
	} else if standardScopes, ok := requestDataMap[jsonKeyStandardScopes].([]string); ok {
		oauthParams.StandardScopes = standardScopes
	} else if requestDataMap[jsonKeyStandardScopes] == nil {
		oauthParams.StandardScopes = []string{}
	}
	// Handle permission_scopes
	if permissionScopes, ok := requestDataMap[jsonKeyPermissionScopes].([]interface{}); ok {
		oauthParams.PermissionScopes = convertToStringArray(permissionScopes)
	} else if permissionScopes, ok := requestDataMap[jsonKeyPermissionScopes].([]string); ok {
		oauthParams.PermissionScopes = permissionScopes
	} else if requestDataMap[jsonKeyPermissionScopes] == nil {
		oauthParams.PermissionScopes = []string{}
	}
	if codeChallenge, ok := requestDataMap[jsonKeyCodeChallenge].(string); ok {
		oauthParams.CodeChallenge = codeChallenge
	}
	if codeChallengeMethod, ok := requestDataMap[jsonKeyCodeChallengeMethod].(string); ok {
		oauthParams.CodeChallengeMethod = codeChallengeMethod
	}
	if rawResources, ok := requestDataMap[jsonKeyResource].([]interface{}); ok {
		oauthParams.Resources = convertToStringArray(rawResources)
	} else if resources, ok := requestDataMap[jsonKeyResource].([]string); ok {
		oauthParams.Resources = resources
	}
	if claimsLocales, ok := requestDataMap[jsonKeyClaimsLocales].(string); ok {
		oauthParams.ClaimsLocales = claimsLocales
	}
	// Nonce is OIDC-specific and should only be set when openid scope is present
	if slices.Contains(oauthParams.StandardScopes, constants.ScopeOpenID) {
		if nonce, ok := requestDataMap[jsonKeyNonce].(string); ok {
			oauthParams.Nonce = nonce
		}
	}

	// Parse claims_request if present
	if claimsData, ok := requestDataMap[jsonKeyClaimsRequest]; ok && claimsData != nil {
		claimsRequest, err := parseClaimsRequestFromJSON(claimsData)
		if err != nil {
			return authRequestContext{}, fmt.Errorf(
				"failed to parse claims_request from authorization request: %w", err)
		}
		oauthParams.ClaimsRequest = claimsRequest
	}

	return authRequestContext{
		OAuthParameters: oauthParams,
	}, nil
}

// convertToStringArray converts []interface{} to []string.
func convertToStringArray(arr []interface{}) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if str, ok := v.(string); ok {
			result = append(result, str)
		}
	}
	return result
}
