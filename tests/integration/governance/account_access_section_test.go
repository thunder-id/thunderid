// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
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
	users *userFixture

	// originalWritable is the writable layer as the suite found it: the bootstrap stores the lock
	// email templates there, so it is put back rather than cleared.
	originalWritable json.RawMessage
}

func TestAccountAccessSectionTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessSectionTestSuite))
}

// sectionThreshold is lower than the deployment's lockThreshold, so an account locked at this count
// was locked by the section.
const sectionThreshold = 2

// sectionWindowPause outlasts a one-second failure window whatever second a failure falls in, since
// the window is compared at second precision.
const sectionWindowPause = 2 * time.Second

func (ts *AccountAccessSectionTestSuite) SetupSuite() {
	applyDeploymentLockoutPolicy(ts.T())
	var err error
	ts.originalWritable, err = testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err, "failed to read the accountAccess section")
	ts.clearSection()

	ts.users = newUserFixture(ts.T(), "policy_section", testutils.UserType{Schema: credentialsSchema(nil)})
}

// SetupTest empties the writable layer, which is stored and outlives a restart, so one test's policy
// cannot decide the next one's outcome.
func (ts *AccountAccessSectionTestSuite) SetupTest() {
	ts.clearSection()
}

func (ts *AccountAccessSectionTestSuite) TearDownSuite() {
	if ts.originalWritable == nil {
		return
	}
	if err := testutils.PutWritableServerConfig(accountAccessSection, ts.originalWritable); err != nil {
		ts.T().Errorf("teardown: failed to restore the %s section: %v", accountAccessSection, err)
	}
}

// The section is served, and a threshold written through it takes effect without a restart.
func (ts *AccountAccessSectionTestSuite) TestAThresholdWrittenThroughTheSectionTakesEffectAtOnce() {
	ts.Require().Contains(ts.sectionNames(), accountAccessSection,
		"the account-lock policy must be one of the sections an operator can edit")

	userID, username, _ := ts.users.newUser(ts.T(), "section_threshold", nil)

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		userThresholdSection(sectionThreshold)), "the section must accept a threshold")

	failAuthentication(ts.T(), "username", username, sectionThreshold)

	ts.Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the written threshold must be in force on the very next attempt, with no restart")
}

// The writable section keeps fields supplied by the deployment defaults. Checked at the read and
// at the behaviour, since a threshold with no window is treated as disabled.
func (ts *AccountAccessSectionTestSuite) TestTheSectionOverlaysTheDeploymentRatherThanReplacingIt() {
	userID, username, password := ts.users.newUser(ts.T(), "section_overlay", nil)

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		userThresholdSection(sectionThreshold)), "the section must accept a threshold")

	merged := readMergedDefault(ts.T(), "user")
	ts.Equal(sectionThreshold, merged.Threshold, "the written layer must win where it speaks")
	ts.Equal(failureWindowSecs, merged.FailureWindowSeconds,
		"a window the write did not mention must still come from the deployment defaults")
	ts.Equal([]int{firstRungSeconds, secondRungSeconds}, merged.LockDurationsSeconds,
		"the ladder the write did not mention must still come from the deployment defaults")

	failAuthentication(ts.T(), "username", username, sectionThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"a threshold with no window locks nobody, so a lock here is the merge working")

	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
		"the correct secret must be refused while the lock stands")
}

// A write that names only the user category leaves the agent policy the defaults give.
func (ts *AccountAccessSectionTestSuite) TestAWriteThatOmitsTheAgentLeavesTheAgentPolicyIntact() {
	before := readMergedDefault(ts.T(), "agent")
	ts.Require().NotZero(before.Threshold, "the defaults must give agents a policy for the write to leave")

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		userThresholdSection(sectionThreshold)), "the section must accept a user threshold")

	ts.Equal(sectionThreshold, readMergedDefault(ts.T(), "user").Threshold, "the user write must apply")
	ts.Equal(before, readMergedDefault(ts.T(), "agent"), "a write without an agent block must leave it intact")
}

// A misspelled key is refused, and nothing is persisted, unlike other server-config sections.
func (ts *AccountAccessSectionTestSuite) TestAMisspelledSettingIsRefusedAndPersistsNothing() {
	status, response := putSection(ts.T(), accountAccessSection,
		fmt.Sprintf(`{"user":{"default":{"threshold":%d,"lockThreshold":%d}}}`, sectionThreshold, sectionThreshold))
	ts.Equal(http.StatusBadRequest, status,
		"a key the contract does not define must be refused rather than ignored: %s", response)
	ts.Contains(response, "SCF-1003", "a misspelled key must be refused as an invalid value")

	writable, err := testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err)
	ts.NotContains(string(writable), "lockThreshold",
		"a refused write must not have persisted")

	userID, username, _ := ts.users.newUser(ts.T(), "section_misspelled", nil)
	failAuthentication(ts.T(), "username", username, sectionThreshold)
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value,
		"the valid key beside the misspelled one must not have persisted either")
}

// Settings the section no longer defines are refused like any unknown key: a per-method `methods`
// block and a disclosure switch, since every method uses the category default and a hold is always
// stated.
func (ts *AccountAccessSectionTestSuite) TestRemovedSettingsAreRefused() {
	for name, body := range map[string]string{
		"per-method scopes": `{"user":{"scopes":{"otp":{"threshold":2}}}}`,
		"disclosure switch": `{"user":{"default":{"discloseAccountHold":"always"}}}`,
	} {
		status, response := putSection(ts.T(), accountAccessSection, body)
		ts.Equal(http.StatusBadRequest, status, "%s must be refused: %s", name, response)
		ts.Contains(response, "SCF-1003", "%s must be refused as an invalid value", name)
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
		status, response := putSection(ts.T(), accountAccessSection, body)
		ts.Equal(http.StatusBadRequest, status, "%s must be refused: %s", name, response)
		ts.Contains(response, "SCF-1003", "%s must be refused as an invalid value", name)

		after, err := testutils.GetWritableServerConfig(accountAccessSection)
		ts.Require().NoError(err)
		ts.JSONEq(string(before), string(after), "a refused %s must leave the section unchanged", name)
	}
	ts.Equal(sectionThreshold, readMergedDefault(ts.T(), "user").Threshold,
		"the valid threshold must still be in force")

	// The largest accepted value is the bound itself.
	ts.NoError(testutils.PutWritableServerConfig(accountAccessSection, []byte(fmt.Sprintf(
		`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, sectionMaxDurationSeconds))),
		"a duration at the bound must be accepted")
}

// A window of zero never resets the failure count on time. Under a one-second window the pause
// restarts the count; under a zero window the second failure locks.
func (ts *AccountAccessSectionTestSuite) TestAZeroWindowKeepsCountingFailuresAcrossAPause() {
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"default":{"threshold":%d,"failureWindowSeconds":1}}}`, sectionThreshold))),
		"the section must accept a one-second window")
	resetID, resetName, _ := ts.users.newUser(ts.T(), "section_window_reset", nil)
	failAuthentication(ts.T(), "username", resetName, 1)
	time.Sleep(sectionWindowPause)
	failAuthentication(ts.T(), "username", resetName, 1)
	ts.Equal("ACTIVE", readStatus(ts.T(), resetID).Value,
		"a failure older than the window must restart the count")

	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection,
		[]byte(fmt.Sprintf(`{"user":{"default":{"threshold":%d,"failureWindowSeconds":0}}}`, sectionThreshold))),
		"the section must accept a zero window")
	ts.Equal(0, readMergedDefault(ts.T(), "user").FailureWindowSeconds, "the zero must be kept, not dropped")
	keptID, keptName, _ := ts.users.newUser(ts.T(), "section_window_never", nil)
	failAuthentication(ts.T(), "username", keptName, 1)
	time.Sleep(sectionWindowPause)
	failAuthentication(ts.T(), "username", keptName, 1)
	ts.Equal("LOCKED", readStatus(ts.T(), keptID).Value,
		"under a zero window the earlier failure must still count")
}

// clearSection empties the writable layer; a suite that starts on somebody else's policy is not
// testing what it says.
func (ts *AccountAccessSectionTestSuite) clearSection() {
	ts.T().Helper()
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, []byte(`{}`)),
		"failed to clear the %s section", accountAccessSection)
}

// sectionNames lists the server-config sections an operator can edit.
func (ts *AccountAccessSectionTestSuite) sectionNames() []string {
	ts.T().Helper()

	body := getManagement(ts.T(), "/server-config")
	var names []string
	ts.Require().NoError(json.Unmarshal(body, &names), "unreadable section list: %s", string(body))
	return names
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
	applyDeclarativePolicy(ts.T(), lockoutPolicy())
}

// The declarative document is in force over default.json, and a write is refused.
func (ts *AccountAccessDeclarativeLockTestSuite) TestADeclarativeDocumentLocksTheSection() {
	merged := readMergedDefault(ts.T(), "user")
	ts.Equal(lockThreshold, merged.Threshold, "the declarative threshold must win over default.json")

	status, body := putSection(ts.T(), accountAccessSection, string(userThresholdSection(sectionThreshold)))
	ts.Equal(http.StatusBadRequest, status, "a write over a declarative document must be refused")
	ts.Contains(body, "SCF-1003")
	ts.Equal(lockThreshold, readMergedDefault(ts.T(), "user").Threshold, "a refused write changes nothing")
}

// A declarative document with an application block stops the server from starting, where the same
// document through the section API is refused: an application never locks, so a policy for one is
// a configuration error rather than something to ignore.
func (ts *AccountAccessDeclarativeLockTestSuite) TestADeclarativeApplicationBlockStopsTheServerStarting() {
	// Back to the valid document and a running server, even if an assertion below fails.
	defer func() {
		ts.Assert().NoError(testutils.UpdateAccountAccessConfig(lockoutPolicy()))
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

const accountAccessSection = "accountAccess"

// sectionMaxDurationSeconds is the largest seconds value the section accepts: the largest whole second
// a duration can carry.
const sectionMaxDurationSeconds = int64(math.MaxInt64 / int64(time.Second))

// sectionPolicy is the part of a resolved policy this suite makes claims about.
type sectionPolicy struct {
	Enabled              bool  `json:"enabled"`
	Threshold            int   `json:"threshold"`
	FailureWindowSeconds int   `json:"failureWindowSeconds"`
	LockDurationsSeconds []int `json:"lockDurationsSeconds"`
}

// userThresholdSection is a writable layer that states a user threshold and nothing else, the
// smallest edit the Console's account-lock card can make.
func userThresholdSection(threshold int) []byte {
	return []byte(fmt.Sprintf(`{"user":{"default":{"threshold":%d}}}`, threshold))
}

// readMergedDefault reads back the merged layer's default policy for a category, which is the policy
// the resolver sees.
func readMergedDefault(t *testing.T, category string) sectionPolicy {
	t.Helper()

	body := getManagement(t, "/server-config/"+accountAccessSection)
	var layers struct {
		Merged map[string]struct {
			Default sectionPolicy `json:"default"`
		} `json:"merged"`
	}
	require.NoError(t, json.Unmarshal(body, &layers), "unreadable section response: %s", string(body))
	return layers.Merged[category].Default
}
