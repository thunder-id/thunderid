// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/system/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
)

// The client Initialize builds refuses a user email endpoint on a loopback, private or link-local
// address, or on a hostname that resolves to one, unless they are allowed.
type InitTestSuite struct {
	suite.Suite
	server *httptest.Server
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupSuite() {
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
	suite.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"email":"user@example.com","primary":true}]`))
	}))
}

func (suite *InitTestSuite) TearDownSuite() {
	suite.server.Close()
}

// fetchPrimaryEmail reports whether the primary email was read from endpoint.
func (suite *InitTestSuite) fetchPrimaryEmail(allowPrivate bool, endpoint string) bool {
	service := Initialize(oauthmock.NewOAuthAuthnServiceInterfaceMock(suite.T()),
		syshttp.Initialize(allowPrivate)).(*githubOAuthAuthnService)
	email, err := service.fetchPrimaryEmail(context.Background(), &oauth.OAuthClientConfig{
		OAuthEndpoints: oauth.OAuthEndpoints{UserEmailEndpoint: endpoint},
	}, testAccessToken)
	return err == nil && email == "user@example.com"
}

func (suite *InitTestSuite) TestRefusesPrivateUserEmailEndpoints() {
	for _, endpoint := range []string{suite.server.URL, strings.Replace(suite.server.URL, "127.0.0.1", "localhost", 1),
		"http://10.0.0.1:8080/user/emails", "http://169.254.169.254/user/emails"} {
		suite.False(suite.fetchPrimaryEmail(false, endpoint), "user email endpoint %s was reached", endpoint)
	}
}

func (suite *InitTestSuite) TestReachesPrivateUserEmailEndpointsWhenAllowed() {
	loopback := strings.Replace(suite.server.URL, "127.0.0.1", "localhost", 1)
	for _, endpoint := range []string{suite.server.URL, loopback} {
		suite.True(suite.fetchPrimaryEmail(true, endpoint), "user email endpoint %s was refused", endpoint)
	}
}
