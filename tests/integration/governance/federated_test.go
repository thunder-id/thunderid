// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// FederatedHoldTestSuite checks, under authentication method granularity, that a federated
// sign-in to a linked account obeys the account's suspension and entity-wide lock, and ignores a lock
// on another authentication method.
type FederatedHoldTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userIDs      []string
	suffix       string
	federated    *federatedFixture
}

func TestFederatedHoldTestSuite(t *testing.T) {
	suite.Run(t, new(FederatedHoldTestSuite))
}

var federatedHoldOU = testutils.OrganizationUnit{
	Handle:      "governance_federated_ou",
	Name:        "Governance Federated Test OU",
	Description: "Organization unit for federated sign-in hold tests",
}

var federatedHoldUserType = testutils.UserType{
	Handle:      "governance_federated_user",
	DisplayName: "governance_federated_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

func (ts *FederatedHoldTestSuite) SetupSuite() {
	// Long rungs: a lock that lapsed mid-test would let a test pass or fail for the wrong reason.
	applyPolicy(ts.T(), longRungLockoutPolicy())

	ouID, err := testutils.CreateOrganizationUnit(federatedHoldOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	federatedHoldUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(federatedHoldUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
	ts.federated = newFederatedFixture(ts.T(), "hold", ts.ouID,
		federatedHoldUserType.Handle)
}

func (ts *FederatedHoldTestSuite) TearDownSuite() {
	ts.federated.tearDown(ts.T())
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

// A password lock does not hold a federated sign-in, and the sign-in does not release it.
func (ts *FederatedHoldTestSuite) TestAPasswordLockDoesNotHoldAFederatedSignIn() {
	userID, username, password, email := ts.newLinkedUser("password_lock")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedScopes(),
		"the password must be locked")

	ts.True(ts.federated.signedIn(ts.T(), email), "a password lock must not hold a federated sign-in")

	ts.Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedScopes(),
		"a federated sign-in must not release the password lock")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the password must stay locked")
}

// A suspended linked account is refused, and the suspension is stated.
func (ts *FederatedHoldTestSuite) TestASuspendedLinkedAccountIsRefused() {
	userID, _, _, email := ts.newLinkedUser("suspended")
	ts.Require().True(ts.federated.signedIn(ts.T(), email), "the linked account must sign in first")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedSuspensionCode, "suspended")
}

// The entity-wide lock an unsuspend leaves holds a federated sign-in, and unlock releases it.
func (ts *FederatedHoldTestSuite) TestTheLockAnUnsuspendLeavesHoldsAFederatedSignIn() {
	userID, _, _, email := ts.newLinkedUser("unsuspended")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().Equal([]string{"entity"}, readStatus(ts.T(), userID).lockedScopes(),
		"unsuspend must land on an entity-wide lock")

	ts.federated.requireRefusedWithStatedHold(ts.T(), email, flowStatedLockCode, "unsuspended")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(ts.federated.signedIn(ts.T(), email), "unlock must release the federated sign-in")
}

// Failed federated sign-ins lock nobody: a federated failure carries no attributable bad-secret
// evidence, and the federated authentication method is not lockable. This is a pure negative, so the
// linked account's password is then shown to lock as normal under the same policy.
func (ts *FederatedHoldTestSuite) TestFailedFederatedSignInsLockNobody() {
	userID, username, password, email := ts.newLinkedUser("federated_failures")

	for i := range lockThreshold * 2 {
		ts.Require().False(ts.federated.signedInWithCode(ts.T(), email, "not-a-code"),
			"attempt %d with a bad code must fail", i+1)
	}

	status := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", status.Value, "federated failures must not lock the account")
	ts.Empty(status.lockedScopes(), "federated failures must lock no authentication method")
	ts.True(ts.federated.signedIn(ts.T(), email), "the federated sign-in must still work")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the password must still work")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedScopes(),
		"the same policy must still lock the password")
}

// newLinkedUser creates an account and links a fresh IdP identity to it.
func (ts *FederatedHoldTestSuite) newLinkedUser(name string) (userID, username, password, email string) {
	ts.T().Helper()

	sub := federatedSub(name, ts.suffix)
	email = ts.federated.addIdentity(sub)
	username = fmt.Sprintf("federated_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
		"email":    email,
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       federatedHoldUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the linked user")
	ts.userIDs = append(ts.userIDs, userID)
	ts.federated.link(ts.T(), email, username, password)
	return userID, username, password, email
}
