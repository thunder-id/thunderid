// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package nextjssdk

import (
	"context"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type NextJSSDKToolsTestSuite struct {
	suite.Suite
}

func TestNextJSSDKToolsTestSuite(t *testing.T) {
	suite.Run(t, new(NextJSSDKToolsTestSuite))
}

func (suite *NextJSSDKToolsTestSuite) SetupTest() {

}

func (suite *NextJSSDKToolsTestSuite) TestIntegrateNextJSSDK() {
	input := integrateNextJSSDKInput{
		ServerURL: "https://thunder.example.com",
	}

	result, output, err := integrateNextJSSDK(context.Background(), nil, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), output.Instructions, "https://thunder.example.com")
	assert.NotContains(suite.T(), output.Instructions, "https://localhost:8090")
	assert.NotEmpty(suite.T(), output.CodeSnippets)
}

func (suite *NextJSSDKToolsTestSuite) TestIntegrateNextJSSDK_Defaults() {
	input := integrateNextJSSDKInput{}

	result, output, err := integrateNextJSSDK(context.Background(), nil, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), output.Instructions, "https://localhost:8090")
	assert.NotEmpty(suite.T(), output.CodeSnippets)
}

func (suite *NextJSSDKToolsTestSuite) TestRegisterTools() {
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

	found := false
	iter := featuresField.MapRange()
	for iter.Next() {
		if iter.Key().String() == "integrate_nextjs_sdk" {
			found = true
			break
		}
	}
	assert.True(suite.T(), found, "integrate_nextjs_sdk tool should be registered")
}
