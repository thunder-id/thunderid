// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The applications that sign the suite's users in through attributeCollectionFlow, one collecting a
// mobile number and one an email.
const (
	attributeCollectionClientID     = "verification_attribute_collection_client"
	attributeCollectionClientSecret = "Verification-Attribute-Collection-Secret1!"
	emailCollectionClientID         = "verification_email_collection_client"
	emailCollectionClientSecret     = "Verification-Email-Collection-Secret1!"
)

// The SCIM endpoints a provisioning connector replaces a user through, and the media type it sends.
const (
	scimSchemasPath = "/scim/v2/Schemas"
	scimUsersPath   = "/scim/v2/Users/"
	scimContentType = "application/scim+json"
)

// InvalidationTestSuite checks that a verified email or mobile number loses its verification whenever
// a profile write changes or removes it, and keeps it through writes that do not.
type InvalidationTestSuite struct {
	suite.Suite
	fixture              *verificationFixture
	sms                  *smsProvider
	collectionAppID      string
	emailCollectionAppID string
	otherTypeHandle      string
}

func TestInvalidationTestSuite(t *testing.T) {
	suite.Run(t, new(InvalidationTestSuite))
}

func (ts *InvalidationTestSuite) SetupSuite() {
	t := ts.T()
	ts.fixture = newVerificationFixture(t, "invalidation")
	ts.sms = newSMSProvider(t, "Invalidation")
	ts.fixture.nameSender(t, mobileSendNode, ts.sms.senderID)

	// A second type with the same contact, so a type change keeps the value but not its verification.
	ts.otherTypeHandle = "verification_invalidation_other_user"
	otherTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      ts.otherTypeHandle,
		DisplayName: "Attribute Verification Invalidation Other User",
		OUID:        ts.fixture.ouID,
		Schema:      verificationUserTypeSchema(),
	})
	ts.Require().NoError(err, "failed to create the second user type")
	t.Cleanup(func() {
		// A user a test moved to this type must go first.
		ts.fixture.deleteUsers(t)
		if err := testutils.DeleteUserType(otherTypeID); err != nil {
			t.Logf("cleanup: failed to delete the second user type: %v", err)
		}
	})

	ts.collectionAppID = ts.createCollectionApp("Attribute Collection", attributeCollectionClientID,
		attributeCollectionClientSecret, mobileAttribute, "PHONE_INPUT")
	ts.emailCollectionAppID = ts.createCollectionApp("Email Collection", emailCollectionClientID,
		emailCollectionClientSecret, emailAttribute, "EMAIL_INPUT")
}

// createCollectionApp creates an application for the fixture's users whose sign-in collects the
// attribute, and deletes it when the suite ends.
func (ts *InvalidationTestSuite) createCollectionApp(name, clientID, clientSecret, attribute,
	inputType string) string {
	t := ts.T()

	flowID := createFlow(t, attributeCollectionFlow("verification-"+strings.ToLower(strings.ReplaceAll(name, " ", "-"))+
		"-"+ts.fixture.suffix, attribute, inputType))
	return createApp(t, testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification " + name + " App",
		Description:      "Application whose sign-in collects " + attribute,
		ClientID:         clientID,
		ClientSecret:     clientSecret,
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.fixture.userType},
		AuthFlowID:       flowID,
	})
}

func (ts *InvalidationTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
	ts.sms.server.ClearMessages()
}

// attributeCollectionFlow signs a user in with their password and then collects the attribute, which
// it merges into the stored attributes.
func attributeCollectionFlow(handle, attribute, inputType string) testutils.Flow {
	return testutils.Flow{
		Name:     "Attribute Verification Collection Flow for " + attribute,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "credentials_auth"},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "attribute_collect",
			},
			{
				"id":   "attribute_collect",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AttributeCollector",
					"inputs": []map[string]interface{}{
						{"ref": "input_" + attribute, "identifier": attribute, "type": inputType, "required": true},
					},
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// newVerifiedUser creates a user and verifies their email through the self-service flow.
func (ts *InvalidationTestSuite) newVerifiedUser(name string) testUser {
	ts.T().Helper()

	user := ts.fixture.newUser(ts.T(), name)
	ts.fixture.verifyEmail(ts.T(), user)
	return user
}

// newUserWithBothVerified creates a user and verifies their email and their mobile number through the
// self-service flow.
func (ts *InvalidationTestSuite) newUserWithBothVerified(name string) testUser {
	ts.T().Helper()

	user := ts.newVerifiedUser(name)
	ts.fixture.verifyMobile(ts.T(), ts.sms, user)
	return user
}

// selfUpdate replaces the user's attributes through PUT /users/me.
func (ts *InvalidationTestSuite) selfUpdate(user testUser, attributes map[string]interface{}) {
	ts.T().Helper()

	status, body := sendJSON(ts.T(), testutils.GetHTTPClientWithToken(ts.fixture.token(ts.T(), user)),
		http.MethodPut, "/users/me", map[string]interface{}{"attributes": attributes})
	ts.Require().Equal(http.StatusOK, status, "the self update must succeed: %s", body)
}

// contacts is the attribute set a write sends, with the given email and mobile number.
func contacts(user testUser, email, mobile string) map[string]interface{} {
	return map[string]interface{}{"username": user.username, "email": email, "mobile_number": mobile}
}

func (ts *InvalidationTestSuite) TestAnAdministratorChangingTheEmailClearsVerification() {
	user := ts.newVerifiedUser("admin_change")

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, otherAddress(user)))

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a changed email must not stay verified")
}

func (ts *InvalidationTestSuite) TestASelfUpdateChangingTheEmailClearsVerification() {
	user := ts.newVerifiedUser("self_change")

	ts.selfUpdate(user, profile(user, otherAddress(user)))

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a changed email must not stay verified")
}

// Changing the email and changing it back does not restore the earlier verification.
func (ts *InvalidationTestSuite) TestChangingTheEmailAndBackLeavesItUnverified() {
	user := ts.newVerifiedUser("change_back")

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, otherAddress(user)))
	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, user.email))

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "the original address must not regain verification")
}

// Removing the email clears its verification, and restoring the same address does not bring it back.
func (ts *InvalidationTestSuite) TestRemovingTheEmailClearsVerification() {
	user := ts.newVerifiedUser("remove")

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, map[string]interface{}{"username": user.username})
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a removed email must not stay verified")

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, user.email))
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "restoring the address must not restore verification")
}

// A self update replaces the whole attribute set, so leaving the email out removes it, and putting the
// same address back does not restore its verification.
func (ts *InvalidationTestSuite) TestASelfUpdateOmittingTheEmailClearsVerification() {
	user := ts.newVerifiedUser("self_omit")

	ts.selfUpdate(user, map[string]interface{}{"username": user.username, "displayName": "Omitted Email"})
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "an omitted email must not stay verified")

	ts.selfUpdate(user, profile(user, user.email))
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "restoring the address must not restore verification")
}

// Updates that leave the email as it is keep its verification and its time, from either write path.
func (ts *InvalidationTestSuite) TestAnUnrelatedUpdateKeepsVerification() {
	user := ts.newVerifiedUser("unrelated")
	before := verifiedAt(ts.T(), user.id, emailAttribute)

	unrelated := profile(user, user.email)
	unrelated["displayName"] = "Changed By Administrator"
	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, unrelated)
	ts.Equal(before, verifiedAt(ts.T(), user.id, emailAttribute),
		"an administrator's update of another attribute must keep the verification")

	unrelated["displayName"] = "Changed By User"
	ts.selfUpdate(user, unrelated)
	ts.Equal(before, verifiedAt(ts.T(), user.id, emailAttribute),
		"a self update of another attribute must keep the verification")
}

// Changing the user's type clears verification even when the address stays the same.
func (ts *InvalidationTestSuite) TestChangingTheUserTypeClearsVerification() {
	user := ts.newVerifiedUser("type_change")

	ts.fixture.adminUpdate(ts.T(), user, ts.otherTypeHandle, profile(user, user.email))

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a user type change must clear verification")
}

// Changing the mobile number clears its verification and keeps the email's, from either write path.
func (ts *InvalidationTestSuite) TestChangingTheMobileNumberClearsOnlyItsVerification() {
	for name, write := range map[string]func(testUser, map[string]interface{}){
		"administrator": func(user testUser, attributes map[string]interface{}) {
			ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, attributes)
		},
		"self": ts.selfUpdate,
	} {
		ts.Run(name, func() {
			user := ts.newUserWithBothVerified("mobile_change_" + name)
			emailVerifiedAt := verifiedAt(ts.T(), user.id, emailAttribute)

			write(user, contacts(user, user.email, ts.fixture.uniqueMobile()))

			ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "a changed mobile number must not stay verified")
			ts.Equal(emailVerifiedAt, verifiedAt(ts.T(), user.id, emailAttribute),
				"changing the mobile number must keep the email's verification")
		})
	}
}

// Removing the mobile number clears its verification, and restoring the same number does not bring it
// back.
func (ts *InvalidationTestSuite) TestRemovingTheMobileNumberClearsVerification() {
	user := ts.newUserWithBothVerified("mobile_remove")
	emailVerifiedAt := verifiedAt(ts.T(), user.id, emailAttribute)

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, user.email))
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "a removed mobile number must not stay verified")
	ts.Equal(emailVerifiedAt, verifiedAt(ts.T(), user.id, emailAttribute),
		"removing the mobile number must keep the email's verification")

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, contacts(user, user.email, user.mobile))
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "restoring the number must not restore verification")
}

// Attribute collection writes only the attribute it collects into the stored set, so a new mobile
// number clears the mobile number's verification and leaves the email's as it was.
func (ts *InvalidationTestSuite) TestAttributeCollectionChangingTheMobileNumberClearsOnlyItsVerification() {
	user := ts.newUserWithBothVerified("collect_mobile")
	emailVerifiedAt := verifiedAt(ts.T(), user.id, emailAttribute)

	ts.signInCollecting(ts.collectionAppID, user, mobileAttribute, ts.fixture.uniqueMobile())

	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "a collected new mobile number must not stay verified")
	ts.Equal(emailVerifiedAt, verifiedAt(ts.T(), user.id, emailAttribute),
		"collecting the mobile number must keep the email's verification")
}

// Attribute collection that collects a new email clears the email's verification.
func (ts *InvalidationTestSuite) TestAttributeCollectionChangingTheEmailClearsVerification() {
	user := ts.newVerifiedUser("collect_email")

	ts.signInCollecting(ts.emailCollectionAppID, user, emailAttribute, otherAddress(user))

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a collected new email must not stay verified")
}

// signInCollecting signs the user in through the application, whose sign-in collects attribute, and
// gives value for it.
func (ts *InvalidationTestSuite) signInCollecting(appID string, user testUser, attribute, value string) {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "precondition: the sign-in asks for credentials")
	done, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": user.username, "password": user.password, attribute: value,
	}, "", step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "precondition: the sign-in collects %s", attribute)
}

// A SCIM replace that changes the email, as a provisioning connector sends it, clears its verification.
func (ts *InvalidationTestSuite) TestASCIMReplaceChangingTheEmailClearsVerification() {
	user := ts.newVerifiedUser("scim_change")

	ts.scimReplace(user, profile(user, otherAddress(user)))

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a changed email must not stay verified")
}

// scimReplace replaces the user's attributes through SCIM, as the administrator.
func (ts *InvalidationTestSuite) scimReplace(user testUser, attributes map[string]interface{}) {
	ts.T().Helper()

	urn := ts.scimExtensionURN()
	payload, err := json.Marshal(map[string]interface{}{"schemas": []string{urn}, urn: attributes})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, testServerURL+scimUsersPath+user.id, bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", scimContentType)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "precondition: the SCIM replace succeeds: %s", body)
}

// scimExtensionURN returns the SCIM schema a client provisions the fixture's user type through.
func (ts *InvalidationTestSuite) scimExtensionURN() string {
	ts.T().Helper()

	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodGet, scimSchemasPath, nil)
	ts.Require().Equal(http.StatusOK, status, "the SCIM schemas must be listed: %s", body)
	var schemas struct {
		Resources []struct {
			ID string `json:"id"`
		} `json:"Resources"`
	}
	ts.Require().NoError(json.Unmarshal(body, &schemas))
	suffix := ":" + strings.ToLower(ts.fixture.userType) + ":2.0:user"
	for _, schema := range schemas.Resources {
		if strings.HasSuffix(strings.ToLower(schema.ID), suffix) {
			return schema.ID
		}
	}
	ts.Require().FailNow("SCIM must list a schema for the fixture's user type", "%s", body)
	return ""
}

// A password change rewrites the user's system attributes and keeps the verification and its time.
func (ts *InvalidationTestSuite) TestAPasswordChangeKeepsVerification() {
	user := ts.newVerifiedUser("password_change")
	before := verifiedAt(ts.T(), user.id, emailAttribute)

	ts.Require().NoError(testutils.UpdateUserCredentials(user.id,
		map[string]string{"password": "Changed-Horse-Password1!"}), "precondition: the password changes")

	ts.Equal(before, verifiedAt(ts.T(), user.id, emailAttribute), "a password change must keep the verification")
}

// A verified email stays verified while its user type declares it plain text, and once it changes it
// stays unverified when the type declares it an email again, although no contact rule applied to it then.
func (ts *InvalidationTestSuite) TestAnEmailChangedWhileNotAContactStaysUnverified() {
	typeID, userType := ts.fixture.newUserType(ts.T(), "schema_flip", verificationUserTypeSchema())
	user := ts.fixture.newUserOfType(ts.T(), "schema_flip", userType)
	ts.fixture.verifyEmail(ts.T(), user)
	notContact := verificationUserTypeSchema()
	notContact[emailAttribute] = map[string]interface{}{"type": "string"}
	ts.fixture.updateUserTypeSchema(ts.T(), typeID, userType, notContact)

	ts.NotEmpty(verifiedAt(ts.T(), user.id, emailAttribute),
		"a verified email must stay verified while its type is not a contact and its value is unchanged")

	ts.fixture.adminUpdate(ts.T(), user, userType, profile(user, otherAddress(user)))
	ts.fixture.updateUserTypeSchema(ts.T(), typeID, userType, verificationUserTypeSchema())

	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute),
		"an email changed while it was not a contact must not be verified once it is one again")
}
