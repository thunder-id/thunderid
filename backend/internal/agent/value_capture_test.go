// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
	clearMockCalls(mockEntity, "CreateEntity")
	mockEntity.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(buildAgentEntityFixture(testAgentName, "", "", "cid-xxx"), nil)
	clearMockCalls(mockInbound, "CreateInboundClient")
	mockInbound.On("CreateInboundClient",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	resp, svcErr := svc.CreateAgent(context.Background(), &providers.Agent{
		Name:               testAgentName,
		Type:               testAgentType,
		OUID:               testOUID,
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
	assert.Equal(suite.T(), testAgentName, agent.Name)
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
	clearMockCalls(mockEntity, "GetEntity")
	mockEntity.On("GetEntity", mock.Anything, testAgentID).
		Return(buildAgentEntityFixture("old-name", "", "", ""), nil)

	_, svcErr := svc.UpdateAgent(context.Background(), testAgentID, &model.UpdateAgentRequest{
		Name: testAgentName, Type: testAgentType,
	})
	suite.Require().Nil(svcErr)

	agent := capturer.captured(suite)
	assert.Equal(suite.T(), testAgentID, agent.ID)
	assert.Equal(suite.T(), testAgentName, agent.Name)
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
		svc.captureValues(context.Background(), &model.AgentCompleteResponse{Name: testAgentName})
	})
}
