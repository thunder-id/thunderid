// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
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
owner: owner-id-123
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
	assert.Equal(s.T(), "owner-id-123", req.Owner)
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

// --- makeAgentInboundParser error paths ---

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
