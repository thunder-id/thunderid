// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"slices"
	"sort"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
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
	cryptoProvider providers.RuntimeCryptoProvider
	jweService     jwe.JWEServiceInterface
}

// newDiscoveryService creates a new discovery service instance
func newDiscoveryService(
	cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface, cfg oauthconfig.Config,
) DiscoveryServiceInterface {
	return &discoveryService{
		cfg:            cfg,
		cryptoProvider: cryptoProvider,
		jweService:     jweService,
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
		ResponseTypesSupported:                     ds.getAllowedResponseTypes(),
		GrantTypesSupported:                        ds.getAllowedGrantTypes(),
		TokenEndpointAuthMethodsSupported:          ds.getAllowedTokenEndpointAuthMethods(),
		TokenEndpointAuthSigningAlgValuesSupported: ds.getSupportedTokenEndpointAuthSigningAlgs(),
		CodeChallengeMethodsSupported:              ds.getSupportedCodeChallengeMethods(),
		AuthorizationResponseIssParameterSupported: true,
		DPoPSigningAlgValuesSupported:              ds.getSupportedDPoPSigningAlgs(),
		AuthorizationGrantProfilesSupported:        ds.getSupportedAuthorizationGrantProfiles(),
	}

	if slices.Contains(metadata.GrantTypesSupported, string(providers.GrantTypeCIBA)) {
		metadata.BackchannelAuthenticationEndpoint = ds.getBackchannelAuthenticationEndpoint()
		metadata.BackchannelTokenDeliveryModesSupported = []string{"poll"}
		metadata.BackchannelUserCodeParameterSupported = false
	}
	if ds.cfg.OAuth.TokenRevocation.IsEnabled() {
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
	encryptionAlgs := ds.jweService.SupportedKeyEncryptionAlgorithms()
	encryptionEncs := ds.jweService.SupportedContentEncryptionAlgorithms()

	oidcProviderMetadata := &OIDCProviderMetadata{
		OAuth2AuthorizationServerMetadata:    *oauth2Meta,
		UserInfoEndpoint:                     ds.getUserInfoEndpoint(),
		ScopesSupported:                      ds.getAllowedScopes(),
		SubjectTypesSupported:                ds.getAllowedSubjectTypes(),
		IDTokenSigningAlgValuesSupported:     signingAlgs,
		UserInfoSigningAlgValuesSupported:    signingAlgs,
		UserInfoEncryptionAlgValuesSupported: encryptionAlgs,
		UserInfoEncryptionEncValuesSupported: encryptionEncs,
		IDTokenEncryptionAlgValuesSupported:  encryptionAlgs,
		IDTokenEncryptionEncValuesSupported:  encryptionEncs,
		ClaimsSupported:                      ds.getAllowedClaims(),
		ClaimsParameterSupported:             true,
		// JAR (RFC 9101) is not implemented.
		RequestParameterSupported:    false,
		RequestURIParameterSupported: false,
		AcrValuesSupported:           ds.getSupportedAcrValues(),
	}

	if ds.cfg.OAuth.Logout.IsEnabled() {
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

func (ds *discoveryService) getAllowedScopes() []string {
	return ds.cfg.OAuth.AllowedScopes
}

func (ds *discoveryService) getAllowedResponseTypes() []string {
	return ds.cfg.OAuth.AllowedResponseTypes
}

func (ds *discoveryService) getAllowedGrantTypes() []string {
	return ds.cfg.OAuth.AllowedGrantTypes
}

func (ds *discoveryService) getAllowedTokenEndpointAuthMethods() []string {
	return ds.cfg.OAuth.AllowedAuthMethods
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
	return ds.cryptoProvider.GetSupportedSigningAlgorithms()
}

func (ds *discoveryService) getSupportedTokenEndpointAuthSigningAlgs() []string {
	return ds.cryptoProvider.GetSupportedSigningAlgorithms()
}

func (ds *discoveryService) getAllowedSubjectTypes() []string {
	return ds.cfg.OAuth.AllowedSubjectTypes
}

func (ds *discoveryService) getSupportedSigningAlgorithms(ctx context.Context) ([]string, error) {
	keys, err := ds.cryptoProvider.GetPublicKeys(ctx, providers.PublicKeyFilter{})
	if err != nil {
		log.GetLogger().Error(ctx,
			"Failed to retrieve public keys for signing algorithm discovery", log.Error(err))
		return nil, err
	}
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		alg := k.Algorithm
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

func (ds *discoveryService) getAllowedClaims() []string {
	return ds.cfg.OAuth.AllowedClaims
}

func (ds *discoveryService) getSupportedAuthorizationGrantProfiles() []string {
	supportedProfiles := make([]string, 0)
	// support Identity Assertion JWT Authorization Grant profile if the JWT Bearer grant type is supported
	if slices.Contains(ds.getAllowedGrantTypes(), string(providers.GrantTypeJWTBearer)) {
		supportedProfiles = append(supportedProfiles, string(constants.SupportedAuthorizationGrantProfileIDJAG))
	}

	return supportedProfiles
}
