// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"slices"
	"sort"

	authzdetail "github.com/thunder-id/thunderid/internal/authzdetail/common"
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
	ListAuthorizationDetailsTypeMetadata(ctx context.Context) ([]AuthorizationDetailsTypeMetadata, error)
	GetAuthorizationDetailsTypeMetadata(ctx context.Context, value string) (*AuthorizationDetailsTypeMetadata,
		error)
}

// discoveryService implements DiscoveryServiceInterface
type discoveryService struct {
	cfg              oauthconfig.Config
	cryptoProvider   providers.RuntimeCryptoProvider
	jweService       jwe.JWEServiceInterface
	authzDetailTypes providers.AuthorizationDetailTypeProvider
}

// newDiscoveryService creates a new discovery service instance
func newDiscoveryService(
	cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface,
	authzDetailTypes providers.AuthorizationDetailTypeProvider, cfg oauthconfig.Config,
) DiscoveryServiceInterface {
	return &discoveryService{
		cfg:              cfg,
		cryptoProvider:   cryptoProvider,
		jweService:       jweService,
		authzDetailTypes: authzDetailTypes,
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
		ClientIDMetadataDocumentSupported:          ds.cfg.OAuth.CIMD.IsEnabled(),
		AuthorizationDetailsTypesSupported:         ds.getSupportedAuthorizationDetailsTypes(ctx),
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
	if ds.isTypeMetadataEnabled() {
		metadata.AuthorizationDetailsTypesEndpoint = ds.cfg.BaseURL + constants.OAuth2AuthorizationDetailsTypesEndpoint
	}
	return metadata
}

// getSupportedAuthorizationDetailsTypes returns the registered type values, or nil when none can be read.
func (ds *discoveryService) getSupportedAuthorizationDetailsTypes(ctx context.Context) []string {
	if ds.authzDetailTypes == nil {
		return nil
	}
	registered, svcErr := ds.authzDetailTypes.ListAllAuthorizationDetailTypes(ctx)
	if svcErr != nil {
		log.GetLogger().Error(ctx, "Failed to list authorization details types", log.String("code", svcErr.Code))
		return nil
	}
	types := make([]string, 0, len(registered))
	for i := range registered {
		types = append(types, registered[i].Type)
	}
	return types
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
	// Delivery does not depend on the logout endpoint, so neither do these. Every logout token
	// carries sid, so session support follows delivery itself.
	backchannel := ds.cfg.OAuth.Logout.Backchannel.IsEnabled()
	oidcProviderMetadata.BackchannelLogoutSupported = backchannel
	oidcProviderMetadata.BackchannelLogoutSessionSupported = backchannel

	return oidcProviderMetadata, nil
}

// ListAuthorizationDetailsTypeMetadata returns the request view of every registered type.
func (ds *discoveryService) ListAuthorizationDetailsTypeMetadata(
	ctx context.Context) ([]AuthorizationDetailsTypeMetadata, error) {
	if ds.authzDetailTypes == nil {
		return []AuthorizationDetailsTypeMetadata{}, nil
	}
	registered, svcErr := ds.authzDetailTypes.ListAllAuthorizationDetailTypes(ctx)
	if svcErr != nil {
		return nil, errAuthorizationDetailsTypesUnavailable
	}
	types := make([]AuthorizationDetailsTypeMetadata, 0, len(registered))
	for i := range registered {
		metadata, err := requestView(&registered[i])
		if err != nil {
			return nil, err
		}
		types = append(types, *metadata)
	}
	return types, nil
}

// GetAuthorizationDetailsTypeMetadata returns the request view of the type registered under value, or nil.
func (ds *discoveryService) GetAuthorizationDetailsTypeMetadata(ctx context.Context,
	value string) (*AuthorizationDetailsTypeMetadata, error) {
	if ds.authzDetailTypes == nil {
		return nil, nil
	}
	t, svcErr := ds.authzDetailTypes.GetAuthorizationDetailTypeByValue(ctx, value)
	if svcErr != nil {
		return nil, errAuthorizationDetailsTypesUnavailable
	}
	if t == nil {
		return nil, nil
	}
	return requestView(t)
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

// getAllowedGrantTypes returns the allowed grant types to advertise, less jwt-bearer while ID-JAG
// acceptance, the only use of that grant, is off and the grant is therefore not served.
func (ds *discoveryService) getAllowedGrantTypes() []string {
	if ds.cfg.OAuth.IDJAG.Acceptance.IsEnabled() {
		return ds.cfg.OAuth.AllowedGrantTypes
	}
	return slices.DeleteFunc(slices.Clone(ds.cfg.OAuth.AllowedGrantTypes), func(grantType string) bool {
		return grantType == string(providers.GrantTypeJWTBearer)
	})
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
	algs := ds.cfg.OAuth.DPoP.AllowedAlgs
	if len(algs) == 0 {
		return nil
	}
	out := make([]string, len(algs))
	copy(out, algs)
	return out
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

// isTypeMetadataEnabled reports whether the type metadata endpoint is served and advertised.
func (ds *discoveryService) isTypeMetadataEnabled() bool {
	return ds.cfg.OAuth.AuthorizationDetails.IsTypeMetadataEnabled()
}

// requestView returns the metadata a client sees for a type.
func requestView(t *providers.AuthorizationDetailType) (*AuthorizationDetailsTypeMetadata, error) {
	schema, err := authzdetail.RequestSchema(t.Schema)
	if err != nil {
		return nil, err
	}
	return &AuthorizationDetailsTypeMetadata{
		Type:          t.Type,
		Name:          t.Name,
		Description:   t.Description,
		Version:       t.Version,
		Schema:        schema,
		Modifications: authzdetail.ClientModifications(t.Schema),
	}, nil
}
