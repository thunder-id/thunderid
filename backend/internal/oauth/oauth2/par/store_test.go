/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package par

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// StoreTestSuite exercises the parRequestStore adapter against a real in-memory runtime store,
// verifying the store/consume-once semantics.
type StoreTestSuite struct {
	suite.Suite
	store       *parRequestStore
	ctx         context.Context
	testRequest pushedAuthorizationRequest
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	s.store = &parRequestStore{storeProvider: inmemory.Initialize("test-deployment")}
	s.ctx = context.Background()
	s.testRequest = pushedAuthorizationRequest{
		ClientID: "test-client",
		OAuthParameters: model.OAuthParameters{
			ClientID:            "test-client",
			RedirectURI:         "https://example.com/callback",
			ResponseType:        "code",
			State:               "test-state",
			StandardScopes:      []string{"openid", "profile"},
			PermissionScopes:    []string{"read", "write"},
			CodeChallenge:       "challenge123",
			CodeChallengeMethod: "S256",
			Resources:           []string{"https://api.example.com"},
			ClaimsLocales:       "en",
			Nonce:               "nonce123",
		},
	}
}

// Tests for Store

func (s *StoreTestSuite) TestStore_Success() {
	randomKey, err := s.store.Store(s.ctx, s.testRequest, 60)

	s.Require().NoError(err)
	s.NotEmpty(randomKey)
}

func (s *StoreTestSuite) TestStore_GeneratesUniqueURIs() {
	key1, err1 := s.store.Store(s.ctx, s.testRequest, 60)
	key2, err2 := s.store.Store(s.ctx, s.testRequest, 60)

	s.Require().NoError(err1)
	s.Require().NoError(err2)
	s.NotEqual(key1, key2)
}

func (s *StoreTestSuite) TestStore_PutError() {
	rt := NewRuntimeStoreProviderMock(s.T())
	rt.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("put failed"))
	store := &parRequestStore{storeProvider: rt}

	randomKey, err := store.Store(s.ctx, s.testRequest, 60)

	s.Error(err)
	s.Contains(err.Error(), "failed to store PAR request")
	s.Empty(randomKey)
}

// Tests for Consume

func (s *StoreTestSuite) TestConsume_Success() {
	randomKey, err := s.store.Store(s.ctx, s.testRequest, 60)
	s.Require().NoError(err)

	result, found, err := s.store.Consume(s.ctx, randomKey)

	s.Require().NoError(err)
	s.True(found)
	s.Equal(s.testRequest.ClientID, result.ClientID)
	s.Equal(s.testRequest.OAuthParameters.RedirectURI, result.OAuthParameters.RedirectURI)
	s.Equal(s.testRequest.OAuthParameters.StandardScopes, result.OAuthParameters.StandardScopes)
	s.Equal(s.testRequest.OAuthParameters.PermissionScopes, result.OAuthParameters.PermissionScopes)
}

func (s *StoreTestSuite) TestConsume_ConsumedOnce() {
	randomKey, err := s.store.Store(s.ctx, s.testRequest, 60)
	s.Require().NoError(err)

	_, found, err := s.store.Consume(s.ctx, randomKey)
	s.Require().NoError(err)
	s.True(found)

	_, found, err = s.store.Consume(s.ctx, randomKey)
	s.Require().NoError(err)
	s.False(found, "a consumed PAR request must not be retrievable again")
}

func (s *StoreTestSuite) TestConsume_NotFound() {
	result, found, err := s.store.Consume(s.ctx, "missing")

	s.Require().NoError(err)
	s.False(found)
	s.Equal(pushedAuthorizationRequest{}, result)
}

func (s *StoreTestSuite) TestConsume_TakeError() {
	rt := NewRuntimeStoreProviderMock(s.T())
	rt.EXPECT().Take(mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("take failed"))
	store := &parRequestStore{storeProvider: rt}

	_, found, err := store.Consume(s.ctx, "abc")

	s.Error(err)
	s.Contains(err.Error(), "failed to retrieve PAR request")
	s.False(found)
}

func (s *StoreTestSuite) TestConsume_InvalidJSON() {
	store := inmemory.Initialize("test-deployment")
	s.Require().NoError(store.Put(s.ctx, providers.NamespacePAR, "abc", []byte("{invalid json"), 60))

	parStore := &parRequestStore{storeProvider: store}
	_, found, err := parStore.Consume(s.ctx, "abc")

	s.Error(err)
	s.False(found)
}

// Tests for helpers

func (s *StoreTestSuite) TestGenerateRandomKey() {
	key, err := generateRandomKey()

	s.Require().NoError(err)
	// 32 bytes base64url encoded = 43 chars.
	s.True(len(key) == 43, "expected random key of length 43, got %d", len(key))
}
