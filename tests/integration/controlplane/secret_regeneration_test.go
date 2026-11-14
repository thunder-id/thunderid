// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// regenerated is a regeneration's answer.
type regenerated struct {
	ClientSecret         string `json:"clientSecret"`
	RevocationIncomplete bool   `json:"revocationIncomplete"`
}

// regenerate asks the Control Plane for a new client secret at path, and returns the status and answer.
func (ts *ControlPlaneTestSuite) regenerate(path string) (int, regenerated, []byte) {
	ts.T().Helper()
	status, raw := ts.call(http.MethodPost, path, nil, nil)
	var answer regenerated
	if status == http.StatusOK {
		ts.Require().NoError(json.Unmarshal(raw, &answer), string(raw))
	}
	return status, answer, raw
}

// The Control Plane issues no tokens and runs no system flows, so it regenerates an application's secret
// without revoking anything, and still answers with the new secret once, uncached.
func (ts *ControlPlaneTestSuite) TestAnApplicationsSecretIsRegeneratedWithoutRevoking() {
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.ouID, Name: unique("cp-regen-app"), ClientID: unique("cp-regen-client"),
		ClientSecret: unique("CpRegenSecret"),
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	status, answer, raw := ts.regenerate("/applications/" + appID + "/client-secret/regenerate")

	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.NotEmpty(answer.ClientSecret)
	ts.False(answer.RevocationIncomplete, "nothing was left to revoke, so the revocation is not incomplete")
}

// The same holds for an agent.
func (ts *ControlPlaneTestSuite) TestAnAgentsSecretIsRegeneratedWithoutRevoking() {
	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID: ts.ouID, Type: "default", Attributes: map[string]interface{}{"name": unique("cp-regen-agent")},
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{{
			Type: "oauth2",
			Config: &testutils.AgentOAuthConfig{
				ClientSecret:            unique("CpRegenAgentSecret"),
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })

	status, answer, raw := ts.regenerate("/agents/" + agentID + "/client-secret/regenerate")

	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.NotEmpty(answer.ClientSecret)
	ts.False(answer.RevocationIncomplete)
}
