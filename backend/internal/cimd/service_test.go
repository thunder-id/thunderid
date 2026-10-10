// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/system/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

const testCIMDClientID = "https://client.example.com/oauth/client.json"

type CIMDServiceTestSuite struct {
	suite.Suite
}

func TestCIMDServiceTestSuite(t *testing.T) {
	suite.Run(t, new(CIMDServiceTestSuite))
}

func (s *CIMDServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
}

func (s *CIMDServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func newTestCIMDService() *cimdService {
	return &cimdService{logger: log.GetLogger()}
}

func cimdResponse(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func (s *CIMDServiceTestSuite) stubCIMDHTTPClient(resp *http.Response, err error) syshttp.HTTPClientInterface {
	client := httpmock.NewHTTPClientInterfaceMock(s.T())
	client.EXPECT().Do(mock.Anything).Return(resp, err)
	return client
}

// checkCIMDRules applies the document rules to a configuration, as preview does.
func checkCIMDRules(c *providers.OAuthConfigWithSecret) *tidcommon.ServiceError {
	return validateCIMDRules(c.ClientID, ruleProfile(c), c.ClientSecret != "")
}

func publicCIMDConfig() *providers.OAuthConfigWithSecret {
	return &providers.OAuthConfigWithSecret{
		ClientID:                 testCIMDClientID,
		ClientIDMetadataDocument: true,
		RedirectURIs:             []string{"https://client.example.com/callback", "http://127.0.0.1/callback"},
		GrantTypes:               []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:            []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod:  providers.TokenEndpointAuthMethodNone,
		PublicClient:             true,
		PKCERequired:             true,
	}
}

func (s *CIMDServiceTestSuite) TestParseClientIdentifierURL() {
	cases := []struct {
		name  string
		url   string
		valid bool
	}{
		{"valid", testCIMDClientID, true},
		{"valid with query", testCIMDClientID + "?v=1", true},
		{"http", "http://client.example.com/client.json", false},
		{"no path", "https://client.example.com", false},
		{"root path", "https://client.example.com/", false},
		{"IPv4 host", "https://203.0.113.10/client.json", false},
		{"IPv6 host", "https://[2001:db8::1]/client.json", false},
		{"user information", "https://user@client.example.com/client.json", false},
		{"fragment", testCIMDClientID + "#frag", false},
		{"empty fragment", testCIMDClientID + "#", false},
		{"dot segment", "https://client.example.com/a/../client.json", false},
		{"single dot segment", "https://client.example.com/./client.json", false},
		{"encoded dot segment", "https://client.example.com/a/%2e%2e/client.json", false},
		{"too long", "https://client.example.com/" + strings.Repeat("a", maxClientIdentifierURLLength), false},
		{"not a URL", "https://client.example.com/%zz", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := parseClientIdentifierURL(tc.url)
			s.Equal(tc.valid, err == nil)
		})
	}
}

func (s *CIMDServiceTestSuite) TestIsCIMDRedirectURI() {
	clientIDURL, err := url.Parse(testCIMDClientID)
	s.Require().NoError(err)
	cases := []struct {
		uri   string
		valid bool
	}{
		{"https://client.example.com/callback", true},
		{"https://CLIENT.example.com/callback", true},
		{"http://localhost/callback", true},
		{"http://127.0.0.1:8080/callback", true},
		{"http://[::1]/callback", true},
		{"https://other.example.com/callback", false},
		{"https://client.example.com:8443/callback", false},
		{"https://client.example.com@other.example.com/callback", false},
		{"http://client.example.com/callback", false},
		{"http://192.168.1.10/callback", false},
		{"com.example.app:/callback", false},
		{"%zz", false},
		{"https://client.example.com/callback#done", false},
		{"https://client.example.com/callback#", false},
		{"https://client.example.com/*/callback", false},
		{"https://client.example.com/callback?state=*", false},
		{"http://localhost/**", false},
	}
	for _, tc := range cases {
		s.Run(tc.uri, func() {
			s.Equal(tc.valid, isCIMDRedirectURI(tc.uri, clientIDURL))
		})
	}
}

func (s *CIMDServiceTestSuite) TestBuildCIMDOAuthConfig_PublicClient() {
	doc := &cimdDocument{
		ClientID:                testCIMDClientID,
		RedirectURIs:            []string{"http://127.0.0.1/callback"},
		TokenEndpointAuthMethod: "none",
		JWKSURI:                 "https://client.example.com/jwks.json",
	}

	cfg, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)

	s.Require().Nil(svcErr)
	s.Equal(testCIMDClientID, cfg.ClientID)
	s.True(cfg.ClientIDMetadataDocument)
	s.True(cfg.PublicClient)
	s.True(cfg.PKCERequired)
	s.Equal(providers.TokenEndpointAuthMethodNone, cfg.TokenEndpointAuthMethod)
	s.Nil(cfg.Certificate)
	s.Equal([]providers.GrantType{providers.GrantTypeAuthorizationCode}, cfg.GrantTypes)
	s.Equal([]providers.ResponseType{providers.ResponseTypeCode}, cfg.ResponseTypes)
}

func (s *CIMDServiceTestSuite) TestBuildCIMDOAuthConfig_PrivateKeyJWT() {
	s.Run("jwks_uri", func() {
		doc := &cimdDocument{TokenEndpointAuthMethod: "private_key_jwt", JWKSURI: "https://client.example.com/jwks"}
		cfg, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)
		s.Require().Nil(svcErr)
		s.False(cfg.PublicClient)
		s.Equal(&providers.Certificate{Type: cert.CertificateTypeJWKSURI,
			Value: "https://client.example.com/jwks"}, cfg.Certificate)
	})
	s.Run("inline jwks", func() {
		doc := &cimdDocument{TokenEndpointAuthMethod: "private_key_jwt",
			JWKS: json.RawMessage(`{ "keys": [ {"kty": "EC"} ] }`)}
		cfg, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)
		s.Require().Nil(svcErr)
		s.Equal(&providers.Certificate{Type: cert.CertificateTypeJWKS,
			Value: `{"keys":[{"kty":"EC"}]}`}, cfg.Certificate)
	})
	s.Run("both key sources", func() {
		doc := &cimdDocument{TokenEndpointAuthMethod: "private_key_jwt",
			JWKSURI: "https://client.example.com/jwks", JWKS: json.RawMessage(`{"keys":[]}`)}
		_, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)
		s.Equal(&ErrorInvalidAuthMethod, svcErr)
	})
	s.Run("no key source", func() {
		doc := &cimdDocument{TokenEndpointAuthMethod: "private_key_jwt", JWKS: json.RawMessage(`null`)}
		_, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)
		s.Equal(&ErrorInvalidAuthMethod, svcErr)
	})
}

func (s *CIMDServiceTestSuite) TestBuildCIMDOAuthConfig_GrantTypes() {
	s.Run("narrowed", func() {
		doc := &cimdDocument{TokenEndpointAuthMethod: "none", GrantTypes: []string{
			"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:device_code"}}
		cfg, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)
		s.Require().Nil(svcErr)
		s.Equal([]providers.GrantType{
			providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken}, cfg.GrantTypes)
	})
	s.Run("without authorization_code", func() {
		doc := &cimdDocument{TokenEndpointAuthMethod: "none", GrantTypes: []string{"client_credentials"}}
		_, svcErr := buildCIMDOAuthConfig(testCIMDClientID, doc)
		s.Equal(&ErrorInvalidGrantTypes, svcErr)
	})
}

func (s *CIMDServiceTestSuite) TestBuildCIMDOAuthConfig_DefaultAuthMethod() {
	cfg, svcErr := buildCIMDOAuthConfig(testCIMDClientID,
		&cimdDocument{RedirectURIs: []string{"https://client.example.com/callback"}})

	s.Require().Nil(svcErr)
	s.Equal(providers.TokenEndpointAuthMethodClientSecretBasic, cfg.TokenEndpointAuthMethod)
	s.Equal(&ErrorInvalidAuthMethod, checkCIMDRules(cfg))
}

func (s *CIMDServiceTestSuite) TestValidateCIMDRules() {
	cases := []struct {
		name   string
		modify func(c *providers.OAuthConfigWithSecret)
		want   *tidcommon.ServiceError
	}{
		{"valid public client", func(c *providers.OAuthConfigWithSecret) {}, nil},
		{"valid confidential client", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "https://x/jwks"}
		}, nil},
		{"invalid client ID", func(c *providers.OAuthConfigWithSecret) {
			c.ClientID = "my-client"
		}, &ErrorInvalidClientID},
		{"client secret", func(c *providers.OAuthConfigWithSecret) {
			c.ClientSecret = "secret"
		}, &ErrorClientSecretNotAllowed},
		{"no redirect URIs", func(c *providers.OAuthConfigWithSecret) {
			c.RedirectURIs = nil
		}, &ErrorInvalidRedirectURI},
		{"foreign redirect URI", func(c *providers.OAuthConfigWithSecret) {
			c.RedirectURIs = append(c.RedirectURIs, "https://other.example.com/callback")
		}, &ErrorInvalidRedirectURI},
		{"secret-based method", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodClientSecretBasic
		}, &ErrorInvalidAuthMethod},
		{"secret-based method with a generated secret", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodClientSecretBasic
			c.ClientSecret = "generated"
		}, &ErrorInvalidAuthMethod},
		{"http JWKS URI", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{
				Type: cert.CertificateTypeJWKSURI, Value: "http://client.example.com/jwks"}
		}, &ErrorInvalidJWKSURI},
		{"loopback JWKS URI", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "https://127.0.0.1/jwks"}
		}, &ErrorInvalidJWKSURI},
		{"private address JWKS URI", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "https://10.0.0.5/jwks"}
		}, &ErrorInvalidJWKSURI},
		{"private_key_jwt without keys", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
		}, &ErrorInvalidAuthMethod},
		{"without authorization_code", func(c *providers.OAuthConfigWithSecret) {
			c.GrantTypes = []providers.GrantType{providers.GrantTypeRefreshToken}
		}, &ErrorInvalidGrantTypes},
		{"public client without PKCE", func(c *providers.OAuthConfigWithSecret) {
			c.PKCERequired = false
		}, &ErrorPKCERequired},
		{"confidential client without PKCE", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.PKCERequired = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "https://x/jwks"}
		}, &ErrorPKCERequired},
		{"JWKS URI too long", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKSURI,
				Value: "https://client.example.com/" + strings.Repeat("a", 4096)}
		}, &ErrorInvalidJWKSURI},
		{"inline JWKS too small", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"}
		}, &ErrorInvalidKeySet},
		{"inline JWKS too large", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKS,
				Value: `{"keys":[{"kty":"RSA","n":"` + strings.Repeat("a", 4096) + `"}]}`}
		}, &ErrorInvalidKeySet},
		{"valid inline JWKS", func(c *providers.OAuthConfigWithSecret) {
			c.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
			c.PublicClient = false
			c.Certificate = &providers.Certificate{Type: cert.CertificateTypeJWKS, Value: `{"keys":[{"kty":"EC"}]}`}
		}, nil},
		{"widened grants", func(c *providers.OAuthConfigWithSecret) {
			c.GrantTypes = append(c.GrantTypes, providers.GrantTypeClientCredentials)
		}, &ErrorInvalidGrantTypes},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			c := publicCIMDConfig()
			tc.modify(c)
			s.Equal(tc.want, checkCIMDRules(c))
		})
	}
}

func (s *CIMDServiceTestSuite) TestValidateOAuthProfile() {
	validate := func(svc *cimdService, c *providers.OAuthConfigWithSecret, existingClientID string,
		existing *providers.OAuthProfile) *tidcommon.ServiceError {
		p := ruleProfile(c)
		p.ClientIDMetadataDocument = c.ClientIDMetadataDocument
		return svc.ValidateOAuthProfile(c.ClientID, p, c.ClientSecret != "", existingClientID, existing)
	}
	stored := func(cimd bool) *providers.OAuthProfile {
		return &providers.OAuthProfile{ClientIDMetadataDocument: cimd}
	}

	s.Run("valid create", func() {
		s.Nil(validate(newTestCIMDService(), publicCIMDConfig(), "", nil))
	})
	s.Run("create applies the rules", func() {
		c := publicCIMDConfig()
		c.RedirectURIs = []string{"https://other.example.com/callback"}
		s.Equal(&ErrorInvalidRedirectURI, validate(newTestCIMDService(), c, "", nil))
	})
	s.Run("create with a client secret", func() {
		c := publicCIMDConfig()
		c.ClientSecret = "secret"
		s.Equal(&ErrorClientSecretNotAllowed, validate(newTestCIMDService(), c, "", nil))
	})
	s.Run("non-CIMD client is not checked", func() {
		c := &providers.OAuthConfigWithSecret{ClientID: "my-client"}
		s.Nil(validate(newTestCIMDService(), c, "my-client", stored(false)))
	})
	s.Run("update keeps the marker and client ID", func() {
		s.Nil(validate(newTestCIMDService(), publicCIMDConfig(), testCIMDClientID, stored(true)))
	})
	s.Run("update applies the rules", func() {
		c := publicCIMDConfig()
		c.RedirectURIs = []string{"https://other.example.com/callback"}
		s.Equal(&ErrorInvalidRedirectURI,
			validate(newTestCIMDService(), c, testCIMDClientID, stored(true)))
	})
	s.Run("update changes the client ID", func() {
		s.Equal(&ErrorImmutable, validate(newTestCIMDService(), publicCIMDConfig(),
			"https://client.example.com/other.json", stored(true)))
	})
	s.Run("update removes the marker", func() {
		c := publicCIMDConfig()
		c.ClientIDMetadataDocument = false
		s.Equal(&ErrorImmutable, validate(newTestCIMDService(), c, testCIMDClientID, stored(true)))
	})
	s.Run("update adds the marker", func() {
		s.Equal(&ErrorImmutable, validate(newTestCIMDService(), publicCIMDConfig(),
			testCIMDClientID, stored(false)))
	})
}

func (s *CIMDServiceTestSuite) TestNewCIMDService() {
	svc := newCIMDService().(*cimdService)

	s.NotNil(svc.httpClient)
	s.NotNil(svc.logger)
}

func (s *CIMDServiceTestSuite) TestFetchCIMDDocument() {
	validBody := `{"client_id":"` + testCIMDClientID + `","client_name":"Client"}`
	cases := []struct {
		name    string
		resp    *http.Response
		err     error
		wantErr bool
	}{
		{"valid", cimdResponse(http.StatusOK, "application/json", validBody), nil, false},
		{"valid with charset", cimdResponse(http.StatusOK, "application/json; charset=utf-8", validBody), nil, false},
		{"redirect", cimdResponse(http.StatusFound, "application/json", ""), nil, true},
		{"not found", cimdResponse(http.StatusNotFound, "application/json", ""), nil, true},
		{"wrong media type", cimdResponse(http.StatusOK, "text/html", validBody), nil, true},
		{"too large", cimdResponse(http.StatusOK, "application/json",
			`{"client_name":"`+strings.Repeat("a", cimdMaxDocumentSize)+`"}`), nil, true},
		{"invalid JSON", cimdResponse(http.StatusOK, "application/json", `{`), nil, true},
		{"connection error", nil, errors.New("connection refused"), true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			client := httpmock.NewHTTPClientInterfaceMock(s.T())
			client.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && req.URL.String() == testCIMDClientID &&
					req.Header.Get("Accept") == "application/json"
			})).Return(tc.resp, tc.err)

			doc, err := fetchCIMDDocument(context.Background(), client, testCIMDClientID)

			if tc.wantErr {
				s.Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(testCIMDClientID, doc.ClientID)
			s.Equal("Client", doc.ClientName)
		})
	}
}

func (s *CIMDServiceTestSuite) TestFetchCIMDDocument_RefusesLoopbackAddress() {
	svc := newCIMDService().(*cimdService)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	s.Require().NoError(err)

	_, err = fetchCIMDDocument(context.Background(), svc.httpClient,
		"https://localhost:"+serverURL.Port()+"/client.json")

	s.ErrorContains(err, "private address")
}

func (s *CIMDServiceTestSuite) TestPreview() {
	document := func(fields string) *http.Response {
		return cimdResponse(http.StatusOK, "application/json", `{"client_id":"`+testCIMDClientID+`",`+
			`"redirect_uris":["https://client.example.com/callback"],"token_endpoint_auth_method":"none"`+
			fields+`}`)
	}

	s.Run("valid document", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(document(`,"client_name":"Client",`+
			`"client_uri":"https://client.example.com",`+
			`"tos_uri":"https://client.example.com/tos","policy_uri":"https://client.example.com/privacy",`+
			`"contacts":["support@example.com"],"grant_types":["authorization_code","refresh_token"]`), nil)

		preview, svcErr := svc.Preview(context.Background(), testCIMDClientID)

		s.Require().Nil(svcErr)
		s.Equal("Client", preview.Name)
		s.Equal("https://client.example.com", preview.URL)
		s.Equal("https://client.example.com/tos", preview.TosURI)
		s.Equal("https://client.example.com/privacy", preview.PolicyURI)
		s.Equal([]string{"support@example.com"}, preview.Contacts)
		s.Require().Len(preview.InboundAuthConfig, 1)
		cfg := preview.InboundAuthConfig[0].OAuthConfig
		s.True(cfg.ClientIDMetadataDocument)
		s.Equal([]string{"https://client.example.com/callback"}, cfg.RedirectURIs)
		s.Equal([]providers.GrantType{
			providers.GrantTypeAuthorizationCode, providers.GrantTypeRefreshToken}, cfg.GrantTypes)
	})
	s.Run("links that are not absolute URLs are left out", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(document(`,"client_uri":"/about","tos_uri":"terms.html",`+
			`"policy_uri":"mailto:privacy@example.com"`), nil)
		preview, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Require().Nil(svcErr)
		s.Empty(preview.URL)
		s.Empty(preview.TosURI)
		s.Empty(preview.PolicyURI)
	})
	s.Run("a long name is shortened", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(document(`,"client_name":"`+strings.Repeat("é", 150)+`"`), nil)
		preview, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Require().Nil(svcErr)
		s.Equal(strings.Repeat("é", 100), preview.Name)
	})
	s.Run("name falls back to the host", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(document(""), nil)
		preview, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Require().Nil(svcErr)
		s.Equal("client.example.com", preview.Name)
	})
	s.Run("invalid URL", func() {
		svc := newTestCIMDService()
		_, svcErr := svc.Preview(context.Background(), "http://client.example.com/client.json")
		s.Equal(&ErrorInvalidClientID, svcErr)
	})
	s.Run("unavailable", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(nil, errors.New("connection refused"))
		_, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Equal(&ErrorDocumentUnavailable, svcErr)
	})
	s.Run("client ID mismatch", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(cimdResponse(http.StatusOK, "application/json",
			`{"client_id":"https://client.example.com/other.json"}`), nil)
		_, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Equal(&ErrorClientIDMismatch, svcErr)
	})
	s.Run("client secret", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(document(`,"client_secret":"secret"`), nil)
		_, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Equal(&ErrorClientSecretNotAllowed, svcErr)
	})
	s.Run("rule violation", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(cimdResponse(http.StatusOK, "application/json", `{"client_id":"`+
			testCIMDClientID+`","redirect_uris":["https://other.example.com/cb"],`+
			`"token_endpoint_auth_method":"none"}`), nil)
		_, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Equal(&ErrorInvalidRedirectURI, svcErr)
	})
	s.Run("unsafe JWKS URI", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(cimdResponse(http.StatusOK, "application/json", `{"client_id":"`+
			testCIMDClientID+`","redirect_uris":["https://client.example.com/callback"],`+
			`"token_endpoint_auth_method":"private_key_jwt","jwks_uri":"http://client.example.com/jwks"}`), nil)
		_, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Equal(&ErrorInvalidJWKSURI, svcErr)
	})
	s.Run("mapping violation", func() {
		svc := newTestCIMDService()
		svc.httpClient = s.stubCIMDHTTPClient(document(`,"grant_types":["client_credentials"]`), nil)
		_, svcErr := svc.Preview(context.Background(), testCIMDClientID)
		s.Equal(&ErrorInvalidGrantTypes, svcErr)
	})
}
