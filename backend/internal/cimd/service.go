// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package cimd supports clients registered from an OAuth Client ID Metadata Document: it previews a
// document and applies the document rules to the values submitted for such a client.
package cimd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/cert"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// CIMDServiceInterface previews Client ID Metadata Documents and applies the document rules to the
// values submitted for a CIMD application.
type CIMDServiceInterface interface {
	Preview(ctx context.Context, clientID string) (*PreviewResponse, *tidcommon.ServiceError)
	ValidateOAuthProfile(clientID string, profile *providers.OAuthProfile, hasClientSecret bool,
		existingClientID string, existing *providers.OAuthProfile) *tidcommon.ServiceError
}

// cimdService is the default implementation of CIMDServiceInterface.
type cimdService struct {
	httpClient syshttp.HTTPClientInterface
	logger     *log.Logger
}

// newCIMDService creates a new instance of cimdService. Documents are retrieved by a client that
// refuses private and loopback addresses on the connected address and follows no redirects.
func newCIMDService() CIMDServiceInterface {
	return &cimdService{
		// The Client ID Metadata Document URL is attacker-controllable input
		// (client_id discovery), so the SSRF dial guard stays on and redirects
		// are never followed: the fetcher must see the 3xx to reject a
		// document served behind one.
		httpClient: syshttp.NewHTTPClient(syshttp.HTTPClientConfig{
			GuardSSRF:       true,
			DisableRedirects: true,
		}),
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CIMDService")),
	}
}

// Preview retrieves and validates the Client ID Metadata Document at clientID and returns the values
// to store. It stores nothing.
func (s *cimdService) Preview(ctx context.Context, clientID string) (
	*PreviewResponse, *tidcommon.ServiceError) {
	clientIDURL, err := parseClientIdentifierURL(clientID)
	if err != nil {
		return nil, &ErrorInvalidClientID
	}

	doc, err := fetchCIMDDocument(ctx, s.httpClient, clientID)
	if err != nil {
		s.logger.Debug(ctx, "Failed to retrieve Client ID Metadata Document",
			log.String("clientID", clientID), log.Error(err))
		return nil, &ErrorDocumentUnavailable
	}
	if doc.ClientID != clientID {
		return nil, &ErrorClientIDMismatch
	}
	if doc.ClientSecret != "" {
		return nil, &ErrorClientSecretNotAllowed
	}

	oauthConfig, svcErr := buildCIMDOAuthConfig(clientID, doc)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateCIMDRules(clientID, ruleProfile(oauthConfig), false); svcErr != nil {
		return nil, svcErr
	}

	// The name and links are editable defaults, so a value the application would reject is shortened or
	// left out rather than failing the registration.
	name := doc.ClientName
	if name == "" {
		name = clientIDURL.Hostname()
	}
	if runes := []rune(name); len(runes) > maxApplicationNameLength {
		name = string(runes[:maxApplicationNameLength])
	}
	return &PreviewResponse{
		Name:      name,
		URL:       absoluteHTTPURL(doc.ClientURI),
		TosURI:    absoluteHTTPURL(doc.TosURI),
		PolicyURI: absoluteHTTPURL(doc.PolicyURI),
		Contacts:  doc.Contacts,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: oauthConfig},
		},
	}, nil
}

// fetchCIMDDocument retrieves the document at clientID as JSON within the time and size limits.
func fetchCIMDDocument(ctx context.Context, client syshttp.HTTPClientInterface, clientID string) (
	*cimdDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, cimdFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, errors.New("document is not served as application/json")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, cimdMaxDocumentSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > cimdMaxDocumentSize {
		return nil, errors.New("document exceeds the size limit")
	}

	var doc cimdDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// buildCIMDOAuthConfig maps a document to the OAuth configuration of a CIMD application. Grants are
// narrowed to authorization_code and refresh_token, never widened.
func buildCIMDOAuthConfig(clientID string, doc *cimdDocument) (
	*providers.OAuthConfigWithSecret, *tidcommon.ServiceError) {
	method := providers.TokenEndpointAuthMethod(doc.TokenEndpointAuthMethod)
	if method == "" {
		// RFC 7591 default, which a CIMD client cannot use.
		method = providers.TokenEndpointAuthMethodClientSecretBasic
	}

	var certificate *providers.Certificate
	if method == providers.TokenEndpointAuthMethodPrivateKeyJWT {
		hasJWKS := len(doc.JWKS) > 0 && string(doc.JWKS) != "null"
		switch {
		case (doc.JWKSURI != "") == hasJWKS:
			return nil, &ErrorInvalidAuthMethod
		case doc.JWKSURI != "":
			certificate = &providers.Certificate{Type: cert.CertificateTypeJWKSURI, Value: doc.JWKSURI}
		default:
			var jwks bytes.Buffer
			if err := json.Compact(&jwks, doc.JWKS); err != nil {
				return nil, &ErrorInvalidAuthMethod
			}
			certificate = &providers.Certificate{Type: cert.CertificateTypeJWKS, Value: jwks.String()}
		}
	}

	declaredGrants := doc.GrantTypes
	if len(declaredGrants) == 0 {
		declaredGrants = []string{string(providers.GrantTypeAuthorizationCode)}
	}
	if !slices.Contains(declaredGrants, string(providers.GrantTypeAuthorizationCode)) {
		return nil, &ErrorInvalidGrantTypes
	}
	grantTypes := []providers.GrantType{providers.GrantTypeAuthorizationCode}
	if slices.Contains(declaredGrants, string(providers.GrantTypeRefreshToken)) {
		grantTypes = append(grantTypes, providers.GrantTypeRefreshToken)
	}

	return &providers.OAuthConfigWithSecret{
		ClientID:                 clientID,
		ClientIDMetadataDocument: true,
		RedirectURIs:             doc.RedirectURIs,
		GrantTypes:               grantTypes,
		ResponseTypes:            []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod:  method,
		PublicClient:             method == providers.TokenEndpointAuthMethodNone,
		PKCERequired:             true,
		Certificate:              certificate,
	}, nil
}

// ValidateOAuthProfile applies the document rules to an OAuth profile marked as CIMD, and keeps the
// marker and the client ID fixed after creation. existing is the stored profile on an update, and nil
// on a create.
func (s *cimdService) ValidateOAuthProfile(clientID string, profile *providers.OAuthProfile,
	hasClientSecret bool, existingClientID string, existing *providers.OAuthProfile) *tidcommon.ServiceError {
	if existing != nil {
		if existing.ClientIDMetadataDocument != profile.ClientIDMetadataDocument ||
			(existing.ClientIDMetadataDocument && existingClientID != clientID) {
			return &ErrorImmutable
		}
	}
	if !profile.ClientIDMetadataDocument {
		return nil
	}
	return validateCIMDRules(clientID, profile, hasClientSecret)
}

// validateCIMDRules checks the document rules that apply to both a previewed document and the values
// submitted for a CIMD client.
func validateCIMDRules(clientID string, p *providers.OAuthProfile, hasClientSecret bool) *tidcommon.ServiceError {
	clientIDURL, err := parseClientIdentifierURL(clientID)
	if err != nil {
		return &ErrorInvalidClientID
	}
	if len(p.RedirectURIs) == 0 {
		return &ErrorInvalidRedirectURI
	}
	for _, redirectURI := range p.RedirectURIs {
		if !isCIMDRedirectURI(redirectURI, clientIDURL) {
			return &ErrorInvalidRedirectURI
		}
	}
	switch providers.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod) {
	case providers.TokenEndpointAuthMethodNone:
	case providers.TokenEndpointAuthMethodPrivateKeyJWT:
		if p.Certificate == nil ||
			(p.Certificate.Type != cert.CertificateTypeJWKS && p.Certificate.Type != cert.CertificateTypeJWKSURI) {
			return &ErrorInvalidAuthMethod
		}
		// The same checks the certificate store applies when the client is created.
		switch {
		case p.Certificate.Type == cert.CertificateTypeJWKSURI && (syshttp.IsSSRFSafeURL(p.Certificate.Value) != nil ||
			!isValidCertificateValue(p.Certificate.Value)):
			return &ErrorInvalidJWKSURI
		case p.Certificate.Type == cert.CertificateTypeJWKS && !isValidCertificateValue(p.Certificate.Value):
			return &ErrorInvalidKeySet
		}
	default:
		return &ErrorInvalidAuthMethod
	}
	if hasClientSecret {
		return &ErrorClientSecretNotAllowed
	}
	if !slices.Contains(p.GrantTypes, string(providers.GrantTypeAuthorizationCode)) {
		return &ErrorInvalidGrantTypes
	}
	for _, grantType := range p.GrantTypes {
		if grantType != string(providers.GrantTypeAuthorizationCode) &&
			grantType != string(providers.GrantTypeRefreshToken) {
			return &ErrorInvalidGrantTypes
		}
	}
	// Every CIMD client uses the authorization code grant, so PKCE is required for confidential clients too.
	if !p.PKCERequired {
		return &ErrorPKCERequired
	}
	return nil
}

// isValidCertificateValue reports whether a certificate value has a length the certificate store accepts.
func isValidCertificateValue(value string) bool {
	return len(value) >= minCertificateValueLength && len(value) <= maxCertificateValueLength
}

// ruleProfile returns the fields of a previewed configuration that the document rules read.
func ruleProfile(c *providers.OAuthConfigWithSecret) *providers.OAuthProfile {
	return &providers.OAuthProfile{
		RedirectURIs:            c.RedirectURIs,
		GrantTypes:              sysutils.ConvertToStringSlice(c.GrantTypes),
		TokenEndpointAuthMethod: string(c.TokenEndpointAuthMethod),
		PKCERequired:            c.PKCERequired,
		Certificate:             c.Certificate,
	}
}

// parseClientIdentifierURL parses a Client Identifier URL: an https URL with a DNS host and a path,
// with no user information, fragment or dot segments.
func parseClientIdentifierURL(raw string) (*url.URL, error) {
	if len(raw) > maxClientIdentifierURLLength {
		return nil, errors.New("client identifier URL is too long")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != schemeHTTPS || u.User != nil || strings.Contains(raw, "#") {
		return nil, errors.New("client identifier URL must be https with no user information or fragment")
	}
	if host := u.Hostname(); host == "" || net.ParseIP(host) != nil {
		return nil, errors.New("client identifier URL must have a DNS host")
	}
	if u.Path == "" || u.Path == "/" {
		return nil, errors.New("client identifier URL must have a path")
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, errors.New("client identifier URL must not contain dot segments")
		}
	}
	return u, nil
}

// isCIMDRedirectURI reports whether a redirect URI is https on the client ID's origin, or http on a
// loopback address.
func isCIMDRedirectURI(raw string, clientIDURL *url.URL) bool {
	// A document supplies exact redirect URIs only: no fragment, and no wildcard pattern even when the
	// deployment allows them.
	if strings.ContainsAny(raw, "#*") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case schemeHTTPS:
		return strings.EqualFold(u.Host, clientIDURL.Host)
	case "http":
		host := u.Hostname()
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	}
	return false
}

// absoluteHTTPURL returns raw when it is an absolute http or https URL, and an empty string otherwise.
func absoluteHTTPURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != schemeHTTPS && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return raw
}
