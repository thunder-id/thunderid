// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// notificationSection is the server-config section naming the default email sender.
const notificationSection = "notification"

// LockNotificationTestSuite checks the holder emails end to end, labelled LE (lock and lifecycle
// email): an automatic lock emails the holder through the notification section's default sender as
// the accountAccess section configures it, and never at the cost of the lock. The suspend, unsuspend
// and unlock flows email through the sender their notify_holder node names.
type LockNotificationTestSuite struct {
	suite.Suite
	smtp                  *testutils.MockSMTPServer
	senderID              string
	ouID                  string
	entityTypeID          string
	userIDs               []string
	suffix                string
	originalNotification  json.RawMessage
	originalAccountAccess json.RawMessage
}

func TestLockNotificationTestSuite(t *testing.T) {
	suite.Run(t, new(LockNotificationTestSuite))
}

var lockNotificationOU = testutils.OrganizationUnit{
	Handle:      "governance_lock_notification_ou",
	Name:        "Governance Lock Notification Test OU",
	Description: "Organization unit for the lock notification tests",
}

var lockNotificationUserType = testutils.UserType{
	Handle:      "governance_lock_notification_user",
	DisplayName: "governance_lock_notification_user",
	Schema: map[string]interface{}{
		"username":  map[string]interface{}{"type": "string"},
		"email":     map[string]interface{}{"type": "string"},
		"workEmail": map[string]interface{}{"type": "string"},
		"password":  map[string]interface{}{"type": "string", "credential": true},
	},
}

// notifiedRungSeconds is a lock that outlasts every test here, so nothing lapses between forming it
// and reading its email.
const notifiedRungSeconds = 300

// noEmailWait is how long a test waits before concluding that no email was sent; it matches the
// wait for an email that is sent.
const noEmailWait = 5 * time.Second

func (ts *LockNotificationTestSuite) SetupSuite() {
	// The default.json lock email is on; each test layers its policy over this package's.
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

	ouID, err := testutils.CreateOrganizationUnit(lockNotificationOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	lockNotificationUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(lockNotificationUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.suffix = uniqueSuffix()
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
	if ts.senderID != "" {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			ts.T().Logf("teardown: failed to delete the email provider: %v", err)
		}
	}
	if ts.smtp != nil {
		_ = ts.smtp.Stop()
	}
}

// A lock sends the account-locked email to the holder through the default sender, filled
// with the committed episode's facts, and a further failure during the live lock sends no second one.
func (ts *LockNotificationTestSuite) TestALockEmailsTheHolderThroughTheDefaultSender() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	userID, username, address, password := ts.newUser("sent", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the lock must form")

	email := awaitEmail(ts.smtp, 5*time.Second)
	ts.Require().NotNil(email, "a lock must send the account-locked email")
	ts.Equal([]string{address}, email.To, "the email goes to the holder's email attribute")
	ts.Contains(email.Body, "Sign-in was temporarily locked on your account",
		"the account-locked template's subject")
	ts.Contains(email.Body, "Credential sign-in has been temporarily locked",
		"the template is filled with the locked sign-in method")
	ts.Contains(email.Body, fmt.Sprintf("It will be available again in 5 minutes, on %s UTC.",
		expiry.UTC().Format("2006-01-02 15:04")),
		"the template is filled with the episode's duration and its expiry")

	failAuthentication(ts.T(), "username", username, 1)
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(), "the lock must still hold")
	ts.Nil(awaitEmail(ts.smtp, noEmailWait), "attempts during the live lock must send no second email")
}

// A lock that does not expire renders the permanent template, which states no expiry.
func (ts *LockNotificationTestSuite) TestAPermanentLockSendsThePermanentTemplate() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(`{"user":{"default":{"lockDurationsSeconds":[0]}}}`)

	userID, username, address, _ := ts.newUser("permanent", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	status := readStatus(ts.T(), userID)
	ts.Require().Equal([]string{"credential"}, status.lockedScopes(), "the lock must form")
	_, expires := status.expiryOf("credential")
	ts.Require().False(expires, "a rung of zero must form a lock with no expiry")

	email := awaitEmail(ts.smtp, 5*time.Second)
	ts.Require().NotNil(email, "a permanent lock must send an email")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Sign-in was locked on your account", "the permanent template's subject")
	ts.Contains(email.Body, "This lock does not expire automatically.")
	ts.NotContains(email.Body, "available again", "a permanent lock must state no expiry")
}

// The recipient is the configured attribute on the holder's record, and an account without that
// attribute is not emailed at its other address.
func (ts *LockNotificationTestSuite) TestTheConfiguredRecipientAttributeIsUsed() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]},`+
		`"notifications":{"onLock":{"email":{"recipientAttribute":"workEmail"}}}}}`, notifiedRungSeconds))

	workAddress := fmt.Sprintf("notify_work_%s@work.example.com", ts.suffix)
	_, username, _, _ := ts.newUser("work_address", map[string]interface{}{"workEmail": workAddress})
	failAuthentication(ts.T(), "username", username, lockThreshold)

	email := awaitEmail(ts.smtp, 5*time.Second)
	ts.Require().NotNil(email, "the lock must email the configured attribute")
	ts.Equal([]string{workAddress}, email.To, "the email goes to the configured attribute")

	userID, username, _, _ := ts.newUser("no_work_address", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock must form")
	ts.Nil(awaitEmail(ts.smtp, noEmailWait), "a named attribute the record lacks must never fall back")
}

// With the lock email switched off the lock forms and nothing is sent.
func (ts *LockNotificationTestSuite) TestADisabledLockEmailSendsNothing() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]},`+
		`"notifications":{"onLock":{"email":{"enabled":false}}}}}`, notifiedRungSeconds))

	userID, username, _, _ := ts.newUser("disabled", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock forms with the email off")

	ts.Nil(awaitEmail(ts.smtp, noEmailWait), "no email is sent when it is switched off")
}

// Without a default sender the lock still forms and no email is sent.
func (ts *LockNotificationTestSuite) TestWithoutADefaultSenderNothingIsSent() {
	userID, username, _, _ := ts.newUser("unsent", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the lock forms without a sender")

	ts.Nil(awaitEmail(ts.smtp, noEmailWait), "no email is sent without a default sender")
}

// The section refuses a sender that does not exist, as a client error.
func (ts *LockNotificationTestSuite) TestAnUnknownSenderIsRefused() {
	status, body := ts.putSection(notificationSection,
		`{"defaultEmailSenderId":"00000000-0000-0000-0000-000000000000"}`)
	ts.Equal(http.StatusBadRequest, status, "an unknown sender must be refused: %s", body)
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

	userID, username, _, password := ts.newUser("unreachable", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)

	expiry, reported := readStatus(ts.T(), userID).expiryOf("credential")
	ts.Require().True(reported, "the lock must form")
	ts.WithinDuration(time.Now().Add(notifiedRungSeconds*time.Second), expiry, 30*time.Second,
		"the lock must serve its rung")
	ts.False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lock must be enforced")

	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lock must release as usual")
}

// Suspend tells the holder an administrator suspended the account.
func (ts *LockNotificationTestSuite) TestSuspendEmailsTheHolder() {
	defer nameFlowSender(ts.T(), suspendFlowHandle, administrationFlowType, "notify_holder", ts.senderID)()
	userID, _, address, _ := ts.newUser("suspended", nil)
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	email := awaitEmail(ts.smtp, 5*time.Second)
	ts.Require().NotNil(email, "suspend must email the holder")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Your account has been suspended by an administrator.")
	ts.Contains(email.Body, "Only an administrator can lift the suspension.")
}

// Unsuspend tells the holder the suspension is lifted and the account is still locked.
func (ts *LockNotificationTestSuite) TestUnsuspendEmailsTheHolder() {
	defer nameFlowSender(ts.T(), unsuspendFlowHandle, administrationFlowType, "notify_holder", ts.senderID)()
	userID, _, address, _ := ts.newUser("unsuspended", nil)
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.smtp.ClearEmails()
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)

	email := awaitEmail(ts.smtp, 5*time.Second)
	ts.Require().NotNil(email, "unsuspend must email the holder")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "The suspension on your account has been lifted")
	ts.Contains(email.Body, "Your account is still locked.")
}

// An administrator unlock tells the holder they can sign in again.
func (ts *LockNotificationTestSuite) TestUnlockEmailsTheHolder() {
	ts.useDefaultSender(ts.senderID)
	ts.setAccountAccess(fmt.Sprintf(`{"user":{"default":{"lockDurationsSeconds":[%d]}}}`, notifiedRungSeconds))

	defer nameFlowSender(ts.T(), unlockFlowHandle, administrationFlowType, "notify_holder", ts.senderID)()
	userID, username, address, _ := ts.newUser("unlocked", nil)
	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value)
	ts.Require().NotNil(awaitEmail(ts.smtp, 5*time.Second), "the lock email arrives first")
	ts.smtp.ClearEmails()
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)

	email := awaitEmail(ts.smtp, 5*time.Second)
	ts.Require().NotNil(email, "unlock must email the holder")
	ts.Equal([]string{address}, email.To)
	ts.Contains(email.Body, "Your account has been unlocked")
}

// useDefaultSender names the notification section's default email sender.
func (ts *LockNotificationTestSuite) useDefaultSender(senderID string) {
	ts.T().Helper()
	ts.Require().NoError(testutils.PutWritableServerConfig(notificationSection,
		[]byte(fmt.Sprintf(`{"defaultEmailSenderId":%q}`, senderID))),
		"the notification section must accept an existing email sender")
}

// setAccountAccess writes the writable accountAccess layer as this package's policy with body laid
// over it; SetupTest resets it.
func (ts *LockNotificationTestSuite) setAccountAccess(body string) {
	ts.T().Helper()

	var override map[string]interface{}
	ts.Require().NoError(json.Unmarshal([]byte(body), &override), "unreadable override %s", body)
	layer, err := json.Marshal(mergeJSONObjects(
		accountAccessPatch()["accountAccess"].(map[string]interface{}), override))
	ts.Require().NoError(err)
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, layer),
		"the accountAccess section must accept %s", body)
}

// putSection writes a section's writable layer and returns the status and the body, for a write that
// is expected to be refused.
func (ts *LockNotificationTestSuite) putSection(section, body string) (int, string) {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodPut, testServerURL+"/server-config/"+section,
		bytes.NewReader([]byte(body)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody)
}

// newUser creates one account with an email address and any extra attributes.
func (ts *LockNotificationTestSuite) newUser(name string, extra map[string]interface{}) (
	userID, username, address, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("notify_%s_%s", name, ts.suffix)
	address = username + "@example.com"
	password = "Correct-Horse-" + name + "1!"
	attributes := map[string]interface{}{
		"username": username,
		"email":    address,
		"password": password,
	}
	for key, value := range extra {
		attributes[key] = value
	}
	raw, err := json.Marshal(attributes)
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       lockNotificationUserType.Handle,
		OUID:       ts.ouID,
		Attributes: raw,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, address, password
}
