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

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AuthorizationCodeStoreTestSuite exercises the authorizationCodeStore adapter against a real
// in-memory runtime store, verifying the insert/get/consume-once semantics.
type AuthorizationCodeStoreTestSuite struct {
	suite.Suite
	store         *authorizationCodeStore
	testAuthzCode AuthorizationCode
}

func TestAuthorizationCodeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationCodeStoreTestSuite))
}

func (suite *AuthorizationCodeStoreTestSuite) SetupTest() {
	suite.store = &authorizationCodeStore{storeProvider: inmemory.Initialize("test-deployment")}

	suite.testAuthzCode = AuthorizationCode{
		CodeID:              "test-code-id",
		Code:                "test-code",
		ClientID:            "test-client-id",
		RedirectURI:         "https://client.example.com/callback",
		RedirectURIProvided: true,
		AuthorizedUserID:    "test-user-id",
		TimeCreated:         time.Now(),
		ExpiryTime:          time.Now().Add(10 * time.Minute),
		Scopes:              "read write",
		State:               AuthCodeStateActive,
		CodeChallenge:       "",
		CodeChallengeMethod: "",
	}
}

func (suite *AuthorizationCodeStoreTestSuite) TestNewAuthorizationCodeStore() {
	store := newAuthorizationCodeStore(inmemory.Initialize("test-deployment"))
	assert.NotNil(suite.T(), store)
	assert.Implements(suite.T(), (*AuthorizationCodeStoreInterface)(nil), store)
}

// Tests for the consumed-code replay markers

func (suite *AuthorizationCodeStoreTestSuite) TestMarkAndReadConsumedTokenFamily() {
	err := suite.store.MarkConsumedTokenFamily(context.Background(), "code-x", "tfid-x", time.Minute)
	suite.NoError(err)

	tfid, found, err := suite.store.ConsumedTokenFamily(context.Background(), "code-x")
	suite.NoError(err)
	suite.True(found)
	suite.Equal("tfid-x", tfid)
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumedTokenFamily_Missing() {
	tfid, found, err := suite.store.ConsumedTokenFamily(context.Background(), "no-such-code")
	suite.NoError(err)
	suite.False(found)
	suite.Empty(tfid)
}

func (suite *AuthorizationCodeStoreTestSuite) TestMarkConsumedTokenFamily_EmptyTokenFamilyIsNoOp() {
	err := suite.store.MarkConsumedTokenFamily(context.Background(), "code-y", "", time.Minute)
	suite.NoError(err)

	_, found, err := suite.store.ConsumedTokenFamily(context.Background(), "code-y")
	suite.NoError(err)
	suite.False(found)
}

// Tests for InsertAuthorizationCode

func (suite *AuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_Success() {
	err := suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode)
	assert.NoError(suite.T(), err)
}

func (suite *AuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_PutError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("put failed"))
	store := &authorizationCodeStore{storeProvider: rt}

	err := store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error inserting authorization code")
}

func (suite *AuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_AlreadyExpired() {
	expiredCode := suite.testAuthzCode
	expiredCode.ExpiryTime = time.Now().Add(-time.Minute)

	err := suite.store.InsertAuthorizationCode(context.Background(), expiredCode)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authorization code already expired")
}

// Tests for GetAuthorizationCode

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_Success() {
	suite.Require().NoError(suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode))

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-code-id", result.CodeID)
	assert.Equal(suite.T(), "test-code", result.Code)
	assert.Equal(suite.T(), "test-client-id", result.ClientID)
	assert.Equal(suite.T(), "https://client.example.com/callback", result.RedirectURI)
	assert.True(suite.T(), result.RedirectURIProvided)
	assert.Equal(suite.T(), "test-user-id", result.AuthorizedUserID)
	assert.Equal(suite.T(), "read write", result.Scopes)
	assert.Equal(suite.T(), AuthCodeStateActive, result.State)
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_NotFound() {
	result, err := suite.store.GetAuthorizationCode(context.Background(), "missing-code")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), errAuthorizationCodeNotFound, err)
	assert.Nil(suite.T(), result)
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_GetError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("get failed"))
	store := &authorizationCodeStore{storeProvider: rt}

	result, err := store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error while retrieving authorization code")
	assert.Nil(suite.T(), result)
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidJSON() {
	store := inmemory.Initialize("test-deployment")
	suite.Require().NoError(store.Put(context.Background(), providers.NamespaceAuthzCode,
		"test-code", []byte("{invalid json"), 60))

	authCodeStore := &authorizationCodeStore{storeProvider: store}
	result, err := authCodeStore.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to unmarshal authorization code")
	assert.Nil(suite.T(), result)
}

// Tests for ConsumeAuthorizationCode

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_Success() {
	suite.Require().NoError(suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode))

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), consumed)
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_AlreadyConsumed() {
	suite.Require().NoError(suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode))

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	suite.Require().NoError(err)
	suite.Require().True(consumed)

	consumed, err = suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), consumed, "a consumed authorization code must not be consumable again")
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_NotFound() {
	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "missing-code")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), consumed)
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_TakeError() {
	rt := NewRuntimeStoreProviderMock(suite.T())
	rt.EXPECT().Take(mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("take failed"))
	store := &authorizationCodeStore{storeProvider: rt}

	consumed, err := store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error consuming authorization code")
	assert.False(suite.T(), consumed)
}

// GetAuthorizationCode after ConsumeAuthorizationCode should also no longer find the code, since
// Consume removes it from the store.

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_AfterConsume() {
	suite.Require().NoError(suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode))

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	suite.Require().NoError(err)
	suite.Require().True(consumed)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.ErrorIs(suite.T(), err, errAuthorizationCodeNotFound)
	assert.Nil(suite.T(), result)
}
