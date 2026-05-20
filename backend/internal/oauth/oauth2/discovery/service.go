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

package discovery

import (
	"context"
	"errors"
	"slices"
	"sort"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// DiscoveryServiceInterface defines the interface for discovery services
type DiscoveryServiceInterface interface {
	GetOAuth2AuthorizationServerMetadata(ctx context.Context) *OAuth2AuthorizationServerMetadata
	GetOIDCMetadata(ctx context.Context) (*OIDCProviderMetadata, error)
}

// discoveryService implements DiscoveryServiceInterface
type discoveryService struct {
	baseURL        string
	cryptoProvider kmprovider.RuntimeCryptoProvider
}

// newDiscoveryService creates a new discovery service instance
func newDiscoveryService(cryptoProvider kmprovider.RuntimeCryptoProvider) DiscoveryServiceInterface {
	runtime := config.GetServerRuntime()
	ds := &discoveryService{cryptoProvider: cryptoProvider}
	ds.baseURL = config.GetServerURL(&runtime.Config.Server)
	return ds
}

// GetOAuth2AuthorizationServerMetadata returns OAuth 2.0 Authorization Server Metadata
func (ds *discoveryService) GetOAuth2AuthorizationServerMetadata(
	ctx context.Context,
) *OAuth2AuthorizationServerMetadata {
	metadata := &OAuth2AuthorizationServerMetadata{
		Issuer:                                     ds.getIssuer(),
		AuthorizationEndpoint:                      ds.getAuthorizationEndpoint(),
		TokenEndpoint:                              ds.getTokenEndpoint(),
		UserInfoEndpoint:                           ds.getUserInfoEndpoint(),
		JWKSUri:                                    ds.getJWKSUri(),
		RegistrationEndpoint:                       ds.getRegistrationEndpoint(),
		IntrospectionEndpoint:                      ds.getIntrospectionEndpoint(),
		PushedAuthorizationRequestEndpoint:         ds.getPAREndpoint(),
		RequirePushedAuthorizationRequests:         ds.isGlobalPARRequired(),
		ScopesSupported:                            ds.getSupportedScopes(),
		ResponseTypesSupported:                     ds.getSupportedResponseTypes(),
		GrantTypesSupported:                        ds.getSupportedGrantTypes(),
		TokenEndpointAuthMethodsSupported:          ds.getSupportedTokenEndpointAuthMethods(),
		CodeChallengeMethodsSupported:              ds.getSupportedCodeChallengeMethods(),
		AuthorizationResponseIssParameterSupported: true,
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
	return &OIDCProviderMetadata{
		OAuth2AuthorizationServerMetadata:    *oauth2Meta,
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
	}, nil
}

func (ds *discoveryService) getIssuer() string {
	return config.GetServerRuntime().Config.JWT.Issuer
}

func (ds *discoveryService) getAuthorizationEndpoint() string {
	return ds.baseURL + constants.OAuth2AuthorizationEndpoint
}

func (ds *discoveryService) getTokenEndpoint() string {
	return ds.baseURL + constants.OAuth2TokenEndpoint
}

func (ds *discoveryService) getJWKSUri() string {
	return ds.baseURL + constants.OAuth2JWKSEndpoint
}

func (ds *discoveryService) getIntrospectionEndpoint() string {
	return ds.baseURL + constants.OAuth2IntrospectionEndpoint
}

func (ds *discoveryService) getUserInfoEndpoint() string {
	return ds.baseURL + constants.OAuth2UserInfoEndpoint
}

func (ds *discoveryService) getRegistrationEndpoint() string {
	return ds.baseURL + constants.OAuth2DCREndpoint
}

func (ds *discoveryService) getSupportedScopes() []string {
	scopes := make([]string, 0, len(constants.StandardOIDCScopes))
	for scope := range constants.StandardOIDCScopes {
		scopes = append(scopes, scope)
	}
	return scopes
}

func (ds *discoveryService) getSupportedResponseTypes() []string {
	return constants.GetSupportedResponseTypes()
}

func (ds *discoveryService) getSupportedGrantTypes() []string {
	return constants.GetSupportedGrantTypes()
}

func (ds *discoveryService) getSupportedTokenEndpointAuthMethods() []string {
	return constants.GetSupportedTokenEndpointAuthMethods()
}

func (ds *discoveryService) getSupportedCodeChallengeMethods() []string {
	return pkce.GetSupportedCodeChallengeMethods()
}

func (ds *discoveryService) getPAREndpoint() string {
	return ds.baseURL + constants.OAuth2PAREndpoint
}

func (ds *discoveryService) isGlobalPARRequired() bool {
	return config.GetServerRuntime().Config.OAuth.PAR.RequirePAR
}

func (ds *discoveryService) getSupportedSubjectTypes() []string {
	return constants.GetSupportedSubjectTypes()
}

func (ds *discoveryService) getSupportedSigningAlgorithms(ctx context.Context) ([]string, error) {
	keys, err := ds.cryptoProvider.GetPublicKeys(ctx, kmprovider.PublicKeyFilter{})
	if err != nil {
		log.GetLogger().Error("Failed to retrieve public keys for signing algorithm discovery", log.Error(err))
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
		log.GetLogger().Error("No valid signing algorithms found in registered public keys", log.Error(err))
		return nil, err
	}
	return result, nil
}

func (ds *discoveryService) getSupportedAcrValues() []string {
	acrAMR := config.GetServerRuntime().Config.OAuth.AuthClass.AcrAMR
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
