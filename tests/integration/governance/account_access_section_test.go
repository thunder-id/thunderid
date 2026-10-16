// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AccountAccessSectionTestSuite checks the lockout policy as the Console edits it, through the
// `accountAccess` server-config section over deployment.yaml defaults: writes apply without a
// restart, the section overlays the defaults, and a misspelled key is refused.
type AccountAccessSectionTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userIDs      []string
	suffix       string
}

func TestAccountAccessSectionTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessSectionTestSuite))
}

var policySectionOU = testutils.OrganizationUnit{
	Handle:      "governance_policy_section_ou",
	Name:        "Governance Policy Section Test OU",
	Description: "Organization unit for the server-config account-access section tests",
}

var policySectionUserType = testutils.UserType{
	Handle:      "governance_policy_section_user",
	DisplayName: "governance_policy_section_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

// sectionThreshold is lower than the deployment's lockThreshold, so an account locked at this count
// was locked by the section.
const sectionThreshold = 2

func (ts *AccountAccessSectionTestSuite) SetupSuite() {
	applyDeploymentLockoutPolicy(ts.T())
	ts.clearSection(true)

	ouID, err := testutils.CreateOrganizationUnit(policySectionOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	policySectionUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(policySectionUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
}

// SetupTest empties the writable layer, which is stored and outlives a restart, so one test's policy
// cannot decide the next one's outcome.
func (ts *AccountAccessSectionTestSuite) SetupTest() {
	ts.clearSection(true)
}

func (ts *AccountAccessSectionTestSuite) TearDownSuite() {
	ts.clearSection(false)

	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// The section is served, and a threshold written through it takes effect without a restart.
func (ts *AccountAccessSectionTestSuite) TestAThresholdWrittenThroughTheSectionTakesEffectAtOnce() {
	ts.Require().Contains(ts.sectionNames(), accountAccessSection,
		"the account-lock policy must be one of the sections an operator can edit")

	userID, username, _ := ts.newUser("section_threshold")

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		userThresholdSection(sectionThreshold)), "the section must accept a threshold")

	failAuthentication(ts.T(), "username", username, sectionThreshold)

	ts.Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the written threshold must be in force on the very next attempt, with no restart")
}

// The writable section keeps fields supplied by the deployment defaults. Checked at the read and
// at the behaviour, since a threshold with no window is treated as disabled.
func (ts *AccountAccessSectionTestSuite) TestTheSectionOverlaysTheDeploymentRatherThanReplacingIt() {
	userID, username, password := ts.newUser("section_overlay")

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		userThresholdSection(sectionThreshold)), "the section must accept a threshold")

	merged := readMergedUserDefault(ts.T())
	ts.Equal(sectionThreshold, merged.Threshold, "the written layer must win where it speaks")
	ts.Equal(failureWindowSecs, merged.FailureWindowSeconds,
		"a window the write did not mention must still come from the deployment defaults")
	ts.Equal([]int{firstRungSeconds, secondRungSeconds}, merged.LockDurationsSeconds,
		"the ladder the write did not mention must still come from the deployment defaults")

	failAuthentication(ts.T(), "username", username, sectionThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"a threshold with no window locks nobody, so a lock here is the merge working")

	// The deployment defaults supply the finite rung.
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the correct secret must be refused while the lock stands")
}

// A misspelled key is refused, and nothing is persisted, unlike other server-config sections.
func (ts *AccountAccessSectionTestSuite) TestAMisspelledSettingIsRefusedAndPersistsNothing() {
	body := []byte(`{"user":{"default":{"lockThreshold":2}}}`)

	status, _ := putAccountAccessSection(ts.T(), body)
	ts.Equal(http.StatusBadRequest, status,
		"a key the contract does not define must be refused rather than ignored")

	writable, err := testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err)
	ts.NotContains(string(writable), "lockThreshold",
		"a refused write must not have persisted")

	userID, username, _ := ts.newUser("section_misspelled")
	failAuthentication(ts.T(), "username", username, sectionThreshold)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"the deployment threshold must still be the one in force")
}

// Settings the section no longer defines are refused like any unknown key: a per-method `scopes`
// block and a disclosure switch, since every method uses the category default and a hold is always
// stated.
func (ts *AccountAccessSectionTestSuite) TestRemovedSettingsAreRefused() {
	for name, body := range map[string]string{
		"per-method scopes": `{"user":{"scopes":{"otp":{"threshold":2}}}}`,
		"disclosure switch": `{"user":{"default":{"discloseAccountHold":"always"}}}`,
	} {
		status, response := putAccountAccessSection(ts.T(), []byte(body))
		ts.Equal(http.StatusBadRequest, status, "%s must be refused: %s", name, response)
	}

	writable, err := testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err)
	ts.NotContains(string(writable), "scopes", "a refused write must not have persisted")
	ts.NotContains(string(writable), "discloseAccountHold", "a refused write must not have persisted")
}

// Invalid values are refused and the stored section is left as it was: an application block
// (applications never lock), a negative window, and a duration past the largest whole second a
// duration can carry.
func (ts *AccountAccessSectionTestSuite) TestInvalidValuesAreRefusedAndChangeNothing() {
	stored := userThresholdSection(sectionThreshold)
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, stored),
		"the section must accept a valid threshold")
	before, err := testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err)

	for name, body := range map[string]string{
		"application block": `{"application":{"default":{"enabled":true,"threshold":3}}}`,
		"negative window":   `{"user":{"default":{"failureWindowSeconds":-1}}}`,
		"negative duration": `{"user":{"default":{"lockDurationsSeconds":[-5]}}}`,
		"duration past the bound": fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`,
			sectionMaxDurationSeconds+1),
		"window past the bound": fmt.Sprintf(`{"user":{"default":{"failureWindowSeconds":%d}}}`,
			sectionMaxDurationSeconds+1),
	} {
		status, response := putAccountAccessSection(ts.T(), []byte(body))
		ts.Equal(http.StatusBadRequest, status, "%s must be refused: %s", name, response)
		ts.Contains(response, "SCF-1003", "%s must be refused as an invalid value", name)

		after, err := testutils.GetWritableServerConfig(accountAccessSection)
		ts.Require().NoError(err)
		ts.JSONEq(string(before), string(after), "a refused %s must leave the section unchanged", name)
	}
	ts.Equal(sectionThreshold, readMergedUserDefault(ts.T()).Threshold,
		"the valid threshold must still be in force")

	// The largest accepted value is the bound itself.
	ts.NoError(testutils.PutWritableServerConfig(accountAccessSection, []byte(fmt.Sprintf(
		`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, sectionMaxDurationSeconds))),
		"a duration at the bound must be accepted")
}

// A window of zero never resets the failure count on time. Under a one-second window the pause
// restarts the count; under a zero window the second failure locks.
func (ts *AccountAccessSectionTestSuite) TestAZeroWindowKeepsCountingFailuresAcrossAPause() {
	const pause = 3 * time.Second

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"default":{"threshold":%d,"failureWindowSeconds":1}}}`, sectionThreshold))),
		"the section must accept a one-second window")
	resetID, resetName, _ := ts.newUser("section_window_reset")
	failAuthentication(ts.T(), "username", resetName, 1)
	time.Sleep(pause)
	failAuthentication(ts.T(), "username", resetName, 1)
	ts.Equal("ACTIVE", readStatus(ts.T(), resetID).Value,
		"a failure older than the window must restart the count")

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"default":{"threshold":%d,"failureWindowSeconds":0}}}`, sectionThreshold))),
		"the section must accept a zero window")
	ts.Equal(0, readMergedUserDefault(ts.T()).FailureWindowSeconds, "the zero must be kept, not dropped")
	keptID, keptName, _ := ts.newUser("section_window_never")
	failAuthentication(ts.T(), "username", keptName, 1)
	time.Sleep(pause)
	failAuthentication(ts.T(), "username", keptName, 1)
	ts.Equal("LOCKED", readStatus(ts.T(), keptID).Value,
		"under a zero window the earlier failure must still count")
}

// AccountAccessDeclarativeLockTestSuite checks that a declarative accountAccess document wins over
// the deployment defaults and locks the section, so a write cannot be a silent no-op.
type AccountAccessDeclarativeLockTestSuite struct {
	suite.Suite
}

func TestAccountAccessDeclarativeLockTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessDeclarativeLockTestSuite))
}

func (ts *AccountAccessDeclarativeLockTestSuite) SetupSuite() {
	applyDeclarativePolicy(ts.T(), accountAccessPatch()["accountAccess"])
}

// The declarative document is in force over default.json, and a write is refused.
func (ts *AccountAccessDeclarativeLockTestSuite) TestADeclarativeDocumentLocksTheSection() {
	merged := readMergedUserDefault(ts.T())
	ts.Equal(lockThreshold, merged.Threshold, "the declarative threshold must win over default.json")

	status, body := putAccountAccessSection(ts.T(), userThresholdSection(sectionThreshold))
	ts.Equal(http.StatusBadRequest, status, "a write over a declarative document must be refused")
	ts.Contains(body, "SCF-1003")
	ts.Equal(lockThreshold, readMergedUserDefault(ts.T()).Threshold, "a refused write changes nothing")
}

// A declarative document with an application block stops the server from starting, where the same
// document through the section API is refused: an application never locks, so a policy for one is
// a configuration error rather than something to ignore.
func (ts *AccountAccessDeclarativeLockTestSuite) TestADeclarativeApplicationBlockStopsTheServerStarting() {
	// Back to the valid document and a running server, even if an assertion below fails.
	defer func() {
		ts.Assert().NoError(testutils.UpdateAccountAccessConfig(accountAccessPatch()["accountAccess"]))
		ts.Assert().NoError(testutils.RestartServer(), "the server must start again with a valid document")
		ts.Assert().NoError(testutils.ObtainAdminAccessToken())
	}()

	ts.Require().NoError(testutils.UpdateAccountAccessConfig(map[string]interface{}{
		"application": map[string]interface{}{
			"default": map[string]interface{}{"enabled": true, "threshold": 3},
		},
	}))
	ts.NoError(testutils.StartServerExpectingExit(2*time.Minute),
		"a declarative document with an application block must stop the server starting")
}

// --- fixtures and helpers ---

const accountAccessSection = "accountAccess"

// sectionMaxDurationSeconds is the largest seconds value the section accepts: the largest whole second
// a duration can carry.
const sectionMaxDurationSeconds = int64(math.MaxInt64 / int64(time.Second))

// sectionPolicy is the part of a resolved policy this suite makes claims about.
type sectionPolicy struct {
	Threshold            int   `json:"threshold"`
	FailureWindowSeconds int   `json:"failureWindowSeconds"`
	LockDurationsSeconds []int `json:"lockDurationsSeconds"`
}

// userThresholdSection is a writable layer that states a user threshold and nothing else, the
// smallest edit the Console's account-lock card can make.
func userThresholdSection(threshold int) []byte {
	return []byte(fmt.Sprintf(`{"user":{"default":{"threshold":%d}}}`, threshold))
}

// readMergedUserDefault reads back the merged layer's default user policy, which is the policy the
// resolver sees.
func readMergedUserDefault(t *testing.T) sectionPolicy {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/server-config/"+accountAccessSection, nil)
	require.NoError(t, err)

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "reading the section failed: %s", string(body))

	var layers struct {
		Merged struct {
			User struct {
				Default sectionPolicy `json:"default"`
			} `json:"user"`
		} `json:"merged"`
	}
	require.NoError(t, json.Unmarshal(body, &layers), "unreadable section response: %s", string(body))
	return layers.Merged.User.Default
}

// sectionNames lists the server-config sections an operator can edit.
func (ts *AccountAccessSectionTestSuite) sectionNames() []string {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/server-config", nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var names []string
	ts.Require().NoError(json.Unmarshal(body, &names), "unreadable section list: %s", string(body))
	return names
}

// putAccountAccessSection writes the writable layer and returns the status, for the cases where the
// refusal is the subject rather than an error.
func putAccountAccessSection(t *testing.T, body []byte) (int, string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut,
		testServerURL+"/server-config/"+accountAccessSection, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(response)
}

// clearSection empties the writable layer. Failures are fatal in setup, since a suite that starts on
// somebody else's policy is not testing what it says, and logged in teardown.
func (ts *AccountAccessSectionTestSuite) clearSection(fatal bool) {
	ts.T().Helper()

	err := testutils.PutWritableServerConfig(accountAccessSection, []byte(`{}`))
	if fatal {
		ts.Require().NoError(err, "failed to clear the %s section", accountAccessSection)
	} else if err != nil {
		ts.T().Logf("failed to clear the %s section: %v", accountAccessSection, err)
	}
}

// newUser creates one account nobody else in this package touches.
func (ts *AccountAccessSectionTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("section_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       policySectionUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}
