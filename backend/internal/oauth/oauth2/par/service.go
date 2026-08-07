// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package par

import (
	"context"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz/requestvalidator"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// requestURIPrefix is the URN prefix used for PAR request URIs per RFC 9126.
const requestURIPrefix = "urn:ietf:params:oauth:request_uri:"

// sensitiveParParams is the deny-list of PAR body parameters that must not be persisted into
// InitiatorRequest.QueryParams, since they carry client credentials and the PAR store is a
// plaintext runtime cache.
var sensitiveParParams = map[string]bool{
	oauth2const.RequestParamClientSecret:        true,
	oauth2const.RequestParamClientAssertion:     true,
	oauth2const.RequestParamClientAssertionType: true,
}

// PARServiceInterface defines the interface for the PAR service.
type PARServiceInterface interface {
	HandlePushedAuthorizationRequest(
		ctx context.Context, params map[string]string, resources []string,
		oauthApp *providers.OAuthClient, dpopHeaderJkt string,
	) (*parResponse, string, string)
	ResolvePushedAuthorizationRequest(
		ctx context.Context, requestURI string, clientID string,
	) (*oauth2model.OAuthParameters, *providers.InitiatorRequest, error)
}

// parService implements PARServiceInterface.
type parService struct {
	store           parStoreInterface
	resourceService providers.ResourceServerProvider
	cfg             oauthconfig.Config
	logger          *log.Logger
}

// newPARService creates a new PAR service instance.
func newPARService(
	store parStoreInterface, resourceService providers.ResourceServerProvider,
	cfg oauthconfig.Config,
) PARServiceInterface {
	return &parService{
		store:           store,
		resourceService: resourceService,
		cfg:             cfg,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PARService")),
	}
}

// HandlePushedAuthorizationRequest validates and stores a pushed authorization request.
// Returns the response on success, or (errorCode, errorDescription) on failure.
func (s *parService) HandlePushedAuthorizationRequest(
	ctx context.Context, params map[string]string, resources []string,
	oauthApp *providers.OAuthClient, dpopHeaderJkt string,
) (*parResponse, string, string) {
	// The request MUST NOT contain a request_uri parameter.
	if params[oauth2const.RequestParamRequestURI] != "" {
		return nil, oauth2const.ErrorInvalidRequest,
			"request_uri parameter must not be included in a pushed authorization request"
	}

	// Validate the redirect URI.
	redirectURI := params[oauth2const.RequestParamRedirectURI]
	if err := oauthApp.ValidateRedirectURI(ctx, redirectURI); err != nil {
		return nil, oauth2const.ErrorInvalidRequest, "Invalid redirect URI"
	}

	// Validate the authorization parameters using the same rules as the authorize endpoint.
	parParams := make(url.Values, len(params))
	for k, v := range params {
		parParams.Set(k, v)
	}
	errCode, errMsg := requestvalidator.ValidateAuthorizationRequestParams(parParams, oauthApp, dpopHeaderJkt)
	if errCode != "" {
		return nil, errCode, errMsg
	}

	if errResp := resourceindicators.ValidateResourceURIs(resources); errResp != nil {
		return nil, errResp.Error, errResp.ErrorDescription
	}
	if len(resources) > 1 {
		return nil, oauth2const.ErrorInvalidTarget, "Only a single resource parameter is supported"
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
	oidcScopes = oauth2utils.FilterOIDCScopesByAllowedScopes(oidcScopes, oauthApp.Scopes)

	// Validate up front that the request can bind to a resource server: an explicit resource must
	// resolve, or (with no resource) either the request is OIDC-only or a default resource server is
	// configured; otherwise reject with invalid_target. This mirrors the redirect-URI validation done
	// here at push time. The authoritative binding and per-resource-server downscoping still happen
	// when the pushed request is redeemed at the authorization endpoint, so both standard and
	// PAR-based requests bind identically.
	if _, errResp := resourceindicators.ResolveAudienceBinding(
		ctx, s.resourceService, resources, nonOidcScopes); errResp != nil {
		return nil, errResp.Error, errResp.ErrorDescription
	}

	redirectURIProvided := redirectURI != ""
	if redirectURI == "" && len(oauthApp.RedirectURIs) == 1 {
		redirectURI = oauthApp.RedirectURIs[0]
	}

	oauthParams := oauth2model.OAuthParameters{
		State:               params[oauth2const.RequestParamState],
		ClientID:            oauthApp.ClientID,
		RedirectURI:         redirectURI,
		RedirectURIProvided: redirectURIProvided,
		ResponseType:        params[oauth2const.RequestParamResponseType],
		StandardScopes:      oidcScopes,
		PermissionScopes:    nonOidcScopes,
		CodeChallenge:       params[oauth2const.RequestParamCodeChallenge],
		CodeChallengeMethod: params[oauth2const.RequestParamCodeChallengeMethod],
		Resources:           resources,
		ClaimsRequest:       claimsRequest,
		ClaimsLocales:       params[oauth2const.RequestParamClaimsLocales],
		UILocales:           params[oauth2const.RequestParamUILocales],
		Nonce:               params[oauth2const.RequestParamNonce],
		AcrValues:           params[oauth2const.RequestParamAcrValues],
		DPoPJkt:             resolveDPoPJkt(params[oauth2const.RequestParamDPoPJkt], dpopHeaderJkt),
		Prompt:              params[oauth2const.RequestParamPrompt],
	}

	initiatorQueryParams := make(map[string][]string, len(params)+1)
	for k, v := range params {
		if sensitiveParParams[k] {
			continue
		}
		initiatorQueryParams[k] = []string{v}
	}
	if len(resources) > 0 {
		initiatorQueryParams[oauth2const.RequestParamResource] = resources
	}

	parRequest := pushedAuthorizationRequest{
		ClientID:        oauthApp.ClientID,
		OAuthParameters: oauthParams,
		InitiatorRequest: providers.InitiatorRequest{
			QueryParams: initiatorQueryParams,
		},
	}

	expiresIn := s.cfg.OAuth.PAR.ExpiresIn

	randomKey, err := s.store.Store(ctx, parRequest, expiresIn)
	if err != nil {
		s.logger.Error(ctx, "Failed to store pushed authorization request", log.Error(err))
		return nil, oauth2const.ErrorServerError, "Failed to process pushed authorization request"
	}

	return &parResponse{
		RequestURI: requestURIPrefix + randomKey,
		ExpiresIn:  expiresIn,
	}, "", ""
}

// resolveDPoPJkt picks the effective DPoP key thumbprint. The proof-derived thumbprint
// takes precedence; the validator has already enforced equality when both are present.
func resolveDPoPJkt(paramJkt, headerJkt string) string {
	if headerJkt != "" {
		return headerJkt
	}
	return paramJkt
}

// ResolvePushedAuthorizationRequest retrieves and consumes a stored PAR request.
// Returns the stored OAuth parameters on success, or an error if the request_uri is invalid.
func (s *parService) ResolvePushedAuthorizationRequest(
	ctx context.Context, requestURI string, clientID string,
) (*oauth2model.OAuthParameters, *providers.InitiatorRequest, error) {
	if !strings.HasPrefix(requestURI, requestURIPrefix) {
		return nil, nil, errInvalidRequestURI
	}
	randomKey := strings.TrimPrefix(requestURI, requestURIPrefix)

	parRequest, found, err := s.store.Consume(ctx, randomKey)
	if err != nil {
		s.logger.Error(ctx, "Failed to consume PAR request", log.Error(err))
		return nil, nil, ErrPARResolutionFailed
	}
	if !found {
		return nil, nil, errRequestURINotFound
	}

	// Verify client_id binding: the client making the authorization request must match
	// the client that pushed the authorization request.
	if parRequest.ClientID != clientID {
		s.logger.Debug(ctx, "Client ID mismatch for PAR request",
			log.String("expected", parRequest.ClientID),
			log.String("actual", clientID))
		return nil, nil, errClientIDMismatch
	}

	return &parRequest.OAuthParameters, &parRequest.InitiatorRequest, nil
}
