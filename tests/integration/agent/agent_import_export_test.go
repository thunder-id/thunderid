/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ---------------------------------------------------------------------------
// Local request/response types mirroring the export/import API shapes.
// ---------------------------------------------------------------------------

type agentExportRequest struct {
	Agents []string `json:"agents,omitempty"`
}

type agentExportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

type agentImportOptions struct {
	Upsert          bool   `json:"upsert"`
	ContinueOnError bool   `json:"continueOnError"`
	Target          string `json:"target"`
}

type agentImportRequest struct {
	Content   string                 `json:"content"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	DryRun    bool                   `json:"dryRun,omitempty"`
	Options   agentImportOptions     `json:"options"`
}

type agentImportSummary struct {
	TotalDocuments int `json:"totalDocuments"`
	Imported       int `json:"imported"`
	Failed         int `json:"failed"`
}

type agentImportItem struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Operation    string `json:"operation,omitempty"`
	Status       string `json:"status"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

type agentImportResponse struct {
	Summary agentImportSummary `json:"summary"`
	Results []agentImportItem  `json:"results"`
}

// ---------------------------------------------------------------------------
// Suite definition
// ---------------------------------------------------------------------------

// AgentImportExportSuite verifies the export → import lifecycle for agents.
type AgentImportExportSuite struct {
	suite.Suite
	ouID               string
	handleSuffix       string
	authFlowID         string
	registrationFlowID string
}

func TestAgentImportExportSuite(t *testing.T) {
	suite.Run(t, new(AgentImportExportSuite))
}

func (s *AgentImportExportSuite) SetupSuite() {
	s.handleSuffix = fmt.Sprintf("%d", time.Now().UnixNano())

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-ie-ou-" + s.handleSuffix,
		Name:        "Agent Import Export OU " + s.handleSuffix,
		Description: "OU for agent import-export lifecycle tests",
		Parent:      nil,
	})
	s.Require().NoError(err)
	s.ouID = ouID

	_, err = testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err, "failed to ensure default agent type exists")

	authFlowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	s.Require().NoError(err, "failed to get default auth flow ID")
	s.authFlowID = authFlowID

	regFlowID, err := testutils.GetFlowIDByHandle("default-flow", "REGISTRATION")
	s.Require().NoError(err, "failed to get default registration flow ID")
	s.registrationFlowID = regFlowID
}

func (s *AgentImportExportSuite) TearDownSuite() {
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestExportImportRoundTrip_EntityOnlyAgent creates an agent without OAuth,
// exports it, deletes it, re-imports, and verifies all fields are preserved.
func (s *AgentImportExportSuite) TestExportImportRoundTrip_EntityOnlyAgent() {
	agentName := "Agent RT Entity " + s.handleSuffix

	orig := Agent{
		OUID:        s.ouID,
		Type:        "default",
		Name:        agentName,
		Description: "Round-trip entity-only agent",
	}

	createdID, err := s.createAgent(orig)
	s.Require().NoError(err, "failed to create source agent")
	pre, err := s.agentGet(createdID)
	s.Require().NoError(err)

	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	s.Require().NotEmpty(exportResp.Resources, "expected exported YAML")
	yamlContent := exportResp.Resources

	s.Assert().Contains(yamlContent, "resource_type: agent")
	s.Assert().Contains(yamlContent, "id: "+createdID)
	s.Assert().Contains(yamlContent, "ouId: "+s.ouID)
	s.Assert().Contains(yamlContent, "name: "+agentName)
	s.Assert().Contains(yamlContent, "description: Round-trip entity-only agent")

	s.Require().NoError(s.deleteAgent(createdID))

	importResp, err := s.importAgents(agentImportRequest{
		Content: yamlContent,
		Options: agentImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.TotalDocuments)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Require().Equal(0, importResp.Summary.Failed)
	s.Assert().Equal("agent", importResp.Results[0].ResourceType)
	s.Assert().Equal("success", importResp.Results[0].Status)

	importedID := importResp.Results[0].ResourceID
	s.Require().NotEmpty(importedID)
	defer func() { _ = s.deleteAgent(importedID) }()

	restored, err := s.agentGet(importedID)
	s.Require().NoError(err)
	s.Assert().Equal(pre.Name, restored.Name)
	s.Assert().Equal(pre.Description, restored.Description)
	s.Assert().Equal(pre.OUID, restored.OUID)
}

// TestExportImportRoundTrip_AgentWithConfidentialOAuth creates an agent with
// confidential OAuth credentials, exports it, verifies the secret is
// parameterized, then re-imports with variable substitution.
func (s *AgentImportExportSuite) TestExportImportRoundTrip_AgentWithConfidentialOAuth() {
	agentName := "Agent RT OAuth " + s.handleSuffix
	clientID := "agent-rt-client-" + s.handleSuffix
	clientSecret := "agent-rt-secret-" + s.handleSuffix

	orig := Agent{
		OUID:        s.ouID,
		Type:        "default",
		Name:        agentName,
		Description: "Round-trip OAuth agent",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                clientID,
					ClientSecret:            clientSecret,
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PublicClient:            false,
				},
			},
		},
	}

	createdID, err := s.createAgent(orig)
	s.Require().NoError(err, "failed to create OAuth agent")
	defer func() { _ = s.deleteAgent(createdID) }()

	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	yamlContent := exportResp.Resources

	s.Assert().Contains(yamlContent, "resource_type: agent")
	// ClientID and ClientSecret are parameterized; the plaintext secret must not appear in YAML.
	s.Assert().NotContains(yamlContent, clientSecret, "client secret must not appear in exported YAML")
	s.Assert().Contains(yamlContent, "{{", "client_id should be a template variable")

	// Bare-`:` regression check.
	for _, line := range strings.Split(yamlContent, "\n") {
		s.Assert().NotEqual(":", strings.TrimSpace(line),
			"exported YAML must not contain a bare `:` key")
	}

	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
	})

	s.Require().NoError(s.deleteAgent(createdID))

	importResp, err := s.importAgents(agentImportRequest{
		Content:   yamlContent,
		Options:   agentImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
		Variables: vars,
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Assert().Equal("success", importResp.Results[0].Status)

	importedID := importResp.Results[0].ResourceID
	s.Require().NotEmpty(importedID)
	defer func() { _ = s.deleteAgent(importedID) }()

	restored, err := s.agentGet(importedID)
	s.Require().NoError(err)
	s.Assert().Equal(agentName, restored.Name)
	s.Require().NotEmpty(restored.InboundAuthConfig)
	cfg := restored.InboundAuthConfig[0].Config
	s.Require().NotNil(cfg)
	s.Assert().Equal(clientID, cfg.ClientID)
	s.Assert().Empty(cfg.ClientSecret, "GET response must not expose client secret")
}

// TestImportAgent_UpsertUpdates verifies that importing the same YAML twice with
// upsert=true results in the second call returning operation="update".
func (s *AgentImportExportSuite) TestImportAgent_UpsertUpdates() {
	agentName := "Agent Upsert " + s.handleSuffix

	orig := Agent{
		OUID:        s.ouID,
		Type:        "default",
		Name:        agentName,
		Description: "First version",
	}

	createdID, err := s.createAgent(orig)
	s.Require().NoError(err)
	defer func() { _ = s.deleteAgent(createdID) }()

	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	yamlContent := exportResp.Resources

	importResp, err := s.importAgents(agentImportRequest{
		Content: yamlContent,
		Options: agentImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Assert().Equal("update", importResp.Results[0].Operation)
	s.Assert().Equal("success", importResp.Results[0].Status)
}

// TestExportImportRoundTrip_AgentWithAllFields creates an agent with every exportable
// field populated, exports it, deletes it, re-imports, and asserts every field is
// preserved — catching any field that the exporter or importer silently drops.
func (s *AgentImportExportSuite) TestExportImportRoundTrip_AgentWithAllFields() {
	agentName := "Agent RT AllFields " + s.handleSuffix
	clientID := "agent-all-fields-client-" + s.handleSuffix
	clientSecret := "agent-all-fields-secret-" + s.handleSuffix

	orig := Agent{
		OUID:                      s.ouID,
		Type:                      "default",
		Name:                      agentName,
		Description:               "Round-trip all-fields agent",
		AuthFlowID:                s.authFlowID,
		RegistrationFlowID:        s.registrationFlowID,
		IsRegistrationFlowEnabled: true,
		Attributes:                json.RawMessage(`{"description":"eng-team"}`),
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                clientID,
					ClientSecret:            clientSecret,
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PublicClient:            false,
				},
			},
		},
	}

	createdID, err := s.createAgent(orig)
	s.Require().NoError(err, "failed to create all-fields agent")

	pre, err := s.agentGet(createdID)
	s.Require().NoError(err)

	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	s.Require().NotEmpty(exportResp.Resources)
	yamlContent := exportResp.Resources

	// Assert every significant field appears in the exported YAML.
	s.Assert().Contains(yamlContent, "resource_type: agent")
	s.Assert().Contains(yamlContent, "id: "+createdID)
	s.Assert().Contains(yamlContent, "ouId: "+s.ouID)
	s.Assert().Contains(yamlContent, "name: "+agentName)
	s.Assert().Contains(yamlContent, "description: Round-trip all-fields agent")
	s.Assert().Contains(yamlContent, "authFlowId: "+s.authFlowID)
	s.Assert().Contains(yamlContent, "registrationFlowId: "+s.registrationFlowID)
	s.Assert().Contains(yamlContent, "isRegistrationFlowEnabled: true")
	s.Assert().Contains(yamlContent, "attributes:")
	s.Assert().Contains(yamlContent, "eng-team")
	s.Assert().Contains(yamlContent, "inboundAuthConfig:")
	s.Assert().Contains(yamlContent, "client_credentials")
	s.Assert().Contains(yamlContent, "tokenEndpointAuthMethod: client_secret_basic")
	s.Assert().NotContains(yamlContent, clientSecret, "client secret must not appear in exported YAML")
	s.Assert().Contains(yamlContent, "{{", "client_id should be parameterized")

	s.Require().NoError(s.deleteAgent(createdID))

	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{
		"clientId":     clientID,
		"clientSecret": clientSecret,
	})

	importResp, err := s.importAgents(agentImportRequest{
		Content:   yamlContent,
		Options:   agentImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
		Variables: vars,
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Assert().Equal("success", importResp.Results[0].Status)

	importedID := importResp.Results[0].ResourceID
	s.Require().NotEmpty(importedID)
	defer func() { _ = s.deleteAgent(importedID) }()

	restored, err := s.agentGet(importedID)
	s.Require().NoError(err)

	s.Assert().Equal(pre.Name, restored.Name)
	s.Assert().Equal(pre.Description, restored.Description)
	s.Assert().Equal(pre.OUID, restored.OUID)
	s.Assert().Equal(pre.Type, restored.Type)
	s.Assert().Equal(pre.AuthFlowID, restored.AuthFlowID)
	s.Assert().Equal(pre.RegistrationFlowID, restored.RegistrationFlowID)
	s.Assert().Equal(pre.IsRegistrationFlowEnabled, restored.IsRegistrationFlowEnabled)
	s.Assert().NotEmpty(restored.Attributes, "agent attributes must survive the export→import round-trip")
	s.Assert().Contains(string(restored.Attributes), "eng-team")

	s.Require().Len(restored.InboundAuthConfig, 1)
	cfg := restored.InboundAuthConfig[0].Config
	s.Require().NotNil(cfg)
	s.Assert().Equal(clientID, cfg.ClientID)
	s.Assert().Empty(cfg.ClientSecret, "GET response must not expose client secret")
	s.Assert().Equal("client_secret_basic", cfg.TokenEndpointAuthMethod)
	s.Assert().False(cfg.PublicClient)
	s.Assert().Contains(cfg.GrantTypes, "client_credentials")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *AgentImportExportSuite) createAgent(a Agent) (string, error) {
	payload, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, testServerURL+agentBasePath, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read create agent response body: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create agent failed status=%d: %s", resp.StatusCode, string(body))
	}
	var created Agent
	if err := json.Unmarshal(body, &created); err != nil {
		return "", err
	}
	return created.ID, nil
}

func (s *AgentImportExportSuite) agentGet(agentID string) (*Agent, error) {
	req, err := http.NewRequest(http.MethodGet, testServerURL+agentBasePath+"/"+agentID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read get agent response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET agent/%s status=%d: %s", agentID, resp.StatusCode, string(body))
	}
	var a Agent
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *AgentImportExportSuite) deleteAgent(agentID string) error {
	req, err := http.NewRequest(http.MethodDelete, testServerURL+agentBasePath+"/"+agentID, nil)
	if err != nil {
		return err
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("delete agent/%s status=%d (failed to read body: %w)", agentID, resp.StatusCode, err)
		}
		return fmt.Errorf("delete agent/%s status=%d: %s", agentID, resp.StatusCode, string(body))
	}
	return nil
}

func (s *AgentImportExportSuite) exportAgents(reqBody agentExportRequest) (*agentExportResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/export", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read export response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("export failed status=%d: %s", resp.StatusCode, string(body))
	}
	var parsed agentExportResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse export response: %w (body=%s)", err, string(body))
	}
	return &parsed, nil
}

func (s *AgentImportExportSuite) importAgents(reqBody agentImportRequest) (*agentImportResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/import", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read import response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("import failed status=%d: %s", resp.StatusCode, string(body))
	}
	var parsed agentImportResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse import response: %w (body=%s)", err, string(body))
	}
	return &parsed, nil
}

// extractTemplateVariables walks the exported YAML and maps template variable
// names (e.g. X_CLIENT_ID) to caller-supplied values, keyed by YAML field name.
func (s *AgentImportExportSuite) extractTemplateVariables(
	yamlContent string, valuesByKey map[string]interface{},
) map[string]interface{} {
	out := make(map[string]interface{})
	lines := strings.Split(yamlContent, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "{{- range .") {
			end := strings.Index(trimmed, "}}")
			if end < 0 {
				continue
			}
			varRef := strings.TrimSpace(trimmed[len("{{- range ."):end])
			if varRef == "" || i == 0 {
				continue
			}
			prev := strings.TrimSpace(lines[i-1])
			key := strings.TrimSuffix(strings.TrimSpace(prev), ":")
			if val, ok := valuesByKey[key]; ok {
				out[varRef] = val
			}
			continue
		}

		idx := strings.Index(trimmed, "{{.")
		if idx < 0 {
			continue
		}
		end := strings.Index(trimmed[idx:], "}}")
		if end < 0 {
			continue
		}
		varRef := trimmed[idx+3 : idx+end]
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:colonIdx])
		if val, ok := valuesByKey[key]; ok {
			out[varRef] = val
		}
	}
	return out
}
