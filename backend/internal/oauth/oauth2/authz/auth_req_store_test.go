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

package authz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AuthorizationRequestStoreTestSuite exercises the authorizationRequestStore adapter against a
// real in-memory runtime store, verifying the add/get/clear round-trip semantics.
type AuthorizationRequestStoreTestSuite struct {
	suite.Suite
	store                  *authorizationRequestStore
	testAuthRequestContext authRequestContext
}

func TestAuthorizationRequestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationRequestStoreTestSuite))
}

func (suite *AuthorizationRequestStoreTestSuite) SetupTest() {
	suite.store = &authorizationRequestStore{
		storeProvider:  inmemory.Initialize("test-deployment"),
		validityPeriod: 10 * time.Minute,
	}

	suite.testAuthRequestContext = authRequestContext{
		OAuthParameters: model.OAuthParameters{
			State:               "test-state",
			ClientID:            "test-client-id",
			RedirectURI:         "https://client.example.com/callback",
			RedirectURIProvided: true,
			ResponseType:        "code",
			StandardScopes:      []string{"openid", "profile"},
			PermissionScopes:    []string{"read", "write"},
			CodeChallenge:       "test-challenge",
			CodeChallengeMethod: "S256",
			Resources:           []string{"https://api.example.com/resource"},
		},
	}
}

func (suite *AuthorizationRequestStoreTestSuite) TestNewAuthorizationRequestStore() {
	store := newAuthorizationRequestStore(inmemory.Initialize("test-deployment"))
	assert.NotNil(suite.T(), store)
	assert.Implements(suite.T(), (*authorizationRequestStoreInterface)(nil), store)
}

// Tests for AddRequest

func (suite *AuthorizationRequestStoreTestSuite) TestAddRequest_Success() {
	identifier, err := suite.store.AddRequest(context.Background(), suite.testAuthRequestContext)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), identifier)
}

func (suite *AuthorizationRequestStoreTestSuite) TestAddRequest_GeneratesUniqueIdentifiers() {
	id1, err1 := suite.store.AddRequest(context.Background(), suite.testAuthRequestContext)
	id2, err2 := suite.store.AddRequest(context.Background(), suite.testAuthRequestContext)

	assert.NoError(suite.T(), err1)
	assert.NoError(suite.T(), err2)
	assert.NotEqual(suite.T(), id1, id2)
}

func (suite *AuthorizationRequestStoreTestSuite) TestAddRequest_PutError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("put failed"))
	store := &authorizationRequestStore{
		storeProvider:  rt,
		validityPeriod: 10 * time.Minute,
	}

	identifier, err := store.AddRequest(context.Background(), suite.testAuthRequestContext)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to insert authorization request")
	assert.Empty(suite.T(), identifier)
}

// Tests for GetRequest

func (suite *AuthorizationRequestStoreTestSuite) TestGetRequest_Success() {
	identifier, err := suite.store.AddRequest(context.Background(), suite.testAuthRequestContext)
	suite.Require().NoError(err)

	ok, result, err := suite.store.GetRequest(context.Background(), identifier)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "test-state", result.OAuthParameters.State)
	assert.Equal(suite.T(), "test-client-id", result.OAuthParameters.ClientID)
	assert.Equal(suite.T(), "https://client.example.com/callback", result.OAuthParameters.RedirectURI)
	assert.True(suite.T(), result.OAuthParameters.RedirectURIProvided)
	assert.Equal(suite.T(), "code", result.OAuthParameters.ResponseType)
	assert.Equal(suite.T(), []string{"openid", "profile"}, result.OAuthParameters.StandardScopes)
	assert.Equal(suite.T(), []string{"read", "write"}, result.OAuthParameters.PermissionScopes)
	assert.Equal(suite.T(), "test-challenge", result.OAuthParameters.CodeChallenge)
	assert.Equal(suite.T(), "S256", result.OAuthParameters.CodeChallengeMethod)
	assert.Equal(suite.T(), []string{"https://api.example.com/resource"}, result.OAuthParameters.Resources)
}

func (suite *AuthorizationRequestStoreTestSuite) TestGetRequest_EmptyKey() {
	ok, _, err := suite.store.GetRequest(context.Background(), "")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), ok)
}

func (suite *AuthorizationRequestStoreTestSuite) TestGetRequest_NotFound() {
	ok, result, err := suite.store.GetRequest(context.Background(), "missing")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), ok)
	assert.Equal(suite.T(), authRequestContext{}, result)
}

func (suite *AuthorizationRequestStoreTestSuite) TestGetRequest_GetError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("get failed"))
	store := &authorizationRequestStore{
		storeProvider: rt,
	}

	ok, _, err := store.GetRequest(context.Background(), "test-request-id")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), ok)
}

func (suite *AuthorizationRequestStoreTestSuite) TestGetRequest_InvalidJSON() {
	store := inmemory.Initialize("test-deployment")
	suite.Require().NoError(store.Put(context.Background(), providers.NamespaceAuthzReq,
		"test-request-id", []byte("{invalid json"), 60))

	authReqStore := &authorizationRequestStore{storeProvider: store}
	ok, _, err := authReqStore.GetRequest(context.Background(), "test-request-id")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), ok)
}

// Tests for ClearRequest

func (suite *AuthorizationRequestStoreTestSuite) TestClearRequest_Success() {
	identifier, err := suite.store.AddRequest(context.Background(), suite.testAuthRequestContext)
	suite.Require().NoError(err)

	err = suite.store.ClearRequest(context.Background(), identifier)
	assert.NoError(suite.T(), err)

	ok, _, err := suite.store.GetRequest(context.Background(), identifier)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), ok)
}

func (suite *AuthorizationRequestStoreTestSuite) TestClearRequest_EmptyKey() {
	err := suite.store.ClearRequest(context.Background(), "")
	assert.NoError(suite.T(), err)
}

func (suite *AuthorizationRequestStoreTestSuite) TestClearRequest_DeleteError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Delete(mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("delete failed"))
	store := &authorizationRequestStore{
		storeProvider: rt,
	}

	err := store.ClearRequest(context.Background(), "test-request-id")
	assert.Error(suite.T(), err)
}

// TestRoundTrip_VerifiedClaims verifies that a claims request carrying userinfo.verified_claims
// survives serialization to the store JSON and reconstruction faithfully.
func (suite *AuthorizationRequestStoreTestSuite) TestRoundTrip_VerifiedClaims() {
	claimsParam := `{
		"userinfo": {
			"email": {"essential": true},
			"verified_claims": {
				"verification": {"trust_framework": {"value": "eidas"}, "evidence": [{"type": "document"}]},
				"claims": {"given_name": null, "address": {"essential": true}}
			}
		}
	}`
	claimsRequest, err := oauth2utils.ParseClaimsRequest(claimsParam)
	suite.Require().NoError(err)

	original := authRequestContext{
		OAuthParameters: model.OAuthParameters{
			ClientID:       "test-client-id",
			ResponseType:   "code",
			StandardScopes: []string{"openid"},
			ClaimsRequest:  claimsRequest,
		},
	}

	identifier, err := suite.store.AddRequest(context.Background(), original)
	suite.Require().NoError(err)

	ok, reconstructed, err := suite.store.GetRequest(context.Background(), identifier)
	suite.Require().NoError(err)
	suite.Require().True(ok)

	reloaded := reconstructed.OAuthParameters.ClaimsRequest
	suite.Require().NotNil(reloaded)

	// verified_claims is reconstructed as a single normalized entry (unmodeled members dropped).
	suite.Require().Len(reloaded.VerifiedUserInfo, 1)
	verifiedEntry := reloaded.VerifiedUserInfo[0]
	assert.Equal(suite.T(), "eidas", verifiedEntry.Verification.TrustFramework.Value)
	assert.Len(suite.T(), verifiedEntry.Claims, 2)
	assert.True(suite.T(), verifiedEntry.Claims["address"].Essential)

	// Normal claims are preserved and resolvable.
	normalClaims := reloaded.UserInfo
	assert.Len(suite.T(), normalClaims, 1)
	assert.NotNil(suite.T(), normalClaims["email"])
	assert.True(suite.T(), normalClaims["email"].Essential)
}
