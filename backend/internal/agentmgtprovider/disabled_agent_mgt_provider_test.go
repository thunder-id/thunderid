// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DisabledAgentMgtProviderTestSuite struct {
	suite.Suite
	provider providers.AgentMgtProvider
}

func (suite *DisabledAgentMgtProviderTestSuite) SetupTest() {
	suite.provider = NewDisabledAgentMgtProvider()
}

func TestDisabledAgentMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DisabledAgentMgtProviderTestSuite))
}

// A deployment that disables the provider must not provision agents from the runtime, and the
// caller must be told why rather than receiving a generic failure.
func (suite *DisabledAgentMgtProviderTestSuite) TestCreateAgentIsRejected() {
	resp, svcErr := suite.provider.CreateAgent(context.Background(), &providers.Agent{
		Owner: testAgentOwner(),
	})

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorAgentProvisioningDisabled.Code, svcErr.Code)
}

// A disabled provider changes no agent's credentials from the runtime, so a reset for an agent is refused.
func (suite *DisabledAgentMgtProviderTestSuite) TestCredentialResetIsRejected() {
	svcErr := suite.provider.UpdateAgentCredentials(context.Background(), testAgentID, json.RawMessage(`{}`))
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorAgentManagementDisabled.Code, svcErr.Code)

	address, svcErr := suite.provider.ValidateRemoveAgentCredential(context.Background(), testAgentID, "password")
	suite.Empty(address)
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorAgentManagementDisabled.Code, svcErr.Code)

	marker, svcErr := suite.provider.RemoveAgentCredential(context.Background(), testAgentID, "password")
	suite.Empty(marker)
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorAgentManagementDisabled.Code, svcErr.Code)

	current, svcErr := suite.provider.IsAgentCredentialUnchangedSince(context.Background(), testAgentID, "m")
	suite.False(current)
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorAgentManagementDisabled.Code, svcErr.Code)
}
