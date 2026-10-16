// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"maps"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AgentLockTestSuite checks that an agent signing in with a password locks under the agent policy,
// and that the shipped agent unlock flow releases it. The shared `default` agent type gains a
// username and a password for the suite's duration, in the OU it already points at.
type AgentLockTestSuite struct {
	suite.Suite
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	agentIDs          []string
	suffix            string
}

func TestAgentLockTestSuite(t *testing.T) {
	suite.Run(t, new(AgentLockTestSuite))
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

func (ts *AgentLockTestSuite) SetupSuite() {
	applyPolicy(ts.T(), agentLockPolicy())

	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	schema := maps.Clone(snapshot.Schema)
	if schema == nil {
		schema = map[string]interface{}{}
	}
	schema["agent_username"] = map[string]interface{}{"type": "string", "unique": true}
	schema["password"] = map[string]interface{}{"type": "string", "credential": true}
	_, err = testutils.CreateAgentType(testutils.UserType{
		DisplayName: snapshot.DisplayName,
		OUID:        snapshot.OUID,
		Schema:      schema,
	})
	ts.Require().NoError(err, "failed to give the default agent type a password")

	ts.suffix = uniqueSuffix()
}

func (ts *AgentLockTestSuite) TearDownSuite() {
	for _, agentID := range ts.agentIDs {
		if err := testutils.DeleteAgent(agentID); err != nil {
			ts.T().Logf("teardown: failed to delete agent %s: %v", agentID, err)
		}
	}
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
		}
	}
}

// The shipped agent unlock flow releases an agent's password lock: the agent is ACTIVE again and
// authenticates with its real secret.
func (ts *AgentLockTestSuite) TestTheAgentUnlockFlowReleasesTheAgent() {
	agentID, username, password := ts.newAgent("unlock")

	failAuthentication(ts.T(), "agent_username", username, lockThreshold)
	status := readEntityStatus(ts.T(), agentCollection, agentID)
	ts.Require().Equal("LOCKED", status.Value, "failing the agent's password to the threshold must lock it")
	ts.Require().Equal([]string{"credential"}, status.lockedScopes(), "the lock must be on the password")
	ts.Require().False(authenticate(ts.T(), "agent_username", username, password).succeeded(),
		"the lock must refuse the agent's real secret")

	runAdministrationFlow(ts.T(), agentUnlockFlowHandle, agentID)

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), agentCollection, agentID).Value,
		"the agent unlock flow must release the lock")
	ts.True(authenticate(ts.T(), "agent_username", username, password).succeeded(),
		"an unlocked agent must authenticate with its real secret")
}

// newAgent creates one agent with a username and password, in the OU its type belongs to.
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
	ts.agentIDs = append(ts.agentIDs, agentID)
	return agentID, username, password
}
