// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

// The federation client Initialize builds refuses identity provider endpoints on a loopback,
// private or link-local address, or on a hostname that resolves to one, unless they are allowed.
type OutboundGuardTestSuite struct {
	suite.Suite
	server *httptest.Server
}

func TestOutboundGuardTestSuite(t *testing.T) {
	suite.Run(t, new(OutboundGuardTestSuite))
}

func (suite *OutboundGuardTestSuite) SetupSuite() {
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/token") {
			_, _ = w.Write([]byte(testTokenRespJSON))
			return
		}
		_, _ = w.Write([]byte(`{"sub":"user_sub_123"}`))
	}))
}

func (suite *OutboundGuardTestSuite) TearDownSuite() {
	suite.server.Close()
}

// endpoints are the base URLs the guard refuses: the test server on a loopback address and on a
// hostname that resolves to one, a private address, and the link-local metadata address.
func (suite *OutboundGuardTestSuite) endpoints() []string {
	return []string{suite.server.URL, strings.Replace(suite.server.URL, "127.0.0.1", "localhost", 1),
		"http://10.0.0.1:8080", "http://169.254.169.254"}
}

// reachableEndpoints are the endpoints a test can reach when private addresses are allowed.
func (suite *OutboundGuardTestSuite) reachableEndpoints() []string {
	return suite.endpoints()[:2]
}

func (suite *OutboundGuardTestSuite) idpWithTokenEndpoint(tokenEndpoint string) *providers.IDPDTO {
	properties := make([]cmodels.Property, 0, 5)
	for name, value := range map[string]string{
		"client_id": "test_client", "client_secret": "test_secret", "redirect_uri": "https://app.com/callback",
		"scopes": "openid", "token_endpoint": tokenEndpoint,
	} {
		property, err := cmodels.NewProperty(name, value, false)
		suite.Require().NoError(err)
		properties = append(properties, *property)
	}
	return &providers.IDPDTO{ID: testIDPID, Name: "Test IDP", Type: providers.IDPTypeOAuth, Properties: properties}
}

// exchange reports whether a code was exchanged for a token at base's token endpoint.
func (suite *OutboundGuardTestSuite) exchange(allowPrivate bool, base string) bool {
	idpService := idpmock.NewIDPServiceInterfaceMock(suite.T())
	idpService.On("GetIdentityProvider", mock.Anything, testIDPID).
		Return(suite.idpWithTokenEndpoint(base+"/token"), nil)
	service := Initialize(idpService, syshttp.NewOutboundClientProvider(allowPrivate))
	tokenResp, err := service.ExchangeCodeForToken(context.Background(), testIDPID, "code", false)
	return err == nil && tokenResp != nil && tokenResp.AccessToken == "access123"
}

// fetchUserInfo reports whether user info was read from base's userinfo endpoint.
func (suite *OutboundGuardTestSuite) fetchUserInfo(allowPrivate bool, base string) bool {
	service := Initialize(idpmock.NewIDPServiceInterfaceMock(suite.T()),
		syshttp.NewOutboundClientProvider(allowPrivate))
	userInfo, err := service.FetchUserInfoWithClientConfig(context.Background(), &OAuthClientConfig{
		OAuthEndpoints: OAuthEndpoints{UserInfoEndpoint: base + "/userinfo"},
	}, "access123")
	return err == nil && userInfo["sub"] == "user_sub_123"
}

func (suite *OutboundGuardTestSuite) TestRefusesPrivateIdentityProviderEndpoints() {
	for _, base := range suite.endpoints() {
		suite.False(suite.exchange(false, base), "token endpoint %s was reached", base)
		suite.False(suite.fetchUserInfo(false, base), "userinfo endpoint %s was reached", base)
	}
}

func (suite *OutboundGuardTestSuite) TestReachesPrivateIdentityProviderEndpointsWhenAllowed() {
	for _, base := range suite.reachableEndpoints() {
		suite.True(suite.exchange(true, base), "token endpoint %s was refused", base)
		suite.True(suite.fetchUserInfo(true, base), "userinfo endpoint %s was refused", base)
	}
}

// A token endpoint that redirects is not followed: the redirect target never receives the client secret
// and code the exchange posts, and the exchange fails.
func (suite *OutboundGuardTestSuite) TestDoesNotFollowATokenEndpointRedirect() {
	var reached bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testTokenRespJSON))
	}))
	defer target.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/token", http.StatusTemporaryRedirect)
	}))
	defer redirecting.Close()

	suite.False(suite.exchange(true, redirecting.URL), "the exchange followed the redirect")
	suite.False(reached, "the redirect target received the token request")
}
