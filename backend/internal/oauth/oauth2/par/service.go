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

package par

import (
	"context"
	"strings"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz/requestvalidator"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// requestURIPrefix is the URN prefix used for PAR request URIs per RFC 9126.
const requestURIPrefix = "urn:ietf:params:oauth:request_uri:"

// PARServiceInterface defines the interface for the PAR service.
type PARServiceInterface interface {
	HandlePushedAuthorizationRequest(
		ctx context.Context, params map[string]string, resources []string,
		oauthApp *inboundmodel.OAuthClient,
	) (*parResponse, string, string)
	ResolvePushedAuthorizationRequest(
		ctx context.Context, requestURI string, clientID string,
	) (*oauth2model.OAuthParameters, error)
}

// parService implements PARServiceInterface.
type parService struct {
	store           parStoreInterface
	resourceService resource.ResourceServiceInterface
	logger          *log.Logger
}

// newPARService creates a new PAR service instance.
func newPARService(
	store parStoreInterface, resourceService resource.ResourceServiceInterface,
) PARServiceInterface {
	return &parService{
		store:           store,
		resourceService: resourceService,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PARService")),
	}
}

// HandlePushedAuthorizationRequest validates and stores a pushed authorization request.
// Returns the response on success, or (errorCode, errorDescription) on failure.
func (s *parService) HandlePushedAuthorizationRequest(
	ctx context.Context, params map[string]string, resources []string,
	oauthApp *inboundmodel.OAuthClient,
) (*parResponse, string, string) {
	// The request MUST NOT contain a request_uri parameter.
	if params[oauth2const.RequestParamRequestURI] != "" {
		return nil, oauth2const.ErrorInvalidRequest,
			"request_uri parameter must not be included in a pushed authorization request"
	}

	// Validate the redirect URI.
	redirectURI := params[oauth2const.RequestParamRedirectURI]
	if err := oauthApp.ValidateRedirectURI(redirectURI); err != nil {
		return nil, oauth2const.ErrorInvalidRequest, "Invalid redirect URI"
	}

	// Validate the authorization parameters using the same rules as the authorize endpoint.
	errCode, errMsg := requestvalidator.ValidateAuthorizationRequestParams(params, oauthApp)
	if errCode != "" {
		return nil, errCode, errMsg
	}

	if errResp := resourceindicators.ValidateResourceURIs(resources); errResp != nil {
		return nil, errResp.Error, errResp.ErrorDescription
	}

	// Parse the claims parameter if present.
	var claimsRequest *oauth2model.ClaimsRequest
	claimsParam := params[oauth2const.RequestParamClaims]
	if claimsParam != "" {
		var err error
		claimsRequest, err = oauth2utils.ParseClaimsRequest(claimsParam)
		if err != nil {
			return nil, oauth2const.ErrorInvalidRequest,
				"The claims request parameter is malformed or contains invalid values"
		}
	}

	scope := params[oauth2const.RequestParamScope]
	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(scope, oauthApp.ScopeClaims)

	// Resolve resource identifiers to Resource Servers and downscope non-OIDC scopes against
	// the union of permissions defined on those Resource Servers. Unknown identifiers cause
	// invalid_target; scopes not defined on any RS are silently dropped.
	_, nonOidcScopes, errResp := resourceindicators.ResolveAndDownscope(
		ctx, s.resourceService, resources, nonOidcScopes)
	if errResp != nil {
		return nil, errResp.Error, errResp.ErrorDescription
	}

	if redirectURI == "" && len(oauthApp.RedirectURIs) == 1 {
		redirectURI = oauthApp.RedirectURIs[0]
	}

	oauthParams := oauth2model.OAuthParameters{
		State:               params[oauth2const.RequestParamState],
		ClientID:            oauthApp.ClientID,
		RedirectURI:         redirectURI,
		ResponseType:        params[oauth2const.RequestParamResponseType],
		StandardScopes:      oidcScopes,
		PermissionScopes:    nonOidcScopes,
		CodeChallenge:       params[oauth2const.RequestParamCodeChallenge],
		CodeChallengeMethod: params[oauth2const.RequestParamCodeChallengeMethod],
		Resources:           resources,
		ClaimsRequest:       claimsRequest,
		ClaimsLocales:       params[oauth2const.RequestParamClaimsLocales],
		Nonce:               params[oauth2const.RequestParamNonce],
		AcrValues:           params[oauth2const.RequestParamAcrValues],
	}

	parRequest := pushedAuthorizationRequest{
		ClientID:        oauthApp.ClientID,
		OAuthParameters: oauthParams,
	}

	expiresIn := config.GetServerRuntime().Config.OAuth.PAR.ExpiresIn

	randomKey, err := s.store.Store(ctx, parRequest, expiresIn)
	if err != nil {
		s.logger.Error("Failed to store pushed authorization request", log.Error(err))
		return nil, oauth2const.ErrorServerError, "Failed to process pushed authorization request"
	}

	return &parResponse{
		RequestURI: requestURIPrefix + randomKey,
		ExpiresIn:  expiresIn,
	}, "", ""
}

// ResolvePushedAuthorizationRequest retrieves and consumes a stored PAR request.
// Returns the stored OAuth parameters on success, or an error if the request_uri is invalid.
func (s *parService) ResolvePushedAuthorizationRequest(
	ctx context.Context, requestURI string, clientID string,
) (*oauth2model.OAuthParameters, error) {
	if !strings.HasPrefix(requestURI, requestURIPrefix) {
		return nil, errInvalidRequestURI
	}
	randomKey := strings.TrimPrefix(requestURI, requestURIPrefix)

	parRequest, found, err := s.store.Consume(ctx, randomKey)
	if err != nil {
		s.logger.Error("Failed to consume PAR request", log.Error(err))
		return nil, ErrPARResolutionFailed
	}
	if !found {
		return nil, errRequestURINotFound
	}

	// Verify client_id binding: the client making the authorization request must match
	// the client that pushed the authorization request.
	if parRequest.ClientID != clientID {
		s.logger.Debug("Client ID mismatch for PAR request",
			log.String("expected", parRequest.ClientID),
			log.String("actual", clientID))
		return nil, errClientIDMismatch
	}

	return &parRequest.OAuthParameters, nil
}
