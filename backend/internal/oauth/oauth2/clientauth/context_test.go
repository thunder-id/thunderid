/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package clientauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
)

type ContextTestSuite struct {
	suite.Suite
}

func TestContextTestSuite(t *testing.T) {
	suite.Run(t, new(ContextTestSuite))
}

func (suite *ContextTestSuite) TestGetOAuthClient_WithNilContext() {
	client := GetOAuthClient(context.Background())
	assert.Nil(suite.T(), client)
}

func (suite *ContextTestSuite) TestGetOAuthClient_WithEmptyContext() {
	ctx := context.Background()
	client := GetOAuthClient(ctx)
	assert.Nil(suite.T(), client)
}

func (suite *ContextTestSuite) TestGetOAuthClient_WithExistingClient() {
	expectedClient := &OAuthClientInfo{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		OAuthApp: &inboundmodel.OAuthClient{
			ClientID: "test-client-id",
		},
	}

	ctx := withOAuthClient(context.Background(), expectedClient)
	client := GetOAuthClient(ctx)

	assert.NotNil(suite.T(), client)
	assert.Equal(suite.T(), expectedClient.ClientID, client.ClientID)
	assert.Equal(suite.T(), expectedClient.ClientSecret, client.ClientSecret)
	assert.NotNil(suite.T(), client.OAuthApp)
}

func (suite *ContextTestSuite) TestWithOAuthClient() {
	expectedClient := &OAuthClientInfo{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		OAuthApp: &inboundmodel.OAuthClient{
			ClientID:                "test-client-id",
			TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		},
	}

	ctx := withOAuthClient(context.Background(), expectedClient)
	client := GetOAuthClient(ctx)

	assert.NotNil(suite.T(), client)
	assert.Equal(suite.T(), expectedClient.ClientID, client.ClientID)
	assert.Equal(suite.T(), expectedClient.OAuthApp.ClientID, client.OAuthApp.ClientID)
}

func (suite *ContextTestSuite) TestWithOAuthClient_NilContext() {
	expectedClient := &OAuthClientInfo{
		ClientID: "test-client-id",
	}

	ctx := withOAuthClient(context.Background(), expectedClient)
	client := GetOAuthClient(ctx)

	assert.NotNil(suite.T(), client)
	assert.Equal(suite.T(), expectedClient.ClientID, client.ClientID)
}

func (suite *ContextTestSuite) TestGetOAuthClient_WithWrongType() {
	ctx := context.WithValue(context.Background(), OAuthClientKey, "wrong-type")
	client := GetOAuthClient(ctx)
	assert.Nil(suite.T(), client)
}

func (suite *ContextTestSuite) TestGetOAuthClient_WithNilValue() {
	ctx := context.WithValue(context.Background(), OAuthClientKey, nil)
	client := GetOAuthClient(ctx)
	assert.Nil(suite.T(), client)
}

func (suite *ContextTestSuite) TestWithOAuthClient_NilClient() {
	ctx := withOAuthClient(context.Background(), nil)
	client := GetOAuthClient(ctx)
	assert.Nil(suite.T(), client)
}

func (suite *ContextTestSuite) TestWithOAuthClient_ContextChaining() {
	client1 := &OAuthClientInfo{
		ClientID: "client-1",
	}
	client2 := &OAuthClientInfo{
		ClientID: "client-2",
	}

	ctx1 := withOAuthClient(context.Background(), client1)
	ctx2 := withOAuthClient(ctx1, client2)

	client := GetOAuthClient(ctx2)
	assert.NotNil(suite.T(), client)
	assert.Equal(suite.T(), "client-2", client.ClientID)
}

func (suite *ContextTestSuite) TestGetOAuthClient_WithEmptyClientInfo() {
	clientInfo := &OAuthClientInfo{
		ClientID:     "",
		ClientSecret: "",
		OAuthApp:     nil,
	}

	ctx := withOAuthClient(context.Background(), clientInfo)
	client := GetOAuthClient(ctx)

	assert.NotNil(suite.T(), client)
	assert.Equal(suite.T(), "", client.ClientID)
}
