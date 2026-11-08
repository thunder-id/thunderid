// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	agentTypeSnapshot  *testutils.AgentTypeSnapshot
	handleSuffix       string
	authFlowID         string
	registrationFlowID string

	// An isolated auth flow referenced by handle, for the handle-only upsert test.
	handleAuthFlowID     string
	handleAuthFlowHandle string
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

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err, "failed to snapshot the default agent type")
	s.agentTypeSnapshot = snapshot

	s.Require().NoError(s.installAgentType(nil), "failed to ensure default agent type exists")

	authFlowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	s.Require().NoError(err, "failed to get default auth flow ID")
	s.authFlowID = authFlowID

	regFlowID, err := testutils.GetFlowIDByHandle("default-flow", "REGISTRATION")
	s.Require().NoError(err, "failed to get default registration flow ID")
	s.registrationFlowID = regFlowID

	s.handleAuthFlowHandle = "agent-ie-handle-flow-" + s.handleSuffix
	handleFlowID, err := testutils.CreateIsolatedAuthFlow(s.handleAuthFlowHandle)
	s.Require().NoError(err, "failed to create the isolated auth flow referenced by handle")
	s.handleAuthFlowID = handleFlowID
}

func (s *AgentImportExportSuite) TearDownSuite() {
	if s.handleAuthFlowID != "" {
		if err := testutils.DeleteFlow(s.handleAuthFlowID); err != nil {
			s.T().Logf("teardown: failed to delete the isolated auth flow: %v", err)
		}
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if s.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(s.agentTypeSnapshot); err != nil {
			s.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if s.ouID != "" && agentTypeRestored {
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
		Attributes:  agentAttrs(agentName, nil),
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
	s.Assert().Contains(yamlContent, "attributes:")
	s.Assert().Contains(yamlContent, `name: "`+agentName+`"`)
	s.Assert().Contains(yamlContent, "description: Round-trip entity-only agent")
	s.Assert().NotContains(yamlContent, "credentials:", "an agent with no password set exports none")

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
	s.Assert().Equal(pre.AttrName(), restored.AttrName())
	s.Assert().Equal(pre.Description, restored.Description)
	s.Assert().Equal(pre.OUID, restored.OUID)
}

// TestExportImportRoundTrip_ExternalOwner verifies an owner managed elsewhere is exported as an
// object and imported again without being looked up, as a control plane applies it to a gateway.
func (s *AgentImportExportSuite) TestExportImportRoundTrip_ExternalOwner() {
	owner := testutils.ExternalAgentOwner("0199a1b2-7c3d-7e4f-9a8b-0000000000aa", "uuid")
	createdID, err := s.createAgent(Agent{
		OUID:       s.ouID,
		Type:       "default",
		Owner:      owner,
		Attributes: agentAttrs("Agent RT External Owner "+s.handleSuffix, nil),
	})
	s.Require().NoError(err, "failed to create source agent")

	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	yamlContent := exportResp.Resources
	s.Assert().Contains(yamlContent, "id: "+owner.ID)
	s.Assert().Contains(yamlContent, "isLocal: false")

	s.Require().NoError(s.deleteAgent(createdID))

	importResp, err := s.importAgents(agentImportRequest{
		Content: yamlContent,
		Options: agentImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)

	importedID := importResp.Results[0].ResourceID
	defer func() { _ = s.deleteAgent(importedID) }()

	restored, err := s.agentGet(importedID)
	s.Require().NoError(err)
	s.Require().NotNil(restored.Owner)
	s.Assert().Equal(owner.ID, restored.Owner.ID)
	s.Require().NotNil(restored.Owner.IsLocal)
	s.Assert().False(*restored.Owner.IsLocal)
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
		Attributes:  agentAttrs(agentName, nil),
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
	s.Assert().Equal(agentName, restored.AttrName())
	s.Require().NotEmpty(restored.InboundAuthConfig)
	cfg := restored.InboundAuthConfig[0].Config
	s.Require().NotNil(cfg)
	s.Assert().Equal(clientID, cfg.ClientID)
	s.Assert().Empty(cfg.ClientSecret, "GET response must not expose client secret")

	// Creation selects sub_type, so the selection has to survive the export and the import as well.
	s.Assert().Contains(yamlContent, "sub_type", "the exported selection must carry the claim")
	s.Require().NotNil(cfg.Token)
	s.Require().NotNil(cfg.Token.AccessToken)
	s.Require().NotNil(cfg.Token.AccessToken.ClientConfig)
	s.Assert().Contains(cfg.Token.AccessToken.ClientConfig.Attributes, "sub_type")
}

// TestImportAgent_UpsertUpdates verifies that importing the same YAML twice with
// upsert=true results in the second call returning operation="update".
func (s *AgentImportExportSuite) TestImportAgent_UpsertUpdates() {
	agentName := "Agent Upsert " + s.handleSuffix

	orig := Agent{
		OUID:        s.ouID,
		Type:        "default",
		Attributes:  agentAttrs(agentName, nil),
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

// TestImportAgent_UpsertWithAuthFlowHandleOnly covers the declarative re-import case: a document
// that binds its auth flow by handle and carries no inboundAuthConfig. The handle must resolve on
// the update path so the agent keeps its inbound client, instead of the document being read as
// carrying no inbound fields at all. The agent starts on a different flow, so the assertion proves
// the handle was resolved rather than the previous binding merely being left in place.
func (s *AgentImportExportSuite) TestImportAgent_UpsertWithAuthFlowHandleOnly() {
	agentName := "Agent Handle Only " + s.handleSuffix

	createdID, err := s.createAgent(Agent{
		OUID:       s.ouID,
		Type:       "default",
		Attributes: agentAttrs(agentName, nil),
		AuthFlowID: s.authFlowID,
	})
	s.Require().NoError(err)
	defer func() { _ = s.deleteAgent(createdID) }()

	yamlContent := fmt.Sprintf(
		"resource_type: agent\nid: %s\nouId: %s\ntype: default\nattributes:\n  name: %s\nauthFlowHandle: %s\n",
		createdID, s.ouID, agentName, s.handleAuthFlowHandle)

	importResp, err := s.importAgents(agentImportRequest{
		Content: yamlContent,
		Options: agentImportOptions{Upsert: true, ContinueOnError: false, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Require().Equal("update", importResp.Results[0].Operation)
	s.Require().Equal("success", importResp.Results[0].Status)

	updated, err := s.agentGet(createdID)
	s.Require().NoError(err)
	s.Assert().Equal(s.handleAuthFlowID, updated.AuthFlowID,
		"authFlowHandle must resolve to its own flow ID on a handle-only upsert")
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
		Description:               "Round-trip all-fields agent",
		AuthFlowID:                s.authFlowID,
		RegistrationFlowID:        s.registrationFlowID,
		IsRegistrationFlowEnabled: true,
		Attributes:                withName(agentName, json.RawMessage(`{"description":"eng-team"}`)),
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
	s.Assert().Contains(yamlContent, "attributes:")
	s.Assert().Contains(yamlContent, `name: "`+agentName+`"`)
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

	s.Assert().Equal(pre.AttrName(), restored.AttrName())
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

// An agent's password is exported as a placeholder, never as its value or its hash, and an import
// that fills the placeholder leaves the agent able to sign in with the value given.
func (s *AgentImportExportSuite) TestExportImportRoundTrip_AgentWithPassword() {
	agentName := "Agent RT Password " + s.handleSuffix
	original := "Original-Pass1!" + s.handleSuffix
	imported := "Imported-Pass1!" + s.handleSuffix

	createdID, err := s.createAgent(Agent{
		OUID:       s.ouID,
		Type:       "default",
		Attributes: agentAttrs(agentName, map[string]interface{}{"password": original}),
	})
	s.Require().NoError(err)
	defer func() { _ = s.deleteAgent(createdID) }()
	s.Require().Equal(http.StatusOK, s.authenticateAgent(agentName, original),
		"the agent cannot sign in with the password it was created with")

	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	yamlContent := exportResp.Resources
	s.Assert().Contains(yamlContent, "credentials:", "an agent with a password must export its credentials")
	s.Assert().NotContains(yamlContent, original, "the password must not appear in the export")
	s.Assert().NotContains(yamlContent, "storageAlgo", "the stored hash must not appear in the export")
	vars := s.extractTemplateVariables(yamlContent, map[string]interface{}{"password": imported})
	s.Require().Len(vars, 1, "the password is not a template variable: %s", yamlContent)
	for name := range vars {
		s.Assert().Contains(exportResp.EnvironmentVariables, name)
	}

	s.Require().NoError(s.deleteAgent(createdID))
	importResp, err := s.importAgents(agentImportRequest{
		Content:   yamlContent,
		Options:   agentImportOptions{Upsert: true, Target: "runtime"},
		Variables: vars,
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	importedID := importResp.Results[0].ResourceID
	defer func() { _ = s.deleteAgent(importedID) }()

	s.Equal(http.StatusOK, s.authenticateAgent(agentName, imported),
		"the imported agent cannot sign in with the password the import gave it")
	s.NotEqual(http.StatusOK, s.authenticateAgent(agentName, original))
	restored, err := s.agentGet(importedID)
	s.Require().NoError(err)
	s.NotContains(string(restored.Attributes), imported, "a read must not return the password")
}

// Importing onto an agent that is already there replaces its password. A placeholder filled with
// nothing is refused: the profile is written, and the agent keeps the password it has.
func (s *AgentImportExportSuite) TestImportAgent_UpsertReplacesThePassword() {
	agentName := "Agent Upsert Password " + s.handleSuffix
	original := "Original-Pass2!" + s.handleSuffix
	rotated := "Rotated-Pass2!" + s.handleSuffix

	createdID, err := s.createAgent(Agent{
		OUID:       s.ouID,
		Type:       "default",
		Attributes: agentAttrs(agentName, map[string]interface{}{"password": original}),
	})
	s.Require().NoError(err)
	defer func() { _ = s.deleteAgent(createdID) }()
	exportResp, err := s.exportAgents(agentExportRequest{Agents: []string{createdID}})
	s.Require().NoError(err)
	importWith := func(password string) *agentImportResponse {
		importResp, err := s.importAgents(agentImportRequest{
			Content: exportResp.Resources,
			Options: agentImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
			Variables: s.extractTemplateVariables(exportResp.Resources,
				map[string]interface{}{"password": password}),
		})
		s.Require().NoError(err)
		s.Require().Len(importResp.Results, 1)
		s.Equal("update", importResp.Results[0].Operation)
		return importResp
	}

	importResp := importWith(rotated)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	s.Equal(http.StatusOK, s.authenticateAgent(agentName, rotated),
		"the agent cannot sign in with the password the import gave it")
	s.NotEqual(http.StatusOK, s.authenticateAgent(agentName, original),
		"the replaced password still signs the agent in")

	importResp = importWith("")
	s.Equal("failed", importResp.Results[0].Status)
	s.Equal("AGT-1016", importResp.Results[0].Code, importResp.Results[0].Message)
	s.Contains(importResp.Results[0].Message, "credential update failed")
	s.Equal(http.StatusOK, s.authenticateAgent(agentName, rotated),
		"an empty password replaced the one the agent had")
}

// A Data Plane imports what a Control Plane exports: a password named as a secret the Data Plane
// holds, which the import fills in before the agent service stores it.
func (s *AgentImportExportSuite) TestImportAgent_ResolvesAPasswordFromTheSecretStore() {
	agentName := "Agent Secret Password " + s.handleSuffix
	secretName := "AGENT_SECRET_PASSWORD_" + s.handleSuffix
	password := "Secret-Pass3!" + s.handleSuffix
	s.Require().Equal(http.StatusCreated, s.send(http.MethodPost, "/secrets",
		map[string]interface{}{"name": secretName, "value": password}))
	defer func() { _ = s.send(http.MethodDelete, "/secrets/"+secretName, nil) }()

	importResp, err := s.importAgents(agentImportRequest{
		Content: fmt.Sprintf("resource_type: agent\ntype: default\nouId: %s\nattributes:\n  name: %s\n"+
			"credentials:\n  password: sec:%s\n", s.ouID, agentName, secretName),
		Options: agentImportOptions{Upsert: true, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Equal(1, importResp.Summary.Imported, "import results: %+v", importResp.Results)
	defer func() { _ = s.deleteAgent(importResp.Results[0].ResourceID) }()

	s.Equal(http.StatusOK, s.authenticateAgent(agentName, password),
		"the agent cannot sign in with the password the secret holds")
}

// A credential the agent's type does not declare is refused, so a secret the document names is never
// stored where a read returns it. A new agent refused one is not left behind.
func (s *AgentImportExportSuite) TestImportAgent_RefusesACredentialTheTypeDoesNotDeclare() {
	agentName := "Agent Undeclared Credential " + s.handleSuffix
	agentID := "01900000-0000-7000-8000-" + s.handleSuffix[len(s.handleSuffix)-12:]
	content := fmt.Sprintf("resource_type: agent\nid: %s\ntype: default\nouId: %s\nattributes:\n  name: %s\n"+
		"credentials:\n  pin: \"1234\"\n", agentID, s.ouID, agentName)

	importResp, err := s.importAgents(agentImportRequest{
		Content: content,
		Options: agentImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Len(importResp.Results, 1)
	s.Equal("failed", importResp.Results[0].Status)
	s.Equal("AGT-1016", importResp.Results[0].Code, importResp.Results[0].Message)
	_, err = s.agentGet(agentID)
	s.Error(err, "the agent refused its credentials was left behind")

	createdID, err := s.createAgent(Agent{OUID: s.ouID, Type: "default", Attributes: agentAttrs(agentName, nil)})
	s.Require().NoError(err)
	defer func() { _ = s.deleteAgent(createdID) }()
	content = strings.Replace(content, "id: "+agentID, "id: "+createdID, 1)

	importResp, err = s.importAgents(agentImportRequest{
		Content: content,
		Options: agentImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	s.Require().NoError(err)
	s.Require().Len(importResp.Results, 1)
	s.Equal("failed", importResp.Results[0].Status)
	s.Equal("update", importResp.Results[0].Operation)
	s.Equal("AGT-1016", importResp.Results[0].Code)
	s.Contains(importResp.Results[0].Message, "credential update failed")
}

// A credential is a string, and a new agent needs a value for each credential its document names. A
// new agent refused either is not left behind.
func (s *AgentImportExportSuite) TestImportAgent_RefusesANewAgentsUnusableCredential() {
	for i, tc := range []struct {
		name, value, code string
	}{
		{name: "not a string", value: "1234", code: "AGT-1001"},
		{name: "empty", value: `""`, code: "AGT-1016"},
	} {
		s.Run(tc.name, func() {
			agentID := fmt.Sprintf("01900000-0000-7000-9000-%011d%d", time.Now().UnixNano()%1e11, i)
			importResp, err := s.importAgents(agentImportRequest{
				Content: fmt.Sprintf("resource_type: agent\nid: %s\ntype: default\nouId: %s\n"+
					"attributes:\n  name: %s\ncredentials:\n  password: %s\n",
					agentID, s.ouID, "Agent Unusable Password "+tc.name+" "+s.handleSuffix, tc.value),
				Options: agentImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
			})
			s.Require().NoError(err)
			s.Require().Len(importResp.Results, 1)
			s.Equal("failed", importResp.Results[0].Status)
			s.Equal(tc.code, importResp.Results[0].Code, importResp.Results[0].Message)
			_, err = s.agentGet(agentID)
			s.Error(err, "the agent refused its credentials was left behind")
		})
	}
}

// Two agents whose display names normalize alike each carry their own ID in the password variable, so
// the export holding both is written and each agent keeps a password variable of its own.
func (s *AgentImportExportSuite) TestExportAgent_NamesTwoAgentsWithOneNormalizedNameApart() {
	// Without a display attribute an agent is named by its ID, which no other agent shares.
	s.Require().NoError(s.installAgentType(&testutils.UserTypeSystemAttributes{Display: "name"}))
	defer func() { s.NoError(s.installAgentType(nil)) }()
	var ids []string
	names := []string{"Agent Clash " + s.handleSuffix, "Agent-Clash-" + s.handleSuffix}
	for _, name := range names {
		id, err := s.createAgent(Agent{
			OUID:       s.ouID,
			Type:       "default",
			Attributes: agentAttrs(name, map[string]interface{}{"password": "Clash-Pass1!" + s.handleSuffix}),
		})
		s.Require().NoError(err)
		defer func() { _ = s.deleteAgent(id) }()
		ids = append(ids, id)
	}

	exported, err := s.exportAgents(agentExportRequest{Agents: ids})

	s.Require().NoError(err, "agents whose names normalize alike must not refuse the export")
	first := testutils.ExportVariableName("AGENT", ids[0], "PASSWORD")
	second := testutils.ExportVariableName("AGENT", ids[1], "PASSWORD")
	s.NotEqual(first, second, "the two agents must not share a password variable")
	s.Contains(exported.Resources, "{{."+first+"}}")
	s.Contains(exported.Resources, "{{."+second+"}}")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// installAgentType points the default agent type at the suite's unit, with a password credential and
// the given system attributes.
func (s *AgentImportExportSuite) installAgentType(systemAttributes *testutils.UserTypeSystemAttributes) error {
	_, err := testutils.CreateAgentType(testutils.UserType{
		Handle:           "default",
		DisplayName:      "Default",
		OUID:             s.ouID,
		SystemAttributes: systemAttributes,
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
			"description": map[string]interface{}{"type": "string"},
			"password":    map[string]interface{}{"type": "string", "credential": true},
		},
	})
	return err
}

// send sends a JSON request to the server under test and returns the status.
func (s *AgentImportExportSuite) send(method, path string, body interface{}) int {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// authenticateAgent signs an agent in by its name and password and returns the status.
func (s *AgentImportExportSuite) authenticateAgent(name, password string) int {
	payload, err := json.Marshal(map[string]interface{}{
		"identifiers": map[string]interface{}{"name": name},
		"credentials": map[string]interface{}{"password": password},
	})
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/auth/credentials/authenticate",
		bytes.NewReader(payload))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

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
