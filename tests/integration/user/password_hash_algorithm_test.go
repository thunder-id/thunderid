// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cryptoConfigKey         = "crypto"
	hashAlgoUserTypeHandle  = "pwd-hash-algo-type"
	hashAlgoUserPassword    = "HashAlg0P@ssword!"
	hashAlgoWrongPassword   = "Wr0ngP@ssword!"
	passwordHashingCfgKey   = "password_hashing"
	passwordHashingAlgoKey  = "algorithm"
	hashAlgoUsernamePattern = "pwd.hash.%s"
)

// PasswordHashAlgorithmTestSuite verifies that users keep authenticating after the configured password
// hashing algorithm changes. It restarts the server with each algorithm and restores the original
// crypto config afterwards.
type PasswordHashAlgorithmTestSuite struct {
	suite.Suite
	originalCrypto interface{}
	ouID           string
	userTypeID     string
	userIDs        []string
}

func TestPasswordHashAlgorithmTestSuite(t *testing.T) {
	suite.Run(t, new(PasswordHashAlgorithmTestSuite))
}

func (ts *PasswordHashAlgorithmTestSuite) SetupSuite() {
	original, err := testutils.ReadDeploymentConfigKey(cryptoConfigKey)
	ts.Require().NoError(err, "failed to read the crypto config")
	ts.originalCrypto = original

	ouID, err := createOrganizationUnit(OUCreateRequest{
		Handle: "pwd-hash-algo-ou",
		Name:   "Password Hash Algorithm OU",
	})
	ts.Require().NoError(err, "failed to create organization unit")
	ts.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      hashAlgoUserTypeHandle,
		DisplayName: "Password Hash Algorithm Type",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "failed to create user type")
	ts.userTypeID = userTypeID
}

func (ts *PasswordHashAlgorithmTestSuite) TearDownSuite() {
	// Assert rather than Require, so one failure does not skip the restoration steps that follow it.
	for _, userID := range ts.userIDs {
		ts.Assert().NoError(testutils.DeleteUser(userID), "teardown: failed to delete user %s", userID)
	}
	if ts.userTypeID != "" {
		ts.Assert().NoError(testutils.DeleteUserType(ts.userTypeID), "teardown: failed to delete user type")
	}
	if ts.ouID != "" {
		ts.Assert().NoError(deleteOrganizationUnit(ts.ouID), "teardown: failed to delete organization unit")
	}

	ts.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		cryptoConfigKey: ts.originalCrypto,
	}), "teardown: failed to restore the crypto config")
	ts.Assert().NoError(testutils.RestartServer(), "teardown: server did not restart cleanly after config restore")
	ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "teardown: failed to re-obtain admin token after restore")
}

// TestAuthenticateAfterAlgorithmChange creates a user under each algorithm in turn and checks that every
// user created so far, hashed with an earlier algorithm, still authenticates.
func (ts *PasswordHashAlgorithmTestSuite) TestAuthenticateAfterAlgorithmChange() {
	usernames := make([]string, 0)
	for i, algorithm := range []string{"PBKDF2", "ARGON2ID", "SHA256"} {
		if i > 0 {
			ts.switchAlgorithm(algorithm)
		}

		username := fmt.Sprintf(hashAlgoUsernamePattern, strings.ToLower(algorithm))
		ts.createUser(username)
		usernames = append(usernames, username)

		for _, existing := range usernames {
			authenticated, err := testutils.AuthenticateWithCredential(
				"username", existing, "password", hashAlgoUserPassword)
			ts.Require().NoError(err)
			ts.True(authenticated, "%s should authenticate with %s configured", existing, algorithm)

			authenticated, err = testutils.AuthenticateWithCredential(
				"username", existing, "password", hashAlgoWrongPassword)
			ts.Require().NoError(err)
			ts.False(authenticated, "%s should reject a wrong password with %s configured", existing, algorithm)
		}
	}
}

// switchAlgorithm restarts the server with the given password hashing algorithm, keeping the rest of
// the crypto config.
func (ts *PasswordHashAlgorithmTestSuite) switchAlgorithm(algorithm string) {
	crypto := map[string]interface{}{}
	if original, ok := ts.originalCrypto.(map[string]interface{}); ok {
		for k, v := range original {
			crypto[k] = v
		}
	}
	hashing := map[string]interface{}{}
	if original, ok := crypto[passwordHashingCfgKey].(map[string]interface{}); ok {
		for k, v := range original {
			hashing[k] = v
		}
	}
	hashing[passwordHashingAlgoKey] = algorithm
	crypto[passwordHashingCfgKey] = hashing

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{cryptoConfigKey: crypto}),
		"failed to set password hashing algorithm to %s", algorithm)
	ts.Require().NoError(testutils.RestartServer(), "failed to restart server with %s", algorithm)
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain admin token with %s", algorithm)
}

// createUser creates a user with the shared password under the suite's user type.
func (ts *PasswordHashAlgorithmTestSuite) createUser(username string) {
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": hashAlgoUserPassword,
	})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ts.ouID,
		Type:       hashAlgoUserTypeHandle,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create user %s", username)
	ts.userIDs = append(ts.userIDs, userID)
}
