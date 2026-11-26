// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"fmt"
	"maps"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const trackingOffUsername = "sso_tracking_off_user"

// restartWithSessionTracking restarts the server with session.tracking.enabled set, and restores the session
// section with another restart when the test ends.
func (ts *SSOLogoutTestSuite) restartWithSessionTracking(enabled bool) {
	ts.T().Helper()

	original, err := testutils.ReadDeploymentConfigKey("session")
	ts.Require().NoError(err, "failed to read the session config")
	current, _ := original.(map[string]interface{})
	ts.T().Cleanup(func() {
		if err := restartWithSessionSection(current); err != nil {
			ts.T().Logf("cleanup: failed to restore the session config: %v", err)
		}
	})

	section := map[string]interface{}{}
	maps.Copy(section, current)
	section["tracking"] = map[string]interface{}{"enabled": enabled}
	ts.Require().NoError(restartWithSessionSection(section), "failed to restart with session tracking %t", enabled)
}

// restartWithSessionSection writes the session section of deployment.yaml, restarts the server, and re-obtains
// the admin token.
func restartWithSessionSection(section map[string]interface{}) error {
	if err := testutils.PatchDeploymentConfig(map[string]interface{}{"session": section}); err != nil {
		return err
	}
	if err := testutils.RestartServer(); err != nil {
		return err
	}
	return testutils.ObtainAdminAccessToken()
}

// sessionRow returns the type and state of the session the sid names, as TYPE|STATE.
func (ts *SSOLogoutTestSuite) sessionRow(sid string) string {
	ts.T().Helper()

	row, err := testutils.QueryRuntimeDB(fmt.Sprintf(`SELECT TYPE, STATE FROM "SESSION" WHERE SESSION_ID = '%s';`,
		sid))
	ts.Require().NoError(err, "failed to read the session")
	return row
}

// TestSessionTracking_OffRecordsNoDirectSessionAndKeepsSSO verifies that with session tracking off, a login
// through a flow without a Session node records no session and its tokens carry no sid, while sessions recorded
// earlier keep working and a flow with a Session node still records and reuses its SSO session.
func (ts *SSOLogoutTestSuite) TestSessionTracking_OffRecordsNoDirectSessionAndKeepsSSO() {
	userID := ts.createUser(trackingOffUsername)
	app := ts.createDirectSessionApp("tracking_off", "")
	before := ts.loginTokensAsClient(ts.newSessionClient(), app.clientID, app.clientSecret, trackingOffUsername,
		"tracking_off_1")
	directSid, has := ts.sidClaim(before.IDToken)
	ts.Require().True(has, "a login before the switch must record a direct session")
	ssoClient := ts.newSessionClient()
	ts.login(ssoClient, trackingOffUsername, "tracking_off_2")

	ts.restartWithSessionTracking(false)

	// Sessions recorded before the switch keep working.
	ts.Equal("DIRECT|ACTIVE", ts.sessionRow(directSid))
	ts.True(ts.sessionSurvives(ssoClient, "tracking_off_3"), "an SSO session from before the switch must be reused")

	// A login through a flow without a Session node records nothing.
	tokens := ts.loginTokensAsClient(ts.newSessionClient(), app.clientID, app.clientSecret, trackingOffUsername,
		"tracking_off_4")
	_, has = ts.sidClaim(tokens.IDToken)
	ts.False(has, "a login without a Session node must carry no sid")
	sessions, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "SESSION" WHERE SUBJECT_ID = '%s' AND TYPE = 'DIRECT';`, userID))
	ts.Require().NoError(err, "failed to count the direct sessions")
	ts.Equal("1", sessions, "only the direct session from before the switch must exist")
	participants, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "SESSION_PARTICIPANT" WHERE TFID = '%s';`, ts.tokenFamily(tokens.AccessToken)))
	ts.Require().NoError(err, "failed to count the grant's participants")
	ts.Equal("0", participants, "the grant must have no participant")

	// A flow with a Session node still records its SSO session, and a later login reuses it.
	client := ts.newSessionClient()
	ssoSid, has := ts.sidClaim(ts.login(client, trackingOffUsername, "tracking_off_5"))
	ts.Require().True(has, "a login through a Session node must name its SSO session")
	ts.Equal("SSO|ACTIVE", ts.sessionRow(ssoSid))
	ts.True(ts.sessionSurvives(client, "tracking_off_6"), "a later login must reuse the SSO session")
}
