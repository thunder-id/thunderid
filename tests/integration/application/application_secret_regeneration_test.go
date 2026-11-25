// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The declarative confidential application loaded from
// tests/integration/resources/declarative_resources/applications/app-declarative-confidential.yaml before
// the suites run. It is confidential on purpose: a declarative public client would be refused for having
// no secret, which would hide whether the declarative refusal itself is reached.
const (
	regenDeclarativeAppID       = "decl-app-confidential-1"
	regenDeclarativeAppClientID = "decl-conf-client-1"
	regenDeclarativeAppSecret   = "decl-conf-secret-1" // #nosec G101 -- declarative test fixture value
)

// The refusals a regeneration answers with.
const (
	errCodeAppNotFound            = "APP-1001"
	errCodeAppHasNoClientSecret   = "APP-1047"
	errCodeAppDeclarativeReadOnly = "APP-1030"
)

// regenerateSecretResult is what a regeneration request answered.
type regenerateSecretResult struct {
	status       int
	cacheControl string
	clientSecret string
	errorCode    string
	body         string
}

// regenerateSecret asks for a new client secret for the application with client, as the admin unless a
// client is given.
func (ts *ApplicationAPITestSuite) regenerateSecret(client *http.Client, appID string) regenerateSecretResult {
	ts.T().Helper()
	if client == nil {
		client = testutils.GetHTTPClient()
	}
	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+"/applications/"+appID+"/client-secret/regenerate", nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "Failed to call the regeneration endpoint")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	result := regenerateSecretResult{status: resp.StatusCode, cacheControl: resp.Header.Get("Cache-Control"),
		body: string(body)}
	var decoded struct {
		ClientSecret string `json:"clientSecret"`
		Code         string `json:"code"`
	}
	_ = json.Unmarshal(body, &decoded)
	result.clientSecret = decoded.ClientSecret
	result.errorCode = decoded.Code
	return result
}

// assertRegenerationRefused asserts a regeneration was refused with status and code, and returned no secret.
func (ts *ApplicationAPITestSuite) assertRegenerationRefused(result regenerateSecretResult, status int,
	code, msg string) {
	ts.T().Helper()
	ts.Equal(status, result.status, "%s: %s", msg, result.body)
	ts.Equal(code, result.errorCode, "%s: the refusal should name its own reason: %s", msg, result.body)
	ts.Empty(result.clientSecret, "%s: a refused regeneration must not return a secret", msg)
}

// A regeneration returns the new secret once, and the change is real: the old secret stops issuing
// tokens, the new one starts, and a token the old secret issued is revoked, as for an update that
// replaces the secret.
func (ts *ApplicationAPITestSuite) TestRegenerateClientSecret_ReplacesTheSecretAndRevokes() {
	const (
		clientID       = "api-regenerate-client"
		oldSecret      = "api-regenerate-secret" // #nosec G101 -- test fixture
		observerID     = "api-regenerate-observer-client"
		observerSecret = "api-regenerate-observer-secret" // #nosec G101 -- test fixture
	)
	appID := ts.clientCredentialsApp("api_regenerate_app", clientID, oldSecret)
	observerAppID := ts.clientCredentialsApp("api_regenerate_observer", observerID, observerSecret)
	defer func() {
		for _, id := range []string{appID, observerAppID} {
			if err := testutils.DeleteApplication(id); err != nil {
				ts.T().Logf("Failed to delete application %s: %v", id, err)
			}
		}
	}()
	issued, err := testutils.RequestClientCredentialsToken(clientID, oldSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, issued.StatusCode, "Token request failed: %s", string(issued.Body))
	token := issued.Token.AccessToken
	active, err := testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.Require().True(active, "The token should be active before the secret is regenerated")

	result := ts.regenerateSecret(nil, appID)

	ts.Require().Equal(http.StatusOK, result.status, "Regeneration failed: %s", result.body)
	ts.Equal("no-store", result.cacheControl, "A response that carries a secret must not be cached")
	ts.Require().NotEmpty(result.clientSecret, "The response should carry the new secret: %s", result.body)
	ts.NotEqual(oldSecret, result.clientSecret, "The returned secret should be a new value")
	refused, err := testutils.RequestClientCredentialsToken(clientID, oldSecret, "", "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, refused.StatusCode, "The old secret was not rejected: %s", string(refused.Body))
	ts.Contains(string(refused.Body), `"invalid_client"`, "The old secret was not rejected as an invalid client")
	ts.True(ts.clientCredentialsAccepted(clientID, result.clientSecret), "The new secret did not issue tokens")
	active, err = testutils.IntrospectToken(token, observerID, observerSecret)
	ts.Require().NoError(err)
	ts.False(active, "A token issued with the regenerated secret was still active")
}

// A public client authenticates without a secret, so it has none to regenerate.
func (ts *ApplicationAPITestSuite) TestRegenerateClientSecret_RefusesPublicClient() {
	appID := ts.oauthApp("api_regenerate_public", map[string]interface{}{
		"clientId":                "api-regenerate-public-client",
		"redirectUris":            []string{"https://localhost:3000"},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "none",
		"publicClient":            true,
		"pkceRequired":            true,
	})
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application %s: %v", appID, err)
		}
	}()

	ts.assertRegenerationRefused(ts.regenerateSecret(nil, appID), http.StatusBadRequest,
		errCodeAppHasNoClientSecret, "Regenerating the secret of a public client")
}

// An application with no OAuth client has no secret to regenerate.
func (ts *ApplicationAPITestSuite) TestRegenerateClientSecret_RefusesAppWithoutOAuthComponent() {
	appID := ts.oauthApp("api_regenerate_no_oauth", nil)
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application %s: %v", appID, err)
		}
	}()

	ts.assertRegenerationRefused(ts.regenerateSecret(nil, appID), http.StatusBadRequest,
		errCodeAppHasNoClientSecret, "Regenerating the secret of an application with no OAuth client")
}

// An application that does not exist is answered as not found.
func (ts *ApplicationAPITestSuite) TestRegenerateClientSecret_UnknownApplication() {
	ts.assertRegenerationRefused(ts.regenerateSecret(nil, "01900000-0000-7000-8000-0000000000fd"),
		http.StatusNotFound, errCodeAppNotFound, "Regenerating the secret of an unknown application")
}

// A declarative application is read only: its secret is not regenerated, and the one its file declares
// keeps working.
func (ts *ApplicationAPITestSuite) TestRegenerateClientSecret_RefusesDeclarativeApplication() {
	ts.Require().True(ts.clientCredentialsAccepted(regenDeclarativeAppClientID, regenDeclarativeAppSecret),
		"The declarative client's secret should issue tokens before the refused regeneration")

	ts.assertRegenerationRefused(ts.regenerateSecret(nil, regenDeclarativeAppID), http.StatusBadRequest,
		errCodeAppDeclarativeReadOnly, "Regenerating the secret of a declarative application")
	ts.True(ts.clientCredentialsAccepted(regenDeclarativeAppClientID, regenDeclarativeAppSecret),
		"A refused regeneration must leave the declarative client's secret usable")
}

// A caller without a token is refused, and the secret stays as it was.
func (ts *ApplicationAPITestSuite) TestRegenerateClientSecret_RejectsAnonymousCaller() {
	const (
		clientID = "api-regenerate-anon-client"
		secret   = "api-regenerate-anon-secret" // #nosec G101 -- test fixture
	)
	appID := ts.clientCredentialsApp("api_regenerate_anon_app", clientID, secret)
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application %s: %v", appID, err)
		}
	}()

	result := ts.regenerateSecret(testutils.GetRawHTTPClient(), appID)

	ts.Equal(http.StatusUnauthorized, result.status, "An anonymous regeneration should be unauthorized: %s",
		result.body)
	ts.Empty(result.clientSecret)
	ts.True(ts.clientCredentialsAccepted(clientID, secret), "The secret changed for an anonymous caller")
}
