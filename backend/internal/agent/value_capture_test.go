// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

// recordingCapturer records every resource handed to it.
type recordingCapturer struct {
	resourceTypes []string
	resources     []interface{}
}

func (r *recordingCapturer) CaptureValues(_ context.Context, resourceType string, resource interface{}) {
	r.resourceTypes = append(r.resourceTypes, resourceType)
	r.resources = append(r.resources, resource)
}

// captured returns the one agent handed to the capturer, in the shape its exporter reads back.
func (r *recordingCapturer) captured(suite *AgentServiceTestSuite) *model.AgentGetResponse {
	suite.Require().Len(r.resources, 1)
	assert.Equal(suite.T(), resourceTypeAgent, r.resourceTypes[0])
	agent, ok := r.resources[0].(*model.AgentGetResponse)
	suite.Require().True(ok, "the capture was not handed the shape the exporter reads back")
	return agent
}

// A created agent is captured with the client secret generated for it, which only the create still
// holds.
func (suite *AgentServiceTestSuite) TestCreateAgent_CapturesTheGeneratedSecret() {
	svc, mockEntity, mockInbound, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "name")
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(buildAgentEntityFixture(testAgentName, "", "", "cid-xxx"), nil)
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		Type:               testAgentType,
		OUID:               testOUID,
		Attributes:         json.RawMessage(`{"name":"test-agent"}`),
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	})
	suite.Require().Nil(svcErr)

	agent := capturer.captured(suite)
	assert.Equal(suite.T(), resp.ID, agent.ID)
	assert.Equal(suite.T(), testAgentName, agent.Display)
	suite.Require().Len(agent.InboundAuthConfig, 1)
	assert.Equal(suite.T(), resp.InboundAuthConfig[0].OAuthConfig.ClientID,
		agent.InboundAuthConfig[0].OAuthConfig.ClientID)
	assert.NotEmpty(suite.T(), agent.InboundAuthConfig[0].OAuthConfig.ClientSecret)
	assert.Equal(suite.T(), resp.InboundAuthConfig[0].OAuthConfig.ClientSecret,
		agent.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// An updated agent is captured as the update returns it.
func (suite *AgentServiceTestSuite) TestUpdateAgent_CapturesTheUpdatedAgent() {
	svc, mockEntity, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "name")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture("old-name", "", "", ""), nil)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, Attributes: json.RawMessage(`{"name":"test-agent"}`),
	})
	suite.Require().Nil(svcErr)

	agent := capturer.captured(suite)
	assert.Equal(suite.T(), testAgentID, agent.ID)
	assert.Equal(suite.T(), testAgentName, agent.Display)
}

// An agent whose type has no display attribute is captured under its ID, the name its export uses.
func (suite *AgentServiceTestSuite) TestCapture_FallsBackToTheIDWithoutADisplayAttribute() {
	svc, _, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "")

	svc.captureValues(context.Background(), &model.AgentCompleteResponse{
		ID: testAgentID, Type: testAgentType, Attributes: json.RawMessage(`{"name":"test-agent"}`),
	}, nil)

	assert.Equal(suite.T(), testAgentID, capturer.captured(suite).Display)
}

// A create that fails captures nothing.
func (suite *AgentServiceTestSuite) TestCreateAgent_FailureCapturesNothing() {
	svc, _, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer

	_, svcErr := svc.CreateAgent(context.Background(), nil)

	suite.Require().NotNil(svcErr)
	assert.Empty(suite.T(), capturer.resources)
}

// Without a capturer, which is every plane but a control plane, nothing is captured.
func (suite *AgentServiceTestSuite) TestCaptureWithoutACapturerDoesNothing() {
	svc, _, _, _, _ := suite.setupService()

	assert.NotPanics(suite.T(), func() {
		svc.captureValues(context.Background(), &model.AgentCompleteResponse{ID: testAgentID}, nil)
	})
}

// declaresPassword makes the entity service read a password, the one credential the agent type
// declares, from the attributes an agent is written with.
func declaresPassword(mockEntity *entitymock.EntityServiceInterfaceMock) {
	clearMockCalls(mockEntity, "GetDeclaredCredentialValues")
	mockEntity.On("GetDeclaredCredentialValues", mock.Anything, providers.EntityCategoryAgent, testAgentType,
		mock.Anything).Return(func(_ context.Context, _ providers.EntityCategory, _ string,
		attributes json.RawMessage) (map[string]string, error) {
		var attrs map[string]interface{}
		if err := json.Unmarshal(attributes, &attrs); err != nil {
			return nil, err
		}
		values := map[string]string{}
		if password, ok := attrs["password"].(string); ok {
			values["password"] = password
		}
		return values, nil
	})
}

// A created agent is captured with the password it was created with, which creation strips from what
// it returns.
func (suite *AgentServiceTestSuite) TestCreateAgent_CapturesTheSubmittedCredentials() {
	svc, mockEntity, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "name")
	declaresPassword(mockEntity)
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	_, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		Type: testAgentType, OUID: testOUID,
		Attributes: json.RawMessage(`{"name":"test-agent","password":"s3cret"}`),
	})
	suite.Require().Nil(svcErr)

	agent := capturer.captured(suite)
	assert.Equal(suite.T(), testAgentName, agent.Display)
	assert.Equal(suite.T(), map[string]interface{}{"password": "s3cret"}, agent.Credentials)
}

// An update that sets a password captures the new one.
func (suite *AgentServiceTestSuite) TestUpdateAgent_CapturesTheSubmittedCredentials() {
	svc, mockEntity, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "name")
	declaresPassword(mockEntity)
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Type: testAgentType, Attributes: json.RawMessage(`{"name":"test-agent","password":"rotated"}`),
	})
	suite.Require().Nil(svcErr)

	assert.Equal(suite.T(), map[string]interface{}{"password": "rotated"}, capturer.captured(suite).Credentials)
}

// A failure to read which attributes are credentials still captures the rest of the agent, its client
// secret included.
func (suite *AgentServiceTestSuite) TestCapture_CredentialLookupFailureStillCapturesTheAgent() {
	svc, mockEntity, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "name")
	clearMockCalls(mockEntity, "GetDeclaredCredentialValues")
	mockEntity.On("GetDeclaredCredentialValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	svc.captureValues(context.Background(), &model.AgentCompleteResponse{
		ID: testAgentID, Type: testAgentType, Attributes: json.RawMessage(`{"name":"test-agent"}`),
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "cid", ClientSecret: "csecret"},
		}},
	}, json.RawMessage(`{"name":"test-agent","password":"s3cret"}`))

	agent := capturer.captured(suite)
	assert.Nil(suite.T(), agent.Credentials)
	suite.Require().Len(agent.InboundAuthConfig, 1)
	assert.Equal(suite.T(), "csecret", agent.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// Setting credentials alone captures them under the agent's display value, with no client to report.
func (suite *AgentServiceTestSuite) TestUpdateAgentCredentials_CapturesTheNewCredentials() {
	svc, mockEntity, _, _, _ := suite.setupService()
	capturer := &recordingCapturer{}
	svc.valueCapturer = capturer
	suite.withDisplayPaths(svc, "name")
	clearMockCalls(mockEntity, "GetEntityProfile")
	mockEntity.On("GetEntityProfile", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture(testAgentName, "", "", ""), nil)
	mockEntity.On("UpdateCredentials", mock.Anything, testAgentID, json.RawMessage(`{"password":"rotated"}`)).
		Return(nil)

	svcErr := svc.UpdateAgentCredentials(context.Background(), testAgentID,
		json.RawMessage(`{"password":"rotated"}`))
	suite.Require().Nil(svcErr)

	agent := capturer.captured(suite)
	assert.Equal(suite.T(), testAgentID, agent.ID)
	assert.Equal(suite.T(), testAgentName, agent.Display)
	assert.Equal(suite.T(), map[string]interface{}{"password": "rotated"}, agent.Credentials)
	assert.Empty(suite.T(), agent.InboundAuthConfig)
}
