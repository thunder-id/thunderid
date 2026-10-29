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
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// notificationSection is the server-config section naming the default email sender.
const notificationSection = "notification"

// The user types the suite's accounts are of, besides the fixture's own, which maps its primary email
// to email: one maps it to workEmail, and one has no primary email, its email attribute plain text.
const (
	lockNotificationPrimaryType   = "governance_lock_notification_primary"
	lockNotificationNoPrimaryType = "governance_lock_notification_no_primary"
)

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

// notifiedRungSeconds is a lock that outlasts every test here, so nothing lapses between forming it
// and reading its email.
const notifiedRungSeconds = 300

// noEmailWait is how long a test watches for an email it expects never to be sent where no later
// email can serve as a barrier: the settings that stop the email are read again at delivery, so a
// barrier email would need settings that could also let the watched one through. It matches the wait
// for an email that is sent, and it cannot prove no email is ever sent, only none within it.
const noEmailWait = deliveryTimeout

// declarativeSenderID is the email sender declarativeSenderResources declares.
const declarativeSenderID = "governance-declarative-email-sender"

// declarativeSenderResources declares an email sender and a notification section naming it.
const declarativeSenderResources = `resource_type: organization_unit
id: governance-declarative-ou
handle: governance-declarative-ou
name: Governance Declarative OU
description: Owns the declarative sender
---
resource_type: connection
id: ` + declarativeSenderID + `
ouHandle: governance-declarative-ou
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

// LockNotificationTestSuite checks the holder emails end to end: an automatic lock emails the holder
// through the notification section's default sender as the accountAccess section configures it, and
// never at the cost of the lock. The suspend, unsuspend and unlock flows email through the sender
// their notify_holder node names.
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
		SystemAttributes: &testutils.UserTypeSystemAttributes{PrimaryEmail: "email"},
		Schema:           credentialsSchema(map[string]interface{}{"email": emailAttribute}),
	})
	ts.users.addUserType(ts.T(), testutils.UserType{
		Handle:           lockNotificationPrimaryType,
		SystemAttributes: &testutils.UserTypeSystemAttributes{PrimaryEmail: "workEmail"},
		Schema: credentialsSchema(map[string]interface{}{
			"email": emailAttribute, "workEmail": emailAttribute,
		}),
	})
	ts.users.addUserType(ts.T(), testutils.UserType{
		Handle: lockNotificationNoPrimaryType,
		Schema: credentialsSchema(map[string]interface{}{"email": stringAttribute}),
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

// A lock sends the authentication-method-locked email to the holder through the default sender, filled
// with the committed episode's facts, and a further failure during the live lock sends no second one.
func (ts *LockNotificationTestSuite) TestALockEmailsTheHolderThroughTheDefaultSender() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	userID, username, address, password := ts.newUser("sent")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the lock must form")

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "a lock must send the authentication-method-locked email")
	ts.Equal([]string{address}, email.To, "the email goes to the holder's email attribute")
	ts.Contains(email.Body, "Sign-in was temporarily locked on your account",
		"the authentication-method-locked template's subject")
	ts.Contains(email.Body, "The credential sign-in method on your account has been temporarily locked",
		"the template is filled with the locked sign-in method")
	ts.Contains(email.Body, fmt.Sprintf("It will be available again in 5 minutes, on %s UTC.",
		expiry.UTC().Format("2006-01-02 15:04")),
		"the template is filled with the episode's duration and its expiry")

	failAuthentication(ts.T(), "username", username, 1)
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(), "the lock must still hold")
	ts.requireOnlyABarrierEmail("live_lock", "attempts during the live lock must send no second email")
}

// A lock that does not expire renders the permanent template, which states no expiry.
func (ts *LockNotificationTestSuite) TestAPermanentLockSendsThePermanentTemplate() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(`{"user":{"default":{"lockDurationsSeconds":[0]}}}`)

	userID, username, address, _ := ts.newUser("permanent")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	status := readStatus(ts.T(), userID)
	ts.Require().Equal([]string{"credential"}, status.lockedMethods(), "the lock must form")
	_, expires := status.expiryOf("credential")
	ts.Require().False(expires, "a rung of zero must form a lock with no expiry")

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "a permanent lock must send an email")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Sign-in was locked on your account", "the permanent template's subject")
	ts.Contains(email.Body, "The credential sign-in method on your account has been locked")
	ts.Contains(email.Body, "This lock does not expire automatically.")
	ts.NotContains(email.Body, "available again", "a permanent lock must state no expiry")
}

// A lock on every sign-in method sends the account template, which names no method.
func (ts *LockNotificationTestSuite) TestAnAccountWideLockSendsTheAccountTemplate() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"lockGranularity":"account",`+
		`"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	userID, username, _, _ := ts.newUser("account")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock must form")

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "an account-wide lock must send an email")
	ts.Contains(email.Body, "Sign-in to your account has been temporarily locked")
	ts.NotContains(email.Body, "sign-in method on your account", "the account template names no method")
}

// A lock on every sign-in method that does not expire sends the permanent account template, which names
// no method and states no expiry.
func (ts *LockNotificationTestSuite) TestAPermanentAccountWideLockSendsThePermanentAccountTemplate() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(`{"user":{"lockGranularity":"account","default":{"lockDurationsSeconds":[0]}}}`)

	userID, username, address, _ := ts.newUser("account_permanent")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	status := readStatus(ts.T(), userID)
	ts.Require().Equal([]string{"account"}, status.lockedMethods(), "the account-wide lock must form")
	_, expires := status.expiryOf("account")
	ts.Require().False(expires, "a rung of zero must form a lock with no expiry")

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "a permanent account-wide lock must send an email")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Sign-in to your account has been locked due to multiple failed attempts.")
	ts.Contains(email.Body, "This lock does not expire automatically.")
	ts.NotContains(email.Body, "sign-in method on your account", "the account template names no method")
	ts.NotContains(email.Body, "temporarily", "a permanent lock must not be described as temporary")
}

// A template set for one method replaces the default for a lock on that method.
func (ts *LockNotificationTestSuite) TestAMethodTemplateReplacesTheDefault() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]},"notifications":`+
		`{"onLock":{"email":{"templates":{"methods":{"credential":{"temporary":"account-locked"}}}}}}}}`,
		notifiedRungSeconds))

	_, username, _, _ := ts.newUser("method_template")
	failAuthentication(ts.T(), "username", username, lockThreshold)

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "the lock must send an email")
	ts.Contains(email.Body, "Sign-in to your account has been temporarily locked", "the credential template")
}

// With no template for the lock, the lock forms and nothing is sent.
func (ts *LockNotificationTestSuite) TestWithoutATemplateNothingIsSent() {
	ts.useDefaultSender(ts.senderID)
	layer, err := json.Marshal(lockoutPolicyWith(userDefaults(map[string]interface{}{
		"lockDurationsSeconds": []int{notifiedRungSeconds},
	})))
	ts.Require().NoError(err)
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, layer))

	userID, username, _, _ := ts.newUser("no_template")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock forms without a template")

	ts.requireNoEmail("no email is sent without a template")
}

// A template name that names no template is accepted, since templates are created separately; the
// lock forms and is enforced, and nothing is sent.
func (ts *LockNotificationTestSuite) TestATemplateThatDoesNotExistSendsNothing() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]},"notifications":`+
		`{"onLock":{"email":{"templates":{"temporary":"governance-no-such-template"}}}}}}`, notifiedRungSeconds))

	userID, username, _, password := ts.newUser("missing_template")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock must form")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(), "the lock must be enforced")

	ts.requireNoEmail("a template that does not exist must send nothing")
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

// The lock email goes to the user type's primary email even when an email attribute also exists, and
// an account with no value for it is not emailed at its other address.
func (ts *LockNotificationTestSuite) TestTheLockEmailGoesOnlyToThePrimaryEmail() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	workAddress := fmt.Sprintf("notify_primary_%s@work.example.com", ts.users.suffix)
	_, username, _, _ := ts.newUserOfType(lockNotificationPrimaryType, "primary",
		map[string]interface{}{"workEmail": workAddress})
	failAuthentication(ts.T(), "username", username, lockThreshold)

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "the lock must email the primary email")
	ts.Equal([]string{workAddress}, email.To, "the email goes to the primary email")

	userID, username, _, _ := ts.newUserOfType(lockNotificationPrimaryType, "no_primary_value", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock must form")
	ts.requireOnlyABarrierEmail("no_primary_value", "an account with no primary email value must not be emailed")
}

// A user type with no primary email sends no lock email, even to an email attribute, and the lock
// still forms.
func (ts *LockNotificationTestSuite) TestAUserTypeWithoutAPrimaryEmailSendsNothing() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	userID, username, _, _ := ts.newUserOfType(lockNotificationNoPrimaryType, "no_primary_type", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock must form")
	ts.requireOnlyABarrierEmail("no_primary_type", "a type without a primary email must send no lock email")
}

// The recipient is not configurable, so the section refuses a recipient attribute.
func (ts *LockNotificationTestSuite) TestARecipientAttributeIsRefused() {
	status, body := putSection(ts.T(), accountAccessSection,
		`{"user":{"notifications":{"onLock":{"email":{"recipientAttribute":"email"}}}}}`)
	ts.Equal(http.StatusBadRequest, status, "a recipient attribute must be refused: %s", body)
	ts.Contains(body, "SCF-1003", "a recipient attribute must be refused as an unknown field")
}

// With the lock email switched off the lock forms and nothing is sent. The setting is checked before
// a notice is queued, so a lock formed once it is back on can serve as the barrier.
func (ts *LockNotificationTestSuite) TestADisabledLockEmailSendsNothing() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]},`+
		`"notifications":{"onLock":{"email":{"enabled":false}}}}}`, notifiedRungSeconds))

	userID, username, _, _ := ts.newUser("disabled")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock forms with the email off")

	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))
	ts.requireOnlyABarrierEmail("disabled", "no email is sent when it is switched off")
}

// Without a default sender the lock still forms and no email is sent.
func (ts *LockNotificationTestSuite) TestWithoutADefaultSenderNothingIsSent() {
	userID, username, _, _ := ts.newUser("unsent")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock forms without a sender")

	ts.requireNoEmail("no email is sent without a default sender")
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

// With a mail server that cannot be reached, the lock forms, is enforced and is released
// exactly as it would be with no email at all.
func (ts *LockNotificationTestSuite) TestAnUnreachableMailServerDoesNotChangeTheLock() {
	closed := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(closed.Start(), "failed to reserve a port")
	unreachablePort := closed.GetPort()
	ts.Require().NoError(closed.Stop(), "failed to close the reserved port")

	unreachableID, err := testutils.CreateSMTPEmailProvider("Lock Notification Unreachable Provider",
		"localhost", unreachablePort, "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the unreachable email provider")
	defer func() {
		if err := testutils.PutWritableServerConfig(notificationSection, []byte(`{}`)); err != nil {
			ts.T().Errorf("cleanup: failed to clear the notification section: %v", err)
		}
		if err := testutils.DeleteNotificationSender(unreachableID); err != nil {
			ts.T().Logf("cleanup: failed to delete the unreachable provider: %v", err)
		}
	}()
	ts.useDefaultSender(unreachableID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	userID, username, _, password := ts.newUser("unreachable")
	failAuthentication(ts.T(), "username", username, lockThreshold)

	ts.Equal(notifiedRungSeconds*time.Second, servedRung(ts.T(), userID, "credential"),
		"the lock must serve its rung")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lock must be enforced")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lock must release as usual")
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

// An administrator unlock tells the holder they can sign in again.
func (ts *LockNotificationTestSuite) TestUnlockEmailsTheHolder() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	_, unlock := copyShippedFlowWithSender(ts.T(), unlockFlowHandle, administrationFlowType, "notify_holder",
		ts.senderID, nil)
	userID, username, address, _ := ts.newUser("unlocked")
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value)
	awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "the lock email arrives first")
	ts.smtp.ClearEmails()
	runAdministrationFlow(ts.T(), unlock, userID)

	email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "unlock must email the holder")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Your account has been unlocked")
}

// requireOnlyABarrierEmail forms a lock on a fresh account under the settings in force, which must
// email it, and requires that email to be the only one captured. The barrier's notice is queued after
// any the test's own lock queued, so the notifier picks the watched one up first; it runs two delivery
// workers, so the barrier orders the two pick-ups, not the two sends.
func (ts *LockNotificationTestSuite) requireOnlyABarrierEmail(label, msg string) {
	ts.T().Helper()

	_, username, address, _ := ts.newUser("barrier_" + label)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Eventually(func() bool {
		for _, email := range ts.smtp.GetEmails() {
			if len(email.To) == 1 && email.To[0] == address {
				return true
			}
		}
		return false
	}, deliveryTimeout, 100*time.Millisecond, "the barrier lock must email its holder")

	emails := ts.smtp.GetEmails()
	ts.Len(emails, 1, "%s: only the barrier's email may have been sent, got %+v", msg, emails)
}

// requireNoEmail watches for noEmailWait and requires no email to arrive.
func (ts *LockNotificationTestSuite) requireNoEmail(msg string) {
	ts.T().Helper()
	ts.Never(func() bool { return len(ts.smtp.GetEmails()) > 0 }, noEmailWait, 100*time.Millisecond, msg)
}

// useDefaultSender names the notification section's default email sender.
func (ts *LockNotificationTestSuite) useDefaultSender(senderID string) {
	ts.T().Helper()
	ts.Require().NoError(testutils.PutWritableServerConfig(notificationSection,
		[]byte(fmt.Sprintf(`{"defaultEmailSenderId":%q}`, senderID))),
		"the notification section must accept an existing email sender")
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
