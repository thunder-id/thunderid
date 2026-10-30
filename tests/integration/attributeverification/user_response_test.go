// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// UserResponseTestSuite checks the read-only verifiedAttributes field on user responses, and that a
// request cannot set verification.
type UserResponseTestSuite struct {
	suite.Suite
	fixture *verificationFixture
}

func TestUserResponseTestSuite(t *testing.T) {
	suite.Run(t, new(UserResponseTestSuite))
}

func (ts *UserResponseTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "response")
}

func (ts *UserResponseTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// userReads returns the body of every user read that reports one user: by ID, in a filtered list,
// and the user's own.
func (ts *UserResponseTestSuite) userReads(user testUser) map[string][]byte {
	ts.T().Helper()

	filter := url.QueryEscape(fmt.Sprintf(`username eq "%s"`, user.username))
	status, list := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodGet, "/users?filter="+filter, nil)
	ts.Require().Equal(http.StatusOK, status, "user list failed: %s", list)
	var listed struct {
		Users []json.RawMessage `json:"users"`
	}
	ts.Require().NoError(json.Unmarshal(list, &listed))
	ts.Require().Len(listed.Users, 1, "the filter must find exactly the user: %s", list)

	return map[string][]byte{
		"GET /users/{id}": readUserRaw(ts.T(), user.id),
		"GET /users":      listed.Users[0],
		"GET /users/me":   readSelfRaw(ts.T(), ts.fixture.token(ts.T(), user)),
	}
}

// verifiedAttributesIn decodes the verifiedAttributes field of one user object, requiring it present.
func (ts *UserResponseTestSuite) verifiedAttributesIn(body []byte) map[string]json.RawMessage {
	ts.T().Helper()

	var read map[string]json.RawMessage
	ts.Require().NoError(json.Unmarshal(body, &read))
	raw, present := read["verifiedAttributes"]
	ts.Require().True(present, "a user response must carry verifiedAttributes: %s", body)
	var verified map[string]json.RawMessage
	ts.Require().NoError(json.Unmarshal(raw, &verified))
	ts.Require().NotNil(verified, "verifiedAttributes must be an object: %s", body)
	return verified
}

// Every read of a user reports a verified email with the same verification time.
func (ts *UserResponseTestSuite) TestAVerifiedEmailIsReportedOnEveryRead() {
	user := ts.fixture.newUser(ts.T(), "verified")
	ts.fixture.verifyEmail(ts.T(), user)
	expected := verifiedAt(ts.T(), user.id, emailAttribute)
	ts.Require().NotEmpty(expected, "precondition: the email is verified")

	for read, body := range ts.userReads(user) {
		var entry testutils.VerifiedAttribute
		ts.Require().NoError(json.Unmarshal(ts.verifiedAttributesIn(body)[emailAttribute], &entry), read)
		ts.Equal(expected, entry.VerifiedAt, "%s must report the verification", read)
	}
}

// A user with nothing verified has verifiedAttributes as an empty object, not null or absent, in the
// create response, every read and the response to an administrator's or the user's own update.
func (ts *UserResponseTestSuite) TestNothingVerifiedReadsAsAnEmptyObject() {
	user := testUser{username: "verify_unverified_" + ts.fixture.suffix, password: "Correct-Horse-unverified1!",
		email: "unverified." + ts.fixture.suffix + "@example.com"}
	status, created := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPost, "/users",
		map[string]interface{}{"ouId": ts.fixture.ouID, "type": ts.fixture.userType,
			"attributes": map[string]interface{}{"username": user.username, "password": user.password,
				"email": user.email}})
	ts.Require().Equal(http.StatusCreated, status, "the create must succeed: %s", created)
	var createdUser struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(created, &createdUser))
	user.id = createdUser.ID
	ts.fixture.userIDs = append(ts.fixture.userIDs, user.id)

	responses := ts.userReads(user)
	responses["POST /users"] = created
	status, responses["PUT /users/{id}"] = sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut,
		"/users/"+user.id, map[string]interface{}{"ouId": ts.fixture.ouID, "type": ts.fixture.userType,
			"attributes": profile(user, user.email)})
	ts.Require().Equal(http.StatusOK, status, "the administrator's update must succeed: %s",
		responses["PUT /users/{id}"])
	status, responses["PUT /users/me"] = sendJSON(ts.T(),
		testutils.GetHTTPClientWithToken(ts.fixture.token(ts.T(), user)), http.MethodPut, "/users/me",
		map[string]interface{}{"attributes": profile(user, user.email)})
	ts.Require().Equal(http.StatusOK, status, "the self update must succeed: %s", responses["PUT /users/me"])

	for response, body := range responses {
		ts.Empty(ts.verifiedAttributesIn(body), "%s must report nothing verified", response)
	}
}

// A verified email whose user type no longer declares it an email is still reported by every read.
func (ts *UserResponseTestSuite) TestAnAttributeNoLongerAContactIsStillReported() {
	typeID, userType := ts.fixture.newUserType(ts.T(), "no_longer_contact", verificationUserTypeSchema())
	user := ts.fixture.newUserOfType(ts.T(), "no_longer_contact", userType)
	ts.fixture.verifyEmail(ts.T(), user)
	notContact := verificationUserTypeSchema()
	notContact[emailAttribute] = map[string]interface{}{"type": "string"}

	ts.fixture.updateUserTypeSchema(ts.T(), typeID, userType, notContact)

	for read, body := range ts.userReads(user) {
		ts.Contains(ts.verifiedAttributesIn(body), emailAttribute,
			"%s must report a verified attribute that is no longer an email", read)
	}
}

// No read exposes the stored verification metadata or other system attributes, for a verified email
// or for one whose verification a change cleared.
func (ts *UserResponseTestSuite) TestResponsesCarryNoStoredVerificationMetadata() {
	verified := ts.fixture.newUser(ts.T(), "metadata_verified")
	ts.fixture.verifyEmail(ts.T(), verified)
	cleared := ts.fixture.newUser(ts.T(), "metadata_cleared")
	ts.fixture.verifyEmail(ts.T(), cleared)
	status, updated := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/users/"+cleared.id,
		map[string]interface{}{"ouId": ts.fixture.ouID, "type": ts.fixture.userType,
			"attributes": profile(cleared, "changed."+cleared.email)})
	ts.Require().Equal(http.StatusOK, status, "precondition: the email changes: %s", updated)

	for name, user := range map[string]testUser{"verified": verified, "cleared": cleared} {
		reads := ts.userReads(user)
		if name == "cleared" {
			reads["PUT /users/{id}"] = updated
		}
		for read, body := range reads {
			ts.NotContains(string(body), "attributeVerification", "%s must not carry stored metadata", read)
			ts.NotContains(string(body), "systemAttributes", "%s must not carry system attributes", read)
		}
	}
}

// Verification supplied with a create request is ignored.
func (ts *UserResponseTestSuite) TestACreateRequestCannotSetVerification() {
	username := "verify_create_" + ts.fixture.suffix
	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPost, "/users",
		map[string]interface{}{
			"ouId": ts.fixture.ouID,
			"type": ts.fixture.userType,
			"attributes": map[string]interface{}{
				"username": username,
				"email":    username + "@example.com",
			},
			"verifiedAttributes":    suppliedVerification(),
			"attributeVerification": suppliedVerification(),
			"systemAttributes":      map[string]interface{}{"attributeVerification": suppliedVerification()},
		})
	ts.Require().Equal(http.StatusCreated, status, "the create must succeed: %s", body)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.fixture.userIDs = append(ts.fixture.userIDs, created.ID)

	ts.Empty(ts.verifiedAttributesIn(body), "the create response must report nothing verified")
	ts.Empty(verifiedAttributes(ts.T(), created.ID), "a created user must have nothing verified")
}

// Verification supplied with an administrator's or the user's own update is ignored.
func (ts *UserResponseTestSuite) TestAnUpdateRequestCannotSetVerification() {
	user := ts.fixture.newUser(ts.T(), "update_supplied")

	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/users/"+user.id,
		map[string]interface{}{
			"ouId":               ts.fixture.ouID,
			"type":               ts.fixture.userType,
			"attributes":         profile(user, user.email),
			"verifiedAttributes": suppliedVerification(),
			"systemAttributes":   map[string]interface{}{"attributeVerification": suppliedVerification()},
		})
	ts.Require().Equal(http.StatusOK, status, "the administrator's update must succeed: %s", body)
	ts.Empty(ts.verifiedAttributesIn(body), "the administrator's update must report nothing verified")

	status, body = sendJSON(ts.T(), testutils.GetHTTPClientWithToken(ts.fixture.token(ts.T(), user)),
		http.MethodPut, "/users/me", map[string]interface{}{
			"attributes":         profile(user, user.email),
			"verifiedAttributes": suppliedVerification(),
		})
	ts.Require().Equal(http.StatusOK, status, "the self update must succeed: %s", body)
	ts.Empty(ts.verifiedAttributesIn(body), "the self update must report nothing verified")

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "no request may set verification")
}

// suppliedVerification is what a client might send to claim a verified email.
func suppliedVerification() map[string]interface{} {
	return map[string]interface{}{
		emailAttribute: map[string]interface{}{"verifiedAt": "2026-01-01T00:00:00Z"},
	}
}

func (ts *UserResponseTestSuite) TestASelfUpdateReportsStoredAttributesWithVerification() {
	user := ts.fixture.newUser(ts.T(), "stored_response")
	ts.fixture.verifyEmail(ts.T(), user)
	expectedVerification := verifiedAttributes(ts.T(), user.id)
	attributes := profile(user, user.email)
	attributes["undeclared"] = "must-be-filtered"

	status, body := sendJSON(ts.T(), testutils.GetHTTPClientWithToken(ts.fixture.token(ts.T(), user)),
		http.MethodPut, "/users/me", map[string]interface{}{"attributes": attributes})

	ts.Require().Equal(http.StatusOK, status, "%s", body)
	var response struct {
		Attributes         map[string]interface{}                 `json:"attributes"`
		VerifiedAttributes map[string]testutils.VerifiedAttribute `json:"verifiedAttributes"`
	}
	ts.Require().NoError(json.Unmarshal(body, &response))
	ts.NotContains(response.Attributes, "undeclared")
	ts.Equal(user.email, response.Attributes[emailAttribute])
	ts.Equal(expectedVerification, response.VerifiedAttributes)
}
