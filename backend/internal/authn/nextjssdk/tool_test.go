// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package nextjssdk

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type NextJSSDKToolsTestSuite struct {
	suite.Suite
	tools *nextJSSDKTools
}

func TestNextJSSDKToolsTestSuite(t *testing.T) {
	suite.Run(t, new(NextJSSDKToolsTestSuite))
}

func (suite *NextJSSDKToolsTestSuite) SetupTest() {
	suite.tools = &nextJSSDKTools{productName: "Acme IAM"}
}

func (suite *NextJSSDKToolsTestSuite) TestIntegrateNextJSSDK() {
	input := integrateNextJSSDKInput{
		ServerURL: "https://thunder.example.com",
	}

	result, output, err := suite.tools.integrateNextJSSDK(context.Background(), nil, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), output.Instructions, "https://thunder.example.com")
	assert.NotContains(suite.T(), output.Instructions, "https://localhost:8090")
	assert.NotEmpty(suite.T(), output.CodeSnippets)
}

func (suite *NextJSSDKToolsTestSuite) TestIntegrateNextJSSDK_Defaults() {
	input := integrateNextJSSDKInput{}

	result, output, err := suite.tools.integrateNextJSSDK(context.Background(), nil, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), output.Instructions, "https://localhost:8090")
	assert.NotEmpty(suite.T(), output.CodeSnippets)
}

func (suite *NextJSSDKToolsTestSuite) TestIntegrateNextJSSDK_NamesConfiguredProduct() {
	_, output, err := suite.tools.integrateNextJSSDK(context.Background(), nil, integrateNextJSSDKInput{})

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), output.Instructions, "# Acme IAM Authentication – Next.js Integration Instructions")
	assert.NotContains(suite.T(), output.Instructions, productNamePlaceholder)
}

func (suite *NextJSSDKToolsTestSuite) TestRegisterTools() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	RegisterTools(server, "Acme IAM")

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	suite.Require().NoError(err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil).
		Connect(ctx, clientTransport, nil)
	suite.Require().NoError(err)
	defer func() { _ = session.Close() }()
	tools, err := session.ListTools(ctx, nil)
	suite.Require().NoError(err)

	suite.Require().Len(tools.Tools, 1)
	suite.Equal("integrate_nextjs_sdk", tools.Tools[0].Name)
	suite.True(tools.Tools[0].Annotations.ReadOnlyHint, "the tool only returns instructions")
}
