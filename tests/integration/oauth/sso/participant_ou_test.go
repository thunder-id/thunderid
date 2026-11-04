// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	participantOUClientID    = "sso_participant_ou_client"
	participantOUChildHandle = "sso-participant-child"
	participantOUUser        = "sso_participant_ou_user"
)

// TestParticipants_AnApplicationIsAParticipantPerOU verifies that an SSO session records an application once for
// each organization unit it signed in under, each with the token family of its own grant, that signing out
// revokes the tokens of every one of them, and that such an application gets a single back-channel logout.
func (ts *SSOLogoutTestSuite) TestParticipants_AnApplicationIsAParticipantPerOU() {
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: participantOUChildHandle, Name: participantOUChildHandle, Parent: &testOUID,
	})
	ts.Require().NoError(err, "create the child OU")
	defer ts.deleteOUExchangeFixture("OU", childOUID, testutils.DeleteOrganizationUnit)

	userID := ts.createOUExchangeUser(participantOUUser)
	defer ts.deleteOUExchangeFixture("user", userID, testutils.DeleteUser)
	ts.shareUserInto(userID, childOUID)

	receiver := ts.newLogoutReceiver(http.StatusOK)
	appID := ts.createParticipantOUApp(receiver.srv.URL + "/bcl")
	defer ts.deleteAppByID(appID)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share the exchanging app")

	// The suite's app signs the user in under its own OU; the second app joins under the child and the parent.
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "participant_ou_state")
	step, collected := ts.runCredentialLogin(client, executionID, participantOUUser)
	first := ts.completeAndExchange(client, authID, step.Assertion, collected)
	ts.Require().NotEmpty(first.DeviceSecret, "the sign-in should return a device secret")
	underChild := ts.exchangeForParticipant("/ou/"+childOUID+"/oauth2/token", first.DeviceSecret)
	underParent := ts.exchangeForParticipant("/ou/"+testOUID+"/oauth2/token", first.DeviceSecret)

	sessionID := ts.idTokenClaim(first.IDToken, "sid")
	ts.Equal(sessionID, ts.idTokenClaim(underChild["id_token"].(string), "sid"), "both apps share one session")
	accessTokens := []string{first.AccessToken, underChild["access_token"].(string),
		underParent["access_token"].(string)}
	ts.ElementsMatch([]string{
		participantRow(ts.applicationID, testOUID, ts.tokenFamily(accessTokens[0])),
		participantRow(appID, childOUID, ts.tokenFamily(accessTokens[1])),
		participantRow(appID, testOUID, ts.tokenFamily(accessTokens[2])),
	}, ts.participantRows(sessionID), "one participant per application and OU, each with its own token family")
	ts.NotEqual(ts.tokenFamily(accessTokens[1]), ts.tokenFamily(accessTokens[2]),
		"the second app's two OUs are two token families")
	for _, accessToken := range accessTokens {
		ts.Require().True(ts.introspectActive(client, accessToken), "every grant is active before sign-out")
	}

	ts.signOut(client, first.IDToken, "participant_ou_signout_state")

	for i, accessToken := range accessTokens {
		ts.False(ts.introspectActive(client, accessToken), "sign-out revokes grant %d", i)
	}
	ts.Empty(ts.participantRows(sessionID), "sign-out removes every participant")
	got := receiver.await(ts, 1)
	ts.assertLogoutToken(got[0], participantOUClientID, underChild["id_token"].(string))
	receiver.quiet(ts, time.Second)
}

// createParticipantOUApp creates an app on the suite's login flow that exchanges device secrets and registers
// a back-channel logout URI.
func (ts *SSOLogoutTestSuite) createParticipantOUApp(logoutURI string) string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "SSOParticipantOUApp",
		Description:      "Application that joins an SSO session under several organization units",
		OUID:             testOUID,
		AuthFlowID:       ts.authFlowID,
		AllowedUserTypes: []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                participantOUClientID,
			"clientSecret":            clientSecret,
			"redirectUris":            []string{redirectURI},
			"backchannelLogoutUri":    logoutURI,
			"grantTypes":              []string{"authorization_code", tokenExchangeGrant},
			"responseTypes":           []string{"code"},
			"scopes":                  []string{"openid"},
			"tokenEndpointAuthMethod": "client_secret_basic",
		}}},
	})
	ts.Require().NoError(err, "Failed to create the participant OU application")
	return appID
}

// exchangeForParticipant exchanges the device secret at the token endpoint at path as the participant OU app.
func (ts *SSOLogoutTestSuite) exchangeForParticipant(path, deviceSecret string) map[string]interface{} {
	status, token := ts.exchangeDeviceSecretAt(path, participantOUClientID, clientSecret, deviceSecret, "openid")
	ts.Require().Equal(http.StatusOK, status, "the exchange at %s should succeed: %v", path, token)
	return token
}

// tokenFamily returns the token family a token belongs to.
func (ts *SSOLogoutTestSuite) tokenFamily(token string) string {
	claims, err := testutils.DecodeJWT(token)
	ts.Require().NoError(err, "decode the token")
	tfid, _ := claims.Additional["tfid"].(string)
	ts.Require().NotEmpty(tfid, "the token should carry a token family")
	return tfid
}

// participantRows returns the session's participants as rows of participantRow.
func (ts *SSOLogoutTestSuite) participantRows(sessionID string) []string {
	out, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT APP_ID || '|' || OU_ID || '|' || COALESCE(TFID, '') FROM "SSO_SESSION_PARTICIPANT" `+
			`WHERE SESSION_ID = '%s';`, sessionID))
	ts.Require().NoError(err, "read the session's participants")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// participantRow is one participant as participantRows reads it.
func participantRow(appID, ouID, tokenFamilyID string) string {
	return appID + "|" + ouID + "|" + tokenFamilyID
}
