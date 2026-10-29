// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package governance checks the account lock and suspension behaviour against a running server.
// Tests check stated behaviour, not the implementation that satisfies it.
package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL          = testutils.TestServerURL
	administrationFlowType = "ADMINISTRATION"

	// subjectInput is the input an administration flow names its target with.
	subjectInput = "subject"

	// operatorNoteInput is the optional free text a suspend flow may carry. Optional at every layer, so a
	// request that omits it suspends with no note recorded.
	operatorNoteInput = "operatorNote"

	// pristineDeploymentPath is the deployment the harness copies in at start-up, relative to this
	// package. Restoring it is how a deployment change this package makes is undone.
	pristineDeploymentPath = "../resources/deployment.yaml"

	// shippedUserLockThreshold is the user lock threshold in default.json's account_access section.
	shippedUserLockThreshold = 5
)

// The shipped administration flows, addressed by handle so the tests do not depend on a seeded id.
// Applications have no unlock flow because an application never locks.
const (
	unlockFlowHandle    = "default-user-unlock-flow"
	suspendFlowHandle   = "default-user-suspend-flow"
	unsuspendFlowHandle = "default-user-unsuspend-flow"

	agentUnlockFlowHandle    = "default-agent-unlock-flow"
	agentSuspendFlowHandle   = "default-agent-suspend-flow"
	agentUnsuspendFlowHandle = "default-agent-unsuspend-flow"

	applicationSuspendFlowHandle   = "default-application-suspend-flow"
	applicationUnsuspendFlowHandle = "default-application-unsuspend-flow"
)

// The shipped sign-in, registration and recovery flows: their handle, and the action the shipped
// sign-in prompt submits the username and password with.
const (
	defaultFlowHandle   = "default-flow"
	shippedSignInAction = "action_001"
)

// The console the root operator signs in to, and the bootstrap organization unit and system resource
// server the root permission lives in.
const (
	boundaryConsoleURI     = "https://localhost:8095/console"
	bootstrapOUID          = "01900000-0000-7000-8000-000000000001"
	systemResourceServerID = "01900000-0000-7000-8000-000000000020"
)

// suspensionCutoffLeeway is jwt.leeway as every suite in this package runs it, shorter than the shipped
// 30 seconds so the waits past a suspension's cutoff stay short, yet long enough for a release and a
// sign-in to fit inside it on a slow runner. A suspension's revocation cutoff is its pre-suspend time
// plus this allowance.
const suspensionCutoffLeeway = 15 * time.Second

// denyListRefreshWait bounds how long a native API takes to refuse a revoked token: the deployment
// refreshes its copy of the deny list every two seconds.
const denyListRefreshWait = 10 * time.Second

// The management collections a governed identity is read back through. The effective status is
// public API on all three.
const (
	userCollection        = "users"
	agentCollection       = "agents"
	applicationCollection = "applications"
)

// The policy this package runs under, written to the section's writable layer for a suite's duration.
// Smaller than the shipped defaults so lockouts are quick and a rung lapses in seconds; the
// second rung is longer than the first so escalation is observable.
const (
	lockThreshold      = 3
	firstRungSeconds   = 8
	secondRungSeconds  = 30
	failureWindowSecs  = 600
	lockDecaySeconds   = 3600
	rungLapseTolerance = 3 * time.Second
)

// The rungs of longRungLockoutPolicy, for suites that never wait for a lock to lapse: the first rung
// outlasts every test, and the second is far from it, so the rung a lock serves is told apart.
const (
	longFirstRungSeconds  = 300
	longSecondRungSeconds = 900
)

// Stated holds, as the direct authentication endpoints answer them.
const (
	statedLockCode       = "AUTHN-1011"
	statedSuspensionCode = "AUTHN-1012"
)

// Stated holds, as the flow executors answer them.
const (
	flowStatedSuspensionCode = "FET-1114"
	flowStatedLockCode       = "FET-1113"
)

const (
	// deliveryTimeout bounds the wait for a message the server is expected to send.
	deliveryTimeout = 5 * time.Second

	// statusPollInterval is how often a wait on a status read reads it again.
	statusPollInterval = 250 * time.Millisecond
)

var (
	// serverFault is the answer a flow gives when a node faults.
	serverFault = flowAnswer{status: http.StatusInternalServerError, code: "SSE-5000"}

	// governanceRefusal is the answer a lifecycle step gives when the authority refuses it.
	governanceRefusal = flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: "FET-1112"}

	// stringAttribute is a plain string attribute in a user type schema.
	stringAttribute = map[string]interface{}{"type": "string"}

	// heldRefusalLeak matches what a stated hold must never say: a date, or a count of seconds, minutes,
	// hours or attempts, which would tell when the lock ends, which rung it serves or how many attempts remain.
	heldRefusalLeak = regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d+ (second|minute|hour|attempt)`)
)

// lockoutPolicy is this package's accountAccess policy. Every sign-in method counts under its own key
// with the category default.
func lockoutPolicy() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"lockGranularity": "authentication_method",
			"default": map[string]interface{}{
				"enabled":              true,
				"threshold":            lockThreshold,
				"failureWindowSeconds": failureWindowSecs,
				"lockDurationsSeconds": []int{firstRungSeconds, secondRungSeconds},
				"lockDecaySeconds":     lockDecaySeconds,
			},
		},
	}
}

// lockoutPolicyWith is this package's policy with override laid over it.
func lockoutPolicyWith(override map[string]interface{}) map[string]interface{} {
	return mergeJSONObjects(lockoutPolicy(), override)
}

// userDefaults is an override of the user category's default policy fields.
func userDefaults(fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"user": map[string]interface{}{"default": fields}}
}

// longRungLockoutPolicy is the package policy with rungs that do not lapse during a test.
func longRungLockoutPolicy() map[string]interface{} {
	return lockoutPolicyWith(userDefaults(map[string]interface{}{
		"lockDurationsSeconds": []int{longFirstRungSeconds, longSecondRungSeconds},
	}))
}

// nonExpiringRung is a user policy override whose one rung never lapses.
func nonExpiringRung() map[string]interface{} {
	return map[string]interface{}{"lockDurationsSeconds": []int{0}}
}

// authResult is everything an authentication attempt returns that a criterion can be written about.
type authResult struct {
	status int
	body   string
}

// succeeded reports whether the attempt was admitted.
func (r authResult) succeeded() bool { return r.status == http.StatusOK }

// authenticate presents one password to the atomic credentials endpoint.
func authenticate(t *testing.T, identifierKey, identifierValue, credential string) authResult {
	t.Helper()

	status, body, err := testutils.AuthenticateWithCredentialRaw(identifierKey, identifierValue, "password",
		credential)
	require.NoError(t, err, "authentication request failed")
	return authResult{status: status, body: string(body)}
}

// requireStatedHold checks a held account's refusal against a wrong secret's on an unheld one: both
// are refused with the same status, and the held one states the hold by code. It also requires the
// refusal to name neither the method nor when the lock ends: not the method's identifier as a word, not
// "unlock", no date, and no count of seconds, minutes, hours or attempts.
func requireStatedHold(t *testing.T, wrong, held authResult, code, method string) {
	t.Helper()
	require.NotEqual(t, http.StatusOK, wrong.status, "the comparison is only meaningful against a refusal")
	require.False(t, held.succeeded(), "a held account must be refused")
	require.Equal(t, wrong.status, held.status, "a stated hold keeps the status a refused sign-in has")
	require.Contains(t, held.body, code, "the refusal must state the hold: %s", held.body)
	require.NotRegexp(t, `\b`+regexp.QuoteMeta(method)+`\b`, held.body, "the refusal must not name the %s method", method)
	require.NotContains(t, held.body, "unlock", "the refusal must not say when the lock ends")
	require.NotRegexp(t, heldRefusalLeak, held.body, "the refusal must not give a time, a rung or a count")
}

// requireRefusedAsHeld requires an attempt on a held account to be refused with the hold stated by
// code, so a fault cannot pass for the hold.
func requireRefusedAsHeld(t *testing.T, res authResult, code, msg string) {
	t.Helper()
	require.False(t, res.succeeded(), "%s: the attempt was admitted", msg)
	require.Contains(t, res.body, code, "%s: the refusal must state the hold", msg)
}

// failAuthentication presents a wrong secret count times, requires every attempt to be refused and
// returns the refusals.
func failAuthentication(t *testing.T, identifierKey, identifierValue string, count int) []authResult {
	t.Helper()
	refusals := make([]authResult, 0, count)
	for i := range count {
		res := authenticate(t, identifierKey, identifierValue, "DefinitelyNotTheSecret!"+fmt.Sprint(i))
		require.False(t, res.succeeded(), "attempt %d was expected to be refused", i+1)
		refusals = append(refusals, res)
	}
	return refusals
}

// failHeldAuthentication presents a wrong secret count times to a held account and requires every
// attempt to state the hold by code, as the correct secret's refusal does.
func failHeldAuthentication(t *testing.T, identifierKey, identifierValue string, count int, code string) {
	t.Helper()
	for i, res := range failAuthentication(t, identifierKey, identifierValue, count) {
		require.Contains(t, res.body, code, "wrong secret %d on a held account must state the hold: %s",
			i+1, res.body)
	}
}

// runtimeColumn returns one column of the entity's runtime row, or an expression over its columns.
func runtimeColumn(t *testing.T, entityID, column string) string {
	t.Helper()
	value, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT %s FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, column, entityID))
	require.NoError(t, err, "failed to read the runtime row")
	return value
}

// runtimeRowCount counts the entity's runtime rows.
func runtimeRowCount(t *testing.T, entityID string) int {
	t.Helper()
	count, err := strconv.Atoi(runtimeColumn(t, entityID, "COUNT(*)"))
	require.NoError(t, err)
	return count
}

// setRuntimeAttributes writes the runtime document of the entity's row directly. The CLI behind
// QueryRuntimeDB takes no parameters, so the document is quoted as an SQL string literal.
func setRuntimeAttributes(t *testing.T, entityID, document string) error {
	t.Helper()
	_, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = '%s' WHERE ENTITY_ID = '%s';`,
		strings.ReplaceAll(document, "'", "''"), entityID))
	return err
}

// criteriaRowCount counts the revocation criteria rows of one type naming the value. A suspension's
// containment writes them before the hold, so none for its target means containment never ran.
func criteriaRowCount(t *testing.T, criterionType, value string) int {
	t.Helper()
	raw, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "REVOCATION_CRITERIA" WHERE CRITERION_TYPE = '%s' AND CRITERION_VALUE = '%s';`,
		criterionType, value))
	require.NoError(t, err, "failed to read the revocation criteria")
	count, err := strconv.Atoi(raw)
	require.NoError(t, err, "unreadable revocation criteria count %q", raw)
	return count
}

// runtimeRevision returns the revision of the entity's runtime row. Every runtime write, a counted
// failure included, moves it.
func runtimeRevision(t *testing.T, entityID string) string {
	t.Helper()
	revision := runtimeColumn(t, entityID, "REVISION")
	require.NotEmpty(t, revision, "the entity must have a runtime row")
	return revision
}

// servedRung returns the duration of the lock the runtime row holds for a method ("account" for the
// account-wide entry): its unlock time less the failure that formed it, both written from one clock
// reading, so the answer does not depend on how long the test took to read it.
func servedRung(t *testing.T, entityID, method string) time.Duration {
	t.Helper()
	unlockAt, failedAt := storedLock(t, entityID, method)
	return unlockAt.Sub(failedAt)
}

// storedExpiry returns the unlock time the runtime row holds for a method. Unlike the status read, it
// is still there once the lock has lapsed, so a slow read cannot miss it.
func storedExpiry(t *testing.T, entityID, method string) time.Time {
	t.Helper()
	unlockAt, _ := storedLock(t, entityID, method)
	return unlockAt
}

// storedLock returns the unlock time of the lock the runtime row holds for a method ("account" for the
// account-wide entry), and the time of the failure that formed it.
func storedLock(t *testing.T, entityID, method string) (time.Time, time.Time) {
	t.Helper()
	raw := runtimeColumn(t, entityID, "RUNTIME_ATTRIBUTES")

	type lockEntry struct {
		UnlockAt     string `json:"unlockAt"`
		LastFailedAt string `json:"lastFailedAt"`
	}
	var row struct {
		AccessState struct {
			Lock struct {
				Account               *lockEntry           `json:"account"`
				AuthenticationMethods map[string]lockEntry `json:"authenticationMethods"`
			} `json:"lock"`
		} `json:"accessState"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &row), "unreadable runtime row: %s", raw)

	entry, recorded := row.AccessState.Lock.AuthenticationMethods[method]
	if method == "account" && row.AccessState.Lock.Account != nil {
		entry, recorded = *row.AccessState.Lock.Account, true
	}
	require.True(t, recorded, "the runtime row must hold a %s lock: %s", method, raw)
	unlockAt, err := time.Parse(time.RFC3339, entry.UnlockAt)
	require.NoError(t, err, "the %s lock must have a finite unlock time: %s", method, raw)
	failedAt, err := time.Parse(time.RFC3339, entry.LastFailedAt)
	require.NoError(t, err, "the %s lock must record the failure that formed it: %s", method, raw)
	return unlockAt, failedAt
}

// getManagement reads one management resource as the administrator, requires it to be answered and
// returns the body.
func getManagement(t *testing.T, path string) []byte {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "the read of %s failed", path)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "an administrator must be able to read %s: %s",
		path, string(body))
	return body
}

// readLastLoginAt returns the lastLoginAt a management read of a user or agent reports, empty when
// absent.
func readLastLoginAt(t *testing.T, collection, entityID string) string {
	t.Helper()

	body := getManagement(t, "/"+collection+"/"+entityID)
	var read struct {
		LastLoginAt string `json:"lastLoginAt"`
	}
	require.NoError(t, json.Unmarshal(body, &read), "unreadable %s response: %s", collection, string(body))
	return read.LastLoginAt
}

// awaitTheSecondAfter waits until the clock has passed the second a stamp names, since a stamp has
// second precision and only a later access could show a moved one, and returns that second.
func awaitTheSecondAfter(t *testing.T, stamp string) time.Time {
	t.Helper()

	at, err := time.Parse(time.RFC3339, stamp)
	require.NoError(t, err, "lastLoginAt must be an RFC 3339 instant, got %q", stamp)
	now := func() time.Time { return time.Now().UTC().Truncate(time.Second) }
	require.Eventually(t, func() bool { return now().After(at) }, 2*time.Second, 50*time.Millisecond)
	return now()
}

// statusReport is the effective access status a management read carries.
type statusReport testutils.AccessStatus

// readStatus returns the effective status a management read of the user reports.
func readStatus(t *testing.T, userID string) statusReport {
	t.Helper()
	return readEntityStatus(t, userCollection, userID)
}

// readEntityStatus is readStatus for any governed category; the status object has the same shape on
// a user, an agent and an application. Every governed read carries a status, ACTIVE included, so an
// absent one is a regression rather than an active account.
func readEntityStatus(t *testing.T, collection, entityID string) statusReport {
	t.Helper()

	status, err := testutils.GetAccessStatus(collection, entityID)
	require.NoError(t, err, "an administrator must be able to read the %s's status", collection)
	if status.Details != nil {
		for _, locked := range status.Details.LockedMethods {
			if locked.ExpiresAt != "" {
				_, err := time.Parse(time.RFC3339Nano, locked.ExpiresAt)
				require.NoError(t, err, "the %s lock reports an unreadable expiry", locked.Method)
			}
		}
	}
	return statusReport(*status)
}

// lockedMethods names every authentication method currently locked, in the order the read reported them.
func (s statusReport) lockedMethods() []string {
	if s.Details == nil {
		return nil
	}
	methods := make([]string, 0, len(s.Details.LockedMethods))
	for _, locked := range s.Details.LockedMethods {
		methods = append(methods, locked.Method)
	}
	return methods
}

// expiryOf returns the reported expiry of one locked method, and whether it was reported at all.
func (s statusReport) expiryOf(method string) (time.Time, bool) {
	if s.Details == nil {
		return time.Time{}, false
	}
	for _, locked := range s.Details.LockedMethods {
		if locked.Method != method || locked.ExpiresAt == "" {
			continue
		}
		// readEntityStatus has already required the expiry to parse.
		at, _ := time.Parse(time.RFC3339Nano, locked.ExpiresAt)
		return at, true
	}
	return time.Time{}, false
}

// awaitLapse polls the user's status until the lock that ends at expiry has lapsed, and fails the test
// if it is still held tolerance after expiry. A status read writes nothing, so polling cannot move the
// lock.
func awaitLapse(t *testing.T, userID string, expiry time.Time, tolerance time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		status, err := testutils.GetAccessStatus(userCollection, userID)
		return err == nil && status.Value == "ACTIVE"
	}, max(time.Until(expiry), 0)+tolerance, statusPollInterval, "the lock must lapse by its reported expiry")
}

// flowAnswer is what one flow execution answered: the HTTP status, the flow status if a step ran, and
// the error code from either the step or the engine.
type flowAnswer struct {
	status     int
	flowStatus string
	code       string
}

// answerOf reads a flow answer from a raw /flow/execute response.
func answerOf(status int, raw string) flowAnswer {
	var read struct {
		FlowStatus string `json:"flowStatus"`
		Code       string `json:"code"`
		Error      *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(raw), &read)
	answer := flowAnswer{status: status, flowStatus: read.FlowStatus, code: read.Code}
	if read.Error != nil {
		answer.code = read.Error.Code
	}
	return answer
}

// postFlow posts one request to /flow/execute and returns the status and the raw answer whatever they
// are. It goes through client when one is given, a browser's carrying its cookies, and sends the
// bearer token when one is given.
func postFlow(t *testing.T, client *http.Client, token string, body map[string]interface{}) (int, string) {
	t.Helper()
	status, raw, err := testutils.ExecuteFlowRaw(client, token, body)
	require.NoError(t, err, "the flow request must reach the server")
	return status, string(raw)
}

// submitSignIn starts the application's sign-in flow and submits a username and password with action,
// returning the raw answer whatever it is, since a stated refusal and a fault are the subject.
func submitSignIn(t *testing.T, appID, action, username, password string) (int, string) {
	t.Helper()

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	require.NoError(t, err, "failed to initiate the sign-in flow")
	require.Equal(t, "INCOMPLETE", step.FlowStatus)

	return postFlow(t, nil, "", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"challengeToken": step.ChallengeToken,
		"inputs":         map[string]string{"username": username, "password": password},
		"action":         action,
	})
}

// flowStepOf requires a flow answer to have been given and returns the step it carries.
func flowStepOf(t *testing.T, status int, body string) *common.FlowStep {
	t.Helper()

	require.Equal(t, http.StatusOK, status, body)
	var step common.FlowStep
	require.NoError(t, json.Unmarshal([]byte(body), &step), body)
	return &step
}

// requireRepromptedWith checks a sign-in step stayed on the credentials prompt and carries the stated
// hold.
func requireRepromptedWith(t *testing.T, step *common.FlowStep, code string) {
	t.Helper()

	require.Equal(t, "INCOMPLETE", step.FlowStatus, "a stated hold must stay on the prompt: %+v", step)
	require.Empty(t, step.Data.RedirectURL, "a stated hold must not leave the sign-in page")
	require.NotNil(t, step.Error, "the prompt must state the hold: %+v", step)
	require.Equal(t, code, step.Error.Code, "the prompt must state the hold")
	require.True(t, common.HasInput(step.Data.Inputs, "password"),
		"the prompt must ask for the credentials again: %+v", step)
}

// adminToken returns the administrator's token, for a flow request the shared clients send without one.
func adminToken(t *testing.T) string {
	t.Helper()
	token, err := testutils.GetAccessToken()
	require.NoError(t, err, "the administrator must hold a token")
	return token
}

// attemptAdministrationFlow executes one shipped governance flow against a subject as the
// administrator, and returns its answer whatever it is.
func attemptAdministrationFlow(t *testing.T, handle, subject string) flowAnswer {
	t.Helper()
	return attemptAdministrationFlowAs(t, adminToken(t), handle, map[string]string{subjectInput: subject})
}

// executeAdministrationFlowAs executes one shipped governance flow as the operator the token names and
// returns the status and the raw answer, whatever they are.
func executeAdministrationFlowAs(t *testing.T, token, handle string, inputs map[string]string) (int, string) {
	t.Helper()
	flowID, err := testutils.GetFlowIDByHandle(handle, "ADMINISTRATION")
	require.NoError(t, err, "the %s flow must ship", handle)
	status, raw, err := testutils.ExecuteFlowRaw(nil, token,
		map[string]interface{}{"flowId": flowID, "inputs": inputs})
	require.NoError(t, err, "failed to execute %s", handle)
	return status, string(raw)
}

// attemptAdministrationFlowAs executes one shipped governance flow as the operator the token names.
func attemptAdministrationFlowAs(t *testing.T, token, handle string, inputs map[string]string) flowAnswer {
	t.Helper()
	return answerOf(executeAdministrationFlowAs(t, token, handle, inputs))
}

// runAdministrationFlow executes one shipped governance flow as the administrator and requires it to
// complete. Use it where the operation is a precondition rather than the thing under test.
func runAdministrationFlow(t *testing.T, handle, subject string) {
	t.Helper()
	runAdministrationFlowAs(t, adminToken(t), handle, map[string]string{subjectInput: subject})
}

// runAdministrationFlowAs runs a flow as the operator the token names, with inputs that may carry more
// than the subject, and requires it to complete.
func runAdministrationFlowAs(t *testing.T, token, handle string, inputs map[string]string) {
	t.Helper()
	answer := attemptAdministrationFlowAs(t, token, handle, inputs)
	require.Equal(t, http.StatusOK, answer.status, "%s was refused at the transport: %+v", handle, answer)
	require.Equal(t, "COMPLETE", answer.flowStatus, "%s did not complete: %+v", handle, answer)
}

// newRootOperator creates a user in the fixture holding the system permission and signs it in to the
// Console, so a test that regresses into governing its own operator holds a throwaway account rather
// than the administrator every other suite runs as. The role is deleted when t ends.
func newRootOperator(t *testing.T, users *userFixture) (userID, username, password, accessToken string) {
	t.Helper()
	userID, username, password = users.newUser(t, "root", nil)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Governance Root " + users.suffix,
		OUID: bootstrapOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: systemResourceServerID, Permissions: []string{"system"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	require.NoError(t, err, "failed to grant the root permission")
	cleanupRole(t, roleID)
	token, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", boundaryConsoleURI, "system",
		username, password, true)
	require.NoError(t, err, "the root operator must sign in to the console")
	return userID, username, password, token.AccessToken
}

// TestMain runs the package with jwt.leeway shortened once, so no suite restarts the server for it,
// and puts the harness deployment back afterwards.
func TestMain(m *testing.M) {
	os.Exit(runWithCutoffLeeway(m))
}

// runWithCutoffLeeway runs the package's tests with the shortened leeway and returns their exit code.
func runWithCutoffLeeway(m *testing.M) int {
	// Restored even when setup fails part way, so the next package starts on the shipped deployment.
	defer func() {
		if err := testutils.UpdateDeploymentConfig(pristineDeploymentPath); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: failed to restore the deployment config: %v\n", err)
			return
		}
		if err := testutils.RestartServer(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: server did not restart after restoring the deployment: %v\n", err)
			return
		}
		if err := testutils.ObtainAdminAccessToken(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: failed to re-obtain the admin token: %v\n", err)
		}
	}()

	if err := setCutoffLeeway(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to shorten jwt.leeway: %v\n", err)
		return 1
	}
	if err := testutils.RestartServer(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to restart with the shortened leeway: %v\n", err)
		return 1
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to re-obtain the admin token: %v\n", err)
		return 1
	}
	return m.Run()
}

// applyLockoutPolicy installs this package's policy through the accountAccess section.
func applyLockoutPolicy(t *testing.T) {
	t.Helper()
	applyPolicy(t, lockoutPolicy())
}

// applyPolicy lays a policy over the section's writable layer, which applies without a restart, and
// puts the layer back as it was when t ends. The layer is merged rather than replaced, since the
// bootstrap stores the lock email templates there.
func applyPolicy(t *testing.T, policy map[string]interface{}) {
	t.Helper()

	original, err := testutils.GetWritableServerConfig(accountAccessSection)
	require.NoError(t, err, "failed to read the accountAccess section")
	t.Cleanup(func() {
		if err := testutils.PutWritableServerConfig(accountAccessSection, original); err != nil {
			t.Errorf("cleanup: failed to restore the accountAccess section: %v", err)
		}
	})
	var layer map[string]interface{}
	if len(original) > 0 {
		require.NoError(t, json.Unmarshal(original, &layer), "unreadable accountAccess layer: %s", original)
	}
	body, err := json.Marshal(mergeJSONObjects(layer, policy))
	require.NoError(t, err)
	require.NoError(t, testutils.PutWritableServerConfig(accountAccessSection, body),
		"failed to apply the lockout policy")
}

// applyDeclarativePolicy installs a declarative accountAccess document, which locks the section and
// needs a restart, and restores the harness deployment when t ends. The restore is registered before
// anything changes, so a setup that fails part way still leaves the next suite on the shipped policy.
func applyDeclarativePolicy(t *testing.T, policy interface{}) {
	t.Helper()

	t.Cleanup(func() { restoreShippedPolicy(t) })
	require.NoError(t, testutils.UpdateAccountAccessConfig(policy), "failed to apply the lockout policy")
	require.NoError(t, testutils.RestartServer(), "failed to restart with the lockout policy")
	require.NoError(t, testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")
}

// setCutoffLeeway sets jwt.leeway to suspensionCutoffLeeway. PatchDeploymentConfig replaces a top-level
// key whole, so the jwt section is read and only its leeway changed.
func setCutoffLeeway() error {
	raw, err := testutils.ReadDeploymentConfigKey("jwt")
	if err != nil {
		return err
	}
	jwtSection, _ := raw.(map[string]interface{})
	if jwtSection == nil {
		jwtSection = make(map[string]interface{})
	}
	jwtSection["leeway"] = int64(suspensionCutoffLeeway / time.Second)
	return testutils.PatchDeploymentConfig(map[string]interface{}{"jwt": jwtSection})
}

// deploymentLockoutPatch is lockoutPolicy in the deployment file's snake_case form.
func deploymentLockoutPatch() map[string]interface{} {
	return map[string]interface{}{
		"account_access": map[string]interface{}{
			"user": map[string]interface{}{
				"lock_granularity": "authentication_method",
				"default": map[string]interface{}{
					"enabled":                true,
					"threshold":              lockThreshold,
					"failure_window_seconds": failureWindowSecs,
					"lock_durations_seconds": []int{firstRungSeconds, secondRungSeconds},
					"lock_decay_seconds":     lockDecaySeconds,
				},
			},
		},
	}
}

// applyDeploymentLockoutPolicy installs this package's policy through deployment.yaml, which is the
// base the section overlays and leaves it writable. The leeway is kept.
func applyDeploymentLockoutPolicy(t *testing.T) {
	t.Helper()

	t.Cleanup(func() { restoreShippedPolicy(t) })
	require.NoError(t, testutils.PatchDeploymentConfig(deploymentLockoutPatch()),
		"failed to apply the lockout policy")
	require.NoError(t, testutils.RestartServer(), "failed to restart with the lockout policy")
	require.NoError(t, testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")
}

// restoreShippedPolicy removes the declarative accountAccess document and restores the harness
// deployment with the package's leeway. Each step runs even if an earlier one failed, and a failure
// fails the test: a policy left behind would show up as unrelated failures in the next package.
func restoreShippedPolicy(t *testing.T) {
	t.Helper()

	if err := testutils.RestoreAccountAccessConfig(); err != nil {
		t.Errorf("cleanup: failed to restore accountAccess config: %v", err)
	}
	if err := testutils.UpdateDeploymentConfig(pristineDeploymentPath); err != nil {
		t.Errorf("cleanup: failed to restore the deployment config: %v", err)
	}
	if err := setCutoffLeeway(); err != nil {
		t.Errorf("cleanup: failed to restore jwt.leeway: %v", err)
	}
	if err := testutils.RestartServer(); err != nil {
		t.Errorf("cleanup: server did not restart cleanly after restoring the policy: %v", err)
		return
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		t.Errorf("cleanup: failed to re-obtain the admin token after restoring the policy: %v", err)
	}
}

// mergeJSONObjects lays override over base, recursing into nested objects; any other value in override
// replaces the base's. Neither argument is modified.
func mergeJSONObjects(base, override map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(base))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		baseChild, baseIsObject := merged[key].(map[string]interface{})
		overrideChild, overrideIsObject := value.(map[string]interface{})
		if baseIsObject && overrideIsObject {
			merged[key] = mergeJSONObjects(baseChild, overrideChild)
			continue
		}
		merged[key] = value
	}
	return merged
}

// uniqueSuffix keeps every account in this package distinct, so no test can lock out another's user.
func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// userFixture is an organization unit and user types of its own, and the accounts created in them. It
// deletes them when the test that built it ends, after the suite's own teardown.
type userFixture struct {
	name     string
	suffix   string
	ouID     string
	userType string
	typeIDs  []string
	userIDs  []string
}

// newUserFixture creates the organization unit governance_<name>_ou and, in it, the user type
// governance_<name>_user as userType describes it.
func newUserFixture(t *testing.T, name string, userType testutils.UserType) *userFixture {
	t.Helper()

	f := &userFixture{name: name, suffix: uniqueSuffix(), userType: "governance_" + name + "_user"}
	t.Cleanup(func() { f.tearDown(t) })
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "governance_" + name + "_ou",
		Name:        "Governance " + name + " OU",
		Description: "Organization unit for the " + name + " governance tests",
	})
	require.NoError(t, err, "failed to create the test organization unit")
	f.ouID = ouID

	userType.Handle = f.userType
	f.addUserType(t, userType)
	return f
}

// addUserType creates a further user type in the fixture's organization unit.
func (f *userFixture) addUserType(t *testing.T, userType testutils.UserType) {
	t.Helper()

	userType.DisplayName = userType.Handle
	userType.OUID = f.ouID
	typeID, err := testutils.CreateUserType(userType)
	require.NoError(t, err, "failed to create user type %s", userType.Handle)
	f.typeIDs = append(f.typeIDs, typeID)
}

// newUser creates an account of the fixture's type that no other test touches, named after label,
// with a password and any extra attributes, and returns its id, username and password.
func (f *userFixture) newUser(t *testing.T, label string, extra map[string]interface{}) (
	userID, username, password string) {
	t.Helper()
	return f.newUserOfType(t, f.userType, label, extra)
}

// newUserOfType is newUser for another of the fixture's user types.
func (f *userFixture) newUserOfType(t *testing.T, userType, label string, extra map[string]interface{}) (
	userID, username, password string) {
	t.Helper()

	username = f.usernameFor(label)
	password = "Correct-Horse-" + label + "1!"
	attributes := map[string]interface{}{"username": username, "password": password}
	for key, value := range extra {
		attributes[key] = value
	}
	raw, err := json.Marshal(attributes)
	require.NoError(t, err)

	userID, err = testutils.CreateUser(testutils.User{Type: userType, OUID: f.ouID, Attributes: raw})
	require.NoError(t, err, "failed to create %s", username)
	f.userIDs = append(f.userIDs, userID)
	return userID, username, password
}

// usernameFor is the username newUser gives the account it creates under label.
func (f *userFixture) usernameFor(label string) string {
	return fmt.Sprintf("%s_%s_%s", f.name, label, f.suffix)
}

// tearDown deletes what the fixture created, logging rather than failing.
func (f *userFixture) tearDown(t *testing.T) {
	t.Helper()
	for _, userID := range f.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			t.Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	for _, typeID := range f.typeIDs {
		if err := testutils.DeleteUserType(typeID); err != nil {
			t.Logf("teardown: failed to delete user type %s: %v", typeID, err)
		}
	}
	if f.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(f.ouID); err != nil {
			t.Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// credentialsSchema is a user type schema with a username and a password, plus any extra attributes.
func credentialsSchema(extra map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	}
	for key, value := range extra {
		schema[key] = value
	}
	return schema
}

// browser signs in as a browser does, through one OAuth client: a cookie jar carries the SSO cookie
// between calls, and redirects are not followed so the gate's Location header can be read.
type browser struct {
	t            *testing.T
	client       *http.Client
	clientID     string
	clientSecret string
	redirectURI  string
}

// newBrowser returns a browser for one OAuth client with an empty cookie jar.
func newBrowser(t *testing.T, clientID, clientSecret, redirectURI string) *browser {
	t.Helper()

	client, err := testutils.NewSessionClient()
	require.NoError(t, err, "failed to create a cookie jar")
	return &browser{
		t:            t,
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
	}
}

// authorize starts an authorization code flow and returns the authId and executionId the gate
// redirect carries.
func (b *browser) authorize(state string) (authID, executionID string) {
	b.t.Helper()

	params := url.Values{}
	params.Set("client_id", b.clientID)
	params.Set("redirect_uri", b.redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", state)

	authID, executionID, err := testutils.StartAuthorizationWithSession(b.client, params)
	require.NoError(b.t, err, "authorize must redirect to the gate")
	return authID, executionID
}

// step posts one flow step through the browser, so the SSO cookie is both carried and updated, and
// requires a flow answer.
func (b *browser) step(body map[string]interface{}) *testutils.FlowStep {
	b.t.Helper()

	status, raw := postFlow(b.t, b.client, "", body)
	require.Equal(b.t, http.StatusOK, status, "flow execute failed: %s", raw)
	var step testutils.FlowStep
	require.NoError(b.t, json.Unmarshal([]byte(raw), &step), "unreadable flow step: %s", raw)
	return &step
}

// signIn authorizes and takes the first step. With credentials, that step must prompt and is
// answered with them; without, the first step is all a live session needs. It returns the authId and
// the last step.
func (b *browser) signIn(state string, credentials map[string]string) (string, *testutils.FlowStep) {
	b.t.Helper()

	authID, executionID := b.authorize(state)
	step := b.step(map[string]interface{}{"executionId": executionID})
	if credentials == nil {
		return authID, step
	}
	require.NotEqual(b.t, "COMPLETE", step.FlowStatus, "a first sign-in must prompt for credentials")
	return authID, b.step(map[string]interface{}{
		"executionId":    executionID,
		"inputs":         credentials,
		"action":         signInAction,
		"challengeToken": step.ChallengeToken,
	})
}

// signInForCode drives a first-time sign-in up to the authorization code, without redeeming it.
func (b *browser) signInForCode(username, password, state string) string {
	b.t.Helper()

	authID, step := b.signIn(state, map[string]string{"username": username, "password": password})
	require.Equal(b.t, "COMPLETE", step.FlowStatus, "the credential sign-in must complete the flow")
	require.NotEmpty(b.t, step.Assertion, "the sign-in must yield an assertion")

	authz, err := testutils.CompleteAuthorizationWithClient(b.client, authID, step.Assertion)
	require.NoError(b.t, err, "the auth callback failed")
	code, err := testutils.ExtractAuthorizationCode(authz.RedirectURI)
	require.NoError(b.t, err, "failed to extract the authorization code")
	return code
}

// redeem swaps an authorization code for tokens and returns the status and body as they came.
func (b *browser) redeem(code string) (int, string) {
	b.t.Helper()

	result, err := testutils.RequestToken(b.clientID, b.clientSecret, code, b.redirectURI, "authorization_code")
	require.NoError(b.t, err, "the token request failed")
	return result.StatusCode, string(result.Body)
}

// login drives a first-time sign-in to completion and returns its tokens. The first step must
// prompt, proving the session was established by this login.
func (b *browser) login(username, password, state string) *testutils.TokenResponse {
	b.t.Helper()

	status, body := b.redeem(b.signInForCode(username, password, state))
	require.Equal(b.t, http.StatusOK, status, "the token request failed: %s", body)
	var token testutils.TokenResponse
	require.NoError(b.t, json.Unmarshal([]byte(body), &token), "unreadable token response")
	return &token
}

// sessionSatisfiesAuthorize reports whether the browser's SSO session is still live: a new authorize
// that completes on its first step was satisfied by it; a prompt means it has ended.
func (b *browser) sessionSatisfiesAuthorize(state string) bool {
	b.t.Helper()
	_, step := b.signIn(state, nil)
	return step.FlowStatus == "COMPLETE" && step.Assertion != ""
}

// ssoCookieNames reports the per-flow SSO cookies the browser currently holds.
func (b *browser) ssoCookieNames() []string {
	names, err := testutils.SSOCookieNames(b.client)
	require.NoError(b.t, err)
	return names
}

// clientCredentials requests the client's own token with the client_credentials grant, for the
// resource when one is given, and returns the answer whatever it is.
func clientCredentials(t *testing.T, clientID, clientSecret, resource string) *testutils.TokenHTTPResult {
	t.Helper()

	result, err := testutils.RequestTokenWithResource(clientID, clientSecret, "", "", "client_credentials",
		resource)
	require.NoError(t, err, "the token request itself must reach the server")
	return result
}

// requireOAuthError requires an OAuth endpoint's answer to be the given refusal: its status, its error
// code and its description.
func requireOAuthError(t *testing.T, status int, body string, wantStatus int, wantError, wantDescription,
	msg string) {
	t.Helper()
	var refusal struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	require.Equal(t, wantStatus, status, "%s: %s", msg, body)
	require.NoError(t, json.Unmarshal([]byte(body), &refusal), "%s: unreadable refusal: %s", msg, body)
	require.Equal(t, wantError, refusal.Error, "%s: %s", msg, body)
	require.Equal(t, wantDescription, refusal.ErrorDescription, "%s: %s", msg, body)
}

// putSection writes a server-config section's writable layer and returns the status and body, for a
// write whose refusal is the subject rather than an error.
func putSection(t *testing.T, section, body string) (int, string) {
	t.Helper()
	status, response, err := testutils.PutWritableServerConfigWithStatus(section, []byte(body))
	require.NoError(t, err, "the %s write must be answered", section)
	return status, string(response)
}

// getWith sends a GET through client and returns the status and the body.
func getWith(t *testing.T, client *http.Client, path string) (int, string) {
	t.Helper()
	resp, err := client.Get(testServerURL + path)
	require.NoError(t, err, "the read of %s must reach the server", path)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// tokenActive reports whether introspection, as the client, finds the token active.
func tokenActive(t *testing.T, token, clientID, clientSecret string) bool {
	t.Helper()
	active, err := testutils.IntrospectToken(token, clientID, clientSecret)
	require.NoError(t, err, "introspection itself must answer")
	return active
}

// copyShippedFlowWithSender creates a copy of a shipped flow under its own handle, with senderID named
// on one of its email nodes as an operator must name one, since a flow email node has no default
// sender. calls maps a CALL node's ID to the flow the copy calls there instead, so the copy can serve
// an application whose flows must reference one another. The shipped flow is left as it is, and the
// copy is deleted when t ends. It returns the copy's ID and handle.
func copyShippedFlowWithSender(t *testing.T, handle, flowType, nodeID, senderID string,
	calls map[string]string) (flowID, copyHandle string) {
	t.Helper()

	shippedID, err := testutils.GetFlowIDByHandle(handle, flowType)
	require.NoError(t, err, "the %s %s flow must ship", handle, flowType)
	var flow testutils.Flow
	require.NoError(t, json.Unmarshal(getManagement(t, "/flows/"+shippedID), &flow))
	require.NoError(t, testutils.SetFlowNodeProperty(&flow, nodeID, "senderId", senderID),
		"flow %s must carry a %s node", handle, nodeID)
	for callNodeID, calledFlowID := range calls {
		require.NoError(t, testutils.ModifyFlowNode(&flow, callNodeID, func(node map[string]interface{}) {
			node["flow"] = map[string]interface{}{"ref": calledFlowID}
		}), "flow %s must carry a %s node", handle, callNodeID)
	}

	flow.Name += " with a sender"
	flow.Handle = handle + "-with-sender"
	flowID, err = testutils.CreateFlow(flow)
	require.NoError(t, err, "failed to copy the %s flow", handle)
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("teardown: failed to delete the copy of %s: %v", handle, err)
		}
	})
	return flowID, flow.Handle
}

// putManagement writes one management resource as the administrator and returns the status and the
// body, whatever they are.
func putManagement(t *testing.T, path string, body interface{}) (int, string) {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPut, testServerURL+path, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "the write of %s failed", path)
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(response)
}

// newShippedFlowApp creates an application in the fixture's organization unit that signs in, registers
// and recovers through the shipped default flows, which call one another, so it names all three. It
// is deleted when t ends, before the fixture.
func newShippedFlowApp(t *testing.T, users *userFixture, name string) string {
	t.Helper()

	shipped := make(map[string]string, 3)
	for _, flowType := range []string{"AUTHENTICATION", "REGISTRATION", "RECOVERY"} {
		flowID, err := testutils.GetFlowIDByHandle(defaultFlowHandle, flowType)
		require.NoError(t, err, "the default %s flow must ship", flowType)
		shipped[flowType] = flowID
	}

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      users.ouID,
		Name:                      "Governance " + name + " Shipped Flows App",
		Description:               "Application that runs the shipped default flows",
		IsRegistrationFlowEnabled: true,
		IsRecoveryFlowEnabled:     true,
		ClientID:                  "governance_" + name + "_shipped_client",
		ClientSecret:              "governance_" + name + "_shipped_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{users.userType},
		AuthFlowID:                shipped["AUTHENTICATION"],
		RegistrationFlowID:        shipped["REGISTRATION"],
		RecoveryFlowID:            shipped["RECOVERY"],
	})
	require.NoError(t, err, "failed to create the %s application", name)
	t.Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			t.Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	})
	return appID
}

// publicClientRedirectURI is where a public client created by newPublicClientApp is registered.
const publicClientRedirectURI = "https://localhost:3000/public-callback"

// newPublicClientApp creates a public application in the fixture's organization unit: it presents no
// secret at the token endpoint and proves its codes with PKCE, so no secret check can refuse it. It is
// deleted when t ends, and its entity and client IDs are returned.
func newPublicClientApp(t *testing.T, users *userFixture, name string) (appID, clientID string) {
	t.Helper()

	clientID = fmt.Sprintf("governance_public_%s_%s", name, users.suffix)
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             users.ouID,
		Name:             fmt.Sprintf("Governance Public %s %s", name, users.suffix),
		Description:      "Public client that authenticates without a secret",
		AllowedUserTypes: []string{users.userType},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"redirectUris":            []string{publicClientRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
				"pkceRequired":            true,
			},
		}},
	})
	require.NoError(t, err, "failed to create the %s public application", name)
	cleanupApplication(t, appID)
	return appID, clientID
}

// redeemUnknownCode redeems a code the server never issued as a public client, with a code verifier
// and no secret, and returns the answer whatever it is. A client that authenticates answers
// invalid_grant; one refused at client authentication answers invalid_client.
func redeemUnknownCode(t *testing.T, clientID string) *testutils.TokenHTTPResult {
	t.Helper()

	verifier, err := testutils.GenerateCodeVerifier()
	require.NoError(t, err)
	result, err := testutils.RequestTokenWithPKCE(clientID, "", "governance-never-issued-code",
		publicClientRedirectURI, "authorization_code", verifier)
	require.NoError(t, err, "the token request itself must reach the server")
	return result
}

// cleanupRole deletes the role when t ends.
func cleanupRole(t *testing.T, roleID string) {
	t.Helper()
	t.Cleanup(func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			t.Logf("teardown: failed to delete role %s: %v", roleID, err)
		}
	})
}

// cleanupApplication deletes the application when t ends.
func cleanupApplication(t *testing.T, appID string) {
	t.Helper()
	t.Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			t.Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	})
}

// cleanupFlow deletes the flow when t ends.
func cleanupFlow(t *testing.T, flowID string) {
	t.Helper()
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	})
}

// awaitDelivered polls take until it hands back a message, and fails the test if none arrives within
// deliveryTimeout.
func awaitDelivered[M any](t *testing.T, take func() *M, msg string) *M {
	t.Helper()

	var got *M
	require.Eventually(t, func() bool {
		got = take()
		return got != nil
	}, deliveryTimeout, 100*time.Millisecond, msg)
	return got
}

// attemptRecovery drives an application's username-and-new-password recovery flow and reports whether
// it completed. extra is sent with every step, for a request naming inputs the flow did not ask for. A
// refusal is still a flow answer, so a failed request fails the test.
func attemptRecovery(t *testing.T, appID, username, newPassword string, extra map[string]string) bool {
	t.Helper()

	with := func(inputs map[string]string) map[string]string {
		for key, value := range extra {
			inputs[key] = value
		}
		return inputs
	}

	step, err := common.InitiateRecoveryFlow(appID, false, nil, "")
	require.NoError(t, err, "failed to initiate the recovery flow")
	require.Equal(t, "INCOMPLETE", step.FlowStatus)

	step, err = common.CompleteFlow(step.ExecutionID,
		with(map[string]string{"username": username}), submitUsernameAction, step.ChallengeToken)
	require.NoError(t, err, "failed to submit the username")
	if step.FlowStatus != "INCOMPLETE" || !common.HasInput(step.Data.Inputs, "password") {
		return false
	}

	step, err = common.CompleteFlow(step.ExecutionID,
		with(map[string]string{"password": newPassword}), submitPasswordAction, step.ChallengeToken)
	require.NoError(t, err, "failed to submit the new password")
	return step.FlowStatus == "COMPLETE"
}

// recoverThrough drives an application's recovery flow to completion for one account, setting
// newPassword.
func recoverThrough(t *testing.T, appID, username, newPassword string) {
	t.Helper()
	require.True(t, attemptRecovery(t, appID, username, newPassword, nil), "the recovery flow must run to completion")
}
