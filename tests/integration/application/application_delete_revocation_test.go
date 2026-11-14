// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// oauthApp creates an application with the given OAuth client configuration, or with no OAuth client
// when none is given, and returns its id. The caller removes it.
func (ts *ApplicationAPITestSuite) oauthApp(name string, config map[string]interface{}) string {
	ts.T().Helper()
	app := testutils.Application{
		Name:        name,
		Description: "Application created to test revocation on deletion",
		OUID:        testOUID,
		Type:        "fullstack",
	}
	if config != nil {
		app.InboundAuthConfig = []map[string]interface{}{{"type": "oauth2", "config": config}}
	} else {
		// Without it, CreateApplication gives the application a default OAuth client.
		app.Embedded = true
	}
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create the application")
	ts.Require().NotEmpty(appID)
	return appID
}

// clientCredentialsApp creates an application that issues client_credentials tokens and returns its
// id. The caller removes it.
func (ts *ApplicationAPITestSuite) clientCredentialsApp(name, clientID, clientSecret string) string {
	ts.T().Helper()
	return ts.oauthApp(name, map[string]interface{}{
		"clientId":                clientID,
		"clientSecret":            clientSecret,
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"client_credentials"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	})
}

// clientCredentialsAccepted reports whether the client still issues client_credentials tokens.
func (ts *ApplicationAPITestSuite) clientCredentialsAccepted(clientID, clientSecret string) bool {
	ts.T().Helper()
	issued, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, "", "")
	ts.Require().NoError(err, "Failed to call the token endpoint")
	return issued.StatusCode == http.StatusOK
}

// applicationExists reports whether the application record is still served.
func (ts *ApplicationAPITestSuite) applicationExists(appID string) bool {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/applications/"+appID, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Deleting an application through the management API revokes the tokens issued to it, as deleting it
// through the administration flow does: the deletion runs the same revocation before the record goes,
// whichever entry point asked. Introspection reads the deny list directly, so the answer is immediate,
// and it is asked by another application because the deleted one can no longer authenticate.
func (ts *ApplicationAPITestSuite) TestDeleteApplication_RevokesItsTokens() {
	const (
		clientID        = "api-delete-revokes-client"
		clientSecret    = "api-delete-revokes-secret" // #nosec G101 -- test fixture
		observerID      = "api-delete-observer-client"
		observerSecret  = "api-delete-observer-secret" // #nosec G101 -- test fixture
		observerAppName = "api_delete_revocation_observer"
	)
	appID := ts.clientCredentialsApp("api_delete_revocation_app", clientID, clientSecret)
	observerAppID := ts.clientCredentialsApp(observerAppName, observerID, observerSecret)
	defer func() {
		if err := testutils.DeleteApplication(observerAppID); err != nil {
			ts.T().Logf("Failed to delete the observer application: %v", err)
		}
	}()

	issued, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, "", "")
	ts.Require().NoError(err, "Failed to request a token")
	ts.Require().Equal(http.StatusOK, issued.StatusCode, "Token request failed: %s", string(issued.Body))
	ts.Require().NotNil(issued.Token)
	token := issued.Token.AccessToken
	active, err := testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.Require().True(active, "The token should be active before the application is deleted")

	ts.Require().NoError(testutils.DeleteApplication(appID), "Failed to delete the application")

	active, err = testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.False(active, "A token issued to a deleted application was still active")
	ts.False(ts.applicationExists(appID), "The application record should be gone")
}

// The deny-list row a deletion writes is sized from the longest-lived artifact the client can issue, and
// user-subject and client_credentials access tokens carry separate validity settings. This application
// configures the client_credentials one longer, so sizing from the user-subject setting alone would let a
// token it already issued outlive its own revocation. The observable outcome is that the deletion accepts
// the shape: the client authenticates before it and is gone after it.
func (ts *ApplicationAPITestSuite) TestDeleteApplication_DeletesAppWithLongerClientValidity() {
	const (
		clientID     = "api-delete-client-validity-client"
		clientSecret = "api-delete-client-validity-secret" // #nosec G101 -- test fixture
	)
	appID := ts.oauthApp("api_delete_client_validity_app", map[string]interface{}{
		"clientId":                clientID,
		"clientSecret":            clientSecret,
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"client_credentials"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig":   map[string]interface{}{"validityPeriod": 300},
				"clientConfig": map[string]interface{}{"validityPeriod": 7200},
			},
		},
	})
	ts.Require().True(ts.clientCredentialsAccepted(clientID, clientSecret),
		"The client should issue client_credentials tokens before deletion")

	ts.Require().NoError(testutils.DeleteApplication(appID),
		"An application whose client_credentials tokens outlive its user tokens should still be deleted")

	ts.False(ts.applicationExists(appID), "The application record should be gone")
	ts.False(ts.clientCredentialsAccepted(clientID, clientSecret), "The deleted client must stop issuing tokens")
}

// A refresh token outlives both access token settings, so a deletion against a refresh-token capable
// client must size its deny-list row from the refresh validity instead. The application pins both access
// validities to a minute and its refresh validity to a day, so the refresh token is unambiguously the
// longest-lived artifact the client can hold when it is deleted.
func (ts *ApplicationAPITestSuite) TestDeleteApplication_DeletesRefreshTokenCapableApp() {
	appID := ts.oauthApp("api_delete_refresh_app", map[string]interface{}{
		"clientId":                "api-delete-refresh-client",
		"clientSecret":            "api-delete-refresh-secret",
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig":   map[string]interface{}{"validityPeriod": 60},
				"clientConfig": map[string]interface{}{"validityPeriod": 60},
			},
			"refreshToken": map[string]interface{}{"validityPeriod": 86400},
		},
	})
	ts.Require().True(ts.applicationExists(appID), "The application should exist before deletion")

	ts.Require().NoError(testutils.DeleteApplication(appID),
		"A refresh-token capable application should be deleted")

	ts.False(ts.applicationExists(appID), "The application record should be gone")
}

// An application with no OAuth component issues no artifacts, so there is nothing to revoke. The
// deletion must still remove it rather than refusing on the absent criterion.
func (ts *ApplicationAPITestSuite) TestDeleteApplication_DeletesAppWithoutOAuthComponent() {
	appID := ts.oauthApp("api_delete_no_oauth_app", nil)
	ts.Require().True(ts.applicationExists(appID), "The application should exist before deletion")

	ts.Require().NoError(testutils.DeleteApplication(appID),
		"An application with no OAuth component should still be deleted")

	ts.False(ts.applicationExists(appID), "The application record should be gone")
}

// Updating an application with a different client secret revokes the tokens issued before the change,
// while an update that carries the same secret, as every apply of a configuration version does,
// leaves them valid.
func (ts *ApplicationAPITestSuite) TestUpdateApplication_RevokesTokensOnlyWhenTheSecretChanges() {
	const (
		clientID       = "api-update-rotates-client"
		clientSecret   = "api-update-rotates-secret"   // #nosec G101 -- test fixture
		newSecret      = "api-update-rotates-secret-2" // #nosec G101 -- test fixture
		observerID     = "api-update-observer-client"
		observerSecret = "api-update-observer-secret" // #nosec G101 -- test fixture
	)
	appID := ts.clientCredentialsApp("api_update_revocation_app", clientID, clientSecret)
	observerAppID := ts.clientCredentialsApp("api_update_revocation_observer", observerID, observerSecret)
	defer func() {
		for _, id := range []string{appID, observerAppID} {
			if err := testutils.DeleteApplication(id); err != nil {
				ts.T().Logf("Failed to delete application %s: %v", id, err)
			}
		}
	}()
	issued, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, issued.StatusCode, "Token request failed: %s", string(issued.Body))
	token := issued.Token.AccessToken

	update := func(secret string) {
		ts.T().Helper()
		ts.Require().NoError(testutils.UpdateApplication(appID, testutils.Application{
			Name:        "api_update_revocation_app",
			Description: "Application created to test revocation on update",
			OUID:        testOUID,
			Type:        "fullstack",
			InboundAuthConfig: []map[string]interface{}{{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            secret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			}},
		}), "Failed to update the application")
	}

	update(clientSecret)
	active, err := testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.True(active, "An update with the same secret revoked the application's tokens")

	update(newSecret)
	active, err = testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.False(active, "A token issued with the replaced secret was still active")
	fresh, err := testutils.RequestClientCredentialsToken(clientID, newSecret, "", "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, fresh.StatusCode, "The new secret should issue tokens: %s", string(fresh.Body))
}

// A client that authenticated without a secret can be given one together with a method that uses it,
// whatever an earlier configuration left stored: nothing was issued under a secret, so there is nothing
// to revoke, and the update is not refused as a rotation of a secret the client never used.
func (ts *ApplicationAPITestSuite) TestUpdateApplication_AClientThatHadNoSecretIsGivenOne() {
	const (
		clientID    = "api_update_first_secret_client"
		firstSecret = "api-update-first-secret"   // #nosec G101 -- test fixture
		newSecret   = "api-update-first-secret-2" // #nosec G101 -- test fixture
	)
	appID := ts.clientCredentialsApp("api_update_first_secret_app", clientID, firstSecret)
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application %s: %v", appID, err)
		}
	}()
	update := func(config map[string]interface{}) error {
		ts.T().Helper()
		config["clientId"] = clientID
		config["redirectUris"] = []string{"https://localhost:3000"}
		return testutils.UpdateApplication(appID, testutils.Application{
			Name:        "api_update_first_secret_app",
			Description: "Application created to test a first secret on update",
			OUID:        testOUID,
			Type:        "fullstack",
			InboundAuthConfig: []map[string]interface{}{{
				"type": "oauth2", "config": config,
			}},
		})
	}

	// The client stops using its secret; the stored one is left where it is.
	ts.Require().NoError(update(map[string]interface{}{
		"publicClient":            true,
		"pkceRequired":            true,
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "none",
	}), "Failed to make the client public")

	// A new secret with a method that uses it is the client's first, not a rotation.
	ts.Require().NoError(update(map[string]interface{}{
		"clientSecret":            newSecret,
		"grantTypes":              []string{"client_credentials"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}), "A client that had no secret was refused one")
	fresh, err := testutils.RequestClientCredentialsToken(clientID, newSecret, "", "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, fresh.StatusCode, "The new secret should issue tokens: %s", string(fresh.Body))
}
