// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// clientCredentialsApp creates an application that issues client_credentials tokens and returns its
// id. The caller removes it.
func (ts *ApplicationAPITestSuite) clientCredentialsApp(name, clientID, clientSecret string) string {
	ts.T().Helper()
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        name,
		Description: "Application created to test revocation on deletion",
		OUID:        testOUID,
		Type:        "fullstack",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"redirectUris":            []string{"https://localhost:3000"},
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err, "Failed to create the application")
	ts.Require().NotEmpty(appID)
	return appID
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
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/applications/"+appID, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusNotFound, resp.StatusCode, "The application record should be gone")
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
