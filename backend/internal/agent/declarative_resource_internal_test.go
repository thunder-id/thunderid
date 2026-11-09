// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/agent/model"
)

type ParseToAgentRequestTestSuite struct {
	suite.Suite
}

func TestParseToAgentRequestTestSuite(t *testing.T) {
	suite.Run(t, new(ParseToAgentRequestTestSuite))
}

func (s *ParseToAgentRequestTestSuite) TestParseToAgentRequest_AllFieldsParsed() {
	yamlData := `
id: test-agent-001
ouId: ou-123
type: service-agent
description: A test agent
owner:
  id: owner-id-123
  idType: uuid
  isLocal: true
authFlowId: flow-abc
registrationFlowId: flow-reg-xyz
themeId: theme-blue
layoutId: layout-standard
allowedUserTypes:
  - person
  - external
attributes:
  name: Test Agent
  department: engineering
`
	var req model.AgentRequestWithID
	err := yaml.Unmarshal([]byte(yamlData), &req)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "test-agent-001", req.ID)
	assert.Equal(s.T(), "ou-123", req.OUID)
	assert.Equal(s.T(), "service-agent", req.Type)
	assert.Equal(s.T(), "Test Agent", req.Attributes["name"])
	assert.Equal(s.T(), "A test agent", req.Description)
	s.Require().NotNil(req.Owner)
	assert.Equal(s.T(), "owner-id-123", req.Owner.ID)
	assert.Equal(s.T(), providers.AgentOwnerIDTypeUUID, req.Owner.IDType)
	assert.True(s.T(), req.Owner.Local())
	assert.Equal(s.T(), "flow-abc", req.AuthFlowID)
	assert.Equal(s.T(), "flow-reg-xyz", req.RegistrationFlowID)
	assert.Equal(s.T(), "theme-blue", req.ThemeID)
	assert.Equal(s.T(), "layout-standard", req.LayoutID)
	assert.Equal(s.T(), []string{"person", "external"}, req.AllowedUserTypes)
	assert.Equal(s.T(), "engineering", req.Attributes["department"])
}

func (s *ParseToAgentRequestTestSuite) TestParseToAgentRequest_MinimalFields() {
	yamlData := `
id: min-agent
ouId: ou-1
type: bot
attributes:
  name: Minimal Agent
`
	var req model.AgentRequestWithID
	err := yaml.Unmarshal([]byte(yamlData), &req)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "min-agent", req.ID)
	assert.Equal(s.T(), "ou-1", req.OUID)
	assert.Equal(s.T(), "bot", req.Type)
	assert.Equal(s.T(), "Minimal Agent", req.Attributes["name"])
	assert.Empty(s.T(), req.Description)
	assert.Empty(s.T(), req.AuthFlowID)
	assert.Empty(s.T(), req.ThemeID)
	assert.Nil(s.T(), req.InboundAuthConfig)
}

func (s *ParseToAgentRequestTestSuite) TestParseToAgentRequest_WithOAuthConfig() {
	yamlData := `
id: oauth-agent
ouId: ou-1
type: service-agent
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: client-abc
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
      publicClient: false
`
	var req model.AgentRequestWithID
	err := yaml.Unmarshal([]byte(yamlData), &req)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), req.InboundAuthConfig, 1)
	assert.Equal(s.T(), providers.OAuthInboundAuthType, req.InboundAuthConfig[0].Type)
	oauthCfg := req.InboundAuthConfig[0].OAuthConfig
	assert.NotNil(s.T(), oauthCfg)
	assert.Equal(s.T(), "client-abc", oauthCfg.ClientID)
	assert.Contains(s.T(), oauthCfg.GrantTypes, providers.GrantType("client_credentials"))
	assert.False(s.T(), oauthCfg.PublicClient)
}

func (s *ParseToAgentRequestTestSuite) TestParseToAgentRequest_InvalidYAML() {
	yamlData := `
id: bad-agent
invalid: [unclosed bracket
`
	var req model.AgentRequestWithID
	err := yaml.Unmarshal([]byte(yamlData), &req)

	assert.Error(s.T(), err)
}

type MakeAgentEntityParserTestSuite struct {
	suite.Suite
}

func TestMakeAgentEntityParserTestSuite(t *testing.T) {
	suite.Run(t, new(MakeAgentEntityParserTestSuite))
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_NoOAuthConfig() {
	yamlData := []byte(`
id: no-oauth-agent
ouId: ou-1
type: service-agent
attributes:
  name: No OAuth Agent
`)
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "no-oauth-agent").
		Return("", "", inboundmodel.InboundClient{}, nil).Once()

	parser := makeAgentEntityParser(mockSvc)
	e, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), e)
	assert.Nil(s.T(), sysCredsJSON)

	var attrs map[string]interface{}
	assert.NoError(s.T(), json.Unmarshal(e.Attributes, &attrs))
	assert.Equal(s.T(), "No OAuth Agent", attrs["name"])
	assert.Empty(s.T(), e.SystemAttributes)
}

// A declared agent keeps its project, which a Data Plane stores without checking.
func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_KeepsProject() {
	yamlData := []byte(`
id: project-agent
ouId: ou-1
projectId: project-1
type: service-agent
`)
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything,
		mock.MatchedBy(func(a *providers.Agent) bool { return a.ProjectID == "project-1" }), "project-agent").
		Return("", "", inboundmodel.InboundClient{}, nil).Once()

	e, _, _, err := makeAgentEntityParser(mockSvc)(yamlData)

	s.Require().NoError(err)
	assert.Equal(s.T(), "project-1", e.ProjectID)
	assert.Equal(s.T(), "ou-1", e.OUID)
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_WithOAuthPublicClient() {
	yamlData := []byte(`
id: public-agent
ouId: ou-1
type: service-agent
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: public-client-id
      publicClient: true
      tokenEndpointAuthMethod: "none"
      grantTypes:
        - client_credentials
`)
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "public-agent").
		Return("public-client-id", "", inboundmodel.InboundClient{}, nil).Once()

	parser := makeAgentEntityParser(mockSvc)
	e, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), e)
	assert.Nil(s.T(), sysCredsJSON, "public clients should have no credentials")

	var sysAttrs map[string]interface{}
	assert.NoError(s.T(), json.Unmarshal(e.SystemAttributes, &sysAttrs))
	assert.Equal(s.T(), "public-client-id", sysAttrs[fieldClientID])
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_WithOAuthConfidentialClient() {
	yamlData := []byte(`
id: conf-agent
ouId: ou-1
type: service-agent
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: conf-client-id
      clientSecret: conf-client-secret
      publicClient: false
      grantTypes:
        - client_credentials
`)
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "conf-agent").
		Return("conf-client-id", "conf-client-secret", inboundmodel.InboundClient{}, nil).Once()

	parser := makeAgentEntityParser(mockSvc)
	e, _, sysCredsJSON, err := parser(yamlData)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), e)
	assert.NotNil(s.T(), sysCredsJSON, "confidential clients should have credentials")

	var sysAttrs map[string]interface{}
	assert.NoError(s.T(), json.Unmarshal(e.SystemAttributes, &sysAttrs))
	assert.Equal(s.T(), "conf-client-id", sysAttrs[fieldClientID])

	var sysCreds map[string]interface{}
	assert.NoError(s.T(), json.Unmarshal(sysCredsJSON, &sysCreds))
	assert.Equal(s.T(), "conf-client-secret", sysCreds[fieldClientSecret])
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_NilService() {
	parser := makeAgentEntityParser(nil)
	_, _, _, err := parser([]byte("id: x\nouId: ou-1\ntype: t\nname: n\n"))
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "agent service is required for declarative entity parsing")
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_InvalidYAML() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	parser := makeAgentEntityParser(mockSvc)
	_, _, _, err := parser([]byte("id: [unclosed bracket"))
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to parse agent YAML")
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_ValidateAgentError() {
	yamlData := []byte("id: err-agent\nouId: ou-1\ntype: t\nname: Err Agent\n")
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "err-agent").
		Return("", "", inboundmodel.InboundClient{}, &tidcommon.ServiceError{
			Code:  "AGT-XXXX",
			Error: tidcommon.I18nMessage{DefaultValue: "validation failed"},
		}).Once()

	parser := makeAgentEntityParser(mockSvc)
	_, _, _, err := parser(yamlData)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to validate agent 'err-agent'")
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_InvalidValidUntil() {
	yamlData := []byte("id: bad-lifespan\nouId: ou-1\ntype: t\nvalidUntil: not-a-date\n")
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "bad-lifespan").
		Return("", "", inboundmodel.InboundClient{}, nil).Once()

	parser := makeAgentEntityParser(mockSvc)
	_, _, _, err := parser(yamlData)

	assert.ErrorIs(s.T(), err, errInvalidDeclarativeLifespan)
	assert.Contains(s.T(), err.Error(), "failed to validate agent 'bad-lifespan'")
}

// The parser hands the personal kind to ValidateAgent, which applies the personal agent setup and its refusals.
func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_PersonalSetupRefusalSurfaces() {
	yamlData := []byte("id: pa-agent\nouId: ou-1\ntype: t\nkind: personal-assistant\nowner:\n  id: user-1\n" +
		"inboundAuthConfig:\n  - type: oauth2\n    config:\n      grantTypes:\n        - client_credentials\n")
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.MatchedBy(func(a *providers.Agent) bool {
		return a.Kind == providers.AgentKindPersonalAssistant
	}), "pa-agent").Return("", "", inboundmodel.InboundClient{}, &ErrorGrantTypeNotAllowedForPersonalAgent).Once()
	parser := makeAgentEntityParser(mockSvc)

	_, _, _, err := parser(yamlData)

	s.Require().Error(err)
	assert.Contains(s.T(), err.Error(), ErrorGrantTypeNotAllowedForPersonalAgent.Code)
}

// A personal agent with an owner and the personal grants loads, keeping its kind and owner.
func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_PersonalAgentStored() {
	yamlData := []byte("id: pa-agent\nouId: ou-1\ntype: t\nkind: personal-assistant\nowner:\n  id: user-1\n" +
		"inboundAuthConfig:\n  - type: oauth2\n    config:\n      grantTypes:\n        - authorization_code\n")
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "pa-agent").
		Return("", "", inboundmodel.InboundClient{}, nil).Once()
	parser := makeAgentEntityParser(mockSvc)

	e, _, _, err := parser(yamlData)

	s.Require().NoError(err)
	s.Require().NotNil(e)
	stored := readSystemAttributes(e.SystemAttributes)
	assert.Equal(s.T(), providers.AgentKindPersonalAssistant, stored.Kind)
	assert.Equal(s.T(), "user-1", stored.Owner.ID)
}

// A file without a kind loads as an ordinary agent.
func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_NoKindIsOrdinaryAgent() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "plain-agent").
		Return("", "", inboundmodel.InboundClient{}, nil).Once()
	parser := makeAgentEntityParser(mockSvc)

	e, _, _, err := parser([]byte("id: plain-agent\nouId: ou-1\ntype: t\n"))

	s.Require().NoError(err)
	s.Require().NotNil(e)
	assert.Empty(s.T(), readSystemAttributes(e.SystemAttributes).Kind)
}

// --- makeAgentInboundParser error paths ---

type MakeAgentSharingParserTestSuite struct {
	suite.Suite
}

func TestMakeAgentSharingParserTestSuite(t *testing.T) {
	suite.Run(t, new(MakeAgentSharingParserTestSuite))
}

// Most agent documents declare no policy, and those are not read against the loaded agent.
func (s *MakeAgentSharingParserTestSuite) TestADocumentWithoutPoliciesDeclaresNone() {
	parser := makeAgentSharingParser(agentmock.NewAgentServiceInterfaceMock(s.T()))

	declared, err := parser([]byte("id: agent-1\nouId: ou-1\ntype: service-agent\n"))

	s.Require().NoError(err)
	s.Equal(&sharing.DeclaredResourcePolicies{ResourceID: "agent-1"}, declared)
}

// The policies are declared against the unit the loaded agent lives in, which the document may name by
// handle.
func (s *MakeAgentSharingParserTestSuite) TestReadsThePoliciesAgainstTheAgentsUnit() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().GetAgent(asRuntime, "agent-1", true).
		Return(&model.AgentGetResponse{ID: "agent-1", OUID: "ou-1", Display: "Agent One"}, nil).Once()
	doc := `
id: agent-1
ouHandle: engineering
type: service-agent
sharingPolicies:
  - id: agent-1-policy-1
    otherTreeRoots:
      mode: selected
      units:
        - ouId: ou-2
`

	declared, err := makeAgentSharingParser(mockSvc)([]byte(doc))

	s.Require().NoError(err)
	s.Equal(&sharing.DeclaredResourcePolicies{
		ResourceID:   "agent-1",
		ResourceName: "Agent One",
		OwningOUID:   "ou-1",
		Policies: []sharing.PolicyRequest{{ID: "agent-1-policy-1", OtherTreeRoots: &sharing.OtherTreeRootsRequest{
			Mode: sharing.OtherTreeRootsSelected, Units: []sharing.RootUnitRequest{{OUID: "ou-2"}}}}},
	}, declared)
}

func (s *MakeAgentSharingParserTestSuite) TestNamesTheAgentByIDWithoutADisplayName() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().GetAgent(asRuntime, "agent-1", true).
		Return(&model.AgentGetResponse{ID: "agent-1", OUID: "ou-1"}, nil).Once()
	doc := "id: agent-1\nouId: ou-1\nsharingPolicies:\n  - id: p1\n    allUnits: {}\n"

	declared, err := makeAgentSharingParser(mockSvc)([]byte(doc))

	s.Require().NoError(err)
	s.Equal("agent-1", declared.ResourceName)
}

func (s *MakeAgentSharingParserTestSuite) TestRefusesPoliciesForAnAgentThatDidNotLoad() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().GetAgent(asRuntime, "agent-1", true).Return(nil, &ErrorAgentNotFound).Once()
	doc := "id: agent-1\nouId: ou-1\nsharingPolicies:\n  - id: p1\n    allUnits: {}\n"

	_, err := makeAgentSharingParser(mockSvc)([]byte(doc))

	s.ErrorContains(err, "failed to read agent 'agent-1' for its sharing policies")
}

func (s *MakeAgentSharingParserTestSuite) TestRefusesMalformedYAML() {
	_, err := makeAgentSharingParser(agentmock.NewAgentServiceInterfaceMock(s.T()))([]byte("id: [unclosed"))

	s.ErrorContains(err, "failed to parse agent YAML")
}

type MakeAgentInboundParserTestSuite struct {
	suite.Suite
}

func TestMakeAgentInboundParserTestSuite(t *testing.T) {
	suite.Run(t, new(MakeAgentInboundParserTestSuite))
}

func (s *MakeAgentInboundParserTestSuite) TestMakeAgentInboundParser_InvalidYAML() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	parser := makeAgentInboundParser(mockSvc)
	_, err := parser([]byte("id: [unclosed bracket"))
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to parse agent YAML")
}

func (s *MakeAgentInboundParserTestSuite) TestMakeAgentInboundParser_ValidateAgentError() {
	yamlData := []byte("id: inbound-err-agent\nouId: ou-1\ntype: t\nname: Inbound Err Agent\n")
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "inbound-err-agent").
		Return("", "", inboundmodel.InboundClient{}, &tidcommon.ServiceError{
			Code:  "AGT-YYYY",
			Error: tidcommon.I18nMessage{DefaultValue: "inbound validation failed"},
		}).Once()

	parser := makeAgentInboundParser(mockSvc)
	_, err := parser(yamlData)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to validate agent 'inbound-err-agent'")
}

// The inbound client carries the OAuth config as ValidateAgent completed it, so a personal agent declared without
// one still gets the OAuth client its setup adds.
func (s *MakeAgentInboundParserTestSuite) TestMakeAgentInboundParser_ReadsTheCompletedOAuthConfig() {
	yamlData := []byte("id: pa-agent\nouId: ou-1\ntype: t\nkind: personal-assistant\nowner:\n  id: user-1\n")
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "pa-agent").
		Run(func(_ context.Context, a *providers.Agent, _ string) {
			a.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
				},
			}}
		}).
		Return("", "", inboundmodel.InboundClient{}, nil).Once()

	client, err := makeAgentInboundParser(mockSvc)(yamlData)

	s.Require().NoError(err)
	s.Require().NotNil(client)
	s.Equal("pa-agent", client.ID)
	s.True(client.IsReadOnly)
	profile, ok := client.Properties[inboundclient.PropOAuthProfile].(providers.OAuthProfile)
	s.Require().True(ok, "the OAuth profile is set")
	s.Equal([]string{string(providers.GrantTypeAuthorizationCode)}, profile.GrantTypes)
}

// --- makeAgentInboundConfig validator ---

type MakeAgentInboundConfigTestSuite struct {
	suite.Suite
}

func TestMakeAgentInboundConfigTestSuite(t *testing.T) {
	suite.Run(t, new(MakeAgentInboundConfigTestSuite))
}

func (s *MakeAgentInboundConfigTestSuite) TestValidator_NilInboundClient() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	cfg := makeAgentInboundConfig(mockSvc)
	err := cfg.Validator(nil)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "parsed inbound client is nil")
}

func (s *MakeAgentInboundConfigTestSuite) TestValidator_ValidInboundClient() {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	cfg := makeAgentInboundConfig(mockSvc)
	err := cfg.Validator(&inboundmodel.InboundClient{ID: "client-1"})
	assert.NoError(s.T(), err)
}

// A declarative agent's credentials are held beside its client secret, as plaintext the loader hashes.
func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_WithCredentials() {
	yamlData := []byte(`
id: cred-agent
ouId: ou-1
type: service-agent
attributes:
  name: Credential Agent
credentials:
  password: s3cret
`)
	mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
	mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "cred-agent").
		Return("cred-client-id", "cred-client-secret", inboundmodel.InboundClient{}, nil).Once()

	parser := makeAgentEntityParser(mockSvc)
	e, _, sysCredsJSON, err := parser(yamlData)

	s.Require().NoError(err)
	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(e.Attributes, &attrs))
	assert.NotContains(s.T(), attrs, "password", "a credential is never kept as an attribute")
	var sysCreds map[string]string
	s.Require().NoError(json.Unmarshal(sysCredsJSON, &sysCreds))
	assert.Equal(s.T(), map[string]string{fieldClientSecret: "cred-client-secret", "password": "s3cret"}, sysCreds)
}

func (s *MakeAgentEntityParserTestSuite) TestMakeAgentEntityParser_RefusesAMalformedCredential() {
	for name, credentials := range map[string]string{
		"not a string":      "  password:\n    - one\n",
		"the client secret": "  " + fieldClientSecret + ": s3cret\n",
		"the flow secret":   "  flowSecret: s3cret\n",
		"a passkey":         "  passkey: s3cret\n",
	} {
		s.Run(name, func() {
			mockSvc := agentmock.NewAgentServiceInterfaceMock(s.T())
			mockSvc.EXPECT().ValidateAgent(mock.Anything, mock.Anything, "bad-cred-agent").
				Return("", "", inboundmodel.InboundClient{}, nil).Once()

			_, _, _, err := makeAgentEntityParser(mockSvc)([]byte(
				"id: bad-cred-agent\nouId: ou-1\ntype: service-agent\ncredentials:\n" + credentials))

			assert.ErrorContains(s.T(), err, "credentials of agent 'bad-cred-agent'")
		})
	}
}

func (s *MakeAgentEntityParserTestSuite) TestWithDeclarativeCredentials_KeepsWhatHasNoCredentials() {
	sysCreds := json.RawMessage(`{"clientSecret":"x"}`)

	merged, err := withDeclarativeCredentials(sysCreds, nil)

	s.NoError(err)
	s.Equal(sysCreds, merged)

	_, err = withDeclarativeCredentials(json.RawMessage(`[1]`), map[string]interface{}{"password": "s3cret"})
	s.Error(err)
}
