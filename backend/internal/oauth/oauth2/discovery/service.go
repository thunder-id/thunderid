/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package discovery

import (
	"context"
	"errors"
	"slices"
	"sort"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// DiscoveryServiceInterface defines the interface for discovery services
type DiscoveryServiceInterface interface {
	GetOAuth2AuthorizationServerMetadata(ctx context.Context) *OAuth2AuthorizationServerMetadata
	GetOIDCMetadata(ctx context.Context) (*OIDCProviderMetadata, error)
}

// discoveryService implements DiscoveryServiceInterface
type discoveryService struct {
	cfg            oauthconfig.Config
	cryptoProvider kmprovider.RuntimeCryptoProvider
}

// newDiscoveryService creates a new discovery service instance
func newDiscoveryService(
	cryptoProvider kmprovider.RuntimeCryptoProvider, cfg oauthconfig.Config,
) DiscoveryServiceInterface {
	return &discoveryService{
		cfg:            cfg,
		cryptoProvider: cryptoProvider,
	}
}

// GetOAuth2AuthorizationServerMetadata returns OAuth 2.0 Authorization Server Metadata
func (ds *discoveryService) GetOAuth2AuthorizationServerMetadata(
	ctx context.Context,
) *OAuth2AuthorizationServerMetadata {
	metadata := &OAuth2AuthorizationServerMetadata{
		Issuer:                                     ds.getIssuer(),
		AuthorizationEndpoint:                      ds.getAuthorizationEndpoint(),
		TokenEndpoint:                              ds.getTokenEndpoint(),
		JWKSUri:                                    ds.getJWKSUri(),
		IntrospectionEndpoint:                      ds.getIntrospectionEndpoint(),
		PushedAuthorizationRequestEndpoint:         ds.getPAREndpoint(),
		RequirePushedAuthorizationRequests:         ds.isGlobalPARRequired(),
		ResponseTypesSupported:                     ds.getSupportedResponseTypes(),
		GrantTypesSupported:                        ds.getSupportedGrantTypes(),
		TokenEndpointAuthMethodsSupported:          ds.getSupportedTokenEndpointAuthMethods(),
		CodeChallengeMethodsSupported:              ds.getSupportedCodeChallengeMethods(),
		AuthorizationResponseIssParameterSupported: true,
		DPoPSigningAlgValuesSupported:              ds.getSupportedDPoPSigningAlgs(),
	}

	if slices.Contains(metadata.GrantTypesSupported, string(providers.GrantTypeCIBA)) {
		metadata.BackchannelAuthenticationEndpoint = ds.getBackchannelAuthenticationEndpoint()
		metadata.BackchannelTokenDeliveryModesSupported = []string{"poll"}
		metadata.BackchannelUserCodeParameterSupported = false
	}
	if ds.cfg.OAuth.TokenRevocation.Enabled {
		metadata.RevocationEndpoint = ds.getRevocationEndpoint()
	}
	if ds.cfg.OAuth.DCR.IsEnabled() {
		metadata.RegistrationEndpoint = ds.getRegistrationEndpoint()
	}
	return metadata
}

// GetOIDCMetadata returns OpenID Connect Provider Metadata
func (ds *discoveryService) GetOIDCMetadata(ctx context.Context) (*OIDCProviderMetadata, error) {
	oauth2Meta := ds.GetOAuth2AuthorizationServerMetadata(ctx)

	signingAlgs, err := ds.getSupportedSigningAlgorithms(ctx)
	if err != nil {
		return nil, err
	}
	oidcProviderMetadata := &OIDCProviderMetadata{
		OAuth2AuthorizationServerMetadata:    *oauth2Meta,
		UserInfoEndpoint:                     ds.getUserInfoEndpoint(),
		ScopesSupported:                      ds.getSupportedOIDCScopes(),
		SubjectTypesSupported:                ds.getSupportedSubjectTypes(),
		IDTokenSigningAlgValuesSupported:     signingAlgs,
		UserInfoSigningAlgValuesSupported:    signingAlgs,
		UserInfoEncryptionAlgValuesSupported: inboundmodel.SupportedUserInfoEncryptionAlgs,
		UserInfoEncryptionEncValuesSupported: inboundmodel.SupportedUserInfoEncryptionEncs,
		IDTokenEncryptionAlgValuesSupported:  inboundmodel.SupportedIDTokenEncryptionAlgs,
		IDTokenEncryptionEncValuesSupported:  inboundmodel.SupportedIDTokenEncryptionEncs,
		ClaimsSupported:                      ds.getSupportedClaims(),
		ClaimsParameterSupported:             true,
		AcrValuesSupported:                   ds.getSupportedAcrValues(),
	}

	if ds.cfg.OAuth.Logout.Enabled {
		oidcProviderMetadata.EndSessionEndpoint = ds.getEndSessionEndpoint()
	}

	return oidcProviderMetadata, nil
}

func (ds *discoveryService) getEndSessionEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2LogoutEndpoint
}

func (ds *discoveryService) getIssuer() string {
	return ds.cfg.JWT.Issuer
}

func (ds *discoveryService) getAuthorizationEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2AuthorizationEndpoint
}

func (ds *discoveryService) getTokenEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2TokenEndpoint
}

func (ds *discoveryService) getJWKSUri() string {
	return ds.cfg.BaseURL + constants.OAuth2JWKSEndpoint
}

func (ds *discoveryService) getIntrospectionEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2IntrospectionEndpoint
}

func (ds *discoveryService) getRevocationEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2RevokeEndpoint
}

func (ds *discoveryService) getUserInfoEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2UserInfoEndpoint
}

func (ds *discoveryService) getRegistrationEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2DCREndpoint
}

func (ds *discoveryService) getSupportedOIDCScopes() []string {
	scopes := make([]string, 0, len(constants.StandardOIDCScopes))
	for scope := range constants.StandardOIDCScopes {
		scopes = append(scopes, scope)
	}
	return scopes
}

func (ds *discoveryService) getSupportedResponseTypes() []string {
	return constants.GetSupportedResponseTypes(ds.cfg)
}

func (ds *discoveryService) getSupportedGrantTypes() []string {
	return constants.GetSupportedGrantTypes(ds.cfg)
}

func (ds *discoveryService) getSupportedTokenEndpointAuthMethods() []string {
	return constants.GetSupportedTokenEndpointAuthMethods(ds.cfg)
}

func (ds *discoveryService) getSupportedCodeChallengeMethods() []string {
	return pkce.GetSupportedCodeChallengeMethods()
}

func (ds *discoveryService) getPAREndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2PAREndpoint
}

func (ds *discoveryService) getBackchannelAuthenticationEndpoint() string {
	return ds.cfg.BaseURL + constants.OAuth2BackchannelAuthEndpoint
}

func (ds *discoveryService) isGlobalPARRequired() bool {
	return ds.cfg.OAuth.PAR.RequirePAR
}

func (ds *discoveryService) getSupportedDPoPSigningAlgs() []string {
	algs := ds.cfg.OAuth.DPoP.AllowedAlgs
	if len(algs) == 0 {
		return nil
	}
	out := make([]string, len(algs))
	copy(out, algs)
	return out
}

func (ds *discoveryService) getSupportedSubjectTypes() []string {
	return constants.GetSupportedSubjectTypes()
}

func (ds *discoveryService) getSupportedSigningAlgorithms(ctx context.Context) ([]string, error) {
	keys, err := ds.cryptoProvider.GetPublicKeys(ctx, kmprovider.PublicKeyFilter{})
	if err != nil {
		log.GetLogger().Error(ctx,
			"Failed to retrieve public keys for signing algorithm discovery", log.Error(err))
		return nil, err
	}
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		alg := string(k.Algorithm)
		if alg == "" || slices.Contains(result, alg) {
			continue
		}
		result = append(result, alg)
	}
	if len(result) == 0 {
		err = errors.New("no valid signing algorithms found")
		log.GetLogger().Error(ctx,
			"No valid signing algorithms found in registered public keys", log.Error(err))
		return nil, err
	}
	return result, nil
}

func (ds *discoveryService) getSupportedAcrValues() []string {
	acrAMR := ds.cfg.OAuth.AuthClass.AcrAMR
	acrs := make([]string, 0, len(acrAMR))
	for acr := range acrAMR {
		acrs = append(acrs, acr)
	}
	sort.Strings(acrs)
	return acrs
}

func (ds *discoveryService) getSupportedClaims() []string {
	// Extract claims from OIDC scopes
	var claims []string
	claims = append(claims, constants.GetStandardClaims()...)

	for _, scope := range constants.StandardOIDCScopes {
		claims = append(claims, scope.Claims...)
	}

	// Remove duplicates
	claimMap := make(map[string]bool)
	var uniqueClaims []string
	for _, claim := range claims {
		if !claimMap[claim] {
			claimMap[claim] = true
			uniqueClaims = append(uniqueClaims, claim)
		}
	}

	return uniqueClaims
}
