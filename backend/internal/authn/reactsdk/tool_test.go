// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package reactsdk

import (
	"context"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ReactSDKToolsTestSuite struct {
	suite.Suite
}

func TestReactSDKToolsTestSuite(t *testing.T) {
	suite.Run(t, new(ReactSDKToolsTestSuite))
}

func (suite *ReactSDKToolsTestSuite) SetupTest() {

}

func (suite *ReactSDKToolsTestSuite) TestIntegrateReactSDK() {
	input := integrateReactSDKInput{
		ServerURL: "https://thunder.example.com",
	}

	result, output, err := integrateReactSDK(context.Background(), nil, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), output.Instructions, "https://thunder.example.com")
	assert.Contains(suite.T(), output.CodeSnippets, "https://thunder.example.com")
}

func (suite *ReactSDKToolsTestSuite) TestIntegrateReactSDK_Defaults() {
	// Empty input to trigger defaults
	input := integrateReactSDKInput{}

	result, output, err := integrateReactSDK(context.Background(), nil, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	// Should contain the default URL
	assert.Contains(suite.T(), output.Instructions, "https://localhost:8090")
	assert.Contains(suite.T(), output.CodeSnippets, "https://localhost:8090")
}

func (suite *ReactSDKToolsTestSuite) TestIntegrateReactSDK_NamesToolsWithoutPrefix() {
	_, output, err := integrateReactSDK(context.Background(), nil, integrateReactSDKInput{})

	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), output.Instructions, "thunderid_")
	assert.Contains(suite.T(), output.Instructions, "`update_application`")
}

func (suite *ReactSDKToolsTestSuite) TestRegisterTools() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	RegisterTools(server)

	toolsField := reflect.ValueOf(server).Elem().FieldByName("tools")
	if !toolsField.IsValid() {
		suite.T().Fatal("tools field not found in mcp.Server")
	}

	featuresField := toolsField.Elem().FieldByName("features")
	if !featuresField.IsValid() {
		suite.T().Fatal("features field not found in featureSet")
	}

	// Check if our tool is in the map
	found := false
	iter := featuresField.MapRange()
	for iter.Next() {
		if iter.Key().String() == "integrate_react_sdk" {
			found = true
			break
		}
	}
	assert.True(suite.T(), found, "integrate_react_sdk tool should be registered")
}
