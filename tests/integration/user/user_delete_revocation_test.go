// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Deleting a user through the management API revokes the tokens issued to them, as deleting them
// through the administration flow does: the deletion runs the same revocation before the record goes.
// Introspection reads the deny list directly, so the answer is immediate.
func (ts *UserAPITestSuite) TestDeleteUser_RevokesTheirTokens() {
	const (
		username       = "api.delete.revokes.user"
		password       = "DeleteRevokes@123" // #nosec G101 -- test fixture
		observerID     = "user-delete-observer-client"
		observerSecret = "user-delete-observer-secret" // #nosec G101 -- test fixture
	)
	attrs, err := json.Marshal(map[string]interface{}{
		"username": username,
		"email":    "api.delete.revokes.user@example.com",
		"password": password,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       testOUID,
		Type:       "admin-cred-update-type",
		Attributes: attrs,
	})
	ts.Require().NoError(err, "Failed to create the user")

	observerAppID, err := testutils.CreateApplication(testutils.Application{
		Name:        "user_delete_revocation_observer",
		Description: "Application that introspects tokens for the user deletion test",
		OUID:        testOUID,
		Type:        "fullstack",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                observerID,
				"clientSecret":            observerSecret,
				"redirectUris":            []string{"https://localhost:3000"},
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err, "Failed to create the observer application")
	defer func() {
		if err := testutils.DeleteApplication(observerAppID); err != nil {
			ts.T().Logf("Failed to delete the observer application: %v", err)
		}
	}()

	tokenResp, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", "https://localhost:8095/console",
		"openid", username, password, true)
	ts.Require().NoError(err, "Failed to sign the user in")
	token := tokenResp.AccessToken
	active, err := testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.Require().True(active, "The token should be active before the user is deleted")

	ts.Require().NoError(testutils.DeleteUser(userID), "Failed to delete the user")

	active, err = testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.False(active, "A token issued to a deleted user was still active")
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/users/"+userID, nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusNotFound, resp.StatusCode, "The user record should be gone")
}
