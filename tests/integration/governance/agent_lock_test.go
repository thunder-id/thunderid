// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AgentLockTestSuite checks that an agent signing in with a password locks under the agent policy,
// states the lock, keeps it under a suspension, and that the shipped agent unlock flow releases it. The
// shared `default` agent type gains a username and a password for the suite's duration, in the OU it
// already points at.
type AgentLockTestSuite struct {
	suite.Suite
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	suffix            string
}

func TestAgentLockTestSuite(t *testing.T) {
	suite.Run(t, new(AgentLockTestSuite))
}

func (ts *AgentLockTestSuite) SetupSuite() {
	t := ts.T()
	applyPolicy(t, agentLockPolicy())

	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot
	t.Cleanup(func() {
		if err := testutils.RestoreAgentType(snapshot); err != nil {
			t.Errorf("teardown: failed to restore the default agent type: %v", err)
		}
	})

	schema := maps.Clone(snapshot.Schema)
	if schema == nil {
		schema = map[string]interface{}{}
	}
	schema["agent_username"] = map[string]interface{}{"type": "string", "unique": true}
	schema["password"] = map[string]interface{}{"type": "string", "credential": true}
	// The update replaces the whole type, so every field but the schema is carried over.
	widened := testutils.UserType{
		DisplayName:           snapshot.DisplayName,
		OUID:                  snapshot.OUID,
		AllowSelfRegistration: snapshot.AllowSelfRegistration,
		Schema:                schema,
	}
	if display, ok := snapshot.SystemAttributes["display"].(string); ok {
		widened.SystemAttributes = &testutils.UserTypeSystemAttributes{Display: display}
	}
	_, err = testutils.CreateAgentType(widened)
	ts.Require().NoError(err, "failed to give the default agent type a password")

	ts.suffix = uniqueSuffix()
}

func (ts *AgentLockTestSuite) TestTheAgentUnlockFlowReleasesTheAgent() {
	agentID, username, password := ts.newAgent("unlock")
	ts.lockPassword(agentID, username)
	requireRefusedAsHeld(ts.T(), authenticate(ts.T(), "agent_username", username, password), statedLockCode,
		"the lock must refuse the agent's real secret")

	runAdministrationFlow(ts.T(), agentUnlockFlowHandle, agentID)

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), agentCollection, agentID).Value,
		"the agent unlock flow must release the lock")
	ts.True(authenticate(ts.T(), "agent_username", username, password).succeeded(),
		"an unlocked agent must authenticate with its real secret")
}

// The correct secret gets the stated lock too, and a wrong one during the lock is not counted.
func (ts *AgentLockTestSuite) TestAWrongSecretOnALockedAgentStatesTheLock() {
	agentID, username, password := ts.newAgent("stated")
	_, liveName, _ := ts.newAgent("stated_live")
	ts.lockPassword(agentID, username)
	revision := runtimeRevision(ts.T(), agentID)

	wrong := authenticate(ts.T(), "agent_username", liveName, "Not-The-Agent-Secret-1!")
	for _, secret := range []string{"Not-The-Agent-Secret-1!", password} {
		held := authenticate(ts.T(), "agent_username", username, secret)
		requireStatedHold(ts.T(), wrong, held, statedLockCode, "credential")
	}
	ts.Equal(revision, runtimeRevision(ts.T(), agentID), "a wrong secret during the lock must not be counted")
}

// A suspension placed over an agent's lock is reported in its place, and the agent's unsuspend, which
// adds no hold of its own, leaves the lock where it was.
func (ts *AgentLockTestSuite) TestAnAgentLockOutlivesASuspension() {
	agentID, username, password := ts.newAgent("suspended")
	_, liveName, _ := ts.newAgent("suspended_live")
	ts.lockPassword(agentID, username)
	expiry, reported := readEntityStatus(ts.T(), agentCollection, agentID).expiryOf("credential")
	ts.Require().True(reported, "the lock must report its expiry")

	runAdministrationFlow(ts.T(), agentSuspendFlowHandle, agentID)
	ts.Equal("SUSPENDED", readEntityStatus(ts.T(), agentCollection, agentID).Value,
		"the suspension must be reported over the lock")
	wrong := authenticate(ts.T(), "agent_username", liveName, "Not-The-Agent-Secret-2!")
	requireStatedHold(ts.T(), wrong, authenticate(ts.T(), "agent_username", username, password),
		statedSuspensionCode, "credential")

	runAdministrationFlow(ts.T(), agentUnsuspendFlowHandle, agentID)
	status := readEntityStatus(ts.T(), agentCollection, agentID)
	ts.Equal("LOCKED", status.Value, "the unsuspended agent must still be locked")
	ts.Equal([]string{"credential"}, status.lockedMethods(), "the lock must be the password's, as formed")
	after, reported := status.expiryOf("credential")
	ts.Require().True(reported, "the lock must still report its expiry")
	ts.Equal(expiry, after, "the suspension must leave the lock as it was")
	requireStatedHold(ts.T(), wrong, authenticate(ts.T(), "agent_username", username, password),
		statedLockCode, "credential")
}

func (ts *AgentLockTestSuite) TestAnUpdateReportsTheAgentLock() {
	agentID, username, _ := ts.newAgent("update")
	ts.lockPassword(agentID, username)
	read := readEntityStatus(ts.T(), agentCollection, agentID)

	status, body := putManagement(ts.T(), "/"+agentCollection+"/"+agentID, map[string]interface{}{
		"ouId": ts.agentTypeSnapshot.OUID,
		"type": "default",
		"attributes": map[string]interface{}{
			"name":           fmt.Sprintf("governance-lock-update-%s", ts.suffix),
			"agent_username": username,
		},
	})
	ts.Require().Equal(http.StatusOK, status, "a locked agent must stay writable: %s", body)

	var updated struct {
		Status statusReport `json:"status"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &updated), body)
	ts.Equal(read, updated.Status, "the update must report the lock as a read does")
	ts.Equal([]string{"credential"}, updated.Status.lockedMethods(), "the update must report what is locked")
}

// agentLockPolicy locks an agent's password at the package threshold, on a rung no test outlasts.
func agentLockPolicy() map[string]interface{} {
	return map[string]interface{}{
		"agent": map[string]interface{}{
			"lockGranularity": "authentication_method",
			"default": map[string]interface{}{
				"enabled":              true,
				"threshold":            lockThreshold,
				"failureWindowSeconds": failureWindowSecs,
				"lockDurationsSeconds": []int{longFirstRungSeconds},
				"lockDecaySeconds":     lockDecaySeconds,
			},
		},
	}
}

// lockPassword fails the agent's password to the threshold and requires the lock it forms.
func (ts *AgentLockTestSuite) lockPassword(agentID, username string) {
	ts.T().Helper()

	failAuthentication(ts.T(), "agent_username", username, lockThreshold)
	status := readEntityStatus(ts.T(), agentCollection, agentID)
	ts.Require().Equal("LOCKED", status.Value, "failing the agent's password to the threshold must lock it")
	ts.Require().Equal([]string{"credential"}, status.lockedMethods(), "the lock must be on the password")
}

// newAgent creates one agent with a username and password, in the OU its type belongs to, and deletes
// it when the test ends.
func (ts *AgentLockTestSuite) newAgent(name string) (agentID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("governance_agent_%s_%s", name, ts.suffix)
	password = "Agent-Password-" + name + "1!"
	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID: ts.agentTypeSnapshot.OUID,
		Type: "default",
		Attributes: map[string]interface{}{
			"name":           fmt.Sprintf("governance-lock-%s-%s", name, ts.suffix),
			"agent_username": username,
			"password":       password,
		},
	})
	ts.Require().NoError(err, "failed to create the test agent")
	t := ts.T()
	t.Cleanup(func() {
		if err := testutils.DeleteAgent(agentID); err != nil {
			t.Logf("teardown: failed to delete agent %s: %v", agentID, err)
		}
	})
	return agentID, username, password
}
