// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

const (
	testAgentID    = "agent-id-123"
	testAgentOwner = "owner-id-456"
)

type DefaultAgentMgtProviderTestSuite struct {
	suite.Suite
	mockService *agentmock.AgentServiceInterfaceMock
	provider    providers.AgentMgtProvider
}

func (suite *DefaultAgentMgtProviderTestSuite) SetupTest() {
	suite.mockService = agentmock.NewAgentServiceInterfaceMock(suite.T())
	suite.provider = newDefaultAgentMgtProvider(suite.mockService)
}

func TestDefaultAgentMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultAgentMgtProviderTestSuite))
}

func newTestAgent() *providers.Agent {
	return &providers.Agent{
		OUID:  "ou-id-abc",
		Type:  "employee",
		Name:  "test-agent",
		Owner: testAgentOwner,
	}
}

// A valid request reaches the agent service unmodified and its response is returned to the caller,
// including the generated client credentials.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentDelegatesAndReturnsCredentials() {
	requested := newTestAgent()
	created := &model.AgentCompleteResponse{
		ID:    testAgentID,
		Owner: testAgentOwner,
		Name:  "test-agent",
	}

	suite.mockService.On("CreateAgent", mock.Anything, requested).Return(created, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), requested)

	suite.Nil(svcErr)
	suite.Equal(testAgentID, resp.ID)
	suite.Equal(testAgentOwner, resp.Owner)
}

// The runtime has no dependable caller identity, so an agent with no owner must be rejected by the
// provider. Reaching the agent service would silently persist an ownerless agent.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentWithoutOwnerIsRejectedBeforeDelegation() {
	a := newTestAgent()
	a.Owner = ""

	resp, svcErr := suite.provider.CreateAgent(context.Background(), a)

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorOwnerRequired.Code, svcErr.Code)
	suite.mockService.AssertNotCalled(suite.T(), "CreateAgent", mock.Anything, mock.Anything)
}

func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentWithNilAgentIsRejectedBeforeDelegation() {
	resp, svcErr := suite.provider.CreateAgent(context.Background(), nil)

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
	suite.mockService.AssertNotCalled(suite.T(), "CreateAgent", mock.Anything, mock.Anything)
}

// Failures raised by the agent service must reach the caller with their original code. A runtime
// caller distinguishes a name clash it can retry with different input from a configuration problem
// it cannot, so collapsing these into one generic error would remove the only signal it has.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentPreservesServiceErrors() {
	tests := []struct {
		name     string
		scenario string
		svcErr   *tidcommon.ServiceError
	}{
		{
			name:     "DuplicateName",
			scenario: "another agent already uses the requested name",
			svcErr:   &agent.ErrorAgentAlreadyExistsWithName,
		},
		{
			name:     "OwnerNotFound",
			scenario: "the supplied owner does not resolve to a known entity",
			svcErr:   &agent.ErrorOwnerNotFound,
		},
		{
			name:     "OrganizationUnitNotFound",
			scenario: "the target organization unit does not exist",
			svcErr:   &agent.ErrorOrganizationUnitNotFound,
		},
		{
			name:     "ClientIDTaken",
			scenario: "the requested client ID is already in use",
			svcErr:   &agent.ErrorAgentAlreadyExistsWithClientID,
		},
		{
			name:     "SchemaValidationFailed",
			scenario: "the attributes do not satisfy the agent type schema",
			svcErr:   &agent.ErrorSchemaValidationFailed,
		},
		{
			name:     "InvalidAgentType",
			scenario: "the agent type is missing",
			svcErr:   &agent.ErrorInvalidAgentType,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			mockService := agentmock.NewAgentServiceInterfaceMock(suite.T())
			provider := newDefaultAgentMgtProvider(mockService)
			mockService.On("CreateAgent", mock.Anything, mock.Anything).
				Return(nil, tt.svcErr).Once()

			resp, svcErr := provider.CreateAgent(context.Background(), newTestAgent())

			suite.Nil(resp)
			suite.NotNil(svcErr, tt.scenario)
			suite.Equal(tt.svcErr.Code, svcErr.Code, tt.scenario)
			suite.Equal(tidcommon.ClientErrorType, svcErr.Type, tt.scenario)
		})
	}
}

// A system failure must stay a server error. Reporting an outage as a client error would tell the
// caller to change its input when retrying is the correct response.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentPreservesServerErrors() {
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent())

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// The agent service must see a runtime-marked context that still carries the caller's values, so
// authorization is elevated without losing request-scoped data such as the trace ID.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentElevatesCallerContext() {
	type ctxKey string
	const traceKey ctxKey = "trace-id"

	callerCtx := context.WithValue(context.Background(), traceKey, "trace-789")

	var observed context.Context
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			observed = args.Get(0).(context.Context)
		}).
		Return(&model.AgentCompleteResponse{ID: testAgentID}, nil).Once()

	_, svcErr := suite.provider.CreateAgent(callerCtx, newTestAgent())

	suite.Nil(svcErr)
	suite.True(security.IsRuntimeContext(observed))
	suite.Equal("trace-789", observed.Value(traceKey))
}

// The generated client secret is the reason a runtime capability provisions an agent at all. It now
// crosses toProviderAgent, so this guards against the mapper dropping it.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentReturnsGeneratedClientCredentials() {
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(&model.AgentCompleteResponse{
			ID: testAgentID,
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "generated-client-id",
					ClientSecret: "generated-client-secret",
				},
			}},
		}, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent())

	suite.Nil(svcErr)
	suite.Require().Len(resp.InboundAuthConfig, 1)
	suite.Require().NotNil(resp.InboundAuthConfig[0].OAuthConfig)
	suite.Equal("generated-client-id", resp.InboundAuthConfig[0].OAuthConfig.ClientID)
	suite.Equal("generated-client-secret", resp.InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

// A service that returns neither a response nor an error must not take the runtime down with a nil
// dereference inside the mapper.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentHandlesEmptyServiceResponse() {
	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent())

	suite.Nil(svcErr)
	suite.Nil(resp)
}

// toProviderAgent is a hand-maintained field copy, so a field the agent service starts returning
// later can be silently zeroed for every caller. This populates every field the response can carry
// and asserts each one survives the mapping.
func (suite *DefaultAgentMgtProviderTestSuite) TestCreateAgentMapsEveryResponseField() {
	assertion := &providers.AssertionConfig{}
	loginConsent := &providers.LoginConsentConfig{ValidityPeriod: 3600}
	attestation := &providers.AttestationConfig{}

	full := &model.AgentCompleteResponse{
		ID:          testAgentID,
		OUID:        "ou-id-abc",
		OUHandle:    "engineering",
		Type:        "employee",
		Name:        "test-agent",
		Description: "an agent",
		LogoURL:     "avatar:shape=circle",
		Owner:       testAgentOwner,
		Attributes:  []byte(`{"k":"v"}`),
	}
	full.AuthFlowID = "auth-flow"
	full.AuthFlowHandle = "auth-handle"
	full.RegistrationFlowID = "reg-flow"
	full.RegistrationFlowHandle = "reg-handle"
	full.IsRegistrationFlowEnabled = true
	full.RecoveryFlowID = "rec-flow"
	full.RecoveryFlowHandle = "rec-handle"
	full.IsRecoveryFlowEnabled = true
	full.SignOutFlowID = "out-flow"
	full.SignOutFlowHandle = "out-handle"
	full.ThemeID = "theme-1"
	full.LayoutID = "layout-1"
	full.Assertion = assertion
	full.LoginConsent = loginConsent
	full.AllowedUserTypes = []string{"customer"}
	full.SubjectAttribute = map[string]string{"customer": "email"}
	full.PasskeyAllowedOrigins = []string{"https://example.com"}
	full.Attestation = attestation

	suite.mockService.On("CreateAgent", mock.Anything, mock.Anything).Return(full, nil).Once()

	resp, svcErr := suite.provider.CreateAgent(context.Background(), newTestAgent())

	suite.Nil(svcErr)
	suite.Equal(full.ID, resp.ID)
	suite.Equal(full.OUID, resp.OUID)
	suite.Equal(full.OUHandle, resp.OUHandle)
	suite.Equal(full.Type, resp.Type)
	suite.Equal(full.Name, resp.Name)
	suite.Equal(full.Description, resp.Description)
	suite.Equal(full.LogoURL, resp.LogoURL)
	suite.Equal(full.Owner, resp.Owner)
	suite.Equal(full.Attributes, resp.Attributes)
	suite.Equal(full.AuthFlowID, resp.AuthFlowID)
	suite.Equal(full.AuthFlowHandle, resp.AuthFlowHandle)
	suite.Equal(full.RegistrationFlowID, resp.RegistrationFlowID)
	suite.Equal(full.RegistrationFlowHandle, resp.RegistrationFlowHandle)
	suite.Equal(full.IsRegistrationFlowEnabled, resp.IsRegistrationFlowEnabled)
	suite.Equal(full.RecoveryFlowID, resp.RecoveryFlowID)
	suite.Equal(full.RecoveryFlowHandle, resp.RecoveryFlowHandle)
	suite.Equal(full.IsRecoveryFlowEnabled, resp.IsRecoveryFlowEnabled)
	suite.Equal(full.SignOutFlowID, resp.SignOutFlowID)
	suite.Equal(full.SignOutFlowHandle, resp.SignOutFlowHandle)
	suite.Equal(full.ThemeID, resp.ThemeID)
	suite.Equal(full.LayoutID, resp.LayoutID)
	suite.Equal(assertion, resp.Assertion)
	suite.Equal(loginConsent, resp.LoginConsent)
	suite.Equal(full.AllowedUserTypes, resp.AllowedUserTypes)
	suite.Equal(full.SubjectAttribute, resp.SubjectAttribute)
	suite.Equal(full.PasskeyAllowedOrigins, resp.PasskeyAllowedOrigins)
	suite.Equal(attestation, resp.Attestation)
}
