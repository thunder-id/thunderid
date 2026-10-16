// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package governance checks the account lock and suspension behaviour against a running server.
// Tests check stated behaviour, not the implementation that satisfies it.
package governance

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL           = testutils.TestServerURL
	credentialsAuthEndpoint = "/auth/credentials/authenticate"
	administrationFlowType  = "ADMINISTRATION"

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

// suspensionCutoffLeeway is jwt.leeway as every suite in this package runs it, shorter than the shipped
// 30 seconds so the waits past a suspension's cutoff stay short, yet long enough for a release and a
// sign-in to fit inside it on a slow runner. A suspension's revocation cutoff is its pre-suspend time
// plus this allowance.
const suspensionCutoffLeeway = 15 * time.Second

// The management collections a governed identity is read back through. The effective status is
// public API on all three.
const (
	userCollection        = "users"
	agentCollection       = "agents"
	applicationCollection = "applications"
)

// The policy this package runs under, applied to the declarative section for the suite's duration.
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

// accountAccessPatch is the declarative policy this package installs. Every sign-in method counts
// under its own key with the category default.
func accountAccessPatch() map[string]interface{} {
	return map[string]interface{}{
		"accountAccess": map[string]interface{}{
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
		},
	}
}

// Stated holds, as the direct authentication endpoints answer them.
const (
	statedLockCode       = "AUTHN-MGR-1005"
	statedSuspensionCode = "AUTHN-MGR-1006"
)

// authResult is everything an authentication attempt returns that a criterion can be written about.
type authResult struct {
	status int
	body   string
}

func (r authResult) succeeded() bool { return r.status == http.StatusOK }

// authenticate presents one credential to the atomic credentials endpoint.
func authenticate(t *testing.T, identifierKey, identifierValue, credential string) authResult {
	t.Helper()

	payload, err := json.Marshal(map[string]interface{}{
		"identifiers": map[string]interface{}{identifierKey: identifierValue},
		"credentials": map[string]interface{}{"password": credential},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+credentialsAuthEndpoint,
		bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "authentication request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return authResult{status: resp.StatusCode, body: string(body)}
}

// requireStatedHold checks a held account's refusal keeps a wrong secret's status and states the hold
// by code, never the sign-in method or when the lock ends.
func requireStatedHold(t *testing.T, wrong, held authResult, code string) {
	t.Helper()
	require.NotEqual(t, http.StatusOK, wrong.status, "the comparison is only meaningful against a refusal")
	require.False(t, held.succeeded(), "a held account must be refused")
	require.Equal(t, wrong.status, held.status, "a stated hold keeps the status a refused sign-in has")
	require.Contains(t, held.body, code, "the refusal must state the hold: %s", held.body)
	require.NotContains(t, held.body, "unlock", "the refusal must not say when the lock ends")
}

// failAuthentication presents a wrong secret count times and requires every attempt to be refused.
func failAuthentication(t *testing.T, identifierKey, identifierValue string, count int) {
	t.Helper()
	for i := range count {
		res := authenticate(t, identifierKey, identifierValue, "DefinitelyNotTheSecret!"+fmt.Sprint(i))
		require.False(t, res.succeeded(), "attempt %d was expected to be refused", i+1)
	}
}

// readLastLoginAt returns the lastLoginAt a management read of a user or agent reports, empty when
// absent.
func readLastLoginAt(t *testing.T, collection, entityID string) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/"+collection+"/"+entityID, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s read failed: %s", collection, string(body))

	var read struct {
		LastLoginAt string `json:"lastLoginAt"`
	}
	require.NoError(t, json.Unmarshal(body, &read), "unreadable %s response: %s", collection, string(body))
	return read.LastLoginAt
}

// statusReport is the effective access status a management read carries.
type statusReport struct {
	Value   string `json:"value"`
	Details *struct {
		Reason       string `json:"reason"`
		Since        string `json:"since"`
		OperatorNote string `json:"operatorNote"`
		LockedScopes []struct {
			Scope     string `json:"scope"`
			ExpiresAt string `json:"expiresAt"`
		} `json:"lockedScopes"`
	} `json:"details"`
}

// readStatus returns the effective status a management read of the user reports.
func readStatus(t *testing.T, userID string) statusReport {
	t.Helper()
	return readEntityStatus(t, userCollection, userID)
}

// readEntityStatus is readStatus for any governed category; the status object has the same shape on
// a user, an agent and an application.
func readEntityStatus(t *testing.T, collection, entityID string) statusReport {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/"+collection+"/"+entityID, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "%s read failed", collection)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"an administrator must keep read access to the record: %s", string(body))

	var read struct {
		Status *statusReport `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &read), "unreadable %s response: %s", collection, string(body))
	// Every governed read carries a status, ACTIVE included, so an absent one is a regression rather
	// than an active account.
	require.NotNil(t, read.Status, "a %s read must report its status: %s", collection, string(body))
	if read.Status.Details != nil {
		for _, locked := range read.Status.Details.LockedScopes {
			if locked.ExpiresAt != "" {
				_, err := time.Parse(time.RFC3339Nano, locked.ExpiresAt)
				require.NoError(t, err, "the %s lock reports an unreadable expiry", locked.Scope)
			}
		}
	}
	return *read.Status
}

// lockedScopes names every authentication method currently locked, in the order the read reported them.
func (s statusReport) lockedScopes() []string {
	if s.Details == nil {
		return nil
	}
	scopes := make([]string, 0, len(s.Details.LockedScopes))
	for _, locked := range s.Details.LockedScopes {
		scopes = append(scopes, locked.Scope)
	}
	return scopes
}

// expiryOf returns the reported expiry of one locked scope, and whether it was reported at all.
func (s statusReport) expiryOf(scope string) (time.Time, bool) {
	if s.Details == nil {
		return time.Time{}, false
	}
	for _, locked := range s.Details.LockedScopes {
		if locked.Scope != scope || locked.ExpiresAt == "" {
			continue
		}
		// readEntityStatus has already required the expiry to parse.
		at, _ := time.Parse(time.RFC3339Nano, locked.ExpiresAt)
		return at, true
	}
	return time.Time{}, false
}

// attemptAdministrationFlow executes one shipped governance flow against a subject as an
// administrator, and returns the HTTP status and the flow status.
func attemptAdministrationFlow(t *testing.T, handle, subject string) (int, string) {
	t.Helper()
	return attemptAdministrationFlowWithInputs(t, handle, map[string]string{subjectInput: subject})
}

// attemptAdministrationFlowWithInputs is the same, for the flows that take more than a subject.
func attemptAdministrationFlowWithInputs(t *testing.T, handle string,
	inputs map[string]string) (int, string) {
	t.Helper()

	status, flowStatus, err := testutils.RunAdministrationFlow(handle, inputs)
	require.NoError(t, err, "failed to execute %s", handle)
	return status, flowStatus
}

// runAdministrationFlow executes one shipped governance flow and requires it to complete. Use it
// where the operation is a precondition rather than the thing under test.
func runAdministrationFlow(t *testing.T, handle, subject string) {
	t.Helper()

	status, flowStatus := attemptAdministrationFlow(t, handle, subject)
	require.Equal(t, http.StatusOK, status, "%s was refused at the transport", handle)
	require.Equal(t, "COMPLETE", flowStatus, "%s did not complete", handle)
}

// runAdministrationFlowWithInputs runs a flow that carries more than the subject, and requires it to
// complete.
func runAdministrationFlowWithInputs(t *testing.T, handle string, inputs map[string]string) {
	t.Helper()

	status, flowStatus := attemptAdministrationFlowWithInputs(t, handle, inputs)
	require.Equal(t, http.StatusOK, status, "%s was refused at the transport", handle)
	require.Equal(t, "COMPLETE", flowStatus, "%s did not complete", handle)
}

// TestMain runs the package with jwt.leeway shortened once, so no suite restarts the server for it,
// and puts the harness deployment back afterwards.
func TestMain(m *testing.M) {
	os.Exit(runWithCutoffLeeway(m))
}

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
	applyPolicy(t, accountAccessPatch()["accountAccess"])
}

// applyPolicy writes a policy into the section's writable layer, which applies without a restart, and
// puts the layer back as it was when t ends.
func applyPolicy(t *testing.T, policy interface{}) {
	t.Helper()

	original, err := testutils.GetWritableServerConfig(accountAccessSection)
	require.NoError(t, err, "failed to read the accountAccess section")
	t.Cleanup(func() {
		if err := testutils.PutWritableServerConfig(accountAccessSection, original); err != nil {
			t.Errorf("cleanup: failed to restore the accountAccess section: %v", err)
		}
	})
	body, err := json.Marshal(policy)
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

// deploymentLockoutPatch is accountAccessPatch's policy in the deployment file's snake_case form.
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
// deployment with the package's leeway. Each step runs even if an earlier one failed, and a failure fails the test: a policy left
// behind would show up as unrelated failures in the next package.
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
	return strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
}

// newSessionClient returns a browser-shaped client: a cookie jar carries the SSO cookie between
// calls, and redirects are not followed so the gate's Location header can be read.
func newSessionClient(t *testing.T) *http.Client {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err, "failed to create a cookie jar")
	return &http.Client{
		Jar:       jar,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// flowExecute posts one step through the session client, so the SSO cookie is both carried and
// updated the way a browser would.
func flowExecute(t *testing.T, client *http.Client, body map[string]interface{}) *testutils.FlowStep {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err, "the flow execute request failed")
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "flow execute failed: %s", string(respBody))

	var step testutils.FlowStep
	require.NoError(t, json.Unmarshal(respBody, &step), "unreadable flow step: %s", string(respBody))
	return &step
}

// clientCredentials requests the client's own token with the client_credentials grant.
func clientCredentials(t *testing.T, clientID, clientSecret string) *testutils.TokenHTTPResult {
	t.Helper()

	result, err := testutils.RequestTokenWithResource(clientID, clientSecret, "", "", "client_credentials", "")
	require.NoError(t, err, "the token request itself must reach the server")
	return result
}

// nameFlowSender names a sender on one email node of a shipped flow, as an operator must, since a flow
// email node has no default sender. It returns a function that restores the flow.
func nameFlowSender(t *testing.T, handle, flowType, nodeID, senderID string) func() {
	t.Helper()

	flowID, err := testutils.GetFlowIDByHandle(handle, flowType)
	require.NoError(t, err)

	original := readFlow(t, flowID)
	updated := readFlow(t, flowID)
	require.NoError(t, testutils.SetFlowNodeProperty(&updated, nodeID, "senderId", senderID),
		"flow %s must carry a %s node", handle, nodeID)
	putFlow(t, flowID, updated)
	return func() { putFlow(t, flowID, original) }
}

// readFlow reads a flow as the definition an update takes.
func readFlow(t *testing.T, flowID string) testutils.Flow {
	t.Helper()

	resp, err := testutils.GetHTTPClient().Get(fmt.Sprintf("%s/flows/%s", testServerURL, flowID))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var flow testutils.Flow
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&flow))
	return flow
}

// putFlow writes a flow definition.
func putFlow(t *testing.T, flowID string, flow testutils.Flow) {
	t.Helper()

	body, err := json.Marshal(flow)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/flows/%s", testServerURL, flowID),
		bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "flow update: %s", respBody)
}

// awaitEmail polls the mock SMTP server for a captured email, and returns nil if none arrives in time.
func awaitEmail(smtp *testutils.MockSMTPServer, timeout time.Duration) *testutils.EmailMessage {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if email := smtp.GetLastEmail(); email != nil {
			return email
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
