// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// federatedHolderSchema is a user type with a username, a password and the email a federated
// identity is matched on for linking.
var federatedHolderSchema = credentialsSchema(map[string]interface{}{"email": stringAttribute})

// FederatedTestSuite checks, under authentication method granularity, that a federated
// sign-in to a linked account obeys the account's suspension and account-wide lock, and ignores a lock
// on another authentication method. Where several users hold the identity, the one a sign-in picks or
// a registration completes as is bound by ID and held the same way.
type FederatedTestSuite struct {
	suite.Suite
	users             *userFixture
	others            *userFixture
	pickerAppID       string
	registrationAppID string
	federated         *federatedFixture
}

func TestFederatedTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedTestSuite))
}

func (ts *FederatedTestSuite) SetupSuite() {
	// Long rungs: a lock that lapsed mid-test would let a test pass or fail for the wrong reason.
	applyPolicy(ts.T(), longRungLockoutPolicy())

	ts.users = newUserFixture(ts.T(), "federated",
		testutils.UserType{AllowSelfRegistration: true, Schema: federatedHolderSchema})
	// The organization unit and type of a link holder a registration into the suite's unit does not
	// count.
	ts.others = newUserFixture(ts.T(), "federated_other", testutils.UserType{Schema: federatedHolderSchema})

	ts.federated = newFederatedFixture(ts.T(), "hold", ts.users.ouID, ts.users.userType)
}

// A password lock does not hold a federated sign-in, and the sign-in does not release it.
func (ts *FederatedTestSuite) TestAPasswordLockDoesNotHoldAFederatedSignIn() {
	userID, username, password, email := ts.newLinkedUser("password_lock")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedMethods(),
		"the password must be locked")

	ts.True(ts.federated.signedIn(ts.T(), email), "a password lock must not hold a federated sign-in")

	ts.Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedMethods(),
		"a federated sign-in must not release the password lock")
	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "username", username, password), statedLockCode,
		"the password must stay locked")
}

// A suspended linked account is refused, and the suspension is stated.
func (ts *FederatedTestSuite) TestASuspendedLinkedAccountIsRefused() {
	userID, _, _, email := ts.newLinkedUser("suspended")
	ts.Require().True(ts.federated.signedIn(ts.T(), email), "the linked account must sign in first")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedSuspensionCode, "suspended")
}

// The account-wide lock an unsuspend leaves holds a federated sign-in, and unlock releases it.
func (ts *FederatedTestSuite) TestTheLockAnUnsuspendLeavesHoldsAFederatedSignIn() {
	userID, _, _, email := ts.newLinkedUser("unsuspended")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().Equal([]string{"account"}, readStatus(ts.T(), userID).lockedMethods(),
		"unsuspend must land on an account-wide lock")

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedLockCode, "unsuspended")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(ts.federated.signedIn(ts.T(), email), "unlock must release the federated sign-in")
}

// A code the IdP never issued fails at the exchange, before any account is resolved, so the failure
// names nobody. The password locking afterwards shows the policy was in force.
func (ts *FederatedTestSuite) TestAFailedCodeExchangeLocksNobody() {
	userID, username, password, email := ts.newLinkedUser("federated_failures")

	for i := range lockThreshold * 2 {
		refused := ts.federated.signInWithCode(ts.T(), email, "not-a-code")
		ts.Require().NotEqual("COMPLETE", refused.flowStatus, "precondition: attempt %d with a bad code must fail",
			i+1)
		ts.Require().NotEmpty(refused.code, "attempt %d must be refused with an error: %+v", i+1, refused)
		ts.Require().NotContains([]string{flowStatedLockCode, flowStatedSuspensionCode}, refused.code,
			"attempt %d must not be refused as a hold", i+1)
	}

	status := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", status.Value, "a failed exchange must not lock the account")
	ts.Empty(status.lockedMethods(), "a failed exchange must lock no authentication method")
	ts.True(ts.federated.signedIn(ts.T(), email), "the federated sign-in must still work")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the password must still work")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedMethods(),
		"the same policy must still lock the password")
}

// newLinkedUser creates an account and links a fresh IdP identity to it.
func (ts *FederatedTestSuite) newLinkedUser(name string) (userID, username, password, email string) {
	ts.T().Helper()

	email = ts.federated.addIdentity(federatedSub(name, ts.users.suffix))
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{"email": email})
	ts.federated.link(ts.T(), email, username, password)
	return userID, username, password, email
}

// A suspended link holder the user picks is refused with the suspension stated, and the other
// holder's sign-in is unaffected.
func (ts *FederatedTestSuite) TestAPickedLinkHolderThatIsSuspendedIsRefused() {
	email, held, other := ts.newHeldIdentity("pick_suspended", ts.users)
	runAdministrationFlow(ts.T(), suspendFlowHandle, held.id)

	refused := ts.federated.signInAndPick(ts.T(), ts.pickerApp(), email, held.username)
	requireStatedHoldStep(ts.T(), refused, flowStatedSuspensionCode, "suspended holder")

	ts.requireSignedInAs(ts.federated.signInAndPick(ts.T(), ts.pickerApp(), email, other.username), other.id,
		"precondition: the other holder of the same identity must sign in")
}

// A link holder held by an account-wide lock is refused when picked, with the lock stated: the bind is
// a federated sign-in, which an account-wide lock holds.
func (ts *FederatedTestSuite) TestAPickedLinkHolderWithAnAccountWideLockIsRefused() {
	email, held, _ := ts.newHeldIdentity("pick_account_lock", ts.users)
	runAdministrationFlow(ts.T(), suspendFlowHandle, held.id)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, held.id)
	ts.Require().Equal([]string{"account"}, readStatus(ts.T(), held.id).lockedMethods(),
		"unsuspend must land on an account-wide lock")

	refused := ts.federated.signInAndPick(ts.T(), ts.pickerApp(), email, held.username)
	requireStatedHoldStep(ts.T(), refused, flowStatedLockCode, "account-wide locked holder")
}

// The pick does not release the password lock.
func (ts *FederatedTestSuite) TestAPickedLinkHolderWithAPasswordLockSignsIn() {
	email, held, _ := ts.newHeldIdentity("pick_password_lock", ts.users)
	failAuthentication(ts.T(), "username", held.username, lockThreshold)
	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), held.id).lockedMethods(),
		"the password must be locked")

	ts.requireSignedInAs(ts.federated.signInAndPick(ts.T(), ts.pickerApp(), email, held.username), held.id,
		"a password lock must not hold the picked holder's federated sign-in")
	ts.Equal([]string{"credential"}, readStatus(ts.T(), held.id).lockedMethods(),
		"the federated sign-in must not release the password lock")
}

// A registration that completes as the one holder in its organization unit is refused with the
// suspension stated when that holder is suspended.
func (ts *FederatedTestSuite) TestARegistrationAsASuspendedLinkHolderIsRefused() {
	email, held, _ := ts.newHeldIdentity("register_suspended", ts.others)
	runAdministrationFlow(ts.T(), suspendFlowHandle, held.id)

	refused := ts.federated.register(ts.T(), ts.registrationApp(), email)
	requireStatedHoldStep(ts.T(), refused, flowStatedSuspensionCode, "registration as a suspended holder")
}

// The registration leaves the holder's password lock in place.
func (ts *FederatedTestSuite) TestARegistrationAsAPasswordLockedLinkHolderCompletes() {
	email, held, _ := ts.newHeldIdentity("register_password_lock", ts.others)
	failAuthentication(ts.T(), "username", held.username, lockThreshold)
	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), held.id).lockedMethods(),
		"the password must be locked")

	before := ts.usersIn(ts.users.ouID)
	done := ts.federated.register(ts.T(), ts.registrationApp(), email)
	ts.Require().Nil(done.Error, "a password lock must not refuse the registration: %+v", done.Error)
	ts.Equal("COMPLETE", done.FlowStatus, "the registration must complete as the existing holder")
	ts.Equal(before, ts.usersIn(ts.users.ouID), "the registration must complete as the holder, not create a user")
	ts.Equal([]string{"credential"}, readStatus(ts.T(), held.id).lockedMethods(),
		"the registration must not release the password lock")
}

// pickerApp returns the application whose sign-in picks between link holders, creating it on first use
// so that a failure to create it fails a test rather than the suite's setup.
func (ts *FederatedTestSuite) pickerApp() string {
	if ts.pickerAppID == "" {
		ts.pickerAppID = ts.federated.newLinkPickerApp(ts.T(), "hold_picker")
	}
	return ts.pickerAppID
}

// registrationApp returns the application that registers through the IdP, creating it on first use.
func (ts *FederatedTestSuite) registrationApp() string {
	if ts.registrationAppID == "" {
		ts.registrationAppID = ts.federated.newRegistrationApp(ts.T(), "hold_registration")
	}
	return ts.registrationAppID
}

// newHeldIdentity registers a fresh IdP identity and links it to two new accounts: held, in the suite's
// unit, and other, in otherUnit. It returns the email the IdP asserts for the identity.
func (ts *FederatedTestSuite) newHeldIdentity(name string, otherUnit *userFixture) (
	email string, held, other linkHolder) {
	ts.T().Helper()

	email = ts.federated.addIdentity(federatedSub(name, ts.users.suffix))
	held = ts.newLinkHolder(name+"_held", email, ts.users)
	other = ts.newLinkHolder(name+"_other", email, otherUnit)
	ts.federated.linkHolders(ts.T(), email, held, other)
	return email, held, other
}

// newLinkHolder creates an account of the fixture carrying the email the linking sign-in matches.
func (ts *FederatedTestSuite) newLinkHolder(name, email string, users *userFixture) linkHolder {
	ts.T().Helper()

	userID, username, password := users.newUser(ts.T(), name, map[string]interface{}{"email": email})
	return linkHolder{id: userID, username: username, password: password, ouID: users.ouID,
		userType: users.userType}
}

// usersIn counts the users in an organization unit.
func (ts *FederatedTestSuite) usersIn(ouID string) int {
	ts.T().Helper()

	var listed struct {
		TotalResults int `json:"totalResults"`
	}
	ts.Require().NoError(json.Unmarshal(getManagement(ts.T(), "/organization-units/"+ouID+"/users"), &listed))
	return listed.TotalResults
}

// requireSignedInAs requires the step to have completed with an assertion issued for userID.
func (ts *FederatedTestSuite) requireSignedInAs(step *common.FlowStep, userID, msg string) {
	ts.T().Helper()

	ts.Require().Nil(step.Error, "%s: %+v", msg, step.Error)
	ts.Require().Equal("COMPLETE", step.FlowStatus, msg)
	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err, "%s: the assertion must decode", msg)
	ts.Equal(userID, claims.Sub, "%s: the assertion must name the picked holder", msg)
}
