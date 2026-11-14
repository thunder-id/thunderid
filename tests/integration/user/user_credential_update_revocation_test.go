// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// credentialRevocationUser is a user signed in through an application in the suite's organization unit,
// with an observer client that introspects the user's tokens. Introspection reads the deny list
// directly, so the answer is immediate.
type credentialRevocationUser struct {
	id             string
	username       string
	signInID       string
	observerID     string
	observerSecret string
}

// newCredentialRevocationUser creates a user with the given password, an application to sign them in and
// an observer client. Everything it creates is removed when the test ends.
func (ts *UserAPITestSuite) newCredentialRevocationUser(name, password string) *credentialRevocationUser {
	u := &credentialRevocationUser{
		username:       "api." + name + ".user",
		signInID:       name + "-sign-in-client",
		observerID:     name + "-observer-client",
		observerSecret: name + "-observer-secret",
	}
	attrs, err := json.Marshal(map[string]interface{}{
		"username": u.username,
		"email":    u.username + "@example.com",
		"password": password,
	})
	ts.Require().NoError(err)
	u.id, err = testutils.CreateUser(testutils.User{
		OUID:       testOUID,
		Type:       "admin-cred-update-type",
		Attributes: attrs,
	})
	ts.Require().NoError(err, "Failed to create the user")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteUser(u.id); err != nil {
			ts.T().Logf("Failed to delete the user: %v", err)
		}
	})

	observerAppID, err := testutils.CreateApplication(testutils.Application{
		Name:        name + "_observer",
		Description: "Application that introspects tokens for the credential update tests",
		OUID:        testOUID,
		Type:        "fullstack",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                u.observerID,
				"clientSecret":            u.observerSecret,
				"redirectUris":            []string{"https://localhost:3000"},
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err, "Failed to create the observer application")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(observerAppID); err != nil {
			ts.T().Logf("Failed to delete the observer application: %v", err)
		}
	})

	signInAppID, err := testutils.CreateOUSignInApplication(testOUID, u.signInID)
	ts.Require().NoError(err, "Failed to create the sign-in application")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(signInAppID); err != nil {
			ts.T().Logf("Failed to delete the sign-in application: %v", err)
		}
	})
	return u
}

// signIn signs the user in with the password and returns the access token.
func (ts *UserAPITestSuite) signIn(u *credentialRevocationUser, password string) string {
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(u.signInID, testutils.OUSignInRedirectURI,
		"openid", u.username, password, true, "")
	ts.Require().NoError(err, "Failed to sign the user in")
	return tokenResp.AccessToken
}

// isActive reports whether introspection finds the token active.
func (ts *UserAPITestSuite) isActive(u *credentialRevocationUser, token string) bool {
	active, err := testutils.IntrospectToken(token, u.observerID, u.observerSecret)
	ts.Require().NoError(err)
	return active
}

// Setting a user's password through the management API revokes the tokens issued to them before the
// change, and a token issued after it is valid.
func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_RevokesTheirTokens() {
	const (
		password    = "BeforeAdminReset@123" // #nosec G101 -- test fixture
		newPassword = "AfterAdminReset@123"  // #nosec G101 -- test fixture
	)
	u := ts.newCredentialRevocationUser("admin-cred-revokes", password)
	token := ts.signIn(u, password)
	ts.Require().True(ts.isActive(u, token), "The token should be active before the password changes")

	ts.Require().NoError(testutils.UpdateUserCredentials(u.id, map[string]string{"password": newPassword}))

	ts.False(ts.isActive(u, token), "A token issued before the password change was still active")

	// A token's iat has second granularity while the cutoff is sub-second, and the comparison is at or before.
	time.Sleep(1100 * time.Millisecond)
	ts.True(ts.isActive(u, ts.signIn(u, newPassword)), "A token issued after the password change was revoked")
}

// A user changing their own password revokes the tokens issued to them before the change, including
// the one that made the request.
func (ts *UserAPITestSuite) TestCredentialUpdateBySelf_RevokesTheirTokens() {
	const (
		password    = "BeforeSelfChange@123" // #nosec G101 -- test fixture
		newPassword = "AfterSelfChange@123"  // #nosec G101 -- test fixture
	)
	u := ts.newCredentialRevocationUser("self-cred-revokes", password)
	otherToken := ts.signIn(u, password)
	token := ts.signIn(u, password)

	payload, err := json.Marshal(map[string]string{
		"name": "password", "currentValue": password, "newValue": newPassword,
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/users/me/update-credentials",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	userClient := testutils.NewHTTPClientWithTokenProvider(func() (string, error) { return token, nil })
	resp, err := userClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusNoContent, resp.StatusCode)

	ts.False(ts.isActive(u, token), "The token that changed the password was still active")
	ts.False(ts.isActive(u, otherToken), "Another token issued before the password change was still active")
}

// A credential the user type refuses changes nothing, so nothing is revoked.
func (ts *UserAPITestSuite) TestCredentialUpdateByAdmin_RefusedValueRevokesNothing() {
	const password = "BeforeRefusedReset@123" // #nosec G101 -- test fixture
	u := ts.newCredentialRevocationUser("admin-cred-refused", password)
	token := ts.signIn(u, password)

	err := testutils.UpdateUserCredentials(u.id, map[string]string{"password": "short"})
	ts.Require().Error(err, "A password shorter than the user type allows was accepted")
	err = testutils.UpdateUserCredentials(u.id, map[string]string{"pin": "1234"})
	ts.Require().Error(err, "A credential the user type does not declare was accepted")

	ts.True(ts.isActive(u, token), "A refused credential change revoked the user's token")
}
