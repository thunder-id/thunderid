// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// DisclosureTestSuite checks that a locked holder is told so: at the credentials endpoint under the
// shipped policy, and on the prompt of the shipped sign-in flow.
type DisclosureTestSuite struct {
	suite.Suite
	users        *userFixture
	shippedAppID string
}

func TestDisclosureTestSuite(t *testing.T) {
	suite.Run(t, new(DisclosureTestSuite))
}

func (ts *DisclosureTestSuite) SetupSuite() {
	ts.users = newUserFixture(ts.T(), "disclosure", testutils.UserType{Schema: credentialsSchema(nil)})
	ts.shippedAppID = newShippedFlowApp(ts.T(), ts.users, "disclosure")
}

// Five failures lock under the shipped policy.
func (ts *DisclosureTestSuite) TestSignInStatesTheLock() {
	userID, username, password := ts.users.newUser(ts.T(), "default_locked", nil)
	_, other, _ := ts.users.newUser(ts.T(), "default_unlocked", nil)
	failAuthentication(ts.T(), "username", username, shippedUserLockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the shipped threshold must lock the account")

	wrongSecret := authenticate(ts.T(), "username", other, "Not-The-Secret-2!")
	for _, presented := range []struct{ name, secret string }{
		{"the correct password", password},
		{"a wrong password", "Not-The-Secret-2!"},
	} {
		held := authenticate(ts.T(), "username", username, presented.secret)
		requireStatedHold(ts.T(), wrongSecret, held, statedLockCode, "credential")
		ts.NotContains(held.body, statedSuspensionCode, "%s: a lock must not be stated as a suspension",
			presented.name)
	}

	unknown := authenticate(ts.T(), "username", "no_such_user_"+ts.users.suffix, password)
	ts.Require().False(unknown.succeeded())
	ts.NotContains(unknown.body, statedLockCode, "an unknown identifier must not be stated as locked")
	ts.NotContains(unknown.body, statedSuspensionCode, "an unknown identifier must not be stated as suspended")
}

// The lock is formed under a rung that never lapses, so the prompt cannot be reached through a lapse.
func (ts *DisclosureTestSuite) TestTheShippedSignInFlowKeepsALockedHolderOnThePrompt() {
	applyPolicy(ts.T(), lockoutPolicyWith(userDefaults(nonExpiringRung())))
	userID, username, password := ts.users.newUser(ts.T(), "flow_locked", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the threshold must lock the account")

	for _, secret := range []string{password, "Not-The-Secret-5!"} {
		status, body := submitSignIn(ts.T(), ts.shippedAppID, shippedSignInAction, username, secret)
		requireRepromptedWith(ts.T(), flowStepOf(ts.T(), status, body), flowStatedLockCode)
		ts.NotContains(body, flowStatedSuspensionCode, "a lock must not be stated as a suspension")
	}
}
