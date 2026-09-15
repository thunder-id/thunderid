// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"context"
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
		Name:  "test-agent",
		Owner: testAgentOwner,
	})

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorAgentProvisioningDisabled.Code, svcErr.Code)
}
