// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// notificationSection is the server-config section naming the default email sender.
const notificationSection = "notification"

// shippedLockEmail is the lock email templates the bootstrap stores in the accountAccess section.
var shippedLockEmail = map[string]interface{}{
	"templates": map[string]interface{}{
		"temporary": "authentication-method-locked",
		"permanent": "authentication-method-locked-permanent",
		"account": map[string]interface{}{
			"temporary": "account-locked",
			"permanent": "account-locked-permanent",
		},
	},
}

// declarativeSenderID is the email sender declarativeSenderResources declares.
const declarativeSenderID = "governance-declarative-email-sender"

// declarativeSenderResources declares an email sender and a notification section naming it.
const declarativeSenderResources = `resource_type: connection
id: ` + declarativeSenderID + `
type: email-smtp
name: Governance Declarative Email Sender
host: localhost
port: 2525
fromAddress: noreply@thunderid.test
---
resource_type: server_config
name: notification
value:
  defaultEmailSenderId: ` + declarativeSenderID + `
`

// LockNotificationTestSuite checks the lock email settings: the templates the bootstrap stores in the
// accountAccess section, the values that section and the notification section refuse, and a
// declarative notification section naming a declarative sender. The suspend and unsuspend flows email
// the holder through the sender their notify_holder node names.
type LockNotificationTestSuite struct {
	suite.Suite
	smtp                  *testutils.MockSMTPServer
	senderID              string
	users                 *userFixture
	originalNotification  json.RawMessage
	originalAccountAccess json.RawMessage
}

func TestLockNotificationTestSuite(t *testing.T) {
	suite.Run(t, new(LockNotificationTestSuite))
}

func (ts *LockNotificationTestSuite) SetupSuite() {
	var err error
	ts.originalNotification, err = testutils.GetWritableServerConfig(notificationSection)
	ts.Require().NoError(err, "failed to read the notification section")
	ts.originalAccountAccess, err = testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err, "failed to read the accountAccess section")

	ts.smtp = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.smtp.Start(), "failed to start the mock SMTP server")
	senderID, err := testutils.CreateSMTPEmailProvider("Lock Notification Test Provider",
		"localhost", ts.smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	ts.senderID = senderID

	emailAttribute := map[string]interface{}{"type": "email"}
	ts.users = newUserFixture(ts.T(), "lock_notification", testutils.UserType{
		Schema: credentialsSchema(map[string]interface{}{"email": emailAttribute}),
	})
}

// SetupTest starts each test from this package's policy and no default sender.
func (ts *LockNotificationTestSuite) SetupTest() {
	ts.setAccountAccess(`{}`)
	ts.Require().NoError(testutils.PutWritableServerConfig(notificationSection, []byte(`{}`)))
	ts.smtp.ClearEmails()
}

func (ts *LockNotificationTestSuite) TearDownSuite() {
	if ts.originalAccountAccess != nil {
		if err := testutils.PutWritableServerConfig(accountAccessSection, ts.originalAccountAccess); err != nil {
			ts.T().Errorf("teardown: failed to restore the accountAccess section: %v", err)
		}
	}
	// Before the sender is deleted, since the section may name it.
	if ts.originalNotification != nil {
		if err := testutils.PutWritableServerConfig(notificationSection, ts.originalNotification); err != nil {
			ts.T().Errorf("teardown: failed to restore the notification section: %v", err)
		}
	}
	if ts.senderID != "" {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			ts.T().Logf("teardown: failed to delete the email provider: %v", err)
		}
	}
	if ts.smtp != nil {
		if err := ts.smtp.Stop(); err != nil {
			ts.T().Logf("teardown: failed to stop the mock SMTP server: %v", err)
		}
	}
}

// The bootstrap stores the lock email templates in the section's writable layer, since they are not
// part of the deployment configuration.
func (ts *LockNotificationTestSuite) TestTheBootstrapStoresTheLockEmailTemplates() {
	var section struct {
		User struct {
			Notifications struct {
				OnLock struct {
					Email map[string]interface{} `json:"email"`
				} `json:"onLock"`
			} `json:"notifications"`
		} `json:"user"`
	}
	ts.Require().NoError(json.Unmarshal(ts.originalAccountAccess, &section))

	email := section.User.Notifications.OnLock.Email
	delete(email, "enabled")
	ts.Equal(shippedLockEmail, email)
}

// The section refuses a template that is not a handle and a method that is not lockable.
func (ts *LockNotificationTestSuite) TestAnInvalidLockTemplateIsRefused() {
	for _, email := range []string{
		`{"templates":{"temporary":"Not A Handle"}}`,
		`{"templates":{"methods":{"passkey":{"temporary":"authentication-method-locked"}}}}`,
	} {
		status, body := putSection(ts.T(), accountAccessSection,
			`{"user":{"notifications":{"onLock":{"email":`+email+`}}}}`)
		ts.Equal(http.StatusBadRequest, status, "%s must be refused: %s", email, body)
		ts.Contains(body, "SCF-1003", "%s must be refused as an invalid value", email)
	}
}

// The recipient is not configurable, so the section refuses a recipient attribute.
func (ts *LockNotificationTestSuite) TestARecipientAttributeIsRefused() {
	status, body := putSection(ts.T(), accountAccessSection,
		`{"user":{"notifications":{"onLock":{"email":{"recipientAttribute":"email"}}}}}`)
	ts.Equal(http.StatusBadRequest, status, "a recipient attribute must be refused: %s", body)
	ts.Contains(body, "SCF-1003", "a recipient attribute must be refused as an unknown field")
}

// Sender resolution belongs to delivery, so the section accepts an unresolved sender.
func (ts *LockNotificationTestSuite) TestAnUnknownSenderIsStoredWithoutResolution() {
	original, err := testutils.GetWritableServerConfig(notificationSection)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() {
		ts.Require().NoError(testutils.PutWritableServerConfig(notificationSection, original))
	})
	const value = `{"defaultEmailSenderId":"00000000-0000-0000-0000-000000000000"}`
	status, body := putSection(ts.T(), notificationSection, value)
	ts.Require().Equal(http.StatusOK, status, "sender resolution must wait until delivery: %s", body)
	stored, err := testutils.GetWritableServerConfig(notificationSection)
	ts.Require().NoError(err)
	ts.JSONEq(value, string(stored))
}

// The section names a sender first, since a read decodes the stored layer and would hide a stored
// unknown key.
func (ts *LockNotificationTestSuite) TestAnUnknownNotificationSettingIsRefusedAndPersistsNothing() {
	senderID, err := testutils.CreateSMTPEmailProvider("Notification Setting Test Provider", "localhost", 2525,
		"noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the email provider")
	defer func() {
		ts.Assert().NoError(testutils.DeleteNotificationSender(senderID), "failed to delete the email provider")
	}()
	ts.Require().NoError(testutils.PutWritableServerConfig(notificationSection,
		[]byte(fmt.Sprintf(`{"defaultEmailSenderId":%q}`, senderID))), "the section must accept an email sender")
	defer func() {
		ts.Assert().NoError(testutils.PutWritableServerConfig(notificationSection, []byte(`{}`)),
			"failed to clear the default sender")
	}()
	before, err := testutils.GetWritableServerConfig(notificationSection)
	ts.Require().NoError(err)

	status, body := putSection(ts.T(), notificationSection, `{"defaultEmailSender":"`+senderID+`"}`)
	ts.Equal(http.StatusBadRequest, status, "a misspelled key must be refused: %s", body)
	ts.Contains(body, "SCF-1003", "a misspelled key must be refused as an invalid value")

	after, err := testutils.GetWritableServerConfig(notificationSection)
	ts.Require().NoError(err)
	ts.JSONEq(string(before), string(after), "a refused write must leave the sender named")
}

// Runs in declarative mode, since outside it the notification service reads no declarative sender.
func (ts *LockNotificationTestSuite) TestADeclarativeSectionCanNameADeclarativeSender() {
	dir := ts.T().TempDir()
	deployment, err := os.ReadFile(filepath.Join(testutils.GetExtractedProductHome(), "deployment.yaml"))
	ts.Require().NoError(err)
	snapshot := filepath.Join(dir, "deployment.yaml")
	ts.Require().NoError(os.WriteFile(snapshot, deployment, 0o600))
	resourcesPath := filepath.Join(dir, "resources.yaml")
	ts.Require().NoError(os.WriteFile(resourcesPath, []byte(declarativeSenderResources), 0o600))
	defer func() {
		ts.Assert().NoError(testutils.UpdateDeploymentConfig(snapshot), "the deployment must be put back")
		ts.Assert().NoError(testutils.RestartServer(), "the server must start again on the deployment it had")
		ts.Assert().NoError(testutils.ObtainAdminAccessToken())
	}()

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"declarative_resources": map[string]interface{}{"enabled": true},
	}))
	ts.Require().NoError(testutils.RestartServerWithResourcesFile(resourcesPath),
		"a declarative section naming a declarative sender must not stop the server starting")
	ts.Require().NoError(testutils.ObtainAdminAccessToken())

	var layers struct {
		ReadOnly struct {
			DefaultEmailSenderID string `json:"defaultEmailSenderId"`
		} `json:"readOnly"`
		Merged struct {
			DefaultEmailSenderID string `json:"defaultEmailSenderId"`
		} `json:"merged"`
	}
	body := getManagement(ts.T(), "/server-config/"+notificationSection)
	ts.Require().NoError(json.Unmarshal(body, &layers), string(body))
	ts.Equal(declarativeSenderID, layers.ReadOnly.DefaultEmailSenderID, "the declarative layer must name the sender")
	ts.Equal(declarativeSenderID, layers.Merged.DefaultEmailSenderID, "the declarative sender must be in force")

	status, response := putSection(ts.T(), notificationSection, `{}`)
	ts.Equal(http.StatusBadRequest, status, "a write over a declarative sender must be refused: %s", response)
	ts.Contains(response, "SCF-1003", "a write over a declarative sender must be refused as an invalid value")
}

// Suspend tells the holder an administrator suspended the account.
func (ts *LockNotificationTestSuite) TestSuspendEmailsTheHolder() {
	_, suspend := copyShippedFlowWithSender(ts.T(), suspendFlowHandle, administrationFlowType, "notify_holder",
		ts.senderID, nil)
	userID, _, address, _ := ts.newUser("suspended")
	runAdministrationFlow(ts.T(), suspend, userID)

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "suspend must email the holder")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Your account has been suspended by an administrator.")
	ts.Contains(email.Body, "Only an administrator can lift the suspension.")
}

// Unsuspend tells the holder the suspension is lifted and the account is still locked.
func (ts *LockNotificationTestSuite) TestUnsuspendEmailsTheHolder() {
	_, unsuspend := copyShippedFlowWithSender(ts.T(), unsuspendFlowHandle, administrationFlowType,
		"notify_holder", ts.senderID, nil)
	userID, _, address, _ := ts.newUser("unsuspended")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.smtp.ClearEmails()
	runAdministrationFlow(ts.T(), unsuspend, userID)

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "unsuspend must email the holder")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "The suspension on your account has been lifted")
	ts.Contains(email.Body, "Your account is still locked.")
}

// setAccountAccess writes the writable accountAccess layer as this package's policy and the shipped
// lock email templates, with body laid over them; SetupTest resets it.
func (ts *LockNotificationTestSuite) setAccountAccess(body string) {
	ts.T().Helper()

	var override map[string]interface{}
	ts.Require().NoError(json.Unmarshal([]byte(body), &override), "unreadable override %s", body)
	templates := map[string]interface{}{"user": map[string]interface{}{"notifications": map[string]interface{}{
		"onLock": map[string]interface{}{"email": shippedLockEmail}}}}
	layer, err := json.Marshal(mergeJSONObjects(lockoutPolicyWith(templates), override))
	ts.Require().NoError(err)
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, layer),
		"the accountAccess section must accept %s", body)
}

// newUser creates one account of the fixture's type, whose primary email is its email address.
func (ts *LockNotificationTestSuite) newUser(name string) (userID, username, address, password string) {
	ts.T().Helper()
	return ts.newUserOfType(ts.users.userType, name, nil)
}

// newUserOfType creates one account of the given user type with an email address and any extra
// attributes.
func (ts *LockNotificationTestSuite) newUserOfType(userType, name string, extra map[string]interface{}) (
	userID, username, address, password string) {
	ts.T().Helper()

	address = ts.users.usernameFor(name) + "@example.com"
	attributes := map[string]interface{}{"email": address}
	for key, value := range extra {
		attributes[key] = value
	}
	userID, username, password = ts.users.newUserOfType(ts.T(), userType, name, attributes)
	return userID, username, address, password
}
