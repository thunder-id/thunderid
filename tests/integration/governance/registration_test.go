// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// RegistrationTestSuite checks that the shipped registration flow still completes and signs the new user in.
type RegistrationTestSuite struct {
	suite.Suite
	users *userFixture
	appID string
}

func TestRegistrationTestSuite(t *testing.T) {
	suite.Run(t, new(RegistrationTestSuite))
}

func (ts *RegistrationTestSuite) SetupSuite() {
	applyLockoutPolicy(ts.T())
	ts.users = newUserFixture(ts.T(), "registration",
		testutils.UserType{AllowSelfRegistration: true, Schema: credentialsSchema(nil)})
	ts.appID = newShippedFlowApp(ts.T(), ts.users, "registration")
}

// The provisioning step binds the user it created by ID, and the provider admits that bind as a
// federated sign-in, which a new user has nothing to hold.
func (ts *RegistrationTestSuite) TestANewlyRegisteredUserCompletesRegistrationAndSignsIn() {
	username := ts.users.usernameFor("new")
	password := "Registered-Horse-1!"

	step, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the registration flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the flow must ask for credentials")
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"), "the flow must ask for a username")

	done, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "password": password}, "action_credentials", step.ChallengeToken)
	// Looked up before the outcome is asserted, so a user provisioned by a failed or refused registration
	// is still deleted.
	userID := ts.findUserID(username)
	ts.Require().NoError(err, "failed to submit the registration")
	ts.Require().Nil(done.Error, "the registration must not be refused: %+v", done.Error)
	ts.Require().Equal("COMPLETE", done.FlowStatus, "a new user must complete registration")
	ts.Require().NotEmpty(userID, "the registration must have created the user")

	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the newly registered user must sign in")
	claims, err := testutils.DecodeJWT(assertion)
	ts.Require().NoError(err, "the sign-in assertion must decode")
	ts.Equal(userID, claims.Sub, "the sign-in must name the registered user")
	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "a newly registered user holds nothing")
}

// findUserID returns the id of the user with this username, recording it for teardown, or empty when
// there is none.
func (ts *RegistrationTestSuite) findUserID(username string) string {
	ts.T().Helper()

	var found testutils.UserListResponse
	ts.Require().NoError(json.Unmarshal(getManagement(ts.T(),
		"/users?filter="+url.QueryEscape(fmt.Sprintf(`username eq %q`, username))), &found),
		"unreadable user search response")
	ts.Require().LessOrEqual(len(found.Users), 1, "a username names at most one user")
	if len(found.Users) == 0 {
		return ""
	}
	ts.users.userIDs = append(ts.users.userIDs, found.Users[0].ID)
	return found.Users[0].ID
}
