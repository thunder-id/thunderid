// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// regenerate_application_secret replaces the application's secret and revokes the tokens the old one issued,
// and its notice says so. The application introspects with its new secret, since the old one no longer works.
func (ts *MCPTestSuite) TestApplicationTools_RegenerateSecretRevokesItsTokens() {
	token := ts.mcpScopedAdminToken()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	clientID, secret := "mcp-regen-client-"+suffix, "mcp-regen-secret-"+suffix
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "mcp_regen_app_" + suffix,
		OUID: ts.ouID,
		Type: "fullstack",
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                clientID,
			"clientSecret":            secret,
			"redirectUris":            []string{"https://localhost:3000"},
			"grantTypes":              []string{"client_credentials"},
			"tokenEndpointAuthMethod": "client_secret_basic",
		}}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	issued, err := testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, issued.StatusCode, "token request failed: %s", issued.Body)
	oldToken := issued.Token.AccessToken

	regenerated, raw := ts.callMCPTool(token, "regenerate_application_secret", map[string]any{"id": appID})
	ts.Require().Falsef(regenerated.Result.IsError, "regenerate_application_secret failed: %s", raw)
	newSecret, _ := regenerated.Result.StructuredContent["clientSecret"].(string)
	ts.Require().NotEmpty(newSecret)
	ts.NotEqual(secret, newSecret)
	ts.Nil(regenerated.Result.StructuredContent["revocationIncomplete"], "the revocation must be complete")
	notice, _ := regenerated.Result.StructuredContent["notice"].(string)
	ts.Contains(notice, "the tokens it issued are revoked")

	active, err := testutils.IntrospectToken(oldToken, clientID, newSecret)
	ts.Require().NoError(err)
	ts.False(active, "a token the old secret issued was still active")
	issued, err = testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, issued.StatusCode, "the old secret must stop working")
	// A token's iat has second granularity while the cutoff is sub-second, and the comparison is at or before.
	time.Sleep(1100 * time.Millisecond)
	issued, err = testutils.RequestClientCredentialsToken(clientID, newSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, issued.StatusCode, "the new secret must work: %s", issued.Body)
	active, err = testutils.IntrospectToken(issued.Token.AccessToken, clientID, newSecret)
	ts.Require().NoError(err)
	ts.True(active, "a token the new secret issued was not active")
}
