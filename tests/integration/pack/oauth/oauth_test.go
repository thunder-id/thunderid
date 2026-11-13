// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/pack/packtest"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const loginRedirectURI = "https://localhost:3000/pack-callback"

type OAuthTestSuite struct {
	packtest.Suite
}

func TestOAuthTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthTestSuite))
}

func (ts *OAuthTestSuite) TestAnApplicationAuthoredOnTheControlPlaneIssuesATokenOnTheDataPlane() {
	clientID := packtest.Unique("pack-client")
	clientSecret := packtest.Unique("pack-secret")
	appID := ts.CreateClientCredentialsApp(clientID, clientSecret)

	// The data plane does not hold the application until the control plane has synced it.
	ts.Sync()

	status, raw := ts.Call(ts.DP, http.MethodGet, "/applications/"+appID, nil, nil)
	ts.Require().Equal(http.StatusOK, status, "the application did not reach the data plane: %s", raw)

	token := ts.requestClientCredentialsToken(clientID, clientSecret)
	ts.NotEmpty(token, "the data plane issued no access token for the synced client")
}

// requestClientCredentialsToken asks the data plane for a token as a client.
func (ts *OAuthTestSuite) requestClientCredentialsToken(clientID, clientSecret string) string {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodPost, testutils.PackDPURL()+"/oauth2/token", nil)
	ts.Require().NoError(err)
	req.SetBasicAuth(clientID, clientSecret)
	status, raw, err := testutils.PackAnonymous(testutils.PackDPURL()).PostForm("/oauth2/token",
		url.Values{"grant_type": {"client_credentials"}}, map[string]string{"Authorization": req.Header.Get("Authorization")})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var issued struct {
		AccessToken string `json:"access_token"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &issued))
	return issued.AccessToken
}

// A person created on the control plane signs in on the data plane through an application created on
// the control plane. The password is a secret, which the control plane sends to the data plane
// beside the version, so this also proves the two arrive together.
func (ts *OAuthTestSuite) TestAPersonSignsInOnTheDataPlaneWithAPasswordSetOnTheControlPlane() {
	clientID := packtest.Unique("pack-login")
	username := packtest.Unique("pack-person")
	password := "Pack-" + packtest.Unique("password")

	var app struct {
		ID string `json:"id"`
	}
	status, raw := ts.Call(ts.CP, http.MethodPost, "/applications", map[string]any{
		"name":                      clientID,
		"description":               "Sign-in application created on the control plane by the pack suite",
		"type":                      "browser",
		"ouId":                      packtest.DefaultOUID,
		"authFlowId":                packtest.DefaultAuthFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId":                clientID,
				"redirectUris":            []string{loginRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"pkceRequired":            true,
				"publicClient":            true,
				"tokenEndpointAuthMethod": "none",
			},
		}},
	}, &app)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.RemoveOnCleanup("/applications/" + app.ID)

	var user struct {
		ID string `json:"id"`
	}
	status, raw = ts.Call(ts.CP, http.MethodPost, "/users", map[string]any{
		"type": "person",
		"ouId": packtest.DefaultOUID,
		"attributes": map[string]any{
			"username": username,
			"email":    username + "@example.com",
			"password": password,
		},
	}, &user)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.RemoveOnCleanup("/users/" + user.ID)

	ts.Sync()

	token, err := testutils.PackLoginAs(clientID, loginRedirectURI, "openid", username, password)
	ts.Require().NoError(err, "the person could not sign in on the data plane")
	ts.NotEmpty(token.AccessToken)

	_, err = testutils.PackLoginAs(clientID, loginRedirectURI, "openid", username, "not-"+password)
	ts.Error(err, "the data plane signed the person in with the wrong password")
}
