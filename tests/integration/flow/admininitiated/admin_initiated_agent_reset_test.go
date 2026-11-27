// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package admininitiated

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	adminAgentResetFlowHandle = "default-admin-agent-credential-reset-flow"
	// adminAgentResetCodeNoOwnerAddress is the agent service's refusal of an agent whose owner has no email address.
	adminAgentResetCodeNoOwnerAddress = "AGT-1070"
)

// resetAgent is an agent a test created, with what it signs in with.
type resetAgent struct {
	id       string
	name     string
	password string
	pin      string
	email    string
}

// An administrator resets an agent's password through the shipped agent flow. The link goes to the owner's email
// address, never to the agent's own, the agent's password stops working once it is sent, and the owner sets the
// new one from the link without signing in.
func (ts *AdminInitiatedResetTestSuite) TestTheOwnerResetsTheAgentsPasswordFromTheEmailedLink() {
	owner := ts.newUser("agent-owner", true)
	agent := ts.newAgent("owned", testutils.LocalAgentOwner(owner.id))

	started := ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id,
		"credential": "password"})

	ts.Require().NotNil(started.Step, "the start must be answered in the flow: %s", started.Body)
	ts.Require().Equal("INCOMPLETE", started.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		started.Body)
	ts.Equal("true", started.Step.Data.AdditionalData["emailSent"], "the administrator must be told it was sent")
	link := ts.awaitLinkAt(owner.email)
	ts.NotContains(string(started.Body), testutils.ExtractQueryParam(link, "inviteToken"),
		"the administrator must not get the link token")
	ts.NotContains(string(started.Body), owner.email, "the administrator must not get the owner's address")
	ts.False(ts.agentAuthenticates(agent, "password", agent.password),
		"the agent's old password must stop working once the link is sent")
	ts.True(ts.agentAuthenticates(agent, "pin", agent.pin), "the reset must leave the agent's PIN working")

	prompt := ts.openLink(link)
	ts.Require().Len(prompt.Data.Inputs, 1, "the owner must be asked for one credential")
	ts.Equal("password", prompt.Data.Inputs[0].Identifier, "the owner must be asked for the password")
	done := ts.submit(prompt, adminResetSubmitAction, map[string]string{"password": "Agent-New-Pass-1!"})

	ts.Require().NotNil(done.Step, "the new password must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "setting the new password must complete the reset: %s", done.Body)
	ts.True(ts.agentAuthenticates(agent, "password", "Agent-New-Pass-1!"), "the new password must work")
	ts.False(ts.agentAuthenticates(agent, "password", agent.password), "the old password must stay blocked")
	ts.True(ts.authenticates(owner.username, "password", owner.password),
		"the owner's own password must be untouched")
}

// An owner managed in another identity system and identified by email receives the link at that address.
func (ts *AdminInitiatedResetTestSuite) TestAnExternalOwnerIdentifiedByEmailReceivesTheLink() {
	address := "external_owner_" + ts.suffix + "@example.com"
	agent := ts.newAgent("external", testutils.ExternalAgentOwner(address, "email"))

	started := ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id,
		"credential": "pin"})

	ts.Require().NotNil(started.Step, "the start must be answered in the flow: %s", started.Body)
	ts.Require().Equal("INCOMPLETE", started.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		started.Body)
	link := ts.awaitLinkAt(address)
	ts.False(ts.agentAuthenticates(agent, "pin", agent.pin), "the chosen credential must be blocked")
	ts.True(ts.agentAuthenticates(agent, "password", agent.password), "the other credential must still work")

	done := ts.submit(ts.openLink(link), adminResetSubmitAction, map[string]string{"pin": "246801"})

	ts.Require().NotNil(done.Step, "the new PIN must be answered in the flow: %s", done.Body)
	ts.Equal("COMPLETE", done.Step.FlowStatus, "setting the new PIN must complete the reset: %s", done.Body)
	ts.True(ts.agentAuthenticates(agent, "pin", "246801"), "the new PIN must work")
}

// An agent whose owner has no usable email address is refused before anything is sent, even when the agent has
// an email address of its own, and its credential keeps working.
func (ts *AdminInitiatedResetTestSuite) TestAnAgentWhoseOwnerHasNoAddressIsRefusedBeforeDelivery() {
	ownerWithoutEmail := ts.newUser("agent-owner-no-email", false)
	ownerAgent := ts.newAgent("owner-agent", testutils.LocalAgentOwner(ownerWithoutEmail.id))
	for name, owner := range map[string]*testutils.AgentOwner{
		"a local owner without an email":          testutils.LocalAgentOwner(ownerWithoutEmail.id),
		"an owner that is an agent with an email": testutils.LocalAgentOwner(ownerAgent.id),
		"an external owner identified by mobile":  testutils.ExternalAgentOwner("+94771234567", "mobile"),
		"an external owner identified by an ID":   testutils.ExternalAgentOwner("ext-"+ts.suffix, "uuid"),
	} {
		ts.Run(name, func() {
			ts.smtp.ClearEmails()
			agent := ts.newAgent("no-address-"+strings.ReplaceAll(name, " ", "-"), owner)

			refused := ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id,
				"credential": "password"})

			ts.requireRefused(refused, adminAgentResetCodeNoOwnerAddress, "a reset the owner cannot receive")
			ts.requireNoEmail("nothing may be sent, to the agent's own address or anywhere else")
			ts.True(ts.agentAuthenticates(agent, "password", agent.password), "the password must keep working")
		})
	}
}

// An address in the start is not used: the link goes to the owner only.
func (ts *AdminInitiatedResetTestSuite) TestAnAddressInAnAgentResetStartIsNotUsed() {
	owner := ts.newUser("agent-owner-start-address", true)
	agent := ts.newAgent("start-address", testutils.LocalAgentOwner(owner.id))
	elsewhere := "elsewhere_" + ts.suffix + "@example.com"

	started := ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id,
		"credential": "password", "email": elsewhere})

	ts.Require().NotNil(started.Step, "the start must be answered in the flow: %s", started.Body)
	ts.Require().Equal("INCOMPLETE", started.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		started.Body)
	ts.awaitLinkAt(owner.email)
	ts.requireNoEmail("the link must go to the owner only")
}

// A suspended agent, and a credential the agent's type does not declare, are refused before anything is sent,
// and the agent's password keeps working.
func (ts *AdminInitiatedResetTestSuite) TestAnAgentResetTheServiceRefusesSendsNothing() {
	owner := ts.newUser("agent-owner-refused", true)
	for name, test := range map[string]struct {
		credential string
		suspend    bool
		code       string
	}{
		"a suspended agent":        {credential: "password", suspend: true, code: adminResetCodeSuspended},
		"an undeclared credential": {credential: "apiKey", code: "AGT-1016"},
	} {
		ts.Run(name, func() {
			ts.smtp.ClearEmails()
			agent := ts.newAgent(fmt.Sprintf("refused-%t", test.suspend), testutils.LocalAgentOwner(owner.id))
			if test.suspend {
				ts.suspendAgent(agent)
			}

			refused := ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id,
				"credential": test.credential})

			ts.requireRefused(refused, test.code, "a reset the agent service refuses")
			ts.requireNoEmail("nothing may be sent")
			if !test.suspend {
				ts.True(ts.agentAuthenticates(agent, "password", agent.password), "the password must keep working")
			}
		})
	}
}

// A credential change after the link was sent voids it, as it does a user's.
func (ts *AdminInitiatedResetTestSuite) TestAnAgentCredentialChangeAfterTheLinkWasSentVoidsIt() {
	owner := ts.newUser("agent-owner-voided", true)
	agent := ts.newAgent("voided", testutils.LocalAgentOwner(owner.id))
	ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id, "credential": "password"})
	link := ts.awaitLinkAt(owner.email)

	ts.setAgentCredentials(agent, map[string]string{"password": "Set-By-Admin-1!"})

	ts.requireRefused(ts.openLinkRaw(link), adminResetCodeVoided, "a link a later change voided")
	ts.True(ts.agentAuthenticates(agent, "password", "Set-By-Admin-1!"), "the later change must stand")
}

// An administrator confined to an organization unit cannot reset an agent outside it: the start is refused before
// anything is sent, and the agent's password keeps working.
func (ts *AdminInitiatedResetTestSuite) TestAnOrganizationUnitAdministratorCannotResetAnAgentOutsideItsUnit() {
	token, childOU := ts.ouAdministratorTokenFor("agent")
	handle := ts.copyAgentFlowIn("admin-agent-reset-ou-"+ts.suffix, childOU)
	owner := ts.newUser("agent-owner-ou", true)
	agent := ts.newAgent("ou-outside", testutils.LocalAgentOwner(owner.id))

	refused := ts.start(token, handle, map[string]string{"subject": agent.id, "credential": "password"})

	ts.Require().NotNil(refused.Step, "the refusal must be answered in the flow: %s", refused.Body)
	ts.Equal("ERROR", refused.Step.FlowStatus, "an agent outside the unit must be refused: %s", refused.Body)
	ts.requireNoEmail("nothing may be sent for an agent outside the unit")
	ts.True(ts.agentAuthenticates(agent, "password", agent.password), "the agent must keep its password")
}

// A link whose agent was deleted after it was sent is voided.
func (ts *AdminInitiatedResetTestSuite) TestALinkWhoseAgentWasDeletedIsVoided() {
	owner := ts.newUser("agent-owner-deleted-agent", true)
	agent := ts.newAgent("deleted-after-send", testutils.LocalAgentOwner(owner.id))
	ts.start(ts.adminToken(), ts.agentFlowHandle(), map[string]string{"subject": agent.id, "credential": "password"})
	link := ts.awaitLinkAt(owner.email)
	ts.Require().NoError(testutils.DeleteAgent(agent.id), "precondition: the agent is deleted")

	ts.requireRefused(ts.openLinkRaw(link), adminResetCodeVoided, "a link whose agent is gone")
}

// setupAgents copies the shipped agent flow, with the suite's sender, and creates the suite's agent type, both
// deleted when the suite ends.
func (ts *AdminInitiatedResetTestSuite) setupAgents() {
	ts.agentHandle = ts.copyAgentFlow()
	ts.agentTypeHandle = ts.createAgentType()
}

// agentFlowHandle returns the handle of the suite's copy of the shipped agent flow.
func (ts *AdminInitiatedResetTestSuite) agentFlowHandle() string {
	return ts.agentHandle
}

// copyAgentFlow copies the shipped agent flow with the suite's sender and returns its handle.
func (ts *AdminInitiatedResetTestSuite) copyAgentFlow() string {
	return ts.copyAgentFlowIn("admin-agent-reset-"+ts.suffix, "")
}

// copyAgentFlowIn copies the shipped agent flow under the handle given, owned by ouID or the default unit when
// empty, with the suite's sender, deletes it when the test ends, and returns its handle.
func (ts *AdminInitiatedResetTestSuite) copyAgentFlowIn(handle, ouID string) string {
	t := ts.T()
	flowID, err := testutils.CopyFlowWithSendersInOU(adminAgentResetFlowHandle, adminInitiatedFlowType, handle, ouID,
		map[string]string{adminResetSendNode: ts.senderID}, nil)
	ts.Require().NoError(err, "failed to copy the shipped administrator agent credential reset flow")
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return handle
}

// newAgent creates an agent of the suite's agent type with the owner given, a password, a PIN and an email of its
// own.
func (ts *AdminInitiatedResetTestSuite) newAgent(name string, owner *testutils.AgentOwner) resetAgent {
	t := ts.T()
	agent := resetAgent{name: fmt.Sprintf("reset-agent-%s-%s", name, ts.suffix), password: "Agent-Old-Pass-1!",
		pin: "975310"}
	agent.email = agent.name + "@agents.example.com"
	var err error
	agent.id, err = testutils.CreateAgent(testutils.Agent{OUID: ts.ouID, Type: ts.agentTypeHandle, Owner: owner,
		Attributes: map[string]interface{}{"name": agent.name, "email": agent.email, "password": agent.password,
			"pin": agent.pin}})
	ts.Require().NoError(err, "failed to create agent %s", agent.name)
	t.Cleanup(func() {
		if err := testutils.DeleteAgent(agent.id); err != nil {
			t.Logf("cleanup: failed to delete agent %s: %v", agent.name, err)
		}
	})
	return agent
}

// createAgentType creates an agent type declaring a password, a PIN and an email of the agent's own, and returns
// its handle.
func (ts *AdminInitiatedResetTestSuite) createAgentType() string {
	t := ts.T()
	handle := "admin_reset_agent_" + ts.suffix
	typeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: handle, DisplayName: "Admin Reset Agent", OUID: ts.ouID,
		Schema: map[string]interface{}{
			"name":     map[string]interface{}{"type": "string", "required": true, "unique": true},
			"email":    map[string]interface{}{"type": "email"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"pin":      map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "failed to create the agent type")
	t.Cleanup(func() {
		if err := testutils.DeleteAgentType(typeID); err != nil {
			t.Logf("cleanup: failed to delete the agent type: %v", err)
		}
	})
	return handle
}

// suspendAgent suspends the agent through the shipped suspension flow.
func (ts *AdminInitiatedResetTestSuite) suspendAgent(agent resetAgent) {
	flowID, err := testutils.GetFlowIDByHandle("default-agent-suspend-flow", "ADMINISTRATION")
	ts.Require().NoError(err)
	result := ts.continueAs(ts.adminToken(),
		map[string]interface{}{"flowId": flowID, "inputs": map[string]string{"subject": agent.id}})
	ts.Require().NotNil(result.Step, "precondition: the agent is suspended: %s", result.Body)
	ts.Require().Equal("COMPLETE", result.Step.FlowStatus, "precondition: the agent is suspended: %s", result.Body)
}

// agentAuthenticates reports whether the credential signs the agent in through the credentials endpoint.
func (ts *AdminInitiatedResetTestSuite) agentAuthenticates(agent resetAgent, credentialKey, value string) bool {
	ok, err := testutils.AuthenticateWithCredential("name", agent.name, credentialKey, value)
	ts.Require().NoError(err)
	return ok
}

// setAgentCredentials sets the agent's credentials, as an administrator does through the API.
func (ts *AdminInitiatedResetTestSuite) setAgentCredentials(agent resetAgent, credentials map[string]string) {
	body, err := json.Marshal(map[string]interface{}{"credentials": credentials})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/agents/"+agent.id+"/update-credentials",
		bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusNoContent, resp.StatusCode, "failed to set the credentials of %s: %s", agent.name,
		respBody)
}
